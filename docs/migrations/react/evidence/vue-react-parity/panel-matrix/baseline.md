# 面板扫描项全量基线（Phase II Batch-1）

- 日期：2026-09-24
- 基线 run：`auto-scan/2026-09-24T02-39-15`（全量 115 项：60 静态/ix + 55 px 面板项；spec：docs/specs/2026-09-24-interaction-panel-parity.md）
- 入表提交：73ed67375（55 项 px-* + hoverCss 机制 + postSettle）
- 口径：pixdiff 容差 8、1280×720 稳态截图；豁免须像素级归因+根因代码级定位（台账 #24/#25 判例）

## 总览

| 口径 | 数值 |
|---|---|
| px 面板项 | 55（入表 55，无缺失） |
| 基线 0.00% | 16 项 |
| 基线 >0 | 39 项（Phase II 工单池） |
| 触发器未命中（warning） | 5 项（见下） |
| 静态页回归（存量，非本批引入） | settings-general 5.439%（根因见文末）；~~settings-integration-api 4.284~~ 已修（批 4，mode-callout 补齐） |

## 批 1（全局壳+对话 12 项）收敛状态（截至 69acbada9：12/12 全收敛，chat 4 工单见 auto-scan/2026-09-24T14-57-58）

| 项 | 基线 | 终值 | 状态 |
|---|---|---|---|
| px-shell-user-menu | 0 | 0 | ✅ 直接零 |
| px-shell-session-more | 1.166 | 1.166 | ⚠️ 豁免 #24（SP13 React-only「分享」菜单项） |
| px-chat-sandbox | 15.765 | **0.00** | ✅ 已收敛（f71293b8f） |
| px-chat-addtokb | 假零(未命中) | **0.00** | ✅ 已收敛（1d3c0c4c1；真值 75.387→0） |
| px-chat-reqinfo | 3.114 | **0.00** | ✅ 已收敛（96162ac2a） |
| px-chat-agent-selector | 2.472 | **0.00** | ✅ 已收敛（11205900e） |
| px-chat-model-selector | 1.64 | **0.00** | ✅ 已收敛（69acbada9） |
| px-chat-attach-tooltip | 0.342 | 0.001 | ⚠️ 残差豁免 #25（8px 图标 AA 相位） |
| ix-chat-header-menu / ix-chat-mention | 0 / 0 | 0 / 0 | ✅ 复用核对通过 |
| ix-kb-list-create | 8.04(瞬态) | 0 | ✅ 复跑 0%（基线 run 与并行单页扫描重叠所致瞬态） |
| ix-agents-create / ix-orgs-create | 0 / 0 | 0 / 0 | ✅ 复用核对通过 |
| ix-kb-doc-detail | 0 | 0 | ✅ 复用核对通过 |

## 55 项基线明细（基线 run 数字；批 1 终值见上表）

### 批 1：全局壳+对话（8 项）
| id | 基线 | 备注 |
|---|---|---|
| px-shell-user-menu | 0 | |
| px-shell-session-more | 1.166 | 豁免 #24 |
| px-chat-sandbox | 15.765 | 根因 A |
| px-chat-addtokb | 0(未命中假零) | 触发器已修（title aria + pickLast），真值 75.387，根因 B |
| px-chat-reqinfo | 3.114 | 已收敛 0.00 |
| px-chat-agent-selector | 2.472 | 根因 C |
| px-chat-model-selector | 1.64 | 根因 D |
| px-chat-attach-tooltip | 0.342 | 已收敛至 0.001，残差豁免 #25 |

### 批 2：知识库域（19 项）——已收敛（run `auto-scan/2026-09-24T08-41-39`，19/19 全 0.00%）
| id | 基线 | 终值 | 备注 |
|---|---|---|---|
| px-kb-faq-breadcrumb | 0.239 | 0.00 | A4 面包屑菜单（27ee21189） |
| px-kb-demo-breadcrumb | 0.234 | 0.00 | 同上 |
| px-kb-wiki-breadcrumb | 0.237 | 0.00 | 同上 |
| px-kb-wiki-tab-graph-breadcrumb | 0.254 | 0.00 | 27ee21189 + kbList type 透传修复（KnowledgeGraphPage） |
| px-kb-wiki-tab-wiki-breadcrumb | 0.255 | 0.00 | 同上（WikiPage） |
| px-kb-faq-card-more | 0 | 0.00 | ✅ |
| px-kb-faq-kb-info | 3.276 | 0.00 | popover+drawer（02a6f91b7） |
| px-kb-faq-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-faq-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-demo-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-demo-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-wiki-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-wiki-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-faq-doctype-select | 0(未命中假零) | 0.00 | 触发器已修（代表页改 kb-demo），真值 0 |
| px-kb-wiki-doctype-select | 0 | 0.00 | ✅（同款选择器，wiki 页命中） |
| px-kb-wiki-tab-wiki-newpage | 76.154 | 0.00 | WkDialog→tdesign Dialog 同构平移 + mouseAway 整定（hover 指针工件，见 auto-scan.mjs 注） |
| px-kb-wiki-tab-wiki-newdir | 1.235 | 0.00 | WikiFolderActions 弹层平移（触发器 warning 已消） |
| px-kb-wiki-tab-wiki-treeview | 2.764 | 0.00 | 树形视图/目录行/reader 头部平移（选择器多匹配假设证伪：双端单匹配同位） |
| px-kb-wiki-tab-wiki-overview | 3.201 | 0.00 | 索引概览 reader 头部平移 |

### 批 3：设置域（18 项）——B3 串行流收敛（截至 9f274c977；run auto-scan/2026-09-24T12-48-10 回归）
| id | 基线 | 终值 | 状态 |
|---|---|---|---|
| px-userprofile-change-password | 2.949 | **0** | ✅ 豁免 #27 回退复勘后已修（7d351553a）：真因=§3 六条 .password-popup-* 规则以 .wk-settings-drawer-root .user-profile 祖先锚定而弹层 portal 到 body 致规则失配（b1f440178 引入、早于 B3c），改锚 .user-profile-password-popup-overlay；04-07-05 轮终值 0 |
| px-mymemory-usage-hint | 0 | 0 | ✅ |
| px-models-model-card | 62.438 | 0 | ✅ 已收敛（批 3 余量：wk-model-editor 居中壳→SettingDrawer model-editor-drawer 家族 + ModelEditorDialog 解剖移植 + CredentialResource 端口；19-08-35 终值 0） |
| px-models-card-more | 0.764 | 0.003 | ✅ §13d dropdown 全局块带零（AA 残差） |
| px-parser-engine-builtin | 65.353 | 0.002 | ✅ 已收敛（cae2d3269，单字形 AA 残差） |
| px-storage-backend-card | 21.599 | 0.002 | ✅ 已收敛（35ccd539c；AA 残差） |
| px-storage-card-more | 1.14 | 0 | ✅ 已收敛（35ccd539c，触发器已命中） |
| px-vectorstore-add-db | 10.629 | 0.121 | ✅ 已收敛（35ccd539c；focus 边框相位残差，复扫 0.002-0.121 波动） |
| px-vectorstore-pg-card | 78.729 | 0 | ✅ 已收敛（35ccd539c；env 卡可点守卫根因） |
| px-sandbox-what-is-hint | 0 | 0 | ✅ |
| px-envvars-sandbox-key-hint | 1.837 | 0.011-1.837 | ⚠️ hint popover 本体同构；波动值来自 settings-envvars 静态底差相位（12-31-57 轮 0.011%、12-48-10 轮 1.837%——静态底色轮换项，交互弹层零差） |
| px-skills-add | 56.966 | 0 | ✅ 已收敛（9f274c977；mouseAway 指针工件判例 #26） |
| px-mcp-add-service | 54.278 | 0 | ✅ 已收敛（批 3 余量：wks-modal 居中壳→SettingDrawer mcp-drawer--{transport} 家族 + 列表卡换 Vue 原名 service-card 家族（添加卡 service-card--add）+ McpServiceDialog 解剖移植；19-08-35 终值 0） |
| px-websearch-provider-card | 14.26 | 0.002 | ✅ 已收敛（35ccd539c；AA 残差） |
| px-websearch-card-more | 0.775 | 0 | ✅ 已收敛（35ccd539c，触发器已命中） |
| px-platform-api-keys-create | 62.964 | 0 | ✅ 已收敛（批 3 余量：pak-drawer 自制壳→SettingDrawer api-key-create-drawer 家族 + 整面板 Vue 原名类系（api-key-table/chip/popconfirm/t-dialog）+ PLATFORM_API_KEY_CAPABILITY_GROUPS 分组全选；19-08-35 终值 0） |
| px-members-rbac-hint | 10.919 | **0.043** | ✅ 豁免 #27 回退复勘后已修（7d351553a/ddad150ae）：真因=.permissions-compact 族于 72de117db 弃置后从未移植 + 面板 DOM 缺 role/perm-item icon、overlayInnerStyle、is-me 判定折叠分歧；§23 按 unscoped 源（TenantMembers.vue:2333-2461）重平移 + DOM 补齐；04-07-05 轮终值 0.043（AA 残差级） |
| px-ollama-redetect | 2.496 | **0** | ✅ 豁免 #27 回退复勘后已修（da187f072）：真因=React 检测失败走自研 wk-settings-toast 右上角 vs Vue t-message 顶部居中（异构早于 B3c），10 处 pushSettingsToast 换 tdesign-react MessagePlugin 同构；04-07-05 轮终值 0 |

### 豁免 #27 回退复勘（2026-09-25，run auto-scan/2026-09-25T04-07-05）

判例 #27 的三项实证项经逐项深度复勘**全部证伪**（详见上行终值列）：

- px-userprofile-change-password：所称 2.949% 静态漂移不存在（settings-userprofile 静态现场 0.000%）；真因为弹层内层排版规则锚定失配——弹层同构判据系 #27 错记。
- px-ollama-redetect：所称静态底差不存在（settings-ollama 静态现场 0.000%）；真因为失败提示呈现异构（自研 toast vs t-message），#27 的"静态底差同源"归因被证伪。
- px-members-rbac-hint：10.919% 是该交互项自身 diff 被 #27 错记为静态底差（settings-members 静态实为 0.084%）；真因为弹层 CSS/DOM 双缺口（含 is-me 判定随 system-admin 折叠的语义分歧）。

教训：判据①"交互项终值与静态基线同值"不构成弹层同构证明——同值可能同为缺陷放大；豁免前必须弹层 rect 级 DOM 探针实证。

批 3 小结：18 项中 11 项 0/AA 残差（含 parser 续作）、3 项静态底差转交（弹层同构实证）、
4 项基线即零；剩 3 项（models/mcp/api-keys）为居中壳/自制壳 vs SettingDrawer 右抽屉的
结构差，根因均已代码级锁定（见上行），移交后续批次。

### 批 4：系统/集成/免登录（10 项）
| id | 基线 | 备注 |
|---|---|---|
| px-system-auth-priority | 2.984 | |
| px-system-create-user | 84.547 | A5 popover vs dialog |
| px-integrations-agent-filter | 1.562 | A6 |
| px-integrations-add-channel | 59.292 | B3 |
| px-integration-embed-agent-filter | 1.558 | A6 |
| px-integration-api-create-key | 71.577 | B6 |
| px-login-lang | 0.001 | AA 残差级 |
| px-register-lang | 0.001 | |
| px-login-register-confirm | 0 | ✅ |
| px-register-register-confirm | 0 | ✅ |

### 批 4 收敛状态（截至 2026-09-25T03-07-18：11/11 ≤0.113 全收敛，含静态项 settings-integration-api；提交 36a610456/1ce982001/891d0ab69/5fbde2205/7edec47b3）

| 项 | 基线 | 终值 | 状态 |
|---|---|---|---|
| px-system-auth-priority | 2.984 | **0.00** | ✅ 前任 t-popup 同构续做收口（priority hint bottom-start→bottom-left，#15） |
| px-system-create-user | 84.547 | **0.00** | ✅ 同上（CreateUser/ResetPassword 弹层换 t-popup 锚定，TInput 悬空引用修复） |
| px-integrations-agent-filter | 1.562 | 0.069 | ⚠️ 残差豁免 #25 同类（下拉同构后余边框亚像素+字形栅格化 AA；无效 calc(100%+4px) 修复+度量对齐，1.2-1.35px 平台期实证） |
| px-integrations-add-channel | 59.292 | 0.078 | ⚠️ 残差豁免 #25 同类（B3 抽屉换 SettingDrawer t-drawer 同构 chrome + step0 体平移；页脚 t-button 亚像素 AA） |
| px-integration-embed-agent-filter | 1.558 | 0.069 | ⚠️ 残差豁免 #25 同类（同 agent-filter 判例，embed 相位单独微调 1.05px） |
| px-integration-api-create-key | 71.577 | 0.113 | ⚠️ 残差豁免 #25 同类（B6 换 api-key-create-drawer 同构；残差=mode-callout 静态底差消除后余 checkbox/hint AA） |
| px-login-lang | 0.001 | 0.001 | ⚠️ 豁免 #25 同类取证：7 像素、bbox(1014-1017,31-62) 语言图标字形 AA，几何对齐 |
| px-register-lang | 0.001 | 0.001 | ⚠️ 同上（同源图标） |
| px-login-register-confirm | 0 | 0 | ✅ 基线即零 |
| px-register-register-confirm | 0 | 0 | ✅ 基线即零 |
| settings-integration-api（静态） | 4.284 | **0.00** | ✅ 已修（5fbde2205：direct_header 补 mode-callout--warning 警示框 + signed_token 提示框） |

## 触发器未命中（基线 warning；批 2 已全消，余 A9 两项批 3 处置）

- ~~px-kb-faq-doctype-select~~：已修（代表页改 kb-demo，真值 0.00）。
- px-storage-card-more / px-websearch-card-more：React 端 `.backend-card__action-btn` / `.provider-card__more` 未命中（A9 React lost，与矩阵一致）。
- ~~px-kb-wiki-tab-wiki-newdir~~：已修（WikiFolderActions 目录操作弹层平移，0.00%）。

## 批 1 待收敛项根因（已代码级定位）

- **A. px-chat-sandbox（15.765%）**：两段问题。①ChatRoutePage.tsx:1953 的 sandboxToggleSlot 用 `openTerminal()`（异步 provision 终端，fixture 后端无沙箱→静默失败→面板永不打开）；page.tsx:633 内建默认 slot 本是即时 `setTerminalOpen(true)`。②React 抽屉（page.tsx:692）只有 TerminalPanel；Vue 是 SandboxSidePanel.vue（375 行，产物/终端/桌面 三 tab + 空态「本会话尚未生成可下载的文件」）。收敛=端口 SandboxSidePanel + 即时开面板。
- **B. px-chat-addtokb（75.387%）**：Vue=t-drawer--right 右侧抽屉（在线编辑 Markdown 知识·选库）；React=wk-bookmark-dialog 居中 dialog（760×532）。收敛=React 改右侧 drawer 同构（packages/views bookmark 弹层 + Vue 对应源）。
- **C. px-chat-agent-selector（2.472%）**：双端都开 agent-selector 下拉；Vue dropdown (316,432,220×219) vs React (316,371,222×200)——锚定方向（下开 vs 上开）+ 条目度量差。收敛=对齐 packages/views/src/chat/agent-selector.tsx 的锚定与行高。
- **D. px-chat-model-selector（1.64%）**：Vue `.model-selector-trigger` 开自定义 model-selector-overlay（思考/对话模型 列表 282×84）；React `.wk-chat-model-chip` 是 native `<select>`（弹层不进截图）。收敛=React 端口自定义 overlay（Input-field.vue:2788 + ModelSelector）。

## 存量静态页分歧（非本批引入，登记待归属域）

- **settings-general 5.439%**：React-only「套餐与额度」卡（SP14 T1，GeneralPreferencesPanel.tsx:256 已注记豁免传导项；像素归因见 task-12a）。整页下移 ~110px。
- ~~**settings-integration-api 4.284%**~~：已修（批 4 5fbde2205）。principalMode=direct_header 状态下 Vue 渲染 `mode-callout--warning` 警示框（含 directWarningDetail 第二行，ApiIntegrationSettings.vue:210-216）；React 原只有 `wk-muted--warn` 纯文本一行。补 mode-callout 双变体（warning/signed）后复扫 0%。

## 扫描器机制注记

- hoverCss：hover 门控触发器（会话行「更多」、附件 tooltip）真实 hover 序列 + hoverWait 停留，hover 后不冻结（揭示过渡走完），定格交给点击后稳态门。
- pickLast：消息工具栏类触发器双端 DOM 数量不同（React 历史消息也渲染 toolbar），取最后一个可见匹配=最新一条回答，避免滚出视口错位取景。
- postSettle：面板项可覆盖点击后 settle（login 创建账户切注册表单用 3200）。
