package commands

import (
	"fmt"
	"os"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

var messageCmd = &cobra.Command{
	Use:   "message <url>",
	Short: "Fetch a single Slack message or thread by URL",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
  slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456' -o json`,
	RunE: runMessage,
}

func init() {
	rootCmd.AddCommand(messageCmd)
}

// runMessage fetches a single Slack message or thread by URL, resolves users,
// formats the output, and optionally caches the result.
func runMessage(cmd *cobra.Command, args []string) error {
	rawURL := args[0]

	format := output.Resolve(cmd)

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
	teamCh := fetchTeamURLAsync(client)

	// Fetch the message/thread via conversations.replies.
	result, err := client.Call("conversations.replies", map[string]string{
		"channel": channelID,
		"ts":      fetchTS,
		"limit":   "200",
	})
	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching message: %w", err)
	}

	messages := api.ExtractItems(result, "messages")
	if len(messages) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no messages found")
		}
		return nil
	}

	// Resolve user IDs to display names.
	messages, err = resolver.ResolveUsers(messages)
	if err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}

	// Collect the team URL result (goroutine already running since before the fetch).
	teamResult := <-teamCh
	teamURL := teamResult.url
	teamErr := teamResult.err
	if teamErr != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamErr)
	}

	// Format and render (always as a list — single message is just len=1).
	formatted := formatMessages(messages, teamURL, channelID, teamErr == nil)

	if output.IsJSON(format) {
		if err := output.PrintJSON(formatted); err != nil {
			return err
		}
	} else {
		// Table output: same format as channel command.
		renderMessagesTable(formatted)
	}

	// Cache the result (best-effort).
	cacheSlug := cache.MessageSlug(channelID, fetchTS)
	cacheWrite(getCache(), "message", cacheSlug, formatted, cache.Metadata{
		SourceURL: rawURL,
		Command:   fmt.Sprintf("message %s", rawURL),
	})

	// Write per-item files if --derived flag was explicitly set.
	// For the message command, thread root + replies go into ONE file.
	if derivedDir := resolveDerivedDir(cmd); derivedDir != "" {
		if err := writeThreadFile(derivedDir, formatted, channelID, "", fetchTS, rawURL); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	return nil
}
