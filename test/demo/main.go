// Command demo shows core-engine at work: an agent talks to a fake GitHub MCP
// server through `engine proxy`. It runs `make demo` and the demo image:
//
//	docker run --rm ghcr.io/axyrasec/core-engine-demo
//
// It checks every result it prints and exits with an error if one is wrong,
// so CI runs it as an end-to-end test. Set NO_COLOR to disable colors.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	bold, dim, green, red, cyan, yellow, reset = "\033[1m", "\033[2m", "\033[32m", "\033[31m", "\033[36m", "\033[33m", "\033[0m"
)

type config struct {
	engine, server, policy, evidence string
}

func main() {
	var c config
	flag.StringVar(&c.engine, "engine", "/engine", "path to the engine binary")
	flag.StringVar(&c.server, "server", "/mcp-demo-server", "path to the fake MCP server")
	flag.StringVar(&c.policy, "policy", "/policies/example.yaml", "policy file")
	flag.Parse()

	if os.Getenv("NO_COLOR") != "" {
		bold, dim, green, red, cyan, yellow, reset = "", "", "", "", "", "", ""
	}

	dir, err := os.MkdirTemp("", "axyra-demo")
	if err != nil {
		fail(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	c.evidence = filepath.Join(dir, "evidence.jsonl")

	if err := run(c); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "\n%sdemo failed: %v%s\n", red, err, reset)
	os.Exit(1)
}

func step(title string) { fmt.Printf("\n%s%s━━ %s%s\n\n", bold, cyan, title, reset) }

func run(c config) error {
	step("1. The policy")
	policy, err := os.ReadFile(filepath.Clean(c.policy))
	if err != nil {
		return err
	}
	for line := range strings.Lines(string(policy)) {
		if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "#") {
			fmt.Print("  ", line)
		}
	}

	step("2. agent:billing-bot connects to the GitHub MCP server")
	fmt.Println("  It lists the tools, then calls create_issue and delete_repo.")
	fmt.Printf("  %sdelete_repo is hidden from its tool list, and blocked when it tries anyway.%s\n\n", dim, reset)
	s, err := session(c, "agent:billing-bot", list(2), call(3, "create_issue"), call(4, "delete_repo"))
	if err != nil {
		return err
	}
	s.print()
	if err := s.expect(map[int]string{2: "tools: create_issue, list_repos", 3: "ran create_issue", 4: "denied"}); err != nil {
		return err
	}

	step("3. agent:intern, an agent the policy does not know")
	fmt.Printf("  %sDeny by default: no rule matches, so nothing is allowed.%s\n\n", dim, reset)
	s, err = session(c, "agent:intern", list(2), call(3, "list_repos"))
	if err != nil {
		return err
	}
	s.print()
	if err := s.expect(map[int]string{2: "tools: (none)", 3: "denied"}); err != nil {
		return err
	}

	step("4. Every decision is in a hash-chained evidence log")
	log, err := os.ReadFile(c.evidence)
	if err != nil {
		return err
	}
	for line := range strings.Lines(string(log)) {
		fmt.Printf("  %s%s%s\n", dim, cut(strings.TrimSpace(line), 110), reset)
	}
	out, err := engine(c, "verify", "-evidence", c.evidence)
	if err != nil {
		return fmt.Errorf("verify on an untouched log: %w: %s", err, out)
	}
	fmt.Printf("\n  %s%s%s\n", green, strings.TrimSpace(out), reset)

	step("5. Someone edits the log to hide the denied delete_repo")
	const from, to = `"allowed":false,"rule_id":"deny-github-delete-repo"`, `"allowed":true,"rule_id":"deny-github-delete-repo"`
	if !bytes.Contains(log, []byte(from)) {
		return errors.New("the denied delete_repo is not in the log")
	}
	// #nosec G703 -- the demo's own log, in the temp directory it created.
	if err := os.WriteFile(c.evidence, bytes.Replace(log, []byte(from), []byte(to), 1), 0o600); err != nil {
		return err
	}
	fmt.Printf("  %s\"allowed\":false  →  \"allowed\":true%s\n\n", yellow, reset)

	out, err = engine(c, "verify", "-evidence", c.evidence)
	if err == nil || !strings.Contains(out, "content does not match its hash") {
		return fmt.Errorf("tampering was not detected: %s", out)
	}
	fmt.Printf("  %s%s%s\n", red, strings.TrimSpace(strings.ReplaceAll(out, c.evidence, "evidence.jsonl")), reset)

	out, err = engine(c, "proxy", "-policy", c.policy, "-evidence", c.evidence, "-subject", "agent:billing-bot", "-server", "github", "--", c.server)
	if err == nil || !strings.Contains(out, "cannot open evidence log") {
		return fmt.Errorf("the proxy started on a broken log: %s", out)
	}
	fmt.Printf("\n  %sThe engine also refuses to start on a broken log.%s\n", dim, reset)

	fmt.Printf("\n%s%s━━ Done.%s Every result above was checked. Source: https://github.com/AxyraSec/core-engine\n\n", bold, cyan, reset)
	return nil
}

// engine runs the engine binary and returns its combined output.
func engine(c config, args ...string) (string, error) {
	// #nosec G204 -- the demo runs its own engine binary.
	out, err := exec.Command(c.engine, args...).CombinedOutput()
	return string(out), err
}

func list(id int) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list"}`, id)
}

func call(id int, tool string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q}}`, id, tool)
}

type result struct {
	decisions []string       // "ALLOW mcp:github/create_issue (allowed_by_rule)"
	replies   map[int]string // id -> one-line summary
}

var decisionRe = regexp.MustCompile(`msg=decision allowed=(\w+) .*resource=(\S+) .*reason=(\S+)`)

// session runs one agent session through the proxy and collects what the
// agent received and what the engine decided.
func session(c config, subject string, requests ...string) (*result, error) {
	// #nosec G204 -- the demo runs its own engine binary.
	cmd := exec.Command(c.engine, "proxy", "-policy", c.policy, "-evidence", c.evidence,
		"-subject", subject, "-server", "github", "--", c.server)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	for _, r := range append([]string{init}, requests...) {
		if _, err := io.WriteString(stdin, r+"\n"); err != nil {
			return nil, err
		}
	}

	res := &result{replies: map[int]string{}}
	want := len(requests) + 1
	done := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for len(res.replies) < want && sc.Scan() {
			id, summary := summarize(sc.Bytes())
			res.replies[id] = summary
		}
		done <- sc.Err()
	}()
	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("no answer from the proxy for %s: %s", subject, stderr.String())
	}

	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("proxy: %w: %s", err, stderr.String())
	}

	for _, m := range decisionRe.FindAllStringSubmatch(stderr.String(), -1) {
		verdict := "DENY "
		if m[1] == "true" {
			verdict = "ALLOW"
		}
		res.decisions = append(res.decisions, fmt.Sprintf("%s %s (%s)", verdict, m[2], m[3]))
	}
	return res, nil
}

// summarize turns a JSON-RPC response into its id and a one-line summary.
func summarize(line []byte) (int, string) {
	var m struct {
		ID     int `json:"id"`
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			ServerInfo *struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &m); err != nil {
		return 0, "unreadable: " + string(line)
	}
	switch {
	case m.Error != nil:
		return m.ID, fmt.Sprintf("denied: error %d %s", m.Error.Code, m.Error.Message)
	case m.Result.ServerInfo != nil:
		return m.ID, "connected to " + m.Result.ServerInfo.Name
	case len(m.Result.Content) > 0:
		return m.ID, m.Result.Content[0].Text
	default:
		names := make([]string, 0, len(m.Result.Tools))
		for _, t := range m.Result.Tools {
			names = append(names, t.Name)
		}
		if len(names) == 0 {
			return m.ID, "tools: (none)"
		}
		return m.ID, "tools: " + strings.Join(names, ", ")
	}
}

func (r *result) print() {
	for _, d := range r.decisions {
		color := red
		if strings.HasPrefix(d, "ALLOW") {
			color = green
		}
		verdict, rest, _ := strings.Cut(d, " ")
		fmt.Printf("  %s%-7s%s  %s\n", color+bold, verdictLabel(verdict), reset, rest)
	}
	fmt.Printf("\n  %swhat the agent received:%s\n", dim, reset)
	ids := make([]int, 0, len(r.replies))
	for id := range r.replies {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		reply := r.replies[id]
		color := ""
		switch {
		case strings.HasPrefix(reply, "denied"):
			color = red
		case strings.HasPrefix(reply, "ran "):
			color = green
		}
		fmt.Printf("  %s←%s #%d %s%s%s\n", dim, reset, id, color, reply, reset)
	}
}

func verdictLabel(v string) string {
	if v == "ALLOW" {
		return "✔ ALLOW"
	}
	return "✘ DENY"
}

// expect checks that each reply starts with the wanted text.
func (r *result) expect(want map[int]string) error {
	for id, prefix := range want {
		if !strings.HasPrefix(r.replies[id], prefix) {
			return fmt.Errorf("reply #%d = %q, want %q", id, r.replies[id], prefix)
		}
	}
	return nil
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
