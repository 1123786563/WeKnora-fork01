# Issue #106 层级事实与实施范围清单

调查日期：2026-09-23。调查方式：只读 GitHub API（`gh api`，账号 1123786563），对每个节点读取正文、评论（`/issues/N/comments --paginate`）、时间线（`/issues/N/timeline --paginate`）、下级列表（`/issues/N/sub_issues --paginate`）。

## 结论摘要

- 树共 **12 个节点**：根 #106 + 11 个原生 sub-issue（#108–#118），层级深度 2，全部 `open`，全部带 `ready-for-agent` 标签。
- #108–#118 的 `sub_issues` 端点均返回 `[]`（已读取内容确认为空数组），**无第三层**。
- 每个 Issue 正文均含 `## Parent` 声明指向 #106，与原生 sub-issue 关联**一一对应、无额外正文声明的补充子任务**。
- 全树 **0 条评论、0 个关联 PR**（时间线中所有 cross-referenced 均为 Issue 间引用，`pull_request=false`，且无 connected/closed 事件）。
- Issue 间 cross-reference 与 `blocking`/`blocked_by` 边完全重合，属**依赖引用**，不构成范围扩展。
- #106 正文与仓库事实源 `docs/specs/2026-09-23-self-hosted-plugins-spec.md` **逐字节一致**（3908 字符，程序化比对 `identical: True`）。
- 代码现状：全仓库（排除 docs/node_modules）搜索 `plugin_manifest|PluginInstallation|pluginManifest|插件清单|self_hosted_plugin` 无任何命中；插件安装为全新实现，落点在既有 MCP 基础之上（见 #106 节点 codeStatus）。
- 读取缺口：无。仅首次请求遇到一次 connection reset，按任务要求重试后成功；其余全部一次成功。

## 层级树

```
#106 Spec: 自行托管插件与清单安装 (open)
├── #108 插件 01｜管理员预览插件清单 (open)
├── #109 插件 02｜提供只读 Jira MCP 示例服务 (open)
├── #110 插件 03｜确认空间安装并供成员发现 (open)   [blocked_by #108]
├── #111 插件 04｜在对话中调用无账号插件工具 (open)  [blocked_by #110]
├── #112 插件 05｜成员个人授权与撤销 (open)          [blocked_by #110]
├── #113 插件 06｜在 WeKnora 对话中查询 Jira 待办 (open) [blocked_by #109,#111,#112]
├── #114 插件 07｜预览候选插件版本差异 (open)        [blocked_by #110]
├── #115 插件 08｜管理员接受插件升级 (open)          [blocked_by #111,#114]
├── #116 插件 09｜阻止已接受版本的远端能力漂移 (open)[blocked_by #115]
├── #117 插件 10｜新增写工具默认关闭 (open)          [blocked_by #116]
└── #118 插件 11｜启用写工具并由成员审批 (open)      [blocked_by #117]
```

依赖边（来源：各 Issue 正文 `## Blocked by` 段，与时间线 `blocked_by_added` 事件逐一核对一致）：

```
#108 → blocks #110 → blocks {#111, #112, #114}
#109 → blocks #113
#111 → blocks {#113, #115}
#112 → blocks #113
#114 → blocks #115
#115 → blocks #116 → blocks #117 → blocks #118
```

可并行起步集：#108、#109（正文自述 "None (can start immediately)"）。

## 引用类型区分

| 类型 | 事实 |
| --- | --- |
| 原生 sub-issue | #108–#118 共 11 个（#106 `sub_issues` 端点 + 时间线 11 条 `sub_issue_added`） |
| 正文声明的补充子任务 | 无（全仓库 139 个 Issue/PR 中仅 #108–#118 正文提及 #106，且均为 Parent 声明，与原生关联完全重合） |
| 普通引用（cross-referenced） | 11 条，全部与 blocking 边重合，属依赖引用 |
| 关联 PR | 无（无 connected/closed/referenced+commit 事件） |
| 依赖引用（blocked_by/blocking） | 11 条边，见上图 |

排除项：#107（Craft 网页作品 Spec）及其子树 #119–#139 属另一功能域（Craft web artifact），与插件范围无引用关系，已核验不引用 #108–#118。

## 节点清单

### #106 — Spec: 自行托管插件与清单安装（根）

- URL：https://github.com/1123786563/WeKnora-fork01/issues/106
- 父节点：无（根）
- 状态：open；label `ready-for-agent`；创建 2026-09-23T05:13:13Z；评论 0
- 需求依据：正文即《自行托管插件与清单安装 Spec》（Problem/Solution/27 条 User Stories/Implementation Decisions/Testing Decisions/Out of Scope/Further Notes）
- 验收标准（Spec 级 Testing Decisions，供子 Issue 承接）：应用边界集成测试 seam（受控远程 MCP 服务 + 两名成员）；覆盖安装预览与确认、清单与远程工具不一致、停用、只读 Jira 查询、未授权引导、成员凭据隔离、个人撤销与过期、旧版可用、候选差异、接受升级、候选不可达、远端漂移、新写工具默认关闭、逐项启用写工具、成员批准/拒绝/超时零外部写入、无账号能力、成员可见性、跨空间隔离、手工 MCP 兼容
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- **与仓库事实源出入**：正文与 `docs/specs/2026-09-23-self-hosted-plugins-spec.md` 逐字节一致（无出入）。配套设计文档 `docs/specs/2026-09-23-self-hosted-plugins-design.md` 仅存于仓库（GitHub 上无对应文本），其状态为"需求与产品行为已确认；协议字段、迁移与实施方案待实施计划细化"
- **codeStatus（实现现状）**：全仓库无任何插件清单/安装实现（关键词 `plugin_manifest|PluginInstallation|pluginManifest|插件清单|self_hosted_plugin` 于代码目录零命中，2026-09-23 搜索）。可复用基础存在：Go 后端 `internal/application/service/mcp_service.go`、`mcp_metadata.go`、`mcp_tool_approval_service.go`、`internal/application/repository/mcp_oauth.go` 等 MCP 服务管理/元数据快照/OAuth 绑定/工具审批；前端 `frontend/src/api/mcp-service.ts` 等。即：11 个子 Issue 全部为在既有 MCP 基础上的新增产品层，无已实现部分
- **根级未被子 Issue 验收标准显式承接的事项**（缺口，最终裁决需知悉）：Spec Further Notes 要求"正式实施前需固定清单协议、网络抓取限制、版本漂移处置、OAuth 身份映射、数据迁移与回退方案"；子 Issue 验收均未单列这些工程前置项（Spec 自身将其定位为实施计划职责，不改变已确认用户行为）。另：#106 正文本身并未写明"11 个下级 Issue"字样，11 这一数量来自原生 sub-issue 列表实测

### #108 — 插件 01｜管理员预览插件清单

- URL：https://github.com/1123786563/WeKnora-fork01/issues/108
- 父节点：#106（原生 sub-issue + 正文 Parent 声明）
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:24:44Z
- 需求依据：空间管理员粘贴自托管插件清单地址，在不创建应用安装的前提下审阅版本、执行端点、实际 MCP 工具、读写分类与个人授权要求（对应 Spec US7/8/9）
- 验收标准：
  1. 管理员看到从清单和远端工具发现核验后的预览，预览有明确有效期且不会创建安装
  2. 普通成员无权预览；受限网络地址、无效清单、端点不可达或工具声明不符时明确拒绝
  3. 以受控 HTTP/MCP 服务验证成功和失败路径，不依赖真实 Jira
- 边界：仅预览的产品入口和校验行为；不创建安装、不改 Agent 调用
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：无（可立即开始）

### #109 — 插件 02｜提供只读 Jira MCP 示例服务

- URL：https://github.com/1123786563/WeKnora-fork01/issues/109
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:24:51Z
- 需求依据：交付可独立部署的远程 MCP 示例服务，提供"查询本人本周 Jira 待办"工具，作为首个插件开发参考（对应 Spec US1/3/6/20/21）
- 验收标准：
  1. 工具描述和输入 schema 固定；模型不能传入令牌、任意用户 ID、任意 JQL 或任意目标 URL
  2. 服务通过成员授权映射 Jira 账号；返回事项标识、标题、状态、截止日期和来源链接，空结果明确为空
  3. 用 Jira API 替身验证两名成员隔离、403、超时、授权失效和空结果；提供真实 Jira 联调说明，自动测试不依赖真实凭据
- 边界：只负责独立插件服务及其文档/测试；不修改 WeKnora 安装页面或平台持久化
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：无（可立即开始）

### #110 — 插件 03｜确认空间安装并供成员发现

- URL：https://github.com/1123786563/WeKnora-fork01/issues/110
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:24:59Z
- 需求依据：管理员从经核验的预览确认确定版本的应用安装；所有本空间成员可发现，管理员可停用（对应 Spec US10/11/16/27）
- 验收标准：
  1. 确认时重新核对预览身份和版本；预览内容已变或过期则拒绝，重复提交不产生重复安装
  2. 本空间成员可见安装与状态，跨空间不可见；只有管理员可安装和停用
  3. 停用后安装状态可观察；现有手工 MCP 服务仍按原路径工作
  4. 固定安装 ID、已接受版本、空间归属和状态的对外语义，供后续 Ticket 消费
- 边界：负责应用安装与成员目录；不在此 Ticket 接入 Agent 工具或成员 OAuth
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #108

### #111 — 插件 04｜在对话中调用无账号插件工具

- URL：https://github.com/1123786563/WeKnora-fork01/issues/111
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:02Z
- 需求依据：成员在对话中调用已安装版本的无账号只读 MCP 工具，获得带来源标识的结果（对应 Spec US5/23）
- 验收标准：
  1. Agent 只发现已安装、已启用且属于已接受版本的工具；未安装、停用或跨空间调用被拒绝
  2. 受控 MCP 服务返回成功、空结果和错误时，对话呈现相应可观察结果；远端失败不被编造成成功
  3. 现有手工 MCP 服务继续可用；测试从对话入口贯通到远程工具
- 边界：负责 Agent 工具暴露与调用的完整路径；不修改个人连接、升级审阅或写工具策略
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #110

### #112 — 插件 05｜成员个人授权与撤销

- URL：https://github.com/1123786563/WeKnora-fork01/issues/112
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:04Z
- 需求依据：成员从已安装插件入口查看本人状态，完成个人 MCP OAuth 授权，并能撤销和重新授权（对应 Spec US17/18/19/26）
- 验收标准：
  1. 两名成员分别授权同一插件；状态与凭据不混用，跨空间不可读
  2. 未授权或授权过期的成员无法使用需个人连接的工具；撤销本人授权不影响其他成员
  3. 授权与重授权从成员界面完成；以受控 MCP 服务验证流程，不依赖真实 Jira
- 边界：负责个人连接和授权入口；不修改 Agent 工具调用或版本升级页面
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #110

### #113 — 插件 06｜在 WeKnora 对话中查询 Jira 待办

- URL：https://github.com/1123786563/WeKnora-fork01/issues/113
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:07Z
- 需求依据：把 Jira 示例插件接入空间安装与成员个人连接，使成员在对话中查询本人本周待办（首个纵向案例，对应 Spec US20/21）
- 验收标准：
  1. 未授权成员先获得个人授权入口；授权后对话返回本人 Jira 事项及可打开来源链接
  2. 两名成员面对不同 Jira 数据时结果隔离；空结果、403 和超时均不生成虚构事项
  3. 以 WeKnora 应用边界、受控 Jira API 和远程 MCP 服务验证完整流程，并说明真实 Jira 联调步骤
- 边界：只完成 Jira 纵向案例与跨系统验收；不引入写操作或改版本升级规则
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #109、#111、#112（汇合点：示例服务 + 工具调用 + 个人授权）

### #114 — 插件 07｜预览候选插件版本差异

- URL：https://github.com/1123786563/WeKnora-fork01/issues/114
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:09Z
- 需求依据：管理员预览候选插件版本相对已接受版本的能力变化，而不立即切换运行版本（对应 Spec US12/24）
- 验收标准：
  1. 展示工具新增/移除、参数 schema、scope、读写分类和执行端点的差异
  2. 候选清单或服务不可达、发现结果与声明不符时给出错误，旧版本保持可用
  3. 两个空间可分别预览或停留在不同版本；预览不更改任何空间已接受版本
- 边界：负责管理员候选版本审阅；不修改 Agent 调用或个人授权
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #110

### #115 — 插件 08｜管理员接受插件升级

- URL：https://github.com/1123786563/WeKnora-fork01/issues/115
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:12Z
- 需求依据：管理员接受候选版本后，空间的已接受版本与 Agent 后续调用一起切换；未接受的空间继续使用旧版（对应 Spec US3/13/24）
- 验收标准：
  1. 只有管理员可接受，接受前再次核对候选快照；重复请求幂等
  2. 升级失败或候选端点不可达时保持旧版；两个空间能使用不同已接受版本
  3. 从成员对话验证升级前调用旧版、升级后调用新版，而不只检查数据库状态
- 边界：负责版本切换与调用绑定；远端定义漂移的阻断由后续 Ticket 处理
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #111、#114

### #116 — 插件 09｜阻止已接受版本的远端能力漂移

- URL：https://github.com/1123786563/WeKnora-fork01/issues/116
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:15Z
- 需求依据：远端在已接受版本端点改变工具目录或 schema 时，Agent 拒绝采用未经管理员审阅的能力（对应 Spec US25）
- 验收标准：
  1. 新增、移除工具及 schema 变更均形成明确漂移状态，受影响调用被阻止并提示管理员复审
  2. 受控 MCP 服务在安装后改变目录，测试从成员对话观察拒绝；其他空间不受影响
  3. 保留现有手工 MCP 服务兼容行为，避免把插件版本治理静默施加到旧服务
- 边界：负责运行时快照核验和漂移状态；不修改候选版本审阅界面
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #115

### #117 — 插件 10｜新增写工具默认关闭

- URL：https://github.com/1123786563/WeKnora-fork01/issues/117
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:18Z
- 需求依据：管理员接受带新写工具的版本后，写工具仍关闭，直到管理员逐项启用（对应 Spec US14/15）
- 验收标准：
  1. 新增写工具在空间安装和升级后默认不可调用；个人授权或 Agent 选择不能绕过
  2. 管理员在插件工具视图中看到关闭原因并能逐项启用；只读工具仍按原策略工作
  3. 用记录外部调用的 MCP 测试服务确认未启用时写入次数为零，且不改变手工 MCP 服务的既有默认语义
- 边界：负责插件工具策略和管理员启停；审批交互由后续 Ticket 处理
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #116

### #118 — 插件 11｜启用写工具并由成员审批

- URL：https://github.com/1123786563/WeKnora-fork01/issues/118
- 父节点：#106
- 状态：open；`ready-for-agent`；创建 2026-09-23T05:25:20Z
- 需求依据：管理员可要求已启用写工具在每次调用前由成员审阅确定的目标和输入；批准才派发（对应 Spec US15/22）
- 验收标准：
  1. 批准、拒绝、超时和停用分别产生明确结果；拒绝、超时或撤销后不派发外部写入
  2. 成员审批展示确定的操作目标和参数；批准后测试服务仅记录一次派发
  3. 从对话入口验证管理员策略、成员决议、远端调用与审计结果一致
- 边界：负责写工具审批闭环；不增加 Jira 写操作，使用受控测试服务
- 评论要点：无评论
- 关联 PR：无
- 读取缺口：无
- 依赖：blocked_by #117

## 调查方法与可复核命令

- 根与子节点正文：`gh api repos/1123786563/WeKnora-fork01/issues/106`、`.../issues/{108..118}`
- 评论：`gh api --paginate repos/1123786563/WeKnora-fork01/issues/{N}/comments`（全部返回 `[]`）
- 时间线：`gh api --paginate repos/1123786563/WeKnora-fork01/issues/{N}/timeline`
- 下级：`gh api --paginate repos/1123786563/WeKnora-fork01/issues/{N}/sub_issues`（#106 返回 11 项；#108–#118 均返回 `[]`）
- 反向排查：`gh api --paginate "repos/1123786563/WeKnora-fork01/issues?state=all&per_page=100"` 共 139 项，仅 #108–#118 正文提及 #106
- 事实源比对：#106 正文 vs `docs/specs/2026-09-23-self-hosted-plugins-spec.md` 逐字符比对一致；配套 `docs/specs/2026-09-23-self-hosted-plugins-design.md` 已读
- 代码现状：`rg -l -i "plugin_manifest|PluginInstallation|pluginManifest|插件清单|self_hosted_plugin"`（代码目录零命中）；`rg -l -i "mcp" --type go internal` 命中 MCP 基础 205 文件
- 分页说明：sub_issues 单页 11 项（<30 上限）且 `--paginate` 已跟随全部页；comments/timeline 同理，无截断

## readGaps 汇总

无读取缺口。唯一异常：首次 `gh api .../issues/106` 遇 connection reset，第 2 次重试成功（任务允许 2–3 次重试）。所有节点四类端点均成功读取，空结果均以原始响应内容（`[]`）核实，未以"页面计数"推断。
