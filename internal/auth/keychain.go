// Package auth provides authentication helpers for slack-cli,
// including macOS Keychain integration and token sanitization.
package auth

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// CommandExecutor is a function type that creates an *exec.Cmd.
// It mirrors the signature of exec.Command and can be replaced in tests.
type CommandExecutor func(name string, arg ...string) *exec.Cmd

// execCommand is the package-level command executor.
// Tests can replace this to intercept shell calls.
var execCommand CommandExecutor = exec.Command

// KeychainGet retrieves a password from the macOS Keychain.
// It calls `security find-generic-password -a <account> -s <service> -w`.
func KeychainGet(service string) (string, error) {
	account := config.KeychainAccount()
	cmd := execCommand("security", "find-generic-password", "-a", account, "-s", service, "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(
			"keychain entry not found (service=%q, account=%q): %w\n"+
				"  Store it with: security add-generic-password -a %q -s %q -w <token>",
			service, account, err, account, service,
		)
	}
	return strings.TrimSpace(string(out)), nil
}

// KeychainSet stores a password in the macOS Keychain.
// It first deletes any existing entry (ignoring errors), then adds the new one.
func KeychainSet(service, token string) error {
	account := config.KeychainAccount()

	// Delete existing entry (ignore errors — it may not exist).
	delCmd := execCommand("security", "delete-generic-password", "-a", account, "-s", service)
	_ = delCmd.Run()

	// Add new entry.
	addCmd := execCommand("security", "add-generic-password", "-a", account, "-s", service, "-w", token)
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to store keychain entry (service=%q, account=%q): %w", service, account, err)
	}
	return nil
}
