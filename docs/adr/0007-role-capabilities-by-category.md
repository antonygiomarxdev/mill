# ADR 0007: Role capabilities are declared by category, not file extension

**Status:** Accepted
**Date:** 2026-08-15
**Decided by:** Architect
**Related:** #167

## Context

`role-enforce` derives a role's capability from the `allowed_files` list in the
role's ROLE.md frontmatter and matches by file extension (`ext=".${file##*.}"`,
`role-enforce:80`). All three `sr-dev-*` roles ship `.go`; `tech-lead` ships
`.go`; `ui-designer`/`ux-designer` ship `.pen`. Mill itself is written in Go and
the contracts inherited that, but nothing documents them as a per-project
starting point.

The scaffold is copied into "your project" (README Install step 1). In a
TypeScript project the `sr-dev-fe` role cannot commit a `.tsx` file — the first
commit a dispatched worker tries is rejected, in the worker's worktree, with
the worker having no permission to fix its own role file
(`forbidden_patterns: ROLE.md`). #152 made the gauntlet language-agnostic; the
roles were left behind.

Extending the extension lists per language is fragile: today `.ts`, tomorrow
`.rs`, `.py`, `.kt`, `.swift`. The one thing that must not survive is a
hardcoded language.

## Decision

**A role declares *categories* of work in its `allowed_files` frontmatter, and
the project maps categories to file patterns once, in
`.mill/role-capabilities`.** `role-enforce` keeps matching by pattern, but the
patterns are the project's, not the role's.

- Every role frontmatter replaces its extension list with a category list:
  `code`, `docs`, `policy`, `config`, `design`. Language-free by construction.
- The project maps each category to patterns in `.mill/role-capabilities` —
  the same file, same install step, same "plain bash, sourced" shape as
  `.mill/gauntlet`. `code` in a TypeScript monorepo is
  `code=".ts .tsx .js .jsx .css .json"`; in a Go project it is `code=".go"`.
- `role-enforce` parses the role's categories from ROLE.md (same
  `parse_list`), reads the project's category→pattern map from
  `.mill/role-capabilities`, and checks the file extension against the union.
- An unrecognised category is a hard failure, not a pass — the matcher stays
  fail-closed. A role that declares a category the project never mapped cannot
  commit anything until the project maps it.
- An absent `.mill/role-capabilities` file fails closed: the matcher blocks
  everything, naming the missing file. A fresh install must create it, exactly
  as it must create `.mill/gauntlet`.

This is the ecosystem argument the issue itself points at: "the one thing
`.mill/gauntlet` already establishes per project" (#167, Suggested direction).
Language identity was already a per-project fact, declared in one file, at
install time. Capability patterns are the same kind of fact and belong beside
it.

### Why not the alternatives

**By path** (`sr-dev-*` may write under `src/`, `lib/`, `test/`). Language-
independent, but assumes a layout. Mill cannot know a project's layout before
the project exists, and a per-project layout is exactly the kind of fact the
roles should not hardcode — the same argument as the language. It also cannot
express "policy-author owns `.mill/`" cleanly when the harness itself must be
installed before the layout exists.

**By exclusion** (roles declare what they may *not* touch). Shorter lists, but
a new file type is permitted by default — it fails open. The constraint in the
issue is explicit: `role-enforce` must keep failing closed. This option is the
inverse of the requirement and was rejected outright.

## Consequences

**Gained.** A TypeScript project, a Rust project, and the Go project all use the
same role contracts; only the one category map differs. The roles stop being
the vector for language assumptions. Adding a language to a project is one line
in `.mill/role-capabilities`, not a diff across every code-writing role.

**Lost.** One more per-project config file to create at install time
(`cp .mill/role-capabilities.example .mill/role-capabilities`, mirroring the
gauntlet step). `role-enforce` grows a second lookup, from the role's
categories to the project's patterns.

**Migration.** The existing Go role contracts map directly: `code` → `.go`,
`docs` → `.md`, `config` → `.yml .yaml .json`. The scaffold ships
`.mill/role-capabilities.example` declaring those defaults, so existing Go
projects lose nothing by upgrading — the Go intent is preserved by the example
file.

**The behavioural contract for Go is unchanged**: `role-enforce --test <role>
<file>` still allows and blocks the same files it does today for the default
map. For a TypeScript project the same command now allows what it should have
allowed all along.

## Notes

The `qa-docs` prose "`go test ./...` or equivalent" and COMMON.md's "build +
vet" were filed in #167 as sibling assumptions. They are fixed as part of this
work: COMMON.md's "vet" is already stale (the pre-commit hook runs build and
lint from `.mill/gauntlet`, not `go vet`), and the scaffold's hooks never
touched `vet` anyway.
