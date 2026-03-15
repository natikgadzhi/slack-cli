package commands

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Slack authentication tokens",
}

var authCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if Slack tokens are configured and valid",
	RunE:  runAuthCheck,
}

var authSetXoxcCmd = &cobra.Command{
	Use:   "set-xoxc <token>",
	Short: "Store xoxc token in the macOS Keychain",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthSetXoxc,
}

var authSetXoxdCmd = &cobra.Command{
	Use:   "set-xoxd <token>",
	Short: "Store xoxd cookie in the macOS Keychain",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthSetXoxd,
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

	// Check xoxc token.
	xoxc, xoxcErr := auth.GetXoxc()
	if xoxcErr != nil {
		fmt.Fprintf(w, "[FAIL] xoxc: %v\n", xoxcErr)
	} else {
		checkToken(w, "xoxc", xoxc, "xoxc-")
	}

	// Check xoxd cookie.
	xoxd, xoxdErr := auth.GetXoxd()
	if xoxdErr != nil {
		fmt.Fprintf(w, "[FAIL] xoxd: %v\n", xoxdErr)
	} else {
		checkToken(w, "xoxd", xoxd, "xoxd-")
	}

	// If either token is missing, stop here.
	if xoxcErr != nil || xoxdErr != nil {
		return fmt.Errorf("one or more tokens are not configured")
	}

	// Try API call with the raw tokens.
	client := api.NewClient(xoxc, xoxd)
	result, err := client.Call("auth.test", nil)
	if err == nil {
		user, _ := result["user"].(string)
		team, _ := result["team"].(string)
		fmt.Fprintf(w, "[OK] authenticated as %s on %s\n", user, team)
		return nil
	}

	// auth.test failed. Print the error.
	if apiErr, ok := isAPIError(err); ok {
		fmt.Fprintf(w, "[FAIL] %s\n", apiErr.Message)
	} else {
		fmt.Fprintf(w, "[FAIL] %v\n", err)
	}

	// Try URL-decoded xoxd as fallback.
	xoxdDecoded, decodeErr := url.QueryUnescape(xoxd)
	if decodeErr == nil && xoxdDecoded != xoxd {
		clientDecoded := api.NewClient(xoxc, xoxdDecoded)
		resultDecoded, err := clientDecoded.Call("auth.test", nil)
		if err == nil {
			user, _ := resultDecoded["user"].(string)
			team, _ := resultDecoded["team"].(string)
			fmt.Fprintf(w, "[OK] authenticated (url-decoded xoxd) as %s on %s\n", user, team)
			return nil
		}
	}

	return fmt.Errorf("authentication failed")
}

// checkToken prints diagnostics about a single token to the given writer.
func checkToken(w *os.File, name, token, expectedPrefix string) {
	clean, warnings := auth.SanitizeToken(token)

	for _, warn := range warnings {
		fmt.Fprintf(w, "[WARN] %s: %s\n", name, warn)
	}

	// Show first 20 chars and length.
	preview := clean
	if len(preview) > 20 {
		preview = preview[:20]
	}
	fmt.Fprintf(w, "[INFO] %s: %s... (length %d)\n", name, preview, len(clean))

	// Check expected prefix.
	if !strings.HasPrefix(clean, expectedPrefix) {
		fmt.Fprintf(w, "[WARN] %s: expected prefix %q not found\n", name, expectedPrefix)
	} else {
		fmt.Fprintf(w, "[OK] %s: has expected prefix %q\n", name, expectedPrefix)
	}
}

// runAuthSetXoxc stores the xoxc token in the macOS Keychain.
func runAuthSetXoxc(cmd *cobra.Command, args []string) error {
	token, warnings := auth.SanitizeToken(args[0])
	for _, warn := range warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn)
	}
	service := config.KeychainXoxcService()
	if err := auth.KeychainSet(service, token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Stored xoxc token in keychain (service=%q, account=%q)\n",
		service, config.KeychainAccount())
	return nil
}

// runAuthSetXoxd stores the xoxd cookie in the macOS Keychain.
func runAuthSetXoxd(cmd *cobra.Command, args []string) error {
	token, warnings := auth.SanitizeToken(args[0])
	for _, warn := range warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn)
	}
	service := config.KeychainXoxdService()
	if err := auth.KeychainSet(service, token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Stored xoxd cookie in keychain (service=%q, account=%q)\n",
		service, config.KeychainAccount())
	return nil
}
