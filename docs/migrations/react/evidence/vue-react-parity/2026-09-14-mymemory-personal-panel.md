# 2026-09-14 R025 我的记忆（MemorySettings.vue → PersonalMemorySettingsPanel）对齐切片

## 范围

Vue frontend/src/views/settings/MemorySettings.vue（Settings.vue mymemory 分区，1327 行）与 React apps/web/src/settings/PersonalMemorySettingsPanel.tsx 的整面重建，以及两条结构性差异的修复：

1. **分区挂载错位**：React 原先把个人记忆开关面板挂在 memory（空间级）分区下、把一个 CRUD stub（PersonalMemoryPanel）挂在 mymemory 分区。Vue 的映射是 memory → MemoryWorkspaceSettings.vue、mymemory → MemorySettings.vue。已按 Vue 语义纠正（SettingsPage loaders + render branches）。
2. **API 契约缺口**：GET /memory/items|topics|documents 的 envelope 带 total，Vue 全部按它分页/计数（frontend/src/api/memory.ts）。api-client 原来 dataArray 丢弃 total → 新增 dataListPage/MemoryListPage { rows, total }，三个 list 全部返回分页页（合同测试扩展并断言 rows/total 与编码 id）。
3. **i18n 回填**：memorySettings.* 在 Vue locale 有 111 键，React 移植版只有 76 键；缺失 35 键 ×5 locale（kinds.*、kindHints.*、origins.*、statusActive/statusPending/statusSuperseded/statusArchived、consolidate skip 文案、usage.rows.*）已按 Vue locale 字节级回填（175 键）。键集一致性测试通过。

## 面板解剖（对照 Vue 模板逐块）

- 分区头：h2 memorySettings.title + hover usage popup（7 行 usage.rows.*）+ description；i18n 全部就位。
- 空间未开启提示：settings && !workspace_enabled 渲染 .wk-mem-notice（warning-1 底色）。
- 开关行：user_enabled 初始值取自 settings 响应（修复了原实现读 enabled 字段的错误——后端 struct 是 user_enabled，internal/types/memory.go:984）；canWrite=effective===true；列表在关闭后仍可读；失败回滚开关位（Vue handleEnabledChange）。
- 工具栏：添加（kind select 5 类 + content textarea 300 上限 + kindHints，写门控 canWrite）、导出（JSON blob weknora-memories.json）、整理（popconfirm + consolidating 防重 + merged/demoted/expired 成功、model_unavailable warning、skip 分支 info 文案）、清空（danger popconfirm + removed 计数 toast + 三存储全空时禁用）。
- 状态页签：active/pending/tracking/documents/superseded/archived，Vue 同款图标映射与计数标签（statusXxx(count)），切页签/翻页均按 Vue 语义 refetch（offset=(page-1)*20）。
- 列表：items（pending Confirm/Reject——Reject 不做 canWrite 门控，与 Vue 一致；active 行内编辑 Ctrl+Enter 保存；非 pending 删除 popconfirm；retired 划线置灰）、topics（进度条 hits/threshold + trackingReady/Progress + promote 后跳回 active + dismiss popconfirm）、documents（打开文档跳 /knowledgeBase/:kbId?knowledge_id=… 对齐 Vue router push、无 kb id 时禁用 + openDocumentUnavailable、停止跟踪 popconfirm）。
- 空态/提示文案按页签区分（emptyTitle/emptyDescription/statusHint）。

## 设计令牌（第三层）

personal-memory.css 以 .wk-memory-settings 作用域落位 Vue theme.css light 值：text rgba(0,0,0,.9/.6/.4)、stroke #e7e7e7、brand #07c05f（hover #08dd6e、disabled #8ce0af）、error #e34d59、warning-1 #fef3e6、popup 320/380px + radius 12 + Vue 同款三层阴影；页签 active 2px 品牌下划线；meta 分隔符「·」。

**共享组件修复（第二层）**：@weknora/ui Switch 原先依赖页面级 Tailwind 工具类（h-5/w-9/translate-x-[18px] 等），这些类只在该包内使用、Tailwind v4 自动源检测不扫描 pnpm node_modules 符号链，导致开关在真实浏览器里塌缩成细条（live 截图证实，MemoryWorkspacePanel 同样受害）。已按包内既有 wk-* 纯 CSS 惯例改为 .wk-switch/.wk-switch-knob（40×20 圆角胶囊、16px 滑块、checked 品牌绿、disabled 保留 on-state 色，对齐 Vue t-switch disabled 表现）。组件 API（checked/onCheckedChange/disabled/data-state）不变。

**重复标题修复**：SettingsPage 的 wrapper heading（wk-settings-panel-heading）对 memory/mymemory 两个自身渲染 h2 的分区停用（沿用 general/models/members 既有豁免模式）。

## 验证

| 层 | 结果 |
|---|---|
| api-client settings 合同测试（含新 action surface + 分页 total 断言） | 7/7 |
| PersonalMemorySettingsPanel.test.tsx（挂载计数/页签切换/pending 确认拒绝/tracking 进度与 promote/documents 深链 helper + 停止跟踪/清空守卫与移除计数/分页 offset/导出下载/空间关闭门控/开关失败回滚/整理 skip 文案） | 11/11 |
| test:shared / test:web | 444/444 · 849/849 |
| typecheck:web / typecheck:shared | 0 错误 · 0 错误 |
| build:web | ✓ |

### Live 同条件双端对照（真实后端 :8080，parity-test@local.dev，1440×900 zh-CN）

脚本 .parity-tools/memory-parity.cjs（只读：仅页签 GET 切换，无任何写请求；apiErrors/pageErrors 双端均为 0）：

- M1 ?section=mymemory：标题/描述/ⓘ、六页签标签+计数（生效中(0)/待确认(0)/观察中(0)/常用资料(0)/已被更新(0)/已归档(0)）、工具栏四动作、共 0 条、空间未开启 notice 文案逐字一致、开关位（user_enabled=true，workspace off）、空态「还没有记忆」。锚点断言双端一致。
- M2/M3 观察中/常用资料页签：React 切换后空态逐字对齐 Vue 键（没有正在观察的主题/还没有常用资料）；Vue 侧 TDesign 页签 force-click 未触发其内部切换（脚本限制），其页签空态文案以 i18n 键逐字核对。
- M4 ?section=memory：双端均只剩空间级开关（workspace off → switch false），React 不再出现个人记忆 dl/开关块（personalDlLeak=false），wrapper 重复标题已消失。
- 截图：screenshots/memory-20260914/m1..m4-{vue,react}.png。

### 修复前差异（本次消除）

1. React memory 分区混入个人开关面板、mymemory 为无权限语义 stub → 结构性错位已纠正。
2. 列表 total 被丢弃 → 分页/计数不可能实现 → 契约已修复。
3. 35 组文案缺失（页签状态、kind/origin/hint、整理 skip、usage 弹层）→ 已回填。
4. 重复的分区级 h2 + 外层 Card 边框（Vue 为裸排版）→ 已消除/改为裸布局。
5. Switch 塌缩成细条（共享组件层 Tailwind 依赖缺陷）→ 改包内纯 CSS，双端截图确认恢复 Vue 同款 40×20 胶囊。

## 仍开放（不宣称完成）

- 真实数据的编辑/确认/整理/导出端到端（当前 parity 账号记忆为空且空间记忆关闭，列表全为空态；写路径已由 11 个单测覆盖但未在真实后端写入）。
- 浏览器 computed-style 逐值对照、暗色主题令牌、Wails/native 平台证据。
- memory 分区 MemoryWorkspacePanel 的外层 Card（Vue 为裸排版）为相邻切片遗留差异，本轮未动。
