# ADR 0011: Mill installs as a versioned copy, not a second source of truth

**Status:** Accepted
**Date:** 2026-08-31
**Decided by:** Architect
**Related:** #162, #163, #170. Supersedes ADR 0008 (non-destructive install).

## Context

Mill used to ship a `scaffold/` directory — 61 files that `mill-install` copied
into a new project. It is being deleted because keeping it in sync with the real
`.mill/` was a tax nobody paid: three divergent copies of the same gate scripts,
and an installer that shipped a version of Mill the repository had already
retired.

Mill is small now. It is not a framework; it is a policy directory that a
coordinator reads, plus bash gates that enforce it:

```
AGENTS.md                          entry file, 28 lines
CLAUDE.md                          symlink -> AGENTS.md
.claude/skills/delegate/SKILL.md   the coordinator's procedure, 98 lines
.mill/roles/*/ROLE.md              12 role briefings + COMMON.md
.mill/checks/                      7 scripts: common.sh, mill-preflight,
                                   mill-role-guard, mill-verify, pre-commit,
                                   pre-push, role-enforce
.mill/role-capabilities            extension -> category map
.mill/gauntlet                     per-project build/lint/test commands
.claude/settings.json              the two hooks that keep the role in context
LESSONS.md, MEMORY.md              history and current state
```

The CTO wants it installable again, across several harnesses (Claude Code,
Codex, Copilot, opencode/omp, Pi, others), with onboarding that is genuinely
easy.

Three open issues describe the current failure, in ascending order of
difficulty:

- **#170** — the skill link lands in `.claude/`, which many projects gitignore,
  so a fresh clone drops it.
- **#163** — the first run does not survive without its author; there is no
  checkable "installed" state.
- **#162** — nobody outside this machine has installed Mill; there is no path a
  stranger can run.

Three prior ADRs bear directly on this one. ADR 0006 made Mill a skill plus a
policy directory, not a binary. ADR 0007 made `role-enforce` match *categories*
that the project maps to file patterns in `.mill/role-capabilities` — a
**per-project** file. ADR 0009 moved verification to the dispatch boundary and
made `.mill/gauntlet` a **per-project** file. Those two per-project files are
the reason the install can never be a pure "point at the source" operation: two
of the files an installed Mill needs are, by earlier decision, the project's
own.

The question this ADR must answer is the one every previous attempt failed on:
**how does an installed Mill receive fixes without becoming a second copy that
drifts?** Everything else — harnesses, onboarding — is downstream of it.

## Decision

**Mill installs as a versioned copy into the target repository: the installer
materialises the files from Mill's own repository at a git tag, records the
tag, and `mill-install --upgrade` re-syncs the copy non-destructively. There is
exactly one source of truth — Mill's repository at a tag — and the installed
copy is a faithful, re-syncable materialisation of it, never a hand-edited
second copy.**

The drift problem is not solved by eliminating the copy. Mill is files-in-repo:
a harness reads `AGENTS.md` and `CLAUDE.md` at the project root, and the
per-project files (`gauntlet`, `role-capabilities`) must live in the project.
Any mechanism that keeps Mill's files *out* of the target repo (submodule,
symlink, plugin, package) either cannot reach the root entry files, or shares
the per-project state it must not share, or both — the alternatives section
shows this. The copy is inevitable. What drifts is a copy with no version and
no re-sync path; the fix is to give the copy a version and a re-sync path.

Concretely:

1. **One source of truth.** Mill's repository is the source. It ships no
   `scaffold/` and no frozen template. The installer fetches a git tag (ADR
   0004 already established SemVer tags; `install.sh` already resolves the
   latest tag) and writes the files into the target. A copy is *generated* from
   the tag; it is never checked in anywhere and never edited by hand. This is
   the difference from the scaffold, which was a second checked-in copy that
   diverged from `.mill/` because nobody paid the sync tax.

2. **The version pin makes drift detectable.** The installer writes the
   installed tag (`.mill/version`, or a field in the install manifest).
   `mill-preflight` compares it against the latest tag and reports "installed
   vX.Y.Z, latest vX.Y.Z+N" — staleness is a machine-readable fact, not a
   suspicion. `role-enforce` is the canonical case: a fix to it reaches a
   project because the project's install records it is behind, and the upgrade
   path (next point) closes the gap.

3. **`mill-install --upgrade` is the fix-delivery path — one command,
   non-destructive.** It re-fetches the latest tag and replaces **Mill-owned**
   files: the entry files, the roles, the checks, the skill, the settings
   hooks. It leaves **project-owned** files alone: `.mill/gauntlet` and
   `.mill/role-capabilities` (per ADR 0007 and 0009), plus everything else in
   the repository. The ownership boundary is a manifest — the installer knows
   which files it wrote, from which tag — so the upgrade touches exactly Mill's
   files and nothing the project authored.

4. **Local edits to Mill-owned files are surfaced, not silently clobbered or
   silently preserved.** If a project edited a Mill-owned file (the thing that
   used to create the three divergent copies), the upgrade detects the diff
   between the on-disk file and what the old tag shipped. It refuses to
   overwrite blindly: it renames the local version to `<file>.local` (or
   refuses and prints the diff, and the operator chooses), then lands the new
   version. Either way nothing is lost — the project is a git repository, so any
   overwritten or renamed content is recoverable from history. "The user
   re-runs the installer" is the honest answer to "how does a fix to
   `role-enforce` reach a project installed six months ago", with the caveat
   that the re-run is a *defined, non-destructive* operation: it replaces
   Mill-owned files, preserves project-owned files, and surfaces the one case
   that used to create drift — a project's own edits to Mill's files — instead
   of letting it accumulate silently.

5. **No submodule, no symlink, no plugin channel, no registry.** The versioning
   spine (tags, SemVer) is kept from ADR 0004; the distribution is a
   tag-fetched source archive materialised by the installer. The reasons the
   non-copying alternatives fail are in *Alternatives considered*.

### One source document per content-kind, many harnesses

The repository already generalises "one file, many harnesses" with `AGENTS.md`
as the real file and `CLAUDE.md` as a symlink to it. That extends only so far,
and the boundary is exact:

- **Symlinks are a local convenience, not a distribution guarantee.** They
  require `core.symlinks` and Developer Mode on Windows; a clone made without
  them loses the link. The installer therefore writes real files and uses
  symlinks only where the platform supports them — the version pin and upgrade
  path, not the symlink, are what keep the copies in sync.

- **A symlink cannot serve structured or distinct documents.**
  `.claude/settings.json` is JSON (the two hooks), not Markdown — no symlink to
  a `.md` can produce it. `.omp/RULES.md` is a *different* document (the
  non-negotiable rules), not a copy of the entry. `.claude/skills/delegate/SKILL.md`
  is the 98-line procedure, and it must sit inside a directory the harness
  controls, so it can be neither a symlink to a single root file nor the entry
  itself. So the pattern that generalises is not "one file, symlinked
  everywhere" but **one source document per content-kind, rendered by the
  installer into each harness's expected filename.**

The installer renders, from the single source, one entry file per harness:

| Harness | Convention | Evidence | Status |
|---|---|---|---|
| Claude Code | `CLAUDE.md` (root), `.claude/skills/<name>/SKILL.md`, `.claude/settings.json` hooks | this repo: root `CLAUDE.md` is a symlink to `AGENTS.md` (mode 120000); `.claude/skills/delegate/SKILL.md` and `.claude/settings.json` are present and read | verified |
| Codex / generic (AGENTS.md standard) | `AGENTS.md` at root | this repo: root `AGENTS.md` (28 lines) | verified |
| Copilot | `.github/copilot-instructions.md` | the deleted scaffold carried `scaffold/.github/copilot-instructions.md` (read before deletion) | verified |
| opencode / omp | `.omp/AGENTS.md`, `.omp/RULES.md` | this repo: both present and read | verified |
| Pi | unknown | none | unverified |

**Pi is unverified.** No file in this repository and no documentation I can read
states Pi's convention for agent context files. A wrong filename in this table
becomes a wrong path in the installer, so the installer writes nothing for Pi
until one of these confirms it: Pi's published documentation naming the file(s)
it auto-reads; a real Pi-managed repository whose entry file can be inspected;
or confirmation that Pi honours `AGENTS.md`. The same rule binds any further
harness (e.g. Cursor): it is **unverified** until its convention is confirmed
from a file or its own documentation.

### What "easy onboarding" means, measurably

Done is checkable, and the three issues map onto it:

1. **Install — one command.** `mill-install` fetches the tag, writes the entry
   files and `.mill/`, and generates `.mill/gauntlet` and
   `.mill/role-capabilities` from templates. The only thing the operator must
   know beforehand is the project's build/lint/test commands, to fill
   `.mill/gauntlet` — knowledge the project's own author already has.

2. **Verify — one command.** `mill-preflight` exits 0 only when the install is
   complete and self-checking: entry files present, `.mill/` populated,
   `gauntlet` and `role-capabilities` filled, and — for a first dispatch — Orca
   reachable and one agent configured. On any failure it prints the exact next
   command. This is the checkable "installed" state that #163 demands, and it
   is what a stranger needs to *know* it worked without the author watching.

3. **First dispatch — still needs a human on a fresh machine, and the ADR says
   so.** A verified dispatch requires Orca (`orca status` reachable) and one
   agent configured for Orca. That configuration is machine-global — MEMORY.md
   records `~/.commandcode/config.json` as the agent table, outside any
   repository — and ADR 0005 accepted it as "a future problem" rather than
   solved it. The installer does not automate it; it *detects* it in
   `mill-preflight` and prints the commands. #162 ("nobody outside this machine
   has installed Mill") is solved as far as Mill's own distribution goes — a
   stranger can run one command and get a self-checking install; the residual
   step, configuring Orca's agents, is not Mill's files to write.

**#170** is covered by the ownership boundary: the coordinator's procedure is
versioned in `.mill/` (Mill-owned, upgraded with everything else), and the
harness-scoped skill link under `.claude/skills/` is per-developer convenience
that may be gitignored — nothing the project relies on for first run depends on
a gitignored path. This is ADR 0008's skill decision, retained.

## Alternatives considered

**Copy-in, as the scaffold did it.** Rejected as it was: a second checked-in
copy with no version pin and no re-sync path, hand-edited until it diverged.
The decision keeps the *act* of copying (unavoidable — Mill is files-in-repo)
and removes the *reason* it drifted (no pin, no ownership boundary, no
upgrade).

**Git submodule.** The target repo references Mill at a pinned commit; fixes
reach it via `git submodule update`. Rejected on three counts. (1) A submodule
cannot place files at the target's root — git hosts it in its own directory —
so `AGENTS.md`, `CLAUDE.md` and `.claude/settings.json` must still be written
into the parent, which reintroduces a copy exactly where drift matters most
(the entry file is the first thing a harness reads). (2) Onboarding cost:
`git clone --recursive`, `git submodule update --init`, and submodule state are
precisely the git knowledge #162 says a stranger does not have. (3)
`.mill/gauntlet` and `.mill/role-capabilities` are per-project files that cannot
live in a shared read-only submodule checkout — they would sit beside the
submodule path, reintroducing the very overlay/conflict the submodule was meant
to eliminate.

**Symlink to a central checkout.** Every project symlinks `.mill/` (or the
shared files) to one developer's `~/mill` checkout, so a fix reaches every
project instantly. Rejected: the symlink target is a per-developer path that
does not exist on a stranger's machine (#162, #163), breaks clone portability,
and needs `core.symlinks` on Windows. Wholesale symlinking of `.mill/` is
additionally wrong on principle — it would share `.mill/gauntlet` and
`.mill/role-capabilities` across projects, which ADR 0007 and ADR 0009 made
per-project.

**Harness-native plugin distribution** (Claude Code plugin marketplaces and
skills). Solves skill distribution for one harness and nothing else: there is
no cross-harness channel (Codex, Copilot, omp and Pi each have their own, or
none), no plugin channel distributes or runs the bash gates, and none can
create the project's `gauntlet` and `role-capabilities`. Rejected as the
mechanism; it may be layered on later as a Claude-only convenience on top of
the versioned copy.

**Published package** (npm, PyPI, or a registry). Copy-in with a registry and a
version. The version pin is the right half of it, but Mill is Markdown and
bash, not a runtime artifact; a registry installs into `node_modules` or
site-packages, not into the repo root where harnesses read it; and the project
has no registry infrastructure (the Go binary that had release tooling was
retired by ADR 0006). The useful half — a versioned, fetchable artifact — is
adopted via git tags and `install.sh`, which ADR 0004 already established.
Rejected as the channel, adopted as the versioning spine.

**Fetch-on-demand script** (no install, fetch every session). Rejected: it
makes the harness depend on network and a live source every run, which breaks
the offline, deterministic property of "the policy lives in the repo and is
versioned with it." The installer fetches *once*, at install and at upgrade;
the copy in between is a normal versioned file.

## Consequences

**Gained.** One source of truth (the tag), a machine-readable staleness signal
(the version pin), and a one-command, non-destructive fix path
(`mill-install --upgrade`). The three divergent gate-script copies become
impossible to recreate: there is no second checked-in copy to diverge. A
project that installed six months ago is one command from current, and that
command is defined in terms of what it replaces and what it leaves alone.

**Lost.** A fix no longer reaches an installed project *without* an operator
action — the upgrade is run, not automatic (a submodule or symlink would have
delivered it on `git pull` or immediately). That is the honest cost of avoiding
submodule and symlink: staleness is made visible and cheap to fix, but it is
not made self-healing. The version pin is the guard against staleness going
unnoticed.

**Per-project files remain per-project.** `.mill/gauntlet` and
`.mill/role-capabilities` are the project's, upgraded never, by design (ADR
0007, 0009). A change to Mill that needs a *new* per-project fact (e.g. a new
capability category) must reach projects through a documented migration note in
the release, exactly as ADR 0007 did for the category map — the upgrade will
not and cannot write those files for the project.

**Migration.** A project installed under ADR 0008 has a manifest and possibly
adopted hooks; it predates the version pin. The first `mill-install --upgrade`
on such a project writes the version pin and adopts the ownership boundary from
that point forward; the ADR 0008 manifest (and the `mill-uninstall` undo path
it records) is superseded along with the installer it described.

**Supersedes ADR 0008.** ADR 0008 described an installer (`mill-install` copying
a `scaffold/`) that no longer exists, and its non-destructive mechanics
(validate first, manifest, uninstaller) were written against that copy. ADR
0009 already superseded 0008's hooks step; this ADR supersedes the rest — the
copy mechanism itself. The non-destructive *intent* (never destroy what is
there) is retained and sharpened into the ownership boundary.

## Notes

- The `scaffold/` directory described in the context is being deleted in a
  parallel dispatch; this ADR does not recover it and does not propose a
  replacement under another name. The versioned copy is generated from a tag,
  not checked in.
- The harness evidence above is what I could read today: the four "verified"
  rows cite files in this repository (or the scaffold, read before deletion).
  Pi is unverified and the installer must not write a path for it until its
  convention is confirmed.
