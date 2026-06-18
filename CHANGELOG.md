# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.1] - 2026-06-18

### Added

- Groups as first-class named wrappers with their own base path and aliases;
  `rig group new/list/rename/move/delete/alias` with real filesystem moves.
- Projects identified by the `(group, name)` pair with fully derived paths;
  local-only projects are first-class.
- `rig create`, `rig adopt`, and `rig clone` (`--read` / `--public`) with group
  auto-vivification and smart name-collision suggestions. `clone` detects a
  public repo and offers the keyless HTTPS path instead of a deploy key.
- Multi-key deploy-key model: many keys per repo, read and write as independent
  key objects; `rig key create/list/delete`.
- Per-project push guard (the `no_push` sentinel) via `rig project guard`, plus
  `rig project key/origin/upstream/finish/delete/alias`. `origin add` reads a
  smart `owner/repo` argument inside a project and demotes a prior source to
  `upstream` when attaching a new writable origin; `project upstream add` accepts
  a positional `owner/repo`.
- Pluggable handoff delivery (clipboard, drop, link, print, file, gh) with a
  dual-mode finish/verify loop; verification is always git-over-SSH.
- Optional read-only GitHub metadata token (`rig config token set/remove/status`).
- Resolver-driven navigation (`rig path`/`rig cd`), with a bare-token fuzzy-nav
  fallback (`rig <token>`), zsh/bash shell integration, and config-defined
  launchers.
- Optional `[guard]` (`expected_user`/`expected_host`) that warns and refuses on
  an unexpected host; skipped in disposable dev mode.
- Reusable project types and per-project `rig.toml` overlays with hooks and
  extra commands.
- Configuration via `config.toml` with `rig config show/get/set/edit`.
- Scriptable destructive commands: `--yes` on `group rename`/`group move`/`type
  delete` and `project delete --force` skip confirmation prompts.

[Unreleased]: https://github.com/AndrewMast/rig/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/AndrewMast/rig/releases/tag/v0.0.1
