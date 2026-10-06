package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable pins the
// migration↔projection alignment for T32 (#62): the lineage columns exist
// on both marketplace tables and the license registry table is present
// after the full migration stream (same pattern as the workbench
// notifications alignment test from #34).
func TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	var count int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_release_submissions WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_releases WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Exec(`SELECT 1 FROM agent_licenses`).Error)
}

// ---------- 下层证据（服务层）：fork 判定与再分发门 ----------

func lineageSnapshot(t *testing.T, id, agentID, prompt string, localBindings bool) (versionID, snapshotJSON string) {
	t.Helper()
	config := map[string]any{"agent_mode": "quick-answer", "system_prompt": prompt}
	if localBindings {
		config["knowledge_bases"] = []string{"kb-sales"}
		config["model_id"] = "gpt-x"
	}
	raw, err := json.Marshal(map[string]any{"id": agentID, "name": "Sales Assistant", "config": config})
	require.NoError(t, err)
	return id, string(raw)
}

// seedFrozenAgentVersion 插入一条冻结版本行。version_number 按
// (tenant, agent) 自动取下一号：uq_agent_versions_scope 唯一索引
// （000108_agent_versions.up.sql）不允许同一 agent 的同号版本重复，
// 而本文件的派生链需要对同一本地 agent 先后播种映射版与 fork 版。
func seedFrozenAgentVersion(t *testing.T, db *gorm.DB, id, agentID, snapshot string) {
	t.Helper()
	var next int64
	require.NoError(t, db.Raw(
		`SELECT COALESCE(MAX(version_number), 0) + 1 FROM agent_versions WHERE tenant_id = 1 AND agent_id = ?`,
		agentID,
	).Scan(&next).Error)
	sum := sha256.Sum256([]byte(snapshot))
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, 1, ?, ?, ?, ?, 'admin')`,
		id, agentID, next, snapshot, hex.EncodeToString(sum[:]),
	).Error)
}

// seedLineageSource publishes a REAL tenant release (listing+submission+
// release rows via the real repository, mirroring the #60 service-test
// seeding) whose manifest declares licenseID and whose payload carries
// systemPrompt. Returns the listing and release ids.
func seedLineageSource(t *testing.T, db *gorm.DB, suffix, licenseID, systemPrompt string) (string, string) {
	t.Helper()
	repo := repository.NewAgentMarketplaceRepository(db)
	payload := map[string]any{"agent_mode": "quick-answer", "system_prompt": systemPrompt, "allowed_tools": []string{}}
	manifest := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Source " + suffix, "summary": "Source",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"minimum_weknora_capability": "1", "license_id": licenseID,
		"source": map[string]any{"agent_version_id": "version-src-" + suffix, "version_number": 1, "source_sha256": "sha"},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(t, err)
	bundle, err := json.Marshal(map[string]any{
		"payload": json.RawMessage(payloadJSON), "manifest": json.RawMessage(manifestJSON),
		"dependency_lock": map[string]any{"dependencies": []any{}},
	})
	require.NoError(t, err)
	sum := sha256.Sum256(bundle)

	seedFrozenAgentVersion(t, db, "version-src-"+suffix, "agent-src-"+suffix,
		`{"id":"agent-src-`+suffix+`","name":"Source","config":{"agent_mode":"quick-answer","system_prompt":"`+systemPrompt+`"}}`)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-src-" + suffix, DisplayName: "Source " + suffix, Summary: "Source", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-src-" + suffix, SourceAgentID: "agent-src-" + suffix,
			AuthorID: "admin", SemanticVersion: "1.0.0", BundleDigest: hex.EncodeToString(sum[:]),
			ManifestJSON: string(manifestJSON), DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, submission.BundleDigest,
		types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	return submission.ListingID, release.ID
}

func newLineageMarketplaceService(t *testing.T, db *gorm.DB) *AgentMarketplaceService {
	t.Helper()
	customAgents := NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	return NewAgentMarketplaceService(versions, marketplaceResolverFake{}, repository.NewAgentMarketplaceRepository(db), t.TempDir())
}

func seedPublishedVariantAgent(t *testing.T, db *gorm.DB, listingID, releaseID, variantID, localAgentID string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-" + variantID, TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: variantID, TenantID: 1, AdoptionID: "adopt-" + variantID, ReleaseID: releaseID,
		Name: "Sales Assistant", State: "published", LocalAgentID: localAgentID,
	}).Error)
}

func lineageMetadataFor(licenseID, changeNotes string) interfaces.SubmitReleaseInput {
	return interfaces.SubmitReleaseInput{Metadata: types.ReleaseMetadata{
		SemanticVersion: "2.0.0", DisplayName: "Derived helper", Summary: "Derived",
		SupportedLanguages: []string{"en"}, UseCases: []string{"support"},
		MinimumWeKnoraCapability: "1", LicenseID: licenseID, ChangeNotes: changeNotes,
	}}
}

// AC1（下层）：纯映射派生（本地绑定不同、可移植核心相同）→ is_fork=false
// 且 lineage 完整；改可移植核心 → is_fork=true。基线是发布管线重投影，
// 不是源 Release 原始载荷（差异记录 #3）。
func TestSubmitReleaseLineageVerdict(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	repo := repository.NewAgentMarketplaceRepository(db)
	_, err0 := repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: true, CreatedBy: "admin",
	})
	require.NoError(t, err0)
	listingID, releaseID := seedLineageSource(t, db, "mit", "MIT", "Be portable.")
	seedPublishedVariantAgent(t, db, listingID, releaseID, "variant-1", "agent-local")
	svc := newLineageMarketplaceService(t, db)

	// 纯映射派生：本地绑定（knowledge_bases/model_id）在快照里，可移植核心不变。
	mappingOnly, snapshot := lineageSnapshot(t, "version-local", "agent-local", "Be portable.", true)
	seedFrozenAgentVersion(t, db, mappingOnly, "agent-local", snapshot)
	view, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("MIT", "re-mapped knowledge binding"))
	require.NoError(t, err)
	require.False(t, view.IsFork, "纯映射修改绝不判为 Fork（AC1）")
	require.Equal(t, listingID, view.ForkSourceListingID)
	require.Equal(t, releaseID, view.ForkSourceReleaseID)
	require.Equal(t, "MIT", view.LineageLicenseID)
	require.Equal(t, "re-mapped knowledge binding", view.ForkNotes, "修改说明 = ChangeNotes")
	require.Contains(t, view.ManifestJSON, `"lineage"`)
	require.Contains(t, view.ManifestJSON, `"is_fork":false`)

	// Fork：改可移植核心（system prompt）。
	forked, forkSnapshot := lineageSnapshot(t, "version-fork", "agent-local", "Be portable, but sharper.", true)
	seedFrozenAgentVersion(t, db, forked, "agent-local", forkSnapshot)
	forkView, err := svc.SubmitRelease(context.Background(), 1, "admin", forked, lineageMetadataFor("MIT", "sharper prompt"))
	require.NoError(t, err)
	require.True(t, forkView.IsFork)
	require.Contains(t, forkView.ManifestJSON, `"is_fork":true`)

	// 原始内容（无 variant 台账）：无 lineage、不查注册表、manifest 无 lineage 段。
	original, originalSnapshot := lineageSnapshot(t, "version-orig", "agent-orig", "Brand new.", false)
	seedFrozenAgentVersion(t, db, original, "agent-orig", originalSnapshot)
	origView, err := svc.SubmitRelease(context.Background(), 1, "admin", original, lineageMetadataFor("unregistered-raw", ""))
	require.NoError(t, err, "原始内容提交不查许可证注册表（差异记录 #5）")
	require.False(t, origView.IsFork)
	require.Empty(t, origView.ForkSourceListingID)
	require.Empty(t, origView.LineageLicenseID)
	require.NotContains(t, origView.ManifestJSON, `"lineage"`)
}

// AC2（下层）：来源许可证禁止/未注册 → 服务端拒绝；许可证翻转后 live 生效；
// 损坏 lineage 数据 fail closed。
func TestSubmitReleaseRedistributionGate(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	repo := repository.NewAgentMarketplaceRepository(db)
	_, err0 := repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "tenant-private", Name: "Internal", AllowsRedistribution: false, CreatedBy: "admin",
	})
	require.NoError(t, err0)
	svc := newLineageMarketplaceService(t, db)

	// 已注册但禁止再分发。
	listingID, releaseID := seedLineageSource(t, db, "private", "tenant-private", "Be portable.")
	seedPublishedVariantAgent(t, db, listingID, releaseID, "variant-p", "agent-private")
	mappingOnly, snapshot := lineageSnapshot(t, "version-private", "agent-private", "Be portable.", true)
	seedFrozenAgentVersion(t, db, mappingOnly, "agent-private", snapshot)
	_, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseRedistributionForbidden, "禁止再分发时派生 Submission 被拒（AC2），与 fork 判定无关")
	require.Contains(t, err.Error(), "tenant-private")

	// 未注册许可证：fail closed。
	ghostListing, ghostRelease := seedLineageSource(t, db, "ghost", "ghost-license", "Be portable.")
	seedPublishedVariantAgent(t, db, ghostListing, ghostRelease, "variant-g", "agent-ghost")
	ghostVersion, ghostSnapshot := lineageSnapshot(t, "version-ghost", "agent-ghost", "Be portable.", true)
	seedFrozenAgentVersion(t, db, ghostVersion, "agent-ghost", ghostSnapshot)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", ghostVersion, lineageMetadataFor("ghost-license", ""))
	require.ErrorIs(t, err, ErrReleaseRedistributionForbidden)
	require.Contains(t, err.Error(), "not registered")

	// 许可证翻转后 live 放行（同一派生链）。
	_, err0 = repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "tenant-private", Name: "Internal", AllowsRedistribution: true, CreatedBy: "admin",
	})
	require.NoError(t, err0)
	allowed, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.NoError(t, err)
	require.False(t, allowed.IsFork)

	// 损坏的源 bundle：fail closed，绝不静默放行（Review Focus 5）。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("id = ?", releaseID).
		Update("bundle", []byte(`{broken`)).Error)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseLineageUnavailable)

	// variant 指向缺失 Release：fail closed。（复用既有 adoption 行：
	// agent_adoptions 受 (tenant_id, listing_id) 唯一索引约束，同一 listing
	// 不能再建第二条 adoption；断言语义不变——variant 的 ReleaseID 指向
	// 不存在的 Release 即可。）
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-dangling", TenantID: 1, AdoptionID: "adopt-variant-p", ReleaseID: "release-missing",
		Name: "Dangling", State: "published", LocalAgentID: "agent-dangling",
	}).Error)
	dangling, danglingSnapshot := lineageSnapshot(t, "version-dangling", "agent-dangling", "Be portable.", true)
	seedFrozenAgentVersion(t, db, dangling, "agent-dangling", danglingSnapshot)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", dangling, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseLineageUnavailable)
}

// 许可证注册表管理：校验与翻转。
func TestRegisterLicenseValidationAndList(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	svc := newLineageMarketplaceService(t, db)

	_, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "  "})
	require.ErrorIs(t, err, ErrAgentLicenseInvalid)
	_, err = svc.RegisterLicense(context.Background(), "", interfaces.LicenseInput{ID: "MIT"})
	require.ErrorIs(t, err, ErrAgentLicenseInvalid)

	created, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "MIT", Name: "MIT", AllowsRedistribution: true})
	require.NoError(t, err)
	require.True(t, created.AllowsRedistribution)
	flipped, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "MIT", Name: "MIT", AllowsRedistribution: false})
	require.NoError(t, err)
	require.False(t, flipped.AllowsRedistribution)

	rows, err := svc.ListLicenses(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
