//go:build darwin || linux

package career

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// acquireLifecycleExecutionGuard excludes another executor for the same
// request until the external effect settles. The guard is OS/session owned, so
// process death releases it without a time-based lease.
func acquireLifecycleExecutionGuard(ctx context.Context, db *gorm.DB, scope Scope, operation, requestID string) (func(), error) {
	guardCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%s", scope.TenantID, scope.UserID, operation, requestID)
	if db.Dialector.Name() == "postgres" {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}
		conn, err := sqlDB.Conn(guardCtx)
		if err != nil {
			return nil, err
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(key))
		lockID := int64(h.Sum64())
		if _, err = conn.ExecContext(guardCtx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
			_ = conn.Close()
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, ErrCareerOperationsBusy
			}
			return nil, err
		}
		return func() {
			_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", lockID)
			_ = conn.Close()
		}, nil
	}
	if db.Dialector.Name() != "sqlite" {
		return nil, fmt.Errorf("lifecycle execution guard unsupported for %s", db.Dialector.Name())
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	var databasePath string
	if err = sqlDB.QueryRowContext(ctx, "PRAGMA database_list").Scan(new(int), new(string), &databasePath); err != nil {
		return nil, err
	}
	if databasePath == "" { // In-memory SQLite is single-process by definition.
		unlock, guardErr := acquireMemoryLifecycleGuard(guardCtx, key)
		if errors.Is(guardErr, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, ErrCareerOperationsBusy
		}
		return unlock, guardErr
	}
	canonical, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	lockPath := canonical + ".career-lifecycle-locks"
	if err = os.MkdirAll(lockPath, 0o700); err != nil {
		return nil, err
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	f, err := os.OpenFile(filepath.Join(lockPath, fmt.Sprintf("%016x.lock", h.Sum64())), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		if err = syscallFlock(f, true); err == nil {
			return func() { _ = syscallFlock(f, false); _ = f.Close() }, nil
		}
		if !errors.Is(err, errLifecycleGuardBusy) {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-guardCtx.Done():
			_ = f.Close()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, ErrCareerOperationsBusy
		case <-time.After(10 * time.Millisecond):
		}
	}
}

var memoryLifecycleGuards = newLifecycleGuardSet()

func acquireMemoryLifecycleGuard(ctx context.Context, key string) (func(), error) {
	return memoryLifecycleGuards.acquire(ctx, key)
}
