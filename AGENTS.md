# AGENTS.md — Watcher

> **Inheritance pointer:** This submodule inherits the project Constitution
> (v2.3.0, §1-§21) and all universal governance in full from the parent
> repository. See the parent repository's `CONSTITUTION.md` / `CLAUDE.md` /
> `AGENTS.md` for the canonical, authoritative text. Only Watcher
> module-specific content is kept below.

## Repo state
This is a `vasic-digital` / `HelixDevelopment` submodule for the consuming project.

## Critical constraints
- **Anti-bluff:** No placeholders, dead code, vacuous tests. Details in Constitution §1.
- **Containers only:** Every service, DB, build, test runs inside a container.
- **Decoupling:** Reusable components live in public `vasic-digital` submodules.
- **Tests:** 100% coverage across all ten types. Only Unit may use mocks.
- **R-18 Operational Integrity:** No command may suspend/hibernate/lock/terminate/crash the host.

## Git topology
`origin` fetch=GitHub, push=GitFlic. Four remotes configured.
Force-push requires explicit authorization. `--no-verify` is forbidden.
