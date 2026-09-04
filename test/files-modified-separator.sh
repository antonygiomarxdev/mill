#!/usr/bin/env bash
# files-modified-separator test — mill-verify refuses a space-separated
# --files-modified list with exit 2.
#
# Why: the old <list> placeholder was undefined, and the natural reading is a
# space-separated list. mill-verify splits only on commas, so a space-separated
# list collapses into ONE entry containing spaces; role-enforce then takes the
# basename of the whole string, and the entire change set is judged by whatever
# follows the last "/". A worker can put a forbidden file anywhere but last and
# the gate never sees it. The fix refuses any entry that still carries
# whitespace after the comma split.
#
# It takes the script under test as $1 and defaults to .mill/checks/mill-verify
# (the working copy), so the same test doubles as its own negative control:
# point it at the pre-change script and the "space-separated list is refused"
# case must fail:
#   git show HEAD:.mill/checks/mill-verify > /tmp/pre-sep-mill-verify \
#       && chmod +x /tmp/pre-sep-mill-verify \
#       && bash test/files-modified-separator.sh /tmp/pre-sep-mill-verify
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

script_under_test="${1:-$repo/.mill/checks/mill-verify}"
if [[ ! -f "$script_under_test" ]]; then
    echo "FAIL: script under test not found: $script_under_test" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# mill-verify needs a project root (with .mill/role-capabilities and a gauntlet)
# and a worktree (a clean git repo) that is NOT the project root. It resolves
# its helper scripts (role-enforce, common.sh) relative to its own directory
# ($here), so we copy the script under test into a self-contained .mill tree
# and copy the real helpers and role definitions around it.
project="$tmp/project"
worktree="$tmp/worktree"

mkdir -p "$project/.mill/checks" "$project/.mill/roles/policy-author" "$worktree"

cp "$script_under_test"                        "$project/.mill/checks/mill-verify"
cp "$repo/.mill/checks/role-enforce"           "$project/.mill/checks/role-enforce"
cp "$repo/.mill/checks/common.sh"              "$project/.mill/checks/common.sh"
cp "$repo/.mill/roles/policy-author/ROLE.md"    "$project/.mill/roles/policy-author/ROLE.md"
cp "$repo/.mill/role-capabilities"              "$project/.mill/role-capabilities"

# A minimal gauntlet: no build/lint/test steps configured, so all three are
# skipped inside the worktree.
cat > "$project/.mill/gauntlet" <<'EOF'
#!/bin/bash
# minimal gauntlet — no steps configured; all skipped.
EOF

# The worktree must be a clean git repo: mill-verify refuses uncommitted work.
git -C "$worktree" init -q
git -C "$worktree" commit --allow-empty -q -m "init"

mill_verify="$project/.mill/checks/mill-verify"

# run_verify runs mill-verify, returning its output in $out and its exit code
# in $rc — a set -e-safe capture.
run_verify() {
    if out="$("$mill_verify" "$@" 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
}

failures=0

# Case 1: a correct comma-separated list verifies as today.
run_verify --project-root "$project" --worktree "$worktree" \
    --role policy-author --files-modified "README.md,INSTALL.md"
if [[ "$rc" -ne 0 ]]; then
    echo "FAIL: comma-separated list expected exit 0, got exit $rc" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: comma-separated list verifies (exit $rc)"
fi

# Case 2: a space-separated list is refused with exit 2. The refusal happens
# before any file is judged, so it must fire whether or not the last path
# would have been allowed — here the last path (README.md) is allowed, and the
# list is still refused.
run_verify --project-root "$project" --worktree "$worktree" \
    --role policy-author --files-modified "INSTALL.md README.md"
if [[ "$rc" -ne 2 ]]; then
    echo "FAIL: space-separated list expected exit 2, got exit $rc" >&2
    echo "$out" >&2
    failures=1
elif ! grep -q "whitespace" <<< "$out"; then
    echo "FAIL: refusal did not name whitespace (exit $rc):" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: space-separated list refused with exit 2"
fi

# Case 3: a single path with no separator at all still works.
run_verify --project-root "$project" --worktree "$worktree" \
    --role policy-author --files-modified "README.md"
if [[ "$rc" -ne 0 ]]; then
    echo "FAIL: single path expected exit 0, got exit $rc" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: single path verifies (exit $rc)"
fi

if [[ "$failures" -ne 0 ]]; then
    echo "files-modified-separator: FAIL — see failures above" >&2
    exit 1
fi
echo "files-modified-separator: PASS — comma works, space refused, single works"
exit 0
