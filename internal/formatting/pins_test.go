package formatting

import (
	"testing"
)

func TestFormatPinnedItem_Message(t *testing.T) {
	raw := map[string]any{
		"type":       "message",
		"created_by": "U123",
		"created":    float64(1706000001),
		"message": map[string]any{
			"text": "Check the runbook before deploying",
			"ts":   "1700000001.000000",
		},
	}

	pin := FormatPinnedItem(raw)

	if pin.Type != "message" {
		t.Errorf("Type = %q, want %q", pin.Type, "message")
	}
	if pin.CreatedBy != "U123" {
		t.Errorf("CreatedBy = %q, want %q", pin.CreatedBy, "U123")
	}
	if pin.Text != "Check the runbook before deploying" {
		t.Errorf("Text = %q, want %q", pin.Text, "Check the runbook before deploying")
	}
	if pin.MessageTS != "1700000001.000000" {
		t.Errorf("MessageTS = %q, want %q", pin.MessageTS, "1700000001.000000")
	}
	if pin.Created == "" {
		t.Error("Created should not be empty")
	}
	if pin.CreatedTS != 1706000001 {
		t.Errorf("CreatedTS = %d, want %d", pin.CreatedTS, 1706000001)
	}
}

func TestFormatPinnedItem_File(t *testing.T) {
	raw := map[string]any{
		"type":       "file",
		"created_by": "U456",
		"created":    float64(1706000002),
		"file": map[string]any{
			"id":   "F123",
			"name": "design-doc.pdf",
		},
	}

	pin := FormatPinnedItem(raw)

	if pin.Type != "file" {
		t.Errorf("Type = %q, want %q", pin.Type, "file")
	}
	if pin.FileID != "F123" {
		t.Errorf("FileID = %q, want %q", pin.FileID, "F123")
	}
	if pin.FileName != "design-doc.pdf" {
		t.Errorf("FileName = %q, want %q", pin.FileName, "design-doc.pdf")
	}
}

func TestFormatPinnedItem_FileFallbackTitle(t *testing.T) {
	raw := map[string]any{
		"type":       "file",
		"created_by": "U789",
		"file": map[string]any{
			"id":    "F456",
			"title": "Team OKRs Q3",
		},
	}

	pin := FormatPinnedItem(raw)

	if pin.FileName != "Team OKRs Q3" {
		t.Errorf("FileName = %q, want %q (should fall back to title)", pin.FileName, "Team OKRs Q3")
	}
}

func TestFormatPinnedItem_NoFields(t *testing.T) {
	raw := map[string]any{}
	pin := FormatPinnedItem(raw)
	if pin.Type != "" {
		t.Errorf("Type = %q, want empty", pin.Type)
	}
}

func TestFormatBookmark(t *testing.T) {
	raw := map[string]any{
		"id":    "Bk1",
		"title": "Oncall Schedule",
		"type":  "link",
		"link":  "https://example.com/oncall",
		"emoji": ":calendar:",
	}

	bm := FormatBookmark(raw)

	if bm.ID != "Bk1" {
		t.Errorf("ID = %q, want %q", bm.ID, "Bk1")
	}
	if bm.Title != "Oncall Schedule" {
		t.Errorf("Title = %q, want %q", bm.Title, "Oncall Schedule")
	}
	if bm.Type != "link" {
		t.Errorf("Type = %q, want %q", bm.Type, "link")
	}
	if bm.Link != "https://example.com/oncall" {
		t.Errorf("Link = %q, want %q", bm.Link, "https://example.com/oncall")
	}
	if bm.Emoji != ":calendar:" {
		t.Errorf("Emoji = %q, want %q", bm.Emoji, ":calendar:")
	}
}

func TestFormatBookmark_Minimal(t *testing.T) {
	raw := map[string]any{
		"id":    "Bk2",
		"title": "Dashboard",
		"type":  "link",
	}

	bm := FormatBookmark(raw)
	if bm.Link != "" {
		t.Errorf("Link = %q, want empty", bm.Link)
	}
	if bm.Emoji != "" {
		t.Errorf("Emoji = %q, want empty", bm.Emoji)
	}
}

func TestPinnedItemDisplayText_Message(t *testing.T) {
	pin := PinnedItem{
		Type: "message",
		Text: "Check the runbook",
	}
	got := PinnedItemDisplayText(pin)
	want := `"Check the runbook"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPinnedItemDisplayText_MessageTruncation(t *testing.T) {
	pin := PinnedItem{
		Type: "message",
		Text: "This is a very long message that should be truncated because it exceeds sixty characters in length by quite a bit",
	}
	got := PinnedItemDisplayText(pin)
	runes := []rune(got)
	// Quoted string: 1 (open quote) + 57 + 3 (ellipsis) + 1 (close quote) = 62
	if len(runes) > 65 {
		t.Errorf("display text too long: %d runes: %q", len(runes), got)
	}
}

func TestPinnedItemDisplayText_MessageEmpty(t *testing.T) {
	pin := PinnedItem{
		Type: "message",
		Text: "",
	}
	got := PinnedItemDisplayText(pin)
	if got != `"(no text)"` {
		t.Errorf("got %q, want %q", got, `"(no text)"`)
	}
}

func TestPinnedItemDisplayText_File(t *testing.T) {
	pin := PinnedItem{
		Type:     "file",
		FileName: "design-doc.pdf",
	}
	got := PinnedItemDisplayText(pin)
	if got != "design-doc.pdf" {
		t.Errorf("got %q, want %q", got, "design-doc.pdf")
	}
}

func TestPinnedItemDisplayText_FileNoName(t *testing.T) {
	pin := PinnedItem{
		Type:   "file",
		FileID: "F123",
	}
	got := PinnedItemDisplayText(pin)
	if got != "file F123" {
		t.Errorf("got %q, want %q", got, "file F123")
	}
}
