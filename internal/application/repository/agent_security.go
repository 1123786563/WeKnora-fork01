package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentReleaseLockRow projects the immutable listing and dependency lock
// attached to a tenant-visible release.
type AgentReleaseLockRow struct {
	ReleaseID string
	ListingID string
	LockJSON  string
}

// AgentSecurityStore persists append-only security revocations and reads the
// tenant-scoped release and variant facts used by security decisions.
type AgentSecurityStore struct {
	db *gorm.DB
}

func NewAgentSecurityStore(db *gorm.DB) *AgentSecurityStore {
	return &AgentSecurityStore{db: db}
}

func prepareReleaseRevocation(row *types.AgentReleaseRevocationEntity) {
	now := time.Now().UTC()
	if row.ID == "" {
		row.ID = uuid.NewString()
	}
	if row.RevokedAt.IsZero() {
		row.RevokedAt = now
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
}

func prepareDependencyRevocation(row *types.AgentDependencyRevocationEntity) {
	now := time.Now().UTC()
	if row.ID == "" {
		row.ID = uuid.NewString()
	}
	if row.RevokedAt.IsZero() {
		row.RevokedAt = now
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
}

func appendReleaseRevocationTx(tx *gorm.DB, row *types.AgentReleaseRevocationEntity) error {
	if row == nil {
		return errors.New("release revocation is required")
	}
	prepareReleaseRevocation(row)
	return tx.Create(row).Error
}

func appendDependencyRevocationTx(tx *gorm.DB, row *types.AgentDependencyRevocationEntity) error {
	if row == nil {
		return errors.New("dependency revocation is required")
	}
	prepareDependencyRevocation(row)
	return tx.Create(row).Error
}

func (s *AgentSecurityStore) AppendReleaseRevocation(ctx context.Context, row *types.AgentReleaseRevocationEntity) error {
	if row == nil {
		return errors.New("release revocation is required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		return appendReleaseRevocationTx(tx, row)
	})
}

// AppendReleaseRevocationWithAudit records the revocation and its audit fact
// atomically. Existing append-only callers may continue using AppendReleaseRevocation.
func (s *AgentSecurityStore) AppendReleaseRevocationWithAudit(ctx context.Context, row *types.AgentReleaseRevocationEntity, audit *types.AuditLog) error {
	if row == nil || audit == nil {
		return errors.New("release revocation and audit entry are required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		if err := appendReleaseRevocationTx(tx, row); err != nil {
			return err
		}
		return NewAuditLogRepository(tx).Create(ctx, audit)
	})
}

func (s *AgentSecurityStore) AppendDependencyRevocation(ctx context.Context, row *types.AgentDependencyRevocationEntity) error {
	if row == nil {
		return errors.New("dependency revocation is required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		return appendDependencyRevocationTx(tx, row)
	})
}

// AppendDependencyRevocationWithAudit records the revocation and its audit
// fact atomically. Existing append-only callers may continue using AppendDependencyRevocation.
func (s *AgentSecurityStore) AppendDependencyRevocationWithAudit(ctx context.Context, row *types.AgentDependencyRevocationEntity, audit *types.AuditLog) error {
	if row == nil || audit == nil {
		return errors.New("dependency revocation and audit entry are required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		if err := appendDependencyRevocationTx(tx, row); err != nil {
			return err
		}
		return NewAuditLogRepository(tx).Create(ctx, audit)
	})
}

func (s *AgentSecurityStore) ListReleaseRevocations(ctx context.Context, tenantID uint64) ([]types.AgentReleaseRevocationEntity, error) {
	var rows []types.AgentReleaseRevocationEntity
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}

func (s *AgentSecurityStore) ListDependencyRevocations(ctx context.Context, tenantID uint64) ([]types.AgentDependencyRevocationEntity, error) {
	var rows []types.AgentDependencyRevocationEntity
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}

func (s *AgentSecurityStore) GetReleaseRevocation(ctx context.Context, tenantID uint64, id string) (*types.AgentReleaseRevocationEntity, error) {
	var row types.AgentReleaseRevocationEntity
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *AgentSecurityStore) GetDependencyRevocation(ctx context.Context, tenantID uint64, id string) (*types.AgentDependencyRevocationEntity, error) {
	var row types.AgentDependencyRevocationEntity
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *AgentSecurityStore) UpdateReleaseRevocationCanceled(ctx context.Context, tenantID uint64, id string, canceled int64) error {
	return s.db.WithContext(ctx).Model(&types.AgentReleaseRevocationEntity{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		UpdateColumn("canceled_run_count", canceled).Error
}

func (s *AgentSecurityStore) UpdateDependencyRevocationCanceled(ctx context.Context, tenantID uint64, id string, canceled int64) error {
	return s.db.WithContext(ctx).Model(&types.AgentDependencyRevocationEntity{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		UpdateColumn("canceled_run_count", canceled).Error
}

func (s *AgentSecurityStore) ReleaseFacts(ctx context.Context, tenantID uint64, releaseID string) (listingID string, lockJSON string, found bool, err error) {
	var local types.AgentReleaseEntity
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, releaseID).Take(&local).Error
	if err == nil {
		return local.ListingID, local.DependencyLockJSON, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", false, err
	}

	var introduced types.TenantIntroducedReleaseEntity
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, releaseID).Take(&introduced).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return introduced.PublicListingID, introduced.DependencyLockJSON, true, nil
}

func (s *AgentSecurityStore) ListTenantReleaseLocks(ctx context.Context, tenantID uint64) ([]AgentReleaseLockRow, error) {
	var local []types.AgentReleaseEntity
	if err := s.db.WithContext(ctx).
		Select("id, listing_id, dependency_lock_json").
		Where("tenant_id = ?", tenantID).Order("id ASC").Find(&local).Error; err != nil {
		return nil, err
	}

	rows := make([]AgentReleaseLockRow, 0, len(local))
	seen := make(map[string]struct{}, len(local))
	for _, release := range local {
		rows = append(rows, AgentReleaseLockRow{ReleaseID: release.ID, ListingID: release.ListingID, LockJSON: release.DependencyLockJSON})
		seen[release.ID] = struct{}{}
	}

	var introduced []types.TenantIntroducedReleaseEntity
	if err := s.db.WithContext(ctx).
		Select("id, public_listing_id, dependency_lock_json").
		Where("tenant_id = ?", tenantID).Order("id ASC").Find(&introduced).Error; err != nil {
		return nil, err
	}
	for _, release := range introduced {
		if _, exists := seen[release.ID]; exists {
			continue
		}
		rows = append(rows, AgentReleaseLockRow{ReleaseID: release.ID, ListingID: release.PublicListingID, LockJSON: release.DependencyLockJSON})
		seen[release.ID] = struct{}{}
	}
	return rows, nil
}

func (s *AgentSecurityStore) VariantsByLocalAgent(ctx context.Context, tenantID uint64, agentID string) ([]types.AgentAdoptionVariantEntity, error) {
	var rows []types.AgentAdoptionVariantEntity
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND local_agent_id = ? AND state = ?", tenantID, agentID, "published").
		Order("created_at ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}

func (s *AgentSecurityStore) ListVariants(ctx context.Context, tenantID uint64) ([]types.AgentAdoptionVariantEntity, error) {
	var rows []types.AgentAdoptionVariantEntity
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("created_at ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}
