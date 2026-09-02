# Mill onboarding gap audit — 2026-09-02

Audit of the path from "someone wants to use Mill" to "a first dispatch is
verified", walked in the five stages of `INSTALL.md`. Method follows
`docs/BACKLOG-TRIAGE-2026-08-31.md`: a verdict is a checkable statement, not an
impression, and every verdict cites the file or command that decides it.

Two constraints on the evidence, both honoured throughout:

- **Read what is committed, not the working tree.** Every "what Mill ships"
  question is answered with `git show HEAD:<path>` or `git grep … HEAD`, never
  `cat`/`grep` on the checkout. The manifest defect in #198 was invisible to a
  dirty-tree read; this audit does not repeat that.
- **Nothing was installed.** A claim that can only be settled by running an
  install is marked `UNVERIFIED — needs an install` and the settling command is
  named. Behaviour of a missing prerequisite is predicted from the code path
  shown and flagged `[code-derived, not run]` — this machine has every tool and
  removing one is out of scope.

Verdicts, exactly one per finding — **BLOCKS INSTALL** · **DEGRADES** · **ASSUMPTION UNSTATED** · **OK**. `OK` findings are kept so the document does not read as a fault list only.

---

## 1. Before installing

### F1.1 — Orca must already be installed, running and reachable

**Assumed.** `orca` on `PATH` and running. No document on the onboarding path
says so. `INSTALL.md` names Orca only at step 4, as an error remedy.

```bash
git grep -n -i orca HEAD -- INSTALL.md
```

```
HEAD:INSTALL.md:139:Orca's coordination guide: load with 'orca skills get orca-cli' and 'orca skills get orchestration'
HEAD:INSTALL.md:142:If it prints `error: Orca is not running. Run 'orca open' first.` start Orca
HEAD:INSTALL.md:143:(`orca open`) and re-run. If it prints `error: Not a Mill project (no
```

```bash
git grep -n -iE 'install orca|orca install' HEAD -- INSTALL.md README.md
```

```
(no matches)
```

`README.md` links `https://onorca.dev` but never says "install Orca first".
ADR 0005 calls Orca "a dependency" but the ADR is not on the onboarding path and
nothing on the path cites it.

**When absent.** `.mill/checks/mill-preflight` prints a named error, but only at
step 4, after steps 1–3 have been attempted:

```bash
git grep -n -E 'Orca is not running|orca open' HEAD -- .mill/checks/mill-preflight
```

```
HEAD:.mill/checks/mill-preflight:68:    echo "error: Orca is not running. Run 'orca open' first." >&2
```

The error conflates "not installed" with "not running", and the remedy
`orca open` presumes the binary exists. A machine without Orca gets a command it
cannot run, wrapped in a message that names the wrong failure.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "INSTALL.md has no step 0 — state Orca (install, start, reachable) before the harness install"

---

### F1.2 — An agent Orca can launch

**Assumed.** Orca has at least one agent configured before the first dispatch.
No onboarding document states this. ADR 0005 records it as a known, accepted gap:

```bash
git grep -n -E 'fresh install has none|configured before' HEAD -- docs/adr/0005-orca-as-execution-substrate.md
```

```
HEAD:docs/adr/0005-orca-as-execution-substrate.md:129:   Code and omp configured. A fresh install has none of them, and Orca requires
```

INSTALL.md's only agent mentions are harness mechanics — the marketplace path,
the `cursor-agent` CLI, and one prose mention of the terminal agent — never "an
agent you must configure for Orca":

```bash
git grep -n -iE 'agent|model|provider' HEAD -- INSTALL.md
```

```
HEAD:INSTALL.md:46:marketplace manifest as `.agents/plugins/marketplace.json`, which declares the
HEAD:INSTALL.md:59:cursor-agent plugin marketplace add <gitUrl>
HEAD:INSTALL.md:63:plugin for install from the manifest. For a local checkout, the terminal agent
HEAD:INSTALL.md:64:loads the plugin directory directly with `cursor-agent --plugin-dir <path>`.
```

None of the four lines mentions a model or a provider.

**When absent.** `orca orchestration worker-start --agent <id>` fails at the
first dispatch with Orca's own error. The exact message is `[code-derived, not
run]`. Nothing on the path tells the user to configure an agent first.

**Verdict:** **ASSUMPTION UNSTATED**

**Covered by:** #162 (its acceptance records "the agent registered").

---

### F1.3 — The operator's `.mill/agents` catalog

**Assumed.** `.mill/agents` (or the tracked `.mill/agents.example`) names the
chosen agent with a `submit:` marker. `mill-dispatch` reads it as step 0 and
exits without it:

```bash
git show HEAD:.mill/checks/mill-dispatch | sed -n '110,116p'
```

```
agent_entry="$(grep -m1 -F '`'"$agent"'`' "$catalog" || true)"
submit_marker="$(printf '%s' "$agent_entry" | grep -oE 'submit: (explicit|self)' | head -n1 | sed 's/^submit: //' || true)"
if [[ "$submit_marker" != "explicit" && "$submit_marker" != "self" ]]; then
    echo "mill-dispatch: no submit marker for agent '$agent' in $catalog" >&2
    echo "mill-dispatch: add \`submit: explicit\` (needs an enter) or \`submit: self\` (submits its own brief) to its entry" >&2
    exit 2
fi
```

`INSTALL.md` never tells the user to create `.mill/agents`. Worse, the shipped
skill says the catalog is inert, which is no longer true — `mill-dispatch` reads
the `submit:` marker from it:

```bash
git grep -n 'No script consults' HEAD -- .claude/skills/delegate/SKILL.md
```

```
HEAD:.claude/skills/delegate/SKILL.md:81:`.mill/agents` is a catalog of what exists on this machine. No script consults
```

And `.mill/agents.example` is explicit that its entries are this machine's, not
the new user's:

```bash
git grep -n 'this machine' HEAD -- .mill/agents.example
```

```
HEAD:.mill/agents.example:1:# .mill/agents — what runs on this machine.
HEAD:.mill/agents.example:3:A catalog of the agents Orca has configured on this machine. The coordinator
HEAD:.mill/agents.example:30:    These ids are this machine's settings, recorded as an example of the
HEAD:.mill/agents.example:32:  - `prewalk.enabled` and `task.prewalk` — both `true` on this machine.
```

**When absent.** A user who follows the skill and skips `.mill/agents` (because
"no script consults it") hits `no submit marker for agent` on the first
dispatch — only if their agent name is not one of the five hardcoded in
`.mill/agents.example`.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "INSTALL.md must direct the operator to write `.mill/agents`; the delegate skill's 'no script consults it' is stale since `mill-dispatch` reads the submit marker"

---

### F1.4 — A git repository

**Assumed.** The project is a git repository. `mill-preflight` never checks for
one; `mill-verify` requires one (`git reflog`, `git diff`, `git status`), and
`mill-dispatch` creates a worktree through Orca.

```bash
git grep -n git HEAD -- .mill/checks/mill-preflight
```

```
(no matches)
```

```bash
git grep -n -iE 'git repo' HEAD -- INSTALL.md README.md
```

```
(no matches)
```

**When absent.** Preflight passes; the first dispatch fails at worktree
creation with Orca's error. `[code-derived, not run]` — but nothing on the path
tells the user a git repository is required.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "Declare 'a git repository' as an install prerequisite; preflight could check it before any dispatch"

---

### F1.5 — bash

**Assumed.** `bash` is available. Every gate script is bash; `INSTALL.md` never
says so.

```bash
git grep -n '^#!' HEAD -- .mill/checks
```

```
HEAD:.mill/checks/common.sh:1:#!/bin/bash
HEAD:.mill/checks/mill-dispatch:1:#!/usr/bin/env bash
HEAD:.mill/checks/mill-preflight:1:#!/usr/bin/env bash
HEAD:.mill/checks/mill-verify:1:#!/usr/bin/env bash
HEAD:.mill/checks/role-enforce:1:#!/usr/bin/env bash
```

`common.sh` is sourced by the others, so its own shebang never executes. The FRD
for #162 (acceptance 3) already demands the README declare OS and shell; the
shipped `INSTALL.md` declares neither.

**When absent.** On a machine without bash the scripts fail at first run; on
macOS the system `bash` is 3.2, which is untested for these scripts.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "Declare bash (and test on macOS 3.2) in INSTALL.md"

---

### F1.6 — jq

**Assumed.** `jq` is on `PATH`. `mill-dispatch` and `mill-verify` pipe Orca's
JSON through it, heavily:

```bash
git grep -c 'jq ' HEAD -- .mill/checks
```

```
HEAD:.mill/checks/mill-dispatch:25
HEAD:.mill/checks/mill-verify:5
```

`INSTALL.md` and `README.md` never mention it:

```bash
git grep -n jq HEAD -- INSTALL.md README.md
```

```
(no matches)
```

**When absent.** The failures are confusing, not named. In `mill-dispatch` the
task-id extraction yields empty and the user sees
`task-create returned no task id` rather than "jq is missing".
`[code-derived, not run]`

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "Declare jq as an install prerequisite in INSTALL.md"

---

## 2. Installing the extension

### F2.1 — The plugin ships the skill only; the checks and roles never arrive

**Assumed.** Step 1's first paragraph states the extension ships "the `delegate`
skill and the gate scripts in `.mill/checks/`":

```bash
git show HEAD:INSTALL.md | sed -n '5,7p'
```

```
The **extension** ships the mechanism — the `delegate` skill and the gate
scripts in `.mill/checks/`. It is identical for every project and is updated
by the harness's own plugin or extension channel.
```

The manifest declares `skills` only, and `.claude/skills/` contains exactly one
file:

```bash
git grep -n '"skills":' HEAD -- .claude-plugin/plugin.json
```

```
HEAD:.claude-plugin/plugin.json:18:  "skills": [
```

```bash
git show HEAD:.claude-plugin/plugin.json | sed -n '18,20p'
```

```
  "skills": [
    "./.claude/skills"
  ],
```

```bash
git ls-tree -r HEAD .claude/skills
```

```
100644 blob 6ca3a00c254865ed9b3b0cdd8fe85c63684ac7cf	.claude/skills/delegate/SKILL.md
```

No manifest references `.mill/roles/` or `.mill/checks/`. So a successful
install puts one skill in the harness and puts nothing in the target project.
`mill-preflight` then fails its layout check:

```bash
git grep -n -E 'Not a Mill project|\.mill/roles' HEAD -- .mill/checks/mill-preflight
```

```
HEAD:.mill/checks/mill-preflight:77:if [[ ! -d .mill/roles ]]; then
HEAD:.mill/checks/mill-preflight:78:    echo "error: Not a Mill project (no .mill/roles/)." >&2
HEAD:.mill/checks/mill-preflight:98:    roles_dir="$mill_root/.mill/roles"
```

INSTALL.md's remedy for that error is to "go back to step 1 and re-check the
install" — but reinstalling the extension cannot produce `.mill/roles/`, because
the extension never shipped it:

```bash
git show HEAD:INSTALL.md | sed -n '142,145p'
```

```
If it prints `error: Orca is not running. Run 'orca open' first.` start Orca
(`orca open`) and re-run. If it prints `error: Not a Mill project (no
.mill/roles/)` the extension's role files are not in the project — go back to
step 1 and re-check the install.
```

**When absent.** The new user cannot get past step 4. Confirmed from the real
install, not just read from the code: #198 reports "`.mill/roles/` and
`.mill/checks/` had to be copied into the target project by hand".

**Verdict:** **BLOCKS INSTALL**

**Covered by:** #194 (the two-root contradiction) and #198 (the real-install confirmation).

---

### F2.2 — Nothing catches an invalid manifest; CI validates a different file than the installer reads

**Assumed.** The committed manifest is installable. It was not, for weeks, and
no gate caught it. CI checks only bash syntax and ROLE.md frontmatter:

```bash
git show HEAD:.github/workflows/ci.yml
```

```
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Syntax-check gate scripts
        run: |
          status=0
          for f in .mill/checks/*; do
            [ -f "$f" ] || continue
            bash -n "$f" || status=1
          done
          exit "$status"
      - name: Validate ROLE.md frontmatter
        run: |
          status=0
          for f in $(find .mill/roles -name 'ROLE.md'); do
            head -1 "$f" | grep -q '^---' || { echo "FAIL: $f — missing frontmatter opening"; status=1; continue; }
            sed -n '/^---$/,/^---$/p' "$f" | grep -q '^role:' || { echo "FAIL: $f — missing role: field"; status=1; }
          done
          exit "$status"
```

No step parses `plugin.json`. #198's reproduction shows `claude plugin validate .`
passing on the tree whose manifest the installer refused — `validate` checks the
marketplace manifest, the installer reads the plugin manifest. The one-character
defect itself is fixed in `7f9de8d`; the absence of any check that would have
caught it is what remains.

**When absent.** A future manifest regression ships through every gate Mill has,
because none of them reads a manifest.

**Verdict:** **DEGRADES**

**Covered by:** #198 ("nothing catches an invalid plugin manifest").

---

### F2.3 — The Codex and Cursor install paths have never been executed

**Assumed.** The Codex and Cursor commands in step 1 install a working
extension. INSTALL.md says so, and in the same breath says it cannot be run:

```bash
git show HEAD:INSTALL.md | sed -n '44,48p'
```

```
`<source>` is a local path, `owner/repo`, or Git URL naming Mill's marketplace,
and `<marketplace>` is the name that marketplace declares. Mill ships its
marketplace manifest as `.agents/plugins/marketplace.json`, which declares the
name `mill-dev` and the plugin `mill`, so once that marketplace is added the
install is `codex plugin add mill@mill-dev`. Mill is not yet published, so
```

**Verdict:** **ASSUMPTION UNSTATED** — the two commands are presented as
install paths but neither has run end to end.

**UNVERIFIED — needs an install.** The settling commands are
`codex plugin marketplace add <source> && codex plugin add mill@mill-dev` and
`cursor-agent plugin marketplace add <gitUrl>` on a machine with those harnesses.

**Covered by:** #185 (multi-harness packaging; Codex/Cursor conventions marked unverified).

---

### F2.4 — A harness that cannot run the gates is refused, and the table is cited

**Assumed.** Nothing further. Step 1's "Any other harness" paragraph names the
three requirements (load the skill, inject identity, run bash gates) and points
at the ADR 0012 table before installing for an unlisted harness. This is the
one place on the path that says "read X before acting".

```bash
git show HEAD:INSTALL.md | sed -n '70,76p'
```

```
**Any other harness.** Mill ships a manifest only where a harness can load the
skill, inject the coordinator's identity per prompt, and run the bash gates. A
harness that cannot do all three is not supported.
`docs/adr/0012-harness-extension-plus-prompt.md` records, for every harness
Mill has examined, which of the three hold and which do not — read its table
before installing for a harness not named here, and stop if its row is not
`keep` or `write`.
```

**Verdict:** **OK**

No issue; recorded for proportion.

---

## 3. The two project files

### F3.1 — Steps 2 and 3 work by hand

**Assumed.** `mkdir -p .mill` then two `cat` heredocs. No tool dependency beyond
the shell, no Mill code involved. The real install confirms it:

```bash
git show HEAD:INSTALL.md | sed -n '84,87p'
```

```
mkdir -p .mill
cat > .mill/gauntlet <<'EOF'
#!/bin/bash
build="<the build command>"
```

#198 reports "INSTALL steps 2 and 3 by hand with no friction" on the target
project.

**Verdict:** **OK**

No issue; recorded for proportion.

---

### F3.2 — Two roots landed, but INSTALL.md still describes one

**Assumed.** One directory serves as both install and project. ADR 0014
(landed in `7f55046`) split them into an install root (`.mill/checks/`,
`.mill/roles/`, the skill) and a project root (`.mill/gauntlet`,
`.mill/role-capabilities`), with `--project-root` passed explicitly and the
distinct-trees refusal in `mill-verify`. `INSTALL.md` never mentions either
root, never mentions `--project-root`, and never says where `.mill/roles/` and
`.mill/checks/` come from in a real install — its step 1 says the extension
ships them (F2.1), which the manifest contradicts.

```bash
git log --oneline -1 7f55046 --format='%h %s'
```

```
7f55046 Land two-roots impl: the project supplies what differs per project
```

```bash
git grep -n 'project-root\|two root\|install root\|project root' HEAD -- INSTALL.md
```

```
(no matches)
```

**When absent.** A user who follows INSTALL.md gets the single-root mental
model; the tooling now refuses the degenerate case (project root == worktree)
that single-root thinking invites, and the refusal is a named error the user has
never been prepared for.

**Verdict:** **ASSUMPTION UNSTATED**

**Covered by:** #194.

---

## 4. Verifying

### F4.1 — The Orca-reachability check matches what `orca status` actually prints

**Assumed.** `orca status` prints `runtimeReachable: true`. It does, on this
machine:

```bash
orca status
```

```
appRunning: true
pid: 3793429
desktopWindowStatus: available
runtimeState: ready
runtimeReachable: true
runtimeId: 6205b63b-1c0c-4d96-9748-a3c979041eb2
graphState: ready
```

Preflight greps exactly that token:

```bash
git grep -n 'runtimeReachable' HEAD -- .mill/checks/mill-preflight
```

```
HEAD:.mill/checks/mill-preflight:67:if ! orca status 2>/dev/null | grep -q "runtimeReachable: true"; then
```

**Verdict:** **OK**

No issue; recorded for proportion.

---

### F4.2 — `mill_root` resolves from the check scripts' physical location, so a symlink mis-resolves

**Assumed.** `.mill/checks/` was copied. A natural shortcut — symlinking the
checks into the project instead of copying — breaks resolution, because
`mill_root` derives from `BASH_SOURCE` with a logical `pwd` (not `pwd -P`):

```bash
git grep -n 'mill_root=' HEAD -- .mill/checks/role-enforce
```

```
HEAD:.mill/checks/role-enforce:41:mill_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
```

```bash
git grep -n -E 'here=|mill_root=' HEAD -- .mill/checks/mill-preflight
```

```
HEAD:.mill/checks/mill-preflight:96:    here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEAD:.mill/checks/mill-preflight:97:    mill_root="$(cd "$here/../.." && pwd)"
```

#198's real-install note confirms the consequence: "Symlinking does not
substitute for copying, because `mill_root` resolves from the physical location
of the check scripts — a symlink sends `role-capabilities` and `gauntlet`
lookups back to the Mill checkout instead of the target project."

**When absent.** The user who symlinks gets through preflight's directory check
and only discovers the mis-resolution at `--brief` or at verification, when role
lookups land in the wrong tree.

**Verdict:** **DEGRADES**

**Covered by:** #198.

---

## 5. The first dispatch

### F5.1 — No onboarding document walks the first dispatch

**Assumed.** The user somehow knows what to do after step 4. `INSTALL.md` ends
at step 4 and a "What Mill does not do" list. `README.md` names
`mill-dispatch --help` and `mill-verify --help` but gives no procedure. The only
procedure lives in the delegate skill — which step 1 shipped into the harness,
but which `INSTALL.md` never tells the user to read — and in
`.mill/roles/product-engineer/ROLE.md`, which the skill references and which the
plugin did not ship.

INSTALL.md's own headings end at step 4; the fifth section is a "what Mill
does not do" list, and the dispatch command never appears anywhere in it:

```bash
git grep -n '^## ' HEAD -- INSTALL.md
```

```
HEAD:INSTALL.md:16:## 1. Install the extension
HEAD:INSTALL.md:78:## 2. Create `.mill/gauntlet`
HEAD:INSTALL.md:104:## 3. Create `.mill/role-capabilities`
HEAD:INSTALL.md:128:## 4. Verify the install
HEAD:INSTALL.md:147:## 5. What Mill does not do
```

```bash
git grep -c 'mill-dispatch' HEAD -- INSTALL.md
```

```
0
```

```bash
git grep -n '\.mill/roles' HEAD -- .claude/skills/delegate/SKILL.md
```

```
HEAD:.claude/skills/delegate/SKILL.md:17:Workers execute and report; you sequence work. Read `.mill/roles/COMMON.md`
HEAD:.claude/skills/delegate/SKILL.md:150:procedure: `.mill/roles/product-engineer/ROLE.md`.
```

The skill instructs reading two files the plugin did not ship (F2.1).

**When absent.** A new user following only `INSTALL.md` reaches "preflight
passes" and has no documented next command. The path from "working install" to
"a first dispatch is verified" is absent.

**Verdict:** **BLOCKS INSTALL**

**Proposed issue:** "INSTALL.md has no step 5 — the first dispatch (brief → mill-dispatch → mill-verify → land) is documented only in the delegate skill, which step 1 ships but step 4 never directs the user to read"

---

### F5.2 — The provider (Orca) and the model are assumed, never established

**Assumed.** Orca is the execution substrate, and the operator names an agent
and a model per dispatch. The CTO's named gap: neither belongs in the critical
path of a first install, and nothing on the path says either is required.

The provider assumption is total — it is in every manifest's description and in
README's first paragraph:

```bash
git grep -n -i orca HEAD -- README.md .claude-plugin/plugin.json | head -3
```

```
HEAD:README.md:5:specialised workers through [Orca](https://onorca.dev).
HEAD:.claude-plugin/plugin.json:3:  "description": "Skill plus policy directory: role definitions and gate scripts that turn one AI session into a coordinator dispatching specialised workers through Orca",
```

The model requirement exists only in the skill (§2 "Name the agent and model per
dispatch", "There is no default to hide behind"), which is shipped into the
harness and never referenced by INSTALL.md:

```bash
git grep -n -E 'Name the agent and model|no default to hide' HEAD -- .claude/skills/delegate/SKILL.md
```

```
HEAD:.claude/skills/delegate/SKILL.md:42:## 2. Name the agent and model per dispatch
HEAD:.claude/skills/delegate/SKILL.md:46:(command-code), set the model first and say so. There is no default to hide
```

Model selection differs per agent — `omp` takes no `--model`, `command-code`
reads a global file it rewrites, `claude`/`cursor` accept `--model` — and the
catalog says so only in the example file:

```bash
git grep -n 'no shared registry' HEAD -- .mill/agents.example
```

```
HEAD:.mill/agents.example:48:There is no shared registry here. Each agent exposes its own models (omp lists
```

`INSTALL.md` never mentions provider, model, or the catalog
(`git grep -n -iE 'model|provider' HEAD -- INSTALL.md` → no matches).

**When absent.** A user can complete steps 1–4 without ever learning that a
first dispatch needs an agent and a model, and that the model must be wired per
agent.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "Establish whether the provider and the model belong in the first-install path; if they do, INSTALL.md must state them as prerequisites and the agent catalog must be created in step 0, not inferred from the developer's machine"

---

### F5.3 — A Run must be bound to the terminal before the first dispatch

**Assumed.** `orca orchestration run-create` / `run-use` has been run once.
`mill-dispatch` explicitly does not create the Run:

```bash
git show HEAD:.mill/checks/mill-dispatch | sed -n '30,32p'
```

```
# A Run must already be bound to the invoking terminal (orca orchestration
# run-create / run-use, once, before the loop). This script does not create one.
set -euo pipefail
```

`INSTALL.md` and `README.md` never mention it:

```bash
git grep -n -E 'run-create|run-use' HEAD -- INSTALL.md README.md
```

```
(no matches)
```

**When absent.** The first `mill-dispatch` fails at `task-create`/`worker-start`
with an Orca error about the missing Run. `[code-derived, not run]`

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "Document `orca orchestration run-create` / `run-use` as a first-dispatch prerequisite in INSTALL.md"

---

### F5.4 — Verification and landing are unreferenced on the path

**Assumed.** The coordinator knows to run `mill-verify --worktree …
--project-root …` from the coordinator's repository, re-run the acceptance
criteria itself, and land only then. That procedure lives in the delegate skill
§5 and `.mill/roles/product-engineer/ROLE.md` — the latter not shipped by the
plugin, the former never referenced by `INSTALL.md`.

```bash
git grep -n 'project-root' HEAD -- .claude/skills/delegate/SKILL.md
```

```
(no matches)
```

`mill-verify` requires `--project-root` and refuses to infer it, so the
two-root model from F3.2 is load-bearing at landing — and the skill that teaches
landing never once names the flag:

```bash
git grep -n 'project-root is required' HEAD -- .mill/checks/mill-verify
```

```
HEAD:.mill/checks/mill-verify:8:# --project-root is required and must NOT be the worktree. It names the
HEAD:.mill/checks/mill-verify:173:    echo "error: --project-root is required (it supplies .mill/role-capabilities and .mill/gauntlet; it is never inferred from the working directory)" >&2
```

**When absent.** A user who reaches a settled dispatch has no documented
verification step and no documented landing step, and the one command that
verifies refuses to run until they pass a flag the onboarding never told them
about.

**Verdict:** **ASSUMPTION UNSTATED**

**Proposed issue:** "INSTALL.md's first-dispatch step must cover mill-verify (with --project-root) and the landing procedure, not just preflight"

---

### Marked-unverified claims

Exactly one finding rests on an install that was not performed:

- **F2.3** — the Codex and Cursor install commands. Settling commands:
  `codex plugin marketplace add <source> && codex plugin add mill@mill-dev`;
  `cursor-agent plugin marketplace add <gitUrl>`.

All other findings are settled by reading the committed tree. Predictions of
"what happens when a tool is absent" (F1.2, F1.4, F1.6, F5.3) are derived from
the code paths shown and are flagged `[code-derived, not run]`; they are not
`UNVERIFIED — needs an install` because they are not install claims — they need a
machine without the tool, and this machine has every tool. Removing one is out of
scope for this audit.
