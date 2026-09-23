# SDD ledger — plan: docs/plans/2026-09-21-tdesign-react-migration.md

- 分支：feat/tdesign-react-migration（自 main eba57c1be 切出）
- Spec：docs/specs/2026-09-21-tdesign-react-migration-design.md（权威，冲突以 spec 为准）
- 环境：后端 :8084 / Vue :5174 / React :5175 全在线；凭据 ~/.weknora-parity-creds.env

## 前置裁决（Rulings）

- Ruling R1: parity 自动化不删除、维持 paused——两个 automation（41aaa5e6 扫描+修复、4a5e3b18 对齐推进）当前均为 enabled:false/paused，前置①目标已达成；Task 17 负责恢复为守护模式。若错误：自动化被误恢复会收割迁移中间态（当前 disabled，风险低）。
- Ruling R2: 在主 checkout 的 feat/tdesign-react-migration 分支执行，不另建 worktree——parity 基础设施（dev servers、scanner ROOT 硬编码、后端）绑定主 checkout 路径，且已批准的计划文本明确写"开迁移分支"；Phase 3 批内并行页面仍可按 AGENTS.md 用 worktree。若错误：:5175 会热载迁移中间态（parity 工作流常态，可接受）。
- Ruling R3: 前置"收尾提交"无操作——并行会话已以 89e17fb6a 提交那批 parity 修改，工作树干净。

## 预检冲突扫描表

| # | 任务对/任务 | 共享面 | 发现 | 裁决 |
|---|---|---|---|---|
| 1 | T1↔T4 | apps/web/src/styles.css | T1 顶部插 tdesign.css；T4 要求 design-tokens 主题链在其后加载 | 顺序兼容；T4 执行时核对 import 顺序 |
| 2 | T2↔T3 | spike 页文件 | T2 创建、T3 验证并删除 | 设计如此（throwaway） |
| 3 | T2↔全局约束 | frontend/ 修改禁令 | 约束自带例外条款（Phase 0 spike 页用后即删） | 按例外执行，T3 Step 4 强制清理 |
| 4 | T8↔T9,T11-14 | scripts/parity/auto-scan.mjs | T8 改稳态门，后续全部扫描依赖 | 顺序执行；T8 需全量基线确认无退化 |
| 5 | T8 自身 | getAnimations().pause() | 冻结 spinner 类动画于中间态可能引入新伪差 | 计划已含 noFreeze 白名单出口，执行时按需加 |
| 6 | T9↔T15 | apps/web/src/agents/agents.css | T9 删 agents 相关段，T15 删残余 | 兼容，分段清理 |
| 7 | T11↔T12-14 | PlatformShell/共享组件 | 批 1 迁共享层，后续复用 | 已写入 Phase 3 头注 |
| 8 | T4 测试 | tokens.test.ts | 内容锚点断言（防平移截断），非空洞断言 | 与评审规则无冲突 |

扫描结论：无需修正计划的冲突；T3 决策树为唯一计划内止损点。

## 任务进度

- Task 1: DONE_WITH_CONCERNS（commit 1018555b6）——安装+import 完成，测试 2293 全绿，vite build 通过且产物含 unlayered TDesign CSS；顾虑：`tsc -b` 基线预存 4 个 TS 错误（DevMarkdownPage.tsx:383、PlatformShell.tsx:1065-1066、packages/views/src/chat/mermaid.ts:127,158），stash 对照证明在 BASE 即失败，与 T1 无关。
- Ruling R4: 基线 tsc 失败属承重缺陷（阻塞 T15 等全部 build 门禁与 135 文件迁移的类型安全验证）——最小解法：由 T1 实现者追加一个独立 commit 修掉这 4 个错误（恢复 build 门禁），不引入其他重构；T1 的 Step 3 验证口径修正为"修复后 build+test 全绿"。若错误：修基线超出 spec 范围，但这 4 处是并行会话 89e17fb6a 遗留的回归，修复属恢复基线而非新设计。
- Task 1: complete (commits 1018555b6..44ccb18e3, review clean)
- Task 1 minors (deferred): ①报告称 4 个 TS 错误实为 5 行（计数笔误，已全修）②PlatformShell 账户卡 Enter 键为预存 no-op（键盘事件 button===undefined 恒提前返回）——迁移中复制账户卡模式时勿继承不可达键盘处理器，交最终评审 triage ③DevMarkdownPage:383 分离根场景 `?.` 静默跳过（与同文件 397/409 既有模式一致，无实际影响）
- Task 2: complete (commits 5c91ef07c, review clean)
- Task 2 关键情报（carry to T5/T6）: ①tdesign-react 命令式 API 在 React 19 下必须 `import 'tdesign-react/es/_util/react-19-adapter'`（dev 已实测，Phase 1 移到应用入口 main.tsx，生产构建需重验）②Vue 端 Notification 导出名 NotifyPlugin；Switch 需 :default-value ③库间已知 DOM 差异：Switch 根标签、Table 附加节点、Dialog 关闭态不挂载、Tabs 面板容器——零视觉占位，T3 截图预期项
- Task 2 minors (deferred): ①brief Files 清单漏列 apps/web/src/router.tsx（route tree 所在，in-scope 必要改动，已记录）②routes.tsx 超长单行条件追加第三条目（跟随既有风格，plan-mandated 一致性选择）
- Ruling R5: Phase 0 闸门判 PASS（有条件）——raw 像素未达标（静态 2.542%/图标 5.814%）100% 归因于 Phase 1 Task 4 尚未平移的主题 token（Vue 端全局加载 theme.css，React 端蓝色默认），归因对照组（React 注入 theme.css+body 上下文）静态 0.000%/dialog 0.199%/图标 0.014% 全达标；运行时兼容原生 PASS（React 19 + react-19-adapter 下 Message/Notification/Dialog 全可用）。计划判定标准错在排序（Phase 0 像素项未扣除主题差），按 spec §7 Phase 0 意图（React 19 兼容+库同构）修正口径。条件：T4 平移 token 后复跑 scripts/parity/tmp-spike-tdesign.mjs 确认 raw 坍缩至 normalized 水平，spike 清理（两端页+路由+tmp 脚本）顺延至 T4 确认后执行；Dialog 取消按钮 variant 差异（Vue base vs React outline，0.199%）为唯一真实库间差异，记入 T10 差异台账。若错误：若 normalized 达标是巧合（如对照注入有副作用），T4 复跑 raw 不坍缩会立即暴露，届时回滚闸门判定。
- Task 3: complete-as-gate (commit c3d806b38 证据；闸门 PASS per R5；清理顺延 T4)
- T3 教训（流程）：派发前忘跑 task-brief 脚本（task-3-brief.md 缺失），靠派发指令+计划文本兜住——后续任务派发前必须先生成 brief。
- Task 4: 评审 ❌ Needs fixes（c3d806b38..e4dde19ce）——平移保真/unlayered 级联/body 对齐/R5 复验清理全过；1 Important：styles.css 误删预置 --primary-foreground/--destructive-foreground 两 token（越权+未披露，index.ts 仍定义，包内分歧）→ fix round 1 已派
- Task 4 minors (deferred): ①锚点测试全在前 30 行防不了尾部截断（brief 设计继承缺口，建议补尾部锚点）②证据文档头部"tmp 脚本未删"与文末 R5 段自相矛盾 ③@import 插在规则后偏离 CSS 原生规范（打包器内联功能等价，brief"末尾"措辞同样违规）④TDD 红灯时序无法从 diff 验证（红态结构性保证）
- Task 4: fix round 1/5 (1 addressed, 0 open; commits e4dde19ce..a5f57ca46)
- Task 4: complete (commits c3d806b38..a5f57ca46, review clean after 1 fix round)
- Task 4 out-of-scope（复审发现，deferred）: ①dark 块历来不声明 --primary-foreground/--destructive-foreground（index.ts dark tokenCss 会输出；靠 light :root 级联等价，基线既有，无功能影响）②design-tokens 缺 styles.css↔index.ts 一致性守护测试（本次误删漏网根因，建议补）
- Task 5: complete (commits 3bdca449b, review clean)——守卫 0.4.5/sprite MD5 双端一致（08d27722…，976,443B）/adapter 落 main.tsx 顶层；headless 2575 请求 gtimg 0 命中（⚠️ 运行时项采信报告，静态项全独立证实）
- Task 5 minors (deferred): ①守卫 DOMContentLoaded 兜底分支理论上可注册两个 once 监听器（第二次调用 installed 已重置；querySelector 兜底再保险，无害，brief 逐字指定非实现者引入）
- Task 6: complete (commits 78655aff2, review clean)——五语言 Record 类型+isLocale 双锁、响应式 usePreferredLocale（weknora:locale-changed 既有约定）、ko_KR"흔한"为官方原文两包逐字同
- Task 6 minors (deferred): ①Vue config-provider 内还有 3 个 overlay 宿主（ManualKnowledgeEditor/ProtectedResourcePreview/UploadConfirmHost）——未来迁移这些 overlay 挂 RouterProvider 外时须放进 TDesignLocaleProvider 内，否则文案脱离 globalConfig（carry to T11-14）②Locale 类型双入口（@weknora/i18n 与 /runtime）并存，既有问题
- Task 7: complete (commits ba7f28f20, review clean)——playbook 六节/映射表 22 行号零误差/台账 6 条与 Phase 0 证据逐字吻合
- Task 7 minors (deferred): ①两处 index.tsx 行号微偏（Card/Status 实为 29-36；导出止于 25 行）②§3.3 kebab→camel 示例是通则非所引行实例（pilot 回填真用例）③§2.1 目标 CSS 文件名留白待 pilot 定稿回填
- Task 8: complete (commits 6deded167, review clean)——稳态门双端接入；实现者纠正 brief 骨架顺序坑（freeze 必须在 settle 后紧贴截图，否则入场动画冻成幽灵态 46.7% 伪差，实证 ix-kb-list-create 46.74%→1.536% 复现基线）；基线均值 2.43% 持平、>1pct 抬升 0、noFreeze 零新增；重扫逐位复现=确定化（0 diff 验收前提达成）
- Task 8 minors (deferred): ①注释"只冻结过渡动画"对循环动画不严格（spinner 冻中帧即改变捕获帧），与 noFreeze 注释拼起来准确 ②动作页二次 waitForSteady 每次多发两次 evaluate（量级可忽略）
- 当前基线：60 项均值 2.4311%（17-16-56 轮）；login/register 轮播相位噪声仍在（freezeCarousel 管相位，文案差异属真实 DOM 差异，待 T14 login/register 页迁移时消化）
- Ruling R6: T9 揭示计划缺陷——页面级扫描含 shell 层（PlatformShell 顶栏/侧栏/会话列表/用户菜单/guide 条），shell 未迁移前页面 0.00% 不可达。裁决：①T9 验收口径修正为"页面自有内容区逐像素 0 + 残差 100% 归因 shell（报告 §5 证据）"；②新增 T9.5（shell 层同构迁移，含 PlatformShell.tsx/侧栏/guide 条）插在 T10 后、T11 批次前执行，完成后 agents/ix-agents-create 应回落 0.00% 作为其验收；③T11 批次 1 计划文本中的"共享层迁移"职责移交 T9.5，批次任务专注各自页面。若错误：shell 若有页面专属分支（某些页 guide 条不同），T9.5 验收时按页复扫暴露，届时补页级 delta。
- Task 9: DONE_WITH_CONCERNS（commit 7f497fec1，评审待做）——内容区逐像素 0（ix 弹层 0px；agents 残 66px 全部=guide 操作条等 shell 件）；7 轮收敛 1.415%→0.741% / 0.52%→0.185%；逻辑文件零改动；测试 2293 全绿；顺带落地台账 #10 所需 0.4.5 本地 sprite 镜像 + playbook 回填；台账新增 #8（tdesign Select 不透传 data-*，测试钩子改语义 className）
- Task 9: 评审 Approved + 2 Important（12 遍通读 10,704 行 diff）——R6 口径达标成立（hot_cells 独立佐证残差全在 shell 列）、台账 #10 机制被独立代码取证证实（pnpm 树 tdesign-icons-vue-next 实际解析 0.4.11→内部 CDN 指向 0.4.5，Vue 守卫拦截表只到 0.4.4，0.4.5 真实加载；本地镜像为正确对齐手段）→ fix round 1 已派（①AgentParserRules 原生 select 迁 tdesign ②恒真断言恢复）
- Task 9 minors (deferred): ③0.4.1 旧 sprite 镜像成死资产（~1MB，无源内引用，T9.5 或图标任务删除）④AgentsPage notice 死参数+两条重复 import ⑤jsdom harness ~40 行在两测试文件复制（Phase 3 建议提取共享 helper，否则复制 60 份）
- Task 9: fix round 1/5 (2 addressed, 0 open; commits 7f497fec1..24c8231f6)
- Task 9: complete (commits 6deded167..24c8231f6, review clean after 1 fix round)——parser select 已迁 tdesign（DOM 对齐 KBParserSettings embedded 态逐节点核实）、恒真断言换 spy+自校验（比原断言更强）
- Task 9 范围外发现（carry to T11）: apps/web/src/knowledge-settings/parserSettings.tsx:219 仍有原生 select（同型偏差，属 knowledge-settings 页任务）
- Ruling R7: T10 判 complete——其实质产出（台账 #7-#11、§2.1 CSS 文件名定稿 agents.td.css、props 换算回填）已随 T9 提交落地；计划 T10 剩余项（每页耗时参考、共享 jsdom test helper 决策）改由 T9.5/T11 派发上下文携带，避免空任务派发。若错误：playbook 缺耗时参考，无功能影响。
- Task 10: complete (per R7, 无独立提交；playbook 现状 = T7 初版 ba7f28f20 + T9 回填 7f497fec1/24c8231f6)
- Task 9.5: 评审 Approved + 2 Important（4 遍通读 3395 行）——三步验收/五硬约束全核实；guide 条假归因被修正（实为收藏星标 DB 化缺失，api-client userFavorites 写穿修复）；→ fix round 1 已派（①toggleFavorite eager-eval 竞态 ref 镜像+写路径测试 ②experts/market/analytics React-only 入口图标 fallback 退化恢复）
- Task 9.5 minors (deferred): ①userFavorites 双重 as cast（旧测试替身兼容模式可议）②水合竞态：水合前 toggle 会被整体覆盖吞掉（Vue 侧有 tenant 失效机制）③probe-shell.mjs 硬编码绝对路径 ④apiOwnerTag 同行三调 ⑤§0 标签归一化模式只记在 CSS 头注未进 playbook（Phase 3 沿用者会发现不了——carry to T11 派发）
- 派发记录：T9.5 实现者 agent_79e10a6d（fix round 1 进行中）；T9.5 评审 agent_7f47c6f1
- Task 9.5: fix round 1/5 (2 addressed, 0 open; commits 24fd4bcd6..f449ec6a3)
- Task 9.5: complete (commits 24c8231f6..f449ec6a3, review clean after 1 fix round)——ref 镜像全写点覆盖+4 写路径测试；三 React-only 入口独立 glyph+披露注释；复扫双 0.00%
- Task 9.5 复审 minors (deferred): ①双击回归用例从已收藏起步（旧实现偶然正确方向），未收藏起步 add→remove 才钉死旧 bug ②同 id 迟到回滚竞态（pre-existing 窗口极窄）③glyph 逐字节一致性采信（入口被过滤无视觉影响）
- Phase 2 收官：agents/ix-agents-create 双 0.00%；全站均值 2.06%；>1% 页 42 个（均为未迁移页面内容差）
- Ruling R8: 计划批次1"kb 详情簇"与批次3"documents/faq"重叠（kb-demo=文档视图、kb-faq=FAQ 管理器）——kb 详情簇（kb-demo/kb-faq/kb-wiki+tabs+ix-kb-settings/ix-kb-batch/ix-kb-listview/ix-kb-doc-detail）归批次 3；批次1=kb-list/orgs/apps/apps-connections/creatchat(+kb-demo-creatchat 随 creatchat)。批次1 拆两派发：T11a kb-list 单独；T11b 其余四小页合批（同型 SOP，per-page commits）。若错误：kb 详情容器与深视图强耦合需同迁时 T13 执行暴露再并。
- Task 11a: complete (commits f449ec6a3..f34177d99, review clean)——kb-list/ix-kb-list-create 双 0.00%（3 轮，radio 描边走 theme.css t-radio-button 平移根因收敛）；agents 回归双 0.00%；留守边界（编辑器深设置段+@weknora/ui 存量 4 引用）声明与 diff 一致；v-if/v-show 逐段区分被评为"后续 50 页范本水准"
- Task 11a minors (deferred): ①空间卡计数 fallback 语义差（Vue `?? '-'` vs React `|| '-'`，count=0 时 0/- 之分，一行修复）②编辑器 nav-badge（datasource 计数徽标）CSS 已平移但 DOM 未消费 ③创建成功后 Vue 停留编辑器 vs React 直接关闭（既有行为沿用，未登记行为差异清单）④死文件 empty-kb-svg.ts 未删（批次顺手清理）
- Task 11b: 评审 Needs fixes（7 遍通读 5299 行）——四页复刻/边界/回归/台账#14 全核实；Critical：ix-orgs-create 0.004% 豁免前提被证伪（tdesign-react Textarea count render-prop 是文档化 seam，单文本节点可复刻）→ fix round 1 已派（含 AppsPages 死代码/page.tsx 叠句注释/playbook 登记 Arial 全局规则三个 Minor 顺手项）
- Task 11b minors (deferred): ①apps.td.css §2 ConnectionsView 段随 apps 提交先行（两页共用一 CSS，无害）
- Task 11b: fix round 1/5 (1 Critical + 3 顺手全 addressed; commits dab217c48..2e2d8e7a9)——ix-orgs-create 0.004%→0.00%（count render-prop 单文本节点，库机制被复审独立核实）；台账 #13 成为批次 2/3 计数器残差处置先例
- Task 11b: complete (commits f34177d99..2e2d8e7a9, review clean after 1 fix round)
- **批次 1 完成**：kb-list/orgs/apps/apps-connections/creatchat + 全部 ix-* 交互项 0.00%（唯一边界外：kb-demo-creatchat 0.112% 归批次 3 收口）
- Ruling R9（批次 2 拆分）: T12 计 29 扫描项（23 settings section + 6 integration tab），单实现者不可行——按面板自然分组三派发：T12a Settings 壳+用户租户组（general/userprofile/mymemory/envvars/tenant/members）、T12b 模型资源组（models/ollama/weknoracloud/chathistory/memory/vectorstore/parser/storage/sandbox/skills/mcp/websearch）、T12c 系统集成组（system/system-global/runtime-queues/platform-api-keys/system-audit-log + 6 integration tabs）。每组内 per-section-group 提交。若错误：面板与 section 非一一对应时实现者报告边界，调整分组即可。
- 派发记录：T11b 复审 agent_a5936270
- T12a 结构性偏离欠账（评审 Important-3 立账，T12b/c/收尾需回收）: ①members 两张原生 table→tdesign Table（修复轮处理）②envvars 编辑器"无沙箱后端编辑态不可达"（扫描不可达，行为欠账）③mymemory 记忆行为 React 保留实现（parity 空态行不可见，行为欠账）④userprofile 密码弹层校验语义（Vue t-form :rules 逐字段行内 vs React 单链+toast——修复轮补 rules 或披露）⑤general 成功反馈 pushSettingsToast vs MessagePlugin（语义近似非同构，扫描不可达）⑥PopupPlacement 无 -start/-end 粒度=真库差无 seam（**正当台账条目**，应与 pagination 分开登记）
- Task 12a: 评审 Needs fixes（1 遍通读 6009 行+独立像素取证）——4/6 section 0.00%；general 豁免传导结构成立但取证数字两处夸大（13.9% 被写成 100%、末行 4,498px 实为 ~14,500px）；members 0.336% 无豁免依据（原生 table 未迁+pagination 有 totalContent seam 重演 #13 错误）→ fix round 1 已派
- Task 12a: fix round 1/5（表格迁移+totalContent+rules+取证修正，members 0.393% 带 4 项豁免主张）
- Task 12a: fix round 2/5（复审证伪 3/4 豁免：②遗留 CSS 钉高 ③漏平移 share-link-title ④空格独立文本节点——全修；members 0.393%→0.084% 仅剩 #16①772px 库级豁免；commits d1fba03aa..a8c9e4543）
- Task 12a: complete (commits 2e2d8e7a9..a8c9e4543, review clean after 2 fix rounds)——4/6 section 0.00%；general 5.439% 豁免传导（证据修正为可复现值）；members 0.084%（#16①豁免：Input autoWidth offsetWidth+1 取整 vs Vue 分数宽，代码级定位 Input.js:194-197 vs useInputWidth.mjs:37，无 #13 式 seam）
- Task 12a 复审遗留 minors (deferred): ①TenantUserProfileSections.tsx:345 双写注释一行未清（报告 §10 超报该子项——下一笔触碰该文件的提交顺手清）②#16 勘误中②"伴生差"表述已删但复盘记录见报告 §9/§10 ③04-28-57 证据目录含 5.896% 中间态（归档勿误引）④双端 font-synthesis 全局差（Vue auto/React none 疑 preflight）——T12b/c 字体类残差先排查此因
- 豁免台账累计（T16 全量验收时呈报用户）: general 套餐块传导 5.439%；members #16① autoWidth 772px=0.084%
- 派发记录：T12a（三次派发后成功）实现者 agent_0507e1f0（含两轮修复）；T12a 评审 agent_935bb2c1；fix round 1 复审 agent_63135cd1；fix round 2 复审 agent_541d82b3
- 环境事故（2026-09-22 午后）：用户中断导致 T12b 首派被取消（留 7+ 文件未提交部分工作）；后端 :8084 与 Vue :5174 进程死亡（仅剩 React :5175）；基础设施 docker 容器健康。恢复：Vue dev 后台重启✓；后端 go run 因**基线缺陷**启动 panic——commercial/benefits.go:104 EnsureSchema 用 SQLite 方言 AUTOINCREMENT 打 PG（42601 语法错误，规范迁移 versioned/000183 实为 BIGSERIAL）。
- Ruling R10: 后端 AUTOINCREMENT 是阻塞性基线缺陷（当前 main 的后端在 PG 上无法冷启动，之前靠旧进程续命）——最小方言分支修复（非 sqlite 走 BIGSERIAL 字面量分支，Mimosa 拦截拼接写法后改为双完整字面量），独立 commit b69b6980e，commercial 包测试 ok，标记最终评审覆盖。首次提交误并入代理暂存文件（09407a355 已 soft reset 拆分）。若错误：方言分支遗漏其他 driver 名（mysql 等）会退回旧行为，但 dev 环境=postgres 不受影响。
- 杂项（fix round 勘误路径）：失控代理误建的设计文档实为**仓库根目录 TDesign-DESIGN.md**（677 行，随 7261885c1 提交；此前误记为 docs/superpowers/plans/2026-09-22-tdesign-design-md.md）——Ruling R14 裁决无 brief 依据，T12b fix round 已 git rm（历史可经 7261885c1 找回）
- Ruling R11（服务自管）：parity 三端服务现由本会话后台进程承载（exec_ef31900f 后端/Vue dev 任务）——扫描任务照常可用；若再次掉线按 dev.sh 装配+go run 直启恢复（Air 的 watch 在缺失 paseo-adapter 目录时退出，不用于恢复）。
- Ruling R13（node 版本环境事故，2026-09-22 16:40 定位）：全量测试 63 处失败（documents 域 createPortal is not a function）非代码问题——默认 node 被外部会话从 v26.x 切到 v22.22.3，测试引导的 node:module registerHooks（react-dom 重定向）在 v22 下行为不同。验证：同文件 v22 失败 38/44、v26.4.0 通过 44/44（且基线 a8c9e4543 同样复现，排除迁移提交）。处置：**本迁移所有 node 命令统一用 `~/.nvm/versions/node/v26.4.0/bin/node`（或先 export PATH=~/.nvm/versions/node/v26.4.0/bin:$PATH）**；后续代理派发指令必须携带此约束。若错误：v26.4.0 与当初 v26.7.0 有差异处会在相应测试暴露（目前 44/44 全过）。
- Task 12b: 评审 Approved（1 遍通读 8513 行 + 终态重扫 8 键独立证实 + 全量 2297 复跑）——12/12 键达标（11 键 0.00% + models 28px #16 亚像素家族豁免成立）；僵尸提交复刻抽查 10/10；@weknora/ui 零新增；frontend/ 零改动 → fix round 已派（①#16 勘误：28px 实为 model-test play-circle 图标 AA 权重差非 doc-link（原取证链跑错元素）②Ruling R14：TDesign-DESIGN.md git rm（根目录僵尸自生物无 brief 依据，历史可找回）③chathistory .t-is-checked 断言恢复④viewer 语义微差登记⑤Promise.all 残迹）
- **批次 2 第二组完成**：settings 12 键全达标。豁免台账累计：general 套餐传导 5.439% / members #16① 772px=0.084% / models #16AA 28px=0.003%
- 派发记录：T12b 实现 agent_3a8ae88d(僵尸1)×2 + agent_不明(僵尸2, models) + agent_680367b5(parser) + agent_cbbcbf1c(sandbox) + agent_517b7719(skills, fix round 进行中)；T12b 评审 agent_d4f118e2
- 派发异常（2026-09-22 15:54 后发现）：T12b 两次派发对 controller 返回 1302 限流错误，但**代理实际已启动并工作**——HEAD 从 b69b6980e 推进至 7465e0812（settings-memory/ollama/weknoracloud/mcp/resource 组 5 个迁移提交 + 1 个 docs 提交 7261885c1），工作树仍有 ModelSettingsPanel/SettingsPage 进行中改动。处置：不重复派发（冲突风险），轮询 git log 稳定+报告文件出现视为完成；该代理的 agentId 未知（派发结果丢失）。根目录 TDesign-DESIGN.md 即该代理所建（已随 7261885c1 提交；fix round 按 Ruling R14 git rm，已删除——初记路径 docs/superpowers/plans/… 系笔误）。教训：1302 错误返回 ≠ 派发失败，必须先查 git 状态再重试。
- T12a 派发事故（2026-09-22 凌晨）：首次派发子代理 600s 无活动被回收，留下未提交部分工作（SettingsPage.tsx 重写 343 行变更 + settings.td.css 新建 + 测试更新，无 commit）——服务三端在线正常，判定为代理自身停摆非环境故障；重派新实现者，指令含"先审计存量未提交工作再续做"。
- Phase 1 完成标志：T1-T7 全 complete，基建链路（依赖/CSS/token/守卫/adapter/locale/playbook）齐备
- Ruling R15（并行化重排，2026-09-22 21:35，用户指令）：Wave1 三流=S1 settings系统管理5节+集成6tab（主 checkout 继承 T12c 僵尸 5 文件）/ S2 chat 家族（worktree .worktrees/tdm-s2-chat，vite :5272）/ S3 kb详情簇+documents+faq+wiki+knowledge-settings（worktree .worktrees/tdm-s3-kb，vite :5273）；所有权互斥，S2/S3 扫描 PARITY_REACT_URL 指向 worktree 端口，测试一律 v26。Wave2（合并后）=S4 login/register+redirects+dev-markdown+404 / S5 未扫清单页批+parserSettings select / S6 编辑抽屉收编（T15 硬前置，抽屉仍引用 @weknora/ui）。Wave3 串行=T15→T16→T17。主会话评审+集成。若错误：所有权隐性交叉集成时暴露，revert 单流重做。
- Wave1-S2（chat 流）: complete-as-stream（worktree 提交 23185f175，评审 Approved，fix round 进行中：i18n 三分支/死按钮/遗留清单三面）——chat 0%×3轮、ix-chat-header-menu 0%、creatchat 回归 0%；ix-chat-mention 0.054%（497px）逐像素归因 PlatformShell 壳域真实回归（swap 图标缺失+租户行排版），评审定性"不可豁免"→ 排队主 checkout 壳层修复轮（S1 完成让出主 checkout 后执行，勿在 S1 运行时碰）
- S2 评审衍生排队项: ①扫描器 ix-chat-mention 条目补 clickCss 兜底（composer @ 按钮）——名义目标面两端均未覆盖 ②views 手写 tdesign DOM 模拟层（~40 行 SpriteIcon/ToolbarButton）架构口径：受控偏离、文件头有注、禁止扩散，views 获准 tdesign 依赖或消息面上移 apps/web 时删除（台账待登记）③hljs/citation pill 静态面仅因 fixture 缺内容不可达——补 rich fixture 后优先复扫
- 派发记录：S2 实现者 agent_8ac96fbb（fix round 进行中）；S2 评审 agent_21d43477
- Wave1-S2: fix round 1/1 完成（48a4eea78：i18n 五语言三分支/检索完成按钮接线复活引用抽屉/遗留清单三面入账/空态键补齐/占位注释）；复核 chat+ix-chat-header-menu 双 0.00%。S2 流终态=complete（worktree 分支 tdm/s2-chat @48a4eea78 待合入主分支，等 S1 让出主 checkout）
- Wave1-S1（T12c 系统集成流）: DONE（5 提交 2878d3259..48870ee55，主 checkout 直提交）——9×0.00% + im/embed 0.002%（14px 台账#18 chevron AA 豁免：symbol 字节级一致+headed 取证+无 seam 论证，评审重点审）；台账#11 反向应用修复 52px；auto-scan 加三处确定性归一（system/runtime-queues 时钟、integration 端口——评审核查对称性）；settings-general 5.439% 复活=后端新二进制激活 /commercial/summary 使套餐卡渲染（与 T12a 豁免口径一致，非新残差）；评审进行中（agent_b7054ef5）
- Wave1-S2 chat 流已合入主分支（merge 3ce454e06，无冲突）
- 壳层修复流已派（agent_0fc735c8）：PlatformShell 租户行 swap 图标+灰阶回归 + ix-chat-mention clickCss 兜底（文件域与 S3 worktree 并行不冲突）
- Wave1-S1 评审: Approved（台账#18 三重独立复验含 CDN symbol 979,316 字节逐字节比对；auto-scan 三处归一确认双端对称同刻冻结不造假；settings-general 5.439% 三重吻合 T12a 口径；僵尸继承 4 处修正全部落地）——1 Important 排队（SystemAuditLogPanel 扫描可见域 header/空态残留 Tailwind+Card/Status 未声明→修复轮按 playbook §4 真迁而非补声明），等壳层流让出主 checkout 后派（避免 index 竞争）
- S1 评审 minors (deferred): ①#18 编号插在 #16/#17 间破坏顺序+坐标只引 embed（im 实为 x423-430）+"全集4703"系运行时计数易误读 ②#18 headed 取证工件未持久化（证据树被清理）→豁免条目注明 ③系统管理组 3 提交超 brief"一~两提交"（僵尸继承背景可谅，注明偏离）④SettingsPage PARTIALLY_PORTED 四死条目 ⑤IntegrationsRoutePage 死 try/catch 守卫——以上随 S1 修复轮顺手回收
- 派发记录：S1 评审 agent_b7054ef5
- 壳层修复流: complete（429bb6474）——三根因（swap 门控条件 memberships>=1 / 角色前缀 12px icon / svg 439 变体）菜单态 0.00%；ix-chat-mention 扫描目标修正后暴露 chat 弹层真实差异 1.471%（弹层无搜索框/空态/tooltip/12px 高差）→ S2 修复轮回 worktree 并行处理中；S1 修复轮（audit-log 真迁+5 Minor）主 checkout 并行处理中
- Wave1-S3 存活推进中：kb-documents（25fdbd86c）、kb-wiki-tab+容器铬（8b7eefabe）已落，faq/kb-demo/ix 项进行中
- Wave1-S3 前段: PARTIAL DONE（7/10 id 0.00%：kb-demo/kb-wiki/两 wiki-tab/kb-demo-creatchat（批次1遗留收口）/ix-kb-batch/ix-kb-listview；提交 25fdbd86c kb-documents 全量 + 8b7eefabe wiki-tab+容器铬）——剩余 kb-faq 0.17%（FAQEntryManager 5650 行）/ix-kb-settings 0.216%/ix-kb-doc-detail 1.895%（doc-content 3331 行）→ 续作代理已派（agent_960fa623，同 worktree）；关注点：PlatformShell 2 行越界豁免已声明（Vue menu.vue:434 事实源）、parserSettings select 前任 jsdom 不稳回退（续作带 Select 测试模式指引）、全量测试因 CPU 争用未整跑（续作终验强制）
- Wave1-S1: fix round 完成（89692860e：audit-log 真迁——Vue 类名结构/t-empty/t-alert、@weknora/ui 引用清零、§18 平移、死 keyframes 删除；#18 勘误+顺序恢复；报告偏离注明；四死条目+死 try/catch 清理）——settings-system-audit-log 0.00% 维持双复扫、2297/2297 绿。**S1 流终态 complete**（5+1 提交，评审 Approved+修复轮毕）
- Ruling R16（用户指令，2026-09-23）: 全部任务完成（含 T15/T16 三轮验收/T17 最终分支评审）后将 feat/tdesign-react-migration 合并回 main 并 push——合并+推送已获用户显式授权，无需再次确认；推送前确保 T16 三轮全量验收通过+最终评审 findings 处置完毕。
- Wave1-S2 mention 修复轮: complete（aa92ec93a，已二次合入）——弹层态 7 轮收敛至 chat 域 0px（裁侧栏取证）；tooltip 发丝影/箭头/锚定几何实测复刻；ix-chat-mention 0.054% 残余=shell 域"近7天"行 12px running spinner 标记缺口（Vue menu.vue sessionActivity 轮询标记 stuck-incomplete 会话，React PlatformShell 无）→ 壳层小流#2 排队；环境坑入账：5272 端口被占+FRONTEND_BACKEND_URL 须写 http://localhost:8084（纯 host:port 缺 host 会解析数字 IP）
- Ruling R17（用户指令，2026-09-23）: 合并回 main 之后增加**双端功能一致性验收**——启动后端(:8084)/Vue(:5174)/React(:5175)，用 ~/.weknora-func-creds.env 的账号（wu18349270334@gmail.com，密码在文件，禁写入源码/输出）对两个 web 做功能实测，要求功能完全一致。测试矩阵（登录会话/导航壳/聊天发送接收流/KB 列表与详情/设置读写/agents CRUD/集成页跳转等核心流），双端同流程执行对比行为结果（像素已由 60 项扫描覆盖，此轮验功能语义）；产出功能一致性报告归档 evidence。执行时机：main 合并后、push 前或后均可（属 T17 收尾延伸，记为 T17.5）。
- Ruling R18（双会话分裂统一，2026-09-23）: 并行 codex 会话 75 分钟前把主 checkout 从 feat 切到 main 做 Pass B（3cecf7465/29c1e5635 docs），致我方 S2 二次合入（b1a3d6dd8）与壳层#2（c6d59d83e）落在 main、S1 侧留在 feat——拓扑分裂。处置：主 checkout 让渡给并行会话；建 .worktrees/tdm-int 挂 feat 分支，merge main 进 feat（e48804afe，3 冲突 hunk：session-activity 常量取 main 侧、外链箭头 path 取 feat 侧 429bb6474 验证版）；此后我方全部工作在 tdm-int 及各流 worktree，最终合并才碰 main。
- 统一后验证：platform 240/240；agents/chat/settings-system-audit-log 全 0%；ix-chat-mention 0.001%（三连同值）
- Ruling R19: ix-chat-mention 0.001% = 单簇 9px @ (466,666)（弹层底缘/composer 交界 AA 级，diff PNG 像素级定位，三连确定性，热格稀疏不可见）→ 登记 #16 亚像素家族豁免；T16 三轮全量验收复验其稳定性，若届时形态变化再升级归因。
- 壳层小流#2: complete（c6d59d83e 落 main，经 R18 统一回收）——sessionActivity 数据通路全新补齐（session-activity.ts 转移规则+5s 轮询+清除）；已知残差：同会话发起新生成标记慢于 Vue 即时 watcher（需 chat 域事件，超壳层许可）记欠账；归因更正：0.054% 实为租户面板行图标族（429bb6474 同源，非 spinner）
- Wave1-S3 续作: complete（3 提交 9ea6e6895/a5da455a9/bff725eda——faq 整域+4 id、kb-settings+parser Select、doc-detail）——8/10 字面 0% + 2 引擎伪影候选豁免（ix-kb-settings 0.003% 弹窗四角 radius AA、ix-kb-doc-detail 0.005% fullscreen 图标栅格相位，均 computed 一致+逐端自比 0px 取证）；全量 2296/2296（计数比主树少 1 待评审查明）；关键发现：Vue 图标几何实际来自本地 sprite 0.4.1 非组件树 / Chrome input 垂直居中两层复刻 / 旧栈 :root font-synthesis:none 是 italic 残差根因（Phase 4 候选）/ 旧栈 token 作用域变量覆写收口 → S3 全分支评审进行中（agent_10fa13d0）
- Wave2 三流已发（基于 e48804afe 各自 worktree）：S4=tdm/w2-s4 :5276（auth/dev/404）；S5=tdm/w2-s5（未扫页批，knowledge-settings 已更正为跳过——S3 分支已修）；S6=tdm/w2-s6（抽屉收编，@weknora/ui 引用清零=T15 硬前置）
- Wave1-S3 评审: Approved（8 id 字面 0% 独立复算证实；两伪影豁免三重独立验证无 seam：ix-kb-settings 31px 四角弧线 AA/ix-kb-doc-detail 47px 栅格相位；测试 2297→2296=FAQ 测试有意合并非丢失；sprite 两版 d 发现证实）→ 2 Important（台账#10 补注+豁免留档）+Minor-1/2 回收小流已派（agent_18166e0f，tdm-int）；Minor-5 PlatformShell 越界已 controller 追认（有声明+Vue 事实源+可回滚）
- Ruling R20: S3 合并误落 main（c91b63ebf，shell cwd 复位所致）——不回滚（内容 destined 汇合+并行会话静默+reset 风险更大），feat 侧以 merge main 补同源（050607891 零冲突）；教训：跨 worktree 操作必须 git -C 显式指定
- Wave1 全流统一完成（feat@050607891）：验证 platform 240/240+kb-faq/kb-demo/agents/chat 全 0%+两豁免稳定 0.003%/0.005%
- 豁免台账累计（T16 呈报）: general 套餐传导 5.439% / members autoWidth 772px 0.084% / models play-circle AA 28px 0.003% / im+embed chevron AA 14px×2 0.002% / ix-chat-mention 弹层交界 9px 0.001% / ix-kb-settings 弧线 AA 31px 0.003% / ix-kb-doc-detail 栅格相位 47px 0.005%
- S3 评审回收: complete（80c41cf6f——台账#10 两版 d 判例补注（取材以 frontend/public/tdesign-icons/0.4.1 sprite 版为准）、#19/#20 两伪影豁免落档（含 2.5px 补偿/双弧线 hack 挂注）、FAQ label 断言恢复、注释笔误修正；103 测试绿）——S3 流全链路收官
- Wave2-S4: DONE（tdm/w2-s4 三提交 3166d3bca/0af22b185/8a99c94e7）——redirect-system/dev-markdown 0%；redirect-integrations 0.002%（14px 归 S1 域 chevron，合入后复扫确认）；login/register 页面本体 0 但 animated-bg 装饰动画冻结相位非确定残差（run 间 275↔1387px 波动）——**不满足豁免确定性标准**，已派扫描器相位同步小流（agent_281238c7，tdm-int）+ S4 评审（agent_e5a45fb9，重点含台账编号冲突裁定）；S4 欠账：packages/views/chat/markdown 缺 repairFlankingEmphasis（chat 生产路径 ***x*** 两端渲染不同，扫描不可达——登记后随 chat 域或 S4 修复轮处理）
- Wave2-S4 评审: Needs fixes（仅文书，代码零改动全达标+独立复算通过）——"页面本体 0"取证成立（PIL 复算 729px=724 animated-bg 相位+5px pill AA 量化）、#19 logo 修复跨 4 轮稳定、dev-markdown 泄漏行为复刻事实链全证、复刻抽查 3 处全过 → fix round 已派（台账编号改 #21/#22+taxonomy 精化+repairFlankingEmphasis 跨域欠账登记）
- Wave2-S4: complete（3166d3bca/0af22b185/8a99c94e7 + 修复轮 7d79531c9，已合入 feat 531506486——playbook 冲突按序保留 #19-#22 四行）——5 id 达标（login/register 页面本体 0+相位残差待同步流+5px pill AA #16 族；redirect-integrations 14px 待合入后复扫；404 无 Vue 对照走单测目检）
- Wave2-S5: DONE（tdm/w2-s5 五提交 27ed779df..4c219cabe）——analytics/commercial/configuration 组件层换装（Vue 无对应路由实测确认）、data-sources 真同构（Sheet→Drawer）、craft 手写 DOM td.tsx（frontend 无对应，简报"有"系误记；views-chat 模拟层同判例）；@weknora/ui 十目录归零；2306+shared 981 全绿；剩余非门槛：embed 完整同构未做（组件层已净）/data-sources 编辑器 Drawer vs Vue 居中 dialog（388 行测试重写，建议独立 ticket）/各域 Tailwind utilities 留 Phase 4 → 评审进行中（agent_8fa36791）
- S7 合并完成（754bfb5d0，9 文件 34+11 hunk 并集，286 域测试+tsc 全绿）→ T15 前全量基线扫（05-59-29 轮，:5175 已清理为唯一服务 tdm-int）**抓到 11 项合并回归**：集成六 tab+redirect 1.78-24.25%（全军覆没）、kb-wiki-tab-wiki 3.23%、ix-kb-doc-detail 4.11%、settings-system 0.07%；均值 1.54%。诊断方向=u.css 语义类未命中 T* DOM（orgs 教训放大）/import 图缺口/并集丢内容 → 回归修复轮已派（agent_5825c6ba 续）
- S7 合并完成（754bfb5d0，9 文件 45 hunk 并集，286 域测试+tsc 全绿）→ T15 前全量基线扫（05-59-29 轮，:5175 已清为唯一服务 tdm-int）**抓到 11 项合并回归**：集成六 tab+redirect 1.78-24.25%、kb-wiki-tab-wiki 3.23%、ix-kb-doc-detail 4.11%、settings-system 0.07%；均值 1.54%。诊断方向=u.css 语义类未命中 T* DOM（orgs 教训放大）/import 图缺口/并集丢内容 → 回归修复轮已派（agent_5825c6ba 续）
- S7 合并回归修复轮: complete（fffb0f5f1，22 文件）——8 项回基线（cli/claw/chrome/api/kb-wiki-tab-wiki 全 0），全量均值 0.1%（仅 general 豁免>1%）；根因=codemod 三类 bug（注释内嵌 */ 致 @layer 整块解析失败/单边 border 未重置 style 98 处/类名拼接缺空格 12 处）——合并后 css 首次真正生效而暴露；三项环境归因：general=商业卡（豁免）、system 0.07=版本后缀 VITE define、doc-detail 0.352=fixture 解析失败态（后端缺 anydoc 库所致，基线未覆盖路径）→ S6+S7 联合评审进行中（agent_52c9c64c，重点=三类 bug 漏网全仓扫描+u.css 层叠+环境项处置方案）
- S6+S7 联合评审（重派）: Needs fixes——**Critical-1：11 个路由文件仍 import @weknora/ui**（documents×4/KnowledgeGraph/GraphSettings/CommandPalette/InvitationInbox/Join/Onboarding/NotFound——此前"全仓归零"结论有误，删包即断构建）+ Important×7（拼接漏网 2/border 误译 3+误删 1+死 token/无效 repeat 2/残留 token 15+3/sr-only 平台缺口/import 顺序 5/S6 抽屉截图欠证）+ 环境项处置方案（VITE_FRONTEND_COMMIT 对齐/anydoc 恢复）→ 两流已派：T15 前置阻塞修复流（agent_bac6e3ec，大）+ 环境对齐流（agent_bbb8e6e7，小）
- 评审 minors (deferred): 死规则 .wk-kd-166/kb-list .wk-kb-des-* 14 处孤儿化（Phase 4 清理）；join !px-[15px] 入 T15 处置清单
- 环境对齐流: complete——settings-system 0%（4a92063ac 版本后缀双层归一）；anydoc 恢复（48s 构建+-tags anydoc 后端+mock-llm 环境修复，两 fixture 回 completed 态）；doc-detail 0.352→0.565%：成功态暴露两真实页面缺口（①chunk-count 徽章缺失 doc-content.vue:1816 vs DetailPage.tsx:832 ②wk-kdd-75 codemod 硬编码 #66758b 应为 var(--td-text-color-secondary)+span 高度 17v22）→ 已追加给 T15 阻塞流（同文件域）。基线口径变更：doc-detail 以成功态为新基线。
- T15 前置收尾: complete（71773d995——onboarding 组 WkDialog 解 jsdom 假挂/抽屉证据 3 张归档/tmp 清理；四验证绿：2305 pass+tsc+build+双 grep 0；扫描七页六 0%，唯 doc-detail 0.352% 闸口未达）→ 字形专项流已派（agent_a219cf17，强假设=font-synthesis 根因家族，headless 计算样式全量对比归因）
- doc-detail 字形专项: complete（bb546b169，0.352%→**0%×3 轮**）——真因=codemod 硬编码旧栈 hex（#66758b/#172033/#edf0f5 vs Vue rgba(0,0,0,.6)/.9+#e7e7e7），font-synthesis 嫌疑排除（计算值双端一致）；连带发现 ix-kb-settings 漂至 0.173%（≠#19 豁免 0.003%，疑同类 hex）→ 全仓旧栈色值清扫流已派（agent_4bca6fe3）
- 旧栈色值清扫: complete（0603053b2——18 文件 253 处 hex→tdesign 值带 Vue 对照注释；ix-kb-settings 0.173% 真因=.kb-id-value 行盒 15vs19.5px（缺 line-height:1.5+mono 栈），回 0.003% #19 底噪；六页+回归全绿；存疑 16 处无 Vue 同位规则未改已列清单）→ **T15 放行，删旧栈流已派（agent_9bc9249d）**
- T15: **complete**（d9d2ec4c5/5d2a86c28/2d04d98e6/c3e5eca4f 四提交——packages/ui 删/tailwind 管线移除/补丁残余+join 15px/C3 勘误）。五验证全绿；60 项首轮均值 0.09%、豁免 9 项全核对、其余全 0%。关键工程：残留 30+ 处 load-bearing utility 超账本 6 处全语义化；apps/embed 相对路径直引 packages/ui（grep 扫不到）本地复刻；存量 CSS 70 处 var(--color-*) 依赖 @theme 块→styles.css 平移为 :root 事实源；二分法三 worktree 对照定位两处自引入回归（rq 8.242/doc-detail 0.135）并归零 → T16 验收流已派（agent_7da20dcb，补 2 轮+体积对比+证据归档）
- T16: **PASS**（三轮 14-28/14-50/15-00 均 60/60、avg 0.09% 逐位一致零波动；豁免 9 项三轮逐位复现+pixdiff 重算+bbox 吻合台账；体积：JS 净中性、CSS gzip -1.9KiB；验收文档 290b7bcea 归档 final-acceptance/）
- T17 收尾进行中：spec 已标记已实施（tdm-int 提交）；守护 automation 已建（automation-20291a7f，每 30 分钟只扫描告警不修复，旧 auto-fix 41aaa5e6 保持 paused）；最终全分支评审运行中（agent_ccdd20e0）
- T17 最终评审: **✅ 合并就绪**（零 Critical/零冲突面 merge-tree exit 0/欠账 9 条全"随合并"无一 load-bearing/独立复验 tsc+89 用例绿；R5/R20/R21 闭环，R13=治理观察项）
- **R16 执行完毕：feat(227 提交) → main 合并 898864c07 → 已 push origin/main**（65b7b07d5..898864c07）
- T17.5 双端功能测试运行中（agent_95af04f6，:5175 已切至合并后 main，UI 表单登录真实账号）
- 评审遗留跟进清单（合并后 1-2 提交回收）：①desktop 死 alias 5 行 ②styles.css 2 孤儿注释+重复头注 ③playbook 补 wk-legacy 登记行 ④CI node 版本统一（engines>=26 vs CI 24/22）⑤chat 域排期 repairFlankingEmphasis ⑥codex/* 旧分支与 worktree 清理
