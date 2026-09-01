# Backlog triage — 2026-08-31

Every open issue read against the repository as it exists today (the Markdown-and-bash
Mill of ADR 0006, rebuilt 2026-08-31). A verdict is a checkable statement, not an
impression: each row cites the command or file that decides it. `SUPERSEDED` and `DONE`
name the commit. `UNCLEAR` is used only where the repository cannot decide — none were
needed here.

## Verdicts

| # | Title | Verdict | Evidence / commit |
|---|---|---|---|
| #184 | mill-dispatch leaves the brief pasted and unsent for agents that need an explicit enter | LIVE | `.mill/checks/mill-dispatch` runs `worker-start`; no submit step |
| #183 | mill-verify --dispatch blocks a landing when a question was answered outside its thread | LIVE | `thread_id` heuristic in `.mill/checks/mill-verify` |
| #182 | Orca's orchestration layer went unused — coordinator hand-rolled worse substitutes | LIVE | delegate skill §3 still lists a subset; `mill-dispatch` (6ae330d) is a partial answer |
| #181 | A dispatch must declare provider and model — tier was chosen by role | LIVE | tier field deleted (24f3019); rule is prose, not a gate |
| #180 | delegate skill should check the target role's allowed_files before writing a brief | DONE | `0631f68` / `2ede2a3` — `mill-preflight --brief` refuses unwritable paths |
| #178 | The model tier lives in a global file the agent rewrites — two tiers cannot run concurrently | LIVE | `.mill/agents.example` documents `command-code` rejects `--model` |
| #177 | Orca reports a running command-code dispatch as failed — completed workers written off | LIVE | `MEMORY.md` dispatch traps; upstream Orca readiness bug |
| #173 | Mill should not take core.hooksPath at all — the gauntlet belongs on the worker's output | DONE | coordinator cleanup 2026-09-01 — hook deleted, `core.hooksPath` unset; no commit |
| #165 | No evals — verification is a human reading a diff | LIVE | no `.mill/checks/eval-*`; `ci.yml` is `bash -n` only |
| #164 | Measure whether the delegation chain beats a single agent | LIVE | no measurement artifact exists in `docs/` |
| #163 | The first run does not survive without its author | LIVE | no installer (6f18cba); ADR 0011 decided, not built |
| #162 | Nobody outside this machine has installed Mill | LIVE | no installer; ADR 0011 decides the mechanism, tracks implementation |
| #156 | Nothing is ever written back to the issue | LIVE | no `gh issue comment`/`edit` path in executable code |
| #154 | Delegation routing must support fan-out — delegates_to is permission, not routing | SUPERSEDED | `delegates_to` deleted (6578719); star topology dissolves fan-out |
| #148 | A delegated agent set core.bare=true and broke the repository | LIVE | `core.hooksPath=/dev/null` set again 2026-08-31 18:54; mechanism changed, class did not |
| #139 | Mill is a company, not a task runner | LIVE | `docs/PRODUCT.md` records it; escalation ladder and write-back still missing |
| #137 | Nothing re-runs — no CI, lessons no code reads | LIVE | `ci.yml` exists (syntax only); lessons read by nothing |
| #129 | Role enforcement only acts at commit | LIVE | PreToolUse guard (cdd639c) closes most; Bash hole remains |
| #110 | Clasificación de fallo — detección y reacción | SUPERSEDED | `classifyResult` deleted with the Go CLI (6578719) |
| #108 | Feedback loop — diff/bug reports auto-train the producing role | LIVE | no auto-training; `LESSONS.md` read by nothing |
| #95 | Sin versionamiento semántico — sin tags, sin CHANGELOG | LIVE | `git tag -l` empty; no `CHANGELOG.md` |
| #86 | prevent --no-verify bypass of role enforcement hooks | LIVE | matcher covers write tools only; `--no-verify` runs via `Bash` (`.claude/settings.json:16`) |
| #56 | Route production and review to different models | LIVE | skill §2 prose; nothing executes it |
| #54 | Add review loop: produce → review → changes → rework | LIVE | reviewer verdict exists; no mechanised rework loop |
| #25 | Role contract enforcement — mechanised process gates | DONE | `role-enforce` + `mill-role-guard` + `mill-verify`; `d3dafbd` |
| #1 | Mill: Multi-Agent Delegation Harness | LIVE | project exists; the Go session summary was deleted (6578719) |

Counts: LIVE 21 · SUPERSEDED 2 · DONE 3 · UNCLEAR 0.

## Notes

### #184 — mill-dispatch leaves the brief pasted and unsent — LIVE
`.mill/checks/mill-dispatch` exists (landed in `6ae330d`) and its step 3 is
`orca orchestration worker-start`; the script contains no submit/enter step
(`grep -c "enter\|submit" .mill/checks/mill-dispatch` → 0). The bug is a property of the
current script, not a deleted system. Note the issue's claim that
"`.mill/agents.example` recorded the enter difference" does not match the file today:
it records that `command-code` rejects `--model` and `omp` takes none, but not the
submit-vs-enter difference.

### #183 — mill-verify --dispatch false positive — LIVE
`.mill/checks/mill-verify` implements `--dispatch` and decides "answered" exactly as the
issue describes: a question is answered when a distinct message carries
`thread_id == question.id` (lines 108–114, "the `read` flag is not the signal"). The
out-of-thread false positive is a property of the current gate, so it is LIVE, not
fixed.

### #182 — orchestration layer unused — LIVE
The delegate skill's dispatch section (§3) still lists only `check`, `worker-release`,
`send`, `worker-read` and defers the rest to Orca's guide; it does not make reading the
mailbox a gate. `mill-dispatch` (`6ae330d`) and the inbox fix (`69ea866`) close part of
the surface, but the issue's three refinement questions (full loop in the skill, mailbox
as a gate, forbidding `terminal send`) are unanswered. LIVE.

### #181 — dispatch must declare provider and model — LIVE
The mechanism the issue names — a role `model:` frontmatter choosing the tier — is
deleted: `grep -rn '^model:' .mill/roles/` returns nothing (`24f3019`). Skill §2 now
requires naming agent and model, and the landing commit records `Mill-Dispatch:
... model=...` (verified on `6ae330d`, `fbc9cc9`, `1ffa703`). What remains is the
issue's own demand: it is prose, not a gate, and `command-code` cannot carry a model
(#178), so the choice is neither enforced nor reliably auditable. LIVE.

### #180 — check allowed_files before writing a brief — DONE
`mill-preflight --brief <role> <path>...` refuses every path the role cannot write
(`.mill/checks/mill-preflight` line 63: "the named role must be able to write every
named path"), landed in `0631f68` / `2ede2a3`. The delegate skill §4 closes with "run
`mill-preflight --brief <role>` with the paths the brief asks the worker to write".
That is the brief-time check the issue asked for. DONE.

### #178 — model tier in a global file — LIVE
`.mill/agents.example` states it verbatim: "`worker-start --agent command-code` rejects
`--model`. Its model comes from the global `~/.commandcode/config.json`, which
command-code rewrites itself, so two dispatches on different models cannot run
concurrently against it (#178)." The tier vocabulary is gone (24f3019) but the
command-code limitation it names is current. LIVE.

### #177 — command-code reported failed while running — LIVE
`MEMORY.md` ("Dispatch traps, measured 2026-08-30") records the failure and the
workaround (read the terminal before trusting the verdict). It is an upstream Orca
readiness-detection defect, still unfixed, and #184 refines the same `agent_prompt_stalled`
signature. LIVE.

### #173 — do not take core.hooksPath — DONE
Resolved by cleanup, not by any commit — the one row whose evidence is not a commit.
The issue argues Mill should not take a project's git hooks at all, and it was being
violated at triage time: `core.hooksPath=/dev/null` disabled every hook in the
repository, the project's own included — precisely the invasiveness the issue objects
to — while a dead `.git/hooks/pre-commit` (63 lines reading `.mill/role`, retired in
`e5068ac`) sat behind it. On 2026-09-01 the coordinator removed both: the hook is
deleted (backup `/tmp/pre-commit.bak-2026-09-01`) and `core.hooksPath` is unset. DONE.

### #165 — no evals — LIVE
`ls .mill/checks/eval-*` → "No such file or directory". `.github/workflows/ci.yml`
runs only `bash -n` over `.mill/checks/*` and a frontmatter check — build/lint/test
verification, not dispatch-quality evaluation. Its own 2026-08-31 comment states the
gauntlet half moved (`c89dacb`) but "no eval of whether a dispatch was any good"
remains. LIVE.

### #164 — measure chain vs single agent — LIVE
No experiment artifact exists: there is no `docs/` file recording two arms, three runs,
cost/time/rework. The comparison has simply never been run. LIVE.

### #163 — first run does not survive without its author — LIVE
The four failure modes it lists (`--agent claude`, `task-update`, `check --ack`) belong
to the deleted Go CLI (`6578719`). But the headline — no checkable "installed" state, no
stranger-verifiable first run — is still true: `ls .mill/checks/mill-install` is absent
(`6f18cba`), and ADR 0011 decides the fix without building it. LIVE.

### #162 — nobody outside this machine has installed Mill — LIVE
No installer exists in the tree; `scaffold/`, `mill-install` and `mill-uninstall` were
deleted (`6f18cba`). ADR 0011 (`3de1001`) decides the mechanism (versioned copy from a
git ref) and its 2026-08-31 comment says the issue now tracks implementing it. LIVE.

### #156 — nothing written back to the issue — LIVE
`grep -rn "gh issue comment\|gh issue edit" .mill/ .claude/ .github/` matches nothing
executable — the only `gh issue` reference in an active path is the read-only
`gh issue view` in delegate skill §6. The Go `reader.go` path is gone (`6578719`) and no
replacement exists, so the write-back requirement #139 depends on is still unmet. LIVE.

### #154 — delegates_to is permission, not routing — SUPERSEDED
`grep -rn delegates_to .mill/roles/` returns nothing — the frontmatter field the issue
calls "permission, not routing" was deleted with the Go delegation layer (`6578719`).
Fan-out is explicitly dissolved, not built: `ARCHITECTURE.md` names it — "This dissolves
rather than fixes several problems: ... and fan-out (#154)." SUPERSEDED.

### #148 — core.bare=true broke the repo — LIVE
The specific mechanism — the Go runner (`6578719`) — is gone, but the class recurred:
on 2026-08-31 at 18:54, fifteen minutes after `6ae330d` landed, a delegated agent wrote
`core.hooksPath=/dev/null` into the shared config again (`git config --local
core.hooksPath` → `/dev/null`; `.git/config` modified Aug 31 18:54). No script in the
repository sets it — `grep -rn hooksPath` finds only a comment in `.mill/checks/common.sh`
and prose in `LESSONS.md` and ADR 0009 — so an agent wrote it in-session and every
worktree inherited it. The mechanism changed; the class did not. LIVE.

### #139 — Mill is a company, not a task runner — LIVE
The product definition is now recorded correctly: `docs/PRODUCT.md` opens "Mill is an
**org chart that executes**". But the FRD's centrepiece — the escalation ladder — has no
implementation (its transport is the issue write-back #156, still missing), so the issue
is not satisfied. Note `.mill/docs/PRODUCT.md` and `.mill/docs/ORG-CHART.md` still carry
the pre-rebuild wording and `model: pro/free` tiers (`diff -q docs/PRODUCT.md
.mill/docs/PRODUCT.md` → "differ") — stale, not the canonical definition. LIVE.

### #137 — nothing re-runs — LIVE
Half of it changed, half did not. CI now exists: `.github/workflows/ci.yml` runs
`bash -n` over checks and a frontmatter check — so "no CI" is false. But
`grep -rln lessons .mill/checks/ .claude/` returns nothing: `LESSONS.md` (18 KB) is read
by no code and injected into no prompt, which is the issue's point about corrections
surviving only as prose. LIVE.

### #129 — enforcement only at commit — LIVE
The literal claim is now false: `.claude/settings.json` wires `mill-role-guard --pretool`
as a `PreToolUse` hook on `Write|Edit|NotebookEdit` (`cdd639c`/`4be15c3`), so the
coordinator's writes are blocked before they happen, not at commit. The issue stays open
on the hole its own comment names: the matcher excludes `Bash`, so a heredoc or `sed -i`
still writes unblocked — recorded as a known limit in `roles/COMMON.md`. LIVE.

### #110 — failure classification taxonomy — SUPERSEDED
The taxonomy was implemented as Go (`classifyResult`, `classify.go`); that code was
deleted with the Go CLI (`6578719`). No `classify*` symbol remains. The detection-and-
reaction requirement has no current implementation, but the issue as written describes a
deleted system. SUPERSEDED.

### #108 — feedback loop auto-trains the role — LIVE
Nothing implements it: no per-role `lessons.md` is loaded into a dispatch prompt
(`grep -rln lessons .mill/checks/ .claude/` → none), and the role files declare lessons
"reference material, not required reading" (`roles/COMMON.md`). The mechanism the issue
proposes is absent. LIVE.

### #95 — no semver, tags, changelog, releases — LIVE
`git tag -l` → empty; `ls CHANGELOG.md` → absent; no `.goreleaser.yml`, no `go.mod`
(the Go tooling they describe was retired). ADR 0004 decides the versioning strategy but
no tag has been cut (ADR 0011 says the same). LIVE.

### #86 — prevent --no-verify bypass — LIVE
The guard cannot block the bypass it claims to. `mill-role-guard --pretool`
(`cdd639c`/`4be15c3`) is wired on the `PreToolUse` matcher `Write|Edit|NotebookEdit`
(`.claude/settings.json` line 16), which covers the file-writing tools only.
`git commit --no-verify` runs through `Bash`, which that matcher does not cover, so the
guard has never fired for it in any session — the gap is recorded deliberately in
`.mill/roles/COMMON.md` ("That path is deliberately left open"). The escape hatch the
issue names remains open. LIVE.

### #56 — route production and review to different models — LIVE
The rule is prose only: delegate skill §2 states "the tier follows the work, not the
role", and the Mill-Dispatch trailer records the model after the fact. Nothing executes
or gates the routing, and `command-code` cannot carry `--model` (#178), so the routing is
not auditable — the issue's own 2026-08-31 comment says "the rule now exists in writing;
nothing executes it." LIVE.

### #54 — review loop produce → review → rework — LIVE
The verdict exists — `.mill/roles/reviewer/ROLE.md` requires a binary APPROVED/CHANGES —
but the loop (rework → re-review → approved) is not mechanised; it is the coordinator's
manual sequencing, with no gate or script enforcing the round-trip. LIVE.

### #25 — role contract enforcement mechanised — DONE
The principle is now embodied in bash, which is exactly what the issue asks: `role-enforce`
(by category via `.mill/role-capabilities`), `mill-role-guard` (PreToolUse write block),
and `mill-verify` (dispatch-boundary check) are all scripts, and `d3dafbd` commits the
capability map so a fresh clone can enforce. The issue's specific "gates to mechanise"
table (spec ≤9 criteria, granularity, brief format) was the six phase gates deleted in
`6166ada` and replaced by dispatch-boundary verification (ADR 0009). DONE.

### #1 — the original epic — LIVE
The epic's destination ("open-source multi-agent delegation harness") still describes
this repository — `AGENTS.md`, 12 roles, 8 check scripts all exist. The 2026-08-09
session summary (Go runner, `classify`, `state.json`, adapters) describes the deleted
Go build (`6578719`), so that half is superseded, but the epic itself is the live
project. LIVE.

## Reference list verification

All eight commits in the brief exist and their subjects match the glosses (wording
differs, content does not): `6166ada` (collapse — gates/skill deleted), `6f18cba`
(scaffold/installers deleted), `24f3019` (tier retirement), `d3dafbd` (capability map
committed), `2ede2a3` (preflight --brief / verify --dispatch), `6ae330d` (mill-dispatch),
`3de1001` + `7091b64` (ADR 0011, corrected to key on a ref).

One item in the "commented today" list is not an open issue: **#132** is CLOSED (closed by
`d3dafbd`, with a prior wrong close in between), so it receives no row — its defect
(gitignored capability map) is the fix `d3dafbd` landed, which is already credited to
#25 and #129 above.

## Post-triage correction — 2026-09-01

Three of 26 verdicts were wrong on spot-check, all three in the direction of judging an
issue resolved. The pattern: a verdict reached from what the repository now *contains*
rather than from what it *does* — the `PreToolUse` guard exists, so #86 read as done;
the Go runner is gone, so #148 read as impossible. Existence is not behaviour.
