//go:build windows

package main

import (
	"os"
	"os/exec"
)

// stopSignals are the signals that stop the proxy. Windows only delivers Ctrl+C.
var stopSignals = []os.Signal{os.Interrupt}

// stopServer stops the MCP server. Windows has no SIGTERM, so the process is killed.
func stopServer(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
