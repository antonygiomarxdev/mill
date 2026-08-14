# Adapter Research: OpenCode & CommandCode Headless APIs

> **Date:** 2026-08-09  
> **Method:** Official docs (`commandcode.ai/docs/headless`, `commandcode.ai/docs/reference/cli`), web searches, AI grounding.  
> Neither CLI binary is installed on this system; all findings from primary docs unless marked `[INFERRED]`.

---

## 1. OpenCode Headless API

**Binary:** `opencode`  
**Homepage:** `opencode.ai`  
**Source:** [`github.com/anomalyco/opencode`](https://github.com/anomalyco/opencode)  
**Language:** Go

### 1.1 Spawning an Agent with a Prompt

```bash
# Direct argument
opencode run "Explain how closures work in JavaScript"

# Piped stdin
cat error.log | opencode run "Explain this error"
```

- `run` is the headless/one-shot subcommand. It executes the prompt, outputs the result, and exits.
- `[VERIFIED]` from docs/web: no TUI launched; prompt is positional or piped.
- `[INFERRED]` If both stdin and an argument are provided, behavior is undocumented — likely argument takes precedence or they are concatenated.

### 1.2 Key Flags

| Flag | Short | Description | Source |
|------|-------|-------------|--------|
| `--model` | `-m` | Model as `provider/model` (e.g. `anthropic/claude-4.5-sonnet`) | [VERIFIED] docs |
| `--agent` | | Agent configuration/profile to use | [VERIFIED] docs |
| `--auto` | | Auto-approve permissions not explicitly denied | [VERIFIED] docs |
| `--file` | `-f` | Attach files to prompt context (repeatable) | [VERIFIED] docs |
| `--format json` | | Machine-readable JSON output | [VERIFIED] docs |
| `--continue` | `-c` | Resume the most recent session | [VERIFIED] docs |
| `--session` | `-s` | Resume a specific session by ID | [VERIFIED] docs |
| `--fork` | | Fork the session (with `-c` or `-s`) | [VERIFIED] docs |
| `--dir` | | Run in a specific directory context | [VERIFIED] docs (experimental) |
| `--variant` | | Provider-specific reasoning effort (e.g. `high`) | [VERIFIED] docs |
| `--pure` | | Run without external plugins | [VERIFIED] docs |
| `--title` | | Session display title | [VERIFIED] docs |
| `--share` | | Publish/share session | [VERIFIED] docs |
| `--attach` | | Connect to running `opencode serve` instance (e.g. `http://localhost:4096`) | [VERIFIED] docs |
| `--log-level` | | `DEBUG`, `INFO`, `WARN`, `ERROR` | [VERIFIED] docs |
| `--print-logs` | | Redirect logs to stderr (clean stdout) | [VERIFIED] docs |

### 1.3 Output Format

- **Default:** Plain text response on stdout.
- **`--format json`:** Machine-readable JSON. `[INFERRED]` exact schema not documented in available sources; likely includes session metadata and the final response text.
- **Stderr:** Progress/logs when `--print-logs` is set; errors always to stderr.
- `[INFERRED]` No documented NDJSON streaming like CommandCode; likely a single JSON object at completion.

### 1.4 Session Handling

- **Session persistence:** Sessions stored in `~/.local/share/opencode/` (macOS/Linux).
- **Continue:** `opencode run -c "next prompt"` resumes the most recent session.
- **Specific session:** `opencode run -s <session-id> "next prompt"`.
- **Fork:** `opencode run -s <id> --fork "try alternative"` branches without mutating original.
- `[INFERRED]` Session IDs are UUIDs or similar; displayed at session start or via some `opencode` subcommand.

### 1.5 Worktree / Path

- `[INFERRED]` OpenCode is git-aware and walks up from CWD to find worktree roots.
- `--dir <path>` flag for explicit directory context (noted as experimental).
- Community plugins exist for git worktree management.
- `[INFERRED]` No native `-w`/`--worktree` flag equivalent to CommandCode.

### 1.6 Completion Detection

- **Exit code:** `0` on success; non-zero on failure. `[INFERRED]` specific error codes not exhaustively documented.
- Process exits when task completes (one-shot model).
- `[INFERRED]` No documented `--max-turns` equivalent; agent runs until it yields or errors.

### 1.7 Server Mode (Alternative)

- `opencode serve` starts a persistent HTTP server with OpenAPI 3.1 spec at `http://<host>:<port>/doc`.
- SSE streaming for real-time events.
- Optional HTTP basic auth via `OPENCODE_SERVER_PASSWORD`.
- `opencode run --attach http://localhost:4096` connects to a running server.
- Full programmatic API; can create sessions, send prompts, subscribe to events.

---

## 2. CommandCode Headless API

**Binary:** `cmd` (Windows: `cmdc`; also `command-code` universal)  
**Homepage:** `commandcode.ai`  
**Source:** [`github.com/CommandCodeAI/command-code`](https://github.com/CommandCodeAI/command-code)  
**Language:** Node.js/TypeScript

### 2.1 Spawning an Agent with a Prompt

```bash
# Direct argument
cmd -p "explain this file"

# Piped stdin (auto-detected when no query argument)
echo "explain this error" | cmd -p

# From file
cat prompt.txt | cmd -p
```

- `-p` / `--print` is the headless flag. The query is optional; if omitted and stdin is a TTY, exits with error.
- `[VERIFIED]` from official docs: stdin auto-detected; 30-second stdin timeout.
- Multi-word queries MUST be quoted.

### 2.2 Key Flags

| Flag | Short | Description | Source |
|------|-------|-------------|--------|
| `-p, --print [query]` | `-p` | Headless mode: execute query, output, exit | [VERIFIED] docs |
| `--output-format` | | `text` (default) or `json` (NDJSON stream) | [VERIFIED] docs |
| `-c, --continue` | `-c` | Resume most recent headless session in CWD | [VERIFIED] docs |
| `-r, --resume <id>` | `-r` | Resume specific headless session by ID | [VERIFIED] docs |
| `--session <path\|id>` | | Resume by transcript path (`.jsonl`) or session-id prefix | [VERIFIED] docs |
| `--fork-session` | | Fork with `--resume`/`--continue` | [VERIFIED] docs |
| `--no-session` | | Don't persist (in-memory only) | [VERIFIED] docs |
| `-n, --name` | `-n` | Session display name | [VERIFIED] docs |
| `-w, --worktree [name]` | `-w` | Managed git worktree: name, path, or `#PR`; auto-generates name if omitted | [VERIFIED] docs |
| `-m, --model` | `-m` | Model ID (see `--list-models`) | [VERIFIED] docs |
| `--max-turns` | | Agent turn cap (default 100); exit code 8 if hit | [VERIFIED] docs |
| `--effort` | | Reasoning effort: `low`, `medium`, `high`, … | [VERIFIED] docs |
| `--yolo` | | Bypass all permission prompts (alias: `--dangerously-skip-permissions`) | [VERIFIED] docs |
| `--auto-accept` | | Alias for `--permission-mode auto-accept` | [VERIFIED] docs |
| `--permission-mode` | | `default`, `plan`, `auto-accept`, `dont-ask` | [VERIFIED] docs |
| `--plan` | | Start in plan mode (read-only exploration) | [VERIFIED] docs |
| `--config key=value` | | Set any setting headlessly (repeatable) | [VERIFIED] docs |
| `--verbose` | | Stream tool progress to stderr; prints session ID for chaining | [VERIFIED] docs |
| `--skip-onboarding` | | Skip taste onboarding (CI/automation) | [VERIFIED] docs |
| `--trust` | `-t` | Auto-trust project | [VERIFIED] docs |
| `--add-dir` | | Add directory to workspace context | [VERIFIED] docs |
| `--mod` | | Load mod file/directory (repeatable) | [VERIFIED] docs |
| `--skill` | | Load extra skills from path (repeatable) | [VERIFIED] docs |
| `--no-skills` | | Skip skill discovery | [VERIFIED] docs |
| `--theme` | | Color theme: `dark`, `light`, `auto` | [VERIFIED] docs |

### 2.3 Output Format

#### Text (default: `--output-format text`)
- Final answer only on stdout.
- Errors/warnings to stderr.
- Pipe-friendly: `cmd -p "generate README" > README.md`.

#### JSON (`--output-format json`)
- **NDJSON** (newline-delimited JSON), one object per line.
- Two frame types:

**Event frames** (streamed during run):
```json
{"type": "event", "event": {"type": "tool_running", "toolCallId": "…", "toolName": "read_file", "description": "…"}}
```

**Final result line** (always last):
```json
{
  "type": "result",
  "subtype": "success",
  "sessionId": "9f4e1c0a-…",
  "stopReason": "end_turn",
  "usage": { … },
  "durationMs": 8421,
  "finalText": "…"
}
```

| Field | Always? | Notes |
|-------|---------|-------|
| `subtype` | Yes | `success`, `error`, or `max_turns` |
| `usage` | Yes | Token usage totals |
| `durationMs` | Yes | Wall-clock duration |
| `finalText` | Yes | Final answer (empty on error) |
| `sessionId` | Optional | Omitted on early failure (auth, bad input) |
| `stopReason` | Optional | `end_turn`, `max_turns`, …; omitted on error |
| `error` | Optional | Only when `subtype` is `error` |

- `[VERIFIED]` Consumers MUST treat unknown `event.type` values as forward-compatible.
- `[VERIFIED]` Parse line-by-line, not buffered.

### 2.4 Session Handling

- **Persistence:** Each headless run persists transcript to disk.
- **Tagging:** Headless sessions are tagged separately — hidden from interactive `/resume` menu and `--continue`.
- **Continue:** `cmd -p --continue "next prompt"` resumes most recent headless session. If no session exists, starts fresh (safe for loop-first iteration).
- **Resume by ID:** `cmd -p --resume <id>` resumes specific session. Bare `--resume` (no id) ERRORS in headless mode.
- **Fork:** `cmd -p --resume <id> --fork-session "try alternative"`.
- **Session discovery:** `--verbose` prints session ID to stderr for chaining. `cmd --session <path|id>` accepts `.jsonl` path or prefix.
- **Interactive open:** `cmd --resume <id>` (without `-p`) opens headless session in full interactive TUI.
- **No persist:** `--no-session` keeps it in-memory.

### 2.5 Worktree

- **`-w, --worktree [name]`:** Runs in an isolated managed git worktree.
  - `cmd -w spike` — named worktree.
  - `cmd -w /path/to/existing` — explicit path.
  - `cmd -w #1234` — by PR number.
  - `cmd -w` (bare) — auto-generated name.
- Combines with `-p`: `cmd -p -w spike "fix the build"`.
- `[VERIFIED]` from docs.

### 2.6 Completion Detection

**Exit codes:**

| Code | Meaning | Constant |
|------|---------|----------|
| 0 | Success | `EXIT_SUCCESS` |
| 1 | General error | `EXIT_ERROR` |
| 3 | Not authenticated | `EXIT_AUTH_ERROR` |
| 4 | Permission denied | `EXIT_PERMISSION_DENIED` |
| 5 | Rate limit exceeded | `EXIT_RATE_LIMITED` |
| 6 | Network failure | `EXIT_CONNECTION_ERROR` |
| 7 | API server error (5xx) | `EXIT_SERVER_ERROR` |
| 8 | Max turns reached | `EXIT_MAX_TURNS_REACHED` |
| 9 | Model produced no response | `EXIT_NO_RESPONSE` |
| 10 | Insufficient credits | `EXIT_INSUFFICIENT_CREDITS` |
| 130 | Interrupted (SIGINT/SIGTERM) | `EXIT_INTERRUPTED` |

- `[VERIFIED]` from official docs.
- In JSON mode, `subtype: "max_turns"` also indicates incomplete run even if exit code is 0 (partial response returned).

### 2.7 Permissions in Headless Mode

| Tool | Default (no flag) | With `--yolo` |
|------|-------------------|---------------|
| File reads, grep, glob | Allowed | Allowed |
| File edits and writes | **Blocked** | Allowed |
| Shell commands | **Blocked** | Allowed |

- `--permission-mode dont-ask` provides a fail-closed allowlist mode for CI.

---

## 3. Comparison Table

| Capability | OpenCode | CommandCode |
|------------|----------|-------------|
| **Binary** | `opencode` | `cmd` (Windows: `cmdc`) |
| **Headless command** | `opencode run "prompt"` | `cmd -p "prompt"` |
| **Piped stdin** | Yes | Yes (auto-detected) |
| **Default output** | Plain text stdout | Plain text stdout |
| **Structured output** | `--format json` (single JSON) | `--output-format json` (NDJSON stream) |
| **Streaming events** | SSE (server mode only) | Yes, NDJSON event frames inline |
| **Exit codes** | `0`/non-zero `[INFERRED]` | Rich: 0–10, 130 (documented) |
| **Session continue** | `--continue` / `-c` | `--continue` / `-c` |
| **Session by ID** | `--session` / `-s <id>` | `--resume` / `-r <id>` or `--session <path\|id>` |
| **Session fork** | `--fork` | `--fork-session` |
| **Session naming** | `--title` | `--name` / `-n` |
| **No persist** | `[UNKNOWN]` | `--no-session` |
| **Headless→interactive** | `[UNKNOWN]` | `cmd --resume <id>` (without `-p`) |
| **Managed worktrees** | Community plugins; `--dir` flag | `-w / --worktree [name\|path\|#PR]` |
| **Model select** | `--model provider/model` | `-m / --model <id>` |
| **Max turns** | `[UNKNOWN]` | `--max-turns` (default 100) |
| **Auto permissions** | `--auto` | `--yolo`, `--auto-accept`, `--permission-mode` |
| **Plan mode** | `[INFERRED]` via custom agents | `--plan` |
| **Config flags** | `opencode.json` file-based | `--config key=value` (repeatable) |
| **Skills/plugins** | `--pure` to disable; file-based config | `--skill`, `--mod`, `--no-skills` |
| **Server/HTTP mode** | `opencode serve` (OpenAPI 3.1 + SSE) | `[UNKNOWN]` |
| **Attach to server** | `--attach http://localhost:4096` | `[UNKNOWN]` |
| **Verbose/progress** | `--print-logs`, `--log-level` | `--verbose` (stderr, incl. session ID) |
| **Language** | Go | Node.js/TypeScript `[INFERRED]` |
| **Source** | `github.com/anomalyco/opencode` | `github.com/CommandCodeAI/command-code` |

---

## 4. Key Differences for Adapter Design

1. **Output format schema:** CommandCode has a precise, documented NDJSON schema with `subtype`, `sessionId`, `stopReason`, `usage`, `durationMs`, `finalText`. OpenCode's JSON format is `[INFERRED]` — likely a single object, schema undocumented in available sources.

2. **Worktree management:** CommandCode has first-class `-w` flag with auto-naming, PR-based setup. OpenCode relies on CWD discovery and `--dir`, with community plugins for worktree lifecycle.

3. **Exit codes:** CommandCode documents 12 distinct exit codes. OpenCode exit codes are `[INFERRED]` limited to 0/non-zero.

4. **Streaming:** CommandCode NDJSON streams event frames inline during headless runs. OpenCode only streams via SSE in server mode.

5. **Session isolation:** CommandCode tags headless sessions separately so they never pollute interactive history. OpenCode's session isolation model is `[UNKNOWN]`.

6. **Server mode:** OpenCode has `opencode serve` for a persistent HTTP API. CommandCode has no documented equivalent.

7. **`--config` system:** CommandCode exposes all settings as `--config key=value` CLI flags. OpenCode uses JSON config files.
