#!/usr/bin/env bash
# supervisor-liveness test — the wait loop checks liveness between slices.
#
# mill-dispatch's wait loop used to do one thing: block on `orca orchestration
# check --wait` for the whole remaining budget and react to messages. A worker
# killed by a provider error sends no message, so nothing arrived, and the loop
# waited out its full --timeout-ms supervising a corpse. The fix slices the wait
# and runs mill-liveness between slices, so a dead/parked/uncommitted worker is
# noticed within one slice instead of at the deadline.
#
# This test drives the dispatch loop against a STUB `orca` on PATH (never the
# real one) and a stub mill-liveness whose exit code is set per scenario via a
# state file. It takes the script under test as $1 and defaults to
# .mill/checks/mill-dispatch (the working copy), so the same test doubles as
# its own negative control: point it at the pre-change script and the "dead
# worker stops within one slice" case must fail:
#   git show HEAD:.mill/checks/mill-dispatch > /tmp/pre-sup \
#       && chmod +x /tmp/pre-sup \
#       && bash test/supervisor-liveness.sh /tmp/pre-sup
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

script_under_test="${1:-$repo/.mill/checks/mill-dispatch}"
if [[ ! -f "$script_under_test" ]]; then
    echo "FAIL: script under test not found: $script_under_test" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

checks="$tmp/.mill/checks"
mkdir -p "$checks" "$tmp/.mill/roles/product-engineer"

cp "$script_under_test"            "$checks/mill-dispatch"
cp "$repo/.mill/checks/mill-preflight" "$checks/mill-preflight"
cp "$repo/.mill/checks/role-enforce"   "$checks/role-enforce"
cp "$repo/.mill/checks/common.sh"      "$checks/common.sh"
cp "$repo/.mill/roles/product-engineer/ROLE.md" "$tmp/.mill/roles/product-engineer/ROLE.md"
cp "$repo/.mill/role-capabilities"     "$tmp/.mill/role-capabilities"
cp "$repo/.mill/agents.example"        "$tmp/.mill/agents"

# Stub mill-liveness: its exit code is read from $state/liveness_rc, so each
# scenario configures the verdict it wants. Its stdout is the evidence the
# script reports.
cat > "$checks/mill-liveness" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
state="${ORCA_STUB_STATE:?ORCA_STUB_STATE not set}"
rc="$(cat "$state/liveness_rc")"
echo "stub mill-liveness: dispatch=$2 rc=$rc"
exit "$rc"
STUB
chmod +x "$checks/mill-liveness"

# Preflight requires a git work tree and a brief file.
git -C "$tmp" init -q
brief="$tmp/brief.md"
printf '%s\n' "# brief" > "$brief"

# State dir the stub uses to remember the ids it handed out, count check --wait
# calls, and record whether worker-release was called.
state="$tmp/state"
mkdir -p "$state"
printf '%s' "0" > "$state/check_count"
printf '%s' "0" > "$state/release_called"
printf '%s' "30" > "$state/liveness_rc"   # default: dead worker

# The stub orca. It is the only `orca` on PATH for the run.
mkdir -p "$tmp/bin"
cat > "$tmp/bin/orca" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
state="${ORCA_STUB_STATE:?ORCA_STUB_STATE not set}"
payload_json() {
    local dispatch_id="$1" task_id="$2" outcome="${3:-succeeded}"
    jq -n --arg d "$dispatch_id" --arg t "$task_id" --arg o "$outcome" \
        '{dispatchId:$d,taskId:$t,outcome:$o}'
}
case "$1" in
    status)
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
                # Never serve a worker_done: the only way the loop settles is
                # via the liveness probe, which is what this test exercises.
                jq -n '{ok:true,result:{count:0,messages:[]}}'
                ;;
            inbox)
                # No hidden deliveries in these scenarios.
                jq -n '{ok:true,result:{messages:[]}}'
                ;;
            worker-release)
                printf '%s' "1" > "$state/release_called"
                jq -n '{ok:true,result:{state:"released"}}'
                ;;
            *)
                echo "stub orca: unknown orchestration subcommand: $2" >&2
                exit 2
                ;;
        esac
        ;;
    terminal)
        # terminal rename / wait: accept silently. The rename verdict is not
        # asserted here (it needs a live terminal); the trap's presence is.
        jq -n '{ok:true,result:{}}'
        ;;
    *)
        echo "stub orca: unknown command: $1" >&2
        exit 2
        ;;
esac
STUB
chmod +x "$tmp/bin/orca"

mill_dispatch="$checks/mill-dispatch"
stub_env="PATH=$tmp/bin:$PATH ORCA_STUB_STATE=$state ORCA_TERMINAL_HANDLE=term_test_handle"

# run_dispatch runs the script-under-test with the given liveness exit code,
# returning its output in $out, its exit code in $rc, and the elapsed
# wall-clock milliseconds in $elapsed_ms.
run_dispatch() {
    local liveness_rc="${1:-30}"
    printf '%s' "$liveness_rc" > "$state/liveness_rc"
    printf '%s' "0" > "$state/check_count"
    printf '%s' "0" > "$state/release_called"
    start_ms="$(date +%s%3N)"
    if out="$(env $stub_env "$mill_dispatch" \
            --brief "$brief" \
            --role product-engineer \
            --agent command-code \
            --name supervisor-liveness-slug \
            --title "supervisor liveness test" \
            --writes plan.md \
            --timeout-ms 120000 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
    end_ms="$(date +%s%3N)"
    elapsed_ms=$((end_ms - start_ms))
}

failures=0

# --- Scenario 1: a dead worker stops the loop within one slice ---------------
# The stub mill-liveness exits 30 (dead). The loop must notice and stop. With
# a 120000 ms deadline and a 60000 ms slice, a listen-only loop would run the
# full deadline; the fixed loop exits after the first slice's probe. The stub's
# check --wait returns immediately (count 0), so the elapsed time is dominated
# by the probe and is well under the deadline.
run_dispatch 30
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL [dead]: expected non-zero exit (dead worker must not release), got exit 0" >&2
    echo "$out" >&2
    failures=1
fi
if ! grep -q "WORKER DEAD" <<< "$out"; then
    echo "FAIL [dead]: loop did not report a dead worker" >&2
    echo "$out" >&2
    failures=1
fi
if [[ "$elapsed_ms" -ge 120000 ]]; then
    echo "FAIL [dead]: loop ran the full deadline (${elapsed_ms} ms) instead of stopping within one slice" >&2
    failures=1
else
    echo "ok: dead worker stopped the loop in ${elapsed_ms} ms (deadline 120000 ms)"
fi
release_called="$(cat "$state/release_called")"
if [[ "$release_called" -ne 0 ]]; then
    echo "FAIL [dead]: worker-release was called for a dead worker — release rule violated" >&2
    failures=1
else
    echo "ok: dead worker was NOT released (release rule intact)"
fi

# --- Scenario 2: a working worker does not stop the loop ---------------------
# The stub mill-liveness exits 0 (working). The loop must keep waiting until the
# deadline, and produce no per-slice noise. Use a short deadline so the test is
# fast; the loop should run the full deadline and exit non-zero.
printf '%s' "0" > "$state/liveness_rc"
printf '%s' "0" > "$state/check_count"
printf '%s' "0" > "$state/release_called"
start_ms="$(date +%s%3N)"
if out="$(env $stub_env "$mill_dispatch" \
        --brief "$brief" \
        --role product-engineer \
        --agent command-code \
        --name supervisor-liveness-slug \
        --title "supervisor liveness test" \
        --writes plan.md \
        --timeout-ms 2000 2>&1)"; then
    rc=0
else
    rc=$?
fi
end_ms="$(date +%s%3N)"
elapsed_ms=$((end_ms - start_ms))
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL [working]: expected non-zero exit (deadline spent, no settlement), got exit 0" >&2
    echo "$out" >&2
    failures=1
fi
if ! grep -q "wait deadline spent" <<< "$out"; then
    echo "FAIL [working]: loop did not run to the deadline" >&2
    echo "$out" >&2
    failures=1
fi
# No per-slice noise: the working path must stay quiet. Between the "waiting
# for dispatch" line and the "wait deadline spent" line there must be NO
# output — a per-slice log would make a supervisor's log unreadable. Extract
# that window and assert it is empty.
window="$(sed -n '/waiting for dispatch/,/wait deadline spent/p' <<< "$out")"
# Remove the bounding lines themselves; anything left is per-slice output.
inner="$(sed -e '/waiting for dispatch/d' -e '/wait deadline spent/d' <<< "$window")"
if [[ -n "$inner" ]]; then
    echo "FAIL [working]: working worker produced per-slice noise between wait and deadline:" >&2
    echo "$inner" >&2
    failures=1
else
    echo "ok: working worker produced no per-slice noise and ran to the deadline (${elapsed_ms} ms)"
fi

# --- Scenario 3: exit 2 from the probe logs once and continues ---------------
# The stub mill-liveness exits 2 (unresolvable). The loop must log once and keep
# waiting until the deadline. With a 2000 ms deadline the probe runs a handful
# of times but the "unresolvable" line must appear exactly once.
printf '%s' "2" > "$state/liveness_rc"
printf '%s' "0" > "$state/check_count"
printf '%s' "0" > "$state/release_called"
start_ms="$(date +%s%3N)"
if out="$(env $stub_env "$mill_dispatch" \
        --brief "$brief" \
        --role product-engineer \
        --agent command-code \
        --name supervisor-liveness-slug \
        --title "supervisor liveness test" \
        --writes plan.md \
        --timeout-ms 2000 2>&1)"; then
    rc=0
else
    rc=$?
fi
end_ms="$(date +%s%3N)"
elapsed_ms=$((end_ms - start_ms))
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL [unresolvable]: expected non-zero exit (deadline spent), got exit 0" >&2
    echo "$out" >&2
    failures=1
fi
unresolvable_count="$(grep -c "unresolvable (2)" <<< "$out" || true)"
if [[ "$unresolvable_count" -ne 1 ]]; then
    echo "FAIL [unresolvable]: expected exactly one 'unresolvable (2)' line, got $unresolvable_count" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: unresolvable probe (exit 2) logged once and the loop kept waiting (${elapsed_ms} ms)"
fi

# --- Scenario 4: release rule intact for verdict 10, 30, 40 ------------------
# A stop on verdict 10 (finished), 30 (dead), or 40 (uncommitted) must NOT call
# worker-release. Scenario 1 already covered 30; here we check 10 and 40.
for verdict_rc in 10 40; do
    run_dispatch "$verdict_rc"
    release_called="$(cat "$state/release_called")"
    if [[ "$release_called" -ne 0 ]]; then
        echo "FAIL [verdict $verdict_rc]: worker-release was called — release rule violated" >&2
        echo "$out" >&2
        failures=1
    else
        echo "ok: verdict $verdict_rc stopped the loop without releasing the worker"
    fi
done

if [[ "$failures" -ne 0 ]]; then
    echo "supervisor-liveness: FAIL — see failures above" >&2
    exit 1
fi
echo "supervisor-liveness: PASS — dead stops within a slice, working stays quiet, exit 2 logs once, release rule intact"
exit 0
