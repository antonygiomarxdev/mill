# Findings — August 2026

A record of what was discovered about Mill's actual behaviour during two days of
running it against itself. Written down because the individual issues scatter and
the *patterns* are what get forgotten.

Every claim here was verified by execution, not by reading code.

---

## The one-sentence summary

**Mill is described far more precisely than it is built, and everything written
down read as if it were done.**

Eleven roles were configured and one operated. A delegation chain was declared
and never executed past its first step. Quality gates were installed and never
ran. Lessons were recorded in a file nothing reads. An issue tracker was named as
the single source of record and nothing ever wrote to it.

None of this produced an error. That is the point.

---

## The recurring failure mode

Four distinct shapes of the same underlying problem. Each was found by accident,
while looking for something else.

### 1. Source living where it is not carried

A delegated worktree is created with `git worktree add`, which materialises only
**tracked** files. Anything an agent must read has to be versioned.

| Issue | What was untracked | Effect |
|---|---|---|
| #125 | role `lessons.md` | learning did not persist |
| #126 | phase gates, `COMMON.md` | delegated worktrees had no gates at all |
| #128 | phase artifacts (`frd.md`, `spec.md`, `tasks.md`) | gates verified files that never survived |
| #140 | `PRODUCT.md`, `ORG-CHART.md`, all ADRs | the project's definition existed on one disk |
| #136 | `role-enforce` missing from the scaffold | every new project shipped with no enforcement |

`.gitignore` used `/.mill/*` plus a re-include per subtree — an allowlist that has
to be extended by hand, where forgetting produces no error.

### 2. A rule written in Markdown with no mechanism behind it

| Issue | The rule | The reality |
|---|---|---|
| #153 | roles delegate down the chain | `delegates_to` is a permission list; no role is ever *told* to hand off. The chain has never run past depth 1. |
| #137 | lessons inform future work | two divergent paths (`.mill/lessons/<role>.md`, `.mill/roles/<role>/lessons.md`), and no code reads either into a prompt |
| #156 | "never leave an issue silent"; "debate is public" | nothing in the codebase posts a comment. Every comment in this repo was written by hand. |
| #151 | coverage ≥90%, non-negotiable | the gate sampled one arbitrary package. Real coverage was 83.6%. |

### 3. Verification that inspects shape instead of behaviour

A check that asserts structure passes on a defect that has the right structure.

- **#145** — `core.hooksPath` was written relative to the main repo and resolved by
  git from the worktree, pointing at a directory that did not exist. Git treats a
  missing hooks directory as *no hooks*, silently. **No delegated worktree ever
  ran the gauntlet.** Every test of hook installation passed throughout, because
  they asserted the configured string.
- **#143 / #158** — the reviewer was handed the produce agent's own narration under
  a heading reading `## Changes (diff)`. The field was empty, so an expensive-tier
  review ran against `(no diff available)` and returned `approved` for code that
  did not compile.
- **#116** — a `provider_config` structure that parsed correctly and changed nothing
  at dispatch, because it populated a different map than the resolver reads.

**Consequence for how work is accepted here:** acceptance criteria must be
behavioural. *Make a violating commit and assert it is rejected. Dispatch a
`model: pro` role and assert the expensive model reaches the argument builder.*
"The config parses" and "the field is set" prove nothing about what runs.

### 4. Fixing the mechanism reveals that what it executed was also broken

#145 made the gauntlet actually run. Within an hour that surfaced:

- **#147** — `role-enforce` had cases for four roles; five others fell through to
  `Unknown role` and were refused outright. An Architect could not commit a
  `spec.md`. Wrong since it was written, unobservable because it never executed.
- **#150** — `gate-coverage` took `head -1` of `go test ./...` output, i.e. one
  arbitrary package chosen by scheduling order. It blocked healthy commits and
  passed packages at 77%. Non-deterministic across runs of the same tree.

Both had been in place for months and neither could have been found while the
mechanism above them was inert.

---

## The economics have never run

The design is: expensive models decompose and review, cheap models execute, and
quality comes from the review. That is the reason Mill spawns its own processes
instead of driving the host harness — a harness runs one model.

**It has never executed, not once.**

`mill.yml` carries no `models:` map, so `cfg.Models` is empty and every branch of
`resolveModel` falls through to a single fallback. Measured on a live delegation:

```
produce phase, /proc/<pid>/cmdline:  -m laguna-s-2.1-free
review  phase, /proc/<pid>/cmdline:  -m laguna-s-2.1-free
```

180 samples at 10-second intervals across both phases: identical, every one. The
architect's frontmatter declares `model: pro`; it received the cheapest model
available. `model: pro` / `model: free` in eleven `ROLE.md` files affects nothing.

So the pipeline in practice is: a cheap model writes, and the same cheap model is
shown nothing (#158) and says `approved`.

**The premise itself held up.** Given a specific brief, the cheap model wrote the
fix for #143 — 363 lines including 203 of tests — and it compiled and passed the
suite. The cost model is not failing. It has never been asked to run.

---

## What was landed

| Commit | What |
|---|---|
| `2b2ba7c` | CI: build, vet, test, gofmt on every push — the repository had none |
| `19bf0b3` | `docs/PRODUCT.md` — the product definition, previously unwritten |
| `09d904a` | #143 — the reviewer receives the real diff |
| `1640338` | #145 — delegated worktrees actually run the gauntlet |
| `aa43044` | #149 — RFC 9457 Problem Details research |
| `874a55e` | #150 — project-total coverage, deterministic |
| `e2428b6` | #147 — role capabilities derived from `ROLE.md` |
| `8a058b9` | #155 — structured logging with `log/slog` |

The logging paid for itself on its first run, producing #158 — three defects in
`SessionResult` that had been invisible: empty output on success, empty stderr,
and a commit count of `3015` against a repository with 106 commits.

---

## Lessons that became mechanisms

Recorded in `.mill/roles/staff/lessons.md`:

- **#16 — commit the agent's output before re-delegating, whatever the verdict.**
  Withholding the commit is the correct review decision and is exactly what makes
  the work destroyable: re-delegation resets the worktree and destroys unstaged
  and untracked files alike (#146, observed five times in one day).
- **#17 — verify by execution, never by inspecting the artefact's shape.** See
  failure mode 3 above.

Two errors worth recording as well, both the author's:

- Three delegations were run against a binary compiled seven hours before HEAD,
  and their results misdiagnosed. **Merging is not deploying.** #155 now records
  binary provenance on every run.
- Hours were spent seeking authorisation for a `--no-verify` commit that was never
  needed. The main repository's hook and a delegated worktree's hook are different
  files with different rules; that was assumed rather than checked. `head -20
  .git/hooks/pre-commit` would have settled it.

---

## Open, and why it matters

| Issue | Why it blocks |
|---|---|
| #153 | roles are never told to hand off — the chain does not run |
| #116 | `model: pro` never reaches dispatch — no cost arbitrage exists |
| #158 | the adapter captures neither output nor stderr |
| #156 | nothing writes to the issue — the escalation ladder cannot be built on top |
| #157 | no supervisor: a delegation that dies stays dead and nothing notices |
| #152 | the gauntlet is hardcoded Go — Mill only works on Go projects |
| #154 | `delegates_to` cannot express fan-out |

#139 holds the product definition. Its central mechanism — an executing role that
raises a hand instead of guessing, and an observer that resolves or escalates one
step — does not exist in any line of code, and depends on #156 to be buildable
at all.
