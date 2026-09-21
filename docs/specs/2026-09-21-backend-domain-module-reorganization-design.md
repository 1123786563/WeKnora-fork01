# 全后端两遍式业务模块化迁移设计

> 状态：2026-09-21 两遍迁移、任务级并行和持续验证设计已确认，待书面审阅与新版实施计划。本 Spec 覆盖服务端全部功能；每个业务模块仍须拆成独立纵向子项目实施。

## 1. 背景

当前服务端按技术层组织：入口位于 `internal/handler` 和 `internal/router`，业务编排位于 `internal/application/service`，数据访问位于 `internal/application/repository`，模型和接口集中在 `internal/types`，依赖由 `internal/container` 统一装配。

该结构已经超出舒适范围：`internal/application` 约 943 个 Go 文件、29 万行，`internal/handler` 约 285 个文件、8.5 万行，`internal/agent` 约 294 个文件、7.2 万行；`container.go` 超过 2,500 行并了解大量业务细节；多个核心业务文件达到 2,000–4,000 行；服务端还有约 60 组 HTTP 路由以及大量 Worker、定时任务和启动钩子。

核心问题不是单纯文件过大，而是目录无法回答“功能在哪里、由谁拥有、可以依赖谁”。继续把新功能加入全局技术层会扩大隐式耦合、循环依赖和装配风险。

## 2. 范围

### 2.1 范围内

- `cmd/server` 启动的 HTTP 服务和后台 Worker；
- 根 Go 模块 `internal/` 下全部业务、Agent 运行、执行环境、集成、商业治理和基础设施代码；
- 路由、Worker、定时任务、生命周期钩子、数据模型和 migration 的所有权整理；
- 单体内部的模块边界、装配方式、依赖规则与迁移策略；
- 旧目录中业务代码迁入新模块后的清理。

### 2.2 范围外

- `cmd/desktop`、`docreader`、`client` Go SDK 及其他独立 SDK/契约模块；
- 拆分微服务或新建独立部署单元；
- 更换 Gin、GORM、dig、asynq 或数据库；
- 在目录迁移中重新设计产品行为；
- 一次性移动整个后端；
- 仅按文件行数机械拆分代码。

## 3. 目标与成功标准

目标是把服务端整理为按业务能力组织的模块化单体。开发者应能从业务名称进入一个模块，并在该目录内找到入口、用例、业务规则、数据适配、Worker 和测试。

1. 每项功能、路由、Worker 和业务表都有唯一所有者模块；
2. 顶层目录首先表达业务功能，而不是框架或技术层；
3. 每个模块只暴露少量稳定入口，其他模块不能引用其内部实现；
4. 一份业务事实只有一个写入所有者，跨模块不共享 repository 或直接写表；
5. 总路由和依赖容器只组合模块，不了解模块内部 handler/service/repository；
6. 外部 API、权限、数据、异步任务和运行行为在兼容迁移中保持稳定；
7. 新功能不再默认堆入全局 `handler`、`service`、`repository`、`types`、`common` 或 `utils`；
8. 非法跨模块导入由自动化检查拒绝。

## 4. 总体架构

```text
internal/
  bootstrap/                 # 进程组合根与生命周期
  modules/
    identity/                # 认证、Tenant、成员、RBAC
    knowledge/               # 知识接入、处理、检索
    conversation/            # Session、Message、Chat、审计
    agentcatalog/            # Agent/Skill/市场定义与版本
    agentruntime/            # Agent 执行、工具、记忆与恢复
    workbench/               # Task、Timeline、Inbox、Artifact
    craft/                   # 开发工作区、运行与制品
    execution/               # Sandbox、Target、Terminal、Browser
    datasource/              # 外部知识源与同步
    appconnector/            # 应用安装、授权与动作
    channels/                # IM、Embed 与外部身份映射
    airesource/              # Model、MCP、Search、Vector、Storage 配置
    commercial/              # 套餐、权益、预算、计量与支付
    insights/                # Analytics、Evaluation 与投影
    system/                  # 初始化、设置、部署能力与维护
    policy/                  # 跨域门控执行
  platform/
    http/ persistence/ queue/ files/ observability/ runtime/
```

### 4.1 模块内部结构

```text
internal/modules/<module>/
  module.go                  # 构造、路由、Worker 和生命周期入口
  README.md                  # 职责、非职责、公开接口、数据与依赖
  transport/http/            # 协议入口；trpc 等按需增加
  application/               # 用例、事务与流程编排
  domain/                    # 模块拥有的规则、状态和值对象
  ports/                     # 模块需要的外部能力接口
  adapters/                  # DB、队列、文件、外部 SDK 和旧代码适配
```

小模块不创建空目录。结构一致性来自职责和依赖规则，而不是形式相同。

### 4.2 依赖方向

```text
transport -> application -> domain
                     |
                     v
                   ports <- adapters -> platform
```

- domain 不依赖 Gin、GORM、Redis、asynq、外部 SDK 或其他模块实现；
- application 只依赖本模块 domain、ports 和其他模块公开接口；
- transport 只处理协议、认证上下文、输入输出和错误映射；
- adapters 可调用 Platform 或迁移期旧代码；
- 接口优先定义在使用方模块，不建立新的全局 interfaces 仓库；
- 回流优先使用版本化事件或明确回调端口；
- 禁止用全局注册表、service locator、复制模型或 `common/utils` 隐藏循环依赖。

### 4.3 模块装配

模块按需暴露 `NewModule`、`RegisterRoutes`、`RegisterWorkers`、`Start` 和 `Stop`。具体 Go 签名由实施计划根据现有 RBAC、router、worker 和 ResourceCleaner seam 确定。

bootstrap 只引用模块入口；同一路由、Worker 和钩子只注册一次；停止顺序必须可验证；不得以包级可变状态绕过依赖注入。

### 4.4 两遍迁移模型

完整迁移分为两个可独立验收、系统始终可运行的阶段。

**Pass A — Move First（功能归位）**：先让所有生产代码和测试按业务功能集中，快速获得可导航结构；保留模块内部原有 handler/service/repository 组织，不在移动提交中改变业务逻辑。

```text
internal/modules/knowledge/
  README.md
  module.go
  legacy/
    handler/
    service/
    repository/
    types/
```

Pass A 只允许 `git mv`、package/import 修复、模块装配接线和必要的无逻辑兼容别名。禁止修改 API、SQL、状态机、权限、错误、重试、schema 或业务规则。新功能从 Pass A 起不得继续进入旧横向业务目录。

**Pass B — Refactor Second（边界收紧）**：在代码归位后，逐模块把 legacy 拆成 transport/application/domain/ports/adapters，建立单写者、公开端口和事件回流，最后删除 legacy、兼容别名和临时导入例外。

单模块轨迹固定为：

```text
基线 → 纯移动 → package/import 修复 → module.go 接线
    → domain/ports/adapters 整理 → legacy/例外删除
```

不复制第二份业务实现。确需过渡时只能保留无业务逻辑的薄别名或 adapter，并记录删除阶段。

## 5. 模块地图与所有权

### 5.1 Identity

负责认证、用户、Tenant、成员、邀请、API Key、Organization、RBAC 和治理审计。拥有 User、Tenant、TenantMember、Invitation、TenantAPIKey、Organization、授权事实和审计日志。公开主体解析、成员资格、授权决定和租户配置读口。其他模块不得直接查询成员表自行推导权限。

### 5.2 Knowledge

负责知识库、文档、Chunk/Revision、Tag、FAQ、Wiki、导入处理、语义索引、GraphRAG 和检索规则。向 Conversation、Agent Runtime、Craft 和 Data Source 提供知识访问、摄取、检索、引用和删除生命周期。向量数据库驱动由 AI Resource/Platform 适配，检索规则仍归 Knowledge。

### 5.3 Conversation

负责 Session、Message、Chat、Feedback、分享、建议、Query History、会话附件和临时文档。拥有相应写模型、分享令牌和导出任务。公开创建/继续会话、追加消息、审计读取、分享和会话事件。Memory 抽取规则归 Agent Runtime。

### 5.4 Agent Catalog

负责 Agent Definition/Version、Persona、Expert、Subagent、Skill、Marketplace、Favorite 和发布/引入治理。拥有 Release/Listing/Adoption/Variant、安装和发布记录。公开可用 Agent 解析、版本固定、能力声明、目录和安装治理；不拥有运行状态。

### 5.5 Agent Runtime

负责 Agent 执行引擎、Run/Attempt、工具、审批、Compaction、Memory、Model Context、Native/tRPC/OpenCode 协议和恢复。拥有运行上下文、工具状态、审批事实、Memory 和恢复状态。通过端口调用 Catalog、Knowledge、AI Resource、Execution、Commercial、Policy 和 App Connector。

### 5.6 Workbench

负责 Task 视图、Timeline、Inbox/Attention、通用 Artifact、通知和移动任务投影。拥有可重建的任务/时间线投影、通知投递状态和通用 Artifact 元数据；不接管 Conversation、Agent Run 或 Craft Run 的源记录。

### 5.7 Craft

负责开发工作区、会话/运行、Interaction、Snapshot、Artifact Version/Preview、用量视图、生命周期和 Scheduled Task。通过 Execution 获取环境，通过 Knowledge 获取上下文，通过 Commercial 做预算与计量，并向 Workbench 发布任务事件。

### 5.8 Execution

负责 Sandbox、Execution Target/Registration、Workspace Lease、Terminal、Browser Skill、环境变量和资源传输。拥有执行目标、注册、租约、Sandbox Config、终端票据、用户环境变量和浏览器会话。向 Agent Runtime 和 Craft 隐藏 Docker、E2B、Cube 和本地实现。

### 5.9 Data Source

负责外部知识源、连接器注册、同步、取消、重试、进度、凭据元数据和调度。拥有 DataSource、SyncLog/ItemError、cursor 和删除清理状态。抓取外部内容后调用 Knowledge 摄取接口，不直接写 Knowledge 表。

### 5.10 App Connector

负责应用目录、安装、OAuth、连接、授权、动作和同步，包括 Open Connector。拥有 AppVersion、Installation、Connection、OAuthBinding、Grant、Action/Approval 和回执。公开安装、连接、授权和动作接口。

### 5.11 Channels

负责 IM、Webhook、Embed Channel、外部身份/会话映射、入站标准化和出站投递。拥有 Channel、Binding、VisitorSession 和 Webhook 状态。经过验签与身份映射后调用 Conversation，不复制 Chat/Session 逻辑。

### 5.12 AI Resource

负责模型、Provider 凭据、MCP、Web Search、Vector Store、Storage Backend 和租户能力配置。公开按 Tenant 解析获准模型、工具、搜索、向量库和存储能力；底层密钥加密由 Platform 提供。

### 5.13 Commercial

负责套餐、权益、预算、Usage、账单、支付、履约和商业网关。拥有 BillingAccount、Plan/Version、Subscription、Entitlement、Budget/Reservation、UsageFact、Invoice/Order、PaymentFact。公开准入、预占、结算、计量、付款和履约接口。

### 5.14 Insights

负责 Analytics、Evaluation、运营指标、报表和可重建分析投影。拥有评测定义/运行和投影，不拥有 Feedback、Session、Usage 等源记录，只消费事件或受控读模型。

### 5.15 System

负责初始化、System Setting、部署能力、系统管理、健康信息和 housekeeping。拥有系统设置、初始化状态、能力描述和维护任务状态。System 不是业务 service facade。

### 5.16 Policy

负责统一执行跨域配额、限流、存储白名单和 capability gate。拥有评估器和短期执行状态，不接管业务表或业务规则。可组合 Identity、Commercial、System 的公开事实返回决定，但不写调用方业务状态。

### 5.17 Platform

Platform 不是业务模块。它拥有连接、migration runner、队列、blob、缓存/锁、加密封装、日志、指标、Tracing、配置和进程资源；不拥有 Tenant、Session、Agent、Knowledge、Task 或 Subscription，也不根据业务字段作授权决定。

### 5.18 现有功能覆盖矩阵

下表是当前代码族的目标归属。它用于保证所有服务端能力进入改造范围；F0 必须把代码级清单逐项对照到此矩阵，并对任何遗漏或一对多归属提交设计修订。

| 现有功能或代码族 | 目标所有者 | 边界说明 |
|---|---|---|
| auth、user、tenant、member、invitation、API key、organization、RBAC、audit log | Identity | HTTP 中间件是 Platform；授权事实与规则归 Identity |
| knowledgebase、knowledge、chunk、tag、FAQ、Wiki、graph、retriever、semantic knowledge | Knowledge | 向量驱动配置归 AI Resource，检索策略归 Knowledge |
| session、message、chat pipeline、feedback、share、suggestion、query history、temporary document | Conversation | Agent 执行调用 Agent Runtime；不在 Conversation 内复制运行引擎 |
| custom agent、agent version、persona、expert、subagent catalog、skill、marketplace、favorite | Agent Catalog | 运行态、审批和工具调用归 Agent Runtime |
| `internal/agent` engine/runtime/tools/native/trpc/compaction、memory、model context、运行审批 | Agent Runtime | Agent 定义读取 Agent Catalog；MCP 配置读取 AI Resource |
| workbench routes/services、task/timeline 投影、inbox、artifact 汇总、notification、mobile voice/device | Workbench | 通用任务投影归 Workbench；Craft 专用源记录归 Craft |
| craft workspace/session/run/interaction/snapshot/artifact preview/version/usage/scheduled task | Craft | Sandbox 和 Target 归 Execution；预算与计量归 Commercial |
| sandbox、execution target/registration、workspace lease、terminal、browser skill、env var、resource transport | Execution | Docker/E2B/Cube 等驱动放在 adapters；通用 blob 存储归 Platform |
| datasource service、connector registry、各知识源 connector、sync scheduler/log/retry/credential metadata | Data Source | 同步结果通过 Knowledge 摄取端口进入知识域 |
| app connector installation/connection/OAuth/action/sync、Open Connector adapter/control | App Connector | Agent 工具只调用公开动作接口，不读取连接表 |
| IM adapters、channel、embed、webhook、外部身份和会话绑定 | Channels | 标准化后调用 Identity 与 Conversation |
| model/provider/credentials、MCP service/OAuth metadata、web search provider、vector store、storage backend | AI Resource | MCP 运行工具位于 Agent Runtime；资源定义与凭据归 AI Resource |
| commercial domain/service/repository、payment、usage、benefit、budget、billing gateway | Commercial | 各模块只发布用量事实或调用准入/结算接口 |
| analytics、evaluation、metric/report/export projections | Insights | 源记录仍由产生事实的业务模块拥有 |
| initialization、system setting、deployment capability、health、housekeeping | System | config 读取和进程启动机制归 Platform/bootstrap |
| rate limit、storage allowlist、embed/access policy、跨域 capability gate | Policy | 具体业务规则仍由业务所有者定义，Policy 负责一致执行 |
| router 基座、middleware、container、database、config、logger、tracing、stream、queue、file service、cache/lock | Platform / Bootstrap | router/container 最终只装配模块；不得继续承载业务规则 |
| common、utils、types、interfaces、event | 按所有权拆分 | 业务类型/接口/事件移入所有者或使用者模块；纯技术原语才能留在 Platform |

同一现有目录可以被拆给多个目标模块，但同一个业务概念不能有多个写入所有者。特别是 `types`、`interfaces`、`common`、`utils` 和 `event` 不作为目标业务模块保留；其内容必须按语义逐项归属。

## 6. 跨模块协作与主数据流

允许同步公开用例、使用方定义的窄端口、版本化领域事件和明确用途的只读投影。禁止直接导入其他模块 repository、跨模块更新表、共享可变 domain struct、从下游同步反调上游。

### 6.1 Chat / Agent

```text
Conversation -> Policy + Commercial -> Agent Runtime
Agent Runtime -> Agent Catalog + Knowledge + AI Resource + Execution + App Connector
Agent Runtime --events--> Conversation + Workbench + Commercial + Insights
```

Conversation 先持久化输入和 turn 边界；计量、任务投影和分析消费结果事件，不进入核心执行事务。

### 6.2 知识摄取

```text
Data Source -> Knowledge -> AI Resource(Vector/Embedding)
Knowledge --events--> Workbench + Insights
```

Data Source 拥有同步状态，Knowledge 拥有文档与索引状态，双方不能直接更新对方表。

### 6.3 Craft

```text
Workbench -> Craft -> Execution + Knowledge + Commercial
Craft --events--> Workbench + Insights
```

Workbench 是任务聚合入口，Craft 是开发工作权威。投影失败不回滚 Craft 状态。

### 6.4 IM / Embed

```text
Channels -> Identity -> Conversation -> Agent Runtime
Agent Runtime --result event--> Channels
```

Channels 只负责协议、验签、身份映射和投递；统一会话行为仍由 Conversation 提供。

### 6.5 事件契约

- 包含稳定名称、schema 版本、Tenant、Actor、时间和幂等键；
- 描述已发生事实，不伪装成隐藏同步命令；
- 源事务通过 outbox 或等价机制保证事实与事件一致；
- 消费失败不回滚源事务，采用幂等消费、重试和死信/告警；
- Workbench 与 Insights 投影可从权威事件重建。

## 7. 错误处理

domain/application 返回稳定领域错误，不构造 Gin 响应。transport 负责映射现有 HTTP/tRPC 状态与 AppError、记录安全字段、隐藏基础设施细节并保持跨租户防枚举。事件消费者区分可重试、永久失败和幂等重复。

迁移期间错误码、状态码和流式终止语义均是外部契约，不因目录整理改变。

## 8. 首个试点：Conversation / Query History

Query History 当前跨 router、session handler、service、repository、types、container 和 worker router，具备完整路由、权限、隐私策略、异步导出、持久化和测试链路，作为首个纵向切片。

```text
internal/modules/conversation/queryhistory/
  transport/http/
  application/
  domain/
  ports/
  adapters/
```

它拥有审计访问策略、匿名化、快照、导出任务和 CSV Worker；Session/Message/Feedback 写模型仍由 Conversation 上层拥有。通过 `AuditReader`、`PolicyReader`、`ExportJobStore`、`FileStore`、`TaskQueue` 等窄端口协作。

试点不得改变现有 Admin 路由、RBAC/API Key、403/404/400/500 映射、匿名化、CSV 列序/BOM/文件名、数据库表或 asynq 重试/幂等语义。

## 9. 并行 Code Agent 组织

完整迁移使用最多七个并发角色，但只允许四条模块写入流：

| 角色 | 数量 | 权限与职责 |
|---|---:|---|
| Coordinator / Integrator | 1 | 冻结接口、分配所有权、串行修改共享装配点、合并 Worktree、处理冲突 |
| Module Implementer | 4 | 每个 Agent 独占一个模块或子域 Worktree，移动代码与测试、修复 import、提交 `module.go` 和迁移报告 |
| Reviewer | 1 | 只读审查纯移动、Spec、边界、安全和 diff，不写生产代码 |
| Verifier | 1 | 运行测试、构建、lint、架构守护和差分门禁，维护基线证据 |

模块 Agent 可以修改自己的 `internal/modules/<module>`、随模块移动的测试、README、迁移台账和 Integration Brief。只有 Integrator 可以修改：

- `internal/router/router.go`；
- `internal/container/container.go`；
- 全局 Worker/生命周期注册；
- migration 编号和共享 schema 入口；
- `go.mod`、`go.sum`；
- 跨模块公共接口、事件契约和所有权清单。

每条实现流使用独立 Git Worktree。共享 `git stash` 不作为协调状态。子 Agent 上下文只包含 Spec、当前 Task Brief、所有权文件和前置接口，不复制整个父会话。

任务只有在接口/事件已冻结、`OWNED_FILES` 清单不相交、没有同一业务表或 migration 的并发写入、并且目标测试不依赖另一未合并分支时才允许并行；任一条件不满足即改为串行。

## 10. Pass A 任务级并行与串行图

### 10.1 Foundation：F0 → F1 → F2 必须串行

- **F0 基线与全量清单**：测试基线、功能/路由/Worker/表/migration 所有权；
- **F1 模块模板**：README、`module.go`、legacy、MOVE-MANIFEST 和 Integration Brief 格式；
- **F2 装配契约与 CI 守护**：Route/Worker/Lifecycle contract、非法导入和唯一注册检查。

三项必须串行，因为所有模块 Agent 都依赖同一套目录、接口、基线和所有权规则。

### 10.2 Parallel Group A1：四项可以并行

- **A1 App Connector 归位**；
- **A2 Commercial 归位**；
- **A3 Data Source 归位**；
- **A4 Channels/IM/Embed 归位**。

四者文件和主要业务状态不重叠。模块 Agent 不修改共享 router/container。

### 10.3 IA1 Integration Barrier：必须串行

Integrator 逐个接入 A1–A4 的 `module.go`，每接一个模块运行装配测试，最后运行批次回归。未通过 IA1 不启动 Parallel Group A2。

### 10.4 Parallel Group A2：四项可以并行

- **A5 Execution/Sandbox 归位**；
- **A6 AI Resource 归位**；
- **A7 Agent Catalog 归位**；
- **A8 System + Policy 归位**。

### 10.5 IA2 Integration Barrier：必须串行

Integrator 接线并冻结 Execution、AI Resource、Agent Catalog、Policy 的公开入口，验证 A1/A2 两批不存在循环 import。

### 10.6 Parallel Group A3：可以并行移动，必须串行接入

- **A9 Knowledge 归位**：按 ingest、process、wiki/faq、retrieval 四个子清单；
- **A10 Conversation 归位**：Session、Message、Feedback、Query History；
- **A11 Agent Runtime 归位**：Engine、Tool、Memory、Native/tRPC、Model Context。

A9–A11 可以同时做纯移动，但 A11 不得自行改变 Knowledge/Conversation 接口。IA3 的生产接入顺序固定为 **Knowledge → Conversation → Agent Runtime**，每一步构建和测试通过后才接下一项。

### 10.7 Parallel Group A4：三项可以并行

- **A12 Workbench 归位**；
- **A13 Craft 归位**；
- **A14 Insights 归位**。

前置条件是 IA3 完成、Conversation/Agent Runtime 入口稳定。A12/A13 在开工前冻结共享 Task/Artifact 契约。

### 10.8 IA4 Pass A 完成屏障：必须串行

Integrator 串行接入 A12–A14，执行全量构建/测试、旧横向目录残留扫描、路由/Worker 唯一注册检查和 Pass A 验收。

## 11. Pass B 任务级并行与串行图

- **B0 必须串行**：冻结 Identity、Capability、Usage、Execution、Knowledge、Conversation 和 Task/Artifact 公共端口与事件版本；
- **B1 可以并行**：Identity、AI Resource、Commercial、Execution；随后 **IB1 串行集成**；
- **B2 可以并行**：Knowledge、Agent Catalog、Data Source、App Connector；随后 **IB2 串行集成**；
- **B3 可以并行**：Agent Runtime、Channels、Insights；Conversation 可在 Agent Runtime 调用接口冻结后并行；随后 **IB3 串行集成**；
- **B4 可以并行**：Workbench 与 Craft 在 Task/Artifact 契约冻结后分别整理；System/Policy 可处理不重叠的治理代码；
- **B5 必须串行**：删除 legacy、compat alias 和临时例外，收敛 router/container，运行全仓回归和最终 Review。

任何阶段中，公共接口/事件变更、router/container 接线、migration 编号、共享表变更、跨模块 legacy 删除、批次全量测试和最终合并必须串行。

## 12. 单模块交付包与迁移循环

每个 Module Agent 必须交付：

1. 范围内代码提交；
2. MOVE-MANIFEST：旧路径、新路径、文件所有者和临时例外；
3. Integration Brief：路由、provider、Worker、生命周期和配置接线说明；
4. Test Evidence：迁移前后命令、结果和基线比较；
5. Review Report：纯移动、行为兼容、安全与 diff 结论。

Pass A 循环：目标测试基线 → `git mv` 生产代码和测试 → package/import 修复 → 目标/依赖测试 → `module.go` → Review → Integrator 接线。

Pass B 循环：特征测试 → domain/application/ports/adapters → 高风险差分 → 装配切换 → legacy 删除 → 模块/消费者/全量测试 → 独立 Review。

## 13. 提交隔离和回滚

提交按目的隔离：

- **M1 Baseline**：只增加测试、清单和证据；
- **M2 Move**：纯 rename，不改函数体；
- **M3 Compile**：只修 package/import 和无逻辑兼容别名；
- **M4 Integrate**：只切换 router/container/Worker/lifecycle 装配；
- **M5 Refactor**：整理模块边界，仍不改变外部行为；
- 业务修复必须另立 Ticket，不混入迁移提交。

切换失败时先回退 M4；M2/M3 的新路径可以保留为未接线代码。Pass A 不修改 schema，回滚不需要数据修复。Pass B 的 schema 任务必须单独设计向前兼容 migration、回填、验证和回滚/roll-forward 策略。

禁止复制并长期保留两份实现或双写。兼容别名只能委托一个真源，必须标注删除批次。

## 14. 持续测试与差分门禁

### 14.1 模块级测试梯度

每个 Agent 执行：

1. **T0 迁移前**：记录基线 SHA、目标测试和结果；
2. **T1 纯移动后**：运行目标包测试和编译；
3. **T2 Import 修复后**：运行直接依赖消费者测试；
4. **T3 module.go 后**：运行装配和唯一注册测试；
5. **T4 Review 前**：运行 diff 范围检查和架构守护。

出现新失败即停止继续移动，定位本批次第一个破坏提交。

### 14.2 纯移动验证

纯移动要求迁移前后同一测试集合结果一致，`git diff --summary` 能识别 rename，除 package/import/接线外无函数体变化，不新增数据库写入、goroutine 或全局状态。纯目录移动不为每个文件建立差分框架。

### 14.3 高风险差分验证

以下行为必须用相同输入运行新旧实现，比较输出、错误、数据库结果、任务参数和副作用：

- Tenant/RBAC、凭据和审批；
- HTTP/流式响应/错误码；
- Worker 状态机、重试和幂等；
- 支付、预算、Usage 和履约；
- Session/Message；
- Knowledge 删除和索引；
- Craft 生命周期与 Artifact。

差分失败只能修正新实现；没有独立批准的行为变更时不得修改期望值迎合新结果。

### 14.4 Integration Barrier 验证

每个 IA/IB 固定执行：本批模块测试 → 直接消费者及 router/container 测试 → 架构守护 → `go build ./...` → `go test ./internal/...` → 变更范围 lint。全部通过后更新批次基线并放行下一组。

既有失败必须在 F0 建立可复现台账。环境缺失或 opt-in 测试 Skip 记录为 `blocked-env`，不能算 PASS；无法区分既有失败与回归时停止集成。

## 15. 模块治理规则

- README 记录职责、非职责、公开入口、依赖端口、拥有数据、事件和临时 legacy；
- 新功能先确定业务所有者，无法确定时先解决领域歧义；
- `platform/common/utils` 不作业务兜底目录；
- 模块 API 最小化，不为测试方便导出内部类型；
- migration 保持全局有序并标明所有者；
- 跨模块报表使用明确 projection，不允许任意 join 形成隐式依赖；
- 架构例外必须是精确路径、说明原因和删除批次，禁止 broad wildcard；
- Pass A 后 CI 拒绝在旧横向业务目录新增生产文件。

## 16. 风险与停止条件

- **移动时顺便重构**：失去失败定位能力；由提交隔离和函数体 diff 检查阻止；
- **共享文件冲突**：并行收益被 merge 消耗；由 Integrator 单写 router/container/migration 阻止；
- **名义模块化永久化**：legacy 不再清理；每个模块必须有 Pass B Task 和删除标准；
- **双注册/双写**：切换必须原子化并由唯一注册/单写者测试证明；
- **循环依赖**：重新确定所有权或用结果事件/回调端口打断，不用全局变量解决；
- **主线持续冲突**：以模块所有权和短生命周期 Worktree 降低；共享 seam 变更由 Coordinator 重新基线。

与批准 Spec/ADR 冲突、需要改变外部语义、出现双写者/不可消除循环、基线无法区分回归、单模块仍大到无法理解时，停止当前批次并升级设计。

## 17. 完成标准

### 17.1 Pass A 完成

- 16 个模块均有所有者、README、`module.go`、代码地图和迁移台账；
- 所有服务端生产文件与测试都有目标模块归属；
- 天然成块代码已经物理归位，剩余 legacy 有精确清单；
- router/container/Worker 按模块注册且不存在重复；
- 新业务代码不再进入旧横向业务目录；
- 所有 Integration Barrier 的构建、测试、架构守护和 Review 不劣于基线；
- `cmd/desktop`、`docreader`、`client`/SDK 未被改造。

Pass A 不声称完成单写者、全部 ports/events 或旧类型拆除。

### 17.2 Pass B / 全量完成

- 16 个模块均完成公开端口、数据所有权和单写者边界；
- 业务规则离开 legacy handler/service/repository；
- 跨模块协作只使用公开 API、使用方 port 或版本化事件；
- 临时 alias、import 例外和 legacy 清零；
- container/router 只组合模块，不引用模块内部实现；
- Workbench/Insights 投影可由权威事实重建；
- 全量测试、构建、lint、差分、验收、Review 和最终 diff 均有证据；
- 所有路由、Worker、钩子、表和 migration 有唯一归属。

## 18. 计划与时间边界

完整项目按 F、A1–A14、IA1–IA4、B0–B5、IB1–IB3 拆成独立 Task Brief。目标周期为 14–16 周，计划区间 12–20 周；时间假设是四条模块实现流持续可用、一个 Integrator 单写共享点、Reviewer/Verifier 不与实现争抢文件。接口返工、基线不稳定或主线高频修改会延长周期。

本 Spec 批准两遍目标架构和任务依赖，不授权一次性大爆炸重构。书面批准后应废止原 Wave 0–1 计划，重新编写 Foundation + Pass A 的详细实施计划；Pass B 在 Pass A 验收和接口复盘后分批规划。
