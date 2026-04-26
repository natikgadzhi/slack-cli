package formatting

import (
	"regexp"
	"strings"
)

// Slack mrkdwn wraps interactive references in angle brackets:
//
//	<@U012|alice>        user mention
//	<@U012>              user mention without label
//	<#C034|general>      channel mention
//	<#C034>              channel mention without label
//	<!subteam^S05|name>  usergroup
//	<!here>, <!channel>  broadcast keywords
//	<https://url|text>   link with label
//	<https://url>        bare link
//
// UserResolver maps a user ID to a display name.
type UserResolver interface {
	DisplayName(userID string) string
}

// ChannelResolver maps a channel ID to a channel name.
type ChannelResolver interface {
	ChannelName(channelID string) string
}

// mrkdwnPattern matches anything enclosed in <>.
var mrkdwnPattern = regexp.MustCompile(`<([^>]+)>`)

// ReplaceMrkdwnLinks rewrites Slack's angle-bracket references into
// human-readable text. Unknown user/channel IDs fall back to the raw ID.
//
// userLookup and channelLookup may be nil; if so, labels are used when present
// and IDs are left as-is otherwise.
func ReplaceMrkdwnLinks(s string, users UserResolver, channels ChannelResolver) string {
	if s == "" {
		return s
	}
	return mrkdwnPattern.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[1 : len(match)-1]
		return renderMrkdwnToken(inner, users, channels)
	})
}

func renderMrkdwnToken(inner string, users UserResolver, channels ChannelResolver) string {
	if inner == "" {
		return "<>"
	}
	body, label := splitLabel(inner)

	switch {
	case strings.HasPrefix(body, "@"):
		uid := body[1:]
		if label != "" {
			return "@" + label
		}
		if users != nil {
			if name := users.DisplayName(uid); name != "" {
				return "@" + name
			}
		}
		return "@" + uid
	case strings.HasPrefix(body, "#"):
		cid := body[1:]
		if label != "" {
			return "#" + label
		}
		if channels != nil {
			if name := channels.ChannelName(cid); name != "" {
				return "#" + name
			}
		}
		return "#" + cid
	case strings.HasPrefix(body, "!subteam^"):
		if label != "" {
			return "@" + label
		}
		return "@" + strings.TrimPrefix(body, "!subteam^")
	case strings.HasPrefix(body, "!"):
		keyword := strings.TrimPrefix(body, "!")
		if label != "" {
			return "@" + label
		}
		// Broadcast keywords (!here, !channel, !everyone) and date tokens.
		if idx := strings.Index(keyword, "^"); idx >= 0 {
			keyword = keyword[:idx]
		}
		return "@" + keyword
	default:
		// URL with or without label.
		if label != "" {
			return label
		}
		return body
	}
}

// splitLabel splits a mrkdwn token body on "|" into (body, label).
func splitLabel(inner string) (body, label string) {
	if idx := strings.Index(inner, "|"); idx >= 0 {
		return inner[:idx], inner[idx+1:]
	}
	return inner, ""
}
