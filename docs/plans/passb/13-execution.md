# Pass B 子计划 13-execution — B1-EX Execution 边界（Sandbox/Target/Workspace/Terminal/Browser）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 ownership-matrix 冻结给 13-execution 的 21 个横向宿主 legacy 文件中的 18 个迁入 `internal/modules/execution/{repository,service,handler}`，跨 owner 未导出耦合改为窄端口/注入 + 原路径无逻辑兼容残差（IB1 删除），`*Handler` 方法去方法化，全部节点门禁绿色并产出差分证据与 Integration Brief；3 个文件（execution_cleanup/dispatch/observation.go）因 agentruntime 子包依赖在无新例外前提下不可合法迁入，登记推迟裁定（§0.4）。

**Architecture:** 纯搬迁 + 端口注入 + 兼容残差，不改任何外部可观察行为（路由、WebSocket 协议、错误码、SQL、worker/hook 计数全部不变）。router/container/module.go 门面实现一概不动（集成工程师独占，conventions §3），本节点交付 Brief 由 IB1 切换。

**Tech Stack:** Go 1.26, Gin, GORM, dig, gorilla/websocket, jwt/v5, Testify, git worktree。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§5.8 Execution 职责、§4.4 两遍迁移、§13 提交隔离、§14.3 高风险差分、§17.2 完成标准）。

---

## 0. 事实源与前置条件

### 0.1 事实源指针（撰写本计划时全部实读）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | 目标架构与约束 |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | B1-EX 表项（93 行）、全局约束、IB1 定义 |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | 派发契约/门禁/禁改/提交/升级/差分/耦合处理/计数口径 |
| `docs/plans/passb/execution-dag.json` 节点 `b1-execution` | owned_files/gates/notes/required_contracts（调度事实源） |
| `docs/architecture/passb/ownership-matrix.yaml` | 21 legacy 行 + 3 alias 行（plan: 13-execution，integration_owner=delete_barrier=ib1） |
| `docs/architecture/passb/contracts.yaml` | execution.facade / lifecycle / routes / workers / sandbox-config-loader-construction / sandbox-terminal-construction / user-env-resolver-construction / workspace-sandbox-policy（1170–1269 行） |
| `docs/architecture/passb/event-catalog.yaml` | execution 无生产者事件；`execution_observation.go`/`execution_cleanup.go` 仅作为 agentrun.tool.result / usage.observed 的 consumer 出现（79/240 行） |
| `docs/architecture/passb/exception-ledger.yaml` | exc-0087（url_guard.go→policy/ipclass，plan 13-execution，remove_at ib1）为本节点唯一在册例外 |
| `docs/architecture/passb/execution.md`（brief） | session 三文件去方法化裁定与差分口径 |
| `docs/architecture/passb/b0-evidence.md` | b0 收口证据与 B1+ 输入债务 |
| `docs/architecture/moves/execution.yaml` | manifest：21 legacy、3 alias、路由集成点、禁改文件 |
| `docs/architecture/passb/execution-ledger.md` | 调度台账（b0 已 done） |

### 0.2 前置条件（开工前逐条核对）

1. **b0 已收口**：`execution-dag.json` 中 `b0.status=done`、`review_status=approved`、`head_sha=d57a2fa708c3fecf5f0510553ea26db3de51d3c9`（本会话读取 JSON 实证）。DAG notes 中「BLOCKED（2026-09-23）：前置 b0 阻塞」为陈旧标注，解除条件（修复并 done b0）已满足；`b1-execution.status=in_progress`、`base_sha=d57a2fa708…`。
2. **分支基线**：worktree `.worktrees/passb-b1-execution`，分支 `codex/passb-b1-execution`，HEAD == base_sha（本会话 `git rev-parse` 实证）。后续命令中 `PASSB_BASE_SHA=d57a2fa708c3fecf5f0510553ea26db3de51d3c9`。
3. **基线门禁绿**（本计划撰写会话已运行）：
   - `go build ./...` → 退出码 0。
   - `go test ./internal/modules/execution/... -count=1` → `ok …/internal/modules/execution`、`ok …/execution/browserskill`、`ok …/execution/sandbox`，退出码 0。
   - 其余门禁以 EX-1 Step 2 为准在节点内重跑留证。
4. **冻结产物在场**：`docs/architecture/passb/{ownership-matrix,contracts,event-catalog,exception-ledger}.yaml` 均在分支上（b0 合入产物）。

### 0.3 写入所有权（精确）

**可写**（DAG owned_files 展开）：

- 21 条 matrix 行的 legacy 路径本身（含保留为兼容残差的同路径文件）：
  `internal/application/repository/{execution_cleanup,execution_dispatch,execution_observation,execution_target,tenant_sandbox_config}.go`；
  `internal/application/service/{sandbox_terminal_auth,sandbox_terminal_service,sandbox_terminal_ticket,tenant_sandbox_config,tenant_sandbox_resolve,user_env,user_env_resolver}.go`；
  `internal/handler/{execution_registration,execution_target,me_env_var,sandbox_check,sandbox_config,sandbox_skill}.go`；
  `internal/handler/session/{browserskill,sandbox_terminal_bridge,sandbox_terminal_ws}.go`。
- 上述文件随迁的 `_test.go`（framework:29）：
  repository：`execution_cleanup_test.go`、`execution_dispatch_test.go`、`execution_observation_test.go`、`execution_target_test.go`、`tenant_sandbox_config_test.go`；
  service：`sandbox_terminal_auth_test.go`、`sandbox_terminal_service_test.go`、`sandbox_terminal_ticket_test.go`、`tenant_sandbox_config_test.go`、`user_env_resolver_test.go`、`user_env_test.go`（`agent_service_user_env_test.go` 测 agentcatalog 属主 agent_service.go 的消费面——matrix 行 plan=25-agentcatalog-program，**不迁**）；
  handler：`execution_registration_test.go`、`execution_registration_integration_test.go`、`execution_target_test.go`、`me_env_var_test.go`、`sandbox_check_test.go`、`sandbox_config_test.go`、`sandbox_skill_test.go`、`sandbox_skill_env_test.go`（`custom_agent_sandbox_test.go` 主体测 agentcatalog 的 custom_agent 面，**不迁**，若其引用了搬迁符号则依赖残差编译）；
  handler/session：`sandbox_terminal_bridge_test.go`、`sandbox_terminal_idle_test.go`。
- 新产物：`internal/modules/execution/**`（新目录 `repository/`、`service/`、`handler/`）。
- 节点报告/证据/Brief：`docs/plans/passb/reports/b1-execution.md`、`docs/architecture/evidence/passb/b1-execution.md`、`docs/architecture/passb/briefs/b1-execution.md`。

**禁写**（conventions §3）：`internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、`internal/modules/execution/module.go`（门面实现归 IB1；本节点不写）、`go.mod`/`go.sum`、`migrations/**`、四份治理 YAML（contracts/ownership/event/exception——barrier 回写）、`tools/architectureguard/check.go` 的 `importExceptions`（新例外禁止）、一切非本节点属主文件（含 `internal/handler/session/handler.go`、`internal/application/service/user.go`、`session*.go`、`tenant_skill_*.go`、`agent_service.go`、`internal/handler/system.go`、`internal/handler/custom_agent.go`）。

### 0.4 不可全量搬迁的实证裁定（3 文件推迟，§5 升级路径）

`internal/application/repository/{execution_cleanup,execution_dispatch,execution_observation}.go` 均 import `internal/modules/agentruntime/agent/runtime`（本会话 grep 实证；dispatch 使用 `agentruntime.RunKey`（contracts.go:27 定义，仅该子包存在，模块根 `internal/modules/agentruntime` 只有 module.go 骨架）与哨兵错误 `agentruntime.ErrNotFound/ErrConflict`，消费方经 `errors.Is` 判等，改本地副本即破坏错误同一性）。迁入模块树后将触发 architectureguard `forbidden-import`（跨模块仅许经模块根门面，check.go:1203-1213），而 conventions §5 禁止新增例外、agentruntime 文件禁写。**裁定：本节点原样保留三文件（含其测试），在报告与 Integration Brief 登记推迟，建议 IB1 裁定「agentruntime 根门面 re-export RunKey/哨兵」或随 B3 agentruntime 搬迁协同收口。** 三文件 `integration_owner=ib1`，推迟不改变 IB1 收口语义。事件目录中两处 consumer 行（agentrun.tool.result/usage.observed）同步在 Brief 标注路径未变。

### 0.5 兼容残差（shim）规则

1. 残差只允许落在**本节点属主的旧路径**上（0.3 清单内），保持旧包内标识符（导出符号 + 被同包他 owner 消费的非导出符号）继续可编译；内容限：类型别名（`type X = exec…X`）、无逻辑转发函数/方法（参数 1:1 委托或纯捕获式闭包适配）、哨兵 var 别名。
2. 每个残差文件头注释：`// Pass B b1-execution 兼容残差（no-logic forwarding）——IB1 按 Integration Brief 删除。`
3. 依赖注入方向恒为「旧包 → 新包」；新包（`internal/modules/execution/{repository,service,handler}`）**禁止 import** `internal/application/service`、`internal/handler`、`internal/handler/session`（防环；`internal/application/repository` 仅允许 service/handler 新包引用其中仍留驻的 agentcatalog 接口 `TenantSkillRepository` 与哨兵错误）。`go build ./...` 即环检测。
4. 残差全量登记进 Integration Brief（IB1 删除清单）。

### 0.6 跨 owner 断链处理总表（定义位置均本会话 grep 实证）

| # | 方向 | 符号（定义处） | 调用点 | 本节点处理 |
|---|---|---|---|---|
| 1 | conversation/agentcatalog → execution | `resolveTenantSandboxForConfig`（tenant_sandbox_resolve.go:70） | session.go:892,1166；session_sandbox_pin.go:162,170,193；pinned_session_sandbox.go:57；artifact_collector.go:182；tenant_skill_install.go:1461（共 8 处非本节点属主） | 新包导出 `ResolveTenantSandboxForConfig`；旧包残差同名非导出函数 1:1 委托；IB1 改调用点后删残差 |
| 2 | conversation → execution | `browserSkillScope`（browserskill.go:13） | handler.go:595,703,751 | 新包导出 `BrowserSkillScope`；session 包残差同名函数委托 |
| 3 | execution → identity | `getJwtSecret`（user.go:81）、`tenantIDFromClaims`（user.go:1404） | sandbox_terminal_ticket.go:52,66,82 | 新包票据函数增 `secret func() string` 注入；旧包残差以 `getJwtSecret` 装配（同包合法引用，不改 user.go） |
| 4 | execution → agentcatalog | `matchSnapshotByName`（tenant_skill_reaper.go:672）、`skillSnapshotNamePrefix`（tenant_skill_install.go:1633）、`snapshotsNotFromOtherConfig`（tenant_skill_install.go:1703）、`validateUserEnvName`（tenant_skill_env_declare.go:158） | tenant_sandbox_config.go(svc):1134,1135,1137；user_env.go:275,381 | 新包以非导出 func 字段/参数注入（构造器装配）；旧包构造器残差捕获上述同包函数注入；不改 agentcatalog 文件 |
| 5 | execution → conversation | `resolveSandboxForExecution`（session_sandbox_pin.go:146） | sandbox_terminal_service.go:169 | 新包 `SandboxTerminalService` 增非导出 func 字段；旧包构造器残差以捕获式闭包 `func(ctx, tenantID, sessionID, agentConfigID) (sandbox.Manager, string, error)` 包装真源注入 |
| 6 | execution(conversation 结构方法) | `*Handler`（handler.go:24，67 字段）与 `*SystemHandler`（system.go:40） | 路由绑定（routes_chat.go:115,116,129,272,283-285,293,294；routes_auth_tenant.go:259） | 去方法化：新包提供显式依赖的包级函数/独立类型；旧路径残差方法仅用既有字段转发（§0.5） |
| 7 | agentruntime 等 → execution | 13 对 import 例外（exc-0019..0047、exc-0095、exc-0105，owner 为 32/33/34/40 计划，remove_at ib3/ib4） | agentruntime/workbench 模块文件 | 不动：消费方走 IB1 冻结门面后由属主计划删除（DAG notes 口径） |

### 0.7 公共可观察行为与兼容要求（迁移不变式）

1. **路由面**：633 条路由总数与逐条路径/方法/守卫不变（`make check-backend-architecture` 输出 `total=633`）。execution 相关 8 项集成点（execution.yaml:133-141）挂载不变：RegisterMyEnvVarRoutes/RegisterSandboxTerminalRoutes/RegisterLocalBrowserRoutes/RegisterMyBrowserRoutes/RegisterSandboxConfigRoutes/RegisterExecutionRegistrationRoutes/execution-targets×4+workspaces×1/session 子路由×3。
2. **WebSocket/ticket 语义**（brief 义务 2，session stream 邻接高风险面）：`/api/v1/sessions/:id/sandbox/terminal` 握手协议、二进制/文本帧、ticket 类型 `sandbox_terminal`、TTL 默认 2 分钟（sandbox_terminal_ticket.go:18）、`ValidateToken` 拒绝 ticket、PTY 周期性复鉴权与错误帧码（terminalErrNotBound/Paused/Unsupported/Internal）不变。
3. **错误同一性**：`ErrTerminalAuthDenied`、`ErrTerminalUnsupported`（= `sandbox.ErrTerminalUnsupported`）、`ErrSandboxesStillLive`、`ErrSandboxConfigCordoned`、`ErrExecutionTargetNotFound`、`ErrDispatch*` 等哨兵经 `errors.Is` 判等语义不变（残差用 var 别名 `= 新包哨兵` 保同一性）。
4. **装配**：container.go 的 Provide 集与参数类型经别名后编译不变；dig 图形状不变（IB1 才切换）。
5. **worker/hook/migration**：execution 0 worker、0 hook（execution.yaml:142-145）；migrations 不触碰（537 不变）。
6. **配置解析语义**：`resolveTenantSandboxForConfig` 的 kill-switch 优先、空/全局默认 configID → `sandbox.NewDisabledManager()`、命名 config 不静默回退三段逻辑（tenant_sandbox_resolve.go:78-112）逐字保留。

### 0.8 目标包结构与关键 Go 签名（以当前代码为准，搬迁后导出名加粗处为更名）

```text
internal/modules/execution/
  repository/            # 新建包 repository（package repository）
    tenant_sandbox_config.go   # 自 internal/application/repository/ 整迁
    execution_target.go        # 自 internal/application/repository/ 整迁
  service/               # 新建包 service（package service）
    tenant_sandbox_resolve.go  # 迁入 + 导出
    tenant_sandbox_config.go   # 迁入 + 4 个 agentcatalog 函数注入
    sandbox_terminal_auth.go   # 迁入（签名不变）
    sandbox_terminal_ticket.go # 迁入 + secret 注入
    sandbox_terminal_service.go# 迁入 + pinner/resolveForExecution 注入
    user_env.go                # 迁入 + validateUserEnvName 注入
    user_env_resolver.go       # 迁入 + 导出类型
  handler/               # 新建包 handler（package handler）
    me_env_var.go / sandbox_config.go / sandbox_skill.go
    execution_target.go / execution_registration.go
    sandbox_check.go           # 去方法化核心
    browserskill.go / sandbox_terminal_ws.go / sandbox_terminal_bridge.go
```

新包关键签名（**=相对旧代码有变更**，其余原样）：

```go
// service 包
type WorkspaceSandboxPolicy interface {                                   // 原样（tenant_sandbox_resolve.go:60）
	WorkspaceScriptsDisabled(ctx context.Context, tenantID uint64) (bool, error)
}
func NewTenantSandboxConfigLoader(repo execrepo.TenantSandboxConfigRepository) sandbox.TenantSandboxConfigLoader // **参数类型改指本模块 repository 包
**func ResolveTenantSandboxForConfig(ctx context.Context, resolver sandbox.TenantSandboxResolver, _ sandbox.Manager, tenantID uint64, configID string, policy WorkspaceSandboxPolicy) (sandbox.Manager, error)
**func IssueSandboxTerminalTicket(secret func() string, userID string, tenantID uint64, sessionID, tokenID string, ttl time.Duration) (string, error)
**func ParseSandboxTerminalTicket(secret func() string, raw string) (*SandboxTerminalTicketClaims, error)
**type SessionPinReader interface { Read(ctx context.Context, sessionID string) (string, error) }   // 消费方窄口（SessionSandboxPinner.Read 结构满足）
**type SandboxTerminalDeps struct {
	Pinner    SessionPinReader
	Resolver  sandbox.TenantSandboxResolver
	Fallback  sandbox.Manager
	Policy    WorkspaceSandboxPolicy
	ResolveForExecution func(ctx context.Context, tenantID uint64, sessionID, agentConfigID string) (sandbox.Manager, string, error)
}
**func NewSandboxTerminalService(deps SandboxTerminalDeps) *SandboxTerminalService
type SkillSnapshotMatcher struct {                                        // **agentcatalog 4 符号注入
	MatchByName              func(listed []sandbox.RemoteSnapshotRef, plannedName string) string
	NamePrefix               func(tenantID uint64, configID string) string
	NotFromOtherConfig       func(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef
	ValidateUserEnvName      func(name string) error
}
// TenantSandboxConfigService 构造器新增非导出字段持有 SkillSnapshotMatcher，
// 既有参数（repo execrepo.TenantSandboxConfigRepository / agents SandboxConfigAgentRepo /
// globalCfg *sandbox.Config / skills repository.TenantSkillRepository / files sandboxConfigBundleResolver）语义不变
**func NewUserEnvResolver(rows []*types.TenantSkillEntity, userEnvs UserEnvReader, tenantID uint64, configID string) *UserEnvResolver
**type UserEnvReader interface { … }   // = 现 userEnvReader（user_env_resolver.go:12）方法集
**type UserEnvChecker struct { ValidateName func(name string) error }   // user_env.go 注入口

// handler 包
**type SandboxConfigService interface { … }   // = 现 sandbox_config.go:18 非导出接口方法集（system.go:66 字段经旧包别名消费）
**func CheckSandboxConfig(c *gin.Context, svc SandboxConfigService)   // 去方法化核心（现 system.go *SystemHandler 方法体）
**func BrowserSkillScope(ctx context.Context) browserskill.Scope      // = 现 browserskill.go:13
**type BrowserSkillDeps struct {
	Skill    *browserskill.Manager
	Sessions interfaces.SessionService
}
**func BrowserSkillConnection(c *gin.Context, d BrowserSkillDeps)      // 及 Extension/Account/Authorize/Internal/Download 同形
**type TerminalOpener interface {                                          // ws 面消费窄口（*SandboxTerminalService 方法集结构满足）
	OpenSessionTerminal(ctx context.Context, sessionID string, opts sandbox.RemoteTerminalOptions) (*execservice.SessionTerminal, error)
	EnsureSessionTerminal(ctx context.Context, sessionID string, sandboxConfigID string, opts sandbox.RemoteTerminalOptions) (*execservice.SessionTerminal, error)
}
**type TerminalDeps struct {
	Users    interfaces.UserService
	Members  interfaces.TenantMemberService
	Sessions interfaces.SessionService
	Cfg      *config.Config
	Terminal TerminalOpener
	Secret   func() string
}
**func SandboxTerminalWS(c *gin.Context, d TerminalDeps)
**func IssueSandboxTerminalTicketHTTP(c *gin.Context, d TerminalDeps)  // 现 *Handler.IssueSandboxTerminalTicket 方法体
```

旧包残差清单（IB1 删除；全部位于本节点属主旧路径）：

| 旧路径 | 残差内容 |
|---|---|
| repository/tenant_sandbox_config.go | `type TenantSandboxConfigRepository = execrepo.TenantSandboxConfigRepository`；`func NewTenantSandboxConfigRepository(db *gorm.DB) TenantSandboxConfigRepository`；`var ErrSandboxConfigCordoned = execrepo.ErrSandboxConfigCordoned` |
| repository/execution_target.go | `type ExecutionTargetStore = execrepo.ExecutionTargetStore`；`NewExecutionTargetStore`/`NewExecutionTargetIdentityProvider`/`NewPersonalTargetProvisioner` 转发；`var ErrExecutionTargetNotFound = execrepo.ErrExecutionTargetNotFound` |
| service/tenant_sandbox_resolve.go | `func resolveTenantSandboxForConfig(...)`（非导出，1:1 委托 ResolveTenantSandboxForConfig）；`type WorkspaceSandboxPolicy = execservice.WorkspaceSandboxPolicy`；`func NewTenantSandboxConfigLoader(repo repository.TenantSandboxConfigRepository) sandbox.TenantSandboxConfigLoader` |
| service/tenant_sandbox_config.go | `type TenantSandboxConfigService = execservice.TenantSandboxConfigService`；`func NewTenantSandboxConfigService(...)`（装配 SkillSnapshotMatcher：捕获同包 `matchSnapshotByName/skillSnapshotNamePrefix/snapshotsNotFromOtherConfig`）；`type SandboxInventory = execservice.SandboxInventory` |
| service/sandbox_terminal_service.go | `type SandboxTerminalService = execservice.SandboxTerminalService`；`type SessionTerminal = execservice.SessionTerminal`；`var ErrTerminalUnsupported = execservice.ErrTerminalUnsupported`；`func NewSandboxTerminalService(pinner *SessionSandboxPinner, resolver sandbox.TenantSandboxResolver, fallback sandbox.Manager, policy WorkspaceSandboxPolicy) *SandboxTerminalService`（nil pinner→nil 接口显式转换后组装 Deps 并捕获式闭包注入 resolveSandboxForExecution） |
| service/sandbox_terminal_ticket.go | `func IssueSandboxTerminalTicket(userID string, tenantID uint64, sessionID, tokenID string, ttl time.Duration) (string, error)`（以 `getJwtSecret` 装配）；`func ParseSandboxTerminalTicket(raw string) (*SandboxTerminalTicketClaims, error)`；`type SandboxTerminalTicketClaims = execservice.SandboxTerminalTicketClaims`；`var DefaultSandboxTerminalTicketTTL = execservice.DefaultSandboxTerminalTicketTTL` |
| service/user_env.go | `type UserEnvService = execservice.UserEnvService`；`func NewUserEnvService(skills repository.TenantSkillRepository, configs repository.TenantSandboxConfigRepository) *UserEnvService`（装配 UserEnvChecker{ValidateName: validateUserEnvName}）；`type EnvVarView/ConfigEnvGroup/SkillEnvGroup = …` |
| service/user_env_resolver.go | `func NewUserEnvResolver(rows []*types.TenantSkillEntity, userEnvs userEnvReader, tenantID uint64, configID string) *execservice.UserEnvResolver`（保留本包既有非导出 `userEnvReader` 接口原定义或改引 execservice.UserEnvReader——二选一以编译最小改动为准，报告注明） |
| handler/execution_registration.go | `type ExecutionRegistrationHandler = exechandler.ExecutionRegistrationHandler`；`func NewExecutionRegistrationHandler(service *execution.RegistrationService) *ExecutionRegistrationHandler` |
| handler/execution_target.go | `type ExecutionTargetHandler = exechandler.ExecutionTargetHandler`；`func NewExecutionTargetHandler(store repository.ExecutionTargetStore, verifier execution.TargetIdentityProvider) *ExecutionTargetHandler` |
| handler/me_env_var.go | `type MeEnvVarHandler = exechandler.MeEnvVarHandler`；`func NewMeEnvVarHandler(s *service.UserEnvService) *MeEnvVarHandler` |
| handler/sandbox_config.go | `type SandboxConfigHandler = exechandler.SandboxConfigHandler`；`func NewSandboxConfigHandler(...)`；`type sandboxConfigService = exechandler.SandboxConfigService`（system.go:66 字段依赖） |
| handler/sandbox_skill.go | `type SandboxSkillHandler = exechandler.SandboxSkillHandler`；`func NewSandboxSkillHandler(...)`（参数签名以现 :91 为准 1:1 转发） |
| handler/sandbox_check.go | `func (h *SystemHandler) CheckSandboxConfig(c *gin.Context) { exechandler.CheckSandboxConfig(c, h.sandboxConfigSvc) }`（唯一残差方法；核心整体迁入 exechandler） |
| handler/session/browserskill.go | 6 个 `func (h *Handler) BrowserSkill*(c *gin.Context)` 残差方法（以 BrowserSkillDeps{Skill: h.browserSkill, Sessions: h.sessionService} 转发）；`func browserSkillScope(ctx) browserskill.Scope` 委托 |
| handler/session/sandbox_terminal_ws.go | `func (h *Handler) SandboxTerminalWS(c *gin.Context)`、`func (h *Handler) IssueSandboxTerminalTicket(c *gin.Context)` 残差方法（以 TerminalDeps{Users: h.userService, Members: h.memberService, Sessions: h.sessionService, Cfg: h.config, Terminal: h.terminalService, Secret: getJwtSecret 同包不可得——经 service 残差包内函数 `SandboxTerminalSecret()` 暴露，该辅助函数体 `return getJwtSecret()`，登记 Brief） |
| handler/session/sandbox_terminal_bridge.go | 无残差需求（整迁；若编译发现 session 包他文件引用其符号，按 §0.5 规则补非导出转发并登记） |

---

## 1. 任务分解（EX-1..EX-8；一任务一 commit）

> 每个 Step 的「预期」均为可机检结果。所有 `go`/`make` 命令在 worktree 根执行。提交规范：conventions §4（`refactor(execution): …`/`test(execution): …`/`docs(passb): …`）。

### Task EX-1: 基线证据与解析 choke-point 特征化测试（RED→GREEN）

**Files:** 新建 `internal/application/service/tenant_sandbox_resolve_test.go`（特征化，随 EX-2 一并迁移）；`docs/architecture/evidence/passb/b1-execution.md`（基线章节）。

特征化测试目前缺失（该文件无既有 `_test.go`），按 conventions §1.4 先锚定旧行为。用例覆盖 §0.7-6 三段逻辑 + 错误路径：

- `TestResolveTenantSandboxForConfig_WorkspaceKillSwitch`：policy 返回 disabled → `*sandbox.DisabledManager`，不触 resolver（nil resolver 亦不 panic）。
- `TestResolveTenantSandboxForConfig_EmptyAndGlobalDefaultDisabled`：`configID==""` 与 `== types.SandboxConfigIDGlobalDefault` → DisabledManager。
- `TestResolveTenantSandboxForConfig_MissingTenantOrResolver`：tenantID==0 / resolver==nil → 非 nil error（含 configID 字样）。
- `TestResolveTenantSandboxForConfig_NoSilentFallback`：resolver.Resolve 出错 → 原样上抛（不得退回 fallback manager）。

- [ ] Step 1 写测试（package service，指向现非导出 `resolveTenantSandboxForConfig`）→ `go test ./internal/application/service -run TestResolveTenantSandboxForConfig -count=1`。预期：RED 阶段（若用例本身写对而实现已在）应直接 GREEN——记录实际输出；任何 FAIL 即修测试而非实现。
- [ ] Step 2 基线门禁留证（写入 evidence §基线）：`PASSB_BASE_SHA=d57a2fa708c3fecf5f0510553ea26db3de51d3c9`；`go build ./...`（exit 0）；`go test ./internal/application/service ./internal/application/repository ./internal/handler ./internal/handler/session ./internal/modules/execution/... -count=1`（记录各包 ok/FAIL 清单——这是后续差分的旧实现基线）；`make check-backend-architecture`（`total=633 | redis=23 lite=23 | hooks=58 | modules=16`，0 violations）；`make verify-module-moves`（`modulemove: OK (16 manifests verified)`）。
- [ ] Step 3 commit `test(execution): characterize tenant sandbox resolve choke point`。

**回滚边界：** `git revert <commit>`；测试为纯新增，无生产影响。

### Task EX-2: Sandbox 配置与解析域搬迁（repo + svc 3 文件 + 残差）

**Files:** `git mv` repository/tenant_sandbox_config.go、repository/execution_target.go → `internal/modules/execution/repository/`；service/tenant_sandbox_config.go、service/tenant_sandbox_resolve.go（含 EX-1 测试）→ `internal/modules/execution/service/`；4 个旧路径改写为 §0.8 残差；随迁测试 repository/tenant_sandbox_config_test.go、repository/execution_target_test.go、service/tenant_sandbox_config_test.go。

- [ ] Step 1 `git mv` 四文件 + 三个 `_test.go`；新包 package 声明改 `repository`/`service`；修 import（moved 代码内对旧 repository 包的引用改指 `internal/modules/execution/repository`；`TenantSkillRepository` 仍指旧包 `internal/application/repository`）。
- [ ] Step 2 导出与注入：`ResolveTenantSandboxForConfig` 更名导出；`tenant_sandbox_config.go`(svc) 的 3 个 agentcatalog 调用点（:1134,1135,1137）改走 `SkillSnapshotMatcher` 字段；构造器按 §0.8 扩参（新增注入参数放最后）；旧路径写残差（构造器残差捕获同包 agentcatalog 真源函数装配 Matcher）。
- [ ] Step 3 `go build ./...` exit 0（环检测即此）；`go vet ./internal/modules/execution/...`。
- [ ] Step 4 `go test ./internal/modules/execution/... ./internal/application/service ./internal/application/repository ./internal/handler -count=1`——与 EX-1 Step 2 基线逐包比对：不得出现新 FAIL（既有 FAIL 台账内除外，基线时记录）。
- [ ] Step 5 `rg -n "resolveTenantSandboxForConfig" internal/ | grep -v _test`——预期仅剩：新包导出名（更名后无命中）、旧包残差 1 处定义、8 处他 owner 调用点（未改，IB1 处理）。
- [ ] Step 6 commit `refactor(execution): move sandbox config/resolve into module with narrow ports`。

**回滚边界：** `git revert <commit>`（残差与移动同一 commit，revert 即整体回到旧实现；不触碰共享装配，无需 M4 回退）。

### Task EX-3: 终端票据/鉴权/服务域搬迁（3 文件 + 注入 + 残差）

**Files:** `git mv` service/{sandbox_terminal_auth,sandbox_terminal_ticket,sandbox_terminal_service}.go + 对应 3 个 `_test.go` → `internal/modules/execution/service/`；3 旧路径写残差。

- [ ] Step 1 移动 + package 修复。ticket 文件签名改 §0.8（`secret func() string` 首参）；auth 文件原样迁（其 `apprepo.ErrTokenNotFound/ErrUserNotFound` 引用保留旧 repository 包——合法，方向为 新包→旧包）。
- [ ] Step 2 `SandboxTerminalService` 重构为 `SandboxTerminalDeps` 构造：`pinner` 转窄口 `SessionPinReader`、`ResolveForExecution` 闭包注入（moved 文件内 :169 调用点改走字段）；`ErrTerminalUnsupported` var 保留原语义（`var ErrTerminalUnsupported = sandbox.ErrTerminalUnsupported`）。
- [ ] Step 3 旧路径残差（§0.8 表）：重点 `NewSandboxTerminalService` 残差内 `var p execservice.SessionPinReader; if pinner != nil { p = pinner }` 防typed-nil 语义漂移；闭包 `func(ctx, tenantID, sessionID, agentConfigID){ return resolveSandboxForExecution(ctx, resolver, fallback, pinner, tenantID, sessionID, agentConfigID, policy) }`。
- [ ] Step 4 `go build ./...`；`go test ./internal/modules/execution/... ./internal/application/service ./internal/handler/session -count=1` 逐包不劣于基线。
- [ ] Step 5 高风险差分（conventions §6，session stream 邻接面）：ticket 双向用例（签发→解析往返、过期拒绝、type 混用拒绝、空 claims 拒绝）在旧基线（EX-1 Step 2 前置记录，如无则补跑 `git stash` 不可用——以 EX-1 基线时同包测试输出为准）与新位置各跑一次，输出贴 evidence 差分章节。命令：`go test ./internal/modules/execution/service -run 'TestSandboxTerminal|TestParseSandboxTerminal|TestIssueSandboxTerminal' -count=1 -v`（用例名以随迁测试实际为准，报告如实摘录）。
- [ ] Step 6 commit `refactor(execution): move terminal auth/ticket/service with secret and pinner ports`。

### Task EX-4: 用户环境变量域搬迁（user_env 2 文件 + me_env_var handler + 残差）

**Files:** `git mv` service/{user_env,user_env_resolver}.go + `user_env_test.go`、`user_env_resolver_test.go`；handler/me_env_var.go + `me_env_var_test.go` → 对应新包；4 旧路径残差。

- [ ] Step 1 移动 + package 修复；`UserEnvReader`/`UserEnvResolver` 导出（§0.8）；user_env.go 的 `validateUserEnvName` 2 调用点（:275,381）改走 `UserEnvChecker` 注入；me_env_var.go 的 `*service.UserEnvService` 具体参数改为接口（现 :18 `meEnvVarService` 方法集）。
- [ ] Step 2 旧路径残差（§0.8 表；`NewUserEnvService` 残差装配 `UserEnvChecker{ValidateName: validateUserEnvName}`——同包引用 agentcatalog 真源，不改其文件）。
- [ ] Step 3 `go build ./...`；`go test ./internal/modules/execution/... ./internal/application/service ./internal/handler -count=1`（`agent_service_user_env_test.go` 留旧包，验证经残差编译且行为不变）。
- [ ] Step 4 `rg -n "NewUserEnvResolver|NewUserEnvService" internal/ | grep -v _test | grep -v modules/execution`——预期仅旧包残差定义 + agent_service.go:730,743 调用点（IB1 处理）。
- [ ] Step 5 commit `refactor(execution): move user env services and me env var handler`。

### Task EX-5: Execution Target/Registration HTTP 域搬迁（repo 1 + handler 2 + 残差）

**Files:** `git mv` handler/{execution_target,execution_registration}.go + `execution_target_test.go`、`execution_registration_test.go`、`execution_registration_integration_test.go` → `internal/modules/execution/handler/`；2 旧路径残差。（repository/execution_target.go 已随 EX-2 迁移，本任务只迁 handler 面。）

- [ ] Step 1 handler 两文件移动 + package 修复；对 `repository.ExecutionTargetStore` 的引用改指本模块 repository 包。
- [ ] Step 2 旧路径残差（§0.8 表）。
- [ ] Step 3 `go build ./...`；`go test ./internal/modules/execution/... ./internal/handler -count=1`。
- [ ] Step 4 commit `refactor(execution): move execution target and registration handlers`。

### Task EX-6: Sandbox 检查/配置/技能 HTTP 域搬迁（去方法化 SystemHandler 面 + 残差）

**Files:** `git mv` handler/{sandbox_check,sandbox_config,sandbox_skill}.go + 4 个 `_test.go`；3 旧路径残差（sandbox_check 仅残差方法）。

- [ ] Step 1 sandbox_check.go 核心整体迁入（`CheckSandboxConfig`/`runDeepSandboxCheck`/`probeSandboxEgress`/全部 helper 与 `SandboxCheck*` 类型），去方法化为 `func CheckSandboxConfig(c *gin.Context, svc SandboxConfigService)` + 包内函数（现 :262/:360 方法体仅用 `h.sandboxConfigSvc`，本会话 grep 实证）。旧路径残差方法一行转发。
- [ ] Step 2 sandbox_config.go/sandbox_skill.go 移动 + 接口导出（`SandboxConfigService`（现 sandbox_config.go:18）、`SandboxSkillService`（现 sandbox_skill.go:39）、`MeEnvVarService` 已在 EX-4 处理）；旧包 `type sandboxConfigService = exechandler.SandboxConfigService` 残差保 `internal/handler/system.go:66` 编译；`NewSandboxSkillHandler` 残差参数以现签名（sandbox_skill.go:91：`svc sandboxSkillService, streams interfaces.StreamManager`）1:1 转发，参数类型用导出接口（结构满足不变）。
- [ ] Step 3 `go build ./...`；`go test ./internal/modules/execution/... ./internal/handler -count=1`；`rg -n "handler.CheckSandboxConfig|CheckSandboxConfig" internal/router` 确认路由绑定未动（应无 diff）。
- [ ] Step 4 commit `refactor(execution): de-methodize and move sandbox check/config/skill handlers`。

### Task EX-7: 会话内 Browser/Terminal 入口去方法化（session 3 文件 + 残差方法）

**Files:** `git mv` handler/session/{browserskill,sandbox_terminal_bridge,sandbox_terminal_ws}.go + `sandbox_terminal_bridge_test.go`、`sandbox_terminal_idle_test.go` → `internal/modules/execution/handler/`；browserskill.go、sandbox_terminal_ws.go 旧路径残差方法（brief 义务 1：IB1 收口，禁常驻）。

- [ ] Step 1 bridge 整迁（无 Handler 方法依赖，本会话实证其仅引用 terminal* 常量与自身状态）；ws/browserskill 按 §0.8 去方法化（`BrowserSkillDeps`/`TerminalDeps` 显式依赖；ws 内 `service.AssertAccessTokenStillActive`/`service.IssueSandboxTerminalTicket` 调用改经 Deps/新包直引，`getJwtSecret` 经 service 残差包内 `SandboxTerminalSecret()` 辅助——该辅助属 service 残差文件内容并登记 Brief）。
- [ ] Step 2 旧路径残差方法 6+2 个（§0.8 表；全部仅用 *Handler 既有字段 `browserSkill/terminalService/sessionService/userService/memberService/config`，禁止改 handler.go）。
- [ ] Step 3 `go build ./...`；`go test ./internal/modules/execution/... ./internal/handler/session -count=1`（包内他 owner 测试经残差编译）。
- [ ] Step 4 WS 差分（conventions §6）：`go test ./internal/modules/execution/handler -run 'TestTerminal|TestBridge|TestBrowser' -count=1 -v`（随迁测试）与基线（EX-1 Step 2 的 ./internal/handler/session 输出）逐用例比对，写入 evidence。
- [ ] Step 5 `git diff --stat $PASSB_BASE_SHA...HEAD -- internal/router internal/container internal/handler/session/handler.go`——预期**空输出**（零触碰共享面）。
- [ ] Step 6 commit `refactor(execution): de-methodize browser skill and terminal session handlers`。

### Task EX-8: 差分汇总、alias/例外核销登记、Integration Brief 与节点收口

**Files:** `docs/architecture/evidence/passb/b1-execution.md`（差分+门禁全量）、`docs/plans/passb/reports/b1-execution.md`、`docs/architecture/passb/briefs/b1-execution.md`（新建）。

- [ ] Step 1 证据文件按 conventions §1.2 模板生成命令包并逐条执行摘录：`git diff --stat "$PASSB_BASE_SHA"...HEAD`；`go build ./...`；`go test -count=1 ./internal/modules/execution/...`；`git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort` 与 §0.3 可写清单求差集（差集非空即失败，如实上报）。
- [ ] Step 2 全部 DAG 节点门禁 + 通用门禁逐条执行并记录退出码：`go build ./...`、`go test -count=1 ./internal/modules/execution/...`、`make check-backend-architecture`（633/23+23/58/16 不变）、`make verify-module-moves`。
- [ ] Step 3 alias 核销证据：`git log --oneline --diff-filter=D -- internal/browserskill internal/execution internal/sandbox`（Pass A 提交 `0bb9b354f` 已删，本会话实证）+ `rg -l "Tencent/WeKnora/internal/(browserskill|execution|sandbox)\"" --include="*.go"` 零命中 → 三条 alias 义务事实已满足，写入 evidence（matrix 行状态回写归 IB1）。
- [ ] Step 4 exc-0087 登记提案（不删行、不改 guard）：Brief 中记录「url_guard.go→policy/ipclass 的解除需 policy 侧公开或 coordinator 裁定（platform 化/根门面 re-export），本节点无写权限，remove_at ib1 期限留给 IB1 执行」；§0.4 的 3 文件推迟同段落登记（含 agentruntime.RunKey 实证与本会话结论）。
- [ ] Step 5 Integration Brief 内容（IB1 切换输入）：(a) §0.8 残差全量删除清单；(b) 8+3 处他 owner 调用点改指新导出名清单；(c) contracts.yaml 需 barrier 回写的 execution 4 行符号新路径/新签名（sandbox-config-loader-construction / sandbox-terminal-construction / user-env-resolver-construction / workspace-sandbox-policy）；(d) module.go 门面 NewModule/RegisterRoutes/RegisterWorkers/Start/Stop 实装所需 provider 清单（8 路由注册函数及其 handler 构造依赖，全部来自本节点新包导出面）；(e) 推迟项与 exc-0087 提案。
- [ ] Step 6 报告 `docs/plans/passb/reports/b1-execution.md`：执行过的每条命令原文+退出码+关键输出、变更文件 vs owned_files 逐条核对、差分指针、未完成项（3 文件推迟、exc-0087、module.go 不实装）如实列出。
- [ ] Step 7 commit `docs(passb): b1-execution evidence, report and integration brief`。

---

## 2. 集成与回滚边界

- **提交隔离（Spec §13）：** 每任务一个 commit；移动+残差同 commit（保证任一 commit 点可独立构建）；不混入行为修改。差分证据先行于任何"必须删除的旧实现"（本节点不删除任何他 owner 旧实现，仅自身旧路径内容替换为残差）。
- **回滚：** 任务级 `git revert <commit>`；节点级 `git reset --hard $PASSB_BASE_SHA`（分支未合入 integration 前）。残差与新实现同 commit revert，无 M4 装配切换（router/container 零触碰），回滚无需数据修复（无 schema 变更）。
- **IB1 输入：** 仅经 Brief；本节点不合并不回写治理 YAML/DAG 状态（coordinator 专职）。
- **停止条件（Spec §16）：** 若任一门禁无法在 §0.3 写权限内达成绿色（如出现未预期的第 8 类跨 owner 断链），按 conventions §5 在报告登记 blocked + 建议裁定，不得扩例外/塞 common/复制实现。

## 3. 必须删除的 legacy/alias/例外（本节点口径）

| 项 | 状态 | 依据 |
|---|---|---|
| 3 条 alias（internal/browserskill、internal/execution、internal/sandbox） | **已在 Pass A 删除**（0bb9b354f），本节点零 importers 复核后登记证据 | git log 实证 + rg 零命中 |
| 18 个旧路径 legacy 文件 | 本节点替换为 no-logic 残差（IB1 删除） | §0.5/§0.8 |
| exc-0087 | 无法节点内解除（policy 属主），登记提案留 IB1 | §0.4/EX-8 Step 4 |
| 3 个 repo legacy 文件（cleanup/dispatch/observation） | 推迟（agentruntime 子包依赖），登记 Brief | §0.4 |
| 残差方法（SystemHandler.CheckSandboxConfig、Handler.BrowserSkill*×6、Handler.SandboxTerminalWS/IssueSandboxTerminalTicket、browserSkillScope、resolveTenantSandboxForConfig 非导出残差） | IB1 按 Brief 收口，禁常驻 | brief execution.md 义务 1 |

## 4. 节点级独立验收标准

1. DAG 四条 gates 全绿：`go build ./...`、`go test -count=1 ./internal/modules/execution/...`、`make check-backend-architecture`（633/23+23/58/16 零漂移）、`make verify-module-moves`。
2. `git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort` ⊆ §0.3 可写清单，差集为空。
3. `git diff "$PASSB_BASE_SHA"...HEAD -- internal/router internal/container internal/bootstrap go.mod go.sum migrations internal/modules/execution/module.go internal/handler/session/handler.go internal/application/service/user.go internal/handler/system.go` 输出为空。
4. 18 文件存在于新包且旧路径仅余 §0.8 清单残差；`rg -n "Pass B b1-execution 兼容残差" internal/ | wc -l` 与 Brief 清单一一对应。
5. EX-1 Step 2 基线与 EX-8 Step 2 终态的受影响包（service/repository/handler/handler/session/modules/execution）测试结果逐包不劣化（新 FAIL=0）。
6. 差分证据（resolve choke-point、ticket 往返、WS/bridge 用例）在 evidence 文件含双跑命令与输出摘录。
7. 报告/Brief/evidence 三产物存在且未完成项如实（3 推迟 + exc-0087 + module.go 归 IB1）。
8. 无新增 import 例外、无 wildcard、无 common/utils 兜底、无他模块实现复制（`git diff` 人工复核 + review 节点把关）。

## 5. 计划自检记录（撰写会话）

- **Spec 覆盖：** §5.8 职责逐条映射到 21 文件归属；§4.4/§13/§14.3/§16 对应 §0.5/§2/EX-3 Step 5/§2 停止条件；§11 B1 并行与 IB1 串行由"零触碰共享装配"保证。
- **无占位符：** 所有签名、路径、行号取自本会话实读代码/治理 YAML；无一 TBD。
- **类型一致：** 全部签名经 `go build` 基线（exit 0）所在代码树核对；§0.8 更名处显式加粗。
- **跨任务接口一致：** 残差→新包导出名、注入端口（SessionPinReader/SkillSnapshotMatcher/UserEnvChecker/UserEnvReader/ResolveForExecution/secret）在 EX-2..EX-7 间复用同一命名；IB1 消费面（Brief 清单 (a)-(e)）与 conventions §1.1/§3 对齐。
- **已知开放点（非占位符，均为有据裁定）：** §0.4 三文件推迟；exc-0087 提案；contracts.yaml 4 行 barrier 回写；me_env_var 残差二选一以编译最小改动定夺。
