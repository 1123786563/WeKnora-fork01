# Evidence — Task A8 policy（Pass A）

- 分支 / worktree：`bm-passa-a8` / `.worktrees/bm-passa-main/.worktrees/bm-passa-a8`
- base：`03032f890`（= 集成线 "refactor: integrate pass-a batch a1"；system 文档 commit
  `35846029c` 在 move 之前，与本模块无交集）
- move commit：`5fef452ae` — `refactor(policy): move packages to internal/modules/policy`
- repair commit：`556fc173d` — `refactor(policy): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未动容器）；Go 1.26.3

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module policy
modulemove: OK (policy)
$ go run ./tools/modulemove verify --module system
modulemove: OK (system)
$ go run ./tools/architectureguard      # 基线
literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16
OK (0 violations)
$ go build ./...                        # exit 0（仅既有 cmd/server、cmd/desktop ld warning）
```

## 2. 测试基线（pre-move，旧路径未动时）

| 命令 | 结果 |
|---|---|
| `go test ./internal/modules/policy/... -count=1`（manifest test_commands，逐字） | exit 0（骨架包 no test files） |
| `go test ./internal/modules/system/... -count=1`（同上，system 侧） | exit 0（no test files） |

说明：pre-move 基线按任务规程以 manifest test_commands 为准；五包旧路径侧未单独跑
pre-move 测试，以 move commit 25/25 纯 rename（见 §4）+ post-move 全绿（§3）作为等价证明。

## 3. 搬迁后（post-move，repair commit 556fc173d 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module policy` | `modulemove: OK (policy)` |
| `go test ./internal/modules/policy/... -count=1`（manifest test_commands） | 5/5 包 ok（access 2.754s、embedpolicy 0.356s、ipclass 0.357s、ratelimit 0.366s、storageallowlist 0.373s），FAIL 0 |
| 直接消费方：`go test ./internal/application/service/... ./internal/handler/... ./internal/middleware/... ./internal/sandbox/... ./internal/types/... ./internal/utils/... ./internal/container/... ./internal/modules/channels/im/... -count=1` | ok 25 / FAIL 0，exit 0 |
| `go test ./internal/router/... -count=1`（files.go、routes_agent.go 两个直接导入方所在包） | exit 0，FAIL 0 |
| `go build ./...` | exit 0（仅既有的 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet ./internal/modules/policy/... ./internal/application/service/... ./internal/handler/... ./internal/middleware/... ./internal/sandbox/... ./internal/types/... ./internal/utils/... ./internal/router/... ./internal/container/... ./internal/modules/channels/...` | exit 0，无输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`（与 F0 基线一致）+ **2 条新 forbidden-import 诊断**（见 §7） |

已知不稳定用例（F0 §2.5：agent/opencode 挂起、TestAgentRunDecisionConcurrentOnlyOneRevision、
commercial payment TestProvidersFromEnvRejectsPartialAlipay）不在本模块测试面内，本轮未出现。

## 4. Rename 证据

```
$ git show --stat 5fef452ae | tail -1
 25 files changed, 0 insertions(+), 0 deletions(-)
$ git diff --summary -M100% 5fef452ae~1..5fef452ae | grep -c "100%"
25
$ git diff --summary 5fef452ae~1..5fef452ae | grep -c '^ rename'
25
```

- move commit `5fef452ae`：25 files changed，**0 insertions(+), 0 deletions(-)**，25/25
  rename 相似度 100% —— 纯 rename，函数体零改动。
- repair commit `556fc173d`：61 files，182+/59−。其中 5 个新建 alias.go 贡献 +123
  （零逻辑转发，73 个导出符号：access 46、embedpolicy 4、ipclass 15、ratelimit 2、
  storageallowlist 6）；56 个修改文件合计恰为 59+/59−，`git diff -U0` 逐行审计：
  118 条变更行**全部**是 `"github.com/Tencent/WeKnora/internal/..."` import 路径行，
  非导入行变更 = 0（qualifier 全部保持不变；56 文件 = 53×1 行 + 3×2 行
  [knowledgebase.go、knowledgebase_pr3_test.go：access+storageallowlist；embed_auth.go：
  embedpolicy+ratelimit]）。**函数体零改动，零测试文件相对路径修正需求**
  （五包测试不引用 migrations 相对路径）。
- gofmt：全部被触碰文件 + 5 个 alias.go `gofmt -l` 零输出；横向包内未触碰文件的
  gofmt 噪音（analytics_test.go 等）为仓库既有状态，未予处理。

## 5. 禁改共享文件未动证明

```
$ git diff --stat 03032f890..HEAD -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/ \
    internal/config/
（空输出 —— 零差异）
```

`internal/container/container.go:107` 仍指向旧路径并由别名包解析 —— 集成者操作见
`docs/architecture/integration/policy.md` §3。`internal/middleware/`（embed_auth.go、
kb_access.go、rbac.go）与 `internal/router/{files.go,routes_agent.go}` 为 manifest 声明的
**可改 importer**，仅 import 行修复。

## 6. 假设与偏差记录

1. **manifest importers 路径已过时（batch A1 集成所致）**：manifest 把 `internal/im`、
   `internal/im/yunzhijia` 列为 ratelimit/ipclass 的 importer；batch A1 把 channels 搬到
   `internal/modules/channels/` 后，同两文件现为 `internal/modules/channels/im/service.go`
   与 `internal/modules/channels/im/yunzhijia/url.go`。A8 按同一 importer 语义仅做 import
   行修复（未动 channels 任何逻辑）；由此产生的 2 条 guard 诊断见 §7。
2. **别名面取全量导出符号**（73 个）：manifest 未指定最小面，取"搬迁包全部导出符号"
   （与 A1 同口径），保证禁改文件及集成期引用可解析；别名零逻辑，全部标注
   `Deleted by Pass B task B-policy`。当前仅 storageallowlist 别名有 importer
   （container.go:107），其余 4 个为 manifest ruling 2 的 1:1 义务产物。
3. **access 目标路径**：manifest `to: internal/modules/policy/access`（叶子名规则，非
   `policy/application/access`）；move 与 import 修复均按 manifest 执行。
4. 未使用 guard 例外、未修改 architectureguard/modulemove 工具；未 dispatch 子代理；
   未触碰 execution/airesource/agentcatalog worker 领地（其模块路径零 diff）。

## 7. Guard 新增发现（交 IA2，未处置）

```
architectureguard: forbidden-import: internal/modules/channels/im/service.go 导入了模块 policy 的内部包 "github.com/Tencent/WeKnora/internal/modules/policy/ratelimit"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/channels/im/yunzhijia/url.go 导入了模块 policy 的内部包 "github.com/Tencent/WeKnora/internal/modules/policy/ipclass"（跨模块只能经模块根公共门面）
```

先于模块化改造的既有耦合（channels→ratelimit/ipclass），被"platform 路径→policy 模块
内部路径"的搬迁暴露；路由/worker/hook 计数与基线完全一致，其余检查 0 违规。
按任务边界未加例外、未改工具；处置建议见 `integration/policy.md` §8。

## 8. Review 结果

（占位 —— 等待独立 review；review 意见回填此处。）
