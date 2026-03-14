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
