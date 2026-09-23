package plugins

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// declaredNoArgSchema is the input schema the fixture manifest declares for
// its single tool. fetcher_test.go serves the exact same bytes from the
// controlled MCP endpoint so the declared digest matches the live one.
const declaredNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`

func validManifest() *types.PluginManifest {
	return &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  "com.example.jira-todo",
		Version:   "1.2.0",
		Name:      "Jira 本周待办",
		Transport: types.PluginTransport{Type: "http-streamable", Endpoint: "https://plugins.example.com/jira-todo/v1.2.0/mcp"},
		Auth:      &types.PluginAuth{PersonalOAuth: true},
		Tools: []types.PluginToolDecl{{
			Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
			InputSchemaDigest: ToolSchemaDigest([]byte(declaredNoArgSchema)),
		}},
	}
}

func TestValidateManifestAcceptsValid(t *testing.T) {
	require.NoError(t, ValidateManifest(validManifest()))
}

func TestValidateManifestRejects(t *testing.T) {
	cases := []func(m *types.PluginManifest){
		func(m *types.PluginManifest) { m.Protocol = "weknora.plugin/2" },
		func(m *types.PluginManifest) { m.PluginID = "Jira" }, // 大写非法
		func(m *types.PluginManifest) { m.PluginID = "x" },    // 过短
		func(m *types.PluginManifest) { m.Version = "1.2" },   // 非 semver
		func(m *types.PluginManifest) { m.Name = "" },
		func(m *types.PluginManifest) { m.Transport.Type = "stdio" }, // Spec 排除 stdio
		func(m *types.PluginManifest) { m.Transport.Endpoint = "ftp://e" },
		func(m *types.PluginManifest) { m.Transport.Endpoint = "" },
		func(m *types.PluginManifest) { m.Tools = nil },
		func(m *types.PluginManifest) { m.Tools[0].Name = "" },
		func(m *types.PluginManifest) { m.Tools[0].InputSchemaDigest = "md5:abc" },
		func(m *types.PluginManifest) { m.Tools[0].InputSchemaDigest = "" },
		func(m *types.PluginManifest) { m.Auth = nil }, // requires_personal_auth 工具要求 PersonalOAuth
	}
	for i, mutate := range cases {
		m := validManifest()
		mutate(m)
		require.Errorf(t, ValidateManifest(m), "case %d must be rejected", i)
	}
}

func TestToolSchemaDigestIsCanonical(t *testing.T) {
	a := ToolSchemaDigest([]byte(`{"type":"object","properties": {"b": {}, "a": {}}}`))
	b := ToolSchemaDigest([]byte(`{"properties":{"a":{},"b":{}},"type":"object"}`))
	require.Equal(t, a, b)
	require.Len(t, a, 64)
}

func TestManifestContentDigestExcludesSelf(t *testing.T) {
	m := validManifest()
	m.ContentDigest = ManifestContentDigest(m)
	raw, _ := json.Marshal(m)
	var round types.PluginManifest
	require.NoError(t, json.Unmarshal(raw, &round))
	require.Equal(t, m.ContentDigest, ManifestContentDigest(&round))
}
