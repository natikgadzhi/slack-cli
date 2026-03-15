package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/output"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Slack messages",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().Int("count", 20, "maximum number of results")
	rootCmd.AddCommand(searchCmd)
}

// runSearch searches Slack messages and renders the results.
func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	count, _ := cmd.Flags().GetInt("count")

	format, err := parseOutputFormat()
	if err != nil {
		return err
	}

	// Set up client (no user resolver needed for search results).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Call search.messages.
	result, err := client.Call("search.messages", map[string]string{
		"query": query,
		"count": strconv.Itoa(count),
	})
	if err != nil {
		return fmt.Errorf("searching messages: %w", err)
	}

	// Extract matches from the nested messages.matches structure.
	matches := extractSearchMatches(result)
	if len(matches) == 0 {
		fmt.Fprintln(os.Stderr, "no results found")
		return nil
	}

	// Build result maps with the fields we want.
	results := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		r := make(map[string]any)

		if ts, ok := m["ts"].(string); ok {
			r["ts"] = ts
		}

		if ch, ok := m["channel"].(map[string]any); ok {
			if name, ok := ch["name"].(string); ok {
				r["channel"] = name
			}
		}

		if user, ok := m["username"].(string); ok && user != "" {
			r["user"] = user
		} else if user, ok := m["user"].(string); ok {
			r["user"] = user
		}

		if text, ok := m["text"].(string); ok {
			text = strings.TrimSpace(text)
			if runes := []rune(text); len(runes) > 500 {
				text = string(runes[:500])
			}
			r["text"] = text
		}

		if permalink, ok := m["permalink"].(string); ok {
			r["permalink"] = permalink
		}

		results = append(results, r)
	}

	// Render output.
	if err := output.RenderSearchResults(os.Stdout, results, format); err != nil {
		return err
	}

	// Cache the result (best-effort).
	cacheSlug := cache.SearchSlug(query)
	cacheWrite(getCache(), "search", cacheSlug, results, cache.Metadata{
		Command: fmt.Sprintf("search %q --count %d", query, count),
	})

	// Write per-item files if --output-dir is set.
	if OutputDir != "" {
		if err := writeSearchItemFiles(OutputDir, results, query); err != nil {
			return fmt.Errorf("writing output files: %w", err)
		}
	}

	return nil
}

// extractSearchMatches pulls the matches array from a search.messages response.
// The structure is: { "messages": { "matches": [...] } }
func extractSearchMatches(result map[string]any) []map[string]any {
	messagesRaw, ok := result["messages"]
	if !ok {
		return nil
	}
	messagesMap, ok := messagesRaw.(map[string]any)
	if !ok {
		return nil
	}
	return api.ExtractItems(messagesMap, "matches")
}
