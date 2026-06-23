# CLAUDE.md — Watcher

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

## Submodule-Specific Notes

`digital.vasic.watcher` is a standalone, project-agnostic Go filesystem-watch
module (recursive watch, debounce, filtering, i18n). It depends on no consuming
project; integrate via its public `watcher.New(cfg)` / `Watch(ctx, dir)` API.
