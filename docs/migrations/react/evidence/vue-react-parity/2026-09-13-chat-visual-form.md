# Chat 视觉形态对齐（chat visual-form slice）— 2026-09-13

## 范围

React 多端工作树（codex/react-multiclient）packages/views/src/chat 渲染层 + apps/web/src/chat/chat.css，
对齐 Vue 基线（frontend/ 为只读权威基准）的会话页 / 新会话（creatChat）页视觉形态。
流式 reducer、resume/continue-stream、cancel、retry、approvals、tool-result 渲染、artifact 预览、
references、会话分组持久化、时间戳、来源徽标、starter questions、agent 选择等逻辑全部保留，仅动渲染层。

## Vue 参照（逐文件）

- frontend/src/views/chat/index.vue — .chat/.chat_thread/.msg_list(960px)/.input-container 布局、conversation-time 位置
- frontend/src/views/chat/components/usermsg.vue — 用户气泡（右对齐、--td-bg-color-secondarycontainer 底、8px 12px、max-width min(76%,820px)）
- frontend/src/views/chat/components/botmsg.vue + components/css/chat-message-shared.less — 助手纯文本 + .answer-toolbar 图标行（30px 圆角图标钮、gap 4px、margin-top 6px/margin-left -7px）
- frontend/src/components/chat/ChatHeader.vue — 左上浮动标题胶囊（毛玻璃、14px/500 secondary 色）+ ⋯ 菜单（重命名/置顶/清空/删除）
- frontend/src/components/chat/MessageTimestamp.vue — 会话时间分隔（12px、placeholder 色、居中、padding 4px 0 8px）
- frontend/src/components/Input-field.vue — .rich-input-container（12px 圆角、focus 绿边）+ .control-bar：左 agent-mode 芯片/回形针/@，右 model-selector-trigger 芯片 + 圆形绿色 send（28px、disabled #8ce0af）
- frontend/src/views/chat/components/SandboxTerminal.vue + frontend/src/composables/useSandboxTerminal.ts + components/chat/SandboxSidePanel.vue — 沙箱面板：默认关闭，右上镜像按钮开启，右侧 420px drawer
- frontend/src/views/creatChat/creatChat.vue + components/css/suggested-questions.less — 新会话页：常驻欢迎标题（28px/600）+ 推荐问题卡片（10px 圆角、8px 14px、居中流式）
- frontend/src/i18n/locales/zh-CN.ts — 全部中文文案字节级来源（chat.*、input.*、createChat.*、time.*、menu.*）

## 变更文件

- packages/views/src/chat/chat-copy.ts（新增）— zh-CN 文案集中层，每条标注迁移目标 i18n key
- packages/views/src/chat/page.tsx — 头部改标题+⋯菜单（置顶/重命名/清空消息/删除会话）+ 沙箱切换；无会话时隐藏头部；agent 选择移入 composer 芯片；沙箱从内联块改为 on-demand drawer（新 prop terminalOpen 控制初始态）；starters 改为常驻欢迎标题 + 按条件渲染卡片/骨架
- packages/views/src/chat/session-sidebar.tsx — 新对话（绿）按钮、来源/搜索/分组筛选、时间分组头（已置顶/今天/昨天/近7天/近30天/更早）、全宽标题、active 绿底绿字、hover ⋯ 菜单、来源徽标仅非 web 会话显示
- packages/views/src/chat/message-list.tsx — 会话时间分隔（今天 HH:mm/昨天 HH:mm/M月D日 HH:mm/…）替代胶囊；用户消息右对齐气泡（无头像/角色标签）；助手消息纯文本 + 复制/收藏(禁用)/fallback info 图标行；pendng 状态气泡内「发送中…/重试」；typing 指示去头像
- packages/views/src/chat/composer.tsx — Vue 输入区解剖：placeholder 直接向模型提问、左 快速问答▾(select#kw-chat-agent)/回形针/@，右 对话模型 芯片 + 圆形绿发送（流式中空输入时变停止按钮，class wk-chat-stop 保留）
- apps/web/src/chat/chat.css — 布局段全部按 Vue scoped 样式 + TDesign 主题 token（#07c05f/#e7e7e7/#f3f3f3/#f9f9f9）重写；工具结果/approval 样式段原样保留
- apps/web/src/chat/ChatRoutePage.tsx — 无需改动（props 全兼容）
- 测试：apps/web/src/chat/chat-page.test.ts（重写断言到新结构/中文文案）、apps/web/src/chat/artifact-preview.test.ts（Preview/Download/Expired → 预览/下载/已过期）

## 测试（red → green）

- 改动前基线（stash 后 HEAD 抽查）通过；重写后第一轮：apps/web/src/chat 3 失败（旧英文断言 + guides.css 连带故障），packages/views 0 失败
- 终态（focused runs）：
  - pnpm exec tsx --test packages/views/src/chat/*.test.tsx packages/views/src/chat/*.test.ts → 48 pass / 0 fail
  - pnpm --filter @weknora/web exec tsx --test 'src/chat/*.test.ts' → 48 pass / 0 fail（含 artifact-preview、message-extras-view、chat-page、resume、send-run、stream-recovery 等逻辑面）
- pnpm run typecheck:shared → 0 error；pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit → src/chat 无 error（剩余 src/agents、src/faq、src/settings error 为并行代理所有文件的既有状态，与本切片无关）

## 实况核验（1440x900 / zh-CN / parity-test@local.dev）

截图：docs/migrations/react/evidence/vue-react-parity/screenshots/chat-visual-slice/

- react-chat-session.png — 会话「修复后首发截图」(id 4dcd7b0d-5981-457f-9d35-1b8ac0929c43)：分组全标题列表+绿色 active、标题+⋯头部、昨天 18:21 时间分隔、右对齐用户气泡、助手纯文本+图标行、Vue 形态 composer、终端不可见（对位 Vue accept-20260913/vue-chat-session.png；欢迎引导弹窗与 Vue 截图同样在场）
- react-chat-sandbox-drawer.png — 头部右侧按钮打开的 420px 右侧 drawer（启动终端/终端输入/断开终端），WS 接线不变
- react-creatChat.png — 新会话页：常驻欢迎标题 + Vue 形态 composer（starter 卡片需选中 agent，与 Vue 数据条件一致）

## 缺失 i18n key（packages/i18n 无 chat 域，本轮未改 i18n；文案暂寄 chat-copy.ts）

chat.suggestedQuestions、chat.refreshSuggestedQuestions、chat.followUpQuestions、chat.thinkingAlt、
chat.fallbackHint、chat.conversationTime.today/yesterday/thisYear/otherYear、createChat.title、
menu.newSession、time.today/yesterday/last7Days/last30Days/earlier/pinned、input.placeholder、
input.normalMode、input.send、input.stopGeneration、input.steerCurrent、input.steerAfter、
chatHeader.moreActions、menu.renameSession、menu.clearMessages、chatHeader.deleteSession、
chat.sandbox.panelTitle/start/terminalInput 相关、agent.copy、agent.addToKnowledgeBase（agent 域已存在，可先行接 formatMessage）
建议：在 @weknora/i18n 生成 chat 域后，将 chat-copy.ts 替换为 formatMessage 调用（文件内已留 TODO 注释逐 key 标注）。

## 已知差异 / open items

1. 【shell 集成 open item】Vue 的会话列表位于平台侧边栏（platform sidebar 一体）；React 暂保留页内 chat 侧栏（260px，视觉对齐 Vue 列表区）。待 shell 集成切片收编。
2. 收藏（bookmark）按钮为禁用态展示：Vue 点击进入知识库手动编辑器，React 尚无该界面，图标保留 aria-disabled。
3. 对话模型芯片为展示性（display-only）：React 无模型注册/选择状态，芯片呈现「对话模型」占位样式；Vue 有完整模型下拉。作为后续模型选择切片接入点（composer 已留 modelLabel prop）。
4. 回形针/@ 按钮为形态占位（无上传/提及面板）：Vue 连接 AttachmentUpload/MentionSelector。
5. drawer 打开时内容不自动让位（Vue 会 padding-right 平移聊天区）；430px 以下 drawer 覆盖内容。平移留给沙箱面板切片。
6. LiveResponse 流式状态条、approval/OAuth 卡片、follow-up 建议区为 React 现有逻辑面，样式收敛到主题但信息结构未对齐 Vue 的 AgentStreamDisplay 时间轴（属 agent 流渲染切片）。
7. 会话时间分隔出现在消息组顶部居中（对齐 Vue MessageTimestamp）；Vue 首屏顶部还有「今天 18:21」式单条时间（同组件渲染），React 由 shouldShowConversationTimestamp 逻辑决定位置，与 Vue 规则一致。
8. onboarding 欢迎弹窗两栈同在（Vue 截图亦有），非本切片范围。

## 工具

- 截图脚本：.parity-tools/chat-visual-slice.cjs（登录→会话页→drawer→creatChat）、.parity-tools/chat-drawer-shot.cjs（drawer 单独重拍；先跳过引导弹窗再点击切换）
- React dev server：VITE_API_BASE_URL=http://127.0.0.1:8080 pnpm --filter @weknora/web exec vite --port 5181（本轮启动，仍在运行）
