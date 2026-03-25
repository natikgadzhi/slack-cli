package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
)

// newTestClient creates a Client pointing at the given test server URL
// with zero-duration page delays and an injected no-op sleep.
func newTestClient(serverURL string, opts ...Option) *Client {
	allOpts := []Option{
		WithBaseURL(serverURL),
		WithPageDelay(0),
		WithTimeout(5 * time.Second),
	}
	allOpts = append(allOpts, opts...)
	c := NewClient("xoxc-test-token", "xoxd-test-cookie", allOpts...)
	c.sleepFn = func(time.Duration) {} // don't actually sleep in tests
	return c
}

func TestCall_CorrectHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify headers
		if got := r.Header.Get("Authorization"); got != "Bearer xoxc-test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "d=xoxd-test-cookie" {
			t.Errorf("Cookie = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("User-Agent header is empty")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	result, err := client.Call("auth.test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
}

func TestCall_ParamsEncoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		if got := r.PostFormValue("channel"); got != "C12345" {
			t.Errorf("channel = %q, want C12345", got)
		}
		if got := r.PostFormValue("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("conversations.history", map[string]string{
		"channel": "C12345",
		"limit":   "100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCall_NilParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		if n != 0 {
			t.Errorf("expected empty body for nil params, got %d bytes: %q", n, string(body[:n]))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCall_429TriggersRetry(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "attempt": n})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	result, err := client.Call("conversations.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
}

func TestCall_429ExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, WithMaxRetries(2))
	_, err := client.Call("conversations.list", nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestCall_NonOKHTTPReturnsCLIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}

	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != 500 {
		t.Errorf("expected code 500, got %d", cliErr.Code)
	}
}

func TestCallPaginated_CollectsPages(t *testing.T) {
	page := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		resp := map[string]any{
			"ok":       true,
			"messages": []any{map[string]any{"text": fmt.Sprintf("msg-page-%d", page)}},
		}
		if page == 1 {
			resp["response_metadata"] = map[string]any{"next_cursor": "cursor-2"}
		}
		// page 2: no next_cursor, so pagination ends.
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	items, err := client.CallPaginated("conversations.history", nil, "next_cursor", "messages")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["text"] != "msg-page-1" {
		t.Errorf("first item text = %v", items[0]["text"])
	}
	if items[1]["text"] != "msg-page-2" {
		t.Errorf("second item text = %v", items[1]["text"])
	}
}

func TestCallPaginated_429ReturnsPartialResults(t *testing.T) {
	page := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			resp := map[string]any{
				"ok":                true,
				"messages":          []any{map[string]any{"text": "first-page"}},
				"response_metadata": map[string]any{"next_cursor": "cursor-2"},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Second page: always 429 to exhaust retries.
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, WithMaxRetries(1))
	items, err := client.CallPaginated("conversations.history", nil, "next_cursor", "messages")

	// Should get partial data.
	if len(items) != 1 {
		t.Fatalf("expected 1 partial item, got %d", len(items))
	}
	if items[0]["text"] != "first-page" {
		t.Errorf("partial item text = %v", items[0]["text"])
	}

	// Should also get a RateLimitError.
	if err == nil {
		t.Fatal("expected error on 429 during pagination")
	}
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
	// PartialData on the error should match the returned items.
	if len(rlErr.PartialData) != 1 {
		t.Errorf("expected 1 partial item in error, got %d", len(rlErr.PartialData))
	}
}

func TestGetTeamURL_EnvOverride(t *testing.T) {
	t.Setenv("SLACK_TEAM_URL", "https://myteam.slack.com/")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call API when env var is set")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": "https://wrong.slack.com/"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	url, err := client.GetTeamURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Trailing slash should be stripped.
	if url != "https://myteam.slack.com" {
		t.Errorf("GetTeamURL = %q, want https://myteam.slack.com", url)
	}
}

func TestGetTeamURL_APIFallback(t *testing.T) {
	// Make sure env var is NOT set.
	t.Setenv("SLACK_TEAM_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":  true,
			"url": "https://apiteam.slack.com/",
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	url, err := client.GetTeamURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://apiteam.slack.com" {
		t.Errorf("GetTeamURL = %q, want https://apiteam.slack.com", url)
	}
}

func TestGetTeamURL_Cached(t *testing.T) {
	t.Setenv("SLACK_TEAM_URL", "")

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": "https://cached.slack.com/"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)

	// Call twice.
	_, _ = client.GetTeamURL()
	url, err := client.GetTeamURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://cached.slack.com" {
		t.Errorf("GetTeamURL = %q", url)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d (caching broken)", calls.Load())
	}
}

func TestBackoffDelay_WithRetryAfter(t *testing.T) {
	c := NewClient("x", "x")
	d := c.backoffDelay(5*time.Second, true, 0)
	// Should be between 5s and 5s + 25% jitter = 6.25s.
	if d < 5*time.Second || d > 6250*time.Millisecond {
		t.Errorf("backoff delay = %v, expected [5s, 6.25s]", d)
	}
}

func TestBackoffDelay_Exponential(t *testing.T) {
	c := NewClient("x", "x")
	// attempt=0 → base=1s, jitter up to 0.5s → [1s, 1.5s]
	d := c.backoffDelay(0, false, 0)
	if d < 0 || d > 2*time.Second {
		t.Errorf("exponential backoff attempt 0 = %v, expected [0, 2s]", d)
	}
	// attempt=3 → base=8s, jitter up to 4s → [8s, 12s]
	d = c.backoffDelay(0, false, 3)
	if d < 8*time.Second || d > 12*time.Second {
		t.Errorf("exponential backoff attempt 3 = %v, expected [8s, 12s]", d)
	}
}

func TestCall_OkFalseReturnsCLIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "invalid_auth",
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err == nil {
		t.Fatal("expected error for ok:false response")
	}

	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != 200 {
		t.Errorf("expected code 200 (Slack returns 200 with ok:false), got %d", cliErr.Code)
	}
	if !strings.Contains(cliErr.Message, "invalid_auth") {
		t.Errorf("expected message containing 'invalid_auth', got %q", cliErr.Message)
	}
}

func TestCall_OkFalseUnknownError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"ok": false,
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err == nil {
		t.Fatal("expected error for ok:false response without error field")
	}

	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T: %v", err, err)
	}
	if !strings.Contains(cliErr.Message, "unknown error") {
		t.Errorf("expected message containing 'unknown error', got %q", cliErr.Message)
	}
}

func TestCall_429WithoutRetryAfterUsesExponentialBackoff(t *testing.T) {
	var attempts atomic.Int32
	var delays []time.Duration
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			// No Retry-After header — should trigger exponential backoff.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	client.sleepFn = func(d time.Duration) {
		mu.Lock()
		delays = append(delays, d)
		mu.Unlock()
	}

	result, err := client.Call("conversations.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}

	mu.Lock()
	defer mu.Unlock()
	// With no Retry-After header, exponential backoff should be used.
	// attempt 0: base=1s, attempt 1: base=2s.
	if len(delays) != 2 {
		t.Fatalf("expected 2 delays, got %d", len(delays))
	}
	// First delay (attempt 0): exponential base=1s + jitter up to 0.5s → [1s, 1.5s]
	if delays[0] < 1*time.Second || delays[0] > 2*time.Second {
		t.Errorf("first delay = %v, expected [1s, 2s]", delays[0])
	}
	// Second delay (attempt 1): exponential base=2s + jitter up to 1s → [2s, 3s]
	if delays[1] < 2*time.Second || delays[1] > 3*time.Second {
		t.Errorf("second delay = %v, expected [2s, 3s]", delays[1])
	}
}

func TestGetTeamURL_MissingURLField(t *testing.T) {
	t.Setenv("SLACK_TEAM_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// auth.test succeeds but with no url field.
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	url, err := client.GetTeamURL()
	if err == nil {
		t.Fatal("expected error when url field is missing")
	}
	if url != "" {
		t.Errorf("expected empty url, got %q", url)
	}
	if err.Error() != "auth.test response missing url field" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCall_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not json"))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !containsAny(err.Error(), "decoding") {
		t.Errorf("expected JSON decode error, got: %v", err)
	}
}

func TestCall_NonOKHTTPReturnsCLIErrorWithCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(repeatStr("x", 500)))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.Call("auth.test", nil)
	if err == nil {
		t.Fatal("expected error for HTTP 502")
	}
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != 502 {
		t.Errorf("expected code 502, got %d", cliErr.Code)
	}
}

func TestCallPaginated_DoesNotMutateCallerParams(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		resp := map[string]any{
			"ok":    true,
			"items": []any{map[string]any{"id": fmt.Sprintf("item-%d", page)}},
		}
		if page == 1 {
			resp["response_metadata"] = map[string]any{"next_cursor": "page-2"}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	params := map[string]string{"limit": "10"}
	_, err := client.CallPaginated("items.list", params, "next_cursor", "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Caller's params should not have been mutated with a "cursor" key.
	if _, hasCursor := params["cursor"]; hasCursor {
		t.Error("CallPaginated should not mutate caller's params map")
	}
}

func TestCallPaginated_EmptyCollectKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	items, err := client.CallPaginated("conversations.history", nil, "next_cursor", "messages")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items when collectKey is missing from response, got %d", len(items))
	}
}

func TestGetTeamURL_APIError(t *testing.T) {
	t.Setenv("SLACK_TEAM_URL", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "not_authed"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetTeamURL()
	if err == nil {
		t.Fatal("expected error when auth.test returns ok:false")
	}
}

func TestGetTeamURL_CachedConcurrent(t *testing.T) {
	t.Setenv("SLACK_TEAM_URL", "")
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": "https://concurrent.slack.com/"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)

	// Call GetTeamURL concurrently from multiple goroutines.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.GetTeamURL()
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("expected exactly 1 API call (sync.Once), got %d", calls.Load())
	}
}

func TestParseRetryAfter_ZeroValue(t *testing.T) {
	c := NewClient("x", "x")
	d, ok := c.parseRetryAfter("0")
	if ok {
		t.Error("parseRetryAfter(\"0\") should return false")
	}
	if d != 0 {
		t.Errorf("expected 0 duration, got %v", d)
	}
}

func TestParseRetryAfter_NegativeValue(t *testing.T) {
	c := NewClient("x", "x")
	_, ok := c.parseRetryAfter("-5")
	if ok {
		t.Error("parseRetryAfter(\"-5\") should return false")
	}
}

func TestParseRetryAfter_NonNumeric(t *testing.T) {
	c := NewClient("x", "x")
	_, ok := c.parseRetryAfter("abc")
	if ok {
		t.Error("parseRetryAfter(\"abc\") should return false")
	}
}

func TestParseRetryAfter_ValidValue(t *testing.T) {
	c := NewClient("x", "x")
	d, ok := c.parseRetryAfter("3")
	if !ok {
		t.Error("parseRetryAfter(\"3\") should return true")
	}
	if d != 3*time.Second {
		t.Errorf("expected 3s, got %v", d)
	}
}

func TestAsAPIError_Nil(t *testing.T) {
	_, ok := AsAPIError(nil)
	if ok {
		t.Error("AsAPIError(nil) should return false")
	}
}

func TestAsAPIError_NonAPIError(t *testing.T) {
	_, ok := AsAPIError(fmt.Errorf("some other error"))
	if ok {
		t.Error("AsAPIError on non-APIError should return false")
	}
}

func TestAsAPIError_Match(t *testing.T) {
	err := &APIError{Code: 401, Message: "not_authed"}
	got, ok := AsAPIError(err)
	if !ok {
		t.Fatal("AsAPIError should return true for *APIError")
	}
	if got.Code != 401 {
		t.Errorf("Code = %d, want 401", got.Code)
	}
}

func TestAsAPIError_Wrapped(t *testing.T) {
	inner := &APIError{Code: 403, Message: "forbidden"}
	err := fmt.Errorf("outer: %w", inner)
	got, ok := AsAPIError(err)
	if !ok {
		t.Fatal("AsAPIError should unwrap wrapped errors")
	}
	if got.Code != 403 {
		t.Errorf("Code = %d, want 403", got.Code)
	}
}

func TestAPIError_ErrorString(t *testing.T) {
	err := &APIError{Code: 500, Message: "internal"}
	s := err.Error()
	if s != "slack api error (HTTP 500): internal" {
		t.Errorf("unexpected error string: %q", s)
	}
}

func TestRateLimitError_ErrorString(t *testing.T) {
	err := &RateLimitError{RetryAfter: 30 * time.Second}
	s := err.Error()
	if s != "rate limited: retry after 30s" {
		t.Errorf("unexpected error string: %q", s)
	}
}

func TestExtractItems_MissingKey(t *testing.T) {
	result := map[string]any{"ok": true}
	items := ExtractItems(result, "messages")
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestExtractItems_WrongType(t *testing.T) {
	result := map[string]any{"messages": "not-an-array"}
	items := ExtractItems(result, "messages")
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestExtractItems_NonMapElements(t *testing.T) {
	result := map[string]any{
		"items": []any{"string-element", 42, map[string]any{"id": "valid"}},
	}
	items := ExtractItems(result, "items")
	if len(items) != 1 {
		t.Errorf("expected 1 valid map item, got %d", len(items))
	}
}

func TestExtractNextCursor_NoResponseMetadata(t *testing.T) {
	result := map[string]any{"ok": true}
	cursor := ExtractNextCursor(result, "next_cursor")
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}

func TestExtractNextCursor_ResponseMetadataNotMap(t *testing.T) {
	result := map[string]any{"response_metadata": "not-a-map"}
	cursor := ExtractNextCursor(result, "next_cursor")
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}

func TestExtractNextCursor_EmptyCursorString(t *testing.T) {
	result := map[string]any{
		"response_metadata": map[string]any{"next_cursor": ""},
	}
	cursor := ExtractNextCursor(result, "next_cursor")
	if cursor != "" {
		t.Errorf("expected empty cursor for empty string, got %q", cursor)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("xoxc-token", "xoxd-cookie")
	if c.xoxc != "xoxc-token" {
		t.Errorf("xoxc = %q", c.xoxc)
	}
	if c.xoxd != "xoxd-cookie" {
		t.Errorf("xoxd = %q", c.xoxd)
	}
	if c.maxRetries != defaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", c.maxRetries, defaultMaxRetries)
	}
	if c.pageDelay != defaultPageDelay {
		t.Errorf("pageDelay = %v, want %v", c.pageDelay, defaultPageDelay)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	c := NewClient("x", "x", WithMaxRetries(10), WithPageDelay(500*time.Millisecond))
	if c.maxRetries != 10 {
		t.Errorf("maxRetries = %d, want 10", c.maxRetries)
	}
	if c.pageDelay != 500*time.Millisecond {
		t.Errorf("pageDelay = %v, want 500ms", c.pageDelay)
	}
}

// helper functions for tests
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func repeatStr(s string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}

func TestExtractNextCursor_UsesCustomKey(t *testing.T) {
	result := map[string]any{
		"response_metadata": map[string]any{
			"next_cursor":   "standard-cursor",
			"custom_cursor": "custom-cursor-value",
		},
	}

	// Should use the specified cursorKey.
	cursor := ExtractNextCursor(result, "next_cursor")
	if cursor != "standard-cursor" {
		t.Errorf("ExtractNextCursor(next_cursor) = %q, want 'standard-cursor'", cursor)
	}

	cursor = ExtractNextCursor(result, "custom_cursor")
	if cursor != "custom-cursor-value" {
		t.Errorf("ExtractNextCursor(custom_cursor) = %q, want 'custom-cursor-value'", cursor)
	}

	// Missing key returns empty.
	cursor = ExtractNextCursor(result, "nonexistent")
	if cursor != "" {
		t.Errorf("ExtractNextCursor(nonexistent) = %q, want empty", cursor)
	}
}
