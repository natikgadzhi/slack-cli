package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Slack authentication tokens",
}

var authCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if Slack tokens are configured",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

var authSetXoxcCmd = &cobra.Command{
	Use:   "set-xoxc <token>",
	Short: "Store xoxc token in the macOS Keychain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

var authSetXoxdCmd = &cobra.Command{
	Use:   "set-xoxd <token>",
	Short: "Store xoxd cookie in the macOS Keychain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not yet implemented")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authCheckCmd)
	authCmd.AddCommand(authSetXoxcCmd)
	authCmd.AddCommand(authSetXoxdCmd)
	rootCmd.AddCommand(authCmd)
}
