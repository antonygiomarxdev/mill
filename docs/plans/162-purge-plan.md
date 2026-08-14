# Spec: Purge execution substrate after ADR 0005

**Issue:** #162
**Author:** Architect
**Date:** 2026-08-14
**ADR:** [0005-orca-as-execution-substrate.md](../../docs/adr/0005-orca-as-execution-substrate.md)

---

## Architecture

### What changes

ADR 0005 makes Orca the execution substrate. Mill keeps the policy layer and
stops owning process spawning, worktree lifecycle, the message bus, concurrency
slots, and recursive delegation. Roughly 2,500 lines of Go become deletable.

The post-purge architecture is a thin CLI that reads configuration and roles,
constructs briefs, resolves model tiers, and delegates all execution to Orca
through one narrow `internal/orca` module. The coordinator (Orca) decides
sequencing; Mill no longer walks a delegation tree.

```
┌─────────────────────────────────────────────────────┐
│                    mill CLI                          │
│  init · role · status · land · version · costs      │
├──────────────┬──────────────┬───────────────────────┤
│  internal/   │  internal/   │  internal/orca        │
│  config      │  role        │  (thin Orca client)   │
│  domain      │  issue       │                       │
│  state       │  ledger      │  → orca orchestration │
│  learning    │              │    task-create         │
│              │              │    worker-start        │
│              │              │    send / check /reply │
└──────────────┴──────────────┴───────────────────────┘
```

### What Mill keeps (per ADR 0005)

| Concern | ADR 0005 clause | Package |
|---|---|---|
| Eleven role definitions and capabilities | "Mill owns" §1 | `internal/role` |
| Phase sequence (FRD → spec → tasks → impl → review) | "Mill owns" §2 | `internal/cli` (reduced) |
| Role-enforce: what each role may write | "Mill owns" §3 | `internal/cli` (hooks_enforce, simplified) |
| Phase gates and acceptance criteria | "Mill owns" §4 | `internal/cli` (gate commands) |
| Brief construction from role + issue + artifact | "Mill owns" §5 | `internal/cli` (issue_context) |
| Model tier selection per dispatch | "Mill owns" §6 | `internal/cli` (routing_56) |

### What Orca takes over

| Concern | Orca command | Was in Mill |
|---|---|---|
| Spawn and supervise workers | `worker-start`, `worker-show/read/stop/abandon` | `internal/adapter`, `internal/cli/delegate.go` |
| Worktree lifecycle | `worktree create/rm/ps` | `internal/cli/delegate.go`, `internal/cli/clean.go`, `internal/recursion` |
| Message bus and raise-a-hand | `send`, `ask`, `check`, `reply`, `inbox` | `internal/cli/review_loop.go` |
| Parallelism and limits | Orca's coordinator scheduling | `internal/slots` |

---

## Components affected

### Package verdicts

#### `internal/adapter` (688 lines) — **DELETE**

Orca provides: `worker-start` spawns and supervises workers;
`worker-read/show/stop/abandon` manage lifecycle; the coordination protocol is
injected into each worker's prompt.

The entire `Adapter` interface, `CommandCodeAdapter`, `Session`, `DispatchOpts`,
`Budget`, `SessionResult`, heartbeat file management, and process spawning are
replaced by Orca orchestration primitives. Nothing in the policy layer depends
on adapter types except `config` (which imports `adapter.Budget` — that type
moves to `internal/domain` or `internal/config`).

#### `internal/cli` (4,736 lines) — **REDUCE**

The CLI root stays. Subcommands change.

| File | Verdict | Rationale |
|---|---|---|
| `app.go` | **REDUCE** | CLI root stays; remove adapter/slots/recursion wiring, add `internal/orca` wiring |
| `delegate.go` | **DELETE** | Entire delegation mechanism replaced by Orca `task-create` + `worker-start` |
| `review_loop.go` | **DELETE** | Produce→review→rework cycle replaced by Orca coordinator sequencing |
| `routing_56.go` | **KEEP** | Model tier selection is explicitly Mill-owned (ADR 0005 "Mill owns" §6). Remove adapter availability checks. |
| `init.go` | **KEEP** | Project scaffolding is not substrate |
| `clean.go` | **DELETE** | Worktree cleanup is Orca's lifecycle (`worktree rm`) |
| `compact.go` | **DELETE** | Conversation compaction is the agent runtime's concern (Command Code's `/compact`) |
| `compact_integration.go` | **DELETE** | Auto-compact during dispatch loop; dispatch loop is now Orca's |
| `costs_56.go` | **KEEP** | Cost tracking is policy (feeds model tier decisions) |
| `hooks_enforce.go` | **REDUCE** | `role-enforce` concept is Mill-owned; hook installation into worktrees is Orca's. Keep the script, remove the worktree installation logic. |
| `issue_context.go` | **REDUCE** | Brief construction is Mill-owned. Reformat to produce Orca briefs instead of `cmd` prompts. |
| `land.go` | **REDUCE** | Gate-running and merge logic stays; worktree location via `git worktree list` changes to Orca worktree paths |
| `logging.go` | **REDUCE** | Structured logging stays; remove adapter-specific fields (heartbeat, process PID) |
| `role.go` | **KEEP** | `mill role get/set` is pure policy |
| `slots.go` | **DELETE** | Concurrency is Orca's concern |
| `slot_app.go` | **DELETE** | See above |
| `slot_delegate.go` | **DELETE** | See above |
| `status.go` | **REDUCE** | Status display stays; reads Orca task state instead of `.mill/state.json` session details |
| `version.go` | **KEEP** | Always needed |
| `watch.go` | **DELETE** | Polling `.mill/state.json` replaced by Orca worker supervision |
| `static/` | **KEEP** | Scaffold files for `mill init` |
| All `*_test.go` for deleted files | **DELETE** | Tests for deleted code |

Lines removed from `internal/cli`: ~3,200 of 4,736 (delegate: ~600, review_loop: ~550, clean: ~200, watch: ~180, slots: ~400, compact: ~300, compact_integration: ~200, hooks_enforce: ~100, plus tests).
Lines remaining: ~1,500.

#### `internal/compact` (257 lines) — **DELETE**

Conversation compaction is the agent runtime's responsibility. Command Code
manages its own context window. Mill never sees the conversation tokens.

#### `internal/config` (155 lines) — **KEEP**

`mill.yml` and `config.json` are project configuration. Model tier mappings,
project name, and phase gate settings survive. Remove `Concurrency.MaxSlots`
(Orca's concern) and `adapter.Budget` import (move Budget to domain or config).

#### `internal/domain` (395 lines) — **KEEP**

Pure domain model with zero infrastructure imports. `Task`, `TaskStatus`,
`TaskPhase`, `Verdict`, `Classification`, `FailureClass`, `Signal` are Mill's
policy vocabulary. `Session` and `SessionStatus` slim down (Mill no longer owns
sessions), but the task lifecycle and failure classification are core policy.

#### `internal/issue` (205 lines) — **KEEP**

GitHub issue reading and acceptance criteria extraction. Brief construction
(Mill-owned) depends on this. Spawns `gh` CLI for data retrieval, not agent
execution.

#### `internal/learning` (455 lines) — **REDUCE**

Per-role lesson recording stays (policy: lessons inform future briefs).
Per-level recursion logging (`level_logs.go`) goes — the recursive delegation
engine is deleted. The `LessonsRecorder` and its compression logic survive.

#### `internal/ledger` (100 lines) — **REDUCE**

Append-only event logging stays (Mill tracks its decisions). Event types change:
remove session-level events (dispatch PID, heartbeat, commit count from adapter),
add Orca task references (task ID, dispatch ID). Interface survives, content
changes.

#### `internal/lessons` (70 lines) — **DELETE** (merge into `internal/learning`)

Overlapping package (#122). `internal/learning` already has a more capable
`LessonsRecorder`. Merge the simpler `Append`/`AppendResult` functions into
`internal/learning` and delete this package.

#### `internal/recursion` (637 lines) — **DELETE**

ADR 0005's star topology dissolves recursive delegation: "The coordinator is a
hub. It dispatches to a role, receives the result, decides the next step —
one-to-N, not one-to-one-to-one." The `Delegator`, `DelegationTree`,
`BinaryCopier`, `CostResolver`, and `ViewRenderer` are replaced by Orca's
coordinator dispatching to workers. Issues #153, #109 are dissolved.

`CostResolver`'s model-tier-to-model-name mapping is policy and moves to
`internal/config` or a reduced `routing_56.go`.

#### `internal/role` (141 lines) — **KEEP**

Role loading and frontmatter parsing. ADR 0005: "the eleven role definitions and
their capabilities" is Mill-owned. Brief construction depends on this.

#### `internal/slots` (531 lines) — **DELETE**

ADR 0005: Orca owns "parallelism and its limits". The `Manager`,
`ChildSlotManager`, goroutine pool, and hard-limit reclaimer are replaced by
Orca's coordinator scheduling. Issues #141, #142 are dissolved.

#### `internal/state` (160 lines) — **REDUCE**

Persistent task state stays (Mill tracks which task is in which phase). Remove
session-level fields (PID, heartbeat path, worktree path, process status) —
those are Orca's concern now. The state format simplifies to phase + verdict +
Orca task reference.

---

### What must be written: `internal/orca`

One narrow module, per ADR 0005's coupling note: "Mill should call Orca
through one narrow internal module rather than scattering invocations, so a
breaking change is a single repair."

```
internal/orca/
    client.go      — OrcaClient struct wrapping os/exec calls to `orca`
    task.go        — TaskCreate, TaskUpdate, TaskList
    worker.go      — WorkerStart, WorkerRead, WorkerStop, WorkerAbandon
    message.go     — Send, Check, Reply, Ask (the raise-a-hand cycle)
    worktree.go    — WorktreeCreate, WorktreeRm, WorktreePs
    types.go       — Thin Go types mirroring Orca's JSON responses
```

Every `orca orchestration ...` invocation in the codebase goes through this
module. When Orca changes a flag shape (`reply --id` vs `worker-show
--dispatch`), the repair is in one file.

The module shells out to `orca` CLI (same as `internal/issue` shells out to
`gh`). If Orca exposes a Go SDK later, the module is the single swap point.

---

## Deletion order

Each step leaves `go build ./...` and `go test ./...` green.

| Step | Action | Why this order |
|---|---|---|
| 1 | Delete `internal/lessons/` — merge `Append`/`AppendResult` into `internal/learning` | No external dependents. Resolve #122 overlap first. |
| 2 | Delete `internal/slots/` — remove `slots.go`, `slot_app.go`, `slot_delegate.go` from `internal/cli` | Only `cli` and `config` reference it. Remove cli references first. |
| 3 | Delete `internal/recursion/` — remove recursion references from `internal/cli/app.go` | Only `cli` references it. |
| 4 | Delete `internal/compact/` — remove `compact.go`, `compact_integration.go` from `internal/cli` | Only `cli` references it. |
| 5 | Delete `internal/adapter/` — remove `delegate.go`, `review_loop.go`, `clean.go`, `watch.go` from `internal/cli` | The big cut. Remove all cli files that import adapter first, then delete the package. Also remove `adapter.Budget` from `internal/config`. |
| 6 | Delete `internal/cli/` substrate files: `delegate.go`, `review_loop.go`, `clean.go`, `watch.go`, `compact.go`, `compact_integration.go`, `slots.go`, `slot_app.go`, `slot_delegate.go`, `hooks_enforce.go` (installation part) | Now safe — their imports are gone. |
| 7 | Create `internal/orca/` — the narrow Orca client module | Foundation for the new wiring. |
| 8 | Rewrite `internal/cli/app.go` — wire to `internal/orca` instead of adapter/slots | CLI compiles against new module. |
| 9 | Reduce `internal/cli/issue_context.go` — produce Orca briefs | Brief construction is Mill-owned. |
| 10 | Reduce `internal/cli/routing_56.go` — move `CostResolver` logic from deleted recursion | Model tier selection survives. |
| 11 | Reduce `internal/state/` — remove session-level fields | Simplify to phase + verdict + Orca task ref. |
| 12 | Reduce `internal/ledger/` — update event types | Replace adapter events with Orca task references. |
| 13 | Reduce `internal/learning/` — remove `level_logs.go` | Recursion logging is gone. |
| 14 | Reduce `internal/domain/` — slim `Session`/`SessionStatus` | Mill no longer owns sessions. |
| 15 | Reduce `internal/config/` — remove `Concurrency.MaxSlots`, move `Budget` to domain | Slots are Orca's concern. |

---

## Risks

### 1. Orca CLI surface instability

Flag shapes already vary between sibling commands (`reply --id`, `worker-show
--dispatch`, `inbox` rejecting `--run`). A breaking change in Orca's CLI breaks
Mill.

**Mitigation:** The `internal/orca` module isolates all CLI calls. A breaking
change is a single-file repair. ADR 0005 already identifies this risk.

### 2. Model tier selection for non-Claude agents

ADR 0005 notes that `--model` accepts only Claude, Codex, and Cursor identifiers.
The cost model (expensive decompose, cheap execute) depends on per-dispatch model
choice, which Orca does not fully provide for non-Claude agents.

**Mitigation:** Mill states the tier in the brief (as Orca's own implementer
template does). This is an acknowledged gap, not a blocker.

### 3. Fresh-install dependency

ADR 0005 notes this works because the machine already has Claude, Command Code,
and Orca configured. A fresh install has none of them.

**Mitigation:** `mill init` should check for `orca` in PATH and fail with a
clear message. This is a separate issue, not a purge blocker.

### 4. Loss of test coverage during transition

Deleting 2,500 lines of tests without replacing them with Orca-integration tests
creates a coverage gap. The remaining policy code (routing, brief construction,
role loading) must maintain ≥90% coverage.

**Mitigation:** Each deletion step runs `go test ./...` to confirm the build
stays green. New tests for `internal/orca` mock the CLI. Policy-layer tests
are unchanged.

### 5. `internal/orca` becomes a second adapter abstraction

There is a risk of rebuilding the same abstraction ADR 0005 just killed.

**Mitigation:** `internal/orca` is a thin CLI wrapper, not an interface with
multiple implementations. It has no `Adapter` trait, no `Session` lifecycle, no
heartbeat management. It is `os/exec` + JSON parsing, nothing more. ADR 0005
explicitly defers the strategy seam (#161): "building an abstraction over one
implementation is speculative."

### 6. Phase gate scripts reference substrate artifacts

Gate scripts in `checks/` may assert on files or structures that change shape.

**Mitigation:** Audit `checks/` during step 8 (app.go rewrite). Gate scripts
that reference session state, worktree paths, or adapter output need updating.

---

## Obsolete GitHub issues

Issues made obsolete by ADR 0005 and this purge. Each is dissolved, not fixed —
the substrate that contained the defect no longer exists.

| Issue | Title | Why obsolete |
|---|---|---|
| #153 | Roles never told to delegate onward | Star topology dissolves: coordinator dispatches, not chain |
| #157 | No supervisor — dead delegation stays dead | Orca provides `worker-start/show/read/stop/abandon/release` |
| #154 | `delegates_to` cannot express fan-out | Parallel worktrees are Orca's core function |
| #156 | Nothing writes back to the issue | Orca injects coordination protocol into each worker's prompt |
| #146 | Re-delegation destroys worktree | Orca manages worktree lifecycle (`worktree create/rm/ps`) |
| #101 | `mill clean --all` leaves branches | Worktree cleanup is `worktree rm` |
| #91 | (worktree lifecycle) | Orca manages worktree lifecycle |
| #92 | (task status updates) | `task-update`, `terminal wait` |
| #119 | Status shows growing runtime | State simplifies; Orca tracks worker lifecycle |
| #141 | Slots hard-limit kills every delegation | Slots are Orca's concern |
| #142 | Make slot limit configurable | Slots are Orca's concern |
| #148 | Agent set `core.bare=true` | Mill no longer creates worktrees |
| #144 | Dead worktree's hooks outlive it | Orca manages worktree lifecycle |
| #135 | `delegate` requires GitHub issue | `delegate` command goes away |
| #138 | `delegate --model` silently discarded | Adapter goes away |
| #127 | Single provider per installation | Orca handles multiple agents natively |
| #109 | Recursive delegation doesn't work | Star topology replaces recursive delegation |
| #107 | `cmd` CLI fails with IPv6 | Mill doesn't call `cmd` directly |
| #106 | `classifyResult` misclassifies | Adapter and classification go away |
| #115 | Harness generates content | Orca drives agents |
| #118 | `--wait` ignored | Orca handles worker lifecycle |
| #117 | `delegate` overwrites `core.hooksPath` | `delegate.go` goes away |
| #114 | Resume doesn't init budget | Adapter resume goes away |
| #99 | `Capabilities().Models` static | Adapter goes away |
| #98 | produce-review cycle fails | Orca coordinator handles dispatch sequencing |
| #97 | Async mode kills agent | `delegate.go` goes away |
| #123 | `buildArgs` no optional tools | Adapter goes away |
| #130 | `-wait`/`-max-turns` silently ignored | `delegate.go` goes away |
| #158 | Adapter captures neither output nor stderr | Adapter goes away |

**Total: 28 issues dissolved.**

Issues that survive (still relevant to the policy layer): #139, #160, #159,
#155, #152, #151, #149, #140, #137, #136, #134, #133, #132, #131, #129, #128,
#125, #122, #111, #110, #108, #96.
