package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/channels"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	"github.com/natikgadzhi/slack-cli/internal/output"
)

var channelCmd = &cobra.Command{
	Use:   "channel <name|id>",
	Short: "Fetch messages from a Slack channel",
	Args:  cobra.ExactArgs(1),
	RunE:  runChannel,
}

func init() {
	channelCmd.Flags().String("since", "", "start time (e.g. 2d, 2026-03-01)")
	channelCmd.Flags().String("until", "", "end time (e.g. 2026-03-10)")
	channelCmd.Flags().Int("limit", 50, "maximum number of messages to fetch")
	rootCmd.AddCommand(channelCmd)
}

// runChannel fetches messages from a Slack channel with optional time range
// and limit. Shows a progress indicator on stderr during pagination.
func runChannel(cmd *cobra.Command, args []string) error {
	nameOrID := args[0]

	format, err := parseOutputFormat()
	if err != nil {
		return err
	}

	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	limit, _ := cmd.Flags().GetInt("limit")

	// Set up client and user resolver.
	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	// Resolve channel name to ID.
	fmt.Fprintf(os.Stderr, "Resolving channel %q...\n", nameOrID)
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
	// Manual pagination loop (instead of CallPaginated) to show progress on stderr.
	var allMessages []map[string]any
	pageParams := make(map[string]string, len(params))
	for k, v := range params {
		pageParams[k] = v
	}

	for {
		fmt.Fprintf(os.Stderr, "\rFetching messages... (%d so far)", len(allMessages))

		result, err := client.Call("conversations.history", pageParams)
		if err != nil {
			clearProgress()
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

	clearProgress()

	if len(allMessages) == 0 {
		fmt.Fprintln(os.Stderr, "no messages found")
		return nil
	}

	// Truncate to the requested limit.
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}

	// Resolve user IDs to display names.
	fmt.Fprintf(os.Stderr, "Resolving users...\n")
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
	if err := output.RenderMessages(os.Stdout, formatted, format); err != nil {
		return err
	}

	// Cache the result (best-effort).
	cacheSlug := cache.ChannelHistorySlug(channelID, sinceStr, untilStr)
	cacheWrite(getCache(), "channel", cacheSlug, formatted, cache.Metadata{
		Command: fmt.Sprintf("channel %s --since %s --until %s --limit %d", nameOrID, since, until, limit),
	})

	fmt.Fprintf(os.Stderr, "Done. %d messages fetched.\n", len(formatted))
	return nil
}
