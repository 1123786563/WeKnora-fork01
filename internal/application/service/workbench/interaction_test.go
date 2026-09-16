package workbench

import (
	"context"
	"errors"
	"sync"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type interactionStoreStub struct {
	current contract.InteractionDecision
	calls   int
}

func TestGormInteractionStoreScopesOwnerAndCASesDecision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_interactions?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&interactionRow{}))
	require.NoError(t, db.Create(&interactionRow{TenantID: 7, ID: "i1", RunID: "r1", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h"}).Error)
	store := NewGormInteractionStore(db)
	_, err = store.Get(context.Background(), 7, "other", "i1")
	require.ErrorIs(t, err, ErrInteractionNotFound)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := store.Decide(context.Background(), 7, "u1", "i1", contract.InteractionDecision{Action: "approve", DecisionID: "d1", ExpectedRevision: 0})
			results <- e
		}()
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
	store := &interactionStoreStub{current: contract.InteractionDecision{ID: "i1", Kind: "tool_approval", ArgsHash: "a"}}
	svc := NewInteractionService(store, nil, nil)
	_, err := svc.Decide(interactionContext(), "i1", contract.InteractionDecision{Kind: "budget", Action: "extend", ArgsHash: "a"})
	require.ErrorIs(t, err, contract.ErrInteractionActionMismatch)
	require.Zero(t, store.calls)
	_, err = svc.Decide(interactionContext(), "i1", contract.InteractionDecision{Kind: "tool_approval", Action: "approve", ArgsHash: "a", DecisionID: "d1"})
	require.NoError(t, err)
	require.Equal(t, 1, store.calls)
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
