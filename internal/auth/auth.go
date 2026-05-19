package auth

import (
	cliauth "github.com/natikgadzhi/cli-kit/auth"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// Credential is a resolved token along with where it came from and any
// copy-paste artifacts that were cleaned up while resolving it.
type Credential struct {
	Token    string   // the cleaned token value
	Source   string   // cliauth.SourceEnvironment or cliauth.SourceKeychain
	Warnings []string // human-readable notes about what was cleaned
}

// ResolveXoxc resolves the xoxc token and reports its source.
// The value is run through SanitizeToken so stray whitespace, quotes, or a
// "Bearer " prefix never reach the Authorization header.
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
	return Credential{Token: clean, Source: source, Warnings: warnings}, nil
}

// ResolveXoxd resolves the xoxd cookie and reports its source.
// The value is run through NormalizeXoxd, which strips a "d=" cookie-name
// prefix and URL-decodes the percent-encoded form a browser stores — both of
// which are the right length but authenticate with the wrong bytes.
func ResolveXoxd() (Credential, error) {
	token, source, err := cliauth.ResolveToken(cliauth.TokenSource{
		EnvVar:          "SLACK_XOXD",
		KeychainService: config.KeychainXoxdService(),
		KeychainKey:     config.KeychainAccount(),
	})
	if err != nil {
		return Credential{}, err
	}
	clean, warnings := NormalizeXoxd(token)
	return Credential{Token: clean, Source: source, Warnings: warnings}, nil
}

// GetXoxc returns the cleaned Slack xoxc token.
// It checks the SLACK_XOXC environment variable first, falling back to the OS
// keychain via cli-kit/auth.ResolveToken.
func GetXoxc() (string, error) {
	cred, err := ResolveXoxc()
	return cred.Token, err
}

// GetXoxd returns the cleaned Slack xoxd cookie.
// It checks the SLACK_XOXD environment variable first, falling back to the OS
// keychain via cli-kit/auth.ResolveToken.
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
