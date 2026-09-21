package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	// Continue through the production notification worker using the same
	// durable database. The device is registered only after the executor has
	// persisted the budget event, matching the normal projection boundary.
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'budget-device', 'dev', 'ios', 'cipher', 'budget-hash')`).Error)
	notifications := repository.NewNotificationStore(db)
	worker := workbench.NewNotificationWorker(
		workbench.NewNotificationProjector(store, notifications), notifications,
	)
	require.NoError(t, worker.RunOnce(durableRunCtx()))
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Where("run_id = ? AND kind = ?", key.RunID, "budget_exhausted").Count(&intents).Error)
	require.EqualValues(t, 1, intents)
	var checkpoint struct{ Cursor int64 }
	require.NoError(t, db.Table("mobile_notification_checkpoints").Select("cursor").Where("consumer = ? AND tenant_id = ? AND run_id = ?", "mobile-notification-projector", key.TenantID, key.RunID).Take(&checkpoint).Error)
	require.Positive(t, checkpoint.Cursor)

	// Recreate both store and worker after closing the SQLite connection. A
	// second pass must replay safely without creating a duplicate intent or
	// moving the committed cursor backwards.
	dialector, ok := db.Dialector.(*sqlite.Dialector)
	require.True(t, ok)
	dsn := dialector.DSN
	conn, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db2, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	defer func() {
		c, _ := db2.DB()
		if c != nil {
			_ = c.Close()
		}
	}()
	store2 := repository.NewAgentRunStore(db2)
	notifications2 := repository.NewNotificationStore(db2)
	worker2 := workbench.NewNotificationWorker(
		workbench.NewNotificationProjector(store2, notifications2), notifications2,
	)
	require.NoError(t, worker2.RunOnce(durableRunCtx()))
	require.NoError(t, db2.Table("mobile_notification_intents").Where("run_id = ? AND kind = ?", key.RunID, "budget_exhausted").Count(&intents).Error)
	require.EqualValues(t, 1, intents)
	require.NoError(t, db2.Table("mobile_notification_checkpoints").Select("cursor").Where("consumer = ? AND tenant_id = ? AND run_id = ?", "mobile-notification-projector", key.TenantID, key.RunID).Take(&checkpoint).Error)
	require.Positive(t, checkpoint.Cursor)
}
