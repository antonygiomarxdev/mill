package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
//   - runDispatchLoop54 replaces the function of the same name in delegate.go
//   - buildReviewPrompt54 replaces the function of the same name in delegate.go
//   - classifyResult in delegate.go handles verdict parsing
//   - extractAcceptanceCriteria replaces issue.ExtractAcceptanceCriteria (local copy)
// ============================================================================

// escalationThreshold is the max number of consecutive CHANGES_REQUESTED
// verdicts before the review loop escalates to Staff.
const escalationThreshold = 3

// stageLabelToModel maps stage labels to hardcoded model strings.
// This bypasses role-based resolution; used for backward compat with
// issue labels like stage:produce or stage:review.
func stageLabelToModel(stageLabel string) string {
	switch stageLabel {
	case "stage:produce":
		return "laguna-free"
	case "stage:review":
		return "laguna-pro"
	case "stage:implement":
		return "laguna-free"
	default:
		return "laguna-free"
	}
}
// runDispatchLoop54 runs the produce→review cycle for a delegated issue.
// Each round: produce phase (cheap model) → review phase (expensive model).
// Exits on APPROVED, BLOCKED/FATAL/AUTH/NO_CREDIT/RATE_LIMITED,
// after MaxRounds, or after escalationThreshold consecutive CHANGES_REQUESTED.
// CHANGES_REQUESTED triggers a rework cycle (new produce with feedback).
// Persists state after each round so `mill watch` can observe progress.
// Returns the final classification and any error.
func runDispatchLoop54(a *App, issueNum int, taskID string, targetRole string, modelOverride string, opts adapter.DispatchOpts, issueBody string, labels []string, cfg config.Config, caps adapter.Capabilities) (domain.Classification, error) {
	var finalClassification domain.Classification
	var finalCommits int
	var reviewFeedbacks []string

	s, err := state.Load(a.statePath())
	if err != nil {
		return "", fmt.Errorf("failed to load state: %w", err)
	}
	task, ok := s.Task(taskID)
	if !ok {
		task = domain.NewTask(taskID, issueNum)
	}

	// Resolve models for produce and review phases.
	// Stage labels bypass role-based resolution.
	stageLabel := issue.StageLabel(labels)
	var produceModel, reviewModel string
	if stageLabel != "" {
		produceModel = stageLabelToModel(stageLabel)
	} else {
		produceModel, err = a.resolveModel(targetRole, modelOverride, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to resolve produce model: %w", err)
		}
	}
	reviewModel, err = a.resolveModel("reviewer", modelOverride, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to resolve review model: %w", err)
	}

	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 4
	}

	acceptanceCriteria := extractAcceptanceCriteria(issueBody)
	reworkFeedback := "" // feedback for the next produce phase
	changesCount := 0    // consecutive CHANGES_REQUESTED verdicts

	for round := range maxRounds {
		task.Round = round

		// --- Produce phase (with retry on transient) ---
		produceOpts := opts
		if reworkFeedback != "" {
			produceOpts.Prompt = fmt.Sprintf("REWORK REQUESTED:\n%s\n\nOriginal task:\n%s", reworkFeedback, opts.Prompt)
		}
		produceOpts.Model = produceModel
		produceResult, produceClass, err := a.retryDispatch(produceOpts, "produce", issueNum, task, cfg)
		if err != nil {
			finalClassification = domain.ClassificationFatal
			goto finish
		}
		finalCommits = produceResult.Commits

		// Write produce output to worktree for inspection.
		outputPath := filepath.Join(opts.Worktree, "output.txt")
		if werr := os.WriteFile(outputPath, []byte(produceResult.Output), 0644); werr != nil {
			fmt.Fprintf(a.Err, "warning: failed to write output.txt: %v\n", werr)
		}

		// Check for non-recoverable produce failures (FATAL, AUTH, etc.).
		// These skip the review phase and exit immediately.
		if produceClass == domain.ClassificationFatal ||
			produceClass == domain.ClassificationAuth ||
			produceClass == domain.ClassificationNoCredit ||
			produceClass == domain.ClassificationBlocked ||
			produceClass == domain.ClassificationRateLimited {
			finalClassification = produceClass
			goto finish
		}


		// Append produce ledger entry
		produceEntry := ledger.Entry{
			Timestamp: time.Now().UTC(),
			Issue:     issueNum,
			Event:     "produce",
			Status:    string(domain.TaskRunning),
			Round:     round,
		}
		if err := ledger.Append(a.ledgerPath(issueNum), produceEntry); err != nil {
			return "", fmt.Errorf("failed to append produce ledger entry: %w", err)
		}
		// Persist state after produce
		s.UpsertTask(task)
		if err := s.Save(a.statePath()); err != nil {
			return "", fmt.Errorf("failed to save state: %w", err)
		}
		// Update agent_id for review phase so the pre-commit hook
		// knows which phase produced any commits.
		agentIDFile := filepath.Join(opts.Worktree, ".mill", "agent_id")
		os.WriteFile(agentIDFile, []byte("review"), 0644)
		// --- Review phase (with retry on transient) ---
		reviewPrompt := buildReviewPrompt54(issueBody, produceResult.Output, acceptanceCriteria, caps)
		reviewOpts := adapter.DispatchOpts{
			Worktree: opts.Worktree,
			Prompt:   reviewPrompt,
			Model:    reviewModel,
			MaxTurns: opts.MaxTurns,
			Budget:   opts.Budget,
		}
		reviewResult, reviewClass, err := a.retryDispatch(reviewOpts, "review", issueNum, task, cfg)
		if err != nil {
			finalClassification = domain.ClassificationFatal
			goto finish
		}
		finalClassification = reviewClass


		// Append review ledger entry with round number
		reviewLedger := ledger.Entry{
			Timestamp:      time.Now().UTC(),
			Issue:          issueNum,
			Event:          "review",
			Status:         string(domain.TaskRunning),
			Classification: string(finalClassification),
			Round:          round,
		}
		if err := ledger.Append(a.ledgerPath(issueNum), reviewLedger); err != nil {
			return "", fmt.Errorf("failed to append review ledger entry: %w", err)
		}
		// Persist state after review
		task.UpdatedAt = time.Now().UTC()
		s.UpsertTask(task)
		if err := s.Save(a.statePath()); err != nil {
			return "", fmt.Errorf("failed to save state: %w", err)
		}

		// Decide next action based on classification
		switch finalClassification {
		case domain.ClassificationOK:
			// Approved — exit loop
			goto finish

		case domain.ClassificationChangesRequested:
			// Changes requested — collect feedback and rework if rounds remain
			changesCount++
			reviewFeedbacks = append(reviewFeedbacks, reviewResult.Stderr)
			reworkFeedback = reviewResult.Stderr
			if changesCount >= escalationThreshold {
				// Escalation threshold hit — escalate to Staff
				goto escalate
			}
			if round+1 >= maxRounds {
				// Max rounds exhausted — escalate to Staff
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
		return "", fmt.Errorf("failed to save state: %w", err)
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
		return "", fmt.Errorf("failed to append ledger entry: %w", err)
	}

	// Check enforcement log for blocked commits → auto-label needs:rework
	enforceLogPath := filepath.Join(opts.Worktree, ".mill", "enforcement.log")
	if data, logErr := os.ReadFile(enforceLogPath); logErr == nil {
		if strings.Contains(string(data), "BLOCKED") {
			if labelErr := issue.AddLabel(issueNum, "needs:rework"); labelErr != nil {
				fmt.Fprintf(a.Err, "warning: failed to add needs:rework label: %v\n", labelErr)
			} else {
				fmt.Fprintf(a.Err, "enforcement: added needs:rework label to #%d\n", issueNum)
			}
		}
	}

	return finalClassification, nil
}

// buildReviewPrompt constructs a review prompt that asks the reviewer agent
// to evaluate the produce agent's output against the issue body and acceptance criteria.
func buildReviewPrompt54(issueBody string, diffOutput string, acceptanceCriteria []string, caps adapter.Capabilities) string {
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

	// Output format — structured plaintext header protocol
	b.WriteString("Output your verdict on stderr using this exact format:\n")
	b.WriteString("\n")
	b.WriteString("If all criteria are met:\n")
	b.WriteString("APPROVED:\n")
	b.WriteString("(explanation of why all criteria are satisfied)\n")
	b.WriteString("\n")
	b.WriteString("If changes are needed:\n")
	b.WriteString("CHANGES_REQUESTED:\n")
	b.WriteString("1. [criterion: \"exact criterion text\"] Specific, actionable feedback.\n")
	b.WriteString("2. [criterion: \"exact criterion text\"] Another specific issue.\n")
	b.WriteString("\n")
	b.WriteString("If blocked by external dependency:\n")
	b.WriteString("BLOCKED:\n")
	b.WriteString("(what is missing)\n")
	b.WriteString("\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Every CHANGES_REQUESTED item MUST start with [criterion: \"...\"]\n")
	b.WriteString("  containing the verbatim acceptance criterion it addresses.\n")
	b.WriteString("- Vague feedback without a criterion reference is INVALID.\n")
	b.WriteString("- APPROVED is the ONLY valid verdict when all criteria pass.\n")
	b.WriteString("  Do NOT write \"APPROVED but note X\" — if anything is wrong, use CHANGES_REQUESTED.\n")
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
func recordError(a *App, s state.State, issueNum int, task domain.Task, err error, event string) {
	task.UpdateStatus(domain.TaskError, domain.VerdictRejected, 0)
	s.UpsertTask(task)
	if saveErr := s.Save(a.statePath()); saveErr != nil {
		fmt.Fprintf(a.Err, "warning: failed to save state after error: %v\n", saveErr)
	}
	entry := ledger.Entry{
		Timestamp: time.Now().UTC(),
		Issue:     issueNum,
		Event:     event,
		Status:    string(domain.TaskError),
	}
	if saveErr := ledger.Append(a.ledgerPath(issueNum), entry); saveErr != nil {
		fmt.Fprintf(a.Err, "warning: failed to write ledger after error: %v\n", saveErr)
	}
}

