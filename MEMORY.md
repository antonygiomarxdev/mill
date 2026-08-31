# MEMORY

Current operational facts. If a fact stops being true, edit the line. History lives in `LESSONS.md`.

## What Mill is now

- `AGENTS.md` — entry point, 29 lines.
- `CLAUDE.md` — symlink to `AGENTS.md` (mode 120000).
- `.claude/skills/delegate/SKILL.md` — the dispatch recipe (98 lines).
- `LESSONS.md` — history; not a checklist.
- `.mill/roles/*/ROLE.md` — one per worker role; brief reads its target role.
- `.mill/checks/` — seven scripts: `common.sh`, `mill-preflight`, `mill-role-guard`, `mill-verify`, `pre-commit`, `pre-push`, `role-enforce`.
- No Mill binary (ADR 0006).

## Orca's coordination guide

- The authoritative guide to Orca's orchestration surface ships with the
  installed binary and is loaded with `orca skills get orca-cli` and
  `orca skills get orchestration`. Mill references it and does not restate it.
- Fallback when `skills get` cannot run (this machine): both documented paths
  fail — `orca skills get` launches the app UI and the single-instance lock
  refuses it, and `orca-ide` reports `bad option: --no-sandbox`. The guide was
  recovered from the AppImage at
  `/tmp/.mount_orca*/resources/app.asar.unpacked/out/cli/bundled-skill-guides.js`,
  which exports `BUNDLED_SKILL_GUIDES` with a `markdown` field per guide. This
  is a workaround for a broken path, not the supported way.

## Coordinator is constant

- The Product Engineer is the only role that talks to the CTO.
- Identity is injected by the `UserPromptSubmit` hook in `.claude/settings.json` running `.mill/checks/mill-role-guard --context`. Survives compaction.
- The `PreToolUse` guard matcher is `Write|Edit|NotebookEdit` and **not** `Bash`. Heredocs and `sed -i` go through unblocked — deliberate, because Bash is needed for verification.

## Verification

- Run `.mill/checks/mill-verify` from the coordinator's repository, never from the worktree. The worktree has no `.mill/role-capabilities`; that config must not live in a worker's output.
- `--files-modified` accepts git status letters (`A`/`M`/`D`/`R`); a `D` is skipped — a deletion is not a write.

## Gauntlet is configured here

- `.mill/gauntlet` sets `lint=` to a one-line `bash -n` over every shebang-script under `.mill/checks/`. `build=` and `test=` are unset (this repo is Markdown and bash — no build, no test suite). A `PASS` here means the scripts parse and the role's `allowed_files` matched the change set. Behaviour is not checked.
- `.mill/role-capabilities` is local (gitignored) and maps role categories to file patterns; `.mill/role-capabilities.example` is the versioned template.

## Dispatch traps, measured 2026-08-30

- `--agent command-code` rejects `--model`. The model comes from `~/.commandcode/config.json`, which is global and which command-code rewrites itself, so two models cannot run concurrently.
- Orca marks a command-code dispatch `failed` with `lastError: agent_prompt_stalled` while the worker is running normally. The prompt did arrive. Read the terminal before believing the verdict: `orca terminal read --terminal <handle> --limit 60`. A live worker shows `esc to interrupt`.
- `.mill/agents` is a catalog of what runs on this machine (gitignored; `.mill/agents.example` is the template). No script reads it; every dispatch names its agent and model.

## Scaffold and installers are gone

- `scaffold/`, the root `checks/`, and `.mill/checks/mill-install` / `mill-uninstall` were deleted. There is no install path into another repository from this tree; installation is being redesigned for multiple harnesses. `mill-preflight` stays — it refuses to dispatch when Orca is unreachable or the working directory is not a Mill project.

## omp prewalk and dispatch model recovery

- This machine's omp is configured with `prewalk` and a `smol` model:
  `modelRoles` names three roles and `prewalk.enabled` / `task.prewalk` are
  `true` (see `.mill/agents.example`).
- A dispatch's models are recoverable after the worker settles from the session
  transcript at `~/.omp/agent/sessions/<worktree-slug>/*.jsonl`:
  `grep -oE '"model(Id)?":"[^"]+"' <session>.jsonl | sed 's/.*://;s/"//g' | sort -u`.
  This transcript is the source for the `Mill-Dispatch` trailer's `model=` field.

## Rejections log (read by 2026-10-01)

- `.mill/rejections.log` records every role rejection as one line — timestamp (ISO-8601 UTC), source (`preflight`|`verify`), role, path.
- It answers: does `mill-verify --role` ever reject a path that `mill-preflight --brief-file` did not catch first?
- Read by 2026-10-01: if every `verify` line has a matching `preflight` line for the same path, retire output-side role enforcement; if not, keep it.
