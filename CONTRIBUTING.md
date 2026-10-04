# Contributing

Thanks for helping. This page explains how we work.

## Branches

We use trunk-based development. See [ADR 0003](docs/adr/0003-trunk-based-branching.md).

- `main` is the only long-lived branch. It must always build and pass tests.
- There is no `develop` branch and no release branch.
- Work happens on short-lived branches, cut from `main`:

| Prefix    | Use for                          | Example                   |
|-----------|----------------------------------|---------------------------|
| `feat/`   | New features                     | `feat/mcp-tool-filter`    |
| `fix/`    | Bug fixes                        | `fix/evidence-hash-order` |
| `docs/`   | Documentation only               | `docs/threat-model`       |
| `chore/`  | Tooling, CI, deps, refactoring   | `chore/bump-go`           |

Keep branches small and short. Aim to merge within a few days.

## Commits

We use [Conventional Commits](https://www.conventionalcommits.org/).

```
<type>(<optional scope>): <short summary>
```

Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`.

Examples:

```
feat(authz): evaluate delegation scope before policy
fix(evidence): keep entry order stable on concurrent writes
ci: pin golangci-lint version
```

Breaking changes: add `!` after the type (`feat!: ...`) and a `BREAKING CHANGE:` footer.

Commits must be signed (GPG, SSH or S/MIME).

## Pull requests

- One topic per PR. Keep it small.
- The PR title must follow Conventional Commits. It becomes the commit message on `main`.
- PRs are merged with **squash merge** only.
- All required checks must pass (lint, tests, build, security scans).
- At least one approval from a code owner is required.
- Never commit secrets. Run `make security` before pushing.
- If your change affects a trust boundary, update [docs/threat-model.md](docs/threat-model.md).

## Releases

Releases are SemVer tags on `main` (`v0.1.0`, `v1.2.3`). There are no release branches.
Release images are signed. See [ADR 0004](docs/adr/0004-signed-releases-with-cosign.md).

## Module boundaries

- `internal/protocol/mcp` may import the other internal modules. They must never import it.
- `internal/evidence` must not import any other internal module.

## Local checks

```sh
make lint
make test
make build
make security
```
