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
//
// On a failed auth.test, runAuthCheck:
//   - extracts the Slack error code via api.SlackCode,
//   - if the code points at credential trouble (invalid_auth, not_authed,
//     invalid_cookie), retries auth.test with the alternate xoxd encoding
//     (encode-or-decode, whichever the stored value isn't already in) and
//     reports which form worked plus how to make the fix permanent,
//   - otherwise prints a plain-English explanation + actionable hint via
//     slackAuthErrorHint.
func runAuthCheck(cmd *cobra.Command, args []string) error {
	w := os.Stderr

	// Check xoxc token.
	xoxc, xoxcErr := auth.GetXoxc()
	if xoxcErr != nil {
		_, _ = fmt.Fprintf(w, "[FAIL] xoxc: %v\n", xoxcErr)
	} else {
		checkToken(w, "xoxc", xoxc, "xoxc-")
	}

	// Check xoxd cookie. Include an encoding-state diagnostic.
	xoxd, xoxdErr := auth.GetXoxd()
	if xoxdErr != nil {
		_, _ = fmt.Fprintf(w, "[FAIL] xoxd: %v\n", xoxdErr)
	} else {
		checkToken(w, "xoxd", xoxd, "xoxd-")
		if auth.LooksURLEncoded(xoxd) {
			_, _ = fmt.Fprintln(w, "[INFO] xoxd: stored as URL-encoded")
		} else {
			_, _ = fmt.Fprintln(w, "[INFO] xoxd: stored as raw — will be auto-encoded on the wire")
		}
	}

	// If either token is missing, stop here.
	if xoxcErr != nil || xoxdErr != nil {
		return fmt.Errorf("one or more tokens are not configured")
	}

	// Primary attempt. api.Client.Call wire-normalizes xoxd to URL-encoded
	// form before sending, so raw stored values "just work" here.
	client := api.NewClient(xoxc, xoxd)
	if result, err := client.Call("auth.test", nil); err == nil {
		user, _ := result["user"].(string)
		team, _ := result["team"].(string)
		_, _ = fmt.Fprintf(w, "[OK] authenticated as %s on %s\n", user, team)
		// Opportunistically clean up the keychain so future reads of the
		// stored value match the wire form. No-op if xoxd is already
		// canonical or if SLACK_XOXD is set via env var.
		persistCanonicalXoxd(w, xoxd)
		return nil
	} else if !tryFallbacksAndReport(w, xoxc, xoxd, err) {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// persistCanonicalXoxd writes the canonical (URL-encoded) form of stored
// back into the keychain when it differs from what's already there, so
// future reads return the wire form directly. No-op if:
//   - stored is already canonical (auth.NormalizeXoxd leaves it unchanged), or
//   - SLACK_XOXD is set via env var (the env var owns the value; writing to
//     keychain would silently disagree with what's actually used on the wire).
//
// Failures to write are non-fatal — the wire normalization in client.Call
// has already made the request work; this is just hygiene.
func persistCanonicalXoxd(w io.Writer, stored string) {
	if os.Getenv("SLACK_XOXD") != "" {
		return
	}
	canonical, _ := auth.NormalizeXoxd(stored)
	if canonical == stored {
		return
	}
	if err := auth.StoreXoxd(canonical); err != nil {
		return
	}
	_, _ = fmt.Fprintln(w, "[INFO] normalized stored xoxd to URL-encoded form in keychain")
}

// tryFallbacksAndReport handles a failed auth.test. It returns true when a
// fallback xoxd encoding succeeds (so the caller can return success), and
// false when all attempts have been exhausted. Either way it has already
// written the user-facing diagnostic to w.
func tryFallbacksAndReport(w io.Writer, xoxc, xoxd string, primaryErr error) bool {
	code := api.SlackCode(primaryErr)

	// For credential-shaped errors, try the opposite encoding of what's
	// stored. With wire-level normalization in api.Client.Call this branch
	// mainly catches the rare double-encoded case (e.g. `%252B` instead of
	// `%2B`) — the common raw-xoxd case is handled at the transport layer
	// before we get here.
	if code == "invalid_auth" || code == "not_authed" || code == "invalid_cookie" || code == "no_auth_in_cookie" {
		for _, fb := range xoxdFallbacks(xoxd) {
			client := api.NewClient(xoxc, fb.value)
			result, err := client.Call("auth.test", nil)
			if err != nil {
				continue
			}
			user, _ := result["user"].(string)
			team, _ := result["team"].(string)
			_, _ = fmt.Fprintf(w, "[OK] authenticated using %s xoxd as %s on %s\n", fb.label, user, team)
			// Persist the working form so subsequent calls hit the primary
			// path. Skipped if SLACK_XOXD is set via env var (env owns it).
			if os.Getenv("SLACK_XOXD") == "" {
				if err := auth.StoreXoxd(fb.value); err == nil {
					_, _ = fmt.Fprintln(w, "[INFO] saved the working xoxd form to keychain — future calls will use it directly")
				}
			}
			return true
		}
	}

	// No fallback succeeded — print the primary error with explanation + hint.
	printPrimaryFailure(w, primaryErr, code)
	return false
}

// xoxdFallback is one alternate encoding of an xoxd cookie value that
// auth check will try after the primary auth.test fails.
type xoxdFallback struct {
	label string // human-readable, e.g. "URL-encoded" or "URL-decoded"
	value string // the xoxd value to send
}

// xoxdFallbacks returns the alternate-encoding attempts worth trying for a
// stored xoxd. Empty alternates and ones equal to the original are filtered
// so we never retry the same value.
func xoxdFallbacks(xoxd string) []xoxdFallback {
	out := make([]xoxdFallback, 0, 2)

	if !auth.LooksURLEncoded(xoxd) {
		if encoded := url.QueryEscape(xoxd); encoded != "" && encoded != xoxd {
			out = append(out, xoxdFallback{label: "URL-encoded", value: encoded})
		}
	} else {
		if decoded, err := url.QueryUnescape(xoxd); err == nil && decoded != "" && decoded != xoxd {
			out = append(out, xoxdFallback{label: "URL-decoded", value: decoded})
		}
	}
	return out
}

// printPrimaryFailure renders the primary auth.test error in human terms.
// code is api.SlackCode(err) — "" when the error wasn't a Slack-level one.
func printPrimaryFailure(w io.Writer, err error, code string) {
	switch {
	case code != "":
		explanation, remedy := slackAuthErrorHint(code)
		if explanation == "" {
			_, _ = fmt.Fprintf(w, "[FAIL] slack api: %s\n", code)
		} else {
			_, _ = fmt.Fprintf(w, "[FAIL] slack api: %s — %s\n", code, explanation)
		}
		if remedy != "" {
			_, _ = fmt.Fprintf(w, "[HINT] %s\n", remedy)
		}
	default:
		if cliErr, ok := api.AsCLIError(err); ok {
			_, _ = fmt.Fprintf(w, "[FAIL] %s\n", cliErr.Message)
		} else if apiErr, ok := api.AsAPIError(err); ok {
			_, _ = fmt.Fprintf(w, "[FAIL] %s\n", apiErr.Message)
		} else {
			_, _ = fmt.Fprintf(w, "[FAIL] %v\n", err)
		}
	}
}

// slackAuthErrorHint maps a Slack error code (e.g. "invalid_auth") to a
// plain-English explanation and an actionable remedy. The remedy is what
// the user should do next; it's printed as a [HINT] line. Returns "", ""
// when the code is unrecognized so the caller can fall back to the raw
// error message.
func slackAuthErrorHint(code string) (explanation, remedy string) {
	switch code {
	case "invalid_auth", "not_authed":
		return "Slack rejected the credentials.",
			"Most common cause: xoxd was pasted in raw form (not URL-encoded). " +
				"Re-extract xoxc and xoxd from a fresh browser session, then re-run " +
				"`slack-cli auth set-xoxc <token>` and `slack-cli auth set-xoxd <cookie>` " +
				"— set-xoxd auto-encodes the cookie now."
	case "invalid_cookie", "no_auth_in_cookie":
		return "Slack didn't accept the d cookie.",
			"Re-extract xoxd from your browser session. set-xoxd accepts either the " +
				"URL-encoded form (with %XX escapes) or the raw form — raw values are " +
				"auto-encoded for you."
	case "token_expired":
		return "Your token has expired.",
			"Re-extract a fresh xoxc from your browser session and re-run `slack-cli auth set-xoxc`."
	case "token_revoked":
		return "Your token was revoked.",
			"Sign in to Slack again in your browser, then re-extract both xoxc and xoxd."
	case "account_inactive":
		return "Your account is inactive on this workspace.",
			"Contact your workspace admin, or switch to a workspace where your account is active."
	case "missing_scope", "no_permission":
		return "Your session lacks the required scope for this call.",
			"Make sure you copied xoxc from a fully logged-in regular browser session " +
				"(not a guest or limited-access session)."
	case "user_is_bot":
		return "This is a bot (xoxb) token — slack-cli needs a user (xoxc) token.",
			"Extract an xoxc token from a regular user browser session, not a bot integration."
	case "org_login_with_sso":
		return "This workspace requires SSO login.",
			"Complete SSO sign-in in your browser first, then re-extract xoxc and xoxd."
	default:
		return "", ""
	}
}

// checkToken prints diagnostics about a single token to the given writer.
// Token values are masked using cli-kit/auth.MaskToken.
func checkToken(w io.Writer, name, token, expectedPrefix string) {
	clean, warnings := auth.SanitizeToken(token)

	for _, warn := range warnings {
		_, _ = fmt.Fprintf(w, "[WARN] %s: %s\n", name, warn)
	}

	// Show masked token preview using cli-kit/auth.MaskToken.
	masked := cliauth.MaskToken(clean)
	_, _ = fmt.Fprintf(w, "[INFO] %s: %s (length %d)\n", name, masked, len(clean))

	// Check expected prefix.
	if !strings.HasPrefix(clean, expectedPrefix) {
		_, _ = fmt.Fprintf(w, "[WARN] %s: expected prefix %q not found\n", name, expectedPrefix)
	} else {
		_, _ = fmt.Fprintf(w, "[OK] %s: has expected prefix %q\n", name, expectedPrefix)
	}
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

// storeXoxdToken sanitizes, normalizes, and stores the xoxd cookie in the
// OS keychain. The normalization step auto-encodes raw (non-URL-encoded)
// values so the stored form is always what Slack expects on the wire.
func storeXoxdToken(cmd *cobra.Command, args []string) error {
	token, warnings := auth.SanitizeXoxd(args[0])
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
