package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// newTestResolver creates a UserResolver backed by a temp directory and
// an httptest server that responds to users.info requests using the
// supplied user map (uid -> display name). If a UID is not in the map,
// the handler returns {"ok": false}.
func newTestResolver(t *testing.T, users map[string]string) (*UserResolver, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		uid := r.FormValue("user")
		name, ok := users[uid]
		if !ok {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "user_not_found",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"user": map[string]any{
				"real_name": name,
				"name":      uid,
			},
		})
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient("xoxc-test", "xoxd-test", api.WithBaseURL(srv.URL))

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "users.json")

	resolver := &UserResolver{
		client:    client,
		cachePath: cachePath,
	}

	return resolver, srv
}

func TestResolveUsers_AllCached(t *testing.T) {
	// Pre-populate cache; expect zero API calls.
	resolver, srv := newTestResolver(t, nil)
	srv.Close() // close the server — any API call should fail

	// Write cache directly.
	cache := map[string]string{
		"U111": "Alice",
		"U222": "Bob",
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(resolver.cachePath, data, 0o644)

	messages := []map[string]any{
		{"user": "U111", "text": "hello"},
		{"user": "U222", "text": "world"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0]["user"] != "Alice" {
		t.Errorf("expected Alice, got %v", result[0]["user"])
	}
	if result[1]["user"] != "Bob" {
		t.Errorf("expected Bob, got %v", result[1]["user"])
	}
}

func TestResolveUsers_UnknownUser_FetchAndCache(t *testing.T) {
	apiUsers := map[string]string{
		"U333": "Charlie",
	}
	resolver, _ := newTestResolver(t, apiUsers)

	messages := []map[string]any{
		{"user": "U333", "text": "hi"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0]["user"] != "Charlie" {
		t.Errorf("expected Charlie, got %v", result[0]["user"])
	}

	// Verify cache was written.
	data, err := os.ReadFile(resolver.cachePath)
	if err != nil {
		t.Fatalf("cache file not created: %v", err)
	}
	var cached map[string]string
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("invalid cache JSON: %v", err)
	}
	if cached["U333"] != "Charlie" {
		t.Errorf("cache should contain Charlie for U333, got %v", cached["U333"])
	}
}

func TestResolveUsers_CacheFileCreated(t *testing.T) {
	// Cache file does not exist; it should be created after resolving.
	apiUsers := map[string]string{
		"U444": "Diana",
	}
	resolver, _ := newTestResolver(t, apiUsers)

	// Verify cache file does not exist yet.
	if _, err := os.Stat(resolver.cachePath); !os.IsNotExist(err) {
		t.Fatal("cache file should not exist before first resolve")
	}

	messages := []map[string]any{
		{"user": "U444", "text": "test"},
	}

	_, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now the cache file should exist.
	if _, err := os.Stat(resolver.cachePath); err != nil {
		t.Fatalf("cache file should exist after resolve: %v", err)
	}
}

func TestResolveUsers_APIFailure_FallsBackToUID(t *testing.T) {
	// The server knows about U555 but not U666.
	apiUsers := map[string]string{
		"U555": "Eve",
	}
	resolver, _ := newTestResolver(t, apiUsers)

	messages := []map[string]any{
		{"user": "U555", "text": "known"},
		{"user": "U666", "text": "unknown"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0]["user"] != "Eve" {
		t.Errorf("expected Eve, got %v", result[0]["user"])
	}
	// U666 should fall back to its raw UID.
	if result[1]["user"] != "U666" {
		t.Errorf("expected U666 (fallback), got %v", result[1]["user"])
	}
}

func TestResolveUsers_MessagesWithoutUserField(t *testing.T) {
	resolver, _ := newTestResolver(t, nil)

	messages := []map[string]any{
		{"text": "system message"},
		{"subtype": "channel_join"},
		{"user": "", "text": "empty user"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	// Messages should pass through unchanged.
	if result[0]["text"] != "system message" {
		t.Errorf("expected system message unchanged, got %v", result[0]["text"])
	}
	if _, hasUser := result[0]["user"]; hasUser {
		t.Errorf("message without user field should not gain one")
	}
}

func TestResolveUsers_DoesNotMutateInput(t *testing.T) {
	apiUsers := map[string]string{
		"U777": "Grace",
	}
	resolver, _ := newTestResolver(t, apiUsers)

	original := []map[string]any{
		{"user": "U777", "text": "hi"},
	}

	result, err := resolver.ResolveUsers(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The original message should still have the UID.
	if original[0]["user"] != "U777" {
		t.Errorf("original message was mutated: user=%v", original[0]["user"])
	}
	if result[0]["user"] != "Grace" {
		t.Errorf("resolved message should have Grace, got %v", result[0]["user"])
	}
}
