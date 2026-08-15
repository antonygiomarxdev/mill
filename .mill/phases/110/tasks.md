# Tasks: Clasificación de fallo — taxonomía de fallos de rol y de entorno (#110)

All tasks modify/update only files listed in the SPEC "Components affected" table.

## Wave 1 (parallel — domain foundations, no deps)

- [ ] **Add `FailureClass` enum + `FailureClassOf` mapper** — role: sr-dev-be, deps: none, est: 20m
  1. In `internal/domain/classification.go`: add enum `FailureClass` (`CLASS_OK | EXECUTION_FAILURE | CONTRACT_FAILURE | GATE_FAILURE | RESULT_FAILURE | ENVIRONMENT_FAILURE`).
  2. Add `FailureClassOf(Classification) FailureClass` mapping: `FATAL/TRANSIENT/RATE_LIMITED → EXECUTION_FAILURE`; `BLOCKED → ENVIRONMENT_FAILURE` (slot/binary cause) or `EXECUTION_FAILURE` (budget-timeout cause); `CHANGES_REQUESTED → RESULT_FAILURE`.
  3. Verify: table-driven unit test covering every mapping case passes.

- [ ] **Add `TaskPhase` + `TaskAborted` + `Task.AbortReason`** — role: sr-dev-be, deps: none, est: 20m
  1. In `internal/domain/status.go`: add `TaskPhase` (`TaskPhaseDispatch | TaskPhaseProduce | TaskPhaseReview | TaskPhaseRework | TaskPhaseRejected | TaskPhaseGateFailed | TaskPhaseAborted`).
  2. Add `TaskAborted` to `TaskStatus`; add `AbortReason` field.
  3. Verify: valid-transition matrix test asserting allowed phase/status transitions; `Aborted` is terminal-precursor.

- [ ] **Extend `SessionResult` with `Duration`, `HeartbeatStaleness`, `ArtifactPath`** — role: sr-dev-be, deps: none, est: 20m
  1. In `internal/domain/session.go`: add `Duration`, `HeartbeatStaleness`, `ArtifactPath` to `SessionResult`.
  2. `Session.End` registers final heartbeat + artifact path, populating all three fields.
  3. Verify: unit test that `Session.End` populates all three fields and `SessionResult` serializes correctly.

## Wave 2 (parallel — build on Wave 1)

- [ ] **Build `SignalRegistry` (signals.go)** — role: sr-dev-be, deps: FailureClass, SessionResult, est: 30m
  1. Create `internal/domain/signals.go` with immutable declarative table `Signal{Predicate, FailureClass, Description}` covering all 8 spec signals (exit 4/9/130/137/143→EXEC; stderr `connection refused`/`network timeout`→EXEC; exit −1/−2 + `blocked: time budget`→EXEC; exit 0 + placeholder→CONTRACT; exit 1 gate stderr→GATE; `CHANGES_REQUESTED:`+`[criterion:...]`→RESULT; heartbeat stale + process active→EXEC hung; git/binary absent / ErrShutdown→ENV).
  2. Implement `Resolve(result SessionResult) FailureClass` with priority stderr → exit → heartbeat → env-guard.
  3. Verify: `SignalRegistry.Resolve` returns correct class per signal scenario; priority-order determinism test (stderr wins over exit when both present).

- [ ] **Add `Phase` + `FailureClass` to `Task` + `Transition` method** — role: sr-dev-be, deps: FailureClass, TaskPhase, est: 30m
  1. In `internal/domain/task.go`: add `Phase TaskPhase` and `FailureClass FailureClass` fields.
  2. Replace `UpdateStatus` with `Transition(phase, status, verdict, commits, failureClass)`; update all callers.
  3. Verify: golden-file round-trip serialization with new fields; `Transition` persists all args and updates phase+verdict atomically.

- [ ] **Add `FailureSignals` + `Heartbeat` to `Adapter` interface** — role: sr-dev-be, deps: SignalRegistry, SessionResult, est: 20m
  1. In `internal/adapter/adapter.go`: add `FailureSignals() []Signal` to `Adapter`; add `HeartbeatPath() string` (or `Heartbeat() <-chan struct{}`) to `Session`.
  2. Verify: each concrete adapter (commandcode) returns a non-empty signal list; `Session.HeartbeatPath()` resolves within `<worktree>/.mill/`.

- [ ] **Extend ledger `Entry` with `FailureClass`, `Phase`, `Role`** — role: sr-dev-be, deps: FailureClass, TaskPhase, est: 20m
  1. In `internal/ledger/ledger.go`: add `FailureClass`, `Phase`, `Role` fields to `Entry`; `Append` remains append-only JSONL (no write-semantics change).
  2. Verify: `Entry` with new fields round-trips through JSONL; append ordering is monotonic.

## Wave 3 (parallel — adapter + orchestration leaf work)

- [ ] **Write `.mill/heartbeat` from `liveSession`** — role: sr-dev-be, deps: Adapter Heartbeat, SessionResult, est: 40m
  1. In `internal/adapter/commandcode.go`: `liveSession` writes `<worktree>/.mill/heartbeat` (timestamp + role, frontmatter `agent_id`) every N ticks while `cmd` runs.
  2. `waitWithBudget` distinguishes timeout-with-live-heartbeat (EXEC) from heartbeat-absent (hung), setting `HeartbeatStaleness`.
  3. Verify: integration test asserting heartbeat file exists and updates each tick during a mock command; staleness exceeds threshold on hang.

- [ ] **Replace `classifyResult` with `classifyFailure`** — role: sr-dev-be, deps: SignalRegistry, SessionResult, est: 40m
  1. In `internal/cli/delegate.go`: convert `classifyResult(exitCode, stderr) Classification` → `classifyFailure(result SessionResult) FailureClass` using `SignalRegistry.Resolve`.
  2. Apply CONTRACT artifact inspection (empty/placeholder/`TODO`/`TBD`) only when process exit is OK, honoring role-frontmatter `artifact_contract:` allowlist.
  3. Verify: `classifyFailure` returns CONTRACT only when exit==0 AND artifact matches placeholder allowlist; table-driven test per spec signal→class mapping.

- [ ] **Update `retryDispatch` model-chain condition** — role: sr-dev-be, deps: FailureClass, classifyFailure, est: 20m
  1. In `internal/cli/delegate.go`: change `retryDispatch` advance condition from `RATE_LIMITED/TRANSIENT` → `EXECUTION_FAILURE`.
  2. Enforce `Config.MaxRetries` (default 4) before escalation.
  3. Verify: returns `true` when `EXECUTION_FAILURE` with retries remaining; `false` (→escalate) when exhausted.

- [ ] **`validateDelegateBinaries` → ENVIRONMENT_FAILURE** — role: sr-dev-be, deps: FailureClass, TaskPhase, est: 20m
  1. In `internal/cli/delegate.go`: on missing provider binary or `git`, mark task state `aborted` + `ENVIRONMENT_FAILURE` instead of returning a direct error.
  2. Verify: missing binary sets `Task.Phase == Aborted` + `FailureClass == ENVIRONMENT_FAILURE` and returns nil to caller.

- [ ] **Add `escalateToParent` in `routing_56.go`** — role: sr-dev-be, deps: Task.Phase, role.ParseFrontmatter, est: 30m
  1. In `internal/cli/routing_56.go`: implement `escalateToParent(issue, role)` validating `delegates_to` via `role.ParseFrontmatter` (reuse `validateDelegation` logic), re-delegating to next role in chain.
  2. Enforce `Config.MaxDepth` (default = ORG-CHART leaf) with hard-stop at Staff + user notification.
  3. Verify: returns next role from valid `delegates_to`; rejects invalid/cyclic delegation; hard-stops at Staff.

- [ ] **Slots exhaustion → ENVIRONMENT_FAILURE (no block)** — role: sr-dev-be, deps: FailureClass, TaskPhase, est: 20m
  1. In `internal/cli/slots.go` / `internal/cli/slot_delegate.go`: `ErrShutdown`/slot exhaustion returns `ENVIRONMENT_FAILURE` with notification "slots agotados" instead of blocking indefinitely.
  2. Verify: exhaustion path sets `aborted` + `ENVIRONMENT_FAILURE`; timeout-bounded test proves no deadlock.

- [ ] **Append-only `lessons.md` per role** — role: sr-dev-be, deps: FailureClass, est: 20m
  1. Create logic writing `.mill/lessons/<role>.md`: append `FailureClass` + observable root-cause signal at session end; NEVER rewrite.
  2. Verify: two consecutive appends produce a concatenated (not overwritten) file; content includes failure class + signal snippet; file never truncated.

## Wave 4 (parallel — reactor integration + persistence)

- [ ] **Implement `FailureReactor` in `runDispatchLoop54`** — role: sr-dev-be, deps: classifyFailure, retryDispatch, escalateToParent, validateDelegateBinaries, slots, est: 60m
  1. In `internal/cli/review_loop.go`: replace `switch finalClassification` with `switch FailureClass` executing per-category reactions:
     - EXEC → retry model-chain (`retryDispatch`) → on `MaxRetries` exhaust, `escalateToParent`.
     - CONTRACT → reject artifact (no `output`/commits accepted) → re-delegate produce to a fresh role+session.
     - GATE → rework same role+worktree, feeding gate stderr back to produce.
     - RESULT → parent correction loop (review feedback `[criterion:...]` → produce); after `escalationThreshold` (3) reworks, escalate to Staff.
     - ENV → abort with NO retry, preserve worktree+logs, notify reason at run end.
  2. Record each transition in ledger with `failure_class`, `phase`, `role`.
  3. Verify: unit test per category asserting correct state transition + ledger append (e.g. ENV→`aborted` no retry; CONTRACT→`rejected`→new `produce` session).

- [ ] **Integrate heartbeat monitor goroutine** — role: sr-dev-be, deps: .mill/heartbeat, HeartbeatStaleness, est: 40m
  1. In `internal/cli/review_loop.go` (`runDispatchLoop54`): heartbeat monitor goroutine around `session.Wait()` using `context.Done` + `sync.Mutex` over `HeartbeatStaleness`; stops before `cmd.Wait()` returns.
  2. Verify: `go test -race` clean; goroutine exits before `Wait()` returns; `HeartbeatStaleness` consistent under concurrent access.

- [ ] **State persistence atomicity with new fields** — role: sr-dev-be, deps: Task.Phase/FailureClass, est: 30m
  1. In `internal/state/state.go`: confirm `Save()` atomic write (temp + fsync + backup rotation `.1`/`.2`) stays intact; ensure task marshaling includes new `Phase`/`FailureClass` fields without breaking round-trip.
  2. Verify: crash-injection test (kill mid-write) proves `.1`/`.2` backup valid + recoverable; restored state matches last-committed `Phase`+`FailureClass`.

- [ ] **Ensure gate hooks emit stderr naming the gate** — role: sr-dev-be, deps: none, est: 15m
  1. Verify `internal/cli/static/scaffold/.mill/checks/gate-{frd,spec,tasks}` emit stderr containing `gate-(frd|spec|tasks)` on rejection (GATE_FAILURE signal surface).
  2. Fix only if the signal surface is missing; no behavior change otherwise.
  3. Verify: each gate hook exits 1 with stderr containing `gate-frd:`/`gate-spec:`/`gate-tasks:` on failure (shell-level test under scaffold).

## Wave 5 (parallel — tests, co-located with their implementation)

- [ ] **`internal/domain/classification_test.go`** — role: sr-dev-be, deps: FailureClass, SignalRegistry, est: 30m
  1. Cases for `FailureClassOf` (all mappings) + `SignalRegistry` determinism across all 8 spec signals + priority ordering (stderr beats exit when both present).
  2. Verify: full branch coverage of `FailureClassOf` mappings and resolution-priority cases.

- [ ] **`internal/cli/delegate_test.go`** — role: sr-dev-be, deps: classifyFailure, est: 30m
  1. `classifyFailure` by category: exit 0+placeholder→CONTRACT; exit 137→EXEC; exit −1+stderr `blocked: time budget`→EXEC; exit 1+gate stderr→GATE; `CHANGES_REQUESTED` stderr→RESULT; git absent→ENV; heartbeat-stale + process-active→EXEC (hung).
  2. Verify: each category returns exact `FailureClass`; contract check uses `artifact_contract:` allowlist, not strict emptiness.

- [ ] **`internal/cli/review_loop_test.go`** — role: sr-dev-be, deps: FailureReactor, est: 45m
  1. Reactor scenarios: EXEC retry→exhaust→escalate; CONTRACT reject→re-delegate fresh; GATE rework same role+worktree; RESULT correction loop with `escalationThreshold=3`→Staff; ENV abort+preserve+notify (no `cleanupWorktree`).
  2. Verify: each branch asserts correct ledger `Entry`, `Task.Phase` transition, and side effect (ENV path leaves worktree on disk with `.mill/aborted` marker).

- [ ] **`internal/adapter/commandcode_test.go`** — role: sr-dev-be, deps: .mill/heartbeat, waitWithBudget, est: 30m
  1. `liveSession` heartbeat cadence during a mock command; `waitWithBudget` heartbeat-staleness distinction (hung vs timeout-with-live-heartbeat).
  2. Verify: heartbeat timestamp monotonic; staleness > threshold only after N ticks of inactivity; `SessionResult.HeartbeatStaleness` populated correctly.

## Cross-cutting

- [ ] **End-to-end smoke: failure per category** — role: sr-dev-be, deps: Waves 1–5, est: 30m
  1. Run a delegated task under each FailureClass (EXEC crash, CONTRACT empty artifact, GATE failing hook, RESULT `CHANGES_REQUESTED`, ENV missing binary) and observe the correct reaction + state transition + ledger entry + notification.
  2. Verify: `go build ./...` passes; full `go test ./... -race` clean; each category produces the documented state transition with no worktree corruption.
