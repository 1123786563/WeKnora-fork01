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

// CraftVersionStore persists immutable Craft artifact versions (W01) in the
// migrated business database. One row per (workspace, run, manifest digest)
// carries the verification facts of that publish; the file manifest pins each
// file's storage ref and content digest. Publishing is idempotent by logical
// identity: the same content under the same run always answers the same
// version row, and rows are never updated after their publish — later
// workspace rounds publish new rows, so old version downloads keep resolving
// to the objects their manifest pinned.
type CraftVersionStore struct {
	db *gorm.DB
}

var _ craft.VersionStore = (*CraftVersionStore)(nil)

// NewCraftVersionStore constructs the craft.VersionStore implementation
// backed by the migrated business database.
func NewCraftVersionStore(db *gorm.DB) craft.VersionStore {
	return &CraftVersionStore{db: db}
}

type craftVersionRow struct {
	ID           string    `gorm:"column:id"`
	TenantID     uint64    `gorm:"column:tenant_id"`
	WorkspaceID  string    `gorm:"column:workspace_id"`
	RunID        string    `gorm:"column:run_id"`
	Kind         string    `gorm:"column:kind"`
	ManifestHash string    `gorm:"column:manifest_hash"`
	ChecksJSON   string    `gorm:"column:checks_json"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (craftVersionRow) TableName() string { return "craft_versions" }

type craftVersionFileRow struct {
	VersionID   string `gorm:"column:version_id"`
	Path        string `gorm:"column:path"`
	ResourceRef string `gorm:"column:resource_ref"`
	FileHash    string `gorm:"column:file_hash"`
	FileBytes   int64  `gorm:"column:file_bytes"`
	MIMEType    string `gorm:"column:mime"`
}

func (craftVersionFileRow) TableName() string { return "craft_version_files" }

// prepareCraftVersion validates the caller-independent invariants and
// derives the canonical identity: complete scope fields are checked by the
// caller, here the version must name its workspace, run and kind, the file
// manifest must be canonical, and a supplied id must equal the derived
// VersionID (an empty id is filled in).
func prepareCraftVersion(in craft.Version) (craft.Version, string, error) {
	if in.WorkspaceID == "" || in.RunID == "" || in.Kind == "" {
		return craft.Version{}, "", fmt.Errorf("%w: version requires workspace, run and kind", craft.ErrInvalidInput)
	}
	digest, err := craft.ManifestDigest(in.Files)
	if err != nil {
		return craft.Version{}, "", err
	}
	if in.ID == "" {
		in.ID = craft.VersionID(in.WorkspaceID, in.RunID, digest)
	}
	if in.ID != craft.VersionID(in.WorkspaceID, in.RunID, digest) {
		return craft.Version{}, "", fmt.Errorf("%w: version id %q does not match its manifest identity", craft.ErrInvalidInput, in.ID)
	}
	if in.Checks == nil {
		in.Checks = []craft.Check{}
	}
	return in, digest, nil
}

// authorizeCraftVersion answers whether the scope may read one version row:
// a cross-tenant or cross-session workspace does not exist for the caller,
// another user's binding is visible but forbidden — the same ACL shape as
// the delegation store.
func authorizeCraftVersion(db *gorm.DB, row craftVersionRow, scope craft.Scope) error {
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

// loadCraftVersion reads one version with its file manifest, files ordered
// by path so the persisted shape is canonical.
func loadCraftVersion(db *gorm.DB, id string) (craft.Version, error) {
	var row craftVersionRow
	err := db.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Version{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Version{}, err
	}
	var fileRows []craftVersionFileRow
	if err := db.Where("version_id = ?", id).Order("path ASC").Find(&fileRows).Error; err != nil {
		return craft.Version{}, err
	}
	files := make([]craft.File, 0, len(fileRows))
	for _, fr := range fileRows {
		files = append(files, craft.File{
			Path: fr.Path, Ref: fr.ResourceRef, SHA256: fr.FileHash,
			MIME: fr.MIMEType, Bytes: fr.FileBytes,
		})
	}
	checks := []craft.Check{}
	if row.ChecksJSON != "" {
		if err := json.Unmarshal([]byte(row.ChecksJSON), &checks); err != nil {
			return craft.Version{}, fmt.Errorf("craft: decode version %s checks: %w", id, err)
		}
	}
	return craft.Version{
		ID: row.ID, WorkspaceID: row.WorkspaceID, RunID: row.RunID,
		Kind: row.Kind, Files: files, Checks: checks,
	}, nil
}

// sameCraftVersion reports publish identity: same derived id and the same
// manifest, checks and kind. Only the identical replay may adopt a stored
// row; anything else is a conflict.
func sameCraftVersion(a, b craft.Version) bool {
	if a.ID != b.ID || a.WorkspaceID != b.WorkspaceID || a.RunID != b.RunID || a.Kind != b.Kind ||
		len(a.Files) != len(b.Files) || len(a.Checks) != len(b.Checks) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			return false
		}
	}
	for i := range a.Checks {
		if a.Checks[i] != b.Checks[i] {
			return false
		}
	}
	return true
}

// Publish records one immutable version. The workspace must belong to the
// publishing scope; file objects are expected to be durably uploaded before
// this call (the artifact service uploads first, then publishes in one
// transaction). A racing or replayed identical publish adopts the stored row
// and answers it with the same id; a different publish under the same
// identity is a conflict.
func (s *CraftVersionStore) Publish(ctx context.Context, scope craft.Scope, in craft.Version) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete version scope", craft.ErrInvalidInput)
	}
	in, digest, err := prepareCraftVersion(in)
	if err != nil {
		return craft.Version{}, err
	}
	encoded, err := json.Marshal(in.Checks)
	if err != nil {
		return craft.Version{}, err
	}
	var out craft.Version
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
		// would order two same-second publishes arbitrarily. The Go clock's
		// sub-second precision keeps "newest first" deterministic.
		row := craftVersionRow{
			ID: in.ID, TenantID: scope.TenantID, WorkspaceID: in.WorkspaceID,
			RunID: in.RunID, Kind: in.Kind, ManifestHash: digest,
			ChecksJSON: string(encoded), CreatedAt: time.Now().UTC(),
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			fileRows := make([]craftVersionFileRow, 0, len(in.Files))
			for _, f := range in.Files {
				fileRows = append(fileRows, craftVersionFileRow{
					VersionID: in.ID, Path: f.Path, ResourceRef: f.Ref,
					FileHash: f.SHA256, FileBytes: f.Bytes, MIMEType: f.MIME,
				})
			}
			if len(fileRows) > 0 {
				if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fileRows).Error; e != nil {
					return e
				}
			}
			out = in
			return nil
		}

		// Lost the identity race (or this is a replay): only the identical
		// publish may adopt the stored version.
		stored, e := loadCraftVersion(tx, in.ID)
		if e != nil {
			return e
		}
		if !sameCraftVersion(stored, in) {
			return fmt.Errorf("%w: version %s already published with different content", craft.ErrConflict, in.ID)
		}
		out = stored
		return nil
	})
	if err != nil {
		return craft.Version{}, err
	}
	return out, nil
}

// List returns the scope's workspace versions, newest first. A session
// without a workspace binding has nothing to list.
func (s *CraftVersionStore) List(ctx context.Context, scope craft.Scope) ([]craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return nil, fmt.Errorf("%w: incomplete version scope", craft.ErrInvalidInput)
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
	var rows []craftVersionRow
	err = s.db.WithContext(ctx).Where("workspace_id = ?", ws.ID).
		Order("created_at DESC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	versions := make([]craft.Version, 0, len(rows))
	for _, row := range rows {
		v, e := loadCraftVersion(s.db.WithContext(ctx), row.ID)
		if e != nil {
			return nil, e
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// Get returns one published version in the requesting scope.
func (s *CraftVersionStore) Get(ctx context.Context, scope craft.Scope, id string) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || id == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete version request", craft.ErrInvalidInput)
	}
	var row craftVersionRow
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Version{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Version{}, err
	}
	if e := authorizeCraftVersion(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.Version{}, e
	}
	return loadCraftVersion(s.db.WithContext(ctx), id)
}
