package channels

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// newTestClient creates an api.Client pointing at the given test server URL
// with zero-duration page delays, short timeout, minimal retries, and no 5xx retries.
func newTestClient(serverURL string) *api.Client {
	c := api.NewClient(
		"xoxc-test-token", "xoxd-test-cookie",
		api.WithBaseURL(serverURL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
		api.WithMaxRetries(1),
		api.WithRetryOn5xx(false),
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
	id, err := ResolveChannel(client, "C12345678", nil, false)
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
	id, err := ResolveChannel(client, "#C12345678", nil, false)
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
		if got := r.PostFormValue("types"); got != "public_channel,private_channel" {
			t.Errorf("types = %q, want public_channel,private_channel", got)
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
	id, err := ResolveChannel(client, "general", nil, false)
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
	id, err := ResolveChannel(client, "deployments", nil, false)
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

func TestResolveChannel_EmptyInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call API for empty input")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)

	// Empty string should return an error.
	_, err := ResolveChannel(client, "", nil, false)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "channel name or ID required") {
		t.Errorf("expected 'channel name or ID required' in error, got: %v", err)
	}

	// Just a "#" should also return an error (becomes empty after trimming).
	_, err = ResolveChannel(client, "#", nil, false)
	if err == nil {
		t.Fatal("expected error for '#' input")
	}
	if !strings.Contains(err.Error(), "channel name or ID required") {
		t.Errorf("expected 'channel name or ID required' in error, got: %v", err)
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
			// No next_cursor -> pagination ends after this page.
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := ResolveChannel(client, "nonexistent", nil, false)
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

func TestResolveChannel_CaseInsensitiveMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": "C11111111", "name": "random"},
				map[string]any{"id": "C22222222", "name": "general"},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)

	// "General" should match "general" (Slack stores lowercase).
	id, err := ResolveChannel(client, "General", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C22222222" {
		t.Errorf("expected C22222222, got %q", id)
	}
}

func TestResolveChannel_APIErrorDuringPagination(t *testing.T) {
	// Server returns HTTP 500 on the first call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok": false, "error": "internal_error"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := ResolveChannel(client, "general", nil, false)
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "listing channels") {
		t.Errorf("expected 'listing channels' in error, got: %v", err)
	}

	// Should be wrapping a CLIError (HTTP errors now use cli-kit errors).
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Errorf("expected error to wrap *clierrors.CLIError, got: %T", err)
	}
}

func TestResolveChannel_RateLimitPartialResults_NotFound(t *testing.T) {
	// Page 1: returns channels that don't match, with a next cursor.
	// Page 2: returns HTTP 429 (rate limited).
	// Channel not found in partial results -> should return an error.
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			resp := map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C11111111", "name": "random"},
					map[string]any{"id": "C22222222", "name": "general"},
				},
				"response_metadata": map[string]any{
					"next_cursor": "cursor-page-2",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		default:
			// Return 429 on subsequent pages.
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()

	// Minimal retries so the client gives up quickly on 429.
	client := api.NewClient(
		"xoxc-test-token", "xoxd-test-cookie",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
		api.WithMaxRetries(1),
		api.WithRetryOn5xx(false),
	)
	_, err := ResolveChannel(client, "deployments", nil, false)
	if err == nil {
		t.Fatal("expected error when channel not found in partial results")
	}
	if !strings.Contains(err.Error(), "listing channels") {
		t.Errorf("expected 'listing channels' in error, got: %v", err)
	}

	// Should wrap a RateLimitError.
	var rlErr *api.RateLimitError
	if !errors.As(err, &rlErr) {
		t.Errorf("expected error to wrap *api.RateLimitError, got: %T", err)
	}
}

func TestResolveChannel_RateLimitPartialResults_Found(t *testing.T) {
	// Page 1: returns the channel we're looking for, with a next cursor.
	// Because the channel is found on page 1, the loop returns immediately
	// without fetching page 2 -- so the 429 on page 2 is never encountered.
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			resp := map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C11111111", "name": "random"},
					map[string]any{"id": "C55555555", "name": "deployments"},
				},
				"response_metadata": map[string]any{
					"next_cursor": "cursor-page-2",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		default:
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()

	client := api.NewClient(
		"xoxc-test-token", "xoxd-test-cookie",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
		api.WithMaxRetries(1),
		api.WithRetryOn5xx(false),
	)
	id, err := ResolveChannel(client, "deployments", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C55555555" {
		t.Errorf("expected C55555555, got %q", id)
	}
	// Channel was found on page 1; page 2 must NOT have been fetched.
	if page != 1 {
		t.Errorf("expected exactly 1 page fetched (early termination), got %d", page)
	}
}

func TestResolveChannel_EarlyTermination(t *testing.T) {
	// Channel is on page 1 of a multi-page result set.
	// The second page must never be fetched.
	pagesFetched := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pagesFetched++
		if pagesFetched > 1 {
			t.Error("second page fetched: early termination did not work")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "channels": []any{}})
			return
		}
		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": "C11111111", "name": "random"},
				map[string]any{"id": "C22222222", "name": "general"},
				map[string]any{"id": "C33333333", "name": "engineering"},
			},
			"response_metadata": map[string]any{
				"next_cursor": "cursor-page-2",
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "general", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C22222222" {
		t.Errorf("expected C22222222, got %q", id)
	}
	if pagesFetched != 1 {
		t.Errorf("expected 1 page fetched, got %d", pagesFetched)
	}
}

func TestResolveChannel_MaxPagesSafetyValve(t *testing.T) {
	// Server always returns non-matching channels with a cursor.
	// The safety valve should stop after maxPages.
	pagesFetched := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pagesFetched++
		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": fmt.Sprintf("C%08d", pagesFetched), "name": fmt.Sprintf("other-%d", pagesFetched)},
			},
			"response_metadata": map[string]any{
				"next_cursor": fmt.Sprintf("cursor-page-%d", pagesFetched+1),
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := ResolveChannel(client, "nonexistent", nil, false)
	if err == nil {
		t.Fatal("expected error after exceeding max pages")
	}
	// First pass (active) hits maxPages and returns an error immediately.
	if pagesFetched != maxPages {
		t.Errorf("expected %d pages fetched, got %d", maxPages, pagesFetched)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestResolveChannel_ProgressOutput(t *testing.T) {
	// Verify that progress output is written to the provided writer
	// when paginating through multiple pages.
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		var resp map[string]any
		switch page {
		case 1:
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
			resp = map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C33333333", "name": "deployments"},
				},
			}
		default:
			resp = map[string]any{"ok": true, "channels": []any{}}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	var buf bytes.Buffer
	id, err := ResolveChannel(client, "deployments", &buf, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C33333333" {
		t.Errorf("expected C33333333, got %q", id)
	}

	output := buf.String()
	// Should contain progress messages for both pages.
	if !strings.Contains(output, "Checked 2 channels across 1 pages") {
		t.Errorf("expected progress for page 1 in output, got: %q", output)
	}
	if !strings.Contains(output, "Checked 3 channels across 2 pages") {
		t.Errorf("expected progress for page 2 in output, got: %q", output)
	}
}

func TestResolveChannel_NilProgressWriter(t *testing.T) {
	// Verify that passing nil for progress does not panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"ok": true,
			"channels": []any{
				map[string]any{"id": "C22222222", "name": "general"},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := ResolveChannel(client, "general", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C22222222" {
		t.Errorf("expected C22222222, got %q", id)
	}
}
