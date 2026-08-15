# Spec: `mill init` must warn before overwriting existing `.mill/`

## Architecture

**Problem:** `mill init` unconditionally overwrites the `.mill/` directory and its contents (roles, checks, skills, docs) without any warning or confirmation prompt. If the user runs `mill init` in a directory that already has a mill project (or a directory with `.mill/` from another tool), all existing data is silently destroyed. The issue reporter lost 30MB+ of session data including two delegated tasks.

**Solution:** Add a pre-init check that detects an existing `.mill/` directory and requires explicit confirmation before overwriting. Two paths:

### Path A: Interactive prompt (default)
When `.mill/` exists, `mill init` prints a warning listing what will be overwritten and prompts for confirmation:
```
⚠ .mill/ already exists.
This will overwrite:
  .mill/state.json (1.1KB, 3 tasks)
  .mill/ledger/ (4 entries)
  .mill/worktrees/ (5 worktrees)
  .mill/roles/ (12 role files)
  .mill/checks/ (9 scripts)
  .mill/skills/ (4 files)
  .mill/docs/ (8 files)
Overwrite? [y/N]:
```
Only `y` or `yes` (case-insensitive) proceeds. Any other input (including empty) aborts.

### Path B: `--force` flag (non-interactive)
When `--force` is passed, the prompt is skipped and `.mill/` is overwritten unconditionally. This supports scripting and CI. The existing `--yes` flag (`-yes`) skips interactive prompts for project name/model but does NOT override the overwrite check — it has a different semantic (accept defaults vs. acknowledge data loss).

### What gets checked
The pre-init check detects `.mill/` at the target path. If `.mill/` exists and is a directory, the check fires. If `.mill/` exists but is a file (edge case, malformed), a different error is shown: "`.mill/` exists but is a file, not a directory — remove it manually and retry".

### What does NOT change
- The scaffold copy logic (`copyScaffold`) is unchanged — it already overwrites files.
- The mill.yml generation is unchanged.
- The runtime directory creation (ledger, worktrees, phases, artifacts, memory) is unchanged.
- The `-yes` flag retains its current semantics (skip project config prompts).

### Architecture decision
The overwrite check is placed **early** in `runInit`, before any filesystem writes (including `os.MkdirAll`). This ensures no partial state is created before the user confirms. The check runs after flag parsing but before `os.MkdirAll(target)`.

## Components affected

| File | Change |
|---|---|
| `internal/cli/init.go` | MODIFY: Add `promptOverwrite` function + call before `os.MkdirAll`. Add `--force` flag. |
| `internal/cli/init_test.go` | MODIFY: Add tests: `--force` bypass, interactive reject, interactive accept, no `.mill/` (skips check), `.mill/` is file edge case |

### Files NOT affected
- `internal/cli/app.go` — no routing changes
- `internal/state/` — no schema changes
- Any other file

## Risks

### Risk 1: `--force` flag is too easy to misuse
**Severity:** Medium. **Mitigation:** `--force` is documented in help text as destructive: "overwrite .mill/ without confirmation (DESTRUCTIVE)". The flag name follows Unix convention (`rm -f`, `rsync --force`). If the user passes `--force`, they accept the consequences. The `-yes` flag does NOT imply `--force` — they are orthogonal. This prevents accidental overwrites from users who type `-yes` without reading.

### Risk 2: Pre-init check slows down `mill init` on large projects
**Severity:** Low. **Mitigation:** The check is a single `os.Stat(".mill/")` call. No recursive directory walk for size calculation — sizes are computed only for the warning message, after the directory is confirmed to exist. If `.mill/` does not exist, the overhead is one stat call (~microseconds).

### Risk 3: Interactive prompt blocks scripting/CI
**Severity:** Medium. **Mitigation:** Scripts use `mill init --force --yes -name project`. CI pipelines always pass `--force`. The `--force` flag is a first-class citizen, not a workaround. Documentation examples show both interactive and non-interactive usage.

### Risk 4: User ignores the warning and loses data anyway
**Severity:** Low. **Mitigation:** The warning is not a legal disclaimer — it's a UX improvement. A user who types `y` after reading the warning made an informed choice. For stronger protection, a future feature could back up `.mill/` before overwriting (`.mill.bak/`), but that's out of scope for this spec.

## ADR

No new ADR. The overwrite check is a UX guard within a single command — not a cross-cutting architectural decision.

## Acceptance criteria

1. `mill init` in a directory with existing `.mill/` shows a warning and prompts for confirmation
2. Responding `n` (or anything other than `y`/`yes`) aborts with no changes
3. Responding `y` proceeds with normal init flow
4. `mill init --force` skips the prompt and overwrites unconditionally
5. `mill init --force --yes` works for scripting (skips all prompts)
6. `mill init` in a directory without `.mill/` works as before (no prompt)
7. `mill init` when `.mill/` is a file (not directory) shows a specific error
8. `go test ./internal/cli/` passes with new init tests
