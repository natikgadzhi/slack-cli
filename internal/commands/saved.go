package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
	"github.com/natikgadzhi/slack-cli/internal/cache"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

type savedMessageItem struct {
	channelID string
	message   map[string]any
}

var savedCmd = &cobra.Command{
	Use:   "saved",
	Short: "List saved Slack messages",
	Long: "List saved Slack messages via Slack's legacy stars API.\n" +
		"Slack's newer Later view is not available via public APIs, so newer saves may be missing.",
	Args: cobra.NoArgs,
	Example: `  slack-cli saved
  slack-cli saved --limit 20
  slack-cli saved -o json`,
	RunE: runSaved,
}

func init() {
	savedCmd.Flags().IntP("limit", "n", 20, "Maximum number of saved messages to return")
	rootCmd.AddCommand(savedCmd)
}

func runSaved(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 {
		return fmt.Errorf("--limit must be at least 1")
	}

	format := output.Resolve(cmd)

	client, resolver, err := setupClient()
	if err != nil {
		return err
	}

	teamCh := fetchTeamURLAsync(client)
	prog := progress.NewCounter("Fetching saved messages", format)

	pageSize := limit
	if pageSize > 200 {
		pageSize = 200
	}

	params := map[string]string{
		"limit": strconv.Itoa(pageSize),
	}

	var allSaved []savedMessageItem
	var isPartial bool

	for {
		prog.Update(len(allSaved))

		result, err := client.Call("stars.list", params)
		if err != nil {
			prog.Finish()

			if _, ok := api.AsRateLimitError(err); ok && len(allSaved) > 0 {
				clierrors.PrintWarning(fmt.Sprintf("rate limited after fetching %d saved messages; showing partial results", len(allSaved)), output.IsJSON(format))
				isPartial = true
				break
			}

			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return fmt.Errorf("fetching saved messages: %w", err)
		}

		allSaved = append(allSaved, extractSavedMessageItems(api.ExtractItems(result, "items"))...)

		cursor := api.ExtractNextCursor(result, "next_cursor")
		if cursor == "" || len(allSaved) >= limit {
			break
		}
		params["cursor"] = cursor
	}

	prog.Finish()

	if len(allSaved) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no saved messages found")
		}
		return nil
	}

	if len(allSaved) > limit {
		allSaved = allSaved[:limit]
	}

	rawMessages := make([]map[string]any, 0, len(allSaved))
	for _, item := range allSaved {
		rawMessages = append(rawMessages, item.message)
	}

	rawMessages, err = resolver.ResolveUsers(rawMessages)
	if err != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: user resolution failed: %v\n", err)
	}
	for i := range allSaved {
		allSaved[i].message = rawMessages[i]
	}

	teamResult := <-teamCh
	teamURL := teamResult.url
	teamErr := teamResult.err
	if teamErr != nil && !output.IsJSON(format) {
		fmt.Fprintf(os.Stderr, "warning: could not get team URL: %v\n", teamErr)
	}

	results := buildSavedResults(allSaved, teamURL, teamErr == nil)

	if output.IsJSON(format) {
		if isPartial {
			pr := clierrors.NewPartialResult(results, "rate limited: results may be incomplete")
			if err := output.PrintJSON(pr); err != nil {
				return err
			}
		} else {
			if err := output.PrintJSON(results); err != nil {
				return err
			}
		}
	} else {
		renderSearchTable(results)
	}

	cacheWrite(getCache(), "saved", fmt.Sprintf("latest-%d", limit), results, cache.Metadata{
		Command: fmt.Sprintf("saved --limit %d", limit),
	})

	if derivedDir := resolveDerivedDir(cmd); derivedDir != "" {
		if err := writeSavedItemFiles(derivedDir, results); err != nil {
			return fmt.Errorf("writing derived files: %w", err)
		}
	}

	if !output.IsJSON(format) {
		if isPartial {
			fmt.Fprintf(os.Stderr, "Done. %d saved messages fetched (partial — rate limited).\n", len(results))
		} else {
			fmt.Fprintf(os.Stderr, "Done. %d saved messages fetched.\n", len(results))
		}
	}

	return nil
}

func extractSavedMessageItems(items []map[string]any) []savedMessageItem {
	saved := make([]savedMessageItem, 0, len(items))
	for _, item := range items {
		if getString(item, "type") != "message" {
			continue
		}

		message, ok := item["message"].(map[string]any)
		if !ok || len(message) == 0 {
			continue
		}

		saved = append(saved, savedMessageItem{
			channelID: getString(item, "channel"),
			message:   copySavedMessage(message),
		})
	}
	return saved
}

func buildSavedResults(items []savedMessageItem, teamURL string, hasTeamURL bool) []map[string]any {
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		formatted := formatting.FormatMessage(item.message)
		result := map[string]any{
			"ts":        formatted.TS,
			"time":      formatted.Time,
			"channel":   item.channelID,
			"user":      savedMessageUser(item.message),
			"text":      savedMessageText(formatted),
			"permalink": savedMessagePermalink(item.message, teamURL, item.channelID, hasTeamURL),
		}
		results = append(results, result)
	}
	return results
}

func savedMessageUser(message map[string]any) string {
	if user := getString(message, "user"); user != "" {
		return user
	}
	return getString(message, "username")
}

func savedMessageText(message formatting.Message) string {
	if message.Text != "" {
		return message.Text
	}
	if message.Attachment == nil {
		return ""
	}

	parts := make([]string, 0, 2)
	if message.Attachment.Title != "" {
		parts = append(parts, message.Attachment.Title)
	}
	if message.Attachment.Text != "" {
		parts = append(parts, message.Attachment.Text)
	}
	return formatting.TruncateRunes(strings.Join(parts, " - "), 500)
}

func savedMessagePermalink(message map[string]any, teamURL, channelID string, hasTeamURL bool) string {
	if permalink := getString(message, "permalink"); permalink != "" {
		return permalink
	}
	if !hasTeamURL || channelID == "" {
		return ""
	}
	ts := getString(message, "ts")
	if ts == "" {
		return ""
	}
	return formatting.BuildPermalink(teamURL, channelID, ts)
}

func copySavedMessage(message map[string]any) map[string]any {
	cp := make(map[string]any, len(message))
	for k, v := range message {
		cp[k] = v
	}
	return cp
}
