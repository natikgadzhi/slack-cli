package channels

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// newTestClient creates an api.Client pointing at the given test server URL
// with zero-duration page delays and short timeout.
func newTestClient(serverURL string) *api.Client {
	c := api.NewClient(
		"xoxc-test-token", "xoxd-test-cookie",
		api.WithBaseURL(serverURL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
	)
	return c
}

func TestResolveChannel_IDPassthrough(t *testing.T) {
	// Should not make any API calls for a channel ID.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call API for ID passthrough")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "C12345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C12345678" {
		t.Errorf("expected C12345678, got %q", id)
	}
}

func TestResolveChannel_HashPrefixedID(t *testing.T) {
	// Should strip the # and return the ID without any API calls.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call API for hash-prefixed ID passthrough")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "#C12345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C12345678" {
		t.Errorf("expected C12345678, got %q", id)
	}
}

func TestResolveChannel_NameFoundFirstPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		// Verify the expected params are sent.
		if got := r.PostFormValue("limit"); got != "200" {
			t.Errorf("limit = %q, want 200", got)
		}
		if got := r.PostFormValue("exclude_archived"); got != "true" {
			t.Errorf("exclude_archived = %q, want true", got)
		}
		if got := r.PostFormValue("types"); got != "public_channel,private_channel,mpim,im" {
			t.Errorf("types = %q", got)
		}

		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": "C11111111", "name": "random"},
				map[string]any{"id": "C22222222", "name": "general"},
				map[string]any{"id": "C33333333", "name": "engineering"},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C22222222" {
		t.Errorf("expected C22222222, got %q", id)
	}
}

func TestResolveChannel_NameFoundSecondPage(t *testing.T) {
	page := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}

		page++
		var resp map[string]any

		switch page {
		case 1:
			// First page: channel not here, but there is a next cursor.
			if cursor := r.PostFormValue("cursor"); cursor != "" {
				t.Errorf("first page should not have cursor, got %q", cursor)
			}
			resp = map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C11111111", "name": "random"},
					map[string]any{"id": "C22222222", "name": "general"},
				},
				"response_metadata": map[string]any{
					"next_cursor": "cursor-page-2",
				},
			}
		case 2:
			// Second page: channel is here.
			if cursor := r.PostFormValue("cursor"); cursor != "cursor-page-2" {
				t.Errorf("second page cursor = %q, want cursor-page-2", cursor)
			}
			resp = map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C44444444", "name": "deployments"},
				},
			}
		default:
			t.Errorf("unexpected page %d", page)
			resp = map[string]any{"ok": true, "channels": []any{}}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "deployments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C44444444" {
		t.Errorf("expected C44444444, got %q", id)
	}
	if page != 2 {
		t.Errorf("expected 2 pages fetched, got %d", page)
	}
}

func TestResolveChannel_NameNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": "C11111111", "name": "random"},
				map[string]any{"id": "C22222222", "name": "general"},
			},
			// No next_cursor → pagination ends after this page.
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := ResolveChannel(client, "nonexistent")
	if err == nil {
		t.Fatal("expected error for channel not found")
	}
	if !strings.Contains(err.Error(), "channel not found") {
		t.Errorf("expected 'channel not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected channel name in error, got: %v", err)
	}
}
