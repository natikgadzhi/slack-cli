package commands

import (
	"fmt"
	"os"
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

var savedCmd = &cobra.Command{
	Use:   "saved",
	Short: "List saved messages",
	Long: `List messages the user has saved for later, sorted in reverse-chronological order
by when they were saved.`,
	Args: cobra.NoArgs,
	Example: `  slack-cli saved
  slack-cli saved --limit 100
  slack-cli saved -o json | jq '.[].text'`,
	RunE: runSaved,
}

func init() {
	savedCmd.Flags().IntP("limit", "n", 50, "Maximum number of saved messages to return")
	rootCmd.AddCommand(savedCmd)
}

// savedItem is the normalized representation of one entry from saved.list.
type savedItem struct {
	id          string
	dateCreated int64  // unix seconds — when the message was saved
	channelID   string // channel the saved message lives in
	messageTS   string // Slack message timestamp
	message     map[string]any
}

// savedRow is the rendered shape we emit (JSON + table).
type savedRow struct {
	Conversation    string `json:"conversation"` // channel name or mpim participants
	ConversationURL string `json:"conversation_url,omitempty"`
	Date            string `json:"date"` // "02 Jan 2006 15:04"
	Permalink       string `json:"permalink,omitempty"`
	User            string `json:"user,omitempty"`
	Text            string `json:"text"`
	SavedAt         int64  `json:"saved_at,omitempty"`
}

func runSaved(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 {
		return fmt.Errorf("--limit must be at least 1")
	}

	format := output.Resolve(cmd)

	client, userResolver, err := setupClient()
	if err != nil {
		return err
	}

	// Fetch workspace URL in parallel with the saved.list call.
	teamCh := fetchTeamURLAsync(client)

	prog := progress.NewCounter("Fetching saved messages", format)
	items, isPartial, err := fetchSavedItems(client, limit, prog)
	prog.Finish()

	if err != nil && !isPartial {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching saved messages: %w", err)
	}

	if len(items) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no saved messages found")
		}
		return nil
	}

	// Reverse-chronological by save date (most recently saved first).
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].dateCreated > items[j].dateCreated
	})
	if len(items) > limit {
		items = items[:limit]
	}

	// Hydrate messages that came back as bare references.
	hydrateSavedMessages(client, items)

	// Resolve channel IDs to display names / mpim participant lists.
	chanLookup := resolveSavedChannels(client, userResolver, items)

	// Resolve user IDs inside message bodies (replaces "user" and caches IDs).
	rawMessages := make([]map[string]any, 0, len(items))
	for _, it := range items {
		rawMessages = append(rawMessages, it.message)
	}
	resolved, err := userResolver.ResolveUsers(rawMessages)
	if err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}
	if err == nil {
		for i := range items {
			items[i].message = resolved[i]
		}
	}

	teamRes := <-teamCh
	teamURL := teamRes.url
	hasTeamURL := teamRes.err == nil
	if teamRes.err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamRes.err)
	}

	rows := buildSavedRows(items, chanLookup, userResolver, teamURL, hasTeamURL)

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
		renderSavedTable(rows, hasTeamURL)
	}

	cacheWrite(getCache(), "saved", fmt.Sprintf("latest-%d", limit), rows, cache.Metadata{
		Command: fmt.Sprintf("saved --limit %d", limit),
	})

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d saved messages fetched (partial — rate limited).\n", len(rows))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d saved messages fetched.\n", len(rows))
		}
	}

	return nil
}

// fetchSavedItems pages through saved.list until we reach `limit` or run out.
// isPartial indicates a rate-limit stopped pagination mid-way.
func fetchSavedItems(client *api.Client, limit int, prog progress.Indicator) (items []savedItem, isPartial bool, err error) {
	pageSize := limit
	if pageSize > 200 {
		pageSize = 200
	}
	params := map[string]string{"limit": strconv.Itoa(pageSize)}

	for {
		prog.Update(len(items))

		result, callErr := client.Call("saved.list", params)
		if callErr != nil {
			if _, ok := api.AsRateLimitError(callErr); ok && len(items) > 0 {
				return items, true, nil
			}
			return items, false, callErr
		}

		batch := extractSavedItems(result)
		items = append(items, batch...)

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(items) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	return items, false, nil
}

// extractSavedItems handles either the documented "saved_items" key or the
// older "items" key (used by stars.list). Each element may be:
//
//	{ "item": { "type": "message", "message": {...}, "channel": "C…" } }
//	{ "item_type": "message", "channel": "C…", "message": {...} }
//	{ "type": "message", "channel": "C…", "message": {...} }
//
// Everything that isn't a message is skipped.
func extractSavedItems(result map[string]any) []savedItem {
	raw := api.ExtractItems(result, "saved_items")
	if len(raw) == 0 {
		raw = api.ExtractItems(result, "items")
	}

	out := make([]savedItem, 0, len(raw))
	for _, r := range raw {
		if archived, _ := r["is_archived"].(bool); archived {
			continue
		}
		si := parseSavedItem(r)
		if si.messageTS == "" || si.channelID == "" {
			continue
		}
		out = append(out, si)
	}
	return out
}

// parseSavedItem normalizes a single saved.list entry into a savedItem.
// The live Slack shape (March 2026) is:
//
//	{
//	  "item_type":    "message",
//	  "item_id":      "C0123ABCD"  // channel ID (C… / D… / G…)
//	  "ts":           "1700000000.000000"
//	  "date_created": 1706000000,
//	  "is_archived":  false,
//	  "state":        "in_progress",
//	  ...
//	}
//
// We also accept two older shapes that embedded the message inline:
//
//	{ "item": { "type": "message", "message": {...}, "channel": "C…" } }
//	{ "type": "message", "channel": "C…", "message": {...} }   // stars.list
func parseSavedItem(r map[string]any) savedItem {
	si := savedItem{
		id:          getString(r, "id"),
		dateCreated: getInt64(r, "date_created"),
	}

	// Inner reference may live under "item" (nested shape) or directly on the
	// outer object (flat shape).
	ref := r
	if inner, ok := r["item"].(map[string]any); ok {
		ref = inner
	}

	itemType := strings.ToLower(getString(ref, "type"))
	if itemType == "" {
		itemType = strings.ToLower(getString(r, "item_type"))
	}
	if itemType != "" && itemType != "message" {
		return savedItem{} // non-message; caller filters out
	}

	if m, ok := ref["message"].(map[string]any); ok {
		si.message = m
		si.messageTS = getString(m, "ts")
		si.channelID = getString(m, "channel")
	}
	if si.message == nil {
		si.message = map[string]any{}
	}

	// Channel ID: look at "channel" (legacy shapes) and then "item_id" (live
	// shape), where item_id is literally the channel ID.
	if si.channelID == "" {
		si.channelID = getString(ref, "channel")
	}
	if si.channelID == "" {
		si.channelID = getString(r, "channel")
	}
	if si.channelID == "" {
		si.channelID = getString(r, "item_id")
	}

	if si.messageTS == "" {
		si.messageTS = getString(ref, "ts")
	}
	if si.messageTS == "" {
		si.messageTS = getString(r, "ts")
	}

	// If dateCreated is missing, fall back to the message timestamp.
	if si.dateCreated == 0 && si.messageTS != "" {
		if f, err := strconv.ParseFloat(si.messageTS, 64); err == nil {
			si.dateCreated = int64(f)
		}
	}

	return si
}

// hydrateSavedMessages fills in missing message bodies by calling
// conversations.history for each (channel, ts) that has no text yet.
// Failures warn to stderr but are non-fatal — the entry will fall back to
// the saved-item timestamp for its date and render with empty text.
func hydrateSavedMessages(client *api.Client, items []savedItem) {
	for i := range items {
		if hasMessageText(items[i].message) {
			continue
		}
		params := map[string]string{
			"channel":   items[i].channelID,
			"latest":    items[i].messageTS,
			"oldest":    items[i].messageTS,
			"inclusive": "true",
			"limit":     "1",
		}
		result, err := client.Call("conversations.history", params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: hydrate %s/%s: %v\n", items[i].channelID, items[i].messageTS, err)
			continue
		}
		msgs := api.ExtractItems(result, "messages")
		if len(msgs) > 0 {
			items[i].message = msgs[0]
		}
	}
}

func hasMessageText(m map[string]any) bool {
	if m == nil {
		return false
	}
	if s, _ := m["text"].(string); strings.TrimSpace(s) != "" {
		return true
	}
	if atts, ok := m["attachments"].([]any); ok && len(atts) > 0 {
		return true
	}
	if blocks, ok := m["blocks"].([]any); ok && len(blocks) > 0 {
		return true
	}
	return false
}

// --- channel resolution -----------------------------------------------------

// savedChannel is the cached metadata we've resolved for a single channel ID.
type savedChannel struct {
	id          string
	displayName string // user-facing "conversation name"
	isMpim      bool
	isIM        bool
	isPrivate   bool
}

// savedChannelMap implements formatting.ChannelResolver.
type savedChannelMap map[string]*savedChannel

func (m savedChannelMap) ChannelName(id string) string {
	if c, ok := m[id]; ok && c != nil {
		return c.displayName
	}
	return ""
}

// resolveSavedChannels calls conversations.info for each unique channel ID,
// and for MPIMs builds a participant list. On errors it logs a warning and
// falls back to the channel ID as the name.
func resolveSavedChannels(client *api.Client, userResolver *users.UserResolver, items []savedItem) savedChannelMap {
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

func fetchSavedChannel(client *api.Client, userResolver *users.UserResolver, id string) *savedChannel {
	c := &savedChannel{id: id, displayName: id}
	result, err := client.Call("conversations.info", map[string]string{"channel": id})
	if err != nil {
		return c
	}
	ch, ok := result["channel"].(map[string]any)
	if !ok {
		return c
	}
	c.isMpim, _ = ch["is_mpim"].(bool)
	c.isIM, _ = ch["is_im"].(bool)
	c.isPrivate, _ = ch["is_private"].(bool)

	switch {
	case c.isMpim:
		c.displayName = mpimParticipantsName(client, userResolver, id, ch)
	case c.isIM:
		if uid, _ := ch["user"].(string); uid != "" {
			name := userResolver.DisplayName(uid)
			if name == "" {
				name = uid
			}
			c.displayName = "@" + name
		}
	default:
		if name, _ := ch["name"].(string); name != "" {
			c.displayName = name
		}
	}
	return c
}

// mpimParticipantsName builds a comma-separated list of participant names for
// an MPIM channel. Slack names mpims like "mpdm-alice--bob--charlie-1", which
// we treat only as a last-resort fallback.
func mpimParticipantsName(client *api.Client, userResolver *users.UserResolver, channelID string, ch map[string]any) string {
	memberIDs := memberIDsFromChannelInfo(ch)
	if len(memberIDs) == 0 {
		memberIDs = fetchMpimMembers(client, channelID)
	}

	names := make([]string, 0, len(memberIDs))
	for _, uid := range memberIDs {
		name := userResolver.DisplayName(uid)
		if name == "" {
			name = uid
		}
		names = append(names, name)
	}
	if len(names) > 0 {
		return strings.Join(names, ", ")
	}
	if raw, _ := ch["name"].(string); raw != "" {
		return raw
	}
	return channelID
}

func memberIDsFromChannelInfo(ch map[string]any) []string {
	raw, ok := ch["members"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func fetchMpimMembers(client *api.Client, channelID string) []string {
	result, err := client.Call("conversations.members", map[string]string{
		"channel": channelID,
		"limit":   "100",
	})
	if err != nil {
		return nil
	}
	raw, ok := result["members"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// --- rendering --------------------------------------------------------------

// buildSavedRows converts savedItems into the final row shape.
func buildSavedRows(items []savedItem, chans savedChannelMap, userResolver formatting.UserResolver, teamURL string, hasTeamURL bool) []savedRow {
	rows := make([]savedRow, 0, len(items))
	for _, it := range items {
		formatted := formatting.FormatMessageWith(it.message, userResolver, chans)

		conversation := chans.ChannelName(it.channelID)
		if conversation == "" {
			conversation = it.channelID
		}

		var conversationURL string
		var permalink string
		if hasTeamURL {
			conversationURL = teamURL + "/archives/" + it.channelID
			if it.messageTS != "" {
				permalink = formatting.BuildPermalink(teamURL, it.channelID, it.messageTS)
			}
		}

		// If hydration failed, FormatMessage leaves Time empty. Fall back to
		// the messageTS from saved.list itself so the date still renders and
		// the permalink remains clickable.
		date := formatted.Time
		if date == "" {
			date = internalOutput.FormatTS(it.messageTS)
		}

		rows = append(rows, savedRow{
			Conversation:    conversation,
			ConversationURL: conversationURL,
			Date:            date,
			Permalink:       permalink,
			User:            formatted.User,
			Text:            savedRowText(formatted),
			SavedAt:         it.dateCreated,
		})
	}
	return rows
}

// savedRowText returns the displayable text for a formatted message, falling
// back to attachment title/text when the message body is empty.
func savedRowText(m formatting.Message) string {
	if m.Text != "" {
		return m.Text
	}
	if m.Attachment == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if m.Attachment.Title != "" {
		parts = append(parts, m.Attachment.Title)
	}
	if m.Attachment.Text != "" {
		parts = append(parts, m.Attachment.Text)
	}
	return formatting.TruncateRunes(strings.Join(parts, " — "), 500)
}

// renderSavedTable writes the saved rows to stdout as a bordered table. The
// conversation column links to the channel on Slack; the date column links
// to the message permalink. Both links are OSC-8 hyperlinks, which modern
// terminals render as clickable without affecting column alignment.
func renderSavedTable(rows []savedRow, hasTeamURL bool) {
	t := table.New()
	t.Header("CONVERSATION", "DATE", "MESSAGE")
	for _, r := range rows {
		convCell := r.Conversation
		if hasTeamURL && r.ConversationURL != "" {
			convCell = table.Hyperlink(r.ConversationURL, r.Conversation)
		}
		dateCell := r.Date
		if hasTeamURL && r.Permalink != "" && r.Date != "" {
			dateCell = table.Hyperlink(r.Permalink, r.Date)
		}
		text := singleLine(r.Text)
		if r.User != "" {
			text = "@" + r.User + ": " + text
		}
		t.Row(convCell, dateCell, text)
	}
	_ = t.Flush()
}

// singleLine collapses newlines and runs of whitespace into single spaces so a
// message body fits on one table row without breaking the right border.
func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// getInt64 reads an integer from a JSON map. JSON numbers decode as float64.
func getInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}
