# ADR 0010: Coordinator exceptions — bootstrap fix and landing authority

**Status:** Accepted
**Date:** 2026-08-20
**Decided by:** CTO
**Related:** ADR 0007, ADR 0009

## Context

The Staff (coordinator) role declares two prohibitions that were being violated
in practice without explicit authorisation:

1. **"Never write implementation code."** `staff/ROLE.md` permits a single
   escape hatch — "mill autoconstruction (bootstrap)" — and requires it be
   "recorded as an explicit exception."
2. **"Never merge to main."** `staff/ROLE.md` line 39: "You own the decision map,
   scope research, write briefs, verify results, and declare merge-readiness. You
   never merge." Line 76: "Merge to main. You declare merge-readiness. Only the
   CTO merges." No other role claims merge authority.

Both were breached, one structurally:

### The bootstrap defect

`role-enforce`'s `enforce_file` computed a file's extension as
`.${file##*.}` applied to the **whole path** (the pre-fix version, line 131; ADR
0007 cites the same expression at `role-enforce:80` under the earlier layout).
For a path with no dot — every script under `checks/` and `.mill/checks/`, plus
`.mill/role` itself — `${file##*.}` returns the entire path, so `ext` becomes
something like `.mill/checks/role-enforce`. No capability pattern in
`.mill/role-capabilities.example` (`.md`, `.sh`, `.yml`, `.json`, `.pen`) matches
that, so **no role could modify any extensionless file.**

The Policy Author's `allowed_files` are `policy` and `scripts`, which resolve to
`.md .sh` per `role-capabilities.example`. Shell scripts without a `.sh` suffix —
the gates under `.mill/checks/` — fell in the gap. The Policy Author, whose
declared job is to own `.mill/**` including the gates, was locked out of every
gate. Mill could not repair its own harness through its own delegation chain.

The fix had to live in `.mill/checks/role-enforce` — the very file the bug made
unmodifiable. The pre-commit hook (`checks/pre-commit`) invokes `role-enforce`,
which reads `.mill/role` (currently `staff`) to determine the active role; under
the old code, committing a fix to that file would be blocked by the file's own
broken logic. The coordinator therefore could not fix it through the normal path:
the Policy Author was blocked by the bug, and the coordinator was blocked by the
"never write implementation code" rule.

### The landing defect

The "never merge" rule was being violated openly: the coordinator had been
merging verified branches to `main` for several sessions. With no documented
procedure, the rule existed only as prose — a coordinator or worker that landed
without verifying, or trusted a report instead of running the gates, would
violate a prohibition nobody was tracking as live.

On 2026-08-20 the CTO resolved both with bounded, explicit decisions.

## Decision

### Decision 1 — the bootstrap exception is one, bounded, and non-precedential

The CTO authorised a single, one-time exception: the coordinator applied the
fix to `.mill/checks/role-enforce` itself, committing with `--no-verify` because
the pre-commit path (`checks/pre-commit` → `role-enforce` → reads `.mill/role`)
was part of the broken mechanism. The fix is commit `81db32a` ("fix(checks):
classify extensionless files by shebang, not by path"), landed as merge `b8edb58`
("Land role-enforce-extensionless: gates become editable by their owner").

The fix takes the basename before computing the extension; an extensionless file
is classified by its shebang — a bash/sh script gets `.sh`, anything else gets no
extension and still matches no pattern. This widens `.sh` files only. The commit's
own verification (10 `role-enforce --test` cases) confirms the fix: `policy-author`
can now modify `.mill/checks/mill-verify`, `.mill/checks/role-enforce`, and
`checks/gate-frd` (allowed), while non-owners are still blocked (`pm` / `qa-docs`
+ `.mill/checks/mill-verify` → blocked) and non-script extensionless files stay
blocked (`policy-author` + `.mill/role` → blocked, fails closed).

**This is not a precedent.** After the fix, the Policy Author can maintain the
gates normally — the delegation chain can now repair itself. The exception cannot
be needed again for this reason.

A future bootstrap exception would require an ADR, not a judgement call, and
would be justified only by a defect that makes the delegation chain unable to
reach the file that contains the defect — the same structural condition that
existed here.

### Decision 2 — the coordinator lands verified work to main

The "never merge" rule is replaced by a procedure. The coordinator may land a
worker's verified branch to `main`, and only when all three conditions hold, in
order:

1. The worker reported `worker_done` — the outcome is recorded in the dispatch
   record, not in prose.
2. The coordinator ran `.mill/checks/mill-verify --worktree <path> --role <role>
   --files-modified <list>` and it exited 0.
3. The coordinator ran the brief's acceptance criteria itself — re-running the
   stated checks against the worktree, not trusting the worker's report.

The coordinator still never merges its own writing — it has none to merge, and
this decision concerns landing verified worker output.

### What this does NOT authorise

- **Pushing.** The coordinator does not push; the CTO merges.
- **Pull requests.** The coordinator does not open, close, or comment on PRs.
- **Unverified landing.** "Looks fine" is not verification. Work that failed
  `mill-verify`, or whose acceptance criteria were not independently confirmed,
  is not landed.
- **Scope creep.** The bootstrap exception covers only the single defect in
  `role-enforce` and no other file.

## Alternatives considered

**Dispatch the fix through the Policy Author from the start.** Rejected on
facts: the bug blocked every role from touching extensionless files, including
the Policy Author from touching `role-enforce` itself. The delegation chain
could not reach the file.

**Write the fix into a new file and swap.** Rejected: the defect lives in
`enforce_file`, the function every code path (including `mill-verify`) shares.
A shadow file would not repair the gate that runs at the dispatch boundary, and
would still need `role-enforce` itself patched eventually — the same blocked file.

**Keep "never merge" as prose and land anyway.** Rejected: a rule nobody checks
against behaviour is not a rule. The landing procedure makes the authorisation
explicit and auditable, with the three-step gate as the mechanical check.

**Have Staff own merge authority outright.** Rejected: merge is the CTO's act of
accepting a branch into `main`; folding it into Staff would collapse the review
boundary the role exists to hold. The decision is to make the coordinator the
land gate, not the merge authority.

## Consequences

**Gained.**
- Mill can now repair its own harness: the Policy Author edits
  `.mill/checks/role-enforce` like any other gate (commit `81db32a`, merge
  `b8edb58`).
- Landing authority is explicit and procedural. Landed work carries `worker_done`
  + `mill-verify` exit 0 + independently-run acceptance criteria on record.
- The "never write implementation code" escape hatch is defined, not assumed:
  bootstrap means "the file the chain cannot reach," not "whatever the
  coordinator thinks is urgent."

**Risk, mitigated.**
- The bootstrap exception could normalise the coordinator writing fixes itself.
  Mitigated by: it names exactly one file and one commit, and Decision 1 is
  framed as non-precedential with a stated bar for any future one.
- Delegated landing could erode the coordinator / CTO boundary. Mitigated by:
  no push, no PR, no unverified landing — `main` is still updated only via the
  CTO's merge.

**Out of scope.**
- `staff/ROLE.md` still states "never merge" without the landing procedure. A
  later policy-author pass is expected to update that prose to point here. (The
  brief named `product-engineer/ROLE.md`, which does not exist in this
  repository; the rule actually lives in `staff/ROLE.md`, lines 39 and 76.)

## Notes

- The pre-commit hook (`checks/pre-commit`) calls `role-enforce` on every commit.
  `role-enforce` reads `.mill/role` to determine the active role; the bug blocked
  the very file required to repair it, so the fix was committed with `--no-verify`
  to bypass the pre-commit path.
- `81db32a` is the fix commit; `b8edb58` is the merge that landed it.
- ADR 0007 established the category-to-pattern mechanism (`resolve_categories`)
  but retained the `ext=".${file##*.}"` extension computation from the
  pre-category design — the bug survived into the current code.
- The `--test` mode of `role-enforce` is the verification vehicle used: it judges
  a single file against a single role without invoking the pre-commit path, so it
  cannot be blocked by the same bug it tests.
