# Skills

Local snapshot of agent skills. Each `.md` file is a self-contained skill that an agent loads as context via `read skills/<name>.md`.

## How it works

1. **`skills.json`** — manifest mapping skill names to source URLs, hashes, and versions.
2. **`.md` files** — local copies. Always available, even offline.
3. **`mill sync-skills`** — checks each source URL for updates (HEAD request → compare hash). Downloads if changed. Skips if offline.

## Why local copies

An agent with a role file that says `skill: wayfinder` must be able to load that skill without network access. The local snapshot guarantees availability. The sync step keeps it fresh.

## Adding a skill

1. Add entry to `skills.json` with source URL
2. Run `mill sync-skills` to download
3. Declare the skill in a role's frontmatter `skills:` list

## Source

Skills originate from [Matt Pocock Skills](https://github.com/antonygiomarxdev/mattpocock-skills). The `source` field in `skills.json` points to the raw SKILL.md URL for each skill.
