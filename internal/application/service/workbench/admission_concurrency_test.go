package workbench

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type countingBudget struct{ ensures atomic.Int32 }

func (b *countingBudget) Ensure(_ context.Context, _ uint64, _ string, requestID string, _ int64, _ time.Time) (string, error) {
	b.ensures.Add(1)
	return "reservation/" + requestID, nil
}
func (*countingBudget) ReleaseUnstarted(context.Context, string) error { return nil }

type retrySafeBudget struct {
	mu           sync.Mutex
	reservations map[string]string
	creates      int
}

func (b *retrySafeBudget) Ensure(_ context.Context, tenant uint64, owner, requestID string, _ int64, _ time.Time) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.reservations == nil {
		b.reservations = map[string]string{}
	}
	key := fmt.Sprintf("%d/%s/%s", tenant, owner, requestID)
	if existing, ok := b.reservations[key]; ok {
		return existing, nil
	}
	b.creates++
	ref := "reservation/" + key
	b.reservations[key] = ref
	return ref, nil
}
func (*retrySafeBudget) ReleaseUnstarted(context.Context, string) error { return nil }

func TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation(t *testing.T) {
	budget := &retrySafeBudget{}
	deadline := time.Now().Add(time.Minute)
	first, err := budget.Ensure(context.Background(), 1, "u1", "crash-window", 100, deadline)
	require.NoError(t, err)
	// Model a process crash after the provider committed but before the
	// request row stored reservation_ref: the retry must use the same key.
	second, err := budget.Ensure(context.Background(), 1, "u1", "crash-window", 100, deadline)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, budget.creates)
}

func TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	budget := &countingBudget{}
	var published atomic.Int32
	coordinator := NewAdmissionCoordinator(db, runs, budget, func(context.Context, agentruntime.RunKey) error {
		published.Add(1)
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "platform", RequestID: "same-request", Text: "hello", BudgetUpper: 100}

	var wg sync.WaitGroup
	results := make(chan agentruntime.Run, 20)
	errorsCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, err := coordinator.Start(ctx, in)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- run
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err)
	}
	var runIDs []string
	for run := range results {
		runIDs = append(runIDs, run.Key.RunID)
	}
	require.Len(t, runIDs, 20)
	for _, id := range runIDs {
		require.Equal(t, runIDs[0], id)
	}
	var runCount, userCount, assistantCount int64
	require.NoError(t, db.Table("agent_runs").Count(&runCount).Error)
	require.NoError(t, db.Table("messages").Where("request_id = ? AND role = 'user'", in.RequestID).Count(&userCount).Error)
	require.NoError(t, db.Table("messages").Where("request_id = ? AND role = 'assistant'", in.RequestID).Count(&assistantCount).Error)
	require.EqualValues(t, 1, runCount)
	require.EqualValues(t, 1, userCount)
	require.EqualValues(t, 1, assistantCount)
	require.EqualValues(t, 1, budget.ensures.Load())
	require.EqualValues(t, 1, published.Load())
}

func TestAdmissionPublishFailureIsRetryable(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	budget := &countingBudget{}
	var published atomic.Int32
	coordinator := NewAdmissionCoordinator(db, runs, budget, func(context.Context, agentruntime.RunKey) error {
		if published.Add(1) == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", TargetID: "platform", RequestID: "retry-request", Text: "hello", BudgetUpper: 100}
	_, err := coordinator.Start(ctx, in)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	run, err := coordinator.Start(ctx, in)
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
	require.EqualValues(t, 2, published.Load())
	require.EqualValues(t, 1, budget.ensures.Load())
}

func TestAdmissionDispatchingStateDoesNotRepublish(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	budget := &countingBudget{}
	var published atomic.Int32
	coordinator := NewAdmissionCoordinator(db, runs, budget, func(context.Context, agentruntime.RunKey) error {
		published.Add(1)
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", TargetID: "platform", RequestID: "uncertain-request", Text: "hello", BudgetUpper: 100}
	first, err := coordinator.Start(ctx, in)
	require.NoError(t, err)
	require.EqualValues(t, 1, published.Load())
	require.NoError(t, db.Exec("UPDATE workbench_requests SET state = 'dispatching' WHERE tenant_id = 1 AND actor_id = 'u1' AND request_id = ?", in.RequestID).Error)
	second, err := coordinator.Start(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.Key, second.Key)
	require.EqualValues(t, 1, published.Load(), "uncertain dispatch must be recovered by durable worker scan")
}

func openAdmissionConcurrencyDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../"))
	dsn := "file:" + filepath.Join(t.TempDir(), "admission.db") + "?_foreign_keys=on&_busy_timeout=10000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(20)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO tenants (id,name,business) VALUES (1,'t1','test')").Error)
	require.NoError(t, db.Exec("INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u1','u1','u1@test','x',1)").Error)
	require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s1',1,'s1','u1','trpc')").Error)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
