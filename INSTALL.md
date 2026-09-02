# Installing Mill

Mill installs into two roots, and this document is the second of them. The two
are named and separated in `docs/adr/0014-two-roots-install-and-project.md`.

The **install root** ships the mechanism — the `delegate` skill, the role
definitions in `.mill/roles/`, and the gate scripts in `.mill/checks/`. It is
identical for every project and is updated by the harness's own plugin or
extension channel.

This document is the **project root** — the state that differs per project:
`.mill/gauntlet` (that project's build, lint and test commands) and
`.mill/role-capabilities` (that project's file patterns). Neither can be
packaged — both differ per project — and both are files a human should see and
approve rather than have a script write unattended. Run each step in the
user's session, with the user watching.

## 1. Install the extension

Ask which harness the project uses, then install Mill's extension for it.
Mill ships one manifest per supported harness; each declares the skill
directory.

**Claude Code** (`.claude-plugin/plugin.json`). Install Mill from its
marketplace:

```
/plugin install mill@<marketplace>
```

`<marketplace>` is the name Mill's marketplace is registered under in Claude
Code. Mill is not yet published to a marketplace, so this exact command cannot
run today; when the marketplace exists, this is the command. If the command
fails or the plugin is not listed after running it, the extension is not
registered — check the harness's plugin list for `mill` before continuing, and
stop if it is not there.

**Codex** (`.codex-plugin/plugin.json`). Add Mill's marketplace, then install
the plugin:

```
codex plugin marketplace add <source>
codex plugin add mill@<marketplace>
```

`<source>` is a local path, `owner/repo`, or Git URL naming Mill's marketplace,
and `<marketplace>` is the name that marketplace declares. Mill ships its
marketplace manifest as `.agents/plugins/marketplace.json`, which declares the
name `mill-dev` and the plugin `mill`, so once that marketplace is added the
install is `codex plugin add mill@mill-dev`. Mill is not yet published, so
these commands cannot run against a live marketplace today. If the add fails
or `codex plugin list` does not show `mill`, the marketplace was not added —
re-run `codex plugin marketplace add`, then `codex plugin list`, and stop if
`mill` is still absent.

**Cursor** (`.cursor-plugin/plugin.json`). Cursor has no single package-install
command: it reads a repository's manifest from an added marketplace. Add Mill's
marketplace:

```
cursor-agent plugin marketplace add <gitUrl>
```

`<gitUrl>` is the Git URL of Mill's repository; the Cursor app then lists the
plugin for install from the manifest. For a local checkout, the terminal agent
loads the plugin directory directly with `cursor-agent --plugin-dir <path>`.
Mill is not yet published, so the marketplace command cannot run today. If the
plugin does not appear in the Plugins panel after the marketplace is added, the
manifest was not read — confirm `.cursor-plugin/plugin.json` is present at the
repository root and stop if it is not.

**Any other harness.** Mill ships a manifest only where a harness can load the
skill, inject the coordinator's identity per prompt, and run the bash gates. A
harness that cannot do all three is not supported.
`docs/adr/0012-harness-extension-plus-prompt.md` records, for every harness
Mill has examined, which of the three hold and which do not — read its table
before installing for a harness not named here, and stop if its row is not
`keep` or `write`.

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

- Mill ships no git hook script at all: nothing in it runs on any git event.
- Mill writes nothing outside `.mill/` and the entry files.
- Mill never writes git configuration: no `git config`, no `core.hooksPath`.
  Issues #148 and #173 exist because an earlier Mill did exactly those things;
  this one does not.
