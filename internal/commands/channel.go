package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/channels"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	internalOutput "github.com/natikgadzhi/slack-cli/internal/output"
)

var channelCmd = &cobra.Command{
	Use:   "channel <name|id>",
	Short: "Fetch messages from a Slack channel",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli channel general --since 2d --limit 100
  slack-cli channel C12345678 --since 2026-03-01 --until 2026-03-10
  slack-cli channel general -o json | jq '.[].text'`,
	RunE: runChannel,
}

func init() {
	channelCmd.Flags().String("since", "", "Start time (e.g. 2d, 2026-03-01)")
	channelCmd.Flags().String("until", "", "End time (e.g. 2026-03-10)")
	channelCmd.Flags().IntP("limit", "n", 50, "Maximum number of messages to fetch")
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
	channelID, err := channels.ResolveChannel(client, nameOrID)
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
	type teamURLResult struct {
		url string
		err error
	}
	teamCh := make(chan teamURLResult, 1)
	go func() {
		u, err := client.GetTeamURL()
		teamCh <- teamURLResult{u, err}
	}()

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
		fmt.Fprintln(os.Stderr, "no messages found")
		return nil
	}

	// Truncate to the requested limit.
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}

	// Resolve user IDs to display names.
	allMessages, err = resolver.ResolveUsers(allMessages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}

	// Collect the team URL result (goroutine already running since before pagination).
	teamResult := <-teamCh
	teamURL := teamResult.url
	teamErr := teamResult.err
	if teamErr != nil {
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
	cacheSlug := cache.ChannelHistorySlug(channelID, sinceStr, untilStr)
	cacheWrite(getCache(), "channel", cacheSlug, formatted, cache.Metadata{
		Command: fmt.Sprintf("channel %s --since %s --until %s --limit %d", nameOrID, since, until, limit),
	})

	// Write per-item files if --derived is set.
	// For the channel command, each message gets its own file.
	if DerivedDir != "" {
		// Use the original input as channel name context (falls back to channelID inside writeItemFiles).
		if err := writeItemFiles(DerivedDir, formatted, channelID, nameOrID); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	if isPartial {
		fmt.Fprintf(os.Stderr, "Done. %d messages fetched (partial — rate limited).\n", len(formatted))
	} else {
		fmt.Fprintf(os.Stderr, "Done. %d messages fetched.\n", len(formatted))
	}
	return nil
}

// renderMessagesTable renders messages as a table to stdout.
func renderMessagesTable(messages []formatting.Message) {
	t := output.NewTable()
	t.Header("TIME", "USER", "TEXT", "LINK")
	for _, msg := range messages {
		text := truncate(msg.Text, 80)
		t.Row(msg.Time, msg.User, text, msg.Link)
	}
	t.Flush()
}

// renderSearchTable renders search results as a table to stdout.
func renderSearchTable(results []map[string]any) {
	t := output.NewTable()
	t.Header("CHANNEL", "TIME", "USER", "TEXT", "LINK")
	for _, r := range results {
		channel, _ := r["channel"].(string)
		ts, _ := r["ts"].(string)
		user, _ := r["user"].(string)
		text, _ := r["text"].(string)
		permalink, _ := r["permalink"].(string)

		timeStr := internalOutput.FormatTS(ts)
		text = truncate(text, 80)

		t.Row(channel, timeStr, user, text, permalink)
	}
	t.Flush()
}

// truncate shortens a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
