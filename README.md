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
| Evidence | `internal/evidence` | ✅ Hash-chained log on disk, verified on open |
| MCP adapter | `internal/protocol/mcp` | ✅ stdio proxy, fails closed (HTTP transport planned) |
| Enforcement | `internal/enforcement` | ⬜ Planned (done inside the MCP proxy for now) |
| Identity | `internal/identity` | ⬜ Planned (the agent is named by `-subject`) |
| Delegation | `internal/delegation` | ⬜ Planned |

See [CHANGELOG.md](CHANGELOG.md).

## See it in action

```sh
make demo
```

An agent talks to a fake GitHub MCP server through the engine. You see an allowed call,
a denied call, a tool hidden from the agent, the evidence log, and a tampered log being caught:

```
  ✔ ALLOW  mcp:github/create_issue  (allowed_by_rule)
  ✘ DENY   mcp:github/delete_repo  (denied_by_rule)

  what the agent received:
  ← #2 tools/list → create_issue, list_repos
  ← #3 result: the server ran create_issue
  ← #4 error -32001: denied by Axyra policy

  evidence error: evidence.jsonl: evidence chain is broken: record #1: content does not match its hash
```

## Use it with an MCP client

Put `engine proxy` in front of any stdio MCP server. The client starts the engine,
the engine starts the server. Example for Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/path/to/engine",
      "args": [
        "proxy",
        "-policy", "/path/to/policy.yaml",
        "-evidence", "/path/to/evidence.jsonl",
        "-subject", "agent:claude-desktop",
        "-server", "filesystem",
        "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "/path/to/folder"
      ]
    }
  }
}
```

With Claude Code:

```sh
claude mcp add filesystem -- /path/to/engine proxy -policy policy.yaml \
  -evidence evidence.jsonl -subject agent:claude-code -server filesystem \
  -- npx -y @modelcontextprotocol/server-filesystem /path/to/folder
```

A matching policy that lets the agent read but never write
([policies/filesystem-read-only.yaml](policies/filesystem-read-only.yaml)):

```yaml
version: 1
rules:
  - id: read-only
    effect: allow
    subjects: ["agent:claude-desktop", "agent:claude-code"]
    actions: ["tools/call"]
    resources:
      - "mcp:filesystem/read_*"
      - "mcp:filesystem/list_*"
      - "mcp:filesystem/search_files"
      - "mcp:filesystem/get_file_info"
```

- Every `tools/call` is checked against the resource `mcp:<server>/<tool>`.
- Denied calls never reach the server; the agent gets error `-32001` with the rule and reason.
- Every decision is appended to the evidence log. Check it any time:
  `engine verify -evidence evidence.jsonl`.
- Design and limits: [ADR 0005](docs/adr/0005-mcp-stdio-proxy-fails-closed.md).

## Write a policy

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
make demo       # runs the MCP proxy demo
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

[Apache-2.0](LICENSE). Copyright 2026 SH Consulting.
Third-party notices are in [NOTICE](NOTICE).
