# MEMORY

Current operational facts. If a fact stops being true, edit the line. History lives in `LESSONS.md`.

## What Mill is now

- `AGENTS.md` — entry point, 28 lines.
- `CLAUDE.md` — symlink to `AGENTS.md` (mode 120000).
- `.claude/skills/delegate/SKILL.md` — the dispatch recipe (98 lines).
- `LESSONS.md` — history; not a checklist.
- `.mill/roles/*/ROLE.md` — one per worker role; brief reads its target role.
- `.mill/checks/` — nine scripts: `common.sh`, `mill-install`, `mill-preflight`, `mill-role-guard`, `mill-uninstall`, `mill-verify`, `pre-commit`, `pre-push`, `role-enforce`.
- No Mill binary (ADR 0006).

## Coordinator is constant

- The Product Engineer is the only role that talks to the CTO.
- Identity is injected by the `UserPromptSubmit` hook in `.claude/settings.json` running `.mill/checks/mill-role-guard --context`. Survives compaction.
- The `PreToolUse` guard matcher is `Write|Edit|NotebookEdit` and **not** `Bash`. Heredocs and `sed -i` go through unblocked — deliberate, because Bash is needed for verification.

## Verification

- Run `.mill/checks/mill-verify` from the coordinator's repository, never from the worktree. The worktree has no `.mill/role-capabilities`; that config must not live in a worker's output.
- `--files-modified` accepts git status letters (`A`/`M`/`D`/`R`); a `D` is skipped — a deletion is not a write.

## Gauntlet is not configured here

- No `.mill/gauntlet` in this repository. `build` / `lint` / `test` are no-ops and `mill-verify` enforces file permissions only. A PASS here checked permissions, not behaviour.
- `.mill/gauntlet.example` is the template.

## Dispatch traps, measured 2026-08-30

- `--agent command-code` rejects `--model`. The model comes from `~/.commandcode/config.json`, which is global and which command-code rewrites itself, so two tiers cannot run concurrently.
- Orca marks a command-code dispatch `failed` with `lastError: agent_prompt_stalled` while the worker is running normally. The prompt did arrive. Read the terminal before believing the verdict: `orca terminal read --terminal <handle> --limit 60`. A live worker shows `esc to interrupt`.
- Tier-to-agent mapping: `.mill/agents` (gitignored; `.mill/agents.example` is the template).

## `scaffold/` is deliberately behind

- `scaffold/.mill/checks/` still ships the seven gates deleted from `.mill/checks/` in commit `6166ada`. Known open decision, not an oversight. Do not "fix" by syncing.
