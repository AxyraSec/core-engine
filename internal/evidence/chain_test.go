package evidence

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// fixedClock returns a clock that starts at a fixed time and moves 1s per call.
func fixedClock() func() time.Time {
	t := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		t = t.Add(time.Second)
		return t
	}
}

func sampleEntries() []Entry {
	return []Entry{
		{Subject: "agent:a", Action: "tools/call", Resource: "mcp:github/create_issue", Allowed: true, RuleID: "allow-github", Reason: "allowed_by_rule"},
		{Subject: "agent:a", Action: "tools/call", Resource: "mcp:github/delete_repo", RuleID: "deny-delete", Reason: "denied_by_rule"},
		{Subject: "agent:b", Action: "resources/read", Resource: "doc:1", Reason: "no_matching_rule"},
	}
}

func buildChain(t *testing.T) *Chain {
	t.Helper()
	c := NewChain(WithClock(fixedClock()))
	for _, e := range sampleEntries() {
		c.Append(e)
	}
	return c
}

func TestAppendLinksRecords(t *testing.T) {
	c := buildChain(t)
	records := c.Records()

	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	if records[0].PrevHash != GenesisHash {
		t.Errorf("first record PrevHash = %s, want genesis", records[0].PrevHash)
	}
	var wantSeq uint64
	for i, r := range records {
		wantSeq++
		if r.Seq != wantSeq {
			t.Errorf("record #%d: Seq = %d, want %d", i, r.Seq, wantSeq)
		}
		if r.Time.Location() != time.UTC {
			t.Errorf("record #%d: time is not UTC", i)
		}
		if i > 0 && r.PrevHash != records[i-1].Hash {
			t.Errorf("record #%d: PrevHash does not match previous Hash", i)
		}
	}

	if err := Verify(records); err != nil {
		t.Fatalf("Verify() on an untouched chain: %v", err)
	}
}

func TestHashIsDeterministic(t *testing.T) {
	a := buildChain(t).Records()
	b := buildChain(t).Records()
	for i := range a {
		if a[i].Hash != b[i].Hash {
			t.Fatalf("record #%d: same input gave different hashes", i)
		}
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	tests := []struct {
		name   string
		tamper func([]Record) []Record
	}{
		{"flip allowed", func(r []Record) []Record { r[1].Allowed = true; return r }},
		{"change subject", func(r []Record) []Record { r[0].Subject = "agent:evil"; return r }},
		{"change resource", func(r []Record) []Record { r[2].Resource = "doc:2"; return r }},
		{"change rule id", func(r []Record) []Record { r[1].RuleID = ""; return r }},
		{"change reason", func(r []Record) []Record { r[2].Reason = "allowed_by_rule"; return r }},
		{"change time", func(r []Record) []Record { r[0].Time = r[0].Time.Add(time.Nanosecond); return r }},
		{"change seq", func(r []Record) []Record { r[2].Seq = 7; return r }},
		{"remove first", func(r []Record) []Record { return r[1:] }},
		{"remove middle", func(r []Record) []Record { return slices.Delete(r, 1, 2) }},
		{"swap two records", func(r []Record) []Record { r[0], r[1] = r[1], r[0]; return r }},
		{"duplicate a record", func(r []Record) []Record { return slices.Insert(r, 1, r[0]) }},
		{
			// An attacker who rewrites a record and recomputes its hash
			// still breaks the link with the next record.
			name: "rewrite and rehash",
			tamper: func(r []Record) []Record {
				r[1].Allowed = true
				r[1].Hash = r[1].computeHash()
				return r
			},
		},
		{
			// Moving bytes from one field to the next must change the hash.
			name: "shift field boundary",
			tamper: func(r []Record) []Record {
				r[2].Action = r[2].Action + r[2].Resource[:1]
				r[2].Resource = r[2].Resource[1:]
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := tt.tamper(buildChain(t).Records())
			if err := Verify(records); !errors.Is(err, ErrBrokenChain) {
				t.Fatalf("Verify() = %v, want ErrBrokenChain", err)
			}
		})
	}
}

func TestVerifyEmpty(t *testing.T) {
	if err := Verify(nil); err != nil {
		t.Fatalf("Verify(nil) = %v, want nil", err)
	}
}

func TestRecordsReturnsACopy(t *testing.T) {
	c := buildChain(t)
	got := c.Records()
	got[0].Allowed = true

	if err := Verify(c.Records()); err != nil {
		t.Fatalf("changing a returned slice altered the chain: %v", err)
	}
}

func TestHead(t *testing.T) {
	c := NewChain(WithClock(fixedClock()))
	if seq, h := c.Head(); seq != 0 || h != GenesisHash {
		t.Fatalf("empty Head() = %d, %s", seq, h)
	}

	var last Record
	for _, e := range sampleEntries() {
		last = c.Append(e)
	}
	if seq, h := c.Head(); seq != last.Seq || h != last.Hash {
		t.Fatalf("Head() = %d, %s, want %d, %s", seq, h, last.Seq, last.Hash)
	}
}

func TestConcurrentAppend(t *testing.T) {
	c := NewChain()
	const workers, perWorker = 8, 100

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				c.Append(Entry{Subject: "agent:a", Action: "tools/call", Resource: "r", Reason: "allowed_by_rule", Allowed: true})
			}
		})
	}
	wg.Wait()

	records := c.Records()
	if len(records) != workers*perWorker {
		t.Fatalf("got %d records, want %d", len(records), workers*perWorker)
	}
	if err := Verify(records); err != nil {
		t.Fatalf("Verify() after concurrent appends: %v", err)
	}
}

func TestHashString(t *testing.T) {
	h := Hash{0xab, 0x01}
	want := "ab01" + "000000000000000000000000000000000000000000000000000000000000"
	if h.String() != want {
		t.Fatalf("String() = %s, want %s", h, want)
	}
	text, err := h.MarshalText()
	if err != nil || string(text) != want {
		t.Fatalf("MarshalText() = %s, %v", text, err)
	}
}
