package auth

import (
	"testing"
)

func TestSanitizeToken(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantClean    string
		wantWarnings []string
	}{
		{
			name:         "clean token unchanged",
			input:        "xoxc-abc123",
			wantClean:    "xoxc-abc123",
			wantWarnings: nil,
		},
		{
			name:         "strips double quotes",
			input:        `"xoxc-abc123"`,
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{"had surrounding quotes — stripped"},
		},
		{
			name:         "strips single quotes",
			input:        "'xoxc-abc123'",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{"had surrounding quotes — stripped"},
		},
		{
			name:         "strips Bearer prefix",
			input:        "Bearer xoxc-abc123",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{`had "Bearer " prefix — stripped`},
		},
		{
			name:         "strips bearer lowercase prefix",
			input:        "bearer xoxc-abc123",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{`had "Bearer " prefix — stripped`},
		},
		{
			name:         "strips whitespace",
			input:        "  xoxc-abc123  ",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{"had leading/trailing whitespace — stripped"},
		},
		{
			name:      "strips multiple artifacts",
			input:     `  "Bearer xoxc-abc123"  `,
			wantClean: "xoxc-abc123",
			wantWarnings: []string{
				"had leading/trailing whitespace — stripped",
				"had surrounding quotes — stripped",
				`had "Bearer " prefix — stripped`,
			},
		},
		{
			name:         "empty string",
			input:        "",
			wantClean:    "",
			wantWarnings: nil,
		},
		{
			name:         "only whitespace",
			input:        "   ",
			wantClean:    "",
			wantWarnings: []string{"had leading/trailing whitespace — stripped"},
		},
		{
			name:         "single character token",
			input:        "x",
			wantClean:    "x",
			wantWarnings: nil,
		},
		{
			name:         "BEARER uppercase prefix",
			input:        "BEARER xoxc-abc123",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{`had "Bearer " prefix — stripped`},
		},
		{
			name:         "mismatched quotes not stripped",
			input:        `"xoxc-abc123'`,
			wantClean:    `"xoxc-abc123'`,
			wantWarnings: nil,
		},
		{
			name:         "tab whitespace stripped",
			input:        "\txoxc-abc123\t",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{"had leading/trailing whitespace — stripped"},
		},
		{
			name:         "newline whitespace stripped",
			input:        "\nxoxc-abc123\n",
			wantClean:    "xoxc-abc123",
			wantWarnings: []string{"had leading/trailing whitespace — stripped"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clean, warnings := SanitizeToken(tc.input)

			if clean != tc.wantClean {
				t.Errorf("SanitizeToken(%q) clean = %q, want %q", tc.input, clean, tc.wantClean)
			}

			if len(warnings) != len(tc.wantWarnings) {
				t.Errorf("SanitizeToken(%q) got %d warnings, want %d: %v", tc.input, len(warnings), len(tc.wantWarnings), warnings)
				return
			}

			for i, w := range warnings {
				if w != tc.wantWarnings[i] {
					t.Errorf("SanitizeToken(%q) warning[%d] = %q, want %q", tc.input, i, w, tc.wantWarnings[i])
				}
			}
		})
	}
}

func TestNormalizeXoxd(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantClean    string
		wantWarnings []string
	}{
		{
			name:         "plain cookie unchanged",
			input:        "xoxd-abc/def+ghi=",
			wantClean:    "xoxd-abc/def+ghi=",
			wantWarnings: nil,
		},
		{
			name:         "percent-encoded cookie left encoded (Slack expects this form)",
			input:        "xoxd-abc%2Fdef%2Bghi%3D",
			wantClean:    "xoxd-abc%2Fdef%2Bghi%3D",
			wantWarnings: nil,
		},
		{
			name:         "strips d= cookie-name prefix",
			input:        "d=xoxd-abc123",
			wantClean:    "xoxd-abc123",
			wantWarnings: []string{`had "d=" cookie-name prefix — stripped`},
		},
		{
			name:      "strips d= prefix but preserves encoding",
			input:     "d=xoxd-abc%2Fdef",
			wantClean: "xoxd-abc%2Fdef",
			wantWarnings: []string{
				`had "d=" cookie-name prefix — stripped`,
			},
		},
		{
			name:      "trims whitespace, preserves encoding",
			input:     "  xoxd-abc%2Fdef  ",
			wantClean: "xoxd-abc%2Fdef",
			wantWarnings: []string{
				"had leading/trailing whitespace — stripped",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clean, warnings := NormalizeXoxd(tc.input)

			if clean != tc.wantClean {
				t.Errorf("NormalizeXoxd(%q) clean = %q, want %q", tc.input, clean, tc.wantClean)
			}
			if len(warnings) != len(tc.wantWarnings) {
				t.Errorf("NormalizeXoxd(%q) got %d warnings, want %d: %v", tc.input, len(warnings), len(tc.wantWarnings), warnings)
				return
			}
			for i, w := range warnings {
				if w != tc.wantWarnings[i] {
					t.Errorf("NormalizeXoxd(%q) warning[%d] = %q, want %q", tc.input, i, w, tc.wantWarnings[i])
				}
			}
		})
	}
}
