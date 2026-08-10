package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/ledger"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// ============================================================================
// Issue #54: Review loop — produce, review, rework, approve/reject
//
// Integration notes:
//   - runDispatchLoop replaces the function of the same name in delegate.go
//   - buildReviewPrompt replaces the function of the same name in delegate.go
//   - classifyResult in delegate.go needs one-line change:
//     domain.ClassificationMaxTurns → domain.ClassificationChangesRequested
//     at the "changes_requested:" check
//   - New function: extractAcceptanceCriteria
// ============================================================================

// runDispatchLoop runs the produce→review cycle for a delegated issue.
// Each round: produce phase (cheap model) → review phase (expensive model).
// Exits on APPROVED, BLOCKED/FATAL/AUTH/NO_CREDIT/RATE_LIMITED, or after MaxRounds.
// CHANGES_REQUESTED triggers a rework cycle (new produce with feedback).
// Persists state after each round so `mill watch` can observe progress.
func runDispatchLoop54(a *App, issueNum int, taskID string, opts adapter.DispatchOpts, issueBody string, labels []string, cfg config.Config) error {
	var finalClassification domain.Classification
	var finalCommits int
	var reviewFeedbacks []string

	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	task, ok := s.Task(taskID)
	if !ok {
		task = domain.NewTask(taskID, issueNum)
	}

	// Stage label determines produce model
	stageLabel := issue.StageLabel(labels)
	produceModel := a.resolveModel("", stageLabel, cfg)
	if stageLabel == "" {
		// No stage label: use the original model from opts
		produceModel = opts.Model
	}
	reviewModel := "laguna-pro"

	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 4
	}

	acceptanceCriteria := extractAcceptanceCriteria(issueBody)
	reworkFeedback := "" // feedback for the next produce phase

	for round := range maxRounds {
		task.Round = round

		// --- Produce phase ---
		produceOpts := opts
		if reworkFeedback != "" {
			produceOpts.Prompt = fmt.Sprintf("REWORK REQUESTED:\n%s\n\nOriginal task:\n%s", reworkFeedback, opts.Prompt)
		}
		produceOpts.Model = produceModel
		session, err := a.Adapter.Dispatch(produceOpts)
		if err != nil {
			recordError(a, s, issueNum, task, err, "failed to dispatch produce agent")
			return fmt.Errorf("failed to dispatch produce agent: %w", err)
		}

		produceResult, err := session.Wait()
		if err != nil {
			recordError(a, s, issueNum, task, err, "produce agent session failed")
			return fmt.Errorf("produce agent session failed: %w", err)
		}
		finalCommits = produceResult.Commits

		// Append produce ledger entry
		produceEntry := ledger.Entry{
			Timestamp: time.Now().UTC(),
			Issue:     issueNum,
			Event:     "produce",
			Status:    string(domain.TaskRunning),
			Round:     round,
		}
		if err := ledger.Append(a.ledgerPath(issueNum), produceEntry); err != nil {
			return fmt.Errorf("failed to append produce ledger entry: %w", err)
		}

		// Persist state after produce
		s.UpsertTask(task)
		if err := s.Save(a.statePath()); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		// --- Review phase ---
		reviewPrompt := buildReviewPrompt54(issueBody, produceResult.Output, acceptanceCriteria)
		reviewOpts := adapter.DispatchOpts{
			Worktree: opts.Worktree,
			Prompt:   reviewPrompt,
			Model:    reviewModel,
			MaxTurns: opts.MaxTurns,
			Budget:   opts.Budget,
		}
		reviewSession, err := a.Adapter.Dispatch(reviewOpts)
		if err != nil {
			recordError(a, s, issueNum, task, err, "failed to dispatch review agent")
			return fmt.Errorf("failed to dispatch review agent: %w", err)
		}

		reviewResult, err := reviewSession.Wait()
		if err != nil {
			recordError(a, s, issueNum, task, err, "review agent session failed")
			return fmt.Errorf("review agent session failed: %w", err)
		}

		finalClassification = classifyResult(reviewResult.ExitCode, reviewResult.Stderr)

		// Append review ledger entry with round number
		reviewEntry := ledger.Entry{
			Timestamp:      time.Now().UTC(),
			Issue:          issueNum,
			Event:          "review",
			Status:         string(domain.TaskRunning),
			Classification: string(finalClassification),
			Round:          round,
		}
		if err := ledger.Append(a.ledgerPath(issueNum), reviewEntry); err != nil {
			return fmt.Errorf("failed to append review ledger entry: %w", err)
		}

		// Persist state after review
		task.UpdatedAt = time.Now().UTC()
		s.UpsertTask(task)
		if err := s.Save(a.statePath()); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		// Decide next action based on classification
		switch finalClassification {
		case domain.ClassificationOK:
			// Approved — exit loop
			goto finish

		case domain.ClassificationChangesRequested:
			// Changes requested — collect feedback and rework if rounds remain
			reviewFeedbacks = append(reviewFeedbacks, reviewResult.Stderr)
			reworkFeedback = reviewResult.Stderr
			if round+1 >= maxRounds {
				// Max cycles exhausted — escalate to Staff
				goto escalate
			}
			continue

		case domain.ClassificationBlocked,
			domain.ClassificationAuth,
			domain.ClassificationNoCredit,
			domain.ClassificationRateLimited,
			domain.ClassificationFatal:
			// Non-recoverable — escalate to Staff
			fmt.Fprintf(a.Err, "ESCALATION: Review cycle aborted for issue %d — %s\n", issueNum, finalClassification)
			goto finish

		default:
			// Transient, MaxTurns, or unknown — continue to next round
			continue
		}
	}

escalate:
	// Build escalation summary with all review feedback
	fmt.Fprintf(a.Err, "ESCALATION: Review cycle exhausted for issue %d\n", issueNum)
	fmt.Fprintf(a.Err, "Issue body:\n%s\n\n", issueBody)
	if len(reviewFeedbacks) > 0 {
		fmt.Fprintf(a.Err, "Review feedback:\n")
		for i, fb := range reviewFeedbacks {
			fmt.Fprintf(a.Err, "Round %d:\n%s\n", i+1, fb)
		}
	}

finish:
	taskStatus := domain.TaskDone
	verdict := domain.VerdictApproved

	switch finalClassification {
	case domain.ClassificationOK:
		verdict = domain.VerdictApproved
	case domain.ClassificationChangesRequested:
		taskStatus = domain.TaskError
		verdict = domain.VerdictChangesRequested
	case domain.ClassificationBlocked,
		domain.ClassificationAuth,
		domain.ClassificationNoCredit,
		domain.ClassificationRateLimited,
		domain.ClassificationFatal:
		taskStatus = domain.TaskError
		verdict = domain.VerdictRejected
	default:
		taskStatus = domain.TaskError
		verdict = domain.VerdictRejected
	}

	task.UpdateStatus(taskStatus, verdict, finalCommits)
	s.UpsertTask(task)
	if err := s.Save(a.statePath()); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	completeEntry := ledger.Entry{
		Timestamp:      time.Now().UTC(),
		Issue:          issueNum,
		Event:          "complete",
		Status:         string(taskStatus),
		Verdict:        string(verdict),
		Classification: string(finalClassification),
		Round:          task.Round,
	}
	if err := ledger.Append(a.ledgerPath(issueNum), completeEntry); err != nil {
		return fmt.Errorf("failed to append ledger entry: %w", err)
	}

	return nil
}

// buildReviewPrompt constructs a review prompt that asks the reviewer agent
// to evaluate the produce agent's output against the issue body and acceptance criteria.
// issueBody is the original GitHub issue text; diffOutput is the produce agent's output;
// acceptanceCriteria is extracted from the issue body (may be nil or empty).
func buildReviewPrompt54(issueBody string, diffOutput string, acceptanceCriteria []string) string {
	var b strings.Builder

	b.WriteString("You are a code reviewer. Review the following code change against the acceptance criteria.\n\n")

	// Issue body
	b.WriteString("## Issue\n")
	if issueBody == "" {
		b.WriteString("(no issue body provided)\n\n")
	} else {
		b.WriteString(issueBody)
		b.WriteString("\n\n")
	}

	// Acceptance criteria
	b.WriteString("## Acceptance Criteria\n")
	if len(acceptanceCriteria) == 0 {
		b.WriteString("(no acceptance criteria provided — evaluate against best practices)\n\n")
	} else {
		for _, c := range acceptanceCriteria {
			b.WriteString("- ")
			b.WriteString(c)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Diff
	b.WriteString("## Changes (diff)\n")
	if diffOutput == "" {
		b.WriteString("(no diff available)\n\n")
	} else {
		b.WriteString(diffOutput)
		b.WriteString("\n\n")
	}

	// Output format template
	b.WriteString("Output your verdict on stderr as one of:\n")
	b.WriteString("- APPROVED: (work meets all acceptance criteria)\n")
	b.WriteString("- CHANGES_REQUESTED: (numbered, specific, criteria-referencing feedback items)\n")
	b.WriteString("- BLOCKED: (cannot proceed — missing info or external dependency)\n")
	b.WriteString("\n")

	// Quality rules
	b.WriteString("Quality rules:\n")
	b.WriteString("- Every CHANGES_REQUESTED item MUST reference a specific acceptance criterion\n")
	b.WriteString("- No vague feedback like \"this doesn't look right\"\n")
	b.WriteString("- If all criteria met, MUST output APPROVED\n")

	return b.String()
}

// extractAcceptanceCriteria parses acceptance criteria from an issue body.
// It looks for markdown checklist items (- [ ]) and returns them as a slice.
// Returns nil if no checklist items are found.
func extractAcceptanceCriteria(issueBody string) []string {
	var criteria []string
	for _, line := range strings.Split(issueBody, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "- [ ]"); ok {
			c := strings.TrimSpace(after)
			if c != "" {
				criteria = append(criteria, c)
			}
		}
	}
	return criteria
}

// recordError updates the task and ledger to reflect an agent failure.
// (Copied here to avoid import cycle; integration pass should deduplicate.)
func recordError(a *App, s state.State, issueNum int, task domain.Task, err error, event string) {
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
