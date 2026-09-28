package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestAgentChatTurnClaimEntityUsesTenantScopedKeysAndRevocationIndex(t *testing.T) {
	parsed, err := schema.Parse(&types.AgentChatTurnClaimEntity{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	primary := make([]string, 0, len(parsed.PrimaryFields))
	for _, field := range parsed.PrimaryFields {
		primary = append(primary, field.DBName)
	}
	require.Equal(t, []string{"id", "source_tenant_id"}, primary)
	indexes := make(map[string][]string)
	for _, index := range parsed.ParseIndexes() {
		fields := make([]string, 0, len(index.Fields))
		for _, field := range index.Fields {
			fields = append(fields, field.DBName)
		}
		indexes[index.Name] = fields
	}
	require.Equal(t, []string{"session_tenant_id", "assistant_message_id"}, indexes["uq_agent_chat_turn_claim_session_assistant"])
	require.Equal(t, []string{"session_tenant_id", "user_message_id"}, indexes["uq_agent_chat_turn_claim_session_user"])
	require.Equal(t, []string{"source_tenant_id", "state", "release_id"}, indexes["idx_agent_chat_turn_claim_source_state_release"])
}

func TestAgentChatTurnClaimIsTenantScopedAndIdempotent(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "claim-agent", TenantID: 1, Name: "claim agent"}).Error)
	store := NewAgentChatTurnClaimRepository(db)
	input := AgentChatTurnClaimInput{
		SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1",
		RequestID: "request-1", RequestHash: "hash-1", LeaseOwner: "worker-1", AgentID: "claim-agent",
		AssistantPlaceholder: &types.Message{Content: "", IsCompleted: false},
	}
	first, replay, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, ReplayNew, replay)
	require.NotEmpty(t, first.AssistantMessageID)

	second, replay, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, ReplayActive, replay)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.AssistantMessageID, second.AssistantMessageID)
	require.Equal(t, first.Generation, second.Generation)
	require.Equal(t, first.LeaseOwner, second.LeaseOwner)
	observed, leaseValid, err := store.GetForHandler(context.Background(), 1, first.ID, first.Generation, first.LeaseOwner)
	require.NoError(t, err)
	require.True(t, leaseValid)
	require.Equal(t, "active", observed.State)
	var placeholder types.Message
	require.NoError(t, db.Table("messages").Where("id = ?", first.AssistantMessageID).Take(&placeholder).Error)
	finished, err := store.Finish(context.Background(), 1, first.ID, first.Generation, first.LeaseOwner, &types.Message{Content: "done", IsCompleted: true}, "completed", "")
	require.NoError(t, err)
	require.True(t, finished)
	finished, err = store.Finish(context.Background(), 1, first.ID, first.Generation, first.LeaseOwner, &types.Message{Content: "late", IsCompleted: true}, "completed", "")
	require.ErrorIs(t, err, ErrAgentChatTurnClaimFenced)
	require.False(t, finished)
	terminal, replay, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, ReplayTerminal, replay)
	require.Equal(t, first.AssistantMessageID, terminal.AssistantMessageID)
	terminal, leaseValid, err = store.GetForHandler(context.Background(), 1, first.ID, first.Generation, first.LeaseOwner)
	require.NoError(t, err)
	require.False(t, leaseValid)
	require.Equal(t, "completed", terminal.State)
	_, _, err = store.GetForHandler(context.Background(), 2, first.ID, first.Generation, first.LeaseOwner)
	require.Error(t, err, "a different source tenant cannot read a claim")

	input.RequestHash = "changed-hash"
	_, _, err = store.Admit(context.Background(), input)
	require.ErrorIs(t, err, ErrAgentChatTurnClaimConflict)
	var claims, assistants int64
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Count(&claims).Error)
	require.NoError(t, db.Model(&types.Message{}).Where("request_id = ? AND role = 'assistant'", "request-1").Count(&assistants).Error)
	require.EqualValues(t, 1, claims)
	require.EqualValues(t, 1, assistants)
}

func TestAgentChatTurnClaimExpiryAndOwnerCancellationFenceMessages(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "claim-agent-expiry", TenantID: 1, Name: "claim agent"}).Error)
	store := NewAgentChatTurnClaimRepository(db)
	input := AgentChatTurnClaimInput{SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "old", RequestHash: "h1", LeaseOwner: "worker-1", AgentID: "claim-agent-expiry", AssistantPlaceholder: &types.Message{}}
	old, _, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Where("id=?", old.ID).Update("lease_expires_at", "2000-01-01 00:00:00").Error)
	input.RequestID = "new"
	input.RequestHash = "h2"
	input.LeaseOwner = "worker-2"
	newClaim, _, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.NotEqual(t, old.ID, newClaim.ID)
	var expired types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id=?", old.ID).Take(&expired).Error)
	require.Equal(t, "failed", expired.State)
	require.EqualValues(t, 2, expired.Generation)
	var placeholder types.Message
	require.NoError(t, db.Where("id=?", old.AssistantMessageID).Take(&placeholder).Error)
	require.True(t, placeholder.IsCompleted)
	_, _, err = store.CancelByOwner(context.Background(), 1, "s1", "intruder", newClaim.AssistantMessageID, "stop")
	require.Error(t, err)
	current, valid, err := store.GetForHandler(context.Background(), 1, newClaim.ID, newClaim.Generation, newClaim.LeaseOwner)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "active", current.State)
	_, changed, err := store.CancelByOwner(context.Background(), 1, "s1", "u1", newClaim.AssistantMessageID, "stop")
	require.NoError(t, err)
	require.True(t, changed)
	current, valid, err = store.GetForHandler(context.Background(), 1, newClaim.ID, newClaim.Generation, newClaim.LeaseOwner)
	require.NoError(t, err)
	require.False(t, valid)
	require.Equal(t, "cancelled", current.State)
	require.Equal(t, "stop", current.Reason)
}

func TestAgentChatTurnClaimMissingPlaceholderRollsBackOwnerCancellation(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "claim-agent-owner-missing", TenantID: 1, Name: "claim agent"}).Error)
	store := NewAgentChatTurnClaimRepository(db)
	claim, _, err := store.Admit(context.Background(), AgentChatTurnClaimInput{SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "owner-missing", RequestHash: "hash", LeaseOwner: "worker", AgentID: "claim-agent-owner-missing", AssistantPlaceholder: &types.Message{}})
	require.NoError(t, err)
	require.NoError(t, db.Where("id = ?", claim.AssistantMessageID).Delete(&types.Message{}).Error)
	require.NoError(t, db.Create(&types.Message{ID: claim.AssistantMessageID, SessionID: "other-session", RequestID: "other-request", Role: "assistant"}).Error)
	_, changed, err := store.CancelByOwner(context.Background(), 1, "s1", "u1", claim.AssistantMessageID, "stop")
	require.Error(t, err, "missing tenant/session-bound placeholder must abort owner cancellation")
	require.False(t, changed)
	var persisted types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ? AND source_tenant_id = ?", claim.ID, 1).Take(&persisted).Error)
	require.Equal(t, "active", persisted.State)
	require.EqualValues(t, claim.Generation, persisted.Generation)
}

func TestAgentChatTurnClaimMissingExpiredPlaceholderRollsBackAdmission(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "claim-agent-expired-missing", TenantID: 1, Name: "claim agent"}).Error)
	store := NewAgentChatTurnClaimRepository(db)
	input := AgentChatTurnClaimInput{SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "expired-missing", RequestHash: "old-hash", LeaseOwner: "worker-1", AgentID: "claim-agent-expired-missing", AssistantPlaceholder: &types.Message{}}
	old, _, err := store.Admit(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, db.Where("id = ?", old.AssistantMessageID).Delete(&types.Message{}).Error)
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Where("id = ?", old.ID).Update("lease_expires_at", "2000-01-01 00:00:00").Error)
	input.RequestID, input.RequestHash, input.LeaseOwner = "new-request", "new-hash", "worker-2"
	_, _, err = store.Admit(context.Background(), input)
	require.Error(t, err, "missing expired placeholder must abort cleanup and new admission")
	var persisted types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ? AND source_tenant_id = ?", old.ID, 1).Take(&persisted).Error)
	require.Equal(t, "active", persisted.State)
	var count int64
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Count(&count).Error)
	require.EqualValues(t, 1, count, "new claim must roll back with failed expiry cleanup")
}

func TestAgentChatTurnClaimTerminalTransitionIsSingleWinner(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{ID: "claim-agent-race", TenantID: 1, Name: "claim agent"}).Error)
	store := NewAgentChatTurnClaimRepository(db)
	claim, _, err := store.Admit(context.Background(), AgentChatTurnClaimInput{SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "race", RequestHash: "hash", LeaseOwner: "worker", AgentID: "claim-agent-race", AssistantPlaceholder: &types.Message{}})
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, finishErr := store.Finish(context.Background(), 1, claim.ID, claim.Generation, claim.LeaseOwner, &types.Message{Content: "done", IsCompleted: true}, "completed", "")
			results <- finishErr
		}()
	}
	wg.Wait()
	close(results)
	winners, fenced := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, ErrAgentChatTurnClaimFenced) {
			fenced++
		} else {
			t.Errorf("unexpected finish error: %v", err)
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, fenced)
}

func TestAgentChatTurnClaimCancelMatchesOnlyPinnedReleases(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	for i, release := range []string{"release-hit", "release-other"} {
		request := fmt.Sprintf("cancel-%d", i)
		msg := types.Message{SessionID: "s1", RequestID: request, Role: "assistant"}
		require.NoError(t, db.Create(&msg).Error)
		version := fmt.Sprintf("version-%d", i)
		releaseID := release
		require.NoError(t, db.Create(&types.AgentChatTurnClaimEntity{ID: fmt.Sprintf("claim-%d", i), SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: request, RequestHash: "hash", AssistantMessageID: msg.ID, AgentID: fmt.Sprintf("agent-%d", i), LocalAgentVersionID: &version, ReleaseID: &releaseID, State: "active", LeaseOwner: "worker", LeaseExpiresAt: time.Now().Add(time.Minute), Generation: 1}).Error)
	}
	err := store.AppendReleaseRevocationWithAuditAndCancelClaims(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: "release-hit", Reason: "compromised", InFlightDisposition: "cancel"}, &types.AuditLog{TenantID: 1, Action: types.AuditActionAgentReleaseRevoked, ActorUserID: "u1"})
	require.NoError(t, err)
	var hit, other types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id=?", "claim-0").Take(&hit).Error)
	require.NoError(t, db.Where("id=?", "claim-1").Take(&other).Error)
	require.Equal(t, "cancelled", hit.State)
	require.EqualValues(t, 2, hit.Generation)
	require.Equal(t, "active", other.State)
	var terminalMessage types.Message
	require.NoError(t, db.Where("id=?", hit.AssistantMessageID).Take(&terminalMessage).Error)
	require.True(t, terminalMessage.IsCompleted)
}

func TestAgentChatTurnClaimRevocationMissingPlaceholderRollsBackTogether(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	versionID, releaseID := "missing-placeholder-version", "missing-placeholder-release"
	claim := types.AgentChatTurnClaimEntity{ID: "claim-without-message", SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "missing-message", RequestHash: "hash", AssistantMessageID: "missing-assistant", AgentID: "agent", LocalAgentVersionID: &versionID, ReleaseID: &releaseID, State: "active", LeaseOwner: "worker", LeaseExpiresAt: time.Now().Add(time.Minute), Generation: 1}
	require.NoError(t, db.Create(&claim).Error)
	err := store.AppendReleaseRevocationWithAuditAndCancelClaims(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: releaseID, Reason: "compromised", InFlightDisposition: "cancel"}, &types.AuditLog{TenantID: 1, Action: types.AuditActionAgentReleaseRevoked, ActorUserID: "u1"})
	require.Error(t, err, "zero-row placeholder transition must roll back the governance transaction")
	var persisted types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ? AND source_tenant_id = ?", claim.ID, claim.SourceTenantID).Take(&persisted).Error)
	require.Equal(t, "active", persisted.State)
	var revocations, audits int64
	require.NoError(t, db.Model(&types.AgentReleaseRevocationEntity{}).Count(&revocations).Error)
	require.NoError(t, db.Model(&types.AuditLog{}).Where("action = ?", types.AuditActionAgentReleaseRevoked).Count(&audits).Error)
	require.Zero(t, revocations)
	require.Zero(t, audits)
}

func TestAgentChatTurnClaimRevocationAndAuditRollbackTogether(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	msg := types.Message{SessionID: "s1", RequestID: "rollback", Role: "assistant"}
	require.NoError(t, db.Create(&msg).Error)
	version, release := "version-r", "release-r"
	claim := types.AgentChatTurnClaimEntity{ID: "claim-rollback", SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "rollback", RequestHash: "hash", AssistantMessageID: msg.ID, AgentID: "agent-r", LocalAgentVersionID: &version, ReleaseID: &release, State: "active", LeaseOwner: "worker", LeaseExpiresAt: time.Now().Add(time.Minute), Generation: 1}
	require.NoError(t, db.Create(&claim).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_claim_placeholder_update BEFORE UPDATE OF is_completed ON messages BEGIN SELECT RAISE(ABORT,'placeholder update unavailable'); END`).Error)
	err := store.AppendReleaseRevocationWithAuditAndCancelClaims(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: release, Reason: "reason", InFlightDisposition: "cancel"}, &types.AuditLog{TenantID: 1, Action: types.AuditActionAgentReleaseRevoked, ActorUserID: "u1"})
	require.Error(t, err)
	var stored types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id=?", claim.ID).Take(&stored).Error)
	require.Equal(t, "active", stored.State)
	require.EqualValues(t, 1, stored.Generation)
	var storedMessage types.Message
	require.NoError(t, db.Where("id=?", msg.ID).Take(&storedMessage).Error)
	require.False(t, storedMessage.IsCompleted)
	var revocations int64
	require.NoError(t, db.Model(&types.AgentReleaseRevocationEntity{}).Where("tenant_id=? AND release_id=?", 1, release).Count(&revocations).Error)
	require.Zero(t, revocations)
}

func TestAgentChatTurnClaimAllowLeavesClaimActiveAndCompletesObligation(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentSecurityStore(db)
	version, release := "version-allow", "release-allow"
	claim := types.AgentChatTurnClaimEntity{ID: "claim-allow", SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1", RequestID: "allow", RequestHash: "hash", AssistantMessageID: "assistant-allow", AgentID: "agent-allow", LocalAgentVersionID: &version, ReleaseID: &release, State: "active", LeaseOwner: "worker", LeaseExpiresAt: time.Now().Add(time.Minute), Generation: 1}
	require.NoError(t, db.Create(&claim).Error)
	err := store.AppendReleaseRevocationWithAuditAndCancelClaims(context.Background(), &types.AgentReleaseRevocationEntity{TenantID: 1, ReleaseID: release, Reason: "allow in flight", InFlightDisposition: "allow"}, &types.AuditLog{TenantID: 1, Action: types.AuditActionAgentReleaseRevoked, ActorUserID: "u1"})
	require.NoError(t, err)
	var stored types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id=?", claim.ID).Take(&stored).Error)
	require.Equal(t, "active", stored.State)
	require.EqualValues(t, 1, stored.Generation)
	var revocation types.AgentReleaseRevocationEntity
	require.NoError(t, db.Where("tenant_id=? AND release_id=?", 1, release).Take(&revocation).Error)
	require.Equal(t, "complete", revocation.RunCancellationState)
	require.Zero(t, revocation.CanceledRunCount)
}
