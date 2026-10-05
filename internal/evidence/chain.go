package evidence

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// ErrBrokenChain is returned by Verify when the records were tampered with.
var ErrBrokenChain = errors.New("evidence chain is broken")

// Chain is an append-only, in-memory chain of records.
// It is safe for concurrent use.
type Chain struct {
	mu      sync.Mutex
	records []Record
	now     func() time.Time
}

// Option configures a Chain.
type Option func(*Chain)

// WithClock sets the clock used to timestamp records. Mostly useful in tests.
func WithClock(now func() time.Time) Option {
	return func(c *Chain) { c.now = now }
}

// NewChain returns an empty chain.
func NewChain(opts ...Option) *Chain {
	c := &Chain{now: time.Now}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Append adds e at the end of the chain and returns the stored record.
func (c *Chain) Append(e Entry) Record {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := Record{
		Entry:    e,
		Seq:      1,
		Time:     c.now().UTC(),
		PrevHash: GenesisHash,
	}
	if n := len(c.records); n > 0 {
		r.Seq = c.records[n-1].Seq + 1
		r.PrevHash = c.records[n-1].Hash
	}
	r.Hash = r.computeHash()

	c.records = append(c.records, r)
	return r
}

// Records returns a copy of all records, oldest first.
func (c *Chain) Records() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.records)
}

// Head returns the number of records and the hash of the last one.
// Publishing the head elsewhere lets you detect later truncation of the chain.
// An empty chain returns 0 and GenesisHash.
func (c *Chain) Head() (uint64, Hash) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.records)
	if n == 0 {
		return 0, GenesisHash
	}
	return c.records[n-1].Seq, c.records[n-1].Hash
}

// Verify checks that records form an unbroken chain starting from GenesisHash.
// It returns an error wrapping ErrBrokenChain that names the first bad record.
//
// Verify cannot detect records removed from the end of the chain: compare the
// result of Head with a copy kept elsewhere for that.
func Verify(records []Record) error {
	prev := GenesisHash
	var want uint64
	for i := range records {
		r := &records[i]
		want++
		switch {
		case r.Seq != want:
			return fmt.Errorf("%w: record #%d: seq is %d, want %d", ErrBrokenChain, i, r.Seq, want)
		case r.PrevHash != prev:
			return fmt.Errorf("%w: record #%d: prev hash does not match the previous record", ErrBrokenChain, i)
		case r.computeHash() != r.Hash:
			return fmt.Errorf("%w: record #%d: content does not match its hash", ErrBrokenChain, i)
		}
		prev = r.Hash
	}
	return nil
}
