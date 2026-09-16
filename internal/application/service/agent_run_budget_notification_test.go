package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type budgetExhaustedDurableChat struct{}

func (budgetExhaustedDurableChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, repocommercial.ErrTaskBudgetExhausted
}

func (budgetExhaustedDurableChat) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	return nil, repocommercial.ErrTaskBudgetExhausted
}

func (budgetExhaustedDurableChat) GetModelName() string { return "budget-exhausted-test" }
func (budgetExhaustedDurableChat) GetModelID() string   { return "budget-exhausted-test" }

// TestExecuteDurableRunPersistsBudgetExhaustionForNotification drives the
// production durable executor with a model/budget failure. The executor must
// append the durable budget_exhausted event; notification projection can then
// consume that event after a worker restart.
func TestExecuteDurableRunPersistsBudgetExhaustionForNotification(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	key := admitDurableRun(t, store, durableRunSnapshot(t))
	fence, err := store.Claim(durableRunCtx(), key, "budget-worker", time.Minute)
	require.NoError(t, err)

	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	svc := newDurableRunSessionService(t, db)
	svc.modelService = &durableRunModelService{chat: budgetExhaustedDurableChat{}}
	execErr := svc.ExecuteDurableRun(durableRunCtx(), fence)
	require.Error(t, execErr)

	events, err := store.ReadEvents(durableRunCtx(), key, 0, 32)
	require.NoError(t, err)
	var found bool
	for _, event := range events {
		if event.Type == "budget_exhausted" {
			found = true
			require.Contains(t, string(event.Payload), "budget_exhausted")
		}
	}
	require.True(t, found, "durable execution error=%v events=%v", execErr, events)
}
