package commands

import (
	"testing"
)

func TestFilterUser_ActiveHuman(t *testing.T) {
	member := map[string]any{
		"id":      "U12345678",
		"name":    "alice",
		"is_bot":  false,
		"deleted": false,
	}
	if !filterUser(member, false, false) {
		t.Error("expected active human user to be included")
	}
}

func TestFilterUser_Bot(t *testing.T) {
	member := map[string]any{
		"id":      "U_BOT_123",
		"name":    "mybot",
		"is_bot":  true,
		"deleted": false,
	}

	// Excluded by default.
	if filterUser(member, false, false) {
		t.Error("expected bot to be excluded by default")
	}

	// Included with flag.
	if !filterUser(member, true, false) {
		t.Error("expected bot to be included with --include-bots")
	}
}

func TestFilterUser_Deactivated(t *testing.T) {
	member := map[string]any{
		"id":      "U_DEACT_1",
		"name":    "bob",
		"is_bot":  false,
		"deleted": true,
	}

	// Excluded by default.
	if filterUser(member, false, false) {
		t.Error("expected deactivated user to be excluded by default")
	}

	// Included with flag.
	if !filterUser(member, false, true) {
		t.Error("expected deactivated user to be included with --include-deactivated")
	}
}

func TestFilterUser_SlackBot(t *testing.T) {
	member := map[string]any{
		"id":      "USLACKBOT",
		"name":    "slackbot",
		"is_bot":  false,
		"deleted": false,
	}

	// Always excluded, even with all include flags set.
	if filterUser(member, true, true) {
		t.Error("expected USLACKBOT to always be excluded")
	}
}

func TestExtractUserFields(t *testing.T) {
	member := map[string]any{
		"id":        "U12345678",
		"name":      "alice",
		"real_name": "Alice Smith",
		"profile": map[string]any{
			"email": "alice@example.com",
		},
	}

	got := extractUserFields(member)

	tests := []struct {
		key  string
		want string
	}{
		{"id", "U12345678"},
		{"name", "alice"},
		{"real_name", "Alice Smith"},
		{"email", "alice@example.com"},
	}

	for _, tc := range tests {
		val, ok := got[tc.key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", tc.key, got[tc.key])
			continue
		}
		if val != tc.want {
			t.Errorf("extractUserFields[%q] = %q, want %q", tc.key, val, tc.want)
		}
	}
}

func TestExtractUserFields_MissingEmail(t *testing.T) {
	member := map[string]any{
		"id":        "U12345678",
		"name":      "alice",
		"real_name": "Alice Smith",
		// No profile field at all.
	}

	got := extractUserFields(member)

	email, ok := got["email"].(string)
	if !ok {
		t.Fatalf("expected email to be a string, got %T", got["email"])
	}
	if email != "" {
		t.Errorf("expected empty email, got %q", email)
	}
}

func TestExtractUserFields_EmptyProfileEmail(t *testing.T) {
	member := map[string]any{
		"id":        "U12345678",
		"name":      "bob",
		"real_name": "Bob Jones",
		"profile":   map[string]any{},
	}

	got := extractUserFields(member)

	email, ok := got["email"].(string)
	if !ok {
		t.Fatalf("expected email to be a string, got %T", got["email"])
	}
	if email != "" {
		t.Errorf("expected empty email when profile has no email, got %q", email)
	}
}

func TestExtractUserFields_MissingFields(t *testing.T) {
	// Completely empty member — all fields should default to "".
	member := map[string]any{}

	got := extractUserFields(member)

	for _, key := range []string{"id", "name", "real_name", "email"} {
		val, ok := got[key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", key, got[key])
			continue
		}
		if val != "" {
			t.Errorf("extractUserFields[%q] = %q, want empty string", key, val)
		}
	}
}
