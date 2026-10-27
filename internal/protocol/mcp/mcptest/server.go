// Package mcptest is a tiny MCP server over stdio or Streamable HTTP, for
// tests and demos.
//
// It pretends to be a GitHub server with three tools. Every tool call
// succeeds and says which tool ran, so a test can tell what reached it.
package mcptest

import (
	"bufio"
	"encoding/json"
	"io"
)

// Tools are the tool names the server offers.
var Tools = []string{"create_issue", "delete_repo", "list_repos"}

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// Serve answers MCP requests read from r on w, until r is closed.
func Serve(r io.Reader, w io.Writer) error {
	enc := json.NewEncoder(w)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)

	for sc.Scan() {
		resp := Answer(sc.Bytes())
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Answer returns the response to one JSON-RPC message, or nil when the
// message is a notification, a response or not valid JSON.
func Answer(msg []byte) map[string]any {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil
	}
	if req.ID == nil || req.Method == "" {
		return nil // notification or response
	}

	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	switch req.Method {
	case "initialize":
		resp["result"] = map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "demo-github", "version": "0.1.0"},
		}
	case "tools/list":
		tools := make([]map[string]any, 0, len(Tools))
		for _, name := range Tools {
			tools = append(tools, map[string]any{
				"name":        name,
				"description": "Demo tool " + name,
				"inputSchema": map[string]any{"type": "object"},
			})
		}
		resp["result"] = map[string]any{"tools": tools}
	case "tools/call":
		resp["result"] = map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ran " + req.Params.Name}},
		}
	case "ping":
		resp["result"] = map[string]any{}
	default:
		resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
	}
	return resp
}
