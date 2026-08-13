# Mill — Product Definition

> This document is the product. `.mill/docs/PRODUCT.md` predates it, is gitignored
> (#140), and describes an earlier framing. This file is the tracked one.

## What Mill is

Mill is an **org chart that executes**.

You describe intent once. An organization of specialized roles turns it into
reviewed work — decomposing it, delegating it down a chain of command, reviewing
what comes back, and telling you when your intent was not clear enough.

It is not a better coding agent. It is the structure a company already uses,
made executable, because that structure exists to solve exactly the problems a
single agent has: scoping, specialization, review, and knowing who to ask.

## Why it exists

Coding agents today are individuals. One agent takes a task and does the whole
thing alone — scoping, designing, implementing, testing — with one context
window and nobody checking it. It has no way to say "this is underspecified"
and no one to say it to. It guesses, and the guess stays invisible until it is
wrong.

Existing tools in this space (spec-kit, superpowers) start at the technical
spec. Mill starts one step earlier, at product intent, and carries it down.
The chain begins with *why*, not with *how*.

## The chain

```
CTO → PM     → Architect → Tech Lead → Sr Dev (BE/FE/Data)
CTO → Staff  → Architect → Tech Lead → Sr Dev
PM  → UX Designer → UI Designer
PM  → QA / Docs
```

- **PM** turns intent into an FRD: what and why, with measurable acceptance
  criteria. Never how.
- **Architect** decomposes one FRD into 1..N specs — boundaries, patterns,
  components affected.
- **Tech Lead** decomposes one spec into granular tasks, each small enough for
  a single developer.
- **Sr Dev, Designer, QA, Docs** are the leaves. They are the only roles that
  execute.

Only Staff and PM speak to the CTO. Everything below is reachable only by
delegation. This is what keeps the chain a chain, and it is enforced — a
delegation outside a role's declared targets is rejected.

Role assignment follows the conversation: engineering topics put the harness in
Staff, product topics in PM. The switch is announced and recorded in tool state,
not merely asserted in prose.

## Recursion

Every level performs the identical cycle: receive, review, do its own part,
delegate down, review what comes back. There is no special case for depth. A
level with nothing to delegate to is a leaf, and it executes.

## The escalation ladder

An executing role that finds the work underspecified, contradictory, or blocked
**must not guess**. It posts a comment on the issue stating precisely what is
missing, and stops.

Its observer picks the raised hand up — for a Sr Dev, that is the Tech Lead. If
the observer can resolve it, it resolves it and re-delegates. If it cannot, it
adds its own comment and escalates one step. This repeats until a role can
resolve it, with the CTO as the last resort.

Two properties matter:

- **Blocking is a first-class outcome, not a failure.** A raised hand is a
  successful result.
- **Escalation walks exactly one step at a time**, so each level gets the chance
  to resolve what it is qualified to resolve.

## Everything goes through issues

The GitHub issue is the single record. Briefs, FRDs, specs, raised hands,
resolutions and results are all issue content. There is no side channel. If it
is not on the issue, it did not happen.

## The economics — why Mill runs its own processes

This is the constraint that determines the architecture, and it is easy to lose.

**Expensive models think. Cheap models write. Expensive models review.**

The intelligence is spent on decomposition and on review; the writing is done by
the cheapest model that can do it. Quality comes from the review step, not from
the writer. This is deliberately where the money goes.

That requirement is the reason Mill cannot be a set of templates driving the
host agent, the way spec-kit and superpowers work. A harness runs **one** model.
Claude Code subagents can vary the model but only within one vendor. Routing a
role to a third-party model means spawning an external process with a chosen
model, which means an adapter, a dispatch loop, and process coordination.

So the runtime earns its place — but only that part of it. **Policy stays in
markdown**: the chain, the roles, who delegates to whom, the ladder. Policy is
what changes often; it must not require a recompile. The runtime is the engine,
not the rules.

The health metric for this codebase is the ratio between the two. Production Go
should shrink toward the irreducible core — adapter, worktree isolation,
dispatch, review loop — and everything else should live in markdown and shell.

## Learning from failure

Two names, deliberately distinct, because they say where the net failed:

- **Defect** — found by QA or in testing, before release.
- **Bug** — found in production, after release.

Every defect and bug is linked to the issue that produced it **and to the role
that produced it**, and feeds back into that role so the mistake is not
repeated. A correction from the CTO is learned from on the same footing.

A lesson that cannot be reached at the moment of work is not learning. Lessons
must arrive in the role's context when that role next executes. Writing them to
a file nothing reads is journaling, and is worse than nothing, because it looks
like the problem was handled.

## Observability

You must be able to see what each model and each specialist is doing, while it
is happening.

This is not a convenience. Every serious defect found so far was invisible from
the outside and had to be dug out of `/proc`, of ledger timestamps, or of a
worktree nobody was looking at: a role declaring `model: pro` and dispatching on
the cheapest model available; a reviewer approving an empty string; a delegation
producing four files and a verdict of `approved` for code that does not compile.

At minimum, a delegation must make visible, as it runs: which role, which model
per phase, which phase it is in, what it produced, and what the verdict rests on.
A green verdict whose basis cannot be inspected is worse than a red one, because
it stops anyone from looking.

## Out of scope

- **Multi-provider and multi-harness support.** Real and necessary; technical
  rather than product.
- **Replacing GitHub.** Issues are the record. Decoupling delegation from `gh`
  is about a fresh project being able to start, not about moving the record.
- **Autonomous merging to main.** Landing stays a human decision.
- **Any role other than Staff and PM talking to the CTO.** Deliberately
  excluded.

## The hard part

Keeping the harness aware, across a long conversation, that it holds a role and
that the tool is how it acts. That is a context problem rather than a feature,
and it does not fall out of the rest of the design. It deserves its own line of
work.

Today it works when the CTO reminds it. That is not a mechanism.
