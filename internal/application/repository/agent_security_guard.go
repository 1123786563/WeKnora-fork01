package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
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
		} else {
			var id uint64
			if err := tx.Raw(query, tenantID).Scan(&id).Error; err != nil {
				return err
			}
			if id != tenantID {
				return ErrTenantNotFound
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
