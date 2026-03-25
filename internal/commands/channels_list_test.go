package commands

import "testing"

func TestExtractChannelFields(t *testing.T) {
	ch := map[string]any{
		"id":          "C12345",
		"name":        "general",
		"num_members": float64(42),
		"is_archived": true,
		"is_private":  false,
		"is_mpim":     false,
		"is_im":       false,
		"topic": map[string]any{
			"value": "Welcome to general",
		},
		"purpose": map[string]any{
			"value": "General discussion",
		},
	}

	result := extractChannelFields(ch)

	if result["id"] != "C12345" {
		t.Errorf("id = %v, want C12345", result["id"])
	}
	if result["name"] != "general" {
		t.Errorf("name = %v, want general", result["name"])
	}
	if result["type"] != "public_channel" {
		t.Errorf("type = %v, want public_channel", result["type"])
	}
	if result["num_members"] != 42 {
		t.Errorf("num_members = %v, want 42", result["num_members"])
	}
	if result["is_archived"] != true {
		t.Errorf("is_archived = %v, want true", result["is_archived"])
	}
	if result["topic"] != "Welcome to general" {
		t.Errorf("topic = %v, want 'Welcome to general'", result["topic"])
	}
	if result["purpose"] != "General discussion" {
		t.Errorf("purpose = %v, want 'General discussion'", result["purpose"])
	}
}

func TestExtractChannelFields_MissingFields(t *testing.T) {
	ch := map[string]any{}

	result := extractChannelFields(ch)

	if result["id"] != "" {
		t.Errorf("id = %v, want empty string", result["id"])
	}
	if result["name"] != "" {
		t.Errorf("name = %v, want empty string", result["name"])
	}
	if result["type"] != "public_channel" {
		t.Errorf("type = %v, want public_channel", result["type"])
	}
	if result["num_members"] != 0 {
		t.Errorf("num_members = %v, want 0", result["num_members"])
	}
	if result["is_archived"] != false {
		t.Errorf("is_archived = %v, want false", result["is_archived"])
	}
	if result["topic"] != "" {
		t.Errorf("topic = %v, want empty string", result["topic"])
	}
	if result["purpose"] != "" {
		t.Errorf("purpose = %v, want empty string", result["purpose"])
	}
}

func TestDeriveChannelType(t *testing.T) {
	tests := []struct {
		name string
		ch   map[string]any
		want string
	}{
		{
			name: "public channel",
			ch: map[string]any{
				"is_private": false,
				"is_mpim":    false,
				"is_im":      false,
			},
			want: "public_channel",
		},
		{
			name: "private channel",
			ch: map[string]any{
				"is_private": true,
				"is_mpim":    false,
				"is_im":      false,
			},
			want: "private_channel",
		},
		{
			name: "mpim",
			ch: map[string]any{
				"is_private": true,
				"is_mpim":    true,
				"is_im":      false,
			},
			want: "mpim",
		},
		{
			name: "im",
			ch: map[string]any{
				"is_private": false,
				"is_mpim":    false,
				"is_im":      true,
			},
			want: "im",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveChannelType(tt.ch)
			if got != tt.want {
				t.Errorf("deriveChannelType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeriveChannelType_Default(t *testing.T) {
	// When no type flags are present, should default to public_channel.
	ch := map[string]any{
		"id":   "C99999",
		"name": "unknown",
	}

	got := deriveChannelType(ch)
	if got != "public_channel" {
		t.Errorf("deriveChannelType() = %v, want public_channel", got)
	}
}
