package commands

import (
	"fmt"
	"os"
	"strconv"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// channelsListCmd lists channels and conversations the user has access to.
var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List channels and conversations",
	Args:  cobra.NoArgs,
	Example: `  slack-cli channels list
  slack-cli channels list --limit 50
  slack-cli channels list --type public_channel,private_channel,mpim,im
  slack-cli channels list --include-archived`,
	RunE: runChannelsList,
}

func init() {
	channelsListCmd.Flags().IntP("limit", "n", 100, "Maximum number of channels to return")
	channelsListCmd.Flags().String("type", "public_channel,private_channel", "Comma-separated conversation types (public_channel,private_channel,mpim,im)")
	channelsListCmd.Flags().Bool("include-archived", false, "Include archived channels")
	channelsCmd.AddCommand(channelsListCmd)
}

// runChannelsList fetches workspace channels via conversations.list with
// cursor-based pagination and renders them as a table or JSON.
func runChannelsList(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	types, _ := cmd.Flags().GetString("type")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")

	format := output.Resolve(cmd)

	// Set up client (no user resolver needed).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Fetch channels with progress indicator.
	prog := progress.NewCounter("Fetching channels", format)

	var allChannels []map[string]any
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
		prog.Update(len(allChannels))

		result, err := client.Call("conversations.list", params)
		if err != nil {
			prog.Finish()

			// On rate limit with partial data, warn and render what we have.
			if _, ok := api.AsRateLimitError(err); ok && len(allChannels) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after fetching %d channels; showing partial results", len(allChannels)), output.IsJSON(format))
				isPartial = true
				break
			}

			// For other CLI errors, print and exit with the right code.
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("fetching channels: %w", err)
		}

		channels := api.ExtractItems(result, "channels")
		allChannels = append(allChannels, channels...)

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(allChannels) >= limit {
			break
		}

		params["cursor"] = cursor
	}

	prog.Finish()

	if len(allChannels) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no channels found")
		}
		return nil
	}

	// Truncate to the requested limit.
	if len(allChannels) > limit {
		allChannels = allChannels[:limit]
	}

	// Build clean result slice.
	results := make([]map[string]any, 0, len(allChannels))
	for _, ch := range allChannels {
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
			fmt.Fprintf(os.Stderr, "Done. %d channels fetched (partial — rate limited).\n", len(results))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d channels fetched.\n", len(results))
		}
	}

	return nil
}

// extractChannelFields extracts display fields from a raw Slack channel object.
func extractChannelFields(ch map[string]any) map[string]any {
	r := map[string]any{
		"id":          getString(ch, "id"),
		"name":        getString(ch, "name"),
		"type":        deriveChannelType(ch),
		"num_members": 0,
		"is_archived": false,
		"topic":       "",
		"purpose":     "",
	}

	if numMembers, ok := ch["num_members"].(float64); ok {
		r["num_members"] = int(numMembers)
	}

	if isArchived, ok := ch["is_archived"].(bool); ok {
		r["is_archived"] = isArchived
	}

	// Topic lives under topic.value.
	if topic, ok := ch["topic"].(map[string]any); ok {
		if val, ok := topic["value"].(string); ok {
			r["topic"] = val
		}
	}

	// Purpose lives under purpose.value.
	if purpose, ok := ch["purpose"].(map[string]any); ok {
		if val, ok := purpose["value"].(string); ok {
			r["purpose"] = val
		}
	}

	return r
}

// deriveChannelType determines the conversation type from the channel's boolean flags.
func deriveChannelType(ch map[string]any) string {
	if isMpim, ok := ch["is_mpim"].(bool); ok && isMpim {
		return "mpim"
	}
	if isIM, ok := ch["is_im"].(bool); ok && isIM {
		return "im"
	}
	if isPrivate, ok := ch["is_private"].(bool); ok && isPrivate {
		return "private_channel"
	}
	return "public_channel"
}

// renderChannelsTable renders channels as a table to stdout.
func renderChannelsTable(channels []map[string]any) {
	t := output.NewTable()
	t.Header("ID", "NAME", "TYPE", "MEMBERS", "TOPIC")
	for _, ch := range channels {
		id, _ := ch["id"].(string)
		name, _ := ch["name"].(string)
		chType, _ := ch["type"].(string)
		numMembers := 0
		if n, ok := ch["num_members"].(int); ok {
			numMembers = n
		}
		topic, _ := ch["topic"].(string)
		topic = truncate(topic, 60)
		t.Row(id, name, chType, strconv.Itoa(numMembers), topic)
	}
	_ = t.Flush()
}
