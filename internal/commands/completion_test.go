package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// seedCompletionCache writes a fresh completion cache file for key (so the
// completion functions read it instead of hitting the network).
func seedCompletionCache(t *testing.T, dir, key string, values []string) {
	t.Helper()
	path := filepath.Join(dir, "completion", key+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := map[string]any{
		"fetched_at": time.Now().Format(time.RFC3339Nano),
		"values":     values,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestCompleteConversationTypes(t *testing.T) {
	got, directive := completeConversationTypes(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"public_channel", "private_channel", "mpim", "im"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestStaticCompletion(t *testing.T) {
	fn := staticCompletion("relevance", "recent")
	got, directive := fn(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	if len(got) != 2 || got[0] != "relevance" || got[1] != "recent" {
		t.Fatalf("got %v, want [relevance recent]", got)
	}
}

func TestCompleteChannelNamesIgnoresSecondArg(t *testing.T) {
	// With an argument already present, no further completion is offered and no
	// network/cache access happens.
	got, directive := completeChannelNames(nil, []string{"general"}, "")
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
}

func TestCompleteChannelNamesFromCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)
	seedCompletionCache(t, dir, "channels", []string{"general", "random", "eng"})

	got, directive := completeChannelNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	if len(got) != 3 || got[0] != "general" {
		t.Fatalf("got %v, want cached channel names", got)
	}
}

func TestCompleteUserHandles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLACK_DATA_DIR", dir)
	seedCompletionCache(t, dir, "users", []string{"alice\tAlice Adams", "bob"})

	t.Run("bare", func(t *testing.T) {
		got, directive := completeUserHandles(nil, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("directive = %v, want NoFileComp", directive)
		}
		if len(got) != 2 || got[0] != "alice\tAlice Adams" || got[1] != "bob" {
			t.Fatalf("got %v, want raw handles", got)
		}
	})

	t.Run("at_prefix", func(t *testing.T) {
		got, _ := completeUserHandles(nil, nil, "@")
		if len(got) != 2 || got[0] != "@alice\tAlice Adams" || got[1] != "@bob" {
			t.Fatalf("got %v, want @-decorated handles", got)
		}
	})
}
