package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// QueryHistoryExportJobRepository persists the async query-history export
// jobs (SP13 Task 4) and answers the aggregated per-session rows a CSV is
// built from.
type QueryHistoryExportJobRepository interface {
	// Create inserts a new export job. Callers set Status pending; an empty
	// status defaults to pending at the repository boundary.
	Create(ctx context.Context, job *types.QueryHistoryExportJob) error
	// GetByID loads one export job scoped to the tenant. A job id from
	// another tenant answers ErrQueryHistoryExportJobNotFound so callers map
	// cross-tenant probes to 404 rather than to a leak.
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*types.QueryHistoryExportJob, error)
	// UpdateStatus transitions the job lifecycle, recording FilePath on done
	// and ErrorMessage on failed. Both columns are always written so a
	// transition never leaves a stale path or message behind.
	UpdateStatus(ctx context.Context, id uint64, status, filePath, errMsg string) error
	// ExportSessionRows returns the aggregated per-session rows for one
	// export (message count + like/dislike counts), honoring the same
	// filters as the Admin+ source=all audit listing and capped at the
	// export row limit (the CSV is a summary surface, not a dump).
	ExportSessionRows(ctx context.Context, q *types.SessionListQuery) ([]types.QueryHistoryExportRow, error)
}
