#!/usr/bin/env bash
# two-roots test — mill-verify refuses when the project root and the worktree
# are the same tree (ADR 0014 / #194).
#
# Why this exists: Mill's own runs have the install root and the project root
# coinciding, so the distinct-trees refusal is never exercised by normal use.
# This is the check that fails if the refusal stops working — delete or weaken
# the comparison in mill-verify and this exits non-zero.
#
# Run: test/two-roots.sh   (from anywhere; resolves mill-verify from its own
# location). No git, no orca, no network: pure refusal-path smoke.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mill_verify="$repo/.mill/checks/mill-verify"
if [[ ! -x "$mill_verify" ]]; then
    echo "FAIL: mill-verify not found at $mill_verify" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0

# run_verify runs mill-verify, returning its output in $out and its exit code
# in $rc — a set -e-safe capture that does not swallow the refusal's code.
run_verify() {
    if out="$("$mill_verify" "$@" 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
}

# Case 1: the same tree given twice, once with a trailing slash — the slash
# must not defeat the comparison (both are resolved to real paths first).
run_verify --worktree "$tmp/" --role policy-author --project-root "$tmp"
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL: mill-verify accepted a project root equal to the worktree (exit 0)" >&2
    echo "$out" >&2
    failures=1
elif ! grep -q "same tree" <<< "$out"; then
    echo "FAIL: refusal did not name the collision (exit $rc, no 'same tree'):" >&2
    echo "$out" >&2
    failures=1
else
    echo "PASS: same tree via trailing slash refused (exit $rc)"
fi

# Case 2: a symlink to the same tree — real-path resolution must catch it.
link="$tmp/link"
ln -s "$tmp" "$link"
run_verify --worktree "$link" --role policy-author --project-root "$tmp"
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL: mill-verify accepted a symlinked worktree pointing at the project root (exit 0)" >&2
    echo "$out" >&2
    failures=1
elif ! grep -q "same tree" <<< "$out"; then
    echo "FAIL: symlink case not refused by name (exit $rc, no 'same tree'):" >&2
    echo "$out" >&2
    failures=1
else
    echo "PASS: same tree via symlink refused (exit $rc)"
fi

if [[ "$failures" -ne 0 ]]; then
    exit 1
fi
echo "two-roots: PASS — the distinct-trees refusal works"
exit 0
