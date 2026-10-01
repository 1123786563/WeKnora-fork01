package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openPublicMarketplaceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseSubmissionEntity{},
		&types.AgentReleaseReviewEntity{}, &types.AgentReleaseEntity{},
		&types.VerifiedPublisherEntity{}, &types.PublicMarketplaceListingEntity{},
		&types.PublicReleaseSubmissionEntity{}, &types.PublicReleaseReviewEntity{},
		&types.PublicAgentReleaseEntity{}, &types.TenantIntroducedReleaseEntity{},
		&types.AgentAdoptionEntity{}, &types.AgentAdoptionVariantEntity{},
		&types.AgentReleaseRevocationEntity{},
	))
	// AutoMigrate 不创建实体未带 uniqueIndex tag 的唯一索引；显式补建使
	// adoptListingTx/IntroduceRelease 的 OnConflict 竞态分支、ReviewConflict
	// 的唯一 review 收敛与 ReleaseConflict 的 release 唯一性（number/semantic/
	// digest）由真实唯一索引驱动，与迁移 000113/000114 的生产 DDL 一致
	// （000113:31、000114:81-84）。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_public_release_review_submission ON public_release_reviews(submission_id)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_public_agent_releases_number ON public_agent_releases(listing_id, release_number)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_public_agent_releases_semantic ON public_agent_releases(listing_id, semantic_version)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_public_agent_releases_digest ON public_agent_releases(listing_id, bundle_digest)").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func digestOf(bundle []byte) string { sum := sha256.Sum256(bundle); return hex.EncodeToString(sum[:]) }

// sourceReleaseNumber derives a stable per-version release_number so
// repeated seeds against the same listing never collide with the real
// uq_agent_releases_number(listing_id, release_number) unique index
// (migrations/sqlite/000109_tenant_agent_marketplace.up.sql) that the
// AutoMigrate schema of this test does NOT carry.
func sourceReleaseNumber(semanticVersion string) int {
	fields := strings.Split(semanticVersion, ".")
	major, err := strconv.Atoi(fields[0])
	if err != nil || major < 1 {
		return 1
	}
	return major
}

// seedApprovedPublicRelease seeds one REAL tenant source release (the
// CreatePublicSubmission tx verifies it exists in agent_releases) and walks
// it through public submission + platform approval.
func seedApprovedPublicRelease(t *testing.T, db *gorm.DB, semanticVersion string) (listing types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) {
	t.Helper()
	ctx := context.Background()
	repo := NewPublicMarketplaceRepository(db)
	bundle := []byte(`{"payload":{"system_prompt":"Be portable."},"manifest":{"semantic_version":"` + semanticVersion + `"},"dependency_lock":{"dependencies":[]}}`)
	sourceReleaseID := "tenant-release-" + semanticVersion
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: sourceReleaseID, TenantID: 1, ListingID: "tenant-listing-1", SubmissionID: "tenant-submission-" + semanticVersion,
		AgentVersionID: "version-x-" + semanticVersion, SourceAgentID: "agent-a", ReleaseNumber: sourceReleaseNumber(semanticVersion), SemanticVersion: semanticVersion,
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
	}).Error)
	listing = types.PublicMarketplaceListingEntity{PublisherTenantID: 1, SourceListingID: "tenant-listing-1", DisplayName: "Public helper", Summary: "s", State: "listed"}
	submission, err := repo.CreatePublicSubmission(ctx, &listing, &types.PublicReleaseSubmissionEntity{
		PublisherTenantID: 1, SourceListingID: "tenant-listing-1", SourceReleaseID: sourceReleaseID,
		PublisherActorID: "publisher-admin", SemanticVersion: semanticVersion, BundleDigest: digestOf(bundle),
		ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
	})
	require.NoError(t, err)
	// CAS prior 指针取公共 listing 当前值：首个版本为 NULL 传 ""，后续
	// 版本必须传当前指针，否则与「prior 指针错误必须拒绝」的 CAS 语义
	// 相撞（ReviewAndPublishAdvancesPointerOnce 断言的行为）。
	var seededListing types.PublicMarketplaceListingEntity
	require.NoError(t, db.Where("publisher_tenant_id = ? AND source_listing_id = ?", 1, "tenant-listing-1").First(&seededListing).Error)
	prior := ""
	if seededListing.CurrentReleaseID != nil {
		prior = *seededListing.CurrentReleaseID
	}
	_, created, err := repo.ReviewAndPublishPublicTx(ctx, prior, submission.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "platform-reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, created)
	loaded, err := repo.GetPublicListing(ctx, created.ListingID)
	require.NoError(t, err)
	return *loaded, created
}

func TestPublicMarketplaceRepositoryVerifyPublisherLifecycle(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()

	created, isFirst, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin", Note: "identity checked"})
	require.NoError(t, err)
	require.True(t, isFirst)
	require.Equal(t, "verified", created.State)

	again, isFirstRepeat, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin"})
	require.NoError(t, err)
	require.False(t, isFirstRepeat)
	require.Equal(t, "verified", again.State)

	require.NoError(t, repo.RevokePublisher(ctx, 7))
	row, err := repo.GetVerifiedPublisher(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "revoked", row.State)

	revived, revivedFirst, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin-2"})
	require.NoError(t, err)
	require.False(t, revivedFirst, "重验证是更新既有行，不是新建")
	require.Equal(t, "verified", revived.State)

	require.ErrorIs(t, repo.RevokePublisher(ctx, 42), ErrPublicMarketplaceNotFound)
}

func TestPublicMarketplaceRepositoryReviewAndPublishAdvancesPointerOnce(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")
	require.NotNil(t, listing.CurrentReleaseID)
	require.Equal(t, release.ID, *listing.CurrentReleaseID)
	require.Equal(t, 1, release.ReleaseNumber)

	// 同一 submission 不得被二次审核决定（唯一 review 约束收敛为显式冲突）
	_, _, err := repo.ReviewAndPublishPublicTx(ctx, "", release.SubmissionID, release.BundleDigest, types.AgentReleaseReviewDecision{ReviewerID: "platform-reviewer-2", Decision: "rejected", Reason: "too risky"})
	require.ErrorIs(t, err, ErrPublicMarketplaceReviewConflict)

	// digest 与 submission 不一致必须拒绝
	bundle := []byte(`{"payload":{"system_prompt":"v2"},"manifest":{"semantic_version":"2.0.0"},"dependency_lock":{"dependencies":[]}}`)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "tenant-release-2.0.0", TenantID: 1, ListingID: "tenant-listing-1", SubmissionID: "tenant-submission-2",
		AgentVersionID: "version-x-2.0.0", SourceAgentID: "agent-a", ReleaseNumber: 2, SemanticVersion: "2.0.0",
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
	}).Error)
	sub2, err := repo.CreatePublicSubmission(ctx, nil, &types.PublicReleaseSubmissionEntity{
		PublisherTenantID: 1, PublicListingID: listing.ID, SourceListingID: "tenant-listing-1", SourceReleaseID: "tenant-release-2.0.0",
		SemanticVersion: "2.0.0", BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
	})
	require.NoError(t, err)
	_, _, err = repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, sub2.ID, "0"+"1", types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplaceDigestMismatch)

	// 指针 CAS：prior 指针错误必须拒绝
	_, _, err = repo.ReviewAndPublishPublicTx(ctx, "", sub2.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplacePointerConflict)

	_, release2, err := repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, sub2.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.NoError(t, err)
	require.Equal(t, 2, release2.ReleaseNumber)
}

// TestPublicMarketplaceRepositoryReviewConflictMapsToReviewNotRelease pins
// the review-insert unique violation to ErrPublicMarketplaceReviewConflict
// now that release-insert violations map to ErrPublicMarketplaceReleaseConflict.
func TestPublicMarketplaceRepositoryReviewConflictMapsToReviewNotRelease(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")

	// 同一 submission 二次审核决定：review 表唯一索引冲突必须收敛为
	// ReviewConflict（不是 ReleaseConflict，也不是裸唯一索引错误）。
	_, _, err := repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, release.SubmissionID, release.BundleDigest, types.AgentReleaseReviewDecision{ReviewerID: "platform-reviewer-2", Decision: "rejected", Reason: "too risky"})
	require.ErrorIs(t, err, ErrPublicMarketplaceReviewConflict)
	require.NotErrorIs(t, err, ErrPublicMarketplaceReleaseConflict)
}

// TestPublicMarketplaceRepositoryReleaseUniquenessConflictIsNotMisreadAsReview
// pins the 000114:82-84 release unique indexes (number/semantic/digest) to
// ErrPublicMarketplaceReleaseConflict: a release-insert collision must NOT be
// reported as "submission already has a review".
func TestPublicMarketplaceRepositoryReleaseUniquenessConflictIsNotMisreadAsReview(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, _ := seedApprovedPublicRelease(t, db, "1.0.0")

	bundle := []byte(`{"payload":{"system_prompt":"v2"},"manifest":{"semantic_version":"2.0.0"},"dependency_lock":{"dependencies":[]}}`)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "tenant-release-2.0.0", TenantID: 1, ListingID: "tenant-listing-1", SubmissionID: "tenant-submission-2",
		AgentVersionID: "version-x-2.0.0", SourceAgentID: "agent-a", ReleaseNumber: 2, SemanticVersion: "2.0.0",
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
	}).Error)
	sub2, err := repo.CreatePublicSubmission(ctx, nil, &types.PublicReleaseSubmissionEntity{
		PublisherTenantID: 1, PublicListingID: listing.ID, SourceListingID: "tenant-listing-1", SourceReleaseID: "tenant-release-2.0.0",
		SemanticVersion: "2.0.0", BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
	})
	require.NoError(t, err)

	// 注入一行与待发布 release 同 bundle_digest 的既有公共 release（semantic
	// 与 number 均刻意错开），使 uq_public_agent_releases_digest（000114:84）
	// 在 release 插入点冲突。
	injected := types.PublicAgentReleaseEntity{
		ID: "injected-release-dup", ListingID: listing.ID, SubmissionID: "injected-submission",
		PublisherTenantID: 1, ReleaseNumber: 99, SemanticVersion: "2.0.0-dup",
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
		PublishedBy: "platform-reviewer", CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&injected).Error)

	_, _, err = repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, sub2.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplaceReleaseConflict, "release 唯一索引冲突必须收敛为 ReleaseConflict")
	require.NotErrorIs(t, err, ErrPublicMarketplaceReviewConflict, "不得误标为「已有 review」")

	// 失败安全：事务回滚，不残留新 release/review 行（仅 seed 1 行 + 注入 1 行）。
	var releaseCount, reviewCount int64
	db.Table("public_agent_releases").Where("listing_id = ?", listing.ID).Count(&releaseCount)
	db.Table("public_release_reviews").Where("submission_id = ?", sub2.ID).Count(&reviewCount)
	require.Equal(t, int64(2), releaseCount)
	require.Zero(t, reviewCount)
}

func TestPublicMarketplaceRepositoryIntroduceReleaseCopiesPortableBundleAndAdopts(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")

	introduced, adoption, created, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, release.Bundle, introduced.Bundle, "引入=可移植 Release 逐字节复制")
	require.Equal(t, release.BundleDigest, introduced.BundleDigest)
	require.Equal(t, uint64(2), introduced.TenantID)
	require.NotNil(t, adoption)
	require.Equal(t, listing.ID, adoption.ListingID, "Adoption 引用公共 Listing 身份")
	require.Equal(t, introduced.ID, adoption.AcceptedReleaseID)
	require.Equal(t, "active", adoption.State)

	// 同一 (tenant, public release) 幂等：二次引入不新建行、不新建 Adoption
	again, againAdoption, againCreated, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.NoError(t, err)
	require.False(t, againCreated)
	require.Equal(t, introduced.ID, again.ID)
	require.Equal(t, adoption.ID, againAdoption.ID)

	// 引入更新的公共 release：同一 Adoption，accepted 指针前进
	listing2, release2 := seedApprovedPublicRelease(t, db, "2.0.0")
	require.Equal(t, listing.ID, listing2.ID)
	_, advanced, advancedCreated, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing2, release2)
	require.NoError(t, err)
	require.True(t, advancedCreated)
	require.Equal(t, adoption.ID, advanced.ID, "Adoption 唯一，不产生第二个")
	// accepted 指针前进到新引入的本地行（Adoption.AcceptedReleaseID 引用
	// tenant_introduced_releases.ID，同类型注释与本测试首个引入断言）。
	var advancedIntroduced types.TenantIntroducedReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND public_release_id = ?", 2, release2.ID).First(&advancedIntroduced).Error)
	require.Equal(t, advancedIntroduced.ID, advanced.AcceptedReleaseID)
	require.Equal(t, release2.ID, advancedIntroduced.PublicReleaseID)
	_, err = NewAgentAdoptionRepository(db).EndAdoption(ctx, 2, adoption.ID, "tenant-admin", "closed")
	require.NoError(t, err)
	_, _, _, err = repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.ErrorIs(t, err, ErrAgentAdoptionTransition, "same-release re-introduction cannot revive an ended Adoption")
	_, _, _, err = repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing2, release2)
	require.ErrorIs(t, err, ErrAgentAdoptionTransition, "a different Release cannot advance an ended Adoption")

	// 其他租户互不可见
	var count int64
	db.Table("tenant_introduced_releases").Where("tenant_id = ?", 3).Count(&count)
	require.Zero(t, count)
}

func TestPublicMarketplaceRepositoryRejectsIntroductionAfterUnlist(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")
	_, err := repo.UnlistPublicListing(ctx, listing.ID, "platform-admin", "closed")
	require.NoError(t, err)
	_, _, _, err = repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.ErrorIs(t, err, ErrPublicMarketplaceLifecycleTransition)
	var introductions, adoptions int64
	require.NoError(t, db.Model(&types.TenantIntroducedReleaseEntity{}).Where("tenant_id = ?", 2).Count(&introductions).Error)
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ?", 2).Count(&adoptions).Error)
	require.Zero(t, introductions)
	require.Zero(t, adoptions)
}
