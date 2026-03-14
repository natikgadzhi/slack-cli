package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message <url>",
	Short: "Fetch a single Slack message or thread by URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(messageCmd)
}
