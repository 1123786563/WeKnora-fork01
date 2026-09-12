package service_test

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// deletionGuardStub records the O02 pre-deletion steps and replays configured
// pending commercial work.
type deletionGuardStub struct {
	calls         []string
	pending       []string
	failingStep   string
	policyVersion string
}

func (g *deletionGuardStub) DisableNewScheduling(ctx context.Context, tenantID uint64) error {
	g.calls = append(g.calls, "disable_scheduling")
	if g.failingStep == "disable_scheduling" {
		return assert.AnError
	}
	return nil
}

func (g *deletionGuardStub) RevokeConnections(ctx context.Context, tenantID uint64) error {
	g.calls = append(g.calls, "revoke_connections")
	if g.failingStep == "revoke_connections" {
		return assert.AnError
	}
	return nil
}

func (g *deletionGuardStub) PendingCommercialWork(ctx context.Context, tenantID uint64) ([]string, error) {
	g.calls = append(g.calls, "check_pending")
	return g.pending, nil
}

func (g *deletionGuardStub) RetentionPolicyVersion() string {
	g.calls = append(g.calls, "retention_policy")
	return g.policyVersion
}

func newDeletionTestService(t *testing.T, guard service.TenantDeletionGuard) (interfaces.TenantService, interfaces.TenantRepository, uint64) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Tenant{}, &types.TenantMember{}))
	repo := repository.NewTenantRepository(db)
	tenant := &types.Tenant{Name: "legacy-space"}
	require.NoError(t, repo.CreateTenant(context.Background(), tenant))
	svc := service.NewTenantService(repo, nil, service.WithDeletionGuard(guard))
	return svc, repo, tenant.ID
}

// Deletion must be refused while in-flight commercial records exist, with an
// error listing them, and the tenant must survive (no cascade deletion of
// pending commercial records via tenant deletion).
func TestTenantDeleteRefusedWhileCommercialWorkInFlight(t *testing.T) {
	guard := &deletionGuardStub{pending: []string{"settlement:41", "payment:7"}, policyVersion: "ret-2026-09"}
	svc, repo, id := newDeletionTestService(t, guard)

	err := svc.DeleteTenant(context.Background(), id)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "settlement:41")
	assert.Contains(t, err.Error(), "payment:7")
	// retention_policy is recorded on the decision even for a refusal.
	assert.Equal(t,
		[]string{"disable_scheduling", "revoke_connections", "check_pending", "retention_policy"},
		guard.calls)
	stillThere, getErr := repo.GetTenantByID(context.Background(), id)
	require.NoError(t, getErr)
	assert.NotNil(t, stillThere, "tenant must not be deleted while commercial work is pending")
}

// With no pending commercial work the deletion proceeds after the guard steps
// ran in order, under the recorded retention policy.
func TestTenantDeleteAllowedAfterCommercialWorkSettles(t *testing.T) {
	guard := &deletionGuardStub{policyVersion: "ret-2026-09"}
	svc, repo, id := newDeletionTestService(t, guard)

	require.NoError(t, svc.DeleteTenant(context.Background(), id))

	assert.Equal(t,
		[]string{"disable_scheduling", "revoke_connections", "check_pending", "retention_policy"},
		guard.calls)
	_, getErr := repo.GetTenantByID(context.Background(), id)
	assert.Error(t, getErr, "tenant should be gone after allowed deletion")
}

// A failing guard step aborts deletion before any tenant resource is removed.
func TestTenantDeleteAbortsWhenGuardStepFails(t *testing.T) {
	guard := &deletionGuardStub{failingStep: "revoke_connections"}
	svc, repo, id := newDeletionTestService(t, guard)

	err := svc.DeleteTenant(context.Background(), id)

	require.Error(t, err)
	assert.Equal(t, []string{"disable_scheduling", "revoke_connections"}, guard.calls)
	_, getErr := repo.GetTenantByID(context.Background(), id)
	require.NoError(t, getErr)
}

// Without a wired guard the legacy deletion path is preserved (documented
// deployment fallback: the commercial module is optional).
func TestTenantDeleteWithoutGuardKeepsLegacyPath(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Tenant{}, &types.TenantMember{}))
	repo := repository.NewTenantRepository(db)
	tenant := &types.Tenant{Name: "plain-space"}
	require.NoError(t, repo.CreateTenant(context.Background(), tenant))
	svc := service.NewTenantService(repo, nil)

	require.NoError(t, svc.DeleteTenant(context.Background(), tenant.ID))

	_, getErr := repo.GetTenantByID(context.Background(), tenant.ID)
	assert.Error(t, getErr)
}
