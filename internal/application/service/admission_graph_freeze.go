package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// AdmissionGraphCoreFreezer freezes the server-resolved graph execution core
// (model identity, AgentConfig, runtime) for a workbench admission. Plain
// parameters keep this type independent of the workbench package (whose test
// binary imports this one); the container adapts it onto the coordinator's
// AdmissionGraphFreezer seam. Failures must carry an explicit short code
// (model_unresolved / model_unavailable / rerank_unresolved /
// config_unbuildable); the coordinator attaches its resolution-failed
// sentinel when settling the rejection.
type AdmissionGraphCoreFreezer func(ctx context.Context, tenantID uint64, actor, sessionID, agentID, text string) (json.RawMessage, error)

var (
	admissionGraphCoreMu    sync.RWMutex
	registeredAdmissionCore AdmissionGraphCoreFreezer
)

// RegisterAdmissionGraphCoreFreezer installs the production freezer built by
// the session service. Mirrors RegisterGraphExecutor: the workbench
// admission coordinator assembles independently of the session service in
// the dependency graph, so the container wires the coordinator through
// RegisteredAdmissionGraphCoreFreezer.
func RegisterAdmissionGraphCoreFreezer(f AdmissionGraphCoreFreezer) {
	admissionGraphCoreMu.Lock()
	defer admissionGraphCoreMu.Unlock()
	registeredAdmissionCore = f
}

// RegisteredAdmissionGraphCoreFreezer returns the production freezer, or nil
// when the session service has not been constructed yet.
func RegisteredAdmissionGraphCoreFreezer() AdmissionGraphCoreFreezer {
	admissionGraphCoreMu.RLock()
	defer admissionGraphCoreMu.RUnlock()
	return registeredAdmissionCore
}

// freezeAdmissionGraphCore reuses the exact capability-resolution seam the
// tRPC engine's AgentQA runs (buildAgentConfig → resolveChatModelID →
// BuildDurableRunSnapshot) so the admitted snapshot freezes the same
// server-resolved scope the builtin engine would have assembled. Every model
// and config value comes from server state; nothing is read from the client.
func (s *sessionService) freezeAdmissionGraphCore(
	ctx context.Context, tenantID uint64, actor, sessionID, agentID, text string,
) (json.RawMessage, error) {
	if strings.TrimSpace(agentID) == "" {
		return nil, graphResolutionFailure(errors.New("workbench admission requires an agent_id to freeze a graph execution identity"))
	}
	if s.customAgents == nil {
		return nil, graphResolutionFailure(errors.New("custom agent service is unavailable"))
	}
	agent, err := s.customAgents.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, graphResolutionFailure(fmt.Errorf("load agent %s: %w", agentID, err))
	}
	session, err := s.GetSessionByID(ctx, tenantID, sessionID)
	if err != nil {
		return nil, graphResolutionFailure(fmt.Errorf("load session %s: %w", sessionID, err))
	}
	// ponytail: v1 resolves the caller-tenant agent (builtin or owned);
	// cross-tenant shared/adopted agents stay on their existing admission lane
	req := &types.QARequest{Query: text, Session: session, CustomAgent: agent}
	agentTenantID := s.resolveRetrievalTenantID(ctx, req)
	var tenantInfo *types.Tenant
	if v := ctx.Value(types.TenantInfoContextKey); v != nil {
		tenantInfo, _ = v.(*types.Tenant)
	}
	if tenantInfo == nil || tenantInfo.ID != agentTenantID {
		if s.tenantService != nil {
			if agentTenant, tenantErr := s.tenantService.GetTenantByID(ctx, agentTenantID); tenantErr == nil && agentTenant != nil {
				tenantInfo = agentTenant
			}
		}
	}
	if tenantInfo == nil {
		tenantInfo = &types.Tenant{ID: agentTenantID}
	}
	req.CustomAgent.EnsureDefaults()
	agentConfig, err := s.buildAgentConfig(ctx, req, tenantInfo, agentTenantID)
	if err != nil {
		return nil, graphResolutionFailure(err)
	}
	if agent.Config.VLMModelID != "" {
		agentConfig.VLMModelID = agent.Config.VLMModelID
	}
	modelID, err := s.resolveChatModelID(ctx, req, agentConfig.KnowledgeBases, agentConfig.KnowledgeIDs)
	if err != nil {
		return nil, graphResolutionFailure(err)
	}
	if modelID == "" {
		return nil, graphResolutionFailure(fmt.Errorf("chat model is not configured: no model resolved for agent %s", agent.ID))
	}
	// Same metadata contract as AgentQA: the model's declared context window
	// sizes the engine's memory consolidator, so it must be frozen too.
	if info, infoErr := s.modelService.GetModelByID(ctx, modelID); infoErr == nil && info != nil {
		agentConfig.MaxContextTokens = types.AgentMaxContextTokens(agentConfig.MaxContextTokens, info.Parameters.ContextWindow)
	}
	rerankModelID := ""
	if agentRequiresRerankModel(agent) {
		rerankModelID = agent.Config.RerankModelID
		if rerankModelID == "" {
			return nil, graphResolutionFailure(errors.New("rerank model is not configured: please set rerank_model_id on the agent"))
		}
	}
	logger.Infof(ctx, "Froze workbench graph core for tenant %d actor %s agent %s model %s", tenantID, actor, agent.ID, modelID)
	return BuildDurableRunSnapshot(text, nil, modelID, rerankModelID, agentConfig)
}

// graphResolutionFailure maps a resolution-chain error onto the admission
// rejection short codes. The mapping is fail-closed: any unrecognized failure
// still rejects admission with an explicit reason instead of degrading.
func graphResolutionFailure(err error) error {
	code := "config_unbuildable"
	message := err.Error()
	switch {
	case strings.Contains(message, "chat model is not configured"):
		code = "model_unresolved"
	case strings.Contains(message, "is unavailable"):
		code = "model_unavailable"
	case strings.Contains(message, "rerank model is not configured"):
		code = "rerank_unresolved"
	}
	return fmt.Errorf("%s: %v", code, err)
}
