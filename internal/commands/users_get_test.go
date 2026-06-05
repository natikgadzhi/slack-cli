package commands

import (
	"testing"
)

func TestExtractFullUserProfile(t *testing.T) {
	user := map[string]any{
		"id":        "U12345678",
		"name":      "alice",
		"real_name": "Alice Smith",
		"tz":        "America/Los_Angeles",
		"is_admin":  true,
		"is_owner":  false,
		"is_bot":    false,
		"deleted":   false,
		"profile": map[string]any{
			"display_name": "Alice",
			"email":        "alice@example.com",
			"title":        "Engineer",
			"status_text":  "Working",
			"status_emoji": ":computer:",
			"phone":        "+1234567890",
		},
	}

	got := extractFullUserProfile(user)

	stringTests := []struct {
		key  string
		want string
	}{
		{"id", "U12345678"},
		{"name", "alice"},
		{"real_name", "Alice Smith"},
		{"display_name", "Alice"},
		{"email", "alice@example.com"},
		{"title", "Engineer"},
		{"status_text", "Working"},
		{"status_emoji", ":computer:"},
		{"timezone", "America/Los_Angeles"},
		{"phone", "+1234567890"},
	}

	for _, tc := range stringTests {
		val, ok := got[tc.key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", tc.key, got[tc.key])
			continue
		}
		if val != tc.want {
			t.Errorf("extractFullUserProfile[%q] = %q, want %q", tc.key, val, tc.want)
		}
	}

	boolTests := []struct {
		key  string
		want bool
	}{
		{"is_admin", true},
		{"is_owner", false},
		{"is_bot", false},
		{"deleted", false},
	}

	for _, tc := range boolTests {
		val, ok := got[tc.key].(bool)
		if !ok {
			t.Errorf("expected key %q to be a bool, got %T", tc.key, got[tc.key])
			continue
		}
		if val != tc.want {
			t.Errorf("extractFullUserProfile[%q] = %v, want %v", tc.key, val, tc.want)
		}
	}
}

func TestExtractFullUserProfile_MissingProfile(t *testing.T) {
	user := map[string]any{
		"id":   "U12345678",
		"name": "bob",
	}

	got := extractFullUserProfile(user)

	// Profile-dependent fields should be empty strings.
	for _, key := range []string{"display_name", "email", "title", "status_text", "status_emoji", "phone"} {
		val, ok := got[key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", key, got[key])
			continue
		}
		if val != "" {
			t.Errorf("extractFullUserProfile[%q] = %q, want empty string", key, val)
		}
	}
}

func TestExtractFullUserProfile_EmptyUser(t *testing.T) {
	user := map[string]any{}

	got := extractFullUserProfile(user)

	// All string fields should be empty.
	for _, key := range []string{"id", "name", "real_name", "display_name", "email", "title", "status_text", "status_emoji", "timezone", "phone"} {
		val, ok := got[key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", key, got[key])
			continue
		}
		if val != "" {
			t.Errorf("extractFullUserProfile[%q] = %q, want empty string", key, val)
		}
	}

	// All bool fields should be false.
	for _, key := range []string{"is_admin", "is_owner", "is_bot", "deleted"} {
		val, ok := got[key].(bool)
		if !ok {
			t.Errorf("expected key %q to be a bool, got %T", key, got[key])
			continue
		}
		if val {
			t.Errorf("extractFullUserProfile[%q] = %v, want false", key, val)
		}
	}
}

// getBool tests live in helpers_test.go since the function is now in helpers.go.
