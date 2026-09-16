# 2026-09-13 Wails 桌面运行时交互验收（登录 → 核心受保护页面全驱动，S14）

分支 `codex/react-multiclient`（worktree `.worktrees/react-multiclient`），无源码改动、无 commit。接续 `2026-09-13-wails-runtime.md`（登录页冒烟）之后的完整交互证据。

## 方法与环境

- 产物：`dist/WeKnora Lite.app`（当日构建、codesign 已验证、含全部切片）
- 启动：`open "dist/WeKnora Lite.app"` → pid 32116；`lsof -nP -a -p <pid> | grep LISTEN` → **127.0.0.1:60908**（Wails 资产服务器，端口每次启动变化，需每次重找）
- 驱动方式：playwright-core chromium 同源加载 `http://127.0.0.1:60908/`（与 `wails-runtime-smoke.cjs` 相同的已验证模式；窗口内容即同 origin 同构建，见 2026-09-12 交互说明——TCC 屏幕录制权限不可用，窗口直接截图被阻止）
- 脚本：`.parity-tools/wails-interaction-slice.cjs`（SPA 点击导航、DOM 断言、逐页截图、pageerror/API≥400 采集、`results.json`）
- 数据准备：`.parity-tools/wails-fixture-setup.cjs`（见下「环境限制」）

## 逐页结果（9/9 步 PASS，pageerror=0，API 4xx/5xx=0）

| # | 页面 / 动作 | 断言标记 | 结果 |
|---|---|---|---|
| 01 | 登录页 `/login?next=%2F` | 邮箱/密码输入、登录、创建账户、多模态文档解析✓、混合检索✓ | PASS |
| 02 | 登录提交 → `/platform/knowledge-bases` | 平台外壳、h1 知识库、导航 新对话/知识库/智能体/共享空间、**外壳会话列表分组头（今天）**、`Parity KB Demo` 卡片、范围筛选 全部/我创建的、用户区 parity-test@local.dev | PASS |
| 03 | KB 卡片动作「设置」→ `/knowledgeBase/:id` 文档页 | 面包屑 **知识库 › Parity KB Demo › 文档**（.document-breadcrumb）、info 按钮、**设置齿轮 kb-settings-button**、搜索框 placeholder 搜索文档名称…、筛选行、文档行「Parity 介绍文档.md」（Processing） | PASS |
| 03b | 面包屑「知识库」crumb 返回 | 返回 KB 网格、外壳会话列表仍在 | PASS |
| 04 | 导航点击 → `/platform/agents` | 页头 **智能体** + 创建智能体按钮、**内置 section 4 卡**（快速问答/智能推理/维基问答/数据分析师，后端 service 层内置合成）、我创建的 2 卡、左侧 icon rail、外壳会话列表 | PASS |
| 05 | 导航点击 → `/platform/organizations` | 页头 **共享空间** + 副标题、创建/加入图标按钮（title/aria-label 创建共享空间、加入共享空间）、org 卡片（全部/我创建的/我加入的 tabs）、外壳会话列表 | PASS |
| 06 | 用户菜单 → 个人设置 → 设置抽屉左导航「模型管理」→ `/platform/settings?section=models` | 抽屉内 **h2 模型配置** + 副标题、**添加模型 tile**、模型类型 tabs 全部(3)/对话(2)/Embedding(1)、模型卡片、内置模型说明、模型测试入口；✕ 关闭抽屉（modal drawer，Vue 对齐）返回原页 | PASS |
| 07 | 导航「新对话」→ `/platform/creatChat` | composer `textarea#wk-chat-draft` placeholder **直接向模型提问**、外壳会话列表（今天分组、6 行）、**无双份侧栏**（`.plat-shell__sessions`=1，outlet 内 session-group=0） | PASS |
| 08 | 外壳会话行点击 → `/platform/chat/:id` | 会话路由、composer 仍在、激活行 `aria-current="page"` 高亮、外壳会话列表唯一 | PASS |

截图：`screenshots/wails-interaction-slice/01-login.png … 08-chat-session.png`（9 张）+ `results.json`（全量断言 JSON）。

## Gap / 差异登记（未修代码，如实记录）

1. **用户菜单渲染原始 i18n key**：外壳用户下拉第一项显示字面量 `general.personalSettings` 而非「个人设置」（`userMenuItems: ["general.personalSettings","退出"]`）。产品缺陷，待修。
2. **KB 卡片标题点击进 KB 设置而非文档列表**：卡片标题按钮走 `openKbSettings` → `/knowledgeBase/:id/settings`；文档页入口是卡片动作条上的「设置」按钮（`openCard` → `/knowledgeBase/:id`）。按钮语义与 Vue（点卡片进文档列表）不一致，交互差异待确认。
3. **`modelsReady` 判定疑似永假（潜在缺陷，本例未触发）**：`App.tsx` 以 `model.type === 'llm'` 判定，而后端 `/api/v1/models` 类型枚举为 KnowledgeQA/Embedding/Rerank/VLLM/ASR，无 `'llm'`。未初始化 KB 的卡片点击会被永久门控到 `/platform/settings`。本例 KB 已初始化（summary+embedding model id 齐全），未受影响。
4. **文档解析停滞 Processing**：fixture 使用的 mock 模型端点不可达，文档行停留 Processing（列表/面包屑形态不受影响）。

## 环境限制（如实登记）

- **昨天分组无法在本实例复现**：`.app` 内置 sqlite 为全新库，所有 fixture 会话均为今天；把会话时间回拨到昨天需直写 `~/Library/Application Support/WeKnora Lite/data/weknora.db`，本会话沙箱仅 workspace-write，sqlite 写入被拒（readonly database (8)）。会话分组机制（已置顶/今天/昨天/近7天…）与 web 批次一致，本批证据到「今天」分组 + 行渲染 + 行点击路由。
- **Lite 全新库无 parity 数据**：登录 parity-test@local.dev 首次 401（库内仅 admin@weknora.local）。经应用自身 API 完成数据准备（`.parity-tools/wails-fixture-setup.cjs`）：注册 parity-test@local.dev / Parity123456!（自建空间 tenant 2）→ 建 KnowledgeQA/Embedding mock 模型 → 建「Parity KB Demo」文档库（summary+embedding 初始化）→ manual 文档 → 会话 ×6 → 「Parity 共享空间」→ 2 个用户智能体。均为运行时数据，不涉及源码。
- 设置页为 modal drawer（遮罩覆盖外壳），抽屉关闭后回到下层页面——脚本先取证后关闭。

## 结论

核心受保护页面（KB 列表 / KB 文档详情 / 智能体 / 共享空间 / 设置-模型配置 / 新对话 / 会话聊天）在桌面 app 运行时内全部按 web 验收批次记录的解剖渲染：外壳会话列表全局存在且聊天页无重复侧栏、面包屑/齿轮/添加模型 tile/内置 section/直接向模型提问 composer 逐项到位。登记 Gap 1–3 供后续切片修复。
