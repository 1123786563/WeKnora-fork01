# /login 首轮视觉对比证据（2026-09-12）

- 环境：Vue dev :5180、React web dev :5181，视口 1440x900，DPR 1，zh-CN，screenshots/login-{vue,react}.png。
- 结论：React 登录页与 Vue 基准存在整页级差异，状态记 `review → implementing`：
  - 缺少左侧品牌区（渐变背景、动画知识节点/连线、Agentic RAG 轮播卡片、标签 chips、宣传语）。
  - 缺少页头：logo、官方网站/GitHub 链接、语言切换下拉（Vue 支持 6 语言切换，React 全英文硬编码）。
  - 登录卡片缺少：标题/副标题 i18n 文案、首次使用提示条、密码可见性切换、功能勾选列表（多模态解析/混合检索/ReAct）、分隔线“首次使用 WeKnora?”、创建账户入口样式。
  - 注册模式、OIDC 按钮样式与位置、校验文案与错误展示样式均与 Vue 不一致（硬编码英文）。
- 业务层已确认 React 有：registrationConfig/oidcConfig 预取、登录/注册/OIDC 重定向（oidc.ts），但需与 Vue 的错误映射、邀请注册（/register invite token）、语言持久化逐项比对（待差异报告）。
- 下一步：依据登录页差异报告重建 React LoginPage（复用 packages/i18n 文案与 design-tokens），补回归测试后重截对比。
