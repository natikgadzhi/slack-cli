//go:build e2e

// Package tests contains end-to-end smoke tests that build the real slack-cli
// binary and run it as a subprocess against the real Slack API.
//
// Gated behind the "e2e" build tag so `go test ./...` skips them.
// Tests that require real credentials skip (not fail) when env vars are absent.
//
// # Required env vars
//
//	SLACK_XOXC               — valid xoxc token
//	SLACK_XOXD               — valid xoxd token
//
// # Optional env vars (with defaults)
//
//	SLACK_TEAM_URL            — workspace URL (avoids extra auth.test call)
//	SLACK_TEST_MESSAGE_URL    — a known message URL (skip message tests if unset)
//	SLACK_TEST_CHANNEL        — channel name or ID (default: "general")
//	SLACK_TEST_SEARCH_QUERY   — search query (default: "wat")
//
// # What to expect
//
// These are smoke tests. We verify:
//   - Exit code 0 for valid commands
//   - Non-zero exit for invalid input
//   - Valid JSON structure in output
//   - Non-empty result sets (>0 messages/results)
//   - Correct field presence (ts, text, user)
//
// We do NOT verify exact content, message counts, or specific text.
package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath holds the path to the compiled slack-cli binary.
var binaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "slack-cli-e2e-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "slack-cli")
	moduleRoot, err := findModuleRoot()
	if err != nil {
		panic("failed to find module root: " + err.Error())
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/slack-cli")
	buildCmd.Dir = moduleRoot
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		panic("failed to build slack-cli: " + err.Error())
	}

	os.Exit(m.Run())
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// requireCreds skips the test if Slack credentials are not set.
func requireCreds(t *testing.T) {
	t.Helper()
	if os.Getenv("SLACK_XOXC") == "" || os.Getenv("SLACK_XOXD") == "" {
		t.Skip("SLACK_XOXC and SLACK_XOXD required")
	}
}

// envOrDefault returns the env var value, or the default if unset.
func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// runBinary executes the slack-cli binary with the given args.
// If env is nil, inherits the current environment.
func runBinary(t *testing.T, env []string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	if env != nil {
		cmd.Env = env
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// --- Tests that do NOT require credentials ---

func TestHelp(t *testing.T) {
	stdout, _, err := runBinary(t, nil, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	for _, cmd := range []string{"auth", "message", "channel", "search"} {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("--help missing %q command", cmd)
		}
	}
}

func TestAuthHelp(t *testing.T) {
	stdout, _, err := runBinary(t, nil, "auth", "--help")
	if err != nil {
		t.Fatalf("auth --help failed: %v", err)
	}
	for _, sub := range []string{"check", "set-xoxc", "set-xoxd"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("auth --help missing %q", sub)
		}
	}
}

func TestInvalidURL(t *testing.T) {
	_, _, err := runBinary(t, nil, "message", "not-a-url")
	if err == nil {
		t.Fatal("expected non-zero exit for invalid URL")
	}
}

func TestMissingTokens(t *testing.T) {
	// Run with minimal env — no tokens, no keychain access.
	cleanEnv := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	_, _, err := runBinary(t, cleanEnv, "message", "https://example.slack.com/archives/C12345/p1741234567123456")
	if err == nil {
		t.Fatal("expected non-zero exit when tokens are missing")
	}
}

func TestInvalidOutputFormat(t *testing.T) {
	_, _, err := runBinary(t, nil, "search", "-o", "xml", "test")
	if err == nil {
		t.Fatal("expected non-zero exit for invalid output format")
	}
}

// --- Tests that REQUIRE credentials ---

func TestAuthCheck(t *testing.T) {
	requireCreds(t)
	_, stderr, err := runBinary(t, nil, "auth", "check")
	if err != nil {
		t.Fatalf("auth check failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stderr, "[OK]") {
		t.Errorf("missing [OK] in output: %s", stderr)
	}
	if !strings.Contains(stderr, "authenticated") {
		t.Errorf("missing 'authenticated' in output: %s", stderr)
	}
}

func TestSearchSmoke(t *testing.T) {
	requireCreds(t)
	query := envOrDefault("SLACK_TEST_SEARCH_QUERY", "wat")

	stdout, stderr, err := runBinary(t, nil, "search", "-o", "json", "--count", "5", query)
	if err != nil {
		t.Fatalf("search failed: %v\nstderr: %s", err, stderr)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, truncate(stdout, 500))
	}
	if len(results) == 0 {
		t.Error("expected >0 search results")
	}
	// Verify structure: each result should have ts and text.
	for i, r := range results {
		if _, ok := r["ts"]; !ok {
			t.Errorf("result[%d] missing 'ts'", i)
		}
		if _, ok := r["text"]; !ok {
			t.Errorf("result[%d] missing 'text'", i)
		}
	}
}

func TestSearchMarkdown(t *testing.T) {
	requireCreds(t)
	query := envOrDefault("SLACK_TEST_SEARCH_QUERY", "wat")

	stdout, stderr, err := runBinary(t, nil, "search", "-o", "markdown", "--count", "3", query)
	if err != nil {
		t.Fatalf("search markdown failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "##") {
		t.Errorf("markdown output missing '##' header: %s", truncate(stdout, 300))
	}
}

func TestChannelSmoke(t *testing.T) {
	requireCreds(t)
	channel := envOrDefault("SLACK_TEST_CHANNEL", "general")

	stdout, stderr, err := runBinary(t, nil, "channel", "-o", "json", "--since", "30d", "--limit", "5", channel)
	if err != nil {
		t.Fatalf("channel failed: %v\nstderr: %s", err, stderr)
	}

	var messages []map[string]any
	if err := json.Unmarshal([]byte(stdout), &messages); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, truncate(stdout, 500))
	}
	if len(messages) == 0 {
		t.Error("expected >0 messages in channel")
	}
	if len(messages) > 5 {
		t.Errorf("expected at most 5 messages, got %d", len(messages))
	}
	// Verify structure.
	for i, m := range messages {
		if _, ok := m["ts"]; !ok {
			t.Errorf("message[%d] missing 'ts'", i)
		}
	}
}

func TestChannelMarkdown(t *testing.T) {
	requireCreds(t)
	channel := envOrDefault("SLACK_TEST_CHANNEL", "general")

	stdout, stderr, err := runBinary(t, nil, "channel", "-o", "markdown", "--since", "30d", "--limit", "3", channel)
	if err != nil {
		t.Fatalf("channel markdown failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "##") {
		t.Errorf("markdown output missing '##' header: %s", truncate(stdout, 300))
	}
}

func TestMessageSmoke(t *testing.T) {
	requireCreds(t)
	msgURL := os.Getenv("SLACK_TEST_MESSAGE_URL")
	if msgURL == "" {
		t.Skip("SLACK_TEST_MESSAGE_URL required for message tests")
	}

	stdout, stderr, err := runBinary(t, nil, "message", "-o", "json", msgURL)
	if err != nil {
		t.Fatalf("message failed: %v\nstderr: %s", err, stderr)
	}

	var messages []map[string]any
	if err := json.Unmarshal([]byte(stdout), &messages); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, truncate(stdout, 500))
	}
	if len(messages) == 0 {
		t.Fatal("expected >=1 message")
	}
	first := messages[0]
	if _, ok := first["ts"]; !ok {
		t.Error("first message missing 'ts'")
	}
	if _, ok := first["text"]; !ok {
		t.Error("first message missing 'text'")
	}
}

func TestMessageMarkdown(t *testing.T) {
	requireCreds(t)
	msgURL := os.Getenv("SLACK_TEST_MESSAGE_URL")
	if msgURL == "" {
		t.Skip("SLACK_TEST_MESSAGE_URL required for message tests")
	}

	stdout, stderr, err := runBinary(t, nil, "message", "-o", "markdown", msgURL)
	if err != nil {
		t.Fatalf("message markdown failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "##") {
		t.Errorf("markdown output missing '##' header: %s", truncate(stdout, 300))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
