# Evidence — agentruntime 模块搬迁（Pass A batch A3, Task A11，续作完成）

Worktree：`.worktrees/bm-passa-a11`（分支 `bm-passa-a11`，基线 `918f90000`）。
**注意：本任务为续作（resumed attempt）**。前任实施者在 move commit 之后、repair 阶段
中断；本 evidence 记录继承状态、续作核验与补全。

## 1. Commits

| # | SHA | 主题 | 内容 |
|---|---|---|---|
| 0 | `918f90000` | docs: correct batch-a2 evidence figures from ia2 review | 基线（不属本任务） |
| 1 | `004248f84` | refactor(agentruntime): move packages to internal/modules/agentruntime | 纯 rename：371 文件 `git mv`，0 insertions / 0 deletions（继承自前任，已核验，未 amend） |
| 2 | （本续作） | refactor(agentruntime): repair imports and add pass-a aliases | 全部非禁改 importer import 修复 + 4 个旧路径 alias.go + 测试相对路径深度修复 + gofmt |
| 3 | （本续作） | docs(agentruntime): add integration brief, pass-b briefs and evidence | integration/agentruntime.md、passb/×4、evidence/agentruntime.md |

## 2. 继承状态核验（resumed attempt）

- move commit `004248f84`：`git diff --summary 918f90000..004248f84` → 371/371 rename
  （全部 100%），`--shortstat` 0 insertions / 0 deletions，与 manifest `move_packages`
  20 个包逐一对应（含 `application/service/memory`→`…/agentruntime/memory`、
  `modelcontext`→`…/agentruntime/modelcontext`）。
- 前任 repair 草稿（未提交）：import 修复覆盖全部非禁改 importer（re-grep 全仓旧路径
  仅剩 `internal/container/container.go` 4 行）+ 4 个 alias.go（approval、experts、
  subagents、memory）+ 5 个文件的测试相对路径深度修复 + gofmt 对齐。续作逐项核验：
  - 全部非 import 改动仅为 gofmt 对齐与测试相对路径深度修复（见 §4）；
  - 修正续作中发现的两处错误（见 §4.1）。

## 3. Pre-move gate（918f90000，前任留存日志 `/tmp/a11-baseline/*.log`）

```
go test ./internal/agent/... -count=1                          → 18 包全 ok，EXIT=0
  （opencode 44.112s；recoverytest 39.531s；trpc ok）
go test ./internal/application/service/memory/... ./internal/modelcontext/... -count=1 → ok / ok，EXIT=0
go test ./internal/application/repository -count=1（agent-run decision 面）            → ok，EXIT=0
go test -race ./internal/agent/recoverytest -count=1 -v     → PASS（21.400s；provider 未设时 SKIP），0 DATA RACE
```

已知 flaky（基线 §2.5）本次均未触发（见 §5）。

## 4. 内容改动披露（repair 变更，续作完成）

- **import 路径修复**：`internal/agent/**`、`internal/modelcontext`、
  `internal/application/service/memory` → `internal/modules/agentruntime/{agent/**,memory,modelcontext}`，
  覆盖 manifest `importers` 全部 21 个包目录中的非禁改文件（含搬入包内部互引）；
  `git diff -w` 下其余改动行全部为 import 行。
- **旧路径别名（4 个，零逻辑）**：`internal/agent/{approval,experts,subagents}/alias.go`、
  `internal/application/service/memory/alias.go`——type alias + var 转发，头注
  `Deleted by Pass B task B-agentruntime`，仅覆盖禁改文件 `internal/container/container.go`
  的引用面（36/37/38/54 行，9 处只读引用，无可变导出 var 赋值，无需 A6 式豁免）。
- **测试相对路径深度修复（搬迁容差，披露）**：目录加深导致 `os.ReadFile`/fixture 相对
  路径基数变化，仅改路径字符串，不改断言：
  `agent/{grounding_prompt_test,prompts_persona_test}.go`（3→4 级）、
  `agent/tools/registry_journal_test.go`、`agent/trpc/checkpoint_test.go`（4→5 级）、
  `agent/experts/pilot_test.go`、`agent/nativecontract/wire_test.go`、
  `agent/subagents/library_test.go`（Join 元素 +2）、
  `internal/application/service/subagent_service_test.go`（testdata 指向新树）。
- **gofmt**：全部触碰文件 `gofmt -l` 为空；含结构体字段对齐重排与
  `agent/opencode/executor_test.go` 两处单行函数体的格式化拆分（该文件在基线即未格式化）。

### 4.1 续作修正项（对前任草稿的两处补丁）

1. 前任对 5 个测试文件的相对路径深度**少算一级**（config ×2、migrations ×2、
   subagents testdata ×1），target 测试 FAIL（`no such file or directory`）；续作逐个
   改正并复测通过。
2. 前任 experts/nativecontract/subagents 三包的 fixture 路径 helper（`pilot_test.go`、
   `wire_test.go`、`library_test.go`）漏改，续作补齐（`filepath.Join` 元素 +2）。

## 5. Post-move 验证

| 检查 | 命令 | 结果 |
|---|---|---|
| manifest test_commands | `go test ./internal/modules/agentruntime/... -count=1` | 21 包全 ok，EXIT=0（opencode 12.977s、recoverytest 19.700s；flaky 未触发） |
| manifest race | `go test -race ./internal/modules/agentruntime/agent/recoverytest -count=1 -v` | PASS（24.747s），0 DATA RACE；provider 未设用例 SKIP（blocked-env） |
| 直接消费方 | `go test ./internal/application/{repository,service}/... ./internal/container/... ./internal/craft/... ./internal/handler/... ./internal/modules/channels/im/... ./internal/types/interfaces/... -count=1` | 40 包全 ok（service 81.014s；`internal/im` 在基线后已不存在，manifest 该行过时） |
| flaky 专项 | `go test ./internal/application/repository -run TestAgentRunDecisionConcurrentOnlyOneRevision -count=1 -v` | PASS（0.34s），留档 |
| 构建 | `go build ./...` | 通过（仅既有 `ld: warning: ignoring duplicate libraries: '-lc++'`） |
| 静态检查 | `go vet ./internal/modules/agentruntime/... ./internal/application/... ./internal/container/... ./internal/craft/... ./internal/handler/... ./internal/types/interfaces/...` | 0 问题 |
| 守卫 | `go run ./tools/architectureguard` | 49 条 forbidden-import（§6）；`literal=564 apiKeyRoute=69 handle=0 total=633`、`redis=23 lite=23`、`hooks=58`、`modules=16`，route/worker/hook 计数与基线一致 |

## 6. 守卫显形的预存耦合（IA3 关注，未加豁免、未改工具）

move 把 `internal/agent/**` 置于 `internal/modules/` 下后，guard 首次扫到这批文件的
跨模块 import（`git grep` 证明 918f90000 的 `internal/agent/**` 已有同样的 import，
属**预存耦合显形**，非本任务新增）。49 条 = airesource/models/chat ×26、
execution/sandbox ×10、channels/im → agentruntime/agent/tools ×2、airesource/mcp ×2、
execution/browserskill ×2、rerank ×1、ollama ×1、commercial ×1、appconnector ×1。
其中除 `commercial`、`appconnector` 外的 7 条路径均在 frozen-entrypoints-batch-a2 §1
登记为冻结入口（guard 的"仅模块根门面"规则比冻结契约更严）。完整清单见
integration/agentruntime.md §10 与本任务 report；全部移交 IA3 Integrator 裁量。

## 7. Review

- [x] 已 Review（final whole-branch review 回填）：确认 move commit 纯 rename、alias 零逻辑、
      无行为变更、测试路径修复仅改相对深度。

**Approved**（Review 范围：task-scoped gate review + batch barrier review）。pass-a 全部 task 均 Approved、四道 barrier 均 Approved；唯一修复轮次为 A9 knowledge——retriever 路径对齐 fb71b3084，复 review ADDRESSED。
结论与证据台账见 `.superpowers/sdd/2026-09-21-backend-modularization-foundation-pass-a/progress.md` 与 `docs/architecture/evidence/batch-a*.md`。

## 8. 遗留与 IA3 关注点

1. §6 的 49 条预存 forbidden-import 待 IA3 裁量（登记精确豁免或 Pass B 收敛）；
   守卫工具未改、未加豁免。
2. 别名收窄为禁改文件引用面（9 符号）；其余 16 个旧路径无禁改引用方、未留别名。
3. Sibling 领地未触碰：`internal/modules/{execution,airesource,agentcatalog,policy,…}`
   零改动、`internal/router/router.go`、`task.go`、`sync_task.go`、
   `internal/container/container.go`、`go.mod`、`go.sum`、`migrations/` 零改动。
