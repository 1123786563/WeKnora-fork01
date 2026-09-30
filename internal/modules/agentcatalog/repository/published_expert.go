package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PublishedExpertRepository persists the tenant-internal expert market's
// publish rows: one live row per (tenant, agent) carrying the immutable
// snapshot pointer, its digest and the metadata recorded at publish time.
type PublishedExpertRepository interface {
	// Upsert publishes one (tenant, agent) slot: a live row with the same
	// key keeps its ID/created_at and has its metadata, snapshot stamp and
	// publisher refreshed (idempotent re-publish); anything else inserts. A
	// soft-deleted slot does not conflict, so the agent comes back under a
	// new row key.
	Upsert(ctx context.Context, e *types.PublishedExpertEntity) error
	// ListByTenant returns every live publish row of one workspace, ordered
	// by creation for a stable listing.
	ListByTenant(ctx context.Context, tenantID uint64) ([]types.PublishedExpertEntity, error)
	// GetByTenantAndAgent returns one live publish row or nil when the slot
	// is empty, soft-deleted or belongs to another tenant.
	GetByTenantAndAgent(ctx context.Context, tenantID uint64, agentID string) (*types.PublishedExpertEntity, error)
	// GetByTenantAndID returns one live publish row addressed by its row id
	// (the market route parameter), or nil under the same conditions.
	GetByTenantAndID(ctx context.Context, tenantID uint64, id string) (*types.PublishedExpertEntity, error)
	// Delete soft-deletes one row by id (unpublish). When the scoped id
	// matches nothing, no rows are touched and the call succeeds.
	Delete(ctx context.Context, tenantID uint64, id string) error
}

type publishedExpertRepository struct{ db *gorm.DB }

// NewPublishedExpertRepository returns a GORM-backed implementation.
func NewPublishedExpertRepository(db *gorm.DB) PublishedExpertRepository {
	return &publishedExpertRepository{db: db}
}

// Upsert conflicts on the partial unique index columns (tenant_id,
// agent_id) — the same shape PublishedSkillRepository.Upsert uses — so a
// re-publish over the same slot refreshes in place instead of accumulating
// rows. The TargetWhere matches the index predicate; without it neither
// dialect accepts the inference from a partial index, and with it a
// soft-deleted slot stays outside the conflict target and reinserts fresh.
func (r *publishedExpertRepository) Upsert(ctx context.Context, e *types.PublishedExpertEntity) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "agent_id"},
			},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				gorm.Expr("deleted_at IS NULL"),
			}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "snapshot_ref", "snapshot_sha256", "published_by", "updated_at",
			}),
		}).
		Create(e).Error
}

func (r *publishedExpertRepository) ListByTenant(
	ctx context.Context, tenantID uint64,
) ([]types.PublishedExpertEntity, error) {
	var list []types.PublishedExpertEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *publishedExpertRepository) GetByTenantAndAgent(
	ctx context.Context, tenantID uint64, agentID string,
) (*types.PublishedExpertEntity, error) {
	var row types.PublishedExpertEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_id = ?", tenantID, agentID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *publishedExpertRepository) GetByTenantAndID(
	ctx context.Context, tenantID uint64, id string,
) (*types.PublishedExpertEntity, error) {
	var row types.PublishedExpertEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *publishedExpertRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.PublishedExpertEntity{}).Error
}
