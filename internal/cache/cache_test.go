package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixed time for deterministic tests.
var testTime = time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)

func testMeta() Metadata {
	return Metadata{
		Tool:       "slack-cli",
		ObjectType: "message",
		Slug:       "C12345678/1741234567.123456",
		CreatedAt:  testTime,
		UpdatedAt:  testTime,
		SourceURL:  "https://myteam.slack.com/archives/C12345678/p1741234567123456",
		Command:    "slack-cli message https://myteam.slack.com/archives/C12345678/p1741234567123456",
	}
}

// ---------- Frontmatter tests ----------

func TestMarshalFrontmatter(t *testing.T) {
	meta := testMeta()
	body := []byte("Hello, world!\n")

	data := MarshalFrontmatter(meta, body)
	s := string(data)

	if !strings.HasPrefix(s, "---\n") {
		t.Fatal("expected opening ---")
	}
	if !strings.Contains(s, "tool: slack-cli\n") {
		t.Error("missing tool field")
	}
	if !strings.Contains(s, "object_type: message\n") {
		t.Error("missing object_type field")
	}
	if !strings.Contains(s, "slug: C12345678/1741234567.123456\n") {
		t.Error("missing slug field")
	}
	if !strings.Contains(s, "created_at: 2026-03-14T10:00:00Z\n") {
		t.Error("missing created_at field")
	}
	if !strings.Contains(s, "updated_at: 2026-03-14T10:00:00Z\n") {
		t.Error("missing updated_at field")
	}
	if !strings.Contains(s, "source_url: https://myteam.slack.com/archives/C12345678/p1741234567123456\n") {
		t.Error("missing source_url field")
	}
	if !strings.Contains(s, `command: "slack-cli message https://myteam.slack.com/archives/C12345678/p1741234567123456"`) {
		t.Error("missing command field")
	}
	if !strings.Contains(s, "Hello, world!\n") {
		t.Error("missing body")
	}
}

func TestMarshalFrontmatterEmptyBody(t *testing.T) {
	meta := testMeta()
	data := MarshalFrontmatter(meta, nil)
	s := string(data)

	// Should still have two --- blocks and not crash.
	if strings.Count(s, "---\n") != 2 {
		t.Errorf("expected exactly 2 --- separators, got content:\n%s", s)
	}
}

func TestMarshalFrontmatterOptionalFieldsOmitted(t *testing.T) {
	meta := Metadata{
		Tool:       "slack-cli",
		ObjectType: "search_result",
		Slug:       "abcdef123456",
		CreatedAt:  testTime,
		UpdatedAt:  testTime,
	}
	data := MarshalFrontmatter(meta, []byte("results"))
	s := string(data)

	if strings.Contains(s, "source_url") {
		t.Error("source_url should be omitted when empty")
	}
	if strings.Contains(s, "command") {
		t.Error("command should be omitted when empty")
	}
}

func TestUnmarshalFrontmatter(t *testing.T) {
	meta := testMeta()
	body := []byte("Hello, world!\n")

	data := MarshalFrontmatter(meta, body)
	got, gotBody, err := UnmarshalFrontmatter(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Tool != meta.Tool {
		t.Errorf("Tool: got %q, want %q", got.Tool, meta.Tool)
	}
	if got.ObjectType != meta.ObjectType {
		t.Errorf("ObjectType: got %q, want %q", got.ObjectType, meta.ObjectType)
	}
	if got.Slug != meta.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, meta.Slug)
	}
	if !got.CreatedAt.Equal(meta.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, meta.CreatedAt)
	}
	if !got.UpdatedAt.Equal(meta.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, meta.UpdatedAt)
	}
	if got.SourceURL != meta.SourceURL {
		t.Errorf("SourceURL: got %q, want %q", got.SourceURL, meta.SourceURL)
	}
	if got.Command != meta.Command {
		t.Errorf("Command: got %q, want %q", got.Command, meta.Command)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body: got %q, want %q", gotBody, body)
	}
}

func TestUnmarshalFrontmatterMissingSeparator(t *testing.T) {
	_, _, err := UnmarshalFrontmatter([]byte("no frontmatter here"))
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestUnmarshalFrontmatterMissingClosing(t *testing.T) {
	_, _, err := UnmarshalFrontmatter([]byte("---\ntool: slack-cli\nbody without closing"))
	if err == nil {
		t.Fatal("expected error for missing closing separator")
	}
}

// ---------- Roundtrip marshal/unmarshal ----------

func TestFrontmatterRoundtrip(t *testing.T) {
	bodies := [][]byte{
		[]byte("Simple body\n"),
		[]byte("Line 1\nLine 2\nLine 3\n"),
		[]byte("# Heading\n\n- bullet\n- bullet 2\n"),
		nil,
	}

	for i, body := range bodies {
		meta := testMeta()
		data := MarshalFrontmatter(meta, body)
		gotMeta, gotBody, err := UnmarshalFrontmatter(data)
		if err != nil {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
		if gotMeta.Tool != meta.Tool {
			t.Errorf("case %d: Tool mismatch", i)
		}
		if body == nil {
			if len(gotBody) != 0 {
				t.Errorf("case %d: expected empty body, got %q", i, gotBody)
			}
		} else if string(gotBody) != string(body) {
			t.Errorf("case %d: body mismatch: got %q, want %q", i, gotBody, body)
		}
	}
}

// ---------- Cache tests ----------

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	c, err := NewCacheWithDir(dir)
	if err != nil {
		t.Fatalf("NewCacheWithDir: %v", err)
	}
	return c
}

func TestPutAndGet(t *testing.T) {
	c := newTestCache(t)

	meta := testMeta()
	body := []byte("Hello from cache\n")

	if err := c.Put("messages", "C12345678/1741234567.123456", body, meta); err != nil {
		t.Fatalf("Put: %v", err)
	}

	gotBody, gotMeta, found, err := c.Get("messages", "C12345678/1741234567.123456")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if string(gotBody) != string(body) {
		t.Errorf("body: got %q, want %q", gotBody, body)
	}
	if gotMeta.Tool != "slack-cli" {
		t.Errorf("Tool: got %q, want %q", gotMeta.Tool, "slack-cli")
	}
	if gotMeta.SourceURL != meta.SourceURL {
		t.Errorf("SourceURL: got %q, want %q", gotMeta.SourceURL, meta.SourceURL)
	}
}

func TestGetMissing(t *testing.T) {
	c := newTestCache(t)

	_, _, found, err := c.Get("messages", "C99999/0000000000.000000")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("expected found=false for missing file")
	}
}

func TestCacheAutoCreatesDirectories(t *testing.T) {
	c := newTestCache(t)

	body := []byte("channel history\n")
	meta := Metadata{
		ObjectType: "channel_history",
		Slug:       "C12345678/2026-03-01_2026-03-10",
	}

	if err := c.Put("channels", "C12345678/2026-03-01_2026-03-10", body, meta); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Verify the directory was created.
	dirPath := filepath.Join(c.baseDir, "channels", "C12345678")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Verify we can read it back.
	_, _, found, err := c.Get("channels", "C12345678/2026-03-01_2026-03-10")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
}

func TestDifferentObjectTypesStoredInCorrectSubdirs(t *testing.T) {
	c := newTestCache(t)

	// Store a message.
	if err := c.Put("messages", "C111/ts1", []byte("msg"), Metadata{}); err != nil {
		t.Fatalf("Put message: %v", err)
	}

	// Store a channel history.
	if err := c.Put("channels", "C222/2026-01-01_2026-01-31", []byte("history"), Metadata{}); err != nil {
		t.Fatalf("Put channel: %v", err)
	}

	// Store a search result.
	if err := c.Put("search", "abcdef123456", []byte("results"), Metadata{}); err != nil {
		t.Fatalf("Put search: %v", err)
	}

	// Verify each lives in the right subdirectory.
	cases := []struct {
		relPath string
	}{
		{"messages/C111/ts1.md"},
		{"channels/C222/2026-01-01_2026-01-31.md"},
		{"search/abcdef123456.md"},
	}
	for _, tc := range cases {
		full := filepath.Join(c.baseDir, tc.relPath)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected file at %s: %v", tc.relPath, err)
		}
	}
}

func TestPutSetsDefaultsWhenEmpty(t *testing.T) {
	c := newTestCache(t)

	meta := Metadata{}
	if err := c.Put("messages", "C999/ts1", []byte("body"), meta); err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, gotMeta, found, err := c.Get("messages", "C999/ts1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if gotMeta.Tool != "slack-cli" {
		t.Errorf("default Tool: got %q, want %q", gotMeta.Tool, "slack-cli")
	}
	if gotMeta.ObjectType != "messages" {
		t.Errorf("default ObjectType: got %q, want %q", gotMeta.ObjectType, "messages")
	}
	if gotMeta.Slug != "C999/ts1" {
		t.Errorf("default Slug: got %q, want %q", gotMeta.Slug, "C999/ts1")
	}
	if gotMeta.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if gotMeta.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestPutOverwritesExisting(t *testing.T) {
	c := newTestCache(t)

	meta := testMeta()
	if err := c.Put("messages", "C111/ts1", []byte("version 1"), meta); err != nil {
		t.Fatalf("Put v1: %v", err)
	}

	meta.UpdatedAt = testTime.Add(time.Hour)
	if err := c.Put("messages", "C111/ts1", []byte("version 2"), meta); err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	body, gotMeta, found, err := c.Get("messages", "C111/ts1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if string(body) != "version 2\n" {
		t.Errorf("body: got %q, want %q", body, "version 2\n")
	}
	if !gotMeta.UpdatedAt.Equal(testTime.Add(time.Hour)) {
		t.Errorf("UpdatedAt should reflect v2")
	}
}

// ---------- Slug helper tests ----------

func TestMessageSlug(t *testing.T) {
	got := MessageSlug("C12345678", "1741234567.123456")
	want := "C12345678/1741234567.123456"
	if got != want {
		t.Errorf("MessageSlug: got %q, want %q", got, want)
	}
}

func TestChannelHistorySlug(t *testing.T) {
	got := ChannelHistorySlug("C12345678", "2026-03-01", "2026-03-10")
	want := "C12345678/2026-03-01_2026-03-10"
	if got != want {
		t.Errorf("ChannelHistorySlug: got %q, want %q", got, want)
	}
}

func TestSearchSlug(t *testing.T) {
	slug := SearchSlug("deployment failed")
	if len(slug) != 12 {
		t.Errorf("SearchSlug length: got %d, want 12", len(slug))
	}

	// Same query should produce the same slug.
	slug2 := SearchSlug("deployment failed")
	if slug != slug2 {
		t.Errorf("SearchSlug not deterministic: %q != %q", slug, slug2)
	}

	// Different query should produce a different slug.
	slug3 := SearchSlug("something else")
	if slug == slug3 {
		t.Error("SearchSlug: different queries should produce different slugs")
	}
}
