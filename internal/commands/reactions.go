package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/channels"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
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

	// Complete the --channel flag from the user's channel list.
	_ = reactionsGetCmd.RegisterFlagCompletionFunc("channel", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeChannelNames(cmd, args, toComplete)
	})

	reactionsCmd.AddCommand(reactionsGetCmd)
	rootCmd.AddCommand(reactionsCmd)
}

// parseReactionsInput resolves the channel ID and message timestamp from either
// a positional URL argument or --channel/--ts flags. It returns an error if the
// input combination is invalid.
func parseReactionsInput(args []string, channelFlag, tsFlag string) (channelID, messageTS string, err error) {
	switch {
	case len(args) == 1 && (channelFlag != "" || tsFlag != ""):
		return "", "", fmt.Errorf("cannot use both a message URL argument and --channel/--ts flags")
	case len(args) == 1:
		cid, mts, _, parseErr := formatting.ParseSlackURL(args[0])
		if parseErr != nil {
			return "", "", fmt.Errorf("parsing URL: %w", parseErr)
		}
		return cid, mts, nil
	case channelFlag != "" && tsFlag != "":
		return channelFlag, tsFlag, nil
	case channelFlag != "" || tsFlag != "":
		return "", "", fmt.Errorf("both --channel and --ts are required when not using a URL")
	default:
		return "", "", fmt.Errorf("provide a message URL or both --channel and --ts")
	}
}

// runReactionsGet fetches reactions on a message and renders them.
func runReactionsGet(cmd *cobra.Command, args []string) error {
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")

	channelID, messageTS, err := parseReactionsInput(args, channelFlag, tsFlag)
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
		resolved, err := channels.ResolveChannel(client, channelID, progressWriter, debug)
		if err != nil {
			return fmt.Errorf("resolving channel: %w", err)
		}
		channelID = resolved
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

	// Collect all unique user IDs across all reactions.
	uniqueUIDs := make(map[string]struct{})
	for _, r := range rawReactions {
		if users, ok := r["users"].([]any); ok {
			for _, u := range users {
				if uid, ok := u.(string); ok && uid != "" {
					uniqueUIDs[uid] = struct{}{}
				}
			}
		}
	}

	// Resolve all user IDs to display names in batch.
	nameMap := make(map[string]string, len(uniqueUIDs))
	var mu sync.Mutex

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(5)

	for uid := range uniqueUIDs {
		g.Go(func() error {
			name := resolver.DisplayName(uid)
			mu.Lock()
			nameMap[uid] = name
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	// Build the structured reaction objects.
	reactions := make([]Reaction, 0, len(rawReactions))
	for _, r := range rawReactions {
		name, _ := r["name"].(string)
		count := extractCount(r)

		var userNames []string
		if users, ok := r["users"].([]any); ok {
			for _, u := range users {
				if uid, ok := u.(string); ok && uid != "" {
					if resolved, found := nameMap[uid]; found {
						userNames = append(userNames, resolved)
					} else {
						userNames = append(userNames, uid)
					}
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

// extractCount extracts the "count" field from a reaction map, handling both
// float64 (JSON default) and int types.
func extractCount(r map[string]any) int {
	switch n := r["count"].(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
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
