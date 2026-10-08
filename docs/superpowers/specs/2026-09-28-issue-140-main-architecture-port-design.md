# Issue #140 Career 移植到当前 main 架构

状态：已批准（用户于 2026-09-28 确认）。该设计承接已批准的产品规格 `docs/specs/2026-09-23-weknora-job-search-design.md`，只定义将其能力移植到当前 `main` 架构的技术边界，不改变产品行为。

## 目标

将 #140 的 Career 能力集成到当前 `main`，保留其现有模块化后端、TDesign Web、Taro 小程序和 Expo 移动端结构。交付可在当前 `main` 上演进的代码与验证证据，而不是把旧集成分支的历史和旧 UI 一并合入。

功能事实源继续是已批准的 #140 产品规格、相关 ADR、`CONTEXT.md` 和正式 Issue 验收。旧集成分支仅作为现有业务实现及测试行为的参考；若其代码与事实源冲突，以事实源为准。

## 当前架构证据

- 后端按 `internal/modules/<domain>` 组织；模块迁移和所有权记录在 `docs/architecture/moves/*.yaml`。容器与 HTTP 路由由现有装配层集中注册。
- `internal/workbench` 已提供模块骨架、HTTP handlers、工作台执行和 artifact 接口；Career 申请应通过既有 Task/Workbench 能力，而不再引入第二套任务运行时。
- Web 采用 feature 目录与当前 TDesign 页面，路由在现有 Web shell 装配；不能依赖旧分支的 `apps/web/src/career` 页面样式和平台封装。
- 小程序以 `features`、`subpackages`、共享 services 和 Taro 原生组件为边界；不能恢复旧的 `apps/miniprogram/src/career` 页面树。
- `packages/career-core/testdata/wire-fixtures.json` 已存在；实现阶段需先检查其消费者和所有者，再决定扩展现有合同或建立正式 package API，不可把 testdata 文件误当成运行时合同。
- 主工作区已有用户未提交的 Issue 30 文档、小程序测试 helper 和截图。实现、生成产物、测试改动都必须在独立工作树中；主工作区文件不属于本移植范围。

## 架构决策

### 1. Career 领域模块与 Workbench 的边界

建立 Career 模块作为求职业务事实的唯一权威：个人求职空间/档案、来源观察与固定岗位快照、资格及匹配评估、申请、结构化材料版本、投递确认、申请事件、找岗规则、提醒、导出和删除。

Career 经窄合同使用既有 Identity/Tenant、Workbench Task、Usage/Budget、Artifact、通知与审计能力。Career 不拥有通用任务执行、通用文件授权或第二套身份实现。每个申请继续关联独立 Task；搜索任务与申请任务保持不同领域身份。

后端新增模块应遵守 `docs/architecture/moves/README.md` 和同类 manifest 的所有权、导入方、集成点、禁止共享文件及定向验证约束。需要改容器、路由、迁移注册等共享装配文件时，由集成任务串行处理，并验证模块边界检查。

### 2. 合同、证据与存储

向客户端公开版本化 Career API。服务端从认证上下文推导 actor/tenant，逐请求检查 owner 与 tenant；客户端传入的 ID 不能代替权限校验。

共享 wire contract 放在经代码探索确认的 canonical package/API client 边界，并用 fixture 驱动 Go、Web 和小程序的兼容测试。所有外部响应先进行形状、枚举、revision、digest 和请求 ID 解码，再影响状态或写入。

固定岗位快照、用户确认的档案事实、材料版本和申请事件采用追加或不可变语义。求职申请固定岗位快照；材料正文改动产生新版本；投递版本由用户确认；申请阶段从确认事件投影。失败或结果未知必须保留原始 request ID 的恢复路径，不通过生成新 ID 覆盖未知操作。

数据库迁移归 Career 模块所有并按仓库当前 migration owner/registration 流程接入。迁移、索引、tenant-owner 条件、删除级联及导出内容需从当前 schema 实际设计，不机械移植旧分支模型。

### 3. Web 与小程序呈现

Web 以当前 route registry、TDesign 控件、i18n 和页面数据层构建 Career feature；优先建立领域状态/服务 seam，使加载、空、错误、冲突、forbidden、unknown 和恢复态可验证。

小程序将 Career 主入口和流程放在匹配当前包预算与导航的 `features`/`subpackages` 结构，通过共享 API client/service 访问同一后端事实。页面使用现有 Taro/TDesign Miniprogram 原生集成方式；下载、分享、订阅通知走当前平台 adapter，不能复用 Web DOM API。

Web 与小程序复用状态、wire contract 和 fixture，不共享视图实现。两端均须覆盖身份/空间变化清理私密状态、回执错配、并发重试、权限撤销、旧异步响应隔离及导出/删除恢复。

### 4. 移动 App、鸿蒙和发布闸口

保留批准规格与 ADR 0018 的 Expo iOS/Android 及鸿蒙原生闸口。先审查当前 `apps/mobile` 路由、API composition、文件和分享 adapter；在实施计划中把移动端业务缺口与认证/Task/文件能力验证拆为可验收 slice。没有实际设备、构建、认证和端能力证据时，状态保持 blocked/unverified，不得把 Web 或模拟器测试说成原生交付通过。

该移植不自动关闭既有外部发布、来源许可、成本/额度数值或合规运营闸口。

## 数据流与信任边界

```text
Web / Expo Mobile / Taro Mini Program
        │  versioned API + decoded DTOs
        ▼
Career HTTP boundary ── authenticated actor/tenant checks
        │
        ▼
Career application services ── narrow ports ── Identity / Workbench / Usage / Artifact / Notification
        │
        ├── Career-owned repositories and migrations
        └── append-only snapshots, confirmations, versions, events, request receipts
```

外部 JD 和来源页面均是不可信数据，不能成为 Agent 指令或权限来源。模型只接收完成当前操作所需的、已确认且最小化的资料事实。对外投递始终由用户本人执行；下载、推送和任务关联均按 actor/tenant 和固定版本授权。

## 移植阶段

1. **代码基线与模块契约**：建立基于当前 `main` 的 DAG/ledger；确认模块 manifest 规则、migration owner、canonical contracts package、路由/容器 seams、主工作区预存改动清单。输出接口与 owned-files 表。
2. **后端领域纵向基础**：个人空间/档案确认、快照/来源、资格评估及授权边界；Career 模块迁移、repositories、服务、handlers、容器和路由接入。
3. **申请与执行协作**：申请与一申请一 Task、材料不可变版本、本人投递确认、事件时间线、恢复合同。
4. **查找、提醒及隐私生命周期**：手动/来源导入、一次性/持续搜索、预算、提醒、导出/删除和并发/回执恢复。
5. **Web**：按上述领域接口接入 TDesign 路由，交付关键状态和用户工作流。
6. **Taro 小程序**：按当前 feature/subpackage 结构接入相同领域流程和平台 adapters。
7. **Expo 移动端与环境闸口**：按 ADR 0018 与可用设备证据排期；缺失环境显式保留为 blocked。
8. **跨层质量门**：完整 Go/contract/Web/Mini 测试、类型检查、构建、模块边界检查；任务级 reviewer、全分支 review 与 OCR，逐项保存结果。

实现计划需将每阶段继续拆成可独立验收的纵向业务片，不按数据库/后端/前端水平拆 Ticket。每个实现 Task 指定专业 Agent、文件所有权、接口输入输出和验证命令；共享装配、迁移和合同依赖必须串行化。

## 非目标

- 不合并旧 `codex/issue-140-integration` 分支或其 367 个分叉提交。
- 不复刻旧客户端目录、旧 Web 样式/路由或旧小程序 JSX 布局。
- 不实现自动网申、自动发邮件、未经确认的档案事实，或录用概率承诺。
- 不将 OCR/provider 未完成、未安装设备、缺少来源许可/发布证据解释为已验收。
- 不修改主工作区预先存在的 Issue 30 改动。

## 验收标准

1. Career 的领域实体、权限、证据版本、确认和恢复语义存在于当前模块化后端，并通过当前容器、路由、数据库迁移和模块边界规则。
2. API 的 DTO 解码与 request receipt 恢复由跨语言 fixtures 和服务/handler 测试验证；同一用户/空间的 Web、小程序和已实现的 Mobile 客户端读到同一权威版本。
3. 经批准规格规定的档案、岗位、评估、申请、材料、投递、进展、提醒、导出和删除流程有对应的可观测验收测试，权限撤销、scope 改变、并发、forbidden、unknown 和 stale response 都 fail closed。
4. Web 与小程序入口遵守当前架构和组件系统，页面状态和平台能力与其权限/结果相符。
5. 每个真实设备/平台发布声明有对应设备/build/认证证据；不能验证的目标保持显式 blocked/unverified。
6. 迁移差异与回滚/保留政策、Issue/DAG 到验收映射、测试证据、Review/OCR rulings 均记录在计划与 ledger 中。
7. 仅在本次移植授权的当前 `main` 派生实现中提交；不推送或发布，除非用户另行授权。
