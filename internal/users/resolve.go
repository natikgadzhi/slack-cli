// Package users resolves Slack user IDs to display names, backed by a
// JSON file cache so that repeated lookups don't require API calls.
package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/config"
)

// UserResolver fetches and caches user display names.
type UserResolver struct {
	client    *api.Client
	cachePath string
}

// NewUserResolver creates a UserResolver that uses the given API client
// and stores the name cache at the path returned by config.UserCachePath().
func NewUserResolver(client *api.Client) (*UserResolver, error) {
	cachePath, err := config.UserCachePath()
	if err != nil {
		return nil, fmt.Errorf("user resolver: %w", err)
	}
	return &UserResolver{
		client:    client,
		cachePath: cachePath,
	}, nil
}

// ResolveUsers replaces user IDs in the "user" field of each message
// with display names. Unknown users are fetched via the users.info API
// and added to the on-disk cache. If the API fails for a given user,
// the raw UID is kept.
func (r *UserResolver) ResolveUsers(messages []map[string]any) ([]map[string]any, error) {
	cache, err := r.loadCache()
	if err != nil {
		return nil, err
	}

	// Collect unique unknown user IDs.
	unknown := make(map[string]struct{})
	for _, msg := range messages {
		uid, ok := msg["user"].(string)
		if !ok || uid == "" {
			continue
		}
		if _, cached := cache[uid]; !cached {
			unknown[uid] = struct{}{}
		}
	}

	// Fetch each unknown user and update the cache.
	// If the API fails and returns the raw UID, we still use it for this
	// batch (so the user field gets a value) but we don't persist it in
	// the cache, allowing a retry on the next invocation.
	dirty := false
	failedUIDs := make(map[string]string) // uid -> uid (for current batch only)
	for uid := range unknown {
		name := r.fetchDisplayName(uid)
		if name == uid {
			// Transient failure or unknown user — don't cache.
			failedUIDs[uid] = uid
		} else {
			cache[uid] = name
			dirty = true
		}
	}

	if dirty {
		if err := r.saveCache(cache); err != nil {
			return nil, err
		}
	}

	// Build result with user fields replaced.
	// For failed UIDs that weren't cached, the raw UID is kept as-is.
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := copyMap(msg)
		if uid, ok := m["user"].(string); ok && uid != "" {
			if name, found := cache[uid]; found {
				m["user"] = name
			}
			// failedUIDs map to themselves, so m["user"] stays as the raw UID.
		}
		result = append(result, m)
	}
	return result, nil
}

// loadCache reads the JSON cache file from disk.
// If the file does not exist, an empty cache is returned.
func (r *UserResolver) loadCache() (map[string]string, error) {
	data, err := os.ReadFile(r.cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("reading user cache: %w", err)
	}
	var cache map[string]string
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupt cache — start fresh.
		return make(map[string]string), nil
	}
	return cache, nil
}

// saveCache writes the cache map to the JSON file, creating parent
// directories if needed.
func (r *UserResolver) saveCache(cache map[string]string) error {
	dir := filepath.Dir(r.cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding user cache: %w", err)
	}
	if err := os.WriteFile(r.cachePath, data, 0o600); err != nil {
		return fmt.Errorf("writing user cache: %w", err)
	}
	return nil
}

// fetchDisplayName calls the users.info API for a single user ID
// and returns the best available display name, falling back to the
// raw UID on any error.
func (r *UserResolver) fetchDisplayName(uid string) string {
	data, err := r.client.Call("users.info", map[string]string{"user": uid})
	if err != nil {
		return uid
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		return uid
	}
	// Prefer display_name from the profile (what users see in Slack),
	// then fall back to real_name, then name, then the raw UID.
	if profile, ok := user["profile"].(map[string]any); ok {
		if name, ok := profile["display_name"].(string); ok && name != "" {
			return name
		}
	}
	if name, ok := user["real_name"].(string); ok && name != "" {
		return name
	}
	if name, ok := user["name"].(string); ok && name != "" {
		return name
	}
	return uid
}

// copyMap returns a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
