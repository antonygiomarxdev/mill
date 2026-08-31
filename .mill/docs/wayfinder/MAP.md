## Destination

**Mill**: open-source multi-agent delegation harness using the
orchestrator-worker pattern. A coordinator (the Product Engineer) coordinates
with a human and delegates to specialized role workers running in isolated
worktrees, writing artifacts to the filesystem, verified at the dispatch
boundary.

Mill provides: role definitions as context files loadable by any model, the
dispatch procedure (`.claude/skills/delegate/SKILL.md`), bash gates that enforce
what each role may write, and per-project gauntlet configuration.

Everything tracked in GitHub Issues + Projects, progress in comments. Code,
docs, commits, issues in English. Any team assembles their agent org with
their own roles.

## Notes

- **Skills**: wayfinder, grilling, domain-modeling, brainstorming, tdd,
  writing-plans, executing-plans, verification-before-completion,
  systematic-debugging, codebase-design, code-review, using-git-worktrees,
  using-superpowers, caveman (for token efficiency)
- **Stack**: role files in Markdown; gate scripts in bash
- **Language**: English for code, docs, commits, issues. Spanish for
  human-facing issue comments OK.
- **Repo**: https://github.com/antonygiomarxdev/mill
- **The coordinator (Product Engineer) is the expensive model.** Subagents use
  cheap models.

## Decisions so far

<!-- closed tickets go here, one line per resolution -->

## Not yet specified

- GitHub integration details

## Out of scope

- RUMAI-specific agent org (mill is open-source, RUMAI is one consumer)
- Web UI / dashboard
- Hosted/SaaS version
