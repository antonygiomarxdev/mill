#!/bin/bash
# Shared helpers for the mill gauntlet. Used by mill-verify (the verification
# entry point at the dispatch boundary) and the other .mill/checks scripts.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass() { echo -e "${GREEN}PASS${NC} $*"; }
fail() { echo -e "${RED}FAIL${NC} $*"; exit 1; }

# load_gauntlet reads the gauntlet configuration.
#
# The config lives at .mill/gauntlet in the PROJECT ROOT, passed in as the
# first argument — never computed from this file's own location. .mill/gauntlet
# differs per project and cannot be packaged (ADR 0014, #194), so the project
# root is resolved by the caller (mill-verify) before it enters the worktree
# and passed explicitly. mill-verify runs the gauntlet against a worker's
# worktree, but the config is read from the project: a worker that edits its
# own copy widens its own gate. The commands still execute in the worktree
# (run_step evaluates them in the caller's cwd), so they check exactly what
# the worker produced. It is plain bash that the checks source with `source`
# — no parser, no new dependency. A project that configures nothing is a
# project with no gauntlet: load_gauntlet reports that and returns 0, and
# mill-verify still enforces the role.
#
# The config declares one shell command per gauntlet step, e.g.:
#
#   build="npm run build"
#   lint="npm run lint"
#   test="npm test"
#
# Environment variables in the command are expanded at run time, so
# `test="go test $PKG"` works. Variables from the project's own environment are
# available too; variables read here are named GAUNTLET_* to avoid clobbering.
load_gauntlet() {
    local project_root="${1:-}"
    if [[ -z "$project_root" ]]; then
        echo "mill: load_gauntlet called without a project root" >&2
        return 0
    fi
    GAUNTLET_CONFIG="$project_root/.mill/gauntlet"
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
# fails the verification.
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
