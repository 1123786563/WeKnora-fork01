package plugins

import (
	"encoding/json"
	"strings"
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
		func(m *types.PluginManifest) { m.PluginID = "a..b" },                                    // 连续分隔符
		func(m *types.PluginManifest) { m.PluginID = "a-" },                                      // 尾部分隔符
		func(m *types.PluginManifest) { m.PluginID = "a." },                                      // 尾部分隔符
		func(m *types.PluginManifest) { m.Version = "01.2.0" },                                   // 前导零
		func(m *types.PluginManifest) { m.Version = "1.2.1234567890" },                           // 版本段超过 9 位
		func(m *types.PluginManifest) { m.Name = "bad\x00name" },                                 // 控制字符
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{"read", "read"} },           // 工具 scope 重复
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{"has space"} },              // scope 非法字符
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{strings.Repeat("x", 129)} }, // scope 超长
		func(m *types.PluginManifest) { m.Auth.Scopes = []string{"dup", "dup"} },                 // auth scope 重复
		func(m *types.PluginManifest) { m.Tools[0].Name = strings.Repeat("t", 129) },             // 工具名超长
		func(m *types.PluginManifest) { m.Tools[0].Name = "bad\x01tool" },                        // 工具名控制字符
		func(m *types.PluginManifest) { m.Description = strings.Repeat("d", 1025) },              // 描述超长
		func(m *types.PluginManifest) { m.Description = "desc with \x00nul" },                    // 描述非法控制字符
		func(m *types.PluginManifest) { m.Name = "bad\u202Ename" },                               // Cf 双向覆盖符（RLO）
		func(m *types.PluginManifest) { m.Name = "x\uE000y" },                                    // Co 私用区
		func(m *types.PluginManifest) { m.Tools[0].Name = "t\u200Bopt" },                         // Cf 零宽空格
		func(m *types.PluginManifest) { m.Description = "d\uFEFFbom" },                           // Cf BOM
		func(m *types.PluginManifest) { m.Auth = nil },                                           // requires_personal_auth 工具要求 PersonalOAuth
	}
	for i, mutate := range cases {
		m := validManifest()
		mutate(m)
		require.Errorf(t, ValidateManifest(m), "case %d must be rejected", i)
	}
}

func TestValidateManifestAcceptsScopedManifest(t *testing.T) {
	m := validManifest()
	m.Auth.Scopes = []string{"read:jira", "write:jira"}
	m.Tools[0].Scopes = []string{"read:jira"}
	require.NoError(t, ValidateManifest(m))
}

// TestValidateManifestAcceptsMultilineDescription: newlines/tabs are legal in
// free-text descriptions (multi-line tool docs are common); only other
// control characters are rejected.
func TestValidateManifestAcceptsMultilineDescription(t *testing.T) {
	m := validManifest()
	m.Description = "line one\nline two\tindented\r\nwindows"
	require.NoError(t, ValidateManifest(m))
}

// TestCanonicalJSONDoesNotEscapeHTML (OCR T01-R2-3): the canonical form must
// not apply Go's default HTML escaping (< > & → \u003c \u003e \u0026), or
// external implementations following the documented algorithm (sorted keys,
// no whitespace, verbatim number literals) would compute different digests
// for schemas containing those characters.
func TestCanonicalJSONDoesNotEscapeHTML(t *testing.T) {
	out := CanonicalJSON(map[string]any{"pattern": "<a>&"})
	require.Contains(t, string(out), `"<a>&"`)
	require.NotContains(t, string(out), `\u003c`)
	require.NotContains(t, string(out), `\u0026`)
}

func TestToolSchemaDigestIsCanonical(t *testing.T) {
	a := ToolSchemaDigest([]byte(`{"type":"object","properties": {"b": {}, "a": {}}}`))
	b := ToolSchemaDigest([]byte(`{"properties":{"a":{},"b":{}},"type":"object"}`))
	require.Equal(t, a, b)
	require.Len(t, a, 64)
}

// TestToolSchemaDigestNumberLiteralsAreVerbatim pins the digest protocol's
// number semantics (OCR T01-R1-3): number literals are preserved verbatim —
// this is NOT RFC 8785 JCS number normalization. 1, 1.0 and 1e2 hash
// differently even though they denote equal numeric values.
func TestToolSchemaDigestNumberLiteralsAreVerbatim(t *testing.T) {
	require.NotEqual(t,
		ToolSchemaDigest([]byte(`{"maximum":1}`)),
		ToolSchemaDigest([]byte(`{"maximum":1.0}`)),
	)
	require.NotEqual(t,
		ToolSchemaDigest([]byte(`{"n":1e2}`)),
		ToolSchemaDigest([]byte(`{"n":100}`)),
	)
	// Same literal modulo key order and insignificant whitespace still agrees.
	require.Equal(t,
		ToolSchemaDigest([]byte("{\n \"a\": 1.50\n}")),
		ToolSchemaDigest([]byte(`{"a":1.50}`)),
	)
}

// TestToolSchemaDigestRejectsTrailingGarbage (OCR T01-R1-6): a document with
// trailing garbage is INVALID JSON and must yield "", never the digest of a
// valid prefix.
func TestToolSchemaDigestRejectsTrailingGarbage(t *testing.T) {
	require.Empty(t, ToolSchemaDigest([]byte(`{"type":"object"}garbage`)))
	require.Empty(t, ToolSchemaDigest([]byte(`{"a":1} {"b":2}`)))
	// Trailing whitespace alone is legal (json.Unmarshal semantics).
	require.NotEmpty(t, ToolSchemaDigest([]byte("{\"a\":1}\n\t ")))
}

func TestManifestContentDigestExcludesSelf(t *testing.T) {
	m := validManifest()
	m.ContentDigest = ManifestContentDigest(m)
	raw, _ := json.Marshal(m)
	var round types.PluginManifest
	require.NoError(t, json.Unmarshal(raw, &round))
	require.Equal(t, m.ContentDigest, ManifestContentDigest(&round))
}
