//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// stopSignals are the signals that stop the proxy.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// stopServer asks the MCP server to exit cleanly.
func stopServer(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGTERM)
}
