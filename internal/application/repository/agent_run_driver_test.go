package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// TestWorkbenchDriverIsolation catches a worker accidentally discovering or
// claiming a remote-driver run through the legacy platform worker methods.
func TestWorkbenchDriverIsolation(t *testing.T) {
	for _, database := range []string{"sqlite", "postgres"} {
		t.Run(database, func(t *testing.T) { testWorkbenchDriverIsolation(t) })
	}
}

func testWorkbenchDriverIsolation(t *testing.T) {
	store := NewAgentRunStore(openRunTestDB(t))
	ctx := context.Background()
	in := testAdmission()
	in.Driver = "paseo"
	in.TargetID = "node-1"
	in.BudgetRef = "budget-1"

	created, err := store.Admit(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "paseo", created.Driver)
	require.Equal(t, "node-1", created.TargetID)
	require.Equal(t, "budget-1", created.BudgetRef)

	platform, err := store.Scan(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, platform)

	paseo, err := store.ScanDriver(ctx, "paseo", 10)
	require.NoError(t, err)
	require.Equal(t, []agentruntime.RunKey{in.Key}, paseo)

	_, err = store.Claim(ctx, in.Key, "platform-worker", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	_, err = store.ClaimDriver(ctx, in.Key, "paseo", "paseo-worker", time.Minute)
	require.NoError(t, err)

	_, err = store.GetOwnedRun(ctx, 1, "other-owner", in.Key.RunID)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	owned, err := store.GetOwnedRun(ctx, 1, "u1", in.Key.RunID)
	require.NoError(t, err)
	require.Equal(t, "paseo", owned.Driver)
}

// TestWorkbenchPaseoAdmissionKeepsSessionEngine catches a remote admission
// mutating the session engine merely to get past the legacy platform guard.
func TestWorkbenchPaseoAdmissionKeepsSessionEngine(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("UPDATE sessions SET engine_type = 'builtin' WHERE id = 's1'").Error)
	in := testAdmission()
	in.Driver = "paseo"

	created, err := NewAgentRunStore(db).Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "paseo", created.Driver)
	var sessionEngine, runEngine string
	require.NoError(t, db.Raw("SELECT engine_type FROM sessions WHERE id = 's1'").Scan(&sessionEngine).Error)
	require.NoError(t, db.Raw("SELECT engine_type FROM agent_runs WHERE tenant_id = 1 AND run_id = 'r1'").Scan(&runEngine).Error)
	require.Equal(t, "builtin", sessionEngine)
	require.Empty(t, runEngine)
}

// TestWorkbenchDriverRejectsUnknown catches a future driver becoming an
// executable row without an explicit admission and worker implementation.
func TestWorkbenchDriverRejectsUnknown(t *testing.T) {
	store := NewAgentRunStore(openRunTestDB(t))
	in := testAdmission()
	in.Driver = "unknown-driver"

	_, err := store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestWorkbenchDriverMethodsNormalizeAndReject(t *testing.T) {
	store := NewAgentRunStore(openRunTestDB(t))
	ctx := context.Background()
	in := testAdmission()
	in.Key.RunID = "platform-run"
	in.RequestID = "platform-request"
	in.AssistantMessageID = "platform-assistant"
	require.NoError(t, func() error { _, err := store.Admit(ctx, in); return err }())

	keys, err := store.ScanDriver(ctx, "", 10)
	require.NoError(t, err)
	require.Equal(t, []agentruntime.RunKey{in.Key}, keys)
	_, err = store.ClaimDriver(ctx, in.Key, "", "platform-worker", time.Minute)
	require.NoError(t, err)

	_, err = store.ScanDriver(ctx, "unimplemented", 10)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	_, err = store.ClaimDriver(ctx, in.Key, "unimplemented", "worker", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestWorkbenchOwnedRunIsTenantScoped(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	first := testAdmission()
	first.Key.RunID = "same-run"
	first.RequestID = "tenant-one-request"
	first.AssistantMessageID = "tenant-one-assistant"
	require.NoError(t, func() error { _, err := store.Admit(ctx, first); return err }())

	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id)
		VALUES ('u2', 'u2', 'u2@example.test', 'x', 2)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)
		VALUES ('s3', 2, 'tenant-two', 'u2', 'trpc')`).Error)
	second := testAdmission()
	second.Key.TenantID, second.Key.RunID = 2, "same-run"
	second.SessionID, second.UserID, second.RequestID, second.AssistantMessageID = "s3", "u2", "tenant-two-request", "tenant-two-assistant"
	require.NoError(t, func() error { _, err := store.Admit(ctx, second); return err }())

	_, err := store.GetOwnedRun(ctx, 2, "u1", "same-run")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	owned, err := store.GetOwnedRun(ctx, 2, "u2", "same-run")
	require.NoError(t, err)
	require.Equal(t, uint64(2), owned.Key.TenantID)
}
