package formatting

import "testing"

func TestReplaceEmojiShortcodes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{":thread:", "🧵"},
		{"hello :wave:", "hello 👋"},
		{":wave::skin-tone-2:", "👋"},
		{"no tokens here", "no tokens here"},
		{":not_an_emoji_xxxyyy:", ":not_an_emoji_xxxyyy:"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ReplaceEmojiShortcodes(c.in); got != c.want {
			t.Errorf("ReplaceEmojiShortcodes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
