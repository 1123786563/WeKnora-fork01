package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/agent"
	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/types"
)

// AgentCapabilities contains the request-scoped dependencies shared by agent
// engine implementations. Its mutable registry and skills manager are created
// for one session execution and must not be reused by another session.
type AgentCapabilities struct {
	Config         *types.AgentConfig
	Chat           chat.Chat
	Rerank         rerank.Reranker
	Tools          *tools.ToolRegistry
	Skills         *skills.Manager
	KnowledgeBases []*agent.KnowledgeBaseInfo
	Documents      []*agent.SelectedDocumentInfo
	SystemPrompt   string
	EventBus       *event.EventBus
	PinnedMCP      []*agent.PinnedMCPServiceInfo
	PinnedSkills   []*agent.PinnedSkillInfo
	ImageDescriber agent.ImageDescriberFunc
}

func (s *agentService) prepareAgentCapabilities(
	ctx context.Context,
	config *types.AgentConfig,
	chatModel chat.Chat,
	rerankModel rerank.Reranker,
	eventBus *event.EventBus,
	sessionID, assistantMessageID string,
) (*AgentCapabilities, error) {
	if err := s.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}
	if chatModel == nil {
		return nil, fmt.Errorf("chat model is nil after initialization")
	}

	toolRegistry := tools.NewToolRegistry()
	if config.MaxToolOutputChars > 0 {
		toolRegistry.SetMaxToolOutputSize(config.MaxToolOutputChars)
	}
	if err := s.registerTools(ctx, toolRegistry, config, rerankModel, chatModel, sessionID); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}
	s.registerMCPTools(ctx, toolRegistry, config)

	// Register the shell first: file discovery needs a separate tool only
	// when no shell is available. File access remains a sandbox capability.
	s.registerSandboxShellIfAllowed(ctx, toolRegistry, sessionID, config)
	s.registerSandboxFileTools(ctx, toolRegistry, sessionID, config)
	s.registerWebPageFiles(ctx, toolRegistry, config, sessionID, assistantMessageID)
	toolRegistry.PrepareMCPTools(ctx)

	knowledgeBases, documents := s.resolveKBAndDocInfos(ctx, config)
	systemPrompt := ""
	if config.UseCustomSystemPrompt || config.SystemPrompt != "" {
		systemPrompt = config.ResolveSystemPrompt(config.WebSearchEnabled)
	}

	pinnedMCP := s.resolvePinnedMCPServiceInfos(ctx, config)
	s.attachPinnedMCPToolNames(toolRegistry, pinnedMCP)

	capabilities := &AgentCapabilities{
		Config:         config,
		Chat:           chatModel,
		Rerank:         rerankModel,
		Tools:          toolRegistry,
		KnowledgeBases: knowledgeBases,
		Documents:      documents,
		SystemPrompt:   systemPrompt,
		EventBus:       eventBus,
		PinnedMCP:      pinnedMCP,
		PinnedSkills:   s.resolvePinnedSkillInfos(config),
	}

	if config.VLMModelID != "" {
		if vlmModel, err := s.modelService.GetVLMModel(ctx, config.VLMModelID); err == nil {
			capabilities.ImageDescriber = func(ctx context.Context, imgBytes []byte, prompt string) (string, error) {
				return vlmModel.Predict(ctx, [][]byte{imgBytes}, prompt)
			}
			logger.Infof(ctx, "VLM image describer set for MCP tool result analysis (model: %s)", config.VLMModelID)
		} else {
			logger.Warnf(ctx, "Failed to load VLM model %s for MCP image fallback: %v", config.VLMModelID, err)
		}
	}

	// TenantSkills is the sandbox image. SkillDirs is retained for host-backed
	// tests and legacy callers. An empty or installing image keeps its shell but
	// does not advertise a skills manager that cannot successfully read skills.
	offerSkills := config.SkillsEnabled &&
		(len(config.SkillDirs) > 0 || len(config.TenantSkills) > 0)
	if offerSkills {
		skillsManager, err := s.initializeSkillsManager(ctx, sessionID, config, toolRegistry)
		if err != nil {
			logger.Warnf(ctx, "Failed to initialize skills manager: %v", err)
		} else if skillsManager != nil {
			capabilities.Skills = skillsManager
			logger.Infof(ctx, "Skills manager initialized with %d skills",
				len(skillsManager.GetAllMetadata()))
		}
	}

	return capabilities, nil
}
