# Spec: Smart init scaffolding

## Architecture

**Problem:** `mill init` today scaffolds identically for every project: Go module scaffold, full role tree, hardcoded harness files. The PM spec (#17 comment 3) requires architecture-aware scaffolding: detect project type, preserve existing directories, and support a minimal mode.

**Solution:** Three additions to the init flow, all within `runInit` and its helpers. No new packages — the detection and preservation logic fits naturally in `internal/cli/init.go` alongside existing `validatePreInit`, `collisionReport`, and `copyScaffold`.

### Current init flow (for reference)

```
runInit
  ├── validatePreInit(target)        // go.mod + .git check, exits early
  ├── dryRunInit(target)             // --dry-run path
  ├── promptOverwrite(target)        // collision check + interactive confirm
  ├── interactive prompts            // --yes skips
  ├── os.MkdirAll(target)
  ├── generateMillYAML(target, cfg)
  ├── copyScaffold(target)           // static/scaffold/ → target
  ├── mkdir .mill/{ledger,worktrees,phases,artifacts,memory}
  └── cleanup empty static/ dirs
```

### New init flow

```
runInit
  ├── validatePreInit(target)        // RELAXED: no longer Go+git-only
  │     └── detectProjectType(target) // NEW: returns ProjectType
  ├── cfg.ProjectType = detected
  ├── dryRunInit(target)              // adjusted to reflect minimal/skip
  ├── promptOverwrite(target)         // EXTENDED: skip preserved dirs in collision report
  ├── interactive prompts             // unchanged
  ├── os.MkdirAll(target)
  ├── generateMillYAML(target, cfg)   // EXTENDED: project-type-aware template vars
  ├── copyScaffold(target, cfg)       // EXTENDED: skip preserved dirs, support --minimal
  ├── mkdir .mill/{ledger,worktrees,phases,artifacts,memory}
  └── cleanup empty static/ dirs
```

Key principle: detection is advisory, not blocking. If detection fails, the project type defaults to `generic` with a warning — init never aborts because it cannot classify the project.

---

## 1. Project type detection

### Algorithm: `detectProjectType(target string) ProjectType`

Walk up from `target` (resolved to absolute path) toward filesystem root, looking for sentinel files. The first sentinel found wins. The walk mirrors the existing `validatePreInit` root-finding loop (lines 157-167 of init.go).

```
detectProjectType(target):
    abs = filepath.Abs(target)
    root = abs
    found go.mod = false
    found package.json = false

    loop:
        if go.mod exists at root:
            found go.mod = true
            break
        if package.json exists at root:
            found package.json = true
            // DON'T break — continue looking for go.mod higher up
        parent = filepath.Dir(root)
        if parent == root: break
        root = parent

    if found go.mod:
        if found package.json at a different level:
            return MonoRepo   // has BOTH go.mod AND package.json
        return GoModule

    if found package.json:
        return JSProject      // only package.json, no go.mod

    return Generic
```

### ProjectType enum

```go
type ProjectType int

const (
    ProjectGeneric  ProjectType = iota  // no recognized sentinel
    ProjectGoModule                      // go.mod found, no package.json
    ProjectJSProject                     // package.json found, no go.mod
    ProjectMonoRepo                      // BOTH go.mod AND package.json at different levels
)
```

### Detection rules

| Sentinel | Found at | Type |
|---|---|---|
| `go.mod` only | any level | `GoModule` |
| `package.json` only | any level | `JSProject` |
| Both `go.mod` AND `package.json` | different levels (e.g. root has go.mod, `frontend/` has package.json) | `MonoRepo` |
| Neither | n/a | `Generic` |

**Edge case — both at same level:** If both sentinels exist in the same directory (a Go project with a JS toolchain file), the first found in the walk wins. Since we break on `go.mod`, it is classified as `GoModule`. This is intentional: `go.mod` is a stronger signal of project identity than `package.json`.

**Edge case — `go.work` file:** Not used for detection. A `go.work` workspace file indicates Go tooling but doesn't change the project type. Detection is based on module-level files only.

### What each type means for scaffolding

| Type | mill.yml `project.type` | Scaffold behavior |
|---|---|---|
| `GoModule` | `"go"` | Full scaffold (current behavior). Checks reference `go vet`, `go test`. |
| `JSProject` | `"js"` | Full scaffold but gate scripts adapted. Checks reference `npm test`, `tsc`. Gate files are templates with placeholders; copyScaffold renders them per-type. |
| `MonoRepo` | `"monorepo"` | Full scaffold plus monorepo-specific `.mill/docs/MONOREPO.md`. Checks are language-agnostic (exit-code gates only). Additional `.mill/config.json` key: `monorepo.roots: []string`. |
| `Generic` | `"generic"` | Full scaffold with language-agnostic checks. No Go-specific or JS-specific gate content. |

**After detection, `validatePreInit` is relaxed.** The Go+git requirement (lines 145-176) is moved to a warning:

```
if detectProjectType(target) == ProjectGeneric {
    fmt.Fprintf(a.Err, "Warning: no go.mod or package.json detected. Scaffolding generic project.\n")
}
```

The `validateBinaries()` call remains at the top — `git` must be on PATH regardless of project type (mill is git-based). `go` binary check is removed from `validateBinaries` and made conditional: only enforced when `ProjectType == GoModule || ProjectType == MonoRepo`.

---

## 2. Directory preservation logic

### Problem

Re-running `mill init` on an already-initialized project overwrites `.github/`, `.claude/`, `.omp/`, and `.mill/roles/` — destroying user customizations (custom issue templates, CLAUDE.md edits, role modifications).

### Design: skip existing top-level scaffold directories

`copyScaffold` (init.go lines 366-408) is extended with a `preserve` set. Before writing any file, the function checks whether the **top-level destination directory** exists on disk. If it does, the entire directory tree is skipped.

### Preserve check

```
preserveDir(dest string) bool:
    // dest is the full destination path for a scaffold file
    // e.g. /home/user/proj/.github/ISSUE_TEMPLATE/bug.md → check .github/
    rel = filepath.Rel(target, dest)
    topDir = first path component of rel       // e.g. ".github"
    // Only preserve known harness directories — never preserve generic dirs
    if topDir not in {".github", ".claude", ".omp", ".mill", "checks"}:
        return false
    // If the top-level directory exists AND contains files (not just an empty dir),
    // preserve it. Empty directories from a prior aborted init → overwrite.
    info = os.Stat(filepath.Join(target, topDir))
    if info != nil && info.IsDir() && dirIsNonEmpty(target, topDir):
        return true
    return false
```

### Merge vs overwrite

**Policy: skip entire directory, never merge individual files.**

Rationale:
- Merging file-by-file requires conflict resolution (which version wins? what if both modified?). This is a product decision, not a technical limitation.
- Users who want to update a specific file can delete the directory and re-run init, or copy the file from the scaffold manually.
- The current code already has `--force` (overwrite everything) and `--yes` (skip prompts). A `--merge` flag could be added later if needed.

### Edge cases

| Scenario | Behavior |
|---|---|
| `.github/` exists with custom issues | Entire `.github/` tree skipped. User's customizations preserved. |
| `.github/` exists but is empty (aborted prior init) | `.github/` overwritten. Empty dir = no user investment. |
| `.claude/CLAUDE.md` exists but no `.claude/` dir | Impossible — file implies dir exists. |
| `.mill/roles/staff/ROLE.md` modified | Entire `.mill/` tree skipped (`.mill/` is a top-level dir in preserve set). Runtime subdirs (`ledger`, `worktrees`, etc.) are still created — they are NOT scaffold files, they are runtime state. |
| `checks/` exists with custom gate scripts | Entire `checks/` tree skipped. |
| `.gitignore`d files inside `.github/` | Irrelevant — we check `os.Stat` on the directory, not individual files. If the directory exists on disk, it is preserved regardless of gitignore. |
| `target` is a completely clean directory | No preservation; full scaffold copied (current behavior). |

### Interaction with `collisionReport`

`collisionReport` (init.go lines 226-291) is extended with the same preserve logic. Preserved directories are excluded from the conflict list so `promptOverwrite` does not warn about files that will be skipped anyway.

### Interaction with `--force`

`--force` overrides preservation: ALL directories are overwritten. This is the "I know what I'm doing" escape hatch. The `preserveDir` check is skipped when `force == true`.

### Interaction with `--dry-run`

`dryRunInit` (init.go lines 294-309) reports which directories will be preserved:

```
Would preserve existing directory: .github/ (3 files)
Would preserve existing directory: .claude/ (1 file)
Would create: 24 scaffold files
```

### `dirIsNonEmpty` helper

```go
func dirIsNonEmpty(root, dir string) bool {
    entries, err := os.ReadDir(filepath.Join(root, dir))
    if err != nil {
        return false
    }
    return len(entries) > 0
}
```

---

## 3. `--minimal` flag

### What minimal means

`mill init --minimal` creates ONLY the files required for mill to function as a delegation harness — no role tree, no check scripts, no issue templates, no harness context files.

### Files created in minimal mode

| File | Purpose |
|---|---|
| `mill.yml` | Project configuration (always needed) |
| `.mill/COMMON.md` | Shared role context (loaded by all roles) |
| `.mill/roles/staff/ROLE.md` | Staff role definition (bootstrap role) |
| `.mill/roles/pm/ROLE.md` | PM role definition (bootstrap role) |

### Files SKIPPED in minimal mode

Everything else in `static/scaffold/`:
- `.mill/roles/{architect,tech-lead,reviewer,sr-dev-be,sr-dev-fe,sr-dev-data,qa-docs,ux-designer,ui-designer}/`
- `.mill/checks/` (all gate scripts)
- `.mill/skills/` (all skill files)
- `.mill/docs/` (PRODUCT.md, README.md)
- `.mill/AGENTS.md`
- `.github/` (ISSUE_TEMPLATE, DISCUSSION_TEMPLATE, copilot-instructions)
- `.claude/CLAUDE.md`
- `.omp/` (AGENTS.md, RULES.md)
- `checks/` (root-level gate scripts)

### Runtime directories (ALWAYS created, even in minimal mode)

```
.mill/ledger/
.mill/worktrees/
.mill/phases/
.mill/artifacts/
.mill/memory/
```

### Interaction with project type detection

`--minimal` skips detection entirely. If the user passes `--minimal`, `detectProjectType` is NOT called. The `mill.yml` still gets a `project.type` field, but it defaults to `"generic"` unless explicitly overridden via a new `--type` flag:

```
mill init --minimal --type go    → mill.yml has project.type: "go"
mill init --minimal              → mill.yml has project.type: "generic"
```

### Interaction with directory preservation

In minimal mode, preservation still applies — but since minimal mode creates very few files, the only directories that could collide are `.mill/` (runtime dirs always created). If `.mill/COMMON.md` or `.mill/roles/staff/ROLE.md` already exist, they are skipped (preserved) unless `--force` is passed.

### New flag definition

Added to `runInit`'s `flagSet`:

```go
var minimal bool
flagSet.BoolVar(&minimal, "minimal", false, "create only mill.yml + bootstrap roles (no full scaffold)")

var projectType string
flagSet.StringVar(&projectType, "type", "", "project type override: go, js, monorepo, generic")
```

---

## 4. Data model

### `initConfig` changes (init.go lines 21-26)

```go
type initConfig struct {
    Name        string
    Provider    string
    Model       string
    MaxRounds   int
    ProjectType string  // NEW: "go", "js", "monorepo", "generic"
    Minimal     bool    // NEW: true when --minimal is set
}
```

### `mill.yml` template changes

The `mill.yml.tmpl` (static/mill.yml.tmpl) gains one new key under `project:`:

```yaml
project: {{.Name}}
  type: {{.ProjectType}}   # NEW: detected or overridden project type
```

Existing keys unchanged:
```yaml
provider: {{.Provider}}
model: {{.Model}}
max-rounds: {{.MaxRounds}}
concurrency:
  max-slots: 4
compact:
  enabled: false
  mode: fast
directories: ...
roles: ...
```

### No new types in `internal/config/`

The `internal/config.Config` struct (config.go lines 32-43) does NOT need a `ProjectType` field. Project type is stored in `mill.yml`, not `config.json`. The `config.json` is for runtime configuration (provider, model, budget); `mill.yml` is for project identity and structure.

### No new types in `internal/domain/`

Project type is a CLI-level concern. It is not part of the domain model (tasks, verdicts, classifications). Domain types are unchanged.

### `ProjectType` defined in `internal/cli/`

```go
// ProjectType classifies the project for scaffolding decisions.
type ProjectType int

const (
    ProjectGeneric  ProjectType = iota
    ProjectGoModule
    ProjectJSProject
    ProjectMonoRepo
)

func (p ProjectType) String() string {
    switch p {
    case ProjectGoModule:
        return "go"
    case ProjectJSProject:
        return "js"
    case ProjectMonoRepo:
        return "monorepo"
    default:
        return "generic"
    }
}

func parseProjectType(s string) (ProjectType, error) {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "go", "gomodule":
        return ProjectGoModule, nil
    case "js", "javascript", "node", "typescript":
        return ProjectJSProject, nil
    case "monorepo", "mono":
        return ProjectMonoRepo, nil
    case "generic", "":
        return ProjectGeneric, nil
    default:
        return ProjectGeneric, fmt.Errorf("unknown project type: %q (valid: go, js, monorepo, generic)", s)
    }
}
```

---

## 5. Error handling

### Detection failures

| Scenario | Behavior |
|---|---|
| `os.Stat` fails on target during walk | Return `ProjectGeneric` with warning to stderr: `"Warning: could not read %s: %v — defaulting to generic project"` |
| Target does not exist | Existing `validatePreInit` handles this before detection runs (returns actionable error) |
| Permission denied walking up | Stop walking at the inaccessible directory. Return whatever was found below it. Warn: `"Warning: cannot scan %s for project markers (permission denied) — detection may be incomplete"` |
| Neither go.mod nor package.json found | Return `ProjectGeneric` — NOT an error. This is the normal case for non-Go, non-JS projects. |

### Directory preservation failures

| Scenario | Behavior |
|---|---|
| `os.Stat` fails on a top-level dir during preserve check | Assume the directory does NOT exist; proceed with overwrite. The `os.Stat` failure is likely transient (NFS, permissions) and blocking init for a stat call on a non-critical path is worse than overwriting. |
| `os.ReadDir` fails during `dirIsNonEmpty` | Treat as empty — proceed with overwrite. Same rationale. |
| Can't write a scaffold file because parent dir creation fails | Existing error handling in `copyScaffold` (line 391: `MkdirAll` failure returns wrapped error). Unchanged. |

### Conflict during `--minimal`

| Scenario | Behavior |
|---|---|
| `mill.yml` exists and `--force` is not set | `promptOverwrite` warns about the conflict. If user declines, init aborts. |
| `.mill/COMMON.md` exists | Preserved (skipped) — minimal mode respects preservation. |

### `--type` flag validation

| Scenario | Behavior |
|---|---|
| `--type` given an unrecognized value | `parseProjectType` returns an error: `"unknown project type: "foobar" (valid: go, js, monorepo, generic)"`. This is a hard error — init aborts. |
| `--type` given with `--minimal` | Allowed. The type is written to `mill.yml` but does not affect scaffold file selection (minimal always creates the same 4 files). |
| `--type` given without `--minimal` | Overrides detection. `detectProjectType` is still called but its result is discarded in favor of the explicit `--type` value. A note is printed: `"Using explicit project type: go (detected: js)"` |

### Backward compatibility

| Scenario | Behavior |
|---|---|
| `mill init` on a Go project (current happy path) | Detection returns `GoModule`. Full scaffold. No behavior change from current code. |
| `mill init` on a Go project with existing `.github/` | Previously: overwrote `.github/`. Now: preserves `.github/`. The user sees: `"Preserving existing .github/ directory"`. If the user wants overwrite, they use `--force`. |
| `mill init` on a non-Go project without `go` binary | Previously: `validatePreInit` failed with "go not found in PATH". Now: `validateBinaries` only checks `git`. If ProjectType is not GoModule, `go` is not required. |

---

## 6. Test strategy

### 6.1 `detectProjectType` tests (new: `init_test.go`)

Table-driven test with temporary directories:

| Test name | Setup | Expected |
|---|---|---|
| `TestDetectGoModule` | Create `go.mod` in target | `ProjectGoModule` |
| `TestDetectGoModuleInParent` | Create `go.mod` in parent of target | `ProjectGoModule` (walk-up works) |
| `TestDetectJSProject` | Create `package.json` in target | `ProjectJSProject` |
| `TestDetectMonoRepo` | Create `go.mod` at root, `package.json` in subdir, target=subdir | `ProjectMonoRepo` |
| `TestDetectGeneric` | Empty directory | `ProjectGeneric` |
| `TestDetectGoModWins` | Both `go.mod` and `package.json` at same level | `ProjectGoModule` (go.mod is stronger signal) |
| `TestDetectNoReadPermission` | Create dir without read permission during walk | `ProjectGeneric` + warning (no panic) |
| `TestDetectGoWorkIgnored` | Create `go.work` and `package.json` | `ProjectJSProject` (go.work not used for detection) |

**Test helper:** `createTempProject(t, files map[string]string) string` — creates a temp dir with the given file contents, returns the path. Used across all detection tests.

### 6.2 `copyScaffold` with preservation tests (new: `init_test.go`)

| Test name | Setup | Expected |
|---|---|---|
| `TestPreserveGithubDir` | Create `.github/ISSUE_TEMPLATE/bug.md` before init | `.github/` not overwritten; existing file untouched |
| `TestPreserveClaudeDir` | Create `.claude/CLAUDE.md` before init | `.claude/` not overwritten |
| `TestPreserveOmpDir` | Create `.omp/AGENTS.md` before init | `.omp/` not overwritten |
| `TestPreserveChecksDir` | Create `checks/gate-review` before init | `checks/` not overwritten |
| `TestOverwriteEmptyGithubDir` | Create empty `.github/` dir | `.github/` overwritten (empty = no user investment) |
| `TestOverwriteWithForce` | Create `.github/` with files, run init with `--force` | `.github/` overwritten |
| `TestPreserveIsPerDirectory` | Create `.github/` but not `.claude/` | `.github/` preserved, `.claude/` created fresh |
| `TestPreserveUnknownDir` | Create a `src/` directory | `src/` NOT preserved — only known harness dirs are preserved |

### 6.3 `--minimal` tests (new: `init_test.go`)

| Test name | Setup | Expected |
|---|---|---|
| `TestMinimalOnlyEssentialFiles` | Run `init --minimal` in empty dir | Only `mill.yml`, `.mill/COMMON.md`, `.mill/roles/staff/ROLE.md`, `.mill/roles/pm/ROLE.md` created |
| `TestMinimalNoGithubDir` | Run `init --minimal` | `.github/` does NOT exist |
| `TestMinimalNoChecks` | Run `init --minimal` | `checks/` does NOT exist |
| `TestMinimalNoSkills` | Run `init --minimal` | `.mill/skills/` does NOT exist |
| `TestMinimalRuntimeDirsExist` | Run `init --minimal` | `.mill/ledger/`, `.mill/worktrees/`, `.mill/phases/` exist |
| `TestMinimalSkipsDetection` | Run `init --minimal` in Go project | No "detected" message; mill.yml has `type: generic` (unless `--type` given) |
| `TestMinimalWithTypeGo` | Run `init --minimal --type go` | mill.yml has `type: go` |

### 6.4 Integration tests — full `runInit` paths (existing pattern, new cases)

| Test name | Setup | Expected |
|---|---|---|
| `TestInitDetectsGoModule` | Run `runInit` in temp dir with `go.mod` + `.git` + `go` on PATH | mill.yml has `type: go`; full scaffold created |
| `TestInitDetectsGeneric` | Run `runInit` in temp dir with `.git` but no sentinel files | Warning printed; mill.yml has `type: generic` |
| `TestInitPreservesExistingDirs` | Pre-create `.github/`, `.claude/` with files; run `runInit` | Dirs preserved; remaining scaffold created; no collision prompt for preserved dirs |
| `TestInitForceOverridesPreservation` | Pre-create `.github/`; run `runInit --force --yes` | `.github/` overwritten |
| `TestInitMinimalFlag` | Run `runInit --minimal --yes` in temp dir with `.git` | Only 4 files created; no warning about missing go.mod |

### 6.5 `validatePreInit` relaxation tests (modify existing)

| Test name | Change |
|---|---|
| `TestValidatePreInitNoGoMod` | Existing test expects error. NEW behavior: no error for generic projects. Test renamed/updated to `TestValidatePreInitNoGoModStillSucceeds` — validates that init does not abort, just warns. |
| `TestValidateBinariesMissingGo` | Existing test expects error. NEW behavior: `go` check only fires for `GoModule`/`MonoRepo`. Test split into: `TestValidateBinariesGoRequiredForGoModule` and `TestValidateBinariesGoOptionalForGeneric`. |

### 6.6 `parseProjectType` tests (new: `init_test.go`)

| Test name | Input | Expected |
|---|---|---|
| `TestParseProjectTypeGo` | `"go"` | `ProjectGoModule, nil` |
| `TestParseProjectTypeJS` | `"js"` | `ProjectJSProject, nil` |
| `TestParseProjectTypeMonoRepo` | `"monorepo"` | `ProjectMonoRepo, nil` |
| `TestParseProjectTypeGeneric` | `"generic"` | `ProjectGeneric, nil` |
| `TestParseProjectTypeEmpty` | `""` | `ProjectGeneric, nil` |
| `TestParseProjectTypeCaseInsensitive` | `"GO"` | `ProjectGoModule, nil` |
| `TestParseProjectTypeUnknown` | `"foobar"` | `ProjectGeneric, error` |

### Test isolation

All tests use `t.TempDir()` for filesystem operations. No test depends on the real filesystem or requires network access. The `detectProjectType` function accepts a `target string` parameter (not an `*App`), making it testable without wiring up the full App struct.

---

## Components affected

| File | Change |
|---|---|
| `internal/cli/init.go` | MODIFY: `runInit` adds `--minimal`, `--type` flags; `validatePreInit` relaxed; `generateMillYAML` gets ProjectType field; `copyScaffold` signature extended with `minimal bool` + preserve logic; NEW: `detectProjectType`, `preserveDir`, `dirIsNonEmpty`, `parseProjectType`, `ProjectType` type |
| `internal/cli/init_test.go` | MODIFY: Existing `validatePreInit` tests updated for relaxed Go requirement; NEW: detection table tests, preservation tests, minimal-mode tests, parseProjectType tests |
| `internal/cli/static/mill.yml.tmpl` | MODIFY: Add `type: {{.ProjectType}}` under `project:` |

### Files NOT affected

- `internal/config/config.go` — no new config types; ProjectType lives in mill.yml, not config.json
- `internal/domain/` — no new domain types
- `internal/adapter/` — no adapter changes
- `internal/state/` — no schema changes
- `internal/ledger/` — no ledger changes
- `cmd/mill/main.go` — no main changes

---

## Risks

### Risk 1: `validatePreInit` relaxation may surprise Go users
**Severity:** Low. **Mitigation:** The relaxation is additive — Go projects still work identically. The only change is that init no longer aborts when `go` is not on PATH for non-Go projects. Go users who accidentally run init outside a Go project will get a warning (not an error), which they can ignore or act on.

### Risk 2: Preservation may hide stale scaffold files from users
**Severity:** Low. **Mitigation:** When directories are preserved, `copyScaffold` prints a message: `"Preserving existing .github/ directory (use --force to overwrite)"`. Users are aware of what happened. The `--dry-run` flag shows what will be preserved before any writes occur.

### Risk 3: Monorepo detection requires walking the entire tree
**Severity:** Low. **Mitigation:** The walk stops at the filesystem root or the first `go.mod` found (whichever comes first). In a deep directory tree (e.g. `/home/user/projects/...`), this is at most ~10 `os.Stat` calls — negligible. The detection is O(depth) with no recursion into subdirectories.

### Risk 4: `--minimal` + `--type` combination is confusing
**Severity:** Low. **Mitigation:** The `--type` flag description clarifies: `"project type override (only affects mill.yml type field; ignored in --minimal mode)"`. In minimal mode, type is written to mill.yml but doesn't change file selection — this is documented in the flag help text.

### Risk 5: Empty directory detection for preservation is racy
**Severity:** Low. **Mitigation:** The check for empty directories (`dirIsNonEmpty`) uses `os.ReadDir`, which is atomic at the syscall level. Between the check and the write, another process could add files — but init is typically run interactively, not concurrently. If this becomes an issue, the `--force` flag is the escape hatch.

---

## ADR

**NEW ADR: ADR 0009 — Directory preservation by top-level skip, not merge.** Rationale:
- Merging individual files requires conflict resolution (which version wins? how to detect user modifications vs original scaffold?). This is a hard problem — `git merge` solves it with three-way diffs, but we have no "base" version to compare against.
- Skipping entire top-level directories is simple, predictable, and matches user expectations: "I have a custom `.github/`, don't touch it."
- The `--force` flag provides the escape hatch for users who want a fresh scaffold.
- If merge behavior is needed later, it can be added as a `--merge` flag with a three-way diff against the embedded scaffold snapshot. For now, YAGNI.

**Existing ADRs apply:**
- **ADR 0001** (Mill as Framework): The init flow remains the entry point to the framework. Smart detection makes the framework more accessible without changing its core model.
- **ADR 0002** (Budget Enforcement): No impact. Init does not consume budget.
- **ADR 0004** (Review Loop as CLI Concern): No impact. Init is a separate CLI command.

---

## Acceptance criteria

1. `mill init` on a Go project (go.mod present) sets `project.type: "go"` in mill.yml
2. `mill init` on a JS project (package.json, no go.mod) sets `project.type: "js"` in mill.yml
3. `mill init` on a project with neither go.mod nor package.json sets `project.type: "generic"` (with warning)
4. `mill init` on a monorepo (go.mod + package.json at different levels) sets `project.type: "monorepo"`
5. Existing `.github/`, `.claude/`, `.omp/` directories are preserved and NOT overwritten on re-init
6. `mill init --force` overwrites preserved directories
7. `mill init --minimal` creates only mill.yml + .mill/COMMON.md + staff ROLE.md + pm ROLE.md
8. `mill init --minimal` skips detection (project.type defaults to "generic")
9. `mill init --type go` overrides detection result
10. `go test ./internal/cli/ -run "TestDetect|TestPreserve|TestMinimal|TestParse|TestInitDetect|TestInitPreserves|TestInitForce|TestInitMinimal"` passes
11. `go test ./internal/cli/` passes (all existing + new tests)
12. Existing Go-project init flow is unchanged (backward compatible)
