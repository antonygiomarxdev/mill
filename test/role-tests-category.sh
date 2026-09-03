#!/usr/bin/env bash
# role-tests-category test — role-enforce decides "a test file" by PATH, not
# by extension.
#
# qa-docs is the role whose job is "Tests, changelogs, and documentation", and
# whose allowed_files are docs/config/tests. Before the `tests` category a
# TypeScript project could not commit a test file through qa-docs: its
# allowed_files mapped to .md/.yml/.yaml/.json/.toml, so role-enforce blocked
# every .ts under __tests__/. The category fixes that by matching paths:
#   tests="test/*.sh */__tests__/* *.test.* *.spec.*"
# matched in role-enforce as one of three forms — a repo-relative path glob
# (contains `/`), a basename glob (contains `*`, no `/`), or a literal
# (extension or exact basename). Literals stay literal: `role-capabilities`
# must never match `my-role-capabilities`.
#
# It takes the script under test as $1 and defaults to
# .mill/checks/role-enforce (the working copy), so the same file doubles as
# its own negative control: point it at the pre-change script and the
# "qa-docs may commit a test file" case must fail:
#   git show HEAD:.mill/checks/role-enforce > /tmp/pre-tests-role-enforce \
#       && chmod +x /tmp/pre-tests-role-enforce \
#       && bash test/role-tests-category.sh /tmp/pre-tests-role-enforce
set -euo pipefail

# The repository root — resolved from this script's own location so the test
# runs from anywhere.
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# The script under test. Default to the working copy; the negative control
# passes the pre-change script extracted from git.
script_under_test="${1:-$repo/.mill/checks/role-enforce}"
if [[ ! -f "$script_under_test" ]]; then
    echo "FAIL: script under test not found: $script_under_test" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# role-enforce resolves its role definitions relative to its own directory
# ($mill_root) and the capability map from --project-root. To run an
# arbitrary script-under-test (including one copied to /tmp) we copy it into
# a self-contained .mill tree and copy the real roles and capability map
# around it, so every lookup resolves inside $tmp.
checks="$tmp/.mill/checks"
mkdir -p "$checks" "$tmp/.mill/roles/qa-docs" "$tmp/.mill/roles/policy-author" "$tmp/test"

cp "$script_under_test"                "$checks/role-enforce"
cp "$repo/.mill/roles/qa-docs/ROLE.md"      "$tmp/.mill/roles/qa-docs/ROLE.md"
cp "$repo/.mill/roles/policy-author/ROLE.md" "$tmp/.mill/roles/policy-author/ROLE.md"
cp "$repo/.mill/role-capabilities"      "$tmp/.mill/role-capabilities"

# Fixture files under judgement. Only the extensionless ones are ever read
# (for their shebang); the dotted ones are decided by name alone.
printf '%s\n' '#!/usr/bin/env bash' '' '# fixture: the test file qa-docs may commit' \
    > "$tmp/test/role-tests-category.sh"
: > "$tmp/my-role-capabilities"

# enforce runs role-enforce in --test mode against the tmp project root and
# expects a specific exit code. Expected 0 = allowed, 1 = blocked.
enforce() {
    local expected="$1" role="$2" file="$3" label="$4" out rc
    if out="$(cd "$tmp" && "$checks/role-enforce" --test "$role" --project-root "$tmp" "$file" 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
    if [[ "$rc" -ne "$expected" ]]; then
        echo "FAIL: $label: expected exit $expected, got exit $rc" >&2
        echo "$out" >&2
        failures=1
    else
        echo "ok: $label (exit $rc)"
    fi
}

failures=0

# qa-docs may commit a test file — the case that was blocked before the
# `tests` category existed.
enforce 0 qa-docs "test/role-tests-category.sh" \
    "qa-docs may commit test/role-tests-category.sh"
# The original provocation: a .ts test file under a __tests__ directory. A
# test file is a fact about its path, so this must pass without qa-docs
# gaining `code`.
enforce 0 qa-docs "packages/utils/src/__tests__/collections.test.ts" \
    "qa-docs may commit a __tests__ test file"

# qa-docs still may NOT commit an implementation file: the gate must not have
# been widened into `code`.
enforce 1 qa-docs ".mill/checks/role-enforce" \
    "qa-docs cannot commit an implementation file"
enforce 1 qa-docs "packages/utils/src/collections.ts" \
    "qa-docs cannot commit an implementation file next to tests"

# A literal entry still does not behave as a glob: policy-author's
# `role-capabilities` entry matches the exact basename and nothing else.
enforce 1 policy-author "my-role-capabilities" \
    "literal role-capabilities is not a prefix"
enforce 0 policy-author "role-capabilities" \
    "literal role-capabilities still matches exactly"

if [[ "$failures" -ne 0 ]]; then
    echo "role-tests-category: FAIL — see failures above" >&2
    exit 1
fi
echo "role-tests-category: PASS — qa-docs commits tests by path, literals stay literal"
exit 0
