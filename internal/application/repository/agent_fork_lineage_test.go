package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tencent/WeKnora/internal/types"
)

// openForkLineageDB follows the fast in-memory convention of
// openAdoptionVariantDB (agent_adoption_test.go): AutoMigrate over the
// entities involved, plus the adoption scope unique index. The migration
// stream itself is exercised by the service/router tests.
func openForkLineageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.AgentAdoptionEntity{}, &types.AgentAdoptionVariantEntity{},
		&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseEntity{},
		&types.AgentLicenseEntity{}, &types.TenantIntroducedReleaseEntity{},
	))
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedDerivation(t *testing.T, db *gorm.DB, releaseID string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-1", TenantID: 1, ListingID: "listing-1", AcceptedReleaseID: releaseID, State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-1", TenantID: 1, AdoptionID: "adopt-1", ReleaseID: releaseID,
		Name: "Sales Assistant", State: "published", LocalAgentID: "agent-local",
	}).Error)
}

func TestFindDerivationResolvesVariantAdoptionAndRelease(t *testing.T) {
	db := openForkLineageDB(t)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "rel-1", TenantID: 1, ListingID: "listing-1", SubmissionID: "sub-1",
		AgentVersionID: "version-src", SourceAgentID: "agent-src", ReleaseNumber: 1,
		SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}"),
	}).Error)
	seedDerivation(t, db, "rel-1")
	repo := NewAgentMarketplaceRepository(db)

	derivation, err := repo.FindDerivation(context.Background(), 1, "agent-local")
	require.NoError(t, err)
	require.NotNil(t, derivation)
	require.Equal(t, "variant-1", derivation.Variant.ID)
	require.Equal(t, "listing-1", derivation.ListingID)
	require.NotNil(t, derivation.Release)
	require.Equal(t, "rel-1", derivation.Release.ID)

	// 未派生（无 variant 台账）的 agent 必须得到 nil, nil——原始内容没有 lineage。
	none, err := repo.FindDerivation(context.Background(), 1, "agent-original")
	require.NoError(t, err)
	require.Nil(t, none)

	// 跨租户读取必须读不到他租户的 variant。
	cross, err := repo.FindDerivation(context.Background(), 2, "agent-local")
	require.NoError(t, err)
	require.Nil(t, cross)
}

func TestFindDerivationFallsBackToIntroducedRelease(t *testing.T) {
	db := openForkLineageDB(t)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "public-rel-1", TenantID: 1, PublicListingID: "public-listing-1", PublicReleaseID: "upstream-1",
		DisplayName: "Public helper", SemanticVersion: "2.0.0", BundleDigest: "pd",
		ManifestJSON: `{"license_id":"community"}`, DependencyLockJSON: "{}", Bundle: []byte(`{"payload":{}}`),
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-p", TenantID: 1, ListingID: "public-listing-1", AcceptedReleaseID: "public-rel-1", State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-p", TenantID: 1, AdoptionID: "adopt-p", ReleaseID: "public-rel-1",
		Name: "Adopted helper", State: "published", LocalAgentID: "agent-adopted",
	}).Error)
	repo := NewAgentMarketplaceRepository(db)

	derivation, err := repo.FindDerivation(context.Background(), 1, "agent-adopted")
	require.NoError(t, err)
	require.NotNil(t, derivation)
	require.Equal(t, "public-listing-1", derivation.ListingID, "引入台账回退解析出公共 Listing id")
	require.NotNil(t, derivation.Release)
	require.Equal(t, "public-rel-1", derivation.Release.ID)
}

func TestLicenseUpsertGetAndList(t *testing.T) {
	db := openForkLineageDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()

	created, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: true, CreatedBy: "admin",
	})
	require.NoError(t, err)
	require.True(t, created.AllowsRedistribution)

	// 翻转：重新注册即更新（许可证可收紧，后续提交 live 生效）。
	updated, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: false, CreatedBy: "admin",
	})
	require.NoError(t, err)
	require.False(t, updated.AllowsRedistribution)

	got, err := repo.GetLicense(ctx, "MIT")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.AllowsRedistribution)

	missing, err := repo.GetLicense(ctx, "ghost")
	require.NoError(t, err)
	require.Nil(t, missing, "未注册许可证返回 nil，由服务层 fail closed")

	rows, err := repo.ListLicenses(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "MIT", rows[0].ID)
}

// UpsertLicense 的 ErrAgentLicenseIDRequired 防御分支直接测试（终局审查
// minor #3）：服务层 RegisterLicense 以 ErrAgentLicenseInvalid 拦截空 ID，
// 但仓储层防御独立成立——nil 指针与空白 ID（TrimSpace 后为空）都必须在此
// 被拒，不触达任何 SQL。
func TestUpsertLicenseRejectsMissingID(t *testing.T) {
	db := openForkLineageDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()

	nilRow, err := repo.UpsertLicense(ctx, nil)
	require.Nil(t, nilRow)
	require.ErrorIs(t, err, ErrAgentLicenseIDRequired)

	blankRow, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{ID: "   ", Name: "blank id"})
	require.Nil(t, blankRow)
	require.ErrorIs(t, err, ErrAgentLicenseIDRequired)

	// 防御分支未落任何行：注册表仍为空。
	rows, err := repo.ListLicenses(ctx)
	require.NoError(t, err)
	require.Empty(t, rows)
}

// TestUpsertLicensePreservesFirstRegistrar pins R5-F3: the entity contract
// says re-registering updates the FLAGS ONLY ("that is how a license flip
// propagates") — the first registrar and its created_at are audit facts and
// must survive; and the upsert must return the STORED row, never the
// caller's input copy (which would echo a fabricated registrar/time to the
// API response).
func TestUpsertLicensePreservesFirstRegistrar(t *testing.T) {
	db := openForkLineageDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()

	_, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "Apache-2.0", Name: "Apache License 2.0", AllowsRedistribution: false, CreatedBy: "first-admin",
	})
	require.NoError(t, err)
	firstRead, err := repo.GetLicense(ctx, "Apache-2.0")
	require.NoError(t, err)
	require.Equal(t, "first-admin", firstRead.CreatedBy)

	// Re-registration by a DIFFERENT actor flips the flag — the propagation
	// path — but must not rewrite WHO first registered the license nor WHEN.
	second, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "Apache-2.0", Name: "Apache License 2.0", AllowsRedistribution: true, CreatedBy: "second-admin",
	})
	require.NoError(t, err)
	require.True(t, second.AllowsRedistribution, "the flag flip must propagate")
	require.Equal(t, "first-admin", second.CreatedBy,
		"the returned row must carry the FIRST registrar, not the input copy's actor")

	stored, err := repo.GetLicense(ctx, "Apache-2.0")
	require.NoError(t, err)
	require.Equal(t, "first-admin", stored.CreatedBy, "re-registration must not overwrite the first registrar (audit)")
	require.Equal(t, firstRead.CreatedAt.UTC(), stored.CreatedAt.UTC(), "the first registration's created_at must survive")
	require.False(t, stored.UpdatedAt.Before(firstRead.UpdatedAt), "updated_at must not go backwards")
	require.True(t, stored.AllowsRedistribution, "the stored flags carry the flip")
	require.Equal(t, *stored, *second, "the upsert's return must equal the stored row — no input-copy echo")
}
