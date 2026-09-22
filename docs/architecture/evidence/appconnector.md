# Evidence — Task A1 appconnector（Pass A）

- 分支 / worktree：`bm-passa-a1` / `.worktrees/bm-passa-a1`
- base：`703884315`（= 集成线起点）
- move commit：`00aeceef6` — `refactor(appconnector): move packages to internal/modules/appconnector`
- repair commit：`16844f173` — `refactor(appconnector): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（dev compose，未动容器）；Go 1.26.3

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module appconnector
modulemove: OK (appconnector)
```

## 2. 测试基线（pre-move，旧路径）

| 命令 | 结果 |
|---|---|
| `go test ./internal/modules/appconnector/... -count=1`（manifest test_commands，逐字） | exit 0（仅骨架包，no test files） |
| `go test ./internal/appconnector/... ./internal/connectorcontrol/... ./internal/application/repository/appconnector/... ./internal/application/service/appconnector/... -count=1` | 5/5 包 ok，exit 0 |
| 直接消费方（handler、container、agent/tools、application/repository+service、cmd/connector-control，同 post 口径） | 29 包 ok（ok 29 / FAIL 0），exit 0 |

已知不稳定用例（F0 §2.4：agent/opencode 挂起、TestAgentRunDecisionConcurrentOnlyOneRevision）
不在本模块测试面内，两轮均未出现。

## 3. 搬迁后（post-move，新路径，repair commit 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module appconnector` | `modulemove: OK (appconnector)` |
| `go test ./internal/modules/appconnector/... -count=1`（manifest test_commands） | 5/5 包 ok（root 0.39s、connectorcontrol 3.50s、openconnector 2.21s、repository/appconnector 0.61s、service/appconnector 2.25s），FAIL 0 |
| 直接消费方 + 模块全量：`go test ./internal/handler/... ./internal/container/... ./internal/agent/tools/... ./internal/application/repository/... ./internal/application/service/... ./cmd/connector-control/... ./internal/modules/appconnector/... -count=1` | ok 29 / FAIL 0，exit 0 |
| `go build ./...` | exit 0（仅既有的 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet ./internal/modules/appconnector/... ./tools/...` | exit 0，无输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 total=633 | redis=23 lite=23 | hooks=58 | modules=16`，`OK (0 violations)` —— 与 F0 基线完全一致 |

## 4. Rename 证据

```
$ git diff --summary 703884315..00aeceef6 | grep -c '^ rename'   # move commit 单独
65
$ git diff --summary 703884315..00aeceef6 | grep -v '^ rename' | wc -l
0
$ git diff --summary -M100% 703884315..00aeceef6 | grep -c '^ rename'   # 全部 100% 相似度
65
```

- move commit `00aeceef6`：65 files changed, **0 insertions(+), 0 deletions(-)** —— 纯 rename。
- 合并 diff（703884315..16844f173）：65 renames + 5 个新建文件 = 恰好 5 个 alias.go
  （`create mode … alias.go` × 5，无其他 create/delete）。
- 内容纯度：repair commit `16844f173` 共 62 files，536+/120−；其中 5 个新建 alias.go 贡献
  +416（新建文件），57 个修改文件合计恰为 120+/120−，逐行审计全部落在两类：import 行
  （108+/108−，qualifier 保持不变，仅路径字符串）与
  搬迁测试文件内的相对迁移脚本路径深度修正（`../../../../migrations/` →
  `../../../../../migrations/` ×11 处、`../../migrations/` → `../../../../migrations/` ×1 处；
  见 §6 偏差说明）。**函数体零改动**（非测试 .go 文件仅 import 块变化）。
- gofmt：所有被触碰文件 gofmt-clean；`internal/handler/analytics_test.go` 等大量
  **未触碰**文件的 gofmt 噪音为仓库既有状态，未予处理。

## 5. 禁改共享文件未动证明

```
$ git diff --stat 703884315..16844f173 -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/
（空输出 —— 零差异）
$ git diff 703884315..16844f173 -- internal/container/container.go | wc -l
0
```

`internal/container/container.go:40,54` 仍指向旧路径并由别名包解析 —— 集成者操作见
`docs/architecture/integration/appconnector.md` §3。

## 6. 假设与偏差记录

1. **搬迁测试文件内相对路径修正（唯一的函数体内文本改动，_test.go only）**：
   8 个 `_test.go` 中 12 处 `os.ReadFile` 相对路径的 `../` 深度按新目录深度 +1 修正
   （repository/appconnector 与 service/appconnector：4→5 层；connectorcontrol：2→4 层；
   `worker_pg_test.go` 注释中的 docker 示例路径非运行时路径，未改）。不修则搬迁包测试
   无法通过 manifest test_commands；运行时行为（生产代码、迁移文件本身）零变化。
2. **别名面取全量导出符号**：manifest 未指定最小面，取"搬迁包全部导出符号"
   （go/types 枚举：141/19/42/112/52），保证禁改文件及任何集成期引用可解析；
   别名零逻辑，全部标注 `Deleted by Pass B task B-appconnector`。
3. **未使用 `go run ./tools/architectureguard` 之外的任何工具改动**；未 dispatch 子代理；
   未触碰其他 worker 领地（commercial/datasource/channels 路径零 diff）。
4. `cmd/connector-control` 归 F0 scope 之外，但为 manifest 声明的 importer，仅做 import 行修复。

## 7. Review 结果

**Approved**（final whole-branch review 回填；Review 范围：task-scoped gate review + batch barrier review）。
pass-a 全部 task 均 Approved、四道 barrier 均 Approved；唯一修复轮次为 A9 knowledge——retriever 路径对齐 fb71b3084，复 review ADDRESSED。
结论与证据台账见 `.superpowers/sdd/2026-09-21-backend-modularization-foundation-pass-a/progress.md` 与 `docs/architecture/evidence/batch-a*.md`。
