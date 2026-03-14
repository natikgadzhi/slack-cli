package formatting

import (
	"math"
	"testing"
	"time"
)

func TestParseTime_Minutes(t *testing.T) {
	now := time.Now().UTC()
	result, err := ParseTimeWithNow("30m", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := float64(now.Add(-30 * time.Minute).Unix())
	if math.Abs(result-expected) > 2 {
		t.Errorf("result = %f, want ~%f", result, expected)
	}
}

func TestParseTime_Hours(t *testing.T) {
	now := time.Now().UTC()
	result, err := ParseTimeWithNow("3h", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := float64(now.Add(-3 * time.Hour).Unix())
	if math.Abs(result-expected) > 2 {
		t.Errorf("result = %f, want ~%f", result, expected)
	}
}

func TestParseTime_Days(t *testing.T) {
	now := time.Now().UTC()
	result, err := ParseTimeWithNow("2d", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := float64(now.Add(-2 * 24 * time.Hour).Unix())
	if math.Abs(result-expected) > 2 {
		t.Errorf("result = %f, want ~%f", result, expected)
	}
}

func TestParseTime_Weeks(t *testing.T) {
	now := time.Now().UTC()
	result, err := ParseTimeWithNow("1w", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := float64(now.Add(-7 * 24 * time.Hour).Unix())
	if math.Abs(result-expected) > 2 {
		t.Errorf("result = %f, want ~%f", result, expected)
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
