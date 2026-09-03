package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunChannelsSearch_PartialResultsOnMidPaginationTransportError is the
// channels search counterpart of the channels list case in
// channels_list_http_test.go — same root cause
// (https://github.com/natikgadzhi/slack-cli/issues/73): runChannelsSearch's
// hand-rolled conversations.list pagination only preserves already-matched
// channels when the failing page returns a rate-limit error. A plain
// transport error on a later page (timeout, connection reset) discarded
// every match found on earlier pages instead of returning them.
func TestRunChannelsSearch_PartialResultsOnMidPaginationTransportError(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/conversations.list") {
			t.Errorf("unexpected path %q", r.URL.Path)
			return
		}
		hits++
		if hits == 1 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []any{
					map[string]any{"id": "C1", "name": "engineering", "is_private": false},
				},
				"response_metadata": map[string]any{"next_cursor": "page2"},
			})
			return
		}
		// Second page: abort the connection with no response, the same
		// failure shape a client-side timeout produces.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	t.Setenv("SLACK_XOXC", "xoxc-test")
	t.Setenv("SLACK_XOXD", "xoxd-test")
	t.Setenv("SLACK_BASE_URL", srv.URL)

	var err error
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"channels", "search", "eng", "-o", "json"})
		err = rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected the page-1 match to be returned as a partial result, got error: %v", err)
	}
	if !strings.Contains(out, `"engineering"`) {
		t.Fatalf("expected match found before the failing page to be in the output, got:\n%s", out)
	}
}
