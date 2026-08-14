# 0003 — Coverage gate must report project total

- **Status**: Accepted
- **Deciders**: Architect
- **Date**: 2026-08-13
- **Tags**: quality-gates, testing, coverage

## Context

`COMMON.md` requires coverage ≥ 90% (no exceptions for priority). The project
ships two `gate-coverage` scripts, plus an inline check in `pre-push`:

| Script | Computes | Deterministic? |
| --- | --- | --- |
| `.mill/checks/gate-coverage` (run by the phase gauntlet) | `head -1` of per-package `coverage:` lines | **No** — a coin flip |
| `checks/gate-coverage` (invoked by the mill skill) | unweighted per-package **average** | Yes, but a *sample*, not the total |
| `.mill/checks/pre-push` | per-package **minimum** (`sort -n | head -1`) | Yes |

The gate sampled one package's coverage instead of computing the project total.
Because `go test` schedules packages in parallel and emits their `coverage:`
lines in non-deterministic order, `head -1` picked whichever package finished
first. The reported number depended on scheduling, not on the code:

- **False pass** — a 77% package passed whenever a ≥90% package happened to
  report first. The total was never computed.
- **False block** — a healthy commit was rejected when a low-coverage package
  reported first.

`COVERAGE_THRESHOLD` reads as a project-level figure, so the gate was
comparing a per-package sample against a project-level threshold.

## Decision

`gate-coverage` (both `.mill/` and `checks/`) now reports the **project-wide
statement coverage** via:

```bash
go test -count=1 -coverprofile="$profile" ./...
go tool cover -func="$profile" | grep '^total:'
```

`go test -coverprofile` accumulates every package into a single profile, and
`go tool cover -func` reports its `total:` line — the aggregate statement
coverage weighted by statement count. This is an order-independent aggregate, so
the same tree always yields the same verdict, and no package can be masked by
scheduling order.

## Alternatives considered

- **Per-package enforcement** (every package ≥ threshold; the message names the
  offenders). `.mill/checks/pre-push` already implements this via the minimum,
  and it is deterministic. This is accepted as the **stricter** alternative and
  is left in place for the push gauntlet. It is *not* adopted for
  `gate-coverage` because the threshold is a project-level figure and the issue
  requests the project total.
- **Pin package order** (`go test -p 1`, or `sort` before `head -1`). Rejected:
  still reports a single package's coverage, never the project total.
- **Unweighted average of per-package lines**. Rejected: not a real aggregate;
  the coverprofile `total:` is the canonical, tool-supported figure.

## Consequences

- The same tree now produces the same verdict on every run (determinism).
- A package below the threshold cannot be masked by scheduling order; it always
  contributes to — and lowers — the total.
- `gate-coverage` (ISSUE interface) and `.mill/checks/gate-coverage`
  (pkg interface) now share the project-total computation despite differing
  interfaces. Unifying the two implementations is deferred.
- `.mill/checks/pre-push` keeps the per-package minimum: the push gauntlet
  stays stricter (every package must clear the bar), while the phase gate checks
  project health. The two figures are intentionally different and documented
  here, not a regression.

## Notes

`TestDelegateSlotExhaustionAbortsWithEnvFailure` and
`TestDelegateWithRealWorktree` create **real** git worktrees on the actual
repository and mutate `core.worktree` without cleanup — a test-isolation
defect. Running the full `go test ./...` on the working tree corrupts git.
Verify only the scoped test with `-run` filters.
