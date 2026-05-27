package commands

import (
	"strings"
	"testing"
)

// --- parseActivityItem ------------------------------------------------------

func TestParseActivityItem_Mention(t *testing.T) {
	raw := map[string]any{
		"feed_ts": "1776600000.000300",
		"item": map[string]any{
			"type": "at_user",
			"message": map[string]any{
				"channel":        "C01ENG00000",
				"ts":             "1776600000.000300",
				"author_user_id": "U01MIKE000",
				"text":           "hey <@U01NATIK00>",
			},
		},
	}
	it, ok := parseActivityItem(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if it.kind != kindMention {
		t.Errorf("kind = %q, want %q", it.kind, kindMention)
	}
	if it.channelID != "C01ENG00000" {
		t.Errorf("channelID = %q", it.channelID)
	}
	if it.messageTS != "1776600000.000300" {
		t.Errorf("messageTS = %q", it.messageTS)
	}
	// author_user_id should be normalized into the "user" field.
	if got := getString(it.message, "user"); got != "U01MIKE000" {
		t.Errorf("normalized user = %q, want U01MIKE000", got)
	}
}

func TestParseActivityItem_ReactionFields(t *testing.T) {
	raw := map[string]any{
		"item": map[string]any{
			"type":     "message_reaction",
			"reaction": map[string]any{"user": "U01ALICE00", "name": "thumbsup"},
			"message":  map[string]any{"channel": "C1", "ts": "1.0", "user": "U01NATIK00", "text": "x"},
		},
	}
	it, ok := parseActivityItem(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if it.kind != kindReaction {
		t.Errorf("kind = %q, want %q", it.kind, kindReaction)
	}
	if it.reactor != "U01ALICE00" || it.reaction != "thumbsup" {
		t.Errorf("reactor=%q reaction=%q", it.reactor, it.reaction)
	}
}

func TestParseActivityItem_Keyword(t *testing.T) {
	raw := map[string]any{
		"feed_ts": "1776600000.000000",
		"item": map[string]any{
			"type":    "keyword",
			"message": map[string]any{"channel": "C01ENG00000", "ts": "1776600000.000000", "user": "U01MIKE000", "text": "the deploy is green"},
		},
	}
	it, ok := parseActivityItem(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if it.kind != kindKeyword {
		t.Errorf("kind = %q, want %q", it.kind, kindKeyword)
	}
	if it.channelID != "C01ENG00000" || it.messageTS != "1776600000.000000" {
		t.Errorf("channel=%q ts=%q", it.channelID, it.messageTS)
	}
}

func TestParseActivityItem_ChannelInvite(t *testing.T) {
	cases := []struct {
		name        string
		raw         map[string]any
		wantOK      bool
		wantChannel string
		wantInviter string
	}{
		{
			name: "internal invite via invite_info",
			raw: map[string]any{"feed_ts": "1.0", "item": map[string]any{
				"type":        "internal_channel_invite",
				"invite_info": map[string]any{"channel_id": "C01NEWCHAN", "inviter_user_id": "U01MIKE000"},
			}},
			wantOK:      true,
			wantChannel: "C01NEWCHAN",
			wantInviter: "U01MIKE000",
		},
		{
			name: "external (slack connect) invite via invite_info",
			raw: map[string]any{"feed_ts": "1.0", "item": map[string]any{
				"type":        "external_channel_invite",
				"invite_info": map[string]any{"channel_id": "C01SHARED00"},
			}},
			wantOK:      true,
			wantChannel: "C01SHARED00",
		},
		{
			name: "invite_info without a channel is skipped",
			raw: map[string]any{"feed_ts": "1.0", "item": map[string]any{
				"type":        "internal_channel_invite",
				"invite_info": map[string]any{"inviter_user_id": "U01MIKE000"},
			}},
			wantOK: false,
		},
	}
	for _, c := range cases {
		it, ok := parseActivityItem(c.raw)
		if ok != c.wantOK {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if !c.wantOK {
			continue
		}
		if it.kind != kindInvite {
			t.Errorf("%s: kind = %q, want %q", c.name, it.kind, kindInvite)
		}
		if it.channelID != c.wantChannel {
			t.Errorf("%s: channel = %q, want %q", c.name, it.channelID, c.wantChannel)
		}
		if c.wantInviter != "" && getString(it.message, "user") != c.wantInviter {
			t.Errorf("%s: inviter = %q, want %q", c.name, getString(it.message, "user"), c.wantInviter)
		}
	}
}

func TestActivityTypesFor(t *testing.T) {
	base := activityTypesFor(false)
	for _, want := range []string{"at_user", "keyword", "internal_channel_invite", "external_channel_invite"} {
		if !sliceHas(base, want) {
			t.Errorf("default types missing %q: %v", want, base)
		}
	}
	if sliceHas(base, activityTypeReaction) {
		t.Errorf("reactions should be excluded by default: %v", base)
	}
	if !sliceHas(activityTypesFor(true), activityTypeReaction) {
		t.Error("reactions should be included when opted in")
	}
}

func sliceHas(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestParseActivityItem_Skips(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
	}{
		{"thread_v2 not handled here", map[string]any{"item": map[string]any{"type": "thread_v2"}}},
		{"bot_dm_bundle not handled here", map[string]any{"item": map[string]any{"type": "bot_dm_bundle"}}},
		{"generic_system_alert is not an invite", map[string]any{"item": map[string]any{"type": "generic_system_alert", "generic_system_alert_payload": map[string]any{"click_target_id": "C01X"}}}},
		{"unknown type", map[string]any{"item": map[string]any{"type": "weird"}}},
		{"missing channel", map[string]any{"item": map[string]any{"type": "at_user", "message": map[string]any{"ts": "1.0"}}}},
		{"missing ts", map[string]any{"item": map[string]any{"type": "at_user", "message": map[string]any{"channel": "C1"}}}},
		{"no item", map[string]any{"feed_ts": "1.0"}},
	}
	for _, c := range cases {
		if _, ok := parseActivityItem(c.raw); ok {
			t.Errorf("%s: expected skip", c.name)
		}
	}
}

// --- parseCountEntries ------------------------------------------------------

func TestParseCountEntries(t *testing.T) {
	result := map[string]any{
		"ims": []any{
			map[string]any{"id": "D1", "has_unreads": true, "last_read": "1.0"},
			map[string]any{"id": "D2", "has_unreads": false, "last_read": "2.0"},
			map[string]any{"has_unreads": true}, // no id -> skipped
		},
	}
	got := parseCountEntries(result, "ims")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].id != "D1" || !got[0].hasUnreads || got[0].lastRead != "1.0" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].hasUnreads {
		t.Errorf("got[1] should not have unreads: %+v", got[1])
	}
}

// --- dedupeUnread -----------------------------------------------------------

func TestDedupeUnread(t *testing.T) {
	items := []unreadItem{
		{channelID: "C", messageTS: "1.0", kind: kindMention},
		{channelID: "C", messageTS: "1.0", kind: kindThread}, // duplicate of first
		{channelID: "C", messageTS: "2.0", kind: kindThread},
		{channelID: "D", messageTS: "1.0", kind: kindDM}, // different channel, kept
		{channelID: "", messageTS: "", kind: kindApp},    // no ts, always kept
	}
	got := dedupeUnread(items)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (one dup removed)", len(got))
	}
	if got[0].kind != kindMention {
		t.Errorf("first kept should be the mention, got %q", got[0].kind)
	}
}

// --- buildUnreadRows --------------------------------------------------------

func TestBuildUnreadRows_KindEmojiMrkdwnPermalink(t *testing.T) {
	items := []unreadItem{
		{
			channelID: "C01ENG00000",
			messageTS: "1776600000.000300",
			kind:      kindMention,
			message: map[string]any{
				"ts":   "1776600000.000300",
				"user": "mike",
				"text": "hey <@U01NATIK00> :thread:",
			},
		},
	}
	chans := savedChannelMap{"C01ENG00000": {id: "C01ENG00000", displayName: "eng"}}
	usr := fakeUserResolver{m: map[string]string{"U01NATIK00": "Natik"}}

	rows, byChannel := buildUnreadRows(items, chans, usr, "https://team.slack.com", true)
	if len(rows) != 1 {
		t.Fatalf("len rows = %d", len(rows))
	}
	if rows[0].Kind != kindMention {
		t.Errorf("kind = %q", rows[0].Kind)
	}
	if rows[0].Conversation != "eng" {
		t.Errorf("conversation = %q", rows[0].Conversation)
	}
	if rows[0].Text != "hey @Natik 🧵" {
		t.Errorf("text = %q, want %q", rows[0].Text, "hey @Natik 🧵")
	}
	wantPerma := "https://team.slack.com/archives/C01ENG00000/p1776600000000300"
	if rows[0].Permalink != wantPerma {
		t.Errorf("permalink = %q, want %q", rows[0].Permalink, wantPerma)
	}
	if rows[0].ConversationURL != "https://team.slack.com/archives/C01ENG00000" {
		t.Errorf("conversationURL = %q", rows[0].ConversationURL)
	}
	if len(byChannel["C01ENG00000"]) != 1 {
		t.Errorf("byChannel should hold 1 formatted message, got %d", len(byChannel["C01ENG00000"]))
	}
}

func TestBuildUnreadRows_Reaction(t *testing.T) {
	items := []unreadItem{
		{
			channelID: "C1",
			messageTS: "1.0",
			kind:      kindReaction,
			reactor:   "Alice",
			reaction:  "thumbsup",
			message:   map[string]any{"ts": "1.0", "text": "shipped"},
		},
	}
	rows, _ := buildUnreadRows(items, savedChannelMap{}, fakeUserResolver{}, "", false)
	if len(rows) != 1 {
		t.Fatalf("len rows = %d", len(rows))
	}
	if rows[0].Kind != kindReaction {
		t.Errorf("kind = %q", rows[0].Kind)
	}
	if !strings.Contains(rows[0].Text, "👍") {
		t.Errorf("reaction emoji not substituted: %q", rows[0].Text)
	}
	if !strings.Contains(rows[0].Text, "from @Alice") || !strings.Contains(rows[0].Text, "shipped") {
		t.Errorf("reaction text = %q", rows[0].Text)
	}
	// The reactor is in the text; the row's User must be cleared so the table
	// doesn't prepend a redundant "@<you>:".
	if rows[0].User != "" {
		t.Errorf("reaction row User = %q, want empty", rows[0].User)
	}
}

func TestBuildUnreadRows_Invite(t *testing.T) {
	items := []unreadItem{
		// The inviter id is stashed on the message "user" field (already resolved
		// to a display name by the time buildUnreadRows runs).
		{channelID: "C01NEWCHAN", kind: kindInvite, message: map[string]any{"user": "Mike Turco"}},
	}
	chans := savedChannelMap{"C01NEWCHAN": {id: "C01NEWCHAN", displayName: "new-project"}}

	rows, _ := buildUnreadRows(items, chans, fakeUserResolver{}, "https://team.slack.com", true)
	if len(rows) != 1 {
		t.Fatalf("len rows = %d", len(rows))
	}
	if rows[0].Kind != kindInvite {
		t.Errorf("kind = %q", rows[0].Kind)
	}
	if rows[0].Conversation != "new-project" {
		t.Errorf("conversation = %q", rows[0].Conversation)
	}
	if rows[0].Text != "invited you to this channel" {
		t.Errorf("text = %q", rows[0].Text)
	}
	if rows[0].User != "Mike Turco" {
		t.Errorf("invite row should show the inviter, got %q", rows[0].User)
	}
	// Conversation links to the channel; there's no per-message permalink.
	if rows[0].ConversationURL != "https://team.slack.com/archives/C01NEWCHAN" {
		t.Errorf("conversationURL = %q", rows[0].ConversationURL)
	}
	if rows[0].Permalink != "" {
		t.Errorf("invite should have no permalink, got %q", rows[0].Permalink)
	}
}

func TestBuildUnreadRows_NoTeamURL(t *testing.T) {
	items := []unreadItem{
		{channelID: "C1", messageTS: "1.0", kind: kindDM, message: map[string]any{"ts": "1.0", "text": "hi"}},
	}
	rows, _ := buildUnreadRows(items, savedChannelMap{}, fakeUserResolver{}, "", false)
	if rows[0].Permalink != "" || rows[0].ConversationURL != "" {
		t.Errorf("without teamURL no links expected, got %+v", rows[0])
	}
	// Conversation falls back to the channel ID when unresolved.
	if rows[0].Conversation != "C1" {
		t.Errorf("conversation fallback = %q, want C1", rows[0].Conversation)
	}
}

// --- isBotMessage -----------------------------------------------------------

func TestIsBotMessage(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]any
		want bool
	}{
		{"nil", nil, false},
		{"human", map[string]any{"user": "U1", "text": "hi"}, false},
		{"bot_id", map[string]any{"bot_id": "B1", "text": "hi"}, true},
		{"app_id", map[string]any{"app_id": "A1", "text": "hi"}, true},
		{"subtype bot_message", map[string]any{"subtype": "bot_message"}, true},
		{"bot_profile", map[string]any{"bot_profile": map[string]any{"name": "x"}}, true},
		{"empty bot_id ignored", map[string]any{"bot_id": "", "user": "U1"}, false},
	}
	for _, c := range cases {
		if got := isBotMessage(c.m); got != c.want {
			t.Errorf("%s: isBotMessage = %v, want %v", c.name, got, c.want)
		}
	}
}

// --- parseTS ----------------------------------------------------------------

func TestParseTS(t *testing.T) {
	if parseTS("") != 0 {
		t.Error("empty -> 0")
	}
	if parseTS("abc") != 0 {
		t.Error("non-numeric -> 0")
	}
	if got := parseTS("1700000000.000100"); got <= 1700000000 || got >= 1700000001 {
		t.Errorf("parseTS = %v", got)
	}
}
