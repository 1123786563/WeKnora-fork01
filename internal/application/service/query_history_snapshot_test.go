package service

import (
	"context"
	"fmt"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubSnapshotMessageRepo serves a fixed number of messages through the
// repository-level GetRecentMessagesBySession the snapshot path reads.
type stubSnapshotMessageRepo struct {
	interfaces.MessageRepository
	messages []*types.Message
	err      error
}

func (r *stubSnapshotMessageRepo) GetRecentMessagesBySession(
	_ context.Context, _ string, limit int,
) ([]*types.Message, error) {
	if r.err != nil {
		return nil, r.err
	}
	if limit < len(r.messages) {
		return r.messages[:limit], nil
	}
	return r.messages, nil
}

// stubSnapshotFeedbackRepo serves the cross-user session feedback listing.
type stubSnapshotFeedbackRepo struct {
	interfaces.FeedbackRepository
	listed []types.MessageFeedback
	err    error
}

func (r *stubSnapshotFeedbackRepo) ListBySession(
	_ context.Context, _ uint64, _ string,
) ([]types.MessageFeedback, error) {
	return r.listed, r.err
}

func snapshotMessages(n int) []*types.Message {
	messages := make([]*types.Message, 0, n)
	for i := 0; i < n; i++ {
		messages = append(messages, &types.Message{
			ID: fmt.Sprintf("m%03d", i), SessionID: "s1", Role: "user",
		})
	}
	return messages
}

func newSnapshotServiceForTest(
	tenantRepo interfaces.TenantRepository,
	sessionRepo interfaces.SessionRepository,
	messageRepo interfaces.MessageRepository,
	feedbackRepo interfaces.FeedbackRepository,
) *sessionService {
	return &sessionService{
		tenantRepo:   tenantRepo,
		sessionRepo:  sessionRepo,
		messageRepo:  messageRepo,
		feedbackRepo: feedbackRepo,
	}
}

func TestGetQueryHistorySnapshotNormalMode(t *testing.T) {
	svc := newSnapshotServiceForTest(
		&stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeNormal)},
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "alice"}},
		&stubSnapshotMessageRepo{messages: snapshotMessages(2)},
		&stubSnapshotFeedbackRepo{listed: []types.MessageFeedback{
			{TenantID: 1, UserID: "alice", MessageID: "m001", SessionID: "s1", Rating: types.FeedbackRatingLike},
			{TenantID: 1, UserID: "bob", MessageID: "m002", SessionID: "s1", Rating: types.FeedbackRatingDislike},
		}},
	)

	snapshot, err := svc.GetQueryHistorySnapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "s1", snapshot.Session.ID)
	require.Equal(t, "alice", snapshot.Session.UserID, "normal mode keeps the owner id")
	require.Len(t, snapshot.Messages, 2)
	require.Len(t, snapshot.Feedback, 2)
	require.Equal(t, "alice", snapshot.Feedback[0].UserID)
	require.Equal(t, "bob", snapshot.Feedback[1].UserID)
	require.False(t, snapshot.Truncated)
}

func TestGetQueryHistorySnapshotAnonymizedMasksOwners(t *testing.T) {
	svc := newSnapshotServiceForTest(
		&stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeAnonymized)},
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "alice"}},
		&stubSnapshotMessageRepo{messages: snapshotMessages(1)},
		&stubSnapshotFeedbackRepo{listed: []types.MessageFeedback{
			{TenantID: 1, UserID: "alice", MessageID: "m001", SessionID: "s1", Rating: types.FeedbackRatingLike},
			{TenantID: 1, UserID: "bob", MessageID: "m002", SessionID: "s1", Rating: types.FeedbackRatingDislike},
		}},
	)

	snapshot, err := svc.GetQueryHistorySnapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "anonymous", snapshot.Session.UserID, "anonymized mode masks the session owner")
	for _, fb := range snapshot.Feedback {
		require.Equal(t, "anonymous", fb.UserID, "anonymized mode masks every feedback row's user")
	}
}

func TestGetQueryHistorySnapshotDisabledIsForbidden(t *testing.T) {
	svc := newSnapshotServiceForTest(
		&stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeDisabled)},
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "alice"}},
		&stubSnapshotMessageRepo{},
		&stubSnapshotFeedbackRepo{},
	)

	snapshot, err := svc.GetQueryHistorySnapshot(context.Background(), 1, "s1")
	require.Nil(t, snapshot)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)
}

func TestGetQueryHistorySnapshotSessionNotFound(t *testing.T) {
	svc := newSnapshotServiceForTest(
		&stubTenantRepoForHistory{tenant: &types.Tenant{ID: 1}},
		&stubSessionRepo{}, // no session served -> ErrSessionNotFound
		&stubSnapshotMessageRepo{},
		&stubSnapshotFeedbackRepo{},
	)

	snapshot, err := svc.GetQueryHistorySnapshot(context.Background(), 1, "missing")
	require.Nil(t, snapshot)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetQueryHistorySnapshotTruncation(t *testing.T) {
	feedback := []types.MessageFeedback{
		{TenantID: 1, UserID: "alice", MessageID: "m001", SessionID: "s1", Rating: types.FeedbackRatingLike},
	}
	newSvc := func(n int) *sessionService {
		return newSnapshotServiceForTest(
			&stubTenantRepoForHistory{tenant: &types.Tenant{ID: 1}},
			&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "alice"}},
			&stubSnapshotMessageRepo{messages: snapshotMessages(n)},
			&stubSnapshotFeedbackRepo{listed: feedback},
		)
	}

	// Exactly the cap: no truncation.
	snapshot, err := newSvc(200).GetQueryHistorySnapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Len(t, snapshot.Messages, 200)
	require.False(t, snapshot.Truncated)

	// Over the cap: keep the most recent messages and flag truncation.
	snapshot, err = newSvc(201).GetQueryHistorySnapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Len(t, snapshot.Messages, 200)
	require.True(t, snapshot.Truncated)
	require.Equal(t, "m001", snapshot.Messages[0].ID, "truncation drops the OLDEST rows")
	require.Equal(t, "m200", snapshot.Messages[199].ID, "the newest row survives truncation")
}

func TestGetQueryHistorySnapshotValidatesScope(t *testing.T) {
	svc := newSnapshotServiceForTest(nil, nil, nil, nil)

	_, err := svc.GetQueryHistorySnapshot(context.Background(), 0, "s1")
	require.Error(t, err)
	_, err = svc.GetQueryHistorySnapshot(context.Background(), 1, "")
	require.Error(t, err)
}
