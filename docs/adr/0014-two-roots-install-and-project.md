# ADR 0014: Two roots — the install and the project

**Date:** 2026-09-02
**Status:** Accepted
**Deciders:** CTO
**Related:** #194, #195, #192, #185. Extends ADR 0012; carries forward the
safety rule ADR 0013 and #195 established.

## Context

`INSTALL.md` has said from its first line that Mill installs in two halves:
the extension ships the mechanism, and the project supplies `.mill/gauntlet`
and `.mill/role-capabilities`, which "cannot be packaged — both differ per
project". The code implements one half. Every Mill file resolves from the
directory the scripts live in:

- `role-enforce:30` — `mill_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"`
- `common.sh:36` — `GAUNTLET_CONFIG=".../../.mill/gauntlet"`, the same root
- `mill-preflight:82` — `mill_root="$here/../.."`, from the script's own location

On this machine the install directory and the project are the same directory,
so the contradiction is invisible. Under a real extension install they are
not: a project's build commands and file patterns would be read from the
plugin's copy, not the project's. `mill-preflight` meanwhile checks
`.mill/roles` against the caller's working directory (line 58), so the
documented install cannot satisfy both halves at once. That is #194.

## Decision

**Mill resolves two roots, named and separated.**

- **Install root** — where the scripts live, derived from the script's own
  location. Holds `.mill/checks/`, `.mill/roles/`, and the `delegate` skill.
  Identical in every project.
- **Project root** — the repository Mill is being used on. Holds
  `.mill/gauntlet` and `.mill/role-capabilities`, and nothing else Mill reads.

**The safety constraint is not optional.** ADR 0013's successor rule from #195
still binds: no file that decides what a role may write may be read from the
worktree being judged. `role-capabilities` is such a file and now comes from
the project, so the constraint is restated for two roots:

> The project root is resolved by the caller before it enters the worktree,
> and passed explicitly. It is never inferred from the working directory by
> the script that reads the file, because inside `mill-verify` that directory
> is the tree under judgement. The project root and the judged worktree must
> be distinct trees, and the tooling refuses to run when they are not.

This is the invariant future changes are checked against — the way
`role-enforce` carries #195's invariant at the top of the file.

### What this ADR does not decide

The implementation belongs to the policy-author in a separate change. This ADR
does not name functions, flags, or line edits; it specifies the rule the
implementation must satisfy: the project root is passed in by the caller, is
never derived from the working directory by the script that reads the file, and
a run whose project root equals the judged worktree is refused.

## Alternatives considered

- **A vendored copy — all of `.mill/` committed into the project.** One root,
  no ambiguity between install and project. Rejected because roughly ten
  thousand lines of Mill land in someone else's repository, and updates arrive
  by manual copy — drift is guaranteed.
- **An extension shipping overridable defaults for both files.** No
  configuration needed to start. Rejected because a wrong default fails
  silently instead of asking, and the two files exist precisely so that a
  human sees and approves them per project.

## Consequences

### Positive

- **The package is now definable.** The extension ships `.mill/checks/`,
  `.mill/roles/`, the `delegate` skill, and the harness manifests — and nothing
  else Mill reads. `.mill/phases/` and `.mill/docs/` are Mill's own history
  and do not travel (#192).
- **`INSTALL.md`'s two-halves framing becomes true rather than aspirational.**
  Each half now has a defined root; the extension half and the project half no
  longer resolve from the same directory.
- **Multi-harness packaging matters again (#185).** The extension is now the
  only thing that carries policy; a harness manifest that loads the wrong
  checks, roles, or skill silently changes what every role may do.
- **A project that never creates the two files gets fail-closed behaviour, not
  a silent default.** A missing `.mill/gauntlet` reports "no gauntlet" and
  skips; a missing `.mill/role-capabilities` blocks in `role-enforce`. Nothing
  silently assumes a default.

### Negative

- The caller must now pass the project root on every invocation — a new
  parameter and a new refusal path.
- The one-directory mental model breaks. Mill's own tests run where install and
  project coincide, so the distinct-trees path is exercised only under a real
  install; the refusal must be tested explicitly.

### Mitigations

- The refusal is loud and named, never a silent fallback.
- The invariant sits at the top of the script that reads the capability file,
  as #195's invariant does today, so an edit that infers the project root from
  the working directory is visibly a violation.
- Verification runs with distinct install and project roots.

## References

- #194 — the two-root decision recorded here.
- #195 — the successor safety rule restated above as the invariant.
- #192 — `.mill/phases/` and `.mill/docs/` are Mill's own history and do not
  travel.
- #185 — multi-harness packaging, load-bearing now that the extension is the
  only carrier of policy.
- ADR 0012 — the extension-plus-prompt distribution this ADR extends.
- ADR 0013 — invoked-not-ambient; the source of #195's rule.
