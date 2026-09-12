# vue-react-parity 交付状态快照（2026-09-12，Round 14）

## 本地提交（本次验收工作累计，至 c5599c6）
- 提交序列：188cd75 → c5599c6（含 kb 重建 39a9180、chat 必修 ac8c763、平台外壳 fe85f30、settings 必修 d1aa583、auth 三切片 4baeb6b/5e5a04c/24dde16、i18n 三域 47d6d17/5093fd6/5b4bd51 及历次证据文档提交）。完整清单见 git log。

## 页面状态一览（矩阵详见 vue-react-parity-matrix.md）
| 页面/入口 | 状态 | 已完成 | 主要开放项 |
|---|---|---|---|
| /login /register /join | implementing | 邀请 token 全链路、OIDC+next、autoSetup、完整校验、品牌视觉、5 locale 语言切换（live 实测） |locales.ts 迁移收尾、注册/邀请视觉细节、Wails |
| /onboarding/workspace | implementing | 创建工作区/邀请/策略重试/自动重定向（live API 实测） | 视觉（卡片/双列按钮）、MESSAGES 迁移 |
| /platform/knowledge-bases | review | 前项 + 折叠分组节头完成（01bccd0，live 验证）；未初始化路由 live 验证 | 收藏/最近 scope、上传进度面板、共享详情抽屉、视觉微调 |
| /platform/settings | implementing | 必修 8 项完成（角色门控/9 sections/envvars/模型选择器/模型锁/密码策略/脏检查/popstate，d1aa583；live 验证） | 抽屉/分组视觉形态、旧面板英文清扫、GeneralSettings 本地偏好、tenant 删除 |
| /platform/chat/:chatid + creatChat | implementing | 必修 7 项完成（ac8c763）；creatChat live 基线 | 分类型工具渲染、modified_args UI、Last-Event-ID、来源分组、时间戳/小地图/复制、空态建议 |
| /platform 全局外壳 | implementing | 侧栏/折叠/导航高亮/KB 过滤/用户区（live 实测，fe85f30）；menu.* i18n 迁移（5b4bd51） | 图标轨、图片 logo、会话列表并入、租户切换/铃/命令面板 |
| organizations / agents / integrations / embed | implementing | 审计完成（必修清单见矩阵）；live 基线截图入库；API-tab live 缺陷已转告实施代理 | 实施子代理进行中 |
| 移动端 / Wails / 真后端全页面 E2E | pending | 环境已具备（后端 :8080 + 双 dev server + playwright 脚本） | 逐页截图矩阵、原生端验收 |

## 环境复现
1. cd .worktrees/react-multiclient && make dev-start（docker 基础设施；.env 由 .env.example 改 DB_HOST=localhost、REDIS_ADDR=localhost:6379）
2. make dev-app（后端 :8080）
3. pnpm --filter @weknora/web exec vite --port 5181（需 VITE_API_BASE_URL=http://localhost:8080）
4. cd frontend && npm run dev -- --port 5180
5. 测试账号：parity-test@local.dev / Parity123456!（隔离本地库）；截图脚本 .parity-tools/*.cjs

## 共享层新增（均有后端/基线依据）
- api-client：acceptInvitationByToken（routes_auth_tenant.go:175）、tenants.admin.create（tenant.go:89-90）、knowledgeBases.togglePin/duplicate（routes_knowledge.go:224/237）、continueStream 接线。
- i18n：authMessages 73 键、onboardingMessages 24 键、knowledgeListMessages 58 键、settingsMessages 1022 键（各 ×5 locale，字节级移植，键集一致性测试）。
- domain：canManageKBCard/canDuplicateKBCard/isKnowledgeBaseInitialized/mergeAllScopeKnowledgeBases、password-policy（settings 实施中）、assistantMessageExtras、resume/steer-submit 决策模块。
