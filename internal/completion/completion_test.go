package completion

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCache writes a completion cache file for key under the data dir rooted
// at dir, with the given age and values.
func writeCache(t *testing.T, dir, key string, age time.Duration, values []string) {
	t.Helper()
	path := filepath.Join(dir, "completion", key+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(entry{FetchedAt: time.Now().Add(-age), Values: values})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestCachedFreshSkipsFetch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)
	writeCache(t, dir, "channels", time.Minute, []string{"general", "random"})

	called := false
	got, err := Cached("channels", time.Hour, func() ([]string, error) {
		called = true
		return []string{"should-not-be-used"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("fetch was called despite fresh cache")
	}
	if len(got) != 2 || got[0] != "general" || got[1] != "random" {
		t.Fatalf("got %v, want [general random]", got)
	}
}

func TestCachedMissWritesCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)

	got, err := Cached("users", time.Hour, func() ([]string, error) {
		return []string{"alice", "bob"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 values", got)
	}

	// A second call should now be served from the freshly written cache.
	got2, err := Cached("users", time.Hour, func() ([]string, error) {
		return nil, errors.New("fetch should not run")
	})
	if err != nil {
		t.Fatalf("unexpected error on cached read: %v", err)
	}
	if len(got2) != 2 || got2[0] != "alice" {
		t.Fatalf("got %v, want cached [alice bob]", got2)
	}
}

func TestCachedStaleFallsBackOnFetchError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)
	writeCache(t, dir, "channels", 2*time.Hour, []string{"stale-chan"})

	got, err := Cached("channels", time.Hour, func() ([]string, error) {
		return nil, errors.New("network down")
	})
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if len(got) != 1 || got[0] != "stale-chan" {
		t.Fatalf("got %v, want stale [stale-chan]", got)
	}
}

func TestCachedNoCacheReturnsFetchError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)

	wantErr := errors.New("boom")
	_, err := Cached("channels", time.Hour, func() ([]string, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}
