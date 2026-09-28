package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if row.RunCancellationState == "" {
		if row.InFlightDisposition == "cancel" {
			row.RunCancellationState = "pending"
		} else {
			row.RunCancellationState = "complete"
		}
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
	if row.RunCancellationState == "" {
		if row.InFlightDisposition == "cancel" {
			row.RunCancellationState = "pending"
		} else {
			row.RunCancellationState = "complete"
		}
	}
}

func (s *AgentSecurityStore) PublishedRunPinsForReleases(ctx context.Context, tenantID uint64, releases map[string]string) ([]AgentSecurityRunPin, error) {
	variants, err := s.ListVariants(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	pins := make([]AgentSecurityRunPin, 0)
	for _, v := range variants {
		if _, ok := releases[v.ReleaseID]; !ok || v.LocalAgentID == "" || v.LocalAgentVersionID == "" {
			continue
		}
		pins = append(pins, AgentSecurityRunPin{AgentID: v.LocalAgentID, LocalAgentVersionID: v.LocalAgentVersionID, ReleaseID: v.ReleaseID})
	}
	return pins, nil
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

func cancelActiveClaimsForReleasesTx(tx *gorm.DB, tenantID uint64, releaseIDs []string, reason string) error {
	if len(releaseIDs) == 0 {
		return nil
	}
	var claims []types.AgentChatTurnClaimEntity
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_tenant_id = ? AND release_id IN ? AND state = 'active'", tenantID, releaseIDs).Order("id ASC").Find(&claims).Error; err != nil {
		return err
	}
	for _, claim := range claims {
		res := tx.Model(&types.AgentChatTurnClaimEntity{}).Where("id=? AND source_tenant_id=? AND state='active'", claim.ID, tenantID).Updates(map[string]any{"state": "cancelled", "reason": reason, "generation": gorm.Expr("generation + 1"), "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			continue
		}
		if res.RowsAffected != 1 {
			return errors.New("agent security claim cancellation changed an unexpected row count")
		}
		if err := terminalizeClaimAssistantPlaceholderTx(tx, claim, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

// AppendReleaseRevocationWithAuditAndCancelClaims commits the release ledger,
// audit row, claim fencing transitions, and placeholder terminalization as one unit.
func (s *AgentSecurityStore) AppendReleaseRevocationWithAuditAndCancelClaims(ctx context.Context, row *types.AgentReleaseRevocationEntity, audit *types.AuditLog) error {
	if row == nil || audit == nil {
		return errors.New("release revocation and audit entry are required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		if row.InFlightDisposition == "cancel" {
			row.RunCancellationState = "pending"
		} else {
			row.RunCancellationState = "complete"
		}
		if err := appendReleaseRevocationTx(tx, row); err != nil {
			return err
		}
		if err := NewAuditLogRepository(tx).Create(ctx, audit); err != nil {
			return err
		}
		if row.InFlightDisposition == "cancel" {
			return cancelActiveClaimsForReleasesTx(tx, row.TenantID, []string{row.ReleaseID}, "agent security revocation: "+row.Reason)
		}
		return nil
	})
}

// AppendDependencyRevocationWithAuditAndCancelClaims matches Release locks by
// the complete dependency tuple under the same tenant transaction.
func (s *AgentSecurityStore) AppendDependencyRevocationWithAuditAndCancelClaims(ctx context.Context, row *types.AgentDependencyRevocationEntity, audit *types.AuditLog) error {
	if row == nil || audit == nil {
		return errors.New("dependency revocation and audit entry are required")
	}
	return withTenantSecurityGuard(ctx, s.db, row.TenantID, func(tx *gorm.DB) error {
		if row.InFlightDisposition == "cancel" {
			row.RunCancellationState = "pending"
		} else {
			row.RunCancellationState = "complete"
		}
		if err := appendDependencyRevocationTx(tx, row); err != nil {
			return err
		}
		if err := NewAuditLogRepository(tx).Create(ctx, audit); err != nil {
			return err
		}
		if row.InFlightDisposition != "cancel" {
			return nil
		}
		ids := make([]string, 0)
		var local []types.AgentReleaseEntity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", row.TenantID).Order("id ASC").Find(&local).Error; err != nil {
			return err
		}
		for _, release := range local {
			if dependencyLockContains(release.DependencyLockJSON, row) {
				ids = append(ids, release.ID)
			}
		}
		var introduced []types.TenantIntroducedReleaseEntity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", row.TenantID).Order("id ASC").Find(&introduced).Error; err != nil {
			return err
		}
		for _, release := range introduced {
			if dependencyLockContains(release.DependencyLockJSON, row) {
				ids = append(ids, release.ID)
			}
		}
		return cancelActiveClaimsForReleasesTx(tx, row.TenantID, ids, "agent security revocation: "+row.Reason)
	})
}

func dependencyLockContains(lockJSON string, row *types.AgentDependencyRevocationEntity) bool {
	var lock struct {
		Dependencies *[]types.AgentReleaseDependency `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(lockJSON), &lock); err != nil || lock.Dependencies == nil {
		return false
	}
	for _, d := range *lock.Dependencies {
		if d.Type == row.DepType && d.ID == row.DepID && d.Version == row.DepVersion && d.Digest == row.DepDigest {
			return true
		}
	}
	return false
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

// ResolvePublishedVariant returns the exact immutable Version and Release
// selected by the tenant's published Variant for this local Agent.
func (s *AgentSecurityStore) ResolvePublishedVariant(ctx context.Context, tenantID uint64, agentID string) (string, string, bool, error) {
	var versionID, releaseID string
	var adopted bool
	err := withTenantSecurityGuard(ctx, s.db, tenantID, func(tx *gorm.DB) error {
		var variants []types.AgentAdoptionVariantEntity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND local_agent_id = ?", tenantID, agentID).Order("id ASC").Find(&variants).Error; err != nil {
			return err
		}
		if len(variants) == 0 {
			_, isAdopted, err := checkLocalAgentReleaseAdmissionTx(tx, tenantID, agentID, "")
			if err != nil {
				return err
			}
			if isAdopted {
				return ErrAgentSecurityReleaseUnresolvable
			}
			return nil
		}
		if len(variants) != 1 {
			return ErrAgentSecurityReleaseUnresolvable
		}
		variant := variants[0]
		if variant.State != "published" || variant.RetiredAt != nil || variant.LocalAgentVersionID == "" {
			return ErrAgentSecurityReleaseUnresolvable
		}
		resolvedRelease, isAdopted, err := checkLocalAgentReleaseAdmissionTx(tx, tenantID, agentID, variant.LocalAgentVersionID)
		if err != nil {
			return err
		}
		if !isAdopted || resolvedRelease != variant.ReleaseID {
			return ErrAgentSecurityReleaseUnresolvable
		}
		versionID, releaseID, adopted = variant.LocalAgentVersionID, resolvedRelease, true
		return nil
	})
	return versionID, releaseID, adopted, err
}

func (s *AgentSecurityStore) ListVariants(ctx context.Context, tenantID uint64) ([]types.AgentAdoptionVariantEntity, error) {
	var rows []types.AgentAdoptionVariantEntity
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("created_at ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}
