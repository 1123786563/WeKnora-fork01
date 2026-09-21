package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func eachNativeSessionBackend(t *testing.T, test func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			test(t, openNativeSchemaTestDB(t, dialect))
		})
	}
}

func TestNativeSessionStableEventReopensAndRejectsChangedPayload(t *testing.T) {
	eachNativeSessionBackend(t, func(t *testing.T, db *gorm.DB) {
		store := NewNativeSessionStore(db)
		key := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/u1", SessionID: "session/s1"}
		require.NoError(t, store.Create(context.Background(), key, session.StateMap{"x": []byte("one")}))
		append := nativecontract.SessionAppend{Key: key, StableEventID: "evt-1", PayloadHash: "hash-1", Event: &event.Event{ID: "evt-1", Author: "agent"}}
		require.NoError(t, store.AppendStable(context.Background(), append))
		require.NoError(t, store.AppendStable(context.Background(), append))
		got, err := NewNativeSessionStore(reopenRunDB(t, db)).Get(context.Background(), key)
		require.NoError(t, err)
		require.Len(t, got.Events, 1)
		err = store.AppendStable(context.Background(), nativecontract.SessionAppend{Key: key, StableEventID: "evt-1", PayloadHash: "hash-2", Event: &event.Event{ID: "evt-1", Author: "changed"}})
		require.ErrorIs(t, err, ErrNativeSessionConflict)
	})
}

func TestNativeSessionStableAppendRejectsMissingOrMismatchedEventIDBeforeWriting(t *testing.T) {
	eachNativeSessionBackend(t, func(t *testing.T, db *gorm.DB) {
		store := NewNativeSessionStore(db)
		key := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/u1", SessionID: "session/invalid-event-id"}
		require.NoError(t, store.Create(context.Background(), key, nil))

		for _, append := range []nativecontract.SessionAppend{
			{Key: key, StableEventID: "stable", PayloadHash: "hash", Event: &event.Event{Author: "agent"}},
			{Key: key, StableEventID: "stable", PayloadHash: "hash", Event: &event.Event{ID: "different", Author: "agent"}},
		} {
			require.Error(t, store.AppendStable(context.Background(), append))
		}

		var rows int64
		require.NoError(t, db.Table("native_agent_session_events").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", 1, key.AppName, key.UserID, key.SessionID).Count(&rows).Error)
		require.Zero(t, rows)
	})
}

func TestNativeSessionScopeIsolation(t *testing.T) {
	eachNativeSessionBackend(t, func(t *testing.T, db *gorm.DB) {
		store := NewNativeSessionStore(db)
		first := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/same", SessionID: "session/same"}
		second := session.Key{AppName: "weknora/native-v1/tenant/2", UserID: "owner/same", SessionID: "session/same"}
		require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')").Error)
		require.NoError(t, store.Create(context.Background(), first, nil))
		require.NoError(t, store.Create(context.Background(), second, nil))
		list, err := store.List(context.Background(), session.UserKey{AppName: first.AppName, UserID: first.UserID})
		require.NoError(t, err)
		require.Len(t, list, 1)
	})
}

func TestNativeSessionStateReopensAndConcurrentStableAppendIsIdempotent(t *testing.T) {
	eachNativeSessionBackend(t, func(t *testing.T, db *gorm.DB) {
		store := NewNativeSessionStore(db)
		key := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/reopen"}
		require.NoError(t, store.UpdateUserState(context.Background(), session.UserKey{AppName: key.AppName, UserID: key.UserID}, session.StateMap{"theme": []byte("dark")}))
		state, err := NewNativeSessionStore(reopenRunDB(t, db)).UserState(context.Background(), session.UserKey{AppName: key.AppName, UserID: key.UserID})
		require.NoError(t, err)
		require.Equal(t, []byte("dark"), state["theme"])
		require.NoError(t, store.Create(context.Background(), key, nil))
		append := nativecontract.SessionAppend{Key: key, StableEventID: "concurrent", PayloadHash: "same", Event: &event.Event{ID: "concurrent", Author: "agent"}}
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- store.AppendStable(context.Background(), append) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		got, err := store.Get(context.Background(), key)
		require.NoError(t, err)
		require.Len(t, got.Events, 1)
	})
}

func TestNativeSessionConcurrentDistinctStableAppendsKeepEveryEvent(t *testing.T) {
	eachNativeSessionBackend(t, func(t *testing.T, db *gorm.DB) {
		store := NewNativeSessionStore(db)
		key := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/distinct-concurrent"}
		require.NoError(t, store.Create(context.Background(), key, nil))

		const writers = 24
		errs := make(chan error, writers)
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := fmt.Sprintf("event-%02d", i)
				errs <- store.AppendStable(context.Background(), nativecontract.SessionAppend{
					Key: key, StableEventID: id, PayloadHash: "hash-" + id,
					Event: &event.Event{ID: id, Author: "agent"},
				})
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		got, err := store.Get(context.Background(), key)
		require.NoError(t, err)
		require.Len(t, got.Events, writers)
		var distinctOrdinals int64
		require.NoError(t, db.Raw(`SELECT COUNT(DISTINCT ordinal) FROM native_agent_session_events
		WHERE tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?`,
			1, key.AppName, key.UserID, key.SessionID).Scan(&distinctOrdinals).Error)
		require.EqualValues(t, writers, distinctOrdinals)
	})
}
