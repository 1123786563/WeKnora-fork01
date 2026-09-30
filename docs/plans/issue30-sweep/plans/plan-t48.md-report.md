# plan-t48 收尾验证报告（第 10/10 任务：计划级最终验证与收尾）

- 日期：2026-09-25
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t48`
- 需求源：`docs/plans/issue30-sweep/plans/plan-t48.md`（T18：Notion 文档发布端到端闭环，Issue #48）

## 1. 任务定位与实现内容

本任务是 plan-t48 的第 10/10 个任务。前置 9 个任务（Task 0–9）已全部完成并提交，最后一个前置交付为 `TestNotionRealPublishLoop`（`internal/modules/appconnector/notion_publish_real_test.go:20`，NOTION_TOKEN/NOTION_PARENT_PAGE_ID 门控的真实 Provider 发布闭环证据：create→receipt→同页 version 实读更新→stale 版本零写拒绝）。

因此本任务不新增生产代码（TDD RED→GREEN 的逐任务证据由前置各任务的提交各自承载），本任务的实现内容为**计划级收尾验证**：

1. 按计划 Task 9 Step 3（`plan-t48.md:4938-4944`）运行计划级验证套件（`go build ./...` + 三个 `go test`）。
2. 按计划 Task 9 Step 2（`plan-t48.md:4931-4934`）实跑 `TestNotionRealPublishLoop` 确认无凭据 SKIP 且文案明示 blocked-env。
3. 按计划 Task 0 Step 4（`plan-t48.md:156-162`）实跑双轨迁移版本号唯一性 shell 断言。
4. 逐条列举计划全部验收测试的 verbose 通过清单（含 Task 8 E2E 与 Task 9 blocked-env SKIP）。
5. 写出本报告并提交。

### 计划交付物盘点（Task 0–9，均已在 HEAD 提交）

| 任务 | 交付 | 提交 |
|---|---|---|
| T1 | `internal/modules/appconnector/notion_update.go` Part A：`NotionUpdateSnapshot`/`ParseNotionUpdateSnapshot`/`IsNotionUpdateArgs`/`DetectNotionVersionConflict`/`NotionPageVersion`/`ParseNotionPageVersion`/`NotionPageReceipt`/`ParseNotionPageReceipt` | `1ade6d678` |
| T2 | `notion_update.go` Part B：`NotionUpdateAdapter`（版本预读→冲突零写拒绝→多步写 progress 落库→unknown→Query 远端对账） | `9e90d2ea7` |
| T0 | 迁移链修复：`agent_adoption_variants` 重编号 versioned 000191→000192 / sqlite 000112→000113 | `d34faec0b` |
| T3 | `repository/appconnector/publication.go`（`PublicationRow`/`PublicationStore`，planned→published/failed/unknown 状态机）+ 双轨迁移 `000193_app_publications`（versioned）/ `000114_app_publications`（sqlite） | `625fcf878` |
| T4 | `internal/modules/appconnector/publish/blocks.go`：Artifact 文本→Notion 段落块确定性投影 | `ae8550845` |
| T5 | `publish/dispatcher.go`：`NotionBridge`（ActionDispatcher+UnknownResolver+ReadPageVersion，create/update 按快照形状路由） | `1ca3e9e2e` |
| T6 | `publish/plan.go`：`NotionPublishService`（FormPlan 版本预读绑定/Execute/Reconcile 远端先行/回执 settle） | `4c8afe840` |
| T7 | `internal/handler/app_connector_notion_publish.go` + 路由注册 + 容器接线（`/apps/notion-publish/*`） | `617e9e1c9` |
| T8 | `internal/handler/app_connector_notion_publish_e2e_test.go`：生产迁移库 + 全真链路（仅 Notion 网点为明示契约双打） | `2cca0b3e2` |
| T9 | `internal/modules/appconnector/notion_publish_real_test.go`：真实 Provider 证据（blocked-env 门控） | `433a6eb62` |

## 2. 测试命令与完整输出（全部在本 worktree 根实跑）

### 2.1 Task 9 Step 2：真实 Provider 测试门控（带占位凭据）

命令：
```
NOTION_TOKEN=x NOTION_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestNotionRealPublishLoop -count=1 -v
```
输出：
```
=== RUN   TestNotionRealPublishLoop
    notion_publish_real_test.go:24: notion real credentials not configured (NOTION_TOKEN/NOTION_PARENT_PAGE_ID in artifacts/connector-real/notion.env); skip is not a pass — T18 real-provider evidence stays blocked-env
--- SKIP: TestNotionRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.383s
```

### 2.2 Task 9 Step 2：无凭据直跑（同样 SKIP）

命令：
```
go test ./internal/modules/appconnector/ -run TestNotionRealPublishLoop -count=1 -v
```
输出（tail）：
```
    notion_publish_real_test.go:24: notion real credentials not configured (...); skip is not a pass — T18 real-provider evidence stays blocked-env
--- SKIP: TestNotionRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.195s
```

**skip is not a pass**：真实 Notion API 验收在本环境 blocked-env（无 `artifacts/connector-real/notion.env` 凭据），如实标注，未放宽门控、未伪造通过。

### 2.3 计划级验证套件（Task 9 Step 3）

命令（逐条实跑）：
```
go build ./...
go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations|TestTaskCollaborationEndToEnd' -count=1
go test ./internal/modules/appconnector/... -count=1
go test ./internal/handler/ -run 'NotionPublish|AppPublications' -count=1
```
输出：

`go build ./...`：成功（仅链接器告警 `ld: warning: ignoring duplicate libraries: '-lc++'`，属本机工具链噪音，非编译错误；计划预期「无输出」指无编译错误/失败，构建退出码 0）。

```
ok  	github.com/Tencent/WeKnora/internal/application/repository	3.871s
```
```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.313s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	1.428s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.425s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.244s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.675s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.073s
```
```
ok  	github.com/Tencent/WeKnora/internal/handler	5.872s
```

### 2.4 Task 0 Step 4：双轨迁移版本号唯一性断言

命令：
```
test -z "$(ls migrations/sqlite | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && test -z "$(ls migrations/versioned | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && echo "migration versions unique on both tracks"
```
输出：
```
migration versions unique on both tracks
```

### 2.5 验收测试逐条清单（verbose，全 PASS）

命令：
```
go test ./internal/modules/appconnector/ ./internal/modules/appconnector/publish/ ./internal/modules/appconnector/repository/appconnector/ ./internal/handler/ -run 'NotionUpdate|NotionPublish|Publication|NotionBridge|NotionParagraph|NotionPage|IsNotionUpdateArgs|DetectNotionVersionConflict|ParseNotion' -count=1 -v
```
以及 `go test ./internal/modules/appconnector/publish/ -count=1 -v`。结果（46 项命名验收测试全 PASS）：

**Task 1（快照/冲突/回执解析，7）**：TestParseNotionUpdateSnapshotAcceptsExactFourFields、TestParseNotionUpdateSnapshotRejectsExtraField、TestParseNotionUpdateSnapshotRejectsMissingOrEmpty、TestIsNotionUpdateArgs、TestDetectNotionVersionConflict、TestParseNotionPageVersion、TestParseNotionPageReceipt

**Task 2（更新适配器，8）**：TestNotionUpdatePublishesTitleAndBlocksHappyPath、**TestNotionUpdateConflictZeroWrites（AC1：冲突零写，断言 PATCH/append 计数为 0）**、TestNotionUpdatePreReadFailureIsFailedNotUnknown、**TestNotionUpdateUnknownOnLostWriteReply（AC2：丢响应停车 unknown）**、**TestNotionUpdateQueryResolvesAfterDroppedAppend（AC2：Query 只读对账，写计数不增）**、TestNotionUpdateQueryStaysUnknownWhenBlocksMissing、TestNotionUpdateQueryVerifiesContainedRun、TestNotionUpdateResumesOnlyRemainingBlocks

**Task 3（回执 store + 迁移对齐，4+1）**：TestPublicationCreateFindRoundTrip、**TestPublicationSettleTransitions（终态不可倒改、幂等）**、TestPublicationSaveProgressIdempotent、TestPublicationLatestPublishedByDestination、TestAppPublicationsTableExistsAfterMigrations

**Task 4/5/6（publish 包，21）**：TestNotionParagraphBlocksSplitsParagraphs、TestNotionParagraphBlocksNormalizesLineEndingsAndWhitespace、TestNotionParagraphBlocksChunksLongParagraphs、TestNotionParagraphBlocksRejectsEmpty、TestNotionParagraphBlocksRejectsTooManyBlocks；TestBridgeDispatchCreateRoutesToCreateAdapter、TestBridgeDispatchUpdateConflictIsDefinitiveFailure、TestBridgeDispatchUnknownOutcomeParks、TestBridgeDispatchNonNotionConnectionFailsClosedPreSend、TestBridgeReadPageVersion；TestFormPlanCreateBindsBaselineVersion、TestFormPlanCreateParentOutOfScopeFailsClosed、TestFormPlanDestinationUnreadable、TestFormPlanUpdateRequiresPriorPublishedReceipt、**TestFormPlanUpdateBindsExpectedVersion（AC1：计划形成绑定外部版本）**、TestExecuteCreatePublishesAndSettlesReceipt、TestExecuteUpdateConflictSettlesFailedReceipt、**TestPublishReconcileResolvesUnknownWithoutRedispatch（AC2：对账不重派）**、TestFormPlanRejectsBadInputs、TestFormPlanRejectsUnsupportedArtifact、TestFormPlanRejectsNotReadyArtifact

**Task 7/8（谓词 + E2E，6）**：**TestNotionPublishPlanGates（越权 403/404）**、**TestNotionPublishActionLookupIsTenantScoped（跨租户 404）**；**TestNotionPublishEndToEndCreateApprovePublishReceipt（AC3：create→approve→publish→receipt 全链）**、**TestNotionPublishEndToEndUpdateConflict（AC1：E2E 409 冲突）**、**TestNotionPublishEndToEndUnknownReconcilesRemoteFirst（AC2：E2E unknown 远端先行对账）**

**Task 9（真实 Provider，1 SKIP）**：TestNotionRealPublishLoop —— blocked-env SKIP（见 2.1/2.2，非 pass）。

## 3. 验收标准覆盖结论

| AC | 原文 | 本地证据（本次实跑通过） | 真实证据 |
|---|---|---|---|
| 1 | 发布前读取外部当前版本并检测冲突 | TestNotionUpdateConflictZeroWrites（零写断言）、TestFormPlanUpdateBindsExpectedVersion、TestNotionPublishEndToEndUpdateConflict | TestNotionRealPublishLoop stale 拒绝段——**blocked-env，SKIP** |
| 2 | 超时和未知结果先核对远端，不盲重试 | TestNotionUpdateUnknownOnLostWriteReply、TestNotionUpdateQueryResolvesAfterDroppedAppend、TestBridgeDispatchUnknownOutcomeParks、TestPublishReconcileResolvesUnknownWithoutRedispatch、TestNotionPublishEndToEndUnknownReconcilesRemoteFirst | 同上——**blocked-env，SKIP** |
| 3 | 端到端行为通过最高稳定 Interface 验证；mock 不冒充真实集成证据 | Task 0 迁移链修复（2.3/2.4 证据）+ Task 8 E2E：生产 sqlite 迁移库 + 真实 ActionService/PublicationStore/NotionBridge/gin 处理器/既有审批端点，仅 Notion 网络端点为测试内明示契约双打（非真实验收） | TestNotionRealPublishLoop——**blocked-env，SKIP，如实在注** |

## 4. 提交

前置任务提交（Task 0–9，按时间序）：
`1ade6d678`、`9e90d2ea7`、`d34faec0b`、`625fcf878`、`ae8550845`、`1ca3e9e2e`、`4c8afe840`、`617e9e1c9`、`2cca0b3e2`、`433a6eb62`

本任务提交（即本报告文件所在提交，当前分支 HEAD）：
- docs(issue30-sweep): plan-t48 final closure verification report (10/10 tasks)

## 5. 自检发现

1. **Task 0 提交信息与计划措辞不同**：计划第 168 行拟提交信息为 `renumber agent_adoption_variants to 000192/000113 ...`，实际提交 `d34faec0b` 为 `fix(migrations): dedupe adoption migration version (000191->000192 / 000112->000113) broken by b3 merge`。语义等价（同组 git mv 重编号），变更内容已由 2.3/2.4 证据确认生效；如实记录差异，非本任务改动范围。
2. **真实 Provider 验收仍为 blocked-env**：`TestNotionRealPublishLoop` 在本环境 SKIP（两次实跑：带占位凭据与无凭据，SKIP 文案均明示 blocked-env）。有凭据环境运行 `NOTION_TOKEN=… NOTION_PARENT_PAGE_ID=… go test ./internal/modules/appconnector/ -run TestNotionRealPublishLoop -count=1 -v` 即自动执行；本报告不将其计为通过。
3. **迁移编号集成风险（计划已声明）**：`app_publications` 现占 versioned 000193 / sqlite 000114；同批 plan-t43/t60/t67 声明了同一组号。若兄弟计划先行合入，需按四方一致的顺延约定整体改号（DDL 与断言零变化）。当前 HEAD 下 2.4 断言通过，无冲突。
4. **本任务无新生产代码，TDD 不适用**：TDD RED→GREEN 证据由 Task 0–9 各自的先红后绿提交承载；本任务为只读验证 + 报告，未触碰任何授权文件之外的代码。
5. **工作区干净**：任务开始时 `git status --short` 为空（无他人未提交改动被卷入），本任务仅新增本报告文件并提交。
