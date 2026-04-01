package auth

import (
	"testing"
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
