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
@.mill/skills/mill.md
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

### 1. Create the Mill Map

Every Mill effort starts with a **map issue** — a tracking issue labelled
\`mill:map\` that charts the work from idea to done. Adapted from wayfinder.

```bash
gh issue create --title "Mill Map: <destination>" \
  --label "mill:map" \
  --body "## Destination
<what done looks like>

## Phases
| Issue | Phase | Role | Status |
|-------|-------|------|--------|

## Decisions so far
"
```

### 2. Child issues follow the pipeline

Each piece of work becomes a child issue of the map:

```
Mill Map (#1)
  ├─ feat: dark mode → FRD → SPEC → TASKS → IMPL → REVIEW
  └─ fix: login bug → FRD → SPEC → ...
```

### 3. Labels

| Label | Phase |
|-------|-------|
| \`mill:map\` | Tracking map |
| \`stage:spec\` | FRD needed (PM) |
| \`stage:design\` | SPEC needed (Architect) |
| \`stage:dev\` | Implementation (Sr Dev) |
| \`stage:review\` | Under review (Reviewer) |
