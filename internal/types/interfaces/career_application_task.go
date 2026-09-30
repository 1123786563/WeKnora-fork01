package interfaces

import (
	"context"
	"errors"
)

// Shared boundary sentinels for the Career→Workbench application-task link.
// Career may not import the Workbench package, so the definite outcomes of
// the linker contract cross this seam as typed errors:
//   - ErrCareerApplicationTaskConflict is a definite rejection: the request
//     ID is bound to different content, or the application already belongs
//     to another request. Callers must stop, not retry.
//   - ErrCareerApplicationTaskNotFound reports that no durable task exists
//     (yet) for the request ID. The creation outcome is undecided, so
//     callers must keep their recovering state instead of failing hard.
//   - ErrCareerApplicationTaskInvalid is a pure input-validation failure:
//     nothing was written, nothing ever will be under this intent, and the
//     caller should surface an invalid request rather than a conflict.
//   - ErrCareerApplicationTaskUndecided means creation raced until the
//     bounded retry budget ran out and no durable task was found afterwards.
//     The outcome is still open: callers must keep their recovering state
//     and reconcile instead of terminally failing.
var (
	ErrCareerApplicationTaskConflict  = errors.New("career application task conflict")
	ErrCareerApplicationTaskNotFound  = errors.New("career application task not found")
	ErrCareerApplicationTaskInvalid   = errors.New("career application task invalid request")
	ErrCareerApplicationTaskUndecided = errors.New("career application task undecided")
)

type CareerApplicationTaskIntent struct {
	ApplicationID string
	RequestID     string
	Title         string
}

type CareerApplicationTaskLink struct {
	TaskID string
	RunID  string
	// ApplicationID names the application that owns the durable task. The
	// finder fills it so callers can verify the request ID did not resolve
	// to a different application's projection; an empty value means the
	// implementation does not disclose ownership and no check is possible.
	ApplicationID string
}

type CareerApplicationTaskLinker interface {
	EnsureCareerApplicationTask(
		ctx context.Context,
		tenantID uint64,
		ownerID string,
		intent CareerApplicationTaskIntent,
	) (CareerApplicationTaskLink, error)
	FindCareerApplicationTask(
		ctx context.Context,
		tenantID uint64,
		ownerID string,
		requestID string,
	) (CareerApplicationTaskLink, error)
}

// CareerApplicationTaskProjection describes one durable Workbench task that
// originated from a Career application. It is returned by the removal port so
// the caller can audit exactly which projections disappeared.
type CareerApplicationTaskProjection struct {
	TaskID        string
	RunID         string
	ApplicationID string
}

// CareerApplicationTaskProjectionRemover is the complete-deletion seam
// (T22): Career asks Workbench to remove every application-task projection
// it owns. Career never imports Workbench repositories or writes their
// tables directly — same boundary shape as the linker above. The call is
// idempotent: re-running it after a partial failure must succeed with the
// remaining (possibly empty) set of projections.
type CareerApplicationTaskProjectionRemover interface {
	RemoveCareerApplicationTaskProjections(
		ctx context.Context,
		tenantID uint64,
		ownerID string,
	) ([]CareerApplicationTaskProjection, error)
}
