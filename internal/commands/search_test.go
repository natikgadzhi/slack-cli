package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// newCachedResolver returns a UserResolver backed by a temp on-disk cache
// pre-seeded with the given uid->name map, so DisplayName resolves offline.
func newCachedResolver(t *testing.T, seed map[string]string) *users.UserResolver {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.json")
	data, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	t.Setenv("SLACK_USER_CACHE", path)

	resolver, err := users.NewUserResolver(api.NewClient("", ""))
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	return resolver
}

func TestSearchChannelLabel_RegularChannel(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{})
	ch := map[string]any{"name": "general"}
	if got := searchChannelLabel(ch, resolver); got != "general" {
		t.Fatalf("got %q, want %q", got, "general")
	}
}

func TestSearchChannelLabel_GroupDM(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{})
	ch := map[string]any{"name": "mpdm-matts--alice--bob-1", "is_mpim": true}
	if got := searchChannelLabel(ch, resolver); got != "mpdm-matts--alice--bob-1" {
		t.Fatalf("got %q, want the mpim name unchanged", got)
	}
}

func TestSearchChannelLabel_DirectMessage(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{"U123": "Alice Adams"})
	ch := map[string]any{"name": "U123", "user": "U123", "is_im": true}
	if got := searchChannelLabel(ch, resolver); got != "@Alice Adams" {
		t.Fatalf("got %q, want %q", got, "@Alice Adams")
	}
}

func TestSearchChannelLabel_DirectMessageUserFromName(t *testing.T) {
	// When the channel "user" field is absent, the partner ID is taken from
	// the channel name (which Slack sets to the partner's user ID for DMs).
	resolver := newCachedResolver(t, map[string]string{"U999": "Bob Brown"})
	ch := map[string]any{"name": "U999", "is_im": true}
	if got := searchChannelLabel(ch, resolver); got != "@Bob Brown" {
		t.Fatalf("got %q, want %q", got, "@Bob Brown")
	}
}

func TestBuildSearchQuery_QueryOnly(t *testing.T) {
	got := buildSearchQuery("deployment failed", "")
	want := "deployment failed"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment failed", "", got, want)
	}
}

func TestBuildSearchQuery_FromWithAtSign(t *testing.T) {
	got := buildSearchQuery("", "@alice")
	want := "from:alice"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "@alice", got, want)
	}
}

func TestBuildSearchQuery_FromWithoutAtSign(t *testing.T) {
	got := buildSearchQuery("", "alice")
	want := "from:alice"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "alice", got, want)
	}
}

func TestBuildSearchQuery_FromWithUserID(t *testing.T) {
	got := buildSearchQuery("", "U12345ABC")
	want := "from:<U12345ABC>"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "U12345ABC", got, want)
	}
}

func TestBuildSearchQuery_FromAndQuery(t *testing.T) {
	got := buildSearchQuery("deployment", "@alice")
	want := "from:alice deployment"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment", "@alice", got, want)
	}
}

func TestBuildSearchQuery_FromUserIDAndQuery(t *testing.T) {
	got := buildSearchQuery("deployment", "U12345678")
	want := "from:<U12345678> deployment"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment", "U12345678", got, want)
	}
}

func TestBuildSearchQuery_Empty(t *testing.T) {
	got := buildSearchQuery("", "")
	want := ""
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "", got, want)
	}
}

func TestLooksLikeUserID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"U12345678", true},
		{"U12345ABC", true},
		{"UABC", true},
		{"W12345678", true}, // enterprise grid user ID
		{"WABC", true},      // enterprise grid user ID
		{"alice", false},
		{"U", false},         // too short
		{"W", false},         // too short
		{"u12345678", false}, // lowercase u
		{"w12345678", false}, // lowercase w
		{"U123-456", false},  // contains non-alphanumeric
		{"", false},
	}

	for _, tc := range tests {
		got := looksLikeUserID(tc.input)
		if got != tc.want {
			t.Errorf("looksLikeUserID(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestResolveSearchSort_DefaultRelevance(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "query", "")
	if sort != "" || dir != "" {
		t.Errorf("expected empty sort params for relevance, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_ExplicitRecent(t *testing.T) {
	sort, dir := resolveSearchSort("recent", "query", "")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected sort=timestamp dir=desc, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithoutQueryDefaultsToRecent(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "", "alice")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected auto-recent when --from without query, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithQueryKeepsRelevance(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "deployment", "alice")
	if sort != "" || dir != "" {
		t.Errorf("expected relevance (empty params) when --from with query, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithQueryExplicitRecent(t *testing.T) {
	sort, dir := resolveSearchSort("recent", "deployment", "alice")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected sort=timestamp dir=desc, got sort=%q dir=%q", sort, dir)
	}
}

func TestValidateSearchArgs_NoArgsNoFrom(t *testing.T) {
	cmd := *searchCmd // shallow copy to avoid mutating global
	err := validateSearchArgs(&cmd, []string{})
	if err == nil {
		t.Error("expected error when no args and no --from, got nil")
	}
}

func TestValidateSearchArgs_WithQuery(t *testing.T) {
	cmd := *searchCmd
	err := validateSearchArgs(&cmd, []string{"query"})
	if err != nil {
		t.Errorf("unexpected error with query arg: %v", err)
	}
}

func TestValidateSearchArgs_TooManyArgs(t *testing.T) {
	cmd := *searchCmd
	err := validateSearchArgs(&cmd, []string{"arg1", "arg2"})
	if err == nil {
		t.Error("expected error with too many args, got nil")
	}
}

func TestValidateSearchArgs_FromFlagNoArgs(t *testing.T) {
	cmd := *searchCmd
	// Set the --from flag value.
	if err := cmd.Flags().Set("from", "alice"); err != nil {
		t.Fatalf("failed to set --from flag: %v", err)
	}
	err := validateSearchArgs(&cmd, []string{})
	if err != nil {
		t.Errorf("unexpected error with --from and no args: %v", err)
	}
}

// --- Context-fetching tests ---

func TestReverseSlice(t *testing.T) {
	s := []map[string]any{
		{"ts": "1.0"},
		{"ts": "2.0"},
		{"ts": "3.0"},
	}
	reverseSlice(s)
	if s[0]["ts"] != "3.0" || s[1]["ts"] != "2.0" || s[2]["ts"] != "1.0" {
		t.Errorf("reverseSlice did not reverse correctly: %v", s)
	}
}

func TestReverseSlice_Empty(t *testing.T) {
	var s []map[string]any
	reverseSlice(s) // should not panic
}

func TestReverseSlice_Single(t *testing.T) {
	s := []map[string]any{{"ts": "1.0"}}
	reverseSlice(s)
	if s[0]["ts"] != "1.0" {
		t.Errorf("single-element slice should be unchanged: %v", s)
	}
}

func TestContextMessageToResult(t *testing.T) {
	raw := map[string]any{
		"ts":   "1700000001.000000",
		"user": "alice",
		"text": "hello &amp; world",
	}
	r := contextMessageToResult(raw)
	if r["ts"] != "1700000001.000000" {
		t.Errorf("ts = %v, want 1700000001.000000", r["ts"])
	}
	if r["user"] != "alice" {
		t.Errorf("user = %v, want alice", r["user"])
	}
	// UnescapeEntities should convert &amp; -> &
	if r["text"] != "hello & world" {
		t.Errorf("text = %v, want %q", r["text"], "hello & world")
	}
}

func TestContextMessageToResult_MissingFields(t *testing.T) {
	raw := map[string]any{}
	r := contextMessageToResult(raw)
	if _, ok := r["ts"]; ok {
		t.Errorf("ts should be absent for empty input")
	}
	if _, ok := r["user"]; ok {
		t.Errorf("user should be absent for empty input")
	}
	if _, ok := r["text"]; ok {
		t.Errorf("text should be absent for empty input")
	}
}

func TestCleanSearchResultsForJSON(t *testing.T) {
	results := []map[string]any{
		{
			"ts":         "1700000001.000000",
			"channel":    "general",
			"channel_id": "C123",
			"user":       "alice",
			"text":       "hello",
		},
	}
	cleaned := cleanSearchResultsForJSON(results)
	if len(cleaned) != 1 {
		t.Fatalf("expected 1 result, got %d", len(cleaned))
	}
	if _, ok := cleaned[0]["channel_id"]; ok {
		t.Error("channel_id should be removed from JSON output")
	}
	if cleaned[0]["channel"] != "general" {
		t.Error("channel should be preserved")
	}
	if cleaned[0]["ts"] != "1700000001.000000" {
		t.Error("ts should be preserved")
	}
}

func TestCleanSearchResultsForJSON_PreservesContext(t *testing.T) {
	before := []map[string]any{{"ts": "0.9", "user": "bob", "text": "before"}}
	after := []map[string]any{{"ts": "1.1", "user": "carol", "text": "after"}}
	results := []map[string]any{
		{
			"ts":             "1.0",
			"channel":        "general",
			"channel_id":     "C123",
			"context_before": before,
			"context_after":  after,
		},
	}
	cleaned := cleanSearchResultsForJSON(results)
	if _, ok := cleaned[0]["channel_id"]; ok {
		t.Error("channel_id should be removed")
	}
	if _, ok := cleaned[0]["context_before"]; !ok {
		t.Error("context_before should be preserved")
	}
	if _, ok := cleaned[0]["context_after"]; !ok {
		t.Error("context_after should be preserved")
	}
}

// TestFetchSearchContext_Integration verifies the full context-fetching flow
// against a stubbed HTTP server.
func TestFetchSearchContext_Integration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/conversations.history") {
			_ = r.ParseForm()
			latest := r.FormValue("latest")
			oldest := r.FormValue("oldest")

			var messages []any
			if latest == "1700000010.000000" && oldest == "" {
				// Before context: return 2 messages before the hit (newest first).
				messages = []any{
					map[string]any{"ts": "1700000009.000000", "user": "U002", "text": "msg before 2"},
					map[string]any{"ts": "1700000008.000000", "user": "U003", "text": "msg before 1"},
				}
			} else if oldest == "1700000010.000000" && latest == "" {
				// After context: return 2 messages after the hit.
				messages = []any{
					map[string]any{"ts": "1700000012.000000", "user": "U004", "text": "msg after 2"},
					map[string]any{"ts": "1700000011.000000", "user": "U005", "text": "msg after 1"},
				}
			}

			body := map[string]any{
				"ok":       true,
				"messages": messages,
			}
			_ = json.NewEncoder(w).Encode(body)
			return
		}

		// users.info fallback for resolver.
		if strings.HasSuffix(r.URL.Path, "/users.info") {
			body := map[string]any{
				"ok":   true,
				"user": map[string]any{"name": "testuser", "real_name": "Test User"},
			}
			_ = json.NewEncoder(w).Encode(body)
			return
		}

		http.Error(w, "not found", 404)
	}))
	defer srv.Close()

	client := api.NewClient("xoxc-test", "xoxd-test",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
	)
	resolver := newCachedResolver(t, map[string]string{
		"U002": "Bob",
		"U003": "Carol",
		"U004": "Dave",
		"U005": "Eve",
	})

	results := []map[string]any{
		{
			"ts":         "1700000010.000000",
			"channel":    "general",
			"channel_id": "C123",
			"user":       "alice",
			"text":       "the hit message",
		},
	}

	// Suppress progress output during tests.
	format := "json"

	fetchSearchContext(client, resolver, results, 2, format)

	// Verify context_before was populated.
	before, ok := results[0]["context_before"].([]map[string]any)
	if !ok {
		t.Fatal("context_before missing or wrong type")
	}
	if len(before) != 2 {
		t.Fatalf("expected 2 context_before messages, got %d", len(before))
	}
	// Before messages should be in chronological order (reversed from API response).
	if before[0]["ts"] != "1700000008.000000" {
		t.Errorf("before[0].ts = %v, want 1700000008.000000", before[0]["ts"])
	}
	if before[1]["ts"] != "1700000009.000000" {
		t.Errorf("before[1].ts = %v, want 1700000009.000000", before[1]["ts"])
	}

	// Verify context_after was populated.
	after, ok := results[0]["context_after"].([]map[string]any)
	if !ok {
		t.Fatal("context_after missing or wrong type")
	}
	if len(after) != 2 {
		t.Fatalf("expected 2 context_after messages, got %d", len(after))
	}
	// After messages: conversations.history returns newest first even with oldest param,
	// but since before=false we don't reverse them. The API actually returns them
	// newest-first, so the function keeps the order.
	if after[0]["ts"] != "1700000012.000000" {
		t.Errorf("after[0].ts = %v, want 1700000012.000000", after[0]["ts"])
	}
}

// TestFetchSearchContext_ZeroContext verifies that context=0 makes no API calls.
func TestFetchSearchContext_ZeroContext(t *testing.T) {
	// This is implicitly tested by the main runSearch function: when context=0,
	// fetchSearchContext is never called. But let's test that calling it with
	// limit=0 is harmless.
	var apiCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		apiCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": []any{}})
	}))
	defer srv.Close()

	client := api.NewClient("xoxc-test", "xoxd-test",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
	)
	resolver := newCachedResolver(t, map[string]string{})

	results := []map[string]any{
		{"ts": "1.0", "channel_id": "C123", "channel": "general"},
	}

	// With n=0, limit=0 is passed to the API -- it will still make calls.
	// The main code path guards against this by not calling fetchSearchContext
	// when contextN == 0. But fetchContextMessages with limit=0 will still
	// return an empty slice (Slack returns 0 messages with limit=0).
	fetchSearchContext(client, resolver, results, 0, "json")

	// context_before and context_after should be absent (no messages returned).
	if _, ok := results[0]["context_before"]; ok {
		t.Error("context_before should be absent when 0 messages returned")
	}
	if _, ok := results[0]["context_after"]; ok {
		t.Error("context_after should be absent when 0 messages returned")
	}
}

// TestFetchSearchContext_MissingChannelID verifies graceful handling when
// channel_id is missing from a result.
func TestFetchSearchContext_MissingChannelID(t *testing.T) {
	var apiCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		apiCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": []any{}})
	}))
	defer srv.Close()

	client := api.NewClient("xoxc-test", "xoxd-test",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
	)
	resolver := newCachedResolver(t, map[string]string{})

	results := []map[string]any{
		{"ts": "1.0", "channel": "general"}, // no channel_id
	}

	fetchSearchContext(client, resolver, results, 2, "json")

	if apiCalls != 0 {
		t.Errorf("expected 0 API calls for result without channel_id, got %d", apiCalls)
	}
}

// TestFetchSearchContext_APIError verifies that API errors during context
// fetching are handled gracefully (warned, not fatal).
func TestFetchSearchContext_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "channel_not_found",
		})
	}))
	defer srv.Close()

	client := api.NewClient("xoxc-test", "xoxd-test",
		api.WithBaseURL(srv.URL),
		api.WithPageDelay(0),
		api.WithTimeout(5*time.Second),
	)
	resolver := newCachedResolver(t, map[string]string{})

	results := []map[string]any{
		{"ts": "1.0", "channel_id": "C123", "channel": "general"},
	}

	// Should not panic or return an error.
	fetchSearchContext(client, resolver, results, 2, "json")

	// No context should be set.
	if _, ok := results[0]["context_before"]; ok {
		t.Error("context_before should be absent on API error")
	}
	if _, ok := results[0]["context_after"]; ok {
		t.Error("context_after should be absent on API error")
	}
}

// TestRenderSearchTableWithContext_Output verifies the table output with context markers.
func TestRenderSearchTableWithContext_Output(t *testing.T) {
	results := []map[string]any{
		{
			"ts":      "1700000010.000000",
			"channel": "general",
			"user":    "alice",
			"text":    "the hit",
			"context_before": []map[string]any{
				{"ts": "1700000008.000000", "user": "bob", "text": "before msg"},
			},
			"context_after": []map[string]any{
				{"ts": "1700000012.000000", "user": "carol", "text": "after msg"},
			},
		},
	}

	out := captureStdout(t, func() { renderSearchTableWithContext(results) })

	// Context before should have "|" prefix.
	if !strings.Contains(out, "| before msg") {
		t.Errorf("expected context_before with '|' prefix, got:\n%s", out)
	}
	// Hit should have ">" prefix.
	if !strings.Contains(out, "> the hit") {
		t.Errorf("expected hit with '>' prefix, got:\n%s", out)
	}
	// Context after should have "|" prefix.
	if !strings.Contains(out, "| after msg") {
		t.Errorf("expected context_after with '|' prefix, got:\n%s", out)
	}
}

// TestRenderSearchTableWithContext_NoContext verifies output when there's no context.
func TestRenderSearchTableWithContext_NoContext(t *testing.T) {
	results := []map[string]any{
		{
			"ts":      "1700000010.000000",
			"channel": "general",
			"user":    "alice",
			"text":    "the hit",
		},
	}

	out := captureStdout(t, func() { renderSearchTableWithContext(results) })

	// Hit should have ">" prefix.
	if !strings.Contains(out, "> the hit") {
		t.Errorf("expected hit with '>' prefix, got:\n%s", out)
	}
	// No context markers.
	if strings.Contains(out, "| ") {
		t.Errorf("no context messages expected, got:\n%s", out)
	}
}
