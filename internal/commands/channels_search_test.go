package commands

import "testing"

func TestMatchesChannelName(t *testing.T) {
	tests := []struct {
		name  string
		ch    map[string]any
		query string
		want  bool
	}{
		{
			name:  "exact match",
			ch:    map[string]any{"name": "general", "name_normalized": "general"},
			query: "general",
			want:  true,
		},
		{
			name:  "substring match",
			ch:    map[string]any{"name": "eng-infrastructure", "name_normalized": "eng-infrastructure"},
			query: "infra",
			want:  true,
		},
		{
			name:  "case insensitive query",
			ch:    map[string]any{"name": "engineering", "name_normalized": "engineering"},
			query: "ENGINEER",
			want:  true,
		},
		{
			name:  "case insensitive channel name",
			ch:    map[string]any{"name": "Engineering", "name_normalized": "engineering"},
			query: "engineer",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesChannelName(tt.ch, tt.query)
			if got != tt.want {
				t.Errorf("matchesChannelName(%v, %q) = %v, want %v", tt.ch, tt.query, got, tt.want)
			}
		})
	}
}

func TestMatchesChannelName_NoMatch(t *testing.T) {
	ch := map[string]any{"name": "general", "name_normalized": "general"}

	if matchesChannelName(ch, "random") {
		t.Error("expected no match for query 'random' on channel 'general'")
	}

	if matchesChannelName(ch, "xyz") {
		t.Error("expected no match for query 'xyz' on channel 'general'")
	}
}

func TestMatchesChannelName_NormalizedName(t *testing.T) {
	tests := []struct {
		name  string
		ch    map[string]any
		query string
		want  bool
	}{
		{
			name:  "matches name_normalized only",
			ch:    map[string]any{"name": "eng_infra", "name_normalized": "eng-infrastructure"},
			query: "infrastructure",
			want:  true,
		},
		{
			name:  "matches name but not name_normalized",
			ch:    map[string]any{"name": "eng-infrastructure", "name_normalized": "eng_infra"},
			query: "infrastructure",
			want:  true,
		},
		{
			name:  "no name_normalized field",
			ch:    map[string]any{"name": "engineering"},
			query: "engineer",
			want:  true,
		},
		{
			name:  "no name_normalized field and no match",
			ch:    map[string]any{"name": "engineering"},
			query: "product",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesChannelName(tt.ch, tt.query)
			if got != tt.want {
				t.Errorf("matchesChannelName(%v, %q) = %v, want %v", tt.ch, tt.query, got, tt.want)
			}
		})
	}
}

func TestMatchesChannelName_EmptyQuery(t *testing.T) {
	ch := map[string]any{"name": "general", "name_normalized": "general"}

	// An empty query is a substring of everything, so it should match.
	if !matchesChannelName(ch, "") {
		t.Error("expected empty query to match any channel")
	}
}

func TestMatchesChannelName_EmptyChannel(t *testing.T) {
	// Channel with no name fields — should not match a non-empty query.
	ch := map[string]any{"id": "C123"}

	if matchesChannelName(ch, "general") {
		t.Error("expected no match when channel has no name fields")
	}

	// But empty query still matches (empty contains empty).
	if !matchesChannelName(ch, "") {
		t.Error("expected empty query to match even channel with no name")
	}
}
