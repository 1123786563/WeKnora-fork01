package repository

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TenantSubagentRepository persists the builtin sub-agent roles copied into
// one tenant's storage. Rows are written by library syncs and read when an
// agent's Subagents config must resolve to runnable prompts.
type TenantSubagentRepository interface {
	// Upsert writes one (tenant, slug, locale) slot: a live row with the same
	// key has its content/division/source replaced, anything else inserts. A
	// soft-deleted slot does not conflict, so the role comes back under a new
	// row key, matching how skill reinstalls reuse a name.
	Upsert(ctx context.Context, e *types.TenantSubagentEntity) error
	// ListByTenant returns every live role row of one workspace, ordered
	// deterministically by slug then locale so pairing locales of one slug is
	// a single pass.
	ListByTenant(ctx context.Context, tenantID uint64) ([]types.TenantSubagentEntity, error)
	// Delete soft-deletes every locale of one slug for one tenant. When the
	// scoped key matches nothing, no rows are touched and the call succeeds.
	Delete(ctx context.Context, tenantID uint64, slug string) error
}

type tenantSubagentRepository struct{ db *gorm.DB }

// NewTenantSubagentRepository returns a GORM-backed implementation.
func NewTenantSubagentRepository(db *gorm.DB) TenantSubagentRepository {
	return &tenantSubagentRepository{db: db}
}

// Upsert conflicts on the partial unique index columns (tenant_id, slug,
// locale) — the same shape UpsertUserEnvVar uses — so a library sync re-running
// over unchanged slots updates in place instead of accumulating rows. The
// TargetWhere matches the index predicate; without it neither dialect accepts
// the inference from a partial index, and with it a soft-deleted slot stays
// outside the conflict target and reinserts fresh.
func (r *tenantSubagentRepository) Upsert(ctx context.Context, e *types.TenantSubagentEntity) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "slug"}, {Name: "locale"},
			},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				gorm.Expr("deleted_at IS NULL"),
			}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "division", "source", "updated_at"}),
		}).
		Create(e).Error
}

func (r *tenantSubagentRepository) ListByTenant(
	ctx context.Context, tenantID uint64,
) ([]types.TenantSubagentEntity, error) {
	var list []types.TenantSubagentEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("slug ASC, locale ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *tenantSubagentRepository) Delete(ctx context.Context, tenantID uint64, slug string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND slug = ?", tenantID, slug).
		Delete(&types.TenantSubagentEntity{}).Error
}
