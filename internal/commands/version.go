package commands

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information as JSON",
	Run: func(cmd *cobra.Command, args []string) {
		info := map[string]string{
			"version": Version,
			"commit":  Commit,
			"date":    Date,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
