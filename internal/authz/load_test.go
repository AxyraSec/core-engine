package authz

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
version: 1
rules:
  # comments are allowed
  - id: allow-github
    effect: allow
    subjects: ["agent:billing-bot"]
    actions: ["tools/call"]
    resources: ["mcp:github/*"]
  - id: deny-delete
    effect: deny
    subjects: ["*"]
    actions: ["tools/call"]
    resources: ["mcp:github/delete_repo"]
`

func TestLoadPolicy(t *testing.T) {
	p, err := LoadPolicy(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", p.Len())
	}

	allowed := p.Evaluate(Request{Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/create_issue"})
	if !allowed.Allowed || allowed.RuleID != "allow-github" {
		t.Errorf("create_issue: got %+v", allowed)
	}
	denied := p.Evaluate(Request{Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/delete_repo"})
	if denied.Allowed || denied.RuleID != "deny-delete" {
		t.Errorf("delete_repo: got %+v", denied)
	}
}

func TestLoadPolicyEmptyRulesDeniesEverything(t *testing.T) {
	p, err := LoadPolicy(strings.NewReader("version: 1\nrules: []\n"))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if d := p.Evaluate(Request{Subject: "a", Action: "b", Resource: "c"}); d.Allowed {
		t.Fatalf("empty policy allowed a request: %+v", d)
	}
}

func TestLoadPolicyRejectsBadFiles(t *testing.T) {
	rule := func(fields string) string {
		return "version: 1\nrules:\n  - " + fields + "\n"
	}
	const okFields = `id: r1
    effect: allow
    subjects: ["*"]
    actions: ["*"]
    resources: ["*"]`

	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"empty input", "", ErrInvalidPolicyFile},
		{"only a comment", "# nothing\n", ErrInvalidPolicyFile},
		{"not yaml", "version: [1\n", ErrInvalidPolicyFile},
		{"missing version", "rules: []\n", ErrInvalidPolicyFile},
		{"unsupported version", "version: 2\nrules: []\n", ErrInvalidPolicyFile},
		{"version as text", "version: one\nrules: []\n", ErrInvalidPolicyFile},
		{"unknown top-level field", "version: 1\ndefault: allow\nrules: []\n", ErrInvalidPolicyFile},
		{"unknown rule field (typo)", rule(strings.Replace(okFields, "effect:", "efect:", 1)), ErrInvalidPolicyFile},
		{"duplicate key", "version: 1\nversion: 1\nrules: []\n", ErrInvalidPolicyFile},
		{"two documents", "version: 1\nrules: []\n---\nversion: 1\nrules: []\n", ErrInvalidPolicyFile},
		{"subjects not a list", rule(strings.Replace(okFields, `subjects: ["*"]`, `subjects: "*"`, 1)), ErrInvalidPolicyFile},
		{"rule fails validation", rule(strings.Replace(okFields, "effect: allow", "effect: Allow", 1)), ErrInvalidRule},
		{"wildcard in the middle", rule(strings.Replace(okFields, `resources: ["*"]`, `resources: ["mcp:*/x"]`, 1)), ErrInvalidRule},
		{"yaml boolean is not an effect", rule(strings.Replace(okFields, "effect: allow", "effect: yes", 1)), ErrInvalidRule},
		{"too large", "version: 1\n# " + strings.Repeat("x", MaxPolicySize) + "\nrules: []\n", ErrInvalidPolicyFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := LoadPolicy(strings.NewReader(tt.in))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("LoadPolicy() error = %v, want %v", err, tt.wantErr)
			}
			if p != nil {
				t.Errorf("LoadPolicy() returned a policy with an error")
			}
		})
	}
}

func TestLoadPolicyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}
	if p.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", p.Len())
	}
}

func TestLoadPolicyFileErrors(t *testing.T) {
	if _, err := LoadPolicyFile(filepath.Join(t.TempDir(), "missing.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: error = %v, want fs.ErrNotExist", err)
	}

	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPolicyFile(path)
	if !errors.Is(err, ErrInvalidPolicyFile) || !strings.Contains(err.Error(), path) {
		t.Errorf("bad file: error = %v, want ErrInvalidPolicyFile naming the path", err)
	}
}

// Every policy shipped in the repo must load.
func TestShippedPolicyFiles(t *testing.T) {
	files, err := filepath.Glob("../../policies/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no policy files found: %v", err)
	}
	for _, f := range files {
		if _, err := LoadPolicyFile(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestLenNilPolicy(t *testing.T) {
	var p *Policy
	if p.Len() != 0 {
		t.Fatal("nil policy Len() != 0")
	}
}

// FuzzLoadPolicy checks that arbitrary input never panics and that a policy
// is returned only without an error.
func FuzzLoadPolicy(f *testing.F) {
	f.Add(validYAML)
	f.Add("version: 1\nrules: []\n")
	f.Add("version: 1\nrules:\n  - &a {id: x}\n  - *a\n")

	f.Fuzz(func(t *testing.T, in string) {
		p, err := LoadPolicy(strings.NewReader(in))
		if (p == nil) == (err == nil) {
			t.Fatalf("got policy=%v err=%v", p != nil, err)
		}
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestLoadPolicyReadError(t *testing.T) {
	if _, err := LoadPolicy(failingReader{}); err == nil {
		t.Fatal("LoadPolicy() with a failing reader returned no error")
	}
}
