package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var channelCmd = &cobra.Command{
	Use:   "channel <name|id>",
	Short: "Fetch messages from a Slack channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

func init() {
	channelCmd.Flags().String("since", "", "start time (e.g. 2d, 2026-03-01)")
	channelCmd.Flags().String("until", "", "end time (e.g. 2026-03-10)")
	channelCmd.Flags().Int("limit", 50, "maximum number of messages to fetch")
	rootCmd.AddCommand(channelCmd)
}
