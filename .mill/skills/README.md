# Skills

Local snapshot of agent skills. Each `.md` file is a self-contained skill that
an agent loads as context via `read skills/<name>.md`.

## What is here

- `using-mill.md` — the coordinator's procedure; the skill this project is
  (ADR 0006). Referenced from `.mill/roles/COMMON.md`, `AGENTS.md`, and the
  harness entry points.
- `wayfinder.md` — a synced skill used by the Product Engineer and PM roles.
- `skills.json` — the manifest mapping skill names to source URLs, hashes, and
  versions.

## Why local copies

An agent with a role file that declares a skill must be able to load it without
network access. The local snapshot guarantees availability.

## Adding a skill

1. Add entry to `skills.json` with source URL
2. Download the skill content into this directory
3. Declare the skill in a role's frontmatter `skills:` list
