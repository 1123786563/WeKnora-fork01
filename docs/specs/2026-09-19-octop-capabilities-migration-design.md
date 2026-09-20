# Octop 四能力迁移 WeKnora 设计（专家 / 技能市场 / MBTI / 子智能体）

- 日期：2026-09-19
- 状态：已与用户逐节确认，待实施
- 源项目：`/Users/wuyongjun/trea/Octop`（Python + FastAPI + harness-agent）
- 目标项目：WeKnora（Go + gin + trpc-agent-go v1.10.0 已是直接依赖）

## 1. 背景与目标

Octop 是腾讯开源的自托管多 agent AI 助手平台，其中四个能力与 WeKnora 的 agent 域高度互补：

| 能力 | Octop 实现 | WeKnora 现状 |
|---|---|---|
| 专家（Experts） | `library/<id>/` 模板目录，三种来源（内置/SkillHub/发布快照），建 agent 时种子进 workspace | `builtin_agents.yaml` 是 i18n 内置注册表，无目录式模板、无市场、无发布 |
| 技能市场 | SkillHub 远程客户端（search/rankings/download）+ 本机快照式发布 | 租户技能安装管线完整（zip 校验 → installer agent 沙箱安装 → 快照镜像），catalog 已有租户级定义；无远程市场客户端、无租户内分享 |
| MBTI 人格 | 16 型档案 + 28 题测试，渲染进 agent SOUL.md | 空白 |
| 子智能体 | md 角色库（zh 272 / en 217），上下文注入式，执行靠外部 harness-agent | `internal/agent/trpc/` 已封装 trpc-agent-go engine/graph/session；无子智能体概念 |

目标：把四个能力以 Go 实现落进 WeKnora 单体，agent 运行时统一走 trpc-agent-go，前端仅 React 新站（apps/web）。

### 现有代码关键挂载点

- `internal/types/custom_agent.go:98` `CustomAgentConfig`——JSON 配置列，扩展点
- `internal/types/custom_agent.go:316` `QuestionSuggestionConfig.Starters`——对齐 Octop quick_prompts
- `internal/agent/skills/`——SKILL.md frontmatter 加载、租户源、沙箱执行
- `internal/application/service/tenant_skill_install.go:63` `InstallSkill(archive)`——市场桥接目标
- `internal/agent/trpc/{engine,graph,state,checkpoint}.go`——子智能体执行底座
- `internal/agent/tools/craft_delegate.go`——委派工具安全模式参考（严格参数解析、服务端组装权限）

## 2. 决策记录（均经用户确认）

1. **范围**：全量四能力，分阶段实施，顺序 MBTI → 专家 → 子智能体 → 技能市场（从简到繁；M3 可与 M2 并行）。
2. **架构**：方案 A「域内嵌单体」——全部落进现有 `internal/` 分层，不拆独立服务、不造插件框架。
3. **前端**：仅 React 新站（apps/web，TanStack Router + Tailwind 4），Vue 主站不跟进。
4. **市场形态**：远程 SkillHub 消费 + 租户内发布分享双轨。
5. **子智能体**：定义层沿用 Octop md 角色库（资产移植），执行层映射 trpc-agent-go 独立 session 委派（非上下文注入式）。
6. **不含 agent 间互调**（ask_agent/@提及/inbox mailbox），本期仅主→子委派。
7. **专家建 agent 时技能未安装**：自动触发安装（异步 202 + 进度订阅）。
8. **技能市场本期只接 SkillHub 单源**，不做多源 URL 导入（skills.sh/clawhub 等二期评估）。
9. **不迁移 Octop 的服务端 curl|bash 安装回退**（安全面，WeKnora 沙箱管线天然更安全）。

## 3. 总体架构与包边界

```
internal/
├── handler/                      # 新增 5 个 router 文件
│   ├── persona.go                # /api/v1/mbti/*
│   ├── expert.go                 # /api/v1/experts/*、/api/v1/agents/{id}/publish-expert
│   ├── skill_market.go           # /api/v1/skills/market/*、/api/v1/experts/market/*
│   ├── subagent.go               # /api/v1/subagent-catalog/*、/api/v1/agents/{id}/subagents*
│   └── tenant_market.go          # /api/v1/market/tenant/*
├── application/service/
│   ├── persona_service.go
│   ├── expert_service.go
│   ├── skill_market_service.go
│   └── subagent_service.go
├── agent/
│   ├── persona/                  # 纯数据 + 渲染（16 型、28 题、模板）——零依赖可单测
│   ├── experts/                  # 专家目录扫描器 + manifest 类型
│   ├── skills/
│   │   └── skillhub/             # SkillHub 远程客户端（search/rankings/skillsets/download）
│   ├── subagents/                # 角色库 catalog + tenant 安装 + delegate 工具定义
│   └── trpc/                     # 【扩展】子智能体节点构建（复用现有 engine）
└── types/                        # 新 DTO + CustomAgentConfig 扩展
config/
├── experts/<id>/                 # 内置专家库（随代码分发）
├── subagents/library/{zh,en}/<division>/*.md   # 角色库（Octop 资产移植）
└── prompt_templates/persona_mbti.yaml          # MBTI 渲染模板
```

原则：persona / experts / subagents / skills.skillhub 是纯逻辑包（可单测、不 import service 层）；service 层管事务与 DB；handler 只做 DTO 转换。与 `chat_pipeline`、agent engine 的接线收敛在各 service 显式函数。

新表：`expert_installs`、`published_experts`、`published_skills`、`tenant_subagents`。迁移文件走现有 golang-migrate 体系。

## 4. MBTI 人格

### 数据层（internal/agent/persona/）

移植 Octop `mbti_profiles.py`（596 行纯数据）：16 型档案——code、中英文名、中文昵称、四轴维度（百分比限定 50–85）、6 行为场景（answer_style/casual_chat/conflict/creativity/emotion/planning，中英双语）、UI 主色。28 题题库（每维 7 题、A/B 强制二选一、中英双语）同样移植为 Go 数据文件。零依赖。

### 挂载

`CustomAgentConfig` 新增：

```go
PersonaMBTI  string `yaml:"persona_mbti" json:"persona_mbti,omitempty"`  // 如 "INTJ"，空=无人格
PersonaStyle string `yaml:"persona_style" json:"persona_style,omitempty"` // 叠加的自由人设文本
```

### 渲染链路

组装 system prompt 时，`PersonaMBTI` 非空则渲染 persona 段落**前置拼接**到 SystemPrompt（不覆盖用户主提示词）；`PersonaStyle` 非空时作为附加人设段落接在 MBTI 段之后。对 Octop 已知缺陷的修正：**行为场景按请求 locale 取中/英文字段**（Octop 只渲染英文）。

计分逻辑照搬：`pct = 50 + dominant/total*35`，clamp 50–85；平票取第一极；少于 20 题答 400。

### API（handler/persona.go，前缀 /api/v1/mbti）

- `GET /types`、`GET /types/{code}`、`GET /preview/{code}`（渲染预览）
- `GET /test/questions`、`POST /test/submit`
- `PUT /agents/{agentID}/persona`（应用/更换）、`DELETE /agents/{agentID}/persona`（移除）——失败明确报错（不学 Octop 的静默 auto_apply）

### 前端

Agent 编辑器内嵌 Personalization 区块：16 宫格选择器 + 四轴可视化 + 28 题 Modal（intro→questions→result 三阶段，含"仅供娱乐"声明）。类型数据从 `GET /mbti/types` 拉取，**前端不硬编码 16 型**（修正 Octop 前后端双份维护的问题）。

## 5. 专家（Experts）

### 概念映射

Octop 专家（文件种子进 agent workspace）→ WeKnora 专家（**生成 CustomAgent 的 i18n 模板包**）。建出的 agent 是普通 CustomAgent 行，`Config` 新增 `ExpertSource struct{ ExpertID, Source, Slug string }` 记录出身。`builtin_agents.yaml` 机制不动。

### 目录格式（config/experts/<id>/）

```
manifest.yaml   # i18n name/description/welcome、avatar/color、
                # agent_config 默认值（模型/温度/AllowedTools/KB 模式/skills_selection…）、
                # quick_prompts[]（title/prompt/icon/color）、引用的技能与子智能体清单
persona/*.md    # 拼接进 SystemPrompt 的人设文档
skills/<slug>/  # 可选：随专家分发的技能（标准 SKILL.md 格式）
agents/*.md     # 可选：随专家分发的子智能体定义
```

quick_prompts ↔ `QuestionSuggestions.Starters.Items` 直接映射，不造新机制。

### 数据流：从专家建 agent

`POST /api/v1/experts/{id}/instantiate`：

1. manifest agent_config 默认值 → CustomAgent 字段；persona md 拼接 → SystemPrompt；
2. quick_prompts → Starters；引用技能进 `SelectedSkills`（未安装则自动触发异步安装，202 + 进度）；引用的子智能体定义写入 `tenant_subagents` 并在 config 引用；
3. 返回 agentID。

### 三种来源、两张表

| 来源 | 存储 |
|---|---|
| 内置（随代码分发） | `config/experts/` 只读目录，启动扫描（不在 import 期做 IO，修正 Octop catalog.py:101 的问题） |
| SkillHub skillset 安装 | 下载物化为专家目录落租户存储；`expert_installs` 表记缓存位置与来源 slug |
| 租户内发布 | `published_experts` 表（发布者/版本/不可变快照引用），staging + 原子替换 |

### 发布（从现有 agent 导出）

白名单导出：system prompt/persona 段、技能引用、子智能体定义、快捷指令。**排除**：KB 绑定、模型密钥相关、SandboxConfigID、个人记忆（安装者自备）。

## 6. 技能市场

### 远程消费（skills/skillhub 客户端包）

- 端点：search / rankings / skillsets 列表（全量分页 + 进程内缓存 300s + 单飞 + 陈旧回退）/ download（`GET /api/v1/download?slug=`）
- ZIP 校验清单移植：zip-bomb 压缩比、symlink、加密条目、路径穿越、条目数/解压上限；剥单层包裹目录
- 下载后**桥接现有 `InstallSkill` 管线**（tenant_skill_install.go:63），不另造安装路径；复用现有 202 + 进度订阅 + reaper

API：`GET /api/v1/skills/market/search?q=`、`GET /market/rankings`、`POST /market/install`（slug + 可选 sandboxConfigID）；skillset 侧 `GET /api/v1/experts/market`、`GET /market/{slug}`、`POST /market/{slug}/install`（物化为专家 → §5 instantiate）。

### 租户内市场

- `published_skills` 表：把租户 catalog 技能标记发布（租户可见），他人安装直接从 catalog archive 走 InstallSkill，**不复制内容**
- `published_experts`：见 §5
- API：`GET /api/v1/market/tenant/{skills|experts}`、`POST /api/v1/market/tenant/{type}/{id}/install`

### 错误处理

SkillHub 不可达 → 返回 `stale=true` 的缓存数据或明确 503，不静默；安装失败沿用现有 install 错误流。

## 7. 子智能体

### 资产层

Octop 角色库整体移植 `config/subagents/library/{zh,en}/<division>/*.md`（zh 272 / en 217，丢弃 `TODO_TRANSLATE` 占位）。frontmatter `name/description/color/emoji/vibe/tools`，slug = 文件名 stem（zh/en 配对）。`divisions.json` 定义 16 分组（图标/颜色/排序）。

### 安装模型

新表 `tenant_subagents`（tenantID + slug + locale + md 内容 + 来源）。安装时按请求 locale 取角色正文，该 locale 缺失则回退 en（显式策略，解决 Octop 目录无回退与安装回退 en 的矛盾）。`CustomAgentConfig` 新增：

```go
Subagents []string `yaml:"subagents" json:"subagents,omitempty"` // 可委派的 slug 列表，空=禁用委派
```

### 委派执行（internal/agent/trpc/ 扩展）

1. `Subagents` 非空时主 agent 注册 `subagent_delegate` 工具，schema 只暴露 `{slug, goal, input_refs}`；权限字段（tenant/user/session/白名单/预算）服务端组装，严格解析沿用 `craft_delegate.go:44` 模式（DisallowUnknownFields、拒绝尾随输入、拒绝空白 goal）；
2. 按 slug 取角色：md 正文 → 子 agent system prompt；frontmatter `tools` 映射为 WeKnora 工具集 ∩ 主 agent AllowedTools 的交集白名单；
3. 子 agent 起独立 trpc session（独立消息历史），跑完整 ReAct 循环；预算：轮次 ≤ 8（可配）、token 上限可配；超限终止回传已产出；
4. 回传：截断摘要（8 KiB，对齐 craftDelegateSummaryLimit）进主上下文，完整轨迹落会话事件。

### DAG 编排剪裁（显式 YAGNI）

V1 不做专门 DAG 执行器。Octop `multi-agent-orchestrator` 的 depends_on 接力由专家 md 的编排约定 + 主 agent 串行多次调用 delegate 表达。trpc graph 原生支持图编排，后续需要时接口已预留。

### API

`GET /api/v1/subagent-catalog`（division/locale 过滤）、`GET /api/v1/subagent-catalog/{slug}`、`GET|POST /api/v1/agents/{agentID}/subagents`（列出/安装）、`DELETE /api/v1/agents/{agentID}/subagents/{slug}`。

## 8. 前端（apps/web，React 新站）

| 页面 | 内容 |
|---|---|
| `experts/` | 三 tab 专家市场（远程 skillset / 租户内发布 / 内置库）、详情、一键创建 agent |
| `market/skills/` | 技能搜索/榜单/安装进度（复用现有技能管理组件） |
| Agent 编辑器扩展 | Personalization 区块（MBTI）、子智能体管理 tab |

MBTI 与子智能体挂在 Agent 编辑器内，不设顶级路由（与 Octop 信息架构一致）。

## 9. 分阶段交付

| 里程碑 | 内容 | 依赖 |
|---|---|---|
| M1 MBTI | persona 包 + config 字段 + 渲染链路 + API + 编辑器区块 | 无 |
| M2 专家 | 目录格式 + 扫描器 + instantiate + 内置专家迁移 3~5 个试点 + 专家页 | M1（persona 渲染挂载点） |
| M3 子智能体 | 角色库移植 + tenant_subagents + delegate 工具 + trpc 子执行 + 编辑器 tab | 无（可与 M2 并行） |
| M4 技能市场 | skillhub 客户端 + 桥接安装 + skillset→专家 + 租户内市场 + 发布流 | M2 |

每个里程碑独立可验收（后端 API + 前端页面 + 测试全绿）。

## 10. 测试策略

- 纯逻辑单测：persona 渲染（16 型快照）、专家扫描器（golden 目录）、zip 校验（zip-bomb/symlink/路径穿越恶意样本集）
- 委派集成：fake LLM 驱动主→子完整循环（预算超限、白名单交集、结果截断各有断言）
- 市场客户端：httptest mock SkillHub（缓存回退、陈旧数据、超时）
- 资产迁移校验：脚本比对 Octop 原仓库（16 型数据/28 题/角色库 md 不丢不改）

## 11. 明确排除项（Out of Scope）

- agent 间互调（ask_agent、@提及、inbox mailbox）
- 多源 URL 技能导入（skills.sh/clawhub/github 白名单链路）
- 服务端 curl|bash 安装回退
- DAG 并行编排执行器（接口预留）
- Vue 主站同步
- MBTI 测试结果持久化（仅会话内返回，与 Octop 一致）

## 12. Octop 源码参考索引

| 能力 | 关键文件 |
|---|---|
| 专家 | `src/octop/infra/agents/experts/{catalog,market_creation,published_creation,publish,skillhub_market}.py`、`api/routers/experts.py` |
| 技能市场 | `src/octop/infra/skills/{skills_hub,skillhub_market,install,skill_packages}.py`、`api/routers/{skills,skill_packages}.py` |
| MBTI | `src/octop/infra/agents/{mbti_profiles,persona}.py`、`api/routers/mbti.py` |
| 子智能体 | `src/octop/infra/agents/subagents/{catalog,library}/`、`api/routers/subagents.py`、`docs/agent-delegation.md` |
