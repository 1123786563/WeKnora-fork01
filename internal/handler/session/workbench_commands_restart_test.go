package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/workbench/service/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// T07 服务端集成证据：全量迁移 sqlite + 真实准入协调器 + 真实命令服务 + 真实 handler。
// stop→确认事实→restart 全链经 HTTP 字节验证（AC3：不以单测/mock 冒充集成证据）。
func newCommandRestartRouter(t *testing.T) (*gin.Engine, *repository.AgentRunStore, *gorm.DB) {
	t.Helper()
	db := openWorkbenchHTTPDB(t)
	runs := repository.NewAgentRunStore(db)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, runs, &integrationBudget{}, nil)
	svc := workbenchservice.NewInteractionServiceWithRestart(
		workbenchservice.NewGormInteractionStore(db),
		nil, // steer 不在本链路；缺端口时 steer 命令 fail closed 501
		workbenchservice.NewGormCancelPort(runs),
		nil,
		workbenchservice.NewGormRunRestartPort(db, coordinator),
	)
	commandHandler := NewWorkbenchCommandHandler(svc)
	startHandler := NewWorkbenchStartHandler(coordinator)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1", withIdentity(1, "u1"))
	v1.POST("/workbench/executions", startHandler.Start)
	v1.POST("/workbench/executions/:run_id/commands", commandHandler.Command)
	return r, runs, db
}

func postCommand(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWorkbenchStopThenRestartHTTPIntegration(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)

	// 1. 创建 Task 的首个 Run（真实准入）。
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r1","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID
	require.NotEmpty(t, runID)

	// 2. queue_next 在活动 Run 上是确定性冲突（单写者），零准入。
	active := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"too early","expected_revision":0}`)
	require.Equal(t, http.StatusConflict, active.Code)

	// 3. 停止：202 + ack 绑定本 Run。
	stop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, stop.Code)
	require.Contains(t, stop.Body.String(), `"action":"cancel"`)
	require.Contains(t, stop.Body.String(), runID)

	// 4. 重复停止：duplicate stop on a canceled run is an idempotent replay
	// (Spec: durable idempotency, reconciliation not rejection; same semantics
	// as CancelRun / Task 3)。
	replayStop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, replayStop.Code)

	// 5. 停止后重启：queue_next 带真实（终态后）revision → 202 + next_run_id。
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, runID).Scan(&revision).Error)
	revisionText := strconv.FormatInt(revision, 10)
	restart := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"restart with this","expected_revision":`+revisionText+`,"external_pending_id":"t37-q1"}`)
	require.Equal(t, http.StatusAccepted, restart.Code)
	var restartAck struct {
		Data struct {
			RunID     string `json:"run_id"`
			Action    string `json:"action"`
			NextRunID string `json:"next_run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(restart.Body.Bytes(), &restartAck))
	require.Equal(t, "queue_next", restartAck.Data.Action)
	require.Equal(t, runID, restartAck.Data.RunID)
	require.NotEmpty(t, restartAck.Data.NextRunID)
	require.NotEqual(t, runID, restartAck.Data.NextRunID)

	// 6. 幂等重放（网络重试语义）：同 idempotency id → 同一个 next_run_id。
	replay := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"restart with this","expected_revision":`+revisionText+`,"external_pending_id":"t37-q1"}`)
	require.Equal(t, http.StatusAccepted, replay.Code)
	var replayAck struct {
		Data struct {
			NextRunID string `json:"next_run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &replayAck))
	require.Equal(t, restartAck.Data.NextRunID, replayAck.Data.NextRunID)
}

// 审查修复轮 1：AC3 的 revision-CAS 拒绝路径需要 HTTP 层直接证据（brief Produces
// 节承诺 queue_next 语义含 revision 409）。与裁决 A 不冲突：cancel 的重复停止是
// 幂等重放（202），而 queue_next 的 revision 栅栏（command_queue_next.go:88-91）
// 无幂等短路——过期视图不得在未观测的事实之上准入重启。
func TestWorkbenchQueueNextStaleRevisionIsAConflict(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r4","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID

	stop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, stop.Code)

	// 实读终态 revision，再以过期视图（revision-1）+ 新 pending id 请求重启：
	// 新 pending id 排除幂等命中解释，409 只能来自 revision 栅栏本身。
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, runID).Scan(&revision).Error)
	require.Greater(t, revision, int64(0), "cancel must advance the revision before this fence is exercisable")
	stale := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"restart from a stale view","expected_revision":`+strconv.FormatInt(revision-1, 10)+`,"external_pending_id":"t37-q-stale"}`)
	require.Equal(t, http.StatusConflict, stale.Code)

	// 拒绝必须零准入：会话内除父 Run 外不得出现任何后续 Run。
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "a stale-revision restart must not admit anything")
}

func TestWorkbenchCancelHTTPWritesEventReleasesSlot(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r2","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID

	stop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, stop.Code)

	var events int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE run_id = ? AND event_type = 'cancellation_requested'`, runID).Scan(&events).Error)
	require.EqualValues(t, 1, events, "the stop request must be a durable timeline fact")
	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot)
}

func TestWorkbenchCommandHTTPForeignOwnerIsIsolated(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r3","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID

	foreign := gin.New()
	// 同一 handler、不同身份（u2）：跨 owner 命令统一 404。
	runs := repository.NewAgentRunStore(db)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, runs, &integrationBudget{}, nil)
	svc := workbenchservice.NewInteractionServiceWithRestart(workbenchservice.NewGormInteractionStore(db), nil, workbenchservice.NewGormCancelPort(runs), nil, workbenchservice.NewGormRunRestartPort(db, coordinator))
	fh := NewWorkbenchCommandHandler(svc)
	foreign.POST("/api/v1/workbench/executions/:run_id/commands", withIdentity(1, "u2"), fh.Command)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/"+runID+"/commands", strings.NewReader(`{"action":"cancel","expected_revision":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	foreign.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE run_id = ?`, runID).Scan(&status).Error)
	require.Equal(t, "queued", status, "a foreign cancel must not mutate the run")
}
