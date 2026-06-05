package commands

import (
	"testing"
)

func TestExtractReactions_ValidResponse(t *testing.T) {
	result := map[string]any{
		"ok": true,
		"message": map[string]any{
			"ts":   "1741234567.123456",
			"text": "hello",
			"reactions": []any{
				map[string]any{
					"name":  "thumbsup",
					"count": float64(3),
					"users": []any{"U111", "U222", "U333"},
				},
				map[string]any{
					"name":  "heart",
					"count": float64(1),
					"users": []any{"U444"},
				},
			},
		},
	}

	reactions := extractReactions(result)
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(reactions))
	}

	name, _ := reactions[0]["name"].(string)
	if name != "thumbsup" {
		t.Errorf("first reaction name = %q, want %q", name, "thumbsup")
	}

	count := extractCount(reactions[0])
	if count != 3 {
		t.Errorf("first reaction count = %d, want 3", count)
	}

	users, ok := reactions[0]["users"].([]any)
	if !ok || len(users) != 3 {
		t.Errorf("first reaction users count = %d, want 3", len(users))
	}
}

func TestExtractReactions_NoMessage(t *testing.T) {
	result := map[string]any{"ok": true}
	reactions := extractReactions(result)
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions, got %d", len(reactions))
	}
}

func TestExtractReactions_NoReactionsField(t *testing.T) {
	result := map[string]any{
		"ok": true,
		"message": map[string]any{
			"ts":   "1741234567.123456",
			"text": "hello",
		},
	}
	reactions := extractReactions(result)
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions, got %d", len(reactions))
	}
}

func TestExtractReactions_EmptyReactions(t *testing.T) {
	result := map[string]any{
		"ok": true,
		"message": map[string]any{
			"ts":        "1741234567.123456",
			"reactions": []any{},
		},
	}
	reactions := extractReactions(result)
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions, got %d", len(reactions))
	}
}

func TestExtractCount(t *testing.T) {
	tests := []struct {
		name string
		r    map[string]any
		want int
	}{
		{"float64", map[string]any{"count": float64(5)}, 5},
		{"int", map[string]any{"count": 7}, 7},
		{"missing", map[string]any{"name": "thumbsup"}, 0},
		{"wrong type", map[string]any{"count": "not a number"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCount(tt.r); got != tt.want {
				t.Errorf("extractCount = %d, want %d", got, tt.want)
			}
		})
	}
}

// parseMessageInput tests for the reactions context live in message_test.go
// since both commands now share the same parseMessageInput helper. The tests
// below cover reactions-specific extraction and rendering logic only.
