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

// TestValidateNameRejectsSeparatorsAndPrivateUse（整分支 OCR 二轮 F2）：
// 拒绝集是 Cc/Cf/Co 且应含 Zl/Zp（U+2028/U+2029 行/段分隔符可在管理端
// 审核界面引入换行布局干扰）；错误文案必须如实列出全部类别——私有区字符
// 被拒却报 format 会误导排障（评审实测：U+2028/U+2029 现通过，
// U+E000 被 Co 拒但文案未提）。
func TestValidateNameRejectsSeparatorsAndPrivateUse(t *testing.T) {
	for _, r := range []rune{'\u2028', '\u2029'} {
		err := validateName("tools[0].name", "bad"+string(r)+"name", maxToolNameLen)
		require.Errorf(t, err, "U+%04X (Zl/Zp separator) must be rejected from names", r)
	}
	err := validateName("tools[0].name", "bad\uE000name", maxToolNameLen)
	require.Error(t, err, "private-use (Co) must be rejected from names")
	require.ErrorContains(t, err, "private-use",
		"the rejection message must name the actual character class")
	// OCR 一轮 F4：消息必须同样命名 Zl/Zp 类别——validateName 注释承诺
	// "names every rejected class"，对 U+2028 场景不成立。
	err = validateName("tools[0].name", "bad\u2028name", maxToolNameLen)
	require.ErrorContains(t, err, "separator",
		"the rejection message must name the Zl/Zp class it actually hit")
}

func validManifest() *types.PluginManifest {
	return &types.PluginManifest{
		Protocol: "weknora.plugin/1",
		PluginID: "com.example.jira-todo",
		Version:  "1.2.0",
		Name:     "Jira 本周待办", Transport: types.PluginTransport{Type: "http-streamable", Endpoint: "https://plugins.example.com/jira-todo/v1.2.0/mcp"},
		Auth: &types.PluginAuth{PersonalOAuth: true},
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
		func(m *types.PluginManifest) { m.PluginID = "a..b" },                                        // 连续分隔符
		func(m *types.PluginManifest) { m.PluginID = "a-" },                                          // 尾部分隔符
		func(m *types.PluginManifest) { m.PluginID = "a." },                                          // 尾部分隔符
		func(m *types.PluginManifest) { m.Version = "01.2.0" },                                       // 前导零
		func(m *types.PluginManifest) { m.Version = "1.2.1234567890" },                               // 版本段超过 9 位
		func(m *types.PluginManifest) { m.Name = "bad\x00name" },                                     // 控制字符
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{"read", "read"} },               // 工具 scope 重复
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{"has space"} },                  // scope 非法字符
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{strings.Repeat("x", 129)} },     // scope 超长
		func(m *types.PluginManifest) { m.Auth.Scopes = []string{"dup", "dup"} },                     // auth scope 重复
		func(m *types.PluginManifest) { m.Tools[0].Name = strings.Repeat("t", 129) },                 // 工具名超长
		func(m *types.PluginManifest) { m.Tools[0].Name = "bad\x01tool" },                            // 工具名控制字符
		func(m *types.PluginManifest) { m.Description = strings.Repeat("d", 1025) },                  // 描述超长
		func(m *types.PluginManifest) { m.Description = "desc with \x00nul" },                        // 描述非法控制字符
		func(m *types.PluginManifest) { m.Name = "bad\u202Ename" },                                   // Cf 双向覆盖符（RLO）
		func(m *types.PluginManifest) { m.Name = "x\uE000y" },                                        // Co 私用区
		func(m *types.PluginManifest) { m.Tools[0].Name = "t\u200Bopt" },                             // Cf 零宽空格
		func(m *types.PluginManifest) { m.Description = "d\uFEFFbom" },                               // Cf BOM
		func(m *types.PluginManifest) { m.Auth = nil },                                               // requires_personal_auth 工具要求 PersonalOAuth
		func(m *types.PluginManifest) { m.Transport.Endpoint = "https://user:pass@example.com/mcp" }, // userinfo 内嵌凭据（OCR T01-R1-F3）
		// 终评 R5 F9：豁免仅限自由文本的 U+200C/U+200D——其余 Cf 仍拒，且
		// 名称（标识符面）不豁免任何 Cf。
		func(m *types.PluginManifest) { m.Description = "d\u200Bzwsp" },   // Cf 零宽空格（描述面仍拒）
		func(m *types.PluginManifest) { m.Description = "d\u202Erlo" },    // Cf 双向覆盖符（描述面仍拒）
		func(m *types.PluginManifest) { m.Name = "名\u200D称" },             // 名称不豁免 ZWJ
		func(m *types.PluginManifest) { m.Tools[0].Name = "t\u200Ctool" }, // 工具名不豁免 ZWNJ
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

// TestValidateManifestBoundsURLErrorEcho (OCR T01-R4-F3): a url.Parse
// failure on the transport endpoint used to be passed through with %w —
// *url.Error embeds the FULL original URL (up to ~maxManifestBytes fetched
// from the remote manifest), so a hostile endpoint ballooned the error into
// the 400 response, bypassing this file's maxEchoRunes echo discipline.
// JSON escapes (\u0000) carry control characters past the length checks and
// into url.Parse.
func TestValidateManifestBoundsURLErrorEcho(t *testing.T) {
	m := validManifest()
	m.Transport.Endpoint = "https://example.com/mcp" + strings.Repeat("x", 4096) + "\x00"
	err := ValidateManifest(m)
	require.Error(t, err)
	require.Less(t, len(err.Error()), 1024, "url.Parse failure must not echo the full endpoint")
	require.NotContains(t, err.Error(), strings.Repeat("x", 100))
}

// TestValidateManifestBoundsParseReasonEcho (OCR round-1 F3): the inner
// url.Parse reason is NOT a fixed-size message family — parseHost's
// `invalid port %q after host` embeds everything after the last colon of the
// authority (review measured a 900KB error via this path). The unwrapped
// uerr.Err must pass through echoQuoted like every other untrusted echo.
func TestValidateManifestBoundsParseReasonEcho(t *testing.T) {
	m := validManifest()
	m.Transport.Endpoint = "http://h:" + strings.Repeat("a", 9000)
	err := ValidateManifest(m)
	require.Error(t, err)
	require.Less(t, len(err.Error()), 1024,
		"the parse reason must be bounded even when it embeds the hostile port")
	require.NotContains(t, err.Error(), strings.Repeat("a", 100))
}

// TestValidateManifestAcceptsMultilineDescription: newlines/tabs are legal in
// free-text descriptions (multi-line tool docs are common); only other
// control characters are rejected.
func TestValidateManifestAcceptsMultilineDescription(t *testing.T) {
	m := validManifest()
	m.Description = "line one\nline two\tindented\r\nwindows"
	require.NoError(t, ValidateManifest(m))
}

// TestValidateManifestAcceptsZWJAndZWNJDescription (final-review R5 F9,
// round-3 finding): U+200D (ZWJ) is a mandatory joiner inside legitimate
// compound emoji sequences (family, mixed-skin-tone handshake) and U+200C
// (ZWNJ) is required by some orthographies (Persian). Blanket Cf rejection
// denied benign remote-controlled descriptions at the install gate with no
// way for the admin to fix them. Free text exempts exactly these two runes;
// names keep rejecting all Cf (see TestValidateManifestRejects).
func TestValidateManifestAcceptsZWJAndZWNJDescription(t *testing.T) {
	m := validManifest()
	m.Description = "家庭待办 👨‍👩‍👧‍👦 与混肤色握手 🫱🏿‍🫲🏽，含波斯语正字法 می‌خواهم"
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

// TestCanonicalJSONAlwaysEscapesLineSeparators (OCR T01-ocr-r3-004) pins a
// protocol-contract detail the doc comment now states: Go's encoder escapes
// U+2028/U+2029 UNCONDITIONALLY (JSONSP compatibility) even with
// SetEscapeHTML(false), while JSON.stringify (ES2019+) keeps them literal.
// The canonical output therefore ALWAYS carries these two code points in
// escaped form, never as literal code points — non-Go reimplementations
// that keep them literal will compute different digests over documents
// containing them.
func TestCanonicalJSONAlwaysEscapesLineSeparators(t *testing.T) {
	// Built via rune()/concatenation on purpose: the code points and their
	// escape sequences are invisible/unreliable as literals in source.
	ls := string(rune(0x2028))
	ps := string(rune(0x2029))
	canonical := canonicalizeJSON([]byte(`{"a":"x` + ls + `y` + ps + `z"}`))
	s := string(canonical)
	require.Contains(t, s, `\`+`u2028`)
	require.Contains(t, s, `\`+`u2029`)
	require.NotContains(t, s, ls)
	require.NotContains(t, s, ps)
}

// TestManifestContentDigestIgnoresUnknownAndEmptyOptionalFields (OCR
// T01-ocr-r3-009) pins the STRUCTURAL digest domain of content_digest: the
// digest covers the parsed manifest's semantic fields, so an unknown
// extension key or an explicitly-empty-but-equivalent optional field in the
// raw document does not change it — such documents verify cleanly instead
// of being rejected as content_digest mismatches. (Injecting "auth": null
// over a manifest whose Auth is set would CHANGE semantics — a nil-vs-set
// pointer is a real difference and must digest differently.) ToolSchemaDigest,
// by contrast, is document-level (raw schema bytes).
func TestManifestContentDigestIgnoresUnknownAndEmptyOptionalFields(t *testing.T) {
	base := validManifest()
	raw, err := json.Marshal(base)
	require.NoError(t, err)
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &doc))
	doc["x_extension"] = json.RawMessage(`{"v":1}`)
	doc["description"] = json.RawMessage(`""`) // base.Description == "" — equivalent
	extended, err := json.Marshal(doc)
	require.NoError(t, err)
	var parsed types.PluginManifest
	require.NoError(t, json.Unmarshal(extended, &parsed))
	require.Equal(t, ManifestContentDigest(base), ManifestContentDigest(&parsed))
}

// TestValidateManifestParseFailureMasksUserinfo（OCR 一轮 R12-E F03）：
// parse 失败分支回显 endpoint 前，authority 中的 userinfo 凭据必须脱敏
// ——格式非法的 URL 不得泄露格式合法 URL 被刻意静默隐藏的凭据。
func TestValidateManifestParseFailureMasksUserinfo(t *testing.T) {
	m := validManifest()
	m.Transport.Endpoint = "https://ci-bot:s3cr3t@jira.example.com:badport/mcp"
	err := ValidateManifest(m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transport endpoint")
	require.Contains(t, err.Error(), "REDACTED@")
	require.NotContains(t, err.Error(), "s3cr3t", "credentials must not leak into the admin-visible error")
	require.NotContains(t, err.Error(), "ci-bot:", "userinfo identity must not leak into the error")
}

// TestValidateManifestParseFailureKeepsReasonReadableWithoutUserinfo（OCR R1
// F12）：无凭据端点（userinfoOf 返回空串）的 parse 失败分支，错误文本必须
// 保持可读——strings.ReplaceAll(s, "", "REDACTED") 按 Go 语义会在每个 rune
// 后各插一次 REDACTED，把最常见的坏端口诊断搅成乱码直达管理员 400 响应。
func TestValidateManifestParseFailureKeepsReasonReadableWithoutUserinfo(t *testing.T) {
	m := validManifest()
	m.Transport.Endpoint = "https://jira.example.com:badport/mcp"
	err := ValidateManifest(m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transport endpoint")
	require.Contains(t, err.Error(), "badport", "the original parse reason must stay readable")
	require.Equal(t, 0, strings.Count(err.Error(), "REDACTED"),
		"no userinfo on the endpoint means nothing to redact — empty-string replace must not corrupt the message")
}

// TestValidateNameRejectsPathSeparators（OCR R1 F16）：含 '/' 或 '%' 的名字
// 可完整通过现有卫生规则进入快照，但策略端点 /tools/:tool_name/policy 按
// 单段匹配（%2F 也会被还原），这类工具永不可寻址——逐工具治理被静默架空。
// 名称里拒绝这两类字符使「快照内工具必然可经策略端点寻址」成为安装时不
// 变量（同时覆盖 manifest 声明名与 live 目录名——两者共用 validateName）。
func TestValidateNameRejectsPathSeparators(t *testing.T) {
	for _, name := range []string{"foo/bar", "foo%2Fbar", "foo%bar", "/"} {
		m := validManifest()
		m.Tools[0].Name = name
		err := ValidateManifest(m)
		require.Error(t, err, "name %q must be rejected", name)
		require.Contains(t, err.Error(), "must not contain", "the rejection must name the character class")
	}
}
