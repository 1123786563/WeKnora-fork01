package service

import (
	"context"
	stderrors "errors"

	"gorm.io/gorm"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// feedbackService implements interfaces.FeedbackService on top of the
// message_feedback table. Acting on a session's feedback is limited to the
// session owner and Admin+ tenant members (cross-user audit).
type feedbackService struct {
	feedbackRepo interfaces.FeedbackRepository
	sessionRepo  interfaces.SessionRepository
	messageRepo  interfaces.MessageRepository
}

// NewFeedbackService creates a feedback service with the required repositories.
func NewFeedbackService(
	feedbackRepo interfaces.FeedbackRepository,
	sessionRepo interfaces.SessionRepository,
	messageRepo interfaces.MessageRepository,
) interfaces.FeedbackService {
	return &feedbackService{
		feedbackRepo: feedbackRepo,
		sessionRepo:  sessionRepo,
		messageRepo:  messageRepo,
	}
}

// canFeedback reports whether the caller may act on the session: the session
// owner themself, or any tenant member at Admin level or above.
func canFeedback(caller types.Caller, session *types.Session) bool {
	if caller.UserID != "" && session.UserID == caller.UserID {
		return true
	}
	return caller.Role.HasPermission(types.TenantRoleAdmin)
}

// SubmitFeedback upserts the caller's like/dislike on one message of a
// session they own (or are Admin+ for). The message must belong to the
// session named on the URL.
func (s *feedbackService) SubmitFeedback(ctx context.Context, caller types.Caller, sessionID, messageID, rating, comment string) error {
	if !types.IsValidFeedbackRating(rating) {
		return apperrors.NewBadRequestError("invalid rating, must be like or dislike")
	}
	session, err := s.loadSession(ctx, caller, sessionID)
	if err != nil {
		return err
	}
	if !canFeedback(caller, session) {
		return apperrors.NewForbiddenError("not allowed to feedback on this session")
	}
	msg, err := s.loadMessage(ctx, sessionID, messageID)
	if err != nil {
		return err
	}
	if msg.SessionID != sessionID {
		return apperrors.NewBadRequestError("message does not belong to session")
	}
	return s.feedbackRepo.UpsertFeedback(ctx, &types.MessageFeedback{
		TenantID:  caller.TenantID,
		UserID:    caller.UserID,
		MessageID: messageID,
		SessionID: sessionID,
		Rating:    rating,
		Comment:   comment,
	})
}

// RemoveFeedback deletes the caller's own feedback row on one message. The
// session gate mirrors SubmitFeedback; the repository delete is additionally
// scoped to the caller's user id.
func (s *feedbackService) RemoveFeedback(ctx context.Context, caller types.Caller, sessionID, messageID string) error {
	session, err := s.loadSession(ctx, caller, sessionID)
	if err != nil {
		return err
	}
	if !canFeedback(caller, session) {
		return apperrors.NewForbiddenError("not allowed to feedback on this session")
	}
	return s.feedbackRepo.RemoveFeedback(ctx, caller.TenantID, messageID, caller.UserID)
}

// ListMyFeedback returns the caller's own feedback rows of a session so the
// chat UI can restore like/dislike state.
func (s *feedbackService) ListMyFeedback(ctx context.Context, caller types.Caller, sessionID string) ([]types.MessageFeedback, error) {
	return s.feedbackRepo.ListBySessionAndUser(ctx, caller.TenantID, sessionID, caller.UserID)
}

// loadSession loads the session inside the caller's tenant. Any miss is
// surfaced as a 404 AppError.
func (s *feedbackService) loadSession(ctx context.Context, caller types.Caller, sessionID string) (*types.Session, error) {
	session, err := s.sessionRepo.GetByID(ctx, caller.TenantID, sessionID)
	if err != nil {
		if stderrors.Is(err, apperrors.ErrSessionNotFound) || stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("session not found")
		}
		return nil, err
	}
	return session, nil
}

// loadMessage loads one message of a session; a miss is surfaced as a 404.
func (s *feedbackService) loadMessage(ctx context.Context, sessionID, messageID string) (*types.Message, error) {
	msg, err := s.messageRepo.GetMessage(ctx, sessionID, messageID)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("message not found")
		}
		return nil, err
	}
	return msg, nil
}
