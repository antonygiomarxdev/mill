# Tasks: `mill init` must warn before overwriting existing `.mill/`

SPEC: `.mill/phases/62/spec.md` — 8 acceptance criteria.

## Task 1: Implement overwrite check in `runInit`

- **role:** sr-dev-be
- **file:** `internal/cli/init.go` (MODIFY)
- **deps:** none
- **est:** 45m

1. Add `force bool` variable and register `--force` flag in `flagSet`: `flagSet.BoolVar(&force, "force", false, "overwrite .mill/ without confirmation (DESTRUCTIVE)")`

2. Implement `promptOverwrite(target string, in *bufio.Reader, out io.Writer) error`:
   - Call `os.Stat(filepath.Join(target, ".mill"))`
   - If `os.IsNotExist(err)`: return nil (proceed)
   - If err is not nil AND not `IsNotExist`: return wrapped error
   - If entry exists and `!entry.IsDir()`: return `fmt.Errorf(".mill/ exists but is a file, not a directory — remove it manually and retry")`
   - If entry exists and `entry.IsDir()`: gather subdirectory counts/sizes for the warning message
     - Walk `.mill/` subdirs (`state.json`, `ledger/`, `worktrees/`, `roles/`, `checks/`, `skills/`, `docs/`) to compute entry counts and sizes
     - Print warning listing what will be overwritten (per SPEC format)
     - Prompt `"Overwrite? [y/N]: "`
     - Read response with `in.ReadString('\n')`, trim whitespace, lowercase
     - If response is `"y"` or `"yes"`: return nil (proceed)
     - Otherwise: return `fmt.Errorf("init aborted")`

3. Call `promptOverwrite(target, in, a.Out)` in `runInit` AFTER flag parsing and interactive prompts, BEFORE `os.MkdirAll(target)`:
   - Only call when `!force` (skip entire check when `--force` is set)
   - `in` is `bufio.NewReader(a.In)` — reuse the same reader created for interactive prompts; create one if `-yes` skipped it
   - Return error if `promptOverwrite` returns error

4. The `-yes` flag does NOT bypass the overwrite check — only `--force` does. The `-yes` and `--force` flags are orthogonal:
   - `--force --yes`: non-interactive scripting (skip all prompts, overwrite unconditionally)
   - `--force` alone: skip overwrite prompt, still ask project config prompts
   - `-yes` alone: skip config prompts, still ask overwrite prompt
   - Neither: all prompts shown

## Task 2: Add tests for all overwrite paths

- **role:** sr-dev-be
- **file:** `internal/cli/init_test.go` (MODIFY)
- **deps:** Task 1
- **est:** 45m

All tests use `t.TempDir()` for isolation. Use `strings.NewReader` for stdin, `bytes.Buffer` for stdout/stderr.

1. `TestInitOverwriteInteractiveReject`: create `.mill/` dir in temp target, feed `"n\n"` to stdin, run `init -target dir`, assert error returned with "aborted" message, assert `.mill/` contents unchanged

2. `TestInitOverwriteInteractiveAccept`: create `.mill/` dir in temp target, feed `"y\n"` to stdin, run `init -target dir`, assert no error, assert init completed (mill.yml exists)

3. `TestInitOverwriteInteractiveAcceptYes`: create `.mill/` dir, feed `"yes\n"` to stdin, run `init -target dir`, assert no error, assert init completed

4. `TestInitOverwriteInteractiveCaseInsensitive`: create `.mill/` dir, feed `"Y\n"` / `"YES\n"` / `"Yes\n"` variants, assert each proceeds

5. `TestInitOverwriteForceBypass`: create `.mill/` dir with known file, run `init --force --yes -target dir`, assert no error, assert init overwrites (no prompt output, no "Overwrite?" in stderr)

6. `TestInitOverwriteForceWithoutYes`: create `.mill/` dir, run `init --force -target dir` with no stdin, assert error (blocks on config prompts since no stdin), but does NOT show overwrite prompt

7. `TestInitOverwriteNoMillDir`: run `init -yes -target dir` in clean temp dir (no `.mill/`), assert `os.Stat(".mill/")` yields directory after init, assert no "Overwrite?" in output

8. `TestInitOverwriteMillDirIsFile`: create `.mill` as a regular file (not directory) in temp target, run `init -yes -target dir`, assert error returned, assert error message contains "file, not a directory"

9. `TestInitOverwriteForceYesSkipsAllPrompts`: create `.mill/` dir, run `init --force --yes -name testproj -target dir`, assert no error, assert mill.yml contains `project: testproj` (--name honored without prompts), assert init completed

---

**Verification:** `go test ./internal/cli/ -run "TestInit" -count=1` passes with all new tests.
