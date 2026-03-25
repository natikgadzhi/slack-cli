package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	"github.com/natikgadzhi/slack-cli/internal/output"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

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

// validateOutputDir checks that the output directory path is safe (no path traversal)
// and returns the cleaned absolute path.
func validateOutputDir(outputDir string) (string, error) {
	cleaned := filepath.Clean(outputDir)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("output-dir: invalid path %q: %w", outputDir, err)
	}
	// Reject paths that contain ".." components after cleaning.
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("output-dir: path %q contains path traversal", outputDir)
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
// under <outputDir>/slack/channels/<context>/<ts>.md.
// The context is channelName if non-empty, otherwise channelID.
func writeItemFiles(outputDir string, items []formatting.Message, channelID, channelName string) error {
	const objectType = "channels"
	absDir, err := validateOutputDir(outputDir)
	if err != nil {
		return err
	}

	context := channelName
	if context == "" {
		context = channelID
	}

	// Create a cache rooted at <outputDir>/slack/
	slackDir := filepath.Join(absDir, "slack")
	c, err := cache.NewCacheWithDir(slackDir)
	if err != nil {
		return fmt.Errorf("output-dir: create directory: %w", err)
	}

	for _, msg := range items {
		ts := sanitizeTS(msg.TS)
		if ts == "" {
			continue // skip messages without timestamps
		}

		body, err := renderSingleMarkdown(msg)
		if err != nil {
			return fmt.Errorf("output-dir: render message %s: %w", ts, err)
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
			return fmt.Errorf("output-dir: write %s: %w", ts, err)
		}
	}

	return nil
}

// writeThreadFile writes all messages in a thread as a single markdown file
// under <outputDir>/slack/messages/<channelID>/<threadTS>.md.
func writeThreadFile(outputDir string, items []formatting.Message, channelID, channelName, threadTS, sourceURL string) error {
	absDir, err := validateOutputDir(outputDir)
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
		return fmt.Errorf("output-dir: create directory: %w", err)
	}

	body, err := renderMultipleMarkdown(items)
	if err != nil {
		return fmt.Errorf("output-dir: render thread: %w", err)
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
		return fmt.Errorf("output-dir: write thread %s: %w", ts, err)
	}

	return nil
}

// writeSearchItemFiles writes each search result as its own markdown file
// under <outputDir>/slack/search/<queryHash>/<ts>.md.
func writeSearchItemFiles(outputDir string, results []map[string]any, query string) error {
	absDir, err := validateOutputDir(outputDir)
	if err != nil {
		return err
	}

	slackDir := filepath.Join(absDir, "slack")
	c, err := cache.NewCacheWithDir(slackDir)
	if err != nil {
		return fmt.Errorf("output-dir: create directory: %w", err)
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
			return fmt.Errorf("output-dir: render search result %s: %w", ts, err)
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
			return fmt.Errorf("output-dir: write search result %s: %w", ts, err)
		}
	}

	return nil
}
