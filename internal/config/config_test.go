package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Constants ---

func TestSlackAPIBase_Default(t *testing.T) {
	t.Setenv("SLACK_BASE_URL", "")
	if got := SlackAPIBase(); got != "https://slack.com/api" {
		t.Errorf("SlackAPIBase() = %q, want %q", got, "https://slack.com/api")
	}
}

func TestSlackAPIBase_Override(t *testing.T) {
	t.Setenv("SLACK_BASE_URL", "http://127.0.0.1:12345/api/")
	if got := SlackAPIBase(); got != "http://127.0.0.1:12345/api" {
		t.Errorf("SlackAPIBase() = %q, want trimmed override", got)
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

// --- DataDir ---

func TestDataDirDefault(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "lambdal", "derived", "slack-cli")
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDirOverride_LegacySlackDataDir(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "/tmp/slack-test-data")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	if got != "/tmp/slack-test-data" {
		t.Errorf("DataDir() = %q, want %q", got, "/tmp/slack-test-data")
	}
}

func TestDataDirOverride_SlackCLIDerivedDir(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "/tmp/slack-cli-derived")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	if got != "/tmp/slack-cli-derived" {
		t.Errorf("DataDir() = %q, want %q", got, "/tmp/slack-cli-derived")
	}
}

func TestDataDirOverride_LambdalDerivedDir(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "/tmp/lambdal-derived")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	if got != "/tmp/lambdal-derived/slack-cli" {
		t.Errorf("DataDir() = %q, want %q", got, "/tmp/lambdal-derived/slack-cli")
	}
}

func TestDataDir_PriorityOrder(t *testing.T) {
	// SLACK_DATA_DIR takes highest priority.
	t.Setenv("SLACK_DATA_DIR", "/tmp/highest-priority")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "/tmp/medium-priority")
	t.Setenv("LAMBDAL_DERIVED_DIR", "/tmp/lowest-priority")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	if got != "/tmp/highest-priority" {
		t.Errorf("DataDir() = %q, want %q (SLACK_DATA_DIR should win)", got, "/tmp/highest-priority")
	}
}

// --- CacheDir ---

func TestCacheDirSameAsDataDir(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	dataDir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() returned unexpected error: %v", err)
	}
	cacheDir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir() returned unexpected error: %v", err)
	}
	if cacheDir != dataDir {
		t.Errorf("CacheDir() = %q, want same as DataDir() = %q", cacheDir, dataDir)
	}
}

func TestCacheDirRespectsDataDirOverride(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "/tmp/slack-test-data")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir() returned unexpected error: %v", err)
	}
	if got != "/tmp/slack-test-data" {
		t.Errorf("CacheDir() = %q, want %q", got, "/tmp/slack-test-data")
	}
}

// --- UserCachePath ---

func TestUserCachePathDefault(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "")
	t.Setenv("SLACK_DATA_DIR", "")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := UserCachePath()
	if err != nil {
		t.Fatalf("UserCachePath() returned unexpected error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "lambdal", "derived", "slack-cli", "users.json")
	if got != want {
		t.Errorf("UserCachePath() = %q, want %q", got, want)
	}
}

func TestUserCachePathOverride(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "/tmp/my-users.json")
	got, err := UserCachePath()
	if err != nil {
		t.Fatalf("UserCachePath() returned unexpected error: %v", err)
	}
	if got != "/tmp/my-users.json" {
		t.Errorf("UserCachePath() = %q, want %q", got, "/tmp/my-users.json")
	}
}

func TestUserCachePathRespectsDataDirOverride(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "")
	t.Setenv("SLACK_DATA_DIR", "/tmp/slack-custom")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	got, err := UserCachePath()
	if err != nil {
		t.Fatalf("UserCachePath() returned unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/slack-custom", "users.json")
	if got != want {
		t.Errorf("UserCachePath() = %q, want %q", got, want)
	}
}

// --- UserCachePath takes priority over DataDir when both set ---

func TestUserCachePathOverrideTakesPriorityOverDataDir(t *testing.T) {
	t.Setenv("SLACK_USER_CACHE", "/tmp/explicit-users.json")
	t.Setenv("SLACK_DATA_DIR", "/tmp/should-be-ignored")
	got, err := UserCachePath()
	if err != nil {
		t.Fatalf("UserCachePath() returned unexpected error: %v", err)
	}
	if got != "/tmp/explicit-users.json" {
		t.Errorf("UserCachePath() = %q, want %q — SLACK_USER_CACHE should take priority over SLACK_DATA_DIR", got, "/tmp/explicit-users.json")
	}
}

// --- CacheDir with env override returns same as DataDir override ---

func TestCacheDirOverrideSameAsDataDir(t *testing.T) {
	t.Setenv("SLACK_DATA_DIR", "/tmp/slack-shared")
	t.Setenv("SLACK_CLI_DERIVED_DIR", "")
	t.Setenv("LAMBDAL_DERIVED_DIR", "")
	d, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	c, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir() error: %v", err)
	}
	if d != c {
		t.Errorf("DataDir()=%q and CacheDir()=%q should be equal when SLACK_DATA_DIR is set", d, c)
	}
}
