package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// emojiAdminEntry is a richer emoji record sourced from emoji.adminList, which
// (unlike emoji.list) exposes per-emoji creation metadata.
type emojiAdminEntry struct {
	Name          string    // raw name without colons
	Type          string    // "custom" or "alias"
	URL           string    // image URL (set for both custom and alias)
	AliasFor      string    // target name (no colons) when Type == "alias"
	Created       time.Time // when the emoji was added to the workspace (UTC)
	CreatedByID   string    // Slack user ID of the uploader
	CreatedByName string    // display name of the uploader at upload time
}

const adminListPageSize = 200

// fetchAllEmojisAdmin paginates emoji.adminList until exhausted and returns
// every workspace emoji with its creation metadata. The spinner reports
// progress in pages.
func fetchAllEmojisAdmin(client *api.Client, format string) ([]emojiAdminEntry, error) {
	spinner := progress.NewSpinner("Fetching emoji metadata", format)
	spinner.Update(0)
	defer spinner.Finish()

	var all []emojiAdminEntry
	page := 1
	for {
		spinner.Update(page)
		params := map[string]string{
			"count": strconv.Itoa(adminListPageSize),
			"page":  strconv.Itoa(page),
		}
		result, err := client.Call("emoji.adminList", params)
		if err != nil {
			if cliErr, ok := api.AsCLIError(err); ok {
				clierrors.PrintError(cliErr, output.IsJSON(format))
				os.Exit(cliErr.ExitCode)
			}
			return nil, fmt.Errorf("fetching emoji metadata (page %d): %w", page, err)
		}

		entries := parseAdminListPage(result)
		all = append(all, entries...)

		totalPages, _ := pagingPages(result)
		if totalPages == 0 || page >= totalPages {
			break
		}
		page++
	}
	return all, nil
}

// parseAdminListPage converts a single emoji.adminList page into typed entries.
func parseAdminListPage(result map[string]any) []emojiAdminEntry {
	raw, ok := result["emoji"].([]any)
	if !ok {
		return nil
	}
	entries := make([]emojiAdminEntry, 0, len(raw))
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		e := emojiAdminEntry{
			Name:          getString(obj, "name"),
			URL:           getString(obj, "url"),
			AliasFor:      getString(obj, "alias_for"),
			CreatedByID:   getString(obj, "user_id"),
			CreatedByName: getString(obj, "user_display_name"),
		}
		if e.Name == "" {
			continue
		}
		// is_alias arrives as a JSON number (0 or 1).
		if n, ok := toInt(obj["is_alias"]); ok && n != 0 {
			e.Type = "alias"
		} else {
			e.Type = "custom"
		}
		if ts, ok := toFloat(obj["created"]); ok && ts > 0 {
			e.Created = time.Unix(int64(ts), 0).UTC()
		}
		entries = append(entries, e)
	}
	return entries
}

// pagingPages extracts paging.pages (total page count) from a paginated response.
func pagingPages(result map[string]any) (int, bool) {
	paging, ok := result["paging"].(map[string]any)
	if !ok {
		return 0, false
	}
	if n, ok := toInt(paging["pages"]); ok {
		return n, true
	}
	return 0, false
}
