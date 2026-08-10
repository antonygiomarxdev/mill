// Package cli implements the mill command-line interface using only the
// Go standard library. It wires together the domain, adapter, state,
// and ledger layers.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/issue"
)

type App struct {
	Adapter     adapter.Adapter
	MillDir     string
	Out         io.Writer
	Err         io.Writer
	In          io.Reader
	IssueReader func(issueNum int) (body string, labels []string, err error)
}

// NewApp creates a new App with defaults: .mill directory, CommandCode adapter,
// stdout/stderr for output.
func NewApp() *App {
	return &App{
		Adapter:     &adapter.CommandCodeAdapter{},
		MillDir:     ".mill",
		Out:         os.Stdout,
		Err:         os.Stderr,
		IssueReader: issue.ReadBody,
	}
}

// statePath returns the path to the state file.
func (a *App) statePath() string {
	return filepath.Join(a.MillDir, "state.json")
}

// configPath returns the path to the config file.
func (a *App) configPath() string {
	return filepath.Join(a.MillDir, "config.json")
}

// ledgerPath returns the path to the ledger file for the given issue.
func (a *App) ledgerPath(issue int) string {
	return filepath.Join(a.MillDir, "ledger", fmt.Sprintf("%d.jsonl", issue))
}

// worktreePath returns the worktree directory for the given issue.
func (a *App) worktreePath(issue int) string {
	return filepath.Join(a.MillDir, "worktrees", fmt.Sprintf("issue-%d", issue))
}

// loadConfig loads the mill configuration, returning defaults if the file is missing.
func (a *App) loadConfig() (config.Config, error) {
	return config.Load(a.configPath())
}

// normalize ensures input/output writers have defaults.
func (a *App) normalize() {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if a.In == nil {
		a.In = os.Stdin
	}
}

// Run dispatches to the appropriate subcommand based on the first argument.
func (a *App) Run(args ...string) error {
	a.normalize()

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(a.Err)
		return nil
	}

	switch args[0] {
	case "delegate":
		return a.runDelegate(args[1:])
	case "init":
		return a.runInit(args[1:])
	case "status":
		return a.runStatus(args[1:])
	case "role":
		return a.runRole(args[1:])
	case "land":
		return a.runLand(args[1:])
	case "watch":
		return a.runWatch(args[1:])
	case "compact":
		return a.runCompact(args[1:])
	default:
		usage(a.Err)
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `mill — AI agent delegation harness

Usage:
  mill <command> [flags]

  init [flags]       Initialize a new mill project (scaffolding)
  delegate <issue>   Delegate work to an AI agent for the given issue
  status             Show status of all mill tasks
  role <get|set>     Show or set the active role (staff|pm)
  watch              Wait for task state changes (blocks until all settle)
  compact [--dry-run]  Compact session context to save tokens
`)
}
