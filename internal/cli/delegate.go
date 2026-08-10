package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/ledger"
	"github.com/antonygiomarxdev/mill/internal/role"
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
	roleName, args := extractFlag(args, "role")
	modelFlag, args := extractFlag(args, "model")

	fs := flag.NewFlagSet("delegate", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var model string
	var maxTurns int
	var wait bool
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

	// Build role-aware prompt and dispatch opts
	prompt := buildRolePrompt(issueNum, targetRole)
	opts := adapter.DispatchOpts{
		Worktree: a.worktreePath(issueNum),
		Prompt:   prompt,
		Model:    model,
		MaxTurns: maxTurns,
		Budget:   cfg.Budget,
	}

	if wait {
		return a.runDispatchLoop(issueNum, taskID, opts)
	}

	// Async: fire and forget. Agent runs in background goroutine.
	go a.runDispatchLoop(issueNum, taskID, opts)
	fmt.Fprintf(a.Out, "Delegated issue %d — task %s (async)\n", issueNum, taskID)
	return nil
}

// runDispatchLoop dispatches an agent, waits for completion, classifies the
// result, retries on transient failures, and persists final state.
func (a *App) runDispatchLoop(issueNum int, taskID string, opts adapter.DispatchOpts) error {
	const maxRetries = 3
	var classification domain.Classification
	var result adapter.SessionResult

	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	task := domain.NewTask(taskID, issueNum)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		session, err := a.Adapter.Dispatch(opts)
		if err != nil {
			a.recordError(s, issueNum, task, err, "failed to dispatch agent")
			return fmt.Errorf("failed to dispatch agent: %w", err)
		}

		result, err = session.Wait()
		if err != nil {
			a.recordError(s, issueNum, task, err, "agent session failed")
			return fmt.Errorf("agent session failed: %w", err)
		}
		classification = classifyResult(result.ExitCode, result.Stderr)

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
		break
	}

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
	rolePath := filepath.Join(root, "roles", activeRole, "ROLE.md")
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
func buildRolePrompt(issueNum int, targetRole string) string {
	root, err := projectRoot()
	if err != nil {
		root = "."
	}
	rolePrompt, err := role.LoadFrom(root, targetRole)
	if err != nil {
		// Fall back to generic prompt if role can't be loaded
		return fmt.Sprintf(`You are mill, an agent delegation harness.
Role: %s
Work on GitHub issue #%d.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.`, targetRole, issueNum)
	}

	return fmt.Sprintf("%s\n\n---\n\nWork on GitHub issue #%d.\n\nRead the codebase, make the necessary changes, and when you are done,\nend your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.", rolePrompt, issueNum)
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
	srcDir := "checks"
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
