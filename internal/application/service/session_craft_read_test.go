package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sessionCraftReadAccess struct {
	registered  []bool
	classifyErr error
	allowed     map[string]bool
	checks      []sessionCraftReadCheck
	tenantID    uint64
	sessionID   string
}

type sessionCraftReadCheck struct {
	scope  craft.Scope
	action craft.TaskAction
}

func (a *sessionCraftReadAccess) IsCraftTask(_ context.Context, tenantID uint64, sessionID string) (bool, error) {
	if a.classifyErr != nil {
		return false, a.classifyErr
	}
	if a.tenantID != 0 && (tenantID != a.tenantID || sessionID != a.sessionID) {
		return false, craft.ErrNotFound
	}
	if len(a.registered) == 0 {
		return false, nil
	}
	registered := a.registered[0]
	if len(a.registered) > 1 {
		a.registered = a.registered[1:]
	}
	return registered, nil
}

func (a *sessionCraftReadAccess) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	a.checks = append(a.checks, sessionCraftReadCheck{scope: scope, action: action})
	if action != craft.TaskRead || !a.allowed[scope.UserID] {
		return craft.ErrForbidden
	}
	return nil
}

func newCraftDirectReadService(t *testing.T, access craft.TaskRunAccess) (*sessionService, *gorm.DB, *types.Session) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Session{}))
	session := &types.Session{TenantID: 1, UserID: "task-owner", Title: "private Craft task metadata"}
	require.NoError(t, db.Create(session).Error)
	if readAccess, ok := access.(*sessionCraftReadAccess); ok {
		readAccess.tenantID = session.TenantID
		readAccess.sessionID = session.ID
	}
	return &sessionService{sessionRepo: repository.NewSessionRepository(db), craftTaskAccess: access}, db, session
}

func TestGetSessionCraftTaskRequiresCurrentTaskRead(t *testing.T) {
	access := &sessionCraftReadAccess{registered: []bool{true, true}, allowed: map[string]bool{"task-owner": true, "viewer": true}}
	svc, _, session := newCraftDirectReadService(t, access)

	owner, err := svc.GetSession(testSessionScopeContext(1, "task-owner"), session.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, owner.ID)
	viewer, err := svc.GetSession(testSessionScopeContext(1, "viewer"), session.ID)
	require.NoError(t, err)
	require.Equal(t, "private Craft task metadata", viewer.Title)
	require.Equal(t, []sessionCraftReadCheck{
		{scope: craft.Scope{TenantID: 1, UserID: "task-owner", SessionID: session.ID}, action: craft.TaskRead},
		{scope: craft.Scope{TenantID: 1, UserID: "viewer", SessionID: session.ID}, action: craft.TaskRead},
	}, access.checks)
}

func TestGetSessionCraftTaskDeniesWithoutTaskReadIncludingTenantAdmin(t *testing.T) {
	access := &sessionCraftReadAccess{registered: []bool{true, true}, allowed: map[string]bool{}}
	svc, _, session := newCraftDirectReadService(t, access)
	adminCtx := context.WithValue(testSessionScopeContext(1, "tenant-admin"), types.TenantRoleContextKey, types.TenantRoleAdmin)

	for name, ctx := range map[string]context.Context{
		"same-tenant nonmember": testSessionScopeContext(1, "nonmember"),
		"revoked Viewer":        testSessionScopeContext(1, "revoked-viewer"),
		"stale membership":      testSessionScopeContext(1, "stale-viewer"),
		"tenant admin":          adminCtx,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := svc.GetSession(ctx, session.ID)
			require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
			require.Nil(t, got, "denial must not return Craft session metadata")
		})
	}
	require.Len(t, access.checks, 4)
}

func TestGetSessionCraftTaskCannotCrossTenant(t *testing.T) {
	access := &sessionCraftReadAccess{registered: []bool{true}, allowed: map[string]bool{"viewer": true}}
	svc, _, session := newCraftDirectReadService(t, access)

	got, err := svc.GetSession(testSessionScopeContext(2, "viewer"), session.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
	require.Nil(t, got)
	require.Empty(t, access.checks, "tenant-scoped classification must fail before TaskRead")
}

func TestGetSessionCraftTaskClassificationErrorsSplitByKind(t *testing.T) {
	// Semantic lookup answers (no such session / caller mismatch) keep the
	// 404 the read surface promises; infrastructure failures propagate so a
	// database outage is never masked as "session not found" (the OCR round
	// corrected the earlier blanket flattening).
	semantics := &sessionCraftReadAccess{classifyErr: craft.ErrNotFound, allowed: map[string]bool{"task-owner": true}}
	svc, _, session := newCraftDirectReadService(t, semantics)
	got, err := svc.GetSession(testSessionScopeContext(1, "task-owner"), session.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
	require.Nil(t, got)
	require.Empty(t, semantics.checks, "TaskRead must not run when classification is unknown")

	infra := &sessionCraftReadAccess{classifyErr: errors.New("registration lookup unavailable"), allowed: map[string]bool{"task-owner": true}}
	svc, _, session = newCraftDirectReadService(t, infra)
	got, err = svc.GetSession(testSessionScopeContext(1, "task-owner"), session.ID)
	require.ErrorContains(t, err, "registration lookup unavailable")
	require.NotErrorIs(t, err, apperrors.ErrSessionNotFound, "infrastructure failures must surface as themselves, not as a missing session")
	require.Nil(t, got)
	require.Empty(t, infra.checks)
}

func TestGetSessionCraftTaskFailsClosedWhenRegistrationChangesBeforeReturn(t *testing.T) {
	access := &sessionCraftReadAccess{registered: []bool{true, false}, allowed: map[string]bool{"task-owner": true}}
	svc, _, session := newCraftDirectReadService(t, access)

	got, err := svc.GetSession(testSessionScopeContext(1, "task-owner"), session.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
	require.Nil(t, got, "a session that loses its Craft registration during the read must stay hidden")
	require.Len(t, access.checks, 1)
}

func TestGetSessionNonCraftOwnerReadDoesNotRequireCraftGrant(t *testing.T) {
	access := &sessionCraftReadAccess{registered: []bool{false}}
	svc, _, session := newCraftDirectReadService(t, access)

	got, err := svc.GetSession(testSessionScopeContext(1, "task-owner"), session.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, got.ID)
	require.Empty(t, access.checks)
}
