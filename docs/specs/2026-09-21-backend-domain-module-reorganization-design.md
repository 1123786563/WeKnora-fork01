# 后端业务模块化整理设计

> 状态：2026-09-21 设计已确认，待实施计划。本文只定义后端代码组织、依赖边界和渐进迁移策略，不授权实现。

## 1. 背景

当前后端主要按技术层组织：请求入口位于 `internal/handler` 和 `internal/router`，业务编排位于 `internal/application/service`，数据访问位于 `internal/application/repository`，模型和接口集中在 `internal/types`，全部依赖再由 `internal/container` 统一装配。

这种结构在功能数量较少时直观，但仓库已经超过该组织方式的舒适范围：

- `internal/application/service` 约有 422 个 Go 文件；
- `internal/handler` 约有 181 个 Go 文件；
- `internal/application/repository` 约有 174 个 Go 文件；
- `internal/container/container.go` 超过 2,500 行并集中了解大量业务细节；
- 多个核心业务文件达到 2,000–4,000 行；
- 修改一个业务功能通常需要跨 router、handler、service、repository、types 和 container 搜索。

问题不只是文件过大，而是目录不能回答“某项功能在哪里”。新增功能继续进入全局技术层后，业务边界会进一步模糊。

## 2. 目标与成功标准

目标是把 WeKnora 后端整理为按业务能力组织的模块化单体，使开发者可以从业务名称进入一个模块，并在该目录内找到入口、用例、业务规则、数据适配和测试。

整理成功后应满足：

1. 一个业务能力的主要代码可在一个模块目录内理解；
2. 顶层目录首先表达业务功能，而不是框架或技术层；
3. 模块对外暴露少量稳定入口，其他模块不能直接引用其内部实现；
4. 总路由和依赖容器只负责组合模块，不持续吸收模块内部细节；
5. 迁移过程中现有 API、权限、数据、异步任务和运行行为保持兼容；
6. 新增业务能力不再默认堆入全局 `handler`、`service`、`repository` 或 `types` 目录。

## 3. 非目标

本次整理不包含：

- 拆分微服务或新建独立部署单元；
- 更换 Gin、GORM、dig、asynq 或现有数据库；
- 在目录迁移中重新设计产品行为；
- 一次性移动整个后端；
- 为追求形式统一而复制 Session、Tenant 等共享业务数据；
- 仅根据文件行数机械拆分文件；
- 顺带修复与模块边界无关的历史问题。

## 4. 目标架构

后端保持单一进程和单一部署，通过业务模块表达代码边界：

```text
internal/
  bootstrap/                 # 进程级组合根和生命周期
  platform/                  # 跨业务技术能力
    config/
    database/
    logging/
    queue/
    storage/
  modules/
    queryhistory/            # 首个试点
    knowledge/
    agent/
    tenant/
    connector/
    workbench/
    commercial/
```

`platform` 只承载无业务归属的技术能力。带有产品规则、租户语义或业务状态的代码不得因为被多个模块使用而自动进入 `platform`；应由拥有该概念的模块暴露端口。

### 4.1 模块内部结构

模块使用统一但可按规模裁剪的结构：

```text
internal/modules/<module>/
  module.go                  # 构造、路由和 worker 装配入口
  README.md                  # 职责、入口、依赖、数据、非职责
  transport/
    http/                    # Gin handler、请求和响应 DTO
    trpc/                    # 仅模块确有 tRPC 入口时存在
  application/              # 用例、事务边界和流程编排
  domain/                   # 模块拥有的规则、状态和值对象
  ports/                    # 模块需要的外部能力接口
  adapters/                 # 数据库、队列、存储及旧代码适配
```

小模块不必创建空目录。只有存在相应职责时才增加目录；一致性来自依赖规则和命名语义，而不是空壳结构。

### 4.2 依赖方向

正常调用方向为：

```text
transport -> application -> domain
                     |
                     v
                   ports <- adapters
```

约束如下：

- `domain` 不依赖 Gin、GORM、Redis、asynq 或其他模块实现；
- `application` 依赖本模块的 domain 和 ports，不直接依赖具体基础设施；
- `transport` 负责协议解析、认证上下文提取、错误映射和响应转换，不承载业务规则；
- `adapters` 实现 ports，可调用平台能力或迁移期旧 repository；
- 模块间协作通过公开 application 接口、端口或领域事件完成；
- 一个模块不得导入另一个模块的 transport、adapters 或未公开内部包；
- 接口优先定义在使用方模块，而不是集中放入新的全局 interfaces 目录。

### 4.3 模块装配接口

每个包含入站接口或后台任务的模块通过 `module.go` 暴露最小组合面，概念上包括：

```go
func NewModule(deps Dependencies) (*Module, error)
func (m *Module) RegisterRoutes(router RouteRegistrar, guards Guards)
func (m *Module) RegisterWorkers(registry WorkerRegistry)
```

具体 Go 签名在实施计划中根据现有 router、RBAC 和 worker registry 接口确定，但必须保持以下原则：

- bootstrap/container 只依赖模块入口；
- 模块内部 handler、service 和 repository 不进入全局装配参数；
- 同一路由和 worker 只能注册一次；
- 不使用包级可变全局注册表绕过依赖注入。

## 5. 首个试点：Query History

Query History 具备完整的路由、权限、隐私策略、快照、异步导出、持久化和测试链路，规模适中，适合验证目标结构。它当前分散于：

- `internal/router/routes_query_history.go`；
- `internal/handler/session/query_history_admin.go`；
- `internal/application/service/query_history_policy.go`；
- `internal/application/service/query_history_export.go`；
- `internal/application/service/session.go` 中的审计快照逻辑；
- `internal/application/repository/query_history_export.go`；
- `internal/types/query_history.go`；
- `internal/types/interfaces/query_history_export.go`；
- container 和 worker router 中的注册代码。

### 5.1 试点职责

`internal/modules/queryhistory` 拥有：

- Admin Query History HTTP 路由及其请求/响应转换；
- 查询历史访问策略和匿名化规则；
- 单会话审计快照用例；
- CSV 导出提交、状态、下载和 worker 处理；
- Query History 自己的导出任务、审计快照和策略领域概念；
- 模块所需端口与适配器；
- 模块装配入口和模块说明。

### 5.2 试点不拥有的职责

第一阶段不迁移以下所有权：

- Session、Message 和 Feedback 的写模型仍归现有 Session 能力；
- Tenant 的创建、更新和成员治理仍归 Tenant 能力；
- 通用文件存储和异步队列仍是平台能力；
- Query History 不复制 Session/Tenant repository，也不建立第二套数据模型真源。

Query History 通过窄端口读取所需数据：

- `AuditReader`：读取租户范围内的 session、message 和 feedback 审计视图；
- `PolicyReader`：读取租户 Query History 策略；
- `ExportJobStore`：持久化导出任务并查询聚合导出行；
- `FileStore`：保存和读取 CSV；
- `TaskQueue`：投递异步导出任务。

迁移期适配器允许委托现有 Session/Tenant repository。待其所属模块迁移后，只替换适配器，不改变 Query History application 层。

### 5.3 兼容契约

试点迁移不得改变：

- `/api/v1/admin/sessions/:session_id/snapshot`；
- `/api/v1/admin/sessions/export`；
- `/api/v1/admin/sessions/export/:job_id/status`；
- `/api/v1/admin/sessions/export/:job_id/download`；
- Admin+ 和 API key full-access 权限要求；
- disabled 模式返回 403；
- anonymized 模式遮蔽用户标识；
- 不存在与跨租户导出任务统一返回 404；
- 非法参数返回 400，未知内部失败返回 500；
- CSV 列名、列顺序、UTF-8 BOM 和下载文件名格式；
- 导出任务 `pending -> running -> done|failed` 生命周期；
- asynq 队列、重试上限、超时和完成后的幂等行为；
- 现有数据库表名、字段和迁移历史。

## 6. 错误处理

domain/application 返回稳定的领域错误，不直接构造 Gin 响应。transport 是 HTTP 映射的唯一位置，负责：

- 将领域错误映射为当前状态码与 AppError 响应；
- 记录 tenant、session、job 等可安全记录的结构化字段；
- 对外隐藏基础设施错误细节；
- 保持现有跨租户防枚举语义。

异步 worker 将可重试基础设施失败返回给 asynq；不可恢复的非法 payload 使用现有 skip-retry 语义；任务状态更新失败必须记录，且不能把失败任务误报为完成。

## 7. 渐进迁移策略

### 批次 1：建立守护规则

- 记录模块目录、依赖方向、README 模板和命名约定；
- 为跨模块非法导入增加可自动执行的检查；
- 用特征测试锁定 Query History 当前 API、权限、隐私、CSV 和 worker 行为；
- 记录基线测试中的既有失败，禁止通过删除或放宽断言掩盖。

### 批次 2：迁移 Query History

- 先建立 domain、ports 和 application 测试；
- 建立旧 repository/platform 适配器；
- 迁移 HTTP transport 与 worker；
- 在组合根切换注册后删除旧的重复注册和失去职责的代码；
- 每一步保持可编译，禁止新旧路径同时处理同一请求或任务。

### 批次 3：瘦身组合根

- 让模块自行提供路由和 worker 注册入口；
- 将 `internal/container` 收敛为跨模块组合与进程生命周期；
- 将总 router 收敛为全局中间件、公共协议入口和模块挂载；
- 不在此批次改变业务 API。

### 批次 4：迁移边界较清晰的业务域

按实际耦合审计结果逐个迁移 usage、commercial、app connector 和 workbench。每个域单独形成可合并批次，不以一次 PR 横跨所有域。

### 批次 5：迁移复杂核心域

knowledge、agent 和 session 最后处理。开始移动目录前，先分别识别域内子能力与所有权；例如 knowledge 至少需要区分接入、处理、FAQ/Wiki 和检索协作，避免把现有大 service 原样移动到一个新目录。

隐藏复杂度若暴露新的领域或接口冲突，必须暂停该批迁移，更新 Spec/ADR 后再继续。

## 8. 测试与验证

每个迁移批次必须提供以下证据：

1. **特征测试**：迁移前锁定外部可观察行为；
2. **domain/application 单元测试**：通过 fake ports 验证策略和用例，不依赖 Gin 或真实数据库；
3. **adapter 测试**：验证租户隔离、not-found 语义、查询过滤和状态更新；
4. **transport 测试**：验证路径、RBAC、输入校验、状态码和响应兼容；
5. **装配测试**：验证模块可构建，路由和 worker 各注册一次；
6. **回归测试**：至少运行变更相关包、`go test ./internal/...`、`go build ./...` 及变更范围 lint；
7. **最终 diff 审查**：确认没有无关行为变化、数据库迁移或 API 漂移。

对 Query History，特征测试至少覆盖：正常/匿名/禁用策略、跨租户防枚举、快照消息截断、导出过滤、CSV 列顺序、入队失败、worker 重试、完成幂等和文件下载。

## 9. 模块治理规则

为防止新结构再次退化：

- 每个模块 README 必须记录职责、非职责、公开入口、依赖端口、拥有的数据和外部契约；
- 新功能评审首先确定业务所有者模块；无法确定时先处理领域歧义，不默认放入 shared/common；
- `platform`、`common`、`utils` 不得成为业务代码的兜底目录；
- 超大文件是职责复核信号，但拆分必须依据用例或业务边界；
- 模块公开 API 保持最小化，内部类型不为测试方便而导出；
- 循环依赖不得通过全局变量、service locator 或复制接口解决；应重新确认所有权或引入明确端口/事件；
- 迁移一项能力时同步迁移其测试，使测试位置也能表达功能归属。

## 10. 风险与缓解

### 双重注册

新旧路由或 worker 并存可能造成 Gin 冲突或任务执行两次。每个切换步骤必须以装配测试验证单一注册，并在同一提交删除旧注册。

### 名义模块化

只移动文件、继续跨模块调用内部 service，会得到新的目录但没有边界。导入检查、使用方定义端口和 README 所有权说明共同作为门禁。

### 共享模型复制

Query History 需要 Session/Tenant 数据，但复制模型会形成双真源。试点必须使用读端口和适配器，明确保留写模型所有权。

### 迁移范围失控

复杂域可能诱发顺手重写。每批以兼容迁移为主；行为变化需要独立 Spec/Ticket，不与目录整理混合。

### 长期双结构

迁移期新旧布局并存会增加认知负担。每批必须有完成条件和旧路径删除清单；新功能优先进入已建立边界的模块，未迁移域继续遵循原结构直到该域正式迁移。

## 11. 验收标准

架构整理的首阶段在满足以下条件时完成：

- Query History 的 transport、application、domain、ports、adapters、装配和测试集中于其模块目录；
- 开发者仅通过模块 README 和目录即可定位其 HTTP 入口、业务规则、数据来源及 worker；
- bootstrap/container 和总 router 只引用 Query History 模块入口，不引用其内部 handler/service/repository；
- Query History 的既有 API、权限、隐私、CSV、数据库和任务语义全部通过兼容测试；
- 自动化依赖检查可以拒绝已定义的非法跨模块导入；
- `go test ./internal/...`、`go build ./...` 和适用 lint 通过，或对迁移前已存在且未被修改掩盖的失败提供明确基线证据；
- 最终 diff 不包含无关功能变更；
- 试点复盘确认该结构适合作为后续模块模板后，才启动下一业务域迁移。

后续全仓整理完成的判定是：主要业务能力均有明确所有者模块，新增功能不再依赖扩大全局技术层，且复杂核心域已经按批准的子能力边界完成迁移。
