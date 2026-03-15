// Package commands defines the cobra command tree for slack-cli.
package commands

import (
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

// Persistent flag values accessible to subcommands.
var (
	OutputFormat string
	NoCache      bool
	OutputDir    string
)

// rootCmd is the top-level command for the CLI.
var rootCmd = &cobra.Command{
	Use:     "slack-cli",
	Short:   "Slack read-only CLI",
	Long:    "Slack read-only CLI for fetching messages, threads, and history.",
	Version: Version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&OutputFormat, "output", "o", "json", "output format (json|markdown)")
	rootCmd.PersistentFlags().BoolVar(&NoCache, "no-cache", false, "disable cache")
	rootCmd.PersistentFlags().StringVarP(&OutputDir, "output-dir", "d", "", "write individual markdown files per item to this directory")
	rootCmd.SilenceErrors = true
}

// Execute runs the root command. Call this from main().
func Execute() error {
	return rootCmd.Execute()
}
