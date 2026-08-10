## Destination

**Mill**: open-source multi-agent delegation harness using the orchestrator-worker pattern. A lead agent (Staff, expensive model) coordinates with a human and delegates to specialized subagents (cheap models) running in isolated worktrees, writing artifacts to the filesystem, verified by final result — not exit code.

Mill provides: role definitions as context files loadable by any model, a Go runner that spawns subagents and deterministically repairs tool calls, filesystem-based artifact passing, a decision/correction ledger for continuous learning, and checkpoints for long-running sessions.

Built with TDD + SDD. Matt Pocock skills as the agent capability system. Everything tracked in GitHub Issues + Projects, progress in comments. Code, docs, commits, issues in English. Staff defined first, then autoconstructs the rest.

Compatible with: opencode, commandcode. Any team assembles their agent org with their own roles.

## Notes

- **Skills**: wayfinder, grilling, domain-modeling, brainstorming, tdd, writing-plans, executing-plans, verification-before-completion, systematic-debugging, codebase-design, code-review, using-git-worktrees, using-superpowers, caveman (for token efficiency)
- **Stack**: Go for the runner; role files in Markdown; ledger in JSONL
- **Language**: English for code, docs, commits, issues. Spanish for human-facing issue comments OK.
- **Repo**: https://github.com/antonygiomarxdev/mill
- **Staff is the expensive model** (Claude Opus / deepseek-v4-pro). Subagents use cheap models (deepseek-v4-flash / laguna-free). Caro reviews, barato executes.

## Decisions so far

<!-- closed tickets go here, one line per resolution -->

## Not yet specified

- Runner design details (depends on Staff contract + adapter research)
- Ledger format specifics (depends on what Staff needs to record)
- Verification system mechanics (depends on Staff contract)
- Skills-to-role binding (depends on role format)
- GitHub integration details (depends on runner design)
- TDD/SDD infra setup (depends on directory structure)
- Project Manager role definition (PM + Staff + CTO collaborate to give light to the project)
- Autoconstruction bootstrap strategy (Staff builds remaining roles and infra)

## Out of scope

- RUMAI-specific agent org (mill is open-source, RUMAI is one consumer)
- Web UI / dashboard
- Hosted/SaaS version
