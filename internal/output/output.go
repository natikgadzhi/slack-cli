// Package output renders formatted Slack messages in JSON or Markdown.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// Format represents an output format for rendering messages.
type Format string

const (
	// JSON renders output as pretty-printed JSON.
	JSON Format = "json"
	// Markdown renders output as human-readable Markdown.
	Markdown Format = "markdown"
)

// ParseFormat converts a string flag value into a Format.
// Accepted values: "json", "markdown", "md".
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return JSON, nil
	case "markdown", "md":
		return Markdown, nil
	default:
		return "", fmt.Errorf("unsupported output format: %q (valid: json, markdown, md)", s)
	}
}

// RenderMessages writes a slice of messages to w in the given format.
func RenderMessages(w io.Writer, messages []formatting.Message, format Format) error {
	switch format {
	case JSON:
		return renderJSON(w, messages)
	case Markdown:
		return renderMessagesMarkdown(w, messages)
	default:
		return fmt.Errorf("unsupported format: %q", format)
	}
}

// RenderSearchResults writes search results to w in the given format.
func RenderSearchResults(w io.Writer, results []map[string]any, format Format) error {
	switch format {
	case JSON:
		return renderJSON(w, results)
	case Markdown:
		return renderSearchResultsMarkdown(w, results)
	default:
		return fmt.Errorf("unsupported format: %q", format)
	}
}

// RenderSingle writes a single message to w in the given format.
func RenderSingle(w io.Writer, msg formatting.Message, format Format) error {
	switch format {
	case JSON:
		return renderJSON(w, msg)
	case Markdown:
		return renderMessageMarkdown(w, msg)
	default:
		return fmt.Errorf("unsupported format: %q", format)
	}
}

// renderJSON writes v as pretty-printed JSON followed by a newline.
func renderJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// renderMessagesMarkdown writes a list of messages in Markdown format.
func renderMessagesMarkdown(w io.Writer, messages []formatting.Message) error {
	for i, msg := range messages {
		if err := renderMessageMarkdown(w, msg); err != nil {
			return err
		}
		if i < len(messages)-1 {
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return err
			}
		}
	}
	return nil
}

// renderMessageMarkdown writes a single message in Markdown format.
func renderMessageMarkdown(w io.Writer, msg formatting.Message) error {
	// Header line: ## time — @user
	// Guard: if both Time and User are empty, use a fallback.
	header := "##"
	if msg.Time != "" {
		header += " " + msg.Time
	}
	if msg.User != "" {
		if msg.Time != "" {
			header += " —"
		}
		header += " @" + msg.User
	}
	if msg.Time == "" && msg.User == "" {
		header += " (no timestamp)"
	}
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Message text.
	if msg.Text != "" {
		if _, err := fmt.Fprintln(w, msg.Text); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Attachment as blockquote. Each line is prefixed with "> ".
	if msg.Attachment != nil {
		if msg.Attachment.Title != "" {
			if err := writeBlockquote(w, msg.Attachment.Title); err != nil {
				return err
			}
		}
		if msg.Attachment.Text != "" {
			if err := writeBlockquote(w, msg.Attachment.Text); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Reactions.
	if len(msg.Reactions) > 0 {
		if _, err := fmt.Fprintf(w, "Reactions: %s\n", strings.Join(msg.Reactions, ", ")); err != nil {
			return err
		}
	}

	// Thread + Link metadata line.
	var meta []string
	if msg.ReplyCount > 0 {
		meta = append(meta, fmt.Sprintf("[Thread: %d replies]", msg.ReplyCount))
	}
	if msg.Link != "" {
		meta = append(meta, fmt.Sprintf("[Link](%s)", msg.Link))
	}
	if len(meta) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(meta, " | ")); err != nil {
			return err
		}
	}

	return nil
}

// renderSearchResultsMarkdown writes search results in Markdown format.
// Each result is rendered as a section with available fields.
func renderSearchResultsMarkdown(w io.Writer, results []map[string]any) error {
	for i, r := range results {
		// Try to extract common fields.
		user, _ := r["user"].(string)
		text, _ := r["text"].(string)
		ts, _ := r["ts"].(string)
		channel, _ := r["channel"].(string)

		// Convert raw ts to human-readable time.
		timeStr := formatTS(ts)

		header := "##"
		if timeStr != "" {
			header += " " + timeStr
		}
		if channel != "" {
			header += " #" + channel
		}
		if user != "" {
			header += " — @" + user
		}
		// Guard: if all fields are empty, use a fallback.
		if timeStr == "" && channel == "" && user == "" {
			header += " (no timestamp)"
		}

		if _, err := fmt.Fprintln(w, header); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}

		if text != "" {
			if _, err := fmt.Fprintln(w, text); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		if i < len(results)-1 {
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeBlockquote writes text as a Markdown blockquote, prefixing each line with "> ".
func writeBlockquote(w io.Writer, text string) error {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "> %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

// formatTS converts a Slack ts string (e.g. "1741234567.000000") to a
// human-readable UTC time string. Returns empty string if parsing fails.
func formatTS(ts string) string {
	if ts == "" {
		return ""
	}
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return ts
	}
	t := time.Unix(int64(f), 0).UTC()
	return t.Format("2006-01-02 15:04 UTC")
}
