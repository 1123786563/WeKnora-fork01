# WeKnora Craft 完整产品 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付基于企业资料创作、预览、持续修改、保存版本和再次打开作品的 React 工作台，按阶段开放网页、文档、表格和演示稿。

**Architecture:** React/assistant-ui 承载对话与作品界面；tRPC-Agent-Go 是完整主 Runtime，经 Go Adapter 委派沙箱中的 OpenCode Runtime。复用主 Run/ToolCall/Decision、会话权限、Sandbox、resource:// 产物和商业系统，Craft 只扩展作品与子执行关联。

**Tech Stack:** Go/Gin/GORM、tRPC-Agent-Go、OpenCode、React、assistant-ui、TypeScript、pnpm、现有 Sandbox 与对象存储；商业接入遵循官方 OpenMeter 专项。

**Spec:** [完整产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)，并受 [tRPC 恢复规格](../specs/2026-09-10-dual-agent-trpc-recovery-design.md)、[React 规格](../specs/2026-09-10-react-multiclient-design.md)、[商业规格](../specs/2026-09-10-saas-billing-connectors-design.md)约束。

## Global Constraints

- “tRPC-Agent-Go：主 Agent Runtime，负责理解需求、管理主上下文、知识检索、规划、工具选择与主执行循环。”
- “OpenCode：沙箱中的执行 Agent Runtime，接受 tRPC-Agent-Go 委派，独立完成文件修改、命令执行、验证和修复循环。”
- “首版同一工作区串行修改。”
- “主 Run、ToolCall 和恢复调度复用现有 tRPC 恢复方案。”
- “关闭或刷新页面只影响订阅，不自动重新提交任务。”
- “历史版本引用保持原始内容，不能静默重定向到最新文件。”
- “基础权限与取消安全是每个可运行版本的前提”。
- “本方案不设定套餐、价格或充值规则，商业权威保持在现有专项方案。”
- 本轮只编写计划；所有复选框表示将来执行，不代表已验证或已实施。新增接口、路径和默认限制均为本计划设计值，不冒充现有 API。

---

## 1. 阅读顺序、基线与执行边界

执行者先读本文件、产品方案、自己的阶段计划及其依赖任务，再读涉及源码。2026-09-10 静态核对仓库是 `/Users/wuyongjun/trea/WeKnora-fork01`；主分支检查点 `aece1c6d537854d3515aaabaaac3300783024fbd`。React、tRPC、商业文档存在并行变更，执行开始须记录实际 HEAD 与依赖交付版本。

已核对 `internal/types/interfaces/agent.go`、`internal/agent/tools/registry.go`、`internal/sandbox/session_binding.go`、`internal/application/service/artifact_collector.go`、`internal/types/artifact_versions.go`、`internal/handler/session/artifact_download.go`、`packages/api-client/src/{client,ports}.ts`、根与 web package.json。当前有 contracts/api-client/domain/ui 和 apps/web；core/views 是 React 迁移目标，不能假定已经就绪。没有在本次规划中启动数据库、模型、沙箱或浏览器验收。

旧 [Adapter 计划](2026-09-10-craft-opencode-adapter.md)与 `.worktrees/craft-opencode` 中暂停实现仅作候选参考；不自动合并，不把局部探测替代完整产品验收。现有 [产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)仍是建议稿，本计划将网页优先、单工作区串行、静态预览作为具体实施建议，不宣称用户已经逐项批准全部产品取舍。

实施时创建隔离工作区并记录进度，只提交当前任务文件。共享入口（router/container/package exports/manifests）按顺序集成，不覆盖并行任务。迁移编号必须在执行当日检查冲突；本计划预留 PostgreSQL `000094_craft`/`000095_craft_versions`/`000096_craft_snapshots`/`000097_craft_usage`、SQLite `000015_craft`/`000016_craft_versions`/`000017_craft_snapshots`/`000018_craft_usage`，以 tRPC 的 `000093/000014` 先落地为前提；冲突时同一任务统一更名并记录，禁止复用已发布编号。

## 2. 阶段、依赖与可验收交付

| 顺序 | 计划 | 任务 | 交付及进入条件 |
|---|---|---|---|
| 1 | [执行与工作区](2026-09-10-craft-01-runtime-workspace.md) | R01–R07 | 固定协议、持久子执行、工作区与两轮委派；R04 起需要 tRPC Run/ToolCall/Fence 接入通过 |
| 2 | [网页与 React 工作台](2026-09-10-craft-02-web-workbench.md) | W01–W06 | 上传→生成→预览→修改→下载；依赖 R07 与 React 登录/空间/对话/分层基础 |
| 3 | [知识、交互与恢复](2026-09-10-craft-03-knowledge-recovery.md) | C01–C06 | 知识引用、持久问题/审批、重连、故障接管与历史继续编辑 |
| 4 | [文档、表格与演示稿](2026-09-10-craft-04-artifact-types.md) | D01–D03 | 三类独立生成、修改、预览、下载闭环；每类单独开关和验收 |
| 5 | [计量、生命周期与发布](2026-09-10-craft-05-operations-release.md) | O01–O05 | 主子调用去重、准入预算、资源维护、运营视图、故障/回退门禁 |

共 **27 个独立任务**。每个任务包含接口、文件、红绿测试、具体实现规则及独立提交；复杂集成步骤继续分成小动作，不能把一个阶段交给单个执行者一次性生成。默认顺序执行；R01 协议探测可在等待 tRPC 基础时完成，W04 纯状态测试可在等待 API 时完成，但不得将 mock 演示标为阶段交付。

外部依赖门禁：

| 门禁 | 所属专项 | Craft 如何判断已满足 |
|---|---|---|
| G1 主 Runtime | [tRPC 计划](2026-09-10-dual-agent-trpc-recovery.md) Task 01–08、11–12 | 已固定 SDK，Run 受理/工具日志/epoch/等待/取消/事件事务有测试证据；子完成后主 Agent 可继续推理 |
| G2 前端基础 | [React 计划](2026-09-10-react-multiclient-migration.md) T01–T04、T09–T12 | 有可用认证/空间 scope、共享层入口、对话及附件上传；Craft 不承担框架迁移 |
| G3 恢复 | tRPC Task 09、14 | 真实 kill/restart，旧 Worker fencing，不明外部写操作不会重发 |
| G4 商业 | 商业规格后续准入/结算接口及官方部署 | 专项提供经核对的准入、预算与原始用量接口；未交付时只开放受控内部试用，禁止收费上线 |

外部依赖缺失只阻塞对应集成/发布门禁；不通过编造 API、默认成功或新建第二套运行/账本来绕过。

## 3. 模块与所有权

| 文件范围 | 责任 | 不负责 |
|---|---|---|
| `internal/craft/` | 工作区、输入、委派、版本、观察结果等稳定业务契约及纯规则 | Gin、数据库、主图执行 |
| `internal/agent/opencode/` | 固定版本 HTTP/SSE、关联、子执行核对 | 主 Run 终态、用户身份、Credits |
| `internal/application/repository/craft_*.go` | Craft 元数据、幂等记录、CAS | 重复建主 Run 或 checkpoint 表 |
| `internal/application/service/craft_*.go` | 授权装配、工作区、委派、产物、生命周期 | 在 service 内重写 OpenCode 编码循环 |
| `internal/handler/session/craft_*.go` | 已授权会话下的作品 HTTP 入口 | 接受用户传入租户或沙箱地址作为权威 |
| `packages/{contracts,api-client,domain,core,views}/src/craft/` | schema、网络、纯状态、控制器、UI | 浏览器保存业务权威或直连 OpenCode |
| `apps/web/src/features/craft/` | 路由装配和平台能力注入 | 复制领域状态与 API 实现 |
| `skills/craft-*`、`docker/craft/` | 版本化构建方法和可复现工具镜像 | 通过技能文本扩大权限 |

## 4. 公共 Go 契约（由 R02 创建）

`internal/craft/contracts.go` 使用标准库 `context`、`time`，导入现有恢复专项包 `github.com/Tencent/WeKnora/internal/agent/runtime`。G1 尚未落地前，R02 的持久化接线等待；不要创建同名替代 runtime 包。

```go
type Scope struct { TenantID uint64; UserID, SessionID string }
type Workspace struct {
    ID string
    Scope Scope
    SandboxID, Generation, OpenCodeSessionID, RuntimeDigest string
    Revision int64
}
type Input struct { Ref, Name, SHA256, CitationID string; Bytes int64 }
type Task struct {
    ID, ToolCallID, Prompt, PromptMessageID, RequestHash string
    Scope Scope
    Fence runtime.Fence
    WorkspaceID string
    Inputs []Input
    SkillDigests []string
    Deadline time.Time
}
type Check struct { Name, Status, Detail string } // passed/failed/not_run
type File struct { Path, Ref, SHA256, MIME string; Bytes int64 }
type Version struct { ID, WorkspaceID, RunID, Kind string; Files []File; Checks []Check }
type Observation struct {
    SessionID, PromptMessageID, AssistantParentID, Finish string
    Completed, Idle, PendingTool, Aborted bool
}
type Result struct { TaskID, Status, Summary string; Files []File; Checks []Check }
type Store interface {
    GetWorkspace(context.Context, Scope) (Workspace, error)
    PutWorkspace(context.Context, Workspace, int64) (Workspace, error)
    PrepareTask(context.Context, Task) (Task, error)
    GetTask(context.Context, Scope, string) (Task, error)
    SaveResult(context.Context, runtime.Fence, Result) error
    GetResult(context.Context, Scope, string) (Result, error)
}
type Executor interface {
    Execute(context.Context, Task) (Result, error)
    Observe(context.Context, Task) (Observation, error)
    Abort(context.Context, Task) error
}
```

所有 scope 来自服务端认证和 session 查询。内部结果 status 仅为 succeeded/failed/canceled/unknown；unknown 映射主 ToolCall 的不明结果等待。`SaveResult` 事务检查实际 Run 的 epoch、lease、session、tool call，不能仅比较 Task 内保存的旧 fence。独立定义 `ErrConflict`、`ErrForbidden`、`ErrNotFound`、`ErrUnknown`、`ErrInvalidInput`、`ErrBusy`、`ErrUnsupported`，调用者使用 `errors.Is`。端口错误与恢复专项错误在装配边界映射。

这些是跨任务接口，不是需要一次填满的巨型 service。未出现的方法由对应任务定义，不允许后续任务自行改名；扩充只在契约文件追加，并同步引用任务。

## 5. 公共产品 API 与事件契约（由 W03/W04 创建）

基路径 `/api/v1/sessions/:session_id/craft`；所有写请求经现有 CSRF/认证/访问策略。通用错误 HTTP：400 输入不合法、403 无写权限、404 资源不可见、409 活跃冲突或 revision 冲突、410 过期、413 超限、503 依赖未就绪。JSON 沿现有 `{data: ...}` 包装，列表 `{data: [], next_cursor: string|null}`。

| 方法/路径 | 请求 | 响应 |
|---|---|---|
| POST `/api/v1/craft/sessions` | `{request_id,title,kind}`，kind=web/document/spreadsheet/slides | `{session_id,workspace_id,engine_type:"trpc"}`；同键异参409 |
| GET `/api/v1/craft/sessions?cursor=...` | 认证 scope | 当前用户可访问作品，按 updated_at+id 游标 |
| GET `/` | 无 | WorkspaceView：工作区、active_run、当前版本、pending、last_seq |
| POST `/inputs` | `{resource_ref,expected_sha256}`；先走现有上传入口 | Input；跨空间/未完成/摘要不符拒绝 |
| POST `/runs` | `{request_id,prompt,input_refs,knowledge_scope,base_version_id?}` | 既有主 RunView；重试只复用受理结果 |
| GET `/versions` | 分页 | Version[]，不可变且关联 Run |
| GET `/versions/:version_id` | 无 | Version 与授权后的预览能力 |
| POST `/versions/:version_id/preview` | 无 | `{url,expires_at,version_id}`，短期、无主站凭据 |
| GET `/versions/:version_id/files/:file_id` | 无 | 指定文件下载，身份校验后读取不可变对象 |
| POST `/restore`（C05） | `{version_id,expected_revision,request_id}` | 恢复完成的 WorkspaceView；有活跃/未知任务409 |

主 Run 状态/事件/decisions/cancel 统一使用 tRPC 专项 `/runs/:run_id` 接口，不在 Craft 复制。Craft 事件封装在已有持久化 RunEvent.payload：`{workspace_id, delegation_id, tool_call_id, kind, data}`，kind 取 `delegation.started/text/tool/finished`、`artifact.published`、`interaction.pending/resolved`、`workspace.unavailable`。外层 seq 由 RunEvent repository 分配。子 finished 不投影主消息 done。预览凭证不写入持久化事件。

## 6. 验收证据与进度规则

- 每任务保存变更 SHA、命令、退出状态、关键断言；失败/跳过与通过分列。建议实施时创建 `docs/superpowers/plans/craft-product-progress.md`，每行记录 ID、依赖 SHA、状态、证据路径、下一步；本轮不把旧进度改成已完成。
- 单测验证协议和状态；数据库测试验证事务/CAS；真实 OpenCode 验证消息关联和 abort；浏览器验证最终用户链路；真实 kill/restart 验证恢复。四者不互相替代。
- 默认 PostgreSQL+Docker 作为首个完整组合；SQLite 单独验证并标注。Cube/E2B 未经历同等测试不可声明具备完整 Craft 恢复；能力返回 unsupported 或明确等待。
- 功能开关默认 `craft.enabled=false`；按阶段启用 `craft.kinds`。只在 O05 门禁通过后扩大开放，旧 builtin 会话保持原路径。

## 7. 方案覆盖自检

| 产品方案章节 | 落点 |
|---|---|
| 1–3 目标与技术职责 | 本文全局约束、R01/R05 |
| 4 入口、工作台、人工交互、文件与版本 | W03–W06、C02/C03/C05 |
| 5 创建到再次打开八步 | R07、W06、C03/C04 |
| 6–7 两 Runtime 与 Adapter | R01–R07，G1 |
| 8 知识、Skills、工具 | R03、C01、C06、D01–D03 |
| 9 会话/Run/工作区/子 session | R02/R04/R05，G1 |
| 10 流式、交互、取消、恢复 | R06、C02–C05 |
| 11 产物、预览、版本 | W01/W02/W05、C05 |
| 12 原始用量与商业边界 | O01/O02/O04，G4 |
| 13 复用资产与禁止直接照搬 | R01/R02/W01，本文件第1节 |
| 14 分阶段交付及非目标 | 五阶段计划与 O05 发布门禁 |
| 15 两层成本与失效条件 | O01/O04 实测，公开托管另立产品规格 |
| 16–17 后续与上游基线 | R01 固定源码/协议/二进制；执行前对照最新已批准规格 |

产品方案中的可选终端按既有权限接入 W05；原生移动工作台、多人合并、公开托管/域名、任意外部应用写操作不进入本计划。商业支付渠道等尚未确认部分由专项推进，O02 明确接口门禁，Craft 不做未经批准的商业决策。

## 8. 本轮计划自查记录

- 6份文件、27个唯一任务，均包含Files、Consumes/Produces、失败测试代码、RED/GREEN命令、实现规则和独立提交步骤。
- 相对文档链接与Markdown围栏检查通过；45个Go示例代码块通过gofmt语法解析；未执行这些示例的功能测试。
- 已对照产品方案第1–17节完成覆盖映射；修正迁移所有权、OpenCode状态/交互方法、工作区初始化、前端类型/导出入口及预览不依赖第三方cookie的细节。
- 依赖未完成的边界保留G1–G4门禁；商业金额、支付渠道和生产托管不在Craft中擅自补定。
- 本轮只新增本总计划和五份阶段计划，未改运行代码、未恢复暂停实现、未合并工作树、未运行数据库/模型/浏览器验收。
