# Evidence — Task A13 craft（Pass A）

- 分支 / worktree：`bm-passa-a13` / `.worktrees/bm-passa-a13`
- base：`8fbc030a8`（= IA3 集成线 HEAD "refactor: integrate pass-a core modules"）
- move commit：`b51d41293` — `refactor(craft): move packages to internal/modules/craft`
- repair commit：`e389562a0` — `refactor(craft): repair imports and add pass-a aliases`
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未启停容器）；未使用子代理

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module craft
modulemove: OK (craft)
```

manifest test_commands 仅一条（`go test ./internal/modules/craft/... -count=1`）；
pre-move 以旧路径等价形式执行（§2）。

## 2. 测试基线（pre-move，旧路径未动时）

| 命令 | 结果 |
|---|---|
| `go test ./internal/craft/... -count=1`（manifest 首条的旧路径等价） | ok 0.782s |
| `go test ./internal/application/repository -count=1` | ok 135.990s |
| `go test ./internal/application/service -count=1` | ok 87.471s |
| `go test ./internal/container -count=1` | ok 3.129s |
| `go test ./internal/handler -count=1` | ok 1.236s |
| `go test ./internal/handler/session -count=1` | ok 12.226s |
| `go test ./internal/modules/agentruntime/agent/tools -count=1` | ok 3.670s |
| `go test ./internal/modules/agentruntime/agent/opencode -run 'Craft' -count=1`（定点；全套为 F0 §2.5 已知不稳定） | ok 1.113s |

全部 exit 0。本轮未出现 TestAgentRunDecisionConcurrentOnlyOneRevision /
TestProvidersFromEnvRejectsPartialAlipay flaky 面。

## 3. 搬迁后（post-move，repair commit e389562a0 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module craft` | `modulemove: OK (craft)` |
| `go test ./internal/modules/craft/... -count=1`（manifest test_commands 逐字） | ok 0.695s（首轮 FAIL 1 条为 testdata 相对路径深度，见 §6.1，修复后复跑全绿） |
| 直接消费方（同 §2 七项，路径不变） | ok 7/7：repository 142.918s、service 86.394s、container 3.151s、handler 1.164s、session 13.345s、tools 3.455s、opencode(Craft 定点) 0.731s，FAIL 0 |
| `go build ./...` | exit 0（仅既有 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet ./internal/craft/... ./internal/modules/craft/... ./internal/container/... ./internal/handler/... ./internal/handler/session/... ./internal/application/repository/... ./internal/application/service/... ./internal/modules/agentruntime/agent/{opencode,tools}/...` | exit 0，无输出 |
| gofmt（86 个修复文件 + alias.go + skill_test.go） | `gofmt -l` 零输出 |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`（与基线一致）+ **4 条新 forbidden-import 诊断**（见 §7） |

## 4. Rename 证据

```
$ git show --stat b51d41293 | tail -1
  47 files changed, 0 insertions(+), 0 deletions(-)
$ git diff --summary -M b51d41293~1..b51d41293 | grep -c "rename"
47
$ git diff --summary -M b51d41293~1..b51d41293 | grep -v "(100%)"
（空输出 —— 47/47 全部 100% 相似度）
```

- move commit `b51d41293`：47 files（41 .go + 6 testdata），**0 insertions / 0 deletions**，
  47/47 rename 100% —— 纯 rename，函数体零改动。
- repair commit `e389562a0`：88 files，106+/89−。构成：
  - 86 个 importer 文件 import 行修复（importer 目录内 87+/87− = 172 行变更，其中
    **172 行全部**是 `"github.com/Tencent/WeKnora/internal/(modules/)craft"` import 路径行；
    唯一非 import 变更是 craft_workspace_guard_test.go 内一处 gofmt 结构体字段对齐空格
    （`Files: []` → `Files:  []`），纯空白，见 §6.3）；
  - `internal/craft/alias.go` 新建 +17（零逻辑转发，7 符号 = container.go 精确引用面）；
  - `internal/modules/craft/skill_test.go` 2+/2−（testdata 相对路径深度修复，§6.1）。
- import 行修复覆盖面与 grep 全集精确相等：全仓 import `internal/craft` 的 .go 文件 87 个，
  修复 86 个，剩余 1 个为禁改的 `internal/container/container.go`（由别名解析，IA4 切换）。

## 5. 禁改共享文件未动证明

```
$ git diff --stat 8fbc030a8..HEAD -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/
（空输出 —— 零差异）
$ git diff 8fbc030a8 HEAD -- internal/container/container.go | wc -l
0
```

`internal/container/container.go:42` 仍指向旧路径并由别名包解析 —— 集成者操作见
`docs/architecture/integration/craft.md` §3。

## 6. 假设、偏差与事件记录

1. **测试相对路径深度修复（任务规程明文容忍，披露）**：skill_test.go:167/:175 读取
   `../../skills/craft-web-report/{manifest.json,SKILL.md}`；包从 `internal/craft` 深化到
   `internal/modules/craft` 后改为 `../../../skills/...`（2 行，非函数体改动）。
2. **sed 事件（已当场恢复，最终零残留）**：执行 importer 修复时，container.go 的排除
   过滤模式 `'^./internal/container/container.go$'` 写错（BSD grep 输出路径无 `./` 前缀，
   模式第二个字符 `/` 与实际行首 `i` 后的 `n` 不匹配，排除失效应于 0 行），sed 一度改写
   禁改文件 container.go 的 import 行。发现后立即 `git restore internal/container/container.go`
   恢复，**未进入任何 commit**；§5 的零 diff（对 base 8fbc030a8）为最终证明。修复后以
   `grep -v "internal/container/container.go"` 纯字符串过滤重建修复清单（86 文件）。
3. **gofmt 对齐空格**：craft_workspace_guard_test.go 一处结构体字面量字段对齐因 import 行
   变更被 gofmt 重排（纯空白，2 行审计中已列）。
4. **别名面取最小精确面**（7 符号 = container.go 引用面，同 A11 口径；A8 曾取全量导出面）：
   旧路径当前唯一 importer 是禁改的 container.go；manifest ruling 2 只要求 1:1 别名包存在，
   面大小 worker 裁量。无可变导出 var，A6 式赋值翻转豁免不适用。
5. manifest `owned_files.importers` 中 `internal/agent/opencode`、`internal/agent/tools` 为
   batch A3 前的旧路径，IA3 后实际位于 `internal/modules/agentruntime/agent/{opencode,tools}`，
   按同一 importer 语义仅做 import 行修复（A8/A11 同类先例）。
6. 未使用 guard 例外、未修改 architectureguard/modulemove 工具；未触碰 workbench（A12）、
   insights（A14）领地；冻结契约 Task/Timeline/Artifact 签名零变化（move commit 100% rename
   为证）。

## 7. Guard 新增发现（交 IA4，未处置；完整输出 /tmp/a13-guard.log）

```
architectureguard: forbidden-import: internal/modules/agentruntime/agent/opencode/executor.go 导入了模块 craft 的内部包 "github.com/Tencent/WeKnora/internal/modules/craft"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/agentruntime/agent/opencode/normalizer.go 导入了模块 craft 的内部包 "github.com/Tencent/WeKnora/internal/modules/craft"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/agentruntime/agent/tools/craft_delegate.go 导入了模块 craft 的内部包 "github.com/Tencent/WeKnora/internal/modules/craft"（跨模块只能经模块根公共门面）
architectureguard: forbidden-import: internal/modules/craft/contracts.go 导入了模块 agentruntime 的内部包 "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"（跨模块只能经模块根公共门面）
```

均为先于模块化改造的既有耦合被"platform 路径→craft 模块内部路径"的搬迁显形
（前 3 条旧路径写法为 `internal/craft`，第 4 条 importer 旧路径为 `internal/craft`、被导入
路径搬迁前后不变；已用 `git show 8fbc030a8:<file>` 逐条证实）。路由/worker/hook 计数与
基线完全一致，其余检查 0 违规。按任务边界未加例外、未改工具；处置建议见
`integration/craft.md` §8。

## 8. Review 结果

（占位 —— 等待独立 review；review 意见回填此处。）
