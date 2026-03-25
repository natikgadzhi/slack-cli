package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Slack messages",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli search "deployment failed" --limit 10
  slack-cli search "from:@alice" -o json | jq '.[].text'`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 20, "Maximum number of results")
	rootCmd.AddCommand(searchCmd)
}

// runSearch searches Slack messages and renders the results.
func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")

	format := output.Resolve(cmd)

	// Set up client (no user resolver needed for search results).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Show spinner while searching.
	spinner := progress.NewSpinner("Searching", format)
	spinner.Update(0)

	// Call search.messages.
	result, err := client.Call("search.messages", map[string]string{
		"query": query,
		"count": strconv.Itoa(limit),
	})

	spinner.Finish()

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
	if output.IsJSON(format) {
		if err := output.PrintJSON(results); err != nil {
			return err
		}
	} else {
		renderSearchTable(results)
	}

	// Cache the result (best-effort).
	cacheSlug := cache.SearchSlug(query)
	cacheWrite(getCache(), "search", cacheSlug, results, cache.Metadata{
		Command: fmt.Sprintf("search %q --limit %d", query, limit),
	})

	// Write per-item files if --derived is set.
	if DerivedDir != "" {
		if err := writeSearchItemFiles(DerivedDir, results, query); err != nil {
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
