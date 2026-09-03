---
name: command-code-headless
description: >-
  Drive Command Code (command-code) non-interactively from a terminal or CI,
  never in the interactive TUI. Use when about to run `command-code -p` (print /
  headless) for a script, a one-shot query, or any automated invocation, and use
  it first to check whether a session that looks idle is actually running. This
  skill records what each flag actually does when run, the JSON event-stream
  shape, the withheld-tool set, the exit codes you will see, the permission
  flags, and the model/config hazards that bite Mill dispatches. Prefer this
  over re-reading `--help`: every fact below is backed by a command run on this
  machine, with the raw output pasted in.
---

# Command Code headless (non-interactive) driving

Driving `command-code` as a script (`-p` / `--print`) is easy to get wrong because
several flags only reveal their real behaviour by running them: `-m` persists to
the global config, the JSON stream has two frame shapes, a different (smaller)
tool set is active, and `--max-turns` has a specific exit code when it fires.
This skill captures only what was observed by running the commands, on this box
(Command Code v1.45.0, Node v24.15.0, Linux x64, terminal "Orca", authenticated
as `antonygiomarxdev` / provider `Command Code`).

## Is command-code actually running? (the reliable signal)

This is the most important section in the skill. command-code fails in a way that
is visually identical to idling, and every status signal Orca gives you lies about
it. The only trustworthy evidence is whether terminal output is advancing — and
you have to measure that yourself.

### The failure mode: a 503 looks exactly like thinking

When command-code's provider fails, the session does not crash or announce itself.
It stops with `Error: 503 Service temporarily unavailable`, returns to its own
prompt, and prints `Type continue to try again`. That screen is indistinguishable
from a session that is merely thinking. On this machine it happened three times in
one session (observed directly: a `-p --output-format json` run printed nine
`api_retry` frames — delays 1000/1000/1600/3200/6400/10000/10000 ms — and a final
`run_error` with `subtype: "error"`, see § "--output-format json"). Nothing on the
screen separates dead from alive. You cannot tell by looking.

### Signals that lie

**Orca's dispatch record lies.** `orca orchestration worker-show --dispatch <id>`
reports `dispatch.status` and `last_failure`, and they are not reliable. Verified
against this session's own dispatch (`ctx_5d8ac89d52ee`): while the agent was running
and producing output, the dispatch record reported `status: failed` with
`last_failure: agent_prompt_stalled`. It did so the entire time the agent was
healthy, and it never corrected itself after the agent recovered. Treat the dispatch
record as metadata, never as proof of liveness.

```
$ orca orchestration worker-show --dispatch ctx_5d8ac89d52ee --json
dispatch.status: failed
dispatch.last_failure: agent_prompt_stalled
worker.state: failed
```

That was the record *while the worker was running fine*. If you trusted it, you
would have killed a healthy worker.

**`observation.status` "live" is not proof of work.** It only says a terminal
exists and is attached, not that the agent inside it is doing anything. A terminal
stuck on a 503'd prompt still reports `live`. Do not use it as evidence.

### The reliable signal: output advancing

The only signal that holds up is **the terminal's output cursor moving**. Read the
terminal twice, at least 20 seconds apart, and compare what changed. If nothing
changed, nothing was produced — regardless of what any status field says.

Two more things freeze exactly when the session dies, so they corroborate the
cursor check: the agent's own elapsed clock (e.g. the `Worked for 44s` counter in
the TUI, which stops incrementing), its accumulated spend, and the worktree diff
(no new files appear). All three stall together when the session is dead.

Runnable check — read twice, compare the tail (copy this):

```
# READ 1
$ orca terminal read --terminal <handle> --json
# wait at least 20 seconds — do NOT interact with the terminal
# READ 2
$ orca terminal read --terminal <handle> --json
# compare result.terminal.tail (or nextCursor) between the two reads
```

Evidence from this session (terminal `term_79bc3a2c-61e6-4069-bd1a-cbd5a71f100a`,
the command-code agent). Two reads, 26 seconds apart:

```
$ orca terminal read --terminal term_79bc3a2c-61e6-4069-bd1a-cbd5a71f100a --json
{ status: running, tail: [ ... "❯ Ask your question...", ... ] }
                                            ^^^ 16:35:13

$ orca terminal read --terminal term_79bc3a2c-61e6-4069-bd1a-cbd5a71f100a --json
{ status: running, tail: [ ... "❯ Ask your question...", ... ] }
                                            ^^^ 16:37:39  (26s later)
```

Both reads report `status: running`. Both show the identical tail — the agent
sitting at its `Ask your question...` prompt. The cursor advanced **zero** lines
in 26 seconds. If you trusted `status: running`, you would believe the agent was
working. The cursor comparison proves it produced nothing in that window. The
signal that says "live" and the signal that says "working" disagreed; the cursor
was right.

When the session IS working, the same two reads show different tails (new output
lines between them) and a moving elapsed clock. That is the only confirmation
worth trusting.

### Recovery

A 503'd session resumes if you send the literal text `continue` followed by an
enter (`orca terminal send --terminal <handle> --text "continue"`). Verified by
the operator: it resumes the stalled session. Note what it does **not** do: it does
not fix the underlying provider. The 503 will come back until the provider recovers,
so resuming is a stopgap, not a cure. If resuming does not get output moving again
within a couple of minutes, the provider is still down — stop resuming and wait.

Run state this skill was built against:

```
$ md5sum ~/.commandcode/config.json                 # before any command-code invocation
ee823c4bab9b21b7b79322ab0d5b690b  /home/ksante/.commandcode/config.json
$ cat ~/.commandcode/config.json
{
  "provider": "command-code",
  "installed": true,
  "model": "poolside/laguna-s-2.1-free",
  "firstMessageSent": true,
  "compactMode": "default",
  "defaultExportFormat": "md",
  "defaultShareGistFormat": "md",
  "imageVisionEnabled": true,
  "tasteLearning": false,
  "reasoningEffort": {
    "stealth/ox-alpha": "max"
  }
}
$ command-code --version
1.45.0
$ command-code status
✔ Authentication verified
✔ Authenticated as antonygiomarxdev
  Provider: Command Code
$ command-code --list-models | head -5
Available models  ·  67 models
... (67 models; the configured default poolside/laguna-s-2.1-free is a FREE open-weight model)
```

Every `-p` run in this skill is trivial, read-only, and carries `--no-session`
(as required for these evidence runs). The configured default model
`poolside/laguna-s-2.1-free` returned "Service temporarily unavailable" on first
contact, so the evidence runs below use `meituan/longcat-2.0:free` (also a FREE
model) via `-m`; see § "The model and the config.json hazard (#178)" for why that
`-m` use is itself the thing to watch.

## Headless print: `-p` / `--print`

`-p [query]` runs one query, prints the response to stdout, and exits. It is
non-interactive — no TUI, no keyboard shortcuts. The query is passed as the
argument:

```
$ command-code -p "say hi" --no-session --model meituan/longcat-2.0:free
Hi! How can I help you?
$ echo "exit=$?"
exit=0
```

Piped stdin is auto-detected when no query argument is given; if stdin is a TTY
and no query is given, it errors (docs: "If stdin is a TTY and no query is given,
it exits with an error"). For scripted calls, always quote the query and prefer
`--no-session` so the run is not persisted to session history.

## `--output-format json` (the stream shape)

`--output-format json` turns print mode into a machine-readable stream. The
reference docs call it "NDJSON event stream + final result line". Running one
confirms exactly two shapes, line by line:

1. **Event frames**, one per `AgentEvent`. Each is
   `{"type":"event","event":{...}}` where the inner object has a `type`.
2. **One final result line**, always last:
   `{"type":"result","subtype":"success"|"error"|"max_turns", ...}`.

Exact command run — a query that forces a tool call so the tool-related events
appear:

```
command-code -p "Use the read_directory tool on /home/ksante/orca/workspaces/mill/mill-cc-headless-skill and list the entries it returns." --no-session --output-format json --model meituan/longcat-2.0:free --max-turns 5
```

`exit=0`. The stream is long (~600 lines, dominated by `thinking_delta`/`text_delta`
fragments). Below are the verbatim structural lines; one sample `thinking_delta`
and one sample `text_delta` are shown, and the repeated `model_trace` lines are
omitted for readability — every line shown is pasted raw from stdout.

```
{"type":"event","event":{"type":"run_start","sessionId":"5b027a82-f565-457a-bc39-6ee9259524e3"}}
{"type":"event","event":{"type":"turn_start","turnNumber":1}}
{"type":"event","event":{"type":"message_start"}}
{"type":"event","event":{"type":"model_request_start","model":"meituan/LongCat-2.0:free"}}
{"type":"event","event":{"type":"model_trace","traceId":"4fa0d665ce376f47633f33c595308670"}}
{"type":"event","event":{"type":"thinking_start"}}
{"type":"event","event":{"type":"thinking_delta","delta":"\n"}}
{"type":"event","event":{"type":"thinking_end","text":"\nThe user wants me to list the directory contents.\n"}}
{"type":"event","event":{"type":"message_update","content":[{"type":"thinking","thinking":"\nThe user wants me to list the directory contents.\n","signature":""},{"type":"tool_use","id":"call_6970b0080f1c47089f734aef","name":"read_directory","input":{"path":"/home/ksante/orca/workspaces/mill/mill-cc-headless-skill"}}]}}
{"type":"event","event":{"type":"model_request_end","model":"meituan/LongCat-2.0:free","usage":{"inputTokens":20976,"outputTokens":44,"cacheReadTokens":9856,"cacheWriteTokens":0},"stopReason":"tool_calls"}}
{"type":"event","event":{"type":"message_end","content":[{"type":"thinking","thinking":"\nThe user wants me to list the directory contents.\n","signature":""},{"type":"tool_use","id":"call_6970b0080f1c47089f734aef","name":"read_directory","input":{"path":"/home/ksante/orca/workspaces/mill/mill-cc-headless-skill"}}]}}
{"type":"event","event":{"type":"tool_queued","toolCallId":"call_6970b0080f1c47089f734aef","toolName":"read_directory","input":{"path":"/home/ksante/orca/workspaces/mill/mill-cc-headless-skill"}}}
{"type":"event","event":{"type":"tool_running","toolCallId":"call_6970b0080f1c47089f734aef","toolName":"read_directory","description":null}}
{"type":"event","event":{"type":"tool_completed","toolCallId":"call_6970b0080f1c47089f734aef","toolName":"read_directory","result":[{"type":"text","text":"Found 22 items (11 dirs, 11 files)\nDirectories:\n  .agents/\n  .claude/\n  .claude-plugin/\n  .codex-plugin/\n  .cursor-plugin/\n  .github/\n  .mill/\n  .omp/\n  docs/\n  local/\n  test/\nFiles:\n  .git\n  .gitignore\n  AGENTS.md\n  ARCHITECTURE.md\n  CLAUDE.md\n  CONTRIBUTING.md\n  INSTALL.md\n  LESSONS.md\n  MEMORY.md\n  README.md\n  package.json"}],"deferred":false}}
{"type":"event","event":{"type":"turn_end","turnNumber":1,"hadToolCalls":true,"usage":{"inputTokens":20976,"outputTokens":44,"cacheReadTokens":9856,"cacheWriteTokens":0}}}
{"type":"event","event":{"type":"turn_start","turnNumber":2}}
{"type":"event","event":{"type":"model_request_end","model":"meituan/LongCat-2.0:free","usage":{"inputTokens":21151,"outputTokens":178,"cacheReadTokens":9856,"cacheWriteTokens":0},"stopReason":"stop"}}
{"type":"event","event":{"type":"turn_end","turnNumber":2,"hadToolCalls":false,"usage":{"inputTokens":21151,"outputTokens":178,"cacheReadTokens":9856,"cacheWriteTokens":0}}}
{"type":"event","event":{"type":"run_end","result":{"finalText":"…<same as the final result line below>…","stopReason":"end_turn","turnCount":2,"usage":{"inputTokens":42127,"outputTokens":222,"cacheReadTokens":19712,"cacheWriteTokens":0},"systemPromptTokens":null,"nextState":{…<session transcript + workspace modState omitted>…}}}}
```

The exact final result line (verbatim, last line of stdout):

```
{"type":"result","subtype":"success","sessionId":"5b027a82-f565-457a-bc39-6ee9259524e3","stopReason":"end_turn","usage":{"inputTokens":42127,"outputTokens":222,"cacheReadTokens":19712,"cacheWriteTokens":0},"durationMs":13789,"finalText":"Here are the contents of `/home/ksante/orca/workspaces/mill/mill-cc-headless-skill`:\n\n**Directories (11):**\n- `.agents/`\n- `.claude/`\n- `.claude-plugin/`\n- `.codex-plugin/`\n- `.cursor-plugin/`\n- `.github/`\n- `.mill/`\n- `.omp/`\n- `docs/`\n- `local/`\n- `test/`\n\n**Files (11):**\n- `.git`\n- `.gitignore`\n- `AGENTS.md`\n- `ARCHITECTURE.md`\n- `CLAUDE.md`\n- `CONTRIBUTING.md`\n- `INSTALL.md`\n- `LESSONS.md`\n- `MEMORY.md`\n- `README.md`\n- `package.json`"}
```

A second `-p --output-format json` run against the configured default model
`poolside/laguna-s-2.1-free` hit a transient backend failure and produced the
**error** result line (verbatim), confirming the `subtype:"error"` shape and the
`error` field. `model_request_start` there reported
`"model":"poolside/laguna-s-2.1-free"` — i.e. with **no** `-m` flag, the session
model is read from `~/.commandcode/config.json` (more in § "The model").

```
{"type":"event","event":{"type":"run_start","sessionId":"3afdb12b-3185-4935-a8e5-c43bb6af18d0"}}
{"type":"event","event":{"type":"turn_start","turnNumber":1}}
{"type":"event","event":{"type":"message_start"}}
{"type":"event","event":{"type":"model_request_start","model":"poolside/laguna-s-2.1-free"}}
{"type":"event","event":{"type":"api_retry","attempt":3,"error":"Service temporarily unavailable. Please try again shortly.","delayMs":1000}}
{"type":"event","event":{"type":"api_retry","attempt":4","error":"Service temporarily unavailable. Please try again shortly.","delayMs":1000}}
{"type":"event","event":{"type":"api_retry","attempt":5","error":"Service temporarily unavailable. Please try again shortly.","delayMs":1600}}
{"type":"event","event":{"type":"api_retry","attempt":6","error":"Service temporarily unavailable. Please try again shortly.","delayMs":3200}}
{"type":"event","event":{"type":"api_retry","attempt":7","error":"Service temporarily unavailable. Please try again shortly.","delayMs":6400}}
{"type":"event","event":{"type":"api_retry","attempt":8","error":"Service temporarily unavailable. Please try again shortly.","delayMs":10000}}
{"type":"event","event":{"type":"api_retry","attempt":9","error":"Service temporarily unavailable. Please try again shortly.","delayMs":10000}}
{"type":"event","event":{"type":"run_error","error":{"name":"TransportError","message":"Service temporarily unavailable. Please try again shortly."}}}
{"type":"event","event":{"type":"run_end","result":{"finalText":"","stopReason":"run_error","turnCount":1,"usage":{"inputTokens":0,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0},"systemPromptTokens":null,"nextState":{…<transcript + modState omitted>…}}}}
{"type":"result","subtype":"error","sessionId":"3afdb12b-3185-4935-a8e5-c43bb6af18d0","usage":{"inputTokens":0,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0},"durationMs":45670,"finalText":"","error":"Error: The API server encountered an error. Please try again later."}
Error: The API server encountered an error. Please try again later.
```

(interleaved `model_trace` lines omitted; the human-readable `Error: …` line on
stderr is the last line above.)

Parsing note: the docs say treat `sessionId` and `stopReason` as **optional** — an
early failure (bad input, auth failure) emits `subtype:"error"` with *neither*
field, so a script that indexes them unconditionally breaks on exactly the case it
most needs to handle. Parse line by line and ignore unknown event types.

## Tools: what a headless run withholds

Two separate mechanisms gate tools in headless mode:

- **Permission blocking** — file edits/writes and shell commands are *available*
  but denied at runtime by the permission engine (see § Permissions). `--yolo`
  lifts the denial.
- **Tool withholding** — a small set of tools are *disabled entirely* in headless
  runs because they only make sense with a person at the keyboard. These are
  re-enabled with `--tools-all` (or `CMD_TOOLS_ALL_ENABLE=true`) or selectively
  with `--tools-enable <names>`.

The CLI names the withheld set itself. The most reliable way to dump it on this
machine is to ask `--tools-enable` for a name that is **not** withheld and read
the resulting CLI warning, which prints the complete withheld list
(`HEADLESS_EXCLUDED_TOOLS` in the bundle):

```
$ command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --tools-enable fake_tool_xyz
exit=0
$ cat stderr
--tools-enable: ignoring fake_tool_xyz — not withheld by this run. It only re-enables the tools a headless run holds back: ask_user_question, enter_plan_mode, exit_plan_mode, plan_review, todo_write, cron_create, cron_list, cron_delete, taste.
```

So the withheld set (9 tools), confirmed by the CLI's own output:

```
ask_user_question, enter_plan_mode, exit_plan_mode, plan_review,
todo_write, cron_create, cron_list, cron_delete, taste
```

Cross-checked against the bundled reference docs
(`…/dist/bundled/command-code-knowledge/reference/headless.md`), which state the
same category ("tools not useful in headless context… asking you a question or
opening a plan for approval") and name `ask_user_question` and `todo_write` in
the `--tools-enable` example. The CLI dump above is the authoritative, complete
list — it matches that description (question-asking → `ask_user_question`;
plan approval → `enter_plan_mode`/`exit_plan_mode`/`plan_review`; plus
`todo_write`, `cron_*`, `taste`).

Contrast — passing a *real* withheld name produces **no** "ignoring" warning (it
is accepted, not ignored):

```
$ command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --tools-enable todo_write,cron_list
exit=0
$ cat stderr
(empty — todo_write and cron_list are withheld, so they are accepted, not ignored)
```

A secondary (less reliable) method is to ask the model to list its own tools in
default vs `--tools-all` mode. That *roughly* matched (it surfaced
`ask_user_question`/`todo_write` but **missed `exit_plan_mode`**), so the CLI
warning above is the source of truth, not the model's self-report.

The withheld tools are a subset of Command Code's tool names. The friendly-name
mapping (from the bundled permissions reference) is: `Shell`→`shell_command`,
`monitor_command`, `kill_shell`; `Read`→`read_file`, `read_directory`, `glob`,
`grep`; `Edit`→`edit_file`, `write_file`; `WebFetch`→`web_fetch`;
`WebSearch`→`web_search`. None of those are withheld — reads run freely in a
headless run (the `read_directory` call in § "`--output-format json`" succeeded
without `--yolo`).

## `--max-turns`

Default is **100** turns. Hitting the cap prints a warning to stderr, returns the
partial response, and exits **8**. Verified by forcing a tool call under
`--max-turns 1` (the first turn makes the tool call; there is no second turn to
answer, so the loop caps):

```
$ command-code -p "Call the read_directory tool on /home/ksante/orca/workspaces/mill/mill-cc-headless-skill and report how many entries it returns." --no-session --model meituan/longcat-2.0:free --max-turns 1
exit=8
$ cat stderr
Warning: Reached maximum conversation turns (1). The response may be incomplete. Retry with --max-turns 2 to extend the budget.
$ cat stdout
(empty — partial response, no final answer)
```

A query that needs **no** tool call (e.g.
`command-code -p "say the word hello" --no-session --model meituan/longcat-2.0:free --max-turns 1`)
completes in one turn and exits **0** — confirming the cap is only hit when a turn
is cut short, not on a single self-contained answer.

## Exit codes observed

Cleanly captured on this machine:

| Exit | What produced it here | Notes |
|------|-----------------------|-------|
| `0`  | successful `-p` runs (e.g. `say hi`, the JSON run above) | `EXIT_SUCCESS` |
| `1`  | `command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --output-format xml` → `error: option '--output-format <format>' argument 'xml' is invalid. Allowed choices are text, json.` | client-side parse error, **no model call** |
| `8`  | `--max-turns 1` cap (see § `--max-turns`) | `EXIT_MAX_TURNS_REACHED` |

Also encountered: the default model `poolside/laguna-s-2.1-free` returned
`Service temporarily unavailable` (retries with backoff, delays
1000/1000/1600/3200/6400/10000/10000 ms, attempts 3→9), emitting
`run_error`/`TransportError` and the `subtype:"error"` result line. The bundled
reference maps API-server errors to **`7` (`EXIT_SERVER_ERROR`)**; a later re-run
of that default model succeeded (exit 0), so the outage was transient. (Its
exact exit code was not cleanly captured in that first attempt — a shell-pipe
status bug on the capturing command obscured it; the error result line and the
stderr `Error:` line are still pasted above as observed output.)

The full reference table (not all observed) is: `0` success, `1` general error,
`3` not authenticated, `4` permission denied, `5` rate limit, `6` network
failure, `7` API server error (5xx), `8` max turns, `9` no response, `10`
insufficient credits, `130` interrupted (SIGINT/SIGTERM).

## Permissions: `--yolo`, `--permission-mode`, `--auto-accept`, `-t` / `--trust`

Headless mode separates "withheld" (§ Tools) from "permission-denied". By default,
**file edits/writes and shell commands are denied** by the permission engine;
file reads, grep, and glob are allowed. `--yolo` (alias
`--dangerously-skip-permissions`) allows everything; `--auto-accept` is shorthand
for `--permission-mode auto-accept`; `--permission-mode` takes `standard`,
`plan`, `auto-accept`, or `dont-ask` (the fail-closed CI mode).

What a headless run **actually needs**, observed: a read-only `-p` query needs
**none** of `-t`, `--yolo`, `--auto-accept`, or `--permission-mode`. The run below
completed with exit 0 and empty stderr using only `-p --no-session` (plus
`--skip-onboarding`, `--yolo`, and `--tools-enable` to prove those flags are
accepted too):

```
$ command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --skip-onboarding --yolo --tools-enable todo_write,cron_list
Hi! How can I help you?
$ echo "exit=$?"
exit=0
$ cat stderr
(empty — no permission prompt, no trust prompt, no "ignoring" warning)
```

So for read-only automation: none of the permission flags are required. Use
`--yolo` **only** when the task must write/edit/run shell commands (default
headless blocks those with `EXIT_PERMISSION_DENIED` = 4). `-t`/`--trust` skips
the initial **workspace-trust prompt** — it is only needed on a first run in an
untrusted workspace; for a read-only `-p` run it is redundant (observed: no trust
prompt blocked these runs). Note: do **not** auto-answer a workspace-trust prompt
interactively — in the Orca launcher that prompt is resolved by the operator
sending the bare trust key (`orca terminal send --text 'a'`); see
`.mill/checks/mill-dispatch` (blockedReason `codex-trust-workspace`).

`--permission-mode dont-ask` (a.k.a. `defaultMode:"dont-ask"` in
`.commandcode/settings.json`) is the CI posture: run only what policy
pre-approves, deny the rest — it fails closed instead of hanging on a prompt no
one answers.

Note on the "always-allowed" tools in the permissions reference
(`ask_user_question`, `agent`, `run_command` skip the deny/ask lists): in
*headless* mode `ask_user_question` is additionally **withheld** (§ Tools) unless
`--tools-all`/`--tools-enable ask_user_question` is passed — so "never prompts"
does not mean "available headless".

## The model and the `config.json` hazard (#178)

`-m`/`--model` overrides the model for a session... and that is the danger: it is
**not** a purely local override. Command Code rewrites `~/.commandcode/config.json`
itself, and a `-p --model X` run persists `X` there — observed directly:

```
# before the -m run:
$ md5sum ~/.commandcode/config.json
ee823c4bab9b21b7b79322ab0d5b690b  /home/ksante/.commandcode/config.json   # "model": "poolside/laguna-s-2.1-free"

$ command-code -p "say hi" --no-session --model meituan/longcat-2.0:free   # run with -m
exit=0

# after the -m run:
$ cat ~/.commandcode/config.json | grep '"model"'
  "model": "meituan/LongCat-2.0:free"                                    # ← rewritten by command-code
$ md5sum ~/.commandcode/config.json
ebdb408442244241a9bc1b23e02091b4                                         # ← changed from ee823c4…
```

Note the persisted value is the **normalized** name (`meituan/LongCat-2.0:free`,
not the lowercase `meituan/longcat-2.0:free` I passed) — confirming Command Code
itself wrote it. Because the global config has one `model` field, **two concurrent
command-code runs cannot use different models** — Mill issue #178.

After noticing this change I proposed restoring the config to its pre-run bytes,
but the operator explicitly instructed me to leave it at LongCat: *"yo lo hice
intencional, laguna se esta cayendo"* ("I did it intentionally, Laguna is
failing"). So `~/.commandcode/config.json` was left at LongCat — final
`md5sum` `ebdb408442244241a9bc1b23e02091b4`, model `meituan/LongCat-2.0:free` —
which is the operator's stated preference, not the pre-run hash. (If the operator
had not given that instruction, the correct action would be to restore the
original bytes so the pre-run and post-run md5sums match.)

Implication for agents: never rely on `-m` to isolate models across concurrent
dispatches. To run a specific model, set it in `~/.commandcode/config.json`
first and say so — exactly what the Orca launcher path does (§ "Orca launcher").

`--list-models` enumerates the 67 available models (provider, name, one-line
description); `--model` may be passed the full id or just the short name after
the last `/` (e.g. `--model kimi-k2.5`).

## `--skip-onboarding` and `--no-session`

- `--no-session` — keep the run in memory only; do not persist the transcript to
  `~/.commandcode/` session history. Used on every evidence run above. Automated
  / throwaway `-p` runs should always pass it so they never pollute the
  interactive `/resume` menu (headless sessions are hidden from that menu by
  default anyway, but `--no-session` makes the intent explicit and leaves no
  on-disk session at all).
- `--skip-onboarding` — skip taste onboarding. Use for CI / fully automated runs.
  Observed accepted without error (exit 0) when combined with `--yolo` above; the
  bundled headless reference shell-script example is `cmd -p --skip-onboarding`.

## Orca launcher: `worker-start --agent command-code` rejects `--model`

Orca's dispatch launcher does **not** accept `--model` for the command-code agent.
Per `.mill/agents.example` and `INSTALL.md` (Mill's own tracked docs, read from
the repository):

> `.mill/agents.example` (line 36): **`command-code`** — `submit: self` —
> `worker-start --agent command-code` rejects `--model`. Its model comes from the
> global `~/.commandcode/config.json`, which command-code rewrites itself, so two
> dispatches on different models cannot run concurrently against it (#178). Set
> the model there first, and say so.

> `INSTALL.md` (lines 46-48): `command-code` reads a global
> `~/.commandcode/config.json` that it rewrites itself, and `claude` and `cursor`
> accept `--model` at launch.

`mill-dispatch` itself never passes `--model` — it calls
`orca orchestration worker-start --task <id> --worktree new-top-level --name
<slug> --agent command-code --setup run --json` (see
`.mill/checks/mill-dispatch` lines 149-155) and relies on Orca to reject any
`--model` for this agent. (This skill does not run `worker-start` — workers never
dispatch. The rejection is stated as Orca's launcher behaviour.) Contrast with
`claude` and `cursor`, which **do** accept `--model` at launch.

So in the Orca dispatch path the model is chosen by **reading/writing
`~/.commandcode/config.json`** (set it before dispatching), not by a launch flag.

## Verified commands (raw, for copy-paste)

```bash
# 1. The JSON stream shape (success, with a tool call):
command-code -p "Use the read_directory tool on /home/ksante/orca/workspaces/mill/mill-cc-headless-skill and list the entries it returns." \
  --no-session --output-format json --model meituan/longcat-2.0:free --max-turns 5

# 2. The cap-hit (exit 8):
command-code -p "Call the read_directory tool on /home/ksante/orca/workspaces/mill/mill-cc-headless-skill and report how many entries it returns." \
  --no-session --model meituan/longcat-2.0:free --max-turns 1

# 3. The withheld-tool dump (stderr prints the full 9-tool list):
command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --tools-enable fake_tool_xyz

# 4. Accept a real withheld name (no "ignoring" warning):
command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --tools-enable todo_write,cron_list

# 5. Client parse error (exit 1, no model call):
command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --output-format xml

# 6. Read-only run needs no permission/trust flags (exit 0):
command-code -p "say hi" --no-session --model meituan/longcat-2.0:free --skip-onboarding --yolo
```
