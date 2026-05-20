package commands

import (
	"fmt"
	"io"
	"net/url"
	"os"

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
		if auth.LooksURLEncoded(xoxdCred.Token) {
			_, _ = fmt.Fprintln(w, "[INFO] xoxd: using URL-encoded wire form")
		} else {
			_, _ = fmt.Fprintln(w, "[INFO] xoxd: using raw wire form")
		}
	}

	// If either token is missing, stop here.
	if xoxcErr != nil || xoxdErr != nil {
		return fmt.Errorf("one or more tokens are not configured")
	}

	// Primary attempt. api.Client.Call wire-normalizes xoxd to URL-encoded
	// form before sending, so raw stored values "just work" here.
	client := api.NewClient(xoxcCred.Token, xoxdCred.Token)
	if result, err := client.Call("auth.test", nil); err == nil {
		user, _ := result["user"].(string)
		team, _ := result["team"].(string)
		_, _ = fmt.Fprintf(w, "[OK] authenticated as %s on %s\n", user, team)
		// Opportunistically clean up the keychain so future reads of the
		// stored value match the wire form. No-op if xoxd is already
		// canonical or if SLACK_XOXD is set via env var.
		persistCanonicalXoxd(w, xoxdCred)
		return nil
	} else if !tryFallbacksAndReport(w, xoxcCred, xoxdCred, err) {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// persistCanonicalXoxd writes the canonical (URL-encoded) form of stored xoxd
// back into the keychain when it differs from what's already there, so
// future reads return the wire form directly. No-op if:
//   - stored is already canonical (auth.NormalizeXoxd leaves it unchanged), or
//   - the value came from an env var (the env var owns what is used).
//
// Failures to write are non-fatal — the wire normalization in client.Call
// has already made the request work; this is just hygiene.
func persistCanonicalXoxd(w io.Writer, stored auth.Credential) {
	if stored.Source != cliauth.SourceKeychain {
		return
	}
	canonical, _ := auth.SanitizeXoxd(stored.RawToken)
	if canonical == stored.RawToken {
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
func tryFallbacksAndReport(w io.Writer, xoxc, xoxd auth.Credential, primaryErr error) bool {
	code := api.SlackCode(primaryErr)

	// For credential-shaped errors, try the opposite encoding of what's
	// stored. With wire-level normalization in api.Client.Call this branch
	// mainly catches the rare double-encoded case (e.g. `%252B` instead of
	// `%2B`) — the common raw-xoxd case is handled at the transport layer
	// before we get here.
	if code == "invalid_auth" || code == "not_authed" || code == "invalid_cookie" || code == "no_auth_in_cookie" {
		for _, fb := range xoxdFallbacks(xoxd.Token) {
			client := api.NewClient(xoxc.Token, fb.value)
			result, err := client.Call("auth.test", nil)
			if err != nil {
				continue
			}
			user, _ := result["user"].(string)
			team, _ := result["team"].(string)
			_, _ = fmt.Fprintf(w, "[OK] authenticated using %s xoxd as %s on %s\n", fb.label, user, team)
			// Persist the working form so subsequent calls hit the primary
			// path. Skipped if SLACK_XOXD is set via env var (env owns it).
			if xoxd.Source == cliauth.SourceKeychain {
				working, _ := auth.SanitizeXoxd(fb.value)
				if err := auth.StoreXoxd(working); err == nil {
					_, _ = fmt.Fprintln(w, "[INFO] saved the working xoxd form to keychain — future calls will use it directly")
				}
			}
			return true
		}
	}

	// No fallback succeeded — print the primary error with explanation + hint.
	printPrimaryFailure(w, primaryErr, code)
	if xoxc.Source != "" && xoxd.Source != "" && xoxc.Source != xoxd.Source {
		_, _ = fmt.Fprintf(w, "[HINT] xoxc came from %s but xoxd came from %s; refresh both from the same browser session and source.\n",
			sourceLabel(xoxc.Source), sourceLabel(xoxd.Source))
	}
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
		if decoded, err := url.PathUnescape(xoxd); err == nil && decoded != "" && decoded != xoxd {
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
			"Re-extract xoxc and xoxd from the same fresh browser session. If one came from env and the other from keychain, refresh both — stale mixes are the usual foot-gun."
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

// reportCredential prints a masked token preview, where it came from, and any
// cleanup that was applied before the API call.
func reportCredential(w io.Writer, name string, cred auth.Credential, expectedPrefix string) {
	for _, warn := range cred.Warnings {
		_, _ = fmt.Fprintf(w, "[WARN] %s: %s\n", name, warn)
	}

	masked := cliauth.MaskToken(cred.Token)
	_, _ = fmt.Fprintf(w, "[INFO] %s: %s (length %d, from %s)\n",
		name, masked, len(cred.Token), sourceLabel(cred.Source))

	if len(cred.Token) < len(expectedPrefix) || cred.Token[:len(expectedPrefix)] != expectedPrefix {
		_, _ = fmt.Fprintf(w, "[WARN] %s: expected prefix %q not found\n", name, expectedPrefix)
	} else {
		_, _ = fmt.Fprintf(w, "[OK] %s: has expected prefix %q\n", name, expectedPrefix)
	}
}

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
