package commands

import (
	"testing"
)

func TestValidateMessageArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		channelFlag string
		tsFlag      string
		wantErr     string
	}{
		{
			name:    "URL only — valid",
			args:    []string{"https://team.slack.com/archives/C123/p1234"},
			wantErr: "",
		},
		{
			name:        "channel+ts only — valid",
			channelFlag: "general",
			tsFlag:      "1741234567.123456",
			wantErr:     "",
		},
		{
			name:    "no args, no flags — error",
			wantErr: "provide either a message URL or --channel and --ts",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMessageArgs(tt.args, tt.channelFlag, tt.tsFlag)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
