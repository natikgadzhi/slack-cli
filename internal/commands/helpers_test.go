package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/cache"
)

func TestFormatMessages_Empty(t *testing.T) {
	result := formatMessages(nil, "https://team.slack.com", "C123", true)
	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}

func TestFormatMessages_WithPermalinks(t *testing.T) {
	messages := []map[string]any{
		{"ts": "1741234567.123456", "user": "U123", "text": "hello"},
		{"ts": "1741234568.000000", "user": "U456", "text": "world"},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// Both should have permalink links.
	if result[0].Link == "" {
		t.Error("first message should have a permalink")
	}
	if result[1].Link == "" {
		t.Error("second message should have a permalink")
	}
	// Verify link format.
	expected := "https://team.slack.com/archives/C12345678/p1741234567123456"
	if result[0].Link != expected {
		t.Errorf("Link = %q, want %q", result[0].Link, expected)
	}
}

func TestFormatMessages_NoTeamURL(t *testing.T) {
	messages := []map[string]any{
		{"ts": "1741234567.123456", "user": "U123", "text": "hello"},
	}

	result := formatMessages(messages, "", "C12345678", false)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// Should NOT have a permalink when hasTeamURL is false.
	if result[0].Link != "" {
		t.Errorf("Link should be empty when hasTeamURL is false, got %q", result[0].Link)
	}
}

func TestFormatMessages_MessageWithoutTS(t *testing.T) {
	messages := []map[string]any{
		{"user": "U123", "text": "no timestamp"},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// No ts means no permalink should be generated.
	if result[0].Link != "" {
		t.Errorf("Link should be empty when message has no ts, got %q", result[0].Link)
	}
}

func TestFormatMessages_PreservesFormattedFields(t *testing.T) {
	messages := []map[string]any{
		{
			"ts":          "1741234567.123456",
			"user":        "U123",
			"text":        "test message",
			"reply_count": float64(5),
			"reactions": []any{
				map[string]any{"name": "thumbsup", "count": float64(2)},
			},
		},
	}

	result := formatMessages(messages, "https://team.slack.com", "C12345678", true)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	msg := result[0]
	if msg.Text != "test message" {
		t.Errorf("Text = %q, want %q", msg.Text, "test message")
	}
	if msg.User != "U123" {
		t.Errorf("User = %q, want %q", msg.User, "U123")
	}
	if msg.ReplyCount != 5 {
		t.Errorf("ReplyCount = %d, want 5", msg.ReplyCount)
	}
	if len(msg.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(msg.Reactions))
	}
}

func TestGetCache_NoCacheFlag(t *testing.T) {
	orig := NoCache
	defer func() { NoCache = orig }()

	NoCache = true
	c := getCache()
	if c != nil {
		t.Error("getCache should return nil when NoCache is true")
	}
}

func TestCacheWrite_NilCache(t *testing.T) {
	// Should not panic on nil cache.
	cacheWrite(nil, "test", "slug", map[string]string{"key": "value"}, cache.Metadata{})
}

func TestTruncate_Short(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate = %q, want %q", got, "hello")
	}
}

func TestTruncate_Long(t *testing.T) {
	got := truncate("hello world this is a long string", 10)
	if got != "hello w..." {
		t.Errorf("truncate = %q, want %q", got, "hello w...")
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := truncate("1234567890", 10)
	if got != "1234567890" {
		t.Errorf("truncate = %q, want %q", got, "1234567890")
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"name": "alice",
		"age":  float64(30),
	}

	if got := getString(m, "name"); got != "alice" {
		t.Errorf("getString(m, name) = %q, want %q", got, "alice")
	}
	if got := getString(m, "age"); got != "" {
		t.Errorf("getString(m, age) = %q, want empty (not a string)", got)
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString(m, missing) = %q, want empty", got)
	}
}

func TestGetBool(t *testing.T) {
	m := map[string]any{
		"is_admin": true,
		"is_owner": false,
		"name":     "alice", // not a bool
	}

	if got := getBool(m, "is_admin"); got != true {
		t.Errorf("getBool(m, is_admin) = %v, want true", got)
	}
	if got := getBool(m, "is_owner"); got != false {
		t.Errorf("getBool(m, is_owner) = %v, want false", got)
	}
	if got := getBool(m, "name"); got != false {
		t.Errorf("getBool(m, name) = %v, want false for non-bool", got)
	}
	if got := getBool(m, "missing"); got != false {
		t.Errorf("getBool(m, missing) = %v, want false for missing key", got)
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
		ok    bool
	}{
		{float64(42), 42, true},
		{float64(0), 0, true},
		{float64(1.9), 1, true},
		{42, 42, true},
		{0, 0, true},
		{"42", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tc := range tests {
		got, ok := toInt(tc.input)
		if ok != tc.ok {
			t.Errorf("toInt(%v) ok = %v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if got != tc.want {
			t.Errorf("toInt(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{float64(42.5), 42.5, true},
		{float64(0), 0, true},
		{42, 42, true},
		{0, 0, true},
		{"42.5", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tc := range tests {
		got, ok := toFloat(tc.input)
		if ok != tc.ok {
			t.Errorf("toFloat(%v) ok = %v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if got != tc.want {
			t.Errorf("toFloat(%v) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestExtractStringSlice(t *testing.T) {
	result := map[string]any{
		"members": []any{"U12345", "U67890", "UABCDE"},
	}

	got := extractStringSlice(result, "members")
	want := []string{"U12345", "U67890", "UABCDE"}

	if len(got) != len(want) {
		t.Fatalf("extractStringSlice() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("extractStringSlice()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestExtractStringSlice_MissingKey(t *testing.T) {
	result := map[string]any{
		"other": []any{"foo"},
	}

	got := extractStringSlice(result, "members")
	if got != nil {
		t.Errorf("extractStringSlice() with missing key = %v, want nil", got)
	}
}

func TestExtractStringSlice_NotArray(t *testing.T) {
	result := map[string]any{
		"members": "not an array",
	}

	got := extractStringSlice(result, "members")
	if got != nil {
		t.Errorf("extractStringSlice() with non-array = %v, want nil", got)
	}
}

func TestExtractStringSlice_EmptyArray(t *testing.T) {
	result := map[string]any{
		"members": []any{},
	}

	got := extractStringSlice(result, "members")
	if len(got) != 0 {
		t.Errorf("extractStringSlice() with empty array = %v, want empty", got)
	}
}

func TestExtractStringSlice_MixedTypes(t *testing.T) {
	// Non-string elements should be silently skipped.
	result := map[string]any{
		"members": []any{"U12345", float64(42), "U67890", nil, "UABCDE"},
	}

	got := extractStringSlice(result, "members")
	want := []string{"U12345", "U67890", "UABCDE"}

	if len(got) != len(want) {
		t.Fatalf("extractStringSlice() with mixed types returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("extractStringSlice()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestExactlyOneArg_ZeroArgs(t *testing.T) {
	validator := exactlyOneArg(
		"a channel name or ID",
		"slack-cli channels get <name|id> [flags]",
		"slack-cli channels get general --since 2d",
		"slack-cli channels get C12345678",
	)

	// Simulate a command with CommandPath "slack-cli channels get".
	cmd := &cobra.Command{Use: "get"}
	parent := &cobra.Command{Use: "channels"}
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(parent)
	parent.AddCommand(cmd)

	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for zero args, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "requires a channel name or ID") {
		t.Errorf("error should mention what's required, got: %s", msg)
	}
	if !strings.Contains(msg, "Usage:") {
		t.Errorf("error should include usage, got: %s", msg)
	}
	if !strings.Contains(msg, "Examples:") {
		t.Errorf("error should include examples, got: %s", msg)
	}
	if !strings.Contains(msg, "slack-cli channels get general --since 2d") {
		t.Errorf("error should include the example, got: %s", msg)
	}
}

func TestExactlyOneArg_OneArg(t *testing.T) {
	validator := exactlyOneArg("a query", "slack-cli search <query>")

	cmd := &cobra.Command{Use: "search"}
	err := validator(cmd, []string{"hello"})
	if err != nil {
		t.Errorf("expected no error for one arg, got: %v", err)
	}
}

func TestExactlyOneArg_TooManyArgs(t *testing.T) {
	validator := exactlyOneArg("a query", "slack-cli search <query>")

	cmd := &cobra.Command{Use: "search"}
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(cmd)

	err := validator(cmd, []string{"hello", "world"})
	if err == nil {
		t.Fatal("expected error for two args, got nil")
	}
	if !strings.Contains(err.Error(), "accepts 1 argument, got 2") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestExactlyOneArg_NoExamples(t *testing.T) {
	validator := exactlyOneArg("a thing", "slack-cli do <thing>")

	cmd := &cobra.Command{Use: "do"}
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(cmd)

	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for zero args, got nil")
	}

	msg := err.Error()
	if strings.Contains(msg, "Examples:") {
		t.Errorf("should not include Examples section when none provided, got: %s", msg)
	}
}
