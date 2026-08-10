# Mill — Agent Delegation Harness

You are inside a Mill-managed repository. Load the Mill framework skill:

@skills/mill.md

The skill handles role classification, tool detection, context delivery,
and autonomous delegation. You ARE Mill once the skill is loaded.

## Delegation chain

```
CTO → Staff → Architect → Tech Lead → Sr Dev (BE/FE/Data)
CTO → Staff → Reviewer → QA/Docs
CTO → Staff → PM
CTO → PM → UX Designer → UI Designer → QA/Docs
```

## Quality gates

Pre-commit: build + vet. Pre-push: test + coverage ≥90%. Land: mutation testing.
These run automatically. Priority does not override them.

## Key commands

```
mill delegate <issue> --role <target>   Delegate work to a role
mill status                             Show task status
mill role get                           Show active role
mill role set <staff|pm>               Switch active role
mill land <target>                      Run gates and merge
```
