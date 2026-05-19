package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	cliauth "github.com/natikgadzhi/cli-kit/auth"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Slack authentication tokens",
}

var authCheckCmd = &cobra.Command{
	Use:     "check",
	Short:   "Check if Slack tokens are configured and valid",
	Example: "  slack-cli auth check",
	RunE:    runAuthCheck,
}

var authSetXoxcCmd = &cobra.Command{
	Use:     "set-xoxc <token>",
	Short:   "Store xoxc token in the OS keychain",
	Args:    cobra.ExactArgs(1),
	Example: "  slack-cli auth set-xoxc xoxc-...",
	RunE:    storeXoxcToken,
}

var authSetXoxdCmd = &cobra.Command{
	Use:     "set-xoxd <token>",
	Short:   "Store xoxd cookie in the OS keychain",
	Args:    cobra.ExactArgs(1),
	Example: "  slack-cli auth set-xoxd xoxd-...",
	RunE:    storeXoxdToken,
}

func init() {
	authCmd.AddCommand(authCheckCmd)
	authCmd.AddCommand(authSetXoxcCmd)
	authCmd.AddCommand(authSetXoxdCmd)
	rootCmd.AddCommand(authCmd)
}

// runAuthCheck validates stored Slack tokens by calling auth.test.
// All output goes to stderr (not affected by -o flag).
func runAuthCheck(cmd *cobra.Command, args []string) error {
	w := os.Stderr

	// Resolve both credentials, reporting their source and any cleanup.
	xoxcCred, xoxcErr := auth.ResolveXoxc()
	if xoxcErr != nil {
		_, _ = fmt.Fprintf(w, "[FAIL] xoxc: %v\n", xoxcErr)
	} else {
		reportCredential(w, "xoxc", xoxcCred, "xoxc-")
	}

	xoxdCred, xoxdErr := auth.ResolveXoxd()
	if xoxdErr != nil {
		_, _ = fmt.Fprintf(w, "[FAIL] xoxd: %v\n", xoxdErr)
	} else {
		reportCredential(w, "xoxd", xoxdCred, "xoxd-")
	}

	// If either token is missing, stop here.
	if xoxcErr != nil || xoxdErr != nil {
		return fmt.Errorf("one or more tokens are not configured")
	}

	// Try the API call with the cleaned tokens. Because ResolveXoxd already
	// strips the "d=" prefix and URL-decodes the cookie, this uses the same
	// bytes the real commands use — no separate decode fallback needed.
	client := api.NewClient(xoxcCred.Token, xoxdCred.Token)
	result, err := client.Call("auth.test", nil)
	if err == nil {
		user, _ := result["user"].(string)
		team, _ := result["team"].(string)
		_, _ = fmt.Fprintf(w, "[OK] authenticated as %s on %s\n", user, team)
		return nil
	}

	// auth.test failed. Surface the Slack error code, explain what it means,
	// and print a checklist of likely fixes.
	code := slackErrorCode(err)
	_, _ = fmt.Fprintf(w, "[FAIL] auth.test rejected the credentials: %s\n", code)
	if explanation := explainAuthError(code); explanation != "" {
		_, _ = fmt.Fprintf(w, "       %s\n", explanation)
	}
	_, _ = fmt.Fprintln(w, "\nWhat to check:")
	for _, item := range authChecklist(xoxcCred, xoxdCred) {
		_, _ = fmt.Fprintf(w, "  • %s\n", item)
	}
	return fmt.Errorf("authentication failed")
}

// reportCredential prints diagnostics about a single resolved credential:
// any cleanup warnings, a masked preview with length and source, and whether
// it carries the expected prefix.
func reportCredential(w io.Writer, name string, cred auth.Credential, expectedPrefix string) {
	for _, warn := range cred.Warnings {
		_, _ = fmt.Fprintf(w, "[WARN] %s: %s\n", name, warn)
	}

	masked := cliauth.MaskToken(cred.Token)
	_, _ = fmt.Fprintf(w, "[INFO] %s: %s (length %d, from %s)\n",
		name, masked, len(cred.Token), sourceLabel(cred.Source))

	if !strings.HasPrefix(cred.Token, expectedPrefix) {
		_, _ = fmt.Fprintf(w, "[WARN] %s: expected prefix %q not found\n", name, expectedPrefix)
	} else {
		_, _ = fmt.Fprintf(w, "[OK] %s: has expected prefix %q\n", name, expectedPrefix)
	}
}

// sourceLabel renders a token source for humans, naming the keychain account
// so users can tell whether the CLI is reading the entry they wrote.
func sourceLabel(source string) string {
	switch source {
	case cliauth.SourceEnvironment:
		return "environment variable"
	case cliauth.SourceKeychain:
		return fmt.Sprintf("keychain (account %q)", config.KeychainAccount())
	default:
		return source
	}
}

// slackErrorCode extracts the Slack error string (e.g. "invalid_auth") from an
// error returned by the API client.
func slackErrorCode(err error) string {
	if cliErr, ok := api.AsCLIError(err); ok {
		return strings.TrimSpace(strings.TrimPrefix(cliErr.Message, "slack api:"))
	}
	if apiErr, ok := api.AsAPIError(err); ok {
		return apiErr.Message
	}
	return err.Error()
}

// explainAuthError maps a Slack error code to a plain-English explanation.
// Returns "" for codes we have no specific guidance for.
func explainAuthError(code string) string {
	switch code {
	case "invalid_auth":
		return "The token/cookie pair was rejected. Most often the xoxc token and xoxd cookie were copied from different browser sessions, or the session has since been signed out."
	case "not_authed":
		return "No session was presented — the xoxd cookie is missing, empty, or wasn't sent."
	case "token_revoked":
		return "This token was revoked (e.g. you signed out of Slack in that browser). Grab a fresh xoxc/xoxd from a current session."
	case "token_expired":
		return "The token has expired. Grab a fresh xoxc token and d cookie from a current browser session."
	case "account_inactive":
		return "The account behind this token is deactivated on the workspace."
	case "no_permission", "missing_scope":
		return "Authenticated, but this token isn't allowed to call that API."
	case "user_removed_from_team":
		return "This user is no longer a member of the workspace."
	default:
		return ""
	}
}

// authChecklist returns context-aware troubleshooting steps for a failed
// auth.test, given the credentials that were tried.
func authChecklist(xoxc, xoxd auth.Credential) []string {
	items := []string{
		"xoxc and xoxd must be copied from the SAME logged-in Slack browser tab, captured at the same time.",
		"Re-copy the d cookie from DevTools → Application → Cookies → your Slack domain (the cookie named \"d\").",
		"Make sure you're still signed into that workspace in the browser — signing out invalidates the token.",
	}
	if xoxc.Source != xoxd.Source {
		items = append(items, fmt.Sprintf(
			"Heads up: xoxc came from the %s but xoxd came from the %s — a stale mix of the two is a common cause. Refresh both from the same place.",
			sourceLabel(xoxc.Source), sourceLabel(xoxd.Source)))
	}
	return items
}

// storeXoxcToken sanitizes and stores the xoxc token in the OS keychain.
func storeXoxcToken(cmd *cobra.Command, args []string) error {
	token, warnings := auth.SanitizeToken(args[0])
	for _, warn := range warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn)
	}
	if err := auth.StoreXoxc(token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Stored xoxc token in keychain (service=%q, account=%q)\n",
		config.KeychainXoxcService(), config.KeychainAccount())
	return nil
}

// storeXoxdToken normalizes and stores the xoxd cookie in the OS keychain.
// NormalizeXoxd strips a "d=" prefix and URL-decodes the value so the stored
// cookie matches what the API expects.
func storeXoxdToken(cmd *cobra.Command, args []string) error {
	token, warnings := auth.NormalizeXoxd(args[0])
	for _, warn := range warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn)
	}
	if err := auth.StoreXoxd(token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Stored xoxd cookie in keychain (service=%q, account=%q)\n",
		config.KeychainXoxdService(), config.KeychainAccount())
	return nil
}
