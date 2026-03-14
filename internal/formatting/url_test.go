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
