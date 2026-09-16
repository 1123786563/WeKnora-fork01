# R013 Integrations 页面切片证据（IM / 网页嵌入 / API / CLI / Chrome）

日期：2026-09-13
目标 worktree：/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient（分支 codex/react-multiclient）
切片目标：React integrations 页面对齐 Vue 设置抽屉「发布集成」组（IM 集成 / 网页嵌入 / API 集成 / CLI / Chrome 插件 / Claw Skill），同时保留既有功能接线。

## Vue 基线对照（frontend/src 只读）

- frontend/src/views/integrations/IntegrationSettingsSection.vue：每个 tab 一个 .section（h2 标题 + .section-description；IM 追加「查看接入文档」外链）。
- frontend/src/components/IMChannelPanel.vue / AgentEmbedChannelPanel.vue：channels-header（渠道标题 + 智能体筛选 + 数量徽标）、channel-grid 渠道卡片（平台徽标 / 名称 / 已停用 tag / 智能体名 / 操作）、虚线「添加渠道」tile、channels-empty 空态。
- frontend/src/components/css/channel-panel-list.less、integration-landing.less：卡片与 landing 样式来源。
- frontend/src/views/integrations/cliIntegration.ts：CLI 连接命令构造（原样移植）。

## React 侧变更（本切片）

- packages/views/src/integrations/messages.ts（新增）：分层翻译器 —— formatMessage（经相对路径引入 packages/i18n/src/index.ts）优先，缺失 key 回退到五语言逐字对照表（agentEditor.im.* / agentEditor.embed.* / embedPublish.* 尚未迁移进共享包）。
- packages/views/src/integrations/view.ts（新增）：integrationSectionCopy(tab, locale) 视图模型 —— 标题 / 描述 / 文档链接 / 渠道标题 / 添加 tile / 空态 / 停用 / 未命名 / 删除确认；imPlatformLabels 平台名（飞书/企业微信/钉钉/云之家…）。
- packages/views/src/integrations/cli.ts（新增）：buildCLIConnectCommand 逐字移植 + 测试。
- packages/views/src/integrations/page.tsx（重写）：按 Vue 结构逐 tab 渲染：
  - IM：IM 集成 + 描述 + 查看接入文档 ↗ + IM 渠道数量头 + 渠道卡片（平台徽标首字、名称、已停用 tag、智能体名、开关/编辑/删除）+ 虚线添加渠道 tile + 折叠创建表单（绑定智能体/平台本地化下拉/渠道名称/平台凭证 JSON）。
  - 网页嵌入：网页嵌入 + 描述 + 嵌入渠道数量头 + 渠道卡片（</> 徽标；无名称回退「网页嵌入」默认名；整卡可点 = 保留 preview-session 流）+ 重置密钥/编辑（走 buildEmbedUpdatePayload）/删除 + 虚线新建嵌入渠道 tile。
  - API 集成：API 地址带复制行 + OpenAPI /docs 行 + API Keys 区（创建 API Key 表单、一次性显示新 Key、复制、删除确认）+ 用户身份模式（三模式 chip / 直传用户 ID 开关 / HMAC 密钥）+ 测试 Token 签发 + Playground。
  - CLI / Chrome / Claw：landing hero + 外部 CTA + 步骤/能力区（integrations.cli.*/chrome.*/claw.* key 已全部在共享包）。
- apps/web/src/styles.css：以 wk- 前缀移植 channel-panel-list.less / integration-landing.less 关键样式；补 wk-button 基础样式（此前无定义）。
- apps/web/vite.config.ts：补一行 @weknora/domain/sandbox/skill-install alias（该模块文件已存在但 file: 依赖副本过期导致 vite 解析 500、全 app 白屏；仅基础设施修复，不动他人面板代码）。
- 未改：packages/i18n/**、packages/api-client/**、packages/domain/**、apps/web 其它面板。

## TDD 记录

- 红：packages/views/src/integrations/page.test.tsx（IM/嵌入 zh-CN 解剖、API/外部 tab 共享 key、五语言无 key 泄漏、平台名）与 cli.test.ts（连接命令）先失败（模块不存在 / 行为不符）。
- 绿：实现 messages/view/cli 后 18/18 pass（既有 apiKeys/form/registry 测试全部保留）。
  命令：pnpm exec tsx --test packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx

## 类型检查

- pnpm run typecheck:shared：通过（0 错误，收尾时复查）。
- pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit：本切片文件 0 错误；剩余错误位于并行切片在改文件（src/faq/FAQPage.test.tsx、src/settings/TenantMembersPanel.tsx）。

## 截图证据（1440x900，zh-CN）

目录：docs/migrations/react/evidence/vue-react-parity/screenshots/integrations-page-slice/

| 文件 | 说明 |
|---|---|
| vue-im.png / vue-embed.png / vue-api.png | Vue :5180 基线（引导弹窗已跳过） |
| react-im.png | React :5181 设置抽屉 IM 集成（feishu/wecom 渠道卡片、已停用 tag、添加渠道 tile） |
| react-embed.png | React 设置抽屉 网页嵌入 |
| react-api.png | React 设置抽屉 API 集成 |
| react-cli.png / react-chrome.png | React 设置抽屉 CLI / Chrome 插件 landing |

数据准备：经 API 以 parity-test@local.dev 登录后创建 feishu（飞书客服）与 wecom（无名称→服务端默认名，随后停用）两条 IM 渠道。

## 已知差异（deliberate deltas）

1. i18n 缺 key（需上游 packages/i18n 补齐；本切片以本地五语言对照表回退，key 名与 Vue 完全一致，上游补齐后自动切换）：
   - agentEditor.im.*：title/description/docLink/channelsTitle/addChannel/empty/disabled/unnamed/deleteConfirm/enabled/platform/channelName(+Placeholder/DefaultHint)/sectionCredentials/十平台名
   - agentEditor.embed.*：title/description
   - embedPublish.*：create/channelsTitle/disabled/empty/deleteConfirm/name(+Placeholder/DefaultHint)/defaultChannelName/allowedOrigins/originsPlaceholder/created/resetKeyTitle
   - IM 标题实际用共享 key integrations.tabs.im（zh-CN 同为「IM 集成」）；嵌入标题用 integrations.tabs.embed（同为「网页嵌入」）。
2. 品牌主题：React 应用整体为蓝色主题（#2e6de6），Vue 为绿色品牌色；随应用级主题另切片处理。
3. 角色门控：Vue 对非 admin 隐藏添加 tile/操作；React 页面暂无角色信息，始终渲染添加 tile 与操作（admin 账号观感一致）。
4. 渠道卡片操作：Vue 为 hover 浮现图标按钮（省略号菜单 + popconfirm 删除）；React 常显文本按钮 + window.confirm + t-switch 风格启用开关。
5. 平台徽标：Vue 用平台 SVG logo；React 用平台名前两字文本徽标（views 包无图片资产）。
6. 智能体筛选/绑定：Vue 下拉加载智能体列表；React api-client 未暴露 agents.list —— 筛选器暂缺、创建表单以 agent ID 文本输入替代（需要 api-client 增加 agents 列表端点）。
7. IM 创建表单：Vue 为 4 步向导抽屉（基础/接入/知识库/凭证，含逐平台凭证字段与微信扫码绑定）；React 保留紧凑内联表单（agent ID + 平台 + 名称 + 平台凭证 JSON + 保存）（需要 api-client 补逐平台凭证类型）。
8. 嵌入渠道：Vue 卡片点开为配置抽屉（安全/能力/嵌入代码含密钥重置）；React 卡片点击 = 打开 preview-session（保留既有流程），重命名走 buildEmbedUpdatePayload，重置密钥按钮保留。
9. API Playground：Vue 为抽屉内分步（创建 Session → Agent Chat SSE → 提取回答）；React 保留原内联请求/响应面板（接线不变），后续可按 Vue 分步重排。

## 阻塞与交叉影响（并行切片）

- 拍摄期间 apps/web/src/settings/SettingsPage.tsx（他人在改）一度处于语法错误状态（vite 'return' outside of function），导致整个 React app 白屏/错误覆盖层约 25 分钟；该文件修复后已重拍全部 React 截图，最终截图均为可编译状态下的真实渲染。本切片文件全程不受影响。
- 其并行问题：apps/web/src/settings/SkillSettingsPanel.tsx 引入 @weknora/domain/sandbox/skill-install 时 vite alias 缺失导致全 app 500，本切片在 apps/web/vite.config.ts 补一行 alias 解除阻塞。
- pnpm run typecheck:shared 与 web tsc 的剩余错误均位于并行切片文件（guides/*、chat/message-list.tsx、settings/SettingsPage.tsx）。

## 证据边界

未包含：Vue/React 同数据同视口像素 diff 报告、E2E 自动化、移动端/Wails 运行、嵌入 iframe 实站验证。未执行 git commit。