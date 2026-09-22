package connectorcontrol

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	repoapp "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The double-claim acceptance MUST run on real PostgreSQL: only there does
// the claim's FOR UPDATE SKIP LOCKED subselect have its true meaning (sqlite
// cannot honor SKIP LOCKED — that is exactly why these tests are gated).
//
// Bring up the disposable container (coordinator ruling R8; NEVER touch the
// shared dev PG on 5432):
//
//	docker run -d --name weknora-oc-t05-pg -e POSTGRES_PASSWORD=oc_test //	  -e POSTGRES_DB=oc_test -p <free-port>:5432 postgres:16-alpine
//	docker exec -i weknora-oc-t05-pg psql -U postgres -d oc_test //	  < migrations/versioned/000117_app_installations.up.sql
//	docker exec -i weknora-oc-t05-pg psql -U postgres -d oc_test //	  < migrations/versioned/000121_open_connector_bindings.up.sql
//	OC_T05_PG_DSN='host=127.0.0.1 port=<free-port> user=postgres //	  password=oc_test dbname=oc_test sslmode=disable' //	  go test ./internal/connectorcontrol -run TestWorkerPG -count=1 -v
//
// and afterwards: docker stop weknora-oc-t05-pg && docker rm weknora-oc-t05-pg
func pgDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("OC_T05_PG_DSN")
	if dsn == "" {
		t.Skip("OC_T05_PG_DSN not set: run with the disposable postgres container (R8)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Exec("DELETE FROM connector_operations_outbox").Error; err != nil {
		t.Fatalf("clean outbox: %v", err)
	}
	return db
}

func pgSeedOp(t *testing.T, db *gorm.DB, id string, attempts int64, due time.Time) {
	t.Helper()
	if err := db.Exec("INSERT INTO connector_operations_outbox (id, tenant_id, resource_id, resource_version, kind, attempts, next_at) VALUES (?,?,?,?,?,?,?)",
		id, 7, "conn1", 1, KindDeleteToken, attempts, due).Error; err != nil {
		t.Fatal(err)
	}
}

func pgRemaining(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_operations_outbox").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// TestWorkerPGConcurrentClaimHasSingleWinner: two workers race one claimable
// operation inside real transactions — SKIP LOCKED must hand it to exactly
// one of them.
func TestWorkerPGConcurrentClaimHasSingleWinner(t *testing.T) {
	db := pgDB(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	pgSeedOp(t, db, "pg-op-1", 0, now.Add(-time.Minute))
	store := repoapp.NewOCStore(db)

	start := make(chan struct{})
	var winners sync.WaitGroup
	var claimed int32
	var mu sync.Mutex
	var winnerOwner string
	for _, owner := range []string{"pgA", "pgB"} {
		winners.Add(1)
		go func(owner string) {
			defer winners.Done()
			<-start
			row, err := store.ClaimNextOperation(ctx, owner, now, 30*time.Second)
			if err != nil {
				t.Errorf("claim by %s: %v", owner, err)
				return
			}
			if row != nil {
				atomic.AddInt32(&claimed, 1)
				mu.Lock()
				winnerOwner = owner
				mu.Unlock()
			}
		}(owner)
	}
	close(start)
	winners.Wait()

	if claimed != 1 {
		t.Fatalf("exactly one worker must win the claim, %d did", claimed)
	}
	ok, err := store.CompleteOperation(ctx, "pg-op-1", winnerOwner, 1)
	if err != nil || !ok {
		t.Fatalf("winner complete failed: ok=%v err=%v", ok, err)
	}
	if n := pgRemaining(t, db); n != 0 {
		t.Fatalf("outbox not empty: %d", n)
	}
}

// TestWorkerPGExpiredLeaseTakeoverOldCompleteAffectsZeroRows: after a lease
// expires the operation is re-claimed at fence+1 and the OLD worker's
// Complete updates ZERO rows on real PostgreSQL.
func TestWorkerPGExpiredLeaseTakeoverOldCompleteAffectsZeroRows(t *testing.T) {
	db := pgDB(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	pgSeedOp(t, db, "pg-op-2", 0, now.Add(-time.Minute))
	store := repoapp.NewOCStore(db)

	a, err := store.ClaimNextOperation(ctx, "pgA", now, 30*time.Second)
	if err != nil || a == nil || a.Fence != 1 {
		t.Fatalf("first claim: %+v %v", a, err)
	}
	b, err := store.ClaimNextOperation(ctx, "pgB", now.Add(31*time.Second), 30*time.Second)
	if err != nil || b == nil || b.Fence != 2 || b.Attempts != 2 {
		t.Fatalf("takeover claim: %+v %v", b, err)
	}
	okA, err := store.CompleteOperation(ctx, "pg-op-2", "pgA", 1)
	if err != nil {
		t.Fatal(err)
	}
	if okA {
		t.Fatal("old worker's complete affected rows after takeover")
	}
	var still int64
	db.Raw("SELECT COUNT(*) FROM connector_operations_outbox WHERE id = 'pg-op-2'").Scan(&still)
	if still != 1 {
		t.Fatal("old worker's complete must not delete the row")
	}
	okB, err := store.CompleteOperation(ctx, "pg-op-2", "pgB", 2)
	if err != nil || !okB {
		t.Fatalf("new owner complete: ok=%v err=%v", okB, err)
	}
}

// TestWorkerPGTwoWorkersDrainQueueExactlyOnce: the full two-worker
// competition — both workers RunOnce-loop over one shared real-PostgreSQL
// outbox; every operation must be claimed and completed EXACTLY once.
func TestWorkerPGTwoWorkersDrainQueueExactlyOnce(t *testing.T) {
	db := pgDB(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	const n = 20
	for i := 0; i < n; i++ {
		pgSeedOp(t, db, fmt.Sprintf("pg-drain-%02d", i), 0, now.Add(-time.Minute))
	}
	admin := newFakeAdmin()
	rec := newRecordingOutbox(repoapp.NewOCStore(db))
	workers := []*ControlWorker{
		newTestWorker("pgA", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("pg-admin-secret")),
		newTestWorker("pgB", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("pg-admin-secret")),
	}

	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(w *ControlWorker) {
			defer wg.Done()
			for pgRemaining(t, db) > 0 {
				if err := w.RunOnce(ctx); err != nil {
					t.Errorf("worker tick: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()

	if left := pgRemaining(t, db); left != 0 {
		t.Fatalf("queue not drained: %d left", left)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.claims) != n {
		t.Fatalf("distinct claimed operations = %d, want %d", len(rec.claims), n)
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("pg-drain-%02d", i)
		if rec.claims[id] != 1 {
			t.Fatalf("operation %s claimed %d times (want exactly 1)", id, rec.claims[id])
		}
		if rec.complete[id] != 1 {
			t.Fatalf("operation %s completed %d times (want exactly 1)", id, rec.complete[id])
		}
	}
}
