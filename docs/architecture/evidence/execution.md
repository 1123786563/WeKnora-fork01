# Evidence — execution 模块搬迁（Pass A batch A2, Task A5）

Worktree：`.worktrees/bm-passa-a5`（分支 `bm-passa-a5`，基线 `03032f890` = batch A1 集成线）。
注意：worktree 由编排方以 bm-passa-main 为 cwd 创建，实际磁盘路径为
`.worktrees/bm-passa-main/.worktrees/bm-passa-a5`；分支名与任务书一致。

## 1. Commits

| # | SHA | 主题 | 内容 |
|---|---|---|---|
| 0 | `03032f890` | refactor: integrate pass-a batch a1 | 基线（不属本任务） |
| 1 | `6ad22241d` | refactor(execution): move packages to internal/modules/execution | 纯 rename：142 文件 `git mv`，0 insertions / 0 deletions |
| 2 | `44c29f19a` | refactor(execution): repair imports and add pass-a aliases | 114 个 importer 文件 import 路径修复 + 3 个旧路径 alias.go + testdata 归位 + 触碰文件 gofmt |

## 2. Pre-move gate

- `go run ./tools/modulemove verify --module execution` → `modulemove: OK (execution)`
- manifest `test_commands` 的搬迁前等价命令（旧路径）：

```
go test ./internal/execution/... ./internal/sandbox/... ./internal/browserskill/... -count=1
ok  github.com/Tencent/WeKnora/internal/execution     1.487s
ok  github.com/Tencent/WeKnora/internal/sandbox       5.957s
ok  github.com/Tencent/WeKnora/internal/browserskill  0.437s
```

已知 flaky（基线 §2.5 登记在案，agentruntime 归属）本模块未触发。

## 3. Rename 证据（纯 move commit）

```
git diff --summary 03032f890..6ad22241d   →  142 rename（100%），无其余类型
  internal/browserskill → internal/modules/execution/browserskill   18 文件
  internal/execution   → internal/modules/execution                17 文件
  internal/sandbox     → internal/modules/execution/sandbox       107 文件
  其中 _test.go 72、非测试 .go 69、testdata/start-command.json 1
```

repair commit 内对 fixture 的二次 `git mv`：
`rename internal/modules/execution/{ => testdata}/start-command.json (100%)`
——修复 move commit 中 fixture 落在包根而非 `testdata/` 的失误，保持测试
`os.ReadFile("testdata/start-command.json")` 相对深度不变（无内容编辑）。

## 4. 内容改动披露（repair commit）

`git diff HEAD -w` 证明：忽略空白后**全部**改动行均为 import 路径行（116 处），零函数体改动：

- 114 个 importer 文件：`internal/{execution,sandbox,browserskill}` → `internal/modules/execution{/sandbox,/browserskill}`；
- 新增 3 个旧路径 alias 包（零逻辑，type alias + var 转发，头注 `Deleted by Pass B task B-execution`）；
- gofmt 触碰文件（9 个）产生的**纯空白**重排（结构体字段注释对齐），`git diff -w` 下消失；
  仓库既有未格式化文件（如 `internal/agent/opencode/executor_test.go`）未被触碰。

无任何 API/SQL/状态机/权限/错误/重试/schema/业务规则改动。

## 5. Post-move 验证

| 检查 | 命令 | 结果 |
|---|---|---|
| manifest test_commands | `go test ./internal/modules/execution/... -count=1` | ok execution 2.535s / browserskill 1.999s / sandbox 5.810s |
| 直接消费方 | `go test ./internal/agent/skills/... ./internal/agent/tools/... ./internal/container/... ./internal/router/... -count=1` | 全 ok |
| 直接消费方 | `go test ./internal/handler/... -count=1` | ok handler 1.925s / dto / session 13.259s |
| 直接消费方 | `go test ./internal/application/repository/... ./internal/application/service/workbench/... -count=1` | 全 ok |
| 直接消费方 | `go test ./internal/application/service -count=1` | ok 236.499s |
| 构建 | `go build ./...` | 通过（仅既有 `ld: warning: ignoring duplicate libraries: '-lc++'`） |
| 静态检查 | `go vet`（modules/execution、旧路径别名、agent/skills、agent/tools、container、router、handler{,/session}、application/repository、application/service） | 0 问题 |
| 守卫 | `go run ./tools/architectureguard` | `OK (0 violations)`；`literal=564 apiKeyRoute=69 handle=0 total=633`（与基线 633 一致）、`redis=23 lite=23`、`hooks=58`、`modules=16` |

## 6. Review

- [ ] 待 Review（Reviewer 填写）：确认 move commit 纯 rename、alias 零逻辑、无行为变更。

## 7. 遗留与 IA2 关注点

1. `internal/modules/execution/sandbox/url_guard.go` 导入 `internal/ipclass`（policy 模块，
   A8 搬迁至 `internal/modules/policy/ipclass`）——A8 落地后需修复该一行 import，详见
   integration brief §7。
2. 旧路径别名覆盖面刻意收窄为禁改文件引用面（5 符号）；如需全量导出符号别名，
   按 integration brief §2 的说明补齐。
3. Sibling 领地未被触碰：未改 `internal/modules/{airesource,agentcatalog,policy,system}`、
   `internal/handler`/`internal/application` 下非 import 行、`internal/router/router.go`、
   `task.go`、`sync_task.go`、`internal/container/container.go`、`go.mod`、`go.sum`、`migrations/`。
