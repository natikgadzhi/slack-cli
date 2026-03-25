// Package config provides shared constants and configuration for the Slack CLI.
// Configuration values can be overridden via environment variables.
package config

import (
	"fmt"
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

// DataDir returns the base data directory for slack-cli.
// Override with SLACK_CLI_DERIVED_DIR or LAMBDAL_DERIVED_DIR env vars.
// Defaults to ~/.local/share/lambdal/derived/slack-cli/.
func DataDir() (string, error) {
	// Legacy env var for backwards compatibility.
	if v := os.Getenv("SLACK_DATA_DIR"); v != "" {
		return v, nil
	}
	// Tool-specific derived dir.
	if v := os.Getenv("SLACK_CLI_DERIVED_DIR"); v != "" {
		return v, nil
	}
	// Base lambdal derived dir (tool appends its name).
	if v := os.Getenv("LAMBDAL_DERIVED_DIR"); v != "" {
		return filepath.Join(v, "slack-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "lambdal", "derived", "slack-cli"), nil
}

// CacheDir returns the path to the cache directory.
// This is intentionally the same as DataDir — slack-cli uses a single
// directory for both data and cache. Kept as a separate function so
// downstream consumers can call CacheDir() without knowing the layout.
func CacheDir() (string, error) {
	return DataDir()
}

// UserCachePath returns the path to the user cache JSON file.
// Override with the SLACK_USER_CACHE environment variable.
func UserCachePath() (string, error) {
	if v := os.Getenv("SLACK_USER_CACHE"); v != "" {
		return v, nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "users.json"), nil
}
