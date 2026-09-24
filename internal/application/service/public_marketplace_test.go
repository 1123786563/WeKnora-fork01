package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPublicMarketplaceServiceForTest(t *testing.T) (*PublicMarketplaceService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	listings := repository.NewAgentMarketplaceRepository(db)
	return NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), listings), db
}

// differentDigest flips the last hex char of a valid digest, yielding a
// DIFFERENT value that still passes the sha-256 format check.
func differentDigest(digest string) string {
	last := digest[len(digest)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}
	return digest[:len(digest)-1] + replacement
}

// seedTenantRelease publishes a REAL tenant release (tenant 1) whose
// Manifest requires the capability "knowledge", mirroring the #59 service
// test seeding (including the agent_versions row CreateSubmission verifies).
func seedTenantRelease(t *testing.T, db *gorm.DB) (listingID, releaseID, digest string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES ('version-a', 1, 'agent-a', 1, '{}', 'sha', 'publisher-admin')`,
	).Error)
	manifest := `{"semantic_version":"1.0.0","display_name":"Public helper","summary":"Portable","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be portable.","allowed_tools":[]},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	sum := sha256.Sum256(bundle)
	digest = hex.EncodeToString(sum[:])
	repo := repository.NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Public helper", Summary: "Portable", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", AuthorID: "publisher-admin",
			SemanticVersion: "1.0.0", BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "tenant-reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID, digest
}

func TestPublicMarketplaceServiceSubmitRequiresVerifiedPublisher(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()

	_, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotVerifiedPublisher)

	var count int64
	db.Table("public_release_submissions").Count(&count)
	require.Zero(t, count, "未验证发布者不得留下任何 submission 行")

	_, _, err = svc.VerifyPublisher(ctx, "sysadmin", 1, "identity checked")
	require.NoError(t, err)
	view, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	require.Equal(t, "submitted", view.Status)
	require.Equal(t, listingID, view.SourceListingID)
	require.NotEmpty(t, view.PublicListingID)
	require.NotEmpty(t, view.BundleDigest)
}

func TestPublicMarketplaceServiceSubmitCopiesPortableReleaseVerbatim(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, releaseID, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)

	view, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	var source types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), releaseID).First(&source).Error)
	require.Equal(t, source.Bundle, view.Bundle, "公共提交=源可移植 Release 逐字节复制")
	require.Equal(t, source.BundleDigest, view.BundleDigest)
	require.Equal(t, source.ManifestJSON, view.ManifestJSON)

	// 源 Release 字节被篡改（digest 列未变）：提交默认指向 current release，
	// 必须在写库前被 digest 校验拒绝（fail closed）
	tampered := []byte(`{"payload":{"system_prompt":"evil"}}`)
	require.NoError(t, db.Exec("UPDATE agent_releases SET bundle = ? WHERE tenant_id = ? AND id = ?", tampered, uint64(1), releaseID).Error)
	_, err = svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.Error(t, err, "digest 与 bundle 不一致必须拒绝")
	require.ErrorIs(t, err, ErrPublicMarketplaceStaleDigest)
}

func TestPublicMarketplaceServiceReviewApprovesAndFillsCatalog(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)

	// 拒绝必须给理由；digest 必须与 submission 一致
	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "rejected"})
	require.ErrorIs(t, err, ErrPublicMarketplaceInvalidInput)
	// stale digest：与记录摘要必然不同的合法 sha-256 形态（末位十六进制翻转）
	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, differentDigest(submission.BundleDigest), types.AgentReleaseReviewDecision{Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplaceStaleDigest)

	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, result.Review)
	require.NotNil(t, result.Release)

	catalog, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	require.Equal(t, result.Release.ID, catalog[0].CurrentRelease.ID)
	require.Equal(t, uint64(1), catalog[0].PublisherTenantID)
	require.True(t, catalog[0].PublisherVerified, "已验证发布者必须在目录可见")

	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer-2", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "rejected", Reason: "late"})
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceReviewConflict, "同一 submission 只能被决定一次")
}

func TestPublicMarketplaceServiceAdoptPropagatesPortableReleaseAndFeedsVariantChain(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)

	// 跨租户引入：tenant 2（无任何本地 listing/release）引入公共 Listing
	adopted, created, err := svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, "")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, result.Release.ID, adopted.Introduction.PublicReleaseID)
	require.Equal(t, result.Release.Bundle, adopted.Introduction.Bundle)
	require.Equal(t, adopted.Introduction.ID, adopted.Adoption.AcceptedReleaseID)
	require.Equal(t, result.Release.ListingID, adopted.Adoption.ListingID)

	// 幂等：同一公共 release 二次引入不新建
	_, createdAgain, err := svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, result.Release.ID)
	require.NoError(t, err)
	require.False(t, createdAgain)

	// #59 链条在引入 release 上原样可用：Variant 固定引入 release，Manifest 能力需求可读
	adoptions := NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), &fakeAdoptionAgentSource{db: db}, fakeAdoptionVersions{})
	variant, err := adoptions.CreateVariant(ctx, 2, "adopter-admin", adopted.Adoption.ID, interfaces.VariantDraftInput{Name: "Imported helper"})
	require.NoError(t, err)
	require.Equal(t, adopted.Introduction.ID, variant.ReleaseID)
	require.Equal(t, []string{"knowledge"}, variant.MissingCapabilities, "Manifest 能力需求经引入台账可读")

	// 未知公共 listing / release 一律 NotFound
	_, _, err = svc.AdoptPublicListing(ctx, 2, "adopter-admin", "no-such-listing", "")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)
	_, _, err = svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, "no-such-release")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)

	// 隐私：目录读模型不含采用方痕迹（结构保证，此处断言行数不因引入而增长字段）
	entries, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, result.Release.ListingID, entries[0].ListingID)
}
