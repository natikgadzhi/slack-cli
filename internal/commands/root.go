// Package commands defines the cobra command tree for slack-cli.
package commands

import (
	"github.com/spf13/cobra"
)

// Persistent flag values accessible to subcommands.
var (
	OutputFormat string
	NoCache      bool
)

// rootCmd is the top-level command for the CLI.
var rootCmd = &cobra.Command{
	Use:   "slack-cli",
	Short: "Slack read-only CLI",
	Long:  "Slack read-only CLI for fetching messages, threads, and history.",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&OutputFormat, "output", "o", "json", "output format (json|markdown)")
	rootCmd.PersistentFlags().BoolVar(&NoCache, "no-cache", false, "disable cache")
	rootCmd.SilenceErrors = true
}

// Execute runs the root command. Call this from main().
func Execute() error {
	return rootCmd.Execute()
}
