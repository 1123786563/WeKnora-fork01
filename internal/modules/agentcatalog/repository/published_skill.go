package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PublishedSkillRepository persists the tenant-internal skill market's
// publish rows: one live row per (tenant, catalog skill) an admin published.
// Rows are a visibility flag over the shared tenant_skill_catalog
// definition — never a content copy.
type PublishedSkillRepository interface {
	// Upsert publishes one (tenant, catalog) slot: a live row with the same
	// key keeps its ID/created_at and has its publisher stamp and timestamp
	// refreshed (idempotent re-publish); anything else inserts. A
	// soft-deleted slot does not conflict, so the skill comes back under a
	// new row key.
	Upsert(ctx context.Context, e *types.PublishedSkillEntity) error
	// ListByTenant returns every live publish row of one workspace, ordered
	// by creation for a stable listing.
	ListByTenant(ctx context.Context, tenantID uint64) ([]types.PublishedSkillEntity, error)
	// GetByTenantAndCatalog returns one live publish row or nil when the
	// slot is empty, soft-deleted or belongs to another tenant.
	GetByTenantAndCatalog(ctx context.Context, tenantID uint64, catalogID string) (*types.PublishedSkillEntity, error)
	// Delete soft-deletes one slot (unpublish). When the scoped key matches
	// nothing, no rows are touched and the call succeeds.
	Delete(ctx context.Context, tenantID uint64, catalogID string) error
}

type publishedSkillRepository struct{ db *gorm.DB }

// NewPublishedSkillRepository returns a GORM-backed implementation.
func NewPublishedSkillRepository(db *gorm.DB) PublishedSkillRepository {
	return &publishedSkillRepository{db: db}
}

// Upsert conflicts on the partial unique index columns (tenant_id,
// catalog_id) — the same shape ExpertInstallRepository.Upsert uses — so a
// re-publish over the same slot refreshes in place instead of accumulating
// rows. The TargetWhere matches the index predicate; without it neither
// dialect accepts the inference from a partial index, and with it a
// soft-deleted slot stays outside the conflict target and reinserts fresh.
func (r *publishedSkillRepository) Upsert(ctx context.Context, e *types.PublishedSkillEntity) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "catalog_id"},
			},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				gorm.Expr("deleted_at IS NULL"),
			}},
			DoUpdates: clause.AssignmentColumns([]string{"published_by", "updated_at"}),
		}).
		Create(e).Error
}

func (r *publishedSkillRepository) ListByTenant(
	ctx context.Context, tenantID uint64,
) ([]types.PublishedSkillEntity, error) {
	var list []types.PublishedSkillEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *publishedSkillRepository) GetByTenantAndCatalog(
	ctx context.Context, tenantID uint64, catalogID string,
) (*types.PublishedSkillEntity, error) {
	var row types.PublishedSkillEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND catalog_id = ?", tenantID, catalogID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *publishedSkillRepository) Delete(ctx context.Context, tenantID uint64, catalogID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND catalog_id = ?", tenantID, catalogID).
		Delete(&types.PublishedSkillEntity{}).Error
}
