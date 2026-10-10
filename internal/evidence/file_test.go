package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func openTemp(t *testing.T) (*FileLog, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	l, err := OpenFile(path, WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, path
}

func appendAll(t *testing.T, l *FileLog) Record {
	t.Helper()
	var last Record
	for _, e := range sampleEntries() {
		r, err := l.Append(e)
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
		last = r
	}
	return last
}

func TestFileLogWritesAVerifiableChain(t *testing.T) {
	l, path := openTemp(t)
	last := appendAll(t, l)

	n, head, err := VerifyFile(path)
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if n != 3 || head != last.Hash {
		t.Fatalf("VerifyFile = %d, %s; want 3, %s", n, head, last.Hash)
	}
	if seq, h := l.Head(); seq != 3 || h != last.Hash {
		t.Fatalf("Head = %d, %s", seq, h)
	}
}

func TestFileLogMatchesInMemoryChain(t *testing.T) {
	l, _ := openTemp(t)
	fileLast := appendAll(t, l)

	c := buildChain(t)
	_, memHead := c.Head()
	if fileLast.Hash != memHead {
		t.Fatal("file and in-memory chains give different hashes for the same input")
	}
}

func TestFileLogContinuesAfterReopen(t *testing.T) {
	l, path := openTemp(t)
	appendAll(t, l)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	l2, err := OpenFile(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = l2.Close() }()

	r, err := l2.Append(Entry{Subject: "agent:c", Action: "tools/call", Resource: "x", Reason: "no_matching_rule"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Seq != 4 {
		t.Fatalf("seq after reopen = %d, want 4", r.Seq)
	}
	if n, _, err := VerifyFile(path); err != nil || n != 4 {
		t.Fatalf("VerifyFile = %d, %v", n, err)
	}
}

func TestFileLogRefusesTamperedFile(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(string) string
	}{
		{"flip a decision", func(s string) string {
			return strings.Replace(s, `"allowed":false`, `"allowed":true`, 1)
		}},
		{"remove a line", func(s string) string {
			lines := strings.SplitAfter(s, "\n")
			return lines[0] + lines[2]
		}},
		{"partial last line", func(s string) string { return s[:len(s)-10] }},
		{"unknown field", func(s string) string {
			return strings.Replace(s, `{"seq":1,`, `{"seq":1,"extra":1,`, 1)
		}},
		{"not json", func(s string) string { return s + "garbage\n" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, path := openTemp(t)
			appendAll(t, l)
			_ = l.Close()

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.tamper(string(data))), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, _, err := VerifyFile(path); !errors.Is(err, ErrBrokenChain) {
				t.Errorf("VerifyFile error = %v, want ErrBrokenChain", err)
			}
			if _, err := OpenFile(path); !errors.Is(err, ErrBrokenChain) {
				t.Errorf("OpenFile error = %v, want ErrBrokenChain", err)
			}
		})
	}
}

func TestFileLogClosed(t *testing.T) {
	l, _ := openTemp(t)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := l.Append(Entry{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Append after Close = %v, want ErrClosed", err)
	}
}

func TestFileLogEmpty(t *testing.T) {
	l, path := openTemp(t)
	if seq, h := l.Head(); seq != 0 || h != GenesisHash {
		t.Fatalf("Head = %d, %s", seq, h)
	}
	if n, h, err := VerifyFile(path); err != nil || n != 0 || h != GenesisHash {
		t.Fatalf("VerifyFile = %d, %s, %v", n, h, err)
	}
}

func TestFileLogConcurrentAppend(t *testing.T) {
	l, path := openTemp(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 25 {
				if _, err := l.Append(Entry{Subject: "a", Action: "b", Resource: "c", Reason: "d"}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	if n, _, err := VerifyFile(path); err != nil || n != 200 {
		t.Fatalf("VerifyFile = %d, %v", n, err)
	}
}

func TestOpenFileErrors(t *testing.T) {
	if _, err := OpenFile(filepath.Join(t.TempDir(), "missing-dir", "e.jsonl")); err == nil {
		t.Error("OpenFile in a missing directory: no error")
	}
	if _, _, err := VerifyFile(filepath.Join(t.TempDir(), "missing.jsonl")); err == nil {
		t.Error("VerifyFile on a missing file: no error")
	}
}

func TestHashUnmarshalText(t *testing.T) {
	var h Hash
	if err := h.UnmarshalText([]byte("abc")); err == nil {
		t.Error("short hash accepted")
	}
	if err := h.UnmarshalText([]byte(strings.Repeat("zz", 32))); err == nil {
		t.Error("non-hex hash accepted")
	}
	want := Hash{0xab}
	if err := h.UnmarshalText([]byte(want.String())); err != nil || h != want {
		t.Errorf("round trip: %s, %v", h, err)
	}
}
