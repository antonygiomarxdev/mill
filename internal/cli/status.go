package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
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

	fmt.Fprintf(w, "ID\tISSUE\tSTATUS\tCOMMITS\tRUNTIME\tVERDICT\n")
	for _, t := range s.Tasks {
		runtime := formatRuntime(t)
		fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%s\t%s\n", t.ID, t.Issue, t.Status, t.Commits, runtime, t.Verdict)
	}

	return nil
}

// formatRuntime returns a human-readable elapsed time for a task.
// Returns "—" if StartedAt is zero (task not yet started).
func formatRuntime(t domain.Task) string {
	if t.StartedAt.IsZero() {
		return "—"
	}
	return time.Since(t.StartedAt).Truncate(time.Second).String()
}
