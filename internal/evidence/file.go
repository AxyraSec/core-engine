package evidence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxLineSize is the largest record line read back from a file.
const maxLineSize = 1 << 20 // 1 MiB

// ErrClosed is returned when appending to a closed FileLog.
var ErrClosed = errors.New("evidence log is closed")

// FileLog is an evidence chain stored in a JSON Lines file: one record per line.
//
// Every Append is written and synced to disk before it returns. On open, the
// whole file is verified first: a broken or partly written file is refused,
// so that new records are never appended to a chain that cannot be trusted.
//
// It is safe for concurrent use. Only one process should write a given file.
type FileLog struct {
	mu   sync.Mutex
	f    *os.File
	last *Record
	now  func() time.Time
}

// OpenFile opens or creates the evidence file at path and verifies it.
// New records continue the chain found in the file.
func OpenFile(path string, opts ...Option) (*FileLog, error) {
	f, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	last, _, err := scan(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &FileLog{f: f, last: last, now: newConfig(opts).now}, nil
}

// Append adds e to the chain, writes it to disk and syncs the file.
// If it returns an error, the record must be considered not written.
func (l *FileLog) Append(e Entry) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.f == nil {
		return Record{}, ErrClosed
	}

	r := next(l.last, e, l.now())
	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, err
	}
	line = append(line, '\n')

	if _, err := l.f.Write(line); err != nil {
		return Record{}, err
	}
	if err := l.f.Sync(); err != nil {
		return Record{}, err
	}

	l.last = &r
	return r, nil
}

// Head returns the seq and hash of the last record, or 0 and GenesisHash.
func (l *FileLog) Head() (uint64, Hash) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last == nil {
		return 0, GenesisHash
	}
	return l.last.Seq, l.last.Hash
}

// Close closes the file. Later appends return ErrClosed.
func (l *FileLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// VerifyFile checks the evidence file at path and returns the number of
// records and the hash of the last one.
func VerifyFile(path string) (uint64, Hash, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return 0, GenesisHash, err
	}
	defer func() { _ = f.Close() }()

	last, n, err := scan(f)
	if err != nil {
		return 0, GenesisHash, fmt.Errorf("%s: %w", path, err)
	}
	if last == nil {
		return 0, GenesisHash, nil
	}
	return n, last.Hash, nil
}

// scan reads and verifies every record of r. It returns the last record
// (nil if there is none) and the number of records.
func scan(r io.Reader) (*Record, uint64, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var (
		last *Record
		n    uint64
	)
	prev := GenesisHash
	for i := 0; sc.Scan(); i++ {
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()

		var rec Record
		if err := dec.Decode(&rec); err != nil {
			return nil, 0, fmt.Errorf("%w: record #%d: %w", ErrBrokenChain, i, err)
		}
		if err := check(i, &rec, n+1, prev); err != nil {
			return nil, 0, err
		}
		n++
		prev = rec.Hash
		last = &rec
	}
	if err := sc.Err(); err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrBrokenChain, err)
	}
	return last, n, nil
}
