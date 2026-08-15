# Mill — Product Definition

> This document is the product definition. It is the single source of truth for
> what Mill is and how it works.

## What Mill is

Mill is an **org chart that executes** — a skill plus a policy directory that
turns a CTO session into a structured organisation of specialised roles.

You describe intent once. An organization of specialized roles turns it into
reviewed work — decomposing it, delegating it down a chain of command, reviewing
what comes back, and telling you when your intent was not clear enough.

It is not a better coding agent. It is the structure a company already uses,
made executable, because that structure exists to solve exactly the problems a
single agent has: scoping, specialization, review, and knowing who to ask.

Mill carries the roles, the phase sequence, brief construction, and the dispatch
procedure. Orca provides the execution substrate — worker spawning, supervision,
worktree isolation, and the message bus. See [ADR 0005](../docs/adr/0005-orca-as-execution-substrate.md) and [ADR 0006](../docs/adr/0006-mill-is-a-skill-not-a-binary.md).

## Why it exists

Coding agents today are individuals. One agent takes a task and does the whole
thing alone — scoping, designing, implementing, testing — with one context
window and nobody checking it. It has no way to say "this is underspecified"
and no one to say it to. It guesses, and the guess stays invisible until it is
wrong.

Existing tools in this space (spec-kit, superpowers) start at the technical
spec. Mill starts one step earlier, at product intent, and carries it down.
The chain begins with *why*, not with *how*.

## The sequence, and who walks it

The organisational sequence is unchanged:

```
intent → FRD → spec(s) → tasks → implementation → review
          PM    Architect  Tech Lead   Sr Dev      Reviewer
```

- **PM** turns intent into an FRD: what and why, with measurable acceptance
  criteria. Never how.
- **Architect** decomposes one FRD into 1..N specs — boundaries, patterns,
  components affected.
- **Tech Lead** decomposes one spec into granular tasks, each small enough for
  a single developer.
- **Sr Dev, Designer, QA, Docs** execute.

**What walks the sequence is the coordinator, not the roles.** The topology is a
star: the coordinator dispatches a role, receives its result, decides the next
step, and dispatches again — one-to-N, never one-to-one-to-one. No role other
than the coordinator needs to know who comes after it.

There is **one** coordinator. The separation that matters — that product does not
decide architecture — comes from *which role the coordinator dispatches to*, not
from having several coordinators. PM is a worker role like the others.

This is why a deep chain was abandoned: it required every role to know the whole
org chart and to delegate onward, which never happened in practice
([ADR 0006](adr/0006-mill-is-a-skill-not-a-binary.md)). Both reference systems
use the same star — Anthropic's lead agent spawning subagents in parallel, and
Orca's coordinator with a mailbox against N workers.

Fan-out is the normal case, not a special one: dispatching four workers at one
step is the same operation as dispatching one, and the join is the coordinator
waiting on its mailbox.

## Raising a hand

A worker that finds the work underspecified, contradictory, or blocked **must
not guess**. It says what is missing and stops:

```
orca orchestration send --to run:<id> --subject "<short>" \
     --body "<precisely what is missing>" --type question
```

The coordinator receives it, and either resolves it and re-dispatches, or
escalates to the CTO. Two levels, not a walk up an org chart — the coordinator
is the only observer, and it holds the context needed to answer.

**Blocking is a first-class outcome, not a failure.** A raised hand is a
successful result: it is the mechanism that stops an underspecified task from
becoming plausible-looking wrong work.

## What Mill verifies, and what it leaves alone

**The gauntlet checks the code Mill produced. It does not touch the project's
own tooling.**

Mill's authority ends at the dispatch boundary. It judges what a worker wrote,
in that worker's worktree, against the role that wrote it and the acceptance
criteria it was given. That is the whole of its remit.

It does not own the project's commit path. `core.hooksPath` is a single slot,
and a project that has husky, lefthook or its own hooks has them for reasons
Mill knows nothing about — commitlint, migrations, staging checks with no
equivalent here. Taking that slot does not add Mill's checks to theirs; it
deletes theirs.

This was learned by doing it wrong. Mill took `core.hooksPath` at install, and
then a worker running `npm install` triggered husky's `prepare`, which took it
back — disabling Mill's own gates repository-wide while doing exactly what its
brief asked. Three parties writing one global slot.

The deeper error was that the hook never guarded what it claimed to. It applied
to the coordinator's commits and the human's; workers commit in worktrees that
do not inherit it. Mill was paying for a project's configuration to enforce
something that never reached the thing it existed to check.

**A tool that damages the project it was installed into has failed, whatever
else it does.**

## Everything goes through issues

The GitHub issue is the single record. Briefs, FRDs, specs, raised hands,
resolutions and results are all issue content. There is no side channel. If it
is not on the issue, it did not happen.

## The economics

**Expensive models think. Cheap models write. Expensive models review.**

The intelligence is spent on decomposition and on review; the writing is done by
the cheapest model that can do it. Quality comes from the review step, not from
the writer. This is deliberately where the money goes.

Orca's orchestration substrate makes this possible: it spawns workers with
different models in parallel, under supervision, in isolated worktrees. Mill's
contribution is **model tier selection per dispatch** — the one substrate
capability Orca does not provide for non-Claude agents. Role frontmatter declares
the tier; the coordinator passes the appropriate `--model` flag when dispatching
the worker.

**Policy stays in markdown**: the chain, the roles, who delegates to whom, the
ladder, the gates, the acceptance criteria. Policy is what changes often; it must
not require a recompile. The skill is the rules, not the engine.

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

Keeping the coordinator agent aware, across a long conversation, that it holds a
role and that Orca's dispatch tools are how it acts. That is a context problem
rather than a feature, and it does not fall out of the rest of the design. It
deserves its own line of work.

Today it works when the CTO reminds it. That is not a mechanism.
