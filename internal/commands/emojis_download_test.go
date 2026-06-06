package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmojiFileExt(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://emoji.slack-edge.com/T123/fire/abc.png", ".png"},
		{"https://emoji.slack-edge.com/T123/party/abc.gif", ".gif"},
		{"https://emoji.slack-edge.com/T123/x/abc.JPG", ".jpg"},
		{"https://emoji.slack-edge.com/T123/x/abc.jpeg", ".jpeg"},
		{"https://emoji.slack-edge.com/T123/x/abc.webp", ".webp"},
		{"https://emoji.slack-edge.com/T123/x/abc.apng", ".apng"},
		{"https://emoji.slack-edge.com/T123/x/no-extension", ".png"},
		{"https://emoji.slack-edge.com/T123/x/abc.unknown", ".png"},
		{"not a url", ".png"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := emojiFileExt(tt.url); got != tt.want {
				t.Errorf("emojiFileExt(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestWriteAliasSidecar_WritesTargetFile(t *testing.T) {
	dir := t.TempDir()
	stats := &emojiDownloadStats{}
	e := emojiEntry{Name: "campfire", Type: "alias", AliasTarget: "fire"}

	writeAliasSidecar(e, dir, stats)

	if stats.Aliases != 1 {
		t.Errorf("Aliases = %d, want 1", stats.Aliases)
	}
	data, err := os.ReadFile(filepath.Join(dir, "campfire.alias"))
	if err != nil {
		t.Fatalf("reading sidecar: %v", err)
	}
	if string(data) != "fire\n" {
		t.Errorf("sidecar = %q, want %q", string(data), "fire\n")
	}
}

func TestWriteAliasSidecar_EmptyTargetCountsAsError(t *testing.T) {
	dir := t.TempDir()
	stats := &emojiDownloadStats{}
	e := emojiEntry{Name: "broken", Type: "alias", AliasTarget: ""}

	writeAliasSidecar(e, dir, stats)

	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
	if stats.Aliases != 0 {
		t.Errorf("Aliases = %d, want 0", stats.Aliases)
	}
}

func TestWriteAliasSidecar_SanitizesNameInPath(t *testing.T) {
	dir := t.TempDir()
	stats := &emojiDownloadStats{}
	e := emojiEntry{Name: "../escape", Type: "alias", AliasTarget: "fire"}

	writeAliasSidecar(e, dir, stats)

	// File must land inside dir, not outside via path traversal.
	want := filepath.Join(dir, "_.._escape.alias")
	if _, err := os.Stat(want); err != nil {
		// sanitizeFilename only strips /, not "..". The leading ".." stays.
		// Confirm at minimum the sidecar didn't land outside dir.
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Fatalf("expected exactly 1 file in dir, got %d", len(entries))
		}
		name := entries[0].Name()
		if filepath.IsAbs(name) || filepath.Dir(filepath.Join(dir, name)) != filepath.Clean(dir) {
			t.Errorf("sidecar escaped dir: %q", name)
		}
	}
}
