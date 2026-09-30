# OCR 增量修复批次 5·三轮（issue30-sweep）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 OCR B5 三轮增量扫描（R5 报告，13 findings）的全部 10 项主发现，按根因聚合为 8 个任务（同根因合并、同文件串行），3 项 lowWorth 按文件/域就近折叠（R5-F4/F10 折叠进同文件任务、R5-F12 无同域宿主聚合为一个最小收尾任务），不单独开流。

**Architecture:** 全部发现集中在 Go 侧三个域：Confluence 适配器（wire 契约、截断、对账语义）、GitLab codedelivery（MR 解析维度、EnsureBranch 锚定与错误语义、截断）、agent 治理（升级建议生命周期、许可登记审计、publish 服务重复）。跨模块同根因合并：R5-F6+F9（LimitReader 静默截断）为一个任务；R5-F11+F13（同函数 EnsureBranch）为一个任务。所有修复维持既有架构约束：settleOutcome「只有可证未出网（ErrDispatchNotStarted）才允许落 ActionFailed」契约、诚实回执（不冒充、不静默）、Query 对账不伪造终态。行号以各任务 Step 前实读为准（本计划作者已对 13 项锚点逐一实读复核，差异见「差异记录」）。

**Tech Stack:** Go（`internal/`，标准 go test）。测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行。本计划作者已实跑下列基线（2026-09-28，当前 HEAD `fbb0f850a`）：

- `go build ./...` → **exit 0**（绿，仅 ld 重复库警告）。
- `go test ./internal/modules/appconnector/ -run "TestConfluenceCreate" -count=1`（ask 指定检查，原样复跑）→ **4 项 FAIL**：`TestConfluenceCreateCloudHappyPath` / `TestConfluenceCreateCloudWikiContextPath` / `TestConfluenceCreateUnknownOnLostCreateReply` / `TestConfluenceCreateWireUsesOfficialCamelCaseKeys`，错误均为 `confluence_provider_error: status=400`——工作区未提交的 RED 测试现场（R5-F5 缺陷复现，见差异记录 2），即 Task 1 的 Step 1 已在工作区。
- `go test ./internal/modules/appconnector/... ./internal/modules/codedelivery/` → **codedelivery ok；appconnector 根包仅上述 4 项 FAIL；其余子包（connectorcontrol/openconnector/plan/publish/repository/appconnector/service/appconnector）全 ok**。
- `go test ./internal/application/...` → **service 及 file 子包 ok；`internal/application/repository` 3 项预存在失败**（`TestDeliveryCollaborationEndToEnd{AC1,AC2,CrossTenantIsolated}`，失败用例名与前序批次差异记录一致，域外预存在；该包全量运行 252s，错误文本本轮未抽取）。

**Spec:**
- 发现来源：B5 三轮增量发现清单（本计划的 ask 材料；编号 R5-F1…R5-F13；本计划作者对 10 项主发现与 3 项 lowWorth 的代码锚点逐一实读复核，行号以复核为准，差异见「差异记录」）
- 报告原文：`docs/plans/issue30-sweep/ocr/ocr-increment-b5.md`（三轮重扫版，"Review partially complete: 13 finding(s)"）
- 领域术语与不变量：`CONTEXT.md`
- 前序批次：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b4.md`、本路径一轮版（git `c9493b49c`，其 Task 1-5 已落地为 HEAD 的多个 fix 提交）
- Parent：Issue #30（issue30-sweep）

## Global Constraints

以下为批准 Spec / ADR / 报告隐含的项目级约束，所有任务隐含遵守：

- **settleOutcome 契约**（`internal/modules/appconnector/service/appconnector/action.go:576-582` 实读）：只有 `ErrDispatchNotStarted`（可证未出网）允许落 `ActionFailed`，其余 dispatch 错误一律 `ActionUnknown`。R5-F13 的正确修复是把「commits API 调用之前的本地可证拒绝」包进 `ErrDispatchNotStarted`，**不是**放宽该契约。
- **HTTP 响应截断必须报错而非返回**：LimitReader 上限是防失控，不是静默截断（R5-F6/F9 同根因）；上限取值必须覆盖上游合法最大载荷（Confluence storage 回显经 EscapeString 放大 4-6 倍、GitLab 单 blob 最大 16MB），否则「超限报错」会误伤合法响应。
- **官方 wire 契约为准**：Confluence Cloud REST v2 请求体为 camelCase（spaceId/parentId；同包响应侧 `confluence_common.go:149` 已按 `json:"spaceId"` 解码，互为佐证）；测试 fake 必须按官方契约解码，不得原样回显生产键名（否则掩盖面再次发生——R5-F5 正是被 snake_case 双打掩盖）。
- **Query 对账不伪造终态**：对账比较归一化的目的是消除服务端重序列化噪声，归一后仍不可比必须维持 unverifiable（诚实优先于收敛，R5-F7）；凭远端事实解析时必须核对全部维度（source+target，R5-F8），不得凭部分匹配判 delivered。
- **值传递 store 不得回显未落库状态**：upsert 冲突路径返回输入副本即虚报（R5-F3 与一轮批次 F74/F75 同族纪律）——重读库中行再返回。
- **Mimosa 安全约束**（会话注入）：服务端发请求仅 http/https、SQL 一律参数绑定（GORM 既有惯例维持）、凭据只从环境变量读取。
- 严格 RED→GREEN→REFACTOR：每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；实现与已批准 Spec 冲突时升级处理。
- **失败集合不扩大**：基线预存在失败（`internal/application/repository` 3 项 delivery e2e）不得因本批次新增；工作区 4 项 ConfluenceCreate RED 属 Task 1 收编对象（Task 1 完成后转绿）。

## Review Focus

报告隐含但任务测试需钉住、最可能咬到真实用户的五类失效模式（每行后标注 owning 任务）：

1. **真实外部系统首次调用即失败**——Confluence Cloud v2 创建请求键名与官方契约不符，T20 的 Confluence Cloud 版本整体不可用（所有创建 400 → ActionFailed）；测试双打按同构错误键校验掩盖了它。——Task 1 测试（官方契约 fake 下创建成功 + 请求体键名断言 camelCase）。
2. **静默数据损坏**——HTTP 响应被 LimitReader 截断后以不完整字节成功返回：GitLab 8-16MB 基线 blob 截断内容写入会话工作区随交付发出（基线损坏且无 sha 校验兜底）；Confluence 数 MB storage 回显截断 → JSON 解析失败 → 已成功的发布永久 unknown。——Task 2 测试（超限必须报错；合法大载荷不误伤；blob 内容与 sha 不符必须报错）。
3. **批准消耗后交付永久滞留 unknown 无自愈**——本地可证未出网的拒绝（空收敛/超上限）因不满足 ErrDispatchNotStarted 契约落 unknown，QueryProvider 查不到任何远端事实，永远无法收敛；Confluence Query 对 storage 做字节精确比较，服务端重序列化后含引号正文永不可比。——Task 5/3 测试（本地拒绝落 failed；实体还原后对账收敛，真实漂移仍拒收）。
4. **凭部分匹配误判终态**——GitLab 同一 source_branch 可对多个 target_branch 存在开放 MR，凭 head 单维度命中即把 unknown 误判 delivered（错误终态无法自愈，写错误回执）。——Task 4 测试（无关 target 的 MR 不命中；>20 条翻页不漏检）。
5. **审计与生命周期失真**——re-adopt 前进后过期 open 建议永久残留且可 Accept（建冗余草稿 Variant）；许可重复登记永久丢失首次登记人且 API 回显与库中行不符；已物化建议每次读请求全量重算 diff（读路径 O(N) 大 JSON 解码）。——Task 6/7 测试（stale open 迁终态 + Accept 拒收；重登记保留首登人 + 返回值与库一致；已物化组合零重算）。

---

## 任务结构与文件地图

| # | 任务 | 根因分组（发现编号） | 主要文件 | 优先级 |
|---|---|---|---|---|
| 1 | Confluence Cloud v2 创建 wire 契约 | R5-F5 | `internal/modules/appconnector/confluence_create.go`、`confluence_create_test.go`（工作区 RED 已在） | **critical** |
| 2 | HTTP 响应截断检测（跨模块同根因） | R5-F6 + R5-F9（同根因） | `confluence_create.go`、`codedelivery/gitlab_client.go` | **high** |
| 3 | Confluence Query storage 归一对账 | R5-F7 | `confluence_update.go` | **high** |
| 4 | GitLab MR 解析 target 维度与翻页 | R5-F8；顺手 R5-F10 | `codedelivery/gitlab_client.go` | medium |
| 5 | GitLab EnsureBranch 基线锚定与可证拒绝 | R5-F11 + R5-F13（同函数） | `codedelivery/gitlab_client.go`、`dispatcher.go` | medium |
| 6 | agent 升级建议 reconcile 生命周期 | R5-F1 + R5-F2；顺手 R5-F4 | `application/service/agent_upgrade.go` | medium |
| 7 | marketplace 许可登记审计保真 | R5-F3 | `application/repository/agent_marketplace_lineage.go` | medium |
| 8 | ConfluencePublishService 收敛 ProviderProfile | lowWorth R5-F12（无同域宿主，最小收尾） | `appconnector/publish/confluence.go`、`container/confluence_publish.go` | low |

**发现覆盖对照**（ask 材料 10 项主发现 + 3 项 lowWorth 逐项对账）：

| 发现 | 任务 | 处置 |
|---|---|---|
| F5（critical） | Task 1 | 修复（wire 键 snake_case → 官方 camelCase；工作区 RED 收编） |
| F6（high） | Task 2 | 修复（与 F9 同根因合并：超限报错 + 上限覆盖合法载荷） |
| F9（medium） | Task 2 | 修复（+ Blob sha 校验兜底） |
| F7（high） | Task 3 | 修复（storage 归一化比较，真实漂移仍拒收） |
| F8（medium） | Task 4 | 修复（target_branch 维度 + 翻页；顺手 F10 解码错误分类） |
| F11（medium） | Task 5 | 修复（新分支锚定基线，与 F13 同函数合并） |
| F13（medium） | Task 5 | 修复（本地可证拒绝包 ErrDispatchNotStarted，落 failed 可重开计划） |
| F1（performance/medium） | Task 6 | 修复（已物化组合存在性短路，与 F2 同文件合并） |
| F2（medium） | Task 6 | 修复（stale open 迁终态 + Accept 校验 FromReleaseID） |
| F3（medium） | Task 7 | 修复（DoUpdates 去 created_by + 冲突后重读返回） |
| F4（lowWorth） | Task 6 | 顺手：移除 now 死字段（同文件） |
| F10（lowWorth） | Task 4 | 顺手：2xx 解码失败不再归类 ErrCodeRequestInvalid（同文件） |
| F12（lowWorth） | Task 8 | 收尾：Confluence publish 服务体收敛到 ProviderProfile（feishu 同型先例） |

**执行顺序**：单 Go 流，按编号线性执行即可满足同文件串行约束——Task 1 → 2（共享 `confluence_create.go`）；Task 2 → 4 → 5（共享 `gitlab_client.go`）；Task 3 独立文件可穿插；Task 6/7/8 互相独立。Task 8 建议最后（行为不变重构，以全绿为准入）。

## 差异记录（报告/ask 材料 vs 代码现状，以代码现状为准）

1. **本路径滚动重写记录（重要）**：本文件 HEAD 版（git `c9493b49c`）为 B5 一轮版计划（B5-F*，39 项），其 Task 1-5 已执行落地为 HEAD 的多个 fix 提交（migrations 去重、created_at 回显、store 错误分类、输入边界、publish 家族归属）。本计划写入前，工作区存在一份**未提交的 B5 二轮版计划**（B5I-F*，36 主 + 29 lowWorth），其证据源（报告 71-findings 版）已被三轮重扫覆盖为现行 13-findings 版（报告文件 `ocr/ocr-increment-b5.md` 为未跟踪文件，本轮材料即出自它）。本版按三轮材料（R5-F*）重写同路径——与该流程既定滚动惯例一致（一轮版在 git 历史可回溯）。两点如实声明：(a) B5I 版未提交，本版覆盖后**无法从 git 恢复**；其与本批重叠的发现（confluence wire 契约、双端截断、EnsureBranch 锚定与错误语义、UpsertLicense 回显、now 死码、单读 reconcile）已以本轮更生鲜的证据链重新聚合入本计划对应任务；(b) B5I 版**独有且未执行**的发现（TS 流 voice-room/research/iOS 全部、Go 侧血缘 fail-open/licenses 路由权限/plan 服务审批纪律等）不在本批 13 findings 范围内，本计划不携带其证据，如需处置应由编排方重新开流（其报告证据已被覆盖，需重扫）。
2. **工作区 RED 现场收编（Task 1 直接受益）**：`internal/modules/appconnector/confluence_create_test.go` 存在未提交修改（+49/-4）——测试 fake 已改为仅认官方 camelCase 键并新增 `TestConfluenceCreateWireUsesOfficialCamelCaseKeys`，4 项 TestConfluenceCreate* 处于 RED（本计划作者原样复跑 ask 指定命令确认 4 FAIL，见 Tech Stack 基线）。**Task 1 的 Step 1 已在工作区，执行者不重写**，按其 RED 续完 GREEN；commit 时将测试文件与生产改动同任务提交。基线中 appconnector 根包的 4 项 FAIL 均源于此现场，非 HEAD 缺陷暴露面（HEAD 的测试双打按 snake_case 校验故自洽绿灯——掩盖面正是 R5-F5 的成因）。
3. **R5-F2 的函数位置修正**：ask 证据写「agent_adoption.go:111-124 reconcileAdoptionTx」——实读该函数在 **repository** 层 `internal/application/repository/agent_adoption.go:112-119`（`adoptListingTx` :76-105 在冲突路径调用它；`if existing.AcceptedReleaseID == acceptedReleaseID` 跳过、`existing.AcceptedReleaseID = acceptedReleaseID` last-write-wins 前进），service 层 `agent_adoption.go` 无此函数（入口为 `Adopt` :61-95 → `repo.AdoptListing`）。结论不变：re-adopt 前进 AcceptedReleaseID 而 open 建议不随之终态化。
4. **R5-F6 的文件路径补全**：ask 证据写「confluence_blocks.go:45」——实读文件在 `internal/modules/appconnector/publish/confluence_blocks.go:45`（`html.EscapeString(p)`；行号一致）。`snap.Storage` 的来源确认在 `publish/confluence.go:111`（`ConfluenceStorageBody(string(content))`）。
5. **R5-F12 的证据修正**：ask 写「ConfluencePublishService 与 NotionPublishService 逐行重复约 150 行」——实读 `NotionPublishService` 定义在 `publish/plan.go:108`（方法集 FormPlan/Execute/Reconcile/Receipt/project = :162/:273/:283/:292/:312），`ConfluencePublishService` 在 `publish/confluence.go:41`（文件 234 行，方法集 = :62/:171/:181/:190/:202），两服务五方法完全平行、FormPlan 开头逐行同构（confluence.go:63-67 ≡ plan.go:163-167，仅 scope 名不同）。**修复先例已存在**：`NewProviderPublishService`（plan.go:130-140，注释自认「#50 confluence」预留）+ feishu 装配先例 `container/feishu_publish.go:39`。Task 8 按该先例收敛，重复量实测为 confluence.go 全文件 234 行（服务体 ~150 行与 plan.go 平行）。
6. **R5-F5 的锚点精确化**：ask 写 `:88`——实读 wire 结构体 `confluenceCloudCreateRequest` 的 json tag 在 `confluence_create.go:87-93`（`json:"space_id"` / `json:"parent_id,omitempty"`）。`publish/confluence_bridge_test.go:99-110` 确仍按 snake_case 解码（掩盖面）——Task 1 顺带核对该双打是否需同步官方契约（按其在位断言实读定，不强制）。
7. **基线预存在失败**：`internal/application/repository` 的 3 项 `TestDeliveryCollaborationEndToEnd*` 失败与前序批次差异记录一致（域外预存在，本批任何任务回归命令触及该包时按「失败集合不扩大」记录，不修复）。

---

### Task 1: Confluence Cloud v2 创建 wire 契约（R5-F5）

**Files:**
- Modify: `internal/modules/appconnector/confluence_create.go:87-93`（wire 键名）
- Test: `internal/modules/appconnector/confluence_create_test.go`（工作区已有 RED，差异记录 2——4 项 TestConfluenceCreate* + `TestConfluenceCreateWireUsesOfficialCamelCaseKeys`）

**Interfaces:**
- Consumes: Confluence Cloud REST v2 `POST /api/v2/pages` 官方契约：请求体属性 `spaceId`（必填）/ `parentId`（可选，camelCase）；同包响应侧 `confluence_common.go:149` 已按 `json:"spaceId"` 解码（ask 材料 + 实读互为佐证）。
- Produces: `confluenceCloudCreateRequest` 的 json tag 改为 `json:"spaceId"` / `json:"parentId,omitempty"`。行为面：真实 Cloud 实例创建请求被官方契约接受（不再 400 → definitive provider_error → ActionFailed）；Server/DC 侧 wire 不动。

**根因与修复说明：** Cloud v2 创建请求体键名 `space_id`/`parent_id` 与官方 camelCase 契约不符，真实 Cloud 实例首次调用即 400。既有测试双打按 snake_case 键解码自洽绿灯（掩盖面）；工作区 RED 已把 fake 改为仅认官方键，4 项 FAIL 复现缺陷（ask 指定命令原样复跑确认）。

- [x] **Step 1: 确认工作区 RED 形态（不重写）**

`git status` 确认 `confluence_create_test.go` 未提交修改在场；实读 fake 的键校验（仅认 `spaceId`/`parentId`）与 `TestConfluenceCreateWireUsesOfficialCamelCaseKeys` 断言形态。

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/ -run "TestConfluenceCreate" -count=1`
Expected: FAIL ×4（`status=400`——本计划作者基线已复跑确认）。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 改两个 json tag（一处结构体、两行改动）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/ -run "TestConfluenceCreate" -count=1`
Expected: PASS。
Run: `go test ./internal/modules/appconnector/...`
Expected: 全 ok（基线 4 项 RED 转绿）。

- [x] **Step 5: Commit**

```bash
git add internal/modules/appconnector/confluence_create.go internal/modules/appconnector/confluence_create_test.go
git commit -m "fix(confluence): use official camelCase keys on the Cloud v2 create wire (R5-F5)"
```

---

### Task 2: HTTP 响应截断检测（R5-F6 + R5-F9，跨模块同根因）

**Files:**
- Modify: `internal/modules/appconnector/confluence_create.go:162`（confluenceDo，1MiB）
- Modify: `internal/modules/codedelivery/gitlab_client.go:128`（callRaw，8MB）与 `:223-231`（Blob sha 校验）
- Test: `internal/modules/codedelivery/gitlab_wire_test.go`、`internal/modules/appconnector/confluence_create_test.go`（或同包新增测试文件）

**Interfaces:**
- Consumes: 两处均为 `io.ReadAll(io.LimitReader(resp.Body, N))` 且超限时 ReadAll 成功返回不完整字节、无检测即返回（实读：confluence_create.go:162、gitlab_client.go:128）。上游合法载荷上限：Confluence 侧 `MaxPublishArtifactBytes = 1<<20`（publish/plan.go:26，原文上限）经 `ConfluenceStorageBody`→`html.EscapeString`（publish/confluence_blocks.go:45，1 字节→5 字节实体）最坏放大 ~5MiB；GitLab 侧 `maxBaselineBytes = 16<<20`（codedelivery/service.go:21，MaterializeBaseline :124-135 逐文件 Blob 且仅查累计 total）允许单 blob 最大 ~16MB。既有错误家族：Confluence `ErrConfluenceOutcomeUnknown`（confluence_create.go:161-164 read 失败分支先例）；GitLab `ErrCodeTransport`。
- Produces: (a) 两处同型截断检测：读 `cap+1` 字节，`len(raw) > cap` 返回错误（含 method/path/cap），绝不返回截断字节；(b) 上限量级修正以覆盖上游合法载荷——Confluence 侧 `maxConfluenceBodyBytes = 8 << 20`（覆盖 ~5MiB 最坏放大），GitLab 侧 raw cap 提至 `16 << 20`（对齐 maxBaselineBytes 单 blob 上界；常量就地命名，与上游常量的对齐关系写注释互指）；(c) GitLab `Blob()` 返回前按内容寻址校验：本地计算 git blob sha（`sha1("blob %d\x00" + content)`，标准 git blob 前缀格式）与请求 sha 不符即返回 `ErrCodeTransport` 包装错误——截断/错读的第二层兜底（CreateBlob :235 附近已按同型 sha 生成内容寻址键，实读对齐算法）。

**根因与修复说明：** 上限本意是防失控响应，静默截断把它变成两处数据损坏。F6：create/update 回显与 Query 对账读（`body-format=storage` 携带全文，confluence_create.go:176-178）一旦超 1MiB 即截断 → JSON 解析失败 → 已成功的发布被判 ActionUnknown 且 Query 永远无法收敛；F9：`maxBaselineBytes=16MB` 允许单 blob 8-16MB，callRaw 8MB 截断无错——MaterializeBaseline 把截断基线写入会话工作区（静默基线损坏随交付发出）、EnsureBranch 兜底取回（gitlab_client.go:325-341）把截断 blob base64 提交进任务分支，且 Blob 无 sha 校验兜底。

- [x] **Step 1: 写失败测试**

```go
// gitlab_wire_test.go（或同包既有 fake 宿主）
func TestGitLabRawRejectsBodyOverCap(t *testing.T) {
	// arrange: fake 返回 16MiB+1 字节、HTTP 200
	// act: callRaw → assert: 错误（含 cap/exceeds 字样），绝不返回截断字节
}
func TestGitLabBlobVerifiesContentAddressedSHA(t *testing.T) {
	// arrange: fake 对某 sha 返回内容与该 sha 不符的字节
	// act: Blob → assert: 错误（内容寻址不符）
	// 负例: 内容与 sha 相符 → 通过
}
// confluence 侧（confluence_create_test.go 追加）
func TestConfluenceDoRejectsBodyOverCap(t *testing.T) {
	// arrange: fake 返回 cap+1 字节 → assert: ErrConfluenceOutcomeUnknown 包装的错误
}
func TestConfluenceDoAcceptsMultiMegabyteStorageEcho(t *testing.T) {
	// arrange: fake 返回 ~3MiB 合法 JSON 回显 → assert: 正常解码（新上限不误伤）
}
```

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'TestGitLabRawRejects|TestGitLabBlobVerifies'` 与 `go test ./internal/modules/appconnector/ -run 'TestConfluenceDoRejects|TestConfluenceDoAccepts'`
Expected: FAIL ×4——当前返回截断字节、无检测；Blob 无校验。

- [x] **Step 3: 最小实现**

```go
raw, err := io.ReadAll(io.LimitReader(resp.Body, cap+1))
if err != nil { /* 既有 read 错误分支 */ }
if len(raw) > cap { return /* 各自家族错误，含 method/path/cap */ }
```

（两处同型；Blob 的 sha 校验按 Produces (c)，算法与 CreateBlob 的生成侧实读对齐。）

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/codedelivery/ ./internal/modules/appconnector/...`
Expected: PASS（两域基线全绿维持，含 Task 1 转绿后的 appconnector）。

- [x] **Step 5: Commit**

```bash
git add internal/modules/appconnector/confluence_create.go internal/modules/codedelivery/gitlab_client.go internal/modules/codedelivery/gitlab_wire_test.go
git commit -m "fix(http): detect cap-exceeding responses and verify blob content addressing instead of silently truncating (R5-F6, R5-F9)"
```

---

### Task 3: Confluence Query storage 归一对账（R5-F7）

**Files:**
- Modify: `internal/modules/appconnector/confluence_update.go:287`（storage 字节比较）
- Test: `internal/modules/appconnector/confluence_update_test.go`

**Interfaces:**
- Consumes: Query 三重全等（:284-289：`cur.VersionNumber == expected+1` 先决、`cur.Title != snap.Title || cur.BodyStorage != snap.Storage` 字节精确比较 → 不等即 `confluence_query_unverifiable` 落 ActionUnknown）；`snap.Storage` 由 FormPlan 经 `ConfluenceStorageBody` 生成（publish/confluence.go:111；EscapeString 产出 `&#39;`/`&#34;` 实体）；Confluence 服务端对 storage XHTML 重序列化（实体还原、空白归一）为已知平台行为（ask 材料声明 + 同族 `confluence_blocks_test.go` 的实体化测试佐证）。
- Produces: storage 比较前对两侧（`cur.BodyStorage` 与 `snap.Storage`）施加**同一**归一化函数：`html.UnescapeString`（实体还原）+ CRLF→LF 归一 + `strings.TrimSpace`；title 比较同步 TrimSpace（title 无实体语义，保守处理）。归一后相等 → 比较通过；仍不等 → 维持 unverifiable（诚实优先：服务端未知归一形态不得伪造收敛）。归一函数放 `confluence_update.go` 就地（或 confluence_common.go，执行者按包内聚拢惯例定），名为 `normalizeStorageXHTML`。

**根因与修复说明：** 对账对 storage 做字节精确相等，而发布侧写入的是「本地转义形态」、服务端读回的是「服务端重序列化形态」——正文含引号/可归一空白时两者永不相等，实际已落地的 PUT 一旦 park unknown，每次对账 unverifiable、action 永久停留 unknown（unknown 行本身永久滞留需人工处置）。归一化只消除**已知**重序列化噪声（实体还原与空白归一），真实内容漂移（多一段文字）仍拒收——不可比时维持 unverifiable 是诚实语义而非缺陷。

- [x] **Step 1: 写失败测试**

```go
// confluence_update_test.go 追加
func TestConfluenceQueryConvergesAfterServerEntityNormalization(t *testing.T) {
	// arrange: snap.Storage 含 &#39;/&#34; 实体与 \r\n；fake Query 回显为实体已还原、空白已归一的等价正文，version=expected+1
	// act: Query → assert: ActionSucceeded（不再 unverifiable）
}
func TestConfluenceQueryStillRejectsRealContentDrift(t *testing.T) {
	// arrange: 回显正文多一段真实文字 → assert: 仍 ActionUnknown（confluence_query_unverifiable）
}
```

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'TestConfluenceQueryConvergesAfterServer|TestConfluenceQueryStillRejects'`
Expected: 第一项 FAIL（当前字节比较永假）、第二项 PASS（先跑出基线语义）。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 落地 `normalizeStorageXHTML` 并替换 :287 的比较操作数（title/storage 各自归一后比较；版本号先决检查不动）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/ -run 'Confluence'` 与 `go test ./internal/modules/appconnector/...`
Expected: PASS。

- [x] **Step 5: Commit**

```bash
git add internal/modules/appconnector/confluence_update.go internal/modules/appconnector/confluence_update_test.go
git commit -m "fix(confluence): normalize server-reserialized storage before query comparison (R5-F7)"
```

---

### Task 4: GitLab MR 解析 target 维度与翻页（R5-F8；顺手 R5-F10）

**Files:**
- Modify: `internal/modules/codedelivery/gitlab_client.go:433-452`（PullRequestForHead）
- Modify: `internal/modules/codedelivery/gitlab_client.go:109-111`（call 2xx 解码分类，顺手 F10）
- Modify: 调用点同步——`DraftPullRequest`（:399-422 两处复用）与 `dispatcher.go:273`（QueryProvider）
- Test: `internal/modules/codedelivery/gitlab_wire_test.go`、`service_gitlab_test.go`（dispatcher 级）

**Interfaces:**
- Consumes: GitLab MR list API（`/merge_requests`）支持 `target_branch` 查询参数与 `page`/`per_page` 分页；`PullRequestForHead(ctx, head)` 现状（实读 :433-452）：`source_branch`+`state=opened`+`per_page=20`、返回首条 opened（响应结构体无 target_branch 字段、无翻页）；`DraftPullRequest` 首查与 409 兜底两处复用（:402-405/:417-422）；`QueryProvider` 凭任意 head 命中 MR 即 `TransitionState(DeliveryDelivered)`（dispatcher.go:273-283）；翻页先例 `Tree`（gitlab_client.go:183 `page <= 50`×per_page=100）。
- Produces: (a) `PullRequestForHead(ctx context.Context, head, base string)`（签名增 base 维度；接口定义在 codedelivery 的 GitHub 链同型接口处同步——执行者实读 `PullRequestForHead` 的接口声明位置同步改）：请求 `q.Set("target_branch", base)` + 响应结构体解析 `target_branch` 字段并在循环内**双保险比对**（参数过滤为主、字段比对防御 fake/代理漏过滤）+ `per_page=100` 翻页（短页终止，页数上限 50 对齐 Tree 先例）；无匹配返回 `(nil, nil)` 语义不变；(b) 三个调用点传 base——`DraftPullRequest` 传 `input.Base`（两处）、`QueryProvider` 传 `material.Repo` 的 base（执行者实读 DeliveryMaterial/Repo 的默认分支字段定名，与 GitHub 链 `d.dispatcher` 同源取值对齐）；(c) 顺手 F10——`call()` 2xx 分支 `json.Unmarshal(raw, out)` 失败改包非 `ErrCodeRequestInvalid` 的错误（服务端 2xx 回非法 JSON 是服务端/协议故障，不是客户端 4xx 语义；按包内既有错误族择最近——执行者实读 `ErrCode*` 家族后定，倾向 `ErrCodeTransport`，在 commit message 记录选型）。

**根因与修复说明：** GitLab 平台语义允许同一 source_branch 对不同 target_branch 存在多个开放 MR（与 GitHub head 唯一键不同），当前凭 head 单维度取首条即复用——DraftPullRequest 可能复用指向其他 base 的无关 MR 写错误回执；QueryProvider 凭无关 MR 把 unknown 误判 delivered（错误终态无法自愈）；`per_page=20` 无翻页使 >20 条时目标 MR 漏检。F10 是同文件错误分类纪律：2xx 解码失败被归类 ErrCodeRequestInvalid（4xx 误导）。

- [x] **Step 1: 写失败测试**

```go
// gitlab_wire_test.go 追加
func TestPullRequestForHeadMatchesTargetBranch(t *testing.T) {
	// arrange: fake 对同一 source_branch 返回两个 opened MR（target=main / target=release）
	// act: PullRequestForHead(head, "main") → assert: 命中 target=main 那条
	// act: PullRequestForHead(head, "feature-x") → assert: (nil, nil)（不误命中）
}
func TestPullRequestForHeadPaginates(t *testing.T) {
	// arrange: fake 分页返回 100+1 条（第二页短页含目标）→ assert: 翻页命中
}
func TestCallClassifies2xxDecodeFailureAsServerFault(t *testing.T) {
	// arrange: fake 返回 200 + 非法 JSON
	// act: call → assert: !errors.Is(err, ErrCodeRequestInvalid)
}
// service_gitlab_test.go（dispatcher 级）
func TestQueryProviderIgnoresForeignTargetMR(t *testing.T) {
	// arrange: 仅存在同 source、target=其他 base 的开放 MR
	// act: QueryProvider → assert: 仍 ErrDispatchUnknown（不 TransitionState delivered、不写回执）
}
```

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'PullRequestForHead|CallClassifies|QueryProviderIgnores'`
Expected: FAIL ×4——当前无 target 维度、无翻页、解码失败误分类、无关 MR 误判 delivered。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 落地（签名变更波及面：grep 全部调用点同步，接口声明处一并改）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/codedelivery/`
Expected: PASS（含 GitHub 链既有用例——GitHub 客户端同型接口若受签名影响，按其现有形态适配并保持行为不变）。

- [x] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/
git commit -m "fix(gitlab): resolve MRs by source+target with pagination, classify 2xx decode failures honestly (R5-F8; F10)"
```

---

### Task 5: GitLab EnsureBranch 基线锚定与可证拒绝（R5-F11 + R5-F13，同函数）

**Files:**
- Modify: `internal/modules/codedelivery/gitlab_client.go:289-312`（新分支锚定）
- Modify: `internal/modules/codedelivery/gitlab_client.go:347-363`（空收敛/超限错误语义）
- Modify: `internal/modules/codedelivery/dispatcher.go:184-186`（错误上抛路径，按需）
- Test: `internal/modules/codedelivery/gitlab_wire_test.go`、`service_dispatch_test.go`

**Interfaces:**
- Consumes: EnsureBranch 现状（实读 :289-312）：`intended = Tree(c.stagedBase) + staged entries`（:293-303）、`startRef := base`（默认分支，:308-309）、`current = Tree(startRef)`、actions = current↔intended 全量差集；cap `2*MaxDeliveryFiles`（:353-354；delivery.go:46 `MaxDeliveryFiles = 500` → 上限 1000）；`if !exists { body["start_branch"] = base }`（:361-363）；空 actions 且 !exists 返回裸 `fmt.Errorf("%w: ...", ErrInvalidMaterial, branch)`（:347-351），位于 commits POST（:367）**之前**（可证未出网）；Tree 翻页上限 `page <= 50`×100 = 5000 entries（:183）；dispatcher `:184-186` 原样上抛 `return ..., err`；settleOutcome 契约（action.go:576-582，Global Constraints）；prepare 面仅 `len(changes)==0` 校验（service.go:226-227，workspace≠baseline 而 baseline+changes==default 树时可全通过）；GitHub 链对照先例 dispatcher.go:147-154（`parent := material.BaselineSHA; if exists { parent = head }`）。
- Produces: (a) **锚定**——分支不存在时 `startRef` 取 `c.stagedBase`（基线锚点；执行者实读 client 构造点确认 stagedBase 即 BaselineSHA 形态）而非 `base`；`body["start_branch"]` 同步取基线 ref（GitLab commits API 的 start_branch 接受 branch/tag/commit SHA——执行者以 API 文档/实测确认传 SHA 形态，选型记入 commit message）；actions 收敛为 baseline↔intended 差集（仅 staged 变更 + 本交付真删文件），新分支路径的两次 Tree 拉取（stagedBase 起点时 current 与 intended 同源）合并为一次；「首次迭代=基线即默认 tip」场景行为不变（stagedBase 树 == default 树，差集语义等价）；已有分支路径（startRef=head）不动；(b) **可证拒绝**——EnsureBranch 内发生在 commits API 之前的本地拒绝（空 actions 且 !exists、超 cap、Tree 翻页超限）统一包 `ErrDispatchNotStarted` 上抛（`fmt.Errorf("%w: task branch %s would be empty: %v", appconnectorsvc.ErrDispatchNotStarted, ...)` 形态——错误族归属执行者按 codedelivery 既有 import 实读定，确保 settleOutcome 的 `errors.Is` 可判）；dispatcher :184-186 若已有包装则保持，无则透传（包错误已满足契约）。

**根因与修复说明：** F11：新任务分支从默认分支 `start_branch` 收敛，commit actions 数 = 完整 default↔baseline 增量而非仅 staged 变更——基线锚定较旧 SHA 时超过 cap（1000）或默认分支超 Tree 50 页（5000 entries）即在批准已消耗后以语义误导的 `ErrBaselineTooLarge` 失败（真因是锚定错误不是基线过大），且每次派发两次全量递归树拉取放大延迟与配额；GitHub 链的对照实现锚定 BaselineSHA，GitLab 链偏离。F13：空收敛+新分支路径的裸 `ErrInvalidMaterial` 发生在出网之前、可证未创建任何远端资源，但 settleOutcome 只认 `ErrDispatchNotStarted`——该路径落 ActionUnknown 且分支/MR 均未创建，QueryProvider 永查不到远端事实，批准被消耗后交付永久滞留 unknown 无自愈路径。修复统一为：锚定对齐 GitHub 链语义；本地可证拒绝进 ErrDispatchNotStarted → settle 落 ActionFailed（确定性失败，用户可重开计划，不滞留 unknown）。

- [x] **Step 1: 写失败测试**

```go
// gitlab_wire_test.go 追加
func TestEnsureBranchAnchorsNewBranchAtBaseline(t *testing.T) {
	// arrange: fake GitLab——默认分支 tip 在基线锚定后有新提交（新增文件 X、修改文件 Y）；
	//   stagedEntries 为本交付内容；任务分支不存在
	// act: EnsureBranch
	// assert: commit actions 不含对 X 的 delete、不含把 Y 回卷到基线旧内容的 update
	//   （actions 仅覆盖 staged 变更）；start_branch 为基线 ref 而非默认分支名
}
func TestEnsureBranchEmptyConvergenceIsDispatchNotStarted(t *testing.T) {
	// arrange: 空 actions（intended == 起点树）且分支不存在
	// act+assert: errors.Is(err, ErrDispatchNotStarted)（当前为裸 ErrInvalidMaterial → 红）
}
// service_dispatch_test.go（settle 契约级）
func TestDispatchSettlesFailedOnEnsureBranchLocalRejection(t *testing.T) {
	// arrange: 构造空收敛场景走 dispatcher → assert: action 状态 ActionFailed（非 Unknown）
}
```

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'EnsureBranchAnchors|EnsureBranchEmptyConvergence|DispatchSettlesFailed'`
Expected: FAIL ×3。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 落地（stagedBase 与 BaselineSHA 的关系、start_branch 传 SHA 的可行性以实读+实测定，选型记入 commit message）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/codedelivery/`
Expected: PASS。

- [x] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/
git commit -m "fix(gitlab): anchor new task branches at the baseline and settle local rejections as not-started (R5-F11, R5-F13)"
```

---

### Task 6: agent 升级建议 reconcile 生命周期（R5-F1 + R5-F2；顺手 R5-F4）

**Files:**
- Modify: `internal/application/service/agent_upgrade.go:195-253`（reconcileProposals：存在性短路 + stale 终态迁移）
- Modify: `internal/application/service/agent_upgrade.go:106-117`（Accept 校验 FromReleaseID）
- Modify: `internal/application/service/agent_upgrade.go:33-42`（顺手 F4：移除 now 字段）
- Test: `internal/application/service/agent_upgrade_test.go`

**Interfaces:**
- Consumes: 读路径入口 `ListUpgradeProposals`（:48）与 `GetUpgradeProposal`（:70）均先 `reconcileProposals`；循环链（:205 GetMarketplaceListing〔内嵌 adoption repo 携带 #60 introducedListing 合成回退，repository/agent_adoption.go:324/:336〕→ :216/:223 GetRelease×2 → :230 diffUpgradeBundles〔对两份 release 各做 ManifestJSON+完整 Bundle+DependencyLock 解码，agent_upgrade_diff.go:121-131〕→ :239 json.Marshal → :243 FindOrCreateProposal）；`FindOrCreateProposal` 的存在性检查 `key()` 在 repo 内部（repository/agent_upgrade.go:52-67）、即昂贵 diff **之后**、且仅按 `(tenant, adoption, to_release)` 匹配——已物化 open/dismissed/accepted 组合每次请求全量重算；`ListProposals(tenantID)`（repository/agent_upgrade.go:102-106，一次拉全）；re-adopt 前进：`Adopt`（service/agent_adoption.go:61-95）→ `adoptListingTx` → `reconcileAdoptionTx`（repository/agent_adoption.go:112-119，last-write-wins 前进 AcceptedReleaseID）；`TransitionProposal`（repository/agent_upgrade.go:112-135，CAS 状态迁移）；proposal 终态语义（service/agent_upgrade.go:23-31 注释：accepted/dismissed 皆终态）；`now` 字段（:33-42）无任何调用点（生产/测试/container.go:458 装配均未引用——本计划作者 grep 实读确认）。
- Produces: (a) **存在性短路（F1）**——`reconcileProposals` 循环前 `ListProposals(ctx, tenantID)` 一次拉全，建 `map[adoptionID+"\x00"+toReleaseID]struct{}` 索引（分隔符用 `\x00` 防碰撞）；循环内该组合已存在于索引（任意状态）直接 `continue`——昂贵 diff 前短路，读路径 DB 往返从 ~3N+2 降为 2 次（ListAdoptions+ListProposals）+ 仅未物化组合的增量；索引方案零 repo 接口改动。(b) **stale 终态迁移（F2 上半）**——reconcile 内对索引中的 open 行检查 `row.FromReleaseID != adoption.AcceptedReleaseID`（该 adoption 循环现场）时 `TransitionProposal(ctx, tenantID, row.ID, []string{open}, dismissed, map[string]any{"resolved_by": "system:adoption-advanced"})`——过期 open 建议随 Adoption 前进迁终态 dismissed（不新增状态；resolved_by 用系统标记区分人工 dismiss，执行者按 row 现有字段宽度定值并在注释说明）；迁移后从索引/列表消失，不再可 Accept。(c) **Accept 校验（F2 下半）**——`AcceptUpgradeProposal` 在 adoption active 校验（:106-109）之后追加：`row.FromReleaseID != adoption.AcceptedReleaseID` → `ErrAgentUpgradeStateConflict`（"proposal's from release no longer matches the adoption's accepted release"）——过期建议不可 Accept，不再创建指向已被越过 Release 的冗余草稿 Variant。(d) **F4 顺手**——移除 `now` 字段与构造注入（:35/:41）；`NewAgentUpgradeService` 签名不变（container.go:458 调用零改动）；`time` import 若无其他引用同步移除（编译器把关）。

**根因与修复说明：** F1：reconcile 在每次 List/Get 请求时对每个 active adoption 重放 listing+两份 Release 读取并完整解码两份 Bundle JSON（携带完整便携 payload）计算 diff，已物化组合不做存在性短路——读路径 O(N) 次大 JSON 解码 + ~3N 条 DB 往返，adoption 数量大的租户列表/详情请求持续退化。F2：open 建议不随 Adoption 前进而关闭——re-adopt 前进 AcceptedReleaseID 后，from_release_id 脱节的 open 建议永久残留且可 Accept（Accept 不校验 FromReleaseID==AcceptedReleaseID），创建指向已被越过 Release 的冗余草稿 Variant。F4：`now func() time.Time` 为死注入（误导维护者以为服务层控制时间源，时间戳实际全由 repository 层生成）。

- [x] **Step 1: 写失败测试**

```go
// agent_upgrade_test.go 追加
func TestReconcileSkipsMaterializedPairsWithoutDiff(t *testing.T) {
	// arrange: 计数 fake repo——已物化 (adoption, to_release) 一条 open 行；listing/release 正常
	// act: ListUpgradeProposals（触发 reconcile）
	// assert: GetRelease/diff 重算计数为 0（或 GetMarketplaceListing 仅被未物化组合触达）；
	//   返回列表仍含该 open 行（行为不变，只省重算）
}
func TestReconcileDismissesStaleOpenProposalsAfterReAdopt(t *testing.T) {
	// arrange: open 行 from_release=旧指针；adoption.AcceptedReleaseID 已前进到新 Release
	// act: reconcile（经 ListUpgradeProposals 触发）
	// assert: 该行 state=dismissed、resolved_by=system 标记；列表不再以 open 呈现
}
func TestAcceptRejectsProposalFromStaleReleasePointer(t *testing.T) {
	// arrange: 同上 stale open 行
	// act: AcceptUpgradeProposal → assert: ErrAgentUpgradeStateConflict；无草稿 Variant 被创建
}
```

（F4 为编译级验证：Step 3 移除字段后 `go build ./...` 通过即证明无调用点。）

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/application/service/ -run 'TestReconcileSkips|TestReconcileDismisses|TestAcceptRejectsProposal'`
Expected: FAIL ×3。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 落地（索引 map 的键编码、resolved_by 系统标记值按实读字段定；now 移除含 time import 清理）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/application/service/`
Expected: PASS。
Run: `go build ./... && go test ./internal/application/service/ ./internal/application/repository/`
Expected: build exit 0；repository 仅差异记录 7 的 3 项预存在失败（不新增）。

- [x] **Step 5: Commit**

```bash
git add internal/application/service/agent_upgrade.go internal/application/service/agent_upgrade_test.go
git commit -m "fix(agent-upgrade): short-circuit materialized pairs, dismiss stale opens, guard accept (R5-F1, R5-F2; F4)"
```

---

### Task 7: marketplace 许可登记审计保真（R5-F3）

**Files:**
- Modify: `internal/application/repository/agent_marketplace_lineage.go:76-92`（UpsertLicense）
- Test: `internal/application/repository/agent_fork_lineage_test.go`（或 service/agent_marketplace_test.go——按既有 license 用例宿主就近，执行者实读定）

**Interfaces:**
- Consumes: 实体契约（types/agent_fork_lineage.go:24-27 注释：AgentLicenseEntity「Re-registering (upsert) updates the flags——that is how a license flip propagates」）；现状（实读 :76-92）：输入副本 `CreatedAt/UpdatedAt = now`（:80-81）、`DoUpdates: AssignmentColumns([]string{"name", "allows_redistribution", "created_by", "updated_at"})`（:86，含 created_by、不含 created_at——两创建期字段处理不一致）、成功后 `return license, nil` 直接返回输入副本（:92）；`GetLicense(ctx, licenseID)`（:95-101，not-found 返回 nil,nil）；handler `agent_marketplace.go:288` `c.JSON(http.StatusCreated, ..., licenseDTO(row))` 直接渲染 upsert 返回值。
- Produces: `DoUpdates` 移除 `"created_by"`（保留 `name/allows_redistribution/updated_at`——与契约「只更新 flags」及 created_at 的保留处理对齐）；upsert 成功后 `return r.GetLicense(ctx, license.ID)` 重读库中行返回——首登路径返回行与库一致（等价替换），冲突路径返回**保留原 created_at/created_by** 的库中行，API 响应不再报出与库不符的登记人/时间。Create 前设置输入副本时间戳的既有逻辑保留（insert 路径需要）。

**根因与修复说明：** 冲突更新面包含 created_by 与实体契约不符（re-register 只更新 flags），许可翻转的正常传播路径会永久覆盖「首次登记人」审计信息；且冲突路径返回的是本次输入副本（CreatedAt=本次 now、CreatedBy=本次 actor），handler 直接渲染——重复登记后 API 报出与库中行不符的 created_at/created_by。与一轮批次「值传递 store 不得回显未落库状态」同族纪律：重读库行再返回。

- [x] **Step 1: 写失败测试**

```go
// repository 层测试追加（宿主按 Step 前实读定）
func TestUpsertLicensePreservesFirstRegistrar(t *testing.T) {
	// arrange: 首登（created_by=A）→ 重复登记（created_by=B，翻转 allows_redistribution）
	// act: UpsertLicense 二次
	// assert: 返回行 created_by 仍为 A、allows_redistribution 为新值、updated_at 前进；
	//   再 GetLicense 读库与返回行一致（负例：当前返回输入副本 created_by=B → 红）
}
```

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/application/repository/ -run TestUpsertLicensePreservesFirstRegistrar`
Expected: FAIL——当前 DoUpdates 覆盖 created_by 且返回输入副本。

- [x] **Step 3: 最小实现**

按 Interfaces Produces 落地（两处改动：DoUpdates 列清单 + 返回前重读）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/application/repository/ -run 'License|Lineage'` 与 `go test ./internal/application/service/`
Expected: PASS（repository 全量仅差异记录 7 的 3 项预存在失败）。

- [x] **Step 5: Commit**

```bash
git add internal/application/repository/agent_marketplace_lineage.go
git commit -m "fix(marketplace): preserve the first license registrar and return the stored row on upsert (R5-F3)"
```

---

### Task 8: ConfluencePublishService 收敛 ProviderProfile（lowWorth R5-F12，收尾）

**Files:**
- Modify: `internal/modules/appconnector/publish/confluence.go`（234 行服务体收敛为薄构造 + profile）
- Modify: `internal/modules/appconnector/publish/plan.go`（ProviderProfile 如有抽象缺口则扩展）
- Modify: `internal/container/confluence_publish.go`（装配改走 NewProviderPublishService）
- Test: `internal/modules/appconnector/publish/confluence_test.go`、`provider_test.go`

**Interfaces:**
- Consumes: `NewProviderPublishService`（plan.go:130-140，注释自认「#49 feishu, #50 confluence」——confluence 预留位）；feishu 装配先例 `container/feishu_publish.go:39`（`publish.NewProviderPublishService(actions, store, pubs, versions, ...)`）；两服务方法集平行（confluence.go:62/:171/:181/:190/:202 ≡ plan.go:162/:273/:283/:292/:312，FormPlan 开头逐行同构）；Confluence 特有决策清单（收敛时需安放）：`ConfluenceScope` 读取与 `scope.AppID != appIDConfluence` 校验、update 目标 authority rule（LatestPublishedByDestination 归属）、`ConfluenceStorageBody` 派生、edition/apiBasePath 携带。
- Produces: `NewConfluencePublishService(...)` 变为 feishu 同型的薄构造——内部调 `NewProviderPublishService(..., confluenceProfile(...))`；confluence 特有决策下沉 profile/bridge 层（feishu 的 `FeishuProfile` 形态为样板，provider.go 实读对齐）；`ConfluencePublishService` 类型删除或改为 `NotionPublishService` 别名（执行者按 handler/container 调用面择最小扰动形态）；对外行为零变化。约束：若发现某决策无处安放（profile 抽象缺口），**扩展 ProviderProfile**（如 scope 校验钩子），绝不在共享服务体加 provider 分支（AC1「provider difference lives only in the adapter/bridge layer」——plan.go:119-121 既有注释契约）。

**根因与修复说明：** `ConfluencePublishService`（confluence.go 全文件 234 行）与 `NotionPublishService`（plan.go:108 起）五方法完全平行、逐行同构，是 #49/#50 ProviderProfile 抽象落地时 confluence 侧未收敛的遗留——每次服务体修 bug 都要双写（本批 Task 3 的对账语义已在两侧各有一份投影）。修复先例齐备（NewProviderPublishService 预留 + feishu 已走通），按样板收敛即可。

- [x] **Step 1: 写守护测试**

```go
// provider_test.go 追加
func TestConfluencePublishServiceIsProviderProfileThin(t *testing.T) {
	// 源断言：publish/confluence.go 不再包含与 plan.go 平行的 FormPlan/Execute/Reconcile/Receipt 服务体
	// （grep 'func (s \*ConfluencePublishService) FormPlan' 零命中，或按最终形态断言薄构造存在）
}
```

（行为面以既有 `confluence_test.go` 全绿为准入——本任务是行为不变重构，既有测试即回归网。）

- [x] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -run TestConfluencePublishServiceIsProviderProfileThin`
Expected: FAIL——当前服务体在位。

- [x] **Step 3: 最小实现（重构）**

按 Interfaces Produces 落地；每挪一段决策跑一次 `go test ./internal/modules/appconnector/publish/ ./internal/modules/appconnector/...` 保持全绿（小步重构、频繁可回退）。

- [x] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/... ./internal/handler/ -run 'Publish|Confluence'`
Expected: PASS（handler/container 调用面零行为变化）。

- [x] **Step 5: Commit**

```bash
git add internal/modules/appconnector/publish/ internal/container/confluence_publish.go
git commit -m "refactor(publish): collapse ConfluencePublishService onto the provider profile (R5-F12)"
```

---

## 终局验收（全批完成后）

- [x] Run: `go build ./...`
  Expected: exit 0（仅 ld 重复库警告）。
- [x] Run: `go test ./internal/modules/appconnector/... ./internal/modules/codedelivery/`
  Expected: 全 ok（基线 4 项 TestConfluenceCreate RED 已转绿）。
- [x] Run: `go test ./internal/application/... 2>&1 | grep -E "^FAIL"`
  Expected: 仅 `internal/application/repository`（差异记录 7 的 3 项预存在 delivery e2e，域外不修）；service 全 ok。
- [x] Run: `go vet ./internal/modules/appconnector/... ./internal/modules/codedelivery/ ./internal/application/...`
  Expected: 无新增告警。
- [x] 发现对账：10 项主发现全部有对应任务与测试（覆盖对照表 10 行）；3 项 lowWorth 全部有折叠归宿（F4→Task 6、F10→Task 4、F12→Task 8）。
