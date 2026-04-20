package commands

import (
	"encoding/json"
	"fmt"
	"os"

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
	rootCmd.AddCommand(savedCmd)
}

func runSaved(cmd *cobra.Command, _ []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	result, err := client.Call(endpoint, nil)
	if err != nil {
		return fmt.Errorf("call %s: %w", endpoint, err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
