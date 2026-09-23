# 求职能力的深 Module 设计

状态：推荐架构，待审阅；落实 [已批准求职规格](../../specs/2026-09-23-weknora-job-search-design.md)，不修改其产品要求，也不授权实施。
方法：按 codebase-design 的 Module、Interface、seam、Adapter、depth、leverage 与 locality 判断代码形状。

## 1. 现有代码给出的约束

- [后端模块化规格](../../specs/2026-09-21-backend-domain-module-reorganization-design.md)要求新业务按纵向能力归位，模块只暴露少量稳定入口，业务事实有唯一写入所有者。当前 [Workbench Module](../../../internal/modules/workbench/module.go) 与 [Identity Module](../../../internal/modules/identity/module.go)仍是 Pass A 骨架，部分实现位于旧目录；新求职代码不应进入全局 handler、service、repository、types。
- [移动深 Module 设计](../../specs/2026-09-20-mobile-module-seams.md)已经把 Deployment/Tenant scope、Task 提交与恢复放在 Mobile Runtime 和 Task Office 后。当前 [mobile-core](../../../packages/mobile-core/src/index.ts)主要实现 Runtime，原生端 [AuthorizedLandingScreen](../../../apps/mobile/src/screens/AuthorizedLandingScreen.tsx)仍是业务占位。
- Web 已有对话、附件和产物入口；小程序 [任务页](../../../apps/miniprogram/src/features/execution/pages.tsx)已有部分 Task 流程。已有 [Workbench Artifact 下载](../../../internal/handler/session/workbench_artifacts.go)按 run、session、message、index 授权；它不能直接代表求职申请固定的材料版本。
- [Tenant 创建入口](../../../internal/handler/tenant.go)受部署自助创建策略和个人空间数量限制。Career 不得绕过 Identity 自建 Tenant，也不能假设每位用户总能自助开通。
- [Workbench admission](../../../internal/modules/workbench/service/workbench/admission.go)负责有 request ID 的准入与预算预占；Career 不能创建第二套 Run、审批或 Credits 账本。

## 2. 比较三种 Module 形状

| 方案 | Interface 的学习成本 | leverage 与 locality | 结论 |
| --- | --- | --- | --- |
| 把求职字段、规则与页面加到 Workbench | Task Interface 会知道求职档案、岗位、简历和申请事件；其他工作类型也要背负这些概念 | 资格判断、来源去重和材料校验散落在 Workbench、Agent 提示词与三端 | 不采用 |
| 后端独立 Career，三端各自调用细粒度传输方法 | 后端拥有数据，但每端需学习抓取、确认事实、版本、重试、订阅的顺序 | 三端会重复实现同一状态机，尤其小程序和原生端易漂移 | 不采用 |
| 后端 Career Office + 跨端 Career Desk 两个深 Module | 调用者只需学习少量领域意图、视图、回执和失效状态 | 后端集中求职不变量；客户端集中 scope、幂等、版本冲突与恢复 | 推荐 |

删除测试：若删掉 Career Office，档案事实、岗位快照、资格、申请时间线和材料版本规则会散回多个后端模块；若删掉 Career Desk，三端页面会各自重新组合传输、缓存与提交恢复。两个 Module 都能消除调用方原本必须承担的复杂度。

## 3. 推荐拓扑与所有权

    apps/web、apps/mobile、apps/miniprogram 的呈现 Adapter
                    ↓
       packages/career-core 的 Career Desk Interface
                    ↓
       packages/api-client/career 的传输 Adapter
                    ↓
       internal/modules/career 的 Career Office Interface
          ↙ Identity、Workbench、Agent Runtime、
            Agent Catalog、Commercial、Platform 的公开 Interface

Career Office 唯一写入：求职档案事实及确认版本、岗位来源观察与快照、资格和匹配评估、持续找岗规则、求职申请、结构化材料正文与版本、申请进展事件和投递版本绑定。数据库表、Worker、路由归这个 Module，迁移由它拥有；通用 Task、Run、成员、预算、通知投递和二进制 Artifact 的权威仍在现有 Module。

Career Desk 唯一拥有三端共同的求职交互状态：当前 Tenant 的资料与申请视图、乐观修订号、待提交意图、未知结果对账、跨端变更提示、短期缓存失效和安全错误呈现。Web、Expo 与 Taro 页面只提供输入、渲染与平台动作；不直接编排多个传输调用。Mobile Runtime 的 Scope Lease 在原生端仍是身份与空间失效的权威，Career Desk 消费它而不重建一份。

不要为每张表建立一个公开 Module。档案、岗位、申请、材料彼此共享版本与证据规则，先作为 Career Office 的内部组织；只有出现独立调用者和真正不同的演进节奏时，才考虑把内部 seam 升为外部 Interface。

## 4. Career Office 的外部 Interface

示意形状：

    Open(scope, CareerRef) -> CareerView
    List(scope, CareerFilter, cursor) -> CareerPage
    Act(scope, CareerIntent, requestID, expectedRevision) -> CareerReceipt
    Changes(scope, cursor) -> CareerChangePage

CareerRef 是封闭的 Profile、Opportunity、Application、MaterialVersion、SearchRule 引用集合。CareerIntent 是封闭的业务意图集合，例如 ConfirmFacts、ImportPosting、StartSearch、SetSearchRule、StartApplication、ReviseMaterial、ConfirmMaterial、RecordProgress、ExportData、DeleteData；不能传任意方法名、任意字段更新或模型原始指令。传输 Adapter 可以把这些意图映射成易读的 HTTP 路由，但路由数量不是另一个业务 Interface。

Interface 还包含以下调用约束：

- scope 中的用户与 Tenant 由认证上下文取得，客户端传来的 owner 或 tenant 字段不能决定权限；所有读写都用 Tenant 与 Owner 谓词。
- 改写既有对象必须带 expectedRevision；冲突返回当前修订与可理解的差异，不做最后写入者覆盖。requestID 对一个确定意图幂等，同 ID 不同内容拒绝。
- Act 可返回 accepted、needs_user、completed 或 unknown。长时间搜索和生成先返回可恢复回执；unknown 只对账，不换 ID 自动重试。Changes 是刷新提示，Open/List 返回权威状态。
- 岗位抓取不完整返回 source_incomplete 与原链接；资格资料缺失返回 needs_fact_confirmation；权限、额度、来源不可用、版本冲突及无法确定结果各有稳定错误类别。调用者不解析提示词或工具日志猜状态。
- 对 Open/List 的公开性能目标是分页、可缓存的视图读取；长时间网络抓取和渲染只能经异步 Run/Worker。具体延迟与额度阈值在实施测量后固定。

普通调用者只需 Open 申请、Act 一个意图、再根据回执恢复；不需要知道先创建 Session 还是 Task、何时读最新 JD、怎样检查预算、PDF 是否已上传或推送是否送达。

## 5. Career Desk 的外部 Interface

三端共享包建议落在 packages/career-core。外部 Interface 保持：

    open(ref) -> CareerView
    list(filter) -> CareerPage
    act(CareerIntent) -> CareerReceipt
    observe(listener) -> unsubscribe

Career Desk 在内部持久化未完成意图与 requestID、管理 expectedRevision、收到 scope 撤销时丢弃迟到结果、在网络恢复后查询回执并重新读取视图。它返回可直接呈现的资格依据、材料状态、待办和操作限制；页面不读取原始 wire DTO。离线允许保留编辑草稿，联网后由用户确认提交；不静默提交申请状态或外部动作。

Web、Expo、Taro 分别提供文件选择、系统分享、通知订阅、受控本地存储与导航 Adapter。三端共享行为，不强求相同布局。packages/contracts 解析传输字段，packages/api-client 负责请求，二者在 Career Desk 的传输 Adapter 后，页面不可直接混合使用它们。现有 packages/mobile-core 继续负责 Mobile Runtime/Task Office；不把求职状态机塞进去。

## 6. 内部 seam 与 Adapter

| 依赖类别 | seam 位置与 Adapter | Career 隐藏的行为 |
| --- | --- | --- |
| in-process | Career 内部的资格规则、申请事件投影、材料主张校验 | 三值资格判断、硬条件警示、事件更正与合格申请指标 |
| local-substitutable | Career 持久化 seam：生产数据库 Adapter、测试数据库 Adapter | 版本化事实、CAS、幂等请求、Outbox、去重索引 |
| remote but owned | Identity 范围/开通、Workbench Task/Artifact/Inbox、Agent Runtime 运行、Catalog 版本解析、Platform 文件/队列：各由拥有方公开 Interface 接入，测试用内存 Adapter | 跨模块状态编排、失败恢复；不直接写其他模块表 |
| true external | 岗位来源读取 Interface：企业官方页、校园平台、允许接入的招聘平台与用户粘贴内容 Adapter；测试用固定响应 Adapter | 来源健康、限流、原文保存、完整性、更新与谨慎去重 |
| local-substitutable / true external | 文档提取与 PDF/DOCX 渲染 Interface：真实渲染 Adapter、测试 Adapter；文件内容测试还须用真实渲染器的验收检查 | 同一正文导出、版面/文本层检查、失败不发布 |

其中“数据库 Adapter 与测试数据库 Adapter”“真实来源与固定响应 Adapter”“真实渲染与测试 Adapter”都是实际需要的变化点。纯粹为了把一个函数包在 Interface 里而只保留一个 Adapter，不增加 depth。内部 seam 不向页面或其他业务 Module 暴露。

上传的简历与证明材料先由 Career 的 Intake 实现处理为事实提案，用户确认后才成为求职档案事实。不要自动导入团队知识库；送入模型前由 Career 选择当前任务所需事实并遮蔽无关个人字段。岗位原文作为不可信数据进入提取器，不得成为工具授权或系统指令。

## 7. 跨 Module 的三处难点

### 7.1 申请与 Task 建立

Career 先按 Tenant、Owner、岗位快照、招聘批次和 requestID 持久化 pending 申请，再通过 Workbench 的 Task 建立 Interface 创建独立 Task 并固定 Agent 版本，最后关联 taskID；Workbench 在其内部协调 Conversation 的 Session。Career 不应自己编排先建 Session、再建 Task 的顺序。跨 Module 调用不能假装单事务：Task 建立结果未知时保留 linking 状态，用同一 requestID 查询并补全，不能另建 Task。只有关联确认后申请进入 ready。搜索 Task 使用同一准入与预算规则，但不等于某份申请。

### 7.2 材料版本与通用 Artifact

Career 拥有结构化正文、事实引用、版本 hash、PDF/DOCX 内容对应关系和“可用于投递”状态；Workbench 拥有通用 Artifact 元数据、授权下载与通知。当前消息绑定的 Artifact grant 无法稳定指向申请材料版本。实现前需要由 Workbench 提供按固定 artifact/version ID 授权的 Interface，验证 Tenant、Owner、申请与版本关系后发短时下载授权；下载时仍须检查版本未撤销、未删除，以满足删除后旧链接立即失效。Career 只保存不透明 artifact 引用，不自行拼装存储 URL。

生成流程依次为：冻结输入版本 → 起草与独立审阅 → 用户确认正文 → 同一内容渲染两种文件 → 验证文本/版面/可编辑性 → 通过 Artifact Interface 幂等发布 → Career 提交版本状态。中途失败或结果未知保留 staged 状态并对账，不展示为已投递可用。投递确认固定用户实际选择的版本；未知版本明确记录为未知。

### 7.3 持续找岗、通知与删除

Career 拥有找岗规则与下次检查时间，通过 Platform 队列/Worker Adapter 原子认领一次触发，再经 Workbench 的准入启动有预算的 Task。任务失败、被暂停或来源不完整都形成可见状态；相同岗位只产生一个待办。Career 向 Workbench 发布不含敏感正文的 Attention 事实，Workbench 决定站内收件箱和平台通知送达；推送仅提示重新同步。

用户可从 Career 提出导出或删除意图；实际删除由 Identity/Tenant 保留策略裁定并发起，Career 清理自己拥有的资料、来源快照、申请和导出引用，再由 Workbench/Platform 清理相关 Task/Artifact；法律保留与外部已投递内容单独呈现。客户端先撤销 scope 并清理受控缓存。不能只删 Career 表而留下可下载简历。

## 8. Interface 验证与交付依赖

测试应从 Career Office Interface 和 Career Desk Interface 观察行为，而非分别复述去重函数或页面状态实现。核心场景：2026 届对 2027 届岗位不符合但用户可显式继续；缺少毕业时间为待确认；同岗多来源和跨批次区分；岗位更新不改旧快照；未确认事实不能进入简历；两种导出对应同一内容；跨端版本冲突；网络 unknown 对账；额度不足不丢记录；Tenant 切换拒绝迟到结果；删除后旧授权不能下载。

适配器契约另测：每个岗位来源的可用性与失败分类、真实 PDF/DOCX 导出、Workbench Artifact 授权、Identity 个人空间开通、WorkBench admission 与通知投递。三端各做真实设备流程验收；共享 Interface 测试不替代 iOS、Android、微信小程序的文件、分享和通知验证。

推荐实施前置顺序：

1. 固定 Career 的领域合同与所有权，并完成 Identity 个人空间开通 Interface 和 Workbench 的通用 Artifact 版本下载 Interface；未提供这些前置能力时不以 UI 假数据掩盖。
2. 建 Career Office 的档案、岗位、申请主链及持久化；先用手动 JD 和一个有资格的来源 Adapter 证明 Interface 的 depth，再扩展来源。
3. 接入 Workbench Task/Run、预算、Artifact、Inbox，完成跨 Module 对账与失败恢复。
4. 建 Career Desk 和三个呈现 Adapter，按同一契约验收全流程；原生 Task Office 现有缺口须同步补齐。

这是一份架构设计，不是逐文件实施计划。待本设计审阅后，再由实施计划确定具体类型、路由、迁移、来源清单、资源限额与任务拆分。
