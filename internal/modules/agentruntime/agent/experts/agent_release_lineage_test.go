package experts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func lineageTestVersion(prompt string) types.AgentVersionSnapshot {
	return types.AgentVersionSnapshot{
		AgentVersionView: types.AgentVersionView{ID: "version-1"},
		Agent: &types.CustomAgent{
			ID: "agent-1", Name: "Helper",
			Config: types.CustomAgentConfig{AgentMode: "quick-answer", SystemPrompt: prompt},
		},
	}
}

func lineageTestMetadata() types.ReleaseMetadata {
	return types.ReleaseMetadata{
		SemanticVersion: "1.0.1", DisplayName: "Helper", Summary: "A helper",
		SupportedLanguages: []string{"en"}, UseCases: []string{"support"},
		MinimumWeKnoraCapability: "1", LicenseID: "MIT",
	}
}

// AC1 的结构性前提：本地映射绑定（知识库/模型）绝不进入可移植载荷，
// 因此纯映射修改在载荷层面与基线字节相等。
func TestPortablePayloadOfProjectsPortableCoreOnly(t *testing.T) {
	version := lineageTestVersion("Be portable.")
	version.Agent.Config.KnowledgeBases = []string{"kb-sales"}
	version.Agent.Config.ModelID = "gpt-x"

	payload, err := PortablePayloadOf(version.Agent)
	require.NoError(t, err)
	require.Equal(t, "quick-answer", payload.AgentMode)
	require.Equal(t, "Be portable.", payload.SystemPrompt)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "kb-sales", "本地映射绑定绝不进入可移植载荷")
	require.NotContains(t, string(encoded), "gpt-x")
}

// 存量回归卫兵：lineage=nil 时字节与既有 BuildAgentReleaseBundle 完全
// 一致——原始内容的 digest 不漂移（Review Focus 4）。
func TestBuildAgentReleaseBundleWithLineageNilKeepsLegacyBytes(t *testing.T) {
	version := lineageTestVersion("Be portable.")
	lock := types.DependencyLock{}
	legacy, err := BuildAgentReleaseBundle(version, lineageTestMetadata(), lock)
	require.NoError(t, err)
	neutral, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), lock, nil)
	require.NoError(t, err)
	require.Equal(t, legacy.Bytes, neutral.Bytes)
	require.Equal(t, legacy.SHA256, neutral.SHA256)
}

// Fork 的 lineage 段进入 Manifest（digest 边界内），载荷可 round-trip 读回。
// （AgentReleaseBundle.Manifest 是结构体，round-trip 经 JSON 序列化往返；
// bundle.Bytes 是 digest 覆盖的信封字节，lineage 必须落在其中。）
func TestBuildAgentReleaseBundleWithLineageEmbedsLineageInManifest(t *testing.T) {
	version := lineageTestVersion("Be portable, but sharper.")
	lineage := &types.AgentReleaseLineage{SourceListingID: "listing-1", SourceReleaseID: "rel-1", IsFork: true}
	bundle, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), types.DependencyLock{}, lineage)
	require.NoError(t, err)

	manifestJSON, err := json.Marshal(bundle.Manifest)
	require.NoError(t, err)
	var manifest types.AgentReleaseManifest
	require.NoError(t, json.Unmarshal(manifestJSON, &manifest))
	require.NotNil(t, manifest.Lineage)
	require.Equal(t, "listing-1", manifest.Lineage.SourceListingID)
	require.Equal(t, "rel-1", manifest.Lineage.SourceReleaseID)
	require.True(t, manifest.Lineage.IsFork)
	require.Contains(t, string(bundle.Bytes), `"lineage"`, "lineage 段必须落在 digest 边界内")

	// 无 lineage 的 manifest 序列化不得出现 "lineage" 键。
	neutral, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), types.DependencyLock{}, nil)
	require.NoError(t, err)
	require.Nil(t, neutral.Manifest.Lineage)
	neutralJSON, err := json.Marshal(neutral.Manifest)
	require.NoError(t, err)
	require.NotContains(t, string(neutralJSON), `"lineage"`)
	require.NotContains(t, string(neutral.Bytes), `"lineage"`)
}
