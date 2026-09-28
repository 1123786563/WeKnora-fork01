package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrPublicMarketplaceNotFound            = errors.New("public marketplace resource not found")
	ErrPublicMarketplaceInvalidDecision     = errors.New("invalid public marketplace review decision")
	ErrPublicMarketplaceReviewConflict      = errors.New("public marketplace submission already has a review")
	ErrPublicMarketplaceDigestMismatch      = errors.New("public marketplace reviewed digest does not match submission")
	ErrPublicMarketplacePointerConflict     = errors.New("public marketplace listing pointer changed")
	ErrPublicMarketplaceReleaseConflict     = errors.New("public marketplace release conflict")
	ErrPublicMarketplaceLifecycleTransition = errors.New("public marketplace lifecycle transition failed")
	ErrPublicMarketplaceLifecycleInvalid    = errors.New("invalid public marketplace lifecycle request")
)

// PublicCatalogRow pairs a discoverable public listing with its current
// release. It deliberately carries NO adopter-derived data (spec §12).
type PublicCatalogRow struct {
	Listing types.PublicMarketplaceListingEntity
	Release *types.PublicAgentReleaseEntity
}

// PublicMarketplaceRepository owns the public catalog and introduction
// ledger SQL. Platform tables are not tenant-scoped; the adopter-side
// introduction ledger always binds the adopter tenant id. Every statement
// is parameter-bound.
type PublicMarketplaceRepository interface {
	VerifyPublisher(ctx context.Context, row *types.VerifiedPublisherEntity) (*types.VerifiedPublisherEntity, bool, error)
	RevokePublisher(ctx context.Context, tenantID uint64) error
	ListVerifiedPublishers(ctx context.Context) ([]types.VerifiedPublisherEntity, error)
	GetVerifiedPublisher(ctx context.Context, tenantID uint64) (*types.VerifiedPublisherEntity, error)
	IsPublicListingDiscoverable(ctx context.Context, listingID string) (bool, error)
	CreatePublicSubmission(ctx context.Context, listing *types.PublicMarketplaceListingEntity, submission *types.PublicReleaseSubmissionEntity) (*types.PublicReleaseSubmissionEntity, error)
	ListPublicSubmissions(ctx context.Context, publisherTenantID uint64) ([]types.PublicReleaseSubmissionEntity, error)
	ListPublicReviewQueue(ctx context.Context) ([]types.PublicReleaseSubmissionEntity, error)
	GetPublicSubmission(ctx context.Context, submissionID string) (*types.PublicReleaseSubmissionEntity, error)
	GetPublicListing(ctx context.Context, listingID string) (*types.PublicMarketplaceListingEntity, error)
	GetPublicRelease(ctx context.Context, releaseID string) (*types.PublicAgentReleaseEntity, error)
	ReviewAndPublishPublicTx(ctx context.Context, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.PublicReleaseReviewEntity, *types.PublicAgentReleaseEntity, error)
	ListPublicCatalog(ctx context.Context) ([]PublicCatalogRow, error)
	IntroduceRelease(ctx context.Context, adopterTenantID uint64, actorID string, listing *types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) (*types.TenantIntroducedReleaseEntity, *types.AgentAdoptionEntity, bool, error)
	UnlistPublicListing(ctx context.Context, listingID, actorID, reason string) (*types.PublicMarketplaceListingEntity, error)
	DeprecatePublicRelease(ctx context.Context, releaseID, replacementReleaseID, actorID, reason string) (*types.PublicAgentReleaseEntity, error)
}

type publicMarketplaceRepository struct{ db *gorm.DB }

func NewPublicMarketplaceRepository(db *gorm.DB) PublicMarketplaceRepository {
	return &publicMarketplaceRepository{db: db}
}

// UnlistPublicListing records a platform catalog transition without changing
// the publisher's tenant Listing, Releases, or existing introductions.
func (r *publicMarketplaceRepository) UnlistPublicListing(ctx context.Context, listingID, actorID, reason string) (*types.PublicMarketplaceListingEntity, error) {
	listingID, actorID, reason = strings.TrimSpace(listingID), strings.TrimSpace(actorID), strings.TrimSpace(reason)
	if listingID == "" || actorID == "" || reason == "" {
		return nil, ErrPublicMarketplaceLifecycleInvalid
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&types.PublicMarketplaceListingEntity{}).
		Where("id = ? AND state = ?", listingID, "listed").
		Updates(map[string]any{"state": "unlisted", "unlisted_by": actorID, "unlisted_at": now, "unlist_reason": reason, "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&types.PublicMarketplaceListingEntity{}).Where("id = ?", listingID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, ErrPublicMarketplaceNotFound
		}
		return nil, ErrPublicMarketplaceLifecycleTransition
	}
	return r.GetPublicListing(ctx, listingID)
}

// DeprecatePublicRelease writes lane-specific platform lifecycle metadata.
// The public Release content remains immutable and the replacement must have
// the same Public Listing key.
func (r *publicMarketplaceRepository) DeprecatePublicRelease(ctx context.Context, releaseID, replacementID, actorID, reason string) (*types.PublicAgentReleaseEntity, error) {
	releaseID, replacementID = strings.TrimSpace(releaseID), strings.TrimSpace(replacementID)
	actorID, reason = strings.TrimSpace(actorID), strings.TrimSpace(reason)
	if releaseID == "" || replacementID == "" || releaseID == replacementID || actorID == "" || reason == "" {
		return nil, ErrPublicMarketplaceLifecycleInvalid
	}
	var changed types.PublicAgentReleaseEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current types.PublicAgentReleaseEntity
		if err := tx.Where("id = ?", releaseID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceNotFound
			}
			return err
		}
		var replacement types.PublicAgentReleaseEntity
		if err := tx.Where("listing_id = ? AND id = ?", current.ListingID, replacementID).First(&replacement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceLifecycleInvalid
			}
			return err
		}
		now := time.Now().UTC()
		updated := tx.Model(&types.PublicAgentReleaseEntity{}).Where("id = ? AND deprecated_at IS NULL", releaseID).
			Updates(map[string]any{"deprecated_by": actorID, "deprecated_at": now, "deprecation_reason": reason, "replacement_release_id": replacementID})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrPublicMarketplaceLifecycleTransition
		}
		return tx.Where("id = ?", releaseID).First(&changed).Error
	})
	if err != nil {
		return nil, err
	}
	return &changed, nil
}

func (r *publicMarketplaceRepository) VerifyPublisher(ctx context.Context, row *types.VerifiedPublisherEntity) (*types.VerifiedPublisherEntity, bool, error) {
	if row == nil || row.TenantID == 0 {
		return nil, false, fmt.Errorf("invalid verified publisher row")
	}
	var existing types.VerifiedPublisherEntity
	var created bool
	err := withTenantSecurityGuard(ctx, r.db, row.TenantID, func(tx *gorm.DB) error {
		err := tx.Where("tenant_id = ?", row.TenantID).First(&existing).Error
		if err == nil {
			if existing.State == "verified" && existing.VerifiedBy == row.VerifiedBy {
				return nil
			}
			existing.State = "verified"
			existing.VerifiedBy = row.VerifiedBy
			existing.Note = row.Note
			existing.VerifiedAt = row.VerifiedAt
			existing.UpdatedAt = time.Now().UTC()
			if err := tx.Model(&types.VerifiedPublisherEntity{}).Where("tenant_id = ?", row.TenantID).
				Updates(map[string]any{"state": existing.State, "verified_by": existing.VerifiedBy, "note": existing.Note, "verified_at": existing.VerifiedAt, "updated_at": existing.UpdatedAt}).Error; err != nil {
				return err
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		newRow := *row
		if newRow.State == "" {
			newRow.State = "verified"
		}
		newRow.UpdatedAt = newRow.VerifiedAt
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&newRow)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			created = true
			existing = newRow
			return nil
		}
		return tx.Where("tenant_id = ?", row.TenantID).First(&existing).Error
	})
	if err != nil {
		return nil, false, err
	}
	return &existing, created, nil
}

func (r *publicMarketplaceRepository) RevokePublisher(ctx context.Context, tenantID uint64) error {
	err := withTenantSecurityGuard(ctx, r.db, tenantID, func(tx *gorm.DB) error {
		updated := tx.Model(&types.VerifiedPublisherEntity{}).
			Where("tenant_id = ?", tenantID).
			Updates(map[string]any{"state": "revoked", "updated_at": time.Now().UTC()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrPublicMarketplaceNotFound
		}
		return nil
	})
	if errors.Is(err, ErrTenantNotFound) {
		return ErrPublicMarketplaceNotFound
	}
	return err
}

func (r *publicMarketplaceRepository) ListVerifiedPublishers(ctx context.Context) ([]types.VerifiedPublisherEntity, error) {
	rows := []types.VerifiedPublisherEntity{}
	err := r.db.WithContext(ctx).Order("verified_at ASC, tenant_id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) GetVerifiedPublisher(ctx context.Context, tenantID uint64) (*types.VerifiedPublisherEntity, error) {
	var row types.VerifiedPublisherEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) CreatePublicSubmission(ctx context.Context, listing *types.PublicMarketplaceListingEntity, submission *types.PublicReleaseSubmissionEntity) (*types.PublicReleaseSubmissionEntity, error) {
	if submission == nil || submission.PublisherTenantID == 0 || strings.TrimSpace(submission.SourceListingID) == "" || strings.TrimSpace(submission.SourceReleaseID) == "" || strings.TrimSpace(submission.BundleDigest) == "" || len(submission.Bundle) == 0 {
		return nil, fmt.Errorf("invalid public marketplace submission")
	}
	var result types.PublicReleaseSubmissionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source types.AgentReleaseEntity
		if err := tx.Where("tenant_id = ? AND id = ?", submission.PublisherTenantID, submission.SourceReleaseID).First(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceNotFound
			}
			return err
		}
		var row types.PublicMarketplaceListingEntity
		err := tx.Where("publisher_tenant_id = ? AND source_listing_id = ?", submission.PublisherTenantID, submission.SourceListingID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if listing == nil {
				return fmt.Errorf("public listing details required for first submission")
			}
			row = types.PublicMarketplaceListingEntity{
				ID: uuid.NewString(), PublisherTenantID: submission.PublisherTenantID, SourceListingID: submission.SourceListingID,
				DisplayName: listing.DisplayName, Summary: listing.Summary, State: "listed",
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		result = *submission
		result.ID, result.PublicListingID = uuid.NewString(), row.ID
		if result.Status == "" {
			result.Status = "submitted"
		}
		result.CreatedAt = time.Now().UTC()
		return tx.Create(&result).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *publicMarketplaceRepository) ListPublicSubmissions(ctx context.Context, publisherTenantID uint64) ([]types.PublicReleaseSubmissionEntity, error) {
	rows := []types.PublicReleaseSubmissionEntity{}
	err := r.db.WithContext(ctx).Where("publisher_tenant_id = ?", publisherTenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) ListPublicReviewQueue(ctx context.Context) ([]types.PublicReleaseSubmissionEntity, error) {
	rows := []types.PublicReleaseSubmissionEntity{}
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{"submitted", "in_review"}).
		Where("NOT EXISTS (SELECT 1 FROM public_release_reviews reviews WHERE reviews.submission_id = public_release_submissions.id)").
		Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) GetPublicSubmission(ctx context.Context, submissionID string) (*types.PublicReleaseSubmissionEntity, error) {
	var row types.PublicReleaseSubmissionEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(submissionID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) GetPublicListing(ctx context.Context, listingID string) (*types.PublicMarketplaceListingEntity, error) {
	var row types.PublicMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(listingID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) IsPublicListingDiscoverable(ctx context.Context, listingID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("public_marketplace_listings AS l").
		Joins("JOIN public_marketplace_verified_publishers AS p ON p.tenant_id = l.publisher_tenant_id AND p.state = ?", "verified").
		Joins("JOIN agent_marketplace_listings AS source ON source.tenant_id = l.publisher_tenant_id AND source.id = l.source_listing_id AND source.state = ?", "listed").
		Where("l.id = ? AND l.state = ? AND l.current_release_id IS NOT NULL", strings.TrimSpace(listingID), "listed").Count(&count).Error
	return count == 1, err
}

func (r *publicMarketplaceRepository) GetPublicRelease(ctx context.Context, releaseID string) (*types.PublicAgentReleaseEntity, error) {
	var row types.PublicAgentReleaseEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) ReviewAndPublishPublicTx(ctx context.Context, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.PublicReleaseReviewEntity, *types.PublicAgentReleaseEntity, error) {
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return nil, nil, ErrPublicMarketplaceInvalidDecision
	}
	var review *types.PublicReleaseReviewEntity
	var release *types.PublicAgentReleaseEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var submission types.PublicReleaseSubmissionEntity
		if err := tx.Where("id = ?", strings.TrimSpace(submissionID)).First(&submission).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceNotFound
			}
			return err
		}
		if submission.BundleDigest != expectedDigest {
			return ErrPublicMarketplaceDigestMismatch
		}
		if submission.Status != "submitted" && submission.Status != "in_review" {
			return ErrPublicMarketplaceInvalidDecision
		}
		review = &types.PublicReleaseReviewEntity{ID: uuid.NewString(), SubmissionID: submission.ID, ReviewerID: decision.ReviewerID, ReviewedDigest: expectedDigest, Decision: decision.Decision, Reason: decision.Reason, CreatedAt: time.Now().UTC()}
		if err := tx.Create(review).Error; err != nil {
			// 唯一 review 约束（uq_public_release_review_submission，000114:81）
			// 冲突 = 该 submission 已有审核决定。
			if isUniqueViolation(err) {
				return ErrPublicMarketplaceReviewConflict
			}
			return err
		}
		if decision.Decision != "approved" {
			return nil
		}
		var max int
		if err := tx.Model(&types.PublicAgentReleaseEntity{}).Where("listing_id = ?", submission.PublicListingID).Select("COALESCE(MAX(release_number), 0)").Scan(&max).Error; err != nil {
			return err
		}
		release = &types.PublicAgentReleaseEntity{
			ID: uuid.NewString(), ListingID: submission.PublicListingID, SubmissionID: submission.ID,
			PublisherTenantID: submission.PublisherTenantID, ReleaseNumber: max + 1,
			SemanticVersion: submission.SemanticVersion, BundleDigest: submission.BundleDigest,
			ManifestJSON: submission.ManifestJSON, DependencyLockJSON: submission.DependencyLockJSON,
			Bundle: append([]byte(nil), submission.Bundle...), PublishedBy: decision.ReviewerID, CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(release).Error; err != nil {
			// release 唯一索引（uq_public_agent_releases_number/semantic/digest，
			// 000114:82-84）冲突 = 与既有公共 release 撞号/版本/摘要，必须与
			// review 冲突区分（fix round 1：原统一出口把这类冲突误标为
			// ReviewConflict，令 ErrPublicMarketplaceReleaseConflict 不可达）。
			if isUniqueViolation(err) {
				return ErrPublicMarketplaceReleaseConflict
			}
			return err
		}
		query := tx.Model(&types.PublicMarketplaceListingEntity{}).Where("id = ?", submission.PublicListingID)
		if expectedPriorReleaseID == "" {
			query = query.Where("current_release_id IS NULL")
		} else {
			query = query.Where("current_release_id = ?", expectedPriorReleaseID)
		}
		updated := query.Updates(map[string]any{"current_release_id": release.ID, "updated_at": time.Now().UTC()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrPublicMarketplacePointerConflict
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return review, release, nil
}

func (r *publicMarketplaceRepository) ListPublicCatalog(ctx context.Context) ([]PublicCatalogRow, error) {
	rows := []types.PublicMarketplaceListingEntity{}
	if err := r.db.WithContext(ctx).Table("public_marketplace_listings AS l").Select("l.*").
		Joins("JOIN public_marketplace_verified_publishers AS p ON p.tenant_id = l.publisher_tenant_id AND p.state = ?", "verified").
		Joins("JOIN agent_marketplace_listings AS source ON source.tenant_id = l.publisher_tenant_id AND source.id = l.source_listing_id AND source.state = ?", "listed").
		Where("l.state = ? AND l.current_release_id IS NOT NULL", "listed").Order("l.created_at ASC, l.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PublicCatalogRow, 0, len(rows))
	for i := range rows {
		release, err := r.GetPublicRelease(ctx, *rows[i].CurrentReleaseID)
		if err != nil {
			return nil, err
		}
		out = append(out, PublicCatalogRow{Listing: rows[i], Release: release})
	}
	return out, nil
}

// IntroduceRelease is the cross-tenant propagation primitive: it copies ONE
// approved public release's portable content verbatim into the adopter
// tenant's introduction ledger and creates/updates the tenant's Adoption
// for that public listing, all in one transaction. Concurrent introduces of
// the same (tenant, public release) converge on the winner's row.
func (r *publicMarketplaceRepository) IntroduceRelease(ctx context.Context, adopterTenantID uint64, actorID string, listing *types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) (*types.TenantIntroducedReleaseEntity, *types.AgentAdoptionEntity, bool, error) {
	var introduced types.TenantIntroducedReleaseEntity
	var adoption *types.AgentAdoptionEntity
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// All cross-tenant paths acquire tenant guards in numeric order. This
		// serializes against source Listing unlist / Publisher revocation and
		// avoids opposite-order deadlocks when publisher == adopter.
		first, second := adopterTenantID, listing.PublisherTenantID
		if first > second {
			first, second = second, first
		}
		if err := acquireTenantSecurityGuardTx(tx, first); err != nil {
			return err
		}
		if second != first {
			if err := acquireTenantSecurityGuardTx(tx, second); err != nil {
				return err
			}
		}
		// Listing is always the first lifecycle gate. UnlistPublicListing CASes
		// this exact row, so PostgreSQL serializes the operations and SQLite
		// holds its writer lock until the full transaction commits.
		listingGate := tx.Model(&types.PublicMarketplaceListingEntity{}).
			Where("id = ? AND state = ?", listing.ID, "listed").
			UpdateColumn("updated_at", gorm.Expr("updated_at"))
		if listingGate.Error != nil {
			return listingGate.Error
		}
		if listingGate.RowsAffected != 1 {
			return ErrPublicMarketplaceLifecycleTransition
		}
		// Custody recheck inside the guarded transaction: the Publisher must
		// still be verified and the source tenant Listing must still be listed.
		var visible int64
		if err := tx.Table("public_marketplace_listings AS l").Joins("JOIN public_marketplace_verified_publishers AS p ON p.tenant_id = l.publisher_tenant_id AND p.state = ?", "verified").
			Joins("JOIN agent_marketplace_listings AS source ON source.tenant_id = l.publisher_tenant_id AND source.id = l.source_listing_id AND source.state = ?", "listed").
			Where("l.id = ? AND l.publisher_tenant_id = ? AND l.state = ? AND l.current_release_id IS NOT NULL", listing.ID, listing.PublisherTenantID, "listed").Count(&visible).Error; err != nil {
			return err
		}
		if visible != 1 {
			return ErrPublicMarketplaceNotFound
		}
		var persistedRelease types.PublicAgentReleaseEntity
		if err := tx.Where("listing_id = ? AND id = ?", listing.ID, release.ID).Take(&persistedRelease).Error; err != nil {
			return ErrPublicMarketplaceReleaseConflict
		}
		err := tx.Where("tenant_id = ? AND public_release_id = ?", adopterTenantID, release.ID).First(&introduced).Error
		if err == nil {
			adoption, _, err = adoptListingTx(tx, &types.AgentAdoptionEntity{
				TenantID: adopterTenantID, ListingID: listing.ID, AcceptedReleaseID: introduced.ID,
				State: "active", CreatedBy: actorID,
			})
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		introduced = types.TenantIntroducedReleaseEntity{
			ID: uuid.NewString(), TenantID: adopterTenantID, PublicListingID: listing.ID, PublicReleaseID: release.ID,
			DisplayName: listing.DisplayName, Summary: listing.Summary, SemanticVersion: release.SemanticVersion,
			BundleDigest: release.BundleDigest, ManifestJSON: release.ManifestJSON,
			DependencyLockJSON: release.DependencyLockJSON, Bundle: append([]byte(nil), release.Bundle...),
			IntroducedBy: actorID, IntroducedAt: time.Now().UTC(),
		}
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&introduced)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			created = true
		} else {
			var winner types.TenantIntroducedReleaseEntity
			if err := tx.Where("tenant_id = ? AND public_release_id = ?", adopterTenantID, release.ID).First(&winner).Error; err != nil {
				return err
			}
			introduced = winner
		}
		adoption, _, err = adoptListingTx(tx, &types.AgentAdoptionEntity{
			TenantID: adopterTenantID, ListingID: listing.ID, AcceptedReleaseID: introduced.ID,
			State: "active", CreatedBy: actorID,
		})
		return err
	})
	if err != nil {
		return nil, nil, false, err
	}
	return &introduced, adoption, created, nil
}
