package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	internalOutput "github.com/natikgadzhi/slack-cli/internal/output"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Slack messages",
	Args:  validateSearchArgs,
	Example: `  slack-cli search "deployment failed" --limit 10
  slack-cli search --from @alice "deployment"
  slack-cli search --from @alice
  slack-cli search --from @alice --sort recent
  slack-cli search "from:@alice" -o json | jq '.[].text'`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 20, "Maximum number of results")
	searchCmd.Flags().String("from", "", "Filter messages from a specific user (handle or user ID)")
	searchCmd.Flags().String("sort", "relevance", "Sort order: relevance or recent")
	rootCmd.AddCommand(searchCmd)
}

// validateSearchArgs ensures that at least a query argument or the --from flag is provided.
func validateSearchArgs(cmd *cobra.Command, args []string) error {
	from, _ := cmd.Flags().GetString("from")
	if len(args) == 0 && from == "" {
		return fmt.Errorf("requires at least 1 arg or --from flag")
	}
	if len(args) > 1 {
		return fmt.Errorf("accepts at most 1 arg, received %d", len(args))
	}
	return nil
}

// runSearch searches Slack messages and renders the results.
func runSearch(cmd *cobra.Command, args []string) error {
	from, _ := cmd.Flags().GetString("from")
	sortFlag, _ := cmd.Flags().GetString("sort")
	limit, _ := cmd.Flags().GetInt("limit")

	var queryArg string
	if len(args) > 0 {
		queryArg = args[0]
	}

	query := buildSearchQuery(queryArg, from)
	sortParam, sortDir := resolveSearchSort(sortFlag, queryArg, from)

	format := output.Resolve(cmd)

	// Set up client (no user resolver needed for search results).
	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	// Show spinner while searching.
	spinner := progress.NewSpinner("Searching", format)
	spinner.Update(0)

	// Build API params.
	params := map[string]string{
		"query": query,
		"count": strconv.Itoa(limit),
	}
	if sortParam != "" {
		params["sort"] = sortParam
		params["sort_dir"] = sortDir
	}

	// Call search.messages.
	result, err := client.Call("search.messages", params)

	spinner.Finish()

	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("searching messages: %w", err)
	}

	// Extract matches from the nested messages.matches structure.
	matches := extractSearchMatches(result)
	if len(matches) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no results found")
		}
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
			r["text"] = formatting.TruncateRunes(strings.TrimSpace(text), 500)
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

	// Write per-item files if --derived flag was explicitly set.
	if derivedDir := resolveDerivedDir(cmd); derivedDir != "" {
		if err := writeSearchItemFiles(derivedDir, results, query); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	return nil
}

// buildSearchQuery constructs the final query string for search.messages.
// If from is set, it prepends a from: modifier to the query. If the from value
// starts with "@", the prefix is stripped. If it looks like a user ID (starts
// with "U" and is all alphanumeric), it is used as-is with from:<UID>.
func buildSearchQuery(queryArg, from string) string {
	var parts []string

	if from != "" {
		from = strings.TrimPrefix(from, "@")
		if looksLikeUserID(from) {
			parts = append(parts, "from:<"+from+">")
		} else {
			parts = append(parts, "from:"+from)
		}
	}

	if queryArg != "" {
		parts = append(parts, queryArg)
	}

	return strings.Join(parts, " ")
}

// looksLikeUserID returns true if s starts with "U" and the rest is alphanumeric.
// Slack user IDs have the form U[A-Z0-9]+.
func looksLikeUserID(s string) bool {
	if len(s) < 2 || s[0] != 'U' {
		return false
	}
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// resolveSearchSort determines the sort and sort_dir API params based on flags.
// When --from is used without a query, the default sort switches to "recent".
func resolveSearchSort(sortFlag, queryArg, from string) (sort, sortDir string) {
	effective := sortFlag

	// When --from is used without a query, default to recent sort
	// (unless the user explicitly chose a sort).
	if from != "" && queryArg == "" && effective == "relevance" {
		effective = "recent"
	}

	switch effective {
	case "recent":
		return "timestamp", "desc"
	default:
		// "relevance" is Slack's default; no need to send sort params.
		return "", ""
	}
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

// renderSearchTable renders search results as a table to stdout.
func renderSearchTable(results []map[string]any) {
	t := table.New()
	t.Header("CHANNEL", "TIME", "USER", "TEXT", "LINK")
	for _, r := range results {
		timeStr := internalOutput.FormatTS(getString(r, "ts"))
		t.Row(getString(r, "channel"), timeStr, getString(r, "user"), truncate(getString(r, "text"), 80), getString(r, "permalink"))
	}
	_ = t.Flush()
}
