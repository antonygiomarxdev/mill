package cli

import (
	"fmt"
	"time"

	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/ledger"
	"github.com/antonygiomarxdev/mill/internal/state"
	"github.com/spf13/cobra"
)

var delegateCmd = &cobra.Command{
	Use:   "delegate <issue>",
	Short: "Delegate work to an AI agent for the given issue",
	Long:  `delegate dispatches an AI agent to work on the specified GitHub issue number.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDelegate(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(delegateCmd)
}

func runDelegate(cmd *cobra.Command, args []string) error {
	issueNum, err := issue.Parse(args[0])
	if err != nil {
		return err
	}

	// Load existing state (creates empty state if file missing)
	s, err := state.Load(statePath())
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Upsert the task with pending status
	taskID := fmt.Sprintf("task-%d", issueNum)
	s.UpsertTask(state.TaskState{
		ID:      taskID,
		Issue:   issueNum,
		Status:  "pending",
		Commits: 0,
	})

	if err := s.Save(statePath()); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Append a ledger entry for this dispatch
	entry := ledger.Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Issue:     issueNum,
		Event:     "dispatch",
		Status:    "pending",
	}
	if err := ledger.Append(ledgerPath(issueNum), entry); err != nil {
		return fmt.Errorf("failed to append ledger entry: %w", err)
	}

	cmd.Printf("Delegated issue %d\n", issueNum)
	return nil
}
