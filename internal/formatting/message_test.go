package formatting

import (
	"strings"
	"testing"
)

func TestFormatMessage_Basic(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"user": "U123",
		"text": "hello",
	})
	if msg.TS != "1741234567.000000" {
		t.Errorf("TS = %q, want %q", msg.TS, "1741234567.000000")
	}
	if msg.User != "U123" {
		t.Errorf("User = %q, want %q", msg.User, "U123")
	}
	if msg.Text != "hello" {
		t.Errorf("Text = %q, want %q", msg.Text, "hello")
	}
	if msg.Time == "" {
		t.Error("Time should not be empty when TS is set")
	}
}

func TestFormatMessage_TruncatesLongText(t *testing.T) {
	longText := strings.Repeat("x", 600)
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"text": longText,
	})
	if len(msg.Text) != 500 {
		t.Errorf("len(Text) = %d, want 500", len(msg.Text))
	}
}

func TestFormatMessage_OmitsEmptyText(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"text": "   ",
	})
	if msg.Text != "" {
		t.Errorf("Text = %q, want empty for whitespace-only input", msg.Text)
	}
}

func TestFormatMessage_IncludesReplyCount(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":          "1741234567.000000",
		"reply_count": float64(5),
	})
	if msg.ReplyCount != 5 {
		t.Errorf("ReplyCount = %d, want 5", msg.ReplyCount)
	}
}

func TestFormatMessage_OmitsZeroReplyCount(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":          "1741234567.000000",
		"reply_count": float64(0),
	})
	if msg.ReplyCount != 0 {
		t.Errorf("ReplyCount = %d, want 0 (omitted)", msg.ReplyCount)
	}
}

func TestFormatMessage_IncludesReactions(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"reactions": []any{
			map[string]any{"name": "thumbsup", "count": float64(3)},
		},
	})
	if len(msg.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(msg.Reactions))
	}
	if msg.Reactions[0] != "thumbsup(3)" {
		t.Errorf("Reactions[0] = %q, want %q", msg.Reactions[0], "thumbsup(3)")
	}
}

func TestFormatMessage_Attachment(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{
				"title": "Alert",
				"text":  "Something broke",
				"color": "danger",
			},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil")
	}
	if msg.Attachment.Title != "Alert" {
		t.Errorf("Attachment.Title = %q, want %q", msg.Attachment.Title, "Alert")
	}
	if msg.Attachment.Color != "danger" {
		t.Errorf("Attachment.Color = %q, want %q", msg.Attachment.Color, "danger")
	}
}

func TestFormatMessage_MissingTS(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"text": "hello",
	})
	if msg.TS != "" {
		t.Errorf("TS = %q, want empty when ts is missing", msg.TS)
	}
	if msg.Time != "" {
		t.Errorf("Time = %q, want empty when ts is missing", msg.Time)
	}
}

// ── BuildPermalink ──────────────────────────────────────────────────────────

func TestBuildPermalink_Basic(t *testing.T) {
	url := BuildPermalink("https://myteam.slack.com", "C12345678", "1741234567.123456")
	expected := "https://myteam.slack.com/archives/C12345678/p1741234567123456"
	if url != expected {
		t.Errorf("url = %q, want %q", url, expected)
	}
}

func TestBuildPermalink_StripsTrailingSlash(t *testing.T) {
	url := BuildPermalink("https://myteam.slack.com/", "C12345678", "1741234567.123456")
	expected := "https://myteam.slack.com/archives/C12345678/p1741234567123456"
	if url != expected {
		t.Errorf("url = %q, want %q", url, expected)
	}
}

func TestBuildPermalink_TSWithoutDot(t *testing.T) {
	url := BuildPermalink("https://myteam.slack.com", "C12345678", "1741234567000000")
	expected := "https://myteam.slack.com/archives/C12345678/p1741234567000000"
	if url != expected {
		t.Errorf("url = %q, want %q", url, expected)
	}
}
