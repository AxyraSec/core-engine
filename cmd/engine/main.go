// Command engine runs the Axyra core engine.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/AxyraSec/core-engine/internal/authz"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("engine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policyPath := fs.String("policy", "", "check a YAML policy file and exit")
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
