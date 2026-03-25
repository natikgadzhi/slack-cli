// Package channels provides channel name resolution for the Slack CLI.
package channels

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// maxPages is the maximum number of pages to paginate through when resolving
// a channel name. At 200 channels per page, this covers 4000 channels.
const maxPages = 20

// channelIDPattern matches Slack channel IDs: 8+ uppercase letters and digits.
var channelIDPattern = regexp.MustCompile(`^[A-Z0-9]{8,}$`)

// ResolveChannel resolves a channel name or ID to a Slack channel ID.
// If nameOrID looks like a channel ID (8+ uppercase alphanumeric chars), it is
// returned as-is. Otherwise, conversations.list is paginated to find a channel
// whose name matches. Pagination stops as soon as the channel is found,
// avoiding unnecessary API calls in large workspaces.
//
// If progress is non-nil, pagination progress is written to it (intended for
// os.Stderr). Pass nil to suppress progress output.
func ResolveChannel(client *api.Client, nameOrID string, progress io.Writer) (string, error) {
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
		"types":            "public_channel,private_channel",
	}

	pages := 0
	checked := 0

	for {
		result, err := client.Call("conversations.list", params)
		if err != nil {
			return "", fmt.Errorf("listing channels: %w", err)
		}

		pages++
		channels := api.ExtractItems(result, "channels")
		checked += len(channels)

		if progress != nil {
			fmt.Fprintf(progress, "\rChecked %d channels across %d pages...", checked, pages)
		}

		if id := findChannelByName(channels, nameOrID); id != "" {
			if progress != nil {
				fmt.Fprintf(progress, "\n")
			}
			return id, nil
		}

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" {
			break
		}

		if pages >= maxPages {
			if progress != nil {
				fmt.Fprintf(progress, "\n")
			}
			return "", fmt.Errorf("channel %q not found after checking %d channels across %d pages (workspace may be too large — try using the channel ID instead)", nameOrID, checked, pages)
		}

		params["cursor"] = cursor
	}

	if progress != nil {
		fmt.Fprintf(progress, "\n")
	}

	return "", fmt.Errorf("channel not found: %q", nameOrID)
}

// findChannelByName searches a slice of channel maps for one whose "name"
// matches the given name, returning its "id" or empty string if not found.
func findChannelByName(channels []map[string]any, name string) string {
	for _, ch := range channels {
		if n, _ := ch["name"].(string); strings.EqualFold(n, name) {
			if id, _ := ch["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}
