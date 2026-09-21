package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent"
	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/agent/persona"
	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	trpcagent "github.com/Tencent/WeKnora/internal/agent/trpc"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/rerank"
	"github.com/Tencent/WeKnora/internal/modules/execution/browserskill"
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
	// SystemPrompt is the agent's custom prompt TEMPLATE only — the builtin
	// ReAct engine resolves it (or falls back to the default scaffolding when
	// empty); it never carries the persona segment.
	SystemPrompt string
	// PersonaSegment is the rendered persona block, kept separate so the
	// builtin engine prepends it in front of whichever template applies
	// instead of the segment becoming the whole prompt.
	PersonaSegment string
	EventBus       *event.EventBus
	PinnedMCP      []*agent.PinnedMCPServiceInfo
	PinnedSkills   []*agent.PinnedSkillInfo
	ImageDescriber agent.ImageDescriberFunc
	// Approval policy and waiting are separate capabilities. Recovery may use
	// the policy checker while durable decisions never enter the live waiter.
	ApprovalChecker approval.ApprovalChecker
	ApprovalWaiter  approval.ApprovalWaiter
	MemoryPrompt    string
	ImageReferences []string
}

// CapabilitySnapshot is the restart boundary for shared agent capabilities.
// It is assembled from the same request-scoped registry used by builtin ReAct.
func (c *AgentCapabilities) CapabilitySnapshot() trpcagent.CapabilitySnapshot {
	if c == nil {
		return trpcagent.CapabilitySnapshot{}
	}
	// The durable graph inserts the snapshot's system prompt verbatim as a
	// system message (trpc/graph.go nodePrepare), so the persona segment is
	// joined here — in the same layout the builtin engine's prepend produces.
	s := trpcagent.CapabilitySnapshot{
		SystemPrompt:    agent.PrependPersonaSegment(c.PersonaSegment, c.SystemPrompt),
		MemoryPrompt:    c.MemoryPrompt,
		ImageReferences: append([]string(nil), c.ImageReferences...),
	}
	if c.Tools != nil {
		for _, definition := range c.Tools.GetFunctionDefinitions() {
			s.ToolIdentities = append(s.ToolIdentities, definition.Name)
		}
		s.DeferredNames = c.Tools.DeferredToolNames()
	}
	if c.Skills != nil {
		s.SkillDigests = make(map[string]string)
		for _, metadata := range c.Skills.GetAllMetadata() {
			if metadata != nil {
				s.SkillDigests[metadata.Name] = trpcagent.SkillDigest(metadata.Name, metadata.Description)
			}
		}
	}
	return s
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
	if config.LocalBrowserEnabled && (!s.browserSkill.Enabled() || config.SkillInstallMode()) {
		return nil, fmt.Errorf("local browser is unavailable for this turn; " +
			"enable the browser integration or update the input-bar selection")
	}

	toolRegistry := tools.NewToolRegistry()
	if config.MaxToolOutputChars > 0 {
		toolRegistry.SetMaxToolOutputSize(config.MaxToolOutputChars)
	}
	if err := s.registerTools(ctx, toolRegistry, config, rerankModel, chatModel, eventBus, sessionID); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}
	s.registerMCPTools(ctx, toolRegistry, config)
	// The open-connector app tool is mounted on the authenticated session
	// assembly path itself (gated by installation visibility), not through
	// the AllowedTools allowlist: like MCP tools, it is a capability the
	// workspace's installed apps expose, and per-action authorization stays
	// with the facade and the actions surface.
	s.registerOpenConnectorTool(ctx, toolRegistry)

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
	personaSegment := renderPersonaSegment(ctx, config)

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
		PersonaSegment: personaSegment,
		EventBus:       eventBus,
		PinnedMCP:      pinnedMCP,
		PinnedSkills:   s.resolvePinnedSkillInfos(config),
	}
	if s.toolApprovalGate != nil {
		capabilities.ApprovalChecker, _ = s.toolApprovalGate.(approval.ApprovalChecker)
		capabilities.ApprovalWaiter, _ = s.toolApprovalGate.(approval.ApprovalWaiter)
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

	// Browser operations are native BrowserSkill RPCs, independent of shell and sandbox setup.
	if config.LocalBrowserEnabled && s.browserSkill.Enabled() && !config.SkillInstallMode() {
		tenant, _ := types.TenantIDFromContext(ctx)
		user, _ := types.UserIDFromContext(ctx)
		scope := browserskill.Scope{Tenant: tenant, User: user}
		instructions, err := s.browserSearchInstructions(ctx)
		if err != nil {
			return nil, err
		}
		toolRegistry.RegisterTool(tools.NewBrowserSkillTool(s.browserSkill, scope, sessionID, instructions))
	}

	return capabilities, nil
}

// personaAgentNameFallback fills the persona template's agent-name slot.
// types.AgentConfig carries no display-name field — the custom agent's name is
// not threaded into capability assembly — and persona.RenderPersona has no
// name fallback of its own, so without this the segment would read
// "You are , an AI assistant…". Keeping it English inside a zh segment matches
// RenderPersona's own "the user" fallback.
const personaAgentNameFallback = "the assistant"

// renderPersonaSegment renders the agent's MBTI persona block. It is carried
// separately from the custom prompt template on AgentCapabilities: the builtin
// ReAct engine prepends it in front of whichever template applies (custom or
// the default scaffolding) so a persona-only agent keeps its retrieval,
// citation and language rules, while the durable trpc path joins it in front of
// the system content verbatim (see CapabilitySnapshot). An unset or unknown
// PersonaMBTI yields an empty segment — including PersonaStyle, which only
// renders when PersonaMBTI is set.
func renderPersonaSegment(ctx context.Context, config *types.AgentConfig) string {
	if config == nil || config.PersonaMBTI == "" {
		return ""
	}
	// RenderPersona uppercases the code before its own lookup; the gate does
	// the same so a case-mismatched stored value keeps its persona instead of
	// silently dropping it.
	code := strings.ToUpper(strings.TrimSpace(config.PersonaMBTI))
	if _, ok := persona.Profile(code); !ok {
		logger.Warnf(ctx, "agent has unknown persona_mbti %q; skipping persona segment", config.PersonaMBTI)
		return ""
	}
	locale := types.LanguageFromContextOrDefault(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	return persona.RenderPersona(code, locale, persona.RenderInput{
		AgentName:   personaAgentNameFallback,
		UserDisplay: userID,
		Custom:      config.PersonaStyle,
	})
}
