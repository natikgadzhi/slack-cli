package auth

import "strings"

// SanitizeToken strips common copy-paste artifacts from a token string.
// It returns the cleaned token and a list of warnings describing what was stripped.
//
// NOTE: This function is not yet wired into CLI commands; it will be integrated
// when the "auth set-xoxc" / "auth set-xoxd" commands are implemented in a later task.
func SanitizeToken(token string) (string, []string) {
	var warnings []string

	// Strip leading/trailing whitespace.
	t := strings.TrimSpace(token)
	if t != token {
		warnings = append(warnings, "had leading/trailing whitespace — stripped")
	}

	// Strip surrounding quotes (single or double).
	if len(t) >= 2 {
		if (t[0] == '"' && t[len(t)-1] == '"') || (t[0] == '\'' && t[len(t)-1] == '\'') {
			t = t[1 : len(t)-1]
			warnings = append(warnings, "had surrounding quotes — stripped")
		}
	}

	// Strip "Bearer " prefix (case-insensitive).
	if len(t) >= 7 && strings.EqualFold(t[:7], "bearer ") {
		t = t[7:]
		warnings = append(warnings, `had "Bearer " prefix — stripped`)
	}

	return t, warnings
}
