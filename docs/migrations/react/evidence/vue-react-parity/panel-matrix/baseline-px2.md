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
| px2-settings-models-tab | 0.005 | 模型类型过滤 tab 全部→对话（.model-type-tabs，ModelSettings.vue:38）。**终值 0.005（pp2/settings 批次收敛后复核恒定）**= #16 play-circle 图标 28px（静态存量豁免）+ #29 指示条边缘 AA 14px（新判例，决定性实验同值 left 替换后 0px） |
| px2-settings-sandbox-tab | 0.001 | 沙箱类型过滤 tab（SandboxSettings.vue:59）。**终值 0.001** = #29 指示条边缘 AA 12px（y244-246 x424-551，单灰阶级） |
| px2-settings-mymemory-tab | 0 | 记忆状态过滤 tab 生效中→待确认（MemorySettings.vue:154；注意挂 mymemory 分区）。复核 0（第 2 tab offset 落整数相位，#29 不显形） |
| px2-settings-systemglobal-tab | 0.001 | 系统设置分区 tab 账户与访问→空间默认值（SystemSettings.vue:69；settings-system 分区是 SystemInfo 无 tab）。**终值 0.001** = #29 指示条边缘 AA 13px（y176-178 x456-571） |
| px2-settings-general-fontradio | 25.519→**5.066** | 字号分段控件 正常→大。**已修（pp2/settings 批次）**，三项根因逐个收敛：①字号应用机制异构——React 新增 `apps/web/src/font.ts`（initFont boot + applyFontSizeZoom，Vue useFont.ts:216-238 平移：`<html>` CSS zoom 全站缩放；原 `--wk-font-scale` 变量全仓无消费者、视觉不缩放），GeneralPreferencesPanel.applyFontCssVariables 接线 + main.tsx boot 调用；②toast 呈现异构——本面板 5 处 pushSettingsToast（右上角自研）换 tdesign-react MessagePlugin.success（Vue GeneralSettings.vue:229-274 同构，da187f072 判例族此前未覆盖此处）；③扫描瞬态——双端点击均弹 3000ms t-message 而扫描器 vue 截图恒晚一个 action+steady 周期（vue toast 必过期、react 必在场），扫描项加 postSettle 3500 让双端 toast 都过期（px2-kb-settings-nav 先例）；④连带修复——`.wk-shell-1` height:100vh 在 zoom 态按未缩放视口求值（720→810）把 .menu_bottom 推出视口，换 Vue .main 同款 height:100% + styles.css 补 html/body/#root 高度链（Vue App.vue:291-297 平移；静态页复扫零回归）。**残值 5.066 = settings-general 静态 5.439（SP14 React-only 套餐卡）的缩放态传导**，决定性实验：React 藏卡+双端回滚顶后点击态截图 diff **0.000%**（DOM 探针：卡仅 React 存在 h=104.75，其余 .settings-group 行几何逐字段一致 855×96.44/155.09/133.05，双端 zoom 均 1.125） |

### system 域（1 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-settings-runtimequeues-autorefresh | 0 | 自动刷新开关（客户端 5s 轮询，无服务端写；clickAria 先行因 React 该页 2 个 switch 须唯一定位） |

### chat 域（1 项）

| id | 基线% | 备注 |
|---|---|---|
| px2-chat-sidebar-collapse | 16.06 | 会话侧栏折叠（.sidebar-toggle：Vue menu.vue:22 / React PlatformShell.tsx:1087 同名类同 `sidebar_collapsed` localStorage 键）；diff 集中顶带（3×3：上中 43.9%/上右 38.6%）——折叠态 rail/头部区渲染差异待工单归因。**【2026-09-26 pp2/settings 批次注】**本项排在 fontradio 之后，基线 16.06 含「Vue 已 zoom 1.125 / React 未 zoom」的错配分量；fontradio 修复（React 补 `<html>` zoom 机制）后复扫 **9.094**（run 05-45-02，双端同 zoom 的真值，仍 >1% 留工单池，归 chat 域） |

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
