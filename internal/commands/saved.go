package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// savedCmd is a scratch command used to capture the raw response shape from
// the Slack saved-items endpoint so we can build realistic test fixtures.
// It prints the raw JSON response to stdout and exits. This is intentionally
// minimal — full feature implementation comes after we've seen the real shape.
var savedCmd = &cobra.Command{
	Use:    "saved",
	Short:  "List saved messages (WIP scratch — dumps raw API response)",
	Hidden: true,
	RunE:   runSaved,
}

func init() {
	savedCmd.Flags().String("endpoint", "saved.list", "Slack API method to hit")
	savedCmd.Flags().IntP("limit", "n", 0, "Max items (0 = server default)")
	rootCmd.AddCommand(savedCmd)
}

func runSaved(cmd *cobra.Command, _ []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	limit, _ := cmd.Flags().GetInt("limit")

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	var params map[string]string
	if limit > 0 {
		params = map[string]string{"count": strconv.Itoa(limit)}
	}

	result, err := client.Call(endpoint, params)
	if err != nil {
		return fmt.Errorf("call %s: %w", endpoint, err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
