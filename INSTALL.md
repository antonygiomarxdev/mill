# Installing Mill

Mill installs into two roots, and this document is the second of them. The two
are named and separated in `docs/adr/0014-two-roots-install-and-project.md`.

The **install root** ships the mechanism — the `delegate` skill, the role
definitions in `.mill/roles/`, and the gate scripts in `.mill/checks/`. It is
identical for every project and is updated by the harness's own plugin or
extension channel.

This document is the **project root** — the state that differs per project:
`.mill/gauntlet` (that project's build, lint and test commands) and
`.mill/role-capabilities` (that project's file patterns). Neither can be
packaged — both differ per project — and both are files a human should see and
approve rather than have a script write unattended. Run each step in the
user's session, with the user watching.

## 0. Confirm the prerequisites

Before installing anything, confirm each of these six things is already true
on the user's machine. Run the check in the user's session and read the
result aloud; if a check does not produce the passing output, stop and report
what you saw instead of continuing. Nothing in this step installs or writes.

| Prerequisite | Check | Missing |
| --- | --- | --- |
| Orca is installed, running and reachable | `orca status` | not installed: `orca: command not found`. Installed but not running: `orca status` prints no `runtimeReachable: true` line — start it with `orca open`. Step 4's preflight names this failure: `error: Orca is not running. Run 'orca open' first.` |
| An agent Orca can launch | none — there is no standalone check | the only way to learn whether an agent can be launched is to launch one, which is the dispatch itself; when the named agent is not configured, Orca fails the worker start with its own error. A fresh machine has no agents, and Mill records the ones it knows in `.mill/agents.example`. |
| The operator's `.mill/agents` catalog | `grep -n 'submit: ' .mill/agents.example` — the template's markers; the operator's own file has no standalone check, `mill-dispatch` enforces it | `mill-dispatch` refuses with exit 2: `mill-dispatch: no submit marker for agent '<agent>' in .mill/agents` |
| A git repository | `git rev-parse --is-inside-work-tree` | `fatal: not a git repository (or any of the parent directories): .git` |
| bash | `bash --version` | `bash: command not found` — no named error; every gate script is `#!/usr/bin/env bash` and fails at first run |
| jq | `jq --version` | `jq: command not found`; a dispatch that reaches it fails later with `task-create returned no task id` |

`.mill/agents` needs more than a line. It is the catalog of the agents Orca
has configured on this machine, one entry per agent. Each entry carries a
`submit:` marker that `mill-dispatch` reads to decide whether the worker needs
an empty enter after its brief is pasted: `submit: self` submits its own
brief, `submit: explicit` needs the enter. `.mill/agents` is gitignored (it
can hold the operator's credentials) and never committed; the example stays
tracked.

Build `.mill/agents` from the answers to these two questions — do not copy
the example and edit it, because the example's ids are this machine's and the
operator's file records what actually runs on theirs.

**Question one: which agent.** The options are exactly the agents
`.mill/agents.example` lists under `## Agents`, and no others:

- **`omp`** — `submit: explicit`
- **`command-code`** — `submit: self`
- **`claude`** — `submit: explicit`
- **`pi`** — `submit: explicit`
- **`cursor`** — `submit: self`

**Question two: which model, for the agent just chosen.** There is no shared
registry — each agent is wired differently, and the second question has a
different answer for each:

- **`omp`** — no `--model` flag. The model is configured inside omp, in the
  `modelRoles` record, and omp reports the active model in its own status
  bar. There is nothing to choose at install time; the setting lives in omp,
  not in Mill.
- **`command-code`** — rejects `--model` at `worker-start`. The model comes
  from the global `~/.commandcode/config.json`, which command-code rewrites
  itself. Its list is exposed and verified read-only: run
  `command-code --list-models`.
- **`claude`** — accepts `--model` at launch (`worker-start --agent claude
  --model <id>`). How to obtain the list of available ids is **unverified** —
  nothing in this repository records it. Do not guess a command.
- **`cursor`** — accepts `--model` at launch (`worker-start --agent cursor
  --model <id>`). How to obtain the list of available ids is **unverified** —
  nothing in this repository records it. Do not guess a command.
- **`pi`** — catalogue `submit: explicit`. How its model is selected is
  **unverified** — nothing in this repository records it.

`.mill/agents.example` records each agent's wiring in its `## Models` section;
the coordinator chooses per dispatch from what the chosen agent offers.

## 1. Install the extension

Ask which harness the project uses, then install Mill's extension for it.
Mill ships one manifest per supported harness; each declares the skill
directory.

**Claude Code** (`.claude-plugin/plugin.json`). Install Mill from its
marketplace:

```
/plugin install mill@<marketplace>
```

`<marketplace>` is the name Mill's marketplace is registered under in Claude
Code. Mill is not yet published to a marketplace, so this exact command cannot
run today; when the marketplace exists, this is the command. If the command
fails or the plugin is not listed after running it, the extension is not
registered — check the harness's plugin list for `mill` before continuing, and
stop if it is not there.

**Codex** (`.codex-plugin/plugin.json`). Add Mill's marketplace, then install
the plugin:

```
codex plugin marketplace add <source>
codex plugin add mill@<marketplace>
```

`<source>` is a local path, `owner/repo`, or Git URL naming Mill's marketplace,
and `<marketplace>` is the name that marketplace declares. Mill ships its
marketplace manifest as `.agents/plugins/marketplace.json`, which declares the
name `mill-dev` and the plugin `mill`, so once that marketplace is added the
install is `codex plugin add mill@mill-dev`. Mill is not yet published, so
these commands cannot run against a live marketplace today. If the add fails
or `codex plugin list` does not show `mill`, the marketplace was not added —
re-run `codex plugin marketplace add`, then `codex plugin list`, and stop if
`mill` is still absent.

**Cursor** (`.cursor-plugin/plugin.json`). Cursor has no single package-install
command: it reads a repository's manifest from an added marketplace. Add Mill's
marketplace:

```
cursor-agent plugin marketplace add <gitUrl>
```

`<gitUrl>` is the Git URL of Mill's repository; the Cursor app then lists the
plugin for install from the manifest. For a local checkout, the terminal agent
loads the plugin directory directly with `cursor-agent --plugin-dir <path>`.
Mill is not yet published, so the marketplace command cannot run today. If the
plugin does not appear in the Plugins panel after the marketplace is added, the
manifest was not read — confirm `.cursor-plugin/plugin.json` is present at the
repository root and stop if it is not.

**Any other harness.** Mill ships a manifest only where a harness can load the
skill, inject the coordinator's identity per prompt, and run the bash gates. A
harness that cannot do all three is not supported.
`docs/adr/0012-harness-extension-plus-prompt.md` records, for every harness
Mill has examined, which of the three hold and which do not — read its table
before installing for a harness not named here, and stop if its row is not
`keep` or `write`.

## 2. Create `.mill/gauntlet`

Ask the user for the project's build, lint and test commands. Then create the
file:

```
mkdir -p .mill
cat > .mill/gauntlet <<'EOF'
#!/bin/bash
build="<the build command>"
lint="<the lint command>"
test="<the test command>"
EOF
```

Leave out the line for any step the project does not have — an unset step is
valid. `mill-verify` reports it and skips it:

```
mill: build: not configured (no build=... in .mill/gauntlet)
```

A step that is set runs, and its command must exit 0. If the `cat` fails, the
`.mill` directory is missing or read-only: re-run `mkdir -p .mill`, confirm the
user has write access, and re-run the `cat`.

## 3. Create `.mill/role-capabilities`

Ask which file extensions the project's code, docs and config actually use.
Write them, one category per line:

```
cat > .mill/role-capabilities <<'EOF'
code=""
docs=".md"
config=".yml .yaml .json .toml"
policy=".md .sh .example"
scripts=".sh"
design=""
EOF
```

- `code=""` is correct for a project with no application code.
- An empty category means no role may write files of that category — the
  capability gate fails closed, deliberately.
- The categories are `code`, `docs`, `config`, `policy`, `scripts`, `design`.

If the `cat` fails, same as step 2: `mkdir -p .mill`, confirm write access,
re-run.

## 4. Verify the install

Run the preflight check:

```
.mill/checks/mill-preflight
```

A passing run prints the coordination-guide note and exits 0:

```
Orca's coordination guide: load with 'orca skills get orca-cli' and 'orca skills get orchestration'
```

If it prints `error: Orca is not running. Run 'orca open' first.` start Orca
(`orca open`) and re-run. If it prints `error: Not a Mill project (no
.mill/roles/)` the extension's role files are not in the project — go back to
step 1 and re-check the install.

## 5. The first dispatch

Step 4 proves the install works. This step proves a dispatch works: one brief,
one worker, one landing. The commands below are the recorded first dispatch —
commit `317172e`, adding a git-work-tree gate to `mill-preflight` — written as
a procedure. `<...>` placeholders are that run's values; a new dispatch
substitutes its own.

**Bind a Run.** A dispatch runs inside an Orca Run. If the terminal has none
bound, bind one:

```
orca orchestration run-use --id <run_id>
```

Do not probe with `orca orchestration run-show`: it requires `--id`, and
called without one it returns
`{"ok": false, "error": {"message": "Missing required --id"}}`. That is an
argument error, not evidence that no Run is bound.

**Write a brief.** Every dispatch starts from a brief with five parts, in
order: *Why*, *What to produce*, *Do not touch*, *Acceptance criteria*, and
*Raise a hand*. What goes in each is the delegate skill's, not this document's
— read `.claude/skills/delegate/SKILL.md`, section 4, and write the brief from
it.

**Dispatch.** One command runs the whole loop — preflight, the task, the
worker, the wait for the worker to settle, and the release:

```
.mill/checks/mill-dispatch --brief <file> --role <role> --agent <agent> \
    --name <slug> --title <title> --writes <path>
```

`--writes` is repeatable, one per path the brief asks the worker to write. The
command does not return until the worker has settled and been released; there
is no separate wait step and no separate release step.

It prints one line that reads as a failure and is not one:

```
mill-dispatch: worker-start: failed (agent_prompt_stalled) — brief pasted but unsent
```

For an agent catalogued `submit: explicit`, Orca pastes the brief and leaves it
unsent; `mill-dispatch` sends the enter and confirms it (`mill-dispatch: enter
accepted and confirmed ...`). The line appears on every successful dispatch to
such an agent. A `submit: self` agent submits its own brief and never produces
it.

**Verify.** Run from the coordinator's repository — never from the worktree,
which is the thing being judged:

```
.mill/checks/mill-verify --project-root <project-root> --worktree <worktree> \
    --role <role> --files-modified "<a,b,c>" --dispatch <ctx_id>
```

`--project-root` names the project whose `.mill/gauntlet` and
`.mill/role-capabilities` judge the worktree. A pass is necessary and not
sufficient: the coordinator re-runs the brief's acceptance criteria itself
rather than trusting the worker's report. That rule is
`.mill/roles/product-engineer/ROLE.md`; it is not restated here.

**Land and clean up.** Land the verified commit to `main` — the landing rule
is the delegate skill's, `.claude/skills/delegate/SKILL.md` section 5 — then
remove the worktree:

```
git worktree remove <worktree>
```

`mill-dispatch` returns without releasing the worker when the wait loop stops
for any reason other than a settled `worker_done`: the wait window can be spent,
or the worker can ask a question or raise an escalation, both of which halt the
loop by design. The worker is still live. Wait for its report, release it
manually (`orca orchestration worker-release --dispatch <ctx_id>`), then remove
the worktree. These are the exception, not the main path.

## 6. What Mill does not do

- Mill ships no git hook script at all: nothing in it runs on any git event.
- Mill writes nothing outside `.mill/` and the entry files.
- Mill never writes git configuration: no `git config`, no `core.hooksPath`.
  Issues #148 and #173 exist because an earlier Mill did exactly those things;
  this one does not.
