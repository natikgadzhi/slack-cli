package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// newCachedResolver returns a UserResolver backed by a temp on-disk cache
// pre-seeded with the given uid->name map, so DisplayName resolves offline.
func newCachedResolver(t *testing.T, seed map[string]string) *users.UserResolver {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.json")
	data, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	t.Setenv("SLACK_USER_CACHE", path)

	resolver, err := users.NewUserResolver(api.NewClient("", ""))
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	return resolver
}

func TestSearchChannelLabel_RegularChannel(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{})
	ch := map[string]any{"name": "general"}
	if got := searchChannelLabel(ch, resolver); got != "general" {
		t.Fatalf("got %q, want %q", got, "general")
	}
}

func TestSearchChannelLabel_GroupDM(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{})
	ch := map[string]any{"name": "mpdm-matts--alice--bob-1", "is_mpim": true}
	if got := searchChannelLabel(ch, resolver); got != "mpdm-matts--alice--bob-1" {
		t.Fatalf("got %q, want the mpim name unchanged", got)
	}
}

func TestSearchChannelLabel_DirectMessage(t *testing.T) {
	resolver := newCachedResolver(t, map[string]string{"U123": "Alice Adams"})
	ch := map[string]any{"name": "U123", "user": "U123", "is_im": true}
	if got := searchChannelLabel(ch, resolver); got != "@Alice Adams" {
		t.Fatalf("got %q, want %q", got, "@Alice Adams")
	}
}

func TestSearchChannelLabel_DirectMessageUserFromName(t *testing.T) {
	// When the channel "user" field is absent, the partner ID is taken from
	// the channel name (which Slack sets to the partner's user ID for DMs).
	resolver := newCachedResolver(t, map[string]string{"U999": "Bob Brown"})
	ch := map[string]any{"name": "U999", "is_im": true}
	if got := searchChannelLabel(ch, resolver); got != "@Bob Brown" {
		t.Fatalf("got %q, want %q", got, "@Bob Brown")
	}
}

func TestBuildSearchQuery_QueryOnly(t *testing.T) {
	got := buildSearchQuery("deployment failed", "")
	want := "deployment failed"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment failed", "", got, want)
	}
}

func TestBuildSearchQuery_FromWithAtSign(t *testing.T) {
	got := buildSearchQuery("", "@alice")
	want := "from:alice"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "@alice", got, want)
	}
}

func TestBuildSearchQuery_FromWithoutAtSign(t *testing.T) {
	got := buildSearchQuery("", "alice")
	want := "from:alice"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "alice", got, want)
	}
}

func TestBuildSearchQuery_FromWithUserID(t *testing.T) {
	got := buildSearchQuery("", "U12345ABC")
	want := "from:<U12345ABC>"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "U12345ABC", got, want)
	}
}

func TestBuildSearchQuery_FromAndQuery(t *testing.T) {
	got := buildSearchQuery("deployment", "@alice")
	want := "from:alice deployment"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment", "@alice", got, want)
	}
}

func TestBuildSearchQuery_FromUserIDAndQuery(t *testing.T) {
	got := buildSearchQuery("deployment", "U12345678")
	want := "from:<U12345678> deployment"
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "deployment", "U12345678", got, want)
	}
}

func TestBuildSearchQuery_Empty(t *testing.T) {
	got := buildSearchQuery("", "")
	want := ""
	if got != want {
		t.Errorf("buildSearchQuery(%q, %q) = %q, want %q", "", "", got, want)
	}
}

func TestLooksLikeUserID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"U12345678", true},
		{"U12345ABC", true},
		{"UABC", true},
		{"alice", false},
		{"U", false},         // too short
		{"u12345678", false}, // lowercase u
		{"U123-456", false},  // contains non-alphanumeric
		{"", false},
	}

	for _, tc := range tests {
		got := looksLikeUserID(tc.input)
		if got != tc.want {
			t.Errorf("looksLikeUserID(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestResolveSearchSort_DefaultRelevance(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "query", "")
	if sort != "" || dir != "" {
		t.Errorf("expected empty sort params for relevance, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_ExplicitRecent(t *testing.T) {
	sort, dir := resolveSearchSort("recent", "query", "")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected sort=timestamp dir=desc, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithoutQueryDefaultsToRecent(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "", "alice")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected auto-recent when --from without query, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithQueryKeepsRelevance(t *testing.T) {
	sort, dir := resolveSearchSort("relevance", "deployment", "alice")
	if sort != "" || dir != "" {
		t.Errorf("expected relevance (empty params) when --from with query, got sort=%q dir=%q", sort, dir)
	}
}

func TestResolveSearchSort_FromWithQueryExplicitRecent(t *testing.T) {
	sort, dir := resolveSearchSort("recent", "deployment", "alice")
	if sort != "timestamp" || dir != "desc" {
		t.Errorf("expected sort=timestamp dir=desc, got sort=%q dir=%q", sort, dir)
	}
}

func TestValidateSearchArgs_NoArgsNoFrom(t *testing.T) {
	cmd := *searchCmd // shallow copy to avoid mutating global
	err := validateSearchArgs(&cmd, []string{})
	if err == nil {
		t.Error("expected error when no args and no --from, got nil")
	}
}

func TestValidateSearchArgs_WithQuery(t *testing.T) {
	cmd := *searchCmd
	err := validateSearchArgs(&cmd, []string{"query"})
	if err != nil {
		t.Errorf("unexpected error with query arg: %v", err)
	}
}

func TestValidateSearchArgs_TooManyArgs(t *testing.T) {
	cmd := *searchCmd
	err := validateSearchArgs(&cmd, []string{"arg1", "arg2"})
	if err == nil {
		t.Error("expected error with too many args, got nil")
	}
}

func TestValidateSearchArgs_FromFlagNoArgs(t *testing.T) {
	cmd := *searchCmd
	// Set the --from flag value.
	if err := cmd.Flags().Set("from", "alice"); err != nil {
		t.Fatalf("failed to set --from flag: %v", err)
	}
	err := validateSearchArgs(&cmd, []string{})
	if err != nil {
		t.Errorf("unexpected error with --from and no args: %v", err)
	}
}
