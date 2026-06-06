package commands

import (
	"fmt"
	"os"
	"sort"
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
	Short: "List, search, and download workspace emojis",
}

// emojisSearchCmd searches custom workspace emojis by name.
var emojisSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search custom emojis by name",
	Args: exactlyOneArg(
		"a search query",
		"slack-cli emojis search <query>",
		"slack-cli emojis search fire",
	),
	Example: `  slack-cli emojis search fire
  slack-cli emojis search party --limit 10
  slack-cli emojis search logo -o json`,
	RunE: runEmojisSearch,
}

// emojisListCmd lists all custom workspace emojis.
var emojisListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all custom emojis",
	Args:  cobra.NoArgs,
	Example: `  slack-cli emojis list
  slack-cli emojis list --limit 500
  slack-cli emojis list --type custom
  slack-cli emojis list -o json`,
	RunE: runEmojisList,
}

func init() {
	emojisSearchCmd.Flags().IntP("limit", "n", 50, "Maximum number of results")

	emojisListCmd.Flags().IntP("limit", "n", 500, "Maximum number of results")
	emojisListCmd.Flags().String("type", "all", "Filter by type: custom, alias, or all")

	emojisCmd.AddCommand(emojisSearchCmd)
	emojisCmd.AddCommand(emojisListCmd)
	rootCmd.AddCommand(emojisCmd)
}

// emojiEntry is the internal representation of one workspace emoji, with
// aliases resolved to the underlying image URL.
type emojiEntry struct {
	Name        string // raw name without colons, e.g. "fire"
	Type        string // "custom" or "alias"
	URL         string // image URL (for aliases this is the target's URL; may be "" if target missing)
	AliasTarget string // target emoji name (without colons) for aliases; "" for custom
}

// emojiResult is the JSON-facing record. Backward-compatible with the
// original `emojis search` output, with an added optional `url` field.
type emojiResult struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

// fetchEmojiEntries pulls the full emoji.list and returns typed entries with
// aliases resolved. Sorted by name.
func fetchEmojiEntries(client *api.Client, format string) ([]emojiEntry, error) {
	raw, err := fetchEmojisRaw(client, format)
	if err != nil {
		return nil, err
	}
	return parseEmojis(raw), nil
}

// fetchEmojisRaw fetches the emoji.list response and returns the
// name -> raw-value map (URL or "alias:target"). Used by callers that
// need the raw map.
func fetchEmojisRaw(client *api.Client, format string) (map[string]string, error) {
	spinner := progress.NewSpinner("Fetching emojis", format)
	spinner.Update(0)

	result, err := client.Call("emoji.list", nil)

	spinner.Finish()

	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return nil, fmt.Errorf("fetching emojis: %w", err)
	}

	return extractEmojiMap(result), nil
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

	emojis, err := fetchEmojisRaw(client, format)
	if err != nil {
		return err
	}
	if len(emojis) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	matched := filterEmojis(emojis, query, limit)

	if len(matched) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	if output.IsJSON(format) {
		if err := output.PrintJSON(matched); err != nil {
			return err
		}
	} else {
		renderEmojisTable(matched)
		fmt.Fprintf(os.Stderr, "Done. %d emojis found.\n", len(matched))
	}

	return nil
}

// runEmojisList fetches all custom emojis and renders them, optionally
// filtered by type.
func runEmojisList(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	typeFilter, _ := cmd.Flags().GetString("type")
	typeFilter = strings.ToLower(typeFilter)

	switch typeFilter {
	case "all", "custom", "alias":
	default:
		return fmt.Errorf("--type must be one of: all, custom, alias (got %q)", typeFilter)
	}

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	entries, err := fetchEmojiEntries(client, format)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	matched := selectEmojis(entries, typeFilter, limit)
	if len(matched) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no emojis found")
		}
		return nil
	}

	if output.IsJSON(format) {
		if err := output.PrintJSON(matched); err != nil {
			return err
		}
	} else {
		renderEmojisTable(matched)
		fmt.Fprintf(os.Stderr, "Done. %d emojis listed.\n", len(matched))
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

// parseEmojis converts the raw name->value map into typed entries with
// aliases resolved to the target's URL. The result is sorted by name.
func parseEmojis(emojis map[string]string) []emojiEntry {
	if len(emojis) == 0 {
		return nil
	}

	entries := make([]emojiEntry, 0, len(emojis))
	for name, value := range emojis {
		e := emojiEntry{Name: name}
		if strings.HasPrefix(value, "alias:") {
			e.Type = "alias"
			e.AliasTarget = strings.TrimPrefix(value, "alias:")
			e.URL = resolveAliasURL(emojis, e.AliasTarget)
		} else {
			e.Type = "custom"
			e.URL = value
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries
}

// resolveAliasURL follows an alias chain until it reaches a non-alias entry
// or a missing target. Walks at most 8 hops to avoid infinite loops.
func resolveAliasURL(emojis map[string]string, target string) string {
	const maxHops = 8
	for i := 0; i < maxHops; i++ {
		val, ok := emojis[target]
		if !ok {
			return ""
		}
		if strings.HasPrefix(val, "alias:") {
			target = strings.TrimPrefix(val, "alias:")
			continue
		}
		return val
	}
	return ""
}

// filterEmojis returns emoji results whose name contains the query as a
// case-insensitive substring, capped at limit results. Used by `emojis search`.
func filterEmojis(emojis map[string]string, query string, limit int) []emojiResult {
	queryLower := strings.ToLower(query)
	entries := parseEmojis(emojis)

	var matched []emojiResult
	for _, e := range entries {
		if !strings.Contains(strings.ToLower(e.Name), queryLower) {
			continue
		}
		matched = append(matched, toEmojiResult(e))
		if len(matched) >= limit {
			break
		}
	}
	return matched
}

// selectEmojis returns up to limit entries, filtered by type ("all",
// "custom", or "alias"). Input is assumed sorted by name.
func selectEmojis(entries []emojiEntry, typeFilter string, limit int) []emojiResult {
	results := make([]emojiResult, 0, len(entries))
	for _, e := range entries {
		if typeFilter != "all" && e.Type != typeFilter {
			continue
		}
		results = append(results, toEmojiResult(e))
		if len(results) >= limit {
			break
		}
	}
	return results
}

// toEmojiResult converts an internal entry into the JSON-facing record.
func toEmojiResult(e emojiEntry) emojiResult {
	r := emojiResult{
		Name: ":" + e.Name + ":",
		Type: e.Type,
		URL:  e.URL,
	}
	if e.Type == "alias" {
		r.Value = ":" + e.AliasTarget + ":"
	} else {
		r.Value = e.URL
	}
	return r
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
