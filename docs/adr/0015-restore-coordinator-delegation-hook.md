# ADR 0015: Restore the coordinator delegation hook in Mill's own repository

**Status:** Accepted
**Date:** 2026-09-04
**Decided by:** CTO
**Related:** #192, #148. Narrows ADR 0013; does not supersede it.

## Context

### The origin of the error

ADR 0013 removed Mill's `UserPromptSubmit` hook from the product manifests.
The CTO has since supplied the root cause of that decision, and it is not the one
its text records. In the CTO's words:

> "hubo una confusion con los hooks de git y los de Claude, yo no queria que se
> tocaran los hooks de git, porque lo que no debemos es reventar el proyecto de
> alguien"

Two different things were conflated. Git hooks and git config can break someone's
repository — #148 is the evidence: an agent set `core.bare=true` and broke a repo.
The constraint handed down was only ever about that first thing. The harness
`UserPromptSubmit` hook cannot break a repository in that way; it reads prompts
and injects text. It was removed as collateral, not on its own merits.

A decision built on a misread constraint is a different failure from a decision
whose reasoning aged badly. This ADR records the misread first, because what
survives ADR 0013 depends on which of its conclusions were drawn from the
git-hook confusion and which stand independently.

### What survives ADR 0013

Two of ADR 0013's conclusions survive this correction and are stated here as
surviving, not overturned:

1. **Removing `PreToolUse` was right on its own evidence.** It fired zero times,
   ever, because agents write through `Bash`, which its matcher excluded. That
   finding was never about git hooks; it stands. Do not fix it, do not widen it,
   do not add it back.
2. **"A tool that reads every prompt in someone else's repository has to earn
   that position" stands on its own merits.** Independent of any git-hook
   confusion, the objection to ambient interception in a foreign repository is a
   sound one. It is now the *only* remaining objection to shipping this hook to
   installs, and it is a product question for the CTO — explicitly not answered
   by this ADR.

### The measurement

The specific claim this ADR narrows is the redundancy claim, quoted from ADR
0013:

> The mechanism it substitutes for already exists and already works. The
> `delegate` skill's own `description` frontmatter is what makes an agent invoke
> it from context, and it fires today with no hook installed.

That claim is now falsified by measurement. In the session that wrote ADR 0013,
the coordinator violated both halves of what was already written down:

- `.mill/roles/product-engineer/ROLE.md` says the coordinator delegates. The
  coordinator edited `.mill/role-capabilities` itself, wrote memory files, and
  hand-corrected briefs.
- `.claude/skills/delegate/SKILL.md` section 3 now says a supervisor runs as an
  Orca terminal. The coordinator had already launched two supervisors as
  background jobs of its own shell before writing that sentence; one of them
  died, leaving a live worker nobody released.

The skill was loaded and listed. The rule was broken anyway, twice, by the agent
that wrote it. A document read once does not survive a forty-turn session; the
agent forgets what it wrote.

What did hold, in the same session, across roughly forty turns with no drift:
two behavioural rulesets injected by a `UserPromptSubmit` hook on every single
prompt. Same session, same agent, same context pressure. The difference is not
the wording. It is that one is re-stated every turn and the other is read once.

## Decision

**Restore a `UserPromptSubmit` hook in Mill's own repository only.** The hook
fires once per prompt in the session that coordinates Mill and injects two
lines, in this order — what the coordinator is, then how it dispatches:

1. **It delegates.** "You are the Mill coordinator: you produce briefs and
   verification, not implementation. Writing here is the expensive work being
   delegated away; you do not do it yourself."
2. **How a dispatch runs.** "Dispatch with `.mill/checks/mill-dispatch`, hosted
   in an Orca terminal created with `orca terminal create ... --command "orca
   orchestration run-use --id <run_id> && .mill/checks/mill-dispatch ..."`;
   never a hand-rolled terminal that sends text and waits for a matching string,
   and never a background job of your own shell — both end with a dead terminal
   and a worker nobody releases."

The hook lives in `.claude/settings.json` and reads the prompts of whatever
project installs Mill on a harness whose hook support is verified — today, that
is Claude Code only (see "Per-harness hook support" below). The two injected
lines live in one place — `.mill/IDENTITY.md` — and the Claude harness config
cats that file rather than embedding the text, so the wording cannot drift
between installs:

- `.claude-plugin/plugin.json` registers `"hooks": ".claude/settings.json"`,
  whose `UserPromptSubmit` command is `cat .mill/IDENTITY.md 2>/dev/null || true`.

The original commit also pointed `.codex-plugin` at `./.claude/settings.json`
and `.cursor-plugin` at `./hooks/hooks-cursor.json`, but neither shape was ever
shown to work — `.codex-plugin` was a Claude-schema file handed to a different
harness. Both are unverified, so neither is restored; hook support for those
harnesses is blocked on verification, not assumed.

### How the surviving objection is answered

The one surviving objection — "a tool that reads every prompt in someone else's
repository has to earn that position" — is independent of the git-hook
confusion and stands on its own merits. It is now answered, not overruled, by
the silence-outside-Mill mitigation: the hook command is
`cat .mill/IDENTITY.md 2>/dev/null || true`. In a repository that is not a Mill
project, `.mill/IDENTITY.md` is absent, `cat` fails, stderr is discarded, and
`|| true` exits clean — the hook emits **nothing**. It reads every prompt in a
foreign repository and produces no output there. It earns its position by being
silent when it has no business speaking. On that mitigation, the CTO decided to
ship.

### What this ADR does not change

`PreToolUse` stays removed — zero fires, agents write through `Bash`. Git hooks
and git config stay untouched — the constraint that started ADR 0013 (#148) is
absolute and is not relaxed by this ADR. #192 stays addressed. Hook support for
non-Claude harnesses stays unverified until observed.

## Alternatives considered

- **Rely on the skill description alone (ADR 0013's redundancy claim):**
  rejected. The skill was loaded and the rule was broken anyway, twice, by the
  agent that wrote it. A document read once does not survive forty turns of
  context pressure.
- **A `PreToolUse` hook:** rejected and explicitly out of scope. ADR 0013
  recorded that it fired zero times ever, because agents write through `Bash`,
  which its matcher excluded. That finding survives this ADR unchanged. Do not
  fix it, do not widen it, do not add it.
- **Ship the hook to installs:** the one surviving objection — earning the
  right to read every prompt in a foreign repository — was answered by the
  silence-outside-Mill mitigation (`cat ... || true` emits nothing where the
  file is absent). On that answer, the CTO decided to ship. See below.

## Consequences

### Positive
- The coordinator's identity and dispatch mechanism are re-stated on every
  prompt, in the session that coordinates Mill. A rule read once is replaced by
  a rule that cannot be forgotten within a session.
- The hook reads only this repository's own prompts. It adds no interceptor to
  any other repository and changes no product manifest.

### Negative
- **Mill's own coordinating session now has a prompt-stream interceptor.** This
  is the exact thing ADR 0013 removed from the product, here restored locally.
  The mitigation is the scope boundary: this repository only, never shipped.
- **The hook is only as fail-safe as its command.** A hook that errors on every
  prompt is worse than no hook. The command is `cat .mill/IDENTITY.md
  2>/dev/null || true`: it emits the two identity lines where the file exists
  and emits nothing where it does not, and it never exits non-zero.

## Decision to ship

The CTO decided to ship the identity hook to projects that install Mill. The
one objection ADR 0013 left standing — "a tool that reads every prompt in someone
else's repository has to earn that position" — is answered by the
silence-outside-Mill mitigation: `cat .mill/IDENTITY.md 2>/dev/null || true`
emits nothing in a repository that is not a Mill project, because the file is not
there. The hook earns its position by being silent when it has no business
speaking. On that answer, shipping is decided.

### Per-harness hook support

ADR 0012's table marks every cell verified, unverified, or fails — never on
faith. ADR 0012 has no "hooks" column, and nothing in this repository shows
that any harness except Claude Code ever executed a prompt hook: ADR 0013 notes
"no outcome is traced to it" for the Claude hook and that `PreToolUse` "fired
zero times, ever". The honest state, in ADR 0012's own format, is one row per
harness and a single new column — hook support:

| Harness | Hook support |
|---|---|
| Claude Code | **verified** — the session running this decision executes `.claude/settings.json` as a UserPromptSubmit hook (`cat .mill/IDENTITY.md`), observed live; hook registered in `.claude-plugin/plugin.json` |
| Codex | **unverified** — manifest exists, skill loads, but no evidence a prompt hook ever ran. Not marked `fails`: nobody looked. Hook registration removed from `.codex-plugin/plugin.json` |
| Cursor | **unverified** — manifest exists, skill loads, but no evidence a prompt hook ever ran. Not marked `fails`: nobody looked. Hook registration removed from `.cursor-plugin/plugin.json` |
| Gemini | **unverified** — `.claude/settings.json` is a Claude-schema file, not shown to be read by Gemini; no evidence a prompt hook ran |
| Devin | **unverified** — no evidence a prompt hook ran |
| Pi | **unverified** — no evidence a prompt hook ran |
| OpenCode | **unverified** — no evidence a prompt hook ran |
| Hermes | **unverified** — no evidence a prompt hook ran |
| Kimi | **unverified** — no evidence a prompt hook ran |
| omp | **unverified** — no manifest read; no evidence a prompt hook ran |

So shipping is **real for Claude Code today** and **blocked on verification
elsewhere**. The Codex/Cursor hook shape was not merely unverified — the
original pointed `.codex-plugin` at `./.claude/settings.json`, a Claude-schema
file handed to a different harness and never shown to work. Restoring it on faith
would be the first unverified claim in a repository whose ADR table marks every
cell verified or fails.

The CTO's decision to ship stands. It is simply not yet executable for
non-Claude harnesses until their hook support is verified the way ADR 0012
verifies everything else — from files and observed runs, not from faith.
`.mill/IDENTITY.md` stays as the single source of truth; it is
harness-agnostic, and is exactly what each harness config will read once its
hook support is verified.

## References
- ADR 0013 — the invoked-not-ambient decision this ADR narrows.
- ADR 0012 — the distribution mechanism ADR 0013 amended.
- #192 — the issue ADR 0013 resolved for installs.
- #148 — an agent set `core.bare=true` and broke a repository; the origin of the
  git-hook constraint ADR 0013 conflated with harness hooks.
- 907b015 — the commit that removed the `hooks` key from the three manifests and
  deleted the settings and cursor-hook files; this ADR restores both.
