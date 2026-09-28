package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// T34 (#64) Task 1: 迁移↔投影对齐——两张撤回台账表必须由生产迁移轨道
// （migrations/sqlite 全量 Up，经 openRunTestDB）创建，列集与
// types.AgentReleaseRevocationEntity / AgentDependencyRevocationEntity 对齐。
func TestAgentSecurityRevocationTablesExistAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("agent_release_revocations"),
		"agent_release_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "listing_id", "release_id", "reason",
		"replacement_release_id", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_release_revocations", column),
			"agent_release_revocations.%s 必须存在（与实体列对齐）", column)
	}
	require.True(t, db.Migrator().HasTable("agent_dependency_revocations"),
		"agent_dependency_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "dep_type", "dep_id", "dep_version", "dep_digest",
		"reason", "replacement_version", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_dependency_revocations", column),
			"agent_dependency_revocations.%s 必须存在（与实体列对齐）", column)
	}
}

func TestAgentSecurityStoreAppendAndListKeepsHistoryTenantScoped(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()

	first := &types.AgentReleaseRevocationEntity{TenantID: 1, ListingID: "l1", ReleaseID: "r1",
		Reason: "CVE-2026-0001 prompt exfiltration", ReplacementReleaseID: "r2",
		InFlightDisposition: "cancel", RevokedBy: "sec-admin"}
	require.NoError(t, store.AppendReleaseRevocation(ctx, first))
	second := &types.AgentReleaseRevocationEntity{TenantID: 1, ListingID: "l1", ReleaseID: "r1",
		Reason: "expanded: subagent path also affected", InFlightDisposition: "allow", RevokedBy: "sec-admin-2"}
	require.NoError(t, store.AppendDependencyRevocation(ctx, &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1",
		Reason: "malicious exfil in pinned skill", ReplacementVersion: "1.2.4", RevokedBy: "sec-admin"}))
	require.NoError(t, store.AppendReleaseRevocation(ctx, second))
	providedRevokedAt := time.Date(2099, 9, 1, 2, 3, 4, 0, time.UTC)
	providedCreatedAt := time.Date(2099, 9, 1, 1, 2, 3, 0, time.UTC)
	provided := &types.AgentReleaseRevocationEntity{ID: "provided-id", TenantID: 1, ListingID: "l1", ReleaseID: "r3",
		Reason: "preserve caller facts", RevokedAt: providedRevokedAt, CreatedAt: providedCreatedAt}
	require.NoError(t, store.AppendReleaseRevocation(ctx, provided))

	rows, err := store.ListReleaseRevocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 3, "撤回历史 append-only：同一 Release 重复撤回保留两条记录")
	require.Equal(t, first.ID, rows[0].ID, "列表按 created_at ASC")
	require.NotEmpty(t, rows[0].ID, "Append 必须填充 ID")
	require.False(t, rows[0].RevokedAt.IsZero(), "Append 必须填充 RevokedAt")
	require.Equal(t, providedRevokedAt, rows[2].RevokedAt, "Append 保留调用方指定的撤回时间")
	require.Equal(t, providedCreatedAt, rows[2].CreatedAt, "Append 保留调用方指定的创建时间")

	deps, err := store.ListDependencyRevocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, "web-search", deps[0].DepID)
	require.NotEmpty(t, deps[0].ID)
	require.False(t, deps[0].RevokedAt.IsZero())

	foreignReleases, err := store.ListReleaseRevocations(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, foreignReleases, "跨租户列表为空")
	foreignDeps, err := store.ListDependencyRevocations(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, foreignDeps)
	dep, err := store.GetDependencyRevocation(ctx, 1, deps[0].ID)
	require.NoError(t, err)
	require.NotNil(t, dep)
	foreignDep, err := store.GetDependencyRevocation(ctx, 2, deps[0].ID)
	require.NoError(t, err)
	require.Nil(t, foreignDep)
	require.NoError(t, store.UpdateDependencyRevocationCanceled(ctx, 1, deps[0].ID, 4))
	updatedDep, err := store.GetDependencyRevocation(ctx, 1, deps[0].ID)
	require.NoError(t, err)
	require.EqualValues(t, 4, updatedDep.CanceledRunCount)

	got, err := store.GetReleaseRevocation(ctx, 1, second.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	miss, err := store.GetReleaseRevocation(ctx, 2, second.ID)
	require.NoError(t, err)
	require.Nil(t, miss, "跨租户单读与不存在同形")

	require.NoError(t, store.UpdateReleaseRevocationCanceled(ctx, 1, second.ID, 7))
	updated, err := store.GetReleaseRevocation(ctx, 1, second.ID)
	require.NoError(t, err)
	require.EqualValues(t, 7, updated.CanceledRunCount)
}

func TestAgentSecurityStoreReleaseFactsCoversLocalAndIntroducedReleases(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()

	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-sec", "1.0.0")
	listing, lockJSON, found, err := store.ReleaseFacts(ctx, 1, releaseID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, listingID, listing)
	require.JSONEq(t, `{"dependencies":[]}`, lockJSON)

	publicRepo := NewPublicMarketplaceRepository(db)
	const publicVersion = "2.0.0"
	bundle := []byte(`{"manifest":{"semantic_version":"2.0.0"},"dependency_lock":{"dependencies":[]}}`)
	submission, err := publicRepo.CreatePublicSubmission(ctx,
		&types.PublicMarketplaceListingEntity{PublisherTenantID: 1, SourceListingID: listingID, DisplayName: "Introduced release"},
		&types.PublicReleaseSubmissionEntity{PublisherTenantID: 1, SourceListingID: listingID, SourceReleaseID: releaseID,
			PublisherActorID: "publisher", SemanticVersion: publicVersion, BundleDigest: "public-digest",
			ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted"})
	require.NoError(t, err)
	_, publicRelease, err := publicRepo.ReviewAndPublishPublicTx(ctx, "", submission.ID, "public-digest",
		types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	publicListing, err := publicRepo.GetPublicListing(ctx, submission.PublicListingID)
	require.NoError(t, err)
	require.NotNil(t, publicListing)
	introduced, _, created, err := publicRepo.IntroduceRelease(ctx, 1, "admin", publicListing, publicRelease)
	require.NoError(t, err)
	require.True(t, created)
	// A same-ID introduction ledger row is an impossible normal-path state,
	// but legacy/import collisions must resolve to the tenant-local release.
	collisionPublicRelease := *publicRelease
	collisionPublicRelease.ID = "public-collision-release"
	collisionPublicRelease.ReleaseNumber++
	collisionPublicRelease.SemanticVersion = "2.0.1"
	collisionPublicRelease.BundleDigest = "public-collision-digest"
	require.NoError(t, db.Create(&collisionPublicRelease).Error)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: releaseID, TenantID: 1, PublicListingID: publicListing.ID, PublicReleaseID: collisionPublicRelease.ID,
		DisplayName: "colliding introduction", SemanticVersion: "2.0.0", BundleDigest: "collision",
		ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[{"type":"skill","id":"wrong"}]}`,
		Bundle: []byte(`{}`), IntroducedBy: "admin",
	}).Error)
	introListing, introLock, found, err := store.ReleaseFacts(ctx, 1, introduced.ID)
	require.NoError(t, err)
	require.True(t, found, "引入式 Release 不得被漏检（#60 台账）")
	require.Equal(t, publicListing.ID, introListing)
	require.Contains(t, introLock, `"dependencies"`)

	_, _, found, err = store.ReleaseFacts(ctx, 2, releaseID)
	require.NoError(t, err)
	require.False(t, found, "跨租户 miss")

	locks, err := store.ListTenantReleaseLocks(ctx, 1)
	require.NoError(t, err)
	require.Len(t, locks, 2, "锁清单只包含预期的本地与引入式 Release")
	localReleaseCount := 0
	introducedReleaseCount := 0
	for _, row := range locks {
		if row.ReleaseID == releaseID {
			localReleaseCount++
		}
		if row.ReleaseID == introduced.ID {
			introducedReleaseCount++
		}
	}
	require.Equal(t, 1, localReleaseCount, "本地 Release 恰好出现一次")
	require.Equal(t, 1, introducedReleaseCount, "引入式 Release 恰好出现一次")
	byID := map[string]AgentReleaseLockRow{}
	for _, row := range locks {
		byID[row.ReleaseID] = row
	}
	require.Contains(t, byID, releaseID, "本地 Release 进入锁清单")
	require.Contains(t, byID, introduced.ID, "引入式 Release 进入锁清单")
	require.Equal(t, listingID, byID[releaseID].ListingID, "ID 冲突时本地 Release 优先")
	require.JSONEq(t, `{"dependencies":[]}`, byID[releaseID].LockJSON)
	require.Contains(t, byID[introduced.ID].LockJSON, `"dependencies"`)
}

func TestAgentSecurityStoreVariantsByLocalAgentTenantScoped(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	ctx := context.Background()
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-sec2", "1.0.0")
	adoptions := NewAgentAdoptionRepository(db)
	adoption, _, err := adoptions.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	variant, err := adoptions.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "V", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	_, err = adoptions.UpdateVariantState(ctx, 1, variant.ID, []string{"draft"}, "published", map[string]any{"local_agent_id": "local-agent-1", "local_agent_version_id": "ver-1", "published_by": "admin"})
	require.NoError(t, err)

	rows, err := store.VariantsByLocalAgent(ctx, 1, "local-agent-1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, variant.ID, rows[0].ID)
	foreign, err := store.VariantsByLocalAgent(ctx, 2, "local-agent-1")
	require.NoError(t, err)
	require.Empty(t, foreign, "跨租户查询为空")
	all, err := store.ListVariants(ctx, 1)
	require.NoError(t, err)
	require.Len(t, all, 1)
}
