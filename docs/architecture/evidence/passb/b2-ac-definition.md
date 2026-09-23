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
| `go test -count=1 ./internal/application/repository ./internal/application/service ./internal/handler ./internal/modules/agentcatalog/...` | 0 | `ok ... internal/application/repository 489.119s`；`ok ... internal/application/service 344.838s`；`ok ... internal/handler 1.825s`；`? ... internal/modules/agentcatalog [no test files]` |
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

- 命令：`test -f internal/modules/commercial/repository/model_usage.go && echo EXPORT-LANDED || echo EXPORT-MISSING` → **`EXPORT-MISSING`**（退出码 0）。
- 旁证：`ls internal/modules/execution/service` → `No such file or directory`。
- 结论：与计划 §1.5 预期一致（B1-CM/B1-EX 分支实施产出止于计划文档，无生产代码）。该门仅影响批次 2 推迟件 `repository/custom_agent.go`（§4-②#5），不阻塞批次 1；缺失时按 conventions §5 blocked 上报，不得自行实现导出。登记待 T5 Brief。
