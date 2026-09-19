package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type FeedbackRepository interface {
	UpsertFeedback(ctx context.Context, fb *types.MessageFeedback) error
	RemoveFeedback(ctx context.Context, tenantID uint64, messageID, userID string) error
	ListBySessionAndUser(ctx context.Context, tenantID uint64, sessionID, userID string) ([]types.MessageFeedback, error)
	// ListBySession returns every feedback row of one session inside the
	// tenant, across users (the admin query-history audit snapshot). Unlike
	// ListBySessionAndUser it carries no user predicate.
	ListBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.MessageFeedback, error)
}

// FeedbackService defines the message feedback service interface. Access is
// restricted to the session owner or Admin+ callers (cross-user audit).
type FeedbackService interface {
	// SubmitFeedback upserts the caller's like/dislike on one message.
	SubmitFeedback(ctx context.Context, caller types.Caller, sessionID, messageID, rating, comment string) error
	// RemoveFeedback deletes the caller's feedback on one message.
	RemoveFeedback(ctx context.Context, caller types.Caller, sessionID, messageID string) error
	// ListMyFeedback returns the caller's own feedback rows of a session.
	ListMyFeedback(ctx context.Context, caller types.Caller, sessionID string) ([]types.MessageFeedback, error)
}
