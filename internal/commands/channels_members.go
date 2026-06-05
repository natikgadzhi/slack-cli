package commands

import (
	"fmt"
	"io"
	"os"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/channels"
)

// channelsMembersCmd lists members of a Slack channel.
var channelsMembersCmd = &cobra.Command{
	Use:   "members <name|id>",
	Short: "List members of a channel",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli channels members general
  slack-cli channels members C12345678
  slack-cli channels members general --limit 50
  slack-cli channels members general --include-bots --include-deactivated
  slack-cli channels members general -o json`,
	RunE: runChannelsMembers,
}

func init() {
	channelsMembersCmd.Flags().IntP("limit", "n", 100, "Maximum number of members to return")
	channelsMembersCmd.Flags().Bool("include-bots", false, "Include bot users")
	channelsMembersCmd.Flags().Bool("include-deactivated", false, "Include deactivated users")

	// Complete the channel name/ID argument from the user's channel list.
	channelsMembersCmd.ValidArgsFunction = completeChannelNames

	channelsCmd.AddCommand(channelsMembersCmd)
}

// runChannelsMembers fetches the member list for a channel via
// conversations.members with cursor-based pagination, resolves each user ID
// to a profile via users.info, and renders the result as a table or JSON.
func runChannelsMembers(cmd *cobra.Command, args []string) error {
	nameOrID := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	includeBots, _ := cmd.Flags().GetBool("include-bots")
	includeDeactivated, _ := cmd.Flags().GetBool("include-deactivated")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Resolve channel name to ID.
	debug, _ := cmd.Flags().GetBool("debug")
	var progressWriter io.Writer
	if !output.IsJSON(format) {
		progressWriter = os.Stderr
	}
	channelID, err := channels.ResolveChannel(client, nameOrID, progressWriter, debug)
	if err != nil {
		return fmt.Errorf("resolving channel: %w", err)
	}

	// Fetch member IDs with cursor-based pagination.
	prog := progress.NewCounter("Fetching members", format)

	var allMemberIDs []string
	var isPartial bool

	params := map[string]string{
		"channel": channelID,
		"limit":   "200",
	}

	for {
		prog.Update(len(allMemberIDs))

		result, err := client.Call("conversations.members", params)
		if err != nil {
			prog.Finish()

			if _, ok := api.AsRateLimitError(err); ok && len(allMemberIDs) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after fetching %d member IDs; showing partial results", len(allMemberIDs)), output.IsJSON(format))
				isPartial = true
				break
			}

			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("fetching channel members: %w", err)
		}

		memberIDs := extractStringSlice(result, "members")
		allMemberIDs = append(allMemberIDs, memberIDs...)

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(allMemberIDs) >= limit {
			break
		}

		params["cursor"] = cursor
	}

	prog.Finish()

	if len(allMemberIDs) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no members found")
		}
		return nil
	}

	// Truncate member IDs to the requested limit.
	if len(allMemberIDs) > limit {
		allMemberIDs = allMemberIDs[:limit]
	}

	// Resolve each user ID to a profile via users.info.
	prog = progress.NewCounter("Resolving users", format)

	var results []map[string]any
	for i, uid := range allMemberIDs {
		prog.Update(i)

		info, err := client.Call("users.info", map[string]string{
			"user": uid,
		})
		if err != nil {
			if _, ok := api.AsRateLimitError(err); ok {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after resolving %d of %d members; showing partial results", len(results), len(allMemberIDs)), output.IsJSON(format))
				isPartial = true
				break
			}

			// Log a warning and skip this user on non-fatal errors.
			if !output.IsJSON(format) {
				fmt.Fprintf(os.Stderr, "warning: could not resolve user %s: %v\n", uid, err)
			}
			continue
		}

		user, ok := info["user"].(map[string]any)
		if !ok {
			continue
		}

		if !filterUser(user, includeBots, includeDeactivated) {
			continue
		}

		results = append(results, extractUserFields(user))
	}

	prog.Finish()

	if len(results) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no members found (after filtering)")
		}
		return nil
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
			fmt.Fprintf(os.Stderr, "Done. %d members fetched (partial — rate limited).\n", len(results))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d members fetched.\n", len(results))
		}
	}

	return nil
}
