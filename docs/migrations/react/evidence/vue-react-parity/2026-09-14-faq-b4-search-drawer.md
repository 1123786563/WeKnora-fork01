# B4 FAQ 检索测试抽屉（search test drawer）

Date: 2026-09-14
Slice: codex/react-multiclient FAQ B4（检索测试抽屉）
Owner files: apps/web/src/faq/**（FAQPage.tsx、faq.css、测试）

## Vue baseline（FAQEntryManager.vue）

- 入口：faq-filter-bar__trailing 的文本图标按钮（search 图标），tooltip/label
  `knowledgeEditor.faq.searchTest`（检索测试），点击打开抽屉（:224-230，
  handleFaqAction case 'search' :1018-1020）。不受 canEdit 门控（检索是读操作）。
- 抽屉：t-drawer 右侧 420px，header=`searchTestTitle`（FAQ 检索测试），带关闭
  按钮（:734-736）。
- 表单（垂直 settings-group，每行 label+desc）：
  - 查询内容 queryLabel / desc 与 placeholder 同为 `queryPlaceholder`，Enter 直接触发检索（:741-750）。
  - 相似度阈值 similarityThresholdLabel / vectorThresholdDesc（范围 0-1，默认 0.7），
    t-slider min=0 max=1 step=0.1，实时值 toFixed(2)（:752-765）。
  - 结果数量 matchCountLabel / matchCountDesc（范围 1-50，默认 10），
    t-slider min=1 max=50 step=1（:767-779）。
  - 主色块宽按钮：searching ? `searching`(检索中...) : `searchButton`(开始检索)，loading 态（:781-788）。
  - 默认值：query ''、vectorThreshold 0.7、matchCount 10（:1334-1338）。
- handleSearch（:2650-2680）：
  - 空白 query（trim 后为空）→ MessagePlugin.warning(queryPlaceholder)，不发请求。
  - POST searchFAQEntries(kbId, { query_text: trim, vector_threshold, match_count })。
  - 成功：res.data 摊开，每条 expanded=false，按 score 降序排序（:2664-2673）。
  - 失败：MessagePlugin.error(error?.message || common.operationFailed)，结果清空。
- 结果区（:792-851）：`results.length > 0 || hasSearched` 才渲染；header
  `searchResults` (n)；空态 `noResults`（未找到匹配的 FAQ 条目）；命中卡片：
  1-based 序号、standard_question、matched_question（仅当与标准问不同，label
  `matchedQuestion`:）、score tag toFixed(3)、点击头部展开/收起；展开体展示
  answers（答案）与 similar_questions（相似问）tag 列表。
- 三态：加载（按钮 loading/检索中...）、空态（noResults）、错误态（toast；
  React 页面已确立的错误通道是 drawer 内 error slot，参考编辑抽屉 faq-editor-error）。

## React implementation

- FAQPage.tsx：
  - 纯函数/常量导出（可测 seam）：`faqSearchDefaultForm`（0.7/10 默认）、
    `FAQ_SEARCH_VECTOR_THRESHOLD`(0-1/0.1)、`FAQ_SEARCH_MATCH_COUNT`(1-50/1)、
    `faqSearchBlocked`、`faqSearchRequestFrom`（trim + 字段映射）、
    `faqSearchResultsFromResponse`（envelope 解包 + score 缺省 0 + 降序）、
    `toggleSearchResultId`（不可变展开翻转）。
  - FAQSearchResults 组件（导出）：结果区 header/空态/命中卡片/展开体，1-based
    序号、matched_question 仅异于标准问时渲染、score toFixed(3)、
    aria-expanded 头部按钮；SSR 可直接断言。
  - FAQPageView 新 props：searchOpen / searchForm / searching / hasSearched /
    searchResults / onOpenSearchTest / onCloseSearchTest / onSearchFormChange /
    onSearchTestSubmit；抽屉 JSX 复用 faq-editor-overlay/faq-editor-drawer 骨架
    （420px 覆盖）+ settings-group 行 + 原生 input[type=range] 两个滑杆 +
    slider-value（threshold toFixed(2)）+ Button loading=searching。
  - 工具栏检索测试按钮 onClick 由误接的 onSearchSubmit（列表关键字过滤）
    改为 onOpenSearchTest —— 修复了按钮点开的是列表过滤而非抽屉的行为偏差。
  - FAQPage 容器：searchOpen/searchForm/searching/hasSearched/searchResults
    状态 + runSearchTest（blank 警告 → faq.search → 成功排序渲染 / 失败
    error+清空），Vue 三态全覆盖；展开集合在新检索发起时重置（对齐 Vue
    每次结果替换 expanded=false）。
  - i18n 回退层（imWizardMessages 模式）：`createFaqTranslator` 把 Vue locale
    byte-exact 的缺失键垫在 formatMessage 之下 —— 本次抽取
    `common.operationFailed`（操作失败/Operation failed/操作に失敗しました/
    작업 실패/Операция не выполнена）与 `common.close`（关闭/Close/閉じる/
    닫기/Закрыть）；检索测试抽屉其余键（searchTest/searchTestTitle/queryLabel/
    queryPlaceholder/similarityThresholdLabel/vectorThresholdDesc/matchCountLabel/
    matchCountDesc/searchButton/searching/searchResults/noResults/matchedQuestion）
    共享目录已具备（5 locale），新 affordance 无新增 zh-only 文案。共享目录键
    上游补齐后回退自动失效。
- faq.css：faq-search-drawer 420px 覆盖 + slider-wrapper/slider-value/search-button
  全宽 + search-results/results-header/no-results/results-list/result-card/
  result-header/result-main/result-question/result-index/matched-question/
  score-tag/expand-icon/result-body/result-section/question-tag(is-answer) 样式。
- 测试：
  - FAQPage.test.tsx 新增 12 个 SSR 断言用例（默认开闭、表单键值/滑杆 bounds/
    默认值、检索中按钮态、结果卡片序号/分数/命中问、展开体、空态门控、
    抽屉内错误 slot、4 个纯函数用例）。
  - faq-search-drawer.test.tsx 新增 5 个 jsdom+act 交互用例（真实点击工具栏
    按钮开抽屉/关闭按钮、空白 query 不发请求且出现警告、完整检索往返
    （trim payload 0.7/10、降序、0.912/0.500 分数）、失败态（drawer 内 alert、
    清空旧结果、退出 loading）、展开命中展示答案/相似问）。

## Verification

- `cd apps/web && npx tsx --test "src/faq/"*.test.tsx "src/faq/"*.test.ts`：
  58/58 通过（41 基线 + 17 新增；faq-import-poll 轮询用例与 tags/import-excel
  解析用例全部保留未回退）。
- TDD 过程：先 14 红（41 pass / 14 fail），实现后转绿；SSR 无法触发
  function component 内部点击，两条元素树遍历用例改为 jsdom 真实点击并按
  账本 Round N+8 collect-then-assert 经验书写（act 链上不抛 AssertionError，
  收集结果后统一断言），全程无 node:test 挂起。
- `pnpm run typecheck:web`：唯一错误为
  `src/integrations/ApiPlaygroundDrawer.test.tsx(26,35): TS1005` —— 并行
  api-playground 切片的未跟踪 WIP 文件（非本切片归属，未触碰）；除此之外
  tsc 无任何诊断，faq 切片文件类型干净。
- 改动文件（git diff --stat）：FAQPage.tsx +276/-1、FAQPage.test.tsx +114/-5、
  faq.css +127、新增 faq-search-drawer.test.tsx。未触碰 docs 账本/matrix、
  PlatformShell、integrations 及任何共享文件。

## 遗留（leftovers）

- Vue t-slider 的 format-tooltip（悬停 0.92 样式气泡）由原生 range 承接，无
  tooltip 呈现（可见 .slider-value 实时值对齐 Vue 语义）；如需像素级一致可
  后续补自绘 tooltip。
- Vue 错误态是 MessagePlugin 全局 toast；React 沿用页面已确立的 message →
  drawer 内 error slot 通道（与编辑抽屉一致），未引入新的 toast 机制。
- `common.operationFailed`/`common.close` 目前走 faq 本地回退层；待上游
  packages/i18n 收录后可整体删除（FAQPage 其他既有调用点同享该修复）。
- 未做认证态 Vue/React 同视口截图对比（无真实后端环境），检索链路以 jsdom
  fake client 契约断言为准。
