package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// ---------- validateOutputDir tests ----------

func TestValidateDerivedDir_ValidPath(t *testing.T) {
	dir := t.TempDir()
	got, err := validateOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("validateOutputDir = %q, want %q", got, dir)
	}
}

func TestValidateDerivedDir_RelativePath(t *testing.T) {
	// A relative path should be resolved to absolute.
	got, err := validateOutputDir("some/relative/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestValidateDerivedDir_PathTraversal(t *testing.T) {
	_, err := validateOutputDir("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

// ---------- sanitizeTS tests ----------

func TestSanitizeTS_Normal(t *testing.T) {
	got := sanitizeTS("1741234567.123456")
	if got != "1741234567.123456" {
		t.Errorf("sanitizeTS = %q, want %q", got, "1741234567.123456")
	}
}

func TestSanitizeTS_RemovesSlashes(t *testing.T) {
	got := sanitizeTS("../../evil")
	if strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Errorf("sanitizeTS should remove slashes, got %q", got)
	}
}

func TestSanitizeTS_RemovesNulls(t *testing.T) {
	got := sanitizeTS("123\x00456")
	if strings.Contains(got, "\x00") {
		t.Errorf("sanitizeTS should remove null bytes, got %q", got)
	}
}

// ---------- writeItemFiles tests ----------

func TestWriteItemFiles_CreatesCorrectStructure(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{TS: "1741234567.123456", Time: "2025-03-06 12:00 UTC", User: "alice", Text: "hello"},
		{TS: "1741234568.000000", Time: "2025-03-06 12:01 UTC", User: "bob", Text: "world"},
	}

	err := writeItemFiles(dir, items, "C12345678", "general")
	if err != nil {
		t.Fatalf("writeItemFiles: %v", err)
	}

	// Verify directory structure: <dir>/slack/channels/general/
	channelDir := filepath.Join(dir, "slack", "channels", "general")
	info, err := os.Stat(channelDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Verify each message has its own file.
	for _, ts := range []string{"1741234567.123456", "1741234568.000000"} {
		filePath := filepath.Join(channelDir, ts+".md")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("expected file at %s: %v", filePath, err)
		}
	}
}

func TestWriteItemFiles_FileContainsFrontmatter(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{
			TS:   "1741234567.123456",
			Time: "2025-03-06 12:00 UTC",
			User: "alice",
			Text: "hello world",
			Link: "https://team.slack.com/archives/C12345678/p1741234567123456",
		},
	}

	err := writeItemFiles(dir, items, "C12345678", "general")
	if err != nil {
		t.Fatalf("writeItemFiles: %v", err)
	}

	filePath := filepath.Join(dir, "slack", "channels", "general", "1741234567.123456.md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)

	// Verify frontmatter.
	if !strings.HasPrefix(content, "---\n") {
		t.Error("file should start with frontmatter")
	}
	if !strings.Contains(content, "tool: slack-cli") {
		t.Error("missing tool field")
	}
	if !strings.Contains(content, "object_type: channels") {
		t.Error("missing object_type field")
	}
	if !strings.Contains(content, "channel: general") {
		t.Error("missing channel field")
	}
	if !strings.Contains(content, "channel_id: C12345678") {
		t.Error("missing channel_id field")
	}
	if !strings.Contains(content, "user: alice") {
		t.Error("missing user field")
	}
	if !strings.Contains(content, "source_url: https://team.slack.com/archives/C12345678/p1741234567123456") {
		t.Error("missing source_url field")
	}

	// Verify body contains the message text.
	if !strings.Contains(content, "hello world") {
		t.Error("file should contain message text")
	}

	// Verify it's valid frontmatter that can be parsed back.
	meta, body, err := cache.UnmarshalFrontmatter(data)
	if err != nil {
		t.Fatalf("UnmarshalFrontmatter: %v", err)
	}
	if meta.Channel != "general" {
		t.Errorf("meta.Channel = %q, want %q", meta.Channel, "general")
	}
	if meta.ChannelID != "C12345678" {
		t.Errorf("meta.ChannelID = %q, want %q", meta.ChannelID, "C12345678")
	}
	if meta.User != "alice" {
		t.Errorf("meta.User = %q, want %q", meta.User, "alice")
	}
	if len(body) == 0 {
		t.Error("body should not be empty")
	}
}

func TestWriteItemFiles_UsesChannelIDWhenNameEmpty(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{TS: "1741234567.123456", Time: "2025-03-06 12:00 UTC", User: "alice", Text: "hello"},
	}

	err := writeItemFiles(dir, items, "C12345678", "")
	if err != nil {
		t.Fatalf("writeItemFiles: %v", err)
	}

	// Should use channel ID as context when name is empty.
	filePath := filepath.Join(dir, "slack", "channels", "C12345678", "1741234567.123456.md")
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("expected file at %s: %v", filePath, err)
	}
}

func TestWriteItemFiles_SkipsMessagesWithoutTS(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{TS: "", Time: "", User: "alice", Text: "no ts"},
		{TS: "1741234567.123456", Time: "2025-03-06 12:00 UTC", User: "bob", Text: "has ts"},
	}

	err := writeItemFiles(dir, items, "C12345678", "general")
	if err != nil {
		t.Fatalf("writeItemFiles: %v", err)
	}

	// Only the message with a TS should be written.
	channelDir := filepath.Join(dir, "slack", "channels", "general")
	entries, err := os.ReadDir(channelDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

// ---------- writeThreadFile tests ----------

func TestWriteThreadFile_CombinesAllMessages(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{TS: "1741234567.123456", Time: "2025-03-06 12:00 UTC", User: "alice", Text: "thread root"},
		{TS: "1741234568.000000", Time: "2025-03-06 12:01 UTC", User: "bob", Text: "reply 1"},
		{TS: "1741234569.000000", Time: "2025-03-06 12:02 UTC", User: "charlie", Text: "reply 2"},
	}

	err := writeThreadFile(dir, items, "C12345678", "", "1741234567.123456", "https://team.slack.com/archives/C12345678/p1741234567123456")
	if err != nil {
		t.Fatalf("writeThreadFile: %v", err)
	}

	// Thread should be one file named after the thread root TS.
	filePath := filepath.Join(dir, "slack", "messages", "C12345678", "1741234567.123456.md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)

	// All three messages should be in the same file.
	if !strings.Contains(content, "thread root") {
		t.Error("file should contain thread root text")
	}
	if !strings.Contains(content, "reply 1") {
		t.Error("file should contain reply 1")
	}
	if !strings.Contains(content, "reply 2") {
		t.Error("file should contain reply 2")
	}

	// Verify frontmatter has thread_ts.
	meta, _, err := cache.UnmarshalFrontmatter(data)
	if err != nil {
		t.Fatalf("UnmarshalFrontmatter: %v", err)
	}
	if meta.ThreadTS != "1741234567.123456" {
		t.Errorf("meta.ThreadTS = %q, want %q", meta.ThreadTS, "1741234567.123456")
	}
	if meta.User != "alice" {
		t.Errorf("meta.User = %q, want %q (should be thread root author)", meta.User, "alice")
	}
	if meta.ObjectType != "message" {
		t.Errorf("meta.ObjectType = %q, want %q", meta.ObjectType, "message")
	}
}

func TestWriteThreadFile_UsesChannelNameWhenAvailable(t *testing.T) {
	dir := t.TempDir()

	items := []formatting.Message{
		{TS: "1741234567.123456", Time: "2025-03-06 12:00 UTC", User: "alice", Text: "message"},
	}

	err := writeThreadFile(dir, items, "C12345678", "general", "1741234567.123456", "")
	if err != nil {
		t.Fatalf("writeThreadFile: %v", err)
	}

	filePath := filepath.Join(dir, "slack", "messages", "general", "1741234567.123456.md")
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("expected file at %s: %v", filePath, err)
	}
}

// ---------- writeSearchItemFiles tests ----------

func TestWriteSearchItemFiles_CreatesCorrectStructure(t *testing.T) {
	dir := t.TempDir()

	results := []map[string]any{
		{"ts": "1741234567.123456", "channel": "general", "user": "alice", "text": "search hit 1"},
		{"ts": "1741234568.000000", "channel": "random", "user": "bob", "text": "search hit 2"},
	}

	err := writeSearchItemFiles(dir, results, "deployment failed")
	if err != nil {
		t.Fatalf("writeSearchItemFiles: %v", err)
	}

	// Verify directory structure: <dir>/slack/search/<queryHash>/
	queryHash := cache.SearchSlug("deployment failed")
	searchDir := filepath.Join(dir, "slack", "search", queryHash)
	info, err := os.Stat(searchDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Verify each result has its own file.
	for _, ts := range []string{"1741234567.123456", "1741234568.000000"} {
		filePath := filepath.Join(searchDir, ts+".md")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("expected file at %s: %v", filePath, err)
		}
	}
}

func TestWriteSearchItemFiles_FileContainsFrontmatter(t *testing.T) {
	dir := t.TempDir()

	results := []map[string]any{
		{
			"ts":        "1741234567.123456",
			"channel":   "general",
			"user":      "alice",
			"text":      "found this message",
			"permalink": "https://team.slack.com/archives/C12345678/p1741234567123456",
		},
	}

	err := writeSearchItemFiles(dir, results, "test query")
	if err != nil {
		t.Fatalf("writeSearchItemFiles: %v", err)
	}

	queryHash := cache.SearchSlug("test query")
	filePath := filepath.Join(dir, "slack", "search", queryHash, "1741234567.123456.md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "object_type: search") {
		t.Error("missing object_type field")
	}
	if !strings.Contains(content, "channel: general") {
		t.Error("missing channel field")
	}
	if !strings.Contains(content, "user: alice") {
		t.Error("missing user field")
	}
	if !strings.Contains(content, "found this message") {
		t.Error("file should contain message text")
	}
}

func TestWriteSearchItemFiles_SkipsResultsWithoutTS(t *testing.T) {
	dir := t.TempDir()

	results := []map[string]any{
		{"channel": "general", "user": "alice", "text": "no ts"},
		{"ts": "1741234567.123456", "channel": "random", "user": "bob", "text": "has ts"},
	}

	err := writeSearchItemFiles(dir, results, "query")
	if err != nil {
		t.Fatalf("writeSearchItemFiles: %v", err)
	}

	queryHash := cache.SearchSlug("query")
	searchDir := filepath.Join(dir, "slack", "search", queryHash)
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

// ---------- DerivedDir does not affect stdout or cache ----------

func TestDerivedDir_DoesNotAffectCacheWrite(t *testing.T) {
	// Verify that cacheWrite still works independently when DerivedDir is set.
	origDerivedDir := DerivedDir
	defer func() { DerivedDir = origDerivedDir }()

	DerivedDir = t.TempDir()

	// cacheWrite to nil cache should not panic.
	cacheWrite(nil, "test", "slug", map[string]string{"key": "value"}, cache.Metadata{})
}

// ---------- Frontmatter round-trip with new fields ----------

func TestFrontmatterRoundtrip_NewFields(t *testing.T) {
	meta := cache.Metadata{
		Tool:       "slack-cli",
		ObjectType: "message",
		Slug:       "general/1741234567.123456",
		Channel:    "general",
		ChannelID:  "C12345678",
		User:       "alice",
		ThreadTS:   "1741234567.123456",
	}

	body := []byte("test body\n")
	data := cache.MarshalFrontmatter(meta, body)

	gotMeta, gotBody, err := cache.UnmarshalFrontmatter(data)
	if err != nil {
		t.Fatalf("UnmarshalFrontmatter: %v", err)
	}
	if gotMeta.Channel != "general" {
		t.Errorf("Channel = %q, want %q", gotMeta.Channel, "general")
	}
	if gotMeta.ChannelID != "C12345678" {
		t.Errorf("ChannelID = %q, want %q", gotMeta.ChannelID, "C12345678")
	}
	if gotMeta.User != "alice" {
		t.Errorf("User = %q, want %q", gotMeta.User, "alice")
	}
	if gotMeta.ThreadTS != "1741234567.123456" {
		t.Errorf("ThreadTS = %q, want %q", gotMeta.ThreadTS, "1741234567.123456")
	}
	if string(gotBody) != string(body) {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
}

func TestFrontmatterRoundtrip_NewFieldsOmittedWhenEmpty(t *testing.T) {
	meta := cache.Metadata{
		Tool:       "slack-cli",
		ObjectType: "search",
		Slug:       "abc123/1741234567.123456",
	}

	data := cache.MarshalFrontmatter(meta, []byte("body\n"))
	content := string(data)

	if strings.Contains(content, "channel:") {
		t.Error("channel should be omitted when empty")
	}
	if strings.Contains(content, "channel_id:") {
		t.Error("channel_id should be omitted when empty")
	}
	if strings.Contains(content, "user:") {
		t.Error("user should be omitted when empty")
	}
	if strings.Contains(content, "thread_ts:") {
		t.Error("thread_ts should be omitted when empty")
	}
}
