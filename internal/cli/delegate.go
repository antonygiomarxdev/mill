package cli

import (
	"context"
	"flag"
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
	fs.BoolVar(&priority, "priority", false, "preempt next available slot (staff only)")
	fs.StringVar(&model, "model", modelFlag, "model to use (default: from config)")
	fs.IntVar(&maxTurns, "max-turns", 100, "max conversation turns")
	fs.BoolVar(&wait, "wait", false, "wait for agent to finish (default: async)")

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

	// Read issue body and labels from GitHub
	issueBody, labels, err := a.IssueReader(issueNum)
	if err != nil {
		return fmt.Errorf("failed to read issue #%d: %w", issueNum, err)
	}
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

	// Initialize slot manager if not already set (from config concurrency settings).
	if a.slots == nil {
		maxSlots := MaxSlotsFromConfig(cfg)
		a.slots = slots.NewManager(maxSlots)
	}

	// Validate --priority flag (staff only).
	if err := ValidatePriority(priority, activeRole); err != nil {
		return err
	}

	// Resolve model from stage label, role frontmatter, or config
	if model == "" {
		model = a.resolveModel(targetRole, stageLabel, cfg)
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

	// Install gauntlet hooks into worktree
	wt := a.worktreePath(issueNum)

	// Scaffold context files so the agent finds AGENTS.md, .omp/, roles/
	if err := a.copyScaffold(wt); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to scaffold worktree: %v\n", err)
	}
	// Write .mill/role so the agent knows its role
	roleFile := filepath.Join(wt, ".mill", "role")
	if err := os.MkdirAll(filepath.Dir(roleFile), 0755); err == nil {
		os.WriteFile(roleFile, []byte(targetRole), 0644)
	}
	if err := installHooks(wt); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to install hooks: %v\n", err)
	}

	// Build role-aware prompt with issue body
	prompt := buildRolePrompt(issueNum, targetRole, issueBody)
	opts := adapter.DispatchOpts{
		Worktree: a.worktreePath(issueNum),
		Prompt:   prompt,
		Model:    model,
		MaxTurns: maxTurns,
		Budget:   cfg.Budget,
	}

	// Acquire slot before dispatching (blocks if all slots occupied).
	ctx := context.Background()
	maxSlots := MaxSlotsFromConfig(cfg)
	if _, err := AcquireSlot(ctx, a.slots, a.Err, issueNum, targetRole, priority, maxSlots); err != nil {
		return fmt.Errorf("slot acquisition failed: %w", err)
	}

	if wait {
		defer a.slots.Release()
		return a.runDispatchLoop(issueNum, taskID, opts, issueBody, labels, cfg)
	}

	// Async: fire and forget. Agent runs in background goroutine.
	go func() {
		defer a.slots.Release()
		a.runDispatchLoop(issueNum, taskID, opts, issueBody, labels, cfg)
	}()
	fmt.Fprintf(a.Out, "Delegated issue %d — task %s (async)\n", issueNum, taskID)
	return nil
}

// runDispatchLoop runs the produce→review cycle for a delegated issue.
// Each round: produce phase (cheap model) → review phase (expensive model).
// Exits on APPROVED, BLOCKED/FATAL/AUTH/NO_CREDIT/RATE_LIMITED, or after MaxRounds.
// Persists state after each round so `mill watch` can observe progress.
func (a *App) runDispatchLoop(issueNum int, taskID string, opts adapter.DispatchOpts, issueBody string, labels []string, cfg config.Config) error {
	var finalClassification domain.Classification
	var finalCommits int

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

	for round := range maxRounds {
		task.Round = round

		// --- Produce phase ---
		produceOpts := opts
		produceOpts.Model = produceModel
		session, err := a.Adapter.Dispatch(produceOpts)
		if err != nil {
			a.recordError(s, issueNum, task, err, "failed to dispatch produce agent")
			return fmt.Errorf("failed to dispatch produce agent: %w", err)
		}

		produceResult, err := session.Wait()
		if err != nil {
			a.recordError(s, issueNum, task, err, "produce agent session failed")
			return fmt.Errorf("produce agent session failed: %w", err)
		}
		finalCommits = produceResult.Commits

		// --- Review phase ---
		reviewPrompt := buildReviewPrompt(issueNum, issueBody, produceResult.Output)
		reviewOpts := adapter.DispatchOpts{
			Worktree: opts.Worktree,
			Prompt:   reviewPrompt,
			Model:    reviewModel,
			MaxTurns: opts.MaxTurns,
			Budget:   opts.Budget,
		}
		reviewSession, err := a.Adapter.Dispatch(reviewOpts)
		if err != nil {
			a.recordError(s, issueNum, task, err, "failed to dispatch review agent")
			return fmt.Errorf("failed to dispatch review agent: %w", err)
		}

		reviewResult, err := reviewSession.Wait()
		if err != nil {
			a.recordError(s, issueNum, task, err, "review agent session failed")
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
			return fmt.Errorf("failed to append ledger entry: %w", err)
		}

		// Persist state after each round
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

		case domain.ClassificationBlocked,
			domain.ClassificationAuth,
			domain.ClassificationNoCredit,
			domain.ClassificationRateLimited,
			domain.ClassificationFatal:
			// Non-recoverable — exit immediately
			goto finish

		case domain.ClassificationChangesRequested, domain.ClassificationMaxTurns:
			// Changes requested or max turns — continue to next round
			continue

		default:
			// Transient or unknown — continue to next round
			continue
		}
	}

	// Max rounds exhausted
finish:
	taskStatus := domain.TaskDone
	verdict := domain.VerdictApproved

	switch finalClassification {
	case domain.ClassificationOK:
		verdict = domain.VerdictApproved
	case domain.ClassificationChangesRequested:
		verdict = domain.VerdictChangesRequested
	case domain.ClassificationMaxTurns:
		verdict = domain.VerdictChanges
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

// buildReviewPrompt constructs a review prompt that asks the reviewer agent
// to evaluate the produce agent's output against the issue body.
func buildReviewPrompt(issueNum int, issueBody string, produceOutput string) string {
	prompt := fmt.Sprintf(`You are a code reviewer for mill, an agent delegation harness.
Review the following work product for GitHub issue #%d.

**Issue Body:**
%s

**Work Product (produce agent output):**
%s

Evaluate whether the work product satisfies all acceptance criteria in the issue body.
End your review by emitting EXACTLY ONE of these signals on stderr:
- APPROVED: if the work is complete and correct
- CHANGES_REQUESTED: if the work needs modifications
- BLOCKED: if you cannot complete the review`, issueNum, issueBody, produceOutput)
	return prompt
}

// modelTier maps role frontmatter model tiers to actual model names.
// "free→paid" starts with free and escalates on complexity.
var modelTier = map[string]string{
	"free":      "deepseek/deepseek-v4-flash",
	"paid":      "deepseek/deepseek-v4-pro",
	"pro":       "deepseek/deepseek-v4-pro",
	"free→paid": "deepseek/deepseek-v4-flash",
}

// resolveModel reads the target role's frontmatter model field and maps
// the tier name to an actual model identifier. Falls back to config.Model.
// The stageLabel (from issue labels) influences model selection:
//
//	stage:produce   → "laguna-free"
//	stage:review    → "laguna-pro"
//	stage:implement → "laguna-free"
//
// When stageLabel is empty, the role frontmatter's model field is used.
func (a *App) resolveModel(targetRole string, stageLabel string, cfg config.Config) string {
	if stageLabel != "" {
		switch stageLabel {
		case "stage:produce":
			return "laguna-free"
		case "stage:review":
			return "laguna-pro"
		case "stage:implement":
			return "laguna-free"
		}
	}
	root, err := projectRoot()
	if err != nil {
		return cfg.Model
	}
	rolePath := filepath.Join(root, ".mill", "roles", targetRole, "ROLE.md")
	fm, err := role.ParseFrontmatter(rolePath)
	if err != nil || fm.Model == "" {
		return cfg.Model
	}

	// "free→paid" means start cheap, escalate on complexity.
	// For initial dispatch, always use the first tier ("free").
	tier := fm.Model
	if tier == "free→paid" {
		tier = "free"
	}

	if m, ok := modelTier[tier]; ok {
		return m
	}
	return cfg.Model
}

// buildPrompt constructs the query passed to the agent for a given issue.
// Deprecated: use buildRolePrompt for role-aware prompts.
func buildPrompt(issueNum int) string {
	return fmt.Sprintf(`You are mill, an agent delegation harness. Work on GitHub issue #%d.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.`, issueNum)
}

// installHooks copies gauntlet hook scripts into the worktree's .git/hooks directory.
func installHooks(worktree string) error {
	srcDir := ".mill/checks"
	hookDir := filepath.Join(worktree, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(hookDir, strings.TrimSuffix(e.Name(), ".sh"))
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0755); err != nil {
			return err
		}
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


// classifyResult examines an agent session's exit code and stderr output
// and returns the corresponding domain.Classification.
//
// Stderr signals are checked first (priority over exit code):
//
//	1. "blocked:"           → BLOCKED
//	2. auth signals         → AUTH
//	3. no-credit signals    → NO_CREDIT
//	4. rate-limit signals   → RATE_LIMITED
//	5. transient signals    → TRANSIENT
//
// If no stderr signal matches, the exit code is mapped:
// 0 → OK, 3 → AUTH, 4/9/130/137/143 → FATAL, 5 → RATE_LIMITED,
// 6/7 → TRANSIENT, 8 → MAX_TURNS, 10 → NO_CREDIT,
// -1/-2 → BLOCKED (budget violation), default → FATAL.
func classifyResult(exitCode int, stderr string) domain.Classification {
	lower := strings.ToLower(stderr)
	// Check stderr signals first
	if strings.Contains(lower, "blocked:") {
		return domain.ClassificationBlocked
	}
	if strings.Contains(lower, "approved:") {
		return domain.ClassificationOK
	}
	if strings.Contains(lower, "changes_requested:") || strings.Contains(lower, "changes requested:") {
		return domain.ClassificationChangesRequested
	}
	if strings.Contains(lower, "not authenticated") || strings.Contains(lower, "no api key") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "401") || strings.Contains(lower, "403") {
		return domain.ClassificationAuth
	}
	if strings.Contains(lower, "insufficient credits") || strings.Contains(lower, "no credits") || strings.Contains(lower, "credit limit") {
		return domain.ClassificationNoCredit
	}
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") {
		return domain.ClassificationRateLimited
	}
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "econnrefused") || strings.Contains(lower, "timeout") {
		return domain.ClassificationTransient
	}
	// Fall back to exit code
	switch exitCode {
	case 0:
		return domain.ClassificationOK
	case 3:
		return domain.ClassificationAuth
	case 4, 9, 130, 137, 143:
		return domain.ClassificationFatal
	case 5:
		return domain.ClassificationRateLimited
	case 6, 7:
		return domain.ClassificationTransient
	case 8:
		return domain.ClassificationMaxTurns
	case 10:
		return domain.ClassificationNoCredit
	case -1, -2:
		return domain.ClassificationBlocked
	default:
		return domain.ClassificationFatal
	}
}
