// Package channels provides channel name resolution for the Slack CLI.
package channels

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// maxPages caps pagination as a safety valve against infinite cursor loops.
const maxPages = 100

// channelIDPattern matches Slack channel IDs: 8+ uppercase letters and digits.
var channelIDPattern = regexp.MustCompile(`^[A-Z0-9]{8,}$`)

// ResolveChannel resolves a channel name or ID to a Slack channel ID.
// If nameOrID looks like a channel ID (8+ uppercase alphanumeric chars), it is
// returned as-is. Otherwise, users.conversations is paginated to find a channel
// whose name matches. First tries active channels only; if not found, retries
// including archived channels.
//
// If progress is non-nil, pagination progress is written to it (intended for
// os.Stderr). Pass nil to suppress progress output.
// If verbose is true, each channel name encountered is logged to progress.
func ResolveChannel(client *api.Client, nameOrID string, progress io.Writer, verbose bool) (string, error) {
	nameOrID = strings.TrimPrefix(nameOrID, "#")

	if nameOrID == "" {
		return "", fmt.Errorf("channel name or ID required")
	}

	if channelIDPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	if progress != nil {
		client.OnRateLimit = func(endpoint string, delay time.Duration, attempt int) {
			_, _ = fmt.Fprintf(progress, "\r  [rate limited] waiting %s (retry %d)...", delay.Round(time.Millisecond), attempt+1)
		}
		defer func() { client.OnRateLimit = nil }()
	}

	// First pass: active channels only.
	id, err := paginateChannels(client, nameOrID, true, progress, verbose)
	if err != nil {
		clearProgress(progress)
		return "", err
	}
	if id != "" {
		clearProgress(progress)
		return id, nil
	}

	// Second pass: include archived channels.
	if progress != nil {
		_, _ = fmt.Fprintf(progress, "\rNot found in active channels, checking archived...")
	}
	id, err = paginateChannels(client, nameOrID, false, progress, verbose)
	if err != nil {
		clearProgress(progress)
		return "", err
	}
	if id != "" {
		clearProgress(progress)
		return id, nil
	}

	clearProgress(progress)
	return "", fmt.Errorf("channel not found: %q", nameOrID)
}

// paginateChannels pages through users.conversations looking for a channel
// matching name. If excludeArchived is true, archived channels are skipped.
// Returns the channel ID if found, empty string if not found, or an error.
func paginateChannels(client *api.Client, name string, excludeArchived bool, progress io.Writer, verbose bool) (string, error) {
	params := map[string]string{
		"limit": "200",
		"types": "public_channel,private_channel",
	}
	if excludeArchived {
		params["exclude_archived"] = "true"
	}

	pages := 0
	checked := 0

	for {
		result, err := client.Call("users.conversations", params)
		if err != nil {
			return "", fmt.Errorf("listing channels: %w", err)
		}

		pages++
		channels := api.ExtractItems(result, "channels")
		checked += len(channels)

		if progress != nil {
			_, _ = fmt.Fprintf(progress, "\rChecked %d channels across %d pages...", checked, pages)
		}

		if verbose && progress != nil {
			for _, ch := range channels {
				n, _ := ch["name"].(string)
				nn, _ := ch["name_normalized"].(string)
				id, _ := ch["id"].(string)
				_, _ = fmt.Fprintf(progress, "\n  [debug] %s  name=%q  name_normalized=%q", id, n, nn)
			}
			_, _ = fmt.Fprintf(progress, "\n")
		}

		if id := findChannelByName(channels, name); id != "" {
			return id, nil
		}

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" {
			return "", nil
		}

		if pages >= maxPages {
			return "", fmt.Errorf("channel %q not found after checking %d channels across %d pages (try using the channel ID instead)", name, checked, pages)
		}

		params["cursor"] = cursor
	}
}

// clearProgress clears the current progress line from stderr.
func clearProgress(w io.Writer) {
	if w != nil {
		_, _ = fmt.Fprintf(w, "\r\033[K")
	}
}

// findChannelByName searches a slice of channel maps for one whose "name"
// or "name_normalized" matches the given name, returning its "id" or empty
// string if not found.
func findChannelByName(channels []map[string]any, name string) string {
	for _, ch := range channels {
		n, _ := ch["name"].(string)
		nn, _ := ch["name_normalized"].(string)
		if strings.EqualFold(n, name) || strings.EqualFold(nn, name) {
			if id, _ := ch["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}
