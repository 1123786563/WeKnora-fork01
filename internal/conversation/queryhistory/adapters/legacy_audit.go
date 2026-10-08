// Package adapters implements the Query History module's outbound ports
// (Wave 1, Task 7) on top of the legacy repositories and platform services:
// the tenant policy over TenantRepository, the audit reads over the temporary
// repository.SessionAuditRepository seam, export-job persistence over GORM,
// and the file/task-queue slice over interfaces.FileService and
// interfaces.TaskEnqueuer.
//
// The adapters are the module's integration boundary — the manifest records
// the `adapters -> internal/application/repository` exception for removal in
// Wave 5 — so ports and domain stay framework-free while everything here may
// import GORM, asynq, and the legacy packages it wraps.
package adapters

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/types"
)

// LegacyAudit satisfies ports.AuditReader by wrapping the legacy repository's
// SessionAuditRepository seam. It is a thin pass-through on purpose: the
// tenant scoping, the 200-message snapshot cap, the foreign==missing
// not-found mapping, and the shared export predicates all live behind the
// seam so there is exactly one implementation of each.
type LegacyAudit struct {
	audit repository.SessionAuditRepository
}

// NewLegacyAudit builds the audit adapter over the repository seam.
func NewLegacyAudit(audit repository.SessionAuditRepository) *LegacyAudit {
	return &LegacyAudit{audit: audit}
}

// compile-time port conformance.
var _ ports.AuditReader = (*LegacyAudit)(nil)

// Snapshot loads one session's audit snapshot, tenant-scoped; a foreign
// session answers the same not-found error as a missing one.
func (a *LegacyAudit) Snapshot(
	ctx context.Context, tenantID uint64, sessionID string,
) (*types.QueryHistorySnapshot, error) {
	return a.audit.Snapshot(ctx, tenantID, sessionID)
}

// ExportRows returns the aggregated per-session rows for one export, oldest
// first, honoring the audit filter.
func (a *LegacyAudit) ExportRows(
	ctx context.Context, tenantID uint64, filter domain.ExportFilter,
) ([]domain.ExportRow, error) {
	return a.audit.ExportRows(ctx, tenantID, filter)
}
