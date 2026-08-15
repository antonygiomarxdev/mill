# Mill — Product Definition

Mill is an **org chart that executes** — a skill plus a policy directory that
turns one AI session into a coordinator dispatching specialised workers.

- Role definitions in Markdown (`.mill/roles/`), gate scripts in bash
  (`.mill/checks/`), and the coordinator's procedure in
  `.mill/skills/using-mill.md`.
- Orca provides the execution substrate: worker spawning, supervision, worktree
  isolation, and the message bus.
- There is no binary and no build step — installing Mill is copying files.

The topology is a **star, not a chain**: the coordinator dispatches one-to-N
and holds the sequence. No worker dispatches another worker. The organisational
sequence is preserved as pipeline stages:

```
intent → FRD → spec(s) → tasks → implementation → review
          PM    Architect  Tech Lead   Sr Dev      Reviewer
```

Each phase is gated by a script in `.mill/checks/`. No artifact, or an artifact
missing its required sections, and the phase does not pass.

**Blocking is a first-class outcome.** A worker that finds the brief
underspecified does not guess — it says what is missing and stops. The
coordinator answers or escalates to the CTO.
