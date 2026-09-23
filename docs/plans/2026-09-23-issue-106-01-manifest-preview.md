# 插件 01｜管理员预览插件清单 实施计划（Issue #108）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 空间管理员粘贴插件清单 URL 后，WeKnora 抓取清单、核验版本端点的实际 MCP 工具目录与清单声明一致，展示版本、端点、工具、读写分类与授权要求供审阅；预览有有效期、不创建安装；受限网络地址、无效清单、端点不可达、声明不符均明确拒绝。

**Architecture:** 新 `internal/modules/plugins` 包：`manifest.go` 定义 `weknora.plugin/1` 协议与校验，`fetcher.go` 用 SSRF 安全 HTTP 客户端抓清单并经 `EndpointLister`（基于 `mcp.MCPManager` 的临时连接）实时 `ListTools` 核验，`snapshot.go` 生成核验后的工具快照与 digest。预览结果持久化到新表 `plugin_previews`（含身份指纹与有效期），由 `PluginService.PreviewFromManifest` + `POST /api/v1/plugins/installations/preview`（Admin）暴露。

**Tech Stack:** Go 1.26；`utils.NewSSRFSafeHTTPClient`/`ValidateURLForSSRF`；`mark3labs/mcp-go`（服务端用于受控测试）；gorm + PostgreSQL/SQLite versioned migrations；gin。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 7/8/9/23；Implementation Decisions 49-51、57 行）；`docs/specs/2026-09-23-self-hosted-plugins-design.md`「清单与版本契约」节；范围裁决 GAP-1/GAP-2 落实（见总索引）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/108 （无 blocked_by）

## Global Constraints

见总索引 `2026-09-23-issue-106-00-index.md` Global Constraints 节（SSRF、参数绑定、无凭据字面量、worktree 边界、中文提交、迁移编号、手工 MCP 兼容）。本切片新增迁移 PG `000189_plugin_previews` / SQLite `000110_plugin_previews`。

## Review Focus（本切片）

- 恶意清单 URL（`file://`/环回/私网/保留地址/超长）→ 抓取前拒绝且零外发请求（T01 Step 1）。
- 清单声明与远端实际目录不符（缺工具/多工具/schema digest 不符）→ 预览拒绝并列出差异（T01 Step 5）。
- 端点 OAuth 保护导致无法核验 → 明确拒绝而非半成功预览（T01 Step 5）。
- 预览 TTL 过期/重复消费 → 确认阶段拒绝（T02 Step 1，供 T06 消费）。
- 非管理员调用预览 API → 403（T02 Step 3）。

---

### Task 1: 清单协议、校验与远端核验（纯函数层）

**Files:**
- Create: `internal/types/plugin.go`
- Create: `internal/modules/plugins/manifest.go`
- Create: `internal/modules/plugins/fetcher.go`
- Create: `internal/modules/plugins/snapshot.go`
- Test: `internal/modules/plugins/manifest_test.go`
- Test: `internal/modules/plugins/fetcher_test.go`

**Interfaces:**
- Consumes: `types.MCPTool`（types/mcp.go:131）、`utils.ValidateURLForSSRF(rawURL string) error`（security.go:1200）、`mcp.MCPManager.GetOrCreateClient`（manager.go:88）。
- Produces（后续任务依赖的精确签名）:
  - `types.PluginManifest`、`types.PluginTransport`、`types.PluginAuth`、`types.PluginToolDecl`、`types.PluginToolSnapshot`（见下方结构）
  - `plugins.ValidateManifest(m *types.PluginManifest) error`
  - `plugins.CanonicalJSON(v any) []byte`（key 排序、无多余空白）
  - `plugins.ToolSchemaDigest(schema []byte) string`（canonical SHA-256，hex 64）
  - `plugins.ManifestContentDigest(m *types.PluginManifest) string`（剔除 `content_digest` 字段后的 canonical SHA-256，hex 64）
  - `plugins.SnapshotDigest(tools []types.PluginToolSnapshot) string`
  - `plugins.IdentityFingerprint(pluginID, version, endpoint string, toolsDigest string) string`
  - `type plugins.EndpointLister func(ctx context.Context, transportType string, endpointURL string) ([]*types.MCPTool, error)`
  - `plugins.NewMCPEndpointLister(manager *mcp.MCPManager) EndpointLister`
  - `plugins.FetchAndVerify(ctx context.Context, manifestURL string, lister EndpointLister) (*plugins.FetchResult, error)`
  - `plugins.BuildVerifiedSnapshot(manifest *types.PluginManifest, live []*types.MCPTool) ([]types.PluginToolSnapshot, string, error)`（返回快照与 tools_digest；声明不符时错误信息列出每个差异工具）
  - `var plugins.ErrOAuthProtectedEndpoint`、`plugins.IsOAuthProtected(err) bool`

- [ ] **Step 1: 写失败测试（协议校验与 digest）**

创建 `internal/modules/plugins/manifest_test.go`：

```go
package plugins

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func validManifest() *types.PluginManifest {
	return &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  "com.example.jira-todo",
		Version:   "1.2.0",
		Name:      "Jira 本周待办",
		Transport: types.PluginTransport{Type: "http-streamable", Endpoint: "https://plugins.example.com/jira-todo/v1.2.0/mcp"},
		Tools: []types.PluginToolDecl{{
			Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
			InputSchemaDigest: ToolSchemaDigest([]byte(`{"type":"object","properties":{},"additionalProperties":false}`)),
		}},
	}
}

func TestValidateManifestAcceptsValid(t *testing.T) {
	require.NoError(t, ValidateManifest(validManifest()))
}

func TestValidateManifestRejects(t *testing.T) {
	cases := []func(m *types.PluginManifest){
		func(m *types.PluginManifest) { m.Protocol = "weknora.plugin/2" },
		func(m *types.PluginManifest) { m.PluginID = "Jira" },             // 大写非法
		func(m *types.PluginManifest) { m.PluginID = "x" },               // 过短
		func(m *types.PluginManifest) { m.Version = "1.2" },              // 非 semver
		func(m *types.PluginManifest) { m.Name = "" },
		func(m *types.PluginManifest) { m.Transport.Type = "stdio" },     // Spec 排除 stdio
		func(m *types.PluginManifest) { m.Transport.Endpoint = "ftp://e" },
		func(m *types.PluginManifest) { m.Transport.Endpoint = "" },
		func(m *types.PluginManifest) { m.Tools = nil },
		func(m *types.PluginManifest) { m.Tools[0].Name = "" },
		func(m *types.PluginManifest) { m.Tools[0].InputSchemaDigest = "md5:abc" },
		func(m *types.PluginManifest) { m.Tools[0].InputSchemaDigest = "" },
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestValidateManifest|TestToolSchemaDigest|TestManifestContentDigest' -v`
Expected: FAIL —— 包 `internal/modules/plugins` 不存在（编译错误 `no required module provides package`）。

- [ ] **Step 3: 实现类型与校验**

`internal/types/plugin.go`：

```go
package types

// PluginManifest is the weknora.plugin/1 protocol document served by the
// plugin developer at a stable URL. It describes ONE version; the endpoint
// must stay reachable for that version's lifetime (spec: developers keep old
// version endpoints available). A manifest is review material, never an
// execution grant.
type PluginManifest struct {
	Protocol      string           `json:"protocol"`
	PluginID      string           `json:"plugin_id"`
	Version       string           `json:"version"`
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Transport     PluginTransport  `json:"transport"`
	Auth          *PluginAuth      `json:"auth,omitempty"`
	Tools         []PluginToolDecl `json:"tools"`
	ContentDigest string           `json:"content_digest,omitempty"`
}

type PluginTransport struct {
	Type     string `json:"type"`     // "http-streamable" | "sse"
	Endpoint string `json:"endpoint"` // version-pinned MCP endpoint
}

type PluginAuth struct {
	PersonalOAuth bool     `json:"personal_oauth"`
	Scopes        []string `json:"scopes,omitempty"`
}

type PluginToolDecl struct {
	Name                 string   `json:"name"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes,omitempty"`
	InputSchemaDigest    string   `json:"input_schema_digest"` // sha256:<64hex>，自报仅用于一致性校验
}

// PluginToolSnapshot is the authoritative capability record the tenant
// accepted at install/upgrade time. Schema digests are recomputed from the
// LIVE ListTools result, never copied from the manifest self-report.
type PluginToolSnapshot struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	InputSchemaDigest    string   `json:"input_schema_digest"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes,omitempty"`
}
```

`internal/modules/plugins/manifest.go` 实现：`ValidateManifest`（protocol 恒等 `weknora.plugin/1`；`plugin_id` 匹配 `^[a-z0-9][a-z0-9.-]{2,127}$`；`version` 匹配 `^\d+\.\d+\.\d+$`；name 非空 ≤255；transport.type ∈ {`http-streamable`,`sse`}；endpoint 先 `url.Parse` 校验 scheme http/https 且 host 非空，**完整 SSRF 校验留给 FetchAndVerify**（校验函数保持纯函数可单测，网络判断在抓取前执行）；tools 非空、name 非空唯一、`input_schema_digest` 匹配 `^sha256:[0-9a-f]{64}$`；声明 `requires_personal_auth=true` 的工具要求 `Auth.PersonalOAuth=true`）；`CanonicalJSON`（`json.Marshal` 对 map 自动按 key 排序，先 `json.Unmarshal` 到 `any` 再 Marshal）；`ToolSchemaDigest`（CanonicalJSON → sha256 → hex）；`ManifestContentDigest`（拷贝 manifest 置空 ContentDigest 后 canonical sha256）；`SnapshotDigest`、`IdentityFingerprint`（`sha256(pluginID + "\x00" + version + "\x00" + endpoint + "\x00" + toolsDigest)` hex）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestValidateManifest|TestToolSchemaDigest|TestManifestContentDigest' -v`
Expected: PASS（4 个测试）。

- [ ] **Step 5: 写失败测试（抓取核验）**

`internal/modules/plugins/fetcher_test.go`（受控远端：httptest 挂清单 JSON + `sdkserver` MCP 服务，先例 mcp_catalog_integration_test.go:34-105）：

```go
package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func newControlledPluginHost(t *testing.T, manifestHandler http.HandlerFunc, mcpHandler http.HandlerFunc) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/mcp", mcpHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func streamableMCPServer(t *testing.T, toolName string) http.HandlerFunc {
	t.Helper()
	server := sdkserver.NewMCPServer("plugin-under-test", "1", sdkserver.WithToolCapabilities(false))
	server.AddTool(
		sdkmcp.NewTool(toolName, sdkmcp.WithDescription("desc")),
		func(_ context.Context, _ sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return sdkmcp.NewToolResultText("ok"), nil
		},
	)
	transport := sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true))
	return transport.ServeHTTP
}

func TestFetchRejectsNonHTTPAndPrivateManifestURLs(t *testing.T) {
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		t.Fatal("lister must not be called for rejected URLs")
		return nil, nil
	}
	for _, url := range []string{
		"file:///etc/passwd", "gopher://x", "http://127.0.0.1:8080/manifest.json",
		"http://localhost/manifest.json", "http://169.254.169.254/latest/meta-data",
		"http://10.1.2.3/manifest.json", "http://[::1]/manifest.json",
	} {
		_, err := FetchAndVerify(context.Background(), url, lister)
		require.Errorf(t, err, "url %s must be rejected", url)
	}
}

func TestFetchAndVerifyHappyPath(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "application/json"); _, _ = w.Write(manifestJSON) },
		streamableMCPServer(t, "search_my_week_issues"),
	)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	res, err := FetchAndVerify(context.Background(), base+"/manifest.json", NewMCPEndpointLister(manager))
	require.NoError(t, err)
	require.Equal(t, "com.example.jira-todo", res.Manifest.PluginID)
	require.Len(t, res.Snapshot, 1)
	require.Equal(t, ToolSchemaDigest(nil2raw(res.LiveTools[0].InputSchema)), res.Snapshot[0].InputSchemaDigest)
	require.NotEmpty(t, res.ToolsDigest)
	require.NotEmpty(t, res.IdentityFingerprint)
}

func TestBuildVerifiedSnapshotRejectsMismatch(t *testing.T) {
	m := validManifest()
	// 远端实际 schema 与清单声明不同 → digest 不符
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(`{"type":"object","properties":{"x":{}}}`)}}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "search_my_week_issues")
	// 远端缺声明工具 → 拒绝
	_, _, err = BuildVerifiedSnapshot(m, nil)
	require.ErrorContains(t, err, "missing")
}
```

（`nil2raw` 为测试内小助手 `func nil2raw(b []byte) []byte { if b == nil { return []byte(`{}`) }; return b }`——sdkserver 生成的无参工具 schema 非空，助手仅防御。）

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestFetch|TestBuildVerifiedSnapshot' -v`
Expected: FAIL —— `FetchAndVerify`/`BuildVerifiedSnapshot`/`NewMCPEndpointLister` 未定义（编译错误）。

- [ ] **Step 7: 实现 fetcher.go 与 snapshot.go**

`fetcher.go` 关键逻辑：
```go
var maxManifestBytes = 1 << 20 // 1 MiB

type FetchResult struct {
	Manifest           *types.PluginManifest
	LiveTools          []*types.MCPTool
	Snapshot           []types.PluginToolSnapshot
	ToolsDigest        string
	IdentityFingerprint string
}

func FetchAndVerify(ctx context.Context, manifestURL string, lister EndpointLister) (*FetchResult, error) {
	// 1) SSRF 前置（GAP-2）：仅 http/https + 拒绝 localhost/环回/私有/保留。
	if err := utils.ValidateURLForSSRF(manifestURL); err != nil {
		return nil, fmt.Errorf("manifest URL rejected: %w", err)
	}
	// 2) 抓取（SSRF 安全客户端 + 1MiB 上限 + 15s 超时 + 仅 2xx + Content-Type 必要性不强制）。
	body, err := fetchLimited(ctx, manifestURL)
	if err != nil { return nil, err }
	// 3) 解析 + 协议校验。
	var m types.PluginManifest
	if err := json.Unmarshal(body, &m); err != nil { return nil, fmt.Errorf("invalid manifest JSON: %w", err) }
	if err := ValidateManifest(&m); err != nil { return nil, err }
	if m.ContentDigest != "" && m.ContentDigest != ManifestContentDigest(&m) {
		return nil, fmt.Errorf("manifest content_digest mismatch")
	}
	// 4) 端点 SSRF 校验 + 远端核验（OAuth 保护端点 → ErrOAuthProtectedEndpoint 明确拒绝）。
	if err := utils.ValidateURLForSSRF(m.Transport.Endpoint); err != nil {
		return nil, fmt.Errorf("plugin endpoint rejected: %w", err)
	}
	live, err := lister(ctx, m.Transport.Type, m.Transport.Endpoint)
	if err != nil {
		if mcp.IsOAuthRequired(err) { return nil, fmt.Errorf("%w: %v", ErrOAuthProtectedEndpoint, err) }
		return nil, fmt.Errorf("plugin endpoint verification failed: %w", err)
	}
	snapshot, digest, err := BuildVerifiedSnapshot(&m, live)
	if err != nil { return nil, err }
	return &FetchResult{Manifest: &m, LiveTools: live, Snapshot: snapshot, ToolsDigest: digest,
		IdentityFingerprint: IdentityFingerprint(m.PluginID, m.Version, m.Transport.Endpoint, digest)}, nil
}

func NewMCPEndpointLister(manager *mcp.MCPManager) EndpointLister {
	return func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		svc := &types.MCPService{
			ID: "plugin-verify-" + hex(sha256(endpointURL)), TenantID: 0,
			Name: "plugin-verify", Enabled: true,
			TransportType: types.MCPTransportType(transportType), URL: &endpointURL,
		}
		client, err := manager.GetOrCreateClient(ctx, svc) // 临时 service，不入库
		if err != nil { return nil, err }
		defer func() { _ = client.Disconnect() }()
		return client.ListTools(ctx)
	}
}
```

（`mcp.IsOAuthRequired` 已存在：client.go OAuthRequiredError 语义，若包内未导出判定函数则以 `errors.As(&mcp.OAuthRequiredError{})` 实现。）

`snapshot.go`：`BuildVerifiedSnapshot` 逐清单工具在 live 中找同名——缺失 → `fmt.Errorf("manifest tool %q missing from live endpoint", name)`；digest 不符 → `fmt.Errorf("manifest tool %q schema digest mismatch (declared %s, live %s)", ...)`；live 有而清单未声明的工具 → 也拒绝（`undeclared tool %q present on endpoint`——Spec 8 行"识别清单和执行服务不一致"）；快照字段 = 清单声明（read_only/requires_personal_auth/scopes）+ live description + **live** schema digest（权威）。

- [ ] **Step 8: 运行确认通过 + 包全测**

Run: `go test ./internal/modules/plugins/ -v`
Expected: PASS（全部测试）。

- [ ] **Step 9: Commit**

```bash
git add internal/types/plugin.go internal/modules/plugins/
git commit -m "feat(plugins): 插件清单协议校验与 SSRF 安全远端核验（weknora.plugin/1）[T01]"
```

---

### Task 2: 预览持久化、服务层与预览 API

**Files:**
- Create: `migrations/versioned/000189_plugin_previews.up.sql`
- Create: `migrations/versioned/000189_plugin_previews.down.sql`
- Create: `migrations/sqlite/000110_plugin_previews.up.sql`
- Create: `migrations/sqlite/000110_plugin_previews.down.sql`
- Modify: `internal/types/plugin.go`（追加 `PluginPreview` gorm 模型）
- Create: `internal/types/interfaces/plugin.go`（`PluginRepository`/`PluginService` 接口，本任务先落预览方法）
- Create: `internal/application/repository/plugin.go`
- Create: `internal/application/service/plugin_service.go`
- Create: `internal/handler/dto/plugin.go`
- Create: `internal/handler/plugin.go`
- Create: `internal/router/routes_plugins.go`
- Modify: `internal/container/container.go`（DI 注册）
- Modify: 路由聚合点（挂载 `RegisterPluginRoutes`；聚合文件为注册 `RegisterInfraRoutes` 等的调用处——执行时以 `grep -rn "RegisterInfraRoutes(" internal/router/` 定位并同点挂载）
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go`（`versionedSQLiteTables` 加 `plugin_previews`）
- Test: `internal/modules/plugins/preview_service_test.go`
- Test: `internal/handler/plugin_test.go`

**Interfaces:**
- Consumes: T01 全部产出；`mcp.NewMCPManager(oauthRepo)`；handler 模式（`internal/handler/mcp_service.go:23-45`）；路由模式（routes_infra.go:158-210 的 `g.Admin()`）。
- Produces:
  - `types.PluginPreview` gorm 模型（表 `plugin_previews`）
  - `interfaces.PluginRepository`：`CreatePreview(ctx context.Context, p *types.PluginPreview) error`、`GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error)`、`MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error`（`WHERE consumed_at IS NULL`，gorm `RowsAffected` 承载一次性消费语义）
  - `interfaces.PluginService.PreviewFromManifest(ctx context.Context, tenantID uint64, actorID, manifestURL string) (*dto.PluginPreviewResponse, error)`
  - `dto.PluginPreviewResponse`（预览 ID + 审阅数据 + `expires_at`）
  - `handler.PluginHandler` + `RegisterPluginRoutes(r *gin.RouterGroup, h *handler.PluginHandler, g RbacGuards)`（RbacGuards 为 routes_infra.go 同款 `*rbacGuards`，以导出参数类型对接聚合点现状）
  - 路由 `POST /api/v1/plugins/installations/preview`（Admin）

- [ ] **Step 1: 写失败测试（服务层，覆盖 TTL 与重复消费语义）**

`internal/modules/plugins/preview_service_test.go`（fake repo + httptest 受控远端，模式同 T01 Step 5）：

```go
package plugins

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestPreviewPersistsFingerprintAndExpiry(t *testing.T) /* 表驱动 fake repo 版 PreviewFromManifest */ {
	// 断言 1：返回 preview_id 非空、expires_at = now+TTL。
	// 断言 2：repo 中 CreatePreview 收到 IdentityFingerprint/ToolsDigest 非空、ToolsSnapshot JSON 可反序列化回 []types.PluginToolSnapshot。
	// 断言 3：清单 URL 为私网地址 → 返回错误且 repo 未收到任何写入。
}

func TestPreviewTTLBoundary(t *testing.T) {
	p := &types.PluginPreview{ExpiresAt: time.Now().Add(-time.Second)}
	require.True(t, p.Expired(time.Now()))
	require.False(t, (&types.PluginPreview{ExpiresAt: time.Now().Add(time.Minute)}).Expired(time.Now()))
}
```

（`PreviewFromManifest` 的服务实现签名依赖 `interfaces.PluginRepository` 与 `plugins.FetchAndVerify`；fake repo 在测试文件内定义，方法集与接口一致。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestPreview' -v`
Expected: FAIL —— `types.PluginPreview`/`Expired`/服务未定义（编译错误）。

- [ ] **Step 3: 实现迁移、模型、repo、服务**

`migrations/versioned/000189_plugin_previews.up.sql`：

```sql
-- Issue #108: verified manifest previews (admin review artifact, TTL-bound,
-- consumed exactly once by installation confirm in 000190).
CREATE TABLE plugin_previews (
 id VARCHAR(36) PRIMARY KEY,
 tenant_id BIGINT NOT NULL,
 manifest_url VARCHAR(512) NOT NULL,
 plugin_id VARCHAR(128) NOT NULL,
 version VARCHAR(64) NOT NULL,
 name VARCHAR(255) NOT NULL,
 transport_type VARCHAR(50) NOT NULL,
 endpoint_url VARCHAR(512) NOT NULL,
 tools_snapshot JSONB NOT NULL,
 tools_digest VARCHAR(64) NOT NULL,
 identity_fingerprint VARCHAR(64) NOT NULL,
 created_by VARCHAR(255) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_plugin_previews_tenant ON plugin_previews(tenant_id, plugin_id);
```

down：`DROP TABLE IF EXISTS plugin_previews;`。SQLite twin `000110`：`JSONB`→`TEXT`、`TIMESTAMPTZ`→`DATETIME`、`BIGINT`→`INTEGER`、`DEFAULT NOW()`→`DEFAULT CURRENT_TIMESTAMP`（对照 000109 twin 惯例）。

`types.PluginPreview`（追加到 internal/types/plugin.go，gorm 标签对齐上表；`func (p *PluginPreview) Expired(now time.Time) bool { return p.ConsumedAt != nil || !now.Before(p.ExpiresAt) }`——已消费同样视为过期）。

`interfaces.PluginRepository`（internal/types/interfaces/plugin.go，本任务方法集）：
```go
type PluginRepository interface {
	CreatePreview(ctx context.Context, p *types.PluginPreview) error
	GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error)
	MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error // WHERE consumed_at IS NULL，返回受影响行数语义由 gorm RowsAffected 承载
}
```

`service.PreviewFromManifest`：TTL 从 `PLUGIN_PREVIEW_TTL`（默认 `15m`，`time.ParseDuration`）读取；调 `plugins.FetchAndVerify`（`EndpointLister` 由构造注入的 `*mcp.MCPManager` 经 `NewMCPEndpointLister` 生成）；持久化 `PluginPreview{ID: uuid.New().String(), ...}`；返回 `dto.PluginPreviewResponse{PreviewID, PluginID, Version, Name, Description, TransportType, EndpointURL, Tools: []dto.PluginPreviewTool{Name, ReadOnly, RequiresPersonalAuth, Scopes, Description}}, IdentityFingerprint, ExpiresAt}`。仓库实现全部参数绑定（`Where("tenant_id = ? AND id = ?", ...)`）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestPreview' -v`
Expected: PASS。

- [ ] **Step 5: 写失败测试（handler 层，RBAC 与错误分支）**

`internal/handler/plugin_test.go`（模式 mcp_tool_approval_test.go：`gin.SetMode(gin.TestMode)` + stub service + `httptest.NewRecorder`）：

```go
func TestPreviewManifestHandler(t *testing.T) {
	// 断言 1：POST body {"manifest_url": "..."} → 200，data.preview_id 非空（stub service 返回固定 DTO）。
	// 断言 2：body 缺 manifest_url → 400。
	// 断言 3：stub service 返回 ErrOAuthProtectedEndpoint 包装错误 → 4xx 且 error 文案含"插件服务要求授权才能核验工具目录"。
	// 断言 4：manifest_url 为 "file:///x" → 400（服务层 SSRF 错误透传）。
}
```

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/handler/ -run 'TestPreviewManifest' -v`
Expected: FAIL —— `handler.PluginHandler` 未定义。

- [ ] **Step 7: 实现 handler、DTO、路由、DI，并登记 SQLite twin**

`routes_plugins.go`：`POST /plugins/installations/preview` 挂 `g.Admin()`（rbacGuards 参数类型与聚合点现有 `RegisterInfraRoutes` 一致，执行时从 routes_infra.go:146-152 抄签名）。`container.go` 依现有 `must(container.Provide(...))` 惯例注册 `repository.NewPluginRepository`、`service.NewPluginService`、`handler.NewPluginHandler`（构造参数：`interfaces.PluginRepository`、`interfaces.MCPServiceRepository`（后续任务用，本任务可先不注入——以实际编译需要为准）、`*mcp.MCPManager`）。`migration_sqlite_versioned_schema_test.go` 的 `versionedSQLiteTables` 追加 `"plugin_previews"`。

- [ ] **Step 8: 运行确认通过 + 相关包回归**

Run: `go test ./internal/handler/ -run 'TestPreviewManifest' -v && go test ./internal/database/ -run TestSQLiteMigrationsCreateVersionedSchema -v && go build ./...`
Expected: PASS/PASS/build 成功。

- [ ] **Step 9: Commit**

```bash
git add migrations/ internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/ internal/container/ internal/database/migration_sqlite_versioned_schema_test.go
git commit -m "feat(plugins): 清单预览 API 与 plugin_previews 迁移（TTL+身份指纹）[T02]"
```

---

### Task 3: 管理端清单预览 UI

**Files:**
- Create: `packages/api-client/src/plugins.ts`
- Test: `packages/api-client/src/plugins.test.ts`
- Create: `apps/web/src/settings/PluginsSettingsPanel.tsx`
- Test: `apps/web/src/settings/PluginsSettingsPanel.test.tsx`
- Modify: `apps/web/src/settings/SettingsPage.tsx`（lazy 面板 + `key === 'plugins'` 分支 + 导航分组，对照 31/445-447/621 行 McpSettingsPanel 挂载点）
- Modify: `packages/views/src/settings/registry.ts`（追加 `{ key: 'plugins', viewId: 'PluginSettings', apiDomain: 'plugins', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'], ported: true }`；**不动** `mcp` 条目——其 `ported:false` 与实际面板的出入由 R8 记录，不在本需求修）

**Interfaces:**
- Consumes: T02 的 `POST /api/v1/plugins/installations/preview` 响应 JSON（`{success, data: {preview_id, plugin_id, version, name, description, transport_type, endpoint_url, tools: [{name, description, read_only, requires_personal_auth, scopes}], identity_fingerprint, expires_at}}`）；`packages/api-client/src/client.ts` 的 `ClientRequest` 模式（configuration.ts:2）。
- Produces: `client.plugins.previewInstallation(manifestUrl: string): Promise<PluginPreviewResult>`（后续 T08/T15 扩展同一文件）；组件 `PluginsSettingsPanel({client, role})`。

- [ ] **Step 1: 写失败测试（api-client envelope 解析）**

`packages/api-client/src/plugins.test.ts`（模式 configuration.ts:231-249 / appconnector.test.ts）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePluginPreview } from './plugins.ts';

test('parsePluginPreview rejects non-success envelope', () => {
  assert.throws(() => parsePluginPreview({ success: false }), /success/);
});
test('parsePluginPreview maps preview fields', () => {
  const value = parsePluginPreview({ success: true, data: {
    preview_id: 'p1', plugin_id: 'com.example.jira-todo', version: '1.2.0', name: 'Jira',
    description: '', transport_type: 'http-streamable', endpoint_url: 'https://e/mcp',
    tools: [{ name: 't', description: 'd', read_only: true, requires_personal_auth: false, scopes: [] }],
    identity_fingerprint: 'f', expires_at: '2026-09-23T00:00:00Z',
  } });
  assert.equal(value.previewId, 'p1');
  assert.equal(value.tools.length, 1);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm gates`（或 `node --test packages/api-client/src/plugins.test.ts`，需 node ≥26——PATH 为 v22 时必须经 gates）
Expected: FAIL —— `plugins.ts` 不存在。

- [ ] **Step 3: 实现 `plugins.ts`**（`parsePluginPreview` 严格校验器 + `previewInstallation` POST 封装，错误文案与既有 api-client 惯例一致）。

- [ ] **Step 4: 运行确认通过**

Run: `pnpm gates`
Expected: 全部 gate 通过（含 typecheck:web / test:web / check:integrity——`check:integrity` 若校验 api-client 导出清单，需同步登记导出，执行时按 gate 报错提示补）。

- [ ] **Step 5: 写失败测试（面板渲染）**

`PluginsSettingsPanel.test.tsx`（模式 McpSettingsPanel.test.tsx：node:test + renderToStaticMarkup + CSS hook）：

```tsx
test('插件面板管理员可提交清单地址并渲染预览结果', async () => {
  // client stub：previewInstallation 返回固定 PluginPreviewResult。
  // 提交 URL 后断言 html 匹配 plugin_id、version、endpoint、工具行、只读/需授权徽标。
});
test('插件面板渲染校验失败错误', async () => {
  // stub 抛错（如"清单 URL rejected"）→ 断言错误文案出现且无预览卡。
});
```

（异步渲染用 `renderToStaticMarkup` 的局限下，可将"提交→渲染"逻辑抽为组件内导出的纯函数 `reducePluginPreviewState(state, action)` 测试状态机，SSR 断言初始与结果两态——与 McpSettingsPanel 导出纯函数的既有惯例一致。）

- [ ] **Step 6: 运行确认失败**

Run: `pnpm gates`
Expected: FAIL —— 组件不存在。

- [ ] **Step 7: 实现面板**（输入框 + 提交按钮 + 预览卡：插件名/版本/端点/工具表（名称、读写分类徽标、是否需个人授权、scope）+ 有效期显示 + 错误条；本任务不含"确认安装"按钮——T08 加入）；挂载 SettingsPage 与 registry。

- [ ] **Step 8: 运行确认通过**

Run: `pnpm gates`
Expected: 全部通过。

- [ ] **Step 9: Commit**

```bash
git add packages/api-client/src/plugins.ts packages/api-client/src/plugins.test.ts packages/views/src/settings/registry.ts apps/web/src/settings/
git commit -m "feat(plugins): 管理端清单预览面板与 API 客户端 [T03]"
```
