# Skills vs Scripts: Does Mill Need Computational Controls?

> Research brief — Architect role, 2026-08-15.
> Answers issue #166: do skills alone suffice, or does Mill need scripts?
> Refs: ADR 0006, spec-kit source, superpowers source.

---

## Executive Summary

**Verdict: ADR 0006 stands, but with one mandatory addition.**

Mill's skill-only approach is sound for its domain — organizational decomposition
and phase sequencing are well-suited to prose that agents follow. However, the
skill currently asks agents to do computational work that should be deterministic:
brief template expansion, project detection, and preflight validation. A minimal
shell helper (50–100 lines of bash) would make these computational without
returning to the 8,000-line Go binary.

spec-kit keeps a CLI because it writes agent-specific files in agent-specific
formats across a multi-layer resolution hierarchy. Mill does none of that — its
policy is agent-agnostic Markdown that agents read directly. The same reasoning
that makes spec-kit need code is what makes Mill not need it.

---

## 1. What spec-kit's CLI Does That Markdown Cannot

Analysis based on reading `github/spec-kit` source, not its README.

### 1.1 Template resolution across priority layers

**Source:** `src/specify_cli/presets/__init__.py:129–160` (`_materialize_constitution_template`), `PresetResolver.collect_all_layers`, `resolve_content`

spec-kit resolves templates through a four-tier priority stack:

1. Project-local overrides (`.specify/templates/`)
2. Installed presets (`.specify/presets/*/templates/`)
3. Bundled extensions
4. Core pack defaults

Each layer can declare a strategy: `replace`, `prepend`, `append`, or `wrap`.
The CLI walks the stack, applies strategies, and writes a single merged file.
This is computational — ordering, strategy application, and file composition
are deterministic operations an agent should not re-derive per session.

**What Mill asks an agent to do instead:** Nothing equivalent. Mill has no
template composition — role definitions are read directly. Mill's skill says
"read the ROLE.md" and the agent reads it. No resolution, no merging.

### 1.2 Agent-specific command file generation

**Source:** `src/specify_cli/agents.py:277–436` (`render_markdown_command`, `render_toml_command`, `render_yaml_command`, `render_skill_command`)

spec-kit converts a single command source into multiple agent-specific formats:
- Markdown for Claude/Codex (with `$ARGUMENTS` placeholders)
- TOML for GitHub Copilot (with `{{args}}` placeholders)
- YAML recipes for Goose
- SKILL.md for skills-backed agents

Each format has different frontmatter keys, different placeholder tokens, and
different path conventions. The CLI computes these deterministically and writes
agent-specific files to agent-specific directories.

**What Mill asks an agent to do instead:** Nothing. Mill's skill is
agent-agnostic — Claude Code reads it, or any other harness that can invoke
skills. There is no "write a command file for this specific agent" step because
Mill's policy is the skill itself, not something generated from a template.

### 1.3 Path rewriting

**Source:** `src/specify_cli/agents.py:197–229` (`rewrite_project_relative_paths`), `rewrite_extension_paths`

spec-kit command templates use repo-relative paths (`../../scripts/`,
`scripts/`, `templates/`). The CLI rewrites these to installed locations
(`.specify/scripts/`, `.specify/extensions/<id>/scripts/`) because the
installed path differs from the source path.

**What Mill asks an agent to do instead:** Nothing. Mill's paths are stable —
`.mill/roles/`, `.mill/checks/`, `.mill/skills/`. They do not change between
source and installed state because Mill has no "install" step; the repo IS the
installation.

### 1.4 Project detection and state

**Source:** `src/specify_cli/__init__.py:532–553` (`_require_specify_project`), `src/specify_cli/_init_options.py`

spec-kit detects whether you are in a spec-kit project (`.specify/` exists),
which agent was initialized (`ai` field in `init-options.json`), which script
variant is selected (`sh`/`ps`/`py`), and whether skills mode is active. These
determine how commands are registered and which directories to write.

**What Mill asks an agent to do instead:** The skill says "check that Orca is
up" and provides a bash one-liner:

```bash
orca status 2>/dev/null | grep -q "runtimeReachable: true" || orca open
```

This is inferential — the agent reads prose and decides whether to run the
command. A single project-detection check that fails is recoverable; a
coordinator that forgets to check before 15 dispatches has wasted an hour.

### 1.5 Extension/preset installation with integrity verification

**Source:** `src/specify_cli/shared_infra.py:42–99` (`verify_archive_sha256`), `src/specify_cli/presets/__init__.py` (catalog download, safe extraction)

spec-kit downloads archives from URLs, verifies SHA-256 hashes against catalog
declarations, extracts safely (preventing zip-slip via path traversal checks),
and registers installed items in a manifest for tracking.

**What Mill asks an agent to do instead:** Nothing. Mill has no downloadable
extensions. Roles are checked into the repo.

### 1.6 Summary: why spec-kit needs code

| Operation | spec-kit | Mill |
|---|---|---|
| Multi-layer template resolution | CLI computes | Not applicable — no layers |
| Agent-specific file generation | CLI writes multiple formats | Agent reads one skill directly |
| Path rewriting (source → installed) | CLI rewrites paths | Paths are stable |
| Project/agent detection | CLI reads state files | Agent reads environment |
| Archive download + integrity | CLI verifies SHA-256 | No downloads |

spec-kit's CLI exists because spec-kit writes derived artifacts. Mill's skill
exists because Mill's policy is the artifact.

---

## 2. Which Parts of Mill's Procedure Are Inferential Today and Should Be Computational

Reading `.mill/skills/using-mill.md` (516 lines):

### 2.1 **Precondition: Orca must be up** — currently inferential, should be computational

The skill says:

> Check and start it before dispatching anything, not after a failure:
> ```bash
> orca status 2>/dev/null | grep -q "runtimeReachable: true" || orca open
> ```

An agent may skip this, forget it, or run it incorrectly. A preflight script
that exits non-zero when Orca is down would be computational and cheap.

### 2.2 **Brief construction** — inferential, and that is correct

The skill provides a template:

```markdown
# [Task Name]

> **Role:** <role-name> | **Model:** <tier>

## Context
...
## Acceptance
...
```

This is judgment work — the coordinator fills in context, acceptance criteria,
and scope based on the task at hand. No program can compute this; it requires
understanding the task. The skill is the right shape for this.

### 2.3 **Role selection** — inferential, and that is correct

The skill provides a decision tree:

```
Is the work product code, or a decision?
├── A decision / document the CTO or user asked for
│   ├── Product decision? → PM
│   ├── Cross-cutting technical decision? → Architect
...
```

This is routing judgment. The skill is the right shape.

### 2.4 **Phase gates** — currently computational (bash), correctly

The skill says:

> Before marking work approved, run the appropriate gate:
> | Gate | Script |
> | `gate-frd` | `checks/gate-frd <issue>` |
> | `gate-spec` | `checks/gate-spec <issue>` |
> ...

Gates are bash scripts in `.mill/checks/`. This is already computational.

### 2.5 **Verification** — inferential, but partially automatable

The skill says:

> 1. Read the report
> 2. Recalculate every quantitative claim
> 3. Run the gates
> 4. Check `allowed_files`
> 5. Check scope

Items 3–5 are computational:
- Gate-running is already bash.
- `allowed_files` checking is a diff stat against a declared pattern — a 10-line bash script.
- Scope checking is harder (requires semantic understanding), but "files outside the brief were touched" is a diff operation.

A `mill-verify` script that runs gates, checks `allowed_files`, and flags scope
violations would remove inferential load from the coordinator without requiring
judgment.

### 2.6 **Model tier selection** — inferential, and broken

The skill says:

> Select the tier by choosing the agent, not by passing a model flag.
> | Role | Tier | Dispatch with |
> | PM, Architect, Tech Lead, Reviewer | thinks | `--agent claude --model <id>` |
> | Sr Dev (BE/FE/Data), QA/Docs | writes | `--agent command-code` |

FINDINGS #116 showed this never worked — every role dispatched on the cheapest
model. The skill documents workarounds ("the per-model route is closed until
Orca can pass arguments through"), but the economic premise is still broken.

A preflight check that warns "dispatching to thinks tier but no model override
available" would at least surface the gap. Whether the gap can be closed depends
on Orca, not Mill.

### 2.7 Summary: what should move from skill to script

| Item | Current | Recommendation |
|---|---|---|
| Orca-up preflight | Skill prose | Bash script, exit 1 if down |
| `allowed_files` check | Skill prose ("check git diff --stat") | Bash script, exit 1 if violated |
| Model tier preflight | Skill prose (documents a broken path) | Bash warning, or surface the gap visibly |
| Gate invocation | Already bash | Keep |
| Brief construction | Skill prose | Keep (judgment work) |
| Role selection | Skill prose | Keep (routing judgment) |
| Verification recalculation | Skill prose | Partial: automate the diff-based checks |

---

## 3. What Comparable Tools Do

### 3.1 superpowers (obra/superpowers) — skills only

**Source:** https://github.com/obra/superpowers

superpowers is pure Markdown:
- `/skills/` contains 15 skill directories, each a `.md` file
- `CLAUDE.md` bootstraps by declaring the skill-loading behavior
- No CLI for core functionality (only packaging scripts for plugin distribution)
- Installation is `plugin install superpowers@<marketplace>` — the harness does the work

superpowers succeeds as skills-only because it has no derived artifacts. The
skill files ARE the policy. Agents read them directly.

**What it gives up:**
- No deterministic enforcement of skill invocation. The bootstrap says "you MUST
  invoke the skill" but an agent can rationalize its way out.
- No project detection. superpowers works identically in any repo.
- No multi-agent coordination. superpowers assumes one agent reading skills.

### 3.2 spec-kit (github/spec-kit) — CLI plus templates

**Source:** https://github.com/github/spec-kit

spec-kit keeps a Python CLI (~54,000 lines in `src/specify_cli/`) because:
1. Templates are resolved across 4 priority layers with composition strategies
2. Commands are written to agent-specific directories in agent-specific formats
3. Extensions and presets are downloaded, verified, and tracked in manifests
4. Project state (`init-options.json`) drives conditional behavior

**What it gives up:**
- Agents cannot modify the templates without running the CLI
- A new agent format requires code changes (though the registrar is designed to be extended)
- Debugging requires understanding both the CLI and the generated output

### 3.3 Orca (stablyai/orca) — application, not toolkit

Orca is a runtime, not a methodology. It provides:
- Process spawning and supervision
- Worktree isolation
- Message bus for coordinator/worker communication
- Task tracking

Orca does not provide:
- Roles, phases, or organizational decomposition
- Brief templates or acceptance criteria patterns
- Phase gates or verification checklists

Mill delegates substrate operations to Orca (ADR 0005) and keeps the methodology.
This split is sound.

### 3.4 Where each draws the line

| Tool | What is code | What is text |
|---|---|---|
| superpowers | Nothing (packaging only) | Everything — skills, bootstrap, methodology |
| spec-kit | Template resolution, format conversion, path rewriting, extension management | Command bodies, workflow descriptions, project configuration |
| Orca | Process spawn, supervision, message bus, worktree lifecycle | None (it's infrastructure, not methodology) |
| Mill (current) | Phase gates (bash) | Everything else — roles, sequence, brief templates, verification |

---

## 4. What a Minimal Mill Program Would Be

Not a return to 8,000 lines of Go. A shell script.

### 4.1 `mill-preflight`

```bash
#!/usr/bin/env bash
# Exit non-zero if Mill cannot dispatch.
# Called by the coordinator before any dispatch.

# 1. Orca must be reachable
if ! orca status 2>/dev/null | grep -q "runtimeReachable: true"; then
  echo "error: Orca is not running. Run 'orca open' first." >&2
  exit 1
fi

# 2. Must be in a Mill project
if [[ ! -d .mill/roles ]]; then
  echo "error: Not a Mill project (no .mill/roles/)." >&2
  exit 1
fi

exit 0
```

~15 lines. Deterministic. Cheap.

### 4.2 `mill-verify <worktree> <role>`

```bash
#!/usr/bin/env bash
# Check that a worker's output respects its role constraints.
# Called by the coordinator after worker_done.

worktree=$1
role=$2
role_file=".mill/roles/$role/ROLE.md"

# Extract allowed_files from role frontmatter
allowed=$(grep -A20 '^---' "$role_file" | grep 'allowed_files:' -A10 | \
  grep '^\s*-' | sed 's/.*- //')

# Check git diff --stat against allowed patterns
cd "$worktree" || exit 1
changed=$(git diff --stat HEAD~1 --name-only)

for file in $changed; do
  matched=0
  for pattern in $allowed; do
    if [[ "$file" == $pattern ]]; then matched=1; break; fi
  done
  if [[ $matched -eq 0 ]]; then
    echo "error: $file not in allowed_files for $role" >&2
    exit 1
  fi
done

exit 0
```

~30 lines. Deterministic. Catches what FINDINGS #137 missed.

### 4.3 What stays in the skill

- Brief construction (judgment)
- Role selection (routing judgment)
- Dispatch procedure (Orca CLI invocation — already documented as shell commands)
- Question handling (judgment)
- Verification recalculation (judgment, but calls the scripts above)
- Phase sequencing (judgment)

### 4.4 Total size

Two scripts, ~50 lines combined. Not a binary. Not a framework. Just the
computational minimum that must not be inferential.

---

## 5. Does ADR 0006 Survive?

**Yes. ADR 0006 stands, with one addendum.**

### 5.1 What ADR 0006 got right

> Examining what actually survives, in the language it is actually written in:
>
> | What Mill keeps | Written in |
> | the eleven roles | Markdown |
> | the phase sequence | Markdown |
> | acceptance criteria | Markdown |
> | role-enforce | bash |
> | the phase gates | bash |
> | brief construction | text |
> | model tier selection | a lookup table |
>
> None of it is Go.

This remains true. The methodology is Markdown. The enforcement is bash.
The Go binary transported policy; it did not execute it.

### 5.2 What ADR 0006 understated

> Lost: Enforcement of the procedure itself. A binary executes the dispatch
> sequence; a skill instructs an agent to.

The harness research (docs/research/harness-engineering-and-evals.md §5.5)
called this out:

> This is true but misleading. The binary *could have been fixed* to enforce
> the sequence; a skill *cannot enforce* it.

ADR 0006 is correct that the Go binary *was not* enforcing the sequence. But
the loss of enforcement capability is real, and the decision understates it.

### 5.3 Why Mill is different from spec-kit

spec-kit needs code because it **generates derived artifacts**:
- Templates → composed templates
- Command sources → agent-specific command files
- Archives → installed extensions

Mill does not generate derived artifacts. Mill's policy IS the artifact:
- Role definitions are read directly
- The skill is read directly
- Gates are invoked directly

The reasoning that makes spec-kit keep a CLI is the same reasoning that lets
Mill drop one.

### 5.4 The addendum

ADR 0006 should be supplemented with ADR 0007: **Mill adds preflight and
verification scripts**.

- `mill-preflight` (computational): Orca up, project detected, tier warning
- `mill-verify` (computational): allowed_files respected, scope respected
- The skill (inferential): judgment work that cannot be computed

This preserves ADR 0006's core claim ("Mill is a skill, not a binary") while
addressing the field's distinction between computational and inferential
controls.

---

## Sources

### Primary sources (spec-kit)

All paths relative to `github/spec-kit`:

1. `src/specify_cli/presets/__init__.py:129–160` — `_materialize_constitution_template`, template composition
2. `src/specify_cli/agents.py:277–436` — command rendering (Markdown, TOML, YAML, SKILL.md)
3. `src/specify_cli/agents.py:197–229` — path rewriting
4. `src/specify_cli/__init__.py:532–553` — project detection
5. `src/specify_cli/shared_infra.py:42–99` — SHA-256 verification
6. `pyproject.toml` — CLI entry point (`specify = "specify_cli:main"`)

### Primary sources (superpowers)

All paths relative to `obra/superpowers`:

7. `skills/using-superpowers/SKILL.md` — bootstrap behavior
8. `CLAUDE.md` — contributor guidelines, no CLI for core functionality

### Mill's own evidence

9. `docs/adr/0006-mill-is-a-skill-not-a-binary.md` — the decision under question
10. `.mill/skills/using-mill.md` — the current skill (516 lines)
11. `docs/research/harness-engineering-and-evals.md` — field research on computational vs inferential controls

### Comparable tools (by URL)

12. https://github.com/github/spec-kit — Python CLI, ~54k lines
13. https://github.com/obra/superpowers — skills only, no core CLI
