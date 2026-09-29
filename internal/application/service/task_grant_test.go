package service

// Task-grant service tests (T12 #42): the owner-only gate (tenant Admin is
// NOT admitted — the grant surface is Owner-explicit by design, unlike the
// SP13 share-token surface), grantee must be an active same-tenant member,
// malformed roles are refused, and ResolveTaskAccess separates owner /
// collaborator / viewer / none. Cross-tenant probes are a uniform 404.

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubTaskGrantRepo struct {
	grants map[string]types.TaskGrant // key: taskID + "|" + granteeID
}

func taskGrantKey(taskID, granteeID string) string { return taskID + "|" + granteeID }

func (r *stubTaskGrantRepo) UpsertGrant(
	_ context.Context, tenantID uint64, taskID, granteeID string,
	role types.TaskGrantRole, grantedBy string,
) (types.TaskGrant, error) {
	grant := types.TaskGrant{
		TenantID: tenantID, TaskID: taskID, GranteeID: granteeID,
		Role: role, GrantedBy: grantedBy,
	}
	if r.grants == nil {
		r.grants = map[string]types.TaskGrant{}
	}
	r.grants[taskGrantKey(taskID, granteeID)] = grant
	return grant, nil
}

func (r *stubTaskGrantRepo) DeleteGrant(_ context.Context, _ uint64, taskID, granteeID string) error {
	delete(r.grants, taskGrantKey(taskID, granteeID))
	return nil
}

func (r *stubTaskGrantRepo) ListGrants(_ context.Context, _ uint64, taskID string) ([]types.TaskGrant, error) {
	var grants []types.TaskGrant
	for _, g := range r.grants {
		if g.TaskID == taskID {
			grants = append(grants, g)
		}
	}
	return grants, nil
}

func (r *stubTaskGrantRepo) RoleForGrantee(
	_ context.Context, _ uint64, taskID, granteeID string,
) (types.TaskGrantRole, bool, error) {
	g, ok := r.grants[taskGrantKey(taskID, granteeID)]
	return g.Role, ok, nil
}

func (r *stubTaskGrantRepo) TaskOwnerID(_ context.Context, tenantID uint64, taskID string) (string, error) {
	return "", apperrors.ErrSessionNotFound
}

type stubTaskGrantSessions struct {
	interfaces.SessionRepository
	session *types.Session
}

func (r *stubTaskGrantSessions) GetByID(_ context.Context, tenantID uint64, id string) (*types.Session, error) {
	if r.session != nil && r.session.ID == id && r.session.TenantID == tenantID {
		return r.session, nil
	}
	return nil, apperrors.ErrSessionNotFound
}

type stubTaskGrantMembers struct {
	members map[string]*types.TenantMember // key: userID
}

func (m *stubTaskGrantMembers) Get(_ context.Context, userID string, _ uint64) (*types.TenantMember, error) {
	return m.members[userID], nil
}

func newTaskGrantServiceForTest(
	grants *stubTaskGrantRepo, session *types.Session, members *stubTaskGrantMembers,
) *TaskGrantService {
	return NewTaskGrantService(grants, &stubTaskGrantSessions{session: session}, members)
}

func taskGrantOwner() types.Caller {
	return types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleContributor}
}

func TestGrantTaskAccessOwnerExplicitOnly(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	// Owner grants a viewer.
	grant, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleViewer, grant.Role)
	require.Equal(t, "owner-1", grant.GrantedBy)

	// A tenant Admin is NOT admitted on the grant surface: the explicit
	// member-role assignment is the owner's alone (CONTEXT.md 任务所有者;
	// contrast canShareSession for the SP13 token surface).
	admin := types.Caller{TenantID: 1, UserID: "admin-1", Role: types.TenantRoleAdmin}
	_, err = svc.GrantTaskAccess(ctx, admin, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))

	// Another member cannot grant either.
	other := types.Caller{TenantID: 1, UserID: "member-2", Role: types.TenantRoleContributor}
	_, err = svc.GrantTaskAccess(ctx, other, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))
}

func TestGrantTaskAccessValidatesInputs(t *testing.T) {
	ctx := context.Background()
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2":  {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		"suspended": {UserID: "suspended", TenantID: 1, Status: types.TenantMemberStatusSuspended},
	}}
	svc := newTaskGrantServiceForTest(&stubTaskGrantRepo{}, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	for _, tc := range []struct {
		name      string
		granteeID string
		role      types.TaskGrantRole
	}{
		{"blank grantee", "   ", types.TaskGrantRoleViewer},
		{"invalid role", "member-2", "owner"},
		{"invalid role empty", "member-2", ""},
	} {
		_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", tc.granteeID, tc.role)
		require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err), tc.name)
	}

	// Granting the owner themself is refused.
	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "owner-1", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// A suspended member cannot hold a grant.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "suspended", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// A user with no membership row cannot hold a grant.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "stranger", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// An ownerless task (sessions.user_id = '') cannot anchor a grant — the
	// grant has no authority to hang from.
	ownerless := newTaskGrantServiceForTest(
		&stubTaskGrantRepo{}, &types.Session{ID: "s9", TenantID: 1, UserID: ""},
		&stubTaskGrantMembers{members: map[string]*types.TenantMember{
			"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		}},
	)
	_, err = ownerless.GrantTaskAccess(ctx, taskGrantOwner(), "s9", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrBadRequest, errorCodeOf(t, err))

	// Unknown task is a 404 miss.
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "missing", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}

func TestRevokeAndListTaskGrantsOwnerOnly(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)

	// A grantee cannot revoke their own grant.
	err = svc.RevokeTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1", "member-2")
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))

	// Owner revokes; revoking again is still a success (idempotent).
	require.NoError(t, svc.RevokeTaskAccess(ctx, taskGrantOwner(), "s1", "member-2"))
	require.NoError(t, svc.RevokeTaskAccess(ctx, taskGrantOwner(), "s1", "member-2"))
	list, err := svc.ListTaskGrants(ctx, taskGrantOwner(), "s1")
	require.NoError(t, err)
	require.Empty(t, list)

	// Listing is owner-only too.
	_, err = svc.ListTaskGrants(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.Equal(t, apperrors.ErrForbidden, errorCodeOf(t, err))
}

func TestResolveTaskAccessSeparatesRoles(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{}
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusActive},
		"member-3": {UserID: "member-3", TenantID: 1, Status: types.TenantMemberStatusActive},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)

	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.NoError(t, err)
	_, err = svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-3", types.TaskGrantRoleCollaborator)
	require.NoError(t, err)

	access, err := svc.ResolveTaskAccess(ctx, taskGrantOwner(), "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessOwner, access.Role)
	require.Equal(t, "owner-1", access.OwnerID)
	require.True(t, types.TaskRoleCanRun(access.Role))
	require.True(t, types.TaskRoleCanManage(access.Role))

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessViewer, access.Role)
	require.False(t, types.TaskRoleCanRun(access.Role), "Viewer 不能运行（AC1）")
	require.False(t, types.TaskRoleCanManage(access.Role))

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-3"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessCollaborator, access.Role)
	require.True(t, types.TaskRoleCanRun(access.Role), "Collaborator 可以请求新运行")
	require.False(t, types.TaskRoleCanManage(access.Role), "Collaborator 不能扩额/审批（AC1）")

	access, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "bystander"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessNone, access.Role)
	require.False(t, types.TaskRoleCanRun(access.Role))
}

func TestResolveTaskAccessCrossTenantIsUniform404(t *testing.T) {
	ctx := context.Background()
	svc := newTaskGrantServiceForTest(
		&stubTaskGrantRepo{}, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"},
		&stubTaskGrantMembers{},
	)
	// A tenant-2 caller probing tenant-1's task id sees the same 404 as an
	// unknown task id — the miss leaks nothing.
	_, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "s1")
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
	_, err = svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "missing")
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
	_, err = svc.GrantTaskAccess(ctx, types.Caller{TenantID: 2, UserID: "owner-1"}, "s1", "member-2", types.TaskGrantRoleViewer)
	require.Equal(t, apperrors.ErrNotFound, errorCodeOf(t, err))
}

// explodingTaskGrantSessions injects a non-NotFound infrastructure failure.
type explodingTaskGrantSessions struct {
	interfaces.SessionRepository
}

func (r *explodingTaskGrantSessions) GetByID(_ context.Context, _ uint64, _ string) (*types.Session, error) {
	return nil, errors.New("db connection refused")
}

func assertNotTaskNotFound404(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		require.NotEqual(t, apperrors.ErrNotFound, appErr.Code, "基础设施故障不得伪装成 task not found（B3-F82）")
	}
}

func TestGrantTaskAccessSurfacesInfrastructureErrorsInsteadOf404(t *testing.T) {
	ctx := context.Background()
	svc := NewTaskGrantService(&stubTaskGrantRepo{}, &explodingTaskGrantSessions{}, &stubTaskGrantMembers{})
	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db connection refused")
	assertNotTaskNotFound404(t, err)
}

func TestResolveTaskAccessSurfacesInfrastructureErrorsInsteadOf404(t *testing.T) {
	ctx := context.Background()
	svc := NewTaskGrantService(&stubTaskGrantRepo{}, &explodingTaskGrantSessions{}, &stubTaskGrantMembers{})
	_, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "owner-1"}, "s1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "db connection refused")
	assertNotTaskNotFound404(t, err)
}

func TestResolveTaskAccessDeactivatesStaleGrants(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{grants: map[string]types.TaskGrant{
		taskGrantKey("s1", "member-2"): {TenantID: 1, TaskID: "s1", GranteeID: "member-2", Role: types.TaskGrantRoleViewer},
	}}
	// The grant row survives, but the grantee's live membership no longer is.
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusSuspended},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)
	access, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessNone, access.Role, "停用成员的 grant 必须收敛（B3-F84，与 GetRunForGrantedReader 的实时 JOIN 一致）")
}
