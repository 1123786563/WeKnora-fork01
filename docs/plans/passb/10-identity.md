# Pass B B1-ID：Identity 边界实施计划（10-identity）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 执行公约：`.superpowers/sdd/passb/conventions.md`（本计划全部任务受其 §1–§9 约束）。

**Goal:** 把 identity 模块的 27 个 legacy 文件从三个横向宿主包（`internal/application/repository`、`internal/application/service`、`internal/handler`）迁入 `internal/modules/identity/{repository,service,handler}`，导出节点裁定要求的窄端口，留下可删除的薄兼容层，交付 IB1 集成所需的 Integration Brief 与差分证据；系统全程可构建、可测试、行为不变。

**Architecture:** Pass B 计划集内的 B1 并行子计划（DAG 节点 `b1-identity`，depends_on `b0`，execution_mode parallel）。本节点不实现 module.go 门面接线（`internal/modules/identity/module.go` 门面注释与装配归 IB1，conventions §3）；本节点产出模块包、导出端口、宿主兼容层与 Integration Brief。

**Tech Stack:** Go 1.26、Gin、GORM、dig、Testify、SQLite 内存测试、Git worktree、`tools/modulemove` / `tools/architectureguard` / `tools/passbguard`。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.4 两遍模型、§5.1 Identity 职责、§12 交付包、§13 提交隔离、§14 差分门禁、§17.2 完成标准）。

## 0. 事实源指针

| 事实源 | 路径 | 本计划消费点 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | §5.1/§5.18 Identity 所有权；§4.4 薄别名过渡规则 |
| 框架计划 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | :88-95 B1-ID 定义（27 文件）；:26-33 全局约束；:38 Review Focus |
| 冻结计划 | `docs/plans/passb/00-contract-and-ownership-freeze.md` | B0.3 裁定 #2/#4（handler/session 逐路径属主、平台 helper 不认领） |
| 所有权矩阵 | `docs/architecture/passb/ownership-matrix.yaml` | `plan: 10-identity` 的 27 行（全部 `integration_owner: ib1`、`delete_barrier: ib1`，destination `internal/modules/identity/{repository,service,handler}`） |
| 契约 | `docs/architecture/passb/contracts.yaml` | identity 12 行契约（`identity.{audit-log-service,auth-handler-construction,facade,lifecycle,organization-service,routes,tenant-api-key-service,tenant-invitation-service,tenant-member-service,tenant-service,user-service,workers}`），全部 `stability: frozen` |
| 事件目录 | `docs/architecture/passb/event-catalog.yaml` | identity 无事件行（B0.5 冻结，本节点不新增事件） |
| 例外台账 | `docs/architecture/passb/exception-ledger.yaml` | identity 相关行数 = 0（`grep "10-identity"` 无命中）——本节点无例外删除义务，禁止新增例外 |
| 搬迁清单 | `docs/architecture/moves/identity.yaml` | 27 条 legacy_files（`passb_task: B-identity`）；alias_obligations 为空；integration_points（5 路由入口、0 worker、2 生命周期挂点） |
| 执行 DAG | `docs/plans/passb/execution-dag.json`（integration 分支） | 节点 `b1-identity` 的 owned_files/gates/notes；top-level `package_private_couplings` |
| 执行公约 | `.superpowers/sdd/passb/conventions.md` | §1 派发契约、§2 门禁、§3 禁改、§7 耦合处理、§8 计数基线 |

## 1. 已验证输入（计划撰写会话实测，实施者必须复核）

以下事实在计划撰写会话中用 `grep`/`python3` 符号级扫描（剥离注释与字符串后匹配同包裸标识符）逐条验证过：

1. **27 个属主文件**：ownership-matrix `plan: 10-identity` 共 27 行 = repository 8 + service 9 + handler 10，与 `moves/identity.yaml` legacy_files 逐条一致。
2. **identity 无 alias 义务、无 import 例外行**（exception-ledger 全文无 `10-identity`）。
3. **契约符号全部定义在 `internal/types/interfaces/`**（如 `internal/types/interfaces/user.go:UserService`、`tenant.go:TenantService`、`tenant.go:TenantAPIKeyService`、`tenant_invitation_service.go:TenantInvitationService`、`tenant_member_service.go:TenantMemberService`、`organization.go:OrganizationService`、`audit_log.go:AuditLogService`）。**这些接口文件不属于本节点 27 文件，本节点不移动、不修改它们**——消费方（middleware/router/container）经接口消费，接口不动则消费方不破。
4. **宿主包内符号级耦合全量清单**见 §3（含 DAG `package_private_couplings` 之外的 4 类新发现：handler 宿主包的策略/平台/知识未导出调用、rbac_lookups 的外属主类型方法、service 宿主包的 semanticScopeGuard 内嵌）。
5. **门禁工具行为**（在 `/tmp` 模拟树上实测）：`go run ./tools/modulemove verify` 对"manifest 行存在但磁盘文件缺失"报 `legacy-file: not found` 并退出 1；删除该 manifest 行后同一命令退出 0。⇒ **搬走一个文件必须同 commit 删除其 manifest legacy_files 行**。`tools/architectureguard` 的 `legacy-guard` 对横向目录内"无归属新生产文件"报诊断（check.go:1264）⇒ **新建的宿主兼容文件必须登记进 identity.yaml legacy_files**。
6. **计数基线**：633 路由 / 23+23 worker / 58 hook / 537 migration（framework:17、conventions §8）。本节点不改任何注册点，计数不变。

## 2. 全局约束（继承，实施者逐条遵守）

- 只写 `owned_files` 列出的文件 + 本节点自身产出的新文件（conventions §1.2）。本节点 owned_files = ownership-matrix `plan: 10-identity` 的 27 行所指文件 + `internal/modules/identity/**`。
- 禁改（conventions §3）：`internal/router/router.go`、`internal/container/container.go`、`internal/bootstrap/{routes,workers,lifecycle}.go`、`router/task.go`、`sync_task.go`、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件、`docs/architecture/passb/{ownership-matrix,contracts,event-catalog}.yaml`（barrier 回写）、其他 owner 的宿主包文件。
- 每搬一个生产文件随迁其主题 `_test.go`；宿主包不保留转发性业务声明（终态；过渡期薄 shim 见 §3.4，全部登记删除点）。
- 禁止复制他模块实现、禁止新增通配例外、禁止把依赖塞进 common（conventions §5）。
- 一个任务一个 commit；搬迁与行为修改不得混 commit（conventions §4）。
- 节点 gates（DAG `b1-identity.gates`，逐条原样执行）：`go build ./...`、`go test -count=1 ./internal/modules/identity/...`、`make check-backend-architecture`、`make verify-module-moves`。

## 3. 耦合面裁定（本计划核心机制）

三个宿主包内，identity 属主文件与其他 owner 文件存在同包未导出互耦。处理规则（依 conventions §7 与 DAG 节点裁定）：

### 3.1 反向耦合：identity 定义的未导出符号被其他 owner 消费 → 导出窄端口 + 宿主薄 shim

identity 文件搬走后，仍留在宿主包的其他 owner 文件对下列符号的调用会断链。裁定：**identity 在新包内把符号导出（改名为首字母大写），并在宿主包留一行委托 shim**；跨 owner 调用点文件本身不改，重线与 shim 删除走 Integration Brief 由集成工程师在 IB1 执行（conventions §1.3、§7.2）。

| 符号（现名@定义处） | 导出名（新包内） | 宿主内消费方（不改） | 消费方属主 |
|---|---|---|---|
| `getJwtSecret` @ service/user.go:81 | `JWTSecret` | service/sandbox_terminal_ticket.go:52,66 | 13-execution |
| `tenantIDFromClaims` @ service/user.go:1404 | `TenantIDFromClaims` | service/sandbox_terminal_ticket.go:82 | 13-execution |
| `auditActor` @ service/tenant_member.go:162 | `AuditActor` | service/system_setting.go:999,1195；service/kb_activity.go:157 | 42-system-policy；22-knowledge-retrieval |
| `auditActorRole` @ service/tenant_member.go:156 | `AuditActorRole` | service/kb_activity.go:160 | 22-knowledge-retrieval |
| `urlQueryEscape` @ handler/auth.go:750 | `URLQueryEscape` | handler/mcp_oauth.go | 11-airesource |
| `resolveTenantSelfServiceCreationEnabled` @ handler/tenant_policy.go:19 | `ResolveTenantSelfServiceCreationEnabled` | handler/tenant.go | 本节点推迟件（见 3.2） |

### 3.2 正向耦合：identity 消费其他 owner 定义且仍留在宿主包的未导出符号 → 8 个文件推迟到 IB1 迁移

identity 属主文件调用"定义在**非 identity 宿主文件**里的未导出符号"。这些定义文件在 B1 阶段不动（knowledge=B2、policy=b4、platform helper 由 B0.3 裁定 #4 永驻宿主待独立抽取计划），identity 无法改它们，也无法经宿主导出桥接——因为宿主兼容层必须 import identity 新包（供 container/router 别名，见 3.4），identity 新包再 import 宿主即成 **Go 包循环**，物理不可能。复制实现被节点裁定明令禁止（framework:38）。**唯一可行解：这 8 个文件本节点不搬，物理迁移推迟到 IB1**——届时集成工程师先拆除宿主兼容层（container/router 直连 identity 包），循环解除后按 Integration Brief 添加跨 owner 导出 shim 并迁移文件。8 文件与阻塞符号：

| 推迟文件 | 阻塞符号@定义文件（属主） |
|---|---|
| repository/tenant.go | `escapeLikeKeyword` @ repository/knowledge.go:22（24-knowledge-process；调用点 tenant.go:85,90） |
| repository/tenant_member.go | `escapeLikePattern` @ repository/wiki_page.go:1258（23-knowledge-wikifaq；调用点 tenant_member.go:109,134）。注：`forUpdateClause`（本文件:25 定义，memory_extraction.go:17 消费）因文件推迟而一并推迟导出 |
| handler/tenant.go | `isStorageProviderAllowed`/`firstAllowedStorageProvider` @ handler/storage_allowlist.go:13（policy.yaml:27，b4-systempolicy 属主；节点裁定明示"不在本节点契约面"） |
| handler/tenant_invitation.go | `parseListPagination` @ handler/list_pagination.go:23（platform，B0.3 裁定 #4 永驻） |
| handler/tenant_member.go | `parseListPagination`（同上） |
| handler/organization.go | `sharedKBRow` @ handler/knowledgebase.go:199（22-knowledge-retrieval） |
| handler/tenant_invite_link.go | 定义 `*TenantInvitationHandler`（tenant_invitation.go，推迟件）上的方法 `CreateInviteLink`（tenant_invite_link.go:69）——接收者类型不在本包则方法无法声明 |
| handler/rbac_lookups.go | 在 `*KnowledgeBaseHandler`/`*KnowledgeHandler`/`*WikiPageHandler`/`*ChunkHandler`（22/21-knowledge）与 `*CustomAgentHandler`（25-agentcatalog）上声明方法（rbac_lookups.go:49 起）；去方法化改动 router 调用形参（integrator 文件），按 conventions §7.4 裁定推迟 |

这 8 个文件本节点保持在原位、manifest 行保留、不产生任何编译变化。它们的迁移编排写入 Integration Brief（任务 B1-ID.6）。推迟不改变 ownership-matrix 属主（仍为 10-identity）与 `delete_barrier: ib1`——IB1 执行迁移后由 barrier 收口矩阵行。

### 3.3 semanticScopeGuard：消费方自持 seam（有记录的偏差）

`semanticScopeGuard`（service/semantic_scope.go:31，22-knowledge-retrieval 属主，B2 才搬）被 identity 的 4 个搬移文件内嵌：service/tenant.go:57、organization.go:47、tenant_member.go:78、user.go:100。容器装饰器（container.go:343-365）对 `interfaces.TenantMemberService`/`interfaces.OrganizationService` 做 `s.(interface{ SetSemanticScopeInvalidator(interfaces.SemanticScopeInvalidator) }).SetSemanticScopeInvalidator(i)` 类型断言，**方法缺失即启动 panic**，故搬移后必须保留该方法面。经 3.2 同样的循环论证无法经宿主桥接。裁定：identity service 新包内新建**同形本地 seam**（文件 `internal/modules/identity/service/semantic_scope_guard.go`，结构体名保持 `semanticScopeGuard`），仅实现 identity 实际使用的 4 个成员（`SetSemanticScopeInvalidator`/`invalidateSemanticTenant`/`invalidateSemanticUser`/`invalidateSemanticOrganization`，方法体逐字复制 semantic_scope.go:35-62 的委托逻辑），委托目标为**冻结导出端口** `interfaces.SemanticScopeInvalidator`（internal/types/interfaces/semantic_scope.go:11-17）。理由：Spec §4.2"接口优先定义在使用方模块"——端口已在 types/interfaces 冻结导出，内嵌体是使用方糖；语义失效收口属于知识域（B2），K2 搬 semantic_scope.go 时将面对同样问题，IB2 barrier 按 conventions §7.1"多属主重复 helper 族在 barrier 收口"统一裁定。此偏差必须在报告与 Brief 中显式登记（见任务 B1-ID.5/B1-ID.6），供审查者复核。

### 3.4 宿主兼容层（compat）：内容、方向与登记

三个宿主包各新建一个兼容文件，**只 import identity 新包（单向）**，内容包括三类：
1. **构造器/类型/哨兵别名**（var/type alias）：保持 container.go 与 router 文件原样编译；
2. **未导出 shim**（3.1 的 6 个符号）：保持宿主内其他 owner 文件原样编译；
3. 全部标注删除点（IB1 按 Brief 删除）。

兼容文件是横向目录新生产文件，必须**同 commit 登记进 `docs/architecture/moves/identity.yaml` legacy_files**（`passb_task: B-identity`），否则 `make check-backend-architecture` 的 legacy-guard 报诊断（§1.5 实测）。passbguard 的矩阵↔manifest 对照是 barrier 门禁：IB1 删除兼容文件时同步删 manifest 行与矩阵行（Brief 指令）。

**别名清单（逐符号，来自对 container.go/router/宿主残留消费方的全量扫描）：**

`internal/application/service/identity_passb_compat.go`：
- var 别名（消费方 container.go:333-340,381）：`NewTenantService`、`NewTenantAPIKeyService`、`NewTenantMemberService`、`NewTenantInvitationService`、`NewAuditLogService`、`NewAuditLogRetentionRunner`、`NewOrganizationService`、`NewUserService`
- type 别名（消费方 container.go:2453 `*service.AuditLogRetentionRunner`）：`AuditLogRetentionRunner`
- 哨兵 var 别名（消费方 service/agent_share.go、service/kbshare.go，留在宿主）：`ErrOrgNotFound`、`ErrTenantNotInOrg`、`ErrInvalidRole`（现定义 service/organization.go:28,32,33）
- 未导出 shim：`getJwtSecret`、`tenantIDFromClaims`、`auditActor`、`auditActorRole`（3.1）

`internal/application/repository/identity_passb_compat.go`：
- var 别名（消费方 container.go:190,200,201,229,285,286,302）：`NewTenantAPIKeyRepository`、`NewTenantInvitationRepository`、`NewAuditLogRepository`、`NewMobileExchangeStore`、`NewUserRepository`、`NewAuthTokenRepository`、`NewOrganizationRepository`
- type 别名（消费方 container.go:751 `*repository.MobileExchangeStore`）：`MobileExchangeStore`
- 函数 var 别名（消费方 repository/voice_session.go，留在宿主）：`HashMobileExchangeValue`
- （无未导出 shim——`forUpdateClause` 定义文件 tenant_member.go 推迟，原符号仍在宿主）

`internal/handler/identity_passb_compat.go`：
- var 别名（消费方 container.go:710 `NewAuditLogHandler`、:750 `NewAuthHandler`）：二者
- type 别名（消费方 router.go:53,83、routes_auth_tenant.go:206,282、routes_knowledge.go）：`AuthHandler`、`AuditLogHandler`
- 未导出 shim：`urlQueryEscape`、`resolveTenantSelfServiceCreationEnabled`（3.1）

## 4. 文件计划（精确清单）

### 4.1 本节点物理搬移（19 生产文件 + 28 测试文件）

**repository → `internal/modules/identity/repository/`（package repository）**，随迁测试 4：

| 生产文件 | 随迁 _test.go |
|---|---|
| internal/application/repository/audit_log.go | audit_log_scope_test.go |
| internal/application/repository/mobile_auth_exchange.go | mobile_auth_exchange_test.go |
| internal/application/repository/organization.go | （无同名测试） |
| internal/application/repository/tenant_api_key.go | tenant_api_key_test.go |
| internal/application/repository/tenant_invitation.go | （无同名测试） |
| internal/application/repository/user.go | user_tenantless_test.go |

（repository/tenant_test.go 测推迟件 tenant.go，留在宿主。）

**service → `internal/modules/identity/service/`（package service）**，随迁测试 17：

| 生产文件 | 随迁 _test.go |
|---|---|
| audit_log.go | audit_log_test.go |
| audit_log_retention.go | audit_log_retention_test.go |
| organization.go | （无同名测试） |
| password_policy.go | password_policy_test.go |
| tenant.go | tenant_deletion_test.go、tenant_storagebackend_test.go（二者为 `package service_test` 外部测试，需 import 重线，见 B1-ID.3 Step 4） |
| tenant_api_key.go | tenant_api_key_test.go |
| tenant_invitation.go | tenant_invitation_test.go |
| tenant_member.go | tenant_member_test.go、tenant_member_connections_test.go |
| user.go | user_admin_create_test.go、user_auth_token_test.go、user_jwt_test.go、user_oidc_security_test.go、user_oidc_verify_test.go、user_preferences_default_model_test.go、user_provisioning_test.go、user_switch_tenant_test.go |

**handler → `internal/modules/identity/handler/`（package handler）**，随迁测试 7 + 拆分 1：

| 生产文件 | 随迁 _test.go |
|---|---|
| audit_log.go | audit_log_test.go |
| auth.go | auth_change_password_test.go、auth_mobile_exchange_test.go、auth_oidc_mobile_test.go、auth_oidc_start_test.go、auth_register_invite_only_test.go |
| auth_register_by_invite.go | auth_register_by_invite_tenant_test.go |
| tenant_policy.go | （拆分自 tenant_self_service_policy_test.go 的 `TestAuthMeProjectsTenantCreationCapability`（:128）迁入新文件 `internal/modules/identity/handler/auth_self_service_policy_test.go`；`TestCreateTenantRejectsRegularUserWhenSelfServiceDisabled`（:71）与 `TestCreateTenantAllowsCrossTenantSuperuserWhenSelfServiceDisabled`（:99）留在宿主原文件——其被测对象 tenant.go 推迟） |

留在宿主的 handler 测试（被测对象为推迟件或他 owner 文件，不得随迁）：rbac_lookups_test.go、tenant_api_key_validation_test.go、tenant_parser_ssrf_test.go、tenant_query_history_config_test.go、tenant_secret_disclosure_test.go、tenant_member_test.go、tenant_invitation_accept_by_token_test.go、tenant_invitation_auto_accept_test.go、tenant_self_service_policy_test.go（拆分后余量）。

### 4.2 本节点新建文件

| 新文件 | 内容 |
|---|---|
| internal/modules/identity/repository/（包目录） | 6 文件 + 4 测试迁入 |
| internal/modules/identity/service/semantic_scope_guard.go | §3.3 本地 seam |
| internal/modules/identity/service/（包目录） | 9 文件 + 17 测试迁入 |
| internal/modules/identity/handler/（包目录） | 4 文件 + 7 测试 + 1 拆分测试迁入 |
| internal/application/repository/identity_passb_compat.go | §3.4 |
| internal/application/service/identity_passb_compat.go | §3.4 |
| internal/handler/identity_passb_compat.go | §3.4 |
| docs/architecture/passb/briefs/b1-identity.md | Integration Brief（任务 B1-ID.6 产出） |
| docs/architecture/evidence/passb/b1-identity.md | 证据文件（B1-ID.1 建骨架，B1-ID.5 完成差分章节） |
| docs/plans/passb/reports/b1-identity.md | 实施报告（B1-ID.6 产出） |

### 4.3 本节点编辑文件

| 文件 | 编辑内容 |
|---|---|
| docs/architecture/moves/identity.yaml | 删除 19 条已搬 legacy_files 行；新增 3 条兼容文件行（`passb_task: B-identity`，reason 注明 IB1 删除） |
| internal/modules/identity/legacy/README.md | 镜像 manifest：19 行标已迁移（记录目标包），8 行推迟件保留并注明阻塞符号与 IB1 编排指针 |
| internal/modules/identity/README.md | 增补 Pass B 包结构（repository/service/handler）、公开端口清单（§5）、兼容层与推迟件状态 |
| （随迁的外部测试 2 个） | import 重线（B1-ID.3 Step 4） |
| （随迁测试中直接调用被导出改名符号者） | 调用点改名（user_jwt_test.go 的 `tenantIDFromClaims` → `TenantIDFromClaims`） |

### 4.4 明确不动

27 文件中的 8 个推迟件（§3.2）；`internal/modules/identity/module.go`（门面注释形态已与 contracts.yaml `identity.facade` 冻结值一致，装配归 IB1）；`internal/types/interfaces/**`；`internal/router/**`、`internal/container/**`、`internal/middleware/**`；所有其他 owner 的宿主文件；`docs/architecture/passb/{ownership-matrix,contracts,event-catalog,exception-ledger}.yaml`。

## 5. 导出端口与真实签名（以现仓库代码为准，零发明）

搬移时在 identity 新包内做"导出改名"（仅首字母大写 + 包内调用点机械更新，函数体不变）：

```go
// internal/modules/identity/service/user.go（自 service/user.go:81 搬移并导出）
// JWTSecret retrieves the JWT secret from the environment, falling back to a securely generated random secret.
func JWTSecret() string

// internal/modules/identity/service/user.go（自 :1404）
// TenantIDFromClaims pulls the active tenant ID out of a parsed JWT claim map.
func TenantIDFromClaims(claims jwt.MapClaims, fallback uint64) uint64

// internal/modules/identity/service/tenant_member.go（自 :162）
// AuditActor returns the calling user id from context, "" when no authenticated caller is present.
func AuditActor(ctx context.Context) string

// internal/modules/identity/service/tenant_member.go（自 :156）
// AuditActorRole picks up the caller's role at write-time. Empty if auth middleware didn't set it.
func AuditActorRole(ctx context.Context) string

// internal/modules/identity/handler/auth.go（自 :750）
func URLQueryEscape(value string) string

// internal/modules/identity/handler/tenant_policy.go（自 :19，签名逐字保留仅改名）
func ResolveTenantSelfServiceCreationEnabled(
	ctx context.Context,
	cfg *config.Config,
	settings interfaces.SystemSettingService,
) bool
```

**搬移后保持不变的公开构造器（container 经别名继续消费，签名逐字不动）：**

```go
// repository 包
func NewAuditLogRepository(db *gorm.DB) interfaces.AuditLogRepository            // audit_log.go:22
func NewMobileExchangeStore(db *gorm.DB) *MobileExchangeStore                     // mobile_auth_exchange.go:30
func NewOrganizationRepository(db *gorm.DB) interfaces.OrganizationRepository     // organization.go:32
func NewTenantAPIKeyRepository(db *gorm.DB) interfaces.TenantAPIKeyRepository    // tenant_api_key.go:19
func NewTenantInvitationRepository(db *gorm.DB) interfaces.TenantInvitationRepository // tenant_invitation.go:25
func NewUserRepository(db *gorm.DB) interfaces.UserRepository                     // user.go:28
func NewAuthTokenRepository(db *gorm.DB) interfaces.AuthTokenRepository           // user.go:287

// service 包
func NewAuditLogService(repo interfaces.AuditLogRepository) interfaces.AuditLogService // audit_log.go:33
func NewAuditLogRetentionRunner(cfg *config.Config, svc interfaces.AuditLogService) *AuditLogRetentionRunner // audit_log_retention.go:62
func NewOrganizationService(orgRepo interfaces.OrganizationRepository, userRepo interfaces.UserRepository, shareRepo interfaces.KBShareRepository, agentShareRepo interfaces.AgentShareRepository) interfaces.OrganizationService // organization.go:55
func NewTenantService(repo interfaces.TenantRepository, storageRepo interfaces.StorageBackendRepository, opts ...TenantServiceOption) interfaces.TenantService // tenant.go:64
func NewTenantAPIKeyService(repo interfaces.TenantAPIKeyRepository) interfaces.TenantAPIKeyService // tenant_api_key.go:30
func NewTenantInvitationService(repo interfaces.TenantInvitationRepository, memberSvc interfaces.TenantMemberService, audit interfaces.AuditLogService) interfaces.TenantInvitationService // tenant_invitation.go:98
func NewTenantMemberService(repo interfaces.TenantMemberRepository, audit interfaces.AuditLogService, userRepo interfaces.UserRepository, tokenRepo interfaces.AuthTokenRepository, opts ...TenantMemberOption) interfaces.TenantMemberService // tenant_member.go:122
func NewUserService(configInfo *config.Config, userRepo interfaces.UserRepository, tokenRepo interfaces.AuthTokenRepository, tenantService interfaces.TenantService, memberService interfaces.TenantMemberService, systemSettingSvc interfaces.SystemSettingService) interfaces.UserService // user.go:110

// handler 包
func NewAuthHandler(configInfo *config.Config, userService interfaces.UserService, tenantService interfaces.TenantService, systemSettingSvc interfaces.SystemSettingService, invitationSvc interfaces.TenantInvitationService) *AuthHandler // auth.go（contracts.yaml identity.auth-handler-construction 冻结签名）
func NewAuditLogHandler(auditService interfaces.AuditLogService) *AuditLogHandler // audit_log.go:25
```

**本地 seam（§3.3）：**

```go
// internal/modules/identity/service/semantic_scope_guard.go
type semanticScopeGuard struct {
	semanticInvalidator interfaces.SemanticScopeInvalidator // 冻结端口：internal/types/interfaces/semantic_scope.go:11-17
}
func (s *semanticScopeGuard) SetSemanticScopeInvalidator(i interfaces.SemanticScopeInvalidator)
func (s *semanticScopeGuard) invalidateSemanticTenant(ctx context.Context, tenant uint64) error
func (s *semanticScopeGuard) invalidateSemanticUser(ctx context.Context, user string) error
func (s *semanticScopeGuard) invalidateSemanticOrganization(ctx context.Context, org string) error
// 方法体 = semantic_scope.go:35-62 同形委托（nil 安全 + 转发），不携带 identity 未用的 KB/Transfer 变体。
```

**宿主兼容文件完整内容模板（三个文件同构，示例给出 service 版）：**

```go
package service

// Pass B b1-identity 断链过渡兼容层（conventions §7.1 薄 shim / §3.4）。
// 唯一真源在 internal/modules/identity/service；本文件零业务逻辑，IB1 按
// docs/architecture/passb/briefs/b1-identity.md 删除（同时删除 manifest 登记行）。
import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	identitysvc "github.com/Tencent/WeKnora/internal/modules/identity/service"
)

// ---- 未导出 shim：宿主内其他 owner 文件的既有调用点不改（重线走 IB1） ----
func getJwtSecret() string                     { return identitysvc.JWTSecret() }
func tenantIDFromClaims(c jwt.MapClaims, f uint64) uint64 { return identitysvc.TenantIDFromClaims(c, f) }
func auditActor(ctx context.Context) string     { return identitysvc.AuditActor(ctx) }
func auditActorRole(ctx context.Context) string { return identitysvc.AuditActorRole(ctx) }

// ---- 构造器/类型/哨兵别名：container.go 与宿主残留消费方不改 ----
var (
	NewTenantService          = identitysvc.NewTenantService
	NewTenantAPIKeyService    = identitysvc.NewTenantAPIKeyService
	NewTenantMemberService    = identitysvc.NewTenantMemberService
	NewTenantInvitationService = identitysvc.NewTenantInvitationService
	NewAuditLogService        = identitysvc.NewAuditLogService
	NewAuditLogRetentionRunner = identitysvc.NewAuditLogRetentionRunner
	NewOrganizationService    = identitysvc.NewOrganizationService
	NewUserService            = identitysvc.NewUserService

	ErrOrgNotFound    = identitysvc.ErrOrgNotFound
	ErrTenantNotInOrg = identitysvc.ErrTenantNotInOrg
	ErrInvalidRole    = identitysvc.ErrInvalidRole
)
type AuditLogRetentionRunner = identitysvc.AuditLogRetentionRunner
```

## 6. 公共可观察行为与兼容要求（差分基准）

以下行为是本节点的外部契约，搬移前后必须逐项等价（Spec §7、framework:32）：

1. **HTTP 面**：633 条路由、方法、路径、RBAC/APIKey 门、状态码映射零变化（本节点不触 router/middleware）。
2. **装配面**：dig Provider 集合与 Decorator 行为零变化——`container.Decorate` 对 TenantMember/Organization service 的 `SetSemanticScopeInvalidator` 类型断言必须仍然成立（§3.3 seam 保障）；`startAuditLogRetention`（container.go:639→:2453）经 `*service.AuditLogRetentionRunner` 别名继续解析，Start/Stop 语义（24h 周期、retentionDays<=0 短路、ResourceCleaner 注册名 "AuditLogRetentionRunner"）不变。
3. **JWT/认证面**：`JWTSecret` 的 env 优先 + 32 字节随机回退 + sync.Once 语义不变；`TenantIDFromClaims` 的 float64/int64/uint64/负值分支不变（特征化测试 `user_jwt_test.go:TestTenantIDFromClaims`）。
4. **审计面**：`AuditActor`/`AuditActorRole` 的 context 取值与空串回退不变；AuditLogService 的 Log/LogDenied 限频（denyDedupWindow=1min）/List/Purge 语义不变。
5. **哨兵错误**：`ErrOrgNotFound`/`ErrTenantNotInOrg`/`ErrInvalidRole` 经 var 别名保持**同一 error 值**（errors.Is 跨包比较不受影响）。
6. **LIKE 逃逸**：推迟件 tenant.go/tenant_member.go 原文件不动，SearchTenants/ListMembersPage 的 SQL 逃逸行为字面不变。
7. **导入即编译**：宿主包、`internal/modules/identity/...`、全仓 `go build ./...` 退出码 0。

## 7. 任务

> 提交规范（conventions §4）：一个任务一个 commit；以下每任务末尾给出 commit 模板。基线 SHA 按派发时 DAG `base_sha`（当前 = `d57a2fa708c3fecf5f0510553ea26db3de51d3c9`，b0 合入点）。

### Task B1-ID.1：前置校验与迁移基线（T0）

**Files:** Create `docs/architecture/evidence/passb/b1-identity.md`（骨架）。

- [ ] **Step 1: 校验前置条件**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/passb-b1-identity
git log --oneline -1                       # 预期：d57a2fa70 docs(passb): 登记 b0 合入 integration…
git branch --show-current                  # 预期：codex/passb-b1-identity
python3 -c "import json;d=json.load(open('docs/plans/passb/execution-dag.json'));n=[x for x in d['nodes'] if x['id']=='b1-identity'][0];print(n['status'],n['base_sha'])"
# 预期：base_sha 与 git rev-parse HEAD 一致；b0 已 done（查 b0 节点 status）
```

- [ ] **Step 2: 记录基线（T0）**

```bash
git rev-parse HEAD | tee /tmp/b1id-baseline-sha
go build ./... ; echo "build_exit=$?"                                # 预期 0（仅链接器既有告警）
go test -count=1 ./internal/application/repository/ 2>&1 | tail -3   # 记录 PASS/FAIL 台账
go test -count=1 ./internal/application/service/ 2>&1 | tail -3
go test -count=1 ./internal/handler/ 2>&1 | tail -3
make verify-module-moves && make check-backend-architecture          # 预期双绿
```

把每条命令原文 + 退出码 + 末行输出写入 evidence 文件 §T0。已知既有失败按 F0 台账标注，不算回归。

- [ ] **Step 3: Commit**

```bash
git add docs/architecture/evidence/passb/b1-identity.md
git commit -m "test(passb): record b1-identity T0 baseline"
```

### Task B1-ID.2：repository 层归位（6 文件）

**Files:** Move §4.1 repository 行；Create `internal/modules/identity/repository/`、`internal/application/repository/identity_passb_compat.go`；Edit `docs/architecture/moves/identity.yaml`、`internal/modules/identity/legacy/README.md`。

- [ ] **Step 1: git mv（纯 rename，不改函数体）**

```bash
mkdir -p internal/modules/identity/repository
for f in audit_log mobile_auth_exchange organization tenant_api_key tenant_invitation user; do
  git mv internal/application/repository/$f.go internal/modules/identity/repository/$f.go
done
git mv internal/application/repository/audit_log_scope_test.go      internal/modules/identity/repository/
git mv internal/application/repository/mobile_auth_exchange_test.go internal/modules/identity/repository/
git mv internal/application/repository/tenant_api_key_test.go       internal/modules/identity/repository/
git mv internal/application/repository/user_tenantless_test.go      internal/modules/identity/repository/
git diff --summary --find-renames HEAD | head -20   # 预期：全部 rename 行（R100）
```

包名保持 `package repository`（目录即包，零 package 声明修改）。

- [ ] **Step 2: 写宿主兼容文件（RED 前置：先确认不写则编译断）**

```bash
go build ./... 2>&1 | head -5   # 预期：报 container.go/voice_session.go 引用 NewAuditLogRepository 等未定义——这是 shim 必要性证据，记入 evidence
```

按 §5 模板创建 `internal/application/repository/identity_passb_compat.go`（7 个构造器 var 别名 + `MobileExchangeStore` type 别名 + `HashMobileExchangeValue` var 别名；无未导出 shim）。

- [ ] **Step 3: 编译与目标测试（GREEN）**

```bash
go build ./... && echo OK                                   # 预期 OK
go test -count=1 ./internal/modules/identity/repository/    # 预期 ok（4 个随迁测试全 PASS）
go test -count=1 ./internal/application/repository/ 2>&1 | tail -2   # 预期与 T0 台账一致（tenant_test.go 等留驻测试不受影响）
```

- [ ] **Step 4: manifest 与镜像更新**

编辑 `docs/architecture/moves/identity.yaml`：删除已搬 6 条 legacy_files 行；新增：

```yaml
  - path: internal/application/repository/identity_passb_compat.go
    reason: 'Pass B b1-identity 断链过渡薄 shim（conventions §7.1），IB1 按 briefs/b1-identity.md 删除'
    navigation_label: Identity passb compat (application/repository)
    passb_task: B-identity
```

同步 `internal/modules/identity/legacy/README.md`（6 行标记已迁移 + 新增 compat 行）。验证：

```bash
go run ./tools/modulemove verify --module identity   # 预期 modulemove: OK (identity)
make verify-module-moves                             # 预期 OK (16 manifests verified)
make check-backend-architecture                      # 预期 0 诊断
```

- [ ] **Step 5: Commit** — `git add -A && git commit -m "refactor(identity): move repository layer into module (6 files, host compat shim)"`

### Task B1-ID.3：service 层归位（9 文件 + 本地 seam + 4 端口导出）

**Files:** Move §4.1 service 行；Create `internal/modules/identity/service/`、`semantic_scope_guard.go`、`internal/application/service/identity_passb_compat.go`；Edit manifest/镜像/2 个外部测试 import。

- [ ] **Step 1: 特征化确认（RED 意义上的既有锚点）**——`user_jwt_test.go:TestTenantIDFromClaims`、`tenant_member_test.go`、`audit_log_retention_test.go` 已锚定 §6.2-6.4 行为，随迁后同名同断言运行即差分。无需新写特征化（contracts.yaml characterization_tests 已登记 9 个 identity 相关测试文件）。
- [ ] **Step 2: git mv（9 生产 + 17 测试，命令同 B1-ID.2 模式）**；`git diff --summary --find-renames HEAD` 全 R100。
- [ ] **Step 3: 导出改名（仅 4 符号 + 包内调用点）**——user.go:81 `getJwtSecret`→`JWTSecret`、:1404 `tenantIDFromClaims`→`TenantIDFromClaims`；tenant_member.go:156/:162 →`AuditActorRole`/`AuditActor`。包内调用点与随迁测试调用点（user_jwt_test.go 等）同步改名。**函数体一行不改**。
- [ ] **Step 4: 本地 seam**——按 §5 创建 `semantic_scope_guard.go`；4 个搬入文件的 `semanticScopeGuard` 内嵌声明零改动（同名同形）。
- [ ] **Step 5: 外部测试 import 重线**——`tenant_deletion_test.go`/`tenant_storagebackend_test.go`（`package service_test`）：
  - `service.NewTenantService`/`service.TenantDeletionGuard`/`service.WithDeletionGuard` → `identitysvc.`（import `identitysvc "github.com/Tencent/WeKnora/internal/modules/identity/service"`）；
  - `repository.NewTenantRepository`/`repository.NewStorageBackendRepository` **保持宿主 import 不变**（二者定义文件本节点不搬）。
- [ ] **Step 6: 宿主兼容文件**——先 `go build ./...` 记录断链证据（container.go 8 构造器 + sandbox_terminal_ticket.go/system_setting.go/kb_activity.go/agent_share.go/kbshare.go 引用未定义），再按 §5 模板写 `internal/application/service/identity_passb_compat.go`（8 var + 1 type + 3 哨兵 + 4 shim）。
- [ ] **Step 7: GREEN + manifest**——`go build ./...` OK；`go test -count=1 ./internal/modules/identity/...` ok；`go test -count=1 ./internal/application/service/` 与 T0 台账一致；manifest 删 9 行加 1 compat 行；`make verify-module-moves && make check-backend-architecture` 双绿。
- [ ] **Step 8: Commit** — `refactor(identity): move service layer into module, export 4 narrow ports, add semantic guard seam`

### Task B1-ID.4：handler 层归位（4 文件 + 2 端口导出 + 1 测试拆分）

**Files:** Move §4.1 handler 行；Create `internal/modules/identity/handler/`、`auth_self_service_policy_test.go`、`internal/handler/identity_passb_compat.go`；Edit `internal/handler/tenant_self_service_policy_test.go`（拆分）、manifest/镜像。

- [ ] **Step 1: git mv**——audit_log.go、auth.go、auth_register_by_invite.go、tenant_policy.go + 7 测试；`git diff --summary` 全 R100。
- [ ] **Step 2: 导出改名**——auth.go:750 `urlQueryEscape`→`URLQueryEscape`（包内调用点同步）；tenant_policy.go:19 `resolveTenantSelfServiceCreationEnabled`→`ResolveTenantSelfServiceCreationEnabled`。搬入文件对 `service.`/`repository.` 的 import 重线到 identity 新包（其引用符号——`ValidatePasswordPolicy`、`ErrInvalidOldPassword`、`MobileExchangeStore` 等——全部定义在已搬入的 identity 文件内，本会话已验证无宿主残留符号）。
- [ ] **Step 3: 拆分混合测试**——`tenant_self_service_policy_test.go`：`TestAuthMeProjectsTenantCreationCapability`（:128 起，含其专用 stub/helper）移入 `internal/modules/identity/handler/auth_self_service_policy_test.go`（`package handler`，可继续用 `&AuthHandler{...}` 未导出字段字面量）；其余两函数留在宿主原文件。
- [ ] **Step 4: 宿主兼容文件**——先记录断链证据（router.go:53,83、routes_auth_tenant.go:206,282、routes_knowledge.go:266、container.go:710,750、handler/tenant.go、handler/mcp_oauth.go），再按 §5 模板写（2 var + 2 type + 2 shim）。
- [ ] **Step 5: GREEN + manifest**——`go build ./...` OK；`go test -count=1 ./internal/modules/identity/...` ok；`go test -count=1 ./internal/handler/` 与 T0 台账一致（留驻 9 测试经别名/shim 编译不变）；manifest 删 4 行加 1 compat 行；双守卫绿。
- [ ] **Step 6: Commit** — `refactor(identity): move handler layer into module, export 2 narrow ports, split mixed policy test`

### Task B1-ID.5：高风险差分证据与门禁复核

**Files:** Edit `docs/architecture/evidence/passb/b1-identity.md`（差分章节）。

高风险面（framework:40）：**Tenant/RBAC**（identity 命中；payment/usage/session stream 等不在本节点面）。差分口径（conventions §6）：旧实现特征化测试 → 搬迁 → 新实现同用例运行 → 逐用例等价比对。

- [ ] **Step 1: 用例集**——T0 已运行的三个宿主包测试在搬移后原地复跑：

```bash
go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/ 2>&1 | tee /tmp/b1id-post-host.txt
go test -count=1 ./internal/modules/identity/... 2>&1 | tee /tmp/b1id-post-module.txt
```

逐包对比 /tmp 下 T0 与本次输出的 PASS/FAIL 集合：**同一测试集合结果一致**（framework:14.2 纯移动验证）。任何新失败 → 停止，定位本批第一个破坏提交（conventions §2）。
- [ ] **Step 2: RBAC/认证要点比对**——在 evidence 中逐条登记 §6.2-6.5 的等价证据：`TestTenantIDFromClaims`（claims 分支）、auth_* 系列（OIDC/邀请注册/改密 400 语义）、tenant_deletion/tenant_storagebackend（CreateTenant 存储后端副作用）、audit_log_retention（Purge/周期）、rbac_lookups_test（留驻未动，作为"未搬迁面不变"对照）。
- [ ] **Step 3: rename 纯度检查**——`git diff --find-renames $PASSB_BASE_SHA...HEAD --stat` 确认搬移文件仅 import/调用点改名差异；对 4+2 导出符号 `git diff` 逐个确认无函数体变更。
- [ ] **Step 4: 节点 gates 全量执行（DAG 原样）**——`go build ./...`；`go test -count=1 ./internal/modules/identity/...`；`make check-backend-architecture`；`make verify-module-moves`。四条全绿记入 evidence（含命令原文与退出码）。
- [ ] **Step 5: Commit** — `test(passb): b1-identity differential evidence and gate record`

### Task B1-ID.6：Integration Brief 与实施报告

**Files:** Create `docs/architecture/passb/briefs/b1-identity.md`、`docs/plans/passb/reports/b1-identity.md`。

- [ ] **Step 1: 撰写 Integration Brief**（IB1 集成工程师的唯一执行依据，必须包含以下精确指令）：
  1. **装配直连**：container.go 8 service 构造器 + 7 repository 构造器 + `NewAuthHandler`/`NewAuditLogHandler` + `*service.AuditLogRetentionRunner`（:2453）+ `*repository.MobileExchangeStore`（:751）改为直连 `internal/modules/identity/{service,repository,handler}`；router.go:53,83 与 routes_auth_tenant.go:206,282、routes_knowledge.go 的 `*handler.AuthHandler`/`*handler.AuditLogHandler` 改直连。
  2. **兼容层删除**：直连完成后删除三个 `identity_passb_compat.go` + manifest 3 行 + ownership-matrix 对应新增行（barrier 写权限）。
  3. **跨 owner 调用点重线**（conventions §1.3，集成工程师执行）：sandbox_terminal_ticket.go:52,66,82 → `identitysvc.JWTSecret()/TenantIDFromClaims`；system_setting.go:999,1195 与 kb_activity.go:157,160 → `AuditActor/AuditActorRole`；handler/mcp_oauth.go → `URLQueryEscape`；handler/tenant.go → `ResolveTenantSelfServiceCreationEnabled`。
  4. **8 个推迟件迁移编排**（§3.2 表格逐文件）：先加跨 owner 导出 shim（parseListPagination / storage allowlist 两函数 / sharedKBRow / escapeLikeKeyword / escapeLikePattern 的宿主内导出包装，或等 b2/b4 属主导出），解除 identity 新包不得 import 宿主的循环约束后，按 B1-ID.2 同法迁移（含各自 _test.go 与 manifest 行），rbac_lookups.go 按 §7.4 去方法化裁决处理（router 调用形参同步改）。`forUpdateClause` 随 tenant_member.go 迁移时导出并给 memory_extraction.go（31-agentruntime）留 shim。
  5. **矩阵/契约回写**：27 行 `plan: 10-identity` 状态收口；contracts.yaml identity 12 行 consumers 路径更新（旧宿主路径 → 新模块路径）；`check-passb-readiness` 必绿。
  6. **semanticScopeGuard 收口登记**：转告 b2/k2——identity 侧 seam 待 barrier 统一收口为单一实现（conventions §7.1）。
- [ ] **Step 2: 撰写实施报告** `docs/plans/passb/reports/b1-identity.md`：conventions §1.2 全部命令原文+退出码+摘录；变更文件 vs owned_files 逐条核对（`git diff --name-only $PASSB_BASE_SHA...HEAD | sort` 求差集，差集必须只含 §4.2/§4.3 文件）；推迟 8 文件与 semanticScopeGuard 偏差如实登记；未完成项零省略。
- [ ] **Step 3: Commit** — `docs(passb): b1-identity integration brief and implementation report`

## 8. 集成与回滚边界

- **集成**：本节点产物经 review 后由集成工程师以 `merge: passb b1-identity` 一次一支合入；IB1 只从 `docs/architecture/passb/briefs/b1-identity.md` 切换共享装配（framework:103）。IB1 顺序 B1-ID → B1-AI → B1-CM → B1-EX（framework:101）。
- **回滚**（Spec §13）：本节点只有 M2（rename）/M3（compat/导出改名）两类提交，无 M4——回滚 = revert 对应任务 commit 即恢复原状；三个 compat 文件与新包目录删除即完全回退，无数据修复、无 schema 影响。差分失败时只许修新实现（framework:31），禁止改期望值。
- **停止条件**：出现宿主包循环 import、guards 红且无法在本节点文件内修复、或发现第二写入者 → 停止并按 conventions §5 在报告中登记 blocked，等待协调者裁定。

## 9. 必须删除的 legacy / alias / 例外

| 项 | 现状 | 删除义务 |
|---|---|---|
| identity alias 义务 | 0 条（manifest alias_obligations 空） | 无 |
| identity import 例外 | 0 条（exception-ledger 无 `10-identity`） | 无；**禁止新增** |
| manifest legacy 行（19 已搬） | 本节点同 commit 删除 | 完成 |
| 3 个 identity_passb_compat.go | 本节点新建并登记 | IB1 按 Brief 删除（Brief 指令 #2） |
| 8 个推迟件 legacy 行 | 保留 | IB1 迁移后由 barrier 收口（Brief 指令 #4/#5） |
| `internal/modules/identity/legacy/README.md` | 镜像更新 | 全部迁空后随 B5 删除目录 |

## 10. 独立验收标准（Reviewer/Verifier 逐条勾验）

1. `git diff --name-only $PASSB_BASE_SHA...HEAD` 与 §4 清单完全一致（无越权文件）。
2. 四条节点 gates 原样执行全绿（命令与退出码在 evidence 中）。
3. `git diff --find-renames` 显示 19 生产文件 + 28 测试为 rename；除 import 重线、6 个导出改名及其调用点、2 个外部测试 import 重线、1 个测试拆分外无函数体变更。
4. T0/搬后三个宿主包 + 模块包测试结果集合一致（差分章节有逐包对照）。
5. 容器装饰器类型断言路径成立（`go test ./internal/container/ -count=1` 与基线一致，含 retrieve_registry_wiring_test.go）。
6. contracts.yaml identity 12 行符号零漂移（`internal/types/interfaces/**` 未被触碰；passbguard 符号面在 IB1 复核）。
7. `make verify-module-moves` 证明 manifest 行与磁盘一致（19 行已删、3 compat 行在册）；`make check-backend-architecture` 证明 legacy-guard/forbidden-import/注册唯一性零诊断。
8. 计数基线不变：633/23+23/58/537（本节点未触注册点；IB1 复核三方一致）。
9. Brief 覆盖 §7 B1-ID.6 Step 1 的全部 6 项指令；报告零未申报偏差（semanticScopeGuard seam 与 8 推迟件已申报）。
10. `internal/modules/identity/module.go` 零改动（门面形态与 contracts.yaml `identity.facade` 冻结值一致）。

## Plan Self-Review Record

- **Spec 覆盖**：§5.1 Identity 职责→27 文件归属；§4.4 纯移动+薄别名+记录删除阶段→compat 层设计；§12 交付包→B1-ID.6；§13 提交隔离→每任务一 commit；§14 差分→B1-ID.5；§17.2 对应 B5 终态（本节点不声明完成）。
- **无发明接口**：§5 全部签名抄自现仓库代码（行号在列）；导出名=现名首字母大写；seam 委托端口为冻结导出接口。
- **耦合处理合规**：反向=导出端口+shim（DAG 节点裁定原文）；正向=推迟+Integration Brief（conventions §1.3/§7.2，物理循环论证在 §3.2）；未复制任何 knowledge/policy/platform 实现——唯一形似复制是 §3.3 seam，已按 §5 升级精神显式登记待审。
- **门禁可达性**：全部 gates 只依赖本节点可写文件（modulemove/legacy-guard 的登记义务已在 B1-ID.2/3/4 Step 内闭环；§1.5 有实测依据）。
- **已知残留风险**：(a) 8 推迟件使 IB1 工作量前移——Brief 指令 #4 已逐文件展开；(b) semanticScopeGuard seam 属审查敏感项——若审查裁定必须重裁所有权，走 conventions §5 升级而非现场改判；(c) 混合测试拆分（tenant_self_service_policy_test.go）若发现遗漏 helper，按"随被测函数走"原则就近归置并在报告登记。
