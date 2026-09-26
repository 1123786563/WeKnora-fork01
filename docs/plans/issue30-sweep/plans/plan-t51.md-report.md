# T21 #51 Task 2 实施报告：plan 包——PlanDigest 纯函数与 FormPlan

- **执行者**：实现员-t51-任务2（subagent-driven-development 实现员，TDD）
- **Worktree**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`）
- **提交**：`aa2558dd1` `feat(appconnector): plan 包——PlanDigest 纯函数 + FormPlan（T21 #51 Task 2）`
- **状态**：DONE

---

## 1. 实现内容

按计划 Task 2 逐字落地两个文件，产出供 Task 3-6 消费的 plan 包接口（签名与计划「Produces」逐字一致，已对照核验）：

`internal/modules/appconnector/plan/plan.go`（新建，329 行）：

- 哨兵：`ErrPlanInvalidInput` / `ErrPlanDigestMismatch` / `ErrPlanState`（`plan.go:31-44`）
- 状态常量：`PlanStateAwaitingApproval` / `PlanStateAuthorized`（`plan.go:47-50`）；逐项处置常量 `ItemExecuted/ItemSkippedSucceeded/ItemSkippedUnapproved/ItemSkippedInFlight/ItemSettled/ItemExcluded`（`plan.go:53-60`，Task 4 消费）
- `PlanDigest(tenantID uint64, actorID string, items []DigestItem) (string, error)`（`plan.go:91`）：结构化 JSON（layout version + tenant + actor + 有序 items 的 action id/digest/connection/target/risk）sha256 —— AC1 纯函数锚；零项返回 `ErrPlanInvalidInput`
- `NewService(plans, actions, approver, pubs) *Service` 与 `PlanApprover` 接口（`*appconnectorsvc.ActionService` 天然满足）（`plan.go:194-207`）
- `Service.FormPlan`（`plan.go:209-260`）：逐项复用 #48 `publish.NotionPublishService.FormPlan`（scope 校验→destination 权威规则→artifact→Notion 段落块→外部版本预读→A03 Prepare→planned 回执行），计划 digest 绑定从权威 `app_actions` 行回读的身份（非 formation 回显），`PlanStore.CreatePlan` 单事务落计划行+有序项；中途失败不留计划行（fail closed）
- 类型：`ItemInput`/`FormInput`/`ItemView`/`PlanView`/`ApproveInput`/`ItemOutcome`/`ExecuteOutcome`/`PlanStatus`（json 标签逐字：`id/state/digest/items`、`seq/action_id/digest/mode/destination/expected_external_version/title` 等）
- Task 3/4 占位：`// Approve ... IMPLEMENT IN TASK 3.`、`// Execute ... IMPLEMENT IN TASK 4.`、`// Status ... IMPLEMENT IN TASK 4.`（`plan.go:262-264`）——计划明示的占位，Task 3/4 必须替换
- helpers：`normalizeExclusions`/`parseExclusions`/`equalSeqs`/`view`（计划携带，Task 3/4 消费）

`internal/modules/appconnector/plan/plan_test.go`（新建，281 行）：4 个测试 + 自包含 doubles（stubArtifacts/stubContent/stubRemote/stubScopes/stubGuard/scriptedDispatcher）：

1. `TestPlanDigestBindsSetOrderAndContent` —— AC1 纯函数锚：集合/顺序/单项内容 digest/连接/目标/actor 任一变化即新 digest，且仅这些变化才变
2. `TestPlanFormBuildsOrderedDigestBoundPlan` —— formation 逐项走 #48 seam；digest 绑定权威存储行（从 store 回读重算比对）；每项有 planned publication 行；formation 零授权零派发（dispatch 计数为 0）
3. `TestPlanFormMidItemFailureLeavesNoPlanRow` —— 第 3 项 update 未发布目标（`publish.ErrPublishUpdateTargetNotPublished` 权威规则）中止 formation：无计划行、无计划项
4. `TestPlanFormRejectsInvalidInput` —— 零项/零租户拒绝

## 2. 前置接口核验（开工前逐一对过当前 HEAD）

开工前逐一核验计划假设与代码现状一致，全部命中：

- Task 1 `PlanStore`/`ActionPlanRow`/`ActionPlanItemRow`（`internal/modules/appconnector/repository/appconnector/plan.go:35-137`，提交 `4276e36ac`）
- `publish.NewNotionPublishService` 7 参签名、`PublishPlanInput`/`PublishPlanView`/`PublishReceiptView`、`ErrPublishUpdateTargetNotPublished` 权威规则（`publish/plan.go:120-165`）
- `appconnectorsvc.NewActionService(store, guard, gate, dispatcher, unknown)`、`ActionStoreSource.FindAction`、`A02Guard.Check(ctx, OCSubject, string, int64) error`、`ActionDispatcher.Dispatch`（`service/appconnector/action.go:82-157,192-195`）
- `repoappconn.ActionRow.ArgsDigest/ConnectionID/Target/Risk`、`PublicationStore.FindByAction`（`repository/appconnector/action.go:35-41`、`publication.go:91`）
- `appconn.ActionAwaitingApproval/ActionAuthorized`、`OCSubject`（`action.go:38-39`、`oc_binding.go:53`）
- `repository.ArtifactVersion{ID,Digest,MIME,Size}`（`internal/application/repository/artifact_version.go:51-61`）
- `github.com/google/uuid v1.6.0`（go.mod:30）

开工前基线：`go build ./internal/modules/appconnector/...` → BUILD_OK。

## 3. TDD 证据

### RED（先测试，确认失败原因正确）

命令：

```
go test ./internal/modules/appconnector/plan/ -count=1
```

输出（节选，完整见执行记录）：

```
# github.com/Tencent/WeKnora/internal/modules/appconnector/plan [github.com/Tencent/WeKnora/internal/modules/appconnector/plan.test]
internal/modules/appconnector/plan/plan_test.go:103:12: undefined: Service
internal/modules/appconnector/plan/plan_test.go:125:9: undefined: NewService
internal/modules/appconnector/plan/plan_test.go:129:25: undefined: ItemInput
internal/modules/appconnector/plan/plan_test.go:134:44: undefined: PlanView
internal/modules/appconnector/plan/plan_test.go:136:50: undefined: FormInput
internal/modules/appconnector/plan/plan_test.go:148:12: undefined: DigestItem
internal/modules/appconnector/plan/plan_test.go:152:15: undefined: PlanDigest
internal/modules/appconnector/plan/plan_test.go:156:17: undefined: PlanDigest
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/plan [build failed]
```

失败原因符合计划 Step 2 预期：包 `plan` 尚不存在，测试引用的类型/函数未定义——失败来自「实现缺失」而非测试自身的语法/依赖错误。

### GREEN（最小实现后确认通过）

命令（计划 Step 4 原文）：

```
go test ./internal/modules/appconnector/plan/ -count=1 && go build ./...
```

输出：

```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	1.525s
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
BUILD_OK
```

（`-v` 细分输出确认 4 个测试逐一 `--- PASS`：TestPlanDigestBindsSetOrderAndContent / TestPlanFormBuildsOrderedDigestBoundPlan / TestPlanFormMidItemFailureLeavesNoPlanRow / TestPlanFormRejectsInvalidResult 之外无失败项，末尾 `PASS` + `ok`。`ld: warning` 是 macOS 链接器对 cmd/desktop、cmd/server 的既有噪音，非错误。）

## 4. 回归与静态检查（提交前自检，均在本 ask 实跑）

```
go test ./internal/modules/appconnector/... -count=1
```

```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.653s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	2.561s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	1.466s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	1.560s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.767s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.723s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.879s
```

```
go vet ./internal/modules/appconnector/plan/   → VET_OK
```

## 5. 文件变更

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/modules/appconnector/plan/plan.go` | 新建 | PlanDigest 纯函数 + Service 构造器 + FormPlan；Approve/Execute/Status 为计划明示的 Task 3/4 占位 |
| `internal/modules/appconnector/plan/plan_test.go` | 新建 | 4 测试 + 自包含 doubles |

无其他文件改动；未触碰冻结面（`ActionRow`/`ActionService`/`publish` 包/既有端点零改动）。

## 6. 自检发现

1. **测试输出中的 gorm 红色 `record not found` 日志为预期路径噪音，非失败**：`TestPlanFormMidItemFailureLeavesNoPlanRow` 断言 `page-404` 无 prior receipt（`LatestPublishedByDestination` 返回 `gorm.ErrRecordNotFound`），gorm logger 会打印该级别日志；断言本身 `--- PASS`。与 #48 publish 包测试同款噪音，不影响 pristine 判定（无 FAIL、无 panic、无 stray warning 型告警）。
2. **`okReceipt` helper 在 Task 2 测试中定义暂未使用**：Go 允许未使用的包级函数，计划原文即包含它（Task 4 的 AC2 测试消费）。编译已验证无问题。
3. **计划代码与当前 HEAD 的两处 import 需求吻合**：plan.go 携带 `time`/`appconn` import 但 Task 2 主体暂只用占位 `var _ =` 保住——这是计划明示的 Task 3/4 桥接手段，已逐字保留。
4. **Mimosa hook 提示**：`git commit` 时 hook 报告 `scanner_enobufs`（扫描器资源不足，完整项目审计未跑成），按兼容策略放行提交。本任务未宣称任何安全扫描结论；新增查询均为 gorm 参数绑定（`Where("col = ?", v)`），无字符串拼接 SQL，无新增凭据。
5. **范围内未做**：计划编号顺延条款（Task 0/1 的迁移号）与本任务无关，未触碰；`plan` 包暂未被任何 HTTP/容器面引用（Task 5/6 接线），属计划节奏。

## 7. 供后续任务消费的接口清单（Produces，逐字）

- `plan.PlanDigest(tenantID uint64, actorID string, items []DigestItem) (string, error)`；`plan.DigestItem{Seq int; ActionID, ActionDigest, Connection, Target, Risk string}`
- `plan.NewService(plans *repoappconn.PlanStore, actions appconnectorsvc.ActionStoreSource, approver PlanApprover, pubs *publish.NotionPublishService) *Service`；`plan.PlanApprover interface { Approve(ctx context.Context, id, actor, digest string) error }`
- `plan.ItemInput{ConnectionID, SessionID, ArtifactVersionID, Title, ParentPageID, PageID string}`；`plan.FormInput{TenantID uint64, ActorID string, Items []ItemInput}`
- `(s *Service) FormPlan(ctx, in FormInput) (PlanView, error)`；`plan.PlanView{ID, State, Digest string, Items []ItemView}`；`plan.ItemView{...}`（json 标签齐备）
- 哨兵 `plan.ErrPlanInvalidInput/ErrPlanDigestMismatch/ErrPlanState`；常量 `plan.PlanStateAwaitingApproval/PlanStateAuthorized` 及六个 `Item*` 处置常量
- 测试 doubles（`newPlanSvcEnv`/`formTwo`/`item`/`scriptedDispatcher`/`okReceipt`）同包可复用，Task 3/4 直接追加测试

**待办提示（给控制器）**：Task 3 必须替换 `plan.go:262` 的 `// Approve ... IMPLEMENT IN TASK 3.` 占位；Task 4 替换 `:263-264` 的 Execute/Status 占位——计划明示占位不得留到计划完成。

---

# 修复轮 1 报告（审查发现修复）

- **执行者**：实现员-t51-任务2（resumed with findings）
- **提交**：见下方「修复轮提交」
- **状态**：DONE（1/1 findings 修复，变异验证 + 全量回归在案）

## F1（important）：TestPlanFormMidItemFailureLeavesNoPlanRow 核心断言空转

**审查指认**（`internal/modules/appconnector/plan/plan_test.go:275-278`）：「失败不留计划行」断言用 `ListPlanItems(ctx,7,"nonexistent")`——对任意不存在的 plan_id 恒返回 0 行，CreatePlan 回归挪位或吞错续建不会使测试变红。

**核实**：属实。实测变异验证（见下）证明旧断言对实现回归盲视。

### 修复内容（仅动本任务授权文件 plan_test.go，零实现改动）

1. `planSvcEnv` 增加 `db *gorm.DB` 字段（`plan_test.go:97-104`），`newPlanSvcEnv` 回填（`plan_test.go:127`）。
2. 空转断言替换为对 `app_action_plans` / `app_action_plan_items` 两表按租户**直接计数**（`plan_test.go:276-293`）：`e.db.Model(&repoappconn.ActionPlanRow{}).Where("tenant_id = ?", uint64(7)).Count(&planCount).Error`（参数绑定，无拼接 SQL），断言 `planCount == 0 && itemCount == 0`；id 探测式断言删除。
3. 修复过程中的实测坑（记录在案）：第一版漏写 `.Error` —— gorm `Count()` 返回 `*gorm.DB` 链对象（恒非 nil），`if err := ...Count(&n); err != nil` 恒真、测试立即假红（错误对象打印为 `count plans: *gorm.DB: &{...}`）；改为取 `.Count(&n).Error` 后语义正确。

### 变异验证（隔离证明：旧断言空转、新断言有杀伤力）

变异体（临时改动，已还原）：`plan.go` FormPlan 的 mid-item failure 分支在返回原错误前先 `CreatePlan` 一行 `plan_leak` 计划 + 1 条计划项（模拟「失败也建计划行」回归；错误仍如实返回，故隔离于「错误类型断言」之外）。

| 步骤 | 被测断言 | 实现 | 命令 | 结果 |
|---|---|---|---|---|
| 1 | 旧断言（ListPlanItems 探测） | 变异（泄漏 1 计划行） | `go test ./internal/modules/appconnector/plan/ -run TestPlanFormMidItemFailureLeavesNoPlanRow -count=1` | `ok` → **旧断言盲视泄漏，审查属实** |
| 2 | 新断言（表计数） | 同一变异 | 同上 | `--- FAIL: ... plan_test.go:291: a failed formation must leave NO plan rows/items, got plans=1 items=1` → **新断言精确抓红** |
| 3 | 新断言 | 还原（HEAD `aa2558dd1` 版 plan.go） | 同上 | `ok` |

（步骤 1 与步骤 2 之间新断言尚未写入时跑的是旧测试代码；步骤 2 起测试为表计数版本。还原用 `git checkout -- internal/modules/appconnector/plan/plan.go`，`git diff --stat` 前后确认变异仅 7 行且还原后仅剩 plan_test.go 的 18+/4-。）

### 修复后回归（本 ask 实跑）

```
go test ./internal/modules/appconnector/plan/ -count=1
→ ok  github.com/Tencent/WeKnora/internal/modules/appconnector/plan  1.008s

go test ./internal/modules/appconnector/... -count=1
→ ok  .../appconnector 0.528s | connectorcontrol 2.775s | openconnector 0.584s
  | plan 2.428s | publish 1.299s | repository/appconnector 2.275s | service/appconnector 3.398s（7 包全 ok）

go vet ./internal/modules/appconnector/plan/ → VET_OK
go build ./... → exit 0（仅 macOS ld 既有噪音）
```

### 附带更正

上轮报告「待办提示」中占位行号笔误：实际占位在 `plan.go:252`（`// Approve ... IMPLEMENT IN TASK 3.`）与 `plan.go:253-254`（Execute/Status），非 `:262-264`（该行号来自变异前草稿）。Task 3/4 以 `grep -n "IMPLEMENT IN TASK" plan.go` 现查为准。

---

# Task 3 实施报告：plan 包——Approve（整体批准 / 排除单项 / AC1 批准面）

- **执行者**：实现员-t51-任务3（subagent-driven-development 实现员，TDD）
- **Worktree**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`）
- **提交**：`6bdf81aa0` `feat(appconnector): plan Approve——整体批准/排除单项/AC1 批准面（T21 #51 Task 3）`
- **状态**：DONE（RED→GREEN→提交，全部检查实跑取证）

## 1. 实现内容

严格按 `docs/plans/issue30-sweep/plans/plan-t51.md` Task 3 执行，仅动两个授权文件：

### `internal/modules/appconnector/plan/plan.go`（修改）
- 把占位注释 `// Approve records the owner's whole-plan decision. IMPLEMENT IN TASK 3.`（原 :252）替换为真实 `func (s *Service) Approve(ctx context.Context, tenantID uint64, planID, actor string, in ApproveInput) (PlanView, error)`，语义与计划逐字一致：
  - 空 actor/digest → `ErrPlanInvalidInput`；
  - `row.Digest != in.Digest` → `ErrPlanDigestMismatch`（AC1 批准面锚：拒绝且零写入）；
  - `normalizeExclusions` 非法排除（越界/重复）→ `ErrPlanInvalidInput`；
  - 已 authorized 时排除集冻结：`parseExclusions` + `equalSeqs`，不同集合 → `ErrPlanState`（排除集永不静默改写）；
  - `PlanStore.ApprovePlan` CAS（digest + 状态∈{awaiting,authorized}）落 state/excluded_json/approved_by/approved_at；
  - 逐项：被排除项 `continue`（永不批准永不派发）；非 `awaiting_approval` 状态项跳过（authorized 幂等、终态不适用）；仍 awaiting 的项经 `PlanApprover.Approve(ctx, id, actor, action.ArgsDigest)` 逐项 digest 绑定批准；
  - 返回 `s.view(...)` 恢复后的 PlanView。
- 删除文件尾部两行占位卫兵 `var _ = appconn.ActionAwaitingApproval` 与 `var _ = time.Now`（`appconn`/`time` 自本任务起被真实使用）。
- `Execute`/`Status` 的 Task 4 占位注释保留不动（计划要求）。

### `internal/modules/appconnector/plan/plan_test.go`（追加）
追加计划 Task 3 Step 1 的测试代码（逐字）：`approveAll` helper + 4 个测试：
1. `TestPlanApproveWholeApprovesEveryIncludedItem`——含项全部逐项 authorized + 计划行记录 approved_by/approved_at
2. `TestPlanApproveExcludesItemNeverApprovesIt`（排除单项）——排除项停留 awaiting_approval + `[2]` 记录在计划行
3. `TestPlanApproveRejectsForeignDigest`（AC1 批准面锚）——p1 旧 digest 不能批准 p2 新内容（内容变化→digest 必不同先断言）、`deadbeef` 拒绝、两次拒绝后两计划状态均停留 awaiting_approval（零写入）
4. `TestPlanApproveRejectsBadExclusions`——越界 seq=3 / 重复 seq=[1,1] 均 `ErrPlanInvalidInput`

计划标注 `TestPlanApproveRecoveryExclusionFrozen` 依赖 Task 4 的 Execute，归 Task 4 Step 1——本任务未添加，符合计划。

## 2. 前置接口核验（开工前本 session 实跑核对）

- `ActionService.Approve(ctx, id, actor, digest) error` 在 `internal/modules/appconnector/service/appconnector/action.go:284` ✓
- `appconn.ActionAuthorized = "authorized"` 在 `internal/modules/appconnector/action.go:39` ✓
- 前置接口就位：`planSvcEnv.db` 字段（plan_test.go:103）、`TestPlanFormMidItemFailureLeavesNoPlanRow` 两表按租户直接计数（plan_test.go:281-292）✓
- Task 1 `PlanStore.ApprovePlan/FindPlan/ListPlanItems`（提交 `4276e36ac`）✓
- 开工基线：`go test ./internal/modules/appconnector/plan/ -count=1` → `ok ... 2.150s`（Task 2 的 4 测试绿）

## 3. TDD 证据（全部本 session 实跑）

### RED（Step 2：追加测试后、实现前）

命令：
```
go test ./internal/modules/appconnector/plan/ -run TestPlanApprove -count=1
```
输出：
```
internal/modules/appconnector/plan/plan_test.go:307:21: e.svc.Approve undefined (type *Service has no field or method Approve)
internal/modules/appconnector/plan/plan_test.go:339:21: e.svc.Approve undefined (type *Service has no field or method Approve)
internal/modules/appconnector/plan/plan_test.go:375:21: e.svc.Approve undefined (type *Service has no field or method Approve)
internal/modules/appconnector/plan/plan_test.go:378:21: e.svc.Approve undefined (type *Service has no field or method Approve)
internal/modules/appconnector/plan/plan_test.go:392:21: e.svc.Approve undefined (type *Service has no field or method Approve)
internal/modules/appconnector/plan/plan_test.go:396:21: e.svc.Approve undefined (type *Service has no field or method Approve)
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/plan [build failed]
```
失败形态与计划 Step 2 预期（`e.svc.Approve` 未定义编译失败）一致。

### GREEN（Step 4：最小实现后）

命令：
```
go test ./internal/modules/appconnector/plan/ -count=1 -v
```
输出（测试清单）：
```
=== RUN   TestPlanDigestBindsSetOrderAndContent
--- PASS: TestPlanDigestBindsSetOrderAndContent (0.00s)
=== RUN   TestPlanFormBuildsOrderedDigestBoundPlan
--- PASS: TestPlanFormBuildsOrderedDigestBoundPlan (0.00s)
=== RUN   TestPlanFormMidItemFailureLeavesNoPlanRow
--- PASS: TestPlanFormMidItemFailureLeavesNoPlanRow (0.00s)
=== RUN   TestPlanFormRejectsInvalidInput
--- PASS: TestPlanFormRejectsInvalidInput (0.00s)
=== RUN   TestPlanApproveWholeApprovesEveryIncludedItem
--- PASS: TestPlanApproveWholeApprovesEveryIncludedItem (0.00s)
=== RUN   TestPlanApproveExcludesItemNeverApprovesIt
--- PASS: TestPlanApproveExcludesItemNeverApprovesIt (0.00s)
=== RUN   TestPlanApproveRejectsForeignDigest
--- PASS: TestPlanApproveRejectsForeignDigest (0.00s)
=== RUN   TestPlanApproveRejectsBadExclusions
--- PASS: TestPlanApproveRejectsBadExclusions (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	3.987s
```
plan 包 8 测试全 PASS（Task 2 原 4 + Task 3 新 4）。

命令：`go build ./...` → 退出码 0（仅 macOS ld 既有噪音：`ld: warning: ignoring duplicate libraries: '-lc++'`，cmd/server 与 cmd/desktop）。

### 回归（超出计划 Step 4 的额外验证，本 ask 实跑）

命令：
```
go test ./internal/modules/appconnector/... -count=1
```
输出：
```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.636s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	3.726s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.727s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	2.743s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.763s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	2.672s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.532s
```
7 包全绿。

## 4. 文件变更

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/modules/appconnector/plan/plan.go` | 修改 | 占位替换为真实 Approve；删除两行 `var _ =` 占位卫兵 |
| `internal/modules/appconnector/plan/plan_test.go` | 追加 | approveAll helper + 4 个 Approve 测试 |

提交：`6bdf81aa0`（2 files changed, 168 insertions(+), 4 deletions(-)），提交后 `git status --short` 干净。未触碰冻结面（`ActionRow`/`ActionService` 生命周期/`publish` 包/既有端点零改动）。

## 5. 自检发现

1. **【跨任务现状，非本任务阻塞】Task 0（迁移轨道去重重编）在本 worktree 未执行**：实核 `migrations/sqlite/` 仍同时存在 `000114_mobile_device_app.*` 与 `000114_public_agent_marketplace.*`（双占未修），`internal/database/migration.go:33` 门控常量仍为 `114`。本任务不依赖全量迁移轨道（plan 包测试走 AutoMigrate 内存库），故不受阻、且未越权代做；但 **Task 6 的 AC3 全量迁移 e2e（`go test ./internal/handler/ -run TestNotionPublish`）在 Task 0 完成前不可运行**——计划 Tech Stack 节记载该命令当前报 `duplicate migration file: 000114_public_agent_marketplace.down.sql`。请编排方确认 Task 0 由其授权执行者完成。
2. **Review Focus 2（批准面腿）已覆盖**：`TestPlanApproveRejectsForeignDigest` 以两个真实计划钉死「计划内容变化→新 digest→旧 digest 拒批新内容」，并断言拒绝零状态迁移；执行面腿（`TestPlanExecuteRefusesUnapprovedOrForeignDigest`）归 Task 4。
3. **gorm trace 红色 `record not found` 日志为预期路径噪音**：Approve 测试路径中 publish seam 的 `LatestPublishedByDestination` 探测返回 gorm.ErrRecordNotFound 时打 trace 日志；全部断言 `--- PASS`，非缺陷（与 Task 2 报告第 6.1 条同判）。
4. **占位纪律**：Task 4 的 `// Execute ...`/`// Status ... IMPLEMENT IN TASK 4.` 占位注释仍在，Task 4 必须替换。
5. 零字符串拼接 SQL（计划层仅经 gorm 参数绑定 store 方法），零新增凭据，测试 token 均为契约双打假值。

---

# 修复轮 1 报告（Task 3 审查发现修复）

- **执行者**：实现员-t51-任务3（resumed with findings，修复轮 1/5）
- **提交**：`1c6779c7c` `fix(appconnector): pin exclusion-set freeze into ApprovePlan CAS, closing concurrent TOCTOU (ruling via escalation)`；ledger：`docs/plans/issue30-sweep/plans/plan-t51.md-ledger.md`（新建，逐字记录裁决 ruling 行）
- **状态**：DONE（F1 经主控裁决授权越界修复，RED→GREEN→并发回归在案；F2 按审查定性记录不修）

## F1（important）：排除集冻结并发 TOCTOU——经裁决越界下沉至 store CAS

**审查指认**：Approve 的冻结检查（`plan/plan.go:280` read-then-act）+ `PlanStore.ApprovePlan` CAS（`repository/appconnector/plan.go:117-122`，WHERE 允许 `state IN {awaiting,authorized}` 不钉 excluded_json）→ 两个并发 Approve 携不同 ExcludeSeqs 双双通过检查与 CAS，后写者覆写 excluded_json/approved_by/approved_at 并可批准前一次已被排除的项——「排除集首次批准后冻结」（plan-t51.md:64、Review Focus 3）并发失守。定性为 plan-mandated 设计缺口，建议「由计划所有者决定」。

**处置**：本实现员核实属实后，因修复必须修改 Task 1 授权文件（超出本任务授权）且属已批准语义的实现策略下沉，escalate 请裁决。**主控裁决（via escalation）：授权越界修复**——理由：TOCTOU 击穿「排除项永不批准执行」审批完整性不变量（load-bearing，不能留到 Task 6）；本修复是一行 WHERE 条件+参数绑定的实现层强化，不改接口/schema/契约，无新设计成分（区别于 t59 延期案）。ruling 行已逐字入 ledger（`plan-t51.md-ledger.md`）。

### 修复内容（最小变更：CAS WHERE 一处 + 注释；两个测试文件追加）

1. `repository/appconnector/plan.go` ApprovePlan CAS WHERE：
   - 原：`Where("tenant_id = ? AND id = ? AND digest = ? AND state IN ?", ..., []string{awaiting, authorized})`
   - 改：`Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)", tenantID, planID, digest, PlanStateAwaitingApproval, excludedJSON)`（全参数绑定，无拼接 SQL）
   - 语义：首批（awaiting→authorized）无条件 CAS；authorized 重批必须携带与已记录集合一致的 excluded_json——并发后写者 0 行得 `ErrPlanState`，**先写者获胜**，冻结决定永不静默改写。方法注释补充钉住语义；其余逻辑零改动。
2. `repository/appconnector/plan_test.go` 追加 `TestPlanApproveCASPinsExclusionSetOnReapproval`（确定性）：authorized+`[2]` 行用 `[]` 再 CAS → `ErrPlanState` 且行未被触碰。
3. `plan/plan_test.go` 追加 `TestPlanApproveConcurrentDistinctExclusionsSingleWinner`（service 层真并发回归护栏）：20 轮，两 goroutine 并发 Approve（集合 nil vs `{2}`），断言恰好一个成功、失败者为 plan-state 错误、终态 excluded_json ∈ {`[]`,`[2]`}、逐项状态与记录集一致（`[2]` 获胜则 item2 停留 awaiting_approval——被排除项无论谁后写都永不批准）。

### TDD 证据（全部本 ask 实跑）

**RED**（store 层，修复前）：
```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApproveCASPins -count=1 -v
=== RUN   TestPlanApproveCASPinsExclusionSetOnReapproval
    plan_test.go:128: CAS must pin the recorded exclusion set — a different set after approval is zero rows, got <nil>
--- FAIL: TestPlanApproveCASPinsExclusionSetOnReapproval (0.00s)
FAIL
```
`got <nil>` 即审查指认的直接证据：authorized 行被不同排除集成功覆写。

**GREEN**（修复后）：
```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlan -count=1 -v
--- PASS: TestPlanCreateFindRoundTrip (0.01s)
--- PASS: TestPlanApproveBindsDigestAndExclusions (0.00s)
--- PASS: TestPlanApproveCASPinsExclusionSetOnReapproval (0.00s)
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.650s
$ go build ./...   → BUILD_OK（仅 macOS ld 既有噪音）
```
既有幂等重批断言（authorized + 同集合 `[2]` 重批合法，Task 1 测试 `:105`）保持 PASS——恢复路径不受钉住影响。

**并发回归**（race detector 开启）：
```
$ go test ./internal/modules/appconnector/plan/ -run TestPlanApproveConcurrent -count=1 -race -v
=== RUN   TestPlanApproveConcurrentDistinctExclusionsSingleWinner
--- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (1.15s)
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	4.860s
$ go test ./internal/modules/appconnector/plan/ ./internal/modules/appconnector/repository/appconnector/ -count=1 -race
ok  .../plan	4.608s
ok  .../repository/appconnector	2.148s
```

**全量回归**：
```
$ go build ./...   → exit 0
$ go test ./internal/modules/appconnector/... -count=1
ok  .../appconnector 0.497s | connectorcontrol 3.092s | openconnector 0.534s | plan 1.922s
  | publish 2.526s | repository/appconnector 1.488s | service/appconnector 3.709s（7 包全 ok）
```

### 待核实项 1 复核（审查称上轮证据未独立复跑）

本 ask 复跑确认上轮（Task 3 主体）证据属实：plan 包 8/8 `--- PASS`（`go test ./internal/modules/appconnector/plan/ -count=1 -v`）+ appconnector 7 包全 ok——与报告 `:236-301` 声称一致。

## F2（important）：Task 0 未执行——跨任务协调项，本修复轮不修（按裁决执行要求 5）

审查定性「跨任务协调项，非 Task 3 缺陷」，裁决确认「记入报告供主控/编排层收口」。本 ask 复核现状属实（Task 0 Step 0 命令实跑）：

```
$ ls migrations/sqlite/ | grep -E "000114|000118|000119"
000114_mobile_device_app.down.sql / .up.sql
000114_public_agent_marketplace.down.sql / .up.sql     ← 双占仍在
000119_app_action_plans.down.sql / .up.sql             ← Task 1 已落位
$ ls migrations/versioned/ | grep -E "000193|000197|000198"
000193_mobile_device_app.* / 000193_public_agent_marketplace.*   ← 双占仍在
000198_app_action_plans.*                               ← Task 1 已落位
$ grep -n "sqliteAdoptionFKRelaxationMigrationVersion = " internal/database/migration.go
33:const sqliteAdoptionFKRelaxationMigrationVersion = 114        ← 未改
$ grep -n "000114_public_agent_marketplace" internal/database/migration.go
123:		_, err = os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")  ← 未改
```

118/197 槽位仍空闲（目录核查无文件占用），Task 0 可按计划原文执行（Step 0 占用核查先跑）。**需编排方授权执行者完成 Task 0，否则 Task 6 的 AC3 全量迁移 e2e（`go test ./internal/handler/ -run TestNotionPublish`）不可运行。**

## 其余待核实项处置（无法从本 diff 验证，如实声明）

- 审批谓词（发起者或 owner/admin）→ 归 Task 5 wire 层，未验证。
- 排除项「永不派发」执行侧保证 + AC1 执行面腿（`TestPlanExecuteRefusesUnapprovedOrForeignDigest`）→ 归 Task 4，未验证。
- `TestPlanApproveRecoveryExclusionFrozen`（同 digest 幂等重批完整闭环）→ 计划明示归 Task 4 Step 1，未验证。
- blocked-env：真实 Notion 多操作验收（NOTION_TOKEN 门控）本环境不可运行；AC3 本地替代证据归 Task 6 且受 F2 阻断——skip 不是 pass，本修复轮零伪造证据。
- 并发测试的局限性声明：service 层并发测试在 sqlite 单连接（MaxOpenConns(1)）下 goroutine 交错受 DB 串行化约束，属「真 goroutine 并发、DB 操作串行化」的护栏；主杀伤力证据是 store 层确定性 RED（`got <nil>`）与 CAS 钉住语义本身。修复前该并发测试为间歇红（双双成功的交错出现即红），修复后 20 轮稳定绿。

## 提交清单（修复轮 1）

```
1c6779c7c fix(appconnector): pin exclusion-set freeze into ApprovePlan CAS, closing concurrent TOCTOU (ruling via escalation)
  4 files changed, 106 insertions(+), 2 deletions(-)
  （repository/appconnector/plan.go + plan_test.go、plan/plan_test.go、plan-t51.md-ledger.md 新建）
```
提交后 `git status --short` 干净。

---

# 修复轮 2 报告（findings 与修复轮 1 逐字相同——F1 已修复在案，本轮复跑取证）

- **执行者**：实现员-t51-任务3（resumed with findings，修复轮 2/5）
- **提交**：本修复轮零代码改动（无需新修复）；证据复核提交见下方
- **状态**：DONE（F1 已按修复轮 1 的主控裁决修复并提交在案；本轮实核修复在 HEAD、全部测试复跑绿）

## 本轮 findings 判定：与修复轮 1 逐字重复，非新发现

本轮两个 findings 及待核实项列表与修复轮 1/5 的 ask **逐字相同**。决定性证据是行号：findings 仍引用 `repository/appconnector/plan.go:117-122` 的 CAS「允许 state IN {awaiting,authorized}」——而该位置在修复轮 1（commit `1c6779c7c`）已改为钉住形态（现 `:125-127`）。本轮实核 HEAD 实码：

```
$ sed -n '112,132p' internal/modules/appconnector/repository/appconnector/plan.go
// The exclusion-set freeze (排除集首次批准后冻结) is pinned INSIDE the
// CAS: a row already authorized can only be re-approved with the SAME
// recorded excluded_json. ...
	Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)",
		tenantID, planID, digest, PlanStateAwaitingApproval, excludedJSON).
```

findings 描述的 TOCTOU 窗口（CAS 不钉 excluded_json、后写者覆写）在 HEAD 已不存在。依据修复轮 1 裁决（ledger `plan-t51.md-ledger.md`：F1 授权越界下沉至 store CAS），F1 无需也不能重复修复；重复改动只会引入回归风险。

## F1 修复在 HEAD 的复跑证据（全部本 ask 实跑）

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApprove -count=1 -race -v
--- PASS: TestPlanApproveBindsDigestAndExclusions (0.01s)      ← 恢复路径幂等重批不受钉住影响
--- PASS: TestPlanApproveCASPinsExclusionSetOnReapproval (0.00s) ← F1 钉住主测试
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	4.256s

$ go test ./internal/modules/appconnector/plan/ -run TestPlanApproveConcurrent -count=1 -race -v
--- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (2.06s) ← 并发不变量 20 轮
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	6.993s

$ go test ./internal/modules/appconnector/plan/ -count=1 -v   → 9 个 --- PASS（8 原 + 1 并发）
$ go test ./internal/modules/appconnector/plan/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	5.755s

$ go test ./internal/modules/appconnector/... -count=1        → 7 包全 ok
（appconnector 0.214s | connectorcontrol 3.829s | openconnector 0.244s | plan 2.623s
  | publish 2.504s | repository/appconnector 0.507s | service/appconnector 4.989s）

$ go build ./...                                              → exit 0
```

## 待核实项逐条复核（与本轮可验证范围）

1. **上轮 RED/GREEN/回归为报告声称**——本 ask 复跑：plan 包 9/9 `--- PASS`、7 包回归 ok、build exit 0（上方输出），与报告声称一致 ✓
2. **审批谓词归 Task 5 wire 层**——本 diff 无法验证，维持声明。
3. **排除项执行侧保证 + AC1 执行面腿归 Task 4**——本 diff 无法验证，维持声明。
4. **blocked-env（NOTION_TOKEN 门控）**——本环境不可运行，维持声明，零伪造证据。
5. **`TestPlanApproveRecoveryExclusionFrozen` 归 Task 4 Step 1**——本 diff 无法验证，维持声明。注：同 digest 幂等重批的 store/service 两层行为已被 `TestPlanApproveBindsDigestAndExclusions:105`（store 层，钉住后仍 PASS）与修复轮 1 service 层测试覆盖其可验证部分，完整闭环仍待 Task 4。

## F2（Task 0 未执行）本轮复核：现状未变，仍待编排方收口

```
$ ls migrations/sqlite/ | grep -cE "^000114_"  → 4（mobile_device_app × marketplace 双占仍在）
$ grep -n "sqliteAdoptionFKRelaxationMigrationVersion = " internal/database/migration.go
33:const sqliteAdoptionFKRelaxationMigrationVersion = 114
```

按修复轮 1 裁决执行要求 5 与审查定性（跨任务协调项，非 Task 3 缺陷），本修复轮不动 migrations/**；**需编排方授权执行者完成 Task 0**（118/197 空闲，可按计划原文执行），否则 Task 6 的 AC3 全量迁移 e2e 不可运行。

## 提交清单（修复轮 2）

零代码改动；报告追加单独入册（见 git log）。HEAD 仍为：
```
1c6779c7c fix(appconnector): pin exclusion-set freeze into ApprovePlan CAS, closing concurrent TOCTOU (ruling via escalation)
```

---

# 修复轮 3 报告（findings 第三次逐字重复——F1 修复持续在案，本轮复跑取证）

- **执行者**：实现员-t51-任务3（resumed with findings，修复轮 3/5）
- **提交**：本修复轮零代码改动（无需新修复）；报告追加单独入册
- **状态**：DONE（F1 已按修复轮 1 主控裁决修复并提交在案 `1c6779c7c`；本轮实核 + 复跑全绿）

## 本轮 findings 判定：与修复轮 1/2 逐字重复（第三次），非新发现

两个 findings 与待核实项列表与修复轮 1、2 的 ask 逐字相同。行号证据再次成立：findings 仍引用 `repository/appconnector/plan.go:117-122`「允许 state IN {awaiting,authorized}」的旧 CAS 形态——本轮实核 HEAD：

```
$ git log --oneline -3
72723074a docs(appconnector): Task 3 修复轮 2 报告入册…
ad888565f docs(appconnector): Task 3 修复轮 1 报告入册…
1c6779c7c fix(appconnector): pin exclusion-set freeze into ApprovePlan CAS, closing concurrent TOCTOU (ruling via escalation)

$ grep -n "state = ? OR excluded_json = ?" internal/modules/appconnector/repository/appconnector/plan.go
129:		Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)",
$ grep -n "state IN ?" internal/modules/appconnector/repository/appconnector/plan.go
（无命中——findings 描述的旧 CAS 形态在 HEAD 不存在）
```

findings 所述 TOCTOU 窗口（CAS 不钉 excluded_json、后写者覆写、可批准先前被排除项）自修复轮 1 起在 HEAD 已不存在。依据修复轮 1 裁决（ledger `plan-t51.md-ledger.md`）与修复轮 2 的同一判定，本轮不重复修复——重复改动只引入回归风险。

## F1 修复持续在案的复跑证据（全部本 ask 实跑）

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApprove -count=1 -race
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.336s
（含 --- PASS: TestPlanApproveBindsDigestAndExclusions / TestPlanApproveCASPinsExclusionSetOnReapproval）

$ go test ./internal/modules/appconnector/plan/ -run TestPlanApprove -count=1 -race
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	3.063s
（含 --- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner）

$ go test ./internal/modules/appconnector/plan/ -count=1 -v
--- PASS: TestPlanDigestBindsSetOrderAndContent (0.00s)
--- PASS: TestPlanFormBuildsOrderedDigestBoundPlan (0.01s)
--- PASS: TestPlanFormMidItemFailureLeavesNoPlanRow (0.03s)
--- PASS: TestPlanFormRejectsInvalidInput (0.00s)
--- PASS: TestPlanApproveWholeApprovesEveryIncludedItem (0.00s)
--- PASS: TestPlanApproveExcludesItemNeverApprovesIt (0.00s)
--- PASS: TestPlanApproveRejectsForeignDigest (0.00s)
--- PASS: TestPlanApproveRejectsBadExclusions (0.00s)
--- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (0.10s)
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	1.167s

$ go test ./internal/modules/appconnector/... -count=1 → 7 包全 ok
（appconnector 0.159s | connectorcontrol 2.151s | openconnector 0.192s | plan 1.952s
  | publish 1.973s | repository/appconnector 0.691s | service/appconnector 3.238s）

$ go build ./... → BUILD_EXIT=0
```

## 待核实项 1 的本轮复核

「上轮证据未独立复跑」——本 ask 第三次复跑确认：plan 包 9/9 `--- PASS`、7 包回归 ok、build exit 0（上方完整输出）。待核实项 2-5（审批谓词归 Task 5、执行侧保证与 AC1 执行面腿归 Task 4、blocked-env、RecoveryExclusionFrozen 归 Task 4 Step 1）依旧无法从本 diff 验证，维持声明，零伪造证据。

## F2（Task 0 未执行）本轮复核：现状未变，仍待编排方收口

```
$ ls migrations/sqlite/ | grep -cE "^000114_"  → 4（双占仍在）
$ grep -n "sqliteAdoptionFKRelaxationMigrationVersion = " internal/database/migration.go
33:const sqliteAdoptionFKRelaxationMigrationVersion = 114
```

按审查定性（跨任务协调项，非 Task 3 缺陷）与修复轮 1 裁决执行要求 5，本修复轮不动 migrations/**；仍需编排方授权执行者完成 Task 0（118/197 空闲，可按计划原文执行），否则 Task 6 的 AC3 全量迁移 e2e 不可运行。

## 给编排方的显式提示（连续三轮相同 findings）

F1 的修复自 commit `1c6779c7c` 起在 HEAD 在案且有裁决背书（ledger `plan-t51.md-ledger.md`）、三轮复跑全绿；F2 的修复权在编排方（需授权 Task 0 执行者）。若后续修复轮仍收到这两条 findings，请编排方核对审查器读入的 HEAD 是否滞后于本分支（`72723074a`），或确认 Task 0 的执行安排——本实现员侧无进一步可修复项。

---

# 修复轮 4 报告（第 4 次接替：findings 第四次逐字重复——根因定位为审查器 diff 基线锚定于修复前提交，本轮新增行号逐行吻合证据）

- **执行者**：实现员-t51-任务3-接替者R4（resumed with findings，修复轮 4/5）
- **提交**：本修复轮零代码改动（无需新修复）；报告追加单独入册
- **状态**：DONE_WITH_CONCERNS（F1 已修复在案 `1c6779c7c` 且本轮独立复核闭环；F2 修复权在编排方；循环未收敛的根因证据见 §1）

## 0. 接替者定位（不盲信前任，独立重审）

前任轮 2/3 已两次以「复跑取证 + 报告重复」应对，第 4 轮 ask 的 findings 仍逐字相同——该策略已被证明不收敛。本轮接替者不复读前任结论，而是：① 独立重读 HEAD 实码推演并发场景找残留缺口；② 检查审查器可能读错代码位置的各假说；③ 把 findings 引用行号与历史提交逐一比对定位根因。结论：**无残留缺口，findings 是基于陈旧 diff 的重复报告**，证据链比前三轮更强（§1）。

## 1. 根因定位：findings 行号与 Task 3 主体提交 `6bdf81aa0` 逐行吻合（本轮新增的决定性证据）

本轮实测（worktree `4256a859f`）：

```
$ git show 6bdf81aa0:internal/modules/appconnector/repository/appconnector/plan.go | sed -n '112,125p'
	Where("tenant_id = ? AND id = ? AND digest = ? AND state IN ?", tenantID, planID, digest,
		[]string{PlanStateAwaitingApproval, PlanStateAuthorized}).
```
findings F1 指认的「`repository/appconnector/plan.go:117-122` 允许 state IN {awaiting,authorized}」与 `6bdf81aa0`（Task 3 主体，修复**前**）的 :117-122 **逐行吻合**。

```
$ git show 6bdf81aa0:internal/modules/appconnector/plan/plan.go | sed -n '276,298p'
	if row.State == PlanStateAuthorized {
		// Recovery re-approval: the exclusion set is frozen at first ...
		if !equalSeqs(recorded, excluded) { ... }
	}
```
findings 指认的「`plan.go:280-296` read-then-act 冻结检查」同样是 `6bdf81aa0` 的形态。

**对照假说逐一排除**：
- 「审查器读主仓」：排除——主仓（main `ab082ad26`）磁盘上不存在这两个文件（`ls /Users/wuyongjun/trea/WeKnora-fork01/internal/modules/appconnector/plan/` → No such file or directory；`git show main:internal/modules/appconnector/repository/appconnector/plan.go` → exists on disk, but not in 'main'）。
- 「修复未提交/被回退」：排除——HEAD `4256a859f` 实码（下方 §2）为钉住形态，`grep "state IN ?" repository/appconnector/plan.go` 零命中。
- **成立假说**：审查上下文锚定在 `6bdf81aa0` 的 diff（Task 3 主体），未纳入其后 `1c6779c7c` 的修复提交。修复轮 1 起每次 findings 都逐字复述修复前代码形态，与此假说完全一致。

## 2. F1 独立复核：HEAD 钉住形态闭环，无残留 TOCTOU（本轮独立推演，非复读前任）

HEAD `4256a859f` 实码（本轮 Read 全文）：

- `internal/modules/appconnector/repository/appconnector/plan.go:129-130`：`Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)", tenantID, planID, digest, PlanStateAwaitingApproval, excludedJSON)`——全参数绑定。
- `plan.go:140-142`：`res.RowsAffected == 0 → ErrPlanState`——单条 UPDATE 语句，sqlite/Postgres 下均原子。
- `internal/modules/appconnector/plan/plan.go:296-298`：service 层 `ApprovePlan` 失败即 `return`——**CAS 失败者不进入 :303-319 逐项循环**，findings 所述「后写者可批准前一次已被排除的项」的路径在 HEAD 已断。

并发场景独立推演（本轮自做，非抄前任报告）：

| 场景 | 推演 | 结果 |
|---|---|---|
| 并发 A(excluded=[]) 先、B(excluded=[2]) 后 | A 走 `state=awaiting` 分支置 authorized+`'[]'`；B 的 WHERE 两分支皆不匹配（state 已 authorized；excluded_json `'[]'`≠`'[2]'`）→ 0 行 → ErrPlanState，不进逐项循环 | item2 停留 awaiting_approval ✓ |
| B 先、A 后 | 对称 | 冻结决定 = 先写者 ✓ |
| 同集合幂等重批（恢复路径，plan-t51.md:64） | authorized + 同 excluded_json → 第二分支匹配 → 1 行重写相同值 | 幂等合法 ✓（store 层 `TestPlanApproveBindsDigestAndExclusions` 在案覆盖） |
| awaiting 行误匹配第二分支？ | awaiting 行 excluded_json=default `''`，而 normalizeExclusions 保证提交值恒为 `'[]'` 或 `[n,...]`——`''` 永不等于 `'[]'` | 无误匹配 ✓ |

## 3. F1/F2 处置与本轮回归取证（全部命令本 ask 实跑）

**F1**：无需新修复（修复在案 `1c6779c7c`，修复轮 1 裁决背书见 ledger `plan-t51.md-ledger.md:7`）。重复改动只会引入回归风险。

**F2**：跨任务协调项，修复权在编排方，本轮不改 migrations/**（纪律：只动本任务授权文件）。现状复核（本轮实跑，未变）：

```
$ ls migrations/sqlite/ | grep -E "^000114_"
000114_mobile_device_app.down.sql / .up.sql
000114_public_agent_marketplace.down.sql / .up.sql     ← 双占仍在
$ grep -n "sqliteAdoptionFKRelaxationMigrationVersion = " internal/database/migration.go
33:const sqliteAdoptionFKRelaxationMigrationVersion = 114   ← 未改
```

**回归覆盖测试（本 ask 实跑命令与输出）**：

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApprove -count=1 -race -v
--- PASS: TestPlanApproveBindsDigestAndExclusions (0.01s)
--- PASS: TestPlanApproveCASPinsExclusionSetOnReapproval (0.00s)
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	2.923s

$ go test ./internal/modules/appconnector/plan/ -run TestPlanApproveConcurrent -count=1 -race -v
--- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (6.06s)
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	14.263s

$ go test ./internal/modules/appconnector/plan/ -count=1 -v      → 9/9 --- PASS
（原 8 测试 + TestPlanApproveConcurrentDistinctExclusionsSingleWinner）
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	3.494s

$ go test ./internal/modules/appconnector/... -count=1           → 7 包全 ok
（appconnector 0.568s | connectorcontrol 11.585s | openconnector 0.630s | plan 9.130s
  | publish 8.076s | repository/appconnector 3.107s | service/appconnector 6.528s）

$ go build ./...                                                 → BUILD_EXIT=0
（仅 macOS ld 既有噪音：ld: warning: ignoring duplicate libraries: '-lc++'）
```

## 4. 待核实项处置（沿用前三轮判定，本轮无可新增验证面）

1. 上轮证据「未独立复跑」→ 本轮已第四次独立复跑：全绿，与前三轮报告声称一致。
2. 审批谓词（发起者或 owner/admin）→ 归 Task 5 wire 层（plan-t51.md:2059-2079），本 diff 无法验证，维持声明。
3. 排除项「永不派发」执行侧保证 + AC1 执行面腿 → 归 Task 4，本 diff 无法验证，维持声明。
4. blocked-env（NOTION_TOKEN 门控的真实 Notion 多操作验收）→ 本环境不可运行，AC3 本地替代证据归 Task 6 且受 F2 阻断，skip 不是 pass，零伪造证据。
5. `TestPlanApproveRecoveryExclusionFrozen` 完整闭环 → 计划明示归 Task 4 Step 1（plan-t51.md:1344）；其 store/service 层可验证部分已由在案测试覆盖（§2 表第 3 行），维持声明。

## 5. 给编排方的终结建议（第 4 轮重复后的升级版）

连续 4 轮 findings 逐字重复、行号始终锚定修复前提交 `6bdf81aa0`，本轮已排除「读主仓」「修复未在案」两假说（§1）。请编排方：

1. **刷新审查基线**：确认审查器 diff 输入包含 `1c6779c7c`（当前 HEAD `4256a859f`）。F1 所述代码形态在 HEAD 不存在，任何基于 `6bdf81aa0` 的重复指认均不构成新的未解决问题。
2. **Task 0 排期**：授权执行者按计划原文完成迁移轨道去重重编（118/197 槽位空闲），否则 Task 6 AC3 e2e 不可运行。
3. 本实现员侧在 Task 3 授权范围内**已无可修复项**：若第 5 轮 findings 仍为此两条且基线未变，建议控制器直接以本报告 §1-§3 证据关闭 findings。

---

# 修复轮 5 报告（第 5 轮接替者 R5：findings 第五次逐字重复——本轮新增变异验证闭环：在案测试对 findings 缺陷形态具确定性杀伤力，HEAD 修复形态即其唯一解）

- **执行者**：实现员-t51-任务3-接替者R5（resumed with findings，修复轮 5/5，最后一轮）
- **提交**：本修复轮零代码改动（无需新修复）；报告追加单独入册
- **状态**：DONE_WITH_CONCERNS（F1 修复在案 `1c6779c7c`，本轮以变异验证一锤定音：findings 所述缺陷形态在 HEAD 不存在、且在案测试能确定性抓红该形态；F2 修复权在编排方）

## 0. R5 接替者定位（独立重审，不盲信前任）

前 4 轮轨迹：轮 1 经主控裁决落地 TOCTOU 修复（`1c6779c7c`）；轮 2/3 复跑取证；R4 定位根因为「审查器 diff 基线锚定修复前提交 `6bdf81aa0`」。本轮 ask 的 findings 与前 4 轮逐字相同。我未复读前任结论，而是：① 独立 Read HEAD 实码重推并发场景找残留缺口；② 复现 findings 行号与历史提交的吻合证据；③ 补做前任从未做过的**变异验证**——把 HEAD 临时回退为 findings 所述旧形态，实证在案测试对该缺陷形态的杀伤力。结论：**无残留缺口，且新增证据链足以终结该循环**（§1-§2）。

## 1. 独立复核：F1 所述代码形态在 HEAD 不存在；findings 建议的修复方向恰是已落地实现

HEAD 实码（本轮 Read 全文核实）：

- `internal/modules/appconnector/repository/appconnector/plan.go:128-130`：CAS WHERE 为钉住形态 `Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)", tenantID, planID, digest, PlanStateAwaitingApproval, excludedJSON)`，全参数绑定；`:140-142` 零行即 `ErrPlanState`。
- `internal/modules/appconnector/plan/plan.go`（Approve）：`ApprovePlan` 返回 err 即 `return`——CAS 失败者**不进入逐项循环**，「后写者批准前次被排除项」的路径在 HEAD 已断。

行号吻合复现（本轮实跑）：

```
$ git show 6bdf81aa0:internal/modules/appconnector/repository/appconnector/plan.go | sed -n '112,125p'
	Where("tenant_id = ? AND id = ? AND digest = ? AND state IN ?", tenantID, planID, digest,
		[]string{PlanStateAwaitingApproval, PlanStateAuthorized}).
$ git show 6bdf81aa0:internal/modules/appconnector/plan/plan.go | sed -n '276,298p'
	if row.State == PlanStateAuthorized { ... !equalSeqs(recorded, excluded) ... }
```

findings F1 引用的 `repository/appconnector/plan.go:117-122`「允许 state IN {awaiting,authorized}」与 `plan.go:280-296` read-then-act 检查均与 `6bdf81aa0`（修复**前**）逐行吻合；HEAD 上 `grep -rn "state IN ?" internal/modules/appconnector/repository/appconnector/ internal/modules/appconnector/plan/` 仅命中无关文件 oc_dispatch.go，plan 两文件零命中。`git log -- internal/modules/appconnector/repository/appconnector/plan.go` 确认最后一次改动即修复提交 `1c6779c7c`。

**本轮新增决定性事实**：findings F1 自己给出的修复建议——「把冻结比对下沉进 ApprovePlan CAS（authorized→authorized 路径在 WHERE 钉住已记录 excluded_json）」——**逐字就是 `1c6779c7c` 已在 HEAD 落地的实现**。审查器在向 HEAD 推荐一个已存在的修复，基线陈旧实锤。

## 2. 本轮新增证据：变异验证闭环（旧 CAS 形态 → 两层测试确定性抓红 → 还原复绿）

变异体（临时改动，已还原）：把 `repository/appconnector/plan.go:128-130` 的 WHERE 精确回退为 findings 所述旧形态 `AND state IN ?` + `[]string{awaiting, authorized}`（与 `6bdf81aa0` 逐字一致）。

```
=== 变异体下：store 层钉住测试（确定性）===
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApproveCASPins -count=1 -v
    plan_test.go:128: CAS must pin the recorded exclusion set — a different set after approval is zero rows, got <nil>
--- FAIL: TestPlanApproveCASPinsExclusionSetOnReapproval (0.00s)
FAIL

=== 变异体下：service 层并发测试（-count=5 增加交错采样）===
$ go test ./internal/modules/appconnector/plan/ -run TestPlanApproveConcurrent -count=5 -v
    plan_test.go:428: round 0: racing approve must fail with a plan-state error, got action_state_conflict
--- FAIL: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (0.01s)   × 5/5 轮全 FAIL
```

解读：变异体（= findings F1 所述缺陷形态）下，authorized 行被不同排除集成功覆写（store 层 `got <nil>`）；service 层并发双 Approve 的输家 CAS 越过、进逐项循环试图批准被赢家排除的项（`action_state_conflict` 即 action 状态机对该越权批准的拒绝）——**5/5 轮确定性抓红**，比修复轮 1 记录的「间歇红」更强。findings 所述 TOCTOU 真实存在于旧形态、且在案测试对它有杀伤力；HEAD 钉住形态是它的修复。

```
=== 还原后（git checkout -- repository/appconnector/plan.go，git status 0 项未提交改动）===
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApprove -count=1
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.559s
$ go test ./internal/modules/appconnector/plan/ -run TestPlanApproveConcurrent -count=3 -race
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/plan	4.038s
```

并发场景独立推演（本轮自做，结论与 HEAD 实码一致）：sqlite 单连接串行化下单条 UPDATE 原子；并发 A(`[]`)/B(`[2]`) 无论谁先 CAS，后写者 WHERE 两分支皆不匹配（state 已 authorized 且 excluded_json 不等）→ 0 行 → ErrPlanState → 不进逐项循环；同集合重批走第二分支幂等合法（plan-t51.md:64 恢复路径）；awaiting 行 excluded_json 默认 `''` ≠ 提交值 `'[]'`，无误匹配。「排除集首次批准后冻结、先写者获胜」在并发下成立。

## 3. F1/F2 处置与本轮回归取证（全部命令本 ask 实跑）

**F1**：无需新修复——修复在案 `1c6779c7c`（修复轮 1 主控裁决授权越界，ledger `plan-t51.md-ledger.md`），本轮以变异验证闭环证明其正确性与测试杀伤力。重复改动只会引入回归风险。

**F2**：跨任务协调项，修复权在编排方（按修复轮 1 裁决执行要求 5，本修复轮不动 migrations/**）。现状复核（本轮实跑，与前任报告一致，未变）：

```
$ ls migrations/sqlite/ | grep -E "^000114_"
000114_mobile_device_app.down.sql / .up.sql / 000114_public_agent_marketplace.down.sql / .up.sql  ← 双占仍在
$ ls migrations/versioned/ | grep -cE "^000193_"  → 4
$ grep -n "sqliteAdoptionFKRelaxationMigrationVersion = " internal/database/migration.go
33:const sqliteAdoptionFKRelaxationMigrationVersion = 114   ← 未改
$ grep -n "000114_public_agent_marketplace" internal/database/migration.go
123: os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")  ← 未改
```

仍需编排方授权执行者完成 Task 0（迁移轨道去重重编，118/197 槽位空闲，可按计划原文执行、Step 0 占用核查先跑），否则 Task 6 的 AC3 全量迁移 e2e（`go test ./internal/handler/ -run TestNotionPublish`）不可运行。

**HEAD 全绿基线复跑（本 ask 实跑）**：

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlanApprove -count=1 -race -v
--- PASS: TestPlanApproveBindsDigestAndExclusions (0.03s)
--- PASS: TestPlanApproveCASPinsExclusionSetOnReapproval (0.01s)
ok  .../repository/appconnector	1.767s

$ go test ./internal/modules/appconnector/plan/ -count=1 -race -v   → 9/9 --- PASS
（TestPlanDigestBindsSetOrderAndContent / TestPlanFormBuildsOrderedDigestBoundPlan /
  TestPlanFormMidItemFailureLeavesNoPlanRow / TestPlanFormRejectsInvalidInput /
  TestPlanApproveWholeApprovesEveryIncludedItem / TestPlanApproveExcludesItemNeverApprovesIt /
  TestPlanApproveRejectsForeignDigest / TestPlanApproveRejectsBadExclusions /
  TestPlanApproveConcurrentDistinctExclusionsSingleWinner (0.72s)）
ok  .../plan	3.802s

$ go test ./internal/modules/appconnector/... -count=1   → 7 包全 ok
（appconnector 0.819s | connectorcontrol 7.032s | openconnector 1.789s | plan 6.331s
  | publish 6.199s | repository/appconnector 2.210s | service/appconnector 6.998s）

$ go build ./...   → BUILD_EXIT=0（仅 macOS ld 既有噪音：duplicate libraries '-lc++'）
```

## 4. 待核实项处置（本轮无可新增验证面，维持前四轮声明）

1. 上轮证据「未独立复跑」→ 本轮第五次独立复跑：plan 包 9/9 PASS、store 层钉住测试 PASS、7 包回归 ok、build exit 0（上方输出），与历轮报告声称一致 ✓
2. 审批谓词（发起者或 owner/admin）→ 归 Task 5 wire 层（plan-t51.md:2059-2079），本 diff 无法验证，维持声明。
3. 排除项「永不派发」执行侧保证 + AC1 执行面腿（TestPlanExecuteRefusesUnapprovedOrForeignDigest）→ 归 Task 4，本 diff 无法验证，维持声明。
4. blocked-env（NOTION_TOKEN 门控的真实 Notion 多操作验收）→ 本环境不可运行；AC3 本地替代证据归 Task 6 且受 F2 阻断——skip 不是 pass，零伪造证据，维持声明。
5. `TestPlanApproveRecoveryExclusionFrozen`（同 digest 幂等重批完整闭环）→ 计划明示归 Task 4 Step 1（plan-t51.md:1344）；其可验证部分（store 层幂等重批 + service 层同集合并发幂等）已在案覆盖（§2 推演第 3 行），维持声明。

## 5. 给编排方的终结建议（第 5 轮重复后的最终版）

连续 5 轮 findings 逐字重复、行号始终锚定 `6bdf81aa0`；本轮已具备三层证据：① findings 引用的代码形态在 HEAD 零存在（§1 grep）；② findings 自己建议的修复方向恰是 HEAD 已落地实现（§1）；③ 变异验证证明 findings 所述缺陷形态被在案测试确定性抓红、HEAD 形态复绿（§2）。请编排方：

1. **刷新审查基线**：审查器 diff 输入必须包含 `1c6779c7c`（当前 HEAD 链）。基于 `6bdf81aa0` 的 F1 重复指认不构成新的未解决问题——其建议的修复已在 HEAD，且有变异验证背书。
2. **Task 0 排期**：授权执行者按计划原文完成迁移轨道去重重编，否则 Task 6 AC3 e2e 不可运行（F2，第 5 次记录）。
3. 本实现员在 Task 3 授权范围内**已无可修复项**：若后续仍收到此两条 findings 且基线未变，请控制器直接以本报告 §1-§3 证据关闭 findings——第 6 轮再接替只会重复本轮工作。

---

# T21 #51 Task 4 实施报告：plan 包——Execute 与 Status（AC2 部分成功恢复）

- **执行者**：实现员-t51-任务4（subagent-driven-development 实现员，TDD）
- **Worktree**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`）
- **任务定位**：Issue #51 实施计划 7 任务中的第 4/7 个（`docs/plans/issue30-sweep/plans/plan-t51.md` 行 1450-1790）
- **提交**：`828b9428c` feat(appconnector): plan Execute/Status——部分成功恢复 AC2（T21 #51 Task 4）
- **状态**：DONE
- **前置接口消费确认**：Task 3 的 `Approve`（HEAD 修复形态 `1c6779c7c`）零改动直接消费；本提交 diff 证实对 plan.go 的改动仅为替换原 `plan.go:323-324` 两行占位注释。

---

## 1. 实现内容

### 1.1 生产代码：`internal/modules/appconnector/plan/plan.go`

把 Task 2 落下的两个占位注释（原 `plan.go:323-324`）替换为真实实现，与计划 Task 4 Step 3 给定代码逐字一致：

- **`Execute(ctx, tenantID, planID, digest) (ExecuteOutcome, error)`**——一次有序执行趟：
  - 前置门（顺序固定）：`FindPlan` 跨租户统一 not-found → digest 不匹配 `ErrPlanDigestMismatch`（AC1 执行面）→ 计划非 `authorized` 拒绝 `ErrPlanState`（未批准计划永不执行）。
  - 逐项按 seq 升序分派 disposition：

    | 动作行状态 | disposition | 语义 |
    |---|---|---|
    | 排除集命中 | `excluded` | 永不派发 |
    | `succeeded` | `skipped_succeeded` | AC2：已确认结果零重发 |
    | `authorized` | `executed` | 唯一派发路径（恢复窗口），走 `publish.Execute` |
    | `awaiting_approval` | `skipped_unapproved` | fail closed，零派发 |
    | `queued`/`dispatched` | `skipped_in_flight` | 活跃写者持有 |
    | `failed`/`unknown` | `settled` | 确认终态绝不重派（重发需新计划→AC1） |

  - 一项失败/未知**不阻断**后续项（逐项独立外部副作用，CONTEXT.md「执行结果仍逐项持久记录」）。
  - 非 executed 项做 `publish.Receipt` 投影（无 publication 行的项容忍为空视图，`if rerr == nil`）。

- **`Status(ctx, tenantID, planID) (PlanStatus, error)`**——持久投影：计划身份（复用 Task 3 的 `view`）+ 冻结排除集（`parseExclusions`）+ 逐项权威动作状态与回执。行动行是唯一权威，计划层只投影，无存储终态。

### 1.2 测试：`internal/modules/appconnector/plan/plan_test.go`（追加 185 行）

六个测试逐字来自计划 Task 4 Step 1：

1. `TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts`——一趟执行：逐项有序、各项结算 published 回执、每项恰好派发 1 次。
2. `TestPlanExecuteSkipsConfirmedOutcomesOnResume`——**AC2 核心（Review Focus 1）**：[成功/失败(版本冲突)/未知(传输丢失)] 三项部分成功后，同 digest 恢复趟 `skipped_succeeded`/`settled`/`settled`，dispatch 计数断言每项全程恰好 1 次。
3. `TestPlanExecuteRefusesUnapprovedOrForeignDigest`——未批准计划拒绝执行（`ErrPlanState`）、异源 digest 拒绝（AC1 执行面，Review Focus 2 的执行腿）、未知计划 not-found。
4. `TestPlanExecuteSkipsUnapprovedItemFailClosed`——partial-approve 窗口（单项批准缺失）该项 `skipped_unapproved` 零派发（Review Focus 4）。
5. `TestPlanStatusProjectsPerItemResults`——投影：计划身份 + 冻结排除集 `[2]` + 逐项权威状态与回执。
6. `TestPlanApproveRecoveryExclusionFrozen`——执行开始后排除集冻结：改集 `ErrPlanState`，同集重批合法（恢复路径）。Review Focus 3 的排除集冻结腿（按计划 plan-t51.md:1629 归入本任务）。

**前置接口在开工前逐一核对**，与计划 Consumes 描述一致：

- `ActionStore.SetActionState(ctx, id, from, to string) error`（`repository/appconnector/action.go:149`）
- `DispatchOutcome{Status, ProviderResult, ExecutionID}`（`service/appconnector/action.go:110`）
- `publish.PublishVersionConflictResult = "notion_version_conflict"`（`publish/dispatcher.go:20`）
- `appconn` 状态常量七个（`appconnector/action.go:38-44`）
- `NotionPublishService.Execute`（`publish/plan.go:252`）/ `.Receipt`（`publish/plan.go:271`）/ `PublishExecuteOutcome{ActionState, Conflict, Receipt}`（`publish/plan.go:98`）

## 2. TDD 证据

### RED（实现前，实跑输出原样）

命令：

```
go test ./internal/modules/appconnector/plan/ -run 'TestPlanExecute|TestPlanStatus|TestPlanApproveRecovery' -count=1
```

输出：

```
# github.com/Tencent/WeKnora/internal/modules/appconnector/plan [github.com/Tencent/WeKnora/internal/modules/appconnector/plan.test]
internal/modules/appconnector/plan/plan_test.go:472:20: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:509:20: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:523:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:549:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:553:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:556:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:575:20: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:603:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:606:19: e.svc.Status undefined (type *Service has no field or method Status)
internal/modules/appconnector/plan/plan_test.go:634:21: e.svc.Execute undefined (type *Service has no field or method Execute)
internal/modules/appconnector/plan/plan_test.go:634:21: too many errors
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/plan [build failed]
```

失败原因符合预期：`Execute`/`Status` 尚未定义（占位注释仍在）。

### GREEN（实现后，实跑输出原样）

命令：

```
go test ./internal/modules/appconnector/plan/ -count=1 -v
```

输出（`--- PASS` 全列表；gorm SQL trace 噪音略，见 §4.1）：

```
--- PASS: TestPlanDigestBindsSetOrderAndContent (0.00s)
--- PASS: TestPlanFormBuildsOrderedDigestBoundPlan (0.01s)
--- PASS: TestPlanFormMidItemFailureLeavesNoPlanRow (0.02s)
--- PASS: TestPlanFormRejectsInvalidInput (0.00s)
--- PASS: TestPlanApproveWholeApprovesEveryIncludedItem (0.00s)
--- PASS: TestPlanApproveExcludesItemNeverApprovesIt (0.01s)
--- PASS: TestPlanApproveRejectsForeignDigest (0.01s)
--- PASS: TestPlanApproveRejectsBadExclusions (0.01s)
--- PASS: TestPlanApproveConcurrentDistinctExclusionsSingleWinner (0.16s)
--- PASS: TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts (0.01s)
--- PASS: TestPlanExecuteSkipsConfirmedOutcomesOnResume (0.01s)
--- PASS: TestPlanExecuteRefusesUnapprovedOrForeignDigest (0.00s)
--- PASS: TestPlanExecuteSkipsUnapprovedItemFailClosed (0.01s)
--- PASS: TestPlanStatusProjectsPerItemResults (0.00s)
--- PASS: TestPlanApproveRecoveryExclusionFrozen (0.02s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	2.267s
```

15/15 PASS（9 个既有 + 6 个本任务新增）。

### 全包回归（计划 Task 4 Step 4 的验证命令，实跑输出原样）

命令：

```
go build ./... && go test ./internal/modules/appconnector/... -count=1
```

输出：

```
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.385s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	2.882s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.495s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/plan	1.643s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	2.113s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.760s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.573s
```

7 个包全部 ok（appconnector root/connectorcontrol/openconnector/plan/publish/repository/service），与计划 Step 4 Expected 一致。`ld: warning` 为 macOS 链接器对 cgo 重复库的系统告警（`go build` 退出码 0），与本次代码无关。

## 3. 提交

```
828b9428c feat(appconnector): plan Execute/Status——部分成功恢复 AC2（T21 #51 Task 4）
 internal/modules/appconnector/plan/plan.go      | 120 ++++++++++++++-
 internal/modules/appconnector/plan/plan_test.go | 185 ++++++++++++++++++++
 2 files changed, 303 insertions(+), 2 deletions(-)
```

改动范围 = 本任务授权的两个文件，零溢出；提交后 `git status` 干净。

## 4. 自检发现

1. **gorm SQL trace 噪音（非缺陷）**：`-v` 输出中 `publication.go:79 record not found` 是 `Execute`/`Status` 对无 publication 行的项做 `Receipt` 探测的 gorm 日志（代码按计划用 `if rerr == nil` 容忍为空视图）；非测试失败，非本任务引入。如需纯净输出可另调 gorm logger 级别，不在本任务授权范围。
2. **占位卫兵残留核查**：Task 2 在 plan.go 尾部落了 `var _ = appconn.ActionAwaitingApproval` / `var _ = time.Now` 占位卫兵，计划（plan-t51.md:1434）要求 Task 3 删除——本任务开工前核实当前 HEAD 无残留，无需处理。
3. **Execute 中 `s.actions.FindAction` 出错会中断整趟**：与计划给定实现逐字一致（action 行丢失属数据完整性故障，fail fast 优于静默跳过——计划裁定，未擅改）。
4. **报告契约核对**：对照 `implementer-prompt.md`（6.4.2 版；ask 给的 6.4.1 路径不存在，插件缓存实际只有 6.4.2）——本报告含实现内容/TDD 证据/测试命令与完整输出/文件变更/自检发现；短契约见 `submit_result`。
5. **报告文件为累积式**：本报告追加在 Task 2/Task 3（五轮修复）报告之后，未删改前序内容。

## 5. 边界与后续

- 本任务未触碰 Task 5（HTTP 面）/Task 6（e2e）的任何文件；Task 6 的 `TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly` 将在本任务的 AC2 语义之上做全链验证。
- blocked-env 声明沿用计划总则：真实 Notion 凭据验收本地不可运行；本任务为服务层单测（契约双打 dispatch 脚本化），无需凭据，无 skip。
- Task 3 R5 报告 §4 第 3 条「归 Task 4」的两项（排除项执行侧永不派发 + AC1 执行面腿）已由本任务 `TestPlanExecuteSkipsUnapprovedItemFailClosed`/`TestPlanExecuteRefusesUnapprovedOrForeignDigest`/`TestPlanApproveExcludesItemNeverApprovesIt`+`Execute` 排除分支闭环。

---

# T21 #51 Task 5 实施报告：HTTP 面——handler、路由与容器接线

- **执行者**：实现员-t51-任务5（subagent-driven-development 实现员，TDD）
- **Worktree**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`）
- **提交**：`60d783afb` `feat(appconnector): action-plans HTTP 面 + 路由 + 容器双输出接线（T21 #51 Task 5）`
- **状态**：DONE_WITH_CONCERNS（预存在跨任务项 F2 未收口，与本任务 diff 无因果，见 §7）

---

## 1. 实现内容

按计划 Task 5 逐字落地，6 文件 +453/-2：

`internal/handler/app_connector_action_plan.go`（新建，271 行）：

- `AppActionPlanHandler{db *gorm.DB; plans *plan.Service}` + `NewAppActionPlanHandler(db)` + `SetActionPlanService(s *plan.Service)`（`app_connector_action_plan.go:27-45`）——nil-service fail-closed，与 #48 兄弟 handler 同判例
- `RequireActionCapabilityForWrites()`（`:48-54`）：`appRequireWriteCapability(appconnector.CanDriveActionWrites, "FORBIDDEN_ACTION_WRITE", ...)` 复用既有写门（GET/HEAD/OPTIONS 天然放行，`app_connector.go:60`）
- 四端点：`FormActionPlan`（201 PlanView）、`ApproveActionPlan`（200 PlanView，审批谓词=发起者或 owner/admin，`row.ActorID != userID && !CanDriveActionWrites(role)` → 403 `ACTION_APPROVAL_FORBIDDEN`）、`ExecuteActionPlan`（200 ExecuteOutcome，执行面 AC1 digest 核对下沉 `plan.Service.Execute`）、`GetActionPlan`（200 PlanStatus）
- `planByID`（`:70-79`）：`Where("tenant_id = ? AND id = ?")` 参数绑定，跨租户与不存在统一 404 `ACTION_PLAN_NOT_FOUND`——零存在性泄漏（Review Focus #5 wire 腿）
- `failForm`（`:194-218`）：formation 侧沿用 #48 publish 哨兵全表（`ARTIFACT_VERSION_NOT_ACCESSIBLE`/`PUBLISH_UNSUPPORTED_ARTIFACT`/`PUBLISH_CONTENT_TOO_LARGE`/`PUBLISH_EMPTY_CONTENT`/`PUBLISH_DESTINATION_OUT_OF_SCOPE`/`PUBLISH_DESTINATION_UNREADABLE`/`PUBLISH_UPDATE_TARGET_NOT_PUBLISHED`）
- `failPlan`（`:220-246`）：`ACTION_PLAN_NOT_FOUND`(404)/`ACTION_PLAN_DIGEST_MISMATCH`(409，AC1 wire 腿)/`ACTION_PLAN_STATE_CONFLICT`(409)/`ACTION_DIGEST_MISMATCH`(409)/`ACTION_STATE_CONFLICT`(409)/`OC_DISPATCH_NOT_CONFIGURED`(503)；消息全静态，上游错误文本与 Provider 细节不过 wire

`internal/router/routes_app_action_plan.go`（新建，26 行）：`RegisterAppActionPlanRoutes(r, h)`——nil handler 静默返回（镜像 #48）；组级挂 `RequireActionCapabilityForWrites()`；四路由 `POST ""`、`POST /:id/approve`、`POST /:id/execute`、`GET /:id`。不在 API-key 路由授权器声明（`/api/v1` 门对 X-API-Key 主 体默认拒绝，与 #48 注释同语义）。

`internal/router/router.go`（修改 2 处）：`RouterParams` 增 `AppActionPlanHandler *handler.AppActionPlanHandler`（`AppNotionPublishHandler` 之后，:150-153）；注册点在 `RegisterAppNotionPublishRoutes`（:440）后追加 `RegisterAppActionPlanRoutes(v1, params.AppActionPlanHandler)`。

`internal/container/notion_publish.go`（修改 3 处）：import 增 `plan` 包；`newNotionPublishHandler` 签名扩为双输出 `(*handler.AppNotionPublishHandler, *handler.AppActionPlanHandler, error)`；尾部构造 `planSvc := plan.NewService(repoappconn.NewPlanStore(db), store, actions, svc)`——与 publish 服务共用同一 `ActionService` 实例（`actions`）与同一 `ActionStoreSource`（`store`），A03 权威零分叉。`container.go:1013` 的 `container.Provide(newNotionPublishHandler)` 调用点零改动（dig 原生多输出）。

测试（计划逐字）：`internal/handler/app_connector_action_plan_test.go`（2 测试）+ `internal/router/routes_app_action_plan_test.go`（2 测试）。

## 2. 测试命令与完整输出

### RED（实现前）

```
$ go test ./internal/handler/ -run TestActionPlan -count=1
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connector_action_plan_test.go:32:44: undefined: AppActionPlanHandler
internal/handler/app_connector_action_plan_test.go:67:7: undefined: NewAppActionPlanHandler
internal/handler/app_connector_action_plan_test.go:94:7: undefined: NewAppActionPlanHandler
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
```

与计划 Step 2 预期逐字一致（`AppActionPlanHandler` 未定义）。

### GREEN（计划 Step 4 命令，verbose 取证）

```
$ go test ./internal/handler/ -run TestActionPlan -count=1 -v
=== RUN   TestActionPlanHandlerFailClosedWithoutService
--- PASS: TestActionPlanHandlerFailClosedWithoutService (0.01s)
=== RUN   TestActionPlanHandlerValidationAndNotFound
--- PASS: TestActionPlanHandlerValidationAndNotFound (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	4.120s

$ go test ./internal/router/ -run TestActionPlanRoutes -count=1 -v
=== RUN   TestActionPlanRoutesNilHandlerRegistersSilently
--- PASS: TestActionPlanRoutesNilHandlerRegistersSilently (0.00s)
=== RUN   TestActionPlanRoutesRegisterWithHandler
--- PASS: TestActionPlanRoutesRegisterWithHandler (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	2.214s
```

handler 2 + router 2，4/4 PASS。

### 计划 Step 6 全量检查（接线后）

```
$ go build ./...
（exit 0；仅预存在链接器警告：ld: warning: ignoring duplicate libraries: '-lc++' ×2）

$ go test ./internal/handler/ -run 'TestActionPlan' -count=1 -v
--- PASS: TestActionPlanHandlerFailClosedWithoutService (0.01s)
--- PASS: TestActionPlanHandlerValidationAndNotFound (0.01s)
ok  	github.com/Tencent/WeKnora/internal/handler	12.471s

$ go test ./internal/router/ -run TestActionPlanRoutes -count=1 -v
--- PASS: TestActionPlanRoutesNilHandlerRegistersSilently (0.00s)
--- PASS: TestActionPlanRoutesRegisterWithHandler (0.00s)
ok  	github.com/Tencent/WeKnora/internal/router	18.960s

$ go vet ./internal/container/ ./internal/router/ ./internal/handler/
VET-CLEAN（exit 0，零输出）
```

### 额外回归（超出 ask 的自检）

```
$ go test ./internal/modules/appconnector/... -count=1
ok  github.com/Tencent/WeKnora/internal/modules/appconnector	                            2.727s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	            16.060s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	                2.546s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/plan	                        11.169s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/publish	                    11.037s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	    4.483s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	        13.940s
```

前置 Task 1-4 全部测试（含 TestPlanExecute*/TestPlanStatus*/TestPlanApproveRecoveryExclusionFrozen）零回归。

## 3. 额外回归中发现的预存在失败（非本任务因果，证据在案）

额外跑了 `go test ./internal/handler/ -run 'TestNotionPublish' -count=1`（#48 e2e，属 Task 6 验证面），3 个 e2e FAIL：

```
--- FAIL: TestNotionPublishEndToEndCreateApprovePublishReceipt (0.03s)
--- FAIL: TestNotionPublishEndToEndUpdateConflict (0.03s)
--- FAIL: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (0.04s)
Error: Received unexpected error:
    failed to open source, "file:///…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
```

归因证据链：
1. `git status --short`（实跑）：本任务改动仅 `internal/container/notion_publish.go`、`internal/router/router.go` 与 4 个新文件——`migrations/` 零触碰，golang-migrate 读到的目录内容与 HEAD 完全一致；
2. 目录实跑 `ls migrations/sqlite/ | grep -E "00011[4-9]"`：`000114_mobile_device_app.*` 与 `000114_public_agent_marketplace.*` 并存（versioned `000193` 同理双占），且 `000119_app_action_plans.*`（Task 1 已落）在场——即本分支 Task 0（public_agent_marketplace 重编 000118/000197）未执行；
3. `git log --all | grep 重编`：存在的重编提交（`4b36054fe`/`bba8ce8db`/`34849775b`）均属其他分支（T20 #53、T17 #47、plan-t62，挪的是 mobile_device_app），本分支历史无 Task 0 提交；
4. Task 4 报告 F2 已在案定性与裁决：「跨任务协调项，非 Task 3/4 缺陷」「本修复轮不动 migrations/**；需编排方授权执行者完成 Task 0」——本任务遵守同一裁决，不越权代做。

结论：该失败是 Task 0 未收口的预存在波级问题（计划 Tech Stack 节预判的原样复现），与本任务 diff 无因果；Task 6 的 AC3 e2e 在 Task 0 完成前不可运行（同 Task 4 报告 F2 结论）。

## 4. 接线完整性核查（dig 双输出）

- `grep NewAppActionPlanHandler` 全仓（非测试）：唯二命中=定义（`app_connector_action_plan.go:28`）+ 双输出构造器内调用（`notion_publish.go:58`）——`*handler.AppActionPlanHandler` 全仓唯一 Provide 点，无双提供 panic 风险；
- `RouterParams` 由 dig 经 `container.Provide(router.NewRouter)`（`container.go:1068`）运行时解析，新字段由双输出构造器自动注入；
- `go build ./...` exit 0 + `go vet` 三包干净（上方完整输出）。

## 5. 计划未列的偏差

无。计划 Task 5 的 Files/Interfaces/测试代码逐字落地；无占位遗留（本任务无占位义务）。

## 6. 自检发现（findings）

- F1（info）：`ApproveActionPlan` 的 501 fail-closed 检查位于审批谓词（403）之后——即 nil-service 时先做 403 谓词再 501。这是计划实现原文的顺序（计划 :2063-2093 逐字落地），测试未覆盖该顺序组合，行为无安全缺口（403 谓词不写任何状态，501 仍兜底），如实记录不擅改。
- F2（跨任务，非本任务阻塞）：见 §3——Task 0 未在本分支执行，migrations 双占（sqlite 000114/versioned 000193）仍在；118/197 槽位实核空闲（`ls` 输出无 000118/000197 文件），Task 0 可按计划原文执行。**需编排方授权执行者完成 Task 0，否则 Task 6 AC3 全量迁移 e2e 不可运行。**

## 7. 范围边界

- 未触碰：`migrations/**`、`internal/database/migration.go`（Task 0 授权面）、`publish/**`、`service/appconnector/**`、既有 `/apps/notion-publish/*` 与 `/apps/actions/*` 端点（#48/A03 冻结面）；
- blocked-env：真实 Notion 多操作验收（NOTION_TOKEN 门控）本地不可运行，维持计划总则声明，零伪造证据；AC3 本地替代证据归 Task 6（且其全量迁移 e2e 受 §3/F2 前置约束）。

---

# Task 5 修复轮 1 报告（findings 复核：唯一 finding 为跨任务预存在项 F2，按既定裁决不代做，本轮复跑取证 + 待核实项逐项回应）

- **执行者**：实现员-t51-任务5（修复轮 1/5）
- **基线**：`521c16f62`（Task 5 docs 入册）→ 本轮 docs 提交见文末
- **findings 处理定性**：唯一 finding（important）= Task 0 未执行（跨任务预存在项，非 Task 5 缺陷）。依 Task 4 报告 F2 既有裁决——「跨任务协调项……本修复轮不动 migrations/**；需编排方授权执行者完成 Task 0」——本轮零代码改动，全部动作为实跑复核取证。

## 1. F2 复核（本轮实跑，现状未变）

1. 双占仍在（`ls migrations/sqlite/ | grep -E "00011[4-9]"` 实跑输出）：
   ```
   000114_mobile_device_app.down.sql / .up.sql
   000114_public_agent_marketplace.down.sql / .up.sql   ← 双占
   000115_app_publications / 000116_task_compliance / 000117_code_deliveries
   000119_app_action_plans（Task 1 已落，在场）
   ```
   versioned 同构：`000193_mobile_device_app.*` × `000193_public_agent_marketplace.*` 双占，`000194/195/196` 在场，`000198_app_action_plans`（Task 1 已落）在场。
2. 门控常量未改（`grep` 实跑）：`internal/database/migration.go:33` 仍为 `const sqliteAdoptionFKRelaxationMigrationVersion = 114`，`:123` 探测串仍指 `000114_public_agent_marketplace.up.sql`。
3. `go test ./internal/handler/ -run 'TestNotionPublish' -count=1` 复跑（本轮实跑）：
   ```
   --- FAIL: TestNotionPublishEndToEndCreateApprovePublishReceipt (0.09s)
   --- FAIL: TestNotionPublishEndToEndUpdateConflict (0.04s)
   --- FAIL: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (0.21s)
   failed to open source, "file:///…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
   ```
   与审查 finding 报错逐字一致。
4. 槽位复核（本轮实跑）：sqlite `000118` 无任何文件、versioned `000197` 无任何文件——**Task 0 可按计划原文执行**。审查所指 `000118_mcp_oauth_binding_states` 仅占 versioned 轨道的 000118 号段（本轮 `ls migrations/versioned/ | grep -E "000118|000119"` 证实其在场），与本计划 Task 0 目标号（sqlite 000118 / versioned 000197）不同段不冲突；sqlite 000119 亦无文件占用（Task 1 新表已用 sqlite 000119 落位，编号顺延条款不触发）。
5. Task 5 提交零触碰证据（`git show --stat 60d783afb` 实跑）：仅 6 个授权文件（container/notion_publish.go、router/router.go、handler×2、router×2），migrations/** 零命中。

## 2. 待核实项逐项回应（本轮实际动作与结论）

1. **dig 运行时装配**：维持未冒烟。核实的边界：`BuildContainer`（`internal/container/container.go:150`）仅被 `cmd/server/main.go:59` 与 `cmd/desktop/main.go:181` 调用，完整 Invoke 依赖 `config.LoadConfig`/`initDatabase`/`initRedisClient` 等真实基础设施；container 包 23 个既有测试均为单元级（无整容器 Invoke 先例），本 ask 未授权起 server 或新增验证文件。在案证据维持：`go build ./...` exit 0（双输出签名编译期自证 `RouterParams` 注入面匹配）+ `go vet` 三包干净 + `*handler.AppActionPlanHandler` 全仓唯一 Provide 点（`grep NewAppActionPlanHandler` 实核）+ `newNotionPublishHandler` 为仓库首个多输出 dig 构造器（本轮 grep 实核无先例），dig v1.19.0（`go.mod:79`）多输出为框架原生能力。运行时冒烟留待有环境的验证面（如 Task 6 e2e 或部署冒烟）。
2. **Task 6 e2e 不存在**：属实，`app_connector_action_plan_e2e_test.go` 属 Task 6 授权面，本 ask 不含；且其全量迁移前置受 F2 阻断（§1.3 复跑在案）。无法验证，如实声明。
3. **真实 Notion blocked-env**：属实，维持计划总则声明（NOTION_TOKEN/NOTION_PARENT_PAGE_ID 本环境不存在），零伪造。
4. **Task 0 落地后的 `go test ./internal/database/ -count=1`**：未运行（Task 0 未发生，无可验证对象）；Task 0 由编排方授权执行者完成后应跑该命令收口。

## 3. 修复轮回归（本任务在案测试复跑，零回归）

```
$ go test ./internal/handler/ -run 'TestActionPlan' -count=1 -v
--- PASS: TestActionPlanHandlerFailClosedWithoutService (0.10s)
--- PASS: TestActionPlanHandlerValidationAndNotFound (0.01s)
ok  	github.com/Tencent/WeKnora/internal/handler	10.225s

$ go test ./internal/router/ -run TestActionPlanRoutes -count=1 -v
--- PASS: TestActionPlanRoutesNilHandlerRegistersSilently (0.00s)
--- PASS: TestActionPlanRoutesRegisterWithHandler (0.00s)
ok  	github.com/Tencent/WeKnora/internal/router	4.981s
```

## 4. 结论与移交

- Task 5 交付面（handler/路由/接线/4 测试）本轮零改动、零回归。
- **需编排方收口**：授权 Task 0 执行者按计划原文执行迁移去重重编（sqlite 000114→000118、versioned 000193→000197、migration.go :29-33 与 :123 联动）——槽位已核实空闲（§1.4）；完成后 Task 6 的 AC3 全量迁移 e2e 方可运行。

---

# Task 5 修复轮 2 报告（findings 与修复轮 1 逐字相同——F2 跨任务项复核现状未变，本轮复跑取证 + 槽位增量核查，仍待编排方收口）

- **执行者**：实现员-t51-任务5（修复轮 2/5）
- **基线**：`1b4a1bf3c`（修复轮 1 入册）→ 本轮 docs 提交见文末
- **定性**：唯一 finding 与修复轮 1 逐字相同（Task 0 未执行的跨任务预存在项，非 Task 5 缺陷）。依 Task 4 报告 F2 既有裁决不代做 migrations/**；本轮全部动作为实跑复核取证，零代码改动。

## 1. F2 复跑（本轮实跑，现状与修复轮 1 完全一致）

1. 双占仍在：`ls migrations/sqlite/ | grep -E "^000114"` → `000114_mobile_device_app.{up,down}.sql` × `000114_public_agent_marketplace.{up,down}.sql` 并存；versioned 同构（`000193_mobile_device_app.*` × `000193_public_agent_marketplace.*`）。
2. 门控常量未改：`grep` 实跑 `internal/database/migration.go:33` 仍为 `const sqliteAdoptionFKRelaxationMigrationVersion = 114`，`:123` 探测串仍指 `000114_public_agent_marketplace.up.sql`。
3. `go test ./internal/handler/ -run 'TestNotionPublish' -count=1` 复跑：3 个 `--- FAIL`（CreateApprovePublishReceipt / UpdateConflict / UnknownReconcilesRemoteFirst），报错均 `duplicate migration file: 000114_public_agent_marketplace.down.sql`——与 finding 逐字一致。
4. **槽位增量核查（本轮新增价值：防同批兄弟计划瞬时落盘占用，Task 4 报告曾记录该现象）**：
   ```
   $ ls migrations/sqlite/ | grep -E "^000118"   → 无输出（exit 1，无占用）
   $ ls migrations/versioned/ | grep -E "^000197" → 无输出（exit 1，无占用）
   ```
   sqlite 000118 / versioned 000197 仍空闲，Task 0 可按计划原文执行、无需顺延。

## 2. 待核实项（4 项与修复轮 1 相同，逐项维持）

1. dig 运行时装配：维持未冒烟（`BuildContainer` 仅 cmd/server、cmd/desktop main 调用，需真实 config/DB/redis 环境；本 ask 未授权起 server 或新增验证文件）。静态+编译证据链不变（build exit 0、vet 三包干净、`*handler.AppActionPlanHandler` 全仓唯一 Provide 点、dig v1.19.0 多输出框架原生能力）。
2. Task 6 e2e 不存在（Task 6 授权面）且受 F2 阻断——无法验证，如实声明。
3. 真实 Notion blocked-env 维持计划总则声明，零伪造。
4. `go test ./internal/database/ -count=1` 未运行（Task 0 未发生无可验证对象）——应由 Task 0 执行者收口时运行。

## 3. 修复轮回归（本任务在案测试复跑，零回归）

```
$ go test ./internal/handler/ -run 'TestActionPlan' -count=1 -v
--- PASS: TestActionPlanHandlerFailClosedWithoutService (0.01s)
--- PASS: TestActionPlanHandlerValidationAndNotFound (0.00s)
ok  	github.com/Tencent/WeKnora/internal/handler	2.304s

$ go test ./internal/router/ -run TestActionPlanRoutes -count=1 -v
--- PASS: TestActionPlanRoutesNilHandlerRegistersSilently (0.00s)
--- PASS: TestActionPlanRoutesRegisterWithHandler (0.00s)
ok  	github.com/Tencent/WeKnora/internal/router	3.330s
```

## 4. 结论

Task 5 交付面零改动、零回归。F2 仍为唯一未决项且其修复者非本任务：**需编排方授权执行者完成 Task 0**（sqlite 000114→000118、versioned 000193→000197、migration.go :29-33 注释与 :33 常量、:123 探测串、sqlite up 文件自引用注释五处联动——槽位本轮再证空闲）。完成后 Task 6 的 AC3 全量迁移 e2e 方可运行。

---

# Task 5 修复轮 3 报告（findings 第三次逐字重复——F2 跨任务项复核现状未变，本轮复跑取证，仍待编排方收口）

- **执行者**：实现员-t51-任务5（修复轮 3/5）
- **基线**：`8af01967a`（修复轮 2 入册）→ 本轮 docs 提交见文末
- **定性**：唯一 finding 第三次逐字重复（Task 0 未执行的跨任务预存在项，非 Task 5 缺陷；Task 3 修复轮 2-5 曾以同模式处理同类重复 findings，均以复跑取证 + 不代做收口）。依 Task 4 报告 F2 既有裁决不代做 migrations/**，本轮零代码改动。

## 1. F2 复跑（本轮实跑，现状与修复轮 1/2 完全一致）

1. 双占仍在：sqlite `000114_mobile_device_app.{up,down}.sql` × `000114_public_agent_marketplace.{up,down}.sql` 并存；versioned `000193_mobile_device_app.*` × `000193_public_agent_marketplace.*` 并存（`ls` 实跑输出）。
2. 门控常量未改：`internal/database/migration.go:33` 仍为 `const sqliteAdoptionFKRelaxationMigrationVersion = 114`（`grep` 实跑）。
3. `go test ./internal/handler/ -run 'TestNotionPublish' -count=1` 复跑：`--- FAIL` 计数 = **3**（grep -c 实跑），报错仍为 `duplicate migration file: 000114_public_agent_marketplace.down.sql`（grep -m1 实跑）——与 finding 逐字一致。
4. 槽位核查：sqlite `000118` 无文件、versioned `000197` 无文件（`ls | grep -E "^000118|^000193|^000197"` 输出仅含 000114/000193 双占对）——Task 0 可按计划原文执行。

## 2. 待核实项（4 项，逐项维持前两轮结论，无新事实）

1. dig 运行时装配维持未冒烟（需真实 config/DB/redis 环境，本 ask 未授权起 server；静态+编译证据链不变）。
2. Task 6 e2e 不存在（Task 6 授权面）且受 F2 阻断——无法验证。
3. 真实 Notion blocked-env 维持声明，零伪造。
4. `go test ./internal/database/ -count=1` 未运行（Task 0 未发生无可验证对象）。

## 3. 修复轮回归（本任务在案测试复跑，零回归）

```
$ go test ./internal/handler/ -run 'TestActionPlan' -count=1 -v
--- PASS: TestActionPlanHandlerFailClosedWithoutService (0.01s)
--- PASS: TestActionPlanHandlerValidationAndNotFound (0.00s)
ok  	github.com/Tencent/WeKnora/internal/handler	2.679s

$ go test ./internal/router/ -run TestActionPlanRoutes -count=1 -v
--- PASS: TestActionPlanRoutesNilHandlerRegistersSilently (0.00s)
--- PASS: TestActionPlanRoutesRegisterWithHandler (0.00s)
ok  	github.com/Tencent/WeKnora/internal/router	5.555s
```

## 4. 结论

Task 5 交付面零改动、零回归。F2 三轮复核结论稳定：其修复者非本任务——**需编排方授权执行者完成 Task 0**（sqlite 000114→000118、versioned 000193→000197、migration.go :29-33 与 :123 联动；槽位三轮均证空闲）。审查循环若继续把同一跨任务项发回实现员，建议编排方直接派发 Task 0 专项执行者以打破循环（Task 3 修复轮 1-5 的同类先例已证明重复回发不产生新事实）。

---

# Task 5 修复轮 4 报告（R4 接替：经 escalation 取得 Task 0 执行授权——finding 根因修复落地，duplicate migration 故障在本分支消除）

- **执行者**：实现员-t51-任务5-接替者R4（resumed with findings，修复轮 4/5）
- **提交**：`e760c9255` `fix(migrations): dedupe mobile_device_app to 000118/000197 (wave-level prerequisite, unified with sibling branches, ruling via escalation)`（Task 0 重编本体）；ledger 与本报告另入册
- **状态**：DONE（唯一 finding 的根因修复已提交；全量回归 + 预存在失败归因在案，零伪造）

## 0. 本轮定性：不复读前任，走授权机制破循环

唯一 finding 第 4 次逐字重复（Task 0 未执行的跨任务预存在项）。前三轮「复跑取证 + 不代做」已被证明不收敛。本轮接替者按仓库先例（Task 3 F1 同款机制）**escalate 请裁决**，获得主控明确授权：**由本轮实现员直接执行 Task 0，且方案改写**——重编对象由计划原文的 `public_agent_marketplace` 改为 `mobile_device_app`（sqlite 000114→000118、versioned 000193→000197），与兄弟分支（t49/t53/t61/t62/t47）统一；理由：若按 t51 原文挪 marketplace 到 000118，将与兄弟分支已落地的 mobile_device_app→000118 在集成时主动制造新双占（主控全局占用表优先于计划原文）。ruling 行已逐字入 ledger（`plan-t51.md-ledger.md` Task 5 修复轮 4 节）。

## 1. 实现内容（提交 `e760c9255`，9 files, +17/−17）

1. **git mv 四个迁移文件**：sqlite `000114_mobile_device_app.{up,down}.sql → 000118_*`、versioned `000193_mobile_device_app.{up,down}.sql → 000197_*`（sqlite 两侧 similarity 100%，内容零改动）。
2. **versioned 两文件头注释同步**（与兄弟分支模板一致）：up 头「同 sqlite 000114 语义」→ 000118；down 头「与 sqlite 000114 down 对称」→ 000118、「回滚卡死在 000193」→ 000197。
3. **5 个 mobile_device 系测试文件引用同步**（7 处路径字符串 + 4 处注释提及）：handler 包 mobile_device_test、app repository 包 mobile_device_test / mobile_push_isolation_test / mobile_device_app_test、workbench 包 notification_app_policy_test。
4. **零触碰**：internal/database/migration.go（:33 门控常量 114 与 :123 探测串指向 marketplace 文件——marketplace 原号不动即零联动，规避计划原文的六处联动风险面）、marketplace 四文件（其 sqlite up 头「SQLite twin of versioned migration 000193.」仍正确，伴生 versioned 000193 未动）、本计划已落位的 000119/000198。
5. **与兄弟分支模板逐字节一致**：`git diff --cached 34849775b -- <9 路径>` → 空输出 exit 0（34849775b 为 plan-t62 分支同款 Task 0 代执行提交，stat 亦逐行一致：9 files, +17/−17）。

## 2. 测试命令与完整输出（全部本 ask 实跑）

### RED 基线（改动前，finding 复现）

```
$ go test ./internal/handler/ -run 'TestNotionPublish' -count=1
--- FAIL: TestNotionPublishEndToEndCreateApprovePublishReceipt (0.16s)
--- FAIL: TestNotionPublishEndToEndUpdateConflict (0.01s)
--- FAIL: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (0.01s)
	failed to open source, "file://…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
FAIL	github.com/Tencent/WeKnora/internal/handler	2.179s

$ go test ./internal/database/ -count=1   （节选，多处同因失败）
failed to create sqlite migrate instance: failed to open source, "file://migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
--- FAIL: TestExecutionTargetSQLiteFullMigrationDownUp / TestWorkbenchSQLiteDownRefusesPaseo 等
FAIL	github.com/Tencent/WeKnora/internal/database	3.687s
```

### GREEN（重编后）

```
$ go build ./…                                            → BUILD_EXIT=0（仅 macOS ld 既有噪音）
$ go test ./internal/database/ -count=1                   → ok 18.688s   ★Task 0 生效标志：FAIL→ok
$ go test ./internal/handler/ -run 'TestNotionPublish|TestAppPublicationsTableExists' -count=1 -v
--- PASS: TestAppPublicationsTableExistsAfterMigrations (1.55s)
--- PASS: TestNotionPublishEndToEndCreateApprovePublishReceipt (2.34s)   ★3 FAIL→PASS
--- PASS: TestNotionPublishEndToEndUpdateConflict (1.44s)                ★
--- PASS: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (2.16s)  ★
--- PASS: TestNotionPublishPlanGates / TestNotionPublishActionLookupIsTenantScoped
ok  github.com/Tencent/WeKnora/internal/handler	9.514s
   —— #48 三个 e2e 现在真实运行在生产全量 sqlite 迁移轨道上（Task 6 AC3 e2e 的前置解除）

$ go test ./internal/handler/ ./internal/router/ -count=1
ok  …/internal/handler	13.961s    （含 TestActionPlan×2 + TestNotionPublish 全家）
ok  …/internal/router	23.502s    （含 TestActionPlanRoutes×2）

$ go test ./internal/modules/appconnector/... -count=1    → 7 包全 ok
（appconnector 0.432s | connectorcontrol 2.205s | openconnector 0.427s | plan 1.634s
  | publish 1.725s | repository/appconnector 1.435s | service/appconnector 2.286s）
  —— 前置 Task 1-4 全部交付（TestPlanExecute*/TestPlanStatus*/TestPlanApprove*）零回归

$ go test ./internal/modules/workbench/service/workbench/ -count=1 -skip 'TestNotificationDeliveryRejectsResolvedInteractionAfterClaim'
ok  …/workbench	14.682s   （除 1 个预存在失败外全绿，见 §3）

$ go test ./internal/application/repository/ -run 'TestMobileDevice|TestMobilePush|TestValidateMobileAppID|TestBindIsolates|TestTokenExclusivity|TestNotificationIntentFanOut|TestClaimJoinsAppID|TestMobileDeviceAppDown' -count=1
ok  …/repository	2.229s    （被改 4 测试文件的全部测试）

$ go test ./internal/application/repository/ ./internal/modules/workbench/... -count=1 -failfast
ok  …/repository	440.667s  （该包全部测试通过）
workbench 首失败见 §3

$ go vet ./internal/handler/ ./internal/application/repository/ ./internal/modules/workbench/service/workbench/ ./internal/container/ ./internal/router/
VET_OK（exit 0）
```

## 3. 回归中发现的预存在失败（归因铁证：HEAD 基线复现，与本轮改动零因果）

- `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim`（workbench 包 notification_delivery_test.go:489，assert Not equal）：
  - 本轮单跑 `-count=3` 3/3 确定性失败；该文件对 migrations/与本轮 9 文件**零引用**（grep 实证）；
  - **HEAD 基线复现**：`git worktree add /tmp/t51-head-baseline 0a778861d`（本轮改动前的 HEAD）后同测试 `--- FAIL` → **预存在缺陷，非本轮因果**（临时 worktree 已清理）。
  - 需编排方/后续任务单独立案，不在本任务授权面（该文件非本改文件）。
- app repository 包首轮全包 FAIL(601.5s) 为 go test 默认 10 分钟超时假象（三包并发编译运行挤压）；`-failfast` 复跑 440.667s **全绿 ok**——非代码失败。

## 4. 自检发现

1. **部署运维交接注记（如实记录，决策层已定）**：存量 sqlite 库若停格在旧版 000114（=旧 mobile_device_app 已应用），重编后新 000118 会被当作未应用迁移再跑一次——up 文件为整表重建（CREATE 临时表→INSERT...SELECT→DROP→RENAME，数据保留）可安全重放；versioned(PostgreSQL) 轨道的 000197（ALTER ADD COLUMN）对停格在旧 000193 的存量库重放会因列已存在失败，需迁移运维按常规重编流程处理。该风险面为兄弟分支同款配方 + 主控裁决（「与其余分支统一」）的整体承接，测试/本地开发库不受影响（迁移历史随库文件走）。
2. **git add 插曲**：`.gitignore:96` 的 `migrations/` 规则对显式路径 `git add` 触发 ignored-advice（exit 1）；实际四个文件均为已跟踪文件且暂存区已含全部改动（先前的 `git add -A` 已入），`git diff --cached --stat` 核验（9 files, +17/−17）后直接提交，无内容缺失。
3. **Mimosa hook**：commit 时 `scanner_enobufs`（扫描器资源不足，完整审计未跑成），按兼容策略放行；本轮不宣称任何安全扫描结论；无新增 SQL 查询（纯文件重编+引用同步），无新增凭据。
4. **dig 运行时装配（待核实项 1）维持声明**：TestNotionPublish e2e 转绿不经过 dig 容器（e2e 手工装配 handler 链）；BuildContainer 完整 Invoke 需真实 config/DB/redis（internal/container/container.go:150，仅 cmd/server、cmd/desktop main 调用）。在案证据不变：build exit 0 + vet 五包干净（本轮扩至含 app repository/workbench）+ `*handler.AppActionPlanHandler` 全仓唯一 Provide 点 + dig v1.19.0 多输出为框架原生能力。运行时冒烟留待 Task 6 e2e 或部署冒烟。
5. **Task 6 e2e（待核实项 2）**：app_connector_action_plan_e2e_test.go 属 Task 6 授权面，本 ask 不含；**其全量迁移前置已由本轮解除**（TestNotionPublish e2e 于全量迁移轨道实跑 PASS 即证）。
6. **真实 Notion blocked-env（待核实项 3）**：维持计划总则声明（NOTION_TOKEN/NOTION_PARENT_PAGE_ID 本环境不存在），skip 不是 pass，零伪造。
7. **`go test ./internal/database/ -count=1`（待核实项 4）**：本轮已实跑 → **ok 18.688s**——此前各轮「Task 0 未发生无可验证对象」的声明就此闭环。

## 5. 给编排方的移交

1. **Task 0 已在本分支收口**（`e760c9255`，与兄弟分支统一配方）：迁移轨道可装载，Task 6 的 AC3 全量迁移 e2e 前置解除，可派发 Task 6。
2. workbench 预存在失败 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim`（§3，HEAD 基线复现）建议单独立案，勿再回流至 #51 任务面。
3. 本任务（Task 5）交付面（handler/路由/容器接线 + 4 测试）经四轮在案，本轮零改动零回归。
