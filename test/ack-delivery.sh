#!/usr/bin/env bash
# ack-delivery test — the wait loop acknowledges a processed Delivery.
#
# mill-dispatch's wait loop blocks on `orca orchestration check --wait`, which
# returns the bound Run's oldest unacknowledged Delivery and replays that exact
# batch until `--ack <delivery_id>`. Without acknowledgement the same batch is
# replayed to every waiter forever and the inbox never drains (#213, #211,
# #210). The fix acknowledges each processed Delivery by its id, after the
# release decision, and never acknowledges a batch the loop did not process.
#
# This test drives the dispatch loop against a STUB `orca` on PATH (never the
# real one). The stub's check --wait serves a Delivery carrying a settled
# worker_done, records every --ack call, and exposes the id it was acked with.
# It takes the script under test as $1 and defaults to .mill/checks/mill-dispatch
# (the working copy), so the same test doubles as its own negative control:
# point it at the pre-change script and the "acknowledgement issued" case must
# fail:
#   git show HEAD:.mill/checks/mill-dispatch > /tmp/pre-ack \
#       && chmod +x /tmp/pre-ack \
#       && bash test/ack-delivery.sh /tmp/pre-ack
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

# Stub mill-liveness: exits 0 (working) so the loop settles only via the
# worker_done the stub serves, not via a liveness verdict.
cat > "$checks/mill-liveness" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
echo "stub mill-liveness: dispatch=$2 rc=0"
exit 0
STUB
chmod +x "$checks/mill-liveness"

# Preflight requires a git work tree and a brief file.
git -C "$tmp" init -q
brief="$tmp/brief.md"
printf '%s\n' "# brief" > "$brief"

# State dir the stub uses to remember ids, count check --wait calls, record
# whether worker-release was called, and record every --ack call.
state="$tmp/state"
mkdir -p "$state"
printf '%s' "0" > "$state/check_count"
printf '%s' "0" > "$state/release_called"
printf '%s' "0" > "$state/ack_count"
printf '%s' ""  > "$state/ack_id"

# The stub orca. It is the only `orca` on PATH for the run.
# mill-preflight refuses any executable beginning with `#!` (it is the
# screen reader, not Orca), so the stub must be a real ELF binary: a small
# C wrapper that execs the bash logic.
mkdir -p "$tmp/bin" "$tmp/stub"

# make_orca_sh writes the stub orca.sh. The first argument is the check
# handler body — the code that runs for a non-ack `check` call. This lets the
# two scenarios serve different responses (a settled worker_done vs count 0)
# without duplicating the whole stub.
make_orca_sh() {
    local check_body="$1"
    cat > "$tmp/stub/orca.sh" <<STUB
#!/usr/bin/env bash
set -euo pipefail
state="\${ORCA_STUB_STATE:?ORCA_STUB_STATE not set}"
payload_json() {
    local dispatch_id="\$1" task_id="\$2" outcome="\${3:-succeeded}"
    jq -n --arg d "\$dispatch_id" --arg t "\$task_id" --arg o "\$outcome" \
        '{dispatchId:\$d,taskId:\$t,outcome:\$o}'
}
# scan_for_ack scans argv for --ack and prints the id that follows it, or
# nothing if --ack is absent. check takes --ack <id> --json with the id in
# its own argument; we must not confuse --types/--wait/--timeout-ms with it.
scan_for_ack() {
    local seen_ack=0
    for a in "\$@"; do
        if [[ "\$seen_ack" -eq 1 ]]; then
            printf '%s' "\$a"
            return 0
        fi
        [[ "\$a" == "--ack" ]] && seen_ack=1
    done
    return 1
}
case "\$1" in
    status)
        echo "runtimeReachable: true"
        ;;
    orchestration)
        case "\$2" in
            task-create)
                task_id="task_\$(head -c 6 /dev/urandom | xxd -p)"
                printf '%s' "\$task_id" > "\$state/task_id"
                jq -n --arg id "\$task_id" '{ok:true,result:{task:{id:\$id}}}'
                ;;
            worker-start)
                dispatch_id="ctx_\$(head -c 6 /dev/urandom | xxd -p)"
                printf '%s' "\$dispatch_id" > "\$state/dispatch_id"
                jq -n --arg d "\$dispatch_id" '{ok:true,result:{dispatchId:\$d,state:"ready"}}'
                ;;
            check)
                ack_id="\$(scan_for_ack "\${@:3}" || true)"
                if [[ -n "\$ack_id" ]]; then
                    prev="\$(cat "\$state/ack_count")"
                    printf '%s' "\$((prev + 1))" > "\$state/ack_count"
                    printf '%s' "\$ack_id" > "\$state/ack_id"
                    jq -n '{ok:true,result:{ack:true}}'
                    return 0
                fi
                $check_body
                ;;
            inbox)
                jq -n '{ok:true,result:{messages:[]}}'
                ;;
            worker-release)
                printf '%s' "1" > "\$state/release_called"
                jq -n '{ok:true,result:{state:"released"}}'
                ;;
            *)
                echo "stub orca: unknown orchestration subcommand: \$2" >&2
                exit 2
                ;;
        esac
        ;;
    terminal)
        jq -n '{ok:true,result:{}}'
        ;;
    *)
        echo "stub orca: unknown command: \$1" >&2
        exit 2
        ;;
esac
STUB
}

# Scenario 1 check body: serve a settled worker_done for this dispatch. The
# delivery id (del_1) is distinct from the message id (msg_1) and the
# dispatch id — the loop must ack by the delivery id.
make_orca_sh 'count="$(cat "$state/check_count")"
count=$((count + 1))
printf "%s" "$count" > "$state/check_count"
dispatch_id="$(cat "$state/dispatch_id")"
task_id="$(cat "$state/task_id")"
payload="$(payload_json "$dispatch_id" "$task_id")"
jq -n --arg p "$payload" --arg i "msg_$count" --arg d "del_$count" \
    "{ok:true,result:{count:1,messages:[{id:\$i,type:\"worker_done\",subject:\"worker done\",body:\"summary\",payload:\$p}],deliveryId:\$d}}"'

chmod +x "$tmp/stub/orca.sh"

# Compile a tiny ELF wrapper that execs the bash stub.
cat > "$tmp/stub/orca.c" <<'EOF'
#include <unistd.h>
#include <stdlib.h>
int main(int argc, char *argv[]) {
    char *path = getenv("ORCA_STUB_BIN");
    if (!path) { return 127; }
    argv[0] = "orca";
    execv(path, argv);
    return 127;
}
EOF
export ORCA_STUB_BIN="$tmp/stub/orca.sh"
cc -o "$tmp/bin/orca" "$tmp/stub/orca.c"
chmod +x "$tmp/bin/orca"

mill_dispatch="$checks/mill-dispatch"
# ORCA_WORKSPACE_ID simulates an Orca-managed terminal: the resolver then
# selects bare `orca` (found via the stub on PATH) instead of `orca-ide`.
stub_env="PATH=$tmp/bin:$PATH ORCA_STUB_STATE=$state ORCA_TERMINAL_HANDLE=term_test_handle ORCA_WORKSPACE_ID=ws_test"

# Per-run wall-clock bound, in seconds. A healthy run finishes in well under a
# second because the stub returns immediately; a script-under-test that never
# stops is killed fast. Exit 124 from `timeout` is treated as a failure.
RUN_BOUND_SECS=15

# run_dispatch runs the script-under-test, returning its output in $out and its
# exit code in $rc.
run_dispatch() {
    if out="$(env $stub_env timeout "$RUN_BOUND_SECS" "$mill_dispatch" \
            --brief "$brief" \
            --role product-engineer \
            --agent command-code \
            --name ack-delivery-slug \
            --title "ack delivery test" \
            --writes plan.md \
            --timeout-ms 2000 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
}

failures=0

# --- Scenario 1: acknowledgement is issued after a processed Delivery --------
# The stub serves a settled worker_done. The loop must process it, release the
# worker, and then acknowledge the Delivery. The ack must carry the delivery
# id (del_1), not the message id (msg_1) or the dispatch id.
run_dispatch
if [[ "$rc" -eq 124 ]]; then
    echo "FAIL [ack-issued]: script under test did not stop within ${RUN_BOUND_SECS}s." >&2
    echo "$out" >&2
    failures=1
fi
if [[ "$rc" -ne 0 ]]; then
    echo "FAIL [ack-issued]: expected exit 0 (reached release), got exit $rc" >&2
    echo "$out" >&2
    failures=1
fi
ack_count="$(cat "$state/ack_count")"
if [[ "$ack_count" -lt 1 ]]; then
    echo "FAIL [ack-issued]: no acknowledgement was issued (ack_count=$ack_count)" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: acknowledgement was issued (ack_count=$ack_count)"
fi
ack_id="$(cat "$state/ack_id")"
if [[ "$ack_id" != "del_1" ]]; then
    echo "FAIL [ack-issued]: acknowledgement carried wrong id '$ack_id', expected delivery id 'del_1' (not message id 'msg_1' or dispatch id)" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: acknowledgement carried the delivery id 'del_1'"
fi
release_called="$(cat "$state/release_called")"
if [[ "$release_called" -ne 1 ]]; then
    echo "FAIL [ack-issued]: worker-release was not called" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: worker was released"
fi

# --- Scenario 2: no acknowledgement when the loop exits early without processing
# The stub's check --wait serves nothing (count 0) and the stub mill-liveness
# exits 30 (dead). The loop must exit on the liveness verdict without ever
# processing a Delivery, and therefore without acknowledging anything.
cat > "$checks/mill-liveness" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
echo "stub mill-liveness: dispatch=$2 rc=30"
exit 30
STUB
chmod +x "$checks/mill-liveness"

# Reset state for this scenario.
printf '%s' "0" > "$state/check_count"
printf '%s' "0" > "$state/release_called"
printf '%s' "0" > "$state/ack_count"
printf '%s' ""  > "$state/ack_id"

# Replace the check handler so it never serves a Delivery: every check --wait
# returns count 0, so the loop can only exit via the liveness verdict.
make_orca_sh 'count="$(cat "$state/check_count")"
count=$((count + 1))
printf "%s" "$count" > "$state/check_count"
jq -n "{ok:true,result:{count:0,messages:[]}}"'

# Use a short deadline so the test is fast: the loop runs one slice, the
# liveness probe returns 30 (dead), and the loop exits.
run_dispatch
if [[ "$rc" -eq 124 ]]; then
    echo "FAIL [no-ack-early-exit]: script under test did not stop within ${RUN_BOUND_SECS}s." >&2
    echo "$out" >&2
    failures=1
fi
if [[ "$rc" -eq 0 ]]; then
    echo "FAIL [no-ack-early-exit]: expected non-zero exit (dead worker must not release), got exit 0" >&2
    echo "$out" >&2
    failures=1
fi
ack_count="$(cat "$state/ack_count")"
if [[ "$ack_count" -ne 0 ]]; then
    echo "FAIL [no-ack-early-exit]: acknowledgement was issued on early exit (ack_count=$ack_count) — a Delivery the loop did not process must not be acked" >&2
    echo "$out" >&2
    failures=1
else
    echo "ok: no acknowledgement was issued when the loop exited early without processing"
fi
if ! grep -q "WORKER DEAD" <<< "$out"; then
    echo "FAIL [no-ack-early-exit]: loop did not report a dead worker" >&2
    echo "$out" >&2
    failures=1
fi

if [[ "$failures" -ne 0 ]]; then
    echo "ack-delivery: FAIL — see failures above" >&2
    exit 1
fi
echo "ack-delivery: PASS — ack after processed Delivery (by delivery id), no ack on early exit"
exit 0
