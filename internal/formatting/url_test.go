package formatting

import (
	"testing"
)

func TestParseSlackURL_Basic(t *testing.T) {
	channel, messageTS, threadTS, err := ParseSlackURL("https://myteam.slack.com/archives/C12345678/p1741234567123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channel != "C12345678" {
		t.Errorf("channel = %q, want %q", channel, "C12345678")
	}
	if messageTS != "1741234567.123456" {
		t.Errorf("messageTS = %q, want %q", messageTS, "1741234567.123456")
	}
	if threadTS != "" {
		t.Errorf("threadTS = %q, want empty", threadTS)
	}
}

func TestParseSlackURL_WithThreadTS(t *testing.T) {
	channel, messageTS, threadTS, err := ParseSlackURL(
		"https://myteam.slack.com/archives/C12345678/p1741234567123456?thread_ts=1741234560.000100&cid=C12345678",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channel != "C12345678" {
		t.Errorf("channel = %q, want %q", channel, "C12345678")
	}
	if messageTS != "1741234567.123456" {
		t.Errorf("messageTS = %q, want %q", messageTS, "1741234567.123456")
	}
	if threadTS != "1741234560.000100" {
		t.Errorf("threadTS = %q, want %q", threadTS, "1741234560.000100")
	}
}

func TestParseSlackURL_InvalidPath(t *testing.T) {
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/messages/C12345678")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestParseSlackURL_InvalidTSFormat(t *testing.T) {
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/archives/C12345678/notavalidts")
	if err == nil {
		t.Fatal("expected error for invalid timestamp, got nil")
	}
}

func TestParseSlackURL_EmptyString(t *testing.T) {
	_, _, _, err := ParseSlackURL("")
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestParseSlackURL_NoArchivesSegment(t *testing.T) {
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/something/else")
	if err == nil {
		t.Fatal("expected error when 'archives' is missing, got nil")
	}
}

func TestParseSlackURL_ArchivesButMissingParts(t *testing.T) {
	// Has "archives" but no channel ID or timestamp after it.
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/archives")
	if err == nil {
		t.Fatal("expected error for URL with only 'archives' and no following parts")
	}
}

func TestParseSlackURL_ArchivesOnlyChannelNoParts(t *testing.T) {
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/archives/C12345678")
	if err == nil {
		t.Fatal("expected error when timestamp part is missing")
	}
}

func TestParseSlackURL_ShortTimestamp(t *testing.T) {
	_, _, _, err := ParseSlackURL("https://myteam.slack.com/archives/C12345678/p12345")
	if err == nil {
		t.Fatal("expected error for timestamp shorter than 12 chars, got nil")
	}
}

func TestParseSlackURL_TrailingSlash(t *testing.T) {
	channel, messageTS, _, err := ParseSlackURL("https://myteam.slack.com/archives/C12345678/p1741234567123456/")
	if err != nil {
		// Trailing slash after timestamp means there's an extra empty segment; should still work.
		// This depends on implementation. Let's just verify it handles gracefully.
		t.Logf("trailing slash produced error (acceptable): %v", err)
		return
	}
	if channel != "C12345678" {
		t.Errorf("channel = %q, want C12345678", channel)
	}
	if messageTS != "1741234567.123456" {
		t.Errorf("messageTS = %q", messageTS)
	}
}

func TestParseSlackURL_DMChannelID(t *testing.T) {
	// DM channels use D prefix.
	channel, _, _, err := ParseSlackURL("https://myteam.slack.com/archives/D12345678/p1741234567123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channel != "D12345678" {
		t.Errorf("channel = %q, want D12345678", channel)
	}
}

func TestParseSlackURL_OnlyThreadTSNoOtherQuery(t *testing.T) {
	_, _, threadTS, err := ParseSlackURL(
		"https://myteam.slack.com/archives/C12345678/p1741234567123456?thread_ts=1741234560.000100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if threadTS != "1741234560.000100" {
		t.Errorf("threadTS = %q, want 1741234560.000100", threadTS)
	}
}

func TestParseSlackURL_NoThreadTSQueryParam(t *testing.T) {
	_, _, threadTS, err := ParseSlackURL(
		"https://myteam.slack.com/archives/C12345678/p1741234567123456?foo=bar",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if threadTS != "" {
		t.Errorf("threadTS = %q, want empty when thread_ts not in query", threadTS)
	}
}
