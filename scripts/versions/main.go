// Command versions keeps the "Versions in use" table of SECURITY.md in sync
// with the versions actually used in go.mod, the Dockerfile and the workflows.
//
// Run it from the repository root:
//
//	go run ./scripts/versions          # rewrite the table
//	go run ./scripts/versions -check   # fail if the table is out of date (CI)
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	securityFile = "SECURITY.md"
	goModFile    = "go.mod"
	dockerFile   = "deploy/docker/Dockerfile"
	ciFile       = ".github/workflows/ci.yml"
	secFile      = ".github/workflows/security.yml"
	releaseFile  = ".github/workflows/release.yml"

	startMarker = "<!-- versions:start -->"
	endMarker   = "<!-- versions:end -->"
)

// row is one line of the table.
type row struct {
	component string
	version   string
}

func main() {
	check := flag.Bool("check", false, "exit with an error if SECURITY.md is out of date")
	flag.Parse()

	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, "versions:", err)
		os.Exit(1)
	}
}

func run(check bool) error {
	rows, err := collect()
	if err != nil {
		return err
	}

	current, err := os.ReadFile(securityFile)
	if err != nil {
		return err
	}
	updated, err := replaceTable(current, render(rows))
	if err != nil {
		return err
	}

	if bytes.Equal(current, updated) {
		return nil
	}
	if check {
		return errors.New(securityFile + " is out of date: run `make versions` and commit the result")
	}
	return os.WriteFile(securityFile, updated, 0o600) // #nosec G703 -- constant path
}

// collect reads every version from its source file.
func collect() ([]row, error) {
	goMod, err := os.ReadFile(goModFile)
	if err != nil {
		return nil, err
	}
	docker, err := os.ReadFile(dockerFile)
	if err != nil {
		return nil, err
	}
	var workflows []byte
	for _, f := range []string{ciFile, secFile, releaseFile} {
		b, err := os.ReadFile(f) // #nosec G304 -- constant paths
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, b...)
	}

	var e extractor
	toolchain := e.find(goMod, "Go toolchain", `(?m)^toolchain go(\S+)$`)
	goMin := e.find(goMod, "Go minimum", `(?m)^go (\S+)$`)

	rows := []row{{"Go toolchain", fmt.Sprintf("%s (minimum `go %s`)", toolchain, goMin)}}
	for _, dep := range directDeps(goMod) {
		rows = append(rows, row{"`" + dep.component + "`", dep.version})
	}
	for _, img := range e.findAll(docker, "Docker images", `(?m)^FROM (?:--platform=\S+ )?([^@\s]+)@sha256:`) {
		rows = append(rows, row{"Image `" + img + "`", "pinned by digest"})
	}

	rows = append(rows,
		row{"golangci-lint", e.find(workflows, "golangci-lint", `golangci-lint-action@[^\n]*\n\s*with:\n\s*version:\s*(\S+)`)},
		row{"govulncheck", e.find(workflows, "govulncheck", `govulncheck@(v[0-9.]+)`)},
		row{"gitleaks", e.find(workflows, "gitleaks", `(?s)gitleaks.*?VERSION:\s*([0-9.]+)`)},
		row{"trivy", e.find(workflows, "trivy", `trivy-action@[^\n]*\n\s*with:\n\s*version:\s*(\S+)`)},
		row{"cosign", e.find(workflows, "cosign", `cosign-release:\s*(\S+)`)},
		row{"syft", e.find(workflows, "syft", `syft-version:\s*(\S+)`)},
	)

	if e.err != nil {
		return nil, e.err
	}
	return rows, nil
}

// extractor runs regexps and keeps the first error, so callers can check once.
type extractor struct {
	err error
}

func (e *extractor) find(src []byte, name, pattern string) string {
	m := regexp.MustCompile(pattern).FindSubmatch(src)
	if m == nil {
		e.fail(name)
		return ""
	}
	return string(m[1])
}

func (e *extractor) findAll(src []byte, name, pattern string) []string {
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllSubmatch(src, -1) {
		out = append(out, string(m[1]))
	}
	if len(out) == 0 {
		e.fail(name)
	}
	return out
}

func (e *extractor) fail(name string) {
	if e.err == nil {
		e.err = fmt.Errorf("cannot find the %s version: update scripts/versions", name)
	}
}

// directDeps returns the requirements of go.mod that are not marked indirect.
func directDeps(goMod []byte) []row {
	var deps []row
	inBlock := false
	for line := range strings.Lines(string(goMod)) {
		line = strings.TrimSpace(line)
		switch {
		case line == "require (":
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimPrefix(line, "require ")
		case !inBlock:
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		if f := strings.Fields(line); len(f) >= 2 {
			deps = append(deps, row{f[0], f[1]})
		}
	}
	return deps
}

func render(rows []row) string {
	var b strings.Builder
	b.WriteString("| Component | Version |\n|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s |\n", r.component, r.version)
	}
	return b.String()
}

// replaceTable swaps the text between the markers for table.
func replaceTable(doc []byte, table string) ([]byte, error) {
	s := string(doc)
	start := strings.Index(s, startMarker)
	end := strings.Index(s, endMarker)
	if start < 0 || end < start {
		return nil, fmt.Errorf("%s: markers %q and %q not found", securityFile, startMarker, endMarker)
	}
	return []byte(s[:start+len(startMarker)] + "\n" + table + s[end:]), nil
}
