# 5. Enforce MCP as a stdio proxy that fails closed

- Status: Accepted
- Date: 2026-10-10

## Context

Agents call tools through MCP servers. Most MCP servers run locally and talk to the
client over stdio: newline-delimited JSON-RPC 2.0. We need to check every tool call
before it reaches the server, without changing the client or the server.

## Decision

The engine wraps the MCP server command: `engine proxy [flags] -- <server command>`.
The client starts the engine instead of the server, and the engine starts the server.

- Every `tools/call` is evaluated by `internal/authz` with action `tools/call` and
  resource `mcp:<server>/<tool>`, then recorded in the evidence log.
- A call is forwarded only if it is allowed **and** recorded. If the evidence log
  cannot be written, the call is denied (`evidence_unavailable`).
- A denied call gets a JSON-RPC error, code `-32001`, with the resource, rule ID and
  reason in `error.data`. It never reaches the server.
- `tools/list` responses are filtered: tools the agent may not call are removed.
  This is a convenience; the check that matters is on `tools/call`.
- The proxy fails closed: unparsable messages get a parse error, JSON-RPC batches
  (removed from MCP in 2025-06-18) are refused, a call without a tool name is denied.
- Messages are forwarded as the proxy parsed them, not as received. Duplicate keys
  keep only the last value, and keys that differ from `method`, `params`, `id`,
  `jsonrpc`, `name` or `arguments` only by case are dropped. The server therefore
  reads the same tool name the policy checked, even if it matches keys without case.
- The agent identity is given by the operator (`-subject`) until `internal/identity`
  exists.
- Server-to-client messages pass through unchanged, except filtered `tools/list` results.

## Consequences

- Works with any stdio MCP server and any MCP client, with no change to either.
- One proxy process per MCP server, each with its own `-server` name.
- The evidence log is on the hot path: each call waits for a disk sync.
- A buggy or full disk blocks tool calls instead of letting them through unrecorded.
- The HTTP transport (Streamable HTTP) is not covered yet.
- The subject is only as trustworthy as the configuration that sets it.
