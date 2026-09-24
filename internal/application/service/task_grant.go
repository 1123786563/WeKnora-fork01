package service

// Task collaboration grants (T12, #42). The grant surface is OWNER-EXPLICIT:
// only sessions.user_id may assign or revoke per-task Viewer/Collaborator
// roles (CONTEXT.md 任务所有者：控制任务共享). This is deliberately narrower
// than the SP13 share-token surface (canShareSession admits Admin+); the two
// surfaces coexist — token links serve anonymous read-only snapshots, grants
// serve named-member roles.

import (
	"context"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TaskGrantStorePort is the persistence seam for task_grants.
type TaskGrantStorePort interface {
	UpsertGrant(ctx context.Context, tenantID uint64, taskID, granteeID string, role types.TaskGrantRole, grantedBy string) (types.TaskGrant, error)
	DeleteGrant(ctx context.Context, tenantID uint64, taskID, granteeID string) error
	ListGrants(ctx context.Context, tenantID uint64, taskID string) ([]types.TaskGrant, error)
	RoleForGrantee(ctx context.Context, tenantID uint64, taskID, granteeID string) (types.TaskGrantRole, bool, error)
	TaskOwnerID(ctx context.Context, tenantID uint64, taskID string) (string, error)
}

// TaskMemberLookupPort resolves same-tenant membership. Misses return
// (nil, nil) — the active-membership decision stays here.
type TaskMemberLookupPort interface {
	Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)
}

type TaskGrantService struct {
	grants   TaskGrantStorePort
	sessions interfaces.SessionRepository
	members  TaskMemberLookupPort
}

func NewTaskGrantService(
	grants TaskGrantStorePort,
	sessions interfaces.SessionRepository,
	members TaskMemberLookupPort,
) *TaskGrantService {
	return &TaskGrantService{grants: grants, sessions: sessions, members: members}
}

// loadTaskForGrantManagement loads the task and enforces the owner-only gate
// shared by grant/revoke/list. A miss is one uniform 404.
func (s *TaskGrantService) loadTaskForGrantManagement(
	ctx context.Context, caller types.Caller, taskID string,
) (*types.Session, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, apperrors.NewBadRequestError("task id is required")
	}
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return nil, apperrors.NewForbiddenError("authenticated tenant identity is required")
	}
	session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
	if err != nil {
		return nil, apperrors.NewNotFoundError("task not found")
	}
	if strings.TrimSpace(session.UserID) == "" {
		// A task with no owner row cannot be shared — there is no authority
		// to anchor the grant to.
		return nil, apperrors.NewBadRequestError("task has no owner")
	}
	if session.UserID != caller.UserID {
		return nil, apperrors.NewForbiddenError("only the task owner may manage task access")
	}
	return session, nil
}

// activeMember refuses anything but an active same-tenant membership row.
func (s *TaskGrantService) activeMember(ctx context.Context, tenantID uint64, userID string) (*types.TenantMember, error) {
	if s.members == nil {
		return nil, apperrors.NewBadRequestError("grantee is not an active member of this tenant")
	}
	member, err := s.members.Get(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if member == nil || member.Status != types.TenantMemberStatusActive {
		return nil, apperrors.NewBadRequestError("grantee is not an active member of this tenant")
	}
	return member, nil
}

// GrantTaskAccess assigns (or rewrites) one member's role on one task.
// Owner-only; the grantee must be an active member of the same tenant and
// must not be the owner themself; "owner" is not a grantable role.
func (s *TaskGrantService) GrantTaskAccess(
	ctx context.Context, caller types.Caller, taskID, granteeID string, role types.TaskGrantRole,
) (*types.TaskGrant, error) {
	granteeID = strings.TrimSpace(granteeID)
	if granteeID == "" {
		return nil, apperrors.NewBadRequestError("grantee_id is required")
	}
	if !role.IsValid() {
		return nil, apperrors.NewBadRequestError("role must be viewer or collaborator")
	}
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return nil, err
	}
	if granteeID == session.UserID {
		return nil, apperrors.NewBadRequestError("the task owner already holds every task permission")
	}
	if _, err := s.activeMember(ctx, caller.TenantID, granteeID); err != nil {
		return nil, err
	}
	// UpsertGrant answers a value TaskGrant; the service surface hands back a
	// pointer (brief Produces contract), so take the address here.
	grant, err := s.grants.UpsertGrant(ctx, caller.TenantID, session.ID, granteeID, role, caller.UserID)
	if err != nil {
		return nil, err
	}
	return &grant, nil
}

// RevokeTaskAccess removes one grant. Owner-only; revoking an absent grant is
// a success as long as the task exists and the caller owns it.
func (s *TaskGrantService) RevokeTaskAccess(
	ctx context.Context, caller types.Caller, taskID, granteeID string,
) error {
	granteeID = strings.TrimSpace(granteeID)
	if granteeID == "" {
		return apperrors.NewBadRequestError("grantee_id is required")
	}
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return err
	}
	return s.grants.DeleteGrant(ctx, caller.TenantID, session.ID, granteeID)
}

// ListTaskGrants returns the task's grants. Owner-only.
func (s *TaskGrantService) ListTaskGrants(
	ctx context.Context, caller types.Caller, taskID string,
) ([]types.TaskGrant, error) {
	session, err := s.loadTaskForGrantManagement(ctx, caller, taskID)
	if err != nil {
		return nil, err
	}
	return s.grants.ListGrants(ctx, caller.TenantID, session.ID)
}

// ResolveTaskAccess derives one caller's full access to one task: owner
// (sessions.user_id), collaborator, viewer or none. A task miss — unknown or
// cross-tenant — is one uniform 404 so the probe learns nothing.
func (s *TaskGrantService) ResolveTaskAccess(
	ctx context.Context, caller types.Caller, taskID string,
) (types.TaskAccess, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskAccess{}, apperrors.NewBadRequestError("task id is required")
	}
	if caller.TenantID == 0 {
		return types.TaskAccess{}, apperrors.NewNotFoundError("task not found")
	}
	session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskAccess{}, apperrors.NewNotFoundError("task not found")
	}
	access := types.TaskAccess{TaskID: session.ID, OwnerID: session.UserID, Role: types.TaskAccessNone}
	if strings.TrimSpace(caller.UserID) != "" && caller.UserID == session.UserID {
		access.Role = types.TaskAccessOwner
		return access, nil
	}
	if role, found, rErr := s.grants.RoleForGrantee(ctx, caller.TenantID, session.ID, caller.UserID); rErr != nil {
		return types.TaskAccess{}, rErr
	} else if found {
		access.GrantRole = role
		switch role {
		case types.TaskGrantRoleCollaborator:
			access.Role = types.TaskAccessCollaborator
		case types.TaskGrantRoleViewer:
			access.Role = types.TaskAccessViewer
		}
	}
	return access, nil
}
