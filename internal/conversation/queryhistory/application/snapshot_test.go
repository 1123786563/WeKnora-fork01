package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// fakeAuditReader is the test double for ports.AuditReader: it records the
// tenant/session scope it was consulted with and serves a fixed snapshot or
// error. ExportRows is unused by the snapshot use case and serves nothing.
type fakeAuditReader struct {
	snapshot *types.QueryHistorySnapshot
	err      error

	snapshotCalls int
	lastTenantID  uint64
	lastSessionID string
}

func (f *fakeAuditReader) Snapshot(
	_ context.Context, tenantID uint64, sessionID string,
) (*types.QueryHistorySnapshot, error) {
	f.snapshotCalls++
	f.lastTenantID = tenantID
	f.lastSessionID = sessionID
	return f.snapshot, f.err
}

func (f *fakeAuditReader) ExportRows(
	context.Context, uint64, domain.ExportFilter,
) ([]domain.ExportRow, error) {
	return nil, nil
}

var _ ports.AuditReader = (*fakeAuditReader)(nil)

// snapshotMessages builds n message pointers with zero-padded ids m000..,
// mirroring the legacy snapshot fixtures.
func snapshotMessages(n int) []*types.Message {
	messages := make([]*types.Message, 0, n)
	for i := 0; i < n; i++ {
		messages = append(messages, &types.Message{
			ID: fmt.Sprintf("m%03d", i), SessionID: "s1", Role: "user",
		})
	}
	return messages
}

func feedbackRows() []types.MessageFeedback {
	return []types.MessageFeedback{
		{TenantID: 1, UserID: "alice", MessageID: "m000", SessionID: "s1", Rating: types.FeedbackRatingLike},
		{TenantID: 1, UserID: "bob", MessageID: "m001", SessionID: "s1", Rating: types.FeedbackRatingDislike},
	}
}

// TestSnapshotNormalModeServesUnmasked ports the legacy normal-mode snapshot
// test: identities stay, messages and feedback pass through, and the reader
// is consulted exactly once with the caller's scope.
func TestSnapshotNormalModeServesUnmasked(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Normal}
	reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
		Session:   types.Session{ID: "s1", TenantID: 1, UserID: "alice"},
		Messages:  snapshotMessages(2),
		Feedback:  feedbackRows(),
		Truncated: false,
	}}
	svc := NewAuditService(policy, reader)

	snapshot, err := svc.Snapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "s1", snapshot.Session.ID)
	require.Equal(t, "alice", snapshot.Session.UserID, "normal mode keeps the owner id")
	require.Len(t, snapshot.Messages, 2)
	require.Len(t, snapshot.Feedback, 2)
	require.Equal(t, "alice", snapshot.Feedback[0].UserID)
	require.Equal(t, "bob", snapshot.Feedback[1].UserID)
	require.False(t, snapshot.Truncated)

	require.Equal(t, 1, reader.snapshotCalls, "the audit reader is called exactly once")
	require.Equal(t, uint64(1), reader.lastTenantID)
	require.Equal(t, "s1", reader.lastSessionID)
}

// TestSnapshotAnonymizedMasksOwners ports the legacy anonymized snapshot
// test: Session.UserID and every Feedback.UserID become "anonymous",
// messages pass through untouched, and the wire form leaks no principal.
func TestSnapshotAnonymizedMasksOwners(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Anonymized}
	reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
		Session:  types.Session{ID: "s1", TenantID: 1, UserID: "alice"},
		Messages: snapshotMessages(1),
		Feedback: feedbackRows(),
	}}
	svc := NewAuditService(policy, reader)

	snapshot, err := svc.Snapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "anonymous", snapshot.Session.UserID, "anonymized mode masks the session owner")
	for _, fb := range snapshot.Feedback {
		require.Equal(t, "anonymous", fb.UserID, "anonymized mode masks every feedback row's user")
	}
	require.Equal(t, "m000", snapshot.Messages[0].ID, "messages carry no owner field and pass through unchanged")

	// Wire-level pin (ported verbatim from the legacy snapshot test): the
	// serialized snapshot leaks neither principal — and carries no IM
	// principal id at all.
	wire, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(wire), "alice")
	require.NotContains(t, string(wire), "bob")
	require.NotContains(t, string(wire), "im_user_id")
}

// TestSnapshotDisabledSkipsReader covers the mandated short-circuit: a
// disabled policy answers the exact legacy forbidden AppError and the audit
// reader is never consulted.
func TestSnapshotDisabledSkipsReader(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Disabled}
	reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
		Session: types.Session{ID: "s1", TenantID: 1, UserID: "alice"},
	}}
	svc := NewAuditService(policy, reader)

	snapshot, err := svc.Snapshot(context.Background(), 1, "s1")
	require.Nil(t, snapshot)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)
	require.Equal(t, "query history is disabled for this tenant", appErr.Message)
	require.Equal(t, 0, reader.snapshotCalls, "a disabled policy short-circuits before the audit reader")
}

// TestSnapshotNotFoundPropagates ports the legacy not-found test: the
// reader's not-found error surfaces unchanged (a foreign session is
// indistinguishable from a missing one behind the port).
func TestSnapshotNotFoundPropagates(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Normal}
	reader := &fakeAuditReader{
		err: fmt.Errorf("load session: %w", apperrors.ErrSessionNotFound),
	}
	svc := NewAuditService(policy, reader)

	snapshot, err := svc.Snapshot(context.Background(), 1, "missing")
	require.Nil(t, snapshot)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

// TestSnapshotTruncationPassthrough ports the legacy 200-message truncation
// coverage to the module seam. The cap now lives behind ports.AuditReader
// (the adapter fetches limit+1 and keeps the newest 200, exactly as the
// legacy service did), so this pins that the service relays the reader's
// capped payload verbatim: same rows, same oldest-first order, same
// Truncated flag — no re-trimming, no reordering, no flag rewriting.
func TestSnapshotTruncationPassthrough(t *testing.T) {
	newSvc := func(messages []*types.Message, truncated bool) *AuditService {
		reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
			Session:   types.Session{ID: "s1", TenantID: 1, UserID: "alice"},
			Messages:  messages,
			Feedback:  feedbackRows()[:1],
			Truncated: truncated,
		}}
		return NewAuditService(&fakePolicyReader{mode: domain.Normal}, reader)
	}

	// Exactly the cap: no truncation.
	svc := newSvc(snapshotMessages(200), false)
	snapshot, err := svc.Snapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Len(t, snapshot.Messages, 200)
	require.False(t, snapshot.Truncated)

	// Over the cap: the reader (like the legacy service before it) kept the
	// most recent 200 messages and flagged truncation; the service must relay
	// exactly that.
	svc = newSvc(snapshotMessages(201)[1:], true)
	snapshot, err = svc.Snapshot(context.Background(), 1, "s1")
	require.NoError(t, err)
	require.Len(t, snapshot.Messages, 200)
	require.True(t, snapshot.Truncated)
	require.Equal(t, "m001", snapshot.Messages[0].ID, "truncation drops the OLDEST rows")
	require.Equal(t, "m200", snapshot.Messages[199].ID, "the newest row survives truncation")
}

// TestSnapshotValidatesScope ports the legacy scope-validation test and pins
// the cross-tenant contract at this seam: validation failures never consult
// the policy or the reader, and the caller's tenant id reaches the reader
// untouched — row-level tenant scoping is enforced behind the port.
func TestSnapshotValidatesScope(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Normal}
	reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
		Session: types.Session{ID: "sess7", TenantID: 7, UserID: "carol"},
	}}
	svc := NewAuditService(policy, reader)

	_, err := svc.Snapshot(context.Background(), 0, "s1")
	require.EqualError(t, err, "workspace id is required")
	_, err = svc.Snapshot(context.Background(), 1, "")
	require.EqualError(t, err, "session id is required")
	require.Equal(t, 0, policy.modeCalls, "validation failures never consult the policy")
	require.Equal(t, 0, reader.snapshotCalls, "validation failures never consult the reader")

	snapshot, err := svc.Snapshot(context.Background(), 7, "sess7")
	require.NoError(t, err)
	require.Equal(t, 1, reader.snapshotCalls)
	require.Equal(t, uint64(7), reader.lastTenantID, "the caller's tenant id scopes the reader call")
	require.Equal(t, "sess7", reader.lastSessionID)
	require.Equal(t, "sess7", snapshot.Session.ID)
}

// TestSnapshotPolicyErrorSkipsReader: a policy lookup failure propagates
// unchanged and the audit reader is never consulted.
func TestSnapshotPolicyErrorSkipsReader(t *testing.T) {
	sentinel := errors.New("tenant lookup down")
	policy := &fakePolicyReader{err: sentinel}
	reader := &fakeAuditReader{snapshot: &types.QueryHistorySnapshot{
		Session: types.Session{ID: "s1", TenantID: 1, UserID: "alice"},
	}}
	svc := NewAuditService(policy, reader)

	snapshot, err := svc.Snapshot(context.Background(), 1, "s1")
	require.Nil(t, snapshot)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 0, reader.snapshotCalls, "a policy failure short-circuits before the audit reader")
}
