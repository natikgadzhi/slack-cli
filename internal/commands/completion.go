package commands

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/auth"
	"github.com/natikgadzhi/slack-cli/internal/completion"
)

// Tuning constants for shell completion. Completion runs on every TAB press, so
// fetches are kept short and bounded; results are reused from cache for a while.
const (
	completionTTL      = time.Hour
	completionMaxPages = 5
	completionMaxItems = 1000
)

// completionClient builds an API client tuned for shell completion: a short
// timeout and minimal retries, so a TAB press never hangs the shell waiting on
// Slack. Missing credentials are returned as an error so callers can silently
// fall back to no completions.
func completionClient() (*api.Client, error) {
	xoxc, err := auth.GetXoxc()
	if err != nil {
		return nil, err
	}
	xoxd, err := auth.GetXoxd()
	if err != nil {
		return nil, err
	}
	return api.NewClient(xoxc, xoxd,
		api.WithTimeout(3*time.Second),
		api.WithMaxRetries(1),
	), nil
}

// fetchChannelNames lists the channels the user belongs to (the same set
// `channels get` resolves against) and returns their names. Pagination is
// bounded so a cold completion never runs away.
func fetchChannelNames() ([]string, error) {
	client, err := completionClient()
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"limit":            "200",
		"types":            "public_channel,private_channel",
		"exclude_archived": "true",
	}

	var names []string
	for pages := 0; ; pages++ {
		result, err := client.Call("users.conversations", params)
		if err != nil {
			return nil, err
		}
		for _, ch := range api.ExtractItems(result, "channels") {
			if n, _ := ch["name"].(string); n != "" {
				names = append(names, n)
			}
		}
		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || pages+1 >= completionMaxPages || len(names) >= completionMaxItems {
			break
		}
		params["cursor"] = cursor
	}
	return names, nil
}

// fetchUserHandles lists workspace members and returns their handles, each
// annotated with the user's real name as a completion description
// ("handle\treal name"). Bots and deactivated users are excluded.
func fetchUserHandles() ([]string, error) {
	client, err := completionClient()
	if err != nil {
		return nil, err
	}

	params := map[string]string{"limit": "200"}

	var handles []string
	for pages := 0; ; pages++ {
		result, err := client.Call("users.list", params)
		if err != nil {
			return nil, err
		}
		for _, m := range api.ExtractItems(result, "members") {
			if !filterUser(m, false, false) {
				continue
			}
			name, _ := m["name"].(string)
			if name == "" {
				continue
			}
			if real := realName(m); real != "" {
				handles = append(handles, name+"\t"+real)
			} else {
				handles = append(handles, name)
			}
		}
		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || pages+1 >= completionMaxPages || len(handles) >= completionMaxItems {
			break
		}
		params["cursor"] = cursor
	}
	return handles, nil
}

// realName extracts profile.real_name from a raw Slack user object.
func realName(member map[string]any) string {
	if profile, ok := member["profile"].(map[string]any); ok {
		if rn, ok := profile["real_name"].(string); ok {
			return rn
		}
	}
	return ""
}

// completeChannelNames completes the channel name/ID positional argument of
// `channels get` (and its deprecated `channel` alias). Only the first arg is
// completed. Any fetch error degrades gracefully to no completions.
func completeChannelNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := completion.Cached("channels", completionTTL, fetchChannelNames)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeUserHandles completes a Slack user handle, used by `search --from`.
// If the user has already typed a leading "@", candidates are decorated to
// match so the shell's prefix filter keeps them.
func completeUserHandles(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	handles, err := completion.Cached("users", completionTTL, fetchUserHandles)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if strings.HasPrefix(toComplete, "@") {
		decorated := make([]string, len(handles))
		for i, h := range handles {
			name, desc, found := strings.Cut(h, "\t")
			decorated[i] = "@" + name
			if found {
				decorated[i] += "\t" + desc
			}
		}
		return decorated, cobra.ShellCompDirectiveNoFileComp
	}
	return handles, cobra.ShellCompDirectiveNoFileComp
}

// completeConversationTypes completes the values accepted by --type. The flag
// takes a comma-separated list; this offers the individual valid values.
func completeConversationTypes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{"public_channel", "private_channel", "mpim", "im"}, cobra.ShellCompDirectiveNoFileComp
}

// staticCompletion returns a completion function that always offers the given
// fixed set of values (used for small enum flags like --sort and --output).
func staticCompletion(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
