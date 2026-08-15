# ADR 0009: The gauntlet runs at the dispatch boundary, not from git hooks

**Status:** Accepted
**Date:** 2026-08-15
**Decided by:** Architect
**Related:** #168, #170, #173. Supersedes the hooks step of ADR 0008.

## Context

Mill's gates ran from git hooks. The installer pointed `core.hooksPath` at
`.mill/checks`, so every commit in the main repository ran `pre-commit` (build,
lint, `role-enforce`) and `pre-push` (test, coverage). That resource is wrong
for the job, and the mechanism never reached the thing it existed to check.

**`core.hooksPath` is one global slot, contested by three parties.** It holds
exactly one directory. A delegation harness asking for it is invasive
regardless of how carefully it asks: whatever Mill takes, something else loses.
In our own repository the value displaced five hooks Mill has no equivalent
for — `commitlint`, `lint-staged`, a staging validator, three pre-push checks
and a `post-merge` that applies database migrations locally. Adopting hooks
does not replace those behaviours, it deletes them. ADR 0008 made the taking
non-destructive — refuse, never chain, record for undo — but the taking itself
remained the default, and with it the conflict. The failure from #168 made the
conflict concrete: a worker running `pnpm install` triggered husky's `prepare`,
which rewrote `core.hooksPath` in the shared `.git/config`, disabling Mill's
gates for the whole repository while the worker did exactly what its brief
asked. The installer no longer starts the fight, but the slot stays contested —
because it is one value that three parties write to.

**The hook never guarded what it claimed.** Enforcement through
`core.hooksPath` is structurally inert over worker output. Measured:

- A relative hooks path (what the installer sets — `.mill/checks`) is resolved
  by git **per worktree**, against each worktree's own working directory. A
  worker's worktree therefore runs the *worktree's copy* of the hooks — which
  the worker can edit, because they are just files — or runs *nothing*: git
  treats a missing hooks directory as no hooks, silently. Workers commit in
  Orca worktrees, which neither inherit the main repo's hook files nor share
  its role state; the commit-time enforcement point was never a worker's
  commit. #129 already noted commit-time enforcement is too late: "un agente
  escribe lo que quiera antes de llegar ahí".
- The hook was also the main repository's gate: it ran on the coordinator's and
  the human's commits. That is a different product — a pre-commit framework —
  and projects that want one already have husky, lefthook or pre-commit. Mill
  competing with those is what forces the exclusive choice.

The gauntlet's actual job is verifying **what a dispatch produced**, before the
coordinator lands it. `using-mill.md` step 8 already described this by hand:
"Run the gates — lint, type-check, build, test. Run them yourself." The
coordinator already holds the two facts that job needs: which worktree the
worker used (`--worktree`) and what changed in it (`--files-modified` in the
`worker_done` payload). The enforcement point is the dispatch boundary, not the
project's commit hook.

## Decision

**Mill's gates — the gauntlet and `role-enforce` — run against a worker's
output, invoked by the coordinator, in the worker's worktree. The install no
longer touches `core.hooksPath` at all.**

Concretely:

1. **`mill-verify` is the single verification entry point at the dispatch
   boundary.** The coordinator runs it in the worker's worktree after
   `worker_done`:

   ```
   .mill/checks/mill-verify --worktree <path> --role <role> [--files-modified <a,b,c>]
   ```

   It runs the configured gauntlet steps (build, lint, test from
   `.mill/gauntlet`, in that worktree), then enforces the role's `allowed_files`
   over the change set — `--files-modified` when given, otherwise the git diff
   since the worktree's base commit — and rejects any uncommitted work left
   behind. `--files-modified` is the authoritative change set: it is the exact
   record from the worker's report, not an inference from the worktree's reflog.

   This extends the existing `mill-verify` (which already checked a worker's
   files against its role) rather than adding a sibling command. It already
   lived at the dispatch boundary and the coordinator already called it; the
   missing pieces were the gauntlet run and the `--files-modified` change set.
   Two overlapping verification commands would recreate the confusion this
   ADR exists to remove.

2. **The install has no hooks step.** `mill-install` copies `.mill/` and
   `checks/` and links the skill, and nothing else. Taking `core.hooksPath`
   becomes an explicit opt-in for a project that has none and asks for it:
   `mill-install <scaffold> --with-hooks`, refused when a value is already set.
   This inverts the shape of ADR 0008, where opting *out* was what needed a
   decision. ADR 0008's non-destructive mechanics (validate first, manifest,
   uninstaller) are kept; its hooks step is superseded.

3. **The project's own hooks stay the project's** — untouched, unmentioned,
   and un-contested. Mill ships no `pre-commit`/`pre-push` hooks of its own; a
   project that wants Mill's gates under git can still opt in, and a project
   that has hooks keeps them running exactly as before.

## Consequences

**Gained.** Enforcement reaches what it exists to check: a worker's actual
output, in the worker's worktree, judged against the role that dispatch was
given. The contested global slot is returned to the project. The husky loop
(#168) becomes a non-event — there is no Mill hook configuration for a worker's
`pnpm install` to rewrite. The coordinator gets a single command that does what
step 8 previously assembled by hand, and the `worker_done` payload — which the
coordinator was already required to check — now feeds the gate directly. A
clean install validates everything before copying anything.

**Lost — and this is the cost, stated plainly.** A hook is **computational**:
git invokes it on every commit, whether or not anyone remembers, and a commit
that does not run it does not exist. A command the coordinator runs is
**inferential**: it depends on a human or an agent deciding to run it, in the
right worktree, against the right role. The gauntlet is no longer automatic
over main-repository commits — a human committing in the main repo (or a
coordinator landing work without verifying) is no longer blocked by Mill's
gates. `role-enforce` no longer stops a misbehaving worker at commit time,
because it never did: workers commit in worktrees the hook never reached, and
the measured resolution behaviour means the hook either ran the worker's own
editable copy or ran nothing. The enforcement that is lost is the enforcement
that did not happen. What replaces it is a gate that actually sees the worker's
change set, run by the one role whose job is to verify — the coordinator.

The remaining risk is procedural: verification is now an act the coordinator
performs, not a property of the repository. Mill's answer is the same one it
uses for the sequence itself — instructions plus a mechanical check. The
mechanical check is `mill-verify`; the instruction is `using-mill.md` step 8,
which now names the command. A coordinator that skips verification produces the
same result as a coordinator that skips the dispatch sequence: it is a process
failure, visible in the record, not silently enforced.

**Migration.** A project installed under ADR 0008 (manifest present, hooks
adopted) keeps its hooks; the manifest still records the previous
`core.hooksPath`, and `mill-uninstall` still restores it. `mill-install` with
`--with-hooks` is the only path that adopts `core.hooksPath` from here on. The
`pre-commit`/`pre-push` scripts remain shipped for opt-in hook users; `mill-verify`
does not depend on them.

## Alternatives considered

**Keep the hook, fix the mechanism (chaining, absolute paths, per-worktree
hooks).** The mechanism cannot be fixed: git offers one hooks directory per
repository, worktrees are not separately configurable for hooks, and chaining
is transient in exactly the projects that matter (husky rewrites
`core.hooksPath` on every install — ADR 0008 documents the loop). Every
mechanism still requires taking the project's one slot.

**Replace the hooks with a Mill-native pre-commit framework.** This is what
husky, lefthook and pre-commit already are. Mill has no reason to compete with
them, and doing so is what forces the exclusive choice this ADR removes.

**Keep the hook as an additional gate on main-repository commits, layered on
top of `mill-verify`.** This preserves the "automatic" property. It also keeps
Mill in the contest for `core.hooksPath`, re-exposes the husky loop, and
re-introduces the false confidence: the hook runs on the coordinator's commits
and the human's, not the worker's, so it would still not guard the thing the
gauntlet exists to check.

## Notes

`role-enforce`'s `--test` mode is the file-judging path `mill-verify` uses; its
pre-commit mode (reading `.mill/role` from the main repo) is the hook path that
never reached a worker and is no longer invoked by default. The phase gates
(`gate-*`) are unchanged and remain separate from the gauntlet. ADR 0008
remains in force for what it decided about skills and manifests; its hooks step
is superseded by this decision.
