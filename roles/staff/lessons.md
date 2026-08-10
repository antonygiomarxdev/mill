# Staff Lessons

Staff-specific failures from autoconstruction cycles.

---

## 1. Approval without code review is not approval

**When:** #14 — first autoconstruction cycle.

**What happened:** Runner MVP approved on mechanical gates alone. Code had no Clean Architecture, used cobra against plan constraints, delegate was a stub.

**Lesson:** Green tests + green build ≠ good code. Read the code. Check architecture. Verify behavior, not output text.

**Mechanised:** `grep -c "cmd -p" internal/` catches stubs. Architecture review requires judgment.

---

## 2. During bootstrap, Staff IS the review chain

**When:** #14 and #16 — Tech Lead and Reviewer roles didn't exist yet.

**What happened:** Staff skipped review gates. #14 had no review at all. #16 first pass had wrong classifier.

**Lesson:** When a role doesn't exist, the next role up absorbs its responsibility. During bootstrap, Staff = Tech Lead + Reviewer + Staff. Load each ROLE.md and execute each gate.

**Mechanised:** The `reviewed_by approved` gate must be unskippable.

---

## 3. Know the expected duration — stuck agents burn tokens silently

**When:** #22 — repair package took 12 minutes for a task that should take 2.

**What happened:** Agent wrote correct code in ~2 minutes, then spent 10 minutes fighting CommandCode tool parser bugs (`--amend`, `=`, `---`). It never errored — just kept thinking and retrying.

**Lesson:** Estimate task duration from the brief. If >3x estimate, the agent is stuck. Kill it, verify what exists, decide: land, fix manually, or re-spawn.

**Mechanised:** Runner should enforce time budgets per task. Exceeded → auto-kill → flag Staff.
