# OCR round-4-resume 增量 findings 独立复核（HEAD 76df0cee0）

复核基准：/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 @ 76df0cee0，仅只读核验（cat/grep/sed）。
范围：ocr-round-4-resume.md（386 findings）相对主报告 ocr-round-4.md 与 round4-highrisk-analysis.md 已判 23 条的**新增**条目：
全部新增 [security·high]/[bug·high]/[test·high]/[security·medium]，及 #140 实现面新增红线 [bug·medium]。
去重口径：按 path:line 与问题描述关键词（grep 主报告全文逐条比对）。主报告已判 10 条 high（6 VALID + 4 OOS）与 12 条红线 medium 在 resume 中全部复现，不重复判定。
#140 面判定口径同前：internal/modules/career|workbench、internal/handler、internal/router、internal/container、migrations、apps/web/src/career、apps/web/src/api*、packages/career-core、packages/api-client、apps/miniprogram(career)。

## 一、新增高危（bug·high 4 + test·high 1 = 5 条）

[VALID] N1 bug·high apps/miniprogram/src/services/career.ts:598-612 deleteWholeSpace 绕过 unresolved_action 预检属实：函数开头直接 newRequestId，无 pendingSpaceDeletion 检查（对照 career-intent.ts:94-98 recoverableWrite 的硬性守卫与 ocr3-030 冻结口径），歧义分支与 partial 成功分支均 `store.write(intentKey('spaceDeletion'),…)` 直接覆盖旧 intent。UI 确定性可达：gating.ts:50 `inMemoryStatus !== 'partial'` 使 partial 确定回执不封锁（deletionOutcomeUnknown=false），用户重新勾选知悉再点「发起完整删除」即生成新 requestId 覆盖旧 intent——旧 requestId 是函数头注释自述的「唯一可恢复的原 request id」，覆盖即永久失去对账入口，服务端为新 requestId 建全新删除记录、旧 partial 记录成孤儿（幂等红线）。onTap 内 gating 快照双击竞态同向加重。fixHint=发送前检查 pendingSpaceDeletion()，存在且无确定回执时抛 unresolved_action；「partial 后重新发起」改为先显式放弃（与 N2 放弃出口接线配合）。

[VALID] N2 bug·high apps/miniprogram/src/career/export-deletion.tsx:131-144 导出/删除两个恢复块只有对账+重试、无放弃动作属实：service 层 abandonPendingSpaceExport/abandonPendingSpaceDeletion 已定义（career.ts:264-265）但本页零消费（其余 career 页 discovery.tsx:134、application-material.tsx:84/101/114/264、rules-usage-reminders.tsx:115/123、progress-preparation.tsx:169/191/203 全有放弃出口）。死锁链逐环核实：intent 落持久 storage（跨页面存续，Web 是内存态）→ reconcileIntent 对账 404（请求未送达）在 fetch 抛错时不 store.remove（career.ts:243-248，remove 在 fetch 成功后）→ 重试按 intent 持久化的原 expectedRevision 逐字节重放、修订已前进必 revision_conflict（definite 不清 intent）→ pendingExport 恒非 null → exportBlocked 恒真且联动封锁删除主按钮与知悉勾选（gating.ts:34-35,41）→ 唯一解除=登出。career-intent.ts:131-135 注释明言显式放弃是未对账封锁的唯一解除出口。fixHint=两个恢复块各加经 confirmAction 确认的放弃 Action，调 abandonPendingSpaceExport/abandonPendingSpaceDeletion 并提示「以服务端记录为准」。

[VALID] N3 bug·high apps/miniprogram/src/career/export-deletion.tsx:212 发起路径 outcome_unknown 未失效旧 partial 回执属实：catch 仅 `setDelErrCode(typedCode(error))`，不清 deletion、不置 delRecoveryUnresolved——deleteWholeSpace 歧义失败已把新 intent(R2) 落盘，但页面仍持有旧 partial 回执(R1)，gating 输入 inMemoryStatus='partial' 使 deletionOutcomeUnknown=false，主按钮与知悉勾选在结果未知期间保持解锁（重新勾选即可再发 R3 覆盖 R2 intent，与 N1 复合）。对账/重试路径已由修复轮 2 F1 的 delRecoveryUnresolved 覆盖，发起路径漏同一语义；Web 基准 runDeletion uncertain → phase='unknown' 页面不保留可解锁旧回执。fixHint=发起 catch 中 outcome_unknown 时 setDeletion(undefined)（或复用 setDelRecoveryUnresolved(true)）回到 unknown 封锁。

[OUT_OF_SCOPE] N4 bug·high packages/views/src/craft/shell.tsx:100 CraftDrawer「full width under 760px」CSS 契约未落地（craft.css 无 .wk-craft-drawer 规则、td Drawer 内联 width 压不过）——packages/views 迁移旧面，代码问题属实但非 #140 career 面。fixHint(供迁移线)=craft.css 补 @media (max-width:760px){.wk-craft-drawer .t-drawer__content-wrapper{width:100%!important}}。

[OUT_OF_SCOPE] N5 test·high packages/views/src/craft interaction-card.test.tsx:84 提取正则 `/<button[^>]*>[^<]*<\/button>/` 匹配不到 td Button 的 span 包裹结构、授权红线断言「terminal card exposes no deciding controls」退化为永真——packages/views 迁移旧面（versions.test.tsx:58 已有可选包裹适配先例）。fixHint(供迁移线)=正则改 `/<button[^>]*>(?:<span[^>]*>)?[^<]*(?:<\/span>)?<\/button>/g`。

## 二、新增 security·medium（2 条）

[VALID] N6 security·medium internal/modules/career/handler.go:171 writeError 500 兜底把原始 e.Error() 写入公网响应体属实：默认分支 `status, code := 500, "internal"` 后 `body := gin.H{"code": code, "message": e.Error()}`，任何未映射内部错误原文透出；典型可达路径 rendering.go:1393 DownloadMaterialExport 中 `exportStorage.ReadExport` 的存储后端原始错误（可含 bucket/endpoint/驱动细节）未包成 ErrExportStorageUnavailable 直达 writeError 落默认分支。career 模块其余错误面均为 bounded typed 映射，唯此兜底裸透。fixHint=500/internal 分支改返回静态文案（如 "internal career office error"）并 slog.Error 记录原始错误。

[OUT_OF_SCOPE] N7 security·medium apps/web/src/agents/AgentEditorModal.tsx:1207 href="javascript:void(0)"（storage/sandbox 两处）——apps/web/src/agents 迁移旧面；且主报告已在同位置以 security·low 收录（ocr-round-4.md:2082,2092），resume 仅升级定级，非实质新增。fixHint(供迁移线)=改 `<button type="button">` 并在 agents.td.css 重置按钮外观。

## 三、#140 实现面新增 [bug·medium]（8 条，红线判定）

[VALID] N8 红线(幂等/逐字节重放) apps/miniprogram/src/services/career.ts:250-252 retryIntent 把 fallbackExpected 恒传 revision()（档案修订）属实：progress 域 CAS 期望是每申请事件计数（appendProgressEvent 走 expectedOverride），缺 expectedRevision 的 progress intent 重放会拿到错误域值必然 revision/idempotency_conflict，恢复链死路；且无条件 fallback 使 retryRecoverable 的 intent_revision_missing 快速失败守卫（career-intent.ts:115-117）对所有经 retryIntent 的 kind 永不触发。注：当前代码新建 progress intent 恒持久化 expectedRevision，实际影响面为遗留/畸形 intent（如测试注入旧形状）+守卫失效。fixHint=retryIntent 增可选 fallbackExpected 参数，progress 传「读取该申请 progress.revision」或不传让其显式失败。

[VALID] N9 红线(intent 键发送时刻预铸) apps/miniprogram/src/services/career.ts:604,632 deleteWholeSpace 成功路径与 retryPendingSpaceDeletion 成功路径均现场 `intentKey('spaceDeletion')` 重取当前作用域（career.ts:234 = intentKeyFor(kind, auth.scope.capture())），而非用函数开头捕获的 stamp 预铸键属实：runtime isCurrent 复验只覆盖 transport 返回那一刻，await 续体到 store.write/remove 之间登出→重登可插队，旧空间删除 intent 落到新作用域键（新空间恢复 UI 展示并可重放异空间 requestId）、retry 的 remove 删错新作用域键留旧键。career-intent.ts 文件头红线明言「intent 键一律按发送时刻 stamp 预铸，绝不按当前作用域补铸」；对照 retryRecoverable 在读取 pending 前即预铸 key。catch 路径因 isCurrent 与 write 同步相邻无窗口，成功路径违约。fixHint=两函数用 `intentKeyFor('spaceDeletion', stamp)` 预铸键统一落盘/清理。

[VALID] N10 红线(类型化冲突) internal/modules/career/handler.go:1823-1825 ErrUploadClaimLost 逃逸 500 属实：finishClaimFailure 内 ClearSourceResource 返回 e 原样上抛（profile_intake.go:126-140 RowsAffected!=1 即 ErrUploadClaimLost），调用方 1710/1740 writeError，而 writeError switch 无该哨兵映射落默认 500/internal；同文件 1673/1732/1756 及 finishClaimFailure 首次 FinishSourceClaim(1813) 对同语义「claim 被接管」均内联翻译为 202+最新 source，唯此清理路径破坏材料/上传 status 契约。fixHint=该分支补 errors.Is(ErrUploadClaimLost) → GetSource 后返回 latest,true,nil（复用 superseded 分支）。

[VALID] N11 红线(类型化冲突/幂等) internal/modules/career/reminder.go:262-269 并发同源 SetReminder 以原始唯一键错误 500 逃逸属实：attemptReminderWrite 内去重读（reminder.go:308，无 FOR UPDATE）发生在 profile head FOR UPDATE（:327）之前，READ COMMITTED 下并发同 source 不同 requestId 均读「无 todo」，后到者在 head 锁等待胜者提交后继续，create reminderRecord 撞 career_reminder_scope_source 唯一键（reminder.go:150-153）→ isReceiptRaceError 命中（office.go:836-842 含 unique constraint）→ 按本 requestId 回放必然落空（胜者持另一 requestId）→ 落到 `return ReminderReceipt{}, txErr` 裸冲突穿透 writeError 500，违背模块承诺的 Deduplicated 回执或 OutcomeUnknownError。fixHint=去重读移到 head FOR UPDATE 之后（与 receipt/head 同序串行化），或对源唯一键冲突做一次有界重试收敛到 Deduplicated。

[VALID] N12 红线邻接(确定成功不得路由进回执恢复) apps/web/src/career/MaterialPage.tsx:327-335 lookupReceipt 已恢复本地编辑状态（setMaterialId/setSavedBody/setSections/replaceState）后裸 `await reloadView(...)`，回读失败落外层 catch 被置 'unknown' 并提示「材料回执暂时无法读取…重新查询/重试」属实——回执已查到且状态已恢复，误导用户走恢复流程；对照同页 publishExport 对成功后 reloadExports 失败的独立 try/catch（成功文案+「列表暂时无法刷新」）先例。fixHint=reloadView 包独立 try/catch，失败仍置 idle 并提示「回执已恢复（内容回显暂时失败）」。

[VALID] N13 红线邻接(瞬态/确定分流) apps/web/src/career/MaterialPage.tsx:184-188 URL 恢复材料的 catch-all 属实：任何失败（含瞬时网络/超时）一律 clearPrivate 进 forbidden 终态分支（整页 alert 无重试），且 clearPrivate（:131-137）setMaterialId(undefined) 并 replaceState 抹掉 ?material= 参数丢失会话内持久指针；对照同页各写入路径 forbidden/error 显式分流口径。fixHint=catch 解析 errorDetails：仅 forbidden 才 clearPrivate，其余 setPhase('error')+可刷新重试提示并保留 URL 参数。

[VALID] N14 红线邻接(不可逆决策的信息完整) apps/web/src/career/SubmissionPage.tsx:138-141 可投递导出列表读取把所有错误静默吞掉置 exports=[] 属实：瞬时失败时 UI 显示「尚无可投递版本…或选择显式未知版本」（:267），诱导用户选 UNKNOWN_VERSION_CHOICE（:265）——一申请仅一条不可撤销投递记录，基于瞬时失败做不可逆选择；与同页 records 读取的显式分流及 fail-closed 原则不一致。fixHint=catch 分流 forbidden（clearPrivate）与瞬态（保留错误态+「读取失败期间勿选未知版本」提示+重试入口）。

[VALID] N15 非红线(UX 契约) apps/web/src/career/MaterialPage.tsx:423-427,464-467 publishExport 与 revokeExport 的 revision_conflict 分支只更新 exportPhase/exportConflict/setMessage、未 setExportMessage 属实：busy 文案「正在渲染并核验导出…/正在撤销导出…」残留，且 phase='error' 时以 error 样式 role=alert 呈现（:576），真实原因只在页首主 message 区。fixHint=两分支补 setExportMessage 与 setMessage 同口径提示。

## 统计

- 新增合计 15 条：VALID 12 / FALSE_POSITIVE 0 / OUT_OF_SCOPE 3（N4/N5 craft 迁移面、N7 agents 迁移面且为主报告 low 升级项）。
- 新增高危 5 条（bug·high 4 + test·high 1）：VALID 3（全部 #140 miniprogram 面）、OOS 2（packages/views craft）。
- 新增 security·medium 2 条：VALID 1（handler.go:171，#140 面）、OOS 1。
- #140 面新增 bug·medium 8 条：VALID 8（红线/红线邻接 7：N8-N14；非红线 UX 1：N15）。
- 结论：resume 全量新面貌在 #140 实现面内新增需修复项共 12 条，其中 3 条 high 全部集中于 miniprogram 导出/删除生命周期（N1 删除重发覆盖 intent、N2 恢复块无放弃出口死锁、N3 发起路径 outcome_unknown 不封锁），均为 round-4 主报告修复漏网；无新增误报。OOS 3 条供迁移线参考。
