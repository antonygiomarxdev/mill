# Tasks: Recursive Delegation — Automatic Chain-of-Command Delegation

> **Spec:** .mill/phases/109/spec.md | **FRD:** .mill/phases/109/frd.md

Recursion engine reads `delegates_to`, child worktrees with mill binary, phase contract pipeline, recursion guard, slot-aware recursion.

## New Components

- [ ] **Implement `RecursiveDelegator` orchestrating delegation chain**
  - role: sr-dev-be
  - Reads `delegates_to` from `ROLE.md` frontmatter, enforces leaf-termination, max-depth guard, cycle detection via visited-role set, triggers artifact handoff between phases, writes child worktree path to parent state
  - Files: `internal/recursion/engine.go` (NEW)
  - Testable: mock role frontmatter, verify delegation chain traversal, depth limit enforcement, cycle detection aborts with FATAL

- [ ] **Implement `BinaryCopier` copying mill binary to child worktree**
  - role: sr-dev-be
  - Copies `mill` binary to child worktree before spawning child session; FATAL classification on failure (permissions, cross-device link, disk full)
  - Files: `internal/recursion/binary.go` (NEW)
  - Testable: copy succeeds on valid path, FATAL on permission denied, FATAL on cross-device failure

- [ ] **Implement `ViewRenderer` formatting output as result or tree**
  - role: sr-dev-be
  - Formats output as final-result-only or full-tree based on `recursion.view` in `mill.yml`; tree view includes per-node role, artifact path, model tier, verdict, duration
  - Files: `internal/recursion/view.go` (NEW)
  - Testable: result view returns single artifact, tree view returns delegation tree with all node metadata

- [ ] **Implement `DelegationTree` tracking child worktree tree**
  - role: sr-dev-be
  - In-memory + persisted tree of child worktrees; tracks depth, per-node role, artifact path, classification, child worktree paths; persisted to `.mill/state/recursion.json`
  - Files: `internal/recursion/tree.go` (NEW)
  - Testable: tree builds correctly from delegation chain, persists to state file, reconstructs from disk

- [ ] **Implement `ChildSlotManager` wrapping slots.Manager for child worktrees**
  - role: sr-dev-be
  - Each child worktree gets independent slot pool (max 4 from config), separate from parent's pool
  - Files: `internal/slots/child.go` (NEW)
  - Testable: child slot count is independent of parent, max slots from config, child slots never consume parent slots

- [ ] **Implement `LevelLogger` writing per-level logs to recursion.jsonl**
  - role: sr-dev-be
  - Writes per-level logs (delegation depth, role, model, session ID, classification, duration, verdict) to `.mill/logs/recursion.jsonl`
  - Files: `internal/learning/level_logs.go` (NEW)
  - Testable: logs written in JSONL format with all required fields, append-only behavior

- [ ] **Implement `LessonsRecorder` appending per-role lessons.md**
  - role: sr-dev-be
  - Appends `lessons.md` to `.mill/roles/<role>/lessons.md` with per-role corrected patterns, gaps detected, acceptance criteria; caps at 50 most recent entries with summary compression
  - Files: `internal/learning/lessons.go` (NEW)
  - Testable: lessons appended correctly, 50-entry cap enforced, older entries compressed into summary block

- [ ] **Implement `CostResolver` mapping role model tier to actual model names**
  - role: sr-dev-be
  - Maps role frontmatter `model` tier to actual model names via `mill.yml` `recursion.models`; resolves fallback chains for `free→paid` roles
  - Files: `internal/recursion/cost.go` (NEW)
  - Testable: pro tier maps to deepseek-v4-pro, cheap maps to deepseek-v4-flash, free→paid resolves cheap with pro fallback on rate-limit

## Modified Components

- [ ] **Extend `delegate.go` to check recursion config and delegate to next role**
  - role: sr-dev-be
  - After producing and reviewing, checks `recursion` config; if enabled and delegating role has `delegates_to`, delegates to next role in chain; if leaf, returns normally
  - Files: `internal/cli/delegate.go` (MODIFY)
  - Testable: non-leaf role with recursion enabled delegates to next role, leaf role returns normally without delegation

- [ ] **Extend `review_loop.go` to validate child worktree output as phase contract**
  - role: sr-dev-be
  - Validates child worktree output as phase contract; gates on classification: CHANGES_REQUESTED triggers child iteration, FATAL/CONFIG_ERROR aborts and escalates, OK triggers handoff to next phase or recursion level
  - Files: `internal/cli/review_loop.go` (MODIFY)
  - Testable: CHANGES_REQUESTED triggers iteration (up to 3 attempts), FATAL aborts with escalation, OK triggers phase handoff

- [ ] **Update `slots.go` to reflect child worktree occupancy with recursive flag**
  - role: sr-dev-be
  - Slot status reflects child worktree occupancy; `--recursive` flag shows both parent and child pools
  - Files: `internal/cli/slots.go` (MODIFY)
  - Testable: status shows parent slots, with `--recursive` also shows child worktree slots, pools remain independent

- [ ] **Update `costs_56.go` to read model tier from role frontmatter**
  - role: sr-dev-be
  - Reads model tier from role frontmatter (`model: pro`, `model: free→paid`, `model: cheap`); resolves to actual model via CostResolver; removes hardcoded provider-specific defaults
  - Files: `internal/cli/costs_56.go` (MODIFY)
  - Testable: pro role resolves to deepseek-v4-pro, free→paid role resolves to flash with pro fallback, no hardcoded defaults used

- [ ] **Update `millyml.go` to add RecursionConfig struct**
  - role: sr-dev-be
  - Adds `RecursionConfig` struct (View, Models, MaxDepth) and CostModel mapping; `MillYML` gains `recursion: RecursionConfig` field
  - Files: `internal/config/millyml.go` (MODIFY)
  - Testable: YAML parses recursion section with View, Models, MaxDepth; MillYML struct has populated RecursionConfig field

- [ ] **Update `init.go` to scaffold recursion directories and lessons.md**
  - role: sr-dev-be
  - Scaffolds `.mill/logs/`, `.mill/state/`, and `.mill/roles/*/lessons.md` when initializing a worktree for recursive delegation
  - Files: `internal/cli/init.go` (MODIFY)
  - Testable: init creates all required directories, lessons.md file created in each role directory

- [ ] **Wire RecursionEngine into App struct in `app.go`**
  - role: sr-dev-be
  - Wires `RecursionEngine` into `App` struct for `delegate` command; conditionally activates based on `recursion` config presence
  - Files: `internal/cli/app.go` (MODIFY)
  - Testable: engine is initialized when recursion config present, not initialized when absent, delegate command uses engine when active

- [ ] **Expose BinaryPath() in `adapter.go` for BinaryCopier**
  - role: sr-dev-be
  - Exposes `BinaryPath()` so `BinaryCopier` knows where mill executable lives for copying to child worktrees
  - Files: `internal/adapter/adapter.go` (MODIFY)
  - Testable: BinaryPath returns correct mill executable path

- [ ] **Update `ledger.go` to add parent_issue and depth fields**
  - role: sr-dev-be
  - Ledger entries gain optional `parent_issue` and `depth` field to reconstruct delegation tree for audit
  - Files: `internal/ledger/ledger.go` (MODIFY)
  - Testable: ledger entries include parent_issue and depth when present, tree reconstruction from ledger works

## Tests & Validation

- [ ] **Test phase contract pipeline gate validation**
  - role: sr-dev-be
  - Verify gate-frd validates FRD.md, gate-spec validates spec.md structure (Architecture, Risks keys), gate-tasks validates tasks.md, gate-review and gate-coverage for terminal phases
  - Testable: each gate rejects incomplete artifacts, accepts valid artifacts, ReviewLoop invokes correct gate per phase

- [ ] **Test model assignment by role tier**
  - role: sr-dev-be
  - Verify Staff/PM/Architect/Tech Lead resolve to pro tier, Sr Dev FE/BE/Data and QA/Docs resolve to cheap tier, free→paid roles use cheap with pro fallback on rate-limit
  - Testable: each role tier maps to correct model, CostResolver warns on thinking role with cheap tier, fallback triggers on rate-limit

- [ ] **Test recursive worktree cleanup on partial completion**
  - role: sr-dev-data
  - Verify depth-1 worktree not cleaned when depth-2 fails, intermediate worktrees removed only when entire branch resolved and merged to main, `mill clean --recursive` reconciles orphans against git worktree list
  - Testable: failed depth-2 preserves depth-1 worktree, successful full chain cleans all intermediate worktrees, orphan reconciliation works
