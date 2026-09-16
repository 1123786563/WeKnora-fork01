package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openExecutionTargetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&executionTargetRow{}, &executionWorkspaceRow{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX uq_execution_targets_runtime_external ON execution_targets (runtime_id, external_target_id)").Error)
	return db
}

func TestExecutionTargetStoreScopesOwnerAndTenant(t *testing.T) {
	store := NewExecutionTargetStore(openExecutionTargetTestDB(t))
	target := execution.Target{ID: "t1", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}
	require.NoError(t, store.CreateTarget(context.Background(), target, "node-root"))
	got, err := store.GetOwnedTarget(context.Background(), 1, "u1", "t1")
	require.NoError(t, err)
	require.Equal(t, target, got)
	_, err = store.GetOwnedTarget(context.Background(), 1, "u2", "t1")
	require.ErrorIs(t, err, ErrExecutionTargetNotFound)
	_, err = store.GetOwnedTarget(context.Background(), 2, "u1", "t1")
	require.ErrorIs(t, err, ErrExecutionTargetNotFound)
}

func TestExecutionTargetStoreRevocationAndUniqueBinding(t *testing.T) {
	store := NewExecutionTargetStore(openExecutionTargetTestDB(t))
	ctx := context.Background()
	target := execution.Target{ID: "t1", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}
	require.NoError(t, store.CreateTarget(ctx, target, "root"))
	require.Error(t, store.CreateTarget(ctx, execution.Target{ID: "t2", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}, "root"))
	require.NoError(t, store.RevokeTarget(ctx, 1, "u1", "t1"))
	_, err := store.GetOwnedTarget(ctx, 1, "u1", "t1")
	require.ErrorIs(t, err, ErrExecutionTargetNotFound)
}

func TestExecutionTargetStoreWorkspaceIsOwnedThroughTarget(t *testing.T) {
	store := NewExecutionTargetStore(openExecutionTargetTestDB(t))
	ctx := context.Background()
	target := execution.Target{ID: "t1", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}
	require.NoError(t, store.CreateTarget(ctx, target, "server-root"))
	require.NoError(t, store.CreateWorkspace(ctx, execution.Workspace{ID: "w1", TenantID: 1, TargetID: "t1", RootRef: "opaque-root"}))
	got, err := store.GetOwnedWorkspace(ctx, 1, "u1", "w1")
	require.NoError(t, err)
	require.Equal(t, execution.Workspace{ID: "w1", TenantID: 1, TargetID: "t1", RootRef: "opaque-root"}, got)
	_, err = store.GetOwnedWorkspace(ctx, 1, "u2", "w1")
	require.ErrorIs(t, err, ErrExecutionTargetNotFound)
}
