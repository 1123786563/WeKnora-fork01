package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSubagentTestRepo(t *testing.T) TenantSubagentRepository {
	t.Helper()
	repo, _ := newSubagentTestRepoWithDB(t)
	return repo
}

// newSubagentTestRepoWithDB also hands back the handle, for the tests that
// must read a row raw to prove a delete was soft rather than destructive.
func newSubagentTestRepoWithDB(t *testing.T) (TenantSubagentRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantSubagentEntity{}))
	// AutoMigrate cannot express the partial unique index, so add it here to
	// match the production migration.
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_subagents_scope
		 ON tenant_subagents (tenant_id, slug, locale) WHERE deleted_at IS NULL`).Error)
	return NewTenantSubagentRepository(db), db
}

func subagentRow(id, slug, locale, content string) *types.TenantSubagentEntity {
	return &types.TenantSubagentEntity{
		ID: id, TenantID: 7, Slug: slug, Locale: locale,
		Content: content, Division: "engineering", Source: types.SubagentSourceBuiltin,
	}
}

func TestSubagentUpsertIsIdempotentPerTenantSlugLocale(t *testing.T) {
	repo := newSubagentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-a", "code-reviewer", "zh", "# 初版")))
	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-b", "code-reviewer", "zh", "---\nname: 评审\ncolor: '#f00'\n---\n# 第二版")))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1, "a second upsert of the same (tenant, slug, locale) replaces the row, it does not add one")
	// Content is the FULL markdown, frontmatter fence included, verbatim.
	require.Equal(t, "---\nname: 评审\ncolor: '#f00'\n---\n# 第二版", list[0].Content)
	require.Equal(t, "sg-a", list[0].ID, "the original row key stays stable across reinstalls")
}

func TestSubagentUpsertKeepsLocalesApart(t *testing.T) {
	repo := newSubagentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-zh", "code-reviewer", "zh", "# 中文")))
	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-en", "code-reviewer", "en", "# English")))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 2, "the same slug in two locales is two rows")
}

func TestSubagentListIsTenantScoped(t *testing.T) {
	repo := newSubagentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-a", "code-reviewer", "zh", "# 评审")))
	other := subagentRow("sg-b", "code-reviewer", "zh", "# someone else's")
	other.TenantID = 8
	require.NoError(t, repo.Upsert(ctx, other))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, uint64(7), list[0].TenantID)
	require.Equal(t, "sg-a", list[0].ID, "another tenant's row must not leak into the listing")
}

func TestSubagentDeleteIsSoftAndSlugWide(t *testing.T) {
	repo, db := newSubagentTestRepoWithDB(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-zh", "code-reviewer", "zh", "# 中文")))
	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-en", "code-reviewer", "en", "# English")))
	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-keep", "web-researcher", "zh", "# 搜索")))

	require.NoError(t, repo.Delete(ctx, 7, "code-reviewer"))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "web-researcher", list[0].Slug, "deleting a slug takes its every locale with it")

	var live, dead int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM tenant_subagents WHERE deleted_at IS NULL`).Scan(&live).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM tenant_subagents WHERE deleted_at IS NOT NULL`).Scan(&dead).Error)
	require.EqualValues(t, 1, live)
	require.EqualValues(t, 2, dead, "the delete must be soft: rows stay for audit")
}

func TestSubagentDeleteIsTenantScopedAndIdempotent(t *testing.T) {
	repo := newSubagentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-a", "code-reviewer", "zh", "# 评审")))

	require.NoError(t, repo.Delete(ctx, 8, "code-reviewer"),
		"a delete from another tenant must affect no rows and still succeed")
	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1, "another tenant's delete must not reach this tenant's row")

	require.NoError(t, repo.Delete(ctx, 7, "missing-slug"),
		"deleting an absent slug remains an idempotent no-op")
}

// The unique index is partial (WHERE deleted_at IS NULL), so a soft-deleted
// slug must be re-upsertable in place — the same semantics a skill reinstall
// gets from uq_tenant_skills_config_name.
func TestSubagentSoftDeleteAllowsSlugReuse(t *testing.T) {
	repo := newSubagentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-old", "code-reviewer", "zh", "# 旧")))
	require.NoError(t, repo.Delete(ctx, 7, "code-reviewer"))
	require.NoError(t, repo.Upsert(ctx, subagentRow("sg-new", "code-reviewer", "zh", "# 新")))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "sg-new", list[0].ID)
	require.Equal(t, "# 新", list[0].Content)
}
