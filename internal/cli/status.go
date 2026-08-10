package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/antonygiomarxdev/mill/internal/state"
)

// runStatus handles the "status" command.
// It loads persisted state and prints a table of all current tasks.
func (a *App) runStatus(args []string) error {
	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	role := a.readActiveRole()
	fmt.Fprintf(a.Out, "Active role: %s\n\n", role)

	w := tabwriter.NewWriter(a.Out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "ID\tISSUE\tSTATUS\tCOMMITS\tVERDICT\n")
	for _, t := range s.Tasks {
		fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%s\n", t.ID, t.Issue, t.Status, t.Commits, t.Verdict)
	}

	return nil
}
