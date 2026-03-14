package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Constants ---

func TestSlackAPIBase(t *testing.T) {
	if SlackAPIBase != "https://slack.com/api" {
		t.Errorf("SlackAPIBase = %q, want %q", SlackAPIBase, "https://slack.com/api")
	}
}

func TestUserAgent(t *testing.T) {
	if !strings.Contains(UserAgent, "Mozilla/5.0") {
		t.Errorf("UserAgent should contain Mozilla/5.0, got %q", UserAgent)
	}
	if !strings.Contains(UserAgent, "Chrome/124.0.0.0") {
		t.Errorf("UserAgent should contain Chrome version, got %q", UserAgent)
	}
}

// --- KeychainAccount ---

func TestKeychainAccountDefault(t *testing.T) {
	t.Setenv("SLACK_KEYCHAIN_ACCOUNT", "")
	if got := KeychainAccount(); got != "natikgadzhi" {
		t.Errorf("KeychainAccount() = %q, want %q", got, "natikgadzhi")
	}
}

func TestKeychainAccountOverride(t *testing.T) {
	t.Setenv("SLACK_KEYCHAIN_ACCOUNT", "other-user")
	if got := KeychainAccount(); got != "other-user" {
		t.Errorf("KeychainAccount() = %q, want %q", got, "other-user")
	}
}

// --- KeychainXoxcService ---

func TestKeychainXoxcServiceDefault(t *testing.T) {
	t.Setenv("SLACK_XOXC_SERVICE", "")
	if got := KeychainXoxcService(); got != "slack-xoxc-token" {
		t.Errorf("KeychainXoxcService() = %q, want %q", got, "slack-xoxc-token")
	}
}

func TestKeychainXoxcServiceOverride(t *testing.T) {
	t.Setenv("SLACK_XOXC_SERVICE", "custom-xoxc")
	if got := KeychainXoxcService(); got != "custom-xoxc" {
		t.Errorf("KeychainXoxcService() = %q, want %q", got, "custom-xoxc")
	}
}

// --- KeychainXoxdService ---

func TestKeychainXoxdServiceDefault(t *testing.T) {
	t.Setenv("SLACK_XOXD_SERVICE", "")
	if got := KeychainXoxdService(); got != "slack-xoxd-token" {
		t.Errorf("KeychainXoxdService() = %q, want %q", got, "slack-xoxd-token")
	}
}

func TestKeychainXoxdServiceOverride(t *testing.T) {
	t.Setenv("SLACK_XOXD_SERVICE", "custom-xoxd")
	if got := KeychainXoxdService(); got != "custom-xoxd" {
		t.Errorf("KeychainXoxdService() = %q, want %q", got, "custom-xoxd")
	}
}

// --- UserCachePath ---

func TestUserCachePathDefault(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "")
	got := UserCachePath()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "slack-cli", "users.json")
	if got != want {
		t.Errorf("UserCachePath() = %q, want %q", got, want)
	}
}

func TestUserCachePathOverride(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "/tmp/my-users.json")
	if got := UserCachePath(); got != "/tmp/my-users.json" {
		t.Errorf("UserCachePath() = %q, want %q", got, "/tmp/my-users.json")
	}
}

// --- CacheDir ---

func TestCacheDir(t *testing.T) {
	got := CacheDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "slack-cli", "cache")
	if got != want {
		t.Errorf("CacheDir() = %q, want %q", got, want)
	}
}
