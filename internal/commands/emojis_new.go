package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/config"
	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

// emojiSnapshot is the persisted view of which emojis the workspace had
// at the time of the last `emojis new` run. first_seen records when
// slack-cli first observed each emoji.
type emojiSnapshot struct {
	UpdatedAt time.Time                     `json:"updated_at"`
	Emojis    map[string]emojiSnapshotEntry `json:"emojis"`
}

type emojiSnapshotEntry struct {
	FirstSeen time.Time `json:"first_seen"`
	Value     string    `json:"value"` // image URL or "alias:target"
}

// emojisNewCmd lists emojis added since the last snapshot.
var emojisNewCmd = &cobra.Command{
	Use:   "new",
	Short: "List emojis added since the last run",
	Long: `List custom emojis that have appeared since the last time this command ran.

The snapshot of last-seen emojis is persisted under the slack-cli data directory.
The first invocation establishes the baseline and prints nothing new; subsequent
runs print emojis added since the previous snapshot.

Use --since to filter by when slack-cli first observed each emoji (e.g. --since 7d).
Use --reset to replace the baseline with the current emoji set.`,
	Args: cobra.NoArgs,
	Example: `  slack-cli emojis new                # since last run
  slack-cli emojis new --since 7d     # added in the last 7 days
  slack-cli emojis new --reset        # rebaseline the snapshot
  slack-cli emojis new -o json`,
	RunE: runEmojisNew,
}

func init() {
	emojisNewCmd.Flags().String("since", "", "Only show emojis first seen at or after this time (e.g. 7d, 2026-03-01)")
	emojisNewCmd.Flags().Bool("reset", false, "Replace the snapshot with the current emoji set and exit")
	emojisNewCmd.Flags().IntP("limit", "n", 0, "Maximum number of emojis to print (0 = no limit)")
	emojisCmd.AddCommand(emojisNewCmd)
}

func runEmojisNew(cmd *cobra.Command, _ []string) error {
	reset, _ := cmd.Flags().GetBool("reset")
	sinceStr, _ := cmd.Flags().GetString("since")
	limit, _ := cmd.Flags().GetInt("limit")

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	raw, err := fetchEmojisRaw(client, format)
	if err != nil {
		return err
	}

	snapshotPath, err := snapshotFilePath()
	if err != nil {
		return err
	}

	prev, err := loadSnapshot(snapshotPath)
	if err != nil {
		return fmt.Errorf("loading snapshot: %w", err)
	}

	now := time.Now().UTC()

	if reset {
		next := buildSnapshot(raw, prev, now, true)
		if err := saveSnapshot(snapshotPath, next); err != nil {
			return fmt.Errorf("saving snapshot: %w", err)
		}
		if !output.IsJSON(format) {
			fmt.Fprintf(os.Stderr, "Snapshot reset: %d emojis baselined.\n", len(next.Emojis))
		} else {
			_ = output.PrintJSON([]emojiResult{})
		}
		return nil
	}

	// Merge prev + current to produce the new snapshot (additive — never drop entries).
	next := buildSnapshot(raw, prev, now, false)

	// First-ever run: baseline everything, print nothing as "new".
	firstRun := prev == nil || len(prev.Emojis) == 0
	if firstRun {
		if err := saveSnapshot(snapshotPath, next); err != nil {
			return fmt.Errorf("saving snapshot: %w", err)
		}
		if !output.IsJSON(format) {
			fmt.Fprintf(os.Stderr, "First run: baseline saved with %d emojis. Re-run `emojis new` to see additions.\n", len(next.Emojis))
		} else {
			_ = output.PrintJSON([]emojiResult{})
		}
		return nil
	}

	// Compute the cutoff for --since.
	var cutoff time.Time
	if sinceStr != "" {
		ts, perr := formatting.ParseTime(sinceStr)
		if perr != nil {
			return fmt.Errorf("parsing --since: %w", perr)
		}
		cutoff = time.Unix(int64(ts), 0).UTC()
	}

	// Pick the emojis to print.
	current := parseEmojis(raw)
	results := pickNewEmojis(current, next, prev, cutoff, limit)

	// Persist the merged snapshot before printing so re-runs don't re-report.
	if err := saveSnapshot(snapshotPath, next); err != nil {
		return fmt.Errorf("saving snapshot: %w", err)
	}

	if len(results) == 0 {
		if !output.IsJSON(format) {
			fmt.Fprintln(os.Stderr, "no new emojis")
		} else {
			_ = output.PrintJSON([]emojiResult{})
		}
		return nil
	}

	if output.IsJSON(format) {
		if err := output.PrintJSON(results); err != nil {
			return err
		}
	} else {
		renderEmojisTable(results)
		fmt.Fprintf(os.Stderr, "Done. %d new emojis.\n", len(results))
	}

	return nil
}

// pickNewEmojis returns the emojis to report based on the cutoff. If cutoff
// is zero, picks emojis missing from the prev snapshot. Otherwise, picks
// emojis whose first_seen >= cutoff.
func pickNewEmojis(current []emojiEntry, next, prev *emojiSnapshot, cutoff time.Time, limit int) []emojiResult {
	var picked []emojiResult
	useCutoff := !cutoff.IsZero()

	for _, e := range current {
		entry, ok := next.Emojis[e.Name]
		if !ok {
			continue
		}
		if useCutoff {
			if entry.FirstSeen.Before(cutoff) {
				continue
			}
		} else {
			if _, existed := prev.Emojis[e.Name]; existed {
				continue
			}
		}
		picked = append(picked, toEmojiResult(e))
		if limit > 0 && len(picked) >= limit {
			break
		}
	}

	return picked
}

// buildSnapshot merges the current emoji set with the previous snapshot,
// preserving first_seen for emojis that already existed. If replace is true,
// every emoji is re-baselined to `now`.
func buildSnapshot(raw map[string]string, prev *emojiSnapshot, now time.Time, replace bool) *emojiSnapshot {
	next := &emojiSnapshot{
		UpdatedAt: now,
		Emojis:    make(map[string]emojiSnapshotEntry, len(raw)),
	}
	for name, value := range raw {
		firstSeen := now
		if !replace && prev != nil {
			if existing, ok := prev.Emojis[name]; ok {
				firstSeen = existing.FirstSeen
			}
		}
		next.Emojis[name] = emojiSnapshotEntry{
			FirstSeen: firstSeen,
			Value:     value,
		}
	}
	return next
}

// snapshotFilePath returns the path to the persisted emoji snapshot.
func snapshotFilePath() (string, error) {
	dir, err := config.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "emojis", "snapshot.json"), nil
}

// loadSnapshot reads and parses the persisted snapshot. Returns (nil, nil)
// if the file does not exist.
func loadSnapshot(path string) (*emojiSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap emojiSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if snap.Emojis == nil {
		snap.Emojis = map[string]emojiSnapshotEntry{}
	}
	return &snap, nil
}

// saveSnapshot writes the snapshot atomically (temp file + rename).
// json.MarshalIndent sorts map keys, so the output is byte-stable across runs.
func saveSnapshot(path string, snap *emojiSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.tmp")
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
	return nil
}
