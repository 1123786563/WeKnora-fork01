package native

import (
	"context"
	"fmt"
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

type sessionScopeResolver struct{ scope nativecontract.Scope }

func (r sessionScopeResolver) Resolve(context.Context) (nativecontract.Scope, error) {
	return r.scope, nil
}
func (r sessionScopeResolver) Recheck(_ context.Context, got nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	if got.TenantID != r.scope.TenantID || got.SessionOwnerID != r.scope.SessionOwnerID {
		return nativecontract.Scope{}, fmt.Errorf("scope denied")
	}
	return r.scope, nil
}

func newNativeSessionFacade(t *testing.T) (*SessionService, context.Context, session.Key) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		"CREATE TABLE native_agent_tenants (tenant_id INTEGER PRIMARY KEY)",
		"CREATE TABLE native_agent_sessions (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL, PRIMARY KEY (tenant_id, owner_id, session_id))",
		"CREATE TABLE native_agent_runs (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL)",
		"CREATE TABLE native_agent_session_events (tenant_id INTEGER NOT NULL, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, stable_event_id TEXT NOT NULL, payload_hash TEXT NOT NULL, payload TEXT NOT NULL, ordinal INTEGER NOT NULL, PRIMARY KEY (tenant_id, app_name, user_id, session_id, stable_event_id), UNIQUE (tenant_id, app_name, user_id, session_id, ordinal))",
		"CREATE TABLE native_session_state (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL, state_key TEXT NOT NULL, state_value TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (tenant_id, owner_id, session_id, state_key))",
		"CREATE TABLE native_user_state (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, state_key TEXT NOT NULL, state_value TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (tenant_id, owner_id, state_key))",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "u1"}
	key, err := nativecontract.SessionKey(scope, "s1")
	require.NoError(t, err)
	return NewSessionService(repository.NewNativeSessionStore(db), sessionScopeResolver{scope: scope}), WithScope(context.Background(), scope), key
}

func TestSessionFacadeCRUDOptionsSummaryUserStateAndDelete(t *testing.T) {
	svc, ctx, key := newNativeSessionFacade(t)
	sess, err := svc.CreateSession(ctx, key, session.StateMap{"draft": []byte("one")})
	require.NoError(t, err)
	require.NoError(t, svc.AppendEvent(ctx, sess, &event.Event{ID: "first", Author: "agent", Timestamp: time.Unix(10, 0)}))
	require.NoError(t, svc.AppendEvent(ctx, sess, &event.Event{ID: "second", Author: "agent", Timestamp: time.Unix(20, 0)}))

	got, err := svc.GetSession(ctx, key, session.WithEventTime(time.Unix(15, 0)), session.WithEventNum(1))
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
	require.Equal(t, "second", got.Events[0].ID)

	got.Summaries = map[string]*session.Summary{"default": {Summary: "brief"}}
	require.NoError(t, svc.CreateSessionSummary(ctx, got, "default", false))
	text, ok := svc.GetSessionSummaryText(ctx, got, session.WithSummaryFilterKey("default"))
	require.True(t, ok)
	require.Equal(t, "brief", text)

	user := session.UserKey{AppName: key.AppName, UserID: key.UserID}
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
