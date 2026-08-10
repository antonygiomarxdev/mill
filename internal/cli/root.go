package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mill",
	Short: "Mill — AI agent delegation harness",
	Long: `Mill routes tasks to the right AI worker and tracks progress.
Like a foreman on a ranch, it dispatches issues to specialized agents,
classifies their output, and persists state to disk.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
