package formatting

import "testing"

type fakeUsers map[string]string

func (f fakeUsers) DisplayName(id string) string { return f[id] }

type fakeChannels map[string]string

func (f fakeChannels) ChannelName(id string) string { return f[id] }

func TestReplaceMrkdwnLinks(t *testing.T) {
	users := fakeUsers{"U012": "alice"}
	channels := fakeChannels{"C034": "general"}

	cases := []struct {
		in, want string
	}{
		{"hello <@U012|alice>", "hello @alice"},
		{"hello <@U012>", "hello @alice"},
		{"unknown <@U999>", "unknown @U999"},
		{"see <#C034|general>", "see #general"},
		{"see <#C034>", "see #general"},
		{"<!here> ping", "@here ping"},
		{"<!channel>", "@channel"},
		{"<!subteam^S05|eng>", "@eng"},
		{"link <https://example.com|click> here", "link click here"},
		{"bare <https://example.com>", "bare https://example.com"},
		{"", ""},
		{"no markup", "no markup"},
	}
	for _, c := range cases {
		got := ReplaceMrkdwnLinks(c.in, users, channels)
		if got != c.want {
			t.Errorf("ReplaceMrkdwnLinks(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReplaceMrkdwnLinks_NilResolvers(t *testing.T) {
	got := ReplaceMrkdwnLinks("hi <@U012> in <#C034>", nil, nil)
	want := "hi @U012 in #C034"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnescapeEntities(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a &gt; b", "a > b"},
		{"x &lt; y", "x < y"},
		{"Tom &amp; Jerry", "Tom & Jerry"},
		{"&gt; quoted line", "> quoted line"},
		{"&amp;lt; stays escaped once", "&lt; stays escaped once"},
		{"", ""},
		{"nothing here", "nothing here"},
	}
	for _, c := range cases {
		if got := UnescapeEntities(c.in); got != c.want {
			t.Errorf("UnescapeEntities(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestFormatMessage_UnescapesEntities is an integration check that the shared
// formatter turns Slack's &gt;/&lt;/&amp; back into real characters.
func TestFormatMessage_UnescapesEntities(t *testing.T) {
	msg := FormatMessage(map[string]any{
		"ts":   "1700000000.000000",
		"text": "if x &gt; 0 &amp;&amp; y &lt; 10",
	})
	want := "if x > 0 && y < 10"
	if msg.Text != want {
		t.Errorf("FormatMessage text = %q, want %q", msg.Text, want)
	}
}
