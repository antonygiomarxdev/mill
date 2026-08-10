---
name: using-mill
description: Bootstrap or activate Mill — the multi-agent delegation framework. Use when the user says "using mill" or wants to delegate work to a role-based agent team.
---

# Using Mill

Mill is a multi-agent delegation framework. It turns your session into a
Staff Engineer or Product Manager that autonomously classifies work,
delegates to specialized subagents, and orchestrates phased review chains.

## Activation

When the user says "using mill" or "let's use Mill":

### If Mill is already installed

`.omp/AGENTS.md` exists in the project root. Load the Mill skill:

```
@skills/mill.md
```

Then follow its startup sequence: classify → announce role → delegate.

### If Mill is not installed

`.omp/AGENTS.md` does NOT exist. Bootstrap:

```bash
go install github.com/antonygiomarxdev/mill/cmd/mill@latest
mill init
```

Then reload the session. The harness will discover `.omp/AGENTS.md` and
load the Mill skill automatically on the next session start.

### If Go is not available

Tell the user: "Mill requires Go 1.21+. Install it from https://go.dev/dl/
or run `brew install go`."

## What happens next

Once Mill is active, your agent becomes [Mill · Staff] or [Mill · PM].
Work follows a phased pipeline:

```
FRD(PM) → SPEC(Architect) → TASKS(Tech Lead) → IMPLEMENT(Sr Dev) → REVIEW(Reviewer)
```

The user just speaks naturally. Mill handles delegation, review chains,
state persistence, and quality gates automatically.
