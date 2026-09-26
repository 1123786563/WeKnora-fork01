# 三期无弹层交互面扫描项基线（px2-*）

- 日期：2026-09-26
- 基线 run：`auto-scan/2026-09-26T04-04-22`（13 项 px2-*；PAGES 逗号过滤按 ALL_PAGES 序）
- 入表提交：ac5eccbb8（feat(parity): 三期交互面扫描项入表）
- 盘点口径：前两期（60 静态/ix + 55 px 面板）之上的**无弹层交互面**——tab 切换、视图切换器、分段控件、开关交互态、折叠/展开、树节点展开、分页器、步骤条、轮播；每类取代表入口，与既有清单严格去重
- 方法：源码盘点（frontend/src 权威源 + apps/web 对端）→ headless 双端存在性探针（:5174/:5175，登录态照 auto-scan.mjs，GUIDE_KEYS 屏新手引导）→ 与 auto-scan.mjs clickFirst 同款兜底链逐项点击核验 → 入表 → 基线摸底

## 总览

| 口径 | 数值 |
|---|---|
| px2 入表项 | 13（触发器全命中，warning 0） |
| 基线 ≈0（≤0.005%） | 8 项 |
| 基线 >1%（工单池） | 4 项 |
| 盘点后不入表（存疑/未发现/已覆盖） | 见文末清单 |

## 13 项基线明细

### kb 域（6 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-kb-wiki-tab | 0 | 面包屑 tab 文档→Wiki（span.breadcrumb-tab 双端同构，KnowledgeBase.vue:2424-2438） |
| px2-kb-graph-tab | 0 | 面包屑 tab 文档→图谱 |
| px2-kb-wiki-reader-tab | 0 | wiki reader 头部 tab 知识→摘要（.wiki-tab：Vue div / React button 标签异构同名类） |
| px2-kb-wiki-tree-expand | 0 | 目录树行点击展开（.wiki-directory-item；Vue WikiBrowser.vue:292 / React WikiPage.tsx:1249） |
| px2-kb-settings-nav | 7.478 | KB 设置抽屉 nav→分块设置；diff 集中在中部内容区（3×3 区域分析：中 15.9%/右下 14.2%），抽屉壳与左侧 nav 基本同构——分块设置 section 内容差异待工单归因 |
| px2-kb-settings-chunkswitch | 9.409 | 父子分块开关；**已知组件级异构**：Vue KBChunkingSettings.vue:113 是 t-switch，React chunkingSection.tsx:195-201 是 native `input[type=checkbox]`（aria-label=父子分块兜底命中）；diff 集中中部 24.8%（开关行+展开的父块参数行） |

### settings 域（5 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-settings-models-tab | 0.005 | 模型类型过滤 tab 全部→对话（.model-type-tabs，ModelSettings.vue:38） |
| px2-settings-sandbox-tab | 0.001 | 沙箱类型过滤 tab（SandboxSettings.vue:59） |
| px2-settings-mymemory-tab | 0 | 记忆状态过滤 tab 生效中→待确认（MemorySettings.vue:154；注意挂 mymemory 分区） |
| px2-settings-systemglobal-tab | 0.001 | 系统设置分区 tab 账户与访问→空间默认值（SystemSettings.vue:69；settings-system 分区是 SystemInfo 无 tab） |
| px2-settings-general-fontradio | 25.519 | 字号分段控件 正常→大；**全页弥散型差异**（3×3 全区域 6.4%~36.6%）。代码级候选归因：字号应用机制异构——Vue `useFont.setFontSize→applyFont()`（frontend/src/composables/useFont.ts:260-266）全站缩放，React `applyFontCssVariables` 只设 `--wk-font-scale` CSS 变量（apps/web/src/settings/GeneralPreferencesPanel.tsx:132-136），React 端消费面不足则视觉不缩放；另 React 点击后 `pushSettingsToast('成功')`（同文件 :219）而 Vue GeneralSettings 无 toast，提示呈现亦异构（判例 #27 da187f072 同族，该修复未覆盖此处）。待工单像素级归因 |

### system 域（1 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-settings-runtimequeues-autorefresh | 0 | 自动刷新开关（客户端 5s 轮询，无服务端写；clickAria 先行因 React 该页 2 个 switch 须唯一定位） |

### chat 域（1 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-chat-sidebar-collapse | 16.06 → **0.238（豁免，台账 #29）** | 会话侧栏折叠（.sidebar-toggle：Vue menu.vue:22 / React PlatformShell.tsx:1087 同名类同 `sidebar_collapsed` localStorage 键）。【2026-09-26 工单收敛】四项前置缺陷修复：①React 字号 zoom 机制缺失（`--wk-font-scale` 全仓零消费，Vue useFont html zoom 1.125 下 rail 67.5px vs React 60px）→applyFontCssVariables 补 zoom+main.tsx initFontPreferences 启动恢复；②`.wk-shell-1` 100vh 在 zoom 下溢出 90px→height:100% 高度链（#root 补 html,body,#root 100%）；③`.chat` 缺 `is-sidebar-collapsed` 类（Vue index.vue:4 同款，折叠后主区卡 1020px）→ChatSidebarCollapsedContext 通道；④composer chip 盒模型 border-box 30px→Vue 同构 content-box 28px+2×.5px border（0.25px zoom 乘法差经 clientHeight 取整放大成 scrollTop 1px 错位）。16.06→0.238（run 05-19-53）。残余 0.238%＝zoom 1.125 下个别文本行基线光栅 snapping 差（引擎 relayout 增量路径，无页面 seam，像素级+代码级取证见台账 #29）。顺带：px2-settings-general-fontradio 25.519→7.663（zoom 机制修复的红利，余量归该工单） |

## 序内污染声明（重验须知）

`px2-settings-general-fontradio`（写 `WeKnora_<uid>_fontsize`）与 `px2-chat-sidebar-collapse`（写 `sidebar_collapsed`）点击后 localStorage 跨 goto 存续，已置于 px2 块最末（其后无 authed 项）。单页重验其**前置**项不受影响；重验 `px2-chat-sidebar-collapse` 须按 ALL_PAGES 顺序连同 fontradio 一起过滤（`PAGES=px2-settings-general-fontradio,px2-chat-sidebar-collapse`，过滤保序）。与存量 `ix-kb-listview`（viewMode localStorage）同族先例。

## 盘点后不入表的类目（存疑/未发现/已覆盖）

| 类目 | 结论 | 证据 |
|---|---|---|
| 步骤条（t-steps） | **双端不存在** | `grep -rn "t-steps" frontend/src` 0 处（AgentEditorModal 的 "step" 均为 t-slider :step 参数）；无向导状态机 |
| 滑杆（t-slider） | **无无弹层入口** | 仅 AgentEditorModal.vue:610+（弹层内）与 RetrievalSettings.vue（挂 GlobalCommandPalette.vue:156 弹层内）；agents 卡片直击不打开编辑器（探针实证 modal=0/sliders=0） |
| 轮播切换（指示点） | **存疑不入表** | login/register `.swiper-pagination-bullet` 双端 4 枚实证存在，但 autoplay 4s 相位双端独立（Vue swiper delay:4000 disableOnInteraction:false，Login.vue:160-163；React setInterval 4000，LoginPage.tsx:111），扫描器无 post-action 重同步机制，入表必假阳；需先给 auto-scan 增加 postActionSync 能力 |
| 分页器翻页 | **fixture 无可用数据** | members「共 1 条数据 1 页」next disabled（探针实证双端）；mymemory/文档 chunk 列表 t-pagination 未渲染（探针 pag=0）；MemorySettings t-pagination `v-if="listTotal > pageSize"` 不满足；runtime-queues 加载更多按钮双端均未渲染（探针 hits=[null]，队列无更多任务） |
| 视图切换器 | **已覆盖** | ix-kb-listview（列表视图）+ px-kb-wiki-tab-wiki-treeview（树形视图）；网格为默认态点击无态变（探针实证 viewbtn-active 前后不变） |
| KB 目录树折叠/展开（KbFolderTree） | **fixture 无目录** | kb-demo/kb-wiki 均 .kb-folder-tree 不存在（探针实证；showFolderTree 需 KB 有目录数据）；树展开代表入口已由 px2-kb-wiki-tree-expand（wiki 目录树）承担 |
| 持久化开关类（chathistory 等） | **不入表（写风险）** | ChatHistorySettings.vue:189 `handleEnabledChange→debouncedSave()` 即时落库；同族 memory(workspace)/sandbox 开关均挂保存链——点击即写 fixture 且逐 run 交替态破坏基线确定性。已入表两开关均为纯客户端：runtime-queues 自动刷新（轮询开关）、KB 抽屉父子分块（表单态无保存不落库） |
| integration-api 分段控件 | **React 缺件（发现）** | 仅空间/直接传用户 ID/签名 Token 三段 radio：Vue 探针实证 radio-btn(6)+switch(1)，React 端均未检出（ApiIntegrationSettings.vue:114+ 有、React 对应面板无）——属 React 缺件差异，建议单独立工单而非扫描项（避免单端点击的服务端写风险） |
| chat 会话分组折叠 | **fixture 无分组** | 单会话 fixture 无分组头（探针 `[class*="session-group"]` 等未检出） |

## 复验命令

```sh
PAGES=px2-kb-wiki-tab,px2-kb-graph-tab,px2-kb-wiki-reader-tab,px2-kb-wiki-tree-expand,px2-kb-settings-nav,px2-kb-settings-chunkswitch,px2-settings-models-tab,px2-settings-sandbox-tab,px2-settings-mymemory-tab,px2-settings-systemglobal-tab,px2-settings-runtimequeues-autorefresh,px2-settings-general-fontradio,px2-chat-sidebar-collapse node scripts/parity/auto-scan.mjs
```

（node 用 v26.4.0：`/Users/wuyongjun/.nvm/versions/node/v26.4.0/bin/node`）
