package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
)

// JSON-RPC error codes sent back to the client.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	// CodeDenied is returned when a tools/call is denied by policy.
	// It is in the range JSON-RPC reserves for implementation errors.
	CodeDenied = -32001
)

// object is a JSON object decoded one level deep. Keys are exact (no case
// folding) and a duplicate key keeps only its last value.
type object map[string]json.RawMessage

func parseObject(data []byte) (object, error) {
	var o object
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errNotObject
	}
	return o, nil
}

// dropCaseVariants removes keys that differ from one of names only by case
// ("Method", "NAME"). Some decoders match keys without case, so a server
// could read such a key instead of the one the proxy checked.
func (o object) dropCaseVariants(names ...string) {
	for k := range o {
		for _, name := range names {
			if k != name && strings.EqualFold(k, name) {
				delete(o, k)
			}
		}
	}
}

// str returns the string value of key, and false if it is missing or not a string.
func (o object) str(key string) (string, bool) {
	raw, ok := o[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// id returns the request id, or nil for a notification.
func (o object) id() json.RawMessage {
	raw, ok := o["id"]
	if !ok {
		return nil
	}
	return raw
}

// idKey turns a JSON id into a comparable key, ignoring whitespace.
func idKey(id json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, id); err != nil {
		return string(id)
	}
	return buf.String()
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   rpcError        `json:"error"`
}

func newError(id json.RawMessage, code int, msg string, data any) errorResponse {
	if id == nil {
		id = json.RawMessage("null")
	}
	return errorResponse{JSONRPC: "2.0", ID: id, Error: rpcError{Code: code, Message: msg, Data: data}}
}
