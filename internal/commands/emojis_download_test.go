package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestSyncAlias_WritesSidecarAndManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{}}
	stats := &emojiDownloadStats{}
	e := emojiAdminEntry{
		Name:          "campfire",
		Type:          "alias",
		AliasFor:      "fire",
		CreatedByName: "Alice",
		Created:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}

	syncAlias(e, dir, manifest, false, stats)

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
	entry, ok := manifest.Emojis["campfire"]
	if !ok {
		t.Fatal("manifest entry missing")
	}
	if entry.AliasFor != "fire" || entry.Type != "alias" || entry.CreatedByName != "Alice" {
		t.Errorf("manifest entry wrong: %+v", entry)
	}
}

func TestSyncAlias_EmptyTargetCountsAsError(t *testing.T) {
	dir := t.TempDir()
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{}}
	stats := &emojiDownloadStats{}
	e := emojiAdminEntry{Name: "broken", Type: "alias", AliasFor: ""}

	syncAlias(e, dir, manifest, false, stats)

	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
	if _, ok := manifest.Emojis["broken"]; ok {
		t.Errorf("broken alias should not appear in manifest")
	}
}

func TestSyncAlias_SkipsWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing sidecar and manifest entry with the same target.
	if err := os.WriteFile(filepath.Join(dir, "campfire.alias"), []byte("fire\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{
		"campfire": {Type: "alias", AliasFor: "fire", LocalPath: "campfire.alias"},
	}}
	stats := &emojiDownloadStats{}
	e := emojiAdminEntry{Name: "campfire", Type: "alias", AliasFor: "fire"}

	syncAlias(e, dir, manifest, false, stats)

	if stats.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", stats.Skipped)
	}
	if stats.Aliases != 0 {
		t.Errorf("Aliases = %d, want 0 (no rewrite)", stats.Aliases)
	}
}

func TestSyncAlias_RewritesWhenTargetChanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "campfire.alias"), []byte("fire\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{
		"campfire": {Type: "alias", AliasFor: "fire", LocalPath: "campfire.alias"},
	}}
	stats := &emojiDownloadStats{}
	// Same name, but now pointing at a different emoji.
	e := emojiAdminEntry{Name: "campfire", Type: "alias", AliasFor: "flame"}

	syncAlias(e, dir, manifest, false, stats)

	if stats.Aliases != 1 {
		t.Errorf("Aliases = %d, want 1 (rewrite)", stats.Aliases)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "campfire.alias"))
	if string(data) != "flame\n" {
		t.Errorf("sidecar = %q, want %q", string(data), "flame\n")
	}
}

func TestPruneManifest_RemovesAbsentEntries(t *testing.T) {
	dir := t.TempDir()
	// Put a stale image on disk for the about-to-be-pruned entry.
	stalePath := filepath.Join(dir, "obsolete.png")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{
		"fire":     {Type: "custom", LocalPath: "fire.png"},
		"obsolete": {Type: "custom", LocalPath: "obsolete.png"},
	}}
	current := []emojiAdminEntry{{Name: "fire", Type: "custom"}}

	removed := pruneManifest(manifest, current, dir)

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if _, ok := manifest.Emojis["obsolete"]; ok {
		t.Errorf("obsolete still in manifest")
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("stale image not removed: %v", err)
	}
	if _, ok := manifest.Emojis["fire"]; !ok {
		t.Errorf("fire (still current) was pruned")
	}
}

func TestPruneManifest_MissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	manifest := &emojiManifest{Emojis: map[string]emojiManifestEntry{
		"ghost": {Type: "custom", LocalPath: "ghost.png"}, // file never written
	}}

	removed := pruneManifest(manifest, nil, dir)

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
}

func TestSaveLoadManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	m := &emojiManifest{
		Version:   emojiManifestVersion,
		UpdatedAt: now,
		Emojis: map[string]emojiManifestEntry{
			"fire": {
				Type:          "custom",
				URL:           "https://example.com/fire.png",
				LocalPath:     "fire.png",
				CreatedAt:     now,
				CreatedByID:   "U1",
				CreatedByName: "Alice",
			},
		},
	}
	if err := saveManifest(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := loadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Version != emojiManifestVersion {
		t.Fatalf("version = %v, want %d", got, emojiManifestVersion)
	}
	if e := got.Emojis["fire"]; e.CreatedByName != "Alice" || e.URL != "https://example.com/fire.png" {
		t.Errorf("fire entry not preserved: %+v", e)
	}
}

func TestLoadManifest_MissingReturnsNil(t *testing.T) {
	dir := t.TempDir()
	got, err := loadManifest(filepath.Join(dir, "nope.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil manifest for missing file, got %+v", got)
	}
}

func TestManifestEntryFor_PopulatesAllFields(t *testing.T) {
	e := emojiAdminEntry{
		Name:          "fire",
		Type:          "custom",
		URL:           "https://example.com/fire.png",
		Created:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		CreatedByID:   "U1",
		CreatedByName: "Alice",
	}
	got := manifestEntryFor(e, "fire.png")
	if got.LocalPath != "fire.png" || got.URL != e.URL || got.CreatedByName != "Alice" {
		t.Errorf("entry incomplete: %+v", got)
	}
}

func TestParseAdminListPage_RealShape(t *testing.T) {
	// JSON shaped like the real emoji.adminList response.
	raw := []byte(`{
		"ok": true,
		"emoji": [
			{
				"name": "fire",
				"is_alias": 0,
				"alias_for": "",
				"url": "https://emoji.slack-edge.com/T1/fire/abc.png",
				"user_id": "U123",
				"user_display_name": "Alice",
				"created": 1592604167
			},
			{
				"name": "campfire",
				"is_alias": 1,
				"alias_for": "fire",
				"url": "https://emoji.slack-edge.com/T1/fire/abc.png",
				"user_id": "U456",
				"user_display_name": "Bob",
				"created": 1700000000
			}
		],
		"paging": {"count": 200, "total": 2, "page": 1, "pages": 1}
	}`)
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}

	entries := parseAdminListPage(result)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	byName := map[string]emojiAdminEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	fire := byName["fire"]
	if fire.Type != "custom" || fire.CreatedByName != "Alice" {
		t.Errorf("fire: %+v", fire)
	}
	if fire.Created.Unix() != 1592604167 {
		t.Errorf("fire.Created = %v, want unix 1592604167", fire.Created)
	}
	campfire := byName["campfire"]
	if campfire.Type != "alias" || campfire.AliasFor != "fire" {
		t.Errorf("campfire: %+v", campfire)
	}

	pages, ok := pagingPages(result)
	if !ok || pages != 1 {
		t.Errorf("pages = %d, ok = %v, want 1, true", pages, ok)
	}
}
