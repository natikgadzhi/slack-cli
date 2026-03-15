// Package channels provides channel name resolution for the Slack CLI.
package channels

import (
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
	nameOrID = strings.TrimLeft(nameOrID, "#")

	if channelIDPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	var cursor string
	for {
		params := map[string]string{
			"limit":            "200",
			"exclude_archived": "true",
			"types":            "public_channel,private_channel,mpim,im",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := client.Call("conversations.list", params)
		if err != nil {
			return "", fmt.Errorf("listing channels: %w", err)
		}

		channels, _ := resp["channels"].([]any)
		for _, ch := range channels {
			chMap, ok := ch.(map[string]any)
			if !ok {
				continue
			}
			if name, _ := chMap["name"].(string); name == nameOrID {
				id, _ := chMap["id"].(string)
				if id != "" {
					return id, nil
				}
			}
		}

		// Check for next cursor in response_metadata.
		meta, _ := resp["response_metadata"].(map[string]any)
		nextCursor, _ := meta["next_cursor"].(string)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return "", fmt.Errorf("channel not found: %q", nameOrID)
}
