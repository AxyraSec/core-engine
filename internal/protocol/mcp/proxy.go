// Package mcp enforces authorization on the Model Context Protocol (MCP).
//
// Proxy sits between an MCP client (the agent) and an MCP server, on the stdio
// transport: newline-delimited JSON-RPC 2.0 messages. For every tools/call it:
//
//  1. asks internal/authz for a decision,
//  2. records the decision in the evidence log,
//  3. forwards the call to the server only if it is allowed and recorded.
//
// It fails closed: a message it cannot parse, a JSON-RPC batch, or a decision
// it cannot record is never forwarded. It also removes the tools the agent may
// not call from tools/list responses, so the agent does not try them.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/AxyraSec/core-engine/internal/authz"
	"github.com/AxyraSec/core-engine/internal/evidence"
)

// MaxMessageSize is the largest JSON-RPC message the proxy accepts, in bytes.
const MaxMessageSize = 16 << 20 // 16 MiB

// ActionToolsCall is the authz action used for MCP tool calls.
const ActionToolsCall = "tools/call"

// ReasonEvidenceUnavailable is used when a decision could not be recorded.
const ReasonEvidenceUnavailable = "evidence_unavailable"

var errNotObject = errors.New("not a JSON object")

// Recorder stores decisions. *evidence.FileLog implements it.
type Recorder interface {
	Append(evidence.Entry) (evidence.Record, error)
}

// Proxy enforces a policy on the MCP traffic between one client and one server.
type Proxy struct {
	// Policy decides which tool calls are allowed. Required.
	Policy *authz.Policy
	// Evidence records every tools/call decision. Required.
	Evidence Recorder
	// Subject is the identity of the agent, for example "agent:claude-desktop".
	Subject string
	// Server names the MCP server in resources: tool "create_issue" on server
	// "github" is the resource "mcp:github/create_issue".
	Server string
	// Log receives one line per decision. Optional.
	Log *slog.Logger

	pendingLists sync.Map // idKey -> struct{}: tools/list requests in flight
	out          *lockedWriter
}

// Resource returns the authz resource for a tool of this proxy's server.
func (p *Proxy) Resource(tool string) string {
	if tool == "" {
		return ""
	}
	return "mcp:" + p.Server + "/" + tool
}

// Run proxies messages until the client closes its input or the server
// closes its output. clientIn/clientOut talk to the agent, serverIn/serverOut
// to the MCP server. serverIn is closed when the client is done.
func (p *Proxy) Run(clientIn io.Reader, clientOut io.Writer, serverIn io.WriteCloser, serverOut io.Reader) error {
	if p.Policy == nil || p.Evidence == nil {
		return errors.New("mcp: proxy needs a policy and an evidence recorder")
	}
	if p.Server == "" || strings.Contains(p.Server, "/") {
		return fmt.Errorf("mcp: invalid server name %q", p.Server)
	}
	p.out = &lockedWriter{w: clientOut}

	fromServer := make(chan error, 1)
	go func() { fromServer <- p.serverLoop(serverOut) }()

	fromClient := make(chan error, 1)
	go func() { fromClient <- p.clientLoop(clientIn, serverIn) }()

	select {
	case err := <-fromServer:
		// The server is gone: nothing more can be served.
		return err
	case err := <-fromClient:
		// The client is done: let the server finish and flush its replies.
		closeErr := serverIn.Close()
		return errors.Join(err, closeErr, <-fromServer)
	}
}

func (p *Proxy) clientLoop(in io.Reader, server io.Writer) error {
	return readLines(in, func(line []byte) error {
		forward, err := p.fromClient(line)
		if err != nil || forward == nil {
			return err
		}
		_, err = server.Write(append(forward, '\n'))
		return err
	})
}

func (p *Proxy) serverLoop(in io.Reader) error {
	return readLines(in, func(line []byte) error {
		return p.out.writeLine(p.fromServer(line))
	})
}

// fromClient returns the bytes to forward to the server, or nil when the
// message is answered (or dropped) by the proxy itself.
func (p *Proxy) fromClient(line []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		return nil, p.reply(newError(nil, codeInvalidRequest, "JSON-RPC batches are not supported", nil))
	}

	msg, err := parseObject(trimmed)
	if err != nil {
		return nil, p.reply(newError(nil, codeParseError, "parse error", nil))
	}
	msg.dropCaseVariants("jsonrpc", "id", "method", "params")

	method, _ := msg.str("method")
	switch method {
	case "tools/call":
		return p.toolsCall(msg)
	case "tools/list":
		if id := msg.id(); id != nil {
			p.pendingLists.Store(idKey(id), struct{}{})
		}
	}

	// Forward the message as we parsed it, not as received, so the server
	// cannot read it differently (duplicate keys, key case).
	return json.Marshal(msg)
}

func (p *Proxy) toolsCall(msg object) ([]byte, error) {
	params, err := parseObject(msg["params"])
	if err != nil {
		params = object{}
	}
	params.dropCaseVariants("name", "arguments")
	tool, _ := params.str("name")

	req := authz.Request{Subject: p.Subject, Action: ActionToolsCall, Resource: p.Resource(tool)}
	d := p.Policy.Evaluate(req)
	reason := string(d.Reason)

	_, recErr := p.Evidence.Append(evidence.Entry{
		Subject:  req.Subject,
		Action:   req.Action,
		Resource: req.Resource,
		Allowed:  d.Allowed,
		RuleID:   d.RuleID,
		Reason:   reason,
	})
	allowed := d.Allowed
	if recErr != nil {
		// No evidence, no call.
		allowed = false
		reason = ReasonEvidenceUnavailable
		p.logError("cannot record decision", recErr)
	}
	p.logDecision(req, allowed, d.RuleID, reason)

	if !allowed {
		id := msg.id()
		if id == nil {
			return nil, nil // notification: nobody waits for an answer
		}
		data := map[string]string{"resource": req.Resource, "rule_id": d.RuleID, "reason": reason}
		return nil, p.reply(newError(id, CodeDenied, "denied by Axyra policy", data))
	}

	// Forward exactly what was evaluated.
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	msg["params"] = rawParams
	return json.Marshal(msg)
}

// fromServer returns the line to send to the client. Responses to tools/list
// are filtered; everything else passes unchanged.
func (p *Proxy) fromServer(line []byte) []byte {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return line
	}
	msg, err := parseObject(trimmed)
	if err != nil {
		return line
	}
	id := msg.id()
	if id == nil {
		return line
	}
	if _, ok := p.pendingLists.LoadAndDelete(idKey(id)); !ok {
		return line
	}

	filtered, err := p.filterTools(msg)
	if err != nil {
		// The list is a convenience; tools/call is where access is enforced.
		return line
	}
	return filtered
}

// filterTools removes from a tools/list result the tools the subject may not call.
func (p *Proxy) filterTools(msg object) ([]byte, error) {
	result, err := parseObject(msg["result"])
	if err != nil {
		return nil, err
	}
	var tools []json.RawMessage
	if err := json.Unmarshal(result["tools"], &tools); err != nil {
		return nil, err
	}

	kept := make([]json.RawMessage, 0, len(tools))
	for _, raw := range tools {
		tool, err := parseObject(raw)
		if err != nil {
			continue
		}
		name, _ := tool.str("name")
		d := p.Policy.Evaluate(authz.Request{Subject: p.Subject, Action: ActionToolsCall, Resource: p.Resource(name)})
		if d.Allowed {
			kept = append(kept, raw)
		}
	}

	if result["tools"], err = json.Marshal(kept); err != nil {
		return nil, err
	}
	if msg["result"], err = json.Marshal(result); err != nil {
		return nil, err
	}
	return json.Marshal(msg)
}

func (p *Proxy) reply(v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.out.writeLine(line)
}

func (p *Proxy) logDecision(req authz.Request, allowed bool, ruleID, reason string) {
	if p.Log == nil {
		return
	}
	p.Log.Info("decision",
		"allowed", allowed, "subject", req.Subject, "resource", req.Resource,
		"rule_id", ruleID, "reason", reason)
}

func (p *Proxy) logError(msg string, err error) {
	if p.Log != nil {
		p.Log.Error(msg, "error", err)
	}
}

// readLines calls fn for each line of r, without the trailing newline.
func readLines(r io.Reader, fn func([]byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxMessageSize)
	for sc.Scan() {
		if err := fn(sc.Bytes()); err != nil {
			return err
		}
	}
	return sc.Err()
}

// lockedWriter writes whole lines from several goroutines.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) writeLine(line []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	buf := make([]byte, 0, len(line)+1)
	buf = append(buf, line...)
	buf = append(buf, '\n')
	_, err := l.w.Write(buf)
	return err
}
