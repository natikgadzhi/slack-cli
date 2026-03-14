// Package config provides shared constants and configuration for the Slack CLI.
// Configuration values can be overridden via environment variables.
package config

import (
	"os"
	"path/filepath"
)

// SlackAPIBase is the base URL for the Slack Web API.
const SlackAPIBase = "https://slack.com/api"

// UserAgent is the browser user-agent string sent with API requests
// to match what a normal browser session would send.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/124.0.0.0 Safari/537.36"

// KeychainAccount returns the macOS Keychain account name.
// Override with the SLACK_KEYCHAIN_ACCOUNT environment variable.
func KeychainAccount() string {
	if v := os.Getenv("SLACK_KEYCHAIN_ACCOUNT"); v != "" {
		return v
	}
	return "natikgadzhi"
}

// KeychainXoxcService returns the Keychain service name for the xoxc token.
// Override with the SLACK_XOXC_SERVICE environment variable.
func KeychainXoxcService() string {
	if v := os.Getenv("SLACK_XOXC_SERVICE"); v != "" {
		return v
	}
	return "slack-xoxc-token"
}

// KeychainXoxdService returns the Keychain service name for the xoxd cookie.
// Override with the SLACK_XOXD_SERVICE environment variable.
func KeychainXoxdService() string {
	if v := os.Getenv("SLACK_XOXD_SERVICE"); v != "" {
		return v
	}
	return "slack-xoxd-token"
}

// dataDir returns the base data directory for slack-cli.
// Defaults to ~/.local/share/slack-cli.
func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "slack-cli")
}

// UserCachePath returns the path to the user cache JSON file.
// Override with the SLACK_USER_CACHE environment variable.
func UserCachePath() string {
	if v := os.Getenv("SLACK_USER_CACHE"); v != "" {
		return v
	}
	return filepath.Join(dataDir(), "users.json")
}

// CacheDir returns the path to the general cache directory.
func CacheDir() string {
	return filepath.Join(dataDir(), "cache")
}
