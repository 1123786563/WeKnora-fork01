# R009 · 知识库列表整页 anatomy 切片（Vue→React 一致性）

- 日期：2026-09-13（实施于 2026-09-14 凌晨）
- 实施者：R009 kb-list anatomy 子代理
- Vue 权威源码：`frontend/src/views/knowledge/KnowledgeBaseList.vue`（3109 行）+ `frontend/src/components/ListSpaceSidebar.vue`（550 行）
- React 侧：`apps/web/src/App.tsx`（KnowledgeBasesPage）+ 新增 `apps/web/src/knowledge-list.css`
- 分支：`codex/react-multiclient`（worktree `.worktrees/react-multiclient`）
- 集成记录：主协调者已将本切片合入 **`80ab0a0a` fix(web): align knowledge base list with Vue anatomy**，并在 **`e6f70f2e`** 中按更严格的 Vue 对齐收紧卡片菜单（移除「编辑/分享」两项、菜单测试改为精确四项断言）并刷新 after 截图
- 登记分歧证据（修复前基线）：`docs/migrations/react/evidence/vue-react-parity/screenshots/state-coverage-20260913/{vue,react}-kblist-loaded.png`、`vue-kblist-error.png`

## 0. 结果总览

| 分歧 | 状态 | 关键产物 |
| --- | --- | --- |
| (a) 卡片栅格 | ✅ 已闭环 | 响应式网格 + 136px 紧凑卡 + hover 显现星标/更多菜单 |
| (b) 筛选 anatomy | ✅ 已闭环 | 左侧竖直图标轨（全部/收藏/最近/本空间）+ 分组标题行；删除横向 chips/select/文本框/计数 |
| (c) 警告横幅 | ✅ 已闭环 | 琥珀色全宽横幅（ⓘ + 文案） |
| (d) 错误/空态语义 | ✅ 已闭环 | 列表失败回退空态（插画 + CTA），UI 不外露 JSON |
| 另：标题行小图标按钮 | ✅ 已核实并对齐 | 即「新建知识库」入口（folder-add，28×28，仅 contributor） |

测试：`test:web` 全量 **665/665 绿**（其中本切片新增 `kb-list-anatomy.test.tsx` 7 项；基线 586 → 665 的增量含其他 agent 在途用例，无 platform/command-palette 红）；`typecheck:web` 绿；`knowledge-bases`+`documents` 定向 126/126 绿。

## 1. 差异表（Vue 权威 ↔ 修复前 React ↔ 修复后 React）

### (a) 卡片栅格

| 维度 | Vue（行号） | React 修复前 | React 修复后 |
| --- | --- | --- | --- |
| 布局 | `.kb-card-wrap` grid，gap 12px（2196-2201）；响应式 1/2/3/4/5/6 列，断点 ≥900/1250/1600/1900/2200（2811-2840） | `.wk-kb-grid` 全宽单列堆叠行 | 同 Vue 的 grid + 全部 6 档媒体查询 |
| 卡片尺寸 | 136px 高、padding 12px 14px、radius 8、1px 描边（2285-2299） | 自适应高度行卡 | 136px 紧凑卡，同 padding/radius |
| 类型渐变+角部装饰 | document 绿 tint / faq 蓝 tint 渐变（2326-2358）+ ::after 60px 角部四分之一圆渐变（2336-2347、2360-2372） | 无 | 同 Vue 双类型渐变 + ::after 角部装饰 |
| hover | 边框品牌绿 + 绿/蓝 shadow（2317-2320、2330-2333、2354-2358） | 无 hover 语义 | 同 Vue（FAQ 卡 hover 用蓝边框，与 Vue 的绿边框有微小色差，见遗留 5） |
| 收藏星标 | 右上角 absolute，opacity 0→hover 1，已收藏常显（2375-2409） | 常显 ☆ 按钮 | 同 Vue hover 显现 + is-favorited 常显 |
| 「更多」菜单 | header 右侧 `.more-wrap` 三点，opacity 0→hover 0.6（2523-2553），点击弹 popup-menu：置顶/创建副本/设置/删除（211-239、2178-2236 menu 项） | 常显整排操作按钮（置顶/创建副本/编辑/设置/分享/删除） | hover 显现三点按钮 + popup 菜单四项（Vue 权威集合），卡片外浮层不再被 136px 卡片裁切 |
| 底部计数器 | feature-badge：文档=绿 folder+count，FAQ=蓝 chat+count（250-263、2579-2586 底部分隔线） | 文本徽章「问答 · 0」 | 图标+count 徽章（绿/蓝 tint）+ hairline 分隔 |
| 计数器口径 | FAQ 用 chunk_count、文档用 knowledge_count（259-260） | 只用 knowledge_count | 同 Vue 口径 |
| 底部右侧 | 共享卡显示来源空间 chip（362-370）；本空间卡在有分组标题时不显示徽章（card-list-badge.ts shouldShowResourceOriginBadge） | 显示 sharedEditable/sharedReadonly 文本徽章 | 共享卡显示 org chip；本空间卡不显示 |
| 分组标题行 | 网格内 sticky 整行：icon + 标签 + 计数 + chevron，可折叠（96-185、2203-2283） | 网格外 chips 式 toggle | 网格内 sticky 分组标题行（pinned/mine/tenant/shared 四类图标）+ 折叠 |

### (b) 筛选 anatomy

| 维度 | Vue（行号） | React 修复前 | React 修复后 |
| --- | --- | --- | --- |
| 作用域切换 | 内容区左侧竖直图标轨（ListSpaceSidebar.vue 2-74：全部 layers/收藏 star/最近 history/本空间 system-sum，46px 项、56px 轨，active 品牌绿，CSS 295-422） | 横向 chips（`.wk-kb-scope`：全部/我创建的/收藏/暂无最近访问） | 同 Vue 图标轨（标签 11px、active 绿底） |
| 计数 | tooltip 形式「名称 (N)」（ListSpaceSidebar.vue 237-239）；本空间=owned 数（KnowledgeBaseList.vue 3-4） | 无 | tooltip 计数；本空间=owned、全部=合并去重、收藏/最近=pin 集合 |
| 默认作用域 | contributor→mine、viewer→all（KnowledgeBaseList.vue 816-830） | 恒 all | URL 无 `?scope` 时随角色（contributor→本空间），用户点击后以用户为准 |
| 我创建的 N | 分组标题行「我创建的 (N)」+ 折叠 chevron（114-132） | `.wk-kb-sections` chips「我创建的 · 3 −」 | 网格内 sticky 分组标题行（同 Vue，含图标/计数/chevron/折叠） |
| 创作者筛选 | 已从 chrome 移除（22-27 注释；URL state 保留） | 原生 select（全部/我的/他人的） | 移除 UI；`?creator` 服务端参数与 domain 过滤能力保留 |
| 搜索框/计数 | 无（搜索在命令面板；knowledge-search 路由重定向） | 文本框 + 「3 项」debug 计数 | 移除；`?q=` 深链过滤保留 |

### (c) 警告横幅

| 维度 | Vue（行号） | React 修复前 | React 修复后 |
| --- | --- | --- | --- |
| 结构 | `div.warning-banner`：info-circle 16px + 文案（30-34） | `<Status tone="warning">` 无样式文本行 | `.kb-list-warning`：ⓘ SVG + 文案，role=status |
| 样式 | 琥珀底/琥珀边/琥珀字、12px 16px padding、radius 6、14px 字号、margin-bottom 20px（2096-2113） | 无样式 | 同 Vue 数值（#fdf3e7/#f7d8b0/#cf6b1d） |
| 触发条件 | `hasUninitializedKbs`：owned 列表任一未初始化（1565-1568） | merged 卡片集合判断 | 改为 owned 列表判断（同 Vue 口径） |

### (d) 错误/空态语义

| 维度 | Vue（行号） | React 修复前 | React 修复后 |
| --- | --- | --- | --- |
| 列表接口失败 | `fetchList` 无 error UI，kbs 置空 → 呈现空态（1227-1242） | 渲染原始 JSON（`{"message":...}`）+ 重试按钮（live 证据 leaksJson=true） | `console.error` 上报后回退空态；UI 无 JSON（live 复核 leaksJson=false）；重试按钮移除（Vue 无） |
| 全部/本空间空态 | upload.svg 插画 162px + 暂无知识库 + 描述 + 新建 CTA（contributor，633-643；CSS 2774-2809） | 纯文字 h2/p + CTA | 移植 upload.svg（empty-kb-svg.ts）+ 文案 + 渐变绿 CTA |
| 收藏空态 | star 图标 48px + 暂无收藏/描述，无 CTA（645-652） | 同「暂无知识库」空态（错误引导） | star 图标 + favoritesTitle/Description，无 CTA |
| 最近空态 | history 图标 + 暂无最近访问/描述，无 CTA（654-659） | 同上错误 | history 图标 + recentsTitle/Description，无 CTA |
| 分页 | 无（全量渲染） | 分页条（上一页/第 N 页/下一页） | 移除，全量渲染（pageSize MAX_SAFE_INTEGER，滚动即 Vue 语义） |

### 另：标题行小图标按钮

- Vue（7-20、1889-1915）：`知识库` h2 旁 28×28 图标按钮，folder-add 图标（品牌绿），tooltip=`knowledgeList.create`（新建知识库），仅 contributor 可见（`authStore.hasRole('contributor')`），点击 `handleCreateKnowledgeBase`（1695-1699）打开创建向导。
- 结论：**功能=新建知识库**（非分享）。
- React 修复前：标题右侧大按钮「+ 新建知识库」；修复后：h1 旁 28×28 图标按钮（data-guide="kb-list-create" 保留，创建向导/引导埋点不变），contributor 门控同 Vue。

## 2. 功能保留核对（任务 2）

| 既有功能 | 状态 |
| --- | --- |
| 上传进度遮罩/面板 | ✅ 事件管线与 UI 原样保留（`wk-upload-progress-*` 样式仍在 styles.css，未动他人文件） |
| pin/duplicate | ✅ 移入更多菜单；成功/失败反馈、列表刷新保留 |
| `?scope` 过滤 | ✅ all/mine/favorites/recents 深链保留（readScopeFromUrl 扩展了 favorites/recents 的读取） |
| 分享抽屉 | ⚠️ 组件与测试保留且照旧挂载；菜单入口按协调者并发编辑移除——与 Vue 逐行对齐（Vue 列表页 handleShare 同样无入口、对话框同样空挂载，1463/707），见遗留 1 |
| 点击语义（已初始化→文档页，未初始化→设置页，5ce821fd） | ✅ openCard 原样保留；卡片整体可点（同 Vue 卡片级 @click） |
| 收藏/最近持久化 | ✅ wk-kb-favorites / wk-kb-recents localStorage 读写保留（有测试） |
| 创建/编辑对话框、删除确认、delete guard、上下文引导（kbList/kbCreate/kbDetail） | ✅ 保留 |
| highlight 滚动 + 闪烁（`?highlightKbId`） | ✅ 保留（移除按页跳转——全量渲染后无需翻页） |

## 3. live 截图证据（docs/migrations/react/evidence/vue-react-parity/screenshots/kb-list-anatomy/）

工具：`.parity-tools/kb-list-anatomy-shots.cjs`（before/after 两相；双端 1440×900，zh-CN，引导完成键种子与 state-coverage.cjs 一致；error 场景 mock 500 `{"message":"mock failure"}`，无数据变更）。

| 分歧 | 修复前 | 修复后（React） | 对照（Vue） |
| --- | --- | --- | --- |
| (a)(b)(c) 加载态 | before-react-kblist-loaded.png | after-react-kblist-loaded.png | after-vue-kblist-loaded.png（=vue-kblist-loaded.png） |
| (a) 卡片 hover（星标/三点显现） | before-react-kblist-card-hover.png | after-react-kblist-card-hover.png | after-vue-kblist-card-hover.png |
| (a) 更多菜单 | （修复前无此控件） | after-react-kblist-more-menu.png | after-vue-kblist-more-menu.png |
| (b) 图标轨切换收藏 | （修复前无图标轨） | after-react-kblist-scope-favorites.png | —（Vue 同轨控件） |
| (d) 500 回退 | before-react-kblist-error-fallback.png（泄漏 JSON） | after-react-kblist-error-fallback.png（空态+CTA，leaksJson=false） | after-vue-kblist-error-fallback.png（leaksJson=false） |

脚本输出的泄漏复核：before react leaksJson=**true** → after react leaksJson=**false**（vue 两相均 false）。

## 4. 测试（TDD）

- 新增 `apps/web/src/knowledge-bases/kb-list-anatomy.test.tsx`（7 用例，先红后绿）：图标轨结构/默认作用域、无旧 toolbar、紧凑卡+类型类+星标/三点、分组标题折叠、琥珀横幅、失败回退空态且无 JSON 泄漏、菜单=Vue 四项、收藏持久化+收藏空态口径。
- 运行记录：`npx tsx --test src/knowledge-bases/… src/documents/…` → 126/126；`pnpm run test:web` → **665/665**（0 失败；基线 586，增量含其他切片用例，无 N003 红）；`pnpm run typecheck:web` → 绿。

## 5. 与其他 agent 的文件边界

- 独占改动：`apps/web/src/App.tsx`、`apps/web/src/knowledge-list.css`（新）、`apps/web/src/knowledge-bases/{kb-list-icons.tsx, empty-kb-svg.ts, kb-list-anatomy.test.tsx}`（新）、`.parity-tools/kb-list-anatomy-shots.cjs`（新）。
- 未触碰：chat/**、platform/GlobalCommandPalette*、settings/** CSS、styles.css（`wk-upload-progress-*` 等既有样式原样引用）。
- ⚠️ 并发编辑说明：实施期间检测到 App.tsx / kb-list-anatomy.test.tsx 被协调者侧并发收紧（`e6f70f2e`：卡片菜单移除「编辑/分享」两项、菜单测试改为 Vue 四项精确断言）。最终以协调者版本为准，本报告按最终代码撰写。注意由此产生的死代码：`openEdit` 与 `setSharingKb` 目前无调用方（share 抽屉组件仍挂载但不可达）——这与 Vue 列表页完全同构（`handleShare` 同样无入口、对话框空挂载，KnowledgeBaseList.vue:1463/707），是否另行处理请协调者定夺；如需恢复入口，在更多菜单加回两项即可（测试 `the more menu exposes exactly the Vue card actions` 现断言精确集合，需同步放开）。

## 6. 遗留清单

1. **分享抽屉/编辑对话框入口**：当前与 Vue 同构地“空挂载”（无入口）。若产品上需要保留 React 既有可达性，需协调者决定入口位置（卡片菜单 or 详情页）。
2. **图标轨展开面板**：Vue ListSpaceSidebar 支持拖拽加宽至 208px 全导航面板（localStorage 持久化 key `sidebar-collapsed-list-expanded`）；React 仅实现 collapsed icon strip（截图所示形态），拖拽展开未移植。
3. **共享空间（org）轨条目**：需 organizations 数据（React kb 页未拉取 org 列表）；parity 环境共享计数为 0 不展示，影响面小。配套的 `?scope=<orgId>` per-space 视图（Vue listOrganizationSharedKnowledgeBases 分支 1244-1269）未实现。
4. **FAQ 卡 hover 色差**：Vue hover 边框统一品牌绿（仅 shadow 分色）；React FAQ 卡 hover 用蓝边框。微小视觉差。
5. **node:test 全文件挂起 quirk**：该测试文件扩到 8 个用例时进程在文件尾挂起（单测/7 用例均正常，组件行为经独立探针验证无死循环）；已通过合并断言至 (d) 规避。如后续加用例请留意。
6. **键盘可达性**：与 Vue 持平（卡片为可点 article，非 tab 序；Vue 亦为 div+click）。未回归也未超越。
7. **上传进度样式归属**：`wk-upload-progress-*` 仍引用 styles.css（他人属主文件）；如需迁移到 knowledge-list.css 由协调者统筹。
