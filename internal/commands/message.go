package commands

import (
	"fmt"
	"os"

	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	"github.com/natikgadzhi/slack-cli/internal/output"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message <url>",
	Short: "Fetch a single Slack message or thread by URL",
	Args:  cobra.ExactArgs(1),
	RunE:  runMessage,
}

func init() {
	rootCmd.AddCommand(messageCmd)
}

// runMessage fetches a single Slack message or thread by URL, resolves users,
// formats the output, and optionally caches the result.
func runMessage(cmd *cobra.Command, args []string) error {
	rawURL := args[0]

	format, err := parseOutputFormat()
	if err != nil {
		return err
	}

	// Parse the Slack URL.
	channelID, messageTS, threadTS, err := formatting.ParseSlackURL(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}

	// Set up client and user resolver.
	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	// Determine which timestamp to fetch replies for.
	fetchTS := messageTS
	if threadTS != "" {
		fetchTS = threadTS
	}

	// Fetch the message/thread via conversations.replies.
	params := map[string]string{
		"channel": channelID,
		"ts":      fetchTS,
		"limit":   "200",
	}

	result, err := client.Call("conversations.replies", params)
	if err != nil {
		return fmt.Errorf("fetching message: %w", err)
	}

	messages := extractMessagesFromResponse(result)
	if len(messages) == 0 {
		fmt.Fprintln(os.Stderr, "no messages found")
		return nil
	}

	// Resolve user IDs to display names.
	messages, err = resolver.ResolveUsers(messages)
	if err != nil {
		// User resolution failure is non-fatal; log and continue with raw UIDs.
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}

	// Get team URL for building permalinks.
	teamURL, teamErr := client.GetTeamURL()
	if teamErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamErr)
	}

	// Cache the result (best-effort).
	c := getCache()
	cacheSlug := cache.MessageSlug(channelID, fetchTS)

	// Format and render.
	if threadTS != "" || len(messages) > 1 {
		// Thread: render all messages.
		formatted := make([]formatting.Message, 0, len(messages))
		for _, m := range messages {
			msg := formatting.FormatMessage(m)
			if teamErr == nil {
				if ts, ok := m["ts"].(string); ok && ts != "" {
					msg.Link = formatting.BuildPermalink(teamURL, channelID, ts)
				}
			}
			formatted = append(formatted, msg)
		}
		if err := output.RenderMessages(os.Stdout, formatted, format); err != nil {
			return err
		}
		cacheWrite(c, "message", cacheSlug, formatted, cache.Metadata{
			SourceURL: rawURL,
			Command:   fmt.Sprintf("message %s", rawURL),
		})
	} else {
		// Single message.
		formatted := formatting.FormatMessage(messages[0])
		if teamErr == nil {
			if ts, ok := messages[0]["ts"].(string); ok && ts != "" {
				formatted.Link = formatting.BuildPermalink(teamURL, channelID, ts)
			}
		}
		if err := output.RenderSingle(os.Stdout, formatted, format); err != nil {
			return err
		}
		cacheWrite(c, "message", cacheSlug, formatted, cache.Metadata{
			SourceURL: rawURL,
			Command:   fmt.Sprintf("message %s", rawURL),
		})
	}

	return nil
}
