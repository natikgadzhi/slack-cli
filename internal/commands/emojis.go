package commands

import (
	"fmt"
	"os"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// emojisCmd is the parent command for emoji-related subcommands.
var emojisCmd = &cobra.Command{
	Use:   "emojis",
	Short: "Search workspace emojis",
}

// emojisSearchCmd searches custom workspace emojis by name.
var emojisSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search custom emojis by name",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli emojis search fire
  slack-cli emojis search party --limit 10
  slack-cli emojis search logo -o json`,
	RunE: runEmojisSearch,
}

func init() {
	emojisSearchCmd.Flags().IntP("limit", "n", 50, "Maximum number of results")
	emojisCmd.AddCommand(emojisSearchCmd)
	rootCmd.AddCommand(emojisCmd)
}

// emojiResult holds the processed fields for a single emoji match.
type emojiResult struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// runEmojisSearch fetches all custom emojis via emoji.list, filters by
// name substring, and renders the results.
func runEmojisSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	spinner := progress.NewSpinner("Fetching emojis", format)
	spinner.Update(0)

	result, err := client.Call("emoji.list", nil)

	spinner.Finish()

	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching emojis: %w", err)
	}

	// The emoji.list response has an "emoji" object: map of name -> value.
	emojis := extractEmojiMap(result)
	if len(emojis) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	// Filter emojis by name (case-insensitive substring match).
	matched := filterEmojis(emojis, query, limit)

	if len(matched) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	// Render output.
	if output.IsJSON(format) {
		if err := output.PrintJSON(matched); err != nil {
			return err
		}
	} else {
		renderEmojisTable(matched)
	}

	if !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "Done. %d emojis found.\n", len(matched))
	}

	return nil
}

// extractEmojiMap pulls the "emoji" map from the emoji.list response.
// Returns nil if the field is missing or not a map.
func extractEmojiMap(result map[string]any) map[string]string {
	raw, ok := result["emoji"]
	if !ok {
		return nil
	}
	emojiRaw, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	emojis := make(map[string]string, len(emojiRaw))
	for name, val := range emojiRaw {
		if s, ok := val.(string); ok {
			emojis[name] = s
		}
	}
	return emojis
}

// filterEmojis returns emoji results whose name contains the query as a
// case-insensitive substring, capped at limit results.
func filterEmojis(emojis map[string]string, query string, limit int) []emojiResult {
	queryLower := strings.ToLower(query)
	var matched []emojiResult

	for name, value := range emojis {
		if !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		r := emojiResult{
			Name: ":" + name + ":",
		}

		if strings.HasPrefix(value, "alias:") {
			r.Type = "alias"
			r.Value = ":" + strings.TrimPrefix(value, "alias:") + ":"
		} else {
			r.Type = "custom"
			r.Value = value
		}

		matched = append(matched, r)
		if len(matched) >= limit {
			break
		}
	}

	return matched
}

// renderEmojisTable renders emoji results as a table to stdout.
func renderEmojisTable(results []emojiResult) {
	t := table.New()
	t.Header("NAME", "TYPE", "VALUE")
	for _, r := range results {
		t.Row(r.Name, r.Type, r.Value)
	}
	_ = t.Flush()
}
