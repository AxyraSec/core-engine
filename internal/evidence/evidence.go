// Package evidence keeps a tamper-evident log of authorization decisions.
//
// Each record holds the hash of the previous one, so the records form a chain.
// Changing, removing, inserting or reordering a record breaks the chain, and
// Verify reports where.
//
// This package must not import any other internal module: callers copy the
// decision fields into an Entry.
package evidence

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// Hash is a SHA-256 digest.
type Hash [sha256.Size]byte

// GenesisHash is the PrevHash of the first record in a chain.
var GenesisHash Hash

// String returns the hash in lowercase hex.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalText encodes the hash as lowercase hex.
func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

// UnmarshalText decodes a hash written by MarshalText.
func (h *Hash) UnmarshalText(text []byte) error {
	if len(text) != 2*len(h) {
		return fmt.Errorf("hash: want %d hex characters, got %d", 2*len(h), len(text))
	}
	_, err := hex.Decode(h[:], text)
	return err
}

// Entry is the decision to record.
type Entry struct {
	Subject  string `json:"subject"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Allowed  bool   `json:"allowed"`
	RuleID   string `json:"rule_id"`
	Reason   string `json:"reason"`
}

// Record is an Entry once it is in the chain.
type Record struct {
	// Seq is the position in the chain, starting at 1.
	Seq uint64 `json:"seq"`
	// Time is when the record was appended, in UTC.
	Time time.Time `json:"time"`
	Entry
	// PrevHash is the Hash of the previous record, or GenesisHash for the first one.
	PrevHash Hash `json:"prev_hash"`
	// Hash covers every other field of the record.
	Hash Hash `json:"hash"`
}

// next builds the record that follows prev, or the first record when prev is nil.
func next(prev *Record, e Entry, now time.Time) Record {
	r := Record{Seq: 1, Time: now.UTC(), Entry: e, PrevHash: GenesisHash}
	if prev != nil {
		r.Seq = prev.Seq + 1
		r.PrevHash = prev.Hash
	}
	r.Hash = r.computeHash()
	return r
}

// check verifies that r is a valid record #i, following a record with hash prev.
func check(i int, r *Record, wantSeq uint64, prev Hash) error {
	switch {
	case r.Seq != wantSeq:
		return fmt.Errorf("%w: record #%d: seq is %d, want %d", ErrBrokenChain, i, r.Seq, wantSeq)
	case r.PrevHash != prev:
		return fmt.Errorf("%w: record #%d: prev hash does not match the previous record", ErrBrokenChain, i)
	case r.computeHash() != r.Hash:
		return fmt.Errorf("%w: record #%d: content does not match its hash", ErrBrokenChain, i)
	}
	return nil
}

// recordVersion is part of every hash, so the encoding can change later
// without old and new records being confused.
const recordVersion = "axyra.evidence.v1"

// computeHash hashes every field of r except Hash.
// Strings are length-prefixed so that field boundaries cannot be shifted
// ("ab"+"c" and "a"+"bc" give different hashes).
func (r *Record) computeHash() Hash {
	buf := make([]byte, 0, 256)

	appendString := func(s string) {
		buf = strconv.AppendInt(buf, int64(len(s)), 10)
		buf = append(buf, ':')
		buf = append(buf, s...)
	}

	appendString(recordVersion)
	buf = binary.BigEndian.AppendUint64(buf, r.Seq)
	appendString(r.Time.UTC().Format(time.RFC3339Nano))
	buf = append(buf, r.PrevHash[:]...)
	appendString(r.Subject)
	appendString(r.Action)
	appendString(r.Resource)
	if r.Allowed {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	appendString(r.RuleID)
	appendString(r.Reason)

	return sha256.Sum256(buf)
}
