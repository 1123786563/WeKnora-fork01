package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type admissionRaceContextKey string

func TestAgentRunAdmitRejectsRetiredVariant(t *testing.T) {
	db := openRunTestDB(t)
	versionID, releaseID := seedAgentVariant(t, db, "retired")
	store := NewAgentRunStore(db)
	in := testRetiredAgentAdmission(versionID, releaseID)

	_, err := store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrAgentUseDenied)
	var runs, messages int64
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
	require.NoError(t, db.Table("messages").Where("session_id = ?", in.SessionID).Count(&messages).Error)
	require.Zero(t, runs)
	require.Zero(t, messages)
	var slot *string
	require.NoError(t, db.Table("sessions").Where("id = ?", in.SessionID).Select("active_agent_run_id").Scan(&slot).Error)
	require.Nil(t, slot)
}

func TestAgentRunAdmitSerializesWithVariantRetirement(t *testing.T) {
	db := openRunTestDB(t)
	versionID, releaseID := seedAgentVariant(t, db, "published")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)

	locked := make(chan struct{})
	allowCommit := make(chan struct{})
	retireAttempted := make(chan struct{})
	var once sync.Once
	name := "test:admit-retire-variant-lock"
	require.NoError(t, db.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoption_variants" && tx.Statement.Context.Value(admissionRaceContextKey("op")) == "admit" {
			once.Do(func() { close(locked) })
			<-allowCommit
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(name) })
	retireName := "test:retire-variant-attempt"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(retireName, func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_adoption_variants" && tx.Statement.Context.Value(admissionRaceContextKey("op")) == "retire" {
			close(retireAttempted)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(retireName) })

	admitDone := make(chan error, 1)
	go func() {
		_, admitErr := NewAgentRunStore(db).Admit(context.WithValue(context.Background(), admissionRaceContextKey("op"), "admit"), testRetiredAgentAdmission(versionID, releaseID))
		admitDone <- admitErr
	}()
	<-locked // the gate's row UPDATE executed while its transaction is open
	retireDone := make(chan error, 1)
	go func() {
		_, retireErr := NewAgentAdoptionRepository(db).UpdateVariantState(context.WithValue(context.Background(), admissionRaceContextKey("op"), "retire"), 1, "retired-variant", []string{"published"}, "retired", nil)
		retireDone <- retireErr
	}()
	<-retireAttempted
	select {
	case err := <-retireDone:
		t.Fatalf("retirement completed before the admission transaction released its variant lock: %v", err)
	default:
	}
	close(allowCommit)
	require.NoError(t, <-admitDone)
	require.NoError(t, <-retireDone)

	_, err = NewAgentRunStore(db).Admit(context.Background(), testRetiredAgentAdmission(versionID, releaseID))
	require.NoError(t, err, "same-request idempotency must return the committed run before checking retirement")
}

func seedAgentVariant(t *testing.T, db *gorm.DB, state string) (versionID, releaseID string) {
	t.Helper()
	require.NoError(t, db.Create(&types.CustomAgent{ID: "source-agent", TenantID: 1, Name: "source", CreatedBy: "u1"}).Error)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "source-agent", "1.0.0")
	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-retired", TenantID: 1, Name: "local", CreatedBy: "u1"}).Error)
	versionID = "agent-retired-version"
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: versionID, TenantID: 1, AgentID: "agent-retired", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha", FrozenBy: "u1"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "retired-adoption", ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "u1"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		TenantID: 1, ID: "retired-variant", AdoptionID: "retired-adoption", ReleaseID: releaseID,
		Name: "retired", State: state, LocalAgentID: "agent-retired", LocalAgentVersionID: versionID,
	}).Error)
	return versionID, releaseID
}

func testRetiredAgentAdmission(versionID, releaseID string) agentruntime.Admission {
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "retired-agent-run"}, SessionID: "s1", AgentID: "agent-retired",
		LocalAgentVersionID: versionID, ReleaseID: releaseID,
		UserID: "u1", RequestID: "retired-agent-request", AssistantMessageID: "retired-agent-assistant", RequestHash: "retired-agent-hash",
		// The snapshot is deliberately inconsistent: the gate must consume
		// explicit server-side Admission.AgentID, never client-shaped JSON.
		Snapshot: json.RawMessage(`{"agent_id":"agent-live"}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	}
}

// TestAgentRunAdmitToleratesPrecreatedAssistantMessage pins the production
// HTTP contract: the SSE handler persists the assistant placeholder with the
// request-scoped id before admission runs, so the admission transaction must
// not fail on the existing row - the run owns finalization by id regardless.
func TestAgentRunAdmitToleratesPrecreatedAssistantMessage(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()

	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	adm := agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "adm-pre-r1"},
		SessionID: "s1", UserID: "u1", RequestID: "adm-pre-q1",
		AssistantMessageID: "amsg-pre-1", RequestHash: "adm-pre-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	}
	_, err := store.Admit(ctx, adm)
	require.NoError(t, err)

	// Terminal the first run and free the slot, then admit a second run
	// that REUSES the same assistant message id, exactly like a handler
	// retry that already persisted the assistant placeholder.
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET status='succeeded' WHERE tenant_id=1 AND run_id='adm-pre-r1'").Error)
	require.NoError(t, db.Exec(
		"UPDATE sessions SET active_agent_run_id=NULL WHERE id='s1'").Error)
	adm2 := adm
	adm2.Key.RunID = "adm-pre-r2"
	adm2.RequestID = "adm-pre-q2"
	adm2.RequestHash = "adm-pre-h2"
	run2, err := store.Admit(ctx, adm2)
	require.NoError(t, err, "admission must tolerate the handler-precreated assistant row")
	require.NotEmpty(t, run2.Key.RunID)
}

// TestAgentRunAdmitReusesHandlerUserMessage pins the single-user-row
// contract: when the handler already persisted the user message and
// admission carries its id, no second user row appears.
func TestAgentRunAdmitReusesHandlerUserMessage(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()

	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "adm-user-r1"},
		SessionID: "s1", UserID: "u1", RequestID: "adm-user-q1",
		UserMessageID:      "umsg-handler-1",
		AssistantMessageID: "amsg-handler-1", RequestHash: "adm-user-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	var users int64
	require.NoError(t, db.Table("messages").
		Where("session_id = ? AND role = ?", "s1", "user").Count(&users).Error)
	require.EqualValues(t, 1, users, "exactly one user message row must exist")
	var id string
	require.NoError(t, db.Table("messages").
		Where("session_id = ? AND role = ?", "s1", "user").Select("id").Scan(&id).Error)
	require.Equal(t, "umsg-handler-1", id, "the handler-persisted row id must be reused")
}
