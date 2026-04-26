package formatting

import (
	"strings"

	"github.com/kyokomi/emoji/v2"
)

func init() {
	// kyokomi/emoji inserts a trailing " " after each substitution for
	// terminal rendering. We want the raw emoji so it lines up in tables
	// and matches what users expect visually.
	emoji.ReplacePadding = ""
}

// ReplaceEmojiShortcodes replaces :shortcode: tokens with their Unicode emoji.
// Unknown shortcodes are preserved as-is. Skin-tone modifiers (::skin-tone-2, etc.)
// common in Slack emoji reactions are stripped since they don't have useful
// Unicode equivalents in plain text.
func ReplaceEmojiShortcodes(s string) string {
	if s == "" {
		return s
	}
	// Strip Slack skin-tone suffixes: ":wave::skin-tone-2:" -> ":wave:".
	s = stripSkinTones(s)
	return emoji.Sprint(s)
}

// stripSkinTones removes `::skin-tone-N:` suffixes from shortcodes.
func stripSkinTones(s string) string {
	for i := 1; i <= 6; i++ {
		tag := "::skin-tone-" + string(rune('0'+i)) + ":"
		s = strings.ReplaceAll(s, tag, ":")
	}
	return s
}
