package formatting

import (
	"testing"
	"time"
)

func TestParseTime_Relative(t *testing.T) {
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"30 minutes", "30m", float64(now.Add(-30 * time.Minute).Unix())},
		{"3 hours", "3h", float64(now.Add(-3 * time.Hour).Unix())},
		{"2 days", "2d", float64(now.Add(-2 * 24 * time.Hour).Unix())},
		{"1 week", "1w", float64(now.Add(-7 * 24 * time.Hour).Unix())},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimeWithNow(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("result = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestParseTime_AbsoluteDate(t *testing.T) {
	result, err := ParseTime("2026-03-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).Unix()
	if result != float64(expected) {
		t.Errorf("result = %f, want %f", result, float64(expected))
	}
}

func TestParseTime_AbsoluteDatetime(t *testing.T) {
	result, err := ParseTime("2026-03-01T14:00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC).Unix()
	if result != float64(expected) {
		t.Errorf("result = %f, want %f", result, float64(expected))
	}
}

func TestParseTime_UnixTimestamp(t *testing.T) {
	result, err := ParseTime("1741234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1741234567.0 {
		t.Errorf("result = %f, want %f", result, 1741234567.0)
	}
}

func TestParseTime_Invalid(t *testing.T) {
	_, err := ParseTime("not-a-time")
	if err == nil {
		t.Fatal("expected error for invalid time, got nil")
	}
}
