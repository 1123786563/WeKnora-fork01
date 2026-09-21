// Package service — Craft recovery snapshots (C05).
//
// CraftSnapshotService captures one complete recovery snapshot of a craft
// workspace — the version's immutable files TOGETHER with the OpenCode
// session state — and restores it into a new sandbox generation. Capture
// only runs while the runtime is verifiably quiescent (OC idle, no pending
// or unknown delegation) under the workspace lifecycle lock; restore never
// executes over an active run, verifies every pinned object checksum and
// the live message chain in a fresh generation first, and swaps the binding
// with one compare-and-swap so a process death mid-restore always leaves
// the previous binding untouched.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// craftSessionExportObjectPath is the canonical object name of the exported
// OpenCode session data inside a snapshot (ValidateArtifactPath-checked, so
// credential-shaped names can never sneak past the same filter the artifact
// collector applies).
const craftSessionExportObjectPath = "session/export-v1.json"

// CraftSnapshotSource moves the isolated execution state of one craft
// session between the live runtime and snapshot objects. The production
// implementation rides the W06 local runtime (craft_runtime.go); tests drive
// fakes at this seam.
type CraftSnapshotSource interface {
	// Quiescent reports whether the bound OpenCode session is verifiably
	// idle with no pending tool round. The reason names what is still
	// running when the answer is false.
	Quiescent(ctx context.Context, ws craft.Workspace) (ok bool, reason string, err error)
	// ExportSessionData exports the OpenCode persistent data of the bound
	// session. Reading through the owning process IS the consistent backup:
	// it never copies a SQLite file mid-write or a half-written WAL.
	ExportSessionData(ctx context.Context, ws craft.Workspace) (craft.SessionExport, error)
	// RestoreGeneration materializes the version files and verifies the
	// OpenCode session in a NEW sandbox generation: every file digest is
	// checked against the manifest, the session must exist, and its live
	// message chain must still match the export. It returns the healthy
	// candidate binding; nothing is live until the caller's CAS wins.
	RestoreGeneration(ctx context.Context, ws craft.Workspace, files []craft.File, export craft.SessionExport) (craft.Workspace, error)
	// ReleaseGeneration cleans a candidate generation that never won the
	// binding CAS.
	ReleaseGeneration(ctx context.Context, candidate craft.Workspace) error
	// IsolatedDataRestore reports whether this provider can restore the
	// OpenCode persistent data isolated per session. Providers without that
	// capability leave restore explicitly unsupported — never a files-only
	// substitution dressed up as a recovery.
	IsolatedDataRestore() bool
}

// CraftWorkspaceLock serializes workspace lifecycle transitions per session
// (the R03 lifecycle-lock contract). The process implementation serializes
// the single WeKnora process; the production containerized deployment swaps
// the shared store lock in at assembly.
type CraftWorkspaceLock interface {
	WithLifecycleLock(ctx context.Context, key sandbox.SessionSandboxKey, fn func(context.Context) error) error
}

// craftWorkspaceLockFromBindings adapts the R03 binding store lock.
type craftWorkspaceLockFromBindings struct {
	store sandbox.SessionSandboxBindingStore
}

// CraftWorkspaceLockFromBindingStore adapts the shared session sandbox
// binding store's lifecycle lock onto the snapshot service's lock port.
func CraftWorkspaceLockFromBindingStore(store sandbox.SessionSandboxBindingStore) CraftWorkspaceLock {
	return craftWorkspaceLockFromBindings{store: store}
}

func (l craftWorkspaceLockFromBindings) WithLifecycleLock(ctx context.Context, key sandbox.SessionSandboxKey, fn func(context.Context) error) error {
	return l.store.WithLifecycleLock(ctx, key, fn)
}

// processCraftWorkspaceLock is the single-process lifecycle lock: sessions
// are sharded by key so unrelated sessions never queue behind each other.
type processCraftWorkspaceLock struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewProcessCraftWorkspaceLock returns the process-local lifecycle lock the
// single-process local runtime deployment uses.
func NewProcessCraftWorkspaceLock() CraftWorkspaceLock {
	return &processCraftWorkspaceLock{locks: map[string]*sync.Mutex{}}
}

func (l *processCraftWorkspaceLock) WithLifecycleLock(ctx context.Context, key sandbox.SessionSandboxKey, fn func(context.Context) error) error {
	id := fmt.Sprintf("%d/%s", key.TenantID, key.SessionID)
	l.mu.Lock()
	lock, ok := l.locks[id]
	if !ok {
		lock = &sync.Mutex{}
		l.locks[id] = lock
	}
	l.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return fn(ctx)
}

// CraftSnapshotConfig assembles the snapshot service.
type CraftSnapshotConfig struct {
	// DB is the migrated business database.
	DB *gorm.DB
	// Sessions is the existing session service: restore is a write entry,
	// so the owned-session ACL gates it exactly like POST /runs.
	Sessions interfaces.SessionService
	// Store persists the craft workspace binding (R02).
	Store craft.Store
	// Versions is the W01 immutable version store.
	Versions craft.VersionStore
	// Snapshots is the C05 snapshot store.
	Snapshots craft.SnapshotStore
	// Files reads stored objects (session export upload + restore verify).
	Files interfaces.FileService
	// Source moves the isolated execution state.
	Source CraftSnapshotSource
	// Lock serializes workspace lifecycle transitions per session. Nil
	// falls back to the process-local lock.
	Lock CraftWorkspaceLock
	// ActiveRuns is the production active-run check restore refuses over.
	ActiveRuns CraftRunActivity
	// RuntimeDigest is the digest of the runtime this process is configured
	// to trust (W06 env semantics); snapshots from another runtime stay
	// unrestorable.
	RuntimeDigest string
	// RestoreGuard refuses a restore onto a session whose resources are being
	// cleaned up (the O03 lifecycle tombstone). Optional: nil keeps the
	// unguarded behavior for assemblies without the lifecycle service.
	RestoreGuard CraftRestoreGuard
	// Now is injectable for tests.
	Now func() time.Time
}

// CraftRestoreGuard refuses a snapshot restore onto a session under
// teardown: the restore would materialize a sandbox generation the sweeper
// is about to reclaim.
type CraftRestoreGuard interface {
	GuardRestore(ctx context.Context, tenantID uint64, sessionID string) error
}

// CraftSnapshotService captures and restores complete craft recovery
// snapshots.
type CraftSnapshotService struct {
	db            *gorm.DB
	sessions      interfaces.SessionService
	store         craft.Store
	versions      craft.VersionStore
	snapshots     craft.SnapshotStore
	files         interfaces.FileService
	source        CraftSnapshotSource
	lock          CraftWorkspaceLock
	activeRuns    CraftRunActivity
	restoreGuard  CraftRestoreGuard
	runtimeDigest string
	now           func() time.Time
}

// NewCraftSnapshotService validates the assembly and returns the service.
func NewCraftSnapshotService(cfg CraftSnapshotConfig) (*CraftSnapshotService, error) {
	if cfg.DB == nil || cfg.Sessions == nil || cfg.Store == nil || cfg.Versions == nil ||
		cfg.Snapshots == nil || cfg.Files == nil || cfg.Source == nil || cfg.ActiveRuns == nil {
		return nil, errors.New("craft: snapshot service requires db, sessions, store, versions, snapshots, files, source and active-runs")
	}
	if strings.TrimSpace(cfg.RuntimeDigest) == "" {
		return nil, errors.New("craft: snapshot service requires the deployment runtime digest")
	}
	lock := cfg.Lock
	if lock == nil {
		lock = NewProcessCraftWorkspaceLock()
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CraftSnapshotService{
		db: cfg.DB, sessions: cfg.Sessions, store: cfg.Store, versions: cfg.Versions,
		snapshots: cfg.Snapshots, files: cfg.Files, source: cfg.Source, lock: lock,
		activeRuns: cfg.ActiveRuns, restoreGuard: cfg.RestoreGuard,
		runtimeDigest: strings.TrimSpace(cfg.RuntimeDigest), now: now,
	}, nil
}

// RuntimeDigest reports the deployment runtime digest this service compares
// snapshots against.
func (s *CraftSnapshotService) RuntimeDigest() string { return s.runtimeDigest }

// SetRestoreGuard installs the lifecycle restore guard (O03 integration
// wiring; see internal/container).
func (s *CraftSnapshotService) SetRestoreGuard(g CraftRestoreGuard) { s.restoreGuard = g }

// unfinishedCraftDelegations counts the workspace's delegations that have no
// persisted terminal result: a pending or outcome-unknown sub-execution
// makes the runtime not quiescent, whatever the OpenCode session reports.
func (s *CraftSnapshotService) unfinishedCraftDelegations(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("craft_delegations").
		Where("workspace_id = ?", workspaceID).
		Where("result_json IS NULL").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Capture records one complete recovery snapshot of the workspace at an
// already published version. The gate is the C05 product constraint: the
// OpenCode runtime must be verifiably idle, no delegation of the workspace
// may be pending or outcome-unknown, and the whole capture runs under the
// workspace lifecycle lock. The snapshot pins the version's immutable file
// objects and uploads the exported OpenCode session data into controlled
// storage with a per-object checksum; the manifest records the runtime,
// schema and skill summary.
func (s *CraftSnapshotService) Capture(ctx context.Context, scope craft.Scope, versionID string) (craft.Snapshot, error) {
	if s == nil {
		return craft.Snapshot{}, fmt.Errorf("%w: snapshot service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Snapshot{}, fmt.Errorf("%w: incomplete snapshot scope", craft.ErrInvalidInput)
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return craft.Snapshot{}, fmt.Errorf("%w: version id is required", craft.ErrInvalidInput)
	}
	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.Snapshot{}, err
	}
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return craft.Snapshot{}, err
	}
	if version.WorkspaceID != workspace.ID {
		return craft.Snapshot{}, fmt.Errorf("%w: version %s belongs to workspace %s", craft.ErrNotFound, version.ID, version.WorkspaceID)
	}
	filesDigest, err := craft.ManifestDigest(version.Files)
	if err != nil {
		return craft.Snapshot{}, err
	}

	var stored craft.StoredSnapshot
	key := sandbox.SessionSandboxKey{TenantID: scope.TenantID, SessionID: scope.SessionID}
	lockErr := s.lock.WithLifecycleLock(ctx, key, func(lockCtx context.Context) error {
		// Quiescence gate: no pending or outcome-unknown delegation, and a
		// runtime-verified idle OpenCode session.
		pending, perr := s.unfinishedCraftDelegations(lockCtx, workspace.ID)
		if perr != nil {
			return perr
		}
		if pending > 0 {
			return fmt.Errorf("%w: workspace %s still has %d pending or outcome-unknown delegation(s)",
				craft.ErrBusy, workspace.ID, pending)
		}
		idle, reason, qerr := s.source.Quiescent(lockCtx, workspace)
		if qerr != nil {
			return qerr
		}
		if !idle {
			return fmt.Errorf("%w: opencode session %s is not quiescent: %s",
				craft.ErrBusy, workspace.OpenCodeSessionID, reason)
		}

		// Export the OpenCode persistent data through the owning runtime.
		export, eerr := s.source.ExportSessionData(lockCtx, workspace)
		if eerr != nil {
			return eerr
		}
		if export.OpenCodeSessionID == "" || export.OpenCodeSessionID != workspace.OpenCodeSessionID {
			return fmt.Errorf("%w: export names opencode session %q but the workspace binds %q",
				craft.ErrConflict, export.OpenCodeSessionID, workspace.OpenCodeSessionID)
		}
		if verr := craft.ValidSessionChain(export.Records); verr != nil {
			return verr
		}
		sessionDigest, derr := craft.SessionDigest(export.Records)
		if derr != nil {
			return derr
		}

		// Snapshot identity is fixed before the upload so the storage object
		// name carries it (content addressing keeps retries de-duplicated).
		snap := craft.Snapshot{
			WorkspaceID: workspace.ID, VersionID: version.ID,
			FilesDigest: filesDigest, SessionDigest: sessionDigest,
			RuntimeDigest: workspace.RuntimeDigest, Quiescent: true,
		}
		id := craft.SnapshotID(snap)

		// Upload the session data object first (all-or-nothing object set,
		// same ordering as the W01 collector), then pin every object.
		encoded, merr := craft.EncodeSessionExport(export)
		if merr != nil {
			return merr
		}
		sum := sha256.Sum256(encoded)
		objectDigest := hex.EncodeToString(sum[:])
		ref, uerr := s.files.SaveBytes(lockCtx, encoded, scope.TenantID,
			"craft_snapshot_"+id+"_session.json", false)
		if uerr != nil {
			return fmt.Errorf("craft: upload session export: %w", uerr)
		}

		objects := make([]craft.SnapshotObject, 0, len(version.Files)+1)
		for _, f := range version.Files {
			objects = append(objects, craft.SnapshotObject{
				Kind: craft.SnapshotObjectFile, Name: f.Path, Ref: f.Ref,
				SHA256: f.SHA256, Bytes: f.Bytes,
			})
		}
		objects = append(objects, craft.SnapshotObject{
			Kind: craft.SnapshotObjectSession, Name: craftSessionExportObjectPath,
			Ref: ref, SHA256: objectDigest, Bytes: int64(len(encoded)),
		})

		candidate := craft.StoredSnapshot{
			Snapshot: snap, ID: id,
			Manifest: craft.SnapshotManifest{
				Version: craft.SnapshotManifestVersion, RuntimeDigest: workspace.RuntimeDigest,
				SchemaVersion: export.SchemaVersion, SkillDigests: export.SkillDigests,
				OpenCodeSession: export.OpenCodeSessionID, Records: len(export.Records),
			},
			Objects: objects,
		}
		stored, err = s.snapshots.Put(lockCtx, scope, candidate)
		if err != nil {
			return err
		}
		logger.Infof(lockCtx, "[CraftSnapshot] captured %s workspace %s version %s (%d files, %d records)",
			stored.ID, workspace.ID, version.ID, len(version.Files), len(export.Records))
		return nil
	})
	if lockErr != nil {
		return craft.Snapshot{}, lockErr
	}
	return stored.Snapshot, nil
}

// verifySnapshotObjects re-reads every pinned session object, checks its
// per-object checksum, decodes the canonical export and re-derives the
// session digest: a corrupt or tampered object refuses the restore before
// anything is materialized. File objects are checked against the pinned
// version manifest (same refs, same digests).
func (s *CraftSnapshotService) verifySnapshotObjects(
	ctx context.Context, scope craft.Scope, stored craft.StoredSnapshot,
) (craft.SessionExport, craft.Version, error) {
	version, err := s.versions.Get(ctx, scope, stored.VersionID)
	if err != nil {
		return craft.SessionExport{}, craft.Version{}, err
	}
	if version.WorkspaceID != stored.WorkspaceID {
		return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: version %s belongs to workspace %s",
			craft.ErrNotFound, version.ID, version.WorkspaceID)
	}
	filesDigest, err := craft.ManifestDigest(version.Files)
	if err != nil {
		return craft.SessionExport{}, craft.Version{}, err
	}
	if filesDigest != stored.FilesDigest {
		return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: version manifest digest %s does not match the snapshot pin %s",
			craft.ErrConflict, filesDigest, stored.FilesDigest)
	}
	byPath := map[string]craft.File{}
	for _, f := range version.Files {
		byPath[f.Path] = f
	}

	var export craft.SessionExport
	sessionObjects := 0
	for _, o := range stored.Objects {
		switch o.Kind {
		case craft.SnapshotObjectFile:
			f, ok := byPath[o.Name]
			if !ok {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: snapshot pins file %q the version manifest does not carry",
					craft.ErrConflict, o.Name)
			}
			if f.Ref != o.Ref || f.SHA256 != o.SHA256 || f.Bytes != o.Bytes {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: snapshot file pin %q disagrees with the version manifest",
					craft.ErrConflict, o.Name)
			}
		case craft.SnapshotObjectSession:
			reader, rerr := s.files.GetFile(ctx, o.Ref)
			if rerr != nil {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("craft: read session object %s: %w", o.Name, rerr)
			}
			// The read is bounded by the recorded size plus one byte: an
			// object that grew after its pin must fail its checksum anyway,
			// but the bound keeps a corrupt row from streaming unbounded.
			content, ierr := io.ReadAll(io.LimitReader(reader, o.Bytes+1))
			_ = reader.Close()
			if ierr != nil {
				return craft.SessionExport{}, craft.Version{}, ierr
			}
			if int64(len(content)) != o.Bytes {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: session object %s stores %d bytes, pin says %d",
					craft.ErrConflict, o.Name, len(content), o.Bytes)
			}
			sum := sha256.Sum256(content)
			if hex.EncodeToString(sum[:]) != o.SHA256 {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: session object %s fails its checksum",
					craft.ErrConflict, o.Name)
			}
			parsed, perr := craft.DecodeSessionExport(content)
			if perr != nil {
				return craft.SessionExport{}, craft.Version{}, perr
			}
			if verr := craft.ValidSessionChain(parsed.Records); verr != nil {
				return craft.SessionExport{}, craft.Version{}, verr
			}
			digest, derr := craft.SessionDigest(parsed.Records)
			if derr != nil {
				return craft.SessionExport{}, craft.Version{}, derr
			}
			if digest != stored.SessionDigest {
				return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: session object digest %s does not match the snapshot pin %s",
					craft.ErrConflict, digest, stored.SessionDigest)
			}
			export = parsed
			sessionObjects++
		default:
			return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: unknown snapshot object kind %q", craft.ErrConflict, o.Kind)
		}
	}
	if sessionObjects == 0 {
		return craft.SessionExport{}, craft.Version{}, fmt.Errorf("%w: snapshot %s carries no OpenCode session data",
			craft.ErrConflict, stored.ID)
	}
	return export, version, nil
}

// Restore rebinds the workspace to a new sandbox generation materialized
// from the snapshot: the version's files and the OpenCode session state of
// exactly that version. Refusals are structural: an active (queued/running/
// recovering/waiting_user) run, a revision race, a provider without isolated
// data restore, a runtime the deployment no longer trusts, a corrupt object
// or a session chain that moved past the snapshot. The previous binding
// stays live until the materialized generation is verified healthy and the
// one compare-and-swap wins — a process death at any earlier point leaves
// the old binding untouched (never a half restore).
func (s *CraftSnapshotService) Restore(ctx context.Context, scope craft.Scope, snapshotID string, revision int64) (craft.Workspace, error) {
	started := time.Now()
	defer func() { metrics.ObserveCraftWorkspaceRestore(time.Since(started)) }()
	if s == nil {
		return craft.Workspace{}, fmt.Errorf("%w: snapshot service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Workspace{}, fmt.Errorf("%w: incomplete restore scope", craft.ErrInvalidInput)
	}
	// O03 wiring: a session under teardown refuses restores — the durable
	// deleting mark blocks materializing a generation the sweeper is about
	// to reclaim, even before the lifecycle lock is taken.
	if s.restoreGuard != nil {
		if gerr := s.restoreGuard.GuardRestore(ctx, scope.TenantID, scope.SessionID); gerr != nil {
			return craft.Workspace{}, gerr
		}
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if !strings.HasPrefix(snapshotID, craft.SnapshotIDPrefix) {
		return craft.Workspace{}, fmt.Errorf("%w: malformed snapshot id %q", craft.ErrInvalidInput, snapshotID)
	}
	if revision <= 0 {
		return craft.Workspace{}, fmt.Errorf("%w: restore requires the workspace revision being restored", craft.ErrInvalidInput)
	}

	// Write ACL: the session must be owned by the caller and stay a craft
	// tRPC session, exactly like the other craft write entries.
	session, err := writeSessionOf(ctx, s.sessions, scope, scope.SessionID)
	if err != nil {
		return craft.Workspace{}, err
	}
	owner := ownerScopeOf(session)

	workspace, err := s.store.GetWorkspace(ctx, owner)
	if err != nil {
		return craft.Workspace{}, err
	}
	if workspace.Revision != revision {
		return craft.Workspace{}, fmt.Errorf("%w: workspace revision moved to %d (restore was prepared at %d); reload and retry",
			craft.ErrConflict, workspace.Revision, revision)
	}
	stored, err := s.snapshots.Get(ctx, owner, snapshotID)
	if err != nil {
		return craft.Workspace{}, err
	}
	if stored.WorkspaceID != workspace.ID {
		return craft.Workspace{}, fmt.Errorf("%w: snapshot %s belongs to workspace %s",
			craft.ErrForbidden, stored.ID, stored.WorkspaceID)
	}
	// Provider capability first: without isolated data restore there is no
	// honest recovery, and a files-only substitution is never offered.
	if !s.source.IsolatedDataRestore() {
		return craft.Workspace{}, fmt.Errorf("%w: this provider cannot restore the opencode session data isolated per session",
			craft.ErrUnsupported)
	}
	// Runtime comparability: a snapshot from a replaced runtime never
	// restores onto the current one.
	if stored.RuntimeDigest != s.runtimeDigest {
		return craft.Workspace{}, fmt.Errorf("%w: snapshot runtime %q is not the configured runtime %q; the old runtime is unavailable for restore",
			craft.ErrUnsupported, stored.RuntimeDigest, s.runtimeDigest)
	}
	if !craft.CanRestore(stored.Snapshot, false, s.runtimeDigest) {
		return craft.Workspace{}, fmt.Errorf("%w: snapshot %s is not a restorable quiescent capture", craft.ErrConflict, stored.ID)
	}

	key := sandbox.SessionSandboxKey{TenantID: owner.TenantID, SessionID: owner.SessionID}
	var restored craft.Workspace
	lockErr := s.lock.WithLifecycleLock(ctx, key, func(lockCtx context.Context) error {
		// Re-read under the lock: the binding may have moved while this
		// restore waited for the lifecycle lock.
		current, gerr := s.store.GetWorkspace(lockCtx, owner)
		if gerr != nil {
			return gerr
		}
		if current.Revision != revision {
			return fmt.Errorf("%w: workspace revision moved to %d (restore was prepared at %d); reload and retry",
				craft.ErrConflict, current.Revision, revision)
		}
		if active, aerr := s.activeRuns(lockCtx, owner); aerr != nil {
			return aerr
		} else if active {
			return fmt.Errorf("%w: session %s has an active run; restore refuses to swap the workspace under it",
				craft.ErrBusy, owner.SessionID)
		}

		// Verify every pinned object and the version manifest before any
		// state is materialized (corrupt objects refuse the restore).
		export, version, verr := s.verifySnapshotObjects(lockCtx, owner, stored)
		if verr != nil {
			return verr
		}

		// Materialize the new generation and verify it (file digests, OC
		// session existence, message chain) — nothing is live yet.
		candidate, rerr := s.source.RestoreGeneration(lockCtx, current, version.Files, export)
		if rerr != nil {
			return rerr
		}
		if candidate.Generation == "" || candidate.Generation == current.Generation {
			_ = s.source.ReleaseGeneration(lockCtx, candidate)
			return fmt.Errorf("%w: restore must produce a new sandbox generation", craft.ErrConflict)
		}
		if candidate.OpenCodeSessionID == "" {
			_ = s.source.ReleaseGeneration(lockCtx, candidate)
			return fmt.Errorf("%w: restored generation carries no opencode session", craft.ErrConflict)
		}

		// The single compare-and-swap: the old binding stays bound until this
		// wins, and a lost CAS cleans the candidate and keeps the old binding.
		next := candidate
		next.ID = current.ID
		next.Scope = current.Scope
		next.RuntimeDigest = current.RuntimeDigest
		swapped, perr := s.store.PutWorkspace(lockCtx, next, current.Revision)
		if perr != nil {
			_ = s.source.ReleaseGeneration(lockCtx, candidate)
			return perr
		}
		logger.Infof(lockCtx, "[CraftSnapshot] restored %s onto generation %s (parent version %s)",
			stored.ID, swapped.Generation, stored.VersionID)
		restored = swapped
		return nil
	})
	if lockErr != nil {
		return craft.Workspace{}, lockErr
	}
	return restored, nil
}

// ListSnapshots returns the workspace's stored snapshots, newest first. The
// session read ACL gates the listing exactly like the workspace view: an
// invisible session stays ErrNotFound, another user's binding is a visible
// but forbidden ErrForbidden.
func (s *CraftSnapshotService) ListSnapshots(ctx context.Context, scope craft.Scope) ([]craft.StoredSnapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: snapshot service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return nil, fmt.Errorf("%w: incomplete snapshot scope", craft.ErrInvalidInput)
	}
	if _, err := readSessionOf(ctx, s.sessions, scope, scope.SessionID); err != nil {
		return nil, err
	}
	return s.snapshots.ListByWorkspace(ctx, scope)
}

// readSessionOf is the read-side session gate (the craft session service's
// readSession semantics): the existing read ACL decides visibility, the
// tenant must match.
func readSessionOf(ctx context.Context, sessions interfaces.SessionService, scope craft.Scope, sessionID string) (*types.Session, error) {
	if scope.TenantID == 0 || scope.UserID == "" || sessionID == "" {
		return nil, fmt.Errorf("%w: incomplete craft scope", craft.ErrInvalidInput)
	}
	session, err := sessions.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, craft.ErrNotFound
	}
	if session.TenantID != scope.TenantID {
		return nil, craft.ErrNotFound
	}
	return session, nil
}

// CraftRestoreRequest is the POST /restore body.
type CraftRestoreRequest struct {
	RequestID  string
	SnapshotID string
	Revision   int64
}

// CraftRestoreOutcome is the answer of one (possibly replayed) restore
// request: the live binding after the restore and the snapshot it came
// from. Replayed marks an idempotent retry that did not execute again.
type CraftRestoreOutcome struct {
	Workspace craft.Workspace
	Snapshot  craft.StoredSnapshot
	Replayed  bool
}

// craftRestoreHash binds the restore idempotency key to its parameters: a
// retried key with a different snapshot or revision is a conflict.
func craftRestoreHash(snapshotID string, revision int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-restore-v1\x00%s\x00%d", snapshotID, revision)))
	return hex.EncodeToString(sum[:])
}

// RestoreIdempotent executes one restore under the request_id idempotency
// key (craft_session_requests, purpose "restore"): a retried key with the
// same parameters replays without re-executing, the same key with different
// parameters conflicts, and a failed restore leaves the key free so the
// retry really retries. The reservation is written before execution and
// removed on failure; the revision CAS in Restore is the authority that a
// crashed-between-reservation-and-execution retry can never apply twice.
func (s *CraftSnapshotService) RestoreIdempotent(ctx context.Context, scope craft.Scope, req CraftRestoreRequest) (CraftRestoreOutcome, error) {
	if strings.TrimSpace(req.RequestID) == "" {
		return CraftRestoreOutcome{}, fmt.Errorf("%w: request_id is required", craft.ErrInvalidInput)
	}
	if s == nil {
		return CraftRestoreOutcome{}, fmt.Errorf("%w: snapshot service is not assembled", craft.ErrInvalidInput)
	}
	wantHash := craftRestoreHash(req.SnapshotID, req.Revision)

	// Replay / conflict / reserve, atomically on the purpose-row key.
	var replayed bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing craftSessionRequestRow
		e := tx.Where("tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ?",
			scope.TenantID, scope.UserID, "restore", strings.TrimSpace(req.RequestID)).Take(&existing).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			row := craftSessionRequestRow{
				TenantID: scope.TenantID, UserID: scope.UserID, Purpose: "restore",
				RequestID: strings.TrimSpace(req.RequestID), RequestHash: wantHash,
				SessionID: scope.SessionID, CreatedAt: s.now(),
			}
			return tx.Create(&row).Error
		}
		if e != nil {
			return e
		}
		if existing.RequestHash != wantHash {
			return fmt.Errorf("%w: restore request %s was already issued with different parameters",
				craft.ErrConflict, req.RequestID)
		}
		replayed = true
		return nil
	})
	if err != nil {
		return CraftRestoreOutcome{}, err
	}
	if replayed {
		stored, gerr := s.snapshots.Get(ctx, scope, strings.TrimSpace(req.SnapshotID))
		if gerr != nil {
			return CraftRestoreOutcome{}, gerr
		}
		workspace, werr := s.store.GetWorkspace(ctx, scope)
		if werr != nil {
			return CraftRestoreOutcome{}, werr
		}
		return CraftRestoreOutcome{Workspace: workspace, Snapshot: stored, Replayed: true}, nil
	}

	workspace, rerr := s.Restore(ctx, scope, req.SnapshotID, req.Revision)
	if rerr != nil {
		// Free the key so a genuine retry retries; a crash before this
		// cleanup leaves a reservation whose replay path answers from
		// current state and never re-executes blindly.
		derr := s.db.WithContext(context.WithoutCancel(ctx)).
			Where("tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ? AND request_hash = ?",
				scope.TenantID, scope.UserID, "restore", strings.TrimSpace(req.RequestID), wantHash).
			Delete(&craftSessionRequestRow{}).Error
		if derr != nil {
			logger.Warnf(ctx, "[CraftSnapshot] could not free restore key %s after failure: %v", req.RequestID, derr)
		}
		return CraftRestoreOutcome{}, rerr
	}
	stored, gerr := s.snapshots.Get(ctx, scope, strings.TrimSpace(req.SnapshotID))
	if gerr != nil {
		return CraftRestoreOutcome{}, gerr
	}
	return CraftRestoreOutcome{Workspace: workspace, Snapshot: stored}, nil
}

// writeSessionOf is the owned-session gate of the craft write entries,
// mirroring the craft session service's writeSession: a readable but not
// owned session answers ErrForbidden, an invisible one stays ErrNotFound,
// and only a tRPC session may be restored onto.
func writeSessionOf(ctx context.Context, sessions interfaces.SessionService, scope craft.Scope, sessionID string) (*types.Session, error) {
	if scope.TenantID == 0 || scope.UserID == "" || sessionID == "" {
		return nil, fmt.Errorf("%w: incomplete craft scope", craft.ErrInvalidInput)
	}
	session, err := sessions.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, craft.ErrNotFound
	}
	if session.TenantID != scope.TenantID {
		return nil, craft.ErrNotFound
	}
	if session.UserID != scope.UserID {
		return nil, fmt.Errorf("%w: session %s belongs to %s", craft.ErrForbidden, sessionID, session.UserID)
	}
	if session.EngineType != string(types.AgentEngineTRPC) {
		return nil, fmt.Errorf("%w: session %s uses engine %q; builtin sessions cannot be restored onto",
			craft.ErrConflict, sessionID, session.EngineType)
	}
	return session, nil
}
