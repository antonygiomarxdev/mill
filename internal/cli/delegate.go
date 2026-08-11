package cli

import (
	"context"
	"flag"
	"fmt"
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
	// Validate mill.yml and capture concurrency settings.
	myml, err := config.LoadAndValidate("mill.yml")
	if err != nil {
		return err
	}

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

	// Validate required binaries before proceeding
	if err := validateDelegateBinaries(cfg); err != nil {
		return err
	}

	// Query adapter capabilities before side effects (spec: eager, before worktree)
	caps := a.Adapter.Capabilities()
	fmt.Fprintf(a.Err, "delegate: adapter capabilities — models=%d selectors=%v recovery=%v line_ceiling=%d byte_ceiling=%d\n",
		len(caps.Models), caps.ReadTool.HasSelectorSupport, caps.ReadTool.HasRecoveryNotes,
		caps.ReadTool.LineCeiling, caps.ReadTool.ByteCeiling)

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

	// Resolve adapter capabilities for prompt generation

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
		Worktree: wt,
		Prompt:   prompt,
		Model:    model,
		MaxTurns: maxTurns,
		Budget:   cfg.Budget,
	}

	// Acquire slot before dispatching (blocks if all slots occupied).
	ctx := context.Background()
	maxSlots := MaxSlotsFromConfig(cfg)
	if myml.Concurrency.MaxSlots > 0 {
		maxSlots = myml.Concurrency.MaxSlots
	}
	if _, err := AcquireSlot(ctx, a.slots, a.Err, issueNum, targetRole, priority, maxSlots); err != nil {
		return fmt.Errorf("slot acquisition failed: %w", err)
	}

	if wait {
		defer a.slots.Release()
		classification, err := runDispatchLoop54(a, issueNum, taskID, targetRole, modelOverride, opts, issueBody, labels, cfg, caps)
		if isIrrecoverable(classification) {
			irrecoverable = true
		}
		return err
	}

	// Async: fire and forget. Agent runs in background goroutine.
	go func() {
		defer a.slots.Release()
		classification, _ := runDispatchLoop54(a, issueNum, taskID, targetRole, modelOverride, opts, issueBody, labels, cfg, caps)
		if isIrrecoverable(classification) {
			a.cleanupWorktree(issueNum)
		}
	}()
	fmt.Fprintf(a.Out, "Delegated issue %d — task %s (async)\n", issueNum, taskID)
	return nil
}



// isIrrecoverable reports whether a classification indicates the worktree
// should be cleaned up (FATAL, AUTH, or NO_CREDIT).
func isIrrecoverable(c domain.Classification) bool {
	return c == domain.ClassificationFatal ||
		c == domain.ClassificationAuth ||
		c == domain.ClassificationNoCredit
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

// cleanupWorktree removes the git worktree and prunes its branch.
// It is best-effort: errors are logged to a.Err, not propagated.
// Used as deferred cleanup when an agent session fails irrecoverably.
func (a *App) cleanupWorktree(issueNum int) {
	wt := a.worktreePath(issueNum)
	branch := a.worktreeBranch(issueNum)

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

// maxRetries is the maximum number of retry attempts for transient failures.
const maxRetries = 4

// retryDispatch wraps Adapter.Dispatch + session.Wait + classifyResult with
// exponential backoff retry on transient failures.
// Backoff: 1s → 2s → 4s → 8s (max 4 retries).
// Non-transient classifications (FATAL, AUTH, NO_CREDIT, BLOCKED, RATE_LIMITED)
// bypass retry and return immediately.
func (a *App) retryDispatch(
	opts adapter.DispatchOpts,
	phase string,
	issueNum int,
	task domain.Task,
	cfg config.Config,
) (adapter.SessionResult, domain.Classification, error) {
	retryLimit := cfg.MaxRetries
	if retryLimit <= 0 {
		retryLimit = maxRetries
	}

	for retry := 0; retry <= retryLimit; retry++ {
		if retry > 0 {
			backoff := time.Duration(1<<(retry-1)) * time.Second
			sleep := a.Backoff
			if sleep == nil {
				sleep = time.Sleep
			}
			sleep(backoff)
		}

		session, err := a.Adapter.Dispatch(opts)
		if err != nil {
			if isTransientError(err) && retry < retryLimit {
				continue
			}
			return adapter.SessionResult{}, "", fmt.Errorf("%s dispatch failed: %w", phase, err)
		}

		result, err := session.Wait()
		if err != nil {
			if isTransientError(err) && retry < retryLimit {
				continue
			}
			return adapter.SessionResult{}, "", fmt.Errorf("%s session failed: %w", phase, err)
		}

		// Auto-compact session context if enabled and near threshold.
		a.maybeAutoCompactSession(session, opts.Model, issueNum, opts.Worktree, cfg)

		class := classifyResult(result.ExitCode, result.Stderr)
		if class == domain.ClassificationTransient && retry < retryLimit {
			continue
		}
		if class != domain.ClassificationTransient {
			return result, class, nil
		}
		// Last retry and still transient — let loop exit and return error
	}

	return adapter.SessionResult{}, "", fmt.Errorf("%s: max retries (%d) exceeded", phase, retryLimit)
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
// into worktree .git/hooks/. It runs go build + go vet, then any
// additional gate scripts found in .mill/checks/*.sh.
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

# Run additional gate scripts if present
for gate in .mill/checks/*.sh; do
    if [ -x "$gate" ]; then
        echo "Running gate: $(basename "$gate")"
        sh "$gate" || { echo "FAIL $(basename "$gate")"; exit 1; }
    fi
done

echo "mill: pre-commit passed"
`

// installHooks installs the gauntlet pre-commit hook for the worktree.
// It first verifies the target is a real git worktree (worktree/.git is
// a file containing "gitdir:"), then creates a .mill/hooks/ directory
// inside the worktree, configures core.hooksPath to point there, and
// writes the pre-commit dispatcher with 0755 permissions.
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

	// 2. Create hooks directory inside the worktree and configure git
	hookDir := filepath.Join(worktree, ".mill", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("cannot create hooks directory: %w", err)
	}
	// Set core.hooksPath so git finds hooks in this worktree-local directory
	setHook := exec.Command("git", "-C", worktree, "config", "core.hooksPath", hookDir)
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

// providerBinary maps provider names to their CLI binary names.
// This is an extensible map — add entries for new providers.
var providerBinary = map[string]string{
	"commandcode": "cmd",
}

// validateDelegateBinaries checks that required toolchain and provider
// binaries are on PATH before delegation begins. It checks git, go,
// and the configured provider's CLI binary. The check runs BEFORE
// worktree creation so the user gets a fast, clear error.
func validateDelegateBinaries(cfg config.Config) error {
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
