package commands

import (
	"testing"
)

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
