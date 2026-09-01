# Mill

Mill is a skill plus a policy directory — role definitions in Markdown, gate
scripts in bash — that turns one AI session into a coordinator dispatching
specialised workers through [Orca](https://onorca.dev).

## How it works

You talk to one session: the coordinator. It reads the work, picks the role for
the next step, builds a brief from that role's definition in `.mill/roles/`,
dispatches a worker through Orca, and verifies what comes back against the
scripts in `.mill/checks/` before deciding the next step.

Orca is the execution substrate: it spawns and supervises workers, isolates
worktrees, and carries the message bus. The topology is a star — the
coordinator dispatches one-to-N and holds the sequence, and no worker
dispatches another. Work flows intent → FRD → spec → tasks → implementation →
review, each phase gated before the next begins.

The scripts document themselves:

```bash
.mill/checks/mill-preflight
.mill/checks/mill-dispatch --help
.mill/checks/mill-verify --help
```

`.mill/checks/mill-preflight` refuses to dispatch when Orca is unreachable or
the directory is not a Mill project. `.mill/checks/mill-dispatch` runs the
dispatch loop end to end. `.mill/checks/mill-verify` runs the build, lint and
test steps from `.mill/gauntlet`, checks every changed file against the
dispatched role's allowances, and requires the work to be committed.

## Installation

Read [INSTALL.md](INSTALL.md) for the full walkthrough. Mill ships as a
harness extension, one manifest per supported harness:

- **Claude Code** — `.claude-plugin/plugin.json`
- **Codex** — `.codex-plugin/plugin.json`
- **Cursor** — `.cursor-plugin/plugin.json`

After the extension is in place, verify the project is dispatchable:

```bash
.mill/checks/mill-preflight
```

A passing run prints:

```text
Orca's coordination guide: load with 'orca skills get orca-cli' and 'orca skills get orchestration'
```

## What Mill does not do

- Mill takes no git hooks: nothing in it runs on commit or push.
- Mill writes nothing outside `.mill/` and the entry files.
- Mill changes no git configuration.

## Project structure

- `.mill/roles/` — role definitions
- `.mill/checks/` — gate and dispatch scripts
- `.mill/gauntlet` — per-project build, lint, test commands
- `.mill/role-capabilities` — per-project file-pattern map
- `docs/PRODUCT.md` — the product definition
- `docs/adr/` — architecture decisions
- `AGENTS.md` — the entry file (symlinked as `CLAUDE.md`)

## Decisions

Decisions that shaped this design are recorded in [docs/adr/](docs/adr/).
