package memoryprobe

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	pgmemory "trpc.group/trpc-go/trpc-agent-go/memory/postgres"
	sqlitememory "trpc.group/trpc-go/trpc-agent-go/memory/sqlite"
)

// openMemoryBackend opens a fresh, isolated backend and returns a function
// which closes its service before reopening exactly the same durable store.
func openMemoryBackend(t *testing.T, backend string) (memory.Service, func() memory.Service) {
	t.Helper()
	digest := sha256.Sum256([]byte(t.Name()))
	table := fmt.Sprintf("p1m_%x", digest[:8])
	switch backend {
	case "sqlite":
		path := filepath.Join(t.TempDir(), "memory.db")
		var current memory.Service
		open := func() memory.Service {
			db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", path))
			require.NoError(t, err)
			svc, err := sqlitememory.NewService(db, sqlitememory.WithTableName(table))
			require.NoError(t, err)
			current = svc
			return svc
		}
		svc := open()
		t.Cleanup(func() { _ = current.Close() })
		return svc, func() memory.Service {
			require.NoError(t, current.Close())
			return open()
		}
	case "postgres":
		dsn := os.Getenv("P1_MEMORY_PG_DSN")
		if dsn == "" {
			t.Skip("blocked-env: set P1_MEMORY_PG_DSN for PostgreSQL memory probe")
		}
		schema := fmt.Sprintf("p1m_%x_%x", digest[:6], time.Now().UnixNano())
		config, err := pgx.ParseConfig(dsn)
		require.NoError(t, err)
		db := sql.OpenDB(stdlib.GetConnector(*config))
		require.NoError(t, db.PingContext(context.Background()))
		_, err = db.Exec(`CREATE SCHEMA "` + schema + `"`)
		require.NoError(t, err)
		var current memory.Service
		open := func() memory.Service {
			svc, err := pgmemory.NewService(
				pgmemory.WithPostgresClientDSN(dsn),
				pgmemory.WithSchema(schema),
				pgmemory.WithTableName(table),
			)
			require.NoError(t, err)
			current = svc
			return svc
		}
		svc := open()
		t.Cleanup(func() {
			if err := current.Close(); err != nil {
				t.Errorf("close PostgreSQL memory service: %v", err)
			}
			if _, err := db.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`); err != nil {
				t.Errorf("drop isolated PostgreSQL schema %q: %v", schema, err)
			}
			if err := db.Close(); err != nil {
				t.Errorf("close PostgreSQL cleanup connection: %v", err)
			}
		})
		return svc, func() memory.Service {
			require.NoError(t, current.Close())
			return open()
		}
	default:
		t.Fatalf("unknown backend %q", backend)
		return nil, nil
	}
}

func eachBackend(t *testing.T, fn func(t *testing.T, svc memory.Service, reopen func() memory.Service)) {
	t.Helper()
	for _, backend := range []string{"sqlite", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			svc, reopen := openMemoryBackend(t, backend)
			fn(t, svc, reopen)
		})
	}
}

func keys() (memory.UserKey, memory.UserKey) {
	a := memory.UserKey{AppName: "weknora/native-v1/tenant/1", UserID: "subject/dQ"}
	b := a
	b.AppName = "weknora/native-v1/tenant/2"
	return a, b
}

func TestMemoryRestartIsolationAndClear(t *testing.T) {
	eachBackend(t, func(t *testing.T, svc memory.Service, reopen func() memory.Service) {
		ctx := context.Background()
		a, b := keys()
		require.NoError(t, svc.AddMemory(ctx, a, "prefers tea", []string{"drink"}))
		require.NoError(t, svc.AddMemory(ctx, b, "prefers coffee", []string{"drink"}))
		svc = reopen()
		require.NoError(t, svc.ClearMemories(ctx, a))
		got, err := svc.ReadMemories(ctx, b, 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "prefers coffee", got[0].Memory.Memory)
	})
}

func TestMemoryIdempotencyMetadataUpdateDeleteAndClear(t *testing.T) {
	eachBackend(t, func(t *testing.T, svc memory.Service, reopen func() memory.Service) {
		ctx := context.Background()
		a, _ := keys()
		metadata := &memory.Metadata{Kind: memory.KindFact, Participants: []string{"u"}, Location: "test"}
		require.NoError(t, svc.AddMemory(ctx, a, "prefers tea", []string{"drink"}, memory.WithMetadata(metadata)))
		require.NoError(t, svc.AddMemory(ctx, a, "prefers tea", []string{"drink"}, memory.WithMetadata(metadata)))
		svc = reopen()
		got, err := svc.ReadMemories(ctx, a, 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, memory.KindFact, got[0].Memory.Kind)
		require.Equal(t, []string{"drink"}, got[0].Memory.Topics)
		require.Equal(t, []string{"u"}, got[0].Memory.Participants)
		require.Equal(t, "test", got[0].Memory.Location)

		result := &memory.UpdateResult{}
		require.NoError(t, svc.UpdateMemory(ctx, memory.Key{AppName: a.AppName, UserID: a.UserID, MemoryID: got[0].ID}, "prefers green tea", []string{"drink", "tea"}, memory.WithUpdateResult(result)))
		require.NotEmpty(t, result.MemoryID)
		svc = reopen()
		got, err = svc.ReadMemories(ctx, a, 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "prefers green tea", got[0].Memory.Memory)
		require.Equal(t, []string{"drink", "tea"}, got[0].Memory.Topics)

		require.NoError(t, svc.DeleteMemory(ctx, memory.Key{AppName: a.AppName, UserID: a.UserID, MemoryID: got[0].ID}))
		got, err = svc.ReadMemories(ctx, a, 10)
		require.NoError(t, err)
		require.Empty(t, got)
		require.NoError(t, svc.AddMemory(ctx, a, "prefers coffee", []string{"drink"}))
		require.NoError(t, svc.ClearMemories(ctx, a))
		got, err = svc.ReadMemories(ctx, a, 10)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// Native memory.Service accepts a raw AddMemory after deletion or clearing;
// it has no generation/tombstone parameter through which a delayed extractor
// could be rejected at commit time. This characterization stays green. The
// opt-in acceptance test below deliberately fails until an extension exists.
func TestMemoryStaleExtractionCanResurrectCharacterization(t *testing.T) {
	eachBackend(t, func(t *testing.T, svc memory.Service, reopen func() memory.Service) {
		ctx := context.Background()
		a, _ := keys()
		const stale = "prefers tea"
		for _, invalidation := range []struct {
			name string
			do   func(*memory.Entry) error
		}{
			{"delete", func(entry *memory.Entry) error {
				return svc.DeleteMemory(ctx, memory.Key{AppName: a.AppName, UserID: a.UserID, MemoryID: entry.ID})
			}},
			{"clear", func(_ *memory.Entry) error { return svc.ClearMemories(ctx, a) }},
		} {
			t.Run(invalidation.name, func(t *testing.T) {
				require.NoError(t, svc.AddMemory(ctx, a, stale, []string{"drink"}))
				entries, err := svc.ReadMemories(ctx, a, 1)
				require.NoError(t, err)
				require.Len(t, entries, 1)
				require.NoError(t, invalidation.do(entries[0]))
				require.NoError(t, svc.AddMemory(ctx, a, stale, []string{"drink"}), "raw service permits stale extraction re-add")
				svc = reopen()
				got, err := svc.ReadMemories(ctx, a, 10)
				require.NoError(t, err)
				require.Len(t, got, 1, "raw service resurrects invalidated content")
				require.NoError(t, svc.ClearMemories(ctx, a))
			})
		}
	})
}

func TestMemoryContractRejectsStaleExtractionAfterClear(t *testing.T) {
	if os.Getenv("P1_REQUIRE_CONTRACT") != "1" {
		t.Skip("set P1_REQUIRE_CONTRACT=1 to run known-unmet generation contract")
	}
	eachBackend(t, func(t *testing.T, svc memory.Service, _ func() memory.Service) {
		ctx := context.Background()
		a, _ := keys()
		require.NoError(t, svc.AddMemory(ctx, a, "prefers tea", []string{"drink"}))
		require.NoError(t, svc.ClearMemories(ctx, a))
		err := svc.AddMemory(ctx, a, "prefers tea", []string{"drink"})
		require.Error(t, err, "contract requires a generation/tombstone CAS that native AddMemory does not expose")
	})
}

func TestMemoryConcurrentIdempotentAdd(t *testing.T) {
	eachBackend(t, func(t *testing.T, svc memory.Service, _ func() memory.Service) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a, _ := keys()
		var wg sync.WaitGroup
		errs := make(chan error, 20)
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- svc.AddMemory(ctx, a, "prefers tea", []string{"drink"})
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		got, err := svc.ReadMemories(ctx, a, 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})
}

func TestMemoryCloseOwnsSQLiteDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "owned.db"))
	require.NoError(t, err)
	svc, err := sqlitememory.NewService(db, sqlitememory.WithTableName("memories_probe"))
	require.NoError(t, err)
	require.NoError(t, svc.Close())
	require.Error(t, db.PingContext(context.Background()), "sqlite service documents that Close owns the supplied db")
}
