package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunChannelsList_PartialResultsOnMidPaginationTransportError reproduces
// https://github.com/natikgadzhi/slack-cli/issues/73: a conversations.list
// page beyond the first fails with a plain transport error (the same shape
// net/http produces for "context deadline exceeded" / "request canceled").
// That error is neither a *api.RateLimitError nor a *clierrors.CLIError, so
// it used to fall through to a bare `return fmt.Errorf(...)` that discarded
// every channel already fetched from the successful first page. The command
// should instead behave like its own rate-limit branch a few lines above:
// warn and return the channels already collected.
func TestRunChannelsList_PartialResultsOnMidPaginationTransportError(t *testing.T) {
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
					map[string]any{"id": "C1", "name": "general", "is_private": false},
				},
				"response_metadata": map[string]any{"next_cursor": "page2"},
			})
			return
		}
		// Second page: abort the connection with no response, the same
		// failure shape a client-side timeout produces (httpClient.Do
		// returns a plain, non-Slack, non-429 error).
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
		rootCmd.SetArgs([]string{"channels", "list", "-o", "json"})
		err = rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected the page-1 channel to be returned as a partial result, got error: %v", err)
	}
	if !strings.Contains(out, `"general"`) {
		t.Fatalf("expected channel fetched before the failing page to be in the output, got:\n%s", out)
	}
}
