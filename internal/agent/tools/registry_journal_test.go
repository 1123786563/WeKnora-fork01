package tools

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/event"
	internalmcp "github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func registryJournalDB(t *testing.T) (*gorm.DB, *repository.AgentRunStore, agentruntime.Fence) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "journal.db")+"?_foreign_keys=on"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id TEXT, tenant_id INTEGER, user_id TEXT, deleted_at DATETIME);
		INSERT INTO sessions VALUES ('s1',1,'u1',NULL);`).Error)
	migration, err := os.ReadFile("../../../migrations/sqlite/000014_agent_runs.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET engine_type='trpc', active_agent_run_id='r1';`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id,run_id,session_id,owner_id,request_id,assistant_message_id,request_hash,snapshot,deadline)
		VALUES (1,'r1','s1','u1','q1','a1','hash','{}',?)`, time.Now().Add(time.Hour)).Error)
	store := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	fence, err := store.Claim(context.Background(), key, "worker", time.Minute)
	require.NoError(t, err)
	return db, store, fence
}

type journalApprovalGate struct {
	wait func() (approval.Decision, error)
}

func (*journalApprovalGate) IsEnabled(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}
func (*journalApprovalGate) NeedsApproval(context.Context, uint64, string, string) bool { return true }
func (g *journalApprovalGate) RequestAndWait(context.Context, approval.PendingRequest) (approval.Decision, error) {
	return g.wait()
}

func TestToolJournalApprovalWaitRemainsPlanned(t *testing.T) {
	db, store, fence := registryJournalDB(t)
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)))
	defer cancel()
	gate := &journalApprovalGate{wait: func() (approval.Decision, error) {
		var status string
		require.NoError(t, db.Table("agent_tool_calls").Select("status").Scan(&status).Error)
		require.Equal(t, "planned", status, "approval wait must not imply an external dispatch")
		var attempts int64
		require.NoError(t, db.Table("agent_tool_attempts").Count(&attempts).Error)
		require.Zero(t, attempts)
		cancel()
		return approval.Decision{}, context.Canceled
	}}
	tool := NewMCPTool(&types.MCPService{ID: "svc", Name: "writer", TenantID: 1},
		&types.MCPTool{Name: "write", InputSchema: json.RawMessage(`{"type":"object"}`)}, nil, gate, 0)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	ctx = WithToolExecContext(ctx, &ToolExecContext{EventBus: event.NewEventBus(), ApprovalCtx: ctx})
	plan := agentruntime.ToolPlan{
		CallID: "c1", Name: tool.Name(), Identity: "mcp/svc/write@1",
		ArgsHash: "hash", Args: json.RawMessage(`{}`),
	}
	_, err := agentruntime.NewToolExecutor(store, store, registry.ExecuteTool).Execute(ctx, fence, plan)
	require.ErrorIs(t, err, context.Canceled)
	record, err := store.EnsureToolPlan(context.Background(), fence, plan)
	require.NoError(t, err)
	require.Equal(t, "planned", record.Status)
}

func TestToolJournalSchemaRejectionDoesNotDispatch(t *testing.T) {
	db, store, fence := registryJournalDB(t)
	registry := NewToolRegistry()
	registry.RegisterTool(&mockTool{name: "validated", parameters: json.RawMessage(
		`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`,
	)})
	plan := agentruntime.ToolPlan{
		CallID: "c1", Name: "validated", Identity: "builtin/validated@1",
		ArgsHash: "hash", Args: json.RawMessage(`{}`),
	}
	executor := agentruntime.NewToolExecutor(store, store, registry.ExecuteTool)
	result, err := executor.Execute(context.Background(), fence, plan)
	require.NoError(t, err)
	require.False(t, result.Result.Success)
	var attempts int64
	require.NoError(t, db.Table("agent_tool_attempts").Count(&attempts).Error)
	require.Zero(t, attempts, "schema failure is not an external dispatch")
	err = store.ValidateCheckpointCalls(context.Background(), fence.RunKey, nil, map[string]bool{"c1": true})
	require.NoError(t, err)
}

func TestToolJournalCannotBypassRequiredApprovalWithoutContext(t *testing.T) {
	db, store, fence := registryJournalDB(t)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	tool := NewMCPTool(&types.MCPService{ID: "svc", Name: "writer", TenantID: 1},
		&types.MCPTool{Name: "write", InputSchema: json.RawMessage(`{"type":"object"}`)},
		nil, &journalApprovalGate{}, 0)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	plan := agentruntime.ToolPlan{
		CallID: "c1", Name: tool.Name(), Identity: "mcp/svc/write@1",
		ArgsHash: "hash", Args: json.RawMessage(`{}`),
	}
	var err error
	require.NotPanics(t, func() {
		_, err = agentruntime.NewToolExecutor(store, store, registry.ExecuteTool).Execute(ctx, fence, plan)
	})
	require.ErrorContains(t, err, "approval context")
	var attempts int64
	require.NoError(t, db.Table("agent_tool_attempts").Count(&attempts).Error)
	require.Zero(t, attempts)
}

func TestToolJournalMCPApprovalThenDispatchAndReplay(t *testing.T) {
	db, store, fence := registryJournalDB(t)
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	server := sdkserver.NewMCPServer("journal", "1", sdkserver.WithToolCapabilities(false))
	var calls atomic.Int32
	server.AddTool(sdkmcp.NewTool("write"), func(
		context.Context, sdkmcp.CallToolRequest,
	) (*sdkmcp.CallToolResult, error) {
		calls.Add(1)
		var status string
		if err := db.Table("agent_tool_calls").Select("status").Scan(&status).Error; err != nil {
			return nil, err
		}
		if status != "dispatching" {
			return sdkmcp.NewToolResultError("journal missing before dispatch"), nil
		}
		return sdkmcp.NewToolResultText("saved"), nil
	})
	httpServer := httptest.NewServer(sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true)))
	defer httpServer.Close()
	manager := internalmcp.NewMCPManager(nil)
	defer manager.Shutdown()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	gate := &journalApprovalGate{wait: func() (approval.Decision, error) {
		var status string
		require.NoError(t, db.Table("agent_tool_calls").Select("status").Scan(&status).Error)
		require.Equal(t, "planned", status)
		return approval.Decision{Approved: true}, nil
	}}
	tool := NewMCPTool(&types.MCPService{
		ID: "svc", Name: "writer", TenantID: 1, Enabled: true,
		URL: &httpServer.URL, TransportType: types.MCPTransportHTTPStreamable,
	},
		&types.MCPTool{Name: "write", InputSchema: json.RawMessage(`{"type":"object"}`)}, manager, gate, 0)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	ctx = WithToolExecContext(ctx, &ToolExecContext{EventBus: event.NewEventBus(), ApprovalCtx: ctx})
	plan := agentruntime.ToolPlan{
		CallID: "c1", Name: tool.Name(), Identity: "mcp/svc/write@1",
		ArgsHash: "hash", Args: json.RawMessage(`{}`),
	}
	executor := agentruntime.NewToolExecutor(store, store, registry.ExecuteTool)
	first, err := executor.Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.True(t, first.Result.Success, first.Result.Error)
	replayed, err := executor.Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, first.Result.Output, replayed.Result.Output)
	require.EqualValues(t, 1, calls.Load())
}
