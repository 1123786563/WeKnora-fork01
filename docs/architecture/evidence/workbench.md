# Evidence — Task A12 workbench（Pass A）

- 分支 / worktree：`bm-passa-a12` / `.worktrees/bm-passa-a12`
- base：`8fbc030a8`（= 集成线 "refactor: integrate pass-a core modules"，IA3 放行）
- move commit：`61e9fa0e7` — `refactor(workbench): move packages to internal/modules/workbench`
- repair commit：`808e8d422` — `refactor(workbench): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未动容器）；Go 1.26.3

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module workbench
modulemove: OK (workbench)
$ go run ./tools/architectureguard        # base 8fbc030a8（bm-passa-main 实测）
literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16
OK (0 violations)
$ go build ./...                          # exit 0（仅既有 cmd ld duplicate-libraries warning）
```

## 2. 测试基线（pre-move，旧路径未动时）

| 命令 | 结果 |
|---|---|
| `go test ./internal/modules/workbench/... -count=1`（manifest test_commands，逐字） | exit 0（骨架包 no test files） |
| `go test ./internal/workbench/... ./internal/notification/... ./internal/voice/... ./internal/application/service/workbench/... -count=1` | 4/4 包 ok（workbench 0.411s、notification 1.277s、voice 1.474s、service/workbench 12.172s），FAIL 0 |
| 直接消费方（targeted）：handler/session `-run 'TestWorkbench\|TestArtifact\|TestMobileWorkbench'`；handler `-run 'TestMobileVoice\|TestVoice'`；router `-run 'TestMobileVoice\|TestWorkbench\|TestArtifact'`；repository `-run 'TestAgentRunSnapshot\|TestExecutionObservation\|TestWorkbench'`；agentruntime/agent 全包 | 全部 ok（1.759s / 1.176s / 1.979s / 8.405s / 1.185s），FAIL 0 |

## 3. 搬迁后（post-move，repair commit 808e8d422 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module workbench` | `modulemove: OK (workbench)` |
| `go run ./tools/modulemove verify --all` | `modulemove: OK (16 manifests verified)` |
| `go test ./internal/modules/workbench/... -count=1`（manifest test_commands） | 4/4 包 ok（模块根 0.375s、notification 1.298s、service/workbench 22.736s、voice 0.539s），FAIL 0 |
| 别名包 ×4 `go test ./internal/application/service/workbench/... ./internal/notification/... ./internal/voice/... ./internal/workbench/...` | 4× `[no test files]`（编译通过） |
| 直接消费方全包：handler/session、handler、router、application/service、container、agentruntime/agent `-count=1` | 6/6 ok（26.730s / 1.617s / 3.622s / 100.174s / 5.624s / 3.172s），FAIL 0 |
| `go test ./internal/application/repository/ -count=1`（全包） | ok 131.376s，FAIL 0（含 TestAgentRunDecisionConcurrentOnlyOneRevision，本轮通过） |
| `go build ./...` | exit 0（仅既有 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet`（模块树 + 4 别名 + container/handler/router/repository/service/cmd 全部消费方） | exit 0，无输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`（与基线一致）+ **14 条新 forbidden-import 诊断**（见 §7；原始输出 `/tmp/a12-guard.log`） |

已知不稳定用例（F0 §2.5）：agent/opencode 套件不在本模块测试面（`internal/agent` 已于
A11 归 agentruntime，本任务未触碰）；`TestAgentRunDecisionConcurrentOnlyOneRevision`
随 repository 全包本轮通过；payment `TestProvidersFromEnvRejectsPartialAlipay` 不在
测试面，未出现。

## 4. Rename 证据

```
$ git show --stat 61e9fa0e7 | tail -1
 36 files changed, 0 insertions(+), 0 deletions(-)
$ git diff --summary 61e9fa0e7~1..61e9fa0e7 | grep -c '^R100\|^ rename'
36
```

- move commit `61e9fa0e7`：36 files changed，**0 insertions(+), 0 deletions(-)** ——
  纯 rename。
- 全范围（base `8fbc030a8..HEAD`）rename 检出 36/36：30×`(100%)`、5×`(99%)`、
  1×`(97%)`。低于 100% 的 6 个文件（全部为 repair commit 所需的披露改动）：
  - `internal/modules/workbench/service/workbench/interaction.go`（99%）、
    `interaction_test.go`（99%）：import 行随路径改名重排（gofmt 排序，函数体零改动）；
  - `notification_delivery.go`（99%）、`notification_delivery_test.go`（99%）：
    1 条 import 路径行（`internal/notification` → `internal/modules/workbench/notification`）；
  - `admission_concurrency_test.go`（99%）：sqlite migrations fixture 相对深度修复
    `../../../../` → `../../../../../`（目录加深 2 级、变浅 1 级，净 +1）；
  - `crosslang_mx003_test.go`（97%）：fixture 相对深度修复
    `../../tests/mobile-v2/fixtures/mx-003-crosslang.json` → `../../../...`（净 +1 级）。
- 函数体改动：**零**。`git diff -U0` 全范围逐行审计，非 import 行内容变更仅 2 类：
  上述 2 处测试相对路径深度（任务规程明示容忍、须披露），以及消费方测试
  `internal/handler/session/workbench_overview_mx013_test.go` 的 1 处 gofmt 单空格
  struct tag 对齐重排（纯空白，`git diff -w` 下消失）。
- gofmt：全部被触碰文件 + 4 个 alias.go 已格式化；`gofmt -l` 残留 14 个文件全部为
  未触碰文件的仓库既有噪音（其中 3 个搬迁文件 crosslang_mx003_test.go、overview.go、
  session_test.go 经 base 版本 gofmt 探针证实 pre-existing，未予处理以保 rename 纯度）。

## 5. 禁改共享文件未动证明

```
$ git diff --stat 8fbc030a8..HEAD -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/
（空输出 —— 零差异）
```

`internal/container/container.go:39/106/113` 仍指向旧路径并由别名包解析 —— 切换操作见
`docs/architecture/integration/workbench.md` §3。其余 25 个修改文件均在 manifest
`owned_files.importers`（含 `internal/router` 内**非禁改**的
`routes_mobile_voice_test.go`）+ 2 个搬迁文件自引用修复 + 4 个新建 alias.go。

Sibling 领地（craft=internal/modules/craft、insights=internal/modules/insights 相关路径）
零 diff；frozen 模块 internal/modules/{knowledge,conversation,agentruntime} 零 diff
（agentruntime/agent/engine_test.go 未触碰，由 service/workbench 别名继续解析）。

## 6. 假设与偏差记录

1. **任务 brief 缺失**：`task-A12-brief.md` 在指定路径不存在；从 plan
   `docs/plans/2026-09-21-backend-modularization-foundation-pass-a.md` 用
   extract-brief.sh 提取 Task A12 小节（4 项 checklist），与任务指令一致，按指令执行。
2. **别名面收窄 + internal/workbench 零符号 stub**（沿 A5 "Forbidden files only"
   先例，integration brief §2 已述）：service/workbench 别名 10 符号 = container.go:39
   8 符号 + frozen `agentruntime/agent/engine_test.go:11` 2 符号
   （`NewGormInteractionStore`、`NewInteractionServiceWithApproval`）；notification 1、
   voice 2 同理；`internal/workbench` 无不可修复引用方，alias.go 仅文档，IA4 可直接删。
3. **frozen agentruntime 消费面**：`engine_test.go:11` 导入 service/workbench 旧路径；
   按任务边界未修改该文件，以别名保持编译（IA4 切换 container.go 时建议同步切换该行，
   见 integration brief §3 步骤 2）。
4. **无 mutable-var 翻转**：container.go 对本模块符号仅构造函数/类型/组合字面量引用，
   无对可变导出 var 的赋值，未触发 task-A6 先例条款。
5. 未使用 guard 例外、未修改 architectureguard/modulemove 工具；未 dispatch 子代理；
   go.mod/go.sum/migrations 零 diff。

## 7. Guard 新增发现（交 IA4，未处置）

```
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/admission.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/admission.go 导入了模块 commercial 的内部包 "github.com/Tencent/WeKnora/internal/modules/commercial"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/admission.go 导入了模块 commercial 的内部包 "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/admission.go 导入了模块 execution 的内部包 "github.com/Tencent/WeKnora/internal/modules/execution"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/interaction.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/interaction.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/notification.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/notification_worker.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_dispatch.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_dispatch.go 导入了模块 commercial 的内部包 "github.com/Tencent/WeKnora/internal/modules/commercial"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_usage.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_usage.go 导入了模块 commercial 的内部包 "github.com/Tencent/WeKnora/internal/modules/commercial"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_usage.go 导入了模块 commercial 的内部包 "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/workbench/service/workbench/remote_usage.go 导入了模块 execution 的内部包 "github.com/Tencent/WeKnora/internal/modules/execution"（跨模块只能经模块根公共门面）
```

逐条核验：14 条的 import 行在 base `8fbc030a8` 的旧路径同名文件中**逐字符相同**
（`git show 8fbc030a8:internal/application/service/workbench/<file>` 比对，见
admission.go:14-17、interaction.go:14-15、notification.go:7、notification_worker.go:10、
remote_dispatch.go:9-10、remote_usage.go:12-15）——先于模块化改造的既有耦合，被
搬迁进 `internal/modules/` 后进入 guard 检查范围。按 A8 §7 同一模式处置：未加例外、
未改工具；路由/worker/hook 计数与基线完全一致（633 / 23+23 / 58），其余检查 0 违规。
处置建议：Pass B B-workbench 收敛为模块门面依赖，或由 IA4 按 45fc6f9b1 先例登记
精确路径例外。

## 8. Review 结果

**Approved**（final whole-branch review 回填；Review 范围：task-scoped gate review + batch barrier review）。
pass-a 全部 task 均 Approved、四道 barrier 均 Approved；唯一修复轮次为 A9 knowledge——retriever 路径对齐 fb71b3084，复 review ADDRESSED。
结论与证据台账见 `.superpowers/sdd/2026-09-21-backend-modularization-foundation-pass-a/progress.md` 与 `docs/architecture/evidence/batch-a*.md`。
