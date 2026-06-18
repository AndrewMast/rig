# rig

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)](LICENSE)

A single CLI to stand up, authenticate, navigate, and manage local coding
projects. rig organizes checkouts into groups, derives every project path,
manages per-repo SSH deploy keys, and produces the exact GitHub mutations a
project needs — delivered however you like (clipboard, file, or run directly
with `gh`).

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/AndrewMast/rig/master/install.sh | bash
```

Installs to `~/.local/bin` (override with `INSTALL_DIR`). The installer verifies
the download by SHA256 and, when `gh` is present, by GitHub build-provenance
attestation. It offers to add zsh shell integration so `rig cd` can change your
shell's directory and completions load.

Verification flags: `--without-attestation` (checksum only, non-interactive),
`--require-attestation` (hard-fail without provenance).

## Concepts

- **Group** — a named folder that owns a base path and holds projects. Group
  name = folder name. Groups auto-vivify.
- **Project** — a managed git checkout identified by `(group, name)`; its path is
  derived as `<base>/<group>/<name>`. Local-only projects are first-class.
- **Key** — a per-repo SSH deploy key. Many per repo; read and write are
  independent keys.
- **Handoff** — the GitHub mutations rig emits (repo create, deploy-key add),
  delivered by a pluggable method and verified over git-over-SSH.

## Common commands

```sh
rig create Group/name           # scaffold a new local-only project
rig adopt [path]                # adopt an existing folder (defaults to cwd)
rig clone owner/repo [--read|--public]
rig list                        # list projects
rig status [token]              # per-project state, key, guard, git status

rig cd <token>                  # jump to a project (needs shell integration)
rig path <token>                # print the resolved path

rig project origin add|remove   # host a local project / unhost
rig project key <g/n>           # pick / create / re-bind the deploy key
rig project guard <g/n> on|off  # toggle the push guard
rig project finish <g/n>        # verify + complete a pending project

rig key create owner/repo [--write] [--label …]
rig group new|list|rename|move|delete
rig type new|list|show|delete
rig config show|get|set|edit
rig self update|uninstall|version
```

Every command accepts zero args and prompts for missing values; flags are the
scriptable path.

## Development

```sh
gofmt -l . && go vet ./... && go test ./...
```

The codebase separates pure logic (`model`, `registry`, `config`, `resolver`,
`types`, `handoff`, `selfupdate`) from IO behind interfaces (`git`, `gh`,
`keygen`, `clock`) with real and fake implementations, so behavior is verified
by tests with IO faked.

Disposable dev mode: set `RIG_HOME` to a throwaway directory and rig keeps its
registry, ssh dir, and project base under that root with guards skipped.

### Solo

This repo ships a [`solo.yml`](solo.yml) for [Solo](https://soloterm.com). Open
the project in Solo to run the checks from the **Go Test** command.

## License

rig is released under the [MIT License](LICENSE).
