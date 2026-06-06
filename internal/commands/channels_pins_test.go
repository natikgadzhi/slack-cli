package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// TestFetchPinsAsync verifies that fetchPinsAsync correctly calls pins.list
// and parses pinned items from the response.
func TestFetchPinsAsync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/pins.list") {
			t.Errorf("unexpected path %q", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unexpected"})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("channel"); got != "C123" {
			t.Errorf("channel = %q, want %q", got, "C123")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{
					"type":       "message",
					"created_by": "U111",
					"created":    float64(1706000001),
					"message": map[string]any{
						"text": "Check the runbook before deploying",
						"ts":   "1700000001.000000",
					},
				},
				map[string]any{
					"type":       "file",
					"created_by": "U222",
					"created":    float64(1706000002),
					"file": map[string]any{
						"id":   "F456",
						"name": "design-doc.pdf",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	ch := fetchPinsAsync(client, "C123")
	result := <-ch

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.pins) != 2 {
		t.Fatalf("len(pins) = %d, want 2", len(result.pins))
	}

	pin0 := result.pins[0]
	if pin0.Type != "message" {
		t.Errorf("pin[0].Type = %q, want %q", pin0.Type, "message")
	}
	if pin0.CreatedBy != "U111" {
		t.Errorf("pin[0].CreatedBy = %q, want %q", pin0.CreatedBy, "U111")
	}
	if pin0.Text != "Check the runbook before deploying" {
		t.Errorf("pin[0].Text = %q, want %q", pin0.Text, "Check the runbook before deploying")
	}

	pin1 := result.pins[1]
	if pin1.Type != "file" {
		t.Errorf("pin[1].Type = %q, want %q", pin1.Type, "file")
	}
	if pin1.FileName != "design-doc.pdf" {
		t.Errorf("pin[1].FileName = %q, want %q", pin1.FileName, "design-doc.pdf")
	}
}

// TestFetchBookmarksAsync verifies that fetchBookmarksAsync correctly calls
// bookmarks.list and parses bookmarks from the response.
func TestFetchBookmarksAsync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/bookmarks.list") {
			t.Errorf("unexpected path %q", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unexpected"})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("channel_id"); got != "C123" {
			t.Errorf("channel_id = %q, want %q", got, "C123")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"bookmarks": []any{
				map[string]any{
					"id":    "Bk1",
					"title": "Oncall Schedule",
					"type":  "link",
					"link":  "https://example.com/oncall",
					"emoji": ":calendar:",
				},
				map[string]any{
					"id":    "Bk2",
					"title": "Grafana Dashboard",
					"type":  "link",
					"link":  "https://grafana.example.com",
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	ch := fetchBookmarksAsync(client, "C123")
	result := <-ch

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.bookmarks) != 2 {
		t.Fatalf("len(bookmarks) = %d, want 2", len(result.bookmarks))
	}

	bm0 := result.bookmarks[0]
	if bm0.Title != "Oncall Schedule" {
		t.Errorf("bookmark[0].Title = %q, want %q", bm0.Title, "Oncall Schedule")
	}
	if bm0.Link != "https://example.com/oncall" {
		t.Errorf("bookmark[0].Link = %q, want %q", bm0.Link, "https://example.com/oncall")
	}

	bm1 := result.bookmarks[1]
	if bm1.Title != "Grafana Dashboard" {
		t.Errorf("bookmark[1].Title = %q, want %q", bm1.Title, "Grafana Dashboard")
	}
}

// TestFetchPinsAsync_Error verifies that fetchPinsAsync propagates API errors.
func TestFetchPinsAsync_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "channel_not_found",
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	ch := fetchPinsAsync(client, "C_INVALID")
	result := <-ch

	if result.err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestFetchBookmarksAsync_Empty verifies that an empty bookmarks list is handled.
func TestFetchBookmarksAsync_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"bookmarks": []any{},
		})
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)
	ch := fetchBookmarksAsync(client, "C123")
	result := <-ch

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.bookmarks) != 0 {
		t.Errorf("len(bookmarks) = %d, want 0", len(result.bookmarks))
	}
}

// TestChannelMetadata_JSONRoundTrip verifies that ChannelMetadata serializes
// correctly with all fields populated.
func TestChannelMetadata_JSONRoundTrip(t *testing.T) {
	meta := formatting.ChannelMetadata{
		PinnedItems: []formatting.PinnedItem{
			{
				Type:      "message",
				CreatedBy: "alice",
				Text:      "Important message",
			},
		},
		Bookmarks: []formatting.Bookmark{
			{
				ID:    "Bk1",
				Title: "Dashboard",
				Type:  "link",
				Link:  "https://example.com",
			},
		},
		Messages: []formatting.Message{
			{
				TS:   "1700000001.000000",
				User: "bob",
				Text: "Hello world",
			},
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded formatting.ChannelMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.PinnedItems) != 1 {
		t.Errorf("PinnedItems len = %d, want 1", len(decoded.PinnedItems))
	}
	if len(decoded.Bookmarks) != 1 {
		t.Errorf("Bookmarks len = %d, want 1", len(decoded.Bookmarks))
	}
	if len(decoded.Messages) != 1 {
		t.Errorf("Messages len = %d, want 1", len(decoded.Messages))
	}
	if decoded.PinnedItems[0].Text != "Important message" {
		t.Errorf("PinnedItems[0].Text = %q, want %q", decoded.PinnedItems[0].Text, "Important message")
	}
}

// TestChannelMetadata_OmitsEmptyFields verifies that when no pins/bookmarks
// exist, those fields are omitted from JSON (backward compatibility).
func TestChannelMetadata_OmitsEmptyFields(t *testing.T) {
	meta := formatting.ChannelMetadata{
		Messages: []formatting.Message{
			{TS: "1.0", Text: "msg"},
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	s := string(data)
	if strings.Contains(s, "pinned_items") {
		t.Error("expected pinned_items to be omitted from JSON")
	}
	if strings.Contains(s, "bookmarks") {
		t.Error("expected bookmarks to be omitted from JSON")
	}
	if !strings.Contains(s, "messages") {
		t.Error("expected messages to be present in JSON")
	}
}

// newTestAPIClient is defined in saved_http_test.go — this file reuses it.
// We declare a compile-time check that it exists.
var _ = func() *api.Client {
	return api.NewClient("xoxc", "xoxd")
}
