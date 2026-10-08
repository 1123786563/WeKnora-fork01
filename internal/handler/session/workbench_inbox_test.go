package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	workbenchservice "github.com/Tencent/WeKnora/internal/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// agent_runs 只读投影（ListPending 的 LEFT JOIN 需要；本包私有，不越包引用 service 测试行）。
type inboxAgentRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	UpdatedAt time.Time
}

func (inboxAgentRunRow) TableName() string { return "agent_runs" }

func inboxRouter(h *WorkbenchCommandHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/workbench/interactions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "u1")
		c.Set(types.PrincipalContextKey.String(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
		h.ListInboxInteractions(c)
	})
	r.POST("/api/v1/workbench/executions/interactions/:id/decisions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "u1")
		c.Set(types.PrincipalContextKey.String(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
		h.DecideInteraction(c)
	})
	return r
}

func newInboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "inbox-http.db") + "?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(2)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&workbenchserviceInteractionRow{}, &inboxAgentRunRow{}))
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (tenant_id INTEGER, id TEXT, title TEXT, archived_at DATETIME)`).Error)
	return db
}

// AC3（服务端最高稳定 Interface）：真实路由 + 真实 GormInteractionStore + 真实 Service。
func TestWorkbenchInboxListsPendingAcrossRunsAndScopesOwner(t *testing.T) {
	db := newInboxTestDB(t)
	require.NoError(t, db.Create([]*workbenchserviceInteractionRow{
		{TenantID: 7, ID: "i-1", RunID: "run-1", OwnerID: "web_user:u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", ExpectedRevision: 3},
		{TenantID: 7, ID: "i-2", RunID: "run-2", OwnerID: "web_user:u1", Kind: "recovery", ArgsHash: "h2", Status: "pending", ExpectedRevision: 0},
		{TenantID: 7, ID: "i-3", RunID: "run-1", OwnerID: "web_user:u2", Kind: "budget", ArgsHash: "h3", Status: "pending", ExpectedRevision: 0},
	}).Error)
	h := NewWorkbenchCommandHandler(workbenchservice.NewInteractionService(workbenchservice.NewGormInteractionStore(db), nil, nil))
	r := inboxRouter(h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/interactions?limit=10", nil))
	require.Equal(t, http.StatusOK, w.Code)
	// data 是数组：逐行按 map 断言（避免双层泛型结构体）
	var raw struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.True(t, raw.Success)
	require.Len(t, raw.Data, 2, "owner 隔离：u2 的行不进 u1 的收件箱")
	require.Equal(t, "i-1", raw.Data[0]["id"])
	require.Equal(t, "run-1", raw.Data[0]["run_id"])
	require.Equal(t, "tool_approval", raw.Data[0]["kind"])
	require.Equal(t, "h1", raw.Data[0]["args_hash"])
	require.Equal(t, float64(3), raw.Data[0]["expected_revision"])
	require.NotEmpty(t, raw.Data[0]["created_at"], "收件箱行携带 created_at（Task 1 输出字段贯通到 wire）")
}

// AC1（多设备并发只生效一次）+ 幂等重放：真实 HTTP 并发 POST。
func TestWorkbenchDecisionHTTPConcurrentDevicesDecideExactlyOnce(t *testing.T) {
	db := newInboxTestDB(t)
	// decision_id/action must be explicit ''（与生产 CreatePending 的 GORM Create
	// 一致）：AutoMigrate 建表无 DEFAULT ''，裸 INSERT 省略这两列会得到 NULL，而
	// SQLite 中 `decision_id = ''` 不匹配 NULL，会让两个并发 CAS 都落 0 行。
	require.NoError(t, db.Exec(`INSERT INTO workbench_interactions (tenant_id,id,run_id,owner_id,kind,args_hash,status,expected_revision,decision_id,action,created_at,updated_at) VALUES (7,'i-c','run-c','web_user:u1','budget','hash','pending',0,'','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	service := workbenchservice.NewInteractionService(workbenchservice.NewGormInteractionStore(db), nil, nil)
	h := NewWorkbenchCommandHandler(service)
	r := inboxRouter(h)
	post := func(decisionID string) *httptest.ResponseRecorder {
		body := `{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"` + decisionID + `","expected_revision":0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/interactions/i-c/decisions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- post(fmt.Sprintf("d%d", i)).Code
		}(i)
	}
	wg.Wait()
	close(results)
	var ok, conflict int
	for code := range results {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("unexpected status %d", code)
		}
	}
	require.Equal(t, 1, ok, "两台设备并发决定：恰一成功")
	require.Equal(t, 1, conflict, "另一台必须得到 409 冲突而非第二个成功")
	var stored struct {
		DecisionID       string
		Action           string
		Status           string
		ExpectedRevision int64
	}
	require.NoError(t, db.Table("workbench_interactions").Select("decision_id, action, status, expected_revision").Where("id = ?", "i-c").Scan(&stored).Error)
	require.Equal(t, "resolved", stored.Status)
	require.Equal(t, int64(1), stored.ExpectedRevision, "恰一次 revision 前进")
	require.Contains(t, []string{"d0", "d1"}, stored.DecisionID)
	// 落败设备以胜者的 decision_id 重放（幂等重试语义）：200 且不再前进 revision。
	require.Equal(t, http.StatusOK, post(stored.DecisionID).Code)
	require.NoError(t, db.Table("workbench_interactions").Select("expected_revision").Where("id = ?", "i-c").Scan(&stored.ExpectedRevision).Error)
	require.Equal(t, int64(1), stored.ExpectedRevision, "重放不二次生效")
	// 决定后收件箱不再列出该行。
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/interactions", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "i-c")
}

type inboxRemoteInteraction struct {
	failures int
	calls    int
}

func (p *inboxRemoteInteraction) SubmitInteraction(context.Context, uint64, string, string, string, string, string, string, int64, int64) error {
	p.calls++
	if p.calls <= p.failures {
		return errors.New("provider unavailable")
	}
	return nil
}

// AC2（审批成功不被误显示成外部派发完成）：远程派发失败时决定保持落地，
// wire 如实返回 502 + code=command_recovery_unknown（绝非成功 envelope）；同 decision_id 重放走服务端重试。
func TestWorkbenchDecisionRemoteFailureIsDeliveryUnknownNotSuccess(t *testing.T) {
	db := newInboxTestDB(t)
	store := workbenchservice.NewGormInteractionStore(db)
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 3}}, approvalHTTPChecker{}, nil)
	service := workbenchservice.NewInteractionServiceWithApproval(store, nil, nil, gate)
	remote := &inboxRemoteInteraction{failures: 1}
	service.SetRemoteInteractionPort(remote)
	h := NewWorkbenchCommandHandler(service)
	r := inboxRouter(h)
	bus := event.NewEventBus()
	pending := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pending <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return nil
	})
	approvalCtx := types.WithPrincipal(context.Background(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
	resolved := make(chan approval.Decision, 1)
	go func() {
		decision, err := gate.RequestAndWait(approvalCtx, approval.PendingRequest{TenantID: 7, CredentialVersion: 1, UserID: "u1", RunID: "run-a", RequestID: "request-a", EventBus: bus, Args: []byte(`{"x":1}`)})
		require.NoError(t, err)
		resolved <- decision
	}()
	id := <-pending
	var row struct{ ArgsHash string }
	require.NoError(t, db.Table("workbench_interactions").Select("args_hash").Where("id = ?", id).Scan(&row).Error)
	post := func() *httptest.ResponseRecorder {
		body := `{"kind":"tool_approval","action":"approve","args_hash":"` + row.ArgsHash + `","decision_id":"d-stable","expected_revision":0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/interactions/"+id+"/decisions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	// 第一次：remote 派发失败 → 502 + 机器可读 code；决定在库中已 resolved。
	first := post()
	require.Equal(t, http.StatusBadGateway, first.Code)
	var body struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Error   string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &body))
	require.False(t, body.Success, "绝不冒充外部派发成功")
	require.Equal(t, "command_recovery_unknown", body.Code)
	require.NotEmpty(t, body.Error)
	var status string
	require.NoError(t, db.Table("workbench_interactions").Select("status").Where("id = ?", id).Scan(&status).Error)
	require.Equal(t, "resolved", status, "决定保持落地：delivery-unknown 不回滚 CAS")
	// 第二次：同一 decision_id 重放 → 服务端重试 remote（本 stub 已恢复）→ 200。
	require.Equal(t, 1, remote.calls)
	second := post()
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, 2, remote.calls, "重放必须重发同一 durable provider 身份")
	require.True(t, (<-resolved).Approved)
}
