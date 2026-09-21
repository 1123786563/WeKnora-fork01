# Evidence — Task A10 conversation（Pass A）

- 分支 / worktree：`bm-passa-a10` / `.worktrees/bm-passa-a10`
- base：`918f90000`（= 集成线 IA2 之后的 "docs: correct batch-a2 evidence figures from ia2 review"）
- move commit：`f8d557c16` — `refactor(conversation): move packages to internal/modules/conversation`
- repair commit：`a4a087780` — `refactor(conversation): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未动容器）；未启动/停止任何容器

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module conversation
modulemove: OK (conversation)
$ go run ./tools/architectureguard      # 基线
architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16
architectureguard: OK (0 violations)
$ go build ./...                        # exit 0（仅既有 cmd/desktop、cmd/server ld duplicate-library warning）
$ go test ./internal/modules/conversation/... -count=1   # manifest test_commands 逐字
ok  github.com/Tencent/WeKnora/internal/modules/conversation  [no test files]
```

## 2. 测试基线（pre-move，旧路径未动时）

| 命令 | 结果 |
|---|---|
| `go test ./internal/application/service/... ./internal/handler/session/... -count=1 -timeout=20m`（session/chat/feedback/query-history 基线；feedback、query-history、session 服务测试在横向 host 包内） | exit 0，`ok application/service 84.648s`（含 feedback/query-history/session）、`ok application/service/chat_pipeline 0.834s`、`ok handler/session 17.768s`，FAIL 0 |

已知不稳定用例（F0 §2.5：internal/agent/opencode 挂起、
TestAgentRunDecisionConcurrentOnlyOneRevision、payment TestProvidersFromEnvRejectsPartialAlipay）
不在本模块测试面内，本轮未出现、未回归。

## 3. 搬迁后（post-move，repair commit a4a087780 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module conversation` | `modulemove: OK (conversation)` |
| `go run ./tools/modulemove verify --all` | `modulemove: OK (16 manifests verified)` |
| `go test ./internal/modules/conversation/... -count=1`（manifest test_commands） | `ok internal/modules/conversation/chat_pipeline 1.799s`，FAIL 0 |
| 直接消费方 + 流式/路由 characterisation：`go test ./internal/modules/conversation/... ./internal/application/service/... ./internal/handler/... ./internal/router/... -count=1 -timeout=25m`（manifest 未列 characterisation 命令，按规程跑等价存在项：handler/session 流式与会话路由 + 搬迁后 chat_pipeline 包测试均已含） | ok 11 / FAIL 0，exit 0 —— chat_pipeline 1.799s、application/service 215.656s、service/file 5.700s、service/memory 6.116s、service/metric 2.252s、service/retriever 2.182s、service/workbench 32.547s、handler 3.407s、handler/dto 2.622s、handler/session 39.391s、router 5.467s |
| `go build ./...` | exit 0（仅既有的 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet ./internal/modules/conversation/... ./internal/application/service/chat_pipeline/... ./internal/handler/... ./internal/container/...` | exit 0，无输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`（计数器与 F0 基线一致）+ **6 条新 forbidden-import 诊断**（见 §6；除诊断行外 guard exit 1，无其他类别发现） |

## 4. Rename 证据

```
$ git diff --summary f8d557c16~1..f8d557c16 | grep -c '^ rename'
47
$ git diff --summary f8d557c16~1..f8d557c16 | grep -vc '^ rename'
0
$ git diff f8d557c16~1..f8d557c16 --shortstat
 47 files changed, 0 insertions(+), 0 deletions(-)
```

- move commit `f8d557c16`：47 files changed，**0 insertions(+), 0 deletions(-)**，
  47/47 `rename ... (100%)` 相似度（`git diff --summary` 全 R，无 A/M/D 条目）。
- **零函数体改动**：纯 rename + 0 插入删除 ⇒ 无任何内容修补；
  本次**不需要** test 相对路径深度修补（moved `_test.go` 共 9 个，无一处引用旧 import 路径）。
- repair commit `a4a087780`：5 files changed，28 insertions / 4 deletions =
  4 个非禁改 importer 各 1 行 import 路径替换（gofmt 只重排了被替换 import 行所在的
  import 块顺序，函数体零改动）+ 新增别名文件 `alias.go`（24 行，见 §5）。

## 5. Import 修复与别名面

非禁改 importer（owned_files.importers 重 grep 全集=5 文件；其中 4 个非禁改，全部翻转）：

| 文件 | 引用符号 | 修复 |
|---|---|---|
| `internal/application/service/extract.go:13` | `chatpipeline.NewExtractor` | import 翻转 |
| `internal/application/service/session.go:14` | `*chatpipeline.EventManager` | import 翻转 |
| `internal/application/service/session_knowledge_qa.go:10` | 进度 helper ×10 | import 翻转 |
| `internal/handler/initialization.go:18` | `chatpipeline.NewExtractor` | import 翻转 |
| `internal/container/container.go:52`（禁改） | `NewEventManager` + `NewPlugin*` ×17（:644-661） | **未触碰**；由别名桥接 |

别名包 `internal/application/service/chat_pipeline/alias.go`：零逻辑 var 转发 18 符号
（= container.go 实际引用面，刻意收窄、非全量导出面），文件头
`Deleted by Pass B task B-conversation`。可变 var 翻转先例（A6 LocalImageResolver）
**不适用**：container.go 不赋值任何 chatpipeline 导出 var（已 grep 验证）；
`registerChatLocalImageResolver`（container.go:1123）赋值的是 airesource
`models/chat.LocalImageResolver`（A6 领地）。

禁改文件 diff 核验：router.go / task.go / sync_task.go / go.mod / go.sum / migrations/
**零 diff**；`internal/container/container.go` **零 diff**（本任务无禁改文件触碰）。

## 6. 偏差与 IA3 移交项

1. **guard 6 条新 forbidden-import（按规程只记录，未修、未加例外、未改工具）**：
   搬迁把 chat_pipeline 变为模块内部路径后，conversation→airesource 跨模块 import 被
   guard 规则命中。6 处 import 行在旧路径时代逐字相同（common.go:13、
   extract_entity.go:14、data_analysis.go:13、query_understand.go:11、rerank.go:11、
   references.go:8），属 pre-existing 耦合被搬迁暴露；且走的是 batch-a2 冻结面
   文档化入口路径（`frozen-entrypoints-batch-a2.md` §airesource：models/chat、
   models/rerank）。张力（冻结面文档 vs guard 模块根门面规则）归 IA3 裁定，
   处置建议见 `docs/architecture/integration/conversation.md` §8。
2. **gofmt 触碰面披露**：4 个翻转文件的 import 块因路径长度变化被 gofmt 重排
   （仅 import 行顺序，无其他改动）；`gofmt -l` 另发现 2 个**预先存在**的未格式化文件
   （`internal/application/service/query_history_policy_test.go`、
   `internal/application/service/workbench/overview.go`）——非本任务触碰、未修
   （workbench 属他模块领地），记录备查。
3. **无行为变更声明依据**：move commit 0 插入删除；repair commit 仅 import 行 + 别名；
   session/message SSE 流式、share token、审计隐私策略、CSV 导出语义均未触碰
   （相关测试 handler/session 39.4s、application/service 215.7s 全绿）。

## 7. 产出物

- Integration Brief（routes 7 入口、workers 2、hooks 3、配置键、禁改文件 file:line
  清单与 IA3 切换步骤、guard 发现）：`docs/architecture/integration/conversation.md`
- Pass B briefs：`docs/architecture/passb/conversation-session.md`、
  `docs/architecture/passb/conversation-queryhistory.md`
- 本证据文件：`docs/architecture/evidence/conversation.md`
