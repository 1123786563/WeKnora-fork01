package session

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
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
