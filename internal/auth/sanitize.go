package auth

import (
	"net/url"
	"strings"
)

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

// LooksURLEncoded reports whether s contains at least one %XX percent-escape
// (where XX are two hex digits). This is a strong indicator that s is
// already in URL-encoded form and should not be re-encoded.
func LooksURLEncoded(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
			return true
		}
	}
	return false
}

func isHexDigit(b byte) bool {
	switch {
	case b >= '0' && b <= '9':
		return true
	case b >= 'a' && b <= 'f':
		return true
	case b >= 'A' && b <= 'F':
		return true
	}
	return false
}

// NormalizeXoxd returns the canonical URL-encoded form of an xoxd cookie
// value. The Slack `d` cookie value must be safe for an HTTP Cookie header,
// which means characters like `+`, `/`, and `=` need to be percent-encoded.
// Different browsers and devtools panels show the cookie in either form, so
// users often paste the raw (decoded) variant by mistake — that goes out on
// the wire as-is and Slack silently rejects it with `invalid_auth`.
//
// Behavior:
//   - If s already looks URL-encoded (contains %XX), it is returned unchanged
//     so we never double-encode.
//   - Otherwise s is passed through url.QueryEscape. If that changes the
//     value, a non-empty warning describes what happened; if it doesn't
//     (no special characters needed encoding), the warning is empty.
func NormalizeXoxd(s string) (normalized string, warning string) {
	if s == "" {
		return s, ""
	}
	if LooksURLEncoded(s) {
		return s, ""
	}
	encoded := url.QueryEscape(s)
	if encoded == s {
		// Nothing needed encoding; safe to send as-is.
		return s, ""
	}
	return encoded, "xoxd appears to be in raw (decoded) form — auto-encoded to URL-encoded form for Cookie header compatibility"
}

// SanitizeXoxd is SanitizeToken followed by NormalizeXoxd. Use this for the
// xoxd cookie path (e.g. `auth set-xoxd`) so values are both cleaned of
// copy-paste artifacts and normalized to the wire form Slack expects.
func SanitizeXoxd(token string) (string, []string) {
	clean, warnings := SanitizeToken(token)
	normalized, w := NormalizeXoxd(clean)
	if w != "" {
		warnings = append(warnings, w)
	}
	return normalized, warnings
}
