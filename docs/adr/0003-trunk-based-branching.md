# 3. Use trunk-based development

- Status: Accepted
- Date: 2026-10-04

## Context

We are a small team shipping a security product. Long-lived branches drift,
create painful merges, and hide untested code. We want `main` to always be
releasable, and every change to be reviewed and checked.

## Decision

We use trunk-based development.

- `main` is the only long-lived branch. It is always stable: it builds and all checks pass.
- No `develop` branch. No release branches.
- Work happens on short-lived branches cut from `main`, merged within a few days:
  - `feat/*` for features
  - `fix/*` for bug fixes
  - `docs/*` for documentation
  - `chore/*` for tooling, CI, dependencies, refactoring
- Every change goes through a pull request with required status checks and review.
- Pull requests are merged with **squash merge** only. The PR title becomes the commit
  message on `main`, so it must follow [Conventional Commits](https://www.conventionalcommits.org/).
- `main` keeps a linear history. Force pushes and deletion of `main` are blocked.
- Commits on `main` must be signed.
- Releases are SemVer tags on `main` (`vMAJOR.MINOR.PATCH`). A fix for a release is
  made on `main` and shipped in a new tag.
- Unfinished work that must land early is hidden behind a feature flag or not wired in yet.

## Consequences

- Small, frequent merges. Fewer conflicts.
- One commit per PR on `main`, which makes history easy to read and revert.
- Conventional Commits let us generate the changelog and pick the next version.
- We cannot patch an old release line without also shipping what is on `main`.
  This is acceptable while the project is young. If we need it later, we will write a new ADR.
- The rules are enforced by a GitHub ruleset. See [docs/github-settings.md](../github-settings.md).
