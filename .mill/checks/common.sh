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

# resolve_orca resolves the Orca CLI executable name once and caches it in
# ORCA_CLI. The rule (from Orca's orchestration guide, measured on this box):
#
#   1. $ORCA_CLI_COMMAND when set — Orca exports it for managed WSL sessions.
#   2. orca-dev when $ORCA_DEV_REPO_ROOT is set.
#   3. Linux and no $ORCA_WORKSPACE_ID → orca-ide.
#   4. Otherwise → orca.
#
# The bare name `orca` collides with GNOME's screen reader at /usr/bin/orca on
# Linux; which one wins depends on PATH order. $ORCA_WORKSPACE_ID is set by Orca
# in every terminal it manages, so its presence means this session is inside one
# and bare `orca` is correct. Its absence on Linux means an ordinary terminal,
# where `orca-ide` is the documented name.
#
# Resolution is lazy (first use), not at source time: a script that sources this
# file in a context with no Orca must not fail merely for being sourced. A
# failure names what it looked for.
#
# Step 5 — the screen-reader guard — lives in the caller (mill-preflight), not
# here: this function resolves a name, it does not judge it. That keeps the
# resolver pure and testable, and lets the refusal fire exactly once at the
# gate that should stop a bad dispatch.
resolve_orca() {
    if [[ -n "${ORCA_CLI:-}" ]]; then
        return 0
    fi

    if [[ -n "${ORCA_CLI_COMMAND:-}" ]]; then
        ORCA_CLI="$ORCA_CLI_COMMAND"
    elif [[ -n "${ORCA_DEV_REPO_ROOT:-}" ]]; then
        ORCA_CLI="orca-dev"
    elif [[ "$(uname -s)" == "Linux" && -z "${ORCA_WORKSPACE_ID:-}" ]]; then
        ORCA_CLI="orca-ide"
    else
        ORCA_CLI="orca"
    fi
}
