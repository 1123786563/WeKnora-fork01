// Package craftegress implements the per-Run model-egress attempt authority
// from the T19 activity-ID protocol research
// (docs/plans/2026-09-24-craft-107-t19-activity-id-protocol.md): the only
// configured provider endpoint of the craft runtime, it durably allocates an
// opaque attempt identity immediately before each physical provider send,
// reuses that identity after an unknown outcome, allocates a distinct identity
// for a deliberately new attempt, and injects X-Craft-Activity-ID toward the
// Craft model gateway. The gateway stays fail-closed without this proof.
package craftegress

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CraftEgressAttemptState names the durable lifecycle of one attempt record.
type CraftEgressAttemptState string

const (
	// CraftEgressAttemptUnresolved means the physical send left no definitive
	// outcome: the identity must be reused, never re-minted.
	CraftEgressAttemptUnresolved CraftEgressAttemptState = "unresolved"
	// CraftEgressAttemptResolved means a definitive response completed; a later
	// request with the same fingerprint is a deliberately new attempt.
	CraftEgressAttemptResolved CraftEgressAttemptState = "resolved"
)

// CraftEgressAttemptRecord is one append-only journal transition. It never
// carries request bodies, headers or credentials — only identities and state.
type CraftEgressAttemptRecord struct {
	Ordinal       int64                   `json:"ordinal"`
	AttemptID     string                  `json:"attempt_id"`
	RequestDigest string                  `json:"request_digest"`
	State         CraftEgressAttemptState `json:"state"`
	GatewayStatus int                     `json:"gateway_status,omitempty"`
	CreatedNano   int64                   `json:"created_nano"`
	ResolvedNano  int64                   `json:"resolved_nano,omitempty"`
}

// CraftEgressAttemptJournal is the durable append-only authority for one Run's
// model egress attempts. Every mutation is fsynced before the caller may act
// on it; replay on construction rebuilds the per-fingerprint index so a
// restarted adapter still knows its unresolved identities.
type CraftEgressAttemptJournal struct {
	mu         sync.Mutex
	path       string
	file       *os.File
	nextOrd    int64
	unresolved map[string]CraftEgressAttemptRecord // requestDigest -> unresolved attempt
	resolved   map[string]bool                     // requestDigest -> has resolved attempt
	now        func() time.Time
}

func OpenCraftEgressAttemptJournal(path string) (*CraftEgressAttemptJournal, error) {
	if path == "" {
		return nil, fmt.Errorf("craftegress: journal path is required")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("craftegress: journal directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("craftegress: journal open: %w", err)
	}
	journal := &CraftEgressAttemptJournal{path: path, file: file, nextOrd: 1,
		unresolved: make(map[string]CraftEgressAttemptRecord), resolved: make(map[string]bool),
		now: time.Now}
	if err := journal.replay(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return journal, nil
}

func (j *CraftEgressAttemptJournal) replay() error {
	reader, err := os.Open(j.path)
	if err != nil {
		return fmt.Errorf("craftegress: journal replay: %w", err)
	}
	defer func() { _ = reader.Close() }()
	states := make(map[string]CraftEgressAttemptRecord) // attemptID -> latest record
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record CraftEgressAttemptRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return fmt.Errorf("craftegress: journal record unreadable: %w", err)
		}
		if record.AttemptID == "" || record.RequestDigest == "" || record.Ordinal <= 0 {
			return fmt.Errorf("craftegress: journal record incomplete")
		}
		states[record.AttemptID] = record
		if record.Ordinal >= j.nextOrd {
			j.nextOrd = record.Ordinal + 1
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("craftegress: journal replay: %w", err)
	}
	for _, record := range states {
		switch record.State {
		case CraftEgressAttemptUnresolved:
			j.unresolved[record.RequestDigest] = record
		case CraftEgressAttemptResolved:
			j.resolved[record.RequestDigest] = true
		}
	}
	return nil
}

// Allocate mints the next opaque identity for a fingerprint that has no
// unresolved attempt, persisting the unresolved record BEFORE returning. The
// caller may only forward after this commit succeeded.
func (j *CraftEgressAttemptJournal) Allocate(requestDigest string) (CraftEgressAttemptRecord, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record := CraftEgressAttemptRecord{
		Ordinal:       j.nextOrd,
		AttemptID:     mintCraftEgressAttemptID(j.nextOrd),
		RequestDigest: requestDigest,
		State:         CraftEgressAttemptUnresolved,
		CreatedNano:   j.now().UnixNano(),
	}
	if err := j.appendLocked(record); err != nil {
		return CraftEgressAttemptRecord{}, err
	}
	j.nextOrd++
	j.unresolved[requestDigest] = record
	return record, nil
}

// Reuse returns the durable unresolved attempt for a fingerprint. ok=false
// means the fingerprint has no parked identity and a new one must be minted.
func (j *CraftEgressAttemptJournal) Reuse(requestDigest string) (CraftEgressAttemptRecord, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record, ok := j.unresolved[requestDigest]
	return record, ok
}

// Resolve durably records a definitive outcome for one attempt. A gateway 409
// keeps the attempt unresolved (parked) and only records the observed status.
func (j *CraftEgressAttemptJournal) Resolve(attemptID, requestDigest string, gatewayStatus int, definitive bool) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	state := CraftEgressAttemptUnresolved
	var resolvedNano int64
	if definitive {
		state = CraftEgressAttemptResolved
		resolvedNano = j.now().UnixNano()
		delete(j.unresolved, requestDigest)
		j.resolved[requestDigest] = true
	}
	return j.appendLocked(CraftEgressAttemptRecord{
		Ordinal: j.ordinalLocked(attemptID), AttemptID: attemptID, RequestDigest: requestDigest,
		State: state, GatewayStatus: gatewayStatus, CreatedNano: j.now().UnixNano(), ResolvedNano: resolvedNano,
	})
}

func (j *CraftEgressAttemptJournal) ordinalLocked(attemptID string) int64 {
	// Transitions reuse the attempt's original ordinal when known; a foreign
	// id cannot occur because only Allocate minted ids reach this path.
	for _, record := range j.unresolved {
		if record.AttemptID == attemptID {
			return record.Ordinal
		}
	}
	return j.nextOrd
}

func (j *CraftEgressAttemptJournal) appendLocked(record CraftEgressAttemptRecord) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("craftegress: journal encode: %w", err)
	}
	encoded = append(encoded, '\n')
	if _, err := j.file.Write(encoded); err != nil {
		return fmt.Errorf("craftegress: journal write: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return fmt.Errorf("craftegress: journal fsync: %w", err)
	}
	return nil
}

func (j *CraftEgressAttemptJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}

func mintCraftEgressAttemptID(ordinal int64) string {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		// The ordinal alone is still unique per journal; entropy loss only
		// widens predictability, never identity.
		return fmt.Sprintf("aeg-%d", ordinal)
	}
	return fmt.Sprintf("aeg-%d-%s", ordinal, hex.EncodeToString(entropy[:]))
}
