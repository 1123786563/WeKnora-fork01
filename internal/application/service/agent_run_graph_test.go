package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDurableRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "durable-runs.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id)"+
			" VALUES ('u1','u1','u1@example.test','x',1)").Error)
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

func newDurableRunSessionService(t *testing.T, _ *gorm.DB) *sessionService {
	t.Helper()
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	return &sessionService{
		cfg:           nil,
		messageRepo:   &durableRunMessageRepo{},
		modelService:  &durableRunModelService{chat: &fakeAgentChatModel{}},
		agentService:  &agentService{mcpManager: manager},
		memoryService: nil,
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
		messageRepo:  &durableRunMessageRepo{},
		modelService: &durableRunModelService{chat: model},
		agentService: &agentService{mcpManager: manager},
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
		messageRepo:  &durableRunMessageRepo{},
		modelService: &durableRunModelService{chat: model},
		agentService: &agentService{mcpServiceService: mcpService, mcpManager: manager},
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
