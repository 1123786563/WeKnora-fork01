package repository

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/execution"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openExecutionTargetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&executionTargetRow{}, &executionWorkspaceRow{}, &executionTargetIdentityRow{}, &registrationProjectionRow{}))
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
	var row executionTargetRow
	require.NoError(t, store.(*executionTargetStore).db.First(&row, "tenant_id = ? AND id = ?", 1, "t1").Error)
	require.NotNil(t, row.RevokedAt)
	_, err := store.GetOwnedTarget(ctx, 1, "u1", "t1")
	require.ErrorIs(t, err, ErrExecutionTargetNotFound)
}

func TestExecutionTargetFacadeRevokesPersonalRegistrationAndIdentityAtomically(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	store := NewExecutionTargetStore(db)
	ctx := context.Background()
	target := execution.Target{ID: "node-1", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 1, RuntimeID: "runtime-1", ExternalTargetID: "external-1"}
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, OwnerID: target.OwnerID, CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, db.Create(&registrationProjectionRow{TenantID: 1, OwnerID: "u1", ID: target.ID, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, store.CreateTarget(ctx, target, "opaque"))

	require.NoError(t, store.RevokeTarget(ctx, 1, "u1", target.ID))
	var revokedTarget executionTargetRow
	require.NoError(t, db.First(&revokedTarget, "tenant_id = ? AND id = ?", 1, target.ID).Error)
	require.Equal(t, "revoked", revokedTarget.State)
	require.EqualValues(t, 2, revokedTarget.CredentialVersion)
	var registration registrationProjectionRow
	require.NoError(t, db.First(&registration, "tenant_id = ? AND registration_id = ?", 1, target.ID).Error)
	require.Equal(t, "revoked", registration.State)
	require.EqualValues(t, 2, registration.CredentialVersion)
	var identity executionTargetIdentityRow
	require.NoError(t, db.First(&identity, "tenant_id = ? AND runtime_id = ?", 1, target.RuntimeID).Error)
	require.Equal(t, "revoked", identity.State)
	require.EqualValues(t, 2, identity.CredentialVersion)

	// The operation is idempotency-safe and fail-closed: the second revoke
	// cannot report success or mutate an already revoked registration.
	require.ErrorIs(t, store.RevokeTarget(ctx, 1, "u1", target.ID), ErrExecutionTargetNotFound)
}

func TestExecutionTargetFacadeRevokeRollsBackWhenProjectionIsMissing(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	store := NewExecutionTargetStore(db)
	target := execution.Target{ID: "node-missing", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 1, RuntimeID: "runtime-missing", ExternalTargetID: "external-missing"}
	require.NoError(t, db.Create(&registrationProjectionRow{TenantID: 1, OwnerID: "u1", ID: target.ID, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, store.CreateTarget(context.Background(), target, "opaque"))
	require.ErrorIs(t, store.RevokeTarget(context.Background(), 1, "u1", target.ID), ErrExecutionTargetNotFound)
	var row executionTargetRow
	require.NoError(t, db.First(&row, "tenant_id = ? AND id = ?", 1, target.ID).Error)
	require.Equal(t, "active", row.State, "target update must roll back when identity projection is absent")
	require.EqualValues(t, 1, row.CredentialVersion)
}

func TestExecutionTargetFacadeRejectsMismatchedRegistrationProjection(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	store := NewExecutionTargetStore(db)
	target := execution.Target{ID: "node-mismatch", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 1, RuntimeID: "runtime-1", ExternalTargetID: "external-1"}
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, OwnerID: target.OwnerID, CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, db.Create(&registrationProjectionRow{TenantID: 1, OwnerID: target.OwnerID, ID: target.ID, RuntimeID: "runtime-other", ExternalTargetID: "external-other", CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, store.CreateTarget(context.Background(), target, "opaque"))
	require.ErrorIs(t, store.RevokeTarget(context.Background(), 1, target.OwnerID, target.ID), ErrExecutionTargetNotFound)
	var row executionTargetRow
	require.NoError(t, db.First(&row, "tenant_id = ? AND id = ?", 1, target.ID).Error)
	require.Equal(t, "active", row.State)
	require.EqualValues(t, 1, row.CredentialVersion)
}

func TestExecutionTargetFacadeRejectsDivergentCredentialVersions(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	store := NewExecutionTargetStore(db)
	target := execution.Target{ID: "node-divergent", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 1, RuntimeID: "runtime-divergent", ExternalTargetID: "external-divergent"}
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, OwnerID: target.OwnerID, CredentialVersion: 2, State: "active"}).Error)
	require.NoError(t, db.Create(&registrationProjectionRow{TenantID: 1, OwnerID: target.OwnerID, ID: target.ID, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, CredentialVersion: 1, State: "active"}).Error)
	require.NoError(t, store.CreateTarget(context.Background(), target, "opaque"))
	require.ErrorIs(t, store.RevokeTarget(context.Background(), 1, target.OwnerID, target.ID), ErrExecutionTargetNotFound)
	var row executionTargetRow
	require.NoError(t, db.First(&row, "tenant_id = ? AND id = ?", 1, target.ID).Error)
	require.Equal(t, "active", row.State)
}

func TestLegacyPersonalTargetRevokeRejectsDivergentRegistrationVersion(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	provisioner := NewPersonalTargetProvisioner(db)
	target := execution.Target{ID: "legacy-divergent", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 1, RuntimeID: "runtime-legacy", ExternalTargetID: "external-legacy"}
	require.NoError(t, db.Create(&executionTargetRow{TenantID: 1, ID: target.ID, OwnerID: target.OwnerID, Kind: target.Kind, State: target.State, CredentialVersion: 1, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID}).Error)
	require.NoError(t, db.Create(&registrationProjectionRow{TenantID: 1, OwnerID: target.OwnerID, ID: target.ID, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, CredentialVersion: 2, State: "active"}).Error)
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: target.RuntimeID, ExternalTargetID: target.ExternalTargetID, OwnerID: target.OwnerID, CredentialVersion: 1, State: "active"}).Error)
	err := provisioner.RevokePersonalTarget(context.Background(), db, target, time.Now().UTC())
	require.Error(t, err)
	var state string
	require.NoError(t, db.Raw("SELECT state FROM execution_targets WHERE id = ?", target.ID).Row().Scan(&state))
	require.Equal(t, "active", state)
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

func TestExecutionTargetIdentityProviderRejectsSelfAssertionAndStaleRotation(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	provider := NewExecutionTargetIdentityProvider(db)
	ctx := context.Background()
	target := execution.Target{RuntimeID: "r1", ExternalTargetID: "x1", CredentialVersion: 2}
	require.ErrorIs(t, provider.VerifyTarget(ctx, 1, "u1", target), execution.ErrTargetUntrusted)
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: "r1", ExternalTargetID: "x1", OwnerID: "u1", CredentialVersion: 1, State: "active"}).Error)
	require.ErrorIs(t, provider.VerifyTarget(ctx, 1, "u1", target), execution.ErrTargetUntrusted)
	require.NoError(t, db.Model(&executionTargetIdentityRow{}).Where("tenant_id = ? AND runtime_id = ? AND external_target_id = ?", 1, "r1", "x1").Update("credential_version", 2).Error)
	require.NoError(t, provider.VerifyTarget(ctx, 1, "u1", target))
}

func TestCreateTargetIfTrustedRejectsCredentialRotationAtomically(t *testing.T) {
	db := openExecutionTargetTestDB(t)
	store := NewExecutionTargetStore(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: "r1", ExternalTargetID: "x1", OwnerID: "u1", CredentialVersion: 1, State: "active"}).Error)
	stale := execution.Target{ID: "stale", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}
	require.NoError(t, db.Model(&executionTargetIdentityRow{}).Where("tenant_id = ? AND runtime_id = ? AND external_target_id = ?", 1, "r1", "x1").Update("credential_version", 2).Error)
	require.ErrorIs(t, store.CreateTargetIfTrusted(ctx, stale, "root"), execution.ErrTargetUntrusted)
	var count int64
	require.NoError(t, db.Model(&executionTargetRow{}).Where("id = ?", "stale").Count(&count).Error)
	require.Zero(t, count)
}

func TestCreateTargetIfTrustedConcurrentCredentialRotation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "concurrent.db")+"?_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&executionTargetRow{}, &executionWorkspaceRow{}, &executionTargetIdentityRow{}))
	store := NewExecutionTargetStore(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&executionTargetIdentityRow{TenantID: 1, RuntimeID: "r1", ExternalTargetID: "x1", OwnerID: "u1", CredentialVersion: 1, State: "active"}).Error)
	target := execution.Target{ID: "concurrent", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 1, RuntimeID: "r1", ExternalTargetID: "x1"}
	var wg sync.WaitGroup
	rotationStarted := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(rotationStarted)
		_ = db.Model(&executionTargetIdentityRow{}).Where("tenant_id = ? AND runtime_id = ? AND external_target_id = ?", 1, "r1", "x1").Update("credential_version", 2).Error
	}()
	<-rotationStarted
	createErr := store.CreateTargetIfTrusted(ctx, target, "root")
	wg.Wait()
	if createErr == nil {
		var row executionTargetRow
		require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, "concurrent").First(&row).Error)
		require.EqualValues(t, 1, row.CredentialVersion, "a target created before rotation keeps the version it was authorized with")
	} else {
		require.ErrorIs(t, createErr, execution.ErrTargetUntrusted)
	}
}

func TestExecutionTargetStorePersistsServerUsageBinding(t *testing.T) {
	store := NewExecutionTargetStore(openExecutionTargetTestDB(t))
	target := execution.Target{ID: "byok-target", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 7, RuntimeID: "r-byok", ExternalTargetID: "x-byok", UsageBinding: execution.UsageBinding{ParentRunID: "root-run", Source: "platform_gateway", Funding: "byok", Service: "model", PriceVersion: "pv-byok", Revision: 2, Status: "final", Dimensions: map[string]int64{"model": 8}}}
	if err := store.CreateTarget(context.Background(), target, "root"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetOwnedTarget(context.Background(), 1, "u1", "byok-target")
	if err != nil {
		t.Fatal(err)
	}
	if got.UsageBinding.Funding != "byok" || got.UsageBinding.ParentRunID != "root-run" || got.UsageBinding.Revision != 2 || got.UsageBinding.Dimensions["model"] != 8 {
		t.Fatalf("binding=%+v", got.UsageBinding)
	}
}
