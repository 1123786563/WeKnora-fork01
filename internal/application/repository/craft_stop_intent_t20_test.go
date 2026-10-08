package repository

// T20 (#139): the production stop-intent store (T17 hook) — the durable
// member stop fact survives service reconstruction, replays idempotently and
// a confirmed stop is never downgraded.
import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
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
	require.NoError(t, db.AutoMigrate(&agentRunRow{}))
	return db
}

func openStopIntentRaceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "stop-intent-race.db") + "?_foreign_keys=on&_busy_timeout=5000&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&craftStopIntentRow{}, &agentRunRow{}))
	return db
}

func seedStopIntentRun(t *testing.T, db *gorm.DB, scope craft.Scope, runID string) {
	t.Helper()
	require.NoError(t, db.Create(&agentRunRow{TenantID: scope.TenantID, RunID: runID,
		SessionID: scope.SessionID, OwnerID: scope.UserID, ActorUserID: scope.UserID,
		Status: "running"}).Error)
}

func TestCraftStopIntentStorePersistsAcrossReconstruction(t *testing.T) {
	db := openStopIntentDB(t)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-t20stop"}
	store := NewCraftStopIntentStore(db)
	require.NotNil(t, store)
	seedStopIntentRun(t, db, scope, "run-a")

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

func TestCraftStopIntentStoreRequiresExactRunIdentityBeforeAcceptingIntent(t *testing.T) {
	db := openStopIntentDB(t)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-exact-run"}
	store := NewCraftStopIntentStore(db)
	seedStopIntentRun(t, db, scope, "run-exact")

	_, err := store.PutStopIntent(context.Background(), scope,
		craft.StopIntent{RunID: "missing", Status: craft.StopRequested})
	require.ErrorIs(t, err, craft.ErrNotFound)

	wrongSession := scope
	wrongSession.SessionID = "another-session"
	_, err = store.PutStopIntent(context.Background(), wrongSession,
		craft.StopIntent{RunID: "run-exact", Status: craft.StopRequested})
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// TestCraftStopIntentStoreConcurrentConfirmNeverDowngrades pins the OCR race
// hardening: the no-downgrade guard must hold when a replay and the
// authoritative confirmation race — including the insert race where BOTH
// writers saw no row (no lock to take) and the conditional upsert assignment
// is the only defense. The file-backed IMMEDIATE-transaction fixture is the
// craft_writer_lease_test CAS recipe: racing transactions queue on the busy
// timeout and the guard itself decides the outcome.
func TestCraftStopIntentStoreConcurrentConfirmNeverDowngrades(t *testing.T) {
	db := openStopIntentRaceDB(t)

	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-race"}
	store := NewCraftStopIntentStore(db)
	require.NotNil(t, store)
	seedStopIntentRun(t, db, scope, "run-race")

	const writers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		status := craft.StopOutcomeStatus(craft.StopRequested)
		if i%3 == 0 {
			status = craft.StopUnknown
		}
		wg.Add(1)
		go func(status craft.StopOutcomeStatus, slot int) {
			defer wg.Done()
			<-start
			if _, err := store.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-race", Status: status}); err != nil {
				errs[slot] = err
			}
		}(status, i)
	}
	// The authoritative confirmation races the replays from an EMPTY row —
	// the harshest interleaving for the guard.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if _, err := store.PutStopIntent(ctx, scope, craft.StopIntent{RunID: "run-race", Status: craft.StopConfirmed}); err != nil {
			errs[writers-1] = err
		}
	}()
	close(start)
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	// However the writers interleaved, the confirmation is durable.
	got, err := NewCraftStopIntentStore(db).GetStopIntent(ctx, scope, "run-race")
	require.NoError(t, err)
	require.Equal(t, craft.StopConfirmed, got.Status, "a raced confirmation must never be downgraded by replays")
}
