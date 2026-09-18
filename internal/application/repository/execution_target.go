package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/execution"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrExecutionTargetNotFound = errors.New("execution target not found")

// ExecutionTargetStore is the ownership boundary used by HTTP and workers.
// Every request-facing read includes both tenant and owner predicates.
type ExecutionTargetStore interface {
	CreateTarget(ctx context.Context, target execution.Target, rootRef string) error
	// CreateTargetIfTrusted rechecks the persisted node identity and inserts the
	// target in one transaction. This closes the verify-then-insert rotation
	// race; callers must still perform the provider preflight at the HTTP seam.
	CreateTargetIfTrusted(ctx context.Context, target execution.Target, rootRef string) error
	GetOwnedTarget(ctx context.Context, tenantID uint64, actor, targetID string) (execution.Target, error)
	ListOwnedTargets(ctx context.Context, tenantID uint64, actor string) ([]execution.Target, error)
	RevokeTarget(ctx context.Context, tenantID uint64, actor, targetID string) error
	CreateWorkspace(ctx context.Context, workspace execution.Workspace) error
	GetOwnedWorkspace(ctx context.Context, tenantID uint64, actor, workspaceID string) (execution.Workspace, error)
}

// ProvisionPersonalTarget is consumed by the registration transaction. It is
// deliberately a separate method so the registration service cannot create a
// grant whose target identity is absent from the W18 admission store.
func (s *executionTargetStore) ProvisionPersonalTarget(ctx context.Context, tx *gorm.DB, target execution.Target) error {
	if tx == nil {
		tx = s.db
	}
	identity := executionTargetIdentityRow{TenantID: target.TenantID, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, OwnerID: target.OwnerID, CredentialVersion: target.CredentialVersion, State: "active"}
	if err := tx.WithContext(ctx).Create(&identity).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&executionTargetRow{TenantID: target.TenantID, ID: target.ID, OwnerID: target.OwnerID, Kind: target.Kind, State: target.State, CredentialVersion: target.CredentialVersion, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID}).Error
}

func (s *executionTargetStore) RevokePersonalTarget(ctx context.Context, tx *gorm.DB, expected execution.Target, now time.Time) error {
	if tx == nil {
		tx = s.db
	}
	var target executionTargetRow
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ?", expected.TenantID, expected.OwnerID, expected.ID, expected.RuntimeID, expected.ExternalTargetID, expected.CredentialVersion).First(&target).Error; err != nil {
		return err
	}
	// The legacy registration alias updates its registration row after this
	// method returns. Preflight that projection here so the target and identity
	// rows cannot be fenced while a divergent registration remains active.
	var registration registrationProjectionRow
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND registration_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", expected.TenantID, expected.OwnerID, expected.ID, expected.RuntimeID, expected.ExternalTargetID, expected.CredentialVersion, "active").First(&registration).Error; err != nil {
		return err
	}
	result := tx.WithContext(ctx).Model(&executionTargetRow{}).Where("tenant_id = ? AND owner_id = ? AND id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", expected.TenantID, expected.OwnerID, expected.ID, expected.RuntimeID, expected.ExternalTargetID, expected.CredentialVersion, "active").Updates(map[string]any{
		"state": "revoked", "revoked_at": now.UTC(), "credential_version": gorm.Expr("credential_version + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrExecutionTargetNotFound
	}
	identityResult := tx.WithContext(ctx).Model(&executionTargetIdentityRow{}).Where("tenant_id = ? AND owner_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", expected.TenantID, expected.OwnerID, target.RuntimeID, target.ExternalTargetID, expected.CredentialVersion, "active").Updates(map[string]any{"state": "revoked", "credential_version": gorm.Expr("credential_version + 1")})
	if identityResult.Error != nil {
		return identityResult.Error
	}
	if identityResult.RowsAffected != 1 {
		return ErrExecutionTargetNotFound
	}
	return nil
}

// executionTargetIdentityProvider is backed by the node-registration
// projection. It is intentionally separate from execution_targets: a target
// cannot authorize its own first registration.
type executionTargetIdentityProvider struct{ db *gorm.DB }

func NewExecutionTargetIdentityProvider(db *gorm.DB) execution.TargetIdentityProvider {
	return &executionTargetIdentityProvider{db: db}
}

type executionTargetIdentityRow struct {
	TenantID          uint64 `gorm:"primaryKey;column:tenant_id"`
	RuntimeID         string `gorm:"primaryKey;column:runtime_id"`
	ExternalTargetID  string `gorm:"primaryKey;column:external_target_id"`
	OwnerID           string `gorm:"column:owner_id"`
	CredentialVersion int64  `gorm:"column:credential_version"`
	State             string `gorm:"column:state"`
}

func (executionTargetIdentityRow) TableName() string { return "execution_target_identities" }

func (p *executionTargetIdentityProvider) VerifyTarget(ctx context.Context, tenantID uint64, actor string, target execution.Target) error {
	var identity executionTargetIdentityRow
	err := p.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", tenantID, actor, target.RuntimeID, target.ExternalTargetID, target.CredentialVersion, "active").First(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return execution.ErrTargetUntrusted
	}
	return err
}

type executionTargetRow struct {
	TenantID          uint64     `gorm:"primaryKey;column:tenant_id"`
	ID                string     `gorm:"primaryKey;column:id"`
	OwnerID           string     `gorm:"column:owner_id"`
	Kind              string     `gorm:"column:kind"`
	State             string     `gorm:"column:state"`
	CredentialVersion int64      `gorm:"column:credential_version"`
	RuntimeID         string     `gorm:"column:runtime_id"`
	ExternalTargetID  string     `gorm:"column:external_target_id"`
	UsageBindingJSON  string     `gorm:"column:usage_binding_json;not null;default:'{}'"`
	RootRef           string     `gorm:"column:root_ref"`
	RevokedAt         *time.Time `gorm:"column:revoked_at"`
}

func (executionTargetRow) TableName() string { return "execution_targets" }

type executionWorkspaceRow struct {
	TenantID uint64 `gorm:"primaryKey;column:tenant_id"`
	ID       string `gorm:"primaryKey;column:id"`
	TargetID string `gorm:"column:target_id"`
	RootRef  string `gorm:"column:root_ref"`
}

func (executionWorkspaceRow) TableName() string { return "execution_workspaces" }

type executionTargetStore struct{ db *gorm.DB }

func NewExecutionTargetStore(db *gorm.DB) ExecutionTargetStore {
	return &executionTargetStore{db: db}
}

// NewPersonalTargetProvisioner exposes only the registration seam to DI;
// request handlers continue to depend on ExecutionTargetStore.
func NewPersonalTargetProvisioner(db *gorm.DB) execution.TargetProvisioner {
	return &executionTargetStore{db: db}
}

func toTarget(row executionTargetRow) execution.Target {
	target := execution.Target{ID: row.ID, TenantID: row.TenantID, OwnerID: row.OwnerID, Kind: row.Kind, State: row.State, CredentialVersion: row.CredentialVersion, RuntimeID: row.RuntimeID, ExternalTargetID: row.ExternalTargetID, RevokedAt: row.RevokedAt}
	if row.UsageBindingJSON != "" {
		_ = json.Unmarshal([]byte(row.UsageBindingJSON), &target.UsageBinding)
	}
	return target
}

func targetBindingJSON(target execution.Target) string {
	b, _ := json.Marshal(target.UsageBinding)
	return string(b)
}

func (s *executionTargetStore) CreateTarget(ctx context.Context, target execution.Target, rootRef string) error {
	return s.db.WithContext(ctx).Create(&executionTargetRow{TenantID: target.TenantID, ID: target.ID, OwnerID: target.OwnerID, Kind: target.Kind, State: target.State, CredentialVersion: target.CredentialVersion, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, UsageBindingJSON: targetBindingJSON(target), RootRef: rootRef}).Error
}

func (s *executionTargetStore) CreateTargetIfTrusted(ctx context.Context, target execution.Target, rootRef string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var identity executionTargetIdentityRow
		query := tx.Where("tenant_id = ? AND owner_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", target.TenantID, target.OwnerID, target.RuntimeID, target.ExternalTargetID, target.CredentialVersion, "active")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&identity).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return execution.ErrTargetUntrusted
		} else if err != nil {
			return err
		}
		return tx.Create(&executionTargetRow{TenantID: target.TenantID, ID: target.ID, OwnerID: target.OwnerID, Kind: target.Kind, State: target.State, CredentialVersion: target.CredentialVersion, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, UsageBindingJSON: targetBindingJSON(target), RootRef: rootRef}).Error
	})
}

func (s *executionTargetStore) GetOwnedTarget(ctx context.Context, tenantID uint64, actor, targetID string) (execution.Target, error) {
	var row executionTargetRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ? AND state = ?", tenantID, actor, targetID, "active").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return execution.Target{}, ErrExecutionTargetNotFound
	}
	if err != nil {
		return execution.Target{}, err
	}
	return toTarget(row), nil
}

func (s *executionTargetStore) ListOwnedTargets(ctx context.Context, tenantID uint64, actor string) ([]execution.Target, error) {
	var rows []executionTargetRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND state = ?", tenantID, actor, "active").Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]execution.Target, 0, len(rows))
	for _, row := range rows {
		result = append(result, toTarget(row))
	}
	return result, nil
}

func (s *executionTargetStore) RevokeTarget(ctx context.Context, tenantID uint64, actor, targetID string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target executionTargetRow
		if err := tx.Where("tenant_id = ? AND owner_id = ? AND id = ? AND state = ?", tenantID, actor, targetID, "active").First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrExecutionTargetNotFound
			}
			return err
		}
		result := tx.Model(&executionTargetRow{}).Where("tenant_id = ? AND owner_id = ? AND id = ? AND state = ?", tenantID, actor, targetID, "active").Updates(map[string]any{
			"state": "revoked", "revoked_at": now, "credential_version": gorm.Expr("credential_version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrExecutionTargetNotFound
		}
		// Personal targets have a registration and identity projection with the
		// same target ID/runtime binding. Revoke the projections in this same
		// transaction so the public target facade cannot leave an active grant
		// behind. Managed targets may not have these rows, so they remain a
		// target-only revoke.
		if target.Kind != "personal_node" {
			return nil
		}
		registration := tx.Model(&registrationProjectionRow{}).Where("tenant_id = ? AND owner_id = ? AND registration_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", tenantID, actor, targetID, target.RuntimeID, target.ExternalTargetID, target.CredentialVersion, "active").Updates(map[string]any{
			"state": "revoked", "revoked_at": now, "credential_version": gorm.Expr("credential_version + 1"),
		})
		if registration.Error != nil {
			return registration.Error
		}
		if registration.RowsAffected != 1 {
			return ErrExecutionTargetNotFound
		}
		identity := tx.Model(&executionTargetIdentityRow{}).Where("tenant_id = ? AND owner_id = ? AND runtime_id = ? AND external_target_id = ? AND credential_version = ? AND state = ?", tenantID, actor, target.RuntimeID, target.ExternalTargetID, target.CredentialVersion, "active").Updates(map[string]any{
			"state": "revoked", "credential_version": gorm.Expr("credential_version + 1"),
		})
		if identity.Error != nil {
			return identity.Error
		}
		if identity.RowsAffected != 1 {
			return ErrExecutionTargetNotFound
		}
		return nil
	})
}

// registrationProjectionRow deliberately mirrors only the columns needed by
// the cross-package target facade. The execution package owns registration
// lifecycle semantics; this projection lets the public target revoke route
// use the same transaction and fencing seam without importing private models.
type registrationProjectionRow struct {
	TenantID          uint64     `gorm:"column:tenant_id"`
	OwnerID           string     `gorm:"column:owner_id"`
	ID                string     `gorm:"column:registration_id"`
	RuntimeID         string     `gorm:"column:runtime_id"`
	ExternalTargetID  string     `gorm:"column:external_target_id"`
	CredentialVersion int64      `gorm:"column:credential_version"`
	State             string     `gorm:"column:state"`
	RevokedAt         *time.Time `gorm:"column:revoked_at"`
}

func (registrationProjectionRow) TableName() string { return "execution_registrations" }

func (s *executionTargetStore) CreateWorkspace(ctx context.Context, workspace execution.Workspace) error {
	return s.db.WithContext(ctx).Create(&executionWorkspaceRow{TenantID: workspace.TenantID, ID: workspace.ID, TargetID: workspace.TargetID, RootRef: workspace.RootRef}).Error
}

func (s *executionTargetStore) GetOwnedWorkspace(ctx context.Context, tenantID uint64, actor, workspaceID string) (execution.Workspace, error) {
	var row executionWorkspaceRow
	err := s.db.WithContext(ctx).Table("execution_workspaces AS w").Select("w.tenant_id, w.id, w.target_id, w.root_ref").Joins("JOIN execution_targets AS t ON t.tenant_id = w.tenant_id AND t.id = w.target_id").Where("w.tenant_id = ? AND w.id = ? AND t.owner_id = ? AND t.state = ?", tenantID, workspaceID, actor, "active").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return execution.Workspace{}, ErrExecutionTargetNotFound
	}
	if err != nil {
		return execution.Workspace{}, err
	}
	return execution.Workspace{ID: row.ID, TargetID: row.TargetID, RootRef: row.RootRef, TenantID: row.TenantID}, nil
}
