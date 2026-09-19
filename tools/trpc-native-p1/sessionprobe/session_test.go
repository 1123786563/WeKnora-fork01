package sessionprobe

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	pgsession "trpc.group/trpc-go/trpc-agent-go/session/postgres"
	sqlitesession "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

type closableService interface {
	session.Service
	Close() error
}

// openBackend returns a service plus a durable reopen operation. Each fixture
// owns its database (or PostgreSQL schema) and uses synchronous persistence.
func openBackend(t *testing.T, backend string) (session.Service, func() session.Service) {
	t.Helper()
	var current closableService
	open := func() closableService { t.Helper(); return nil }
	switch backend {
	case "sqlite":
		path := filepath.Join(t.TempDir(), "session.db")
		open = func() closableService {
			db, err := sql.Open("sqlite3", path)
			require.NoError(t, err)
			svc, err := sqlitesession.NewService(db, sqlitesession.WithEnableAsyncPersist(false))
			require.NoError(t, err)
			return svc
		}
	case "postgres":
		dsn := os.Getenv("P1_SESSION_PG_DSN")
		if dsn == "" {
			t.Skip("blocked-env: P1_SESSION_PG_DSN is not set")
		}
		schema := fmt.Sprintf("p1_session_%d", time.Now().UnixNano())
		admin := openPostgresAdmin(t, dsn)
		_, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema)
		require.NoError(t, err)
		createPostgresProbeTables(t, admin, schema)
		t.Cleanup(func() {
			_, err := admin.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
			require.NoError(t, err)
			require.NoError(t, admin.Close())
		})
		open = func() closableService {
			svc, err := pgsession.NewService(
				pgsession.WithPostgresClientDSN(dsn),
				pgsession.WithSchema(schema),
				pgsession.WithEnableAsyncPersist(false),
				pgsession.WithSkipDBInit(true),
			)
			require.NoError(t, err)
			return svc
		}
	default:
		t.Fatalf("unknown backend %q", backend)
	}
	current = open()
	t.Cleanup(func() {
		if current != nil {
			require.NoError(t, current.Close())
		}
	})
	return current, func() session.Service {
		require.NoError(t, current.Close())
		current = open()
		return current
	}
}

func createPostgresProbeTables(t *testing.T, db *sql.DB, schema string) {
	t.Helper()
	for _, table := range []string{
		"session_states (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, state JSONB, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
		"session_events (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, event JSONB NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
		"session_track_events (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, track TEXT NOT NULL, event JSONB NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
		"session_summaries (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, filter_key TEXT NOT NULL DEFAULT '', summary JSONB, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
		"app_states (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, key TEXT NOT NULL, value TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
		"user_states (id BIGSERIAL PRIMARY KEY, app_name TEXT NOT NULL, user_id TEXT NOT NULL, key TEXT NOT NULL, value TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at TIMESTAMP, deleted_at TIMESTAMP)",
	} {
		_, err := db.ExecContext(context.Background(), "CREATE TABLE "+schema+"."+table)
		require.NoError(t, err)
	}
	for _, index := range []string{
		"CREATE UNIQUE INDEX session_states_unique_active ON " + schema + ".session_states(app_name, user_id, session_id) WHERE deleted_at IS NULL",
		"CREATE INDEX session_events_lookup ON " + schema + ".session_events(app_name, user_id, session_id, created_at)",
		"CREATE UNIQUE INDEX session_summaries_unique_active ON " + schema + ".session_summaries(app_name, user_id, session_id, filter_key) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX app_states_unique_active ON " + schema + ".app_states(app_name, key) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX user_states_unique_active ON " + schema + ".user_states(app_name, user_id, key) WHERE deleted_at IS NULL",
	} {
		_, err := db.ExecContext(context.Background(), index)
		require.NoError(t, err)
	}
}

func openPostgresAdmin(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(context.Background()))
	return db
}

func eachBackend(t *testing.T, fn func(t *testing.T, backend string, svc session.Service, reopen func() session.Service)) {
	t.Helper()
	for _, backend := range []string{"sqlite", "postgres"} {
		t.Run(backend, func(t *testing.T) { svc, reopen := openBackend(t, backend); fn(t, backend, svc, reopen) })
	}
}

func scopeKey(tenant string, suffix string) session.Key {
	return session.Key{AppName: "weknora/native-v1/tenant/" + tenant, UserID: "owner/dQ", SessionID: "session/" + suffix}
}

func validEvent(content string) *event.Event {
	return event.NewResponseEvent("inv-1", "assistant", &model.Response{
		ID: "response-" + content, Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: content}}},
	})
}

func bootstrapUserEvent() *event.Event {
	return event.NewResponseEvent("inv-1", "user", &model.Response{
		ID: "event-user", Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleUser, Content: "start"}}},
	})
}

func assistantEventCount(events []event.Event) int {
	count := 0
	for _, e := range events {
		if e.Response != nil && len(e.Response.Choices) > 0 && e.Response.Choices[0].Message.Role == model.RoleAssistant {
			count++
		}
	}
	return count
}

func sessionIDs(sessions []*session.Session) []string {
	ids := make([]string, len(sessions))
	for i, sess := range sessions {
		ids[i] = sess.ID
	}
	return ids
}

func sessionByID(t *testing.T, sessions []*session.Session, id string) *session.Session {
	t.Helper()
	for _, sess := range sessions {
		if sess.ID == id {
			return sess
		}
	}
	t.Fatalf("session %q not found", id)
	return nil
}

func TestSessionPersistenceAndIsolation(t *testing.T) {
	eachBackend(t, func(t *testing.T, _ string, svc session.Service, reopen func() session.Service) {
		ctx := context.Background()
		a, b := scopeKey("1", "cw"), scopeKey("2", "cw")
		_, err := svc.CreateSession(ctx, a, session.StateMap{"scope": []byte("a")})
		require.NoError(t, err)
		_, err = svc.CreateSession(ctx, b, session.StateMap{"scope": []byte("b")})
		require.NoError(t, err)
		svc = reopen()
		got, err := svc.GetSession(ctx, b)
		require.NoError(t, err)
		require.Equal(t, []byte("b"), got.State["scope"])
		require.NoError(t, svc.UpdateSessionState(ctx, b, session.StateMap{"changed": []byte("yes")}))
		require.NoError(t, svc.DeleteSession(ctx, a))
		got, err = svc.GetSession(ctx, b)
		require.NoError(t, err)
		require.Equal(t, []byte("yes"), got.State["changed"])
	})
}

func TestPostgresDefaultSchemaInitializationIsIncompatible(t *testing.T) {
	dsn := os.Getenv("P1_SESSION_PG_DSN")
	if dsn == "" {
		t.Skip("blocked-env: P1_SESSION_PG_DSN is not set")
	}
	admin := openPostgresAdmin(t, dsn)
	defer admin.Close()
	schema := fmt.Sprintf("p1_session_init_%d", time.Now().UnixNano())
	_, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") }()
	svc, err := pgsession.NewService(pgsession.WithPostgresClientDSN(dsn), pgsession.WithSchema(schema), pgsession.WithEnableAsyncPersist(false))
	if svc != nil {
		_ = svc.Close()
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "required unique index")
}

func TestSessionListPagingAndCloseOwnership(t *testing.T) {
	eachBackend(t, func(t *testing.T, _ string, svc session.Service, reopen func() session.Service) {
		ctx := context.Background()
		user := session.UserKey{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dQ"}
		require.NoError(t, svc.UpdateAppState(ctx, user.AppName, session.StateMap{session.StateAppPrefix + "shared": []byte("app")}))
		require.NoError(t, svc.UpdateUserState(ctx, user, session.StateMap{session.StateUserPrefix + "shared": []byte("user")}))
		for _, id := range []string{"one", "two", "three"} {
			_, err := svc.CreateSession(ctx, session.Key{AppName: user.AppName, UserID: user.UserID, SessionID: id}, session.StateMap{"session": []byte(id)})
			require.NoError(t, err)
			time.Sleep(time.Millisecond)
		}
		one, err := svc.GetSession(ctx, session.Key{AppName: user.AppName, UserID: user.UserID, SessionID: "one"})
		require.NoError(t, err)
		require.NoError(t, svc.AppendEvent(ctx, one, bootstrapUserEvent()))
		require.NoError(t, svc.AppendEvent(ctx, one, validEvent("listed")))
		listed, err := svc.ListSessions(ctx, user)
		require.NoError(t, err)
		require.Equal(t, []string{"one", "three", "two"}, sessionIDs(listed), "list is updated_at DESC")
		listedOne := sessionByID(t, listed, "one")
		require.Equal(t, []byte("app"), listedOne.State[session.StateAppPrefix+"shared"])
		require.Equal(t, []byte("user"), listedOne.State[session.StateUserPrefix+"shared"])
		require.Equal(t, []byte("one"), listedOne.State["session"])
		require.Len(t, listedOne.Events, 2, "normal ListSessions includes persisted event history")
		require.Equal(t, 1, assistantEventCount(listedOne.Events))
		listed, err = svc.ListSessions(ctx, user, session.WithListSessionPage(0, 1), session.WithListSessionOnlyMeta())
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, "one", listed[0].ID)
		require.Empty(t, listed[0].Events, "OnlyMeta omits persisted event history")
		listed, err = svc.ListSessions(ctx, user, session.WithListSessionPage(10, 1))
		require.NoError(t, err)
		require.Empty(t, listed)
		_, err = svc.ListSessions(ctx, user, session.WithListSessionPage(-1, 1))
		require.ErrorIs(t, err, session.ErrInvalidListSessionPage)
		_, err = svc.ListSessions(ctx, user, session.WithListSessionPage(0, 0))
		require.ErrorIs(t, err, session.ErrInvalidListSessionPage)
		closed := svc.(closableService)
		require.NoError(t, closed.Close())
		_, err = svc.ListSessions(ctx, user)
		require.Error(t, err, "Close must close the service-owned connection")
		svc = reopen()
		listed, err = svc.ListSessions(ctx, user)
		require.NoError(t, err)
		require.Len(t, listed, 3)
	})
}

func TestSessionAppendCharacterization(t *testing.T) {
	eachBackend(t, func(t *testing.T, backend string, svc session.Service, reopen func() session.Service) {
		ctx := context.Background()
		key := scopeKey("1", "events")
		sess, err := svc.CreateSession(ctx, key, nil)
		require.NoError(t, err)
		require.NoError(t, svc.AppendEvent(ctx, sess, bootstrapUserEvent()))
		first := validEvent("one")
		first.ID = "event-1"
		require.Equal(t, "event-1", first.ID)
		require.NoError(t, svc.AppendEvent(ctx, sess, first))
		require.NoError(t, svc.AppendEvent(ctx, sess, first))
		require.Equal(t, 2, assistantEventCount(sess.Events), "current Session object records duplicate assistant event IDs")
		svc = reopen()
		got, err := svc.GetSession(ctx, key)
		require.NoError(t, err)
		require.Equal(t, 2, assistantEventCount(got.Events), "native backend stores duplicate event IDs")
		changed := first.Clone()
		changed.ID = first.ID
		changed.Timestamp = first.Timestamp
		changed.Response.Choices[0].Message.Content = "changed"
		require.Equal(t, first.ID, changed.ID)
		require.Equal(t, first.Timestamp, changed.Timestamp)
		require.NoError(t, svc.AppendEvent(ctx, got, changed))
		svc = reopen()
		got, err = svc.GetSession(ctx, key)
		require.NoError(t, err)
		require.Equal(t, 3, assistantEventCount(got.Events))
		if backend == "sqlite" {
			_, err = svc.GetSession(ctx, key, session.WithGetSessionEventPage(0, 1))
			require.ErrorIs(t, err, session.ErrEventPageUnsupported)
		} else {
			page, err := svc.GetSession(ctx, key, session.WithGetSessionEventPage(0, 1))
			require.NoError(t, err)
			require.Len(t, page.Events, 1)
		}
	})
}

func TestSessionAppendCancelledContextReturnsPersistenceError(t *testing.T) {
	eachBackend(t, func(t *testing.T, _ string, svc session.Service, reopen func() session.Service) {
		ctx := context.Background()
		sess, err := svc.CreateSession(ctx, scopeKey("1", "cancelled"), nil)
		require.NoError(t, err)
		bootstrap := bootstrapUserEvent()
		require.NoError(t, svc.AppendEvent(ctx, sess, bootstrap))
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		failed := validEvent("will not persist")
		failed.ID = "event-cancelled"
		err = svc.AppendEvent(cancelled, sess, failed)
		require.Error(t, err, "synchronous append must expose its persistence failure")
		require.Equal(t, 1, assistantEventCount(sess.Events), "characterization: in-memory Session is updated before failed persistence")
		svc = reopen()
		got, err := svc.GetSession(ctx, scopeKey("1", "cancelled"))
		require.NoError(t, err)
		require.Len(t, got.Events, 1, "reopen retains only the durable bootstrap event")
		require.Equal(t, bootstrap.ID, got.Events[0].ID)
		require.Equal(t, 0, assistantEventCount(got.Events), "cancelled assistant append was not persisted")
	})
}

func TestSessionContractReplayRejection(t *testing.T) {
	if os.Getenv("P1_REQUIRE_CONTRACT") != "1" {
		t.Skip("contract acceptance is opt-in; native v1.11.0 characterization is intentionally green")
	}
	eachBackend(t, func(t *testing.T, _ string, svc session.Service, reopen func() session.Service) {
		ctx := context.Background()
		key := scopeKey("1", "contract")
		sess, err := svc.CreateSession(ctx, key, nil)
		require.NoError(t, err)
		require.NoError(t, svc.AppendEvent(ctx, sess, bootstrapUserEvent()))
		first := validEvent("one")
		first.ID = "event-1"
		require.NoError(t, svc.AppendEvent(ctx, sess, first))
		// Required P1 contract: a changed replay under the same stable ID must conflict.
		changed := first.Clone()
		changed.ID = first.ID
		changed.Timestamp = first.Timestamp
		changed.Response.Choices[0].Message.Content = "changed"
		require.Error(t, svc.AppendEvent(ctx, sess, changed))
		svc = reopen()
		got, err := svc.GetSession(ctx, key)
		require.NoError(t, err)
		require.Equal(t, 1, assistantEventCount(got.Events))
	})
}
