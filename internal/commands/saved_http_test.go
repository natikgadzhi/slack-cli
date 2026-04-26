package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natikgadzhi/cli-kit/progress"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// newTestAPIClient creates an api.Client wired to the given test server.
func newTestAPIClient(t *testing.T, serverURL string) *api.Client {
	t.Helper()
	return api.NewClient("xoxc-test", "xoxd-test",
		api.WithBaseURL(serverURL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
	)
}

// TestFetchSavedItems_Pagination verifies that fetchSavedItems follows the
// next_cursor chain and correctly parses the documented "saved_items" shape.
func TestFetchSavedItems_Pagination(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/saved.list") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		switch hits {
		case 1:
			body = map[string]any{
				"ok": true,
				"saved_items": []any{
					map[string]any{
						"id":           "Ss1",
						"date_created": float64(1706000001),
						"item": map[string]any{
							"type": "message",
							"message": map[string]any{
								"channel": "C111",
								"ts":      "1700000001.000000",
								"text":    "first",
							},
						},
					},
				},
				"response_metadata": map[string]any{"next_cursor": "page2"},
			}
		default:
			body = map[string]any{
				"ok": true,
				"saved_items": []any{
					map[string]any{
						"id":           "Ss2",
						"date_created": float64(1706000002),
						"item": map[string]any{
							"type": "message",
							"message": map[string]any{
								"channel": "C222",
								"ts":      "1700000002.000000",
								"text":    "second",
							},
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
	prog := progress.NewCounter("test", "json") // json → silent

	items, partial, err := fetchSavedItems(client, 10, prog)
	if err != nil {
		t.Fatalf("fetchSavedItems returned error: %v", err)
	}
	if partial {
		t.Fatal("unexpected partial")
	}
	if len(items) != 2 {
		t.Fatalf("len items = %d, want 2", len(items))
	}
	if hits != 2 {
		t.Errorf("hits = %d, want 2 (paginated)", hits)
	}
}

// TestFetchSavedChannel_MpimParticipants verifies that an mpim channel's
// display name is built from its participant list rather than Slack's raw
// "mpdm-..." name.
func TestFetchSavedChannel_MpimParticipants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/conversations.info"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channel": map[string]any{
					"id":      "G1",
					"name":    "mpdm-alice--bob--charlie-1",
					"is_mpim": true,
					"members": []any{"U1", "U2", "U3"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/users.info"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"user": map[string]any{
					"id":        r.FormValue("user"),
					"real_name": map[string]string{"U1": "Alice", "U2": "Bob", "U3": "Charlie"}[r.FormValue("user")],
				},
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	client := newTestAPIClient(t, srv.URL)

	// Use an isolated user cache file so the test doesn't touch the real one.
	t.Setenv("SLACK_USER_CACHE", t.TempDir()+"/users.json")
	resolver, err := users.NewUserResolver(client)
	if err != nil {
		t.Fatal(err)
	}

	ch := fetchSavedChannel(client, resolver, "G1")
	if !ch.isMpim {
		t.Error("expected isMpim")
	}
	if ch.displayName != "Alice, Bob, Charlie" {
		t.Errorf("displayName = %q, want %q", ch.displayName, "Alice, Bob, Charlie")
	}
}
