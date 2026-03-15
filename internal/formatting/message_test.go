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
	if len([]rune(msg.Text)) != 500 {
		t.Errorf("len(runes(Text)) = %d, want 500", len([]rune(msg.Text)))
	}
}

func TestFormatMessage_TruncatesMultibyteText(t *testing.T) {
	// Use a multi-byte character (3 bytes in UTF-8) repeated beyond the limit.
	longText := strings.Repeat("\u4e16", 600) // 600 runes, each 3 bytes
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"text": longText,
	})
	if len([]rune(msg.Text)) != 500 {
		t.Errorf("len(runes(Text)) = %d, want 500", len([]rune(msg.Text)))
	}
	// Ensure no broken UTF-8 sequences.
	for i, r := range msg.Text {
		if r == '\uFFFD' {
			t.Errorf("replacement character at byte %d — truncation broke a rune", i)
			break
		}
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

// ── Empty/nil message edge cases ────────────────────────────────────────────

func TestFormatMessage_EmptyMap(t *testing.T) {
	msg := FormatMessage(map[string]any{})
	if msg.TS != "" {
		t.Errorf("TS = %q, want empty for empty map", msg.TS)
	}
	if msg.Text != "" {
		t.Errorf("Text = %q, want empty for empty map", msg.Text)
	}
	if msg.User != "" {
		t.Errorf("User = %q, want empty for empty map", msg.User)
	}
	if msg.ReplyCount != 0 {
		t.Errorf("ReplyCount = %d, want 0 for empty map", msg.ReplyCount)
	}
	if len(msg.Reactions) != 0 {
		t.Errorf("Reactions should be empty for empty map")
	}
	if msg.Attachment != nil {
		t.Error("Attachment should be nil for empty map")
	}
}

func TestFormatMessage_TextExactly500Runes(t *testing.T) {
	text := strings.Repeat("a", 500)
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"text": text,
	})
	if len([]rune(msg.Text)) != 500 {
		t.Errorf("len(runes(Text)) = %d, want 500", len([]rune(msg.Text)))
	}
}

func TestFormatMessage_UnicodeText(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":   "1741234567.000000",
		"text": "Hello world! Emoji: \U0001F600 Kanji: \u4e16\u754c",
	})
	if msg.Text == "" {
		t.Error("Text should not be empty for unicode input")
	}
	if !strings.Contains(msg.Text, "\U0001F600") {
		t.Error("Text should preserve emoji")
	}
}

func TestFormatMessage_ReplyCountAsInt(t *testing.T) {
	// toInt should handle actual int values too.
	msg := FormatMessage(map[string]any{
		"ts":          "1741234567.000000",
		"reply_count": 3,
	})
	if msg.ReplyCount != 3 {
		t.Errorf("ReplyCount = %d, want 3 (from int type)", msg.ReplyCount)
	}
}

func TestFormatMessage_MultipleReactions(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"reactions": []any{
			map[string]any{"name": "thumbsup", "count": float64(3)},
			map[string]any{"name": "heart", "count": float64(1)},
			map[string]any{"name": "rocket", "count": float64(7)},
		},
	})
	if len(msg.Reactions) != 3 {
		t.Fatalf("len(Reactions) = %d, want 3", len(msg.Reactions))
	}
	if msg.Reactions[2] != "rocket(7)" {
		t.Errorf("Reactions[2] = %q, want %q", msg.Reactions[2], "rocket(7)")
	}
}

func TestFormatMessage_EmptyReactionsArray(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":        "1741234567.000000",
		"reactions": []any{},
	})
	if len(msg.Reactions) != 0 {
		t.Errorf("len(Reactions) = %d, want 0 for empty array", len(msg.Reactions))
	}
}

func TestFormatMessage_AttachmentWithFallback(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{
				"fallback": "Fallback text here",
			},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil when fallback is present")
	}
	if msg.Attachment.Text != "Fallback text here" {
		t.Errorf("Attachment.Text = %q, want fallback text", msg.Attachment.Text)
	}
}

func TestFormatMessage_EmptyAttachment(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{},
		},
	})
	if msg.Attachment != nil {
		t.Error("Attachment should be nil when all fields are empty")
	}
}

func TestFormatMessage_AttachmentTextTruncated(t *testing.T) {
	longText := strings.Repeat("y", 400)
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{"text": longText},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil")
	}
	if len([]rune(msg.Attachment.Text)) != 300 {
		t.Errorf("Attachment.Text rune length = %d, want 300 (truncated)", len([]rune(msg.Attachment.Text)))
	}
}

func TestFormatMessage_AttachmentWithActions(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{
				"title": "Alert",
				"text":  "Something broke",
				"actions": []any{
					map[string]any{"text": "View Source", "url": "https://example.com/source"},
					map[string]any{"text": "Silence Alert", "url": "https://example.com/silence"},
					map[string]any{"text": "Playbook Link", "url": "https://example.com/playbook"},
				},
			},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil")
	}
	if msg.Attachment.Source != "https://example.com/source" {
		t.Errorf("Source = %q, want https://example.com/source", msg.Attachment.Source)
	}
	if msg.Attachment.Silence != "https://example.com/silence" {
		t.Errorf("Silence = %q, want https://example.com/silence", msg.Attachment.Silence)
	}
	if msg.Attachment.Playbook != "https://example.com/playbook" {
		t.Errorf("Playbook = %q, want https://example.com/playbook", msg.Attachment.Playbook)
	}
}

func TestFormatMessage_AttachmentActionsNoMatch(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{
				"title": "Something",
				"actions": []any{
					map[string]any{"text": "Unrelated", "url": "https://example.com/other"},
				},
			},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil (title is set)")
	}
	if msg.Attachment.Source != "" {
		t.Errorf("Source should be empty when no matching action, got %q", msg.Attachment.Source)
	}
}

func TestFormatMessage_ActionURLWithoutURL(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts": "1741234567.000000",
		"attachments": []any{
			map[string]any{
				"title": "Alert",
				"actions": []any{
					// Action has matching text but no URL field.
					map[string]any{"text": "View Source"},
				},
			},
		},
	})
	if msg.Attachment == nil {
		t.Fatal("Attachment should not be nil")
	}
	if msg.Attachment.Source != "" {
		t.Errorf("Source should be empty when action has no url, got %q", msg.Attachment.Source)
	}
}

func TestFormatMessage_InvalidTSNotParsed(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":   "not-a-number",
		"text": "hello",
	})
	if msg.TS != "not-a-number" {
		t.Errorf("TS = %q, want the raw value", msg.TS)
	}
	if msg.Time != "" {
		t.Errorf("Time = %q, want empty for unparseable ts", msg.Time)
	}
}

func TestBuildPermalink_EmptyTS(t *testing.T) {
	url := BuildPermalink("https://myteam.slack.com", "C12345678", "")
	expected := "https://myteam.slack.com/archives/C12345678/p"
	if url != expected {
		t.Errorf("url = %q, want %q", url, expected)
	}
}

// ── toInt edge cases ────────────────────────────────────────────────────────

func TestToInt_Float64(t *testing.T) {
	v, ok := toInt(float64(42))
	if !ok || v != 42 {
		t.Errorf("toInt(float64(42)) = (%d, %v), want (42, true)", v, ok)
	}
}

func TestToInt_Int(t *testing.T) {
	v, ok := toInt(int(7))
	if !ok || v != 7 {
		t.Errorf("toInt(int(7)) = (%d, %v), want (7, true)", v, ok)
	}
}

func TestToInt_String(t *testing.T) {
	_, ok := toInt("not-a-number")
	if ok {
		t.Error("toInt(string) should return false")
	}
}

func TestToInt_Nil(t *testing.T) {
	_, ok := toInt(nil)
	if ok {
		t.Error("toInt(nil) should return false")
	}
}
