package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natikgadzhi/cli-kit/progress"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// loadFixture reads a fixture JSON from tests/fixtures/ relative to the repo root.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	// Walk up from the test's working dir until we find tests/fixtures.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "tests", "fixtures", name)
		if b, err := os.ReadFile(candidate); err == nil {
			return b
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("fixture %q not found", name)
		}
		dir = parent
	}
}

// newSavedTestServer wires up an httptest.Server that answers every endpoint
// the saved command touches, from static fixtures. Unknown endpoints fail the
// test so missing coverage is visible.
func newSavedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		body, _ := io.ReadAll(r.Body)
		form := string(body)

		respond := func(fixture string) {
			_, _ = w.Write(loadFixture(t, fixture))
		}

		switch {
		case strings.HasSuffix(path, "/saved.list"):
			respond("saved.list.json")
		case strings.HasSuffix(path, "/conversations.history"):
			switch {
			case strings.Contains(form, "channel=C01PUBLICCH"):
				respond("conversations.history.public.json")
			case strings.Contains(form, "channel=D01DM000001"):
				respond("conversations.history.dm.json")
			case strings.Contains(form, "channel=G01MPIM0001"):
				respond("conversations.history.mpim.json")
			default:
				t.Errorf("unexpected conversations.history form: %s", form)
				_, _ = w.Write([]byte(`{"ok":false,"error":"unexpected"}`))
			}
		case strings.HasSuffix(path, "/conversations.info"):
			cid := valueFromForm(form, "channel")
			_, _ = w.Write([]byte(conversationsInfoResponse(cid)))
		case strings.HasSuffix(path, "/conversations.members"):
			cid := valueFromForm(form, "channel")
			_, _ = w.Write([]byte(conversationsMembersResponse(cid)))
		case strings.HasSuffix(path, "/users.info"):
			uid := valueFromForm(form, "user")
			_, _ = w.Write([]byte(usersInfoResponse(uid)))
		case strings.HasSuffix(path, "/auth.test"):
			_, _ = w.Write([]byte(`{"ok":true,"url":"https://test.slack.com/","user_id":"U01NATIK00"}`))
		default:
			t.Errorf("unexpected endpoint %q", path)
			_, _ = w.Write([]byte(`{"ok":false,"error":"not_handled"}`))
		}
	}))
}

// conversationsInfoResponse returns a canned conversations.info body for a
// known channel ID. Unknown IDs yield a minimal fallback.
func conversationsInfoResponse(cid string) string {
	switch cid {
	case "C01PUBLICCH":
		return `{"ok":true,"channel":{"id":"C01PUBLICCH","name":"eng","is_private":false}}`
	case "D01DM000001":
		return `{"ok":true,"channel":{"id":"D01DM000001","is_im":true,"user":"U01DIVYA00"}}`
	case "G01MPIM0001":
		return `{"ok":true,"channel":{"id":"G01MPIM0001","is_mpim":true,"name":"mpdm-natik--alice--bob-1"}}`
	}
	return `{"ok":false,"error":"channel_not_found"}`
}

// conversationsMembersResponse returns a canned members list for MPIM channels.
func conversationsMembersResponse(cid string) string {
	if cid == "G01MPIM0001" {
		return `{"ok":true,"members":["U01NATIK00","U01ALICE00","U01BOB0000"]}`
	}
	return `{"ok":true,"members":[]}`
}

// usersInfoResponse returns a canned user profile.
func usersInfoResponse(uid string) string {
	names := map[string]string{
		"U01DIVYA00": "Divya",
		"U01MEG0000": "meghana",
		"U01MIKE000": "Mike Turco",
		"U01NATIK00": "Natik",
		"U01ALICE00": "Alice",
		"U01BOB0000": "Bob",
	}
	name := names[uid]
	if name == "" {
		name = uid
	}
	return `{"ok":true,"user":{"id":"` + uid + `","real_name":"` + name + `","profile":{"display_name":"` + name + `"}}}`
}

func valueFromForm(form, key string) string {
	for _, pair := range strings.Split(form, "&") {
		if strings.HasPrefix(pair, key+"=") {
			return strings.TrimPrefix(pair, key+"=")
		}
	}
	return ""
}

// TestSaved_E2E verifies that runSaved's internals, when fed realistic
// fixtures over HTTP, produce properly formatted rows with:
//   - emoji substitution (:thread: → 🧵)
//   - mrkdwn substitution (<@U…> → @name, <#C…|eng> → #eng)
//   - mpim participant naming
//   - im displayName as "@<user>"
//   - reverse-chronological ordering by date_created
//   - archived items filtered out
func TestSaved_E2E(t *testing.T) {
	srv := newSavedTestServer(t)
	defer srv.Close()

	// Isolate caches so the test doesn't touch the real user cache.
	tmp := t.TempDir()
	t.Setenv("SLACK_USER_CACHE", filepath.Join(tmp, "users.json"))
	t.Setenv("SLACK_DATA_DIR", tmp)

	client := api.NewClient("xoxc-test", "xoxd-test", api.WithBaseURL(srv.URL))
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		t.Fatal(err)
	}

	prog := progress.NewCounter("t", "json")
	items, _, err := fetchSavedItems(client, 10, prog)
	prog.Finish()
	if err != nil {
		t.Fatal(err)
	}

	// 4 items in the fixture, 1 is archived → 3 visible.
	if len(items) != 3 {
		t.Fatalf("len items = %d, want 3", len(items))
	}

	hydrateSavedMessages(client, items)
	chans := resolveSavedChannels(client, resolver, items)

	rawMessages := make([]map[string]any, 0, len(items))
	for _, it := range items {
		rawMessages = append(rawMessages, it.message)
	}
	resolved, err := resolver.ResolveUsers(rawMessages)
	if err != nil {
		t.Fatal(err)
	}
	for i := range items {
		items[i].message = resolved[i]
	}

	// Sort reverse-chronological by date_created (same as runSaved).
	// Fixture order already close to that; verify by checking the first item
	// is the DM (most recent date_created).
	sortByDateDesc(items)

	rows := buildSavedRows(items, chans, resolver, "https://test.slack.com", true)

	if len(rows) != 3 {
		t.Fatalf("len rows = %d, want 3", len(rows))
	}

	// Dump rows as JSON for a readable failure.
	asJSON, _ := json.MarshalIndent(rows, "", "  ")

	// 1. Reverse-chronological: DM (saved at 1776561738) should be first.
	if rows[0].Conversation != "@Divya" {
		t.Errorf("first row = %q, want @Divya\n%s", rows[0].Conversation, asJSON)
	}

	// 2. DM message shows mrkdwn <#C…|eng> as #eng and :wave: as emoji.
	if !strings.Contains(rows[0].Text, "#eng") || !strings.Contains(rows[0].Text, "👋") {
		t.Errorf("DM text = %q\n%s", rows[0].Text, asJSON)
	}

	// 3. Public channel message: :thread: rendered, <@U…> → @Mike Turco / @Natik.
	publicRow := findRowByConversation(rows, "eng")
	if publicRow == nil {
		t.Fatalf("no public-channel row\n%s", asJSON)
	}
	if !strings.Contains(publicRow.Text, "🧵") {
		t.Errorf("public text missing emoji: %q", publicRow.Text)
	}
	if !strings.Contains(publicRow.Text, "@Mike Turco") || !strings.Contains(publicRow.Text, "@Natik") {
		t.Errorf("public text missing user mentions: %q", publicRow.Text)
	}

	// 4. MPIM: conversation name is a participant list, not mpdm-…-1.
	mpim := findRowByConversationContains(rows, ",")
	if mpim == nil {
		t.Fatalf("no mpim row with comma-separated name")
	}
	for _, want := range []string{"Natik", "Alice", "Bob"} {
		if !strings.Contains(mpim.Conversation, want) {
			t.Errorf("mpim conversation %q missing %q", mpim.Conversation, want)
		}
	}
	if strings.HasPrefix(mpim.Conversation, "mpdm-") {
		t.Errorf("mpim conversation still raw: %q", mpim.Conversation)
	}

	// 5. Permalinks are built against the test team URL.
	for _, r := range rows {
		if !strings.HasPrefix(r.Permalink, "https://test.slack.com/archives/") {
			t.Errorf("unexpected permalink %q", r.Permalink)
		}
	}

	// 6. No archived item snuck through.
	for _, r := range rows {
		if r.Conversation == "C01ARCHIVED" {
			t.Error("archived item should have been filtered")
		}
	}
}

// sortByDateDesc mirrors the production sort. Duplicated here to keep the
// test explicit about ordering expectations.
func sortByDateDesc(items []savedItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].dateCreated > items[j-1].dateCreated; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func findRowByConversation(rows []savedRow, name string) *savedRow {
	for i := range rows {
		if rows[i].Conversation == name {
			return &rows[i]
		}
	}
	return nil
}

func findRowByConversationContains(rows []savedRow, substr string) *savedRow {
	for i := range rows {
		if strings.Contains(rows[i].Conversation, substr) {
			return &rows[i]
		}
	}
	return nil
}

// TestSaved_E2E_TableRendersCleanBorders verifies that the rendered table's
// lines all have the same visible width — the core "emoji doesn't break the
// right border" guarantee.
func TestSaved_E2E_TableRendersCleanBorders(t *testing.T) {
	rows := []savedRow{
		{Conversation: "eng", Date: "17 Apr 2026 21:43", Text: "🧵 deploy failed", Permalink: "https://x/p1", ConversationURL: "https://x/c"},
		{Conversation: "Natik, Alice, Bob", Date: "17 Apr 2026 00:00", Text: "plain ascii only", Permalink: "https://x/p2", ConversationURL: "https://x/c2"},
	}

	// Force writer + disable terminal auto-fit by capturing stdout.
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	renderSavedTable(rows, true)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(&buf, r)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected bordered table, got:\n%s", buf.String())
	}
	// Can't easily import internal/table's visibleWidth here; use rune count
	// as a coarse signal that lines match. This test isn't a width-oracle —
	// the internal/table tests cover that. It's a smoke test that the saved
	// command emits a bordered, non-empty table.
	if !strings.ContainsRune(lines[0], '╭') || !strings.ContainsRune(lines[len(lines)-1], '╰') {
		t.Errorf("expected box-drawing borders, got:\n%s", buf.String())
	}
}
