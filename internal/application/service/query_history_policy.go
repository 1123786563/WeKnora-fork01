package service

import (
	"context"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// CheckQueryHistoryAccess enforces the tenant's query-history privacy policy
// for the audit entry points (the admin "all-source" session listing, and —
// in later SP13 tasks — snapshots and exports). It returns the effective mode
// and an error only when the tenant disabled query history:
//
//   - disabled: ForbiddenError("query history is disabled for this tenant")
//   - anonymized: mode returned; the caller must mask owner ids on the rows
//   - normal (also: tenant without a config, or a nil repo): mode returned
//
// A tenant row whose stored config holds an empty/unknown mode normalizes to
// normal, so a corrupt row can never silently lock the workspace out of its
// audit listing (see types.NormalizeQueryHistoryMode).
func CheckQueryHistoryAccess(
	ctx context.Context,
	tenantRepo interfaces.TenantRepository,
	tenantID uint64,
) (string, error) {
	// A nil repo means the deployment wired the session service without tenant
	// lookup; the least-privacy-restrictive default (normal) keeps the audit
	// listing functional rather than failing every request.
	if tenantRepo == nil {
		return types.QueryHistoryModeNormal, nil
	}

	tenant, err := tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		return "", err
	}
	if tenant == nil || tenant.QueryHistoryConfig == nil {
		return types.QueryHistoryModeNormal, nil
	}

	mode := types.NormalizeQueryHistoryMode(tenant.QueryHistoryConfig.Mode)
	if mode == types.QueryHistoryModeDisabled {
		return mode, apperrors.NewForbiddenError("query history is disabled for this tenant")
	}
	return mode, nil
}

// AnonymizeSessionOwner masks the owner identity on session-list rows when the
// tenant's query-history mode is anonymized: every UserID becomes "anonymous"
// so an admin auditing usage cannot tie rows back to individual principals.
// Any other mode (or unknown mode) leaves the rows untouched.
func AnonymizeSessionOwner(mode string, rows []*types.SessionListItem) {
	if mode != types.QueryHistoryModeAnonymized {
		return
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.UserID = "anonymous"
	}
}
