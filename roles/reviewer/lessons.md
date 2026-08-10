# Reviewer Lessons

---

## 1. Spec compliance is not gate compliance

**When:** #16 first review pass.

**What happened:** Mechanical gates passed (build, test, no cobra, time.Time). But the Sr. Dev implemented a verdict classifier (APPROVED/CHANGES/REJECTED) instead of the session classifier the spec asked for (OK/FATAL/AUTH/...). Gate check said "something was built." Spec check says "the right thing was built." Only spec check caught the error.

**Lesson:** Verify what was asked, not what was delivered. Mechanical gates are necessary but not sufficient. Every acceptance criterion must be checked against the code. A criterion like "Classification: OK, FATAL, MAX_TURNS, AUTH, NO_CREDIT, RATE_LIMITED, TRANSIENT, BLOCKED" means grep for those exact strings. If the code exports different types, CHANGES.

**Mechanised:** Partially. Post-hook: acceptance criteria pattern-match against exported types. Spec says `X` types, code exports `Y` types → auto-reject.
