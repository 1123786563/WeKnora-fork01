package service

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Mock stores covering the A11 phase-2 fork decision chain as ported from the
// upstream service: message-only fork (no sandbox port), degradation reasons,
// busy refusal, ownership, and history semantics at user vs assistant points.

type forkSessionsMock struct {
	session   *types.Session
	created   *types.Session
	messages  []*types.Message
	leaseByID map[string]*types.ForkSnapshotLease
	createErr error
}

func (m *forkSessionsMock) GetByID(ctx context.Context, tenantID uint64, id string) (*types.Session, error) {
	if m.session != nil && m.session.ID == id && m.session.TenantID == tenantID {
		return m.session, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *forkSessionsMock) CreateForked(ctx context.Context, session *types.Session, messages []*types.Message) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = session
	m.messages = messages
	return nil
}

func (m *forkSessionsMock) CreateForkSnapshotLease(ctx context.Context, lease *types.ForkSnapshotLease) error {
	if m.leaseByID == nil {
		m.leaseByID = map[string]*types.ForkSnapshotLease{}
	}
	m.leaseByID[lease.SnapshotID] = lease
	return nil
}

func (m *forkSessionsMock) DeleteForkSnapshotLease(ctx context.Context, snapshotID string) error {
	delete(m.leaseByID, snapshotID)
	return nil
}

type forkMessagesMock struct {
	byID     map[string]*types.Message
	listed   []*types.Message
	upToSeen bool
}

func (m *forkMessagesMock) GetMessage(ctx context.Context, sessionID, messageID string) (*types.Message, error) {
	msg, ok := m.byID[messageID]
	if !ok || msg.SessionID != sessionID {
		return nil, gorm.ErrRecordNotFound
	}
	return msg, nil
}

func (m *forkMessagesMock) ListMessagesBySessionUpTo(
	ctx context.Context, sessionID string, boundary time.Time, boundaryID string,
) ([]*types.Message, error) {
	m.upToSeen = true
	return m.listed, nil
}

func forkTestHistory() ([]*types.Message, map[string]*types.Message, *types.Message) {
	base := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	user1 := &types.Message{ID: "u1", SessionID: "s-src", Role: "user", Content: "first question", RequestID: "r1", CreatedAt: base}
	assistant1 := &types.Message{ID: "a1", SessionID: "s-src", Role: "assistant", Content: "first answer", RequestID: "r1", IsCompleted: true, CreatedAt: base.Add(time.Minute)}
	user2 := &types.Message{ID: "u2", SessionID: "s-src", Role: "user", Content: "second question", RequestID: "r2", CreatedAt: base.Add(2 * time.Minute)}
	byID := map[string]*types.Message{"u1": user1, "a1": assistant1, "u2": user2}
	return []*types.Message{user1, assistant1}, byID, user2
}

func forkTestService(sessions *forkSessionsMock, messages *forkMessagesMock) *SessionForkService {
	return NewSessionForkService(sessions, messages, nil)
}

func TestForkCopiesHistoryBeforeUserPointWithoutSandboxState(t *testing.T) {
	// Fork at the FIRST user message: no assistant history exists, so no
	// sandbox state is needed and the fork is not degraded (upstream §4.2).
	base := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	firstUser := &types.Message{ID: "u1", SessionID: "s-src", Role: "user", Content: "first question", RequestID: "r1", CreatedAt: base}
	sessions := &forkSessionsMock{session: &types.Session{ID: "s-src", TenantID: 7, UserID: "user-1", Title: "原会话"}}
	messages := &forkMessagesMock{byID: map[string]*types.Message{"u1": firstUser}, listed: nil}
	svc := forkTestService(sessions, messages)

	result, err := svc.Fork(context.Background(), 7, "user-1", "s-src", "u1", "")
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionID)
	require.False(t, result.Degraded, "forking at the first user message needs no sandbox state")
	require.Equal(t, ForkDegradeReason(""), result.Reason)

	created := sessions.created
	require.Equal(t, "s-src", created.ParentSessionID)
	require.Equal(t, "u1", created.ForkedFromMessageID)
	require.Nil(t, created.ForkBootstrap)
	require.Contains(t, created.Title, "（分支）")
	require.Empty(t, sessions.messages, "nothing precedes the first user message")
}

func TestForkDegradesNoCheckpointWhenAssistantHistoryLacksOne(t *testing.T) {
	history, byID, point := forkTestHistory()
	sessions := &forkSessionsMock{session: &types.Session{ID: "s-src", TenantID: 7, UserID: "user-1"}}
	// Fork at the user point after an assistant answer without a checkpoint.
	messages := &forkMessagesMock{byID: byID, listed: history}
	svc := forkTestService(sessions, messages)

	result, err := svc.Fork(context.Background(), 7, "user-1", "s-src", point.ID, "")
	require.NoError(t, err)
	require.True(t, result.Degraded)
	require.Equal(t, ForkDegradeNoCheckpoint, result.Reason)
	require.Len(t, sessions.messages, 2, "degraded forks still copy history")
	require.Equal(t, sessions.messages[0].RequestID, sessions.messages[1].RequestID, "pair stays paired after remap")
	require.NotEqual(t, "r1", sessions.messages[0].RequestID, "remapped away from the parent")
}

func TestForkDegradesSandboxGoneWithCheckpointAndNilPort(t *testing.T) {
	base := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	checkpoint := &types.SandboxCheckpoint{SandboxID: "sbx-1", CommitSHA: "sha1", CommittedAt: base}
	assistant := &types.Message{ID: "a1", SessionID: "s-src", Role: "assistant", IsCompleted: true, SandboxCheckpoint: checkpoint, RequestID: "r1", CreatedAt: base}
	point := &types.Message{ID: "u2", SessionID: "s-src", Role: "user", RequestID: "r2", CreatedAt: base.Add(time.Minute)}
	sessions := &forkSessionsMock{session: &types.Session{ID: "s-src", TenantID: 7, UserID: "user-1"}}
	messages := &forkMessagesMock{byID: map[string]*types.Message{"a1": assistant, "u2": point}, listed: []*types.Message{assistant}}

	result, err := forkTestService(sessions, messages).Fork(context.Background(), 7, "user-1", "s-src", "u2", "")
	require.NoError(t, err)
	require.True(t, result.Degraded)
	require.Equal(t, ForkDegradeSandboxGone, result.Reason, "nil sandbox port with a live checkpoint degrades, never fails")
}

func TestForkAssistantPointIncludesAnswerAndRequiresCompletion(t *testing.T) {
	history, byID, _ := forkTestHistory()
	assistantPoint := byID["a1"]
	sessions := &forkSessionsMock{session: &types.Session{ID: "s-src", TenantID: 7, UserID: "user-1"}}
	messages := &forkMessagesMock{byID: byID, listed: history}

	_, err := forkTestService(sessions, messages).Fork(context.Background(), 7, "user-1", "s-src", assistantPoint.ID, "自定义标题")
	require.NoError(t, err)
	require.Equal(t, "自定义标题", sessions.created.Title)
	require.Len(t, sessions.messages, 3, "assistant fork point copies through the answer")
	require.Equal(t, "assistant", sessions.messages[2].Role)

	incomplete := *assistantPoint
	incomplete.IsCompleted = false
	incomplete.ID = "a9"
	messages2 := &forkMessagesMock{byID: map[string]*types.Message{"a9": &incomplete}, listed: nil}
	_, err = forkTestService(sessions, messages2).Fork(context.Background(), 7, "user-1", "s-src", "a9", "")
	require.ErrorIs(t, err, ErrForkSourceBusy)
}

func TestForkOwnershipAndNotFoundSemantics(t *testing.T) {
	history, byID, point := forkTestHistory()
	sessions := &forkSessionsMock{session: &types.Session{ID: "s-src", TenantID: 7, UserID: "user-1"}}
	messages := &forkMessagesMock{byID: byID, listed: history}
	svc := forkTestService(sessions, messages)

	_, err := svc.Fork(context.Background(), 7, "someone-else", "s-src", point.ID, "")
	require.ErrorIs(t, err, ErrForkSessionNotFound, "foreign owner maps to not-found so ownership is not enumerable")

	_, err = svc.Fork(context.Background(), 7, "user-1", "missing", point.ID, "")
	require.ErrorIs(t, err, ErrForkSessionNotFound)

	_, err = svc.Fork(context.Background(), 7, "user-1", "s-src", "missing-msg", "")
	require.ErrorIs(t, err, ErrForkMessageNotFound)

	system := &types.Message{ID: "sys1", SessionID: "s-src", Role: "system", CreatedAt: time.Now()}
	messages.byID["sys1"] = system
	_, err = svc.Fork(context.Background(), 7, "user-1", "s-src", "sys1", "")
	require.ErrorIs(t, err, ErrForkMessageNotUser)
}

func TestForkNotFoundWrapsRepositorySentinel(t *testing.T) {
	sessions := &forkSessionsMock{}
	messages := &forkMessagesMock{byID: map[string]*types.Message{}}
	svc := forkTestService(sessions, messages)
	require.True(t, isForkNotFound(apperrors.ErrSessionNotFound))
	require.True(t, isForkNotFound(gorm.ErrRecordNotFound))
	require.False(t, isForkNotFound(errors.New("other")))
	_ = svc
}
