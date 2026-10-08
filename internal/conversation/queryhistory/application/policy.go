// Package application implements the Query History module's use cases over
// its ports. This file and snapshot.go move the legacy audit policy gate and
// per-session snapshot orchestration (CheckQueryHistoryAccess /
// GetQueryHistorySnapshot) behind the module's PolicyReader / AuditReader
// seams; the application layer depends only on the module's domain and ports
// plus — as the manifest-recorded Wave 5 compatibility exception — the legacy
// types.QueryHistorySnapshot the ports still reference.
package application

import (
	"context"
	"errors"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
)

// AuditService enforces the workspace's query-history visibility policy on
// the admin audit surfaces: the per-session snapshot here, the export flows
// in Task 6. It is the module-side replacement for the legacy
// CheckQueryHistoryAccess / GetQueryHistorySnapshot pair.
type AuditService struct {
	policy ports.PolicyReader
	audit  ports.AuditReader
}

// NewAuditService wires the audit use cases onto their policy and audit-read
// ports.
func NewAuditService(policy ports.PolicyReader, audit ports.AuditReader) *AuditService {
	return &AuditService{policy: policy, audit: audit}
}

// CheckAccess enforces the tenant's query-history privacy policy for the
// audit entry points. It returns the effective mode and an error only when
// the tenant disabled query history:
//
//   - disabled: ForbiddenError("query history is disabled for this tenant")
//   - anonymized: mode returned; the caller must mask owner ids on the rows
//   - normal (also: tenant without a config): mode returned
//
// The PolicyReader port normalizes an empty/unknown stored mode to normal,
// so a corrupt row can never silently lock the workspace out of its audit
// surfaces. A zero tenant id is rejected up front with the legacy
// validation error.
func (s *AuditService) CheckAccess(ctx context.Context, tenantID uint64) (domain.Mode, error) {
	if tenantID == 0 {
		return "", errors.New("workspace id is required")
	}
	mode, err := s.policy.Mode(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if mode == domain.Disabled {
		return mode, apperrors.NewForbiddenError("query history is disabled for this tenant")
	}
	return mode, nil
}
