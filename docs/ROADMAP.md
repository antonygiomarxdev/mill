# Roadmap

Where Mill is, and what has to be true before each next thing.

**Status: a working method, not yet a product.** It works on one machine,
operated by the person who knows its edges. Everything below is the distance
from that to something someone else can use.

Written 2026-08-15, after two days of running Mill against itself. Every claim
of "works" here was measured; every claim of "unknown" means nobody has tried.

---

## Where we are

**Works, verified end to end**

- The coordinator dispatches a role worker through Orca, with a brief built from
  that role's `ROLE.md` and explicit acceptance criteria
- A worker raises a hand, the coordinator answers, the worker continues
- Role capabilities are enforced at commit by `role-enforce`: roles declare
  categories in frontmatter, resolved to file patterns per project via
  `.mill/role-capabilities` — adding a role is writing a Markdown file
- Phase gates block a phase whose artifact is missing or malformed
- The cost tiers dispatch: `--agent claude --model <id>` for roles that think,
  `--agent command-code` for roles that write
- Real work landed this way: `internal/ledger` 77.1% → 94.3%, verified
  independently and merged

**Does not work, or was never tried**

- The gauntlet shipped to new projects runs Go tooling unconditionally
- Nobody outside this machine has installed Mill
- There is no eval; verification is a human reading a diff
- Whether the delegation chain beats a single agent has never been measured

---

## 1. Make it work outside Go — #152

**Why first:** until this is done, "any project" means "any Go project".
`scaffold/.mill/checks/` runs `go build`, `go vet` and `go test` with no
condition. A TypeScript project installs Mill and its first commit fails.

**What:** the commands move into configuration, declared at install or detected
from the project. The gauntlet's *shape* — build, lint, test, coverage, each a
named check that blocks — is language-agnostic and stays.

**Done when:** `mill init` in a non-Go repository produces a gauntlet that runs
that ecosystem's commands, and no check invokes Go tooling unless the project is
Go. A fixture project in a second language proves it.

**Size:** three bash scripts reading a config file. There is no binary to thread
anything through any more.

---

## 2. Have someone else install it

**Why:** the installation in `README.md` was written by its author and executed
by nobody. It is the first thing a new user does and the only claim in the
project that is unmeasured.

**What:** a person who is not the author, on a machine that is not this one,
follows only the README: install Orca, register an agent, copy `scaffold/`, link
the skill, dispatch one worker.

**Done when:** they complete it without asking a question the README does not
answer — and every place they got stuck is fixed.

**Size:** half an hour, plus whatever it breaks.

---

## 3. Survive the first run without its author

Today, with the author present, one session hit: a brief that lands unsubmitted
and never starts, a provider connection error that nothing surfaces, an `--ack`
used wrongly that made a counter lie for hours, and a manual `task-update` that
left a task `blocked`.

A new user meets those four and leaves.

**What:** each failure mode either cannot happen, or explains itself where it
happens. The skill already documents them; documentation is not the same as
recovery.

**Done when:** a dispatch that parks, dies, or stalls says so, and says what to
do, without anyone reading a guide.

---

## 4. Measure whether the chain is worth it

**Why:** this can invalidate the design, and it is cheap. The research
(`docs/research/harness-engineering-and-evals.md`) finds the field arguing
against multi-agent architectures for coding specifically — fewer parallelisable
components, shared context needed, conflicting implicit decisions.

**What:** the same task, twice. Once through the full chain — PM, Architect,
Tech Lead, Sr Dev. Once as a single agent given the complete brief. Measure
quality, cost, and wall-clock.

**Done when:** we can say which is better and by how much. If the single agent
matches, Mill is overhead with good documentation and we should know that now
rather than after building more on top.

---

## 5. Evals

**Why:** verification is a human reading a diff, and that human failed twice in
one session. Every source in the research treats evals as foundational and warns
they get harder to build the longer you wait.

**What:** start with computational graders in bash, from failures that already
happened — does the artifact exist, does it build, does it pass its gates, did
the declared tier actually dispatch. `docs/FINDINGS-2026-08.md` is the dataset;
it was written from real defects.

**Done when:** a delegation's result can be judged without a human reading it,
for the classes of failure we have already seen.

---

## 6. Close the loop back to the issue — #156

**Why:** the product says the issue is the single record. Nothing writes to it.
Every comment in this repository was written by hand.

**What:** Mill posts what it knows — which role ran, on which model, what the
verdict rests on. Stage labels advance. A raised hand reaches the issue, so
escalation survives the session that produced it.

**Done when:** a delegation leaves a trace on its issue that someone who was not
in the session can follow.

---

## 7. Learn from defects — #110, #139

**Why:** this is in the product definition and has no mechanism. Lessons are
written to files no prompt reads.

**What:** a defect is linked to the issue and the role that produced it, and
reaches that role the next time it works. Defect (found in QA) and bug (found in
production) stay distinct, because the distinction says where the net failed.

**Done when:** a role that made a mistake receives that lesson in its next brief,
without anyone remembering to include it.

---

## Not on this roadmap, deliberately

- **Unattended operation.** Mill needs a session to drive it (ADR 0006). Cron and
  CI are out until something changes.
- **Replacing Orca or abstracting over it.** One backend, adopted directly. A
  seam gets extracted when a second backend is real, not before (#161).
- **More roles.** Twelve is enough to test the idea. Adding roles is cheap and
  proves nothing.

---

## The order, and why

**1 and 2 are the product gate.** Until Mill installs somewhere else and runs on
a project that is not Go, there is no product to improve — there is a method one
person uses.

**4 comes next because it can end the project.** It is a day's work and it
answers whether any of the rest is worth building.

**3, 5, 6, 7 are what make it good.** They matter enormously and none of them
matters if 1, 2 and 4 do not hold.
