package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	trpcagent "github.com/Tencent/WeKnora/internal/agent/trpc"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/types"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// durableRunSnapshotVersion versions the admission snapshot schema. Unknown
// versions are rejected instead of leniently decoded.
const durableRunSnapshotVersion = 1

// RunConfigRuntimeSnapshot carries the AgentConfig fields whose standard JSON
// encoding is runtime-only. Recovery must rebuild the exact request-scoped
// assembly, so they cross the admission boundary explicitly.
type RunConfigRuntimeSnapshot struct {
	SandboxConfigID     string                     `json:"sandbox_config_id,omitempty"`
	VLMModelID          string                     `json:"vlm_model_id,omitempty"`
	TenantSkills        []*types.TenantSkillEntity `json:"tenant_skills,omitempty"`
	PinnedMCPServiceIDs []string                   `json:"pinned_mcp_service_ids,omitempty"`
	PinnedSkillNames    []string                   `json:"pinned_skill_names,omitempty"`
	SharedAgentReadOnly bool                       `json:"shared_agent_read_only,omitempty"`
	SearchTargets       types.SearchTargets        `json:"search_targets,omitempty"`
}

// DurableRunSnapshot is the immutable request identity persisted with an
// admitted tRPC run. Model and tool identities are frozen here; live
// credentials, registries and permissions are always re-resolved from the
// current database state when the run executes, so a narrowed permission or a
// removed model fails recovery with a concrete reason instead of being
// overridden by the snapshot.
type DurableRunSnapshot struct {
	Version       int                      `json:"version"`
	Query         string                   `json:"query"`
	ImageURLs     []string                 `json:"image_urls,omitempty"`
	ModelID       string                   `json:"model_id"`
	RerankModelID string                   `json:"rerank_model_id,omitempty"`
	AgentConfig   json.RawMessage          `json:"agent_config"`
	Runtime       RunConfigRuntimeSnapshot `json:"runtime"`
}

// BuildDurableRunSnapshot freezes the resolved request configuration at
// admission time. It is called after the shared capability resolution so the
// snapshot reflects the same scope the builtin engine would have used.
func BuildDurableRunSnapshot(
	query string,
	images []string,
	modelID, rerankModelID string,
	config *types.AgentConfig,
) (json.RawMessage, error) {
	if query == "" || modelID == "" || config == nil {
		return nil, fmt.Errorf("durable run snapshot requires a query, model id and agent config")
	}
	cfgRaw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal agent config: %w", err)
	}
	snapshot := DurableRunSnapshot{
		Version:       durableRunSnapshotVersion,
		Query:         query,
		ImageURLs:     append([]string(nil), images...),
		ModelID:       modelID,
		RerankModelID: rerankModelID,
		AgentConfig:   cfgRaw,
		Runtime: RunConfigRuntimeSnapshot{
			SandboxConfigID:     config.SandboxConfigID,
			VLMModelID:          config.VLMModelID,
			TenantSkills:        config.TenantSkills,
			PinnedMCPServiceIDs: config.PinnedMCPServiceIDs,
			PinnedSkillNames:    config.PinnedSkillNames,
			SharedAgentReadOnly: config.SharedAgentReadOnly,
			SearchTargets:       config.SearchTargets,
		},
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal durable run snapshot: %w", err)
	}
	return raw, nil
}

// ParseDurableRunSnapshot strictly decodes an admitted snapshot and rejects
// unknown versions instead of decoding them into empty values.
func ParseDurableRunSnapshot(raw json.RawMessage) (DurableRunSnapshot, error) {
	var snapshot DurableRunSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return DurableRunSnapshot{}, fmt.Errorf("decode durable run snapshot: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return DurableRunSnapshot{}, fmt.Errorf("durable run snapshot has trailing data")
	}
	if snapshot.Version != durableRunSnapshotVersion {
		return DurableRunSnapshot{}, fmt.Errorf("unsupported durable run snapshot version %d", snapshot.Version)
	}
	if snapshot.Query == "" || snapshot.ModelID == "" || len(snapshot.AgentConfig) == 0 {
		return DurableRunSnapshot{}, fmt.Errorf("durable run snapshot is missing query, model id or agent config")
	}
	return snapshot, nil
}

// RestoreAgentConfig rebuilds the resolved AgentConfig, re-attaching the
// runtime-only fields the plain JSON encoding drops.
func (s DurableRunSnapshot) RestoreAgentConfig() (*types.AgentConfig, error) {
	var config types.AgentConfig
	decoder := json.NewDecoder(bytes.NewReader(s.AgentConfig))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode agent config: %w", err)
	}
	config.SandboxConfigID = s.Runtime.SandboxConfigID
	config.VLMModelID = s.Runtime.VLMModelID
	config.TenantSkills = s.Runtime.TenantSkills
	config.PinnedMCPServiceIDs = s.Runtime.PinnedMCPServiceIDs
	config.PinnedSkillNames = s.Runtime.PinnedSkillNames
	config.SharedAgentReadOnly = s.Runtime.SharedAgentReadOnly
	config.SearchTargets = s.Runtime.SearchTargets
	return &config, nil
}

var (
	graphExecutorMu    sync.RWMutex
	registeredExecutor func(context.Context, agentruntime.Fence) error
)

// RegisterGraphExecutor installs the production graph executor built by the
// session service. The container wires the worker lazily through
// RegisteredGraphExecutor because the runtime is constructed before the
// session service in the dependency graph.
func RegisterGraphExecutor(fn func(context.Context, agentruntime.Fence) error) {
	graphExecutorMu.Lock()
	defer graphExecutorMu.Unlock()
	registeredExecutor = fn
}

// RegisteredGraphExecutor returns the production executor, or nil when the
// session service has not been constructed yet.
func RegisteredGraphExecutor() func(context.Context, agentruntime.Fence) error {
	graphExecutorMu.RLock()
	defer graphExecutorMu.RUnlock()
	return registeredExecutor
}

// submitDurableAgentRun admits a tRPC-engine request after the shared
// capability resolution has produced the agent config and model identity. The
// snapshot freezes that resolution; execution happens on the durable worker so
// an HTTP/SSE disconnect never cancels the run.
func (s *sessionService) submitDurableAgentRun(
	ctx context.Context,
	req *types.QARequest,
	config *types.AgentConfig,
	modelID string,
	supportsVision bool,
) error {
	if s.cfg == nil || s.cfg.Agent == nil || !s.cfg.Agent.Recovery.AdmissionEnabled {
		return errors.New("tRPC agent runs are disabled")
	}
	runs := RegisteredAgentRunService()
	if runs == nil {
		return errors.New("tRPC agent run service is unavailable")
	}
	query := req.Query
	if req.QuotedContext != "" {
		query += "\n\n" + req.QuotedContext
	}
	if len(req.Attachments) > 0 {
		query += req.Attachments.BuildPrompt()
	}
	images := req.ImageURLs
	if !supportsVision {
		images = nil
		if req.ImageDescription != "" {
			query += "\n\n[用户上传图片内容]\n" + req.ImageDescription
		}
	}
	rerankModelID := ""
	if agentRequiresRerankModel(req.CustomAgent) {
		rerankModelID = req.CustomAgent.Config.RerankModelID
	}
	snapshot, err := BuildDurableRunSnapshot(query, images, modelID, rerankModelID, config)
	if err != nil {
		return fmt.Errorf("build durable run snapshot: %w", err)
	}
	// The request hash binds the idempotency key to the admitted content: a
	// retried request key with different content is a conflict, not a replay.
	digest := sha256.Sum256(append(append([]byte(nil), snapshot...), []byte(req.AssistantMessageID)...))
	user, err := json.Marshal(map[string]any{"role": "user", "content": query})
	if err != nil {
		return err
	}
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	if err != nil {
		return err
	}
	_, err = runs.Submit(ctx, agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: req.Session.TenantID, RunID: uuid.NewString()},
		SessionID:          req.Session.ID,
		UserID:             req.Session.UserID,
		RequestID:          uuid.NewString(),
		AssistantMessageID: req.AssistantMessageID,
		RequestHash:        hex.EncodeToString(digest[:]),
		Snapshot:           snapshot,
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           time.Now().Add(30 * time.Minute),
	})
	if err != nil {
		logger.Warnf(ctx, "durable run admission failed for session %s: %v", req.Session.ID, err)
	}
	return err
}

// durableHistoryTurns mirrors the builtin engine history defaults.
func durableHistoryTurns(config *types.AgentConfig) int {
	if config == nil || !config.MultiTurnEnabled {
		return 0
	}
	if config.HistoryTurns > 0 {
		return config.HistoryTurns
	}
	return 5
}

// chatHistoryToModelMessages converts business history into the SDK message
// shape used by the durable graph. History replays as text and tool-call
// records; images never cross a history boundary.
func chatHistoryToModelMessages(history []chat.Message) ([]model.Message, error) {
	out := make([]model.Message, 0, len(history))
	for _, in := range history {
		msg := model.Message{
			Role:             model.Role(in.Role),
			Content:          in.Content,
			ToolID:           in.ToolCallID,
			ToolName:         in.Name,
			ReasoningContent: in.ReasoningContent,
		}
		switch msg.Role {
		case model.RoleSystem, model.RoleUser, model.RoleAssistant, model.RoleTool:
		default:
			return nil, fmt.Errorf("unsupported history role %q", in.Role)
		}
		for _, call := range in.ToolCalls {
			arguments := json.RawMessage(call.Function.Arguments)
			if len(arguments) == 0 || !json.Valid(arguments) {
				return nil, fmt.Errorf("history tool call %s has invalid arguments", call.ID)
			}
			msg.ToolCalls = append(msg.ToolCalls, model.ToolCall{
				ID: call.ID, Type: call.Type,
				Function: model.FunctionDefinitionParam{Name: call.Function.Name, Arguments: arguments},
			})
		}
		out = append(out, msg)
	}
	return out, nil
}

// durableUserMessage rebuilds the entry message of the run, including
// multimodal content when the resolved model accepted images for this request.
func durableUserMessage(snapshot DurableRunSnapshot) (model.Message, error) {
	msg := model.NewUserMessage(snapshot.Query)
	for _, url := range snapshot.ImageURLs {
		if url == "" {
			return model.Message{}, fmt.Errorf("durable run snapshot has an empty image reference")
		}
		msg.ContentParts = append(msg.ContentParts, model.ContentPart{
			Type: model.ContentTypeImage, Image: &model.Image{URL: url},
		})
	}
	return msg, nil
}

// ExecuteDurableRun is the production graph executor claimed by the durable
// worker. It re-resolves the model, tools, skills and permissions from the
// current database state, rebuilds the capability assembly through the same
// prepareAgentCapabilities path used by the builtin engine, and drives the
// tRPC graph with the repository checkpoint saver, tool journal, durable
// decision waiter and finalize transaction.
func (s *sessionService) ExecuteDurableRun(ctx context.Context, fence agentruntime.Fence) error {
	runs := RegisteredAgentRunService()
	if runs == nil || runs.Store() == nil {
		return errors.New("durable agent run service is not registered")
	}
	store := runs.Store()
	run, err := store.Get(ctx, fence.RunKey)
	if err != nil {
		return err
	}
	if run.Owner != fence.Owner || run.Epoch != fence.Epoch {
		return fmt.Errorf("%w: durable run fence superseded", agentruntime.ErrLeaseLost)
	}
	snapshot, err := ParseDurableRunSnapshot(run.Snapshot)
	if err != nil {
		return fmt.Errorf("durable run %s: %w", fence.RunID, err)
	}
	config, err := snapshot.RestoreAgentConfig()
	if err != nil {
		return fmt.Errorf("durable run %s: %w", fence.RunID, err)
	}

	// Model identity is frozen by the snapshot; credentials and availability
	// are not. A removed or unreachable model fails the run with a concrete
	// reason and never silently switches implementations.
	chatModel, err := s.modelService.GetChatModel(ctx, snapshot.ModelID)
	if err != nil {
		return fmt.Errorf("model %q unavailable for durable run %s: %w", snapshot.ModelID, fence.RunID, err)
	}
	var rerankModel rerank.Reranker
	if snapshot.RerankModelID != "" {
		rerankModel, err = s.modelService.GetRerankModel(ctx, snapshot.RerankModelID)
		if err != nil {
			return fmt.Errorf(
				"rerank model %q unavailable for durable run %s: %w",
				snapshot.RerankModelID, fence.RunID, err)
		}
	}

	preparer, ok := s.agentService.(*agentService)
	if !ok {
		return errors.New("agent service does not support durable capability assembly")
	}
	// The event bus is run-scoped: durable runs never attach the SSE handler
	// lifecycle, tools still receive a usable bus instance.
	bus := event.NewEventBus()
	caps, err := preparer.prepareAgentCapabilities(
		ctx, config, chatModel, rerankModel, bus, run.SessionID, run.AssistantMessageID)
	if err != nil {
		return fmt.Errorf("rebuild capabilities for durable run %s: %w", fence.RunID, err)
	}

	// A fresh run imports the business history exactly once, together with
	// the memory recall envelope; resumed runs restore the checkpointed
	// message list and never re-import or re-recall.
	_, checkpointErr := store.LoadCheckpoint(ctx, fence.RunKey)
	fresh := errors.Is(checkpointErr, agentruntime.ErrNotFound)
	if checkpointErr != nil && !fresh {
		return fmt.Errorf("durable run %s checkpoint unavailable: %w", fence.RunID, checkpointErr)
	}
	capabilities := caps.CapabilitySnapshot()
	initial := trpcagent.State{Version: trpcagent.StateVersion, Capabilities: capabilities}
	if fresh {
		history, historyErr := LoadAgentHistory(ctx, s.messageRepo, run.SessionID, durableHistoryTurns(config))
		if historyErr != nil {
			logger.Warnf(ctx, "durable run %s history load failed: %v", fence.RunID, historyErr)
			history = nil
		}
		messages, convertErr := chatHistoryToModelMessages(history)
		if convertErr != nil {
			return fmt.Errorf("durable run %s: %w", fence.RunID, convertErr)
		}
		entry, entryErr := durableUserMessage(snapshot)
		if entryErr != nil {
			return fmt.Errorf("durable run %s: %w", fence.RunID, entryErr)
		}
		initial.Messages = append(messages, entry)
		if s.memoryService != nil {
			memoryCtx := types.ApplyAgentMemoryPreference(ctx, config.MemoryEnabled)
			if recall := s.memoryService.Recall(memoryCtx, snapshot.Query); recall.Prompt != "" {
				capabilities.MemoryPrompt = recall.Prompt
				initial.Capabilities = capabilities
			}
		}
	}

	journal, ok := store.(agentruntime.ToolJournal)
	if !ok {
		return errors.New("run store does not provide the durable tool journal")
	}
	inputSource, ok := store.(trpcagent.RunInputSource)
	if !ok {
		return errors.New("run store does not provide the durable steering inputs")
	}
	eventStore, ok := store.(agentruntime.RunEventStore)
	if !ok {
		return errors.New("run store does not provide the durable event store")
	}
	emit := func(ectx context.Context, evt agentruntime.RunEvent) {
		if _, err := eventStore.AppendEvent(ectx, fence, evt); err != nil {
			logger.Warnf(ectx, "durable run %s event %s not persisted: %v", fence.RunID, evt.Type, err)
		}
	}
	if raw, merr := json.Marshal(map[string]string{"run_id": fence.RunID, "session_id": run.SessionID}); merr == nil {
		emit(ctx, agentruntime.RunEvent{Type: "run_started", Payload: raw})
	}

	modelTools, err := trpcagent.DeclarationTools(caps.Tools.GetModelFunctionDefinitions())
	if err != nil {
		return fmt.Errorf("durable run %s tool declarations: %w", fence.RunID, err)
	}
	executeTool := func(tctx context.Context, name string, args json.RawMessage) (*types.ToolResult, error) {
		tctx = types.WithSessionID(tctx, run.SessionID)
		// MCP wrappers read the exec context for approval/OAuth metadata;
		// without it the durable path would fail the preflight instead of
		// parking. The run-scoped bus is detached from SSE lifetimes.
		meta := &tools.ToolExecContext{
			SessionID:          run.SessionID,
			AssistantMessageID: run.AssistantMessageID,
			RequestID:          run.RequestID,
			UserID:             run.UserID,
			EventBus:           bus,
			ApprovalCtx:        tctx,
		}
		if dispatch, ok := agentruntime.ToolDispatchFromContext(tctx); ok {
			meta.ToolCallID = dispatch.CallID
		}
		tctx = tools.WithToolExecContext(tctx, meta)
		// Directory discovery and the call proxy are already dispatched when
		// they hit OAuth, so a durable park there would misclassify as an
		// unknown outcome; the non-interactive path returns a notice instead.
		if name == tools.ToolDiscoverMCPTools || name == tools.ToolCallMCPTool {
			tctx = types.WithMCPOAuthNonInteractive(tctx)
		}
		return caps.Tools.ExecuteTool(tctx, name, args)
	}
	executorTools := agentruntime.NewToolExecutor(store, journal, executeTool)
	bindings := trpcagent.GraphBindings{
		Model:           trpcagent.NewModel(caps.Chat),
		Store:           store,
		Tools:           executorTools,
		Finalize:        eventStore.Finalize,
		WaitForDecision: runs.WaitForDecision,
		InitialState:    initial,
		Capabilities:    capabilities,
		Events:          eventStore,
		ModelTools:      modelTools,
		Inputs:          inputSource,
	}
	runner, err := trpcagent.NewGraphRunner(bindings)
	if err != nil {
		return fmt.Errorf("build graph for durable run %s: %w", fence.RunID, err)
	}
	execErr := runner.Run(ctx, fence)
	if execErr != nil {
		payload := map[string]string{"error": execErr.Error()}
		eventType := "run_failed"
		if errors.Is(execErr, agentruntime.ErrMCPOAuthWait) {
			// The run is durably parked waiting for the user to reconnect
			// authorization; this is a wait, not a failure.
			eventType = "run_waiting"
			payload = map[string]string{"wait_kind": "mcp_oauth"}
		}
		if raw, merr := json.Marshal(payload); merr == nil {
			emit(context.WithoutCancel(ctx), agentruntime.RunEvent{Type: eventType, Payload: raw})
		}
		return execErr
	}
	admitAfterFollowUps(context.WithoutCancel(ctx), store, fence.RunKey, snapshot)
	trimRetainedEvents(context.WithoutCancel(ctx), store, fence.RunKey)
	return nil
}

// durableEventRetention bounds how many replay events each finished run
// retains; older events are trimmed so reconnecting clients with stale
// cursors receive the explicit reload error instead of silent gaps.
const durableEventRetention = 1000

// admitAfterFollowUps enqueues the next durable run for steering messages
// that arrived with delivery=after: they were parked on the finished run
// and are only admissible once it reached a terminal state (spec 11).
func admitAfterFollowUps(
	ctx context.Context, store agentruntime.RunStore,
	key agentruntime.RunKey, snapshot DurableRunSnapshot,
) {
	reader, ok := store.(trpcagent.RunInputSource)
	if !ok {
		return
	}
	pending, err := reader.ListPendingInputs(ctx, key, "after")
	if err != nil || len(pending) == 0 {
		return
	}
	first := pending[0]
	var payload struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if jerr := json.Unmarshal(first.Message, &payload); jerr != nil || payload.Content == "" {
		logger.Warnf(ctx, "durable run %s after input %s invalid", key.RunID, first.SteerID)
		return
	}
	followSnapshot := snapshot
	followSnapshot.Query = payload.Content
	runs := RegisteredAgentRunService()
	if runs == nil {
		return
	}
	raw, merr := json.Marshal(followSnapshot)
	if merr != nil {
		return
	}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": payload.Content})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	digest := sha256.Sum256(append(append([]byte(nil), raw...), []byte(first.SteerID)...))
	next := agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: key.TenantID, RunID: uuid.NewString()},
		SessionID:          runField(store, key, func(r agentruntime.Run) string { return r.SessionID }),
		UserID:             runField(store, key, func(r agentruntime.Run) string { return r.UserID }),
		RequestID:          "followup-" + first.SteerID,
		AssistantMessageID: uuid.NewString(),
		RequestHash:        hex.EncodeToString(digest[:]),
		Snapshot:           raw,
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           time.Now().Add(30 * time.Minute),
	}
	if _, serr := runs.Submit(ctx, next); serr != nil {
		logger.Warnf(ctx, "durable run %s follow-up admission failed: %v", key.RunID, serr)
		return
	}
	if consumer, cok := store.(agentruntime.RunInputConsumer); cok {
		_ = consumer.MarkInputsProcessed(ctx, key, first.SteerID)
	}
	logger.Infof(ctx, "durable run %s admitted follow-up run %s for steer %s",
		key.RunID, next.Key.RunID, first.SteerID)
}

func runField(store agentruntime.RunStore, key agentruntime.RunKey, pick func(agentruntime.Run) string) string {
	if run, err := store.Get(context.Background(), key); err == nil {
		return pick(run)
	}
	return ""
}

func trimRetainedEvents(ctx context.Context, store agentruntime.RunStore, key agentruntime.RunKey) {
	trimmer, ok := store.(agentruntime.RunEventTrimmer)
	if !ok {
		return
	}
	newest, err := trimmer.LastEventSeq(ctx, key)
	if err != nil || newest <= durableEventRetention {
		return
	}
	watermark := newest - durableEventRetention + 1
	if watermark <= 0 {
		return
	}
	if _, err := trimmer.TrimEventsBefore(ctx, key, watermark); err != nil {
		logger.Warnf(ctx, "durable run %s event trim failed: %v", key.RunID, err)
	}
}
