package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of slack-cli",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("slack-cli version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
