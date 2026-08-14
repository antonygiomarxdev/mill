# ADR 0006: Mill is a skill, not a binary

**Status:** Accepted
**Date:** 2026-08-14
**Decided by:** CTO
**Supersedes parts of:** ADR 0005 and `.mill/phases/162/spec.md`, which assumed a
reduced Go CLI calling Orca

## Context

ADR 0005 made Orca the execution substrate and left Mill owning the policy layer:
role definitions, the phase sequence, capability enforcement, phase gates, brief
construction, and model tier selection.

The purge plan written against that decision (`.mill/phases/162/spec.md`) deletes
seven packages and reduces most of `internal/cli`, leaving a small Go binary that
calls Orca through a narrow module.

Examining what actually survives, in the language it is actually written in:

| What Mill keeps | Written in |
|---|---|
| the eleven roles | Markdown |
| the phase sequence, FRD → spec → tasks | Markdown |
| acceptance criteria | Markdown |
| `role-enforce` | bash |
| the phase gates | bash |
| brief construction | text |
| model tier selection | a lookup table |

None of it is Go. The packages the purge plan keeps as code — `routing_56.go`,
`costs_56.go`, `init.go` — are a lookup, a counter, and a file copier. The binary
is not executing the policy; it is transporting it.

The evidence is direct: the work that succeeded today did not use Mill. Coverage
in `internal/ledger` went from 77.1% to 94.3% via a 1 KB brief,
`orca orchestration task-create`, `worker-start`, and the gates in bash. The
binary took no part. What Mill contributed was the criteria, and criteria are text.

## Decision

**Mill becomes a skill plus a policy directory.** The Go CLI is retired.

- **The skill** carries the roles, the phase sequence, brief construction, and the
  dispatch procedure against `orca orchestration`.
- **`.mill/`** keeps what is already bash or Markdown: `roles/`, `checks/`, `docs/`.
- **`mill init`** becomes installation of the skill and its policy directory.
- Enforcement stays where it already is: git hooks running `role-enforce` and the
  phase gates. Those are bash and require no binary.

## Alternatives considered

**A reduced Go CLI over Orca**, per `.mill/phases/162/spec.md`. Rejected: the
surviving code transports policy rather than enforcing it, and every line of it is
a line to maintain against a third party's CLI surface.

**Keep the binary for enforcement.** Rejected on inspection: enforcement is
already bash invoked by git hooks. It never needed the binary, which is why it
kept working today while the binary was stale.

## Consequences

**Gained.** The thing that changes most often — roles, sequence, criteria — becomes
the thing that is easiest to change. No compile step between deciding a rule and
applying it. #152 (the gauntlet being hardcoded Go) stops being a problem to solve:
a skill ships no Go gauntlet. Adoption drops to installing a skill.

**Lost.**

1. **Unattended execution.** A skill needs an agent to read it. Mill can no longer
   run from cron or CI without one. This was the deciding question and the CTO's
   answer is that Mill always starts from a session.
2. **Enforcement of the procedure itself.** A binary executes the dispatch
   sequence; a skill instructs an agent to. Git hooks still enforce *what may be
   committed* — the part that matters most — but nothing forces the coordinator to
   follow the sequence. Today's evidence cuts both ways: the roles were never told
   to hand off (#153), so the binary was not enforcing it either.
3. **The Go test suite**, currently the only automated check of any of this,
   largely disappears with the code it tests. The gates and hooks must carry more
   of that weight, and they are less covered.

## Notes

This decision follows the same shape as ADR 0005 and for the same reason: what was
built duplicated something that already existed, and the distinctive value sat in
the layer above it. There the substrate was Orca's; here the language is Markdown's.
