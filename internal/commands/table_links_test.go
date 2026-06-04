package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written. Mirrors the pattern used in the saved/unread table tests.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRenderSearchTable_TimeIsHyperlinkNoLinkColumn(t *testing.T) {
	link := "https://lambda.slack.com/archives/C123/p1780535782468999"
	results := []map[string]any{
		{"channel": "general", "ts": "1780535782.468999", "user": "alice", "text": "hello there", "permalink": link},
	}

	out := captureStdout(t, func() { renderSearchTable(results) })

	if strings.Contains(out, "LINK") {
		t.Errorf("LINK column header should be gone, got:\n%s", out)
	}
	if !strings.Contains(out, "\033]8;;"+link+"\033\\") {
		t.Errorf("expected OSC-8 hyperlink to permalink, got:\n%q", out)
	}
	// The raw URL must not appear as visible text (only inside the escape).
	if strings.Count(out, link) != 1 {
		t.Errorf("permalink should appear once (inside the escape), got %d:\n%q", strings.Count(out, link), out)
	}
}

func TestRenderSearchTable_NoPermalinkPlainTime(t *testing.T) {
	results := []map[string]any{
		{"channel": "general", "ts": "1780535782.468999", "user": "alice", "text": "hello"},
	}
	out := captureStdout(t, func() { renderSearchTable(results) })
	if strings.Contains(out, "\033]8;;") {
		t.Errorf("no permalink => no hyperlink escape expected, got:\n%q", out)
	}
}

func TestRenderMessagesTable_TimeIsHyperlinkNoLinkColumn(t *testing.T) {
	link := "https://lambda.slack.com/archives/C123/p1780535782468999"
	msgs := []formatting.Message{
		{Time: "04 Jun 2026 01:16", User: "alice", Text: "hello there", Link: link},
	}

	out := captureStdout(t, func() { renderMessagesTable(msgs) })

	if strings.Contains(out, "LINK") {
		t.Errorf("LINK column header should be gone, got:\n%s", out)
	}
	if !strings.Contains(out, "\033]8;;"+link+"\033\\") {
		t.Errorf("expected OSC-8 hyperlink to permalink, got:\n%q", out)
	}
}

func TestRenderMessagesTable_NoLinkPlainTime(t *testing.T) {
	msgs := []formatting.Message{
		{Time: "04 Jun 2026 01:16", User: "alice", Text: "hello"},
	}
	out := captureStdout(t, func() { renderMessagesTable(msgs) })
	if strings.Contains(out, "\033]8;;") {
		t.Errorf("no link => no hyperlink escape expected, got:\n%q", out)
	}
}
