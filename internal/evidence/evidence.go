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

// Entry is the decision to record.
type Entry struct {
	Subject  string
	Action   string
	Resource string
	Allowed  bool
	RuleID   string
	Reason   string
}

// Record is an Entry once it is in the chain.
type Record struct {
	Entry
	// Seq is the position in the chain, starting at 1.
	Seq uint64
	// Time is when the record was appended, in UTC.
	Time time.Time
	// PrevHash is the Hash of the previous record, or GenesisHash for the first one.
	PrevHash Hash
	// Hash covers every other field of the record.
	Hash Hash
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
