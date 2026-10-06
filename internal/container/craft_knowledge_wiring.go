package container

import (
	"context"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// wireCraftKnowledgeRuntime binds the independently reviewed per-Run knowledge
// package chain onto the local craft runtime:
//
//   - the H1 builder (CraftKnowledgeRunViewBuilder) resolves the admitted Run's
//     durable KB selection and constructs a fresh service and per-Run
//     NewCraftKnowledgePackagePublisher inside the verified RunView generation —
//     there is no shared singleton publisher on this path;
//   - the H2 exact final-byte verifier (VerifyAcceptedForRun) re-checks the
//     published record and the sealed package;
//   - the read-only current-authority recheck (RevalidateForDispatch) runs at
//     the last delegate boundary, immediately before the inner executor prompt,
//     against the Run's persisted actor (T08) — never the Task owner.
//
// Missing ports fail closed. Without the local runtime dial there is no
// delegation to gate, so wiring is skipped and the runtime keeps its
// unresolved-knowledge behavior; without the RunView production pins the
// knowledgeResolver stays nil and localCraftRuntime.Execute refuses every
// delegation before dispatch.
func wireCraftKnowledgeRuntime(
	executor craft.Executor,
	store craft.Store,
	knowledge interfaces.KnowledgeService,
	knowledgeBases interfaces.KnowledgeBaseService,
	runs *repository.AgentRunStore,
	assembly *CraftRunViewProductionAssembly,
	taskAccess craft.TaskAccessChecker,
	records *repository.CraftKnowledgeRecordRepository,
	svc *service.CraftKnowledgeService,
) error {
	runtime, ok := executor.(*localCraftRuntime)
	if !ok {
		logger.Infof(context.Background(), "[CraftKnowledge] runtime wiring skipped: the local craft runtime dial is not assembled")
		return nil
	}
	if taskAccess == nil || records == nil {
		return fmt.Errorf("craft knowledge runtime wiring requires the current Task access authority and the durable source record store")
	}
	if svc == nil {
		return fmt.Errorf("craft knowledge runtime wiring requires the read-only dispatch recheck service")
	}
	if assembly == nil || assembly.Unavailable != "" || assembly.Provider == nil {
		reason := "RunView production assembly is unavailable"
		if assembly != nil && assembly.Unavailable != "" {
			reason = assembly.Unavailable
		}
		logger.Infof(context.Background(), "[CraftKnowledge] runtime wiring skipped: %s (delegations stay fail-closed)", reason)
		return nil
	}
	builder, err := NewCraftKnowledgeRunViewBuilder(CraftKnowledgeRunViewConfig{
		Store:      store,
		Access:     service.BindCraftKnowledgeAccess(knowledge),
		Search:     service.BindCraftKnowledgeSearch(knowledgeBases),
		TaskAccess: taskAccess,
		Records:    records,
	})
	if err != nil {
		return fmt.Errorf("craft knowledge runtime wiring: %w", err)
	}
	dispatch := svc
	runtime.knowledgeResolver = func(ctx context.Context, task craft.Task, material CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error) {
		run, err := craftKnowledgeAdmittedRunForTask(ctx, runs, task)
		if err != nil {
			return CraftKnowledgeRunViewAcceptance{}, err
		}
		return builder.BuildForRun(ctx, run, material)
	}
	runtime.knowledgeVerifier = func(ctx context.Context, task craft.Task, material CraftRunViewMaterialHandle, accepted CraftKnowledgeRunViewAcceptance) error {
		run, err := craftKnowledgeAdmittedRunForTask(ctx, runs, task)
		if err != nil {
			return err
		}
		if err := builder.VerifyAcceptedForRun(ctx, run, material, accepted); err != nil {
			return err
		}
		// Last current-authority gate: bind the caller from the Run's
		// persisted actor (T08), never the delegating Task owner, and
		// re-resolve every recorded source through the shared-aware ACL read.
		scope, _, _, err := craftKnowledgeRunIdentity(run)
		if err != nil {
			return err
		}
		return dispatch.RevalidateForDispatch(
			types.WithCaller(ctx, types.Caller{TenantID: scope.TenantID, UserID: scope.UserID}),
			scope, run.Key.RunID)
	}
	logger.Infof(context.Background(), "[CraftKnowledge] runtime knowledge chain assembled: per-Run builder, exact verifier and dispatch recheck")
	return nil
}

// craftKnowledgeAdmittedRunForTask loads the one durable admitted Run behind a
// craft delegation and rejects any fence/identity drift before the per-Run
// knowledge chain runs. A delegation cannot resolve another Run's package.
func craftKnowledgeAdmittedRunForTask(ctx context.Context, runs *repository.AgentRunStore, task craft.Task) (agentruntime.Run, error) {
	if runs == nil || task.Fence.TenantID == 0 || task.Fence.RunID == "" ||
		task.Fence.TenantID != task.Scope.TenantID || task.Scope.SessionID == "" ||
		task.Scope.UserID == "" || task.Fence.Owner == "" || task.Fence.Epoch < 1 {
		return agentruntime.Run{}, fmt.Errorf("%w: incomplete craft delegation fence for knowledge resolution", craft.ErrForbidden)
	}
	run, err := runs.Get(ctx, agentruntime.RunKey{TenantID: task.Fence.TenantID, RunID: task.Fence.RunID})
	if err != nil {
		return agentruntime.Run{}, fmt.Errorf("craft: load admitted Run for knowledge resolution: %w", err)
	}
	if run.SessionID != task.Scope.SessionID || run.UserID != task.Scope.UserID ||
		run.Owner == "" || run.Owner != task.Fence.Owner ||
		run.ActorUserID == "" || run.Epoch != task.Fence.Epoch {
		return agentruntime.Run{}, fmt.Errorf("%w: durable Run does not match the craft delegation fence", craft.ErrConflict)
	}
	return run, nil
}

// registerCraftKnowledgeFeature installs the T05 read surface (actual Run
// sources and per-citation open authorization) through the T00 constrained
// feature registry at central assembly time. A nil service is the recorded
// fail-closed state (no local runtime dial): the routes stay mounted and
// answer 503 explicitly instead of silently disappearing behind a 404.
func registerCraftKnowledgeFeature(svc *service.CraftKnowledgeService, routes *session.CraftFeatureRoutes) error {
	if routes == nil {
		return fmt.Errorf("Craft knowledge feature routes unavailable")
	}
	var api session.CraftKnowledgeAPI
	if svc != nil {
		api = svc
	}
	return RegisterCraftFeature(routes, "knowledge", func(group session.CraftRouteGroup) {
		session.NewCraftKnowledgeHandler(api).MountCraftKnowledgeRoutes(group)
	})
}
