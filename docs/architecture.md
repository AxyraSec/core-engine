# Architecture

## Overview

The engine is a single Go binary. Today it runs as an MCP proxy: it sits between an
agent (the MCP client) and an MCP server, and checks every tool call against a policy.

## Request flow

```
agent ──tools/call──▶ engine proxy ──(allowed + recorded)──▶ MCP server
                         │   ▲
              authz ◀────┘   └──── error -32001 if denied
                         │
              evidence (append + fsync)
```

1. The agent sends `tools/call` with a tool name.
2. The proxy builds the request `subject` / `tools/call` / `mcp:<server>/<tool>`.
3. `internal/authz` decides.
4. `internal/evidence` appends the decision to the log and syncs it to disk.
5. Allowed and recorded: the call goes to the server. Otherwise the agent gets an error.

## Identity

## Delegation

## Authorization

`internal/authz` decides if a request is allowed. A request has a subject (the agent),
an action (for example `tools/call`) and a resource (for example `mcp:github/create_issue`).

- Deny by default: a request is allowed only if an allow rule matches it.
- A matching deny rule always wins over allow rules.
- A request with an empty field is denied.
- Patterns are exact values, `*`, or a prefix ending with `*` (`mcp:github/*`).
- Rules are validated when the policy is built. A built policy cannot change.

Each decision carries the rule ID and a stable reason, for the evidence log.

### Policy file

Policies are written in YAML. See [policies/example.yaml](../policies/example.yaml).

```yaml
version: 1
rules:
  - id: allow-billing-bot-github
    effect: allow            # allow or deny
    subjects: ["agent:billing-bot"]
    actions: ["tools/call"]
    resources: ["mcp:github/*"]
```

Loading is strict, and a file is either fully loaded or rejected:

- `version: 1` is required.
- Unknown fields (typos such as `efect`) and duplicate keys are rejected.
- One YAML document per file, at most 1 MiB.
- Every rule then goes through the same validation as rules built in code.

`engine -policy <file>` checks a file and exits with a non-zero code if it is invalid.

## Enforcement

## Evidence

`internal/evidence` records every decision in a hash chain: each record holds the hash of
the previous one, so any later change is detected by `Verify`. It does not import any other
internal module; callers copy the decision into an `evidence.Entry`.

`FileLog` stores the chain in a JSON Lines file, syncs every record to disk, and refuses to
open a file that does not verify. `engine verify -evidence <file>` checks a file.
See [evidence-chain.md](evidence-chain.md).

## Protocol adapters

### MCP

`internal/protocol/mcp` implements the proxy for the stdio transport.
See [ADR 0005](adr/0005-mcp-stdio-proxy-fails-closed.md) for the design and its limits.

```sh
engine proxy -policy policies/example.yaml -evidence evidence.jsonl \
  -subject agent:billing-bot -server github -- <MCP server command>
```

- Stdout carries MCP messages only. Logs (one line per decision) go to stderr.
- Denied calls get JSON-RPC error `-32001` with `resource`, `rule_id` and `reason`.
- `tools/list` responses only show the tools the agent may call.
- Fails closed: unparsable messages, batches and calls that cannot be recorded are refused.

## Module dependencies

- `internal/protocol/mcp` imports `internal/authz` and `internal/evidence`. They never import it.
- `internal/evidence` imports no other internal module.
- `cmd/engine` wires everything together.

Both rules are checked by depguard in `golangci-lint`.

## Deployment

### Docker

### On-prem
