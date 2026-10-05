# core-engine

Runtime authorization and enforcement engine for AI agents and MCP servers.

> **Status:** early development. Nothing here is ready for production use.

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
docs/                    Architecture, threat model, ADRs
test/                    Integration and end-to-end tests
scripts/                 Dev and CI helper scripts
```

### Dependency rules

- `internal/protocol/mcp` may import the other internal modules. They must never import it.
- `internal/evidence` must not import any other internal module.

## Build

Requires Go 1.26 or later.

```sh
make build      # builds bin/engine
make test       # runs unit tests with -race
make lint       # runs golangci-lint
```

Check a policy file:

```sh
./bin/engine -policy policies/example.yaml
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Found a vulnerability? Do not open a public issue. See [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
