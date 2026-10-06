package service

import (
	"context"
	"errors"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type f17TaskAccess struct{ roles map[string]craft.TaskRole }

func (a f17TaskAccess) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	role := a.roles[scope.UserID]
	if !role.AllowsTaskAction(action) {
		return craft.ErrForbidden
	}
	return nil
}

func f17Context(userID string) context.Context {
	return types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: userID})
}

func f17ControlService(run agentruntime.Run, roles map[string]craft.TaskRole) *CraftControlService {
	runs := &fakeControlRuns{run: run}
	svc := NewCraftControlService(runs, nil, nil, nil, nil)
	svc.SetTaskAccess(f17TaskAccess{roles: roles})
	svc.SetStopIntents(&memoryStopIntentStore{})
	return svc
}

func TestCraftControlCollaboratorMayStopOnlyTheirOwnRun(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "run-actor"}, SessionID: "task-f17",
		UserID: "task-owner", ActorUserID: "collaborator", Status: "running"}
	roles := map[string]craft.TaskRole{"task-owner": craft.TaskRoleOwner, "collaborator": craft.TaskRoleCollaborator,
		"other-collaborator": craft.TaskRoleCollaborator}
	svc := f17ControlService(run, roles)
	scope := craft.Scope{TenantID: 1, UserID: "task-owner", SessionID: run.SessionID}
	request := CraftStopRequest{Scope: scope, RunKey: run.Key, TaskID: "delegation"}

	status, err := svc.Stop(f17Context("collaborator"), request)
	require.NoError(t, err, "the initiating Collaborator may stop their Run while storage remains Task-owner scoped")
	require.Equal(t, "stopping", status.Phase)

	_, err = svc.Stop(f17Context("other-collaborator"), request)
	require.ErrorIs(t, err, craft.ErrForbidden, "a Collaborator cannot stop another actor's Run")

	status, err = svc.Stop(f17Context("task-owner"), request)
	require.NoError(t, err, "the Task Owner may control any Run in the Task")
	require.Equal(t, "stopping", status.Phase)
}

func TestCraftControlTaskRolesGateStopAndStatus(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "run-view"}, SessionID: "task-f17-view",
		UserID: "task-owner", ActorUserID: "collaborator", Status: "running"}
	roles := map[string]craft.TaskRole{"task-owner": craft.TaskRoleOwner, "collaborator": craft.TaskRoleCollaborator,
		"viewer": craft.TaskRoleViewer}
	svc := f17ControlService(run, roles)
	scope := craft.Scope{TenantID: 1, UserID: "task-owner", SessionID: run.SessionID}

	_, err := svc.Stop(f17Context("viewer"), CraftStopRequest{Scope: scope, RunKey: run.Key, TaskID: "delegation"})
	require.ErrorIs(t, err, craft.ErrForbidden, "Viewer has read-only Task access")
	_, err = svc.Stop(f17Context("stranger"), CraftStopRequest{Scope: scope, RunKey: run.Key, TaskID: "delegation"})
	require.ErrorIs(t, err, craft.ErrForbidden, "nonmember cannot stop")

	status, err := svc.DelegationStatus(f17Context("viewer"), scope, run.Key, "delegation")
	require.NoError(t, err, "TaskRead members may query the Run status")
	require.Equal(t, "running", status.Phase)
}

func TestCraftControlRequiresTaskAccessChecker(t *testing.T) {
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "run-no-checker"},
		SessionID: "task-no-checker", UserID: "owner", ActorUserID: "owner", Status: "running"}
	svc := NewCraftControlService(&fakeControlRuns{run: run}, nil, nil, nil, nil)
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "owner"})
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: run.SessionID}
	_, err := svc.Stop(ctx, CraftStopRequest{Scope: scope, RunKey: run.Key, TaskID: "delegation"})
	require.ErrorIs(t, err, craft.ErrForbidden, "a missing access checker must not infer authorization from Run.UserID")
	_, err = svc.DelegationStatus(ctx, scope, run.Key, "delegation")
	require.ErrorIs(t, err, craft.ErrForbidden, "status reads must fail closed too")
}

func TestCraftControlRejectsDelegationFromAnotherRunBeforeStopMutation(t *testing.T) {
	svc, runs, store, executor, _, _, seq := newControlFixture(t)
	ownerAccess := f17TaskAccess{roles: map[string]craft.TaskRole{"u1": craft.TaskRoleOwner}}
	svc.SetTaskAccess(ownerAccess)
	intents := &memoryStopIntentStore{}
	svc.SetStopIntents(intents)
	otherRunTask := craft.Task{
		ID: "dlg_other_run", Scope: controlScope(),
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-other"}, Owner: "worker-other", Epoch: 1},
	}
	_, err := store.PrepareTask(context.Background(), otherRunTask)
	require.NoError(t, err)
	ctx := f17Context("u1")
	_, err = svc.Stop(ctx, CraftStopRequest{Scope: controlScope(), RunKey: runs.run.Key, TaskID: otherRunTask.ID})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, 0, executor.aborts, "cross-run delegation must not be aborted")
	require.NotContains(t, seq.steps(), "abort")
	if _, intentErr := intents.GetStopIntent(ctx, controlScope(), runs.run.Key.RunID); !intentErrIsNotFound(intentErr) {
		t.Fatalf("cross-run delegation must be rejected before the addressed Run gets a stop intent, got %v", intentErr)
	}
}

func TestCraftControlRejectsDelegationFromAnotherRunInStatus(t *testing.T) {
	svc, runs, store, executor, _, _, _ := newControlFixture(t)
	svc.SetTaskAccess(f17TaskAccess{roles: map[string]craft.TaskRole{"u1": craft.TaskRoleOwner}})
	otherRunTask := craft.Task{ID: "dlg_status_other_run", Scope: controlScope(),
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-other-status"}, Owner: "worker-other", Epoch: 1}}
	_, err := store.PrepareTask(context.Background(), otherRunTask)
	require.NoError(t, err)
	_, err = svc.DelegationStatus(f17Context("u1"), controlScope(), runs.run.Key, otherRunTask.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, 0, executor.observes, "cross-run delegation must not be observed")
}

func intentErrIsNotFound(err error) bool { return errors.Is(err, craft.ErrNotFound) }
