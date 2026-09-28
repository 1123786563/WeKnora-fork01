package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArtifactVersion struct {
	TenantID   uint64
	OwnerID    string
	ResourceID string
	VersionID  string
	SessionID  string
	Digest     string
	ObjectKey  string
	MIME       string
	Size       int64
}

// ArtifactGrant is Career's local authorization query shape. Keeping it
// module-owned prevents this persistence adapter from depending on Workbench.
type ArtifactGrant struct {
	TenantID                               uint64
	OwnerID, ResourceID, VersionID, Digest string
}

type artifactBindingRow struct {
	TenantID   uint64     `gorm:"column:tenant_id;primaryKey"`
	OwnerID    string     `gorm:"column:owner_id;primaryKey"`
	ResourceID string     `gorm:"column:resource_id;primaryKey"`
	VersionID  string     `gorm:"column:version_id;primaryKey"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	DeletedAt  *time.Time `gorm:"column:deleted_at"`
}

func (artifactBindingRow) TableName() string { return "career_artifact_bindings" }

type artifactVersionRow struct {
	TenantID  uint64 `gorm:"column:tenant_id"`
	ID        string `gorm:"column:id"`
	SessionID string `gorm:"column:session_id"`
	Digest    string `gorm:"column:digest"`
	ObjectKey string `gorm:"column:object_key"`
	MIME      string `gorm:"column:mime"`
	Size      int64  `gorm:"column:size"`
	ScanState string `gorm:"column:scan_state"`
}

func (artifactVersionRow) TableName() string { return "artifact_versions" }

// ArtifactCatalogStore binds caller-owned Career resources to immutable
// Workbench versions. BindVersion is a trusted server-side publisher hook;
// clients can only request grants for existing bindings.
type ArtifactCatalogStore struct{ db *gorm.DB }

func NewArtifactCatalogStore(db *gorm.DB) *ArtifactCatalogStore { return &ArtifactCatalogStore{db: db} }

// BindVersion attaches a ready version to a Career resource. The session,
// digest, object key, MIME and size are read from the artifact catalog; callers
// cannot supply or override them.
func (s *ArtifactCatalogStore) BindVersion(ctx context.Context, scope Scope, resourceID, versionID string) (ArtifactVersion, error) {
	if err := scope.Validate(); err != nil {
		return ArtifactVersion{}, err
	}
	resourceID, versionID = strings.TrimSpace(resourceID), strings.TrimSpace(versionID)
	if s == nil || s.db == nil || resourceID == "" || versionID == "" {
		return ArtifactVersion{}, ErrNotFound
	}
	version, err := s.readReadyVersion(ctx, scope.TenantID, versionID, "")
	if err != nil {
		return ArtifactVersion{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("career_spaces").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"tenant_id": scope.TenantID, "owner_id": scope.OwnerID}).Error; err != nil {
			return err
		}
		return tx.Create(&artifactBindingRow{TenantID: scope.TenantID, OwnerID: scope.OwnerID, ResourceID: resourceID, VersionID: versionID}).Error
	})
	if err != nil {
		return ArtifactVersion{}, err
	}
	version.OwnerID, version.ResourceID = scope.OwnerID, resourceID
	return version, nil
}

func (s *ArtifactCatalogStore) Resolve(ctx context.Context, grant ArtifactGrant) (ArtifactVersion, error) {
	if s == nil || s.db == nil || grant.TenantID == 0 || strings.TrimSpace(grant.OwnerID) == "" || strings.TrimSpace(grant.ResourceID) == "" || strings.TrimSpace(grant.VersionID) == "" {
		return ArtifactVersion{}, ErrNotFound
	}
	var binding artifactBindingRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND resource_id = ? AND version_id = ? AND revoked_at IS NULL AND deleted_at IS NULL", grant.TenantID, grant.OwnerID, grant.ResourceID, grant.VersionID).Take(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ArtifactVersion{}, ErrNotFound
	}
	if err != nil {
		return ArtifactVersion{}, err
	}
	version, err := s.readReadyVersion(ctx, grant.TenantID, grant.VersionID, grant.Digest)
	if err != nil {
		return ArtifactVersion{}, err
	}
	version.OwnerID, version.ResourceID = grant.OwnerID, grant.ResourceID
	return version, nil
}

func (s *ArtifactCatalogStore) AuthorizeArtifactGrant(ctx context.Context, grant ArtifactGrant) error {
	_, err := s.Resolve(ctx, grant)
	return err
}

func (s *ArtifactCatalogStore) Revoke(ctx context.Context, scope Scope, resourceID, versionID string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrNotFound
	}
	result := s.db.WithContext(ctx).Model(&artifactBindingRow{}).Where("tenant_id = ? AND owner_id = ? AND resource_id = ? AND version_id = ? AND revoked_at IS NULL AND deleted_at IS NULL", scope.TenantID, scope.OwnerID, resourceID, versionID).Update("revoked_at", time.Now().UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ArtifactCatalogStore) Delete(ctx context.Context, scope Scope, resourceID, versionID string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return ErrNotFound
	}
	result := s.db.WithContext(ctx).Model(&artifactBindingRow{}).Where("tenant_id = ? AND owner_id = ? AND resource_id = ? AND version_id = ? AND deleted_at IS NULL", scope.TenantID, scope.OwnerID, resourceID, versionID).Update("deleted_at", time.Now().UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ArtifactCatalogStore) readReadyVersion(ctx context.Context, tenantID uint64, versionID, digest string) (ArtifactVersion, error) {
	var row artifactVersionRow
	query := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND scan_state = ?", tenantID, versionID, "ready")
	if digest != "" {
		query = query.Where("digest = ?", digest)
	}
	err := query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ArtifactVersion{}, ErrNotFound
	}
	if err != nil {
		return ArtifactVersion{}, err
	}
	return ArtifactVersion{TenantID: row.TenantID, VersionID: row.ID, SessionID: row.SessionID, Digest: row.Digest, ObjectKey: row.ObjectKey, MIME: row.MIME, Size: row.Size}, nil
}
