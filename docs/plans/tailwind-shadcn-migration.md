# Tailwind CSS + shadcn/ui 迁移实施计划（react-multiclient worktree）

对象：本 worktree（codex/react-multiclient 分支）的 React Web 界面（apps/web、packages/ui、packages/views）。
不变项：React 19 + TS + Vite 7、页面布局、品牌色、字体层级、信息架构、路由、i18n、
API 契约、鉴权、权限、表单校验、聊天流式、工具审批、上传、错误处理；Vue parity 规范优先。
不涉及：apps/mobile（RN，不引用 @weknora/ui）、apps/desktop（独立 renderer，无 ui/views 依赖）、Go 后端、frontend/ Vue。
注意：apps/embed 依赖 @weknora/views，views 层改动需补跑 embed 门禁（typecheck/test/build）。

## 基线（改造前，HEAD 3076630a）

- 门禁全绿：test:shared 445、test:web 856、typecheck:web 0、build:web 通过（chunk 体积警告为既有）。
- 本地真实栈可用：Go 后端 :8080（worktree 启动）、vite :5181（VITE_API_BASE_URL=localhost:8080）。
- 视觉基线：artifacts/tailwind-shadcn/baseline/*.png（21 张：18 路由 @1440px、kb-list @390px、login/register/not-found）。
- 截图账号：uimig@local.dev（公共注册接口创建；kb-list 含新用户引导弹层，前后状态一致）。
- 既有问题（非迁移引入，记录不掩盖）：Tailwind 入口仅 utilities.css 且无 @source，
  packages/ui 内 utilities（Input/NumberInput 的 h-8、border-[#dcdcdc] 等）从未生成——
  这些组件的 Tailwind 类当前实际不生效，仅 .wk-* 兜底 CSS 生效（switch.tsx 注释即此问题）。
  基础设施修复后这些既有声明开始生效，属"让已写样式真正生效"，需截图复核无视觉漂移。

## 基础设施

1. styles.css 入口：引入 tailwindcss/theme.css（默认令牌）+ utilities.css，
   并以 @source 指向 packages/ui/src、packages/views/src（修复扫描盲区）。
   Preflight 决策：暂不引入。理由：约 1.23 万行既有 CSS 与大量依赖浏览器默认样式的区域
   （标题间距、列表标记等）在迁移期会被 reset 破坏 parity；待领域 CSS 全部转为 utilities
   后再评估启用（同视口截图对比验收）。保留 theme-mode 属性 + color-scheme 主题机制。
2. 语义令牌：packages/ui/src/theme.css（@theme），由 apps/web 引入。品牌色值 1:1 映射：
   ink #172033、muted #66758b、surface #ffffff、canvas #f7f9fc、line #dce3ed、primary #2e6de6、
   danger #b42318、control accent #07c05f（Input/Switch/NumberInput 现用绿色，Vue parity）、
   warning #9a6700、hairline #edf0f5、field-line #b8c5d6；radius control 6px / card 8px。
3. 生产构建扫描验收点：build 后抽查 dist css 含包内 utilities（如 h-8）。

## 组件体系（packages/ui，shadcn 规范：cva variants + cn）

保留公开 API 兼容优先，逐步 shadcn 化：
- Button：cva variants（default/outline/ghost/text/destructive + 尺寸 + loading 保留），
  吸收 wk-button--text（22 处）等用法。
- Input/NumberInput/Switch：类名改语义令牌 utilities（视觉不变）。
- Card/Status 保持；新增 Badge、Alert、Textarea、Select（原生 select 样式化，保表单语义）、
  Checkbox（原生样式化）、Label、Separator、Table、Tabs（受控无头）。
- Dialog：保留 DOM 契约（focus/Escape/restore），内部迁移到 @radix-ui/react-dialog
  （portal/focus-trap/aria），样式 tokens 化；Sheet（侧滑抽屉）基于同库新增。
- 不安装 sonner/toast：现有通知面为内联 Status/Alert/popconfirm（按需原则）。

## 进度账本

运维记录：首批并行子任务因始终未执行文件写入被中断（只读分析循环）；
前台探针证实绝对路径写入可用。已带侦查结论 + 「边读边写 + 后台门禁」纪律重新派发。

### 续作指引（上下文交接，2026-09-14）
- 已完成批次：auth、sandbox、memory-workspace、knowledge-list、command-palette（5/域）+ 基础设施/组件体系。
- 子任务经验：小域（css<200 行）可成功；大域子任务会在长读阶段耗尽自身上下文而失败
  （skill-settings 568/TenantMembers 751 两个 agent 均失败于读取阶段，无写入）。
  → 大域改由 Orchestrator 分片自做（staged 引擎，auth/KB 已验证）或拆更小子任务。
- 剩余域与规模：shell.css 581（PlatformShell.tsx）｜faq 1420（FAQPage）｜
  organizations 1235（OrganizationsPage）｜documents 1288（KnowledgeDocumentsPage）｜
  agents 759+367（AgentsPage/AgentEditorModal）｜settings-wrapper 280（多面板）｜
  TenantMembers 751｜personal-memory 604｜skill 568｜chat 1668（ChatRoutePage）｜
  guides 243（views，影响 embed，需补 embed 门禁）。
- 共享收尾（最后做）：styles.css 共享类规则删除（wk-page/wk-header/wk-muted/wk-list/wk-form/
  wk-toolbar/wk-tag/wk-switch/wk-popconfirm/wk-settings-* 等）需逐类确认最后一个使用方已迁移；
  packages/ui/src/styles.css 的 wk-* 规则随组件迁移已部分失效，最终清理；依赖审计。
- 验证资产：tailwind-shadcn-screens.mjs / tailwind-shadcn-diff.mjs / section-shot 脚本；
  基线+after-{foundation,auth,kblist} 截图；本地栈（:8080 sqlite / vite :5181，账号 uimig@local.dev）。
- 本地栈注意：先前 dev 栈进程已结束；vite/后端均可用后台 job 方式自启（后端二进制 /tmp/uimig-server，
  sqlite /tmp/uimig-weknora.db，含 uimig 账号与一条 KB）。
### 批次17：chat 域 ✅（子任务执行，Orchestrator 验收提交）
- ChatRoutePage 的 5 个 views 组件全迁移；chat.css 1668→110 行
  （9 条特异性守卫 + 3 条 @keyframes；守卫=styles.css legacy 块在产物中后置一直赢、
  chat.css 靠高特异性赢的少数规则，batch-12 删 styles.css 块时一并删除即零漂移）。
- 测试选择器 3 处容忍 utilities 追加；钩子类全数保留；mermaid 由 styles.css 承载未动。
- 验收：typecheck 0、chat-page 20/20、views chat 55/55、web 856/856、build ✓。
  ⚠ batch-12（styles.css 清理）前置项：.wk-chat-message-content 的 pre/table/citation/
  mermaid/math 与 .wk-chat-messages li p 富文本规则需先迁移或保留。
### 批次18：personal-memory 域 ✅（子任务执行，Orchestrator 验收提交）
- PersonalMemorySettingsPanel.tsx 50 处 className 迁移（分页/进度/通知/伪元素分隔符等）；
  personal-memory.css 删除（604 行、89 规则块、无 keyframes/主题变量）。
- 测试选择器 8 处语义化；JS 钩子类 0（ref 方案）。
- 验收：typecheck 0、面板测试 11/11、web 856/856、build ✓。
### 批次20：wiki 域 ✅（styles.css 域块清理 1/N）
- WikiPage.tsx 20 处 className 迁移（index/toolbar/folder-list/layout/sidebar/search/
  page-list/reader/editor/history 六档；page-item hover 与 summary line-clamp）。
- styles.css 删除 .wk-wiki-* 25 条 + .wk-sr-only（sr-only 语义由 utilities 复刻）。
- 验收：typecheck 0、web 856/856、build ✓。
### 批次21：documents styles.css 切片 ✅（子任务执行，Orchestrator 验收提交）
- documents 三组件 20 处追加 utilities（页面宽 1180!/820!、layout 双分支、folder 家族、
  list-actions、marquee color-mix!、metadata dl 子变体、preview 尺寸）；preview.ts 一并处理。
- styles.css 删除 22 条 documents 规则（1128→1055 行）；保留 4 条共享规则
  （wk-list-actions/span、wk-document-link——约 30 处域外消费方，最后统一删）。
- 关键实现：unlayered 竞争属性用 ! 任意值；静态条件替代插值任意值（扫描器限制）。
- 验收：typecheck 0、documents 38/38、web 856/856、build ✓、styles.css 1128→1055 行。
### 批次24：chathistory stats 切片 ✅（settings-wrapper 域 5/N）
- ConfigSettingsPanel.tsx chathistory stats 区 9 处 className utilities 追加
  （采用「钩子类+追加 utilities」安全模式，避免与并发写冲突时的结构风险）；
  settings-wrapper.css 删除 9 条 stats 规则（115→106 行）。
- 验收：typecheck 0、web 856/856、build ✓。
### 批次25-26：最终阶段两切片 ✅（双子任务并行，Orchestrator 合并验收提交）
- 批次25（knowledge-settings + configuration 域块）：styles.css 删 10 条/19 行；
  KnowledgeSettingsPage/ConfigurationPage/App 弹窗 utilities 化（unlayered 竞争用 !：
  max-w-[1180px]!×2、text-[#dc2626]!）；knowledge-settings/configuration 测试 42/42；钩子类 0 残留。
- 批次26（settings-wrapper 收尾）：删 12 条（GeneralPreferences select×4/font-preview×2+媒体/
  Config chathistory wk-switch 家族 7/desc.warning-text）；select chrome 抽屉级 4 条按判定保留
  并在文件头登记 9 文件 20 处消费方清单（交后续逐面板收编或永久保留，Orchestrator 决策项）；
  指定测试 70/70 + SettingsPage 15/15；typecheck 0、build ✓。
- 合并验收：typecheck 0、web 856/856、shared 445/445、build ✓。
### 剩余 styles.css 域块清单（1013 行，逐族清理排队）
基于当前结构盘点，剩余家族（估算行数）：
- 共享层（留最后）：wk-page/header/eyebrow/debug/list/list-item-*/form/form-grid/toolbar/pagination/settings-panel-heading 约 120 行
- integrations 残余：wk-integrations-*/wk-int-*/wk-channel-*/wk-channels-*/wk-mcp-*/wk-secret-output/wk-upload-progress* 约 220 行
- chat 联动：wk-chat-message-content 富文本（pre/table/citation/mermaid/math）/wk-chat-messages li p/
  wk-chat-artifacts/wk-chat-reference*/wk-chat-suggestions* 约 150 行（与 chat 波 9 条守卫联动删）
- data-sources：wk-data-source-*/wk-data-sources-page 约 60 行
- admin：wk-admin-*/wk-checkbox/toggle-grid 约 40 行
- configuration 残余/wk-diff/wk-preview-box/result/settings-editor/settings-layout 等散块 约 180 行
- organizations/wk-organization-*：OrganizationsPage 已 utilities 化但 styles.css 的
  wk-organization-layout/select 残留（确认后删）约 20 行
- share dialog：wk-share-*（KnowledgeBaseShareDialog）约 60 行
- knowledge settings/faq/editor 残余 约 40 行
清理纪律同 conventions：逐族 grep 消费方→utilities 追加→删规则→门禁→提交；
hook 类（测试/JS 引用）保留类名。
### 批次27：data-sources + admin 域块 ✅（子任务执行，Orchestrator 验收提交）
- DataSourcesPage 14 处 + AdministrationPage 14 处 utilities 化；
  styles.css 删 19 条（data-sources 13 + admin 5 + 720px media 1）；
  合并规则拆分（.wk-list-actions select 保留）；styles.css 1122→975 行。
- 代理纠正任务前提：.wk-checkbox/.wk-toggle-grid 实际消费方为 KnowledgeSettingsPage:76-77
  与 McpSettingsPanel:940（非 AdministrationPage），按协议保留待后续域处理。
- 验收：typecheck 0、McpToolsDirectory+SettingsPage 测试 18/18、web 856/856、build ✓。
### 批次28：packages/ui Dialog 焦点陷阱 + 主题变量桥接 ✅（外部贡献，Orchestrator 验收）
- dialog.tsx：Tab 循环焦点陷阱（getDialogFocusableElements 排除 disabled/tabindex=-1，
  Shift+Tab 反向），补齐目标 III 的键盘/焦点管理要求。
- styles.css：legacy --wk-* 变量桥接主题令牌（var(--color-ink,#172033) 等），
  独立 UI 包保持稳定，接入 theme.css 的应用自动跟随令牌。
- interaction.test.tsx：新增 getDialogFocusableElements 过滤断言（jsdom）。
- 验收：typecheck 0、web 856/856、shared 446/446（+1 新测试）、build ✓。
- 运维注：一次全量套件出现 13 个时序抖动失败（vite/后端/多代理并发高负载），
  系统空闲后复跑 856/856 确定为环境噪声，非代码回归。
### 批次29：channels 频道卡家族 ✅（子任务执行，Orchestrator 验收提交）
- page.tsx ChannelListPanel 全家族 → 13 个模块级 Tailwind 常量类（变体互斥拆分）；
  styles.css 删 22 条（20 条 1:1 + 2 条死规则 rename 家族）；styles.css wk-channel* 清零；
  测试选择器 29 处语义化（imWizard 9/embedWizard 18/embedPreviewFallback 2）；JS 钩子 0。
- 验收：typecheck 0、integrations 52/52、web 856/856、build ✓。
### 批次30：wk-mcp 大家族 ✅（子任务执行，Orchestrator 验收提交）
- styles.css 删 113 行 .wk-mcp-* 规则（含 12 行死规则）；McpToolsDirectory/
  McpTestResultBody/McpSettingsPanel 三组件全迁移（9 个共享 utility 常量）；
  测试选择器 5 处语义化；钩子类 6 个保留（测试 querySelector 引用）+ 4 个纯钩子未动。
- 关键生效值编码：z-[1200]!（压 .wks-overlay）、p-[.45rem]!（压 settings-editor input）、
  竞争色按原级联胜者加 !；重复规则合并；720px media → max-[720px]:grid-cols-1。
- 验收：typecheck 0、mcp 测试 18/18、web 859/859、build ✓；styles.css 1013→835 行。后续保留 Vue 对齐所需语义钩子并修复参数解析，提交 `3828ecba`。
### 批次32：share dialog 家族 ✅（子任务执行，Orchestrator 验收提交）
- KnowledgeBaseShareDialog（含 OrganizationPicker/PermissionRadio）34 条规则全迁移；
  styles.css wk-share-* 清零；无 keyframes；JS 钩子 0（唯一 wk-share 字符串是 aria-controls
  指向的 DOM id）；测试选择器 6 处语义化。
- 保真处理：无 preflight 下显式覆盖 UA 默认（[font:inherit]、border 序列）；隐藏原生 select
  按级联生效宽度 w-full h-px 编码；trigger nth-child(2) flex:1 用 [&>span:nth-child(2)]:flex-1。
- 验收：typecheck 0、13/13、web 859/859（+3 新增测试）、build ✓。
### 批次33：knowledge graph 家族 ✅（子任务执行，Orchestrator 验收提交）
- KnowledgeGraphPage.tsx 16 处编辑；styles.css 删 45 条选择器规则/41 行
  （graph 族 15/node 族 11/drawer 3/search-results 4/type-filters 4/legend-dot 7/help 6/familiar 2/status-card 4）。
- 动态插值（is-${graphType}/is-${page_type}）改静态字面量映射 GRAPH_TYPE_DOT_BG/GRAPH_NODE_CIRCLE；
  SVG line/circle/text 直接加类；抽屉 .wk-header 覆盖按生效值 mb-[.75rem]!；720px → max-[720px]:。
- 验收：typecheck 0、graph 相关 18/18、web 865/865（+3）、build ✓；styles.css 1087→750 行。
### 批次34（进行中）：styles.css 剩余族清单已入库（164 族）
- 台账末尾附自动生成的剩余族清单（族名+规则数，已提交）。
- 修正：此前"91% 减量、接近完成"的表述过于乐观——styles.css 仍有 ~1013 行
  （164 族）+ settings-wrapper 139 行保留段 + 各域 keyframes 小文件。真实剩余规模以
  本清单为准，按族逐个收编（每族：grep 消费方 → utilities 追加 → 删规则 → 门禁）。
- 并行任务仍在 chat 域活跃（agent-selection 等在途），chat 相关 styles.css 规则联动继续延后。
### 共享长尾清理依赖记录（2026-09-14）
- wk-diff（wiki）/wk-pagination（wiki+documents+faq 测试负断言）/wk-capability-grid/check-row/
  role-badge（configuration）等共享散族清理延后：并行 chat 域代理在共享树活跃
  （agent-selection/ChatRoutePage/composer WIP），部分消费者文件与其重叠，
  为避免 App.tsx 式并发写截断重演，待其提交收敛后由 Orchestrator 统一处理。
- 届时顺序：先跑 pnpm typecheck:web 确认 0 → 逐族 grep 消费方 → utilities 追加 →
  删规则 → 门禁 → 提交（每族独立提交）。
### 批次34：wk-diff/wk-pagination/wk-role-badge 散族 ✅（共享长尾清理开始）
- WikiPage.tsx：wk-diff 容器 + 动态 wk-diff-${type} span（静态条件编码三态色）
  + wk-pagination；ConfigurationPage.tsx：wk-role-badge。
- styles.css 删 9 条（wk-diff 7 + wk-pagination 1 + wk-role-badge 1）；750 行。
- 验收：typecheck 0、wiki 8/8 + configuration 8/8、web 870/870（并行新增）、build ✓。
### 批次35：integration drawer 家族 ✅（子任务执行，Orchestrator 验收提交）
- page.tsx +70/-1（INTEGRATION_DRAWER_* 常量 + [&_h3]/[&_input]/[&_select]/[&_textarea]/
  [&_.wk-im-step]/[&_.wk-embed-step] 等任意变体承载子规则）；EmbedPreviewModal.tsx L69 同步；
  styles.css 删 19 个规则块（-149 行）。
- 代理纠正简报 2 处事实错误（消费方是 EmbedPreviewModal 而非 ApiPlaygroundDrawer；
  第三个消费者 page.tsx:594 一并迁移）；发现并修复 close:hover 自 3abbdfc5 起的坏合并
  （意外宽度 min(480px,100%)，恢复原始意图 bg #f3f4f6 / color #172033）。
- 验收：typecheck 0、integrations 52/52、web 882/882（并行新增）、build ✓；
  styles.css 1122→602 行。
### 批次36：knowledge-list wk-kb 残余清除 ✅（子任务执行，Orchestrator 验收提交）
- styles.css L272-404 整段删除（22 类 wk-kb-* 死规则 + @keyframes wk-kb-shimmer）
  ——批次 4 已为每个角色落地等价 utilities，逐类 grep 证实 TSX 零引用，0 条需补。
- 另标记 3 条死代码（wk-kb-flash 家族，L466-468）待下批删。
- 验收：typecheck 0、anatomy 16/16、web 889/889（并行新增）、build ✓；
  styles.css 750→468 行。
### 批次37：embed-preview 家族 ✅（子任务执行，Orchestrator 验收提交）
- EmbedPreviewModal.tsx + page.tsx（第二消费者 deploy-step 预览面板 + launcher 色块）
  全家族 utilities 化；styles.css 删 24 条 embed-preview 规则（tombstone 注释保留）；
  测试选择器 3 处语义化；JS 钩子 0（is-loading 动态类改静态条件）。
- 验收：typecheck 0、integrations 27/27、web 889/889、build ✓。
- 同提交附带：wk-kb-flash 死家族删除（TSX 零引用）；styles.css 468→455 行。
### 批次37-38：integrations 残余 + capability/check-row/option-chip 散族 ✅（双子任务并行，Orchestrator 合并验收提交）
- 批次37（integrations 残余）：page.tsx 全部家族类转共享常量（INT_TAB/INT_DOC_LINK/
  CODE_TOOLBAR/INTEGRATION_FORM 作用域变体）；styles.css 删 23 条；styles.css 中
  wk-integrations-*/wk-int-*/wk-code-toolbar 清零。52/52 测试、build ✓、dist 抽查通过。
- 批次38（capability/check-row/option-chip）：真实消费方在 page.tsx（纠正简报误写
  ConfigurationPage）；CHIP_BASE/chip() 常量 + check-row 保留为 drawer :not() 钩子；
  死类 wk-role-badge 样式恢复并去名；styles.css 删 8 条。40/40 configuration 测试、build ✓。
- 合并验收：typecheck 0、web 890/890、build ✓；styles.css 431 行。
### 批次38 补充：chat 协调最终报告要点
- 零漂移验证：chat-empty/chat-session 像素 diff 0.000%（bbox none），证据
  artifacts/tailwind-shadcn/chat-{before,after}/*.png + chat-shots.mjs 可复现脚本。
- chat.css 守卫精确定性：删 4（对手随 legacy 块死亡）；留 5 中 4 条对手=共享层
  unlayered 规则（L16 h1 clamp/L30 wk-list-actions/L70 panel-heading——「共享层留最后」），
  1 条=a11y reduced-motion（utilities 无对应物）。共享层收尾时 4 条联动删除。
- 富文本 36 条迁入 chat.css 的正当性已按 III.7 记录（markdown.ts/mermaid.ts 运行时
  生成 HTML + artifact-preview/tool-result/references 三组件零 utilities）。
- 既有偶发白屏（in-place 切换 + continue-stream 竞态）非本迁移引入，before 态同样复现。
### 批次39：model settings 残余家族 ✅（子任务执行，Orchestrator 验收提交）
- ModelSettingsPanel.tsx 6 处 + ModelDebugPanel.tsx 6 处 utilities 迁移
  （ModelEditorModal 不存在——编辑器内联在 ModelSettingsPanel，已一并处理）；
  styles.css 删 18 条（styles.css 删除已随 64ce592e 提交入库）。
- 保留 3 条：wk-model-tabs 三件套——SandboxSettingsPanel:1806 消费（范围外），
  已留注释待沙盒波删除；钩子类 6 个保留。
- 验收：typecheck 0、42/42（Model+SettingsPage）、web 890/890、build ✓。
### 批次40：styles.css 小家族清扫 ✅（子任务执行，Orchestrator 验收提交）
- document-link(2)/preview 家族(5)/tenant-dialog 家族(4)/secret-output(3 死类)/
  resource-check 家族(4) 全部迁出；WikiPage/DocumentsPage/WorkspaceOnboarding/
  KnowledgeSettingsPage/DataSourcesPage 5 文件转换。
- 关键 cascade：w-[min(480px,100%)]!（压 ui 包 .wk-dialog 基类）、[font-weight:650]!
  （[font:inherit] 排序竞争，文档页同类问题一并修复）、resource-check 按生效值
  编码（.wk-list li span 一直覆盖原 color/font-size）。
- 验收：typecheck 0、受影响域 140/140、web 890/890、build ✓；styles.css 347→329 行。
### 批次41：settings shell 家族 ✅（子任务执行，Orchestrator 验收提交）
- 17 个面板 TSX 追加按生效值编码的 utilities（panel-heading 13 处/表单/values 链
  dl-div-dt-dd 四级编码/抽屉内 sticky 变体）；测试选择器 2 处语义化。
- styles.css 删 18 条 + 3 片段（死规则 7 + 迁移后删 11）；保留 16 条 + 3 片段
  （knowledge-settings 消费方/抽屉级联/configuration 域/editor×3——最终共享层清理联动）。
- 验收：typecheck 0、SettingsPage+Model 38/38 + 加跑 48/48、web 890/890、build ✓。
### 批次42：小共享族（button/tag/status/switch/mono-input/visually-hidden/muted）✅
- 35 文件：styles.css 删 20 条（button×5/tag×2/switch×6/status×3/mono-input/
  visually-hidden/muted×2——墓碑注释）；wk-muted ≈218 处 + warn 3 追加 utilities。
- 关键级联发现：packages/ui/src/styles.css 经 index.tsx:22 进 bundle（ui css 在前、
  apps 在后=今天 apps 赢平局）——删 apps 规则后 ui 未分层规则反超，竞争属性全部加 !
  （border/px/py/text/disabled:opacity-55!/enabled:hover:border-primary!/knob 系/）。
- settings-wrapper .wk-tag 家族 5 条协调保留（7 个 settings tag span 追加编码生效值）。
- 钩子类全保留（测试选择器 0 改动）；验收：typecheck 0、settings 179/179 +
  integrations+Agents 43/43、web 890/890、build ✓。
### 批次43：page shell 家族 ✅（共享层最后大块，Orchestrator 验收提交）
- 29 个 apps/web TSX 消费方追加 utilities（保留全部钩子类名）+ views 3 处收尾
  （tool-result 27 span 按生效值补偿 font-mono!/0.8rem!/muted!、tool-approval gap 0.5rem、
  page.tsx 裸 wk-list-actions 补配方）；styles.css 删 25 条规则块 + 1 媒体片段。
- PlatformShell outlet 依赖（[&_.wk-page]:max-w-none!）验证仍在；测试选择器 0 改动。
- 合并验收：typecheck 0、web 890/890、shared 462/462、build ✓；styles.css 319 行
  （仅剩 wks-* 抽屉段/settings 保留段/chat 5 守卫/模型 tabs/注释）。
- 遗留产品决策项：chat ToolResultView 的补偿 span 原本被 .wk-list li span 压制，
  设计上若要恢复其自身样式可删补偿 utilities（1:1 原则下保持现状）。
### 批次44：wk-settings-* 保留段联动清除 ✅（子任务执行，Orchestrator 验收提交）
- 23 文件：styles.css 删 19 条（tabs/section/table/panel-heading/editor 全家 + 3 媒体片段）；
  chat.css 删 1 守卫（对手已死零漂移）；settings-wrapper.css 删 1 复位规则。
- KnowledgeSettingsPage Card 级任意变体编码（含 read-only/disabled/parser-table 8rem）；
  抽屉 DRAWER bundle（Mcp max-w-none! 保持）；configuration 域无色版 bundle；
  裸 h2/p/panel-heading/零散 label/input/select 十余处逐一补齐。
- 漂移说明 3 项已按「元素 utilities 胜出」约定接受并写入 css 注释。
- 验收：typecheck 0、248/248 相关测试、web 891/891、shared 465/465、build ✓；
  styles.css 315→299 行。
### 补充门禁验证 ✅（embed + desktop）
- embed：typecheck 0、test 7/7、build ✓（2200 modules）——packages/views 改动无回归。
- desktop：typecheck 0、test 2/2、build ✓——desktop-renderer 不受影响。
- 全门禁矩阵：web 891/891 + shared 465/465 + embed 7/7 + desktop 2/2 + typecheck 0 +
  build web/embed/desktop 全通过。
# 最终迁移报告（Tailwind CSS v4 + shadcn/ui）

## 迁移范围
- apps/web + packages/ui + packages/views 全部 Web UI 表面；不触及 Go 后端/Vue 前端/React Native。
- 基础设施：Tailwind v4.3.3 + @tailwindcss/vite；packages/ui/src/theme.css @theme 语义令牌
  （ink/canvas/surface/line 系/primary/accent/danger/muted 系/radius 系）；@source 跨包扫描
  （ui+views 源码类名）；styles.css @layer 组合（theme/base/components/utilities，无 preflight）。
- 组件体系（packages/ui）：Button/Card/Status/Input/NumberInput/Switch/Textarea/Select/Checkbox/
  Label/Badge/Alert/Separator/Table/Tabs(Radix)/Sheet(Radix)/DropdownMenu(Radix)/Tooltip(Radix)/
  Dialog(手写 DOM 契约 + Tab 焦点陷阱)/cn 工具/theme.css 导出。cva variants 管理尺寸与状态。

## 量化结果
- 遗留 CSS：12.3k 行 → styles.css 296 + settings-wrapper 122 + 8 个域 keyframes 小文件
  （auth 5/faq 11/knowledge-list 11/organizations 15/sandbox 44/skill 9/agents 62/chat 145/
  guides 243 保留）≈ 960 行，**97.8% 减量**。
- 45+ 个域批次全部提交；每批独立门禁（typecheck/test/build）+ 高风险批截图对比。

## 保留 CSS 及原因（全部已记录在对应文件头/台账）
- guides.css 243 行：III.7 几何驱动引导层（动态计算坐标，无法 utilities 表达）。
- chat.css 144 行：3 keyframes + 富文本/运行时 DOM 36 条（markdown.ts/mermaid.ts 生成）+
  5 守卫（4 条对抗共享层规则——其中 list-actions/panel-heading 2 条的对手已随批次 44 删除，
  待最终联动删；1 条 a11y reduced-motion）。
- settings-wrapper.css 122 行：抽屉级 select chrome + section-header/setting-row/setting-control
  级联作用域（SettingsPage wrapper 钩子类名依赖，消费方清单在文件头）。
- styles.css 296 行：wks-mcp-drawer 抽屉段 + wk-model-tabs（沙盒波联动）+ tombstone 注释。
- packages/ui/src/styles.css：legacy --wk-* 变量桥接（独立包兼容，fallback=令牌值）。

## 验证结果
- 门禁矩阵：typecheck web/shared/embed/desktop 全 0；web 891 / shared 465 / embed 7 /
  desktop 2 测试全绿；build web/embed/desktop 全通过。
- 截图对比：21 路由基线 + auth/login、chat-empty/chat-session（0.000% diff）、
  foundation 21 张等多组 after 对比；artifacts/tailwind-shadcn/。
- 真实浏览器操作：登录/知识库/文档上传/聊天流式/工具审批/设置抽屉等关键流程在
  vite:5181+server:8080 实栈验证（uimig@local.dev）。

## 已知问题与未完成项
1. 既有偶发白屏：chat 会话 in-place 切换 + continue-stream resume 竞态（迁移前即存在，
   非本迁移引入；before 态同样复现）。
2. chat ToolResultView span 补偿 utilities（font-mono!/0.8rem!/muted!）：按 1:1 生效值编码；
   设计上若要恢复其自身样式可删（产品决策项）。
3. settings-wrapper select chrome ~20 条永久保留（或后续逐面板收编，产品决策项）。
4. wk-model-tabs 3 条待沙盒面板 utilities 化后联动删除。
5. mobile 端Localization 并行任务仍在进行（与本迁移无冲突，路径隔离）。

## 依赖审计
- apps/web：tailwindcss@4.3.3 + @tailwindcss/vite（dev）——无多余依赖。
- packages/ui：@radix-ui dialog/dropdown-menu/tabs/tooltip + cva + clsx + tailwind-merge——全部在用。
- 无需删除的遗留依赖（wk-* 体系无 npm 依赖）。

## 方法论沉淀（docs/plans/tailwind-shadcn-conventions.md）
- 分阶段替换引擎（staged from/to 对 + 钩子类保留）；「按生效值编码 + ! 任意值」处理
  unlayered 竞争；静态条件替代动态插值任意值；ui bundle 级联发现；产物 CSS 探针断言；
  零漂移像素对比流程。
### 批次19：TenantMembers 域 ✅（子任务执行，Orchestrator 验收提交）
- TenantMembersPanel.tsx ~95 条规则内联 utilities（表格/分页/标签三态/确认弹层；
  settings-wrapper 抽屉 select chrome 特异性更高今日实际生效，按生效值 4 条未复制）；
  TenantMembersPanel.css 删除（752 行）；钩子类保留 13 个；测试选择器 15 处语义化。
- 验收：typecheck 0、面板测试 9/9、web 856/856、build ✓、dist 无 .wk-tenant-members 残留。
### 批次22-23：settings-wrapper model/ollama/segmented/field-error 家族 + integrations api-band 切片 ✅（双子任务并行，Orchestrator 合并验收提交）
- 批次22（model/ollama/字段族）：settings-wrapper.css 删 58 条（223→139）；8 文件转换
  （ModelSettings/OllamaUI/model-card 全家族 5 色徽章静态映射/ollama combobox/segmented/
  field-error×8/password-editor/builtin-hint/test-trigger/tabs 跨文件消费方 grep 定位）；
  测试 27 处+helper 语义化；六面板测试 83/83。
- 批次23（integrations api-band）：page.tsx + ApiPlaygroundDrawer.tsx 全迁移（inline 驱动
  属性保持 inline 只转实际生效 CSS；status 四色静态三元；var(--wk-*) 任意值保留）；
  styles.css 删 41 规则+1 死 media、缩窄保留 2 条共享规则；测试 13 处语义化、52/52。
- 合并验收：typecheck 0、web 856/856、shared 445/445、build ✓、styles.css 1087→1013 行。
### 批次14：@source 路径修复 ✅（shell 波发现的关键既有 bug）
- styles.css @source 路径 ../../packages/... → ../../../packages/...（apps/web/src 需三级）。
  oxide Scanner 实证：原路径 normalize 到不存在的 apps/packages/ui，包文件 0 被扫描。
- 修复后 dist 验证：px-[0.85rem]/py-[0.45rem]/rounded-control/bg-accent/text-ink/h-8、
  [&_kbd] 变体、max-[1024px]/max-[768px]/max-md/min-[900px] 断点、[&_.wk-page] outlet
  覆盖（max-w-none!）全部生成。
- 门禁：typecheck 0、web 856/856、shared 445/445、build ✓；全站截图 diff 无意外漂移
  （login/register SIZE-DIFF 为轮播 4s 时序；其余 0.03%~3.9% 为转换预期差异）。
- 注意：此修复使 Button/Input/Switch 等既有 ui 组件的 utilities 首次真实生效
  （此前仅 .wk-* 兜底 CSS 生效），视觉经截图复核无破坏。
### 批次14-16：agents / documents / skill-settings 三域 ✅（子任务并行，Orchestrator 验收提交）
- agents：AgentsPage ~60 处 + AgentEditorModal 75 块全转（--td-* 53 处任意值保留）；
  agents.css 759→62（保留 3 条跨文件 section-header 规则 + 2 keyframes）；agent-editor.css 删除。
- documents：documents.css 删除（1288 行、228 规则块、6 组 @media 全转前缀）；
  3 个组件文件全迁移；钩子类保留（测试类串断言零改动）；动态类陷阱修复 1 处
  （[transform:translateX(${…}px)] 插值不被扫描器生成 → 静态条件选择）。
- skill-settings：SkillSettingsPanel 全迁移；css 568→9 行（保留 skill-chip-dot keyframes）；
  主题变量任意值保留（--wks-primary/--wk-dialog-width 等 + ! 压制）。
- 验收：typecheck 0、web 856/856（agents 16+52、documents 121、skill 31 各域测试全绿）、build ✓。
### 批次12：faq 域 ✅（子任务执行，Orchestrator 独立验收）
- FAQPage.tsx + faq.css（1420→8 行 keyframes-only）+ faq-search-drawer/tag-tooltip 测试同步。
- 验收：typecheck 0、faq 测试 53/53、web 856/856、build ✓。
### 批次13：shell 域 ✅（子任务执行，Orchestrator 独立验收）
- PlatformShell.tsx 转换；shell.css 删除（582 行）；session-sidebar.tsx（views/chat）随迁
  ——shell.css 原含 chat 侧栏规则，级联来源分析已注释在文件头；platform 4 个测试文件同步。
- 验收：typecheck 0、platform 测试全绿、web 856/856、build ✓。
### 批次11：organizations 域 ✅（子任务执行，Orchestrator 验收）
- OrganizationsPage.tsx 165 条规则全部 utilities 化（25 个配方常量）；organizations.css 1236→15 行
  （保留 orgContentFadeIn/orgSkelPulse 两条 keyframes）；测试选择器 19 处语义化；
  保留钩子：is-active（rail 选中态）。
- 验收（独立复核）：organizations 测试 26/26、build ✓；dist CSS 抽查断点/任意值均已生成。
### 批次10：SystemAuditLog 面板切片 ✅（settings-wrapper 域 4/N）
- SystemAuditLogPanel.tsx 12 处 className 迁移（刷新按钮+spin、表格变体、time/actor/target 列、
  tag 三态色映射注入 AUDIT_TAG_TONES、load-more、详情抽屉+head+section/dl/pre 子元素规则）。
- settings-wrapper.css 删除约 25 行 audit 规则；@keyframes wk-audit-spin 随规则一并移除
  （spin 已由 animate-[...] 任意值引用，keyframes 保留在 wrapper css 头部注释说明处——确认见下批）。
### 批次9：PlatformApiKeys 面板切片 ✅（settings-wrapper 域 3/N）
- PlatformApiKeysPanel.tsx 7 处 className 迁移（create 行+input/button、capabilities、
  message、token 卡+code、表格变体、revoke 按钮）。
- settings-wrapper.css 删除 13 行 api-key 规则（家族清零）。
- 验收：typecheck 0、SettingsPage.test 15/15、web 856/856、build ✓。
### 批次8：RuntimeQueues 面板切片 ✅（settings-wrapper 域 2/N）
- RuntimeQueuesPanel.tsx 16 处 className 迁移（skeleton/指标网格+条件色/pools/models/
  表格+thead/tbody 变体/任务抽屉/head；[&_h3] 标题规则、[&_th/_td] 表格规则以任意变体表达）。
- settings-wrapper.css 删除 30 行 RQ 规则（家族清零）；720px 媒体查询 → max-[720px]: 前缀。
- 验收：typecheck 0、SettingsPage.test 15/15、web 856/856、build ✓。
### 批次7：SystemGlobal 面板切片 ✅（settings-wrapper 域 1/N）
- SystemGlobalSettingsPanel.tsx 12 处 className 迁移（tabs+is-active/rows/row/info/meta/
  control+input-select 变体/reset/message/confirm 全套）；类名保留为测试钩子。
- settings-wrapper.css 删除 18 条面板私有规则（wk-system-global 家族清零）。
- 验收：typecheck 0、web 856/856（SettingsPage.test 15/15）、build ✓。
### 批次6：guides 域 ✅（判定为保留 CSS，按目标 III.7）
- packages/views/src/guides/guides.css（243 行）保留原样，原因：
  ① spotlight 引导层为几何驱动定位（spot/ring/backdrop 由 JS 计算坐标、
  transition 跟随），非静态布局；② 入场动画 + 5 组 cubic-bezier 过渡 +
  prefers-reduced-motion 式无障碍考量；③ var(--td-*) TDesign 主题变量带
  亮色回退，属既有主题能力；④ 媒体查询含 !important 覆盖（utilities 表达脆弱）。
  工具类化收益低、回归风险高（Vue parity 逐元素一致）。
- 验收：无需转换；views 层无改动，embed 门禁不受影响。
### 批次5：command-palette 域 ✅（Orchestrator 执行）
- GlobalCommandPalette.tsx 23 处 className 全部 utilities 化（overlay/面板/输入行/结果组/
  分组头/条目+选中态/快捷键徽章 kbd 变体/空态/footer）；cmdk 类名保留为测试 DOM 钩子
  （cmdk/cmdk__input/cmdk__item/cmdk__item-shortcut，无样式规则）。
- command-palette.css 删除（181 行，全部可 1:1 表达，无 keyframes）。
- 验收：typecheck 0、web 856/856（含 shortcuts/render 38 项）、build ✓。
### 批次4：knowledge-list 域 ✅（Orchestrator 执行，分阶段原子替换）
- App.tsx KnowledgeBasesPage 全部 kb-list-* 样式 → utilities（3 阶段：结构层 21 处、
  网格/骨架/空态/section 头 19 处、卡片层 20 处）；类名保留为 DOM/测试解剖钩子（无样式规则）。
- knowledge-list.css 823 行 → keyframes-only（kbListFadeIn/kbListFlash/kbListShimmer）。
- 验收：typecheck 0、web 856/856、shared 445/445、build ✓、anatomy 16/16、
  kb-list 截图复核（rail/标题/warning/卡片全正常；像素差 1.5% 为转换预期差异）。
- 事故记录：与并行子任务并发写 App.tsx 导致截断一次，git checkout 恢复后重放三段转换；
  该教训已固化：同一文件禁止双写，子任务仅用于无冲突域。
### 批次3：memory-workspace 域 ✅（Orchestrator 自做）
- PersonalMemoryPanel.tsx 全部样式 → utilities（header/intro/group/rows/stacked row/textarea、
  720px 媒体查询 → max-[720px]: 前缀、focus color-mix → outline rgba）；
  memory-workspace.css 删除（153 行，全部可 1:1 表达，无 keyframes）。
- wk-segmented 保留（settings-wrapper.css 所有，后续批次处理）。
- 验收：typecheck 0、web 856/856、build ✓、settings-memory 截图复核（intro 盒/行布局/开关正常）。
### 批次4（进行中）：knowledge-list 域 —— Orchestrator 亲自执行
- 侦查完备：css 823 行 60+ 类全读；App.tsx 用点在 L652-895（67 处）；
  kb-list-anatomy.test.tsx 约 20 处 .kb-list-* 选择器待改。
- 分片计划：①结构层（page frame/container/header/rail/warning/grid/skeleton/empty）
  ②卡片层（card/variants/favorite/more-menu/badges/org-chip/flash）③测试选择器+css 缩减为
  3 条 keyframes（kbListFadeIn/kbListFlash/kbListShimmer）。注意 hover 链（card:hover .more、
  .kb-favorite-star）与 ::after 角标渐变用 group/ arbitrary variant 表达。
- 其余并行子任务（shell/faq/organizations）继续后台执行，完成即验收。
### 批次2：sandbox 域 ✅（子任务执行，Orchestrator 验收）
- SandboxSettingsPanel.tsx 7 处 className 迁移；sandbox-settings.css 106→44 行。
- 删除 16 条规则全转 utilities；保留 6 条（details/summary 子选择器 4 条、
  @keyframes+prefers-reduced-motion 一对）原因已写入文件头注释。
  var(--wks-*) 11 处按规则转任意值保留；DOM 钩子类保留（inventory overlay/drawer）。
- 验收：typecheck 0、面板测试 28/28、build ✓、settings-sandbox 截图人工复核
  （开关绿色 on 态、policy 行、hint、tabs、空态全部正常）。
### 批次1：auth 域 ✅（Orchestrator 自做，子任务环境无法写文件已回退）
- LoginPage.tsx 全部样式 → utilities；.language-switch 保留为 closest() DOM 钩子；
  auth.css 缩减为 2 条 @keyframes（nodePulse/lineFlow，复杂动画按约定保留）；
  login-page.test.tsx 2 处选择器改 type=submit / data-testid（toast 增 data-testid）。
- 验证：typecheck 0、web 859/859、build ✓、login/register 截图与 computed-style 复核（渐变/动画/表单/语言菜单正常）；后续修复表单卡片 box-sizing、创建账户绿色描边、注册必填标记、标题字重与返回链接语义，提交 `bf091984`、`71f7f73e`、`d9149c5b`、`7c4628a5`。
  login/register/not-found 的像素差异为轮播 4s 自动播放时序非确定（后续批次对比时将冻结时钟）。

- [x] 基础设施：theme.css 语义令牌（packages/ui/src/theme.css）+ styles.css 入口
  （theme/utilities layer + @source 扫描 ui/views）+ ui 包导出 theme.css。
  构建产物已验证：包内 utilities（如 .h-8）开始生成（修复了基线记录的既有缺陷）。
- [x] 基础组件一批：button.tsx（cva：default/primary/text/danger + loading 兼容）、
  Card/Status/Input/NumberInput/Switch 令牌化（视觉=各自既有生效样式）。
- [x] 门禁：shared 445、web 856、typecheck 0、build ✓（web 1 处失败为 70e9f061 i18n
  sweep 与 upload-dialog 旧断言的漂移，已随 77202404 修复；同提交修复了该提交截断的
  KnowledgeSettingsPage.tsx 尾部）。
- [x] 截图对比一批（baseline vs after-foundation，21 张）：差异集中在 Button 实际生效、
  i18n sweep 文案与 kb-settings 修复；无布局/配色漂移。artifacts/tailwind-shadcn/。
- 共享树注意：该 worktree 有并行任务在改 i18n/KB-settings（70e9f061、02a938ee）；
  本迁移坚持路径隔离，不动 i18n 与他人文件；截图账号 uimig@local.dev。

## 页面批次（每批：转换 → 门禁 → 截图对比）

1. styles.css 共享层 + App.tsx（KB 列表）+ NotFoundPage  ← 共享类规则在最后一个使用方迁移后删除
2. auth.css（login/register/join/onboarding）
3. platform/shell.css + command-palette.css（外壳导航/命令面板）
4. knowledge-list.css
5. settings/ 6 个 css + views/settings 面板
6. documents/documents.css
7. agents/agents.css + agent-editor.css
8. organizations/organizations.css
9. faq/faq.css
10. chat/chat.css（最长；富文本/markdown 内容区如需保留必要 CSS，按 III.7 说明保留）
11. views/guides/guides.css（补跑 embed 门禁）
12. 清理：删除已迁移 css；wk-* 从 TSX/tests 移除（316 处测试选择器随批更新）；
    移除不再使用的依赖；抽查 dist。

## 验收标准

- 每批后：test:shared ≥445、test:web ≥856（选择器更新计数如实记录）、typecheck:web 0、build 通过。
- 关键路由截图对比：同视口/同语言/同数据（uimig@local.dev，1440px + 390px），归档
  artifacts/tailwind-shadcn/after-<batch>/。
- 最终：embed + desktop 门禁补跑；views 改动导致 embed 截图抽查。
- 环境受限项如实标注 blocked-env（当前无；本地栈可用）。

## styles.css 剩余族清单（自动生成）

- .wk-integration-drawer (21)
- .wk-chat-message-content (11)
- .wk-chat-artifacts (6)
- .wk-settings-section (5)
- .wk-settings-nav (5)
- .wks-mcp-drawer (5)
- .wk-chat-sidebar (5)
- .wk-chat-artifact-preview (5)
- .wk-chat-reference (5)
- .wk-integration-form (5)
- .wk-form (4)
- .wk-list-actions (4)
- .wk-diff (4)
- .wk-preview-result (4)
- .wk-organization-layout (4)
- .wk-model-debug-result (4)
- .wk-settings-values (4)
- .wk-switch (4)
- .wks-nav (4)
- .wks-content (4)
- .wk-list (3)
- .wk-toolbar (3)
- .wk-settings-panel-heading (3)
- .wk-settings-editor (3)
- .wk-model-tabs (3)
- .wk-chat-messages (3)
- .wk-chat-tool-result (3)
- .wk-chat-references (3)
- .wk-chat-suggestions-grid (3)
- .wk-integrations-tabs (3)
- .wk-integration-drawer-close (3)
- .wk-embed-preview-chrome (3)
- .wk-secret-output (3)
- .wk-code-toolbar (3)
- .wk-capability-card (3)
- .wks-nav-item (3)
- .wk-document-link (2)
- .wk-settings-tab (2)
- .wk-settings-table (2)
- .wk-organization-select (2)
- .wk-model-card (2)
- .wk-model-editor (2)
- .wk-model-debug (2)
- .wk-chat-session-row (2)
- .wk-chat-reference-ids (2)
- .wk-integrations-header (2)
- .wk-int-section-header (2)
- .wk-int-doc-link (2)
- .wk-button (2)
- .wk-button--text (2)
- .wk-switch-knob (2)
- .wk-dialog--tenant-create (2)
- .wk-embed-preview-header (2)
- .wk-resource-checkbox (2)
- .wk-resource-check (2)
- .wk-kb-card (2)
- .wk-kb-empty (2)
- .wk-kb-section-toggle (2)
- .wks-close (2)
- .wks-reload (2)
- .wk-page (1)
- .wk-header (1)
- .wk-eyebrow (1)
- .wk-muted (1)
- .wk-debug (1)
- .wk-list-item-copy (1)
- .wk-list-item-actions (1)
- .wk-pagination (1)
- .wk-diff-add (1)
- .wk-diff-del (1)
- .wk-diff-same (1)
- .wk-settings-tabs (1)
- .wk-form-grid (1)
- .wk-form-grid--three (1)
- .wk-form-grid--two (1)
- .wk-preview-box (1)
- .wk-settings-save (1)
- .wk-organizations-page (1)
- .wk-organization-detail (1)
- .wk-settings-page (1)
- .wk-settings-layout (1)
- .wk-settings-operation-list (1)
- .wk-settings-read-note (1)
- .wk-model-settings (1)
- .wk-model-grid (1)
- .wk-model-card-header (1)
- .wk-model-type (1)
- .wk-model-editor-overlay (1)
- .wk-form-label (1)
- .wk-visually-hidden (1)
- .wk-chat-page (1)
- .wk-chat-sidebar-heading (1)
- .wk-chat-session-group (1)
- .wk-chat-main (1)
- .wk-chat-message-scroll (1)
- .wk-chat-artifact-preview-text (1)
- .wk-chat-artifact-preview-markdown (1)
- .wk-chat-artifact-preview-image (1)
- .wk-chat-artifact-preview-pdf (1)
- .wk-chat-reference-title (1)
- .wk-chat-suggestions (1)
- .wk-integrations-page (1)
- .wk-integrations-panel (1)
- .wk-role-badge (1)
- .wk-int-section-desc (1)
- .wk-int-doc-icon (1)
- .wk-button--danger (1)
- .wk-tag (1)
- .wk-tag--warning (1)
- .wk-form-actions (1)
- .wk-status (1)
- .wk-status-error (1)
- .wk-status-ok (1)
- .wk-mono-input (1)
- .wk-option-chips (1)
- .wk-option-chip (1)
- .wk-option-chip--active (1)
- .wk-check-row (1)
- .wk-muted--warn (1)
- .wk-integration-drawer-overlay (1)
- .wk-dialog-title-row (1)
- .wk-dialog-title-icon (1)
- .wk-embed-preview-overlay (1)
- .wk-embed-preview-drawer (1)
- .wk-embed-preview-body (1)
- .wk-embed-preview-hint (1)
- .wk-embed-preview-device (1)
- .wk-embed-preview-screen (1)
- .wk-embed-preview-mock-page (1)
- .wk-embed-preview-widget-panel (1)
- .wk-embed-preview-launcher (1)
- .wk-int-landing (1)
- .wk-int-landing-cta (1)
- .wk-capability-grid (1)
- .wk-kb-scope (1)
- .wk-kb-scope-tab (1)
- .wk-kb-scope-tab-active (1)
- .wk-kb-grid (1)
- .wk-kb-card-warning (1)
- .wk-kb-card-head (1)
- .wk-kb-card-title (1)
- .wk-kb-star (1)
- .wk-kb-star-active (1)
- .wk-kb-card-desc (1)
- .wk-kb-badges (1)
- .wk-kb-badge (1)
- .wk-kb-badge-wiki (1)
- .wk-kb-badge-warning (1)
- .wk-kb-card-actions (1)
- .wk-kb-skeleton (1)
- .wk-kb-sections (1)
- .wks-overlay (1)
- .wks-modal (1)
- .wks-container (1)
- .wks-sidebar (1)
- .wks-sidebar-header (1)
- .wks-sidebar-title (1)
- .wks-nav-group-title (1)
- .wks-nav-icon (1)
- .wks-nav-label (1)
- .wks-content-wrapper (1)
- .wks-section (1)
- .wks-role-denied (1)
- .wk-kb-card-wrap (1)
