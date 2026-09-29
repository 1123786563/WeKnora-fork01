package repository

// Agent-upgrade repository tests (T31 #61): find-or-create idempotence on
// the real unique index, CAS transitions, tenant scoping. Store-level SQL
// semantics under test — direct-DDL setup mirrors openAdoptionVariantDB.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openUpgradeProposalDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.AgentUpgradeProposalEntity{}, &types.AgentAdoptionEntity{}, &types.AgentMarketplaceListingEntity{}, &types.AgentReleaseEntity{}))
	// AutoMigrate 不创建 uq_agent_upgrade_proposals_scope（实体无 uniqueIndex
	// tag）；显式补建使 FindOrCreateProposal 的竞态分支由真实唯一索引驱动，
	// 与迁移 000120/000200 的生产 DDL 一致。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_upgrade_proposals_scope ON agent_upgrade_proposals(tenant_id, adoption_id, to_release_id)").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestAgentUpgradeRepositoryFindOrCreateProposalIsIdempotent(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()

	// 夹具基线：adoption 行存在；FindOrCreateProposal 只写 proposal 行，
	// 绝不触碰 adoption（AC1 的存储侧半边，HTTP 侧另一半在 Task 6 e2e）。
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		TenantID: 1, ID: "a1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin",
	}).Error)

	first, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2",
		ToSemanticVersion: "1.1.0", DiffJSON: `{"behavior":[]}`, State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, first.ID)
	require.Equal(t, "open", first.State)

	// 同键重复 materialize：返回既有行，绝不新行。
	second, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, second.ID)

	// 同一 adoption 的新目标 Release → 新行。
	third, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r3", State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEqual(t, first.ID, third.ID)

	// 跨租户同键 → 独立行（租户隔离由复合主键承载）。
	fourth, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 2, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)

	// adoption 行原封未动（Review Focus 1 的存储侧断言）。
	var adoptionAfter types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), "a1").First(&adoptionAfter).Error)
	require.Equal(t, "r1", adoptionAfter.AcceptedReleaseID, "proposal 写入不触碰 adoption 行")
	require.Equal(t, "active", adoptionAfter.State)

	rows, err := repo.ListProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 2, "tenant 1 恰好两条：r2 与 r3")
	rows2, err := repo.ListProposals(ctx, 2)
	require.NoError(t, err)
	require.Len(t, rows2, 1)
	require.Equal(t, fourth.ID, rows2[0].ID)
}

func TestAgentUpgradeRepositoryTransitionProposalCAS(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "agent1", DisplayName: "Agent", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r1", ListingID: "l1", SubmissionID: "s1", AgentVersionID: "v1", SourceAgentID: "agent1", SemanticVersion: "1.0.0", BundleDigest: "d1", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r2", ListingID: "l1", SubmissionID: "s2", AgentVersionID: "v2", SourceAgentID: "agent1", SemanticVersion: "1.1.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "a1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{
		TenantID: 1, ID: "p1", AdoptionID: "a1", ListingID: "l1",
		FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	}).Error)

	updated, err := repo.TransitionProposal(ctx, 1, "p1", []string{"open"}, "accepted",
		map[string]any{"accepted_variant_id": "v9", "resolved_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "accepted", updated.State)
	require.Equal(t, "v9", updated.AcceptedVariantID)
	require.Equal(t, "admin", updated.ResolvedBy)

	// 重放 open→accepted：期望集不再匹配，CAS 显式拒绝，不静默二次应用。
	_, err = repo.TransitionProposal(ctx, 1, "p1", []string{"open"}, "dismissed", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalTransition)

	// 终态之间互斥：accepted 不得再 dismissed。
	_, err = repo.TransitionProposal(ctx, 1, "p1", []string{"dismissed"}, "dismissed", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalTransition)

	// 不存在的行：明确 not found 哨兵。
	_, err = repo.TransitionProposal(ctx, 1, "missing", []string{"open"}, "accepted", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalNotFound)
}

func TestAgentUpgradeRepositoryTransitionProposalPreservesStorageError(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "l1", SourceAgentID: "agent1", DisplayName: "Agent", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "r2", ListingID: "l1", SubmissionID: "s2", AgentVersionID: "v2", SourceAgentID: "agent1", SemanticVersion: "1.1.0", BundleDigest: "d2", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("b")}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "a1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin"}).Error)
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{TenantID: 1, ID: "p1", AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open"}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_adoption_guard BEFORE UPDATE ON agent_adoptions BEGIN SELECT RAISE(ABORT, 'injected adoption storage failure'); END`).Error)

	_, err := repo.TransitionProposal(ctx, 1, "p1", []string{"open"}, "accepted", map[string]any{"accepted_variant_id": "v9"})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAgentUpgradeProposalTransition)
	require.NotErrorIs(t, err, ErrAgentUpgradeProposalNotFound)
	require.Contains(t, err.Error(), "injected adoption storage failure")
}

func TestAgentUpgradeRepositoryProposalTenantScope(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{
		TenantID: 1, ID: "p1", AdoptionID: "a1", ListingID: "l1",
		FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	}).Error)

	other, err := repo.GetProposal(ctx, 2, "p1")
	require.NoError(t, err)
	require.Nil(t, other, "他租户的同形 id 读取为不存在（不泄露存在性）")

	mine, err := repo.GetProposal(ctx, 1, "p1")
	require.NoError(t, err)
	require.NotNil(t, mine)
	require.Equal(t, "p1", mine.ID)
}
