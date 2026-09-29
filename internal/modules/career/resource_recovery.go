package career

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

func (o *Office) markCatalogCleanupPending(ctx context.Context, row sourceRevision, ref string) error {
	scope, err := getScope(ctx)
	if err != nil {
		return err
	}
	category := strings.TrimPrefix(row.ErrorCategory, "cleanup_pending_")
	if category == "" {
		category = "interrupted"
	}
	res := o.db.WithContext(ctx).Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='failed' AND claim_token=? AND resource_ref=''", scope.TenantID, scope.UserID, row.ID, row.ClaimToken).Updates(map[string]any{"resource_ref": ref, "error_category": "cleanup_pending_" + category})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrUploadClaimLost
	}
	return nil
}

// catalogCandidates uses only server-issued source identity and the saved
// upload digest. A catalog registration remains a durable locator even when
// the separate Career source-ref update did not complete.
func (o *Office) catalogCandidates(ctx context.Context, sourceID string) (sourceRevision, []types.StoredResource, error) {
	return o.catalogCandidatesWithStates(ctx, sourceID, false)
}

func (o *Office) catalogCandidatesWithStates(ctx context.Context, sourceID string, includeDeleting bool) (sourceRevision, []types.StoredResource, error) {
	row, err := o.privateSource(ctx, sourceID)
	if err != nil {
		return sourceRevision{}, nil, err
	}
	scope, err := getScope(ctx)
	if err != nil {
		return sourceRevision{}, nil, err
	}
	name := "career_source_" + row.ID + strings.ToLower(filepath.Ext(row.FileName))
	var candidates []types.StoredResource
	query := o.db.WithContext(ctx).Where("tenant_id=? AND original_name=? AND content_hash=? AND size=? AND lifecycle=?", scope.TenantID, name, row.Digest, row.Size, types.ResourceLifecyclePersistent)
	if includeDeleting {
		query = query.Where("state IN (?,?)", types.ResourceStateActive, types.ResourceStateDeleting)
	} else {
		query = query.Where("state=?", types.ResourceStateActive)
	}
	err = query.Order("created_at, id").Find(&candidates).Error
	return row, candidates, err
}

func (o *Office) candidateHasForeignOwner(ctx context.Context, resourceID, sourceID string) (bool, error) {
	var count int64
	err := o.db.WithContext(ctx).Model(&types.ResourceBinding{}).Where("resource_id=? AND NOT (owner_type=? AND owner_id=?)", resourceID, careerSourceOwner, sourceID).Count(&count).Error
	return count != 0, err
}

func (h *Handler) recoverCatalogRef(ctx context.Context, sourceID, token, requestID, ownerToken string) (string, error) {
	row, candidates, err := h.office.catalogCandidates(ctx, sourceID)
	if err != nil {
		return "", &OutcomeUnknownError{RequestID: requestID}
	}
	if row.ResourceRef != "" {
		return row.ResourceRef, nil
	}
	if row.Status != "processing" || row.ClaimToken != token {
		return "", ErrUploadClaimLost
	}
	for _, candidate := range candidates {
		foreign, lookupErr := h.office.candidateHasForeignOwner(ctx, candidate.ID, sourceID)
		if lookupErr != nil {
			return "", &OutcomeUnknownError{RequestID: requestID}
		}
		if foreign {
			continue
		}
		ref := types.BuildResourcePath(candidate.Handle)
		if err = h.upload.catalog.Bind(ctx, ref, careerSourceOwner, sourceID, types.ResourceRelationSourceFile); err != nil {
			return "", &OutcomeUnknownError{RequestID: requestID}
		}
		if err = h.office.PersistUploadResourceOwned(ctx, sourceID, token, ownerToken, ref); err != nil {
			return "", &OutcomeUnknownError{RequestID: requestID}
		}
		return ref, nil
	}
	return "", nil
}

// A terminal source cannot be taken over. Its active ref is preserved; exact
// surplus registrations are retried after a failed physical delete.
func (h *Handler) cleanupCatalogCandidates(ctx context.Context, sourceID string) error {
	row, candidates, err := h.office.catalogCandidatesWithStates(ctx, sourceID, true)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil || row.Status == "processing" {
		return err
	}
	for _, candidate := range candidates {
		ref := types.BuildResourcePath(candidate.Handle)
		if ref == row.ResourceRef {
			continue
		}
		foreign, lookupErr := h.office.candidateHasForeignOwner(ctx, candidate.ID, sourceID)
		if lookupErr != nil {
			return lookupErr
		}
		if foreign {
			continue
		}
		if row.Status == "failed" && row.ResourceRef == "" {
			return h.office.markCatalogCleanupPending(ctx, row, ref)
		}
		// Re-read terminal state at the delete boundary. A processing source
		// may still adopt this candidate, and an unknown DB result is never
		// evidence that it is safe to delete.
		current, lookupErr := h.office.privateSource(ctx, sourceID)
		if lookupErr != nil {
			return lookupErr
		}
		if current.Status == "processing" || current.ResourceRef == ref {
			continue
		}
		if err := h.upload.Release(ctx, ref, sourceID); err != nil {
			return err
		}
	}
	return nil
}
