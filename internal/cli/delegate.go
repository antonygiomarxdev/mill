package cli

import (
	"flag"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/classify"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/ledger"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// runDelegate handles the "delegate <issue>" command.
// It creates a task, dispatches an AI agent via the configured adapter,
// waits for completion, classifies the result, and persists state + ledger entries.
// Classification drives the retry/abort policy:
// OK/MAX_TURNS→done, AUTH/NO_CREDIT→abort, RATE_LIMITED/TRANSIENT→backoff+retry,
// BLOCKED→persist+stop, FATAL→retry.
func (a *App) runDelegate(args []string) error {
	fs := flag.NewFlagSet("delegate", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var model string
	var maxTurns int
	fs.StringVar(&model, "model", "", "model to use (default: from config)")
	fs.IntVar(&maxTurns, "max-turns", 100, "max conversation turns")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	fsArgs := fs.Args()
	if len(fsArgs) < 1 {
		fs.Usage()
		return fmt.Errorf("usage: mill delegate <issue>")
	}

	issueNum, err := issue.Parse(fsArgs[0])
	if err != nil {
		return err
	}

	// Load config for default model
	cfg, err := a.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if model == "" {
		model = cfg.Model
	}

	// Create task and persist initial state
	taskID := fmt.Sprintf("task-%d", issueNum)
	task := domain.NewTask(taskID, issueNum)

	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	s.UpsertTask(task)
	if err := s.Save(a.statePath()); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Append dispatch ledger entry
	dispatchEntry := ledger.Entry{
		Timestamp: time.Now().UTC(),
		Issue:     issueNum,
		Event:     "dispatch",
		Status:    string(domain.TaskRunning),
	}
	if err := ledger.Append(a.ledgerPath(issueNum), dispatchEntry); err != nil {
		return fmt.Errorf("failed to append ledger entry: %w", err)
	}

	// Build prompt and dispatch opts
	prompt := buildPrompt(issueNum)
	opts := adapter.DispatchOpts{
		Worktree: a.worktreePath(issueNum),
		Prompt:   prompt,
		Model:    model,
		MaxTurns: maxTurns,
	}

	// Dispatch loop with classification-driven retry/abort.
	const maxRetries = 3
	var classification domain.Classification
	var result adapter.SessionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Dispatch the agent
		session, err := a.Adapter.Dispatch(opts)
		if err != nil {
			a.recordError(s, issueNum, task, err, "failed to dispatch agent")
			return fmt.Errorf("failed to dispatch agent: %w", err)
		}

		// Wait for the agent to finish
		result, err = session.Wait()
		if err != nil {
			a.recordError(s, issueNum, task, err, "agent session failed")
			return fmt.Errorf("agent session failed: %w", err)
		}

		// Classify the result
		classification = classify.Classify(result.ExitCode, result.Stderr)

		// Log classification decision to ledger
		classifyEntry := ledger.Entry{
			Timestamp:      time.Now().UTC(),
			Issue:          issueNum,
			Event:          "classify",
			Status:         string(domain.TaskRunning),
			Classification: string(classification),
		}
		if err := ledger.Append(a.ledgerPath(issueNum), classifyEntry); err != nil {
			return fmt.Errorf("failed to append ledger entry: %w", err)
		}

		// Retry/abort decisions based on classification
		switch classification {
		case domain.ClassificationRateLimited, domain.ClassificationTransient:
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
		case domain.ClassificationFatal:
			if attempt < maxRetries {
				continue
			}
		}
		break // done: OK, MAX_TURNS, AUTH, NO_CREDIT, BLOCKED, or retries exhausted
	}

	// Determine final task status and verdict
	taskStatus := domain.TaskDone
	verdict := domain.VerdictApproved
	if classification != domain.ClassificationOK && classification != domain.ClassificationMaxTurns {
		taskStatus = domain.TaskError
		verdict = domain.VerdictRejected
	}
	task.UpdateStatus(taskStatus, verdict, result.Commits)

	s.UpsertTask(task)
	if err := s.Save(a.statePath()); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Append completion ledger entry
	completeEntry := ledger.Entry{
		Timestamp:      time.Now().UTC(),
		Issue:          issueNum,
		Event:          "complete",
		Status:         string(taskStatus),
		Verdict:        string(verdict),
		Classification: string(classification),
	}
	if err := ledger.Append(a.ledgerPath(issueNum), completeEntry); err != nil {
		return fmt.Errorf("failed to append ledger entry: %w", err)
	}

	fmt.Fprintf(a.Out, "Delegated issue %d — verdict: %s, commits: %d\n", issueNum, verdict, result.Commits)
	return nil
}

// recordError updates the task and ledger to reflect an agent failure.
func (a *App) recordError(s state.State, issueNum int, task domain.Task, err error, event string) {
	task.UpdateStatus(domain.TaskError, domain.VerdictRejected, 0)
	s.UpsertTask(task)
	s.Save(a.statePath())

	ledger.Append(a.ledgerPath(issueNum), ledger.Entry{
		Timestamp: time.Now().UTC(),
		Issue:     issueNum,
		Event:     event,
		Status:    string(domain.TaskError),
	})
}

// buildPrompt constructs the query passed to `cmd -p` for a given issue.
func buildPrompt(issueNum int) string {
	return fmt.Sprintf(`You are mill, an agent delegation harness. Work on GitHub issue #%d.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.`, issueNum)
}
