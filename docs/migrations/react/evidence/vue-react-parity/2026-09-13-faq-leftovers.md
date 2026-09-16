# FAQ 页收尾切片 — Vue→React 一致性证据（2026-09-13）

分支 `codex/react-multiclient`（worktree `.worktrees/react-multiclient`），未 commit，待主协调者复核集成。
切片可写范围：`apps/web/src/faq/**`（FAQPage.tsx / faq.css / FAQPage.test.tsx）。未触碰他人文件。

## 1. Vue 基准与差异表

基准：`frontend/src/views/knowledge/components/FAQEntryManager.vue`（5650 行）；
辅助：`FAQBatchBar.vue`、`FAQTagTooltip.vue`、`frontend/src/api/knowledge-base/index.ts`（getFAQImportProgress）。

| # | 账本遗留 | Vue 行为（出处） | React 切片前 | 本切片处置 |
|---|---|---|---|---|
| 1 | 分页器 vs 无限滚动 | 滚动容器 `@scroll`（:1624，距底 200px 触发）；`pageSize=20`（:1058）；追加加载；`hasMore = entries.length < total`（:1604）；`loadingMore` spinner → `common.loading`；`hasMore===false && entries.length>0` → `common.noMoreData`；内容短于视口时自动补页（:1640-1651）；无 Previous/Next | 50 条/页 + 上一页/下一页分页器 | ✅ 已实现：PAGE_SIZE=20、追加加载、hasMore 三态、.faq-load-more/.faq-no-more、容器滚动 + window 滚动双通道（React 壳层高度不定）、短页自动补页、分页器移除 |
| 5 | 导入预览 | `processFile`（:1900-1921）选文件即解析；对话框内 `.import-preview`：`previewCount` 头部 + 前 5 条（序号+标准问）+ `previewMore`（:644-660）；xlsx/xls 走 Excel 解析，不支持格式 → `unsupportedFormat` warning；解析失败 → `parseFailed` | 无预览 | ✅ 已实现：选文件即用 parseFAQImportText 解析并渲染预览（previewCount/前5条/previewMore）；excel 格式按不支持处理（React 侧无 Excel 解析器，见交接 B3） |
| 5b | 导入进度条 | `upsert` 返回 task_id → 轮询 `GET /api/v1/faq/import/progress/{taskId}`（:2091-2165）；标题行 `.faq-import-strip`：图标旋转、文本 error‖message‖状态文案、72px 进度条 width:%、processed/total；成功 3s 后自动收起，失败保留 | 一次性成功消息（含 task_id），无进度 UI | ◐ 部分实现：标题行 `.faq-import-strip` 视图（running/success/failed/pending 四态样式、进度条、processed/total、is-spinning）+ `faqImportTaskView`/`importProgressText` 纯函数已落地并有测试；轮询本身被 api-client 缺口阻塞 → 交接 A1/A2 |
| 6 | 卡片级开关 | 条目卡 footer `t-switch`（:397-406）：tooltip=已启用/已禁用、`entryStatusLoading` 逐条 loading、`:disabled="!canEdit"`；`handleEntryStatusChange`（:1487-1514）乐观更新 → `updateFAQEntryFieldsBatch by_id`，成功 `statusEnableSuccess/statusDisableSuccess`，失败回滚 + `statusUpdateFailed` | 仅批量栏可改启用态，行内无开关 | ✅ 已实现：行内 `role="switch"` 按钮（aria-checked/title=Vue tooltip 文案、逐条 disabled、viewer 隐藏）、乐观更新 + 失败回滚（`setEntryStatus` 纯函数）、成功/失败文案走共享目录既有键 |
| 2 | 编辑抽屉形态 | t-drawer 520px 右侧，每字段带 label+desc（standardQuestionDesc 等）、maxlength=200、标签 t-select、开关组 | 简化抽屉：无字段 desc、无 maxlength | ⏸ 交接 B1 |
| 3 | 标签管理链接/逐条改标 | 逐条 tag chip dropdown（:362-376）+ KbTagManageDrawer 入口 | 仅批量设标 select | ⏸ 交接 B2 |
| 4 | 信息卡 myRole/chunkCount/hitCount | KBInfoPopover 引用；键在 Vue locale 不存在（最近似 accessInfo.myRole/knowledgeBase.chunkCount） | 简化信息卡 | ⏸ 维持既有账本：键不存在不可凭空造（packages/i18n/src/generated/kbListExtras.ts:2 已注明），需协调者决策 |
| 7 | 检索测试抽屉 | search drawer：query、vector threshold(0.7)、match count(10)、结果列表（:332 导出入口） | 图标按钮目前触发的是搜索提交 | ⏸ 交接 B4（`faq.search` client 方法已存在，缺抽屉 UI 与状态接线） |
| 8 | tooltip | FAQTagTooltip 整文气泡（相似问/反例/答案 chip）、t-tooltip 包裹开关 | 仅原生 title | ◐ 开关已用 title=已启用/已禁用（原生 tooltip 近似）；chip 气泡 ⏸ 交接 B5 |

## 2. 实现明细（TDD 先红后绿）

- 红：新增 10 个失败测试（FAQPage.test.tsx），覆盖三优先项 + 纯函数契约。
- 绿：FAQPage.tsx 新增导出 `faqHasMore` / `setEntryStatus` / `importFormatFromName` / `importProgressText` / `faqImportTaskView`（FAQImportTaskView）；容器改为追加式加载（`load(append)`、`loadingMoreRef` 防抖）、`toggleEntryStatus` 乐观更新/回滚、`handleImportFile` 预览解析；视图移除分页器，新增滚动/加载 UI、行内开关、导入预览块、头部进度条。
- 容器对外契约不变：`main.tsx` 的 `<FAQPage client knowledgeBaseId />` 未动。
- faq.css：新增 .faq-load-more/.faq-no-more、.faq-import-strip 族（含 running/success/failed 修饰与旋转动画）、.import-preview 族、.faq-status-switch 族；.faq-scroll-container 补 Vue 的 `overflow-y: auto`。
- 环境注意：`typecheck:web` 需在仓库根以 `pnpm -w run typecheck:web` 运行（apps/web 目录内无该 script）。

## 3. i18n

- 直接可用（共享目录 byte-exact 已存在，验证过 zh 值）：`knowledgeEditor.faqImport.previewCount`（共解析 {count} 条记录）、`previewMore`（还有 {count} 条未展示）、`knowledgeEditor.faq.statusEnabled/Disabled/EnableSuccess/DisableSuccess/UpdateFailed`、`common.noMoreData`（已加载全部内容）、`common.loading`（加载中...）、`knowledgeEditor.faqImport.parseFailed/unsupportedFormat`。
- 目录缺失（Vue locale 有、@weknora/i18n 未回填）：`faqManager.import.{importing,importDone,importFailed,waiting}`。已在切片内加 zh-only byte-exact 回填映射（FAQPage.tsx `zhFallbackMessages`，值取自 frontend/src/i18n/locales/zh-CN.ts:845-848），非 zh locale 在目录回填前也会显示 zh 值（catalog 回退链 locale→en-US→key 会露出裸键名，取 zh 值更可用）。
- 标注 zh-only 的新 affordance：无（进度条文案全部有 Vue 键；预览/开关/加载文案均有共享目录键）。
- 回填请求（交接 A3）：把 `faqManager.import` 整块回填进 packages/i18n 生成目录（en-US/zh-CN/ja-JP/ko-KR/ru-RU 五语），回填后可删除切片内 `zhFallbackMessages`。

## 4. 测试与类型检查

- `cd apps/web && npx tsx --test src/faq/*.test.tsx src/faq/*.test.ts` → **21/21 通过**（切片前 10/10，新增 11）。
- 关联面：`npx tsx --test src/*.test.ts src/faq/*.test.* src/knowledge/*.test.ts` → **39/39 通过**（含 routes/pagination 回归）。
- `pnpm -w run typecheck:web` → src/faq/** **0 错误**；当前全仓 7 个错误全部位于他人正在编辑的 src/documents/**、src/integrations/**、packages/views/src/integrations/**（切片期间由其所有者持续变动，与 FAQ 无关，复核时以其为准）。

## 5. 精确交接清单（需协调者/其他切片处理）

**A. 阻塞项（跨包，超出本切片可写范围）**
- **A1** packages/api-client：`packages/api-client/src/knowledge/faq.ts` 增加进度方法，Vue 契约：`getFAQImportProgress(taskId) → GET /api/v1/faq/import/progress/{taskId}`（frontend/src/api/knowledge-base/index.ts:602-604），响应字段 status/progress/total/processed/error/message，status 映射 processing→running、completed→success。
- **A2** apps/web/src/faq/FAQPage.tsx 容器接线：拿到 A1 后在 `confirmImport` 成功分支设置 `importTask`（用已导出的 `faqImportTaskView`）并按 Vue :2091-2165 轮询（processed 增长时 `load(false)` 刷新；success 3s 后自动收起；failed 保留；404 清 taskId 停轮询）。视图已就绪，无需再改。
- **A3** packages/i18n 目录回填 `faqManager.import.*`（见第 3 节），回填后删 `zhFallbackMessages`。
- **A4** Excel 导入解析：Vue parseExcelFile（xlsx/xls）在 React 侧无实现；如需对齐需引入 xlsx 解析依赖或服务端转换，属共享依赖决策。

**B. 后续 UI 切片（均在 apps/web/src/faq/** 内可做，工作量另估）**
- **B1** 编辑抽屉形态：t-drawer 520px、逐字段 desc 键（standardQuestionDesc/similarQuestionsDesc/negativeQuestionsDesc/answersDesc 已在共享目录）、standard_question maxlength=200。
- **B2** 逐条改标 dropdown（tag chip 点击换标，`faq.updateTags` 已支持单条）+ 标签管理抽屉入口（KbTagManageDrawer 对应面）。
- **B3** 条目卡布局对齐：Vue 为 faq-card（相似问/反例/答案三段折叠、卡片多选、more 菜单），React 仍为列表行 — 大切片，建议独立登记。
- **B4** 检索测试抽屉：`client.knowledge.faq.search` 已存在，缺抽屉 UI/表单/结果渲染（query/vector threshold 默认 0.7/match count 默认 10）。
- **B5** FAQTagTooltip 气泡（chip 整文 hover 显示，含 answer/similar/negative 三种配色）。
- **B6** 导入结果持久化条（importResult 摘要 + 下载失败明细 downloadReasons + display_status 开合），依赖 A1 类似的 last-result 接口（`GET/PUT /api/v1/knowledge-bases/{kbId}/faq/import/last-result*`）。
- **B7** 滚动加载的自动化测试需 DOM 环境（现套件为 SSR 断言 + 纯函数契约；window 滚动/自动补页路径建议在引入 jsdom/happy-dom 后补挂载测试）。
