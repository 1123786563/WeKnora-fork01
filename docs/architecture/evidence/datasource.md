# datasource — Pass A 证据（Task A3）

分支 `bm-passa-a3`（worktree `.worktrees/bm-passa-a3`，基线 703884315）。
Manifest：`docs/architecture/moves/datasource.yaml`。行为零变更；路径/导入/组装之外无改动。

## 1. Gate

- 搬迁前：`go run ./tools/modulemove verify --module datasource` → `modulemove: OK (datasource)`（exit 0）
- 搬迁后：同命令再次运行 → `modulemove: OK (datasource)`（exit 0）

## 2. 基线测试（搬迁前，旧路径等价命令 `go test ./internal/datasource/... -count=1`）

```
ok  github.com/Tencent/WeKnora/internal/datasource                              5.717s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/confluence         2.283s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/dingtalk           2.395s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/feishu/core        4.781s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/feishu/drive       4.218s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/feishu/wiki       14.392s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/gitlab             2.341s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/ima                2.437s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/moauth             2.464s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/notion             1.524s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/rss                0.835s
ok  github.com/Tencent/WeKnora/internal/datasource/connector/yuque              8.659s
EXIT=0（12/12 包通过）
```

已知 flaky（F0 §2.4，归 agentruntime）本任务未出现：`internal/agent/opencode` 超时、
`TestAgentRunDecisionConcurrentOnlyOneRevision` 并发偶败。

## 3. 搬迁后测试（manifest 命令 + 直接消费方）

- `go test ./internal/modules/datasource/... -count=1`（manifest test_commands）：
  12/12 包 `ok`（datasource 6.262s；confluence 3.807s；dingtalk 3.857s；feishu/core 6.231s；
  feishu/drive 5.705s；feishu/wiki 15.602s；gitlab 3.578s；ima 3.540s；moauth 3.377s；
  notion 4.075s；rss 0.409s；yuque 8.359s），EXIT=0。
- 直接消费方 datasource 测试（`-run` 精确名单）：
  - `go test ./internal/application/service/ -run '^Test(...109 个 datasource 相关测试...)$' -count=1`
    → `ok github.com/Tencent/WeKnora/internal/application/service 1.180s`，EXIT=0
    （覆盖 datasource_service/stream/purge/reindex/cancel/credential_refresh/delete_sqlite/
    sync_cancel/knowledgebase_delete_datasource 全部测试函数）
  - `go test ./internal/handler/ -run '^Test(...20 个 datasource handler 测试...)$' -count=1`
    → `ok github.com/Tencent/WeKnora/internal/handler 0.552s`，EXIT=0
    （datasource_test.go / datasource_credentials_test.go / datasource_documents_count_test.go /
    datasource_reindex_test.go 全部测试函数）

## 4. 构建 / 静态检查

- `go build ./...` → exit 0（仅 cmd/server、cmd/desktop 的 ld "duplicate libraries" 链接警告，与本次改动无关）
- `go vet ./internal/datasource/... ./internal/modules/datasource/... ./internal/application/service/... ./internal/handler/` → exit 0
- `go run ./tools/architectureguard` → `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`，`OK (0 violations)`
- gofmt：所有触碰过的文件 `gofmt -l` 为空（gofmt 实际增量仅为 sed 替换导致的 import 块排序；
  repo 中既有未触碰文件的 gofmt 漂移未处理，见 concerns）

## 5. 纯改名证据（spec M2）

- MOVE 提交（6ad77f0cd）：`git diff --summary <base>..6ad77f0cd` → **97 条 rename，0 条非 rename**；
  `git diff --shortstat -M` → `97 files changed, 0 insertions(+), 0 deletions(-)`；
  以 `-M100%` 复核仍是 97 rename —— 相似度 100%，无任何函数体/内容变化。
- 95 个 .go 文件 + 2 个文档随树搬移；`internal/datasource/README.md` 因与模块骨架
  `internal/modules/datasource/README.md` 同名冲突，落位为 `README.pkg.md`（内容字节不变，
  仅目标文件名不同，-M100% 下仍为 rename）。
- COMPILE-REPAIR 提交（c12c4a139）：77 files changed, 169 insertions(+), 72 deletions(-)
  = 65 个文件 import 行修复（72 行成对换）+ 12 个新别名文件（约 97 行新增，全部无逻辑转发）。

## 6. 禁改文件未触碰证明

`git diff --name-only 703884315..HEAD -- internal/router/router.go internal/router/task.go
internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/`
→ **空输出**（整个分支两笔提交均未触碰）。

横向遗留文件的编辑仅为 import 行（`git diff -U0` 过滤 import 行后为空）：
`internal/application/service/datasource_service.go`、`internal/application/service/knowledgebase.go`、
`internal/handler/datasource.go` 及 10 个 `internal/application/service/*_test.go`。

## 7. 别名面（alias surface）

12 个旧路径各留一个无逻辑别名包（文件头注明 `Pass B task B-datasource` 删除）：

| 旧路径（package） | 转发面 |
|---|---|
| `internal/datasource`（datasource） | `type ConnectorRegistry = …; type Scheduler = …; var NewConnectorRegistry = …; var NewScheduler = …` |
| `…/connector/confluence`（confluence） | `var NewConnector` |
| `…/connector/dingtalk`（dingtalk） | `var NewConnector` |
| `…/connector/feishu/core`（core） | `var RegionFeishu / RegionLark / RegionFeishuDrive / RegionLarkDrive`（原为 struct 型 var，不能 const 化，var 转发） |
| `…/connector/feishu/drive`（drive） | `var NewDriveConnector` |
| `…/connector/feishu/wiki`（wiki） | `var NewConnector` |
| `…/connector/gitlab`（gitlab） | `var NewConnector` |
| `…/connector/ima`（ima） | `var NewConnector` |
| `…/connector/moauth`（moauth） | 最小包文件（无任何 importer 引用，故无转发面） |
| `…/connector/notion`（notion） | `var NewConnector` |
| `…/connector/rss`（rss） | `var NewConnector` |
| `…/connector/yuque`（yuque） | `var NewConnector` |

转发面 = 禁改文件 `internal/container/container.go` 的真实引用集合（grep 全仓复核，
router 与 go.mod/migrations 无 datasource 包引用）。切换与删除指引见
`docs/architecture/integration/datasource.md` §3。

## 8. 提交

| 提交 | 主题 |
|---|---|
| 6ad77f0cd | `refactor(datasource): move packages to internal/modules/datasource`（纯改名，97 文件 0±0） |
| c12c4a139 | `refactor(datasource): repair imports and add pass-a aliases`（import 修复 + 别名包，禁改文件未触碰） |
| （本文档 + integration brief） | `docs(datasource): add pass-a integration brief and evidence` |

## 9. Review 结果

（占位）待独立 review 后回填结论与 findings。
