# vue-react-parity 交付状态快照（2026-09-12，Round 14）

## 本地提交（本次验收工作累计）
- ff70ff4 docs(migration): record settings live baseline screenshots
- f6283c8 docs(migration): record platform shell integration
- fe85f30 feat(web/platform): add Vue-parity navigation shell around /platform routes
- ac8c763 feat(web/chat): implement chat stream must-fix parity items
- 646e4e2 docs(migration): fold settings/chat audit findings into parity matrix
- d313e95 docs(migration): record invitation and share-link live E2E
- b2dce81 docs(migration): record settings i18n port
- 47d6d17 feat(i18n): port all settings-surface copy (1038 keys x5 locales) into shared i18n
- de515ea docs(migration): record live-backend E2E environment and platform-shell gap
- 4591459 docs(migration): record round-8 progress and settings/chat diff intake
- 24dde16 refactor(i18n): move onboarding/tenant/invitation copy into shared i18n; auth pages consume shared formatMessage
- a5a2d13 docs(migration): record shared auth i18n migration
- 5093fd6 refactor(i18n): move auth-page copy into shared i18n package; LoginPage consumes formatMessage
- 7ee52a5 docs(migration): record KB list integration review and round-6 progress
- 39a9180 feat(web/kb): rebuild knowledge-base list page to Vue parity
- e0cc0a7 docs(migration): add node icon order fix evidence
- dc169d7 fix(web/auth): restore Vue node icon order in animated login background
- 381d878 docs(migration): add round-5 login interaction evidence
- 00e530a fix(web/auth): make carousel bullets visible; record live language-switch and carousel verification
- f240b85 feat(web/auth): add login stylesheet, 5-locale copy module, and carousel assets

## 页面状态一览（矩阵详见 vue-react-parity-matrix.md）
| 页面/入口 | 状态 | 已完成 | 主要开放项 |
|---|---|---|---|
| /login /register /join | implementing | 邀请 token 全链路、OIDC+next、autoSetup、完整校验、品牌视觉、5 locale 语言切换（live 实测） |locales.ts 迁移收尾、注册/邀请视觉细节、Wails |
| /onboarding/workspace | implementing | 创建工作区/邀请/策略重试/自动重定向（live API 实测） | 视觉（卡片/双列按钮）、MESSAGES 迁移 |
| /platform/knowledge-bases | review | 卡片网格/权限门控/删除确认/i18n 58 键/共享合并/pin/duplicate/?scope=（live 实测） | 折叠分组节头、收藏/最近 scope、上传进度面板、共享详情抽屉、视觉微调 |
| /platform/settings | implementing | settings.* i18n 1038 键就绪；功能必修实施中 | 全部必修项（审计 15 项）+ 抽屉/分组视觉形态 |
| /platform/chat/:chatid + creatChat | implementing | 必修 7 项（流取消/续传/steer 并发/审批持久/session_title/事件扩展/thinking 持久） | 分类型工具渲染、modified_args UI、Last-Event-ID、来源分组、时间戳/小地图/复制、空态建议 |
| /platform 全局外壳 | implementing | 侧栏/折叠/导航高亮/KB 过滤/用户区（live 实测） | 图标轨、图片 logo、会话列表并入、租户切换/铃/命令面板 |
| organizations / agents / integrations / embed | pending→审计中 | 审计子代理进行中 | 待审计结论后实施 |
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
