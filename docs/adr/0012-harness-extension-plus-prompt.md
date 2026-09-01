# ADR 0012: Mill installs as a harness extension plus a prompt, not a script

**Status:** Accepted
**Date:** 2026-09-01
**Decided by:** Architect
**Related:** #162, #148, #173. Supersedes the mechanism of ADR 0011; retains its
analysis. Superseded by ADR 0013, which retires the per-prompt-identity
criterion.

## Context

ADR 0011 decided that Mill installs as a versioned copy materialised from a git
ref by an installer, with `mill-install --upgrade` as the fix-delivery path. The
CTO has changed that decision, and two findings support the change.

**A script that writes into someone else's repository is the defect class we
already have.** On 2026-08-31 at 18:54 a delegated agent wrote
`core.hooksPath=/dev/null` into this repository's `.git/config`. Every worktree
inherited it; every git hook in the project, including the project's own, was
disabled. No script in the tree sets it. That is #148's class — a delegated
agent mutating a repository's git configuration — recurring three weeks after
the Go runner that first caused it was deleted, and it is what #173 objects to
on principle: Mill should not touch a project's git configuration at all. An
installer that runs in a stranger's repository is the same hazard with nobody
watching.

**The harnesses already solve distribution and updates.** Superpowers 6.3.0 is
installed on this machine as one package carrying a manifest per harness,
installed with `/plugin install superpowers@claude-plugins-official` and updated
by the marketplace. No bespoke installer, no `--upgrade`, no versioned copy.

## Decision

**Mill is distributed as a harness-native extension, and configured by a
prompt. Mill ships no installer binary and no installer script, and never
writes into a repository it was not asked to write into.**

The split is not arbitrary — ADR 0007 and ADR 0009 forced it:

- **What the extension ships: the mechanism.** The `delegate` skill, the hooks
  that keep the coordinator's identity in context, and the gate scripts. Same
  for every project, updated by the harness's own plugin or extension channel.
- **What the prompt guides: the project's own state.** `.mill/gauntlet` holds
  that project's build, lint and test commands (ADR 0009).
  `.mill/role-capabilities` maps categories to that project's file patterns
  (ADR 0007). Neither can be packaged, because both differ per project — and
  both are exactly the files a human should see and approve rather than have a
  script write unattended.

The install prompt is a document any agent can read and execute, in the user's
own session, with the user watching each write.

### 1. Which harnesses, and with what evidence

Claude Code, Gemini, Pi and OpenCode are verified from files read in the
superpowers 6.3.0 package on this machine; `omp`'s `@oh-my-pi/pi-coding-agent`
name is unverified.

**The per-prompt-identity question is retired by ADR 0013.** Mill is invoked,
not ambient: it registers no hooks, so whether a harness can inject the
coordinator's identity per prompt is no longer a criterion. The table asks the
two surviving questions — can the harness load the skill, and can it run the
bash gates. Pi, OpenCode and Hermes were `none` only on the strength of the
retired question; re-derived on the two survivors, each is `write`.

| Harness | Loads skill | Runs bash gates | Manifest |
|---|---|---|---|
| Claude Code | verified — `.claude-plugin/plugin.json` (`skills`) | verified — shell tool | keep |
| Codex | verified — `.codex-plugin/plugin.json` (`skills`); `.agents/` is Codex's marketplace dir, not a separate harness | verified — shell tool | write |
| Cursor | verified — `.cursor-plugin/plugin.json` (`skills`) | verified — shell tool | write |
| Kimi | verified — `.kimi-plugin/plugin.json` (`skills`) | verified — shell tool | none |
| Devin | fails — `.devin-plugin/plugin.json` has no `skills` field | verified — shell tool | none |
| Gemini | verified — `gemini-extension.json` (`contextFileName`); `skills/` auto-discovered | verified — shell tool | none |
| Pi | verified — `package.json` (`pi.skills`) | verified — shell tool | write |
| OpenCode | verified — `.opencode/plugins/superpowers.js` (`config` hook) | verified — shell tool | write |
| Hermes | verified — `.hermes-plugin/__init__.py` (`register_skill`) | verified — shell tool | write |
| omp | unverified — no manifest read; catalog name `@oh-my-pi/pi-coding-agent` remains unverified (no file names it) | verified — shell tool | none |

The package root also carries `.codex-plugin/`, `.cursor-plugin/`,
`.devin-plugin/`, `.kimi-plugin/`, `.hermes-plugin/` and `.agents/` directories.
Their presence is verified from the directory listing; their conventions are
**unverified** until their manifest bodies are read.

### 2. When the extension and the project's files disagree

**The extension version owns the schema; the project file owns the values.**
When an extension version expects a `role-capabilities` key the project's file
does not have, the extension fails closed: it names the missing key and its own
version, and points at the install prompt — it never writes the fix itself.
This replaces ADR 0011's manifest answer: instead of an installer reconciling
two copies by diffing against a recorded ref, the extension validates the
project file against its own declared schema and reports, and the prompt —
executed under supervision — performs the write.

### 3. How a project knows which Mill it is running

**The extension's version, from the harness's plugin or extension registry, is
the authoritative Mill version; the project's files carry no version and need
none, because they are validated against the extension's schema.** The
extension stamps its version in its own outputs — the coordinator-identity hook
context carries it — so the running version is visible in every session. No ref
is recorded in the repository.

### 4. What replaces `--upgrade` for the project-owned half

**The extension updates itself through the harness channel; the project-owned
half is migrated by the prompt, and the project is told by the validation
failure itself.** When a new Mill needs a new key in `.mill/gauntlet` or a new
category in `.mill/role-capabilities`, the extension's validation reports the
missing key together with the extension version, and the install prompt tells
the project what to add and why. The project re-runs that prompt section in the
user's session, watching the write — there is no background upgrade and no
script that edits the project's files.

### 5. What this supersedes

**ADR 0011's mechanism is superseded: the versioned copy materialised from a git
ref, the install manifest, and `mill-install --upgrade`.** Its analysis survives
unchanged: the Mill-owned versus project-owned split (the extension is
Mill-owned; `.mill/gauntlet` and `.mill/role-capabilities` are project-owned),
the harness table with its verified/unverified discipline, and the conclusion
that a copy into the target repository is unavoidable for the root entry files
— those entry files are still written into the repository, now by the prompt
under the user's supervision rather than by an installer script.

## Alternatives considered

**ADR 0011's installer (versioned copy + `--upgrade`).** Rejected as the
mechanism: the installer is the defect class this ADR exists to remove — a
program that writes into a repository it was not asked to write into. Its
analysis is retained (see question 5).

**Harness-native extension plus a script for the project-owned half.** A hybrid:
the extension ships the mechanism, a script migrates `.mill/gauntlet` and
`.mill/role-capabilities` when a new Mill needs a new key. Rejected: the
project-owned half is exactly the half that must be human-approved, and a
script that edits it unattended reintroduces the unattended writer under
another name. The prompt — read and executed with the user watching — is the
migration path.

**Registry package (npm, PyPI).** Rejected in ADR 0011 because a registry
installs into `node_modules` or site-packages, not into the repo root where a
harness reads it. The harness's own channel avoids this by construction: the
harness already knows where to place a plugin and how to update it. A
harness-native extension is a registry install on the harness's channel, not a
bespoke one.

**Submodule / symlink to a central checkout.** Already rejected in ADR 0011
(entry files must be at the repo root; per-project files cannot be shared).
Still rejected.

## Consequences

**Gained.** No Mill-owned program writes into a stranger's repository — the
defect class of #148 and #173 is removed by construction. Fixes to the
mechanism reach every project through the harness's update channel, the same
channel that already updates superpowers 6.3.0. The project-owned files are
always seen and approved by a human before they change.

**Lost.** There is no single `mill-install --upgrade` command. The project-owned
half migrates only when someone re-reads and re-runs the install prompt, so a
new key reaches a project only after a human acts — staleness is surfaced by
the extension's validation failure, not fixed silently.

**The version-skew answer has a cost.** Failing closed means a project whose
files lag the extension stops working until it re-runs the prompt. That is
deliberate: a loud, named failure is preferable to a script silently editing
project state.

**Migration.** A project installed under ADR 0011 (manifest present, version pin
recorded) keeps its files; the manifest and `mill-install --upgrade` are
superseded. Nothing in this ADR changes `.mill/gauntlet` or
`.mill/role-capabilities` — they remain per-project, upgraded never, by design
(ADR 0007, ADR 0009).
