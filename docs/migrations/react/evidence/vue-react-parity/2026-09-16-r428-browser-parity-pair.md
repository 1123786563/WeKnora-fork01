# 2026-09-16 Round R428 — 并行切片 + 登录态双端浏览器对照证据

- 环境：React `:5181`、Vue `:5180`（均 vite dev）、真实后端 docker `:8080`（WeKnora-app/postgres/redis/docreader）。
- 账号：本地隔离库 parity 账号（tenant 10000，owner，self_serve）；双端同视口 1440×900、浅色主题、简体中文（React 端依赖本轮 locale 修复后才与 Vue 同语言）。
- 证据目录：`screenshots/r428-20260916/`（12 张，react-*/vue-* 成对）。

## 本轮代码切片（5 个并行切片 + 1 个审查代理，详见 progress Round R428）

| 切片 | 行 | 主要落点 |
|---|---|---|
| KB 列表域 | R009/N004/N005 | `App.tsx` 卡片/导轨/分享对话框图标与几何、`kb-list-icons.tsx` 新增 5 图标、`KnowledgeBaseShareDialog.tsx` |
| 图谱域 | N012 | `KnowledgeGraphPage.tsx`/`graph.ts` 节点半径公式、并发 worker 池、legend/-frontier/familiar 环、480px 抽屉；`graph.test.ts` +14 用例 |
| 成员审计 | R038 | 新增 `TenantAuditDrawer.tsx`（拖拽调宽+持久化+Esc+焦点陷阱）、`TenantMembersPanel.tsx` 接线；测试 portal 语义更新 |
| 技能域 | R033 | `SkillSettingsPanel.tsx` SettingDrawer header（icon+title+description）接入三个向导、SandboxPickList 残留 props 清理 |
| chat i18n | N020 | `chat-copy.ts` 五语言新增 thinking/deepThoughtCompleted/deepThoughtAlt/finish/rawOutputLabel（键先行，无消费者） |
| 共享 locale 修复 | R007/N001 及全局 | 新增 `apps/web/src/locale.ts`：`usePreferredLocale()` 读 `locale` 偏好（默认 zh-CN，Vue GeneralSettings 对齐）并响应 `weknora:locale-changed`；App/PlatformShell/InvitationInbox/AgentsPage 弃用 navigator.language 解析 |

审查代理修复：KnowledgeGraphPage TS null 缩窄守卫；TenantMembersPanel jsdom rAF 兼容；TenantAuditDrawer portal 焦点时序（mounted 依赖）；SkillSettingsPanel DrawerTitle 接入 + SandboxPickList backendLabel 残留；两个测试文件按 portal/受控语义更新。

## 门禁（静态/单元/组件层）

- 各域 scoped：KB 102/102、图谱 14/14、成员+审计 15/15、技能/MCP 61/61、chat views 53/53、shell 39/39+20/20+29/29（locale 修复后追加）、kb-list-anatomy 18/18。
- `pnpm typecheck:web` 0 error（locale 修复后复跑通过）。
- 集成：`pnpm test:web` 与 `pnpm test:shared` 结果见本轮 progress 条目（复审后含 locale 修复的复跑）。

## 浏览器实测发现（真实后端、登录态；差异均以 Vue 为基线，未修复项留待下轮）

1. **React 侧栏会话区持续显示英文 "Loading..."**（`react-kb-list.png` vs `vue-kb-list.png`）：Vue 同位置渲染日期分组会话列表（昨天/知识库检索讨论/Wails 验收会话 A/B）。疑似会话加载文案未走 i18n 或加载态未解除。→ 待修（R007/N018 域）。
2. **MCP 空态缺"添加服务"admin tile**（`react-settings-mcp.png` vs `vue-settings-mcp.png`）：Vue 空态渲染虚线 + 添加服务卡片，React 仅"暂无 MCP 服务"文本。与 2026-09-14 R024 "admin add tile" 记录不符，需复核触发条件。→ 待修（R024 域）。
3. **技能/审计空态缺插画与"暂无数据"**（skills/audit 两对截图）：Vue TDesign 空态含插画图标 + "暂无数据" 主行；React 只有说明文本。React 技能标题亦缺 Vue 的 ？ 帮助图标。→ 待修。
4. **KB 创建对话框**（`*-kb-create-dialog.png`）：React 缺 类型选择器外框（Vue 为带边框 radio-group 容器）、Wiki 知识库卡 "NEW" 徽标、描述 0/200 字数计数器。其余结构（section rail、必填星号、footer 顺序）已一致。→ 待修（N004 域）。
5. **图谱 surface 澄清**：React `?tab=graph`（KnowledgeGraphPage）是 Wiki 页面链接图谱；Vue KB 设置内"知识图谱配置"（GraphSettings.vue）是 LLM 实体-关系抽取配置，两者是不同概念（Vue 配置页明示）。本轮 vue-kb-graph.png 实为 Vue 配置页（数据库未启用告警）。**图谱 canvas 同条件对比需 wiki 型 KB fixture**，当前 fixture 为文档型，记 `blocked-fixture`。
6. locale 修复前后对照：修复前 React 导航为英文（New Chat/Knowledge bases/Agents/Shared Spaces），修复后与 Vue 一致（新对话/知识库/智能体/共享空间）。修复前截图见会话 artifacts，最终证据以修复后为准。

## 未纳入本轮

- `docs/superpowers/plans/mobile-workbench-progress.md` 及 apps/mobile、internal/handler/auth.go、packages/api-client/src/auth/oidc.ts 等处为外部并发进程改动，本round未审查、未提交。
- 移动端（apps/mobile）与本轮无关，未做平台验证。
