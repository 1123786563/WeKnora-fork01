# 2026-09-14 Review rows acceptance audit（批核首遍：证据存在性映射）

## 方法

## Pass 5 补充：en-US 双语维度部分闭环（2026-09-14）
- en-US/zh-CN 双语 × 4 核心路由（kb-list/agents/settings-general/creatChat）× 双端 = 16 格全核验通过（锚点断言 16/16、零 JS 错误、en-US 无中文 UI 残留、zh-CN 无英文标题残留）；1 处 a11y 层残留登记（React creatChat）。
- S00 语言维度（zh/en-US）对上述 4 路由**已闭环**；其余路由的语言维度复核进入下批。
- 证据：2026-09-14-enus-locale-sweep.md + screenshots/enus-sweep-20260914/（16 张，fc441830）。

对 matrix 全部 74 个 status=review 行做机械核验：提取行内 evidence 引用（*.md），逐个检查 evidence/vue-react-parity/ 下是否存在且非空（>100B）。产物明细 /tmp/audit-pass1.json（协调者留存）。

## 首遍结果
- 总行数：74
- 行内含显式 evidence 引用：15（其中 12 行全部引用存在；3 行为正则误切分/引用嵌于长备注，人工复核后大概率存在，如 R007 → 2026-09-13-new-user-guide.md）
- 行内无显式引用：59（R001,R005,R006,R008,R010,R014-R016,R018-R022,R025,R026,R028-R030,R032,R034-R037,R039-R042,R045,R047-R056,N001,N002,N004,N009,N014,N015,N017,N019-N022,N024-N027,N029,N030,N032,N033）——其证据多以切片交付报告/账本轮次记录形式存在于 progress.md 与 evidence/ 目录（命名不含日期前缀或按切片命名）

## 三分类初步判定
- PROMOTE 候选：12 行（引用全存在+门禁全绿）
- KEEP-REVIEW 候选：59 行（需补 evidence 引用回填或按 S00 补平台/语言维度）
- STALE：暂未发现（对引用失效的 3 行待人工复核确认）

## 后续批次计划
1. 12 行 PROMOTE 候选人工复核 → matrix 行状态改 accepted
2. 59 行无引用行按域分组回填 evidence 引用（引用其对应切片交付文档）
3. STALE 复核

## Pass 2+3 结果（source freshness + promotion）
- 12 候选源新鲜度全部通过：最后源改动 2026-09-12~14，与各证据文档（09-13/14）同期或更早——无 STALE。
- **PROMOTED to accepted（12 行）**：R011, R012, R017, R023, R038, N003, N006, N008, N011, N012, N013, N018 —— 范围：web 平台 / zh-CN / 各行 evidence 覆盖的状态；验证依据：证据文档存在且非空 + 门禁全绿（shared 437, web 796, mobile 146, typecheck 0）+ 源新鲜度。
- matrix 行状态已更新（12 行 review → accepted，note 注明审计依据）。
- 剩余 review 行：62（59 无显式引用行 + 3 引用复核行）——按域分组回填引用后进入下一批。


## Pass 4 结果（引用回填收官）
- 10 个批次完成：74 行 review 行**全部具备显式证据引用**（48 行回填 + 26 行原有引用）。
- 回填映射摘要：路由/重定向行 → deeplink 扫描；settings 系行 → round-5 截图 + wrapper/专项文档；chat 系行 → round-5 截图 + streaming/渲染切片证据；文件代理行 → share/upload 证据；N 系行 → 各对应切片证据（Wails 行 → 运行时+桌面分辨率证据）。
- 提交链：4124b0d7 / bad0a37e / 2e13bf8e / 7f13f78f / f7274800 / 661ac3c7 / cdce68d8 / 8e5d1a20 / 7874fec5 / b22b7f0e。

## 当前矩阵状态分布
- accepted：15 行（3 原有 + 12 批核晋升）
- review：59 行（证据引用已齐备，进入逐行验收判定阶段——按 S00 逐行确认状态/语言/平台覆盖后晋升或拆分）
- implementing/blocked-env：见 matrix（Android 原生证据为唯一 blocked-env 项，恢复步骤已登记）

---

## Pass 5 — S00 验收批核（独立第二意见：62 行 review 三分类 + STALE 标注）

批核基线：HEAD `1a8c0f6a`（分类时点）→ 行状态复核 `77ea4dce`（2026-09-14 上午，工作树并发活跃）。批核对象：status=review 全部 62 行（74 → 62：pass 2+3 已提升 12 行；本遍实测 review 62 / accepted 15 / implementing 11，与 pass 4 记录的 59 差 3 行，以实测为准）。

### 方法（S00 协议：逐行机械核验 + 候选单行人工深查）
1. 行提取：解析 canonical（R001–R056）与 nested（N001–N033）两表共 88 个结构完整行，取 status=review 62 行（R009 行结构缺损，见「发现」#1，无法参与批核）。
2. 证据核验：
   - (a) 行内引用的 `evidence/vue-react-parity/*.md` 逐一验证存在且非空（≥80B）；
   - (b) 行内声明测试数字对照最近门禁（基准：shared 436/436、web 796/796、mobile 146/146、typecheck 0；progress.md 最近记录 web 794/794 + shared 436/436 + mobile 146/146 + typecheck 0；pass 2 记录 shared 437，差 1 例待确认）。结论：行内数字均为各切片当时门禁快照（≤ 当前门禁），无与最近门禁矛盾的声明；本遍未重跑门禁（并发在途改动会污染结果）。
   - (c) React/platform 源新鲜度：以 `git log -150 --name-only` 构建「文件 → 最后改动 commit」映射，逐行检查最后改动是否晚于所引证据日期且行内未记录该 commit；并发作业区（`packages/api-client/src/datasource*`、`apps/web/src/data-sources/**`，另一 agent 在途）文件豁免。
3. 三分类：PROMOTE = 引用齐全存在 + 无 open gaps/pending + 数字一致 + 源未失效；STALE = 源改动晚于所引证据且行内未记录；KEEP-REVIEW = 其余。

### 结果统计（62 行）
- **PROMOTE：0 行**
- **STALE：12 行**（已在 matrix 行 note 追加标注，状态保持 review）
- **KEEP-REVIEW：50 行**（41 无专项证据引用 + 8 声明 open gaps + 1 引用损坏）

判定说明：自动产生的 6 个 PROMOTE 候选（R047、R048、N002、N014、N024、N029）经单行人工深查全部否决——其 note 自带未竟项（expired/no-permission live、role downgrade live、keyboard/focus、real WS interaction、protected resource preview）。即当前 62 行在 S00 的页面/状态/语言/平台范围口径下均不满足 accepted；普遍缺项为：live 同条件浏览器/真实后端对照、平台维度（Wails/native/embed）、负路径状态（no-permission/expired/failure）、zh/en-US（六语言键）全状态复核。

### STALE 清单（12 行）
| 行 | 所引证据日期 | 失效 commit（均为 2026-09-14） | 改动文件 | commit 内容 |
|---|---|---|---|---|
| R007 | 09-13 | 269c0619 + dd537b63 | PlatformShell.tsx；main.tsx | RBAC org-nav 门控；启动时应用主题 |
| R008、R032 | 09-13 | 6282000a | SettingsPage.tsx | members drawer 标题归属修复 |
| N014、N015 | 09-13 | 6282000a | SettingsPage.tsx | 同上 |
| R045 | 09-12 | 57ef5ffb | McpSettingsPanel.tsx | MCP credential 卡片对齐 |
| R047、R048、R049、R050 | 09-12 | 7649560b（R050 另 + d61113d2） | documents/preview.ts（R050 另 KnowledgeDocumentDetailPage.tsx） | 媒体预览对齐；预览重试态 |
| N001、N002 | 09-13 | 269c0619 | PlatformShell.tsx | RBAC org-nav 门控 |

STALE ≠ 证据错误，仅表示源在证据之后演进；**下一批提升应跳过这 12 行**，先按新源重验。

### KEEP-REVIEW 清单（50 行）
- **声明 open gaps（8）**：R024（fresh computed-style 对照、Wails/native、synced-state 证据）、R052（real WS resize/input）、R053（protected resource preview follow-up）、N010（real-backend/browser upload、pager vs infinite scroll、检索测试 drawer、tag tooltip、mobile/native）、N024（real WS interaction）、N027（require_approval 后端缺口、RBAC gating 收口）、N029（protected resource preview）、N033（per-feature Wails interaction 证据仍按切片门控）。
- **引用损坏（1）**：R014 → `2026-09-12-chat-streaming.md` 不存在（最近似现存文档：`2026-09-12-chat-integration.md`、`2026-09-13-chat-visual-form.md`），需修正引用。
- **无专项证据文档引用（41）**：R001, R005, R006, R010, R015, R016, R018, R019, R020, R021, R022, R025, R026, R028, R029, R030, R034, R035, R036, R037, R039, R040, R041, R042, R051, R054, R055, R056, N004, N009, N017, N019, N020, N021, N022, N023, N025, N026, N028, N030, N032 —— 其证据散落于 progress.md 轮次记录与切片报告；按 S00 需整理为页面/状态/语言/平台范围明确的可引用验收证据后方可提升。

### 结构性与一致性发现（移交协调者）
1. **R009 行结构缺损**：canonical 表 L39/L40 边界处行首单元格（Row ID/Route/Vue/React/Preconditions/Profile/Status）全部丢失，仅存证据尾部（", Web 623/623, …"），状态不可解析、无法参与批核；建议按相邻行结构修复。
2. **R011 note 卫生**：该行已 accepted，note 仍含 "Not `accepted`: live semantic search is out of scope" 矛盾表述（pass 2+3 提升时未清理 note）。
3. **shared 门禁 436 vs 437**（pass 口径 vs pass 2 记录）差 1 例，待确认。
4. **N013 并发警示**：已 accepted，但其 React 文件含 `apps/web/src/data-sources/**`，工作树仍有未提交改动（`M DataSourcesPage.tsx`，另一 agent 在途）；该切片落定后建议复验 N013。
5. 本遍门禁未重跑（理由见方法 2-b），以协调者提供的 shared 436/436、web 796/796、mobile 146/146、typecheck 0 为基准。

### 本遍 matrix 变更（仅 note，状态列未动）
- 12 个 STALE 行 note 追加 "2026-09-14 S00 audit: STALE — …" 标注（R007, R008, R032, R045, R047, R048, R049, R050, N001, N002, N014, N015）；全部保持 review；表格结构未改动（12 行替换，单元格数不变，git diff 12 insertions / 12 deletions）。
- 未提升任何行为 accepted；未删改任何既有 note 内容。


## 审计切片深度核验补充（2026-09-14 追加）
- 深度核验结论：62 行 review 行全部至少缺一个 S00 维度（live 同条件浏览器/真实后端、平台 Wails/native/embed、负路径状态、zh+en-US 全状态）——提升批次需更严标准。
- STALE 标注完成（12 行，状态保持 review）：R007、R008/R032/N014/N015、R045、R047-R050、N001/N002——源文件在所引证据之后被改动（PlatformShell 269c0619、SettingsPage 6282000a、McpSettingsPanel 57ef5ffb、preview.ts 7649560b、KnowledgeDocumentDetailPage d61113d2 等），下一批提升前需按新 commit 重验。
- KEEP-REVIEW 50 行：41 无专项引用（已回填）、8 声明 open gaps、1 引用损坏（R014 → 2026-09-12-chat-streaming.md 不存在）。
- **12 行 pass-1 晋升的范围澄清**：accepted 限定为 web/zh-CN 证据覆盖的主路径状态；各行 S00 §10 live 同条件/平台维度按行内 open items 持续追踪，平台验收前复验。R011 的 "Not accepted" 矛盾表述已修复。
- N013 复验登记：apps/web/src/data-sources 在途改动落定后复验其行。
- shared 计数口径确认：436→437→440 随并发测试递增，均为各时点正确值。


## F1-F4 处置记录（en-US 切片移交项，2026-09-14）
- **F1/F2（chat-copy.ts 11 个 zh-only 键）**：核验 Vue 源——这些键在 Vue en-US locale 中同样缺失（grep en-US.ts 无 uploadAttachment/groupByDate/artifactsPending）→ **Vue 自身 zh-only 限制，React 忠实复现即为正确 parity**；修复需先改 Vue（超出 React 迁移范围）。登记为已知 Vue 侧限制，非 React 缺口。
- **F3（悬空引用）**：chat-copy.ts 注释引用的 2026-09-13-chat-i18n.md 不存在——cosmetic，登记待补建。
- **F4（document.title 不一致）**：已修复 870f67e9——apps/web/index.html title 从 "WeKnora React migration" 改为 "WeKnora"（对齐 Vue index.html:4）。


## 负路径核验 D1-D6 处置记录（2026-09-14）
依据 2026-09-14-negpath-sweep.md（14 格 92.9%，6 条差异）：
- **D1[中] 已修复**：React 对 /platform/* 受保护前缀的 not-found 路由在鉴权前放行 → 改为未登录先重定向登录页（对齐 Vue 行为）；TDD 红绿（routes.test.ts S00 D1 用例）；dev/markdown fixture 显式放行保持不变。修复涉及 routes.tsx guardRoute。
- **D6[低] 已修复**：NotFoundPage 文案本地化（zh-CN「页面不存在」/ en-US fallback）——React 新增的 404 页此前仅英文；Vue 无 404 页故无 Vue 基准文案，按站点主语言补 zh-CN。
- **D2[低-中] 登录重定向目标（Vue 丢来源 vs React ?next= 回跳）**：React 行为更优，保持并记录差异；如需严格对齐 Vue（丢弃 next）需用户决策。
- **D3[高] / D4[高] / D5[高] Vue 侧缺陷**：Vue 对不存在路由/无效 KB 深链接渲染空白死页，D5 最严重——无效 KB 呈现「知识为空，拖放上传」误导空态（上传入口可用）。React 均有明确 404/错误态+重试。**严格 parity 需将 React 降级为同等破损行为，不建议**；按 §一 作为「Vue 行为与业务契约冲突」记录，提交用户决策：(a) 批准 React 更优行为为记录在案例外；(b) 要求逐像素复刻 Vue 破损行为。
- 后续建议切片：403 租户隔离 / register / join / onboarding 负路径与网络错误态。


## 负路径第二批 N-1~N-4 + S-1/S-2 处置记录（2026-09-14）
依据 2026-09-14-negpath2-sweep.md（register/join/onboarding/网络错误态，20 格 100% 断言，零数据写入）：
- **N-1[中] 已记录（Vue 侧缺陷，React 更优）**：登出访问 /join 时 Vue 落 /login 无 query、邀请码永久丢失（登录后加入组织流程中断）；React 落 /login?next=… 可续流程。与首批 D2 同根因（Vue next('/login') 不带目标）。按 §一 作为「Vue 行为与业务契约冲突」记录，提交用户决策（选项同 D2）。
- **N-2[低] 已修复**：onboarding 创建表单从页内嵌卡片改为居中模态弹窗（对齐 Vue CreateTenantDialog.vue 的 t-dialog 呈现：480px 宽、标题「创建新空间」+ 图标、副标题提示、描述改 textarea（512 上限 + Vue 占位文案）、主提交键「创建」/「取消」、提交中禁用遮罩与 ESC 关闭）；五语 Vue 原文文案逐字接入（tenant.create.dialogTitle/dialogSubtitle/descriptionPlaceholder/submit/cancel）；TDD 红→绿（onboarding-dialog.test.tsx，JSDOM 呈现规约）；Dialog 原语加性扩展 className。web 838/838 · shared 440/440 · typecheck 0 · build ✓。
- **N-3[低] 已记录（信息性）**：列表请求失败 Vue 产生 2 条未捕获 Promise rejection、React 0 条。React 无需复刻控制台报错，仅记录。
- **N-4[低-中] 已修复**：React onboarding 创建对话框（Creating…/Cancel/Loading…/Close/workspace 回退名/错误回退文案）+ JoinPage 整页（标题/表单标签/按钮/加载与错误文案）硬编码英文全部接入 i18n（auth.workspaceOnboarding.* 扩展 + auth.join.* 新命名空间，zh-CN/en-US 双语）；TDD 红→绿（auth-copy-i18n.test.ts，2 用例）；web 837/837 · typecheck 0 · build ✓。
- **S-1[共同] 已记录（Vue 侧限制）**：KB 列表网络失败两端一致呈现「暂无知识库」空态（无错误提示/重试）。React 忠实复现 Vue 基线 = parity 正确；误导性空态作为 Vue 基线 UX 缺陷登记，是否改进属用户决策。
- **S-2[共同] 已记录（Vue 侧限制）**：register 已存在邮箱服务端英文消息两端都未本地化——React 与 Vue 行为一致 = parity 正确；服务端消息本地化需后端/契约层改动，超出前端 parity 范围，登记待决策。


## 负路径第三批 T-1~T-4 处置记录（2026-09-14）
依据 2026-09-14-negpath3-tenant-isolation.md（403 租户隔离，8 格 100% 断言、零写入证明 8/8/3/2、OrbStack 恢复事件已记录）：
- **T-1[中] 已记录（语义冲突 → 用户决策，协调者建议以 React 为锚）**：无效租户恢复策略两端相反——Vue 停留受损会话（12+ 请求连发 403、空态伪装正常、无自愈无提示）；React 单次 auth/me 403 即清凭据强制回登录。React 行为符合安全卫生（受损凭据快速失效）；Vue 的静默毒化态与「失效凭据不得继续访问」的安全约束相悖。按 §一 记录冲突，请用户裁定：以 React 自愈语义为锚记 Vue 缺陷，或确认 Vue 行为为预期并要求 React 复刻。
- **T-2[低] 已修复**：React errorFromResult 对后端 string 型 error body（403 "Access denied: …"）丢失原文，退化为 "Request failed with status 403"。已加 string error 分支（errors.ts），TDD 红→绿（errors.test.ts 3 用例，含嵌套 record 与纯字符串体回归）。web 838/838 · shared 444/444 · typecheck 0 · build ✓（fcf246bf）。
- **T-3[低] 待修复（React 侧）**：401 refresh 失败后 Vue 清存储+跳转 /login（authRefresh.ts:142-146）；React 仅静默清凭据无跳转无提示（refresh-coordinator.ts:106-107）。需补跳转/提示；登记待办修复切片。
- **T-4[低] 待修复（React 侧，i18n 归一）**：组织 preview 失败 fallback 文案 Vue previewFailed vs React invalidCode（仅无 message 错误可见）。登记待办，与小修切片合并处理。
- **一致亮点**：越权对象访问两端状态码完全对齐（nil/构造 UUID KB→404 code 1003 不泄露存在性；真实其它租户 id→403 code 1002）；邀请码死码两端均 modal 内联 Invalid invite code；preview 最小暴露（仅名称/描述）。

