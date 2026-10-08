// Package ports declares the outbound interfaces the Query History module's
// application layer consumes: policy lookup, audit reads, export-job
// persistence, file storage, task enqueueing, and time. Adapters (Wave 1,
// Task 7) implement them on top of the legacy repositories and platform
// services; nothing here imports a framework.
//
// The single *types.QueryHistorySnapshot seam (AuditReader.Snapshot) is the
// manifest-recorded Wave 5 compatibility exception: the snapshot DTO still
// reuses the legacy Session/Message/MessageFeedback serializations.
package ports

import (
	"context"
	"io"
	"time"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
)

// PolicyReader answers the workspace's current query-history visibility
// mode. Nil/missing config normalizes to domain.Normal.
type PolicyReader interface {
	Mode(ctx context.Context, tenantID uint64) (domain.Mode, error)
}

// AuditReader serves the admin audit surfaces: the per-session snapshot and
// the aggregated per-session rows a CSV export is built from.
type AuditReader interface {
	// Snapshot loads one session's audit snapshot (session row, most recent
	// messages capped at 200, every feedback row), tenant-scoped.
	Snapshot(ctx context.Context, tenantID uint64, sessionID string) (*types.QueryHistorySnapshot, error)
	// ExportRows returns the aggregated per-session rows for one export,
	// oldest first, honoring the audit filter.
	ExportRows(ctx context.Context, tenantID uint64, filter domain.ExportFilter) ([]domain.ExportRow, error)
}

// ExportJobStore persists the async export job lifecycle. Every status
// transition is tenant-scoped and rewrites both FilePath and ErrorMessage so
// a transition never leaves a stale path or message behind.
type ExportJobStore interface {
	// Create inserts a new export job; an empty status defaults to pending
	// at the store boundary.
	Create(ctx context.Context, job *domain.ExportJob) error
	// Get loads one export job scoped to the tenant; a missing or foreign
	// job id answers not-found so ids cannot be probed across workspaces.
	Get(ctx context.Context, tenantID uint64, jobID uint64) (*domain.ExportJob, error)
	// UpdateStatus transitions the job lifecycle, recording FilePath on done
	// and ErrorMessage on failed.
	UpdateStatus(ctx context.Context, tenantID uint64, jobID uint64, status domain.ExportStatus, filePath string, errMsg string) error
}

// FileStore stores and opens export archives. It is the narrow slice of the
// platform file service the export worker needs.
type FileStore interface {
	// SaveBytes writes data under the tenant and returns its storage path;
	// temp marks derived artifacts that ride the expiry policy.
	SaveBytes(ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool) (string, error)
	// Open returns a reader over a stored path.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
}

// TaskQueue enqueues the module's async work; the adapter owns the task
// type, queue, retry, and timeout options.
type TaskQueue interface {
	EnqueueQueryHistoryExport(ctx context.Context, payload domain.ExportPayload) error
}

// Clock abstracts time reading so the worker's transitions stay testable.
type Clock interface {
	Now() time.Time
}
