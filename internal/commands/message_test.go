package commands

import (
	"strings"
	"testing"
)

func TestParseMessageInput(t *testing.T) {
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
			name:        "URL only — valid",
			args:        []string{"https://team.slack.com/archives/C12345678/p1741234567123456"},
			wantChannel: "C12345678",
			wantTS:      "1741234567.123456",
		},
		{
			name:        "channel+ts only — valid",
			channelFlag: "general",
			tsFlag:      "1741234567.123456",
			wantChannel: "general",
			wantTS:      "1741234567.123456",
		},
		{
			name:    "no args, no flags — error",
			wantErr: "provide a message URL or --channel and --ts",
		},
		{
			name:        "URL and channel flag — error",
			args:        []string{"https://team.slack.com/archives/C123/p1234"},
			channelFlag: "general",
			wantErr:     "cannot combine a positional URL with --channel/--ts flags",
		},
		{
			name:    "URL and ts flag — error",
			args:    []string{"https://team.slack.com/archives/C123/p1234"},
			tsFlag:  "1741234567.123456",
			wantErr: "cannot combine a positional URL with --channel/--ts flags",
		},
		{
			name:        "URL and both flags — error",
			args:        []string{"https://team.slack.com/archives/C123/p1234"},
			channelFlag: "general",
			tsFlag:      "1741234567.123456",
			wantErr:     "cannot combine a positional URL with --channel/--ts flags",
		},
		{
			name:        "channel without ts — error",
			channelFlag: "general",
			wantErr:     "--channel and --ts must be provided together",
		},
		{
			name:    "ts without channel — error",
			tsFlag:  "1741234567.123456",
			wantErr: "--channel and --ts must be provided together",
		},
		{
			name:    "invalid URL — error",
			args:    []string{"not-a-url"},
			wantErr: "parsing URL:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args == nil {
				tt.args = []string{}
			}
			channelID, messageTS, err := parseMessageInput(tt.args, tt.channelFlag, tt.tsFlag)

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
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
