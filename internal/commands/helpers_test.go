package commands

import (
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/cache"
)

func TestFormatMessages_Empty(t *testing.T) {
	result := formatMessages(nil, "https://team.slack.com", "C123", true)
	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}

func TestFormatMessages_WithPermalinks(t *testing.T) {
	messages := []map[string]any{
		{"ts": "1741234567.123456", "user": "U123", "text": "hello"},
		{"ts": "1741234568.000000", "user": "U456", "text": "world"},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// Both should have permalink links.
	if result[0].Link == "" {
		t.Error("first message should have a permalink")
	}
	if result[1].Link == "" {
		t.Error("second message should have a permalink")
	}
	// Verify link format.
	expected := "https://team.slack.com/archives/C12345678/p1741234567123456"
	if result[0].Link != expected {
		t.Errorf("Link = %q, want %q", result[0].Link, expected)
	}
}

func TestFormatMessages_NoTeamURL(t *testing.T) {
	messages := []map[string]any{
		{"ts": "1741234567.123456", "user": "U123", "text": "hello"},
	}

	result := formatMessages(messages, "", "C12345678", false)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// Should NOT have a permalink when hasTeamURL is false.
	if result[0].Link != "" {
		t.Errorf("Link should be empty when hasTeamURL is false, got %q", result[0].Link)
	}
}

func TestFormatMessages_MessageWithoutTS(t *testing.T) {
	messages := []map[string]any{
		{"user": "U123", "text": "no timestamp"},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// No ts means no permalink should be generated.
	if result[0].Link != "" {
		t.Errorf("Link should be empty when message has no ts, got %q", result[0].Link)
	}
}

func TestFormatMessages_PreservesFormattedFields(t *testing.T) {
	messages := []map[string]any{
		{
			"ts":          "1741234567.123456",
			"user":        "U123",
			"text":        "test message",
			"reply_count": float64(5),
			"reactions": []any{
				map[string]any{"name": "thumbsup", "count": float64(2)},
			},
		},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	msg := result[0]
	if msg.Text != "test message" {
		t.Errorf("Text = %q, want %q", msg.Text, "test message")
	}
	if msg.User != "U123" {
		t.Errorf("User = %q, want %q", msg.User, "U123")
	}
	if msg.ReplyCount != 5 {
		t.Errorf("ReplyCount = %d, want 5", msg.ReplyCount)
	}
	if len(msg.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(msg.Reactions))
	}
}

func TestGetCache_NoCacheFlag(t *testing.T) {
	orig := NoCache
	defer func() { NoCache = orig }()

	NoCache = true
	c := getCache()
	if c != nil {
		t.Error("getCache should return nil when NoCache is true")
	}
}

func TestCacheWrite_NilCache(t *testing.T) {
	// Should not panic on nil cache.
	cacheWrite(nil, "test", "slug", map[string]string{"key": "value"}, cache.Metadata{})
}

func TestTruncate_Short(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate = %q, want %q", got, "hello")
	}
}

func TestTruncate_Long(t *testing.T) {
	got := truncate("hello world this is a long string", 10)
	if got != "hello w..." {
		t.Errorf("truncate = %q, want %q", got, "hello w...")
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := truncate("1234567890", 10)
	if got != "1234567890" {
		t.Errorf("truncate = %q, want %q", got, "1234567890")
	}
}
