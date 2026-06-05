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
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/channels"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

var messageCmd = &cobra.Command{
	Use:   "message [url]",
	Short: "Fetch a single Slack message or thread by URL, or by --channel and --ts",
	Args:  cobra.MaximumNArgs(1),
	Example: `  slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
  slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456' -o json
  slack-cli message --channel general --ts 1741234567.123456
  slack-cli message --channel C12345 --ts 1741234567.123456 -o json`,
	RunE: runMessage,
}

func init() {
	messageCmd.Flags().String("channel", "", "Channel name or ID")
	messageCmd.Flags().String("ts", "", "Message timestamp")
	messageCmd.Flags().Bool("download-files", false, "Download file attachments to disk")
	messageCmd.Flags().String("download-dir", "slack-files", "Directory for downloaded files")

	_ = messageCmd.RegisterFlagCompletionFunc("channel", completeChannelNames)

	rootCmd.AddCommand(messageCmd)
}

// validateMessageArgs checks that exactly one input mode is used: either a
// positional URL or the --channel + --ts flag pair. It returns an error
// describing the conflict when the caller gets it wrong.
func validateMessageArgs(args []string, channelFlag, tsFlag string) error {
	hasURL := len(args) == 1
	hasChannel := channelFlag != ""
	hasTS := tsFlag != ""

	if hasURL && (hasChannel || hasTS) {
		return fmt.Errorf("cannot combine a positional URL with --channel/--ts flags")
	}
	if hasChannel != hasTS {
		return fmt.Errorf("--channel and --ts must be provided together")
	}
	if !hasURL && !hasChannel {
		return fmt.Errorf("provide either a message URL or --channel and --ts")
	}
	return nil
}

// runMessage fetches a single Slack message or thread by URL (positional arg)
// or by --channel + --ts flags, resolves users, formats the output, and
// optionally caches the result.
func runMessage(cmd *cobra.Command, args []string) error {
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")

	if err := validateMessageArgs(args, channelFlag, tsFlag); err != nil {
		return err
	}

	format := output.Resolve(cmd)

	var channelID, fetchTS, rawURL string

	if len(args) == 1 {
		// URL mode: parse the Slack URL.
		rawURL = args[0]
		cid, messageTS, threadTS, err := formatting.ParseSlackURL(rawURL)
		if err != nil {
			return fmt.Errorf("parsing URL: %w", err)
		}
		channelID = cid
		fetchTS = messageTS
		if threadTS != "" {
			fetchTS = threadTS
		}
	} else {
		// Flag mode: resolve channel name → ID, use --ts directly.
		client, _, err := setupClient()
		if err != nil {
			return err
		}

		debug, _ := cmd.Flags().GetBool("debug")
		var progressWriter io.Writer
		if !output.IsJSON(format) {
			progressWriter = os.Stderr
		}
		cid, err := channels.ResolveChannel(client, channelFlag, progressWriter, debug)
		if err != nil {
			return fmt.Errorf("resolving channel: %w", err)
		}
		channelID = cid
		fetchTS = tsFlag
	}

	// Set up client and user resolver.
	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	// Start team URL fetch concurrently — it's independent of the message fetch.
	teamCh := fetchTeamURLAsync(client)

	// Show spinner while fetching.
	spinner := progress.NewSpinner("Fetching message", format)

	// Fetch the message/thread via conversations.replies.
	result, err := client.Call("conversations.replies", map[string]string{
		"channel": channelID,
		"ts":      fetchTS,
		"limit":   "200",
	})

	spinner.Finish()

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

	// Download file attachments when requested.
	if dl, _ := cmd.Flags().GetBool("download-files"); dl {
		dlDir, _ := cmd.Flags().GetString("download-dir")
		downloadMessageFiles(client, formatted, dlDir)
	}

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
	cacheCmd := fmt.Sprintf("message --channel %s --ts %s", channelID, fetchTS)
	if rawURL != "" {
		cacheCmd = fmt.Sprintf("message %s", rawURL)
	}
	cacheWrite(getCache(), "message", cacheSlug, formatted, cache.Metadata{
		SourceURL: rawURL,
		Command:   cacheCmd,
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
