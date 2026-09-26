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

| id | 基线% | 终值% | 备注 |
|---|---|---|---|
| px2-kb-wiki-tab | 0 | 0 | 面包屑 tab 文档→Wiki（span.breadcrumb-tab 双端同构，KnowledgeBase.vue:2424-2438） |
| px2-kb-graph-tab | 0 | 0 | 面包屑 tab 文档→图谱 |
| px2-kb-wiki-reader-tab | 0 | 0 | wiki reader 头部 tab 知识→摘要（.wiki-tab：Vue div / React button 标签异构同名类） |
| px2-kb-wiki-tree-expand | 0 | 0 | 目录树行点击展开（.wiki-directory-item；Vue WikiBrowser.vue:292 / React WikiPage.tsx:1249） |
| px2-kb-settings-nav | 7.478 | **0.003（#19 豁免）** | KB 设置抽屉 nav→分块设置；**已收敛**（pp2/kb）：分块段换 KBChunkingSettings.vue 同构（chunkingSection.tsx 重写为 tdesign 控件 + chunking.td.css 平移 scoped 块）；终值 28px 全落弹窗四角圆弧（x140-147/1129-1139 × y54-61/655-665，灰阶 ±9~30），与台账 #19 取证盒（x140/1140，y54/666）同签名——#19 引擎栅格伪影豁免适用，内容区 0 差分（run 2026-09-26T06-22-01） |
| px2-kb-settings-chunkswitch | 9.409 | **0.003（#19 豁免）** | 父子分块开关；**已收敛**（pp2/kb）：React 换 tdesign Switch（补 aria-label 维持 clickAria 命中链）；父/子块行、滚动锚定（Vue .section margin-bottom 32px 复刻进 scrollHeight）、分隔符空 input 折行（.kb-sep-relaid touched 态复刻 Vue config 回写后态）逐项对齐；终值与 nav 同为 #19 四角弧线残差（run 2026-09-26T06-22-01） |

#### px2-kb-settings-nav / chunkswitch 收敛记录（2026-09-26，pp2/kb）

- **根因**：React 分块段是原生控件实现（select/range/checkbox/number + inline-style 行布局），Vue 是 KBChunkingSettings.vue 的 tdesign 组件 + scoped 类族布局——中部内容区整体异构（基线 3×3 中 15.9%/右下 14.2% 即此）。
- **修复**（apps/web/src/knowledge-settings/chunkingSection.tsx 重写 + chunking.td.css 新增 + KnowledgeSettingsPage.tsx/documents-list.css 局部）：
  1. 布局类族平移：.kb-chunking-settings/.settings-group/.setting-row/.setting-info/.setting-control/.strategy-control/.slider-container/.value-display/.advanced-toggle/.section-header（sticky 带负 margin 补偿）——KnowledgeSettingsPage chunking 分区不再渲染页级 h3+p（Vue 由组件自带 header）。
  2. 控件换 tdesign：Select（策略 280px/分隔符 multiple+creatable+filterable）、Slider（marks 数组）、Switch（补 aria-label=父子分块，clickAria 命中链恢复）、InputNumber（token limit）。
  3. debug 触发器换 tdesign Button（icon 槽，台账 #28 判例）+ play-circle/chevron-right 用 Vue 组件内联 d 逐属性复刻（TIcon sprite 版 d 几何不同，环描边 AA 差 28px 实证）。
  4. **滚动锚定**：Vue `.section { margin-bottom: 32px }`（KnowledgeBaseEditorModal.vue:1796）未复刻时开关展开后双端 scrollTop 229 vs 261 整体错位 32px——React chunking 分区补 marginBottom 32px 后 scrollHeight 一致、锚定行程一致。
  5. **分隔符空 input 折行**：vue-next tag-input 空 input 保留 inline width:auto（intrinsic ≈154px），config 回写（watch 置换数组）触发 tags 重排后 input 折到独立行、盒高 122→145；tdesign-react TagInput 由库 JS 恒写 width:0px 不折行——React 以 touched 态（任一控件变更后）加 .kb-sep-relaid 放开 input 宽度复刻（空 input 无视觉面，仅盒高对齐）。
  6. **关闭钮层级**：宿主 .wk-kb-settings-dialog .wk-dialog-header z-index 5→10 对齐 Vue .close-btn z-index:10（KnowledgeBaseEditorModal.vue:1670）——与分块段 sticky header（z-index 5，KBChunkingSettings.vue:412）同层级时按 DOM 序被盖住（y74-105 全红实证）。
- **豁免引用**：台账 #19（Dialog 弹窗圆角弧线 AA 阶梯错位）——双项终值 0.003%（28px）全落四角圆弧带，与已豁免的 ix-kb-settings（同弹窗壳 0.003%≈31px）同签名；带外（内容区/关闭钮/控件区）0 差分。

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
| px2-chat-sidebar-collapse | 16.06 | 会话侧栏折叠（.sidebar-toggle：Vue menu.vue:22 / React PlatformShell.tsx:1087 同名类同 `sidebar_collapsed` localStorage 键）；diff 集中顶带（3×3：上中 43.9%/上右 38.6%）——折叠态 rail/头部区渲染差异待工单归因 |

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
