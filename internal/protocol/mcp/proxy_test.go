package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AxyraSec/core-engine/internal/authz"
	"github.com/AxyraSec/core-engine/internal/evidence"
	"github.com/AxyraSec/core-engine/internal/protocol/mcp/mcptest"
)

const testPolicy = `
version: 1
rules:
  - id: allow-billing-bot-github
    effect: allow
    subjects: ["agent:billing-bot"]
    actions: ["tools/call"]
    resources: ["mcp:github/*"]
  - id: deny-github-delete-repo
    effect: deny
    subjects: ["*"]
    actions: ["tools/call"]
    resources: ["mcp:github/delete_repo"]
`

// memRecorder keeps decisions in memory and can be made to fail.
type memRecorder struct {
	mu      sync.Mutex
	chain   *evidence.Chain
	failing bool
}

func (m *memRecorder) Append(e evidence.Entry) (evidence.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing {
		return evidence.Record{}, errors.New("disk full")
	}
	return m.chain.Append(e), nil
}

func (m *memRecorder) setFailing(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failing = v
}

func (m *memRecorder) records() []evidence.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chain.Records()
}

type harness struct {
	t      *testing.T
	client *io.PipeWriter
	out    *bufio.Scanner
	rec    *memRecorder
	done   chan error
}

func start(t *testing.T, subject string) *harness {
	t.Helper()
	policy, err := authz.LoadPolicy(strings.NewReader(testPolicy))
	if err != nil {
		t.Fatal(err)
	}

	clientR, clientW := io.Pipe()
	outR, outW := io.Pipe()
	srvInR, srvInW := io.Pipe()
	srvOutR, srvOutW := io.Pipe()

	go func() {
		_ = mcptest.Serve(srvInR, srvOutW)
		_ = srvOutW.Close()
	}()

	rec := &memRecorder{chain: evidence.NewChain()}
	p := &Proxy{Policy: policy, Evidence: rec, Subject: subject, Server: "github"}

	h := &harness{t: t, client: clientW, out: bufio.NewScanner(outR), rec: rec, done: make(chan error, 1)}
	go func() {
		err := p.Run(clientR, outW, srvInW, srvOutR)
		_ = outW.Close()
		h.done <- err
	}()
	t.Cleanup(func() { _ = clientW.Close() })
	return h
}

func (h *harness) send(line string) {
	h.t.Helper()
	if _, err := io.WriteString(h.client, line+"\n"); err != nil {
		h.t.Fatalf("send: %v", err)
	}
}

func (h *harness) recv() map[string]any {
	h.t.Helper()
	if !h.out.Scan() {
		h.t.Fatalf("no message from proxy: %v", h.out.Err())
	}
	var m map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &m); err != nil {
		h.t.Fatalf("bad JSON from proxy %q: %v", h.out.Bytes(), err)
	}
	return m
}

func (h *harness) call(id int, body string) map[string]any {
	h.t.Helper()
	h.send(`{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":` + body + `}`)
	return h.recv()
}

func (h *harness) stop() error {
	_ = h.client.Close()
	select {
	case err := <-h.done:
		return err
	case <-time.After(5 * time.Second):
		h.t.Fatal("proxy did not stop")
		return nil
	}
}

func itoa(i int) string { return strings.TrimSpace(string(must(json.Marshal(i)))) }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func ranTool(m map[string]any) string {
	res, _ := m["result"].(map[string]any)
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return strings.TrimPrefix(text, "ran ")
}

func denial(m map[string]any) (code float64, data map[string]any) {
	e, ok := m["error"].(map[string]any)
	if !ok {
		return 0, nil
	}
	code, _ = e["code"].(float64)
	data, _ = e["data"].(map[string]any)
	return code, data
}

func toolNames(m map[string]any) []string {
	res, _ := m["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	var names []string
	for _, t := range tools {
		tool, _ := t.(map[string]any)
		name, _ := tool["name"].(string)
		names = append(names, name)
	}
	return names
}

func TestAllowedCallReachesServer(t *testing.T) {
	h := start(t, "agent:billing-bot")

	resp := h.call(1, `{"name":"create_issue","arguments":{"title":"bug"}}`)
	if got := ranTool(resp); got != "create_issue" {
		t.Fatalf("server ran %q, want create_issue (response %v)", got, resp)
	}

	recs := h.rec.records()
	if len(recs) != 1 || !recs[0].Allowed || recs[0].Resource != "mcp:github/create_issue" || recs[0].RuleID != "allow-billing-bot-github" {
		t.Fatalf("evidence = %+v", recs)
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
}

func TestDeniedCallNeverReachesServer(t *testing.T) {
	h := start(t, "agent:billing-bot")

	resp := h.call(2, `{"name":"delete_repo"}`)
	code, data := denial(resp)
	if code != CodeDenied || data["rule_id"] != "deny-github-delete-repo" || data["reason"] != "denied_by_rule" {
		t.Fatalf("response = %v, want a policy denial", resp)
	}
	if resp["id"] != float64(2) {
		t.Errorf("denial id = %v, want 2", resp["id"])
	}

	// The next call is answered by the server, which proves the denied one
	// was not queued there.
	if got := ranTool(h.call(3, `{"name":"list_repos"}`)); got != "list_repos" {
		t.Fatalf("server ran %q after a denial, want list_repos", got)
	}

	recs := h.rec.records()
	if len(recs) != 2 || recs[0].Allowed || recs[0].Reason != "denied_by_rule" {
		t.Fatalf("evidence = %+v", recs)
	}
}

func TestUnknownAgentIsDeniedEverything(t *testing.T) {
	h := start(t, "agent:intern")

	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if names := toolNames(h.recv()); len(names) != 0 {
		t.Fatalf("tools/list = %v, want none", names)
	}

	_, data := denial(h.call(2, `{"name":"create_issue"}`))
	if data["reason"] != "no_matching_rule" {
		t.Fatalf("denial data = %v, want no_matching_rule", data)
	}
}

func TestToolsListHidesDeniedTools(t *testing.T) {
	h := start(t, "agent:billing-bot")

	h.send(`{"jsonrpc":"2.0","id":"list-1","method":"tools/list"}`)
	names := toolNames(h.recv())
	if strings.Join(names, ",") != "create_issue,list_repos" {
		t.Fatalf("tools/list = %v, want create_issue,list_repos", names)
	}
	if len(h.rec.records()) != 0 {
		t.Error("listing tools must not write evidence")
	}
}

func TestOtherMessagesPassThrough(t *testing.T) {
	h := start(t, "agent:billing-bot")

	h.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	resp := h.recv()
	res, _ := resp["result"].(map[string]any)
	if res["protocolVersion"] == nil {
		t.Fatalf("initialize = %v", resp)
	}

	h.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	h.send(``) // blank lines are ignored
	h.send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if resp := h.recv(); resp["id"] != float64(2) {
		t.Fatalf("ping = %v", resp)
	}
}

func TestFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantCode float64
	}{
		{"not json", `{"jsonrpc":`, codeParseError},
		{"not an object", `"tools/call"`, codeParseError},
		{"null", `null`, codeParseError},
		{"batch", `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_repo"}}]`, codeInvalidRequest},
		{"no tool name", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`, CodeDenied},
		{"no params", `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`, CodeDenied},
		{"name not a string", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":42}}`, CodeDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := start(t, "agent:billing-bot")
			h.send(tt.line)
			if code, _ := denial(h.recv()); code != tt.wantCode {
				t.Fatalf("error code = %v, want %v", code, tt.wantCode)
			}
		})
	}
}

func TestEvidenceFailureDeniesTheCall(t *testing.T) {
	h := start(t, "agent:billing-bot")
	h.rec.setFailing(true)

	code, data := denial(h.call(1, `{"name":"create_issue"}`))
	if code != CodeDenied || data["reason"] != ReasonEvidenceUnavailable {
		t.Fatalf("got code %v data %v, want denial with %s", code, data, ReasonEvidenceUnavailable)
	}
}

// The proxy must forward exactly what it evaluated, whatever tricks the
// client plays with the JSON.
func TestNoParserDifferential(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		wantRan  string // empty = must be denied
		resource string
	}{
		{"duplicate name, allowed last", `{"name":"delete_repo","name":"create_issue"}`, "create_issue", "mcp:github/create_issue"},
		{"duplicate name, denied last", `{"name":"create_issue","name":"delete_repo"}`, "", "mcp:github/delete_repo"},
		{"case variant of name", `{"Name":"delete_repo","name":"create_issue"}`, "create_issue", "mcp:github/create_issue"},
		{"only a case variant", `{"NAME":"delete_repo"}`, "", ""},
		{"escaped characters", `{"name":"delete_repo"}`, "", "mcp:github/delete_repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := start(t, "agent:billing-bot")
			resp := h.call(1, tt.params)

			ran := ranTool(resp)
			if ran != tt.wantRan {
				t.Fatalf("server ran %q, want %q (response %v)", ran, tt.wantRan, resp)
			}
			recs := h.rec.records()
			if len(recs) != 1 || recs[0].Resource != tt.resource {
				t.Fatalf("evidence = %+v, want resource %q", recs, tt.resource)
			}
		})
	}
}

func TestCaseVariantOfMethodIsDropped(t *testing.T) {
	h := start(t, "agent:billing-bot")
	// Without the fix, a case-insensitive server would see "tools/call".
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping","Method":"tools/call","params":{"name":"delete_repo"}}`)
	resp := h.recv()
	if _, isErr := resp["error"]; isErr || ranTool(resp) != "" {
		t.Fatalf("response = %v, want a plain ping result", resp)
	}
}

func TestDeniedNotificationGetsNoReply(t *testing.T) {
	h := start(t, "agent:billing-bot")
	h.send(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"delete_repo"}}`)
	h.send(`{"jsonrpc":"2.0","id":9,"method":"ping"}`)

	if resp := h.recv(); resp["id"] != float64(9) {
		t.Fatalf("first reply = %v, want the ping", resp)
	}
	if recs := h.rec.records(); len(recs) != 1 || recs[0].Allowed {
		t.Fatalf("evidence = %+v", recs)
	}
}

func TestRunValidatesConfig(t *testing.T) {
	policy := must(authz.NewPolicy(nil))
	rec := &memRecorder{chain: evidence.NewChain()}
	for _, p := range []*Proxy{
		{Evidence: rec, Server: "github"},
		{Policy: policy, Server: "github"},
		{Policy: policy, Evidence: rec},
		{Policy: policy, Evidence: rec, Server: "a/b"},
	} {
		if err := p.Run(strings.NewReader(""), io.Discard, nopCloser{io.Discard}, strings.NewReader("")); err == nil {
			t.Errorf("Run(%+v) accepted an invalid config", p)
		}
	}
}

func TestRunEndsWhenServerExits(t *testing.T) {
	policy := must(authz.NewPolicy(nil))
	p := &Proxy{Policy: policy, Evidence: &memRecorder{chain: evidence.NewChain()}, Server: "github"}

	clientR, clientW := io.Pipe()
	defer func() { _ = clientW.Close() }()

	done := make(chan error, 1)
	go func() { done <- p.Run(clientR, io.Discard, nopCloser{io.Discard}, strings.NewReader("")) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the server closed its output")
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// FuzzNeverForwardsADeniedCall checks that whatever the client sends, a
// tools/call reaching the server always names an allowed tool, whether the
// server reads keys exactly (a map) or without case (a Go struct).
func FuzzNeverForwardsADeniedCall(f *testing.F) {
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_issue"}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_repo","name":"create_issue"}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"ping","Method":"tools/call","params":{"Name":"delete_repo"}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"METHOD":"tools/call","PARAMS":{"NAME":"delete_repo"}}`)

	policy := must(authz.LoadPolicy(strings.NewReader(`
version: 1
rules:
  - id: only-create
    effect: allow
    subjects: ["agent:a"]
    actions: ["tools/call"]
    resources: ["mcp:github/create_issue"]
`)))

	f.Fuzz(func(t *testing.T, line string) {
		p := &Proxy{Policy: policy, Evidence: &memRecorder{chain: evidence.NewChain()}, Subject: "agent:a", Server: "github"}
		p.out = &lockedWriter{w: io.Discard}

		forward, err := p.fromClient([]byte(line))
		if err != nil || forward == nil {
			return
		}

		var exact map[string]json.RawMessage
		if json.Unmarshal(forward, &exact) == nil && string(exact["method"]) == `"tools/call"` {
			var params map[string]json.RawMessage
			_ = json.Unmarshal(exact["params"], &params)
			if string(params["name"]) != `"create_issue"` {
				t.Fatalf("forwarded a denied call (exact keys): %s", forward)
			}
		}

		var loose struct {
			Method string
			Params struct{ Name string }
		}
		if json.Unmarshal(forward, &loose) == nil && loose.Method == "tools/call" && loose.Params.Name != "create_issue" {
			t.Fatalf("forwarded a denied call (case-insensitive keys): %s", forward)
		}
	})
}
