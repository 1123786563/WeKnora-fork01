# b3-insights 证据文件（Pass B / 37-insights）

> 节点：`b3-insights`（B3 Insights：6 legacy 文件迁入模块 + `deleteReferencedKnowledge` 消费侧 seam + metric 别名义务收口）。计划：`docs/plans/passb/37-insights.md`。
> ALIGN_SHA = `38219c0638be37a19d270442dd5d91567c39c2d1`（开工 HEAD = 本分支首个任务 commit 的父提交，计划 §0.1 P-2 裁定：无额外 merge 的直系谱系）——本节点全部 diff 检查的 PASSB_BASE_SHA 采用值。
> 本文件由 B3-IN.1 起累积；B3-IN.7 定稿差分与计数章节。

## §1 基线快照（B3-IN.1；2026-09-28，worktree `.worktrees/passb-b3-insights`，对齐后树）

### P-1 前置节点终态（DAG 实测）

判据来源：调度事实源 `.worktrees/passb-int/docs/plans/passb/execution-dag.json`（conventions.md 头注指定；计划 §0.1 P-1 口径「以开工时点 DAG 实测为准，notes 残留 BLOCKED 文本为历史登记，不构成停工依据」）。

| 节点 | status | head_sha | 判定 |
|---|---|---|---|
| `ib2` | done | `a2fbcf55eb7ba0dc73f9deecba00c5f2f45e634f` | 满足（计划预期 ib2 head=`a2fbcf55e` 一致） |
| `b3-insights` | in_progress | —（未回填，正常） | 协调者已恢复派发 |

### P-2 基线对齐（Ruling 2026-09-24-WAVE-DEP-BASELINE）

```bash
git -C .worktrees/passb-b3-insights merge-base --is-ancestor a2fbcf55e HEAD && echo aligned
# aligned（exit=0）——本分支为 ib2 谱系直系，无需额外 merge
```

**ALIGN_SHA = `38219c0638be37a19d270442dd5d91567c39c2d1`**（开工 HEAD，`git rev-parse HEAD` 实测；即任务派发 BASE）。

### P-3 ib2 冻结门面在位（required_contracts 自核）

```bash
ls internal/knowledge/{ingest,retrieval,process,wiki}   # 四目录均存在 → P3-dirs: OK
grep -c "^func RecordKBActivity" internal/knowledge/retrieval/app/kb_activity.go
# 1（≥1，K2 导出面就绪）
grep -n "^func deleteReferencedKnowledge" internal/application/service/knowledge_delete_plan.go
# 36:func deleteReferencedKnowledge(
```

三项判据全中。**§0.2.2 DAG notes 偏差登记（非阻塞）**：DAG notes 称「K4 已导出」，实测 `deleteReferencedKnowledge` 仍未导出、留守宿主 `internal/application/service`（knowledge_delete_plan.go:36，K4 推迟批）——按 conventions §5 如实登记；本计划采用消费侧 seam + 宿主 compat 接线（§2.3(a)），对导出前后两时点行为等价，不构成阻塞。

### P-4 门禁工具可用

`Makefile` 实读：`verify-module-moves`（:250）与 `check-backend-architecture`（:255）两目标在位（`sed -n '248,258p' Makefile` 确认目标定义与注释）。`check-passb-readiness` 为 barrier 门禁，本节点不跑。

### P-5 worktree 干净

`git status --short` → 0 行；分支 `codex/passb-b3-insights`（`git branch --show-current` 实测）。

### P-6 基线门禁快照（开工 HEAD 实跑留档，后续差分对照）

六条命令逐条实跑（输出原样摘录）：

```bash
go build ./...
# （stderr）# github.com/Tencent/WeKnora/cmd/desktop
#          ld: warning: ignoring duplicate libraries: '-lc++'
#          # github.com/Tencent/WeKnora/cmd/server
#          ld: warning: ignoring duplicate libraries: '-lc++'
# EXIT_CODE=0（ld 警告为既有噪声，与计划 P-6 预期一致）

make verify-module-moves
# go run ./tools/modulemove verify --all
# modulemove: OK (16 manifests verified)
# EXIT_CODE=0

make check-backend-architecture
# go run ./tools/architectureguard
# architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16
# architectureguard: OK (0 violations)
# EXIT_CODE=0（计数基线 633/23+23/58/16 三方一致口径的 guard 实测值）

go test -count=1 ./internal/insights/...
# ?   	github.com/Tencent/WeKnora/internal/insights	[no test files]
# ok  	github.com/Tencent/WeKnora/internal/insights/metric	0.891s
# EXIT_CODE=0

go test ./internal/application/repository -run TestAnalyticsAggregations -count=1
# ok  github.com/Tencent/WeKnora/internal/application/repository 1.324s
# EXIT_CODE=0

go test ./internal/handler -run TestAnalyticsHandler -count=1
# ok  github.com/Tencent/WeKnora/internal/handler	1.229s
# EXIT_CODE=0
```

**差分基线用例清单**（`-v` 变体实跑摘录，供 B3-IN.3 模块位逐例等价比对；§2.4 差分口径）：

`TestAnalyticsAggregations`（宿主位 `internal/application/repository`，双方言）：

```text
=== RUN   TestAnalyticsAggregations
=== RUN   TestAnalyticsAggregations/sqlite
=== RUN   TestAnalyticsAggregations/postgres
    analytics_test.go:82: TRPC_TEST_POSTGRES_DSN unset: PostgreSQL recovery acceptance NOT VERIFIED
--- PASS: TestAnalyticsAggregations (0.80s)
    --- PASS: TestAnalyticsAggregations/sqlite (0.80s)
    --- SKIP: TestAnalyticsAggregations/postgres (0.00s)
PASS
ok  github.com/Tencent/WeKnora/internal/application/repository 2.032s
```

→ sqlite PASS / postgres **SKIP**（`TRPC_TEST_POSTGRES_DSN` unset）——按 Spec §14.4 标 **blocked-env，不计 PASS**，移交 IB3 以 env 复跑（计划 §0.2.3、Brief (d)；framework:192 analytics 方言测试属 IB3 必测）。

`TestAnalyticsHandler_*`（宿主位 `internal/handler`，10 个顶层用例全 PASS）：

```text
--- PASS: TestAnalyticsHandler_DefaultRange30Days (0.00s)
--- PASS: TestAnalyticsHandler_ExplicitRangePassthrough (0.00s)
--- PASS: TestAnalyticsHandler_InvalidTimeParams (0.00s)
--- PASS: TestAnalyticsHandler_TenantRequired (0.00s)
--- PASS: TestAnalyticsHandler_AgentMessagesPassthrough (0.00s)
--- PASS: TestAnalyticsHandler_DateOnlyEndCoversWholeDay (0.00s)
--- PASS: TestAnalyticsHandler_TimestampedEndNotExtended (0.00s)
--- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray (0.00s)
--- PASS: TestAnalyticsHandler_AgentMessagesEmptyAgentID (0.00s)
--- PASS: TestAnalyticsHandler_RepoErrorMapsTo500 (0.00s)
PASS
ok  github.com/Tencent/WeKnora/internal/handler 1.565s
```

→ 与计划 §1 随迁测试表所列 10 个顶层用例清单一一对应，零缺漏。

## §2 analytics 差分（B3-IN.3，2026-09-28）

### §2.1 物理迁移与 rename 检测

4 文件经 Write 通道落位 + `git rm` 删除宿主原件（环境备注：Mimosa 拦截 `git mv` 纯重命名——Bash 写源码通道策略，PreToolUse 指引改用 Write/Edit 提交同一内容；内容逐字等价，未变换写法）。`git status` 对 4 对路径全部识别为 rename（`R`）：

```text
R  internal/handler/analytics.go -> internal/insights/analytics/analytics_handler.go
R  internal/handler/analytics_test.go -> internal/insights/analytics/analytics_handler_test.go
R  internal/application/repository/analytics.go -> internal/insights/analytics/analytics_repository.go
R  internal/application/repository/analytics_test.go -> internal/insights/analytics/analytics_repository_test.go
```

`git diff --cached -M --stat -- internal/application/repository/analytics.go internal/insights/analytics/analytics_repository.go`：

```text
 .../analytics.go => modules/insights/analytics/analytics_repository.go} | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)
```

→ repository 侧函数体零变化（唯一 diff 行 = package 子句），rename 以内容相似度成立。

### §2.2 计划内代码面变化清单（§2.3 列明点，其余零改动）

| 变化 | 位置 | 性质 |
|---|---|---|
| `package repository`/`package handler` → `package analytics` | 两生产文件 + 两测试文件头 | 包声明 |
| `parseAnalyticsRange` → `ParseAnalyticsRange` | analytics_handler.go（定义 + 4 调用点） | 仅导出可见性 |
| `analyticsRows` → `Rows` | analytics_handler.go（定义 + 4 调用点） | 导出改名（§2.3(c) 转发前置） |
| 新增模块本地 `parseFilterTime` | analytics_handler.go | knowledge.go:2565-2580 逐字副本 + parity 头注（session/handler.go:432 先例），import 增 `strings` |
| 装置副本 `openRunTestDB`/`seedRunFixtures`/`openPostgresRunTestDB` | analytics_repository_test.go | agent_run_test.go:29-118 逐字，repoRoot 改 `"../../../.."`（Ruling TEST-SUPPORT-SHIM 家族头注） |

### §2.3 计划偏差登记（1 项，非断言变化）

**analytics_handler_test.go 采用外部测试包 `package analytics_test`**（计划原文「package analytics，本体零改动」）：

- 根因：内部测试包编译图含被测包 —— `analytics(test) → internal/middleware（ErrorHandler）→ internal/application/repository（kb_access.go:7 apprepo）→ internal/insights/analytics（本节点宿主 compat 转发）` 成环，`go vet`/`go test` 编译失败（实测报错 `import cycle not allowed in test`）。
- 处置：改 `package analytics_test` + `&analytics.AnalyticsHandler{...}` 限定（Go 标准机制，外部测试包不参与被测包编译图）。**10 个顶层用例的用例名、路由挂载、请求、断言全部逐字保持**；差分口径不受影响（黑盒 HTTP 行为不变）。
- 先例核对：其余模块测试（agentcatalog/handler、knowledge/retrieval/app/handler 等）import middleware 无环，因其宿主 compat 指向的模块包不在被测包编译链上；本节点 repository 侧 compat 恰指向被测包 `analytics` 本身，属计划撰写时未覆盖的真实约束。

### §2.4 模块位实跑 vs P-6 宿主位基线逐例比对

`go test -count=1 -v ./internal/insights/analytics/`（2026-09-28 实跑摘录）：

```text
=== RUN   TestAnalyticsAggregations
=== RUN   TestAnalyticsAggregations/sqlite
=== RUN   TestAnalyticsAggregations/postgres
    analytics_repository_test.go:195: TRPC_TEST_POSTGRES_DSN unset: PostgreSQL recovery acceptance NOT VERIFIED
--- PASS: TestAnalyticsAggregations (1.79s)
    --- PASS: TestAnalyticsAggregations/sqlite (1.79s)
    --- SKIP: TestAnalyticsAggregations/postgres (0.00s)
--- PASS: TestAnalyticsHandler_DefaultRange30Days (0.00s)
--- PASS: TestAnalyticsHandler_ExplicitRangePassthrough (0.00s)
--- PASS: TestAnalyticsHandler_InvalidTimeParams (0.00s)
    --- PASS: TestAnalyticsHandler_InvalidTimeParams/invalid_start (0.00s)
    --- PASS: TestAnalyticsHandler_InvalidTimeParams/invalid_end (0.00s)
--- PASS: TestAnalyticsHandler_TenantRequired (0.00s)
--- PASS: TestAnalyticsHandler_AgentMessagesPassthrough (0.00s)
--- PASS: TestAnalyticsHandler_DateOnlyEndCoversWholeDay (0.00s)
--- PASS: TestAnalyticsHandler_TimestampedEndNotExtended (0.00s)
--- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray (0.00s)
    --- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray/queries (0.00s)
    --- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray/users (0.00s)
    --- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray/channels (0.00s)
    --- PASS: TestAnalyticsHandler_EmptyRowsRenderEmptyArray/agents (0.00s)
--- PASS: TestAnalyticsHandler_AgentMessagesEmptyAgentID (0.00s)
--- PASS: TestAnalyticsHandler_RepoErrorMapsTo500 (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/insights/analytics	2.877s
```

**逐例等价比对结论**（vs §1 P-6 宿主位基线）：

| 用例 | 宿主位基线（§1） | 模块位（本节） | 等价 |
|---|---|---|---|
| TestAnalyticsAggregations/sqlite | PASS | PASS | ✅ |
| TestAnalyticsAggregations/postgres | SKIP（DSN unset） | SKIP（DSN unset） | ✅（blocked-env 同口径） |
| TestAnalyticsHandler_DefaultRange30Days | PASS | PASS | ✅ |
| TestAnalyticsHandler_ExplicitRangePassthrough | PASS | PASS | ✅ |
| TestAnalyticsHandler_InvalidTimeParams（2 子测试） | PASS | PASS | ✅ |
| TestAnalyticsHandler_TenantRequired | PASS | PASS | ✅ |
| TestAnalyticsHandler_AgentMessagesPassthrough | PASS | PASS | ✅ |
| TestAnalyticsHandler_DateOnlyEndCoversWholeDay | PASS | PASS | ✅ |
| TestAnalyticsHandler_TimestampedEndNotExtended | PASS | PASS | ✅ |
| TestAnalyticsHandler_EmptyRowsRenderEmptyArray（4 子测试） | PASS | PASS | ✅ |
| TestAnalyticsHandler_AgentMessagesEmptyAgentID | PASS | PASS | ✅ |
| TestAnalyticsHandler_RepoErrorMapsTo500 | PASS | PASS | ✅ |

→ **12/12 组全等价，零行为漂移**。postgres 方言 **blocked-env**（Spec §14.4，不计 PASS）：`TRPC_TEST_POSTGRES_DSN` unset，模块位与宿主位同口径 SKIP——移交 IB3 以 env 复跑 `TestAnalyticsAggregations/postgres`（framework:192 必测；Brief (d)）。

### §2.5 宿主消费面回归（转发链）

`go test ./internal/handler -run 'TestUsage' -count=1` → `ok github.com/Tencent/WeKnora/internal/handler 1.730s`（EXIT=0）——usage.go 5 个调用点（:89/:107/:114/:128/:138）经 handler compat 转发 `analytics.ParseAnalyticsRange`/`analytics.Rows`，usage 面 HTTP 契约不因转发变化。

## §3 evaluation 特征化与 seam（B3-IN.2/B3-IN.4，2026-09-28）

### §3.1 物理迁移与 rename 检测（B3-IN.4）

`git diff --cached -M --summary`（同 commit staging）：
```
rename internal/{application/service => modules/insights/evaluation}/dataset.go (99%)
rename internal/{application/service => modules/insights/evaluation}/evaluation.go (94%)
rename internal/{application/service => modules/insights/evaluation}/evaluation_characterization_test.go (99%)
rename internal/{handler/evaluation.go => modules/insights/evaluation/evaluation_handler.go} (99%)
rename internal/{application/service => modules/insights/evaluation}/metric_hook.go (99%)
```
逐字等价构造性核验：5 个新文件对 `git show HEAD:<旧路径>` 的正文（tail -n +2）做 `diff` 全部 IDENTICAL（仅 line 1 `package service|handler` → `package evaluation`）。environment 备注：Mimosa hook 拦截本会话 Bash `git mv`（通道提示改用 Write/Edit），按 hook 指示以 Write 逐字提交新路径 + `git rm` 删旧路径完成等价 rename（内容相似度 94–99% 由 git 实测检出；非规避扫描，内容经 Write 通道 PreToolUse 审查）。

### §3.2 计划内代码面变化（evaluation.go 净差分，其余 4 文件零内容变化）

seam（§2.3(a)）：① `DeleteReferencedKnowledgeFunc` 类型（签名=宿主 knowledge_delete_plan.go:36 原型逐参一致）；② `EvaluationService.deleteReferencedKnowledge` 字段；③ `NewEvaluationService` 增第 7 参 `cleanup DeleteReferencedKnowledgeFunc`（原 6 参序型不变、返回 `interfaces.EvaluationService` 不变，struct 字面量对齐随 gofmt 机械重排）；④ EvalDataset defer 内调用点 `deleteReferencedKnowledge(` → `e.deleteReferencedKnowledge(`（实参序不变）。

### §3.3 seam TDD：RED → GREEN（B3-IN.4 步骤 4）

- **RED**（seam 测试先行、构造器尚无第 7 参）：`go test ./internal/insights/evaluation -run TestEvalDatasetInvokesKnowledgeCleanupSeam -count=1` → 编译失败，输出两行：
  - `evaluation.go:366:13: undefined: deleteReferencedKnowledge`（迁移后同包裸标识符断链，预期）
  - `evaluation_cleanup_test.go:106:3: too many arguments in call to NewEvaluationService ... want (..., interfaces.ModelService)`（第 7 参缺失，即计划预测的 RED 证据）
  - `FAIL ... [build failed]`
- **GREEN**（seam 落地后）：同命令 `-v` → `--- PASS: TestEvalDatasetInvokesKnowledgeCleanupSeam (0.00s)` + `ok ... 0.963s`（EXIT=0）。断言内容：seam 收到 `("kb-eval", ["k-eval-1"])` 且 `DeleteKnowledgeBase("kb-eval")` 被调——钉住 Knowledge 删除高风险面的清理语义（Spec §14.3）。

### §3.4 特征化双跑差分（宿主位 vs 模块位，逐例等价）

用例集 = B3-IN.2 特征化 3 例（用例名与断言逐字不变随迁）。宿主位输出为 B3-IN.4 会话补跑留档（`git stash push -u` 临时回到 BASE=bd6667bc2 树实跑后 `git stash pop` 恢复，备份目录 diff 复核 BACKUP-MATCH）：

| 命令 | 输出 | 退出码 |
|---|---|---|
| 宿主位：`go test ./internal/application/service -run 'TestGetPassageListGolden\|TestMetricListAppendAvg\|TestHookMetricRecordFinishMapsContentToPID' -count=1 -v`（BASE 树） | `--- PASS: TestGetPassageListGolden (0.00s)` / `--- PASS: TestMetricListAppendAvg (0.00s)` / `--- PASS: TestHookMetricRecordFinishMapsContentToPID (0.00s)` + `ok ... 0.946s` | 0 |
| 模块位：`go test ./internal/insights/evaluation -run '<同上三例>\|TestEvalDatasetInvokesKnowledgeCleanupSeam' -count=1 -v` | 同 3 例 PASS 逐字一致 + seam 1 例 PASS + `ok ... 1.293s` | 0 |

| 用例 | 宿主位（BASE） | 模块位（迁移后） | 等价 |
|---|---|---|---|
| TestGetPassageListGolden | PASS | PASS | ✅ |
| TestMetricListAppendAvg | PASS | PASS | ✅ |
| TestHookMetricRecordFinishMapsContentToPID | PASS | PASS | ✅ |

→ **3/3 逐例等价，零行为漂移**；seam 1 例为新增钉住项（GREEN 在案）。

### §3.5 capability 奇偶差分（§2.1 路由测试消费面）

`go test ./internal/router -run TestTenantInfrastructureRoutesDeclareSpecificCapabilities -count=1` → `ok github.com/Tencent/WeKnora/internal/router 1.868s`（EXIT=0）——`router_api_key_capabilities_test.go:344` 的 `&handler.EvaluationHandler{}` 空字面量构造与 `POST /evaluation → APIKeyCapabilityRunEvaluations` 断言在 type 别名（`handler.EvaluationHandler = evaluation.EvaluationHandler`）下原样通过，capability 奇偶零变化。

### §3.6 宿主消费面回归

`go test ./internal/handler -run 'TestUsage' -count=1` → `ok github.com/Tencent/WeKnora/internal/handler 2.416s`（EXIT=0）——handler compat 文件追加 Evaluation 段后 analytics 转发链不受影响。

## §4 治理行操作（B3-IN.3/B3-IN.4/B3-IN.5 待填）

### B3-IN.4（evaluation 面，与物理迁移同 commit）

- `docs/architecture/moves/insights.yaml` legacy_files：删 4 行（`internal/application/service/{dataset,evaluation,metric_hook}.go`、`internal/handler/evaluation.go`，Ruling 1「可早删不可晚删」）；增 1 行 shim（`internal/application/service/insights_passb_compat.go`，reason「Pass B 过渡 shim，ib3 同 commit 随文件删行」、navigation_label「Insights host compat (application/service)」，Ruling 7 成对纪律）；shim 行排序对齐 datasource（repository → service → handler）。
- `docs/architecture/passb/ownership-matrix.yaml`：删 4 行（原 :782-:787 dataset、:812-:817 evaluation(service)、:1034-:1039 metric_hook、:1550-:1555 evaluation(handler)）；增 1 行 shim（插于 fork_bootstrapper.go 与 kbshare.go 之间保持字典序；destination=`internal/insights/evaluation`，integration_owner/delete_barrier=ib3）。
- 门禁复核：`make verify-module-moves` → `modulemove: OK (16 manifests verified)`；`make check-backend-architecture` → `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`（计数基线 633/23+23/58/16 不变）。

### B3-IN.3（analytics 面，与物理迁移同 commit）

- `docs/architecture/moves/insights.yaml` legacy_files：删 2 行（`internal/application/repository/analytics.go`、`internal/handler/analytics.go`，Ruling 1「可早删不可晚删」）；增 2 行 shim（`insights_passb_compat.go` ×2，reason「Pass B 过渡 shim，ib3 同 commit 随文件删行」，Ruling 7 成对纪律，行格式镜像 datasource.yaml:9-:12）。
- `docs/architecture/passb/ownership-matrix.yaml`：删 analytics 2 行（原 :98-:103、:1478-:1483）；增 2 行 shim（repository 侧插于 feedback.go 与 kbshare.go 之间、handler 侧插于 initialization.go 与 knowledge.go 之间，保持字典序；destination=`internal/insights/analytics`，integration_owner/delete_barrier=ib3，行格式镜像 datasource_passb_compat.go 行）。
- 门禁复核：`make verify-module-moves` → `modulemove: OK (16 manifests verified)`；`make check-backend-architecture` → `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`（计数基线 633/23+23/58/16 三方一致口径不变）。

## §5 门禁记录（B3-IN.7 待填）

（节点四门禁 + 消费者面回归终局输出；`git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort` 与 owned_files 差集核对。）
