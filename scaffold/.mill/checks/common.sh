#!/bin/bash
# Shared helpers for the mill gauntlet. Used by mill-verify (the verification
# entry point at the dispatch boundary) and, when a project opts in with
# --with-hooks, by the git hooks.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass() { echo -e "${GREEN}PASS${NC} $*"; }
fail() { echo -e "${RED}FAIL${NC} $*"; exit 1; }

# load_gauntlet reads the gauntlet configuration.
#
# The config lives at .mill/gauntlet in the Mill root — the repository the
# check scripts themselves live in, derived from this file's own location.
# mill-verify runs against a worker's worktree, but the config is read from
# the coordinator's repo: a worker that edits its own copy widens its own
# gate. The commands still execute in the worktree (run_step evaluates them
# in the caller's cwd), so they check exactly what the worker produced. When
# a project opts into hooks (core.hooksPath -> .mill/checks), the hooks run
# from the project's own copy of the checks and read that project's config,
# as before. It is plain bash that the checks source with `source` — no
# parser, no new dependency. A project that configures nothing is a project
# with no gauntlet: load_gauntlet reports that and returns 0, so a fresh
# install never breaks the first commit and mill-verify still enforces the
# role.
#
# The config declares one shell command per gauntlet step, e.g.:
#
#   build="npm run build"
#   lint="npm run lint"
#   test="npm test"
#   coverage="npm run coverage"   # optional; a step that reports a percentage
#
# Environment variables in the command are expanded at run time, so
# `test="go test $PKG"` works. Variables from the project's own environment are
# available too; variables read here are named GAUNTLET_* to avoid clobbering.
load_gauntlet() {
    GAUNTLET_CONFIG="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.mill/gauntlet"
    if [[ ! -f "$GAUNTLET_CONFIG" ]]; then
        echo "mill: no .mill/gauntlet — no gauntlet commands configured for this project"
        echo "mill: (copy the example from .mill/gauntlet.example and set build/lint/test)"
        return 0
    fi
    # shellcheck disable=SC1090
    source "$GAUNTLET_CONFIG"
}

# run_step runs one named gauntlet step: the command from $stepname, or a clear
# skip when the config does not define one. Failure of a configured command
# fails the verification (or rejects the commit, in the opt-in hook path).
run_step() {
    local stepname="$1"
    local cmd
    cmd="${!stepname:-}"
    if [[ -z "$cmd" ]]; then
        echo "mill: $stepname: not configured (no $stepname=... in .mill/gauntlet)"
        return 0
    fi
    echo "mill: $stepname: $cmd"
    if eval "$cmd"; then
        pass "$stepname"
    else
        fail "$stepname — run: $cmd"
    fi
}

# run_coverage_step is the optional coverage gate. The step's command must print
# a single percentage; the lowest percentage seen blocks the push. Without a
# configured command, the step is skipped like any other.
run_coverage_step() {
    local cmd="${coverage:-}"
    if [[ -z "$cmd" ]]; then
        echo "mill: coverage: not configured (no coverage=... in .mill/gauntlet)"
        return 0
    fi
    echo "mill: coverage: $cmd"
    local output
    output="$(eval "$cmd" 2>&1)" || fail "coverage — run: $cmd"
    local cov
    cov="$(echo "$output" | grep -oP '[0-9]+([.][0-9]+)?%' | tr -d '%' | sort -n | head -1)"
    if [[ -z "$cov" ]]; then
        fail "coverage — could not parse a percentage from the output"
    fi
    if awk "BEGIN {exit !($cov < 90)}"; then
        fail "coverage ${cov}% < 90% — add tests for uncovered code"
    fi
    pass "coverage ${cov}%"
}
