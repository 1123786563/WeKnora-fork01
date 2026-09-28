# OCR round-4-resume2 增量 findings 独立复核（HEAD 76df0cee0）

复核基准：/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 @ 76df0cee0，仅只读核验（cat/grep/sed）。禁止代码编辑与 push。

范围：ocr-round-4-resume2.md（399 findings）相对前一份 ocr-round-4-resume.md（386 findings）的**新增差量**条目。
去重口径（比 resume 轮更严格，双重比对）：
1. finding 标识行（`─── path:start-end ───`）多重集合 diff：resume2 新增 14 个标识、移除 1 个标识（packages/views/src/craft 10 文件聚合条目 0-0），净 +13 与 386→399 一致；
2. 共同 385 个标识的**描述段**（body 至 diff 块前）逐条 md5 比对：全部一致，无「同位置描述重写」的隐性新增。

已完成判定基线（不重复判定）：round4-highrisk-analysis.md（主报告 23 条）+ round4-resume-increment-analysis.md（N1-N15）。
#140 面判定口径同前：internal/modules/career|workbench、internal/handler、internal/router、internal/container、migrations、apps/web/src/career、apps/web/src/api*、packages/career-core、packages/api-client、apps/miniprogram(career)；packages/views 及 apps/web 其余迁移面为 OOS。

新增编号沿用 N 序列（N16-N29，接 N1-N15）。

## 结论先行

**#140 career 面零新增**：14 条新增条目全部位于 `apps/web/src/documents/`（Vue→React documents 迁移面），无一条落在 #140 career 实现面。技术核验 14/14 属实（无 FALSE_POSITIVE），按前例口径（round4-highrisk-analysis.md 对 KnowledgeDocumentsPage.tsx:3159 的判定）统一判 OUT_OF_SCOPE，fixHint 供迁移线。

## 一、新增 bug·high（4 条，全部 OOS·documents 迁移面）

[OUT_OF_SCOPE] N16 bug·high apps/web/src/documents/DocumentsPage.tsx:137 onClick no-op 属实：基线核验 137 行 Reload 与 165 行 Try again 均为 `onClick={() => void load}`——对 useCallback 异步函数（:60 定义）引用求值即丢弃，未调用；:69 的 effect 用的是正确的 `void load()`，证明本处是笔误而非约定。错误态重试完全失效。fixHint(供迁移线)=两处改 `onClick={() => void load()}`。

[OUT_OF_SCOPE] N21 bug·high apps/web/src/documents/KnowledgeDocumentsPage.tsx:4220 类名粘连属实：stageNoticeClass（:2764-2766）返回 `` `wk-documents-toast ${tone}` `` 尾部无空格，模板 `${stageNoticeClass(tone)}wk-stage-notice}` 拼出 `wk-documents-toast errorwk-stage-notice`，tone 词与 `.wk-stage-notice` 粘成无效 token；且 documents.td.css 的 tone 选择器为 `is-error` 前缀式，与返回的裸 tone 名双重不匹配，stage notice 以裸 div 渲染挤坏布局。STAGE_NOTICE_TONE_CLASS（:2769-2774）定义后零引用成死常量（noUnusedLocals 下编译报错）。fixHint(供迁移线)=改 `className={`wk-stage-notice is-${stageNotice.tone}`}` 并删除死常量。

[OUT_OF_SCOPE] N28 bug·high apps/web/src/documents/documents-u.css:2704-2707 非法颜色值属实：基线 2705 行 `border-color: #07c05f]/4;`（Tailwind `border-[#07c05f]/40` 的截断误译），整条声明被解析器丢弃，view-mode 按钮 hover 边框失效。fixHint(供迁移线)=`border-color: rgb(7 192 95 / 40%);`。

[OUT_OF_SCOPE] N29 bug·high apps/web/src/documents/documents-u.css:76-79 calc() 语法错误属实：基线核验 L79 `calc(100vw-20px)`、L348 与 L859 `calc(100%+4px)` 共 3 处运算符缺空白，声明整体被丢弃（悬浮气泡 max-width 与两处下拉 top 定位失效）。fixHint(供迁移线)=三处补空格：`calc(100vw - 20px)`、`calc(100% + 4px)`。

## 二、新增 bug·medium（4 条，全部 OOS·documents 迁移面）

[OUT_OF_SCOPE] N18 bug·medium apps/web/src/documents/DocumentsPageChrome.tsx:153-160 span role="link" 回归属实：基线 153-160 行 `<span role="link" tabIndex={0} onClick=…>` 无 href 无 onKeyDown——中键/Ctrl+点击不开新页、键盘 Enter 无法触发、状态栏无 URL。fixHint(供迁移线)=恢复 `<a href={tab.href}>`，onClick 内修饰键/非左键短路回退浏览器默认，`aria-current={tab.active?'page':undefined}`。

[OUT_OF_SCOPE] N23 bug·medium apps/web/src/documents/KnowledgeDocumentsPage.tsx:4574-4581 批量重建双重确认属实：Popconfirm onConfirm（:4580）→ reparseSelected（:3979）仅 setPendingBatchReparse，:5240 起 legacy Dialog 用同一段 confirmBatchReparseDocument 文案（:5246）二次询问，确认两次才发 batchReparse。fixHint(供迁移线)=onConfirm 直接执行确认后逻辑（过滤 ids + confirmBatchReparseDocument 请求体），删除遗留 Dialog 或 Popconfirm 二选一。

[OUT_OF_SCOPE] N24 bug·medium apps/web/src/documents/KnowledgeDocumentsPage.tsx:4610-4617 批量删除双重确认属实：Popconfirm onConfirm 仅 `setConfirmingDelete(true)`，:5205 起 confirmingDelete 仍渲染 legacy 确认 Dialog（:5217 确认按钮才调 deleteSelected），同文案确认两次。fixHint(供迁移线)=onConfirm 改 `() => void deleteSelected()`，Popconfirm 作唯一确认层。

（注：N23/N24 同根因——迁移换装 Popconfirm 时未拆 legacy Dialog，属同一修复模式的两处实例。）

## 三、新增 maintainability（6 条，全部 OOS·documents 迁移面）

[OUT_OF_SCOPE] N17 maintainability·low apps/web/src/documents/DocumentsPageChrome.tsx:132 不可达空态分支属实：sortedKbList（:99-104）构造为——current KB 不在列表时原样返回 kbList（位于 :116 `kbList.length ?` 真值分支内，必非空），在列表时返回 `[current, ...其余]` 必非空，`!sortedKbList.length`（:132）恒 false。fixHint(供迁移线)=删除该分支。

[OUT_OF_SCOPE] N19 maintainability·low apps/web/src/documents/DocumentsPageChrome.tsx:41-44 Chevrons 死代码属实：grep 全文件仅定义处一处引用，换装 TIcon chevron-right/chevron-down 后无消费。fixHint(供迁移线)=删除常量。

[OUT_OF_SCOPE] N20 maintainability·medium apps/web/src/documents/KnowledgeDocumentDetailPage.tsx:181 chunks 预取 `.catch(() => {})` 吞错属实：基线 181 行确认，失败时「共 N 个片段」徽章静默缺失且无线索。fixHint(供迁移线)=catch 内 console.warn 保留排查线索，或注释显式声明吞错意图。

[OUT_OF_SCOPE] N22 maintainability·medium apps/web/src/documents/KnowledgeDocumentsPage.tsx:4564-4565 批量下载上限 200 魔法数字属实：基线 `selected.size > 200` 硬编码，超限仅静默禁用无提示。fixHint(供迁移线)=提取 `MAX_BATCH_DOWNLOAD = 200` 具名常量并在禁用场景给 tooltip/文案。

[OUT_OF_SCOPE] N25 maintainability·medium apps/web/src/documents/KnowledgeDocumentsPage.tsx:463-469 菜单项 div role="menuitem" 键盘不可达属实：基线 463-469 确认 `<div role="menuitem" onClick>` 无 tabIndex 无 onKeyDown（同文件 more-wrap 触发器、card-analyze-trace-link 等同类）。fixHint(供迁移线)=恢复 `<button type="button">`，或统一补 tabIndex={0} + Enter/Space 处理。

[OUT_OF_SCOPE] N26 maintainability·low apps/web/src/documents/TagPickerDialog.tsx:279-284 TagFilterPanel 死 API props 属实：onClear（:239 定义、:259 解构）在渲染体内零调用点；onClose（:240/:260）同组件内亦无调用（:217 的 onClose 消费属同文件另一组件），关闭改由 Popup 外点承担。fixHint(供迁移线)=从 props 接口与解构移除两者，同步清理 KnowledgeDocumentsPage 传参。

## 四、新增 bug·low（1 条，OOS·documents 迁移面）

[OUT_OF_SCOPE] N27 bug·low apps/web/src/documents/documents-u.css:2089-2091 blockquote 左边框宽度平移失真属实：基线 2091 行 `border-left-width: 8px`，与注释自述 Vue 权威值 4px、被平移原 utility `border-l-2`=2px 均不符，属 codemod 误译。fixHint(供迁移线)=按注释跟进口径改 4px 或先回 2px，与注释保持一致。

## 五、差量构成附注（非新增）

- 移除 1 条：packages/views/src/craft/{document,files,home,interaction-card,library,preview,shell,slides,sources,spreadsheet}.tsx:0-0 聚合条目在 resume2 中消失（resume 有、resume2 无）。该条属 craft 迁移旧面（前几轮已多次判定 OOS），消失不影响 #140 面结论。
- resume 报告头 "19 of 319 selected failed" → resume2 "9 of 319 failed"：resume2 补扫了 resume 中 failed 的 items（本轮 14 条新增 documents 条目即来自此前 failed 的 apps/web/src/documents 组），并因 LLM retry 自身 cancellation 仍有 9 项 failed。
- 共同 385 条描述段逐条一致：resume 轮已判定条目（含主报告 23 条 + N1-N15）在 resume2 中原样复现，不改变既有判定。

## 统计

- 新增合计 14 条（N16-N29）：**#140 面 VALID 0 / FALSE_POSITIVE 0 / OUT_OF_SCOPE 14**。
- 优先级分布（全部 OOS·documents 迁移面，技术核验均属实）：high 4（N16 onClick no-op、N21 类名粘连、N28 非法颜色、N29 calc 语法）、medium 6（N18/N20/N22/N23/N24/N25）、low 4（N17/N19/N26/N27）。
- 落点分布：apps/web/src/documents/ 单一模块 14 条（KnowledgeDocumentsPage.tsx 5、DocumentsPageChrome.tsx 3、documents-u.css 3、DocumentsPage.tsx 1、KnowledgeDocumentDetailPage.tsx 1、TagPickerDialog.tsx 1）。
- **结论：resume2 相对 resume 在 #140 career 面零新增修复漏网**（internal/modules/career、apps/web/src/career、career-core、api-client、miniprogram career 均无新增条目）；14 条全部是 documents Vue→React 迁移面的真实缺陷，供迁移线按 fixHint 处理，其中 N16/N21/N28/N29 四条 high 建议 documents 迁移线优先修（两条为 CSS 静默失效、一条为重试按钮完全失效、一条为 toast 无样式挤坏布局）。
