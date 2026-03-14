package formatting

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseSlackURL parses a Slack message permalink into its components.
//
// URL formats:
//
//	.../archives/C12345678/p1741234567123456
//	.../archives/C12345678/p1741234567123456?thread_ts=1741234560.123456&cid=C12345678
//
// Returns channelID, messageTS, threadTS (empty string if absent), and any error.
func ParseSlackURL(rawURL string) (channelID, messageTS, threadTS string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("unrecognized Slack URL: %s", rawURL)
	}

	// Split path into non-empty segments.
	var parts []string
	for _, p := range strings.Split(parsed.Path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}

	// Find "archives" segment and extract channel + raw timestamp.
	archivesIdx := -1
	for i, p := range parts {
		if p == "archives" {
			archivesIdx = i
			break
		}
	}
	if archivesIdx < 0 || archivesIdx+2 >= len(parts) {
		return "", "", "", fmt.Errorf("unrecognized Slack URL: %s", rawURL)
	}

	channelID = parts[archivesIdx+1]
	rawTS := parts[archivesIdx+2]

	// Validate timestamp format: starts with 'p' and at least 12 chars long.
	if !strings.HasPrefix(rawTS, "p") || len(rawTS) < 12 {
		return "", "", "", fmt.Errorf("unrecognized message timestamp format: %q", rawTS)
	}

	digits := rawTS[1:]
	messageTS = digits[:10] + "." + digits[10:]

	// Extract thread_ts from query string if present.
	qs := parsed.Query()
	if v := qs.Get("thread_ts"); v != "" {
		threadTS = v
	}

	return channelID, messageTS, threadTS, nil
}
