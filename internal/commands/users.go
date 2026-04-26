package commands

import (
	"fmt"
	"os"
	"strconv"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "List workspace users",
	Args:  cobra.NoArgs,
	Example: `  slack-cli users
  slack-cli users --limit 50
  slack-cli users --include-bots --include-deactivated
  slack-cli users -o json | jq '.[] | select(.email | contains("@example.com"))'`,
	RunE: runUsers,
}

func init() {
	usersCmd.Flags().IntP("limit", "n", 100, "Maximum number of users to list")
	usersCmd.Flags().Bool("include-bots", false, "Include bot users")
	usersCmd.Flags().Bool("include-deactivated", false, "Include deactivated users")
	rootCmd.AddCommand(usersCmd)
}

// runUsers fetches workspace members via users.list with cursor-based
// pagination and renders them as a table or JSON.
func runUsers(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	includeBots, _ := cmd.Flags().GetBool("include-bots")
	includeDeactivated, _ := cmd.Flags().GetBool("include-deactivated")

	format := output.Resolve(cmd)

	// Set up client (no user resolver needed).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Fetch users with progress indicator.
	prog := progress.NewCounter("Fetching users", format)

	var allUsers []map[string]any
	var isPartial bool
	pageSize := 200

	params := map[string]string{
		"limit": strconv.Itoa(pageSize),
	}

	for {
		prog.Update(len(allUsers))

		result, err := client.Call("users.list", params)
		if err != nil {
			prog.Finish()

			// On rate limit with partial data, warn and render what we have.
			if _, ok := api.AsRateLimitError(err); ok && len(allUsers) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after fetching %d users; showing partial results", len(allUsers)), output.IsJSON(format))
				isPartial = true
				break
			}

			// For other CLI errors, print and exit with the right code.
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("fetching users: %w", err)
		}

		members := api.ExtractItems(result, "members")
		for _, member := range members {
			if filterUser(member, includeBots, includeDeactivated) {
				allUsers = append(allUsers, member)
			}
		}

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(allUsers) >= limit {
			break
		}

		params["cursor"] = cursor
	}

	prog.Finish()

	if len(allUsers) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no users found")
		}
		return nil
	}

	// Truncate to the requested limit.
	if len(allUsers) > limit {
		allUsers = allUsers[:limit]
	}

	// Build clean result slice.
	results := make([]map[string]any, 0, len(allUsers))
	for _, member := range allUsers {
		results = append(results, extractUserFields(member))
	}

	// Render output.
	if output.IsJSON(format) {
		if isPartial {
			pr := clierrors.NewPartialResult(results, "rate limited: results may be incomplete")
			if err := output.PrintJSON(pr); err != nil {
				return err
			}
		} else {
			if err := output.PrintJSON(results); err != nil {
				return err
			}
		}
	} else {
		renderUsersTable(results)
	}

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d users fetched (partial — rate limited).\n", len(results))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d users fetched.\n", len(results))
		}
	}

	return nil
}

// filterUser returns true if the user should be included in the results.
// It always excludes USLACKBOT. Bots and deactivated users are excluded
// unless the corresponding include flags are set.
func filterUser(member map[string]any, includeBots, includeDeactivated bool) bool {
	// Always exclude USLACKBOT — it's a system pseudo-user.
	if id, ok := member["id"].(string); ok && id == "USLACKBOT" {
		return false
	}

	// Exclude bots unless --include-bots is set.
	if !includeBots {
		if isBot, ok := member["is_bot"].(bool); ok && isBot {
			return false
		}
	}

	// Exclude deactivated users unless --include-deactivated is set.
	if !includeDeactivated {
		if deleted, ok := member["deleted"].(bool); ok && deleted {
			return false
		}
	}

	return true
}

// extractUserFields extracts the display fields from a raw Slack user object.
func extractUserFields(member map[string]any) map[string]any {
	r := map[string]any{
		"id":        getString(member, "id"),
		"name":      getString(member, "name"),
		"real_name": getString(member, "real_name"),
		"email":     "",
	}

	// Email lives under profile.email.
	if profile, ok := member["profile"].(map[string]any); ok {
		if email, ok := profile["email"].(string); ok {
			r["email"] = email
		}
	}

	return r
}

// getString safely extracts a string field from a map, returning "" if missing.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// renderUsersTable renders users as a table to stdout.
func renderUsersTable(users []map[string]any) {
	t := table.New()
	t.Header("ID", "NAME", "REAL NAME", "EMAIL")
	for _, u := range users {
		t.Row(getString(u, "id"), getString(u, "name"), getString(u, "real_name"), getString(u, "email"))
	}
	_ = t.Flush()
}
