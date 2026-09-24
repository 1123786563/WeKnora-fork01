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
| 静态页回归（存量，非本批引入） | settings-general 5.439%、settings-integration-api 4.284%（根因见文末） |

## 批 1（全局壳+对话 12 项）收敛状态（截至 96162ac2a）

| 项 | 基线 | 终值 | 状态 |
|---|---|---|---|
| px-shell-user-menu | 0 | 0 | ✅ 直接零 |
| px-shell-session-more | 1.166 | 1.166 | ⚠️ 豁免 #24（SP13 React-only「分享」菜单项） |
| px-chat-sandbox | 15.765 | 15.765 | ⬜ 待收敛（根因 A） |
| px-chat-addtokb | 假零(未命中) | 75.387 | ⬜ 待收敛（根因 B） |
| px-chat-reqinfo | 3.114 | **0.00** | ✅ 已收敛（96162ac2a） |
| px-chat-agent-selector | 2.472 | 2.472 | ⬜ 待收敛（根因 C） |
| px-chat-model-selector | 1.64 | 1.64 | ⬜ 待收敛（根因 D） |
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

### 批 2：知识库域（19 项）
| id | 基线 | 备注 |
|---|---|---|
| px-kb-faq-breadcrumb | 0.239 | A4 面包屑菜单（5 页） |
| px-kb-demo-breadcrumb | 0.234 | |
| px-kb-wiki-breadcrumb | 0.237 | |
| px-kb-wiki-tab-graph-breadcrumb | 0.254 | |
| px-kb-wiki-tab-wiki-breadcrumb | 0.255 | |
| px-kb-faq-card-more | 0 | ✅ |
| px-kb-faq-kb-info | 3.276 | popover+drawer（.kb-info-button ↔ aria） |
| px-kb-faq-tagfilter-prefix | 0 | ✅ |
| px-kb-faq-tagfilter-suffix | 0 | ✅ |
| px-kb-demo-tagfilter-prefix | 0 | ✅ |
| px-kb-demo-tagfilter-suffix | 0 | ✅ |
| px-kb-wiki-tagfilter-prefix | 0 | ✅ |
| px-kb-wiki-tagfilter-suffix | 0 | ✅ |
| px-kb-faq-doctype-select | 0(未命中假零) | 触发器 `.doc-type-select` 未命中待修（kb 页文档工具栏 select） |
| px-kb-wiki-doctype-select | 0 | ✅（同款选择器，wiki 页命中） |
| px-kb-wiki-tab-wiki-newpage | 76.154 | B2：Vue t-dialog vs React wk-dialog 异构 |
| px-kb-wiki-tab-wiki-newdir | 1.235 | React 未检出触发器（warning） |
| px-kb-wiki-tab-wiki-treeview | 2.764 | |
| px-kb-wiki-tab-wiki-overview | 3.201 | React lost |

### 批 3：设置域（18 项）——B3 串行流收敛（截至 9f274c977；run auto-scan/2026-09-24T12-48-10 回归）
| id | 基线 | 终值 | 状态 |
|---|---|---|---|
| px-userprofile-change-password | 2.949 | 2.949 | ⚠️ 静态底差转交（判例 #27）：popconfirm 双端同构 (566,228,320×132)，弹层内仅 5px AA；2.949% 与 settings-userprofile 静态基线同值，全部来自分区静态底差 |
| px-mymemory-usage-hint | 0 | 0 | ✅ |
| px-models-model-card | 62.438 | 62.44 | ⬜ 未收敛：React wk-model-editor（dae977404 居中 dialog）vs Vue ModelEditorDialog（SettingDrawer 右抽屉 + model-editor-drawer 家族）；换壳+类名复刻待续 |
| px-models-card-more | 0.764 | 0.003 | ✅ §13d dropdown 全局块带零（AA 残差） |
| px-parser-engine-builtin | 65.353 | 0.002 | ✅ 已收敛（cae2d3269，单字形 AA 残差） |
| px-storage-backend-card | 21.599 | 0.002 | ✅ 已收敛（35ccd539c；AA 残差） |
| px-storage-card-more | 1.14 | 0 | ✅ 已收敛（35ccd539c，触发器已命中） |
| px-vectorstore-add-db | 10.629 | 0.121 | ✅ 已收敛（35ccd539c；focus 边框相位残差，复扫 0.002-0.121 波动） |
| px-vectorstore-pg-card | 78.729 | 0 | ✅ 已收敛（35ccd539c；env 卡可点守卫根因） |
| px-sandbox-what-is-hint | 0 | 0 | ✅ |
| px-envvars-sandbox-key-hint | 1.837 | 0.011-1.837 | ⚠️ hint popover 本体同构；波动值来自 settings-envvars 静态底差相位（12-31-57 轮 0.011%、12-48-10 轮 1.837%——静态底色轮换项，交互弹层零差） |
| px-skills-add | 56.966 | 0 | ✅ 已收敛（9f274c977；mouseAway 指针工件判例 #26） |
| px-mcp-add-service | 54.278 | 54.278 | ⬜ 未收敛：React wks-modal 自制居中壳（McpSettingsPanel:990）vs Vue McpServiceDialog（SettingDrawer + mcp-drawer--{transport} + #header-extra mcp-steps + footer-left 上一步 + width 680/min560/max920/storageKey mcp-config-v2）；添加卡类名 mcp-add-card→service-card--add；根因已锁待续 |
| px-websearch-provider-card | 14.26 | 0.002 | ✅ 已收敛（35ccd539c；AA 残差） |
| px-websearch-card-more | 0.775 | 0 | ✅ 已收敛（35ccd539c，触发器已命中） |
| px-platform-api-keys-create | 62.964 | 62.964 | ⬜ 未收敛：React pak-drawer 自制壳 + 简化勾选表单（PlatformApiKeysPanel:112）vs Vue SettingDrawer（api-key-create-drawer 家族 + 平台控制面/空间能力分组 + 全选行）；根因已锁待续 |
| px-members-rbac-hint | 10.919 | 10.919 | ⚠️ 静态底差转交（判例 #27）：role-hint popover 双端同构 (620,118,360×120) 内 0px；10.919% 与 settings-members 静态基线同值 |
| px-ollama-redetect | 2.496 | 2.197-2.496 | ⚠️ 静态底差转交（判例 #27）：弹层内 3px AA；差异与 settings-ollama 静态基线同源（轮间 2.197↔2.496 为静态底差相位波动） |

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

## 触发器未命中（基线 warning，除 addtokb/reqinfo 已修外）

- px-kb-faq-doctype-select：双端 `.doc-type-select` 未命中（kb-faq 页 FAQ 视图工具栏无该 select，kb-wiki 命中）——批 2 修正触发器或改代表页。
- px-storage-card-more / px-websearch-card-more：React 端 `.backend-card__action-btn` / `.provider-card__more` 未命中（A9 React lost，与矩阵一致）。
- px-kb-wiki-tab-wiki-newdir：React 无「新建目录」入口（B2）。

## 批 1 待收敛项根因（已代码级定位）

- **A. px-chat-sandbox（15.765%）**：两段问题。①ChatRoutePage.tsx:1953 的 sandboxToggleSlot 用 `openTerminal()`（异步 provision 终端，fixture 后端无沙箱→静默失败→面板永不打开）；page.tsx:633 内建默认 slot 本是即时 `setTerminalOpen(true)`。②React 抽屉（page.tsx:692）只有 TerminalPanel；Vue 是 SandboxSidePanel.vue（375 行，产物/终端/桌面 三 tab + 空态「本会话尚未生成可下载的文件」）。收敛=端口 SandboxSidePanel + 即时开面板。
- **B. px-chat-addtokb（75.387%）**：Vue=t-drawer--right 右侧抽屉（在线编辑 Markdown 知识·选库）；React=wk-bookmark-dialog 居中 dialog（760×532）。收敛=React 改右侧 drawer 同构（packages/views bookmark 弹层 + Vue 对应源）。
- **C. px-chat-agent-selector（2.472%）**：双端都开 agent-selector 下拉；Vue dropdown (316,432,220×219) vs React (316,371,222×200)——锚定方向（下开 vs 上开）+ 条目度量差。收敛=对齐 packages/views/src/chat/agent-selector.tsx 的锚定与行高。
- **D. px-chat-model-selector（1.64%）**：Vue `.model-selector-trigger` 开自定义 model-selector-overlay（思考/对话模型 列表 282×84）；React `.wk-chat-model-chip` 是 native `<select>`（弹层不进截图）。收敛=React 端口自定义 overlay（Input-field.vue:2788 + ModelSelector）。

## 存量静态页分歧（非本批引入，登记待归属域）

- **settings-general 5.439%**：React-only「套餐与额度」卡（SP14 T1，GeneralPreferencesPanel.tsx:256 已注记豁免传导项；像素归因见 task-12a）。整页下移 ~110px。
- **settings-integration-api 4.284%**：principalMode=direct_header 状态下 Vue 渲染 `mode-callout--warning` 警示框（含 directWarningDetail 第二行，ApiIntegrationSettings.vue:210-216）；React 只有 `wk-muted--warn` 纯文本一行（integrations/page.tsx:1572）。09-23 验收三轮 0%→数据态切到 direct_header 后暴露（功能测试或数据变化），非像素回归。

## 扫描器机制注记

- hoverCss：hover 门控触发器（会话行「更多」、附件 tooltip）真实 hover 序列 + hoverWait 停留，hover 后不冻结（揭示过渡走完），定格交给点击后稳态门。
- pickLast：消息工具栏类触发器双端 DOM 数量不同（React 历史消息也渲染 toolbar），取最后一个可见匹配=最新一条回答，避免滚出视口错位取景。
- postSettle：面板项可覆盖点击后 settle（login 创建账户切注册表单用 3200）。
