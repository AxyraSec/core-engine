// Command mcp-demo-server is a fake GitHub MCP server, used by `make demo`,
// the end-to-end tests and the dev environment (deploy/dev). Do not use it
// for anything else.
//
//	mcp-demo-server               stdio
//	mcp-demo-server -http :3001   Streamable HTTP, endpoint /mcp
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/AxyraSec/core-engine/internal/protocol/mcp/mcptest"
)

func main() {
	addr := flag.String("http", "", "serve Streamable HTTP on this address instead of stdio")
	flag.Parse()

	if *addr == "" {
		if err := mcptest.Serve(os.Stdin, os.Stdout); err != nil {
			fail(err)
		}
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcptest.NewHTTPServer())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, "ok") })
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintln(os.Stderr, "mcp-demo-server: Streamable HTTP on", *addr+"/mcp")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mcp-demo-server:", err)
	os.Exit(1)
}
