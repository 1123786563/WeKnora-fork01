# T19 飞书文档发布端到端闭环（Issue #49）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 复用 #48 的 Publication seam，为飞书 docx 增加与 Notion 同构的发布闭环（创建/更新、发布前版本读取、确定内容审批、部分成功恢复与核对、回执），并使飞书差异只存在于 Adapter/桥层。

**Architecture:** 把 `internal/modules/appconnector/publish` 中 `NotionPublishService`（`plan.go`）的 provider 特定点（含 AC1 预读判定的飞书语义适配，共 8 处点状修改）提取为 `ProviderProfile`（新文件 `provider.go`），服务层变为 provider 中立；新增 `internal/modules/appconnector/feishu_docx.go`（飞书 docx 合同常量、快照解析、块投影、`FeishuDocxAdapter`，全部对照官方 oapi-sdk-go v3.9.7 `service/docx/v1` 源码锚定）与 `publish/feishu.go`（`FeishuBridge`：ActionDispatcher + UnknownResolver + 预读）；HTTP 面镜像 Notion（`/api/v1/apps/feishu-publish`）。`app_publications` 表（#48 迁移 000194/000115）的 `provider` 列本就预留 `"feishu"`，本计划零新迁移。

**Tech Stack:** Go 1.x（go.mod 模块 `github.com/Tencent/WeKnora`）、gorm + sqlite 内存库（单测）/ golang-migrate 全量生产迁移（E2E）、gin、testify/require、httptest 契约替身、官方 `github.com/larksuite/oapi-sdk-go/v3 v3.9.7`（仅作合同事实源，运行时不使用其 client——与 `feishu_send.go:148-154` 同一纪律：SDK transport 绕过 A04 出站策略校验）。

**Spec:** docs/specs/2026-09-20-mobile-ai-office-design.md（外部发布）；领域术语 CONTEXT.md「外部发布（External Publication）」（CONTEXT.md:342）；上游事实源 docs/plans/issue30-sweep/issues/issue-49.md；被复用 seam 见 docs/plans/issue30-sweep/plans/plan-t48.md。

## Global Constraints

以下逐字来自批准需求（issue-49.md 正文）与既有约束；每条同时约束全部任务：

- 「复用 Publication seam 完成飞书创建/更新、版本读取、确定内容审批、部分成功核对和回执。」
- 「飞书差异只存在于 Adapter。」
- 「读取/同步权限不会自动升级为写权限。」
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」
- Blocked by #48 —— `publish` seam 已在 HEAD（本计划实测 `internal/modules/appconnector/publish/{plan,dispatcher,blocks}.go` 与 `internal/modules/appconnector/repository/appconnector/publication.go` 均存在并已集成）。
- 外部凭据类验收（真实飞书 API）本环境无凭据：对应验收项如实列为 blocked-env，给出本地可验证等价替身（全迁移 sqlite + 契约替身的 E2E），opt-in 真实集成证据走环境变量门控（`FEISHU_*`，参照既有 `NOTION_TOKEN`/`FS_APP_ID` 模式），不得伪造通过。
- 服务端出站请求仅允许 http/https，且必须经 A04 `HTTPPolicy.ValidateRequest`（host 钉死 `open.feishu.cn`，测试替身走 loopback `AuthorizedNetworks`，先例 `internal/handler/app_connector_notion_publish_e2e_test.go:330-341`）。
- 数据库查询全部参数绑定（gorm `Where("id = ?", …)` 形态，沿用 `publication.go` 既有写法）。
- 凭据只从环境变量或既有加密凭据体系（`appconnectorsvc.CredentialResolver`）读取；源码与测试不写可用凭据字面量（测试 token 一律 `secret_test_token` 假值）。
- 波级问题 1（迁移撞号）：本计划 Task 5 E2E 需要全量迁移装载，sqlite 轨道 `000114` 与 versioned 轨道 `000193` 各有双迁移（`mobile_device_app` 与 `public_agent_marketplace`），实测 `go test ./internal/database/` FAIL（`duplicate migration file: 000114_public_agent_marketplace.down.sql`）。Task 0 必须先重编。
- 并行批次约束：对 #48 已集成文件的修改收敛为「`plan.go` 7 处点状替换 + `dispatcher.go` +2 行 + `router.go` 2 行 + `container/container.go` 1 行」；其余全部落在本计划独有新文件。

## Review Focus

规范沉默但最可能咬人的五类输入，每条已钉到拥有该代码的任务：

1. **只读/同步 scope 的飞书连接被用于发布**（schema_json `scopes` 不含 `write_docx`，例如同步导入类连接）→ 发布终局必须 failed 且零网络写调用，绝不因连接存在而升级权限（AC2）。测试归属：Task 2 Step 1（Adapter capability 拒绝，零 HTTP 调用断言）与 Task 5 Test 4（E2E 全链 fail closed）。
2. **`revision_id` 是 JSON 数字而快照 `expected_revision` 是字符串**（官方 `Document.RevisionId *int`，SDK `model.go:2143`）→ Adapter 读取后必须 `strconv.Itoa` 字符串化再比对；远端 revision 缺失/为 0 视为不可读，绝不伪造版本 token。测试归属：Task 1 Step 1（`TestDetectFeishuRevisionConflict` 含数字形状）与 Task 2（fake 返回数字 revision）。
3. **子块分页被忽略导致核对漏块**（`GET .../children` 返回 `items`+`has_more`+`page_token`，SDK `model.go:8862-8870`）→ Query 核对必须循 `page_token` 收集全部子块，只比对首页会把未取回的块误判为已提交。测试归属：Task 2 Step 1（fake 分页双页，`TestFeishuDocxAdapterQueryFollowsPagination`）。
4. **飞书创建文档合同无 title 参数**（`CreateDocumentReqBody` 仅 `folder_token`，SDK `model.go:7895-7896`）→ 批准的 title 只作审批绑定与台账元数据，Adapter 不发送、不比对远端标题；这是与 Notion 的已知行为差异，必须在 Task 1 注释与计划差异记录中显式声明，不得"顺手"引入未经 SDK 锚定的重命名 API。
5. **Query 核对用整块字节比对会永远失败**（飞书返回的 block 携带 `block_id`/`parent_id` 等服务端字段，与快照规范化字节不同——Notion 同位置的字节比对在真实环境下同样退化为 unknown）→ 飞书 Query 必须做语义比对：解析每个子块的 `text.elements[].text_run.content` 序列与快照块逐一比对。测试归属：Task 2 Step 1（fake 返回带 `block_id` 的真实形状块，`TestFeishuDocxAdapterQueryReconcilesByContent`）。

---

## File Structure

```
migrations/
  sqlite/000118_mobile_device_app.{up,down}.sql        # Task 0：git mv 自 000114（若集成时未收编）
  versioned/000197_mobile_device_app.{up,down}.sql     # Task 0：git mv 自 000193（若集成时未收编）
internal/modules/appconnector/
  feishu_docx.go                    # 新建（Task 1+2）：FE-PUB-01 合同常量、哨兵错误、快照解析、
                                    #   FeishuTextBlocks 块投影、版本检测、FeishuDocxAdapter（Execute/Query）、
                                    #   ReadFeishuDocumentVersion 预读
  feishu_docx_test.go               # 新建（Task 1+2）：纯函数单测 + fakeFeishuDocx 契约替身 + Adapter 行为测试
  feishu_publish_real_test.go       # 新建（Task 6）：opt-in 真实 Provider 证据（FEISHU_* 门控，本环境 SKIP）
  publish/
    provider.go                     # 新建（Task 3）：ProviderProfile + NotionProfile() + FeishuProfile()
    feishu.go                       # 新建（Task 3）：FeishuBridge（dispatcher+resolver+预读）、
                                    #   飞书常量策略 provider、production progress、FeishuVersionConflictResult
    plan.go                         # 修改（Task 3）：8 处 provider 特定点走 profile（含预读判定）；新增 NewProviderPublishService；
                                    #   NewNotionPublishService 签名不变（#48 调用点零改动）
    dispatcher.go                   # 修改（Task 3）：NotionConnectionScope 增加 Scopes []string 透传（AC2 数据面）
    provider_test.go                # 新建（Task 3）：双 profile 表驱动（AC1 正面证据）
    feishu_test.go                  # 新建（Task 3）：FeishuBridge 路由/能力门/预读测试
internal/handler/
  app_connector_feishu_publish.go   # 新建（Task 4）：AppFeishuPublishHandler（镜像 Notion handler 谓词）
  app_connector_feishu_publish_test.go      # 新建（Task 4）：HTTP 门测试（镜像 notion_publish_test.go）
  app_connector_feishu_publish_e2e_test.go  # 新建（Task 5）：全迁移 E2E（AC1/AC2/AC3 证据）
internal/router/
  routes_app_feishu_publish.go      # 新建（Task 4）
  router.go                         # 修改（Task 4）：+1 参数字段 +1 Register 行
internal/container/
  feishu_publish.go                 # 新建（Task 4）：newFeishuPublishHandler dig 构造
  container.go                      # 修改（Task 4）：+1 行 Provide
internal/handler/mobile_device_test.go                         # Task 0：000114→000118 引用更新（若未收编）
internal/application/repository/mobile_device_test.go          # Task 0：同上
internal/application/repository/mobile_push_isolation_test.go  # Task 0：同上
internal/application/repository/mobile_device_app_test.go      # Task 0：同上（含 versioned 000193→000197）
internal/modules/workbench/service/workbench/notification_app_policy_test.go  # Task 0：同上
```

---

### Task 0: 迁移撞号去重重编（E2E 硬前置）

**Files:**
- Rename: `migrations/sqlite/000114_mobile_device_app.{up,down}.sql` → `migrations/sqlite/000118_mobile_device_app.{up,down}.sql`
- Rename: `migrations/versioned/000193_mobile_device_app.{up,down}.sql` → `migrations/versioned/000197_mobile_device_app.{up,down}.sql`
- Modify: `internal/handler/mobile_device_test.go:36`（列表第三项文件名）
- Modify: `internal/application/repository/mobile_device_test.go:31`
- Modify: `internal/application/repository/mobile_push_isolation_test.go:67`
- Modify: `internal/application/repository/mobile_device_app_test.go:37,188-189`
- Modify: `internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`

**Interfaces:**
- Consumes: HEAD 现状（两条迁移轨道）。本任务前期实测：sqlite `000114_mobile_device_app.*`（#67 引入，提交 `fb9037710`）与 `000114_public_agent_marketplace.*`（#60 引入，提交 `34565aa41`）同号；versioned `000193_*` 同样双占；`go test ./internal/database/` 实测 FAIL `duplicate migration file: 000114_public_agent_marketplace.down.sql`；#48 的 `TestAppPublicationsTableExistsAfterMigrations` 同因 FAIL。重编对象是后来者 `mobile_device_app`（git log 实证 `fb9037710` 晚于 `34565aa41`）。
- Produces: 可装载的全量迁移轨道（Task 5 E2E 依赖）；目标号 `000118`/`000197` 为 sqlite/versioned 轨道当前最大号（000117/000196）之后的第一个空闲号。

**前置条件声明（集成约定；worktree 状态是流动的）：** 撰写本计划期间实测到本 worktree 的迁移区状态发生过翻转——先存在同目标的未提交重编（000118/000197 + 5 个测试文件引用更新，重编后 `go test ./internal/database/` 与 `TestAppPublicationsTableExists` 实测 PASS；疑为同批另一作者如 plan-t51 所留），撰写后期实测该重编又被并行作者还原（000114/000193 双占复发、`TestAppPublicationsTableExists` 复测 FAIL）。多个并行计划共享同一 worktree，谁需要迁移装载谁就落 Task 0——**执行本任务时一律以执行时刻的实际状态按下面三态判定，不依赖撰写时刻的快照**：

- 若 `git status` 显示重编仍在（工作树含 `000118_mobile_device_app.*`），跳过 Step 1-3 的文件操作，只执行 Step 4 验证并提交现状；
- 若集成时该变更已被主控收编进 HEAD（`ls migrations/sqlite/000118_mobile_device_app.up.sql` 存在），本任务整体只执行 Step 4 验证，无需任何文件操作；
- 若两者都不成立（HEAD 干净且撞号复发），按 Step 1-3 执行完整重编。若届时 `000118`/`000197` 已被其它批次占用，按「下一个空闲号」规则让位（重编前 `ls migrations/` 核对），并同步更新 5 个测试文件中的引用为目标号。

**Steps:**

- [ ] **Step 1: 重命名 sqlite 轨道迁移（内容零变化）**

```bash
git mv migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.up.sql
git mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
```

- [ ] **Step 2: 重命名 versioned 轨道迁移（内容零变化）**

```bash
git mv migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.up.sql
git mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
```

- [ ] **Step 3: 同步 5 个测试文件的迁移文件名引用**

顺序依赖已核实：sqlite `000115`–`000117` 与 versioned `000194`–`000196` 均不引用 `mobile_devices`/`mobile_notification_intents`（`grep -l` 实测零命中），重编到 000118/000197 不产生顺序倒置。用 sed 一次性替换（只动这 5 个文件）：

```bash
sed -i '' 's/000114_mobile_device_app/000118_mobile_device_app/g' \
  internal/handler/mobile_device_test.go \
  internal/application/repository/mobile_device_test.go \
  internal/application/repository/mobile_push_isolation_test.go \
  internal/application/repository/mobile_device_app_test.go \
  internal/modules/workbench/service/workbench/notification_app_policy_test.go
sed -i '' 's/000193_mobile_device_app/000197_mobile_device_app/g' \
  internal/application/repository/mobile_device_app_test.go \
  internal/modules/workbench/service/workbench/notification_app_policy_test.go
grep -rn "000114_mobile_device_app\|000193_mobile_device_app" --include="*.go" --include="*.sql" . | grep -v node_modules
```

Expected: 最后的 grep 无任何输出（引用改净）。

- [ ] **Step 4: 验证迁移轨道恢复**

```bash
go test ./internal/database/ -count=1
go test ./internal/handler/ -run 'TestAppPublicationsTableExistsAfterMigrations' -count=1
```

Expected: 两命令均 `ok`（Task 0 前实测 FAIL，重编后本 worktree 实测 PASS）。

- [ ] **Step 5: Commit**

```bash
git add migrations/ internal/handler/mobile_device_test.go \
  internal/application/repository/mobile_device_test.go \
  internal/application/repository/mobile_push_isolation_test.go \
  internal/application/repository/mobile_device_app_test.go \
  internal/modules/workbench/service/workbench/notification_app_policy_test.go
git commit -m "fix(migrations): 重编 mobile_device_app 迁移去重（sqlite 000118/versioned 000197），修复全量迁移轨道"
```

---

### Task 1: 飞书 docx 合同层——常量、哨兵错误、快照解析、块投影、版本冲突检测（纯函数）

**Files:**
- Create: `internal/modules/appconnector/feishu_docx.go`
- Test: `internal/modules/appconnector/feishu_docx_test.go`

**Interfaces:**
- Consumes: 同包既有 `HTTPPolicy`（`http_policy.go:67`）、`Action`、`NormalizeArgs`（`feishu_send.go` 域既有）、`FeishuAPIHost = "open.feishu.cn"`（`feishu_send.go:45`）。
- Produces（Task 2/3 消费，签名逐字）:
  - `const FeishuDocumentCreatePath = "/open-apis/docx/v1/documents"`、`FeishuDocumentGetFormat = "/open-apis/docx/v1/documents/%s"`、`FeishuDocumentChildrenFormat = "/open-apis/docx/v1/documents/%s/blocks/%s/children"`、`FeishuCapabilityWriteDocx = "write_docx"`、`FeishuDocAppendBatchLimit = 50`
  - `var ErrFeishuPublishSnapshotInvalid / ErrFeishuPublishMissingCapability / ErrFeishuPublishOutcomeUnknown / ErrFeishuPublishRevisionConflict / ErrFeishuPublishNotConfigured / ErrFeishuPublishNotFound error`（`ErrFeishuPublishNotFound`：目标在远端不存在——provider code 99991661 或 HTTP 404 的类型化形态，Task 3 桥的预读用它区分「folder 无版本概念」与「目标不可读」）
  - `type FeishuDocProgress struct { DocumentID string; BlocksDone int }`
  - `type FeishuDocCreateSnapshot struct { ParentFolder, Title string; Blocks []json.RawMessage }`；`func ParseFeishuDocCreateSnapshot(args json.RawMessage) (FeishuDocCreateSnapshot, error)`
  - `type FeishuDocUpdateSnapshot struct { DocumentID, ExpectedRevision, Title string; Blocks []json.RawMessage }`；`func ParseFeishuDocUpdateSnapshot(args json.RawMessage) (FeishuDocUpdateSnapshot, error)`
  - `func IsFeishuDocUpdateArgs(args json.RawMessage) bool`
  - `func DetectFeishuRevisionConflict(expected, actual string) error`
  - `func FeishuTextBlocks(text string) ([]json.RawMessage, error)`
  - `type FeishuDocVersion struct { DocumentID, RevisionID string }`；`func ParseFeishuDocumentVersion(raw []byte) (FeishuDocVersion, error)`
  - `type FeishuDocReceipt struct { ExternalID, ExternalVersion string }`；`func ParseFeishuDocReceipt(raw []byte) (FeishuDocReceipt, error)`
  - `func feishuDocBlockContents(raw []byte) ([]string, error)`（包内私有；Task 2 Query 语义比对用）

合同事实源（全部逐字核对自 `~/go/pkg/mod/github.com/larksuite/oapi-sdk-go/v3@v3.9.7/service/docx/v1/`，运行时不 import 该 SDK）：

| 事实 | SDK 锚点 |
|---|---|
| `POST /open-apis/docx/v1/documents`，body 仅 `{"folder_token": …}`（无 title 参数） | `resource.go:285`、`model.go:7895-7896` |
| `GET /open-apis/docx/v1/documents/:document_id` → `data.document.{document_id, revision_id(int), title}` | `resource.go:315`、`model.go:2140-2145` |
| `POST /open-apis/docx/v1/documents/:document_id/blocks/:block_id/children`，body `{"children": [Block…], "index": int}` → `data.children` + `data.document_revision_id(int)` | `resource.go:533`、`model.go:8742-8747`、`model.go:8753-8759` |
| `GET` 同路径（分页）→ `data.items` + `data.page_token` + `data.has_more` | `resource.go:563`、`model.go:8862-8870` |
| 文本块形状 `{"block_type":2,"text":{"elements":[{"text_run":{"content":…}}],"style":{}}}` | `model.go:502-510`（Block）、`model.go:5298`（Text）、`model.go:5346`（TextElement）、`model.go:5687`（TextRun） |
| 飞书响应信封 `{"code":int,"msg":string,"data":…}`，`code==0` 为成功 | `model.go:7923-7927`（Success() 判定） |
| 文档根块的 `block_id == document_id`（往文档顶层追加块时二者相同） | 飞书公开约定；本计划的 fake 与测试按此构造 |

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/feishu_docx_test.go`：

```go
package appconnector

// FE-PUB-01 contract-layer tests for the Feishu docx publish family
// (#49). The contract facts are pinned against the official oapi-sdk-go
// v3.9.7 docx/v1 service source; the fakes in Task 2 reproduce the wire
// shapes from that source.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFeishuDocCreateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"parent_folder":"fld-1","title":"Report","blocks":[{"block_type":2}]}`)
	snap, err := ParseFeishuDocCreateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "fld-1", snap.ParentFolder)
	require.Equal(t, "Report", snap.Title)
	require.Len(t, snap.Blocks, 1)
}

func TestParseFeishuDocCreateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":     json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[],"x":1}`),
		"missing title":   json.RawMessage(`{"parent_folder":"f","blocks":[]}`),
		"missing folder":  json.RawMessage(`{"title":"T","blocks":[]}`),
		"missing blocks":  json.RawMessage(`{"parent_folder":"f","title":"T"}`),
		"empty folder":    json.RawMessage(`{"parent_folder":"","title":"T","blocks":[]}`),
		"empty title":     json.RawMessage(`{"parent_folder":"f","title":"","blocks":[]}`),
		"invalid block":   json.RawMessage(`{"parent_folder":"f","title":"T","blocks":["not-json-object"]}`),
		"not an object":   json.RawMessage(`["parent_folder"]`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocCreateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestParseFeishuDocUpdateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"document_id":"doc-1","expected_revision":"3","title":"Report v2","blocks":[]}`)
	snap, err := ParseFeishuDocUpdateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "doc-1", snap.DocumentID)
	require.Equal(t, "3", snap.ExpectedRevision)
	require.Equal(t, "Report v2", snap.Title)
	require.Empty(t, snap.Blocks)
}

func TestParseFeishuDocUpdateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":          json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[],"x":1}`),
		"missing revision":     json.RawMessage(`{"document_id":"d","title":"T","blocks":[]}`),
		"empty revision":       json.RawMessage(`{"document_id":"d","expected_revision":"","title":"T","blocks":[]}`),
		"missing document":     json.RawMessage(`{"expected_revision":"1","title":"T","blocks":[]}`),
		"missing title":        json.RawMessage(`{"document_id":"d","expected_revision":"1","blocks":[]}`),
		"missing blocks":       json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T"}`),
		"blocks not json":      json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[12]}`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocUpdateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestIsFeishuDocUpdateArgs(t *testing.T) {
	require.True(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d"}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`not json`)))
}

func TestDetectFeishuRevisionConflict(t *testing.T) {
	require.NoError(t, DetectFeishuRevisionConflict("3", "3"))
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", "4"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("", "3"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", ""), ErrFeishuPublishRevisionConflict)
	// The numeric wire shape must not leak into the comparison: "03" vs
	// "3" is a mismatch — the adapter always stores strconv.Itoa output.
	require.ErrorIs(t, DetectFeishuRevisionConflict("03", "3"), ErrFeishuPublishRevisionConflict)
}

func TestFeishuTextBlocksDerivesTextParagraphs(t *testing.T) {
	blocks, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	var first struct {
		BlockType int `json:"block_type"`
		Text      struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
			Style struct{} `json:"style"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &first))
	require.Equal(t, 2, first.BlockType, "block_type 2 = text block")
	require.Len(t, first.Text.Elements, 1)
	require.Equal(t, "第一段。", first.Text.Elements[0].TextRun.Content)

	// Deterministic: the same bytes always derive the same blocks.
	again, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Equal(t, blocks, again)
}

func TestFeishuTextBlocksEmptyAndOversize(t *testing.T) {
	_, err := FeishuTextBlocks("  \n\n  ")
	require.ErrorIs(t, err, ErrFeishuPublishEmptyContent)

	long := strings.Repeat("段", feishuDocMaxParagraphs+1)
	_, err = FeishuTextBlocks(long)
	require.ErrorIs(t, err, ErrFeishuPublishContentTooLarge)
}

func TestFeishuTextBlocksChunksLongParagraph(t *testing.T) {
	long := strings.Repeat("x", feishuTextRunChunk+10)
	blocks, err := FeishuTextBlocks(long)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	joined := ""
	var parsed struct {
		Text struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &parsed))
	for _, e := range parsed.Text.Elements {
		joined += e.TextRun.Content
	}
	require.Equal(t, long, joined, "chunking must preserve the full content")
}

func TestParseFeishuDocumentVersionReadsNumericRevision(t *testing.T) {
	// data.document.revision_id is a JSON NUMBER on the wire (SDK
	// Document.RevisionId *int); the parser stringifies it.
	raw := []byte(`{"code":0,"data":{"document":{"document_id":"doc-1","revision_id":7,"title":""}}}`)
	v, err := ParseFeishuDocumentVersion(raw)
	require.NoError(t, err)
	require.Equal(t, "doc-1", v.DocumentID)
	require.Equal(t, "7", v.RevisionID)

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":0,"data":{"document":{"document_id":"doc-1"}}}`))
	require.Error(t, err, "a reply without a revision is never a version")

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991663,"msg":"denied"}`))
	require.Error(t, err)

	// The official not-exist code is TYPED: the plan pre-read tells
	// "folder has no revision" apart from an unreadable destination.
	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991661,"msg":"not exist"}`))
	require.ErrorIs(t, err, ErrFeishuPublishNotFound)
}

func TestParseFeishuDocReceipt(t *testing.T) {
	rcpt, err := ParseFeishuDocReceipt([]byte(`{"document":{"document_id":"doc-9","revision_id":12}}`))
	require.NoError(t, err)
	require.Equal(t, "doc-9", rcpt.ExternalID)
	require.Equal(t, "12", rcpt.ExternalVersion)
}

func TestFeishuDocBlockContentsExtractsTextRuns(t *testing.T) {
	raw := []byte(`[{"block_id":"b1","parent_id":"doc-1","block_type":2,"text":{"elements":[{"text_run":{"content":"a"}}],"style":{}"}},
		{"block_id":"b2","block_type":2,"text":{"elements":[{"text_run":{"content":"b"}}]}}]`)
	got, err := feishuDocBlockContents(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, got)
}
```

注意：`feishu_docx.go` 位于父包 `appconnector`，不能使用子包 `publish` 的哨兵（包依赖边界见 Step 3），因此空内容/超限哨兵用本包的 `ErrFeishuPublishEmptyContent` / `ErrFeishuPublishContentTooLarge`、上限用本包常量 `feishuDocMaxParagraphs`（均已列入 Produces）。

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/modules/appconnector/ -run 'TestParseFeishuDoc|TestIsFeishuDocUpdateArgs|TestDetectFeishuRevisionConflict|TestFeishuTextBlocks|TestParseFeishuDocumentVersion|TestParseFeishuDocReceipt|TestFeishuDocBlockContents' -count=1
```

Expected: FAIL，`undefined: ParseFeishuDocCreateSnapshot` 等编译错误。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/feishu_docx.go`：

```go
package appconnector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
)

// FE-PUB-01 fixed contract for the Feishu docx publish family (#49),
// validated against the official oapi-sdk-go v3.9.7 docx/v1 service
// source (service/docx/v1/resource.go + model.go). The SDK client itself
// is NOT used at runtime (its transport would bypass the A04 outbound
// policy — the same discipline as the FS-01 IM send family); these
// constants pin the method contract instead. Nothing here is derived
// from model output.
const (
	// FeishuDocumentCreatePath is POST /open-apis/docx/v1/documents
	// (SDK resource.go:285). The contract body carries ONLY folder_token
	// (SDK model.go:7895-7896) — the create-document API has NO title
	// parameter. The approved title therefore stays approval/ledger
	// metadata (recorded on the publication row); it is never sent and
	// never compared against the remote title.
	FeishuDocumentCreatePath = "/open-apis/docx/v1/documents"
	// FeishuDocumentGetFormat is GET /open-apis/docx/v1/documents/{id}
	// (SDK resource.go:315) — the reliable version read; the version
	// token is document.revision_id (a JSON number, stringified here).
	FeishuDocumentGetFormat = "/open-apis/docx/v1/documents/%s"
	// FeishuDocumentChildrenFormat is the nested-block children resource
	// (SDK resource.go:533 POST / :563 GET) under the document's root
	// block, whose block_id equals the document_id.
	FeishuDocumentChildrenFormat = "/open-apis/docx/v1/documents/%s/blocks/%s/children"
	// FeishuCapabilityWriteDocx is the reviewed schema_json scope that
	// grants document WRITES. A connection whose reviewed scopes carry
	// only read/sync capabilities never satisfies the publish capability
	// check (AC2: 读取/同步权限不会自动升级为写权限).
	FeishuCapabilityWriteDocx = "write_docx"
	// FeishuDocAppendBatchLimit is the official per-request children cap
	// of the create-children API. The adapter never exceeds it; MaxBatch
	// (Task 2) only lowers it.
	FeishuDocAppendBatchLimit = 50
	// feishuTextRunChunk is the conservative chunk (runes) per
	// text_run.content object — below any documented provider cap, and
	// identical to the Notion projection's conservative value.
	feishuTextRunChunk = 1900
)

// Feishu docx publish rejections. Distinct from the FS-01 IM send
// sentinels: a different provider family fails with its own names.
var (
	// ErrFeishuPublishSnapshotInvalid: the action arguments are not
	// exactly the approved snapshot (an extra field, a missing field, a
	// non-JSON block) — refused rather than silently forwarded.
	ErrFeishuPublishSnapshotInvalid = errors.New("feishu_docx_snapshot_invalid")
	// ErrFeishuPublishMissingCapability: the connection does not carry
	// the reviewed write_docx capability (AC2).
	ErrFeishuPublishMissingCapability = errors.New("feishu_docx_missing_capability")
	// ErrFeishuPublishOutcomeUnknown: the request may or may not have
	// produced its remote effect. Resolves ONLY via Query's reliable
	// read — never via a second create.
	ErrFeishuPublishOutcomeUnknown = errors.New("feishu_docx_outcome_unknown")
	// ErrFeishuPublishRevisionConflict: the remote document's current
	// revision no longer equals the approved expected_revision; the
	// update is refused BEFORE any write (CONTEXT.md「外部发布」: 再次
	// 更新前必须读取外部当前版本并形成新的候选变更).
	ErrFeishuPublishRevisionConflict = errors.New("feishu_docx_revision_conflict")
	// ErrFeishuPublishNotConfigured: the adapter is missing a reviewed
	// outbound policy or a token source — fail closed.
	ErrFeishuPublishNotConfigured = errors.New("feishu_docx_adapter_not_configured")
	// ErrFeishuPublishEmptyContent: the artifact carries no publishable text.
	ErrFeishuPublishEmptyContent = errors.New("feishu_docx_empty_content")
	// ErrFeishuPublishContentTooLarge: the derived plan exceeds the publish bounds.
	ErrFeishuPublishContentTooLarge = errors.New("feishu_docx_content_too_large")
	// ErrFeishuPublishNotFound: the remote target does not exist (provider
	// code 99991661 or HTTP 404). The plan-time pre-read distinguishes
	// this typed shape from transport failure: a create destination (the
	// reviewed FOLDER) legitimately has no document revision, while an
	// unreadable update target must fail the plan.
	ErrFeishuPublishNotFound = errors.New("feishu_docx_target_not_found")
)

// FeishuDocProgress is the persisted recovery record of a multi-step
// docx publication — the FE-03 counterpart of NotionPageProgress
// (notion_create.go:106-109): the REAL document id, persisted the moment
// the provider confirms the create BEFORE any content step, plus the
// number of leading snapshot blocks already appended, persisted after
// every successful batch. A crash between steps resumes from exactly
// this record, so a second document is never created and committed
// blocks are never re-sent.
type FeishuDocProgress struct {
	DocumentID string
	BlocksDone int
}

// FeishuDocCreateSnapshot is the A03-approved argument snapshot for one
// document creation: exactly parent_folder (the reviewed destination
// folder), title (approval/ledger metadata — see the create-path note
// above) and blocks.
type FeishuDocCreateSnapshot struct {
	ParentFolder string
	Title        string
	Blocks       []json.RawMessage
}

// ParseFeishuDocCreateSnapshot validates that args are EXACTLY the
// approved three-field create snapshot.
func ParseFeishuDocCreateSnapshot(args json.RawMessage) (FeishuDocCreateSnapshot, error) {
	var s FeishuDocCreateSnapshot
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return s, err
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly parent_folder, title, blocks", ErrFeishuPublishSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["parent_folder"], &s.ParentFolder); err != nil {
		return s, fmt.Errorf("%w: parent_folder: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if s.ParentFolder == "" {
		return s, fmt.Errorf("%w: empty parent_folder", ErrFeishuPublishSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrFeishuPublishSnapshotInvalid)
	}
	s.Blocks, err = feishuDocBlocksField(raw["blocks"])
	if err != nil {
		return s, err
	}
	return s, nil
}

// FeishuDocUpdateSnapshot is the A03-approved argument snapshot for
// appending to ONE existing external document: exactly document_id,
// expected_revision (the revision the plan read before approval — the
// immutable approval anchor), title and blocks. Blocks may be empty.
type FeishuDocUpdateSnapshot struct {
	DocumentID      string
	ExpectedRevision string
	Title           string
	Blocks          []json.RawMessage
}

// ParseFeishuDocUpdateSnapshot validates that args are EXACTLY the
// approved four-field update snapshot.
func ParseFeishuDocUpdateSnapshot(args json.RawMessage) (FeishuDocUpdateSnapshot, error) {
	var s FeishuDocUpdateSnapshot
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return s, err
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly document_id, expected_revision, title, blocks", ErrFeishuPublishSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["document_id"], &s.DocumentID); err != nil {
		return s, fmt.Errorf("%w: document_id: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["expected_revision"], &s.ExpectedRevision); err != nil {
		return s, fmt.Errorf("%w: expected_revision: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if s.DocumentID == "" {
		return s, fmt.Errorf("%w: empty document_id", ErrFeishuPublishSnapshotInvalid)
	}
	if s.ExpectedRevision == "" {
		return s, fmt.Errorf("%w: empty expected_revision", ErrFeishuPublishSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrFeishuPublishSnapshotInvalid)
	}
	s.Blocks, err = feishuDocBlocksField(raw["blocks"])
	if err != nil {
		return s, err
	}
	return s, nil
}

// IsFeishuDocUpdateArgs reports whether args carry the update snapshot's
// distinguishing key pair (document_id AND expected_revision). It never
// parses the full snapshot — the bridge routes on it; full validation
// happens in ParseFeishuDocUpdateSnapshot.
func IsFeishuDocUpdateArgs(args json.RawMessage) bool {
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return false
	}
	_, hasDoc := raw["document_id"]
	_, hasRev := raw["expected_revision"]
	return hasDoc && hasRev
}

// DetectFeishuRevisionConflict compares the approved expected revision
// with the revision just read from the provider. Anything but an exact
// string match — including an unreadable empty side — is a conflict; an
// unobservable remote state must never authorize an overwrite.
func DetectFeishuRevisionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrFeishuPublishRevisionConflict, expected, actual)
	}
	return nil
}

// feishuTextElement / feishuTextBlock mirror the official block shapes
// (SDK Block:502, Text:5298, TextElement:5346, TextRun:5687).
type feishuTextElement struct {
	TextRun struct {
		Content string `json:"content"`
	} `json:"text_run"`
}

type feishuTextBlock struct {
	BlockType int                 `json:"block_type"`
	Text      struct {
		Elements []feishuTextElement `json:"elements"`
		Style    struct{}            `json:"style"`
	} `json:"text"`
}

// FeishuTextBlocks derives Feishu text blocks from plain text — the
// deterministic pure counterpart of NotionParagraphBlocks
// (publish/blocks.go:53): paragraphs split on blank lines, each trimmed,
// long paragraphs chunked into ≤feishuTextRunChunk-rune text_run content
// objects. The same artifact bytes always produce the same blocks, so
// the approval digest pins exactly what will be sent.
func FeishuTextBlocks(text string) ([]json.RawMessage, error) {
	blocks, err := publish.DeriveParagraphs(text)
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(blocks))
	for _, p := range blocks {
		block := feishuTextBlock{BlockType: 2}
		runes := []rune(p)
		chunks := make([]string, 0, len(runes)/feishuTextRunChunk+1)
		for start := 0; start < len(runes); start += feishuTextRunChunk {
			end := start + feishuTextRunChunk
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[start:end]))
		}
		if len(chunks) > 100 {
			return nil, fmt.Errorf("%w: one paragraph needs %d text_run objects", ErrFeishuPublishContentTooLarge, len(chunks))
		}
		for _, c := range chunks {
			var el feishuTextElement
			el.TextRun.Content = c
			block.Text.Elements = append(block.Text.Elements, el)
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// FeishuDocVersion is the reliable read shape of the get-document call:
// the exact document id and its revision as a string (the wire value is
// a JSON number — parsed with UseNumber and stringified via Itoa so the
// snapshot's expected_revision and the live value compare byte-for-byte).
type FeishuDocVersion struct {
	DocumentID string
	RevisionID string
}

// FeishuDocReceipt is the persisted external receipt of one publish:
// the provider document id and the revision the publish itself produced
// (read back from the provider's own reply — never fabricated locally).
type FeishuDocReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseFeishuDocReceipt extracts the receipt fields from a provider
// reply payload (the create/update evidence the adapter recorded as the
// action's output).
func ParseFeishuDocReceipt(raw []byte) (FeishuDocReceipt, error) {
	v, err := ParseFeishuDocumentVersion(raw)
	if err != nil {
		return FeishuDocReceipt{}, err
	}
	return FeishuDocReceipt{ExternalID: v.DocumentID, ExternalVersion: v.RevisionID}, nil
}

type feishuDocEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Document *struct {
			DocumentID string `json:"document_id"`
			RevisionID *int   `json:"revision_id"`
			Title      string `json:"title"`
		} `json:"document"`
	} `json:"data"`
}

// ParseFeishuDocumentVersion extracts the document identity + current
// revision from a get-document reply. A reply without a real id or a
// real (present, non-nil) revision is an error — a fabricated version is
// never a basis for conflict detection or a receipt.
func ParseFeishuDocumentVersion(raw []byte) (FeishuDocVersion, error) {
	var env feishuDocEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return FeishuDocVersion{}, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if env.Code != 0 {
		if env.Code == 99991661 { // the official "not exist / no permission" code
			return FeishuDocVersion{}, fmt.Errorf("%w: code=%d msg=%s", ErrFeishuPublishNotFound, env.Code, env.Msg)
		}
		return FeishuDocVersion{}, fmt.Errorf("feishu_provider_error: code=%d msg=%s", env.Code, env.Msg)
	}
	if env.Data.Document == nil || env.Data.Document.DocumentID == "" || env.Data.Document.RevisionID == nil {
		return FeishuDocVersion{}, fmt.Errorf("%w: reply carries no real document id/revision", ErrFeishuPublishOutcomeUnknown)
	}
	return FeishuDocVersion{
		DocumentID: env.Data.Document.DocumentID,
		RevisionID: strconv.Itoa(*env.Data.Document.RevisionID),
	}, nil
}

// feishuDocBlockContents extracts the text_run content sequence from a
// children payload (a JSON array of provider blocks). Query reconciliation
// compares THIS semantic projection — provider blocks carry server-side
// fields (block_id/parent_id/…) the approved snapshot bytes never have,
// so whole-block byte comparison would report every real success as
// unverifiable (see Review Focus 5).
func feishuDocBlockContents(raw []byte) ([]string, error) {
	var blocks []struct {
		Text struct {
			Elements []feishuTextElement `json:"elements"`
		} `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		joined := ""
		for _, el := range b.Text.Elements {
			joined += el.TextRun.Content
		}
		out = append(out, joined)
	}
	return out, nil
}

// ---- shared parsing helpers ----

func feishuDocArgsObject(args json.RawMessage) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	return raw, nil
}

func feishuDocBlocksField(field json.RawMessage) ([]json.RawMessage, error) {
	var blocks []json.RawMessage
	if err := json.Unmarshal(field, &blocks); err != nil {
		return nil, fmt.Errorf("%w: blocks: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	out := make([]json.RawMessage, 0, len(blocks))
	for i, b := range blocks {
		n, err := NormalizeArgs(b)
		if err != nil {
			return nil, fmt.Errorf("%w: block %d: %v", ErrFeishuPublishSnapshotInvalid, i, err)
		}
		out = append(out, n)
	}
	return out, nil
}
```

**包依赖边界（写实现前必读）：** `feishu_docx.go` 位于父包 `appconnector`；子包 `publish` import 了父包（`publish/plan.go:11`），因此父包**不能**反向 import `publish`——`FeishuTextBlocks` 的段落切分必须自带（不得复用 `publish.DeriveParagraphs`/`ErrPublishEmptyContent`），段数上限用本文件常量 `feishuDocMaxParagraphs = 500`（与 `publish/blocks.go:19` 的 `MaxPublishBlocks` 同值同义）。同理上文实现代码中的 import 修正为：去掉 `"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"`、补入 `"strings"`；`FeishuTextBlocks` 开头的 `blocks, err := publish.DeriveParagraphs(text)` 整段替换为自带切分：

```go
func FeishuTextBlocks(text string) ([]json.RawMessage, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	if len(paragraphs) == 0 {
		return nil, ErrFeishuPublishEmptyContent
	}
	if len(paragraphs) > feishuDocMaxParagraphs {
		return nil, fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrFeishuPublishContentTooLarge, len(paragraphs), feishuDocMaxParagraphs)
	}
	out := make([]json.RawMessage, 0, len(paragraphs))
	// 余下与上文 for p := range paragraphs 循环逐字相同（chunk 切分 + marshal）。
```

并在常量块追加 `feishuDocMaxParagraphs = 500 // 与 Notion 投影上限同值同义（publish/blocks.go:19）`。测试文件中 `MaxPublishBlocks+1` 相应改为 `feishuDocMaxParagraphs+1`（上文 Step 1 的第二版测试已是如此）。

- [ ] **Step 4: 运行测试通过**

```bash
go test ./internal/modules/appconnector/ -run 'TestParseFeishuDoc|TestIsFeishuDocUpdateArgs|TestDetectFeishuRevisionConflict|TestFeishuTextBlocks|TestParseFeishuDocumentVersion|TestParseFeishuDocReceipt|TestFeishuDocBlockContents' -count=1
```

Expected: PASS（全部新测试）。

- [ ] **Step 5: 确认既有 Notion/发送域零回归**

```bash
go test ./internal/modules/appconnector/ -run 'TestNotion|TestFeishuSend|TestFeishuRealControlledSend' -count=1
```

Expected: PASS（既有测试不受影响）。

- [ ] **Step 6: Commit**

```bash
git add internal/modules/appconnector/feishu_docx.go internal/modules/appconnector/feishu_docx_test.go
git commit -m "feat(appconnector): 飞书 docx 发布合同层——常量/快照解析/块投影/版本检测（FE-PUB-01）"
```

---

### Task 2: 飞书 docx Adapter——Execute/Query、部分成功恢复、可靠核对

**Files:**
- Modify: `internal/modules/appconnector/feishu_docx.go`（追加 Adapter 与传输层）
- Test: `internal/modules/appconnector/feishu_docx_test.go`（追加 fakeFeishuDocx 契约替身与 Adapter 测试）

**Interfaces:**
- Consumes: Task 1 全部产出；既有 `HTTPPolicy.ValidateRequest/NewClient`（`http_policy.go`）、`Action`、`ActionResult{State, ExternalID, Output}`、状态常量 `ActionFailed/ActionUnknown/ActionSucceeded/ActionAwaitingApproval`、`NormalizeArgs`。
- Produces（Task 3 消费，签名逐字）:
  - `type FeishuDocxAdapter struct { Policy HTTPPolicy; Token func(ctx context.Context) (string, error); ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error); Recheck func(ctx context.Context, a Action) error; LoadProgress func(a Action) FeishuDocProgress; SaveProgress func(a Action, p FeishuDocProgress) error; MaxBatch int }`
  - `var _ Adapter = (*FeishuDocxAdapter)(nil)`；`func (m *FeishuDocxAdapter) Execute(ctx context.Context, a Action) (ActionResult, error)`；`func (m *FeishuDocxAdapter) Query(ctx context.Context, a Action) (ActionResult, error)`
  - `func ReadFeishuDocumentVersion(ctx context.Context, policy HTTPPolicy, token func(context.Context) (string, error), documentID string) (FeishuDocVersion, error)`（计划预读；`publish.FeishuBridge.ReadPageVersion` 委托它）

执行语义与 Notion 家族逐点对齐（`notion_create.go:488-560` / `notion_update.go:399-505`）：

- **顺序**：A03 Recheck → 快照解析 → capability 检查（AC2，零网络写之前）→ update 分支先版本预读 + 冲突检测（预读是 GET，绝不可能产生写，预读失败=终局 failed）→ 多步写入，步间持久化 progress。
- **create**：`POST FeishuDocumentCreatePath`（body `{"folder_token": snap.ParentFolder}`）→ provider 确认的 `document_id` **先落 progress**（`FeishuDocProgress{DocumentID, 0}`）→ 循环批量 `POST children`（`block_id=document_id`、`index:-1`、每批 ≤`batchSize()`）→ 每批成功后落 `BlocksDone`。输出证据 = create 回复原文（含 `document_id`+`revision_id`）。
- **update**：`GET document` 读当前 revision → `DetectFeishuRevisionConflict` → 循环批量 append 到 `snap.DocumentID` → 读回 `GET document`（读回丢失=unknown，不谎报 failed——写已发生）。输出证据 = 读回的 envelope（含新 revision）。
- **unknown 判定**：传输失败/5xx/响应不可解析 → `ErrFeishuPublishOutcomeUnknown`；progress 持久化失败 → unknown（效果可能已在远端）。快照畸形/capability 缺失/预读失败/provider 4xx → 终局 failed。
- **Query（部分成功核对）**：无持久化 `DocumentID` → 诚实 unknown（飞书无可靠的标题搜索等价物，永不凭标题认领）。有 → `GET document` 存在性 + 循环 `page_token` 收集全部 children → **语义比对**：`feishuDocBlockContents` 提取 text_run 内容序列，与快照块的序列逐一比对（create：前缀精确相等；update：连续段包含）。全部证实 → succeeded + 回执；否则保持 unknown。
- **Query 比对的是 content 序列而非整块字节**（Review Focus 5）：fake 与 E2E 的 children 响应携带 `block_id`/`parent_id` 等服务端字段。

- [ ] **Step 1: 写失败测试**

向 `internal/modules/appconnector/feishu_docx_test.go` 追加（文件顶部 import 增加 `context`、`fmt`、`net`、`net/http`、`net/http/httptest`、`strings`、`sync`、`time`）：

```go
// ---- the docx contract double: reproduces the official docx/v1 wire
// shapes (envelope {code,msg,data}; numeric revision_id; children carry
// server-side fields) from the SDK source. Same self-containment
// discipline as the handler package's e2eNotion. ----

type fakeFeishuDocx struct {
	mu             sync.Mutex
	token          string
	docs           map[string]*fakeFeishuDoc
	nextID         int
	revisionBase   int
	createCalls    int
	appendCalls    int
	appendSizes    []int
	dropNextCreate bool
	dropNextAppend bool
}

type fakeFeishuDoc struct {
	id       string
	revision int
	children []json.RawMessage
}

func newFakeFeishuDocx(token string) *fakeFeishuDocx {
	return &fakeFeishuDocx{token: token, docs: map[string]*fakeFeishuDoc{}, revisionBase: 1}
}

func (f *fakeFeishuDocx) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+f.token }
	mux.HandleFunc("/open-apis/docx/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		if r.Method != http.MethodPost {
			write(w, `{"code":99991661,"msg":"method"}`)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.dropNextCreate {
			f.dropNextCreate = false
			panic(http.ErrAbortHandler)
		}
		f.createCalls++
		var req struct {
			FolderToken string `json:"folder_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.nextID++
		id := fmt.Sprintf("doc-%d", f.nextID)
		f.docs[id] = &fakeFeishuDoc{id: id, revision: f.revisionBase}
		write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, id, f.revisionBase))
	})
	mux.HandleFunc("/open-apis/docx/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/open-apis/docx/v1/documents/")
		switch {
		case !strings.Contains(rest, "/blocks/"):
			// GET /open-apis/docx/v1/documents/{id} — the version read.
			id := rest
			f.mu.Lock()
			defer f.mu.Unlock()
			d, ok := f.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, d.id, d.revision))
		case strings.HasSuffix(rest, "/children"):
			// {id}/blocks/{block_id}/children — POST append / GET list.
			segs := strings.Split(rest, "/")
			id := segs[0]
			f.mu.Lock()
			defer f.mu.Unlock()
			d, ok := f.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			switch r.Method {
			case http.MethodPost:
				var req struct {
					Children []json.RawMessage `json:"children"`
					Index    *int              `json:"index"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				// Record the effect FIRST, then optionally drop the reply —
				// the exact partial-success shape the notion double pins
				// (effect applied remotely, response lost mid-flight).
				f.appendCalls++
				f.appendSizes = append(f.appendSizes, len(req.Children))
				d.children = append(d.children, req.Children...)
				d.revision++
				if f.dropNextAppend {
					f.dropNextAppend = false
					panic(http.ErrAbortHandler)
				}
				write(w, fmt.Sprintf(`{"code":0,"data":{"children":[],"document_revision_id":%d}}`, d.revision))
			case http.MethodGet:
				// Paginated list; the fake honors page_size=1 so the
				// pagination-follow test can prove the adapter walks it.
				q := r.URL.Query()
				pageToken := q.Get("page_token")
				start := 0
				if pageToken != "" {
					_, _ = fmt.Sscanf(pageToken, "%d", &start)
				}
				end := start + 1
				hasMore := end < len(d.children)
				items := d.children
				if start < len(d.children) {
					if end > len(d.children) {
						end = len(d.children)
					}
					items = d.children[start:end]
				} else {
					items = nil
				}
				next := ""
				if hasMore {
					next = fmt.Sprintf("%d", end)
				}
				write(w, fmt.Sprintf(`{"code":0,"data":{"items":[%s],"page_token":%q,"has_more":%t}}`,
					strings.Join(rawList(items), ","), next, hasMore))
			default:
				write(w, `{"code":99991661,"msg":"method"}`)
			}
		default:
			write(w, `{"code":99991661,"msg":"unknown path"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func rawList(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = string(it)
	}
	return out
}

// docAdapter builds a FeishuDocxAdapter over the fake with an in-memory
// progress map, mirroring the publicationProgress wiring the bridge does
// in production.
func docAdapter(t *testing.T, fake *fakeFeishuDocx, caps []string, batch int) (*FeishuDocxAdapter, map[string]FeishuDocProgress) {
	t.Helper()
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	progress := map[string]FeishuDocProgress{}
	return &FeishuDocxAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return fake.token, nil },
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) {
			return caps, nil
		},
		LoadProgress: func(a Action) FeishuDocProgress { return progress[a.ID] },
		SaveProgress: func(a Action, p FeishuDocProgress) error { progress[a.ID] = p; return nil },
		MaxBatch:     batch,
	}, progress
}

func feishuCreateAction(id string, folder string, n int) (Action, []string) {
	blocks, _ := FeishuTextBlocks(strings.TrimSuffix(strings.Repeat("段落。\n\n", n), "\n\n"))
	args, _ := json.Marshal(map[string]any{"parent_folder": folder, "title": "Report", "blocks": blocks})
	want := make([]string, 0, n)
	for i := 0; i < n; i++ {
		want = append(want, fmt.Sprintf("段落%d。", i+1))
	}
	return Action{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: folder, Risk: RiskWrite, Args: args}, want
}

func TestFeishuDocxAdapterCreateFullLoop(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, progress := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, want := feishuCreateAction("act-1", "fld-1", 3)

	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)
	require.NotEmpty(t, out.ExternalID)

	// One create + two batch appends (2+1): the documented cap is never
	// exceeded, and the progress record ends complete.
	fake.mu.Lock()
	require.Equal(t, 1, fake.createCalls)
	require.Equal(t, []int{2, 1}, fake.appendSizes)
	doc := fake.docs[out.ExternalID]
	fake.mu.Unlock()
	require.Len(t, doc.children, 3)

	p := progress[a.ID]
	require.Equal(t, out.ExternalID, p.DocumentID)
	require.Equal(t, 3, p.BlocksDone)

	// The committed children carry the approved content.
	got, err := feishuDocBlockContents([]byte("[" + strings.Join(rawList(doc.children), ",") + "]"))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestFeishuDocxAdapterCreateResumesFromCheckpoint(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, progress := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, _ := feishuCreateAction("act-2", "fld-1", 5)

	// First attempt: create + batch1(2) + batch2(2) succeed with
	// checkpoints persisted; batch3(1) lands REMOTELY but loses its
	// reply → the honest state is unknown and the checkpoint stops at 4.
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()
	out, err := ad.Execute(context.Background(), a)
	require.Error(t, err)
	require.Equal(t, ActionUnknown, out.State, "a lost append reply must park unknown")
	require.Equal(t, 4, progress[a.ID].BlocksDone, "checkpoint holds the confirmed batches")
	require.Equal(t, out.ExternalID, progress[a.ID].DocumentID)

	// Recovery on the SAME action: the persisted document id forces the
	// SAME document; confirmed batches (blocks 1-4) are NEVER re-sent;
	// only the unconfirmed tail re-dispatches (its remote effect already
	// landed once — the duplicated tail is the honest #48-aligned
	// semantics that Query's contiguous-run check still settles).
	out2, err2 := ad.Execute(context.Background(), a)
	require.NoError(t, err2)
	require.Equal(t, ActionSucceeded, out2.State)
	require.Equal(t, out.ExternalID, out2.ExternalID, "resume lands on the SAME document")

	fake.mu.Lock()
	require.Equal(t, 1, fake.createCalls, "NO second create after a persisted document id")
	require.Equal(t, []int{2, 2, 1, 1}, fake.appendSizes, "confirmed batches re-dispatch ZERO; only the tail re-sends")
	doc := fake.docs[out.ExternalID]
	fake.mu.Unlock()
	require.Len(t, doc.children, 6, "5 approved + 1 duplicated tail (recorded honestly)")
}

func TestFeishuDocxAdapterUpdateRevisionConflictRefusesBeforeWrite(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	// A remote document at revision 1.
	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-x"] = &fakeFeishuDoc{id: "doc-x", revision: 1}
	fake.mu.Unlock()
	blocks, _ := FeishuTextBlocks("追加内容")
	args, _ := json.Marshal(map[string]any{"document_id": "doc-x", "expected_revision": "1", "title": "T", "blocks": blocks})
	a := Action{ID: "act-3", TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: "doc-x", Risk: RiskWrite, Args: args}

	// A collaborator moves the revision between approval and execute.
	fake.mu.Lock()
	fake.docs["doc-x"].revision = 5
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), a)
	require.ErrorIs(t, err, ErrFeishuPublishRevisionConflict)
	require.Equal(t, ActionFailed, out.State)

	fake.mu.Lock()
	require.Equal(t, 0, fake.appendCalls, "AC1: conflict leaves ZERO writes")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterUpdateHappyPathAppends(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-y"] = &fakeFeishuDoc{id: "doc-y", revision: 1}
	fake.mu.Unlock()
	blocks, _ := FeishuTextBlocks("追加内容")
	args, _ := json.Marshal(map[string]any{"document_id": "doc-y", "expected_revision": "1", "title": "T", "blocks": blocks})
	a := Action{ID: "act-4", TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: "doc-y", Risk: RiskWrite, Args: args}

	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)
	require.Equal(t, "doc-y", out.ExternalID)

	// The output evidence carries the NEW revision the publish produced.
	rcpt, rerr := ParseFeishuDocReceipt(out.Output)
	require.NoError(t, rerr)
	require.Equal(t, "doc-y", rcpt.ExternalID)
	require.NotEqual(t, "1", rcpt.ExternalVersion, "the receipt version must be the post-write revision")
}

func TestFeishuDocxAdapterReadOnlyCapabilityRefused(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	// AC2: the connection's reviewed scopes carry only a read capability.
	ad, _ := docAdapter(t, fake, []string{"read_docx"}, 0)
	a, _ := feishuCreateAction("act-5", "fld-1", 1)

	out, err := ad.Execute(context.Background(), a)
	require.ErrorIs(t, err, ErrFeishuPublishMissingCapability)
	require.Equal(t, ActionFailed, out.State)

	fake.mu.Lock()
	require.Equal(t, 0, fake.createCalls, "refusal happens BEFORE any network write")
	require.Equal(t, 0, fake.appendCalls)
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryReconcilesByContent(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, _ := feishuCreateAction("act-6", "fld-1", 3)

	// The create and both batches land REMOTELY; the final batch's reply
	// is lost → unknown, checkpoint=2.
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()
	out, _ := ad.Execute(context.Background(), a)
	require.Equal(t, ActionUnknown, out.State)

	// The full approved content IS remote (all 3 batches recorded).
	// Query's content check settles success WITHOUT any re-send — even
	// though the provider children carry server-side fields the snapshot
	// bytes never had (semantic comparison, Review Focus 5).
	fake.mu.Lock()
	appendsBefore := fake.appendCalls
	fake.mu.Unlock()

	q2, err2 := ad.Query(context.Background(), a)
	require.NoError(t, err2)
	require.Equal(t, ActionSucceeded, q2.State)
	require.Equal(t, out.ExternalID, q2.ExternalID)
	rcpt, rerr := ParseFeishuDocReceipt(q2.Output)
	require.NoError(t, rerr)
	require.Equal(t, out.ExternalID, rcpt.ExternalID)

	fake.mu.Lock()
	require.Equal(t, appendsBefore, fake.appendCalls, "reconcile must not re-send")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryWithoutDocumentIDStaysUnknown(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	a, _ := feishuCreateAction("act-7", "fld-1", 1)
	// No progress: the create itself is unobservable. Feishu offers no
	// reliable title search — the honest outcome is unknown, never a
	// fabricated id from a title match.
	q, err := ad.Query(context.Background(), a)
	require.Error(t, err)
	require.Equal(t, ActionUnknown, q.State)
	fake.mu.Lock()
	require.Equal(t, 0, fake.createCalls, "Query never re-creates")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryFollowsPagination(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	a, _ := feishuCreateAction("act-8", "fld-1", 3)
	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)

	// The fake lists ONE child per page; the adapter must walk page_token
	// to see all three before claiming success.
	q, err := ad.Query(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, q.State)
}

func TestReadFeishuDocumentVersionPreRead(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := HTTPPolicy{Scheme: "http", Host: host, Port: port, Methods: []string{"GET"},
		PathPrefix: "/open-apis/docx/", AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second}
	tok := func(ctx context.Context) (string, error) { return fake.token, nil }

	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-v"] = &fakeFeishuDoc{id: "doc-v", revision: 9}
	fake.mu.Unlock()

	v, err := ReadFeishuDocumentVersion(context.Background(), pol, tok, "doc-v")
	require.NoError(t, err)
	require.Equal(t, "doc-v", v.DocumentID)
	require.Equal(t, "9", v.RevisionID)
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapter|TestReadFeishuDocumentVersion' -count=1
```

Expected: FAIL，`undefined: FeishuDocxAdapter` 与 `undefined: ReadFeishuDocumentVersion`。

- [ ] **Step 3: 最小实现（追加到 `feishu_docx.go`）**

```go
// FeishuDocxAdapter executes ONE approved docx publication against the
// FE-PUB-01 reviewed contract, routed through the A04 outbound policy.
// Outcome semantics mirror the Notion family exactly (the service layer
// above cannot tell the providers apart — AC1):
//
//   - pre-read failures (update branch) are definitive FAILED — a GET
//     can never have produced the write;
//   - every write-step transport failure / 5xx / unparseable reply is
//     ErrFeishuPublishOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable document read + children
//     read; a fresh create is never the recovery for an unknown create.
type FeishuDocxAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Token returns the connection's Feishu credential (Bearer).
	Token func(ctx context.Context) (string, error)
	// ConnectionCapabilities reports the connection's reviewed scopes;
	// the write_docx capability is required (AC2).
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before outbound calls.
	Recheck func(ctx context.Context, a Action) error
	// LoadProgress / SaveProgress persist the FE-03 recovery record.
	LoadProgress func(a Action) FeishuDocProgress
	SaveProgress func(a Action, p FeishuDocProgress) error
	// MaxBatch caps children per append request (test hook; 0 = the
	// documented FeishuDocAppendBatchLimit, never above it).
	MaxBatch int
}

var _ Adapter = (*FeishuDocxAdapter)(nil)

func (m *FeishuDocxAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrFeishuPublishNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no token source", ErrFeishuPublishNotConfigured)
	}
	return nil
}

func (m *FeishuDocxAdapter) requireWriteCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrFeishuPublishMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFeishuPublishMissingCapability, err)
	}
	for _, c := range caps {
		if c == FeishuCapabilityWriteDocx {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrFeishuPublishMissingCapability, FeishuCapabilityWriteDocx)
}

func (m *FeishuDocxAdapter) loadProgress(a Action) FeishuDocProgress {
	if m.LoadProgress == nil {
		return FeishuDocProgress{}
	}
	return m.LoadProgress(a)
}

func (m *FeishuDocxAdapter) storeProgress(a Action, p FeishuDocProgress) error {
	if m.SaveProgress == nil {
		return nil
	}
	if err := m.SaveProgress(a, p); err != nil {
		return fmt.Errorf("%w: persisting progress: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	return nil
}

func (m *FeishuDocxAdapter) batchSize() int {
	if m.MaxBatch > 0 && m.MaxBatch < FeishuDocAppendBatchLimit {
		return m.MaxBatch
	}
	return FeishuDocAppendBatchLimit
}

// Execute routes on the approved snapshot's shape (AC1 lives BELOW this
// point only).
func (m *FeishuDocxAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("approval revoked: %v", err)
		}
	}
	if IsFeishuDocUpdateArgs(a.Args) {
		return m.executeUpdate(ctx, a)
	}
	return m.executeCreate(ctx, a)
}

func (m *FeishuDocxAdapter) executeCreate(ctx context.Context, a Action) (ActionResult, error) {
	snap, err := ParseFeishuDocCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	docID := progress.DocumentID
	done := progress.BlocksDone
	var output json.RawMessage
	if docID == "" {
		body, _ := json.Marshal(map[string]string{"folder_token": snap.ParentFolder})
		raw, cerr := m.do(ctx, http.MethodPost, FeishuDocumentCreatePath, body)
		if cerr != nil {
			state := ActionFailed
			if errors.Is(cerr, ErrFeishuPublishOutcomeUnknown) {
				// Unknown create: NO document id is persisted, NO second
				// create may follow; reconciliation happens via Query.
				state = ActionUnknown
			}
			return ActionResult{State: state}, cerr
		}
		v, verr := ParseFeishuDocumentVersion(raw)
		if verr != nil {
			return ActionResult{State: ActionUnknown}, verr
		}
		docID = v.DocumentID
		done = 0
		output = json.RawMessage(raw)
		// The REAL document id is persisted the moment the provider
		// confirms the create — BEFORE any content step.
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: 0}); serr != nil {
			return ActionResult{State: ActionUnknown}, serr
		}
	}
	for done < len(snap.Blocks) {
		end := done + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if aerr := m.appendChildren(ctx, docID, snap.Blocks[done:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrFeishuPublishOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: docID}, aerr
		}
		done = end
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: done}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: docID}, serr
		}
	}
	return ActionResult{State: ActionSucceeded, ExternalID: docID, Output: output}, nil
}

func (m *FeishuDocxAdapter) executeUpdate(ctx context.Context, a Action) (ActionResult, error) {
	snap, err := ParseFeishuDocUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current revision FIRST — an unreadable
	// pre-read is a definitive failure with zero writes.
	ver, gerr := ReadFeishuDocumentVersion(ctx, m.Policy, m.Token, snap.DocumentID)
	if gerr != nil {
		return ActionResult{State: ActionFailed}, gerr
	}
	if cerr := DetectFeishuRevisionConflict(snap.ExpectedRevision, ver.RevisionID); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	docID := snap.DocumentID
	progress := m.loadProgress(a)
	blocksDone := 0
	if progress.DocumentID == docID && progress.BlocksDone >= 0 && progress.BlocksDone <= len(snap.Blocks) {
		blocksDone = progress.BlocksDone
	}
	for blocksDone < len(snap.Blocks) {
		end := blocksDone + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if aerr := m.appendChildren(ctx, docID, snap.Blocks[blocksDone:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrFeishuPublishOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: docID}, aerr
		}
		blocksDone = end
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: blocksDone}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: docID}, serr
		}
	}
	// Reliable read-back: the payload carrying the document id + the
	// revision THIS publish produced is the receipt basis. A lost
	// read-back parks unknown — the writes already landed.
	raw, ferr := m.do(ctx, http.MethodGet, fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(docID)), nil)
	if ferr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: docID}, ferr
	}
	final, perr := ParseFeishuDocumentVersion(raw)
	if perr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: docID}, perr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: docID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for an unknown outcome.
// Without a persisted document id it stays honestly unknown (Feishu has
// no reliable create-confirmation search; a title match is never proof).
// With one, it reconciles via the reliable document read + the FULL
// paginated children read, comparing the text_run CONTENT sequences —
// provider blocks carry server-side fields the snapshot bytes never do.
func (m *FeishuDocxAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	if progress.DocumentID == "" {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("feishu_query_unverifiable: no persisted document id")
	}
	var want []string
	if IsFeishuDocUpdateArgs(a.Args) {
		snap, err := ParseFeishuDocUpdateSnapshot(a.Args)
		if err != nil {
			return ActionResult{State: ActionUnknown}, err
		}
		for _, b := range snap.Blocks {
			contents, err := feishuDocBlockContents([]byte("[" + string(b) + "]"))
			if err != nil {
				return ActionResult{State: ActionUnknown}, err
			}
			want = append(want, contents...)
		}
	} else {
		snap, err := ParseFeishuDocCreateSnapshot(a.Args)
		if err != nil {
			return ActionResult{State: ActionUnknown}, err
		}
		for _, b := range snap.Blocks {
			contents, err := feishuDocBlockContents([]byte("[" + string(b) + "]"))
			if err != nil {
				return ActionResult{State: ActionUnknown}, err
			}
			want = append(want, contents...)
		}
	}
	// The document must still exist.
	if _, gerr := ReadFeishuDocumentVersion(ctx, m.Policy, m.Token, progress.DocumentID); gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	kids, kerr := m.readAllChildren(ctx, progress.DocumentID)
	if kerr != nil {
		return ActionResult{State: ActionUnknown}, kerr
	}
	got, gerr := feishuDocBlockContents(kids)
	if gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	if !containsPrefix(got, want) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("feishu_query_unverifiable: %d of %d approved paragraphs present in order", len(got), len(want))
	}
	raw, rerr := m.do(ctx, http.MethodGet, fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(progress.DocumentID)), nil)
	if rerr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: progress.DocumentID}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: progress.DocumentID, Output: json.RawMessage(raw)}, nil
}

// containsPrefix reports whether want appears in got as one contiguous
// run starting at any offset (external collaborators may have appended
// their own paragraphs before or after ours) — the semantic counterpart
// of notionBlocksContained (notion_update.go:375-397).
func containsPrefix(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	if len(got) < len(want) {
		return false
	}
	for start := 0; start+len(want) <= len(got); start++ {
		match := true
		for i := range want {
			if got[start+i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func urlPathEscape(s string) string { return url.PathEscape(s) }

// appendChildren performs one recoverable append step of the REMAINING
// blocks (index -1 = append at the document end).
func (m *FeishuDocxAdapter) appendChildren(ctx context.Context, docID string, blocks []json.RawMessage) error {
	body, err := json.Marshal(map[string]any{"children": blocks, "index": -1})
	if err != nil {
		return err
	}
	u := fmt.Sprintf(FeishuDocumentChildrenFormat, urlPathEscape(docID), urlPathEscape(docID))
	raw, err := m.do(ctx, http.MethodPost, u, body)
	if err != nil {
		return err
	}
	var env feishuDocEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if env.Code != 0 {
		return fmt.Errorf("feishu_provider_error: code=%d msg=%s", env.Code, env.Msg)
	}
	return nil
}

// readAllChildren walks the paginated children list to the end — a
// reconciliation that reads only the first page would miscount committed
// blocks (Review Focus 3).
func (m *FeishuDocxAdapter) readAllChildren(ctx context.Context, docID string) ([]byte, error) {
	pageToken := ""
	var all []json.RawMessage
	for {
		u := fmt.Sprintf(FeishuDocumentChildrenFormat, urlPathEscape(docID), urlPathEscape(docID))
		if pageToken != "" {
			u += "?page_token=" + urlPathEscape(pageToken)
		}
		raw, err := m.do(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Code int `json:"code"`
			Data struct {
				Items     []json.RawMessage `json:"items"`
				PageToken string            `json:"page_token"`
				HasMore   bool              `json:"has_more"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
		}
		if page.Code != 0 {
			return nil, fmt.Errorf("feishu_provider_error: code=%d", page.Code)
		}
		all = append(all, page.Data.Items...)
		if !page.Data.HasMore || page.Data.PageToken == "" {
			break
		}
		pageToken = page.Data.PageToken
	}
	return json.Marshal(all)
}

// do performs ONE policy-validated request through the A04 client — the
// same request/redirect re-validation as every adapter in this package.
// Transport failures wrap ErrFeishuPublishOutcomeUnknown; provider 4xx
// is a definitive provider error; 5xx is unknown.
func (m *FeishuDocxAdapter) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	u := m.targetURL(path)
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return nil, err // policy denial: the request never leaves
	}
	tok, err := m.Token(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := m.Policy.NewClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status=%d", ErrFeishuPublishOutcomeUnknown, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: status=404", ErrFeishuPublishNotFound)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("feishu_provider_error: status=%d body=%s", resp.StatusCode, truncateForLog(raw))
	}
	return raw, nil
}

func (m *FeishuDocxAdapter) targetURL(path string) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	return &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
}

func truncateForLog(raw []byte) string {
	s := string(raw)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// ReadFeishuDocumentVersion performs a one-off version read through the
// given reviewed policy and token source — the plan-formation pre-read
// shared by the publish seam (the counterpart of ReadNotionPageVersion,
// notion_update.go:507+). It performs no write.
func ReadFeishuDocumentVersion(ctx context.Context, policy HTTPPolicy, token func(context.Context) (string, error), documentID string) (FeishuDocVersion, error) {
	probe := &FeishuDocxAdapter{Policy: policy, Token: token}
	raw, err := probe.do(ctx, http.MethodGet, fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(documentID)), nil)
	if err != nil {
		return FeishuDocVersion{}, err
	}
	return ParseFeishuDocumentVersion(raw)
}
```

同时更新 `feishu_docx.go` 的 import 为：`bytes`、`context`、`encoding/json`、`errors`、`fmt`、`io`、`net`、`net/http`、`net/url`、`strconv`、`strings`（`context` 在 Task 1 部分尚无引用时编译器会报 unused——Task 1 的实现不含 `ReadFeishuDocumentVersion`，因此在 Task 1 阶段 import 集为 `bytes`/`encoding/json`/`errors`/`fmt`/`strconv`/`strings`，Task 2 追加实现时一次性补齐 `context`/`io`/`net`/`net/http`/`net/url`）。

- [ ] **Step 4: 运行测试通过**

```bash
go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapter|TestReadFeishuDocumentVersion' -count=1 -v 2>&1 | tail -20
```

Expected: PASS（11 个 Adapter 测试全绿）。若 `TestFeishuDocxAdapterQueryReconcilesByContent` 失败，先检查 fake 的 children GET 是否按 `page_size` 语义分页——本计划 fake 固定每页 1 条，该测试同时覆盖 Review Focus 3。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/feishu_docx.go internal/modules/appconnector/feishu_docx_test.go
git commit -m "feat(appconnector): 飞书 docx Adapter——多步写入/进度恢复/语义核对（FE-03）"
```

---

### Task 3: publish 包 provider 泛化（ProviderProfile）+ FeishuBridge

**Files:**
- Create: `internal/modules/appconnector/publish/provider.go`
- Create: `internal/modules/appconnector/publish/feishu.go`
- Modify: `internal/modules/appconnector/publish/plan.go`（7 处点状替换 + 1 个新构造函数）
- Modify: `internal/modules/appconnector/publish/dispatcher.go`（+1 字段 +1 行赋值）
- Test: `internal/modules/appconnector/publish/provider_test.go`、`internal/modules/appconnector/publish/feishu_test.go`

**Interfaces:**
- Consumes: Task 1/2 全部产出（`appconn.FeishuTextBlocks`、`appconn.FeishuDocxAdapter`、`appconn.IsFeishuDocUpdateArgs`、`appconn.ReadFeishuDocumentVersion`、`appconn.ParseFeishuDocReceipt`、`appconn.FeishuCapabilityWriteDocx`、`appconn.ErrFeishuPublishRevisionConflict`）；#48 既有 `NotionBridge` 三端口（`NotionScopeSource`/`NotionPolicyProvider`/`NotionTokenSource`，`dispatcher.go:39-55`）、`PublicationSource`（`dispatcher.go:59-62`）、`PublishVersionConflictResult`（`dispatcher.go:20`）。
- Produces（Task 4/5 与后续 #50 消费，签名逐字）:
  - `type ProviderProfile struct { AppID, Provider, ActionVersion, ConflictResultPrefix string; BlocksOf func(string) ([]json.RawMessage, error); CreateArgs func(parent, title string, blocks []json.RawMessage) ([]byte, error); UpdateArgs func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error); ReadRemoteVersion func(ctx context.Context, connectionID, destination string) (string, error); ParseReceipt func(providerResult string) (externalID, externalVersion string, err error) }`
  - `func NotionProfile(remote NotionRemoteReader) ProviderProfile`
  - `func FeishuProfile(read NotionRemoteReader) ProviderProfile`
  - `func NewProviderPublishService(actions *appconnectorsvc.ActionService, store appconnectorsvc.ActionStoreSource, pubs *repoappconn.PublicationStore, artifacts ArtifactVersionReader, content ArtifactContentReader, scopes NotionScopeSource, profile ProviderProfile) *NotionPublishService`
  - `type FeishuBridge struct`；`func NewFeishuBridge(scopes NotionScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *FeishuBridge`（实现 `appconnectorsvc.ActionDispatcher` + `appconnectorsvc.UnknownResolver` + `NotionRemoteReader`）
  - `const FeishuVersionConflictResult = "feishu_docx_revision_conflict"`
  - `func NewConstantFeishuPolicyProvider() NotionPolicyProvider`
  - `NotionConnectionScope` 增加 `Scopes []string` 字段（原始审核 scopes 透传；`InsertCapability` 行为不变）
  - 服务类型沿用历史名 `NotionPublishService`（provider 中立化后不改名，避免翻动 #48 的调用面；provider 绑定点在 `ProviderProfile`）

**共享文件修改的完整清单（并行集成预警）：** `plan.go` 恰好 8 处点状替换（下文 Step 3 逐字给出：AppID 检查、BlocksOf、CreatePublication 的 Provider 行、AC1 预读判定、args 构造、ActionVersion、回执解析+settle、Conflict 判定）+ struct 字段/构造函数；`dispatcher.go` 恰好 +1 字段 +1 行赋值；无其它对既有文件的改写。

- [ ] **Step 1: 写失败测试（provider_test.go + feishu_test.go）**

创建 `internal/modules/appconnector/publish/provider_test.go`——**AC1「飞书差异只存在于 Adapter」的正面证据**：同一组服务层断言同时驱动 Notion 与飞书两个 profile，服务层代码路径完全共享：

```go
package publish

// AC1 evidence: the publish service layer is provider-neutral. The SAME
// table of assertions drives BOTH the notion and the feishu profile
// through plan formation, approval, execution and receipt settlement;
// the only per-provider input is the ProviderProfile (whose blocks/snapshot
// functions live in the adapter domain). If a future edit re-introduces
// a provider branch into plan.go, this table stops being symmetric and
// the test fails.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/stretchr/testify/require"
)

type feishuPlanEnv struct {
	svc   *NotionPublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
}

func newFeishuPlanEnv(t *testing.T, remote *fakeRemote, scope NotionConnectionScope) *feishuPlanEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	bridge := NewFeishuBridge(
		&fakeScopes{scope: scope},
		&fakePolicies{pol: appconn.HTTPPolicy{}}, // unused: dispatch is refused at caps/scope before dialing in these cases
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	actions := newPublishActionsForTest(t, store, bridge)
	artifacts := &fakeArtifacts{version: planArtifactVersion()}
	svc := NewProviderPublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("飞书第一段。\n\n飞书第二段。")},
		&fakeScopes{scope: scope}, FeishuProfile(remote))
	return &feishuPlanEnv{svc: svc, pubs: pubs, store: store}
}

func planArtifactVersion() (v planArtifactVersionT) { return }
```

等等——`planArtifactVersion` 返回类型未定义，直接内联。删去上一行，`newFeishuPlanEnv` 中 `artifacts` 写为：

```go
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
	}}
```

（与 `plan_test.go:95-99` 的既有字面量同形；文件顶部补 import `"github.com/Tencent/WeKnora/internal/application/repository"`。）

`fakeScopes`/`fakePolicies`/`fakeTokens` 定义于 `dispatcher_test.go`（本计划实施时先读该文件确认字段名——撰写时已核实 `plan_test.go:86-93` 的用法：`&fakeScopes{scope: NotionConnectionScope{…}}`、`&fakePolicies{pol: pol}`、`&fakeTokens{tok: "…"}`）。`newPublishActionsForTest` 是下述测试共享的小助手，放在 `provider_test.go` 顶部：

```go
func newPublishActionsForTest(t *testing.T, store *repoappconn.ActionStore, bridge appconnectorsvc.ActionDispatcher) *appconnectorsvc.ActionService {
	t.Helper()
	return appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
}
```

（文件 import 追加 `appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"`。）

双 profile 表驱动测试主体：

```go
// TestPublishServiceProfilesAreSymmetric drives BOTH profiles through
// the identical service-layer assertion set.
func TestPublishServiceProfilesAreSymmetric(t *testing.T) {
	type profileCase struct {
		name            string
		appID           string
		createDest      string // PublishPlanInput.ParentPageID
		expectedMode    string
		expectedDest    string
		expectedBaseline string // notion: the parent page version; feishu: empty (folder has no revision)
		expectSnapshot  func(t *testing.T, argsJSON string)
	}
	notionCase := profileCase{
		name: "notion", appID: "notion", createDest: "parent-1",
		expectedMode: "create", expectedDest: "parent-1", expectedBaseline: "v-1",
		expectSnapshot: func(t *testing.T, argsJSON string) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(argsJSON), &raw))
			require.Len(t, raw, 3, "notion create snapshot: parent, title, blocks")
			require.Contains(t, raw, "parent")
			var parent string
			require.NoError(t, json.Unmarshal(raw["parent"], &parent))
			require.Equal(t, "parent-1", parent)
		},
	}
	feishuCase := profileCase{
		name: "feishu", appID: "feishu", createDest: "fld-1",
		expectedMode: "create", expectedDest: "fld-1", expectedBaseline: "",
		expectSnapshot: func(t *testing.T, argsJSON string) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(argsJSON), &raw))
			require.Len(t, raw, 3, "feishu create snapshot: parent_folder, title, blocks")
			require.Contains(t, raw, "parent_folder")
			var folder string
			require.NoError(t, json.Unmarshal(raw["parent_folder"], &folder))
			require.Equal(t, "fld-1", folder)
		},
	}
	for _, tc := range []profileCase{notionCase, feishuCase} {
		t.Run(tc.name, func(t *testing.T) {
			scope := NotionConnectionScope{
				AppID: tc.appID, ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
				ApprovedParents: []string{tc.createDest},
				Scopes:          []string{appconn.FeishuCapabilityWriteDocx},
			}
			if tc.appID == "notion" {
				scope.InsertCapability = true
			}
			// The feishu case leaves the remote map EMPTY: the folder
			// destination has no revision, so the pre-read yields an
			// empty baseline (the shared layer accepts it for create).
			remoteByApp := map[string]map[string]string{
				"notion": {tc.createDest: "v-1"},
				"feishu": {},
			}
			remote := &fakeRemote{versions: remoteByApp[tc.appID]}
			var env *feishuPlanEnv
			if tc.appID == "notion" {
				// The notion profile rides the SAME symmetric assertions
				// via the shared constructor path.
				db := openPlanDB(t)
				store := repoappconn.NewActionStore(db)
				pubs := repoappconn.NewPublicationStore(db)
				actions := newPublishActionsForTest(t, store, NewNotionBridge(&fakeScopes{scope: scope}, &fakePolicies{pol: appconn.HTTPPolicy{}}, &fakeTokens{tok: "secret_test_token"}, pubs))
				svc := NewNotionPublishService(actions, store, pubs, &fakeArtifacts{version: repository.ArtifactVersion{
					TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
					Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					ObjectKey: "k", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
				}}, &fakeContent{data: []byte("一。\n\n二。")}, remote, &fakeScopes{scope: scope})
				env = &feishuPlanEnv{svc: svc, pubs: pubs, store: store}
			} else {
				env = newFeishuPlanEnv(t, remote, scope)
			}

			input := PublishPlanInput{TenantID: 7, ActorID: "u1", ConnectionID: "conn-x",
				SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "T", ParentPageID: tc.createDest}
			view, err := env.svc.FormPlan(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedMode, view.Mode)
			require.Equal(t, tc.expectedDest, view.Destination)
			if tc.expectedBaseline == "" {
				require.Empty(t, view.ExpectedExternalVersion,
					"AC1: a feishu folder destination records an EMPTY baseline (typed not-found → create proceeds)")
			} else {
				require.Equal(t, tc.expectedBaseline, view.ExpectedExternalVersion, "AC1 baseline pre-read rides the shared path")
			}

			// The recorded publication row is provider-branded by the profile.
			row, err := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
			require.NoError(t, err)
			require.Equal(t, tc.appID, row.Provider)

			// The prepared action's snapshot shape comes from the profile.
			snap, err := env.store.FindAction(context.Background(), view.ActionID)
			require.NoError(t, err)
			tc.expectSnapshot(t, snap.ArgsSnapshot)

			// Execute requires approval first; after approval the shared
			// execute path settles the receipt from the action row. The
			// empty test policy makes the adapter refuse to dial — the
			// SAME terminal transition for BOTH providers (the symmetric
			// settle assertion).
			require.NoError(t, env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest))
			outcome, execErr := env.svc.Execute(context.Background(), 7, view.ActionID)
			require.Error(t, execErr) // empty test policy: adapter refuses to dial — same for BOTH providers
			require.Equal(t, repoappconn.PublicationFailed, outcome.Receipt.State)
		})
	}
}
```

创建 `internal/modules/appconnector/publish/feishu_test.go`：

```go
package publish

// FeishuBridge tests: snapshot-shape routing, the non-feishu connection
// refusal, the AC2 write-capability gate (schema_json scopes), the plan
// pre-read, and the pinned production policy.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

func feishuSnapshot(id string) appconnectorsvc.ActionSnapshot {
	args, _ := json.Marshal(map[string]any{"parent_folder": "fld-1", "title": "T", "blocks": []any{}})
	return appconnectorsvc.ActionSnapshot{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "conn-f",
		Version: "feishu/v1", Target: "fld-1", Risk: "write", AuthVersion: 1, Args: args}
}

func newFeishuBridge(scope NotionConnectionScope, pubs *repoappconn.PublicationStore) *FeishuBridge {
	return NewFeishuBridge(&fakeScopes{scope: scope}, &fakePolicies{pol: appconn.HTTPPolicy{}}, &fakeTokens{tok: "secret_test_token"}, pubs)
}

func TestFeishuBridgeRefusesNonFeishuConnection(t *testing.T) {
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "notion", AuthVersion: 1,
		Scopes: []string{appconn.FeishuCapabilityWriteDocx}}, repoappconn.NewPublicationStore(db))
	_, err := bridge.Dispatch(context.Background(), feishuSnapshot("a1"), "k")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted, "a provable pre-send wiring gap")
}

func TestFeishuBridgeRefusesReadOnlyScope(t *testing.T) {
	// AC2: a connection whose reviewed scopes carry only read/sync
	// capabilities never satisfies the write gate.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1,
		Scopes: []string{"read_docx", "sync_content"}}, repoappconn.NewPublicationStore(db))
	out, err := bridge.Dispatch(context.Background(), feishuSnapshot("a2"), "k")
	require.NoError(t, err, "capability refusal is a definitive failed OUTCOME, not a wiring gap")
	require.Equal(t, appconn.ActionFailed, out.Status)
	require.Contains(t, out.ProviderResult, appconn.FeishuCapabilityWriteDocx)
}

func TestFeishuBridgeRoutesUpdateSnapshotToAdapter(t *testing.T) {
	// Routing itself is adapter-domain (IsFeishuDocUpdateArgs); the bridge
	// only proves both shapes pass the scope/caps gate without erroring
	// at the wiring layer. With an empty test policy the adapter fails
	// closed BEFORE dialing — a deterministic, networkless outcome.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1,
		Scopes: []string{appconn.FeishuCapabilityWriteDocx}}, repoappconn.NewPublicationStore(db))
	updateArgs, _ := json.Marshal(map[string]any{"document_id": "doc-1", "expected_revision": "1", "title": "T", "blocks": []any{}})
	snap := feishuSnapshot("a3")
	snap.Args = updateArgs
	out, err := bridge.Dispatch(context.Background(), snap, "k")
	require.NoError(t, err)
	require.Equal(t, appconn.ActionFailed, out.Status, "empty test policy → configError, identical to the notion bridge's loopback-hook pattern")
}

func TestFeishuBridgeReadPageVersionDelegatesToAdapter(t *testing.T) {
	// The pre-read rides ReadFeishuDocumentVersion; with no reviewed
	// policy (empty) it fails closed — pinned here so the bridge cannot
	// silently dial with an unpinned contract.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1}, repoappconn.NewPublicationStore(db))
	_, err := bridge.ReadPageVersion(context.Background(), "conn-f", "doc-1")
	require.Error(t, err)
}

func TestConstantFeishuPolicyProviderPinsContract(t *testing.T) {
	pol, err := NewConstantFeishuPolicyProvider().PolicyFor(context.Background(), "conn-f")
	require.NoError(t, err)
	require.Equal(t, "https", pol.Scheme)
	require.Equal(t, appconn.FeishuAPIHost, pol.Host, "open.feishu.cn — pinned, never model-derived")
	require.Equal(t, []string{http.MethodGet, http.MethodPost}, pol.Methods)
	require.Equal(t, "/open-apis/docx/", pol.PathPrefix)
	require.Equal(t, 30*time.Second, pol.Timeout)
	require.Empty(t, pol.AuthorizedNetworks, "production policy never authorizes private ranges")
}

func TestFeishuProfileSnapshotShapes(t *testing.T) {
	profile := FeishuProfile(&fakeRemote{versions: map[string]string{}})
	blocks, err := appconn.FeishuTextBlocks("a。\n\nb。")
	require.NoError(t, err)

	createArgs, err := profile.CreateArgs("fld-1", "T", blocks)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(createArgs, &raw))
	require.Len(t, raw, 3)
	require.Contains(t, raw, "parent_folder")

	updateArgs, err := profile.UpdateArgs("doc-1", "4", "T", blocks)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(updateArgs, &raw))
	require.Len(t, raw, 4)
	require.Contains(t, raw, "document_id")
	require.Contains(t, raw, "expected_revision")

	require.Equal(t, "feishu", profile.AppID)
	require.Equal(t, "feishu", profile.Provider)
	require.Equal(t, "feishu/v1", profile.ActionVersion)
	require.Equal(t, FeishuVersionConflictResult, profile.ConflictResultPrefix)

	// ParseReceipt projects the adapter-domain receipt onto the shared view.
	id, ver, err := profile.ParseReceipt(`{"document":{"document_id":"doc-9","revision_id":12}}`)
	require.NoError(t, err)
	require.Equal(t, "doc-9", id)
	require.Equal(t, "12", ver)
}

func TestNotionProfileUnchanged(t *testing.T) {
	// #48's profile is a pure extraction: same fields, same snapshot bytes.
	profile := NotionProfile(&fakeRemote{versions: map[string]string{}})
	require.Equal(t, "notion", profile.AppID)
	require.Equal(t, "notion", profile.Provider)
	require.Equal(t, "notion/v1", profile.ActionVersion)
	require.Equal(t, PublishVersionConflictResult, profile.ConflictResultPrefix)
	id, ver, err := profile.ParseReceipt(`{"object":"page","id":"p1","last_edited_time":"t1"}`)
	require.NoError(t, err)
	require.Equal(t, "p1", id)
	require.Equal(t, "t1", ver)
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/modules/appconnector/publish/ -run 'TestPublishServiceProfiles|TestFeishuBridge|TestConstantFeishuPolicy|TestFeishuProfile|TestNotionProfileUnchanged' -count=1
```

Expected: FAIL，`undefined: NewFeishuBridge` / `undefined: FeishuProfile` / `undefined: NewProviderPublishService` / `Scopes` 字段不存在等编译错误。

- [ ] **Step 3: 最小实现——先改共享文件（plan.go 7 处 + dispatcher.go 2 行），再新增文件**

**3a. `plan.go` 修改（逐字 old→new）：**

① struct 加字段 + 构造函数（`plan.go:109-130` 区域）：

```go
// old
type NotionPublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    NotionRemoteReader
	scopes    NotionScopeSource
}
```

```go
// new
type NotionPublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    NotionRemoteReader
	scopes    NotionScopeSource
	// profile carries every provider-specific decision. AC1: the
	// provider difference lives only in the adapter/bridge layer — the
	// service body below has zero provider branches.
	profile ProviderProfile
}
```

`NewNotionPublishService` 函数体替换为：

```go
// NewNotionPublishService builds the publish service over the Notion
// profile (#48 signature unchanged — all existing call sites keep
// compiling).
func NewNotionPublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote NotionRemoteReader,
	scopes NotionScopeSource,
) *NotionPublishService {
	return NewProviderPublishService(actions, store, pubs, artifacts, content, scopes, NotionProfile(remote))
}

// NewProviderPublishService builds the provider-neutral publish service
// for any provider profile (#49 feishu, #50 confluence).
func NewProviderPublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	scopes NotionScopeSource,
	profile ProviderProfile,
) *NotionPublishService {
	return &NotionPublishService{actions: actions, store: store, pubs: pubs, artifacts: artifacts, content: content, scopes: scopes, profile: profile}
}
```

② `FormPlan` 中 4 处（`plan.go:155-156`、`plan.go:192`、`plan.go:203-213`、`plan.go:219`；预读行 `plan.go:199-202` 在 ③ 单独处理）：

```go
// old (plan.go:156)
	if scope.AppID != "notion" {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not notion", ErrPublishInvalidInput, scope.AppID)
	}
// new
	if scope.AppID != s.profile.AppID {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not %s", ErrPublishInvalidInput, scope.AppID, s.profile.AppID)
	}
```

```go
// old (plan.go:192)
	blocks, berr := NotionParagraphBlocks(string(content))
// new
	blocks, berr := s.profile.BlocksOf(string(content))
```

```go
// old (plan.go:203-213)
	var args []byte
	if mode == "update" {
		args, err = json.Marshal(map[string]any{
			"page_id": in.PageID, "expected_version": remoteVersion,
			"title": in.Title, "blocks": blocks,
		})
	} else {
		args, err = json.Marshal(map[string]any{
			"parent": in.ParentPageID, "title": in.Title, "blocks": blocks,
		})
	}
	if err != nil {
		return PublishPlanView{}, err
	}
// new
	var args []byte
	if mode == "update" {
		args, err = s.profile.UpdateArgs(in.PageID, remoteVersion, in.Title, blocks)
	} else {
		args, err = s.profile.CreateArgs(in.ParentPageID, in.Title, blocks)
	}
	if err != nil {
		return PublishPlanView{}, err
	}
```

```go
// old (plan.go:219)
		Version: "notion/v1", Target: destination, Risk: appconn.RiskWrite,
// new
		Version: s.profile.ActionVersion, Target: destination, Risk: appconn.RiskWrite,
```

②（续）`FormPlan` 的 `CreatePublication`（`plan.go:227`）：

```go
// old
		Provider: "notion", Mode: mode, Destination: destination,
// new
		Provider: s.profile.Provider, Mode: mode, Destination: destination,
```

③ **AC1 预读判定的飞书语义适配**（`plan.go:196-202`）。飞书的 create destination 是审核过的 FOLDER——folder 没有 docx revision（get-document 对它 404/99991661，adapter 层已把该形态类型化为 `appconn.ErrFeishuPublishNotFound`，见 Task 1/2）。共享层据此把「baseline 缺失」区分为 create 的合法空值与 update 的拒绝：

```go
// old (plan.go:196-202)
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot; for create it is the
	// destination's recorded baseline.
	remoteVersion, rerr := s.remote.ReadPageVersion(ctx, in.ConnectionID, destination)
	if rerr != nil || remoteVersion == "" {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
// new
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot and MUST succeed.
	// For create the baseline is provider-defined: Notion reads the parent
	// page's version; Feishu's reviewed destination is a FOLDER with no
	// document revision, which the adapter reports as the typed
	// not-found shape and the plan records an EMPTY baseline. Any other
	// create pre-read failure (transport, permission) still fails closed.
	remoteVersion, rerr := s.profile.ReadRemoteVersion(ctx, in.ConnectionID, destination)
	if rerr != nil && !errors.Is(rerr, appconn.ErrFeishuPublishNotFound) {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	if mode == "update" && (rerr != nil || remoteVersion == "") {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
```

安全性论证（写入实现注释的理由）：update 的 destination 恒为「本连接此前 published 的 document id」（`LatestPublishedByDestination` 权威规则）——若该文档已被外部删除，预读 not-found → 本分支拒绝计划，fail closed 保持。create 的空 baseline 无安全后果：create 是唯一写、无冲突可能，真正的门是 folder 审核名单（`ApprovedParents`）与 A03 审批。

④ `project` 的回执解析（`plan.go:307`）：

```go
// old
		rcpt, rerr := appconn.ParseNotionPageReceipt([]byte(row.ProviderResult))
// new
		rcptExternalID, rcptExternalVersion, rerr := s.profile.ParseReceipt(row.ProviderResult)
```

及其后 2 行的 settle 调用相应改为：

```go
// old (plan.go:309)
			if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationPublished, rcpt.ExternalID, rcpt.ExternalVersion, row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
// new
			if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationPublished, rcptExternalID, rcptExternalVersion, row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
```

⑤ `project` 的 Conflict 判定（`plan.go:299`）：

```go
// old
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: strings.HasPrefix(row.ProviderResult, PublishVersionConflictResult)}
// new
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: s.profile.ConflictResultPrefix != "" && strings.HasPrefix(row.ProviderResult, s.profile.ConflictResultPrefix)}
```

**3b. `dispatcher.go` 修改（+1 字段 +1 行）：**

```go
// old (dispatcher.go:27-34)
type NotionConnectionScope struct {
	AppID            string
	ConnectionKind   string
	OwnerID          string
	AuthVersion      int64
	ApprovedParents  []string
	InsertCapability bool
}
// new
type NotionConnectionScope struct {
	AppID            string
	ConnectionKind   string
	OwnerID          string
	AuthVersion      int64
	ApprovedParents  []string
	InsertCapability bool
	// Scopes carries the connection's reviewed scopes verbatim (AC2 data
	// plane): downstream providers (#49 feishu) gate their own write
	// capability on this list; the notion-specific InsertCapability
	// projection above is unchanged for #48 behavior.
	Scopes []string
}
```

```go
// old (dispatcher.go:296-302, scope 构造之后、InsertCapability 循环之前)
	scope := NotionConnectionScope{
		AppID:           inst.AppID,
		ConnectionKind:  conn.Kind,
		OwnerID:         conn.OwnerID,
		AuthVersion:     conn.AuthVersion,
		ApprovedParents: parsed.ApprovedParents,
	}
// new（在构造字面量中加一行）
	scope := NotionConnectionScope{
		AppID:           inst.AppID,
		ConnectionKind:  conn.Kind,
		OwnerID:         conn.OwnerID,
		AuthVersion:     conn.AuthVersion,
		ApprovedParents: parsed.ApprovedParents,
		Scopes:          append([]string(nil), parsed.Scopes...),
	}
```

**3c. 新增 `provider.go`：**

```go
package publish

// ProviderProfile is the ONLY place a provider's plan-level identity
// lives (AC1: 飞书差异只存在于 Adapter). Everything the service layer
// cannot decide generically — the reviewed app id, the publication-row
// provider brand, the action version, the conflict-result prefix, the
// block projection, the approved-snapshot byte shapes, the plan-time
// version pre-read and the receipt projection — is a profile field
// backed by adapter-domain functions.

import (
	"context"
	"encoding/json"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
)

type ProviderProfile struct {
	AppID         string // reviewed installation app id ("notion" / "feishu")
	Provider      string // app_publications.provider brand
	ActionVersion string // prepared action version tag
	// ConflictResultPrefix is the ProviderResult prefix the provider's
	// bridge records for a definitive version conflict; the handler maps
	// it onto 409.
	ConflictResultPrefix string
	// BlocksOf derives the provider's content blocks from artifact text.
	BlocksOf func(text string) ([]json.RawMessage, error)
	// CreateArgs / UpdateArgs marshal the approved snapshot bytes — the
	// field names ARE the provider's snapshot contract (approve-then-
	// rewrite protection parses exactly these).
	CreateArgs func(parent, title string, blocks []json.RawMessage) ([]byte, error)
	UpdateArgs func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error)
	// ReadRemoteVersion is the plan-formation pre-read (AC1 baseline).
	ReadRemoteVersion func(ctx context.Context, connectionID, destination string) (string, error)
	// ParseReceipt projects the adapter's output evidence onto
	// (externalID, externalVersion) for the publication settle.
	ParseReceipt func(providerResult string) (externalID, externalVersion string, err error)
}

// NotionProfile is #48's profile, extracted verbatim.
func NotionProfile(remote NotionRemoteReader) ProviderProfile {
	return ProviderProfile{
		AppID: "notion", Provider: "notion", ActionVersion: "notion/v1",
		ConflictResultPrefix: PublishVersionConflictResult,
		BlocksOf:             appconn.NotionParagraphBlocks,
		CreateArgs: func(parent, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"parent": parent, "title": title, "blocks": blocks})
		},
		UpdateArgs: func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"page_id": destination, "expected_version": expectedVersion, "title": title, "blocks": blocks})
		},
		ReadRemoteVersion: remote.ReadPageVersion,
		ParseReceipt: func(providerResult string) (string, string, error) {
			rcpt, err := appconn.ParseNotionPageReceipt([]byte(providerResult))
			return rcpt.ExternalID, rcpt.ExternalVersion, err
		},
	}
}

// FeishuProfile is #49's profile. The snapshot field names bind the
// FE-PUB-01 contract parsed by appconn.ParseFeishuDocCreate/UpdateSnapshot:
// create = exactly {parent_folder, title, blocks}; update = exactly
// {document_id, expected_revision, title, blocks}. The title is
// approval/ledger metadata — the Feishu create-document contract has no
// title parameter (SDK model.go:7895-7896), so nothing sends or compares it.
func FeishuProfile(read NotionRemoteReader) ProviderProfile {
	return ProviderProfile{
		AppID: "feishu", Provider: "feishu", ActionVersion: "feishu/v1",
		ConflictResultPrefix: FeishuVersionConflictResult,
		BlocksOf:             appconn.FeishuTextBlocks,
		CreateArgs: func(parent, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"parent_folder": parent, "title": title, "blocks": blocks})
		},
		UpdateArgs: func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"document_id": destination, "expected_revision": expectedVersion, "title": title, "blocks": blocks})
		},
		ReadRemoteVersion: read.ReadPageVersion,
		ParseReceipt: func(providerResult string) (string, string, error) {
			rcpt, err := appconn.ParseFeishuDocReceipt([]byte(providerResult))
			return rcpt.ExternalID, rcpt.ExternalVersion, err
		},
	}
}
```

**3d. 新增 `feishu.go`：**

```go
package publish

// The Feishu member of the publication seam (#49). Structurally the
// Notion bridge's twin (dispatcher.go): the A03 pipeline sees ONE
// adapter family behind the same three ports. The historical
// Notion-prefixed port type names (NotionScopeSource / NotionPolicyProvider
// / NotionTokenSource) are provider-neutral despite their names — they
// are reused unchanged so #48's surface does not churn.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// FeishuVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive revision-conflict refusal (mapped onto 409
// by the feishu publish handler, mirroring PublishVersionConflictResult).
const FeishuVersionConflictResult = "feishu_docx_revision_conflict"

// appIDFeishu is the installation app identity this bridge serves
// (appOAuthDefaults key, internal/handler/app_connector_oauth.go:49).
const appIDFeishu = "feishu"

// FeishuScopeSource aliases the shared scope port (the DB-backed
// production implementation NewDBNotionScopeSource is provider-neutral:
// connections → installations → app_versions).
type FeishuScopeSource = NotionScopeSource

// FeishuBridge adapts the Feishu docx adapter to the A03 pipeline's
// dispatch boundary. Outcome mapping is identical to NotionBridge.run
// (dispatcher.go:185-220): adapter FAILED → definitive failed outcome,
// adapter UNKNOWN → unknown outcome, wiring gaps → ErrDispatchNotStarted.
type FeishuBridge struct {
	scopes   FeishuScopeSource
	policies NotionPolicyProvider
	tokens   NotionTokenSource
	pubs     PublicationSource
}

func NewFeishuBridge(scopes FeishuScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *FeishuBridge {
	return &FeishuBridge{scopes: scopes, policies: policies, tokens: tokens, pubs: pubs}
}

var _ appconnectorsvc.ActionDispatcher = (*FeishuBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*FeishuBridge)(nil)
var _ NotionRemoteReader = (*FeishuBridge)(nil)

func (b *FeishuBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.NotionScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDFeishu {
		return nil, fmt.Errorf("%w: connection %q is %q, not a feishu connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	tok, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: token for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	// AC2: the write gate reads the connection's reviewed scopes — a
	// read/sync-only scope list never satisfies it.
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		for _, sc := range scope.Scopes {
			if sc == appconn.FeishuCapabilityWriteDocx {
				return scope.Scopes, nil
			}
		}
		return nil, fmt.Errorf("connection lacks %s", appconn.FeishuCapabilityWriteDocx)
	}
	progress := newFeishuPublicationProgress(b.pubs, snap.TenantID, snap.ID)
	return &appconn.FeishuDocxAdapter{
		Policy:                 pol,
		Token:                  func(ctx context.Context) (string, error) { return tok, nil },
		ConnectionCapabilities: caps,
		LoadProgress:           progress.load,
		SaveProgress:           progress.save,
	}, nil
}

// feishuPublicationProgress persists FE-03 checkpoints on the
// publication row's progress_json — the feishu twin of publicationProgress
// (dispatcher.go:155-183); load/save deliberately use context.Background()
// so the dispatch deadline cannot cut a checkpoint short.
type feishuPublicationProgress struct {
	pubs     PublicationSource
	tenantID uint64
	actionID string
}

func newFeishuPublicationProgress(pubs PublicationSource, tenantID uint64, actionID string) *feishuPublicationProgress {
	return &feishuPublicationProgress{pubs: pubs, tenantID: tenantID, actionID: actionID}
}

func (p *feishuPublicationProgress) load(a appconn.Action) appconn.FeishuDocProgress {
	row, err := p.pubs.FindByAction(context.Background(), p.tenantID, p.actionID)
	if err != nil {
		return appconn.FeishuDocProgress{}
	}
	var prog appconn.FeishuDocProgress
	if json.Unmarshal([]byte(row.ProgressJSON), &prog) != nil {
		return appconn.FeishuDocProgress{}
	}
	return prog
}

func (p *feishuPublicationProgress) save(a appconn.Action, prog appconn.FeishuDocProgress) error {
	raw, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	return p.pubs.SaveProgress(context.Background(), p.tenantID, p.actionID, string(raw))
}

func (b *FeishuBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
	adapter, err := b.adapterFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	action := appconn.Action{
		ID: snap.ID, TenantID: snap.TenantID, ActorID: snap.ActorID,
		ConnectionID: snap.ConnectionID, Version: snap.Version,
		Target: snap.Target, Risk: snap.Risk, AuthVersion: snap.AuthVersion,
		Args: json.RawMessage(snap.Args),
	}
	var res appconn.ActionResult
	var aerr error
	if query {
		res, aerr = adapter.Query(ctx, action)
	} else {
		res, aerr = adapter.Execute(ctx, action)
	}
	if aerr != nil {
		switch res.State {
		case appconn.ActionFailed:
			reason := aerr.Error()
			if errors.Is(aerr, appconn.ErrFeishuPublishRevisionConflict) {
				reason = FeishuVersionConflictResult + ": remote revision moved since approval"
			}
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: reason, ExecutionID: res.ExternalID}, nil
		case appconn.ActionUnknown:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: aerr.Error(), ExecutionID: res.ExternalID}, nil
		case appconn.ActionAwaitingApproval:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "approval revoked before send", ExecutionID: res.ExternalID}, nil
		default:
			return appconnectorsvc.DispatchOutcome{}, aerr
		}
	}
	return appconnectorsvc.DispatchOutcome{Status: res.State, ProviderResult: string(res.Output), ExecutionID: res.ExternalID}, nil
}

func (b *FeishuBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

func (b *FeishuBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadPageVersion is the plan-formation pre-read (AC1 baseline) through
// the same policy/token ports; the interface name is historical.
func (b *FeishuBridge) ReadPageVersion(ctx context.Context, connectionID, documentID string) (string, error) {
	scope, err := b.scopes.NotionScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDFeishu {
		return "", fmt.Errorf("connection %q is not a feishu connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	tok, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	v, err := appconn.ReadFeishuDocumentVersion(ctx, pol, func(ctx context.Context) (string, error) { return tok, nil }, documentID)
	if err != nil {
		if errors.Is(err, appconn.ErrFeishuPublishNotFound) {
			// The create destination is a reviewed FOLDER: it has no
			// document revision. An EMPTY baseline (nil error) is the
			// provider's honest "no version concept" answer — the plan
			// layer accepts it for create and still refuses it for
			// update (whose destination is a previously published
			// document; a deleted one must fail the plan).
			return "", nil
		}
		return "", err
	}
	return v.RevisionID, nil
}

// ---- production ports ----

// constantFeishuPolicyProvider pins the reviewed FE-PUB-01 outbound
// contract: HTTPS to the pinned Feishu host, GET/POST under the docx
// path, 30s per-request timeout.
type constantFeishuPolicyProvider struct{}

func NewConstantFeishuPolicyProvider() NotionPolicyProvider { return constantFeishuPolicyProvider{} }

func (constantFeishuPolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return appconn.HTTPPolicy{
		Scheme:     "https",
		Host:       appconn.FeishuAPIHost,
		Methods:    []string{"GET", "POST"},
		PathPrefix: "/open-apis/docx/",
		Timeout:    30 * time.Second,
	}, nil
}

// feishuCredentialTokenSource reuses the A02 credential resolver via the
// existing constructor; the alias documents the feishu wiring point.
var NewFeishuCredentialTokenSource = NewCredentialTokenSource
```

- [ ] **Step 4: 运行新测试 + #48 回归（关键门）**

```bash
go test ./internal/modules/appconnector/publish/ -count=1
```

Expected: PASS——**既有 `plan_test.go`/`dispatcher_test.go`/`blocks_test.go` 全部保持绿是本任务最重要的回归证据**（NewNotionPublishService 签名未变；Notion 行为逐点保持）。新测试全绿。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/publish/provider.go internal/modules/appconnector/publish/feishu.go \
  internal/modules/appconnector/publish/plan.go internal/modules/appconnector/publish/dispatcher.go \
  internal/modules/appconnector/publish/provider_test.go internal/modules/appconnector/publish/feishu_test.go
git commit -m "refactor(publish): 服务层 provider 中立化（ProviderProfile）+ FeishuBridge——飞书差异只存在于 Adapter"
```

---

### Task 4: 飞书发布 HTTP 面——handler、router、container 装配

**Files:**
- Create: `internal/handler/app_connector_feishu_publish.go`
- Create: `internal/router/routes_app_feishu_publish.go`
- Create: `internal/container/feishu_publish.go`
- Modify: `internal/router/router.go`（+1 参数字段 +1 Register 行）
- Modify: `internal/container/container.go`（+1 行 Provide）
- Test: `internal/handler/app_connector_feishu_publish_test.go`

**Interfaces:**
- Consumes: Task 3 的 `publish.NewProviderPublishService` / `publish.FeishuProfile` / `publish.NewFeishuBridge` / `publish.NewConstantFeishuPolicyProvider` / `publish.NewCredentialTokenSource` / `publish.FeishuVersionConflictResult`；#48 既有 `repoappconn.NewPublicationStore`、`appconnectorsvc.NewActionService/NewCredentialResolver`、`publish.NewDBNotionScopeSource`、`container.tenantStorageArtifactContent`（`notion_publish.go:59-98`，跨文件复用同包类型，零改动）；handler 既有 `appTenantScope`（`app_connector.go:46`）、`appRequireWriteCapability`（`app_connector.go:60`）、`appconnector.CanDriveActionWrites`、`appOK/appFail`（`app_connector.go:28-32`）。
- Produces: `handler.AppFeishuPublishHandler`（`NewAppFeishuPublishHandler(db)`、`SetFeishuPublishService(*publish.NotionPublishService)`、`RequireActionCapabilityForWrites()`、`FormFeishuPublishPlan/PublishFeishuAction/ReconcileFeishuAction/GetFeishuPublication`）；`router.RegisterAppFeishuPublishRoutes(r, h)`；`container.newFeishuPublishHandler(...)`（dig Provider，参数集与 `newNotionPublishHandler` 相同）。wire 面 `POST/…/api/v1/apps/feishu-publish/{plans,actions/:id/publish,actions/:id/reconcile}` 与 `GET actions/:id`，冲突 409 码 `FEISHU_PUBLISH_REVISION_CONFLICT`。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/app_connector_feishu_publish_test.go`（镜像 `app_connector_notion_publish_test.go` 的谓词钉测试；publish 管线本体在服务层与 E2E 验证，这里钉 HTTP 谓词）：

```go
package handler

// Feishu publish endpoint gates (#49). Mirrors the notion publish gates:
// role gate, personal-connection owner predicate, fail-closed
// unconfigured service, malformed input, tenant-scoped lookups.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newFeishuPublishTestEngine(t *testing.T) (*gin.Engine, *AppFeishuPublishHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.AppVersion{}, &appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{}, &appconnectorrepo.PublicationRow{}))
	// conn-feishu is user-a's PERSONAL feishu connection with the
	// reviewed write_docx scope; conn-feishu-ro carries only read scopes.
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-feishu", TenantID: 7, AppID: "feishu", AppVersion: "v1", State: "active"}).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('feishu', 'v1', '{"scopes":["write_docx"],"approved_parents":["fld-1"]}', '{}')`).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-feishu", InstallationID: "inst-feishu", Kind: "personal", OwnerID: "user-a", CredentialRef: "mcp_oauth_token:feishu", State: "active", AuthVersion: 1}).Error)

	h := NewAppFeishuPublishHandler(db)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		tenant := uint64(7)
		role := "admin"
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		if c.GetHeader("X-Test-Role") != "" {
			role = c.GetHeader("X-Test-Role")
		}
		if c.GetHeader("X-Test-Tenant") == "8" {
			tenant = 8
		}
		c.Request = c.Request.WithContext(publishTestContext(c.Request.Context(), tenant, role, user))
		c.Next()
	})
	g := engine.Group("/api/v1/apps/feishu-publish", h.RequireActionCapabilityForWrites())
	g.POST("/plans", h.FormFeishuPublishPlan)
	g.POST("/actions/:id/publish", h.PublishFeishuAction)
	g.POST("/actions/:id/reconcile", h.ReconcileFeishuAction)
	g.GET("/actions/:id", h.GetFeishuPublication)
	return engine, h, db
}

func feishuPublishDo(t *testing.T, engine *gin.Engine, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestFeishuPublishPlanGates(t *testing.T) {
	engine, _, _ := newFeishuPublishTestEngine(t)
	body := `{"connection_id":"conn-feishu","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"fld-1"}`

	// Service not wired: fail closed 501.
	w := feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body)
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_PIPELINE_NOT_CONFIGURED")

	// Viewer role: the write gate refuses.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body, "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Another member may not use user-a's PERSONAL connection.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body, "X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "NOT_CONNECTION_OWNER")

	// Both destination shapes at once: malformed.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans",
		`{"connection_id":"conn-feishu","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"f","page_id":"d"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_REQUEST")

	// Unknown connection: 404 without leaking existence details.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans",
		`{"connection_id":"conn-nope","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"fld-1"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestFeishuPublishActionLookupIsTenantScoped(t *testing.T) {
	engine, h, db := newFeishuPublishTestEngine(t)
	h.SetFeishuPublishService(nil)
	require.NoError(t, db.Create(&appconnectorrepo.ActionRow{ID: "act-fx", TenantID: 7, ConnectionID: "conn-feishu",
		AppVersion: "feishu/v1", Target: "fld-1", Risk: "write", ArgsSnapshot: "{}", ArgsDigest: "d",
		State: "authorized"}).Error)
	w := feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-fx/publish", "")
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	// Foreign tenant id is indistinguishable from missing: 404.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-fx/publish", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = feishuPublishDo(t, engine, http.MethodGet, "/api/v1/apps/feishu-publish/actions/act-fx", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/handler/ -run 'TestFeishuPublish' -count=1
```

Expected: FAIL，`undefined: NewAppFeishuPublishHandler`。

- [ ] **Step 3: 最小实现**

**3a. `internal/handler/app_connector_feishu_publish.go`**（镜像 `app_connector_notion_publish.go`；差异：路由前缀文案、冲突码 `FEISHU_PUBLISH_REVISION_CONFLICT`）：

```go
package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppFeishuPublishHandler serves the T19 (#49) Feishu publish closed
// loop under /api/v1/apps/feishu-publish — the structural twin of
// AppNotionPublishHandler over the same provider-neutral publish
// service. Plan formation derives the approved snapshot SERVER-SIDE
// from a confirmed artifact version + destination; approval stays on
// the existing POST /apps/actions/:id/approve endpoint; execution and
// reconciliation run through the publish service whose ActionService
// holds the Feishu bridge. Nil-service fail-closed like its siblings.
type AppFeishuPublishHandler struct {
	db      *gorm.DB
	publish *publish.NotionPublishService
}

func NewAppFeishuPublishHandler(db *gorm.DB) *AppFeishuPublishHandler {
	return &AppFeishuPublishHandler{db: db}
}

// SetFeishuPublishService wires the publish service (container
// injection point). Until called every endpoint fails closed with 501.
func (h *AppFeishuPublishHandler) SetFeishuPublishService(s *publish.NotionPublishService) {
	h.publish = s
}

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation and execution are action writes.
func (h *AppFeishuPublishHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"publish writes require owner or admin role")
}

type feishuPublishPlanInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"` // create: the reviewed folder token
	PageID            string `json:"page_id"`        // update: the published document id
}

func (h *AppFeishuPublishHandler) feishuActionByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.ActionRow, bool) {
	var row appconnectorrepo.ActionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_NOT_FOUND", "action not found")
		return row, false
	}
	return row, true
}

func (h *AppFeishuPublishHandler) feishuConnection(c *gin.Context, tenantID uint64, connectionID, userID string) (appconnectorrepo.ConnectionRow, bool) {
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, connectionID).First(&conn).Error; err != nil {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return conn, false
	}
	if conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID != userID {
		appFail(c, http.StatusForbidden, "NOT_CONNECTION_OWNER",
			"a personal connection may only be used by its owner")
		return conn, false
	}
	return conn, true
}

// FormFeishuPublishPlan POST /apps/feishu-publish/plans — derives the
// approved publish snapshot server-side, reads the external revision,
// prepares the A03 action and records the planned publication.
func (h *AppFeishuPublishHandler) FormFeishuPublishPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input feishuPublishPlanInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.SessionID == "" ||
		input.ArtifactVersionID == "" || input.Title == "" ||
		(input.ParentPageID == "") == (input.PageID == "") {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"connection_id, session_id, artifact_version_id, title and exactly one of parent_page_id / page_id are required")
		return
	}
	if _, ok := h.feishuConnection(c, tenantID, input.ConnectionID, userID); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Feishu publish pipeline is not wired in this environment; refusing to form a plan")
		return
	}
	view, err := h.publish.FormPlan(c.Request.Context(), publish.PublishPlanInput{
		TenantID: tenantID, ActorID: userID, ConnectionID: input.ConnectionID,
		SessionID: input.SessionID, ArtifactVersionID: input.ArtifactVersionID,
		Title: input.Title, ParentPageID: input.ParentPageID, PageID: input.PageID,
	})
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// PublishFeishuAction POST /apps/feishu-publish/actions/:id/publish.
func (h *AppFeishuPublishHandler) PublishFeishuAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.feishuActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Feishu publish pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	outcome, err := h.publish.Execute(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	if outcome.Conflict {
		appFail(c, http.StatusConflict, "FEISHU_PUBLISH_REVISION_CONFLICT",
			"the external document changed since approval; form a new plan from its current revision")
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// ReconcileFeishuAction POST /apps/feishu-publish/actions/:id/reconcile.
func (h *AppFeishuPublishHandler) ReconcileFeishuAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.feishuActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Feishu publish pipeline is not wired in this environment; refusing to reconcile")
		return
	}
	outcome, err := h.publish.Reconcile(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// GetFeishuPublication GET /apps/feishu-publish/actions/:id.
func (h *AppFeishuPublishHandler) GetFeishuPublication(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.feishuActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Feishu publish pipeline is not wired in this environment")
		return
	}
	view, err := h.publish.Receipt(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

func (h *AppFeishuPublishHandler) failPublish(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publish.ErrPublishInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid publish plan input")
	case errors.Is(err, publish.ErrPublishArtifactNotReady):
		appFail(c, http.StatusNotFound, "ARTIFACT_VERSION_NOT_ACCESSIBLE", "the artifact version is not readable for this session")
	case errors.Is(err, publish.ErrPublishUnsupportedArtifact):
		appFail(c, http.StatusUnsupportedMediaType, "PUBLISH_UNSUPPORTED_ARTIFACT", "only text artifacts are publishable in this version")
	case errors.Is(err, publish.ErrPublishContentTooLarge):
		appFail(c, http.StatusRequestEntityTooLarge, "PUBLISH_CONTENT_TOO_LARGE", "the artifact exceeds the publish bounds")
	case errors.Is(err, publish.ErrPublishEmptyContent):
		appFail(c, http.StatusBadRequest, "PUBLISH_EMPTY_CONTENT", "the artifact carries no publishable text")
	case errors.Is(err, publish.ErrPublishDestinationOutOfScope):
		appFail(c, http.StatusForbidden, "PUBLISH_DESTINATION_OUT_OF_SCOPE", "the destination is not in the reviewed scope for this connection")
	case errors.Is(err, publish.ErrPublishDestinationUnreadable):
		appFail(c, http.StatusBadGateway, "PUBLISH_DESTINATION_UNREADABLE", "the external destination could not be read; no plan was formed")
	case errors.Is(err, publish.ErrPublishUpdateTargetNotPublished):
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only documents this workspace published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_PLAN_FAILED", "failed to form the publish plan")
	}
}

func (h *AppFeishuPublishHandler) failPublishExecute(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconnectorsvc.ErrActionState):
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrNoDispatcher):
		appFail(c, http.StatusServiceUnavailable, "OC_DISPATCH_NOT_CONFIGURED",
			"no outbound dispatcher is configured in this deployment; refusing to fabricate a dispatch")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_EXECUTE_FAILED", "publish execution failed")
	}
}
```

**3b. `internal/router/routes_app_feishu_publish.go`**：

```go
package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppFeishuPublishRoutes registers the T19 (#49) Feishu publish
// closed loop under /api/v1/apps/feishu-publish. Like the notion publish
// routes, these are intentionally NOT declared in the API-key route
// authorizer: the /api/v1 gate default-denies every X-API-Key principal.
func RegisterAppFeishuPublishRoutes(r *gin.RouterGroup, h *handler.AppFeishuPublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/feishu-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormFeishuPublishPlan)
		g.POST("/actions/:id/publish", h.PublishFeishuAction)
		g.POST("/actions/:id/reconcile", h.ReconcileFeishuAction)
		g.GET("/actions/:id", h.GetFeishuPublication)
	}
}
```

**3c. `internal/router/router.go` 修改**（`RouterParams` 字段区，`router.go:145` 的 `AppNotionPublishHandler` 字段之后）：

```go
// old
	AppNotionPublishHandler *handler.AppNotionPublishHandler
// new（追加一行）
	AppNotionPublishHandler *handler.AppNotionPublishHandler
	AppFeishuPublishHandler *handler.AppFeishuPublishHandler
```

注册行（`router.go:436` 之后）：

```go
// old
		RegisterAppNotionPublishRoutes(v1, params.AppNotionPublishHandler)
// new（追加一行）
		RegisterAppNotionPublishRoutes(v1, params.AppNotionPublishHandler)
		RegisterAppFeishuPublishRoutes(v1, params.AppFeishuPublishHandler)
```

**3d. `internal/container/feishu_publish.go`**：

```go
package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"gorm.io/gorm"
)

// newFeishuPublishHandler is the T19 (#49) dig constructor — the feishu
// twin of newNotionPublishHandler (notion_publish.go): the same ActionStore
// authority, A02 guard and U05 gate; its dispatcher AND unknown resolver
// are the Feishu bridge; the provider difference is the FeishuProfile.
func newFeishuPublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppFeishuPublishHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewFeishuBridge(
		scopeSrc,
		publish.NewConstantFeishuPolicyProvider(),
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewProviderPublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, scopeSrc,
		publish.FeishuProfile(bridge))
	h := handler.NewAppFeishuPublishHandler(db)
	h.SetFeishuPublishService(svc)
	return h, nil
}
```

**3e. `internal/container/container.go`**（`container.go:1013` 的 `must(container.Provide(newNotionPublishHandler))` 之后追加一行）：

```go
		must(container.Provide(newNotionPublishHandler))
		must(container.Provide(newFeishuPublishHandler))
```

- [ ] **Step 4: 运行测试 + 全链编译**

```bash
go test ./internal/handler/ -run 'TestFeishuPublish' -count=1
go build ./...
```

Expected: 测试 PASS；`go build ./...` 成功（router/container 接线编译通过）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/app_connector_feishu_publish.go internal/handler/app_connector_feishu_publish_test.go \
  internal/router/routes_app_feishu_publish.go internal/router/router.go \
  internal/container/feishu_publish.go internal/container/container.go
git commit -m "feat(feishu-publish): 飞书发布 HTTP 面——handler/router/container 装配"
```

---

### Task 5: E2E 端到端证据（最高稳定 Interface）

**Files:**
- Test: `internal/handler/app_connector_feishu_publish_e2e_test.go`

**Interfaces:**
- Consumes: Task 0（可装载迁移轨道）、Task 3（provider 中立服务 + FeishuBridge）、Task 4（HTTP 面）；#48 E2E 的全部夹具模式（`app_connector_notion_publish_e2e_test.go`：`openNotionPublishE2EDB`、种子行集合、loopback policy、中间件注入、`formPlan/approve` 流）。
- Produces: AC1/AC2/AC3 的本地可验证端到端证据；真实迁移 sqlite 上 `app_publications` 的 `provider='feishu'` 行为证据。

验收 3（「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据」）的落实方式与 #48 完全一致（`app_connector_notion_publish_e2e_test.go:3-12` 的声明）：全量生产迁移 sqlite + 真实 `ActionService`/`PublicationStore`/`FeishuBridge`/publish 服务/handler/**既有审批端点**，唯一替身是飞书 wire 端点（按官方 SDK 合同形状构造的 httptest 契约替身）。真实 Provider 证据单独走 Task 6 的环境变量门控，SKIP 永不冒充 PASS。

- [ ] **Step 1: 写 E2E 测试（完整代码）**

创建 `internal/handler/app_connector_feishu_publish_e2e_test.go`：

```go
package handler

// End-to-end evidence for T19 (#49). Everything runs on a FULLY MIGRATED
// sqlite database (the production migrations/sqlite track) with the REAL
// ActionService / PublicationStore / FeishuBridge / publish service /
// handlers and the REAL approval endpoint. The ONLY replaced piece is
// the Feishu docx wire endpoint: a local contract double implementing
// the official docx/v1 shapes (envelope {code,msg,data}, numeric
// revision_id, children carrying server-side fields) pinned against the
// official oapi-sdk-go v3.9.7 source. This is the highest stable
// Interface evidence available without provider credentials — it is NOT
// the real-provider acceptance, which stays FEISHU_*-gated (Task 6) and
// honestly SKIPs in this environment.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openFeishuPublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "feishu-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

// ---- the Feishu docx contract double ----

type e2eFeishu struct {
	mu          sync.Mutex
	token       string
	docs        map[string]*e2eFeishuDoc
	nextID      int
	createCalls int
	appendCalls int
	appendSizes []int
	// dropNextAppend aborts the NEXT append request after the effect
	// applied (connection lost mid-flight → unknown outcome).
	dropNextAppend bool
}

type e2eFeishuDoc struct {
	id       string
	revision int
	children []json.RawMessage
}

func newE2EFeishu(token string) *e2eFeishu {
	return &e2eFeishu{token: token, docs: map[string]*e2eFeishuDoc{}}
}

func (e *e2eFeishu) lock()   { e.mu.Lock() }
func (e *e2eFeishu) unlock() { e.mu.Unlock() }

func (e *e2eFeishu) addDoc(id string, revision int) {
	e.lock()
	e.docs[id] = &e2eFeishuDoc{id: id, revision: revision}
	e.unlock()
}

func (e *e2eFeishu) childCount(id string) int {
	e.lock()
	defer e.unlock()
	return len(e.docs[id].children)
}

func (e *e2eFeishu) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+e.token }
	mux.HandleFunc("/open-apis/docx/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		e.lock()
		defer e.unlock()
		e.createCalls++
		e.nextID++
		id := fmt.Sprintf("doc-%d", e.nextID)
		e.docs[id] = &e2eFeishuDoc{id: id, revision: 1}
		write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":1,"title":""}}}`, id))
	})
	mux.HandleFunc("/open-apis/docx/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/open-apis/docx/v1/documents/")
		if !strings.Contains(rest, "/blocks/") {
			id := rest
			e.lock()
			defer e.unlock()
			d, ok := e.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, d.id, d.revision))
			return
		}
		if !strings.HasSuffix(rest, "/children") {
			write(w, `{"code":99991661,"msg":"unknown path"}`)
			return
		}
		id := strings.SplitN(rest, "/", 2)[0]
		e.lock()
		defer e.unlock()
		d, ok := e.docs[id]
		if !ok {
			write(w, `{"code":99991661,"msg":"not found"}`)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Children []json.RawMessage `json:"children"`
				Index    *int              `json:"index"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			// Record the effect FIRST, then optionally drop the reply —
			// the exact partial-success shape the notion double pins.
			e.appendCalls++
			e.appendSizes = append(e.appendSizes, len(req.Children))
			d.children = append(d.children, req.Children...)
			d.revision++
			if e.dropNextAppend {
				e.dropNextAppend = false
				panic(http.ErrAbortHandler)
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"children":[],"document_revision_id":%d}}`, d.revision))
		case http.MethodGet:
			write(w, fmt.Sprintf(`{"code":0,"data":{"items":[%s],"page_token":"","has_more":false}}`,
				strings.Join(rawStrings(d.children), ",")))
		default:
			write(w, `{"code":99991661,"msg":"method"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- the full-stack environment ----

type feishuE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eFeishu
}

func newFeishuPublishE2E(t *testing.T) *feishuE2EEnv {
	return newFeishuPublishE2EWithScopes(t, `{"scopes":["write_docx"],"approved_parents":["fld-1"]}`, "conn-feishu")
}

// newFeishuPublishE2EWithScopes seeds a second feishu app version /
// connection with arbitrary reviewed scopes (the AC2 read-only variant).
func newFeishuPublishE2EWithScopes(t *testing.T, schemaJSON, connID string) *feishuE2EEnv {
	t.Helper()
	db := openFeishuPublishE2EDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (9, 't9', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 9)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (9, ?, 'contributor', 'active')`, u).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-9', 9, 'task-9', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 9, RunID: "run-9"}, SessionID: "sess-9", UserID: "user-a",
		RequestID: "q9", AssistantMessageID: "a9", RequestHash: "hash-9",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	baseDir := t.TempDir()
	digest := strings.Repeat("b", 64)
	objectKey := fmt.Sprintf("artifact-versions/9/run-9/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("飞书发布第一段。\n\n飞书发布第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (9, 'ver-9', 'run-9', 'sess-9', ?, ?, 'text/plain', 'ready', 30)`, digest, objectKey).Error)
	// Feishu installation + reviewed app version (schema_json drives AC2)
	// + user-a's personal connection + its token row.
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-feishu', 9, 'feishu', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('feishu', 'v1', ?, '{}')`, schemaJSON).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (9, ?, 'inst-feishu', 'personal', 'user-a', 'mcp_oauth_token:feishu', 'active', 1)`, connID).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('feishu', 9, 'feishu', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-f9', 9, 'user-a', 'web_user', 'user-a', 'feishu', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)

	fake := newE2EFeishu("secret_test_token")
	srv := fake.server(t)

	// The production composition with the loopback policy provider (the
	// documented test hook) replacing the pinned open.feishu.cn one.
	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewFeishuBridge(scopeSrc, &feishuE2EPolicy{pol: pol},
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))), pubs)
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &feishuE2EContent{svc: filesvc.NewLocalFileService(baseDir, "")}
	svc := publish.NewProviderPublishService(publishActions, store, pubs,
		repository.NewArtifactVersionStore(db), content, scopeSrc, publish.FeishuProfile(bridge))

	publishHandler := NewAppFeishuPublishHandler(db)
	publishHandler.SetFeishuPublishService(svc)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(publishActions)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		role := types.TenantRoleAdmin
		if c.GetHeader("X-Test-Role") != "" {
			role = types.TenantRole(c.GetHeader("X-Test-Role"))
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(9))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	g := v1.Group("/apps/feishu-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormFeishuPublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishFeishuAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileFeishuAction)
		g.GET("/actions/:id", publishHandler.GetFeishuPublication)
	}
	return &feishuE2EEnv{engine: engine, db: db, fake: fake}
}

type feishuE2EPolicy struct{ pol appconn.HTTPPolicy }

func (p *feishuE2EPolicy) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return p.pol, nil
}

type feishuE2EContent struct{ svc interfaces.FileService }

func (e *feishuE2EContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	reader, err := e.svc.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (e *feishuE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *feishuE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *feishuE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func decodePublishOutcome(t *testing.T, w *httptest.ResponseRecorder) struct {
	ActionState string `json:"action_state"`
	Conflict    bool   `json:"conflict"`
	Publication struct {
		State             string `json:"state"`
		ExternalID        string `json:"external_id"`
		ExternalVersion   string `json:"external_version"`
		ArtifactVersionID string `json:"artifact_version_id"`
	} `json:"publication"`
} {
	t.Helper()
	var out struct {
		Data struct {
			ActionState string `json:"action_state"`
			Conflict    bool   `json:"conflict"`
			Publication struct {
				State             string `json:"state"`
				ExternalID        string `json:"external_id"`
				ExternalVersion   string `json:"external_version"`
				ArtifactVersionID string `json:"artifact_version_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data
}

// TestFeishuPublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external revision baseline read) → approve on the EXISTING endpoint →
// publish → receipt with the real document id + revision; the
// migration-aligned publication row lands 'published' with
// provider='feishu'.
func TestFeishuPublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "fld-1", plan["destination"])
	require.Empty(t, plan["expected_external_version"],
		"AC1: a feishu FOLDER destination carries no revision — the plan records an empty baseline (typed not-found)")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decodePublishOutcome(t, w)
	require.Equal(t, "succeeded", out.ActionState)
	require.Equal(t, "published", out.Publication.State)
	require.NotEmpty(t, out.Publication.ExternalID, "the receipt saves the real document id")
	require.NotEmpty(t, out.Publication.ExternalVersion, "the receipt saves the revision the publish produced")
	require.Equal(t, "ver-9", out.Publication.ArtifactVersionID)

	// The publication row is durable and provider-branded.
	row, err := repoappconn.NewPublicationStore(env.db).FindByAction(context.Background(), 9, plan["action_id"].(string))
	require.NoError(t, err)
	require.Equal(t, "feishu", row.Provider)
	require.Equal(t, "published", row.State)

	// The receipt is queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published document exists remotely with the derived blocks, and
	// the title was NEVER sent (the create contract has no title field).
	env.fake.lock()
	doc := env.fake.docs[out.Publication.ExternalID]
	env.fake.unlock()
	require.NotNil(t, doc)
	require.Len(t, doc.children, 2, "two paragraphs derived from the artifact text")
}

// TestFeishuPublishEndToEndUpdateRevisionConflict: AC1 — an external
// edit between plan formation and publish is detected at execute time
// and the publish is refused with 409 and ZERO write requests.
func TestFeishuPublishEndToEndUpdateRevisionConflict(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out1 := decodePublishOutcome(t, w)
	docID := out1.Publication.ExternalID
	require.NotEmpty(t, docID)

	// The update plan reads the document's CURRENT revision.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告 v2","page_id":%q}`, docID))
	require.Equal(t, "update", plan2["mode"])
	require.NotEmpty(t, plan2["expected_external_version"], "AC1: the update plan read the document's live revision")
	env.approve(t, plan2)

	// External collaborator edits between approval and publish: bump the
	// remote revision directly on the double.
	env.fake.lock()
	appendsBefore := env.fake.appendCalls
	env.fake.docs[docID].revision += 5
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "FEISHU_PUBLISH_REVISION_CONFLICT")
	env.fake.lock()
	require.Equal(t, appendsBefore, env.fake.appendCalls, "AC1: conflict leaves ZERO block writes")
	env.fake.unlock()
}

// TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst: 部分成功核对 —
// a lost append reply parks the action unknown; a blind re-publish is
// structurally refused; reconcile reads the REMOTE first and settles the
// receipt with no re-dispatch ever.
func TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out1 := decodePublishOutcome(t, w)
	docID := out1.Publication.ExternalID

	// The update plan; the append lands remotely but its reply is lost
	// (the double records the effect FIRST, then drops the response).
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告 v2","page_id":%q}`, docID))
	env.approve(t, plan2)
	env.fake.lock()
	env.fake.dropNextAppend = true
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store; the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsBefore+1, env.fake.appendCalls, "the dropped append counted once; NO re-dispatch happened")
	appendsAfterUnknown := env.fake.appendCalls
	env.fake.unlock()

	// Reconcile: provider query FIRST — the dropped batch's content IS
	// remote (recorded before the drop), so the semantic content check
	// settles success and the receipt lands published, with no re-send.
	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsAfterUnknown, env.fake.appendCalls, "reconcile must not re-send")
	env.fake.unlock()
}

// TestFeishuPublishEndToEndReadOnlyScopeCannotPublish: AC2 — a
// connection whose reviewed scopes carry only read/sync capabilities can
// never publish: the terminal outcome is failed and ZERO writes leave
// the process, at the highest stable Interface.
func TestFeishuPublishEndToEndReadOnlyScopeCannotPublish(t *testing.T) {
	env := newFeishuPublishE2EWithScopes(t, `{"scopes":["read_docx","sync_content"],"approved_parents":["fld-1"]}`, "conn-feishu")
	// Plan formation does not check capabilities (the approval still
	// binds real content — same predicate split as #48).
	plan := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"只读连接","parent_page_id":"fld-1"}`)
	env.approve(t, plan)

	env.fake.lock()
	createsBefore := env.fake.createCalls
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decodePublishOutcome(t, w)
	require.Equal(t, "failed", out.ActionState, "AC2: read-only scope can never write")
	require.Equal(t, "failed", out.Publication.State)

	env.fake.lock()
	require.Equal(t, createsBefore, env.fake.createCalls, "zero document creates")
	require.Equal(t, appendsBefore, env.fake.appendCalls, "zero block appends")
	env.fake.unlock()

	// Reading/syncing itself is untouched: the datasource feishu
	// connector (knowledge import) gained no write path — asserted
	// structurally by this suite never granting one (grep-level guarantee
	// documented in the plan's差异记录; the runtime proof is the zero
	// write counts above).
}
```

注意 `feishuE2EContent.ReadArtifactContent` 需要 import `"io"`（顶部 import 块补入）。`e2ePassGuard` 已由 `app_connector_notion_publish_e2e_test.go` 定义（同包复用，勿重复定义）；`rawStrings` 同样复用。`appconn` import 供 `TestFeishuPublishEndToEndUpdateRevisionConflict` 直接操纵 fake revision 使用；若最终代码未引用则从 import 中移除（`go vet` 会提示）。

- [ ] **Step 2: 运行确认失败（实现缺失时）**

Task 5 在 Task 3/4 之后执行，此时实现已在；此步确认四个测试初始可运行：

```bash
go test ./internal/handler/ -run 'TestFeishuPublishEndToEnd' -count=1 -v 2>&1 | tail -12
```

Expected: 若前序任务实现正确则全绿；任何 RED 都是前序任务的回归信号，先修前序再继续。

- [ ] **Step 3: 运行 #48 E2E 回归（同包共存门）**

```bash
go test ./internal/handler/ -run 'TestNotionPublishEndToEnd|TestAppPublicationsTableExists' -count=1
```

Expected: PASS——#48 的 E2E 在本计划改动后必须逐字节保持绿。

- [ ] **Step 4: Commit**

```bash
git add internal/handler/app_connector_feishu_publish_e2e_test.go
git commit -m "test(feishu-publish): 全迁移 E2E——创建/审批/发布/回执、版本冲突 409 零写、unknown→核对零重发、只读 scope fail closed（AC1/AC2/AC3）"
```

---

### Task 6: 真实 Provider opt-in 证据（blocked-env 如实声明）

**Files:**
- Test: `internal/modules/appconnector/feishu_publish_real_test.go`

**Interfaces:**
- Consumes: Task 1/2 的 Adapter 全家（`FeishuDocxAdapter`、`ReadFeishuDocumentVersion`、`ParseFeishuDocReceipt`、`FeishuTextBlocks`、`FeishuCapabilityWriteDocx`、`FeishuAPIHost`）。
- Produces: `FEISHU_APP_ID`/`FEISHU_APP_SECRET`/`FEISHU_TEST_FOLDER_TOKEN` 三环境变量门控的真实闭环证据。**本环境无凭据 → 该验收项如实列为 blocked-env；SKIP 不是 PASS。**

- [ ] **Step 1: 写真实 Provider 测试**

创建 `internal/modules/appconnector/feishu_publish_real_test.go`（镜像 `notion_publish_real_test.go` 的门控纪律）：

```go
package appconnector

// Real-provider controlled evidence for the Feishu publish loop (#49):
// the user designates a test folder and a tenant-level app (app id +
// app secret from the deployment env). The REAL adapter runs the full
// closed loop against open.feishu.cn: create (baseline revision read) →
// receipt → append to the SAME document (revision read + conflict-free
// write) → a stale-revision append must be REFUSED with zero writes.
// Gated on FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN; a
// skip is never a pass — T19 real-provider evidence stays blocked-env.
//
// Token acquisition mirrors the datasource feishu connector's internal
// auth (client.go:92 tenant_access_token/internal); the docx API accepts
// tenant tokens (SDK SupportedAccessTokenTypes: Tenant + User).
func TestFeishuRealPublishLoop(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	folder := os.Getenv("FEISHU_TEST_FOLDER_TOKEN")
	if appID == "" || appSecret == "" || folder == "" {
		t.Skip("feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env")
	}
	ctx := context.Background()
	token, terr := feishuTenantAccessToken(ctx, appID, appSecret)
	if terr != nil {
		t.Fatalf("real tenant token: %v", terr)
	}
	pol := HTTPPolicy{
		Scheme: "https", Host: FeishuAPIHost,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		Timeout: 30 * time.Second,
	}
	tok := func(ctx context.Context) (string, error) { return token, nil }
	progress := map[string]FeishuDocProgress{}
	ad := &FeishuDocxAdapter{
		Policy: pol, Token: tok,
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) {
			return []string{FeishuCapabilityWriteDocx}, nil
		},
		LoadProgress: func(a Action) FeishuDocProgress { return progress[a.ID] },
		SaveProgress: func(a Action, p FeishuDocProgress) error { progress[a.ID] = p; return nil },
	}
	stamp := time.Now().Format("150405")
	blocks, _ := FeishuTextBlocks("t19 real publish " + stamp)
	createArgs, _ := json.Marshal(map[string]any{"parent_folder": folder, "title": "T19 real " + stamp, "blocks": blocks})
	createAction := Action{ID: "act_t19_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: folder, Risk: RiskWrite, Args: createArgs}

	out, err := ad.Execute(ctx, createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	docID := out.ExternalID
	rcpt, rerr := ParseFeishuDocReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != docID {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL document created: id=%s (title is empty by contract — the create API has no title field)", docID)

	// Append to the SAME document: read its current revision, then write.
	live, lerr := ReadFeishuDocumentVersion(ctx, pol, tok, docID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	updateBlocks, _ := FeishuTextBlocks("t19 real update " + stamp)
	updateArgs, _ := json.Marshal(map[string]any{"document_id": docID, "expected_revision": live.RevisionID,
		"title": "T19 real " + stamp, "blocks": updateBlocks})
	updateAction := Action{ID: "act_t19_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: docID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := ad.Execute(ctx, updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseFeishuDocReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalVersion == live.RevisionID {
		t.Fatalf("real update receipt must carry a NEW revision: %+v (had %s)", urcpt, live.RevisionID)
	}
	t.Logf("REAL document updated: id=%s revision %s -> %s", docID, live.RevisionID, urcpt.ExternalVersion)

	// Stale-revision append: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{"document_id": docID, "expected_revision": live.RevisionID,
		"title": "T19 stale " + stamp, "blocks": updateBlocks})
	staleAction := Action{ID: "act_t19_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: docID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := ad.Execute(ctx, staleAction)
	if !errors.Is(serr, ErrFeishuPublishRevisionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale append must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale append refused with zero writes (conflict on %s)", live.RevisionID)
}

// feishuTenantAccessToken exchanges app credentials for a
// tenant_access_token (the internal-auth shape the datasource feishu
// connector also uses).
func feishuTenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+FeishuAPIHost+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.TenantAccessToken == "" {
		return "", fmt.Errorf("token refused: code=%d msg=%s", out.Code, out.Msg)
	}
	return out.TenantAccessToken, nil
}
```

（import 补齐：`bytes`、`context`、`encoding/json`、`errors`、`fmt`、`io`、`net/http`、`os`、`testing`、`time`。）

- [ ] **Step 2: 运行确认 SKIP（本环境无凭据）**

```bash
go test ./internal/modules/appconnector/ -run 'TestFeishuRealPublishLoop' -count=1 -v 2>&1 | tail -4
```

Expected: `--- SKIP` + skip 理由文案。**SKIP 是该验收项在本环境的诚实终态，不是通过。**

- [ ] **Step 3: 确认编译与 vet**

```bash
go vet ./internal/modules/appconnector/
```

Expected: 无输出（成功）。

- [ ] **Step 4: Commit**

```bash
git add internal/modules/appconnector/feishu_publish_real_test.go
git commit -m "test(appconnector): 飞书真实发布闭环证据（FEISHU_* 门控，本环境 blocked-env 如实 SKIP）"
```

---

### Task 7: 计划级验证收尾

**Files:**
- 无新文件（全量验证 + 收尾提交）

**Interfaces:**
- Consumes: Task 0–6 全部产出。
- Produces: 本计划 `testCommand` 的全绿记录。

- [ ] **Step 1: 全量验证**

```bash
go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/
go test ./internal/database/ -count=1
go test ./internal/modules/appconnector/... -count=1
go test ./internal/handler/ -run 'FeishuPublish|NotionPublish|TestAppPublicationsTableExists' -count=1
go test ./internal/application/repository/ -run 'MobileDevice|MobilePush' -count=1
go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlindStripsKind|TestHTTPNotificationProviderBlindOmitsKind|TestAppRoutingProviderDispatchesByApp|TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm|TestAppRoutingProviderRevokesOnlyOwnAppRegistration' -count=1
```

Expected: 全部 `ok`；`go vet` 无输出。

- [ ] **Step 2: 重复提交兜底**

```bash
git status --short
```

若仍有本计划范围内的未提交文件，按归属补提交（消息格式沿用前六个任务的 conventional 前缀）。

---

## Consumes / Produces 汇总

**Consumes（#48 已集成接口，本计划实测核实于 HEAD）：**
- `publish.NotionPublishService`（`FormPlan/PublishPlanInput/PublishPlanView/PublishExecuteOutcome/PublishReceiptView`，`publish/plan.go:104-102`）
- `publish.NewNotionBridge/NewDBNotionScopeSource/NewConstantNotionPolicyProvider/NewCredentialTokenSource` 与端口类型 `NotionScopeSource/NotionPolicyProvider/NotionTokenSource/PublicationSource`（`publish/dispatcher.go:39-84`）
- `repoappconn.PublicationStore/PublicationRow` 与状态常量（`repository/appconnector/publication.go:26-66`）
- `appconnectorsvc.ActionService.NewActionService(store, guard, gate, dispatcher, resolver)`、`ActionSnapshot`、`DispatchOutcome`、`ErrDispatchNotStarted/ErrDispatchUnknown`、`A02Guard`、`ExecutionGate`、`ConnectionCredentialSource`
- `appconn.Adapter/Action/ActionResult/HTTPPolicy/NormalizeArgs/ActionFailed/ActionUnknown/ActionSucceeded/ActionAwaitingApproval/RiskWrite/CanDriveActionWrites/ConnectionKindPersonal/FeishuAPIHost`
- 迁移 `000194_app_publications`（versioned）/`000115_app_publications`（sqlite）

**Produces（后续计划消费）：**
- **#50（Confluence 发布）**：`publish.ProviderProfile` + `publish.NewProviderPublishService` —— 第三个 provider 只需实现自己的 Adapter 域 + profile + 桥 + handler 镜像；服务层零改动。
- **#51（多文档发布/单操作计划记录扩展）**：`app_publications` 的 `provider='feishu'` 行为面（`planned→published/failed/unknown` 状态机、`progress_json` FE-03 checkpoint）。
- **发布面横向**：`/api/v1/apps/feishu-publish` wire 合同（`FEISHU_PUBLISH_REVISION_CONFLICT` 409 语义）与飞书 docx Adapter 域（`internal/modules/appconnector/feishu_docx.go` 全部导出）。

## 计划差异记录（调查结论 vs 代码现状，以代码为准）

1. **迁移撞号差异**：调查摘要按波级问题 1 提示"若需要迁移装载须纳入 Task 0，并与既有重编历史核对可用序号"。实测 HEAD：`go test ./internal/database/` FAIL（`duplicate migration file: 000114_public_agent_marketplace.down.sql`）；#48 自身的 E2E `TestAppPublicationsTableExistsAfterMigrations` 同因 FAIL。本计划 E2E 硬依赖全量迁移轨道，故 Task 0 执行重编。与 t43/t60 先例（"无测试钉住旧文件名"）不同，本次有 5 个测试文件钉住 `000114/000193_mobile_device_app` 文件名（逐一列出于 Task 0 Files），必须同步更新。撰写期间 worktree 已存在同目标的未提交重编（000118/000197），Task 0 按三态前置条件处理。
2. **飞书 OAuth 连接体系差异**：调查摘要称"飞书文档读写权限分离无代码；仅有 Notion 侧先例模式"。实际代码：feishu 已在 `appOAuthDefaults` 第一批（`app_connector_oauth.go:49`），appconnector 体系已有 feishu 安装/连接/OAuth user token 与 `CredentialResolver` 解密通道——发布连接直接复用该体系；datasource feishu connector（`internal/modules/datasource/connector/feishu/core/client.go`）是独立的知识导入体系（tenant token），本计划零改动、零写路径（AC2 的"读取/同步"面）。
3. **AC2 的数据面差异**：`dbNotionScopeSource.NotionScope` 只把 `NotionCapabilityInsert` 投影为 `InsertCapability`，不透传原始 scopes。为不改变 #48 行为，`NotionConnectionScope` 增加 `Scopes []string` 透传字段（`dispatcher.go` +2 行），飞书写能力门（`write_docx`）读它。
4. **飞书创建合同无 title**：官方 SDK `CreateDocumentReqBody` 仅 `folder_token`（`model.go:7895-7896`）。批准的 title 只作审批绑定与台账元数据，Adapter 不发送、不比对远端标题；**未采用**未经 SDK 锚定的 `PATCH /drive/v1/files/:file_token` 重命名 API（SDK v3.9.7 未封装、官方文档页无法核实）——这是与 Notion 的显式行为差异，留给后续经核实再补。
5. **Query 比对语义升级**：#48 Notion 的 Query 用整块字节比对（`notion_create.go:601-607`），真实 provider 返回块带服务端字段时会退化为 unknown。飞书侧按语义比对实现（`feishuDocBlockContents` content 序列），并在 Review Focus 5 记录；Notion 侧不在本计划范围内改动（其 E2E 用替身记录原样块故仍绿）。
6. **部分成功恢复的诚实语义**：飞书镜像 #48——checkpoint 只在 append 成功返回后推进，因此"响应丢失的最后一批"在恢复时会重发一次（远端内容重复）；Query 的连续段包含核对仍能如实收敛为成功。测试如实记录该重复（`TestFeishuDocxAdapterCreateResumesFromCheckpoint` 断言 6 块 = 5 批准 + 1 重复尾），不掩盖。
7. **服务类型名**：provider 中立化后服务类型沿用 `NotionPublishService` 历史名（改名会翻动 #48 全部调用面，违背并行批次最小改动约束）；provider 绑定点是 `ProviderProfile`，类型名的 provider 残留已在代码注释声明。
8. **AC1 预读在飞书 create 上的落点**：Notion create 的 baseline 预读读 parent page 版本；飞书的 create destination 是审核过的 folder，无 docx revision 可读。处理：adapter 层把「目标不存在」（provider code 99991661 / HTTP 404）类型化为 `ErrFeishuPublishNotFound`（Task 1/2），桥把该形态翻译为空 baseline（`feishu.go` ReadPageVersion），共享层 FormPlan 接受 create 的空 baseline、仍强制 update 的 live 预读（Task 3 ③，含安全性论证）。update 目标被外部删除的场景：预读 not-found → 计划拒绝；若在审批后被删，execute 预读失败 → 终局 failed——两条路都 fail closed。
9. **部分成功恢复的尾批重复**：checkpoint 只在 append 成功返回后推进，「响应丢失的最后一批」恢复时会重发一次（远端重复一块）。测试如实断言（`TestFeishuDocxAdapterCreateResumesFromCheckpoint`：6 块 = 5 批准 + 1 重复尾），Query 的连续段包含核对仍收敛 succeeded——与 #48 NO-03 语义逐点对齐，不掩盖。

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：Issue 正文一句（复用 Publication seam 完成飞书创建/更新、版本读取、确定内容审批、部分成功核对和回执）→ Task 1（合同/快照/版本检测）、Task 2（Adapter 创建/更新/部分成功/核对）、Task 3（审批前的 A03 计划与发布记录复用）、Task 5（回执 E2E）。验收 1「飞书差异只存在于 Adapter」→ ProviderProfile 泛化 + `TestPublishServiceProfilesAreSymmetric`（Task 3）+ Task 7 共享服务层零 provider 分支。验收 2「读取/同步权限不会自动升级为写权限」→ `FeishuCapabilityWriteDocx` 能力门（Task 2/3）+ `TestFeishuPublishEndToEndReadOnlyScopeCannotPublish`（Task 5）+ datasource connector 零改动（差异记录 2）。验收 3「最高稳定 Interface」→ Task 5 全迁移 E2E + Task 6 blocked-env 如实 SKIP。任务清单 3 条 checkbox 全部有落点。无缺口。
2. **占位符扫描**：全文无 TBD/TODO/"实现细节略"/"与 Task N 相同"（Task 2 的测试注释中"the notion double pins"是语义参照而非省略代码）；全部代码块为完整可编译内容；唯二需执行者现场决定的点（Task 0 的三态前置条件、Task 5 的 import 清理提示）均已显式写出判定条件。
3. **类型/签名一致性**：`ProviderProfile` 字段在 Task 3 Interfaces、`provider.go` 实现、`NotionProfile/FeishuProfile` 与 `plan.go` 使用处三处逐字一致；`FeishuDocxAdapter` 字段在 Task 2 Interfaces、实现、Task 3 桥装配处一致；`ParseReceipt` 返回 `(externalID, externalVersion string, err error)` 在 profile 定义、`project()` 解构（`rcptExternalID, rcptExternalVersion, rerr`）、测试投影处一致；`FeishuVersionConflictResult = "feishu_docx_revision_conflict"` 与 `ErrFeishuPublishRevisionConflict` 哨兵串同值（run() 的 errors.Is 映射依赖此配对，Task 2/3 各自定义处已对齐）；`FeishuCapabilityWriteDocx = "write_docx"` 在 schema_json 种子、caps 门、测试断言三处一致；`ErrFeishuPublishNotFound` 的产生（Task 1 ParseFeishuDocumentVersion code 99991661 / Task 2 do() 404）、翻译（Task 3 feishu.go ReadPageVersion → 空 baseline）、消费（Task 3 plan.go 预读判定 + provider_test 的 feishu 空 baseline 断言 + E2E create 断言）四处闭环。
4. **Review Focus 落实**：五条逐一有测试（只读 scope→Task 2 Step 1 第 6 测 + Task 5 Test 4；数字 revision→Task 1 `TestParseFeishuDocumentVersionReadsNumericRevision` + Task 2 fake 数字 revision；分页→Task 2 `TestFeishuDocxAdapterQueryFollowsPagination`；无 title 合同→Task 1 注释 + Task 5 Test 1 的"title was NEVER sent"断言；语义比对→Task 2 `TestFeishuDocxAdapterQueryReconcilesByContent`）。

## 计划级验证命令（testCommand）

```bash
go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/database/ -count=1 && go test ./internal/modules/appconnector/... -count=1 && go test ./internal/handler/ -run 'FeishuPublish|NotionPublish|TestAppPublicationsTableExists' -count=1 && go test ./internal/application/repository/ -run 'MobileDevice|MobilePush' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlindStripsKind|TestHTTPNotificationProviderBlindOmitsKind|TestAppRoutingProviderDispatchesByApp|TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm|TestAppRoutingProviderRevokesOnlyOwnAppRegistration' -count=1
```

（在 worktree 根执行；`FEISHU_*`/`NOTION_TOKEN` 等真实凭据未设置时真实 Provider 测试 SKIP，属预期。）
