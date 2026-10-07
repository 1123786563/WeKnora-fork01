package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/models/chat"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openDurableRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := cloneMigratedSQLiteDB(t, "durable-runs.db")
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id)"+
			" VALUES ('u1','u1','u1@example.test','x',1)").Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at)
		VALUES (1,'u1','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)"+
			" VALUES ('s1',1,'session-1','u1','trpc')").Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

type durableRunModelService struct {
	interfaces.ModelService
	chat chat.Chat
}

type recordingDurableRunChat struct {
	messages []chat.Message
}

type scriptedMCPDurableRunChat struct {
	mu    sync.Mutex
	calls int
}

func (m *scriptedMCPDurableRunChat) Chat(
	ctx context.Context, messages []chat.Message, opts *chat.ChatOptions,
) (*types.ChatResponse, error) {
	stream, err := m.ChatStream(ctx, messages, opts)
	if err != nil {
		return nil, err
	}
	var response *types.ChatResponse
	for item := range stream {
		if item.Done {
			response = &types.ChatResponse{
				Content: item.Content, ToolCalls: item.ToolCalls,
				FinishReason: item.FinishReason,
			}
		}
	}
	if response == nil {
		return nil, errors.New("scripted MCP model returned no terminal response")
	}
	return response, nil
}

func (m *scriptedMCPDurableRunChat) ChatStream(
	_ context.Context, messages []chat.Message, _ *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	m.mu.Lock()
	step := m.calls
	m.calls++
	m.mu.Unlock()

	response := types.StreamResponse{Done: true, FinishReason: "tool_calls"}
	switch step {
	case 0:
		args, _ := json.Marshal(map[string]string{
			"mode": "describe", "server_id": "orders", "tool_name": "get_order",
		})
		response.ResponseType = types.ResponseTypeToolCall
		response.ToolCalls = []types.LLMToolCall{{
			ID: "discover-1", Type: "function",
			Function: types.FunctionCall{Name: tools.ToolDiscoverMCPTools, Arguments: string(args)},
		}}
	case 1:
		var discovery struct {
			ToolRef string `json:"tool_ref"`
		}
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "tool" {
				if err := json.Unmarshal([]byte(messages[i].Content), &discovery); err != nil {
					return nil, fmt.Errorf("decode MCP discovery result: %w", err)
				}
				break
			}
		}
		if strings.TrimSpace(discovery.ToolRef) == "" {
			return nil, fmt.Errorf("MCP discovery did not return one callable tool: %#v", discovery)
		}
		args, _ := json.Marshal(map[string]any{
			"tool_ref":  discovery.ToolRef,
			"arguments": map[string]string{"id": "order-42"},
		})
		response.ResponseType = types.ResponseTypeToolCall
		response.ToolCalls = []types.LLMToolCall{{
			ID: "call-1", Type: "function",
			Function: types.FunctionCall{Name: tools.ToolCallMCPTool, Arguments: string(args)},
		}}
	default:
		response.ResponseType = types.ResponseTypeAnswer
		response.Content = "order-42"
		response.FinishReason = "stop"
	}
	stream := make(chan types.StreamResponse, 1)
	stream <- response
	close(stream)
	return stream, nil
}

func (*scriptedMCPDurableRunChat) GetModelName() string { return "scripted-mcp-durable-chat" }
func (*scriptedMCPDurableRunChat) GetModelID() string   { return "scripted-mcp-durable-chat-id" }

func (m *recordingDurableRunChat) Chat(_ context.Context, messages []chat.Message, _ *chat.ChatOptions) (*types.ChatResponse, error) {
	m.messages = append([]chat.Message(nil), messages...)
	return &types.ChatResponse{Content: "ok", FinishReason: "stop"}, nil
}

func (m *recordingDurableRunChat) ChatStream(ctx context.Context, messages []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	response, err := m.Chat(ctx, messages, opts)
	if err != nil {
		return nil, err
	}
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: response.Content, Done: true, FinishReason: response.FinishReason}
	close(out)
	return out, nil
}

func (*recordingDurableRunChat) GetModelName() string { return "recording-durable-chat" }
func (*recordingDurableRunChat) GetModelID() string   { return "recording-durable-chat-id" }

func (s *durableRunModelService) GetChatModel(context.Context, string) (chat.Chat, error) {
	return s.chat, nil
}

type durableRunMessageRepo struct {
	interfaces.MessageRepository
	rows []*types.Message
}

func (r *durableRunMessageRepo) GetRecentMessagesBySession(context.Context, string, int) ([]*types.Message, error) {
	return r.rows, nil
}

// LoadAgentHistory pages history backwards and opens with a checkpoint
// lookup; both land here so the nil embedded interface is never reached.
func (r *durableRunMessageRepo) GetLatestContextCheckpoint(context.Context, string) (*types.Message, error) {
	return nil, nil
}

func (r *durableRunMessageRepo) ListMessagesBySessionBeforeCursor(
	context.Context, string, time.Time, string, int,
) ([]*types.Message, error) {
	return r.rows, nil
}

func newDurableRunSessionService(t *testing.T, _ *gorm.DB) *sessionService {
	t.Helper()
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	return &sessionService{
		cfg:             nil,
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: &fakeAgentChatModel{}},
		agentService:    &agentService{mcpManager: manager},
		memoryService:   nil,
		craftTaskAccess: &runActorTaskAccessChecker{},
	}
}

func durableRunCtx() context.Context {
	return types.WithPrincipal(
		context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)),
		types.Principal{Type: types.PrincipalWebUser, ID: "u1"},
	)
}

func admitDurableRun(t *testing.T, store *repository.AgentRunStore, snapshot json.RawMessage) agentruntime.RunKey {
	t.Helper()
	user, err := json.Marshal(map[string]any{"role": "user", "content": "hello"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: "run-" + t.Name()}
	_, err = store.Admit(durableRunCtx(), agentruntime.Admission{
		Key:                key,
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "request-" + t.Name(),
		AssistantMessageID: "assistant-" + t.Name(),
		RequestHash:        "hash-" + t.Name(),
		Snapshot:           snapshot,
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	return key
}

func admitDurableCraftRunAsActor(t *testing.T, store *repository.AgentRunStore, snapshot json.RawMessage, actor string) agentruntime.RunKey {
	t.Helper()
	user, err := json.Marshal(map[string]any{"role": "user", "content": "hello"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: "craft-run-" + t.Name()}
	_, err = store.Admit(durableRunCtx(), agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", ActorUserID: actor,
		RequestID: "craft-request-" + t.Name(), AssistantMessageID: "craft-assistant-" + t.Name(),
		RequestHash: "craft-hash-" + t.Name(), Snapshot: snapshot,
		UserMessage: user, AssistantMessage: assistant, Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	return key
}

type runActorMCPObservation struct {
	CallerUserID  string
	Principal     types.Principal
	ContextUserID string
	VisibleIDs    []string
}

type runActorTaskAccessChecker struct {
	userID     string
	calls      int
	err        error
	scope      craft.Scope
	action     craft.TaskAction
	registered bool
	lookupErr  error
}

func (c *runActorTaskAccessChecker) IsCraftTask(context.Context, uint64, string) (bool, error) {
	return c.registered, c.lookupErr
}

func (c *runActorTaskAccessChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	c.calls++
	c.scope, c.action = scope, action
	if action != craft.TaskWrite {
		return craft.ErrForbidden
	}
	c.userID = scope.UserID
	return c.err
}

type countingDurableRunModelService struct {
	durableRunModelService
	calls int
}

func (s *countingDurableRunModelService) GetChatModel(ctx context.Context, modelID string) (chat.Chat, error) {
	s.calls++
	return s.durableRunModelService.GetChatModel(ctx, modelID)
}

type runActorMCPService struct {
	interfaces.MCPServiceService
	observations []runActorMCPObservation
}

type runActorConnectorFacade struct {
	subject      appconn.OCSubject
	sessionID    string
	toolCallID   string
	prepareCalls int
}

func (f *runActorConnectorFacade) PrepareForTool(_ context.Context, subject appconn.OCSubject, sessionID, toolCallID, _, _ string, _ json.RawMessage) (string, error) {
	f.subject, f.sessionID, f.toolCallID = subject, sessionID, toolCallID
	f.prepareCalls++
	return "action-pending", nil
}

func (*runActorConnectorFacade) StatusForTool(context.Context, appconn.OCSubject, string) (string, error) {
	return appconn.ActionAwaitingApproval, nil
}

func TestDurableCraftActorFlowsThroughAppConnectorToolSubject(t *testing.T) {
	run := agentruntime.Run{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "run-actor-tool"}, SessionID: "s1",
		UserID: "u1", ActorUserID: "collaborator",
	}
	manifest := []craft.Input{}
	snapshot := DurableRunSnapshot{CraftInputManifest: &manifest}
	access := &runActorTaskAccessChecker{registered: true}
	svc := &sessionService{craftTaskAccess: access}
	_, actorID, isCraft, err := svc.durableRunActorContext(durableRunCtx(), run, snapshot)
	require.NoError(t, err)
	require.True(t, isCraft)
	ctx := durableRunActorIdentityContext(durableRunCtx(), run.Key.TenantID, actorID)
	ctx = durableToolExecContext(ctx, run, actorID, event.NewEventBus())
	meta, ok := tools.ToolExecFromContext(ctx)
	require.True(t, ok)
	meta.ToolCallID = "connector-call"
	ctx = tools.WithToolExecContext(ctx, meta)

	facade := &runActorConnectorFacade{}
	tool := tools.NewAppConnectorTool(facade)
	_, err = tool.Execute(ctx, json.RawMessage(`{"connection_id":"private-connection","action_id":"mail.read","input":{"id":"1"}}`))
	var wait *agentruntime.OCActionWaitError
	require.ErrorAs(t, err, &wait, "the real model-facing tool prepares then parks for human approval")
	require.Equal(t, 1, facade.prepareCalls)
	require.Equal(t, appconn.OCSubject{TenantID: 1, ActorID: "collaborator"}, facade.subject)
	require.Equal(t, "s1", facade.sessionID)
	require.Equal(t, "connector-call", facade.toolCallID)
}

func (s *runActorMCPService) ListMCPServices(ctx context.Context, _ uint64) ([]*types.MCPService, error) {
	caller := types.CallerFromContext(ctx)
	principal, _ := types.PrincipalFromContext(ctx)
	ids := []string{}
	var services []*types.MCPService
	switch caller.UserID {
	case "collaborator":
		ids = append(ids, "collaborator-private")
		services = []*types.MCPService{{ID: "collaborator-private", TenantID: 1, Name: "collaborator private", Enabled: true}}
	case "u1":
		ids = append(ids, "owner-private")
		services = []*types.MCPService{{ID: "owner-private", TenantID: 1, Name: "owner private", Enabled: true}}
	}
	s.observations = append(s.observations, runActorMCPObservation{
		CallerUserID: caller.UserID, Principal: principal,
		ContextUserID: func() string { id, _ := types.UserIDFromContext(ctx); return id }(),
		VisibleIDs:    ids,
	})
	return services, nil
}

func TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery(t *testing.T) {
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('collaborator','collaborator','collaborator@example.test','x',1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	_, err := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
	}, 0)
	require.NoError(t, err)
	store := repository.NewAgentRunStore(db)
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MCPSelectionMode: "all", MultiTurnEnabled: true}
	snapshot, err := BuildDurableCraftRunSnapshot("collaborator work", nil, "model-1", "", config, []craft.Input{})
	require.NoError(t, err)
	key := admitDurableCraftRunAsActor(t, store, snapshot, "collaborator")
	run, err := store.Get(durableRunCtx(), key)
	require.NoError(t, err)
	var durableSnapshot DurableRunSnapshot
	require.NoError(t, json.Unmarshal(run.Snapshot, &durableSnapshot))
	require.NotNil(t, durableSnapshot.CraftWorkspaceSeed, "the repository must attach the selected Workspace head")
	require.Equal(t, "u1", run.UserID, "Task storage owner remains durable owner")
	require.Equal(t, "collaborator", run.ActorUserID)

	first, err := store.Claim(durableRunCtx(), key, "worker-1", time.Minute)
	require.NoError(t, err)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Update("lease_until", time.Now().Add(-time.Second)).Error)
	second, err := store.Claim(durableRunCtx(), key, "worker-2", time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, first.Owner, second.Owner)

	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	observer := &runActorMCPService{}
	model := &recordingDurableRunChat{}
	access := &runActorTaskAccessChecker{registered: true}
	svc := &sessionService{
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: model},
		agentService:    &agentService{mcpServiceService: observer, mcpManager: manager},
		craftTaskAccess: access,
	}
	require.NoError(t, svc.ExecuteDurableRun(durableRunCtx(), second))

	require.Len(t, observer.observations, 1)
	seen := observer.observations[0]
	require.Equal(t, "collaborator", seen.CallerUserID)
	require.Equal(t, types.Principal{Type: types.PrincipalWebUser, ID: "collaborator"}, seen.Principal)
	require.Equal(t, "collaborator", seen.ContextUserID)
	require.Equal(t, []string{"collaborator-private"}, seen.VisibleIDs, "Owner's private connector must not be loaded")
	require.NotEqual(t, second.Owner, seen.CallerUserID, "worker lease owner is never an authorization actor")
	require.Equal(t, 1, access.calls)
	require.Equal(t, "collaborator", access.userID, "current TaskWrite is checked as the admitted actor")
	require.Equal(t, craft.TaskWrite, access.action)
	require.Equal(t, craft.Scope{TenantID: 1, UserID: "collaborator", SessionID: "s1"}, access.scope)
	toolCtx := durableToolExecContext(durableRunCtx(), run, "collaborator", event.NewEventBus())
	toolMeta, ok := tools.ToolExecFromContext(toolCtx)
	require.True(t, ok)
	require.Equal(t, "collaborator", toolMeta.UserID, "HITL/OAuth tool metadata carries the durable actor")
	require.Equal(t, run.UserID, "u1", "storage owner remains separate from tool principal")
}

func TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed(t *testing.T) {
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	store := repository.NewAgentRunStore(db)
	snapshot, err := BuildDurableRunSnapshot("legacy Craft run", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: "legacy-seed-" + t.Name()}
	requestID, assistantID := "legacy-seed-request-"+t.Name(), "legacy-seed-assistant-"+t.Name()
	require.NoError(t, db.Table("agent_runs").Create(map[string]any{
		"tenant_id": key.TenantID, "run_id": key.RunID, "session_id": "s1", "owner_id": "u1", "actor_user_id": "u1",
		"request_id": requestID, "assistant_message_id": assistantID, "request_hash": "legacy-seed-hash",
		"driver": "platform", "status": "queued", "snapshot": string(snapshot), "graph_version": "1", "schema_version": 1,
		"deadline": time.Now().Add(time.Hour),
	}).Error)
	require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", key.TenantID, "s1").
		Update("active_agent_run_id", key.RunID).Error)
	fence, err := store.Claim(durableRunCtx(), key, "worker-legacy-seed", time.Minute)
	require.NoError(t, err)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	model := &countingDurableRunModelService{durableRunModelService: durableRunModelService{chat: &recordingDurableRunChat{}}}
	access := &runActorTaskAccessChecker{registered: true}
	svc := &sessionService{
		messageRepo: &durableRunMessageRepo{}, modelService: model,
		agentService: &agentService{mcpManager: manager}, craftTaskAccess: access,
	}

	err = svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Zero(t, model.calls, "a legacy Craft run without a frozen seed fails before model resolution")
}

func TestExecuteDurableCraftRunRechecksTaskWriteBeforeModelResolution(t *testing.T) {
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('collaborator','collaborator','collaborator@example.test','x',1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	// Craft admission derives the repository-owned workspace seed at Admit
	// time (agent_run.go GetWorkspace): the fixture must seed the workspace.
	_, wsErr := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
	}, 0)
	require.NoError(t, wsErr)
	store := repository.NewAgentRunStore(db)
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	snapshot, err := BuildDurableCraftRunSnapshot("revoked task", nil, "model-1", "", config, []craft.Input{})
	require.NoError(t, err)
	key := admitDurableCraftRunAsActor(t, store, snapshot, "collaborator")
	fence, err := store.Claim(durableRunCtx(), key, "worker-retry", time.Minute)
	require.NoError(t, err)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	model := &countingDurableRunModelService{durableRunModelService: durableRunModelService{chat: &recordingDurableRunChat{}}}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	observer := &runActorMCPService{}
	access := &runActorTaskAccessChecker{err: craft.ErrForbidden, registered: true}
	svc := &sessionService{
		messageRepo: &durableRunMessageRepo{}, modelService: model,
		agentService: &agentService{mcpServiceService: observer, mcpManager: manager}, craftTaskAccess: access,
	}
	err = svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, 1, access.calls)
	require.Equal(t, "collaborator", access.userID)
	require.Zero(t, model.calls, "revoked TaskWrite is rejected before model resolution")
	require.Empty(t, observer.observations, "revoked TaskWrite is rejected before capability resolution")
}

func TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(t *testing.T) {
	testExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(t, false)
}

func TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete(t *testing.T) {
	testExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(t, true)
}

func testExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(t *testing.T, deleteSession bool) {
	t.Helper()
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	// Craft admission derives the repository-owned workspace seed at Admit
	// time (agent_run.go GetWorkspace): the fixture must seed the workspace.
	_, wsErr := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
	}, 0)
	require.NoError(t, wsErr)
	store := repository.NewAgentRunStore(db)
	// Admit under the CURRENT contract (marked Craft snapshot + actor), then
	// regress the durable row to the LEGACY shape — snapshot without the
	// Craft manifest and no actor — exactly the way the actor NULL-ing below
	// simulates a pre-contract row. Admission itself must keep refusing the
	// legacy shape; the executor sees only the regressed row.
	legacy, err := BuildDurableRunSnapshot("legacy unmarked craft", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}})
	require.NoError(t, err)
	marked, err := BuildDurableCraftRunSnapshot("legacy unmarked craft", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}, []craft.Input{})
	require.NoError(t, err)
	key := admitDurableCraftRunAsActor(t, store, marked, "u1")
	// A真 pre-contract row carries no admission digest either: the digest
	// columns bind the admitted snapshot, and Claim's digest fence only
	// governs rows the CURRENT contract admitted.
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Updates(map[string]any{"actor_user_id": nil, "snapshot": json.RawMessage(legacy),
			"snapshot_digest": "", "snapshot_digest_version": 0}).Error)
	require.NoError(t, store.AppendInput(durableRunCtx(), key, agentruntime.RunInput{
		SteerID: "after-legacy", Mode: "after", Message: json.RawMessage(`{"role":"user","content":"follow up"}`),
	}))
	fence, err := store.Claim(durableRunCtx(), key, "worker-legacy-unmarked", time.Minute)
	require.NoError(t, err)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	model := &countingDurableRunModelService{durableRunModelService: durableRunModelService{chat: &recordingDurableRunChat{}}}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	observer := &runActorMCPService{}
	access := NewCraftAccessService(db)
	require.NoError(t, access.CheckTaskAccess(durableRunCtx(), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}, craft.TaskWrite), "fixture Owner has current TaskWrite, so falling back to owner would execute")
	if deleteSession {
		require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", 1, "s1").Update("deleted_at", time.Now()).Error,
			"simulate a retained Craft registration with a soft-deleted Session and a still-live Run lease")
	}
	svc := &sessionService{
		messageRepo: &durableRunMessageRepo{}, modelService: model,
		agentService: &agentService{mcpServiceService: observer, mcpManager: manager}, craftTaskAccess: access,
	}
	err = svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.ErrorIs(t, err, craft.ErrForbidden, "Craft kind is determined by durable registration, not manifest version")
	require.Zero(t, model.calls, "historical Craft runs without an actor fail before model resolution")
	require.Empty(t, observer.observations)
	var runs int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, "s1").Count(&runs).Error)
	require.EqualValues(t, 1, runs, "actorless historical Craft run cannot admit a follow-up")
	admitAfterFollowUps(durableRunCtx(), store, key, DurableRunSnapshot{}, true, access)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, "s1").Count(&runs).Error)
	require.EqualValues(t, 1, runs, "follow-up path independently refuses actorless registered Craft run")
}

func TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		db := openDurableRunTestDB(t)
		store := repository.NewAgentRunStore(db)
		key := admitDurableRun(t, store, durableRunSnapshot(t))
		fence, err := store.Claim(durableRunCtx(), key, "worker-lookup-error", time.Minute)
		require.NoError(t, err)
		prev := RegisteredAgentRunService()
		RegisterAgentRunService(NewAgentRunService(store))
		t.Cleanup(func() { RegisterAgentRunService(prev) })
		model := &countingDurableRunModelService{durableRunModelService: durableRunModelService{chat: &recordingDurableRunChat{}}}
		manager := mcp.NewMCPManager(nil)
		t.Cleanup(manager.Shutdown)
		access := &runActorTaskAccessChecker{lookupErr: errors.New("registration query unavailable")}
		svc := &sessionService{messageRepo: &durableRunMessageRepo{}, modelService: model,
			agentService: &agentService{mcpManager: manager}, craftTaskAccess: access}
		err = svc.ExecuteDurableRun(durableRunCtx(), fence)
		require.ErrorIs(t, err, craft.ErrForbidden)
		require.Zero(t, model.calls, "classification failure is not treated as generic")
	})

	t.Run("manifest without registration", func(t *testing.T) {
		db := openDurableRunTestDB(t)
		store := repository.NewAgentRunStore(db)
		config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
		snapshot, err := BuildDurableCraftRunSnapshot("mismatched registration", nil, "model-1", "", config, []craft.Input{})
		require.NoError(t, err)
		require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
		_, wsErr := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
			Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
		}, 0)
		require.NoError(t, wsErr)
		key := admitDurableCraftRunAsActor(t, store, snapshot, "u1")
		fence, err := store.Claim(durableRunCtx(), key, "worker-mismatch", time.Minute)
		require.NoError(t, err)
		prev := RegisteredAgentRunService()
		RegisterAgentRunService(NewAgentRunService(store))
		t.Cleanup(func() { RegisterAgentRunService(prev) })
		model := &countingDurableRunModelService{durableRunModelService: durableRunModelService{chat: &recordingDurableRunChat{}}}
		manager := mcp.NewMCPManager(nil)
		t.Cleanup(manager.Shutdown)
		access := &runActorTaskAccessChecker{registered: false}
		svc := &sessionService{messageRepo: &durableRunMessageRepo{}, modelService: model,
			agentService: &agentService{mcpManager: manager}, craftTaskAccess: access}
		err = svc.ExecuteDurableRun(durableRunCtx(), fence)
		require.ErrorIs(t, err, craft.ErrForbidden)
		require.Zero(t, model.calls, "stale/new Craft snapshot cannot mask a missing registered Task")
	})
}

func TestExecuteDurableGenericLegacyRunRetainsOwnerFallback(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	key := admitDurableRun(t, store, durableRunSnapshot(t))
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Update("actor_user_id", nil).Error)
	fence, err := store.Claim(durableRunCtx(), key, "worker-generic", time.Minute)
	require.NoError(t, err)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	model := &recordingDurableRunChat{}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	access := &runActorTaskAccessChecker{registered: false}
	svc := &sessionService{messageRepo: &durableRunMessageRepo{}, modelService: &durableRunModelService{chat: model},
		agentService: &agentService{mcpManager: manager}, craftTaskAccess: access}
	require.NoError(t, svc.ExecuteDurableRun(durableRunCtx(), fence))
	require.NotEmpty(t, model.messages, "ordinary legacy sessions retain their previous identity behavior")
}

func TestExecuteDurableCraftRunRejectsLegacyMissingActorBeforeCapabilities(t *testing.T) {
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	// Craft admission derives the repository-owned workspace seed at Admit
	// time (agent_run.go GetWorkspace): the fixture must seed the workspace.
	_, wsErr := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
	}, 0)
	require.NoError(t, wsErr)
	store := repository.NewAgentRunStore(db)
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	snapshot, err := BuildDurableCraftRunSnapshot("legacy actor", nil, "model-1", "", config, []craft.Input{})
	require.NoError(t, err)
	key := admitDurableCraftRunAsActor(t, store, snapshot, "u1")
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Update("actor_user_id", nil).Error)
	fence, err := store.Claim(durableRunCtx(), key, "worker-legacy", time.Minute)
	require.NoError(t, err)

	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	model := &recordingDurableRunChat{}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	observer := &runActorMCPService{}
	svc := &sessionService{
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: model},
		agentService:    &agentService{mcpServiceService: observer, mcpManager: manager},
		craftTaskAccess: &runActorTaskAccessChecker{registered: true},
	}
	err = svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, model.messages, "legacy actorless Craft Run fails before model access")
	require.Empty(t, observer.observations, "legacy actorless Craft Run fails before capability resolution")
}

func durableRunSnapshot(t *testing.T) json.RawMessage {
	t.Helper()
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	raw, err := BuildDurableRunSnapshot("hello", nil, "model-1", "", config)
	require.NoError(t, err)
	return raw
}

func TestDurableRunSnapshotRoundTripKeepsRuntimeFields(t *testing.T) {
	config := &types.AgentConfig{
		AllowedTools:        []string{tools.ToolThinking},
		SandboxConfigID:     "ws-1",
		VLMModelID:          "vlm-1",
		PinnedMCPServiceIDs: []string{"svc-1"},
		PinnedSkillNames:    []string{"skill-1"},
		SharedAgentReadOnly: true,
	}
	raw, err := BuildDurableRunSnapshot("query", []string{"img-1"}, "model-1", "rerank-1", config)
	require.NoError(t, err)

	parsed, err := ParseDurableRunSnapshot(raw)
	require.NoError(t, err)
	require.Equal(t, "query", parsed.Query)
	require.Equal(t, "model-1", parsed.ModelID)
	require.Equal(t, "rerank-1", parsed.RerankModelID)
	require.Equal(t, []string{"img-1"}, parsed.ImageURLs)

	restored, err := parsed.RestoreAgentConfig()
	require.NoError(t, err)
	require.Equal(t, "ws-1", restored.SandboxConfigID)
	require.Equal(t, "vlm-1", restored.VLMModelID)
	require.Equal(t, []string{"svc-1"}, restored.PinnedMCPServiceIDs)
	require.Equal(t, []string{"skill-1"}, restored.PinnedSkillNames)
	require.True(t, restored.SharedAgentReadOnly)
	require.Equal(t, []string{tools.ToolThinking}, restored.AllowedTools)
}

func TestDurableCraftRunSnapshotKeepsManifestOutOfModelMessage(t *testing.T) {
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	inputs := []craft.Input{{Ref: "resource://selected-secret-ref", Name: "selected.json",
		SHA256: strings.Repeat("a", 64), Bytes: 12}}
	raw, err := BuildDurableCraftRunSnapshot("visible query", nil, "model-1", "", config, inputs)
	require.NoError(t, err)
	var encoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &encoded))
	require.Contains(t, encoded, "craft_input_manifest")
	require.NotContains(t, string(encoded["query"]), "selected-secret-ref")

	parsed, err := ParseDurableRunSnapshot(raw)
	require.NoError(t, err)
	require.NotNil(t, parsed.CraftInputManifest)
	require.Equal(t, inputs, *parsed.CraftInputManifest)
	message, err := durableUserMessage(parsed)
	require.NoError(t, err)
	require.Equal(t, "visible query", message.Content)
	require.NotContains(t, message.Content, "selected-secret-ref")

	legacy, err := BuildDurableRunSnapshot("legacy query", nil, "model-1", "", config)
	require.NoError(t, err)
	legacyParsed, err := ParseDurableRunSnapshot(legacy)
	require.NoError(t, err, "version-1 ordinary snapshots without Craft metadata remain compatible")
	require.Nil(t, legacyParsed.CraftInputManifest)
}

func TestParseDurableRunSnapshotRejectsUnknownVersion(t *testing.T) {
	raw := json.RawMessage(`{"version":99,"query":"q","model_id":"m","agent_config":{},"runtime":{}}`)
	_, err := ParseDurableRunSnapshot(raw)
	require.Error(t, err)
}

func TestExecuteDurableRunCompletesFreshRun(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	fence, err := store.Claim(durableRunCtx(), key, "worker-1", time.Minute)
	require.NoError(t, err)

	svc := newDurableRunSessionService(t, db)
	err = svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.NoError(t, err)

	run, err := store.Get(durableRunCtx(), key)
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)

	var content string
	require.NoError(t, db.Raw("SELECT content FROM messages WHERE id = ?", "assistant-"+t.Name()).Scan(&content).Error)
	require.Equal(t, "ok", content)

	events, err := store.ReadEvents(durableRunCtx(), key, 0, 10)
	require.NoError(t, err)
	typesSeen := map[string]bool{}
	for _, evt := range events {
		typesSeen[evt.Type] = true
	}
	require.True(t, typesSeen["run_started"], "events: %v", events)
	require.True(t, typesSeen["run_completed"], "events: %v", events)

	// The session active slot is released by the finalize transaction.
	var active *string
	require.NoError(t, db.Raw("SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&active).Error)
	require.Nil(t, active)
}

func TestExecuteDurableRunPreservesImageInputThroughProductionGraph(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	snapshot, err := BuildDurableRunSnapshot("describe image", []string{"artifact://image-1"}, "model-1", "", config)
	require.NoError(t, err)
	key := admitDurableRun(t, store, snapshot)
	fence, err := store.Claim(durableRunCtx(), key, "worker-1", time.Minute)
	require.NoError(t, err)

	model := &recordingDurableRunChat{}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	svc := &sessionService{
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: model},
		agentService:    &agentService{mcpManager: manager},
		craftTaskAccess: &runActorTaskAccessChecker{},
	}
	require.NoError(t, svc.ExecuteDurableRun(durableRunCtx(), fence))

	require.Len(t, model.messages, 1)
	require.Len(t, model.messages[0].MultiContent, 2)
	require.Equal(t, "image_url", model.messages[0].MultiContent[1].Type)
	require.Equal(t, "artifact://image-1", model.messages[0].MultiContent[1].ImageURL.URL)
}

func TestExecuteDurableRunExecutesMCPDiscoveryAndCallThroughProductionGraph(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	server := sdkserver.NewMCPServer("Orders", "2", sdkserver.WithToolCapabilities(false))
	var remoteCalls atomic.Int32
	server.AddTool(
		sdkmcp.Tool{
			Name: "get_order", Description: "Fetch one order",
			RawInputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		},
		func(_ context.Context, req sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			remoteCalls.Add(1)
			var args struct {
				ID string `json:"id"`
			}
			if err := req.BindArguments(&args); err != nil {
				return nil, err
			}
			return sdkmcp.NewToolResultText(args.ID), nil
		},
	)
	upstream := httptest.NewServer(sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true)))
	t.Cleanup(upstream.Close)

	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	now := time.Now()
	mcpService := &agentCapabilitiesMCPService{
		service: &types.MCPService{
			ID: "orders", TenantID: 1, Name: "Orders", Enabled: true,
			UpdatedAt: now, URL: &upstream.URL,
			TransportType: types.MCPTransportHTTPStreamable,
		},
		metadata: &types.MCPMetadata{
			ServiceID: "orders", Tools: []*types.MCPTool{{
				Name: "get_order", Description: "Fetch one order",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
			}},
		},
	}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	model := &scriptedMCPDurableRunChat{}
	svc := &sessionService{
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: model},
		agentService:    &agentService{mcpServiceService: mcpService, mcpManager: manager},
		craftTaskAccess: &runActorTaskAccessChecker{},
	}
	config := &types.AgentConfig{
		AllowedTools: []string{tools.ToolThinking}, MCPSelectionMode: "all", MultiTurnEnabled: true,
	}
	snapshot, err := BuildDurableRunSnapshot("fetch order", nil, "model-1", "", config)
	require.NoError(t, err)
	key := admitDurableRun(t, store, snapshot)
	fence, err := store.Claim(durableRunCtx(), key, "worker-1", time.Minute)
	require.NoError(t, err)

	require.NoError(t, svc.ExecuteDurableRun(durableRunCtx(), fence))
	run, err := store.Get(durableRunCtx(), key)
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)
	var content string
	require.NoError(t, db.Raw("SELECT content FROM messages WHERE id = ?", "assistant-"+t.Name()).Scan(&content).Error)
	require.Equal(t, "order-42", content)
	mcpCallCount := 0
	require.NoError(t, db.Raw("SELECT count(*) FROM agent_tool_calls WHERE run_id = ? AND tool_name = ?", key.RunID, tools.ToolCallMCPTool).Scan(&mcpCallCount).Error)
	require.Equal(t, 1, mcpCallCount)
	require.Equal(t, int32(1), remoteCalls.Load())
	model.mu.Lock()
	require.GreaterOrEqual(t, model.calls, 3)
	model.mu.Unlock()
}

func TestExecuteDurableRunRejectsSupersededFence(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	first, err := store.Claim(durableRunCtx(), key, "worker-1", time.Millisecond)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	second, err := store.Claim(durableRunCtx(), key, "worker-2", time.Minute)
	require.NoError(t, err)
	require.Greater(t, second.Epoch, first.Epoch)

	svc := newDurableRunSessionService(t, db)
	err = svc.ExecuteDurableRun(durableRunCtx(), first)
	require.Error(t, err)
	require.True(t, errors.Is(err, agentruntime.ErrLeaseLost), "want ErrLeaseLost, got %v", err)
}

// WB-GRAPH: a workbench admission that froze the graph execution core
// (ModelID non-empty) passes the D8 fence naturally — no executor semantics
// change, the run executes through the production graph with a fake model and
// produces events and output without any Craft workspace seed requirement.
func TestExecuteDurableRunWorkbenchAdmissionWithGraphCore(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true}
	graphCore, err := BuildDurableRunSnapshot("整理本周周报", nil, "model-1", "", config)
	require.NoError(t, err)
	var combined map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(graphCore, &combined))
	admissionKeys := map[string]string{
		"session_id": "s1", "agent_id": "builtin-quick-answer", "target_id": "platform",
		"request_id": "req-wb-graph", "text": "整理本周周报",
	}
	for key, value := range admissionKeys {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		combined[key] = encoded
	}
	snapshot, err := json.Marshal(combined)
	require.NoError(t, err)

	key := admitDurableRun(t, store, snapshot)
	fence, err := store.Claim(durableRunCtx(), key, "worker-wb-graph", time.Minute)
	require.NoError(t, err)

	model := &recordingDurableRunChat{}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	access := &runActorTaskAccessChecker{registered: false}
	svc := &sessionService{
		messageRepo:     &durableRunMessageRepo{},
		modelService:    &durableRunModelService{chat: model},
		agentService:    &agentService{mcpManager: manager},
		craftTaskAccess: access,
	}
	require.NoError(t, svc.ExecuteDurableRun(durableRunCtx(), fence), "the D8 fence passes because the model identity is frozen")

	run, err := store.Get(durableRunCtx(), key)
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)
	require.NotEmpty(t, model.messages, "the frozen graph core drives a real model turn")
	events, err := store.ReadEvents(durableRunCtx(), key, 0, 10)
	require.NoError(t, err)
	typesSeen := map[string]bool{}
	for _, evt := range events {
		typesSeen[evt.Type] = true
	}
	require.True(t, typesSeen["run_started"], "events: %v", events)
	require.True(t, typesSeen["run_completed"], "events: %v", events)
	require.Zero(t, access.calls, "a non-Craft workbench session never consults the Craft task gate")
}

func TestWorkerParksWaitClassFailureDurable(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	waitErr := fmt.Errorf("wrapped: %w", agentruntime.ErrToolWaitUser)
	waitExecutor := func(context.Context, agentruntime.Fence) error { return waitErr }
	worker, err := NewAgentRunWorker(store, waitExecutor, WorkerConfig{
		Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second, ScanInterval: time.Second, MaxWorkers: 2,
	})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(durableRunCtx()))
	deadline := time.Now().Add(5 * time.Second)
	for {
		run, getErr := store.Get(durableRunCtx(), key)
		require.NoError(t, getErr)
		if run.Status == "waiting_user" {
			require.Equal(t, "tool_outcome_unknown", run.WaitReason)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not park at waiting_user, status=%s", run.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
