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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// ordinals preserves attemptID -> ordinal across resolves (the
	// unresolved index alone deletes entries on definitive outcomes, which
	// used to make a late second Resolve fall back to nextOrd and collide
	// ordinals with the next Allocate).
	ordinals map[string]int64
	records  int64
	now      func() time.Time
}

// maxCraftEgressJournalRecords bounds the append-only journal from
// unbounded growth (a sandboxed client can append at fsync rate with fresh
// body fingerprints). Past the cap, allocation fails closed (503) instead of
// silently consuming the host disk and replay memory.
const maxCraftEgressJournalRecords = 1 << 20

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
		unresolved: make(map[string]CraftEgressAttemptRecord), ordinals: make(map[string]int64),
		now: time.Now}
	if err := journal.replay(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return journal, nil
}

func (j *CraftEgressAttemptJournal) replay() error {
	data, err := os.ReadFile(j.path)
	if err != nil {
		return fmt.Errorf("craftegress: journal replay: %w", err)
	}
	states := make(map[string]CraftEgressAttemptRecord) // attemptID -> latest record
	offset := 0
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		if offset > len(data) {
			return fmt.Errorf("craftegress: journal replay offset overshot")
		}
		if line == "" {
			if index == len(lines)-1 {
				break // trailing newline
			}
			offset += 1
			continue
		}
		var record CraftEgressAttemptRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil || record.AttemptID == "" || record.RequestDigest == "" || record.Ordinal <= 0 {
			// A crash between Write and the newline hitting the disk can
			// leave ONE torn record at the very end of the append-only
			// journal. Only that trailing fragment is tolerated (and
			// truncated away): an unreadable line in the middle means real
			// corruption and refuses to start rather than silently
			// rewriting history.
			if index == len(lines)-1 {
				if truncateErr := j.file.Truncate(int64(offset)); truncateErr != nil {
					return fmt.Errorf("craftegress: journal torn tail truncate: %w", truncateErr)
				}
				break
			}
			return fmt.Errorf("craftegress: journal record unreadable at byte %d", offset)
		}
		states[record.AttemptID] = record
		j.ordinals[record.AttemptID] = record.Ordinal
		j.records++
		if record.Ordinal >= j.nextOrd {
			j.nextOrd = record.Ordinal + 1
		}
		offset += len(line) + 1
	}
	for _, record := range states {
		if record.State == CraftEgressAttemptUnresolved {
			j.unresolved[record.RequestDigest] = record
		}
	}
	// A complete final record whose trailing newline was lost would make the
	// next append concatenate onto it; restore the separator explicitly.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := j.file.Write([]byte("\n")); err != nil {
			return fmt.Errorf("craftegress: journal newline restore: %w", err)
		}
		if err := j.file.Sync(); err != nil {
			return fmt.Errorf("craftegress: journal newline restore fsync: %w", err)
		}
	}
	return nil
}

// AllocateIfNotParked atomically returns the parked (unresolved) attempt for
// a fingerprint, minting and durably persisting a new one only when none is
// parked. The single-lock check-and-mint is what preserves the protocol
// invariant "at most one parked identity per fingerprint": separate Reuse and
// Allocate calls would let two concurrent same-fingerprint requests each mint
// an identity and corrupt the parked index.
func (j *CraftEgressAttemptJournal) AllocateIfNotParked(requestDigest string) (CraftEgressAttemptRecord, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if parked, ok := j.unresolved[requestDigest]; ok {
		return parked, false, nil
	}
	if j.records >= maxCraftEgressJournalRecords {
		return CraftEgressAttemptRecord{}, false, fmt.Errorf("craftegress: attempt journal reached its record cap %d", maxCraftEgressJournalRecords)
	}
	record := CraftEgressAttemptRecord{
		Ordinal:       j.nextOrd,
		AttemptID:     mintCraftEgressAttemptID(j.nextOrd),
		RequestDigest: requestDigest,
		State:         CraftEgressAttemptUnresolved,
		CreatedNano:   j.now().UnixNano(),
	}
	if err := j.appendLocked(record); err != nil {
		return CraftEgressAttemptRecord{}, false, err
	}
	j.nextOrd++
	j.records++
	j.ordinals[record.AttemptID] = record.Ordinal
	j.unresolved[requestDigest] = record
	return record, true, nil
}

// Reuse returns the durable unresolved attempt for a fingerprint. ok=false
// means the fingerprint has no parked identity and a new one must be minted.
//
// NOTE: production traffic must go through AllocateIfNotParked — this
// exported read exists for reconciliation/audit tooling that needs to
// inspect the parked identity WITHOUT minting; it never mutates state.
func (j *CraftEgressAttemptJournal) Reuse(requestDigest string) (CraftEgressAttemptRecord, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record, ok := j.unresolved[requestDigest]
	return record, ok
}

// Resolve durably records a definitive outcome for one attempt. A gateway
// conflict (ACTIVITY_UNRESOLVED) keeps the attempt unresolved (parked) and
// only records the observed status. The durable append commits BEFORE the
// in-memory index changes, mirroring Allocate's fail-closed ordering: if the
// append fails, the process still holds the old state and a restarted
// replay cannot resurrect an identity the journal never recorded as
// resolved.
func (j *CraftEgressAttemptJournal) Resolve(attemptID, requestDigest string, gatewayStatus int, definitive bool) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	state := CraftEgressAttemptUnresolved
	var resolvedNano int64
	if definitive {
		state = CraftEgressAttemptResolved
		resolvedNano = j.now().UnixNano()
	}
	if err := j.appendLocked(CraftEgressAttemptRecord{
		Ordinal: j.ordinalLocked(attemptID), AttemptID: attemptID, RequestDigest: requestDigest,
		State: state, GatewayStatus: gatewayStatus, CreatedNano: j.now().UnixNano(), ResolvedNano: resolvedNano,
	}); err != nil {
		return err
	}
	j.records++
	if definitive {
		// Guard the one-parked-identity invariant: only the holder of the
		// CURRENT parked id may unpark the digest (a late duplicate resolve
		// must not erase a newer id's parked state).
		if parked, ok := j.unresolved[requestDigest]; ok && parked.AttemptID == attemptID {
			delete(j.unresolved, requestDigest)
		}
	} else if parked, ok := j.unresolved[requestDigest]; ok && parked.AttemptID == attemptID {
		// Backfill ONLY when this journal's parked id is still the CURRENT
		// holder for the digest: an unknown observation on an id that a
		// racing definitive resolve already unparked must NOT resurrect it
		// (that parks a gateway-finalized identity forever — the 409 loop).
		// The appended record remains on disk as a reconciliation trail,
		// but the in-memory index follows the durable latest state.
		_ = parked
	}
	return nil
}

func (j *CraftEgressAttemptJournal) ordinalLocked(attemptID string) int64 {
	// The dedicated ordinal index survives definitive resolves; a miss (a
	// foreign id) returns 0 rather than nextOrd, which used to collide the
	// transition record with the NEXT Allocate's ordinal.
	if ordinal, ok := j.ordinals[attemptID]; ok {
		return ordinal
	}
	return 0
}

func (j *CraftEgressAttemptJournal) appendLocked(record CraftEgressAttemptRecord) error {
	// A non-positive ordinal replays as unreadable on the next start and
	// would refuse the whole journal (self-poisoning) — refuse at write time.
	if record.Ordinal <= 0 {
		return fmt.Errorf("craftegress: journal record ordinal must be positive, got %d", record.Ordinal)
	}
	// The record cap bounds Resolve-side growth too: a persistently failing
	// gateway drives an unlimited unknown-resolve retry loop on an already
	// parked fingerprint, and without this check the cap only stopped NEW
	// allocations.
	if j.records >= maxCraftEgressJournalRecords {
		return fmt.Errorf("craftegress: attempt journal reached its record cap %d", maxCraftEgressJournalRecords)
	}
	if j.file == nil {
		// In-flight handlers can outlive the shutdown budget and reach here
		// after Close; a nil dereference panic serves nobody.
		return fmt.Errorf("craftegress: journal is closed")
	}
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
