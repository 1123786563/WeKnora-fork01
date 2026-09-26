package appconnector

// Per-actor grants on SPACE connections (T23, #53). CONTEXT.md 代码平台连接:
// 个人连接只能由其所有者使用；空间连接按仓库、成员角色和操作策略授权。
// This store is the persistence half; the decision half lives in
// appconnector.CanUseConnection (access.go) and the A02 authorizer's Check
// (service/appconnector/oc_authorizer.go) — this package never decides.

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// SpaceConnectionGrantRow is one explicit actor's grant on one space
// connection. Personal connections never carry a row: the management
// endpoint refuses them and the authorizer never reads this table for
// Kind='personal'.
type SpaceConnectionGrantRow struct {
	TenantID     uint64    `gorm:"primaryKey;column:tenant_id"`
	ConnectionID string    `gorm:"primaryKey;column:connection_id;type:varchar(64)"`
	ActorID      string    `gorm:"primaryKey;column:actor_id;type:varchar(512)"`
	GrantedBy    string    `gorm:"column:granted_by;type:varchar(512);not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName binds SpaceConnectionGrantRow to app_space_connection_grants
// (versioned 000198 / sqlite 000119).
func (SpaceConnectionGrantRow) TableName() string { return "app_space_connection_grants" }

// SpaceConnectionGrantStore persists and adjudicates space-connection
// grants. It structurally satisfies appconnectorsvc.SpaceGrantSource via
// SpaceConnectionGranted.
type SpaceConnectionGrantStore struct{ db *gorm.DB }

func NewSpaceConnectionGrantStore(db *gorm.DB) *SpaceConnectionGrantStore {
	return &SpaceConnectionGrantStore{db: db}
}

// GrantSpaceConnection upserts one grant (idempotent per
// tenant+connection+actor). All inputs are bound parameters.
func (s *SpaceConnectionGrantStore) GrantSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID, grantedBy string) error {
	row := SpaceConnectionGrantRow{
		TenantID: tenantID, ConnectionID: connectionID, ActorID: actorID,
		GrantedBy: grantedBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

// RevokeSpaceConnection removes one grant; revoking an absent grant is a
// success (idempotent).
func (s *SpaceConnectionGrantStore) RevokeSpaceConnection(ctx context.Context, tenantID uint64, connectionID, actorID string) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", tenantID, connectionID, actorID).
		Delete(&SpaceConnectionGrantRow{}).Error
}

// ListSpaceConnectionGrants returns the connection's grants, tenant-scoped.
func (s *SpaceConnectionGrantStore) ListSpaceConnectionGrants(ctx context.Context, tenantID uint64, connectionID string) ([]SpaceConnectionGrantRow, error) {
	var rows []SpaceConnectionGrantRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ?", tenantID, connectionID).
		Order("actor_id ASC").Find(&rows).Error
	return rows, err
}

// SpaceConnectionGranted answers the A02 authorizer's per-call question.
// Tenant scope is part of the predicate: another tenant's grant row is
// indistinguishable from no grant.
func (s *SpaceConnectionGrantStore) SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&SpaceConnectionGrantRow{}).
		Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", tenantID, connectionID, actorID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
