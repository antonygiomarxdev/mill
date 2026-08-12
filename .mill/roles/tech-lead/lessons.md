# Tech Lead Lessons

---

## 1. Atomic tasks need complete interface contracts

**When:** #26 — classify signature change broke delegate.go.

**What happened:** classify changed from `Classify(string) Verdict` to `Classify(int, string) Classification`. The task brief only said "fix classify.go." It didn't list callers that needed updating. Build broke on integration.

**Lesson:** When decomposing a task that changes a public interface, the task scope must include updating all callers. Or: the interface change goes in one task, caller updates go in a separate dependent task. Either way, the contract is explicit in the brief.

**Mechanised:** Yes — `go build` catches compile errors. But the process lesson is: Tech Lead must identify all callers when writing task briefs, not assume the Sr. Dev will find them.
