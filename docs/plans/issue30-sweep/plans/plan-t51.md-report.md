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
