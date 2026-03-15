package commands

import (
	"encoding/json"
	"fmt"
	"os"

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

// parseOutputFormat parses the output format from the persistent flag.
func parseOutputFormat() (output.Format, error) {
	return output.ParseFormat(OutputFormat)
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

// clearProgress clears the stderr progress line using ANSI escape (erase to end of line).
func clearProgress() {
	fmt.Fprintf(os.Stderr, "\r\033[K")
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
