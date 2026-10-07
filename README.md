<div align="center">

# core-engine

**Runtime authorization and enforcement engine for AI agents and MCP servers.**

[![CI](https://github.com/AxyraSec/core-engine/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/AxyraSec/core-engine/actions/workflows/ci.yml)
[![Security](https://github.com/AxyraSec/core-engine/actions/workflows/security.yml/badge.svg?branch=main)](https://github.com/AxyraSec/core-engine/actions/workflows/security.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](go.mod)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue?style=flat-square)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-first-6E56CF?style=flat-square)](#what-it-does)
[![Status](https://img.shields.io/badge/status-early%20development-orange?style=flat-square)](#status)

</div>

> [!WARNING]
> Early development. Nothing here is ready for production use.

## What it does

Axyra checks every action an AI agent takes, at runtime, before it happens.
It is MCP-first, but not MCP-only.

The engine follows one chain:

```
Identity → Delegation → Authorization → Enforcement → Evidence
```

- **Identity**: who is the agent, and who does it act for?
- **Delegation**: what rights were handed to the agent, by whom?
- **Authorization**: is this action allowed by policy?
- **Enforcement**: block or allow the call, at runtime.
- **Evidence**: keep a tamper-evident record of every decision.

## Status

| Module | Package | State |
|---|---|---|
| Authorization | `internal/authz` | ✅ Deny-by-default rules, YAML policy files |
| Evidence | `internal/evidence` | 🟡 Hash-chained log, in memory only |
| MCP adapter | `internal/protocol/mcp` | ⬜ Planned |
| Enforcement | `internal/enforcement` | ⬜ Planned |
| Identity | `internal/identity` | ⬜ Planned |
| Delegation | `internal/delegation` | ⬜ Planned |

The modules are not wired together yet: the engine does not intercept agent calls today.
See [CHANGELOG.md](CHANGELOG.md).

## Quick start

Requires Go 1.26 or later.

```sh
make build
./bin/engine -policy policies/example.yaml
# axyra core-engine <version>
# policy OK: 2 rules
```

A policy is a YAML file. A request is allowed only if an `allow` rule matches it,
and a matching `deny` rule always wins:

```yaml
version: 1
rules:
  - id: allow-billing-bot-github
    effect: allow
    subjects: ["agent:billing-bot"]
    actions: ["tools/call"]
    resources: ["mcp:github/*"]

  - id: deny-github-delete-repo
    effect: deny
    subjects: ["*"]
    actions: ["tools/call"]
    resources: ["mcp:github/delete_repo"]
```

Format and rules: [docs/architecture.md](docs/architecture.md#policy-file).

## Development

```sh
make build      # builds bin/engine
make test       # runs unit tests with -race
make lint       # runs golangci-lint
make security   # runs govulncheck and gitleaks
make docker     # builds the container image
```

Every pull request runs lint, tests, build, a Docker build, gitleaks, govulncheck and trivy.
Releases are signed with cosign (keyless) and ship an SBOM and build provenance,
see [ADR 0004](docs/adr/0004-signed-releases-with-cosign.md).

## Repository layout

```
cmd/engine/              Entry point of the engine binary
internal/identity/       Agent and user identity
internal/delegation/     Delegation of rights to agents
internal/authz/          Policy evaluation
internal/enforcement/    Runtime allow/deny
internal/evidence/       Tamper-evident decision log
internal/protocol/mcp/   MCP protocol adapter
pkg/                     Public Go packages (stable API only)
policies/                Example and default policies
api/                     API definitions
deploy/docker/           Container image
deploy/onprem/           On-prem deployment files
docs/                    Architecture, threat model, evidence chain, ADRs
test/                    Integration and end-to-end tests
scripts/                 Dev and CI helper scripts
```

### Dependency rules

- `internal/protocol/mcp` may import the other internal modules. They must never import it.
- `internal/evidence` must not import any other internal module.

Both rules are enforced by `golangci-lint` (depguard).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Found a vulnerability? Do not open a public issue. See [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
