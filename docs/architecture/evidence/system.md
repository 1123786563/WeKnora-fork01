# Evidence — Task A8 system（Pass A）

- 分支 / worktree：`bm-passa-a8` / `.worktrees/bm-passa-main/.worktrees/bm-passa-a8`
- base：`03032f890`（= 集成线 "refactor: integrate pass-a batch a1"）
- move commit：**无**（manifest `move_packages: []`）
- repair commit：**无**（无搬迁即无 import 修复、无别名包）
- docs commit：见本文件所在提交
- 环境：postgres :5432 / redis :6379（共享本地服务，未动容器）；未 start/stop 任何容器

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module system
modulemove: OK (system)
```

## 2. 测试基线（pre-move 与 post-move 相同 —— 零代码变更）

| 命令 | 结果 |
|---|---|
| `go test ./internal/modules/system/... -count=1`（manifest test_commands，逐字） | exit 0（`internal/modules/system` [no test files]，F1 骨架） |
| `go build ./...` | exit 0（仅既有的 cmd/server、cmd/desktop ld duplicate-library warning，与基线一致） |
| `go run ./tools/architectureguard` | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`，`OK (0 violations)` —— 与 F0 基线一致 |

## 3. 结论：documentation-only Pass A 贡献（合法结果，依任务边界记录）

- F0 判定 system 拥有 **0 个专属包**：初始化、System Setting、部署能力、系统管理、
  housekeeping 代码全部以 legacy_files 形式落在横向包内：
  - `internal/application/repository/system_setting.go`
  - `internal/application/service/system_setting.go`
  - `internal/handler/deployment_capabilities.go`
  - `internal/handler/initialization.go`
  - `internal/handler/system.go`
- manifest 未声明任何 move_packages，任务边界明确"Config/middleware/router/container 文件
  不得编辑，共享文件只索引不触碰"，故本模块 Pass A 产出为：
  1. `docs/architecture/integration/system.md`（Integration Brief：43 条路由的逐文件归属、
     1 个生命周期挂点、配置键、禁改文件内 file:line 清单与 Pass B 切换点）；
  2. 本 evidence 文件；
  3. 零代码 diff、零共享文件触碰（git diff 可证）。

## 4. 禁改共享文件未动证明

```
$ git diff --stat 03032f890..HEAD -- internal/router/ internal/container/ go.mod go.sum migrations/ internal/config/ internal/middleware/
（空输出 —— 零差异）
$ git diff --stat 03032f890..HEAD -- internal/
（空输出 —— 本模块全部 internal/ 零差异）
```

`internal/container/container.go:641-642,2384-2403`（startHousekeepingService 挂点）与
`internal/router/router.go:385-388`（三条 system 注册函数挂载）保持原样，作为 Pass B
`B-system` 的切换点记录于 integration/system.md §4/§6。补充：`migrations/versioned/
000053_system_admin_and_settings.{up,down}.sql` 属本模块域 schema（system_settings 表），
`migrations/` 为禁改目录，未触碰。

## 5. 假设与偏差记录

1. **housekeeping 归属分裂（原样记录，未自行裁量）**：manifest 把 `startHousekeepingService`
   hook 记在 system 名下，但其实现 `service.HousekeepingService` 的文件
   `internal/application/service/knowledge_housekeeping.go` 在 knowledge.yaml legacy_files
   （:203）名下。Pass A 不改 manifest、不动 container.go；口径说明见 integration/system.md §4。
2. **baseline 计数核对**：17（RegisterInitializationRoutes 全 apiKeyRoute）+ 8
   （RegisterSystemRoutes 字面量）+ 18（RegisterSystemAdminRoutes，含 audit-log 1 条）
   = 43，与 backend-baseline.md §3.2 per-owner split "system 43" 一致；audit 端点域归
   identity 的分裂注记与基线一致。
3. 未 dispatch 子代理；未触碰其他 worker 领地（execution/airesource/agentcatalog 相关
   路径零 diff）。

## 6. Review 结果

**Approved**（final whole-branch review 回填；Review 范围：task-scoped gate review + batch barrier review）。
pass-a 全部 task 均 Approved、四道 barrier 均 Approved；唯一修复轮次为 A9 knowledge——retriever 路径对齐 fb71b3084，复 review ADDRESSED。
结论与证据台账见 `.superpowers/sdd/2026-09-21-backend-modularization-foundation-pass-a/progress.md` 与 `docs/architecture/evidence/batch-a*.md`。
