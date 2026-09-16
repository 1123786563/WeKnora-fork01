# 2026-09-14 R013 API Playground 真实 Session/SSE 验证 + 两处阻断级修复

## 背景

R013（/platform/integrations API 集成面）的 API Playground 此前仅做过结构级验证，运行路径从未在真实后端打通。本轮以受控 API Key + 内置智能体完成端到端验证，途中发现并修复两个真实缺陷。

## 缺陷一（阻断级）：Playground 抽屉整体不可点击

- 现象：抽屉打开、结构完整，但所有控件（运行/停止/输入）点击无效。elementsFromPoint 探针证实：设置弹窗遮罩 .wks-overlay（z-index 1100，fixed 全屏）位于抽屉之上，吞掉全部指针事件；抽屉本体 z-index 仅 60/59/61（组件内联样式）。
- 根因：React 抽屉虽 createPortal 到 body（ApiPlaygroundDrawer.tsx:402），但内联 z 层低于设置弹窗遮罩；Vue 侧 SettingDrawer teleport 到 body 后 z 高于遮罩，无此问题。
- **修复**：抽屉三层的内联 z 提升为常量 PLAYGROUND_OVERLAY_Z=1200 / PLAYGROUND_DRAWER_Z=1201 / PLAYGROUND_RESIZE_Z=1202（高于 wks-overlay 1100）。ApiPlaygroundDrawer.test 12/12 通过。

## 缺陷二（阻断级）：Playground 请求打到错误的 API 地址

- 现象：修复一后点击运行，结果区显示 HTTP 404；网络日志显示 POST http://localhost:5181/api/v1/sessions 404（dev 无 /api 代理，VITE_API_BASE_URL=http://localhost:8080 才是真实 API 地址；直连 :8080 同请求 201）。
- 根因：IntegrationsRoutePage/抽屉硬编码 apiBaseUrl=window.location.origin（:5181），而应用其余 API 全部走 VITE_API_BASE_URL（:8080）。
- **修复**：新增 apps/web/src/platform/api-base.ts（resolveApiBaseUrl：VITE_API_BASE_URL → 桌面注入 → 空串，与 main.tsx 同序）；IntegrationsRoutePage 增加 apiBaseUrl prop（默认 resolveApiBaseUrl()）并透传给抽屉与 IntegrationsPage；main.tsx 传真实 apiBaseUrl。EmbedPreviewModal 的 origin 用法保持不变（其语义为渠道预览地址，非 API 地址）。

## 真实 Session/SSE 链路验证（双端，真实后端 :8080）

前置数据（受控、已记录）：租户 API Key parity-playground（full_access，经 API 创建/重建）；内置智能体 builtin-quick-answer / builtin-smart-reasoning 的 config.model_id 绑定 builtin-llm-mock（内置 mock 模型，此前内置智能体均未绑模型、对话必失败——绑定即修复其可用性）。

| 步骤 | 结果（React 抽屉，X-API-Key 身份） |
|---|---|
| POST /api/v1/sessions | 201（user=api_tenant_key:10000:3，会话真实创建） |
| POST /api/v1/agent-chat/:id | 200（SSE 连接建立） |
| 终端错误渲染 | 「failed to get chat model: baseURL SSRF check failed …」正确显示在运行结果区（抽屉的终端错误分支生效） |

- Vue 侧同一后端状态：Playground 打开并运行（p2-result-vue.png；本脚本对 Vue 抽屉的文本锚点未适配，结构对照以既有切片证据为准）。
- 残余（环境配置，非 UI 缺陷）：内置 mock 模型 baseURL 为 IP 直连，被后端 SSRF 白名单拦截（需 SSRF_WHITELIST 或域名），最终回答文本双端均无法产生；该限制对 Vue/React 等效，已如实记录。

## 门禁

ApiPlaygroundDrawer.test 12/12 · typecheck:web 0 错误 · registry 6/6 · test:shared 444/444 · test:web 856/856（上轮基线）

## 截图

screenshots/playground-20260914/: p1-drawer-{vue,react}.png（入口/抽屉）、p2-result-{vue,react}.png（运行结果）、probe-after-run.png。
