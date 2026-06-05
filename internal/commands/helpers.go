package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	"github.com/natikgadzhi/slack-cli/internal/output"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// teamURLResult holds the result of a concurrent GetTeamURL call.
type teamURLResult struct {
	url string
	err error
}

// fetchTeamURLAsync starts a goroutine to fetch the team URL and returns
// a channel that will receive the result. This is used by commands that
// want to overlap the team URL fetch with other API calls.
func fetchTeamURLAsync(client *api.Client) <-chan teamURLResult {
	ch := make(chan teamURLResult, 1)
	go func() {
		u, err := client.GetTeamURL()
		ch <- teamURLResult{u, err}
	}()
	return ch
}

// resolveDerivedDir returns the derived directory path if the --derived flag was
// explicitly set on the command line. Returns "" if the flag was not set,
// preserving the original behavior of only writing derived files on explicit request.
func resolveDerivedDir(cmd *cobra.Command) string {
	f := cmd.Flags().Lookup("derived")
	if f == nil {
		f = cmd.PersistentFlags().Lookup("derived")
	}
	if f != nil && f.Changed {
		return f.Value.String()
	}
	return ""
}

// setupClientOnly creates an API client from stored credentials without a user resolver.
// Used by commands that don't need user resolution (e.g. search).
func setupClientOnly() (*api.Client, error) {
	xoxc, err := auth.GetXoxc()
	if err != nil {
		return nil, fmt.Errorf("getting xoxc token: %w", err)
	}
	xoxd, err := auth.GetXoxd()
	if err != nil {
		return nil, fmt.Errorf("getting xoxd cookie: %w", err)
	}
	return api.NewClient(xoxc, xoxd), nil
}

// setupClient creates an API client and user resolver from the stored credentials.
func setupClient() (*api.Client, *users.UserResolver, error) {
	client, err := setupClientOnly()
	if err != nil {
		return nil, nil, err
	}
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		return nil, nil, fmt.Errorf("creating user resolver: %w", err)
	}
	return client, resolver, nil
}

// getCache returns a cache instance if caching is enabled, or nil if --no-cache is set.
func getCache() *cache.Cache {
	if NoCache {
		return nil
	}
	c, err := cache.NewCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cache unavailable: %v\n", err)
		return nil
	}
	return c
}

// cacheWrite is a best-effort cache write. Errors are logged to stderr, not returned.
func cacheWrite(c *cache.Cache, objectType, slug string, data any, meta cache.Metadata) {
	if c == nil {
		return
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cache encode failed: %v\n", err)
		return
	}
	if err := c.Put(objectType, slug, content, meta); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cache write failed: %v\n", err)
	}
}

// formatMessages converts raw Slack messages to formatted Messages with permalinks.
func formatMessages(messages []map[string]any, teamURL, channelID string, hasTeamURL bool) []formatting.Message {
	formatted := make([]formatting.Message, 0, len(messages))
	for _, m := range messages {
		msg := formatting.FormatMessage(m)
		if hasTeamURL {
			if ts, ok := m["ts"].(string); ok && ts != "" {
				msg.Link = formatting.BuildPermalink(teamURL, channelID, ts)
			}
		}
		formatted = append(formatted, msg)
	}
	return formatted
}

// downloadMessageFiles downloads file attachments for all messages in the slice.
// Files are saved to dir/<sanitized-name>, and each File's LocalPath is updated
// with the path on disk. Errors are logged as warnings, not returned.
func downloadMessageFiles(client *api.Client, messages []formatting.Message, dir string) {
	for i := range messages {
		for j := range messages[i].Files {
			f := &messages[i].Files[j]
			if f.URL == "" {
				continue
			}
			dest := filepath.Join(dir, sanitizeFilename(f.Name))
			if err := client.DownloadFile(f.URL, dest); err != nil {
				fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", f.Name, err)
				continue
			}
			f.LocalPath = dest
		}
		for j := range messages[i].Replies {
			for k := range messages[i].Replies[j].Files {
				f := &messages[i].Replies[j].Files[k]
				if f.URL == "" {
					continue
				}
				dest := filepath.Join(dir, sanitizeFilename(f.Name))
				if err := client.DownloadFile(f.URL, dest); err != nil {
					fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", f.Name, err)
					continue
				}
				f.LocalPath = dest
			}
		}
	}
}

// sanitizeFilename removes path separators and null bytes from a filename.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "" {
		name = "unnamed"
	}
	return name
}

// validateDerivedDir checks that the derived directory path is safe (no path traversal)
// and returns the cleaned absolute path.
func validateDerivedDir(dir string) (string, error) {
	cleaned := filepath.Clean(dir)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("derived: invalid path %q: %w", dir, err)
	}
	// Reject paths that contain ".." components after cleaning.
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("derived: path %q contains path traversal", dir)
		}
	}
	return abs, nil
}

// sanitizeTS sanitizes a Slack timestamp for use as a filename.
// Dots are kept; slashes and other problematic characters are removed.
func sanitizeTS(ts string) string {
	ts = strings.ReplaceAll(ts, "/", "")
	ts = strings.ReplaceAll(ts, "\\", "")
	ts = strings.ReplaceAll(ts, "\x00", "")
	return ts
}

// renderSingleMarkdown renders a single message to markdown bytes.
func renderSingleMarkdown(msg formatting.Message) ([]byte, error) {
	var buf bytes.Buffer
	if err := output.RenderSingle(&buf, msg, output.Markdown); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderMultipleMarkdown renders multiple messages to markdown bytes.
func renderMultipleMarkdown(msgs []formatting.Message) ([]byte, error) {
	var buf bytes.Buffer
	if err := output.RenderMessages(&buf, msgs, output.Markdown); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeItemFiles writes each message as its own markdown file with frontmatter
// under <derivedDir>/slack/channels/<context>/<ts>.md.
// The context is channelName if non-empty, otherwise channelID.
func writeItemFiles(derivedDir string, items []formatting.Message, channelID, channelName string) error {
	const objectType = "channels"
	absDir, err := validateDerivedDir(derivedDir)
	if err != nil {
		return err
	}

	context := channelName
	if context == "" {
		context = channelID
	}

	// Create a cache rooted at <derivedDir>/slack/
	slackDir := filepath.Join(absDir, "slack")
	c, err := cache.NewCacheWithDir(slackDir)
	if err != nil {
		return fmt.Errorf("derived: create directory: %w", err)
	}

	for _, msg := range items {
		ts := sanitizeTS(msg.TS)
		if ts == "" {
			continue // skip messages without timestamps
		}

		body, err := renderSingleMarkdown(msg)
		if err != nil {
			return fmt.Errorf("derived: render message %s: %w", ts, err)
		}

		slug := filepath.Join(context, ts)
		meta := cache.Metadata{
			ObjectType: objectType,
			Slug:       slug,
			SourceURL:  msg.Link,
			Channel:    channelName,
			ChannelID:  channelID,
			User:       msg.User,
		}

		if err := c.PutItem(objectType, slug, body, meta); err != nil {
			return fmt.Errorf("derived: write %s: %w", ts, err)
		}
	}

	return nil
}

// writeThreadFile writes all messages in a thread as a single markdown file
// under <derivedDir>/slack/messages/<channelID>/<threadTS>.md.
func writeThreadFile(derivedDir string, items []formatting.Message, channelID, channelName, threadTS, sourceURL string) error {
	absDir, err := validateDerivedDir(derivedDir)
	if err != nil {
		return err
	}

	context := channelName
	if context == "" {
		context = channelID
	}

	slackDir := filepath.Join(absDir, "slack")
	c, err := cache.NewCacheWithDir(slackDir)
	if err != nil {
		return fmt.Errorf("derived: create directory: %w", err)
	}

	body, err := renderMultipleMarkdown(items)
	if err != nil {
		return fmt.Errorf("derived: render thread: %w", err)
	}

	ts := sanitizeTS(threadTS)
	slug := filepath.Join(context, ts)

	// Use the first message's user as the thread author if available.
	var user string
	if len(items) > 0 {
		user = items[0].User
	}

	meta := cache.Metadata{
		ObjectType: "message",
		Slug:       slug,
		SourceURL:  sourceURL,
		Channel:    channelName,
		ChannelID:  channelID,
		User:       user,
		ThreadTS:   threadTS,
	}

	if err := c.PutItem("messages", slug, body, meta); err != nil {
		return fmt.Errorf("derived: write thread %s: %w", ts, err)
	}

	return nil
}

// --- Shared type-extraction helpers ---
//
// These extract typed values from map[string]any (the shape Slack's JSON API
// responses are decoded into). They're used across multiple command files.

// getString safely extracts a string field from a map, returning "" if missing.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getBool safely extracts a boolean field from a map, returning false if missing.
func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// toInt converts an any value to int, handling both float64 (JSON default) and int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

// toFloat converts an any value to float64, handling float64 and int.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// extractStringSlice extracts a []string from result[key], where the API
// returns an array of strings (e.g. conversations.members returns
// {"members": ["U1", "U2"]}). The key parameter allows reuse across different
// API responses that return string arrays under different field names.
func extractStringSlice(result map[string]any, key string) []string { //nolint:unparam
	raw, ok := result[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	strs := make([]string, 0, len(arr))
	for _, elem := range arr {
		if s, ok := elem.(string); ok {
			strs = append(strs, s)
		}
	}
	return strs
}

// --- Argument validation helpers ---

// exactlyOneArg returns a cobra.PositionalArgs validator that produces a
// helpful error message (with usage and examples) when the user passes zero
// arguments, and a concise "too many arguments" message otherwise.
func exactlyOneArg(noun, usage string, examples ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			msg := fmt.Sprintf("%s requires %s\n\nUsage:\n  %s", cmd.CommandPath(), noun, usage)
			if len(examples) > 0 {
				msg += "\n\nExamples:"
				for _, ex := range examples {
					msg += "\n  " + ex
				}
			}
			return fmt.Errorf("%s", msg)
		}
		if len(args) > 1 {
			return fmt.Errorf("%s accepts 1 argument, got %d", cmd.CommandPath(), len(args))
		}
		return nil
	}
}

// --- Shared input-parsing helpers ---

// parseMessageInput resolves a channel ID and message timestamp from either a
// positional URL argument or --channel/--ts flags. This is the shared input
// validator for commands that target a single message (message, reactions get).
//
// When the --channel flag is a name rather than an ID, the caller is
// responsible for resolving it to an ID afterward (via channels.ResolveChannel).
func parseMessageInput(args []string, channelFlag, tsFlag string) (channelID, messageTS string, err error) {
	hasURL := len(args) == 1
	hasChannel := channelFlag != ""
	hasTS := tsFlag != ""

	switch {
	case hasURL && (hasChannel || hasTS):
		return "", "", fmt.Errorf("cannot combine a positional URL with --channel/--ts flags")
	case hasURL:
		cid, mts, threadTS, parseErr := formatting.ParseSlackURL(args[0])
		if parseErr != nil {
			return "", "", fmt.Errorf("parsing URL: %w", parseErr)
		}
		ts := mts
		if threadTS != "" {
			ts = threadTS
		}
		return cid, ts, nil
	case hasChannel && hasTS:
		return channelFlag, tsFlag, nil
	case hasChannel || hasTS:
		return "", "", fmt.Errorf("--channel and --ts must be provided together")
	default:
		return "", "", fmt.Errorf("provide a message URL or --channel and --ts")
	}
}

// --- Shared key-value table rendering ---

// kvField describes a single row in a key-value table.
type kvField struct {
	Key   string // map key to look up the value
	Label string // human-readable label for the KEY column
}

// renderKeyValueTable renders a map as a two-column KEY/VALUE table using the
// supplied field list. All fields are shown including false/empty values; use
// this for profile-style output where boolean fields should be visible.
func renderKeyValueTable(data map[string]any, fields []kvField) {
	t := table.New()
	t.Header("KEY", "VALUE")

	for _, f := range fields {
		t.Row(f.Label, fmt.Sprintf("%v", data[f.Key]))
	}

	_ = t.Flush()
}

// writeSearchItemFiles writes each search result as its own markdown file
// under <derivedDir>/slack/search/<queryHash>/<ts>.md.
func writeSearchItemFiles(derivedDir string, results []map[string]any, query string) error {
	absDir, err := validateDerivedDir(derivedDir)
	if err != nil {
		return err
	}

	slackDir := filepath.Join(absDir, "slack")
	c, err := cache.NewCacheWithDir(slackDir)
	if err != nil {
		return fmt.Errorf("derived: create directory: %w", err)
	}

	queryHash := cache.SearchSlug(query)

	for _, r := range results {
		ts, _ := r["ts"].(string)
		ts = sanitizeTS(ts)
		if ts == "" {
			continue
		}

		// Render the search result as markdown.
		var buf bytes.Buffer
		if err := output.RenderSearchResults(&buf, []map[string]any{r}, output.Markdown); err != nil {
			return fmt.Errorf("derived: render search result %s: %w", ts, err)
		}

		slug := filepath.Join(queryHash, ts)
		channel, _ := r["channel"].(string)
		user, _ := r["user"].(string)
		permalink, _ := r["permalink"].(string)

		meta := cache.Metadata{
			ObjectType: "search",
			Slug:       slug,
			SourceURL:  permalink,
			Channel:    channel,
			User:       user,
		}

		if err := c.PutItem("search", slug, buf.Bytes(), meta); err != nil {
			return fmt.Errorf("derived: write search result %s: %w", ts, err)
		}
	}

	return nil
}
