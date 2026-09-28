package repository

// T20 (#139): the production stop-intent store (T17 hook) — the durable
// member stop fact survives service reconstruction, replays idempotently and
// a confirmed stop is never downgraded.
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openStopIntentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&craftStopIntentRow{}))
	return db
}

func TestCraftStopIntentStorePersistsAcrossReconstruction(t *testing.T) {
	db := openStopIntentDB(t)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-t20stop"}
	store := NewCraftStopIntentStore(db)
	require.NotNil(t, store)

	// No stop journey yet answers the pinned sentinel — never a fabricated
	// status.
	_, err := store.GetStopIntent(ctx, scope, "run-a")
	require.ErrorIs(t, err, craft.ErrNotFound)

	// requested persists BEFORE anything is aborted (the T17 ordering).
	got, err := store.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-a", Status: craft.StopRequested})
	require.NoError(t, err)
	require.Equal(t, craft.StopRequested, got.Status)

	// A reconstructed store (page refresh / new process) reads the same fact.
	fresh := NewCraftStopIntentStore(db)
	got, err = fresh.GetStopIntent(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, craft.StopRequested, got.Status)

	// The authoritative confirmation lands; the timestamped row survives.
	_, err = fresh.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-a", Status: craft.StopConfirmed})
	require.NoError(t, err)
	got, err = NewCraftStopIntentStore(db).GetStopIntent(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, craft.StopConfirmed, got.Status)

	// A late requested/unknown replay NEVER downgrades the confirmation.
	for _, replay := range []craft.StopOutcomeStatus{craft.StopRequested, craft.StopUnknown} {
		got, err = store.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-a", Status: replay})
		require.NoError(t, err)
		require.Equal(t, craft.StopConfirmed, got.Status, "a %s replay must not downgrade the confirmed stop", replay)
	}
	got, err = NewCraftStopIntentStore(db).GetStopIntent(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, craft.StopConfirmed, got.Status)

	// Scoped isolation: another session's run is a different journey.
	other := scope
	other.SessionID = "s-other"
	_, err = store.GetStopIntent(ctx, other, "run-a")
	require.ErrorIs(t, err, craft.ErrNotFound)

	// The closed vocabulary refuses foreign statuses; a nil store refuses
	// closed.
	_, err = store.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-b", Status: "maybe"})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = NewCraftStopIntentStore(nil).GetStopIntent(ctx, scope, "run-b")
	require.Error(t, err)
}
