package session

// T18 (#137) — authoritative reconnect journey tests.
//
// The ticket's user journey is "refresh or disconnect, then resume observing
// the SAME Run without submitting a replacement command". Everything here
// drives the highest available Craft API seam over the real database, the
// real craft session service and the real run-event store, and asserts only
// externally visible responses plus persisted rows:
//
//   - refresh during Run loads the authoritative workspace snapshot first
//     (active run identity + last_seq watermark) and only then continues the
//     event stream after that watermark;
//   - duplicate replays never redeliver seqs the cursor already covers;
//   - a sequence gap (retention-trimmed middle event) answers the explicit
//     cursor_expired reload error instead of silently skipping events;
//   - an expired cursor never leads to a resubmission: one user intent keeps
//     exactly one Run row across refresh/reconnect/replay;
//   - a delegation-finished event arrives on the stream but never finishes
//     the main Run, and an unknown outcome stays waiting_user (with its
//     pending identity) instead of being resolved optimistically.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// mountT18RunEvents mounts the durable run read surface (the same routes the
// production chat router registers) onto the craft HTTP environment so one
// journey can walk craft snapshot -> run events -> reconnect.
func (env *craftHTTPEnv) mountT18RunEvents() {
	h := &Handler{sessionService: &craftHTTPSessions{db: env.db}}
	h.SetAgentRunService(env.runs)
	env.engine.GET("/api/v1/sessions/:id/runs/:run_id", h.GetAgentRun)
	env.engine.GET("/api/v1/sessions/:id/runs/:run_id/events", h.GetAgentRunEvents)
}

func (env *craftHTTPEnv) t18Store(t *testing.T) *repository.AgentRunStore {
	t.Helper()
	store, ok := env.runs.Store().(*repository.AgentRunStore)
	require.True(t, ok, "the craft HTTP harness must run on the real durable run store")
	return store
}

func (env *craftHTTPEnv) t18Claim(t *testing.T, runID string) agentruntime.Fence {
	t.Helper()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	fence, err := env.t18Store(t).Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, "t18-worker", time.Hour)
	require.NoError(t, err)
	return fence
}

func (env *craftHTTPEnv) t18AppendEvent(t *testing.T, fence agentruntime.Fence, eventType string, payload string) {
	t.Helper()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	_, err := env.t18Store(t).AppendEvent(ctx, fence, agentruntime.RunEvent{
		Type: eventType, Payload: json.RawMessage(payload),
	})
	require.NoError(t, err)
}

// TestCraftT18Journey walks refresh/duplicate/gap/expired-cursor/delegation
// on one live craft run, asserting the reconnect contract end to end.
func TestCraftT18Journey(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	env.mountT18RunEvents()
	created := env.createSession(t, "t18-create", "重连作品站", "web")
	sessionID := created["session_id"].(string)

	// One user intent: the first submit admits exactly one Run…
	w := env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		`{"request_id":"t18-intent","prompt":"做一个落地页"}`)
	require.Equal(t, http.StatusAccepted, w.Code, "submit body: %s", w.Body.String())
	var runBody struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runBody))
	runID := runBody.Data["run_id"].(string)
	require.NotEmpty(t, runID)

	// …and a retried submit of the SAME intent replays the SAME admission —
	// a reconnect never mints a second Run for one intent.
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		`{"request_id":"t18-intent","prompt":"做一个落地页"}`)
	require.Equal(t, http.StatusAccepted, w.Code, "replay body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), runID, "intent replay must return the original run")
	var runCount int64
	require.NoError(t, env.db.Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", 1, sessionID).Count(&runCount).Error)
	require.EqualValues(t, 1, runCount, "one user intent must persist exactly one Run row")

	// Refresh during the Run: the authoritative workspace snapshot comes
	// first and names the same run, a non-terminal status and the watermark.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code, "snapshot body: %s", w.Body.String())
	var snapshot struct {
		Data struct {
			ActiveRunID string `json:"active_run_id"`
			ActiveRun   *struct {
				RunID  string `json:"run_id"`
				Status string `json:"status"`
			} `json:"active_run"`
			LastSeq int64 `json:"last_seq"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snapshot))
	require.Equal(t, runID, snapshot.Data.ActiveRunID, "refresh must reuse the same authoritative Run")
	require.NotNil(t, snapshot.Data.ActiveRun)
	require.Equal(t, runID, snapshot.Data.ActiveRun.RunID)
	require.Equal(t, "queued", snapshot.Data.ActiveRun.Status, "a just-admitted run projects its durable status, never a guessed terminal one")
	require.EqualValues(t, 0, snapshot.Data.LastSeq)

	// The durable worker records run events (the seam the stream replays).
	fence := env.t18Claim(t, runID)
	for i := 1; i <= 3; i++ {
		env.t18AppendEvent(t, fence, "tool_dispatched", fmt.Sprintf(`{"n":%d}`, i))
	}

	// The snapshot watermark catches up before any event continuation.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snapshot))
	require.EqualValues(t, 3, snapshot.Data.LastSeq, "the snapshot is the authority for the resume cursor")

	// Event continuation AFTER the watermark: seq 1..3 arrive in order.
	fetch := func(after string) *httptest.ResponseRecorder {
		return env.do(t, http.MethodGet,
			"/api/v1/sessions/"+sessionID+"/runs/"+runID+"/events?after="+after+"&once=1", "", "")
	}
	w = fetch("0")
	require.Equal(t, http.StatusOK, w.Code, "events body: %s", w.Body.String())
	firstPull := w.Body.String()
	for seq := 1; seq <= 3; seq++ {
		require.Contains(t, firstPull, fmt.Sprintf(`"seq":%d`, seq))
	}

	// Duplicate replays are harmless: the same cursor redelivers the same
	// page and never anything at or before the cursor.
	w = fetch("1")
	require.Equal(t, http.StatusOK, w.Code)
	secondPull := w.Body.String()
	require.Contains(t, secondPull, `"seq":2`)
	require.Contains(t, secondPull, `"seq":3`)
	require.False(t, strings.Contains(secondPull, `"seq":1`),
		"a replay must never redeliver seq the cursor already covers: %s", secondPull)

	// A sequence gap (retention trimmed the middle event) answers the
	// explicit reload error instead of silently skipping ahead.
	require.NoError(t, env.db.Exec(
		"DELETE FROM agent_run_events WHERE tenant_id = 1 AND run_id = ? AND seq = 2", runID).Error)
	w = fetch("1")
	require.Equal(t, http.StatusConflict, w.Code, "gap body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "cursor_expired")

	// The expired-cursor recovery path is snapshot-first and never submits:
	// reload the authoritative snapshot, resume after its watermark, and the
	// persisted Run count is still exactly one.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snapshot))
	require.EqualValues(t, 3, snapshot.Data.LastSeq)
	w = fetch("3")
	require.Equal(t, http.StatusOK, w.Code, "resume body: %s", w.Body.String())
	require.NotContains(t, w.Body.String(), `"seq":1`)
	require.NotContains(t, w.Body.String(), `"seq":2`)
	require.NoError(t, env.db.Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", 1, sessionID).Count(&runCount).Error)
	require.EqualValues(t, 1, runCount, "cursor expiry must never mint a replacement Run")

	// A child delegation finishes while the main Run is still working: the
	// event reaches the stream…
	env.t18AppendEvent(t, fence, "delegation.finished", `{"task_id":"dlg_1","status":"finished"}`)
	w = fetch("3")
	require.Equal(t, http.StatusOK, w.Code, "delegation body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "delegation.finished")
	// …but the main Run projection stays non-terminal.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snapshot))
	require.Equal(t, runID, snapshot.Data.ActiveRunID)
	require.NotNil(t, snapshot.Data.ActiveRun)
	require.NotContains(t, []string{"succeeded", "failed", "canceled"}, snapshot.Data.ActiveRun.Status,
		"a delegation completion must never finish the main Run")

	// Unknown stays unknown: the durable unknown park (waiting_user with the
	// delegation's pending identity) projects as a wait, never as a guessed
	// terminal outcome, and the run still owns the session slot.
	require.NoError(t, env.db.Exec(
		"UPDATE agent_runs SET status = 'waiting_user', wait_reason = 'call-t18-1' WHERE tenant_id = 1 AND run_id = ?",
		runID).Error)
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	var parked struct {
		Data struct {
			ActiveRun *struct {
				Status string `json:"status"`
			} `json:"active_run"`
			PendingID string `json:"pending_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parked))
	require.Equal(t, "waiting_user", parked.Data.ActiveRun.Status)
	require.Equal(t, "call-t18-1", parked.Data.PendingID, "the unknown park keeps its decision identity")
}
