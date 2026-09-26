package service

// Upgrade-diff pure-function tests (T31 #61, AC2): the four review
// dimensions over REAL AgentReleaseEntity rows (real manifests, real
// dependency locks, real bundle envelopes — the same shapes the exporter
// produces). No DB, no mocks of the function under test.

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

const (
	diffManifestV1 = `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	diffManifestV2 = `{"semantic_version":"1.1.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["knowledge","model","sandbox"],"data_categories":["chat_content"],"external_side_effects":["web_search"],"minimum_weknora_capability":"1","license_id":"Apache-2.0","source":{"agent_version_id":"version-b","version_number":2,"source_sha256":"sha"}}`
	diffLockV1     = `{"dependencies":[{"type":"skill","id":"calendar","version":"2.0.0","digest":"bb","license_id":"Apache-2.0"},{"type":"skill","id":"weather","version":"1.0.0","digest":"aa","license_id":"MIT"}]}`
	diffLockV2     = `{"dependencies":[{"type":"skill","id":"calendar","version":"2.1.0","digest":"cc","license_id":"Apache-2.0"},{"type":"skill","id":"translate","version":"1.0.0","digest":"dd","license_id":"MIT"}]}`
	diffBundleV1   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"],"starter_prompts":["Help me"]},"manifest":` + diffManifestV1 + `,"dependency_lock":` + diffLockV1 + `}`
	diffBundleV2   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be extra useful.","allowed_tools":["search","mail"],"starter_prompts":["Help me now"]},"manifest":` + diffManifestV2 + `,"dependency_lock":` + diffLockV2 + `}`
)

func diffRelease(id, manifest, lock, bundle string) *types.AgentReleaseEntity {
	return &types.AgentReleaseEntity{ID: id, ManifestJSON: manifest, DependencyLockJSON: lock, Bundle: []byte(bundle)}
}

func TestDiffUpgradeBundlesCoversAllFourDimensions(t *testing.T) {
	diff, err := diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.NoError(t, err)

	// 行为维：提示词、工具表、开场提示变化；agent_mode/persona 不变 → 不出现。
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "system_prompt", From: "Be useful.", To: "Be extra useful."},
		{Field: "allowed_tools", From: "search", To: "mail,search"},
		{Field: "starter_prompts", From: "Help me", To: "Help me now"},
	}, diff.Behavior)

	// 依赖维：按 (type, id) 对齐且键序确定——calendar 版本+摘要变化，translate 新增，weather 移除。
	require.Equal(t, []types.UpgradeDependencyChange{
		{Type: "skill", ID: "calendar", Change: "version_changed", FromVersion: "2.0.0", ToVersion: "2.1.0"},
		{Type: "skill", ID: "calendar", Change: "digest_changed", FromVersion: "2.0.0", ToVersion: "2.1.0"},
		{Type: "skill", ID: "translate", Change: "added", ToVersion: "1.0.0"},
		{Type: "skill", ID: "weather", Change: "removed", FromVersion: "1.0.0"},
	}, diff.Dependencies)

	// 安全维：能力需求（排序拼接）、数据类别、外部副作用三项变化。
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "capability_requirements", From: "knowledge,model", To: "knowledge,model,sandbox"},
		{Field: "data_categories", From: "", To: "chat_content"},
		{Field: "external_side_effects", From: "", To: "web_search"},
	}, diff.Security)

	// 许可维：依赖许可变化按依赖键序在前（calendar 许可未变 → 不产生条目；
	// translate 新增 → 空 From；weather 移除 → 空 To），Release 许可收尾。
	require.Equal(t, []types.UpgradeLicenseChange{
		{Scope: "dependency", ID: "translate", From: "", To: "MIT"},
		{Scope: "dependency", ID: "weather", From: "MIT", To: ""},
		{Scope: "release", ID: "license_id", From: "MIT", To: "Apache-2.0"},
	}, diff.License)
}

func TestDiffUpgradeBundlesIdenticalReleasesProduceEmptySections(t *testing.T) {
	diff, err := diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
		diffRelease("r1-copy", diffManifestV1, diffLockV1, diffBundleV1),
	)
	require.NoError(t, err)
	require.NotNil(t, diff.Behavior)
	require.Len(t, diff.Behavior, 0)
	require.NotNil(t, diff.Dependencies)
	require.Len(t, diff.Dependencies, 0)
	require.NotNil(t, diff.Security)
	require.Len(t, diff.Security, 0)
	require.NotNil(t, diff.License)
	require.Len(t, diff.License, 0)
}

func TestDiffUpgradeBundlesRejectsMissingReleasesAndMalformedPayload(t *testing.T) {
	_, err := diffUpgradeBundles(nil, diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2))
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)

	_, err = diffUpgradeBundles(diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1), nil)
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)

	// bundle JSON 畸形：fail closed，绝不产出半成品差异。
	_, err = diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, `{"payload":"not-an-object"`),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "r1")

	// manifest JSON 畸形：同样 fail closed。
	_, err = diffUpgradeBundles(
		diffRelease("r1", `{broken`, diffLockV1, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)

	// 依赖锁 JSON 畸形：同样 fail closed。
	_, err = diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, `{broken`, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)
}

func TestDiffUpgradeBundlesIsDeterministic(t *testing.T) {
	run := func() types.AgentUpgradeDiff {
		diff, err := diffUpgradeBundles(
			diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
			diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
		)
		require.NoError(t, err)
		return diff
	}
	first, second := run(), run()
	firstRaw, err := json.Marshal(first)
	require.NoError(t, err)
	secondRaw, err := json.Marshal(second)
	require.NoError(t, err)
	require.JSONEq(t, string(firstRaw), string(secondRaw), "同一对 Release 永远产出相同差异")
}
