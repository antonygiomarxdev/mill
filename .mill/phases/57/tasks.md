# Tasks: Auto-compact via `--config compact-mode=fast`

All tasks modify/update only files listed in the SPEC Components affected table.

## Wave 1 (parallel — three independent tasks)

### Task 1: Create `internal/compact/compact.go` — Compaction engine

- role: sr-dev-be
- deps: none
- est: 30m
- file: `internal/compact/compact.go` (NEW)

#### Acceptance Criteria

1. Define package-level types:
   - `Mode string` — accepted values: `"fast"` (only mode for now; future extensible)
   - `Config struct { Enabled bool; Mode Mode }` — mirrors the config-level Compact struct
   - `ContextWindow int` — tokens per model tier
   - `Event struct { Timestamp time.Time; Issue int; PreTokens int; PostTokens int; Saved int; Trigger string }` — log event shape

2. Implement `ShouldCompact(contextText string, tier string) (bool, int)`:
   - `tier` maps to context window: `"free"` → 128K, `"paid"` → 200K, `"pro"` → 200K; unknown tier → 128K (conservative default)
   - Estimates tokens as `len(contextText) / 4`
   - Returns `true` when estimated tokens ≥ 80% of context window
   - Also returns the estimated token count (second return value)
   - Zero-length context → `(false, 0)`

3. Implement `Compact(contextText string, tier string, issueNum int) (string, Event)`:
   - Parses context text into turns (a "turn" = user message + assistant response + tool calls block)
   - **Preserves:** (a) original delegation prompt (text up to first turn boundary), (b) role/capability boundaries (any block matching `.mill/role` or ROLE.md references), (c) last 3 turns, (d) unresolved items (turns containing "BLOCKED" or "unresolved"), (e) current working state (last turn + any preceding open-file mentions)
   - **Discards:** tool outputs older than the last 3 turns, completed sub-agent dialogue (turns with APPROVED verdict), speculative exploration that produced no file changes, error messages from resolved failures
   - Replaces discarded content with structured summary: `[COMPACTED: explored <N> paths, made <M> changes, resolved <K> errors. Full history in .mill/compact.log]`
   - `<N>` = count of turns with exploration/read patterns, `<M>` = count of turns with write/edit/file changes, `<K>` = count of turns with error messages that were later resolved
   - Returns the compacted text and the log Event

4. Implement `WriteLog(event Event) error`:
   - Appends JSONL line to `.mill/compact.log`
   - Creates the file if it does not exist
   - Uses `os.O_APPEND|os.O_CREATE|os.O_WRONLY` with `0o644`
   - Event serialized as single-line JSON (no indent)
   - Returns error if write fails

5. Follow existing project conventions: standard library only, no external dependencies, same error-wrapping style (`fmt.Errorf("...: %w", err)`)

### Task 2: Add `Compact` struct to `internal/config/config.go`

- role: sr-dev-be
- deps: none
- est: 10m
- file: `internal/config/config.go` (MODIFY)

#### Acceptance Criteria

1. Define new exported types (can be in same file or a new `compact.go` in the config package):
   - `CompactMode string` — type alias for compact mode values
   - `CompactConfig struct { Enabled bool \`json:"enabled"\`; Mode CompactMode \`json:"mode,omitempty"\` }` — compact settings

2. Add `Compact *CompactConfig \`json:"compact,omitempty"\`` field to the `Config` struct

3. Update `Default()`: set `Compact` to `&CompactConfig{Enabled: false, Mode: "fast"}` (disabled by default, "fast" is the default mode when enabled)

4. Existing tests in `internal/config/config_test.go` continue to pass — verify `Save`/`Load` round-trips the new field correctly (omitempty: when `Compact` is nil or its default, JSON omits the key)

5. Loading partial config (missing `compact` key) deserializes with `Compact` as nil rather than panicking

### Task 5: Update `mill.yml` template with `compact:` section

- role: sr-dev-be
- deps: none
- est: 5m
- file: `internal/cli/static/mill.yml.tmpl` (MODIFY)

#### Acceptance Criteria

1. Add a `compact:` section to the template after `max-rounds` and before `directories`:
   ```yaml
   # Context compaction — trims old conversation to prevent context-window exhaustion
   compact:
     enabled: false   # set to true to auto-compact long sessions
     mode: fast        # compaction strategy (only "fast" currently supported)
   ```

2. Template renders correctly with existing `initConfig` (no new template variables needed — the section is static YAML)

3. Existing `mill init` tests continue to pass

---

## Wave 2 (sequential — depends on Wave 1)

### Task 3: Pass `compactMode` to dispatch loop in `internal/cli/delegate.go`

- role: sr-dev-be
- deps: Task 1, Task 2
- est: 20m
- file: `internal/cli/delegate.go` (MODIFY)

#### Acceptance Criteria

1. Parse `--config compact-mode=fast` from delegate args:
   - Add `--config` flag to the `flag.FlagSet` in `runDelegate`
   - Parse the value: split on `=` to extract `compact-mode` key; validate value is `"fast"` (the only supported mode)
   - Unknown mode → return error `"unsupported compact mode: <mode>"`

2. Resolve effective compact mode (priority: CLI flag → config file → disabled):
   - CLI `--config compact-mode=fast` enables it for that delegation only
   - If no CLI flag: check `cfg.Compact.Enabled` and `cfg.Compact.Mode` from config file
   - If neither: compaction is off (the default)
   - Return resolved `compact.Mode` (empty string = disabled)

3. Pass resolved mode into `runDispatchLoop` as a new parameter `compactMode compact.Mode`

4. In `runDispatchLoop`, inject compaction after each agent turn (between `session.Wait()` and `classifyResult`):
   - If `compactMode` is non-empty, call `compact.ShouldCompact` with the session's accumulated context text
   - If compaction triggers: call `compact.Compact`, replace the session context, call `compact.WriteLog`
   - The session must expose its accumulated context text (read NDJSON frames or maintain an in-memory buffer)
   - If the adapter's Session interface does not expose context text directly, add a method or track it in the loop

5. Backward compat: when compactMode is empty, no compaction code runs — behavior is identical to today

6. Import `"github.com/antonygiomarxdev/mill/internal/compact"` in delegate.go

### Task 4: Create `internal/cli/compact.go` + add routing in `internal/cli/app.go`

- role: sr-dev-be
- deps: Task 1
- est: 20m
- file: `internal/cli/compact.go` (NEW), `internal/cli/app.go` (MODIFY)

#### Acceptance Criteria

1. Create `internal/cli/compact.go` with `func (a *App) runCompact(args []string) error`:
   - Define a `flag.FlagSet` for `compact` with `--dry-run` (bool, default false)
   - Parse args; reject unknown flags and positional args

2. When `--dry-run` is true:
   - Read the current session's accumulated context (from the session file or adapter state)
   - Call `compact.ShouldCompact` to determine if compaction would trigger
   - Call `compact.Compact` but do NOT write the result or log
   - Print to stdout:
     - Estimated token count and context window
     - Whether compaction would trigger (yes/no + threshold %)
     - What would be preserved (turn count, role info)
     - What would be discarded (turn count, types of content)
     - Post-compaction estimated token count
   - Do not write `.mill/compact.log`

3. When `--dry-run` is false (normal mode):
   - Read current session context
   - Call `compact.ShouldCompact`; if below threshold, print "Context at <N>% of window — compaction not needed." and exit 0
   - Call `compact.Compact` to produce compacted text
   - Replace the session's context with the compacted version
   - Call `compact.WriteLog` to record the event
   - Print summary: `Compacted: <pre> → <post> tokens (saved <saved>)`
   - If no active session exists, print "No active session to compact." and return nil (not error)

4. In `internal/cli/app.go` Run switch, add:
   ```go
   case "compact":
       return a.runCompact(args[1:])
   ```

5. Update `usage()` in app.go to include `compact` in the command listing

---

## Wave 3 (sequential — depends on Wave 2)

### Task 6: Create tests for all compaction paths

- role: sr-dev-be
- deps: Task 1, Task 2, Task 3, Task 4
- est: 35m
- file: `internal/compact/compact_test.go` (NEW), `internal/cli/delegate_test.go` (MODIFY), `internal/cli/compact_test.go` (NEW)

#### Acceptance Criteria

1. `internal/compact/compact_test.go`:
   - `TestShouldCompactBelowThreshold`: context at 50% of window → false
   - `TestShouldCompactAtThreshold`: context at exactly 80% → true
   - `TestShouldCompactAboveThreshold`: context at 95% → true
   - `TestShouldCompactEmptyContext`: zero-length → false, 0 tokens
   - `TestShouldCompactTierMapping`: "free"→128K, "paid"→200K, "pro"→200K, unknown→128K
   - `TestCompactPreservesOriginalPrompt`: first block (before any turn) survives compaction
   - `TestCompactPreservesLastThreeTurns`: the 3 most recent turns are in the output
   - `TestCompactPreservesUnresolvedItems`: turns containing "BLOCKED" survive even if older than 3 turns
   - `TestCompactDiscardsOldToolOutputs`: tool outputs from turns beyond last 3 are absent
   - `TestCompactDiscardsResolvedErrors`: error messages from resolved failures are absent
   - `TestCompactProducesSummaryLine`: output contains `[COMPACTED: explored` structured summary
   - `TestCompactSummaryCounts`: N, M, K in summary match actual turn analysis
   - `TestWriteLogCreatesFile`: log file created with valid JSONL
   - `TestWriteLogAppendsMultiple`: second call appends, doesn't overwrite
   - Follow existing test conventions: table-driven where appropriate, `*testing.T`, temp dirs via `t.TempDir()`

2. `internal/cli/compact_test.go`:
   - `TestCompactCommandDryRun`: `runCompact([]string{"--dry-run"})` prints what would change, does not write log
   - `TestCompactCommandNoSession`: no active session → prints "No active session" message, exits 0
   - `TestCompactCommandBelowThreshold`: context below 80% → prints "not needed", exits 0
   - Use `fakeAdapter` / test fixtures to simulate session context
   - Follow existing CLI test conventions from `delegate_test.go`

3. `internal/cli/delegate_test.go` (MODIFY):
   - `TestDelegateCompactModeFlag`: `runDelegate` with `--config compact-mode=fast` extracts and passes mode
   - `TestDelegateCompactModeInvalid`: `--config compact-mode=bogus` → error "unsupported compact mode"
   - `TestDelegateCompactModeDefaultOff`: no `--config` flag and config Compact disabled → mode is empty, no compaction
   - Add or update `fakeAdapter` / `fakeSession` as needed to expose context text for compaction testing
   - All existing delegate tests continue to pass

4. `go test ./internal/compact/` passes
5. `go test ./internal/cli/` passes

---

## Acceptance criteria (cross-cutting)

1. `mill delegate --config compact-mode=fast` enables compaction
2. `mill.yml` `compact.enabled: true` enables compaction globally
3. Compaction fires at 80% context window (character-based estimate)
4. Preserved: original prompt, role, last 3 turns, unresolved items
5. Discarded: old tool outputs, completed sub-agent dialogue, resolved errors
6. Structured summary replaces discarded content
7. `.mill/compact.log` records every compaction event
8. `mill compact` triggers manual compaction
9. `mill compact --dry-run` shows what would change
10. `go test ./internal/compact/` passes
11. `go test ./internal/cli/` passes (compact command, delegate integration)
