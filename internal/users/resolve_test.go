package users

import (
	"encoding/json"
	"errors"
	"io/fs"
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
	if _, err := os.Stat(resolver.cachePath); !errors.Is(err, fs.ErrNotExist) {
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

func TestResolveUsers_FailedUID_NotCached(t *testing.T) {
	// The server does not know about U999, so the API returns ok:false
	// and fetchDisplayName returns the raw UID. The raw UID should NOT
	// be persisted to the cache, so a future call can retry.
	apiUsers := map[string]string{
		"U888": "Helen",
	}
	resolver, _ := newTestResolver(t, apiUsers)

	messages := []map[string]any{
		{"user": "U888", "text": "known"},
		{"user": "U999", "text": "unknown"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0]["user"] != "Helen" {
		t.Errorf("expected Helen, got %v", result[0]["user"])
	}
	if result[1]["user"] != "U999" {
		t.Errorf("expected U999 (fallback), got %v", result[1]["user"])
	}

	// Verify cache file exists and contains U888 but NOT U999.
	data, err := os.ReadFile(resolver.cachePath)
	if err != nil {
		t.Fatalf("cache file not created: %v", err)
	}
	var cached map[string]string
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("invalid cache JSON: %v", err)
	}
	if cached["U888"] != "Helen" {
		t.Errorf("cache should contain Helen for U888, got %v", cached["U888"])
	}
	if _, found := cached["U999"]; found {
		t.Errorf("cache should NOT contain U999 (failed lookup), but found %q", cached["U999"])
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

func TestResolveUsers_PrefersDisplayName(t *testing.T) {
	// When the API returns a profile with display_name, it should be
	// preferred over real_name and name.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		uid := r.FormValue("user")
		var resp map[string]any
		switch uid {
		case "U_DN":
			// Has display_name, real_name, and name.
			resp = map[string]any{
				"ok": true,
				"user": map[string]any{
					"name":      "u_dn",
					"real_name": "Display Real",
					"profile": map[string]any{
						"display_name": "Preferred Display",
					},
				},
			}
		case "U_RN":
			// Has real_name and name, but empty display_name.
			resp = map[string]any{
				"ok": true,
				"user": map[string]any{
					"name":      "u_rn",
					"real_name": "Real Name Only",
					"profile": map[string]any{
						"display_name": "",
					},
				},
			}
		default:
			resp = map[string]any{"ok": false, "error": "user_not_found"}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient("xoxc-test", "xoxd-test", api.WithBaseURL(srv.URL))
	resolver := &UserResolver{
		client:    client,
		cachePath: filepath.Join(t.TempDir(), "users.json"),
	}

	messages := []map[string]any{
		{"user": "U_DN", "text": "has display_name"},
		{"user": "U_RN", "text": "empty display_name"},
	}

	result, err := resolver.ResolveUsers(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0]["user"] != "Preferred Display" {
		t.Errorf("expected 'Preferred Display', got %v", result[0]["user"])
	}
	if result[1]["user"] != "Real Name Only" {
		t.Errorf("expected 'Real Name Only', got %v", result[1]["user"])
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
