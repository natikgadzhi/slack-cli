package auth

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHelperProcess is invoked as a subprocess by the mocked exec.Command.
// It is not a real test — the go test framework calls it, but it exits immediately
// unless the TEST_HELPER_PROCESS env var is set.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("TEST_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	// Find the "--" separator that we inject in fakeExecCommand.
	idx := -1
	for i, a := range args {
		if a == "--" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "no command after --\n")
		os.Exit(1)
	}
	cmd := args[idx+1:]

	// Dispatch based on the expected behavior env var.
	behavior := os.Getenv("TEST_HELPER_BEHAVIOR")

	switch behavior {
	case "keychain_get_success":
		// Simulate `security find-generic-password ... -w` returning a token.
		fmt.Fprintf(os.Stdout, "xoxc-test-token-123\n")
		os.Exit(0)

	case "keychain_get_failure":
		// Simulate `security find-generic-password` not finding the entry.
		fmt.Fprintf(os.Stderr, "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain.\n")
		os.Exit(44)

	case "keychain_set_success":
		// Both delete and add succeed (exit 0).
		os.Exit(0)

	case "keychain_set_add_failure":
		// Simulate the add step failing. We look at the args to differentiate
		// delete (should succeed) vs add (should fail).
		for _, a := range cmd {
			if a == "add-generic-password" {
				fmt.Fprintf(os.Stderr, "security: add failed\n")
				os.Exit(1)
			}
		}
		// delete-generic-password or anything else succeeds.
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "unknown behavior: %q\n", behavior)
		os.Exit(1)
	}
}

// fakeExecCommand returns a function that replaces exec.Command in tests.
// Instead of running the real binary, it re-invokes the test binary with the
// helper process env vars set.
func fakeExecCommand(behavior string) CommandExecutor {
	return func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"TEST_HELPER_PROCESS=1",
			"TEST_HELPER_BEHAVIOR="+behavior,
		)
		return cmd
	}
}

func TestKeychainGet_Success(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_get_success")

	token, err := KeychainGet("slack-xoxc-token")
	if err != nil {
		t.Fatalf("KeychainGet returned unexpected error: %v", err)
	}
	if token != "xoxc-test-token-123" {
		t.Errorf("KeychainGet = %q, want %q", token, "xoxc-test-token-123")
	}
}

func TestKeychainGet_Failure(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_get_failure")

	_, err := KeychainGet("slack-xoxc-token")
	if err == nil {
		t.Fatal("KeychainGet should have returned an error")
	}
	if !strings.Contains(err.Error(), "keychain entry not found") {
		t.Errorf("error message should mention 'keychain entry not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "slack-xoxc-token") {
		t.Errorf("error message should contain service name, got: %v", err)
	}
}

func TestKeychainSet_Success(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_set_success")

	err := KeychainSet("slack-xoxc-token", "xoxc-new-token")
	if err != nil {
		t.Fatalf("KeychainSet returned unexpected error: %v", err)
	}
}

func TestKeychainSet_AddFailure(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = fakeExecCommand("keychain_set_add_failure")

	err := KeychainSet("slack-xoxc-token", "xoxc-new-token")
	if err == nil {
		t.Fatal("KeychainSet should have returned an error when add fails")
	}
	if !strings.Contains(err.Error(), "failed to store keychain entry") {
		t.Errorf("error message should mention 'failed to store keychain entry', got: %v", err)
	}
}
