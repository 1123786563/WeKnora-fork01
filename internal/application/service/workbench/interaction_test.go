package workbench

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type interactionStoreStub struct {
	current contract.InteractionDecision
	calls   int
}

type runProjection struct {
	TenantID                  uint64
	RunID, OwnerID, RequestID string
}

func (runProjection) TableName() string { return "agent_runs" }

func TestGormInteractionStoreScopesOwnerAndCASesDecision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_interactions?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
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
	err := svc.Command(interactionContext(), "r1", contract.ExecutionCommand{Action: "cancel"})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
	steer := &commandPortStub{}
	svc = NewInteractionService(nil, steer, nil)
	require.NoError(t, svc.Command(interactionContext(), "r1", contract.ExecutionCommand{Action: "steer", Text: "continue"}))
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
