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
