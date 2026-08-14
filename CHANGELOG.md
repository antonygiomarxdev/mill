# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.1.0] - 2026-08-14

Initial release of Mill — the AI agent delegation harness. Mill turns your AI
agent into a Staff Engineer or Product Manager that autonomously classifies work,
delegates to specialized subagents, orchestrates review chains, and persists state.

### Added

- **Runner MVP**: `delegate`, `status`, state persistence (`state.json`),
  append-only ledger, and config loading from `mill.yml`.
- **Clean Architecture**: domain, adapter, state, and ledger layers with
  dependency inversion and interface boundaries.
- **Classification system**: 8 session outcomes for exit-code classification
  (success, blocked, changes requested, etc.).
- **Tool call repair pipeline**: 4 automatic repair patterns for failed tool calls.
- **Project scaffolding**: `mill init` command bundles `.mill/` scaffold with
  roles, skills, checks, and context files into a Go binary via `go:embed`.
- **Adapter system**: OpenCode adapter for AI agent dispatch with model resolution.
- **Land command**: merges worktree branches with gate execution.
- **Role system**: 11 roles (Staff, PM, Architect, Tech Lead, Sr Dev, Reviewer,
  QA/Docs, UX/UI Designers) with delegation chains and capability enforcement.
- **Pre-commit gauntlet**: automatic quality gates on commit (lint, vet, build).
- **Pre-push coverage gate**: project-wide statement coverage ≥90% minimum.
- **Mutation testing gate**: quality enforcement on landing.
- **Phase gates**: mechanical enforcement of FRD → SPEC → TASKS → IMPLEMENT →
  REVIEW → DONE workflow.
- **Budget enforcement**: per-target TimeSeconds, MaxTurns, and TokenBudget limits
  with real-time enforcement in delegate loop.
- **Recursion**: recursive delegation down the chain of command (CTO → Staff →
  Architect → Tech Lead → Sr Dev).
- **Slots**: concurrency control with configurable max-slots (default 4).
- **Watch command**: blocks until all delegated tasks settle.
- **Compact command**: auto-compaction of session context to save tokens.
- **Clean command**: remove completed/failed worktrees with `--all` factory reset.
- **Multi-harness support**: omp, claude code, opencode, pi, GitHub Copilot.
- **Session checkpointing**: state and context persistence across crashes.
- **GitHub integration**: issue and discussion templates with
  Discussion→Issue workflow.

### Changed

- **Framework on harness**: Mill is loaded as a skill into CTO sessions, not a
  standalone spawner. Uses native `task()` when available, CLI as fallback.
- **Model tiers**: free/paid/pro model selection per role for cost optimization.
- **Async by default**: `mill delegate` returns immediately; `--wait` for sync.

### Fixed

- Roles derive capabilities from `ROLE.md` instead of hand-written cases.
- Phase gates and role sources are versioned so worktrees can see them.
- `core.hooksPath` stored as absolute path for worktree portability.
- Reviewer receives the real diff and build result from delegated work.
- `classifyResult` returns `ClassificationChangesRequested` for
  `changes_requested` signal.
- Coverage gate computes project-wide total instead of sampling one package.
- Worktree branch is merged instead of checking out the target branch.
- Scaffold context files are properly embedded with dot-directories.
- `core.hooksPath` scoped per worktree with real gate execution.

### Security

- Role enforcement mechanically blocks delegation outside authorized targets.

[Unreleased]:
[v0.1.0]:
