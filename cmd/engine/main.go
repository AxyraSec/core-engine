// Command engine runs the Axyra core engine.
//
//	engine -policy <file>                 check a policy file
//	engine proxy [flags] -- <command...>  enforce a policy on an MCP server
//	engine verify -evidence <file>        check an evidence log
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"

	"github.com/AxyraSec/core-engine/internal/authz"
	"github.com/AxyraSec/core-engine/internal/evidence"
	"github.com/AxyraSec/core-engine/internal/protocol/mcp"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// stopDelay is how long the MCP server gets to exit after its input closes.
const stopDelay = 5 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "proxy":
			return runProxy(args[1:], stdin, stdout, stderr)
		case "verify":
			return runVerify(args[1:], stdout, stderr)
		}
	}
	return runCheck(args, stdout, stderr)
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("engine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policyPath := fs.String("policy", "", "check a YAML policy file and exit")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, `Usage:
  engine -policy <file>                 check a policy file
  engine proxy [flags] -- <command...>  enforce a policy on an MCP server (run "engine proxy -h")
  engine verify -evidence <file>        check an evidence log
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_, _ = fmt.Fprintln(stdout, "axyra core-engine", version)

	if *policyPath == "" {
		return 0
	}
	p, err := authz.LoadPolicyFile(*policyPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "policy error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "policy OK: %d rules\n", p.Len())
	return 0
}

// runProxy starts the MCP server command and proxies stdin/stdout to it.
// Stdout carries MCP messages only: everything else goes to stderr.
func runProxy(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("engine proxy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policyPath := fs.String("policy", "", "YAML policy file (required)")
	evidencePath := fs.String("evidence", "", "evidence log file, created if missing (required)")
	subject := fs.String("subject", "", `identity of the agent, for example "agent:claude-desktop" (required)`)
	server := fs.String("server", "", `name of the MCP server, used in resources "mcp:<server>/<tool>" (required)`)
	quiet := fs.Bool("quiet", false, "do not log decisions on stderr")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: engine proxy -policy <file> -evidence <file> -subject <agent> -server <name> -- <command> [args...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	command := fs.Args()
	if *policyPath == "" || *evidencePath == "" || *subject == "" || *server == "" || len(command) == 0 {
		fs.Usage()
		return 2
	}

	// The engine logs and the MCP server both write to stderr.
	stderr = &syncWriter{w: stderr}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	fail := func(msg string, err error) int {
		logger.Error(msg, "error", err)
		return 1
	}

	policy, err := authz.LoadPolicyFile(*policyPath)
	if err != nil {
		return fail("cannot load policy", err)
	}
	log, err := evidence.OpenFile(*evidencePath)
	if err != nil {
		return fail("cannot open evidence log", err)
	}
	defer func() { _ = log.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()

	// #nosec G204 -- the operator chooses which MCP server to run.
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stderr = stderr
	cmd.Cancel = func() error { return stopServer(cmd) }
	cmd.WaitDelay = stopDelay

	serverIn, err := cmd.StdinPipe()
	if err != nil {
		return fail("cannot start MCP server", err)
	}
	serverOut, err := cmd.StdoutPipe()
	if err != nil {
		return fail("cannot start MCP server", err)
	}
	if err := cmd.Start(); err != nil {
		return fail("cannot start MCP server", err)
	}

	p := &mcp.Proxy{Policy: policy, Evidence: log, Subject: *subject, Server: *server}
	if !*quiet {
		p.Log = logger
	}
	seq, head := log.Head()
	logger.Info("proxy started", "server", *server, "subject", *subject, "rules", policy.Len(), "evidence_seq", seq, "evidence_head", head.String())

	runErr := p.Run(stdin, stdout, serverIn, serverOut)
	waitErr := waitServer(cmd)

	seq, head = log.Head()
	logger.Info("proxy stopped", "evidence_seq", seq, "evidence_head", head.String())

	if runErr != nil {
		return fail("proxy error", runErr)
	}
	if waitErr != nil {
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) {
			return exit.ExitCode()
		}
		return fail("MCP server error", waitErr)
	}
	return 0
}

// waitServer waits for the MCP server to exit, and stops it if it does not.
func waitServer(cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(stopDelay):
		_ = cmd.Process.Kill()
		return <-done
	}
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("engine verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("evidence", "", "evidence log file to check (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		fs.Usage()
		return 2
	}

	n, head, err := evidence.VerifyFile(*path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "evidence error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "evidence OK: %d records, head %s\n", n, head)
	return 0
}

// syncWriter serializes writes from several goroutines.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
