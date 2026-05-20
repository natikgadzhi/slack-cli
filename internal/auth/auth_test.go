package auth

import (
	"testing"

	cliauth "github.com/natikgadzhi/cli-kit/auth"
)

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

func TestResolveXoxc_EnvVarSanitizesAndReportsSource(t *testing.T) {
	t.Setenv("SLACK_XOXC", "  Bearer xoxc-from-env  ")

	cred, err := ResolveXoxc()
	if err != nil {
		t.Fatalf("ResolveXoxc returned unexpected error: %v", err)
	}
	if cred.Token != "xoxc-from-env" {
		t.Errorf("ResolveXoxc token = %q, want sanitized token", cred.Token)
	}
	if cred.RawToken != "  Bearer xoxc-from-env  " {
		t.Errorf("ResolveXoxc raw token = %q, want original token", cred.RawToken)
	}
	if cred.Source != cliauth.SourceEnvironment {
		t.Errorf("ResolveXoxc source = %q, want environment", cred.Source)
	}
	if len(cred.Warnings) != 2 {
		t.Errorf("ResolveXoxc warnings = %v, want whitespace + Bearer warnings", cred.Warnings)
	}
}

func TestResolveXoxd_EnvVarStripsCookieNameAndEncodes(t *testing.T) {
	t.Setenv("SLACK_XOXD", "d=xoxd-X+y/Z=")

	cred, err := ResolveXoxd()
	if err != nil {
		t.Fatalf("ResolveXoxd returned unexpected error: %v", err)
	}
	if cred.Token != "xoxd-X%2By%2FZ%3D" {
		t.Errorf("ResolveXoxd token = %q, want stripped and encoded xoxd", cred.Token)
	}
	if cred.RawToken != "d=xoxd-X+y/Z=" {
		t.Errorf("ResolveXoxd raw token = %q, want original token", cred.RawToken)
	}
	if cred.Source != cliauth.SourceEnvironment {
		t.Errorf("ResolveXoxd source = %q, want environment", cred.Source)
	}
	if len(cred.Warnings) != 2 {
		t.Errorf("ResolveXoxd warnings = %v, want d= strip + encoding warnings", cred.Warnings)
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
