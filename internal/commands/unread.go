package commands

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
	internalOutput "github.com/natikgadzhi/slack-cli/internal/output"
	"github.com/natikgadzhi/slack-cli/internal/users"
)

// Unread "kinds" — why a message is a notification.
const (
	kindMention  = "mention"
	kindKeyword  = "keyword"
	kindInvite   = "invite"
	kindDM       = "dm"
	kindGroupDM  = "group_dm"
	kindThread   = "thread"
	kindReaction = "reaction"
	kindApp      = "app"
)

// activity.feed item types (the Slack Activity/bell tab).
var mentionActivityTypes = []string{"at_user", "at_user_group", "at_channel", "at_everyone"}

// channelInviteTypes announce being invited/added to a channel. The invited
// channel and inviter live in item.invite_info.{channel_id, inviter_user_id}.
// (generic_system_alert is intentionally excluded — it carries admin events like
// "channel archived", not invitations.)
var channelInviteTypes = []string{"internal_channel_invite", "external_channel_invite"}

const (
	// activityTypeKeyword is a highlight-word ("My keywords") notification —
	// a message containing a word you asked Slack to alert you on. Same payload
	// shape as an @-mention.
	activityTypeKeyword = "keyword"
	// activityTypeReaction is requested only with --include-reactions. Bot/app
	// DMs are not read from the activity feed — they come through the IMs path
	// (see collectUnread).
	activityTypeReaction = "message_reaction"
)

// Page-size limits for the upstream endpoints.
const (
	activityFeedMaxPage = 50  // activity.feed caps a page at 50 items
	historyPageCap      = 100 // max messages fetched per DM/group-DM page
	threadViewPageSize  = 8   // subscriptions.thread.getView threads per page
	threadRepliesLimit  = 200 // conversations.replies messages per thread
)

var unreadCmd = &cobra.Command{
	Use:   "unread",
	Short: "List unread mentions, DMs, and thread replies",
	Long: `List messages in your unread queue that you'd be notified about — channel
@-mentions (including @user-group, @here, @channel, @everyone), keyword highlights,
channel invitations, unread 1:1 DMs and group DMs, and unread replies in threads you
follow. Unlike a raw unread feed, this does not return every unread message in every
channel — only the ones worth a ping.

Reactions to your messages and bot/app DMs are excluded by default; opt in with
--include-reactions and --include-apps.

Reading is non-destructive: running this command never marks anything as read.`,
	Args: cobra.NoArgs,
	Example: `  slack-cli unread
  slack-cli unread --limit 100
  slack-cli unread --include-reactions --include-apps
  slack-cli unread -o json | jq '.[] | {kind, conversation, text}'`,
	RunE: runUnread,
}

func init() {
	unreadCmd.Flags().IntP("limit", "n", 50, "Maximum number of unread messages to return")
	unreadCmd.Flags().Bool("include-reactions", false, "Include reactions to your messages")
	unreadCmd.Flags().Bool("include-apps", false, "Include direct messages from bots and apps")
	rootCmd.AddCommand(unreadCmd)
}

// unreadItem is the normalized representation of one unread notification,
// regardless of which endpoint it came from.
type unreadItem struct {
	channelID string
	messageTS string
	threadTS  string // set for thread replies
	kind      string // mention | dm | group_dm | thread | reaction | app
	dateTS    float64
	reactor   string // for kind=reaction: who reacted (user id, later resolved)
	reaction  string // for kind=reaction: emoji shortcode name
	message   map[string]any
}

// unreadRow is the rendered shape we emit (JSON + table).
type unreadRow struct {
	Conversation    string `json:"conversation"`
	ConversationURL string `json:"conversation_url,omitempty"`
	Kind            string `json:"kind"`
	Date            string `json:"date"`
	Permalink       string `json:"permalink,omitempty"`
	User            string `json:"user,omitempty"`
	Text            string `json:"text"`
	ThreadTS        string `json:"thread_ts,omitempty"`
}

func runUnread(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 {
		return fmt.Errorf("--limit must be at least 1")
	}
	includeReactions, _ := cmd.Flags().GetBool("include-reactions")
	includeApps, _ := cmd.Flags().GetBool("include-apps")

	format := output.Resolve(cmd)

	client, userResolver, err := setupClient()
	if err != nil {
		return err
	}

	// Fetch the team URL concurrently with the unread fetches.
	teamCh := fetchTeamURLAsync(client)

	prog := progress.NewCounter("Fetching unread", format)
	items, isPartial, err := collectUnread(client, activityTypesFor(includeReactions), includeApps, limit, prog)
	prog.Finish()

	if err != nil && !isPartial {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching unread messages: %w", err)
	}

	items = dedupeUnread(items)

	if len(items) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no unread mentions or DMs found")
		}
		return nil
	}

	// Reverse-chronological by message timestamp (most recent first).
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].dateTS > items[j].dateTS
	})
	if len(items) > limit {
		items = items[:limit]
	}

	// Hydrate messages that came back without text (e.g. bare activity refs).
	hydrateUnreadMessages(client, items)

	// Resolve channel IDs to display names / DM partners / mpim participants.
	chanLookup := resolveUnreadChannels(client, userResolver, items)

	// Resolve user IDs inside message bodies (and reaction authors).
	resolveUnreadUsers(userResolver, items, format)

	teamRes := <-teamCh
	teamURL := teamRes.url
	hasTeamURL := teamRes.err == nil
	if teamRes.err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamRes.err)
	}

	rows, formattedByChannel := buildUnreadRows(items, chanLookup, userResolver, teamURL, hasTeamURL)

	if output.IsJSON(format) {
		if isPartial {
			pr := clierrors.NewPartialResult(rows, "rate limited: results may be incomplete")
			if err := output.PrintJSON(pr); err != nil {
				return err
			}
		} else {
			if err := output.PrintJSON(rows); err != nil {
				return err
			}
		}
	} else {
		renderUnreadTable(rows, hasTeamURL)
	}

	cacheWrite(getCache(), "unread", fmt.Sprintf("latest-%d", limit), rows, cache.Metadata{
		Command: fmt.Sprintf("unread --limit %d", limit),
	})

	// Write per-item markdown files if --derived was explicitly set (parity with
	// `channels get` / `search`). Group by channel so each file lands under its
	// conversation directory.
	if derivedDir := resolveDerivedDir(cmd); derivedDir != "" {
		if err := writeUnreadDerived(derivedDir, formattedByChannel, chanLookup); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d unread messages fetched (partial — rate limited).\n", len(rows))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d unread messages fetched.\n", len(rows))
		}
	}

	return nil
}

// collectUnread gathers unread notifications from all sources: activity.feed
// (mentions, optionally reactions/app DMs), subscriptions.thread.getView (thread
// replies), and client.counts + conversations.history (1:1 and group DMs).
//
// isPartial is true when a rate limit stopped collection mid-way but we still
// have some results to show.
func collectUnread(client *api.Client, activityTypes []string, includeApps bool, limit int, prog progress.Indicator) (items []unreadItem, isPartial bool, err error) {
	// 1. Mentions (+ optional reactions) from the Activity feed.
	activity, partial, aerr := fetchActivityItems(client, activityTypes, limit)
	items = append(items, activity...)
	prog.Update(len(items))
	if aerr != nil {
		if stop, p, e := bailOnRateLimit(aerr, items); stop {
			return items, p, e
		}
		return items, false, aerr // non-rate-limit: mentions are core, fail hard
	}
	isPartial = isPartial || partial

	// 2. Unread thread replies. The thread view is undocumented and best-effort:
	// a non-rate-limit failure warns and we continue without it.
	threads, terr := fetchUnreadThreads(client, limit)
	if terr != nil {
		if stop, p, e := bailOnRateLimit(terr, items); stop {
			return items, p, e
		}
		fmt.Fprintf(os.Stderr, "warning: fetching threads: %v\n", terr)
	}
	items = append(items, threads...)
	prog.Update(len(items))

	// 3. Unread 1:1 DMs and group DMs from client.counts + conversations.history.
	ims, mpims, cerr := fetchUnreadCounts(client)
	if cerr != nil {
		if stop, p, e := bailOnRateLimit(cerr, items); stop {
			return items, p, e
		}
		return items, false, cerr // non-rate-limit: DMs are core, fail hard
	}

	pageLimit := limit
	if pageLimit > historyPageCap {
		pageLimit = historyPageCap
	}

	for _, entries := range []struct {
		list []countEntry
		kind string
	}{{ims, kindDM}, {mpims, kindGroupDM}} {
		for _, e := range entries.list {
			if !e.hasUnreads {
				continue
			}
			msgs, herr := fetchConversationUnread(client, e.id, e.lastRead, pageLimit)
			if herr != nil {
				if stop, p, e := bailOnRateLimit(herr, items); stop {
					return items, p, e
				}
				// A single conversation failing is non-fatal: warn and skip it.
				fmt.Fprintf(os.Stderr, "warning: history %s: %v\n", e.id, herr)
				continue
			}
			for _, m := range msgs {
				kind := entries.kind
				// A bot/app-authored message in a DM is an "app DM": only
				// surface it when the user opted in, and tag it as such.
				if isBotMessage(m) {
					if !includeApps {
						continue
					}
					kind = kindApp
				}
				items = append(items, newUnreadItem(e.id, "", kind, m))
			}
			prog.Update(len(items))
		}
	}

	// 4. Expand threads. A mention/reaction inside a thread arrives as a single
	// message from the activity feed; the rest of the thread is not in the feed
	// (and may not be in the subscribed-thread view either), so fetch it via
	// conversations.replies. Thread items from the subscribed view (kind=thread)
	// are skipped here — they already carry their replies.
	expanded := map[string]bool{}
	var threadItems []unreadItem
	for _, it := range items {
		if it.threadTS == "" || it.kind == kindThread {
			continue
		}
		key := it.channelID + "\x00" + it.threadTS
		if expanded[key] {
			continue
		}
		expanded[key] = true
		replies, rerr := fetchThreadUnread(client, it.channelID, it.threadTS)
		if rerr != nil {
			if stop, p, rlErr := bailOnRateLimit(rerr, items); stop {
				return items, p, rlErr
			}
			fmt.Fprintf(os.Stderr, "warning: thread %s/%s: %v\n", it.channelID, it.threadTS, rerr)
			continue
		}
		threadItems = append(threadItems, replies...)
	}
	items = append(items, threadItems...)
	prog.Update(len(items))

	return items, isPartial, nil
}

// bailOnRateLimit centralizes how collection reacts to a rate-limit error.
// When err is a rate limit, collection stops: it returns a partial result if
// anything was gathered, or surfaces the error if nothing was. For any other
// error it returns stop=false and the caller decides (fatal vs. best-effort).
func bailOnRateLimit(err error, items []unreadItem) (stop, isPartial bool, retErr error) {
	if _, ok := api.AsRateLimitError(err); !ok {
		return false, false, nil
	}
	if len(items) > 0 {
		return true, true, nil
	}
	return true, false, err
}

// newUnreadItem builds an unreadItem from a raw message map, deriving the
// sortable timestamp from the message ts.
func newUnreadItem(channelID, threadTS, kind string, message map[string]any) unreadItem {
	ts := getString(message, "ts")
	return unreadItem{
		channelID: channelID,
		messageTS: ts,
		threadTS:  threadTS,
		kind:      kind,
		dateTS:    parseTS(ts),
		message:   message,
	}
}

// dedupeUnread removes duplicate items keyed by (channelID, messageTS). A thread
// reply that also @-mentions you can be reported by both activity.feed and
// subscriptions.thread.getView; we keep the first occurrence.
func dedupeUnread(items []unreadItem) []unreadItem {
	seen := make(map[string]struct{}, len(items))
	out := items[:0]
	for _, it := range items {
		key := it.channelID + "\x00" + it.messageTS
		if it.messageTS == "" {
			out = append(out, it)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, it)
	}
	return out
}

// --- activity.feed ----------------------------------------------------------

// activityTypesFor returns the activity.feed item types to request. Mentions,
// keyword highlights, and channel invitations are always included (they're all
// notifications); reactions are added only when the user opts in. Bot/app DMs
// are deliberately omitted — they arrive via the IMs path (see collectUnread).
func activityTypesFor(includeReactions bool) []string {
	types := append([]string{}, mentionActivityTypes...)
	types = append(types, activityTypeKeyword)
	types = append(types, channelInviteTypes...)
	if includeReactions {
		types = append(types, activityTypeReaction)
	}
	return types
}

// fetchActivityItems pages through activity.feed (unread only) until it reaches
// `limit` items or runs out. partial signals a mid-pagination rate limit with
// usable results.
func fetchActivityItems(client *api.Client, types []string, limit int) (items []unreadItem, partial bool, err error) {
	pageSize := limit
	if pageSize > activityFeedMaxPage {
		pageSize = activityFeedMaxPage
	}
	params := map[string]string{
		"mode":              "priority_reads_and_unreads_v1",
		"limit":             strconv.Itoa(pageSize),
		"unread_only":       "true",
		"archive_only":      "false",
		"snooze_only":       "false",
		"priority_only":     "false",
		"is_activity_inbox": "false",
		"types":             strings.Join(types, ","),
	}

	for {
		result, callErr := client.Call("activity.feed", params)
		if callErr != nil {
			if _, ok := api.AsRateLimitError(callErr); ok && len(items) > 0 {
				return items, true, nil
			}
			return items, false, callErr
		}

		for _, raw := range api.ExtractItems(result, "items") {
			if it, ok := parseActivityItem(raw); ok {
				items = append(items, it)
			}
		}

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(items) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	return items, false, nil
}

// parseActivityItem normalizes a single activity.feed entry into an unreadItem.
// Returns ok=false for entries we can't place (unknown type, missing channel/ts).
func parseActivityItem(raw map[string]any) (unreadItem, bool) {
	inner, ok := raw["item"].(map[string]any)
	if !ok {
		return unreadItem{}, false
	}
	itemType := getString(inner, "type")
	feedTS := getString(raw, "feed_ts")

	switch {
	case slices.Contains(mentionActivityTypes, itemType):
		msg, _ := inner["message"].(map[string]any)
		return activityMessageItem(msg, kindMention, feedTS)

	case itemType == activityTypeKeyword:
		// A keyword highlight is a normal message that happens to contain one of
		// your highlight words — same shape as an @-mention.
		msg, _ := inner["message"].(map[string]any)
		return activityMessageItem(msg, kindKeyword, feedTS)

	case slices.Contains(channelInviteTypes, itemType):
		return inviteActivityItem(inner, feedTS)

	case itemType == activityTypeReaction:
		msg, _ := inner["message"].(map[string]any)
		it, ok := activityMessageItem(msg, kindReaction, feedTS)
		if !ok {
			return it, false
		}
		if r, ok := inner["reaction"].(map[string]any); ok {
			it.reactor = getString(r, "user")
			it.reaction = getString(r, "name")
		}
		return it, true

	default:
		return unreadItem{}, false
	}
}

// inviteActivityItem builds an unread item for a channel invitation. The invited
// channel and inviter come from item.invite_info.{channel_id, inviter_user_id};
// item.message.channel is a fallback. We only emit when we find a channel-like
// ID. The inviter is stashed on the message's "user" field so it resolves to a
// display name like any other author.
func inviteActivityItem(inner map[string]any, feedTS string) (unreadItem, bool) {
	channel, inviter := "", ""
	if info, ok := inner["invite_info"].(map[string]any); ok {
		channel = getString(info, "channel_id")
		inviter = getString(info, "inviter_user_id")
	}
	if channel == "" {
		if msg, ok := inner["message"].(map[string]any); ok {
			channel = getString(msg, "channel")
		}
	}
	if channel == "" {
		channel = getString(inner, "channel")
	}
	if !strings.HasPrefix(channel, "C") && !strings.HasPrefix(channel, "G") {
		return unreadItem{}, false
	}

	message := map[string]any{}
	if inviter != "" {
		message["user"] = inviter
	}
	return unreadItem{
		channelID: channel,
		kind:      kindInvite,
		dateTS:    parseTS(feedTS),
		message:   message,
	}, true
}

// activityMessageItem builds an unreadItem from an activity message reference.
// A message carrying a thread_ts belongs to a thread; we record it so the rest
// of the thread can be fetched (collectUnread) and so the table groups it.
func activityMessageItem(msg map[string]any, kind, feedTS string) (unreadItem, bool) {
	if msg == nil {
		return unreadItem{}, false
	}
	channel := getString(msg, "channel")
	ts := getString(msg, "ts")
	if channel == "" || ts == "" {
		return unreadItem{}, false
	}
	// activity.feed message refs use author_user_id; normalize to "user" so the
	// shared formatter and resolver pick it up.
	if _, has := msg["user"]; !has {
		if author := getString(msg, "author_user_id"); author != "" {
			msg["user"] = author
		}
	}
	// Sort by the activity (feed) time — for a reaction this is when someone
	// reacted, which is the notification moment, not the original message ts.
	date := parseTS(feedTS)
	if date == 0 {
		date = parseTS(ts)
	}
	return unreadItem{
		channelID: channel,
		messageTS: ts,
		threadTS:  getString(msg, "thread_ts"),
		kind:      kind,
		dateTS:    date,
		message:   msg,
	}, true
}

// --- subscriptions.thread.getView ------------------------------------------

// fetchUnreadThreads returns one unreadItem per unread thread reply.
func fetchUnreadThreads(client *api.Client, limit int) ([]unreadItem, error) {
	result, err := client.Call("subscriptions.thread.getView", map[string]string{
		"limit":          strconv.Itoa(threadViewPageSize),
		"org_wide_aware": "true",
	})
	if err != nil {
		return nil, err
	}

	var items []unreadItem
	for _, thread := range api.ExtractItems(result, "threads") {
		root, _ := thread["root_msg"].(map[string]any)
		channel := getString(root, "channel")
		threadTS := getString(root, "ts")

		replies, _ := thread["unread_replies"].([]any)
		for _, r := range replies {
			reply, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if channel != "" {
				reply["channel"] = channel
			}
			it := newUnreadItem(channel, threadTS, kindThread, reply)
			items = append(items, it)
			if len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

// fetchThreadUnread fetches a thread via conversations.replies and returns its
// unread messages as thread items. "Unread" means newer than the viewer's
// last_read marker for the thread, which Slack returns on the parent (first)
// message; when it's absent we return every reply (best-effort context).
func fetchThreadUnread(client *api.Client, channel, threadTS string) ([]unreadItem, error) {
	result, err := client.Call("conversations.replies", map[string]string{
		"channel": channel,
		"ts":      threadTS,
		"limit":   strconv.Itoa(threadRepliesLimit),
	})
	if err != nil {
		return nil, err
	}
	msgs := api.ExtractItems(result, "messages")
	if len(msgs) == 0 {
		return nil, nil
	}
	lastReadF := parseTS(getString(msgs[0], "last_read"))

	items := make([]unreadItem, 0, len(msgs))
	for _, m := range msgs {
		if lastReadF > 0 && parseTS(getString(m, "ts")) <= lastReadF {
			continue // already read
		}
		if m["channel"] == nil {
			m["channel"] = channel
		}
		items = append(items, newUnreadItem(channel, threadTS, kindThread, m))
	}
	return items, nil
}

// --- client.counts + conversations.history (DMs and group DMs) --------------

// countEntry is the subset of a client.counts conversation we use.
type countEntry struct {
	id         string
	hasUnreads bool
	lastRead   string
}

// fetchUnreadCounts calls client.counts and returns the unread 1:1 DMs (ims)
// and group DMs (mpims). Channel mentions are intentionally ignored here — they
// come from activity.feed.
func fetchUnreadCounts(client *api.Client) (ims, mpims []countEntry, err error) {
	result, err := client.Call("client.counts", map[string]string{
		"thread_counts_by_channel": "true",
		"org_wide_aware":           "true",
		"include_file_channels":    "true",
	})
	if err != nil {
		return nil, nil, err
	}
	return parseCountEntries(result, "ims"), parseCountEntries(result, "mpims"), nil
}

// parseCountEntries extracts the conversation list under key from a client.counts
// response.
func parseCountEntries(result map[string]any, key string) []countEntry {
	raw := api.ExtractItems(result, key)
	out := make([]countEntry, 0, len(raw))
	for _, r := range raw {
		id := getString(r, "id")
		if id == "" {
			continue
		}
		has, _ := r["has_unreads"].(bool)
		out = append(out, countEntry{
			id:         id,
			hasUnreads: has,
			lastRead:   getString(r, "last_read"),
		})
	}
	return out
}

// fetchConversationUnread fetches one page of history for a conversation and
// returns the messages strictly newer than lastRead (the message that equals
// lastRead has already been read).
func fetchConversationUnread(client *api.Client, channelID, lastRead string, limit int) ([]map[string]any, error) {
	params := map[string]string{
		"channel":   channelID,
		"limit":     strconv.Itoa(limit),
		"inclusive": "true",
	}
	// Only send `oldest` when it's a real timestamp. Never-opened conversations
	// report last_read as "0000000000.000000", which Slack rejects with
	// invalid_ts_oldest; in that case we fetch the recent page and treat it all
	// as unread.
	if parseTS(lastRead) > 0 {
		params["oldest"] = lastRead
	}
	result, err := client.Call("conversations.history", params)
	if err != nil {
		return nil, err
	}

	lastReadF := parseTS(lastRead)
	msgs := api.ExtractItems(result, "messages")
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		ts := getString(m, "ts")
		if lastReadF > 0 && parseTS(ts) <= lastReadF {
			continue // already read (boundary message or older)
		}
		if m["channel"] == nil {
			m["channel"] = channelID
		}
		out = append(out, m)
	}
	return out, nil
}

// isBotMessage reports whether a message was authored by a bot or app, rather
// than a human. Used to gate app/bot DMs behind --include-apps.
func isBotMessage(m map[string]any) bool {
	if m == nil {
		return false
	}
	if getString(m, "bot_id") != "" {
		return true
	}
	if getString(m, "app_id") != "" {
		return true
	}
	if getString(m, "subtype") == "bot_message" {
		return true
	}
	if _, ok := m["bot_profile"].(map[string]any); ok {
		return true
	}
	return false
}

// --- hydration / resolution -------------------------------------------------

// hydrateUnreadMessages fills in message bodies that came back as bare
// references (e.g. some activity.feed items) by reusing the saved command's
// per-message conversations.history hydration.
func hydrateUnreadMessages(client *api.Client, items []unreadItem) {
	saved := make([]savedItem, 0, len(items))
	idx := make([]int, 0, len(items))
	for i := range items {
		// Items without a message timestamp (e.g. channel invitations) have no
		// message to hydrate — skip them so we don't fetch and clobber them with
		// an unrelated recent message from the channel.
		if items[i].messageTS == "" {
			continue
		}
		saved = append(saved, savedItem{
			channelID: items[i].channelID,
			messageTS: items[i].messageTS,
			message:   items[i].message,
		})
		idx = append(idx, i)
	}
	hydrateSavedMessages(client, saved)
	for j, i := range idx {
		items[i].message = saved[j].message
	}
}

// resolveUnreadChannels resolves every unique channel ID to a display name,
// reusing the saved command's fetchSavedChannel (handles DM → "@user",
// MPIM → participant list, channel → name).
func resolveUnreadChannels(client *api.Client, userResolver *users.UserResolver, items []unreadItem) savedChannelMap {
	seen := make(savedChannelMap)
	for _, it := range items {
		if it.channelID == "" {
			continue
		}
		if _, ok := seen[it.channelID]; ok {
			continue
		}
		seen[it.channelID] = fetchSavedChannel(client, userResolver, it.channelID)
	}
	return seen
}

// resolveUnreadUsers replaces user IDs in message bodies with display names and
// resolves reaction authors. Failures warn but are non-fatal.
func resolveUnreadUsers(userResolver *users.UserResolver, items []unreadItem, format string) {
	rawMessages := make([]map[string]any, len(items))
	for i := range items {
		rawMessages[i] = items[i].message
	}
	resolved, err := userResolver.ResolveUsers(rawMessages)
	if err != nil {
		if !output.IsJSON(format) {
			fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
		}
		return
	}
	for i := range items {
		items[i].message = resolved[i]
		if items[i].reactor != "" {
			if name := userResolver.DisplayName(items[i].reactor); name != "" {
				items[i].reactor = name
			}
		}
	}
}

// --- rendering --------------------------------------------------------------

// buildUnreadRows converts unreadItems into the final row shape. It also returns
// the formatted messages grouped by channel ID, for optional --derived output.
func buildUnreadRows(items []unreadItem, chans savedChannelMap, userResolver formatting.UserResolver, teamURL string, hasTeamURL bool) ([]unreadRow, map[string][]formatting.Message) {
	rows := make([]unreadRow, 0, len(items))
	byChannel := make(map[string][]formatting.Message)

	for _, it := range items {
		formatted := formatting.FormatMessageWith(it.message, userResolver, chans)

		conversation := chans.ChannelName(it.channelID)
		if conversation == "" {
			conversation = it.channelID
		}

		var conversationURL, permalink string
		if hasTeamURL {
			conversationURL = teamURL + "/archives/" + it.channelID
			if it.messageTS != "" {
				permalink = formatting.BuildPermalink(teamURL, it.channelID, it.messageTS)
			}
		}

		date := formatted.Time
		if date == "" {
			date = internalOutput.FormatTS(it.messageTS)
		}

		text := savedRowText(formatted)
		user := formatted.User
		switch it.kind {
		case kindReaction:
			// The reactor is already shown in the text; the message author is
			// usually you, so omit the redundant "@you:" row prefix.
			text = reactionText(it, text)
			user = ""
		case kindInvite:
			// Invitations carry no message body; the conversation column names
			// the channel, and `user` (if any) is the inviter.
			text = "invited you to this channel"
		}

		rows = append(rows, unreadRow{
			Conversation:    conversation,
			ConversationURL: conversationURL,
			Kind:            it.kind,
			Date:            date,
			Permalink:       permalink,
			User:            user,
			Text:            text,
			ThreadTS:        it.threadTS,
		})

		if it.messageTS != "" {
			formatted.TS = it.messageTS
			formatted.Link = permalink
			byChannel[it.channelID] = append(byChannel[it.channelID], formatted)
		}
	}
	return rows, byChannel
}

// reactionText prefixes a reaction row with the emoji and reactor.
func reactionText(it unreadItem, base string) string {
	emoji := formatting.ReplaceEmojiShortcodes(":" + it.reaction + ":")
	who := it.reactor
	prefix := emoji
	if who != "" {
		prefix = emoji + " from @" + who
	}
	if base == "" {
		return prefix
	}
	return prefix + ": " + base
}

// renderUnreadTable writes the unread rows to stdout as a bordered table. The
// conversation column links to the channel; the date column links to the
// message permalink (both OSC-8 hyperlinks, like the saved command).
// maxConversationCell bounds the visible width of the conversation column. We
// truncate the name ourselves and wrap the *already-truncated* text in the
// hyperlink so the table never has to clip it — cli-kit's table strips ANSI
// (and thus the OSC-8 link) when it truncates a cell, which would otherwise
// make long conversation names unclickable.
const maxConversationCell = 30

func renderUnreadTable(rows []unreadRow, hasTeamURL bool) {
	t := table.New()
	t.Header("CONVERSATION", "DATE", "MESSAGE")

	// A conversation stream (a thread, or a DM/group/app DM) can have several
	// unread messages, each its own row. The table collapses them to one line:
	// the latest message plus a "[+N]" badge for the others. Rows arrive
	// newest-first, so the first row seen for a stream is its latest message;
	// later ones are counted and skipped. (JSON/markdown keep every message.)
	count := map[string]int{}
	for _, r := range rows {
		if k := collapseKey(r); k != "" {
			count[k]++
		}
	}
	shown := map[string]bool{}

	for _, r := range rows {
		key := collapseKey(r)
		if key != "" {
			if shown[key] {
				continue // an earlier message in a stream already on the table
			}
			shown[key] = true
		}

		name := table.Truncate(r.Conversation, maxConversationCell)
		convCell := name
		if hasTeamURL && r.ConversationURL != "" {
			convCell = table.Hyperlink(r.ConversationURL, name)
		}
		dateCell := r.Date
		if hasTeamURL && r.Permalink != "" && r.Date != "" {
			dateCell = table.Hyperlink(r.Permalink, r.Date)
		}
		text := singleLine(r.Text)
		if r.User != "" {
			text = "@" + r.User + ": " + text
		}
		if key != "" {
			if extra := count[key] - 1; extra > 0 {
				text = fmt.Sprintf("[+%d] %s", extra, text)
			}
		}
		t.Row(convCell, dateCell, text)
	}
	_ = t.Flush()
}

// collapseKey returns the key used to collapse multiple unread messages into a
// single table line, or "" if the row should stay on its own. Conversations
// that form one continuous stream collapse: a thread (by thread_ts) and a DM /
// group DM / app DM (by conversation). Discrete pings — channel mentions and
// reactions — are left as individual rows. ConversationURL embeds the channel
// ID (a stable identifier); the display name is the fallback when there's no
// team URL.
func collapseKey(r unreadRow) string {
	conv := r.ConversationURL
	if conv == "" {
		conv = r.Conversation
	}
	if r.ThreadTS != "" {
		return "thread\x00" + conv + "\x00" + r.ThreadTS
	}
	switch r.Kind {
	case kindDM, kindGroupDM, kindApp:
		return "conv\x00" + conv
	}
	return ""
}

// writeUnreadDerived writes each unread message as its own markdown file under
// <derivedDir>/slack/channels/<context>/<ts>.md, grouped by channel. Reuses the
// shared writeItemFiles helper (parity with `channels get` / `search`).
func writeUnreadDerived(derivedDir string, byChannel map[string][]formatting.Message, chans savedChannelMap) error {
	for channelID, msgs := range byChannel {
		name := chans.ChannelName(channelID)
		if err := writeItemFiles(derivedDir, msgs, channelID, name); err != nil {
			return err
		}
	}
	return nil
}

// parseTS parses a Slack timestamp string (e.g. "1700000000.000100") into a
// float for comparison/sorting. Returns 0 on failure or empty input.
func parseTS(ts string) float64 {
	if ts == "" {
		return 0
	}
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return 0
	}
	return f
}
