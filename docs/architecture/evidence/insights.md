# Evidence — Task A14 insights（Pass A）

- 分支 / worktree：`bm-passa-a14` / `.worktrees/bm-passa-a14`
- base：`8fbc030a8`（= 集成线 "refactor: integrate pass-a core modules"，IA3 放行点）
- move commit：`6f19c34d8` — `refactor(insights): move packages to internal/modules/insights`
- repair commit：`b476463d6` — `refactor(insights): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未启停容器）；未派 subagent

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module insights
modulemove: OK (insights)
$ go build ./...                        # exit 0（仅既有 cmd/server、cmd/desktop ld warning）
```

## 2. 测试基线（pre-move，旧路径未动时，base=8fbc030a8）

| 命令 | 结果 |
|---|---|
| `go test ./internal/application/service/metric/... -count=1`（manifest test_commands 的搬迁前等价路径） | ok 0.805s |
| `go test ./internal/handler -run TestAnalyticsHandler -count=1 -v`（analytics handler legacy 文件，11 例） | PASS 11/11，ok 1.066s |
| `go test ./internal/application/repository -run TestAnalyticsAggregations -count=1 -v` | PASS（sqlite 0.55s；`/postgres` 子测试 SKIP：`TRPC_TEST_POSTGRES_DSN` 未设——本机凭据不可得，与 F0 基线同环境），ok 1.509s |
| `go test ./internal/application/service -count=1`（metric 唯一 importer 所在包全量） | ok 101.661s |

已知不稳定用例（F0 §2.5：agent/opencode 挂起、TestAgentRunDecisionConcurrentOnlyOneRevision、
payment TestProvidersFromEnvRejectsPartialAlipay）不在本模块测试面内，本轮未出现。

## 3. 搬迁后（post-move，repair commit b476463d6 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module insights` / `--all` | OK / OK（16 manifests） |
| `go test ./internal/modules/insights/... -count=1`（manifest test_commands 逐字） | insights 骨架 no test files + metric ok 1.349s，FAIL 0 |
| `go test ./internal/application/service -count=1`（直接消费方全量） | ok 137.753s |
| `go test ./internal/handler -run TestAnalyticsHandler -count=1` | ok 0.934s（11/11 PASS） |
| `go test ./internal/application/repository -run TestAnalyticsAggregations -count=1` | ok 1.117s（sqlite PASS；`/postgres` SKIP，与 pre-move 同Skip，环境对等无回归） |
| `go test ./internal/application/service/metric/... -count=1`（旧路径别名包，no test files 确认零残留） | no test files，exit 0 |
| `go build ./...` | exit 0（仅既有 ld duplicate-library warning，与基线一致） |
| `go vet ./internal/modules/insights/... ./internal/application/service/` | exit 0，无输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`（与 F0 基线一致），**OK (0 violations)**，落盘 `/tmp/a14-guard.log` |

pre vs post 结论：测试集合相同、全部绿、零新失败；guard 计数器 633/23+23/58/16 与
F0 基线逐项一致。

## 4. Rename 证据

move commit `6f19c34d8`：13 files changed, **0 insertions(+), 0 deletions(-)**；
`git diff --summary 8fbc030a8..6f19c34d8` 13/13 条 `rename ... (100%)`
（bleu/common/map(+test)/mrr(+test)/ndcg/precision(+test)/recall(+test)/rouge/rouge_score，
`internal/{application/service => modules/insights}/metric/`）。
无测试相对路径修复（包内测试自含，post-move 原样通过）。

repair commit `b476463d6`：2 files changed, +9/−1 ——
`internal/application/service/metric_hook.go`（唯一 importer，1 行 import 翻转 +
gofmt 对该 import 块的同块重排，函数体零改动）+ 新增
`internal/application/service/metric/alias.go`（8 行空壳别名，moauth 先例：
零转发面，头部标注 `Deleted by Pass B task B-insights`）。

## 5. Forbidden-untouched 证明

```
$ git diff --name-only 8fbc030a8..HEAD -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/
（空）
```

兄弟领地核查：`git diff --name-only 8fbc030a8..HEAD | grep -E 'workbench|craft'` → 空
（A12/A13 领地零触碰）。batch-a2/a3 冻结面：move 树零跨模块 import，零新增消费。

## 6. Guard findings for IA4

**0 violations，0 条新发现** —— 无预存耦合被本搬迁暴露，无需例外登记。
（对照：A10 曾移交 6 条、A11 曾移交 49 条；insights 搬迁面纯净。）

## 7. 假设与偏差记录

1. **Scope=manifest**：move 仅 `internal/application/service/metric` 1 包；
   analytics/evaluation/dataset/metric_hook 的 handler/service/repository 混合文件按
   manifest legacy_files 留横向包（6 文件，索引见 `internal/modules/insights/legacy/README.md`
   与 `docs/architecture/passb/insights.md`）。
2. **别名面=空（moauth 先例）**：唯一 importer `metric_hook.go` 属非禁改 importer
   （owned_files.importers），直接翻转 import；翻转后旧路径零消费者，禁改文件从未引用
   metric 包，故别名包为零转发面最小包文件（A3 datasource/moauth 同例，review 已认可
   该形态）；manifest alias_obligations 1:1 义务仍按 ruling 2 留置实体。
3. **基线口径**：manifest test_commands 首条为后搬迁路径；pre-move 基线按任务规程跑其
   搬迁前等价路径（metric 旧路径）+ analytics（handler/repository）+ evaluation 消费方
   （service 全量）+ export 语义未被本模块拥有（query_history_export 归 conversation，
   manifest 可查），未纳入本模块测试面。
4. **postgres 子测试 SKIP 对等**：`TestAnalyticsAggregations/postgres` 依赖
   `TRPC_TEST_POSTGRES_DSN`，本机 5432 凭据不可得（探针记录于任务过程），pre/post 同
   SKIP，sqlite 方言断言全绿——环境门控，非回归。
