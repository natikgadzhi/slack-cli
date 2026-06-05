package commands

import (
	"fmt"
	"os"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// usersSearchCmd searches for users by name, handle, or email using Slack's
// internal users.search API.
var usersSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search users by name or handle",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli users search alice
  slack-cli users search "alice smith"
  slack-cli users search alice --limit 10
  slack-cli users search alice -o json`,
	RunE: runUsersSearch,
}

func init() {
	usersSearchCmd.Flags().IntP("limit", "n", 20, "Maximum number of results")
	usersCmd.AddCommand(usersSearchCmd)
}

// runUsersSearch calls Slack's users.search endpoint and renders matching users
// in the same columnar table format as "users list".
func runUsersSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	prog := progress.NewSpinner("Searching users", format)

	result, err := client.Call("users.search", map[string]string{
		"query": query,
	})
	if err != nil {
		prog.Finish()

		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("searching users: %w", err)
	}

	prog.Finish()

	// The users.search endpoint returns results under a "results" key.
	rawResults, ok := result["results"].([]any)
	if !ok || len(rawResults) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no users found")
		}
		return nil
	}

	// Convert and truncate to limit.
	var users []map[string]any
	for _, r := range rawResults {
		if len(users) >= limit {
			break
		}
		if u, ok := r.(map[string]any); ok {
			users = append(users, u)
		}
	}

	if len(users) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no users found")
		}
		return nil
	}

	// Build clean result slice with the same fields as "users list".
	results := make([]map[string]any, 0, len(users))
	for _, u := range users {
		results = append(results, extractUserFields(u))
	}

	if output.IsJSON(format) {
		return output.PrintJSON(results)
	}

	renderUsersTable(results)

	if !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "Done. %d users found.\n", len(results))
	}

	return nil
}
