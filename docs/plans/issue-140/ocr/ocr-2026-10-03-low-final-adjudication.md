# 2026-10-03 OCR Low 终局裁定（CAREER-OCR-GATE L）

## 范围与方法
- 源报告 A：`ocr-2026-10-03-fullrange-final.md`（标称 336 low；按 `─── file:lines ───`+severity 配对解析得 335，重复键 ['apps/miniprogram/src/career/progress-preparation.tsx:137-139', 'apps/web/src/agents/AgentsPage.tsx:255-259', 'apps/web/src/commercial/surface.tsx:10-13', 'apps/web/src/settings/SettingsPage.tsx:113-113', 'apps/web/src/settings/TenantMembersPanel.tsx:770-773']）。
- 源报告 B：`ocr-2026-10-03-fullrange-partial.md`（244 low）。任务书所指 high-medium-digest.md 实无 Low 条目（其 133 条均为 H/M），第二来源以 partial 报告 Low 段为准。
- 去重：file:line-range 键并集 → **338 条 unique low**（A∩B=241，A 独有 94，B 独有 3；双源同键以 final 文本为准）。
- 裁定基线：main @ 12b95330a（已含全部 CAREER-OCR H/M 修复波，24 commit / 67 文件）。
- 方法：8 个只读子代理按文件域（career/mobile/webmisc/agentswiki/settings/weborg/backend/tail）逐条对 worktree 核证 file:line 证据；主会话对 fixed-on-main/fp/trivial-fix 抽查复核（f06ee3ebc6、72e36c586、common.close 键、color:color: 修复等均实证成立）。

## 计数汇总
| class | n | 说明 |
|---|---|---|
| fell-with-fix | 1 | 随 H/M 波消失 |
| fixed-on-main | 11 | 非波 commit 已独立修复 |
| trivial-fix | 73 | **本轮应用 15**（§应用清单），递延 58 并入 backlog |
| accept | 41 | 有意模式/纯外观/无行为影响 |
| follow-up | 192 | 真实但非平凡，注册 backlog |
| fp | 20 | 报告主张不成立 |
| 合计 | 338 | |

follow-up backlog 总量 = 192 + 58（递延 trivial）= **250 项**。

## trivial-fix 应用清单（15，随域批次 commit）（15）
按域批次提交：agents(71,75) / wiki+knowledge(213,215,287,288) / auth(97,98) / organizations(224) / settings(244,253) / mobile(31) / embed(5) / web css(61,235)。全部为死代码/死声明/死导入/死类名/格式残留删除或键名更正，零行为语义变化（common.close 为既有 i18n 键更正，无测试断言旧键）。

| # | 位置 | 证据/理由 |
|---|---|---|
| 5 | apps/embed/src/button.tsx:1 | React 标识符全文 0 处使用（grep 仅行1 import 与 ReactNode），apps/embed/tsconfig.json:19 jsx=react-jsx，且为 apps/embed 唯一 import React 文件——死 import |
| 31 | apps/miniprogram/src/career/rules-usage-reminders.tsx:2 | `import Taro from '@tarojs/taro'` 全文件无 `Taro.` 使用（grep 实证），死 import |
| 61 | apps/web/src/administration/administration-u.css:18 | L18 font-weight:400 被同块 L24 font-weight:inherit 覆盖，死声明 |
| 71 | apps/web/src/agents/AgentEditorModal.tsx:92 | agents-u.css 全文件 3 行全注释零规则；全仓唯一导入点即本行，删导入+删文件零行为差。 |
| 75 | apps/web/src/agents/AgentsPage.tsx:556 | aria-label 用 general.close（全语种值=「关闭设置/Close Settings」，settings.ts 实证）；同域先例均用 common.close（「关闭」）；AgentsPage.test.tsx 无断言。 |
| 97 | apps/web/src/auth/login.td.css:223-227 | :224 margin-top:0 被 :227 margin 简写覆盖（top 同为 0），死声明，删除零行为 |
| 98 | apps/web/src/auth/login.td.css:43-47 | :44 transition-property:transform 被 :47 同规则 transition-property 覆盖，死声明，删除零行为 |
| 213 | apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:750(漂移) | `border-color: none` 非法值被浏览器整条丢弃（.kb-text-input 自身 border:0，本就无渲染效果），删除零渲染差。 |
| 215 | apps/web/src/knowledge/KnowledgeGraphPage.tsx:712 | `className=" wk-graph-arrow-stroke wk-kg-4"` 前导空格为迁移残留，去除纯格式。 |
| 224 | apps/web/src/organizations/OrganizationsPage.tsx:21-22 | :14 裸 FormEvent 死导入属实（:736/:755/:957 均 React.FormEvent），可单行删除；双别名 import 合并属后续重构 |
| 235 | apps/web/src/platform/platform-u.css:387 | 150ms 被 L388 100ms 覆盖，死声明（wk-cmdk-36） |
| 244 | apps/web/src/settings/ModelSelector.tsx:134 | :134 className="add-model-option" 全仓（css/tsx/test）零规则零引用，样式实际由内部 .model-option.add 承载 |
| 253 | apps/web/src/settings/RuntimeQueuesPanel.tsx:484 | ClockIcon 仅 :484-486 定义、零引用（TIcon name="time" 替换后残留）；ErrorIcon :235 仍在用需保留 |
| 287 | apps/web/src/wiki/wiki-u.css:190 | 190 `font-size: 13px;` 被 192 `font-size: inherit;` 同规则后置覆盖（级联本就 inherit 胜出），死声明删除零渲染差。 |
| 288 | apps/web/src/wiki/wiki-u.css:640 | 640 `font-weight: inherit;` 随即被 641 `font-weight: 650 !important` 覆盖，死声明删除零渲染差。 |

## fell-with-fix（1）

| # | 位置 | 证据/理由 |
|---|---|---|
| 174 | apps/web/src/documents/KnowledgeDocumentsPage.tsx:2769(漂移) | c6b4b5196（H2 波，"删除死常量"）git log -S 证实删除 STAGE_NOTICE_TONE_CLASS；现全文件 0 引用（仅剩 2790 注释指向 .wk-stage-notice）。 |

## fixed-on-main（11）

| # | 位置 | 证据/理由 |
|---|---|---|
| 25 | apps/miniprogram/src/career/progress-preparation.tsx:151-158 | 73f4f641a 已把 claims 取回纳入 try/catch：setReviseErrCode+草稿保留提示+looksOffline 断网标注 |
| 87 | apps/web/src/apps/AppsPages.tsx:227 | 559f210b6（非波 parity 收口 commit）已删除 TPopconfirm 的 placement="left"，恢复默认 |
| 89 | apps/web/src/apps/AppsPages.tsx:62 | HEAD 全文件已无本地 Tag/palette 三元与 wk-apps-tag 引用（559f210b6 TTag 收编移除）；残余 apps-u.css:205-209 .wk-apps-tag--* 五条死规则可随 [91] 一并清理 |
| 176 | apps/web/src/documents/KnowledgeDocumentsPage.tsx:600-622(漂移) | 559f210b6（非波 parity 修复）已移除 .more-wrap 手动 toggle：onClick 仅剩 stopPropagation，onVisibleChange 单通道（600-617 注释实证）；仅残留键盘 updater 内副作用为次级项。 |
| 205 | apps/web/src/integrations/views-integrations-u.css:39 | 891d0ab69（非波）已把 color: color:inherit 修正为 color: inherit（git -S 实证，L39/L64/L148 现均为合法写法）；波 cf1c5e529 未涉及 |
| 293 | internal/modules/career/career_export.go:1079-1081 | f06ee3ebc6（非波）已在 deletionRevokeExports L1101-1106 对 nil exportStorage 返 ErrExportStorageUnavailable、L1136-1138 对 nil releaser 报错，删除回执不再虚报完成 |
| 295 | internal/modules/career/career_export.go:625-627 | f06ee3ebc6 增 countErr 捕获（L634-637）并在 L668-670 整体返回错误，Count 失败不再静默归零 |
| 306 | internal/modules/codedelivery/dispatcher.go:242-245 | 全文件已无 "succeeded" 裸串，L246/L267 均用 appconnector.ActionSucceeded（25b32789e5 引入），编译期联动已恢复 |
| 307 | internal/modules/codedelivery/service.go:460-462 | L534-535 用 appconnector.ConnectionKindPersonal/ConnectionActive，L247 RiskDeliver、L259 ActionAwaitingApproval 同（5494127b0），裸词表前提不成立 |
| 331 | packages/views/src/chat/message-face.tsx:401-403 | L595 已传 {invalidImageLabel: props.copy.invalidImageLink}（25de61ec9），空 options 退中文占位的前提不成立 |
| 336 | packages/views/src/craft/td.tsx:107-109 | 该文件已被 72e36c586 删除（改用 TDesign：shell.tsx L100-105 closeOnEscKeydown），所述自绘 document keydown/stopPropagation 双层关闭代码不复存在 |

## accept（41）

| # | 位置 | 证据/理由 |
|---|---|---|
| 1 | .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:115 | 一次性 T33 闭证脚本未接 CI/门禁（全仓无引用）；断言值与本次造数同批内联即证据快照性质，finding 自认仅重放维护成本且路由已核对一致 |
| 2 | .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:18 | 行18现状属实，但系手动重放路径非门禁执行；未注入回落 9433、头部已给正确重放示例，坏值仅重放者可见；修复需加控制流非 trivial 类 |
| 3 | .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:53 | tapConsent 上方注释已锚定 T24 先例（t24r1-live-driver.cjs:161）并声明「回退页面唯一 checkbox」前提，登录页唯一性已核实成立；一次性脚本无需防御未来改版 |
| 4 | .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:64 | 实际 shot() 在行72：catch 有 log('shot-fail') 可查；退出码/RESULT 契约按断言步骤设计（ocr1-031 前轮已裁定口径），截图为报告附件非断言步骤 |
| 7 | apps/miniprogram/config/index.ts:48 | 自述"暂无现行缺陷"：全部 6 页仅用 button；闭包自动扫描属构建期增强，非现行行为问题 |
| 59 | apps/web/src/DevMarkdownPage.tsx:259 | dev-only parity 工具，L247-259 注释已详述后置改写机制，fixture 下行为正确；局限注记/快照属文档增强 |
| 64 | apps/web/src/agent-marketplace/am-u.css:126 | 字面量硬编码为 *-u.css 既定约定（finding 自认多数文件同此），单点改 var 反增不一致 |
| 85 | apps/web/src/analytics/analytics-u.css:313 | .wk-anl-an-usage-num 确拆两处（249/313），纯 CSS 组织偏好，无行为影响 |
| 96 | apps/web/src/auth/login.td.css:193-201 | 两块均 Vue 原样平移；921-930 末块(anim none!important + .animated-bg display:none)全遮蔽首块，无行为影响，注释化可选 |
| 99 | apps/web/src/auth/login.td.css:675-678 | 与 :654-657 已注解的 .t-form-item 同一"笔误类名原样平移不命中"惯例，保留系有意策略，无行为影响 |
| 100 | apps/web/src/career/ApplicationPage.tsx:122-126 | HEAD 同位置。剥离错配 application 参数防错误关联是有意行为（:112-113 注释），页面回退全新创建流（idle 表单可用），无数据丢失；加提示属 UX 增强。 |
| 103 | apps/web/src/career/ApplicationPage.tsx:248-248 | :248 三层三元为 career 目录统一既有模式（MaterialPage:534/PreparationPage:295 同款），纯可读性无行为影响；家族级重构另立。 |
| 105 | apps/web/src/career/CareerPage.tsx:179-179 | :180 错误标题嵌套三元 + :202 状态标签，纯样式一致模式，无行为影响；Record 查表重构归并到嵌套三元家族项。 |
| 106 | apps/web/src/career/CareerPage.tsx:209-209 | 全文件内联 style 为本页既有写法（TDesign 组件+内联布局），渲染行为正确；抽取 career.css 属重构级，与 138/141 家族一并考量（此处标 accept，重构并入 CSS 家族 backlog）。 |
| 113 | apps/web/src/career/ExportDeletionPage.tsx:385-385 | :407 receipt tone 嵌套三元，纯样式一致模式，无行为影响（文件顶部 :27 已有映射先例，重构归并家族项）。 |
| 114 | apps/web/src/career/InboxPage.tsx:208-209 | lookupReceipt(:247) 靠 writePhase 'busy' 置位后恢复按钮(:302-305)即卸载，离散事件间 React 同步重渲染封死连点；同族页同模式；即便并发，服务端按 requestId 幂等兜底。H10 波未触及此逻辑。 |
| 116 | apps/web/src/career/InboxPage.tsx:50-55 | :71-76 三层三元与 '/platform/career/search' 字面量：纯样式+常量收敛（routes.tsx 亦为字面量比对，全仓无 careerSearchPath 先例），无行为影响。 |
| 121 | apps/web/src/career/OpportunityPage.tsx:147-147 | :147 嵌套三元+双码重复文案，纯样式/可读性（行为正确），Record 查表重构归并嵌套三元家族项。 |
| 122 | apps/web/src/career/OpportunityPage.tsx:277-277 | :277 factAnchor 剥离中文确使锚点退化为 fact------rev，但档案 CAS（每次确认 revision+1，evaluation.go FactRevision=fact.Revision）保证一修订一事实，当前唯一性成立；encodeURIComponent 属防御性增强。 |
| 124 | apps/web/src/career/PreparationPage.tsx:295-295 | :295 修订状态三层三元，career 家族统一模式，纯样式无行为影响。 |
| 127 | apps/web/src/career/ProgressPage.tsx:27-27 | :27 两层三元 className，家族统一模式，纯样式无行为影响。 |
| 130 | apps/web/src/career/RulePage.tsx:299-301 | :363-365 enabled/paused/off 三分支链，纯样式可读性，无行为影响。 |
| 134 | apps/web/src/career/SearchPage.tsx:305-305 | :354 错误标题嵌套三元，家族统一模式，纯样式无行为影响。 |
| 135 | apps/web/src/career/SubmissionPage.tsx:176-178 | :177-178 occurredAtFromInput 调 4 次（纯函数无行为影响）+ selectedExport! 有 :171-172 守卫保护；局部变量化为可读性重构，非缺陷。 |
| 146 | apps/web/src/career/reconciliation.tsx:183-183 | :216 及多处 phase→文案链式三元，家族统一模式，纯样式无行为影响。 |
| 163 | apps/web/src/commercial/surface.tsx:18-18 | :18 三层三元 toneClass；tone 为四值闭合联合，可一行模板串折叠（等价重构），无行为影响——按家族模式处理，若做归入 162 重构顺带。 |
| 167 | apps/web/src/configuration/ui.tsx:12-15 | 文件头注释明示 playbook §1 有意保留域内原生封装+既有类名（utilities 逐字一致、视觉零变化），三处并存是迁移决策非漂移 |
| 195 | apps/web/src/faq/FAQPage.tsx:277 | Vue 事实源 FAQTagTooltip.vue:5-6 仅 mouseenter/mouseleave；onClick 兜底系 9ea6e6895 tdesign 迁移时对齐事实源删除 |
| 209 | apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:55 | 拆分导入上方注释明示「留守段旧栈」标记意图（S6 收编说明），纯组织性、无行为影响。 |
| 220 | apps/web/src/market/market-u.css:587 | 同文件单类顺序覆盖是 *-u.css 迁移的普遍既定模式（wk-mkt-38@587 后于 mk-tab@37），无行为影响 |
| 233 | apps/web/src/platform/PlatformShell.tsx:1187 | 全部现行条目要么在 NAV_ICON_URLS（creatChat/knowledge-bases/agents/organizations）要么带 iconNode（experts/market/analytics）；career 键不在 HEAD，src='' 路径不可达，类型收紧属防御增强 |
| 245 | apps/web/src/settings/ModelSettingsPanel.tsx:1058 | :1058 `loading={false}` 复刻 Vue t-loading DOM（恒 false 无行为影响）；接真实列表加载态属功能增强 |
| 258 | apps/web/src/settings/SettingsPage.tsx:592-598 | kbd-nav 焦点门控为既有机制且测试锚定（:333-338、SettingsPage.test.tsx:165-174），Tab 导航可见默认轮廓；仅 main→div landmark 语义弱化，无功能影响 |
| 270 | apps/web/src/settings/TenantUserProfileSections.tsx:321 | :321 '/login' 与 :308 80 阈值为硬编码，纯常量提取重构，无行为影响 |
| 284 | apps/web/src/wiki/WikiPage.tsx:1404(漂移) | `(value: unknown)=>String(value ?? "")` 与推断签名输出恒等（tdesign InputValue=string|number），纯风格统一，无行为差异。 |
| 285 | apps/web/src/wiki/WikiPage.tsx:1400,1408(漂移) | form onSubmit 与 onEnter 双提交路径属实，但 submitSearch 为幂等同步状态更新（finding 自认无用户可见 bug），冗余路径如要收敛列 backlog 即可。 |
| 290 | docs/design/job-search/prototype/app.js:131 | 全部 data-view 发射点（navButton 的 esc(n[0]) + applications/materials/profile）均在 navItems 四 id 内，无 undefined 解引用路径；自标 THROWAWAY 原型，新增视图漏更新属迭代可接受风险 |
| 329 | packages/views/src/chat/message-face.tsx:203-207 | L337-341 嵌套三元为纯样式约定问题，三分支文案输出行为不变 |
| 334 | packages/views/src/chat/tool-approval.tsx:190 | L190 嵌套三元纯样式，三态 className 映射行为不变 |
| 335 | packages/views/src/chat/tool-result.tsx:857 | L857 嵌套三元纯样式，statusKind 有限联合映射行为不变 |
| 337 | packages/views/src/integrations/page.tsx:1743 | L1950/L2011 color-mix 无 rgba 回退属实，但报告自认 2026 基线浏览器普遍支持、风险有限，无现行缺陷 |

## fp（误报，关闭）（20）

| # | 位置 | 证据/理由 |
|---|---|---|
| 8 | apps/miniprogram/src/adapters/career-platform.ts:357-359 | focus 全链为 PreparationFocus 枚举（progress-preparation.tsx:65,112,136），无自由输入路径；save/read/clear 均用同一原始值，键恒一致 |
| 39 | apps/miniprogram/src/services/career-intent.ts:106 | "无调用方传值"不成立：career.ts:216 confirmSharedImport 传 `reuseId: draft.requestId` |
| 58 | apps/web/package.json:9 | L9 为 `node --import tsx --test 'src/**/*'`，无 --test-concurrency；全仓 json grep 与 git log -S 均无该串，主张前提不存在 |
| 101 | apps/web/src/career/ApplicationPage.tsx:232 | clearPrivate(:81-86) 清 receipt 并置 phase=scope-changed；:247 该相位仅渲染告警，对账按钮(:260-261)随 receipt 卸载；表单不可达（canSubmit/渲染守卫），恢复必经重挂载→useState 重置 reconcileBusy。永久 disabled 不可达。 |
| 107 | apps/web/src/career/ExportDeletionPage.tsx:10-11 | protocol.ts:37 确定码列表为 forbidden/invalid_request/idempotency_conflict/request_too_large/PAYLOAD_TOO_LARGE/not_found/unauthorized——不含 revision_conflict；基础契约对该码返回 true(uncertain)，包装(:10-11)使其确定，非零效果，「逐输入等价」不成立。 |
| 129 | apps/web/src/career/RulePage.tsx:192-192 | send()(:217) 在发请求前即 localStorage.setItem(attemptKey,…,JSON.stringify(next))(:221)，存储失败直接中止写入；load()(:148-166) 读 pending 先 ruleReceipt 后同号 setRule 对账。创建路径与更新路径同等持久化，「不写 localStorage」前提不成立。 |
| 131 | apps/web/src/career/RulePage.tsx:330-330 | 后端入库前已白名单校验：search_once.go:571-577 非 http://或https://前缀、url.Parse 失败、无 Hostname 的链接一律 continue，不落库；todo.link(RulePage:398) 全部经此闸，javascript:/data: 不可达（前端白名单仅为纵深防御可选项）。 |
| 160 | apps/web/src/commercial/BillingPage.tsx:55-55 | 本 HEAD BillingPage.tsx:96-99 常量仍为 tailwind utilities（'w-full border-collapse…'），未改为 wk-bill-* 语义类；注释「（tailwind utilities）」如实。语料所述「已改为语义类名」的前提在本分支不成立（commercial-u.css 的 .wk-bill-* 规则存在但未被本页引用）。 |
| 190 | apps/web/src/faq/FAQPage.tsx:1653-1660 | 前提不成立：onOpenImport(:2526) 从不清 importTask（git -L 552f466c9/e95f9a1cb 均无 setImportTask(null)），弹窗开着 task 可非空，所指条件实际可达 |
| 194 | apps/web/src/faq/FAQPage.tsx:2489 | :2526 onOpenImport 无 setImportTask(null)，历史(552f466c9 引入→e95f9a1cb 仅加 setImportPreview)亦无；所指"清任务"代码不存在 |
| 217 | apps/web/src/main.tsx:20 | main.tsx 无 opportunity.css 引入（仅 L22 styles.css）；git -S 于 main.tsx 无记录。附注：opportunity.css 现全仓无人加载（仅测试 readFileSync），系相邻新问题非本 finding 主张|；替换此处竖线 |
| 230 | apps/web/src/platform/PlatformShell.tsx:1 | L1 为命名导入（useCallback…），全文件无 default React 也无 React. 值引用；git -S "import React" 无历史，死 import 不存在 |
| 231 | apps/web/src/platform/PlatformShell.tsx:1106 | HEAD 无 mobile-overlay/isNarrowViewport（aside 仅 collapsed 两态，td.css 亦无该类），所指 z-index/不收起缺口无载体 |
| 242 | apps/web/src/settings/McpSettingsPanel.tsx:1237-1242 | 现码 :1245/1257/1269 三处均带 type="number"；wk-mcp-number-input 类名从未存在于代码（git log -S 仅命中 docs），缩进也正常 |
| 250 | apps/web/src/settings/PlatformApiKeysPanel.tsx:5 | :5 是注释行；:9 为单条 tdesign-react import（Button as TButton），无未别名 Button 导入，按钮全用 TButton |
| 255 | apps/web/src/settings/SettingsPage.tsx:507 | shellFeedback 标识符在现码与全历史均不存在（grep、git log -S 皆空）；:553 确有嵌套三元但属 256/257 已覆盖范畴 |
| 261 | apps/web/src/settings/SystemGlobalSettingsPanel.tsx:6 | 主张不成立：未别名 Button :583、Input :483/572-578、Checkbox :697 均在用；仅 unaliased Switch(:8) 确为残留 |
| 262 | apps/web/src/settings/SystemGlobalSettingsPanel.tsx:10 | useRef 在 :383 用于 draftRef 并于 :480/:483 读写，非死导入 |
| 265 | apps/web/src/settings/TenantMembersPanel.tsx:4 | 现码仅 :8 一条 tdesign-react import，全文件 TDialog/Dialog 引用计数 0，「双 import」前提不存在 |
| 332 | packages/views/src/chat/message-list.tsx:377-378 | 手动按钮仍在：L378-384 渲染 wk-chat-load-older 且消费 t.loadingHistory/t.loadOlder（L381/384）；25de61ec9 "restore chat history loading action" 已恢复 |

## follow-up backlog（192，非平凡）（192）

| # | 位置 | 证据/理由 |
|---|---|---|
| 6 | apps/miniprogram/config/index.ts:19-20 | 属实：realpathSync 先于 existsSync，依赖缺失时抛裸 ENOENT；修复需重排+补检查（3-4 行构建脚本改动） |
| 9 | apps/miniprogram/src/career/application-material.tsx:115-120 | 静态错误后缀失实仍在（:120）；`void receipt;`(:118) 仍在；c1eb5ae7f 仅改 materialIdRef，未触及 |
| 10 | apps/miniprogram/src/career/application-material.tsx:185 | 三层链式三元仍在:185；改 Record 映射属多行重构 |
| 11 | apps/miniprogram/src/career/application-material.tsx:23-25 | tdesignButtonStyle×5/digestHead×3/typedCode×4（grep 实证）重复；抽共享模块为重构 |
| 12 | apps/miniprogram/src/career/application-material.tsx:306-309 | 共享 downloadBusy+checks 无上限仍在:306-308；需按 exportId+format 的状态模型改造 |
| 13 | apps/miniprogram/src/career/discovery.config.ts:8 | '/npm/tdesign/button/button' 实际硬编码 6 处（5 career config+artifact:7）；抽常量跨文件重构 |
| 14 | apps/miniprogram/src/career/discovery.tsx:115-117 | 属实：:115 仅 disabled={proposeBusy.busy}，career.ts:54 proposeFact 不校验空值；补 disabled 为行为修复 |
| 15 | apps/miniprogram/src/career/discovery.tsx:45 | message.includes 判定耦合仍在:45；改 code 判定需 useAction/errors.ts 联动改造 |
| 16 | apps/miniprogram/src/career/discovery.tsx:69 | 属实：t-button.d.ts 全仓唯一声明在 src/subpackages/execution/artifact/；迁至 src 顶层属文件移动 |
| 17 | apps/miniprogram/src/career/discovery.tsx:89-96 | chooseMessageFile 取消 reject 无 code → 兜底文案（files.ts:47,errors.ts:19）；需 try/catch 识别 cancel |
| 18 | apps/miniprogram/src/career/discovery.tsx:101-108 | 重发不校验文件一致性属实；涉产品文案与错误码设计 |
| 19 | apps/miniprogram/src/career/export-deletion.tsx:124-141 | reconcileDeletion/retryDeletion 结构性双份属实；抽参数化包装为重构 |
| 20 | apps/miniprogram/src/career/export-deletion.tsx:47 | recoveryNotice 仅被覆盖不清除属实（:146,159,167,177,195,201,204 全为 set 非空）；清空时机需设计 |
| 21 | apps/miniprogram/src/career/export-deletion.tsx:159-165 | 四分支嵌套三元仍在:159-165；Record 化为重构 |
| 22 | apps/miniprogram/src/career/export-deletion.tsx:240+353 | partial 时恢复区(:240)与回执卡片(:353)双"重试删除"入口并存属实；渲染条件调整涉 Web 对齐语义 |
| 23 | apps/miniprogram/src/career/export-deletion.tsx:54 | `const [, setDelRecoveryUnresolved]`(:54) 写不读属实；删除需同步移 4 处 setter（:118,129,138,193,324），渲染等价性需验证 |
| 24 | apps/miniprogram/src/career/export-deletion.tsx:96-98 | clearCareerCaches 在 try 外属实:96；需补 try/catch+呈报分支 |
| 26 | apps/miniprogram/src/career/progress-preparation.tsx:197 | 属实：recPrepBusy 成功仅 setGenNotice 未 setGenErrCode(undefined)（:197），retryPrepBusy:202 有先例；单行行为修复 |
| 27 | apps/miniprogram/src/career/progress-preparation.tsx:304 | genErrCode 链式三元仍在:304；Record 化为重构 |
| 28 | apps/miniprogram/src/career/progress-preparation.tsx:310 | status 嵌套三元仍在:310；statusLabel 映射化为重构 |
| 29 | apps/miniprogram/src/career/rules-usage-reminders.tsx:248 | expectedRevision: revision! 仅靠 disabled(:247)属实；补回调内守卫为行为修复（两处一并见[32]） |
| 30 | apps/miniprogram/src/career/rules-usage-reminders.tsx:263-267 | 三分支嵌套三元仍在:263-267（:208,:245 同型）；抽辅助函数为重构 |
| 32 | apps/miniprogram/src/career/rules-usage-reminders.tsx:379 | createReminder 的 revision!(:379) 同[29]第二实例；guard 为行为修复 |
| 33 | apps/miniprogram/src/career/rules-usage-reminders.tsx:140 | usageErrCode 写(:171,172)不读属实（grep 全文仅声明+2 写）；但删除涉 3 个编辑点，非单行 |
| 34 | apps/miniprogram/src/career/rules-usage-reminders.tsx:157-158 | 渲染期直读 storage 属实（:157,176,239）；收敛为 state 属架构级 |
| 35 | apps/miniprogram/src/core/errors.ts:4-5 | 大写 ARTIFACT_GRANT_* 与后端小写 code 的跨文件契约确无注释；补 3 行注释块（非单行） |
| 36 | apps/miniprogram/src/platform/files.ts:34-36 | dot>0 使 '.pdf' 白名单通过(:53)但副本无扩展名属实；修 dot 边界+stem 限长为边界条件逻辑改动 |
| 37 | apps/miniprogram/src/platform/files.ts:44 | 用户可见错误拼 USER_DATA_PATH 绝对路径属实:44；改文案+code 为行为修复 |
| 38 | apps/miniprogram/src/platform/files.ts:69-71 | 嵌套三元仍在:69-71；改 if 为风格重构 |
| 40 | apps/miniprogram/src/services/career.ts:170-184 | retryPendingSearch 手写三件套仍在:174-183，与 recoverableWrite 判据并行演化属实；委托化重构 |
| 41 | apps/miniprogram/src/services/career.ts:367-373 | op 非 edit/confirm 静默落 confirm 分支属实:369-373；补形状守卫为行为修复 |
| 42 | apps/miniprogram/src/services/career.ts:49 | syncFromWeb 未 activateCareerScope 属实:49；现实路径难触发（useData 键含身份→先 loadCareer），加一行对齐为行为修复 |
| 43 | apps/miniprogram/src/subpackages/execution/artifact/index.tsx:48 | 无条件追加"授权过期…重新获取"属实:48（errors.ts:4 已含同义指引）；去后缀/条件渲染为文案行为改动 |
| 44 | apps/miniprogram/tests/application-material.test.mjs:61-85 | backend/freshLogin 与 progress-preparation.test.mjs 骨架逐字重复属实；抽 helpers 为测试重构 |
| 45 | apps/miniprogram/tests/assembly.test.mjs:237-243 | stub.use 不判 call.kind 属实:237-240（:264 同型）；补 kind 判定为测试行为改动 |
| 49 | apps/miniprogram/tests/export-deletion.test.mjs:615 | 断言手工重写 deletionUnknown 判定属实:615；改真实 controller 接线为测试重构 |
| 50 | apps/miniprogram/tests/helpers/taro-stub.mjs:101-104 | copyFile 先 push 后校验属实:103-104；改记录时机会变更 fixture 契约（copies.length 断言），需审计 |
| 51 | apps/miniprogram/tests/live/quota-inject-proxy.mjs:31-36 | 无 timeout/req close 监听/hop-by-hop 剥离属实:31-36；fd2b5e809 仅改注释未触及 |
| 52 | apps/miniprogram/tests/live/t24r1-live-driver.cjs:178 | api() 裸 JSON.parse 属实（:178,189,191）；补 status/上下文包装为驱动重构 |
| 53 | apps/miniprogram/tests/live/t24r1-live-driver.cjs:232-233 | 跨快照断言假失败风险属实:232-233；组合正则改造为测试语义改动 |
| 54 | apps/miniprogram/tests/live/t24r1-live-driver.cjs:28-31 | readFileSync(TOKA) 先于 env 校验属实:28→31；集中校验重排为多行改动 |
| 56 | apps/mobile/src/task-office-integration-smoke.ts:59-63 | 路径硬编码+fetch 无 signal 属实:59-63；路径常量化+超时为两项改动 |
| 57 | apps/mobile/src/task-office-integration-smoke.ts:136 | restoreFailed 恒 'failed' 属实:136（scopeStillMatches 已在:102 可用）；证据语义分型为行为修复 |
| 60 | apps/web/src/administration/AdministrationPage.tsx:206 | TInput 确无 inputMode（L206），但 tdesign 透传未验证（node_modules 缺失）；仓内 inputMode 先例均为原生 input |
| 65 | apps/web/src/agent-marketplace/am-u.css:98 | .wk-amr-14 确为 overflow-wrap:anywhere；改 word-break:break-all 是断行语义变更（影响普通词内断行），需对照 Vue 源视觉核对 |
| 66 | apps/web/src/agents/AgentEditorModal.tsx:1298,1753 | 两处 `href="javascript:void(0)"` 仍在（全仓仅此 2 处）；换 button/href="#" 涉 .go-settings-link 样式与 CSP 行为，非一行。 |
| 67 | apps/web/src/agents/AgentEditorModal.tsx:1913-1917 | Option 仍只渲染 provider.name，无 is_default 标记（接口 111 行有 is_default 字段）；恢复需多行 JSX 改写。 |
| 68 | apps/web/src/agents/AgentEditorModal.tsx:650 | 650 定时器空回调确系死代码，但删除涉 284/302/647/650 四处联动，超出一行安全修复。 |
| 69 | apps/web/src/agents/AgentEditorModal.tsx:844 | 844 `void navigator.clipboard?.writeText(editId)` 无 catch 属实；补静默或失败提示属 UX/i18n 决策。 |
| 70 | apps/web/src/agents/AgentEditorModal.tsx:1909 等 | 静态内联样式散布多处（如 1909 `style={{ width: '240px' }}`），下沉 agents.td.css 类为多点重构。 |
| 72 | apps/web/src/agents/AgentParserRules.tsx:177 | `<a className="go-settings">` 确无 href；改 `<button>` 需核对 .go-settings CSS 对 button 默认边框/背景的覆盖。 |
| 73 | apps/web/src/agents/AgentsPage.tsx:261-265 | ResourceOriginBadge 4 层嵌套三元仍在（波 fd9524f77 未触及此段）；映射对象重写非一行。 |
| 74 | apps/web/src/agents/AgentsPage.tsx:553 | 抽屉容器仍无 role="dialog"/aria-modal/aria-label（波 fd9524f77 未修复）；补属性属 DOM/行为变更。 |
| 76 | apps/web/src/agents/AgentsPage.tsx:785-795 | 创建按钮仅 Tooltip 无 aria-label 属实；补属性属行为变更（可与 74 一并做）。 |
| 78 | apps/web/src/agents/MbtiTestModal.tsx:124,159 等 | 6 处内联 error 色（Mbti 2+Persona 2+Subagents 2）确认存在；收敛 .wk-ae-error-text 需新增类+6 处替换。 |
| 79 | apps/web/src/agents/agents.td.css:3955/4147 等 | .wk-ae-mbti-question 3955(15px) 被 4147(13px) 覆盖、footer 3963/4144 双定义属实；合并需裁决生效值（border-top 去留）。 |
| 80 | apps/web/src/agents/agents.td.css:861-868,873+ | .agent-section-header 同名连续两块属实；合并为机械重构但多行搬运（含注释）。 |
| 83 | apps/web/src/analytics/AnalyticsPage.tsx:335 | wk-anl-1/2/3 内联占位类实证（335/339/459-466 十余处）；改名+收进 AN_* 常量为 TSX/CSS 联动重构 |
| 84 | apps/web/src/analytics/analytics-u.css:114 | L443 导出按钮确用 AN_BTN_OUTLINE+disabled={exporting} 而无 :disabled 规则；补规则属新增样式非死码清理 |
| 86 | apps/web/src/apps/AppsPages.tsx:218 | 链式嵌套三元与 maxHeight 520/360 字面量实证；抽 stateThemeOf/常量属重构 |
| 88 | apps/web/src/apps/AppsPages.tsx:249 | Catalog/Connections 确仅靠 TTable loading 遮罩无 role=status 播报；补 live region 属 a11y 增强 |
| 90 | apps/web/src/apps/apps-u.css:193 | wk-apps-26 与 .apps-view 同源双拷贝+多字面色令牌化，确证；跨文件去重属重构 |
| 91 | apps/web/src/apps/apps-u.css:5 | wk-apps-1~4 仅被无调用方的本地 Popconfirm（L51 定义，全文件零调用）引用；删组件+规则约 40 行死码，超一行粒度 |
| 93 | apps/web/src/auth/LoginPage.tsx:346 | 双源属实（:46-47 inline delay vs login.td.css .node-N/.line-N）；收敛单一事实源需先核对取值等价再删一侧 |
| 94 | apps/web/src/auth/LoginPage.tsx:413 | :410 分页 span 无 tabIndex/role/onKeyDown，键盘不可达属实；补键盘交互属行为修复，超出 typo/死码范围 |
| 95 | apps/web/src/auth/WorkspaceOnboardingPage.tsx:175 | :195-199 邀请弹窗 TDialog(=WkDialog，wk-legacy.tsx:35 默认'Close') 未传 closeLabel；i18n 键存在(index.ts:751)，补 prop 属行为修复 |
| 102 | apps/web/src/career/ApplicationPage.tsx:248-248 | startAnotherBatch(:234-238) 确未复位 progressOpen/submissionOpen/preparationOpen/careerMaterialId，新 receipt 后面板残留展开，与 :53-63 注释相悖；补三/四个 setter 属行为级复位语义，进 backlog。 |
| 104 | apps/web/src/career/CareerPage.tsx:12-12 | :12 makeId 与 protocol.ts:25 newRequestId 逐字符相同；CareerPage 未 import protocol.ts，替换需加 import+5 处调用点（:158/:189/:212×3），机械多站点去重进 backlog。 |
| 110 | apps/web/src/career/ExportDeletionPage.tsx:256-257 | acceptDeletion :276-277 将 status==='deleting' 归入 phase 'error'（:403 红色 alert 播报「删除仍在进行中」）；需引入中性展示相位，非一行改动，进 backlog。 |
| 111 | apps/web/src/career/ExportDeletionPage.tsx:339-339 | :359 文案承诺可重试，但重读按钮仅 :368/:404 conflict 态出现；导出(:362)/删除(:397) 因 revision===undefined 长期禁用，同挂载内无重读入口（recoverCurrentScope 仅 scope-changed 分支），确为死胡同；补入口属行为级。 |
| 112 | apps/web/src/career/ExportDeletionPage.tsx:348-348 | :368 重读仅清 exportConflict+readRevision，exportPhase 停 'error'、exportMessage 残留（:367 持续 alert）；需在 readRevision 成功回调清错误态，牵连删除侧同名按钮，进 backlog。 |
| 115 | apps/web/src/career/InboxPage.tsx:4-5 | :4-5 深相对路径导入属实（api-client index 未导出 Reminder 类型）；修复需先扩 packages/api-client/src/index.ts 导出再改约 20+ 文件，跨包重构进 backlog（与 137 合并）。 |
| 117 | apps/web/src/career/MaterialPage.tsx:115-115 | :109 每渲染双重 JSON.stringify 属实；键序误判为条件性风险（当前 TS 构造序与服务端 Go 序一致、confirm 可用），修法=useMemo+稳定序列化，进 backlog。H9 波未触及。 |
| 118 | apps/web/src/career/MaterialPage.tsx:249-250 | :244/:300 两处 catch 内裸 await reloadView，再抛将成 unhandled rejection（调用方 :562-563 void）；两处补 .catch 属行为语义选择（吞或提示），进 backlog。 |
| 120 | apps/web/src/career/MaterialPage.tsx:546-546 | :540 章节以 sectionIndex 作 key 属实；修复需 EditableSection 增设 sectionId（创建/恢复/迁移三处），类型级改动进 backlog。 |
| 123 | apps/web/src/career/PreparationPage.tsx:202-202 | 同一数组字面量在 PreparationPage:202/:282 + ProgressPage:137 三份 + protocol.ts:37 第四份属实；导出 definiteClientErrorCodes 收敛为跨文件重构进 backlog。 |
| 125 | apps/web/src/career/PreparationPage.tsx:311-311 | revisionState==='error' 时重读按钮仅 :311/:356 conflict 态渲染，refresh(:304) 只 reload 列表（readRevision useCallback 依赖稳定不重跑）；面板内死胡同属实（收起再展开可恢复），扩渲染条件属行为级进 backlog。 |
| 126 | apps/web/src/career/PreparationPage.tsx:357-357 | :357 以中文前缀 startsWith('修订已保存') 判定 role/样式，隐式契约属实（成功文案两处 :264/:268 需同步）；改显式 reviseOk 状态牵连多 setter，进 backlog。 |
| 128 | apps/web/src/career/RulePage.tsx:14-14 | :14 makeId 与 protocol.ts:25 逐字符相同属实，且 RulePage 尚有本地 errorDetails(:22)/isUncertainOutcome 未迁 protocol；整页收敛进 backlog（与 104 合并）。 |
| 133 | apps/web/src/career/SearchPage.tsx:147-152 | :109-114 与 :284-288 在 setState updater 内 writeHistory 属实（违反纯函数约定，StrictMode 双调用当前幂等）；改 ref 镜像模式为结构级改动，进 backlog。 |
| 137 | apps/web/src/career/UsagePanel.tsx:5-5 | :5 深相对导入属实，.wk-career-usage-wrap 无任何样式定义；修复前置条件=api-client index.ts 补导出，跨包重构进 backlog（与 115 合并）。 |
| 138 | apps/web/src/career/application.css:74-78 | #07c05f×5、警示橙与 #0052d9 回退未 token 化属实（tdesign-theme.css --td-brand-color=#07c05f、--wk-color-brand 同值）；三文件约百行重复亦属实——token 化+career-base.css 抽取为家族级重构进 backlog。 |
| 139 | apps/web/src/career/export-deletion.css:43-47 | :43-47 #07c05f+!important 属实；改 var(--td-brand-color,…) 并以 specificity 替代 !important 为家族级 token 化重构进 backlog（与 138/141 合并）。 |
| 140 | apps/web/src/career/inbox.css:46-46 | inbox.css #07c05f×5+reconciliation.css×6 字面量属实，其余属性已用 --td-* 唯品牌色硬编码；token 化归并 CSS 家族 backlog。 |
| 141 | apps/web/src/career/material.css:193-193 | :193 #07c05f 及警示色硬编码+三文件基础样式重复属实；token 化+career-base.css 家族级重构进 backlog（与 138 合并）。 |
| 144 | apps/web/src/career/reconciliation.css:123-125 | :123-125 启用态按钮常态绿边（inbox.css:46 为 hover 态）不一致+品牌绿字面量；:hover 意图无法考据，与 token 家族一并进 backlog。 |
| 145 | apps/web/src/career/reconciliation.tsx:104-104 | :136 同快照→setDiffPhase('idle')，渲染 :242-257 无 idle 分支落入「正在读取两个固定快照…」永久加载文案；两个 select 可选同快照用户可达。H11 波仅加 attempt 持久化未触及。补显式分支属行为级。 |
| 147 | apps/web/src/career/rule.css:12-12 | rule.css #07c05f×5（eyebrow :12/focus 描边/run-status/todos/basis）属实；token 化归并 CSS 家族 backlog。 |
| 149 | apps/web/src/career/submission.css:82-89 | 全文确无 :focus-visible（application/material.css 均有 3px outline），键盘焦点不一致属实；补规则为 a11y 增量改动进 backlog。 |
| 150 | apps/web/src/career/submission.css:92-92 | submission.css #07c05f×7+警示色体系不统一+基础样式三文件重复属实；token 化+career-base.css 家族级重构进 backlog（与 138/141 合并）。 |
| 151 | apps/web/src/career/usage.css:14-14 | usage.css #07c05f×2（:14/:6 附近 eyebrow/basis）属实；token 化归并 CSS 家族 backlog。 |
| 153 | apps/web/src/chat/ChatRoutePage.tsx:1929-1932 | 三连 sessions.find 属实（现 :2019/:2020/:2026），提取 useMemo 复用属小重构 |
| 155 | apps/web/src/chat/chat-header.tsx:125-126 | menuMode 为非空三态(:57)，!menuMode 恒 false 属实；守卫改 === 'menu' 属防御性行为变更 |
| 157 | apps/web/src/chat/chat-header.tsx:188 | onTogglePin! 断言加固与 titleDisplay 冗余两问题混合，收敛方式需抉择（局部变量/删回退），非单行 |
| 158 | apps/web/src/chat/chat-u.css:201-209 | :207 全边 border-color 配单边 width 的模式系统性横跨 chat-u/views-chat-u 多规则，统一改写非单行 |
| 159 | apps/web/src/chat/views-chat-u.css:220-233 | 系统性死声明属实（:229/232 font-size 双写、pulse 重复 :640/:2375），批量清理属重构 |
| 162 | apps/web/src/commercial/surface.tsx:10-13 | Card/Status 与 shared/wk-legacy.tsx 等逐行同构、色值逐字重复、描边已分叉(#dce3ed vs #e7e7e7)属实；收敛共享实现+参数化前缀为跨域重构进 backlog。 |
| 164 | apps/web/src/commercial/surface.tsx:4-5 | 域内 13 处 theme="default" variant="outline" 三连样板属实；导出带默认值的 Button 封装+13 调用点改造为机械多站点重构进 backlog。 |
| 165 | apps/web/src/configuration/ConfigurationPage.tsx:111 | :111 确为 errors→loading→agents→空列表→列表 4 层嵌套三元；抽 renderSectionBody 属重构级 |
| 169 | apps/web/src/data-sources/DataSourcesPage.tsx:475 | confirmDelete 失败态确不重置 deleteSource 且错误渲染于遮罩外；Dialog 内错误行需新增 state+渲染分支 |
| 171 | apps/web/src/data-sources/ui.tsx:18 | toneClass 嵌套三元实证；查表化重构（含 DataSourcesPage 同型两处） |
| 175 | apps/web/src/documents/KnowledgeDocumentsPage.tsx:4716(漂移) | `selected.size > 200` 裸字面量仍在（波 5123688d2 未动）；连同详情页 25 页多处，提常量属小重构。 |
| 177 | apps/web/src/documents/TagPickerDialog.tsx:239-260 | TagFilterPanel 体内（254-341）确无 onClose/onClear 引用，死参数属实；删除涉接口+调用方，且清除入口去留是 UX 决策。 |
| 178 | apps/web/src/documents/documents-u.css:2093(漂移) | blockquote 仍 `border-left-width: 8px`（波 5123688d2 未修此值）；改 2px 是可见渲染变化需目检，另 Vue 4px 对齐待专项。 |
| 179 | apps/web/src/documents/preview.ts:8 | 跨域引 views-chat-u.css 属实，但同模式已有 5+ 文件（Embed/Chat/Experts/DevMarkdown 等），下沉 shared 层为专项重构。 |
| 184 | apps/web/src/experts/experts-u.css:2 | 头注释失实（无 td.css）属实，但捆绑重复注释合并+多处冗余声明清理，整体重构级 |
| 185 | apps/web/src/experts/experts-u.css:66 | 硬编码色值 vs var() 约定不一实证；统一 var(--color-accent) 等多点改写 |
| 187 | apps/web/src/faq/FAQPage.tsx:1373 | editorTitle 死 prop 属实（仅 :683/:779/:2497 三处，抽屉头 :1371 用 editorMode）；但 test :64/:364 等仍传参，需连测更新 |
| 191 | apps/web/src/faq/FAQPage.tsx:2177-2181 | message prop 零读取属实（:688/:784/:2502，message.tone/text 无引用）；删除跨接口/解构/父传参 3 处死代码 |
| 192 | apps/web/src/faq/FAQPage.tsx:2345 | updateEntryTag(:2377) 仅 Number() 无防护（confirmBatchTag :2352-2353 有 isSafeInteger+非负）；补守卫属行为修复 |
| 193 | apps/web/src/faq/FAQPage.tsx:2481 | removeMany(:2365-2370) catch 吞错后正常 resolve，:2518 then 无条件清空选中属实；修复需 removeMany 回传成败 |
| 197 | apps/web/src/faq/FAQPage.tsx:956-960 | 嵌套三元多处属实（:950-954 及 importTask 图标/文案等），改辅助函数/查表属批量重构 |
| 199 | apps/web/src/faq/faq.td.css:43-46 | fade-enter/leave 等 Vue transition 类在 TSX 零引用属实；.question-tag(:2670/:2680)、.add-item-btn(:1726/:1737) 重复；批量死 CSS 清理 |
| 200 | apps/web/src/integrations/ApiPlaygroundDrawer.tsx:317 | 三层三元实证（L317）；查表化+死 data-status 规则取舍属重构 |
| 202 | apps/web/src/integrations/IntegrationsRoutePage.tsx:23 | 模块级 setIntegrationSpriteIconRenderer 注册实证；测试清理约定或改注入属架构级 |
| 203 | apps/web/src/integrations/integrations.td.css:19 | wk-api-playground-*/wk-embed-preview-* 死选择器实证（tsx 全部改用 wk-apd-*/wk-epm-*，含保留项 .wk-integration-drawer-close）；~110 行甄别删除 |
| 204 | apps/web/src/integrations/views-integrations-u.css:28 | 死声明群实证（150ms/180ms 等）；波 cf1c5e529 仅在 L830 追加 iframe 规则未触及此问题；跨 2 文件 ~8 处清理 |
| 206 | apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1748 | 嵌套三元+`?? 'standard'` 三重求值仍在；提取局部变量需进 JSX 所在回调作用域，非机械一行。 |
| 207 | apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:178-186 | OrgGreenIcon 双份内联属实（本文件 179 与 Drawer 61，签名略异）；抽共享模块涉两文件。 |
| 208 | apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:192 | SPACE_GRADIENTS 与 organizations/SpaceAvatar 重复属实；去留涉 Vue 1:1 parity 决策（finding 自提 Phase 4 归并）。 |
| 212 | apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:549(漂移) | 基类 border-left:0 使首段左缘缺边（波 94dc7a1a9 未修）；删除会新增可见 1px 描边，需目检对齐 tdesign 模型。 |
| 218 | apps/web/src/market/MarketPage.tsx:539 | 嵌套三元实证（539 及 660/727）；抽 TOAST_TONE_CLASS 属重构 |
| 221 | apps/web/src/organizations/OrganizationsPage.tsx:1289-1291 | :1332-1334 末尾 :null 不可达属实（四条件穷尽），但收敛 JSX 链涉分支重排，非纯单行死码删除 |
| 222 | apps/web/src/organizations/OrganizationsPage.tsx:1332-1334 | wk-org-5 <720px 显示(org-u.css:318-322)而 orgs.td.css 无 .settings-sidebar 移动隐藏(:1320，仅 min-width 媒询)，双导航+注释失实属实 |
| 223 | apps/web/src/organizations/OrganizationsPage.tsx:1400 | 5 处 htmlFor 悬空属实：id 仅有 edit 态/textarea(:1512/:1523/:1787/:2114)；create Input(:1443)、join-code(:2134)、join-search(:2143)、upgrade-role(:1782)、join-request-role(:2109) 无 id |
| 225 | apps/web/src/organizations/OrganizationsPage.tsx:264 | :254 嵌套三元属实；查表重构+create/edit 两模式输入控件栈统一非单行 |
| 226 | apps/web/src/organizations/OrganizationsPage.tsx:291 | :281-287 嵌套三元+IconUser 默认尺寸 vs 14px svg 不齐属实；查表+尺寸统一属小重构 |
| 227 | apps/web/src/organizations/SpaceAvatar.tsx:29 | :29-32 三处嵌套三元属实；另发现 :29/:32 模板串缺空格（`wk-avatar-2${…}`→类名粘连，org-u.css:1459/:1483 独立类未生效），应一并修 |
| 228 | apps/web/src/organizations/org-u.css:53-58 | 波 774357247 仅改 wk-org-67 calc+org-card 键盘(git show)，未触此规则不算 fell；ORG_FIELD 挂 TDesign 包裹层(:1523/:1673/:1698/:1783) :focus 永不命中属实，改 :focus-within 属视觉行为修复 |
| 229 | apps/web/src/organizations/orgs.td.css:769-773 | 双套规则属实（:769/:780/:786 6px vs :827/:904/:928 8px，靠 0-3-0 特异性取胜）；合并需逐卡片视觉核对 |
| 234 | apps/web/src/platform/PlatformShell.tsx:768 | 渲染体直写 ref 实证（L858）；移入 useEffect 属语义等价重构非死码 |
| 237 | apps/web/src/routes.tsx:59 | resolveRoute 强制 snapshotId（L59）vs router.tsx:897 ?? '' 宽容，口径分裂实证；二者取其一属设计决策 |
| 238 | apps/web/src/settings/ChatHistorySettingsPanel.tsx:114-118 | :115 ModelSelector 未传 onAddModel（ModelSelector.tsx:94 点击为空操作）；retrieval 至今保留回退（ConfigSettingsPanel.tsx:204）；壳层 models 失败 setModels([])（SettingsPage.tsx:239-240） |
| 239 | apps/web/src/settings/CloudSettingsPanel.tsx:136 | :136 `!form.appId |\| !form.appSecret` 无 trim；surface.ts:268-272 trim 后抛英文 Error，本地化 fillRequired 警告被跳过 |
| 240 | apps/web/src/settings/EnvVarSettingsPanel.tsx:84-86 | 1890a957f 仅加壳层 loading/错误态+重试，未加 name/scopeId 本地化守卫；surface.ts:291-292 仍抛英文 |
| 241 | apps/web/src/settings/GeneralPreferencesPanel.tsx:351 | 全文件无 aria-label：:278/293/306/325 Select 仅 placeholder、:352 Switch 裸控件，a11y 回退属实 |
| 243 | apps/web/src/settings/ModelDebugPanel.tsx:323-325 | :323-325 三处 Number(value)，TInputNumber 清空回调 null→Number(null)=0；fromTInputNumber 在 ModelSettingsPanel.tsx:219 未导出，需 export+三处替换 |
| 246 | apps/web/src/settings/OllamaSettingsPanel.tsx:234-239 | :231-236 下载 TInput 无 disabled，进度轮询闭包持旧 downloadModelName 与实时 state 不一致；附带内联样式规约问题 |
| 247 | apps/web/src/settings/OllamaSettingsPanel.tsx:54 | :54 `testing ? null : status ? … : null` 嵌套三元，抽具名辅助函数属风格重构 |
| 251 | apps/web/src/settings/PortedSectionsPanel.tsx:56 | wk-settings-read-note 唯一存活规则限定 .env-settings 作用域（settings-wrapper.css:553），本面板不在该子树；wk-ported-note 有全局规则（settings-wrapper.css:619），但换类属视觉变更 |
| 252 | apps/web/src/settings/ResourceSettingsPanel.tsx:1040 | :1040 与 :1063 两处 `theme: 'error' as never`；Sandbox 口径 :1649-1657 本地声明 theme?:'error' 直写；统一需类型改造（worktree 无 node_modules，TDesign DropdownOption 类型不可验），12b95330a 未触及 |
| 254 | apps/web/src/settings/SandboxSettingsPanel.tsx:1911 | :1911 keyDown 仅判 Enter，Space 滚屏不触发 openEdit；波 3036535cf 未触及 keyDown |
| 256 | apps/web/src/settings/SettingsPage.tsx:541-546 | 死分支属实：parser/system/userprofile ∈ SELF_HEADER_SECTIONS(:108) 早退不到 legacy 分支；members ∈ SELF_ERROR_SECTIONS(:99) → load :254-257 setError(null)，banner-retry 块恒不可达（连带 TAlert/TButton import） |
| 257 | apps/web/src/settings/SettingsPage.tsx:558-561 | wrapperModifier 嵌套三元 + SELF_HEADER/legacy 双面板链重复且各含死分支，需查表函数+panelForKey 重构 |
| 260 | apps/web/src/settings/SkillSettingsPanel.tsx:194-197 | :196 帮助触发器为 TIcon(span) 不可聚焦，TTooltip placement right 无 focus 触发，键盘用户无法触达 |
| 263 | apps/web/src/settings/SystemInfoPanel.tsx:93 | :93 index===1 魔法下标锚定第二行；SystemInfoRow 有 labelKey（surface.ts:324，:370 frontendVersion 行为 'system.frontendVersionLabel'），改语义锚属行为等价重构 |
| 264 | apps/web/src/settings/TenantDeleteZone.tsx:91-97 | :91-97 Input 仅 placeholder 无 aria-label（全文件 aria-label 仅 :54 aside）；附带 error 静态 inline style 归类问题 |
| 266 | apps/web/src/settings/TenantMembersPanel.tsx:343-377 | permissionsOpen(:343)/permissionsRef(:367)/revokeConfirmKey(:376)/removeConfirmKey(:377) 无任何置真/置非空路径，effect(:523-531) 因恒 false 永不挂载；清理涉多处声明+setter，重构级 |
| 267 | apps/web/src/settings/TenantMembersPanel.tsx:786 | :786 TPopup trigger="hover" 且触发按钮无键盘展开路径/aria-expanded；需 focus 触发方案（"hover focus" 写法是否合法待验） |
| 268 | apps/web/src/settings/TenantMembersPanel.tsx:883 | :534-536 submitInvite 空邮箱静默 return；:883 TInput type="text" 无必填/格式校验，非法输入直发后端 |
| 272 | apps/web/src/settings/settings-toast.tsx:62-66 | :62-66 tone 三层嵌套三元；查表 Record<tone,string> 重构，属风格重构 |
| 273 | apps/web/src/settings/settings.td.css:2329-2331 | settings.td.css 全文件无 @keyframes spin 定义；注释所指 kb-list.td.css 为懒加载 chunk，实际兜底是 platform-u.css:469 静态导入的隐式跨域依赖；本地补 4 行定义 |
| 274 | apps/web/src/settings/settings.td.css:3805 | 同名 keyframes 两处取值不一致：:3805 一行式 from translateX(18px)/opacity .7 vs :5739-5742 from translateX(24px)/opacity 0（后者胜出）；删哪份需核对 sandbox-settings.css 事实源 |
| 275 | apps/web/src/shared/shared-u.css:151 | anywhere 实证（L64/L151）；改 break-all 是断行语义变更，需对 Vue 事实源做折行视觉核对（.wk-shared-7/16 两处） |
| 278 | apps/web/src/shared/wk-legacy.css:47 | 全局 keyframes 与 TenantAuditDrawer.tsx:161 inline animation 撞名实证（其 L138 注释已过时）；改名涉 2 文件 3 处并撤销意外获得的动画 |
| 279 | apps/web/src/shared/wk-legacy.tsx:75 | WkDialog/WkSheet 双份 focusable 查询+不对称过滤实证；波 97e76f8d6 仅删 aria-label 未触及；提取 getFocusables 属重构 |
| 281 | apps/web/src/tdesign-icon-offline.ts:15 | installed 标志 true→false 状态舞步实证（L15-22）；简化以占位节点存在性为幂等依据属小重构 |
| 282 | apps/web/src/tdesign-icon-offline.ts:8 | 与 0.6.11 内部 URL 强耦合实证（版本已精确锁 0.6.11）；失效检测属新增防护（dev 告警/grep 测试） |
| 286 | apps/web/src/wiki/wiki-reader.css:262-265 | 死选择器 `.wk-wiki-search-input.t-input__wrap` + `.wk-wiki-search-input` 全局泄漏属实（波 94dc7a1a9 未合并）；合并需核对 tdesign className 落点。 |
| 289 | apps/web/src/wiki/wiki-u.css:762(漂移) | .wk-wiki-78(758) transition-property 仅 transform 而 .wk-wiki-79(771) 用独立 rotate 属性属实；补 `rotate` 恢复 150ms 动画属可见行为变化，需目检。 |
| 291 | internal/handler/session/artifact_download.go:702-706 | L709-712 GetFile 失败仍静默 404；姊妹路径 L257/L622 均有 Warnf，属观测性补日志（行为新增）非 trivial |
| 292 | internal/handler/session/workbench_artifacts.go:277-285 | L199-207 与 L277-285 TTL 解析/封顶逐字重复；提共享 helper 属重构 |
| 294 | internal/modules/career/career_export.go:307-310 | L298 FOR UPDATE 锁内 L311 buildCareerExportArchive 全量读+写 text 列仍在；快照移锁外/体积上限属性能重构 |
| 296 | internal/modules/career/career_export.go:632 | L644 profile 节仍只 Count career_facts；career_proposals 在 careerPurgeTables(L721) 却无任何 sectionCount 行，补计数属展示面变更 |
| 297 | internal/modules/career/handler.go:1584-1588 | L1601-1604 仍返 ErrInvalidRequest(400)；同文件 17 处 MaxBytesError→413（L241 等）；改 413 是契约级行为变更 |
| 298 | internal/modules/career/handler.go:45-47 | L45-47 keyErr 仍完全吞掉，非法密钥装配期无日志；加 warn/报错属行为新增 |
| 299 | internal/modules/career/material.go:948-951 | L954-960 仍以 heading 为唯一 map 键；拒重复 heading 或复合键对齐均为语义级改动 |
| 300 | internal/modules/career/office.go:606-608 | L646 仍 TrimSpace 仅判空而存原值 k（propose L616 先规范化）；改存储行为变更 |
| 301 | internal/modules/career/profile_intake.go:583 | L668 仍无条件 AND claim_token=?，finishSource L519 已条件化；潜伏不一致需语义对齐 |
| 302 | internal/modules/career/reminder.go:231-238 | L231-237 事务内回放命中后仍无条件 remindAfterCommit，无 created 区分 |
| 303 | internal/modules/career/reminder.go:538-542 | L533-548 reminderReceiptFromRow 仍不填 Revision，去重回执恒 0 |
| 304 | internal/modules/career/search_once.go:298-300 | L298-299 仍对行缺失返 ErrIdempotencyConflict；所述整体中止已被 5e98b23b1(H8) 围栏（search_rule.go L621-654 保留 outcomes），余留哨兵语义待区分 |
| 305 | internal/modules/career/search_once.go:425-429 | L431/L437 仍返裸 errSearchReceipt；%w 包装变更错误文本需过测试面 |
| 310 | packages/api-client/src/career.ts:1 | L1-2 仍 '../../career-core/src/contracts.ts' 相对跨包导入；改 '@weknora/career-core/contracts' 需验证 workspace 解析与构建，非零风险 |
| 311 | packages/api-client/src/career.ts:1272 | L1386/L1406 发送 trim 后值 vs L1350/L1440 原样发送并存；统一策略是波及多方法的契约决定 |
| 313 | packages/api-client/src/career.ts:1432 | L1455-1459 仍原样返回 grant.digest、不复核 body SHA-256；加 crypto.subtle 校验属功能新增 |
| 314 | packages/api-client/src/career.ts:1437 | L1464 DELETE 携 body 属实且后端 decodeStrictJSON 依赖之；改查询参数需后端同步改绑定，链路假设待确认 |
| 316 | packages/api-client/src/index.ts:209-211 | L210-212 仅露 createCareerApi/CareerRequest+8 类型；补全公开导出或移除半截入口属 API 面设计决定 |
| 317 | packages/career-core/src/contracts.ts:158-159 | decodeEvaluation 的 isRecord/onlyKeys 与 validEvaluationReceipt(:151-152) 重复判死属实；共享键列表常量化为重构 |
| 318 | packages/career-core/src/contracts.ts:160-164 | TextEncoder 依赖属实(:161,164)；但前提有误——小程序已在调用：career.ts:240 evaluateOpportunity→decodeAs(decodeEvaluation)，风险比报告更高，需守卫/标注 |
| 319 | packages/career-core/src/contracts.ts:89 | OpportunitySource(:89) 与 CareerSource(:1) 逐字段重复、校验双轨属实；类型收敛为重构 |
| 320 | packages/career-core/src/desk.ts:148 | send() 解码 TypeError→歧义→unresolved 锁死路径属实(:148,157,160)；contract_violation 码+解除出口为设计级 |
| 321 | packages/career-core/src/desk.ts:188 | mergeProposal pending→pending 无 revision 防回退属实(:188)，与 mergeFact(:181) 不对称；守卫含 revision 缺失容忍语义 |
| 322 | packages/career-core/src/desk.ts:29 | sameAction JSON.stringify 键序敏感属实:29；逐字段比较/规范化序列化为 API 语义改动 |
| 323 | packages/design-tokens/src/tdesign-theme.css:2 | 行2/92（light/dark）title-large 行高均以 title-medium 为基数，Vue 源 theme.css 同款写法证实为上游笔误被 ff7380052 保真平移；需上游确认后双端统一修+补来源注释，单端改破坏保真契约 |
| 324 | packages/design-tokens/src/tdesign-theme.css:38 | dark 块 error 2→9 为 #5e2a2d→#edb1b6 红系递亮而 color-10=#eeced0 黄绿断层；Vue 源同值证实上游笔误平移，需与上游确认后双端统一修正 |
| 325 | packages/mobile-core/src/runtime/mobile-runtime.ts:189 | 注释在行188；行401-406 authorizedRequest 以 readOnly(GET/HEAD/OPTIONS) 作 retryUnauthorized，写方法 401 上抛不刷新；188 行注释与 types.ts:57「refresh-once on 401」口径过时，需双文件同步更新注释（非 typo 类，宁缺毋滥） |
| 326 | packages/views/src/chat/chat-copy.ts:1177-1178 | ko L1314/ru L1693 requestInfoEmpty 仍英文兜底（ja L935 已译）；补译属内容新增且译文质量需把关 |
| 327 | packages/views/src/chat/composer.tsx:238-239 | L657-658 两个 setTimeout（kbTipTimer L270，attachTipTimer L274 同病）仍无卸载清理 effect；加 effect 属行为新增 |
| 328 | packages/views/src/chat/composer.tsx:538 | setMentionQuery 仅 L417/L428 写 ''，L345 过滤恒真、L353/L369-371 恒无效、mentionNoResults 全仓无消费点；清理跨 6 站点+5 语言表 |
| 330 | packages/views/src/chat/message-face.tsx:289-290 | L522 私有 writeClipboardText 与 message-list.tsx L103 导出版（含测试覆盖）重复；下沉/复用属重构 |
| 333 | packages/views/src/chat/session-sidebar.tsx:307 | L408 单行 4×apiOwnerTagOf(session)+3 处非空断言；提 const 需改该 JSX 行结构 |

## trivial-fix 递延候选（58，并入 backlog，标记 deferred-trivial）（58）

| # | 位置 | 证据/理由 |
|---|---|---|
| 46 | apps/miniprogram/tests/assembly.test.mjs:24-25 | execDto/execEvent 全文件仅 24-25 行出现（grep 实证）；office-assembly.test.mjs:16-17 有独立在用副本 |
| 47 | apps/miniprogram/tests/assembly.test.mjs:65 | meCount 仅:65 声明无引用（grep 实证），死变量 |
| 48 | apps/miniprogram/tests/career-discovery.test.mjs:62 | ambiguous helper 仅:62 定义（155/469 为标题文案）；errorCode 独立在用（:462,487）须保留 |
| 55 | apps/miniprogram/tests/rules-usage-reminders.test.mjs:30 | clearPrivateCache 仅:30 导入无使用（grep 实证）；storage 模块已经 runtime.ts 传递加载，无副作用损失 |
| 62 | apps/web/src/administration/administration-u.css:33 | L33 #07c05f；styles.css:472 已定义 --color-accent，本文件 L220-221 同令牌已用 var()，改写行为不变 |
| 63 | apps/web/src/administration/administration-u.css:57 | .wk-admin-3 color:#2e6de6；styles.css:466 定义 --color-primary:#2e6de6，改 var() 零行为差 |
| 77 | apps/web/src/agents/AgentsPage.tsx:902 | 渲染期 `favoritesRef.current = favorites` 冗余：936/947/1022/1036 全部 setFavorites 点已同 tick 写 ref，删行零行为差。 |
| 81 | apps/web/src/agents/list.ts:427 | `scope.kind === 'selected'` 收窄后 `scope.kind === undefined` 恒 false，`'' :` 分支不可达，简化零行为差。 |
| 82 | apps/web/src/analytics/analytics-u.css:243,304 | 两处同行双声明实证（243/304）；其余边无 border 宽度，border-color 收窄为 bottom 零视觉差 |
| 92 | apps/web/src/auth/JoinPage.tsx:8 | JoinPage.tsx:6 LoginPage(携带 login.td.css:21) 先于 :8 auth-u.css，违 auth-u.css:3 契约；wk-join/onb 与 login.td 选择器无交集，重排零视觉影响 |
| 108 | apps/web/src/career/ExportDeletionPage.tsx:193-194 | :209-210 anchor.click() 后同步 revokeObjectURL；同目录 MaterialPage.tsx:52 saveBlob 已用 setTimeout(…,5_000) 延迟回收，对齐即一行。 |
| 109 | apps/web/src/career/ExportDeletionPage.tsx:247-247 | :267 setDeletionMessage('空间已完全删除…') 后 :266 已置 phase 'idle'，消息仅在 :403 deletionPhase!=='idle' 渲染；deleted 后 startDeletionDisabled(:347) 永锁，无后续非 idle 转换，确为死写（:95 同款孪生死写）。 |
| 119 | apps/web/src/career/MaterialPage.tsx:43-47 | :37-41 sha256Hex 直用 crypto.subtle.digest 无特性检测（同文件 :6 newRequestId 已检测 crypto）；非安全上下文抛 TypeError 被 :514 兜底成误导文案。加一行守卫即可。 |
| 132 | apps/web/src/career/RulePage.tsx:92-93 | load()(:104) 起始清 viewError/storedRuleUnreadable 但不清 notice；:140 与 :141 附近分支设置的过期告警在重读成功后仍渲染于 :346 compose 卡，与解锁状态矛盾。load 起始补 setNotice('') 一行即除。 |
| 136 | apps/web/src/career/UsagePanel.tsx:27-34 | SearchPage:347 用 usageAllowsChargedRun(ready+wouldAdmit)，RulePage enableRequiresEstimate 仅 phase!=='ready'（有意设计，RulePage 注释自述）——注释「single admission gate consumed by…RulePage enable」失实，fd2b5e809 只修了证据文档未修此注释；纯注释改写零行为。 |
| 142 | apps/web/src/career/opportunity.css:127-128 | :127-128 仅 --ineligible/--unknown 左边框，--eligible 落空；而 JSX OpportunityPage:377 按 outcome 拼接该类，且同页 __verdict--eligible/opportunity.css:119 有绿色先例——补一行同款规则即闭合枚举覆盖。 |
| 143 | apps/web/src/career/preparation.css:107-107 | tdesign-theme.css 真实 token 为 --td-bg-color-secondarycontainer（:1/:68 定义，dark :68 重定义），secondary-container 仅此一处拼写错误；变量永不生效回退 #f5f5f5 常驻，dark 主题与 10 处正确用法不一致。 |
| 148 | apps/web/src/career/rule.css:65-65 | tdesign-theme.css --td-warning-color-7=#ba431b（深红棕）≠回退 #e37318（橙），token 生效后渲染色与警示意图不符；export-deletion.css:68/:90 同类警示用 var(--td-warning-color,#e37318)——去 -7 后缀对齐即一行。 |
| 152 | apps/web/src/career/usage.css:18-19 | UsagePanel.tsx forbidden(:74)/unavailable(:77) 分支外层 wk-career-usage-wrap、Card 无 wk-career-usage 类，:18-19 选择器命中不了（ready 分支 :80 命中）；wk-career-usage-wrap 全仓零样式定义，放宽前缀四分支通吃且 overage 样式不变。 |
| 154 | apps/web/src/chat/SessionShareDialog.tsx:6-7 | :8-9 注释仍称"内联 Tailwind utilities"，实际 :15 引 chat-u.css、:28-30 用 wk-ssd-* 语义类；仅改注释零行为 |
| 156 | apps/web/src/chat/chat-header.tsx:175-179 | :177 'document' 分支与兜底均执行同一 onMenuVisibleChange(visible)，删分支行为恒等 |
| 161 | apps/web/src/commercial/commercial-u.css:19-21 | .wk-bill-usage-number-cell 在 :19 与 :41 双定义、:13 两个 border 声明挤行，合并+border-bottom 简写渲染值逐像素等价（其余边框 width=0），零视觉变化的整理性修复。 |
| 166 | apps/web/src/configuration/config-u.css:262-266 | :262-263 font-size 0.8rem/color #6941c6 被同块 0.85rem/rgba !important 覆盖，恒不生效的死声明 |
| 168 | apps/web/src/data-sources/DataSourcesPage.tsx:451 | cancel-replace Button 残留空 className="" 实证（L451） |
| 170 | apps/web/src/data-sources/data-sources-u.css:28 | 三处 transition-duration:150ms 均被紧随 200ms 覆盖（L28/356/669），死声明 |
| 172 | apps/web/src/documents/DocumentsPage.tsx:137,165 | 两处 `() => void load` 丢括号未调用（对比 69 行 `void load()`、143 行 `void upload()`）；documents 测试无 Reload 断言。 |
| 173 | apps/web/src/documents/DocumentsPageChrome.tsx:41-44,146 | Chevrons 常量（非导出）全文件零引用；146 空态在 `kbList.length` 真值分支内经排序恒 ≥1，不可达。 |
| 180 | apps/web/src/embed/embed-u.css:2 | 头注释指向不存在的本域 .td.css（embed/ 目录 ls 证实无），注释纠错一行 |
| 181 | apps/web/src/embed/embed-u.css:151,287 | .wk-emb-15/29 基类均已有 border-style:solid（L139/L275），hover 内重复声明死码 |
| 182 | apps/web/src/embed/embed-u.css:30,298 | 两处同行双声明实证，拆行纯格式 |
| 183 | apps/web/src/experts/ExpertsPage.tsx:49 | 注释仍称 "Tailwind v4 utility recipes" 而常量已是 wk-exp-xp-* 语义类，comment-only 纠偏 |
| 186 | apps/web/src/experts/experts-u.css:276 | 基类 width:200px 死值：XP_INPUT 全部 3 处使用（752/819/888）均追加 wk-exp-45(100%)，删基类 width 行零视觉差 |
| 188 | apps/web/src/faq/FAQPage.tsx:1519 | :1517 字面量 5 vs :121 FAQ_ANSWER_CAP(=5，:1510 已引用)；替换零行为且消除上限漏改风险 |
| 189 | apps/web/src/faq/FAQPage.tsx:1546 | :1544 与 :1697 两处 value == null 违项目规则；改显式 === null|\|===undefined 语义恒等 |
| 196 | apps/web/src/faq/FAQPage.tsx:521 | :536 !sortedKbList.length 恒 false（:477-481 排序保长、外层 :513 kbList.length 门）且 common.noData 键不存在于 packages/i18n |
| 198 | apps/web/src/faq/FAQPage.tsx:991 | :985 case 'export' 死分支：handleFaqAction 输入仅 create/import/export_csv/export_json(:963-970)+字面 'search'(:1159) |
| 201 | apps/web/src/integrations/EmbedPreviewModal.tsx:92 | 硬编码"正在加载预览…"实证；locale prop 已在作用域（L48），embedWizardMessages 5 语言已备 previewLoading 键，一行换用 |
| 210 | apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 | permissionTone 兼容层全仓（ts/tsx/mjs 含测试）零消费者，本文件已改用 permissionTheme（48/160）；删注释+导出零行为差。 |
| 211 | apps/web/src/knowledge-bases/kb-editor-parity.css:22-24 | 全仓 tsx 无任何 className="grid" 的 label（留守段均 form-label/wk-kbl-*），两条 label.grid 规则为死规则；.kb-editor-desc-count 三处收敛另列 follow-up。 |
| 214 | apps/web/src/knowledge-settings/knowledge-settings-u.css:163,179 | wk-kss-19/21 两处 `border-bottom-width: 1px;border-color: #dcdcdc;` 同行挤写，拆两行纯格式零行为差。 |
| 216 | apps/web/src/knowledge/knowledge-u.css:315(漂移) | 315 `transition-duration: 150ms;` 被 316 300ms 同规则后置覆盖为死声明，删除零渲染差；附带 420 同行挤写一并拆行。 |
| 219 | apps/web/src/market/market-u.css:379 | 仅 L379 一处同行双声明实证（.wk-mkt-32 处不存在，前提部分失实）；拆行纯格式 |
| 232 | apps/web/src/platform/PlatformShell.tsx:1212 | 内联 style={{cursor:'pointer'}} 实证；td.css:153 已有 .aside_box .logo_box 规则可并入，行为不变 |
| 236 | apps/web/src/router.tsx:45 | 注释仍称 "pulls in packages/ui 旧栈(theme.css)" 而 packages/ui 目录已不存在，comment-only 纠偏 |
| 248 | apps/web/src/settings/PersonalMemoryPanel.tsx:129 | :129 `as never`；kvApi update 形参 SettingsPayload=Record<string,unknown>（api-client settings/index.ts:6,177-178），memoryWorkspacePatch 返回同型（surface.ts:174），断言纯冗余 |
| 249 | apps/web/src/settings/PersonalMemorySettingsPanel.tsx:791-793 | od 证实文件以 `}` 结尾无换行符；波 3036535cf 触及该文件但未补 |
| 259 | apps/web/src/settings/SkillSettingsPanel.tsx:1331 | :1331 `maxLength={10000}`；TDesign 口径为小写 maxlength，同域先例 McpSettingsPanel:1302、PersonalMemoryPanel:298 等全用小写；波 3036535cf 未改 |
| 269 | apps/web/src/settings/TenantUserProfileSections.tsx:317-319 | 注释称 owner-only，实际门控 :320 canEditTenant(owner|\|system-admin，:58)；Vue 源 showDeleteDangerZone 确为 hasRole('owner')（TenantInfo.vue:301-303），注释失实 |
| 271 | apps/web/src/settings/TenantUserProfileSections.tsx:362 | :362 与 auth/validation.ts:5 导出的同名常量逐字相同；SystemGlobalSettingsPanel:14 已示范从该处导入，本地副本有漂移风险 |
| 276 | apps/web/src/shared/shared-u.css:175 | 单类覆盖依赖注入顺序实证；元素三类并存（SharedSessionPage.tsx:82），升为 .wk-page--std.wk-shared-main 零行为差 |
| 277 | apps/web/src/shared/shared-u.css:26 | 四块双写 border-style:solid 实证（.wk-shared-4/8/13/15，L30/32、75/77、116/118、137/139），删第二条 |
| 280 | apps/web/src/styles.css:56 | 规则实际在 platform-u.css:665，注释误指 platform-shell.td.css；NotFoundPage.tsx:14 同源笔误，comment-only |
| 283 | apps/web/src/test-tdom-harness.ts:51 | tdomWindow 全仓零消费方（仅定义处命中），死导出 |
| 308 | internal/modules/workbench/service/workbench/application_task.go:84-88 | L84 与 L106 同参重复 normalizeApplicationTaskIntent（幂等）；ensureWithRetry 无其他非测试调用方，删入口处重复块行为恒等 |
| 309 | package.json:13 | packages/ 目录已无 ui（ls 证实），test:shared 的 packages/ui/src/*.test.ts(x) 两 glob 永不匹配；c77da2710 round-2 清理的是 i18n .tsx 非 ui，且该两段仍残留——死配置 |
| 312 | packages/api-client/src/career.ts:1355 | L1378 第二条件 operation.trim()!==operation 不可达（'search_once'.trim()===自身）；删冗余子句零行为差 |
| 315 | packages/api-client/src/career.ts:773-775 | L796-798 注释 "write inputs carry no source at all" 与 L1471/L1479 body 携 source:{kind:'manual'} 矛盾（服务端 handler.go:1126 progressClientSource 白名单必需）；纯注释更正 |
| 338 | packages/views/src/integrations/page.tsx:1915-1917 | L2122-2124 注释称 1em=14px 且 fallback svg width/height=14，唯 size="15px" 失配——注入/回退两路 1px 尺寸差；对齐 14px 消除失配 |

## 应用批次 commit（worktree chore/ocr-low-adjudicate）
- bb316d52d fix(agents) #71(导入) #75 + ddf486a5d fix(agents) #71(文件删除，stash 往返暂存丢失补交)
- a60a393a1 fix(wiki) #213 #215 #287 #288
- c0bb53911 fix(auth) #97 #98
- 9a41c308d fix(organizations) #224
- 4cb555d37 fix(settings) #244 #253
- ae56b1c5d fix(mobile) #31
- a30c6a151 fix(embed) #5
- bd45736e0 fix(web) #61 #235

## 门禁证据（12b95330a 基线对照）
- 受影响套件（Node 26.4.0 + tsx）：agents 178(1败)、auth 56(1败)、organizations 60/60、wiki 75/75、knowledge-settings 132/132、settings 子集 51(1败)、embed 21/21、miniprogram rules-usage-reminders 24/24（node --import tsx --test；裸 --experimental-transform-types 在 Node 26 无此 flag、无 loader 时 .tsx 不可载，须 tsx）。
- knowledge/KnowledgeGraphPage.test.tsx 2/3、knowledge-bases/shared-kb-drawer.test.tsx 2/3。
- 上述 5 处失败全部经 stash 往返在干净基线 12b95330a 复现同数（agents KB-warn toast / auth login 双提交锁 / settings ESC add draft / graph 3rd / drawer 3rd）——**均为基线预存，与本批 15 项修复无关**。
- typecheck：apps/web `tsc -b` 基线即有 1 错（ApplicationPage materialId prop，9fc83ceec 后遗留）与本批后同为 1 错；apps/miniprogram 基线即有 account/pages.tsx CommercialSummary 系错误（未触文件）；apps/embed 干净。本批新增 0。
- `git diff --check` / `--cached --check`：干净。

## 附注
- 任务书所述第二来源 high-medium-digest.md 实无 Low 段（133 条全为 H/M）；实际第二 Low 源为 fullrange-partial.md，已按此去重裁定。
- final 报告存在 5 个重复 file:line 键（progress-preparation.tsx:137-139、AgentsPage.tsx:255-259、surface.tsx:10-13、SettingsPage.tsx:113、TenantMembersPanel.tsx:770-773），按键去重后各裁一次。
- 语料与裁定中间产物存 .superpowers/sdd/2026-10-03-ocr-low/（gitignored）。

---

## 族级终判（2026-10-03 controller，门禁链闭环）

Career 族 OCR 门禁三段全闭环：**全范围完整报告**（333/333、0 LLM 失败，ae01b9c49）→ **High 全清**（首轮 13+增量 4=17 条全处置，含 H7 设计裁决与 r1 锁序修复）→ **Medium 全清**（27 条：22 修/2 登记/3 降级）→ **Low 全裁定**（338 条：fell 1/fixed 11/trivial 73〔应用 15〕/accept 41/follow-up 192/fp 20；250 项 backlog 注册于本表）。按 issue140 既定完成规则（OCR 完整 + valid critical/high/medium 修复），**#141–#171 与 #173 到达 closure-ready；#140（spec 根）待 #172；#172（T33）移动验收阻塞维持（Harmony 真机/生产源 allowlist/运维检查）**。遗留 backlog（非阻塞）：250 follow-up、2 项登记重构、agents error 面 owner 裁定。

