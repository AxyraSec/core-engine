package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AxyraSec/core-engine/internal/protocol/mcp/mcptest"
)

// When this variable is set, the test binary acts as the fake MCP server, so
// the end-to-end tests can start a real subprocess without building anything.
const fakeServerEnv = "ENGINE_TEST_FAKE_MCP_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeServerEnv) == "1" {
		if err := mcptest.Serve(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no args", nil, 0, "axyra core-engine dev", ""},
		{"valid policy", []string{"-policy", "../../policies/example.yaml"}, 0, "policy OK: 2 rules", ""},
		{"missing policy", []string{"-policy", "does-not-exist.yaml"}, 1, "", "policy error:"},
		{"unknown flag", []string{"-nope"}, 2, "", "Usage:"},
		{"proxy without flags", []string{"proxy"}, 2, "", "Usage: engine proxy"},
		{"proxy without command", []string{"proxy", "-policy", "p", "-evidence", "e", "-subject", "s", "-server", "x"}, 2, "", "Usage: engine proxy"},
		{"proxy bad policy", []string{"proxy", "-policy", "nope.yaml", "-evidence", "e", "-subject", "s", "-server", "x", "--", "true"}, 1, "", "cannot load policy"},
		{"verify without flags", []string{"verify"}, 2, "", "-evidence"},
		{"verify missing file", []string{"verify", "-evidence", "nope.jsonl"}, 1, "", "evidence error:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// TestProxyEndToEnd runs the real proxy against a real subprocess: the agent
// lists tools, makes one allowed and one denied call, and the evidence log
// written to disk must verify.
func TestProxyEndToEnd(t *testing.T) {
	t.Setenv(fakeServerEnv, "1")
	evidencePath := filepath.Join(t.TempDir(), "evidence.jsonl")

	session := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_issue"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"delete_repo"}}`,
	}, "\n") + "\n"

	proxyArgs := []string{
		"proxy", "-quiet",
		"-policy", "../../policies/example.yaml",
		"-evidence", evidencePath,
		"-subject", "agent:billing-bot",
		"-server", "github",
		"--", os.Args[0],
	}

	var stdout, stderr bytes.Buffer
	if code := run(proxyArgs, strings.NewReader(session), &stdout, &stderr); code != 0 {
		t.Fatalf("proxy exit code = %d, stderr:\n%s", code, stderr.String())
	}

	byID := map[float64]map[string]any{}
	sc := bufio.NewScanner(&stdout)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("stdout must carry only MCP messages, got %q", sc.Text())
		}
		id, _ := m["id"].(float64)
		byID[id] = m
	}
	if len(byID) != 4 {
		t.Fatalf("got %d responses, want 4: %v", len(byID), byID)
	}

	list, _ := byID[2]["result"].(map[string]any)
	if tools, _ := list["tools"].([]any); len(tools) != 2 {
		t.Errorf("tools/list returned %d tools, want 2 (delete_repo hidden)", len(tools))
	}
	if _, ok := byID[3]["result"]; !ok {
		t.Errorf("create_issue: %v, want a result", byID[3])
	}
	if errObj, ok := byID[4]["error"].(map[string]any); !ok || errObj["code"] != float64(-32001) {
		t.Errorf("delete_repo: %v, want a policy denial", byID[4])
	}

	// The evidence log on disk verifies and holds both decisions.
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"verify", "-evidence", evidencePath}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("verify exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "evidence OK: 2 records") {
		t.Fatalf("verify output = %q", stdout.String())
	}

	// Rewriting history is detected.
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"allowed":false`, `"allowed":true`, 1)
	if err := os.WriteFile(evidencePath, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := run([]string{"verify", "-evidence", evidencePath}, nil, &stdout, &stderr); code != 1 {
		t.Fatalf("verify on a tampered log: exit code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "record #1: content does not match its hash") {
		t.Fatalf("verify error = %q", stderr.String())
	}

	// And the proxy refuses to start on it.
	stderr.Reset()
	if code := run(proxyArgs, strings.NewReader(""), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "cannot open evidence log") {
		t.Fatalf("proxy on a tampered log: exit %d, stderr %q", code, stderr.String())
	}
}

func TestProxyReportsServerExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"proxy", "-quiet",
		"-policy", "../../policies/example.yaml",
		"-evidence", filepath.Join(t.TempDir(), "e.jsonl"),
		"-subject", "agent:x", "-server", "github",
		"--", "sh", "-c", "exit 3",
	}
	if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 3 {
		t.Fatalf("exit code = %d, want the server's 3 (stderr: %s)", code, stderr.String())
	}
}

func TestProxyCommandNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"proxy",
		"-policy", "../../policies/example.yaml",
		"-evidence", filepath.Join(t.TempDir(), "e.jsonl"),
		"-subject", "agent:x", "-server", "github",
		"--", "/does/not/exist",
	}
	if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "cannot start MCP server") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}
