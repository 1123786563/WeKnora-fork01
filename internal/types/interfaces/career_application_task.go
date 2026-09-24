package interfaces

import "context"

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
