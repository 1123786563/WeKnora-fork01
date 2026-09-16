package workbench

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
)

type interactionStoreStub struct {
	current contract.InteractionDecision
	calls   int
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
