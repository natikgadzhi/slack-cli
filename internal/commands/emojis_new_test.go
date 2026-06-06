package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseEmojis_SortedByName(t *testing.T) {
	raw := map[string]string{
		"zebra":    "https://example.com/zebra.png",
		"apple":    "https://example.com/apple.png",
		"mango":    "https://example.com/mango.png",
		"campfire": "alias:fire",
		"fire":     "https://example.com/fire.png",
	}
	entries := parseEmojis(raw)
	wantOrder := []string{"apple", "campfire", "fire", "mango", "zebra"}
	if len(entries) != len(wantOrder) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantOrder))
	}
	for i, want := range wantOrder {
		if entries[i].Name != want {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, want)
		}
	}
}

func TestParseEmojis_AliasResolvesToTargetURL(t *testing.T) {
	raw := map[string]string{
		"fire":     "https://example.com/fire.png",
		"campfire": "alias:fire",
	}
	entries := parseEmojis(raw)

	byName := map[string]emojiEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	if got := byName["campfire"].Type; got != "alias" {
		t.Errorf("campfire.Type = %q, want alias", got)
	}
	if got := byName["campfire"].AliasTarget; got != "fire" {
		t.Errorf("campfire.AliasTarget = %q, want fire", got)
	}
	if got := byName["campfire"].URL; got != "https://example.com/fire.png" {
		t.Errorf("campfire.URL = %q, want target URL", got)
	}
}

func TestParseEmojis_AliasChainResolves(t *testing.T) {
	raw := map[string]string{
		"fire":  "https://example.com/fire.png",
		"hot":   "alias:fire",
		"flame": "alias:hot",
	}
	entries := parseEmojis(raw)

	for _, e := range entries {
		if e.Name == "flame" && e.URL != "https://example.com/fire.png" {
			t.Errorf("flame.URL = %q, want fire's URL", e.URL)
		}
	}
}

func TestParseEmojis_AliasCycleDoesNotHang(t *testing.T) {
	raw := map[string]string{
		"a": "alias:b",
		"b": "alias:a",
	}
	entries := parseEmojis(raw)
	for _, e := range entries {
		if e.URL != "" {
			t.Errorf("cycle should yield empty URL, got %q for %s", e.URL, e.Name)
		}
	}
}

func TestParseEmojis_AliasMissingTarget(t *testing.T) {
	raw := map[string]string{
		"ghost": "alias:nope",
	}
	entries := parseEmojis(raw)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].URL != "" {
		t.Errorf("missing target should yield empty URL, got %q", entries[0].URL)
	}
	if entries[0].AliasTarget != "nope" {
		t.Errorf("AliasTarget = %q, want nope", entries[0].AliasTarget)
	}
}

func TestSelectEmojis_TypeFilter(t *testing.T) {
	entries := []emojiEntry{
		{Name: "apple", Type: "custom", URL: "https://example.com/apple.png"},
		{Name: "banana", Type: "alias", AliasTarget: "apple", URL: "https://example.com/apple.png"},
		{Name: "cherry", Type: "custom", URL: "https://example.com/cherry.png"},
	}

	custom := selectEmojis(entries, "custom", 100)
	if len(custom) != 2 {
		t.Errorf("custom filter: got %d, want 2", len(custom))
	}

	alias := selectEmojis(entries, "alias", 100)
	if len(alias) != 1 {
		t.Errorf("alias filter: got %d, want 1", len(alias))
	}

	all := selectEmojis(entries, "all", 100)
	if len(all) != 3 {
		t.Errorf("all filter: got %d, want 3", len(all))
	}
}

func TestSelectEmojis_Limit(t *testing.T) {
	entries := []emojiEntry{
		{Name: "a", Type: "custom"},
		{Name: "b", Type: "custom"},
		{Name: "c", Type: "custom"},
		{Name: "d", Type: "custom"},
	}
	got := selectEmojis(entries, "all", 2)
	if len(got) != 2 {
		t.Errorf("got %d, want 2", len(got))
	}
}

func TestBuildSnapshot_PreservesFirstSeen(t *testing.T) {
	then := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prev := &emojiSnapshot{
		UpdatedAt: then,
		Emojis: map[string]emojiSnapshotEntry{
			"old": {FirstSeen: then, Value: "https://example.com/old.png"},
		},
	}
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	raw := map[string]string{
		"old": "https://example.com/old.png",
		"new": "https://example.com/new.png",
	}
	got := buildSnapshot(raw, prev, now, false)
	if got.Emojis["old"].FirstSeen != then {
		t.Errorf("old.FirstSeen = %v, want %v", got.Emojis["old"].FirstSeen, then)
	}
	if got.Emojis["new"].FirstSeen != now {
		t.Errorf("new.FirstSeen = %v, want %v", got.Emojis["new"].FirstSeen, now)
	}
}

func TestBuildSnapshot_ReplaceRebaselines(t *testing.T) {
	then := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prev := &emojiSnapshot{
		Emojis: map[string]emojiSnapshotEntry{
			"old": {FirstSeen: then, Value: "https://example.com/old.png"},
		},
	}
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	raw := map[string]string{"old": "https://example.com/old.png"}
	got := buildSnapshot(raw, prev, now, true)
	if got.Emojis["old"].FirstSeen != now {
		t.Errorf("FirstSeen = %v, want now %v", got.Emojis["old"].FirstSeen, now)
	}
}

func TestPickNewEmojis_DiffMode(t *testing.T) {
	current := []emojiEntry{
		{Name: "a", Type: "custom", URL: "u1"},
		{Name: "b", Type: "custom", URL: "u2"},
		{Name: "c", Type: "custom", URL: "u3"},
	}
	prev := &emojiSnapshot{
		Emojis: map[string]emojiSnapshotEntry{
			"a": {},
		},
	}
	next := &emojiSnapshot{
		Emojis: map[string]emojiSnapshotEntry{
			"a": {FirstSeen: time.Now()},
			"b": {FirstSeen: time.Now()},
			"c": {FirstSeen: time.Now()},
		},
	}
	got := pickNewEmojis(current, next, prev, time.Time{}, 0)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	wantNames := map[string]bool{":b:": true, ":c:": true}
	for _, r := range got {
		if !wantNames[r.Name] {
			t.Errorf("unexpected name %q", r.Name)
		}
	}
}

func TestPickNewEmojis_SinceMode(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-30 * 24 * time.Hour)
	current := []emojiEntry{
		{Name: "fresh", Type: "custom"},
		{Name: "stale", Type: "custom"},
	}
	next := &emojiSnapshot{
		Emojis: map[string]emojiSnapshotEntry{
			"fresh": {FirstSeen: now.Add(-2 * 24 * time.Hour)},
			"stale": {FirstSeen: old},
		},
	}
	cutoff := now.Add(-7 * 24 * time.Hour)
	got := pickNewEmojis(current, next, nil, cutoff, 0)
	if len(got) != 1 || got[0].Name != ":fresh:" {
		t.Errorf("got %v, want only [:fresh:]", got)
	}
}

func TestPickNewEmojis_LimitApplies(t *testing.T) {
	current := []emojiEntry{
		{Name: "a", Type: "custom"},
		{Name: "b", Type: "custom"},
		{Name: "c", Type: "custom"},
	}
	prev := &emojiSnapshot{Emojis: map[string]emojiSnapshotEntry{}}
	next := &emojiSnapshot{
		Emojis: map[string]emojiSnapshotEntry{
			"a": {FirstSeen: time.Now()},
			"b": {FirstSeen: time.Now()},
			"c": {FirstSeen: time.Now()},
		},
	}
	got := pickNewEmojis(current, next, prev, time.Time{}, 2)
	if len(got) != 2 {
		t.Errorf("got %d, want 2", len(got))
	}
}

func TestSaveLoadSnapshot_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	snap := &emojiSnapshot{
		UpdatedAt: now,
		Emojis: map[string]emojiSnapshotEntry{
			"fire": {FirstSeen: now, Value: "https://example.com/fire.png"},
		},
	}
	if err := saveSnapshot(path, snap); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadSnapshot(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("got nil snapshot")
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
	}
	if e, ok := got.Emojis["fire"]; !ok || e.Value != "https://example.com/fire.png" {
		t.Errorf("fire entry not preserved: %+v", e)
	}
}

func TestLoadSnapshot_MissingReturnsNil(t *testing.T) {
	dir := t.TempDir()
	got, err := loadSnapshot(filepath.Join(dir, "nope.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil snapshot for missing file, got %+v", got)
	}
}

func TestSaveSnapshot_AtomicallyOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	a := &emojiSnapshot{UpdatedAt: time.Now(), Emojis: map[string]emojiSnapshotEntry{"a": {Value: "1"}}}
	if err := saveSnapshot(path, a); err != nil {
		t.Fatal(err)
	}
	b := &emojiSnapshot{UpdatedAt: time.Now(), Emojis: map[string]emojiSnapshotEntry{"b": {Value: "2"}}}
	if err := saveSnapshot(path, b); err != nil {
		t.Fatal(err)
	}
	got, err := loadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := got.Emojis["a"]; has {
		t.Errorf("expected old entry replaced, still found 'a'")
	}
	if _, has := got.Emojis["b"]; !has {
		t.Errorf("expected new entry 'b', not found")
	}

	// Confirm no stale tmp files left around.
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", f.Name())
		}
	}
}
