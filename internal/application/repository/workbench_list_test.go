package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedWorkbenchListFixtures extends the shared run fixtures with a second
// owner in tenant 1 and a full tenant 2, so ownership and tenant isolation can
// be asserted against realistic neighbors (including a same-named agent).
func seedWorkbenchListFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('u2', 'u2', 'u2@example.test', 'x', 1)`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('s3', 1, 'session-3', 'u2', 'trpc')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('v1', 'v1', 'v1@example.test', 'x', 2)`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('t1', 2, 'session-t1', 'v1', 'trpc')`,
	).Error)
}

type workbenchRunSeed struct {
	tenant  uint64
	owner   string
	runID   string
	session string
	status  string
	agent   string
	target  string
	space   string
	at      time.Time
}

func insertWorkbenchRun(t *testing.T, db *gorm.DB, seed workbenchRunSeed) {
	t.Helper()
	snapshot := map[string]any{"session_id": seed.session, "agent_id": seed.agent, "target_id": seed.target, "workspace_ref": "ws-" + seed.runID}
	if seed.space != "" {
		snapshot["space_id"] = seed.space
	}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id,
		   request_hash, snapshot, deadline, created_at, updated_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		seed.tenant, seed.runID, seed.session, seed.owner, "q-"+seed.runID, "a-"+seed.runID,
		"h-"+seed.runID, string(raw), seed.at.Add(time.Hour), seed.at, seed.at, seed.status,
	).Error)
}

// TestWorkbenchListScopesToAuthenticatedOwnerAndTenant covers the acceptance
// matrix: two owners inside one tenant, plus a same-named agent in a different
// tenant. Every list call must only ever hit the caller's own rows.
func TestWorkbenchListScopesToAuthenticatedOwnerAndTenant(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a1", session: "s1", status: "running", agent: "agent-x", target: "platform", space: "space-1", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a2", session: "s2", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u2", runID: "r-b1", session: "s3", status: "running", agent: "agent-x", target: "platform", at: base.Add(2 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 2, owner: "v1", runID: "r-c1", session: "t1", status: "running", agent: "agent-x", target: "platform", at: base.Add(3 * time.Second)})

	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"r-a2", "r-a1"}, runIDs(page))

	page, err = store.ListOwnedExecutions(ctx, 1, "u2", WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"r-b1"}, runIDs(page))

	page, err = store.ListOwnedExecutions(ctx, 2, "v1", WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"r-c1"}, runIDs(page))

	// The agent filter selects by the run snapshot agent, never by an
	// unscoped name: the same-named agent in tenant 2 must stay invisible.
	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{AgentID: "agent-x"})
	require.NoError(t, err)
	require.Equal(t, []string{"r-a1"}, runIDs(page))

	page, err = store.ListOwnedExecutions(ctx, 1, "u2", WorkbenchExecutionFilter{AgentID: "agent-x"})
	require.NoError(t, err)
	require.Equal(t, []string{"r-b1"}, runIDs(page))

	_, err = store.ListOwnedExecutions(ctx, 1, "missing", WorkbenchExecutionFilter{})
	require.NoError(t, err)
}

// TestWorkbenchListSummaryProjectsSnapshotMetadata pins the row shape: status
// and timestamps come from agent_runs, the navigation metadata from the
// immutable snapshot.
func TestWorkbenchListSummaryProjectsSnapshotMetadata(t *testing.T) {
	db := openRunTestDB(t)
	base := time.Date(2026, 9, 12, 9, 30, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-nav", session: "s1", status: "waiting_user", agent: "agent-nav", target: "platform", space: "space-nav", at: base})

	page, err := NewWorkbenchListStore(db).ListOwnedExecutions(context.Background(), 1, "u1", WorkbenchExecutionFilter{Status: "waiting_user"})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	item := page.Items[0]
	require.Equal(t, "r-nav", item.RunID)
	require.Equal(t, "s1", item.SessionID)
	require.Equal(t, "waiting_user", item.Status)
	require.Equal(t, "agent-nav", item.AgentID)
	require.Equal(t, "platform", item.TargetID)
	require.Equal(t, "ws-r-nav", item.WorkspaceRef)
	require.Equal(t, "space-nav", item.SpaceID)
	require.Equal(t, base.UTC().Format(time.RFC3339Nano), item.CreatedAt)
	require.Empty(t, page.NextCursor)

	// Status filtering is exact: unknown statuses are rejected outright.
	_, err = NewWorkbenchListStore(db).ListOwnedExecutions(context.Background(), 1, "u1", WorkbenchExecutionFilter{Status: "jogging"})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
}

// TestWorkbenchListPaginationStableUnderConcurrentInserts walks the keyset
// pages to exhaustion while a second connection keeps inserting newer runs:
// no row may repeat and none of the already-paged rows may be dropped.
func TestWorkbenchListPaginationStableUnderConcurrentInserts(t *testing.T) {
	db := openRunTestDB(t)
	base := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	const total = 45
	for i := 0; i < total; i++ {
		insertWorkbenchRun(t, db, workbenchRunSeed{
			tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%03d", i), session: "s1",
			status: "running", agent: "agent-x", target: "platform",
			at: base.Add(time.Duration(i) * 10 * time.Millisecond),
		})
	}
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	seen := make([]string, 0, total)
	cursor := ""
	pages := 0
	for {
		page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Limit: 20, Cursor: cursor})
		require.NoError(t, err)
		seen = append(seen, runIDs(page)...)
		pages++
		if pages == 1 {
			// Simulate concurrent writers on a separate connection: five
			// runs newer than the whole original window.
			writer := reopenRunDB(t, db)
			for i := total; i < total+5; i++ {
				insertWorkbenchRun(t, writer, workbenchRunSeed{
					tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%03d", i), session: "s2",
					status: "running", agent: "agent-x", target: "platform",
					at: base.Add(time.Duration(i) * 10 * time.Millisecond),
				})
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	require.Equal(t, 3, pages)
	require.Len(t, seen, total)
	requireNoDuplicateRunIDs(t, seen)
	// Keyset order is newest-first; the concurrent rows above the window must
	// not have shifted it.
	require.Equal(t, "r-044", seen[0])

	// A fresh walk now observes the concurrent inserts, again without overlap.
	fresh, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, fresh.Items, total+5)
	ids := runIDs(fresh)
	requireNoDuplicateRunIDs(t, ids)
	require.Equal(t, "r-049", ids[0])
}

// TestWorkbenchListRejectsForgedOrCrossFilterCursors fails closed on every
// malformed, foreign or stale cursor instead of restarting the list.
func TestWorkbenchListRejectsForgedOrCrossFilterCursors(t *testing.T) {
	db := openRunTestDB(t)
	base := time.Date(2026, 9, 12, 7, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		insertWorkbenchRun(t, db, workbenchRunSeed{
			tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%d", i), session: "s1",
			status: "running", agent: "agent-x", target: "platform", at: base.Add(time.Duration(i) * time.Second),
		})
	}
	store := NewWorkbenchListStore(db)
	ctx := context.Background()
	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{AgentID: "agent-x", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"r-2", "r-1"}, runIDs(page))
	require.NotEmpty(t, page.NextCursor)

	for name, cursor := range map[string]string{
		"not base64":     "!!!not-base64!!!",
		"empty payload":  base64.RawURLEncoding.EncodeToString([]byte("{}")),
		"not json":       base64.RawURLEncoding.EncodeToString([]byte("nonsense")),
		"wrong version":  encodeWorkbenchListCursor(workbenchListCursor{Version: 2, TenantID: 1, OwnerID: "u1", CreatedAt: base.Format(time.RFC3339Nano), RunID: "r-2"}),
		"foreign tenant": encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 2, OwnerID: "u1", CreatedAt: base.Format(time.RFC3339Nano), RunID: "r-2"}),
		"foreign owner":  encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 1, OwnerID: "u2", CreatedAt: base.Format(time.RFC3339Nano), RunID: "r-2"}),
		"missing run id": encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 1, OwnerID: "u1", CreatedAt: base.Format(time.RFC3339Nano)}),
		"bad timestamp":  encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 1, OwnerID: "u1", CreatedAt: "yesterday", RunID: "r-2"}),
		"changed agent":  encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 1, OwnerID: "u1", AgentID: "agent-z", CreatedAt: base.Format(time.RFC3339Nano), RunID: "r-2"}),
		"changed status": encodeWorkbenchListCursor(workbenchListCursor{Version: 1, TenantID: 1, OwnerID: "u1", Status: "succeeded", CreatedAt: base.Format(time.RFC3339Nano), RunID: "r-2"}),
	} {
		_, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{AgentID: "agent-x", Cursor: cursor})
		require.ErrorIs(t, err, ErrWorkbenchCursor, "cursor case %q must be rejected", name)
	}

	// The same agent-scoped cursor replayed without the agent facet is a
	// different filter and must be refused, not silently widened.
	_, replayErr := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Cursor: page.NextCursor})
	require.ErrorIs(t, replayErr, ErrWorkbenchCursor)

	// The legitimate cursor still pages to the remaining rows.
	rest, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{AgentID: "agent-x", Limit: 2, Cursor: page.NextCursor})
	require.NoError(t, err)
	require.Equal(t, []string{"r-0"}, runIDs(rest))
	require.Empty(t, rest.NextCursor)
}

// TestWorkbenchListLimitBounds clamps the page size: default 30, hard cap 100.
func TestWorkbenchListLimitBounds(t *testing.T) {
	db := openRunTestDB(t)
	base := time.Date(2026, 9, 12, 6, 0, 0, 0, time.UTC)
	const total = 105
	for i := 0; i < total; i++ {
		insertWorkbenchRun(t, db, workbenchRunSeed{
			tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%03d", i), session: "s1",
			status: "queued", agent: "agent-x", target: "platform",
			at: base.Add(time.Duration(i) * time.Millisecond),
		})
	}
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 30)
	require.NotEmpty(t, page.NextCursor)

	page, err = store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Limit: 500})
	require.NoError(t, err)
	require.Len(t, page.Items, 100)
	require.NotEmpty(t, page.NextCursor)

	// Identity-free callers get nothing: the predicate is never optional.
	_, err = store.ListOwnedExecutions(ctx, 0, "u1", WorkbenchExecutionFilter{})
	require.Error(t, err)
	_, err = store.ListOwnedExecutions(ctx, 1, " ", WorkbenchExecutionFilter{})
	require.Error(t, err)
}

// TestWorkbenchListConcurrentReadersSharePages exercises the list under
// parallel readers and writers to surface cursor races beyond the sequential
// walk above.
func TestWorkbenchListConcurrentReadersSharePages(t *testing.T) {
	db := openRunTestDB(t)
	base := time.Date(2026, 9, 12, 5, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		insertWorkbenchRun(t, db, workbenchRunSeed{
			tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%02d", i), session: "s1",
			status: "running", agent: "agent-x", target: "platform",
			at: base.Add(time.Duration(i) * time.Millisecond),
		})
	}
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	var writers sync.WaitGroup
	for i := 12; i < 20; i++ {
		writers.Add(1)
		go func(index int) {
			defer writers.Done()
			insertWorkbenchRun(t, db, workbenchRunSeed{
				tenant: 1, owner: "u1", runID: fmt.Sprintf("r-%02d", index), session: "s2",
				status: "running", agent: "agent-x", target: "platform",
				at: base.Add(time.Duration(index) * time.Millisecond),
			})
		}(i)
	}

	var readers sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			seen := map[string]bool{}
			cursor := ""
			for {
				page, err := store.ListOwnedExecutions(ctx, 1, "u1", WorkbenchExecutionFilter{Limit: 5, Cursor: cursor})
				if err != nil {
					t.Errorf("concurrent list failed: %v", err)
					return
				}
				for _, id := range runIDs(page) {
					if seen[id] {
						t.Errorf("row %s repeated across pages", id)
					}
					seen[id] = true
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
		}()
	}
	readers.Wait()
	writers.Wait()
}

func runIDs(page WorkbenchExecutionPage) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.RunID)
	}
	return ids
}

func requireNoDuplicateRunIDs(t *testing.T, ids []string) {
	t.Helper()
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		require.False(t, seen[id], "run %s appeared more than once", id)
		seen[id] = true
	}
}

// TestWorkbenchListCursorErrorDistinct ensures the cursor sentinel is not
// confusable with other repository errors at the handler boundary.
func TestWorkbenchListCursorErrorDistinct(t *testing.T) {
	require.False(t, errors.Is(ErrWorkbenchCursor, agentruntime.ErrNotFound))
	require.True(t, strings.Contains(ErrWorkbenchCursor.Error(), "cursor"))
}
