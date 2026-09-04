#!/usr/bin/env bash
# orca-resolution test — every branch of resolve_orca resolves as specified.
#
# The resolver lives in .mill/checks/common.sh as resolve_orca; it caches into
# ORCA_CLI. This test sources that file in a subshell, sets/unsets the relevant
# environment variables, and asserts the resolved name. It must NOT touch the
# real PATH or invoke any real executable — each branch is exercised by
# controlling the environment the resolver reads.
#
# It takes the script under test as $1, defaulting to .mill/checks/common.sh
# (the working copy), so the same file doubles as its own negative control:
# point it at the pre-change script and the branch cases must fail:
#   git show HEAD:.mill/checks/common.sh > /tmp/pre-orca.sh \
#       && bash test/orca-resolution.sh /tmp/pre-orca.sh
#
# Each run is bounded with `timeout`; a timeout is a failure (a suite that
# hangs against the old code is worse than one that fails).
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_under_test="${1:-$repo/.mill/checks/common.sh}"
if [[ ! -f "$script_under_test" ]]; then
    echo "FAIL: script under test not found: $script_under_test" >&2
    exit 1
fi

# Sourcing common.sh pulls in `set -euo pipefail` and the helpers. resolve_orca
# is a function defined there; invoking it sets ORCA_CLI in our shell.
# shellcheck disable=SC1090
source "$script_under_test"

# run_case <label> <expected> <env-setup-code> — clear any cached resolution,
# eval the setup code (which sets the knobs for this case), resolve, compare.
run_case() {
    local label="$1" expected="$2" setup="$3"
    # Clear any cached resolution and the knobs a prior case may have set.
    unset ORCA_CLI ORCA_CLI_COMMAND ORCA_DEV_REPO_ROOT ORCA_WORKSPACE_ID 2>/dev/null || true
    # Drop any uname override from a prior case.
    unset -f uname 2>/dev/null || true
    eval "$setup"
    resolve_orca
    if [[ "$ORCA_CLI" != "$expected" ]]; then
        echo "FAIL: $label — expected '$expected', got '$ORCA_CLI'" >&2
        failures=1
    else
        echo "ok: $label → $ORCA_CLI"
    fi
}

failures=0

# --- Case 1: ORCA_CLI_COMMAND wins over everything ---------------------------
run_case "ORCA_CLI_COMMAND wins" "/custom/path/orca" \
    'ORCA_CLI_COMMAND="/custom/path/orca"; ORCA_DEV_REPO_ROOT="/some/repo"; ORCA_WORKSPACE_ID="ws_123"'

# --- Case 2: ORCA_DEV_REPO_ROOT selects orca-dev ----------------------------
run_case "ORCA_DEV_REPO_ROOT selects orca-dev" "orca-dev" \
    'ORCA_DEV_REPO_ROOT="/some/repo"; ORCA_WORKSPACE_ID="ws_123"'

# --- Case 3: Linux without ORCA_WORKSPACE_ID selects orca-ide ---------------
run_case "Linux without ORCA_WORKSPACE_ID selects orca-ide" "orca-ide" \
    ':'

# --- Case 4: Linux with ORCA_WORKSPACE_ID selects orca ----------------------
run_case "Linux with ORCA_WORKSPACE_ID selects orca" "orca" \
    'ORCA_WORKSPACE_ID="ws_123"'

# --- Case 5: non-Linux without ORCA_WORKSPACE_ID selects orca ----------------
# The resolver keys on `uname -s`. To exercise the "otherwise" branch without
# a macOS box, wrap uname so it reports Darwin.
run_case "macOS (uname=Darwin) selects orca" "orca" \
    'uname() { echo "Darwin"; }; export -f uname; ORCA_WORKSPACE_ID="ws_123"'

# --- Case 6: caching — once set, resolve_orca returns without overwriting ----
unset ORCA_CLI ORCA_CLI_COMMAND ORCA_DEV_REPO_ROOT ORCA_WORKSPACE_ID 2>/dev/null || true
unset -f uname 2>/dev/null || true
ORCA_CLI_COMMAND="/custom/path/orca"
resolve_orca
# Now change the knobs and call again; the cached value must win.
ORCA_CLI_COMMAND="/other/path"
ORCA_DEV_REPO_ROOT="/some/repo"
resolve_orca
if [[ "$ORCA_CLI" != "/custom/path/orca" ]]; then
    echo "FAIL: caching — resolve_orca overwrote a cached value (got '$ORCA_CLI')" >&2
    failures=1
else
    echo "ok: caching — cached value is not overwritten"
fi

if [[ "$failures" -ne 0 ]]; then
    echo "orca-resolution: FAIL — see failures above" >&2
    exit 1
fi
echo "orca-resolution: PASS — every branch resolves as specified"
exit 0
