# Tailwind 迁移约定（子任务执行手册）

本文档是各页面/域批量迁移的统一规范。目标：把域 CSS 的样式改为 Tailwind utilities（优先语义令牌），删除域 CSS 文件，保持视觉与行为不变。

## 硬性规则
1. 只改任务清单列出的文件。不得改 apps/web/src/styles.css、packages/ui/src/**、packages/i18n/**、路由、业务逻辑、API 调用。
2. 视觉 1:1：颜色/间距/圆角/字号用令牌或任意值精确还原（对照表见下）。不确定时保留任意值（如 border-[#cbd5e1]）。
3. data-testid、aria-*、role、disabled/loading 逻辑、事件处理一律不动。
4. 不新增依赖、不跑 pnpm install、不 git commit。
5. 测试：TSX/TS 测试中选择器引用被删类名的，改为语义查询（role/text/data-testid），只改选择器不改断言语义；在回复中列出改动数。
6. 完成后必须跑：pnpm typecheck:web 和任务指定的测试文件（在 worktree 根 /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient 执行），全绿才算完成。

## 令牌对照（packages/ui/src/theme.css @theme）
- 表面：bg-canvas #f7f9fc｜bg-surface #fff｜bg-surface-alt #f2f4f8｜bg-hover-wash #f2f5fa｜bg-surface-wash #eff4ff
- 文本：text-ink #172033｜text-ink-strong #101828｜text-muted #66758b｜text-muted-strong #506078｜text-faint #98a2b8
- 线：border-line #dce3ed｜border-line-soft #edf0f5｜border-line-strong #b8c5d6｜border-line-input #dcdcdc｜border-line-neutral #e7e7e7｜border-line-subtle #e4e7ec｜border-line-control #cbd5e1
- 蓝：text-primary #2e6de6｜primary-strong #0052d9｜primary-deep #1849a9｜primary-soft #245a9b
- 绿：accent #07c05f｜accent-strong #0a8f4c｜accent-deep #00a870｜accent-soft rgba(7,192,95,.12)
- 状态：success #00a870｜success-text #137333｜warning #e37318｜warning-text #9a6700/#b54708｜danger #b42318｜danger-strong #d54941
- 圆角：rounded-control 6px｜rounded-card 8px｜rounded-pill 999px
- rgba 精确值用任意值：bg-[rgb(120_135_155_/_0.35)]、shadow-[0_4px_14px_rgba(15,23,42,.06)]。

## 共享类配方（迁移时替换为）
- wk-page → mx-auto box-border max-w-[960px] px-[1.25rem] py-12（个别文件有自己的 max-width 就以该文件 CSS 为准）
- wk-header → flex items-start justify-between gap-4 mb-6
- wk-eyebrow → m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary
- wk-muted → text-muted（若原 css 带 font-size 就一并带上）
- wk-debug → border-b border-line-soft pb-2 font-mono text-xs text-muted break-words
- wk-list → m-0 list-none p-0；li: flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]；li span: font-mono text-[0.8rem] text-muted
- 注意：以上为 apps/web/src/styles.css 中的定义；每个文件迁移前必须先读该文件实际引用的 CSS 规则原文，按原文转换，不照抄本配方。

## 组件使用与测试环境约定

- packages/ui 现有：Button（variant: default/primary/text/danger；loading 保留）、Card、Status、
  Input、NumberInput、Switch、Textarea、Select（原生）、Checkbox（原生）、Label、Badge、Alert、
  Separator、Table、Tabs/Sheet/DropdownMenu/Tooltip（Radix）、Dialog（手写 DOM 契约）。
- Dialog 有意不用 Radix/Portal：仓库的 node --test 测试环境只注入最小 DOM 全局
  （window/document/HTMLElement/Event/navigator），Radix 的 Presence/FocusScope 需要
  getComputedStyle/MutationObserver，会炸一片既有测试。既有弹窗均页面顶层挂载，原位渲染即旧行为。
- 若某页面的测试需要渲染 Radix Sheet/DropdownMenu（含 Portal），在该测试文件的 jsdom
  引导里补：getComputedStyle、MutationObserver（模式：
  `Object.assign(globalThis, { getComputedStyle: dom.window.getComputedStyle.bind(dom.window), MutationObserver: dom.window.MutationObserver })`），
  或优先使用无 Portal 的等价组件。
- 视觉未动之前：不动共享类在 apps/web/src/styles.css 的规则；仅当某类的最后一个使用方完成迁移时，
  由 Orchestrator 统一删除该规则。

## 主题变量保留规则

settings 域大量样式引用 .wks-modal 抽屉作用域的主题变量（var(--wks-text-secondary)、
var(--wks-border)、var(--wks-primary) 等）。这些是既有主题能力，转 utilities 时必须保留为
任意值形式（如 text-[var(--wks-text-secondary,#6b7280)]、bg-[var(--wks-border,#e5e7eb)]），
禁止压平成固定 hex。仅当某变量在全局只此一处且无主题语义时才可直接用令牌。

## JS 钩子类保留规则

类名可能被事件逻辑用作 DOM 钩子（querySelector/closest/matches 参数等）。删除任何类名前，
必须 grep 该类名在域内 tsx 的字符串出现；凡作为 JS 钩子的类名一律保留（样式可改 utilities，
类名留作钩子），并在报告中列出保留清单。

## 结构规则
- 每个域：先读域 css 全文与全部消费 tsx，列出「类名→规则→出现位置」清单，再逐文件替换。
- 域 css 中确属第三方/复杂动画/富文本的规则可保留为 css（放回同文件），并在回复中说明保留原因与行数。
- css 文件只有在所有规则都被替换后才能删除 import 与文件；删不掉的列出来。
- 完成回复格式：改动文件清单、删除的 css 规则数/保留数与原因、测试选择器改动数、门禁输出摘要。
