package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func TestNativeSessionStableEventReopensAndRejectsChangedPayload(t *testing.T) {
	db := openRunTestDB(t)
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
}

func TestNativeSessionScopeIsolation(t *testing.T) {
	db := openRunTestDB(t)
	store := NewNativeSessionStore(db)
	first := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/same", SessionID: "session/same"}
	second := session.Key{AppName: "weknora/native-v1/tenant/2", UserID: "owner/same", SessionID: "session/same"}
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')").Error)
	require.NoError(t, store.Create(context.Background(), first, nil))
	require.NoError(t, store.Create(context.Background(), second, nil))
	list, err := store.List(context.Background(), session.UserKey{AppName: first.AppName, UserID: first.UserID})
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestNativeSessionStateReopensAndConcurrentStableAppendIsIdempotent(t *testing.T) {
	db := openRunTestDB(t)
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
}
