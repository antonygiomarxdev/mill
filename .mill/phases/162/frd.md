# FRD: First installation by someone else

**Issue:** #162  
**Roadmap:** Item 2 — Have someone else install it

## User need

A developer who has heard of Mill wants to try it on their own project. They have Orca installed, they can read English, and they have a terminal. They do not have context about how Mill was built or what its edges are.

Their experience today: they read `README.md`, follow the steps, and at some point either succeed or stop. If they stop, they either ask a question the README does not answer, or give up silently. The author has no way to know which steps lost them, because the author wrote the instructions and has never watched someone else follow them.

This is the first thing a new user does. If it fails, nothing else matters.

## Functional requirements

1. The README installation section is the only document required. No cross-references to other files, no "see also" links that are actually mandatory.
2. Every shell command in the README runs without modification on the declared target platforms.
3. The README states what must already be true before starting (Orca version, OS, shell, git).
4. Each step that can fail names what failure looks like and what to check.
5. The final step is an observable success — not "you're done" but a command that produces specific output proving it worked.

## Out of scope

- Automating installation (scripted installer, package manager). The manual path must work first.
- Supporting platforms not declared in the README. Pick one and prove it; broaden later.
- Onboarding content beyond installation. This FRD ends at "first dispatch completes."

## Priority

**P0 — blocks everything.**

Until this is verified, Mill is a method one person uses, not a product. Roadmap items 3–7 assume someone else can install it. This gates them all.

## Acceptance criteria

1. `grep -c '^[0-9]\.' README.md` returns ≥1 — numbered steps exist
2. A tester on a clean machine reaches a completed dispatch (`orca orchestration task-list` shows `status: completed`) following only README
3. Tester records every point they stopped or asked — count is captured
4. Each recorded stop-point is fixed in README — `git diff README.md` shows changes for each
5. README declares Orca version, OS, and shell — `grep -E 'Orca|OS:|shell:' README.md` matches
6. Final README step includes a verification command with expected output — `grep -A2 'Verify' README.md` shows command and expected result
7. Tester's environment (Orca version, OS, agent ID) is recorded in a dated file under `docs/`
