# Mill Runner MVP — Implementation Plan
Date: 2026-08-09
Issue: #14
Role: sr-dev-be
Model: free
Reviewed by: staff

## Goal

Bootstrap the Mill runner: a Go binary at `cmd/mill/main.go` that dispatches AI
agents, classifies output, repairs tool calls, and persists state. This MVP
covers the core CLI surface (`delegate`, `status`) with TDD-verified modules.

## Acceptance Criteria (Gate)

1. `go build -o mill ./cmd/mill` succeeds (exit 0)
2. `./mill status` prints a table header (even when empty), exit 0
3. `./mill delegate` prints a usage error, exit 1
4. `go test ./...` passes all tests
5. `grep -r "SIN_CREDITO\|TRANSITORIO\|LIMITE" internal/` returns zero matches
6. `.mill/state.json` is created on first delegate run
7. `.mill/ledger/<issue>.jsonl` has append-only entries

## Do Not Touch

- `roles/` — role definitions
- `docs/` — documentation (read-only reference)
- `skills/` — skills snapshot

## Architecture Reference

From `ARCHITECTURE.md`:

- **Adapter interface**: `Dispatch(worktree, prompt, model) → Session`,
  `Resume(sessionID) → Session`, `Capabilities() → AdapterCapabilities`
- **Session interface**: `id`, `status` (running|done|error), `subscribe fn`, `wait → SessionResult`
- **SessionResult**: `exitCode`, `commits`, `verdict` (approved|changes|rejected)
- **Task lifecycle**: dispatch → produce → review → reword → review → APPROVED → land
  (max 4 rounds, then REJECTED)
- **State persistence**: `.mill/state.json` (task states), `.mill/ledger/<issue>.jsonl` (session events), `.mill/config.json` (provider config)
- **Models**: review uses *caro* (deepseek-v4-pro), production uses *barato* (laguna-free / deepseek-v4-flash)

## 11 TDD Tasks

### Task 1 — Scaffold

- Create `go.mod` (module `github.com/antonygiomarxdev/mill`, `go 1.26`)
- Create `cmd/mill/main.go` — minimal `main()` calling `cli.Execute()`
- Create `internal/cli/root.go` — cobra root command
- Verify `go build -o mill ./cmd/mill` succeeds
- Commit

### Task 2 — Config

- `internal/config/config.go`:
  - `Config` struct: `Provider`, `Model`, `MaxRounds`
  - `Default()` returns sane defaults
  - `Load(path)` reads JSON, falls back to `Default()`
  - `Save(path)` writes JSON, creates `.mill/` dir
- Test first (TDD)
- Commit

### Task 3 — State

- `internal/state/state.go`:
  - `TaskState` struct: `ID`, `Issue`, `Status`, `Commits`, `Verdict`
  - `State` struct: `Tasks map[string]TaskState`
  - `Load(path)` → empty `State` if file missing
  - `Save(path)` → writes JSON, creates `.mill/` dir
  - `UpsertTask(t TaskState)` → inserts/updates
  - `Task(id)` → lookup
- Test first (TDD)
- Commit

### Task 4 — Ledger

- `internal/ledger/ledger.go`:
  - `Entry` struct: `Timestamp`, `Issue`, `Event`, `Status`, `Verdict` (omitempty)
  - `Append(path, entry)` → opens file in append mode, writes JSON line, creates `.mill/ledger/` dir
- Test first (TDD)
- Commit

### Task 5 — Issue parsing

- `internal/issue/issue.go`:
  - `Parse(s)` → `(int, error)` — parses issue number from string, must be positive
  - `MustParse(s)` → `int` (panics on error, for internal use)
- Test first (TDD)
- Commit

### Task 6 — CLI commands

- `internal/cli/status.go` — status subcommand skeleton
- `internal/cli/delegate.go` — delegate subcommand skeleton
- Register both on root command
- Commit

### Task 7 — Status command

- `./mill status` prints table header via `text/tabwriter`:
  ```
  ID    ISSUE  STATUS     COMMITS  VERDICT
  ```
- Always exits 0 (even when empty)
- Test the output
- Commit

### Task 8 — Delegate command

- `Args: cobra.ExactArgs(1)` → `./mill delegate` (no args) prints usage error, exit 1
- On valid args:
  - Parse issue number
  - Create `.mill/state.json` if missing
  - Upsert task in state
  - Append ledger entry
- Test both paths
- Commit

### Task 9 — Session model

- `internal/session/session.go`:
  - `Status` type: `Pending`, `Running`, `Done`, `Error`
  - `Verdict` type: `Approved`, `Changes`, `Rejected`
  - `Session` struct: `ID`, `Issue`, `Status`, `Commits`, `Verdict`
  - `NewSession(issue)` → `Session` with `Status=Pending`
- Test first (TDD)
- Commit

### Task 10 — Adapter interface

- `internal/adapter/adapter.go`:
  - `Capabilities` struct: `Models []string`
  - `SessionResult` struct: `ExitCode`, `Commits`, `Verdict`
  - `Session` interface: `ID() string`, `Status() string`, `Wait() SessionResult`
  - `Adapter` interface: `Dispatch(...)`, `Resume(...)`, `Capabilities()`
- Test with a mock adapter
- Commit

### Task 11 — Verification

- `go mod tidy`
- `go build -o mill ./cmd/mill`
- `go test ./...`
- Verify all acceptance criteria
- Comment on issue #14
- Final commit
