// Package cache provides a Markdown-based caching layer with YAML frontmatter.
// Each cached object is stored as a Markdown file with structured metadata
// in the YAML frontmatter block.
//
// Frontmatter is rendered and parsed via cli-kit/derived.Render and
// derived.Parse. Slack-specific fields (channel, channel_id, user, thread_ts)
// are stored as extra keys in the generic map[string]any metadata.
package cache

import (
	"fmt"
	"time"

	"github.com/natikgadzhi/cli-kit/derived"
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

// MarshalFrontmatter encodes metadata and a Markdown body into a single
// byte slice with YAML frontmatter via cli-kit/derived.Render.
func MarshalFrontmatter(meta Metadata, body []byte) []byte {
	m := metadataToMap(meta)
	result := derived.Render(m, body)
	// Ensure trailing newline.
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}
	return result
}

// UnmarshalFrontmatter splits a Markdown file with YAML frontmatter into
// its Metadata and body via cli-kit/derived.Parse. Returns an error if the
// frontmatter block is missing or malformed.
func UnmarshalFrontmatter(data []byte) (Metadata, []byte, error) {
	rawMeta, body, err := derived.Parse(data)
	if err != nil {
		return Metadata{}, nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	if rawMeta == nil {
		return Metadata{}, nil, fmt.Errorf("missing opening frontmatter separator")
	}
	meta, err := mapToMetadata(rawMeta)
	if err != nil {
		return Metadata{}, nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	return meta, body, nil
}

// metadataToMap converts a Metadata struct to a map[string]any for
// cli-kit/derived.Render. Empty optional fields are omitted.
func metadataToMap(meta Metadata) map[string]any {
	m := map[string]any{}

	if meta.Tool != "" {
		m["tool"] = meta.Tool
	}
	if meta.ObjectType != "" {
		m["object_type"] = meta.ObjectType
	}
	if meta.Slug != "" {
		m["slug"] = meta.Slug
	}
	if !meta.CreatedAt.IsZero() {
		m["created_at"] = meta.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !meta.UpdatedAt.IsZero() {
		m["updated_at"] = meta.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if meta.SourceURL != "" {
		m["source_url"] = meta.SourceURL
	}
	if meta.Command != "" {
		m["command"] = meta.Command
	}
	if meta.Channel != "" {
		m["channel"] = meta.Channel
	}
	if meta.ChannelID != "" {
		m["channel_id"] = meta.ChannelID
	}
	if meta.User != "" {
		m["user"] = meta.User
	}
	if meta.ThreadTS != "" {
		m["thread_ts"] = meta.ThreadTS
	}

	return m
}

// mapToMetadata converts a map[string]any from derived.Parse back into
// a Metadata struct. Unknown keys are silently ignored.
func mapToMetadata(m map[string]any) (Metadata, error) {
	var meta Metadata

	if v, ok := m["tool"].(string); ok {
		meta.Tool = v
	}
	if v, ok := m["object_type"].(string); ok {
		meta.ObjectType = v
	}
	if v, ok := m["slug"].(string); ok {
		meta.Slug = v
	}
	if v, err := parseTimeField(m, "created_at"); err != nil {
		return Metadata{}, err
	} else if !v.IsZero() {
		meta.CreatedAt = v
	}
	if v, err := parseTimeField(m, "updated_at"); err != nil {
		return Metadata{}, err
	} else if !v.IsZero() {
		meta.UpdatedAt = v
	}
	if v, ok := m["source_url"].(string); ok {
		meta.SourceURL = v
	}
	if v, ok := m["command"].(string); ok {
		meta.Command = v
	}
	if v, ok := m["channel"].(string); ok {
		meta.Channel = v
	}
	if v, ok := m["channel_id"].(string); ok {
		meta.ChannelID = v
	}
	if v, ok := m["user"].(string); ok {
		meta.User = v
	}
	if v, ok := m["thread_ts"].(string); ok {
		meta.ThreadTS = v
	} else if f, ok := m["thread_ts"].(float64); ok {
		// yaml.v3 parses numeric-looking values as float64.
		meta.ThreadTS = fmt.Sprintf("%.6f", f)
	} else if i, ok := m["thread_ts"].(int); ok {
		meta.ThreadTS = fmt.Sprintf("%d.000000", i)
	}

	return meta, nil
}

// parseTimeField extracts a time.Time from the map. yaml.v3 may have
// already parsed it as time.Time, or it may be a string that needs
// manual parsing.
func parseTimeField(m map[string]any, key string) (time.Time, error) {
	raw, ok := m[key]
	if !ok {
		return time.Time{}, nil
	}
	switch v := raw.(type) {
	case time.Time:
		return v, nil
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid %s %q: %w", key, v, err)
		}
		return t, nil
	default:
		return time.Time{}, fmt.Errorf("invalid %s: unexpected type %T", key, raw)
	}
}
