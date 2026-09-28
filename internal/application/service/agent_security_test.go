package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const securityManifest = `{"semantic_version":"%s","display_name":"Sec","summary":"s","supported_languages":["en"],"use_cases":["u"],"capability_requirements":[],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"v","version_number":1,"source_sha256":"sha"}}`

const securityBundle = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"p"},"manifest":` + securityManifest + `,"dependency_lock":{"dependencies":[]}}`

const lockV123 = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}`
const lockV124 = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.4","digest":"D2","license_id":"MIT"}]}`
const lockV123OtherDigest = `{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D9","license_id":"MIT"}]}`

// securityBundleWithLock 让 bundle 的 dependency_lock 与播种锁一致（bundle
// 字节只作迁移占位，判定面只读 releases.dependency_lock_json 列）。
func securityBundleWithLock(lock string) string {
	return `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"p"},"manifest":` + securityManifest + `,"dependency_lock":` + lock + `}`
}

func newAgentSecurityServiceForTest(t *testing.T) (*AgentSecurityService, *repository.AgentSecurityStore, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	store := repository.NewAgentSecurityStore(db)
	return NewAgentSecurityService(store, repository.NewAgentRunStore(db)), store, db
}

// publishSecurityVariant 经真实仓储播种一个 published Variant 并挂本地 agent。
func publishSecurityVariant(t *testing.T, db *gorm.DB, adoption *types.AgentAdoptionEntity, releaseID, name, localAgentID string) *types.AgentAdoptionVariantEntity {
	t.Helper()
	adoptions := repository.NewAgentAdoptionRepository(db)
	variant, err := adoptions.CreateVariant(context.Background(), &types.AgentAdoptionVariantEntity{
		TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: name, State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	updated, err := adoptions.UpdateVariantState(context.Background(), 1, variant.ID, []string{"draft"}, "published", map[string]any{
		"local_agent_id": localAgentID, "local_agent_version_id": "ver-" + localAgentID, "published_by": "admin"})
	require.NoError(t, err)
	return updated
}

func TestAgentSecurityVerdictBlocksRevokedReleaseAndPassesUnaffected(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r2 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	blockedVariant := publishSecurityVariant(t, db, adoption, r1, "On r1", "local-agent-r1")
	publishSecurityVariant(t, db, adoption, r2, "On r2", "local-agent-r2")
	ctx := context.Background()

	verdict, err := svc.VerdictForAgent(ctx, 1, "local-agent-r1")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, verdict.State, "撤回前一切照常")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r1))
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r2))

	require.NoError(t, store.AppendReleaseRevocation(ctx, &types.AgentReleaseRevocationEntity{
		TenantID: 1, ListingID: listingID, ReleaseID: r1,
		Reason: "CVE-2026-0001 prompt exfiltration", ReplacementReleaseID: r2, RevokedBy: "sec-admin"}))

	verdict, err = svc.VerdictForAgent(ctx, 1, "local-agent-r1")
	require.NoError(t, err)
	require.True(t, verdict.Blocked())
	require.Equal(t, interfaces.AgentSecurityVerdictReleaseRevoked, verdict.State)
	require.Contains(t, verdict.Reason, "CVE-2026-0001")
	require.Equal(t, r2, verdict.ReplacementReleaseID, "替代版本随判定可达")
	require.NotEmpty(t, verdict.RevocationID)

	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, r1), ErrAgentSecurityReleaseBlocked, "撤回 Release 拒绝新引入/新变体（治理面准入）")

	clean, err := svc.VerdictForAgent(ctx, 1, "local-agent-r2")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, clean.State, "未受影响 Variant 照常")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r2))

	nonAdoption, err := svc.VerdictForAgent(ctx, 1, "agent-a")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, nonAdoption.State, "非 adoption 派生 agent 不受治理")
	_ = blockedVariant
}

func TestRevokeReleaseRecordsAuditScopeAndCancelsRuns(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r2 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, `{"dependencies":[]}`, securityBundleWithLock(`{"dependencies":[]}`))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	publishSecurityVariant(t, db, adoption, r1, "V1", "local-agent-r1")
	publishSecurityVariant(t, db, adoption, r2, "V2", "local-agent-r2")
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'security test', 'u1', 'trpc')`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-live-1' WHERE tenant_id = 1 AND id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-live-1', 's1', 'u1', 'req-1', 'm1', 'h1', 'trpc', 'running', ?, datetime('now','+1 hour'))`,
		`{"session_id":"s1","agent_id":"local-agent-r1","request_id":"req-1","text":"hi"}`).Error)

	view, err := svc.RevokeRelease(context.Background(), 1, "sec-admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "CVE-2026-0001", ReplacementReleaseID: r2})
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityRevocationKindRelease, view.Kind)
	require.Equal(t, "cancel", view.InFlightDisposition)
	require.EqualValues(t, 1, view.CanceledRunCount)
	require.Equal(t, listingID, view.ListingID)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'sec-live-1'`).Scan(&status).Error)
	require.Equal(t, "canceled", status)
	var audit types.AuditLog
	require.NoError(t, db.Where("tenant_id = ? AND action = ?", 1, types.AuditActionAgentReleaseRevoked).Take(&audit).Error)
	require.Equal(t, "sec-admin", audit.ActorUserID)
	require.Equal(t, "marketplace", audit.ScopeType)
	require.Equal(t, "agent_release", audit.TargetType)
	require.Equal(t, r1, audit.TargetID)
	require.JSONEq(t, `{"reason":"CVE-2026-0001","replacement":"`+r2+`","in_flight_disposition":"cancel"}`, string(audit.Details))

	detail, err := svc.GetRevocation(context.Background(), 1, view.ID)
	require.NoError(t, err)
	require.Len(t, detail.Scope.BlockedReleases, 1)
	require.Equal(t, r1, detail.Scope.BlockedReleases[0].ReleaseID)
	require.Equal(t, []string{adoption.ID}, detail.Scope.AffectedAdoptionIDs)
	require.Len(t, detail.Scope.AffectedVariants, 1)
	require.Equal(t, "local-agent-r1", detail.Scope.AffectedVariants[0].LocalAgentID)
	list, err := svc.ListRevocations(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Nil(t, list[0].Scope)
}

func TestRevokeDependencyUsesExactLockIdentityForScopeAndCancellation(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r123 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r124 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	adoption := adoptUpgradeRelease(t, db, listingID, r123)
	publishSecurityVariant(t, db, adoption, r123, "v123", "local-agent-v123")
	publishSecurityVariant(t, db, adoption, r124, "v124", "local-agent-v124")
	view, err := svc.RevokeDependency(context.Background(), 1, "sec-admin", interfaces.DependencyRevocationInput{
		Dependency: types.AgentReleaseDependency{Type: "skill", ID: "web-search", Version: "1.2.3", Digest: "D1"},
		Reason:     "supply-chain compromise", ReplacementVersion: "1.2.4", InFlightDisposition: interfaces.AgentSecurityInFlightAllow})
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityRevocationKindDependency, view.Kind)
	require.Equal(t, "1.2.4", view.ReplacementVersion)
	detail, err := svc.GetRevocation(context.Background(), 1, view.ID)
	require.NoError(t, err)
	require.Len(t, detail.Scope.BlockedReleases, 1)
	require.Equal(t, r123, detail.Scope.BlockedReleases[0].ReleaseID)
	require.Equal(t, interfaces.AgentSecurityRevocationKindDependency, detail.Scope.BlockedReleases[0].BlockedBy)
	require.Len(t, detail.Scope.AffectedVariants, 1)
	require.ErrorIs(t, svc.ReleaseAdmission(context.Background(), 1, r123), ErrAgentSecurityReleaseBlocked)
	require.NoError(t, svc.ReleaseAdmission(context.Background(), 1, r124))
	_, err = svc.GetRevocation(context.Background(), 2, view.ID)
	require.ErrorIs(t, err, ErrAgentSecurityNotFound)
}

func TestRevokeRejectsMalformedInputWithoutWrites(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	_, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, err := svc.RevokeRelease(context.Background(), 1, "admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: " "})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput)
	_, err = svc.RevokeRelease(context.Background(), 1, "admin", interfaces.ReleaseRevocationInput{ReleaseID: "missing", Reason: "x"})
	require.ErrorIs(t, err, ErrAgentSecurityReleaseUnresolvable)
	_, err = svc.RevokeRelease(context.Background(), 1, "admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "x", InFlightDisposition: "maybe"})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput)
	_, err = svc.RevokeRelease(context.Background(), 1, "admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "x", ReplacementReleaseID: "missing"})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput)
	_, err = svc.RevokeDependency(context.Background(), 1, "admin", interfaces.DependencyRevocationInput{Dependency: types.AgentReleaseDependency{Type: "skill", ID: "x", Version: "1"}, Reason: "x"})
	require.ErrorIs(t, err, ErrAgentSecurityInvalidInput)
	var releases, dependencies, audits int64
	require.NoError(t, db.Model(&types.AgentReleaseRevocationEntity{}).Count(&releases).Error)
	require.NoError(t, db.Model(&types.AgentDependencyRevocationEntity{}).Count(&dependencies).Error)
	require.NoError(t, db.Model(&types.AuditLog{}).Where("action LIKE ?", "agent_security.%").Count(&audits).Error)
	require.Zero(t, releases)
	require.Zero(t, dependencies)
	require.Zero(t, audits)
}

func TestRevokeReleaseAllowLeavesActiveRunsAndDuplicateAppendsHistory(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	listingID, r1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	adoption := adoptUpgradeRelease(t, db, listingID, r1)
	publishSecurityVariant(t, db, adoption, r1, "V1", "local-agent-r1")
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'security test', 'u1', 'trpc')`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-live-2' WHERE tenant_id = 1 AND id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-live-2', 's1', 'u1', 'req-2', 'm2', 'h2', 'trpc', 'running', ?, datetime('now','+1 hour'))`,
		`{"session_id":"s1","agent_id":"local-agent-r1","request_id":"req-2","text":"hi"}`).Error)
	first, err := svc.RevokeRelease(context.Background(), 1, "admin", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "reason", InFlightDisposition: interfaces.AgentSecurityInFlightAllow})
	require.NoError(t, err)
	require.Zero(t, first.CanceledRunCount)
	second, err := svc.RevokeRelease(context.Background(), 1, "admin2", interfaces.ReleaseRevocationInput{ReleaseID: r1, Reason: "expanded", InFlightDisposition: interfaces.AgentSecurityInFlightAllow})
	require.NoError(t, err)
	history, err := svc.ListRevocations(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, first.ID, history[0].ID)
	require.Equal(t, second.ID, history[1].ID)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'sec-live-2'`).Scan(&status).Error)
	require.Equal(t, "running", status)
}

// AC1 核心断言：依赖撤回的匹配键是 (type,id,version,digest) 四元组，
// 不是名称——同名不同版本不阻断（不得借同名替换「自愈」），同名同版本
// 不同 digest 仍阻断（digest 是锁定身份的一部分）。
func TestAgentSecurityVerdictDependencyBlockedKeysOnExactLockedIdentity(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, r123 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	_, r124 := publishUpgradeServiceRelease(t, db, 2, "2.0.0", securityManifest, lockV124, securityBundleWithLock(lockV124))
	_, r123d9 := publishUpgradeServiceRelease(t, db, 3, "3.0.0", securityManifest, lockV123OtherDigest, securityBundleWithLock(lockV123OtherDigest))
	adoption := adoptUpgradeRelease(t, db, listingID, r123)
	publishSecurityVariant(t, db, adoption, r123, "v123", "local-agent-v123")
	publishSecurityVariant(t, db, adoption, r124, "v124", "local-agent-v124")
	publishSecurityVariant(t, db, adoption, r123d9, "v123d9", "local-agent-v123d9")
	ctx := context.Background()

	require.NoError(t, store.AppendDependencyRevocation(ctx, &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1",
		Reason: "malicious exfil in pinned skill content", ReplacementVersion: "1.2.4", RevokedBy: "sec-admin"}))

	blocked, err := svc.VerdictForAgent(ctx, 1, "local-agent-v123")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictDependencyBlocked, blocked.State, "锁定被撤回依赖的 Release 传递阻断")
	require.NotNil(t, blocked.Dependency)
	require.Equal(t, "1.2.3", blocked.Dependency.Version)
	require.Equal(t, "D1", blocked.Dependency.Digest)
	require.Equal(t, "1.2.4", blocked.ReplacementVersion)
	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, r123), ErrAgentSecurityReleaseBlocked)

	sameNameNewVersion, err := svc.VerdictForAgent(ctx, 1, "local-agent-v124")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, sameNameNewVersion.State,
		"AC1：依赖不按名称自动替换——同名不同版本/摘要的新 Release 不被阻断也不被顶替")
	require.NoError(t, svc.ReleaseAdmission(ctx, 1, r124))

	sameVersionOtherDigest, err := svc.VerdictForAgent(ctx, 1, "local-agent-v123d9")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, sameVersionOtherDigest.State,
		"digest 是锁定身份的一部分：不同内容不受同名同版本撤回牵连")
}

func TestAgentSecurityVerdictCoversIntroducedRelease(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	ctx := context.Background()
	_, sourceReleaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	require.NoError(t, db.Create(&types.PublicMarketplaceListingEntity{
		ID: "pub-listing-1", PublisherTenantID: 9, SourceListingID: "src-listing-1", DisplayName: "Introduced", State: "listed",
	}).Error)
	require.NoError(t, db.Create(&types.PublicReleaseSubmissionEntity{
		ID: "pub-sub-1", PublisherTenantID: 1, PublicListingID: "pub-listing-1", SourceListingID: "src-listing-1",
		SourceReleaseID: sourceReleaseID, SemanticVersion: "2.0.0", BundleDigest: "d-intro", ManifestJSON: "{}",
		DependencyLockJSON: lockV123, Bundle: []byte("{}"),
	}).Error)
	require.NoError(t, db.Create(&types.PublicAgentReleaseEntity{
		ID: "pub-rel-1", ListingID: "pub-listing-1", SubmissionID: "pub-sub-1", PublisherTenantID: 1,
		ReleaseNumber: 1, SemanticVersion: "2.0.0", BundleDigest: "d-intro", ManifestJSON: "{}",
		DependencyLockJSON: lockV123, Bundle: []byte("{}"),
	}).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_introduced_releases
		(id, tenant_id, public_listing_id, public_release_id, display_name, semantic_version,
		 bundle_digest, manifest_json, dependency_lock_json, bundle, introduced_by, introduced_at)
		VALUES ('intro-r1', 1, 'pub-listing-1', 'pub-rel-1', 'Introduced', '2.0.0', 'd-intro', '{}',
		 '{"dependencies":[{"type":"skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}]}',
		 '{}', 'admin', datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
		VALUES ('a-intro', 1, 'pub-listing-1', 'intro-r1', 'active', 'admin', datetime('now'), datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoption_variants (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, created_by, created_at, updated_at)
		VALUES ('var-intro', 1, 'a-intro', 'intro-r1', 'Intro', 'published', 'local-agent-intro', 'admin', datetime('now'), datetime('now'))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_dependency_revocations
		(id, tenant_id, dep_type, dep_id, dep_version, dep_digest, reason, revoked_by, revoked_at, created_at)
		VALUES ('dep-rev-1', 1, 'skill', 'web-search', '1.2.3', 'D1', 'supply-chain revocation', 'sec-admin', datetime('now'), datetime('now'))`).Error)

	verdict, err := svc.VerdictForAgent(ctx, 1, "local-agent-intro")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictDependencyBlocked, verdict.State,
		"引入式 Release 的锁同样参与传播——只查 agent_releases 会把 #60 引入面漏成放行")
	require.ErrorIs(t, svc.ReleaseAdmission(ctx, 1, "intro-r1"), ErrAgentSecurityReleaseBlocked)

	foreign, err := svc.VerdictForAgent(ctx, 2, "local-agent-intro")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, foreign.State, "跨租户：撤回行不外溢")
}

func TestAgentSecurityVerdictRejectsStructurallyInvalidLocks(t *testing.T) {
	invalidLocks := []struct {
		name string
		json string
	}{
		{name: "null top level", json: `null`},
		{name: "empty object", json: `{}`},
		{name: "missing dependencies", json: `{"other":[]}`},
		{name: "null dependencies", json: `{"dependencies":null}`},
		{name: "malformed json", json: `{"dependencies":`},
		{name: "null dependency", json: `{"dependencies":[null]}`},
		{name: "missing tuple identity", json: `{"dependencies":[{"type":"skill","id":"web-search"}]}`},
	}
	for i, testCase := range invalidLocks {
		t.Run(testCase.name, func(t *testing.T) {
			svc, _, db := newAgentSecurityServiceForTest(t)
			listingID, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0."+string(rune('0'+i)), securityManifest, lockV123, securityBundleWithLock(lockV123))
			adoption := adoptUpgradeRelease(t, db, listingID, releaseID)
			publishSecurityVariant(t, db, adoption, releaseID, "Invalid lock", "local-agent-invalid")
			require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).Update("dependency_lock_json", testCase.json).Error)

			verdict, err := svc.VerdictForAgent(context.Background(), 1, "local-agent-invalid")
			require.Error(t, err)
			require.NotEqual(t, interfaces.AgentSecurityVerdictOK, verdict.State, "a data error must not become an ok verdict")
			require.Error(t, svc.ReleaseAdmission(context.Background(), 1, releaseID))
		})
	}
}

func TestAgentSecurityVerdictAllowsValidEmptyLock(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, `{"dependencies":[]}`, securityBundleWithLock(`{"dependencies":[]}`))
	adoption := adoptUpgradeRelease(t, db, listingID, releaseID)
	publishSecurityVariant(t, db, adoption, releaseID, "Empty lock", "local-agent-empty")
	require.NoError(t, store.AppendDependencyRevocation(context.Background(), &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1", Reason: "revoked", RevokedBy: "sec-admin",
	}))

	verdict, err := svc.VerdictForAgent(context.Background(), 1, "local-agent-empty")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictOK, verdict.State)
	require.NoError(t, svc.ReleaseAdmission(context.Background(), 1, releaseID))
}

func TestAgentSecurityVerdictDependencyTupleMismatchBoundaries(t *testing.T) {
	mismatches := []struct {
		name string
		dep  string
	}{
		{name: "type differs", dep: `{"type":"Skill","id":"web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}`},
		{name: "id differs", dep: `{"type":"skill","id":"Web-search","version":"1.2.3","digest":"D1","license_id":"MIT"}`},
		{name: "version differs", dep: `{"type":"skill","id":"web-search","version":"V1.2.3","digest":"D1","license_id":"MIT"}`},
		{name: "digest differs", dep: `{"type":"skill","id":"web-search","version":"1.2.3","digest":"d1","license_id":"MIT"}`},
	}
	for i, mismatch := range mismatches {
		t.Run(mismatch.name, func(t *testing.T) {
			svc, store, db := newAgentSecurityServiceForTest(t)
			lock := `{"dependencies":[` + mismatch.dep + `]}`
			listingID, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0."+string(rune('0'+i)), securityManifest, lock, securityBundleWithLock(lock))
			adoption := adoptUpgradeRelease(t, db, listingID, releaseID)
			publishSecurityVariant(t, db, adoption, releaseID, mismatch.name, "local-agent-mismatch")
			require.NoError(t, store.AppendDependencyRevocation(context.Background(), &types.AgentDependencyRevocationEntity{
				TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1", Reason: "revoked", RevokedBy: "sec-admin",
			}))

			verdict, err := svc.VerdictForAgent(context.Background(), 1, "local-agent-mismatch")
			require.NoError(t, err)
			require.Equal(t, interfaces.AgentSecurityVerdictOK, verdict.State)
		})
	}
}

func TestAgentSecurityVerdictReleaseRevocationPrecedesDependencyRevocation(t *testing.T) {
	svc, store, db := newAgentSecurityServiceForTest(t)
	listingID, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	adoption := adoptUpgradeRelease(t, db, listingID, releaseID)
	publishSecurityVariant(t, db, adoption, releaseID, "Both revoked", "local-agent-both")
	require.NoError(t, store.AppendDependencyRevocation(context.Background(), &types.AgentDependencyRevocationEntity{
		TenantID: 1, DepType: "skill", DepID: "web-search", DepVersion: "1.2.3", DepDigest: "D1", Reason: "dependency reason", RevokedBy: "sec-admin",
	}))
	require.NoError(t, store.AppendReleaseRevocation(context.Background(), &types.AgentReleaseRevocationEntity{
		TenantID: 1, ListingID: listingID, ReleaseID: releaseID, Reason: "release reason", RevokedBy: "sec-admin",
	}))

	verdict, err := svc.VerdictForAgent(context.Background(), 1, "local-agent-both")
	require.NoError(t, err)
	require.Equal(t, interfaces.AgentSecurityVerdictReleaseRevoked, verdict.State)
	require.Contains(t, verdict.Reason, "release reason")
	require.Nil(t, verdict.Dependency)
}

func TestAgentSecurityReleaseAdmissionUnknownAndForeignRelease(t *testing.T) {
	svc, _, db := newAgentSecurityServiceForTest(t)
	_, releaseID := publishUpgradeServiceRelease(t, db, 1, "1.0.0", securityManifest, lockV123, securityBundleWithLock(lockV123))
	require.ErrorIs(t, svc.ReleaseAdmission(context.Background(), 1, "missing-release"), ErrAgentSecurityReleaseUnresolvable)
	require.ErrorIs(t, svc.ReleaseAdmission(context.Background(), 2, releaseID), ErrAgentSecurityReleaseUnresolvable)
}
