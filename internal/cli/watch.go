package cli

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// CommandError is an error that carries an exit code for os.Exit usage.
type CommandError struct {
	Code int
	Msg  string
}

func (e *CommandError) Error() string { return e.Msg }

// runWatch polls .mill/state.json at a configurable interval until all
// delegate tasks (task- prefix) reach a terminal state, then prints a
// summary table and returns an exit code via CommandError.
func (a *App) runWatch(args []string) error {
	// Pre-scan for help flags so we can show custom usage.
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printWatchUsage(a.Out)
			return nil
		}
	}

	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	interval := fs.Int("interval", 2, "polling interval in seconds")
	timeout := fs.Duration("timeout", 0, "maximum wait time (0 = no timeout)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printWatchUsage(a.Out)
			return nil
		}
		return err
	}

	// Initial state load.
	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	delegateTasks := filterDelegateTasks(s.Tasks)
	if len(delegateTasks) == 0 {
		fmt.Fprintln(a.Out, "No tasks to watch")
		return nil
	}

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	var deadline <-chan time.Time
	if *timeout > 0 {
		timer := time.NewTimer(*timeout)
		defer timer.Stop()
		deadline = timer.C
	}

	// Print initial progress immediately.
	printProgress(a.Out, delegateTasks)

	for {
		select {
		case <-ticker.C:
			s, err = state.Load(a.statePath())
			if err != nil {
				return fmt.Errorf("error loading state: %w", err)
			}

			delegateTasks = filterDelegateTasks(s.Tasks)

			if allTerminal(delegateTasks) {
				clearProgress(a.Out)
				printFinalSummary(a.Out, delegateTasks)
				return computeExitCode(delegateTasks)
			}

			printProgress(a.Out, delegateTasks)

		case <-deadline:
			clearProgress(a.Out)
			printTimeoutSummary(a.Out, delegateTasks)
			return &CommandError{Code: 124, Msg: "timeout reached with tasks still running"}
		}
	}
}

// printWatchUsage writes the watch subcommand help text to w.
func printWatchUsage(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(w, `Usage: mill watch [--interval <seconds>] [--timeout <duration>]

--interval: polling interval in seconds (default: 2)
--timeout: maximum wait duration (default: 0 = no timeout). Examples: 30s, 5m, 1h

Exit codes:
  0 — all tasks completed successfully
  1 — one task errored
  N — N tasks errored (capped at 125)
  124 — timeout reached with tasks still running
`)
}


// isTerminal returns true when a task has reached a terminal state.
func isTerminal(t domain.Task) bool {
	return t.Status == domain.TaskDone || t.Status == domain.TaskError
}

// allTerminal returns true when every task in the slice is terminal.
func allTerminal(tasks []domain.Task) bool {
	for _, t := range tasks {
		if !isTerminal(t) {
			return false
		}
	}
	return true
}

// filterDelegateTasks returns tasks whose ID has the "task-" prefix (created
// by mill delegate), sorted by ID for deterministic output.
func filterDelegateTasks(tasks map[string]domain.Task) []domain.Task {
	var result []domain.Task
	for id, t := range tasks {
		if strings.HasPrefix(id, "task-") {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// printProgress writes a single-line progress report that overwrites the
// previous line via \r. Format:
//
//	Watching N tasks... task-392: running (12s) | task-393: running (8s)
func printProgress(w interface{ Write([]byte) (int, error) }, tasks []domain.Task) {
	parts := make([]string, 0, len(tasks))
	now := time.Now()
	for _, t := range tasks {
		elapsed := int(now.Sub(t.StartedAt).Seconds())
		parts = append(parts, fmt.Sprintf("%s: %s (%ds)", t.ID, t.Status, elapsed))
	}
	fmt.Fprintf(w, "\rWatching %d tasks... %s", len(tasks), strings.Join(parts, " | "))
}

// clearProgress erases the progress line by overwriting it with spaces
// and returning to column 0.
func clearProgress(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(w, "\r                                                                                \r")
}

// printFinalSummary writes the tab-separated summary table and a counts line.
func printFinalSummary(w interface{ Write([]byte) (int, error) }, tasks []domain.Task) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "ID\tISSUE\tSTATUS\tCOMMITS\tVERDICT\n")
	for _, t := range tasks {
		verdict := string(t.Verdict)
		if t.Status == domain.TaskError && verdict == "" {
			verdict = "FATAL"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%s\n", t.ID, t.Issue, t.Status, t.Commits, verdict)
	}
	tw.Flush()

	succeeded := 0
	failed := 0
	for _, t := range tasks {
		if t.Status == domain.TaskDone {
			succeeded++
		} else {
			failed++
		}
	}
	fmt.Fprintf(w, "\n%d/%d tasks succeeded, %d failed\n", succeeded, len(tasks), failed)
}

// printTimeoutSummary reports which tasks are still running after timeout.
func printTimeoutSummary(w interface{ Write([]byte) (int, error) }, tasks []domain.Task) {
	var running []string
	for _, t := range tasks {
		if !isTerminal(t) {
			running = append(running, t.ID)
		}
	}
	if len(running) > 0 {
		fmt.Fprintf(w, "Timeout reached. Still running: %s\n", strings.Join(running, ", "))
	}
}

// computeExitCode returns a CommandError with the appropriate exit code:
// 0 if all succeeded, otherwise 1 + errorCount capped at 125.
func computeExitCode(tasks []domain.Task) error {
	errors := 0
	for _, t := range tasks {
		if t.Status == domain.TaskError {
			errors++
		}
	}
	if errors == 0 {
		return nil
	}
	code := errors + 1
	if code > 125 {
		code = 125
	}
	return &CommandError{Code: code, Msg: fmt.Sprintf("%d/%d tasks succeeded, %d failed", len(tasks)-errors, len(tasks), errors)}
}
