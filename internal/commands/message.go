package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	"github.com/natikgadzhi/slack-cli/internal/output"
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

	// Fetch the message/thread via conversations.replies.
	result, err := client.Call("conversations.replies", map[string]string{
		"channel": channelID,
		"ts":      fetchTS,
		"limit":   "200",
	})
	if err != nil {
		return fmt.Errorf("fetching message: %w", err)
	}

	messages := api.ExtractItems(result, "messages")
	if len(messages) == 0 {
		fmt.Fprintln(os.Stderr, "no messages found")
		return nil
	}

	// Resolve user IDs to display names.
	messages, err = resolver.ResolveUsers(messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}

	// Collect the team URL result (goroutine already running since before the fetch).
	teamResult := <-teamCh
	teamURL := teamResult.url
	teamErr := teamResult.err
	if teamErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamErr)
	}

	// Format and render (always as a list — single message is just len=1).
	formatted := formatMessages(messages, teamURL, channelID, teamErr == nil)
	if err := output.RenderMessages(os.Stdout, formatted, format); err != nil {
		return err
	}

	// Cache the result (best-effort).
	cacheSlug := cache.MessageSlug(channelID, fetchTS)
	cacheWrite(getCache(), "message", cacheSlug, formatted, cache.Metadata{
		SourceURL: rawURL,
		Command:   fmt.Sprintf("message %s", rawURL),
	})

	return nil
}
