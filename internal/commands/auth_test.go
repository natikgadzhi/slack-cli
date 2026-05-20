package commands

import (
	"testing"
)

// slackAuthErrorHint is a pure mapping; each known code must produce both
// a non-empty explanation and a non-empty actionable remedy. Unknown codes
// return empty strings so the caller can fall back to the raw error message.
func TestSlackAuthErrorHint(t *testing.T) {
	knownCodes := []string{
		"invalid_auth",
		"not_authed",
		"invalid_cookie",
		"no_auth_in_cookie",
		"token_expired",
		"token_revoked",
		"account_inactive",
		"missing_scope",
		"no_permission",
		"user_is_bot",
		"org_login_with_sso",
	}
	for _, code := range knownCodes {
		t.Run(code, func(t *testing.T) {
			exp, rem := slackAuthErrorHint(code)
			if exp == "" {
				t.Errorf("slackAuthErrorHint(%q) returned empty explanation", code)
			}
			if rem == "" {
				t.Errorf("slackAuthErrorHint(%q) returned empty remedy", code)
			}
		})
	}

	t.Run("unknown code returns empty pair", func(t *testing.T) {
		exp, rem := slackAuthErrorHint("totally_made_up_code")
		if exp != "" || rem != "" {
			t.Errorf("slackAuthErrorHint(unknown) = (%q, %q), want empty pair", exp, rem)
		}
	})

	t.Run("empty code returns empty pair", func(t *testing.T) {
		exp, rem := slackAuthErrorHint("")
		if exp != "" || rem != "" {
			t.Errorf("slackAuthErrorHint(\"\") = (%q, %q), want empty pair", exp, rem)
		}
	})
}

// xoxdFallbacks returns the alternate-encoding attempts for a stored xoxd.
// Verify the encode-vs-decode branching and that empty/equal alternates are
// filtered.
func TestXoxdFallbacks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLabel string // empty means: want no fallbacks
		wantValue string
	}{
		{
			name:      "raw form yields URL-encoded fallback",
			input:     "xoxd-X+y/Z=",
			wantLabel: "URL-encoded",
			wantValue: "xoxd-X%2By%2FZ%3D",
		},
		{
			name:      "URL-encoded form yields URL-decoded fallback",
			input:     "xoxd-X%2By%2FZ%3D",
			wantLabel: "URL-decoded",
			wantValue: "xoxd-X+y/Z=",
		},
		{
			name:      "plain value (no special chars) yields no fallback",
			input:     "xoxd-AbCdEf123456",
			wantLabel: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := xoxdFallbacks(tc.input)
			if tc.wantLabel == "" {
				if len(got) != 0 {
					t.Errorf("xoxdFallbacks(%q) = %+v, want empty", tc.input, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("xoxdFallbacks(%q) = %+v, want exactly 1 fallback", tc.input, got)
			}
			if got[0].label != tc.wantLabel {
				t.Errorf("xoxdFallbacks(%q) label = %q, want %q", tc.input, got[0].label, tc.wantLabel)
			}
			if got[0].value != tc.wantValue {
				t.Errorf("xoxdFallbacks(%q) value = %q, want %q", tc.input, got[0].value, tc.wantValue)
			}
		})
	}
}
