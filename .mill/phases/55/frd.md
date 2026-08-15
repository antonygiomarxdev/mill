# FRD: Read issue body and acceptance criteria for delegation prompts

## User need

When `mill delegate` spawns a subagent, the subagent currently receives a generic prompt that does not include the issue body or acceptance criteria. This forces the subagent to look up the issue itself — wasting context, risking misinterpretation, and breaking the delegation contract where the delegating role should provide complete context.

The delegating role already has the issue body. It must be forwarded to the subagent as part of the delegation prompt so that the subagent knows exactly what to build without hunting for it.

## Functional requirements

1. **Issue body included in delegation prompt.** When `mill delegate` spawns a subagent for an issue, the delegation prompt MUST include the full issue body (title + description + acceptance criteria). The subagent receives this as part of its initial context, not as a reference to look up.

2. **Acceptance criteria are extracted and highlighted.** Within the delegation prompt, acceptance criteria are extracted from the issue body and presented as a structured checklist. If the issue uses `- [ ]` markdown checkboxes, they are preserved. If criteria are in plain text, they are formatted as a numbered list.

3. **Spec document is linked when available.** If `.mill/phases/<issue>/frd.md` or `.mill/phases/<issue>/spec.md` exists, the delegation prompt includes a reference to that file path. The subagent must read it; the content is not inlined to keep the prompt compact.

4. **Delegation prompt format.** The prompt follows a fixed structure: (a) issue title, (b) issue body, (c) extracted acceptance criteria, (d) spec document reference when available, (e) role-specific instructions from the delegating role's ROLE.md. No other boilerplate.

5. **Fallback for missing issues.** If `gh issue view <N>` fails (issue deleted, no network, no `gh` CLI), delegation proceeds with a warning: "Issue #N could not be read (reason). Proceeding with title only." The subagent is not blocked by a connectivity failure.

## Out of scope

- Fetching linked issues (parent/child relationships). Only the target issue's body is included.
- Reading PR bodies or commit messages. This is issue-driven only.
- Caching issue bodies. Every delegation reads the issue fresh from GitHub.
- Rich formatting of issue body (HTML → markdown conversion). The body is included as-is from the GitHub API.

## Priority

**P1** — quality of life. Subagents can work around missing context by looking up the issue themselves, but it wastes tokens and introduces inconsistency. Fixing this makes every delegation more reliable and cheaper.
