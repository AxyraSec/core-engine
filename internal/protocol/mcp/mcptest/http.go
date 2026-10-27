package mcptest

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

// SessionHeader carries the MCP session ID on the Streamable HTTP transport.
const SessionHeader = "Mcp-Session-Id"

// HTTPServer serves the same tools as Serve on the Streamable HTTP transport
// (MCP 2025-06-18), at any path. It answers every request with a single JSON
// response and never opens an SSE stream.
//
// It gives a session ID on initialize and checks it on later requests, so a
// proxy in front of it must pass the header through.
type HTTPServer struct {
	mu       sync.Mutex
	sessions map[string]bool
}

// NewHTTPServer returns a server with no sessions.
func NewHTTPServer() *HTTPServer {
	return &HTTPServer{sessions: map[string]bool{}}
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.post(w, r)
	case http.MethodDelete:
		s.mu.Lock()
		delete(s.sessions, r.Header.Get(SessionHeader))
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		// No server-initiated stream: GET is not offered.
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *HTTPServer) post(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var head struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &head)

	if head.Method == "initialize" {
		id := rand.Text()
		s.mu.Lock()
		s.sessions[id] = true
		s.mu.Unlock()
		w.Header().Set(SessionHeader, id)
	} else {
		s.mu.Lock()
		ok := s.sessions[r.Header.Get(SessionHeader)]
		s.mu.Unlock()
		if !ok {
			http.Error(w, "unknown or missing session", http.StatusNotFound)
			return
		}
	}

	resp := Answer(body)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted) // notification or response
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
