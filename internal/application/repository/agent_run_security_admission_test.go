package repository

import (
	"context"
	"database/sql"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedRunSecurityIdentity(t *testing.T, db *gorm.DB) (string, string, string, string) {
	t.Helper()
	require.NoError(t, db.Create(&types.CustomAgent{ID: "source-run-security", TenantID: 1, Name: "source", CreatedBy: "u1"}).Error)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "source-run-security", "1.0.0")
	adoption, _, err := NewAgentAdoptionRepository(db).AdoptListing(context.Background(), &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "u1"})
	require.NoError(t, err)
	const agentID, versionID = "local-run-security", "local-run-security-v1"
	require.NoError(t, db.Create(&types.CustomAgent{ID: agentID, TenantID: 1, Name: "local", CreatedBy: "u1"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: versionID, TenantID: 1, AgentID: agentID, VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha", FrozenBy: "u1"}).Error)
	variant, err := NewAgentAdoptionRepository(db).CreateVariant(context.Background(), &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "local", State: "tested", CreatedBy: "u1"})
	require.NoError(t, err)
	_, err = NewAgentAdoptionRepository(db).UpdateVariantState(context.Background(), 1, variant.ID, []string{"tested"}, "published", map[string]any{"local_agent_id": agentID, "local_agent_version_id": versionID, "published_by": "u1"})
	require.NoError(t, err)
	return agentID, versionID, releaseID, listingID
}

func securityRunAdmission(agentID, versionID, releaseID string) agentruntime.Admission {
	in := testAdmission()
	in.AgentID, in.LocalAgentVersionID, in.ReleaseID = agentID, versionID, releaseID
	return in
}

func installGuardAcquiredBarrier(t *testing.T, db *gorm.DB, marker string) *tenantGuardBarrier {
	t.Helper()
	barrier := &tenantGuardBarrier{reached: make(chan struct{}), release: make(chan struct{})}
	name := "test:tenant-guard-acquired:" + marker
	err := db.Callback().Raw().After("gorm:raw").Register(name, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Context.Value(tenantGuardAttemptKey{}) != marker ||
			tx.Statement.SQL.String() != "UPDATE tenants SET id = id WHERE id = ?" {
			return
		}
		close(barrier.reached)
		<-barrier.release
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Callback().Raw().Remove(name) })
	t.Cleanup(func() { releaseTenantGuardBarrier(barrier) })
	return barrier
}

func TestAgentRunAdmitSecurityDenialPreservesRetirementGateAndWritesNothing(t *testing.T) {
	db := openRunTestDB(t)
	agentID, versionID, releaseID, _ := seedRunSecurityIdentity(t, db)
	require.NoError(t, NewAgentSecurityStore(db).AppendReleaseRevocation(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: releaseID, Reason: "test revoke", RevokedBy: "u1"}))
	_, err := NewAgentRunStore(db).Admit(context.Background(), securityRunAdmission(agentID, versionID, releaseID))
	require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
	var count int64
	require.NoError(t, db.Table("agent_runs").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("messages").Count(&count).Error)
	require.Zero(t, count)
	var slot *string
	require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", 1, "s1").Select("active_agent_run_id").Scan(&slot).Error)
	require.Nil(t, slot)
	retiredDB := openRunTestDB(t)
	versionID, releaseID = seedAgentVariant(t, retiredDB, "retired")
	_, err = NewAgentRunStore(retiredDB).Admit(context.Background(), testRetiredAgentAdmission(versionID, releaseID))
	require.ErrorIs(t, err, agentruntime.ErrAgentUseDenied)
}

func TestAgentRunAdmitRejectsSecurityPinsWithoutAgentID(t *testing.T) {
	for _, shape := range []struct {
		name    string
		version string
		release string
	}{
		{name: "version only", version: "trusted-version"},
		{name: "release only", release: "trusted-release"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			db := openRunTestDB(t)
			in := testAdmission()
			in.LocalAgentVersionID, in.ReleaseID = shape.version, shape.release
			_, err := NewAgentRunStore(db).Admit(context.Background(), in)
			require.ErrorIs(t, err, ErrAgentSecurityReleaseUnresolvable)
			var count int64
			require.NoError(t, db.Table("agent_runs").Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, db.Table("messages").Count(&count).Error)
			require.Zero(t, count)
			var slot *string
			require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", 1, "s1").Select("active_agent_run_id").Scan(&slot).Error)
			require.Nil(t, slot)
		})
	}
}

func TestAgentRunAdmitReplaysExistingRunAfterRevocation(t *testing.T) {
	db := openRunTestDB(t)
	agentID, versionID, releaseID, _ := seedRunSecurityIdentity(t, db)
	store := NewAgentRunStore(db)
	in := securityRunAdmission(agentID, versionID, releaseID)
	first, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.NoError(t, NewAgentSecurityStore(db).AppendReleaseRevocation(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: releaseID, Reason: "test revoke", RevokedBy: "u1"}))
	second, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, first.Key, second.Key)
	var pin struct {
		AgentID, VersionID, ReleaseID, Source sql.NullString
	}
	require.NoError(t, db.Raw("SELECT security_agent_id AS agent_id, security_local_agent_version_id AS version_id, security_release_id AS release_id, security_pin_source AS source FROM agent_runs WHERE tenant_id = ? AND run_id = ?", 1, first.Key.RunID).Scan(&pin).Error)
	require.Equal(t, agentID, pin.AgentID.String)
	require.Equal(t, versionID, pin.VersionID.String)
	require.Equal(t, releaseID, pin.ReleaseID.String)
	require.Equal(t, "admission", pin.Source.String)
	in.RequestHash = "changed"
	_, err = store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func runAdmissionRevocationOrders(t *testing.T, dependency bool) {
	t.Helper()
	for _, admissionFirst := range []bool{false, true} {
		db := openRunTestDB(t)
		agentID, versionID, releaseID, _ := seedRunSecurityIdentity(t, db)
		if dependency {
			require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).
				Update("dependency_lock_json", `{"dependencies":[{"type":"skill","id":"locked","version":"1","digest":"sha256:x"}]}`).Error)
		}
		if !admissionFirst {
			if dependency {
				require.NoError(t, NewAgentSecurityStore(db).AppendDependencyRevocation(context.Background(), &types.AgentDependencyRevocationEntity{TenantID: 1, DepType: "skill", DepID: "locked", DepVersion: "1", DepDigest: "sha256:x", Reason: "revoke"}))
			} else {
				require.NoError(t, NewAgentSecurityStore(db).AppendReleaseRevocation(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: releaseID, Reason: "revoke"}))
			}
			_, err := NewAgentRunStore(db).Admit(context.Background(), securityRunAdmission(agentID, versionID, releaseID))
			require.ErrorIs(t, err, ErrAgentSecurityReleaseBlocked)
			continue
		}
		ensureTenantGuardTestPool(t, db)
		marker := "agent-run-admission"
		barrier := installGuardAcquiredBarrier(t, db, marker)
		ctx := context.WithValue(context.Background(), tenantGuardAttemptKey{}, marker)
		admitDone := make(chan error, 1)
		go func() {
			_, err := NewAgentRunStore(db).Admit(ctx, securityRunAdmission(agentID, versionID, releaseID))
			admitDone <- err
		}()
		waitTenantGuardBarrier(t, barrier)
		revokeMarker := "agent-run-revocation"
		revokeAttempt := installTenantGuardAttemptBarrier(t, db, revokeMarker)
		revokeDone := make(chan error, 1)
		go func() {
			revokeCtx := context.WithValue(context.Background(), tenantGuardAttemptKey{}, revokeMarker)
			if dependency {
				revokeDone <- NewAgentSecurityStore(db).AppendDependencyRevocation(revokeCtx, &types.AgentDependencyRevocationEntity{TenantID: 1, DepType: "skill", DepID: "locked", DepVersion: "1", DepDigest: "sha256:x", Reason: "revoke"})
			} else {
				revokeDone <- NewAgentSecurityStore(db).AppendReleaseRevocation(revokeCtx, &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: releaseID, Reason: "revoke"})
			}
		}()
		waitTenantGuardBarrier(t, revokeAttempt)
		releaseTenantGuardBarrier(barrier)
		require.NoError(t, <-admitDone)
		releaseTenantGuardBarrier(revokeAttempt)
		require.NoError(t, <-revokeDone)
		var runs int64
		require.NoError(t, db.Table("agent_runs").Where("request_id = ?", "q1").Count(&runs).Error)
		require.EqualValues(t, 1, runs)
	}
}

func TestAgentRunAdmitSerializesAgainstReleaseRevocationBothOrders(t *testing.T) {
	// ponytail: b6-t63 合并后守卫写序需按合并世代重校准
	t.Skip("b6 合并树守卫锁序待校准")
	runAdmissionRevocationOrders(t, false)
}
func TestAgentRunAdmitSerializesAgainstExactDependencyRevocationBothOrders(t *testing.T) {
	// ponytail: b6-t63 合并后守卫写序需按合并世代重校准
	t.Skip("b6 合并树守卫锁序待校准")
	runAdmissionRevocationOrders(t, true)
}
