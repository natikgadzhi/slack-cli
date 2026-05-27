package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natikgadzhi/cli-kit/progress"
)

// TestFetchActivityItems_Pagination verifies that fetchActivityItems follows the
// next_cursor chain, parses mention items, and sends unread_only=true.
func TestFetchActivityItems_Pagination(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/activity.feed") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = r.ParseForm()
		if got := r.FormValue("unread_only"); got != "true" {
			t.Errorf("unread_only = %q, want true", got)
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		switch hits {
		case 1:
			body = map[string]any{
				"ok": true,
				"items": []any{
					map[string]any{
						"feed_ts": "1776600000.000300",
						"item": map[string]any{
							"type":    "at_user",
							"message": map[string]any{"channel": "C1", "ts": "1776600000.000300", "user": "U1", "text": "a"},
						},
					},
				},
				"response_metadata": map[string]any{"next_cursor": "page2"},
			}
		default:
			if got := r.FormValue("cursor"); got != "page2" {
				t.Errorf("cursor = %q, want page2", got)
			}
			body = map[string]any{
				"ok": true,
				"items": []any{
					map[string]any{
						"feed_ts": "1776600001.000000",
						"item": map[string]any{
							"type":    "at_channel",
							"message": map[string]any{"channel": "C2", "ts": "1776600001.000000", "user": "U2", "text": "b"},
						},
					},
				},
				"response_metadata": map[string]any{"next_cursor": ""},
			}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	items, partial, err := fetchActivityItems(client, mentionActivityTypes, 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if partial {
		t.Error("unexpected partial")
	}
	if len(items) != 2 {
		t.Fatalf("len items = %d, want 2", len(items))
	}
	if hits != 2 {
		t.Errorf("hits = %d, want 2 (paginated)", hits)
	}
	if items[0].kind != kindMention || items[1].kind != kindMention {
		t.Errorf("kinds = %q,%q", items[0].kind, items[1].kind)
	}
}

// TestFetchUnreadThreads parses unread_replies and attaches channel + thread_ts.
func TestFetchUnreadThreads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/subscriptions.thread.getView") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"threads": []any{
				map[string]any{
					"root_msg": map[string]any{"channel": "C01THREAD00", "ts": "1776500000.000000", "user": "U9"},
					"unread_replies": []any{
						map[string]any{"type": "message", "ts": "1776590000.000100", "user": "U9", "text": "ping"},
						map[string]any{"type": "message", "ts": "1776590001.000000", "user": "U8", "text": "pong"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	items, err := fetchUnreadThreads(client, 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	for _, it := range items {
		if it.kind != kindThread {
			t.Errorf("kind = %q", it.kind)
		}
		if it.channelID != "C01THREAD00" {
			t.Errorf("channelID = %q", it.channelID)
		}
		if it.threadTS != "1776500000.000000" {
			t.Errorf("threadTS = %q", it.threadTS)
		}
		if getString(it.message, "channel") != "C01THREAD00" {
			t.Errorf("reply message channel not attached: %v", it.message["channel"])
		}
	}
}

// TestFetchUnreadThreads_RespectsLimit stops after `limit` replies.
func TestFetchUnreadThreads_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"threads": []any{
				map[string]any{
					"root_msg": map[string]any{"channel": "C1", "ts": "1.0"},
					"unread_replies": []any{
						map[string]any{"ts": "2.0", "text": "a"},
						map[string]any{"ts": "3.0", "text": "b"},
						map[string]any{"ts": "4.0", "text": "c"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	items, err := fetchUnreadThreads(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2 (limit)", len(items))
	}
}

// TestFetchUnreadCounts parses ims and mpims from client.counts.
func TestFetchUnreadCounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/client.counts") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":  true,
			"ims": []any{map[string]any{"id": "D1", "has_unreads": true, "last_read": "1.0"}},
			"mpims": []any{
				map[string]any{"id": "G1", "has_unreads": true, "last_read": "2.0"},
				map[string]any{"id": "G2", "has_unreads": false, "last_read": "3.0"},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	ims, mpims, err := fetchUnreadCounts(client)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ims) != 1 || ims[0].id != "D1" {
		t.Errorf("ims = %+v", ims)
	}
	if len(mpims) != 2 {
		t.Errorf("mpims = %+v", mpims)
	}
}

// TestFetchConversationUnread_BoundaryFilter verifies the message equal to
// last_read (already read) is dropped and only strictly-newer ones are kept.
func TestFetchConversationUnread_BoundaryFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/conversations.history") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = r.ParseForm()
		if got := r.FormValue("oldest"); got != "1776546930.000000" {
			t.Errorf("oldest = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []any{
				map[string]any{"ts": "1776546931.000000", "text": "newer"},
				map[string]any{"ts": "1776546930.000000", "text": "boundary (already read)"},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	msgs, err := fetchConversationUnread(client, "D01DM000001", "1776546930.000000", 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1 (boundary dropped)", len(msgs))
	}
	if getString(msgs[0], "text") != "newer" {
		t.Errorf("kept wrong message: %v", msgs[0])
	}
	if getString(msgs[0], "channel") != "D01DM000001" {
		t.Errorf("channel not attached: %v", msgs[0]["channel"])
	}
}

// TestFetchConversationUnread_NeverRead verifies that a never-opened
// conversation (last_read == "0000000000.000000") does NOT send an `oldest`
// param (which Slack rejects with invalid_ts_oldest) and returns all messages.
func TestFetchConversationUnread_NeverRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if _, ok := r.Form["oldest"]; ok {
			t.Errorf("oldest should not be sent for a never-read conversation, got %q", r.FormValue("oldest"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []any{
				map[string]any{"ts": "1776580000.000400", "text": "hi there"},
				map[string]any{"ts": "1776580000.000100", "text": "earlier"},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	msgs, err := fetchConversationUnread(client, "D01NEVER000", "0000000000.000000", 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2 (no filtering when never read)", len(msgs))
	}
}

// TestFetchThreadUnread_FiltersByLastRead verifies that conversations.replies is
// used to pull a thread's messages, keeping only those newer than the viewer's
// last_read (the marker the parent message carries), tagged as thread items.
func TestFetchThreadUnread_FiltersByLastRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/conversations.replies") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("ts") != "1776605000.000000" {
			t.Errorf("ts = %q", r.FormValue("ts"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []any{
				map[string]any{"ts": "1776605000.000000", "user": "U2", "text": "root", "last_read": "1776606000.000000"},
				map[string]any{"ts": "1776607000.000000", "user": "U2", "text": "unread one"},
				map[string]any{"ts": "1776609000.000000", "user": "U3", "text": "unread two"},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	items, err := fetchThreadUnread(client, "C1", "1776605000.000000")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2 (root is read, two replies unread)", len(items))
	}
	for _, it := range items {
		if it.kind != kindThread {
			t.Errorf("kind = %q", it.kind)
		}
		if it.threadTS != "1776605000.000000" {
			t.Errorf("threadTS = %q", it.threadTS)
		}
		if getString(it.message, "channel") != "C1" {
			t.Errorf("channel not attached: %v", it.message["channel"])
		}
	}
}

// TestCollectUnread_ExpandsMentionThread verifies the bug fix: a mention inside
// a thread (a single message in the activity feed) triggers a conversations.replies
// fetch that pulls the rest of the thread's unread messages. The mention itself
// is preserved (kind=mention) and the sibling reply is added (kind=thread); both
// share the thread_ts so the table can collapse them.
func TestCollectUnread_ExpandsMentionThread(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/activity.feed"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "items": []any{
				map[string]any{"feed_ts": "1776610000.000000", "item": map[string]any{
					"type":    "at_user",
					"message": map[string]any{"channel": "C1", "ts": "1776608000.000000", "thread_ts": "1776605000.000000", "user": "U2", "text": "<@U1> ping"},
				}},
			}})
		case strings.HasSuffix(r.URL.Path, "/subscriptions.thread.getView"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "threads": []any{}})
		case strings.HasSuffix(r.URL.Path, "/client.counts"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ims": []any{}, "mpims": []any{}})
		case strings.HasSuffix(r.URL.Path, "/conversations.replies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": []any{
				map[string]any{"ts": "1776605000.000000", "user": "U2", "text": "root", "last_read": "1776606000.000000"},
				map[string]any{"ts": "1776608000.000000", "user": "U2", "text": "<@U1> ping"},
				map[string]any{"ts": "1776609000.000000", "user": "U3", "text": "follow-up reply"},
			}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	prog := progress.NewCounter("t", "json")
	items, partial, err := collectUnread(client, mentionActivityTypes, false, 50, prog)
	prog.Finish()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if partial {
		t.Fatal("unexpected partial")
	}

	items = dedupeUnread(items)
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2 (mention + one expanded reply; root read, mention deduped)", len(items))
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.kind]++
		if it.threadTS != "1776605000.000000" {
			t.Errorf("threadTS = %q, want shared thread root", it.threadTS)
		}
	}
	if kinds[kindMention] != 1 || kinds[kindThread] != 1 {
		t.Errorf("kinds = %v, want 1 mention + 1 thread", kinds)
	}
}
