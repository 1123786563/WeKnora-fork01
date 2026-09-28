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
ls internal/modules/knowledge/{ingest,retrieval,process,wiki}   # 四目录均存在 → P3-dirs: OK
grep -c "^func RecordKBActivity" internal/modules/knowledge/retrieval/app/kb_activity.go
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

go test -count=1 ./internal/modules/insights/...
# ?   	github.com/Tencent/WeKnora/internal/modules/insights	[no test files]
# ok  	github.com/Tencent/WeKnora/internal/modules/insights/metric	0.891s
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

## §2 analytics 差分（B3-IN.3 待填）

（B3-IN.3：P-6 宿主位基线 vs 模块位同用例实跑，逐用例等价比对结论；postgres blocked-env 移交登记。）

## §3 evaluation 特征化与 seam（B3-IN.2/B3-IN.4 待填）

（B3-IN.2：宿主位特征化 3 例首跑输出；B3-IN.4：模块位双跑输出、seam RED→GREEN 记录、capability 奇偶结论。）

## §4 治理行操作（B3-IN.3/B3-IN.4/B3-IN.5 待填）

（manifest/matrix 行级成对操作记录；alias 义务收口；exception-ledger 属主复核零命中实测。）

## §5 门禁记录（B3-IN.7 待填）

（节点四门禁 + 消费者面回归终局输出；`git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort` 与 owned_files 差集核对。）
