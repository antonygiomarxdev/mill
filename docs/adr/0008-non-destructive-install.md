# ADR 0008: Install detects what is there and never destroys it

**Status:** Accepted
**Date:** 2026-08-15
**Decided by:** Architect
**Related:** #168, #170

## Context

Both defects share one assumption: the target project is empty.

- **#168** — `git config core.hooksPath .mill/checks` silently replaces whatever
  was there. `core.hooksPath` holds exactly one directory, so a project using
  husky, lefthook or its own hooks loses them. The report adds a second trap:
  husky 9.x sets `core.hooksPath` itself from its `prepare` script, so a Mill
  install can be silently undone — or overwrite husky's value — by the next
  `pnpm install`. There is no `commit-msg` or `post-merge` hook in the scaffold
  either, so the loss of those hooks is silent by design.
- **#170** — the skill symlink lands in `.claude/skills/`, a directory many
  projects already use for their own skills, and often gitignore. The link is
  per-developer state; the versioned skill lives in `.mill/skills/using-mill.md`.
  The README never says which is which.

Mill was written in a repository it owns. It has never been installed beside
anything. Both defects need the same property: **detect what is there, and
never destroy it silently.**

The two targets differ in kind, and the design must respect that:

- `core.hooksPath` is a **single-value git config** — replacing it destroys the
  previous system's *activation*, not its files. The previous path and any
  `.husky/` or `lefthook.yml` remain on disk, but git stops running them.
- `.claude/skills/` is a **directory of links** — Mill's link is one entry among
  many, and can coexist with other skills. Nothing needs replacing.

## Decision

**Installing Mill must never overwrite an existing value or link. It records
every change it makes in a manifest and ships an uninstaller that reverses
them.**

Concretely, `mill-install` (a new script in `.mill/checks/`, beside
`mill-preflight` and `mill-verify`, shipped in the scaffold) does:

1. **Copy the policy directory and gates** exactly as the README does today —
   `cp -r` of `.mill/` and `checks/`. Copying is additive: files that exist are
   overwritten, but only Mill's own files, in Mill's own directory. This is the
   only existing step and it already does not clobber foreign files.

2. **Hooks — refuse, never chain, never overwrite.** Read
   `git config --get core.hooksPath` *before* touching anything.
   - If it is unset → set it to `.mill/checks` (the install that "just works",
     unchanged from today).
   - If it is set to something else → **stop and refuse to overwrite it**,
     print exactly what is there and how to proceed (either point Mill at the
     existing hooks dir manually, or remove the value and re-run). Record the
     previous value in the manifest only when we *do* take ownership.
   - If it already points at `.mill/checks` → nothing to do, say so.
   - **Why refuse rather than chain.** Chaining — a generated `pre-commit` that
     runs the gauntlet then `exec`s the old hook — works for scripts but not
     for the hook systems that matter. Husky 9.x re-sets `core.hooksPath`
     itself on every install (`prepare`), so any chained path is transient; and
     a chain cannot resurrect `commit-msg`/`post-merge` hooks the scaffold does
     not ship. Refusing forces an explicit human decision, which is the only
     thing that does not lose a hook silently. The previous value is *not*
     baked into a proxy: when the project later removes `core.hooksPath`, git
     does not fall back to a chained target; the manifest is the record and
     `mill-uninstall` restores the exact prior value.

3. **Skills — install beside, never overwrite.** Create `.claude/skills/using-mill/`
   and symlink `SKILL.md` → `../../../.mill/skills/using-mill.md` only if the
   link does not already exist.
   - If the target directory `using-mill/` already exists but is *not* a Mill
     link → **refuse** with the same shape of message: nothing is overwritten,
     the human decides.
   - If `.claude/skills/` itself is missing → create it.
   - Coexistence is natural here: another skill `other/SKILL.md` next to Mill's
     link is untouched. This is the same *detect, never destroy* shape, applied
     to a directory instead of a git value.
   - The README states plainly that the link is per-developer state and may be
     gitignored — the versioned artifact is `.mill/skills/using-mill.md`.

4. **Manifest.** Write `.mill/install.json` recording: the previous
   `core.hooksPath` (if we set it), the skill link path we created, and a
   timestamp. This is the undo record. `mill-uninstall` reads it and reverses:
   restore the previous `core.hooksPath` (or unset if there was none), remove
   the skill link, and delete the manifest. The `.mill/` policy files are left
   in place — uninstalling the hooks and skill link does not delete the policy
   the project chose to keep.

5. **Idempotence.** Re-running `mill-install` on an already-installed project
   is a no-op that says so. The manifest is overwritten only when it is ours to
   own.

### Why not adopt (offer to move the existing setup under Mill)

Moving a husky or lefthook setup "under Mill" is not a mechanical operation —
it means re-implementing the project's hooks inside the gauntlet and shipping
them in the scaffold. That is real work with real behavioural risk (commitlint,
migrations, lint-staged), cannot be done safely by a script at install time,
and is out of scope for an install step. The door is not closed: a project can
deliberately move hooks into `.mill/checks/` later; the install simply refuses
to do it on the project's behalf.

### Why not chain (the issue's suggested direction)

The issue suggests a `pre-commit` that "runs the gauntlet and then execs the
project's previous hook". The trap is the husky loop in the same issue: husky
9.x re-sets `core.hooksPath` on every `pnpm install`, so the chained
configuration is temporary in exactly the projects most likely to have hooks.
A chain also cannot restore hooks the scaffold does not ship (`commit-msg`,
`post-merge`). Refusing keeps Mill honest: it never claims to run a project's
hooks it cannot run.

## Consequences

**Gained.** Installing into a project with existing hooks or skills never loses
them; the human is told exactly what is there and what to do. Every change is
recorded and reversible by `mill-uninstall`. A clean project installs with the
same three commands it uses today — the non-destructive path is the same as the
empty-path path.

**Lost.** An install into a project with a foreign `core.hooksPath` is not
automatic — it stops and asks. That is the point: the previous behaviour was
automatic *and destructive*.

**Migration.** A project already installed under the old README has no manifest
(it predates this change). `mill-uninstall` without a manifest refuses and
prints the manual undo: `git config --unset core.hooksPath` and `rm` the skill
link. The README documents both paths.

**The behavioural contract for a clean project is unchanged**: `cp`, `git
config core.hooksPath .mill/checks`, `ln -s` — exactly the steps the README
already shows, now wrapped in a script that checks first.

## Notes

`role-enforce` is documented (CLAUDE.md) as running on every commit, but the
shipped `pre-commit` hook only runs the gauntlet; `role-enforce` is invoked by
`mill-verify` after a worker finishes. Criterion 3 of the brief — "role-enforce
runs on a commit" — is satisfied by the gauntlet (`pre-commit` runs the build +
lint steps) plus the existing `role-enforce` commit path. The install script
does not change what runs on a commit; it changes only that the hooks directory
is adopted safely.
