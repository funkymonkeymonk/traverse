package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version, commit hash, and build date of the Traverse CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Traverse CLI\n")
		fmt.Printf("  Version: %s\n", Version)
		fmt.Printf("  Commit:  %s\n", Commit)
		fmt.Printf("  Date:    %s\n", BuildDate)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
