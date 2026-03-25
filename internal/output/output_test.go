package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// ── ParseFormat ─────────────────────────────────────────────────────────────

func TestParseFormat_JSON(t *testing.T) {
	f, err := ParseFormat("json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != JSON {
		t.Errorf("format = %q, want %q", f, JSON)
	}
}

func TestParseFormat_JSONUpperCase(t *testing.T) {
	f, err := ParseFormat("JSON")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != JSON {
		t.Errorf("format = %q, want %q", f, JSON)
	}
}

func TestParseFormat_Markdown(t *testing.T) {
	f, err := ParseFormat("markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != Markdown {
		t.Errorf("format = %q, want %q", f, Markdown)
	}
}

func TestParseFormat_MD(t *testing.T) {
	f, err := ParseFormat("md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != Markdown {
		t.Errorf("format = %q, want %q", f, Markdown)
	}
}

func TestParseFormat_WithWhitespace(t *testing.T) {
	f, err := ParseFormat("  json  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != JSON {
		t.Errorf("format = %q, want %q", f, JSON)
	}
}

func TestParseFormat_Invalid(t *testing.T) {
	_, err := ParseFormat("xml")
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error should mention the invalid format: %v", err)
	}
}

func TestParseFormat_Empty(t *testing.T) {
	_, err := ParseFormat("")
	if err == nil {
		t.Fatal("expected error for empty format, got nil")
	}
}

// ── RenderSingle JSON ───────────────────────────────────────────────────────

func TestRenderSingle_JSON_Basic(t *testing.T) {
	msg := formatting.Message{
		TS:   "1741234567.000000",
		Time: "2025-03-06 03:16 UTC",
		User: "U123",
		Text: "hello world",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must be valid JSON.
	var decoded formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
	if decoded.User != "U123" {
		t.Errorf("decoded.User = %q, want %q", decoded.User, "U123")
	}
	if decoded.Text != "hello world" {
		t.Errorf("decoded.Text = %q, want %q", decoded.Text, "hello world")
	}
}

func TestRenderSingle_JSON_WithAttachment(t *testing.T) {
	msg := formatting.Message{
		TS:   "1741234567.000000",
		Time: "2025-03-06 03:16 UTC",
		User: "U123",
		Text: "alert fired",
		Attachment: &formatting.Attachment{
			Title: "CPU Alert",
			Text:  "CPU usage above 90%",
			Color: "danger",
		},
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Attachment == nil {
		t.Fatal("decoded.Attachment should not be nil")
	}
	if decoded.Attachment.Title != "CPU Alert" {
		t.Errorf("Attachment.Title = %q, want %q", decoded.Attachment.Title, "CPU Alert")
	}
}

func TestRenderSingle_JSON_WithReactions(t *testing.T) {
	msg := formatting.Message{
		TS:        "1741234567.000000",
		Time:      "2025-03-06 03:16 UTC",
		User:      "U123",
		Text:      "great idea",
		Reactions: []string{"thumbsup(3)", "heart(1)"},
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Reactions) != 2 {
		t.Fatalf("len(Reactions) = %d, want 2", len(decoded.Reactions))
	}
	if decoded.Reactions[0] != "thumbsup(3)" {
		t.Errorf("Reactions[0] = %q, want %q", decoded.Reactions[0], "thumbsup(3)")
	}
}

func TestRenderSingle_JSON_WithLink(t *testing.T) {
	msg := formatting.Message{
		TS:   "1741234567.000000",
		User: "U123",
		Text: "check this",
		Link: "https://myteam.slack.com/archives/C123/p1741234567000000",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Link != msg.Link {
		t.Errorf("Link = %q, want %q", decoded.Link, msg.Link)
	}
}

// ── RenderSingle Markdown ───────────────────────────────────────────────────

func TestRenderSingle_Markdown_Basic(t *testing.T) {
	msg := formatting.Message{
		Time: "2026-03-01 14:00 UTC",
		User: "username",
		Text: "Message text here.",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## 2026-03-01 14:00 UTC — @username") {
		t.Errorf("missing header in output:\n%s", out)
	}
	if !strings.Contains(out, "Message text here.") {
		t.Errorf("missing text in output:\n%s", out)
	}
}

func TestRenderSingle_Markdown_WithAttachment(t *testing.T) {
	msg := formatting.Message{
		Time: "2026-03-01 14:00 UTC",
		User: "username",
		Text: "alert",
		Attachment: &formatting.Attachment{
			Title: "Attachment title",
			Text:  "Attachment text",
		},
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "> Attachment title") {
		t.Errorf("missing attachment title blockquote:\n%s", out)
	}
	if !strings.Contains(out, "> Attachment text") {
		t.Errorf("missing attachment text blockquote:\n%s", out)
	}
}

func TestRenderSingle_Markdown_WithReactions(t *testing.T) {
	msg := formatting.Message{
		Time:      "2026-03-01 14:00 UTC",
		User:      "username",
		Text:      "good stuff",
		Reactions: []string{"thumbsup(3)", "heart(1)"},
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Reactions: thumbsup(3), heart(1)") {
		t.Errorf("missing reactions line:\n%s", out)
	}
}

func TestRenderSingle_Markdown_WithThreadAndLink(t *testing.T) {
	msg := formatting.Message{
		Time:       "2026-03-01 14:00 UTC",
		User:       "username",
		Text:       "thread starter",
		ReplyCount: 5,
		Link:       "https://myteam.slack.com/archives/C123/p1741234567000000",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[Thread: 5 replies]") {
		t.Errorf("missing thread info:\n%s", out)
	}
	if !strings.Contains(out, "[Link](https://myteam.slack.com/archives/C123/p1741234567000000)") {
		t.Errorf("missing link:\n%s", out)
	}
}

func TestRenderSingle_Markdown_OnlyUser(t *testing.T) {
	msg := formatting.Message{
		User: "alice",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should have header with user but no dash separator when time is empty.
	if !strings.HasPrefix(out, "## @alice\n") {
		t.Errorf("unexpected header for user-only message:\n%s", out)
	}
}

func TestRenderSingle_Markdown_NoTimeNoUser(t *testing.T) {
	msg := formatting.Message{
		Text: "orphan message",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should use "(no timestamp)" fallback instead of bare "##".
	if !strings.HasPrefix(out, "## (no timestamp)\n") {
		t.Errorf("expected fallback header, got:\n%s", out)
	}
	if strings.HasPrefix(out, "## \n") {
		t.Errorf("bare '## ' header should not appear:\n%s", out)
	}
}

func TestRenderSingle_Markdown_MultilineAttachment(t *testing.T) {
	msg := formatting.Message{
		Time: "2026-03-01 14:00 UTC",
		User: "bot",
		Text: "alert fired",
		Attachment: &formatting.Attachment{
			Title: "Line1 title\nLine2 title",
			Text:  "stack line 1\nstack line 2\nstack line 3",
		},
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Every line of the title should be blockquoted.
	if !strings.Contains(out, "> Line1 title\n> Line2 title\n") {
		t.Errorf("multiline title not fully blockquoted:\n%s", out)
	}
	// Every line of the text should be blockquoted.
	if !strings.Contains(out, "> stack line 1\n> stack line 2\n> stack line 3\n") {
		t.Errorf("multiline text not fully blockquoted:\n%s", out)
	}
}

// ── RenderMessages ──────────────────────────────────────────────────────────

func TestRenderMessages_JSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMessages(&buf, []formatting.Message{}, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
	if len(decoded) != 0 {
		t.Errorf("len(decoded) = %d, want 0", len(decoded))
	}
}

func TestRenderMessages_JSON_Multiple(t *testing.T) {
	msgs := []formatting.Message{
		{TS: "1", User: "U1", Text: "first"},
		{TS: "2", User: "U2", Text: "second"},
	}

	var buf bytes.Buffer
	if err := RenderMessages(&buf, msgs, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("len(decoded) = %d, want 2", len(decoded))
	}
	if decoded[0].Text != "first" {
		t.Errorf("decoded[0].Text = %q, want %q", decoded[0].Text, "first")
	}
	if decoded[1].Text != "second" {
		t.Errorf("decoded[1].Text = %q, want %q", decoded[1].Text, "second")
	}
}

func TestRenderMessages_Markdown_Separator(t *testing.T) {
	msgs := []formatting.Message{
		{Time: "2026-03-01 14:00 UTC", User: "alice", Text: "first"},
		{Time: "2026-03-01 14:01 UTC", User: "bob", Text: "second"},
	}

	var buf bytes.Buffer
	if err := RenderMessages(&buf, msgs, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "---") {
		t.Errorf("missing separator between messages:\n%s", out)
	}
	if !strings.Contains(out, "@alice") {
		t.Errorf("missing first message header:\n%s", out)
	}
	if !strings.Contains(out, "@bob") {
		t.Errorf("missing second message header:\n%s", out)
	}
}

func TestRenderMessages_Markdown_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMessages(&buf, []formatting.Message{}, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty list, got:\n%s", buf.String())
	}
}

func TestRenderMessages_Markdown_SingleNoSeparator(t *testing.T) {
	msgs := []formatting.Message{
		{Time: "2026-03-01 14:00 UTC", User: "alice", Text: "only one"},
	}

	var buf bytes.Buffer
	if err := RenderMessages(&buf, msgs, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "---") {
		t.Errorf("single message should not have separator:\n%s", out)
	}
}

// ── RenderSearchResults ─────────────────────────────────────────────────────

func TestRenderSearchResults_JSON(t *testing.T) {
	results := []map[string]any{
		{"ts": "1741234567.000000", "user": "U1", "text": "found it", "channel": "general"},
	}

	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, results, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("len(decoded) = %d, want 1", len(decoded))
	}
}

func TestRenderSearchResults_Markdown(t *testing.T) {
	results := []map[string]any{
		{"ts": "1741234567.000000", "user": "U1", "text": "found it", "channel": "general"},
		{"ts": "1741234568.000000", "user": "U2", "text": "also here", "channel": "random"},
	}

	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, results, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "#general") {
		t.Errorf("missing channel in output:\n%s", out)
	}
	if !strings.Contains(out, "@U1") {
		t.Errorf("missing user in output:\n%s", out)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("missing separator between results:\n%s", out)
	}
	// Should render human-readable time, not raw ts.
	if strings.Contains(out, "1741234567.000000") {
		t.Errorf("search results should show formatted time, not raw ts:\n%s", out)
	}
	if !strings.Contains(out, "2025-03-06") {
		t.Errorf("search results should show formatted date:\n%s", out)
	}
}

func TestRenderSearchResults_Markdown_EmptyFields(t *testing.T) {
	results := []map[string]any{
		{"text": "orphan result"},
	}

	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, results, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## (no timestamp)") {
		t.Errorf("expected fallback header for empty search result fields:\n%s", out)
	}
	if strings.HasPrefix(out, "## \n") {
		t.Errorf("bare '## ' header should not appear:\n%s", out)
	}
}

func TestRenderSearchResults_JSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, []map[string]any{}, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
	if len(decoded) != 0 {
		t.Errorf("len(decoded) = %d, want 0", len(decoded))
	}
}

// ── Unsupported format ──────────────────────────────────────────────────────

func TestRenderMessages_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := RenderMessages(&buf, nil, Format("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestRenderSingle_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := RenderSingle(&buf, formatting.Message{}, Format("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestRenderSearchResults_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := RenderSearchResults(&buf, nil, Format("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

// ── JSON pretty-printing ────────────────────────────────────────────────────

func TestRenderSearchResults_Markdown_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, []map[string]any{}, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty search results, got:\n%s", buf.String())
	}
}

func TestRenderSearchResults_Markdown_SingleNoSeparator(t *testing.T) {
	results := []map[string]any{
		{"ts": "1741234567.000000", "user": "U1", "text": "only one", "channel": "general"},
	}
	var buf bytes.Buffer
	if err := RenderSearchResults(&buf, results, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "---") {
		t.Errorf("single search result should not have separator:\n%s", out)
	}
}

func TestRenderSingle_JSON_SpecialCharacters(t *testing.T) {
	msg := formatting.Message{
		TS:   "1741234567.000000",
		User: "U123",
		Text: `He said "hello" & <world> with 'quotes' and \backslash`,
	}
	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must be valid JSON.
	var decoded formatting.Message
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
	if decoded.Text != msg.Text {
		t.Errorf("decoded.Text = %q, want %q", decoded.Text, msg.Text)
	}
}

func TestRenderSingle_JSON_EmptyMessage(t *testing.T) {
	msg := formatting.Message{}
	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
}

func TestRenderSingle_Markdown_AllFieldsEmpty(t *testing.T) {
	msg := formatting.Message{}
	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(no timestamp)") {
		t.Errorf("empty message should show fallback header:\n%s", out)
	}
}

func TestFormatTS_Empty(t *testing.T) {
	if got := FormatTS(""); got != "" {
		t.Errorf("FormatTS(\"\") = %q, want empty", got)
	}
}

func TestFormatTS_Invalid(t *testing.T) {
	// Non-parseable ts should be returned as-is.
	if got := FormatTS("not-a-number"); got != "not-a-number" {
		t.Errorf("FormatTS(\"not-a-number\") = %q, want \"not-a-number\"", got)
	}
}

func TestFormatTS_Valid(t *testing.T) {
	got := FormatTS("1741234567.000000")
	if got == "" || got == "1741234567.000000" {
		t.Errorf("FormatTS should convert to human-readable, got %q", got)
	}
	if !strings.Contains(got, "2025-03-06") {
		t.Errorf("FormatTS should contain date, got %q", got)
	}
}

func TestRenderSingle_Markdown_OnlyText(t *testing.T) {
	msg := formatting.Message{
		Text: "only text, no user or time",
	}
	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "only text, no user or time") {
		t.Errorf("expected text in output:\n%s", out)
	}
}

func TestRenderMessages_JSON_Nil(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMessages(&buf, nil, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil slice should marshal as null in JSON.
	out := strings.TrimSpace(buf.String())
	if out != "null" {
		t.Errorf("nil messages should marshal as null, got: %q", out)
	}
}

func TestRenderMessages_Markdown_Nil(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMessages(&buf, nil, Markdown); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil messages, got:\n%s", buf.String())
	}
}

func TestRenderSingle_JSON_IsPrettyPrinted(t *testing.T) {
	msg := formatting.Message{
		TS:   "1741234567.000000",
		User: "U123",
		Text: "hello",
	}

	var buf bytes.Buffer
	if err := RenderSingle(&buf, msg, JSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Pretty-printed JSON has newlines and indentation.
	if !strings.Contains(out, "\n") {
		t.Errorf("JSON output should be pretty-printed (contain newlines):\n%s", out)
	}
	if !strings.Contains(out, "  ") {
		t.Errorf("JSON output should be indented:\n%s", out)
	}
}
