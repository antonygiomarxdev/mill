#!/bin/bash
# scripts/release.sh — Cut a new release: bump version, generate changelog, tag.
#
# Usage:   scripts/release.sh <version>
# Example: scripts/release.sh v0.2.0
#
# Requirements:
#   - Working tree must be clean (no uncommitted changes)
#   - Version must be a valid SemVer tag (vMAJOR.MINOR.PATCH)
#   - Git history must use Conventional Commits (feat:, fix:, etc.)
#
set -euo pipefail

# ── helpers ───────────────────────────────────────────────────────────────────

die() {
    echo "release.sh: $*" >&2
    exit 1
}

usage() {
    echo "Usage: scripts/release.sh <version>"
    echo "Example: scripts/release.sh v0.2.0"
    echo ""
    echo "The version must follow SemVer: vMAJOR.MINOR.PATCH"
    exit 1
}

# ── arguments ─────────────────────────────────────────────────────────────────

if [[ $# -ne 1 ]]; then
    usage
fi

VERSION="$1"

# Validate SemVer format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    die "Invalid version '$VERSION'. Must be vMAJOR.MINOR.PATCH (e.g. v0.2.0)"
fi

# ── pre-flight checks ─────────────────────────────────────────────────────────

# Ensure we're in a git repo
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "Not a git repository"

# Ensure the working tree is clean
if [[ -n "$(git status --porcelain)" ]]; then
    echo "Working tree is dirty:" >&2
    git status --short >&2
    die "Commit or stash changes before releasing"
fi

# Ensure git user is configured
git config user.name >/dev/null 2>&1 || die "git user.name is not set"
git config user.email >/dev/null 2>&1 || die "git user.email is not set"

# Ensure changelog exists
if [[ ! -f CHANGELOG.md ]]; then
    die "CHANGELOG.md not found"
fi

# Ensure VERSION file exists (bootstrap it if missing)
if [[ ! -f VERSION ]]; then
    echo "v0.0.0" > VERSION
fi

# Parse the previous version from VERSION file
PREV_VERSION=$(cat VERSION | tr -d '[:space:]')

echo "Releasing $VERSION (previous: $PREV_VERSION)"

# ── generate changelog entry from conventional commits ────────────────────────

# Collect commits since the last tag (or all commits if no tags exist)
if git describe --tags --exact-match >/dev/null 2>&1; then
    die "Current commit is already tagged. Checkout the next commit first."
fi

LAST_TAG=""
if git tag -l | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)
    echo "Last tag: $LAST_TAG"
    COMMITS=$(git log --format="%s" "${LAST_TAG}..HEAD" --no-merges)
else
    echo "No previous release tags found — generating changelog from all commits"
    COMMITS=$(git log --format="%s" --reverse --no-merges)
fi

# Parse conventional commits into Keep a Changelog categories
# Types: feat→Added, fix→Fixed, perf→Performance, refactor→Changed,
#        revert→Removed, BREAKING CHANGE→Removed/Changed
parse_commits() {
    local section="$1"
    shift
    local types=("$@")
    local lines=()
    while IFS= read -r commit; do
        [[ -z "$commit" ]] && continue
        local lower=$(echo "$commit" | tr '[:upper:]' '[:lower:]')
        for t in "${types[@]}"; do
            if [[ "$lower" == "$t"* ]]; then
                # Strip the type prefix: "feat(api): description" → "description"
                local desc=$(echo "$commit" | sed -E 's/^[^:]*:? *//')
                lines+=("- $desc")
                break
            fi
        done
    done <<< "$COMMITS"
    if [[ ${#lines[@]} -gt 0 ]]; then
        echo "### $section"
        printf '%s\n' "${lines[@]}"
        echo ""
    fi
}

ADDED=$(parse_commits "Added" "feat")
FIXED=$(parse_commits "Fixed" "fix")
PERF=$(parse_commits "Performance" "perf")
CHANGED=$(parse_commits "Changed" "refactor" "build" "style" "chore" "ci")
# BREAKING CHANGE detection (scans commit bodies)
BREAKING=$(git log --format="%b" ${LAST_TAG:+${LAST_TAG}..HEAD} --no-merges 2>/dev/null \
    | grep -i "BREAKING CHANGE" 2>/dev/null || true)

# Determine release date
RELEASE_DATE=$(date +%Y-%m-%d)

# Build the changelog entry
CHANGELOG_ENTRY="## [$VERSION] - $RELEASE_DATE\n"
[[ -n "$ADDED" ]] && CHANGELOG_ENTRY+="\n$ADDED"
[[ -n "$CHANGED" ]] && CHANGELOG_ENTRY+="\n$CHANGED"
[[ -n "$FIXED" ]] && CHANGELOG_ENTRY+="\n$FIXED"
[[ -n "$PERF" ]] && CHANGELOG_ENTRY+="\n$PERF"
[[ -n "$BREAKING" ]] && CHANGELOG_ENTRY+="\n### Removed\n\n- **Breaking changes:** See commit details\n"

# Insert the entry after the "# Changelog" header and "[Unreleased]" section
# If there's no [Unreleased] section, insert after the header block
if grep -q "## \[Unreleased\]" CHANGELOG.md; then
    # Insert before the first ## [Unreleased] section
    awk -v entry="$CHANGELOG_ENTRY" '
        /^## \[Unreleased\]/ && !inserted {
            print entry
            print
            inserted=1
            next
        }
        { print }
    ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
elif grep -qi "^## \[" CHANGELOG.md; then
    # Insert before the latest version section
    awk -v entry="$CHANGELOG_ENTRY" '
        /^## \[v?[0-9]/ && !inserted {
            print entry
            print
            inserted=1
            next
        }
        { print }
    ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
else
    # No version sections yet — just append after the header
    awk -v entry="$CHANGELOG_ENTRY" '
        /^# Changelog/ { print; print ""; print entry; print ""; next }
        { print }
    ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
fi

echo "Generated changelog entry for $VERSION"

# ── bump version ─────────────────────────────────────────────────────────────

echo "$VERSION" > VERSION

# ── build and verify ──────────────────────────────────────────────────────────

echo "Building..."
go build ./...

echo "Running tests..."
go test ./...

# ── commit and tag ───────────────────────────────────────────────────────────

git add VERSION CHANGELOG.md

git commit -m "release: $VERSION"

git tag "$VERSION"

echo ""
echo "✓ Release $VERSION created and tagged"
echo "  Next steps:"
echo "  - Review: git show $VERSION"
echo "  - Push:   git push origin HEAD --tags"
echo "  - GoReleaser will build binaries on the tag"
