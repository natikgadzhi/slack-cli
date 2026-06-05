package commands

import (
	"strings"
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

func TestParseReactionsInput(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		channelFlag string
		tsFlag      string
		wantChannel string
		wantTS      string
		wantErr     string
	}{
		{
			name:        "URL only",
			args:        []string{"https://team.slack.com/archives/C12345678/p1741234567123456"},
			wantChannel: "C12345678",
			wantTS:      "1741234567.123456",
		},
		{
			name:        "channel ID and ts",
			channelFlag: "C12345678",
			tsFlag:      "1741234567.123456",
			wantChannel: "C12345678",
			wantTS:      "1741234567.123456",
		},
		{
			name:        "channel name and ts",
			channelFlag: "general",
			tsFlag:      "1741234567.123456",
			wantChannel: "general",
			wantTS:      "1741234567.123456",
		},
		{
			name:    "no args, no flags",
			wantErr: "provide a message URL or both --channel and --ts",
		},
		{
			name:        "URL and channel flag",
			args:        []string{"https://team.slack.com/archives/C12345/p1741234567123456"},
			channelFlag: "general",
			wantErr:     "cannot use both a message URL argument and --channel/--ts flags",
		},
		{
			name:    "URL and ts flag",
			args:    []string{"https://team.slack.com/archives/C12345/p1741234567123456"},
			tsFlag:  "1741234567.123456",
			wantErr: "cannot use both a message URL argument and --channel/--ts flags",
		},
		{
			name:        "URL and both flags",
			args:        []string{"https://team.slack.com/archives/C12345/p1741234567123456"},
			channelFlag: "general",
			tsFlag:      "1741234567.123456",
			wantErr:     "cannot use both a message URL argument and --channel/--ts flags",
		},
		{
			name:        "channel without ts",
			channelFlag: "general",
			wantErr:     "both --channel and --ts are required when not using a URL",
		},
		{
			name:    "ts without channel",
			tsFlag:  "1741234567.123456",
			wantErr: "both --channel and --ts are required when not using a URL",
		},
		{
			name:    "invalid URL",
			args:    []string{"not-a-url"},
			wantErr: "parsing URL:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args == nil {
				tt.args = []string{}
			}
			channelID, messageTS, err := parseReactionsInput(tt.args, tt.channelFlag, tt.tsFlag)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if channelID != tt.wantChannel {
					t.Errorf("channelID = %q, want %q", channelID, tt.wantChannel)
				}
				if messageTS != tt.wantTS {
					t.Errorf("messageTS = %q, want %q", messageTS, tt.wantTS)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
