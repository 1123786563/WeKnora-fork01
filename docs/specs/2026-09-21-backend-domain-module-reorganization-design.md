# 全后端业务模块化改造设计

> 状态：2026-09-21 全量设计已确认，待书面审阅与实施计划。本 Spec 是覆盖服务端全部功能的架构总蓝图；每个业务模块仍须拆成独立纵向子项目实施。

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

下表是当前代码族的目标归属。它用于保证所有服务端能力进入改造范围；Wave 0 必须把代码级清单逐项对照到此矩阵，并对任何遗漏或一对多归属提交设计修订。

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

## 9. 全量迁移路线

这是架构项目群。每个模块形成独立纵向子项目，不能用一个超大 PR 完成。

### Wave 0：基线与守护

- 建立路由、Worker、定时任务、启动钩子、表和 migration 全量清单；
- 记录当前所有者和目标模块；
- 建立 README、port、event 和 Task Brief 模板；
- 建立非法导入检查、契约特征测试和既有失败台账；
- 记录 container/router 基线。

### Wave 1：证明方法可行

- 完成 Conversation/Query History 试点；
- 定稿 Module、RouteRegistrar、WorkerRegistry 和 lifecycle 接口；
- 建立 System/bootstrap 骨架，只分离装配职责；
- 复盘试点，确认模板后再扩展。

### Wave 2：底层边界

- Platform：DB、Queue、File、Cache、Observability、Runtime；
- Identity：Tenant、主体、成员、RBAC 与审计；
- AI Resource：模型、MCP、搜索、向量库和存储配置；
- Commercial：权益、预算、计量和支付；
- Policy：组合公开事实执行门控。

### Wave 3：核心能力

- Knowledge：先拆接入、处理、FAQ/Wiki、检索等内部子模块；
- Agent Catalog：统一定义、版本、Persona、Expert、Subagent、Skill 和市场；
- Execution：统一 Sandbox、Target、Workspace、Terminal 和 Browser 门面。

### Wave 4：外部集成

- Data Source：同步状态机及 Knowledge 摄取端口；
- App Connector：安装、授权、连接、动作和同步；
- Channels：IM/Embed 标准化输入输出并隔离 SDK。

### Wave 5：执行编排

- Agent Runtime：Run、Tool、Approval、Memory、Recovery；
- Conversation 剩余能力：Session、Message、Chat、Feedback、Share；
- Insights：Analytics/Evaluation 转为事件和投影消费者。

### Wave 6：用户工作聚合与收尾

- Workbench：Task、Timeline、Inbox、Artifact、Notification；
- Craft：在 Execution/Knowledge/Commercial 稳定后收敛开发能力；
- 删除已迁移的全局 handler/service/repository/types 路径；
- 将 container/router 收敛为纯组合根；
- 完成全仓依赖和所有权审计。

## 10. 单模块迁移循环

1. 盘点入口、规则、表、Worker、外部契约和测试；
2. 明确 Task Brief、所有权、公开接口、事件和非目标；
3. RED：增加特征测试和边界守护测试；
4. GREEN：迁移 domain、application、ports、adapters 和 transport；
5. 切换路由、Worker、定时任务和生命周期装配；
6. 同批删除旧注册、重复类型和失去职责的代码；
7. 运行目标测试、全量验证、独立 Review 和 diff 审查；
8. 更新清单、README、ADR/Spec 和迁移台账。

## 11. 并行规则

仅当接口/事件已冻结、所有权无争议、写文件/migration/装配点不重叠时并行。每条实现流使用独立 worktree；主集成流串行修改 bootstrap/container/router；Knowledge、Agent Runtime、Conversation、Craft 不同时修改同一核心 seam。共享 `git stash` 不作为协调机制。

## 12. 测试与验证门禁

每批必须覆盖：

1. 外部契约：API、错误、流、事件、任务名、CSV/文件格式；
2. 安全：Tenant 隔离、RBAC、凭据、审批和 capability gate；
3. 数据：表/migration 兼容、单写者、事务、幂等和回滚；
4. 运行：启动、关闭、Worker、重试、清理、降级和资源释放；
5. domain/application 单元测试；
6. SQLite/PostgreSQL 适用范围内的 adapter 测试；
7. transport 与装配测试；
8. 目标包测试、`go test ./internal/...`、`go build ./...` 和变更范围 lint；
9. Spec compliance、架构、安全和最终 diff 独立 Review。

既有失败在 Wave 0 建立可复现台账。不得删除或放宽测试制造绿色；无法区分既有失败与回归时停止迁移。

## 13. 模块治理规则

- README 记录职责、非职责、公开入口、依赖端口、数据、事件和契约；
- 新功能先确定业务所有者，无法确定时先解决领域歧义；
- `platform/common/utils` 不作业务兜底目录；
- 超大文件只作为职责复核信号；
- 模块 API 最小化，不为测试方便导出内部类型；
- migration 保持全局有序，并标明所有者模块；
- 跨模块报表使用明确 projection，不允许任意 join 成为隐式依赖；
- 迁移能力时同步迁移测试。

## 14. 风险与停止条件

- **名义模块化**：只移动文件但继续跨模块调用内部 service；由导入检查、ports 和单写者门禁阻止。
- **双重注册/双写**：切换必须原子化，由装配测试和单写者测试证明。
- **循环依赖**：重新确定所有权，或用结果事件/回调端口打断；不得用全局变量解决。
- **范围失控**：外部行为、授权、数据语义或部署拓扑变化必须另立 Spec/Ticket。
- **长期双结构**：每个子项目包含旧路径删除条件和迁移台账。

与批准 Spec/ADR 冲突、需要改变外部语义、出现双写者/不可消除循环、基线无法区分回归、单模块仍大到无法理解时，停止当前批次并升级设计。

## 15. 完成标准

### 15.1 首阶段

- Wave 0 清单、契约基线、失败台账和依赖守护完成；
- Query History 试点进入目标结构；
- bootstrap 只引用试点模块入口；
- 既有 API、权限、隐私、CSV、数据库和任务语义全部通过兼容测试；
- 试点复盘确认模板可用于后续模块。

### 15.2 全量

- 16 个模块均有所有者、README、公开接口、事件和测试；
- 所有路由、Worker、定时任务、钩子、表和 migration 有唯一归属；
- 一份业务事实只有一个写入所有者；
- container/router 只组合模块，不引用内部实现；
- 非法跨模块导入由 CI 拒绝；
- 全局旧技术层不再承载已迁移业务，不保留永久兼容壳；
- Workbench/Insights 投影可从权威事实重建；
- 全量测试、构建、lint、验收、Review 和 diff 均有证据；
- `cmd/desktop`、`docreader`、`client`/SDK 未被意外改造。

本 Spec 批准的是全后端目标架构与迁移项目群，不等于批准一次性实现。后续先为 Wave 0 与 Wave 1 编写实施计划，再逐模块形成独立 Task Brief 和执行计划。
