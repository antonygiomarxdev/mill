package cli

import (
	"fmt"
	"io"
)

// RunSlotsCommand is the entry point for the "slots" subcommand.
// It delegates to (*App).runSlots after passing through the remaining args.
//
// Integration with App.Run — add these two changes to internal/cli/app.go:
//
// 1. In the switch statement (after the "compact" case), add:
//
//	case "slots":
//	    return RunSlotsCommand(a, args[1:])
//
// 2. In the usage function, add a line:
//
//	slots             Show slot/concurrency status
func RunSlotsCommand(a *App, args []string) error {
	return a.runSlots(args)
}

// SlotsUsage returns the usage line for the "slots" command.
func SlotsUsage() string {
	return "  slots              Show slot/concurrency status"
}

// PrintSlotsUsage writes the slots usage line to w.
func PrintSlotsUsage(w io.Writer) {
	fmt.Fprintln(w, SlotsUsage())
}
