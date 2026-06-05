package commands

import (
	"sort"
	"testing"
)

func TestExtractEmojiMap(t *testing.T) {
	tests := []struct {
		name   string
		result map[string]any
		want   map[string]string
	}{
		{
			name: "normal emoji map",
			result: map[string]any{
				"ok": true,
				"emoji": map[string]any{
					"fire":       "https://emoji.slack-edge.com/T123/fire/abc123.png",
					"party_blob": "alias:blob-party",
				},
			},
			want: map[string]string{
				"fire":       "https://emoji.slack-edge.com/T123/fire/abc123.png",
				"party_blob": "alias:blob-party",
			},
		},
		{
			name:   "missing emoji key",
			result: map[string]any{"ok": true},
			want:   nil,
		},
		{
			name:   "emoji key is not a map",
			result: map[string]any{"ok": true, "emoji": "not-a-map"},
			want:   nil,
		},
		{
			name:   "empty emoji map",
			result: map[string]any{"ok": true, "emoji": map[string]any{}},
			want:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEmojiMap(tt.result)
			if tt.want == nil {
				if got != nil {
					t.Errorf("extractEmojiMap() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("extractEmojiMap() returned %d entries, want %d", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("extractEmojiMap()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestFilterEmojis_SubstringMatch(t *testing.T) {
	emojis := map[string]string{
		"fire":       "https://emoji.slack-edge.com/T123/fire/abc.png",
		"on_fire":    "https://emoji.slack-edge.com/T123/on_fire/def.png",
		"campfire":   "alias:fire",
		"party_blob": "https://emoji.slack-edge.com/T123/party_blob/ghi.png",
	}

	matched := filterEmojis(emojis, "fire", 50)

	// Should match fire, on_fire, campfire but not party_blob.
	if len(matched) != 3 {
		t.Fatalf("filterEmojis(\"fire\") returned %d results, want 3", len(matched))
	}

	// Collect names for deterministic checking (map iteration order is random).
	names := make([]string, len(matched))
	for i, r := range matched {
		names[i] = r.Name
	}
	sort.Strings(names)

	wantNames := []string{":campfire:", ":fire:", ":on_fire:"}
	for i, want := range wantNames {
		if names[i] != want {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestFilterEmojis_CaseInsensitive(t *testing.T) {
	emojis := map[string]string{
		"ThumbsUp": "https://example.com/thumbsup.png",
		"TADA":     "https://example.com/tada.png",
	}

	matched := filterEmojis(emojis, "thumbsup", 50)
	if len(matched) != 1 {
		t.Fatalf("filterEmojis(\"thumbsup\") returned %d results, want 1", len(matched))
	}
	if matched[0].Name != ":ThumbsUp:" {
		t.Errorf("matched name = %q, want %q", matched[0].Name, ":ThumbsUp:")
	}
}

func TestFilterEmojis_Limit(t *testing.T) {
	emojis := map[string]string{
		"test1": "https://example.com/1.png",
		"test2": "https://example.com/2.png",
		"test3": "https://example.com/3.png",
		"test4": "https://example.com/4.png",
		"test5": "https://example.com/5.png",
	}

	matched := filterEmojis(emojis, "test", 3)
	if len(matched) != 3 {
		t.Errorf("filterEmojis with limit=3 returned %d results, want 3", len(matched))
	}
}

func TestFilterEmojis_NoMatch(t *testing.T) {
	emojis := map[string]string{
		"fire": "https://example.com/fire.png",
	}

	matched := filterEmojis(emojis, "zzz_no_match", 50)
	if len(matched) != 0 {
		t.Errorf("filterEmojis(\"zzz_no_match\") returned %d results, want 0", len(matched))
	}
}

func TestFilterEmojis_CustomType(t *testing.T) {
	emojis := map[string]string{
		"logo": "https://emoji.slack-edge.com/T123/logo/abc.png",
	}

	matched := filterEmojis(emojis, "logo", 50)
	if len(matched) != 1 {
		t.Fatalf("expected 1 result, got %d", len(matched))
	}
	if matched[0].Type != "custom" {
		t.Errorf("type = %q, want \"custom\"", matched[0].Type)
	}
	if matched[0].Value != "https://emoji.slack-edge.com/T123/logo/abc.png" {
		t.Errorf("value = %q, want the URL", matched[0].Value)
	}
}

func TestFilterEmojis_AliasType(t *testing.T) {
	emojis := map[string]string{
		"campfire": "alias:fire",
	}

	matched := filterEmojis(emojis, "campfire", 50)
	if len(matched) != 1 {
		t.Fatalf("expected 1 result, got %d", len(matched))
	}
	if matched[0].Type != "alias" {
		t.Errorf("type = %q, want \"alias\"", matched[0].Type)
	}
	if matched[0].Value != ":fire:" {
		t.Errorf("value = %q, want \":fire:\"", matched[0].Value)
	}
}

func TestFilterEmojis_NameWrappedInColons(t *testing.T) {
	emojis := map[string]string{
		"wave": "https://example.com/wave.png",
	}

	matched := filterEmojis(emojis, "wave", 50)
	if len(matched) != 1 {
		t.Fatalf("expected 1 result, got %d", len(matched))
	}
	if matched[0].Name != ":wave:" {
		t.Errorf("name = %q, want \":wave:\"", matched[0].Name)
	}
}

func TestFilterEmojis_EmptyInput(t *testing.T) {
	// Empty emoji map should return nothing.
	matched := filterEmojis(map[string]string{}, "test", 50)
	if len(matched) != 0 {
		t.Errorf("filterEmojis on empty map returned %d results, want 0", len(matched))
	}
}

func TestFilterEmojis_EmptyQuery(t *testing.T) {
	// Empty query matches everything (substring of everything).
	emojis := map[string]string{
		"fire":  "https://example.com/fire.png",
		"water": "https://example.com/water.png",
	}

	matched := filterEmojis(emojis, "", 50)
	if len(matched) != 2 {
		t.Errorf("filterEmojis with empty query returned %d results, want 2", len(matched))
	}
}
