package auth

import (
	"testing"

	cliauth "github.com/natikgadzhi/cli-kit/auth"
)

func TestResolveXoxc_EnvVarSanitizesAndReportsSource(t *testing.T) {
	// Trailing newline is a common paste artifact that would corrupt the header.
	t.Setenv("SLACK_XOXC", "  xoxc-from-env\n")

	cred, err := ResolveXoxc()
	if err != nil {
		t.Fatalf("ResolveXoxc returned unexpected error: %v", err)
	}
	if cred.Token != "xoxc-from-env" {
		t.Errorf("ResolveXoxc token = %q, want %q (should be sanitized)", cred.Token, "xoxc-from-env")
	}
	if cred.Source != cliauth.SourceEnvironment {
		t.Errorf("ResolveXoxc source = %q, want %q", cred.Source, cliauth.SourceEnvironment)
	}
	if len(cred.Warnings) == 0 {
		t.Errorf("ResolveXoxc expected a whitespace warning, got none")
	}
}

func TestResolveXoxd_EnvVarStripsCookieNameAndReportsSource(t *testing.T) {
	// "d=" cookie-name prefix is a common paste artifact; encoding is preserved.
	t.Setenv("SLACK_XOXD", "d=xoxd-abc%2Fdef%3D")

	cred, err := ResolveXoxd()
	if err != nil {
		t.Fatalf("ResolveXoxd returned unexpected error: %v", err)
	}
	if cred.Token != "xoxd-abc%2Fdef%3D" {
		t.Errorf("ResolveXoxd token = %q, want %q (d= stripped, encoding preserved)", cred.Token, "xoxd-abc%2Fdef%3D")
	}
	if cred.Source != cliauth.SourceEnvironment {
		t.Errorf("ResolveXoxd source = %q, want %q", cred.Source, cliauth.SourceEnvironment)
	}
}

func TestGetXoxd_PreservesEncodedEnvVar(t *testing.T) {
	// Slack expects the percent-encoded cookie sent as-is; do not decode it.
	t.Setenv("SLACK_XOXD", "xoxd-abc%2Fdef")

	token, err := GetXoxd()
	if err != nil {
		t.Fatalf("GetXoxd returned unexpected error: %v", err)
	}
	if token != "xoxd-abc%2Fdef" {
		t.Errorf("GetXoxd = %q, want unchanged %q", token, "xoxd-abc%2Fdef")
	}
}

func TestGetXoxc_EnvVar(t *testing.T) {
	t.Setenv("SLACK_XOXC", "xoxc-from-env")

	token, err := GetXoxc()
	if err != nil {
		t.Fatalf("GetXoxc returned unexpected error: %v", err)
	}
	if token != "xoxc-from-env" {
		t.Errorf("GetXoxc = %q, want %q", token, "xoxc-from-env")
	}
}

func TestGetXoxd_EnvVar(t *testing.T) {
	t.Setenv("SLACK_XOXD", "xoxd-from-env")

	token, err := GetXoxd()
	if err != nil {
		t.Fatalf("GetXoxd returned unexpected error: %v", err)
	}
	if token != "xoxd-from-env" {
		t.Errorf("GetXoxd = %q, want %q", token, "xoxd-from-env")
	}
}

func TestGetXoxc_EnvVarPriority(t *testing.T) {
	// Env var should take priority over keychain.
	t.Setenv("SLACK_XOXC", "xoxc-env-priority")

	token, err := GetXoxc()
	if err != nil {
		t.Fatalf("GetXoxc returned unexpected error: %v", err)
	}
	if token != "xoxc-env-priority" {
		t.Errorf("GetXoxc = %q, want %q — env var should take priority over keychain", token, "xoxc-env-priority")
	}
}

func TestGetXoxd_EnvVarPriority(t *testing.T) {
	// Env var should take priority over keychain.
	t.Setenv("SLACK_XOXD", "xoxd-env-priority")

	token, err := GetXoxd()
	if err != nil {
		t.Fatalf("GetXoxd returned unexpected error: %v", err)
	}
	if token != "xoxd-env-priority" {
		t.Errorf("GetXoxd = %q, want %q — env var should take priority over keychain", token, "xoxd-env-priority")
	}
}
