# Evidence — Task A6 airesource（Pass A）

- 分支 / worktree：`bm-passa-a6` / `.worktrees/bm-passa-main/.worktrees/bm-passa-a6`
- base：`03032f890`（= 集成线起点，batch A1 集成后）
- move commit：`3a14875d4` — `refactor(airesource): move packages to internal/modules/airesource`
- repair commit：`e9e2368db` — `refactor(airesource): repair imports and add pass-a aliases`
- docs commit：见本文件与 `docs/architecture/integration/airesource.md` 所在提交
- 环境：postgres :5432 / redis :6379（dev compose，未动容器）；Go 1.26.x

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module airesource
modulemove: OK (airesource)
```

## 2. 测试基线（pre-move，旧路径，base `03032f890`）

| 命令 | 结果 |
|---|---|
| `go test ./internal/models/... -count=1` | 7 包 ok（asr/chat/embedding/limiter/provider/rerank/vlm），utils 与 utils/ollama no test files，exit 0 |
| `go test ./internal/mcp/... -count=1` | ok 0.956s |
| `go test ./internal/infrastructure/web_search/... ./internal/storageurl/... -count=1` | ok 0.781s / ok 0.650s |

已知不稳定用例（F0 §2.4：`agent/opencode` 挂起、`TestAgentRunDecisionConcurrentOnlyOneRevision`、
payment `TestProvidersFromEnvRejectsPartialAlipay`）均不在搬迁包测试面内；本轮 `agent/opencode`
实际通过（41.8s / 13.4s 两次）。

## 3. 搬迁后（post-move，新路径，repair commit `e9e2368db` 之上）

| 命令 | 结果 |
|---|---|
| `go run ./tools/modulemove verify --module airesource` | `modulemove: OK (airesource)` |
| `go test ./internal/modules/airesource/... -count=1`（manifest test_commands） | 10/10 包 ok（mcp 3.13s、asr 3.67s、chat 2.93s、embedding 4.81s、limiter 3.84s、provider 4.01s、rerank 3.91s、vlm 3.61s、storageurl 4.41s、web_search 3.81s），FAIL 0 |
| 直接消费方批次 1：`go test ./internal/container/... ./internal/handler/... ./internal/modelcontext/... ./internal/types/... ./internal/infrastructure/docparser/... ./semantic/experiments/model_gateway/... ./internal/modules/channels/im/... -count=1` | ok 21 / FAIL 0，exit 0 |
| 直接消费方批次 2：`go test ./internal/application/... -count=1` | 全 ok（application/service 135.9s），FAIL 0 |
| 直接消费方批次 3：`go test ./internal/agent/... -count=1 -timeout 600s` | **18/18 包 ok**（含 opencode 13.4s、recoverytest 21.3s），FAIL 0 |
| `go build ./...` | exit 0（仅既有的 cmd/desktop、cmd/server ld duplicate-library warning，与基线一致） |
| `go vet`（modules/airesource、infrastructure、models、mcp、storageurl、container、handler、agent、application、modelcontext、types、semantic、channels/im） | exit 0，无输出 |
| gofmt（177 个修改文件 + 12 个 alias.go） | 全部 clean |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`，与 F0 基线一致（计数不变）；另有 1 条 forbidden-import 记录，见 §6 |

## 4. Rename 证据

```
$ git diff --summary 03032f890 3a14875d4 | grep -c '^ rename'   # move commit 单独
188
$ git diff --summary 03032f890 3a14875d4 | grep -v '^ rename' | wc -l
0
$ git diff --summary -M100% 03032f890 3a14875d4 | grep -c '^ rename'   # 全部 100% 相似度
188
```

- move commit `3a14875d4`：188 files changed, **0 insertions(+), 0 deletions(-)** —— 纯 rename。
- repair commit `e9e2368db`：189 files（177 修改 + 12 新建 alias.go），727+/230−。
- 修改文件内容纯度：全部 460 条变更行（230+/230−）逐行审计**全部落在 import 块**
  （旧路径字符串 → 新路径字符串，含 gofmt 对 import 块内行的字典序重排），
  qualifier 保持不变；**非测试 .go 函数体零改动**；搬迁树的 `_test.go` 无相对路径
  引用（grep `os.ReadFile`/`../` 零命中），无需 A1 那样的测试路径深度修正。

## 5. 禁改共享文件触碰证明（1 处已披露偏差）

```
$ git diff 03032f890 HEAD --stat -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go go.mod go.sum migrations/
（空输出 —— 零差异）
$ git diff 03032f890 HEAD -- internal/container/container.go | grep -E '^[+-][^+-]'
-	"github.com/Tencent/WeKnora/internal/models/chat"
+	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
```

**偏差（唯一，自主裁定并已向协调者报备）**：`internal/container/container.go` import 块
仅翻转 `internal/models/chat` 一行指向新路径。原因：`container.go:1133` 附近
（`registerChatLocalImageResolver`）对 `chat.LocalImageResolver` **赋值**，而该符号是搬迁包的
可变导出 var（`internal/modules/airesource/models/chat/image_resolve.go:63`，由
`readLocalStorageBytes` 在同包读取）。Go 没有 var alias，"var 转发"别名只能拷贝初值，
赋值会落在别名副本上，静默丢失多租户 `local://` 图片解析 wiring（行为回归）。翻转这一行
使赋值直达唯一真实 var，行为零变化；这正是 IA2 集成清单中的一步，提前执行并在此披露。
container.go 其余部分零 diff（行号未漂移：hooks 仍在 :439/:604/:615，函数体 :1216/:1232/:2263）。
其余 5 行 container.go import（web_search/mcp/embedding/limiter/utils-ollama）仍走别名，
交 IA2 按 Integration Brief §3 处理。

## 6. architectureguard 新增 forbidden-import（记录给 IA2，未修、未加例外）

```
forbidden-import: internal/modules/airesource/models/chat/usage.go 导入了模块 commercial 的
内部包 "github.com/Tencent/WeKnora/internal/modules/commercial"（跨模块只能经模块根公共门面）
```

核实：该 import 在搬迁前即存在（`git show 03032f890:internal/models/chat/usage.go` 第 9 行
同内容），属 **pre-existing 耦合被搬迁暴露**（usage.go 从非模块内部包变为模块内部包后触发
检查）。按任务步骤 5 原样记录，不改工具、不加例外，归 IA2 裁定（候选方向：commercial
门面收敛或 usage fact 适配器外移，均超出 A6 scope）。

## 7. 别名包（12 处，零逻辑）

每旧路径一个 `alias.go`：type alias + const/var/func 值转发 + 泛型函数一行委托
（`internal/models/utils`：`ChunkSlice`/`MapSlice`，泛型函数无法用 var 转发）。
全导出面枚举（go/ast，非测试文件），共 **385 符号**：
web_search 38、mcp 33、asr 9、chat 40、embedding 50、limiter 10、provider 110、rerank 49、
utils 3、utils/ollama 4、vlm 11、storageurl 28。
头部均注明 `Deleted by Pass B task B-airesource`。
**已知残留 importer**（别名为其解析，IA2 翻转后即可删除全部别名）：
`internal/container/container.go` 5 行、`internal/modules/channels/im/service.go` 2 行
（channels 文件属其他模块领地，A6 未触碰 —— manifest 声明的 importer `internal/im` 已在
batch A1 迁移为 `internal/modules/channels/im`）。

## 8. 假设与偏差记录

1. container.go 单行 import 翻转（见 §5）——唯一禁改文件触碰，行为零变化，已披露。
2. `internal/modules/channels/im/service.go`（manifest importer `internal/im` 的 A1 后身）
   **未**做 import 修复：该文件现属 channels 模块 owned 领地，并行批次下不修改他模块
   owned 文件；别名桥接保证编译与测试，交接 IA2（Integration Brief §3 第 2 步）。
3. 别名面取全量导出符号（385），manifest 未指定最小面；与 A1 口径一致。
4. `TestCrashMatrixSQLite/unknown_result_user_retry` 首轮失败：三套重型测试并发跑时的
   资源竞争（该包 167s vs 单独 19s）；单独重跑 ×2（含全 agent 套件 solo 轮）全绿。
   判定为负载性 flake，非本搬迁回归 —— 搬迁仅改 import 路径，无逻辑变化。已记录。
5. 未使用任何工具改动 architectureguard/modulemove；未 dispatch 子代理；未触碰
   A5/A7/A8 领地（execution/agentcatalog/system/policy 相关路径零 diff）。

## 9. Review 结果

（占位 —— 等待独立 review；review 意见回填此处。）
