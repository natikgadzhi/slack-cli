// Package commands defines the cobra command tree for slack-cli.
package commands

import (
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/version"
	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

// Persistent flag values accessible to subcommands.
var (
	NoCache    bool
	DerivedDir string
)

// versionInfo is populated at init time from build-time vars.
var versionInfo = &version.Info{
	Version: buildVersion,
	Commit:  buildCommit,
	Date:    buildDate,
}

// rootCmd is the top-level command for the CLI.
var rootCmd = &cobra.Command{
	Use:   "slack-cli",
	Short: "Slack read-only CLI",
	Long:  "slack-cli — Slack read-only CLI for fetching messages, threads, and history.",
	Example: `  slack-cli auth check
  slack-cli message 'https://yourteam.slack.com/archives/C12345/p1741234567123456'
  slack-cli channel general --since 2d --limit 100
  slack-cli channel C12345678 --since 2026-03-01 --until 2026-03-10
  slack-cli search "deployment failed" --limit 10
  slack-cli search "from:@alice" -o json | jq '.[] | .text'`,
}

func init() {
	// Register cli-kit output flag (-o/--output with TTY auto-detection).
	output.RegisterFlag(rootCmd)

	rootCmd.PersistentFlags().BoolVar(&NoCache, "no-cache", false, "Skip cache for this request")
	rootCmd.PersistentFlags().StringVarP(&DerivedDir, "derived", "d", "", "Derived data directory (default: ~/.local/share/lambdal/derived/slack-cli)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging to stderr")

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	// Register version subcommand and --version flag from cli-kit.
	rootCmd.AddCommand(version.NewCommand(versionInfo))
	version.SetupFlag(rootCmd, versionInfo)
}

// Execute runs the root command. Call this from main().
func Execute() error {
	return rootCmd.Execute()
}
