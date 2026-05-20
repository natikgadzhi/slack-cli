package auth

import (
	cliauth "github.com/natikgadzhi/cli-kit/auth"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// Credential is a resolved token plus metadata that helps auth check explain
// where the value came from and what copy-paste cleanup was applied.
type Credential struct {
	Token    string // cleaned value used by API calls
	RawToken string // original value from env/keychain before cleanup
	Source   string
	Warnings []string
}

// ResolveXoxc returns the sanitized Slack xoxc token with its source.
// It checks SLACK_XOXC first, then the OS keychain.
func ResolveXoxc() (Credential, error) {
	token, source, err := cliauth.ResolveToken(cliauth.TokenSource{
		EnvVar:          "SLACK_XOXC",
		KeychainService: config.KeychainXoxcService(),
		KeychainKey:     config.KeychainAccount(),
	})
	if err != nil {
		return Credential{}, err
	}
	clean, warnings := SanitizeToken(token)
	return Credential{Token: clean, RawToken: token, Source: source, Warnings: warnings}, nil
}

// ResolveXoxd returns the sanitized and URL-encoded Slack xoxd cookie with its
// source. It checks SLACK_XOXD first, then the OS keychain.
func ResolveXoxd() (Credential, error) {
	token, source, err := cliauth.ResolveToken(cliauth.TokenSource{
		EnvVar:          "SLACK_XOXD",
		KeychainService: config.KeychainXoxdService(),
		KeychainKey:     config.KeychainAccount(),
	})
	if err != nil {
		return Credential{}, err
	}
	clean, warnings := SanitizeXoxd(token)
	return Credential{Token: clean, RawToken: token, Source: source, Warnings: warnings}, nil
}

// GetXoxc returns the sanitized Slack xoxc token.
func GetXoxc() (string, error) {
	cred, err := ResolveXoxc()
	return cred.Token, err
}

// GetXoxd returns the sanitized and URL-encoded Slack xoxd cookie.
func GetXoxd() (string, error) {
	cred, err := ResolveXoxd()
	return cred.Token, err
}

// StoreXoxc stores the xoxc token in the OS keychain.
func StoreXoxc(token string) error {
	return cliauth.StoreToken(config.KeychainXoxcService(), config.KeychainAccount(), token)
}

// StoreXoxd stores the xoxd cookie in the OS keychain.
func StoreXoxd(token string) error {
	return cliauth.StoreToken(config.KeychainXoxdService(), config.KeychainAccount(), token)
}

// DeleteXoxc removes the xoxc token from the OS keychain.
func DeleteXoxc() error {
	return cliauth.DeleteToken(config.KeychainXoxcService(), config.KeychainAccount())
}

// DeleteXoxd removes the xoxd cookie from the OS keychain.
func DeleteXoxd() error {
	return cliauth.DeleteToken(config.KeychainXoxdService(), config.KeychainAccount())
}
