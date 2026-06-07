package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// emojisDownloadCmd downloads custom emoji images to disk.
var emojisDownloadCmd = &cobra.Command{
	Use:   "download [name]",
	Short: "Download custom emoji images to disk",
	Long: `Download custom emoji images to a local directory.

With no arguments, syncs every custom emoji in the workspace. The download is
incremental: a manifest.json at the root of --download-dir tracks every emoji
seen on previous runs, and only emojis missing from the manifest (or missing
their image file on disk) are fetched. Pass --overwrite to re-download
everything.

Aliases produce a sidecar "<name>.alias" file containing the target's name
instead of a duplicated image. The manifest records creator and creation
timestamp for every emoji (sourced from emoji.adminList).

With a positional argument, downloads only that emoji. The name may be given
with or without surrounding colons (e.g. "fire" or ":fire:").`,
	Args: cobra.MaximumNArgs(1),
	Example: `  slack-cli emojis download
  slack-cli emojis download --download-dir ./wiki/img
  slack-cli emojis download fire
  slack-cli emojis download :fire:
  slack-cli emojis download --overwrite
  slack-cli emojis download --keep-removed`,
	RunE: runEmojisDownload,
}

func init() {
	emojisDownloadCmd.Flags().String("download-dir", "slack-emojis", "Directory for downloaded files")
	emojisDownloadCmd.Flags().Bool("overwrite", false, "Re-download files that already exist on disk")
	emojisDownloadCmd.Flags().Bool("keep-removed", false, "Keep manifest entries for emojis that no longer exist in the workspace")
	emojisCmd.AddCommand(emojisDownloadCmd)
}

const emojiManifestVersion = 1
const emojiManifestFilename = "manifest.json"

// emojiManifest is the persisted catalog of downloaded emojis. It lives at
// <download-dir>/manifest.json and is the source of truth for incremental syncs.
type emojiManifest struct {
	Version   int                           `json:"version"`
	UpdatedAt time.Time                     `json:"updated_at"`
	Emojis    map[string]emojiManifestEntry `json:"emojis"`
}

type emojiManifestEntry struct {
	Type          string    `json:"type"` // "custom" or "alias"
	URL           string    `json:"url,omitempty"`
	AliasFor      string    `json:"alias_for,omitempty"`
	LocalPath     string    `json:"local_path"` // path relative to the download dir
	CreatedAt     time.Time `json:"created_at,omitempty"`
	CreatedByID   string    `json:"created_by_id,omitempty"`
	CreatedByName string    `json:"created_by_name,omitempty"`
}

// emojiDownloadStats accumulates per-run counts for the summary line.
type emojiDownloadStats struct {
	Downloaded int
	Skipped    int
	Aliases    int
	Removed    int
	Errors     int
}

func runEmojisDownload(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("download-dir")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	keepRemoved, _ := cmd.Flags().GetBool("keep-removed")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	entries, err := fetchAllEmojisAdmin(client, format)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no emojis found")
		return nil
	}

	// Filter to one if a name was provided.
	singleName := ""
	if len(args) == 1 {
		singleName = strings.Trim(args[0], ":")
		var picked []emojiAdminEntry
		for _, e := range entries {
			if e.Name == singleName {
				picked = append(picked, e)
				break
			}
		}
		if len(picked) == 0 {
			return fmt.Errorf("no emoji named %q", singleName)
		}
		entries = picked
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating download dir: %w", err)
	}

	manifestPath := filepath.Join(dir, emojiManifestFilename)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}
	if manifest == nil {
		manifest = &emojiManifest{Version: emojiManifestVersion, Emojis: map[string]emojiManifestEntry{}}
	}

	var stats emojiDownloadStats
	for _, e := range entries {
		syncOneEmoji(client, e, dir, manifest, overwrite, &stats)
	}

	// Prune manifest entries for emojis that no longer exist in the workspace.
	// Only safe when we just synced the full workspace, not a single name.
	if singleName == "" && !keepRemoved {
		stats.Removed = pruneManifest(manifest, entries, dir)
	}

	manifest.Version = emojiManifestVersion
	manifest.UpdatedAt = time.Now().UTC()
	if err := saveManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Done. %d downloaded, %d aliases, %d skipped, %d removed, %d errors.\n",
		stats.Downloaded, stats.Aliases, stats.Skipped, stats.Removed, stats.Errors)

	if stats.Errors > 0 {
		return fmt.Errorf("%d emoji download(s) failed", stats.Errors)
	}
	return nil
}

// syncOneEmoji writes the image (or alias sidecar) and updates the manifest entry.
// If the local file already exists, the download is skipped and the manifest
// entry is (re)populated — so a directory full of images from a pre-manifest
// run gets adopted on the next sync without re-downloading anything.
func syncOneEmoji(client *api.Client, e emojiAdminEntry, dir string, manifest *emojiManifest, overwrite bool, stats *emojiDownloadStats) {
	if e.Type == "alias" {
		syncAlias(e, dir, manifest, overwrite, stats)
		return
	}

	if e.URL == "" {
		fmt.Fprintf(os.Stderr, "warning: %s has no URL, skipping\n", e.Name)
		stats.Errors++
		return
	}

	ext := emojiFileExt(e.URL)
	localName := sanitizeFilename(e.Name) + ext
	dest := filepath.Join(dir, localName)

	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			stats.Skipped++
			// Backfill / refresh the manifest entry with adminList metadata.
			manifest.Emojis[e.Name] = manifestEntryFor(e, localName)
			return
		}
	}

	if err := client.DownloadFile(e.URL, dest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", e.Name, err)
		stats.Errors++
		return
	}
	stats.Downloaded++
	manifest.Emojis[e.Name] = manifestEntryFor(e, localName)
}

// syncAlias writes the alias sidecar file and updates the manifest. If the
// sidecar already exists with the right target, the write is skipped.
// Re-points (sidecar exists but contains a stale target) trigger a rewrite.
func syncAlias(e emojiAdminEntry, dir string, manifest *emojiManifest, overwrite bool, stats *emojiDownloadStats) {
	if e.AliasFor == "" {
		fmt.Fprintf(os.Stderr, "warning: alias %s has no target, skipping\n", e.Name)
		stats.Errors++
		return
	}
	localName := sanitizeFilename(e.Name) + ".alias"
	path := filepath.Join(dir, localName)

	if !overwrite {
		if existing, err := os.ReadFile(path); err == nil {
			if strings.TrimSpace(string(existing)) == e.AliasFor {
				stats.Skipped++
				manifest.Emojis[e.Name] = manifestEntryFor(e, localName)
				return
			}
		}
	}

	if err := os.WriteFile(path, []byte(e.AliasFor+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing alias %s: %v\n", e.Name, err)
		stats.Errors++
		return
	}
	stats.Aliases++
	manifest.Emojis[e.Name] = manifestEntryFor(e, localName)
}

func manifestEntryFor(e emojiAdminEntry, localName string) emojiManifestEntry {
	return emojiManifestEntry{
		Type:          e.Type,
		URL:           e.URL,
		AliasFor:      e.AliasFor,
		LocalPath:     localName,
		CreatedAt:     e.Created,
		CreatedByID:   e.CreatedByID,
		CreatedByName: e.CreatedByName,
	}
}

// pruneManifest removes manifest entries (and their on-disk files) for emojis
// that no longer exist in the workspace. Returns the number of entries pruned.
func pruneManifest(manifest *emojiManifest, current []emojiAdminEntry, dir string) int {
	currentNames := make(map[string]struct{}, len(current))
	for _, e := range current {
		currentNames[e.Name] = struct{}{}
	}
	var removed int
	for name, entry := range manifest.Emojis {
		if _, ok := currentNames[name]; ok {
			continue
		}
		if entry.LocalPath != "" {
			if err := os.Remove(filepath.Join(dir, entry.LocalPath)); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "warning: removing %s: %v\n", entry.LocalPath, err)
			}
		}
		delete(manifest.Emojis, name)
		removed++
	}
	return removed
}

// loadManifest reads and parses an existing manifest. Returns (nil, nil) if
// the file does not exist.
func loadManifest(path string) (*emojiManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m emojiManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m.Emojis == nil {
		m.Emojis = map[string]emojiManifestEntry{}
	}
	return &m, nil
}

// saveManifest writes the manifest atomically (temp file + rename).
// json.MarshalIndent sorts map keys for byte-stable output.
func saveManifest(path string, m *emojiManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating manifest dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	// CreateTemp uses 0600 perms; relax to 0644 so wiki tooling can read it.
	_ = os.Chmod(path, 0o644)
	return nil
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
