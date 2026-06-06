package formatting

import (
	"strconv"
	"time"
)

// PinnedItem represents a pinned item in a Slack channel.
type PinnedItem struct {
	Type      string `json:"type"`                 // "message", "file", or "channel" (canvas)
	CreatedBy string `json:"created_by"`           // user who pinned it
	Created   string `json:"created,omitempty"`    // human-readable timestamp
	CreatedTS int64  `json:"created_ts,omitempty"` // raw unix timestamp

	// For pinned messages:
	Text      string `json:"text,omitempty"`
	MessageTS string `json:"message_ts,omitempty"`

	// For pinned files/canvases:
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

// Bookmark represents a channel bookmark (tab).
type Bookmark struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"` // "link", "file", etc.
	Link  string `json:"link,omitempty"`
	Emoji string `json:"emoji,omitempty"`
}

// ChannelMetadata wraps pinned items and bookmarks for a channel.
// It is included in the JSON output when --with-pins is set.
type ChannelMetadata struct {
	PinnedItems []PinnedItem `json:"pinned_items,omitempty"`
	Bookmarks   []Bookmark   `json:"bookmarks,omitempty"`
	Messages    []Message    `json:"messages"`
	Warning     string       `json:"warning,omitempty"`
}

// FormatPinnedItem converts a raw Slack pins.list item into a PinnedItem.
func FormatPinnedItem(raw map[string]any) PinnedItem {
	pin := PinnedItem{}

	if t, ok := raw["type"].(string); ok {
		pin.Type = t
	}

	if cb, ok := raw["created_by"].(string); ok {
		pin.CreatedBy = cb
	}

	if created, ok := toFloat64(raw["created"]); ok {
		pin.CreatedTS = int64(created)
		pin.Created = time.Unix(int64(created), 0).UTC().Format("02 Jan 2006 15:04")
	}

	// Extract message text for pinned messages.
	if msg, ok := raw["message"].(map[string]any); ok {
		if text, ok := msg["text"].(string); ok {
			text = UnescapeEntities(text)
			text = ReplaceEmojiShortcodes(text)
			pin.Text = text
		}
		if ts, ok := msg["ts"].(string); ok {
			pin.MessageTS = ts
		}
	}

	// Extract file info for pinned files/canvases.
	if file, ok := raw["file"].(map[string]any); ok {
		if id, ok := file["id"].(string); ok {
			pin.FileID = id
		}
		if name, ok := file["name"].(string); ok {
			pin.FileName = name
		} else if title, ok := file["title"].(string); ok {
			pin.FileName = title
		}
	}

	return pin
}

// FormatBookmark converts a raw Slack bookmarks.list bookmark into a Bookmark.
func FormatBookmark(raw map[string]any) Bookmark {
	bm := Bookmark{}

	if id, ok := raw["id"].(string); ok {
		bm.ID = id
	}
	if title, ok := raw["title"].(string); ok {
		bm.Title = title
	}
	if t, ok := raw["type"].(string); ok {
		bm.Type = t
	}
	if link, ok := raw["link"].(string); ok {
		bm.Link = link
	}
	if emoji, ok := raw["emoji"].(string); ok {
		bm.Emoji = emoji
	}

	return bm
}

// PinnedItemDisplayText returns a one-line description of a pinned item suitable
// for table output.
func PinnedItemDisplayText(pin PinnedItem) string {
	switch pin.Type {
	case "message":
		text := pin.Text
		if text == "" {
			text = "(no text)"
		}
		runes := []rune(text)
		if len(runes) > 60 {
			text = string(runes[:57]) + "..."
		}
		return strconv.Quote(text)
	case "file":
		if pin.FileName != "" {
			return pin.FileName
		}
		if pin.FileID != "" {
			return "file " + pin.FileID
		}
		return "(file)"
	default:
		if pin.FileName != "" {
			return pin.FileName
		}
		return pin.Type
	}
}

// toFloat64 converts an any to float64, handling both float64 and int.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
