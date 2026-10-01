package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

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
	evaluations := NewAgentEvaluationService(repository.NewAgentEvaluationRepository(db))
	metrics := NewMarketplaceMetricsService(repository.NewMarketplaceMetricsRepository(db))
	return NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), listings, evaluations, metrics), db
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
	require.NoError(t, db.Exec(`INSERT OR IGNORE INTO tenants (id, name, business) VALUES (1, 'publisher', 'test')`).Error)
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
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
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

// TestPublicMarketplaceServiceGetListingMatchesCatalogVisibility pins the
// detail endpoint to the SAME catalog visibility as ListPublicCatalog
// (state=listed AND current release approved). The listing row already
// exists between the first submission landing and platform approval
// (state=listed, current_release_id NULL) — detail must not leak
// display_name/summary/publisher_tenant_id ahead of approval.
func TestPublicMarketplaceServiceGetListingMatchesCatalogVisibility(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)

	// 审批窗口内：listing 行已落库（state=listed，current_release_id NULL），
	// detail 必须与目录列表同口径 404，不得提前暴露元数据
	var row types.PublicMarketplaceListingEntity
	require.NoError(t, db.Where("id = ?", submission.PublicListingID).First(&row).Error)
	require.Equal(t, "listed", row.State, "前置：窗口期 listing 行 state 确为 listed")
	require.Nil(t, row.CurrentReleaseID, "前置：窗口期 current_release_id 确为 NULL")
	detail, err := svc.GetPublicListing(ctx, submission.PublicListingID)
	require.Nil(t, detail)
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceNotFound)
	catalog, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog, "同刻目录列表也为空：detail 与列表口径一致")

	// 平台批准后：detail 可见且带 current release（正常路径不被误伤）
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	detail, err = svc.GetPublicListing(ctx, submission.PublicListingID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, result.Release.ListingID, detail.ListingID)
	require.NotNil(t, detail.CurrentRelease)
	require.Equal(t, result.Release.ID, detail.CurrentRelease.ID)
	require.Equal(t, uint64(1), detail.PublisherTenantID)
	require.True(t, detail.PublisherVerified)

	// state 离开 listed（如 #63 unlist）后 detail 同样 404
	require.NoError(t, db.Model(&types.PublicMarketplaceListingEntity{}).Where("id = ?", submission.PublicListingID).Update("state", "unlisted").Error)
	detail, err = svc.GetPublicListing(ctx, submission.PublicListingID)
	require.Nil(t, detail)
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceNotFound)

	// 未知 id 保持 404 语义
	detail, err = svc.GetPublicListing(ctx, "no-such-listing")
	require.Nil(t, detail)
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceNotFound)
}

// TestPublicMarketplaceServiceRecordEvaluationDelegatesToEvaluationService pins
// the T35 #65 Task 4 delegation seam: RecordEvaluation forwards to the T1
// AgentEvaluationService (evaluator identity from the authenticated reviewer)
// and surfaces its invalid/conflict errors unchanged.
func TestPublicMarketplaceServiceRecordEvaluationDelegatesToEvaluationService(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)

	valid := types.AgentEvaluationEntity{
		ReleaseID: result.Release.ID, TestSetID: "ts-suite", TestSetVersion: "v1", EnvironmentClass: "standard",
		EvaluatedAt: time.Now().UTC(),
		ResultsJSON: `{"status":"pass","checks":[{"code":"manifest_completeness","status":"pass"},{"code":"privacy","status":"pass"}]}`,
	}
	view, err := svc.RecordEvaluation(ctx, "sysadmin-evaluator", valid)
	require.NoError(t, err)
	require.Equal(t, "sysadmin-evaluator", view.EvaluatorID)
	require.Equal(t, result.Release.ID, view.ReleaseID)
	var rows int64
	db.Table("agent_release_evaluations").Where("release_id = ?", result.Release.ID).Count(&rows)
	require.EqualValues(t, 1, rows)

	duplicate := valid
	duplicate.EvaluatedAt = valid.EvaluatedAt.Add(time.Second)
	_, err = svc.RecordEvaluation(ctx, "sysadmin-evaluator", duplicate)
	require.ErrorIs(t, err, repository.ErrAgentEvaluationConflict)

	invalid := valid
	invalid.TestSetID = "ts-suite-2"
	invalid.ResultsJSON = `{"status":"pass","checks":[{"code":"evil","status":"pass"}]}`
	_, err = svc.RecordEvaluation(ctx, "sysadmin-evaluator", invalid)
	require.ErrorIs(t, err, repository.ErrAgentEvaluationInvalid)

	emptyReviewer := valid
	emptyReviewer.TestSetID = "ts-suite-3"
	_, err = svc.RecordEvaluation(ctx, "  ", emptyReviewer)
	require.ErrorIs(t, err, ErrAgentEvaluationInvalidInput)
}

// TestPublicMarketplaceServiceComposesTrustAndMetricsIntoCatalogAndDetail
// pins the T35 #65 Task 4 read-model composition: catalog/detail entries
// carry the safe review summary (never Reason), the manifest compatibility
// floor/capability/license projection, the immutable Evaluation summary and
// the threshold-bucketed MarketplaceMetricsView.
func TestPublicMarketplaceServiceComposesTrustAndMetricsIntoCatalogAndDetail(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved", Reason: "MK-SVC-REASON"})
	require.NoError(t, err)
	_, err = svc.RecordEvaluation(ctx, "sysadmin-evaluator", types.AgentEvaluationEntity{
		ReleaseID: result.Release.ID, TestSetID: "ts-suite", TestSetVersion: "v1", EnvironmentClass: "standard",
		EvaluatedAt: time.Now().UTC(),
		ResultsJSON: `{"status":"pass","checks":[{"code":"license","status":"pass"}]}`,
	})
	require.NoError(t, err)

	adoptAs := func(tenantID uint64) {
		t.Helper()
		require.NoError(t, db.Exec(`INSERT OR IGNORE INTO tenants (id, name, business) VALUES (?, 'adopter', 'test')`, tenantID).Error)
		_, _, err := svc.AdoptPublicListing(ctx, tenantID, "adopter-admin", result.Release.ListingID, "")
		require.NoError(t, err)
	}
	for tenantID := uint64(2); tenantID <= 5; tenantID++ {
		adoptAs(tenantID)
	}

	catalog, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	entry := catalog[0]
	require.NotNil(t, entry.CurrentReleaseReview)
	require.Equal(t, "approved", entry.CurrentReleaseReview.Decision)
	require.Equal(t, "platform-reviewer", entry.CurrentReleaseReview.ReviewerID)
	require.False(t, entry.CurrentReleaseReview.ReviewedAt.IsZero())
	require.NotNil(t, entry.Metrics)
	require.Equal(t, "suppressed", entry.Metrics.IntroductionsBucket, "4 adopters 低于阈值必须抑制")
	require.Len(t, entry.Evaluations, 1)
	require.Equal(t, result.Release.ID, entry.Evaluations[0].ReleaseID)
	require.Equal(t, "ts-suite", entry.Evaluations[0].TestSetID)

	require.Equal(t, "1", entry.CurrentRelease.MinimumWeKnoraCapability)
	require.Equal(t, []string{"knowledge"}, entry.CurrentRelease.CapabilityRequirements)
	require.Equal(t, "MIT", entry.CurrentRelease.LicenseID)

	detail, err := svc.GetPublicListing(ctx, result.Release.ListingID)
	require.NoError(t, err)
	require.NotNil(t, detail.CurrentReleaseReview)
	require.Equal(t, "approved", detail.CurrentReleaseReview.Decision)
	require.NotNil(t, detail.Metrics)
	require.Equal(t, "suppressed", detail.Metrics.IntroductionsBucket)
	require.Len(t, detail.Evaluations, 1)

	// 第五个采用租户翻转桶位；无升级提案的度量保持抑制。
	adoptAs(6)
	catalog, err = svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	require.Equal(t, "5-9", catalog[0].Metrics.IntroductionsBucket)
	require.Equal(t, "5-9", catalog[0].Metrics.ActiveAdoptersBucket)
	require.Equal(t, "suppressed", catalog[0].Metrics.UpgradeProposalsBucket)
	require.Equal(t, "suppressed", catalog[0].Metrics.AcceptedUpgradesBucket)
	require.Equal(t, "not_collected", catalog[0].Metrics.ErrorCategoryAvailability)

	// Review Reason 永不进入读模型（结构保证 + 序列化证据）。
	encoded, err := json.Marshal(catalog[0])
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "MK-SVC-REASON")
	require.NotContains(t, string(encoded), `"Reason"`)
}

func TestPublicMarketplaceServiceCustodyHidesRevokedPublisher(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	introduced, _, err := svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, "")
	require.NoError(t, err)
	require.NoError(t, svc.RevokePublisher(ctx, "sysadmin", 1))
	catalog, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog)
	detail, err := svc.GetPublicListing(ctx, result.Release.ListingID)
	require.Nil(t, detail)
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceNotFound)
	_, _, err = svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, result.Release.ID)
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)
	var preserved types.TenantIntroducedReleaseEntity
	require.NoError(t, db.First(&preserved, "tenant_id = ? AND public_release_id = ?", 2, result.Release.ID).Error)
	require.Equal(t, introduced.Introduction.ID, preserved.ID)
	require.Equal(t, result.Release.Bundle, preserved.Bundle)
}
