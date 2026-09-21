package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

// These tests run against the REAL versioned SQLite migration stream
// (openRunTestDB applies every migrations/sqlite up migration, including
// 000099_agent_versions), so the unique index, the append-only columns and
// the tenant predicate are the production schema, not an AutoMigrate
// approximation.

// TestAgentVersionFreezeIncrementsAndIsolatesTenants proves the brief's core
// contract: two consecutive freezes of one agent get increasing version
// numbers, both snapshots survive verbatim (append-only), and another tenant
// cannot fetch either version.
func TestAgentVersionFreezeIncrementsAndIsolatesTenants(t *testing.T) {
	repo := NewAgentVersionRepository(openRunTestDB(t))
	ctx := context.Background()

	first, err := repo.Freeze(ctx, &types.AgentVersionEntity{
		TenantID: 1, AgentID: "agent-a",
		Snapshot: `{"name":"first config"}`, SourceSHA256: "sha-first", FrozenBy: "user-1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)
	require.Equal(t, 1, first.VersionNumber, "the first frozen version must be number 1")

	second, err := repo.Freeze(ctx, &types.AgentVersionEntity{
		TenantID: 1, AgentID: "agent-a",
		Snapshot: `{"name":"second config"}`, SourceSHA256: "sha-second", FrozenBy: "user-1",
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID, "each freeze appends a new immutable row")
	require.Equal(t, 2, second.VersionNumber, "the second frozen version must be number 2")

	// Both snapshots survive verbatim: freezing again never rewrites the
	// prior row's bytes or digest.
	gotFirst, err := repo.GetByTenantAndID(ctx, 1, first.ID)
	require.NoError(t, err)
	require.NotNil(t, gotFirst)
	require.Equal(t, `{"name":"first config"}`, gotFirst.Snapshot)
	require.Equal(t, "sha-first", gotFirst.SourceSHA256)
	require.Equal(t, 1, gotFirst.VersionNumber)

	gotSecond, err := repo.GetByTenantAndID(ctx, 1, second.ID)
	require.NoError(t, err)
	require.NotNil(t, gotSecond)
	require.Equal(t, `{"name":"second config"}`, gotSecond.Snapshot)
	require.Equal(t, "sha-second", gotSecond.SourceSHA256)
	require.Equal(t, 2, gotSecond.VersionNumber)

	// Another tenant cannot fetch either version: cross-tenant reads fail
	// closed to not-found (nil), never data.
	crossFirst, err := repo.GetByTenantAndID(ctx, 2, first.ID)
	require.NoError(t, err)
	require.Nil(t, crossFirst)
	crossSecond, err := repo.GetByTenantAndID(ctx, 2, second.ID)
	require.NoError(t, err)
	require.Nil(t, crossSecond)

	// The same version ID inside the owning tenant still resolves (the nil
	// above is the tenant predicate, not a broken lookup).
	again, err := repo.GetByTenantAndID(ctx, 1, first.ID)
	require.NoError(t, err)
	require.NotNil(t, again)

	// Listing is tenant-scoped and ascending by version number.
	rows, err := repo.ListByTenantAndAgent(ctx, 1, "agent-a")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 1, rows[0].VersionNumber)
	require.Equal(t, 2, rows[1].VersionNumber)

	other, err := repo.ListByTenantAndAgent(ctx, 2, "agent-a")
	require.NoError(t, err)
	require.Empty(t, other)
}

// TestAgentVersionFreezeNumberStartsPerAgentScope proves the (tenant_id,
// agent_id, version_number) scope: a different agent in the same tenant and
// the same agent id in a different tenant both start at 1.
func TestAgentVersionFreezeNumberStartsPerAgentScope(t *testing.T) {
	repo := NewAgentVersionRepository(openRunTestDB(t))
	ctx := context.Background()

	for _, scope := range []struct {
		tenant uint64
		agent  string
	}{
		{1, "agent-b"}, {1, "agent-c"}, {2, "agent-b"},
	} {
		first, err := repo.Freeze(ctx, &types.AgentVersionEntity{
			TenantID: scope.tenant, AgentID: scope.agent,
			Snapshot: `{}`, SourceSHA256: "sha", FrozenBy: "user-1",
		})
		require.NoError(t, err)
		require.Equal(t, 1, first.VersionNumber, "scope %+v must start at version 1", scope)
	}
}

// TestAgentVersionFreezeEnforcesScopeUniqueness proves the migration-level
// unique index (not just the repository's number allocation): two rows with
// the same (tenant_id, agent_id, version_number) cannot coexist, while the
// same number under a different agent or tenant can.
func TestAgentVersionFreezeEnforcesScopeUniqueness(t *testing.T) {
	db := openRunTestDB(t)
	insert := `INSERT INTO agent_versions
		(id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
	require.NoError(t, db.Exec(insert, "v-1", 1, "agent-a", 1, `{}`, "sha", "u1").Error)
	require.Error(t, db.Exec(insert, "v-2", 1, "agent-a", 1, `{}`, "sha", "u1").Error,
		"duplicate (tenant, agent, version_number) must be rejected by the schema")
	require.NoError(t, db.Exec(insert, "v-3", 1, "agent-b", 1, `{}`, "sha", "u1").Error,
		"another agent may reuse the number")
	require.NoError(t, db.Exec(insert, "v-4", 2, "agent-a", 1, `{}`, "sha", "u1").Error,
		"another tenant may reuse the number")
}
