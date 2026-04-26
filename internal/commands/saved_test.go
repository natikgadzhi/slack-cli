package commands

import (
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// --- extractSavedItems -----------------------------------------------------

func TestExtractSavedItems_NewShape(t *testing.T) {
	// Documented "saved_items" shape: each item wraps a { type, message } ref.
	result := map[string]any{
		"saved_items": []any{
			map[string]any{
				"id":           "Ss1",
				"date_created": float64(1706000000),
				"item": map[string]any{
					"type": "message",
					"message": map[string]any{
						"channel": "C111",
						"ts":      "1700000000.000100",
						"user":    "U1",
						"text":    "hello",
					},
				},
			},
			map[string]any{
				"id":   "Ss2",
				"item": map[string]any{"type": "file"},
			},
		},
	}

	got := extractSavedItems(result)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].channelID != "C111" {
		t.Errorf("channelID = %q", got[0].channelID)
	}
	if got[0].messageTS != "1700000000.000100" {
		t.Errorf("ts = %q", got[0].messageTS)
	}
	if got[0].dateCreated != 1706000000 {
		t.Errorf("dateCreated = %d", got[0].dateCreated)
	}
}

func TestExtractSavedItems_LegacyStarsShape(t *testing.T) {
	// stars.list shape: items[] with type + channel + message peers.
	result := map[string]any{
		"items": []any{
			map[string]any{
				"type":    "message",
				"channel": "C222",
				"message": map[string]any{
					"ts":   "1701000000.000200",
					"user": "U2",
					"text": "legacy",
				},
			},
			map[string]any{"type": "file"},
		},
	}

	got := extractSavedItems(result)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].channelID != "C222" {
		t.Errorf("channelID = %q, want C222", got[0].channelID)
	}
}

func TestExtractSavedItems_SkipsWithoutChannel(t *testing.T) {
	result := map[string]any{
		"saved_items": []any{
			map[string]any{
				"id": "Ss0",
				"item": map[string]any{
					"type":    "message",
					"message": map[string]any{"ts": "1700000000.000100"},
				},
			},
		},
	}
	got := extractSavedItems(result)
	if len(got) != 0 {
		t.Errorf("items without channel should be skipped, got %d", len(got))
	}
}

// --- buildSavedRows --------------------------------------------------------

type fakeUserResolver struct{ m map[string]string }

func (f fakeUserResolver) DisplayName(uid string) string { return f.m[uid] }

func TestBuildSavedRows_EmojiAndMrkdwnReplacement(t *testing.T) {
	items := []savedItem{
		{
			id:          "Ss1",
			dateCreated: 1706000000,
			channelID:   "C111",
			messageTS:   "1700000000.000100",
			message: map[string]any{
				"ts":   "1700000000.000100",
				"user": "alice",
				"text": "done :thread: cc <@U999>",
			},
		},
	}
	chans := savedChannelMap{
		"C111": {id: "C111", displayName: "eng"},
	}
	users := fakeUserResolver{m: map[string]string{"U999": "bob"}}

	rows := buildSavedRows(items, chans, users, "https://team.slack.com", true)
	if len(rows) != 1 {
		t.Fatalf("len rows = %d", len(rows))
	}
	if rows[0].Conversation != "eng" {
		t.Errorf("conversation = %q", rows[0].Conversation)
	}
	if rows[0].Text != "done 🧵 cc @bob" {
		t.Errorf("text = %q", rows[0].Text)
	}
	wantPerma := "https://team.slack.com/archives/C111/p1700000000000100"
	if rows[0].Permalink != wantPerma {
		t.Errorf("permalink = %q, want %q", rows[0].Permalink, wantPerma)
	}
	if rows[0].ConversationURL != "https://team.slack.com/archives/C111" {
		t.Errorf("conversationURL = %q", rows[0].ConversationURL)
	}
}

func TestBuildSavedRows_NoTeamURL(t *testing.T) {
	items := []savedItem{
		{channelID: "C111", messageTS: "1700000000.000100", message: map[string]any{"ts": "1700000000.000100", "text": "hi"}},
	}
	rows := buildSavedRows(items, savedChannelMap{}, fakeUserResolver{}, "", false)
	if rows[0].Permalink != "" || rows[0].ConversationURL != "" {
		t.Errorf("without teamURL no links expected, got %+v", rows[0])
	}
}

func TestSavedRowText_AttachmentFallback(t *testing.T) {
	m := formatting.Message{
		Attachment: &formatting.Attachment{Title: "Build failed", Text: "service red"},
	}
	if got := savedRowText(m); got != "Build failed — service red" {
		t.Errorf("savedRowText = %q", got)
	}
}

// --- parseSavedItem integration with date fallback -------------------------

func TestParseSavedItem_FallsBackDateCreatedToTS(t *testing.T) {
	r := map[string]any{
		"id": "Ss1",
		"item": map[string]any{
			"type":    "message",
			"message": map[string]any{"channel": "C1", "ts": "1700000000.000100"},
		},
	}
	si := parseSavedItem(r)
	if si.dateCreated != 1700000000 {
		t.Errorf("dateCreated = %d, want 1700000000 (derived from ts)", si.dateCreated)
	}
}

// --- hasMessageText --------------------------------------------------------

func TestHasMessageText(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]any
		want bool
	}{
		{"nil", nil, false},
		{"empty", map[string]any{}, false},
		{"only ts", map[string]any{"ts": "123"}, false},
		{"with text", map[string]any{"text": "hi"}, true},
		{"whitespace text", map[string]any{"text": "  "}, false},
		{"attachment", map[string]any{"attachments": []any{map[string]any{}}}, true},
		{"blocks", map[string]any{"blocks": []any{map[string]any{}}}, true},
	}
	for _, c := range cases {
		if got := hasMessageText(c.m); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGetInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{float64(1706000000), 1706000000},
		{int(42), 42},
		{int64(43), 43},
		{"100", 100},
		{"abc", 0},
		{nil, 0},
	}
	for _, c := range cases {
		m := map[string]any{"k": c.in}
		if got := getInt64(m, "k"); got != c.want {
			t.Errorf("getInt64(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
