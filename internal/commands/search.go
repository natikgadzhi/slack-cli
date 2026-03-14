package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Slack messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

func init() {
	searchCmd.Flags().Int("count", 20, "maximum number of results")
	rootCmd.AddCommand(searchCmd)
}
