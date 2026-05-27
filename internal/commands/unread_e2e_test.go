package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/natikgadzhi/cli-kit/progress"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// newUnreadTestServer wires an httptest.Server answering every endpoint the
// unread command touches, from fixtures. It also enforces the read-only
// contract: any request to a *.mark* (or otherwise mutating) endpoint fails the
// test, proving `slack-cli unread` never marks anything as read.
func newUnreadTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		// Read-only contract: reject any mutating endpoint outright.
		if strings.Contains(path, "mark") ||
			strings.Contains(path, "clearAllUnreads") ||
			strings.Contains(path, "setPresence") {
			t.Errorf("read-only violation: unread called mutating endpoint %q", path)
			_, _ = w.Write([]byte(`{"ok":false,"error":"read_only_violation"}`))
			return
		}

		body, _ := io.ReadAll(r.Body)
		form := string(body)

		switch {
		case strings.HasSuffix(path, "/activity.feed"):
			writeFilteredActivityFeed(t, w, form)
		case strings.HasSuffix(path, "/subscriptions.thread.getView"):
			_, _ = w.Write(loadFixture(t, "subscriptions.thread.getView.json"))
		case strings.HasSuffix(path, "/client.counts"):
			_, _ = w.Write(loadFixture(t, "client.counts.json"))
		case strings.HasSuffix(path, "/conversations.history"):
			switch {
			case strings.Contains(form, "channel=D01DM000001"):
				_, _ = w.Write(loadFixture(t, "conversations.history.unread.dm.json"))
			case strings.Contains(form, "channel=G01MPIM0001"):
				_, _ = w.Write(loadFixture(t, "conversations.history.unread.mpim.json"))
			case strings.Contains(form, "channel=D01APPDM001"):
				_, _ = w.Write(loadFixture(t, "conversations.history.unread.app.json"))
			default:
				t.Errorf("unexpected conversations.history form: %s", form)
				_, _ = w.Write([]byte(`{"ok":true,"messages":[]}`))
			}
		case strings.HasSuffix(path, "/conversations.info"):
			_, _ = w.Write([]byte(unreadConversationsInfo(valueFromForm(form, "channel"))))
		case strings.HasSuffix(path, "/conversations.members"):
			_, _ = w.Write([]byte(unreadConversationsMembers(valueFromForm(form, "channel"))))
		case strings.HasSuffix(path, "/users.info"):
			_, _ = w.Write([]byte(unreadUsersInfo(valueFromForm(form, "user"))))
		case strings.HasSuffix(path, "/auth.test"):
			_, _ = w.Write([]byte(`{"ok":true,"url":"https://test.slack.com/","user_id":"U01NATIK00"}`))
		default:
			t.Errorf("unexpected endpoint %q", path)
			_, _ = w.Write([]byte(`{"ok":false,"error":"not_handled"}`))
		}
	}))
}

// writeFilteredActivityFeed simulates Slack's server-side `types` filtering: it
// returns only the fixture items whose item.type is in the requested set. This
// is what makes the --include-reactions / --include-apps gating testable.
func writeFilteredActivityFeed(t *testing.T, w http.ResponseWriter, form string) {
	t.Helper()
	values, err := url.ParseQuery(form)
	if err != nil {
		t.Fatalf("parse activity.feed form: %v", err)
	}
	requested := map[string]bool{}
	for _, ty := range strings.Split(values.Get("types"), ",") {
		if ty != "" {
			requested[ty] = true
		}
	}

	var feed map[string]any
	if err := json.Unmarshal(loadFixture(t, "activity.feed.json"), &feed); err != nil {
		t.Fatalf("decode activity.feed fixture: %v", err)
	}
	rawItems, _ := feed["items"].([]any)
	kept := make([]any, 0, len(rawItems))
	for _, ri := range rawItems {
		m, _ := ri.(map[string]any)
		inner, _ := m["item"].(map[string]any)
		ty, _ := inner["type"].(string)
		if requested[ty] {
			kept = append(kept, ri)
		}
	}
	feed["items"] = kept
	_ = json.NewEncoder(w).Encode(feed)
}

func unreadConversationsInfo(cid string) string {
	switch cid {
	case "C01ENG00000":
		return `{"ok":true,"channel":{"id":"C01ENG00000","name":"eng","is_private":false}}`
	case "C01THREAD00":
		return `{"ok":true,"channel":{"id":"C01THREAD00","name":"planning","is_private":false}}`
	case "D01DM000001":
		return `{"ok":true,"channel":{"id":"D01DM000001","is_im":true,"user":"U01DIVYA00"}}`
	case "D01APPDM001":
		return `{"ok":true,"channel":{"id":"D01APPDM001","is_im":true,"user":"U01APPBOT0"}}`
	case "G01MPIM0001":
		return `{"ok":true,"channel":{"id":"G01MPIM0001","is_mpim":true,"name":"mpdm-natik--alice--bob-1"}}`
	}
	return `{"ok":false,"error":"channel_not_found"}`
}

func unreadConversationsMembers(cid string) string {
	if cid == "G01MPIM0001" {
		return `{"ok":true,"members":["U01NATIK00","U01ALICE00","U01BOB0000"]}`
	}
	return `{"ok":true,"members":[]}`
}

func unreadUsersInfo(uid string) string {
	names := map[string]string{
		"U01MIKE000": "Mike Turco",
		"U01NATIK00": "Natik",
		"U01DIVYA00": "Divya",
		"U01MEG0000": "meghana",
		"U01ALICE00": "Alice",
		"U01BOB0000": "Bob",
		"U01APPBOT0": "deploybot",
	}
	name := names[uid]
	if name == "" {
		name = uid
	}
	return `{"ok":true,"user":{"id":"` + uid + `","real_name":"` + name + `","profile":{"display_name":"` + name + `"}}}`
}

// runUnreadPipeline mirrors the data path of runUnread (minus flag parsing and
// rendering) so the e2e test can assert on the resulting rows.
func runUnreadPipeline(t *testing.T, client *api.Client, resolver *users.UserResolver, types []string, includeApps bool) []unreadRow {
	t.Helper()
	prog := progress.NewCounter("t", "json")
	items, partial, err := collectUnread(client, types, includeApps, 50, prog)
	prog.Finish()
	if err != nil {
		t.Fatalf("collectUnread: %v", err)
	}
	if partial {
		t.Fatal("unexpected partial")
	}
	items = dedupeUnread(items)

	sort.SliceStable(items, func(i, j int) bool { return items[i].dateTS > items[j].dateTS })
	hydrateUnreadMessages(client, items)
	chans := resolveUnreadChannels(client, resolver, items)
	resolveUnreadUsers(resolver, items, "json")
	rows, _ := buildUnreadRows(items, chans, resolver, "https://test.slack.com", true)
	return rows
}

// TestUnread_E2E_Default verifies the default pipeline: mentions + DMs + group
// DMs + threads, with reactions/apps EXCLUDED, correctly ordered and resolved.
func TestUnread_E2E_Default(t *testing.T) {
	srv := newUnreadTestServer(t)
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("SLACK_USER_CACHE", filepath.Join(tmp, "users.json"))
	t.Setenv("SLACK_DATA_DIR", tmp)

	client := api.NewClient("xoxc-test", "xoxd-test", api.WithBaseURL(srv.URL))
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		t.Fatal(err)
	}

	rows := runUnreadPipeline(t, client, resolver, mentionActivityTypes, false)

	asJSON, _ := json.MarshalIndent(rows, "", "  ")

	// mention + 2 thread replies + dm + group_dm. JSON keeps every thread reply
	// (the table collapses them; see TestRenderUnreadTable_CollapsesThread).
	if len(rows) != 5 {
		t.Fatalf("len rows = %d, want 5\n%s", len(rows), asJSON)
	}
	threadRows := 0
	for _, r := range rows {
		if r.Kind == kindThread {
			threadRows++
		}
	}
	if threadRows != 2 {
		t.Errorf("thread rows = %d, want 2 (JSON keeps all replies)\n%s", threadRows, asJSON)
	}

	// Reactions and apps must NOT appear without their flags.
	for _, r := range rows {
		if r.Kind == kindReaction || r.Kind == kindApp {
			t.Errorf("kind %q should be excluded by default\n%s", r.Kind, asJSON)
		}
	}

	// All four expected kinds present.
	kinds := map[string]unreadRow{}
	for _, r := range rows {
		kinds[r.Kind] = r
	}
	for _, want := range []string{kindMention, kindThread, kindDM, kindGroupDM} {
		if _, ok := kinds[want]; !ok {
			t.Errorf("missing kind %q\n%s", want, asJSON)
		}
	}

	// Reverse-chronological: mention (highest ts) first, group_dm (lowest) last.
	if rows[0].Kind != kindMention {
		t.Errorf("first row kind = %q, want mention\n%s", rows[0].Kind, asJSON)
	}
	if rows[len(rows)-1].Kind != kindGroupDM {
		t.Errorf("last row kind = %q, want group_dm\n%s", rows[len(rows)-1].Kind, asJSON)
	}

	// Conversation names resolved.
	if got := kinds[kindMention].Conversation; got != "eng" {
		t.Errorf("mention conversation = %q, want eng", got)
	}
	if got := kinds[kindThread].Conversation; got != "planning" {
		t.Errorf("thread conversation = %q, want planning", got)
	}
	if got := kinds[kindDM].Conversation; got != "@Divya" {
		t.Errorf("dm conversation = %q, want @Divya", got)
	}
	for _, want := range []string{"Natik", "Alice", "Bob"} {
		if !strings.Contains(kinds[kindGroupDM].Conversation, want) {
			t.Errorf("group_dm conversation %q missing %q", kinds[kindGroupDM].Conversation, want)
		}
	}

	// Mention: <@U01NATIK00> -> @Natik, :thread: -> 🧵, author resolved to Mike Turco.
	m := kinds[kindMention]
	if !strings.Contains(m.Text, "@Natik") || !strings.Contains(m.Text, "🧵") {
		t.Errorf("mention text = %q", m.Text)
	}
	if m.User != "Mike Turco" {
		t.Errorf("mention author = %q, want Mike Turco", m.User)
	}

	// Thread carries thread_ts and resolves the mention.
	th := kinds[kindThread]
	if th.ThreadTS != "1776500000.000000" {
		t.Errorf("thread_ts = %q", th.ThreadTS)
	}
	if !strings.Contains(th.Text, "@Natik") {
		t.Errorf("thread text = %q", th.Text)
	}

	// DM: <#C01ENG00000|eng> -> #eng, :wave: -> 👋. (Boundary msg dropped.)
	dm := kinds[kindDM]
	if !strings.Contains(dm.Text, "#eng") || !strings.Contains(dm.Text, "👋") {
		t.Errorf("dm text = %q", dm.Text)
	}
	if strings.Contains(dm.Text, "already read") {
		t.Errorf("boundary (already-read) message leaked: %q", dm.Text)
	}

	// Permalinks built against the test team URL.
	for _, r := range rows {
		if !strings.HasPrefix(r.Permalink, "https://test.slack.com/archives/") {
			t.Errorf("unexpected permalink %q", r.Permalink)
		}
	}
}

// TestUnread_E2E_WithFlags verifies --include-reactions and --include-apps add
// the reaction and bot-DM rows.
func TestUnread_E2E_WithFlags(t *testing.T) {
	srv := newUnreadTestServer(t)
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("SLACK_USER_CACHE", filepath.Join(tmp, "users.json"))
	t.Setenv("SLACK_DATA_DIR", tmp)

	client := api.NewClient("xoxc-test", "xoxd-test", api.WithBaseURL(srv.URL))
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		t.Fatal(err)
	}

	// Reactions come from the activity feed; app/bot DMs come from the IMs path
	// gated by includeApps (not from the activity feed).
	types := append(append([]string{}, mentionActivityTypes...), activityTypeReaction)
	rows := runUnreadPipeline(t, client, resolver, types, true)

	asJSON, _ := json.MarshalIndent(rows, "", "  ")
	// mention + reaction + 2 thread replies + dm + group_dm + app.
	if len(rows) != 7 {
		t.Fatalf("len rows = %d, want 7\n%s", len(rows), asJSON)
	}

	var reaction, app *unreadRow
	for i := range rows {
		switch rows[i].Kind {
		case kindReaction:
			reaction = &rows[i]
		case kindApp:
			app = &rows[i]
		}
	}
	if reaction == nil {
		t.Fatalf("no reaction row\n%s", asJSON)
	}
	if !strings.Contains(reaction.Text, "👍") || !strings.Contains(reaction.Text, "from @Alice") || !strings.Contains(reaction.Text, "shipped") {
		t.Errorf("reaction text = %q", reaction.Text)
	}

	if app == nil {
		t.Fatalf("no app row\n%s", asJSON)
	}
	if app.Conversation != "@deploybot" {
		t.Errorf("app conversation = %q, want @deploybot", app.Conversation)
	}
	if !strings.Contains(app.Text, "Deploy succeeded") || !strings.Contains(app.Text, "🚀") {
		t.Errorf("app text = %q", app.Text)
	}
}

// TestRenderUnreadTable_LongConversationStaysClickable locks in the fix for the
// reported bug: a conversation name too long for its column must still render as
// an OSC-8 hyperlink (cli-kit's table strips ANSI when it truncates a cell, so
// we truncate the name ourselves and hyperlink the truncated text).
func TestRenderUnreadTable_LongConversationStaysClickable(t *testing.T) {
	convURL := "https://test.slack.com/archives/G01LONGMPIM"
	rows := []unreadRow{
		{
			Conversation:    "Natik, Alice, Bob, Carol, Dave, Eve, Frank, Grace, Heidi",
			ConversationURL: convURL,
			Kind:            kindGroupDM,
			Date:            "19 Apr 2026 09:13",
			Permalink:       "https://test.slack.com/archives/G01LONGMPIM/p1776590000000100",
			Text:            strings.Repeat("a long message that forces the message column to be the widest one ", 3),
		},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	renderUnreadTable(rows, true)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	// The OSC-8 hyperlink opener carrying the conversation URL must be present.
	if !strings.Contains(out, "\033]8;;"+convURL+"\033\\") {
		t.Errorf("conversation hyperlink missing/stripped; output:\n%q", out)
	}
	// The name should have been truncated with an ellipsis (it exceeds the cap).
	if !strings.Contains(out, "…") {
		t.Errorf("expected truncated conversation name with ellipsis; output:\n%q", out)
	}
	// No KIND column header.
	if strings.Contains(out, "KIND") {
		t.Errorf("KIND column should be gone; output:\n%q", out)
	}
}

// TestRenderUnreadTable_CollapsesThread verifies that several unread replies in
// one thread render as a single table line — the latest reply, annotated with
// the message count — while non-thread rows are untouched. Rows arrive
// newest-first (as runUnread sorts them).
func TestRenderUnreadTable_CollapsesThread(t *testing.T) {
	rows := []unreadRow{
		{Kind: kindMention, Conversation: "eng", Date: "19 Apr 2026 10:00", Text: "ping you"},
		{Kind: kindThread, Conversation: "planning", ThreadTS: "T1", Date: "19 Apr 2026 09:30", User: "alice", Text: "latest reply here"},
		{Kind: kindThread, Conversation: "planning", ThreadTS: "T1", Date: "19 Apr 2026 09:00", User: "meg", Text: "older reply here"},
		{Kind: kindDM, Conversation: "@Divya", Date: "18 Apr 2026 21:15", Text: "hi"},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	renderUnreadTable(rows, false)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "latest reply here") {
		t.Errorf("expected the latest thread reply; output:\n%s", out)
	}
	if strings.Contains(out, "older reply here") {
		t.Errorf("older thread reply should be collapsed away; output:\n%s", out)
	}
	if !strings.Contains(out, "[+1]") {
		t.Errorf("expected a [+1] badge for the one collapsed reply; output:\n%s", out)
	}
	// Non-thread rows are untouched.
	if !strings.Contains(out, "ping you") || !strings.Contains(out, "hi") {
		t.Errorf("non-thread rows missing; output:\n%s", out)
	}
}

// TestRenderUnreadTable_GroupsMentionWithThread verifies that a mention living
// inside a thread collapses together with that thread's other replies into a
// single table line (the latest message + count), not separate rows.
func TestRenderUnreadTable_GroupsMentionWithThread(t *testing.T) {
	rows := []unreadRow{
		{Kind: kindMention, Conversation: "eng", ThreadTS: "T9", Date: "19 Apr 2026 10:00", User: "mike", Text: "ping you here"},
		{Kind: kindThread, Conversation: "eng", ThreadTS: "T9", Date: "19 Apr 2026 09:30", User: "alice", Text: "a thread reply"},
		{Kind: kindThread, Conversation: "eng", ThreadTS: "T9", Date: "19 Apr 2026 09:00", User: "bob", Text: "another thread reply"},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	renderUnreadTable(rows, false)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "ping you here") {
		t.Errorf("expected the latest message (the mention); output:\n%s", out)
	}
	if strings.Contains(out, "a thread reply") || strings.Contains(out, "another thread reply") {
		t.Errorf("thread replies should be collapsed into the mention's line; output:\n%s", out)
	}
	if !strings.Contains(out, "[+2]") {
		t.Errorf("expected a [+2] badge for the 2 collapsed replies; output:\n%s", out)
	}
}

// TestRenderUnreadTable_CollapsesDM verifies that several unread messages from
// the same DM collapse to one line (latest + [+N]), while a different DM and a
// channel mention stay on their own rows.
func TestRenderUnreadTable_CollapsesDM(t *testing.T) {
	rows := []unreadRow{
		{Kind: kindDM, Conversation: "@Alice", ConversationURL: "u/DALICE", Date: "19 Apr 2026 10:00", User: "Alice", Text: "third from alice"},
		{Kind: kindDM, Conversation: "@Alice", ConversationURL: "u/DALICE", Date: "19 Apr 2026 09:30", User: "Alice", Text: "second from alice"},
		{Kind: kindDM, Conversation: "@Alice", ConversationURL: "u/DALICE", Date: "19 Apr 2026 09:00", User: "Alice", Text: "first from alice"},
		{Kind: kindDM, Conversation: "@Bob", ConversationURL: "u/DBOB", Date: "19 Apr 2026 08:00", User: "Bob", Text: "hey from bob"},
		{Kind: kindMention, Conversation: "eng", ConversationURL: "u/CENG", Date: "19 Apr 2026 07:00", User: "carol", Text: "ping in channel"},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	renderUnreadTable(rows, false)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	// Alice's DM collapses to the latest message with a [+2] badge.
	if !strings.Contains(out, "third from alice") || !strings.Contains(out, "[+2]") {
		t.Errorf("expected Alice's latest message with [+2]; output:\n%s", out)
	}
	if strings.Contains(out, "second from alice") || strings.Contains(out, "first from alice") {
		t.Errorf("earlier DM messages should be collapsed away; output:\n%s", out)
	}
	// Bob's single DM and the channel mention are untouched (no badge).
	if !strings.Contains(out, "hey from bob") || !strings.Contains(out, "ping in channel") {
		t.Errorf("other rows missing; output:\n%s", out)
	}
}
