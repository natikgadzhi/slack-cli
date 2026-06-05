package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/channels"
)

// Reaction holds a single emoji reaction with resolved user names.
type Reaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// reactionsCmd is the parent command for reaction-related subcommands.
var reactionsCmd = &cobra.Command{
	Use:   "reactions",
	Short: "View reactions on messages",
}

// reactionsGetCmd fetches reactions on a specific Slack message.
var reactionsGetCmd = &cobra.Command{
	Use:   "get [message-url]",
	Short: "Show reactions on a Slack message",
	Args:  cobra.MaximumNArgs(1),
	Example: `  slack-cli reactions get 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
  slack-cli reactions get --channel general --ts 1741234567.123456
  slack-cli reactions get 'https://...' -o json`,
	RunE: runReactionsGet,
}

func init() {
	reactionsGetCmd.Flags().String("channel", "", "Channel name or ID")
	reactionsGetCmd.Flags().String("ts", "", "Message timestamp")

	_ = reactionsGetCmd.RegisterFlagCompletionFunc("channel", completeChannelNames)

	reactionsCmd.AddCommand(reactionsGetCmd)
	rootCmd.AddCommand(reactionsCmd)
}

// runReactionsGet fetches reactions on a message and renders them.
func runReactionsGet(cmd *cobra.Command, args []string) error {
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")

	channelID, messageTS, err := parseMessageInput(args, channelFlag, tsFlag)
	if err != nil {
		return err
	}

	format := output.Resolve(cmd)

	// Set up client and user resolver.
	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	// Resolve channel name to ID if the --channel flag was used (it may be a name).
	if channelFlag != "" {
		debug, _ := cmd.Flags().GetBool("debug")
		var progressWriter io.Writer
		if !output.IsJSON(format) {
			progressWriter = os.Stderr
		}
		channelID, err = channels.ResolveChannel(client, channelID, progressWriter, debug)
		if err != nil {
			return fmt.Errorf("resolving channel: %w", err)
		}
	}

	// Show spinner while fetching.
	spinner := progress.NewSpinner("Fetching reactions", format)

	result, err := client.Call("reactions.get", map[string]string{
		"channel":   channelID,
		"timestamp": messageTS,
		"full":      "true",
	})

	spinner.Finish()

	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching reactions: %w", err)
	}

	// Extract the reactions array from the message.
	rawReactions := extractReactions(result)
	if len(rawReactions) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no reactions found")
		}
		return nil
	}

	// Resolve user IDs to display names. We collect unique IDs first to avoid
	// resolving the same user multiple times.
	nameMap := make(map[string]string)
	for _, r := range rawReactions {
		if users, ok := r["users"].([]any); ok {
			for _, u := range users {
				if uid, ok := u.(string); ok && uid != "" {
					if _, seen := nameMap[uid]; !seen {
						nameMap[uid] = resolver.DisplayName(uid)
					}
				}
			}
		}
	}

	// Build the structured reaction objects.
	reactions := make([]Reaction, 0, len(rawReactions))
	for _, r := range rawReactions {
		name, _ := r["name"].(string)
		count := extractCount(r)

		var userNames []string
		if users, ok := r["users"].([]any); ok {
			for _, u := range users {
				if uid, ok := u.(string); ok && uid != "" {
					userNames = append(userNames, nameMap[uid])
				}
			}
		}

		reactions = append(reactions, Reaction{
			Name:  name,
			Count: count,
			Users: userNames,
		})
	}

	// Render output.
	if output.IsJSON(format) {
		if err := output.PrintJSON(reactions); err != nil {
			return err
		}
	} else {
		renderReactionsTable(reactions)
	}

	return nil
}

// extractReactions pulls the reactions array from a reactions.get response.
// The structure is: { "message": { "reactions": [...] } }
func extractReactions(result map[string]any) []map[string]any {
	messageRaw, ok := result["message"]
	if !ok {
		return nil
	}
	messageMap, ok := messageRaw.(map[string]any)
	if !ok {
		return nil
	}
	reactionsRaw, ok := messageMap["reactions"]
	if !ok {
		return nil
	}
	reactionsArr, ok := reactionsRaw.([]any)
	if !ok {
		return nil
	}
	reactions := make([]map[string]any, 0, len(reactionsArr))
	for _, r := range reactionsArr {
		if rm, ok := r.(map[string]any); ok {
			reactions = append(reactions, rm)
		}
	}
	return reactions
}

func extractCount(r map[string]any) int {
	n, _ := toInt(r["count"])
	return n
}

// renderReactionsTable renders reactions as a table to stdout.
func renderReactionsTable(reactions []Reaction) {
	t := table.New()
	t.Header("EMOJI", "COUNT", "USERS")
	for _, r := range reactions {
		t.Row(
			":"+r.Name+":",
			fmt.Sprintf("%d", r.Count),
			strings.Join(r.Users, ", "),
		)
	}
	_ = t.Flush()
}
