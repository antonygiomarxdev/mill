# 0004 — Versioning strategy and release process

- **Status**: Accepted
- **Deciders**: Architect
- **Date**: 2026-08-14
- **Tags**: release, versioning, changelog, goreleaser

## Context

Mill has no release infrastructure:

- `mill version` prints `4f577ab-dirty` — a commit hash with a `--dirty` suffix
  whenever the working tree has uncommitted files.
- No git tags exist, so `git describe --tags --always` falls back to the commit
  hash.
- No `CHANGELOG.md` exists. 24 issues have been implemented with no record of
  what changed.
- GoReleaser (`.goreleaser.yml`) uses `{{.Version}}` for ldflags, but with no
  tags it always resolves to the commit hash.
- No documented release process — contributors do not know how to cut a release.
- `resolveVersion()` appends `--dirty` to any version when the working tree is
  dirty, making development builds permanently report `-dirty`.

## Decision

Mill adopts Semantic Versioning with the following changes:

### 1. Version file (`VERSION`)

A committed `VERSION` file contains the current version string (e.g. `v0.1.0`).
This file is the source of truth for development checkouts — it provides a
meaningful, human-readable version regardless of git state.

### 2. Version resolution priority

```
ldflags Version  →  VERSION file  →  git describe  →  "dev"
```

- **ldflags** (highest): GoReleaser sets `Version` at build time from the git
  tag. This is the version for released binaries.
- **VERSION file**: Read from `./VERSION` in the working directory. Provides a
  meaningful version for `go run ./cmd/mill version` in development checkouts.
- **git describe**: Fallback when no VERSION file exists (e.g. a bare git clone
  without the file). Uses `--tags --always` **without** `--dirty` to avoid the
  permanent dirty suffix.
- **"dev"**: Last resort when git is unavailable.

### 3. No `--dirty` suffix

The `--dirty` flag is removed from `git describe`. Uncommitted changes in a dev
checkout should not permanently degrade the version string. The VERSION file
provides a stable version identifier for the checked-out release line.

### 4. v-prefix normalization

`normalizeVersion()` ensures every version string starts with `v` (e.g.
`0.1.0` → `v0.1.0`). This handles the case where GoReleaser's `{{.Version}}`
strips the `v` prefix from the tag — the normalization re-adds it at runtime
for consistent output.

### 5. Release script (`scripts/release.sh`)

`make release VERSION=v0.2.0` runs `scripts/release.sh`, which:

1. Validates the version format (`vX.Y.Z`).
2. Verifies the working tree is clean.
3. Collects commits since the last tag using Conventional Commits.
4. Generates a `CHANGELOG.md` entry grouped by type (feat→Added, fix→Fixed,
   refactor→Changed, etc.).
5. Bumps the `VERSION` file.
6. Builds and runs tests.
7. Commits `VERSION` + `CHANGELOG.md` as `release: vX.Y.Z`.
8. Tags the commit with `vX.Y.Z`.

### 6. CHANGELOG.md

Adopted [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format with
an `[Unreleased]` section for pending changes and dated sections for each
release.

### 7. RELEASING.md

Released `RELEASING.md` documenting the full release workflow, version
resolution priority, conventional commit mapping, and post-tag steps
(GoReleaser, `install.sh` pickup).

## Alternatives considered

- **Pure `git describe` without VERSION file**: Rejected. With no tags,
  `git describe --always` produces a commit hash (`4f577ab`), which is
  non-informative. The VERSION file gives dev checkouts a meaningful version
  immediately after a commit.
- **Keep `--dirty`**: Rejected. The permanent `-dirty` suffix has no practical
  value for development builds and confuses users who simply have uncommitted
  files (e.g. local config). The VERSION file provides a stable identifier.
- **`{{.Tag}}` in GoReleaser instead of `{{.Version}}`**: Rejected. GoReleaser's
  `{{.Version}}` strips the `v` prefix, but normalizing in Go (via
  `normalizeVersion`) achieves the same result without coupling the build
  template to GoReleaser internals. This also means future release tooling
  changes do not require GoReleaser config edits.
- **Manual changelog editing only**: Rejected. While changelogs can be hand-
  edited, the release script generates a first draft from Conventional Commits,
  which can be edited before the commit is finalized. This reduces friction for
  contributors.

## Consequences

- **Users** see `v0.1.0` from `mill version` in dev checkouts instead of a
  commit hash.
- **Released binaries** print `v0.1.0` (normalized from GoReleaser's `0.1.0`
  via ldflags).
- **Contributors** have a documented `make release` flow and clear version
  resolution behavior.
- **GoReleaser** can now publish real `vX.Y.Z` releases with meaningful
  `{{.Version}}` values.
- **`install.sh`** already queries the GitHub API for the latest tag, so it
  picks up new releases automatically once tags exist.
- **Tests** in `version_test.go` cover `normalizeVersion` (pure function),
  `resolveVersion` (VERSION file + git fallback chain), and `runVersion`
  (ldflags + normalization end-to-end).
