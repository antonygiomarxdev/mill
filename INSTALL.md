# Installing Mill

Mill installs in two halves, and this document is the second half.

The **extension** ships the mechanism — the `delegate` skill, the two hooks
that keep the coordinator's identity in context, and the gate scripts in
`.mill/checks/`. It is identical for every project and is updated by the
harness's own plugin or extension channel.

This document is the **prompt** that guides the project's own state:
`.mill/gauntlet` (that project's build, lint and test commands) and
`.mill/role-capabilities` (that project's file patterns). Neither can be
packaged — both differ per project — and both are files a human should see and
approve rather than have a script write unattended. Run each step in the
user's session, with the user watching.

## 1. Install the extension

Ask which harness the project uses, then install Mill's extension for it.

**Claude Code.** Install Mill from its marketplace:

```
/plugin install mill@<marketplace>
```

`<marketplace>` is the name Mill's marketplace is registered under in Claude
Code. Mill is not yet published to a marketplace, so this exact command cannot
run today; when the marketplace exists, this is the command. If the command
fails or the plugin is not listed after running it, the extension is not
registered — check the harness's plugin list for `mill` before continuing, and
stop if it is not there.

**Pi.** Mill's repo-root `package.json` declares the `pi-package` keyword and a
`pi` block naming its skill directory. Install the package with Pi's package
command for the Mill repository; Pi reads the skill path from those fields.
Mill ships no `.pi/extensions/*.ts`, so Pi's context-injection hook is not
implemented — only the skill is available.

**Any other harness.** Mill's manifests declare Claude Code and Pi only. Do not
install for a harness Mill has not been verified on: report that the harness is
unsupported and stop.

## 2. Create `.mill/gauntlet`

Ask the user for the project's build, lint and test commands. Then create the
file:

```
mkdir -p .mill
cat > .mill/gauntlet <<'EOF'
#!/bin/bash
build="<the build command>"
lint="<the lint command>"
test="<the test command>"
EOF
```

Leave out the line for any step the project does not have — an unset step is
valid. `mill-verify` reports it and skips it:

```
mill: build: not configured (no build=... in .mill/gauntlet)
```

A step that is set runs, and its command must exit 0. If the `cat` fails, the
`.mill` directory is missing or read-only: re-run `mkdir -p .mill`, confirm the
user has write access, and re-run the `cat`.

## 3. Create `.mill/role-capabilities`

Ask which file extensions the project's code, docs and config actually use.
Write them, one category per line:

```
cat > .mill/role-capabilities <<'EOF'
code=""
docs=".md"
config=".yml .yaml .json .toml"
policy=".md .sh .example"
scripts=".sh"
design=""
EOF
```

- `code=""` is correct for a project with no application code.
- An empty category means no role may write files of that category — the
  capability gate fails closed, deliberately.
- The categories are `code`, `docs`, `config`, `policy`, `scripts`, `design`.

If the `cat` fails, same as step 2: `mkdir -p .mill`, confirm write access,
re-run.

## 4. Verify the install

Run the preflight check:

```
.mill/checks/mill-preflight
```

A passing run prints the coordination-guide note and exits 0:

```
Orca's coordination guide: load with 'orca skills get orca-cli' and 'orca skills get orchestration'
```

If it prints `error: Orca is not running. Run 'orca open' first.` start Orca
(`orca open`) and re-run. If it prints `error: Not a Mill project (no
.mill/roles/)` the extension's role files are not in the project — go back to
step 1 and re-check the install.

## 5. What Mill does not do

- Mill takes no git hooks: nothing in it runs on commit or push.
- Mill writes nothing outside `.mill/` and the entry files.
- Mill never changes git configuration: no `git config`, no `core.hooksPath`.
  Issues #148 and #173 exist because an earlier Mill did exactly those things;
  this one does not.
