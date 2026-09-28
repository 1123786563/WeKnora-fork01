package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAgentSecurityReleaseBlocked      = errors.New("agent security policy blocked the release")
	ErrAgentSecurityReleaseUnresolvable = errors.New("release is not resolvable in this tenant")
	ErrAgentSecurityUnsupportedDialect  = errors.New("tenant security guard unsupported database dialect")
)

func tenantSecurityLockPlan(dialect string) (query string, write bool, err error) {
	switch dialect {
	case "postgres":
		return "SELECT id FROM tenants WHERE id = ? FOR UPDATE", false, nil
	case "sqlite":
		return "UPDATE tenants SET id = id WHERE id = ?", true, nil
	default:
		return "", false, fmt.Errorf("%w: %s", ErrAgentSecurityUnsupportedDialect, dialect)
	}
}

// withTenantSecurityGuard serializes admission and revocation writes for one
// tenant. The callback must keep all guarded reads and writes on tx.
func withTenantSecurityGuard(ctx context.Context, db *gorm.DB, tenantID uint64, callback func(*gorm.DB) error) error {
	if tenantID == 0 {
		return ErrTenantNotFound
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := acquireTenantSecurityGuardTx(tx, tenantID); err != nil {
			return err
		}
		return callback(tx)
	})
}

// acquireTenantSecurityGuardTx takes the same lock used by security admission
// and revocation. Callers already inside a transaction can order multiple
// tenant guards without opening nested transactions.
func acquireTenantSecurityGuardTx(tx *gorm.DB, tenantID uint64) error {
	if tenantID == 0 {
		return ErrTenantNotFound
	}
	query, write, err := tenantSecurityLockPlan(tx.Dialector.Name())
	if err != nil {
		return err
	}
	if write {
		locked := tx.Exec(query, tenantID)
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return ErrTenantNotFound
		}
		return nil
	}
	var id uint64
	if err := tx.Raw(query, tenantID).Scan(&id).Error; err != nil {
		return err
	}
	if id != tenantID {
		return ErrTenantNotFound
	}
	return nil
}

// withTenantSecurityGuards acquires each distinct tenant guard in ascending
// numeric order inside one transaction. Callers must perform all protected
// reads and writes in callback using the supplied transaction.
func withTenantSecurityGuards(ctx context.Context, db *gorm.DB, tenantIDs []uint64, callback func(*gorm.DB) error) error {
	ids := make([]uint64, 0, len(tenantIDs))
	seen := make(map[uint64]struct{}, len(tenantIDs))
	for _, id := range tenantIDs {
		if id == 0 {
			return ErrTenantNotFound
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ErrTenantNotFound
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if err := acquireTenantSecurityGuardTx(tx, id); err != nil {
				return err
			}
		}
		return callback(tx)
	})
}

// checkReleaseAdmissionTx evaluates the current release and exact locked
// dependency identities through the transaction holding the tenant guard.
func checkReleaseAdmissionTx(tx *gorm.DB, tenantID uint64, releaseID string) error {
	var releaseRevocation types.AgentReleaseRevocationEntity
	err := tx.Where("tenant_id = ? AND release_id = ?", tenantID, releaseID).
		Order("created_at DESC").Order("id DESC").Take(&releaseRevocation).Error
	if err == nil {
		return fmt.Errorf("%w: release %s security-revoked: %s", ErrAgentSecurityReleaseBlocked, releaseID, releaseRevocation.Reason)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var releaseLock struct{ DependencyLockJSON string }
	err = tx.Model(&types.AgentReleaseEntity{}).
		Select("dependency_lock_json").Where("tenant_id = ? AND id = ?", tenantID, releaseID).Take(&releaseLock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Model(&types.TenantIntroducedReleaseEntity{}).
			Select("dependency_lock_json").Where("tenant_id = ? AND id = ?", tenantID, releaseID).Take(&releaseLock).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %s", ErrAgentSecurityReleaseUnresolvable, releaseID)
	}
	if err != nil {
		return err
	}
	lockJSON := releaseLock.DependencyLockJSON
	var lock struct {
		Dependencies *[]types.AgentReleaseDependency `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(lockJSON), &lock); err != nil {
		return fmt.Errorf("decode dependency lock for release %s: %w", releaseID, err)
	}
	if lock.Dependencies == nil {
		return fmt.Errorf("decode dependency lock for release %s: dependencies must be a non-null array", releaseID)
	}
	for _, dependency := range *lock.Dependencies {
		if dependency.Type == "" || dependency.ID == "" || dependency.Version == "" || dependency.Digest == "" {
			return fmt.Errorf("decode dependency lock for release %s: dependency identity is incomplete", releaseID)
		}
		var revocation types.AgentDependencyRevocationEntity
		err := tx.Where(`tenant_id = ? AND dep_type = ? AND dep_id = ? AND dep_version = ? AND dep_digest = ?`,
			tenantID, dependency.Type, dependency.ID, dependency.Version, dependency.Digest).
			Order("created_at DESC").Order("id DESC").Take(&revocation).Error
		if err == nil {
			return fmt.Errorf("%w: release %s has revoked dependency %s/%s@%s#%s: %s",
				ErrAgentSecurityReleaseBlocked, releaseID, dependency.Type, dependency.ID,
				dependency.Version, dependency.Digest, revocation.Reason)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}

// checkLocalAgentReleaseAdmissionTx resolves a published local Variant and its
// immutable Agent Version under the caller's tenant security guard. All
// mapping rows for the local identity are locked in deterministic ID order
// before their state is inspected, so stale or ambiguous lineage fails closed.
func checkLocalAgentReleaseAdmissionTx(tx *gorm.DB, sourceTenantID uint64, localAgentID, localAgentVersionID string) (releaseID string, adopted bool, err error) {
	if tx == nil || sourceTenantID == 0 || localAgentID == "" {
		return "", false, ErrAgentSecurityReleaseUnresolvable
	}
	var variants []types.AgentAdoptionVariantEntity
	query := tx.Where("tenant_id = ? AND local_agent_id = ?", sourceTenantID, localAgentID).
		Order("id ASC").Clauses(clause.Locking{Strength: "UPDATE"})
	if err := query.Find(&variants).Error; err != nil {
		return "", false, err
	}
	// The local Agent is the tenant-owned runtime identity for both adopted
	// and ordinary Agents. Lock it after Variant lineage and before Version so
	// a concurrent delete cannot turn a stale Marketplace mapping into an
	// admitted identity. GORM's model scope excludes soft-deleted rows.
	var localAgent types.CustomAgent
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", sourceTenantID, localAgentID).Take(&localAgent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, ErrAgentSecurityReleaseUnresolvable
		}
		return "", false, err
	}
	if len(variants) == 0 {
		// Non-Marketplace means a live, tenant-owned Agent with no Variant
		// lineage at all. A client-supplied/stale Version cannot classify it.
		if localAgentVersionID != "" {
			return "", false, ErrAgentSecurityReleaseUnresolvable
		}
		return "", false, nil
	}
	if len(variants) != 1 || localAgentVersionID == "" {
		return "", false, ErrAgentSecurityReleaseUnresolvable
	}
	variant := variants[0]
	if variant.State != "published" || variant.LocalAgentVersionID != localAgentVersionID || variant.ReleaseID == "" {
		return "", false, ErrAgentSecurityReleaseUnresolvable
	}
	var version types.AgentVersionEntity
	versionQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ? AND agent_id = ?", sourceTenantID, localAgentVersionID, localAgentID)
	if err := versionQuery.Take(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, ErrAgentSecurityReleaseUnresolvable
		}
		return "", false, err
	}
	if err := checkReleaseAdmissionTx(tx, sourceTenantID, variant.ReleaseID); err != nil {
		return "", false, err
	}
	return variant.ReleaseID, true, nil
}
