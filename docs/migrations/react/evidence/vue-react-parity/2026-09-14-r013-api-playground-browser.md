# R013 / N028 API Playground 浏览器证据（2026-09-14）

## 环境

- React：`http://localhost:5181/platform/settings?section=integration-api`
- Chrome 已认证现有 WeKnora 账号；视口使用当前桌面默认视口。
- Vue 同会话未提供登录态，因此本记录只证明 React 页面实际状态，不构成 Vue/React 同条件截图对照。

## 已观察状态

- API 集成页显示“打开 Playground”入口。
- 打开后实际呈现右侧 API Playground 抽屉，包含“请求配置 / 请求预览 / 运行结果”三个区段和关闭按钮。
- 默认问题为 `hello`；仅空间模式下“外部用户 ID”按 Vue 语义禁用，并显示对应提示。
- 请求预览显示 Session 与 Agent Chat 两步 JSON，`X-API-Key` 被替换为 `<API_KEY>`，未泄露任何密钥。
- 当前空间没有 API Key，运行按钮实际为 disabled；这是 Vue 门禁状态的浏览器验证。

## 未完成

- 当前空间没有 API Key 且智能体列表为空，无法在不创建外部权限凭证的情况下执行真实 Session/SSE 请求，记为 `blocked-env`。
- Vue 登录态、同数据截图/computed-style、真实后端成功/失败/权限矩阵及 Wails/native 仍未验收。
