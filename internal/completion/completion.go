// Package completion provides a small TTL-backed cache for shell-completion
// data (channel names, user handles). Completion functions run on every TAB
// press, so they must be fast: this cache lets a cold fetch happen once and be
// reused for a while, and falls back to stale data if a refresh fails.
package completion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/natikgadzhi/slack-cli/internal/config"
)

// entry is the on-disk cache format for a single completion key.
type entry struct {
	FetchedAt time.Time `json:"fetched_at"`
	Values    []string  `json:"values"`
}

// Cached returns completion values for key. If a cache file younger than ttl
// exists, its values are returned without calling fetch. Otherwise fetch is
// invoked; on success the result is cached and returned, and on failure any
// stale cached values are returned as a best-effort fallback.
//
// Cache writes are best-effort: failures to create the directory or write the
// file are ignored, since completion is non-essential.
func Cached(key string, ttl time.Duration, fetch func() ([]string, error)) ([]string, error) {
	path := cachePath(key)

	if e, ok := readEntry(path); ok && time.Since(e.FetchedAt) < ttl {
		return e.Values, nil
	}

	values, err := fetch()
	if err != nil {
		// Refresh failed — fall back to stale data if we have any.
		if e, ok := readEntry(path); ok {
			return e.Values, nil
		}
		return nil, err
	}

	writeEntry(path, entry{FetchedAt: time.Now(), Values: values})
	return values, nil
}

// cachePath returns the on-disk path for a completion key, or "" if the data
// directory can't be resolved (in which case the cache is simply skipped).
func cachePath(key string) string {
	dir, err := config.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "completion", key+".json")
}

// readEntry reads and decodes a cache file. The boolean is false if the path is
// empty, the file is missing, or its contents can't be decoded.
func readEntry(path string) (entry, bool) {
	if path == "" {
		return entry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return entry{}, false
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return entry{}, false
	}
	return e, true
}

// writeEntry encodes and writes a cache file, best-effort.
func writeEntry(path string, e entry) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
