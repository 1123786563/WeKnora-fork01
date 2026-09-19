package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubFeedbackRepo records feedback writes. Read paths used by the service
// under test are implemented; everything else panics via the nil embed.
type stubFeedbackRepo struct {
	interfaces.FeedbackRepository
	upserted []*types.MessageFeedback
	removed  []string
	listed   []types.MessageFeedback
}

func (r *stubFeedbackRepo) UpsertFeedback(_ context.Context, fb *types.MessageFeedback) error {
	r.upserted = append(r.upserted, fb)
	return nil
}

func (r *stubFeedbackRepo) RemoveFeedback(_ context.Context, _ uint64, messageID, _ string) error {
	r.removed = append(r.removed, messageID)
	return nil
}

func (r *stubFeedbackRepo) ListBySessionAndUser(_ context.Context, _ uint64, _, _ string) ([]types.MessageFeedback, error) {
	return r.listed, nil
}

// stubSessionRepo serves one tenant-scoped session through GetByID; any
// other id reports the same sentinel the real repository returns.
type stubSessionRepo struct {
	interfaces.SessionRepository
	session *types.Session
}

func (r *stubSessionRepo) GetByID(_ context.Context, _ uint64, id string) (*types.Session, error) {
	if r.session != nil && r.session.ID == id {
		return r.session, nil
	}
	return nil, apperrors.ErrSessionNotFound
}

// stubMessageRepo serves one message regardless of the session filter so
// the service's own session-membership check stays exercised.
type stubMessageRepo struct {
	interfaces.MessageRepository
	message *types.Message
}

func (r *stubMessageRepo) GetMessage(_ context.Context, _, _ string) (*types.Message, error) {
	if r.message != nil {
		return r.message, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func newFeedbackServiceForTest(fr *stubFeedbackRepo, sr *stubSessionRepo, mr *stubMessageRepo) interfaces.FeedbackService {
	return NewFeedbackService(fr, sr, mr)
}

func TestFeedbackServiceSubmitEnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
	other := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleContributor}
	admin := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleAdmin}
	svc := newFeedbackServiceForTest(
		&stubFeedbackRepo{},
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}},
		&stubMessageRepo{message: &types.Message{ID: "m1", SessionID: "s1"}},
	)

	// 非 owner 非 admin 拒绝
	require.Error(t, svc.SubmitFeedback(ctx, other, "s1", "m1", types.FeedbackRatingLike, ""))
	// owner 放行
	require.NoError(t, svc.SubmitFeedback(ctx, owner, "s1", "m1", types.FeedbackRatingLike, ""))
	// admin 放行（跨用户审计场景）
	require.NoError(t, svc.SubmitFeedback(ctx, admin, "s1", "m1", types.FeedbackRatingDislike, "x"))
	// 非法 rating 拒绝
	require.Error(t, svc.SubmitFeedback(ctx, owner, "s1", "m1", "meh", ""))
	// 消息不属于该 session 拒绝
	msgRepo := &stubMessageRepo{message: &types.Message{ID: "m2", SessionID: "other-session"}}
	svc2 := newFeedbackServiceForTest(
		&stubFeedbackRepo{},
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}},
		msgRepo,
	)
	require.Error(t, svc2.SubmitFeedback(ctx, owner, "s1", "m2", types.FeedbackRatingLike, ""))
}

// owner 提交必须把 caller 的 tenant/user 归属写进反馈行。
func TestFeedbackServiceSubmitRecordsCallerScope(t *testing.T) {
	ctx := context.Background()
	owner := types.Caller{TenantID: 7, UserID: "owner-1", Role: types.TenantRoleContributor}
	fr := &stubFeedbackRepo{}
	svc := newFeedbackServiceForTest(
		fr,
		&stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 7, UserID: "owner-1"}},
		&stubMessageRepo{message: &types.Message{ID: "m1", SessionID: "s1"}},
	)

	require.NoError(t, svc.SubmitFeedback(ctx, owner, "s1", "m1", types.FeedbackRatingDislike, "bad"))
	require.Len(t, fr.upserted, 1)
	fb := fr.upserted[0]
	require.Equal(t, uint64(7), fb.TenantID)
	require.Equal(t, "owner-1", fb.UserID)
	require.Equal(t, "s1", fb.SessionID)
	require.Equal(t, "m1", fb.MessageID)
	require.Equal(t, types.FeedbackRatingDislike, fb.Rating)
	require.Equal(t, "bad", fb.Comment)
}

// 撤销与回显走 caller 自己的范围；session 缺失同样返回错误。
func TestFeedbackServiceRemoveAndList(t *testing.T) {
	ctx := context.Background()
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
	fr := &stubFeedbackRepo{listed: []types.MessageFeedback{{ID: 1, TenantID: 1, UserID: "owner-1", SessionID: "s1", MessageID: "m1", Rating: types.FeedbackRatingLike}}}
	sr := &stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}}
	svc := newFeedbackServiceForTest(fr, sr, &stubMessageRepo{})

	require.NoError(t, svc.RemoveFeedback(ctx, owner, "s1", "m1"))
	require.Equal(t, []string{"m1"}, fr.removed)

	list, err := svc.ListMyFeedback(ctx, owner, "s1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, types.FeedbackRatingLike, list[0].Rating)

	missing := newFeedbackServiceForTest(fr, &stubSessionRepo{}, &stubMessageRepo{})
	require.Error(t, missing.RemoveFeedback(ctx, owner, "no-such-session", "m1"))
}
