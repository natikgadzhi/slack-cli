//go:build e2e

// Package tests contains end-to-end tests that build the real slack-cli binary
// and run it as a subprocess. These tests are gated behind the "e2e" build tag
// so they are excluded from normal `go test ./...` runs.
//
// Tests that require real Slack credentials check for SLACK_XOXC and SLACK_XOXD
// environment variables and skip (not fail) if they are absent.
//
// Required env vars for credential tests:
//
//	SLACK_XOXC             — valid xoxc token
//	SLACK_XOXD             — valid xoxd token
//	SLACK_TEST_MESSAGE_URL — a known Slack message URL
//	SLACK_TEST_CHANNEL     — a channel name or ID
//	SLACK_TEST_SEARCH_QUERY — a search term known to return results
//	SLACK_TEAM_URL         — workspace URL (to avoid auth.test call in tests)
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
// Set once in TestMain and reused by all tests.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "slack-cli-e2e-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "slack-cli")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/slack-cli")
	// Set the working directory to the repo root (one level up from tests/).
	buildCmd.Dir = filepath.Join(".")
	// Walk up to find the module root (where go.mod lives).
	// Since tests/ is a subdirectory, we need the parent.
	moduleRoot, err := findModuleRoot()
	if err != nil {
		panic("failed to find module root: " + err.Error())
	}
	buildCmd.Dir = moduleRoot
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdout = os.Stdout

	if err := buildCmd.Run(); err != nil {
		panic("failed to build slack-cli binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// findModuleRoot walks up from the current working directory to find go.mod.
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
		t.Skip("SLACK_XOXC and SLACK_XOXD required for E2E tests")
	}
}

// requireEnv skips the test if the named environment variable is not set,
// and returns its value.
func requireEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s required for this E2E test", name)
	}
	return v
}

// runBinary executes the slack-cli binary with the given arguments and
// environment. It returns stdout, stderr, and any error.
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
		t.Fatalf("slack-cli --help failed: %v", err)
	}

	for _, want := range []string{"auth", "message", "channel", "search"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--help output missing %q", want)
		}
	}
}

func TestAuthHelp(t *testing.T) {
	stdout, _, err := runBinary(t, nil, "auth", "--help")
	if err != nil {
		t.Fatalf("slack-cli auth --help failed: %v", err)
	}

	// The auth subcommand should list its children.
	for _, want := range []string{"check", "set-xoxc", "set-xoxd"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("auth --help output missing %q", want)
		}
	}
}

func TestInvalidURL(t *testing.T) {
	_, _, err := runBinary(t, nil, "message", "not-a-url")
	if err == nil {
		t.Fatal("expected non-zero exit for invalid URL, got success")
	}
}

func TestMissingTokens(t *testing.T) {
	// Run with a clean environment that has no SLACK_XOXC / SLACK_XOXD
	// and no access to Keychain tokens (since they won't match).
	// We keep PATH so the binary can find system libraries.
	cleanEnv := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	_, _, err := runBinary(t, cleanEnv, "message", "https://example.slack.com/archives/C12345/p1741234567123456")
	if err == nil {
		t.Fatal("expected non-zero exit when tokens are missing, got success")
	}
}

// --- Tests that REQUIRE credentials ---

func TestAuthCheck(t *testing.T) {
	requireCreds(t)

	// auth check writes to stderr, not stdout.
	_, stderr, err := runBinary(t, nil, "auth", "check")
	if err != nil {
		t.Fatalf("auth check failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stderr, "[OK]") {
		t.Errorf("auth check output missing [OK], got: %s", stderr)
	}
	if !strings.Contains(stderr, "authenticated") {
		t.Errorf("auth check output missing 'authenticated', got: %s", stderr)
	}
}

func TestMessageJSON(t *testing.T) {
	requireCreds(t)
	msgURL := requireEnv(t, "SLACK_TEST_MESSAGE_URL")

	stdout, stderr, err := runBinary(t, nil, "message", "-o", "json", msgURL)
	if err != nil {
		t.Fatalf("message command failed: %v\nstderr: %s", err, stderr)
	}

	// The output should be a valid JSON array of messages.
	var messages []map[string]any
	if err := json.Unmarshal([]byte(stdout), &messages); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nstdout: %s", err, stdout)
	}

	if len(messages) == 0 {
		t.Fatal("expected at least one message, got empty array")
	}

	// Each message should have "ts" and "text" fields.
	first := messages[0]
	if _, ok := first["ts"]; !ok {
		t.Error("first message missing 'ts' field")
	}
	if _, ok := first["text"]; !ok {
		t.Error("first message missing 'text' field")
	}
}

func TestMessageMarkdown(t *testing.T) {
	requireCreds(t)
	msgURL := requireEnv(t, "SLACK_TEST_MESSAGE_URL")

	stdout, stderr, err := runBinary(t, nil, "message", "-o", "markdown", msgURL)
	if err != nil {
		t.Fatalf("message markdown command failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "##") {
		t.Errorf("markdown output missing '##' header, got: %s", truncate(stdout, 200))
	}
}

func TestChannelJSON(t *testing.T) {
	requireCreds(t)
	channel := requireEnv(t, "SLACK_TEST_CHANNEL")

	stdout, stderr, err := runBinary(t, nil, "channel", "-o", "json", "--since", "7d", "--limit", "5", channel)
	if err != nil {
		t.Fatalf("channel command failed: %v\nstderr: %s", err, stderr)
	}

	var messages []map[string]any
	if err := json.Unmarshal([]byte(stdout), &messages); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nstdout: %s", err, stdout)
	}

	if len(messages) > 5 {
		t.Errorf("expected at most 5 messages, got %d", len(messages))
	}
}

func TestChannelMarkdown(t *testing.T) {
	requireCreds(t)
	channel := requireEnv(t, "SLACK_TEST_CHANNEL")

	stdout, stderr, err := runBinary(t, nil, "channel", "-o", "markdown", "--since", "7d", "--limit", "5", channel)
	if err != nil {
		t.Fatalf("channel markdown command failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "##") {
		t.Errorf("markdown output missing '##' header, got: %s", truncate(stdout, 200))
	}
}

func TestSearchJSON(t *testing.T) {
	requireCreds(t)
	query := requireEnv(t, "SLACK_TEST_SEARCH_QUERY")

	stdout, stderr, err := runBinary(t, nil, "search", "-o", "json", "--count", "3", query)
	if err != nil {
		t.Fatalf("search command failed: %v\nstderr: %s", err, stderr)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nstdout: %s", err, stdout)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one search result, got empty array")
	}

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

// truncate returns s truncated to maxLen characters, with "..." appended if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
