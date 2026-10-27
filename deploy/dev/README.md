# Dev environment: MCP over Streamable HTTP

Two MCP servers on the Streamable HTTP transport, to try the proxy by hand.
Ports are bound to `127.0.0.1` only.

| Service | URL | What it is |
|---|---|---|
| `github` | `http://localhost:3001/mcp` | Fake GitHub server from this repo (`test/mcp-demo-server -http`). JSON responses only, checks `Mcp-Session-Id`. |
| `everything` | `http://localhost:3002/mcp` | Reference server from the MCP project. Answers with SSE streams, sends notifications. |

```sh
make dev-up     # start (first run downloads Go modules and npm packages)
make dev-down   # stop
docker compose -f deploy/dev/compose.yaml restart github   # after changing mcptest
```

## Call the servers with curl

```sh
curl -si localhost:3001/mcp -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
# then send Mcp-Session-Id from the response on every later request
```

## Use Claude Code as the agent

Register the servers for yourself only (local scope, not committed):

```sh
claude mcp add --transport http dev-github http://localhost:3001/mcp
claude mcp add --transport http dev-everything http://localhost:3002/mcp
```

Once `engine` can proxy HTTP, point Claude Code at the engine instead of the
server and ask it to call `delete_repo`: the call must be denied and recorded.
