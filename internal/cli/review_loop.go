package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
//   - classifyFailure in delegate.go maps results to FailureClass
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
// Produces and reviews are classified into a domain.FailureClass. Each class
// has a per-category FailureReactor that decides whether to rework (continue),
// escalate to a parent role, or abort.
// Persists state after each transition so `mill watch` can observe progress.
// Returns the final failure class and any error.
func runDispatchLoop54(a *App, issueNum int, taskID string, targetRole string, modelOverride string, opts adapter.DispatchOpts, issueBody string, labels []string, cfg config.Config, caps adapter.Capabilities) (domain.FailureClass, error) {
	var finalClass domain.FailureClass
	var finalCommits int
	var reviewFeedbacks []string
	var lastResult adapter.SessionResult

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
		produceResult, produceClass, perr := a.retryDispatch(produceOpts, "produce", issueNum, task, cfg)

		if perr != nil {
			finalClass = domain.EXECUTION_FAILURE
		} else {
			finalClass = produceClass
		}
		lastResult = produceResult

		// Append produce ledger entry (always, includes FailureClass/Phase/Role)
		produceEntry := ledger.Entry{
			Timestamp:      time.Now().UTC(),
			Issue:          issueNum,
			Event:          "produce",
			Status:         string(domain.TaskRunning),
			Classification: string(finalClass),
			FailureClass:   finalClass,
			Phase:          domain.TaskPhaseProduce,
			Role:           targetRole,
			Round:          round,
		}
		if lerr := ledger.Append(a.ledgerPath(issueNum), produceEntry); lerr != nil {
			return "", fmt.Errorf("failed to append produce ledger entry: %w", lerr)
		}
		// Persist state after produce
		s.UpsertTask(task)
		if serr := s.Save(a.statePath()); serr != nil {
			return "", fmt.Errorf("failed to save state: %w", serr)
		}

		// If produce succeeded, write artifact and run review phase.
		if finalClass == domain.CLASS_OK {
			finalCommits = produceResult.Commits

			// Write produce output to worktree for inspection.
			outputPath := filepath.Join(opts.Worktree, "output.txt")
			if werr := os.WriteFile(outputPath, []byte(produceResult.Output), 0644); werr != nil {
				fmt.Fprintf(a.Err, "warning: failed to write output.txt: %v\n", werr)
			}
			// Update agent_id for review phase so the pre-commit hook
			// knows which phase produced any commits.
			agentIDFile := filepath.Join(opts.Worktree, ".mill", "agent_id")
			os.WriteFile(agentIDFile, []byte("review"), 0644)

			// --- Review phase (with retry on transient) ---
			reviewPrompt := buildReviewPrompt54(issueBody, produceResult.Output, acceptanceCriteria, caps)
			reviewOpts := adapter.DispatchOpts{
				Worktree:   opts.Worktree,
				Prompt:     reviewPrompt,
				Model:      reviewModel,
				ModelChain: opts.ModelChain,
				MaxTurns:   opts.MaxTurns,
				Budget:     opts.Budget,
			}
			reviewResult, reviewClass, rerr := a.retryDispatch(reviewOpts, "review", issueNum, task, cfg)

			if rerr != nil {
				finalClass = domain.EXECUTION_FAILURE
			} else {
				finalClass = reviewClass
			}
			lastResult = reviewResult

			// Append review ledger entry (includes FailureClass/Phase/Role)
			reviewLedger := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "review",
				Status:         string(domain.TaskRunning),
				Classification: string(finalClass),
				FailureClass:   finalClass,
				Phase:          domain.TaskPhaseReview,
				Role:           targetRole,
				Round:          round,
			}
			if lerr := ledger.Append(a.ledgerPath(issueNum), reviewLedger); lerr != nil {
				return "", fmt.Errorf("failed to append review ledger entry: %w", lerr)
			}
			// Persist state after review
			task.UpdatedAt = time.Now().UTC()
			s.UpsertTask(task)
			if serr := s.Save(a.statePath()); serr != nil {
				return "", fmt.Errorf("failed to save state: %w", serr)
			}
		}

		// --- Per-category FailureReactor ---
		// Both produce-phase and review-phase failures are handled here.
		switch finalClass {
		case domain.CLASS_OK:
			// Approved — exit loop
			goto finish

		case domain.RESULT_FAILURE:
			// Changes requested — collect [criterion:] feedback and rework.
			changesCount++
			reviewFeedbacks = append(reviewFeedbacks, lastResult.Stderr)
			reworkFeedback = lastResult.Stderr
			if changesCount >= escalationThreshold {
				goto escalate
			}
			if round+1 >= maxRounds {
				goto escalate
			}
			// Record rework reaction ledger entry
			reworkEntry := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "rework",
				Status:         string(domain.TaskRunning),
				Classification: string(domain.RESULT_FAILURE),
				FailureClass:   domain.RESULT_FAILURE,
				Phase:          domain.TaskPhaseRework,
				Role:           targetRole,
				Round:          round,
			}
			ledger.Append(a.ledgerPath(issueNum), reworkEntry)
			continue

		case domain.GATE_FAILURE:
			// Rework the same role+worktree — feed gate stderr into next produce.
			reworkFeedback = lastResult.Stderr
			gateEntry := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "rework",
				Status:         string(domain.TaskRunning),
				Classification: string(domain.GATE_FAILURE),
				FailureClass:   domain.GATE_FAILURE,
				Phase:          domain.TaskPhaseGateFailed,
				Role:           targetRole,
				Round:          round,
			}
			ledger.Append(a.ledgerPath(issueNum), gateEntry)
			continue

		case domain.CONTRACT_FAILURE:
			// Reject the artifact — do NOT write output.txt or accept commits.
			// Reset reworkFeedback so no stale feedback leaks into next produce.
			reworkFeedback = ""
			rejectEntry := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "reject",
				Status:         string(domain.TaskRunning),
				Classification: string(domain.CONTRACT_FAILURE),
				FailureClass:   domain.CONTRACT_FAILURE,
				Phase:          domain.TaskPhaseRejected,
				Role:           targetRole,
				Round:          round,
			}
			ledger.Append(a.ledgerPath(issueNum), rejectEntry)
			continue

		case domain.EXECUTION_FAILURE:
			// Model-chain retry already happened inside retryDispatch; on
			// exhaustion, escalate to the next parent role.
			parentRole, escErr := a.escalateToParent(issueNum, targetRole, cfg)
			if escErr != nil {
				fmt.Fprintf(a.Err, "ESCALATION: escalate to parent failed for issue %d — %v\n", issueNum, escErr)
			} else if parentRole != "" {
				fmt.Fprintf(a.Err, "ESCALATION: issue %d escalated to %s\n", issueNum, parentRole)
			}
			escEntry := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "escalate",
				Status:         string(domain.TaskAborted),
				Classification: string(domain.EXECUTION_FAILURE),
				FailureClass:   domain.EXECUTION_FAILURE,
				Phase:          domain.TaskPhaseAborted,
				Role:           targetRole,
				Round:          round,
			}
			if lerr := ledger.Append(a.ledgerPath(issueNum), escEntry); lerr != nil {
				fmt.Fprintf(a.Err, "warning: failed to append escalation ledger entry: %v\n", lerr)
			}
			goto finish

		case domain.ENVIRONMENT_FAILURE:
			// Abort — preserve worktree+logs, notify reason, no retry.
			reason := lastResult.Stderr
			if reason == "" {
				reason = "environment failure during session"
			}
			fmt.Fprintf(a.Err, "ESCALATION: Environment failure for issue %d — %s\n", issueNum, reason)
			envEntry := ledger.Entry{
				Timestamp:      time.Now().UTC(),
				Issue:          issueNum,
				Event:          "abort",
				Status:         string(domain.TaskAborted),
				Classification: string(domain.ENVIRONMENT_FAILURE),
				FailureClass:   domain.ENVIRONMENT_FAILURE,
				Phase:          domain.TaskPhaseAborted,
				Role:           targetRole,
				Round:          round,
			}
			if lerr := ledger.Append(a.ledgerPath(issueNum), envEntry); lerr != nil {
				fmt.Fprintf(a.Err, "warning: failed to append abort ledger entry: %v\n", lerr)
			}
			goto finish

		default:
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

	switch finalClass {
	case domain.CLASS_OK:
		verdict = domain.VerdictApproved
	case domain.RESULT_FAILURE:
		taskStatus = domain.TaskError
		verdict = domain.VerdictChangesRequested
	default:
		taskStatus = domain.TaskError
		verdict = domain.VerdictRejected
	}

	task.Transition(domain.TaskPhaseReview, taskStatus, verdict, finalCommits, finalClass)
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
		Classification: string(finalClass),
		FailureClass:   finalClass,
		Phase:          domain.TaskPhaseReview,
		Role:           targetRole,
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

	return finalClass, nil
}

// waitSession wraps adapter.Session.Wait() with a heartbeat monitor goroutine.
// While the session is waiting, a goroutine samples the session's heartbeat
// file ModTime every ~1s (under a sync.Mutex). After Wait returns, the monitor
// is cancelled and joined. If the adapter's own HeartbeatStaleness is zero
// but the monitor measured a non-zero last beat, the monitor's measurement
// is used so the classifier can observe liveness.
func (a *App) waitSession(session adapter.Session) (adapter.SessionResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	hbPath := session.HeartbeatPath()

	var (
		mu       sync.Mutex
		lastBeat time.Time
	)
	done := make(chan struct{})

	go func() {
		defer close(done)
		if hbPath == "" {
			return
		}
		sample := func() {
			info, err := os.Stat(hbPath)
			if err == nil {
				mu.Lock()
				lastBeat = info.ModTime()
				mu.Unlock()
			}
		}
		sample()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sample()
			case <-ctx.Done():
				return
			}
		}
	}()

	result, err := session.Wait()
	cancel()
	<-done

	mu.Lock()
	staleness := time.Since(lastBeat)
	mu.Unlock()

	if result.HeartbeatStaleness == 0 && !lastBeat.IsZero() {
		result.HeartbeatStaleness = staleness
	}
	return result, err
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
	task.Transition(domain.TaskPhaseAborted, domain.TaskError, domain.VerdictRejected, 0, domain.EXECUTION_FAILURE)
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
