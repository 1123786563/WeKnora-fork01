# 2026-09-14 en-US 语言维度全路由扫尾（双端，S00 语言维度扩展）

依据 2026-09-14-review-rows-audit.md Pass 5（当时仅 4 条核心路由双语核验）。本轮把 en-US 语言维度扩展到 **19 个表面**（登录/注册/KB 列表/智能体/共享空间/creatChat/设置 11 分区/集成 API），双端同条件（1440x900、locale=en-US、parity owner 账号、只读导航）。脚本 .parity-tools/enus-full-sweep.cjs；截图 enus-full-20260914/（38 张）。

## 结论：React en-US 无 UI 中文残留

| 表面 | Vue CJK | React CJK | 判定 |
|---|---|---|---|
| login / register | 9 / 9 | **0 / 0** | React 干净；Vue 自身泄漏 9 处中文（Vue 侧限制，登记不修 Vue） |
| kb-list / agents / organizations | 8-9 | 8-9 | 一致（样例为侧栏会话标题等用户数据） |
| settings 11 分区 | 8 | 8 | 一致 |
| creat-chat | 0 | 8 | React 的 CJK 为共享后端的会话标题（用户数据，如实展示）；Vue 同路由未渲染标题，非残留 |
| integrations-api | 8 | 8 | 一致 |

React 的 reactOnly 样本仅两类，均非文案泄漏：
1. creat-chat 的会话标题（用户数据，正确原样展示）；
2. settings-general 的「简体中文」「日本語」——语言下拉选项的 endonym（原生名）惯例，Vue locale 同值（en-US 亦为 简体中文/日本語，frontend/src/i18n/locales/en-US.ts language 块），Vue 截图时下拉未展开故未计入。

## 判定

- React en-US 语言维度在全 19 表面无中文残留；zh-CN 维度此前各切片已核。
- login/register 的 9 处 CJK 为 Vue 自身泄漏（Vue 侧限制，按 §一 不修改 Vue），React 忠实于 en-US 基线且更干净。
- S00 语言维度（zh-CN/en-US）在主要路由 + 设置分区上：React 全部通过。

## 门禁

本轮为只读 sweep + 文档，无代码变更。既有门禁基线维持（shared 444/444、web 856/856、typecheck 0、build ✓）。
