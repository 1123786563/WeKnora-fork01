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
