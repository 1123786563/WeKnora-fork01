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
var (
	ErrCareerApplicationTaskConflict = errors.New("career application task conflict")
	ErrCareerApplicationTaskNotFound = errors.New("career application task not found")
)

type CareerApplicationTaskIntent struct {
	ApplicationID string
	RequestID     string
	Title         string
}

type CareerApplicationTaskLink struct {
	TaskID string
	RunID  string
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
