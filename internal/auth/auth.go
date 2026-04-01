package auth

import (
	cliauth "github.com/natikgadzhi/cli-kit/auth"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// GetXoxc returns the Slack xoxc token.
// It checks the SLACK_XOXC environment variable first,
// falling back to the OS keychain via cli-kit/auth.ResolveToken.
func GetXoxc() (string, error) {
	token, _, err := cliauth.ResolveToken(cliauth.TokenSource{
		EnvVar:          "SLACK_XOXC",
		KeychainService: config.KeychainXoxcService(),
		KeychainKey:     config.KeychainAccount(),
	})
	return token, err
}

// GetXoxd returns the Slack xoxd cookie.
// It checks the SLACK_XOXD environment variable first,
// falling back to the OS keychain via cli-kit/auth.ResolveToken.
func GetXoxd() (string, error) {
	token, _, err := cliauth.ResolveToken(cliauth.TokenSource{
		EnvVar:          "SLACK_XOXD",
		KeychainService: config.KeychainXoxdService(),
		KeychainKey:     config.KeychainAccount(),
	})
	return token, err
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
