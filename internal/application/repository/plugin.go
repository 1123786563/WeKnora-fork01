package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// pluginRepository implements the PluginRepository interface.
// Every query is parameter-bound — no string-assembled SQL.
type pluginRepository struct {
	db *gorm.DB
}

// NewPluginRepository creates a new plugin repository.
func NewPluginRepository(db *gorm.DB) interfaces.PluginRepository {
	return &pluginRepository{db: db}
}

// CreatePreview persists one verified preview row.
func (r *pluginRepository) CreatePreview(ctx context.Context, p *types.PluginPreview) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// GetPreview retrieves a preview by ID within a tenant; not found is
// (nil, nil) per the repository convention.
func (r *pluginRepository) GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error) {
	var preview types.PluginPreview
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&preview).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &preview, nil
}

// MarkPreviewConsumed flips consumed_at exactly once, with the TTL verdict
// INSIDE the same atomic UPDATE: the WHERE clause carries both
// consumed_at IS NULL and expires_at > now. A second call, a call against
// an absent ID, or a call after the preview crossed its expiry boundary all
// affect zero rows, which surfaces as gorm.ErrRecordNotFound — the
// confirmation path reads one rejection for "already consumed / expired /
// absent" and there is no check-then-act window between an Expired() read
// and this update.
func (r *pluginRepository) MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&types.PluginPreview{}).
		Where("tenant_id = ? AND id = ? AND consumed_at IS NULL AND expires_at > ?", tenantID, id, now).
		Update("consumed_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteExpiredPreviews drops preview rows past their expiry, consumed or
// not: a preview is a TTL-bound review artifact (expires_at is the TTL
// already shown to the admin), not an audit record — the table must not
// grow without bound (整分支 OCR 一轮 F1). Parameter-bound via gorm, the
// same shape as DeleteExpiredGrants; the service triggers it lazily and
// best-effort on the preview success path.
func (r *pluginRepository) DeleteExpiredPreviews(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("expires_at <= ?", before).
		Delete(&types.PluginPreview{}).Error
}

// CreateInstallation persists one accepted installation row.
func (r *pluginRepository) CreateInstallation(ctx context.Context, inst *types.PluginInstallation) error {
	return r.db.WithContext(ctx).Create(inst).Error
}

// GetInstallation retrieves an installation by ID within a tenant; not
// found (including a foreign tenant's ID) is (nil, nil) — existence is not
// leaked across tenants.
func (r *pluginRepository) GetInstallation(ctx context.Context, tenantID uint64, id string) (*types.PluginInstallation, error) {
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// GetInstallationByTenantPlugin retrieves the unique installation of one
// plugin within a tenant; not found is (nil, nil).
func (r *pluginRepository) GetInstallationByTenantPlugin(ctx context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error) {
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// GetByServiceID retrieves the installation bound to a materialized service
// ID within a tenant; not found (including manual services) is (nil, nil).
// The runtime snapshot guard resolves accepted capability through this.
func (r *pluginRepository) GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error) {
	if serviceID == "" {
		return nil, nil
	}
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_id = ?", tenantID, serviceID).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// ListInstallationsByTenant returns all installations of a tenant.
func (r *pluginRepository) ListInstallationsByTenant(ctx context.Context, tenantID uint64) ([]*types.PluginInstallation, error) {
	var installations []*types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&installations).Error
	return installations, err
}

// UpdateInstallationState flips state in place and refreshes updated_at;
// a zero-row update surfaces as gorm.ErrRecordNotFound.
func (r *pluginRepository) UpdateInstallationState(ctx context.Context, tenantID uint64, id, state string) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginInstallation{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{"state": state, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateInstallationServiceID backfills the materialized service binding.
func (r *pluginRepository) UpdateInstallationServiceID(ctx context.Context, tenantID uint64, id, serviceID string) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginInstallation{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{"service_id": serviceID, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteInstallation HARD-deletes (Unscoped) — this is the compensation
// path of ConfirmInstallation: a failed materialization must free the
// (tenant_id, plugin_id) unique slot. Installations carry no soft-delete
// column; the row either is the tenant's accepted baseline or does not
// exist.
func (r *pluginRepository) DeleteInstallation(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.PluginInstallation{}).Error
}
