# R013 / N028 API Playground 对齐证据（2026-09-14）

React API Playground 已接入 Vue `ApiIntegrationSettings.vue` 的三段式抽屉：请求配置、遮蔽密钥的双请求预览、Session/Agent Chat SSE/最终回答结果。停止、错误、无 API Key、无智能体、空问题和签名 Token 流程均有行为测试。

实现文件：`apps/web/src/integrations/ApiPlaygroundDrawer.tsx`、`apiPlaygroundModel.ts`、`apiPlaygroundSSE.ts`；入口接线在 `packages/views/src/integrations/page.tsx` 与 `apps/web/src/integrations/IntegrationsRoutePage.tsx`，样式在 `apps/web/src/styles.css`。

验证：模型测试 12/12；SSE 测试 12/12；抽屉交互测试 9/9；`pnpm typecheck:web` 通过；正式 `pnpm test:web` 757/757 通过。

尚未验收：当前 Vue 浏览器会话没有登录态，未完成同账号同数据截图/computed-style；未用真实后端 API Key 完成 Session、Agent Chat、权限拒绝矩阵；Wails/native 与 iOS/Android 未验证。因此 R013/N028 继续保持 `implementing`。
