package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// emojisDownloadCmd downloads custom emoji images to disk.
var emojisDownloadCmd = &cobra.Command{
	Use:   "download [name]",
	Short: "Download custom emoji images to disk",
	Long: `Download custom emoji images to a local directory.

With no arguments, downloads every custom emoji in the workspace. Files already
present on disk are skipped unless --overwrite is set. Aliases produce a
sidecar "<name>.alias" file containing the target's name, instead of a
duplicated image.

With a positional argument, downloads only that emoji. The name may be given
with or without surrounding colons (e.g. "fire" or ":fire:").`,
	Args: cobra.MaximumNArgs(1),
	Example: `  slack-cli emojis download
  slack-cli emojis download --download-dir ./wiki/img
  slack-cli emojis download fire
  slack-cli emojis download :fire:
  slack-cli emojis download --overwrite`,
	RunE: runEmojisDownload,
}

func init() {
	emojisDownloadCmd.Flags().String("download-dir", "slack-emojis", "Directory for downloaded files")
	emojisDownloadCmd.Flags().Bool("overwrite", false, "Re-download files that already exist on disk")
	emojisCmd.AddCommand(emojisDownloadCmd)
}

// emojiDownloadStats accumulates per-run counts for the summary line.
type emojiDownloadStats struct {
	Downloaded int
	Skipped    int
	Aliases    int
	Errors     int
}

func runEmojisDownload(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("download-dir")
	overwrite, _ := cmd.Flags().GetBool("overwrite")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	entries, err := fetchEmojiEntries(client, format)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no emojis found")
		return nil
	}

	// Filter to one if a name was provided.
	if len(args) == 1 {
		want := strings.Trim(args[0], ":")
		var picked []emojiEntry
		for _, e := range entries {
			if e.Name == want {
				picked = append(picked, e)
				break
			}
		}
		if len(picked) == 0 {
			return fmt.Errorf("no emoji named %q", want)
		}
		entries = picked
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating download dir: %w", err)
	}

	var stats emojiDownloadStats
	for _, e := range entries {
		downloadOneEmoji(client, e, dir, overwrite, &stats)
	}

	fmt.Fprintf(os.Stderr, "Done. %d downloaded, %d aliases, %d skipped, %d errors.\n",
		stats.Downloaded, stats.Aliases, stats.Skipped, stats.Errors)

	if stats.Errors > 0 {
		return fmt.Errorf("%d emoji download(s) failed", stats.Errors)
	}
	return nil
}

func downloadOneEmoji(client *api.Client, e emojiEntry, dir string, overwrite bool, stats *emojiDownloadStats) {
	if e.Type == "alias" {
		writeAliasSidecar(e, dir, stats)
		return
	}

	if e.URL == "" {
		fmt.Fprintf(os.Stderr, "warning: %s has no URL, skipping\n", e.Name)
		stats.Errors++
		return
	}

	ext := emojiFileExt(e.URL)
	dest := filepath.Join(dir, sanitizeFilename(e.Name)+ext)

	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			stats.Skipped++
			return
		}
	}

	if err := client.DownloadFile(e.URL, dest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", e.Name, err)
		stats.Errors++
		return
	}
	stats.Downloaded++
}

// writeAliasSidecar drops a "<name>.alias" file next to the downloaded images
// so wiki tooling can render "name -> target" without fetching another image.
func writeAliasSidecar(e emojiEntry, dir string, stats *emojiDownloadStats) {
	if e.AliasTarget == "" {
		fmt.Fprintf(os.Stderr, "warning: alias %s has no target, skipping\n", e.Name)
		stats.Errors++
		return
	}
	path := filepath.Join(dir, sanitizeFilename(e.Name)+".alias")
	if err := os.WriteFile(path, []byte(e.AliasTarget+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing alias %s: %v\n", e.Name, err)
		stats.Errors++
		return
	}
	stats.Aliases++
}

// emojiFileExt returns a sensible filename extension for an emoji image URL.
// Falls back to ".png" if the URL has no recognizable extension.
func emojiFileExt(emojiURL string) string {
	parsed, err := url.Parse(emojiURL)
	if err != nil {
		return ".png"
	}
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	switch ext {
	case ".png", ".gif", ".jpg", ".jpeg", ".webp", ".apng":
		return ext
	default:
		return ".png"
	}
}
