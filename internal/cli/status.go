package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/antonygiomarxdev/mill/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all mill tasks",
	Long:  `status prints a table of current task states.`,
	Run: func(cmd *cobra.Command, args []string) {
		runStatus(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	out := cmd.OutOrStdout()
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	s, err := state.Load(statePath())
	if err != nil {
		cmd.PrintErrf("error loading state: %v\n", err)
		return
	}

	// Header
	fmt.Fprintf(w, "ID\tISSUE\tSTATUS\tCOMMITS\tVERDICT\n")

	for _, t := range s.Tasks {
		fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%s\n", t.ID, t.Issue, t.Status, t.Commits, t.Verdict)
	}
}
