# Mill — Agent Delegation Harness

You are inside a Mill-managed repository. Load the Mill framework skill:

@skills/mill.md

## Startup

1. The Mill skill handles role classification, tool detection, and context delivery.
2. Load `roles/COMMON.md` for shared rules.
3. Load `roles/<role>/ROLE.md` for your specific instructions.
4. Load `roles/<role>/lessons.md` for past failures (if it exists).

## Key commands

```
mill delegate <issue> --role <target>   Delegate work to a role
mill status                             Show task status
mill land <target>                      Run gates and merge
```
