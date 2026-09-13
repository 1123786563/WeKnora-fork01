# R013 · integrations 深水区：IM 4 步向导对齐（Vue→React 一致性）

- 日期：2026-09-13（执行于 09-14 深夜档）
- 工作区：`.worktrees/react-multiclient`（分支 `codex/react-multiclient`，未自行 commit；e6f70f2e / 487aa730 已由协调者顺手入库，`packages/views/src/integrations/page.tsx` 最新态仍为工作区未提交修改，待复核）
- 独占改动范围：`packages/views/src/integrations/**`、`apps/web/src/integrations/**`（未触碰 knowledge-bases / App.tsx / documents / faq / organizations）
- 基准：`frontend/src/components/IMChannelPanel.vue`（1476 行）、`frontend/src/config/integrations.ts`
- 现状：`packages/views/src/integrations/page.tsx` + `registry.ts`（紧凑表单上线形态，即遗留 R013/N028 (1)）

## 1. 完成项

| # | 项 | 结果 |
|---|---|---|
| 1 | 精读 Vue 基准并输出差异表 | ✅ 见 §2 |
| 2 | IM 向导：步骤结构/顺序/校验/载荷 + 分平台凭证字段表 + 微信二维码 | ✅ 见 §3–§5 |
| 3 | embed 配置抽屉 | ➡ 工作量超预算，按任务第 5 条输出精确交接，见 §6 |
| 4 | 测试与类型检查 | ✅ views 42/42（基线 18 + 新 24）、web 6/6（基线 1 + 新 5）、`pnpm run typecheck:web` 通过 |

## 2. 差异表（Vue 基准 → React 改造前 → 本次改造后）

| 维度 | Vue（IMChannelPanel.vue） | React 改造前 | React 改造后 |
|---|---|---|---|
| 新建形态 | SettingDrawer 4 步向导（L72–579） | 内联紧凑表单（agent id 文本框 + 平台下拉 + 名称 + 凭证 JSON textarea） | 4 步向导面板：步骤条（active/done）+ 上一步/下一步/保存页脚 |
| 步骤顺序 | 基本信息→连接设置→文件存储→平台凭证（stepTitles L651–656） | 无步骤 | 同序（`IM_WIZARD_STEPS`，测试锚定） |
| 绑定智能体 | t-select（listAgents，L107–109） | 自由文本 agent id | select（agents prop，路由页注入）；无 agents 时回退自由文本 |
| 平台默认值 | onPlatformChange L810–838 + watch L788–801（wechat→longpoll/full；mattermost/yunzhijia→webhook/stream；yunzhijia 默认凭证；thread 不支持回 user；未改名跟随平台默认名） | 完全缺失 | `applyImPlatformChange` 逐条移植（含 post_to_main=false、yunzhijia {timeout_seconds:10, allowed_webhook_host_suffix:'yunzhijia.com'}） |
| 接入/输出/会话 | mode、output_mode chips；session_mode chips + thread 平台白名单（L784–786）；wechat 隐藏接入区（L149） | 三个字段完全不存在（请求载荷也不含） | chips（同禁用规则：mattermost 禁 WebSocket；thread 按白名单禁用）+ 提示文案 |
| 文件存储 | 第 3 步文件知识库 select（L227–239） | 无 | select（knowledgeBases prop）+ placeholder/hint |
| 凭证字段 | 分平台×模式字段表（L246–531） | 任意平台共用一个 JSON textarea | `imCredentialFields(platform, mode)` 字段表逐行渲染（text/password/number/switch、placeholder、hint、必填星号） |
| 平台控制台链接 | 各平台 doc-link + consoleTip（L249–451）、feishu/lark 双开放平台（L679–683） | 无 | `imConsoleLink`（8 平台 + lark/feishu 分流；yunzhijia/wechat 无） |
| 微信二维码 | 扫码绑定→api.qrserver.com 出图（L865）→500ms 轮询（L909）→confirmed 回填三元组（L888–893）/expired 停止 | JSON textarea | 见 §5 |
| 校验 | 步骤 0 缺 agent → selectAgentHint（L692–698）；保存：wechat 缺 bot_token（L1021–1024）、yunzhijia 缺 send_msg_url + normalize（L840–848, L1025–1033） | 仅 JSON.parse | `validateImWizardStep` / `validateImWizardSave`（同 key、同 normalize 副作用）；未用原生 required，与 Vue 一致（警告内联展示） |
| 创建载荷 | platform/name(平台名兜底)/mode/output_mode/session_mode/knowledge_base_id(''归一)/credentials（L1053–1061） | platform/name/credentials | 完全一致（`buildImCreatePayload`） |
| 更新载荷 | name/mode/output_mode/session_mode/knowledge_base_id/credentials/enabled/agent_id?（L1036–1045） | 仅 {name} | 完全一致（`buildImUpdatePayload`，agent_id 仅在有值时携带） |
| 编辑入口 | 点卡片打开同一抽屉（L957–993）：平台 select 禁用、enabled 开关（L136–143）、webhook 编辑显示回调地址+复制（L210–223, L940–947） | 仅内联改名 | 点卡片打开预填向导：平台禁用、enabled 开关、回调地址 + 复制（`imCallbackUrl` = origin + /api/v1/im/callback/:id） |
| 卡片动作 | 点击开抽屉 + 下拉开关 + 删除 | Edit(改名)/开关/删除 | IM 卡 Edit 按钮移除（编辑走向导），开关/删除保留；embed 不受影响 |
| i18n | vue locales | messages.ts 部分回退 | 新增 67 键 × 5 locale 逐字回退层（见 §4） |

## 3. 落点与新增文件

- 新增 `packages/views/src/integrations/imWizard.ts`：纯逻辑（步骤表、平台默认、thread 白名单、凭证字段表、console 链接、步骤/保存校验、create/update 载荷、回调地址、微信 QR 工具、`imWizardFormFromChannel` 预填）。
- 新增 `packages/views/src/integrations/imWizard.test.ts`：24 条纯逻辑测试（TDD 先红：模块缺失 → 后绿 24/24）。
- 改造 `packages/views/src/integrations/page.tsx`：`ImCreateForm`（JSON textarea）整体替换为 `ImWizardPanel`；页面持有向导状态机（step/form/nameTouched/editing/enabled/warning）与微信 QR 轮询状态机；IM 卡片点击 → 预填编辑；`IntegrationActions.onCreateIm` 签名升级为 `(input: { agentId; payload })`（唯一消费方 IntegrationsRoutePage 同步更新）；新增 `agents` / `knowledgeBases` props 与 `IntegrationWeChatQrPorts`。
- 改造 `apps/web/src/integrations/IntegrationsRoutePage.tsx`：注入 `client.configuration.agents.list()`（Vue listAgents 对应）、`client.knowledgeBases.list()`（第 3 步选项）、`wechatQr` 端口（`client.embed.im.wechat.qrCode/status`，api-client 既有端点，未改 api-client）。
- 新增 `apps/web/src/integrations/imWizardRender.test.tsx`：jsdom + createRoot 交互渲染测试 5 条（步骤条文案、step0 拦截、wecom 全流程载荷快照、wechat 隐藏接入区 + 保存闸、卡片编辑预填 + 回调地址 + 更新载荷）。
- 改造 `packages/views/src/integrations/view.ts`：`unresolvedCopyKeys` 纳入向导键（5 locale 无裸键泄漏，既有测试守护）。
- 改造 `packages/views/src/index.ts`：导出向导 API 与新类型。

## 4. i18n 处理与报备

- 需要的 67 个键（65 个 `agentEditor.im.*` + 下一步/上一步）在 **5 个 Vue locale 全部存在**（脚本从 `frontend/src/i18n/locales/{zh-CN,en-US,ja-JP,ko-KR,ru-RU}.ts` 的 agentEditor.im 子树逐字提取，零缺失）。
- 落点：`packages/views/src/integrations/imWizardMessages.ts`（生成式逐字回退表，33.7KB）+ `messages.ts` 第二层回退合并。未写入 `packages/i18n/src/generated/integrations.ts`——这些键属 agentEditor.im 子树而非 integrations 块，与该文件「integrations block 逐字移植」的注释边界不符；如需上游迁移至 @weknora/i18n 请协调者定夺。
- 两个页脚键为**新建 integrations 域键**（Vue 上游分别是 common.next / datasource.back）：`integrations.wizard.next`← Vue `common.next`（下一步/Next/次へ…）、`integrations.wizard.back`← Vue `datasource.back`（上一步/Back…）。因共享 @weknora/i18n 的 `common.next` 绑定分页文案（下一页），不能直接复用；值仍逐字取自 Vue locale。**向协调者报备此命名。**

## 5. 微信二维码：外部服务依赖与降级行为（按要求记录）

- Vue 依赖外部服务 **api.qrserver.com** 生成二维码图片（`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=<encodeURIComponent>`，IMChannelPanel.vue L865）。React 侧**按原样对齐**（`wechatQrImageUrl`，有测试锚定同一 URL）——未替换为本地 QR 库，避免引入新依赖/行为漂移。
- 状态机对齐：扫码绑定 → wait → scaned（绑定中...）→ confirmed（回填 bot_token/ilink_bot_id/ilink_user_id 并停止）/ expired（停止 + 「二维码已过期」重取按钮）。轮询节奏同 Vue：后端长轮询返回后 500ms 再发下一次（L907–910）；瞬时网络错误吞掉继续轮询（L904–906）。
- 降级行为：生成失败 → 内联错误文案 + 停留在未绑定态（对齐 Vue MessagePlugin.error + 不改绑定态）；端口未接线（`actions.wechatQr` 缺省）→ 内联提示端口缺失，按钮仍可重试。保存闸：未绑定时保存被 'agentEditor.im.wechatScanBind' 拦截（渲染测试覆盖）。
- 端点 `/api/v1/wechat/qrcode(+/status)` 由 `client.embed.im.wechat` 既有实现提供（packages/api-client 未改动，保持他人归属）。

## 6. embed 配置抽屉——精确交接（未实施）

**Vue 基准** `frontend/src/components/AgentEmbedChannelPanel.vue`（1549 行）：

| 区块 | 行号 | 内容 |
|---|---|---|
| 抽屉骨架 + 步骤条 | L70–89 | SettingDrawer（storage-key setting-drawer:embed-channel, 560px）；步骤条可点击跳转（goToWizardStep L640–643）；footer 上一步 L74–78 |
| stepTitles（5/6 步） | L601–611 | 渠道/安全/能力/外观/Webhook（+编辑时 部署）；drawerConfirmText L615–617；步骤越界守卫 watch L714–718 |
| 第 1 步 渠道 | L92–122 | agent select（必填）、enabled 开关（编辑+admin L104–111）、name + nameDesc L118 |
| 第 2 步 安全 | L125–152 | origins textarea + originsError 内联（L129–136）；rate_limit_per_minute 1–600（L138–143）；rate_limit_per_day 1–1e6（L145–150） |
| 第 3 步 能力 | L155–204 | welcome_message；show_suggested_questions / allow_web_search / allow_file_upload 开关行 + agent 能力缺失警示（L181–183、L194–196） |
| 第 4 步 外观 | L207–253 | page_title、header_title_mode、widget_position、default_locale、primary_color（HEX 取色器）、挂件预览（L241–251） |
| 第 5 步 Webhook | L256–281 | webhook_url、webhook_secret（password，已配置时占位符 L271）；仅创建显示 deployAfterSaveHint（L277–280） |
| 第 6 步 部署（编辑） | L284–386 | iframe/widget/secure 三 tab（L293–297）+ 场景提示 + token 警示（L306–309）+ 代码面板（预览 openPreviewFromDrawer L984–991、复制）；secure 服务端 node/go 示例（L331–349）；渠道密钥块：显示/显隐/复制/重置（popconfirm，L352–383） |
| 校验 | L619–634 | 步骤 0 缺 agent；步骤 1 validateAllowedOrigins（originsValidationMessage L876–880：required / wildcard_prod / invalid:{origin}） |
| 保存载荷 | L896–982 | name（默认「智能体名 · 网页嵌入」L596–598）/ welcome_message / allowed_origins / 两个限流 / primary_color / page_title / header_title_mode / show_suggested_questions / widget_position / allow_web_search / allow_file_upload / default_locale(''归一) / webhook_url(''归一) / webhook_secret(空则省略) / enabled / agent_id；创建成功跳部署步（L966）；API 报错映射 mapOriginsApiError（L882–894） |

**React 落点**：
- 纯逻辑：新建 `packages/views/src/integrations/embedWizard.ts`，镜像 imWizard.ts（步骤表、默认值、步骤校验、create/update 载荷）。origins 校验复用 Vue 语义（parse/validateAllowedOrigins 在 Vue utils，需随键一起移植或引用 @weknora/domain 既有实现——需先核查 `grep -rn validateAllowedOrigins frontend/src`）。现有 `form.ts buildEmbedUpdatePayload` 已守住服务端字段保真，可在新载荷构建器中并入或被替换（`form.test.ts` 两条断言保持绿）。
- UI：`packages/views/src/integrations/page.tsx` 的 `EmbedCreateForm` → `EmbedWizardPanel`（步骤 1–5）；卡片点击现仅 embed=开预览新标签页（route page openEmbed），需改为开抽屉、预览入口移入部署步。
- 集成：`apps/web/src/integrations/IntegrationsRoutePage.tsx` 增加 embed 详情拉取（`client.embed.channels.get`）、previewSession（已有）、rotateToken（已有 onRotateEmbed）。
- i18n：`embedPublish.step*/section*/origins*/rateLimit*/welcome*/... ` 约 40 键 × 5 locale，用本切片的提取脚本模式（/tmp/im-keys.json 管道已验证）。
- 建议拆分（按价值序）：① embedWizard.ts 纯逻辑 + 载荷（小，先落）；② 步骤 1–5 面板 + 校验（中）；③ 部署步（snippet 生成器 + secure server 示例 + 渠道密钥显隐/重置 + 预览模态，大，可独立一切片，涉及 EmbedChannelPreview.vue 的 React 对应物）。

## 7. 共享文件与遗留

- 未触碰：`apps/web/src/styles.css`、`packages/api-client/**`、其他切片目录。向导步骤条新样式（`.wk-im-steps/.wk-im-step/.wk-im-step-num/.wk-im-step-title/.wk-im-legend/.wk-im-step-body`）暂以**内联样式 + 语义 class** 存于 page.tsx；后续如需视觉精修，建议协调者批准后把这段样式并入 styles.css 的 wk-integrations 区块（约 15 行）。
- 遗留登记更新建议：(R013/N028-(1)) IM 4 步向导 + 分平台凭证 + 微信二维码 → **本切片已完成**（紧凑表单已下线）；(2) embed 配置抽屉 → 见 §6 交接；(3) API playground 分步抽屉（SSE）→ 未动，维持原登记。
- 已知小偏差：① Vue 抽屉为浮层，React 为内联面板（复用既有 wk-channel-create 容器；浮层壳属于全局 Drawer 原语课题）；② 编辑入口由「内联改名」改为「卡片点击开向导」后，IM 的内联改名不再可达（embed 保留）；③ 微信 QR 图片服务与 Vue 同源依赖 api.qrserver.com（外网不可达时与 Vue 表现一致：图挂、提示可重试）。

## 8. 测试证据

```
npx tsx --test packages/views/src/integrations/*.test.*
  tests 42 / pass 42 / fail 0   （基线 18 + imWizard.test.ts 24）
cd apps/web && npx tsx --test src/integrations/*.test.*
  tests 6 / pass 6 / fail 0     （基线 tenant 1 + imWizardRender.test.tsx 5）
pnpm run typecheck:web → 通过（无输出错误）
```

关键断言锚点：wecom websocket 创建载荷 deepEqual（platform/name=企业微信/mode/output_mode/session_mode/knowledge_base_id=''/credentials）；slack 编辑更新载荷 deepEqual（enabled=false、agent_id 透传、回调地址 https://…/api/v1/im/callback/ch-1）；wechat 平台隐藏「接入与输出」、保存被「扫码绑定微信」拦截。
