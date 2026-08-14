# ADR 0005: Orca as the execution substrate

**Status:** Accepted
**Date:** 2026-08-14
**Decided by:** CTO, on evidence from a live test of Orca's orchestration CLI

## Context

Mill set out to be an org chart that executes: eleven roles, a chain of command,
capability enforcement, phase gates, and an FRD → spec → tasks artifact flow.

To run that, it also built an execution substrate — process spawning, worktree
isolation, concurrency slots, supervision, a ledger. Roughly 1,800–2,500 lines of
the ~7,900 lines of production Go, plus much of `internal/cli`'s dispatch
machinery.

Two days of running Mill against itself established that the substrate is where
essentially every defect lived, and that several of its most important pieces had
never worked at all:

- the gauntlet never ran in a delegated worktree (#145)
- the chain never executed past its first step (#153)
- the reviewer was shown the agent's narration instead of the diff (#143)
- no supervisor existed: a dead delegation stayed dead unnoticed (#157)
- nothing wrote back to the issue record (#156)
- the cost model — expensive models decompose and review, cheap models execute —
  never ran once, because every role resolved to the same cheap model (#116)

Meanwhile Orca, already installed on the development machine, was found to
provide the same substrate. It was tested directly, from the CLI, with no code
written.

## What was verified, not assumed

```
orca orchestration run-create      run created, coordinator bound to a terminal
orca orchestration task-create     task_df5fbfd6423c [ready]
orca orchestration worker-start    supervised worker; --agent, --worktree, --model, --timeout-ms
orca terminal send --text … --enter task injected and submitted
orca orchestration worker-read     live agent output
orca orchestration send            worker raised a hand: msg_0e761c9eceab [question]
orca orchestration check           coordinator received it
orca orchestration reply           coordinator answered; reply reached the worker
```

A `command-code` worker ran a real task inside a managed worktree under
supervision, and the full raise-a-hand cycle completed.

Mapped against Mill's open work:

| Mill issue | Orca |
|---|---|
| #139 raise a hand | `send` / `ask` / `check` / `reply` / `inbox` — verified end to end |
| #157 supervisor | `worker-start`, `worker-show/read/stop/abandon/release`; `worker-abandon` fences an uncertain worker *without claiming it stopped* |
| #154 fan-out | parallel worktrees are its core function |
| #146, #91, #101 | `worktree create/rm/ps`, managed lifecycle |
| #92, #119 | `task-update`, `terminal wait` (exit, tui-idle) |
| #156 messaging | a coordination protocol is injected into each worker's prompt |

## Decision

**Orca becomes Mill's execution substrate, and a dependency.**

Mill keeps the policy layer and stops owning the substrate:

**Mill owns**
- the eleven role definitions and their capabilities
- the phase sequence — FRD → spec → tasks → implementation → review
- `role-enforce`: what each role may write
- the phase gates, including acceptance criteria (#159)
- brief construction from role definition, issue, and upstream artifact
- **model tier selection per dispatch** — the one substrate capability Orca does
  not provide for non-Claude agents

**Orca owns**
- spawning and supervising workers
- worktree lifecycle
- the message bus, and the raise-a-hand cycle
- parallelism and its limits

### Topology: star, not chain

The coordinator is a hub. It dispatches to a role, receives the result, decides
the next step, and dispatches again — one-to-N, not one-to-one-to-one. The
organisational sequence is preserved; no role other than the coordinator needs to
know who comes next.

This dissolves rather than fixes #153 (roles never told to hand off), #109
(recursion), and much of #154 (fan-out). It also matches both reference systems:
Anthropic's lead agent spawning 3–5 subagents in parallel, and Orca's coordinator
with a mailbox against N workers.

**One coordinator, not two.** Staff and PM do not each coordinate. A single
coordinator holds the state and the mailbox; PM becomes a worker role like the
others. The separation that matters — that product does not decide architecture —
comes from *which role the coordinator dispatches to*, not from having two
coordinators.

## Alternatives considered

**Keep the in-house substrate.** Rejected. It is where every defect lived, its
supervisor does not exist, and rebuilding what Orca already does supervised is
work with no distinguishing value.

**Drive the host harness instead (spec-kit / superpowers model).** Rejected
earlier and still rejected: a harness runs one model, which forecloses the cost
model. Orca runs 30+ CLI agents, so it does not.

**Orca as one strategy among several (#161).** Deferred rather than rejected. The
seam is worth keeping in mind, but building an abstraction over one implementation
is speculative. Adopt Orca directly; extract a seam when a second backend is real.

## Consequences

**Gained.** Supervision, worktree lifecycle, the message bus, the raise-a-hand
cycle, parallelism — all working today, none of which Mill has to build. Roughly
1,800–2,500 lines of Go become deletable, and the six issues above stop being
work.

**Lost or at risk.**

1. **Per-dispatch model choice for non-Claude agents.** `--model` accepts only
   Claude, Codex and Cursor identifiers; a `command-code` worker reported
   `mimo-v2.5-pro`, chosen by the TUI. This is the basis of the cost model and
   remains Mill's problem to solve — via Command Code's own configuration, or by
   stating the tier in the brief as Orca's own implementer template does.

2. **Initial setup.** This works because the machine already has Claude, Command
   Code and omp configured. A fresh install has none of them, and Orca requires
   agents to be *configured* before `--agent` accepts them (`command-code` works;
   `commandcode` and `cmd` do not). Accepted knowingly as a future problem.

3. **Coupling to a third party's CLI surface.** Flag shapes already vary between
   sibling commands (`reply --id`, `worker-show --dispatch`, `inbox` rejecting
   `--run`). Mill should call Orca through one narrow internal module rather than
   scattering invocations, so a breaking change is a single repair.

4. **A running Orca.** `orca status` reports `appRunning: true`; the desktop app
   was up throughout. `orca serve` provides a headless runtime, which is what
   CI or cron would need. Untested.

**Two defects found while testing, worth reporting upstream.**

- `worker-start` does not submit the prompt. With `--agent claude` it injects a
  `TASK` block and sends no Enter; with `--agent command-code` it launches the TUI
  and injects nothing. Both unblock with `terminal send --text … --enter`.
- `orchestration ask` fails with `The Dispatch capability is missing` when invoked
  outside a dispatched worker. Expected, but the message does not say so.

## Notes

This ADR records a decision made on measurement rather than on argument. The
substrate was tested in about twenty minutes; the two days before it were spent
specifying things that already existed, working, on the same machine.
