package auth

import (
	"os"
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

func TestGetXoxc_KeychainFallback(t *testing.T) {
	// t.Setenv registers cleanup to restore the var after the test;
	// os.Unsetenv actually clears it so the code-under-test sees it as absent.
	t.Setenv("SLACK_XOXC", "")
	os.Unsetenv("SLACK_XOXC")

	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_get_success")

	token, err := GetXoxc()
	if err != nil {
		t.Fatalf("GetXoxc returned unexpected error: %v", err)
	}
	if token != "xoxc-test-token-123" {
		t.Errorf("GetXoxc = %q, want %q", token, "xoxc-test-token-123")
	}
}

func TestGetXoxc_KeychainError(t *testing.T) {
	// t.Setenv registers cleanup to restore the var after the test;
	// os.Unsetenv actually clears it so the code-under-test sees it as absent.
	t.Setenv("SLACK_XOXC", "")
	os.Unsetenv("SLACK_XOXC")

	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_get_failure")

	_, err := GetXoxc()
	if err == nil {
		t.Fatal("GetXoxc should have returned an error when keychain fails")
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

func TestGetXoxd_KeychainFallback(t *testing.T) {
	// t.Setenv registers cleanup to restore the var after the test;
	// os.Unsetenv actually clears it so the code-under-test sees it as absent.
	t.Setenv("SLACK_XOXD", "")
	os.Unsetenv("SLACK_XOXD")

	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_get_success_xoxd")

	token, err := GetXoxd()
	if err != nil {
		t.Fatalf("GetXoxd returned unexpected error: %v", err)
	}
	if token != "xoxd-test-token-123" {
		t.Errorf("GetXoxd = %q, want %q", token, "xoxd-test-token-123")
	}
}
