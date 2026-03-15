// Package channels provides channel name resolution for the Slack CLI.
package channels

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// channelIDPattern matches Slack channel IDs: 8+ uppercase letters and digits.
var channelIDPattern = regexp.MustCompile(`^[A-Z0-9]{8,}$`)

// ResolveChannel resolves a channel name or ID to a Slack channel ID.
// If nameOrID looks like a channel ID (8+ uppercase alphanumeric chars), it is
// returned as-is. Otherwise, conversations.list is paginated to find a channel
// whose name matches.
func ResolveChannel(client *api.Client, nameOrID string) (string, error) {
	nameOrID = strings.TrimPrefix(nameOrID, "#")

	if nameOrID == "" {
		return "", fmt.Errorf("channel name or ID required")
	}

	if channelIDPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	params := map[string]string{
		"limit":            "200",
		"exclude_archived": "true",
		"types":            "public_channel,private_channel,mpim,im",
	}

	channels, err := client.CallPaginated("conversations.list", params, "next_cursor", "channels")
	if err != nil {
		// On rate limit, search partial results before giving up.
		var rlErr *api.RateLimitError
		if errors.As(err, &rlErr) {
			if id := findChannelByName(channels, nameOrID); id != "" {
				return id, nil
			}
		}
		return "", fmt.Errorf("listing channels: %w", err)
	}

	if id := findChannelByName(channels, nameOrID); id != "" {
		return id, nil
	}

	return "", fmt.Errorf("channel not found: %q", nameOrID)
}

// findChannelByName searches a slice of channel maps for one whose "name"
// matches the given name, returning its "id" or empty string if not found.
func findChannelByName(channels []map[string]any, name string) string {
	for _, ch := range channels {
		if n, _ := ch["name"].(string); n == name {
			if id, _ := ch["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}
