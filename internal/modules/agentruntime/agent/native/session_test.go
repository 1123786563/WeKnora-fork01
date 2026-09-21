package native

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

type sessionScopeResolver struct {
	scope   nativecontract.Scope
	revoked bool
}

func (r *sessionScopeResolver) Resolve(context.Context) (nativecontract.Scope, error) {
	return r.scope, nil
}
func (r *sessionScopeResolver) Recheck(_ context.Context, got nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	if r.revoked {
		return nativecontract.Scope{}, fmt.Errorf("scope revoked")
	}
	if got.TenantID != r.scope.TenantID || got.SessionOwnerID != r.scope.SessionOwnerID {
		return nativecontract.Scope{}, fmt.Errorf("scope denied")
	}
	return r.scope, nil
}

func newNativeSessionFacade(t *testing.T) (*SessionService, context.Context, session.Key, *sessionScopeResolver, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "session.db")+"?_foreign_keys=on"), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		"CREATE TABLE native_agent_tenants (tenant_id INTEGER PRIMARY KEY)",
		"CREATE TABLE users (id TEXT NOT NULL, tenant_id INTEGER NOT NULL, UNIQUE (tenant_id, id))",
		"CREATE TABLE native_agent_sessions (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL, PRIMARY KEY (tenant_id, owner_id, session_id))",
		"CREATE TABLE native_agent_runs (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL)",
		"CREATE TABLE native_agent_session_events (tenant_id INTEGER NOT NULL, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, stable_event_id TEXT NOT NULL, payload_hash TEXT NOT NULL, payload TEXT NOT NULL, ordinal INTEGER NOT NULL, PRIMARY KEY (tenant_id, app_name, user_id, session_id, stable_event_id), UNIQUE (tenant_id, app_name, user_id, session_id, ordinal))",
		"CREATE TABLE native_session_state (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL, state_key TEXT NOT NULL, state_value TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (tenant_id, owner_id, session_id, state_key))",
		"CREATE TABLE native_user_state (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, state_key TEXT NOT NULL, state_value TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (tenant_id, owner_id, state_key), FOREIGN KEY (tenant_id, owner_id) REFERENCES users (tenant_id, id))",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO users (id, tenant_id) VALUES (?, ?)", "u1", 1).Error)
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "u1"}
	key, err := nativecontract.SessionKey(scope, "s1")
	require.NoError(t, err)
	resolver := &sessionScopeResolver{scope: scope}
	return NewSessionService(repository.NewNativeSessionStore(db), resolver), WithScope(context.Background(), scope), key, resolver, db
}

func TestNativeSessionFacadeCRUDOptionsSummaryUserStateAndDelete(t *testing.T) {
	svc, ctx, key, _, _ := newNativeSessionFacade(t)
	sess, err := svc.CreateSession(ctx, key, session.StateMap{"draft": []byte("one")})
	require.NoError(t, err)
	require.NoError(t, svc.AppendEvent(ctx, sess, &event.Event{ID: "first", Author: "agent", Timestamp: time.Unix(10, 0)}))
	require.NoError(t, svc.AppendEvent(ctx, sess, &event.Event{ID: "second", Author: "agent", Timestamp: time.Unix(20, 0)}))
	require.NoError(t, svc.AppendStable(ctx, nativecontract.SessionAppend{
		Key: key, StableEventID: "conflict", PayloadHash: "first", Event: &event.Event{ID: "conflict", Author: "agent"},
	}))
	err = svc.AppendStable(ctx, nativecontract.SessionAppend{
		Key: key, StableEventID: "conflict", PayloadHash: "changed", Event: &event.Event{ID: "conflict", Author: "agent", Timestamp: time.Unix(1, 0)},
	})
	var conflict *nativecontract.Failure
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, nativecontract.ErrConflict, conflict.Code)

	got, err := svc.GetSession(ctx, key, session.WithEventTime(time.Unix(15, 0)), session.WithEventNum(1))
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
	require.Equal(t, "second", got.Events[0].ID)
	user := session.UserKey{AppName: key.AppName, UserID: key.UserID}
	listed, err := svc.ListSessions(ctx, user, session.WithEventTime(time.Unix(15, 0)), session.WithEventNum(1))
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Len(t, listed[0].Events, 1)
	require.Equal(t, "second", listed[0].Events[0].ID)

	got.Summaries = map[string]*session.Summary{"default": {Summary: "brief"}}
	require.NoError(t, svc.CreateSessionSummary(ctx, got, "default", false))
	text, ok := svc.GetSessionSummaryText(ctx, got, session.WithSummaryFilterKey("default"))
	require.True(t, ok)
	require.Equal(t, "brief", text)

	require.NoError(t, svc.UpdateUserState(ctx, user, session.StateMap{"theme": []byte("dark")}))
	states, err := svc.ListUserStates(ctx, user)
	require.NoError(t, err)
	require.Equal(t, []byte("dark"), states["theme"])
	require.NoError(t, svc.DeleteUserState(ctx, user, "theme"))
	states, err = svc.ListUserStates(ctx, user)
	require.NoError(t, err)
	require.NotContains(t, states, "theme")

	require.NoError(t, svc.DeleteSession(ctx, key))
	_, err = svc.GetSession(ctx, key)
	require.Error(t, err)
}

func TestNativeSessionFacadeRejectsRevokedScope(t *testing.T) {
	svc, ctx, key, resolver, _ := newNativeSessionFacade(t)
	sess, err := svc.CreateSession(ctx, key, nil)
	require.NoError(t, err)
	_, err = svc.GetSession(ctx, key)
	require.NoError(t, err)
	sess.Summaries = map[string]*session.Summary{"default": {Summary: "brief"}}
	require.NoError(t, svc.CreateSessionSummary(ctx, sess, "default", false))

	deletedKey, err := nativecontract.SessionKey(resolver.scope, "deleted")
	require.NoError(t, err)
	_, err = svc.CreateSession(ctx, deletedKey, nil)
	require.NoError(t, err)
	require.NoError(t, svc.DeleteSession(ctx, deletedKey))

	resolver.revoked = true
	for _, err := range []error{
		func() error { _, err := svc.GetSession(ctx, key); return err }(),
		svc.CreateSessionSummary(ctx, sess, "default", true),
		svc.DeleteSession(ctx, deletedKey),
	} {
		var failure *nativecontract.Failure
		require.ErrorAs(t, err, &failure)
		require.Equal(t, nativecontract.ErrForbidden, failure.Code)
	}
}

func TestNativeSessionFacadeRejectsChangedEventWhenCallerReusesHash(t *testing.T) {
	svc, ctx, key, _, _ := newNativeSessionFacade(t)
	_, err := svc.CreateSession(ctx, key, nil)
	require.NoError(t, err)
	first := &event.Event{ID: "stable", Author: "first", Timestamp: time.Unix(10, 0)}
	hash, err := canonicalEventHash(first)
	require.NoError(t, err)
	require.NoError(t, svc.AppendStable(ctx, nativecontract.SessionAppend{Key: key, StableEventID: first.ID, PayloadHash: hash, Event: first}))
	changed := first.Clone()
	changed.ID = first.ID
	changed.Timestamp = time.Unix(11, 0)
	changedHash, err := canonicalEventHash(changed)
	require.NoError(t, err)
	require.NotEqual(t, hash, changedHash)
	err = svc.AppendStable(ctx, nativecontract.SessionAppend{Key: key, StableEventID: changed.ID, PayloadHash: hash, Event: changed})
	var conflict *nativecontract.Failure
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, nativecontract.ErrConflict, conflict.Code)
	got, err := svc.GetSession(ctx, key)
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
	require.Equal(t, "first", got.Events[0].Author)
}

func TestNativeSessionFacadeRejectsStableAppendWithoutMatchingEventIDBeforeWriting(t *testing.T) {
	svc, ctx, key, _, db := newNativeSessionFacade(t)
	_, err := svc.CreateSession(ctx, key, nil)
	require.NoError(t, err)

	for _, append := range []nativecontract.SessionAppend{
		{Key: key, StableEventID: "stable", PayloadHash: "hash", Event: &event.Event{Author: "agent"}},
		{Key: key, StableEventID: "stable", PayloadHash: "hash", Event: &event.Event{ID: "different", Author: "agent"}},
	} {
		var failure *nativecontract.Failure
		require.ErrorAs(t, svc.AppendStable(ctx, append), &failure)
		require.Equal(t, nativecontract.ErrInvalid, failure.Code)
	}

	var rows int64
	require.NoError(t, db.Table("native_agent_session_events").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", 1, key.AppName, key.UserID, key.SessionID).Count(&rows).Error)
	require.Zero(t, rows)
}

func TestNativeSessionFacadeConcurrentSummaryReadAndWrite(t *testing.T) {
	svc, ctx, key, _, _ := newNativeSessionFacade(t)
	sess, err := svc.CreateSession(ctx, key, nil)
	require.NoError(t, err)
	require.NoError(t, svc.AppendEvent(ctx, sess, &event.Event{ID: "event", Author: "agent"}))
	sess.SummariesMu.Lock()
	sess.Summaries = map[string]*session.Summary{"default": {Summary: "brief"}}
	sess.SummariesMu.Unlock()
	require.NoError(t, svc.CreateSessionSummary(ctx, sess, "default", false))

	const attempts = 32
	errs := make(chan error, attempts*2)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- svc.CreateSessionSummary(ctx, sess, "default", false)
		}()
		go func() {
			defer wg.Done()
			text, ok := svc.GetSessionSummaryText(ctx, sess, session.WithSummaryFilterKey("default"))
			if !ok || text != "brief" {
				errs <- fmt.Errorf("summary read = %q, %t", text, ok)
				return
			}
			errs <- nil
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
