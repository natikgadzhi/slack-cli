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
	"github.com/natikgadzhi/slack-cli/internal/users"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Slack messages",
	Args:  validateSearchArgs,
	Example: `  slack-cli search "deployment failed" --limit 10
  slack-cli search --from @alice "deployment"
  slack-cli search --from @alice
  slack-cli search --from @alice --sort recent
  slack-cli search "deployment failed" --context 3
  slack-cli search "from:@alice" -o json | jq '.[].text'`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 20, "Maximum number of results")
	searchCmd.Flags().String("from", "", "Filter messages from a specific user (handle or user ID)")
	searchCmd.Flags().String("sort", "relevance", "Sort order: relevance or recent")
	searchCmd.Flags().IntP("context", "C", 0, "Number of surrounding messages to fetch for each hit")
	_ = searchCmd.RegisterFlagCompletionFunc("from", completeUserHandles)
	_ = searchCmd.RegisterFlagCompletionFunc("sort", staticCompletion("relevance", "recent"))
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
	contextN, _ := cmd.Flags().GetInt("context")

	var queryArg string
	if len(args) > 0 {
		queryArg = args[0]
	}

	query := buildSearchQuery(queryArg, from)
	sortParam, sortDir := resolveSearchSort(sortFlag, queryArg, from)

	format := output.Resolve(cmd)

	// Set up client and user resolver. The resolver is used to turn the
	// partner user ID that 1:1 DM hits carry in their channel name into a
	// readable display name.
	client, resolver, err := setupClient()
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
			r["channel"] = searchChannelLabel(ch, resolver)
			// Store channel ID for context fetching.
			if chID, ok := ch["id"].(string); ok {
				r["channel_id"] = chID
			}
		}

		if user, ok := m["username"].(string); ok && user != "" {
			r["user"] = user
		} else if user, ok := m["user"].(string); ok {
			r["user"] = user
		}

		if text, ok := m["text"].(string); ok {
			text = formatting.UnescapeEntities(strings.TrimSpace(text))
			r["text"] = text
		}

		if permalink, ok := m["permalink"].(string); ok {
			r["permalink"] = permalink
		}

		results = append(results, r)
	}

	// Fetch surrounding context messages when --context N > 0.
	if contextN > 0 {
		fetchSearchContext(client, resolver, results, contextN, format)
	}

	// Render output.
	if output.IsJSON(format) {
		// Strip internal channel_id field before rendering JSON.
		cleaned := cleanSearchResultsForJSON(results)
		if err := output.PrintJSON(cleaned); err != nil {
			return err
		}
	} else {
		if contextN > 0 {
			renderSearchTableWithContext(results)
		} else {
			renderSearchTable(results)
		}
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

// looksLikeUserID returns true if s starts with "U" or "W" and the rest is
// alphanumeric. Slack user IDs have the form U[A-Z0-9]+ or W[A-Z0-9]+
// (enterprise grid).
func looksLikeUserID(s string) bool {
	if len(s) < 2 || (s[0] != 'U' && s[0] != 'W') {
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

// searchChannelLabel returns a human-readable label for a search hit's
// conversation. For 1:1 DMs the Slack API reports the partner's user ID in both
// the channel name and the channel "user" field, so we resolve it to a display
// name prefixed with "@". Regular channels and group DMs already carry a
// readable name (e.g. "general", "mpdm-...") and are returned as-is.
func searchChannelLabel(ch map[string]any, resolver *users.UserResolver) string {
	name, _ := ch["name"].(string)

	if isIM, _ := ch["is_im"].(bool); isIM {
		uid, _ := ch["user"].(string)
		if uid == "" {
			uid = name
		}
		if uid != "" {
			return "@" + resolver.DisplayName(uid)
		}
	}

	return name
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

// renderSearchTable renders search results as a table to stdout. Rather than a
// LINK column (whose long URL would be truncated), the fixed-width TIME cell is
// rendered as an OSC-8 hyperlink to the message permalink, so it stays clickable
// without ever being clipped.
func renderSearchTable(results []map[string]any) {
	t := table.New()
	t.Header("CHANNEL", "TIME", "USER", "TEXT")
	for _, r := range results {
		timeCell := internalOutput.FormatTS(getString(r, "ts"))
		if link := getString(r, "permalink"); link != "" {
			timeCell = table.Hyperlink(link, timeCell)
		}
		t.Row(getString(r, "channel"), timeCell, getString(r, "user"), truncate(getString(r, "text"), 80))
	}
	_ = t.Flush()
}

// fetchSearchContext fetches surrounding messages for each search hit using
// conversations.history. For each hit, it makes two API calls:
//   - Before: conversations.history with latest=<hit_ts>, limit=N, inclusive=false
//   - After: conversations.history with oldest=<hit_ts>, limit=N+1, inclusive=false
//
// Context messages are stored on each result as "context_before" and "context_after"
// arrays. Rate-limit errors are warned about, not fatal.
func fetchSearchContext(client *api.Client, resolver *users.UserResolver, results []map[string]any, n int, format string) {
	counter := progress.NewCounter("Fetching context", format)

	for i, r := range results {
		counter.Update(i + 1)

		channelID := getString(r, "channel_id")
		hitTS := getString(r, "ts")
		if channelID == "" || hitTS == "" {
			continue
		}

		// Fetch N messages before the hit (newest first, so reverse for chronological order).
		beforeMsgs := fetchContextMessages(client, channelID, hitTS, n, true)

		// Fetch N messages after the hit.
		afterMsgs := fetchContextMessages(client, channelID, hitTS, n, false)

		// Resolve user IDs in context messages.
		if len(beforeMsgs) > 0 {
			resolved, err := resolver.ResolveUsers(beforeMsgs)
			if err == nil {
				beforeMsgs = resolved
			}
		}
		if len(afterMsgs) > 0 {
			resolved, err := resolver.ResolveUsers(afterMsgs)
			if err == nil {
				afterMsgs = resolved
			}
		}

		// Build context result slices.
		if len(beforeMsgs) > 0 {
			before := make([]map[string]any, 0, len(beforeMsgs))
			for _, m := range beforeMsgs {
				before = append(before, contextMessageToResult(m))
			}
			results[i]["context_before"] = before
		}
		if len(afterMsgs) > 0 {
			after := make([]map[string]any, 0, len(afterMsgs))
			for _, m := range afterMsgs {
				after = append(after, contextMessageToResult(m))
			}
			results[i]["context_after"] = after
		}
	}

	counter.Finish()
}

// fetchContextMessages calls conversations.history to get messages around a
// given timestamp. If before is true, it fetches messages older than ts
// (returned in chronological order). If before is false, it fetches messages
// newer than ts.
func fetchContextMessages(client *api.Client, channelID, ts string, n int, before bool) []map[string]any {
	params := map[string]string{
		"channel":   channelID,
		"limit":     strconv.Itoa(n),
		"inclusive": "false",
	}

	if before {
		// Get messages before the hit: latest=ts means "messages older than ts".
		params["latest"] = ts
	} else {
		// Get messages after the hit: oldest=ts means "messages newer than ts".
		params["oldest"] = ts
	}

	result, err := client.Call("conversations.history", params)
	if err != nil {
		// Warn on rate limit or other errors; don't fail the whole command.
		if _, ok := api.AsRateLimitError(err); ok {
			fmt.Fprintf(os.Stderr, "warning: rate limited fetching context for %s; skipping\n", ts)
		} else {
			fmt.Fprintf(os.Stderr, "warning: fetching context for %s: %v\n", ts, err)
		}
		return nil
	}

	messages := api.ExtractItems(result, "messages")

	if before {
		// conversations.history returns newest first; reverse for chronological order.
		reverseSlice(messages)
	}

	return messages
}

// contextMessageToResult converts a raw conversations.history message into a
// result map with ts, user, and text fields, matching the shape of search results.
func contextMessageToResult(m map[string]any) map[string]any {
	r := make(map[string]any)
	if ts, ok := m["ts"].(string); ok {
		r["ts"] = ts
	}
	if user, ok := m["user"].(string); ok {
		r["user"] = user
	}
	if text, ok := m["text"].(string); ok {
		text = formatting.UnescapeEntities(strings.TrimSpace(text))
		r["text"] = text
	}
	return r
}

// reverseSlice reverses a slice of maps in place.
func reverseSlice(s []map[string]any) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// renderSearchTableWithContext renders search results with surrounding context
// messages. The hit is marked with a ">" prefix, context messages with "|".
// Groups are separated by a blank row.
func renderSearchTableWithContext(results []map[string]any) {
	t := table.New()
	t.Header("CHANNEL", "TIME", "USER", "TEXT")

	for i, r := range results {
		channel := getString(r, "channel")

		// Render context_before messages.
		if before, ok := r["context_before"].([]map[string]any); ok {
			for _, ctx := range before {
				timeCell := internalOutput.FormatTS(getString(ctx, "ts"))
				t.Row(channel, timeCell, getString(ctx, "user"), truncate("| "+getString(ctx, "text"), 80))
			}
		}

		// Render the hit itself, marked with ">".
		timeCell := internalOutput.FormatTS(getString(r, "ts"))
		if link := getString(r, "permalink"); link != "" {
			timeCell = table.Hyperlink(link, timeCell)
		}
		t.Row(channel, timeCell, getString(r, "user"), truncate("> "+getString(r, "text"), 80))

		// Render context_after messages.
		if after, ok := r["context_after"].([]map[string]any); ok {
			for _, ctx := range after {
				timeCell := internalOutput.FormatTS(getString(ctx, "ts"))
				t.Row(channel, timeCell, getString(ctx, "user"), truncate("| "+getString(ctx, "text"), 80))
			}
		}

		// Add separator between groups (blank row), except after the last.
		if i < len(results)-1 {
			t.Row("", "", "", "")
		}
	}

	_ = t.Flush()
}

// cleanSearchResultsForJSON removes internal fields (channel_id) from results
// and returns a cleaned copy suitable for JSON output.
func cleanSearchResultsForJSON(results []map[string]any) []map[string]any {
	cleaned := make([]map[string]any, 0, len(results))
	for _, r := range results {
		c := make(map[string]any, len(r))
		for k, v := range r {
			if k == "channel_id" {
				continue
			}
			c[k] = v
		}
		cleaned = append(cleaned, c)
	}
	return cleaned
}
