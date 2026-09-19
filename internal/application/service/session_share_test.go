package service

// Session-share service tests (SP13 Task 5): the owner/Admin+ gate mirrors
// canFeedback, share-again rotates the token (old link dies), a unique-index
// collision retries once, and the shared read resolves only inside the
// caller's tenant with a uniform 404 for revoked / unknown / cross-tenant
// tokens.

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubShareSetCall records one SetShareToken invocation.
type stubShareSetCall struct {
	tenantID  uint64
	sessionID string
	token     *string
}

// stubShareSessionRepo backs the share service tests: GetByID serves the one
// configured session (tenant-scoped by the caller's TenantID equality),
// GetByShareToken serves the token index, and SetShareToken records calls
// while optionally failing the first N with a unique-index violation.
type stubShareSessionRepo struct {
	interfaces.SessionRepository
	session *types.Session
	byToken map[string]*types.Session
	// setFailures makes the next N SetShareToken calls answer a unique
	// violation on sessions.share_token (collision simulation).
	setFailures int
	setCalls    []stubShareSetCall
}

func (r *stubShareSessionRepo) GetByID(
	_ context.Context, tenantID uint64, id string,
) (*types.Session, error) {
	if r.session != nil && r.session.ID == id && r.session.TenantID == tenantID {
		return r.session, nil
	}
	return nil, apperrors.ErrSessionNotFound
}

func (r *stubShareSessionRepo) GetByShareToken(
	_ context.Context, tenantID uint64, token string,
) (*types.Session, error) {
	if s, ok := r.byToken[token]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, apperrors.ErrSessionNotFound
}

func (r *stubShareSessionRepo) SetShareToken(
	_ context.Context, tenantID uint64, sessionID string, token *string,
) (int64, error) {
	r.setCalls = append(r.setCalls, stubShareSetCall{
		tenantID: tenantID, sessionID: sessionID, token: token,
	})
	if r.setFailures > 0 {
		r.setFailures--
		return 0, errFakeShareTokenCollision()
	}
	if r.session != nil && r.session.ID == sessionID && r.session.TenantID == tenantID {
		if token != nil {
			r.session.ShareToken = *token
		} else {
			r.session.ShareToken = ""
		}
		return 1, nil
	}
	return 0, nil
}

// errFakeShareTokenCollision mimics the driver-level unique violation the
// partial index uq_sessions_share_token raises (SQLite phrasing).
func errFakeShareTokenCollision() error {
	return &fakeShareTokenError{msg: "UNIQUE constraint failed: sessions.share_token"}
}

type fakeShareTokenError struct{ msg string }

func (e *fakeShareTokenError) Error() string { return e.msg }

func newShareServiceForTest(
	sessionRepo interfaces.SessionRepository,
	messageRepo interfaces.MessageRepository,
) *sessionService {
	return &sessionService{sessionRepo: sessionRepo, messageRepo: messageRepo}
}

func shareOwnerSession() *types.Session {
	return &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}
}

func TestShareSessionOwnerAndAdminAllowed(t *testing.T) {
	ctx := context.Background()
	repo := &stubShareSessionRepo{session: shareOwnerSession()}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{})

	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
	token, err := svc.ShareSession(ctx, owner, "s1")
	require.NoError(t, err)
	require.Len(t, token, 43, "32 random bytes base64url-encode to 43 chars")
	require.Equal(t, token, repo.session.ShareToken, "the token lands on the row")

	// Admin+ may share another principal's session (audit/assistance flow).
	admin := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleAdmin}
	adminToken, err := svc.ShareSession(ctx, admin, "s1")
	require.NoError(t, err)
	require.NotEmpty(t, adminToken)
}

func TestShareSessionForbiddenForNonOwner(t *testing.T) {
	ctx := context.Background()
	svc := newShareServiceForTest(
		&stubShareSessionRepo{session: shareOwnerSession()}, &stubSnapshotMessageRepo{},
	)
	other := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleContributor}

	_, err := svc.ShareSession(ctx, other, "s1")
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)
}

func TestShareSessionNotFound(t *testing.T) {
	ctx := context.Background()
	svc := newShareServiceForTest(&stubShareSessionRepo{}, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}

	_, err := svc.ShareSession(ctx, owner, "missing")
	require.Error(t, err)
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}

// errorCodeOf digs the AppError code out for table-free assertions.
func errorCodeOf(t *testing.T, err error) apperrors.ErrorCode {
	t.Helper()
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func TestShareSessionRotatesToken(t *testing.T) {
	ctx := context.Background()
	repo := &stubShareSessionRepo{session: shareOwnerSession()}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}

	first, err := svc.ShareSession(ctx, owner, "s1")
	require.NoError(t, err)
	second, err := svc.ShareSession(ctx, owner, "s1")
	require.NoError(t, err)
	require.NotEqual(t, first, second, "share-again rotates; the old link dies")
	require.Len(t, repo.setCalls, 2)
	require.Equal(t, second, repo.session.ShareToken)
}

func TestShareSessionRetriesOnceOnCollision(t *testing.T) {
	ctx := context.Background()
	repo := &stubShareSessionRepo{session: shareOwnerSession(), setFailures: 1}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}

	token, err := svc.ShareSession(ctx, owner, "s1")
	require.NoError(t, err, "one collision is retried with a fresh token")
	require.Len(t, repo.setCalls, 2)
	require.NotEmpty(t, token)

	// A collision on the retry too is surfaced, not looped forever.
	repo2 := &stubShareSessionRepo{session: shareOwnerSession(), setFailures: 2}
	svc2 := newShareServiceForTest(repo2, &stubSnapshotMessageRepo{})
	_, err = svc2.ShareSession(ctx, owner, "s1")
	require.Error(t, err)
	require.Len(t, repo2.setCalls, 2)
}

func TestUnshareSessionClearsToken(t *testing.T) {
	ctx := context.Background()
	repo := &stubShareSessionRepo{session: shareOwnerSession()}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
	admin := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleAdmin}

	// Non-owner cannot revoke.
	err := svc.UnshareSession(ctx, types.Caller{
		TenantID: 1, UserID: "other-1", Role: types.TenantRoleContributor,
	}, "s1")
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))
	require.Empty(t, repo.setCalls, "a denied caller never reaches the repo")

	require.NoError(t, svc.UnshareSession(ctx, owner, "s1"))
	require.NoError(t, svc.UnshareSession(ctx, admin, "s1"))
	require.Len(t, repo.setCalls, 2)
	for _, call := range repo.setCalls {
		require.Nil(t, call.token, "unshare writes NULL, killing the link")
	}
}

func TestUnshareSessionNotFound(t *testing.T) {
	ctx := context.Background()
	svc := newShareServiceForTest(&stubShareSessionRepo{}, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}

	err := svc.UnshareSession(ctx, owner, "missing")
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}

func TestGetSharedSessionAssemblesReadonlySnapshot(t *testing.T) {
	ctx := context.Background()
	session := shareOwnerSession()
	session.Title = "shared chat"
	repo := &stubShareSessionRepo{
		session: session,
		byToken: map[string]*types.Session{"tok-1": session},
	}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{messages: snapshotMessages(2)})
	caller := types.Caller{TenantID: 1, UserID: "reader-1", Role: types.TenantRoleViewer}

	snapshot, err := svc.GetSharedSession(ctx, caller, "tok-1")
	require.NoError(t, err)
	require.Equal(t, "s1", snapshot.Session.ID)
	require.Equal(t, "owner-1", snapshot.Session.UserID,
		"share readers are same-tenant logged-in users; the owner id stays visible")
	require.Len(t, snapshot.Messages, 2)
	require.False(t, snapshot.Truncated)
}

func TestGetSharedSessionTruncationMatchesAuditSnapshot(t *testing.T) {
	ctx := context.Background()
	session := shareOwnerSession()
	repo := &stubShareSessionRepo{
		session: session,
		byToken: map[string]*types.Session{"tok-1": session},
	}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{messages: snapshotMessages(201)})

	snapshot, err := svc.GetSharedSession(ctx, types.Caller{TenantID: 1}, "tok-1")
	require.NoError(t, err)
	require.Len(t, snapshot.Messages, 200)
	require.True(t, snapshot.Truncated)
	require.Equal(t, "m001", snapshot.Messages[0].ID, "the OLDEST rows are dropped")
	require.Equal(t, "m200", snapshot.Messages[199].ID, "the newest row survives")
}

func TestGetSharedSessionMissIsUniform404(t *testing.T) {
	ctx := context.Background()
	// A live share in tenant 2: the token exists but must not resolve for a
	// tenant-1 caller (cross-tenant probe).
	foreign := &types.Session{ID: "s9", TenantID: 2, UserID: "owner-9"}
	repo := &stubShareSessionRepo{
		session: shareOwnerSession(),
		byToken: map[string]*types.Session{"foreign-tok": foreign},
	}
	svc := newShareServiceForTest(repo, &stubSnapshotMessageRepo{})
	caller := types.Caller{TenantID: 1, UserID: "reader-1", Role: types.TenantRoleViewer}

	for _, token := range []string{"", "   ", "unknown", "revoked-tok", "foreign-tok"} {
		snapshot, err := svc.GetSharedSession(ctx, caller, token)
		require.Nil(t, snapshot, "token %q", token)
		require.Error(t, err, "token %q", token)
		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr, "token %q", token)
		require.Equal(t, apperrors.ErrNotFound, appErr.Code, "token %q", token)
		// One uniform message: revoked / unknown / cross-tenant are
		// indistinguishable, so the miss leaks nothing.
		require.Equal(t, "shared session not found", appErr.Message)
	}
}

func TestShareServiceValidatesScope(t *testing.T) {
	ctx := context.Background()
	svc := newShareServiceForTest(&stubShareSessionRepo{}, &stubSnapshotMessageRepo{})
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}

	_, err := svc.ShareSession(ctx, owner, "")
	require.Error(t, err)
	err = svc.UnshareSession(ctx, owner, "")
	require.Error(t, err)
	_, err = svc.GetSharedSession(ctx, owner, "")
	require.Error(t, err)
}
