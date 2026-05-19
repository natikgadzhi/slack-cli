package auth

import "strings"

// SanitizeToken strips common copy-paste artifacts from a token string.
// It returns the cleaned token and a list of warnings describing what was stripped.
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

// NormalizeXoxd cleans an xoxd cookie value beyond the generic SanitizeToken
// rules. The browser-stored "d" cookie is commonly copied with the cookie name
// still attached ("d=xoxd-..."), so we strip that here.
//
// Note: the "d" cookie is percent-encoded in the browser, and Slack expects it
// sent in exactly that form. We deliberately do NOT URL-decode it — decoding
// turns a valid cookie into the wrong bytes and breaks authentication.
//
// SanitizeToken is applied first, so callers can pass a raw pasted value.
func NormalizeXoxd(token string) (string, []string) {
	t, warnings := SanitizeToken(token)

	// Strip a leading "d=" cookie name (case-insensitive) — people often copy
	// the whole "name=value" pair out of devtools.
	if len(t) >= 2 && strings.EqualFold(t[:2], "d=") {
		t = t[2:]
		warnings = append(warnings, `had "d=" cookie-name prefix — stripped`)
	}

	return t, warnings
}
