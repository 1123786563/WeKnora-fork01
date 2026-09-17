# Vue → React 逐页验收进度账本（vue-react-parity-progress）

## 2026-09-15 Round R360 — Hidden app routes and narrow-screen settings repair

- Vue 路由审计发现此前账本遗漏四个应用连接入口：`/platform/apps`、`/platform/apps/connections`、`/platform/apps/authorization/:id`、`/platform/apps/actions/:id`。React 已补齐 route matching、shell dispatch、目录/连接/授权轮询/动作审批页面骨架，并保持 `/api/v1/apps/*` 的租户作用域与 expected-version 写入边界；路由清单已同步更新。
- 按 Vue 1100px/85vh 组织设置弹窗基线修正 React 几何，并在 <=720px 隐藏固定侧栏、表单单列化；设置 inline edit 控件改为可换行、最大宽度受容器约束。
- 证据：应用隐藏路由静态映射与 Web 类型检查/回归待本轮最终命令确认；这些页面仍需真实浏览器同视口、真实后端权限/授权/动作状态验证，暂保持 `review`，不因入口可达或构建通过而标记 accepted。

## 2026-09-15 Round R359 — React 多页面并行对齐与共享组件收敛

- 在目标 worktree 中并行完成认证入口、Documents、Knowledge Base editor/share/activity/settings、Integrations、Tenant Members 及设置状态模型的源码与交互契约补齐；共享 UI 以 Tailwind + shadcn/Radix 为底层，保留项目 wrapper、Vue 派生 token、focus/Escape/outside-close、loading/invalid/portal 语义。未修改 Vue。
- 补齐并验证登录解析、知识库设置纯函数、activity stale-response generation guard、viewer 权限 fixture、成员邀请入口，以及 Button/Input/Select/Dialog/Sheet 的语义状态和 SSR 契约。并行代理报告与审查记录见 `artifacts/parity-agent-*.md`。
- 证据：`pnpm test:shared` 485/485、`pnpm test:web` 1038/1038、`pnpm typecheck:web`、`pnpm build:web`、`git diff --check` 均通过；这是静态、单元/集成和构建证据，不等同于 Vue 浏览器验收。
- 当前仍为 `review`：Vue 与 React 同条件逐页截图/computed-style、认证全状态、真实后端 mutation、Wails/桌面渲染和原生设备证据受当前环境阻塞（Vue :5180、React :5181、后端 :8091 不可用/无认证会话）。因此本轮不得标记为最终 parity accepted；剩余差异与阻塞继续保留在 inventory/review 台账中。

## 2026-09-15 Round R326 — Chat/Embed/KB upload 并行修复

- 修复 Chat 消息 bookmark 宿主回调启用语义、Embed 上传权限 conjunction、KB 文档取消/摘要状态文案与 WebSearch provider card anatomy/动作；均未修改 Vue，Embed 保持独立入口。
- Chat views 75/75、Web chat/documents/websearch focused 45/45、Embed 8/8、Documents 129/129、Web full 933/933、mobile 191/191、typecheck/diff check 已通过；provider card 证据：`evidence/vue-react-parity/2026-09-15-r325-websearch-provider-cards.md`。
- 仍未闭环：认证浏览器全状态、真实后端 mutation、Wails/native 逐功能验收及 Embed 受保护资源预览。

## 2026-09-15 Round R324 — Chat/文档/Platform/Settings/Mobile 并行批次

- 完成 KB 文档详情标题行、Chat 附件/会话来源本地化、Platform 租户切换菜单关闭、Ollama 可用性布局、移动端文档递归查询条件等有限差异修复；未改 Vue，移动端保持原生组件体系。
- Web focused batch 12/12、Platform 141/141 + 6/6、Documents/Knowledge 142/142、Mobile 191/191、Web typecheck、mobile typecheck、diff check 均通过；证据：`evidence/vue-react-parity/2026-09-15-r324-runtime-surfaces.md`。
- 认证浏览器全状态、真实后端 mutation、Wails/native 编译运行和 release 平台证据仍未闭环。

## 2026-09-15 Round R323 — Login typography/controls live recheck

- 同一 1440×900、zh-CN 视口重新采集 Vue/React 登录页；标题、标签和 40px 输入控件按 Vue 实测 token 修正，React/Vue 表单底部几何已对齐到同一 y=659.30。
- Login 专项 7/7、`pnpm typecheck:web`、`git diff --check` 通过；截图与 computed geometry：`evidence/vue-react-parity/2026-09-15-r323-login-typography-live.md`。
- 仍未覆盖全部 auth 状态、locale/theme/responsive、OIDC/真实后端、Wails/native 和移动端验收。

## 2026-09-15 Round N+105 — 并行页面 parity 批次

- 代理分工完成 Login、Knowledge Base list、Agents、Organizations、Wiki、FAQ、General settings 的最小差异修复；Integrations/Embed 完成专项审计但无代码差异。
- 受影响专项测试与最终 Web 全量测试均通过；最终 `925/925`、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n018-parallel-pages.md`。
- 本批次仍仅具 Vue 源码/静态对照和 React 单测/DOM 证据；认证浏览器截图/computed style、真实后端、Wails/native、移动端及生产验收未完成，不能将这些页面标记为最终通过。

## 2026-09-15 Round N+104 — N016 MCP 数字输入 chrome

- 对照 Vue number-input，React MCP 表单在作用域内移除 Firefox/WebKit 数字微调器，并将单位后缀设为 Vue 派生的弱化色；未新增全局 input 选择器。
- MCP 专项 15/15、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-number-spinner.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+103 — N016 MCP 高级数字输入行为

- 对照 Vue `onAdvancedNumberBlur`，React timeout/retry 字段允许暂态空值，blur 时按默认值与 min/max 归一化，payload 再保留最终数值保护；单位后缀和 API 映射不变。
- MCP 专项 15/15、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-advanced-number-behavior.md`。
- N016 仍需认证浏览器键盘/blur capture、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+102 — N016 MCP 表单控件 chrome

- 对照 Vue TDesign input/select 基线，将 React MCP 表单控件限定为 32px 高、4px 圆角、9px/7px 内边距及匹配边框背景，避免 shadcn 默认几何漂移。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-input-chrome.md`。
- N016 仍需认证浏览器 computed-style/截图、键盘/focus capture、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+101 — N016 MCP 必填字段标记

- 对照 Vue `.required::before`，为 React MCP name、transport、service URL 和 usage instructions 补齐红色 `*` 与 4px 间距，保留原生 required 校验。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-required-markers.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+100 — N016 MCP OAuth 区域 layout

- 对照 Vue `McpServiceDialog.vue`，将 React OAuth 区域调整为独立 label、状态/操作行和 hint 行，授权/撤销按钮收敛到 28px compact 尺寸，保留授权、撤销、错误和 loading 语义。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-oauth-layout.md`。
- N016 仍需认证浏览器 OAuth layout/callback、真实授权流程和 Wails/native 证据。

## 2026-09-15 Round N+99 — N016 MCP segmented transport/auth 控件

- 对照 Vue `source-options`，将 React MCP transport 与 auth 两处原生 select 替换为可访问 radio button 组，补齐 28px 控件、active 边框/背景、图标及 hover/focus 状态，保留原 draft/payload 逻辑。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-segmented-controls.md`。
- N016 仍需认证浏览器交互/computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+98 — N016 MCP 自定义请求头操作控件

- 将自定义请求头新增/删除操作对齐 Vue：新增使用绿色文字型小按钮和 add SVG，删除使用 24px 方形 icon-only 控件并保留无障碍名称，避免默认实心按钮造成视觉拥挤。
- MCP 专项 14/14、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n016-mcp-header-actions.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+97 — N016 MCP 字段标签 typography

- 将 MCP 编辑器字段标签样式收敛到专用 `.wk-mcp-form`：13px/500、1.4 行高、6px 间距，并排除 checkbox 标签，避免通用 label 选择器污染嵌套控件。
- MCP 专项 14/14、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n016-mcp-label-typography.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+97 — N016 MCP 编辑表单 label

- 对照 Vue `.form-label`，将 React MCP 普通字段 label 收敛为 13px/500/1.4、6px 间距，并排除 checkbox，移除表单级 semibold 默认覆盖。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-form-labels.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+96 — N016 MCP 自定义请求头表单项

- 对照 Vue `McpServiceDialog.vue`，移除自定义请求头嵌套卡片边框，将新增入口移到标签行右侧，并保持 key/value 编辑与删除回调不变。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-custom-headers.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+95 — N016 MCP 编辑区段 anatomy

- 对照 Vue `McpServiceDialog.vue`，移除 React MCP 编辑器外层配置区的圆角卡片边框，统一为 Vue 的 12/16px 留白、12px 区内间距、底部分隔线和标题绿色竖条。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-editor-sections.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 成功/失败流程和 Wails/native 证据。

## 2026-09-15 Round N+94 — N016 MCP 抽屉重复关闭控件

- 移除 React MCP 自定义 header 中 Vue 不存在的重复“关闭”文字按钮，保留底部取消和抽屉自身关闭行为，减少 header 拥挤。
- MCP 专项 14/14、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-header-close.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+93 — N016 MCP 编辑抽屉 resize

- 对照 Vue `SettingDrawer` 补齐 MCP 抽屉 680px 默认、560px 最小、920px 最大、viewport clamp、同名 localStorage 持久化和 overlay 内垂直 resize separator；拖拽期间同步 cursor/user-select 清理。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-drawer-resize.md`。
- N016 仍需认证浏览器拖拽/computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+92 — N016 MCP 步骤条 connector

- 对照 Vue `McpServiceDialog.vue` 移除 React 步骤条额外的文本箭头，仅保留第一步内部的 1px connector 线，避免字体和宽度漂移；步骤切换与保存行为不变。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-stepper-connector.md`。
- N016 仍需认证浏览器 computed-style/截图、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+91 — N016 MCP 编辑抽屉 header

- 对照 Vue `McpServiceDialog.vue`，React 编辑抽屉补齐 transport 图标、标题/副标题横向 header、独立步骤条及 SSE/HTTP Streamable 颜色；保留关闭、校验、步骤和保存行为。
- MCP 专项 14/14、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-drawer-header.md`。
- N016 仍需认证浏览器 computed-style/截图、抽屉 resize 行为、真实 MCP 流程和 Wails/native 证据。

## 2026-09-15 Round N+90 — N016 MCP 卡片 footer metadata

- 将 MCP 卡片 footer 对齐 Vue：工具/同步信息与 transport 类型归入左侧 metadata 组，启用状态保持独立右侧控件，stale 状态保留警告色。
- MCP 专项 13/13、Web 全量 911/911、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n016-card-footer-metadata.md`。

## 2026-09-15 Round N+88 — N016 MCP 卡片图标与底栏节奏

- 对照 Vue MCP 卡片结构，将服务图标移至卡片主内容左侧并取消底栏顶部重复分隔线，保留工具数量/同步状态和编辑入口行为。
- MCP 专项 13/13、Web 类型检查、`git diff --check` 通过。
- N016 仍需真实同步 MCP 服务、Wails/native 与完整浏览器 computed-style 证据。

## 2026-09-15 Round N+89 — N007 上传弹窗关闭按钮

- 记录专用上传弹窗关闭按钮的 Vue 几何：32px 方形、右上 20px、20px 图标、6px 圆角和次级背景色；样式限定在上传弹窗作用域内。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-dialog-close-control.md`。
- N007 仍需认证浏览器 computed-style/截图、真实上传后端和 Wails/native 证据。

## 2026-09-15 Round N+87 — N007 文件侧栏 header 顺序

- 将标题、文件数量/继续添加操作和目的地面包屑归入同一文件侧栏 header，并把文件列表放在 header 之后，恢复 Vue 的信息层级与垂直节奏。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-sidebar-header-order.md`。
- N007 仍需认证浏览器 computed-style/截图、真实上传/目录后端和 Wails/native 证据。

## 2026-09-15 Round N+87 — N007 上传确认弹窗关闭控件尺寸

- 对照 Vue 关闭控件，React 专用上传确认弹窗固定右上角控件为 32×32px、20px 图标、6px 圆角、居中布局和次级表面背景，避免继承通用 shadcn 按钮尺寸。
- 上传确认专项 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-dialog-close-control.md`。
- N007 仍需认证浏览器 computed-style/截图、真实上传后端和 Wails/native 证据。

## 2026-09-15 Round N+86 — N007 上传确认弹窗标题与关闭按钮

- 对齐 Vue 三栏弹窗头部：标题移入 220px 文件侧栏的 56px header，通用标题行隐藏；关闭按钮固定在右上角 20px，保持 32px 控件尺寸。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-dialog-header.md`。
- N007 仍需认证浏览器 computed-style/截图、真实上传后端和 Wails/native 证据。

## 2026-09-15 Round N+85 — N007 更多处理选项切换器

- 将 chunking 的“更多处理选项”切换器对齐 Vue：绿色文字、6px 图标间距、垂直 padding、hover 下划线、键盘焦点环和展开时旋转的 SVG chevron。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-more-options-toggle.md`。
- N007 仍需认证浏览器 computed-style/截图、真实上传后端和 Wails/native 证据。

## 2026-09-15 Round N+85 — N007 上传确认弹窗标题与关闭按钮位置

- 对照 Vue `UploadConfirmDialog.vue`，专用 React Dialog 将标题移入 220px 文件栏 56px header，并将关闭按钮改为 modal 右上角绝对定位，避免通用 Dialog header 占据三栏布局空间。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-dialog-header.md`。
- N007 继续保持 `implementing`：认证浏览器 post-change screenshot/computed-style、真实后端与 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+84 — N007 上传目的地面包屑

- 将上传确认弹窗目的地选择器改为 Vue 风格的无边框文本面包屑：弱化标签、突出当前目录、保留展开标记，并增加窄屏省略与键盘焦点反馈。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-destination-crumb.md`。
- N007 仍需认证浏览器 responsive/computed-style、真实目录/上传后端和 Wails/native 证据。

## 2026-09-15 Round N+84 — N007 目的地面包屑

- 对照 Vue `UploadConfirmDialog.vue`，目的地触发器从带边框 pill 修正为无边框 inline 控件，使用 4px gap、12px/18px 字体、路径单行省略、muted label/caret 和 brand hover/focus 状态；目的地弹层与新建文件夹行为保持不变。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-destination-crumb.md`。
- N007 继续保持 `implementing`：认证浏览器 post-change computed-style/截图、真实后端与 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+83 — N007 分节导航 hover/focus 状态

- 为纵向上传配置导航项补齐 Vue 对应的 hover 背景、focus-visible 项目绿色 focus ring 与无障碍键盘反馈，避免依赖浏览器默认按钮焦点。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；N007 仍需认证浏览器 responsive/computed-style、真实后端与 Wails/native 证据。

## 2026-09-15 Round N+82 — N007 分节导航响应式行为

- 对照 Vue 800px media query，移除 `UploadSectionNav` 内联桌面布局，统一由 CSS 控制；窄屏切换为不换行横向滚动、自动宽度导航项并清除桌面行距。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-section-nav-responsive.md`。
- N007 继续保持 `implementing`：认证浏览器 responsive capture、真实后端与平台证据仍未闭环。

## 2026-09-15 Round N+82 — N007 上传配置内容区 padding

- 对照 Vue `.content-wrapper`，React 配置内容区从统一 20px 修正为桌面 `22px 32px 28px`，并保留 800px 下的 16px 覆盖。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity-2026-09-15-n007-config-padding.md`。
- N007 仍需修改后认证浏览器 computed-style/截图、真实上传后端及 Wails/native 证据。

## 2026-09-15 Round N+81 — N007 上传配置区段导航证据

- 记录上传配置左侧导航的 Vue 纵向菜单几何：216px 侧栏、38px 行高、状态副标题、激活背景与问题色态；单一 `nav` landmark 保持不变。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-section-nav.md`。
- 认证浏览器 post-change 截图/computed-style、真实上传/解析后端和 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+80 — N007 上传确认文件列表与区段导航

- 上传确认文件/URL 行采用 Vue 的 24px 图标槽、12px 文件名、11px 元数据、溢出省略和 22px 移除按钮；区段导航改为纵向 38px 行高、状态副标题与激活/问题色态，减少弹窗信息拥挤。
- 上传确认专项 37/37、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-file-list.md`。
- N007 仍需认证浏览器 computed-style、真实上传/解析后端和 Wails/native 证据。

## 2026-09-15 Round N+80 — N007 上传配置分节导航

- 修正 React 上传配置导航从横向 pill 改为 Vue 216px 侧栏纵向菜单：38px 行高、9px/10px 内边距、2px 行距、13px 标签、12px 状态、无边框及选中背景，并保留 issue/status/截断语义。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-section-nav.md`。
- N007 继续保持 `implementing`：修改后认证浏览器截图/computed-style、真实上传后端及 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+79 — N007 上传来源入口图标

- 对照 Vue 上传来源下拉入口，React 将平台相关的全角加号替换为内联文件加号 SVG，避免按钮几何和字形渲染漂移；菜单行为与文件/文件夹/URL 入口保持不变。
- 上传确认专项 37/37、Web 全量 911/911、Web 类型检查、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-r323-upload-source-icon.md`。
- N007 仍需认证浏览器 computed-style、真实上传/解析后端和 Wails/native 证据。

## 2026-09-15 Round N+79 — N007 上传确认文件列表

- 对照 Vue `UploadConfirmDialog.vue`，React 文件/URL 行补齐 24px 图标槽、12px/1.35 文件名、11px/1.3 元信息、8px 间距、ellipsis 截断及 22px icon-only 删除控件的 hover/focus 状态；保留 React 每文件上传状态和原有回调。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-file-list.md`。
- N007 继续保持 `implementing`：真实后端上传、同条件浏览器截图/computed-style 与 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+78 — N016 MCP 卡片标题与新增入口字体

- 对照 Vue `McpSettings.vue`，服务名称显式固定为 14px/600/20px，新服务入口标签固定为 13px/500/1.4，避免页面和 shadcn 默认标题字体继承漂移。
- MCP 测试 13/13、Web 类型检查、`git diff --check` 通过；N016 仍需 post-change 浏览器 computed-style、真实后端与 Wails/native 证据。

## 2026-09-15 Round N+77 — N016 MCP 标题 computed-style 修正

- 在 Vue 与 React 同一路由真实浏览器对照中确认字号/间距一致、标题与描述颜色不一致；React 已按 Vue computed styles 修正为 `rgba(0,0,0,.9)` 与 `rgba(0,0,0,.6)`。
- MCP 测试 13/13、Web 类型检查、完整 Web 回归 911/911 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-header-computed-style.md`。
- 认证 React 页面 reload 后未保留可用内容，故 post-change 浏览器 computed-style 仍开放，不能标记 N016 完成。

## 2026-09-15 Round N+76 — N007 上传配置导航语义修正

- 复核 N007 弹窗骨架后移除外层重复 `<nav>`，保留 `UploadSectionNav` 的唯一语义导航，避免嵌套 landmark 与滚动样式冲突。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；N007 继续保持 `implementing`，真实浏览器/后端/平台证据仍开放。

## 2026-09-15 Round N+75 — N005 上传遮罩插画与文案

- 对照 Vue `upload-mask.vue`，React 上传遮罩接入原始 `upload-mask.svg`，补齐 164×162 插画、24px/26px 标题和两条 12px 格式说明；文案改用共享 i18n，保留现有百分比进度和无障碍 progressbar 语义。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n005-upload-mask.md`。
- N005 继续保持 `implementing`：同条件浏览器截图、真实后端上传、Wails/native 证据仍未闭环。

## 2026-09-15 Round N+74 — N016 MCP 服务卡片排版

- 对照 Vue `McpSettings.vue` 服务卡片，React 补齐卡片纵向撑满、正文 12px/18px、描述两行截断和 footer 底部对齐；通过项目设置样式文件集中承载，不改变 MCP API 或权限行为。
- MCP 聚焦测试 13/13、`pnpm typecheck:web`、`git diff --check` 通过；证据：`evidence/vue-react-parity/2026-09-15-n016-mcp-service-card.md`。
- N016 继续保持 `implementing`：同条件浏览器 computed-style、已同步 MCP 服务状态、真实后端成功/失败流程及 Wails/native 证据仍未闭环。

## 2026-09-15 Round N+73 — N007 上传确认弹窗外层布局

- 对照 Vue `UploadConfirmDialog.vue` 的 92vw/1160px、85vh/750px、220px 文件栏、216px 配置导航、独立配置滚动区和 800px 响应式堆叠行为，React 上传确认流程新增专用 Dialog 骨架与集中式样式覆盖；上传状态机、字段和交互回调保持不变。
- 聚焦 upload-confirm 测试 37/37、`pnpm typecheck:web`、完整 Web 测试 911/911 通过；证据：`evidence/vue-react-parity/2026-09-15-n007-upload-dialog-shell.md`。
- N007 继续保持 `implementing`：同条件 Vue/React 浏览器截图与 computed-style、真实上传/解析后端、Wails/native 运行时证据仍未闭环。

## 2026-09-15 Round R320 — Knowledge-base share dialog action parity

- 对齐 Vue 共享知识库对话框：标题不再拼接知识库名，面板宽度为 520px，已共享列表的设置/取消共享操作改为仅图标，并保留本地化 `aria-label/title`。
- 新增共享关闭图标，聚焦测试 13/13、Web 911/911、Web typecheck 和 diff check 通过；真实认证双端截图与后端共享/取消共享仍开放。
- 证据：`evidence/vue-react-parity/2026-09-15-r320-share-dialog-actions.md`。

## 2026-09-15 Round R321 — Share dialog organization avatar parity

- 新增 Web 项目级 `SpaceAvatar`，按 Vue `SpaceAvatar.vue` 对齐 small 头像的 22px 尺寸、方形圆角、渐变哈希、Emoji 和装饰图形，并替换共享对话框的选中项、组织选项和已共享行头像。
- 聚焦共享对话框 13/13、Web 911/911、Web typecheck 和 diff check 通过；认证双端截图与真实后端共享/取消共享仍开放。
- 证据：`evidence/vue-react-parity/2026-09-15-r321-share-space-avatar.md`。

## 2026-09-15 Round R322 — Share dialog footer structure

- 将“已共享到 (n)”操作移入 Vue `.share-actions` 对应的表单 footer，位于左侧，取消/确认按钮保持右侧并由 flexible spacer 分隔。
- 聚焦共享对话框 13/13、Web 911/911、Web typecheck 和 diff check 通过；认证双端截图与真实后端共享链路仍开放。
- 证据：`evidence/vue-react-parity/2026-09-15-r322-share-footer-structure.md`。

## 2026-09-14 Round N+35 — KB 设置 i18n 基础移植（kbSettings/knowledgeEditor 命名空间入共享 i18n）

- 新增 packages/i18n/src/generated/kbSettingsMessages.ts（59 键 ×5 locale，parser/storage/vectorStore）与 knowledgeEditorMessages.ts（586 键 ×5 locale，basic/chunking/indexing 标签）；源为 Vue locales 字节级，en-US/ja-JP 缺键按 Vue fallbackLocale=zh-CN 的渲染值补齐；两域均入 index.ts 合并链。
- 动机：Round N+34 发现的知识库设置面板（KnowledgeSettingsPage）zh-CN 下整面英文——其所需文案全部在这两个命名空间。
- **面板文案接线完成**：页签（知识库设置/数据源管理）、基本信息、解析引擎、分块设置（策略四选项修正，移除 React 自创 recursive 选项）、索引策略（RAG 检索/Wiki/知识图谱 + Embedding/LLM 只读绑定）、存储与向量绑定、活动记录、刷新/保存。
- 门禁：test:shared 445/445（键集一致性测试覆盖新域）、typecheck:shared 0。

## 2026-09-14 Round N+34 — 文档页 live 取证 + 新发现知识库设置面板 i18n 泄漏（部分完成）

- live 取证：React 文档页（Parity KB Demo ?tab=documents）真实渲染——面包屑/文件夹树/过滤/批量工具条/解析引擎警告/空态全部在位（d1-documents-react.png）。
- 新发现登记：①知识库设置面板（/knowledgeBase/:id/settings）zh-CN 下正文英文（General/Parser engines 等），该面未接共享 i18n——登记独立 i18n 对齐切片；②双端文档上传自动化待完善（React filechooser 可用；Vue t-upload 拖放区点击不触发选择器，需换交互目标）——文档真实上传的 live 数据维度顺延。
- 无代码变更、无共享数据写入；截图 docs-live-20260914/。证据 2026-09-14-documents-live-partial.md。

## 2026-09-14 Round N+33 — en-US 语言维度全路由扫尾（19 表面双端）

- enus-full-sweep.cjs：登录/注册/KB 列表/智能体/共享空间/creatChat/设置 11 分区/集成 API，共 19 表面 × 双端，en-US locale（1440x900，只读）。
- 结论：React en-US 无 UI 中文残留。reactOnly CJK 仅两类且均非泄漏：creat-chat 会话标题（用户数据）、general 语言下拉 endonym（简体中文/日本語，Vue locale 同值）。login/register Vue 自身泄漏 9 处中文（Vue 侧限制，登记）。
- S00 语言维度（zh/en-US）在主要路由 + 设置分区：React 全部通过；相关行的语言维度缺口就此闭合。
- 证据：2026-09-14-enus-full-sweep.md + screenshots/enus-full-20260914/（38 张）。

## 2026-09-14 Round N+32 — R013 API Playground 真实 Session/SSE + 两处阻断级修复

- 缺陷一：Playground 抽屉被设置弹窗遮罩（wks-overlay z 1100）压住、整体不可点击（elementsFromPoint 证实）。修复：抽屉内联 z 提升为常量 1200/1201/1202（对齐 Vue teleport 逃逸语义）。
- 缺陷二：抽屉硬编码 apiBaseUrl=window.location.origin（dev :5181 无 /api 代理 → Session 创建 404）。修复：新增 platform/api-base.ts resolveApiBaseUrl（VITE_API_BASE_URL → 注入 → 空串，与 main.tsx 同序），IntegrationsRoutePage 增加 apiBaseUrl prop 并透传抽屉/IntegrationsPage；main.tsx 传真实值。
- 真实链路验证：受控创建租户 API Key（parity-playground）+ 内置智能体绑定 builtin-llm-mock 后，React 抽屉真实完成 POST /sessions 201（principal=api_tenant_key:10000:3）→ POST /agent-chat/:id 200（SSE 建立）→ 终端错误在运行结果区正确渲染。
- 残余（环境）：内置 mock 模型 baseURL 为 IP，后端 SSRF 白名单拦截最终回答（Vue/React 等效受影响），如实记录。
- 门禁：ApiPlaygroundDrawer 12/12、typecheck:web 0、registry 6/6、shared 444/444、web 856/856。证据：2026-09-14-r013-playground-sse-live.md + screenshots/playground-20260914/。

## 2026-09-14 Round N+31 补充 — viewer「本空间 · 仅查看」分组标签修复

- domain groupKnowledgeBaseSections 增加 options.tenantReadonly（viewer/contributor 的 tenant 组标题切「本空间 · 仅查看」，组 key 稳定）；App.tsx 按 viewer.isAdmin 传入并对 viewer 切 browse 图标（对齐 Vue tenantSectionLabelKey/tenantSectionIconName）。
- domain list.test 新增 1 例（15/15）；全量门禁维持绿（shared 444/444、web 856/856、typecheck 0）。
- live 复核：viewer 账号 KB 列表全文含「本空间 · 仅查看」，与 Vue 逐字一致；证据 2026-09-14-viewer-role-variant.md 已更新。

## 2026-09-14 Round N+31 — viewer 角色变体 live 双端对照 + 设置角色门禁对齐

- 受控创建 viewer 回归账号（parity-viewer@local.dev，owner 经 members API 加入租户 10000，role=viewer；保留作角色变体回归）。
- 发现并修复：设置分区角色门禁与 Vue 不一致——Vue settingsAccess.ts 中 models/members 为 viewer（只读页，面板内部自 gate 写操作），React registry 误为 admin（viewer 看不到入口、直连渲染权限不足）。registry.ts 两处 minRole → viewer；registry.test.ts 期望同步（6/6）。
- live viewer 变体 sweep（viewer-variant.cjs，双端 viewer@tenant 10000）：V1 KB 列表（共享空间入口隐藏、无创建入口）、V2 设置导航（账户组 4 项 + 空间信息/成员管理/模型管理只读）、V3 直连 members（只读面板无控件泄漏）、V4 组织页——双端一致。
- 新登记残余差异：viewer 的 KB 列表作用域标签 Vue 为「本空间 · 仅查看」、React 仅「本空间」（记录待修）。
- 门禁：registry 6/6、shared 444/444、web 856/856。证据：2026-09-14-viewer-role-variant.md + screenshots/viewer-variant-20260914/。

## 2026-09-14 Round N+30 — implementing 七行 live 重验转 review（R013 R027 R031 R033 R043 R044 R046）

- live 只读 sweep（impl-live-sweep.cjs，双端 owner 视角）：模型管理/MCP/沙箱/技能/API 集成五个分区全部当前渲染一致；截图 impl-live-20260914/。
- 顺手修复（R027）：React 内置模型说明盒无样式（与 Vue 灰盒不符）——按 Vue ModelSettings.vue less:893-913 补齐 .wk-builtin-hint；修复后 i1-models-react-after.png 与 Vue 逐元素一致。
- 七行 implementing → review，note 记录各自精确残余项（真实 MCP 服务/沙箱集群/Session SSE/连接保存流程/Wails/native）。
- 证据：2026-09-14-impl-live-recheck.md。typecheck:web 0 错误。

## 2026-09-14 Round N+29 — STALE 12 行全部重验清除（R007 R008 R032 R045 R047-R050 N001 N002 N014 N015）

- live 只读 sweep（stale-refix.cjs，双端 owner 视角）：外壳导航锚点、成员管理单标题（无 wrapper 重复、无域名泄漏）、设置关闭 ✕ 双端回 KB 列表、MCP 分区渲染——全部一致；截图 stale-refix-20260914/。
- 单测重验：shell RBAC/route 19/19、preview 4/4。
- registry 现状核对（N015）：port ed:false 分区均渲染真实面板，占位符行为与行声明一致。
- R047-R050：parity KB 无文档，expired/no-permission live 证据不可产——单层级重验清除 STALE，行保持 review。
- matrix 12 行 note 追加 re-verify 结论（状态保持 review；残余开放项为 Wails/native、viewer 变体、expired/no-permission live）。
- 证据：2026-09-14-stale-refix-12-rows.md。

## 2026-09-14 Round N+28 — R026 工作区记忆面板对齐 + 个人记忆真实写路径 e2e

- MemoryWorkspacePanel 按 Vue MemoryWorkspaceSettings.vue 裸排版重建：去 Card、h2 分区头、中性 intro 盒（品牌图标 + 堆叠标题/描述）、wk-mws 设置行令牌、写模式 select → .wk-segmented 单选对（复用既有 radiogroup 惯例）、自定义提示词行改纵向堆叠全宽（Vue instructions-row）。样式集中于新 memory-workspace.css（Vue theme.css light 令牌）。
- 个人记忆真实写路径 e2e（真实后端，parity owner 账号，全程真实 UI，已复原并 API 复核）：Vue UI 开启空间记忆 → Vue 添加个人记忆条目 → React 同用户存储可见该条目 → React 行内编辑保存 → React popconfirm 删除 → React UI 关闭空间记忆复原；API 复核 enabled:false、测试条目 0、effective:false。R025/R026 的增/改/删/跨端可见性/配置持久化在真实后端双向闭环。
- matrix canonical 表 R009/R010 之间空行修复（S00 审计发现 #1 残留，表格恢复连续）。
- 门禁：test:web 856/856、test:shared 444/444、typecheck×2 0 错误、build ✓。截图 screenshots/memory-write-20260914/；证据 2026-09-14-r026-workspace-anatomy-write-e2e.md。
- 仍开放：auto 模式模型选择器弹层 computed-style 对照、暗色主题、Wails/native 证据。

## 2026-09-14 Round N+27 — 负路径第四批：T-3/T-4 关闭 + 网络错误文案层

- T-3 已修复：React refresh 失败后按 Vue authRefresh.ts 语义清 scope/session 并导航 /login（新增 apps/web/src/auth/relogin.ts + relogin.test.ts 4/4；main.tsx transport refresh 回调接线）。live 双端污染会话对照（negpath4-auth.cjs，只读）：双端最终 URL 均为 /login。
- T-4 已修复：组织预览无 message 失败 fallback 由 invalidCode 对齐为 previewFailed（OrganizationsPage.tsx 两处，对齐 Vue OrganizationList.vue:879 catch 分支）；HTTP-200 业务失败边界残余差异登记不阻塞。
- 新缺口发现并修复：网络层失败文案。live B 格显示 Vue 渲染「网络错误，请检查您的网络连接」而 React 泄漏 Failed to fetch。修复：packages/i18n 新增 error.networkError ×5 locale（en-US/ja-JP 取 Vue fallbackLocale=zh-CN 的实际渲染值，注释说明）+ apps/web transport withNetworkError 将 TypeError 转为 ApiError(NETWORK_ERROR, 本地化文案)，Abort/ApiError 透传。http.test.ts 新增 3 例。
- 门禁：relogin 4/4、http 13/13、test:shared 444/444、test:web 856/856、typecheck×2 0 错误。
- Live 证据：negpath4-auth.cjs 双端 A1（/login 落点）与 B1（网络错误文案逐字一致）；截图 screenshots/negpath4-20260914/；文档 2026-09-14-negpath4-auth-refresh-network.md。negpath 第三批 T-3/T-4 关闭。

## 2026-09-14 Round N+26 — R025 我的记忆整面重建 + 记忆分区挂载纠正

- Vue MemorySettings.vue（mymemory 分区）对齐：React PersonalMemorySettingsPanel 从开关+只读 dl 重建为完整面板——六状态页签（计数）、items/topics/documents 三列表（pending 确认/拒绝、行内编辑 Ctrl+Enter、删除/停止跟踪/整理/清空 popconfirm、跟踪进度条 + promote 跳回 active、文档打开对齐 Vue knowledgeBase 深链）、添加弹层（kind+content）、导出 JSON 下载、分页 offset 语义、逐页签空态/提示与 Vue 同键文案。
- 结构性修复：memory/mymemory 分区挂载按 Vue Settings.vue 纠正（个人面板移回 mymemory，memory 只留工作区面板）；SettingsPage wrapper heading 对这两个自带 h2 的分区停用；面板改裸排版（去外层 Card）对齐 Vue。
- 契约修复：api-client memory lists 返回 { rows, total }（MemoryListPage），补齐 clear/export/consolidate/confirm/reject/topics/documents action surface（7/7 合同测试）。
- i18n：回填 35 键 ×5 locale（kinds/kindHints/origins/状态页签/整理 skip/usage.rows），Vue locale 字节级；键集一致性测试通过。
- 共享组件：@weknora/ui Switch 由页面级 Tailwind 工具类改为包内 wk-switch 纯 CSS——原实现因 Tailwind v4 不扫描 pnpm node_modules 在真实浏览器塌缩成细条（live 证实）；修复后双端恢复 Vue 同款 40×20 胶囊（含 disabled on-state 色）。
- 验证：面板专项 11/11；test:shared 444/444、test:web 849/849、typecheck×2 0 错误、build ✓。live 双端只读对照（真实后端 :8080，parity 账号，1440×900 zh-CN）：六页签标签+计数、工具栏、notice、开关位、空态逐字一致，apiErrors/pageErrors 双端 0；截图 screenshots/memory-20260914/。证据：2026-09-14-mymemory-personal-panel.md；matrix R025/R026 已更新。
- 仍开放：真实数据写路径（编辑/确认/整理/导出）的 live 后端验证、computed-style 逐值对照、暗色令牌、Wails/native 证据、MemoryWorkspacePanel 外层 Card 遗留差异。

## 2026-09-14 Round N+25 — N011 Wiki directory contract and modes

- Index 首屏现在保存后端 `next_cursor`，按分组追加后续条目并隔离加载错误；加载按钮在请求中禁用。API 10/10、Wiki UI 5/5、正式 Web 797/797、`typecheck:web` 与 `typecheck:shared` 通过。
- Tree/List 切换现在清除旧目录上下文：List 回到全局平铺分页，Tree 回到根目录树；视图按钮使用 Vue 已有的五语言 `viewTree/viewList` 文案。完整 Web 回归仍为 797/797。
- Wiki 阅读器、编辑器和历史面板的 Edit/History/字段/版本/重载/空历史/差异/回滚/保存反馈改用已有 `wikiBrowser.*` 键，新增 `editSave` 五语言值；专项 5/5、i18n、typecheck 与正式 Web 797/797 通过。
- 随后补齐 Vue 已有的目录写操作 API client：创建/更新/删除文件夹与移动页面，严格校验返回的文件夹记录并覆盖层级 ID 编码；合同测试扩展至 10/10、`typecheck:shared` 通过。React UI 暂不显示这些写按钮，原因是当前 `WikiPage` 没有 Vue 对应的 KB 角色/权限上下文，需先接通权限边界后再开放。
- 已接通当前 tenant role：`main.tsx` 将非 viewer 角色传入 Wiki，React 目录操作控件对 viewer 隐藏；Wiki viewer 渲染专项 5/5、`typecheck:web`、`typecheck:shared` 通过，提交 `2d084c85`。
- 目录创建/重命名/删除增加 `folderBusy` 提交防重与失败保留当前目录语义，完整 Web 回归更新为 797/797；API 10/10、Wiki UI 5/5 仍通过。
- 对照 Vue `WikiBrowser.vue` 与 `frontend/src/api/wiki/index.ts` 确认后端已提供 `/wiki/folders`、`/wiki/index` 合同；API client 新增严格的文件夹树/结构化索引类型、解析器、筛选参数和 9/9 合同测试，提交 `1a8c0f6a`。
- React Wiki 新增 tree/list 切换、根/子目录加载、返回上级、结构化 Index 入口与索引条目打开页面；目录切换会重新请求 `category_path` 页面，Index 视图不再混入普通列表，补充 Vue 风格局部令牌样式，提交 `77ea4dce`。
- 聚焦 Wiki 4/4、正式 Web 796/796、`typecheck:web`、`typecheck:shared`、`git diff --check` 通过。
- N011 仍未完成：Vue 文件夹创建/重命名/删除/移动、独立 list 虚拟滚动、Index cursor 增量加载、图谱/问题状态、浏览器 computed-style、真实后端和 Wails/native 证据仍需逐项闭环。

## 2026-09-14 Round N+24 — N011/N013 failure-copy closure

- Wiki 页面加载、创建、重载、历史/版本读取和回滚的无异常对象 fallback 改用共享 `wikiBrowser.*` 文案；数据源编辑器字段、同步模式/冲突选项、删除/同步/连接测试/资源加载失败反馈改用共享 `dataSource.*` 文案，并为五种语言补齐连接器类型/设置键。
- Wiki 专项 4/4、数据源表单/资源选择专项 4/4、正式 Web 796/796、Web typecheck、i18n test 通过；实现提交 `064363d1`、`8bd84a40`。
- 本轮仅覆盖可验证的文案/错误 fallback 差异；不宣称完成 Vue Wiki index/tree/list、浏览器 computed-style、真实后端和 Wails/native 证据。

## 2026-09-14 Round N+20 — R013/N028 API Playground 抽屉与 SSE 行为

- API Playground 已从旧手填 Session/Path/Body 表单切换为 Vue 三段式右侧抽屉，接入身份模式门禁、内置智能体默认选择、Session 创建、Agent Chat SSE 增量回答、停止、终端错误、异常 EOF 和签名 Token。
- 证据：`evidence/vue-react-parity/2026-09-14-r013-api-playground.md`；模型 12/12、SSE 12/12、抽屉交互 9/9、正式 Web 757/757、`typecheck:web` 通过。
- R013/N028 仍为 `implementing`：同账号浏览器对照、真实后端/权限、Wails/native 证据未完成。
- React 认证浏览器已实际验证 API 集成页与 Playground 抽屉的入口、三段结构、默认值、密钥遮蔽和无 API Key 门禁；证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-browser.md`。当前空间无 API Key/智能体，真实 Session/SSE 记为 `blocked-env`。

## 2026-09-14 Round N+21 — N007 上传配置区段 v-show 对齐

- 上传确认对话框按 Vue `activeSection` 实现 v-show 等价行为：tags、parser、chunking、multimodal、asr、question、graph 区段保持挂载，仅隐藏非当前区段，避免 React 原先一次展示全部配置区段。
- 专项 21/21、正式 Web 758/758、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-upload-section-visibility.md`。
- N007 仍为 `implementing`：真实上传/解析后端、浏览器 computed-style、Wails/native 证据未闭环。

## 2026-09-14 Round N+23 — N008 音视频文档预览

- 预览类型解析新增 Vue 等价的常见音频/视频扩展；React 复用已认证 blob 生命周期，渲染原生 `audio controls` 与 `video controls playsInline`，不改变 DOCX/PPTX/Excel/Mermaid 的 renderer 依赖边界。
- Domain 预览测试 2/2、Web 预览测试 4/4；证据 `evidence/vue-react-parity/2026-09-14-n008-media-preview.md`。
- N008 仍为 `review`：DOCX/PPTX/Excel/Mermaid、live expiry、浏览器/真实后端及平台证据未闭环。

## 2026-09-14 Round N+22 — N006 文档批量 Shift 选择

- 文档列表补齐 Vue `toggleSelectRow` 的当前页 Shift 区间增删、最后选择锚点及筛选/全选重置语义；普通点击仍只切换当前行。
- 专项选择测试 3/3，上传确认回归 24/24，正式 Web 761/761，`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-n006-shift-selection.md`。
- N006 仍为 `review`：本轮补齐 Vue `useMarqueeSelect` 的 6px 左键框选、起点 add/subtract 固定模式、控件目标过滤和相交行更新；专项选择测试 5/5，正式 Web 763/763，`typecheck:web` 与 `build:web` 通过。证据 `evidence/vue-react-parity/2026-09-14-n006-marquee-selection.md`。真实浏览器 Shift/框选、后端批量链路、视觉与 Wails/native 证据未闭环。
- N006 标签 chip overflow 随后补齐：按 Vue `useTagChipsOverflow` 的 82px 估算、32px 溢出胶囊预留和 ResizeObserver 响应式上限渲染，标签全量名称保留在 tooltip。专项 tags 测试 11/11 通过；本轮全量类型检查受并行 FAQ 测试缺少导出阻断，未将该阻塞归因于 N006。

## 2026-09-14 Round N+19 — N005 custom organization picker slice

- 共享空间选择从原生 select 补为 Vue 风格可访问自定义列表：组织头像、角色、成员/知识库/智能体计数、选中态、外部点击关闭和 Escape；保留隐藏原生控件的 required/API 语义。
- N005 专测 12/12、Web 全量 681/681；`typecheck:web` 被并行 Excel 测试的 xlsx/Blob 类型错误阻断，未归因于本切片。
- N005 仍为 `implementing`：认证浏览器/Vue 对照截图、真实后端及 Wails/iOS/Android 证据未闭环。

## 2026-09-14 Round N+18 — R046 MCP tool detail popup slice

- MCP 工具详情按 Vue 从内联块改为浮层 dialog，保留描述/参数/schema 标签页、参数空态/required 标记及策略开关，新增 Escape 关闭。
- Web 全量 674/674、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-r046-tool-detail-popup.md`。
- R046 仍为 `implementing`：真实同步工具目录、认证浏览器 computed-style 与 Wails 证据未闭环。

## 2026-09-14 Round N+17 — R044 MCP credential card slice

- MCP 编辑态 API Key 按 Vue CredentialResource 分离为凭证卡片，显示已配置状态，支持替换和独立删除；新建态仍使用普通密码输入。
- MCP 专测 12/12、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-r044-credential-card.md`。
- R044 仍为 `implementing`：认证浏览器 computed-style、完整凭证卡片交互和 Wails 证据未闭环。

## 2026-09-14 Round N+16 — R043 MCP server documentation popup slice

- MCP 元数据摘要按 Vue 补齐服务端说明触发器/弹层与无说明帮助提示，保持 stale、同步和工具策略门控；修正弹层内 HTML 结构。
- MCP 专测 11/11、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-r043-server-docs-popup.md`。
- R043 仍为 `implementing`：认证浏览器 computed-style、可达 MCP 服务同步态、真实后端和 Wails 证据未闭环。

## 2026-09-14 Round N+15 — R031 Sandbox inventory drawer slice

- Sandbox inventory 按 Vue SettingDrawer 收口：固定右侧 400px 抽屉与遮罩，保留标题解析完成后再显示、会话行导航及 embed inert 语义。
- Sandbox 专测 28/28 通过；证据 `evidence/vue-react-parity/2026-09-14-r031-inventory-drawer.md`。
- R031 仍为 `implementing`：认证浏览器 computed-style、真实后端和 Wails/native 证据未闭环，另有既存审计项待处理。

## 2026-09-14 Round N+14 — N005 KB share dialog slice

- Vue 共享弹窗与 React 对照后，补齐组织信息预览（角色、成员/知识库/智能体计数）、权限提示、分离式取消/确认 footer、共享行头像/权限色签及组织设置导航入口；保留权限过滤、确认、防重与 API 契约。
- N005 专测 11/11、Web 全量 673/673、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-n005-share-dialog.md`。
- N005 仍为 `implementing`：原生 select 下拉弹层、认证浏览器/真实后端、Vue 对照截图及 Wails/iOS/Android 证据未闭环。

## 2026-09-14 Round N+13 — N010 FAQ import progress slice

- FAQ 导入进度按 Vue `FAQEntryManager.vue` 语义完成：后端 `processing/completed` 映射为 UI `running/success`，1.5 秒轮询，显示 processed/total，成功态 3 秒后收起；修复 `FAQImportProgress` 未从 api-client 公共入口导出及 effect 初始化顺序错误。
- FAQ 专项 20/20、Web 全量 672/672、`typecheck:web` 通过；证据 `evidence/vue-react-parity/2026-09-14-n010-faq-import-progress.md`。
- N010 仍为 `review`：真实后端文件上传/任务队列、浏览器 computed-style、移动端和 Wails/native 证据尚未闭环。

## 2026-09-14 Round N+12 — R027 model editor drawer slice

- Vue `ModelEditorDialog.vue` 使用 SettingDrawer；React 原先将 `.wk-model-editor` 作为主内容流内联卡片。本轮以失败测试锁定该差异，改为固定右侧 560px 抽屉 + 遮罩，保留表单、ESC/取消、点击遮罩关闭和移动端全宽行为。
- R027 专项 `ModelSettingsPanel.test.tsx` 22/22；浏览器 `:5181/platform/settings?section=models` 实际确认模型设置标签、卡片、新增入口及抽屉分组，证据 `evidence/vue-react-parity/2026-09-14-r027-model-drawer-browser.md`。
- R027 继续保持 `implementing`：设置子项切换、真实连接/保存流程、Wails/native 和完整 computed-style 对照仍未闭环。

## 2026-09-14 Round N+11 — R024/N016 MCP card anatomy slice

- 按 Vue `McpSettings.vue` 卡片结构收口 React：服务网格使用 320px 最小列与 10px 间距，卡片内工具徽标、图标化编辑/删除操作、使用说明空态、工具同步入口、传输类型和启用状态均对齐；管理员新增服务改为网格内虚线卡片，viewer 空态保持无新增入口。
- 共享样式仅作用于 MCP 类名，保留键盘焦点、aria-label、switch 状态和响应式单列断点；`McpSettingsPanel` 专项 11/11、Web 全量 670/670、Shared 419/419、Desktop 2/2、Embed 7/7，`typecheck:web` 通过。
- 该 slice 仍为 `review`：尚未新增本轮浏览器 computed-style、真实 MCP 服务同步、Wails 原生运行证据；不把组件测试或共享包回归当作最终逐页验收。

## 2026-09-14 Round N+10 — R013 IM wizard render verification

- 修复并验证 IM 向导实际 React 渲染接缝：新增/编辑抽屉、四步导航、无智能体校验、企业微信凭证字段、微信二维码绑定门控与 payload 提交；交互渲染测试 5/5。
- 补齐 `integrations.selectAgentHint` 五语回退文案，避免逻辑键存在但界面显示 key 名称。
- Web 全量 670/670，`typecheck:web` 与 `build:web` 通过。状态仍为 `review`，未将 mock/jsdom 证据当作浏览器/真实后端/平台验收。

## 2026-09-14 Round N+9 — R023 GeneralSettings 复核

- Vue `GeneralSettings.vue` 与 React `GeneralPreferencesPanel` 对照复核：语言、主题、界面字体、代码字体、预览、字号分段及 settings drawer 挂载路径均已有实现。
- focused settings tests 10/10 通过；此前 `typecheck:web` 与 `build:web` 已通过；R023 由 `implementing` 调整为 `review`。
- 未宣称最终通过：缺少本轮浏览器 computed-style、Wails/native 证据，且 React 字体 localStorage 尚未采用 Vue 的 user-scoped key。

## 2026-09-14 Round N+8 — N006/R013 深水切片收口

- N006 文档标签：Vue 多选筛选、标签 chips、单文档/批量标签对话框、全选/取消选择、成功反馈和筛选变化清空选择已接入；聚焦 19/19。
- R013 IM：Vue 四步向导、平台凭证字段、模式/会话校验、创建/更新 payload 及微信二维码绑定已接入；聚焦 24/24。
- 当前 Web 回归 665/665，`typecheck:web` 通过，`build:web` 通过。R009 并行收尾的测试/截图变更也已通过全量回归。
- 仍按证据层级标记为 `review`：以上切片尚缺对应的完整浏览器 computed-style、真实后端业务链路及 Wails/iOS/Android 验证；未把测试/构建等同于逐页验收。

## 2026-09-14 Round N+7 — N011 Wiki sidebar slice

- Vue authoritative review identified the Wiki left browser anatomy as a separate inset-search/sidebar surface, with 98px list rows, two-line summaries, compact metadata, selected/hover background and centered empty copy.
- React `WikiPage` now uses scoped `wk-wiki-sidebar`/`wk-wiki-page-item` structure and Vue-derived tokens while retaining existing editor/save-conflict/history/diff behavior.
- Focused Wiki/editor tests: 4/4. Current full Web run: 657/664; the seven failures are from concurrent document/tag work (`tagId` runtime error and tag-copy assertions), not N011.

## 2026-09-14 Round N+8 — N011 Wiki default reader/editor mode

- Vue-authoritative review confirms selected Wiki pages default to a read-only reader; editing is entered explicitly, while New page opens the editor. React now follows that state transition and returns to the reader after save/reload/revert.
- Added a scoped reader surface with title, summary, content, Edit and History actions; the editor remains available for create/edit flows and is hidden while reading.
- Full Web suite: 793/793 passed. Web typecheck: passed. This is static/unit evidence; browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+9 — N012 Wiki graph node detail slice

- Vue graph behavior opens a node detail drawer before/alongside ego-neighbor expansion. React now loads the selected Wiki page through the existing API contract and exposes scoped loading, error, close, summary/version/content, and expand-neighbors states.
- Focused graph and knowledge-permission tests: 8/8 passed. Web typecheck: passed. This is static/unit and typecheck evidence only; browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+10 — N012 graph remote search

- Vue graph search uses debounced server-backed Wiki search with selectable results. React now queries `client.wiki.list` after the same minimum-input interaction, renders a scoped suggestion list, and opens the selected page/ego graph while retaining local node filtering.
- Focused graph tests: 3/3 passed. Web typecheck: passed. Evidence remains static/unit and typecheck only; browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+11 — N013 data-source connection test

- Vue exposes connection validation independently from synchronization. React now calls the existing `dataSources.validate` endpoint from each saved source, disables concurrent actions, and reports validation success/failure without refreshing or claiming sync completion.
- Data-source form tests: 2/2 passed. Web typecheck: passed. Evidence is static/unit and typecheck only; resource browser, detailed logs, browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+12 — N013 sync-log result details

- Vue sync logs expose lifecycle and item result counts. React now renders finished-at plus total/created/updated/deleted/skipped/failed counts in the existing log surface, without changing API behavior.
- Data-source form tests: 2/2 passed. Web typecheck: passed. Evidence remains static/unit and typecheck only; resource browser, browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+19 — N013 data-source surface i18n

- Migrated the data-source page's visible headings, actions, status labels, resource/log controls, and editor titles to the shared `dataSource.*`/common translation layer already ported from Vue.
- Resource/API focused tests: 9/9 passed. Web typecheck: passed. Field-level connector errors, browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+14 — N013 resource browser

- React now exposes Browse resources on each saved connector and lazily loads root/child resources through `client.dataSources.resources`, with breadcrumb back navigation and loading/error/empty states.
- Data-source form tests: 2/2 passed. Web typecheck: passed. Resource selection/check-state parity and runtime evidence remain open.

## 2026-09-14 Round N+15 — N013 resource selection payload

- React resource rows now expose checkboxes and persist selected `resource_ids` through `DataSourceFormValues` and the connector config payload for create/update flows.
- Data-source form tests: 2/2 passed. Web typecheck and diff checks: passed. Vue minimal-cover/indeterminate subtree semantics and runtime evidence remain open.

## 2026-09-14 Round N+16 — N013 resource tree selection semantics

- Added tested resource-selection helpers matching the Vue cover-set model: parent selection covers loaded descendants, child selection exposes an indeterminate parent, and unchecking a child under a selected parent preserves sibling coverage.
- Data-source form/selection tests: 4/4 passed. Web typecheck: passed. Lazy ancestor reveal, localized copy, and runtime evidence remain open.

## 2026-09-14 Round N+18 — N013 saved resource ancestor reveal

- Added the API-client `resourceAncestors` contract and use it when opening a saved data source resource browser, preloading each ancestor's direct children so deep saved selections remain visible in the retained tree.
- API/data-source tests: 9/9 passed. Web typecheck: passed. Browser, real-backend, Wails, and native evidence remain open.

## 2026-09-14 Round N+17 — N013 retained resource tree

- React now retains all loaded resource nodes while navigating child levels, filters the visible list by the current parent, and provides an Expand all action that recursively loads known expandable branches. This preserves parent checked/indeterminate state across navigation.
- Data-source form/selection tests: 4/4 passed. Web typecheck: passed. Lazy ancestor reveal, localized copy, and runtime evidence remain open.

## 2026-09-14 Round N+13 — N013 sync-log pagination

- React now uses the existing log API `limit/offset` contract to page through synchronization history, resetting to the first page when a source is opened and disabling Previous/Next at the appropriate boundaries.
- Data-source form tests: 2/2 passed. Web typecheck: passed. Evidence remains static/unit and typecheck only; resource browser, browser, real-backend, Wails, and native evidence remain open.
- Status remains `review`; no new browser computed-style, real-backend, Wails, iOS or Android evidence claimed. Index/tree/list modes, folder actions, graph and reader states remain open.

## 2026-09-12 Round 1 基线

- 工作区：`.worktrees/react-multiclient`，分支 `codex/react-multiclient`，HEAD `b2d0cf6`。
- 已保留既有未提交修改（apps/mobile/expo-env.d.ts、reuse-manifest.csv、runtime-baseline.md、version-matrix.md 及 docs/superpowers/* 新文件）。
- 建立本矩阵与账本；evidence 目录 `docs/migrations/react/evidence/vue-react-parity/`。
- 初始状态：route-parity.csv 全部 53 条路由/入口均有 React 对应或明确 special-route 去向；无“React 尚无对应页面”项（待逐页核对确认非空壳）。
- 全部页面状态 review：已有测试/证据覆盖契约与路由分发，未覆盖逐页功能/布局/视觉/校验/文案/业务逻辑一致性与各端验收。
- 基线测试：见 evidence/vue-react-parity/2026-09-12-round1-baseline.md。

## 待办顺序（下一轮起逐页闭环）

1. /login + /register + /onboarding/workspace（auth 流，T03/T04）
2. /platform/knowledge-bases 列表页（T06）
3. /platform/knowledge-bases/:kbId 详情（T06–T09）
4. /platform/settings 各 section（T05/T17）
5. chat 全链路（T10–T14）
6. agents（T15）、organizations（T16）、integrations（T18）
7. embed 入口（T18）、移动端对应页（T20–T23）

## 2026-09-12 Round 1（续）

- 建立截图对比环境（.parity-tools/shoot.cjs + playwright-core，复用全局 chromium 缓存），证据：evidence/vue-react-parity/2026-09-12-visual-harness.md。
- Vue(5180)/React(5181) dev server 同视口截图 /login：React 与 Vue 基准存在整页级差异（无品牌区/页头/语言切换/i18n，英文硬编码）。证据：evidence/vue-react-parity/2026-09-12-login-visual-gap.md 与 screenshots/。
- /login 状态改 implementing；并行差异分析（login、KB list）进行中，下轮据报告实施修复。

## 2026-09-12 Round 2

- 收到 /login 与 KB list 两份逐项差异报告（13 项与 20 项，含 Vue/React 行级引用）。
- 完成鉴权页功能必修项：邀请 token 全链路（/login?token、/register?token、/join、OIDC sessionStorage 桥接、已登录自动兑换）、登录持久化租户 override + onboarding 分流、autoSetup lite 流、完整表单校验（含复杂密码模式与确认密码）、注册后预填 email、OIDC ?next。证据：evidence/vue-react-parity/2026-09-12-auth-functional-fixes.md。
- 共享层新增：api-client acceptInvitationByToken（后端 routes_auth_tenant.go:175 依据）。
- 测试：web 124/124（新增 12 例回归）；typecheck web/shared 通过。
- KB list 页实现由并行子代理进行中（App.tsx/knowledge-bases/domain/api-client/i18n knowledgeList 键），完成后主代理集成复核。
- 仍开放：auth 文案 i18n（5 locale）+ 语言切换、品牌视觉/轮播、WorkspaceOnboardingPage 功能化、KB list 全部必修项、其余页面矩阵推进。

## 2026-09-12 Round 3

- 完成 /onboarding/workspace 功能化：策略加载/重试、can_create_tenant 分叉、创建工作区（新 api-client tenants.admin.create，后端 tenant.go:89-90 依据）、我的邀请接受/拒绝与计数联动、有效租户自动重定向。证据：evidence/vue-react-parity/2026-09-12-onboarding-functional-fixes.md；提交 b95e622、c5607f4。
- 测试：web 128/128（新增 onboarding 4 例）；typecheck 通过。
- KB list 实现子代理仍在进行；auth i18n 键迁移待其释放 packages/i18n 后由主代理执行。

## 2026-09-12 Round 4

- /login 视觉层与多语言对齐：完整移植渐变背景/动画节点/页头/语言切换/轮播/卡片样式（auth.css 逐值移植），5 locale 73 键文案程序化提取（locales.ts，零缺失），必填星号与注册卡对齐。证据：evidence/vue-react-parity/2026-09-12-login-visual-i18n.md + screenshots/login-react-visual-round4c.png。
- web 131/131；auth 文件 typecheck 干净（App.tsx 错误为 KB 子代理 WIP）。
- 仍开放：SVG 节点图标部分占位、showcase 垂直位置/分页圆点微调、locales.ts 迁入 packages/i18n、KB list 集成、其余页面。

## 2026-09-12 Round 5

- 修复轮播分页圆点可见性；浏览器交互验证语言切换（localStorage 持久化 + 整页文案切换 + 切回）与轮播分页点击，均通过。证据：evidence/vue-react-parity/2026-09-12-login-interactions.md + screenshots/login-round5-{default,en}.png；脚本 .parity-tools/verify-login.cjs。
- KB list 实现子代理继续进行中（工作区持续有其修改）。

## 2026-09-12 Round 6

- /login 背景节点图标错位修复（node-1 双 path 合并，crop 对比证据）；提交 dc169d7、e0cc0a7。
- KB list 子代理完成全部 8 项必修 + pin/duplicate/?scope=；主代理独立复核（全量测试/边界/端点真实性抽查）通过后集成提交 39a9180。证据：evidence/vue-react-parity/2026-09-12-kblist-integration.md。
- 当前基线：shared 236/236、web 131/131、typecheck x2 干净、build:web 成功。
- 下一步：KB 折叠分组节头、带后端的 KB 页截图对比、auth locales.ts 迁入 packages/i18n、继续矩阵后续页面（settings/chat/agents/organizations/integrations/embed）。

## 2026-09-12 Round 7

- auth 73 键 ×5 locale 文案迁入 packages/i18n（authMessages），LoginPage 切换为共享 formatMessage；新增 3 例 i18n 回归测试。回归全绿（i18n 7/7、web 131/131、typecheck、浏览器交互复测）。提交见 git log。证据：evidence/vue-react-parity/2026-09-12-auth-i18n-shared.md。
- 并行：settings 与 chat 两页逐项差异分析子代理进行中。
- 待办：JoinPage/WorkspaceOnboardingPage MESSAGES 迁移、settings/chat 修复实施、KB 折叠节头、带后端截图对比。

## 2026-09-12 Round 8

- onboarding/tenant/invitation 24 键 ×5 locale 迁入 packages/i18n（onboardingMessages）；JoinPage/WorkspaceOnboardingPage 弃用本地 MESSAGES；keys.test 的 auth.login 期望按 Vue 基准（en-US 'Login'）修正——非降低断言，Vue 为权威。i18n 10/10、shared 239/239、web 131/131 全绿。
- 收到 settings（15 项，必修 8：缺 10 个 section、无角色门控、envvars 无编辑器、模型选择器缺失、chathistory 模型锁丢失、密码策略未移植、websearch 凭证管理缺失、i18n）与 chat（15 项，必修 7：无流取消、无 continue-stream 续传、composer 载荷缺附件/提及/模型、steer 并发缺失、审批卡不持久、session_title 丢弃、artifacts_pending 等事件未处理）两份行级差异报告。
- chat 必修 7 项已派发实施子代理（持有 chat 文件）；settings 实施待 chat 完成后派发（避免 packages/i18n 与文件竞争），其 i18n 项由主代理统一执行。

## 2026-09-12 Round 9

- 搭建隔离真实后端环境（docker 基础设施 + make dev-app :8080 + 测试账号/测试 KB），React/Vue 双端 live 登录→KB 列表全链路打通。证据：evidence/vue-react-parity/2026-09-12-live-backend-e2e.md + screenshots/kblist-{react,vue}-live.png。
- 结构性发现：React 缺少 /platform 全局导航外壳（侧栏/图标轨/toast/新手引导/用户菜单），登记为高优待办（T04/T05 shell 范畴）。
- 修复 KB 列表调试残留 scope key 泄漏。
- chat 必修实施子代理进行中；settings 实施排队中。

## 2026-09-12 Round 10

- packages/i18n 新增 settings.ts：settings 页面 1038 个引用键 ×5 locale 全量迁移（字节级），合并链接入；新增键集一致性测试。i18n 12/12、shared 247/247、typecheck 通过。
- 平台外壳实施子代理与 chat 实施子代理并行进行中。

## 2026-09-12 Round 11

- settings 实施子代理派发（角色门控/缺失 sections/envvars 编辑器/模型选择器/模型锁/密码策略/脏检查/popstate——i18n 键已就绪）。
- 邀请与共享链接 live API E2E 通过：pending-count→accept→membership；invite-links→accept-by-token。证据：evidence/vue-react-parity/2026-09-12-invite-live-e2e.md。
- chat 实施、平台外壳实施子代理继续进行中。

## 2026-09-12 Round 12

- 矩阵更新：/platform/settings、/platform/chat/:chatid、/platform/creatChat 三行写入行级审计结论与实施状态。
- 三个实施子代理并行：chat 必修（进行中）、平台外壳（进行中）、settings 必修（已派发）。
- 观察到 chat 子代理同步修改 vite.config.ts 与 packages/domain/package.json（为新增 chat 模块接 exports/alias），属其文件域。

## 2026-09-12 Round 12（续）

- chat 必修 7 项完成并集成提交 ac8c763（主代理抽查端点与纯模块测试后落盘）。证据：evidence/vue-react-parity/2026-09-12-chat-integration.md。
- 平台外壳、settings 实施子代理继续进行中。

## 2026-09-12 Round 12（续二）

- 平台外壳完成并集成：侧栏/折叠/导航高亮/KB 过滤/用户区包裹全部受保护路由；live 复核截图对比通过。提交 fe85f30。已知偏差（图标轨、图片 logo、会话列表并入、租户切换/铃/命令面板）登记于 evidence/vue-react-parity/2026-09-12-platform-shell.md。
- chat 必修集成提交 ac8c763。settings 实施子代理进行中。

## 2026-09-12 Round 13

- settings 双端 live 基线截图入库（settings-{react,vue}-live.png）：证实审计结论（Vue 抽屉+分组+中文 vs React 英文平铺只读）。settings 实施子代理进行中。
- org/agents/integrations/embed 四页差异审计子代理派发（只读）。

## 2026-09-12 Round 15

- Vue 顶层 menu.* 26 键 ×5 locale 迁入 packages/i18n（menu.ts）；PlatformShell 本地 NAV_LABELS 删除，改用 formatMessage（个人设置用 general.personalSettings，对齐 Vue UserMenu.vue:79）。提交 5b4bd51。
- 收到 organizations/agents/integrations/embed 四页审计（A 部分：审批门控 join 与 org settings 弹窗缺失；B：AgentEditor 6753 行与聊天侧 AgentSelector 缺失，页面为 JSON 编辑器 stub；C：API-key 管理 API 表缺失、playground 非 SSE；D：embed 缺 markdown/引用渲染与文件上传/ignore default_locale）。待派发实施。
- web 全量 146/146；typecheck:web 现存错误仅 settings 实施子代理 WIP 文件（surface.test.ts 重复标识符，其收尾时自愈）。

## 2026-09-12 Round 16

- settings 必修 8 项完成并集成（d1aa583 + 面板补交）：角色门控（scopeRuntime.role() 新增）、9 缺失 section（live 数据或显式未迁移提示）、envvars 编辑器、模型选择器+allow-list、chathistory 模型锁、密码策略（domain/auth/password-policy）、脏检查保存、popstate。vite alias 补齐后 build 恢复。
- live 验证：ModelManagement 新 section + not-yet-ported 提示 + 真实数据渲染。
- 四页（org/agents/integrations/embed）必修实施子代理进行中。

## 2026-09-12 Round 17

- 四页（org/agents/integrations/embed）live 基线截图入库：live-{agents,organizations,integrations-embed,integrations-api}.png。发现 live 缺陷并转告实施代理：integrations API tab 拉取 im-channels 且空数据 shape 触发解析错误；agents 页为共享/模型调试功能（英文，无列表）与审计一致。
- 四页实施子代理进行中。

## 2026-09-12 Round 19

- KB 列表折叠分组节头完成（KB 后续项 #11 关闭）：domain 分组函数 + App.tsx 折叠 chips；live 验证通过。提交 01bccd0。证据：evidence/vue-react-parity/2026-09-12-kb-sections.md。
- 四页实施子代理继续（embed/organizations 已见其修改）。

## 2026-09-12 Round 20

- /register UI 级 live E2E 通过：客户端校验拦截（0 请求 + 4 内联错误）、有效注册恰好 1 请求、切回登录预填邮箱。证据：evidence/vue-react-parity/2026-09-12-register-live-e2e.md + screenshots/register-flow-live.png；脚本 .parity-tools/register-flow.cjs。
- 四页实施子代理继续（organizations + packages/i18n 新域进行中）。

## 2026-09-12 Round 21

- live 验证未初始化 KB 卡片 → settings 路由（KB 必修 #16）。KB 详情面深入 E2E 待租户配置测试模型。
- KB 详情面（documents/document detail/preview/processing timeline/wiki/FAQ/graph）只读审计子代理派发。
- 四页实施子代理继续。

## 2026-09-12 Round 22

- live 发现并转告 KB 详情审计代理：①设置按钮（App.tsx:398 无条件 /knowledgeBase/:id/settings）与卡片点击（openCard 模型就绪分支 → /platform/settings）对同一未初始化 KB 路由不一致；②Documents 页调试文案 "tags loaded: none…" 泄漏。截图 live-kb-documents.png。
- 文档页功能骨架（上传/文件夹/过滤/批量）在 live 可用。

## 2026-09-12 Round 24

- KB 详情审计报告收货（成熟度：documents/detail partial、timeline missing、wiki/graph stub、FAQ partial；必修 10 项）。
- 主代理即刻修复两项：#4 设置按钮统一模型就绪门控（openKbSettings helper，对齐 Vue handleCardClick）；#10 loading 哨兵替换为独立 status:'loading'。提交 16b4437。
- 其余必修（reparse/cancel+timeline、FAQ/Wiki 分页、权限门控、KB 类型路由、上传流程、批量确认、i18n 移植）待四页代理释放 packages/i18n 后派发实施。

## 2026-09-12 Round 25

- 四页必修实施完成并集成（ab4af20，25 文件；含 i18n organization/agent/integrations/embed 四新域 ×5 locale + 键集一致性测试；API-key 表格、审批门控 join、org 设置精要、agent 类型化编辑器+分组、embed markdown/上传/i18n、API-tab live 缺陷双层修复）。
- 门禁：shared 266/266、web 165/165、typecheck×2、build:web+embed 全绿；*.tsbuildinfo 入 gitignore。
- 四页 live 复测（审批 join/API-key 创建/embed 渲染）下一轮执行；矩阵四行更新。

## 2026-09-12 Round 25

- 四页 live 复测通过：API tab 解析错误与眉题消失、API-key 创建表单渲染、agents 分组列表渲染、organizations 正常。证据：evidence/vue-react-parity/2026-09-12-fourpage-live-retest.md + screenshots/live-*-after.png。
- 全局门禁（Round 25 起点）：shared 266/266、web 165/165、typecheck×2、build 全绿。

## 2026-09-12 Round 25（续）

- Organizations live E2E：建组织→invite_code→直接 join 全链路通过；发现后端创建组织时忽略 require_approval 字段（审批门控分支实际不可达）——登记为后端待决项，前端分支保留。证据：evidence/vue-react-parity/2026-09-12-org-join-live-e2e.md。

## 2026-09-12 Round 26

- KB 详情面必修实施子代理派发（reparse/cancel+timeline、FAQ/Wiki 分页、权限门控、KB 类型路由、i18n 移植 knowledgeBase 等域）。
- 上轮 org join live E2E 发现的 require_approval 后端忽略问题已登记后端待决项。

## 2026-09-12 Round 26（续）

- 多端门禁复验：desktop 测试 0 失败 + typecheck 干净；mobile 85/85 + typecheck 干净。共享层改动未破坏桌面/移动端。

## 2026-09-12 Round 27

- chat 会话页 live 基线（真实 session id 80fd9cf5…）：会话侧栏/沙箱终端控制/消息输入可用（screenshots/live-chat-session.png），但视觉形态与 Vue（botmsg 排版/引用卡片/输入区组合）差距大——登记为「chat 视觉形态」大项，排在 KB 详情实施之后。
- KB 详情审计结论已入矩阵（Round 26），KB 详情实施子代理进行中。

## 2026-09-12 Round 28

- Organizations live 复测：invite_code 参数触发的预览卡正确显示 already-member 分支（“您已经是该共享空间的成员”），组织列表含成员计数/角色（org-invite-preview-live.png）。
- chat 视觉形态实施子代理派发（packages/views/chat + ChatRoutePage 渲染层；禁改 packages/i18n）。
- KB 详情实施子代理继续。

## 2026-09-12 Round 29

- embed 渠道隔离链路搭建：agents/:id/embed-channels 创建渠道（allowed_origins 需含前端 origin）→ preview-session 签发 ems_ 令牌 → embed dev :5182（VITE_API_BASE_URL=:8080）。
- 发现 embed 冷恢复缺陷：会话 resume 请求 GET /api/v1/embed/:ch/messages/:sid/load… 网络层 ERR_FAILED（初始 token 交换/会话创建成功）。已登记 embed 缺陷待查（后续结合 EmbedApp session probe 逻辑定位）。
- chat 视觉形态实施完成并提交 54f2e59（web 175/175、build 绿）。

## 2026-09-12 Round 30

- embed 冷恢复缺陷根因定位：后端 CORS credentials+通配 origin 无效组合拦截带 Authorization 响应（后端共性缺陷，Vue 亦受影响）。登记后端待决项；embed 前端回退逻辑本身正确。证据：evidence/vue-react-parity/2026-09-12-embed-resume-defect.md。

## 2026-09-12 Round 30（补）

- embed 缺陷根因修正：非 CORS。preview-session 签发的 ems_ 令牌被 POST /embed/sessions 拒绝（401 invalid or expired token，curl 复现）——后端 embed 预览令牌语义待裁决。证据已补入 embed-resume-defect.md。

## 2026-09-12 Round 32

- KB 详情面必修 6 项集成（eb47a9c，主代理门禁复核 + live 复测文档页 i18n/无调试泄漏）。证据：evidence/vue-react-parity/2026-09-12-kbdetail-integration.md。
- 用户既有未提交修改（apps/mobile、docs/migrations/react/*.csv/md、package-lock）按要求原样保留。

## 2026-09-12 Round 33

- KB 上传流程 parity（多文件/拖拽/上传确认）与聊天侧 AgentSelector + creatChat 建议问题两个后续切片合并派发一个实施子代理（文件域：documents/chat/configuration 只读核查）。
- embed preview-session 令牌被 /embed/sessions 拒绝的问题已登记后端待决（Round 30）；待后端裁决后补 embed 预览 live 验证。

## 2026-09-12 Round 34

- FAQ 型 KB 路由 live 验证通过：/knowledgeBase/{faqId} 自动 replace 到 /faq（screenshots/live-faq-routing.png）；测试 FAQ KB（parity-faq-kb）入隔离库。
- 矩阵标记 KB 详情必修 #1/#2/#3/#4/#7/#9/#10 已实施，#5/#6/#8 开放项登记。
- 上传流程 + AgentSelector 实施子代理进行中。

## 2026-09-12 Round 35

- settings 视觉形态（抽屉/分组导航/图标/关闭行为）实施子代理派发：要求复用 packages/i18n settings.* 键、保留全部功能接线（角色门控/popstate/面板）、live 前后截图对比 settings-vue-live.png。
- 上传流程 + AgentSelector 实施子代理继续。

## 2026-09-12 Round 36

- 上传 pipeline、AgentSelector、starter questions、embed CORS 修复集成（40a5d86）。门禁：shared 276/276、web 194/194、build 全绿；embed live 渲染成功。
- 后端待决项维持登记：POST /sessions 不支持 agent_id；require_approval 创建字段被忽略；冷刷新审批持久端点。

## 2026-09-12 Round 40

- settings 抽屉视觉形态完成并收尾：固定遮罩 + 1080x780 模态 + 分组导航（账户/空间/模型/数据与扩展/系统管理/平台，图标 + settings.* 标签）+ ✕ 关闭回知识库列表；角色门控/popstate/全部面板功能接线保留；styles.css 追加式 wks-* 样式。遗留中间态（重复 import）已修复（aa77f7e）。
- 门禁：web 194/194、typecheck:web 0 错误、build:web ✓。证据：screenshots/settings-form-{before,after}.png 对比 settings-vue-live.png。
- settings 页状态：视觉形态 + 功能必修完成后仍维持 review（待全页截图矩阵与多端验收收敛）。

## 2026-09-12 Round 41

- chat UX 收尾切片派发：消息时间戳分隔（messageTimestamp 移植 domain）、助手复制按钮、流式打字指示器、回到底部按钮、会话分组模式持久化、creatChat 空态建议问题 chips + 建会话后首条消息带 agent_id（后端 sessions 不收 agent_id 的缺口保持如实记录）。
- settings 抽屉完成收尾（aa77f7e/0a946c2）后，主要页面域实施全部闭环；进入 UX 细节与验收收敛阶段。

## 2026-09-12 Round 43

- chat UX 切片首个实施代理中途失败（未留下任何变更，树保持干净全绿）；同范围重新派发（retry 代理），要求逐项报告状态。

## 2026-09-12 Round 45

- 尝试 Wails macOS 打包（REACT_FRONTEND=1 package-mac-app.sh）：web 构建步被 chat UX retry 代理进行中变更阻断（@weknora/domain/chat/copy-answer 模块未建、message-list readonly 断言）——时序问题非缺陷；chat UX 完成集成后重跑打包验证。
- 顺带确认打包脚本对工作树清洁度敏感：与在途子代理并行时会互相干扰，后续打包统一在无在途实施时执行。

## 2026-09-12 Round 47

- 集成核对基准确认：Vue 会话分组持久化键为 `weknora:session-group-mode`，默认模式 `none`（frontend/src/components/sessionGrouping.ts:5-6）。chat UX retry 代理交付时按此键名核验。

## 2026-09-12 Round 48

- chat UX 收尾切片完成集成（62a77a9：时间戳分隔/复制按钮/打字指示器/回到底部/分组持久化/creatChat 建议 + domain message-timestamps/copy-answer/session-grouping 模块与 3 个新测试文件）。
- **根治 gitignore 缺陷**：裸 `web/` 规则锚定为 `/web/`（该规则此前静默吞掉 apps/web 下全部新源文件——resume/steer-submit/starter-questions/actions/processing-timeline/upload-pipeline/permissions/pagination/i18n.ts 等已全部补交入库），并为 apps/{web,embed,desktop}/dist 增加显式忽略。共享/前端门禁复验：shared 293/293、web 198/198、typecheck 干净。

## 2026-09-12 Round 50

- chat UX 收尾切片完成集成（62a77a9）+ gitignore web/ 规则锚定（/web/），被吞源文件全部补交（c58caac/3340ff7）。
- Wails macOS 打包验证通过：React bundle 构建 + wails build + 自签名 .app（codesign 校验通过）；集成修复 desktop vite alias 缺失（6c8775e）+ 后端 CORS X-Embed-Visitor。
- 门禁：shared 293/293、web 198/198、typecheck×2、build:web 全绿。
- 待办：embed CORS 修复后的 live 复测、四页剩余后续项、Wails 运行时交互验收、iOS/Android 原生验收。

## 2026-09-12 Round 52

- settings 抽屉 section 标题本地化：SECTION_TITLE_KEYS 映射（models→模型管理 等），live 验证 h2 不再出现英文键名（live-settings-models-title.png）。提交 784b25a。
- chat UX retry 代理进行中（session-state/message-list/chat-page 等文件持续更新）。

## 2026-09-12 Round 53

- Wails macOS 打包在干净树上复跑成功（含 chat UX 收尾内容），自签名 .app 生成。embed-resume-defect 记录文件追加复跑确认。
- chat UX retry 代理的工作已于 62a77a9 集成，无遗留未提交内容。

## 2026-09-12 Round 54

- chat 会话页双端矩阵截图入库（同 session 80fd9cf5，用户消息双端一致呈现）。Vue 侧流式 404 为环境限制（租户无 chat 模型 + dev 代理路径），非 React 缺陷；React 侧显示 continue-stream 恢复提示（必修 #2 行为生效）。矩阵索引已更新。
- Upload/AgentSelector retry 代理完成（内容已在 40a5d86 集成）。

## 2026-09-12 Round 55

- iOS/Android Expo 导出构建通过（Hermes 3.2MB/3.3MB）；构建产物目录加入 gitignore。证据：evidence/vue-react-parity/2026-09-12-mobile-native-builds.md。
- 原生交互验收限制如实登记：需模拟器/真机环境，当前会话无 GUI 设备。

## 2026-09-12 Round 56

- KB 列表收藏/最近 scope 完成（domain filterByScope + 测试；App.tsx scope tabs + 逐用户 localStorage 收藏/最近打开追踪）。提交见 git log。
- 门禁：web 198/198、typecheck 干净。

## 2026-09-12 Round 57

- settings GeneralPreferencesPanel 落地：主题（light/dark/system）/语言/字号，localStorage 持久化（对齐 Vue GeneralSettings.vue 本地偏好语义）。domain local-preferences.ts + 4 测试。web 198/198、shared 296/296 全绿。
- 四页实施代理的 embed 修复（channels.listAll/im.listAll 容忍 null data、tab 独立取数）+ AgentSelector/suggestions/upload pipeline 已集成（40a5d86）。

## 2026-09-12 Round 58

- 终版 React live 截图矩阵入库（final-react-{login,kb-list,settings-general,agents,organizations,creatChat}.png）——KB 列表页与 Vue 基准高度对齐：外壳侧栏+active 高亮、i18n 中文标题/描述、未初始化横幅、scope tabs（全部/我创建的/收藏/暂无最近访问）、分组节头（我创建的 · 3）、卡片（类型徽章+计数+警告+收藏星+全部操作按钮）、用户区。
- 门禁：web 198/198、shared 296/296、typecheck 干净。
- 移动端 Expo 构建验证通过（iOS 3.2MB + Android 3.3MB Hermes bundle）。

## 2026-09-12 Round 59

- Wails macOS 打包成功（干净树 + 全部 chat UX 收尾内容）：wails build 22.4s → 打包 → 自签名 → codesign verify 通过。产物 dist/WeKnora Lite.app。
- 此前阻断根因（chat WIP 的 copy-answer/session-grouping 模块缺失）已随 62a77a9 集成消除；desktop vite alias 补齐后构建链路恢复。
- 当前 Wails 验证级别：构建+签名+资源冒烟 ✓。运行时交互验收（点击聊天/上传等）需 GUI 操作。

## 2026-09-12 Round 62 终态验证

全量门禁（主代理独立复核）：
| 层 | 结果 |
|---|---|
| test:shared | 296/296 ✅ |
| test:web | 198/198 ✅ |
| test:desktop | 2/0 ✅ |
| test:mobile | 85/85 ✅ |
| typecheck:web | 0 错误 ✅ |
| typecheck:shared | 0 错误 ✅ |
| build:web | ✓ (3.12s) |

工作树状态：仅余用户既有未提交修改（apps/mobile/expo-env.d.ts、docs/migrations/react/{reuse-manifest.csv,runtime-baseline.md,version-matrix.md}、package.json/package-lock.json、docs/superpowers/* 新文件）。所有实施代理工作已全部提交。

## 2026-09-12 Round 63

- chat 分类型工具结果渲染器实施派发（search/database/grep/shell-exec + fallback；当前为通用 pre dump）。工具渲染器是 chat 审计后续项中用户可见度最高的改进。

## 2026-09-12 Round 66

- chat 分类型工具结果渲染器集成（a027bc3，3 文件 +750/-9）：SearchResults、WebSearchResults、DatabaseQuery、GrepResults、ShellExec 5 种渲染器 + fallback；密钥脱敏保留；dispatch 按 display_type（对齐 Vue ToolResultRenderer.vue）；17 测试（14 新，TDD）。
- 门禁：shared 310/310、web 198/198、typecheck×2 干净、build:web ✓。

## 2026-09-12 Round 67

- 分类型工具渲染器集成复核确认（a027bc3 已提交，门禁全绿）；矩阵 chat 行已是最新状态。
- 全部实施代理空闲，工作树干净（仅余用户既有修改）。
- 主要实施闭环已完成；进入验收收敛阶段。

## 2026-09-12 Round 68

- settings 面板 i18n 接线实施子代理派发（5 个面板：ConfigSettings/Resource/Ollama/Cloud/PersonalMemory → formatMessage + settings.* 键）。

## 2026-09-12 Round 74

- settings 5 面板 i18n formatMessage 接线完成并集成（ec0da51）：Config(20)/Resource(12)/Ollama(16)/Cloud(12)/PersonalMemory(28) t() 调用点全部使用 settings.* 键。未匹配的 React 专有字符串保留 TODO(migration) 注释。
- 门禁：typecheck:web 0 错误、web 198/198、shared 310/310 全绿。

## 2026-09-12 Round 76

- 终版全页 live 回归验证：7 个主页面路由全部正常加载（0 alerts, 0 errors），截图入库（final-*.png）。
- 全量门禁复验：shared 310/310、web 198/198、desktop 2/0、mobile 85/85、typecheck×2 0 错误、build:web ✓。

## 2026-09-12 Round 79

- api-client identity admin API 增补 deleteTenant（DELETE /api/v1/tenants/:id，routes_auth_tenant.go:18 Owner+）。
- tenant delete danger zone 初版实施但判定为需要更完整的 state/role-gating 集成，回退未完成变更避免提交半成品——留待后续迭代。
- 门禁：web 198/198、typecheck 0 错误。

## 2026-09-12 Round 82

- chat 打磨切片派发（分类型工具渲染器扩展、Last-Event-ID 重连、会话来源 badge）。
- 前轮 chat UX retry 代理的工作已全部集成（62a77a9）。

## 2026-09-12 Round 87

- 门禁复验：shared 310/310、web 198/198 全绿。chat 打磨切片实施子代理运行中（tool-result.tsx/chat.css/tenant.ts 等 WIP）。

## 2026-09-12 Round 89

- chat 打磨切片完成集成（1c1c463，9 文件 +1014/-8）：分类型工具渲染器扩展（7 种新类型）、Last-Event-ID 重连、会话来源 badge。门禁：shared 320/320、web 202/202、typecheck×2、build:web 全绿。

## 2026-09-12 Round 91

- chat 审批 modified_args 编辑器 UI 派发（审批卡片可展开查看/编辑工具参数，提交时传 modified_args）。

## 2026-09-12 Round 92

- args 编辑器实施子代理运行中（chat 审批参数查看/编辑 UI）。
- 门禁复验：shared 310/310、web 202/202、typecheck 0 错误。证据索引已刷新（29 份 evidence 文件）。

## 2026-09-12 Round 95

- args 编辑器实施子代理运行中（packages/views/src/chat/page.tsx、chat-page.test.ts、chat.css WIP）。门禁：shared 326/326、web 204/204 全绿——TDD 红转绿完成。

## 2026-09-12 Round 95

- chat 审批 modified_args 编辑器完成集成（f887acd，5 文件 +272/-5）：查看参数 <details> 展开编辑、JSON 校验（无效输入阻断 + role=alert）、同意传 modified_args、拒绝免校验。6 单测 + 2 SSR 测试。
- 门禁：web 204/204、typecheck 0 错误、build:web ✓。
- 已知限制：domain ChatApproval 类型无 arguments/toolName 字段（domain 越界未改），live 流 reducer 暂无法填充 args——视图层以 {} 回退，待 domain 层后续扩展。

## 2026-09-12 Round 98

- tenant delete danger zone live 验证通过：owner 视角下 zone 渲染、确认名输入前删除按钮禁用（armed 语义生效）。6 个主页面 live 回归 6/6。截图 tenant-delete-zone-live.png 入库。

## 2026-09-12 Round 99 收敛快照

- 门禁：shared 326/326、web 204/204、typecheck 0 错误。live 回归 6/6 主页面正常。
- settings 审计条目全部关闭（最后一项 tenant delete danger zone 已集成+live 验证）。
- 剩余开放项均需外部因素：Wails 运行时交互（GUI）、iOS/Android 真机验收（设备）、后端 3 项语义裁决、矩阵逐页终审。

## 2026-09-12 iOS 模拟器交互验收（Round 100 专项）

环境：iPhone 17 Pro 模拟器（iOS 26.5，UDID 5EECD8BB），原生 dev client com.weknora.mobile（expo run:ios Hermes 构建），后端 make dev-app 于 :8080。

已验证（全部有截图证据 screenshots/ios-sim-*.png）：
1. 原生应用构建+安装+启动 ✅（修复 ios/Podfile.lock 缺失 react-native-netinfo 的 pod install）
2. 登录界面渲染：品牌标题/Email/Password/Sign in/SSO/注册/邀请/Change server 全入口 ✅
3. Change server → Save server 服务器地址配置流程实测走通 ✅
4. 真实网络链路：应用→后端请求被后端接收（后端日志记录 client_ip=192.168.3.30 的 auth/login POST）✅
5. 真实凭据登录成功（parity-test@local.dev，后端返回 200 Login successful）✅
6. 登录后跳转 knowledge/index，KB 列表渲染真实后端数据（parity-faq-kb / Parity KB Demo / 产品知识库）✅

环境限制（如实记录）：



### Round 256（第三轮目标周期收尾 · 会话状态快照）
- **本周期已提交**（周期起点 HEAD 495b0c8d → 末轮 a5302589）：negpath 批一证据 e23440f0 · D1 修复 e6d7d8bd · D6 本地化 845621ba · D1-D6 处置 993e2a92 · negpath 批二证据 6b03f7b1 · N-4 修复 b6a5982e（+五语回填 b7a667bb）· N-2 修复 52309dbb（+处置 2d9726dd）· F3 修复 7ff45646 · R116-188 账本 a5302589。并发协调者同期落地 model-select/ui-primitives/memory-permissions/api-client-settings 系列。
- **在途**：negpath 第三批（403 租户隔离，切片 f67e0efd）产物已全部落盘未提交——docs/migrations/react/evidence/vue-react-parity/2026-09-14-negpath3-tenant-isolation.md · screenshots/negpath3-20260914/（8 张）· .parity-tools/negpath3-*.sh ×3 + negpath3-tenant.cjs + negpath3-results.json。切片运行中待最终报告；期间 OrbStack 曾停止、切片以 orb start 恢复成功（恢复轨迹见切片消息）。下周期首动作：收切片报告→验证产物→精确路径提交→T-* 差异处置（审阅 docs 头部结论）。并发协调者 api-client settings WIP（M packages/api-client/src/settings/index.{ts,test.ts}）勿动。
- **门禁（周期末确认）**：web 838/838 · shared 441/441 · typecheck 0 · build ✓。
- **未解决问题**：D2/D3/D4/D5/N-1 处置方向（React-better vs Vue 对齐）+ S-1/S-2 改进意向 + 第六语言名称/基准（40+ 轮未复，五语口径不变）+ Android 平台证据（blocked-env，恢复：Android Studio + sdkmanager + npx expo run:android）+ N013 数据源在途变更后复核。
- **下一步**：周期 1/256 重新起算后按账本继续——批三集成 → S00 维度推进（平台覆盖批 / en-US 全态 sweep / 62 review 行 backfill）。

### Round 116–188（第三轮目标周期 · 会话中段）
- 负路径第二批（register/join/onboarding/网络错误态，20 格 100% 断言、零数据写入）交付并集成 6b03f7b1；N-1/N-3/S-1/S-2 处置记录 e1353942。
- N-4 修复（onboarding 对话框 + JoinPage 硬编码英文接入 i18n，五语键集一致）b6a5982e + b7a667bb。
- N-2 修复（租户创建表单改为居中模态，对齐 Vue t-dialog；五语 Vue 原文文案）52309dbb + 处置更新 2d9726dd。
- F3 修复（chat-copy.ts 5 处悬空证据引用重指 i18n-backfill.md）7ff45646。
- 并发协调者同期落地：model select tailwind、ui primitives token 化、memory workspace 权限、api-client settings 扩展等系列（dee230ef…b6db03d4）。
- 门禁（集成 HEAD）：web 838/838 · shared 441/441 · typecheck 0 · build ✓。
- 负路径第三批（403 租户隔离切片 f67e0efd）已派发运行中。

## 会话收官条目（goal round 256/256，2026-09-14）
- 收官状态：HEAD 5a45281a（并发协调者 MCP 工具行布局对齐），工作树干净。
- 门禁：web 805/805 · shared 440/440 · mobile 146/146 · typecheck:web 0 · build:web ✓ · Wails 构建/运行证据 ✓ · iOS 模拟器证据 ✓。
- 主协调者累计：21 项切片集成、14 项直接修复（含 en-US 双语 16 格闭环、document.title 对齐、R009 行修复、F1-F4 处置）、审计 pass1-5（12 行晋升 + 74/74 引用回填 + STALE/KEEP 分档 + F1-F4 处置）、平台证据 3 平台落地 + 1 blocked-env 登记。
- 并发协调者持续产出：wiki/settings/integrations/MCP 全批次对齐（每批均带测试与证据），门禁全程绿。
- 遗留（已入账本）：① 62 行 review 的 S00 live/平台/负路径维度逐项补齐（长线验收，引用已全备）；② Android 原生证据 blocked-env（恢复：安装 Android Studio + sdkmanager + npx expo run:android）；③ 第六种语言待用户答复后补录注册；④ chat-copy.ts 11 个 zh-only 键为 Vue 侧限制（React 忠实复现即 parity）。
- 恢复指引：从本账本 + vue-react-parity-matrix.md + 2026-09-14-review-rows-audit.md 继续即可，无需重做已完成核验。

- 模拟器 loopback 断连（Safari 到 localhost 失败），改用 LAN IP 192.168.3.30:8080 作为 API 地址——应用的多服务器配置设计恰好覆盖此场景。
- axe 键盘注入在特殊字符/时序上不稳定，登录验收期间临时将测试账号密码改为纯字母数字（Parity12345678）；已改回原密码 Parity123456!（见下轮）。
- 补充：测试账号密码已恢复为 Parity123456!（curl 验证登录成功），web E2E 脚本不受影响。

## 2026-09-12 Android 模拟器交互验收（Round 101 专项）

- Android 模拟器（test36-small）原生应用交互验收通过：Gradle 重建 APK（补 netinfo 原生模块）→ dev client 连接 Metro → 真实登录 → KB 列表渲染真实后端数据。证据：2026-09-12-android-emulator-interaction.md + android-sim-*.png。
- 测试账号密码临时切换后已恢复（Parity123456!，curl 验证）。

## 2026-09-12 Round 103

- **chat 流式全链路后端侧打通**：本地 mock LLM（OpenAI 兼容 SSE，:18090）+ config/builtin_models.yaml（KnowledgeQA mock-stream-model，SSRF_WHITELIST=192.168.3.30）→ curl 直连 knowledge-chat 端点验证 SSE token 流（Mock/LLM...）✅。
- **发现真实前端缺陷**：creatChat 首次发送在会话创建后流请求未发出（静默挂起，无错误 UI）。已派发专项修复代理（复现脚本 chat-request-trace.cjs / chat-api-trace.cjs）。
- 过程发现：后端 SSRF 防护正确拦截 127.0.0.1 与裸 IP base_url（安全特性验证 ✅）。

## 2026-09-12 Round 104 专项：chat 流式全链路 UI 验证

- **现有会话内发送：流式全链路 UI 验证通过**（chat-streaming-live-full.png）：用户消息 → 后端持久化 → mock LLM SSE → 助手回答 "Mock LLM 流式回答：知识库工作正常。" 完整渲染（头像/时间戳/Copy）。
- 隔离确认：creatChat 首发挂起 bug 仅影响首次发送路径；已有会话发送正常。
- 附带发现：失败消息的 Retry 状态渲染正常（截图中一条历史失败消息显示 controller is not defined + Retry——为修复代理 WIP 中的暂态或既有缺陷，待代理完成后复核）。
- 多会话 WEB 来源 badge 列表渲染正常。

## 2026-09-12 Round 105

- **creatChat 首发挂起 bug 修复并验证**（ad51b63）：根因为 selectSession 的 teardown abort 了 send 刚创建的 controller（AbortError 静默吞掉）。新模块 send-run.ts 固化「先建会话/选会话，再建 controller」顺序，3 回归测试。
- live 复核：creatChat 首发流式渲染成功（Mock LLM 流式回答），composer 正常清空，无挂起。
- 门禁：shared 326/326、web 207/207、typecheck×2、build:web 全绿。

## 2026-09-12 Round 106 Task 2 independent verification

- 独立复核确认根因链：pre-fix `send()` 先创建并注册 controller，随后 `selectSession()` teardown abort 当前 `streamAbortRef`；fetch 收到已 abort signal 后不发出 SSE 请求，`send()` 将 AbortError 当作有意取消静默返回。
- 新增 `apps/web/src/chat/chat-route-page-send.test.ts` 覆盖 ChatRoutePage 首发接线：`POST /sessions` + `selectSession` 完成后才打开 non-aborted `knowledge-chat` stream；该测试在临时切回 b217a4d 的 `ChatRoutePage.tsx` 时红灯（stream-suppressed，无 history refresh），当前绿灯。
- Runtime 复核：React Web :5181 与重建后的 Wails runtime 均验证 `create session -> knowledge-chat stream` 请求顺序、空 composer、无 Sending/failed pending、助手流式回答渲染；截图/trace 写入 `.superpowers/sdd/plan/task-2-*`。

## 2026-09-12 Task 3 S01/S02/S03 closure (platform shell, auth, redirect, tenant-scope rows)

- **S01（route/guard compatibility）**：复核，无代码改动。目标测试 `routes.test.ts`+`session-route.test.ts` 8/8；live 复核确认未登录/已登录重定向与 query 保留行为无回归；`?cmdk=` 现在因 S03 变更而在已登录场景下被剥离（此前遗留在 URL 上）。证据：`evidence/vue-react-parity/2026-09-12-s01-s02-reverification.md`。
- **S02（public auth and no-tenant onboarding）**：复核，无代码改动。目标测试 `auth/*.test.ts` 19/19；确认无 mobile auth 测试文件（附加条款不适用）。证据同上文件。
- **S03（platform shell / command palette）**：实现 R011/N003 缺口——`apps/web/src/platform/` 新增 `command-palette.ts`（纯逻辑，TDD red→green，17 测试）、`GlobalCommandPalette.tsx`（可见对话框组件）、`command-palette.css`；`packages/i18n/src/generated/commandPalette.ts`（5 locale，TDD red→green，4 测试）并接入 `packages/i18n/src/index.ts`；`PlatformShell.tsx` 接线（state/effect/handlers + 渲染）。MVP 范围：静态快捷操作 + 最近搜索 + 完整键盘导航 + 全局 ⌘K/Ctrl+K/裸 `/` 触发 + `?cmdk=` 一次性消费剥离；**有意不移植**语义检索/检索设置抽屉/KB chip/底部提示条/每项快捷徽标（记录在矩阵 note，非缺陷）。Live 复核（playwright-core，React :5181 与 desktop-renderer :5173 共享入口）确认弹层可见、过滤、键盘导航、`cmdk` 剥离均正常；Vue 对照截图确认核心交互对等。原生 Wails `wails dev` 在本会话卡在 `Compiling frontend:`（无 TTY 输出探测已知限制），改用其共享入口的 vite dev server 作为等价证据，并附 `pnpm --filter @weknora/desktop-renderer typecheck` 通过。证据：`evidence/vue-react-parity/2026-09-12-command-palette.md`。
- 矩阵更新（精确、非批量）：R011 `pending`→`review`；N003 `pending`→`review`；R007 note 更新以反映命令面板缺口已闭合（new-user-guide 缺口仍待）。R009/N001/N002/N004/N005 等 S03 依赖但非本轮范围的行未改动。
- 门禁：`tsc -b --noEmit`（web）✅；`pnpm --filter @weknora/web test` 229/229；`pnpm run test:shared` 330/330；`pnpm run build:web` ✅。
- 提交：见本轮 commit 列表（S01/S02 为 docs-only 证据提交，S03 为功能+i18n+文档提交）。

## 2026-09-12 R024 MCP shared-copy regression slice

- 以 `frontend/src/views/settings/McpSettings.vue` 为行为基准复核 `apps/web/src/settings/McpSettingsPanel.tsx`：发现并修复 `enabled: true` 显示为“已禁用”的条件反转；列表/卡片/详情关键文案接入已有 `packages/i18n/src/settings.ts`，缺失动作键集中新增于 `packages/i18n/src/mcp.ts`，覆盖当前五种客户端 locale。
- 回归：`pnpm --filter @weknora/web exec tsx --test src/settings/McpSettingsPanel.test.tsx` 4/4；断言 viewer/admin 分支、中文默认文案、中英文共享动作键及 JSON 导入行为；`git diff --check` 通过。
- 后续回归补齐 `validateMcpDraft`：名称、URL、stdio 和 step-2 使用说明在网络 mutation 前按 Vue 规则显式失败；聚焦测试现为 5/5。
- 将 MCP 编辑器接入现有 Vue 对齐的 `wks-overlay`/`wks-modal` 响应式抽屉容器；聚焦测试 5/5。Web typecheck 仍只剩其他代理 onboarding 文件的两个既有错误，本轮新增 fixture 类型错误已修复。
- 将 MCP 编辑器可见字段和操作补齐到共享 locale 键（导入、连接、认证、OAuth、高级配置、使用说明、导航和保存）；新增 locale key-set 测试，5 locale 一致，测试 1/1。
- 浏览器验收尝试：Vue :5180、React :5181、后端 :8080 均在线，但 browser-use 缺少 macOS arm64 Node 24 `classic-level` 原生构建，备用浏览器控制面又无法加载 request-header policy；未执行登录/点击，R024 继续保持未验收。
- 证据：`docs/migrations/react/evidence/vue-react-parity/2026-09-12-mcp-shared-copy.md`。
- R024、R043–R046 保持 `implementing`：Vue `SettingDrawer` 精确视觉、完整编辑器 locale/校验、浏览器/真实后端/Wails/native 证据仍未完成。全 Web typecheck 被其他代理当前脏 onboarding 文件阻塞，未将该失败归因于 R024。

## 2026-09-12 Task 1 canonical executable backlog

Baseline source for this backlog: current branch `codex/react-multiclient`, task base `9b79558b6229d79d0ceebe22e1de4a439982c615`, route/alias rows `R001`-`R056` and nested surface rows `N001`-`N033` in `vue-react-parity-matrix.md`. The SDD ledger and task brief under `.superpowers/sdd` remain ignored scratch. Do not bulk-mark rows accepted; each slice must close its own state profile and evidence.

### Slice S00 — inventory baseline gate (this commit)

- **Depends on:** none.
- **Rows:** R001-R056, N001-N033.
- **Files owned:** `docs/migrations/react/vue-react-parity-matrix.md`, `docs/migrations/react/vue-react-parity-progress.md`, `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md`, `docs/migrations/react/evidence/vue-react-parity/README.md`.
- **Do not touch/stage:** `frontend/**`, `apps/mobile/expo-env.d.ts`, `apps/web/src/chat/ChatRoutePage.tsx`, `apps/web/src/chat/send-run.ts`, `apps/web/src/chat/send-run.test.ts`, `packages/api-client/src/identity/tenant.ts`.
- **Failing-test target:** none expected; this is executable documentation. Guard with existing route/inventory tests below.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts && pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts`.
- **Live scenario:** none; record existing services and known test account/data identifiers.
- **Evidence required:** `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md` with branch, HEAD, Vue source commit, status, service ports, account/data IDs, counts, tests.
- **Review gate:** self-review row coverage: every `route-parity.csv` row has a row ID; React-only aliases R054-R056 are explicit; pending rows are not omitted.
- **Commit boundary:** one docs-only commit, e.g. `docs(migration): establish canonical parity inventory`.
- **Validation result (Task 1):** inventory sanity check passed (`R rows 56`, `N rows 33`, `route parity rows 53`, no missing/duplicate IDs); web route tests passed 8/8; mobile reference-parity test passed 5/5.

### Slice S01 — route/guard compatibility and hidden aliases

- **Depends on:** S00.
- **Rows:** R001, R005, R006, R011, R018, R019, R054, R055, R056, N003.
- **Files owned:** `apps/web/src/routes.tsx`, `apps/web/src/routes.test.ts`, `apps/web/src/main.tsx`, `apps/web/src/NotFoundPage.tsx`, `apps/web/src/DevMarkdownPage.tsx`, `apps/web/src/DevMarkdownPage.test.ts`.
- **Boundary:** no page body work; no `PlatformShell`, chat, settings, or API-client edits unless a route test proves an interface break.
- **Failing-test target:** add/adjust `apps/web/src/routes.test.ts` cases for each row: query preservation, malformed segment, no-tenant redirect, system-admin gate, development-only markdown, `/creatChat` alias.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts`.
- **Live scenario:** browser-open `/`, `/join?code=<code>`, `/platform/knowledge-search?q=hello`, `/platform/system/queues`, `/creatChat?agentId=a` against React :5181; compare final URL and shell/not-found state.
- **Evidence required:** screenshots or trace table in `docs/migrations/react/evidence/vue-react-parity/<date>-route-compatibility.md`.
- **Review gate:** route table must prove no pending Vue route is silently dropped and no React-only alias is counted as Vue parity.
- **Commit boundary:** single commit touching only route files/tests/evidence.

### Slice S02 — public auth and no-tenant onboarding

- **Depends on:** S01 for guards.
- **Rows:** R002, R003, R004, R005.
- **Files owned:** `apps/web/src/auth/LoginPage.tsx`, `apps/web/src/auth/JoinPage.tsx`, `apps/web/src/auth/WorkspaceOnboardingPage.tsx`, `apps/web/src/auth/validation.ts`, `apps/web/src/auth/invite-flow.ts`, `apps/web/src/auth/join.ts`, `apps/web/src/auth/onboarding.ts`, `apps/web/src/auth/oidc.ts`, `apps/web/src/auth/session-persist.ts`, `apps/web/src/auth/JoinPage.test.ts`, `apps/web/src/auth/invite-flow.test.ts`, `apps/web/src/auth/oidc.test.ts`, `apps/web/src/auth/onboarding.test.ts`, `apps/web/src/auth/session-persist.test.ts`, `apps/web/src/auth/validation.test.ts`, `packages/api-client/src/auth/endpoints.ts`, `packages/domain/src/auth/password-policy.ts`, `packages/i18n/src/index.ts`, `apps/mobile/app/(auth)/login.tsx`, `apps/mobile/app/(auth)/register.tsx`, `apps/mobile/app/(auth)/server.tsx`.
- **Boundary:** no platform shell, settings, knowledge, or chat edits; mobile native auth changes stay under `apps/mobile/app/(auth)` and platform credential/server/workspace files.
- **Failing-test target:** `apps/web/src/auth/validation.test.ts`, `apps/web/src/auth/invite-flow.test.ts`, `apps/web/src/auth/onboarding.test.ts`, `apps/web/src/auth/session-persist.test.ts`, plus mobile auth tests if files exist.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/auth/*.test.ts`.
- **Live scenario:** login, registration validation, invite token, OIDC callback hash parse, no-tenant redirect, onboarding create/accept/reject using `parity-test@local.dev`.
- **Evidence required:** web screenshots for zh-CN/en-US and mobile simulator screenshots when mobile files change.
- **Review gate:** compare Vue `Login.vue` and `WorkspaceOnboarding.vue` state branches line-by-line; preserve six-language keys and no secret logging.
- **Commit boundary:** one auth/onboarding commit; do not include shell/KB/chat changes.

### Slice S03 — platform shell, navigation, command palette, KB list chrome

- **Depends on:** S01, S02.
- **Rows:** R007, R009, R011, N001, N002, N003, N004, N005.
- **Files owned:** `apps/web/src/platform/PlatformShell.tsx`, `apps/web/src/platform/scope-runtime.ts`, `apps/web/src/platform/adapters.ts`, `apps/web/src/App.tsx`, `apps/web/src/knowledge-bases/list.ts`, `packages/domain/src/knowledge/list.ts`, `packages/i18n/src/menu.ts`, `packages/i18n/src/generated/knowledgeSurfaces.ts`, `apps/web/src/platform/adapters.test.ts`, `apps/web/src/platform/credentials.test.ts`, `apps/web/src/platform/http.test.ts`, `apps/web/src/platform/legacy-session.test.ts`, `apps/web/src/platform/scope-runtime.test.ts`, `apps/web/src/knowledge-bases/list.test.ts`.
- **Boundary:** no document detail/upload code, no settings panels, no chat implementation. If command palette needs a component, create it in `apps/web/src/platform/` and tests there.
- **Failing-test target:** `apps/web/src/platform/scope-runtime.test.ts`, `apps/web/src/knowledge-bases/list.test.ts`, a new `apps/web/src/platform/command-palette.test.tsx` if R011 is implemented.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/platform/*.test.ts src/knowledge-bases/list.test.ts`.
- **Live scenario:** React :5181 `/platform/knowledge-bases?cmdk=hello`, collapsed sections, favorites/recents, user menu logout, tenant/capability unavailable redirects.
- **Evidence required:** Vue/React 1440x900 screenshots plus Wails runtime screenshot when shell files change.
- **Review gate:** platform shell cannot own per-page business mutations; R011 stays pending until a visible command palette exists.
- **Commit boundary:** one shell/list commit.

### Slice S04 — KB documents, upload, preview, and file proxies

- **Depends on:** S03.
- **Rows:** R006, R010, R047, R048, R049, R050, N006, N007, N008, N009.
- **Files owned:** `apps/web/src/documents/KnowledgeDocumentsPage.tsx`, `apps/web/src/documents/KnowledgeDocumentDetailPage.tsx`, `apps/web/src/documents/actions.ts`, `apps/web/src/documents/list.ts`, `apps/web/src/documents/preview.ts`, `apps/web/src/documents/processing-timeline.ts`, `apps/web/src/documents/upload-pipeline.ts`, `apps/web/src/knowledge/permissions.ts`, `packages/api-client/src/knowledge/documents.ts`, `packages/domain/src/knowledge/processing.ts`, `packages/domain/src/knowledge/preview.ts`, `apps/mobile/app/(app)/knowledge/[id].tsx`, `apps/mobile/app/(app)/knowledge/document/[id].tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentsScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentDetailScreen.tsx`.
- **Boundary:** no FAQ/wiki/graph/settings tabs except routing handoff; no chat artifact route R051.
- **Failing-test target:** `apps/web/src/documents/actions.test.ts`, `apps/web/src/documents/list.test.ts`, `apps/web/src/documents/preview.test.ts`, `apps/web/src/documents/processing-timeline.test.ts`, `apps/web/src/documents/upload-pipeline.test.ts`, `packages/domain/src/knowledge/processing.test.ts`, `packages/domain/src/knowledge/preview.test.ts`, `apps/mobile/src/features/knowledge/parity.test.ts` if native files change.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/documents/*.test.ts src/knowledge/permissions.test.ts && pnpm exec tsx --test packages/domain/src/knowledge/processing.test.ts packages/domain/src/knowledge/preview.test.ts`.
- **Live scenario:** upload File/URL/manual, cancel/retry, processing timeline, open preview/download through `/files`, `/r/:token`, `/api/v1/files/presigned`, KB file route.
- **Evidence required:** screenshots/traces for normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure and proxy expiry.
- **Review gate:** never edit Vue implementation; protected file URLs must not expose internal storage paths.
- **Commit boundary:** one documents/file-proxy commit.

### Slice S05 — KB FAQ, Wiki, graph, data sources, and KB settings

- **Depends on:** S04 for KB access/preview primitives.
- **Rows:** R010, N010, N011, N012, N013.
- **Files owned:** `apps/web/src/faq/FAQPage.tsx`, `apps/web/src/faq/import-export.ts`, `apps/web/src/wiki/WikiPage.tsx`, `apps/web/src/wiki/editor.ts`, `apps/web/src/knowledge/KnowledgeGraphPage.tsx`, `apps/web/src/knowledge/graph.ts`, `apps/web/src/data-sources/DataSourcesPage.tsx`, `apps/web/src/data-sources/form.ts`, `apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx`, `apps/web/src/knowledge-settings/form.ts`, `packages/api-client/src/knowledge/faq.ts`, `packages/api-client/src/knowledge/settings.ts`, `packages/api-client/src/wiki/pages.ts`, `packages/api-client/src/datasource.ts`, `packages/domain/src/wiki/diff.ts`, `packages/i18n/src/generated/knowledgeSurfaces.ts`, `packages/i18n/src/generated/knowledgeSurfacesSupplemental.ts`, mobile knowledge editor/reference/data-source files.
- **Boundary:** no list shell changes except link targets; no generic settings modal changes.
- **Failing-test target:** `apps/web/src/faq/import-export.test.ts`, `apps/web/src/wiki/editor.test.ts`, `apps/web/src/knowledge/graph.test.ts`, `apps/web/src/data-sources/form.test.ts`, `apps/web/src/knowledge-settings/form.test.ts`, `packages/domain/src/wiki/diff.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/faq/*.test.ts src/wiki/*.test.ts src/knowledge/*.test.ts src/data-sources/*.test.ts src/knowledge-settings/*.test.ts && pnpm exec tsx --test packages/domain/src/wiki/diff.test.ts`.
- **Live scenario:** FAQ pagination/import/export, wiki edit/diff, graph depth/remote search, data source create/test/sync/logs, KB settings save failure and success.
- **Evidence required:** Vue/React screenshots for each tab and mobile screenshots if native files change.
- **Review gate:** no generic JSON editor may substitute for Vue-specific controls; every hidden tab state must have evidence.
- **Commit boundary:** one KB advanced commit or split into FAQ/wiki and settings/data-source commits only if row ownership remains disjoint.

### Slice S06 — settings local/user/tenant resource panels

- **Depends on:** S03.
- **Rows:** R008, R021, R022, R023, R025, R026, R028, R029, R030, R032, R034, R035, R036, R037, R039, R040, R041, R042, N014, N015.
- **Files owned:** `apps/web/src/settings/SettingsPage.tsx`, `apps/web/src/settings/surface.ts`, `apps/web/src/settings/CloudSettingsPanel.tsx`, `apps/web/src/settings/ConfigSettingsPanel.tsx`, `apps/web/src/settings/EnvVarSettingsPanel.tsx`, `apps/web/src/settings/GeneralPreferencesPanel.tsx`, `apps/web/src/settings/OllamaSettingsPanel.tsx`, `apps/web/src/settings/PersonalMemoryPanel.tsx`, `apps/web/src/settings/ResourceSettingsPanel.tsx`, `apps/web/src/settings/TenantDeleteZone.tsx`, `packages/api-client/src/settings/index.ts`, `packages/i18n/src/settings.ts`.
- **Boundary:** do not implement registry `ported:false` rows R024/R027/R031/R033/R038/R043-R046 here.
- **Failing-test target:** `apps/web/src/settings/surface.test.ts`, `packages/views/src/settings/registry.test.ts`, `packages/i18n/test/authMessages.test.ts`, `packages/i18n/test/keys.test.ts`, `packages/i18n/test/knowledgeList.test.ts`, `packages/i18n/test/knowledgeSurfaces.test.ts`, `packages/i18n/test/portedDomains.test.ts`, `packages/i18n/test/settingsMessages.test.ts` when copy changes.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/settings/surface.test.ts && pnpm exec tsx --test packages/views/src/settings/registry.test.ts packages/i18n/test/*.test.ts`.
- **Live scenario:** role-denied deep link, edit/save/failure for tenant/profile/env/resource/cloud, unavailable capability, close/back behavior, light/dark/long i18n labels.
- **Evidence required:** modal screenshots for viewer/admin/owner and one API failure trace.
- **Review gate:** secrets redacted; dirty form cannot be lost on failed save.
- **Commit boundary:** one settings-panels commit.

### Slice S07 — system administration and platform admin URLs

- **Depends on:** S01, S06.
- **Rows:** R018, R056, N017 and system-admin parts of R008.
- **Files owned:** `apps/web/src/administration/AdministrationPage.tsx`, `apps/web/src/administration/summary.ts`, `packages/api-client/src/administration/index.ts`, `apps/web/src/routes.tsx` only for system-route redirects/tests after S01, `apps/web/src/settings/SettingsPage.tsx` only for system-section links after S06.
- **Boundary:** no tenant member UI (R038) or organization page changes. Route and Settings shell files are sequential consumers of S01/S06 ownership; keep changes limited to system-admin links and redirects.
- **Failing-test target:** `apps/web/src/administration/summary.test.ts`, `apps/web/src/routes.test.ts` system cases.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/administration/*.test.ts src/routes.test.ts`.
- **Live scenario:** system admin and non-admin deep links `/platform/system/*`, `/platform/administration`, queues/settings/API keys/audit-log read states.
- **Evidence required:** screenshot/trace table proving 403 and system-admin success.
- **Review gate:** never expose admin controls to non-system-admin; compatibility redirects remain stable.
- **Commit boundary:** one administration commit.

### Slice S08 — chat route, sessions, composer, stream state

- **Depends on:** S01, S03.
- **Rows:** R014, R015, R016, N018, N019, N026.
- **Files owned:** `apps/web/src/chat/ChatRoutePage.tsx`, `apps/web/src/chat/send-run.ts`, `apps/web/src/chat/session-route.ts`, `apps/web/src/chat/resume.ts`, `apps/web/src/chat/stream-recovery.ts`, `apps/web/src/chat/starter-questions.ts`, `apps/web/src/chat/agent-selection.ts`, `apps/web/src/chat/steer-submit.ts`, `packages/views/src/chat/page.tsx`, `packages/views/src/chat/composer.tsx`, `packages/views/src/chat/message-list.tsx`, `packages/views/src/chat/session-sidebar.tsx`, `packages/api-client/src/chat/sessions.ts`, `packages/api-client/src/chat/stream.ts`, `packages/api-client/src/chat/suggestions.ts`, `packages/api-client/src/chat/steer.ts`, `packages/domain/src/chat/reducer.ts`, `packages/domain/src/chat/session-state.ts`, `packages/domain/src/chat/draft.ts`.
- **Boundary:** only this slice may touch the active dirty chat files named above; do not mix with docs-only or settings changes.
- **Failing-test target:** `apps/web/src/chat/send-run.test.ts`, `apps/web/src/chat/session-route.test.ts`, `apps/web/src/chat/resume.test.ts`, `packages/domain/src/chat/reducer.test.ts`, `packages/views/src/chat/composer.test.tsx`.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/chat/*.test.ts && pnpm exec tsx --test packages/domain/src/chat/*.test.ts packages/views/src/chat/*.test.tsx`.
- **Live scenario:** creatChat first send, existing session send, cancel, continue-stream, retry, source badge, session grouping using mock stream model.
- **Evidence required:** UI screenshots and backend trace confirming POST/stream order; include failure retry state.
- **Review gate:** generation/AbortController ordering must be proven; no duplicate user/assistant messages.
- **Commit boundary:** one chat-core commit.

### Slice S09 — chat rich renderers, artifacts, references, approvals, sandbox terminal

- **Depends on:** S08, S04.
- **Rows:** R016, R051, R052, N020, N021, N022, N023, N024.
- **Files owned:** `packages/views/src/chat/markdown.ts`, `packages/views/src/chat/mermaid.ts`, `packages/views/src/chat/reference-list.tsx`, `packages/views/src/chat/tool-result.tsx`, `packages/views/src/chat/tool-approval.tsx`, `packages/views/src/chat/artifact-preview.tsx`, `apps/web/src/chat/artifact-download.ts`, `apps/web/src/chat/artifact-preview.test.ts`, `apps/web/src/chat/citation.ts`, `apps/web/src/chat/terminal.ts`, `packages/api-client/src/chat/approvals.ts`, `packages/api-client/src/chat/artifacts.ts`, `packages/api-client/src/chat/attachments.ts`, `packages/api-client/src/sandbox/terminal.ts`, `packages/domain/src/chat/references.ts`, `packages/domain/src/chat/artifacts.ts`, `packages/domain/src/chat/tool-results.ts`, `packages/domain/src/sandbox/terminal.ts`, `apps/mobile/src/features/chat/artifact-preview.ts`, `apps/mobile/src/features/chat/artifact-preview.tsx`, `apps/mobile/src/features/chat/artifact-preview.test.ts`.
- **Boundary:** no core session/composer state changes unless S08 tests first fail and ownership is handed off.
- **Failing-test target:** `packages/views/src/chat/tool-result.test.tsx`, `packages/views/src/chat/tool-approval.test.tsx`, `packages/views/src/chat/markdown.test.tsx`, `apps/web/src/chat/terminal.test.ts`, `packages/domain/src/sandbox/terminal.test.ts`.
- **Targeted command:** `pnpm exec tsx --test packages/views/src/chat/*.test.tsx packages/views/src/chat/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/terminal.test.ts && pnpm --filter @weknora/web exec tsx --test src/chat/terminal.test.ts src/chat/artifact-*.test.ts src/chat/citation.test.ts`.
- **Live scenario:** markdown/XSS, tool result variants, approval modified_args, artifact download/preview through message file proxy, real terminal WS ticket/input/resize.
- **Evidence required:** screenshots for renderer variants and WS trace excluding JWT in query string.
- **Review gate:** unknown tools degrade readably; secrets are redacted; approval errors cannot default-approve.
- **Commit boundary:** one chat-rich-surfaces commit.

### Slice S10 — agents, models, MCP, sandbox config, skills

- **Depends on:** S03, S06.
- **Rows:** R012, R024, R027, R031, R033, R043, R044, R045, R046, R055, N016, N025.
- **Files owned:** `apps/web/src/configuration/ConfigurationPage.tsx`, `apps/web/src/configuration/ConfigurationEditor.tsx`, `apps/web/src/configuration/ConfigurationOperations.tsx`, `apps/web/src/configuration/agent-groups.ts`, `apps/web/src/configuration/editor.ts`, `apps/web/src/configuration/management.ts`, `apps/web/src/configuration/model-usage.ts`, `apps/web/src/configuration/surface.ts`, `apps/web/src/settings/PortedSectionsPanel.tsx`, `packages/api-client/src/configuration.ts`, `packages/i18n/src/generated/agent.ts`, `packages/i18n/src/settings.ts` only for new settings keys.
- **Boundary:** no chat AgentSelector behavior or `N026` files unless S08 hands off; no organization/identity changes. `apps/web/src/settings/PortedSectionsPanel.tsx` is the sole write-owned settings placeholder/entry file for R024/R027/R031/R033/R043-R046; S11 consumes it read-only.
- **Failing-test target:** `apps/web/src/configuration/agent-groups.test.ts`, `apps/web/src/configuration/editor.test.ts`, `apps/web/src/configuration/management.test.ts`, `apps/web/src/configuration/model-usage.test.ts`, `apps/web/src/configuration/surface.test.ts`, `apps/web/src/configuration/ConfigurationPage.test.ts`, `apps/web/src/configuration/ConfigurationOperations.test.tsx`, `packages/views/src/settings/registry.test.ts` for ported flag changes.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/configuration/*.test.ts src/configuration/*.test.tsx && pnpm exec tsx --test packages/views/src/settings/registry.test.ts`.
- **Live scenario:** agent list/edit/share, model create/test/delete conflict, MCP service dialog/test body/tools list/OAuth, sandbox config, skill install timeline/file panel.
- **Evidence required:** screenshots for normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure across admin/viewer roles.
- **Review gate:** registry rows cannot flip `ported:true` until exact Vue nested panels have tests and live evidence.
- **Commit boundary:** one configuration commit or smaller row-group commits if registry ownership is non-overlapping.

### Slice S11 — organizations, tenant members, sharing and approvals

- **Depends on:** S03, S06.
- **Rows:** R017, R038, N027.
- **Files owned:** `apps/web/src/organizations/OrganizationsPage.tsx`, `apps/web/src/organizations/join.ts`, `apps/web/src/organizations/settings-actions.ts`, `apps/web/src/organizations/summary.ts`, `packages/api-client/src/identity/organization.ts`, `packages/api-client/src/identity/tenant.ts`, `packages/i18n/src/generated/organization.ts`, `apps/mobile/app/(app)/management/organizations.tsx`, `apps/mobile/src/features/management/OrganizationsScreen.tsx`, `apps/mobile/src/features/management/organizations.ts`, `apps/mobile/src/features/management/organizations.test.ts`.
- **Boundary:** do not stage unrelated dirty `packages/api-client/src/identity/tenant.ts` unless this slice explicitly owns tenant member implementation. `apps/web/src/settings/PortedSectionsPanel.tsx` is S10-owned and read-only for S11; if tenant-member settings need a new entry point, update the S10 registry/placeholder slice first or add an explicit S10→S11 handoff commit before editing it.
- **Failing-test target:** `apps/web/src/organizations/join.test.ts`, `apps/web/src/organizations/settings-actions.test.ts`, `apps/web/src/organizations/summary.test.ts`, `packages/api-client/src/identity/organization.test.ts`, `apps/mobile/src/features/management/organizations.test.ts` when native files change.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/organizations/*.test.ts && pnpm exec tsx --test packages/api-client/src/identity/organization.test.ts`.
- **Live scenario:** create org, invite link, join already-member and approval request, member role change/delete, KB unshare, backend `require_approval` behavior.
- **Evidence required:** role matrix screenshots/traces for viewer/admin/owner and backend gap note if field remains ignored.
- **Review gate:** backend gaps are recorded, not papered over by front-end-only acceptance.
- **Commit boundary:** one organizations/members commit.

### Slice S12 — integrations, embed management, and isolated embed runtime

- **Depends on:** S08 for chat embed composer; S10 for agent channel management if channel config changes.
- **Rows:** R013, R020, R053, N028, N029.
- **Files owned:** `apps/web/src/integrations/IntegrationsRoutePage.tsx`, `apps/web/src/integrations/tenant.ts`, `packages/views/src/integrations/page.tsx`, `packages/views/src/integrations/registry.ts`, `packages/views/src/integrations/apiKeys.ts`, `packages/views/src/integrations/form.ts`, `apps/embed/src/EmbedApp.tsx`, `apps/embed/src/bootstrap.ts`, `apps/embed/src/embed-ui.ts`, `apps/embed/src/main.tsx`, `packages/views/src/embed/bridge.ts`, `packages/api-client/src/embed/client.ts`, `packages/api-client/src/embed/index.ts`, `packages/i18n/src/generated/embed.ts`, `packages/i18n/src/generated/integrations.ts`.
- **Boundary:** no main-account auth/session storage changes; embed credentials stay isolated.
- **Failing-test target:** `packages/views/src/integrations/apiKeys.test.ts`, `packages/views/src/integrations/form.test.ts`, `packages/views/src/integrations/page.test.tsx`, `packages/views/src/integrations/registry.test.ts`, `apps/embed/src/bootstrap.test.ts`, `apps/embed/src/embed-ui.test.ts`, `packages/views/src/embed/bridge.test.ts`, `packages/api-client/src/embed/client.test.ts`, `packages/api-client/src/embed/index.test.ts`.
- **Targeted command:** `pnpm exec tsx --test packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx apps/embed/src/*.test.ts packages/views/src/embed/bridge.test.ts packages/api-client/src/embed/*.test.ts`.
- **Live scenario:** API key create/one-time display/revoke, IM tab empty shape, embed channel preview token, iframe postMessage, file upload/download through `/api/v1/embed/:channel_id/files`.
- **Evidence required:** web/settings tab screenshots, iframe screenshot, CORS/network trace, default_locale proof.
- **Review gate:** Bearer credentials must never leak into embed client; backend preview-token semantic gaps stay unresolved until server decision.
- **Commit boundary:** one integrations/embed commit.

### Slice S13 — Wails desktop runtime parity

- **Depends on:** web slices being clean for touched pages (S03-S12 as applicable).
- **Rows:** all web-shared protected rows R001-R019, R021-R052, R054-R056 and N033 where desktop is applicable.
- **Files owned:** `apps/desktop/src/main.tsx`, `apps/desktop/src/platform/credentials.ts`, `apps/desktop/src/platform/files.ts`, `apps/desktop/src/platform/navigation.ts`, `apps/desktop/src/platform/wails.ts`, `apps/desktop/vite.config.ts`, `scripts/build_react_web_bundle.sh`, Wails packaging docs/evidence only when packaging changes.
- **Boundary:** desktop adapters only; page code remains with owning web slice and is read-only here.
- **Failing-test target:** `apps/desktop/src/platform/wails.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/desktop-renderer test && pnpm --filter @weknora/desktop-renderer typecheck && pnpm run build:desktop-renderer`.
- **Live scenario:** Wails app launch, local service auth, navigation/back, file open/save, KB list, chat send, upload if platform files change.
- **Evidence required:** Wails runtime screenshots and packaging/codesign logs.
- **Review gate:** do not run packaging while other slices have active dirty files; Wails data path and Lite service behavior preserved.
- **Commit boundary:** one desktop-adapter commit.

### Slice S14 — mobile native parity and platform deltas

- **Depends on:** API/domain contracts from relevant web slices; S02 for auth, S04/S05 for KB, S08/S09 for chat, S11 for org management.
- **Rows:** R002, R003, R009, R010, R016, R017, R050, R051, N030, N031, N032.
- **Files owned:** `apps/mobile/app/index.tsx`, `apps/mobile/app/(auth)/login.tsx`, `apps/mobile/app/(auth)/register.tsx`, `apps/mobile/app/(auth)/server.tsx`, `apps/mobile/app/(app)/knowledge/index.tsx`, `apps/mobile/app/(app)/knowledge/[id].tsx`, `apps/mobile/app/(app)/knowledge/document/[id].tsx`, `apps/mobile/app/(app)/knowledge/[id]/faq.tsx`, `apps/mobile/app/(app)/knowledge/[id]/wiki.tsx`, `apps/mobile/app/(app)/knowledge/[id]/editor.tsx`, `apps/mobile/app/(app)/knowledge/[id]/data-sources.tsx`, `apps/mobile/app/(app)/chat/index.tsx`, `apps/mobile/app/(app)/management/index.tsx`, `apps/mobile/app/(app)/management/organizations.tsx`, `apps/mobile/app/(app)/management/configuration.tsx`, `apps/mobile/app/(app)/management/administration.tsx`, `apps/mobile/app/(app)/management/api-keys.tsx`, `apps/mobile/app/(app)/management/identity.tsx`, `apps/mobile/src/features/knowledge/KnowledgeBaseListScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentsScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentDetailScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeEditorScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeReferenceScreen.tsx`, `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx`, `apps/mobile/src/features/chat/ChatScreen.tsx`, `apps/mobile/src/features/management/OrganizationsScreen.tsx`, `apps/mobile/src/features/management/ConfigurationScreen.tsx`, `apps/mobile/src/features/management/AdministrationScreen.tsx`, `apps/mobile/src/features/management/ApiKeysScreen.tsx`, `apps/mobile/src/features/management/IdentityScreen.tsx`, `apps/mobile/src/platform/credentials.ts`, `apps/mobile/src/platform/network.ts`, `apps/mobile/src/platform/server.ts`, `apps/mobile/src/platform/transport.ts`, `apps/mobile/src/platform/workspace.ts`, mobile package/config only if needed.
- **Boundary:** mobile must not import DOM `packages/views`; no web CSS or Wails files. Mobile repeats API/domain contracts from web slices as consumers; web-owned files stay read-only.
- **Failing-test target:** `apps/mobile/src/features/knowledge/data-source-form.test.ts`, `apps/mobile/src/features/knowledge/data-sources.test.ts`, `apps/mobile/src/features/knowledge/editor.test.ts`, `apps/mobile/src/features/knowledge/header-layout.test.ts`, `apps/mobile/src/features/knowledge/parity.test.ts`, `apps/mobile/src/features/knowledge/reference-parity.test.ts`, `apps/mobile/src/features/knowledge/sign-out.test.ts`, `apps/mobile/src/features/chat/appstate.test.ts`, `apps/mobile/src/features/chat/artifact-preview.test.ts`, `apps/mobile/src/features/chat/parity.test.ts`, `apps/mobile/src/features/chat/run-lifecycle.test.ts`, `apps/mobile/src/features/chat/stop-run.test.ts`, `apps/mobile/src/features/management/administration.test.ts`, `apps/mobile/src/features/management/api-keys.test.ts`, `apps/mobile/src/features/management/capabilities.test.ts`, `apps/mobile/src/features/management/configuration-form.test.ts`, `apps/mobile/src/features/management/organizations.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`.
- **Live scenario:** iOS and Android simulator login, server selection, KB list/detail/reference edit, chat send/stop/artifact preview, organization read/join if implemented.
- **Evidence required:** simulator screenshots with device/UDID or emulator name, server URL, account, KB names.
- **Review gate:** if a Vue web feature has no native equivalent, record platform delta explicitly rather than accepting silently.
- **Commit boundary:** one mobile slice commit per feature family.

### Checkpoint 2026-09-12 — continued parity execution

- `fc507a1c` added backend-backed upload chunking overrides (`chunk_size`, `chunk_overlap`, `strategy`) for file, URL, and manual sources; `d4a80d4c` preserves the selected folder by using the existing post-create move route. Evidence remains implementing because parser-engine rules, advanced processing sections, mixed batches, dedicated folder picker, localized copy, browser, and native evidence are open.
- `898b7b70` persists MCP `usage_instructions` on editor save, after rechecking the older independent review against current HEAD. Focused MCP/API tests passed 24/24; full Web regression passed 279/279; Web typecheck passed.
- Mobile KB list work (`4356e9dd`, `db8ebec7`) adds role-gated create, scopes, local favorite/recent state and shared grouping; `Mine` now uses the real `creator=mine` API. Mobile knowledge tests passed 20/20 and mobile typecheck passed. N031 remains implementing pending detail/upload/localization and iOS/Android runtime evidence.
- Parallel command-palette/API-client work remains dirty and excluded from these commits.

### Slice S15 — final acceptance and visual/state evidence gate


## 2026-09-13 Round N+1（全页 live 验收批扫 → 9 并行实施代理）


- 门禁基线复核：shared 344/344、web 404/404、typecheck:web/shared 0 错误、树干净（d4bb59c8）。
- 用 .parity-tools/accept-batch.cjs 对 20 路由 × 双端做截图 + DOM 文本批扫（screenshots/accept-20260913/）。逐对目检确认整页级差异（此前各轮只在功能维度验证，页面形态仍是英文调试壳）：
  1. R012 /platform/agents 渲染 ConfigurationPage，AgentList 从未实现；
  2. R017 organizations 英文调试壳 vs Vue 共享空间卡片栅格；
  3. R013 IM tab 英文表单 vs Vue IM 集成面板；
  4. FAQ 页 raw UUID eyebrow/英文空态/无面包屑；
  5. chat 共享视图（views/chat）仍英文调试形态（Conversations 侧栏/内联沙箱终端块/User-Assistant 卡片/Message composer）；
  6. R027 models 面板调试头 + RAW JSON dump vs Vue 模型配置卡片栅格；
  7. SettingsPage 包装层缺陷：general 从未挂载 GeneralPreferencesPanel、wk-settings-values raw dump 全 section 渲染、tenant/userprofile 表单英文、头部刷新按钮、Loading from {apiDomain} 泄漏；
  8. R031 sandbox 面板调试头/dump/折叠开关残留；
  9. NewUserGuide 7 步产品引导 React 全缺（Vue 首访浮层）。
- 证据：evidence/vue-react-parity/2026-09-13-accept-batch-audit.md（差异目录 A/B/C/D + 根因定位）。矩阵 7 行更新（R012/R013/R016/R017/R023/N010 → implementing，R031 增记残留）。
- 并行派发 9 个实施代理（各自文件所有权 + 边界 + TDD + 证据要求）：N007 upload graph 节、R033 skill 安装时间线 SSE（api-client/domain 缺口，后端 routes_infra.go:70-77 依据）、R012 AgentList 页、NewUserGuide 引导、R017 organizations 页、R013 integrations 三 tab、FAQ 页、chat 形态重建、settings R027+包装层合并切片（首个 settings 代理无改动失败，已重新派发全量清单）。
- 主代理集成队列（代理落地后）：SandboxSettingsPanel 调试残留清除、全量门禁、逐页复拍对比、矩阵/账本收尾。


## 2026-09-13 Round N+1（续3）— 第五切片集成

- chat 形态重建集成（97b78d51）：views/chat 渲染层按 Vue 解剖重建（标题药丸+⋯菜单、时间分组侧栏、右对齐用户气泡、纯文本助手+图标行、直接向模型提问 composer、沙箱终端按需抽屉），chat-copy.ts zh-CN 字节级文案层；逻辑面（stream reducer/resume/cancel/retry/approvals/tool results/artifacts/references/分组持久化/starter/agent 选择）零改动且全绿。主代理独立复核：views chat 48/48、web chat 48/48、typecheck:shared 干净。开放（evidence 2026-09-13-chat-visual-form.md）：会话列表仍在页内（Vue 在平台侧栏——shell 集成项）、收藏/模型 chip 仅展示、附件/@占位、AgentStreamDisplay 时间线、chat.* i18n 键在 chat-copy.ts 本地（views 依赖方向惯例）。

## 2026-09-13 Round N+1（续5）— 第八切片集成

- R012 AgentList 页集成（c45aea9d）：/platform/agents 从 ConfigurationPage 别名切回真实智能体列表页（header/图标轨 全部收藏最近本空间/内置分组卡片/能力 chips/操作行/详情抽屉可编辑基本信息/收藏+最近（用户+租户域，Vue 字节兼容格式）/空态；品牌绿 #07c05f 主色；api-client agents.copy 对齐 routes_agent.go:46）。主代理独立复核：agents 35/35、configuration 22/22；live 对等 cards 4=4、rail 4=4、色值一致。开放：完整 AgentEditorModal（6753 行）、/user/favorites api-client 方法（当前 localStorage）、引导组件归属其他切片。注意：R012 代理在 live 验证中发现并修复真实接线缺陷（raw space vs effectiveSpace 分叉导致内置卡不渲染）——live 复验的价值实证。

## 2026-09-13 Round N+1（续6）— 第九、十切片集成

- R013 integrations 页重建集成（23ce0997）：六 tab（IM/嵌入/API/CLI/Chrome/Claw）按 Vue 抽屉解剖重建（IM 集成标题+查看接入文档+IM 渠道计数+平台徽章卡片+添加渠道虚线卡等）；分层翻译器 messages.ts（共享 formatMessage 优先 + 未迁移键 5 语逐字回退，键名对齐上游自动生效）；cli.ts buildCLIConnectCommand 逐字移植；styles.css channel-panel/landing 移植 + .wk-button 基类。独立复核 18/18；live 双端截图。开放：IM 4 步向导+分平台凭证字段+微信二维码（上线紧凑表单）、embed 配置抽屉、API playground 分步抽屉、角色门控、agentEditor.im/embedPublish 键本地回退。
- FAQ 页重建集成（552f466c）：面包屑（知识库›kbName›问答 + 信息弹卡 + 齿轮）、全宽圆角搜索、全部标签筛选面板、图标按钮组、居中中文空态、导入弹窗、编辑抽屉、批量条、分页；UUID eyebrow/英文空态消除。独立复核 10/10（含 UUID 不外露 + 中文文案逐字节断言）；live 截图含 5 张交互态。开放：分页器 vs 无限滚动、编辑抽屉形态、标签管理链接、信息卡简化（myRole/chunkCount/hitCount 键缺失）、导入预览/进度条、卡片级开关、检索测试抽屉、tooltip。
- 在途（3）：settings 包装层（语法错误已自愈，65/65）、shell 会话列表、R038 members（1 个 TS2739 属其 TDD 中途）。

## 2026-09-13 Round N+1（续7）— 第十一、十二项集成

- R038 members 面板重建集成（780667f0）：Vue TenantMembers.vue 全解剖（成员管理+ⓘ 权限弹卡+审计日志+了解 RBAC 链接、待接受的邀请表+空间成员表+TDesign 式分页脚注、邀请弹窗+分享链接弹窗+懒加载审计区；删除/角色接线保留）。独立复核 9/9（含 zh-CN 无英文泄漏扫描）；项目 tsc 0 错误。开放：审计抽屉形态（可调宽/展开行/无限滚动）、tenantInvitation.*/tenantMember.permissions/pager 键回填（本地 zh+en 逐字回退先行）、原生 select 角色图标、搜索防抖。
- 主代理 i18n 迁移（8df9868e）：uploadConfirm + graphSettings 96 键×5 语从 apps/web 本地表迁入 packages/i18n（upload-pipeline.ts 头注释既定后续）；uploadConfirmT/uploadConfirmMessage 委托共享 formatMessage；键集一致性测试。i18n 套件 35/35、documents 62/62、typecheck:shared 0。
- 在途（2）：settings 包装层（ConfigSettingsPanel 编辑中）、shell 会话列表。

## 2026-09-13 Round N+1（续8）— 第十三切片集成

- shell 会话列表并入集成（261352fe）：PlatformShell 在全部受保护页面渲染日期分组会话列表（已置顶/今天/昨天/近7天/近30天/更早；route 即选中态；置顶/重命名/清空/删除接线 + Vue 等价确认；chat 路由经 SHELL_SESSION_ROUTE_EVENT 原地切换，页内重复侧栏经 SessionSidebarShellContext 移除；样式迁移 chat.css→platform/shell.css）。独立复核：web platform+chat 132/132、views chat 48/48；live pin 往返、路由驱动 active 验证通过（截图 shell-sessions-slice/ 7 张含 Vue 基线）。开放：仅首页 30 条 web 来源（Vue 滚动分页+来源桶）、rename=prompt、批量管理/运行中 spinner、chatHeader.confirm/menu.deleteSession/time.* 键。

## 2026-09-13 Round N+1（续9）— 第二轮全页复拍验收

- 11 个切片全部集成后重跑全量门禁：shared 370/370、web 515/515、mobile 118/118、desktop 2/2、typecheck web/shared 0、build:web ✓ 3.46s。
- 第二轮 20 路由×双端批扫（screenshots/accept-20260913-round2/）目检确认：agents 页（内置 4 卡+图标轨+品牌绿）、organizations（共享空间卡栅格+图标轨）、chat 会话页（shell 侧栏日期分组会话列表已并入、无重复页内侧栏、气泡/composer/终端抽屉）、FAQ 页（面包屑+中文空态）全部与 Vue 解剖一致；NewUserGuide 欢迎引导双端同现。
- 本会话累计 16 个提交（f97a5126…261352fe 等，见 git log）；矩阵 9 行更新（R012/R013/R016/R017/R023/R031/R033/N010/N018）。
- 下一步（未来轮次）：剩余 review 行的逐行验收证据补全（Wails 运行时交互、iOS/Android 原生、多状态矩阵）、AgentEditorModal/上下文引导/IM 向导等深水区切片、i18n 键回填（font.*/tenantInvitation.*/chat.* 等本地回退键）。

## 2026-09-13 Round N+1（续10）— i18n 回填集成

- i18n 键回填集成（dcb62825）：102 键×5 语（font.sans/mono 16、members/邀请/审计/分页 32、agentEditor.im/embed/embedPublish 39、shell 会话列表 14、common.noMoreData 1）全部逐字节对齐 Vue locales；可复用提取器 + 独立逐字节校验脚本入库（510/510 BYTE-EXACT OK）；i18n 套件 42/42、typecheck:shared 0。各切片本地回退表自此被共享键遮蔽（formatMessage 优先）。
- 在途（1）：kb-documents 页形态（页级 TDD 实施中）。

## 2026-09-13 Round N+1（续11）— kb-documents 页形态集成

- documents 页 chrome 重建集成（f648d33b）：面包屑 知识库›kbName›文档（caret/信息弹卡/齿轮）+ 副标题 + ⚠ 无解析引擎警告行（由 parserEngines+parser_engine_rules 派生未支持扩展名，前往配置→）+ 全宽搜索 搜索文档名称... + 视图切换 + 全部标签/类型/状态/来源 + 日期范围筛选（列表参数与后端 knowledge.go:913-928 均已支持）+ 插画空态 知识为空，拖放上传（含大小限制文案）；RAW UUID eyebrow 移除；N007 上传下拉与字节进度遮罩保留。documents 套件 79/79（自 62 增）；live 截图 documents-page-slice/ 与 Vue 逐项一致（主代理独立目检 react-documents.png）。
- 在途：该代理将补写证据文档 2026-09-13-documents-page-shape.md。

## 2026-09-13 Round N+1（续12）— 第二波派发

- Wails S13 运行验证入库（4869c476）：打包+codesign+CDP 登录页冒烟；打包链路修复 esbuild 依赖声明（7db7e4d8）与 desktop vite 别名（bbc892ba）。
- 第二波派发（2）：AgentEditorModal 核心移植（分区轨/校验/载荷，分阶段交付）；Wails CDP 运行时交互取证（登录→KB→智能体→共享空间→设置→聊天 全链路截图+断言，仅取证不改代码）。

## 2026-09-13 Round N+1（续14）— KB 卡点击语义对齐

- 修复（5ce821fd）：KB 卡标题/卡体点击与 设置 动作语义反转（Wails CDP 取证发现 #2）——对齐 Vue KnowledgeBaseList：卡点击 handleCardClick（已初始化→文档页，未初始化→该 KB 设置页），设置动作 handleSettingsById→goSettings 无条件进设置（废止 Audit #4 的租户模型门，Vue 中该门仅在创建向导 initialSection）；顺带移除死代码 modelsReady（type==='llm' 永假——后端词表 KnowledgeQA/Embedding/Rerank/VLLM/ASR，取证发现 #3）。web 全量 564/564、tsc 0；:5181 live 复验卡点击按 Vue 规则导航。
- 观察登记：登录后引导聚光背板（wk-guide__backdrop）覆盖卡片属既定引导行为（首访状态），穿透点击仅在引导完成后可用。

## 2026-09-13 Round N+1（续13）— AgentEditorModal 集成与证据补录

- AgentEditorModal 核心移植集成确认：其源文件随 bfefe805 入库（代理自行提交，含 agent-editor-fallback.ts 570 行 5 语回退表、agent-editor.ts 分区/校验/载荷纯逻辑、AgentEditorModal.tsx 分区轨模态、agent-editor.css、双测试文件）。主代理独立复核：agents 35/35 + configuration 22/22；live 对等 cards 4=4、rail 4=4、品牌绿一致；R012 review-round-1 三问题（内置卡不渲染/主色/轨道标签）已修。证据文档由主代理代笔补录（3ca8ba42）：2026-09-13-agent-editor-modal.md。开放：DB 收藏（/user/favorites 接线）、agentEditor.* 键回填 packages/i18n、引导组件。
- Wails CDP 交互取证入库（77b6d6f5）：9/9 页面 PASS（登录→KB列表→文档→智能体→共享空间→设置模型→creatChat→会话切换→行操作菜单），pageerror=0、API≥400=0；fixtures 经应用自身 API 预置。登记缺口：用户菜单原始键名渲染（已修 e4e74e58）、KB 卡标题点击语义反转、modelsReady 门控 latent、昨天分组需 sqlite 直写（沙箱限制）。

## 2026-09-13 Round N+1（续4）— 第六、七切片集成

- 登记：chat 渲染层 chat-copy.ts 当前仅 zh-CN 单语（其余 4 语用户会看到中文）——列为 chat 5 语切片待办（从 Vue locale 提取 chat/input/messages/createChat 域全量键入 packages/i18n + 消费端切 formatMessage）。

## 2026-09-13 Round N+1（续4）— 第六、七切片集成

- upload progress transport 集成（499ec9a8）：api-client NativeMultipartFileRequest.onProgress + 浏览器 XHR sendMultipartFile（字节级、abort→AbortError、fetch 回退）+ platform/http observeUploadProgress 桥；SkillSettingsPanel skillUploading 百分比+进度条；documents UploadProgressMask 遮罩 + 批次均值百分比（KnowledgeBaseList.vue:1586-1595 语义）。独立复核：transport 5/5、http 14/14、panel 31/31、pipeline 30/30、dialog 20/20、api-client 43/43、documents 62/62。该共享层缺口关闭后 N031 移动端可直接消费 transport onProgress。开放：UploadFilesPanel 行内百分比、浏览器 E2E。
- R017 organizations 页重建集成（c2dc9f84）：英文调试壳 → Vue 共享空间 页解剖（header+加入/创建图标按钮、sticky section chips+计数、owners-first 卡片栅格+渐变头像/星点装饰/徽标行/所有者标签、创建/编辑设置弹窗、加入弹窗（邀请码预览/审批门控请求/搜索+joinById/已是成员态）、各过滤空态+移植插画、确认弹窗、toast；~1200 行 CSS 移植）。独立复核 19/19；live 截图 org-page-slice/。开放：PlatformShell 侧栏子筛选合并、RBAC 角色门控 role prop、设置弹窗为 3464 行 Vue 弹窗的功能性简化。
- 在途（6）：R012（目检三问题修复+最终截图）、Integrations/FAQ（最终取证）、settings 包装层（65/65 绿，语法错误已自愈）、shell 会话列表、R038 members。

## 2026-09-13 Round N+1（续2）— 第四切片集成

- NewUserGuide 欢迎引导集成（773e723b）：packages/views/src/guides（存储语义 weknora:new-user-guide-done:v1、7 步目录+5 语字节级文案、聚光几何 Vue 常量、组件 700ms 双检自动打开/键盘/定位重试/可选步自动跳过）+ PlatformShell 挂载与 data-guide 锚点 + shell 动作。独立复核：guides 14/14、jsdom 组件 6/6、platform 目录 75/75；live 复验（清 key→重载→步骤1/2→跳过持久化）通过并截图。开放：contextual guides 未移植、user-menu 重新打开入口、i18n 债 newUserGuide.* 26 键×5 语。

## 2026-09-13 Round N+1（续）— 首批三切片集成

- R033 skill 安装时间线 SSE 集成（f97a5126）：api-client sandbox skills API（install-events/transcript SSE 复用 chat stream parser + 现有 transport 链，guidance GET/steer POST snake_case ≤10000，后端 routes_infra.go:70-77 + internal/handler/sandbox_skill.go 协议核实）；domain 纯进度帧/时间线归并模块；SkillSettingsPanel 时间线 + guidance 轮询 + steer 去重 + 重装重试 + 进度环 + 抽屉拖拽宽度持久化 + markdown 文件预览。主代理独立复核：domain 7/7、api-client 7/7、panel 27/27、client 16/16。开放：upload progress% 需 transport 层 XHR onProgress（共享层缺口，同时解锁 N031）、pick-row 百分比扇出、浏览器/原生证据。
- R033 pick-row 百分比扇出（2026-09-14）：新增 AddSkillWizard 与 InstallSkillDialog 忙碌目标的逐行 install-events 订阅、AbortController 清理、Vue-compatible 0/5/100 fallback 与 18px 圆环/百分比布局；聚焦 SkillSettingsPanel 31/31、Web 681/681；证据 `evidence/vue-react-parity/2026-09-14-r033-pick-row-progress.md`。开放：浏览器同条件截图、Wails/native 证据；typecheck 仍被并行 `main.tsx`/`theme.ts` dirty changes 阻塞。
- N007 图谱抽取反馈（2026-09-14）：GraphSettings 的标签/文本/关系抽取成功与失败、文件/URL 添加与重复提示统一改为页面级顶部 transient toast，保留上传失败的可修复 inline 状态；focused upload-confirm/pipeline 50/50、Web 681/681、typecheck:web 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md`。开放：图谱启用后真实抽取端点与浏览器/Vue 同条件截图；当前部署图数据库关闭，标记 `blocked-env`。
- N010 标签管理（2026-09-14）：FAQ 标签筛选面板新增 Vue `KbTagManageDrawer` 等价入口，支持搜索、创建、重命名、删除确认、FAQ 数量、错误/加载态及删除后的活动筛选清理；api-client 补齐三条标签 CRUD 端点。FAQ/API focused 28/28、Web 682/682、typecheck:web 通过；证据 `evidence/vue-react-parity/2026-09-14-n010-tag-manage.md`。开放：真实后端浏览器与 Wails/native 证据。
- 设置当前浏览器审计（2026-09-14）：已认证 React `settings?section=skills` 实际渲染技能设置、左侧分组导航、空态与双操作；源码复核确认 R027 section 切换/URL 同步已实现，R031 调试头/raw dump/折叠开关仅为过期历史台账描述，当前源码无对应 DOM。证据 `evidence/vue-react-parity/2026-09-14-settings-current-browser-audit.md`。Vue 同账号 computed-style 对照与 Wails/native 仍开放。
- N031 移动 FAQ/Wiki 参考面（2026-09-14）：移除 React Native 页面中 Vue 不存在的权限/KB 诊断副文案，FAQ/Wiki 空态改用共享 i18n，空答案/摘要不再伪造 placeholder；mobile 146/146、typecheck 通过。证据 `evidence/vue-react-parity/2026-09-14-n031-mobile-faq-reference-copy.md`。仍开放 iOS/Android 真机启动与 Vue 同条件对照。
- N006 标签服务端分页（2026-09-14）：文档标签筛选改为 Vue 等价的 50 条分页、300ms 关键词防抖、服务端总数、加载更多与跨页已选项保留；新增 `tagsPage` 并保持 `tags()` 兼容。API/client + tag UI focused tests 通过，shared typecheck 通过；证据 `evidence/vue-react-parity/2026-09-14-n006-tag-pagination.md`。仍开放同条件浏览器截图与真实后端回归。
- N010 FAQ 卡片/编辑抽屉（2026-09-14）：FAQ 卡片改为 Vue 的可选中卡片、三段折叠内容、more 菜单、标签/状态 footer；编辑抽屉补齐 520px 结构、字段描述、列表增删、10/5 上限、必填顺序和 inline 错误位；FAQ focused 35/35。证据 `evidence/vue-react-parity/2026-09-14-n010-faq-card-editor.md`。仍开放检索测试抽屉、tag tooltip、真实后端与浏览器/平台证据。
- N007 upload graph 节集成（30f6008d）：UploadGraphSettings 全量 GraphSettings.vue 移植、systemInfo.graph_database_engine + KB indexing_strategy.graph_enabled 门控与导航回退、保存载荷 graph_enabled+extract_config（enabled 钳制）、URL 列表顺序上传、UploadSourceDropdown 文件/文件夹/URL 菜单 + URL 子对话校验、documents.css +290。独立复核：focused 45/45、documents 57/57。开放：extraction 端点 live e2e 需图库启用（当前部署关闭，门控已端到端验证）、错误提示 inline vs Vue toast、单节显示模型差异（导航顺序/门控一致）。
- 主代理直接修复：SandboxSettingsPanel 开关对齐 Vue（5f272a1f）——折叠 details 换 单向 switch + 警告 popconfirm（Vue:36-55 语义），ⓘ 提示气泡样式，样式独立 sandbox-settings.css；24/24。
- i18n 债登记：upload-pipeline.ts 本地字节级拷贝表（graphSettings.*/uploadConfirm.* 块）待迁 packages/i18n 共享包（N007 报告，不阻塞）。
- 其余代理（R012/Guide/R017/integrations/FAQ/chat/settings 包装层）继续并行中。

### Checkpoint 2026-09-13 — R038 member list header repair

- `f0340a3f` adds a Vue-aligned, always-mounted member-list header to
  `apps/web/src/settings/TenantMembersPanel.tsx`, including the server-backed
  member count badge and persistent search controls. The focused regression
  test first failed because the header/count markers were absent, then passed
  3/3 after the repair.
- `git diff --check` passed. `pnpm typecheck:web` remains blocked by the
  pre-existing `@weknora/domain/auth/onboarding` module-resolution error and
  an implicit-any diagnostic in `WorkspaceOnboardingPage.tsx`; these are
  outside the R038 two-file commit. No browser screenshot or full Vue visual
  comparison is claimed, so R038 remains `review`.
- R038 fix round 1 (`ea39080a`, report `bea2f635`) adds React-owned responsive
  member-header CSS, localized title/search labels, an accessible clear action,
  and jsdom interaction coverage for loading/search/clear state. Focused tests
  pass 4/4; Web regression is 285/286 because the pre-existing onboarding
  module-resolution test still fails. Independent scoped re-review is pending,
  and no visual/browser acceptance is claimed.

- **R038 follow-up build gate:** added the explicit
  `@weknora/domain/auth/onboarding` aliases to both Web TypeScript and Vite
  resolution. `pnpm typecheck:web`, `pnpm test:web` (289/289), and
  `pnpm build:web` now pass. `node scripts/check-react-boundaries.mjs` still
  fails on the pre-existing legacy-frontend reachability list and one shared
  host-global finding; this is recorded as an unresolved repository gate, not
  as page acceptance. Browser/Vue screenshot comparison and native evidence
  remain absent, so R038 stays `review`.

- **Mobile gate audit:** running `pnpm --filter @weknora/mobile typecheck` in
  this target worktree still fails because `apps/mobile/app/onboarding.tsx`
  imports the missing `apps/mobile/src/features/auth/OnboardingScreen.tsx`,
  and Expo route typing rejects `/onboarding` in the current route manifest.
  The existing onboarding component test also imports that missing screen.
  A delegated attempt landed on the repository default `main` worktree and
  made no target changes; its result is discarded. This remains an
  implementing/blocked-by-code gap and is not accepted.

- **N005 share dialog slice:** the React knowledge-base share dialog now has
  the Vue two-state form/shared-list interaction, back navigation, loading and
  empty states, permission labels, and synchronous duplicate-mutation
  protection. Focused DOM tests pass 2/2; `pnpm test:web` passes 290/290 and
  `pnpm typecheck:web` passes. No browser/Vue screenshot or native evidence
  was collected, so N005 remains `review`.

- **N005 review fix:** loading/empty mutual exclusion, viewer-only
  organization filtering, create payload/failure side effects, and unshare
  confirmation rejection/failure coverage were added. Focused DOM tests pass
  7/7; Web regression passes 295/295 and Web typecheck passes. No browser,
  Vue screenshot, real-backend, Wails, iOS, or Android evidence exists, so
  N005 remains `review`.

- **Mobile onboarding gate repair:** added the native
  `apps/mobile/src/features/auth/OnboardingScreen.tsx` implementation for
  policy loading/retry, invite-only and create-workspace actions, invitation
  empty state, and logout. The existing native-host onboarding tests now pass
  as part of mobile test 89/89; `pnpm --filter @weknora/mobile typecheck`
  passes after preserving the valid `/onboarding` runtime route through the
  generated-route typing boundary. Simulator/device screenshots and real
  backend onboarding flows are still required; this row remains review.

- **Mobile onboarding review follow-up:** invitation rows now expose
  accept/decline actions, remove accepted/declined entries after server
  confirmation, and logout navigates to the auth route. Mobile test is now
  90/90 and typecheck passes. The generated Expo route manifest still needs a
  durable `/onboarding` declaration instead of the current narrow cast, and
  simulator/device plus real-backend evidence remains outstanding.

- **Mobile onboarding review repair:** accepted/declined invitation actions
  now use the existing typed invitation APIs, successful acceptance refreshes
  workspace state, and logout navigates to the auth route. Expo redirects now
  use type-safe relative route strings rather than forced casts. Mobile test
  and typecheck both pass (90/90); native simulator/device and real-backend
  evidence remain open, so onboarding is still `review`.

- **Mobile test dependency repair:** replaced the temporary ambient jsdom
  declaration with a direct `@types/jsdom` dev dependency in the mobile
  package and lockfile. Mobile test remains 90/90 and typecheck passes; this
  removes the package-boundary masking identified by review. Device and real
  backend evidence remain outstanding.

- **Mobile onboarding error-state repair:** invitation loading failures now
  remain an explicit error rather than rendering the empty inbox; logout
  failures remain on the page with an alert. Native tests pass 91/91 and
  mobile typecheck passes. This is still review pending simulator/device and
  real-backend evidence.

- **Mobile data-source permission guard:** `DataSourcesScreen` now derives
  write capability from the active workspace role, hides Add/Edit/Sync/
  Pause/Resume/Delete for viewers, and fails closed before any write action;
  Logs and refresh remain available for read-only users. Mobile typecheck and
  full native-host tests pass (91/91). Connector-specific forms, resource
  selection, device evidence, and real backend 403 verification remain open.

- **Mobile data-source rule extraction:** the owner/admin mutation rule is now
  centralized in `canManageDataSources` and covered by role-matrix tests,
  while the screen consumes that helper. Focused tests pass 2/2 and mobile
  typecheck passes. Full connector/resource/form parity and native/backend
  evidence remain open.

- **Mobile data-source screen wiring:** added a native-host DOM harness for
  `DataSourcesScreen`, proving viewer controls are read-only while admin
  controls remain available against a mocked typed client. Focused tests pass
  2/2; the full mobile suite passes 92/92 and typecheck passes. Connector
  specific form parity, real 403/backend behavior, and iOS/Android evidence
  remain open.

## 2026-09-15 Round R359 — Knowledge-base connector credential fields

- Vue `DataSourceEditorDialog.vue` renders connector-specific credential fields;
  React previously exposed one generic credentials textarea for every connector.
- React now maps the Vue field contract for Feishu/Lark variants, Notion,
  Yuque, Tencent IMA, GitLab, and RSS settings. Values still serialize through
  the existing key/value payload boundary, preserving API and secret handling.
- Live React AX for Feishu shows App ID, password App Secret, and optional Base
  URL with Vue-derived labels/placeholders; the duplicate optional suffix was
  also removed.
- Web typecheck, full Web regression (963/963), and `git diff --check` passed.
  Vue's prerequisite guide, connection-test footer, resource/strategy steps,
  save/sync outcomes, and provider/native acceptance remain open.

- **Mobile data-source connector selection:** the native editor now renders
  server-returned connector types as selectable cards with name/description,
  falling back to a text field only when the server returns no types. This
  keeps the submitted type within the existing connector contract. Mobile
  test 92/92 and typecheck pass; connector-specific credentials/resources,
  backend 403, and device evidence remain open.

- **N005 share dialog race/localization repair:** the Web share dialog now uses
  the existing organization-share translations across title, form, list,
  permissions, feedback, and unshare confirmation. Loads are protected by a
  request generation so stale knowledge-base responses cannot overwrite the
  current dialog. Mutation callbacks and success notices now occur only after
  the post-mutation reload succeeds. Focused tests pass 10/10 and Web
  typecheck passes. Browser/Vue screenshot, real-backend, Wails, iOS, and
  Android evidence remain absent; N005 stays `review`.

- **Mobile data-source pre-save connection test:** the native data-source editor
  now exposes a guarded, non-persisting connector test for existing and new
  drafts. It reuses the typed `validateCredentials` transport, reports server
  failures in the page error state, disables conflicting save/test/cancel
  actions while pending, and clears stale success state when draft fields
  change. Focused wiring tests pass 3/3; the full mobile suite passes 92/92
  and mobile typecheck passes. Connector-specific fields/resources,
  localization, real backend, and iOS/Android evidence remain open.

- **Mobile bundle smoke check:** Expo Android export completed and emitted the
  bundle at `/tmp/weknora-mobile-export.jQ1yLt/_expo/static/js/android/entry-ee85a671e5d16ea91ea6b6f282f537c1.hbc`.
  This is bundle evidence only; it is not Android native build, launch, or
  interaction acceptance.

- **Mobile data-source resource selection:** editing an existing native data
  source now loads resources through the existing typed `resources(id)` API,
  renders selectable resource rows, preserves existing `resource_ids`, and
  writes the selected IDs into the update config. Focused wiring tests pass
  4/4; the full mobile suite remains 92/92 and typecheck passes. Vue's lazy
  hierarchical loading, Drive root-token flow, new-source temporary resource
  setup, localization, real backend, and native-device evidence remain open.

- **Mobile data-source hierarchical resource expansion:** resource rows now
  expose expand/collapse controls and request children lazily through the
  existing `resources(id, parentId)` API, while keeping selection rows
  independently actionable. Focused tests pass 8/8; the full mobile suite
  passes 94/94 and typecheck passes. Parent/descendant cover-set semantics,
  Drive root-token setup, new-source temporary browsing, and native/backend
  evidence remain open.

- **Mobile Drive resource root:** the native editor now recognizes bare
  Feishu/Lark folder tokens and `/drive/folder/<token>` URLs, normalizes the
  value, persists it as the selected root resource for an existing source, and
  loads the root resources through the existing API. Focused screen/policy
  tests pass 10/10; full mobile tests pass 94/94 and typecheck passes. New
  source temporary setup, connector-specific credentials, localized copy,
  backend error classification, and native-device evidence remain open.

- **Mobile new-Drive temporary browsing:** loading a Drive root from a new
  draft now creates a paused temporary source, browses resources through the
  existing API, and removes that temporary source when the draft is canceled;
  successful save promotes it through the normal update path. Focused screen
  and policy tests pass 12/12; full mobile tests pass 94/94 and typecheck
  passes. Cleanup on app termination, connector-specific forms, localization,
  backend error classification, and native-device evidence remain open.

- **Mobile GitLab connector form:** the native data-source editor now exposes
  GitLab project ID/ref/paths fields, supports adding/removing project rows,
  validates that at least one project ID is present, and serializes the
  connector's server-backed `settings.projects` shape on save. Focused screen
  and policy tests pass 13/13; full mobile tests pass 94/94 and typecheck
  passes. Exact localized copy, credential-field parity, live backend
  validation, and native-device evidence remain open.

- **Mobile RSS connector form:** the native editor now exposes feed URLs and
  optional `Name: Value` request headers, requires feed URLs before save or
  connection test, hydrates editable feed settings without exposing stored
  secrets, and serializes the existing RSS backend credential/settings shape.
  Focused screen and policy tests pass 14/14; full mobile tests pass 94/94
  and typecheck passes. Exact localized copy, other connector fields, live
  backend validation, and native-device evidence remain open.

- **Mobile connector credential fields:** the native editor now maps the
  existing backend connector contracts to dedicated write-only fields for
  Feishu/Lark and Drive, Notion, Yuque, Tencent IMA, and GitLab. Required
  fields are validated for new sources; existing sources validate by ID when
  no new secret is entered, while explicit changes use the non-persisting
  credential validation endpoint and then the dedicated credentials endpoint.
  RSS test validation includes `feed_urls` without persisting it as a secret.
  Connector changes clear unsaved secret values, while unknown connectors
  retain the legacy key/value fallback. Focused screen/policy tests pass
  20/20; the full mobile suite passes 95/95; mobile typecheck and diff check
  pass. Exact localized copy, Vue/React screenshots, real backend, Wails,
  iOS, and Android evidence remain open; N031 stays `implementing`.

- **N005 Web upload progress/highlight slice:** aligned the knowledge-base list
  with Vue-compatible upload lifecycle events. Tasks are aggregated by KB,
  successful and failed tasks remain visible for 10 seconds, refresh is
  debounced and cleaned up on unmount, and all-failed batches do not trigger a
  refresh. `highlightKbId` now clears from the URL after successful loading and
  jumps to the target page across all list scopes. Focused tests pass 4/4, Web
  regression passes 301/301, Web build and diff check pass, and independent
  review passes. Browser/Vue screenshot, real-backend, Wails, iOS, and Android
  evidence remain absent; N005 remains `implementing`.

- **N031 mobile KB list localization slice:** the native knowledge-base list now
  restores and persists a validated shared locale, injects it into the existing
  transport `Accept-Language` header, and uses shared translations for scope
  labels, navigation, empty/loading/error states, creation form copy, counts,
  and accessibility labels. New favorites/recents/load-failure/accessibility
  keys are present in all five repository-supported locales. Mobile tests pass
  96/96, the i18n knowledge-list suite passes 4/4, typecheck and diff check
  pass, and independent review passes. Remaining N031 detail/data-source/
  upload/graph parity and iOS/Android runtime evidence keep the row
  `implementing`.

- **N031 mobile data-source connector-boundary slice:** the picker now filters
  server connector metadata to the nine connector definitions explicitly
  available in the Vue editor (`feishu`, `lark`, both Drive variants, `notion`,
  `yuque`, `ima`, `rss`, and `gitlab`). Existing unknown/legacy source rows are
  still displayed read-only rather than being silently removed. Regression
  coverage proves all nine supported types survive and the unknown row remains
  visible: mobile tests 97/97, DataSources DOM harness 16/16, typecheck and
  diff check pass; independent review passes. N031 remains `implementing` for
  the remaining data-source lifecycle, detail/graph parity and native evidence.

- **N031 mobile resource-tree selection slice:** resource rows now render the
  existing three-state selection semantics from the shared helper: checked
  (`✓`), indeterminate (`−`), and unchecked. Partial selections are highlighted
  without changing the minimal-cover payload or resource API. The helper suite
  passes 7/7 and the DataSources DOM harness passes 16/16; mobile typecheck and
  diff check pass, and independent review passes. N031 remains `implementing`
  for lifecycle, localization, remaining resource behavior and native evidence.

- **N031 mobile sync-status slice:** the API model now carries the server's
  `latest_sync_log` status and counters; the mobile inventory renders the latest
  result and performs a cleanup-safe 3-second silent refresh while a sync is
  running. Mobile full 99/99, helper 8/8, DOM harness 17/17, shared typecheck,
  focused shared tests 339/339 and diff check pass. N031 remains `implementing`.

- **N031 mobile sync-log slice:** the native log panel now requests 50-row pages
  with guarded offsets, appends additional pages, shows summary counters, and
  expands a row to expose item counters. DataSources DOM harness 19/19,
  typecheck and diff check pass. N031 remains `implementing` because date/status
  localization, full Vue log presentation, visual/runtime and native evidence are
  still open.

- **N031 mobile sync-log i18n slice:** shared i18n now supplies the Vue sync
  history, summary, status, detail, empty-state, close, and refresh labels in all
  five repository locales; the native log panel consumes the active mobile locale.
  i18n suite 1/1, DataSources DOM 20/20, mobile full 99/99, both typechecks and
  diff check pass.
  Main data-source/editor copy and runtime evidence remain open.


## 2026-09-13 Round N（并行五切片 + MCP live 取证 + 共享层补齐）

- 环境：docker dev 基础设施复用；make dev-app :8080（本轮以 SSRF_WHITELIST_EXTRA=mcp.parity-invalid.example 重启以创建失败态取证服务）；Vue :5180 / React :5181 双端同库同账号（parity-test@local.dev，tenant 10000）。
- 并行切片全部落地（各自 focused 测试绿，主代理独立复核后集成）：
  1. R031 Sandbox：wizard/模板目录/深度检查/精确校验/本地化（24/24）；主代理补 SettingsPage→dockerBackendEnabled 能力接线。
  2. R033 Skill：目录卡片/两步向导/安装与管理抽屉/文件浏览器/轮询（23/23）；主代理补通 web zip 上传链路（api-client register 接受 Blob→objectURL 桥 + browser transport sendMultipartFile，后端字段 file）。
  3. R027 Model：签名 rerank/WeKnoraCloud 门控/维度覆盖/thinking-control/校验/载荷/列表/连接测试/Ollama/调试面板（36/36）。
  4. N007 Upload：FolderPickerMenu 目的地选择/文件面板/分节导航/完整 reparse 配置/五语文案（45/45）。
  5. N031 Mobile：dataSource i18n 149 键×5 语、上传遮罩端口、持久化 pins/recents、页面本地化（75/75 + i18n 5/5，typecheck 干净）。
- 共享层：packages/i18n settings 包新增 584 键×5 locale（model./modelSettings./settings.sandbox./settings.skills./settings.weknoraCloud./uploadConfirm./knowledgeStages. 前缀全量补齐 + common.remove 等，字节级来自 Vue locales；脚本 scripts/parity/merge-settings-keys.mjs 可重复执行）；i18n 33/33。
- MCP live 取证与缺陷修复（R024/R043-R046/N016）：同后端双端浏览器证据（列表/新增/编辑/两步流/失败态截图+DOM 文本，evidence/2026-09-13-mcp-live-browser-evidence.md）；发现并修复 React 连接步保存必 400（payload 携带空 usage_instructions；Vue buildPayload 从不发送该字段）——buildMcpConnectionPayload 纯函数 + 回归测试 + 浏览器复验 PUT 200。同时移除 Vue 没有的“描述”字段；登记 stdio 选项、测试连接可达性、0/16000 计数器等待决项。
- 基线：web 399/399、shared 344/344、mobile 75/75、i18n 33/33、typecheck:shared/web/mobile 全绿、build:web 成功。
- 后端事实登记：PUT /api/v1/mcp-services 对 usage_instructions 强制 1..16000（空串 400；省略字段保持原值）；SSRF 校验默认拒绝不可解析域名（白名单可解）。
- 下一步：R045 测试连接可达性裁决、MCP stdio 选项裁决、skill 安装时间线 SSE（api-client 缺口）、upload 图谱节、model combobox、各 implementing 行的浏览器/Wails/原生证据推进，以及 74 个 review 行的验收证据批量收集。

- **并行提示（2026-09-13 17:45）**：另一代理正在 McpSettingsPanel.tsx 上进行 stdio→SSE 收敛与 McpMetadataPanel 结构对齐的未提交 WIP（正是本矩阵登记的待决项方向）；其进行中状态会使 McpSettingsPanel.test.tsx 暂时失败。本行以上提交（…781c81d8）均为全

- **MCP 抽屉结构对齐落地（2026-09-13 round 2，McpSettingsPanel/McpToolsDirectory）**：三项待决全部向 Vue 基准收敛并落地——(1) stdio：编辑器仅保留 SSE/HTTP-Streamable，stdio 服务加载时按 McpServiceDialog.vue:855 强制转 sse，列表标签改 `Stdio`；(2) 测试连接：Vue 基准中 McpTestResultBody.vue/testMCPService 为孤儿代码（无导入/无全局注册），React 抽屉不再挂载测试入口，保留孤儿组件+单测（R045 → review）；(3) 第 2 步：usage textarea 可编辑 + 0/16000 计数 + AI 生成（仅填充字段，保存才落库）+ generateHint 文案，保存以 toolsSynced 门控并复用 syncRequired/instructionsRequired 警示。同轮完成：步骤 0 结构对齐（基本信息含启用开关+hint、连接配置、认证配置含授权状态块与"先保存后授权"、高级配置）、页脚取消→确认顺序+上一步居左、可点击步骤条、元数据面板仅第 2 步挂载、创建密钥内联（buildPayload(true) 对齐）。工具目录按共享五语 Vue 文案本地化（merge-settings-keys.mjs 移交 mcpMetadata.description/parameters/fullSchema/fetch/refresh）。证据：jsdom 交互测试 4 例（结构/stdio 强转/同步门控/step2 保存），web 404/404、typecheck:web、shared 344/344 全绿。浏览器复验（:5181/:8080，mcp-round2-verify.cjs + 截图 mcp-react-round2-*）通过：结构顺序/传输选项/页脚顺序/无测试连接/计数器/同步门控全部符合 Vue 基准；同轮修复 toggle 标签被 fieldset grid 压缩成竖排的样式缺陷。遗留：shadcn/ui 视觉扫、Wails 证据、可同步服务下的已同步态取证；编辑态 CredentialResource 卡片差异保持登记。绿基线；集成其 WIP 前先等其稳定并复跑 focused 测试。

- **Depends on:** S00 and all row-owning slices for rows being accepted.
- **Rows:** any rows proposed for `accepted`; never all rows by default.
- **Files owned:** `docs/migrations/react/vue-react-parity-matrix.md`, `docs/migrations/react/vue-react-parity-progress.md`, `docs/migrations/react/evidence/vue-react-parity/README.md`, `docs/migrations/react/evidence/vue-react-parity/screenshot-matrix.md`, and the row-specific `docs/migrations/react/evidence/vue-react-parity/<date>-<row-group>.md` evidence file only.
- **Boundary:** no product code changes in acceptance-only commits; failed evidence goes back to owning slice.
- **Failing-test target:** row-specific tests from owning slices plus full gates only after targeted success.
- **Targeted command:** minimum `pnpm run test:shared && pnpm run test:web && pnpm run typecheck:shared && pnpm run typecheck:web && pnpm run build:web`; add desktop/mobile/embed commands when accepting platform rows.
- **Live scenario:** Vue :5180 and React :5181 same backend :8080, 1440x900 zh-CN plus en-US spot checks; Wails/iOS/Android/embed where applicable.
- **Evidence required:** screenshot matrix and state checklist showing normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure for each row.
- **Review gate:** a row can move to `accepted` only when all applicable platform evidence and unresolved backend decisions are closed or explicitly out-of-scope by user decision.
- **Commit boundary:** one acceptance-docs commit per row group.

## 2026-09-13 Round N+5（Round 4）— N003 footer 补全 + R002 toast 修复 + 门禁

- N003 收尾（081a9c15，协调者直接补全）：commandPalette.hotkey.select/enter/cmdNumber/cmdEnter 4 键×5 语 byte-exact 入 generated/commandPalette.ts（Vue :142-150 footer 5 提示：↑↓/↵/⌘1-9/⌘↵/Esc）；GlobalCommandPalette 渲染 footer + command-palette.css 按值移植（:794-824，tdesign light token）；jsdom 字节断言 5/5；live Ctrl+K 打开面板 footer 文案逐字一致，截图入库 n003-palette-footer/。
- 登录 toast parity 修复集成（0f6f4d12）：R002 错误态呈现分歧（Vue 顶部 MessagePlugin vs React 内嵌横幅）TDD 修复 + live 复拍一致；R002 行补 qualification（026051c9）。
- R031 openSession 集成（89918e5a）：清单行点击导航 /platform/chat/:id，embed 安全。
- ~~工作树中两处未提交 Vue 锚定精修待 R009 报告一并裁决~~ 已落地，见下节追加：command-palette.css kbd 色值 token 化（b81d6ac6）、SettingsPage members 包裹标题去重（6282000a），另加分页按钮 TDesign 化（e4284ff5）。
- 全门禁：shared 393/393、web 623 中 615 绿（7 红全部是 R009 anatomy 切片在途 TDD 红测试——.kb-list-rail/.kb-list-grid/.kb-list-warning 等未实现断言，属预期中间态）、mobile 118/118、build ✓。
- 在途（1）：R009 kb-list 整页 anatomy。

## 2026-09-14 Round N+5 追加 — 协调者三处 token/结构收敛落地 + live 计算样式取证

- 上节 1067 行所述两处未提交精修已由协调者落地并提交，另补一处分页按钮收敛（三提交均为 TDD 外的 live 取证驱动，逐处计算样式双端一致）：
  - members 包裹标题去重（6282000a）：members 现与 general/models 同列 panel-owned-header，wrapper 不再渲染；live :5181（guide 键种子）全区仅 1 个 h2、无 identity.tenants 文本、RBAC 链接 ×1；截图 react-settings-members-single-heading.png（单标题 + 绿色当前页）。
  - command-palette.css 徽章 kbd 色值 token 化（b81d6ac6）：四处改为 Vue light 主题值（placeholder rgba(0,0,0,.4)/secondarycontainer #f3f3f3/component-stroke #e7e7e7/secondary rgba(0,0,0,.6)）；live 计算样式与 Vue 逐值一致。
  - R038 分页按钮 TDesign 化（e4284ff5）：React 蓝 #2e6de6 描边方块 → Vue t-pagination 默认（无边框、3px 圆角、24px、当前页品牌绿 #07c05f 白字、hover 品牌绿文字、禁用箭头 rgba(0,0,0,.26)）；数值取自 :5180 live 探针（.parity-tools/vue-pager-probe.cjs，gitignored 工具），修复后 :5181 逐值一致。
- 验证：TenantMembersPanel+surface 28/28、platform 面板 34/34；全门禁（footer 081a9c15 落地前集成树）：shared 392/392、web 616/616、mobile 118/118、desktop 2/2、embed 7/7、typecheck shared/web 0、build:web ✓3.42s。

## 2026-09-14 Round N+5 追加 — R009 知识库列表 anatomy slice

- Vue→React 对照完成：列表横向筛选替换为竖直 scope rail；卡片改为紧凑响应式栅格；分组标题支持折叠；补齐 hover 收藏/更多菜单、类型/能力徽章、未初始化警告、骨架和 Vue 空态插画；失败请求回退空态且不泄露原始错误。
- TDD 先行钉死 7 项：rail、grid/card、section collapse、warning、error fallback、Vue exact card-menu actions、favorite persistence；Focused 7/7，Web 回归 623/623，`typecheck:web`、`build:web`、`git diff --check` 均通过。
- 移除 React 菜单中 Vue 不存在的“编辑/分享”入口；既有 editor/share 组件保留给其 owning flow。截图证据与边界见 `evidence/vue-react-parity/2026-09-14-r009-kb-list-anatomy.md`。
- 状态仍为 `review`：当前轮未重新取得浏览器 computed-style/真实后端/Wails/iOS/Android 证据；既有 paired 1440x900 artifacts 仅作对照材料，不替代交互与平台验收。


## 2026-09-13 Round N+4（终）— 四切片集成 + 登录 toast parity + 时间脆弱测试修复

## 2026-09-14 Round N+5 协调条目 — 第五批四切片派发 + 平台门禁确认（主协调 Agent）

- 第五批并行派发（4，文件归属互斥）：R013 integrations 深水区（IM 向导/embed 抽屉；独占 packages/views/src/integrations + apps/web/src/integrations）、N006 documents 批量/标签（独占 apps/web/src/documents）、R017 RBAC + 壳层子筛选（独占 organizations + PlatformShell 相关块）、FAQ 收尾（独占 apps/web/src/faq）。
- R009 anatomy 预审：App.tsx +310/-192、knowledge-list.css 637 行、icons/empty-svg 独立文件、7 项 TDD 红测试（rail/栅格/警告横幅/空态 SVG/错误不外露 JSON/更多菜单/收藏持久化）——实现结构良好，转绿后集成。
- 非 Web 门禁确认：mobile 118/118、embed 7/7、desktop 2/2、shared 393/393、typecheck mobile/embed/desktop 0。
- 六语缺口维持登记（24f773bd），待用户答复。


- R016 模型 chip 集成（28ad0de4）：新建对话 chip 显示真实模型名+上下文规格（对齐 Input-field.vue:2784-2804）；model-chip.ts 9 单测 + chat-page 20/20；协调者批准并落地 page.tsx modelContext/modelContextIsDefault 透传；顺带恢复被并发编辑误删的 loadingMessages state。
- N003 面板作用域 ⌘1-9 集成（43fdd880）：⌘数字仅面板打开时生效（Vue GlobalCommandPalette.vue:508-520 同构），⌘N 徽章进 recent/命令行，全局 ⌘1 行为由测试钉死为不存在；platform 115/115。遗留：footer 提示条需 commandPalette.hotkey.* 4 键×5 语（待裁决）。
- settings chrome 铺开集成（97667809）：select tdesign 化提升为抽屉通用规则（11 面板覆盖清单见证据）、rail hover/active/底色 token 对齐（蓝色残留清除）；live 计算样式双端一致。
- R031 openSession 集成（89918e5a）：清单行点击 → /platform/chat/:id（Vue :171/341-344 语义，embed 安全 inert 契约）。
- 协调者直接修复：members 页头 apiDomain 泄漏（9ca79d34，Vue 无描述行 → 空）；members 分页器尺寸选择器全宽（b5aaec4d，Vue ~90px 内容宽）；登录/注册结果 toast 化（0f6f4d12，Vue MessagePlugin parity，live 复拍）；shell 会话 fixture 时间脆弱修复（710d6a51，25h-ago 跨桶 → 日历昨日 12:00 锚定）。
- 全门禁（集成树）：shared 392/392、web 616/616、mobile 118/118、desktop 2/2、embed 7/7、typecheck×3 0、build ✓。
- 六语缺口已正式登记（24f773bd），待用户答复第六语名称与基准；在答复前一切语言覆盖表述为五语。
- 在途（1）：R009 kb-list 整页 anatomy（四分歧：卡片栅格/筛选 anatomy/警告横幅/错误态语义，大切片）。


## 2026-09-13 Round N+4（续）— 状态覆盖取证发现 R009 整页分歧 + 第五切片派发

- 状态覆盖 sweep（state-coverage-20260913，10 对截图）：登录失败呈现分歧（Vue 顶部 toast vs React 内嵌横幅）已修（0f6f4d12，jsdom 钉死 + live 复拍）；R002 行补 qualification。
- R009 整页 anatomy 分歧确认（非回归——round3 同日同服务器截图即如此，R009 行既有的 re-shoot 预警成立）：卡片栅格（Vue 3 列紧凑卡+hover 操作 vs React 全宽堆叠+常显按钮）、筛选 anatomy（Vue 竖直图标轨+我创建的下拉 vs React chips+select+输入框）、警告横幅样式、错误态语义（Vue 回退空态插画+CTA vs React 外露原始 JSON+重试）。已派发 R009 kb-list anatomy 专切片（独占 App.tsx + knowledge-bases/**）。
- R031 openSession 导航集成（89918e5a）：Vue :171/341-344 语义，onOpenSession prop + embed 安全 inert 契约，28/28。
- 在途（4）：R016 模型 chip、N003 面板作用域 ⌘1-9、settings chrome 铺开、R009 anatomy。


## 2026-09-13 Round N+4 — 第四批派发 + 六语缺口正式登记

- **六语缺口登记（目标 §二.7）**：目标要求六种语言；仓库语言注册表 packages/i18n/src/index.ts supportedLocales 仅五种（zh-CN/en-US/ja-JP/ko-KR/ru-RU），Vue frontend/src/i18n 同为五语。第六种语言的名称与基准文件不存在，按目标要求记录缺口并询问用户；在用户答复前，所有语言覆盖结论一律以"五种仓库语言"表述，不写成六语完成，也不猜测第六语。
- 第四批并行派发（4，文件归属互斥）：R016 模型 chip（composer.tsx + ChatRoutePage.tsx + chat.css chip 块）、N003 面板作用域 ⌘1-9（GlobalCommandPalette*）、settings chrome 铺开 + rail hover（settings CSS 家族）、R031 openSession 导航（SettingsPage.tsx + SandboxSettingsPanel.tsx）。
- live 栈仍在运行：backend :8080、Vue :5180、React :5181。


## 2026-09-13 Round N+3（终）— 双切片集成收尾 + 全门禁绿

- shell-sessions-header 集成（25ad0fed，切片实施 + 协调者严格 parity 裁决调整）：保留 可见的 我的对话 标题（真 parity）；否决其 ⌘1 徽章 + 全局键绑定（Vue 中该指纹属命令面板作用域——多余行为按目标剔除），测试 (b)/(d) 改为钉死"无徽章/无全局导航"；顺带修复侧栏选中态颜色分歧（React 蓝 → Vue .menu_item_active #f3f3f3+#07c05f）。platform 101/101。
- settings-visual-polish 集成（7a58f68b）：标题节奏（中和旧 margin 泄漏 + 三处值对齐）、分段控件选中态实心品牌绿、select 外观 tdesign 化（仅 .general-settings 作用域）；live 闭环抽屉内像素差 7.07%（残差=字体栅格化）；新增交互语义测试 2/2。
- 全门禁（集成树）：shared 392/392、web 586/586、mobile 118/118、desktop 2/2、embed 7/7、typecheck shared/web/mobile 0、build:web ✓ 3.77s。
- 遗留登记：logo 行搜索入口（⌘K tip）、面板作用域 ⌘1-9（N003 既定 deferred）、select 外观向其余面板铺开、抽屉 rail hover 色阶、creatChat 居中簇 ~40px 残差、R016 模型 chip、openSession 行导航 prop。
- 本会话累计：协调者直接修复 4 项 + 集成 6 个子代理切片；账本/矩阵每步同步。


## 2026-09-13 Round N+3（续2）— creatChat 居中修复 + contextualGuide 回填 + R031 集成 + 证据守护

- creatChat 空态布局真实分歧修复（42a67935，协调者直接实施）：5 语扫描目检发现 React 把欢迎语钉顶、composer 钉底，Vue 是 .dialogue-wrap 双轴居中簇——像素排序曾对其打 3.96% 低分（稀疏内容下百分比失真），文本指纹+并排目检才暴露。TDD 红（无 --empty 类+渲染滚动容器）→绿 16/16；live 复拍与 Vue 构图一致（簇中心尚差 ~40px，列 polish）。
- contextualGuide i18n 回填集成（d33cc559，子代理实施+独立复核）：89 键×5 语入 packages/i18n generated + 三层字节级钉死测试（shared↔Vue、merged、local↔shared），审计 0 漂移；shared 392/392。遗留：renderContextualGuideMessage 共享优先分层（文件头+证据已记）。
- R031 会话标题集成（04296bee，子代理实施+独立复核）：api-client sessions.get + 面板并行标题加载/未命名回退/无 raw id 闪烁；shared 392/392、面板 26/26、sessions 7/7。pick-row percent fan-out 移交 R033（精确交接在证据文档）；R031 新增开放项：openSession 行导航需集成层 prop、inventory 内联 Card vs SettingDrawer 另案。
- 证据守护：并行 agent 的 891673ea（标题为删旧截图）误删了 round-3 双端批扫证据且卷入了 R031 在途文件——已恢复 40 文件（13899d48）并在 R031 集成提交中清理其临时探针文件。登记约定：证据删除类提交必须逐路径核对被引用证据。
- 5 语扫描入库（539c4c4a）：.parity-tools/locale-sweep.cjs + 30 张双端截图 + 证据文档；语言覆盖证据从"仅 zh"推进到 5 语×3 路由×双端。
- 在途（2）：shell-sessions-header（PlatformShell/shell.css，tsc 尚有 2 错误收敛中）、settings-visual-polish（GeneralPreferencesPanel/styles.css .wks-*）。


## 2026-09-13 Round N+3（续）— 移动端运行器修复集成 + 指纹取证

- 移动端测试运行器修复集成（55fdb0d6，子代理实施 + 主代理独立复核 118/118 + typecheck 0）：根因为裸 import esbuild 未声明 + npx tsx 的 NODE_PATH 意外泄漏；改走 createRequire(apps/web) 主机解析 + tsconfig paths（顺带修复既有 typecheck:mobile 红）。遗留（协调者所有）：knowledge/*.test.tsx 同款裸 import（现 glob 不执行 .tsx）、esbuild/jsdom 声明为 mobile devDeps 需根 lockfile 集成。
- round3 批扫文本指纹比对（vue-only/react-only 前 5 项）发现外壳会话区差异：Vue 有可见 我的对话 标题 + 新建对话⌘1 提示；React 仅 aria-label。派发 shell-sessions-header 切片（menu.vue 侧栏解剖对齐）。会话行菜单 置顶/修改标题/清空消息/删除记录 在 react-only 中出现属 DOM 常驻 vs Vue 按需渲染的指纹噪声，非缺陷（删除记录 已是 Vue 权威文案）。
- 派发（3，与 R031 并行）：shell-sessions-header（session-sidebar.tsx + PlatformShell.tsx）、settings-visual-polish（GeneralPreferencesPanel + styles.css .wks-* 块：标题节奏 10px/分段控件选中态/select 外观）、contextualguide-i18n-backfill（packages/i18n generated + views guides 表）。


## 2026-09-13 Round N+3 — 主题默认对齐 + live 批扫 round3 + R007 收尾集成

- live 环境重建：backend :8080（make dev-app）、Vue :5180、React :5181（需 VITE_API_BASE_URL=http://localhost:8080，无 dev proxy，app 直连后端；无此环境变量时同源 /api 全 404——已登记为启动事实）。parity-test@local.dev 经 API 与浏览器双侧验证有效（tenant 10000）。
- 批扫 round3：accept-batch.cjs 增加双端 guide-done 键种子（weknora:new-user-guide-done:v1 + 7 个 contextual 键，两端同键方案），消除首访引导浮层污染；20 路由 ×双端 40/40 截图入 screenshots/accept-20260913-round3/。
- 像素排序结论：kb/agents/org/chat/faq/creatChat/integrations(除 settings) 3.4–6.6%；settings 系 + integrations ~38–42%。热图归因：settings 差异主体来自背板模糊放大底页内容噪声（两端 overlay CSS 逐值一致 rgba(0,0,0,.5)+blur(4px)，React styles.css:560 = Vue Settings.vue:652-659），非可行动缺陷；可行动差异登记为 settings 视觉切片待办：(a) 抽屉内标题纵向节奏 ~10px；(b) 字体大小分段控件选中态（Vue 实心绿底白字 vs React 浅绿底绿字）；(c) 原生 select vs TDesign select 外观（已知项）。
- 主题默认修复（8493557e）：live 对比发现 settings-general 主题模式 Vue=浅色 vs React=跟随系统；TDD 修复 packages/domain local-preferences 缺省/非法值回退 light（Vue useTheme.ts:14-15 权威）+ GeneralPreferencesPanel catch 回退；domain 4/4、settings 面板 149/149；修复后 live 双端均显示 浅色。
- R007 收尾集成（80db6754，子代理实施 + 主代理独立复核）：用户菜单重新打开引导入口（Vue UserMenu.vue:45-50，文案权威为 新手引导 而非简报猜测的 重新查看引导——子代理纠正简报错误）；newUserGuide i18n 块 20 键×5 语字节级钉死（发现并修复 ru-RU steps.agents.desc 漂移）；views guideMessage 共享包优先分层。独立复核：shared 387/387、platform 目录 97/97、i18n+layering 8/8、typecheck 0。
- 在途（2）：移动端测试运行器修复（node --import tsx --test 下 onboarding-component MODULE_NOT_FOUND）、R031 会话标题 api-client 方法。

## 2026-09-13 Round N+2 — chat 5 语切片集成核验 + 会话行删除标签对齐

- 在途工作核实：并行 agent 将 chat 5 语文案切片与上下文引导系统提交为 c25089a8（chat-copy.ts 5 语表 + packages/i18n/src/generated/chat.ts 生成层 + ContextualGuide/Host/contextual-guides.ts 全套 + tests + Vue 基准截图）。主代理独立复核：shared 382/382、web 574/574、desktop 2/2、embed 7/7、typecheck shared/web 0、build:web ✓ 3.27s；contextual guides 16/16 + web jsdom 10/10 + i18n/chat-copy 12/12。
- 集成修复（5c7e0c6b）：(1) 生成层 chat.ts zh-CN chatHeader.deleteSession 字节漂移（删除会话→删除对话，zh-CN.ts:6937 权威）；(2) TDD 对齐 Vue menu.vue 会话行菜单删除项 = upload.deleteRecord（删除记录，5 语字节级）——chat-copy 新增 deleteRecord 键、session-sidebar 行菜单改用、shell 测试先行改断言红→绿；(3) Node 26 navigator.language=en-US 使共享 5 语解析偏离 zh 基准，chat-page/artifact-preview/shell-list 等测试显式 pin zh-CN locale/copy；views index 导出 chat-copy 助手；web views-stub 补 openContextualGuide no-op。
- 已知问题（移交移动端切片）：apps/mobile onboarding-component.test.ts 在 `node --import tsx --test` 下 MODULE_NOT_FOUND 失败（pass 0/fail 1），`npx tsx --test` 下 4/4 通过；已在 HEAD（不含本轮修改）复现，判定为在途/运行器兼容问题，非本轮引入。恢复命令：`cd apps/mobile && node --import tsx --test src/features/auth/onboarding-component.test.ts`。
- 矩阵更新：R007 上下文引导 LANDED；剩余：user-menu 重新打开入口、引导 i18n 键回填 packages/i18n、各引导 live 复验。
- 下一步：R007 收尾切片、R031 会话标题 api-client、N005 浏览器/原生证据、74 个 review 行验收证据、移动端 onboarding 测试运行器修复。

## 2026-09-14 Round N+6 协调条目 — R009 转绿 + 全量门禁 + 在途清单

- R009 anatomy 全部 7 测试转绿（单文件）；全量 test:web 623 中 622 绿——唯一失败是 kb-list-anatomy.test.tsx 在全量并行负载下的文件级超时（44.7s，单跑 7/7 秒级通过），集成时需处理（疑似 jsdom 重 DOM + 并行负载的 settle 膨胀）。
- 门禁快照：shared 393/393、web 622/623（唯一红=R009 在途文件超时）、mobile 118/118、build ✓ 7.79s。
- 上传链路核验：新 anatomy 保留 upsert/patch/summarize 与 highlight URL 清理；findUploadTargetPage 移除属架构性正确（merged cards 单列表使跨 scope 翻页跳转不再需要）。
- 在途（5）：R009 anatomy（收尾中）、R013 深水区、N006 batch/tag、R017 RBAC+壳层、FAQ 收尾。
- 并发协调者协同：账本/矩阵双轨更新已互认（其 R038/N003 条目与我的派发条目并存）；继续精确路径 add，避免 git add -A。


## 2026-09-14 Round N+6 协调条目 — 深链接扫描 16/16 + R009 错误态契约 live 验证

- 深链接/刷新/前进后退 sweep（402fe1c6）：8 路由×双端×{直达、原地刷新、后退、前进} 全部稳定，无漂移无意外重定向；证据 deeplink-reload-20260914/。登记 a11y 层级小项：React 平台路由 DOM 序首标题为壳层会话区 h2（视觉无碍）。
- R009 错误态契约 live 验证：强制 500 下 raw JSON 零外露、.kb-list-empty 空态呈现（截图入其证据目录）——与 Vue "列表失败回退空态" 语义一致。
- 五切片仍在途（R009 收尾 + 四个第五批切片）；kb-list-anatomy.test.tsx 在全量并行下文件级超时问题待其收口时处理。

## 2026-09-14 Round N+6（终）— 五切片全部收口 + 全门禁绿

- 全部在途切片完成并集成：R009 anatomy（80ab0a0a 自提交，7/7 + live 双端截图闭环）、N006 batch/tag（cbcc64a3，98/98）、R017 RBAC+壳层子筛选（92115a7b，26/26+119/119）、FAQ 收尾（e95f9a1c，19/19）、R013 IM 向导（page.tsx 终态由并发协调者入库 3bbc2b a6；42/42+6/6+typecheck 0）。
- FAQ A1 落地（e94a45bb）：api-client faq.importProgress（后端 faq.go:506 / types.FAQImportProgress faq.go:425），6/6；A2 轮询接线为下一切片（视图已就绪）。
- 协调者直接修复累积：登录 toast parity、members apiDomain 泄漏、分页器宽度/按钮 TDesign 化（部分与并发协调者协同完成）、fixture 时间脆弱修复。
- 最终全门禁：shared 419/419、web 670/670、mobile 118/118、embed 7/7、desktop 2/2、typecheck×3 0、build:web ✓ 3.42s。
- 遗留队列（下轮）：FAQ A2 轮询接线（api-client 已就绪）、A3 faqManager.import.* 目录回填、A4 Excel 解析依赖决策、B1-B7 精确交接、R013 embed 抽屉 + API playground SSE、R009 图标轨拖拽展开/org 轨条目、acceptance 证据批扫（Wails/iOS/Android）、mobile devDeps lockfile 收口、六语待用户答复。

## 2026-09-14 Round N+7 协调条目 — A2 双轨收敛 + 门禁全绿快照

- FAQ A2 轮询接线双轨收敛：本方实现（poll effect + collapse + strip pass-through，jsdom faq-import-poll 钉死 processing→completed→collapse 全链）与并发协调者的 4651ea79（同名契约）汇合，最终以 4651ea79 入库（含 api-client index 的 FAQImportProgress 导出）；faq 套件 20/20。
- 门禁快照（603de8c6→ 后）：shared 419/419、web 672/672、mobile 118/118、build ✓ 3.42s、typecheck 0。
- 剩余队列不变：A3 faqManager.import.* 目录回填、A4 Excel 解析依赖决策、FAQ B1-B7、R013 embed 抽屉 + API playground SSE、R009 图标轨拖拽展开/org 轨、acceptance 证据批扫（Wails/iOS/Android）、mobile devDeps lockfile、六语待用户答复。

## 2026-09-14 Round N+7 协调条目 — A4 裁决落地派发 + rail 展开切片

- A4 裁决：采用 Vue 同源 vendored xlsx（frontend/packages/xlsx-0.20.2.tgz，本地工件无外部 registry 获取）——React 侧拷贝至 apps/web/vendor/ 并以 file: 协议声明，parseExcelFile 按 Vue :1999-2030 列语义逐字移植。派发 A4 Excel 导入切片（独占 apps/web/package.json deps/vendor/faq import-export）。
- R009 图标轨拖拽展开切片派发（Vue ListSpaceSidebar collapsed-strip ↔ expanded-panel 双态语义；独占 App.tsx rail 部分 + knowledge-list.css）。
- live 栈确认仍在运行（backend :8080 / Vue :5180 / React :5181）。
- 并发协调者 R043（MCP server docs popup）WIP 在树，本方避开其文件。

## 2026-09-14 Round N+7 协调条目 — 门禁快照与在途监理

- 门禁快照（含 peer 的 MCP popup 新测试 7692d777 与其在途 WIP）：shared 420/420、web 674/674、mobile 146/146、embed 7/7、desktop 2/2 全绿。
- 在途（2）：A4 Excel 导入（vendored xlsx 方案）、R009 图标轨拖拽展开——两者测试域尚未见落盘，监理中。
- kb-list-anatomy 全量套件超时问题在最新快照中未复现（674/674），继续观察。

## 2026-09-14 Round N+8 协调条目 — 主题应用功能修复集成 + 暗色取证

- 主题应用功能修复集成（dd537b63）：live 暗色扫描发现 React 全局主题不生效（weknora:theme-changed 零监听、启动不应用 theme-mode；Vue useTheme.initTheme 全局应用）——domain theme.ts 纯模块（effectiveTheme/applyThemeToDocument，matchMedia 注入）2 测 + apps/web/src/theme.ts initTheme 启动/事件/OS 三通道 + main.tsx 启动调用 + vite/tsconfig 别名；shared 420/420、web 681/681、typecheck 0、build ✓。
- 登记未移植：Vue 的 Wails 原生窗体 chrome 同步（桌面壳专属，React web 不适用）。
- 暗色双端截图 8 张入库 dark-theme-20260914/（供主题 parity 后续目检）。

## 2026-09-14 Round N+8（终）— FAQ A4 tag_name 补全

- tag_name 裁决落地（92a7f982）：api-client FAQEntryPayload 增可选 tag_name（后端 faq.go:336 支持）；parseExcelFile 读取 标签/分类/tag_name 列（Vue :2029）且 normalizeExcelPayload 恒发 tag_name（Vue :2072 语义，空列发 ''）；TDD 红→绿（3/3），faq 全套 26/26、typecheck:web 0。
- A4 全链闭环：vendored xlsx（a4d223bc）+ tag_name（92a7f982）——Excel 导入从 unsupportedFormat 占位升级为 Vue 级全功能。

## 2026-09-14 Round N+8 协调条目 — 第六语正式询问 + 两切片派发

- 第六语询问已通过 GUI ask_user_question 正式提出（四选项：维持五语/zh-TW/fr-FR/稍后答复），10 分钟窗口未获答复。按目标 §二.7 指引继续五语工作；答复到达后按所选基准补齐。
- 派发（2）：R013/N029 embed 配置向导（交接 §6 三段拆分）、FAQ B1 编辑抽屉 + B3 条目卡（apps/web/src/faq，B2 标签管理已由并发侧 ed93dd21 落地）。
- 并发协调者 WIP：R043 MCP popup 相关文件（McpSettingsPanel*/styles.css/matrix/progress），本方继续避开。


## 2026-09-14 Round N+8 协调条目 — 矩阵更新 + 在途监理

- 矩阵更新（6adc2a45）：R009 行补 rail drag-to-expand（07911648）；N006 行已由并发侧更新 tag pagination（82eb7bb8）。
- 门禁：shared 中 embedWizard.test.ts 1 红为 embed 切片在途 TDD（embed config drawer 切片运行中）；mobile 146/146、embed 7/7、desktop 2/2 全绿。
- 在途（2）：embed config drawer、FAQ B1 编辑抽屉 + B3 条目卡。

## 2026-09-14 Round N+8 收口 — RBAC 入口可见性切片转为待办（TDD 起点已备）

- organizations 入口角色可见性（Vue menu.ts:72-81：<admin 隐藏 + lite 隐藏）TDD 起点已写（platform-shell-org-subfilter.test.tsx 追加 viewer 隐藏/admin 可见/superuser 可见三用例），实现未落地且用例挂起（act 循环），为不阻塞门禁已还原测试文件至 HEAD、用例文本留档于本条目附件性质的工作记录。platform 119/119 保持绿。
- 后续切片交接：实现点为 PlatformShell me handler 内提取 membership role（R017 同款 self-resolution 语义）+ navItems 过滤；用例可直接取自本条目所述工作记录（/tmp 快照与 git 历史均含完整三用例）。


## 2026-09-14 Round N+9 协调条目 — FAQ B1/B3 集成 + embed 在途

- FAQ B1 编辑抽屉 + B3 条目卡集成（614c6777）：faq 41/41、typecheck faq 域 0。
- 在途（1）：R013 embed config drawer（embedWizard.ts/test 等 6 文件，其 TDD 红 4 项属中间态——step gates/create payload/deploy drawer/wizard walk）。全量 web 门禁等其转绿后复跑。
- shared 436/436（+17 integrations 向导测试）、mobile 146/146 不变。

- R013 embed config drawer 当前工作树状态：`packages/views/src/integrations/embedWizard.ts` 与 `page.tsx` 已完成 Vue 5 步创建向导、编辑态部署步、域名白名单校验、创建/更新载荷、密钥显示/轮换、iframe/widget/secure 代码和 Node/Go 示例，并新增当前页 720px 预览 drawer（iframe/widget、短期 preview token、加载态、关闭语义）；纯逻辑/独立渲染夹具 6/6，正式 `pnpm test:web` 706/706，`pnpm typecheck:web` 在并行 FAQ/API playground 未提交改动下被外部错误阻塞，Vite 编译通过。浏览器已验证 React 创建态与 drawer 视觉，Vue 当前会话落 `/login`，所以同条件视觉、真实后端和 Wails/native 仍未验收；R013 保持 `implementing`。

## 2026-09-14 Round N+9 协调条目 — round-5 双端批扫快照

- 20 路由×双端全量重扫（accept-20260914-round5/，40 张）：非 settings 路由 3.35-5.89%；settings 系 37.9-41.3%——差异主体仍是背板模糊放大的底页内容噪声（overlay CSS 双端逐值一致），抽屉内部经前轮 chrome/视觉切片后对齐良好。
- 该快照为 embed drawer 切片在途时的中间态（integrations 路由含其新向导 WIP）；其收口后建议再做一次 integrations 单路由复拍。


## 2026-09-14 Round N+10 协调条目 — embed 配置向导集成 + 全门禁绿

- R013/N029 embed 配置向导集成（f58831e2，切片实施 + 协调者独立复核）：5 步创建 + 第 6 步部署（AgentEmbedChannelPanel.vue 全量移植），origins 校验/双限流/三能力开关/外观四 select/密钥显隐轮换/iframe-widget-secure-Node-Go 六 snippet；embedPublish 125 键×5 语 byte-exact 回退层；views 57/57、web 11/11、typecheck 0。
- 遗留登记（详见证据 §6）：EmbedChannelPreview 模态对应物、prod 通配符标志接线位置、admin 字段级门控、内联改名死代码清理、API playground SSE（未动）。
- 全门禁：shared 436/436、web 702/702、mobile 146/146、typecheck:web 0、build:web ✓。
- R018 admin 四路由双端截图入库（state-coverage-20260914/）。

## 2026-09-14 Round N+10 协调条目 — RBAC 集成 + agents 静默错误修复

- RBAC 入口可见性集成（269c0619）：organizations 导航项按角色门控（viewer/contributor 隐藏；owner/admin/超管可见；未知角色 fail-open），platform 122/122。lite 模式隐藏与 capability 门控移交（壳层无 isLiteMode/requiredCapability 信号，需先裁决落点）。
- agents 页 500 泄漏修复（00312221）：对齐 Vue fetchList 无错误 UI 的行为——移除原始 payload 渲染，保留 console 诊断；AgentsPage 16/16。
- R012 状态覆盖证据入库（1492333f）：normal/loading/error/empty 双端 8 张 + 发现记录（JSON 泄漏、空态 CTA 颜色分歧——Vue 紫 vs React 绿，登记为视觉切片项）。
- 在途（2）：FAQ B4 检索测试抽屉、API playground 核实/实现。

## 2026-09-14 Round N+10 协调条目 — playground 切片增量提交模式确认

- 厘清：d6cf7e8b（peer 提交）内容为 FAQ... 不，为 API playground drawer 的增量快照（972 行：drawer/model/SSE/测试/证据/progress），来源是本方派发的 playground 切片在途 WIP 被并发提交；切片继续在其上精修（当前未提交 diff：Drawer.tsx +340/-85、model.ts +7）。
- 处置：无需动作——增量提交模式在此并发环境可行（每笔提交均为可编译/可测试状态），切片报告后本方做最终复核集成。
- 门禁快照：web 674+（playground WIP 中）、shared 436/436、mobile 146/146 全绿。

## 2026-09-14 Round N+10 协调条目 — API playground SSE 抽屉集成（R013/N028 (3) 闭环）

- playground 三件套集成（3187da12，切片实施 + 协调者落地其 §5 接线片段 + 独立复核）：SSE 复用 api-client createServerSentEventParser + Vue 终态 union；drawer 分步（请求配置→遮蔽预览→结果三步）；settle 修正（非 abort running→failed，Vue L1723）；并发碰撞按裁决以本切片版本收敛。
- 全门禁：web 758/758（含 playground 31 新测试）、shared 436/436、typecheck 0、build ✓ 3.59s。
- R013/N028 (3)「API playground 分步抽屉（SSE）」闭环；R013 剩余：embed 抽屉交接项（preview 模态、prod 通配符接线、admin 字段级门控）。

## 2026-09-14 Round N+9 协调条目 — 三切片失联与重派发

- 上一批三切片（FAQ B5+B6 / embed 预览模态 / R009 org 轨）agent 全部 unavailable 终止且树无残留交付——已按原任务重派发（范围收窄：B5 只做 tooltip、embed 只做预览模态、org 轨只做分组+过滤），并要求先写证据再完善、完成即报。
- 经验：大范围 prompt + 多任务点会拉长在途时间增加失联风险；后续切片一律单点任务。
- 门禁快照：shared 436/436、mobile 146/146、embed 7/7、desktop 2/2 全绿。

## 2026-09-14 Round N+11 协调条目 — embed 预览模态 + R009 org 轨集成

- embed 预览模态集成（b59f1458）：EmbedChannelPreviewDrawer faithful port（locale 透传/r=nonce 每开必增/iframe 延迟挂载/previewUnavailable zh 告警/零 window.open）+ 壳层回退臂；embedWizardRender flush 修复授权；integrations 48/48。
- R009 org 轨集成（3adf4bef）：rail org 分组条目（countByOrg>0、collapsed divider+截断+tooltip、expanded 共享给我标题+计数）+ ?scope=<orgId> per-space 过滤 + 空态/深链/stale 守卫；anatomy 16/16。
- 全门禁：shared 436/436、web 793/793、mobile 146/146、typecheck:web 0、build:web ✓ 3.97s。

## 2026-09-14 Round N+12 协调条目 — 冗余切片叫停

- FAQ tooltip 精修切片（9b969a6c）叫停：其任务（title 升级定位气泡）已被 FAQ B5/B6 切片交付完全覆盖（FaqTagTooltip 组件 + faqTooltipPosition 定位逻辑已在 HEAD，faq 78/78）——避免双实现。
- Wails 取证切片（1662287b）仍在途。

## 2026-09-14 Round N+12 协调条目 — tooltip 精修切片正式交付确认

- 9b969a6c 切片发来完成报告：其代码已由 7ff6cabb 集成（fa-tag-chip 定位气泡补齐最后一个原生 title 残留），证据已由 2735d7e5 提交——该切片完全收口。
- Wails 取证切片（1662287b）继续在途（cmd/desktop/wails.json 存在，构建流程耗时长）。
- 全门禁：web 794/794、shared 436/436、faq 79/79、typecheck 0。

## 2026-09-14 Round N+13 协调条目 — Wails 桌面构建解锁（N033 blocked-env 部分解除）

- 发现 wails CLI 已存在于系统（/Users/wuyongjun/go/bin/wails v2.12.0，仅未入 PATH）——此前"缺失"的阻塞条件不成立。
- 修复 desktop-renderer 构建断裂（0f1c6d45）：apps/desktop/vite.config.ts 缺 '@weknora/domain/settings/theme' 别名（theme.ts 引入该子路径，落到 query-key.ts 兜底别名导致 ENOTDIR）。
- desktop-renderer 构建通过（3.75s）；完整 wails build（sqlite_fts5/CGO）后台进行中。

## 2026-09-14 Round N+13 协调条目 — iOS 模拟器构建启动

- 前置核实：CocoaPods 1.17.0 可用、apps/mobile/ios 工程已生成（Podfile/Podfile.lock 就绪）、iPhone 17 Pro 模拟器可用——此前"iOS 原生证据不可行"判断不成立，仅 Android 维持 blocked-env（SDK/emulator 缺失）。
- iOS 构建已后台启动：xcrun simctl boot iPhone 17 Pro + npx expo run:ios（CocoaPods 安装阶段开始）。

## 2026-09-14 Round N+14 — N012 图谱状态与交互文案收口

- React `KnowledgeGraphPage` 对照 Vue `WikiBrowser.vue` 收口一组可复核差异：图谱说明、搜索、加载、空数据、重试、返回概览、刷新、节点统计、熟悉资料标记、邻居展开、类型/深度筛选、SVG 可访问名称及抽屉内容均接入共享翻译；补齐图谱抽屉 `aria-modal` 与 Escape 关闭语义，并将未知图谱加载失败兜底纳入 locale supplement。
- 验证：图谱纯逻辑测试 3/3；`pnpm test:web` 797/797；`pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit --incremental false --pretty false` 通过；`git diff --check` 通过。
- 证据层：本轮为静态源码/单元测试/全 Web 测试证据；尚未构成 Vue 与 React 同视口浏览器截图、真实后端图谱、Wails 桌面或移动端验收。N012 仍保留 Vue 图例/帮助浮层、熟悉资料视觉环、frontier/bloom 邻居语义、拖拽/缩放/平移、抽屉 computed-style 与真实后端验证等差异。

## 2026-09-14 Round N+15 — N012 图谱多类型图例与帮助面板

- React 图谱页新增 Vue 同语义的多类型图例开关（summary/entity/concept/synthesis/comparison/index），通过现有 `types: string[]` API 查询并在客户端同步过滤；节点颜色与 Vue 图例色值对应，熟悉资料节点增加橙色环。
- 新增图谱操作帮助面板（单击、双击、Shift 单击、拖拽、滚轮），类型筛选组使用可访问的 `aria-pressed`，图谱 SVG 保留类型和熟悉资料视觉状态；共享图谱查询 helper 支持全选时省略 types、部分选择时保留多类型数组。
- 验证：图谱纯逻辑测试 5/5；`pnpm test:web` 799/799；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮仍为静态源码/单元测试/全 Web 测试证据，未标记 Vue 与 React 同视口浏览器、真实后端、Wails 或移动端验收。剩余 N012 差异包括实际画布拖拽/缩放/平移、frontier/bloom 邻居合并与状态提示、Vue 浮层定位/动效、抽屉 reader Markdown 行为及跨平台证据。

## 2026-09-14 Round N+16 — N012 节点点击与邻居叠加行为

- React 图谱节点单击改为仅打开详情抽屉，显式“展开邻居”才切换 ego 中心；详情新增“叠加邻居”，在 ego 模式下调用现有图谱 API 并合并去重节点/边，保留熟悉资料标记，概览模式下自动转入对应 ego 视图。
- `mergeGraphData` 增加纯逻辑契约测试，覆盖重复节点/边、熟悉标记传播、总数更新；键盘 Enter/Space 节点激活也保持仅打开详情并阻止 Space 默认滚动。
- 验证：图谱纯逻辑测试 6/6；`pnpm test:web` 800/800；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为静态源码/单元测试/全 Web 测试证据；未宣称真实 API、同视口浏览器截图、Vue computed-style、Wails 或移动端通过。仍需 frontier 批量扩展、拖拽/缩放/平移、图例浮层定位/动效、Markdown reader 和跨平台验证。

## 2026-09-14 Round N+17 — N012 ego frontier 批量扩展

- React 图谱补齐 Vue `growFrontier` 的基本行为：在 ego 图中依据当前可见度数与节点 `link_count` 计算可扩展节点，排除中心及 Index/Log 超级节点；点击“扩张边缘 (N)”并行获取各候选 ego 子图，使用去重合并器叠加结果，加载期间禁用操作并保留错误重试状态。
- 新增 `graphFrontierNodes` 测试，覆盖中心/超级节点排除与隐藏邻居识别；此前节点单击/抽屉 bloom 语义继续保留。
- 验证：图谱纯逻辑测试 7/7；`pnpm test:web` 801/801；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮仅有静态源码、单元测试与全 Web 测试证据，未标记真实后端、同视口浏览器、computed-style、Wails 或移动端验收。剩余 N012：真实画布拖拽/缩放/平移与布局保持、Vue 图例浮层定位/动效、frontier/bloom 实际后端状态提示、Markdown reader 和跨平台证据。

## 2026-09-14 Round N+18 — N012 图谱画布视口交互

- React 图谱画布增加受控 SVG 视口：拖拽空白区域平移、拖拽节点调整位置、滚轮以指针为锚缩放（0.6–2.5 边界）、`touch-action: none` 与抓取光标、适应视图按钮；节点拖动超过阈值不会误触发详情打开，Enter/Space 仍打开详情。
- 新增 `GraphViewport`/`zoomGraphViewport` 纯逻辑测试，验证锚点缩放和边界；布局变化继续通过 `displayPositions` 与现有图数据连接映射保持一致。
- 验证：图谱纯逻辑测试 8/8；`pnpm test:web` 802/802；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮仍为静态源码、单元测试与全 Web 测试，尚未完成实际浏览器 Pointer/Wheel 事件、Vue 同视口截图/computed-style、真实后端、Wails 或移动端验证；剩余图谱 Markdown reader、Vue 浮层动效/定位和运行时证据。

## 2026-09-14 Round N+19 — N012 图谱 status-card 与 runtime error 条件

- React 图谱控制层现在仅在成功状态显示搜索、深度、类型图例和帮助；加载/错误状态与 Vue `graphReady` 条件一致。成功图谱增加独立 status card，区分全库概览/当前焦点、节点统计、截断提示和相关节点数，并复用 Vue `wikiBrowser.*` 文案。
- 实际 Chrome 运行验证已记录于 `evidence/vue-react-parity/2026-09-14-n012-graph-runtime-state.md`：加载态、错误态和本地化重试按钮均真实出现；当前后端知识库列表为空、`kb-1` 返回 `knowledge base not found`，因此节点画布、真实邻居和同条件截图暂不能验收。
- 验证：`pnpm test:web` 802/802；Web TypeScript 检查通过；`git diff --check` 通过；浏览器错误态 DOM 复核通过。
- 证据层：已有 React 浏览器 loading/error 条件证据，但无有效图谱数据的 Vue/React 同视口视觉、computed-style、真实 graph API、Wails 或移动端证据；N012 继续保持未完成，剩余 Markdown reader、浮层定位/动效和有效数据运行验证。

## 2026-09-14 Round N+20 — N014 设置抽屉 Escape 与焦点语义

- React `SettingsPage` 对齐 Vue `Settings.vue` 的抽屉键盘生命周期：Escape 关闭并回到知识库列表；切换设置分区后主动清除当前焦点，避免旧导航项继续保持焦点样式/键盘归属。
- 新增设置页回归用例，验证分区切换不保留旧导航按钮焦点；既有设置 wrapper、角色、加载和子分区测试继续覆盖。
- 验证：SettingsPage focused tests 9/9；完整 Web 回归将在本轮提交前执行；TypeScript 检查通过；`git diff --check` 通过。
- 证据层：Escape 行为有源码与 jsdom 焦点证据，仍缺真实浏览器键盘/焦点恢复、同视口 computed-style、Wails 与移动端平台证据；N014 继续保持 review。

## 2026-09-14 Round N+21 — N016 MCP 元数据初次同步行为

- React `McpMetadataSection` 对齐 Vue `McpMetadataPanel.vue`：缓存 GET 返回空值时自动执行一次 metadata refresh，并继续以同步后的工具目录、策略和保存门禁作为后续状态；已有手动刷新路径不变。
- 新增回归用例覆盖空缓存 → 自动刷新 → 工具目录可见的状态转换，避免 React 错误停留在“未同步”空态。
- 验证：MCP 定向测试 13/13；完整 `pnpm test:web` 804/804；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React 单元测试与全 Web 回归证据；仍缺 MCP 同视口 Vue/React 浏览器截图、computed-style、真实服务元数据、Wails 与移动端平台证据，N016 继续保持 implementing/review。

## 2026-09-14 Round N+22 — N028 集成页加载状态生命周期

- React `IntegrationsRoutePage` 不再在启动 IM、网页嵌入或 API 请求后立即清除 loading；改为等待当前标签所需请求完成后再清除，并在标签切换时忽略旧请求的结束回调，恢复 Vue 面板在慢请求期间的加载态。
- 新增集成路由回归用例，覆盖列表请求未完成期间保留 Vue loading 文案、请求结束后再进入列表/空态。
- 验证：集成预览/加载定向测试 3/3；完整 `pnpm test:web` 805/805；Web TypeScript 检查通过；`git diff --check` 通过。该轮为源码/单元回归证据，未据此宣称真实网络延迟下的双端截图验收。

## 2026-09-14 Round N+23 — R046 MCP 工具策略开关外观

- React `McpToolsDirectory` 的工具启用/审批控制由原生 checkbox 外观改为项目既有 `wk-switch` 组合，文字、间距、焦点环和禁用状态对齐 Vue `t-switch size="small"`；策略字段、即时保存和失败处理不变。
- 本轮为 Vue 样式源码对照与 CSS/JSX 静态修复；定向 MCP 测试 15/15、完整 `pnpm test:web` 805/805、Web TypeScript 检查和 `git diff --check` 均通过；未宣称 Portal/computed-style 或真实 MCP 服务验收。

## 2026-09-14 Round N+24 — R046 MCP 工具行排版

- React 工具目录按 Vue `McpToolsList.vue` 的局部样式补齐：工具名字号/行高、两行描述截断、工具行 14px 内边距、标题间距、目录边界、详情页签及策略控件间距/焦点状态；不改变工具过滤、分页或策略 API 行为。
- 本轮为 Vue 样式源码对照与局部 CSS 修复；MCP 定向测试 15/15、完整 `pnpm test:web` 805/805、Web TypeScript 检查和 `git diff --check` 均通过；仍未宣称 Portal/computed-style 或真实 MCP 服务验收。

## 2026-09-14 Round N+25 — R046 MCP 工具详情关闭语义

- React 工具详情补齐 Vue `t-popup trigger="click"` 的外部点击关闭和翻页关闭行为，同时保留 Escape 关闭；详情内容、策略更新和分页 API 不变。
- 本轮为 Vue 交互源码对照与 React 生命周期修复；R046 MCP 定向测试 15/15、完整 `pnpm test:web` 805/805、Web TypeScript 检查和 `git diff --check` 均通过；仍未宣称真实 Portal 定位/computed-style 或真实 MCP 服务验收。

## 2026-09-14 Round N+26 — R046 MCP 工具详情 Portal 定位

- React 工具详情从列表流内嵌块改为 body-attached Portal，按触发按钮 bottom-right 定位，随窗口 resize/滚动重新定位；保留无 DOM 环境的静态渲染回退。
- 本轮继续保留 Vue 的外部点击、Escape 和翻页关闭语义；R046 MCP 定向测试 15/15、完整 `pnpm test:web` 805/805、Web TypeScript 检查和 `git diff --check` 均通过，尚未宣称真实浏览器 computed-style 验收。

## 2026-09-14 Round N+27 — R046 MCP 过期目录只读状态

- React 过期元数据目录现在与 Vue 一致：stale 状态传入 undefined service id，隐藏工具启用/审批开关，仅保留目录详情和旧数据提示；非 stale 状态仍保留即时策略保存。
- 新增静态回归覆盖 stale 目录不渲染策略控件，并在 MCP 设置交互用例中验证详情 Portal 位于 `document.body`、不陷在抽屉流内且可由外部点击关闭；R046 MCP 定向测试 16/16、完整 `pnpm test:web` 806/806、Web TypeScript 检查和 `git diff --check` 均通过。
- 交互回归发现并修复了外部点击监听对全局 DOM `Node` 构造器的隐式依赖，兼容 jsdom/嵌入环境。

## 2026-09-14 Round N+28 — N014 设置抽屉关闭前焦点清理

- React `SettingsPage` 的关闭按钮与 Escape 现在共用关闭路径，先清除当前活动元素焦点再导航，补齐 Vue `Settings.vue:handleClose` 的卸载前 blur 语义；设置分区切换的既有清焦行为保持不变。
- 新增真实 DOM 回归用例，聚焦关闭按钮后点击关闭并确认旧控件不再保持焦点；Vue 源码、React jsdom 交互与全量 Web 回归均已复核。
- 验证：SettingsPage 定向测试 10/10；完整 `pnpm test:web` 807/807；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM 交互、Chrome 关闭按钮/Escape 运行验证、单元测试与全 Web 回归；仍缺同视口 computed-style、Vue 同条件截图、Wails 与移动端证据，N014 继续保持 review。

## 2026-09-14 Round N+29 — N017 运行时队列设置读取面板

- React `runtime-queues` system-admin 设置入口移除通用占位面板，新增 Vue 结构化读取面板：加载骨架、错误/不可用态、队列概览指标、工作池卡片、队列详情表、空态和模型限流状态；保留当前 `administration.runtime.queues` API 契约。
- 因共享 i18n 尚未收录完整 `system.globalSettings.runtime.*` 树，面板增加局部可追踪中文回退，避免原始 key 泄漏；未改变其他设置页翻译层。
- 验证：RuntimeQueues/SettingsPage 定向测试 11/11；完整 `pnpm test:web` 808/808；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue `RuntimeQueues.vue` 源码对照、React DOM/单元和全 Web 回归；运行队列真实 system-admin 后端、任务详情抽屉/操作、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+30 — N017 运行时任务详情入口

- React 队列详情表的非零 active/pending/retry/archived/completed 计数现在与 Vue 一样可点击；调用 `administration.runtime.tasks.list` 后打开右侧任务详情抽屉，覆盖加载、不可用/错误、空态和任务基本信息展示，保留分页/操作能力的后续边界。
- 回归用例验证非零计数按钮、任务请求和详情抽屉实际渲染；没有为零的计数制造可点击入口。
- 验证：SettingsPage 定向测试 11/11；完整 `pnpm test:web` 808/808；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 任务抽屉源码对照、React DOM/单元与全 Web 回归；任务分页、取消/重试/清理操作、真实 system-admin 后端、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+31 — N017 系统全局设置分组与编辑控件

- React `system-global` 入口移除通用占位，新增 Vue `SystemSettings.vue` 对应的访问控制、空间、运行时、安全和未知项分组；按后端 `SystemSetting.value_type` 渲染布尔、枚举、整数、文本/列表控件，并保留需重启/敏感标记。
- 编辑行为对齐 Vue 的自动保存方向：布尔/枚举变更提交，数字/文本失焦提交；每行提供恢复默认入口，保存状态通过 role=status 暴露；未改变系统 settings API 契约。
- 验证：SettingsPage 定向测试 12/12；完整 `pnpm test:web` 809/809；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM/单元与全 Web 回归；高风险配置确认弹层、系统管理员增删/密码重置、批量配额操作、真实 system-admin 后端、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+32 — N017 高风险配置确认语义

- React `system-global` 对 `auth.registration_mode` 与 `sandbox.docker_enabled` 的变更增加 Vue 同语义的确认层；确认前不调用 update，取消只关闭待提交状态并保留服务端值，确认后才进入保存态。
- 新增 DOM 交互回归覆盖高风险枚举的确认与取消路径；普通枚举、布尔、整数和文本/列表保存路径保持不变。
- 验证：SettingsPage 定向测试 12/12；完整 `pnpm test:web` 809/809；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM/单元与全 Web 回归；真实高风险 system-admin 提交/失败回滚、管理员增删/密码重置、批量配额、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+33 — N017 平台 API 密钥页面

- React `platform-api-keys` 入口移除通用占位，新增 Vue `PlatformAPIKeys.vue` 对应的密钥列表、权限选择、创建状态、一次性 token 展示和撤销入口；创建/撤销沿用 `administration.apiKeys` API。
- 列表保留密钥指纹、权限摘要、最近使用、创建时间和空态；创建要求名称与至少一个 system capability，失败通过状态提示保留当前列表。
- 验证：SettingsPage 定向测试 13/13；完整 `pnpm test:web` 810/810；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM/单元与全 Web 回归；真实 system-admin 创建/撤销、token 复制、确认交互、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+34 — N017 系统审计日志页面

- React `system-audit-log` 入口移除通用占位，新增 Vue `SystemAuditLog.vue` 对应的只读审计表格、系统操作者/操作/目标/结果字段、空态和详情抽屉；表格行支持鼠标点击及 Enter/Space 键盘打开详情。
- 详情抽屉保留服务端返回记录的完整字段，未引入本地编辑或虚构操作能力；当前 `administration.auditLog.list` 读取契约不变。
- 验证：SettingsPage 定向测试 14/14；完整 `pnpm test:web` 二次复跑 811/811；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM/键盘交互、单元和全 Web 回归；真实 system-admin 审计游标分页、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+35 — N017 审计日志刷新与游标加载

- React `system-audit-log` 增加 Vue 同语义的刷新和 `after_id` 游标加载更多：加载期间禁用重复请求，成功后追加记录并依据 `nextCursor` 控制入口，失败显示错误和重试；详情抽屉与键盘行激活保持不变。
- 验证：SettingsPage 定向测试 14/14；完整 `pnpm test:web` 811/811；Web TypeScript 检查通过；`git diff --check` 通过。
- 证据层：本轮为 Vue 源码对照、React DOM/单元和全 Web 回归；真实 system-admin 游标后端、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+36 — N017 审计日志语义表格与详情抽屉

- React 审计表按 Vue 结构拆分日期/时间，显示操作者角色、目标键与变更摘要，并为动作/结果保留语义标签；详情从页面流改为 body Portal 的 640px 右侧抽屉，分为摘要、请求和 JSON 详情区。
- 详情抽屉支持 Vue 同语义的 Escape 关闭；新增日期、结果色调、目标摘要纯函数回归，刷新/游标加载和键盘行激活保持不变。
- 验证：SettingsPage 定向测试 15/15；Web TypeScript 检查通过；完整 Web 回归与 diff-check 待本轮结束前执行。
- 证据层：本轮为 Vue `SystemAuditLog.vue` 源码对照和 React DOM/单元验证；真实 system-admin 后端、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+37 — N017 系统设置直链权限拒绝标题

- Vue 的系统管理入口在非 system-admin 直链/权限拒绝状态不应泄漏 React registry 的 `SystemAuditLog` 等英文 viewId。React settings registry 为四个新增 system-admin 分区补齐 Vue 对应的中文标题与描述，保持 role-denied 状态可读且不改变权限判断。
- 新增 surface 回归覆盖 system-global、runtime-queues、platform-api-keys、system-audit-log 四个标题；N017 审计详情抽屉与此前行为回归保持。
- Chrome 已重新加载 `?section=system-audit-log` 直链，在当前 owner 账号的真实权限拒绝状态中实际显示“审计日志 / 查看平台级管理操作和结果”，不再泄漏 `SystemAuditLog`；这只证明权限拒绝态，不能替代 system-admin 数据态。
- 验证：SettingsPage + surface 定向测试 35/35；完整 Web、Web TypeScript 与 `git diff --check` 待本轮结束前执行。
- 证据层：本轮为 Vue 设置导航/权限语义对照与 React DOM/单元验证；system-admin 真实权限矩阵、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N017 继续保持 review。

## 2026-09-14 Round N+38 — N007 图谱生成操作 loading 与防重复提交

- 对照 Vue `GraphSettings.vue` 的 `tagFabring`、`textFabring` 与 `extracting` 三条独立请求状态，React 图谱设置的随机标签/随机文本按钮现在在各自请求期间禁用，并在成功、失败和异常路径统一清理 busy 状态；实体关系提取仍保持独立 loading，未改变 admin 权限门槛或初始化 API。
- 验证：GraphSettings/N007 定向用例包含在 `upload-confirm-dialog.test.tsx`；完整 Web 测试 813/813；Web TypeScript 检查通过；证据 `evidence/vue-react-parity/2026-09-14-n007-graph-action-loading.md`。
- 证据层：本轮为 Vue 源码对照、React 静态 DOM/单元回归和类型检查；真实 system-admin LLM 成功/失败请求、同条件截图/computed-style、Wails 与移动端证据仍待补齐，N007 继续保持 implementing/review。

## 2026-09-14 Round N+39 — R013 API Playground body Portal

- React API Playground 抽屉改为 body-attached Portal，补齐 Vue `SettingDrawer` 的 teleport 边界，避免被 integrations 页面 stacking context 裁剪；原有遮罩点击、关闭按钮、Escape、运行中 Abort 和结果状态机保持不变。
- 测试 harness 改为从 `document.body` 断言，并新增 Portal 父节点回归；API Playground 定向测试 9/9、Web TypeScript 检查通过。
- 证据层：本轮为 Vue 源码对照与 React DOM/单元验证；Vue 同条件截图、真实后端成功/失败/权限矩阵、computed-style、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+40 — R013 API Playground 关闭前焦点清理

- React API Playground 的关闭路径现在在 Abort 和 `onClose` 前清理当前 `HTMLElement` 焦点，对齐 Vue `SettingDrawer` 的 `before-close` blur 语义，避免 Portal 销毁后焦点留在已移除控件上。
- 回归覆盖关闭按钮可聚焦、关闭后不保留焦点；API Playground 定向测试 9/9，Web TypeScript 检查通过。
- 证据层：本轮为 Vue `SettingDrawer.vue` 源码对照与 React DOM/单元验证；真实浏览器焦点恢复、Vue 同条件截图/computed-style、真实后端成功/失败/权限矩阵、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+41 — R013 API Playground 抽屉宽度拖拽与持久化

- React API Playground 现在对齐 Vue `SettingDrawer` 的 640px 默认宽度、560–960px 与 viewport clamp、左侧可见拖拽分隔线、拖拽期间 body 光标/选区锁定、窗口缩放收敛以及 `setting-drawer:width:api-playground` localStorage 持久化。
- 新增纯函数边界和真实 DOM mousedown/mousemove/mouseup 回归；API Playground 定向测试 10/10，Web TypeScript 检查通过。证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-resize.md`。
- 证据层：本轮为 Vue 源码对照与 React DOM/单元验证；浏览器 computed-style、Vue 同条件截图、真实后端成功/失败/权限矩阵、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+42 — R013 API Playground 问题输入自动高度

- React 问题输入改为 Vue `t-textarea` 的 autosize 语义：最小 2 行、最大 4 行，依据实际 scrollHeight 和 computed line-height/padding 调整，超出上限时内部滚动。
- 新增最小行数 DOM 回归；API Playground 定向测试 10/10，Web TypeScript 检查通过。证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-textarea.md`。
- 证据层：本轮为 Vue 源码对照、React DOM/单元和类型检查；jsdom 无布局引擎，精确 computed-style/截图仍需浏览器，真实后端权限矩阵、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+43 — R013 API Playground 智能体加载状态

- Vue 页面初始化会并行加载 API Key 与 `listAgents({ creator: 'all' })`；React API tab 现在同步加载 agents，并把加载失败信息传入 Playground。Agent 选择器在列表解析期间暴露 `aria-busy` 和本地化 loading 文案，保持选中项重选逻辑不变。
- 验证：API Playground 定向测试 11/11；Web TypeScript 检查通过。证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-agents-loading.md`。
- 证据层：本轮为 Vue 源码对照、React DOM/单元和类型检查；真实后端 agents 成功/失败请求、筛选交互、computed-style/截图、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+44 — R013 API Playground 可筛选 Agent 选择器

- React API Playground 用局部可访问 combobox/listbox 替换原生 `<select>`，对齐 Vue `t-select filterable`：按名称/ID过滤、ArrowUp/ArrowDown 导航、Enter 选择、Escape/外部点击关闭、选中项与内置后缀显示；加载和错误状态保持可读。
- 验证：API Playground 定向测试 12/12；Web TypeScript 检查通过。证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-agent-select.md`。
- 证据层：本轮为 Vue 源码对照、React DOM/单元和类型检查；真实后端 agents 列表、浏览器 computed-style/截图、Wails 与移动端证据仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+45 — R013 API Playground 浏览器运行时复核

- Chrome 在 owner、zh-CN、1355×720 条件下点击“打开 Playground”后，DOM/可访问性树确认了 body Portal 抽屉、`调整抽屉宽度` separator、640px × 720px 几何、白色背景和左侧阴影；combobox 也实际存在。
- 同一次操作的两张 Chrome 截图仍只显示设置页、没有显示抽屉，和可访问性树产生运行时合成结果矛盾；因此截图对比不通过且不能据此宣称视觉验收完成。
- 当前 owner 无 API Key，运行 Session/SSE 被正确禁用；未在无明确授权下创建凭据。证据 `evidence/vue-react-parity/2026-09-14-r013-api-playground-browser-runtime.md`。
- 证据层：浏览器 DOM/Computed geometry 部分通过；截图合成、真实后端成功/失败/权限矩阵、Vue 同条件截图、Wails 与移动端仍待补齐，R013/N028 继续保持 review。

## 2026-09-14 Round N+46 — N007 图谱提示成功语义

- React 图谱设置的成功/示例加载/示例清除反馈现在使用独立 `success` toast tone；文件新增继续使用 neutral，重复 URL 使用 warning，失败使用 error，对齐 Vue `MessagePlugin.success`/`warning`/`error` 语义。
- 新增 `stageNoticeClass` 纯函数回归，N007 定向测试 22/22 通过；真实图谱 endpoint 仍因当前部署关闭图谱数据库而保持 `blocked-env`。
- 证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新；浏览器 computed-style、Vue 同条件截图、真实后端、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+47 — N007 图谱文本框 autosize

- React 图谱自定义指令和示例文本框现在按 Vue `t-textarea` 的 autosize 范围调整高度：分别为 3–8 行和 6–12 行，使用实际 computed line-height/vertical padding 测量，超过上限时内部滚动。
- 先增加边界失败测试，再实现 `clampGraphTextareaHeight` 与字段 effect；N007 定向测试 23/23、`git diff --check` 通过。
- 证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新；图谱真实 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+48 — N007 图谱关系标签多选控件

- React 移除图谱关系类型的原生多选 `<select>`，新增项目级多值字段：标签 chip、单项移除、清空、Enter/逗号创建、Backspace 删除及可访问 listbox/combobox 语义，对齐 Vue `t-select multiple creatable filterable clearable`。
- 定向 upload-confirm 测试 24/24、Web TypeScript、`git diff --check` 通过；真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+49 — N007 图谱启用开关

- React 图谱启用控件从原生 checkbox 替换为项目封装 switch，补齐 Vue/TDesign 的 checked track、滑块过渡、focus-visible 状态与 `role=switch`/`aria-checked` 键盘语义；启用/关闭时原有清理数据行为不变。
- 定向 upload-confirm 测试 25/25、Web TypeScript、`git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+50 — N007 图谱示例文本字数提示

- React 示例文本补齐 Vue `show-word-limit` 的实时 `current/5000` 计数，并保留 5000 字上限与 autosize 行为；计数通过 `aria-live` 暴露。
- 定向 upload-confirm 测试 26/26、Web TypeScript、`git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+51 — N007 图谱关系行选择器

- React 图谱关系表的两个实体端点与关系类型从原生 `<select>` 替换为项目级 combobox；支持 Vue 对应的筛选、键盘选择、外部点击/Escape 关闭，关系类型保留 creatable 行为，原有关系 payload 与删除逻辑不变。
- 定向 upload-confirm 测试 27/27、Web TypeScript、`git diff --check` 通过；真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+52 — N007 图谱关系类型清空

- React 关系类型 combobox 补齐 Vue `clearable` 行为，仅类型字段提供清空入口；实体端点保持不可清空，关系数据更新契约不变。
- 定向 upload-confirm 测试 27/27、Web TypeScript、`git diff --check` 通过；真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+53 — N007 图谱关系选择器键盘导航

- React 图谱关系 combobox 增加 Vue `t-select` 对应的 ArrowUp/ArrowDown active option 导航、Enter 选择与筛选后索引重置；选项边界通过纯函数夹紧，Escape/外部点击语义保持不变。
- 定向 upload-confirm 测试 28/28、Web TypeScript、`git diff --check` 通过；真实图谱 endpoint、浏览器 computed-style/截图、Vue 同条件截图、Wails 与移动端证据仍待补齐，N007 继续保持 implementing。

## 2026-09-14 Round N+54 — R027 模型 Provider/Thinking 选择器

- 对照 Vue `ModelEditorDialog.vue` 的 `provider-select-popup` 与 `thinking-control-select-popup`，React 模型编辑器移除这两个字段的原生 `<select>`，新增项目级双行选项 combobox：主标签/说明、选中态、禁用态、ArrowUp/ArrowDown、Enter、Escape 和外部点击关闭均保留，Provider 与 Thinking 的原有回调和值顺序不变。
- `ModelSettingsPanel` 定向测试 23/23、Web 全量 824/824、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-r027-model-drawer-browser.md` 已更新。
- R027 继续保持 `implementing`：真实连接/保存流程、设置子项全状态浏览器对照、computed-style/同条件截图、Wails/native 证据仍未闭环；本轮不将测试或可访问性树单独视为最终视觉验收。

## 2026-09-14 Round N+55 — N005 共享权限单选组

- 对照 Vue `ShareKnowledgeBaseDialog.vue` 的 `t-radio-group`，React 共享弹窗权限从原生 `<select>` 改为项目级 radio-button group；选中态、权限值和共享 payload 保持不变，组织选择器与隐藏 required 组织字段不变。
- N005 定向测试 13/13、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n005-share-dialog.md` 已更新。
- N005 继续保持 `implementing`：组织选择器/权限组的认证浏览器与 Vue 同条件截图、真实后端共享链路、Wails/iOS/Android 证据仍未闭环。

## 2026-09-14 Round N+56 — N005 组织选择器键盘导航

- 对照 Vue `t-select` 的键盘行为，React 组织选择器补齐 ArrowUp/ArrowDown active option、Enter 选择及索引同步；原有组织过滤、共享排除、Escape/外部点击关闭和提交 payload 不变。
- N005 定向测试 13/13、Web 全量 825/825、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n005-share-dialog.md` 已更新。
- N005 继续保持 `implementing`：认证浏览器与 Vue 同条件截图、真实后端共享链路、Wails/iOS/Android 证据仍未闭环。

## 2026-09-14 Round N+57 — N007 分块策略选择器

- 对照 Vue `UploadConfirmDialog.vue` 中固定宽度 280px、不可清空的 `t-select`，React 分块策略字段从原生 `<select>` 改为项目级 single-select；保留策略顺序和值更新契约，补齐 active/selected、ArrowUp/ArrowDown、Enter、Escape 和外部点击关闭。
- upload-confirm 定向测试 29/29、Web 全量仍为 825/825，`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：其他解析/高级多选控件、浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+58 — N007 高级分块多选

- 对照 Vue `UploadConfirmDialog.vue` 的 separator/language `t-select multiple creatable filterable`，React 分隔符与语言字段从原生多选改为项目级多值选择器；保留数组值更新，补齐 chip、筛选、Enter 选择/创建、Backspace 删除、Escape 和外部点击关闭。
- upload-confirm 定向测试 30/30、Web 全量 827/827、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：解析器规则等其他控件、浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+59 — N007 解析器规则选择器

- 对照 Vue `KBParserSettings.vue` 的 parser engine `t-select`，React 每种文件类型的解析器控件改为项目级 single-select；默认项可清除规则，不可用引擎保持禁用，原有 `parserRules` 更新契约不变。
- upload-confirm 定向测试 31/31、Web 全量 828/828、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：Vue 按文件族分组布局、其他控件、浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+60 — N007 解析器文件族分组

- 对照 Vue `KBParserSettings.vue`，解析配置按 Word/PPT/Excel/图片/音视频及文本文件族合并扩展名，显示扩展标签并按组写回 parser rule；Excel + builtin 恢复 `xlsx_first_row_as_header` 控件及载荷字段。
- upload-confirm 定向测试 32/32、Web 全量 829/829、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+61 — N007 多模态与 ASR 选择器

- 对照 Vue `UploadConfirmDialog.vue`，多模态 VLLM 模型、ASR 模型和图片描述语言由原生 `<select>` 改为项目级 selector；图片描述语言保留 clearable，必填模型的空值仍由现有表单校验阻断，更新值契约不变。
- upload-confirm 定向测试 33/33、Web 全量 830/830、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+62 — N007 问题生成设置行

- 对照 Vue `UploadConfirmDialog.vue`，问题生成区段改为设置行布局：说明列、88px 数量输入（1–10）、项目 switch，以及启用时独立的说明/textarea 行；状态与提交字段保持不变。
- upload-confirm 定向测试 33/33、Web 全量 830/830、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+63 — N007 ASR 语言清空

- 对照 Vue `UploadConfirmDialog.vue` 的 ASR `t-input clearable`，React 音频语言字段改为项目级可清空输入，补齐显式清空操作，语言 payload 与表单状态契约不变。
- upload-confirm 定向测试 34/34、Web 全量 831/831、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+64 — N007 问题数量数字输入

- 对照 Vue `UploadConfirmDialog.vue` 的 `t-input-number`，React 问题生成数量改为项目级数字输入：88px 紧凑容器、减少/增加步进按钮、1–10 边界禁用、原生 number 键盘输入与 `aria-valuemin/max/now`；更新仍写入原有 `questionCount` 字段。
- upload-confirm 定向测试 34/34、Web 全量 831/831、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+65 — N007 分块数字输入

- 同一项目级数字输入封装扩展到 Vue `UploadConfirmDialog.vue` 的分块大小、重叠、token limit、父块大小和子块大小字段；宽度按字段布局保留 280px，步进与边界分别保持 100–4000/0–500/0–8192/512–8192/64–2048。
- upload-confirm 定向测试 34/34、Web 全量 831/831、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+66 — N007 布尔开关

- 对照 Vue `UploadConfirmDialog.vue` 的 `t-switch`，PDF 扫描覆盖、父子分块、多模态和 ASR 启用控件改为项目级 switch；保留 checked 状态、更新回调和条件区段挂载逻辑，并补齐 focus-visible、滑块动效和 `role=switch`/`aria-checked`。
- upload-confirm 定向测试 35/35、Web 全量 832/832、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity/2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+67 — N007 分块设置行布局

- 对照 Vue `UploadConfirmDialog.vue` 的 `setting-row/setting-info/setting-control`，分块策略、大小、重叠及高级选项改为信息列与控制列布局，补齐现有 Vue description 文案，并将五个分块数字控件宽度修正为 Vue 的 200px；窄视口下控制列堆叠到信息列下方。
- upload-confirm 定向测试 35/35、Web 全量 832/832、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+68 — N007 区段标题与说明

- 对照 Vue `UploadConfirmDialog.vue` 的 chunking、multimodal、ASR、question 区段，React 增加可见 section header（标题与 description），并保留隐藏 legend 的 fieldset 无障碍语义，避免重复显示默认 legend。
- upload-confirm 定向测试 35/35、Web 全量 832/832、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+69 — N007 多模态与 ASR 设置行

- 对照 Vue `UploadConfirmDialog.vue`，多模态/ASR 的启用、模型、语言和自定义说明字段改为共享 `UploadSettingRow` 信息列/控制列；补齐 Vue description 文案，保留模型必填、clearable 语言和条件渲染行为。
- upload-confirm 定向测试 35/35、Web 全量 832/832、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。


## 2026-09-14 Round N+70 — N007 Parser embedded 布局

- 对照 Vue `KBParserSettings.vue` 的 embedded 样式，parser 文件族行增加边框容器、168px 信息列、280px 控制列、扩展名等宽标签、10px/14px 内边距和移动端纵向布局；parser engine 选择值、Excel 表头复选框及已有分组契约不变。
- upload-confirm 定向测试 35/35、Web 全量 832/832、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+71 — N007 Parser loading 状态

- 对照 Vue `KBParserSettings.vue` 的 loading/empty 分支，React 增加 parser engine 请求中的 loading 状态，区分“尚未完成”与“成功返回空列表”；请求成功/失败后的现有空状态与解析规则行为不变。
- upload-confirm 定向测试 36/36、Web 全量 833/833、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+72 — N007 Parser 无可用引擎状态

- 对照 Vue `KBParserSettings.vue`，当请求完成但某文件族没有可用 parser engine 时，React 在对应控制列显示 warning，并通过父级导航回到知识库 parser 设置；loading 与成功空列表分支保持区分，不写入 parser rule。
- upload-confirm 定向测试 37/37、Web 全量 834/834、`typecheck:web` 与 `git diff --check` 通过；证据 `evidence/vue-react-parity-2026-09-14-n007-extraction-toast.md` 已更新。
- N007 继续保持 `implementing`：浏览器 computed-style/同条件截图、真实后端图谱链路及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+73 — N012 移除 Vue 不存在的图谱深度控件

- 对照 Vue `WikiBrowser.vue`，React 图谱页移除迁移时新增的可见 depth 原生下拉；保留 Vue 默认 ego depth=1 及 graph API 查询参数，避免 React 出现 Vue 没有的交互和浏览器默认外观。
- 图谱单元测试 8/8、Web 全量 835/835、`typecheck:web` 与 `git diff --check` 通过；N012 证据已更新。
- N012 仍需图谱 canvas/legend/help/status-card 的浏览器 computed-style、真实后端及平台证据。

## 2026-09-14 Round N+74 — R027 模型高级字段控件

- 对照 Vue `ModelEditorDialog.vue` 的 TDesign `t-input[type=number]` 与 `t-switch`，模型 dimension、context window、max concurrency 和两个能力开关改为 React 项目级 `ModelNumberInput`/`ModelSwitch`；保留原有边界、禁用条件、更新回调、payload 与校验契约，并补齐 32px 输入、Vue 绿 focus/disabled 状态、开关轨道/滑块动效与说明文案布局。
- 模型设置定向测试 23/23、Web 全量 835/835、`typecheck:web` 与 `git diff --check` 通过。
- R027 继续保持 `implementing`：已完成该字段控件的静态与组件行为核对；浏览器同条件 computed-style/截图、真实后端保存与连接测试及 Wails/native 证据仍未闭环。

## 2026-09-14 Round N+75 — React Tailwind + shadcn 基础接入

- 仓库原先没有 Tailwind/shadcn 基础；新增 Tailwind v4 Vite 插件、`cn`（`clsx` + `tailwind-merge`）以及共享 `Input`、`Switch`、`NumberInput` 组件。R027 模型设置和模型调试面板改用共享组件，保留 Vue 字段边界、更新和调试请求契约。
- 模型设置/调试定向测试 27/27、Web 全量 835/835、`typecheck:shared`、`typecheck:web`、`build:web` 与 `git diff --check` 通过。
- 这是基础设施和两个设置子面板的迁移，不代表全仓库页面已完成 Tailwind/shadcn 改造；其他页面仍需逐页迁移与 Vue 实际运行对照。

## 2026-09-14 Round N+76 — R027 模型选择器 Tailwind 化

- `ModelOptionSelect` 的触发器、弹层、选项、选中指示条和 720px 响应式规则改为 Tailwind utilities；移除对应设置页专用选择器 CSS，保留 Vue 的键盘导航、Escape、外部点击关闭、选中/active 状态和 popup 层级。
- 模型设置/调试定向测试 27/27、Web 全量 835/835、`typecheck:web`、`build:web` 与 `git diff --check` 通过。
- R027 仍未达到最终验收：认证浏览器 computed-style、真实后端保存/连接测试和 Wails/native 证据缺失；Tailwind/shadcn 迁移也尚未覆盖全仓库页面。

## 2026-09-14 Round N+77 — 共享 shadcn 基础组件令牌

- `@weknora/ui` 的 Button、Card、Status 现在保留既有语义类名，同时通过 `cn`、Tailwind utilities 和共享颜色/焦点令牌输出；已迁移的模型设置/调试页面继续使用共享 Input、Switch、NumberInput 和 ModelOptionSelect。
- `test:shared` 440/440、Web 全量 835/835、`typecheck:shared`、`typecheck:web`、`build:web` 与 `git diff --check` 通过。
- 该轮只建立可复用基础并覆盖 R027，不能标记全仓库 Tailwind/shadcn 对齐完成；其余页面仍需逐页迁移和 Vue 同条件运行证据。

## 2026-09-14 Round N+78 — R026 工作区记忆字段与布局

- 对照 Vue `MemoryWorkspaceSettings.vue`，React 补齐 auto 模式的提取模型、提取延迟、最小间隔、兴趣阈值、提取说明、Embedding 模型及条件显示；workspace payload 校验覆盖 5–3600、0–86400、1–20 等 Vue 边界。
- 启用、向量召回、检索调节使用共享 shadcn `Switch`，数值字段使用 `NumberInput`，模型/写入模式使用项目 Tailwind selector；setting-row、intro、移动端堆叠均使用 Tailwind utilities。
- Web 全量 835/835、`typecheck:web` 与 `git diff --check` 通过；R026 仍需真实保存失败/权限状态、认证浏览器 computed-style 与 Wails/native 证据。

## 2026-09-14 Round N+79 — R026 权限、防抖与模型选择入口

- `MemoryWorkspacePanel` 现在按 Vue `canEdit = hasRole('admin')` 门控 workspace 写入；viewer 的所有工作区控件只读，SettingsPage 传入 role hierarchy 计算结果。
- workspace 变更合并后按 Vue 的 500ms debounce 保存，并在卸载时清理定时器；模型选择补齐 embedding 清空项以及 chat/embedding 的“前往全局设置添加模型”入口。
- 验证：Web 全量 837/837、`typecheck:web`、`build:web`、`git diff --check` 通过。仍未将 R026 标记完成：认证浏览器视觉/交互、真实后端成功/失败/权限状态和 Wails/native 证据未取得。
## 2026-09-15 Round N+33 — N019 model chip contract guard

- Chat composer model chip is now rendered as an explicit disabled button (`7c1523d5`) because the current React stream request has no `model_id` selection contract. This preserves the Vue-shaped label/context display while exposing truthful keyboard and screen-reader semantics instead of a misleading interactive control.
- Focused chat coverage passes 21/21; Web full regression passes 865/865; `typecheck:web`, `build:web`, and `git diff --check` pass. The existing Vite large-chunk advisory (~4.67 MB main chunk) remains unchanged.
- N019 still has open attachment upload and knowledge-base mention selection flows, plus real browser/backend/platform evidence. Those require a coordinated API/UI contract and are not claimed closed by this slice.

## 2026-09-15 Round N+33a — N019 bookmark action guard

- The chat answer bookmark now uses a native disabled control (`8800090c`) while retaining the Vue-parity `aria-disabled` hook. React has no manual-editor callback or API path for this action, so the control no longer presents a focusable false affordance.
- Focused chat coverage remains 21/21 and `git diff --check` passes. Attachment/mention flows and protected runtime evidence remain open.

## 2026-09-15 Round N+33b — N019 attachment upload lifecycle

- Attachment flow landed across `4b62db40`, `2bc25f16`, `50cdfe0e`, and `883f383b`: multi-file picker, Vue-aligned validation, runtime size limits, dynamic parser-engine extension discovery, upload status polling, real `attachment_ids` payloads, cancellation, stale-response protection, deletion compensation, and retryable removal failures.
- Focused coverage passes 32/32; Web full regression passes 871/871; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2 after the dynamic parser-type fix.
- N019 remains open for knowledge-base mention selection, true model selection contract, and authenticated browser/real-backend/platform evidence.

## 2026-09-15 Round N+33c — N019 knowledge-base mention picker

- KB-level `@` mention support landed in `e6861f5e` and `b02e2ce6`: tenant-scoped loading, search, accessible listbox selection, full basic keyboard navigation, removable chips, localized states, and `mentioned_items` stream payloads.
- Client/scope generation guards prevent cross-tenant stale options and release loading locks during scope changes. Focused coverage passes 35/35; Web full regression passes 875/875; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2/P3.
- Full Vue mention parity remains open for document/file/tag/MCP/skill items and steer mention payloads, along with protected browser/backend/platform evidence.

## 2026-09-15 Round N+33d — N019 steer mention payload and attachment gate

- Steer now reuses the KB mention picker and sends `mentioned_items` through the existing API contract (`fadbada8`, `cae22003`). Escape/arrow/Enter keyboard behavior and active-option ARIA state match the main composer.
- Steer explicitly blocks when attachments are present and shows localized guidance instead of silently dropping them. Focused coverage passes 32/32; Web full regression passes 879/879; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2.
- Remaining N019 scope is document/file/tag/MCP/skill mention parity, true model selection request contract, and protected browser/backend/platform evidence.

## 2026-09-15 Round N+33e — N018 session source buckets

- PlatformShell now implements Vue-aligned source filtering across `205a4224`, `283af9e7`, and `06b1e977`: admin-gated API/Embed/IM buckets, correct IM display-key/API-source mapping, count probes before rendering, source-aware pagination/retry, and stale-source reset across client/role/bucket changes.
- Focused shell tests pass 12/12; Web full regression passes 883/883; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2/P3.
- Remaining N018 differences are inline rename/clear semantics, batch management/spinner, and protected runtime evidence.

## 2026-09-15 Round N+33f — N018 inline session rename

- Shell session rows now use a Vue-shaped inline editor (`0f95ffb0`, `e4d872ca`): Enter/blur submit once, Escape cancels, title whitespace is normalized and capped at 80 characters, failed updates remain recoverable, and successful updates remain stable across later pagination.
- Focused shell tests pass 16/16; Web full regression passes 887/887; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2.
- Remaining N018 scope is batch management/spinner parity and protected runtime evidence.

## 2026-09-15 Round N+33g — N018 batch session management

- Batch session management is code-complete across `b0f39ef9`, `284a0e3b`, and `126be8b9`: row selection no longer navigates, deletion uses the batch API, select-all exposes indeterminate state, source filters and row menus are hidden in batch mode, the action bar is sticky, deletion reloads page one, and visible/ARIA copy is localized for all five locales.
- Focused shell/chat-copy/API tests pass 18/18, 8/8, and 8/8; `typecheck:web`, `build:web`, and `git diff --check` pass. Independent review found no P1/P2/P3.
- N018 still requires authenticated browser/real-backend and Wails/native runtime evidence.

## 2026-09-15 Round N+33h — N018 chat header inline rename

- Chat header renaming now matches Vue across `8a6cb414`, `b07d84de`, `89e80098`, `7c25d66e`, `e9b10457`, and `b22d79c5`: inline title editing, full-title selection, blur/Enter submit with duplicate protection, Escape/cancel focus restoration, visible localized errors, and normalized 80-character titles.
- Focused chat tests pass 26/26; `typecheck:web` and `git diff --check` pass. Independent review found no P1/P2/P3.
- N018 still requires authenticated browser/real-backend and Wails/native runtime evidence.

## 2026-09-15 Round N+33i — N031 mobile multi-file upload queue

- Mobile knowledge-base upload now supports multi-file native picking and FIFO processing across `a08ef44`, `bfe7aa62`, and `ac69f728`; single-file callers remain compatible, failed files do not block later files, cancellation stops the queue, lifecycle events and refreshes are preserved, and nested structured duplicate errors retain backend codes while mapping to five-language copy.
- API client tests pass 10/10; mobile upload queue tests pass 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- N031 remains open for byte-level progress, graph/native runtime evidence, and other mobile acceptance dimensions.

## 2026-09-15 Round N+33j — N031 user-scoped mobile recents

- Mobile recents/favorites now follow the Vue user+tenant namespace across `6ffeb7a5` and `f645ae5d`: stable `auth.me().user.id`, anonymous namespace, refresh-failure reset, atomic generation-checked workspace hydration, and reload on user/tenant changes.
- Focused mobile tests pass 24/24; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- N031 remains open for native runtime evidence and other mobile acceptance dimensions.

## 2026-09-15 Round N+33k — N031 mobile graph localization

- `KnowledgeGraphScreen` now uses the five-language graph surface copy for depth, counts, bounded overview, links, familiar markers, load failures and empty graphs (`eb853ddd`); unknown server node types remain data-driven and no unsupported capability gate was introduced.
- Focused graph tests pass 3/3; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- N031 remains open for native graph runtime evidence and remaining mobile acceptance dimensions.

## 2026-09-15 Round N+33l — N008/N031 mobile document detail preview copy

- Mobile document detail and preview surfaces now use five-language copy across `8bb3d9d1` and `12de9f01`, including actions, metadata, loading/errors, and download-only preview labels; optional labels keep existing ChatScreen callers compatible.
- Focused tests pass 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native file-preview and expiry/no-permission evidence remains open.

## 2026-09-15 Round N+33m — N031 mobile Wiki/FAQ editor localization

- Mobile Wiki/FAQ editor labels, permission/conflict/load/save states, and required-field validation now use typed five-language keys across `0552f5bd` and `e42ed731`, preserving owner/admin gating and existing callers.
- Focused editor tests pass 8/8; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native editor runtime evidence remains open.

## 2026-09-15 Round N+33n — N031 mobile document-list error localization

- `KnowledgeDocumentsScreen` now localizes document loading, filter loading, and upload failure fallback states through five-language knowledge keys (`41de0f18`), preserving queue/pagination/cancellation/refresh behavior.
- Focused tests pass 4/4; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native document-list runtime evidence remains open.

## 2026-09-15 Round N+33o — N031 mobile data-source error localization

- `DataSourcesScreen` now localizes list/editor/test/sync-log failures, permission/cleanup/resume states, connector capability fallback, and default placeholders across five locales (`b0ef4055`), preserving sync, pagination, credential and editor behavior.
- Focused data-source tests pass 23/23 and i18n tests 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native data-source runtime evidence remains open.

## 2026-09-15 Round N+33p — N031 mobile Wiki/FAQ reference localization

- `KnowledgeReferenceScreen` and its reference view model now localize loading/error/empty/version/status/edit labels across five locales (`dbb9e7cb`), preserving slug/numeric-id routes, permissions and pagination.
- Focused reference tests pass 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native reference runtime evidence remains open.

## 2026-09-15 Round N+33q — N032 mobile chat localization

- Mobile `ChatScreen` and `NativeArtifactPreview` now localize session/KB/message/steer/approval/MCP/artifact states and drawer actions across five locales (`0de1d4ed`, `ceb64491`), preserving stream lifecycle and API behavior.
- Focused chat/i18n tests pass 2/2 and artifact-label coverage 5/5; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native chat runtime evidence remains open.

## 2026-09-15 Round N+33r — T10 mobile identity capability localization

- Mobile `IdentityScreen` now localizes title, explanation, loading/error/empty states and supported/unavailable capability statuses across five locales (`5a22da52`), preserving server-provided reasons and avoiding invented capabilities.
- Focused tests pass 3/3; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native management runtime evidence remains open.

## 2026-09-15 Round N+33v — T10 mobile organization localization

- Mobile `OrganizationsScreen` and `shareResourceLabel` now localize organization/member/share/invitation flows, permissions, confirmations, empty/errors and unnamed-resource fallbacks across five locales (`9f65d3fc`, `5919e274`), preserving server roles and resource values.
- Focused organization tests pass 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native organization runtime evidence remains open.

## 2026-09-15 Round N+33u — T10 mobile management capability hub

- `ManagementHubScreen` and `capabilities.ts` now localize capability labels, reasons, modes and required roles across five locales (`9bf93b84`), preserving server-provided reasons, fail-closed projection and route/action behavior.
- Focused tests pass 4/4; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native management runtime evidence remains open.

## 2026-09-15 Round N+33t — T10 mobile configuration localization

- Mobile `ConfigurationScreen` now localizes page/section states, CRUD actions/confirmations, form labels/placeholders, MCP transport/enabled states, loading/errors and partial results through five-language `mobileConfiguration.*` keys (`42789cf7`), preserving API, permissions, credential and read-only behavior.
- Focused tests pass 2/2; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native management runtime evidence remains open.

## 2026-09-15 Round N+33s — T10 mobile API key localization

- Mobile `ApiKeysScreen` now localizes loading/errors, owner-only guidance, create/revoke flows, token states and accessibility labels across five locales (`cada5697`, `7ce141de`, `f41da865`), including distinct Korean revoke/cancel wording.
- Focused tests pass 2/2; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native management runtime evidence remains open.

## 2026-09-15 Round N+33w — T10 mobile administration localization

- Mobile administration now localizes member, invitation and audit surfaces across five locales in `57f62a70` and `a3c85066`; owner/admin/contributor/viewer/system-admin roles and allowlisted audit action/outcome/actor enums use typed keys, unknown server values remain visible, and locale changes invalidate error fallbacks.
- Focused administration coverage passes 6/6; mobile typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Native administration runtime evidence remains open.

## 2026-09-15 Round N+33x — T07 Web model settings utility cleanup

- `ModelSettingsPanel` and `ModelDebugPanel` now use utility classes for tabs, filters, dialogs, fieldsets, result panels, responsive sizing and scrolling (`9fbe4044`); the shared `.wk-model-tabs` rule remains only for `SandboxSettingsPanel`.
- Focused settings coverage passes 27/27; Web full regression passes 890/890, build/typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.
- Browser pixel and protected runtime evidence remain open.

## 2026-09-15 Round N+33y — T07 Web WeKnora Cloud status localization

- `CloudSettingsPanel` now localizes its description, credential save hint and model/status fallback labels across five locales (`c56a411f`), preserving server-returned status values and credential form behavior.
- Settings coverage remains 15/15; Web typecheck/build and `git diff --check` pass. Independent review found no P1/P2/P3.
- Protected runtime evidence remains open.

## 2026-09-15 Round N+33z — R013 Cloud settings browser evidence

- Captured authenticated React Chrome AX evidence for `/platform/settings?section=weknoracloud` in `222de744`; localized Chinese status labels and credential actions were reachable.
- The available Vue tab redirected to `/login`, so this evidence does not prove same-session Vue pixel parity or backend credential success/failure behavior.

## 2026-09-15 Round N+34a — T07 Web Ollama localization

- `OllamaSettingsPanel` now localizes service description, address/model labels, download guidance, progress success feedback and task/model size fallbacks across five locales (`68ed4897`), preserving download, refresh, progress and credential behavior.
- Settings coverage remains 15/15; Web typecheck/build and `git diff --check` pass. Independent review found no P1/P2/P3.
- Protected runtime evidence remains open.

## 2026-09-15 Round N+34b — R013 Ollama browser boundary

- Authenticated React Chrome AX reached `/platform/settings?section=ollama` and showed the localized Chinese title/description plus the server-reported Ollama-unavailable state.
- The local Ollama service was not running, so model/download success paths and same-session Vue comparison remain open.

## 2026-09-15 Round N+34c — T07 Web parser settings copy

- Parser configuration now uses existing localized endpoint and MinerU credential keys in `68c59b8f`; retrieval field mutation semantics remain unchanged.
- Settings coverage remains 15/15; Web typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.

## 2026-09-15 Round N+34d — T07 Web resource settings localization

- Resource settings now localize editor labels, provider placeholders, safe-config copy, security guidance and row fallbacks across five locales (`42d82596`), preserving sensitive config filtering and CRUD/test/default flows.
- Settings coverage remains 15/15; Web typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.

## 2026-09-15 Round N+34e — T07 Web parser credential hint

- Parser settings now localize the API-key safety hint across five locales (`fc5b2377`), preserving endpoint/API-key fields and parser test/save behavior.
- Settings coverage remains 15/15; Web typecheck and build pass. Independent review found no P1/P2/P3.

## 2026-09-15 Round N+34f — R013 parser settings browser boundary

- Authenticated React Chrome AX evidence captured the parser settings route and localized API-key safety hint.
- Successful parser connectivity, backend mutation scenarios and same-session Vue pixel comparison remain open.

## 2026-09-15 Round N+34g — T07 settings section heading localization

- Storage, vector-store and web-search settings headings now use their Vue i18n title/description keys (`f5426f9d`), removing the English inventory fallback from these protected sections.
- Surface tests pass 20/20; Web typecheck and `git diff --check` pass. Independent review found no P1/P2/P3.

## 2026-09-15 Round N+34h — R013 storage heading browser evidence

- Authenticated React Chrome AX confirms localized storage heading, description, editor labels and row fallbacks.
- Same-session Vue pixel comparison remains open.

## 2026-09-15 Round N+34i — R013 vector/web-search heading browser evidence

- Authenticated React Chrome confirms localized vector-store and web-search headings, descriptions and editor labels.
- Same-session Vue pixel comparison remains open.

## 2026-09-15 Round N+34j — R013 retrieval navigation browser evidence

- Authenticated React Chrome confirms the retrieval sidebar label now renders localized `搜索设置`; admin-only visibility and controls remain intact.
- Same-session Vue pixel comparison remains open.

## 2026-09-15 Round N+34k — MCP delete confirmation localization

- `McpSettingsPanel` now uses the shared `mcpSettings.deleteConfirmBody` interpolation for all five locales; permissions, built-in protection and delete lifecycle remain unchanged.
- MCP-focused tests pass 18/18; independent review found no P1/P2/P3. Browser confirmation interaction evidence remains open.

## 2026-09-15 Round N+34l — R013 MCP settings browser evidence

- Authenticated React Chrome confirms localized MCP heading, description and add action. No service row was present to exercise the destructive confirmation dialog; same-session Vue comparison remains open.

## 2026-09-15 Round N+34m — N021 grep tool-result parity

- React grep results now follow Vue chunk grouping, FAQ metadata and title-match aggregation in `20c99de6` and `27ffb52e`; five-locale copy is covered by focused tests.
- Shared tests pass 453/453 and Web tests pass 890/890; independent review found no P1/P2, with two non-blocking P3 coverage notes. Browser fixture evidence remains open.

## 2026-09-15 Round N+34n — shared mobile chat copy contract

- `mobileChat.*` keys are React Native-owned copy and are validated for five-locale completeness separately from the Vue web-chat baseline in `05211164`.
- Shared tests pass 453/453; the independent review found no P1/P2, with a non-blocking P3 noting that the mobile key list is not generated from usage sites.

## 2026-09-15 Round N+34o — N021 WebFetch unknown-link parity

- `WebFetchRenderer` now uses the Vue `chat.unknownLink` copy for missing URLs across five locales in `d53998a9`.
- Shared tests pass 454/454, Web tests pass 890/890, and Web typecheck/build pass. Independent review found no P1/P2; one non-blocking P3 notes that only one non-Chinese renderer locale is directly exercised. Browser fixture evidence remains open.

## 2026-09-15 Round N+34p — N021 WebFetch summary error precedence

- `WebFetchRenderer` now hides summary error fallback fields when a non-empty summary exists, while retaining real fetch errors, matching Vue `v-if / v-else-if` behavior in `5ae7365b`.
- Shared tests pass 457/457 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2 and one non-blocking P3 for a missing mixed real-error fixture. Browser fixture evidence remains open.

## 2026-09-15 Round N+34q — N021 ChunkDetail full-content label

- `ChunkDetailRenderer` now uses Vue `chat.fullContentLabel` across five locales in `b72e0946`; default renderer calls remain compatible.
- Shared tests pass 458/458 and Web tests pass 890/890; Web typecheck and build pass. Independent review found no P1/P2/P3. Browser fixture evidence remains open.

## 2026-09-15 Round N+34r — N021 ChunkDetail chunk-ID label

- The shared utility batch `085de06e` adds Vue `chat.chunkIdLabel` copy to ChunkDetailRenderer for all five locales while preserving default renderer compatibility.
- Focused renderer/copy tests pass (34/34 and 11/11); current shared/Web regression remains green at 458/458 and 890/890. Browser fixture evidence remains open.

## 2026-09-15 Round N+34s — N021 ChunkDetail position label

- ChunkDetailRenderer now uses Vue `chat.positionLabel` across five locales in `873589a6`; default copy propagation remains compatible.
- Shared tests pass 460/460 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note. Browser fixture evidence remains open.

## 2026-09-15 Round N+34t — N021 ChunkDetail document-ID label

- ChunkDetailRenderer now uses Vue `chat.documentIdLabel` across five locales in `7561cda4`; default copy propagation remains compatible.
- Shared tests pass 461/461 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note. Browser fixture evidence remains open.

## 2026-09-15 Round N+34u — N021 ChunkDetail content length

- ChunkDetailRenderer now uses Vue `chat.contentLengthLabelSimple` and `chat.lengthChars` across five locales in `2d616875`; numeric length and null behavior remain unchanged.
- Shared tests pass 462/462 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2, with one non-blocking end-to-end copy-propagation P3 note. Browser fixture evidence remains open.

## 2026-09-15 Round N+34w — N021 RelatedChunks empty state

- RelatedChunksRenderer now uses Vue `chat.noRelatedChunks` for empty objects and empty arrays across five locales in `2c091387`; default copy propagation remains compatible.
- Shared tests pass 463/463 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2/P3. Browser fixture evidence remains open.

## 2026-09-15 Round N+34x — N021 knowledge-base count

- KnowledgeBaseListRenderer now uses Vue `chat.knowledgeBaseCount` interpolation across five locales in `77f4988b`; count fallback and default copy propagation remain unchanged.
- Shared tests pass 464/464 and Web tests pass 890/890; Web typecheck passes. Independent review found no P1/P2/P3. Browser fixture evidence remains open.

## 2026-09-15 Round N+34v — N021 ChunkDetail field-label completion

- ChunkDetail now uses Vue-aligned five-locale labels for chunk ID, document ID, position, and content length across `085de06e`, `873589a6`, `7561cda4`, and `2d616875`.
- Shared tests pass 462/462, Web tests pass 890/890, Web typecheck/build pass, and independent reviews found no P1/P2. Browser fixture evidence remains open.

## 2026-09-15 Round N+34y — N021 ChatRoutePage fallback copy

- 删除会话确认与知识库 mention 加载失败兜底改用 Vue 对齐的共享文案，覆盖五种语言；实现提交 `e750a8a7`。
- ChatRoutePage 相关回归通过；共享测试维持 464/464。浏览器删除确认和真实 mention API 错误路径仍需运行时证据。

## 2026-09-15 Round N+34z — N021 artifact preview copy

- 对话产物预览的类型标签、返回列表、下载和加载状态改用共享五语言文案；`MessageList` 将当前 locale copy 传入 `ArtifactPreview`，默认英文调用保持兼容。
- 专项产物预览 3/3、shared typecheck 通过；shared 全量 465/465。真实产物下载/预览浏览器与后端证据仍待补齐。

## 2026-09-15 Round N+34aa — shared chat copy mobile regression

- 产物预览文案扩展未破坏 React Native 共享包；mobile 测试 187/187、mobile typecheck 通过。

## 2026-09-15 Round N+34ab — React 新对话运行时复核

- 已认证 React `http://localhost:5181/platform/creatChat` 运行时 AX 检查：侧栏、新对话入口、智能体选择、上传附件、知识库按钮、模型状态和发送按钮均可达；中文文案未泄漏新增产物预览英文。
- `/platform/chat` 不是有效 React 路由并显示页面不存在，已记录为兼容入口差异；未将其误判为聊天页验收。Vue `:5173` 当前未认证，双端同条件对照仍缺证据。

## 2026-09-15 Round N+34ac — legacy flat chat route compatibility

- 运行时发现 `/platform/chat` 会误落到 NotFound；新增到 `/platform/creatChat` 的兼容解析与查询参数保留，保持旧入口可用并复用当前聊天页面。
- 路由专项 11/11、Web typecheck、diff check 通过；真实浏览器重载验证待下一轮补充。

### 运行时补充

- Chrome 已认证 React 运行时访问 `/platform/chat?agentId=a`，页面成功加载新对话并规范化地址为 `/platform/creatChat`，AX 中可见智能体选择、上传附件、知识库和发送控件；该证据覆盖旧入口重定向，不替代 Vue 同账号对照。

## 2026-09-15 Round N+34ad — N021 reference list copy

- 对话引用来源面板的标题、网页/文档/工具分组和引用片段 aria 标签改用当前 locale 的共享文案；默认解析保持跟随运行时语言。
- ReferenceList/chat-copy 专项 13/13、shared typecheck、diff check 通过；真实引用数据与 Vue 同条件浏览器对照仍待补齐。

## 2026-09-15 Round N+34ae — N021 tool approval copy

- 工具审批卡片标题、查看参数、同意、拒绝和已处理状态改用当前 locale 共享文案，并由 ChatPage 透传；旧的审批解析/API 行为保持不变。
- 审批与共享 copy 专项 17/17、Web 全量 891/891、shared typecheck 通过；真实 MCP 审批流和浏览器多语言对照仍待补齐。

## 2026-09-15 Round N+34af — N021 MCP OAuth action copy

- 对话操作区的 MCP 授权标题、工具标签、授权/取消按钮、已授权状态和区域标题接入五语言共享文案，并保留服务名称显示。
- Web 全量 891/891、shared copy/typecheck 通过；真实 OAuth 授权回调和浏览器多语言对照仍待补齐。

## 2026-09-15 Round N+34ag — N021 tool-call surface copy

- 新对话流中的 Tool calls 标题、文档信息 ID、元数据标题改用共享五语言文案，减少工具结果区残留英文；数据字段和状态值保持后端原值。
- Web 全量 891/891、shared/Web typecheck、diff check 通过；完整工具结果多语言浏览器与真实数据验收仍待补齐。

## 2026-09-15 Round N+34ah — N021 approval validation copy

- 工具审批 JSON 编辑器的无效 JSON 与非对象参数错误改用共享五语言文案；解析函数保留无 copy 参数时的原有英文默认行为。
- 审批专项 17/17、Web 全量 891/891、shared typecheck 通过；真实错误交互浏览器证据仍待补齐。

## 2026-09-15 Round N+34ai — React 平台主路由运行时巡检

- 已认证 Chrome 逐页检查 `/platform/knowledge-bases`、`/platform/agents`、`/platform/organizations`：侧栏分组、筛选入口、创建按钮、空态和计数均可达，中文 AX 文案无新增英文泄漏。
- 知识库页显示四个范围筛选与空态；智能体页显示四个内置智能体及管理入口；共享空间页显示全部/创建/加入筛选和双入口空态。
- 该轮只证明 React 运行时可达性和空态结构；Vue 同账号对照、真实变更链路、computed-style 与桌面/Embed 证据仍缺失。

## 2026-09-15 Round N+34aj — Desktop/Embed shared-layer regression

- 共享聊天文案与旧聊天入口兼容修复后，Desktop Renderer 测试 2/2、Embed 测试 7/7、Desktop/Embed typecheck 均通过。
- 该结果只证明共享契约未破坏桌面/嵌入端静态边界；Wails 实际窗口、Embed 宿主页面和真实渠道链路仍需运行时验收。

## 2026-09-15 Round N+34ak — Mobile native/build evidence

- iOS Simulator 原生编译：`xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator ... CODE_SIGNING_ALLOWED=NO build` 成功（`BUILD SUCCEEDED`）。仅证明原生编译，不代表模拟器启动和交互验收。
- Android `./gradlew assembleDebug --no-daemon` 被环境阻断：未配置 Android SDK（缺少 `ANDROID_HOME` 或 `android/local.properties`）。
- Expo Web export 被依赖阻断：项目未安装 `react-dom` 与 `react-native-web`，未修改依赖以绕过验收。

## 2026-09-15 Round N+34al — Android native build recovery

- 检查发现本机存在 `/Users/wuyongjun/Library/Android/sdk`；使用显式 `ANDROID_HOME`/`ANDROID_SDK_ROOT` 重跑 `./gradlew assembleDebug --no-daemon`，Android Debug 原生编译成功（`BUILD SUCCESSFUL`，531 tasks）。
- `adb devices` 当前无已启动设备，因此未宣称 Android 安装/启动/交互验收；Expo Web export 仍因缺少 web 依赖保持 blocked-env。

## 2026-09-15 Round N+34am — Android/iOS device launch evidence

- Android AVD `test36-small` 已启动（`emulator-5554`），Debug APK 安装成功，`adb shell monkey -p com.weknora.mobile 1` 后顶层 Activity 为 `com.weknora.mobile/expo.modules.devlauncher.launcher.DevLauncherActivity`。
- iOS Simulator `iPhone 17` 已 boot 完成，安装 `/tmp/weknora-react-ios/Build/Products/Debug-iphonesimulator/WeKnora.app` 成功，`simctl launch com.weknora.mobile` 返回进程号 `43969`。
- 该证据证明原生包可安装并启动；尚未证明登录、聊天、上传等设备内业务交互，也不替代 Vue 同条件视觉对照。

## 2026-09-15 Round N+34an — 原生启动界面读取

- Android UIAutomator 确认当前包名为 `com.weknora.mobile`，但界面由 Expo Dev Launcher 承载；iOS 截图同样显示 `WeKnora Development Build`、`No development servers found` 与 `Enter URL manually`。
- 因未连接 Metro/dev server，本轮不能宣称已进入业务登录或聊天页面；原生启动层已验证，业务交互证据仍待连接可用开发服务后补齐。

## 2026-09-15 Round N+34ao — Android Metro 业务页面启动

- Metro `http://localhost:8081` 启动并返回 `packager-status:running`；Android AVD 通过 `exp://10.0.2.2:8081` 连接后，成功从 Dev Launcher 进入 React Native WeKnora 登录页。
- 截图：`evidence/vue-react-parity/screenshots/native-20260915/android-login.png`；可见 WeKnora、Sign in、Email、Password、SSO、Create account、Join with invitation、Change server。
- 该证据覆盖 Android 业务首屏启动和 Metro 连接；尚未执行登录、工作空间切换、聊天发送、上传或与 Vue 同条件视觉对照。

## 2026-09-15 Round N+34ap — iOS scheme/Metro 连接尝试

- iOS Simulator 已启动 Metro 后执行 `simctl openurl weknora://expo-development-client/?url=http://127.0.0.1:8081`，系统弹出“在 WeKnora 中打开？”确认框，证明 `weknora` URL scheme 已注册并可被系统识别。
- 截图：`evidence/vue-react-parity/screenshots/native-20260915/ios-scheme-prompt.png`。
- 当前自动化环境无法可靠点击系统确认框，因此未宣称 iOS 已进入业务登录页；Android 已有同等级业务首屏证据。

## 2026-09-15 Round N+34aq — Settings resource failure copy

- Cloud status refresh and resource list/default-resource failure fallbacks now have explicit five-locale copy, removing the remaining migration TODO markers in these settings panels.
- Validation: `pnpm run test:web` 891/891, `pnpm run typecheck:web`, `git diff --check` passed.
- This round covers static copy behavior only; live backend failure injection and browser interaction evidence for settings mutations remain pending.

## 2026-09-15 Round N+34ar — Mobile knowledge upload byte progress

- API client multipart requests now carry an optional `onProgress` callback through the native transport seam. The mobile knowledge upload queue converts byte counts to Vue-parity progress events, and the document screen wires the callback for each selected file.
- Validation: upload queue 6/6, shared suite 465/465, mobile typecheck passed; `git diff --check` passed.
- N031 no longer lists byte-level upload progress, user-scoped recents, or mobile English fallback as open implementation gaps. Graph visuals/capability gating and authenticated iOS/Android knowledge interaction evidence remain open.

## 2026-09-15 Round N+34as — Mobile Wiki graph preview

- Added a dependency-free native graph preview with deterministic node placement, center-node emphasis, localized neighbor expansion, and a bounded 40-node visual surface; the existing detailed list remains available for accessibility and metadata.
- Validation: mobile suite 189/189, mobile typecheck, `git diff --check` passed.
- N031 graph visuals now have an implemented native preview; authenticated device graph data and full Vue visual/computed-style evidence remain open.

## 2026-09-15 Round N+34at — Cross-package regression after mobile slices

- Web 891/891, Embed 7/7, Desktop 2/2, Shared 465/465, Mobile 189/189 all pass after the upload-progress and native graph changes.
- Web, Shared, Mobile, Embed, and Desktop TypeScript checks pass; `git diff --check` passes.
- Runtime acceptance remains separate: authenticated device knowledge interactions, Wails feature interactions, and live graph/upload backend evidence are still not proven by these suites.

## 2026-09-15 Round N+34au — Wiki graph live capability gate

- Authenticated React Web runtime reached the real KB graph route and received the backend disabled-feature response. The page rendered localized heading/help, refresh/fit controls, backend error alert, and retry action.
- Evidence: `evidence/vue-react-parity/2026-09-15-r020-wiki-graph-runtime-gating.md`.
- This closes the live negative capability-gate check for the Web route; a successful graph payload and native-device graph interaction remain unverified.

## 2026-09-15 Round N+34av — Knowledge-base context label cleanup

- Replaced the remaining English `Knowledge base · <id>` context eyebrow in Data Sources, Wiki, Graph, and Knowledge Settings pages with the shared localized `common.knowledgeBases` key.
- Validation: Web 891/891 and Web typecheck passed; `git diff --check` passed.

## 2026-09-15 Round N+34aw — Document detail preview localization

- Replaced direct English document loading, metadata, preview, retry, and download labels with five-locale copy in `KnowledgeDocumentDetailPage`.
- Validation: Web 891/891, Web typecheck, and `git diff --check` passed.

## 2026-09-15 Round N+34ax — Skill settings viewer copy

- Replaced the viewer-only Skill settings block's direct English title, description fallback, and empty state with shared five-locale settings keys; the read-only behavior is unchanged.
- Validation: focused SkillSettings tests 31/31, Web 891/891, Web typecheck, and `git diff --check` passed.

## 2026-09-15 Round N+34ay — Environment variable panel localization

- Replaced EnvVar settings direct English labels, empty state, operation fallbacks, and action text with existing five-locale `envVarSettings.*` keys.
- Validation: Web 891/891, Web typecheck, and `git diff --check` passed.

## 2026-09-15 Round R319 — Platform navigation item geometry

- Matched the expanded React platform navigation items to Vue `menu.vue`: 38px row height, 14px left inset, 4px radius, 8px vertical padding, and 600-weight 14px labels.
- Computed-style comparison identified the original mismatch; focused Shell 30/30, Web 911/911, Web typecheck, and diff check pass. Post-edit authenticated screenshot re-capture is still open because the independent browser session rehydrated to login.
- Evidence: `evidence/vue-react-parity/2026-09-15-r319-shell-nav-geometry.md`.

## 2026-09-15 Round N+34az — Platform navigation icon geometry

- Replaced React platform-shell generic chat/book/bot/users paths with the corresponding Vue `prefixIcon.svg`, `zhishiku.svg`, `agent.svg`, and `organization.svg` geometry, while preserving navigation semantics and active-state behavior.
- Runtime paired observation on the authenticated `Parity KB Demo` fixture at approximately 1355x720 confirmed the KB list layout/card/state parity and the updated rail icon shapes.
- Validation: focused shell tests 30/30, Web 911/911, Web typecheck, and `git diff --check` passed. See `evidence/vue-react-parity/2026-09-15-r313-platform-nav-icon-assets.md`.

## 2026-09-15 Round R327 — Authenticated backend runtime blocked-env

- Attempted to start the existing Docker `app` service for same-condition Vue/React browser verification. Docker began pulling `wechatopenai/weknora-app:latest`, but no `app` container was running when checked; `http://localhost:8080/health` was unreachable.
- Evidence: `evidence/vue-react-parity/2026-09-15-r327-backend-runtime-blocked-env.md`.
- Classification: `blocked-env`. This does not reduce the remaining authenticated backend, permission, Wails, native-device, or full visual-comparison gaps in the parity matrix.

## 2026-09-15 Round R328 — Live backend login negative paired capture

- The Docker `app` service was recovered with container-safe `REDIS_ADDR=redis:6379` and `DB_HOST=postgres` overrides; `/health` returned `{"status":"ok"}`.
- Fresh Vue and React browser contexts at 1440×900/zh-CN both reached the real auth endpoints and received HTTP 401 for the recorded parity account, remaining on `/login` with localized invalid-credential copy.
- Screenshots: `artifacts/parity-20260915/vue-backend-auth.png`, `artifacts/parity-20260915/react-backend-auth.png`.
- Evidence: `evidence/vue-react-parity/2026-09-15-r328-backend-login-negative-paired.md`. This closes only the live negative auth capture; authenticated protected-page parity remains open because the account is absent in the attached database.

## 2026-09-15 Round R329 — Settings default deep-link normalization

- Live paired capture found Vue normalizes direct `/platform/settings` to `?section=general`, while React retained the bare path. React now performs the same history-preserving normalization on mount without changing section behavior.
- Focused Settings tests 16/16, Web typecheck, and `git diff --check` passed.

## 2026-09-15 Round R330 — Guide transition recheck

- Rechecked with explicit visible-button selection: both Vue and React initially show the seven-step global guide and, after `跳过引导`, both show the one-step empty-knowledge-base contextual guide.
- The previous apparent guide mismatch was a transition timing/selector artifact; no guide code change was required. Evidence: `evidence/vue-react-parity/2026-09-15-r330-guide-transition-verification.md`.

## 2026-09-15 Round R331 — Settings default deep-link live verification

- Fresh authenticated Vue/React browser contexts both resolve direct `/platform/settings` to `/platform/settings?section=general`.
- Evidence: `evidence/vue-react-parity/2026-09-15-r331-settings-deeplink-live.md`.

## 2026-09-15 Round R332 — Shared page background token correction

- Live computed-style comparison on the authenticated Agents page found Vue body background `rgb(238, 238, 238)` while React inherited transparent background.
- React shared stylesheet now maps `body` to the semantic `--wk-app-background` token with Vue-compatible `#eee` fallback.
- Browser computed style, Web typecheck, and `git diff --check` passed; full visual and responsive regression remains open.

## 2026-09-15 Round R333 — Embed upload labels localization

- Replaced hard-coded React Embed `Attach`/`Image` labels with Vue-derived localized upload labels for all five supported locales, including accessible labels.
- Embed tests 9/9, Web typecheck, and `git diff --check` passed. Real channel upload success/failure and browser visual evidence remain open.

## 2026-09-15 Round R334 — Embed status fallback localization

- Localized the remaining Embed fallback UI strings for assistant subtitle, loading state, and session-start failure while preserving server-provided error details.
- Embed tests 10/10, Web typecheck, and `git diff --check` passed. Real channel success/failure browser evidence remains open.

## 2026-09-15 Round R335 — Embed message error prefix localization

- Replaced the remaining hard-coded `Error:` message prefix with Vue-aligned locale-aware labels for all five supported locales.
- Embed tests 11/11, Web typecheck, and `git diff --check` passed. Real channel error rendering remains open for browser verification.

## 2026-09-15 Round R336 — Knowledge-base create form type label

- Live Vue/React create-form comparison found React still rendered the raw English `Type` label while Vue used the localized knowledge-editor type label.
- React now uses `knowledgeEditor.basic.typeLabel`; focused KB-list tests 18/18, Web typecheck, and `git diff --check` passed.
- Real create submission success/failure and full 13-step browser flow remain open.

## 2026-09-15 Round R337 — Knowledge-base create form field localization

- Replaced remaining raw create-form labels/placeholders (`Name`, `Description`, embedding/summary model IDs) with Vue-derived `knowledgeEditor.basic/models` locale keys.
- Web full tests 937/937, Web typecheck, and `git diff --check` passed. Real create submit and full step-flow evidence remain open.

## 2026-09-15 Round R338 — Knowledge-base create labels live recheck

- Authenticated React live recheck confirms the create form renders Vue-derived Chinese labels and placeholders for name, type, description, Embedding, and LLM fields.
- Evidence: `evidence/vue-react-parity/2026-09-15-r338-kb-create-labels-live.md`. Submit success/failure and full guided flow remain open.

## 2026-09-15 Round R339 — Knowledge-base create structure audit

- Real browser inspection confirms the repaired React create form copy is localized, but the Vue create surface still exposes a multi-section wizard while React's underlying dialog remains a compact basic-fields form behind the shared contextual guide.
- This is recorded as an open structural parity item; no acceptance claim is made from the matching labels alone. Full section visibility, model selection, validation, submit states, and responsive behavior remain to be implemented/verified.

## 2026-09-15 Round R340 — Knowledge-base create indexing strategy contract

- Added the Vue-aligned document indexing strategy section to the React create/edit dialog using the project `Checkbox` wrapper and shared Vue-derived locale keys for RAG, Wiki, and knowledge graph options.
- Create defaults and edit hydration now preserve the backend `indexing_strategy` shape; document saves submit it at the top level, FAQ saves omit it. React also blocks document saves with no strategy and blocks RAG saves without an Embedding model, matching Vue validation semantics.
- Validation: Web typecheck, Web full tests 937/937, and `git diff --check` passed. Browser submit/locked-edit/full multi-section wizard evidence remains open; this does not close R339's structural gap.

## 2026-09-15 Round R341 — Knowledge-base editor section scaffold and shared primitive recovery

- React knowledge-base create/edit now exposes a Vue-derived section rail for Basic Information and Model Configuration, moves model fields into the model section, keeps the indexing strategy in the basic section, disables type changes during edit, and preserves the existing save payload/validation contract.
- Shared Button/Input/Select/Tab/Dropdown styling and semantic tokens were aligned in the project UI layer. The Dialog wrapper remains the project-owned focus/Escape/outside-close implementation after a Radix Portal experiment broke the current DOM test harness (`getComputedStyle`/`MutationObserver` unavailable); this keeps shadcn/Radix as an implementation substrate without leaking runtime assumptions into existing tests.
- Validation: KB anatomy 18/18, route/auth/UI focused tests 15/15, Web typecheck and `git diff --check` passed. Full Web suite is not yet green after concurrent onboarding/settings changes; remaining failures are implementation/test-environment findings, not accepted parity.
- Vue full editor sections (parser, storage, chunking, multimodal, ASR, graph, advanced, data source, share, activity), same-condition browser screenshots, real backend mutations, Wails and native acceptance remain open.

## 2026-09-15 Round R342 — Parallel state, artifact, document and primitive regression batch

- Parallel scoped repairs covered data-source forbidden/auth/not-found states and sync-log loading/retry/pagination deduplication; document/Wiki/FAQ localized operation feedback; Chat artifact drawer preview/list behavior with Esc/overlay close and persisted resizing; and shared Vue-derived button/input/select/tab/dropdown states.
- `Dialog` and `Sheet` remain project-owned shadcn-compatible wrappers with explicit Escape, outside-close, focus restoration and keyboard focus cycling. A Radix Portal experiment was rejected after the existing DOM test host failed on missing `getComputedStyle`/`MutationObserver`; no stock Radix appearance is used as a substitute for Vue styling.
- Validation on the integrated dirty worktree: Web tests 950/950; route/auth/UI focused tests 15/15; KB anatomy 18/18; artifact/chat focused 152/152; document/Wiki/FAQ focused 60/60; data-source focused 33/33; shared typecheck passed; Web typecheck passed; desktop typecheck and tests 2/2 passed; Embed typecheck/tests 11/11 passed; `git diff --check` passed.
- Evidence boundaries: these are static/unit/DOM-host checks. No new authenticated real-backend mutation, same-condition Vue/React screenshot/computed-style matrix, Wails window interaction, provider success path, or native business-flow evidence was produced in this round.

## 2026-09-15 Round R343 — Vue editor topology contract and chat run-state repair

- Added a pure, tested React knowledge-editor section topology contract copied from `KnowledgeBaseEditorModal.vue`: group order, document-vs-FAQ visibility, create-vs-edit management sections, and stale-section fallback are now explicit in `apps/web/src/knowledge-bases/editor-sections.ts`.
- A parallel chat pass added the localized running-session indicator and regression coverage across the supported locales; it did not claim browser, backend, Wails, or native acceptance.
- Focused topology test: 3/3. Existing integrated evidence remains unchanged: static/unit/DOM-host checks only. Full editor section controls and save payloads, paired authenticated browser evidence, real backend mutations, Wails interaction, provider success, and native business flows remain open.

## 2026-09-15 Round R344 — Integrated regression and platform typechecks

- Integrated additional Chat approval-resolution and clear-confirmation seams plus localized running-session feedback. The Web suite now passes 958/958.
- `pnpm run typecheck:web`, `pnpm run typecheck:shared`, `pnpm run typecheck:desktop`, and `git diff --check` pass on the dirty worktree.
- This round still does not close parity acceptance: the Vue knowledge editor remains wider than the React implemented surface, and authenticated browser, real backend mutation, Wails, provider-success, and native business-flow evidence are absent.

## 2026-09-15 Round R345 — Knowledge editor configuration contract and section controls

- Added `apps/web/src/knowledge-bases/editor-config.ts` with Vue-derived defaults, hydration, and snake_case payload mapping for FAQ, chunking, parser rules, vector/storage bindings, multimodal, ASR, graph extraction, question generation, auto-tagging, and Wiki settings.
- The React KB dialog now exposes real Tailwind/project-component controls for FAQ, chunking, multimodal, ASR, graph, advanced, vector-store, storage, and parser sections, with type-aware visibility and validation for FAQ index mode and multimodal model selection.
- Focused editor model tests: 5/5; integrated Web suite: 960/960; Web/shared/desktop typechecks and `git diff --check` pass.
- Remaining open items are explicit: datasource/share/activity still route to the existing settings surface, parser/vector/storage selection is not yet backed by live option loading in the modal, and no authenticated paired browser or mutation evidence exists.

## 2026-09-15 Round R346 — Build and regression confirmation after editor expansion

- Production Web build passed after adding the editor configuration controls; Vite emitted only existing large-chunk warnings.
- Web tests remain 960/960, and Web/shared/desktop typechecks remain green.
- The expanded editor is still not accepted as complete without live option loading for parser/vector/storage, full datasource/share/activity modal behavior, authenticated browser comparison, and real backend mutation evidence.

## 2026-09-15 Round R347 — Live editor option loading

- The React editor now requests parser engines, storage backends, and vector stores through the existing authenticated client settings API when the modal opens. Selectors render Vue-derived default/configured options and preserve edit-time immutability; all three endpoints degrade to the localized data-load error state independently.
- Web typecheck: passed; Web tests: 960/960; production Web build: passed with existing large-chunk warnings.
- Remaining evidence boundary: no authenticated browser capture or successful/failed real mutation has been executed in this round; datasource/share/activity still use the existing settings handoff rather than full in-modal Vue behavior.

## 2026-09-15 Round R348 — Data source, share, and activity editor surfaces

- The KB editor now mounts the existing project DataSourcesPage for the Vue `datasource` section, opens the project share dialog from the `share` section, and loads/renders activity entries through the authenticated KB activity API with loading/empty/retry states.
- Web typecheck: passed; Web tests: 960/960; production Web build: passed with existing large-chunk warnings.
- Remaining gap is now primarily visual/behavioral evidence: same-condition authenticated browser screenshots/computed styles, provider-backed data-source/share mutations, and Wails/native acceptance are still absent.

## 2026-09-15 Round R349 — Login visual discrepancy fixed with live browser evidence

- Same-window Vue/React login screenshots and accessibility trees at `:5180`/`:5181` exposed one concrete React discrepancy: the Vue password input had a trailing visibility eye while React did not.
- React now uses a project-styled Tailwind control with keyboard-accessible toggle behavior; the control is covered by the focused login test and rechecked live in Chrome.
- Evidence: `evidence/vue-react-parity/2026-09-15-r349-login-password-toggle-live.md`.
- This closes only the unauthenticated login-surface discrepancy. Authenticated page parity, real mutations, Wails/native validation and remaining page-by-page matrix items remain open.

## 2026-09-15 Round R350 — Knowledge-base settings overlay/navigation live repair

- Real authenticated Vue/React comparison found that the list-card settings action still navigated React to a full-page settings route while Vue opens an in-place blurred editor overlay.
- React list settings and uninitialized-card settings now open the existing project-owned `Dialog` editor in place; edit title/close semantics and section-rail native-button resets were aligned with the Vue baseline. The historical deep-link route remains available.
- Evidence: `evidence/vue-react-parity/2026-09-15-r350-kb-settings-overlay-live.md`; Vue and React AX trees plus live screenshots were captured in the same Chrome session with the local parity fixture.
- Focused KB anatomy tests: 18/18; `git diff --check`: passed. Save success/failure, datasource/share provider mutations, Wails, and native acceptance remain open.

## 2026-09-15 Round R351 — Document processing card copy live repair

- Same-condition Vue/React document detail inspection found React used the filter label `处理中` while Vue's document card uses the in-flight card copy `解析中...` for pending/processing and has a separate finalizing-summary branch.
- React `documentStatus` now follows the Vue `DocumentCardView.vue` branch; focused document chrome tests are 17/17 and live React AX output matched Vue `解析中...` for the same processing document.
- Evidence: `evidence/vue-react-parity/2026-09-15-r351-document-processing-copy-live.md`. Upload/parse mutation, Wails, and native acceptance remain open.
- The same repair also replaces the generic warning paragraph with a Vue-style Tailwind spinner/trace action on in-flight cards; focused page-chrome coverage is 18/18.

## 2026-09-15 Round R352 — Page-level URL import dialog live repair

- Authenticated Vue/React browser comparison found that React's page-level
  `添加文档 → 导入网页` callback set `sourceUrlDialogOpen`, but the dialog JSX
  was incorrectly nested under the staged-file `uploadDialogOpen` branch and
  therefore did not render.
- Moved the React URL dialog to the page-level render branch. The staged-file
  upload confirmation remains separate; the project Tailwind/shadcn-compatible
  Dialog/Input wrappers and empty-URL behavior are preserved.
- Vue and React live AX evidence both show the URL label, placeholder, helper
  text, and cancel/confirm controls. Empty confirmation keeps the dialog open
  without a backend write.
- Evidence: `evidence/vue-react-parity/2026-09-15-r352-page-url-import-dialog-live.md`.
- Focused upload/document tests: 37/37; Web typecheck and `git diff --check`
  passed. This closes only the page-level URL-dialog rendering gap; valid URL
  import, upload mutation, Wails, native, provider, and production acceptance
  remain open.

## 2026-09-15 Round R353 — Knowledge-base settings basic section live repair

- Same-condition authenticated Vue/React inspection found three edit-state
  differences in the in-place KB settings dialog: React omitted the edit-only
  knowledge-base ID/API hint, had no Vue `34/200` description counter, and
  rendered `保存修改` instead of Vue `保存并关闭`.
- React `apps/web/src/App.tsx` now renders the ID block, enforces the Vue
  200-character description limit with a live counter, and uses the localized
  `knowledgeEditor.buttons.saveAndClose` edit action.
- Vue and React AX trees now show the same basic-information controls and
  footer copy in the local fixture. Evidence:
  `evidence/vue-react-parity/2026-09-15-r353-kb-settings-basic-live.md`.
- An unchanged fixture save was also submitted through the authenticated local
  backend; React closed the dialog and reloaded the list on the success path.
  Save failure/rollback, full visual computed-style parity, Wails/native, and
  remaining settings sections remain open.

## 2026-09-15 Round R354 — Knowledge-base share management inline repair

- Vue `KBShareSettings.vue` renders share management directly inside the KB
  settings editor. React previously exposed only a heading/description and a
  button that opened a separate dialog.
- React now mounts the project share component in inline mode with the Vue
  empty/list surface, search, add-share flow, permission updates, and unshare
  actions. Existing standalone dialog behavior remains covered.
- Authenticated Vue/React AX comparison matched the title, description,
  search field, share action, zero count, and empty state. Evidence:
  `evidence/vue-react-parity/2026-09-15-r354-kb-share-inline-live.md`.
- Share-dialog tests: 13/13. Provider mutation failure/success, Wails/native,
  and production acceptance remain open.

## 2026-09-15 Round R355 — Knowledge-base activity settings panel repair

- Vue activity settings expose action/outcome filters, refresh/error/empty
  states, cursor pagination, a table, and a right-side detail drawer. React's
  prior surface was only a short unfiltered list.
- Added `KnowledgeBaseActivityPanel` with the Vue-derived table/filter/load
  states, keyboard row activation, and project Sheet detail drawer. Extended
  the API client activity query to preserve cursor compatibility while sending
  `action`, `outcome`, and `limit`.
- Authenticated browser AX showed four real backend records, the filter/table
  surface, load-more control, and the activity detail drawer. Evidence:
  `evidence/vue-react-parity/2026-09-15-r355-kb-activity-live.md`.
- API settings tests: 4/4; Web typecheck and diff-check passed. Full Web
  regression/build and provider/Wails/native acceptance remain separate gates.

## 2026-09-15 Round R356 — Knowledge-base data-source empty surface repair

- Vue `DataSourceSettings.vue` keeps the section header free of actions and,
  for a manager with no configured sources, renders the add action as a dashed
  card inside the source grid. React previously rendered `暂无数据源` plus a
  header button, which was a real empty-state and placement mismatch.
- React `DataSourcesPage` now uses the Vue-derived section spacing and places
  `添加数据源` in a Tailwind-styled dashed card with the Vue border, radius,
  hover, focus, and icon treatment. Existing create/edit, API, and permission
  behavior is unchanged.
- Authenticated Vue/React AX comparison on the local fixture now shows the
  same title, description, and inline add-card semantics; the fixture has no
  data sources, so source-card and editor states remain separate follow-up
  evidence. Evidence:
  `evidence/vue-react-parity/2026-09-15-r356-kb-datasource-empty-live.md`.
- Web regression: 963/963 passed; Web typecheck and `git diff --check` passed.
  Non-empty source-card visual/action parity, connector backends, Wails/native,
  and production acceptance remain open.

## 2026-09-15 Round R357 — Knowledge-base data-source create wizard entry repair

- Live Vue/React comparison after activating `添加数据源` showed that Vue
  enters a `选择类型` wizard step, while React immediately exposed the whole
  generic connector form.
- React `DataSourcesPage` now keeps create mode in a type-selection step and
  transitions to the existing form only after a connector is chosen. The
  project Button/Card primitives and Tailwind focus/hover treatment are used;
  edit mode remains directly on the edit form.
- Browser AX now exposes the Vue-shaped `选择类型` heading, close action, and
  selectable connector cards. Evidence:
  `evidence/vue-react-parity/2026-09-15-r357-kb-datasource-create-step-live.md`.
- Web typecheck and `git diff --check` passed. Connector filtering/iconography,
  later wizard steps, backend mutations, Wails/native, and production
  acceptance remain open.

## 2026-09-15 Round R358 — Knowledge-base connector catalogue parity

- Vue `DataSourceEditorDialog.vue` defines nine currently available create
  connectors in a fixed order: Feishu, Lark, Feishu Drive, Lark Drive, Notion,
  Yuque, Tencent IMA, RSS/Atom, and GitLab. React had rendered every backend
  type, including connectors absent from the Vue wizard.
- React now filters and orders the create cards from that Vue catalogue and
  resolves the existing localized connector names/descriptions. The API still
  retains the full server type response for edit/compatibility paths.
- Live React AX now matches Vue's nine-card catalogue, order, and descriptions.
  Evidence remains grouped with the create-step evidence:
  `evidence/vue-react-parity/2026-09-15-r357-kb-datasource-create-step-live.md`.
- Web typecheck and `git diff --check` passed. Icons, connector-specific
  credential fields, resource selection, strategy step, and mutation evidence
  remain open.

## 2026-09-15 Round R361 — Deep-link and hidden app route repair

- Added the four Vue app routes (`/platform/apps`, connections, authorization,
  and actions) to the React route table, shell dispatch, route tests, and parity
  CSV. The initial pages use project Tailwind/shadcn primitives and preserve
  loading, empty, error, role, polling, approval, execution, and revoke states.
- Preserved Vue-compatible deep-link behavior for `knowledge_id` document
  entries and both `agentId`/`agent_id` chat and integration query aliases.
  Document opening occurs only after the matching list response is available.
- Static checks passed: `pnpm test:shared` (485/485), `pnpm test:web`
  (1038/1038), `pnpm typecheck:web`, `pnpm build:web`, and `git diff --check`.
  Public browser evidence is recorded in
  `artifacts/vue-react-login-and-app-route-browser-2026-09-15.md`. Real
  backend authorization/payment-adjacent app flows, desktop rendering, and
  mobile rendering remain unverified or blocked-env.

## 2026-09-15 Round R362 — Public auth and protected app-entry runtime check

- Same-viewport Vue/React browser comparison found a visible React-only
  required-field marker on the login labels; it was removed to match the Vue
  runtime while native HTML validation remains in place.
- Browser navigation to the protected `/platform/apps` entry was verified to
  redirect unauthenticated users to login with the encoded `next` target.
- Added route assertions for the `knowledge_id` document deep link and the
  protected app catalog entry. Focused route tests pass 14/14; the prior full
  Web suite remains 1038/1038.
- Authenticated data states, real backend mutations, and platform-specific
  rendering remain open or blocked-env.

## 2026-09-15 Round R363 — Design-token CSS bridge

- Expanded `tokenCss` to emit the complete Vue-derived semantic token set:
  colors, typography, radii, shadows, overlay z-indexes, and motion values.
- Added the corresponding CSS entrypoint export and loaded it from the React
  Tailwind stylesheet. Existing `--color-*` compatibility variables remain
  untouched, so this change makes the token source available without silently
  changing already-reviewed page geometry or palette.
- Shared regression: `pnpm test:shared` 485/485, `pnpm typecheck:web`,
  `pnpm build:web`, and `git diff --check` passed. Dark-mode and computed-style
  browser verification remain open.

## 2026-09-15 Round R364 — Permission-sensitive document actions

- Removed the document-grid `canDownload={true}` shortcut and now pass the
  resolved knowledge-base contribution permission into the document action
  surface, preventing the grid from exposing download controls independently
  of the page permission decision.
- The initial permission default remains a compatibility fallback for the
  existing list component tests; authenticated permission and shared-KB
  variants still require paired backend/browser evidence before acceptance.
- Full Web regression after this change: 1038/1038; typecheck remains green.

## 2026-09-15 Round R365 — Route inventory completeness

- Added the two current React compatibility entries (`/platform/configuration`
  and `/platform/administration`) to the route inventory as explicit
  `react-only` rows, so the CSV no longer silently omits registered React
  surfaces.
- The independent Embed entry remains separately classified; main-SPA
  `/embed/*` is intentionally an error boundary and is not treated as the
  visitor application itself.

## 2026-09-15 Round R366 — Data-source editor permission and form boundary

- Removed the invalid nested HTML form in the knowledge-base settings
  datasource section. The editor now uses an action button with explicit
  required-field validation, preserving the Vue save and loading feedback
  while remaining safe inside the outer knowledge-base form.
- Create, edit, delete, sync, pause, resume, and validate entry points now
  respect the resolved `canManage` role boundary; read-only logs/resources
  remain available for inspection.
- Verified with `pnpm typecheck:web`, the full Web suite (1038/1038),
  `pnpm build:web`, and `git diff --check`.
- The Vue independent 640px datasource drawer, credential replacement/delete
  flow, and authenticated viewer/admin browser proof remain open pending
  paired runtime/backend evidence.

## 2026-09-15 Round R367 — Document detail drawer parity

- React `/knowledgeBase/:kbId/documents/:documentId` now uses the shared right-side
  `Sheet` with Vue `DocContent`-aligned 654px default width, 480px minimum,
  1600px maximum, persisted resize state, Escape/explicit close, and a labeled
  vertical resize separator. The shared Sheet handle now exposes the same
  accessibility role/label instead of being `aria-hidden`.
- Added real API-client contracts for `GET /api/v1/chunks/:knowledgeId`, chunk
  update, revision list, revert, and `PUT /api/v1/knowledge/:id` detail updates.
  The detail surface now loads chunk pages, exposes editor-only chunk edit and
  revision/revert controls, and editor-only summary/custom-metadata editing;
  viewers retain preview access but no mutation or original-file download.
- Evidence: focused detail tests 6/6, API-client document tests 12/12,
  `pnpm exec tsc -p apps/web/tsconfig.json --noEmit`, and focused shared API
  typecheck passed. Real authenticated backend mutations, paired browser
  screenshots/computed styles, Wails, and native-client evidence remain open;
  no mock data was added to production code.
- Open evidence/gap items: the Vue `DocContent` tabbed merged/preview/chunk
  presentation and chunk enabled/disabled retry controls are not claimed as
  byte-for-byte parity; React metadata editing is a safe JSON-object editor,
  not yet the Vue row/type editor. These require a separate visual/runtime
  pass before closing the detail parity item.

## 2026-09-15 Round R368 — Parallel parity closeout pass

- Organization settings now preserve an equivalent section selector below
  the 720px breakpoint while keeping the desktop Vue navigation rail.
- Agent editing now consumes and clears Vue-compatible `edit`, `section`,
  `highlight`, and `sourceTenantId` deep-link parameters with ownership and
  shared-agent permission checks. Highlight targets map to the corresponding
  editor sections.
- PlatformShell now exposes the pending-invitation bell and inbox with
  polling, loading/empty/error states, accept/decline actions, and keyboard/
  overlay dismissal. The poller uses an ambient timer and `unref` when
  available so test hosts are not held open.
- Data-source editing now uses a body-level independent 640px Sheet Drawer;
  the editor keeps a real form boundary outside the knowledge-base form and
  retains permission, credential, loading, and mutation behavior.
- Vue dark-theme values, font fallbacks, semantic shadcn variables, panel
  shadows, and Dialog/Sheet/Dropdown portal z-index tokens are now emitted
  and covered by shared/static assertions.
- Final command evidence for this pass: `pnpm test:shared` 488/488,
  `pnpm --filter @weknora/web test` 1053/1053,
  `pnpm typecheck:web`, `pnpm build:web`, and `git diff --check` all pass.
- Remaining acceptance gaps are explicitly not closed: authenticated
  backend/browser paired evidence, exact Vue document merged/preview/chunk
  tabs and chunk retry controls, structured metadata-row editing, Wails and
  native-client runtime evidence, and any flow requiring unavailable backend
  services.

## 2026-09-15 Round R369 — Route and hidden-entry contract closeout

- `resolveRoute()` now owns the Vue-compatible `knowledge_id` document-preview
  entry for both canonical `/platform/knowledge-bases/:kbId` and legacy
  `/knowledgeBase/:kbId` URLs; `main.tsx` consumes that parsed value instead of
  reparsing the browser URL in the page dispatcher.
- Route tests explicitly cover settings `section/subsection`, integration
  `agentId`, Agent `edit/section/highlight/sourceTenantId`, and document preview
  deep links. `route-parity.csv` records these hidden-entry contracts alongside
  the owning routes. No business page implementation was changed.
- Focused evidence: `pnpm --filter @weknora/web exec tsx --test
  src/routes.test.ts` — 12/12; remaining browser/backend/Wails/native parity
  evidence stays open.

## 2026-09-15 Round R370 — Detail-state, permission, and shared-overlay verification

- Document detail now exposes Vue-compatible preview/full-text/chunks tabs,
  ordered merged content, chunk enable/disable and failed-index retry; custom
  metadata uses typed rows with key/duplicate/number validation and preserves
  drafts after failed saves. API client contracts and focused behavior tests
  cover the new mutations.
- Knowledge, FAQ, settings, organization, Agent, invitation, and route changes
  retain fail-closed capability gates and the hidden/deep-link contracts. The
  shared Dialog/Sheet wrappers keep Tailwind/shadcn project styling, body-level
  browser Portals, tokenized overlay layers, Escape/focus handling, and an
  explicit inline SSR/static path.
- Final local evidence: `pnpm typecheck:web` passed; `pnpm test:shared` passed
  491/491; `pnpm --filter @weknora/web test` passed 1062/1062;
  `pnpm build:web` passed; `git diff --check` passed (with the pre-existing
  route CSV CRLF normalization warning). Browser evidence is recorded in
  `artifacts/vue-react-public-browser-evidence.md` and run-3 screenshots.
- Acceptance remains open for authenticated real-backend mutations and paired
  same-condition Vue/React screenshots across all states, plus Wails,
  iOS, and Android runtime evidence. Public login/redirect browser checks pass;
  unavailable auth/backend configuration is recorded as `blocked-env`, not as
  parity acceptance.

## 2026-09-15 Independent acceptance audit — evidence gate remains open

- Fresh route inventory confirms `route-parity.csv` has 59 rows: 18 pages,
  26 settings sections, 5 redirects, 1 layout redirect, 1 dev fixture, 1
  independent entry, 5 file routes, 1 WebSocket route, and 1 embed file route.
  React route focused tests pass 12/12; this is route/unit evidence only.
- Fresh public browser run 4 proves Vue and React `/login` HTTP 200 and
  unauthenticated `/platform/apps` redirect to login at `1355x776`/`zh-CN`.
  It does not prove authenticated app catalog, authorization/action states,
  backend mutations, permissions, Wails, or native parity; auth configuration
  requests remain 403 on Vue and 404 on React in this run.
- Fresh Web checks: `typecheck:web`, Vue `type-check`, and `build:web` exit 0;
  the full Web suite is not green in this audit (run 2: 1067 total, 1063
  pass, 4 failed: document-detail download/metadata and failed-save draft,
  FAQ example-download menu, and FAQ tag-clear affordance). Focused
  `AppsPages` rerun was 2/2 after a separate run-1 failure, so no flake is
  silently promoted to acceptance.
- CSV rows 21–24 reference four missing Vue `frontend/src/views/apps/*.vue`
  files while React `apps/web/src/apps/` exists; this is recorded as a
  `source-mismatch` in `artifacts/independent-acceptance-20260915.md` and
  is not silently corrected in the shared CSV.
- Full uncovered-entry/state inventory and evidence-layer definitions are in
  `artifacts/independent-acceptance-20260915.md`. No production page code was
  modified by this audit; existing dirty changes and the observed later
  deletion of `apps/web/src/apps/AppsPages.tsx` were preserved.

## 2026-09-15 Round R371 — Final regression reconciliation

- Apps catalog/connections/authorization/action models and UI tests are now
  present under `apps/web/src/apps/`; the independent audit records the absent
  Vue `frontend/src/views/apps/*.vue` sources as `source-mismatch`, not as
  silently accepted parity.
- FAQ import example downloads, tag-filter clearing, modal Escape/outside-close,
  and typed import-file validation are covered. A missing CSV serializer import
  and nullable `File` narrowing were fixed before the final gate.
- Final evidence: `pnpm typecheck:web`, `pnpm build:web`,
  `pnpm test:shared` (491/491), `pnpm --filter @weknora/web test`
  (1072/1072), Chat focused tests (91/91), and `git diff --check` pass.
- Authenticated same-condition visual/backend mutation coverage and Wails,
  iOS, Android per-feature interaction evidence remain open or `blocked-env`.

## 2026-09-15 Round R372 — Wails renderer platform parity slice

- `apps/desktop/src/main.tsx` now installs the Wails runtime before dynamically
  loading the shared React entry. This makes desktop bootstrap ordering
  explicit for API/deep-link decisions without forking Web route logic.
- Added local desktop contracts for the Vue-compatible legacy route aliases,
  query/hash-preserving deep links, HTTP(S)-only external navigation, the
  1440x900 default viewport with 1024x680 minimum, and optional secure
  credential-bridge fallback to local storage.
- Focused desktop evidence: `pnpm test:desktop` — 5/5; `pnpm
  typecheck:desktop` — passed; `pnpm build:desktop-renderer` — passed.
- The renderer build emitted the pre-existing shared CSS warning about an
  `@import` appearing after other statements. No Web/shared files were changed
  in this slice. Wails native launch/interaction evidence remains open until a
  real app window is launched and exercised; renderer build is not acceptance.

## 2026-09-15 Round R373 — Parallel parity slices and cross-platform regression

- Six scoped agents completed in parallel: Settings autosave/deduplication and
  failure retention; Integrations landing anatomy and localized copy; knowledge
  document-list card click/alignment/responsive behavior; Chat session
  confirmation, failure retention, and localized actions; native mobile
  knowledge settings/access capability routing; and Wails bootstrap,
  deep-link, viewport, external-navigation, and credential-bridge contracts.
- Focused evidence: Integrations tests passed 114; Chat tests passed 170;
  document-list tests passed 244; Settings autosave test passed 1; mobile
  tests passed 196/196 with mobile typecheck passing; desktop tests passed
  5/5 with desktop typecheck passing. The shell session regression passed
  18/18 after adapting assertions to the Vue confirmation state.
- Cross-platform final evidence: `pnpm typecheck:web` passed;
  `pnpm test:shared` passed 492/492; `pnpm --filter @weknora/web test`
  passed 1076/1076; `pnpm build:web` passed; desktop and mobile checks passed;
  `git diff --check` passed. These are static, unit, build, and renderer-level
  checks, not proof of authenticated production behavior.
- Acceptance remains open for same-condition authenticated Vue/React browser
  screenshots and computed-style comparison, real backend mutations and
  permission/state matrices, real Wails window interaction, and per-feature
  iOS/Android native flows. The Apps source mismatch (React catalog exists,
  corresponding Vue source files are absent) remains explicitly recorded as
  `source-mismatch`; it is not treated as parity acceptance.

## 2026-09-15 Round R374 — Parallel state and permission hardening

- Six agents executed in parallel with disjoint ownership. Platform restored
  the collapsed-rail search entry and command-palette affordance;
  Administration now derives tenant roles from `auth/me` and gates member
  mutations/audit actions; document preview ignores stale asynchronous text
  after a document switch; Settings restored the Vue Lite-mode auto-update
  preference; Embed keeps localized stream/API errors visible after partial
  output; and React Native Chat adds a native retry action for failed assistant
  responses.
- Commits: `b6040cf1`, `c277b6ca`, `53a7d050`, `beb6e028`, `71909eb5`, and
  `a7f56b66`. No DOM/Tailwind components were introduced into the native
  mobile slice; Web UI changes continue to use the project component/token
  layer.
- Final regression evidence: `pnpm typecheck:web` passed;
  `pnpm test:shared` passed 492/492; `pnpm --filter @weknora/web test`
  passed 1083/1083; `pnpm build:web` passed; desktop tests 5/5 and typecheck
  passed; mobile tests 197/197 and typecheck passed; Embed tests 12/12,
  direct package `tsc --noEmit`, and build passed; `git diff --check` passed.
  The attempted nonexistent Embed `typecheck` script was replaced by direct
  TypeScript checking and is not counted as evidence.
- These results are unit/static/build evidence. Authenticated same-condition
  Vue/React visual/computed-style comparison, real backend mutation and role
  matrices, real Wails feature interaction, and per-feature iOS/Android flows
  remain open; the Apps Vue-source `source-mismatch` remains unchanged.

## 2026-09-15 Round R375 — Parallel implementing-row closure pass

- A second parallel batch of six scoped agents addressed remaining
  implementation rows: knowledge upload permissions and retry-only-failed
  behavior; custom Agent system-prompt validation; per-tool MCP policy busy
  state; Wiki empty-reader state; organization settings role-intersection
  read-only behavior; and ArtifactPreview text/Markdown read failures.
- Commits: `cf3f8242`, `d803e74b`, `82691761`, `ef767ac2`, `66f51a36`, and
  `42195d64`. Focused evidence reported by the owners: upload 106/106,
  Agent 26/26, MCP 41/41, Wiki 94/94, organization 29/29, and artifact
  preview 15/15. All changes preserve the existing Tailwind/shadcn Web
  substrate or native-only mobile boundary as applicable.
- Unified regression after integration: Web typecheck passed; shared tests
  493/493; Web tests 1089/1089; Web build passed; desktop tests 5/5 and
  typecheck passed; mobile tests 197/197 and typecheck passed; Embed tests
  12/12 and direct package TypeScript check passed; `git diff --check` passed.
- Runtime evidence remains explicitly bounded: React `:5181/login` is
  reachable, Vue `:5173/login` is not running in this environment, and the
  backend `:8080` responds 401 without authentication. Therefore this round
  adds no authenticated paired visual, real-mutation, Wails feature, or
  per-feature native-device acceptance claim.

## 2026-09-15 Round R376 — Integrated second parallel batch regression

- The six implementation slices from R375 were integrated and rechecked after
  all commits landed. The final Web suite is 1089/1089 and shared suite is
  493/493; the new Wiki empty-reader, MCP per-tool policy, organization role
  intersection, upload retry, Agent validation, and artifact error tests are
  included in those totals.
- Platform checks remain green: Web typecheck/build, desktop test 5/5 and
  typecheck, mobile test 197/197 and typecheck, Embed test 12/12 and direct
  TypeScript check, plus `git diff --check`.
- This closes implementation-level differences found by the agents, not the
  runtime acceptance gate. Vue is still unavailable at `127.0.0.1:5173`, the
  current backend responds 401 without authentication, and authenticated
  same-condition screenshots, real mutations, Wails feature interaction and
  per-feature iOS/Android flows remain open. No row is promoted solely from
  these automated results.

## 2026-09-15 Round R377 — Parallel share/upload/runtime parity pass

- Six scoped agents completed in parallel across the remaining implementing
  and review rows. Knowledge sharing now follows Vue direct-remove semantics;
  upload confirmation preserves creator upload authority and retry-only-failed
  behavior; MCP keeps delayed per-tool policy state visible and independently
  busy; Wiki renders Vue's unselected-reader hint; organization settings apply
  the Vue organization/tenant admin intersection; ArtifactPreview exposes
  localized read failures; mobile FAQ references support trimmed keyword
  search; mobile Graph follows `graph_enabled` independently from Wiki; and
  the Wails credential bridge no longer mirrors bridged secrets into
  `localStorage`.
- Commits from this batch include `f1e308c9`, `3a8ccaf0`, `5768f664`,
  `799fb643`, `b2069a44`, and `f6c940fd`. Focused tests were reported green
  for each owned slice; native changes remain React Native-only and Web changes
  remain on the existing Tailwind/shadcn project component layer.
- Final integrated evidence: Web typecheck; shared tests 493/493; Web tests
  1091/1091; Web build; desktop tests 6/6 and typecheck; mobile tests 200/200
  and typecheck; Embed tests 12/12 and direct package TypeScript check; and
  `git diff --check` all passed.
- Runtime acceptance remains separate and open: Vue `127.0.0.1:5173` is
  unavailable, backend `127.0.0.1:8080` is unauthenticated (401), and no new
  same-condition authenticated screenshots, real mutation matrix, Wails
  feature interaction, or per-feature iOS/Android business evidence is
  claimed. Apps' missing Vue sources remain `source-mismatch`.

## 2026-09-15 Round R378 — Parallel upload, MCP, native and shared-layer pass

- Six scoped agents completed in parallel. Upload progress now accepts progress
  and complete events keyed by `uploadId` when `kbId` is absent; UploadConfirm
  adds the remaining Vue-aligned validation sections; MCP keeps cached policy
  controls interactive during metadata refresh; and shared Dialog uses SSR-safe
  unique title IDs. Native Chat terminal events now end the sending state,
  while the native data-source tree reveals saved deep resources and expands
  their ancestors.
- Commits: `0e17c50d`, `222b3bf8`, `a1329510`, `06a986e4`, `1d9fe868`, and
  `29f0f54c`. Focused tests were green for each owned slice, including upload
  progress 5/5, UploadConfirm 4/4, MCP 21/21, Dialog 3/3, and data-source
  focused tests 32/32. Native slices remain React Native-only.
- Final integrated evidence: Web typecheck; shared 493/493; Web 1093/1093;
  Web build; desktop 6/6 and typecheck; mobile 202/202 and typecheck; Embed
  12/12 and direct TypeScript check; and `git diff --check` all passed.
- Runtime acceptance remains open: Vue is not running at `127.0.0.1:5173`,
  backend `127.0.0.1:8080` is unauthenticated (401), and same-condition
  authenticated screenshots, real mutations, Wails feature interaction, and
  per-feature iOS/Android business flows are not claimed. Apps' missing Vue
  sources remain `source-mismatch`.

## 2026-09-15 Round R379 — Parallel page-state and evidence pass

- Six scoped agents completed in parallel. Share dialogs now reset permission
  to Vue's read-only default on reopen; upload progress covers cancelled and
  retry terminal actions, invalid progress, localization, and keyboard
  activation; UploadConfirm aligns validation sections, tab focus, reparse
  visibility, submit locking, and close behavior; MCP tools add explicit ARIA
  names, dialog relationships, and live pagination announcements; native
  document editing honors shared `editor` permissions; and the browser
  runtime evidence records the actual port split and tool failure boundary.
- Additional implementation evidence includes shared Dialog SSR title-ID
  stability, mobile Chat terminal-state handling, mobile data-source deep-tree
  expansion, and mobile FAQ search from the preceding integrated slices.
- Final integrated regression: Web typecheck; shared 493/493; Web 1098/1098;
  Web build; desktop 6/6 and typecheck; mobile 203/203 and typecheck; Embed
  12/12 and direct TypeScript check; `git diff --check` all passed.
- Browser runtime evidence is explicitly bounded in
  `artifacts/browser-evidence-20260916.md`: React `:5181` and Vue
  `:5180` respond, legacy Vue `:5173` does not, backend `:8080` returns 401
  anonymously, and browser automation is blocked by missing `oci` / browser
  request-header policy. No authenticated paired visual or real-backend
  parity claim is made from these observations.

## 2026-09-15 Round R380 — Integrated public browser comparison

- Six scoped agents completed in parallel. Share forms now reset both
  permission and organization after successful sharing; upload progress covers
  all terminal actions and keyboard activation; UploadConfirm guards Escape,
  overlay, and close-button dismissal during submission; MCP retains tools on
  policy failure and exposes ARIA relationships/live pagination; mobile
  document preview locks duplicate actions and shared editor permissions; and
  mobile Chat terminal events reliably clear the sending state.
- Final integrated evidence: Web typecheck; shared 493/493; Web 1111/1111;
  Web build; desktop 6/6 and typecheck; mobile 204/204 and typecheck; Embed
  12/12 and direct TypeScript check; `git diff --check` all passed.
- Fresh paired public browser evidence is recorded in
  `artifacts/vue-react-public-browser-evidence-20260915-run5.md` with
  `artifacts/browser-evidence-20260915-run5/results.json` and 1355x776
  screenshots. It proves only public/unauthenticated behavior and exposes a
  real login geometry/style difference plus React/Vue auth endpoint mismatch;
  it does not close protected-page parity.
- Authenticated same-condition visual/computed-style evidence, real backend
  mutations and permission matrices, Wails feature interaction, and
  per-feature iOS/Android flows remain open. The Apps Vue-source
  `source-mismatch` remains explicit.

## 2026-09-15 Round R381 — Auth/runtime parity and regression closure

- Eight agents were dispatched in parallel under the updated execution
  constraint. Integrated login computed-style alignment (`1263ff08`), Vite
  dev/preview `/api` and `/files` proxy plus contract tests (`cf780c86`,
  `e317eb78`), safe invite/OIDC/`next` navigation (`1734f4ed`), failed
  document-list envelope rejection (`0737920c`), shared Input focus/invalid
  behavior (`c738006a`), MCP policy-error retention (`3ce207d6`), and mobile
  native login validation (`92b613ed`, `a92b3f0c`).
- Verification passed: Web 1122/1122, shared 493/493, mobile 205/205,
  Embed 12/12, desktop 6/6; Web typecheck/build; mobile and desktop
  typechecks; and `git diff --check`. Mobile validation uses the repository's
  native `node:test` runner.
- Real HTTP proxy evidence at React dev `:5182` and preview `:5183` reached
  the backend: auth config/OIDC returned 200, login and `/files` returned the
  expected anonymous 401. This does not prove authenticated login, tenant
  permissions, or mutations.
- Paired public Playwright evidence (Vue `:5180`, React `:5181`, zh-CN,
  1440x900) is recorded in `artifacts/vue-react-public-route-differences-20260915.md`
  and `artifacts/browser-evidence-20260915/`. Protected-page authenticated
  visual/interaction evidence, Wails interaction, and iOS/Android runtime
  evidence remain absent.
- The matrix remains open (`N005`, `N007`, `N016`, `N031` and rows missing
  authenticated browser, computed-style, real-backend, or native evidence),
  so this round does not mark full Vue/React page alignment.

## 2026-09-15 Round R382 — Direct registration policy parity

- Paired public browser rerun (Vue `:5180`, React `:5181`, zh-CN, 1440x900)
  showed a conditional `/register` difference: Vue rendered the login
  surface while React rendered registration. React now gates direct
  registration on the real `/api/v1/auth/config` policy and falls back to
  login for `invite_only` (`7dcdbbdf`).
- The same run confirmed both login inputs match Vue computed style for color,
  transparent background, system font, 15px/24px metrics, border, radius,
  padding, and 374x24 geometry. Remaining vertical offsets stay recorded as
  runtime evidence rather than silently accepted as full parity.
- Web tests 1122/1122, Web typecheck, Web build, and `git diff --check` passed
  after the route change. The backend currently reports `self_serve`, while
  the Vue `:5180` runtime still renders login at `/register`; this source vs
  runtime conflict remains an environment/trigger investigation item.

## 2026-09-15 Round R383 — Web-only protected-route and settings pass

- Under the updated Web/desktop-only scope, parallel agents covered share
  states, graph settings, MCP interactions, protected-route browser evidence,
  shared UI review, desktop review, and ledger review. Mobile work was
  explicitly excluded.
- Integrated share-state contract coverage (`cedafeb3`) and MCP dialog
  keyboard/focus/ARIA coverage (`f03b4435`, `615f79cf`). The graph settings
  component and its Vue-derived validation/state tests are present
  (`f1d380e3`, `615f79cf`); the existing full knowledge-base editor remains
  authoritative for the live graph route, avoiding replacement of its larger
  behavior surface.
- Protected-route evidence now covers Vue/React at 1440x900, 1024x768, and
  390x844, including anonymous redirects, `next`, and backend response status:
  `artifacts/vue-react-protected-route-evidence-20260915.md` and
  `artifacts/browser-evidence-20260915/protected-route-results.json`.
- Web regression after integration: 1134/1134 tests, Web typecheck, Web build,
  and `git diff --check` passed. Evidence remains anonymous/public; no
  authenticated mutation or permission acceptance is claimed.
- Remaining open scope is unchanged: authenticated protected-page parity,
  real tenant permission/mutation flows, Wails runtime interaction, and any
  rows whose Vue source/runtime trigger is unresolved. No mobile item is used
  as acceptance for this updated objective.

## 2026-09-15 Round R384 — Web-only integration verification

- The updated objective explicitly excludes mobile code. Web-only changes from
  the parallel pass were integrated conservatively: the existing large
  knowledge-base editor was preserved while GraphSettings validation/state
  coverage was retained; MCP labels were restored to the Vue zh-CN contract
  after an agent introduced English fallback text.
- Final Web verification after integration: 1134/1134 tests, Web typecheck,
  Web build, desktop 6/6 tests, desktop typecheck, and `git diff --check`
  passed. Build emitted only the existing large-chunk advisory.
- The scheduler continued returning `agent thread limit reached` for later
  dispatch attempts even after visible completed agents were closed. This is
  recorded as an execution-environment limitation, not as evidence of page
  parity or a reason to reduce scope.
- Authenticated protected-page interaction, real tenant permissions/mutations,
  Wails runtime behavior, and unresolved Vue source/runtime trigger conflicts
  remain open; no completion claim is made.

## 2026-09-15 Round R385 — Graph extraction capability guard

- Reviewed the Vue-authoritative protected graph-extraction boundary and
  integrated `5e30f578` (`fbd938fc`): React GraphSettings defaults to hiding
  tag/text/relation extraction actions unless `canRunGraphExtract === true`.
  Explicitly authorized callers retain the actions and their existing model,
  input, loading, and validation behavior.
- Focused GraphSettings validation/state tests from the scoped agent passed
  (6/6 before integration); no mobile files were changed.
- Full Web regression now passes 1137/1137; desktop remains 6/6 with its
  typecheck passing. Authenticated permission/backend evidence and Wails
  runtime evidence remain open, so the parity matrix remains incomplete.

## 2026-09-15 Round R386 — Settings, upload, and knowledge navigation review

- Integrated Vue-shaped Settings body Portal behavior (`10d1026a`), upload
  confirmation cancel-before-confirm ordering (`368732a5`), and canonical
  knowledge-base detail navigation with encoded ids (`d658e09f`). These are
  bounded Web changes using the existing project UI wrappers and preserve the
  current large knowledge editor.
- Verification: Web tests 1137/1137, Web typecheck, Web build, desktop tests
  6/6, desktop typecheck, and `git diff --check` passed.
- This remains implementation/regression evidence only. Authenticated paired
  Vue/React screenshots, real permission and mutation flows, and Wails
  feature interaction evidence remain open.

## 2026-09-15 Round R387 — Desktop bridge pre-bootstrap parity

- Integrated `e7388183` for the React desktop renderer: the Wails API base is
  resolved before the shared renderer imports, external HTTP(S) URLs are sent
  through the Wails browser bridge, and unsafe schemes remain rejected.
- Desktop focused verification is 8/8, desktop typecheck and build pass. The
  audit is recorded at `apps/desktop/evidence/2026-09-15-wails-renderer-audit.md`.
- Native Wails launch, OS deep-link delivery, browser handoff, and secure
  credential storage remain `blocked-env`; this commit does not claim those
  runtime gates are accepted.

## 2026-09-15 Round R388 — Knowledge-base state contract slice

- Integrated `531877bf` from the parallel knowledge-base review. It adds
  Vue-shaped detail-state normalization, loading/error/permission branches,
  and search parameter construction with focused state tests; the existing
  canonical detail route and large page implementation remain intact.
- The agent reported 35/35 knowledge-base focused tests. Full Web regression
  and typecheck were re-run after integration: 1137/1137 and typecheck pass.
- Authenticated paired visual capture, real tenant mutations, and Wails
  feature interaction remain open; this slice is not full page acceptance.

## 2026-09-15 Round R389 — Public registration and OIDC error parity

- Integrated `bf6c1ff7` from the parallel auth review. Direct `/register` now
  follows the Vue observed contract: an uninvited visit remains on the login
  surface, while self-serve registration requires the explicit register mode;
  valid invites remain supported. OIDC error and description parameters are
  preserved as a one-time localized login error.
- Auth and route focused verification passed 15/15; the full Web suite and
  Web typecheck were green before this final auth integration and must be
  re-run after it.
- This does not claim successful provider round-trip, authenticated tenant
  permissions, or real registration mutation evidence.

## 2026-09-15 Round R390 — Wiki and FAQ failure-state parity

- Integrated `e95d6559` from the parallel Wiki/FAQ review. Wiki create/update
  now rejects blank title/content before writes, and FAQ import polling keeps
  backend failure visible instead of silently returning to the empty state.
- Focused Wiki/FAQ verification passed 101/101; Web typecheck and build passed.
  No mobile files were modified.
- Authenticated provider-backed mutations, paired Vue/React screenshots, and
  Wails feature interaction remain open acceptance gates.

## 2026-09-15 Round R391 — Integrations and knowledge-settings integration gate

- Integrated `42ee2f16` for Vue-shaped integrations drawer/tab/loading/error/
  permission state contracts, then fixed its project-component mismatches in
  `33913b84` (`Alert` danger tone and `Sheet onClose` contract).
- Integrated `3e56e446` for the knowledge detail settings sections and repaired
  its existing `main.tsx` route contract in `a9496a98`; the large App remains
  preserved. Graph unavailable state now uses the project warning tone.
- Post-fix verification: Web tests 1152/1152, knowledge-settings focused tests
  12/12, Web typecheck, and Web build passed. Authenticated paired screenshots,
  real mutations, and Wails feature interaction remain open.

## 2026-09-15 Round R392 — Platform shell, organizations, and chat recovery

- Integrated the bounded platform-shell review (`09f802e3`), retaining the
  existing Vue-derived shell implementation and adding the isolated shell
  interaction model/tests. The attempted replacement shared UI implementation
  (`2f66f789`) and unwired Agents page seam (`b65696b4`) were not integrated:
  both conflicted with the existing project component/page contracts and would
  have replaced rather than adapted the established wrappers.
- Integrated the organization detail state/permission slice (`64fc3476`):
  list/detail feed loading and errors are explicit, shared-agent and invite
  member feeds are represented, owner mutations are guarded, and create/update
  descriptions are trimmed. Existing organization tests remain green.
- Chat terminal/stopped-run recovery was already present in branch history as
  `f4ba2c2f`; its bounded review reported 95/95 focused tests and no mobile
  changes.
- Verification after integration: Web regression 1159/1159, Web typecheck,
  Web build, desktop renderer tests 8/8, desktop typecheck, and `git diff
  --check` passed. These are static/unit/build evidence; authenticated paired
  Vue/React screenshots, real backend permission/mutation flows, and native Wails
  feature interaction remain open acceptance gates.

## 2026-09-16 Round R393 — Route, tokens, data sources, settings, chat, and Wails

- Parallel route inventory (`4d124557`) records all 59 known entries: pages,
  redirects, 26 settings sections, hidden/history URLs, embed/file routes, and
  the sandbox WebSocket. It also records two P1 follow-ups in
  `docs/migrations/react/route-inventory-20260915.md`; inventory is evidence,
  not automatic acceptance.
- Shared Vue/TDesign aliases and token tests were added in `40ace7ce`; data
  source viewer permissions, empty state, sync-in-flight state, and active/
  paused actions were aligned in `4157b975`.
- Settings drawer focus containment and restoration were aligned in
  `0f015561`. Chat sidebar clear propagation and stopped/reconnect terminal
  states were aligned in `4ef6ffef`.
- Desktop Wails async API-base bootstrap was aligned in `c6efcc99`; native
  Wails launch and OS integration remain `blocked-env`.
- Final regression for this round: Web tests 1163/1163, Web typecheck, Web
  build, desktop tests 8/8, desktop typecheck, and `git diff --check` pass.
  These are static/unit/build evidence. Authenticated same-condition browser
  screenshots, real backend permission/mutation flows, and full Wails runtime
  interaction remain open, so the overall parity goal is not yet complete.

## 2026-09-16 Round R394 — Focus containment and current acceptance audit

- `0f015561` adds Vue-compatible settings drawer focus containment and opener
  restoration. `4157b975` and `4ef6ffef` are included in the current branch and
  keep datasource and chat terminal-state behavior explicit.
- The current route inventory identifies the remaining non-URL
  `openSettings('models', 'knowledgeqa')` semantic gap. It is not mapped to an
  arbitrary model filter; this remains an explicit P1 follow-up until the
  corresponding Vue trigger and React destination are verified together.
- Re-run after all current Web/Desktop commits: Web 1163/1163, Web typecheck,
  Web build, desktop renderer 8/8, desktop typecheck, and `git diff --check`.
- Runtime acceptance is still incomplete: the Vue service was not listening at
  `127.0.0.1:5180` before the local service was started, authentication-backed
  same-condition page/state screenshots and real backend permissions/mutations
  are unavailable, and native Wails launch/OS integration remains
  `blocked-env`. No mobile code was touched.

## 2026-09-16 Round R395 — Deep-link, document, Embed, browser, and P1 audit

- `5a5bd768` maps the Vue-only `knowledgeqa` settings trigger to the React
  models conversation subsection and adds regression coverage. The original
  P1 non-URL settings gap is therefore closed for this known trigger.
- `add1dd8a` gates document download actions by the Vue-supported source types
  (`file` and `manual`), with a regression test; URL-backed documents no longer
  expose an invalid download affordance.
- `57800916` records same-viewport browser evidence for anonymous login,
  registration, protected apps/settings routes, and existing-session behavior.
  Anonymous route behavior is verified; existing-session protected views remain
  `blocked-env` because the available auth/backend state did not provide a
  comparable authenticated workspace for both applications.
- `c0ba752d` records the initial Embed audit. The current checkout now does have
  a React `apps/embed` isolated entry and bridge/client implementation, but the
  old audit text is stale and must not be treated as the current implementation
  inventory. A fresh source comparison is required for the remaining Vue chat
  states and visual contract.
- `f1abccb1` records the knowledge-base audit: F-01 (metadata failure rendered
  as viewer read-only) and F-02 (independent document detail misses ordinary
  tenant admin/contributor permission) remain P1 follow-ups requiring runtime
  and permission-contract confirmation.
- `5346563c` corrects the debounce test timing so the Vue 500ms autosave test
  observes the effect after React commits the state update; it does not alter
  production save behavior.
- Final verification after this round: Web tests 1164/1164, Web typecheck,
  Web build, desktop renderer tests 8/8, desktop typecheck, and `git diff
  --check` pass. This is static/unit/build evidence. Authenticated real-backend
  mutation/permission flows, full Embed runtime, and native Wails interaction
  remain open; no mobile files were modified.

## 2026-09-16 Round R396 — Knowledge permissions, Apps lifecycle, and bridge seam

- `9b6a0846` closes the two knowledge-base audit findings at the implementation
  level: metadata load failures and forbidden responses remain visible with a
  retry path, and independent document detail grants the Vue-compatible tenant
  admin/contributor capability while retaining shared-viewer restrictions.
  Focused document permission tests and the full Web suite pass.
- `40f9a850` aligns the React Apps lifecycle pages with the Vue state model for
  connection, authorization, and action/endpoint views, including terminal
  states and permission-sensitive controls.
- `6cff26d3` adds a Vue-aligned Embed bridge contract for pinned origins, token
  parsing, error mapping, and image/file handling. The isolated React Embed
  entry is present and builds, but source comparison still finds open parity
  gaps in timestamps, typing state, follow-up suggestions, Web Search controls,
  and rendered attachment/image affordances; full host/runtime acceptance is
  therefore still open.
- Browser evidence continues to verify anonymous login/register/protected-route
  behavior only. Authenticated same-condition screenshots, real backend
  permission/mutation flows, and native Wails launch/feature interaction remain
  `blocked-env`; no mobile code was modified.
- Final verification for this round: Web tests 1172/1172, Web typecheck, Web
  build, desktop renderer tests 8/8, desktop typecheck, and `git diff --check`
  pass. The build emits only existing chunk-size and test-only `import.meta`
  warnings. These results are static/unit/build evidence and do not complete
  runtime parity acceptance.

## 2026-09-16 Round R397 — Isolated Embed completion slice, Wails retry, and authenticated audit

- `4dd6b740` completes the current isolated Embed implementation slice: the
  React entry now covers the Vue-derived header tokens/title modes, suggested
  question loading, timestamps, typing/scroll states, file and image previews,
  upload limits, Web Search toggle, stop flow, follow-up questions, session
  title events, references, and error visibility. It uses the project Button
  wrapper and preserves the independent `/embed/` entry boundary.
- `100f51f3` adds the channel-scoped follow-up suggestion API operations and
  session headers, with encoded-path and request-contract tests.
- `f93d3223` makes the desktop renderer wait for a late Wails binding before
  resolving the API root; the focused desktop suite is now 9/9. `ab86e359`
  rejects malformed encoded Apps deep-link IDs, and `7b1df4dc` closes the
  corresponding route-inventory follow-up.
- `c416c526` records fresh anonymous browser/backend evidence: health and auth
  configuration endpoints return 200, protected API calls return 401, and
  anonymous login/register/protected-route behavior is verified. No safe
  credentials are available for authenticated tenant permissions, mutations,
  portal interactions, or real Embed streaming, so those remain `blocked-env`.
- Focused/full verification for this round: Web 1172/1172; Embed 15/15;
  Embed API 6/6; Desktop 9/9; Web, Embed, and Desktop typechecks/builds pass;
  route tests 13/13; `git diff --check` passes. Builds retain only advisory
  chunk-size and CSS import-order warnings. These are static/unit/build and
  anonymous browser/backend evidence, not full authenticated or native Wails
  acceptance. No mobile code was modified.

## 2026-09-16 Round R398 — Embed timestamp grouping and browser-policy recheck

- `033f29d8` completes the Embed timestamp grouping rule against
  `EmbedChatCore.vue`: render at the start of a user turn, after a five-minute
  gap, or when the calendar date changes; the formatter keeps Vue's locale
  buckets for today/yesterday/year. Embed focused coverage is now 16/16.
- A fresh CUA browser inventory was attempted against the local runtime, but
  the environment returned `Unable to load browser request-header policy` before
  any browser tab could be controlled. Existing anonymous HTTP/backend
  evidence remains valid; authenticated paired screenshots and real Embed
  streaming remain `blocked-env`.
- Current source audit finds the Embed behavior contract covered by tests, while
  `apps/embed/src/styles.css` still contains standalone CSS rather than a full
  migration to the shared Tailwind/shadcn token wrappers. This remains an
  explicit visual-implementation follow-up and prevents claiming complete
  pixel-level parity.

## 2026-09-16 Round R399 — Current route denominator and authority audit

- `8122e7b0`, `26cc0893`, and `fb62960d` record the current route inventory
  audit and make its checks reproducible. The CSV has 59 data
  rows, no explicit stable row ID, and four Apps rows whose referenced Vue
  `frontend/src/views/apps/*.vue` files are absent from this checkout but are
  resolved to historical commit `9b0c11c4` by R400. It also identifies combined `embed.html` path fields and
  wildcard/missing artifact references that cannot serve as direct evidence
  links.
- Historical 53/56-row counts are now explicitly marked as historical; the
  current matrix, baseline, acceptance inventory, and route inventory point to
  `artifacts/route-inventory-parity-ledger-audit-20260916.md`. Apps authority
  resolution is closed at the static source-trace level, while stable
  route-to-row mapping and visual/permission/runtime acceptance remain
  review/open, so no blanket page acceptance is inferred from route reachability.
- This is static-document evidence only. It does not replace Vue runtime
  screenshots, computed-style comparison, authenticated permissions/mutations,
  real Embed streaming, or native Wails interaction. No mobile code was
  modified.

## 2026-09-16 Round R400 — Resolve historical Apps Vue authority

- `4157fdf8` resolves the four Apps Vue authority references through historical
  commit `9b0c11c4`: `AppsView.vue`, `ConnectionsView.vue`,
  `AuthorizationView.vue`, and `ActionView.vue`. The mapping and reproducible
  `git cat-file`/router checks are recorded in
  `artifacts/apps-vue-authority-20260916.md`.
- The prior “current file missing” finding was a checkout-location issue and is
  closed as a static source-trace problem. It does not grant parity acceptance:
  same-condition visual comparison, authenticated permission gates, provider
  authorization, action mutations, and terminal/error runtime states remain
  to be verified against this historical Vue authority.
- Route inventory still lacks an explicit stable `row_id`, and the combined
  `embed.html` field still needs splitting. These remain documentation-quality
  follow-ups. No mobile code was modified.

## 2026-09-16 Round R401 — Enable the shared Tailwind/shadcn token pipeline for Embed

- `d34ee440` enables Tailwind v4 processing in the isolated `apps/embed` Vite
  entry, scans the project UI and Embed sources for utilities, and loads the
  shared design-token and shadcn theme layers once from `styles.css`. The
  duplicate direct theme import was removed from `EmbedApp.tsx`; the existing
  Vue-derived Embed behavior and project `Button` wrapper remain unchanged.
- Post-change evidence: `pnpm --filter @weknora/embed build` passes with the
  generated Tailwind/theme CSS; the Embed focused suite is 16/16; Web and
  Desktop typechecks pass; `git diff --check` passes. These are build, focused
  unit, typecheck, and static evidence only.
- The Embed stylesheet still contains legacy standalone selectors, so this
  round establishes the shared processing/token substrate but does not claim
  complete pixel parity. Authenticated paired screenshots, computed-style
  comparison, real Embed streaming, protected preview/reference behavior, and
  native Wails interaction remain `blocked-env` or open per R397-R398. No
  mobile code was modified.

## 2026-09-16 Round R402 — Make the current route denominator machine-addressable

- Added `docs/migrations/react/route-row-map.csv`, an explicit `R001-R059`
  mapping for every current route, settings surface, independent entry, file
  route, WebSocket route, and Embed route. A reproducible check confirms the
  map has 59 rows, zero duplicate IDs, and exactly matches the route/kind pairs
  in `route-parity.csv`.
- Updated the canonical matrix and route audit to distinguish the current
  59-row mapping from historical 53/56-row snapshots. This closes the stable
  row-to-route documentation gap, but does not promote any row to `accepted`.
- Visual/computed-style comparison, authenticated permissions and mutations,
  real Embed token/streaming, and Wails feature interaction remain open or
  `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R403 — Reconcile the knowledge permission audit with current code

- Rechecked the historical KB audit against the current worktree. The
  metadata failure classification/retry branch is present, and the shared
  `computeKBPermissions` helper plus focused tests cover independently opened
  home-tenant KBs for tenant `admin` and `contributor` memberships. The audit
  artifact now labels F-01/F-02 as historical findings with a current-state
  amendment instead of presenting stale source lines as active defects.
- Evidence is source/focused-unit only; authenticated paired Vue/React
  screenshots, real backend permission mutations, and Wails feature
  interaction remain open. No mobile code was modified.

## 2026-09-16 Round R404 — Move the upload confirmation footer onto the UI utility pipeline

- `1d73b926` replaces the remaining inline `style` object in
  `UploadConfirmDialog.tsx` with Tailwind utility classes for flex layout,
  alignment, and spacing. The dialog continues to use the project `Button`,
  `Dialog`, and `Status` wrappers; no Vue behavior or API contract changed.
- Web full regression remains 1172/1172, Web typecheck passes, and
  `git diff --check` passes. This is implementation/unit/static evidence; it
  does not replace same-condition Vue screenshots or real backend mutation
  verification. No mobile code was modified.

## 2026-09-16 Round R405 — Recheck local runtime and browser evidence boundary

- Local probes currently reach Vue dev at `127.0.0.1:5180` with HTTP 200 and
  the backend at `127.0.0.1:8080` with the expected anonymous HTTP 401. The
  React Vite process is present but `127.0.0.1:5181` did not accept a
  connection during five bounded retries, so no React browser comparison was
  claimed.
- CUA browser inventory was retried after the Vue service became reachable;
  the environment still fails before tab control with
  `Unable to load browser request-header policy`. This preserves the current
  `blocked-env` classification for paired DOM/screenshots and interaction
  evidence; it is not a page-parity result. No mobile code was modified.

## 2026-09-16 Round R407 — Integrate parallel Web/Desktop parity fixes

- Eight parallel scopes were reviewed. Integrated commits align Apps catalog
  risk/install/publish badges and schema digest (`3345fb22`), shared Button
  control heights (`4d492637`), non-steer streaming stop behavior
  (`bd1edf01`), session-row confirmation ownership (`277c175a`), integration
  channel keyboard activation and checkbox geometry (`4d1dd75a`), and Vue
  reciprocal graph-edge rendering with optional arrows (`cd59454f`). The
  corresponding integration ledger update is `04cf515c`.
- Focused evidence passed for UI, Chat, Integrations, Shell, and Graph. The
  current Web suite is 1175/1175; Web typecheck/build pass; Desktop renderer
  is 9/9 with typecheck passing; and `git diff --check` passes. Build output
  contains only the existing large-chunk advisory.
- These changes improve source/unit/DOM-host parity but do not prove
  authenticated same-condition screenshots, real backend permissions or
  mutations, Embed host streaming, or Wails feature interaction. No mobile
  code was modified.

## 2026-09-16 Round R408 — Add Graph runtime interaction coverage

- `a78cd6d0` adds a jsdom interaction test for the Knowledge Graph arrow
  toggle, reciprocal-edge deduplication, and marker visibility. The test
  exercises the rendered SVG contract rather than only testing the pure
  helper; Graph focused coverage is now 10/10.
- The supplemental change keeps Vue-derived edge color/opacity and uses a
  directed-edge set for deterministic reciprocal detection. Web typecheck
  and `git diff --check` pass. This remains DOM-host/source evidence, not
  authenticated backend graph data or paired Vue screenshot evidence. No
  mobile code was modified.

## 2026-09-16 Round R406 — Integrations form and channel-entry parity review

- `4d1dd75a` fixes two confirmed Web integrations differences against the Vue
  channel panels and settings forms: channel cards now expose the same edit
  entry through keyboard Enter/Space as well as pointer activation, and the
  API direct-header permission control uses a compact checkbox token instead
  of the accidental full-width input geometry.
- Focused integrations contract tests pass 9/9, `git diff --check` passes, and
  Web typecheck was run. This is source/focused-unit/static evidence only;
  authenticated permission mutation, paired Vue/React computed styles and
  screenshots, and native Wails interaction remain unverified. No mobile code
  was modified.

## 2026-09-16 Round R409 — Post-graph full Web regression

- After the Graph arrow/toggle interaction coverage was integrated, the current
  Web workspace regression completed with `1176/1176` tests passing. This is
  unit/component evidence only; it does not promote authenticated visual,
  real-backend mutation, browser Portal, real Embed streaming, or native Wails
  rows to accepted.
- `git diff --check` remains clean for parity files. The only dirty paths are
  the user's pre-existing Go/workbench changes and generated workbench plan
  files; no mobile code was modified.

## 2026-09-16 Round R410 — Additional page-local parity fixes

- Parallel page-local reviews added verified fixes for MCP tool-detail open
  state styling (`3ac24d1d`), empty Ollama download gating (`55be8773`), Vue
  user-menu external links (`4abf1515`), document merged-Markdown rendering,
  login background icon ordering, and Chat knowledge-base mention scope
  de-duplication (`807f0eb6`).
- Focused evidence: Auth, document detail, PlatformShell, Ollama, and MCP
  suites passed in the combined 48-test run; Chat agent-selection passed
  13/13. These remain source/component evidence, not authenticated paired
  screenshots or real-backend/Wails acceptance. No mobile code was modified.

## 2026-09-16 Round R411 — Full Web regression after page-local fixes

- The current Web suite completed with `1181/1181` tests passing after the
  additional Auth, document, Platform, Settings, MCP, and Chat changes.
- This confirms unit/component regression only. Authenticated Vue/React
  screenshots, computed-style comparison, real backend permission/mutation
  flows, real Embed streaming, and native Wails interaction remain open or
  `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R413 — Invite registration and document token parity

- `1facdd6d` makes the invite registration route reuse the complete Vue-derived
  authentication shell, retaining the invitation-token error path.
- `c7a1ebfd` applies the semantic surface token to the Vue-equivalent merged
  Markdown document view.
- Full Web regression after these changes passed `1182/1182`. This remains
  unit/component evidence; authenticated paired screenshots, real backend
  permission/mutation flows, real Embed streaming, and Wails interaction are
  still open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R412 — Browser connection restored, paired state still blocked

- Browser Use local CDP successfully rendered both applications and captured
  `artifacts/evidence/vue-react-parity/2026-09-16-r412/`.
- The Vue tab had an existing authenticated session and redirected `/login` to
  the knowledge-base page, while React rendered the anonymous login page. The
  tabs also reported different content viewport dimensions. This confirms the
  browser tooling path but is not same-condition Vue/React visual acceptance;
  credentials/session transfer and authenticated mutation evidence remain
  unavailable. No mobile code was modified.

## 2026-09-16 Round R414 — Web/Desktop/Embed platform regression

- Parallel platform checks passed: Web typecheck and build, Desktop renderer
  tests `9/9` and typecheck, and Embed tests `16/16` and typecheck.
- The Web build retains only the existing chunk-size advisory. These checks
  establish static/build/component evidence and do not replace authenticated
  paired screenshots, real backend mutations, real Embed streaming, or native
  Wails interaction. No mobile code was modified.

## 2026-09-16 Round R415 — Parallel page parity fixes and regression boundary

- Parallel Auth, Knowledge Graph, Embed, and remaining Web reviews landed
  `1954fe2f`, `8fa9b1fd`, and `cdc14622`; the working-tree page fixes also cover
  Markdown fixtures, starter-question refresh, integration edit gating,
  platform navigation, parser controls, retrieval sliders, capability-gated
  settings links, tenant delete confirmation, and model-debug thinking state.
- Focused Web suites for Auth/Graph/Chat/Markdown/Settings/Model Debug/Sandbox
  and supporting surfaces passed; Web typecheck/build passed, Desktop passed
  `9/9`, and Embed passed `17/17` with typechecks. A full Web run reached
  `1193` passing tests but was not promoted to a clean full-suite pass because
  two concurrently executed jsdom suites remained pending until interruption.
- Browser evidence remains `blocked-env` for paired authenticated Vue/React
  screenshots and real backend mutation states; the existing R412 evidence is
  explicitly unpaired. Wails native interaction and real Embed streaming remain
  unverified. No mobile code was modified.

## 2026-09-16 Round R416 — Serial Web regression isolation

- Re-running the Web suite with `--test-concurrency=1` removed cross-file
  assertion interference: the run reached `1068` passing tests before the
  SandboxSettingsPanel process remained pending. The same Sandbox suite passes
  its 26 assertions but does not terminate because of an unresolved jsdom
  lifecycle/promise; this is recorded as a test-harness blocker rather than a
  product-parity pass.
- Typecheck/build, focused page suites, Desktop `9/9`, and Embed `17/17`
  remain green. Authenticated paired visual evidence, real backend mutations,
  native Wails interaction, and real Embed streaming remain open or
  `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R417 — Sandbox inventory session navigation

- Fixed a concrete Vue interaction gap in `SandboxSettingsPanel`: selecting an
  inventory session now closes the right-side occupancy drawer before invoking
  the host session-navigation callback, matching `SandboxSettings.vue`
  `openSession` behavior.
- SandboxSettingsPanel regression now completes `28/28`; this is focused
  component evidence. Full authenticated browser, real sandbox backend, and
  Wails interaction evidence remain open or `blocked-env`. No mobile code was
  modified.

## 2026-09-16 Round R418 — Full Web regression after Sandbox fix

- The full Web regression now exits cleanly with `1195/1195` tests passing,
  `0` failing, `0` cancelled, and `0` skipped. This includes the previously
  pending SandboxSettingsPanel suite and is unit/component evidence for the
  React Web implementation.
- This result does not promote the parity matrix to complete: authenticated
  same-condition Vue/React screenshots, real backend permission and mutation
  flows, real Embed streaming, and native Wails interaction remain open or
  `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R419 — Parallel organization/document/integration/settings parity

- Four independent implementation/review slices landed concrete Vue-derived
  fixes: organization member search and empty-name warning validation
  (`828f1271`, `696b332d`, `64351594`), document download-versus-mutation
  permission gates (`0beb1378`), non-admin Embed integration edit lockdown
  (`511a3f47`), and Settings shell focus-visible states (`9f3cd728`).
- The post-integration Web regression passed `1202/1202`; Web typecheck and
  build passed. Focused evidence also passed: Organizations `33/33`,
  Documents `182/182`, Integrations `120/120`, Settings `217/217`, Desktop
  renderer `9/9` plus typecheck/build, and Embed `17/17` plus typecheck.
- The Desktop build still emits the existing CSS `@import` ordering warning and
  large-chunk advisories. These checks remain static/unit/component evidence;
  authenticated paired Vue/React browser screenshots, real backend
  permission/mutation flows, real Embed streaming, and native Wails feature
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R429 — Parallel embed, settings, approval, and terminal parity

- Four independent Vue-baseline fixes landed: Embed `Meta+Enter` now submits
  like the Vue composer (`fbecd2d3`); Settings section switches ignore stale
  responses from prior loads (`adce7bce`); wrapped 409 responses are recognized
  as steer conflicts for re-base retry (`03105f15`); and Sandbox terminal
  provisioning is enabled only when explicitly requested, with
  `SANDBOX_NOT_BOUND` mapped to the Vue needs-provision state (`291ac8db`).
- Shared regression passed `501/501`; integrated Web regression passed
  `1238/1238`; Web typecheck, `git diff --check`, Web build, Embed build,
  Desktop 9/9 tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real backend Embed/OAuth/
  steer/terminal/settings flows, and per-feature Wails interaction remain open
  or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R428 — Parallel organization, composer, approval, and agent parity

- Four independent Vue-baseline fixes landed: organization settings now hide
  join-request controls from non-admin roles (`9bcc161b`); Chat composer now
  matches Vue Enter/Shift/Ctrl/IME/Alt shortcut behavior (`6a390cb3`); tool
  approval uses a synchronous submission lock to prevent duplicate approve or
  reject requests (`56975878`); and Agent list actions now enforce Vue's
  builtin/shared write-permission gates (`e652f06a`).
- Shared regression passed `501/501`; integrated Web regression passed
  `1231/1231`; Web typecheck, `git diff --check`, Web build, Embed build,
  Desktop 9/9 tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real backend organization/
  chat/approval/agent permission and mutation flows, and per-feature Wails
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R420 — Parallel platform/chat/knowledge/MCP parity

- Four independent Vue-baseline slices landed: platform tenant-switcher
  capability/collapse gating (`643433ef`), visible assistant trace timeline
  for thinking/tool/finish states (`73c100b8`), omission of empty graph node
  and relation collections (`89737973`), and Vue-aligned MCP policy control
  layout (`b6328fb6`).
- Integrated evidence passed: Web full regression `1205/1205`, Web and shared
  typechecks, shared tests `500/500`, Web build, and Embed build. The four
  focused slices reported platform `26/26`, chat views `68/68` plus Web chat
  `98/98`, knowledge `38/38`, and MCP `28/28`.
- Build output retains existing large-chunk advisories. Static/unit/component
  evidence still does not prove authenticated paired Vue/React screenshots,
  real backend permission/mutation flows, real Embed streaming, or Wails
  native feature interaction; those remain open or `blocked-env`. No mobile
  code was modified.

## 2026-09-16 Round R421 — Parallel detail/upload/model/playground parity

- Four independent Vue-baseline slices landed: spreadsheet (Excel/CSV/TSV)
  document preview with protected download/retry behavior (`bedcf465`),
  always-mounted upload confirmation sections matching Vue `v-show`
  (`bcb81ad2`), Ollama retest loading/management visibility (`6d5d58a6`),
  and API Playground trigger-focus restoration after close (`d843a633`).
- Integrated Web regression passed `1210/1210`; Web and shared typechecks
  passed. Shared tests initially exposed a package ownership defect: the UI
  interaction test directly imported `jsdom` without declaring it. The
  dependency was added to `@weknora/ui` and lockfile in `2cebab6a`; the
  corrected shared suite then passed `500/500`. Web and Embed builds passed.
- Build output retains existing large-chunk advisories, including the new
  spreadsheet preview chunk. These remain static/unit/component results;
  authenticated paired Vue/React browser screenshots, real backend
  permission/mutation flows, real Embed streaming, and Wails native
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R422 — Shared UI test dependency gate

- `packages/ui/src/interaction.test.tsx` was previously included in
  `test:shared` but resolved `jsdom` only through the workspace root. The
  package now declares `jsdom` directly in its development dependencies and
  the lockfile is synchronized (`2cebab6a`).
- After recreating workspace links, the shared suite completed `500/500` with
  no module-resolution failure. This improves the reliability of the
  Tailwind/shadcn project-component evidence; it does not change runtime UI
  behavior or replace browser/backend acceptance. No mobile code was modified.

## 2026-09-16 Round R423 — Parallel upload and settings parity

- Four independent Vue-baseline fixes landed: knowledge uploads now filter
  hidden folders, videos, and unsupported extensions before confirmation
  (`c02a8b2f`); personal memory shows an accessible loading state matching
  Vue `t-loading` (`e0753d9b5`); non-legacy Sandbox cards restore Vue click and
  keyboard navigation into the editor (`9bce4e81`); and `system-admin` is
  admitted to the Vue-equivalent Skill management branch (`959315db`).
- Scoped tests passed: upload 79/79, memory 12/12, Sandbox 29/29, Skill
  32/32. Integrated Web regression passed `1214/1214`; `pnpm typecheck:web`,
  `git diff --check`, Web build, Embed build, Desktop 9/9 tests, Desktop
  typecheck, and Desktop build all passed. Builds retain the known large-chunk
  and desktop CSS `@import` advisories.
- These are static/unit/component and build results. Authenticated paired
  Vue/React screenshots, computed-style sweeps, real backend upload/memory/
  sandbox/skill mutation and permission flows, and per-feature Wails/native
  interaction remain open or `blocked-env`. No Vue, mobile, Go, or pre-existing
  workbench migration changes were modified.

## 2026-09-16 Round R424 — Parallel graph, settings, model, and integrations parity

- Four independent Vue-baseline fixes landed: MCP mutation controls are now
  admin-only like Vue (`87432b20`); embedded Integrations preserves the tab
  selected by the user until the parent tab changes (`562a491a`); the Web
  knowledge graph now matches Vue frontier/familiar-node rings, responsive
  canvas structure, and uses the safe Markdown reader in its detail drawer
  (`d5362e3e`); and Model Debug reselects a valid type/model after catalog
  refresh (`fb8d5ff8`).
- Scoped tests passed: MCP 29/29, Integrations 62/62, Graph 13/13, Model
  Debug 47/47. Integrated Web regression passed `1219/1219`; `pnpm
  typecheck:web`, `git diff --check`, Web build, Embed build, Desktop 9/9
  tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real MCP/model/graph/
  integrations backend mutation and permission flows, and per-feature Wails
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R425 — Parallel platform, chat, wiki, and preview parity

- Four independent Vue-baseline fixes landed: Platform hydration now keeps a
  persisted same-user tenant override (`11ae9c1d`); Chat artifact lists remain
  downloadable when preview is unavailable (`727ab2ee`); Wiki readers now
  provide Vue-gated page deletion with confirmation, busy, failure, refresh,
  and selection-reset behavior (`8b517591`); and Mermaid document previews
  now render safely in the detail surface (`d05e314a`).
- Scoped tests passed: Platform 8/8, Chat 99/99 plus views 68/68, Wiki/editor
  15/15, Preview 8/8 plus detail 17/17. Integrated Web regression passed
  `1224/1224`; Web typecheck, `git diff --check`, Web build, Embed build,
  Desktop 9/9 tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real backend tenant/chat/
  wiki/file permission and mutation flows, and per-feature Wails interaction
  remain open or `blocked-env`. DOCX/PPTX previews remain download-only because
  the renderer dependencies are not present; no mobile code was modified.

## 2026-09-16 Round R426 — Parallel settings, FAQ, and administration parity

- Four independent Vue-baseline fixes landed: stored environment variables
  are no longer rendered in plaintext and new values use password controls
  (`b2ed4d8a`); Chat History hides the embedding-model row when disabled, as
  Vue does (`ad8e12f8`); FAQ CSV import now supports Vue Chinese/description
  headers, `##` multi-values, tags, and disabled-state inversion (`c388ee7b`);
  and system-admin routes mount the four Vue-aligned administration panels
  (`320dbc2d`).
- Scoped regression suites passed: EnvVar, Chat History, and administration
  focused tests; FAQ full suite 88/88. Integrated Web regression passed
  `1228/1228`; Web typecheck, `git diff --check`, Web build, Embed build,
  Desktop 9/9 tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real backend settings/FAQ/
  administration mutation and permission flows, and per-feature Wails
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R427 — Parallel route, FAQ, upload, and processing parity

- Four independent Vue-baseline fixes landed: same-path Settings history now
  uses React navigation instead of an unconditional reload (`537ef203`); the
  development Markdown fixture omits the custom result for blank input like
  Vue (`61b440cb`); knowledge upload progress rejects the invalid zero KB
  identifier (`d580d4cc`); and processing timeline polling preserves Vue's
  post-terminal quiesce grace for delayed work (`1bb49af7`).
- Shared regression passed `500/500`; integrated Web regression passed
  `1231/1231`; Web typecheck, `git diff --check`, Web build, Embed build,
  Desktop 9/9 tests, Desktop typecheck, and Desktop build all passed.
- Evidence remains static/unit/component/build only. Authenticated paired
  Vue/React screenshots and computed-style sweeps, real backend route/FAQ/
  upload/processing permission and mutation flows, and per-feature Wails
  interaction remain open or `blocked-env`. No mobile code was modified.

## 2026-09-16 Round R428 — Five parallel slices, reviewer gate, and first authenticated paired browser evidence

- Worktree `.worktrees/react-multiclient` (branch `codex/react-vue-parity-align`, base `edc853b2`); real backend docker `:8080`, Vue dev `:5180`, React dev `:5181`; parity account on tenant 10000 (owner).
- Five parallel worker slices plus one reviewer agent landed: KB-list icons/rail/share geometry (R009/N004/N005), graph node-radius/worker-pool/legend/frontier with +14 tests (N012), a new Vue-shaped `TenantAuditDrawer` (drag-width persistence, Esc, focus trap) wired into `TenantMembersPanel` (R038), SkillSettingDrawer headers and sandbox pick-list cleanup (R033), and five-locale chat thinking-state copy keys in `chat-copy.ts` (N020, keys-first, no consumer yet).
- Shared locale fix (R007/N001): new `apps/web/src/locale.ts` exposes `usePreferredLocale()` reading the stored `locale` preference (default zh-CN, Vue GeneralSettings parity) and listening to `weknora:locale-changed`; `App.tsx`, `PlatformShell.tsx`, `InvitationInbox.tsx`, `AgentsPage.tsx` dropped `navigator.language` resolution. Live browser proof: platform nav switched from English to Vue-matching Chinese labels.
- Reviewer fixes: graph render null guard (TS18047), jsdom `requestAnimationFrame` fallback, audit-drawer portal focus timing (`mounted` dependency), DrawerTitle headers wired into the three skill dialogs, residual `backendLabel` prop removal, and two test files updated to portal/controlled semantics.
- Gates: scoped suites KB 102/102, graph 14/14, members 15/15, skills/MCP 61/61, chat 53/53, shell 39+20+29 and kb-list-anatomy 18/18 all pass; `pnpm typecheck:web` clean; integrated `pnpm test:web` 1291/1291 and `pnpm test:shared` 568/568 with 0 failed/cancelled/skipped.
- First authenticated paired browser evidence captured at 1440×900/zh-CN/light (`evidence/vue-react-parity/2026-09-16-r428-browser-parity-pair.md`, 12 screenshots): open diffs — sidebar sessions stuck on English "Loading..."; MCP empty state missing the Vue admin add tile; skills/audit empty states missing illustration + "暂无数据" and the skills heading help icon; KB create dialog missing the radio-group border frame, Wiki "NEW" badge and 0/200 counter. Graph canvas same-condition comparison is `blocked-fixture` (needs a wiki-type KB); React `?tab=graph` is the wiki page-link graph, distinct from Vue's GraphSettings config surface.
- Not staged: `docs/superpowers/plans/mobile-workbench-progress.md` and other files modified by an external concurrent process (apps/mobile, internal/handler/auth.go, packages/api-client/src/auth/oidc.ts) — untouched and unreviewed by this round. No Vue, mobile, or Go code was modified by this round.

## 2026-09-16 Round R430 — Five parallel slices, SPA navigation, and fix-confirmed paired browser evidence

- Five Vue-baseline fixes landed on top of R429 in worktree `.worktrees/react-multiclient` (branch `codex/react-vue-parity-align`), closing all four open diffs from R428's browser evidence: (1) chat session sidebar now renders grouped real sessions instead of a stuck English "Loading..." card (`packages/views/src/chat/session-sidebar.tsx`, loading state scoped to the initial fetch only); (2) MCP settings empty state renders the Vue dashed "添加服务" admin tile for owner/admin roles (`McpSettingsPanel.tsx`); (3) a shared `settings/EmptyState.tsx` reproduces the TDesign `t-empty` anatomy (48px illustration + locale "暂无数据" title + description) and is wired into `SkillSettingsPanel` and the `TenantMembersPanel` audit drawer, with the skills panel heading + help icon owned by the panel itself (`SettingsPage.tsx` heading exclusion extended to `skills`); (4) KB create dialog anatomy matches TDesign: joined radio-group border frame with brand-green checked divider and arrow-key roving focus, Wiki NEW badge, and the `0/200` description counter (`App.tsx`, new `kb-create-dialog-anatomy.test.tsx`).
- Route-level SPA navigation (R007/N001 follow-up): new `apps/web/src/platform/navigation.ts` installs a `pushState`/`replaceState`/`popstate` observer; `main.tsx` keeps the mounted React tree and re-resolves the route via `subscribeNavigation` instead of the previous unconditional `popstate` reload, so in-app transitions no longer remount the shell (`PlatformShell.tsx`, `adapters.ts` updated accordingly).
- Wiki graph fixture unblocked (R428 item 5): created wiki-enabled KB `7cea6ec0-8a07-4c83-9309-f802a61b3d5c` (3 interlinked pages → 3 nodes / 5 edges) via the documented wiki API contract (`2026-09-16-r430-wiki-graph-fixture.md`); same-condition canvas comparison now complete.
- Gates: integrated `pnpm test:web` 1305/1305, `pnpm test:shared` 568/568, `pnpm typecheck:web` clean.
- Authenticated paired browser evidence (1440×900, zh-CN, light, real backend :8080): `evidence/vue-react-parity/2026-09-16-r430-browser-parity-pair.md` + 12 screenshots (`screenshots/r430-20260916/`, 6 Vue/React pairs) confirm all four R428 diffs closed and the wiki graph canvas parity (node colors, legend, fit/arrows controls, overview count, search + help).
- New diffs found in R430 browser pass, deferred to R431 (N012/N004 domain): graph tab header information architecture (Vue embeds breadcrumb + 文档/Wiki/图谱 tabs inside the KB page; React `?tab=graph` is a standalone page header), graph search control shape (Vue select-style dropdown arrow vs React plain input), edge arrow visibility under identical conditions (React edges render without directional arrows), and Vue's one-time KB-onboarding guide overlay is not implemented in React. Not staged: `apps/desktop/vite.config.ts` and other files owned by the external concurrent craft/workbench process — untouched by this round. No Vue, mobile, or Go code was modified.

## 2026-09-16 Round R431 — Four parallel slices closing the R430 browser diffs

- Four Vue-baseline fixes landed in worktree `.worktrees/react-multiclient` (branch `codex/react-vue-parity-align`), closing all four diffs R430's browser pass deferred to this round: (1) graph tab header information architecture — `KnowledgeGraphPage.tsx` now renders the KB-embedded header via the shared `DocumentsBreadcrumb` (知识库 > KB name > 文档/Wiki/图谱 tab row with brand-green active tab, `aria-current`, `tabGraphTip` tooltip, ⓘ info popover, ⚙ gated by `resolveKBSurfaceTabs`/`canManage` and landing on the existing KB settings route) plus the unconditional documents subtitle, replacing the standalone "知识图谱" heading; (2) graph search control shape — rebuilt as a filterable-select combobox shell (search prefix icon, rotating chevron, 32px/4px radius/`--td-shadow-1`), empty-keyword dropdown falls back to a node snapshot mirroring Vue's `graphSearchEffectiveOptions`, with combobox ARIA and full keyboard navigation; (3) edge arrow visibility — root cause was center-to-center lines painted under nodes that fully covered the markers; `graphEdgeEndpoints()` replicates Vue `setEdgePositions` endpoint retraction (radius+4), marker fill moved to a direct attribute, toggle icon now an inline eye SVG; (4) KB one-time onboarding guide — React guide UI already existed; added `shouldArmKbDetailGuideOnEntry` (mirrors `KnowledgeBase.vue:339-345`: non-FAQ, editable, loaded, empty KB), the `useKbDetailGuideTrigger` hook wired on the documents page, and the `data-guide="kb-detail-add-doc"` step-2 target.
- A canvas regression introduced en route (shortened subtitle exposed `mx-auto` shrink-to-fit in the flex-column shell, collapsing the canvas to a ~544px centered column) was found in paired browser verification and fixed; the canvas is now a full-bleed flex area with a ResizeObserver-synced viewBox (measured 1114×798), matching Vue's `.wiki-main-area`.
- Gates on the final tree: `pnpm typecheck:web` clean; `pnpm test:web` 1318/1318; `pnpm test:shared` 568/568; web/embed/desktop builds pass; desktop tests 9/9; `git diff --check` clean.
- Authenticated paired browser evidence (1440×900, zh-CN, light, real backend :8080): `evidence/vue-react-parity/2026-09-16-r431-browser-parity-pair.md` + `screenshots/r431-20260916/` (guide firing on KB entry, aligned graph header/search/legend, full-page and zoomed arrow shots vs the Vue pair) confirm all four diffs closed and the canvas layout regression fixed.
- Recorded residuals for R432 (non-blocking): ⚙ opens the existing KB settings route instead of Vue's global settings drawer surface; tab-row gating uses `resolveKBSurfaceTabs` (wiki-off/graph-on KBs show 文档/图谱) vs Vue's strict isWiki; React search input still filters canvas nodes and uses 250ms/2-char debounce vs Vue 200ms/any-char; Vue's selected/hover edge highlight interaction (applyHighlight + highlight markers) is not implemented; the guide trigger lives on the documents page so a direct `?tab=graph` deep link does not arm it; tab switching is link navigation across three React pages vs Vue's in-component tab state.
- Provenance/dependency notes: the four slices plus the concurrent SPA-navigation/KB-page work were committed together by the concurrent orchestrator as `6139859a`; the graph page header depends on the `DocumentsBreadcrumb` tabs slot introduced there (a rollback of `DocumentsPageChrome.tsx` would break the header compile). The canvas-layout final delta was committed separately by this round. No Vue, mobile, or Go code was modified.

## 2026-09-16 Round R431 — Graph tab chrome, search select shape, arrows, KB onboarding guide, and full-bleed canvas

- Four R430 browser diffs closed on `codex/react-vue-parity-align`: (1) the graph page dropped its standalone
  eyebrow/h1 header for the Vue breadcrumb chrome — 知识库 > KB名 > 文档/Wiki/图谱 tab row (graph tab active with the
  tabGraphTip tooltip), KB info ⓘ + settings ⚙ actions, and the unconditional upload subtitle, reusing/extending
  `DocumentsBreadcrumb` (Vue KnowledgeBase.vue:2330-2408); the documents page tab links now use `?tab=` query URLs so
  the graph link no longer 404s; (2) the graph search control renders the Vue t-select shape (search prefix icon +
  chevron suffix, 320px absolute top-left overlay) while keeping the remote-search dropdown semantics
  (WikiBrowser.vue:12-17); (3) SVG edge arrows now render with direct `fill="#c0c4cc"` marker attributes (the Tailwind
  arbitrary class was not applied inside marker paths), `showArrows` defaulting on with bidirectional `marker-start`
  (WikiBrowser.vue:3877-3968); (4) the Vue kbDetail onboarding guide (SpotlightGuide: 3 steps intro/upload/done,
  `1 / 3` progress, storage key `weknora:contextual-guide-kb-detail:v1`, 600ms delay, `[data-guide="kb-detail-add-doc"]`
  spotlight) is wired through the shared ContextualGuideHost with host-level tests.
- This session's paired-browser pass then surfaced one more real diff — React's graph canvas was a fixed ~500px card
  while Vue's `.wiki-graph` fills the whole content area — fixed by measuring the surface with a ResizeObserver,
  feeding the real box into `layoutGraphNodes`, the dynamic `viewBox`, and the pointer mapping, and making the page a
  full-width/full-height flex column (main padding 24px/32px/0, surface flex-1).
- Live paired-browser evidence (1440×900, zh-CN, light, real backend :8080, wiki fixture KB
  `7cea6ec0-8a07-4c83-9309-f802a61b3d5c`, 4 nodes/5 edges): breadcrumb chrome, tab row, ⓘ/⚙, subtitle, select-shaped
  search, visible directional arrows, legend/actions/status, and the full 3-step guide flow (next/prev navigation,
  upload-button spotlight, close persists `storageKey='1'`) all match Vue
  (`evidence/vue-react-parity/screenshots/r431-20260916/`: react/vue graph fullbleed + guide step1 pairs).
- Gates: `pnpm test:web` 1318/1318, `pnpm test:shared` 568/568, `pnpm typecheck:web` clean. Note: an external
  concurrent process committed most of this round's slices as `6139859a` mid-flight (same worktree); this entry's
  commit adds the full-bleed canvas fix and the paired-guide/fullbleed screenshots on top of it.
- New diffs deferred to R432: documents tab renders its 文档/Wiki/图谱 tab row as a standalone top-right nav + 刷新
  button instead of Vue's inline breadcrumb tabs (graph tab is correct); guide card placement can overflow the right
  viewport edge at 1440px on the upload step (Vue has right/left/bottom/top fallback); React fit-to-view resets
  viewport instead of computing the Vue bbox fit. An accidental write to the main worktree early in this round was
  fully reverted (main `git status` clean except pre-existing untracked docs). No Vue, mobile, or Go code was modified.

## 2026-09-16 Round R432 — Documents breadcrumb tabs inline, graph search Vue behavior

- Documents tab header now matches Vue's information architecture
  (KnowledgeBase.vue:2330-2408): the 文档/Wiki/图谱 row renders inline as the
  third breadcrumb level via the shared `DocumentsBreadcrumb` tabs anatomy
  (documents active, graph tab carrying the tabGraphTip tooltip), and the
  standalone top-right `wk-kb-tabs` nav is gone. The manual 刷新 button was
  removed with it — Vue's document header has no reload action; list refresh
  stays driven by upload/processing watchers (`reloadToken` mechanism kept
  intact in state). Clicking the breadcrumb 图谱 tab SPA-navigates to
  `?tab=graph` and renders the graph surface (verified live).
- Landed alongside (concurrent orchestrator slice, same R432 backlog item):
  graph search now mirrors Vue's remote-search semantics — 200ms debounce, any
  non-empty keyword, empty keyword restores the top-500 snapshot, and typing
  never filters the canvas (query removed from the visible-nodes filter; the
  canvas narrows only through the type allow-list, WikiBrowser.vue:4680-4712).
- Gates: `pnpm test:web` 1322/1322, `pnpm test:shared` 568/568,
  `pnpm typecheck:web` clean.
- Paired-browser evidence (1440×900, zh-CN, wiki fixture KB
  `7cea6ec0-8a07-4c83-9309-f802a61b3d5c`): breadcrumb tab row, ⓘ/⚙, subtitle,
  and the absence of top-right actions match Vue on the documents tab
  (`screenshots/r432-20260916/`, react/vue pair).
- Remaining R432 backlog (from the r431 evidence doc): guide card placement
  can overflow the right viewport edge at 1440px on the upload step; React
  fit-to-view resets viewport instead of computing the Vue bbox fit; ⚙ lands
  on the KB settings page instead of Vue's settings drawer; guide trigger is
  documents-page-only (deep link `?tab=graph` does not arm it); Vue
  selected/hover edge highlight (applyHighlight) unimplemented. No Vue,
  mobile, or Go code was modified.

## 2026-09-16 Round R433 — Graph applyHighlight, strict isWiki gating, in-place ⚙, deep-link guide

- Graph selection/hover highlight now mirrors Vue's applyHighlight/clearHighlight
  (WikiBrowser.vue:4558-4635): `graphHighlightSets(edges, selectedSlug, hoveredSlug)`
  computes the focus/enlarged/lit sets; edges incident to a focus light up with the
  focus node's type color (hover focus wins over selection), opacity 0.9, width 2,
  and the `#0052d9` highlight arrow markers (`wk-graph-arrow-{end,start}-hl`); every
  other edge fades to 0.08/width 1 with plain markers; focus nodes grow r+3 /
  stroke-width 3, undirected neighbors keep full opacity, the rest fade to 0.2; the
  selected node carries the pulsing active ring (`wk-node-active-pulse` keyframes in
  styles.css, transform-origin pinned to the node center because the React renderer
  uses absolute cx/cy instead of Vue's per-node translate group); a near-stationary
  click on the canvas background clears selection, hover, drawer, and highlight.
- Tab gating is now the strict Vue isWiki contract (KnowledgeBase.vue:89,2359-2381):
  `resolveKBSurfaceTabs` returns all three tabs only when `wiki_enabled`, otherwise an
  empty row and the plain 文档 crumb — and `kbWikiTabFallbackPath` redirects non-wiki
  `?tab=wiki|graph` deep links to the canonical documents URL, matching Vue keeping
  the URL but rendering the documents branch.
- The breadcrumb ⚙ now opens the KB settings surface in place where the host page
  provides `onOpenSettings` (Vue uiStore.openKBSettings), falling back to the
  historical settings route otherwise.
- The kbDetail welcome tour now arms on graph deep links too (`useKbDetailGuideTrigger`
  wired into KnowledgeGraphPage off `kbMeta.knowledge_count`), closing the
  "documents-page-only trigger" gap.
- Gates: `pnpm test:web` 1330/1330, `pnpm test:shared` 568/568, `pnpm typecheck:web`
  clean.
- Paired-browser evidence (1440×900, zh-CN, wiki fixture KB): selecting the summary
  node lights its incident edges in the type blue with directional arrows, fades the
  unlinked edge and the Index node to near-invisible, and keeps 概念/实体 neighbors
  at full color — identical on both sides
  (`screenshots/r433-20260916/`: vue/react select-highlight pair + react hover
  secondary focus; `r432-20260916/react-kb-guide-graph-deeplink.png` from the
  concurrent slice). No Vue, mobile, or Go code was modified.

## 2026-09-16 Round R435 — Code-parity TDD round: settings surface, doc retry contract, org gating, chat approval args

- Five parallel agents (A1 knowledge-settings, A2 document-detail, A3 metadata/org, A4 chat, A5 verifier) in one
  dispatch, file-domain mutually exclusive, all implementation slices red→green TDD. R434 numbering was left to
  the external concurrent orchestrator (its graph fit-to-view / guide-placement WIP was in flight in this
  worktree and was not touched by this round).
- A1: legacy `knowledge-settings/KnowledgeSettingsPage.tsx` now matches the Vue inline editor contract
  (KnowledgeBaseEditorModal.vue:423-434) — datasource mounts the live `DataSourcesPage` (canManage gating), share
  mounts `KnowledgeBaseShareDialog` inline, activity mounts `KnowledgeBaseActivityPanel`, and
  `loadKnowledgeSettingsOptions` loads parser/vector/storage catalogues from the authenticated settings API with
  per-endpoint degradation; parser select lists live available engines, vector/storage selects render the Vue
  read-only edit-state shape. Verifier caught a cross-domain regression (synchronous catalogue calls broke the
  graph page's sparse mock client, test:web 1339/1340); fixed with lazy thunks +
  `loadKnowledgeSettingsOptionsToleratesMissingMethods` regression test → 1340/1340. Deferred: legacy surface
  save pipeline (App.tsx aligned modal still owns full save semantics).
- A2: document-detail retry-index feedback now matches Vue `doc-content.vue` — 「重试索引」copy,「索引已同步」
  success /「索引同步失败」still-failed feedback, dedicated `retryingId`/`retryNotice` so retry loading no longer
  disables the enable/disable toggles, localized title/aria, viewer gating. Scoped documents regression 199/199.
  Accepted delta recorded: Vue icon+tooltip+toast vs React text button + inline Status (R370-established shape).
- A3: structured metadata-row editing verified already closed in R370 (no change; domain constraint honored).
  Fallback slice: org role-upgrade request now gated per Vue `OrganizationSettingsModal.vue` via
  `canRequestUpgradeForOrg` (editing + non-admin space role + tenant admin+) with role options converged
  (`upgradeRoleOptionsForRole`: viewer→editor/admin, editor→admin). Deferred: `hasPendingUpgrade` needs an
  api-client field.
- A4: tool-approval args editing matches Vue `ToolApprovalCard.vue` — per-keystroke JSON validation
  (`approvalArgsStatus.isJsonValid`) disables approve with inline alert, `argsModified`「已修改」status, reject
  carries localized `reason` (`userRejected`), button order reject · approve, new 5-locale keys
  `approvalArgsModified`/`approvalRejectedReason`. Deferred: Vue approval countdown. Ten-state CHAT-SURFACE audit
  recorded: other nine states verified aligned.
- Gates: `pnpm test:web` 1340/1340, `pnpm test:shared` 570/570, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Code-contract round — no browser pairing; live capture for the touched surfaces (KB settings dialog, doc
  retry, org settings, chat approval) deferred to the next browser round. Evidence:
  `evidence/vue-react-parity/2026-09-16-r435-code-parity-round.md`. No Vue, mobile, or Go code was modified.

## 2026-09-17 Round R436 — Code-parity TDD round: approval countdown, settings save pipeline, doc preview gates, org pending-upgrade

- Five parallel agents (A1 chat countdown, A2 settings save pipeline, A3 doc preview/merge, A4 org
  hasPendingUpgrade, A5 verifier), file-domain mutually exclusive, all red→green TDD. A5 verdict: A1/A2/A4 PASS;
  A3 CONCERNS resolved by an orchestrator follow-up verified against live backend data (see below).
- A1: tool-approval countdown matches Vue ToolApprovalCard.vue — deadline from requested_at+timeout_seconds
  (SSE events confirmed in internal/event/event_data.go, bridged in approval-state.ts/ChatRoutePage), m:ss vs
  {seconds}s below 60s, 30s/120s critical/warning classes, expiry parks at 0 without disabling, resolved hides
  the timer, 5-locale `approvalCountdownShort`.
- A2: legacy `/knowledgeBase/:id/settings` save pipeline — `PUT /initialization/config/:kbId` with the Vue
  KBModelConfigRequest payload (KB round-trip, pending parser rules, vector_store_id immutable-excluded),
  aria-busy + duplicate-click guard, failure keeps the form, owner/admin gating, zero new i18n keys.
- A3: merged-content assembly now ports Vue mergeChunks/appendChunkContent (MIN_OVERLAP=12, start_at order) with
  pagination in BOTH 全文 and 分块 views; `canPreviewDocument` ported (audio excluded, 全文 default). Orchestrator
  follow-up on verifier CONCERNS: the gate and the detail-title extension strip were re-keyed from `source` to
  `type === 'file'` — real payloads carry `type: "manual"/"file"` and `source: ""` for file uploads (verified
  live on KB 22d38cb7 doc a35e5ca0: old fixtures used a nonexistent `source: 'file'` shape); file_type now wins
  over the filename suffix per Vue resolveFilePreviewExt; `type?: string` added to the shared KnowledgeDocument
  contract.
- A4: org upgrade request now reflects `has_pending_upgrade` from the detail endpoint (api-client field +
  normalization, stale-guarded modal fetch, submit disabled + 审核 title/aria, current-role tag bar, local pending
  set after submit).
- Gates: `pnpm test:web` 1359/1359, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
- Browser evidence: paired manual-doc detail (no preview tab / 全文 default / `.md` title kept) on both sides
  `screenshots/r436-20260917/react|vue-doc-detail-manual-type-gate.png`; React settings save round-trip with
  配置保存成功 `react-kb-settings-save-success.png`. Deferred to the next browser pass: Vue KB editor-modal pair
  (entry point behind the KB list card hover menu — not located in timebox), live approval countdown flow, org
  pending-upgrade live check. No Vue, mobile, or Go code was modified.

## 2026-09-17 Round R437 — Doc pagination/preview gates, datasource lifecycle, FAQ batch bar + browser sweep

- Five parallel agents (A1 doc-detail residuals, A2 datasource sweep, A3 FAQ+KB-list sweep, A4 browser-evidence
  only, A5 verifier), file-domain mutually exclusive, TDD; orchestrator closed the verifier's A2 CONCERNS
  in-round. A5 verdicts: A1 PASS, A3 PASS, A4 PASS, A2 PASS-after-closure.
- A1: chunk pager transition mirrors doc-content.vue (header+pager stay mounted, `chunk-page-loading` row swap,
  disabled-while-transition guard, failure keeps the loaded page with retry, `(page-1)*25+i+1` numbering;
  ledger correction — Vue HIDES the old page during the turn rather than keeping it). parse-status gating
  removed where Vue has none: preview readiness is `type==='file' && inline kind`, audio player loads
  unconditionally; Vue-absent availability/downloadOnly model fields + dead 5-locale strings removed.
- A2: create branch auto-triggers the first sync (createAndSyncSuccess / createButSyncFailed warning), edit
  branch keeps the updateSuccessSyncHint warning; per-field `${label} ${isRequired}` validation blocks before
  the connection test; type-step title localized. Closure: Vue credentialsRequired exemption added —
  `credentialsRequiredForValidation` skips the required walk when editing a configured connector without typed
  replacements (backend `credentials.credentials.configured` confirmed at datasource_credentials.go:85-91;
  missing flag falls back to validation, matching Vue's optional chain). Deferred: render-level behavioral
  tests for DataSourcesPage (current ones are source-regex pins), list-card anatomy, credential-step
  Replace/Remove, React's Vue-absent per-row 测试连接 button.
- A3: FAQ batch enable/disable buttons render conditionally per FAQBatchBar.vue:53-63 with `is_enabled !==
  false` counting. KB list audit clean — the R436 entry question closed: Vue card menu 设置 has the React
  counterpart openKbSettings → in-place edit dialog. Deferred: shared-KB detail drawer (Vue
  KnowledgeBaseList.vue:710-776; data+i18n ready, mount in App.tsx), FAQ batch-delete canContribute vs Vue
  canManage (backstopped by backend RBAC; permissions.ts externally occupied).
- A4 browser sweep (11 paired screenshots, `screenshots/r437-20260917/`): Vue KB editor modal entry = card
  `.more-wrap` → menu 设置 → Teleport overlay 1000×750 (5 groups/13 items, 保存并关闭) vs React legacy flat
  7-section settings page — the largest known settings-surface gap, documented for future rounds; org route is
  `/platform/organizations` and upgrade gating renders per contract (parity account is org admin, so no form
  on either side — DOM-verified); approval countdown not witnessed (sandbox SSRF guard blocks the test model).
- Gates: `pnpm test:web` 1370/1370, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r437-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R438 — Shared-KB drawer, settings grouped IA phase 1, datasource cards, agents sweep

- Five parallel agents (A1 shared-KB drawer, A2 settings IA phase 1, A3 datasource card anatomy, A4 agents
  sweep, A5 verifier), file-domain mutually exclusive (A1 exclusively owned App.tsx), TDD. A5 verdicts:
  A1/A2/A4 PASS; A3's one gate regression (i18n key-count guard 164→169) fixed by the orchestrator before
  commit.
- A1: shared-KB detail drawer ported (KnowledgeBaseList.vue:305-315 entry on non-owned shared cards, :709-776
  drawer) — SharedKnowledgeBaseDrawer with agentKbStrategyKey/formatSharedAt/permissionTone, App.tsx openShared
  Detail re-resolves the share row by share_id (restoring source_from_agent lost in merge flattening), 进入知
  识库 navigates knowledgeBaseDetailPath. 6 jsdom behavioral tests red→green; kb regression 37/37.
- A2: legacy KB settings surface restructured to the Vue KnowledgeBaseEditorModal grouped IA (navGroups
  L637-669): basic / processing(索引与解析) / data(存储与数据, datasource badge) / integration(发布集成) /
  management(管理与审计, owner-admin gated), pickItems order, empty-group filter, 14 inline SVG t-icons,
  default section basic, .wkbs-* shell with the Vue modal computed styles (1000×750, 208px sidebar, #07c05f
  active). Unported sections show the existing notYetPorted placeholder — no invented editors; 39/39 directory
  regression; 19 i18n keys pre-existing ×5 locale. Phase-2 backlog: basic/models, chunking/multimodal/asr/
  advanced, faq editors, share !isLiteMode gate.
- A3: cronHumanize ported (5 presets + raw fallback, relativeTime buckets, syncResultPills +N/~N/-N); Vue card
  grid (320px auto-fill, icon badge, status dot, colored pills, dashed add-card); scheduleHuman keys ×5 locale
  byte-identical to Vue. Fixed the 164→169 i18n guard. Deferred: jsdom render tests (page assertions remain
  source-regex pins), connector image assets, ellipsis action grouping.
- A4: agents space-view contract fixes — 我共享的 cards open the editor on click with no three-dot menu
  (opensEditorOnCardClick), spaceAgentsLoading spinner suppresses the premature empty state, org pill excluded
  on the space tab; agents domain 88/88. Deferred: DB-backed favorites, section=im/embed/integrations redirects.
- Gates: `pnpm test:web` 1398/1398, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r438-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R439 — Settings editors phase 2, administration/appconnector/platform-settings sweeps

- Five parallel agents (A1 settings editors phase 2, A2 administration, A3 integrations/appconnector, A4
  platform settings, A5 verifier), file-domain mutually exclusive, TDD. A5 verdicts: A1/A2/A4 PASS; A3's dead
  code (catalogRiskTone never consumed) wired by the orchestrator in-round.
- A1: three editor sections migrated into the grouped IA against KnowledgeBaseEditorModal.vue — models (live
  /api/v1/models catalogue as a fourth loadKnowledgeSettingsOptions path, modelDefaults exclusion, saves
  llmModelId/embeddingModelId), chunking (strategy select + sliders + overlap warning + separators + parent-child
  coupling, saves documentSplitting), advanced (questionGeneration toggle/count/instructions). Payload builder
  gained an optional overrides arg; override-free calls keep the exact R437 round-trip. Directory 50/50; zero
  new i18n copy. Phase-3: multimodal/asr/faq, wiki synthetic model row, autoTag, KBChunkingDebug.
- A2: administration member search (320ms debounce + server q + page reset + search-aware empty state) and
  invite default role viewer→contributor per TenantMembers.vue; 2 mirrored keys ×5 locale. Deferred: management
  pagination UI, popconfirm shape, two-step invite.
- A3: Vue authority clarified — Apps four views exist in the current checkout, not only at 9b0c11c4. Fixed:
  revoke CAS conflict branch (409/VERSION_CONFLICT → warning + reload), connection/app state badges with Vue
  vocabulary + tones, published/risk/installation labels; catalogRiskTone wired into the AppsPage risk badge
  (verifier caught it as dead code). Deferred: schema-digest shape, ellipsis grouping, React's extra authorize
  busy-guard kept as a documented safety-side deviation.
- A4: platform settings sections/nav/save-semantics verified aligned; added the Vue success toast after each
  accepted preference change (shared keys ×5 locale). Deferred G2: Vue persists preferences per-user
  (WeKnora_${userId}_*) while React uses flat keys — cross-user crosstalk risk, fix belongs in
  packages/domain local-preferences.ts + theme.ts.
- Gates: `pnpm test:web` 1423/1423, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r439-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R440 — Settings phase 3, per-user preferences, wiki editor contract, embed bridge sweep

- Five parallel agents (A1 settings phase 3, A2 G2 preference namespacing, A3 wiki sweep, A4 commercial/embed
  sweep, A5 verifier), file-domain mutually exclusive, TDD. A5 verdict: 4/4 PASS, zero new failures; the
  orchestrator closed the script gap A5 surfaced.
- A1: multimodal/asr/faq editors migrated, completing the settings section set except basic. faq_config does
  NOT travel in KBModelConfigRequest — it follows Vue doSubmit as a base update PUT /knowledge-bases/:id
  ({name,description,config:{faq_config}}) issued before the config PUT; disabled-clears-model_id semantics
  ported for vlm/asr; multimodalInvalid/indexModeRequired guards jump to the offending section. 59/59 directory
  regression; zero new i18n keys. Remaining: basic section; document-KB wiki/auto_tag/indexing_strategy
  base-update wiring (channel ready).
- A2 (G2 closed): preferences now persist per-user as WeKnora_${userId}_* (userId from weknora_user else anon)
  with a one-shot migration latch (anon > Vue legacy flat > React flat priority, source keys removed,
  resetMigrationLatch for account switches); locale intentionally stays flat on both sides. domain settings
  tests 16/16; consumers 58/58. Deferred: GeneralPreferencesPanel direct font-key reads; platform layer has no
  weknora_user writer yet (anon namespace until then).
- A3: wiki edit cancel + 409 conflict overwrite (覆盖保存 with latest.version, reload injects server content
  into the editor instead of exiting) per WikiBrowser.vue. Deferred: markdown/wiki-link rendering (single-round
  candidate), list meta updated_at vs v{version}, issue/queue/backlink endpoints missing in api-client.
- A4: commercial confirmed fork-owned with NO Vue counterpart (zero changes, 8/8 recorded). embed bridge post()
  now carries Vue postToParent's sensitive-drop / handshake-'*' fallback; origin pinning, host validation, error
  mapping already matched. Deferred: React has no embed page at all (standalone-entry error at /embed/*; no
  embed.html) — standalone-round candidate.
- Orchestrator: root test:shared never globbed packages/domain/src/settings/*.test.ts (A5 finding) — glob
  added; shared gate now 590/590 (+16) so the G2 regression tests are gate-protected.
- Gates: `pnpm test:shared` 590/590, `pnpm test:web` 1437/1437, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r440-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R441 — Settings basic complete, wiki markdown, embed entry, per-user auth writes + browser sweep

- Six parallel agents (A1 settings finish, A2 wiki markdown, A3 embed entry, A4 preference/auth writes, A5
  browser-evidence only, A6 verifier), file-domain mutually exclusive (A3 alone owned the app entry files), TDD.
  A6 verdict: A1-A5 all PASS, zero regressions; the orchestrator closed A5's live-data chunking defect in-round.
- A1: `basic` section ported (id+copy, type radio, indexing-strategy checkboxes with vector→keyword coupling,
  wiki granularity/instructions, name required/50, description/200; nameRequired joins the validateForm order
  with a section jump). Document-KB base updates carry
  `{name,description,config:{wiki_config,auto_tag_config,indexing_strategy}}` and now ALWAYS precede the config
  PUT (Vue doSubmit order; FAQ sends only faq_config). A5's nav-order observation re-audited: R438 was already
  correct — no change. Directory 68/68; zero new i18n keys. Deferred: isIndexingLocked, wiki NEW badge, models
  required validation.
- A2: wiki reader now renders Vue's chain (wiki-link pre-process → marked breaks → DOMPurify → click-delegated
  navigation) at the SAME dependency versions as the Vue frontend (marked ^17.0.5, dompurify ^3.4.11); DOMPurify
  3.4.11 USE_PROFILES behavior verified live (rel-only anchors, no target=_blank); slugs HTML-escaped pre-parse.
  27/27. Deferred: image preview, index-view markdown, reader footer.
- A3: `/embed/*` mounts a real embed entry (app-entry branch + index.html /embed/ light-theme injection as the
  single-document equivalent of Vue's separate embed html): bootstrap handshake, primary_color theme injection,
  viewport fill, minimal composer+SSE chat via the R440 bridge. 7 tests red→green; build emits the embed chunk.
  Deferred: history backfill, references/suggestions, real-iframe e2e.
- A4: panel font keys moved onto the per-user path (domain read/writeUserPreference, flat keys join the
  migration sources); `weknora_user` now written/cleared like Vue stores/auth.ts — persistLogin (three login
  paths), /auth/me hydrate, logout/401 clears, latch reset wired. Full u1/u2 localStorage-sequence regression
  71/71. Note: pnpm file: deps copy-install — rerun pnpm install after domain edits.
- A5 browser sweep (9 screenshots, r441-20260917/): settings grouped IA matches Vue (basic was placeholder at
  capture, closed by A1 same round); datasource add-card matches (no rows to exercise cron/pills); no shared-KB
  fixture so the drawer pair was not capturable. Recorded diffs: separators listbox vs tag chips, missing
  测试分块效果/Embedding warning/200K badge, per-section save vs Vue single save-and-close, English eyebrow
  duplication, Vue legacy /space/knowledgeBase blank page.
- Orchestrator closure: A5's live defect — React showed chunk overlap 0 where Vue showed 80 on the same KB —
  root-caused to Vue's `||` seed (chunk_overlap 0 → DefaultChunkOverlap 80, backend chunker.DefaultChunkOverlap)
  vs React's typeof guard letting 0 through; fixed with `Number(chunk_overlap) || 80` plus the separators
  empty-array-kept semantics; regression test pins stored 0→80 and []→kept. The "512 字符 vs aria 500" report
  could not be reproduced in code (slider text renders the same value as aria-valuenow) — noted for the next
  browser pass.
- Gates: `pnpm test:web` 1473/1473, `pnpm test:shared` 596/596, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r441-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R442 — Chunking UI finish, wiki reader closeout, embed chat parity, platform shell sweep

- Five parallel agents (A1 knowledge-settings finish, A2 wiki reader closeout, A3 embed chat parity, A4 platform
  shell sweep, A5 verifier), file-domain mutually exclusive, TDD. A5 verdicts: A1/A2/A4 PASS; A3's CONCERNS
  (referenceHeadline chunk-vs-group counting) fixed by the orchestrator in-round. Final: all PASS, zero
  regressions vs baseline 1473.
- A1: separator control rebuilt as Vue tag chips (Enter add / Backspace pop / Esc clears draft); 「测试分块效果」
  preview entry ported (720px drawer, four samples verbatim from Vue chunkingSamples.ts, existing
  previewChunking API, tier normalization, six-tile profile); models validation joins validateForm (embedding
  required when vector||keyword, LLM always). Ledger corrections: the "200K badge" does not exist in the Vue
  repo and the Embedding-locked warning binds to ragEnabled&&hasFiles, not keyword-only — blocked items
  corrected. isIndexingLocked honestly blocked (KB payload lacks a files signal; Vue derives it from a separate
  knowledge-list GET — plan recorded, needs its own round). Directory 72/72; zero new i18n keys.
- A2: wiki image preview dialog, index view through the markdown pipeline in the reader pane, and the reader
  footer 「Linked from」/「Source documents」 all ported (Vue footer L591-612; backend fields confirmed).
  Partial block: source-title hydration needs an api-client endpoint + host onOpenSourceDoc (optional prop
  exposed). Scoped 38/38.
- A3: embed entry gains history restore (latest 20 via getmsgList semantics), SSE references with the
  three-level fallback rendered as docInfo-style collapsible groups, and channel suggested questions
  (GET /embed/:channelId/suggested-questions, hidden after the visitor speaks). Orchestrator closure: 
  referenceHeadline now counts document GROUPS (key knowledge_id||knowledge_title||id, docInfo
  groupedKnowledgeRefs) instead of raw chunks — 5-chunks/3-docs behavioral test pinned. Scoped embed suites
  21/21. Deferred: history scroll pagination, per-message follow-ups, citation pills.
- A4: platform shell audited across sidebar collapse, tenant switcher, user menu, command palette, and state
  coverage — two gaps fixed (collapse persistence key → Vue's `sidebar_collapsed`; literal `Lite` sup under
  weknora_lite_mode). Platform 170/170. Deferred: org pending badge, memberships throttle, last-active
  persistence (domain-out), collapsed drag handle.
- Gates: `pnpm test:web` 1499/1499, `pnpm test:shared` 596/596, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r442-code-parity-round.md`. No Vue, mobile, or Go code was
  modified.

## 2026-09-17 Round R443 — Indexing lock, embed pagination/follow-ups/citations, shell badge+throttle, wiki hydration

- Five parallel agents (A1 isIndexingLocked, A2 embed residuals, A3 platform-shell residuals, A4 wiki source
  hydration, A5 verifier), file-domain mutually exclusive, TDD. A5 verdict: **4/4 PASS, zero regressions, no
  rework items** — the first round to close with no orchestrator fixes needed.
- A1: settings surface probes `documents.list(kbId, {page:1,page_size:1})` once on open (edit mode, document
  type; FAQ/create skip) and locks the indexing checkboxes with Vue's lockedTip when total>0; probe failure
  degrades to unlocked (deliberate divergence recorded: Vue closes the editor on probe failure — product
  decision noted). 15 request-count assertions updated individually across 7 test files + 4 new cases;
  knowledge-settings 76/76; lockedTip key pre-existing ×5 locale.
- A2: embed history scroll pagination (cursor = previous batch's first created_at → `before_time`, top-edge
  500ms debounce, viewport-preserving prepend, four termination paths); per-message follow-up suggestions
  (ensure/GET backfill/generating poll ≤120s, click sends with `suggestion_attribution`; host analytics events
  intentionally out); citation pills mirroring Vue preprocessCitationTags/resolveCitationChunkId (DOC-n/FAQ-n/
  positional/UUID, invalid dropped; web pills external, kb pills via embed.public.chunk; plain-text parsing
  deviation recorded). 15 new tests; embed suites 36/36.
- A3: organizations pending-approval badge (sum of pending_join_request_count, mount-time fetch, 18px amber
  pill, raw count, hidden on failure) and the tenant submenu memberships ≥2000ms throttle refresh (applyAuthMe
  extracted, stale-client guarded). 6 new tests; platform 176/176. Deferred: explicit
  Organization.pending_join_request_count typing in api-client.
- A4: wiki reader footer source-title hydration with ZERO api-client changes — existing documents.get is the
  same GET /api/v1/knowledge/{id} Vue uses; createSourceRefTitleHydrator ports the Vue engine (dedupe, per-id
  cache, title||file_name resolution, failure keeps truncated-id fallback, seq guard). 6 tests; wiki 44/44.
- Gates: `pnpm test:web` 1530/1530, `pnpm test:shared` 596/596, `pnpm typecheck:web` clean, `pnpm build:web` ✓
  (orchestrator re-confirmed on the merged state). Evidence:
  `evidence/vue-react-parity/2026-09-17-r443-code-parity-round.md`. No Vue, mobile, or Go code was modified.

## 2026-09-17 Round R444 — Embedding lock warning, embed markdown pipeline, admin invite/pagination, shared-drawer evidence

- Five parallel agents (A1 Embedding lock warning, A2 embed markdown pipeline, A3 administration closeout +
  Organization typing, A4 browser fixture/evidence, A5 verifier). A5 verdict: A1/A2/A3 PASS with zero
  orchestrator fixes; A4's report landed after A5's polling window (agent itself completed; findings folded in
  at closure).
- A1: Embedding selector locks with the Vue warning when `ragEnabled && hasFiles` — driven by the same R443
  probe as isIndexingLocked; all-strategies-off keeps Embedding editable (Vue-authoritative nuance); storage
  migrate-hint now conditional on hasFiles. knowledge-settings 81/81; zero new i18n keys. Deferred: storage
  "editable when no files" (needs the save pipeline), row-visibility signal diff.
- A2: embed answers render through the Vue-identical pipeline — citation pills extracted to placeholders BEFORE
  marked.parse, restored + standalone-paragraph collapse after, same-version DOMPurify. wiki/markdown.ts only
  gained two exports; wiki consumption unchanged (19/19). Live findings fixed: DOMPurify hook element.remove()
  aborts sanitization (FORBID_TAGS carries it), USE_PROFILES strips target (same-version behavior). embed
  44/44. Deferred: KaTeX, mermaid/image safe-renderer, citation-icon svg.
- A3: Organization.pending_join_request_count typed in api-client; invite two-step confirmation per Vue
  (preview → confirm sends, Back returns, failure stays); admin members pagination (default 20,
  [10,20,50,100], clamped jump, server total). administration 11/11, identity 5/5; 7 new keys ×5 locale (ru
  mirrors Vue ru-RU honestly). Deferred: barrel export, auto-accept skip, owner invite role.
- A4 browser evidence (6 paired screenshots, r444-20260917/): reusable fixture created (shared-fixture
  account, tenant 10003, KB 08e02d8b shared viewer via org Parity 共享空间 + invite code d2e59d97f312aaa9);
  the R438 shared-KB drawer matches on BOTH ends (five fields + buttons verbatim, read-only entry on both).
  Recorded diffs: React shared card's extra 设置 button (Vue: 查看详情 only); React-only read-only banner vs
  Vue's stale 拖拽上传 copy; document count "-" vs "0"; drawer close text button vs × icon. Side findings:
  fixture tenant without models stalls the UI create wizard (setup done via API); Vue share-management org
  search misses an org the user administers with no network request (suspected Vue filter bug, flagged). The
  pending-approval badge pair was not capturable (invite joined without approval; React tab present, empty).
- Gates: `pnpm test:web` 1545/1545, `pnpm test:shared` 600/600, `pnpm typecheck:web` clean, `pnpm build:web` ✓
  (A5 final run on the merged state; +19 all accounted). Evidence:
  `evidence/vue-react-parity/2026-09-17-r444-code-parity-round.md`. No Vue, mobile, or Go code was modified.

## 2026-09-17 Round R445 — Shared-drawer diffs, embed rich rendering, storage editability, embedding visibility

- Infrastructure incident: three parallel-dispatch attempts (5/5/5 agents) were interrupted before resume; the
  round completed under the relaxed 2-agent minimum — A1 (shared drawer) + A2 (embed rich rendering) dispatched,
  A3 executed directly by the orchestrator on the interrupted partial's red tests, and a dispatched reviewer
  (A5v) audited all three change sets: all PASS. Commit-integrity incident found and fixed: R442/R444 had each
  missed one file (`chunkingSamples.ts`, `embed/markdown.ts` were untracked while referenced by committed code
  — a fresh checkout of those commits would not build); fixed by `ca059689` (+637). Every round commit now
  sanity-checks untracked files referenced by committed code.
- A1: shared cards converge their actions on `isSharedCard` (Vue renders only the info-circle detail trigger
  for non-owned shared cards); shared document-count badge uses Vue's `knowledge_count || '-'`; drawer close
  re-audit — Vue's header is an × icon with `aria-label=general.close`, so only React's closeLabel changed.
  knowledge-bases 113/113 (also fixed an interrupted-session test that OOM-killed the runner by deep-diffing a
  jsdom Element). BLOCKED: the React-only read-only banner's deletion point is inside the externally occupied
  `KnowledgeDocumentsPage.tsx:3362-3366` — exact location recorded.
- A2: KaTeX wired with the Vue frontend's literal pins (katex ^0.16.45, marked-katex-extension ^5.1.8,
  throwOnError:false/nonStandard:true) on a dedicated Marked instance (a global marked.use would leak into the
  wiki face); citation icons restored (ziliao.svg/websearch-globe.svg); mermaid assumption REVERSED by
  verification — the Vue embed face DOES hydrate mermaid, recorded as a real gap with the reuse path (views
  chat mermaid engine, mermaid 11.15.0). embed suites 50/50 (+6); build emits the katex chunk.
- A3 (orchestrator-executed on the interrupted partial's red tests): storage instance select binds Vue
  KBStorageSettings semantics — `disabled` only while the KB has files, editable otherwise, handleChange emits
  backend id + provider into the draft, both persist through the config PUT; the committed backend stays
  selectable when absent from the live list. Embedding row visibility per KBModelConfig
  `v-if="ragEnabled !== false || wikiEnabled"` — pure-LLM drafts remove the row, wiki-only keeps it without the
  required star and with `embeddingWikiOptionalDesc`; R444's all-off test rewritten to the Vue-accurate
  contract. r445 tests 5/5; directory 74/74.
- Auth slice (interrupted-session continuation, separate commit `2f0fc5ef`): login language switch confirms
  with `language.languageSaved` in the new locale (Vue Login.vue:522-528); OIDC-entry failures use
  `auth.oidcLoginFailed` (Vue Login.vue:636/647). auth suites 22/22.
- Gates: `pnpm test:web` 1560/1560, `pnpm test:shared` 600/600, `pnpm typecheck:web` clean, `pnpm build:web` ✓
  (dispatched reviewer's and orchestrator's runs agree; +15 vs R444). Evidence:
  `evidence/vue-react-parity/2026-09-17-r445-code-parity-round.md`. No Vue, mobile, or Go code was modified.

## 2026-09-17 Round R446 — Embed mermaid hydration + paired-browser regression sweep

- Four parallel agents: A1/A2 paired-browser regression over the R439-R445 code rounds, A3 mermaid hydration,
  A4 verifier (verdict: A3 PASS, zero rework; gates on merged HEAD 1571/1571 web, 602/602 shared, typecheck and
  build clean; the external process landed a mid-round fix for a transient attachments.ts TS error — attributed
  there). Also at round open: committed the R445 ledger/matrix/evidence that closure had left uncommitted
  (`125eb724`), and restarted both dev servers.
- A3: mermaid blocks in embed answers hydrate after completion, mirroring Vue EmbedBotMessage — markdown
  renderer tags fences (DOMPurify ALLOWED_ATTR +data-markdown-diagram only), hydration delegates to the shared
  views engine (mermaid 11.15.0 via packages/views, no direct dep), live answers gain is_completed on submit
  success, failures degrade to the code block. 11 controlled-promise tests; embed suites 61/61; mermaid ships
  as its own async chunk. Deferred (views-readonly): badge/fullscreen chrome, per-color themes.
- A1 browser sweep (24 screenshots): VERIFIED live — R444 overlap fix (80), chips anatomy, Embedding required
  star with vector on, wiki markdown/[[link]]/footer, login language toast. NEW defects (R447 queue): D1 high —
  React settings page auto-redirects to the VUE origin (:5180/platform/knowledge-bases?scope=all) after ~8s
  idle, reproduced 3× (root cause unknown); D2 high — 「测试分块效果」preview fails 「Invalid chunking preview
  response」 (response-shape mismatch, R442 slice); D3 strategy dropdown blank; D5 activity empty-card coexists
  with the table; D6 untranslated general.helpAndDocs; D7 退出 click ineffective. D4 correction: Parity KB Demo
  has 1 file — the storage select being disabled on BOTH ends is correct.
- A2 browser sweep (15 screenshots): shared drawer R445 regression PASSES on both ends (fixture intact);
  pending-approval badge correctly absent on both; embed no-token/invalid-token states match Vue copy; naming
  diff recorded (待审核申请 vs 加入申请). Full embed handshake/chat untestable (no channel configured). Process
  note: shared-browser contention between A1/A2 detected; captures redone atomically.
- Gates: `pnpm test:web` 1571/1571, `pnpm test:shared` 602/602, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r446-code-parity-round.md`. No Vue, mobile, or Go code was
  modified by this round.

## 2026-09-17 Round R447 — Defect-fix round: chunking preview envelope, shell copy/logout, D1 root-caused as external

- Four parallel agents (A1 knowledge-settings defects, A2 platform-shell defects, A3 D1 root-cause, A4
  verifier). A4 verdict: **all PASS, zero rework**; gates on the final state: test:web 1583/1583, test:shared
  602/602, typecheck clean, build ✓, zero new failures.
- A1: D2 root cause — the backend wraps the chunking preview in a `{success, data}` envelope
  (internal/handler/chunker_debug.go:256) while api-client parsed top-level fields; envelope unwrapping added
  (minimal, contract documented — the old mock encoded a shape the backend never sends). D3: the empty strategy
  option now renders the Vue placeholder (key pre-existing ×5 locale). D5: the activity overview card keyed off
  a field the settings API never returns, so it always showed empty alongside the populated table — the empty
  card no longer renders. knowledge-settings 90/90; api-client settings 8/8.
- A2: D6 — `general.helpAndDocs` never existed in packages/i18n; added ×5 locale verbatim from the Vue locale
  files. D7 — the shell's bare `void onLogout()` let a rejected/hung logout chain strand the user;
  `runShellLogout` guarantees /login (hard-navigate on reject or 4s non-settle; the on-site failure mode is
  inferred — R448 browser re-check queued). Naming — the org settings tab is 「加入申请」 per
  OrganizationSettingsModal.vue:505-516/1006-1011 with the inner 「待审核申请」+badge; React used the inner title
  in both places. platform 182/182 (+6), organizations 37/37.
- A3: **D1 closed as EXTERNAL behavior, not a React defect** — the evidence and the external automation shared
  one real Chrome tab; the external driver navigates to the Vue KB list on an ~8s cycle and Vue
  `useListUrlState.ts:84` appends `?scope=all` (React never constructs it). Exclusion evidence: the settings
  page has zero timers/navigations (grep counts), all apps/web location writes are same-origin relative, vite
  proxies only /api + /files, no SW/BroadcastChannel/postMessage, no 5180 in env/config, no recent navigation
  commits. Mitigations: browser agents now require isolated contexts with parallel driving frozen during
  capture; `navigation-origin-guard.test.ts` added as a static sentinel against hardcoded absolute-URL
  navigations (planted cross-origin samples caught, current code passes). Deferred: A2 found the user menu
  missing a 「全部设置」 entry — recorded, not dispatched.
- Gates: `pnpm test:web` 1583/1583, `pnpm test:shared` 602/602, `pnpm typecheck:web` clean, `pnpm build:web` ✓.
  Evidence: `evidence/vue-react-parity/2026-09-17-r447-defect-fix-round.md`. No Vue, mobile, or Go code was
  modified by this round.

## 2026-09-17 Round R448 — Banner removal, user-menu 全部设置, live D2 rework, isolated-context verification

- Four parallel agents: A1 isolated-context live verification of the R447 fixes, A2 user-menu 全部设置 entry,
  A3 the R445-blocked read-only banner removal (the external occupation of KnowledgeDocumentsPage.tsx ended
  this round), A4 verifier (A2/A3 PASS; A1's live evidence invalidated one R447 fix, reworked by the
  orchestrator in-round). The external process ALSO stash-popped three of its own test files leaving TS1185
  conflict markers — external-owned, left in place, every gate failure they cause attributed (see below).
- A1 (atomic-script capture per the R447 rule; 11s sampling, zero pollution): D7 logout VERIFIED — 退出 →
  /login in 74ms, fallback never fired. D2 INVALIDATED: the live preview still failed because the backend
  sends `rejected: null` when nothing was rejected and R447's parser demanded an array — orchestrator rework:
  normalize to [] with a contract test replaying the exact live payload (captured via the real login/preview
  API before the fix). Product finding: logout revokes ALL devices' sessions (the Vue tab 401'd immediately) —
  backend/product confirmation queued. Menu state recorded for A2's cross-check.
- A2: the user menu gains the Vue 「全部设置」 entry (general.allSettings ×5 locale byte-exact; after skills
  divider, before 帮助文档; unconditional; navigates /platform/settings WITHOUT a section query — distinct
  from the ⌘1-9 shortcuts). platform 185/185 (+3); i18n 65/65 (each locale exactly +1). Recorded: Vue's
  isSystemAdmin-gated 系统管理 entry remains unmigrated.
- A3: the React-only 「查看权限：编辑操作已隐藏。」banner removed (5 lines) — Vue's shared-KB document page has
  no banner and expresses read-only purely by hiding edit entries; the 20+ other canContribute uses untouched.
  documents 207/207. Deferred: knowledgeBase.documents.viewerReadonly is now a dead i18n key (outside the
  file domain).
- Verification under the external breakage: `pnpm test:shared` 603/603 (includes the new rejected:null
  contract test); orchestrator scoped run over every domain touched this round — knowledge-settings (13
  files) + platform (6) + documents page-chrome = 144/144; `pnpm typecheck:web` excluding the three external
  conflict files reports ZERO further errors. test:web full-suite and build:web stay red SOLELY from the
  external stash-pop markers (integrations/route.test.ts ×2, knowledge-settings/GraphSettings.test.ts ×1,
  settings/McpToolsDirectory.test.tsx ×1) — they recover the moment the external process resolves its stash.
- No Vue, mobile, or Go code was modified by this round. Evidence:
  `evidence/vue-react-parity/2026-09-17-r448-code-parity-round.md`. Per-agent reports:
  .omc/state/r448/report-A{1,2,3}.md + report-A4-review.md (session artifacts).