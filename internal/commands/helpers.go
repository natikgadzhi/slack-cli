package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/output"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// setupClient creates an API client and user resolver from the stored credentials.
// This is the common setup pattern used by all data-fetching commands.
func setupClient() (*api.Client, *users.UserResolver, error) {
	xoxc, err := auth.GetXoxc()
	if err != nil {
		return nil, nil, fmt.Errorf("getting xoxc token: %w", err)
	}
	xoxd, err := auth.GetXoxd()
	if err != nil {
		return nil, nil, fmt.Errorf("getting xoxd cookie: %w", err)
	}
	client := api.NewClient(xoxc, xoxd)
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		return nil, nil, fmt.Errorf("creating user resolver: %w", err)
	}
	return client, resolver, nil
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
		// Cache errors are not fatal; log to stderr and continue without cache.
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

// clearProgress clears the stderr progress line by writing a carriage return
// and enough spaces to overwrite it, then returns to the start of the line.
func clearProgress() {
	fmt.Fprintf(os.Stderr, "\r%80s\r", "")
}

// extractMessagesFromResponse extracts the "messages" slice from a Slack API response.
func extractMessagesFromResponse(result map[string]any) []map[string]any {
	raw, ok := result["messages"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var messages []map[string]any
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			messages = append(messages, m)
		}
	}
	return messages
}

// isAPIError checks if an error is an api.APIError and returns it.
func isAPIError(err error) (*api.APIError, bool) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
