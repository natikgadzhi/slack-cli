package formatting

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Message is the structured representation of a formatted Slack message.
type Message struct {
	TS         string      `json:"ts,omitempty"`
	Time       string      `json:"time,omitempty"`
	User       string      `json:"user,omitempty"`
	Text       string      `json:"text,omitempty"`
	ReplyCount int         `json:"reply_count,omitempty"`
	Reactions  []string    `json:"reactions,omitempty"`
	Attachment *Attachment `json:"attachment,omitempty"`
	Link       string      `json:"link,omitempty"`
}

// Attachment holds structured attachment data (e.g. from alert bots).
type Attachment struct {
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	Color    string `json:"color,omitempty"`
	Source   string `json:"source,omitempty"`
	Silence  string `json:"silence,omitempty"`
	Playbook string `json:"playbook,omitempty"`
}

// FormatMessage converts a raw Slack API message (map[string]any) into a Message struct.
func FormatMessage(raw map[string]any) Message {
	var msg Message

	if ts, ok := raw["ts"].(string); ok && ts != "" {
		msg.TS = ts
		if f, err := strconv.ParseFloat(ts, 64); err == nil {
			t := time.Unix(int64(f), 0).UTC()
			msg.Time = t.Format("02 Jan 2006 15:04")
		}
	}

	if user, ok := raw["user"].(string); ok && user != "" {
		msg.User = user
	}

	if text, ok := raw["text"].(string); ok {
		text = strings.TrimSpace(text)
		if text != "" {
			msg.Text = TruncateRunes(text, 500)
		}
	}

	// reply_count: include only if > 0.
	if rc, ok := toInt(raw["reply_count"]); ok && rc > 0 {
		msg.ReplyCount = rc
	}

	// reactions: format as "name(count)".
	if reactions, ok := raw["reactions"].([]any); ok {
		for _, r := range reactions {
			if rm, ok := r.(map[string]any); ok {
				name, _ := rm["name"].(string)
				count, _ := toInt(rm["count"])
				msg.Reactions = append(msg.Reactions, fmt.Sprintf("%s(%d)", name, count))
			}
		}
	}

	// attachments: process first attachment only.
	if attachments, ok := raw["attachments"].([]any); ok && len(attachments) > 0 {
		if att, ok := attachments[0].(map[string]any); ok {
			a := buildAttachment(att)
			if a != nil {
				msg.Attachment = a
			}
		}
	}

	return msg
}

// buildAttachment extracts structured fields from a raw attachment map.
func buildAttachment(att map[string]any) *Attachment {
	a := &Attachment{}
	empty := true

	if title, ok := att["title"].(string); ok && title != "" {
		a.Title = title
		empty = false
	}

	// Text falls back to fallback.
	text := ""
	if t, ok := att["text"].(string); ok {
		text = t
	} else if fb, ok := att["fallback"].(string); ok {
		text = fb
	}
	text = strings.TrimSpace(text)
	text = TruncateRunes(text, 300)
	if text != "" {
		a.Text = text
		empty = false
	}

	if color, ok := att["color"].(string); ok && color != "" {
		a.Color = color
		empty = false
	}

	if u := actionURL(att, "source"); u != "" {
		a.Source = u
		empty = false
	}
	if u := actionURL(att, "silence"); u != "" {
		a.Silence = u
		empty = false
	}
	if u := actionURL(att, "playbook"); u != "" {
		a.Playbook = u
		empty = false
	}

	if empty {
		return nil
	}
	return a
}

// actionURL searches the attachment's actions for one whose text contains keyword (case-insensitive).
func actionURL(att map[string]any, keyword string) string {
	actions, ok := att["actions"].([]any)
	if !ok {
		return ""
	}
	for _, a := range actions {
		if am, ok := a.(map[string]any); ok {
			text, _ := am["text"].(string)
			if strings.Contains(strings.ToLower(text), keyword) {
				if u, ok := am["url"].(string); ok {
					return u
				}
			}
		}
	}
	return ""
}

// BuildPermalink constructs a Slack message permalink.
func BuildPermalink(teamURL, channelID, ts string) string {
	base := strings.TrimRight(teamURL, "/")
	tsCompact := strings.ReplaceAll(ts, ".", "")
	return fmt.Sprintf("%s/archives/%s/p%s", base, channelID, tsCompact)
}

// TruncateRunes truncates s to at most maxRunes runes. No suffix is added.
func TruncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// toInt converts an any value to int, handling both float64 (JSON default) and int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}
