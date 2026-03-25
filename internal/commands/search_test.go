package commands

import (
	"testing"
)

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
