// Command mcp-demo-server is a fake GitHub MCP server on stdio, used by
// `make demo` and the end-to-end tests. Do not use it for anything else.
package main

import (
	"fmt"
	"os"

	"github.com/AxyraSec/core-engine/internal/protocol/mcp/mcptest"
)

func main() {
	if err := mcptest.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-demo-server:", err)
		os.Exit(1)
	}
}
