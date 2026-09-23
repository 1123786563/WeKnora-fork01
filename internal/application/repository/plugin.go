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

// MarkPreviewConsumed flips consumed_at exactly once. The WHERE
// consumed_at IS NULL guard makes the update one-shot: a second call (or a
// call against an absent ID) affects zero rows, which surfaces as
// gorm.ErrRecordNotFound so the confirmation path must reject.
func (r *pluginRepository) MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginPreview{}).
		Where("tenant_id = ? AND id = ? AND consumed_at IS NULL", tenantID, id).
		Update("consumed_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
