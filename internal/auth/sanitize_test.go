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

func TestLooksURLEncoded(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"plain", false},
		{"xoxd-abc-def", false},
		{"xoxd-X+y/Z=", false},      // raw — needs encoding, not encoded
		{"xoxd-X%2By%2FZ%3D", true}, // %2B, %2F, %3D present
		{"%2B", true},               // minimal positive
		{"%aB", true},               // mixed-case hex
		{"%ZZ", false},              // %XX present but XX not hex
		{"trailing-%", false},       // bare % at end
		{"%2", false},               // truncated %X
		{"xoxd-%20-encoded-space", true},
		{"50%-off", false}, // % not followed by hex
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := LooksURLEncoded(tc.input); got != tc.want {
				t.Errorf("LooksURLEncoded(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeXoxd(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantOut     string
		wantWarning bool
	}{
		{
			name:        "raw form with special chars gets encoded",
			input:       "xoxd-X+y/Z=",
			wantOut:     "xoxd-X%2By%2FZ%3D",
			wantWarning: true,
		},
		{
			name:        "already URL-encoded passes through",
			input:       "xoxd-X%2By%2FZ%3D",
			wantOut:     "xoxd-X%2By%2FZ%3D",
			wantWarning: false,
		},
		{
			name:        "plain base64 without special chars passes through",
			input:       "xoxd-AbCdEf123456",
			wantOut:     "xoxd-AbCdEf123456",
			wantWarning: false,
		},
		{
			name:        "empty string passes through",
			input:       "",
			wantOut:     "",
			wantWarning: false,
		},
		{
			name:        "mixed: looks encoded, leave alone even if contains raw chars",
			input:       "xoxd-X%2By+raw/here",
			wantOut:     "xoxd-X%2By+raw/here",
			wantWarning: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, warning := NormalizeXoxd(tc.input)
			if got != tc.wantOut {
				t.Errorf("NormalizeXoxd(%q) out = %q, want %q", tc.input, got, tc.wantOut)
			}
			if (warning != "") != tc.wantWarning {
				t.Errorf("NormalizeXoxd(%q) warning = %q, want non-empty? %v", tc.input, warning, tc.wantWarning)
			}
		})
	}
}

func TestSanitizeXoxd(t *testing.T) {
	// Combined: whitespace + quotes + raw form → cleaned + encoded with three warnings.
	clean, warnings := SanitizeXoxd(`  "xoxd-X+y/Z="  `)
	const want = "xoxd-X%2By%2FZ%3D"
	if clean != want {
		t.Errorf("SanitizeXoxd combined: got %q, want %q", clean, want)
	}
	if len(warnings) != 3 {
		t.Errorf("SanitizeXoxd combined: got %d warnings, want 3: %v", len(warnings), warnings)
	}

	// Already-encoded value with no copy-paste cruft → no changes, no warnings.
	clean, warnings = SanitizeXoxd("xoxd-X%2By%2FZ%3D")
	if clean != "xoxd-X%2By%2FZ%3D" {
		t.Errorf("SanitizeXoxd encoded passthrough: got %q", clean)
	}
	if len(warnings) != 0 {
		t.Errorf("SanitizeXoxd encoded passthrough: got %d warnings, want 0: %v", len(warnings), warnings)
	}

	clean, warnings = SanitizeXoxd("d=xoxd-X+y/Z=")
	if clean != want {
		t.Errorf("SanitizeXoxd d= prefix: got %q, want %q", clean, want)
	}
	if len(warnings) != 2 {
		t.Errorf("SanitizeXoxd d= prefix: got %d warnings, want 2: %v", len(warnings), warnings)
	}
}

func TestStripCookieName(t *testing.T) {
	got, stripped := StripCookieName("D=xoxd-abc", "d")
	if !stripped || got != "xoxd-abc" {
		t.Errorf("StripCookieName case-insensitive = (%q, %v), want stripped xoxd-abc", got, stripped)
	}

	got, stripped = StripCookieName("xoxd-abc", "d")
	if stripped || got != "xoxd-abc" {
		t.Errorf("StripCookieName without prefix = (%q, %v), want unchanged false", got, stripped)
	}
}
