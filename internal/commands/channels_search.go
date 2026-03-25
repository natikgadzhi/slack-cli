package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// channelsSearchCmd searches for channels by name substring.
var channelsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search channels by name",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli channels search eng
  slack-cli channels search "product" --type public_channel,private_channel,mpim,im
  slack-cli channels search infra --include-archived`,
	RunE: runChannelsSearch,
}

func init() {
	channelsSearchCmd.Flags().IntP("limit", "n", 20, "Maximum number of results to return")
	channelsSearchCmd.Flags().String("type", "public_channel,private_channel,mpim,im", "Comma-separated conversation types to search")
	channelsSearchCmd.Flags().Bool("include-archived", false, "Include archived channels")
	channelsCmd.AddCommand(channelsSearchCmd)
}

// runChannelsSearch paginates conversations.list and filters channels whose
// name contains the query as a case-insensitive substring.
func runChannelsSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	types, _ := cmd.Flags().GetString("type")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")

	format := output.Resolve(cmd)

	// Set up client (no user resolver needed).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Show spinner while searching.
	spinner := progress.NewSpinner("Searching channels", format)
	spinner.Update(0)

	var matched []map[string]any
	var isPartial bool
	pageSize := 200

	params := map[string]string{
		"limit": strconv.Itoa(pageSize),
		"types": types,
	}

	if !includeArchived {
		params["exclude_archived"] = "true"
	}

	for {
		result, err := client.Call("conversations.list", params)
		if err != nil {
			spinner.Finish()

			// On rate limit with partial data, warn and render what we have.
			if _, ok := api.AsRateLimitError(err); ok && len(matched) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after searching %d channels; showing partial results", len(matched)), output.IsJSON(format))
				isPartial = true
				break
			}

			// For other CLI errors, print and exit with the right code.
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("searching channels: %w", err)
		}

		channels := api.ExtractItems(result, "channels")
		for _, ch := range channels {
			if matchesChannelName(ch, query) {
				matched = append(matched, ch)
				if len(matched) >= limit {
					break
				}
			}
		}

		// Stop if we've collected enough matches.
		if len(matched) >= limit {
			break
		}

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" {
			break
		}

		params["cursor"] = cursor
	}

	spinner.Finish()

	if len(matched) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no channels found")
		}
		return nil
	}

	// Truncate to the requested limit.
	if len(matched) > limit {
		matched = matched[:limit]
	}

	// Build clean result slice.
	results := make([]map[string]any, 0, len(matched))
	for _, ch := range matched {
		results = append(results, extractChannelFields(ch))
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
		renderChannelsTable(results)
	}

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d channels found (partial — rate limited).\n", len(results))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d channels found.\n", len(results))
		}
	}

	return nil
}

// matchesChannelName returns true if the channel name or name_normalized
// contains the query as a case-insensitive substring.
func matchesChannelName(ch map[string]any, query string) bool {
	query = strings.ToLower(query)
	name := strings.ToLower(getString(ch, "name"))
	nameNorm := strings.ToLower(getString(ch, "name_normalized"))
	return strings.Contains(name, query) || strings.Contains(nameNorm, query)
}
