package session

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type runStoreFake struct {
	run    agentruntime.Run
	events []agentruntime.RunEvent
	got    int
}

func (f *runStoreFake) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return f.run, nil
}
func (f *runStoreFake) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return agentruntime.Fence{}, nil
}
func (f *runStoreFake) Renew(context.Context, agentruntime.Fence, time.Duration) error { return nil }
func (f *runStoreFake) Scan(context.Context, int) ([]agentruntime.RunKey, error)       { return nil, nil }
func (f *runStoreFake) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}
func (f *runStoreFake) SetStatus(context.Context, agentruntime.Fence, string, string) error {
	return nil
}
func (f *runStoreFake) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{State: json.RawMessage(`{}`)}, nil
}
func (f *runStoreFake) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	f.got++
	return f.run, nil
}
func (f *runStoreFake) ReadEvents(context.Context, agentruntime.RunKey, int64, int) ([]agentruntime.RunEvent, error) {
	return f.events, nil
}
func TestAgentRunHandlerRejectsRunFromOtherOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := &runStoreFake{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r"}, SessionID: "s", UserID: "other"}}
	h := &Handler{}
	h.SetAgentRunService(service.NewAgentRunService(f))
	r := gin.New()
	r.GET("/sessions/:session_id/runs/:run_id", h.GetAgentRun)
	req := httptest.NewRequest("GET", "/sessions/s/runs/r", nil)
	req = req.WithContext(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if f.got != 0 {
		t.Fatal("store should not be called before owner is established")
	}
}

func TestAgentRunEventsReplayReportsLatestSeq(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := &runStoreFake{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r"}, SessionID: "s", UserID: "web_user:u1", Status: "succeeded"}, events: []agentruntime.RunEvent{{Seq: 2, Type: "token", Payload: json.RawMessage(`{"x":1}`)}, {Seq: 4, Type: "done", Payload: json.RawMessage(`{}`)}}}
	h := &Handler{sessionService: ownedSessionForRun{}}
	h.SetAgentRunService(service.NewAgentRunService(f))
	r := gin.New()
	r.GET("/sessions/:id/runs/:run_id", h.GetAgentRun)
	r.GET("/sessions/:id/runs/:run_id/events", h.GetAgentRunEvents)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
	statusReq := httptest.NewRequest("GET", "/sessions/s/runs/r", nil).WithContext(ctx)
	statusW := httptest.NewRecorder()
	r.ServeHTTP(statusW, statusReq)
	if !strings.Contains(statusW.Body.String(), "\"seq\":4") {
		t.Fatalf("status body=%s", statusW.Body.String())
	}
	req := httptest.NewRequest("GET", "/sessions/s/runs/r/events?once=1", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "token") || !strings.Contains(w.Body.String(), "done") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

type ownedSessionForRun struct{ interfaces.SessionService }

func (ownedSessionForRun) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return &types.Session{ID: "s"}, nil
}
