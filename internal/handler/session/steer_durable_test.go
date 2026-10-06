package session

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// steerStoreFake extends the run store fake with the durable steering input
// contracts the durable steer branch consumes.
type steerStoreFake struct {
	runStoreFake
	run       agentruntime.Run
	appended  []agentruntime.RunInput
	appendErr error
}

func (f *steerStoreFake) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return f.run, nil
}

func (f *steerStoreFake) AppendInput(
	_ context.Context, _ agentruntime.RunKey, in agentruntime.RunInput,
) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	for _, existing := range f.appended {
		if existing.SteerID == in.SteerID {
			if existing.Mode != in.Mode || string(existing.Message) != string(in.Message) {
				return agentruntime.ErrConflict
			}
			return nil
		}
	}
	f.appended = append(f.appended, in)
	return nil
}

func (f *steerStoreFake) ApplyInput(context.Context, agentruntime.Fence, string, agentruntime.CheckpointRecord) error {
	return nil
}

func (f *steerStoreFake) ListPendingInputs(
	_ context.Context, _ agentruntime.RunKey, mode string,
) ([]agentruntime.RunInput, error) {
	out := make([]agentruntime.RunInput, 0, len(f.appended))
	for _, in := range f.appended {
		if in.Mode == mode {
			out = append(out, in)
		}
	}
	return out, nil
}

type trpcSessionFake struct{ interfaces.SessionService }

func (trpcSessionFake) GetOwnedSession(context.Context, string) (*types.Session, error) {
	active := "run-1"
	return &types.Session{
		ID: "s", TenantID: 1, EngineType: "trpc", ActiveAgentRunID: &active,
	}, nil
}

func steerTestRouter(t *testing.T, session interfaces.SessionService, store *steerStoreFake) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{sessionService: session}
	h.SetAgentRunService(service.NewAgentRunService(store))
	r := gin.New()
	r.POST("/sessions/:session_id/steer", h.SteerMessage)
	return r
}

func TestSteerDurableRunQueuesInput(t *testing.T) {
	store := &steerStoreFake{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "s", Status: "running",
	}}
	r := steerTestRouter(t, trpcSessionFake{}, store)
	req := httptest.NewRequest("POST", "/sessions/s/steer", strings.NewReader(
		`{"query":"more context","delivery":"inject","steer_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"queued"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(store.appended) != 1 {
		t.Fatalf("appended=%v", store.appended)
	}
	got := store.appended[0]
	if got.SteerID != "11111111-1111-1111-1111-111111111111" || got.Mode != "inject" {
		t.Fatalf("input=%+v", got)
	}
	var payload struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(got.Message, &payload); err != nil ||
		payload.Role != "user" || payload.Content != "more context" {
		t.Fatalf("message=%s err=%v", got.Message, err)
	}
}

func TestSteerDurableRunConflictOnSteerIDReuse(t *testing.T) {
	store := &steerStoreFake{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "s", Status: "running",
	}}
	r := steerTestRouter(t, trpcSessionFake{}, store)
	body := `{"query":"first","steer_id":"11111111-1111-1111-1111-111111111111"}`
	first := httptest.NewRequest("POST", "/sessions/s/steer", strings.NewReader(body))
	first.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), first)
	clash := httptest.NewRequest("POST", "/sessions/s/steer", strings.NewReader(
		`{"query":"different","steer_id":"11111111-1111-1111-1111-111111111111"}`))
	clash.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, clash)
	if w.Code != 409 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSteerDurableRunTerminalAnswersNewRun(t *testing.T) {
	store := &steerStoreFake{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "s", Status: "succeeded",
	}}
	r := steerTestRouter(t, trpcSessionFake{}, store)
	req := httptest.NewRequest("POST", "/sessions/s/steer", strings.NewReader(`{"query":"next"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"new_run"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(store.appended) != 0 {
		t.Fatalf("terminal run must not consume input, appended=%v", store.appended)
	}
}
