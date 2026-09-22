package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftSnapshotStore persists immutable Craft recovery snapshots (C05) in
// the migrated business database. One row per content identity carries the
// quiescent capture's manifest; craft_snapshot_objects pins every object it
// references with the per-object checksum the capture recorded. Putting is
// idempotent by logical identity: the same quiescent state always answers
// the same row, and rows are never updated after their first store — a
// later capture derives a new identity, so an old snapshot's objects keep
// resolving exactly as they were pinned (W01 semantics).
type CraftSnapshotStore struct {
	db *gorm.DB
}

var _ craft.SnapshotStore = (*CraftSnapshotStore)(nil)

// NewCraftSnapshotStore constructs the craft.SnapshotStore implementation
// backed by the migrated business database.
func NewCraftSnapshotStore(db *gorm.DB) craft.SnapshotStore {
	return &CraftSnapshotStore{db: db}
}

type craftSnapshotRow struct {
	ID              string    `gorm:"column:id"`
	TenantID        uint64    `gorm:"column:tenant_id"`
	WorkspaceID     string    `gorm:"column:workspace_id"`
	VersionID       string    `gorm:"column:version_id"`
	FilesDigest     string    `gorm:"column:files_digest"`
	SessionDigest   string    `gorm:"column:session_digest"`
	RuntimeDigest   string    `gorm:"column:runtime_digest"`
	Quiescent       bool      `gorm:"column:quiescent"`
	ManifestVersion string    `gorm:"column:manifest_version"`
	ManifestJSON    string    `gorm:"column:manifest_json"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (craftSnapshotRow) TableName() string { return "craft_snapshots" }

type craftSnapshotObjectRow struct {
	SnapshotID  string    `gorm:"column:snapshot_id"`
	Kind        string    `gorm:"column:kind"`
	Name        string    `gorm:"column:name"`
	ResourceRef string    `gorm:"column:resource_ref"`
	SHA256      string    `gorm:"column:sha256"`
	Bytes       int64     `gorm:"column:bytes"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (craftSnapshotObjectRow) TableName() string { return "craft_snapshot_objects" }

// authorizeCraftSnapshot answers whether the scope may read one snapshot
// row: a cross-tenant or cross-session workspace does not exist for the
// caller, another user's binding is visible but forbidden — the same ACL
// shape as the version store.
func authorizeCraftSnapshot(db *gorm.DB, row craftSnapshotRow, scope craft.Scope) error {
	if row.TenantID != scope.TenantID {
		return craft.ErrNotFound
	}
	var ws craftWorkspaceRow
	err := db.Where("id = ?", row.WorkspaceID).Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if ws.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if ws.OwnerID != scope.UserID {
		return fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	return nil
}

// loadCraftSnapshot reads one snapshot with its pinned objects, objects
// ordered by kind and name so the persisted shape is canonical.
func loadCraftSnapshot(db *gorm.DB, id string) (craft.StoredSnapshot, error) {
	var row craftSnapshotRow
	err := db.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.StoredSnapshot{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.StoredSnapshot{}, err
	}
	var objectRows []craftSnapshotObjectRow
	if err := db.Where("snapshot_id = ?", id).Order("kind ASC, name ASC").Find(&objectRows).Error; err != nil {
		return craft.StoredSnapshot{}, err
	}
	objects := make([]craft.SnapshotObject, 0, len(objectRows))
	for _, o := range objectRows {
		objects = append(objects, craft.SnapshotObject{
			Kind: o.Kind, Name: o.Name, Ref: o.ResourceRef,
			SHA256: o.SHA256, Bytes: o.Bytes,
		})
	}
	var manifest craft.SnapshotManifest
	if err := json.Unmarshal([]byte(row.ManifestJSON), &manifest); err != nil {
		return craft.StoredSnapshot{}, fmt.Errorf("craft: decode snapshot %s manifest: %w", id, err)
	}
	return craft.StoredSnapshot{
		Snapshot: craft.Snapshot{
			WorkspaceID: row.WorkspaceID, VersionID: row.VersionID,
			FilesDigest: row.FilesDigest, SessionDigest: row.SessionDigest,
			RuntimeDigest: row.RuntimeDigest, Quiescent: row.Quiescent,
		},
		ID: row.ID, Manifest: manifest, Objects: objects, CreatedAt: row.CreatedAt,
	}, nil
}

// sameCraftSnapshot reports store identity: same derived id, manifest and
// object set. Only the identical replay may adopt a stored row; anything
// else is a conflict.
func sameCraftSnapshot(a, b craft.StoredSnapshot) bool {
	if a.ID != b.ID || a.Snapshot != b.Snapshot || len(a.Objects) != len(b.Objects) {
		return false
	}
	if !sameCraftSnapshotManifest(a.Manifest, b.Manifest) {
		return false
	}
	for i := range a.Objects {
		if a.Objects[i] != b.Objects[i] {
			return false
		}
	}
	return true
}

// sameCraftSnapshotManifest compares manifests element-wise (the skill
// digest slice is not struct-comparable).
func sameCraftSnapshotManifest(a, b craft.SnapshotManifest) bool {
	if a.Version != b.Version || a.RuntimeDigest != b.RuntimeDigest ||
		a.SchemaVersion != b.SchemaVersion || a.OpenCodeSession != b.OpenCodeSession ||
		a.Records != b.Records || len(a.SkillDigests) != len(b.SkillDigests) {
		return false
	}
	for i := range a.SkillDigests {
		if a.SkillDigests[i] != b.SkillDigests[i] {
			return false
		}
	}
	return true
}

// Put records one immutable snapshot. The workspace must belong to the
// storing scope; every object the snapshot pins must already be durably
// uploaded (the capture uploads first, then stores in one transaction). A
// racing or replayed identical put adopts the stored row and answers it with
// the same id; a different snapshot under the same identity is a conflict.
func (s *CraftSnapshotStore) Put(ctx context.Context, scope craft.Scope, in craft.StoredSnapshot) (craft.StoredSnapshot, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.StoredSnapshot{}, fmt.Errorf("%w: incomplete snapshot scope", craft.ErrInvalidInput)
	}
	in, err := craft.ValidateStoredSnapshot(in)
	if err != nil {
		return craft.StoredSnapshot{}, err
	}
	manifestJSON, err := json.Marshal(in.Manifest)
	if err != nil {
		return craft.StoredSnapshot{}, err
	}
	var out craft.StoredSnapshot
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ws craftWorkspaceRow
		e := tx.Where("id = ?", in.WorkspaceID).Take(&ws).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: workspace %s", craft.ErrNotFound, in.WorkspaceID)
		}
		if e != nil {
			return e
		}
		if ws.TenantID != scope.TenantID || ws.SessionID != scope.SessionID {
			return fmt.Errorf("%w: workspace %s is bound to another session", craft.ErrNotFound, in.WorkspaceID)
		}
		if ws.OwnerID != scope.UserID {
			return fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
		}

		// created_at is written by the store, not the database default: the
		// SQL CURRENT_TIMESTAMP defaults have whole-second precision, which
		// would order two same-second snapshots arbitrarily. The Go clock's
		// sub-second precision keeps "newest first" deterministic.
		row := craftSnapshotRow{
			ID: in.ID, TenantID: scope.TenantID, WorkspaceID: in.WorkspaceID,
			VersionID: in.VersionID, FilesDigest: in.FilesDigest,
			SessionDigest: in.SessionDigest, RuntimeDigest: in.RuntimeDigest,
			Quiescent: in.Quiescent, ManifestVersion: in.Manifest.Version,
			ManifestJSON: string(manifestJSON), CreatedAt: time.Now().UTC(),
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			objectRows := make([]craftSnapshotObjectRow, 0, len(in.Objects))
			for _, o := range in.Objects {
				objectRows = append(objectRows, craftSnapshotObjectRow{
					SnapshotID: in.ID, Kind: o.Kind, Name: o.Name,
					ResourceRef: o.Ref, SHA256: o.SHA256, Bytes: o.Bytes,
					CreatedAt: row.CreatedAt,
				})
			}
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&objectRows).Error; e != nil {
				return e
			}
			out = in
			out.CreatedAt = row.CreatedAt
			return nil
		}

		// Lost the identity race (or this is a replay): only the identical
		// snapshot may adopt the stored row.
		stored, e := loadCraftSnapshot(tx, in.ID)
		if e != nil {
			return e
		}
		if !sameCraftSnapshot(stored, in) {
			return fmt.Errorf("%w: snapshot %s already stored with different content", craft.ErrConflict, in.ID)
		}
		out = stored
		return nil
	})
	if err != nil {
		return craft.StoredSnapshot{}, err
	}
	return out, nil
}

// Get returns one stored snapshot in the requesting scope.
func (s *CraftSnapshotStore) Get(ctx context.Context, scope craft.Scope, id string) (craft.StoredSnapshot, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || id == "" {
		return craft.StoredSnapshot{}, fmt.Errorf("%w: incomplete snapshot request", craft.ErrInvalidInput)
	}
	var row craftSnapshotRow
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.StoredSnapshot{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.StoredSnapshot{}, err
	}
	if e := authorizeCraftSnapshot(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.StoredSnapshot{}, e
	}
	return loadCraftSnapshot(s.db.WithContext(ctx), id)
}

// ListByWorkspace returns the scope's workspace snapshots, newest first. A
// session without a workspace binding has nothing to list.
func (s *CraftSnapshotStore) ListByWorkspace(ctx context.Context, scope craft.Scope) ([]craft.StoredSnapshot, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return nil, fmt.Errorf("%w: incomplete snapshot scope", craft.ErrInvalidInput)
	}
	var ws craftWorkspaceRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).
		Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, craft.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ws.OwnerID != scope.UserID {
		return nil, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	var rows []craftSnapshotRow
	err = s.db.WithContext(ctx).Where("workspace_id = ?", ws.ID).
		Order("created_at DESC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	snapshots := make([]craft.StoredSnapshot, 0, len(rows))
	for _, row := range rows {
		stored, e := loadCraftSnapshot(s.db.WithContext(ctx), row.ID)
		if e != nil {
			return nil, e
		}
		snapshots = append(snapshots, stored)
	}
	return snapshots, nil
}
