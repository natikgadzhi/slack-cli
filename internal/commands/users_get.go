package commands

import (
	"fmt"
	"os"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// usersGetCmd looks up a single user by handle or user ID and displays
// their full profile.
var usersGetCmd = &cobra.Command{
	Use:   "get <handle|id>",
	Short: "Get a user's profile",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli users get alice
  slack-cli users get U12345678
  slack-cli users get @alice
  slack-cli users get alice -o json`,
	RunE: runUsersGet,
}

func init() {
	usersGetCmd.ValidArgsFunction = completeUserHandlesPositional
	usersCmd.AddCommand(usersGetCmd)
}

// completeUserHandlesPositional completes the first positional argument of
// "users get" with user handles.
func completeUserHandlesPositional(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeUserHandles(nil, nil, toComplete)
}

// runUsersGet resolves a handle or user ID to a full user profile and renders
// it as a key-value table (table output) or a single JSON object.
func runUsersGet(cmd *cobra.Command, args []string) error {
	handleOrID := args[0]

	// Strip a leading "@" if present (e.g. "@alice" -> "alice").
	handleOrID = strings.TrimPrefix(handleOrID, "@")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	prog := progress.NewSpinner("Looking up user", format)

	// Determine if the input looks like a user ID (starts with U or W and is
	// alphanumeric) or a handle.
	var userID string
	if looksLikeUserID(handleOrID) {
		userID = handleOrID
	} else {
		// Search for the user by handle.
		userID, err = searchUserIDByHandle(client, handleOrID)
		if err != nil {
			prog.Finish()
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("searching for user %q: %w", handleOrID, err)
		}
		if userID == "" {
			prog.Finish()
			return fmt.Errorf("no user found matching %q", handleOrID)
		}
	}

	// Fetch the full user profile via users.info.
	result, err := client.Call("users.info", map[string]string{"user": userID})
	if err != nil {
		prog.Finish()
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching user info: %w", err)
	}

	prog.Finish()

	user, ok := result["user"].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected response: missing user object")
	}

	profile := extractFullUserProfile(user)

	if output.IsJSON(format) {
		return output.PrintJSON(profile)
	}

	renderUserProfileTable(profile)
	return nil
}

// searchUserIDByHandle uses the users.search internal API to find a user by
// their handle/name and returns their user ID. Returns "" if no match is found.
func searchUserIDByHandle(client *api.Client, handle string) (string, error) {
	result, err := client.Call("users.search", map[string]string{
		"query": handle,
	})
	if err != nil {
		return "", err
	}

	results, ok := result["results"].([]any)
	if !ok || len(results) == 0 {
		return "", nil
	}

	// Look for an exact handle match first.
	for _, r := range results {
		u, ok := r.(map[string]any)
		if !ok {
			continue
		}
		name := getString(u, "name")
		if strings.EqualFold(name, handle) {
			return getString(u, "id"), nil
		}
	}

	// If no exact match, use the first result.
	if first, ok := results[0].(map[string]any); ok {
		return getString(first, "id"), nil
	}

	return "", nil
}

// extractFullUserProfile extracts all displayable fields from a raw Slack
// user object returned by users.info.
func extractFullUserProfile(user map[string]any) map[string]any {
	profile, _ := user["profile"].(map[string]any)

	r := map[string]any{
		"id":           getString(user, "id"),
		"name":         getString(user, "name"),
		"real_name":    getString(user, "real_name"),
		"display_name": getProfileString(profile, "display_name"),
		"email":        getProfileString(profile, "email"),
		"title":        getProfileString(profile, "title"),
		"status_text":  getProfileString(profile, "status_text"),
		"status_emoji": getProfileString(profile, "status_emoji"),
		"timezone":     getString(user, "tz"),
		"phone":        getProfileString(profile, "phone"),
		"is_admin":     getBool(user, "is_admin"),
		"is_owner":     getBool(user, "is_owner"),
		"is_bot":       getBool(user, "is_bot"),
		"deleted":      getBool(user, "deleted"),
	}

	return r
}

// getProfileString extracts a string from a profile map, returning "" if the
// profile is nil or the key is missing.
func getProfileString(profile map[string]any, key string) string {
	if profile == nil {
		return ""
	}
	return getString(profile, key)
}

// renderUserProfileTable renders a single user's profile as a two-column
// key-value table.
func renderUserProfileTable(profile map[string]any) {
	t := table.New()
	t.Header("KEY", "VALUE")

	// Display fields in a logical order.
	fields := []struct {
		key   string
		label string
	}{
		{"id", "ID"},
		{"name", "Name"},
		{"real_name", "Real Name"},
		{"display_name", "Display Name"},
		{"email", "Email"},
		{"title", "Title"},
		{"status_text", "Status"},
		{"status_emoji", "Status Emoji"},
		{"timezone", "Timezone"},
		{"phone", "Phone"},
		{"is_admin", "Admin"},
		{"is_owner", "Owner"},
		{"is_bot", "Bot"},
		{"deleted", "Deactivated"},
	}

	for _, f := range fields {
		val := profile[f.key]
		t.Row(f.label, fmt.Sprintf("%v", val))
	}

	_ = t.Flush()
}
