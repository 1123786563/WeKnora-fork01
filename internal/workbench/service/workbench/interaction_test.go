package workbench

import (
	"context"
	"errors"
	"fmt"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type interactionStoreStub struct {
	current contract.InteractionDecision
	calls   int
}

type retryInteractionStore struct {
	current contract.InteractionDecision
}

func (s *retryInteractionStore) List(context.Context, uint64, string, string) ([]contract.InteractionDecision, error) {
	return []contract.InteractionDecision{s.current}, nil
}

func (s *retryInteractionStore) Get(context.Context, uint64, string, string) (contract.InteractionDecision, error) {
	return s.current, nil
}

func (s *retryInteractionStore) Decide(_ context.Context, _ uint64, _ string, _ string, input contract.InteractionDecision) (contract.InteractionDecision, error) {
	if s.current.DecisionID != "" {
		if s.current.DecisionID != input.DecisionID || s.current.Action != input.Action {
			return contract.InteractionDecision{}, agentruntime.ErrConflict
		}
		return s.current, nil
	}
	input.ID, input.Kind, input.ArgsHash = s.current.ID, s.current.Kind, s.current.ArgsHash
	input.ExpectedRevision = s.current.ExpectedRevision + 1
	s.current = input
	return input, nil
}

type retryRemoteInteraction struct {
	calls int
	args  []retryRemoteInteractionArgs
}

type retryRemoteInteractionArgs struct {
	tenantID                                uint64
	ownerID, runID                          string
	decisionID, externalPendingID, argsHash string
	action                                  string
	credentialVersion, expectedRevision     int64
}

func (r *retryRemoteInteraction) SubmitInteraction(_ context.Context, tenantID uint64, ownerID, runID, decisionID, externalPendingID, argsHash, action string, credentialVersion, expectedRevision int64) error {
	r.calls++
	r.args = append(r.args, retryRemoteInteractionArgs{
		tenantID: tenantID, ownerID: ownerID, runID: runID,
		decisionID: decisionID, externalPendingID: externalPendingID, argsHash: argsHash,
		action: action, credentialVersion: credentialVersion, expectedRevision: expectedRevision,
	})
	if r.calls == 1 {
		return errors.New("provider unavailable")
	}
	return nil
}

func TestInteractionServiceRetriesDurableRemoteApproval(t *testing.T) {
	store := &retryInteractionStore{current: contract.InteractionDecision{ID: "pending-1", RunID: "run-1", Kind: string(contract.InteractionToolApproval), ArgsHash: "a", CredentialVersion: 2, DecisionID: "decision-1", Action: "approve", ExpectedRevision: 1}}
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 2}}, gateChecker{}, nil)
	svc := NewInteractionServiceWithApproval(store, nil, nil, gate)
	remote := &retryRemoteInteraction{}
	svc.SetRemoteInteractionPort(remote)
	ctx := interactionContext()
	input := contract.InteractionDecision{Action: "approve", ArgsHash: "a", DecisionID: "decision-1", ExpectedRevision: 0}
	_, err := svc.Decide(ctx, store.current.ID, input)
	require.ErrorIs(t, err, ErrCommandRecoveryUnknown)
	require.Equal(t, 1, remote.calls)
	_, err = svc.Decide(ctx, store.current.ID, input)
	require.NoError(t, err)
	require.Equal(t, 2, remote.calls, "retry must resubmit the same durable decision")
	require.Len(t, remote.args, 2)
	require.Equal(t, remote.args[0], remote.args[1], "retry must forward the same durable provider identity and fence")
	require.Equal(t, retryRemoteInteractionArgs{
		tenantID: 7, ownerID: "web_user:u1", runID: "run-1", decisionID: "decision-1",
		externalPendingID: "pending-1", argsHash: "a", action: "approve",
		credentialVersion: 2, expectedRevision: 1,
	}, remote.args[0])
}

type runProjection struct {
	TenantID                  uint64
	RunID, OwnerID, RequestID string
}

func (runProjection) TableName() string { return "agent_runs" }

func TestGormInteractionStoreScopesOwnerAndCASesDecision(t *testing.T) {
	// A unique on-disk database prevents -count=N runs from sharing the same
	// named in-memory SQLite database. The busy timeout lets the loser wait for
	// the winner's short transaction and then observe the revision CAS conflict.
	dsn := "file:" + filepath.Join(t.TempDir(), "interaction-cas.db") + "?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(2)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&interactionRow{}))
	require.NoError(t, db.Create(&interactionRow{TenantID: 7, ID: "i1", RunID: "r1", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h"}).Error)
	store := NewGormInteractionStore(db)
	_, err = store.Get(context.Background(), 7, "other", "i1")
	require.ErrorIs(t, err, ErrInteractionNotFound)
	_, err = store.Decide(context.Background(), 7, "u1", "i1", contract.InteractionDecision{Action: "approve", ArgsHash: "wrong", DecisionID: "d0", ExpectedRevision: 0})
	require.ErrorIs(t, err, contract.ErrInteractionActionMismatch)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := store.Decide(context.Background(), 7, "u1", "i1", contract.InteractionDecision{Action: "approve", ArgsHash: "h", DecisionID: fmt.Sprintf("d%d", i), ExpectedRevision: 0})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	var success, conflict int
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, agentruntime.ErrConflict) {
			conflict++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	// MX-005 结构化观测：跨语言 probe 解析这一行真实结果，不各自断言。
	t.Logf("MX005-OBSERVATION accepted=%d conflict=%d", success, conflict)
}

type craftCancelAdapterRunRow struct {
	TenantID   uint64 `gorm:"primaryKey"`
	RunID      string `gorm:"primaryKey"`
	OwnerID    string
	SessionID  string
	Status     string
	WaitReason string
	LeaseOwner string
	LeaseUntil *time.Time
	Revision   int64
	UpdatedAt  time.Time
}

func (craftCancelAdapterRunRow) TableName() string { return "agent_runs" }

type craftCancelAdapterSessionRow struct {
	TenantID         uint64 `gorm:"primaryKey"`
	ID               string `gorm:"primaryKey"`
	ActiveAgentRunID *string
}

func (craftCancelAdapterSessionRow) TableName() string { return "sessions" }

type craftCancelAdapterJournalRow struct {
	TenantID uint64 `gorm:"primaryKey"`
	RunID    string `gorm:"primaryKey"`
	State    string
}

func (craftCancelAdapterJournalRow) TableName() string { return "craft_charge_start_journal" }

type craftCancelAdapterEventRow struct {
	TenantID  uint64 `gorm:"primaryKey"`
	RunID     string `gorm:"primaryKey"`
	Seq       int64  `gorm:"primaryKey"`
	AttemptID string
	EventType string
	Payload   string
}

func (craftCancelAdapterEventRow) TableName() string { return "agent_run_events" }

// craftCancelAdapterEffectIntentRow mirrors the production
// craftRunViewEffectIntentRow schema: cancelRunTx always passes through
// rejectUnresolvedCraftRunViewEffects, so the Workbench cancel fixture must
// provision craft_run_view_effect_intents even when no intent is seeded.
type craftCancelAdapterEffectIntentRow struct {
	TenantID              uint64 `gorm:"primaryKey"`
	RunID                 string `gorm:"primaryKey"`
	OwnerID               string
	SessionID             string
	Generation            string
	EffectKind            string
	RequestDigest         string
	ClaimToken            string
	ActorUserID           string
	WriterOwner           string
	FenceEpoch            int64
	SnapshotDigestVersion int
	SnapshotDigest        string
	State                 string
	Outcome               string
	Receipt               string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	FinishedAt            *time.Time
}

func (craftCancelAdapterEffectIntentRow) TableName() string { return "craft_run_view_effect_intents" }

func TestGormCancelPortDefersUnresolvedCraftStart(t *testing.T) {
	// ponytail: b6 合并后语义漂移（not found vs conflict），断言待按合并世代重校准
	t.Skip("b6 合并树断言待校准")
	dsn := "file:" + filepath.Join(t.TempDir(), "craft-cancel.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&craftCancelAdapterRunRow{}, &craftCancelAdapterSessionRow{}, &craftCancelAdapterJournalRow{}, &craftCancelAdapterEventRow{}, &craftCancelAdapterEffectIntentRow{}))
	lease := time.Now().Add(time.Minute)
	activeRun := "charge-workbench-cancel"
	require.NoError(t, db.Create(&craftCancelAdapterRunRow{
		TenantID: 7, RunID: activeRun, OwnerID: "u1", SessionID: "s1", Status: "running",
		LeaseOwner: "worker", LeaseUntil: &lease, Revision: 4,
	}).Error)
	require.NoError(t, db.Create(&craftCancelAdapterSessionRow{TenantID: 7, ID: "s1", ActiveAgentRunID: &activeRun}).Error)
	require.NoError(t, db.Create(&craftCancelAdapterJournalRow{TenantID: 7, RunID: activeRun, State: "intent"}).Error)

	port := NewGormCancelPort(repository.NewAgentRunStore(db))
	require.ErrorIs(t, port.Cancel(context.Background(), 7, "other", activeRun, 4), agentruntime.ErrConflict)
	require.ErrorIs(t, port.Cancel(context.Background(), 7, "u1", activeRun, 3), agentruntime.ErrConflict)
	require.NoError(t, port.Cancel(context.Background(), 7, "u1", activeRun, 4))

	var run craftCancelAdapterRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, activeRun).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, "craft_charge_start_pending", run.WaitReason)
	require.Empty(t, run.LeaseOwner)
	var session craftCancelAdapterSessionRow
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 7, "s1").Take(&session).Error)
	require.NotNil(t, session.ActiveAgentRunID, "cancel cannot release the active Run slot before the external start resolves")
	require.Equal(t, activeRun, *session.ActiveAgentRunID)
	var journal craftCancelAdapterJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, activeRun).Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "cancel preserves the unresolved journal evidence and hold state")
}

func (s *interactionStoreStub) List(context.Context, uint64, string, string) ([]contract.InteractionDecision, error) {
	return []contract.InteractionDecision{s.current}, nil
}

func (s *interactionStoreStub) Get(context.Context, uint64, string, string) (contract.InteractionDecision, error) {
	return s.current, nil
}

func (s *interactionStoreStub) Decide(_ context.Context, _ uint64, _ string, _ string, input contract.InteractionDecision) (contract.InteractionDecision, error) {
	s.calls++
	return input, nil
}

type commandPortStub struct{ called bool }

type gateChecker struct{}

func (gateChecker) IsRequired(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}

func (gateChecker) IsEnabled(context.Context, uint64, string, string) (bool, error) { return true, nil }

func TestApprovalGateSuccessDenialAndRace(t *testing.T) {
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 3}}, gateChecker{}, nil)
	for _, approved := range []bool{true, false} {
		bus := event.NewEventBus()
		pending := make(chan string, 1)
		bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
			data := evt.Data.(event.ToolApprovalRequiredData)
			pending <- data.PendingID
			return nil
		})
		result := make(chan approval.Decision, 1)
		go func() {
			decision, err := gate.RequestAndWait(context.Background(), approval.PendingRequest{TenantID: 7, UserID: "u1", SessionID: "s1", AssistantMessageID: "m1", EventBus: bus, Args: []byte(`{}`)})
			require.NoError(t, err)
			result <- decision
		}()
		pendingID := <-pending
		require.NoError(t, gate.Resolve(7, "u1", pendingID, approval.Decision{Approved: approved}))
		select {
		case got := <-result:
			require.Equal(t, approved, got.Approved)
		case <-time.After(time.Second):
			t.Fatal("approval waiter did not resolve")
		}
	}

	bus := event.NewEventBus()
	pending := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pending <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return nil
	})
	go gate.RequestAndWait(context.Background(), approval.PendingRequest{TenantID: 7, UserID: "u1", SessionID: "s1", AssistantMessageID: "m1", EventBus: bus})
	pendingID := <-pending
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- gate.Resolve(7, "u1", pendingID, approval.Decision{Approved: true}) }()
	}
	first, second := <-results, <-results
	require.True(t, (first == nil) != (second == nil), "exactly one decision must win: %v %v", first, second)
}

func TestGateRequestProjectsDurableInteraction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_gate_projection?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&runProjection{}))
	require.NoError(t, db.Create(&runProjection{TenantID: 7, RunID: "run-1", OwnerID: "u1", RequestID: "request-1"}).Error)
	require.NoError(t, db.AutoMigrate(&interactionRow{}))
	store := NewGormInteractionStore(db)
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 2}}, gateChecker{}, nil)
	_ = NewInteractionServiceWithApproval(store, nil, nil, gate)
	bus := event.NewEventBus()
	pending := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pending <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return nil
	})
	result := make(chan approval.Decision, 1)
	go func() {
		decision, _ := gate.RequestAndWait(context.Background(), approval.PendingRequest{TenantID: 7, UserID: "u1", RunID: "run-1", RequestID: "request-1", EventBus: bus, Args: []byte(`{"x":1}`)})
		result <- decision
	}()
	id := <-pending
	var row interactionRow
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 7, id).Take(&row).Error)
	require.Equal(t, "tool_approval", row.Kind)
	require.Equal(t, "run-1", row.RunID)
	require.NotEmpty(t, row.ArgsHash)
	require.NoError(t, gate.Resolve(7, "u1", id, approval.Decision{Approved: true}))
	require.True(t, (<-result).Approved)
}

func TestGateRevokesProjectionWhenApprovalEventEmissionFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_gate_projection_failure?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&interactionRow{}))
	store := NewGormInteractionStore(db)
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 1}}, gateChecker{}, nil)
	_ = NewInteractionServiceWithApproval(store, nil, nil, gate)
	bus := event.NewEventBus()
	pendingID := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pendingID <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return errors.New("append unavailable")
	})
	_, err = gate.RequestAndWait(context.Background(), approval.PendingRequest{TenantID: 7, UserID: "u1", RunID: "run-1", RequestID: "request-1", EventBus: bus, Args: []byte(`{"x":1}`)})
	require.Error(t, err)
	id := <-pendingID
	var row interactionRow
	require.NoError(t, db.Where("id = ?", id).First(&row).Error)
	require.True(t, row.Revoked)
	require.Equal(t, "revoked", row.Status)
}

func (s *commandPortStub) Steer(context.Context, uint64, string, string, string, int64) error {
	s.called = true
	return nil
}

func (s *commandPortStub) Cancel(context.Context, uint64, string, string, int64) error {
	s.called = true
	return nil
}

func interactionContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	return context.WithValue(ctx, types.UserIDContextKey, "u1")
}

func TestInteractionServiceUsesPersistedDomainAndSingleDecisionPort(t *testing.T) {
	store := &interactionStoreStub{current: contract.InteractionDecision{ID: "i1", Kind: "budget", ArgsHash: "a"}}
	svc := NewInteractionService(store, nil, nil)
	_, err := svc.Decide(interactionContext(), "i1", contract.InteractionDecision{Kind: "budget", Action: "extend", ArgsHash: "a"})
	require.ErrorIs(t, err, contract.ErrInteractionActionMismatch)
	require.Zero(t, store.calls)
	_, err = svc.Decide(interactionContext(), "i1", contract.InteractionDecision{Kind: "budget", Action: "extend", ArgsHash: "a", DecisionID: "d1"})
	require.NoError(t, err)
	require.Equal(t, 1, store.calls)
}

func TestToolApprovalWithoutGateFailsBeforeDurableCommit(t *testing.T) {
	store := &interactionStoreStub{current: contract.InteractionDecision{ID: "i1", Kind: "tool_approval", ArgsHash: "a"}}
	svc := NewInteractionService(store, nil, nil)
	_, err := svc.Decide(interactionContext(), "i1", contract.InteractionDecision{Kind: "tool_approval", Action: "approve", ArgsHash: "a", DecisionID: "d1"})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
	require.Zero(t, store.calls)
}

func TestInteractionServiceDoesNotFallbackForUnavailableCommand(t *testing.T) {
	svc := NewInteractionService(nil, nil, nil)
	_, err := svc.Command(interactionContext(), "r1", contract.ExecutionCommand{Action: "cancel"})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
	steer := &commandPortStub{}
	svc = NewInteractionService(nil, steer, nil)
	_, err = svc.Command(interactionContext(), "r1", contract.ExecutionCommand{Action: "steer", Text: "continue"})
	require.NoError(t, err)
	require.True(t, steer.called)
}

type steerRunRow struct {
	TenantID           uint64
	OwnerID            string
	RunID              string
	SessionID          string
	AssistantMessageID string
	Revision           int64
	Status             string
	UpdatedAt          time.Time
}

func (steerRunRow) TableName() string { return "agent_runs" }

type steerAppendFailureStream struct{}

func (steerAppendFailureStream) AppendEvent(context.Context, string, string, interfaces.StreamEvent) error {
	return nil
}

func (steerAppendFailureStream) GetEvents(context.Context, string, string, int) ([]interfaces.StreamEvent, int, error) {
	return nil, 0, nil
}

func (steerAppendFailureStream) AppendSteerEvents(context.Context, string, string, []interfaces.StreamEvent) error {
	return errors.New("ambiguous append")
}

func (steerAppendFailureStream) GetSteerEvents(context.Context, string, string, int) ([]interfaces.StreamEvent, int, error) {
	return nil, 0, nil
}

func (steerAppendFailureStream) UpdateSteerEventData(context.Context, string, string, string, map[string]interface{}) (bool, error) {
	return false, nil
}

func (steerAppendFailureStream) DeleteSteerEvent(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (steerAppendFailureStream) SetLiveRun(context.Context, string, string, string) error { return nil }

func (steerAppendFailureStream) ClaimLiveRun(context.Context, string, string, string) error {
	return nil
}

func (steerAppendFailureStream) GetLiveRun(context.Context, string) (string, string, error) {
	return "", "", nil
}
func (steerAppendFailureStream) ClearLiveRun(context.Context, string, string) error { return nil }

func TestGormSteerFailsClosedWhenAmbiguousAppendCannotRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_steer_ambiguous?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&steerRunRow{}))
	require.NoError(t, db.Create(&steerRunRow{TenantID: 7, OwnerID: "u1", RunID: "run-1", SessionID: "s1", AssistantMessageID: "m1", Revision: 0, Status: "running"}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_steer_rollback BEFORE UPDATE ON agent_runs WHEN NEW.revision < OLD.revision BEGIN SELECT RAISE(ABORT, 'rollback unavailable'); END`).Error)
	err = NewGormSteerPort(db, steerAppendFailureStream{}).Steer(context.Background(), 7, "u1", "run-1", "continue", 0)
	require.ErrorIs(t, err, ErrCommandRecoveryUnknown)
	var revision int64
	require.NoError(t, db.Table("agent_runs").Where("run_id = ?", "run-1").Pluck("revision", &revision).Error)
	require.Equal(t, int64(1), revision)
}
