# b2-ac-definition 证据文件（evidence）

> 节点：`b2-ac-definition`（Pass B 阶段 B2，25a Agent 定义/版本/人格/专家/子代理/收藏）。
> 计划：`docs/plans/passb/25a-agent-definition-version.md`（v3）。执行公约：`.superpowers/sdd/passb/conventions.md`。
> 本文件由该节点各 Task 持续追加；差分/等价章节按 conventions §6 要求随 T4 补全。

## 1. T1 基线锚定（2026-09-23）

### 1.1 基线 SHA

- 协调者派发任务起点 `BASE = 02e7be617c041bbf796b7f0acff0a4fbb6402a38`（节点分支 `codex/passb-b2-ac-definition` 当前 HEAD，实跑 `git rev-parse HEAD` 输出一致）。
- 计划 v3 撰写基线 `8c45a8815` = DAG `base_sha` 口径（passb-int `execution-dag.json` 节点 `b2-ac-definition.base_sha = 8c45a88153d0b20088252dbb29d2fb3815b253c2`，实查）。两者差异仅计划文档自身两次审校修订提交（`git diff --name-only 8c45a8815..02e7be617` 输出唯一文件 `docs/plans/passb/25a-agent-definition-version.md`），无代码差异；按计划 §6-T1「若协调者派发时另给基线以其为准」，本节点后续差集核对（T3 Step 5、§9-2）统一采用 `BASE=02e7be617`。
- 基线时工作树干净（`git status --porcelain` 零输出）。

### 1.2 调度前置复核（计划 §1.1/§1.2）

- 调度事实源 `.worktrees/passb-int/docs/plans/passb/execution-dag.json` 实查：`b0: done`、`ib1: done`、`b2-ac-definition: in_progress`——前置满足（DAG 状态回填归协调者，conventions §9；本 worktree 内 DAG 副本为分支点冻结快照，`ib1: in_progress` 系快照滞后，非阻塞）。
- 共享 immutable-version 契约冻结（framework:142 并行前提）：B0.4 产物，b0 已 done，满足。

### 1.3 基线门禁实跑记录（全部在 BASE=02e7be617 未改动工作树执行）

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | 0 | 仅 `cmd/desktop`、`cmd/server` 各一行 `ld: warning: ignoring duplicate libraries: '-lc++'`（既有现象，非错误） |
| `go test -count=1 ./internal/application/repository ./internal/application/service ./internal/handler ./internal/agentcatalog/...` | 0 | `ok ... internal/application/repository 489.119s`；`ok ... internal/application/service 344.838s`；`ok ... internal/handler 1.825s`；`? ... internal/agentcatalog [no test files]` |
| `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

计数基线零漂移：633 / 23+23 / 58（conventions §8 三方一致口径，本值与台账预期一致）。

### 1.4 随迁测试宿主侧预跑（旧实现基线，T4 等价比对「旧实现双跑」侧）

命令：`go test -count=1 -run 'TestGetShareByAgentIDAndSource|TestSubagent' ./internal/application/repository ./internal/handler -v`，退出码 0，两包 `ok`。用例清单（13 用例全 PASS）：

**`internal/application/repository`（7 用例）**：
| # | 用例 | 结果 |
|---|---|---|
| 1 | TestGetShareByAgentIDAndSourceForTenantDisambiguatesSource | PASS (0.01s) |
| 2 | TestSubagentUpsertIsIdempotentPerTenantSlugLocale | PASS (0.00s) |
| 3 | TestSubagentUpsertKeepsLocalesApart | PASS (0.00s) |
| 4 | TestSubagentListIsTenantScoped | PASS (0.00s) |
| 5 | TestSubagentDeleteIsSoftAndSlugWide | PASS (0.00s) |
| 6 | TestSubagentDeleteIsTenantScopedAndIdempotent | PASS (0.00s) |
| 7 | TestSubagentSoftDeleteAllowsSlugReuse | PASS (0.00s) |

**`internal/handler`（6 用例）**：
| # | 用例 | 结果 |
|---|---|---|
| 8 | TestSubagentCatalogList | PASS (0.00s) |
| 9 | TestSubagentCatalogDetail | PASS (0.00s) |
| 10 | TestSubagentAgentList | PASS (0.00s) |
| 11 | TestSubagentInstallHappyAndIdempotent | PASS (0.00s) |
| 12 | TestSubagentInstallRejections | PASS (0.00s) |
| 13 | TestSubagentRemoveIdempotent | PASS (0.00s) |

与计划 §5 口径核对：`tenant_subagent_test.go` 6 用例 ✓；`handler/subagent_test.go` 6 Test 函数（CatalogList/CatalogDetail/AgentList/InstallHappyAndIdempotent/InstallRejections/RemoveIdempotent）✓；`agent_share_source_test.go` 1 用例 ✓。留宿锚定件 `agent_version_test.go` 不在随迁集（计划 §4-②#4b、§5），不在此清单。

### 1.5 前置门复核（计划 §1.5 / §4-②#5）

- 命令：`test -f internal/commercial/repository/model_usage.go && echo EXPORT-LANDED || echo EXPORT-MISSING` → **`EXPORT-MISSING`**（退出码 0）。
- 旁证：`ls internal/execution/service` → `No such file or directory`。
- 结论：与计划 §1.5 预期一致（B1-CM/B1-EX 分支实施产出止于计划文档，无生产代码）。该门仅影响批次 2 推迟件 `repository/custom_agent.go`（§4-②#5），不阻塞批次 1；缺失时按 conventions §5 blocked 上报，不得自行实现导出。登记待 T5 Brief。

## 2. T4 门禁全套与等价证据（2026-09-24）

> 任务起点 BASE = `bd9e0f8e9`（= T3 提交点，`git rev-parse HEAD` 实证一致，工作树干净）。
> 本节全部命令与退出码均为 T4 会话在 worktree 实跑；节点级差集核对沿用 §1.1 确立的 `BASE=02e7be617` 口径。

### 2.1 DAG 四条 gates（计划 §6-T4 Step 1，逐条原文执行）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅 `cmd/desktop`、`cmd/server` 各一行 `ld: warning: ignoring duplicate libraries: '-lc++'`（§1.3 已记录的既有现象，非错误） |
| 2 | `go test -count=1 ./internal/agentcatalog/...` | 0 | `? ...internal/agentcatalog [no test files]`；`ok ...agentcatalog/handler 1.479s`；`ok ...agentcatalog/repository 1.408s`；`ok ...agentcatalog/service 1.404s` |
| 3 | `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` |
| 4 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

- 计数基线零漂移：`633 / 23+23 / 58` 与 T1 基线（§1.3）及台账一致（conventions §8 三方一致口径）。
- Gate 4 通过机制实证了计划 §2.3 的 shim 设计：7 个原路径 shim 保留使 legacy `os.Stat`（`tools/modulemove/verify.go:327-334`）逐条命中 → 16 manifests OK。

### 2.2 等价双跑（计划 §6-T4 Step 2 前半）

**旧实现侧（基线，已留证）**：

- T1 宿主侧预跑 13 用例（§1.4：repository 7 + handler 6，2026-09-23 在宿主包旧实现上实跑全 PASS）；
- T3 宿主侧特征化锚定 11 用例（T3 报告 §2.1：`go test -count=1 -run 'TestFavorite' ./internal/application/service ./internal/handler -v` → service 7 + handler 4 全 PASS，搬迁前旧实现）；
- T2 特征化补证（T2 报告 §0.2/§2.4：repo 侧 8 用例宿主侧先跑绿后随迁）。

**新实现侧（T4 实跑）**：

命令：`go test -count=1 -run 'TestGetShareByAgentIDAndSource|TestSubagent|TestFavorite' ./internal/agentcatalog/... -v`

退出码 0；`ok ...agentcatalog/handler 1.405s`、`ok ...agentcatalog/repository 0.618s`、`ok ...agentcatalog/service 1.316s`；28 用例全 PASS。

**逐用例比对表（旧侧 → 新侧）**：

| 组 | 用例（同名同果，全 PASS） | 旧侧出处 |
|---|---|---|
| 迁移·repository（7） | TestGetShareByAgentIDAndSourceForTenantDisambiguatesSource、TestSubagentUpsertIsIdempotentPerTenantSlugLocale、TestSubagentUpsertKeepsLocalesApart、TestSubagentListIsTenantScoped、TestSubagentDeleteIsSoftAndSlugWide、TestSubagentDeleteIsTenantScopedAndIdempotent、TestSubagentSoftDeleteAllowsSlugReuse | §1.4 #1–#7 |
| 迁移·handler（6） | TestSubagentCatalogList、TestSubagentCatalogDetail、TestSubagentAgentList、TestSubagentInstallHappyAndIdempotent、TestSubagentInstallRejections、TestSubagentRemoveIdempotent | §1.4 #8–#13 |
| 特征化·repository（4，T2） | TestFavoriteAddIsIdempotentAndReportsCreated、TestFavoriteListFiltersByResourceTypeAndTenant、TestFavoriteIsFavoriteReportsExistenceWithoutError、TestFavoriteRemoveIsScopedAndReportsDeleted | T2 报告 §2.4 |
| 特征化·service（7，T3） | TestFavoriteServiceListRejectsUnknownResourceType、TestFavoriteServiceListPassesThroughToRepository、TestFavoriteServiceAddValidatesTypeAndID、TestFavoriteServiceAddPassesThroughValidRequest、TestFavoriteServiceAddPropagatesRepositoryError、TestFavoriteServiceRemoveValidatesTypeAndID、TestFavoriteServiceRemovePassesThroughValidRequest | T3 报告 §2.1 |
| 特征化·handler（4，T3） | TestFavoriteHandlerRequiresUserAndWorkspaceContext、TestFavoriteHandlerListReturnsServiceFavorites、TestFavoriteHandlerAddMapsSentinelsToBadRequest、TestFavoriteHandlerRemoveMapsSentinelsToBadRequest | T3 报告 §2.1 |

**关键断言等价性依据**：3 个随迁测试文件经 `git diff -M --summary 02e7be617` 实证为 `rename (100%)`（subagent_test.go、agent_share_source_test.go、tenant_subagent_test.go）——断言体与宿主基线逐字节同一，双跑同名同果即行为等价；favorite 特征化用例按 conventions §1.4 宿主侧先跑绿（T2/T3 留证）再随迁，本次模块侧 28/28 PASS。

**结论**：批次 1 全部既有用例搬迁前后逐用例一致，等价双跑成立（13 迁移 + 15 特征化 = 28）。

### 2.3 高风险面推迟登记 presence 核对（计划 §6-T4 Step 2 后半、§5）

批次 1 无 framework:40 高风险面；节点高风险面全部位于批次 2 留宿文件。逐条 presence 实测（`grep` 实跑，行号为当前 HEAD 实测值）：

| 项 | 定义（实测） | 消费点（实测） | 结论 |
|---|---|---|---|
| `agentRequiresRerankModel` | `internal/application/service/agent_share.go:40` | `session_agent_qa.go:133`、`agent_run_graph.go:207`（另有同文件 ：165 内部消费） | 原样留宿 ✓ |
| `skillsForRun` | `internal/application/service/tenant_skill_effective.go:35` | （25b 面） | 原样留宿 ✓ |
| `resolveSharedAgentForRequest`/`filterKnowledgeByAgentScope`/`filterKnowledgeBasesForSharedAgent` | `internal/handler/shared_agent_access.go:18/:51/:64` | `knowledge.go:1607/:1667/:2185/:2217` + `knowledgebase.go:516/:533`，共 6 点 | 原样留宿 ✓ |
| `decodeExpertInstantiateBody`/`expertServiceError`/`maxExpertAgentNameLen` | `internal/handler/expert.go:178/:260/:23` | `skill_market.go:219/:228` + `tenant_expert_market.go:110/:186/:195/:202`，共 6 点 | 原样留宿 ✓ |

与计划 §4-②尾段清单逐条对照全部命中（含「共 6 点」口径）。差分义务随推迟件登记转移至 IB2/IB3 执行窗口，T5 Brief 显式登记（计划 §5）。

### 2.4 变更文件 vs owned_files 核对（计划 §6-T4 Step 3；conventions §1.2）

- 节点累计：`git diff --name-only 02e7be617 | sort` = **22 文件**（实测），构成：7 宿主 shim 重写（§4-③ 行 1–7）+ 7 模块生产新文件（§2.2 批次 1 #1–#7）+ 3 随迁测试（`rename (100%)`）+ 4 特征化新测试（§5 授权按需补）+ 1 evidence 文档（§2.3 节点产出文档清单）。**与授权清单差集为空**。
- 禁改面复核：`internal/router/**`、`internal/container/container.go`、`internal/bootstrap/**`、`internal/agentcatalog/module.go`、治理 YAML 四份、`docs/architecture/moves/*.yaml`、`tools/**`、`go.mod`/`go.sum` 均不在差集清单（零触碰）。
- 任务级：T4 自身改动仅本 evidence 文件（`git diff --name-only bd9e0f8e9` 在提交前仅此一项）。
- 注：T3 报告曾记节点累计 21 文件，实数为 22（其口径漏计 1 个纯 rename 项）；以本节实测 22 为准，构成核对结论不变。

### 2.5 未完全满足项（如实列出，禁止省略）

1. **宿主三包全量测试未在 T4 重跑**：T4 无代码改动，未重跑 `internal/application/{repository,service}`、`internal/handler` 全量；最近一次同 HEAD 代码态实跑为 T3（其报告 §2.2：service 100.288s ok、repository 218.697s ok、handler ok、router ok）。DAG gates 口径（agentcatalog 包）已在 §2.1 全绿。
2. **前置门 EXPORT-MISSING 仍未解除**（§1.5）：批次 2 推迟件 `repository/custom_agent.go` 的消费前提（`internal/commercial/repository/model_usage.go` 导出落地）在当前 HEAD 仍缺失——T5 Brief 登记义务，非批次 1 义务。
3. **swagger 产物源码-文档漂移窗口**（OCR R1 low·documentation）：`docs/docs.go:16472/:23635`、`docs/swagger.json:16465/:23628`、`docs/swagger.yaml:5016` 仍引用 `internal_handler.AddFavoriteRequest`（T4 grep 实证）；受零逻辑约束本轮未再生成 docs，登记 IB2 删除 shim 时统一刷新。
4. **favorite handler 兜底分支错误原文外泄**（OCR R1 low·security，随迁既有行为）：模块 `handler/user_resource_favorite.go` :84-87/:112/:140 兜底分支将底层 err.Error() 传入 NewInternalServerError；零逻辑约束本轮不改，登记 IB2 统一加固项（固定文案 + 原始 err 仅落日志）。
5. **高风险差分双跑义务未在本节点执行**：共享代理 KB 可见性过滤三函数与 `agentRequiresRerankModel` 差分随推迟件转移至 IB2/IB3 执行窗口（计划 §5 既有安排，presence 核对见 §2.3）。
