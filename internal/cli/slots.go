package cli

import (
	"fmt"
	"time"

	"github.com/antonygiomarxdev/mill/internal/slots"
)

// runSlots handles the "slots" command.
// It prints the current slot/concurrency status including occupied slots
// and queued waiters from the slot manager.
//
// Subcommands:
//
//	status  — full status table (default when no subcommand)
//	limit   — print maxSlots only
func (a *App) runSlots(args []string) error {
	if a.slots == nil {
		fmt.Fprintln(a.Out, "No active slot manager. Run 'mill delegate' first.")
		return nil
	}

	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "status", "":
		return a.printSlotStatus(a.slots.Status())
	case "limit":
		status := a.slots.Status()
		fmt.Fprintf(a.Out, "%d\n", status.MaxSlots)
		return nil
	default:
		return fmt.Errorf("unknown slots subcommand: %s (valid: status, limit)", sub)
	}
}

// printSlotStatus formats and writes the slot status table to a.Out.
func (a *App) printSlotStatus(status slots.SlotStatus) error {
	occupied := len(status.Occupied)

	if occupied == 0 {
		fmt.Fprintf(a.Out, "SLOTS: 0/%d — idle\n", status.MaxSlots)
	} else {
		fmt.Fprintf(a.Out, "SLOTS: %d/%d occupied\n", occupied, status.MaxSlots)
		for _, s := range status.Occupied {
			fmt.Fprintf(a.Out, "  %d: %s (issue #%d) — running %s\n",
				s.SlotID, s.Role, s.Issue, formatDuration(s.Running))
		}
	}

	if len(status.Queue) > 0 {
		queued := len(status.Queue)
		fmt.Fprintf(a.Out, "QUEUE: %d waiting\n", queued)
		for _, q := range status.Queue {
			fmt.Fprintf(a.Out, "  #%d: %s (issue #%d) — waiting %s\n",
				q.Position, q.Role, q.Issue, formatDuration(q.Waiting))
		}
	}

	return nil
}

// formatDuration returns a human-readable duration string truncated to seconds.
// Examples: "5s", "2m30s", "1h5m".
func formatDuration(d time.Duration) string {
	return d.Truncate(time.Second).String()
}
