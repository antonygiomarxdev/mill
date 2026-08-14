# Releasing Mill

This document describes how to cut a release of Mill.

## Versioning

Mill uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

- **Tags**: `vX.Y.Z` (e.g. `v0.1.0`, `v0.2.0`)
- **VERSION file**: contains the current version string (e.g. `v0.1.0`)
- **No `--dirty` suffix**: dev checkouts read the VERSION file; `git describe`
  is used only as a last-resort fallback and does not append `--dirty`.

### Version resolution priority

```
ldflags Version  →  VERSION file  →  git describe  →  "dev"
```

- **Released binaries**: GoReleaser sets `Version` via ldflags at build time.
- **Dev checkouts**: the committed `VERSION` file provides a meaningful version
  (e.g. `v0.1.0`) regardless of working-tree state.
- **Fallback**: if neither is available, `git describe --tags --always` is used.
- **Last resort**: `"dev"`.

## Prerequisites

1. **Git identity** configured:
   ```bash
   git config user.name
   git config user.email
   ```
2. **Clean working tree**:
   ```bash
   git status  # must show nothing to commit, working tree clean
   ```
3. **CI passing** on the current branch (build, vet, test, gofmt).

## Cutting a release

### Option A: `make release` (recommended)

```bash
make release VERSION=v0.2.0
```

This runs `scripts/release.sh`, which:

1. Validates the version format (`vX.Y.Z`).
2. Verifies the working tree is clean.
3. Collects commits since the last tag using Conventional Commits.
4. Generates a `CHANGELOG.md` entry under `## [vX.Y.Z] - YYYY-MM-DD`.
5. Bumps the `VERSION` file.
6. Builds and runs tests.
7. Commits `VERSION` and `CHANGELOG.md` as `release: vX.Y.Z`.
8. Tags the commit with `vX.Y.Z`.

### Option B: manual

```bash
# 1. Ensure clean tree
git status

# 2. Update VERSION
echo "v0.2.0" > VERSION

# 3. Update CHANGELOG.md — add entry under [Unreleased] with today's date

# 4. Commit and tag
git add VERSION CHANGELOG.md
git commit -m "release: v0.2.0"
git tag v0.2.0

# 5. Push
git push origin main --tags
```

## After tagging

1. **Push the tag**:
   ```bash
   git push origin main --tags
   ```
2. **GoReleaser** (configured in `.goreleaser.yml`) automatically:
   - Builds binaries for Linux/macOS, amd64/arm64.
   - Cross-compiles with `CGO_ENABLED=0`.
   - Injects the version via ldflags.
   - Publishes release archives and checksums to GitHub Releases.
   - Generates release notes from `CHANGELOG.md`.
3. The `install.sh` script picks up the new release automatically (queries the
   GitHub API for the latest tag).

## Conventional Commits

Commits must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new feature
fix(scope): fix a bug
feat(scope): add something to a scope
BREAKING CHANGE: something broke
```

The release script uses the type prefix to categorize changelog entries:

| Type      | Changelog section |
|-----------|-------------------|
| `feat`    | Added             |
| `fix`     | Fixed             |
| `perf`    | Performance       |
| `refactor`| Changed           |
| `BREAKING`| Removed/Changed   |

## Bumping versions

- **Patch** (`v0.1.0` → `v0.1.1`): bug fixes, no new features.
- **Minor** (`v0.1.0` → `v0.2.0`): backward-compatible feature additions.
- **Major** (`v0.1.0` → `v1.0.0`): breaking changes (not yet in use; Mill is pre-1.0).
