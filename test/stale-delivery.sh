#!/usr/bin/env bash
# stale-delivery test — a stale worker_done must not end the wait (#191).
#
# mill-dispatch waits for its worker with a single `orca orchestration check
# --wait`. That call returns the bound Run's oldest unacknowledged delivery,
# which can be a worker_done left over from an earlier dispatch. The pre-fix
# code treated any non-settling first delivery as failure and exited,
# abandoning the live worker. The fix ignores a worker_done that is not this
# dispatch and keeps waiting until the real one arrives or the deadline is
# spent.
#
# This test drives the whole dispatch flow against a STUB `orca` on PATH (never
# the real one). The stub's check --wait serves a stale worker_done (a dispatch
# id that is not the one under test) on its first call, and a settled
# worker_done for the dispatch under test on its second. The fixed script must
# reach release (exit 0); the pre-fix script must exit non-zero.
#
# It takes the script under test as $1 and defaults to .mill/checks/mill-dispatch
# (the working copy), so the same test can be pointed at the pre-fix script as
# a negative control:
#   git show HEAD:.mill/checks/mill-dispatch > /tmp/pre-fix-mill-dispatch \
#       && chmod +x /tmp/pre-fix-mill-dispatch \
#       && bash test/stale-delivery.sh /tmp/pre-fix-mill-dispatch
set -euo pipefail

# The repository root — resolved from this script's own location so the test
# runs from anywhere.
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# The script under test. Default to the working copy; the negative control
# passes the pre-fix script extracted from git.
script_under_test="${1:-$repo/.mill/checks/mill-dispatch}"
if [[ ! -f "$script_under_test" ]]; then
    echo "FAIL: script under test not found: $script_under_test" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# mill-dispatch resolves all its helper paths relative to its own directory
# ($here). To run an arbitrary script-under-test (including one copied to
# /tmp) we copy it into a self-contained .mill tree and copy the real helper
# scripts around it, so every $here-relative lookup resolves inside $tmp.
checks="$tmp/.mill/checks"
mkdir -p "$checks" "$tmp/.mill/roles/product-engineer"

cp "$script_under_test"            "$checks/mill-dispatch"
cp "$repo/.mill/checks/mill-preflight" "$checks/mill-preflight"
cp "$repo/.mill/checks/role-enforce"   "$checks/role-enforce"
cp "$repo/.mill/checks/common.sh"      "$checks/common.sh"
cp "$repo/.mill/roles/product-engineer/ROLE.md" "$tmp/.mill/roles/product-engineer/ROLE.md"
cp "$repo/.mill/role-capabilities"     "$tmp/.mill/role-capabilities"
cp "$repo/.mill/agents.example"        "$tmp/.mill/agents"

# Preflight requires a git work tree and a brief file.
git -C "$tmp" init -q
brief="$tmp/brief.md"
printf '%s\n' "# brief" > "$brief"

# State dir the stub uses to remember the ids it handed out and to count
# check --wait calls.
state="$tmp/state"
mkdir -p "$state"
printf '%s' "0" > "$state/check_count"

# The stub orca. It is the only `orca` on PATH for the run.
mkdir -p "$tmp/bin"
cat > "$tmp/bin/orca" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
state="${ORCA_STUB_STATE:?ORCA_STUB_STATE not set}"
# payload_json: a worker_done payload (a JSON object) for the given dispatch
# and task ids. Printed as a JSON-encoded string value, since mill-dispatch
# parses `.payload | fromjson`.
payload_json() {
    local dispatch_id="$1" task_id="$2" outcome="${3:-succeeded}"
    jq -n --arg d "$dispatch_id" --arg t "$task_id" --arg o "$outcome" \
        '{dispatchId:$d,taskId:$t,outcome:$o}'
}
case "$1" in
    status)
        # mill-preflight greps for this token.
        echo "runtimeReachable: true"
        ;;
    orchestration)
        case "$2" in
            task-create)
                task_id="task_$(head -c 6 /dev/urandom | xxd -p)"
                printf '%s' "$task_id" > "$state/task_id"
                jq -n --arg id "$task_id" '{ok:true,result:{task:{id:$id}}}'
                ;;
            worker-start)
                dispatch_id="ctx_$(head -c 6 /dev/urandom | xxd -p)"
                printf '%s' "$dispatch_id" > "$state/dispatch_id"
                jq -n --arg d "$dispatch_id" '{ok:true,result:{dispatchId:$d,state:"ready"}}'
                ;;
            check)
                count="$(cat "$state/check_count")"
                count=$((count + 1))
                printf '%s' "$count" > "$state/check_count"
                dispatch_id="$(cat "$state/dispatch_id")"
                task_id="$(cat "$state/task_id")"
                if [[ "$count" -eq 1 ]]; then
                    # First check --wait: a STALE worker_done. Its dispatch id is
                    # not the one under test, so a correct loop ignores it.
                    payload="$(payload_json "ctx_STALE_0000000000000000" "$task_id")"
                else
                    # Second check --wait: the SETTLED worker_done for this
                    # dispatch. dispatch id + task id both match.
                    payload="$(payload_json "$dispatch_id" "$task_id")"
                fi
                jq -n --arg p "$payload" --arg i "msg_$count" \
                    '{ok:true,result:{count:1,messages:[{id:$i,type:"worker_done",subject:"worker done",body:"summary",payload:$p}]}}'
                ;;
            worker-release)
                jq -n '{ok:true,result:{state:"released"}}'
                ;;
            *)
                echo "stub orca: unknown orchestration subcommand: $2" >&2
                exit 2
                ;;
        esac
        ;;
    *)
        echo "stub orca: unknown command: $1" >&2
        exit 2
        ;;
esac
STUB
chmod +x "$tmp/bin/orca"

# Run the dispatch from the project dir ($tmp) with the stub on PATH.
mill_dispatch="$checks/mill-dispatch"
stub_env="PATH=$tmp/bin:$PATH ORCA_STUB_STATE=$state"

# run_dispatch runs the script-under-test, returning its output in $out and its
# exit code in $rc — a set -e-safe capture.
run_dispatch() {
    if out="$(env $stub_env "$mill_dispatch" \
            --brief "$brief" \
            --role product-engineer \
            --agent command-code \
            --name stale-delivery-slug \
            --title "stale delivery test" \
            --writes plan.md \
            --timeout-ms 2000 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
}

run_dispatch

failures=0

# The fixed script must reach release: exit 0, having ignored the stale
# worker_done and then settled the real one.
if [[ "$rc" -ne 0 ]]; then
    echo "FAIL: expected exit 0 (reached release), got exit $rc" >&2
    echo "$out" >&2
    failures=1
fi
if ! grep -q "ignoring stale worker_done" <<< "$out"; then
    echo "FAIL: loop did not ignore the stale worker_done" >&2
    echo "$out" >&2
    failures=1
fi
if ! grep -q "settled worker_done" <<< "$out"; then
    echo "FAIL: loop did not reach the settled worker_done" >&2
    echo "$out" >&2
    failures=1
fi
if ! grep -q "worker released" <<< "$out"; then
    echo "FAIL: loop did not reach release" >&2
    echo "$out" >&2
    failures=1
fi

if [[ "$failures" -ne 0 ]]; then
    exit 1
fi
echo "stale-delivery: PASS — stale worker_done ignored, real one settled, worker released (exit $rc)"
exit 0
