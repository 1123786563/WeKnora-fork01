package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentMarketplaceDigestMismatch  = errors.New("agent marketplace reviewed digest does not match submission")
	ErrAgentMarketplacePointerConflict = errors.New("agent marketplace listing pointer changed")
	ErrAgentMarketplaceNotFound        = errors.New("agent marketplace submission not found")
	ErrAgentMarketplaceInvalidDecision = errors.New("invalid agent marketplace review decision")
)

type AgentMarketplaceRepository interface {
	CreateSubmission(context.Context, *types.AgentMarketplaceListingEntity, *types.AgentReleaseSubmissionEntity) (*types.AgentReleaseSubmissionEntity, error)
	ListReviewQueue(context.Context, uint64) ([]types.AgentReleaseSubmissionEntity, error)
	ReviewAndPublishTx(context.Context, uint64, string, string, string, types.AgentReleaseReviewDecision) (*types.AgentReleaseReviewEntity, *types.AgentReleaseEntity, error)
	ListTenantCatalog(context.Context, uint64) ([]types.AgentMarketplaceListingEntity, error)
	GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error)
}

type agentMarketplaceRepository struct{ db *gorm.DB }

func NewAgentMarketplaceRepository(db *gorm.DB) AgentMarketplaceRepository {
	return &agentMarketplaceRepository{db: db}
}

func (r *agentMarketplaceRepository) CreateSubmission(ctx context.Context, listing *types.AgentMarketplaceListingEntity, submission *types.AgentReleaseSubmissionEntity) (*types.AgentReleaseSubmissionEntity, error) {
	if submission == nil || submission.TenantID == 0 || strings.TrimSpace(submission.AgentVersionID) == "" || strings.TrimSpace(submission.SourceAgentID) == "" || strings.TrimSpace(submission.BundleDigest) == "" || len(submission.Bundle) == 0 {
		return nil, fmt.Errorf("invalid agent marketplace submission")
	}
	var result types.AgentReleaseSubmissionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row types.AgentMarketplaceListingEntity
		err := tx.Where("tenant_id = ? AND source_agent_id = ?", submission.TenantID, submission.SourceAgentID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if listing == nil {
				return fmt.Errorf("listing details required for first submission")
			}
			row = *listing
			row.ID, row.TenantID, row.SourceAgentID = uuid.NewString(), submission.TenantID, submission.SourceAgentID
			if row.State == "" {
				row.State = "listed"
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		result = *submission
		result.ID, result.ListingID = uuid.NewString(), row.ID
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

func (r *agentMarketplaceRepository) ListReviewQueue(ctx context.Context, tenantID uint64) ([]types.AgentReleaseSubmissionEntity, error) {
	var rows []types.AgentReleaseSubmissionEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status IN ?", tenantID, []string{"submitted", "in_review"}).
		Where("NOT EXISTS (SELECT 1 FROM agent_release_reviews reviews WHERE reviews.tenant_id = agent_release_submissions.tenant_id AND reviews.submission_id = agent_release_submissions.id)").
		Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *agentMarketplaceRepository) ReviewAndPublishTx(ctx context.Context, tenantID uint64, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.AgentReleaseReviewEntity, *types.AgentReleaseEntity, error) {
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return nil, nil, ErrAgentMarketplaceInvalidDecision
	}
	if review, release, found, err := r.findReviewResult(ctx, tenantID, submissionID, expectedDigest, decision); err != nil || found {
		return review, release, err
	}
	var review *types.AgentReleaseReviewEntity
	var release *types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var submission types.AgentReleaseSubmissionEntity
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, submissionID).First(&submission).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentMarketplaceNotFound
			}
			return err
		}
		if submission.BundleDigest != expectedDigest {
			return ErrAgentMarketplaceDigestMismatch
		}
		if submission.Status != "submitted" && submission.Status != "in_review" {
			return ErrAgentMarketplaceInvalidDecision
		}
		var priorReviews int64
		if err := tx.Model(&types.AgentReleaseReviewEntity{}).Where("tenant_id = ? AND submission_id = ?", tenantID, submission.ID).Count(&priorReviews).Error; err != nil {
			return err
		}
		if priorReviews > 0 {
			return ErrAgentMarketplaceInvalidDecision
		}
		review = &types.AgentReleaseReviewEntity{ID: uuid.NewString(), TenantID: tenantID, SubmissionID: submission.ID, ReviewerID: decision.ReviewerID, ReviewedDigest: expectedDigest, Decision: decision.Decision, Reason: decision.Reason, CreatedAt: time.Now().UTC()}
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		if decision.Decision == "approved" {
			var max int
			if err := tx.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND listing_id = ?", tenantID, submission.ListingID).Select("COALESCE(MAX(release_number), 0)").Scan(&max).Error; err != nil {
				return err
			}
			release = &types.AgentReleaseEntity{ID: uuid.NewString(), TenantID: tenantID, ListingID: submission.ListingID, SubmissionID: submission.ID, AgentVersionID: submission.AgentVersionID, ReleaseNumber: max + 1, SemanticVersion: submission.SemanticVersion, BundleDigest: submission.BundleDigest, ManifestJSON: submission.ManifestJSON, DependencyLockJSON: submission.DependencyLockJSON, Bundle: append([]byte(nil), submission.Bundle...), PublishedBy: decision.ReviewerID, CreatedAt: time.Now().UTC()}
			if err := tx.Create(release).Error; err != nil {
				return err
			}
		}
		if release == nil {
			return nil
		}
		query := tx.Model(&types.AgentMarketplaceListingEntity{}).Where("tenant_id = ? AND id = ?", tenantID, submission.ListingID)
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
			return ErrAgentMarketplacePointerConflict
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			if priorReview, priorRelease, found, lookupErr := r.findReviewResult(ctx, tenantID, submissionID, expectedDigest, decision); lookupErr != nil || found {
				return priorReview, priorRelease, lookupErr
			}
		}
		return nil, nil, err
	}
	return review, release, nil
}

func (r *agentMarketplaceRepository) findReviewResult(ctx context.Context, tenantID uint64, submissionID, digest string, decision types.AgentReleaseReviewDecision) (*types.AgentReleaseReviewEntity, *types.AgentReleaseEntity, bool, error) {
	var review types.AgentReleaseReviewEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND submission_id = ? AND reviewer_id = ? AND reviewed_digest = ?", tenantID, submissionID, decision.ReviewerID, digest).First(&review).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if review.Decision != decision.Decision || review.Reason != decision.Reason {
		return nil, nil, false, ErrAgentMarketplaceInvalidDecision
	}
	var release types.AgentReleaseEntity
	err = r.db.WithContext(ctx).Where("tenant_id = ? AND submission_id = ?", tenantID, submissionID).First(&release).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &review, nil, true, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	return &review, &release, true, nil
}

func (r *agentMarketplaceRepository) ListTenantCatalog(ctx context.Context, tenantID uint64) ([]types.AgentMarketplaceListingEntity, error) {
	var rows []types.AgentMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *agentMarketplaceRepository) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, releaseID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
