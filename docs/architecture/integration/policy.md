# Integration Brief — policy (IA2)

适用对象：Pass A 集成者（Integrator）。任务：把 `bm-passa-a8` 分支的搬迁成果合入集成线。
本文件只描述 policy 模块（manifest：`docs/architecture/moves/policy.yaml`），
不动其他模块领地。system 模块（同分支）无搬迁，见 `integration/system.md`。

## 1. 本次搬迁（A8 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
|---|---|
| `internal/application/access` | `internal/modules/policy/access` |
| `internal/embedpolicy` | `internal/modules/policy/embedpolicy` |
| `internal/ipclass` | `internal/modules/policy/ipclass` |
| `internal/ratelimit` | `internal/modules/policy/ratelimit` |
| `internal/storageallowlist` | `internal/modules/policy/storageallowlist` |

共 25 个文件（含全部 `_test.go` 与 access 的 README.md），move commit 为纯 rename
（25/25 相似度 100%，0 insertions/deletions）。五包之间无相互导入（已 grep 验证）。

## 2. 旧路径别名（alias_obligations，5 处）

每个旧路径留了一个**零逻辑**转发别名包（type alias + const/var/func 值转发；每文件头部
注明 `Deleted by Pass B task B-policy`）：

| 旧路径（别名包目录） | 转发目标 | 别名文件 |
|---|---|---|
| `internal/application/access` | `internal/modules/policy/access` | `alias.go`（46 符号） |
| `internal/embedpolicy` | `internal/modules/policy/embedpolicy` | `alias.go`（4 符号） |
| `internal/ipclass` | `internal/modules/policy/ipclass` | `alias.go`（15 符号） |
| `internal/ratelimit` | `internal/modules/policy/ratelimit` | `alias.go`（2 符号） |
| `internal/storageallowlist` | `internal/modules/policy/storageallowlist` | `alias.go`（6 符号） |

别名面 = 搬迁包的全部导出符号（AST 枚举，73 个），风格与 A1 appconnector 别名一致。
**当前唯一 load-bearing 的别名是 `internal/storageallowlist`**（禁改文件
`internal/container/container.go:107` 仍导入旧路径）；其余 4 个别名现无任何 importer，
是 manifest ruling 2（别名与 move_packages 一一对应）的义务产物。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单）

Pass A 结束时（本分支 HEAD），禁改共享文件中引用旧导入路径的行**只剩**：

- `internal/container/container.go:107` — `"github.com/Tencent/WeKnora/internal/storageallowlist"`（无 qualifier）
- `internal/container/container.go:1516` — `storageallowlist.Supported()`（该行不需要改，import 切换后自动解析）

`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
`go.mod`、`go.sum`、`migrations/`：零处引用（已 grep 验证；router 对本模块仅经
`internal/router/files.go`（access）与 `internal/router/routes_agent.go`（embedpolicy）两个
**非禁改**文件直接导入，A8 已把这两处切到新路径）。

**给集成者的操作**：
1. 在 `internal/container/container.go:107` 把导入改为
   `"github.com/Tencent/WeKnora/internal/modules/policy/storageallowlist"`（import 行唯一改动，
   包名 qualifier `storageallowlist` 不变，`container.go` 其余行零改动）；
2. `go build ./...` 通过后，删除第 2 节所列 5 个别名目录（连同 alias.go）——它们此刻已全部
   无 importer；
3. 再次 `go build ./...` + `go test ./internal/modules/policy/... -count=1` +
   `go test ./internal/container/... ./internal/application/service/... ./internal/handler/... -count=1` 确认。

## 4. 路由集成点（routes）

**无独立业务路由**（manifest：embed 页面 frame-ancestors 策略，无 policy 专属端点）。
机制挂点（Pass A 后均不在本模块文件内，集成时不要动语义）：

- `embedFrameAncestorsMiddleware` 定义于 `internal/router/routes_agent.go:344`（函数体
  :366 调用 `embedpolicy.FrameAncestors(...)`，import 已切新路径）；
- 挂载点 `internal/router/router.go:196` — `r.Use(embedFrameAncestorsMiddleware(params.EmbedChannelService))`
  （router.go 无本模块 import，仅函数调用）。

基线路由总数 633 不变（guard 复核 literal=564 + apiKeyRoute=69）。

## 5. Workers

**0 项**（manifest `integration_points.workers: []`）。policy 五包无 asynq 任务类型；
`internal/ratelimit` 是被 embed 认证（middleware/embed_auth.go）与 channels im 使用的
**库**（滑动窗口限流器，依赖注入 `*redis.Client`），不是 worker。router/task.go、
router/sync_task.go 无本模块条目。

## 6. container.Invoke 生命周期挂点

**0 项**（manifest `integration_points.lifecycle_hooks: []`；已验证 container.go 对五包
唯一引用是 :1516 的 `storageallowlist.Supported()`，非 Invoke 挂点）。

## 7. 配置键（模块读取）

- `STORAGE_ALLOW_LIST` — `internal/modules/policy/storageallowlist/allowlist.go:21`
  （const `AllowListEnv`；逗号分隔的存储 provider 白名单）。这是五包唯一的 env 键。
- `internal/ratelimit` 构造参数 `keyPrefix/window/instanceID` 由调用方（embed_auth）传入，
  无自有 env/配置键。

## 8. Guard 新增发现（A8 Pass A 不修，交 IA2 处置）

architectureguard 在本分支报 2 条**新** forbidden-import 诊断（基线 0 违规；搬迁把
platform 路径变成模块内部路径，暴露出**先于本次改造**的跨模块耦合）：

- `internal/modules/channels/im/service.go` 导入 `internal/modules/policy/ratelimit`
- `internal/modules/channels/im/yunzhijia/url.go` 导入 `internal/modules/policy/ipclass`

两条的旧路径写法（`internal/ratelimit`、`internal/ipclass`）即 batch A1 集成前 channels
模块的既有依赖；A8 仅按 manifest importers 做了 import 行修复（文件属 channels 领地，
无结构改动）。**处置建议（IA2/Pass B）**：channels 组合根切到模块路径时，参照
appconnector 先例在 guard 加精确路径例外（临时）或改走 policy 模块门面（Pass B）；
不得为过 guard 而回退本搬迁。

## 9. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/policy/... -count=1`（manifest test_commands）
- `go test ./internal/application/service/... ./internal/handler/... ./internal/middleware/... ./internal/sandbox/... ./internal/types/... ./internal/utils/... ./internal/container/... ./internal/router/... ./internal/modules/channels/im/... -count=1`（直接消费方回归）
- `go run ./tools/modulemove verify --module policy`
- `go run ./tools/architectureguard`（路由 633 / worker 23+23 / hooks 58 不得变化；
  第 8 节两条 forbidden-import 诊断在集成处置前允许存在，其余必须 0）
