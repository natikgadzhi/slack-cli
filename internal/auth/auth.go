package auth

import (
	"os"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// GetXoxc returns the Slack xoxc token.
// It checks the SLACK_XOXC environment variable first,
// falling back to the macOS Keychain.
func GetXoxc() (string, error) {
	if v := os.Getenv("SLACK_XOXC"); v != "" {
		return v, nil
	}
	return KeychainGet(config.KeychainXoxcService())
}

// GetXoxd returns the Slack xoxd cookie.
// It checks the SLACK_XOXD environment variable first,
// falling back to the macOS Keychain.
func GetXoxd() (string, error) {
	if v := os.Getenv("SLACK_XOXD"); v != "" {
		return v, nil
	}
	return KeychainGet(config.KeychainXoxdService())
}
