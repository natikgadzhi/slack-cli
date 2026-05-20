package commands

import (
	"fmt"
	"io"
	"net/url"
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

// storeXoxdToken normalizes, verifies, and stores the xoxd cookie.
//
// The browser's "d" cookie is percent-encoded and Slack expects that exact
// form, but users sometimes paste the decoded value (right length, wrong
// bytes). Rather than guess, we verify against auth.test: try the pasted form
// and its opposite encoding, and store whichever Slack accepts. Verification
// needs an xoxc token already present; without one (or offline), we store the
// value as pasted and say so.
func storeXoxdToken(cmd *cobra.Command, args []string) error {
	token, warnings := auth.NormalizeXoxd(args[0])
	for _, warn := range warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn)
	}

	toStore := token
	if xoxc, err := auth.GetXoxc(); err == nil && xoxc != "" {
		if working, ok := probeXoxd(xoxc, xoxdCandidates(token)); ok {
			toStore = working
			if working == token {
				fmt.Fprintf(os.Stderr, "[OK] verified cookie against auth.test\n")
			} else {
				fmt.Fprintf(os.Stderr,
					"[INFO] the pasted cookie authenticated only after re-encoding; storing the %s\n",
					encodingLabel(working))
			}
		} else {
			fmt.Fprintf(os.Stderr,
				"[WARN] auth.test rejected this cookie in both encodings — it may be invalid/expired, "+
					"or the xoxc token may be stale. Storing it as pasted; run 'slack-cli auth check' to diagnose.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr,
			"[INFO] no xoxc token found yet, so the cookie can't be verified; storing as pasted. "+
				"Set xoxc, then run 'slack-cli auth check'.\n")
	}

	if err := auth.StoreXoxd(toStore); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Stored xoxd cookie in keychain (service=%q, account=%q)\n",
		config.KeychainXoxdService(), config.KeychainAccount())
	return nil
}

// xoxdCandidates returns the distinct encodings of an xoxd cookie to try against
// auth.test: the value as given, plus its opposite encoding. If the value looks
// percent-encoded (contains "%") we also try the decoded form; otherwise we also
// try the percent-encoded form.
//
// The decoded xoxd value is "xoxd-" + base64 (alphabet A-Za-z0-9+/=), so
// url.QueryEscape reproduces exactly the browser's encoding here: it leaves the
// unreserved characters alone and escapes +, /, and = to %2B, %2F, %3D.
func xoxdCandidates(value string) []string {
	candidates := []string{value}

	var alt string
	if strings.Contains(value, "%") {
		// PathUnescape (not QueryUnescape) so a literal "+" is left intact.
		if decoded, err := url.PathUnescape(value); err == nil {
			alt = decoded
		}
	} else {
		alt = url.QueryEscape(value)
	}

	if alt != "" && alt != value {
		candidates = append(candidates, alt)
	}
	return candidates
}

// probeXoxd calls auth.test with each candidate cookie and returns the first one
// Slack accepts. The xoxc token is held constant.
func probeXoxd(xoxc string, candidates []string) (string, bool) {
	for _, cand := range candidates {
		client := api.NewClient(xoxc, cand)
		if _, err := client.Call("auth.test", nil); err == nil {
			return cand, true
		}
	}
	return "", false
}

// encodingLabel describes which wire form a cookie value is in, for messages.
func encodingLabel(value string) string {
	if strings.Contains(value, "%") {
		return "URL-encoded form"
	}
	return "raw (decoded) form"
}
