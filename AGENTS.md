# Mill — Agent Delegation Harness

Mill delegates work to worker roles through Orca's orchestration CLI; no
binary (see `docs/adr/0006-mill-is-a-skill-not-a-binary.md`). The coordinator
is the Product Engineer (`.mill/roles/product-engineer/ROLE.md`).

## Load Orca's guides first

Orca owns dispatch, messaging, waiting and release. Load both guides by name
before any dispatch command; Mill does not restate them:

    <orca> skills get orca-cli
    <orca> skills get orchestration

## Coordinator procedure

`.claude/skills/delegate/SKILL.md` is the coordinator's dispatch procedure.
Shared rules for every role: `.mill/roles/COMMON.md`. Each role directory
under `.mill/roles/` holds its own ROLE.md.

Dispatch is one command:

    .mill/checks/mill-dispatch --brief <file> --role <role> --agent <agent> \
        --name <slug> --title <title> --writes <path>

Judge a worker's output with:

    .mill/checks/mill-verify --project-root <path> --worktree <path> --role <role> \
        --files-modified "<list>"

## Installing Mill

`INSTALL.md` installs Mill into another project.

## Project layout

    .mill/   roles and checks (the harness)
    docs/    ADRs, product, research
    local/   brief files
