# Spec: Recursive Delegation — Automatic Chain-of-Command Delegation

## Architecture

### Core design

Recursive delegation extends Mill's single-level produce→review loop into a multi-level delegation chain. The recursion engine reads the delegating role's `delegates_to` list from its `ROLE.md` frontmatter, resolves the next role via the routing table, creates a child worktree, copies the `mill` binary, and invokes the child session. This continues until a leaf role (no `delegates_to`) executes the terminal task.

**Delegation tree** — A tree of worktrees, each with its own binary copy, independent slot manager (max 4), and `state.json`. Tree depth is bounded by the ORG-CHART chain: Staff → PM/Architect → Tech Lead → Sr Dev → QA. No arbitrary depth beyond role hierarchy (P0 scope limit).

**Phase contract pipeline** — Artifacts flow as immutable contracts guarded by phase gates. Each gate validates the prior artifact before the next role can accept the handoff:

```
PM         →  FRD.md         (gate-frd)
Architect  →  spec.md        (gate-spec)   [1..N specs per FRD]
Tech Lead  →  tasks.md       (gate-tasks)
Sr Dev     →  implementation (leaf)
```

**Recursion guard** — A `delegationDepth` counter in the delegation state is compared against the org-chart depth. Leaf roles (`delegates_to: []`) terminate recursion. A visited-role set detects cycles; detection aborts with `FATAL` and escalates to the delegator's reviewer.

### Data flow

1. User delegates at the parent worktree (typically Staff).
2. `RecursionEngine` reads the delegating role's `ROLE.md` frontmatter — `delegates_to`, `model`, `reviewed_by`, `skills`.
3. Parent resolves the target role via the routing table, creates a child worktree (`git worktree add`), and copies the `mill` binary into it (`BinaryCopier`). If the binary cannot be placed, recursion aborts — FATAL.
4. Child session starts with its role-appropriate model. Model tier is resolved from the role's `model` frontmatter field (`pro` for thinking roles, `cheap`/free→paid for executors), **not** from recursion depth.
5. Child produces its phase artifact (FRD → spec → tasks → code) and its `ReviewLoop` validates it against the phase gates.
6. If approved and the child role is non-leaf, the child itself enters delegation mode → recursion continues. If the child is a leaf role, it produces the final implementation and the chain unwinds.
7. Per-role `lessons.md` and per-level logs are written into each worktree's `.mill/` tree and preserved on worktree removal (exported to parent).

### Configuration

`mill.yml` gains a `recursion` section:

```yaml
recursion:
  view: result | tree        # result = final artifact only; tree = full delegation tree
  models:
    pro: deepseek-v4-pro     # thinking roles
    cheap: deepseek-v4-flash # executing roles
  max-depth: auto            # derived from ORG-CHART at startup
```

The existing `concurrency.max-slots` (default 4) governs the slot manager. Each child worktree inherits this limit but gets its **own independent** slot pool — child slots never consume parent slots.

### Model assignment by role tier

| Role                              | tier  | model                              |
|-----------------------------------|-------|------------------------------------|
| Staff, PM, Architect, Tech Lead   | pro   | deepseek-v4-pro / claude-sonnet-5  |
| Sr Dev (FE/BE/Data), QA/Docs      | cheap | deepseek-v4-flash / laguna-s-2.1-free |

Assignment reads `model` from the role frontmatter at dispatch time. A `free→paid` role (Sr Dev, QA) resolves to the cheap tier with pro fallback on rate-limit — per the existing model resilience spec.

## Components affected

### New

- **`internal/recursion/engine.go`** — `RecursiveDelegator`: orchestrates the delegation chain. Reads `delegates_to` from role frontmatter, enforces leaf-termination and max-depth guard, detects cycles via visited-role set, triggers artifact handoff between phases, writes child worktree path to parent state.
- **`internal/recursion/binary.go`** — `BinaryCopier`: copies the `mill` binary to the child worktree before spawning the child session. Fails fast (FATAL classification) if the binary cannot be placed, blocking recursion.
- **`internal/recursion/view.go`** — `ViewRenderer`: formats output as final-result-only or full-tree based on `recursion.view` in `mill.yml`. Tree view includes per-node role, artifact path, model tier, verdict, and duration.
- **`internal/recursion/tree.go`** — `DelegationTree`: in-memory + persisted tree of child worktrees. Tracks depth, per-node role, artifact path, classification, and child worktree paths. Persisted to `.mill/state/recursion.json`.
- **`internal/slots/child.go`** — `ChildSlotManager`: wraps the existing `slots.Manager` so each child worktree gets its own independent slot pool (max 4 from config), separate from the parent's pool.
- **`internal/learning/level_logs.go`** — `LevelLogger`: writes per-level logs (delegation depth, role, model, session ID, classification, duration, verdict) to `.mill/logs/recursion.jsonl`.
- **`internal/learning/lessons.go`** — `LessonsRecorder`: appends `lessons.md` to `.mill/roles/<role>/lessons.md` with per-role corrected patterns, gaps detected, and acceptance criteria.
- **`internal/recursion/cost.go`** — `CostResolver`: maps role frontmatter `model` tier to actual model names via `mill.yml` `recursion.models`; resolves fallback chains for `free→paid` roles.

### Modified

- **`internal/cli/delegate.go`** — Main delegation path. After producing and reviewing, checks `recursion` config. If enabled and the delegating role has `delegates_to`, delegates to the next role in the chain instead of returning. If the role is a leaf, returns normally.
- **`internal/cli/review_loop.go`** — Extends produce→review cycle to validate child worktree output as a phase contract. Gates on classification: `CHANGES_REQUESTED` → child iterates; `FATAL` / `CONFIG_ERROR` → abort and escalate; `OK` → handoff to next phase or next recursion level.
- **`internal/cli/slots.go`** — Slot status reflects child worktree occupancy. Parent and child slot pools are independent; status command shows both if `--recursive` flag is set.
- **`internal/cli/costs_56.go`** — Reads model tier from role frontmatter (`model: pro` or `model: free→paid` or `model: cheap`), resolves to actual model via `CostResolver`. Removes hardcoded provider-specific model defaults.
- **`internal/config/millyml.go`** — Adds `RecursionConfig` struct (`View`, `Models`, `MaxDepth`) and `CostModel` mapping. `MillYML` gains `recursion: RecursionConfig`.
- **`internal/cli/init.go`** — Scaffolds `.mill/logs/`, `.mill/state/`, and `.mill/roles/*/lessons.md` when initializing a worktree for recursive delegation.
- **`internal/cli/app.go`** — Wires `RecursionEngine` into the `App` struct for the `delegate` command; conditionally activates based on `recursion` config presence.
- **`internal/adapter/adapter.go`** — Exposes `BinaryPath()` so the `BinaryCopier` knows where the `mill` executable lives to copy it to child worktrees.
- **`internal/role/role.go`** — Parses `delegates_to`, `model`, `reviewed_by`, `skills` from `ROLE.md` frontmatter (already parsed; consumed by the engine, no change to parsing logic needed).
- **`internal/ledger/ledger.go`** — Ledger entries gain an optional `parent_issue` and `depth` field to reconstruct the delegation tree for audit.

### Unchanged (consumed as-is)

- **`internal/domain/`** — `Classification`, `Verdict`, `TaskStatus` used unchanged by the engine.
- **`internal/issue/`** — Issue context flows through the tree as the delegation payload; `reviewed_by` field on the issue tracks the reviewing role.
- **`internal/state/`** — Child worktrees get their own `state.json`; parent state records child worktree paths for cleanup.
- **`checks/gate-{frd,spec,tasks,review,coverage}`** — Phase gates invoked by `ReviewLoop` for each phase handoff; validation logic unchanged, only invocation is extended.
- **`internal/cli/land.go`** — Land command cleans up child worktrees after merge, walking the delegation tree.
- **`internal/cli/slot_delegate.go`** — Existing single-level slot delegation reused for child-initiated delegations; child slot manager wraps the same logic.
- **`internal/compact/`** — Compaction applies to the tree root; child logs compacted when child worktree is removed.

## Risks

1. **Recursion explosion / infinite loops** — A misconfigured `delegates_to` cycle (A→B→A) would recurse forever. Mitigated by: max-depth cap read from ORG-CHART at startup (P0 scope: depth bounded by role hierarchy), visited-role set per delegation branch, and leaf-termination guard. Cycle detection aborts with `FATAL` and escalates to the delegator's reviewer.

2. **Binary availability on copy failure** — If copying the `mill` binary to the child worktree fails (permissions, cross-device link, disk full), the child cannot bootstrap. `BinaryCopier` treats this as FATAL and blocks the recursion — parent retains ownership and reports the failure rather than attempting a childless spawn.

3. **Slot exhaustion deadlock** — With max 4 slots, deep recursion within a single worktree could queue branches indefinitely. Mitigated by: each child worktree gets its **own independent** slot pool — parent slots are never consumed by children. Within a single worktree, concurrent recursive branches beyond 4 are queued (acceptable — the pipeline is fundamentally sequential by phase). No fan-out beyond slot limit.

4. **Phase gap between artifacts** — If the Architect produces a spec that fails `gate-spec` (e.g., missing "Architecture" or "Risks" key), the Tech Lead `ReviewLoop` must catch it before tasks are written. The loop validates artifact structure via gates before allowing handoff; `CHANGES_REQUESTED` triggers child iteration (not escalation); only `FATAL`/`CONFIG_ERROR` abort the chain.

5. **Model misassignment** — A thinking-tier role receiving a cheap model would degrade quality on reasoning-heavy phases (spec, tasks). Model assignment reads the role's `model` frontmatter at dispatch time, not recursion depth. Mitigation: `CostResolver` warns if a thinking role (`model: pro`) is resolved to a cheap tier, and Sr Dev/QA (`model: free→paid`) always get cheap-with-pro-fallback — never hard-pro.

6. **Lessons drift / stale learning** — `lessons.md` is append-only and accumulates per role; without bounds it grows unbounded or captures low-signal corrections. Mitigation: per-role lessons capped at 50 most recent entries; older entries compressed into a summary block. `LevelLogger` writes structured JSONL for machine processing.

7. **State fragmentation / orphan worktrees** — Each child worktree has its own `state.json`; the parent must track all child worktree paths for cleanup. If a child crashes mid-recursion and the parent loses track of the path, orphan worktrees accumulate. The engine writes child worktree paths to parent `state.json` **before** spawning; `mill clean --recursive` reconciles orphans against git worktree list.

8. **Observability debt** — Long-chain progress is not streamed in real time. The FRD explicitly defers this to issue #105. `ViewRenderer` shows only terminal state (final result or full tree post-completion). Users cannot observe mid-chain progress of a 4-level delegation in flight — they see the tree only after it completes or fails.

9. **Failure escalation policy undefined** — The FRD notes #4 (failure policy) is pending PM clarification. Until resolved, the engine uses a placeholder: `FATAL`/`CONFIG_ERROR` → abort chain and escalate to delegator's reviewer; `CHANGES_REQUESTED` → child iterates within its phase loop (up to 3 attempts before escalating); `RATE_LIMITED`/`TRANSIENT` → model fallback chain retry, then escalate. This policy must be revisited when #4 is resolved.

10. **Worktree cleanup on partial completion** — If recursion succeeds at depth 1 (e.g., Architect produces spec) but depth 2 fails (Tech Lead rejects tasks), the depth-1 worktree must not be cleaned up — its output may still be valid. Cleanup is gated on full subtree completion and merge. Intermediate worktrees are removed only when the entire branch is resolved and merged to main.
