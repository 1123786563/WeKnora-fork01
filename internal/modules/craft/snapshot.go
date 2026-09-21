package craft

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SnapshotIDPrefix marks every persisted snapshot identity. The remainder of
// the id is the hex digest of the snapshot's content identity, so a well
// formed snapshot id is exactly "snap_" + 64 lowercase hex characters.
const SnapshotIDPrefix = "snap_"

// SnapshotManifestVersion is the schema version stamped into every snapshot
// manifest. A manifest whose version is not understood by the reading
// process is a restore-time unsupported, never a best-effort parse.
const SnapshotManifestVersion = "1"

// Snapshot object kinds recorded on craft_snapshot_objects rows. File objects
// reference the immutable version objects the snapshot's VersionID pins
// (same storage refs, same digests); session objects reference the exported
// OpenCode persistent data uploaded into controlled storage by the capture.
const (
	// SnapshotObjectFile references one version file object.
	SnapshotObjectFile = "file"
	// SnapshotObjectSession references one exported OpenCode data object.
	SnapshotObjectSession = "session"
)

// Snapshot is the durable identity of one complete recovery snapshot: the
// workspace files AND the OpenCode session state of one version, captured
// only while the execution was verifiably quiescent. An empty SessionDigest
// is a files-only export and is never a complete snapshot.
type Snapshot struct {
	WorkspaceID, VersionID, FilesDigest, SessionDigest, RuntimeDigest string
	Quiescent                                                         bool
}

// CanRestore reports whether this snapshot may be restored right now: never
// over an active execution, only a quiescent capture, only with the complete
// identity (workspace, version, both digests), and only onto the runtime the
// snapshot was captured on — a changed runtime invalidates observation
// comparability the same way it does for post-failure reconciliation (C04).
func CanRestore(s Snapshot, active bool, runtimeDigest string) bool {
	return !active && s.Quiescent && s.WorkspaceID != "" && s.VersionID != "" &&
		s.FilesDigest != "" && s.SessionDigest != "" && s.RuntimeDigest == runtimeDigest
}

// SnapshotKey derives the immutable content identity of a snapshot from the
// workspace, the version, both digests and the recorded runtime. It is a
// pure function of its inputs: the same quiescent state always derives the
// same key, so a capture retried after a crash adopts the stored row.
func SnapshotKey(s Snapshot) string {
	raw, _ := json.Marshal([]string{
		s.WorkspaceID, s.VersionID, s.FilesDigest, s.SessionDigest, s.RuntimeDigest,
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// SnapshotID is the durable row identity of one snapshot: the snapshot key
// in its persisted form.
func SnapshotID(s Snapshot) string {
	return SnapshotIDPrefix + SnapshotKey(s)
}

// SessionRecord is one message of the exported OpenCode session chain in its
// canonical snapshot form. Parts are the verbatim JSON part objects of the
// pinned OpenCode 1.18.4 message projection — bytes in, bytes out, so the
// digest follows exactly what the runtime persisted.
type SessionRecord struct {
	ID          string   `json:"id"`
	ParentID    string   `json:"parent_id"`
	Role        string   `json:"role"`
	Finish      string   `json:"finish"`
	CompletedAt int64    `json:"completed_at"`
	Parts       []string `json:"parts"`
}

// sessionDigestEntry is the canonical per-record form hashed into a session
// digest. Field order is fixed by struct order; parts are copied verbatim.
type sessionDigestEntry struct {
	ID          string   `json:"id"`
	ParentID    string   `json:"parent_id"`
	Role        string   `json:"role"`
	Finish      string   `json:"finish"`
	CompletedAt int64    `json:"completed_at"`
	Parts       []string `json:"parts"`
}

// SessionDigest derives the content digest of one exported OpenCode session
// chain: records sorted by message id — the pinned id scheme sorts
// lexicographically in submission order, so listing order cannot change
// identity — each record contributing its full canonical form. An empty
// chain has no digest: a session that never exchanged a message is not a
// restorable session state.
func SessionDigest(records []SessionRecord) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("%w: session export has no records", ErrInvalidInput)
	}
	entries := make([]sessionDigestEntry, 0, len(records))
	seen := make(map[string]bool, len(records))
	for _, r := range records {
		if r.ID == "" {
			return "", fmt.Errorf("%w: session record without id", ErrInvalidInput)
		}
		if seen[r.ID] {
			return "", fmt.Errorf("%w: duplicate session record %s", ErrInvalidInput, r.ID)
		}
		seen[r.ID] = true
		entry := sessionDigestEntry{
			ID: r.ID, ParentID: r.ParentID, Role: r.Role,
			Finish: r.Finish, CompletedAt: r.CompletedAt, Parts: r.Parts,
		}
		if entry.Parts == nil {
			entry.Parts = []string{}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ValidSessionChain verifies the exported chain is a real message chain:
// every record's parent is either empty (a chain root) or the id of an
// earlier record in the chain. A chain whose parent points forward or
// outside the chain is not the session's history and can never be adopted
// as one.
func ValidSessionChain(records []SessionRecord) error {
	if len(records) == 0 {
		return fmt.Errorf("%w: session export has no records", ErrInvalidInput)
	}
	known := make(map[string]bool, len(records))
	for _, r := range records {
		if r.ID == "" {
			return fmt.Errorf("%w: session record without id", ErrInvalidInput)
		}
		if known[r.ID] {
			return fmt.Errorf("%w: duplicate session record %s", ErrInvalidInput, r.ID)
		}
		if r.ParentID != "" && !known[r.ParentID] {
			return fmt.Errorf("%w: session record %s parents unknown %s", ErrInvalidInput, r.ID, r.ParentID)
		}
		known[r.ID] = true
	}
	return nil
}

// SessionChainPrefix verifies that the snapshot's exported chain is a
// verifiable prefix of the live session chain: same records, same order,
// deep-equal content. A restore continues the session's atomic history —
// the live chain may have moved past the snapshot (a later round already
// ran), but it must still contain the snapshot's chain verbatim; a chain
// that diverged or replaced the snapshot's records is not the same session
// history and can never be adopted as one.
func SessionChainPrefix(prefix, chain []SessionRecord) error {
	if len(prefix) == 0 {
		return fmt.Errorf("%w: snapshot chain is empty", ErrInvalidInput)
	}
	if len(prefix) > len(chain) {
		return fmt.Errorf("%w: live chain has %d records, snapshot pins %d; the session history no longer contains the snapshot state",
			ErrConflict, len(chain), len(prefix))
	}
	for i := range prefix {
		a, b := prefix[i], chain[i]
		if a.ID != b.ID || a.ParentID != b.ParentID || a.Role != b.Role ||
			a.Finish != b.Finish || a.CompletedAt != b.CompletedAt {
			return fmt.Errorf("%w: live chain record %d (%s) differs from the snapshot record %s",
				ErrConflict, i, b.ID, a.ID)
		}
		if len(a.Parts) != len(b.Parts) {
			return fmt.Errorf("%w: live chain record %s carries %d parts, snapshot pins %d",
				ErrConflict, a.ID, len(b.Parts), len(a.Parts))
		}
		for p := range a.Parts {
			if a.Parts[p] != b.Parts[p] {
				return fmt.Errorf("%w: live chain record %s part %d differs from the snapshot", ErrConflict, a.ID, p)
			}
		}
	}
	return nil
}

// SessionExport is the OpenCode persistent data of one session, exported
// through the runtime that owns it while the service held the workspace
// lifecycle lock and had verified the runtime quiescent. Reading through
// the owning process is the consistent backup: it never copies a SQLite
// file mid-write or a half-written WAL.
type SessionExport struct {
	// OpenCodeSessionID is the exported session's pinned id.
	OpenCodeSessionID string `json:"opencode_session_id"`
	// SchemaVersion names the export format (runtime + projection).
	SchemaVersion string `json:"schema_version"`
	// SkillDigests are the skill digests the runtime associates with the
	// session; the manifest records them verbatim.
	SkillDigests []string `json:"skill_digests"`
	// Records is the full message chain in export order.
	Records []SessionRecord `json:"records"`
}

// EncodeSessionExport serializes one export into its canonical stored form.
func EncodeSessionExport(e SessionExport) ([]byte, error) {
	if e.Records == nil {
		e.Records = []SessionRecord{}
	}
	if e.SkillDigests == nil {
		e.SkillDigests = []string{}
	}
	return json.Marshal(e)
}

// DecodeSessionExport parses canonical stored bytes back into an export.
func DecodeSessionExport(raw []byte) (SessionExport, error) {
	var e SessionExport
	if err := json.Unmarshal(raw, &e); err != nil {
		return SessionExport{}, fmt.Errorf("%w: decode session export: %v", ErrInvalidInput, err)
	}
	return e, nil
}

// SnapshotManifest records the runtime, schema and skill summary of one
// capture: the facts a restore compares the world against before any state
// is materialized.
type SnapshotManifest struct {
	Version         string   `json:"version"`
	RuntimeDigest   string   `json:"runtime_digest"`
	SchemaVersion   string   `json:"schema_version"`
	SkillDigests    []string `json:"skill_digests"`
	OpenCodeSession string   `json:"opencode_session_id"`
	Records         int      `json:"records"`
}

// SnapshotObject is one durable object a snapshot pins, with its per-object
// checksum. File objects reuse the version's immutable storage objects;
// session objects are the exported OpenCode data uploaded by the capture.
// Names are canonical output-relative paths, so credential-shaped names
// (".env", "*.key", "secrets/*", …) are refused before any byte moves.
type SnapshotObject struct {
	Kind, Name, Ref, SHA256 string
	Bytes                   int64
}

// StoredSnapshot is the persisted snapshot entity: the identity, the
// manifest and every pinned object.
type StoredSnapshot struct {
	Snapshot
	ID        string
	Manifest  SnapshotManifest
	Objects   []SnapshotObject
	CreatedAt time.Time
}

// ValidateSnapshotObject checks one object reference: a known kind, a
// canonical non-credential name, a well-formed digest and a real size.
func ValidateSnapshotObject(o SnapshotObject) error {
	switch o.Kind {
	case SnapshotObjectFile, SnapshotObjectSession:
	default:
		return fmt.Errorf("%w: snapshot object kind %q", ErrInvalidInput, o.Kind)
	}
	if err := ValidateArtifactPath(o.Name); err != nil {
		return err
	}
	if !ValidSHA256(o.SHA256) {
		return fmt.Errorf("%w: snapshot object %s has malformed digest %q", ErrInvalidInput, o.Name, o.SHA256)
	}
	if o.Ref == "" {
		return fmt.Errorf("%w: snapshot object %s has no storage ref", ErrInvalidInput, o.Name)
	}
	if o.Bytes < 0 {
		return fmt.Errorf("%w: snapshot object %s has negative size", ErrInvalidInput, o.Name)
	}
	return nil
}

// ValidateStoredSnapshot validates the caller-independent invariants and
// derives the canonical row identity: complete identity fields, well-formed
// digests, a manifest pinned to this manifest version and runtime, every
// object reference valid, and an id that equals the derived SnapshotID (an
// empty id is filled in).
func ValidateStoredSnapshot(in StoredSnapshot) (StoredSnapshot, error) {
	if in.WorkspaceID == "" || in.VersionID == "" || in.RuntimeDigest == "" {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot requires workspace, version and runtime digest", ErrInvalidInput)
	}
	if !ValidSHA256(in.FilesDigest) {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot files digest %q", ErrInvalidInput, in.FilesDigest)
	}
	if !ValidSHA256(in.SessionDigest) {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot session digest %q", ErrInvalidInput, in.SessionDigest)
	}
	if !in.Quiescent {
		return StoredSnapshot{}, fmt.Errorf("%w: only a quiescent capture may be persisted", ErrInvalidInput)
	}
	if in.Manifest.Version != SnapshotManifestVersion {
		return StoredSnapshot{}, fmt.Errorf("%w: manifest version %q is not %q", ErrInvalidInput, in.Manifest.Version, SnapshotManifestVersion)
	}
	if in.Manifest.RuntimeDigest != in.RuntimeDigest {
		return StoredSnapshot{}, fmt.Errorf("%w: manifest runtime %q disagrees with snapshot runtime %q", ErrInvalidInput, in.Manifest.RuntimeDigest, in.RuntimeDigest)
	}
	if in.ID == "" {
		in.ID = SnapshotID(in.Snapshot)
	}
	if in.ID != SnapshotID(in.Snapshot) {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot id %q does not match its content identity", ErrInvalidInput, in.ID)
	}
	if len(in.Objects) == 0 {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot pins no objects", ErrInvalidInput)
	}
	seen := make(map[string]bool, len(in.Objects))
	hasSession := false
	for _, o := range in.Objects {
		if err := ValidateSnapshotObject(o); err != nil {
			return StoredSnapshot{}, err
		}
		key := o.Kind + "\x00" + o.Name
		if seen[key] {
			return StoredSnapshot{}, fmt.Errorf("%w: duplicate snapshot object %s %s", ErrInvalidInput, o.Kind, o.Name)
		}
		seen[key] = true
		if o.Kind == SnapshotObjectSession {
			hasSession = true
		}
	}
	if !hasSession {
		return StoredSnapshot{}, fmt.Errorf("%w: snapshot carries no OpenCode session data", ErrInvalidInput)
	}
	return in, nil
}

// SnapshotStore persists immutable recovery snapshots. Putting is idempotent
// by logical identity: the same quiescent state always answers the same
// stored row, and a row that was once stored never changes its objects or
// manifest afterwards.
type SnapshotStore interface {
	// Put records one snapshot for the scope's workspace after every object
	// it references was durably uploaded. Repeating the identical put
	// returns the stored snapshot with the same id.
	Put(ctx context.Context, scope Scope, snap StoredSnapshot) (StoredSnapshot, error)
	// Get returns one stored snapshot in the requesting scope.
	Get(ctx context.Context, scope Scope, id string) (StoredSnapshot, error)
	// ListByWorkspace returns the scope's workspace snapshots, newest first.
	ListByWorkspace(ctx context.Context, scope Scope) ([]StoredSnapshot, error)
}
