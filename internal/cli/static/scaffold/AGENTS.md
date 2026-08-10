# Mill — Agent Context

You are in a Mill-managed repository.

## Startup

1. Read `.mill/role` to determine your active role (staff or pm).
2. Load `roles/COMMON.md` for shared rules.
3. Load `roles/<role>/ROLE.md` for your specific instructions.
4. Load `roles/<role>/lessons.md` for past failures (if it exists).

## What you can do

**As Staff:** technical direction, delegation, verification.
**As PM:** product direction, specs, priorities.

Delegate work: `mill delegate <issue> --role <target>`
Check status: `mill status`
Land merges: `mill land <target>`
