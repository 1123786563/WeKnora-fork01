package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrPublicMarketplaceInvalidInput         = errors.New("invalid public marketplace request")
	ErrPublicMarketplaceNotFound             = errors.New("public marketplace resource not found")
	ErrPublicMarketplaceNotVerifiedPublisher = errors.New("tenant is not a verified publisher")
	ErrPublicMarketplaceStaleDigest          = errors.New("public marketplace digest is stale or does not match the recorded bundle")
)

const (
	VerifiedPublisherStateVerified = "verified"
	PublicListingStateListed       = "listed"
)

type PublicMarketplaceService struct {
	repo     repository.PublicMarketplaceRepository
	listings interfaces.AgentMarketplaceRepository
	now      func() time.Time
}

var _ interfaces.PublicMarketplaceService = (*PublicMarketplaceService)(nil)

func NewPublicMarketplaceService(repo repository.PublicMarketplaceRepository, listings interfaces.AgentMarketplaceRepository) *PublicMarketplaceService {
	return &PublicMarketplaceService{repo: repo, listings: listings, now: time.Now}
}

func (s *PublicMarketplaceService) VerifyPublisher(ctx context.Context, actorID string, tenantID uint64, note string) (interfaces.VerifiedPublisherView, bool, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" || tenantID == 0 {
		return interfaces.VerifiedPublisherView{}, false, ErrPublicMarketplaceInvalidInput
	}
	row, created, err := s.repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{
		TenantID: tenantID, State: VerifiedPublisherStateVerified, VerifiedBy: actorID, Note: strings.TrimSpace(note), VerifiedAt: s.now().UTC(),
	})
	if err != nil {
		return interfaces.VerifiedPublisherView{}, false, err
	}
	return interfaces.VerifiedPublisherView{VerifiedPublisherEntity: *row}, created, nil
}

func (s *PublicMarketplaceService) RevokePublisher(ctx context.Context, actorID string, tenantID uint64) error {
	if strings.TrimSpace(actorID) == "" || tenantID == 0 {
		return ErrPublicMarketplaceInvalidInput
	}
	return s.repo.RevokePublisher(ctx, tenantID)
}

func (s *PublicMarketplaceService) ListVerifiedPublishers(ctx context.Context) ([]interfaces.VerifiedPublisherView, error) {
	rows, err := s.repo.ListVerifiedPublishers(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.VerifiedPublisherView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.VerifiedPublisherView{VerifiedPublisherEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) requireVerifiedPublisher(ctx context.Context, tenantID uint64) error {
	row, err := s.repo.GetVerifiedPublisher(ctx, tenantID)
	if err != nil {
		return err
	}
	if row == nil || row.State != VerifiedPublisherStateVerified {
		return ErrPublicMarketplaceNotVerifiedPublisher
	}
	return nil
}

func (s *PublicMarketplaceService) SubmitPublicRelease(ctx context.Context, tenantID uint64, actorID, sourceListingID, releaseID string) (interfaces.PublicSubmissionView, error) {
	actorID, sourceListingID, releaseID = strings.TrimSpace(actorID), strings.TrimSpace(sourceListingID), strings.TrimSpace(releaseID)
	if tenantID == 0 || actorID == "" || sourceListingID == "" {
		return interfaces.PublicSubmissionView{}, ErrPublicMarketplaceInvalidInput
	}
	if err := s.requireVerifiedPublisher(ctx, tenantID); err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	// 源读取走 #58 marketplace 仓储（无引入回退）：只有本租户自有 listing/release
	// 可以被提升为公共提交，杜绝把别人发布的内容冒名再提交。
	listing, err := s.listings.GetListing(ctx, tenantID, sourceListingID)
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	if listing == nil {
		return interfaces.PublicSubmissionView{}, ErrPublicMarketplaceNotFound
	}
	if listing.State != "listed" {
		return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: listing state is %q", ErrPublicMarketplaceInvalidInput, listing.State)
	}
	resolvedReleaseID := releaseID
	if resolvedReleaseID == "" {
		if listing.CurrentReleaseID == nil {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: listing has no current release", ErrPublicMarketplaceInvalidInput)
		}
		resolvedReleaseID = *listing.CurrentReleaseID
	}
	release, err := s.listings.GetRelease(ctx, tenantID, resolvedReleaseID)
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: release does not belong to the source listing", ErrPublicMarketplaceInvalidInput)
	}
	if err := verifyBundleDigest(release.Bundle, release.BundleDigest); err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	// T32 #62 AC2（公共 lane）：带 lineage 的 Release 再次分发前必须 live
	// 通过许可证注册表——租户 lane 发布时的放行不缓存（注册表可翻转）。
	if strings.TrimSpace(release.LineageLicenseID) != "" {
		license, err := s.listings.GetLicense(ctx, release.LineageLicenseID)
		if err != nil {
			return interfaces.PublicSubmissionView{}, err
		}
		if license == nil {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: source lineage license %q is not registered in the license registry", ErrReleaseRedistributionForbidden, release.LineageLicenseID)
		}
		if !license.AllowsRedistribution {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: source lineage license %q does not permit redistribution", ErrReleaseRedistributionForbidden, release.LineageLicenseID)
		}
	}
	created, err := s.repo.CreatePublicSubmission(ctx,
		&types.PublicMarketplaceListingEntity{DisplayName: listing.DisplayName, Summary: listing.Summary},
		&types.PublicReleaseSubmissionEntity{
			PublisherTenantID: tenantID, SourceListingID: listing.ID, SourceReleaseID: release.ID,
			PublisherActorID: actorID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
			ManifestJSON: release.ManifestJSON, DependencyLockJSON: release.DependencyLockJSON,
			Bundle: append([]byte(nil), release.Bundle...), Status: "submitted",
		})
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	return interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: *created}, nil
}

func (s *PublicMarketplaceService) ListPublicSubmissions(ctx context.Context, tenantID uint64) ([]interfaces.PublicSubmissionView, error) {
	if tenantID == 0 {
		return nil, ErrPublicMarketplaceInvalidInput
	}
	rows, err := s.repo.ListPublicSubmissions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.PublicSubmissionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) ListPublicReviewQueue(ctx context.Context) ([]interfaces.PublicSubmissionView, error) {
	rows, err := s.repo.ListPublicReviewQueue(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.PublicSubmissionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) ReviewPublicSubmission(ctx context.Context, reviewerID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (interfaces.PublicReviewResult, error) {
	reviewerID, submissionID, expectedDigest = strings.TrimSpace(reviewerID), strings.TrimSpace(submissionID), strings.ToLower(strings.TrimSpace(expectedDigest))
	decision.Decision, decision.Reason = strings.TrimSpace(decision.Decision), strings.TrimSpace(decision.Reason)
	if reviewerID == "" || submissionID == "" || !validMarketplaceDigest(expectedDigest) {
		return interfaces.PublicReviewResult{}, ErrPublicMarketplaceInvalidInput
	}
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return interfaces.PublicReviewResult{}, fmt.Errorf("%w: invalid review decision", ErrPublicMarketplaceInvalidInput)
	}
	if decision.Decision != "approved" && decision.Reason == "" {
		return interfaces.PublicReviewResult{}, fmt.Errorf("%w: a reason is required", ErrPublicMarketplaceInvalidInput)
	}
	decision.ReviewerID = reviewerID
	submission, err := s.repo.GetPublicSubmission(ctx, submissionID)
	if err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	if submission == nil {
		return interfaces.PublicReviewResult{}, repository.ErrPublicMarketplaceNotFound
	}
	if submission.BundleDigest != expectedDigest {
		return interfaces.PublicReviewResult{}, ErrPublicMarketplaceStaleDigest
	}
	if err := verifyBundleDigest(submission.Bundle, submission.BundleDigest); err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	priorReleaseID := ""
	if decision.Decision == "approved" {
		listing, err := s.repo.GetPublicListing(ctx, submission.PublicListingID)
		if err != nil {
			return interfaces.PublicReviewResult{}, err
		}
		if listing == nil {
			return interfaces.PublicReviewResult{}, repository.ErrPublicMarketplaceNotFound
		}
		if listing.CurrentReleaseID != nil {
			priorReleaseID = *listing.CurrentReleaseID
		}
	}
	review, release, err := s.repo.ReviewAndPublishPublicTx(ctx, priorReleaseID, submissionID, expectedDigest, decision)
	if err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	return interfaces.PublicReviewResult{Review: review, Release: release}, nil
}

func (s *PublicMarketplaceService) ListPublicCatalog(ctx context.Context) ([]interfaces.PublicCatalogEntryView, error) {
	rows, err := s.repo.ListPublicCatalog(ctx)
	if err != nil {
		return nil, err
	}
	verified := map[uint64]bool{}
	publishers, err := s.repo.ListVerifiedPublishers(ctx)
	if err != nil {
		return nil, err
	}
	for _, publisher := range publishers {
		if publisher.State == VerifiedPublisherStateVerified {
			verified[publisher.TenantID] = true
		}
	}
	views := make([]interfaces.PublicCatalogEntryView, 0, len(rows))
	for _, row := range rows {
		if row.Listing.State != PublicListingStateListed || row.Release == nil {
			continue
		}
		views = append(views, interfaces.PublicCatalogEntryView{
			ListingID: row.Listing.ID, DisplayName: row.Listing.DisplayName, Summary: row.Listing.Summary,
			State: row.Listing.State, PublisherTenantID: row.Listing.PublisherTenantID,
			PublisherVerified: verified[row.Listing.PublisherTenantID],
			CurrentRelease: &interfaces.PublicReleaseSummary{
				ID: row.Release.ID, SemanticVersion: row.Release.SemanticVersion, BundleDigest: row.Release.BundleDigest,
				ManifestJSON: row.Release.ManifestJSON, DependencyLockJSON: row.Release.DependencyLockJSON, CreatedAt: row.Release.CreatedAt,
			},
			CreatedAt: row.Listing.CreatedAt, UpdatedAt: row.Listing.UpdatedAt,
		})
	}
	return views, nil
}

func (s *PublicMarketplaceService) GetPublicListing(ctx context.Context, listingID string) (*interfaces.PublicListingDetailView, error) {
	listingID = strings.TrimSpace(listingID)
	if listingID == "" {
		return nil, ErrPublicMarketplaceInvalidInput
	}
	listing, err := s.repo.GetPublicListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if listing == nil {
		return nil, repository.ErrPublicMarketplaceNotFound
	}
	// 目录可见性与 ListPublicCatalog 同口径（state=listed 且已批准 release）：
	// 首个 submission 落库到平台批准之间 current_release_id 为 NULL，listing
	// 行虽已存在，detail 不得提前暴露 display_name/summary/publisher_tenant_id。
	// repo 层不加此过滤——ReviewPublicSubmission 首个批准时必须读到 NULL 指针行。
	discoverable, err := s.repo.IsPublicListingDiscoverable(ctx, listing.ID)
	if err != nil {
		return nil, err
	}
	if !discoverable || listing.State != PublicListingStateListed || listing.CurrentReleaseID == nil {
		return nil, repository.ErrPublicMarketplaceNotFound
	}
	entry := interfaces.PublicCatalogEntryView{
		ListingID: listing.ID, DisplayName: listing.DisplayName, Summary: listing.Summary,
		State: listing.State, PublisherTenantID: listing.PublisherTenantID,
		CreatedAt: listing.CreatedAt, UpdatedAt: listing.UpdatedAt,
	}
	publisher, err := s.repo.GetVerifiedPublisher(ctx, listing.PublisherTenantID)
	if err != nil {
		return nil, err
	}
	entry.PublisherVerified = publisher != nil && publisher.State == VerifiedPublisherStateVerified
	if listing.CurrentReleaseID != nil {
		release, err := s.repo.GetPublicRelease(ctx, *listing.CurrentReleaseID)
		if err != nil {
			return nil, err
		}
		if release != nil {
			entry.CurrentRelease = &interfaces.PublicReleaseSummary{
				ID: release.ID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
				ManifestJSON: release.ManifestJSON, DependencyLockJSON: release.DependencyLockJSON, CreatedAt: release.CreatedAt,
			}
		}
	}
	return &interfaces.PublicListingDetailView{PublicCatalogEntryView: entry}, nil
}

func (s *PublicMarketplaceService) AdoptPublicListing(ctx context.Context, tenantID uint64, actorID, listingID, releaseID string) (interfaces.PublicAdoptionResult, bool, error) {
	actorID, listingID, releaseID = strings.TrimSpace(actorID), strings.TrimSpace(listingID), strings.TrimSpace(releaseID)
	if tenantID == 0 || actorID == "" || listingID == "" {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceInvalidInput
	}
	listing, err := s.repo.GetPublicListing(ctx, listingID)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	if listing == nil || listing.State != PublicListingStateListed {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceNotFound
	}
	discoverable, err := s.repo.IsPublicListingDiscoverable(ctx, listing.ID)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	if !discoverable {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceNotFound
	}
	resolvedReleaseID := releaseID
	if resolvedReleaseID == "" {
		if listing.CurrentReleaseID == nil {
			return interfaces.PublicAdoptionResult{}, false, fmt.Errorf("%w: listing has no current release", ErrPublicMarketplaceInvalidInput)
		}
		resolvedReleaseID = *listing.CurrentReleaseID
	}
	release, err := s.repo.GetPublicRelease(ctx, resolvedReleaseID)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceNotFound
	}
	// AC1 传播完整性：公共 release 的字节必须仍与记录摘要一致，否则零传播。
	if err := verifyBundleDigest(release.Bundle, release.BundleDigest); err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	introduced, adoption, created, err := s.repo.IntroduceRelease(ctx, tenantID, actorID, listing, release)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	return interfaces.PublicAdoptionResult{Introduction: *introduced, Adoption: *adoption}, created, nil
}

// verifyBundleDigest fails closed on any byte drift between the bundle and
// its recorded digest — the shared guard for submit, review and introduce.
func verifyBundleDigest(bundle []byte, digest string) error {
	sum := sha256.Sum256(bundle)
	if hex.EncodeToString(sum[:]) != digest {
		return ErrPublicMarketplaceStaleDigest
	}
	return nil
}
