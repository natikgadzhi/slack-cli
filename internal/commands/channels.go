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
	"github.com/natikgadzhi/slack-cli/internal/users"
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
	Args: exactlyOneArg(
		"a channel name or ID",
		"slack-cli channels get <name|id> [flags]",
		"slack-cli channels get general --since 2d",
		"slack-cli channels get C12345678",
	),
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
	Args: exactlyOneArg(
		"a channel name or ID",
		"slack-cli channels get <name|id> [flags]",
		"slack-cli channels get general --since 2d",
		"slack-cli channels get C12345678",
	),
	RunE: runChannel,
}

func init() {
	// Register flags on the "channels get" subcommand.
	channelsGetCmd.Flags().String("since", "", "Start time (e.g. 2d, 2026-03-01)")
	channelsGetCmd.Flags().String("until", "", "End time (e.g. 2026-03-10)")
	channelsGetCmd.Flags().IntP("limit", "n", 50, "Maximum number of messages to fetch")
	channelsGetCmd.Flags().Bool("with-replies", false, "Expand thread replies inline")
	channelsGetCmd.Flags().Bool("with-pins", false, "Fetch and display pinned items and bookmarks")
	channelsGetCmd.Flags().Bool("download-files", false, "Download file attachments to disk")
	channelsGetCmd.Flags().String("download-dir", "slack-files", "Directory for downloaded files")

	// Register the same flags on the deprecated "channel" alias.
	channelCmd.Flags().String("since", "", "Start time (e.g. 2d, 2026-03-01)")
	channelCmd.Flags().String("until", "", "End time (e.g. 2026-03-10)")
	channelCmd.Flags().IntP("limit", "n", 50, "Maximum number of messages to fetch")
	channelCmd.Flags().Bool("with-replies", false, "Expand thread replies inline")
	channelCmd.Flags().Bool("with-pins", false, "Fetch and display pinned items and bookmarks")
	channelCmd.Flags().Bool("download-files", false, "Download file attachments to disk")
	channelCmd.Flags().String("download-dir", "slack-files", "Directory for downloaded files")

	// Complete the channel name/ID argument from the user's channel list.
	channelsGetCmd.ValidArgsFunction = completeChannelNames
	channelCmd.ValidArgsFunction = completeChannelNames

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

	// If --with-pins is set, start fetching pins and bookmarks concurrently.
	withPins, _ := cmd.Flags().GetBool("with-pins")
	var pinsCh, bookmarksCh <-chan pinsResult
	if withPins {
		pinsCh = fetchPinsAsync(client, channelID)
		bookmarksCh = fetchBookmarksAsync(client, channelID)
	}

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

	// Expand thread replies when requested.
	withReplies, _ := cmd.Flags().GetBool("with-replies")
	if withReplies {
		expandThreadReplies(client, resolver, formatted, channelID, teamURL, teamErr == nil)
	}

	// Download file attachments when requested.
	if dl, _ := cmd.Flags().GetBool("download-files"); dl {
		dlDir, _ := cmd.Flags().GetString("download-dir")
		downloadMessageFiles(client, formatted, dlDir)
	}

	// Collect pins and bookmarks if requested.
	var pinnedItems []formatting.PinnedItem
	var bookmarks []formatting.Bookmark
	if withPins {
		pinsResult := <-pinsCh
		if pinsResult.err != nil {
			if !output.IsJSON(format) {
				fmt.Fprintf(os.Stderr, "warning: fetching pins: %v\n", pinsResult.err)
			}
		} else {
			pinnedItems = pinsResult.pins
			// Resolve user IDs in pinned item creators.
			for i := range pinnedItems {
				if pinnedItems[i].CreatedBy != "" {
					pinnedItems[i].CreatedBy = resolver.DisplayName(pinnedItems[i].CreatedBy)
				}
			}
		}

		bmResult := <-bookmarksCh
		if bmResult.err != nil {
			if !output.IsJSON(format) {
				fmt.Fprintf(os.Stderr, "warning: fetching bookmarks: %v\n", bmResult.err)
			}
		} else {
			bookmarks = bmResult.bookmarks
		}
	}

	if output.IsJSON(format) {
		if withPins && (len(pinnedItems) > 0 || len(bookmarks) > 0) {
			// Wrap in ChannelMetadata when pins/bookmarks are present.
			meta := formatting.ChannelMetadata{
				PinnedItems: pinnedItems,
				Bookmarks:   bookmarks,
				Messages:    formatted,
			}
			if isPartial {
				meta.Warning = "rate limited: results may be incomplete"
			}
			if err := output.PrintJSON(meta); err != nil {
				return err
			}
		} else {
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
		}
	} else {
		// Table output: print pins/bookmarks header, then messages.
		if withPins {
			renderPinsHeader(pinnedItems, bookmarks)
		}
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

// expandThreadReplies fetches thread replies for each message with ReplyCount > 0
// and populates the Replies field. The parent message itself is excluded from replies.
func expandThreadReplies(client *api.Client, resolver *users.UserResolver, messages []formatting.Message, channelID, teamURL string, hasTeamURL bool) {
	for i := range messages {
		if messages[i].ReplyCount == 0 || messages[i].TS == "" {
			continue
		}
		result, err := client.Call("conversations.replies", map[string]string{
			"channel": channelID,
			"ts":      messages[i].TS,
			"limit":   "200",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: fetching replies for %s: %v\n", messages[i].TS, err)
			continue
		}
		rawReplies := api.ExtractItems(result, "messages")
		if len(rawReplies) <= 1 {
			continue
		}
		// Skip the first element (the parent message itself).
		rawReplies = rawReplies[1:]

		rawReplies, _ = resolver.ResolveUsers(rawReplies)

		replies := make([]formatting.Message, 0, len(rawReplies))
		for _, r := range rawReplies {
			msg := formatting.FormatMessage(r)
			if hasTeamURL {
				if ts, ok := r["ts"].(string); ok && ts != "" {
					msg.Link = formatting.BuildPermalink(teamURL, channelID, ts)
				}
			}
			replies = append(replies, msg)
		}
		messages[i].Replies = replies
	}
}

// renderMessagesTable renders messages as a table to stdout. Instead of a LINK
// column (whose long permalink would be truncated), the fixed-width TIME cell is
// rendered as an OSC-8 hyperlink to the message permalink, keeping it clickable
// without being clipped. Messages without a permalink keep a plain TIME cell.
// Thread replies (when expanded via --with-replies) are rendered with a "↳"
// prefix on their text.
func renderMessagesTable(messages []formatting.Message) {
	t := table.New()
	t.Header("TIME", "USER", "TEXT")
	for _, msg := range messages {
		text := truncate(messageTextWithFiles(msg), 80)
		timeCell := msg.Time
		if msg.Link != "" {
			timeCell = table.Hyperlink(msg.Link, msg.Time)
		}
		t.Row(timeCell, msg.User, text)

		for _, reply := range msg.Replies {
			rText := truncate("↳ "+messageTextWithFiles(reply), 80)
			rTime := reply.Time
			if reply.Link != "" {
				rTime = table.Hyperlink(reply.Link, reply.Time)
			}
			t.Row(rTime, reply.User, rText)
		}
	}
	_ = t.Flush()
}

// messageTextWithFiles appends file indicators to the message text for table display.
func messageTextWithFiles(msg formatting.Message) string {
	text := msg.Text
	for _, f := range msg.Files {
		text += " [file: " + f.Name + "]"
	}
	return text
}

// truncate shortens a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// --- Pins and bookmarks helpers ---

// pinsResult holds the result of a concurrent pins or bookmarks fetch.
type pinsResult struct {
	pins      []formatting.PinnedItem
	bookmarks []formatting.Bookmark
	err       error
}

// fetchPinsAsync starts a goroutine to fetch pinned items for a channel.
func fetchPinsAsync(client *api.Client, channelID string) <-chan pinsResult {
	ch := make(chan pinsResult, 1)
	go func() {
		result, err := client.Call("pins.list", map[string]string{
			"channel": channelID,
		})
		if err != nil {
			ch <- pinsResult{err: err}
			return
		}
		rawItems := api.ExtractItems(result, "items")
		pins := make([]formatting.PinnedItem, 0, len(rawItems))
		for _, raw := range rawItems {
			pins = append(pins, formatting.FormatPinnedItem(raw))
		}
		ch <- pinsResult{pins: pins}
	}()
	return ch
}

// fetchBookmarksAsync starts a goroutine to fetch bookmarks for a channel.
func fetchBookmarksAsync(client *api.Client, channelID string) <-chan pinsResult {
	ch := make(chan pinsResult, 1)
	go func() {
		result, err := client.Call("bookmarks.list", map[string]string{
			"channel_id": channelID,
		})
		if err != nil {
			ch <- pinsResult{err: err}
			return
		}
		rawBookmarks := api.ExtractItems(result, "bookmarks")
		bms := make([]formatting.Bookmark, 0, len(rawBookmarks))
		for _, raw := range rawBookmarks {
			bms = append(bms, formatting.FormatBookmark(raw))
		}
		ch <- pinsResult{bookmarks: bms}
	}()
	return ch
}

// renderPinsHeader prints a header section with pinned items and bookmarks
// before the messages table.
func renderPinsHeader(pins []formatting.PinnedItem, bookmarks []formatting.Bookmark) {
	if len(pins) == 0 && len(bookmarks) == 0 {
		return
	}

	for _, pin := range pins {
		text := formatting.PinnedItemDisplayText(pin)
		by := ""
		if pin.CreatedBy != "" {
			by = " by " + pin.CreatedBy
		}
		_, _ = fmt.Fprintf(os.Stdout, "\xf0\x9f\x93\x8c Pinned: %s%s\n", text, by)
	}

	for _, bm := range bookmarks {
		label := bm.Title
		if label == "" {
			label = "(untitled)"
		}
		link := ""
		if bm.Link != "" {
			link = " (" + bm.Link + ")"
		}
		_, _ = fmt.Fprintf(os.Stdout, "\xf0\x9f\x94\x96 Tab: %s%s\n", label, link)
	}

	_, _ = fmt.Fprintln(os.Stdout)
}
