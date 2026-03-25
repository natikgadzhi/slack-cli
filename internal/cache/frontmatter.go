// Package cache provides a Markdown-based caching layer with YAML frontmatter.
// Each cached object is stored as a Markdown file with structured metadata
// in the YAML frontmatter block.
//
// The base frontmatter schema (tool, object_type, slug, timestamps) follows
// the cli-kit/derived.Frontmatter convention. This package extends it with
// Slack-specific fields (channel, channel_id, user, thread_ts).
package cache

import (
	"fmt"
	"strings"
	"time"
)

// Metadata holds the structured fields stored in YAML frontmatter.
type Metadata struct {
	Tool       string
	ObjectType string
	Slug       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SourceURL  string
	Command    string

	// Optional per-item fields (used by --derived file-per-item writes).
	Channel   string // resolved channel name
	ChannelID string // Slack channel ID
	User      string // message author display name
	ThreadTS  string // thread parent timestamp
}

// frontmatterSeparator is the YAML frontmatter delimiter.
const frontmatterSeparator = "---"

// MarshalFrontmatter encodes metadata and a Markdown body into a single
// byte slice with YAML frontmatter. The schema is small and fixed, so we
// use fmt.Fprintf rather than pulling in a YAML library.
func MarshalFrontmatter(meta Metadata, body []byte) []byte {
	var b strings.Builder

	b.WriteString(frontmatterSeparator)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "tool: %s\n", meta.Tool)
	fmt.Fprintf(&b, "object_type: %s\n", meta.ObjectType)
	fmt.Fprintf(&b, "slug: %s\n", meta.Slug)
	fmt.Fprintf(&b, "created_at: %s\n", meta.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "updated_at: %s\n", meta.UpdatedAt.UTC().Format(time.RFC3339))
	if meta.SourceURL != "" {
		fmt.Fprintf(&b, "source_url: %s\n", meta.SourceURL)
	}
	if meta.Command != "" {
		fmt.Fprintf(&b, "command: \"%s\"\n", meta.Command)
	}
	if meta.Channel != "" {
		fmt.Fprintf(&b, "channel: %s\n", meta.Channel)
	}
	if meta.ChannelID != "" {
		fmt.Fprintf(&b, "channel_id: %s\n", meta.ChannelID)
	}
	if meta.User != "" {
		fmt.Fprintf(&b, "user: %s\n", meta.User)
	}
	if meta.ThreadTS != "" {
		fmt.Fprintf(&b, "thread_ts: %s\n", meta.ThreadTS)
	}
	b.WriteString(frontmatterSeparator)
	b.WriteByte('\n')

	if len(body) > 0 {
		b.Write(body)
		// Ensure trailing newline.
		if body[len(body)-1] != '\n' {
			b.WriteByte('\n')
		}
	}

	return []byte(b.String())
}

// UnmarshalFrontmatter splits a Markdown file with YAML frontmatter into
// its Metadata and body. Returns an error if the frontmatter block is
// missing or malformed.
func UnmarshalFrontmatter(data []byte) (Metadata, []byte, error) {
	s := string(data)

	// Must start with "---\n".
	if !strings.HasPrefix(s, frontmatterSeparator+"\n") {
		return Metadata{}, nil, fmt.Errorf("missing opening frontmatter separator")
	}

	// Find the closing "---".
	rest := s[len(frontmatterSeparator)+1:]
	idx := strings.Index(rest, frontmatterSeparator+"\n")
	if idx < 0 {
		// Also accept "---" at EOF (no trailing newline after closing).
		if strings.HasSuffix(rest, frontmatterSeparator) {
			idx = len(rest) - len(frontmatterSeparator)
		} else {
			return Metadata{}, nil, fmt.Errorf("missing closing frontmatter separator")
		}
	}

	fmBlock := rest[:idx]

	// When the closing "---" is at the very end of the string (no trailing
	// newline), the body is empty. Guard against an out-of-bounds slice.
	var body []byte
	if afterClose := idx + len(frontmatterSeparator) + 1; afterClose <= len(rest) {
		body = []byte(rest[afterClose:])
	}

	meta, err := parseFrontmatter(fmBlock)
	if err != nil {
		return Metadata{}, nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	return meta, body, nil
}

// parseFrontmatter parses the key-value pairs between the --- delimiters.
func parseFrontmatter(block string) (Metadata, error) {
	var meta Metadata

	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes if present.
		value = stripQuotes(value)

		switch key {
		case "tool":
			meta.Tool = value
		case "object_type":
			meta.ObjectType = value
		case "slug":
			meta.Slug = value
		case "created_at":
			t, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return Metadata{}, fmt.Errorf("invalid created_at %q: %w", value, err)
			}
			meta.CreatedAt = t
		case "updated_at":
			t, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return Metadata{}, fmt.Errorf("invalid updated_at %q: %w", value, err)
			}
			meta.UpdatedAt = t
		case "source_url":
			meta.SourceURL = value
		case "command":
			meta.Command = value
		case "channel":
			meta.Channel = value
		case "channel_id":
			meta.ChannelID = value
		case "user":
			meta.User = value
		case "thread_ts":
			meta.ThreadTS = value
		}
	}

	return meta, nil
}

// stripQuotes removes a matching pair of double quotes from a string.
func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
