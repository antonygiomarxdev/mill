package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/ledger"
	"github.com/antonygiomarxdev/mill/internal/role"
	"github.com/antonygiomarxdev/mill/internal/slots"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// runDelegate handles the "delegate <issue>" command.
// It creates a task, dispatches an AI agent via the configured adapter,
// waits for completion, classifies the result, and persists state + ledger entries.
// Classification drives the retry/abort policy:
// OK/MAX_TURNS→done, AUTH/NO_CREDIT→abort, RATE_LIMITED/TRANSIENT→backoff+retry,
// BLOCKED→persist+stop, FATAL→retry.
func (a *App) runDelegate(args []string) error {
	// Extract --role and --model before flag parsing since Go's flag
	// package stops at the first positional argument.
	// Note: --priority is a boolean flag handled by the FlagSet directly.
	roleName, args := extractFlag(args, "role")
	modelFlag, args := extractFlag(args, "model")

	fs := flag.NewFlagSet("delegate", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var model string
	var wait bool
	var priority bool
	var maxTurns int
	var logLevelFlag string
	fs.BoolVar(&priority, "priority", false, "preempt next available slot (staff only)")
	fs.StringVar(&model, "model", modelFlag, "model to use (default: from config)")
	fs.IntVar(&maxTurns, "max-turns", 100, "max conversation turns")
	fs.BoolVar(&wait, "wait", false, "wait for agent to finish (default: async)")
	fs.StringVar(&logLevelFlag, "log-level", "", "log level (debug|info|warn|error)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	fsArgs := fs.Args()
	if len(fsArgs) < 1 {
		fs.Usage()
		return fmt.Errorf("usage: mill delegate <issue> [--role <role>]")
	}

	issueNum, err := issue.Parse(fsArgs[0])
	if err != nil {
		return err
	}
	// Validate mill.yml and capture concurrency settings.
	myml, err := config.LoadAndValidate("mill.yml")
	if err != nil {
		return err
	}

	// Initialize the recursive delegation engine only when mill.yml
	// configures a recursion section; otherwise it stays nil.
	a.initRecursion(myml)

	// Read issue body and labels from GitHub
	issueBody, labels, err := a.IssueReader(issueNum)
	if err != nil {
		return fmt.Errorf("failed to read issue #%d: %w", issueNum, err)
	}
	ac := issue.ExtractAcceptanceCriteria(issueBody)
	stageLabel := issue.StageLabel(labels)
	if stageLabel != "" {
		// Warn if multiple stage:* labels exist
		count := 0
		for _, l := range labels {
			if strings.HasPrefix(l, "stage:") {
				count++
			}
		}
		if count > 1 {
			fmt.Fprintf(a.Err, "warning: multiple stage:* labels on issue #%d, using %q\n", issueNum, stageLabel)
		}
	}

	// Determine active role from .mill/role (defaults to "staff")
	activeRole := a.readActiveRole()

	// If --role specified, validate delegation chain
	var targetRole string
	if roleName != "" {
		targetRole = roleName
		if err := a.validateDelegation(activeRole, targetRole); err != nil {
			return err
		}
	} else {
		targetRole = activeRole
	}

	// Load config for default model
	cfg, err := a.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create task and persist initial state so environment failures during
	// binary validation can be recorded against the task.
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

	// Validate required binaries before proceeding. On a missing binary the
	// task is marked aborted + ENVIRONMENT_FAILURE and persisted; delegation
	// stops without a direct error.
	if err := a.validateDelegateBinaries(cfg, &task, s); err != nil {
		return fmt.Errorf("failed to record environment failure: %w", err)
	}

	// Query adapter capabilities before side effects (spec: eager, before worktree)
	caps := a.Adapter.Capabilities()
	fmt.Fprintf(a.Err, "delegate: adapter capabilities — models=%d selectors=%v recovery=%v line_ceiling=%d byte_ceiling=%d\n",
		len(caps.Models), caps.ReadTool.HasSelectorSupport, caps.ReadTool.HasRecoveryNotes,
		caps.ReadTool.LineCeiling, caps.ReadTool.ByteCeiling)

	// Resolve and validate model fallback chain
	modelChain, err := a.resolveModelChain(model, cfg)
	if err != nil {
		return err
	}
	if err := validateModelChain(modelChain, caps); err != nil {
		return err
	}

	// Set up per-issue structured logging: JSON to .mill/logs/<issue>.jsonl
	// and human-readable text to stderr. Falls back to the default logger
	// (text to stderr) when the file cannot be opened.
	var logFile *os.File
	{
		level := slogLevelFromString(logLevelFlag)
		if logLevelFlag == "" {
			level = logLevelFromEnv()
		}
		issueLogger, lf, lerr := a.newIssueLogger(issueNum, level)
		if lerr != nil {
			fmt.Fprintf(a.Err, "warning: failed to open log file for issue %d: %v\n", issueNum, lerr)
			logFile = nil
		} else {
			a.Logger = issueLogger
			logFile = lf
		}
	}

	// Record binary provenance once per run so a stale binary is visible
	// in the log (#137).
	a.logger().Info("mill run started",
		slog.String("binary", binaryProvenance()),
		slog.Int("issue", issueNum),
		slog.String("role", targetRole),
		slog.String("model", modelChain[0]),
	)

	// Initialize slot manager if not already set.
	// mill.yml's concurrency.max-slots takes precedence over config.json.
	if a.slots == nil {
		maxSlots := MaxSlotsFromConfig(cfg)
		if myml.Concurrency.MaxSlots > 0 {
			maxSlots = myml.Concurrency.MaxSlots
		}
		a.slots = slots.NewManager(maxSlots)
	}

	// Validate --priority flag (staff only).
	if err := ValidatePriority(priority, activeRole); err != nil {
		return err
	}

	modelOverride := model

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
	// Create a real git worktree for branch isolation.
	wt, err := a.createWorktree(issueNum)
	if err != nil {
		return fmt.Errorf("failed to create worktree for issue #%d: %w", issueNum, err)
	}

	// Deferred cleanup: if the agent fails irrecoverably (FATAL/AUTH/NO_CREDIT),
	// remove the worktree and prune the branch. Best-effort.
	irrecoverable := false
	defer func() {
		if irrecoverable {
			a.cleanupWorktree(issueNum)
		}
	}()

	// Scaffold context files so the agent finds AGENTS.md, .omp/, roles/
	if err := a.copyScaffold(wt, initConfig{}, false); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to scaffold worktree: %v\n", err)
	}
	// Write .mill/role so the agent knows its role
	roleFile := filepath.Join(wt, ".mill", "role")
	if err := os.MkdirAll(filepath.Dir(roleFile), 0755); err == nil {
		os.WriteFile(roleFile, []byte(targetRole), 0644)
	}
	// Write .mill/agent_id so the pre-commit hook knows the dispatch phase
	agentIDFile := filepath.Join(wt, ".mill", "agent_id")
	if err := os.MkdirAll(filepath.Dir(agentIDFile), 0755); err == nil {
		os.WriteFile(agentIDFile, []byte("produce"), 0644)
	}
	if err := installHooks(wt); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to install hooks: %v\n", err)
	}
	if err := installRoleEnforceHook(wt); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to install role-enforce hook: %v\n", err)
	}

	// Build issue context prompt with full body, acceptance criteria, and role
	prompt := buildIssueContextPrompt(issueNum, issueBody, ac, targetRole, caps)
	opts := adapter.DispatchOpts{
		Worktree:   wt,
		Prompt:     prompt,
		Model:      modelChain[0],
		ModelChain: modelChain,
		MaxTurns:   maxTurns,
		Budget:     cfg.Budget,
	}
	// Record the dispatched prompt (hash + length) so an agent that failed
	// can be distinguished from a brief that never arrived.
	a.logger().Info("dispatch",
		slog.String("phase", "produce"),
		slog.Int("issue", issueNum),
		slog.String("role", targetRole),
		slog.String("model", opts.Model),
		slog.Int("max_turns", opts.MaxTurns),
		slog.Int("prompt_length", len(prompt)),
		slog.String("prompt_sha256", promptHash(prompt)),
	)
	// Acquire slot before dispatching (blocks if all slots occupied).
	ctx := context.Background()
	maxSlots := MaxSlotsFromConfig(cfg)
	if myml.Concurrency.MaxSlots > 0 {
		maxSlots = myml.Concurrency.MaxSlots
	}
	if _, err := AcquireSlot(ctx, a.slots, a.Err, issueNum, targetRole, priority, maxSlots); err != nil {
		if errors.Is(err, ErrSlotsExhausted) {
			// Slot exhaustion is an environment failure: abort with no retry
			// and no indefinite block. The "slots agotados" notification was
			// already emitted by AcquireSlot.
			task.Transition(domain.TaskPhaseAborted, domain.TaskAborted, domain.VerdictRejected, 0, domain.ENVIRONMENT_FAILURE)
			task.AbortReason = "slots agotados"
			s.UpsertTask(task)
			if saveErr := s.Save(a.statePath()); saveErr != nil {
				fmt.Fprintf(a.Err, "warning: failed to save state after slot exhaustion: %v\n", saveErr)
			}
			ledger.Append(a.ledgerPath(issueNum), ledger.Entry{
				Timestamp:    time.Now().UTC(),
				Issue:        issueNum,
				Event:        "slots_exhausted",
				Status:       string(domain.TaskAborted),
				FailureClass: domain.ENVIRONMENT_FAILURE,
				Phase:        domain.TaskPhaseAborted,
				Role:         targetRole,
			})
			return nil
		}
		return fmt.Errorf("slot acquisition failed: %w", err)
	}

	if wait {
		defer a.slots.Release()
		if logFile != nil {
			defer logFile.Close()
		}
		classification, err := runDispatchLoop54(a, issueNum, taskID, targetRole, modelOverride, opts, issueBody, labels, cfg, caps)
		// Commit agent output to the worktree branch so review and rework
		// operate on committed state, whatever the verdict.
		a.commitWorktreeOnComplete(issueNum, wt)
		if isIrrecoverable(classification) {
			irrecoverable = true
		}
		// Post-loop recursion handoff: when the produce-review loop ends with
		// CLASS_OK and the target role delegates to subordinates, hand the
		// worktree off to the recursive delegation engine.
		if err == nil && classification == domain.CLASS_OK {
			if rerr := a.runRecursionHandoff(targetRole, wt); rerr != nil {
				return rerr
			}
		}
		return err
	}

	// Async: fire and forget. Agent runs in background goroutine.
	go func() {
		defer a.slots.Release()
		if logFile != nil {
			defer logFile.Close()
		}
		classification, _ := runDispatchLoop54(a, issueNum, taskID, targetRole, modelOverride, opts, issueBody, labels, cfg, caps)
		// Commit agent output to the worktree branch so review and rework
		// operate on committed state, whatever the verdict.
		a.commitWorktreeOnComplete(issueNum, wt)
		if isIrrecoverable(classification) {
			a.cleanupWorktree(issueNum)
		}
	}()
	fmt.Fprintf(a.Out, "Delegated issue %d — task %s (async)\n", issueNum, taskID)
	return nil
}

// isIrrecoverable reports whether a failure class indicates the worktree
// should be cleaned up. EXECUTION_FAILURE (which subsumes the old FATAL,
// AUTH, and NO_CREDIT classifications) triggers cleanup; environment
// failures preserve the worktree for post-mortem.
func isIrrecoverable(fc domain.FailureClass) bool {
	return fc == domain.EXECUTION_FAILURE
}

// runRecursionHandoff triggers the recursive delegation engine when the
// target role delegates to subordinates. It is a no-op when recursion is
// not configured (a.Recursion nil) or the role is a leaf (empty
// delegates_to). The result is logged to a.Err; the artifact path points at
// the produce-loop's output.txt in the worktree.
func (a *App) runRecursionHandoff(targetRole, worktreePath string) error {
	if a.Recursion == nil {
		return nil
	}
	rolePath := filepath.Join(a.MillDir, "roles", targetRole, "ROLE.md")
	fm, err := role.ParseFrontmatter(rolePath)
	if err != nil {
		return fmt.Errorf("recursion: cannot read role %s: %w", targetRole, err)
	}
	if len(fm.DelegatesTo) == 0 {
		return nil
	}
	res, derr := a.Recursion.Delegate(targetRole, worktreePath, filepath.Join(worktreePath, "output.txt"))
	if res != nil {
		fmt.Fprintf(a.Err, "recursion: %s → %s (depth %d, %s)\n", targetRole, res.Role, res.Depth, res.Failure)
	}
	if derr != nil {
		return fmt.Errorf("recursion: %w", derr)
	}
	return nil
}

// createWorktree creates a real git worktree at .mill/worktrees/issue-N
// on branch agent/N, rooted at the current HEAD. If the worktree directory
// already exists (e.g. from a prior crash), it calls cleanupWorktree first
// (idempotent retry), then creates fresh. After creation, it writes a PID
// file at .mill/worktrees/issue-N/.mill/agent.pid. Returns the worktree path.
func (a *App) createWorktree(issueNum int) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not found in PATH; git is required for worktree isolation")
	}

	wt := a.worktreePath(issueNum)
	branch := a.worktreeBranch(issueNum)

	// Idempotent retry: if worktree directory already exists from a prior
	// crash, clean it up first, then create fresh.
	if info, err := os.Stat(wt); err == nil && info.IsDir() {
		a.cleanupWorktree(issueNum)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, wt)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %w\n%s", err, out)
	}

	// Write PID file for #70 stale-lock detection.
	pidDir := filepath.Join(wt, ".mill")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .mill dir in worktree: %w", err)
	}
	pidFile := filepath.Join(pidDir, "agent.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return "", fmt.Errorf("failed to write PID file: %w", err)
	}

	return wt, nil
}

// worktreeHasChanges reports whether the worktree at wt has any uncommitted
// changes: staged, unstaged, or untracked files.
func worktreeHasChanges(wt string) (bool, error) {
	cmd := exec.Command("git", "-C", wt, "status", "--porcelain", "--untracked-files=all")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w\n%s", err, out)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// scratchBranchName returns the name of the scratch branch used to preserve
// uncommitted changes from a worktree before it is removed.
func scratchBranchName(issueNum int) string {
	return fmt.Sprintf("scratch/%d", issueNum)
}

// preserveChanges checks whether the worktree for issueNum has uncommitted
// changes. If so, it stages and commits them so they are not lost when the
// worktree is removed. Returns true if changes were committed.
func (a *App) preserveChanges(issueNum int, wt string) bool {
	hasChanges, err := worktreeHasChanges(wt)
	if err != nil {
		fmt.Fprintf(a.Err, "warning: could not check worktree status for issue %d: %v\n", issueNum, err)
		return false
	}
	if !hasChanges {
		return false
	}

	addCmd := exec.Command("git", "-C", wt, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(a.Err, "warning: git add failed while preserving changes for issue %d: %v\n%s\n", issueNum, err, out)
		return false
	}

	commitMsg := fmt.Sprintf("scratch: preserve uncommitted changes from issue #%d delegation", issueNum)
	commitCmd := exec.Command("git", "-C", wt, "commit", "--no-verify", "-m", commitMsg)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "nothing to commit") {
			return false
		}
		fmt.Fprintf(a.Err, "warning: git commit failed while preserving changes for issue %d: %v\n%s\n", issueNum, err, out)
		return false
	}

	return true
}

// cleanupWorktree removes the git worktree and prunes its branch.
// Before removal, any uncommitted changes are committed (via preserveChanges)
// and a scratch branch (scratch/N) is created from the worktree HEAD so that
// uncommitted work is not silently lost. It is best-effort: errors are logged
// to a.Err, not propagated. Used as deferred cleanup when an agent session
// fails irrecoverably, and during idempotent worktree re-creation.
func (a *App) cleanupWorktree(issueNum int) {
	wt := a.worktreePath(issueNum)
	branch := a.worktreeBranch(issueNum)

	// Preserve any uncommitted changes before removing the worktree.
	// If changes were committed, snapshot the worktree HEAD on a scratch ref
	// so they survive the agent-branch deletion.
	if a.preserveChanges(issueNum, wt) {
		scratch := scratchBranchName(issueNum)
		scratchCmd := exec.Command("git", "-C", wt, "branch", "-f", scratch)
		if out, err := scratchCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(a.Err, "warning: could not create scratch branch %s: %v\n%s\n", scratch, err, out)
		} else {
			fmt.Fprintf(a.Err, "Preserving uncommitted changes in branch %s (inspect with: git log %s)\n", scratch, scratch)
		}
	}

	// Remove PID file.
	pidFile := filepath.Join(wt, ".mill", "agent.pid")
	os.Remove(pidFile)

	// Remove the worktree.
	cmd := exec.Command("git", "worktree", "remove", "--force", wt)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(a.Err, "cleanup: git worktree remove failed: %v\n%s\n", err, out)
	}

	// Prune the branch.
	cmd = exec.Command("git", "branch", "-D", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(a.Err, "cleanup: git branch -D failed: %v\n%s\n", err, out)
	}
}

// commitWorktreeOnComplete commits all changes in the worktree to the agent
// branch after a delegation loop finishes, regardless of verdict. This
// ensures review operates on committed state and rework starts from a known
// point. Changes already committed by the agent are left untouched.
func (a *App) commitWorktreeOnComplete(issueNum int, wt string) {
	hasChanges, err := worktreeHasChanges(wt)
	if err != nil {
		fmt.Fprintf(a.Err, "warning: could not check worktree status for commit: %v\n", err)
		return
	}
	if !hasChanges {
		return
	}

	addCmd := exec.Command("git", "-C", wt, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(a.Err, "warning: git add failed during completion commit for issue %d: %v\n%s\n", issueNum, err, out)
		return
	}

	commitMsg := fmt.Sprintf("delegate: commit agent output for issue #%d", issueNum)
	commitCmd := exec.Command("git", "-C", wt, "commit", "--no-verify", "-m", commitMsg)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "nothing to commit") {
			fmt.Fprintf(a.Err, "warning: git commit failed during completion for issue %d: %v\n%s\n", issueNum, err, out)
		}
		return
	}

	revCmd := exec.Command("git", "-C", wt, "rev-parse", "--short", "HEAD")
	if revOut, err := revCmd.Output(); err == nil {
		fmt.Fprintf(a.Err, "Committed agent output for issue #%d (%s)\n", issueNum, strings.TrimSpace(string(revOut)))
	}
}

// recordError updates the task and ledger to reflect an agent failure.
func (a *App) recordError(s state.State, issueNum int, task domain.Task, err error, event string) {
	task.Transition(domain.TaskPhaseAborted, domain.TaskError, domain.VerdictRejected, 0, domain.EXECUTION_FAILURE)
	s.UpsertTask(task)
	s.Save(a.statePath())

	ledger.Append(a.ledgerPath(issueNum), ledger.Entry{
		Timestamp: time.Now().UTC(),
		Issue:     issueNum,
		Event:     event,
		Status:    string(domain.TaskError),
	})
}

// retryDispatch wraps Adapter.Dispatch + session.Wait + classifyFailure with
// model-chain fallback on execution failures.
// Models are tried in order from opts.ModelChain; on EXECUTION_FAILURE the
// dispatcher advances to the next model, bounded by Config.MaxRetries
// (default 4). When retries are exhausted the dispatcher returns the last
// result with EXECUTION_FAILURE so the caller can escalate. Any other
// FailureClass returns immediately. When the chain is empty, only opts.Model
// is tried.
func (a *App) retryDispatch(
	opts adapter.DispatchOpts,
	phase string,
	issueNum int,
	task domain.Task,
	targetRole string,
	cfg config.Config,
) (adapter.SessionResult, domain.FailureClass, error) {
	chain := opts.ModelChain
	// Prepend opts.Model so the role-resolved model is tried first,
	// then the config/adapter chain provides fallback.
	allModels := make([]string, 0, len(chain)+1)
	allModels = append(allModels, opts.Model)
	for _, m := range chain {
		if m != opts.Model {
			allModels = append(allModels, m)
		}
	}

	// Bound the number of model attempts at Config.MaxRetries (default 4) so
	// execution failures are retried a finite number of times before escalation.
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 4
	}
	if len(allModels) > maxRetries {
		allModels = allModels[:maxRetries]
	}

	var aggErrs []string

	for i, model := range allModels {
		if i > 0 {
			fmt.Fprintf(a.Err, "model %s execution failed, falling back to %s (%d/%d)\n",
				allModels[i-1], model, i+1, len(allModels))
		}
		opts.Model = model

		session, err := a.Adapter.Dispatch(opts)
		if err != nil {
			if isTransientError(err) {
				aggErrs = append(aggErrs, fmt.Sprintf("%s: %v", model, err))
				continue
			}
			return adapter.SessionResult{}, domain.EXECUTION_FAILURE, fmt.Errorf("%s dispatch failed: %w", phase, err)
		}

		result, err := a.waitSession(session)
		if err != nil {
			if isTransientError(err) {
				aggErrs = append(aggErrs, fmt.Sprintf("%s: %v", model, err))
				continue
			}
			return adapter.SessionResult{}, domain.EXECUTION_FAILURE, fmt.Errorf("%s session failed: %w", phase, err)
		}

		// Persist the full SessionResult per dispatch attempt.
		a.logSessionResult(phase, model, targetRole, task.Round, result)

		// Auto-compact session context if enabled and near threshold.
		a.maybeAutoCompactSession(session, opts.Model, issueNum, opts.Worktree, cfg)

		fc := classifyFailure(toSessionResult(result))
		if fc == domain.EXECUTION_FAILURE {
			aggErrs = append(aggErrs, fmt.Sprintf("%s: %s", model, fc))
			continue
		}
		return result, fc, nil
	}

	errMsg := fmt.Sprintf("%s: all %d models exhausted", phase, len(allModels))
	if len(aggErrs) > 0 {
		errMsg += ": " + strings.Join(aggErrs, "; ")
	}
	return adapter.SessionResult{}, domain.EXECUTION_FAILURE, fmt.Errorf("%s", errMsg)
}

// If the resolved value is an alias:
//   - config.Models[alias] → single-element chain
//   - adapter.DefaultFallbackChain()[alias] → multi-element chain
//   - passthrough: alias IS the model name → single-element chain
func (a *App) resolveModelChain(modelFlag string, cfg config.Config) ([]string, error) {
	modelName := modelFlag
	if modelName == "" {
		modelName = cfg.Model
	}
	if modelName == "" {
		modelName = a.Adapter.DefaultModel()
	}
	if modelName == "" {
		return nil, fmt.Errorf("no model configured: set model in config or use --model flag")
	}

	// Alias resolution
	if mapped, ok := cfg.Models[modelName]; ok && mapped != "" {
		return []string{mapped}, nil
	}
	if chain := a.Adapter.DefaultFallbackChain()[modelName]; len(chain) > 0 {
		return chain, nil
	}

	// Passthrough: the value IS the model name
	return []string{modelName}, nil
}

// validateModelChain checks that every model in chain exists in the adapter's
// Capabilities().Models. Returns an error listing any invalid models.
func validateModelChain(chain []string, caps adapter.Capabilities) error {
	modelSet := make(map[string]bool, len(caps.Models))
	for _, m := range caps.Models {
		modelSet[m] = true
	}
	var invalid []string
	for _, m := range chain {
		if !modelSet[m] {
			invalid = append(invalid, m)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("model %s not supported by provider. Available: %s",
			strings.Join(invalid, ", "), strings.Join(caps.Models, ", "))
	}
	return nil
}

// isTransientError reports whether an error from Adapter.Dispatch or
// session.Wait is likely transient (network-related) and worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "econnrefused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "network")
}

// readActiveRole reads the current active role from .mill/role.
// Returns "staff" if no role file exists (backward compat).
func (a *App) readActiveRole() string {
	roleFile := filepath.Join(a.MillDir, "role")
	data, err := os.ReadFile(roleFile)
	if err != nil {
		return "staff"
	}
	role := strings.TrimSpace(string(data))
	if role == "" {
		return "staff"
	}
	return role
}

// projectRoot walks up from the current working directory to find the
// project root, identified by the presence of go.mod or mill.yml.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "mill.yml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found (no go.mod or mill.yml)")
		}
		dir = parent
	}
}

// validateDelegation checks that the active role can delegate to the target role.
// Reads the active role's ROLE.md frontmatter to get delegates_to.
func (a *App) validateDelegation(activeRole, targetRole string) error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("cannot find project root: %w", err)
	}
	rolePath := filepath.Join(root, ".mill", "roles", activeRole, "ROLE.md")
	fm, err := role.ParseFrontmatter(rolePath)
	if err != nil {
		return fmt.Errorf("cannot read role %s: %w", activeRole, err)
	}

	// If no delegates_to defined, the role cannot delegate
	if len(fm.DelegatesTo) == 0 {
		return fmt.Errorf("%s has no delegation targets. Valid: none", activeRole)
	}

	// Check if target is in the delegates_to list
	for _, allowed := range fm.DelegatesTo {
		if allowed == targetRole {
			return nil
		}
	}

	validList := "'" + strings.Join(fm.DelegatesTo, "', '") + "'"
	return fmt.Errorf("%s delegates to %s, not %s. Valid targets: %s",
		activeRole,
		strings.Join(fm.DelegatesTo, ", "),
		targetRole,
		validList,
	)
}

// buildRolePrompt constructs a role-aware prompt for a given issue and target role.
// When issueBody is non-empty, it is appended as context for the agent.
func buildRolePrompt(issueNum int, targetRole string, issueBody string) string {
	root, err := projectRoot()
	if err != nil {
		root = "."
	}
	rolePrompt, err := role.LoadFrom(root, targetRole)
	if err != nil {
		// Fall back to generic prompt if role can't be loaded
		prompt := fmt.Sprintf(`You are mill, an agent delegation harness.
Role: %s
Work on GitHub issue #%d.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.`, targetRole, issueNum)
		if issueBody != "" {
			prompt += fmt.Sprintf("\n\n**Issue Body:**\n%s", issueBody)
		}
		return prompt
	}

	prompt := fmt.Sprintf("%s\n\n---\n\nWork on GitHub issue #%d.\n\nRead the codebase, make the necessary changes, and when you are done,\nend your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.", rolePrompt, issueNum)
	if issueBody != "" {
		prompt += fmt.Sprintf("\n\n**Issue Body:**\n%s", issueBody)
	}
	return prompt
}

// buildPrompt constructs the query passed to the agent for a given issue.
// Deprecated: use buildRolePrompt for role-aware prompts.
func buildPrompt(issueNum int) string {
	return fmt.Sprintf(`You are mill, an agent delegation harness. Work on GitHub issue #%d.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.`, issueNum)
}

// preCommitHookScript is the generated pre-commit dispatcher installed
// into worktree .git/hooks/. It runs go build + go vet, then role
// capability enforcement, then any additional gate scripts found in
// .mill/checks/gate-*.
const preCommitHookScript = `#!/bin/bash
# Mill gauntlet — pre-commit. Runs on every git commit.
# Fast checks (<30s). Fail = commit rejected.
set -euo pipefail

echo "mill: pre-commit gauntlet"

go build ./... && echo "PASS go build" || { echo "FAIL go build — run: go build ./..."; exit 1; }
go vet ./...  && echo "PASS go vet"   || { echo "FAIL go vet — run: go vet ./..."; exit 1; }

# --- Version conflict detection ---
# Find project root by walking up to the main .git directory
PROJECT_ROOT=$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)
if [ -n "$PROJECT_ROOT" ] && [ "${PROJECT_ROOT##*/}" = ".git" ]; then
    PROJECT_ROOT=$(dirname "$PROJECT_ROOT")
fi

# Determine issue number from worktree path
ISSUE_NUM=""
WT_DIR=$(pwd)
if [[ "$WT_DIR" =~ issue-([0-9]+) ]]; then
    ISSUE_NUM="${BASH_REMATCH[1]}"
fi

LEDGER_FILE=""
if [ -n "$PROJECT_ROOT" ] && [ -n "$ISSUE_NUM" ]; then
    LEDGER_FILE="$PROJECT_ROOT/.mill/ledger/$ISSUE_NUM.jsonl"
fi

# Read agent identity
AGENT_ID="unknown"
if [ -f .mill/agent_id ]; then
    AGENT_ID=$(cat .mill/agent_id)
fi

# Version conflict check for each staged file
if [ -n "$LEDGER_FILE" ] && [ -f "$LEDGER_FILE" ]; then
    # Get list of staged files (relative to worktree root)
    STAGED=$(git diff --cached --name-only 2>/dev/null || true)
    for FILE in $STAGED; do
        # Find latest file_read version for this file by this agent
        LATEST_READ=$(grep "file_read" "$LEDGER_FILE" 2>/dev/null | grep "\"file\":\"$FILE\"" | grep "\"agent_id\":\"$AGENT_ID\"" | tail -1 | grep -o '"version":[0-9]*' | grep -o '[0-9]*$' || echo "")
        if [ -z "$LATEST_READ" ]; then
            # No read tracked — first write is always allowed
            continue
        fi

        # Find latest file_write version for this file by any agent
        LATEST_WRITE_LINE=$(grep "file_write" "$LEDGER_FILE" 2>/dev/null | grep "\"file\":\"$FILE\"" | tail -1 || echo "")
        if [ -z "$LATEST_WRITE_LINE" ]; then
            LATEST_WRITE=0
            LATEST_WRITER=""
        else
            LATEST_WRITE=$(echo "$LATEST_WRITE_LINE" | grep -o '"version":[0-9]*' | grep -o '[0-9]*$' || echo "0")
            LATEST_WRITER=$(echo "$LATEST_WRITE_LINE" | grep -o '"agent_id":"[^"]*"' | cut -d'"' -f4 || echo "")
        fi

        # Conflict detection
        CONFLICT=0
        if [ "$LATEST_WRITE" -gt "$LATEST_READ" ] 2>/dev/null; then
            CONFLICT=1
        elif [ "$LATEST_WRITE" = "$LATEST_READ" ] && [ "$LATEST_WRITER" != "$AGENT_ID" ] && [ -n "$LATEST_WRITER" ]; then
            CONFLICT=1
        fi

        if [ "$CONFLICT" = "1" ]; then
            echo "BLOCKED: version conflict on $FILE (read v$LATEST_READ, current v$LATEST_WRITE by $LATEST_WRITER)" >&2
            mkdir -p .mill
            echo -e "BLOCKED\t$FILE\tread v$LATEST_READ\tcurrent v$LATEST_WRITE\tby $LATEST_WRITER" >> .mill/enforcement.log
            exit 1
        fi
    done
fi
# --- End version conflict detection ---

# Role capability enforcement — runs before the phase gates
if [ -x .mill/checks/role-enforce ]; then
    echo "Running role-enforce"
    .mill/checks/role-enforce || { echo "FAIL role-enforce"; exit 1; }
fi

# Run additional gate scripts if present
for gate in .mill/checks/gate-*; do
    if [ -x "$gate" ]; then
        echo "Running gate: $(basename "$gate")"
        "$gate" || { echo "FAIL $(basename "$gate")"; exit 1; }
    fi
done

echo "mill: pre-commit passed"
`

// installHooks installs the gauntlet pre-commit hook for the worktree.
// It first verifies the target is a real git worktree (worktree/.git is
// a file containing "gitdir:"), then creates a .mill/hooks/ directory
// inside the worktree, enables per-worktree config (extensions.worktreeConfig),
// configures core.hooksPath for this worktree only via `git config --worktree`,
// and writes the pre-commit dispatcher with 0755 permissions.
func installHooks(worktree string) error {
	// 1. Verify this is a real git worktree
	gitFile := filepath.Join(worktree, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		return fmt.Errorf("worktree is not a git worktree: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("worktree has .git directory, not a git worktree file. " +
			"Run 'git worktree add' to create a proper worktree")
	}
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return fmt.Errorf("cannot read worktree .git file: %w", err)
	}
	if !strings.HasPrefix(string(data), "gitdir:") {
		return fmt.Errorf("worktree .git is not a valid git worktree reference")
	}

	// 2. Create hooks directory inside the worktree and configure git.
	// Resolve to an absolute path: a relative core.hooksPath is resolved by
	// git from the committer's current working directory (the worktree root),
	// which would double-nest the path (e.g. <wt>/<wt>/.mill/hooks). An
	// absolute path resolves identically from any CWD and outlives the
	// worktree if the relative root shifts.
	absWorktree, err := filepath.Abs(worktree)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute worktree path: %w", err)
	}
	hookDir := filepath.Join(absWorktree, ".mill", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("cannot create hooks directory: %w", err)
	}
	// Enable per-worktree config (idempotent), then set core.hooksPath with
	// --worktree so only this worktree (not the main repo or other worktrees)
	// reads it. Without extensions.worktreeConfig, --worktree would fail.
	enableWT := exec.Command("git", "-C", worktree, "config", "extensions.worktreeConfig", "true")
	if out, err := enableWT.CombinedOutput(); err != nil {
		return fmt.Errorf("cannot enable extensions.worktreeConfig: %w\n%s", err, out)
	}
	setHook := exec.Command("git", "-C", worktree, "config", "--worktree", "core.hooksPath", hookDir)
	if out, err := setHook.CombinedOutput(); err != nil {
		return fmt.Errorf("cannot configure core.hooksPath: %w\n%s", err, out)
	}

	// 3. Install pre-commit dispatcher with 0755
	preCommitPath := filepath.Join(hookDir, "pre-commit")
	if err := os.WriteFile(preCommitPath, []byte(preCommitHookScript), 0755); err != nil {
		return err
	}

	// 4. Verify pre-commit is executable post-install
	info, err = os.Stat(preCommitPath)
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hook pre-commit is not executable after install")
	}

	return nil
}

// extractFlag extracts a --flag value from args, returning the value
// and the remaining args with the flag and its value removed.
// Used to work around Go's flag package stopping at the first positional arg.
func extractFlag(args []string, name string) (string, []string) {
	flag := "--" + name
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			val := args[i+1]
			rest := make([]string, 0, len(args)-2)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+2:]...)
			return val, rest
		}
	}
	return "", args
}

// classifyFailure examines an agent session result and returns the
// corresponding domain.FailureClass using the default SignalRegistry.
//
// The registry resolves signals in priority order: stderr-derived signals
// first, then exit-code signals, then the heartbeat guard, then the
// environment guard. The CONTRACT artifact inspection (empty/placeholder/
// TODO/TBD output) only fires when the process exit code is OK (0).
func classifyFailure(result domain.SessionResult) domain.FailureClass {
	return domain.NewSignalRegistry().Resolve(result)
}

// toSessionResult converts an adapter.SessionResult into a domain.SessionResult
// for failure classification. HeartbeatStaleness is copied from the adapter's
// wait path so the classifier can observe session liveness.
func toSessionResult(r adapter.SessionResult) domain.SessionResult {
	return domain.SessionResult{
		ExitCode:           r.ExitCode,
		Stderr:             r.Stderr,
		Output:             r.Output,
		HeartbeatStaleness: r.HeartbeatStaleness,
	}
}

// providerBinary maps provider names to their CLI binary names.
// This is an extensible map — add entries for new providers.
var providerBinary = map[string]string{
	"commandcode": "cmd",
}

// missingBinaryErr returns an error naming the first required binary missing
// from PATH (git, go, or the configured provider binary), or nil if all are
// present.
func missingBinaryErr(cfg config.Config) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH. Install git to continue.")
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found in PATH. Install Go to continue.")
	}
	if cfg.Provider == "" {
		return nil
	}
	binary, ok := providerBinary[cfg.Provider]
	if !ok {
		binary = cfg.Provider
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("provider binary %q not found in PATH", binary)
	}
	return nil
}

// validateDelegateBinaries checks that the required toolchain and provider
// binaries are on PATH before delegation begins. It checks git, go, and the
// configured provider's CLI binary. On a missing binary the task is marked
// aborted + ENVIRONMENT_FAILURE and persisted (instead of returning a direct
// error), so the environment failure is recorded in state. Returns nil in
// that case; a non-nil error means the failure could not be persisted.
func (a *App) validateDelegateBinaries(cfg config.Config, task *domain.Task, s state.State) error {
	if err := missingBinaryErr(cfg); err != nil {
		task.Transition(domain.TaskPhaseAborted, domain.TaskAborted, domain.VerdictRejected, 0, domain.ENVIRONMENT_FAILURE)
		task.AbortReason = err.Error()
		s.UpsertTask(*task)
		return s.Save(a.statePath())
	}
	return nil
}
