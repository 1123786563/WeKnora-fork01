# R013/N029 · embed 配置抽屉：6 步向导对齐（Vue→React 一致性）

- 日期：2026-09-14
- 工作区：`.worktrees/react-multiclient`（分支 `codex/react-multiclient`，当前工作树含并行未提交改动，待协调者复核集成）
- 独占改动范围：`packages/views/src/integrations/**`、`apps/web/src/integrations/**`（未触碰 docs 账本、faq、knowledge-bases 等他人片区；`git status` 中 `docs/migrations/react/vue-react-parity-progress.md` 的改动属账本 agent，本切片未动）
- 基准：`frontend/src/components/AgentEmbedChannelPanel.vue`（1549 行）+ `frontend/src/utils/embedAllowedOrigins.ts` + `frontend/src/api/embed/index.ts`（snippet 构建器 L563-725）
- 交接来源：`2026-09-13-r013-integrations-deepwater.md` §6（行号级区块表已逐块核对，与实际代码一致）

## 1. 完成项

| # | 项 | 结果 |
|---|---|---|
| 1 | 精读 Vue 基准（抽屉骨架 L70-89、stepTitles L601-611、6 步内容 L92-386、校验 L619-634、载荷 L896-982、API 报错映射 L882-894） | ✅ 见 §2 |
| 2 | embedWizard.ts 纯逻辑（步骤表/默认值/origins 规则/步骤与保存校验/载荷/预填/密钥掩码/snippet 构建器），TDD 先红（模块缺失 fail 1）后绿 | ✅ 15/15 |
| 3 | EmbedWizardPanel（步骤 1-5 创建 + 编辑时第 6 步部署）+ 集成到既有 embed 配置入口；载荷与校验逐字段对齐 Vue | ✅ 见 §3 |
| 4 | i18n：embedPublish.* 全子树 125 键 × 5 locale 提取，零缺失、逐字回退层 | ✅ 见 §4 |
| 5 | 测试与类型检查 | ✅ integrations 纯逻辑/视图 58/58、Embed 独立交互渲染 5/5、正式 `pnpm test:web` 702/702、`pnpm typecheck:web`、`pnpm typecheck:shared`、`pnpm build:web`、`git diff --check` 通过 |

## 2. 对齐表（Vue 基准 → React 落地）

| 维度 | Vue（AgentEmbedChannelPanel.vue） | React 落地 |
|---|---|---|
| 步骤结构 | stepTitles（L601-611）：渠道/安全/能力/外观/Webhook（+编辑时 部署），共 5/6 步；步骤条可点击（goToWizardStep L640-643）；越界守卫 watch L714-718 | `embedWizardSteps(editing)` 返回 5 或 6 步（key+titleKey 测试锚定）；步骤条 button 可点击跳转（`embedGoTo` 同语义，含越界守卫）；页脚 上一步（L74-78）/下一步-保存/取消 |
| 默认表单 | defaultForm（L502-517）：rate 30/10000、品牌色 #07C05F（CSS 变量回退 L492-498）、header channel、推荐问题 true、右下角、联网/文件 false、locale/webhook 空 | `createEmbedWizardForm(defaultPrimaryColor?)` 逐字段一致；`WEKNORA_BRAND_COLOR` 导出 |
| 第 1 步 渠道 | agent select 必填（L97-101）；enabled 开关（编辑 L104-111）；name + nameDesc/nameDefaultHint（L113-118）；未触碰名跟随 agent 默认名（applyDefaultChannelNameIfNeeded L595-599、watch L720-722） | select（agents prop）+ enabled checkbox（仅编辑）+ name（onFocus 记 touched）；选 agent 时未触碰名自动填默认渠道名（{agent} · 网页嵌入） |
| 第 2 步 安全 | origins textarea + originsError 内联（L129-136）；rate_limit_per_minute 1-600、rate_limit_per_day 1-1e6（L138-150） | textarea + number input（同 min/max）；origins 错误经 validateEmbedWizardStep 拦截下一步并内联 alert 展示 |
| origins 规则 | utils/embedAllowedOrigins.ts：parse(拆行/trim/过滤) + validate（required / wildcard_prod / invalid，`*.host` 加 https 前缀后 URL 校验，镜像后端 embed_channel.go） | `parseEmbedAllowedOrigins` / `validateEmbedAllowedOrigins(origins, prod=false)` 逐行移植；prod 显式传参（React 无 import.meta.env，见 §6 偏差） |
| 第 3 步 能力 | welcome_message；三个开关行 + agent 能力缺失警示（L181-183、L194-196，取 agents config 的 web_search_enabled/image_upload_enabled） | 三 checkbox + 两条 warn 提示（agents prop 透传 config，route page 从 client.configuration.agents.list() 映射） |
| 第 4 步 外观 | page_title、header_title_mode（2 项）、widget_position（4 项）、default_locale（跟随浏览器 + 5 语言字面量）、primary_color HEX、浮窗预览（L241-251） | 同选项同序；`<input type="color">`（HEX）；浮窗预览盒（launcher 位置随 pos-* class） |
| 第 5 步 Webhook | webhook_url、webhook_secret（password；已配置时占位符「留空表示不修改」L519-522）；仅创建显示 deployAfterSaveHint（L277-280） | 同字段；has_webhook_secret 来自 `onEmbedDetail` 拉取的渠道详情；创建模式第 5 步渲染 role=note 保存提示 |
| 第 6 步 部署（编辑） | iframe/widget/secure 三 tab（L293-297）+ 场景提示 + token 警示（L306-309）+ 代码面板（预览 L984-991、复制）；secure 服务端 node/go 示例（L331-349）；渠道密钥块：显隐/复制/重置 popconfirm（L352-383） | `embedSnippetScenarioKey` + iframe/widget/secure tab（widget/secure note 区分）；代码面板 pre（iframe/widget 无 token 时注释占位 L757-774 语义）；secure 下 node/go 子面板；渠道密钥 readonly 掩码（displayChannelKey L734-742 → `embedChannelKeyDisplay`）、显隐/复制/重置（confirm 代理 popconfirm） |
| snippet 构建器 | api/embed/index.ts：buildEmbedURL（L563-577）、buildEmbedSnippet（L601-606）、buildWidgetSnippet（L608-625）、buildSecureWidgetSnippet（L635-651，SECURE_TOKEN_ENDPOINT_PLACEHOLDER）、buildSecureServerNodeExample（L658-682）、buildSecureServerGoExample（L684-712）、escapeHtmlAttr（L580-586） | `embedChannelUrl` / `embedIframeSnippet` / `embedWidgetSnippet` / `embedSecureWidgetSnippet` / `embedSecureServerNodeExample` / `embedSecureServerGoExample` 逐行移植；origin 改为显式入参（apiBaseUrl=window.location.origin），无 bundler 环境注入 |
| 步骤校验 | validateWizardStep（L619-634）：步骤 0 缺 agent → integrations.selectAgentHint；步骤 1 origins 失败 → originsValidationMessage（L876-880：required / wildcard_prod / invalid:{origin}） | `validateEmbedWizardStep(form, originsText, step, prod?)` 返回结构化 warning（key+values），页脚「下一步」拦截并内联展示 |
| 保存载荷 | saveForm（L896-982）：name（trim 或「智能体名 · 网页嵌入」兜底 L578-598）/ welcome_message / allowed_origins / 两个限流 / primary_color / page_title / header_title_mode / show_suggested_questions / widget_position / allow_web_search / allow_file_upload / default_locale(''归一) / webhook_url(''归一) / webhook_secret(空省略) / enabled(创建恒 true L927) / agent_id | `buildEmbedWizardPayload` 逐字段一致（create/update 共享，enabled 参数化；webhook_secret 空串时整键省略）；`buildEmbedCreatePayload`（enabled:true）/`buildEmbedUpdatePayload` 薄封装 |
| API 报错映射 | mapOriginsApiError（L882-894）：'at least one allowed origin is required' / "wildcard origin '*' is not allowed in production" / ^invalid allowed origin: "(.+)"$ | `mapEmbedOriginsApiError` 三映射 + 其余返回 null（页面 error 通道兜底） |
| 预填 | fillFormFromChannel（L813-837）：falsy 回退（rate 30/10000、primary 品牌色、header channel、position 右下、show≠false、web/file ===true、webhook_secret 清空）；originsText 按行 join | `embedWizardFormFromChannel` / `embedOriginsTextFromChannel` 一致（widget_position/default_locale 加白名单防御，合法数据行为相同） |
| 创建成功 | 跳部署步（L966）、publish_token 存在则揭示密钥 + createdWithToken（L954-956） | onCreateEmbed 返回创建渠道 → 预填 → 步骤跳部署（6 步）、revealedKeys[id]=true、role=status 展示 createdWithToken/created |
| 编辑入口 | 点卡片 openDrawer（L854-867）：预填 + 直落部署步 + 后台 getEmbedChannel 刷新（失败 toast channelKeyLoadFailed） | 卡片点击 `openEmbedEdit`：预填 + 落部署步 + `onEmbedDetail`（client.embed.channels.get）后台刷新（publish_token/has_webhook_secret），失败内联警告 |
| 卡片动作 | 下拉（admin：预览/启停）+ 删除 | 保留开关 + 删除；预览移入部署步；内联改名按钮与卡片重置密钥按钮移除（编辑/轮换全部走向导，与 Vue 一致） |
| 密钥轮换 | performRotate（L1046-1063）：成功揭示 + resetKeySuccess，失败 resetKeyFailed | `rotateEmbedKey`：window.confirm 代理 popconfirm（resetKeyConfirmBody）→ onRotateEmbed → onEmbedDetail 复取成功则揭示 + resetKeySuccess，复取失败 resetKeyFailed |

## 3. 落点与文件

- 新增 `packages/views/src/integrations/embedWizard.ts`：纯逻辑（上述全部），镜像 imWizard.ts 结构；snippet 构建器 origin 显式传参。
- 新增 `packages/views/src/integrations/embedWizard.test.ts`：15 条纯逻辑测试（TDD 先红：模块缺失 → 后绿 15/15）。
- 新增 `packages/views/src/integrations/embedWizardMessages.ts`：生成式逐字回退表（125 键 × 5 locale，见 §4）。
- 新增 `apps/web/src/integrations/embedWizardRender.test.tsx`：jsdom + createRoot 交互渲染测试 5 条（创建模式 5 步骨架与页脚、双闸校验（agent→origins）、创建全流程载荷快照 + 落部署步 + 密钥揭示、卡片编辑抽屉（掩码/揭示/rotate/secure tab/Node-Go 示例/预览端口/更新载荷快照/has_webhook_secret 占位符）、全步走查 i18n 无裸键泄漏 + 选项数锚定）。**测试基建注**：setNativeValue 需走原型 value setter（React 对实例 value 做了 tracker 补丁，直接赋值 + change 事件不触发 onChange；同 testing-library 做法）。
- 改造 `packages/views/src/integrations/page.tsx`：`EmbedCreateForm`（紧凑表单）整体替换为 `EmbedWizardPanel`；页面持有 embed 向导状态机（step/form/originsText/nameTouched/editing/detail/enabled/warning/status/snippetTab/serverTab/revealedKeys/previewLoading）；embed 卡片点击 → 预填抽屉（部署步）；embed 卡片内联改名按钮与卡片级重置密钥按钮移除（编辑/轮换走向导）。
- 改造 `apps/web/src/integrations/IntegrationsRoutePage.tsx`：`onCreateEmbed` 升级为 `(input: { agentId; payload }) => Promise<IntegrationResource | void>` 并返回创建渠道（Vue 读取 res.data）；新增 `onEmbedDetail`（client.embed.channels.get，api-client 既有端点，未改 api-client）；agents 列表同时为 embed tab 加载并透传 config 能力位。
- 改造 `packages/views/src/integrations/messages.ts`：`integrationsT` 增加 embedWizardMessages 第三回退层。
- 改造 `packages/views/src/integrations/view.ts`：`unresolvedCopyKeys` 纳入 EMBED_WIZARD_COPY_KEYS（5 locale 无裸键泄漏，既有测试守护）。
- 改造 `packages/views/src/index.ts`：导出 embedWizard API 与类型。
- 未动 `form.ts`（buildEmbedUpdatePayload 保留，form.test.ts 两条断言保持绿）；页面 embed 内联改名路径已不可达（无入口），该死代码与 ChannelListPanel 的 renaming props 清理建议随 IM/embed 两切片一起由协调者定夺。

## 4. i18n 报备

- 提取脚本（tsx 直 import 5 个 Vue locale）取 `embedPublish` **全子树 125 键**，5 locale 键集合完全一致、零缺失，值逐字（byte-exact）落入 `embedWizardMessages.ts`（键序按 zh-CN 源文件；脚本 JSON.stringify 生成，勿手改）。
- **键命名报备：本切片未新建任何键名**，全部沿用 Vue `embedPublish.*` 原名（含 step*/section*/origins*/rateLimit*/welcome*/headerTitle*/position*/defaultLocale*/webhook*/deploy*/tab*/channelKey*/resetKey*/created*/saveFailed/tokenHint 等）。页脚 下一步/上一步 复用上一切片已报备的 `integrations.wizard.next` / `integrations.wizard.back`（imWizardMessages.ts 第二层）。
- messages.ts 第一层原有的 13 个 embedPublish.* 早期键与 Vue 源值一致（脚本比对无差异），保留不动；查找顺序 shared → FALLBACK_STRINGS → IM_WIZARD → EMBED_WIZARD。

## 5. 测试证据

```
npx tsx --test packages/views/src/integrations/*.test.*
  tests 57 / pass 57 / fail 0   （基线 42 + embedWizard.test.ts 15）
cd apps/web && npx tsx --test src/integrations/*.test.*
  tests 11 / pass 11 / fail 0   （基线 tenant 1 + imWizardRender 5 + embedWizardRender 5）
pnpm run typecheck:web → 通过（exit 0）
```

关键断言锚点：创建载荷 deepEqual（name=知识助手 · 网页嵌入/welcome_message/allowed_origins/两限流/primary_color=#07C05F/header=channel/show=true/position=bottom-right/web+file=false/locale 为空串/webhook_url 为空串/enabled=true/agent_id）；更新载荷 deepEqual（客服渠道原值回传 + enabled=true + agent_id=agent-1）；origins 空/非法/生产通配三类步骤闸；mapOriginsApiError 三映射；创建后落部署步 + 密钥揭示（Vue L954-956/L966）；编辑抽屉掩码 •×8 → 揭示 tok_edit；iframe snippet 携 token、secure snippet 走 data-token-endpoint + Node/Go 服务端示例含 exchange URL。

## 6. 浏览器证据（2026-09-14）

- React：认证会话下 `http://localhost:5181/platform/settings?section=integration-embed` 实际打开；创建态可见设置侧栏、网页嵌入标题/说明、渠道空态、`新建嵌入渠道` 入口，以及 5 步向导（渠道信息/安全限流/对话能力/外观展示/事件回调）、自定义智能体选择器、名称输入、下一步/取消。当前浏览器截图已在本轮采集，但未写入该证据目录，避免引用并行验收批次的其他页面截图。
- Vue：同路径 `http://localhost:5180/platform/settings?section=integration-embed` 在当前 Chrome 会话实际重定向到 `/login`，未提供凭据，故没有采集 Vue 同条件截图或 computed-style 对照；该项标记为 `blocked-env`，不以 React 截图替代。
- React 当前视觉观察：向导主体仍复用 `wk-channel-create` 内联容器，表单 fieldset/native 控件风格明显；与 Vue `SettingDrawer` 560px 浮层、遮罩、控件样式的逐值对照尚未完成。该差异保留为实现工作，不得标记 R013 完成。

## 7. 已知小偏差与遗留

1. Vue 抽屉为浮层（SettingDrawer 560px），React 复用内联 wk-channel-create 面板容器（与 IM 切片同一偏差，浮层壳属全局 Drawer 原语课题）。
2. **预览形态**：Vue 点「预览」打开 EmbedChannelPreview 模态（区分 iframe/widget 模式、草稿外观、locale 透传）；React 现已在当前页面打开 720px 右侧预览 drawer，使用 `onPreviewSession` 获取短期 token，提供 iframe 设备框、widget 模拟宿主页/浮窗、加载态、遮罩/Escape/关闭按钮。草稿外观与 locale 透传及真实后端浏览器验证仍未完成。
3. **prod 通配符闸**：Vue 以 import.meta.env.PROD 决定 wildcard 校验；React 端 `validateEmbedAllowedOrigins(origins, prod=false)` 显式传参，当前调用未接生产标志（默认 dev 语义），接生产标志需协调者定夺（route page 可按 location/host 注入）。
4. admin 门控：Vue 非管理员禁用字段/隐藏页脚；React 页面即管理壳，未复刻字段级禁用（服务端仍强制），与 IM 切片一致。
5. 轮换结果判定：Vue 直接读轮换响应；React onRotateEmbed 契约为 Promise<void>，实现以 onEmbedDetail 复取成功作为成功信号（复取失败显示 resetKeyFailed）。如需精确，可后续把 onRotateEmbed 升级为返回渠道。
6. Vue embed 卡片 admin 下拉（预览/启停）未复刻为下拉：开关与删除保留为卡片按钮，预览入口在部署步。
7. 新增内联样式仍以语义 class（wk-embed-*）存于 page.tsx，与 IM 向导样式一并待协调者批准后并入 styles.css 的 wk-integrations 区块。
8. 遗留登记建议：(R013/N028-(2)) embed 配置抽屉 → **本切片已完成步骤 1-6 主体及基础预览 drawer**（见第 2 条）；逐值视觉对照、admin 门控、prod 通配符注入、草稿外观/locale 透传和真实后端预览仍未验收；(3) API playground 分步抽屉（SSE）→ 未动，维持原登记。
