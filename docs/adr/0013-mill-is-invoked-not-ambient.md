# ADR 0013: Mill is invoked, not ambient

**Status:** Accepted
**Date:** 2026-09-01
**Decided by:** Architect
**Related:** #192. Supersedes the per-prompt-identity criterion of ADR 0012.

## Context

ADR 0012 distributed Mill as a harness-native extension, and that extension
ships two interceptors into every project that adopts it, registered through
the `hooks` key in `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`
and `.cursor-plugin/plugin.json`:

- **`UserPromptSubmit`** (and its Codex/Cursor equivalents) reads every message
  the user types in that repository, Mill-related or not. Its only output is one
  identity line kept in context, and no outcome is traced to it.
- **`PreToolUse`** matches `Write|Edit|NotebookEdit` and has fired **zero times,
  ever** — agents write through `Bash`, which the matcher excludes.

A tool that reads every prompt in someone else's repository has to earn that
position. `PreToolUse` does not, and `UserPromptSubmit` is redundant: the
mechanism it substitutes for already exists and already works. The `delegate`
skill's own `description` frontmatter is what makes an agent invoke it from
context, and it fires today with no hook installed. The hook was added on top
of a mechanism that was not broken (#192).

## Decision

**Mill is invoked — by the user, or by the agent from the skill's own
description — and registers no hooks.** The `hooks` key is removed from all
three harness manifests, and `.claude/settings.json` and
`hooks/hooks-cursor.json`, which exist only to register the hooks, are deleted.
The gate script the hooks called (`.mill/checks/mill-role-guard`) is not removed
here; its policy is re-homed by the policy-author in a separate change.

## Alternatives considered

- **spec-kit (the compared prior art):** spec-kit solves the same problem for
  the same audience and is invoked, not ambient. Its CLI installs outside the
  repository (`uv tool install specify-cli`); `specify init` writes `.specify/`,
  `.claude/commands/` and `specs/`; and it registers **no hooks at all**. Nothing
  intercepts the session — the user types `/specify` and the rest of the time it
  is inert. Chosen over an ambient interceptor.
- **An ambient interceptor (status quo):** the `UserPromptSubmit` hook ADR 0012
  shipped. Rejected: it reads every prompt in a repository it was not asked to
  read, its only output is untraced, and it duplicates the skill description,
  which already fires.

## Consequences

### Positive
- A Mill install adds no file that runs without the user or the agent invoking
  Mill. The prompt-stream interceptor is removed by construction.
- One invocation path instead of two: the skill description is the single
  mechanism that fires, and it is exercised every session today.

### Negative
- **Mill no longer holds role state ambiently across a session.** It
  re-establishes context at each invocation, from the skill's description,
  rather than having a hook inject the coordinator's identity on every prompt.
- A session that does not invoke `delegate` has no role context until it does.

### Mitigations
- The coordinator-identity line moves into the `delegate` skill — the one place
  that re-establishes context on every invocation (#192).

## References

- #192 — measurements: `PreToolUse` fires zero times; `UserPromptSubmit` is
  redundant with the skill description.
- ADR 0012 — the distribution mechanism this ADR amends.
- spec-kit — prior art for "invoked, not ambient".
