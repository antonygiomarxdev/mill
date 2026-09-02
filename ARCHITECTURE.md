# Mill Architecture

Mill is a skill plus a policy directory: role definitions in Markdown, gate
scripts in bash, no binary. Orca provides the execution substrate. The
coordinator is the Product Engineer, the single head of a star topology.

## Design goals

1. **Skill on substrate.** Mill carries the policy — roles, the phase sequence,
   brief construction, the dispatch procedure — as Markdown and bash. Orca
   spawns and supervises the workers.
2. **Role-driven.** Twelve role definitions under `.mill/roles/`, each
   declaring what it produces and the file categories it may write.
3. **Single coordinator.** One head dispatches, receives, and decides the next
   step. No worker dispatches another.
4. **Verified at the dispatch boundary.** Verification is one command,
   `.mill/checks/mill-verify`, run against a worker's worktree after it
   reports done.

## Entry file

`AGENTS.md` (symlinked as `CLAUDE.md`) is read at session start. The
coordinator identity is established when the `delegate` skill is invoked, not
re-injected on every prompt: a session that has not invoked the skill has no
role context.

## Star topology

The coordinator is a hub: it dispatches to a role, receives the result, decides
the next step, and dispatches again — one-to-N, not one-to-one-to-one.

```
              ┌─ PM
              ├─ Architect
coordinator ──┼─ Tech Lead
              ├─ Sr Dev
              ├─ Reviewer
              └─ QA/Docs, UX, UI
```

The organisational sequence is preserved. No role other than the coordinator
knows who comes next; PM is a worker role like the others.

## Mill / Orca boundary

**Mill owns** (policy — Markdown and bash):

- the role definitions and their capabilities, under `.mill/roles/`
- the phase sequence: FRD → spec → tasks → implementation → review
- `.mill/checks/role-enforce`: what each role may write, categories resolved
  to file patterns through `.mill/role-capabilities`
- brief construction from role definition, issue, and upstream artifact
- model tier selection per dispatch

**Orca owns** (execution substrate):

- spawning and supervising workers
- worktree lifecycle
- the message bus and the raise-a-hand cycle
- parallelism and its limits

## Where policy lives

- `.mill/roles/` — role definitions (Markdown + YAML frontmatter)
- `.mill/checks/` — gate and dispatch scripts: common.sh, mill-dispatch,
  mill-preflight, mill-verify, pre-commit, pre-push, role-enforce
- `.mill/gauntlet` — per-project build, lint, test commands
- `.mill/role-capabilities` — category → file-pattern map

`.mill/gauntlet`, `.mill/role-capabilities`, and the agent catalog are
per-project; each ships a tracked example template beside it.

## Enforcement

Enforcement happens at the dispatch boundary, not per prompt or per write:

- `.mill/checks/role-enforce` resolves each changed file to a category through
  `.mill/role-capabilities` and refuses files outside the dispatched role's
  allowed set.
- `.mill/checks/mill-verify` runs the gauntlet from `.mill/gauntlet`, invokes
  `role-enforce` over the change set, and requires the work to be committed. It
  is a command the coordinator runs.

Mill installs no git hooks.

## Phased workflow

Each phase produces an artifact, and progress is enforced at the dispatch
boundary:

```
PM ─ FRD ─→ Architect ─ spec ─→ Tech Lead ─ tasks ─→ Sr Dev ─ commits ─→ Reviewer ─ review
```

The coordinator verifies each worker's worktree with `.mill/checks/mill-verify`
before dispatching the next role.

## How dispatch works

1. Classify the work and select the role.
2. Build a brief from the role definition, the issue, and the upstream
   artifact.
3. Dispatch via `.mill/checks/mill-dispatch`: preflight, create the task, start
   the worker, wait, report, release.
4. Read the worker's report from the mailbox.
5. Verify the worktree with `.mill/checks/mill-verify`.
6. Advance the phase, escalate, or re-delegate.

## Model tiers

Each role declares a tier in its definition:

- pro tier directly: Product Engineer, PM, Architect, Tech Lead, Reviewer, UX,
  UI, Policy Author
- free first, escalated to paid when the free attempt fails on judgement:
  Sr Dev, QA/Docs
