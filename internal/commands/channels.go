package commands

import (
	"fmt"
	"io"
	"os"
	"strconv"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/channels"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// channelsCmd is the parent command for channel-related subcommands.
var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage and view Slack channels",
}

// channelsGetCmd fetches messages from a Slack channel.
var channelsGetCmd = &cobra.Command{
	Use:   "get <name|id>",
	Short: "Fetch messages from a Slack channel",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli channels get general --since 2d --limit 100
  slack-cli channels get C12345678 --since 2026-03-01 --until 2026-03-10
  slack-cli channels get general -o json | jq '.[].text'`,
	RunE: runChannel,
}

// channelCmd is a hidden backward-compatible alias for "channels get".
var channelCmd = &cobra.Command{
	Use:        "channel <name|id>",
	Short:      "Fetch messages from a Slack channel",
	Hidden:     true,
	Deprecated: "use 'channels get' instead",
	Args:       cobra.ExactArgs(1),
	RunE:       runChannel,
}

func init() {
	// Register flags on the "channels get" subcommand.
	channelsGetCmd.Flags().String("since", "", "Start time (e.g. 2d, 2026-03-01)")
	channelsGetCmd.Flags().String("until", "", "End time (e.g. 2026-03-10)")
	channelsGetCmd.Flags().IntP("limit", "n", 50, "Maximum number of messages to fetch")

	// Register the same flags on the deprecated "channel" alias.
	channelCmd.Flags().String("since", "", "Start time (e.g. 2d, 2026-03-01)")
	channelCmd.Flags().String("until", "", "End time (e.g. 2026-03-10)")
	channelCmd.Flags().IntP("limit", "n", 50, "Maximum number of messages to fetch")

	// Wire up the command tree.
	channelsCmd.AddCommand(channelsGetCmd)
	rootCmd.AddCommand(channelsCmd)
	rootCmd.AddCommand(channelCmd)
}

// runChannel fetches messages from a Slack channel with optional time range
// and limit. Shows a progress indicator on stderr during pagination.
func runChannel(cmd *cobra.Command, args []string) error {
	nameOrID := args[0]

	format := output.Resolve(cmd)

	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	limit, _ := cmd.Flags().GetInt("limit")

	// Set up client and user resolver.
	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	// Resolve channel name to ID.
	// Suppress progress output in JSON mode to keep stdout clean for piping.
	debug, _ := cmd.Flags().GetBool("debug")
	var progressWriter io.Writer
	if !output.IsJSON(format) {
		progressWriter = os.Stderr
	}
	channelID, err := channels.ResolveChannel(client, nameOrID, progressWriter, debug)
	if err != nil {
		return fmt.Errorf("resolving channel: %w", err)
	}

	// Build request params.
	params := map[string]string{
		"channel": channelID,
		"limit":   strconv.Itoa(limit),
	}

	sinceStr := ""
	untilStr := ""

	if since != "" {
		oldest, err := formatting.ParseTime(since)
		if err != nil {
			return fmt.Errorf("parsing --since: %w", err)
		}
		sinceStr = strconv.FormatFloat(oldest, 'f', -1, 64)
		params["oldest"] = sinceStr
	}

	if until != "" {
		latest, err := formatting.ParseTime(until)
		if err != nil {
			return fmt.Errorf("parsing --until: %w", err)
		}
		untilStr = strconv.FormatFloat(latest, 'f', -1, 64)
		params["latest"] = untilStr
	}

	// Start team URL fetch concurrently — it's independent of the message fetch.
	teamCh := fetchTeamURLAsync(client)

	// Fetch messages with progress indicator.
	prog := progress.NewCounter("Fetching messages", format)

	var allMessages []map[string]any
	var isPartial bool
	pageParams := make(map[string]string, len(params))
	for k, v := range params {
		pageParams[k] = v
	}

	for {
		prog.Update(len(allMessages))

		result, err := client.Call("conversations.history", pageParams)
		if err != nil {
			prog.Finish()

			// On rate limit with partial data, warn and render what we have.
			if _, ok := api.AsRateLimitError(err); ok && len(allMessages) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after fetching %d messages; showing partial results", len(allMessages)), output.IsJSON(format))
				isPartial = true
				break // fall through to render what we have
			}

			// For other CLI errors, print and exit with the right code.
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("fetching channel history: %w", err)
		}

		messages := api.ExtractItems(result, "messages")
		allMessages = append(allMessages, messages...)

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(allMessages) >= limit {
			break
		}

		pageParams["cursor"] = cursor
	}

	prog.Finish()

	if len(allMessages) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no messages found")
		}
		return nil
	}

	// Truncate to the requested limit.
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}

	// Resolve user IDs to display names.
	allMessages, err = resolver.ResolveUsers(allMessages)
	if err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}

	// Collect the team URL result (goroutine already running since before pagination).
	teamResult := <-teamCh
	teamURL := teamResult.url
	teamErr := teamResult.err
	if teamErr != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamErr)
	}

	// Format and render.
	formatted := formatMessages(allMessages, teamURL, channelID, teamErr == nil)

	if output.IsJSON(format) {
		if isPartial {
			pr := clierrors.NewPartialResult(formatted, "rate limited: results may be incomplete")
			if err := output.PrintJSON(pr); err != nil {
				return err
			}
		} else {
			if err := output.PrintJSON(formatted); err != nil {
				return err
			}
		}
	} else {
		// Table output: use table format for terminal.
		renderMessagesTable(formatted)
	}

	// Cache the result (best-effort).
	// Keep "channel" slug for cache compatibility.
	cacheSlug := cache.ChannelHistorySlug(channelID, sinceStr, untilStr)
	cacheWrite(getCache(), "channel", cacheSlug, formatted, cache.Metadata{
		Command: fmt.Sprintf("channel %s --since %s --until %s --limit %d", nameOrID, since, until, limit),
	})

	// Write per-item files if --derived flag was explicitly set.
	// For the channel command, each message gets its own file.
	if derivedDir := resolveDerivedDir(cmd); derivedDir != "" {
		if err := writeItemFiles(derivedDir, formatted, channelID, nameOrID); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d messages fetched (partial — rate limited).\n", len(formatted))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d messages fetched.\n", len(formatted))
		}
	}
	return nil
}

// renderMessagesTable renders messages as a table to stdout.
func renderMessagesTable(messages []formatting.Message) {
	t := table.New()
	t.Header("TIME", "USER", "TEXT", "LINK")
	for _, msg := range messages {
		text := truncate(msg.Text, 80)
		t.Row(msg.Time, msg.User, text, msg.Link)
	}
	_ = t.Flush()
}

// truncate shortens a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
