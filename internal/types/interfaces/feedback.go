package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type FeedbackRepository interface {
	UpsertFeedback(ctx context.Context, fb *types.MessageFeedback) error
	RemoveFeedback(ctx context.Context, tenantID uint64, messageID, userID string) error
	ListBySessionAndUser(ctx context.Context, tenantID uint64, sessionID, userID string) ([]types.MessageFeedback, error)
}
