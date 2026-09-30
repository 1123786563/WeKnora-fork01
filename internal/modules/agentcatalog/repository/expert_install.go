package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExpertInstallRepository persists the per-tenant expert install ledger:
// one row per (tenant, skillset slug) a tenant installed from the SkillHub
// market. Rows are written by installs and read to answer "what is
// installed" without scanning tenant storage.
type ExpertInstallRepository interface {
	// Upsert writes one (tenant, slug) slot: a live row with the same key
	// has its storage ref, snapshot digest and creator replaced, anything
	// else inserts. A soft-deleted slot does not conflict, so the skillset
	// comes back under a new row key, matching how skill reinstalls reuse
	// a name.
	Upsert(ctx context.Context, e *types.ExpertInstallEntity) error
	// ListByTenant returns every live install of one workspace, ordered by
	// slug for stable catalog merges.
	ListByTenant(ctx context.Context, tenantID uint64) ([]types.ExpertInstallEntity, error)
	// GetByTenantAndSlug returns one live install or nil when the slot is
	// empty, soft-deleted or belongs to another tenant.
	GetByTenantAndSlug(ctx context.Context, tenantID uint64, slug string) (*types.ExpertInstallEntity, error)
	// Delete soft-deletes one slot. When the scoped key matches nothing, no
	// rows are touched and the call succeeds.
	Delete(ctx context.Context, tenantID uint64, slug string) error
}

type expertInstallRepository struct{ db *gorm.DB }

// NewExpertInstallRepository returns a GORM-backed implementation.
func NewExpertInstallRepository(db *gorm.DB) ExpertInstallRepository {
	return &expertInstallRepository{db: db}
}

// Upsert conflicts on the partial unique index columns (tenant_id, slug) —
// the same shape TenantSubagentRepository.Upsert uses — so a reinstall
// re-running over the same slot updates in place instead of accumulating
// rows. The TargetWhere matches the index predicate; without it neither
// dialect accepts the inference from a partial index, and with it a
// soft-deleted slot stays outside the conflict target and reinserts fresh.
func (r *expertInstallRepository) Upsert(ctx context.Context, e *types.ExpertInstallEntity) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "slug"},
			},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				gorm.Expr("deleted_at IS NULL"),
			}},
			DoUpdates: clause.AssignmentColumns([]string{"storage_ref", "snapshot_sha256", "created_by", "updated_at"}),
		}).
		Create(e).Error
}

func (r *expertInstallRepository) ListByTenant(
	ctx context.Context, tenantID uint64,
) ([]types.ExpertInstallEntity, error) {
	var list []types.ExpertInstallEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("slug ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *expertInstallRepository) GetByTenantAndSlug(
	ctx context.Context, tenantID uint64, slug string,
) (*types.ExpertInstallEntity, error) {
	var row types.ExpertInstallEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND slug = ?", tenantID, slug).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *expertInstallRepository) Delete(ctx context.Context, tenantID uint64, slug string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND slug = ?", tenantID, slug).
		Delete(&types.ExpertInstallEntity{}).Error
}
