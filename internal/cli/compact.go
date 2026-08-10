package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/antonygiomarxdev/mill/internal/compact"
)

func (a *App) runCompact(args []string) error {
	fs := flag.NewFlagSet("compact", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	dryRun := fs.Bool("dry-run", false, "show what would be compacted without applying")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if len(fs.Args()) > 0 {
		fs.Usage()
		return fmt.Errorf("compact takes no positional arguments")
	}

	sessionPath := filepath.Join(a.MillDir, "session.ndjson")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(a.Out, "No active session to compact.")
			return nil
		}
		return fmt.Errorf("failed to read session: %w", err)
	}
	contextText := string(data)

	cfg, err := a.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	tier := tierForModel(cfg.Model)

	should, estimated := compact.ShouldCompact(contextText, tier)
	window := int(compact.WindowPaid)
	switch tier {
	case "free":
		window = int(compact.WindowFree)
	case "paid", "pro":
		window = int(compact.WindowPaid)
	}
	pct := float64(estimated) / float64(window) * 100

	if *dryRun {
		printDryRun(a.Out, contextText, tier, estimated, window, pct, should)
		return nil
	}

	if !should {
		fmt.Fprintf(a.Out, "Context at %.0f%% of window — compaction not needed.\n", pct)
		return nil
	}

	compacted, event := compact.Compact(contextText, tier, 0)
	event.Trigger = "manual"

	if err := os.WriteFile(sessionPath, []byte(compacted), 0o644); err != nil {
		return fmt.Errorf("failed to write compacted session: %w", err)
	}

	if err := compact.WriteLog(event); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to write compaction log: %v\n", err)
	}

	fmt.Fprintf(a.Out, "Compacted: %d → %d tokens (saved %d)\n",
		event.PreTokens, event.PostTokens, event.Saved)
	return nil
}

func printDryRun(w io.Writer, contextText, tier string, estimated, window int, pct float64, wouldTrigger bool) {
	fmt.Fprintf(w, "Estimated tokens: %d\n", estimated)
	fmt.Fprintf(w, "Context window: %d (tier: %s)\n", window, tier)
	fmt.Fprintf(w, "Current usage: %.0f%%\n", pct)
	if wouldTrigger {
		fmt.Fprintln(w, "Compaction would trigger: yes")
	} else {
		fmt.Fprintln(w, "Compaction would trigger: no")
	}

	compacted, event := compact.Compact(contextText, tier, 0)
	fmt.Fprintf(w, "Post-compaction estimate: %d tokens\n", event.PostTokens)
	fmt.Fprintf(w, "Would save: %d tokens\n", event.Saved)
	fmt.Fprintln(w, "Preserved: original prompt, role info, last 3 turns, unresolved items")
	fmt.Fprintln(w, "Discarded: old tool outputs, completed sub-agent dialogue, resolved errors")
	_ = compacted
}
