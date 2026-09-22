# agentcatalog — Integration Brief（Pass A → IA1 / Pass B）

事实源：`docs/architecture/moves/agentcatalog.yaml`。本文是 Pass A worker A7 交给集成任务
IA1 与 Pass B（`B-agentcatalog`）的交接说明。

**本文最重要的一句话：agentcatalog 在 Pass A 是零搬迁模块。**
manifest `move_packages: []`、`owned_files.move_sources/move_targets/importers: []`，
与 F0 `backend-modules.yaml:281` 的声明一致（`packages: []  # 无独立包；见
internal/application/service 拆分注记`）。IA1 在 batch A2 集成中**无需为 agentcatalog
做任何路径切换、别名删除或导入修复**；本模块的归位在 Pass B 文件拆分时完成。

## 1. 搬迁结果（from → to）

无。没有 `git mv`、没有新别名包、没有 import 路径修复；两次 refactor 提交不存在，
本分支仅含文档提交。模块现有内容仍是 Pass A 骨架（commit 35ee7e088 引入）：

- `internal/modules/agentcatalog/module.go` — 零逻辑门面骨架（`NewModule` /
  `RegisterRoutes` / `RegisterWorkers` / `Start` / `Stop` 契约注释，无实现无导入）
- `internal/modules/agentcatalog/README.md`、`legacy/README.md`（legacy 索引，逐条镜像
  manifest legacy_files）

## 2. 边界裁定：internal/agent 子包不属于本模块（A7 → IA2 澄清）

任务简报模板提到 "experts/persona/subagents/skills/catalog packages"，但**绑定事实源
（manifest wins）**将 `internal/agent` 全树归 agentruntime：`docs/architecture/moves/
agentruntime.yaml` move_packages 明确列出 `internal/agent` →
`internal/modules/agentruntime/agent` 及其全部子包，包括：

| 包 | 归属 | 内容佐证 |
|---|---|---|
| `internal/agent/experts` | agentruntime（A11） | builtin expert 模板库 + AgentRelease bundle 导出（agent_release.go） |
| `internal/agent/persona` | agentruntime（A11） | MBTI 人格档案 / 28 题测验（profile.go） |
| `internal/agent/skills`（含 `skills/skillhub`） | agentruntime（A11） | skill env resolver、运行时 skill 注入 |
| `internal/agent/subagents` | agentruntime（A11） | subagent executor/library（executor.go） |
| `internal/agent/catalog` | 不存在 | 仓库中无此包 |

这些子包被 `internal/agent` 引擎本体大量导入，随最长已拥有祖先包（`internal/agent`，
agentruntime 的 move_sources）整体搬移，符合 moves/README.md 规则 1。**A7 未触碰
`internal/agent` 下任何文件**；若 IA2 看到 A11 的搬移改变了上述路径，属预期，不是回归。

本模块当前的业务实现全部以 legacy_files 形式困在横向包里（§5），Pass B
`B-agentcatalog` 拆分归位。

## 3. 路由（11 个注册入口，55 条路由 = F0 by_owner.agentcatalog:55）

| 入口（manifest integration_points.routes） | 位置 | literal | apiKeyRoute | 小计 |
|---|---|---|---|---|
| RegisterCustomAgentRoutes | internal/router/routes_agent.go:22 | 8 | 1 | 9 |
| RegisterUserFavoriteRoutes | internal/router/routes_agent.go:61 | 3 | 0 | 3 |
| RegisterSkillRoutes | internal/router/routes_agent.go:76 | 7 | 0 | 7 |
| RegisterAgentVersionRoutes | internal/router/routes_agent_versions.go:29 | 0 | 3 | 3 |
| RegisterAgentMarketplaceRoutes | internal/router/routes_agent_marketplace.go:21 | 1 | 3 | 4 |
| RegisterPersonaRoutes | internal/router/routes_persona.go:24 | 5 | 2 | 7 |
| RegisterExpertRoutes | internal/router/routes_expert.go:21 | 3 | 0 | 3 |
| RegisterSubagentRoutes | internal/router/routes_subagent.go:26 | 2 | 3 | 5 |
| RegisterSkillMarketRoutes | internal/router/routes_skill_market.go:32 | 5 | 1 | 6 |
| RegisterTenantSkillMarketRoutes | internal/router/routes_tenant_skill_market.go:31 | 3 | 1 | 4 |
| RegisterTenantExpertMarketRoutes | internal/router/routes_tenant_expert_market.go:35 | 3 | 1 | 4 |
| **合计** | | **40** | **15** | **55** |

行号/计数在基线 03032f890 逐条复核（AST 同口径：literal `.GET/...` + `g.apiKeyRoute`，
不含 `apiKeyGroup`）。`routes_agent.go` 文件级 88 条路由中仅上表 3 个入口（19 条）归
本模块，其余（Organization 32、EmbedPublic 19、EmbedChannel 9、IM 2+8）归其他模块。

## 4. Workers / 生命周期

- **Workers：0**（manifest `integration_points.workers: []`；本模块无 asynq 任务类型，
  Redis/Lite 拓扑无 agentcatalog 条目）。
- **生命周期挂点：1** — `startTenantSkillReaper`：
  - Invoke 调用：internal/container/container.go:702（`must(container.Invoke(startTenantSkillReaper))`）
  - func 定义：internal/container/container.go:2405（nil-safe；`svc.Start(ctx)` 失败仅
    Warnf 不阻断容器启动；清理名 `"TenantSkillReaper"` 注册到 ResourceCleaner，
    `svc.Stop()` 优雅停止；卡死安装/孤儿快照 cron 在 TenantSkillService 内部）。

## 5. legacy_files（Pass B `B-agentcatalog` 的拆分清单，共 55 个非测试 .go）

同目录 `_test.go` 随主题文件一并拆分（manifest 规则 3）；完整清单见
`internal/modules/agentcatalog/legacy/README.md`（逐条镜像）。

| 目录 | 文件数 | 主题 |
|---|---|---|
| internal/application/repository | 11 | agent_marketplace / agent_share / agent_version / custom_agent / expert_install / published_expert / published_skill / tenant_disabled_shared_agent / tenant_skill / tenant_subagent / user_resource_favorite |
| internal/application/service | 31 | agent（browser_preferences/marketplace/service/share/version）、custom_agent、expert（market_source/service/skills）、skill_market、subagent_service、tenant_expert_market、tenant_skill_*（admin/bundle/catalog/effective/env_declare/files/install/market_service/progress/reaper/remove/runtime_verify/service/source/steer/stop/transcript/verify）、user_resource_favorite |
| internal/handler | 13 | agent_marketplace / agent_version / custom_agent / expert / persona / shared_agent_access / skill_catalog / skill_handler / skill_market / subagent / tenant_expert_market / tenant_skill_market / user_resource_favorite |

相关 schema 位于禁改目录 `migrations/versioned/`（如 000047_user_resource_favorites、
000086_tenant_skills、000187_agent_versions、000188_tenant_agent_marketplace）——
Pass A 未触碰，Pass B 也不得改 schema，仅拆 Go 代码。

## 6. 配置键

本模块在 `internal/config/config.go` 只有 M4 skill market 一节：

- `skillhub_market.host`（`SkillHubMarketConfig.Host`，默认
  `https://api.skillhub.cn`，`applySkillHubMarketDefaults` @ config.go:1677 补默认）
- `skillhub_market.timeout_seconds`（默认 30；nil-safe 访问器
  `Config.SkillHubMarketHost()` @1697、`Config.SkillHubMarketTimeout()` @1710）

其余目录数据（agent 定义、版本、persona、expert、skill 文件、收藏）均存 DB，
无新增环境变量。

## 7. 禁改文件中的本模块触点（file:line 清单 + 切换指引）

Pass A 全程未改任何禁改文件（evidence §6）。当前 agentcatalog 在禁改文件中的
全部真实触点：

| 文件:行 | 触点 |
|---|---|
| internal/router/router.go:394-404 | 11 个 `Register*Routes(v1, params.XxxHandler, rbacGuards)` 委托调用 |
| internal/container/container.go:702 | `must(container.Invoke(startTenantSkillReaper))` |
| internal/container/container.go:2402-2416 | `startTenantSkillReaper` func 定义（2405 签名行） |
| go.mod / go.sum | 无 agentcatalog 独占依赖（无独立包） |
| migrations/ | schema 归属 migrations 目录本身，任何任务不得改 |

### IA1（batch A2）切换指引

**无事可做。** 本模块零搬迁、零别名：IA1 不需要改 router.go、container.go 或任何
导入路径。验收仅需：

```
go test ./internal/modules/agentcatalog/... -count=1   # 骨架包，no test files，exit 0
go run ./tools/modulemove verify --module agentcatalog  # OK
```

### Pass B（B-agentcatalog）切换指引

1. 按 §5 清单把 55 个 legacy 文件（连同同目录 `_test.go`）从横向包拆入
   `internal/modules/agentcatalog/...`，填充 `module.go` 门面契约。
2. router.go:394-404 的 11 行委托改为模块 `RegisterRoutes`；container.go:702 的
   Invoke 改为模块 `Start`/`Stop`（TenantSkillReaper 语义不变）。
3. 同步更新 `internal/modules/agentcatalog/legacy/README.md`（删除已拆条目）。
