Review partially complete: 312 finding(s); 100 of 319 selected item(s) failed.

─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:114-115 ───
[test · medium] waitText 谓词 /毕业时间|求职/ 混入了静态文案：discovery 页 Screen title
为『求职工作台』且正文静态句含『同一份求职档案』，这些文案在数据加载前即渲染。等待条件可能在 Web 同源数据未到达时提前满足，随后 career-page（毕业时间：2026-06 · 修订 2）与
search-entry 断言在数据缺失的快照上评估，产生偶发假 FAIL、影响退出码门禁的可复现性。这与第 5
步注释（ocr1-030：静态文案与数据可见需分开断言）的原则相悖，本步应等待数据专属标记本身。

-     const careerText = await waitText(page, /毕业时间|求职/, 25000).catch(() => allTexts(page));
+     const careerText = await waitText(page, /毕业时间：2026-06/, 25000).catch(() => allTexts(page));
      record('career-page', /毕业时间：2026-06/.test(careerText) && /修订 2/.test(careerText), 'web-created profile fact 毕业时间=2026-06 · 修订 2 visible in weapp');


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:53-53 ───
[maintainability · low] tapConsent 的回退分支会命中页面内任意 checkbox。当前登录页恰好只有一个同意勾选框（features/auth/pages.tsx
中唯一 Checkbox），但若登录页后续新增其他复选框（如营销订阅），回退将误触错误控件，且以 consent-tap PASS 的形式掩盖问题。建议回退仅在页面 checkbox 唯一时生效。

-     const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
+     let box = await page.$('.wk-consent checkbox');
+     if (!box && (await page.$$('checkbox')).length === 1) box = await page.$('checkbox');


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:32-36 ───
[maintainability · low] connectRetry/waitText/tapText/tapConsent/relaunch 五处各自手写『deadline + while +
sleep』轮询骨架，超时语义与轮询间隔各自为政（5000/900/900/900/5000ms），维护时需五处同步修改。可提取通用 pollUntil(fn, timeoutMs,
intervalMs) helper 统一骨架与超时语义。

- async function waitText(page, re, timeoutMs = 25000) {
+ async function pollUntil(fn, timeoutMs, intervalMs = 900) {
    const deadline = Date.now() + timeoutMs;
-   while (Date.now() < deadline) { const all = await allTexts(page); if (re.test(all)) return all; await sleep(900); }
-   throw new Error(`waitText timeout ${re}`);
+   while (Date.now() < deadline) { const r = await fn(); if (r) return r; await sleep(intervalMs); }
+   return null;
  }
+ // waitText = async (page, re, t = 25000) => { const all = await pollUntil(async () => { const t2 = await allTexts(page); return re.test(t2) ? t2 : null; }, t); if (!all) throw new Error(`waitText timeout ${re}`); return all; };


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:10-10 ───
[security · low] /tmp 为多用户可写目录，将其作为 require 回退路径存在被预置同名模块劫持的供应链/路径注入风险（本机其他用户可写入
/tmp/wk-t33-automator/...）。脚本已用注释文档化属受控用法，但 env 注入 + NODE_PATH 解析已覆盖换机重放场景，建议移除 /tmp 历史安装位回退。

-   const candidates = [process.env.T33_AUTOMATOR, 'miniprogram-automator', '/tmp/wk-t33-automator/node_modules/miniprogram-automator'].filter(Boolean);
+   const candidates = [process.env.T33_AUTOMATOR, 'miniprogram-automator'].filter(Boolean);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:78-78 ───
[test · low] 口令以 REDACTED 占位字面量提交后，脚本原样重放必然登录失败，且失败将以 record('login', false)『未到达
home』的形式呈现而非明确的『凭据未注入』，误导排障方向。建议与 T33_AUTOMATOR/T33_WS_PORT 保持同一 env 注入模式，缺失时快速失败并给出明确提示。

-     await fillInput(page, '请输入密码', '[REDACTED-disposable]');
+     const password = process.env.T33_PASS;
+     if (!password) throw new Error('set T33_PASS to the disposable account password (redacted in evidence)');
+     await fillInput(page, '请输入密码', password);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:70-72 ───
[bug · medium] connectRetry 在 try/finally 之外执行，且 IIFE 未挂 .catch()：当自动化端口连不上（connect timeout，如
DevTools 未开/端口注入错误）时，promise 拒绝处于未处理状态，finally 里的 RESULT 日志与 process.exitCode 设置都不会执行。在 Node < 15
上未处理拒绝仅是警告、进程以退出码 0 结束，直接违反脚本自己注释的门禁约定「任一步骤 FAIL（含 driver-error）必须以非零退出码结束（ocr1-031）」；Node ≥ 15
虽会崩溃为非零退出，但按退出码/RESULT JSON 消费的门禁与复验工具拿不到任何步骤级结果。建议给 IIFE 补顶层 .catch（record + exitCode=1），或把
connectRetry 纳入同一 try。

  (async () => {
    const mp = await connectRetry(WS_PORT, 4);
    try {
+     // ...
+   } catch (e) {
+     record('driver-error', false, e.message);
+   } finally {
+     log('RESULT', JSON.stringify(results));
+     try { await mp.disconnect(); } catch {}
+     process.exitCode = results.some(r => !r.pass) ? 1 : 0;
+   }
+ })().catch(e => {
+   record('driver-error', false, e && e.message);
+   log('RESULT', JSON.stringify(results));
+   process.exitCode = 1;
+ });


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:82-83 ───
[test · medium] consent-tap 的 PASS 仅以 tap() 未抛错为依据，未复核勾选后的实际状态。T24 已证实 tap 落点偏移时 tap()
仍正常返回而勾选不生效（ocr3-022），此时本步骤记 PASS、实际未同意，随后点登录必然失败并被记成 login FAIL——与注释声称的「不允许勾选未生效仍继续点登录、把根因埋进 login
步骤」意图不符，脚本自设的前置保障形同虚设。建议 tap 后回读 checkbox 状态（或对应页面 data）确认已勾选，未生效时记 FAIL 并终止。

-     const consentOk = await tapConsent(page, 8000);
-     record('consent-tap', consentOk, consentOk ? 'consent checkbox tapped before login' : 'consent row not reachable');
+     const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
+     if (box) {
+       try {
+         await box.tap();
+         await sleep(600);
+         const checked = await box.attribute('checked').catch(() => null);
+         if (checked === null || String(checked) === 'true') return true;
+       } catch {}
+     }


─── internal/modules/career/career_export.go:643-643 ───
[bug · high] 导出/删除不对称导致不可逆数据丢失：边界说明向用户承诺 search_rules 分区「及其运行与发现待办（随完整导出携带后删除）」，且
CareerExportArchive 的文档注释声明「Deletion purges every one of these sections, so the export must carry
all of them or the data would be destroyed unrecoverably」，但 buildCareerExportArchive 实际只携带 rule
行本身。对照
careerPurgeTables，被删除却从未进入导出归档的还有：career_search_rule_runs、career_search_discovery_todos、career_evalu
ations、career_searches、career_search_results、career_opportunity_observations、career_source_revisions
（简历/JD 原件记录）、career_reconciliations、以及非 pending 的
career_proposals。删除是不可逆的，这些数据类在"完整导出"中被静默销毁，违反本文件自己声明的契约。请扩展归档补齐上述分区，或修正边界文案与归档不变量注释使其与实际携带范围一致。

- 			{Section: "search_rules", Description: "周期搜索规则及其运行与发现待办（随完整导出携带后删除）", Count: sectionCount("career_search_rules", "")},
+ 			{Section: "search_rules", Description: "周期搜索规则（其运行与发现待办不随导出携带，删除后不可恢复）", Count: sectionCount("career_search_rules", "")},
+ // 并在 CareerExportArchive 中补充 RuleRuns/DiscoveryTodos/Evaluations/Searches/... 等分区，使导出范围与 careerPurgeTables 对齐


─── internal/modules/career/application.go:352-352 ───
[bug · medium] 提交后外呼与 DeleteCareer 的投影移除步骤存在交错窗口，可遗留孤儿 Workbench 任务：CreateApplication 的 Career
行提交后、EnsureCareerApplicationTask 尚未执行时，若并发的完整删除已完成 purge（删除本申请行）与 RemoveProjections 步骤，随后 Ensure
仍会创建 session/run/mapping 投影，而 finalize 阶段的防御性清扫只重清 Career 行、不会重跑投影移除；updateApplicationLink 随后返回
NotFound，客户端只看到 OutcomeUnknown，但 Workbench 中残留引用已删除申请的任务投影，破坏 deleted 回执的真实性。建议：在
updateApplicationLink 返回 ErrApplicationNotFound 的 ready 分支中，通过 applicationTaskRemover
补偿回收本次刚创建的投影；或让 DeleteCareer 的 finalize 阶段在持锁清扫后重跑一次 RemoveCareerApplicationTaskProjections。

  	link, ensureErr := o.linker.EnsureCareerApplicationTask(ctx, scope.TenantID, scope.UserID, interfaces.CareerApplicationTaskIntent{
+ 		ApplicationID: receipt.ApplicationID,
+ 		RequestID:     input.RequestID,
+ 		Title:         applicationTaskTitle(resolvedOpportunityID),
+ 	})
+ 	if ensureErr == nil {
+ 		updated, updateErr := o.updateApplicationLink(ctx, scope, input.RequestID, ApplicationLinkStateReady, link)
+ 		if updateErr == nil {
+ 			return updated, nil
+ 		}
+ 		if errors.Is(updateErr, ErrApplicationNotFound) && o.applicationTaskRemover != nil {
+ 			// 空间已被完整删除：回收本次外呼刚创建的投影，避免孤儿任务。
+ 			_, _ = o.applicationTaskRemover.RemoveCareerApplicationTaskProjections(ctx, scope.TenantID, scope.UserID)
+ 		}
+ 		return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 	}


─── internal/modules/career/submission.go:170-173 ───
[bug · medium] 并发双确认时唯一索引冲突会以原始 DB 错误（500）而非类型化冲突返回：两个不同 requestID 对同一 application 的并发确认，在 READ
COMMITTED 以下的快照隔离下可能都通过 existing 检查，随后后者命中 career_submission_scope_application
唯一索引；isReceiptRaceError 将其判为 ambiguous 后仅按 requestID 重放（必然未命中），随后直接 return txErr，把未分类的 unique
constraint 错误透出公共边界。application.go 的 CreateApplication 对同等场景补做了 occupied 复查并返回
ErrApplicationConflict，此处缺少对应分支。建议在重放未命中时按 application_id 复查并返回 ErrSubmissionAlreadyConfirmed。

  		if ambiguous {
- 			// Never retry blindly with a new ID: the original request ID
- 			// decides whether the identical intent already committed.
  			replay, found, lookupErr := o.replaySubmissionReceipt(operationCtx, s, input.RequestID, fingerprint)
+ 			if lookupErr != nil { /* ... */ }
+ 			if found {
+ 				return replay, nil
+ 			}
+ 			var occupied submissionRecord
+ 			if e := o.db.WithContext(operationCtx).
+ 				Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, input.ApplicationID).
+ 				First(&occupied).Error; e == nil {
+ 				return SubmissionReceipt{}, ErrSubmissionAlreadyConfirmed
+ 			}
+ 		}


─── internal/modules/career/career_export.go:330-330 ───
[performance · medium] 导出载荷被双份落库且 ArchiveBody 是死列：ReceiptBody 内联了完整
Archive（mustJSON(receipt)），ArchiveBody 又存一份相同 payload；全仓库无任何读取 ArchiveBody
的代码（仅此处写入），每次导出行存储翻倍，而导出本身内联全部快照原文、无大小上限。同时导出事务在 profile 行 FOR UPDATE 锁内全量扫描并序列化所有 Career
表，大空间下会长时间阻塞该 scope 的全部写入。建议删除 ArchiveBody 列（摘要校验已用 payload 计算 digest），或将落库的回执副本改为不含内联 Archive
的精简形态；必要时评估将归档构建移出写锁。



─── internal/modules/career/career_export.go:625-627 ───
[bug · low] sectionCount 将计数查询错误吞为 0：该视图是删除前的用户确认界面数据源，任何 Count 失败（连接抖动、表暂时不可用）都会显示"0
条将被删除"，可能诱导用户在不知情下执行不可逆删除。建议将错误向上传播（CareerDeletionBoundary 返回 error），或至少以 -1/未知标记区分"确认为 0"与"计数失败"。

  		if err := query.Count(&total).Error; err != nil {
- 			return 0
+ 			return -1 // 计数失败：界面应显示"未知"而非 0
  		}


─── internal/modules/career/material.go:578-580 ───
[bug · low] ConfirmMaterial 成功后把行状态重置回 draft，MaterialStatusConfirmed
永远不会被持久化：career_materials.status 只会是 draft/failed，确认结果只存在于回执与 version 行中。后果：Material()/导出归档的
materials[].status 对已确认过版本的材料仍报 draft（web 端导出测试夹具甚至期望 'confirmed'），回执契约（status=confirmed+version）与
MaterialView/导出视图不一致，前端若按行状态判断确认态将永远失败。建议确认成功后将 status 落为 MaterialStatusConfirmed（下一次 EditMaterial
再回落 draft），或删除该死枚举并在视图/导出中改由 versionCount 表达确认语义。

  			Updates(map[string]any{
  				"version_count":   version,
- 				"status":          MaterialStatusDraft,
+ 				"status":          MaterialStatusConfirmed,


─── internal/handler/session/artifact_download.go:644-652 ───
[maintainability · low] 新版本 grant 消费端点将所有失败统一折叠为裸 404，与同文件既有 DownloadWorkbenchArtifactGrant（第
312-330 行）刻意设计的状态码语义不一致：既有路径对签名密钥未配置返回 501 + `artifact_signing_disabled`，对过期/无效签名分别返回 401 +
`artifact_grant_expired` / `artifact_grant_invalid`，其注释明确说明区分过期是为了让客户端提供重新授权 UX。新路径下：(1)
移动端无法区分"链接过期可重新签发"与"版本不存在"，无法复用同一重新授权交互；(2) 运维无法从下载侧状态码发现签名密钥未配置的部署错误（签发侧 501、下载侧
404）。VerifyArtifactVersionGrantAt 的错误信息已区分 expired 与 mismatch（"artifact grant expired"），可低成本镜像既有语义。

  	secret, err := workbench.ArtifactSigningKeyFromEnv()
  	if err != nil {
- 		c.AbortWithStatus(http.StatusNotFound)
+ 		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"success": false, "code": "artifact_signing_disabled", "error": "artifact signing key not configured"})
  		return
  	}
  	if err := workbench.VerifyArtifactVersionGrantAt(secret, grant, signature, time.Now()); err != nil {
- 		c.AbortWithStatus(http.StatusNotFound)
+ 		code := "artifact_grant_invalid"
+ 		if strings.Contains(err.Error(), "expired") {
+ 			code = "artifact_grant_expired"
+ 		}
+ 		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": code})
  		return
  	}


─── apps/miniprogram/src/career/progress-preparation.tsx:225-225 ───
[bug · low] `void Taro.pageScrollTo(...)` 放在 try/catch 里捕获不到它的异步拒绝——catch 只对同步抛出生效，选择器不存在或滚动失败时会变成
unhandled promise rejection，与注释“滚动失败不阻断入口”的意图不符。建议改为 `.catch()` 兜底。

-               {event.eventType === 'interview' ? <Text className='wk-small' onClick={() => { setFocus('interview_prep'); setFromTimeline(true); try { void Taro.pageScrollTo({ selector: '#wk-preparation', duration: 300 }); } catch { /* 滚动失败不阻断入口 */ } }}>为这场面试做准备 ›</Text> : <Text />}
+               {event.eventType === 'interview' ? <Text className='wk-small' onClick={() => { setFocus('interview_prep'); setFromTimeline(true); Taro.pageScrollTo({ selector: '#wk-preparation', duration: 300 }).catch(() => { /* 滚动失败不阻断入口 */ }); }}>为这场面试做准备 ›</Text> : <Text />}


─── apps/miniprogram/src/career/progress-preparation.tsx:262-262 ───
[bug · low] `fromTimeline` 在时间线入口置 true
后没有任何复位路径：用户随后手动把准备焦点切到「求职信」时，页面仍常驻显示「已从时间线进入：将为这场面试准备……」，提示与实际选中的焦点不符。建议在手动切换焦点时一并复位。

-       {(Object.keys(focusLabels) as PreparationFocus[]).map(option => <Text key={option} className='wk-small' onClick={() => setFocus(option)}>{focus === option ? '● ' : '○ '}{focusLabels[option]}</Text>)}
+       {(Object.keys(focusLabels) as PreparationFocus[]).map(option => <Text key={option} className='wk-small' onClick={() => { setFocus(option); setFromTimeline(false); }}>{focus === option ? '● ' : '○ '}{focusLabels[option]}</Text>)}


─── apps/miniprogram/src/career/progress-preparation.tsx:71-71 ───
[performance · low] `readPreparationDraft` 是同步存储读取（getStorageSync + JSON 解析 +
逐节结构校验），写在渲染体内意味着编辑小节时每次按键触发的重渲染都会重复执行这段同步 IO。建议用 useMemo 缓存；注意 memo 后需额外依赖一个在
`clearPreparationDraft`/保存成功后自增的 nonce，否则草稿被清除后「本地草稿」卡片不会消失。

-   const localDraft = applicationId.trim() ? readPreparationDraft(applicationId.trim(), focus) : undefined;
+   const localDraft = useMemo(() => (applicationId.trim() ? readPreparationDraft(applicationId.trim(), focus) : undefined), [applicationId, focus, draftNonce]);


─── apps/miniprogram/src/career/progress-preparation.tsx:0-0 ───
[style · low] `item.status` 的链式嵌套三元（draft/failed/兜底）违反团队“禁止嵌套三元表达式”规则，也与本文件顶部各 *Labels
映射的风格不一致。建议抽成辅助函数。

-           <Text className='wk-row-title'>{item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）'}</Text>
+ const preparationStatusLabel = (item: PreparationReceipt): string => {
+   if (item.status === 'draft') return '草稿（可审阅、可修订）';
+   if (item.status === 'failed') return `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}`;
+   return '生成中（可恢复）';
+ };
+ // …
+ <Text className='wk-row-title'>{preparationStatusLabel(item)}</Text>


─── apps/miniprogram/src/career/application-material.tsx:165-165 ───
[style · low] `appErrCode` 的三层链式三元违反“禁止嵌套三元”规则（本页 `submitBusy.error`、progress-preparation 的
`genBusy/reviseBusy`、rules 页 `loadRuleBusy` 均有同款）。建议抽成按 code 查表的 `Record<string, string>`
提示映射，新增错误码时只加一行。

-       {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode === 'revision_conflict' ? ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。' : appErrCode === 'application_conflict' ? ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。' : appErrCode === 'hard_ineligible_requires_continue' ? ' 硬性条件不符，需要先勾选显式继续才能提交申请。' : ''}</Notice>}
+ const APPLY_ERROR_HINT: Record<string, string> = {
+   revision_conflict: ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。',
+   application_conflict: ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。',
+   hard_ineligible_requires_continue: ' 硬性条件不符，需要先勾选显式继续才能提交申请。',
+ };
+ // …
+ {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode ? APPLY_ERROR_HINT[appErrCode] ?? '' : ''}</Notice>}


─── apps/miniprogram/src/career/application-material.tsx:284-284 ───
[maintainability · low] 同一个 `downloadBusy` 绑定到所有导出记录 × 所有格式的下载按钮：下载任一文件时全部按钮同时转圈，且 `useAction` 的
running 守卫会静默忽略并发点击，用户无法区分哪个在下载。同页 `listBusy` 同时驱动「刷新导出列表」与「读取投递记录」、discovery 页 `recoverBusy`
同时驱动对账与「同步 Web 端档案变更」是同款共享。建议按动作键位（如 `${exportId}:${format}`）记录当前 busy 项。



─── apps/miniprogram/src/career/application-material.tsx:23-25 ───
[maintainability · low] `tdesignButtonStyle` 在 4 个 career
页面（discovery/application-material/progress-preparation/rules-usage-reminders）逐字重复，`typedCode` 重复 3
处、`digestHead` 重复 2 处。后续要补映射（如 `--td-button-default-*` 边框色）需要同步改 4 个文件，容易漂移出不一致的主题。建议抽到共享模块（如
`src/career/shared.ts`）统一导出。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
- const digestHead = (digest: string): string => digest.slice(0, 12);
- const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };
+ // src/career/shared.ts
+ export const tdesignButtonStyle = '…'; // 唯一一份
+ export const typedCode = (error: unknown): string | undefined => { … };
+ export const digestHead = (digest: string): string => digest.slice(0, 12);


─── apps/miniprogram/src/career/discovery.tsx:44-44 ───
[maintainability · low] `quotaRefused` 靠用户可见文案的子串「搜索额度不足」来识别 typed 429 `search_quota_refused`，与
`core/errors.ts` 的中文文案强耦合：文案一旦微调，这里会静默退化成 danger 语气且没有任何编译期/测试期信号。建议让 `useAction` 保留原始 error（或 typed
code），按 code 判定而不是按呈现文案判定。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:48-48 ───
[maintainability · low] `usageErrCode` 只在 `loadEstimate` 里被写入，整个组件从未读取（预估失败的呈现走
`usageBusy.error`）——属于死状态。要么删除，要么真正在预估 Notice 中按 code 分型给出不同提示。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:169-173 ───
[style · low] `nextDueAt` 的两层嵌套三元违反“禁止嵌套三元”规则（同文件 `subscription.reason` 的链式提示同理）。建议抽成提前返回的辅助函数。

-         {live.status === 'enabled' && live.nextDueAt
-           ? <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>
-           : live.status === 'paused'
-             ? <Text className='wk-muted wk-small'>已暂停：下一次触发已取消，当前没有排程。恢复启用后按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</Text>
-             : <Text className='wk-muted wk-small'>规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</Text>}
+ function renderNextRunPlan(live: SetRuleReceipt | RuleView) {
+   if (live.status === 'enabled' && live.nextDueAt) return <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>;
+   if (live.status === 'paused') return <Text className='wk-muted wk-small'>已暂停：下一次触发已取消，当前没有排程。恢复启用后按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</Text>;
+   return <Text className='wk-muted wk-small'>规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</Text>;
+ }
+ // …
+ {renderNextRunPlan(live)}


─── apps/mobile/src/task-office-integration-smoke.ts:46-50 ───
[bug · medium] probeServerAuthBoundary 的 fetch 未设置任何超时：若部署端挂起（TCP 建连后不响应），整个冒烟会无限阻塞，evidence 无法落盘，CI
卡死。runTaskOfficeIntegration 里其余调用走 runtime/transport 有其自身超时策略，这里直连 fetch 是唯一裸通道。建议加
AbortSignal.timeout（超时抛 TimeoutError 会被 catch 归入 'unreachable'，与现有"不可达如实记录"语义兼容）。

      const response = await fetch(new URL('/api/v1/workbench/executions?limit=1', deploymentOrigin).toString(), {
        method: 'GET',
        redirect: 'error',
        headers: { accept: 'application/json' },
+       signal: AbortSignal.timeout(15_000),
      });


─── apps/mobile/src/task-office-integration-smoke.ts:44-44 ───
[maintainability · low] 探针路径 '/api/v1/workbench/executions?limit=1' 为硬编码字符串，与服务端 routes_workbench.go
的路由注册（r.Group("/workbench/executions") 挂在 /api/v1 下）重复维护：路由前缀一旦调整，探针会打到 404 并按当前逻辑记为
failed-open，得出"服务端鉴权失效"的误导性证据。建议与 createTaskOfficeRemote 的列表端点共享同一常量，或至少在两侧注释建立交叉引用。

+ // 该路径须与服务端 routes_workbench.go 的 /api/v1/workbench/executions 注册同步维护；
+ // 建议与 createTaskOfficeRemote 的列表端点共享同一常量，路由漂移时探针才会同步。
+ const WORKBENCH_EXECUTIONS_LIST_PATH = '/api/v1/workbench/executions?limit=1';
  export async function probeServerAuthBoundary(deploymentOrigin: string): Promise<'rejected' | 'failed-open' | 'unreachable'> {


─── internal/modules/workbench/service/workbench/application_task.go:317-320 ───
[bug · low] 标题限长用 len()（字节数）而错误文案写的是 "characters"：PG 侧 workbench_application_tasks.title 为
VARCHAR(255)（按字符计），SQLite TEXT 无限制，因此约 86 个汉字（258 字节）的合法标题会被误判 ErrApplicationTaskInvalid 而确定性拒绝。当前
Career 调用方 applicationTaskTitle 生成 ASCII 标题不会触发，但该 port 是公开 seam，建议改用 rune 计数使校验与列语义、文案一致。

  	intent.Title = strings.Join(strings.Fields(intent.Title), " ")
- 	if intent.Title == "" || len(intent.Title) > 255 {
+ 	if intent.Title == "" || utf8.RuneCountInString(intent.Title) > 255 {
  		return intent, fmt.Errorf("%w: title must be 1 to 255 characters", ErrApplicationTaskInvalid)
  	}


─── internal/modules/workbench/service/workbench/application_task.go:84-89 ───
[maintainability · low] EnsureCareerApplicationTask 与 ensureWithRetry 对同一 intent 各调用一次
normalizeApplicationTaskIntent，双重校验纯属冗余，且容易让读者误判两层的契约边界。保留 ensureWithRetry 内部的 normalize（它同时是测试注入自定义
ensure 函数的入口），公开入口直接委托即可。

- 	intent, err := normalizeApplicationTaskIntent(tenantID, ownerID, intent)
- 	if err != nil {
- 		return interfaces.CareerApplicationTaskLink{}, err
- 	}
+ 	// ensureWithRetry 入口已做 normalizeApplicationTaskIntent，公开入口直接委托，避免双重校验。
  	return c.ensureWithRetry(ctx, tenantID, ownerID, intent, c.ensureOnce)
  }


─── internal/modules/career/material.go:946-951 ───
[bug · low] diffMaterialBodies 以 heading 作为 section 的唯一键，但 validateMaterialShape 只校验了 claimId
全局唯一，未约束 heading 唯一，而 MaterialBody 是三端可自由编辑的结构化 JSON。当同一 body 内出现重复 heading 时 diff 结果会失真：baseline
侧同标题的后一个 section 在 baselineSections 映射中覆盖前一个，target 侧的 changed 比较只针对最后一个 baseline section——例如
baseline=[A(x), A(y)]、target=[A(x)] 时，实际是删除了第二个 section，却被报告为 "changed y→x"；若 target 无该
heading，则会为每个重复的 baseline section 各产出一条 section_removed 重复条目。这违背 CompareMaterialVersions 注释承诺的
honest diff 契约。建议在 validateMaterialShape 中拒绝重复 heading（与 claimId 去重一致），或让 diff 按 (heading, 出现序号)
等稳定标识配对 section。

- func diffMaterialBodies(baseline, target MaterialBody) []MaterialVersionChange {
- 	changes := []MaterialVersionChange{}
- 	baselineSections := map[string]MaterialSection{}
- 	for _, section := range baseline.Sections {
- 		baselineSections[section.Heading] = section
+ // 方案一：在 validateMaterialShape 中补充 heading 唯一性校验
+ func validateMaterialShape(body MaterialBody) error {
+ 	if len(body.Sections) == 0 || len(body.Sections) > maxMaterialSections {
+ 		return ErrInvalidRequest
+ 	}
+ 	seenClaims := map[string]bool{}
+ 	seenHeadings := map[string]bool{}
+ 	for _, section := range body.Sections {
+ 		if strings.TrimSpace(section.Heading) == "" || len(section.Heading) > maxMaterialHeadingBytes {
+ 			return ErrInvalidRequest
+ 		}
+ 		if seenHeadings[section.Heading] {
+ 			return ErrInvalidRequest
+ 		}
+ 		seenHeadings[section.Heading] = true
+ 		// ...
+ 	}
+ 	return nil
- 	}
+ }


─── apps/miniprogram/src/career/application-material.tsx:149-151 ───
[bug · high] 「显式继续」勾选框（acknowledged）没有任何提交控制作用：按钮 disabled 只看 pendingApplication，且
continueDespiteHardFailure 发送的是 ineligible 而与 acknowledged 无关。评估为 ineligible 时用户未勾选即点「创建申请」，请求仍以
continueDespiteHardFailure=true 发出并被服务端受理，直接创建「不合格申请（显式继续）」——与页面警示文案「默认阻断申请；勾选显式继续后可提交」相悖，也与 Web
端门禁不一致（apps/web/src/career/ApplicationPage.tsx L149：`if (ineligible && !acknowledged) return
undefined`）。顺带后果：本端 hard_ineligible_requires_continue 错误分支永远不可达（本端在 ineligible 时永远发 true），成为死分支。建议对齐
Web 加客户端门禁，并可将同意位改为 ineligible && acknowledged 双保险。

-         <t-button block size='large' theme='primary' ariaLabel='创建求职申请' customStyle={tdesignButtonStyle} loading={applyBusy.busy} disabled={pendingApplication !== null} onTap={() => void applyBusy.run(async () => {
+         <t-button block size='large' theme='primary' ariaLabel='创建求职申请' customStyle={tdesignButtonStyle} loading={applyBusy.busy} disabled={pendingApplication !== null || (ineligible && !acknowledged)} onTap={() => void applyBusy.run(async () => {
            try {
-             const receipt = await career.createApplication({ opportunityId, snapshotId, evaluationId: evaluation.evaluationId, batchIdentity, continueDespiteHardFailure: ineligible });
+             const receipt = await career.createApplication({ opportunityId, snapshotId, evaluationId: evaluation.evaluationId, batchIdentity, continueDespiteHardFailure: ineligible && acknowledged });


─── apps/miniprogram/src/career/progress-preparation.tsx:133-140 ───
[bug · medium] saveRevision 中 claims 取回的 `await career.material(materialId)` 位于 try 之外：断网时这次 GET
会先失败，错误直接抛给 useAction，走不到 catch 里的 looksOffline「已保留本地草稿（可继续编辑，未提交）」提示，也不会执行 setReviseErrCode
分型（outcome_unknown/revision_conflict 提示缺失）。而本地草稿此刻其实已经保存（savePreparationDraft
已执行），用户却只看到通用错误，与注释承诺的断网语义不符。建议把 try 提前到取回之前，让现有 catch 的断网提示与错误码分型自然覆盖取回失败。

      savePreparationDraft(draftFromSections());
      let body = career.bodyFromEditable(sections);
+     try {
-     // 主张保全（R1-F1）：服务端材料编辑是整体替换草稿正文——任何 claims 为空的小节，
+       // 主张保全（R1-F1）：服务端材料编辑是整体替换草稿正文——任何 claims 为空的小节，
-     // 提交前先从材料域当前正文按同名小节取回主张，绝不把 claims 为空的正文整体提交。
+       // 提交前先从材料域当前正文按同名小节取回主张，绝不把 claims 为空的正文整体提交。
-     if (body.sections.some(section => (section.claims ?? []).length === 0)) {
+       if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-       body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+         body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
-     }
+       }
-     try {


─── apps/miniprogram/src/career/application-material.tsx:108-110 ───
[bug · low] 投递对账成功后回执被 `void receipt` 丢弃，刷新投递列表依赖本地 application 状态：`if (application) await
loadSubmissions(...)`。冷启动/重进页面直入本页时 application
为空（这是恢复横幅最常见的场景），对账成功后既不回读申请也不刷新任何记录，用户只看到横幅消失、无正反馈。SubmissionReceipt 自带权威
applicationId，可直接用它恢复申请上下文（career.getApplication 已存在）并刷新列表，让恢复链自洽。

          const receipt = await career.reconcilePendingSubmission();
-         if (application) await loadSubmissions(application.applicationId);
-         void receipt;
+         setApplication(await career.getApplication(receipt.applicationId));
+         await loadSubmissions(receipt.applicationId);


─── apps/miniprogram/src/career/application-material.tsx:61-64 ───
[performance · low] 渲染体内每次重渲染都同步读 4 次 intent 存储（各一次 getStorageSync + JSON
解析：pendingApplication/pendingMaterialWrite/pendingSubmission/pendingMaterialPublish）——任意 Field
每个按键都会触发重渲染并重复这些同步 IO。discovery.tsx 的 pendingSearch()/pendingUpload()、rules-usage-reminders.tsx 的
pendingRuleWrite()/pendingReminderWrite()/readStoredRuleId() 是同款模式（与已确认的 progress-preparation
readPreparationDraft 问题同类）。建议按已确认 finding 的方案统一处理（useMemo + 写入/清除后自增的 nonce，或抽一个共享 hook），一次修复四个页面。



─── apps/miniprogram/src/career/application-material.tsx:10-10 ───
[maintainability · low] errorMessage 导入后从未使用（本页错误均经 typedCode/useAction 呈现，全文件仅此 import
行出现），属于死导入，建议删除。

- import { errorMessage } from '../core/errors.ts';
+ import { formatTime } from '../core/format.ts';


─── internal/modules/workbench/service/workbench/application_task.go:268-272 ───
[bug · medium] race 分类器在 SQLite 部署下会漏掉最典型的竞争错误形态：两个 request 唯一性 marker（uq_agent_runs_request /
uq_workbench_application_tasks_request）只出现在 PostgreSQL 的错误文本（duplicate key value violates unique
constraint "uq_agent_runs_request"）里；而 SQLite 在唯一索引冲突时返回的是 "UNIQUE constraint failed:
agent_runs.tenant_id, agent_runs.owner_id, agent_runs.request_id"——不含索引名，且主服务 gorm.Open 未启用
TranslateError（container.go:1363），也不会得到 gorm.ErrDuplicatedKey。生产 SQLite
DSN（container.go:1355，DB_DRIVER=sqlite，WAL + _busy_timeout=5000）下，并发同 request ID 的典型交错是：输家在写锁上等
busy_timeout 直到赢家提交，随后 agent_runs 插入撞 uq_agent_runs_request——该错误不匹配任何 marker，ensureWithRetry
直接把原始驱动错误抛给 Career，既不走重试/恢复 Find，也不返回 ErrApplicationTaskUndecided；Career
侧（application.go:379-384）将其归入 OutcomeUnknownError、申请停在 linking。下次重放可自愈，但 "request ID 幂等并发重放" 契约在
SQLite 部署下失效（PG 不受影响）。仓库内已有先例（resource.go isUniqueViolation 对裸驱动消息回退匹配 "unique
constraint"）表明这一消息形态是已知问题。建议为两个索引补充精确到列清单的 SQLite marker（mattn 与 glebarez 两种驱动的消息都包含完整列清单，且
strings.ToLower 已做大小写归一），既恢复 SQLite 覆盖又不扩大到无关表的约束失败；或在主连接上启用 gorm.Config.TranslateError。

  	message := strings.ToLower(err.Error())
  	for _, marker := range []string{
  		"uq_agent_runs_request",
  		"uq_workbench_application_tasks_request",
+ 		// SQLite（TranslateError 关闭时）的重复键消息不含索引名，只有列清单；
+ 		// 精确到索引列可避免误吞其它表的唯一约束失败。
+ 		"unique constraint failed: agent_runs.tenant_id, agent_runs.owner_id, agent_runs.request_id",
+ 		"unique constraint failed: workbench_application_tasks.tenant_id, workbench_application_tasks.owner_id, workbench_application_tasks.origin, workbench_application_tasks.origin_request_id",
  		"database is locked",


─── apps/miniprogram/tests/career-discovery.test.mjs:61-61 ───
[maintainability · low] 顶部声明的 ambiguous 判定函数（L61）在全文无任何调用点——L129/L443 只是测试标题中包含 "ambiguous"
单词，并非引用。这是死代码，且它编码了“哪些错误算歧义结果”的判定口径，会误导后续读者以为存在基于该谓词的断言分支。建议删除；若想保留口径说明，应移入 career.ts 的
ambiguousOutcome 注释或以断言形式真正使用它。

- const ambiguous = error => ['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED'].includes(errorCode(error)) || errorCode(error) === 'outcome_unknown' || (error?.status === undefined || error.status >= 500);
+ // 删除该函数；歧义结果的判定口径已由被测源码 ambiguousOutcome 实现并经 C2/D5 等用例的行为断言覆盖。


─── apps/miniprogram/tests/application-material.test.mjs:37-37 ───
[maintainability · low] materialView fixture（含 versions/versionCount 的完整材料视图）构造后未被任何用例使用：A3 走
materialVersions 列表端点，H2 走 403 错误路径，M1 用本地 webBody，R1-P4 里的 matViewFor 是
progress-preparation.test.mjs 中另一个独立定义。属于死代码，建议删除，或补充一个消费该 fixture 的 GET /materials/:id 视图解码用例。

- const materialView = (over = {}) => ({ materialId: 'mat-1', status: 'confirmed', pinnedEvidence: matPin, body: materialBody, reviewRisks: [], versionCount: 2, versions: [{ version: 1, createdAt: T }, { version: 2, createdAt: T }], createdAt: T, updatedAt: T, ...over });
+ // 删除 materialView；若需覆盖 decodeMaterialView 的完整字段，请补充：
+ // 'GET /api/v1/career/materials/mat-1': call => stub.succeed(call, { data: materialView() }),
+ // const view = await career.material('mat-1'); assert.equal(view.versionCount, 2);


─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:65-77 ───
[maintainability · medium] backend()/freshLogin()/careerCall()/errorCode() 四个装配帮助函数（连同顶部
registerHooks 重定向 + __API_ORIGIN__ + import 序列，约 60 行）在 review 组内 5
个文件中逐字重复：application-material、career-discovery、progress-preparation、rules-usage-reminders（本文件外的
assembly.test.mjs 也有同款 backend()）。冻结合同（登录流程、错误包络 {error:{code,message}}、method+pathname
路由器）一旦调整需同步改多处，漂移后各文件测得的行为不再一致——这正是这类合同测试最怕的失效模式。建议参照 taro-stub.mjs 的先例抽取
tests/helpers/career-harness.mjs（导出
installTaroStub/registerBackend/freshLogin/careerCall/errorCode），freshLogin 保留 me 与默认路由参数以兼容
progress-preparation 的 A/B 账号变体。

- function backend(routes) {
-   stub.use(call => {
-     const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
-     const path = new URL(call.options.url).pathname;
-     let fn = routes[`${method} ${path}`];
-     if (fn === undefined) {
-       const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
-       if (prefix) fn = routes[prefix];
-     }
-     if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
-     fn(call);
-   });
- }
+ // tests/helpers/career-harness.mjs
+ // export { installTaroRedirect } from './taro-stub.mjs'; // registerHooks + __API_ORIGIN__
+ // export function backend(routes) { /* 统一的 method+pathname 路由器 */ }
+ // export async function freshLogin({ me, open, extraRoutes } = {}) { /* 统一登录装配 */ }
+ // export const careerCall = suffix => ...;
+ // 各测试文件：import { backend, freshLogin, careerCall, errorCode } from './helpers/career-harness.mjs';


─── apps/web/src/career/ProgressPage.tsx:166-168 ───
[bug · high] [round-3 遗留，未消除] lookupReceipt 的 catch 未识别 acceptReceipt 抛出的
ReceiptMismatchError：acceptReceipt 在 try 内抛出的「回执与本次请求不匹配」是确定性协议错误（protocol.ts 注释明确要求消费方在 catch 顶部
instanceof 识别并直接置 'error'），当前实现将其与网络不确定失败一同落入
'unknown'，提示用户继续用原请求编号查询/重试——把确定性协议错误路由进回执恢复，违反红线；用户会基于错误回执反复重试，掩盖服务端协议故障。runWrite 已有该判定（本文件仅 1 处
instanceof），此处遗漏。

-    if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的进展，已清除进展内容。'); return }
+    if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`进展回执查询未完成：${cause.message}，已放弃本次结果。请刷新后重新提交。`); return }
     setWritePhase('unknown')
-    setMessage(parsed.code === 'not_found' ? `尚未找到进展回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '进展回执暂时无法读取。原请求编号已保留，可稍后重试查询。')


─── apps/web/src/career/SearchPage.tsx:210-210 ───
[bug · high] [round-3 遗留，未消除] 回执 requestId 与本次请求不匹配（stored.requestId !==
attempt.requestId）是确定性协议错误，当前置 phase 'unknown' 并提示「请重试查询」，把协议故障路由进回执恢复，用户会陷入无限查询循环；与本页 send()
中同类分支（置 error 'invalid_response'、清空 attempt、放弃结果）自相矛盾，也与 OpportunityPage 对 ReceiptMismatch 的处理不一致。

-    if (stored.requestId !== attempt.requestId) { setPhase('unknown'); setNotice('服务返回的请求编号与本次找岗不匹配；保留原指令与请求编号，请重试查询。'); return }
+    if (stored.requestId !== attempt.requestId) { setError({ code: 'invalid_response', text: '服务返回的请求编号与本次找岗不匹配，已放弃本次结果。请开始一次新的找岗。' }); setPhase('idle'); setAttempt(undefined); setNotice(''); return }


─── apps/web/src/career/CareerPage.tsx:5-6 ───
[maintainability · medium] [round-3 遗留，未整改] 跨包深相对导入绕过包公共出口：packages/career-core/package.json 已声明
exports（"./contracts"、"./desk"），应改用 '@weknora/career-core/desk' 与
'@weknora/career-core/contracts'；同理对 '../../../../packages/api-client/src/career.ts' 的深导入绕过了
api-client 的 exports 映射，且与同文件中的 '@weknora/api-client' 别名混用。OpportunityPage/ProgressPage/SearchPage
存在完全相同的模式，包结构移动或 exports 收紧时会直接编译失败。

- import { CareerDesk } from '../../../../packages/career-core/src/desk.ts'
- import type { CareerAction, CareerDocumentSource, CareerSource, CareerUpload, CareerView } from '../../../../packages/career-core/src/contracts.ts'
+ import { CareerDesk } from '@weknora/career-core/desk'
+ import type { CareerAction, CareerDocumentSource, CareerSource, CareerUpload, CareerView } from '@weknora/career-core/contracts'


─── apps/web/src/career/SearchPage.tsx:26-29 ───
[maintainability · medium] [round-3 遗留，未整改] 本地 errorDetails / makeId / isUncertainOutcome 与
protocol.ts 的 errorDetails / newRequestId / isUncertainWrite 重复实现且行为已分叉（本地确定码集合额外含
search_quota_refused、unauthorized）；CareerPage 亦本地定义 makeId 与 message()。round-3 已为 ProgressPage
建立共享实现 + endpointDefiniteCodes 叠加模式，建议 SearchPage/CareerPage 同步迁移，避免后续维护漏改。

- function errorDetails(cause: unknown): TypedError {
-  const error = cause as { code?: string; currentRevision?: number; message?: string }
-  return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
- }
+ // 复用 protocol.ts：
+ // import { errorDetails, newRequestId } from './protocol.ts'
+ // const endpointDefiniteCodes = ['search_quota_refused', 'revision_conflict'] as const
+ // const isUncertainOutcome = (cause: unknown): boolean =>
+ //   endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : isUncertainWrite(cause)


─── apps/web/src/career/SearchPage.tsx:41-43 ───
[bug · low] formatCheckTime 直接按固定偏移切取并追加 "UTC" 后缀，但客户端契约
validTimestamp（packages/api-client/src/career.ts 的 rfc3339Timestamp 正则）同时接受 'Z' 与 '±hh:mm'
偏移；若服务端返回带本地偏移的时间戳（如 +08:00），展示的时间与 UTC 标注将相差数小时，误导用户。建议先用 new Date(timestamp) 归一化再格式化。

  function formatCheckTime(timestamp: string): string {
-  return `${timestamp.slice(0, 10)} ${timestamp.slice(11, 16)} UTC`
+  const date = new Date(timestamp)
+  if (Number.isNaN(date.getTime())) return timestamp
+  return `${date.toISOString().slice(0, 10)} ${date.toISOString().slice(11, 16)} UTC`
  }


─── apps/web/src/career/OpportunityPage.tsx:379-379 ───
[maintainability · low] facts.get(...)! 非空断言目前运行时安全——已核实 client.career.evaluation 经 decodeEvaluation
校验（contracts.ts 的 citesPinnedFact 保证每个 soft.match 的 profileEvidence 必在 facts 清单中，且
sameEvaluationFact 含 factKey+factRevision 比较）。但该不变量是隐式跨包耦合，且应用未配置
ErrorBoundary：一旦契约调整或新增数据源绕过解码器，此处将抛 TypeError 导致整页崩溃。建议改用与上方硬性规则区一致的条件渲染兜底（fact ? ... :
「没有可用的已确认事实」）。



─── apps/web/src/career/OpportunityPage.tsx:268-268 ───
[style · low] 链式/嵌套三元表达式（state === 'unknown' ? … : state === 'saved' ? … : …，主按钮内再嵌 state === 'busy'
? …）违反「禁止嵌套三元」规范；同文件评估按钮区（evaluationState 的四级链及 onClick
内嵌套）同理。建议提取状态→按钮组/标签的映射表或拆分为按状态渲染的小函数，提升可读性与后续扩展性。



─── apps/web/src/career/CareerPage.tsx:176-176 ───
[style · low] CareerPage 全文大量使用内联 style 对象（数十处），与同目录其他 career 页面（search.css / progress.css /
opportunity.css + 语义类名）做法不一致，违反团队 React 规范（禁用内联样式，动态样式除外），且每次渲染重建样式对象影响可维护性。建议迁移到独立的 career.css 类名（如
wk-career-page、wk-career-card 等语义类）。



─── apps/web/src/career/progress.css:15-19 ───
[maintainability · low] #07c05f 在本文件多处硬编码（stage 边框/文字、correction 侧条、按钮主色/hover），与项目 design-tokens /
TDesign 变量体系脱节；且同文件 fallback 混用（--td-brand-color 默认为蓝 #0052d9，与本绿色并存），主题调整时必然遗漏。建议在 design-tokens
定义如 --wk-career-brand 并统一引用。



─── apps/web/src/career/search.css:11-11 ───
[maintainability · low] #07c05f 硬编码（眉题、链接、焦点 outline、scope-notes 侧条）与 progress.css 同一问题；且同 career
模块的 opportunity.css 取色走 --td-brand-color（蓝 #0052d9），同一模块存在多种取色路径，主题令牌调整时各表面会不一致。建议统一改为共享的品牌令牌变量。



─── apps/web/src/career/ExportDeletionPage.tsx:71-75 ───
[bug · high] [high] 作用域切换/登出后的状态清理不完整且无恢复路径：clearPrivate 未重置
revision/revisionState/verifyMessage，也未重置 refs（lastExportRequest、deletedAnnounced）；且 readRevision
的副作用依赖仅为 [readRevision]（其 useCallback 依赖 [client, scopeController] 稳定），不含
scope.scope.generation，切换后不会重新读取。已核实 CareerPage 中本组件无 key、scope 切换不重挂载，而 forbidden/scope-changed
分支只渲染一条消息、没有任何重试按钮（对比 InboxPage 的「重新读取」与 RulePage 的「重新读取」）——用户离开路由前页面永久死锁。残留的旧空间 revision 会作为
expectedRevision 展示/提交；deletedAnnounced 残留为 true 会吞掉新空间首次删除成功后的 onCareerDeleted 与
verifyOldGrants（T22「删除后验证旧授权已失效」契约）；lastExportRequest 也可能跨空间重放旧回执查询。建议：clearPrivate 内补齐上述 state/refs
重置，readRevision 副作用增加 generation 依赖，并在 forbidden/scope-changed 分支提供「重新读取」按钮。

   const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
    setExported(undefined); setExportAttempt(undefined); setExportPhase('idle'); setExportMessage(''); setExportConflict(undefined)
    setBoundary(undefined); setBoundaryState(nextState); setBoundaryMessage(notice); setAcknowledged(false)
    setDeletion(undefined); setDeletionAttempt(undefined); setDeletionPhase('idle'); setDeletionMessage(''); setDeletionConflict(undefined)
+   setVerifyMessage('')
+   lastExportRequest.current = undefined
+   deletedAnnounced.current = false
   }, [])


─── apps/web/src/career/ExportDeletionPage.tsx:184-186 ───
[bug · medium] [medium] anchor.click() 后同步调用 URL.revokeObjectURL：部分浏览器（尤其 Firefox）中下载尚未真正启动即撤销 blob
URL，导出包下载会偶发失败，使「导出留存」流程不可靠。仓库既有实现均为延迟撤销（MaterialPage 用 setTimeout 5_000、KnowledgeDocumentsPage 用
setTimeout 0、knowledge-batch-download.ts 用 60s），应沿用同一惯例。

    anchor.href = url; anchor.download = `career-export-${exported.exportId}.json`
    anchor.click()
-   URL.revokeObjectURL(url)
+   setTimeout(() => URL.revokeObjectURL(url), 5_000)


─── apps/web/src/career/ExportDeletionPage.tsx:4-4 ───
[maintainability · medium] [medium] 以 ../../../../packages/api-client/src/career.ts 深相对路径导入类型，绕过同文件中
WeKnoraClient 所用的 @weknora/api-client 包别名。已核实包索引（packages/api-client/src/index.ts）目前仅导出
createCareerApi 与 CareerRequest，career 域类型未从公共入口导出——包 exports/paths 一旦收紧即编译失败。同一模式还出现在本次评审的
InboxPage.tsx、PreparationPage.tsx、RulePage.tsx、reconciliation.tsx 及 OpportunityPage.tsx。建议在包索引统一导出
career 类型（如 export type { CareerDeletionReceipt, ReminderView, PreparationReceipt, ... } from
'./career.ts'），页面改从 '@weknora/api-client' 导入。



─── apps/web/src/career/ExportDeletionPage.tsx:30-30 ───
[maintainability · low] [low] snapshotCount 为死代码：全文仅此一处定义、无任何调用（渲染实际使用
snapshotTotal/eventTotal/versionTotal），且参数类型引用了
CareerDeletionBoundaryView，易误导维护者以为边界计数在导出包区被使用，建议删除。



─── apps/web/src/career/InboxPage.tsx:119-131 ───
[performance · medium] [medium] 待办定位在 for 循环中逐条 await client.career.application：含 applicationId 的待办为
N 条时首屏就绪需 N 次串行往返，慢网络下线性放大。已核实 ReminderView 不携带 pinnedEvidence，逐条查询确有必要，但各条目相互独立，应先按 applicationId
去重再用 Promise.allSettled 并行（同模块 reconciliation.tsx 的双快照读取即用 Promise.all），任一失败单独标记 'failed' 不影响其余条目。

+     const ids = [...new Set(next.reminders.map((item) => item.applicationId).filter((id): id is string => Boolean(id)))]
      const refs: Record<string, ApplicationRef | 'failed'> = {}
-     for (const item of next.reminders) {
-      const applicationId = item.applicationId
-      if (!applicationId || applicationId in refs) continue
+     await Promise.all(ids.map(async (applicationId) => {
       try {
        const receipt = await client.career.application(applicationId, requestScope.signal)
        if (!scopeController.isCurrent(requestScope.scope)) return
        refs[applicationId] = { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }
       } catch {
        if (!scopeController.isCurrent(requestScope.scope)) return
        refs[applicationId] = 'failed'
-      }
-     }
+      }
+     }))


─── apps/web/src/career/InboxPage.tsx:55-55 ───
[maintainability · low] [low] 硬编码路由字符串 /platform/career/search：该路径已确认与 routes.tsx 一致，但同模块
OpportunityPage.tsx 已沉淀 opportunityEvidencePath/evaluationPath 等路径构造器，此处（及 discovery
待办跳转）应引用统一的路径常量，避免路由调整时散落失效。



─── apps/web/src/career/InboxPage.tsx:182-182 ───
[bug · low] [low] 订阅写入的 expectedRevision 以 view?.revision ?? 0 兜底：发起按钮在 view === undefined
时已禁用，但「用原请求编号重试」路径复用 attempt 重建 action 时若 view 被清空（如读取失败后），会以无效修订 0 提交一次必然失败的写入。与其静默兜底，不如显式阻断（view
缺失时直接拒绝重试并提示刷新），保持与 revision === undefined 时暂缓提交的一致语义。



─── apps/web/src/career/PreparationPage.tsx:136-136 ───
[bug · medium] [medium] readRevision 副作用只依赖 [readRevision]（其自身依赖 [client, scopeController] 稳定），未包含
scope.scope.generation：作用域切换后准备列表会经主读取副作用自动恢复，但档案修订仍是旧空间的值并继续以「当前档案修订 X」展示、作为生成/保存修订的
expectedRevision 提交，导致新空间首次写入必然被 CAS 拒绝（可恢复但状态展示失真、产生多余失败写入）。主 effect 已含 generation
依赖，此处应保持一致（ExportDeletionPage 存在同款问题，已另列）。

-  useEffect(() => { void readRevision() }, [readRevision])
+  useEffect(() => { void readRevision() }, [readRevision, scope.scope.generation])


─── apps/web/src/career/RulePage.tsx:23-26 ───
[maintainability · medium] [medium] 本页本地重造 errorDetails/isUncertainOutcome/makeId，未复用 protocol.ts
共享实现：已核实 career
目录其余全部页面（ApplicationPage/ExportDeletionPage/InboxPage/MaterialPage/OpportunityPage/PreparationPage/P
rogressPage/SubmissionPage/reconciliation）均已统一从 './protocol.ts' 导入，仅本页例外，形成两套可能漂移的实现——本地
isUncertainOutcome 缺少 ReceiptMismatchError instanceof 短路，本地 errorDetails 不解析 requestId、不处理字符串型
currentRevision，makeId 与 newRequestId 重复。建议改用 protocol.ts 的
errorDetails/isUncertainWrite/newRequestId（本页无本地 ReceiptMismatchError 抛出路径，可直接替换）。

- function errorDetails(cause: unknown): TypedError {
-  const error = cause as { code?: string; currentRevision?: number; message?: string }
-  return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
- }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'
+ // 删除本地 errorDetails / isUncertainOutcome / makeId，调用点改用共享实现


─── apps/web/src/career/RulePage.tsx:229-229 ───
[bug · medium] [medium] 查询回执路径在 requestId 不匹配时置 phase 'unknown'
并保留「查询回执/用原请求编号重试」按钮：回执与请求不匹配是确定性协议错误，违反本模块已确立的「确定性协议错误不得路由进回执恢复」红线（reconciliation.tsx 的
runReconcile/lookupReconcileReceipt 已按 ocr2-082 改为置 error 并提示更换新请求编号），本页 send 路径的同款校验也是置 error。此处维持
unknown 会形成「unknown → 查询 → mismatch → unknown」循环，应置 error、提示更换新请求编号，与 send 路径对齐。

-    if (stored.requestId !== attempt.requestId) { setPhase('unknown'); setNotice('服务返回的请求编号与本次保存不匹配；保留原内容与请求编号，请重试查询。'); return }
+    if (stored.requestId !== attempt.requestId) { setPhase('idle'); setAttempt(undefined); setError({ code: 'invalid_response', text: '服务返回的请求编号与本次保存不匹配，已退出恢复流程。请更换新的请求编号重试。' }); return }


─── apps/web/src/career/reconciliation.tsx:183-183 ───
[style · low] [low] 嵌套三元表达式：phase 文案链（error/forbidden/scope-changed 三支）可读性差，违反「禁止嵌套三元」约定。同款模式还出现在
ExportDeletionPage（删除回执 className 三支链）、InboxPage（applicationRef 三支链）、PreparationPage（readState
渲染链）。建议提取为映射对象或小函数，如 const phaseMessage: Record<typeof phase, string> / className 由 status 直接映射。

-   if (phase !== 'ready' || !status) return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'error' ? '岗位状态暂时无法读取。' : phase === 'forbidden' ? '当前空间不可访问此岗位。' : '空间已切换，已清除岗位状态。'}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>
+  const phaseMessage: Record<'error' | 'forbidden' | 'scope-changed', string> = { error: '岗位状态暂时无法读取。', forbidden: '当前空间不可访问此岗位。', 'scope-changed': '空间已切换，已清除岗位状态。' }
+  if (phase !== 'ready' || !status) return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{phaseMessage[phase]}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>


─── apps/web/src/career/export-deletion.css:43-47 ───
[maintainability · low] [low] 品牌色 #07c05f 在五个新
CSS（export-deletion/inbox/preparation/reconciliation/rule）中直接硬编码，且以 !important 覆盖 tdesign 令牌。已核实
design-tokens 已提供 --wk-color-brand（= #07c05f）且 tdesign-theme.css 将 --td-brand-color
映射为同一品牌绿——本文件部分位置已在使用 var(--td-brand-color-light)。品牌色应统一引用令牌（如 var(--td-brand-color,
#07c05f)），否则主题调整需五处散改，!important 也会削弱令牌优先级。

  .wk-lifecycle__export, .wk-lifecycle__download {
-   border-color: #07c05f !important;
-   background: #07c05f !important;
-   color: #fff !important;
+   border-color: var(--wk-color-brand, #07c05f);
+   background: var(--wk-color-brand, #07c05f);
+   color: #fff;
  }


─── apps/web/src/career/reconciliation.css:74-78 ───
[maintainability · low] [low] color-mix(in srgb, #07c05f N%, #fff) 无回退值：不支持 color-mix
的旧浏览器（Chrome<111/Safari<16.2）会整条声明失效，回执底色/边框直接丢失。design-tokens 的 tokens.ts 已有 accent 变体可沉淀为令牌；若保留
color-mix，建议在前面先写一条十六进制回退（background: #eafaf1; background: color-mix(...)）。文件内其余 #07c05f 硬编码同
export-deletion.css 评论所述应改用品牌令牌。

  .wk-reconciliation__diff-marker--changed {
-   background: color-mix(in srgb, #07c05f 14%, #fff);
-   color: #07c05f;
+   background: #e4f7ec;
+   background: color-mix(in srgb, var(--wk-color-brand, #07c05f) 14%, #fff);
+   color: var(--wk-color-brand, #07c05f);
    font-weight: 600;
  }


─── packages/api-client/src/career.ts:1255-1257 ───
[bug · high] open()/list()/changes() 以 `as` 强转返回，完全绕过
decodeCareerView/decodeCareerChangeSet——注释（decodeCareerExportReceipt 上方）声称校验统一在 desk 边界完成，但
createCareerApi 是 client 上的公开入口：apps/web 中
MaterialPage.tsx:142、SearchPage.tsx:97/173、RulePage.tsx:90/183、ApplicationPage.tsx:91、SubmissionPage
.tsx:120、PreparationPage.tsx:128、InboxPage.tsx:98、ExportDeletionPage.tsx:87 均直接调用
client.career.open() 并把 next.revision 写入页面状态（MaterialPage 的 readRevision 还将其作为后续写请求的
expectedRevision 来源），畸形响应（如 revision 缺失/非数字）会直达 UI 与写路径，重现 ocr3-142 修复前的状态污染。career-core 已导出
decodeCareerView/decodeCareerChangeSet，在此处直接套用即可，desk 边界的重复解码幂等无害。

-   async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
-   async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
-   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
+   async open(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) },
+   async list(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) },
+   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return decodeCareerChangeSet(await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) })) },
+ // 并在文件头部 import 中补充 decodeCareerView、decodeCareerChangeSet


─── packages/api-client/src/career.ts:1-1 ───
[maintainability · medium] api-client 以越出包根的相对路径 `../../career-core/src/contracts.ts` 跨包导入，与同包其余模块的
`@weknora/*` 包名导入约定不一致（client.ts 顶部即以 @weknora/contracts 导入）；且 packages/api-client/package.json 的
dependencies 只声明了 @weknora/contracts 与 @weknora/domain，未声明 @weknora/career-core（该包已被
pnpm-workspace.yaml 纳入并导出 ./contracts 子路径）。文件系统相对路径暂时可解析，但在 TS rootDir
约束、独立打包或包发布场景下会直接断裂，依赖图也失真。建议在 package.json 补充 `"@weknora/career-core": "workspace:*"`
并改用包名导入（index.ts 末尾的类型再导出同样使用此相对路径，需一并调整）。

- import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
+ import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '@weknora/career-core/contracts'
+ // 同时在 packages/api-client/package.json 的 dependencies 中补充："@weknora/career-core": "workspace:*"


─── packages/api-client/src/career.ts:1355-1355 ───
[maintainability · low] `operation.trim() !== operation` 是恒不可达的死条件：operation 的类型是 'search_once'
字面量联合，任何带空白或不同值的入参已被前一子条件 `operation !== 'search_once'` 拦截；当值恰为 'search_once' 时 trim
结果与原值相等。删除冗余子条件以降低阅读成本。

-    if (operation !== 'search_once' || operation.trim() !== operation) throw new CareerValidationError('usage operation must be search_once')
+    if (operation !== 'search_once') throw new CareerValidationError('usage operation must be search_once')


─── packages/career-core/src/desk.ts:172-174 ───
[bug · low] mergeProposal 在 current 与 incoming 均为 pending 时不比较 revision 直接覆盖：syncChanges() 对
changes() 增量集合逐条应用，若同一提案的多条 pending 变更非按 revision 升序到达（或与回执乱序交错），旧 pending 提案会覆盖新 pending
提案，造成状态回退。建议 pending→pending 分支同样比较 revision 后再覆盖。

   const current = proposals[index]!
   if (current.status !== 'pending' && incoming.status === 'pending') return
-  if (current.status === 'pending' || incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming
+  if (current.status === 'pending' && incoming.status === 'pending') {
+   if ((incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming
+   return
+  }
+  if (incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming


─── apps/web/src/career/ExportDeletionPage.tsx:172-173 ───
[bug · medium] 查询导出回执路径未处理 acceptExport 抛出的 ReceiptMismatchError：回执 requestId
与本次请求不匹配是确定性协议错误（protocol.ts 头部注释明确要求「消费方应在 catch 顶部 instanceof 识别并直接置 error」），但此处 errorDetails 解析后
code 为 undefined，落入 setExportPhase('unknown') 分支，形成「unknown → 查询 → 再 mismatch →
unknown」循环，用户被永久引导重试同一必然失败的查询。runExport 的 catch 已有 instanceof 检查，InboxPage.lookupReceipt 与
reconciliation.tsx（ocr2-082）也已修复，本处应保持一致。

+    if (cause instanceof ReceiptMismatchError) { setExportAttempt(undefined); setExportPhase('error'); setExportMessage(`导出回执与本次请求不匹配，已退出查询。请更换新的请求编号重试。`); return }
     setExportPhase('unknown')
     setExportMessage(parsed.code === 'not_found' ? `尚未找到导出回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '导出回执暂时无法读取。原请求编号已保留，可稍后重试查询。')


─── apps/web/src/career/ExportDeletionPage.tsx:276-277 ───
[bug · medium] 查询删除回执路径同款问题：acceptDeletion 抛出的 ReceiptMismatchError 未被 instanceof 识别，code 为
undefined 时直接置 deletionPhase 'unknown' 并保留「查询删除回执/用原请求编号重试」恢复按钮，确定性协议错误被路由进回执恢复循环，违反模块红线（同 RulePage
已确认的 ocr2-082 同类问题，此处为独立漏配位置）。应与 runDeletion 的 catch 一致：置 error、清除 attempt 并提示更换新请求编号。

+    if (cause instanceof ReceiptMismatchError) { setDeletionAttempt(undefined); setDeletionPhase('error'); setDeletionMessage(`删除回执与本次请求不匹配，已退出查询。请更换新的请求编号重试。`); return }
     setDeletionPhase('unknown')
     setDeletionMessage(parsed.code === 'not_found' ? `尚未找到删除回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '删除回执暂时无法读取。原请求编号已保留，可稍后重试查询。')


─── apps/web/src/career/ExportDeletionPage.tsx:221-223 ───
[bug · medium] status 'deleting'（删除仍在进行中）时页面没有任何恢复入口：文案明确提示「可查询删除回执查看最新状态」，但「查询删除回执」按钮仅在
deletionPhase==='unknown' 时渲染，「用原请求编号重试删除」按钮仅对 status==='partial' 渲染；'deleting' 落在 phase
'error'，两个按钮都不出现。此时用户唯一出路是重新勾选确认后用新 requestId 再发起一次删除（可能与进行中的删除冲突），或刷新页面——刷新后 deletionAttempt
丢失，更无从查询该删除的结果。应在 'deleting' 状态下也渲染查询回执入口（deletionAttempt 此时尚在，未被清除）。

    // Partial keeps the attempt recoverable under the same request ID.
    setDeletionPhase('error')
    setDeletionMessage(next.status === 'partial' ? '删除部分失败：未完全删除。失败步骤已列出，状态与审计已保留，可用原请求编号重试恢复。' : '删除仍在进行中。可查询删除回执查看最新状态。')
+   // 渲染侧同步补充（与 partial 重试按钮同一区块）：
+   // {deletion?.status === 'deleting' && deletionAttempt && deletionPhase !== 'busy' ?
+   //  <div className="wk-lifecycle__actions"><button type="button" onClick={() => void lookupDeletionReceipt()}>查询删除回执</button></div> : null}


─── apps/web/src/career/PreparationPage.tsx:227-228 ───
[bug · medium] 查询准备回执路径未处理 acceptReceipt 抛出的 ReceiptMismatchError（'准备回执与本次请求不匹配'）：该错误无 code，会跳过
forbidden/invalid_request 检查落入 setWritePhase('unknown')，确定性协议错误被路由进回执恢复循环，违反 protocol.ts
红线。runGenerate 的 catch 已有 instanceof 检查，此处应保持一致（InboxPage.lookupReceipt 的同类路径已正确处理）。

+    if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`准备回执与本次请求不匹配，已退出查询。请更换新的请求编号重试。`); return }
     setWritePhase('unknown')
     setMessage(parsed.code === 'not_found' ? `尚未找到准备回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '准备回执暂时无法读取。原请求编号已保留，可稍后重试查询。')


─── apps/web/src/career/PreparationPage.tsx:282-283 ───
[bug · low] saveRevision 的最终兜底未调用 isUncertainWrite 判定：白名单之外的确定性失败（如 401 unauthorized，protocol.ts 的
isUncertainWrite 对其明确返回 false）会直接落入「修订结果暂时未知（原请求编号 …）…用原请求编号查询回执」的未知分支，向用户误报写入结果未知并引导走回执恢复。同文件
runGenerate 的兜底已是 `if (!isUncertainWrite(cause)) { …确定性失败… }` 的模式，此处应保持一致（该页顶部也已引入 isUncertainWrite
包装）。

     if (cause instanceof ReceiptMismatchError) { setReviseMessage(`修订未完成：${cause.message}`); return }
+    if (!isUncertainWrite(cause)) { setReviseMessage(`修订未保存：${parsed.message}`); return }
     setReviseMessage(`修订结果暂时未知（原请求编号 ${current.requestId}）。本次编辑内容已保留；可稍后在材料区用原请求编号查询回执，或重试保存。`)


─── apps/web/src/career/SearchPage.tsx:256-258 ───
[bug · medium] importResult 未校验回执请求编号：importUrl 返回的 imported.requestId 与本次 requestId
不一致时（确定性协议错误），当前直接存为该行的 receipt 并渲染「查看岗位证据」链接，可能把其他请求的证据当作本次导入结果展示。同文件
send()、OpportunityPage.importURLAttempt、EvaluationDetailPage 均按 ReceiptMismatch/回执身份校验处理；且本页读取路径
load() 的历史自动加载与 openHistoryEntry() 对 client.career.search() 的返回也未校验 stored.searchId 与请求一致（对照
EvaluationDetailPage/OpportunityEvidencePage 的做法）。建议补齐回执标识校验，不匹配时置确定错误并放弃结果。

     const imported = await client.career.importUrl({ requestId, url: row.link }, requestScope.signal)
     if (!scopeController.isCurrent(requestScope.scope)) return
+    if (imported.requestId !== requestId) {
+     setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], busy: false, error: '服务返回的请求编号与本次导入不匹配，已放弃本次结果。' } }))
+     return
+    }
     setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], busy: false, receipt: imported } }))


─── apps/web/src/career/protocol.ts:37-37 ───
[maintainability · medium] 共享基座 isUncertainWrite 的确定性失败码清单遗漏 'revision_conflict'：本模块所有消费方（SearchPage
本地清单、ProgressPage 的 endpointDefiniteCodes 叠加、CareerPage 的显式分支）都把 revision_conflict 视为确定失败，且 desk.ts
的 isAmbiguousOutcome 同样将其列为确定。基座遗漏意味着任何直接使用基座的端点（如 OpportunityPage 评估流）一旦返回 revision_conflict，会被判为
uncertain 路由进回执恢复，违反「确定性 4xx 拒绝不得路由进回执恢复」红线；同时迫使 ProgressPage 维护 overlay 叠加。建议将 'revision_conflict'
纳入基座清单。

-  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
+  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false


─── apps/web/src/career/OpportunityPage.tsx:144-146 ───
[bug · medium] 内联确定码清单遗漏 'not_found'：importOpportunity 在追加粘贴时携带 owner-scoped 的
opportunityId/priorObservationId 引用（career.ts:1274），当引用对象已被删除（ExportDeletionPage
的删除流程可移除求职数据）时服务端会返回确定的 not_found；protocol.ts 基座与 ProgressPage 的清单都把 not_found 列为确定失败，此处却落入 unknown
回执恢复，提示「网络未能确认保存结果」误导用户，随后查询回执又得到「尚未找到回执」，陷入无法终止的恢复循环。importURLAttempt 的同款内联清单同理。建议对齐基座清单（或改用
endpointDefiniteCodes 叠加模式）并为 not_found 提供明确文案。

-    if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
+    if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
      setState('error')
      setAttempt(undefined)


─── apps/web/src/career/CareerPage.tsx:163-163 ───
[bug · low] retryReceipt 成功找回回执后未清除 error：平行的恢复路径 retrySameAction 在入口 setError(undefined)、doAction
亦然；此处成功后只 setUnknownAction(undefined)，顶部错误卡片（标题「提交结果暂时未知」+「重新读取」）持续显示，与绿色成功提示 receiptNotice
矛盾，用户无法从界面上确认异常已恢复。建议成功分支同步 setError(undefined)。

-   try { const receipt = await desk.reconcile(unknownAction.requestId); if (!isCurrent(epoch)) return; sync(); if (receipt) { setReceiptNotice(`已找回回执 ${receipt.requestId}，修订 ${receipt.revision}`); setUnknownAction(undefined) } else setReceiptNotice('暂未找到回执；保留原请求，请勿更换请求编号重试。') }
+   try { const receipt = await desk.reconcile(unknownAction.requestId); if (!isCurrent(epoch)) return; sync(); if (receipt) { setError(undefined); setReceiptNotice(`已找回回执 ${receipt.requestId}，修订 ${receipt.revision}`); setUnknownAction(undefined) } else setReceiptNotice('暂未找到回执；保留原请求，请勿更换请求编号重试。') }


─── apps/web/src/career/CareerPage.tsx:114-114 ───
[bug · low] 上传结果未知分支 setError(parsed) 复用页面级错误卡片，但标题映射未覆盖上传上下文：网络层 TIMEOUT/HTTP_5xx 等错误（ApiError code
不在 forbidden/invalid_request/idempotency_conflict/revision_conflict/outcome_unknown
之列）落入此分支后，顶部卡片标题固定为「暂时无法打开档案」——而档案已成功打开、失败的是上传；卡片的「重新读取」按钮也与上传恢复流程无关。上传区已有 uploadNotice
与恢复按钮组完整覆盖该场景，建议不再写入页面级 error，或扩展标题映射区分上传失败。

-    else { setError(parsed); setUploadUnknown(attempt); setUploadNotice('上传结果暂时未知。已保留原文件和请求编号；先查询来源状态，再决定是否用相同内容重试。') }
+    else { setUploadUnknown(attempt); setUploadNotice('上传结果暂时未知。已保留原文件和请求编号；先查询来源状态，再决定是否用相同内容重试。') }


─── apps/web/src/career/opportunity.css:143-144 ───
[maintainability · low] 死选择器：全仓库检索 .wk-evaluation-action__status 仅在样式表出现，OpportunityPage.tsx
实际使用的类名是 wk-evaluation-status（wk-evaluation-status--{status}）。删除该死选择器，避免后续维护者误以为该变体有对应组件状态。

- .wk-evaluation-action__status,
  .wk-evaluation-status { box-sizing: border-box; min-width: 0; margin: 10px 0; padding: 12px 14px; border-left: 4px solid var(--td-warning-color, #d68b00); border-radius: 4px; background: var(--td-warning-color-1, #fff7e6); overflow-wrap: anywhere; }


─── packages/api-client/src/career.ts:938-939 ───
[bug · medium] 遗留 finding（round-1~3 均已提出，本轮仍未修复）：非 draft 快照的 snapshotSha256 校验仅在
opportunityId/snapshotId 存在时触发。若一个 failed/generating 行的负载只携带 `snapshotSha256: "garbage"`（不带两个
ID），两个分支都不抛错，而返回值处 `...(typeof snapshotRecord.snapshotSha256 === 'string' ? { snapshotSha256 } :
{})` 会把畸形 digest 直接送进 UI——与紧邻注释「whatever it does carry must still be a well-formed
digest」的冻结契约相悖。建议改为：快照任一字段存在即要求 digest 良构。

-  if (!draft && (validIdentifier(snapshotRecord.opportunityId) || validIdentifier(snapshotRecord.snapshotId))
+  if ((snapshotRecord.snapshotSha256 !== undefined || validIdentifier(snapshotRecord.opportunityId) || validIdentifier(snapshotRecord.snapshotId))
    && (typeof snapshotRecord.snapshotSha256 !== 'string' || !sha256Hex.test(snapshotRecord.snapshotSha256))) throw new TypeError('invalid preparation sources')


─── packages/api-client/src/career.ts:1471-1471 ───
[bug · low] 遗留 finding（先前轮已提出，仍未修复）：`.length` 统计的是 UTF-16 code units 而非字节。中文备注 1 个字符占 3 个 UTF-8
字节，客户端校验会放行实际超过 4096 字节的备注，随后被后端 maxSubmissionNoteBytes 拒绝（invalid_request），错误信息声称 "4096 bytes"
但测的并不是字节。建议用 TextEncoder 计量真实字节数（career-core contracts.ts 中已有 TextEncoder 先例）。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')


─── packages/api-client/src/career.ts:943-943 ───
[style · low] 嵌套三元表达式（团队规范禁止，round-3 已以 style·low 提出，仍未展开）：`a ? [] : Array.isArray(x) ? x :
undefined` 两层嵌套，且 factKeys 的三种取值（空数组/原数组/undefined 哨兵）混在一个表达式里，维护时易引入错误。建议展开为显式分支。

-  const factKeys = rawFactKeys === null || rawFactKeys === undefined ? [] : Array.isArray(rawFactKeys) ? rawFactKeys : undefined
+  let factKeys: unknown[] | undefined
+  if (rawFactKeys === null || rawFactKeys === undefined) factKeys = []
+  else if (Array.isArray(rawFactKeys)) factKeys = rawFactKeys


─── packages/career-core/src/contracts.ts:42-42 ───
[bug · low] revision 仅校验 `typeof === 'number'`，未校验安全整数与非负：小数（如 1.5）或负数（如 -1）的 invented payload
可通过校验进入 desk 状态机（applyView/syncChanges 的 Math.max 与 revision 比较）。这与同文件
validEvaluationFact（Number.isSafeInteger 且 >=0）以及 career.ts 的 validRevision 严度不一致，也使 ocr3-142
注释声称的「畸形响应在 desk 边界统一校验」不完整——该修复只覆盖了 revision 缺失形态。decodeCareerChangeSet（第 47-48 行）与
validFact/validProposal 的 revision 字段存在同样的宽松校验，建议一并收紧。

-  if (!isRecord(value) || typeof value.revision !== 'number' || !Array.isArray(value.facts) || !value.facts.every(validFact) || !Array.isArray(value.proposals) || !value.proposals.every(validProposal)) throw new TypeError('invalid career view')
+ function validRevision(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 }
+ export function decodeCareerView(value: unknown): CareerView {
+  if (!isRecord(value) || !validRevision(value.revision) || !Array.isArray(value.facts) || !value.facts.every(validFact) || !Array.isArray(value.proposals) || !value.proposals.every(validProposal)) throw new TypeError('invalid career view')
+  return value as CareerView
+ }


─── packages/api-client/src/index.ts:209-210 ───
[maintainability · medium] career 模块的公开导出面不完整：index.ts 只再导出 createCareerApi/CareerRequest 及 8
个核心类型，而 T09–T22
的领域类型与工具（MaterialView/MaterialReceipt/SearchOnceReceipt/RuleView/ApplicationReceipt/SubmissionReceip
t/ProgressView/CareerExportReceipt/MaterialExportDownload、CareerValidationError、各 decode*
校验器等）全部未导出，迫使消费方深路径导入源文件——apps/web/src/career/MaterialPage.tsx:5 等已以
`../../../../packages/api-client/src/career.ts` 绕过包入口取类型，破坏了包封装（与已确认的 career-core
跨包导入问题是同一接缝的另一侧）。建议按消费面在包入口统一再导出领域类型与 CareerValidationError。

- export { createCareerApi } from './career.ts';
- export type { CareerRequest } from './career.ts';
+ export { createCareerApi, CareerValidationError } from './career.ts';
+ export type { CareerRequest, ApplicationReceipt, CreateApplicationInput, MaterialView, MaterialReceipt, MaterialBody, MaterialExportReceipt, MaterialExportDownload, SearchOnceReceipt, RuleView, SetRuleInput, SubmissionReceipt, RecordSubmissionInput, ProgressView, ProgressReceipt, ReminderList, CareerExportReceipt, CareerDeletionReceipt, OpportunityURLImportReceipt, ReconcileReceipt, UsageEstimateView } from './career.ts';


─── apps/web/src/router.tsx:716-720 ───
[bug · medium] careerRoute 的懒加载组件未包 Suspense：同批其余四个 career
路由（careerSearchRoute/careerRulesRoute/careerOpportunityRoute/careerEvaluationRoute）及本文件所有其他懒加载路由均在组件
内包 `<Suspense fallback={<RoutePending …/>}>`，唯独求职主入口没有。全项目无 pendingComponent/defaultPendingComponent
配置，最近的边界是 shellPage 内包裹整个 PlatformShell 的 Suspense（router.tsx:212）——首次导航 /platform/career 且 chunk
未缓存时，CareerPage 挂起会把整个 shell（含侧栏导航）隐藏并替换为 RoutePending 加载页，造成整页闪断，且与既有模式不一致。建议补齐与同批一致的 Suspense 包裹。

    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',
-     component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
+     component: (): ReactNode => (
+       <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
+         <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />
+       </Suspense>
+     ),
    });


─── apps/web/src/routes.tsx:57-59 ───
[maintainability · low] snapshotId 语义与 router.tsx 不一致：这里对 /platform/career/opportunities/:id 要求
query 中 snapshotId 非空（且 trim）否则判 not-found（routes.test.ts:45 已锁定该行为），而 router.tsx:754 同路由缺参时以 ''
兜底（不 trim）传给 OpportunityEvidencePage，页面自行进入 invalid 态（OpportunityPage.tsx:400「链接缺少有效的快照编号」）。经核
guardRoute:201-204 对已认证用户的 not-found 返回 allow，运行时两路径最终都落到页面 invalid UI，但同一路径在两套路由层被分类为 not-found vs
可渲染页面，后续任一层演进（如 not-found 接 404 页）即产生行为分叉。建议两层对齐：要么 resolveRoute 放宽为允许空 snapshotId（交页面校验并同步改测试断言），要么
router 侧镜像同样的非空要求。

      const opportunityId = decodeSegment(careerOpportunity[1]!);
-     const snapshotId = query.get('snapshotId')?.trim();
-     return opportunityId && snapshotId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };
+     const snapshotId = query.get('snapshotId')?.trim() ?? '';
+     return opportunityId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };


─── apps/web/src/router.tsx:714-718 ───
[documentation · low] 注释错位：这段「Octop M2 expert-template catalog…」注释原属 expertsRoute，在插入五个 career 路由后紧贴
careerRoute 定义，会误导后续维护者将 career 路由归属为 expert 模块；expertsRoute（现位于下方）反而失去了说明。建议把该注释移回 expertsRoute
上方，并为 careerRoute 补一段求职入口自身的说明。

-   // Octop M2 expert-template catalog; list/detail are Viewer+ reads, the
-   // instantiate write stays Contributor+ server-side (routes_expert.go guard).
+   // 求职空间主入口（Issue #140）：档案确认 / 岗位发现 / 规则管理聚合页。
    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',


─── pnpm-workspace.yaml:3-3 ───
[maintainability · medium] career-core 已注册进
workspace，但全仓没有任何消费方声明对它的依赖：packages/api-client/src/career.ts:1-2 与 index.ts:211 以
'../../career-core/src/contracts.ts' 跨包相对路径引入，apps/web/src/career/* 与
apps/miniprogram/src/services/career.ts:3-6 同样以 '../../../../packages/career-core/src/...'
相对路径引入，完全绕过包名 exports 与 workspace 协议。这是 round-1~3 OCR 反复提出的遗留
finding（依赖边界/构建健壮性），本轮仍未消除：一旦目录调整、该包独立构建/发布，或单独对 api-client 做 typecheck/构建即会断裂。建议至少在
packages/api-client/package.json（及 apps/web、apps/miniprogram）dependencies 中声明
"@weknora/career-core": "workspace:*"，并将相对路径导入改为 '@weknora/career-core/contracts' /
'@weknora/career-core/desk'。

-   - packages/career-core
+ # pnpm-workspace.yaml 本身正确；配套修改消费方，例如 packages/api-client/package.json：
+ # "dependencies": { "@weknora/career-core": "workspace:*" }
+ # 并将 import ... from '../../career-core/src/contracts.ts' 改为 from '@weknora/career-core/contracts'


─── package.json:19-19 ───
[maintainability · low] career-core 只加入了 test:shared，未加入 typecheck:shared 的入口文件列表。contracts.ts 会经
packages/api-client/src/index.ts 的相对路径再导出被传递检查，但 desk.ts 不在 typecheck:shared 的 tsc 程序内（仅靠
typecheck:web / miniprogram 各自的 tsconfig 间接覆盖），`pnpm typecheck:shared` 无法兜住 desk.ts 的类型回归；tsx --test
只做类型剥离不做检查。作为与 contracts 同性质的共享合同包，建议在入口列表补齐两个源文件以保持覆盖一致。

-     "typecheck:shared": "tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions --jsx react-jsx packages/contracts/src/index.ts packages/contracts/src/craft/index.ts packages/contracts/src/craft/command-ports.ts packages/api-client/src/client.ts packages/api-client/src/configuration.ts packages/api-client/src/datasource.ts packages/api-client/src/errors.ts packages/api-client/src/index.ts packages/api-client/src/ports.ts packages/api-client/src/transport/json.ts packages/api-client/src/knowledge/documents.ts packages/api-client/src/knowledge/faq.ts packages/api-client/src/knowledge/settings.ts packages/api-client/src/wiki/pages.ts packages/api-client/src/chat/stream.ts packages/api-client/src/chat/sessions.ts packages/api-client/src/chat/approvals.ts packages/api-client/src/chat/steer.ts packages/api-client/src/chat/suggestions.ts packages/api-client/src/sandbox/terminal.ts packages/api-client/src/mobile/executions.ts packages/api-client/src/mobile/runtime.ts packages/api-client/src/settings/index.ts packages/api-client/src/embed/index.ts packages/api-client/src/identity/common.ts packages/api-client/src/identity/tenant.ts packages/api-client/src/identity/organization.ts packages/api-client/src/identity/index.ts packages/api-client/src/administration/index.ts packages/api-client/src/craft/index.ts packages/domain/src/query-key.ts packages/domain/src/knowledge/processing.ts packages/domain/src/knowledge/folders.ts packages/domain/src/knowledge/preview.ts packages/domain/src/wiki/diff.ts packages/domain/src/chat/reducer.ts packages/domain/src/chat/draft.ts packages/domain/src/chat/session-state.ts packages/domain/src/chat/references.ts packages/domain/src/chat/artifacts.ts packages/domain/src/chat/tool-results.ts packages/domain/src/sandbox/terminal.ts packages/domain/src/auth/password-policy.ts packages/domain/src/craft/reconnect.ts packages/domain/src/craft/state.ts packages/domain/src/craft/usage.ts packages/domain/src/craft/version-selection.ts packages/core/src/craft/controller.ts packages/core/src/craft/command-bridge.ts packages/views/src/index.ts packages/views/src/settings/registry.ts packages/views/src/embed/bridge.ts packages/views/src/integrations/registry.ts packages/views/src/craft/presentation.ts packages/views/src/craft/assistant-runtime.tsx packages/views/src/craft/shell.tsx packages/views/src/craft/thread.tsx",
+ ... packages/views/src/craft/thread.tsx packages/career-core/src/contracts.ts packages/career-core/src/desk.ts",


─── pnpm-workspace.yaml:3-3 ───
[maintainability · low] packages/ui 从 workspace 移除且目录已删除，但 apps/desktop/vite.config.ts:38-42 仍残留 5
条指向已不存在的 packages/ui/src/*（button/checkbox/input/textarea/index）的 resolve.alias。apps/desktop 不在
workspace packages 列表内、且其源码已无 @weknora/ui 导入，因此当前是死配置不会实际报错，但保留会误导后续维护者以为该栈仍可用；建议本次退役一并清理这组
alias（或在确认 desktop 已停用后归档其配置）。

-   - packages/career-core
+ # 配套清理 apps/desktop/vite.config.ts 中残留的 ui alias：
+ # 删除 '@weknora/ui/button' / '@weknora/ui/checkbox' / '@weknora/ui/input' / '@weknora/ui/textarea' / '@weknora/ui' 五条指向 packages/ui/src/* 的别名


─── docs/design/job-search/prototype/app.js:150-151 ───
[bug · medium] 重复 id：默认配置（platform=all 且 variant=C）下 commandView 会在 web/mobile/mini
三个设备帧各渲染一次，id="command-form" 与 id="prompt-input" 在同一文档中出现 3 份，属非法 HTML。当前 submit 依赖事件委托 +
event.target 作用域内的 querySelector 侥幸正确，但任何后续按 id
的全局查询/表单关联（autofill、label、getElementById）只会命中第一个设备帧，跨端操作可能串台。建议 id 按 platform 派生（如
command-form-web）或直接改用 class + closest 判定。

  root.addEventListener('submit', event => {
-   if (event.target.id !== 'command-form') return;
+   if (!event.target.closest('form.composer')) return;


─── docs/design/job-search/prototype/app.js:131-131 ───
[bug · low] navItems.find(...) 结果未做空值保护：一旦未来新增不在 navItems
四项（discover/applications/materials/profile）内的 data-view 取值，find 返回 undefined 后 [1] 会抛
TypeError，中断整个点击委托、setNotice 不执行。当前所有 data-view 均为硬编码合法值尚不可触发，但这是单点 chokepoint，建议加兜底。

-   if (view) { state.view = view.dataset.view; setNotice('已打开' + navItems.find(n => n[0] === state.view)[1]); return; }
+   if (view) { state.view = view.dataset.view; const target = navItems.find(n => n[0] === state.view); setNotice('已打开' + (target ? target[1] : state.view)); return; }


─── docs/design/job-search/prototype/app.js:66-66 ───
[maintainability · low] 死参数/死样式：compact 仅为元素追加 compact 类名，但 app.css 全文不存在任何 .compact
选择器（已检索确认），evidenceCard(compact = false) 同理。该分支永不产生视觉效果，会误导维护者以为存在紧凑态分支，建议删除参数与类名拼接（同步清理
evidenceCard(true) 调用点），或补齐对应 CSS。

- function jobRow(item, index, compact = false) {
+ function jobRow(item, index) {


─── docs/design/job-search/prototype/app.css:14-14 ───
[maintainability · low] 追加式补丁导致同选择器重复定义且前者成为死代码：本行与前段
.journey-hero{...linear-gradient(120deg,var(--td-gray-color-13),var(--td-gray-color-13))}
同特异度、后者胜出，前段 background 永不生效。同类被覆盖的死定义还有 .mobile .details-pane .evidence-card 的
display:none→display:block、:root 的 background gray-3→gray-2、.btn.primary:hover、.orbit-core
等。建议将追加块合并回前段单一定义，避免双份维护与阅读误导（顶部色板已与 tdesign-theme.css 亮色块逐值核对一致，无漂移，硬编码本身有注释声明可保留）。



─── docs/design/job-search/prototype/index.html:12-12 ───
[bug · low] type="module" 与 app.js 中 syncUrl 降级注释矛盾：app.js 未使用任何模块语法（无 import/export），而模块脚本在 file://
直开时会被浏览器 CORS 策略整体拒绝加载，页面完全空白（render() 不会执行）——即注释声称覆盖的「file:// 直开」场景实际根本到不了 syncUrl 的
try/catch，该降级仅对沙箱 iframe（opaque origin）可达。建议移除 type="module" 改为普通脚本，或将 app.js 中注释收窄为仅沙箱
iframe，避免误导后续排查。

-     <script type="module" src="./app.js"></script>
+     <script src="./app.js"></script>


─── docs/design/job-search/prototype/app.js:43-43 ───
[style · low] 嵌套三元：外层 (web ? ... : 手机状态栏 + (mobile ? app-header : mini-header)) 内再嵌一层 mobile 三元，违反项目
JS 规约「禁止嵌套三元」。头部标记本身为静态字面量，建议提取为独立 helper 用 if/return 组织，shell() 主链保持扁平。

-     (web ? '<header class="web-header"><div class="brand"><span class="brand-icon">W</span><strong>WeKnora</strong><span class="brand-suffix">Career</span></div><div class="web-header-right"><span class="header-date">2026 校招季</span><span class="avatar">林</span></div></header>' : '<div class="phone-status"><span>9:41</span><span>●●● ▰</span></div>' + (mobile ? '<div class="app-header"><span class="brand-mini">WeKnora <b>Career</b></span><span class="header-icon">◌</span></div>' : '<div class="mini-header"><span>‹</span><strong>求职空间</strong><span class="wechat-capsule">••• <i></i> ◉</span></div>')) +
+ function deviceHeader(platform) {
+   if (platform === 'web') return '<header class="web-header">…</header>';
+   const status = '<div class="phone-status"><span>9:41</span><span>●●● ▰</span></div>';
+   const head = platform === 'mobile' ? '<div class="app-header">…</div>' : '<div class="mini-header">…</div>';
+   return status + head;
+ }


─── packages/design-tokens/src/styles.css:23-23 ───
[bug · medium] 【round-2 遗留未消除】`@import` 位于第 3-17 行 `:root { ... }` 样式规则块之后，违反 CSS 规范（@import 只能出现在
@charset 与空 @layer 语句之后、所有其他规则之前）。当前仅因 Vite postcss-import 的宽容内联才生效；一旦构建管道切换为规范符合实现（如 Lightning
CSS）或该文件被浏览器原生处理，整个 TDesign 主题（约 200 行 --td-* 令牌及组件覆盖）会被静默丢弃且无任何报错。建议将此 @import（连同注释）移至文件顶部 `:root`
块之前；或按下一条建议直接移除聚合引入。

+ /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。... */
  @import "./tdesign-theme.css";
+ 
+ :root {
+   --wk-color-brand: #07c05f; ...


─── packages/design-tokens/src/styles.css:19-20 ───
[maintainability · medium] 【round-2 遗留未消除】在令牌包聚合入口引入 tdesign-theme.css，会把文件尾部的应用级全局组件样式（暗色
`*::-webkit-scrollbar-*` !important、`input:-webkit-autofill` 的 -webkit-text-fill-color
!important、`.t-input:focus`、`.doc-link`、`.more-icon`、`.t-radio-button` 覆盖）注入 design-tokens
的所有消费方。已核实 `apps/embed/src/styles.css:4` 也以 layer(theme) 引入本文件，而 embed 全程不使用 TDesign——它将被动继承
autofill 强制重着色与暗色滚动条 !important 覆盖，属跨应用样式泄漏，且与 design-tokens「纯令牌」职责不符。同时 apps/web 侧形成「unlayered 直引 +
经本文件 layer(theme) 再带入」的双份引入，依赖 postcss-import 的 skipDuplicates 去重才不膨胀产物。建议：styles.css 保持纯令牌，移除此处
@import，tdesign-theme.css（或至少其应用级组件规则）作为独立入口由 TDesign 消费方显式引入（apps/web 已有直引，仅需保留）。

- /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。
-    注意：消费方若将本文件整体 layer() 引入（如 apps/web layer(theme)），这里的
+ /* TDesign 组件主题改为独立入口（tdesign-theme.css），由 TDesign 消费方显式引入：
+    styles.css 保持纯令牌聚合，避免 embed 等非 TDesign 应用被动带入全局组件覆盖。 */
+ /* （移除此处 @import "./tdesign-theme.css"；apps/web/src/styles.css 已有对该文件的直引） */


─── apps/web/src/styles.css:41-42 ───
[documentation · low] 【round-1/round-2 均已指出、仍未修正】注释声称壳内全宽覆盖位于 platform-shell.td.css，但
`.plat-shell__outlet .wk-page { height: 100%; overflow-y: auto; max-width: none !important; }` 实际在
`apps/web/src/platform/platform-u.css:659`（platform-shell.td.css 内无任何 wk-page
规则，本轮已再次核实）。功能本身成立，但注释指向错误文件会误导后续维护者到错误位置排查层叠问题。

-    PlatformShell 壳内由 platform-shell.td.css 的 .plat-shell__outlet .wk-page
+    PlatformShell 壳内由 platform-u.css 的 .plat-shell__outlet .wk-page
     覆盖为全宽滚动（max-width:none!important，层叠与先前壳层 utility 等价）。 */


─── apps/web/vite.config.ts:15-18 ───
[maintainability · low] 【round-2 遗留未消除】FRONTEND_VERSION 改读仓库根 ../../frontend/package.json
属跨应用目录耦合：frontend/ 缺失（独立检出/子目录裁剪构建）时静默退化为 'unknown'，system 分区前端版本行随之失真且无任何告警，与
T12c「两端版本字符串完全一致」的目标相悖时不易察觉。建议 catch 中输出一次性告警，让退化可观测。

      return frontendPkg.version ?? 'unknown';
    } catch {
+     console.warn('[vite.config] 读取 frontend/package.json 失败，FRONTEND_VERSION 回退为 unknown');
      return 'unknown';
    }


─── packages/design-tokens/src/tdesign-theme.css:92-92 ───
[bug · low] 【round-2 遗留未消除】亮色与暗色两处 `--td-line-height-title-large` 均按
`--td-font-size-title-medium`（16px）计算得 24px，而 TDesign 官方默认按 title-large（20px）计算为 28px。这是 Vue 端
frontend/src/assets/theme.css 的上游笔误被「原样平移」继承（两端一致，保持像素对齐可接受），但 `--td-font-title-large` 消费该值，所有
title-large 文本行高偏挤。round-2 已建议：或两端一并修正，或在此处注释显式标注「已知上游偏差、刻意对齐」，避免后续被当作迁移错误反复排查——本轮仍未添加标注。

- /* 字体配置 */  --td-font-family: var(--app-font-family);  --td-font-family-medium: var(--app-font-family);  --td-font-size-link-small: 12px;  --td-font-size-link-medium: 14px;  --td-font-size-link-large: 16px;  --td-font-size-mark-small: 12px;  --td-font-size-mark-medium: 14px;  --td-font-size-body-small: 12px;  --td-font-size-body-medium: 14px;  --td-font-size-body-large: 16px;  --td-font-size-title-small: 14px;  --td-font-size-title-medium: 16px;  --td-font-size-title-large: 20px;  --td-font-size-headline-small: 24px;  --td-font-size-headline-medium: 28px;  --td-font-size-headline-large: 36px;  --td-font-size-display-medium: 48px;  --td-font-size-display-large: 64px;  --td-line-height-common: 8px;  --td-line-height-link-small: calc(    var(--td-font-size-link-small) + var(--td-line-height-common)  );  --td-line-height-link-medium: calc(    var(--td-font-size-link-medium) + var(--td-line-height-common)  );  --td-line-height-link-large: calc(    var(--td-font-size-link-large) + var(--td-line-height-common)  );  --td-line-height-mark-small: calc(    var(--td-font-size-mark-small) + var(--td-line-height-common)  );  --td-line-height-mark-medium: calc(    var(--td-font-size-mark-medium) + var(--td-line-height-common)  );  --td-line-height-body-small: calc(    var(--td-font-size-body-small) + var(--td-line-height-common)  );  --td-line-height-body-medium: calc(    var(--td-font-size-body-medium) + var(--td-line-height-common)  );  --td-line-height-body-large: calc(    var(--td-font-size-body-large) + var(--td-line-height-common)  );  --td-line-height-title-small: calc(    var(--td-font-size-title-small) + var(--td-line-height-common)  );  --td-line-height-title-medium: calc(    var(--td-font-size-title-medium) + var(--td-line-height-common)  );  --td-line-height-title-large: calc(    var(--td-font-size-title-medium) + var(--td-line-height-common)  );  --td-line-height-headline-small: calc(    var(--td-font-size-headline-small) + var(--td-line-height-common)  );  --td-line-height-headline-medium: calc(    var(--td-font-size-headline-medium) + var(--td-line-height-common)  );  --td-line-height-headline-large: calc(    var(--td-font-size-headline-large) + var(--td-line-height-common)  );  --td-line-height-display-medium: calc(    var(--td-font-size-display-medium) + var(--td-line-height-common)  );  --td-line-height-display-large: calc(    var(--td-font-size-display-large) + var(--td-line-height-common)  );  --td-font-link-small: var(--td-font-size-link-small) /    var(--td-line-height-link-small) var(--td-font-family);  --td-font-link-medium: var(--td-font-size-link-medium) /    var(--td-line-height-link-medium) var(--td-font-family);  --td-font-link-large: var(--td-font-size-link-large) /    var(--td-line-height-link-large) var(--td-font-family);  --td-font-mark-small: 600 var(--td-font-size-mark-small) /    var(--td-line-height-mark-small) var(--td-font-family);  --td-font-mark-medium: 600 var(--td-font-size-mark-medium) /    var(--td-line-height-mark-medium) var(--td-font-family);  --td-font-body-small: var(--td-font-size-body-small) /    var(--td-line-height-body-small) var(--td-font-family);  --td-font-body-medium: var(--td-font-size-body-medium) /    var(--td-line-height-body-medium) var(--td-font-family);  --td-font-body-large: var(--td-font-size-body-large) /    var(--td-line-height-body-large) var(--td-font-family);  --td-font-title-small: var(--td-font-size-title-small) /    var(--td-line-height-title-small) var(--td-font-family);  --td-font-title-medium: var(--td-font-size-title-medium) /    var(--td-line-height-title-medium) var(--td-font-family);  --td-font-title-large: var(--td-font-size-title-large) /    var(--td-line-height-title-large) var(--td-font-family);  --td-font-headline-small: var(--td-font-size-headline-small) /    var(--td-line-height-headline-small) var(--td-font-family);  --td-font-headline-medium: var(--td-font-size-headline-medium) /    var(--td-line-height-headline-medium) var(--td-font-family);  --td-font-headline-large: var(--td-font-size-headline-large) /    var(--td-line-height-headline-large) var(--td-font-family);  --td-font-display-medium: var(--td-font-size-display-medium) /    var(--td-line-height-display-medium) var(--td-font-family);  --td-font-display-large: var(--td-font-size-display-large) /    var(--td-line-height-display-large) var(--td-font-family);  /* 字体颜色 */  --td-text-color-primary: var(--td-font-gray-1);  --td-text-color-secondary: var(--td-font-gray-2);  --td-text-color-placeholder: var(--td-font-gray-3);  --td-text-color-disabled: var(--td-font-gray-4);  --td-text-color-anti: #fff;  --td-text-color-brand: var(--td-brand-color);  --td-text-color-link: var(--td-brand-color);  /* end 字体配置 */
+ --td-line-height-title-large: calc(    var(--td-font-size-title-medium) + var(--td-line-height-common)  );  /* 已知上游偏差：Vue theme.css 即按 title-medium 计算（24px vs 官方 28px），刻意对齐现网 */


─── docs/design/job-search/prototype/app.js:87-87 ───
[bug · low] 交互缺陷（两处叠加）：1) search-strip 输入框未包在 <form> 内，按 Enter 不会触发搜索，必须点击按钮——与方案 C composer（form +
submit 委托，Enter 可用）行为不一致，而该输入框 placeholder 明确引导「一句话指令」；2) 任何点击都经 setNotice→render() 全量重建 DOM，用户在
search-strip 已输入的内容会被重置回硬编码 value，composer 中的草稿（如粘贴的长 JD 文本）也会被清空——例如粘贴 JD 后点了「收藏岗位」或切了
variant，草稿即丢失。建议：将 search-strip 包成 <form> 并在 submit 委托中统一处理；把输入草稿存入 state（searchDraft /
promptDraft），render() 时回填 value。

- '<div class="search-strip"><span>⌕</span><input aria-label="搜索岗位" placeholder="岗位、公司、城市或一句话指令" value="前端开发 · AI 产品团队" />' + actionButton('search', '搜索岗位', 'primary') + '</div>' + pills() +
+ <form class="search-strip" data-role="search-strip"><span>⌕</span><input aria-label="搜索岗位" placeholder="岗位、公司、城市或一句话指令" value="' + esc(state.searchDraft || '前端开发 · AI 产品团队') + '" />' + actionButton('search', '搜索岗位', 'primary') + '</form>'
+ // submit 委托中：if (event.target.matches('[data-role="search-strip"]')) { state.searchDraft = …; … }


─── docs/design/job-search/prototype/app.js:102-102 ───
[maintainability · low] 重复代码：quickActions() 已封装 start/save 按钮组，commandView 的 answer-actions
又内联重写一遍（仅文案略异），收藏标签三元 state.savedJobs.includes(state.selectedJob) ? '已收藏 ✓' : … 在 workbenchView 与
commandView 各出现一次；此外 company · city · salary 元信息行的手拼（esc(job().company) + ' · ' + esc(job().city) +
…）在 workbenchView 详情头、journeyView 聚焦卡、applicationsView 申请头重复三处。后续调整文案或转义策略时容易漏改其一。建议参数化复用：如
quickActions({ startLabel, saveLabel }) 和 jobMetaLine(['company','city','salary']) 两个 helper。

- '<div class="command-layout"><section class="conversation"><div class="prompt-bubble"><div class="prompt-avatar">林</div><p>' + esc(state.prompt) + '</p></div><div class="assistant-message"><span class="assistant-avatar">✦</span><div class="assistant-body"><div class="assistant-name">WeKnora <span>刚刚</span></div><p>我按你的条件整理了 <strong>3 个值得优先看的岗位</strong>。以下是目前最匹配的一份，资格判断基于你确认过的资料。</p><div class="answer-card"><div class="answer-title"><span class="company-mark ' + esc(job().accent) + '">' + esc(job().company.slice(0, 1)) + '</span><div><strong>' + esc(job().title) + '</strong><small>' + esc(job().company) + ' · ' + esc(job().city) + '</small></div><span class="fit-score">' + esc(job().fit) + '<small>%</small></span></div><div class="answer-reason"><span>' + esc(job().tag) + ' · 匹配依据</span><p>' + esc(job().reason) + '</p><p><strong>投递前核实：</strong>' + esc(job().gap) + '</p><div class="source-line">来源：' + esc(job().source) + '</div></div><div class="answer-actions">' + actionButton('start', '准备申请 ↗', 'primary') + actionButton('save', state.savedJobs.includes(state.selectedJob) ? '已收藏 ✓' : '先收藏', 'subtle') + '</div></div><div class="suggestion-row"><button data-action="prompt-next">还有哪些类似岗位？</button><button data-view="materials">帮我准备岗位简历</button><button data-view="applications">查看申请进度</button></div></div></div><form class="composer" id="command-form"><span>✦</span><input id="prompt-input" aria-label="求职指令" placeholder="例如：找上海的 AI 产品实习，适合 2027 届" /><button type="submit">发送 ↑</button></form><p class="composer-hint">支持粘贴招聘链接、JD 或直接描述求职目标。信息需自行核验。</p></section>' +
+ function quickActions(options) {
+   const startLabel = (options && options.startLabel) || '准备这份申请 ↗';
+   const saveLabel = state.savedJobs.includes(state.selectedJob) ? '已收藏 ✓' : ((options && options.saveLabel) || '收藏岗位');
+   return '<div class="action-row">' + actionButton('start', startLabel, 'primary') + actionButton('save', saveLabel, 'subtle') + '</div>';
+ }


─── apps/web/src/chat/chat-header.tsx:161-161 ───
[bug · medium] 重命名输入框 onBlur 即触发 submitTitleEdit，而校验失败（空标题）分支会执行 titleInputRef.current?.focus()
把焦点强制拉回输入框：用户点击页面任何其他位置（Tab 切换、点击发送按钮等）都会触发 blur→校验失败→焦点被程序化拽回，且 role=alert 错误文案在 blur
瞬间突兀插入。这构成焦点陷阱（仅 Escape 或输入有效标题可退出），对键盘与读屏用户不友好，且会导致点击外部区域的操作全部失效。建议空标题时降级为
cancelTitleEdit()（回到展示态）或仅标记错误而不强制 focus，避免程序化劫持焦点。

              onBlur={() => { void submitTitleEdit(); }}
+ // submitTitleEdit 空标题分支建议改为：
+ // if (!title) { cancelTitleEdit(); return; }


─── apps/web/src/chat/chat-header.tsx:126-126 ───
[bug · low] `!menuMode` 守卫永假：menuMode 的类型是 'menu' | 'clear' | 'delete'，恒为非空字符串。该函数的本意是拦截菜单态误调用（在
'menu' 态执行时会落入 else 分支直接调用 onDeleteSession，即静默删除会话而非 no-op），当前仅靠 UI 结构（confirm 视图才渲染危险按钮）兜底。应改为
`menuMode === 'menu'` 才符合语义。

-     if (!menuMode || dangerBusy) return;
+     if (menuMode === 'menu' || dangerBusy) return;


─── apps/web/src/chat/chat-header.tsx:177-178 ───
[maintainability · low] onVisibleChange 中两个分支执行完全等价的逻辑（if 提前 return 与顺序调用均调用
onMenuVisibleChange(visible)），注释声称要区分 trigger='document' 的关闭场景但实现未落实，属冗余死代码，直接调用
onMenuVisibleChange(visible) 即可。

-             if (context?.trigger === 'document') { onMenuVisibleChange(visible); return; }
-             onMenuVisibleChange(visible);
+             onVisibleChange={(visible) => onMenuVisibleChange(visible)}


─── apps/web/src/chat/chat-header.tsx:188-188 ───
[maintainability · low] 两处清理项：1) props.onTogglePin! 非空断言虽有外层条件渲染保护，但可在组件顶部提取 `const togglePin =
props.onTogglePin` 后在回调中直接调用 togglePin(...)，消除 `!` 断言；2) SandboxHeaderToggle 的 props.copy
从未被组件体使用（仅用 label/onOpen），属未使用 prop，应从接口中移除或真正消费，避免误导调用方。



─── apps/web/src/chat/ChatRoutePage.tsx:1931-1932 ───
[maintainability · low] headerSlot 内对 sessions.find((session) => session.id === selectedSessionId)
重复求值 3 次（title / isPinned / renameTitle），且三者共享的 fallback 逻辑（|| copy.newSession）也写了两遍。建议提取局部变量（如
const activeSession = ...），减少每次渲染的重复扫描并便于后续统一维护。

-         title={sessions.find((session) => session.id === selectedSessionId)?.title || copy.newSession}
-         isPinned={sessions.find((session) => session.id === selectedSessionId)?.is_pinned === true}
+         title={activeSession?.title || copy.newSession}
+         isPinned={activeSession?.is_pinned === true}


─── apps/web/src/chat/SessionShareDialog.tsx:8-9 ───
[documentation · low] 顶部注释与实现已矛盾：注释仍称「这里全部用内联 Tailwind utilities（QueryHistorySnapshotDrawer
抽屉同风格）」，但本模块已迁移为 wk-ssd-* 语义类并新增 import './chat-u.css'。过时注释会误导后续维护者继续以内联 utility 方式改动此文件。

- // 模块加载（router.tsx 对 craft 的 lazy 处理同理）——这里全部用内联
- // Tailwind utilities（QueryHistorySnapshotDrawer 抽屉同风格）。
+ // 模块加载（router.tsx 对 craft 的 lazy 处理同理）——样式已平移至
+ // ./chat-u.css 的 wk-ssd-* 语义类（node 测试经 registerHooks 将 css 解析为空模块）。


─── apps/web/src/chat/views-chat-u.css:637-640 ───
[maintainability · low] @keyframes pulse 在本文件定义了两次（page 段末尾与 message-list
段末尾，内容相同），重复定义虽不破坏渲染但属冗余；建议保留一处（或依赖全局已有定义）。另请顺带清理同文件多处迁移工具产生的单行粘连格式（如 `border-top-width:
1px;border-color: #eef1f5;`，chat-u.css 的 .wk-bad-19/.wk-ssd-14 同模式）。



─── apps/web/src/chat/views-chat-u.css:1668-1677 ───
[maintainability · low] .wk-vc-tool-result-2/-8/-14/-15/-24/-25 等规则内存在「先普通声明、后同属性 !important
声明」的死代码模式：如本规则的 font-size: 0.7rem 与 color: #8a94a6 恒被后面的 font-size: 0.8rem !important / color:
rgba(0,0,0,0.6) !important 覆盖。前面两个声明是永不生效的死代码，且双值并存（0.7rem vs 0.8rem）易误导后续维护者改错位置。建议删除无 !important
的重复声明，只保留生效值。



─── packages/views/src/chat/page.tsx:629-631 ───
[maintainability · medium] 会话分支删除内置 ChatHeaderMenu 后，结构回退面的 ⋯ 按钮没有任何 onClick/handler，点击无响应；同时
ChatPageProps.headerUtilityItems 已变为死属性——宿主 ChatRoutePage:1927 仍向 ChatPage 传入，但 page.tsx
内已无任何消费点（headerSlot 内 ChatHeader 使用的是它自己的 props，见 chat-header.tsx:143）。当前 apps/web 已注入 headerSlot
不受影响，但任何未注入 headerSlot 的直接消费方会静默丢失全部会话管理入口。建议：① 从 ChatPageProps 移除 headerUtilityItems 死属性并停止传入；②
回退面按钮补 disabled 或最小操作接线，避免渲染一个不可用的交互控件。

-           <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions}>
+           <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions} disabled title={copy.moreActions}>
              <svg className="t-icon t-icon-ellipsis" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
            </button>
+           {/* 同时从 ChatPageProps 移除已无消费者的 headerUtilityItems，或在此回退面消费它 */}


─── packages/views/src/chat/message-face.tsx:403-403 ───
[bug · medium] 旧渲染路径是 renderMessageHtml({content}, t.invalidImageLink)，会把本地化的无效图片占位文案传入；此处改为
renderChatMarkdown(props.content, {}) 后，invalidImageLabel 回退到 markdown.ts 的 zh-CN
默认占位（INVALID_IMAGE_LINK_PLACEHOLDER，见 markdown.ts:160/266），非中文
locale（en/ja/ko/ru）下的无效图片占位文案将回归中文。AgentStreamAnswerFace 手头已有 props.copy，chat-copy.ts 五个语言表均有
invalidImageLink key（zh:58/en:514/ja:851/ko:1188/ru:1525），应透传本地化值。

-   const html = renderChatMarkdown(props.content, {});
+   const html = renderChatMarkdown(props.content, { invalidImageLabel: props.copy.invalidImageLink });


─── packages/views/src/chat/composer.tsx:538-538 ───
[maintainability · medium] 提及菜单内的搜索输入框随本次重写被移除后，mentionQuery 只剩 openMentions/closeMentions 两处重置为 ''
的写入点（composer.tsx:312/334），再无任何用户写入；第 306 行 filteredMentionOptions 的 `.includes(mentionQuery...)`
过滤因此恒为真，成为死状态 + 死过滤逻辑，用户也失去了在弹层内键入过滤提及项的能力（旧实现有搜索输入）。建议：确认 Vue 同构意图后二选一——删除 mentionQuery
状态与过滤条件（保留全量列表 + 方向键导航），或恢复一个过滤输入并接回 setMentionQuery。



─── packages/views/src/chat/composer.tsx:521-521 ───
[bug · low] kbTipTimer 只有 hover 进入/离开时的清抖逻辑，没有组件卸载时的清理：卸载发生在 300ms 延时窗口内时，定时器仍会触发 setKbTipOpen（React
18 虽不再告警，但属于定时器泄漏，且对频繁挂卸的列表场景会累积）。建议补一个 unmount 清理 effect，与组件内其他 effect（如 focusSignal）保持一致的清理纪律。

-             onMouseLeave={() => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); kbTipTimer.current = window.setTimeout(() => setKbTipOpen(false), 80); }}
+   // 与 kbTipTimer 声明处配套：
+   useEffect(() => () => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); }, []);


─── packages/views/src/chat/message-face.tsx:289-289 ───
[maintainability · low] 此处私有 writeClipboardText 与 message-list.tsx:104
导出的同名函数完全重复（navigator.clipboard 优先 + execCommand 回退）；且自 CopyAnswerButton 迁移到本文件后，message-list
的导出版已无生产消费方（仅 message-list.test.tsx 引用），renderMessageHtml 同样只剩 markdown.test.tsx 引用，两者均沦为 test-only
死导出。建议将剪贴板工具收敛到单一实现（本文件改为 import message-list 的导出，或抽到共享模块），并同步清理 message-list 中不再被生产代码消费的导出。

- async function writeClipboardText(text: string): Promise<void> {
+ // 改为复用 message-list 的导出（或抽到共享模块）：
+ import { writeClipboardText } from './message-list.ts';


─── packages/views/src/chat/message-list.tsx:220-220 ───
[bug · medium] agentEventStream 在 packages/ 与 apps/web/src/（含 apps/embed）全库均无写入点——唯一出现即此读取，hasStream
恒为 false，该守卫成为不可达死分支。其后果：is_completed=true 且无 knowledge_references、content 为空但 agent_steps 非空的消息会被
is-empty-segment 整行隐藏（旧实现始终渲染该行并展示工具时间线），完成态消息存在被误吞风险。若意图是保留 rag 历史流恢复的行，应改用本数据流真实存在的字段（如 artifacts
/ agent_steps），或由宿主在恢复历史时补写 agentEventStream 字段。



─── packages/views/src/chat/session-sidebar.tsx:307-307 ───
[maintainability · low] apiOwnerTagOf(session) 在同一行内重复调用 3 次（且每行会话渲染都会重复执行前缀解析），应提取局部常量复用；另外 domain
侧 sessionSourceBadge（IM/Embed 来源徽标）自本次替换后已无生产消费方（仅 session-grouping.test.ts
引用），来源徽标展示行为也从「IM/Embed/API 三类」收窄为「api_external_user/api_tenant_key 合成徽标」，建议确认该行为收窄是有意的 Vue
对齐，并清理或保留 domain 死导出。

-                       {apiOwnerTagOf(session) ? <span className={'session-owner-tag session-owner-tag--' + apiOwnerTagOf(session)!.kind} title={apiOwnerTagOf(session)!.full}>{apiOwnerTagOf(session)!.label}</span> : null}
+                       {(() => { const ownerTag = apiOwnerTagOf(session); return ownerTag ? <span className={'session-owner-tag session-owner-tag--' + ownerTag.kind} title={ownerTag.full}>{ownerTag.label}</span> : null; })()}


─── packages/views/src/chat/page.tsx:687-689 ───
[bug · low] 回底控件由旧实现的 <button>（带 aria-label）改为 div + onClick：无
role/tabIndex/键盘事件，键盘用户无法聚焦与触发，且缺少无文本图标按钮所必需的 aria-label（现仅靠子 svg 的 aria-hidden，无障碍名称为空）。建议改回
<button type="button" aria-label={...}> 或补 role="button" tabIndex={0} + Enter/Space 处理。

-       <div
+       <button
+         type="button"
          className="scroll-to-bottom-btn wk-chat-scroll-bottom"
+         aria-label={copy.chatScrollBottom}
          style={{ display: userScrolledUp ? undefined : 'none' }}


─── packages/views/src/chat/chat-copy.ts:1176-1177 ───
[other · low] ko 表（此处）与 ru 表（约1514行）的 requestInfoEmpty 直接沿用英文 'No request info available'，而
zh（'暂无请求信息'）与 ja（'リクエスト情報はありません'）均已本地化，五语言表填充不一致；同批新增的
noResult/referencesWebCount/referencesDocAndWebCount 则五语齐全。建议补齐 ko（如 '요청 정보가 없습니다'）与 ru（如 'Нет
данных о запросе'）译文。



─── packages/views/src/chat/message-face.tsx:203-207 ───
[style · low] referenceText 使用嵌套三元表达式（违反编码规范：不允许嵌套三元）。doc/web 三分支文案逻辑可提取为局部函数或以提前返回组织，与文件内其他
helper（如 fileExt/formatFileSize）风格保持一致，可读性更好。

-   const referenceText = docCount > 0 && webCount > 0
-     ? props.copy.referencesDocAndWebCount.replace('{docCount}', String(docCount)).replace('{webCount}', String(webCount))
-     : docCount > 0
-       ? props.copy.referencesDocCount.replace('{count}', String(docCount))
-       : props.copy.referencesWebCount.replace('{count}', String(webCount));
+   function referenceSummaryText(docCount: number, webCount: number): string {
+     if (docCount > 0 && webCount > 0) return props.copy.referencesDocAndWebCount.replace('{docCount}', String(docCount)).replace('{webCount}', String(webCount));
+     if (docCount > 0) return props.copy.referencesDocCount.replace('{count}', String(docCount));
+     return props.copy.referencesWebCount.replace('{count}', String(webCount));
+   }
+   const referenceText = referenceSummaryText(docCount, webCount);


─── apps/web/src/chat/ChatRoutePage.tsx:1935-1936 ───
[bug · medium] 双重确认 + 取消被误判为成功：ChatHeader 的 ⋯ 菜单本身已内置 clear/delete 二次确认弹层（本接线已传入
clearConfirmTitle/Body/Action、deleteConfirm* 全套文案），但宿主函数内部还会再弹一次原生确认——clearMessages（L1433
`window.confirm(chatClearConfirmation(...))`）与 deleteSession（L1420
`window.confirm(copy.deleteConfirmBody)`，与弹层用的是同一个 copy key）。后果：1) 用户需对同一段确认文案连续确认两次；2)
若用户在原生弹窗点「取消」，函数静默 return、Promise 正常 resolve，ChatHeader.submitDangerAction
会按成功处理——确认层关闭、菜单复位，无任何失败反馈，用户以为已清空/删除。建议为 clearMessages/deleteSession 抽出跳过 window.confirm 的内核（或增加
skipConfirm 参数）供 ChatHeader 使用，侧栏等旧调用点保留原生确认。

-         onClearSession={clearMessages}
-         onDeleteSession={async () => { await deleteSession(selectedSessionId); }}
+         onClearSession={() => clearMessages({ skipConfirm: true })}
+         onDeleteSession={async () => { await deleteSession(selectedSessionId, { skipConfirm: true }); }}
+ // 并在 clearMessages/deleteSession 增加 skipConfirm 参数：为 true 时跳过 window.confirm（侧栏等旧调用点保持默认原生确认）


─── apps/web/src/chat/ChatRoutePage.tsx:2024-2024 ───
[bug · medium] conversationActionSlot
挂载缺少能力门控：OpportunityImportPanel（../career/OpportunityPage.tsx:29）仅接收 client/scopeController，组件内没有任何
workspace/tenant 能力或特性开关判断（其状态机中的 'forbidden' 只在提交失败后才出现）。ChatRoutePage
是所有空间共享的聊天入口，这会让不具备求职能力的空间在每个会话视图顶部也渲染整套 JD 导入/评估表单，用户填写并提交后才收到
forbidden。建议在挂载点按能力/特性解析结果做条件渲染，或让面板先探测能力再展示表单。

-     conversationActionSlot={<OpportunityImportPanel client={client} scopeController={scopeController} />}
+     conversationActionSlot={careerEnabled ? <OpportunityImportPanel client={client} scopeController={scopeController} /> : undefined}
+ // careerEnabled 由工作台/租户能力或特性开关解析得出


─── apps/web/src/chat/chat.td.css:673-683 ───
[maintainability · low] 同一规则内 color 声明了两次：前面的 var(--td-text-color-secondary) 恒被后面的
var(--td-error-color) 覆盖，属永不生效的死声明（与已确认的 views-chat-u.css tool-result 死声明同模式，此为 chat.td.css
中的新实例）。steer-failure 语义上就是错误提示，建议删除第一个 color 声明，只保留 error 色，避免误导后续维护者改错位置。

  .steer-failure {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
    font-size: 12px;
-   color: var(--td-text-color-secondary);
    margin-bottom: 4px;
    margin-top: 6px;
    color: var(--td-error-color);
  }


─── apps/web/src/chat/views-chat-u.css:220-233 ───
[maintainability · low] 除已确认的 tool-result !important 死声明外，本文件还残留若干同模式的被覆盖前值：.wk-vc-page-23 的
font-size: inherit 恒被末尾 font-size: 13px 覆盖；.wk-vc-page-52 与 .wk-vc-page-53 的 transition-duration:
150ms 恒被 200ms 覆盖；.wk-vc-composer-18/-20/-22/-24 的 transition-duration: 150ms 恒被 120ms
覆盖。这些前值均为永不生效的死代码，且双值并存易误导后续维护者改错位置。建议统一删除被覆盖的前值，只保留生效值。

  .wk-vc-page-23 {
    min-height: 40px;
    resize: none;
    border-radius: 6px;
    border-style: solid;
    border-width: 0;
    padding-inline: 8px;
    padding-block: 6px;
    font-family: inherit;
-   font-size: inherit;
    line-height: inherit;
    font-weight: inherit;
    font-size: 13px;
  }


─── apps/web/src/chat/chat-u.css:376-382 ───
[bug · low] 保真度漂移：SessionShareDialog 中会话标题的原 utility 是 break-all（Tailwind 语义 = word-break:
break-all），此处被译成 overflow-wrap: anywhere——两者断行语义不同（break-all 允许在任意字符间断行、长词会被截断以填满当前行；anywhere
仅在词无法整词放下时才断），长会话标题的换行位置可能与迁移前不一致。本文件头注释约定「值 = utilities 编码的生效值」，此处应译回 word-break: break-all。

  .wk-ssd-6 {
    margin: 0;
    margin-bottom: 12px;
-   overflow-wrap: anywhere;
+   word-break: break-all;
    font-size: 13px;
    color: rgba(23,26,29,0.6);
  }


─── packages/views/src/craft/td.tsx:110-115 ───
[bug · medium] useOverlayFocus 将 onClose 纳入 effect 依赖：调用方（versions.tsx:98、shell.test.tsx:73）均以内联箭头传入
onClose，弹层打开期间父组件任一次重渲染（如 craft 版本列表随轮询/流式更新、locale 切换）都会重跑 effect——cleanup
先把焦点归还给触发元素（触发元素可能已滚出视口，引发滚动跳变），setup 再把焦点抢回面板根节点；用户在弹层 body 内已聚焦的控件（如取消/确认按钮）的焦点位置被反复重置。建议用 ref
持有最新 onClose，effect 仅依赖 [open]，使"聚焦面板/归还焦点"只在开关切换时各执行一次。

+   const onCloseRef = useRef(onClose);
+   onCloseRef.current = onClose;
+   useEffect(() => {
+     if (!open) return undefined;
+     // ...
+     const onKeyDown = (event: KeyboardEvent) => {
+       // ...
+       onCloseRef.current();
+     };
+     document.addEventListener('keydown', onKeyDown);
      return () => {
        document.removeEventListener('keydown', onKeyDown);
        restoreRef.current?.focus();
        restoreRef.current = null;
      };
-   }, [onClose, open]);
+   }, [open]);


─── packages/views/src/craft/td.tsx:101-108 ───
[bug · medium] Drawer/Dialog 均声明 aria-modal="true"（shell.tsx:2 注释亦称 "the td Drawer owns focus
trapping"），但 onKeyDown 只处理 Escape、未实现 Tab 焦点圈禁：Tab 会从弹层逸出到背景内容，而读屏用户依据 aria-modal 将背景视为 inert，构成
WCAG 2.4.3/4.1.2 缺口；且焦点一旦落到面板外（如被聚焦元素被卸载后 activeElement 回落
body），`!panelRef.current.contains(active)` 防御会使 Escape 静默失效，弹层无法键盘关闭。建议在 keydown 中实现 Tab
循环回绕（首/尾可聚焦元素），或最低限度在实现圈禁前移除 aria-modal，避免对读屏用户作出虚假 inert 承诺。

      const onKeyDown = (event: KeyboardEvent) => {
-       if (event.key !== 'Escape') return;
+       if (event.key === 'Escape') {
-       const active = typeof document !== 'undefined' ? document.activeElement : null;
+         const active = typeof document !== 'undefined' ? document.activeElement : null;
-       if (active && panelRef.current && !panelRef.current.contains(active)) return;
+         if (active && panelRef.current && !panelRef.current.contains(active)) return;
-       event.preventDefault();
+         event.preventDefault();
-       event.stopPropagation();
+         event.stopPropagation();
-       onClose();
+         onClose();
+         return;
+       }
+       if (event.key === 'Tab' && panelRef.current) {
+         // 首尾可聚焦元素回绕，保证 Tab 不逸出弹层（兑现 aria-modal 承诺）
+       }
      };


─── packages/views/src/craft/td.tsx:205-205 ───
[maintainability · low] 两处关闭钮（Drawer 的 t-drawer__close-btn div（166 行）、Dialog 的 t-dialog__close
span）为 role="button" 但均无 tabIndex 与 Enter/Space 激活——键盘用户无法触达关闭控件（目前仅靠 Escape
兜底，与"维持原键盘/读屏契约"的自我声明不符）。另外 Dialog 关闭钮携带静态内联 style={{ marginLeft: 'auto'
}}，违反"避免静态内联样式"规范，tdesign.css 的 __header 已是 flex 布局，建议移入类名。

-               <span className="t-dialog__close" style={{ marginLeft: 'auto' }} role="button" aria-label={closeLabel} onClick={onClose}>
+               <span
+                 className="t-dialog__close"
+                 role="button"
+                 tabIndex={0}
+                 aria-label={closeLabel}
+                 onClick={onClose}
+                 onKeyDown={(event) => {
+                   if (event.key === 'Enter' || event.key === ' ') {
+                     event.preventDefault();
+                     onClose();
+                   }
+                 }}
+               >


─── packages/views/src/craft/td.tsx:119-122 ───
[maintainability · low] useBodyPortal 名不副实：函数只返回布尔值作为树内渲染开关，并未创建 body portal，与文件头"fixed
定位树内渲染"的设计容易让后来者误解为 portal 方案（注释里还写着"宿主 tdesign 命令式 API 挂在 body 上"的对照语境）。建议更名为 useClientMounted /
useInlineOverlayRender 等布尔语义命名——当前仅 td.tsx 内部 Drawer/Dialog 两处调用，可安全更名。

- function useBodyPortal(open: boolean): boolean {
+ function useClientMounted(open: boolean): boolean {
    // 树内渲染：仅在浏览器（非 SSR 静态标记）且 open 时挂载。
    return open && typeof document !== 'undefined';
  }


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-52 ───
[bug · medium] 去抖保存存在两处静默丢失配置的边界：(1) 定时器触发时若上一轮 save 仍在途（savingRef.current === true），save()
直接早退且不重排队，用户在请求窗口内的最后一次开关/模型改动被丢弃，UI 显示与服务端持久值不一致且无任何提示；(2) 保存成功后 onSaved 触发壳层回刷
initialValue，useEffect([initialValue]) 会整体覆写 enabled/embeddingModelId 与
latestRef，回刷期间用户的新编辑同样被清掉。该模式与 ConfigSettingsPanel 的旧实现（savingRef 早退 + initialValue effect
整体重置）一致，属平移时带入的既有缺陷，建议在新面板修复：在 finally 中检测 latestRef 与 saved 的差异并重新 scheduleSave，而非依赖用户再次操作。

    async function save(next: ChatHistoryConfig) {
      if (savingRef.current) return;
+     // ...
+     } finally {
+       savingRef.current = false;
+       // 保存完成后若用户在途有新改动，重新排队而非丢弃
+       const latest = latestRef.current;
+       if (latest.enabled !== latest.saved.enabled || latest.embedding_model_id !== latest.saved.embedding_model_id) scheduleSave();
+     }


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:56-56 ───
[maintainability · low] `patch as never` 是多余的强转：settingsConfigPatch 返回 Record<string, unknown>，而
client.settings.chatHistory.config.update（packages/api-client/src/settings/index.ts kvApi）的入参
SettingsPayload 即 Record<string, unknown>，直接可赋值（同文件 ConfigSettingsPanel 的旧调用 `api.update(patch)`
即无强转）。as never 会向任何目标类型收敛，一旦 api-client 未来将载荷收紧为具体类型，该调用点不会产生编译报错，等于移除了这处的类型保护。建议直接传 patch。

-       const saved = await client.settings.chatHistory.config.update(patch as never);
+       const saved = await client.settings.chatHistory.config.update(patch);


─── apps/web/src/settings/CloudSettingsPanel.tsx:316-316 ───
[security · low] 使用 dangerouslySetInnerHTML 注入 t('settings.weknoraCloud.usageSteps') 渲染多行说明：旧实现是
split('\n') + <span> 的安全节点渲染，新实现把翻译文案经 replace 后未转义直接进 innerHTML。当前五种语言包均为静态开发者文案、无用户输入路径（与 Vue 原版
v-html 行为一致），现实风险低，但违反 innerHTML 安全基线——任一语言包后续引入尖括号/标签即成为标记注入面，且相比旧实现没有任何收益。建议恢复节点渲染。

-         <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
+         <p className="hint-text">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="block">{line}</span>)}</p>


─── apps/web/src/settings/McpSettingsPanel.tsx:1301-1303 ───
[maintainability · low] 此 TTextarea（usageInstructions 编辑框）沿用驼峰 maxLength={16000}，而同文件上方 TInput（name
字段）已正确改为 tdesign 的全小写 maxlength={128}——两处拼写不一致。tdesign-react 组件 props 为 maxlength，驼峰写法不被组件识别（仅靠
React 向 DOM 透传兜底），tdesign 侧的长度限制能力不生效，且与同文件约定相悖。建议统一为 maxlength={16000}。

-                       onChange={(value) =>
-                         setField("usageInstructions", String(value))
-                       }
+                     <TTextarea
+                       rows={5}
+                       maxlength={16000}
+                       ...


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:351-351 ───
[other · low] 迁移到 tdesign-react 后丢失了可访问性标注：Switch（自动检查更新）不再带
aria-label={autoUpdateCopy.label}；语言/主题/字体三处 Select 也删掉了 aria-label，仅以 placeholder 充当提示——placeholder
不能作为可访问名称，读屏器在聚焦时会报空标签控件。建议恢复 Switch 的 aria-label，并为三处 Select 补回各自 aria-label（或改用 label 属性）。

-             <Switch value={autoCheckUpdate} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} />
+             <Switch value={autoCheckUpdate} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} aria-label={autoUpdateCopy.label} />


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:127-127 ───
[other · low] 帮助弹层从手写 onMouseOver/onFocus + aria-expanded 迁移到 Popup trigger="hover"
后，键盘用户彻底失去打开途径（hover 触发不响应 focus），触发按钮也不再携带 aria-expanded，读屏器无法感知弹层开合。旧实现明确支持 onFocus 打开。建议 Popup 改为
trigger={['hover', 'focus']}（或手动受控），并在按钮上补回 aria-expanded。

-         <button type="button" className="hint-trigger" aria-label={t('envVarSettings.helpAria')}>
+         <button type="button" className="hint-trigger" aria-label={t('envVarSettings.helpAria')} aria-expanded={helpOpen}>
+         {/* 并将 Popup 设为 trigger={['hover', 'focus']} 或受控 visible */}


─── packages/views/src/craft/td.tsx:109-109 ───
[bug · low] 嵌套弹层时 Escape 会双关关闭：Drawer 与 Dialog 的 keydown 监听都注册在 document 上，而 event.stopPropagation()
无法阻止同一元素（document）上的其他监听器（那需要 stopImmediatePropagation）。versions.tsx 的 CraftVersionsDrawer（头注释自称
"the versions drawer content"）内部的确认 Dialog 是设计为渲染在 CraftDrawer children 内的组合——接线后 Drawer 的监听先注册，且其
contains(activeElement) 守卫对嵌套在自身 DOM 内的 Dialog 焦点判定为真，按一次 Escape 会同时执行两层
onClose，确认弹窗与抽屉一起被关闭。建议：记录当前打开的 overlay 层级（如模块级栈），Escape 仅由最顶层处理；或改用 stopImmediatePropagation
并保证后打开者后注册（依赖注册顺序较脆弱，不推荐）。

-     document.addEventListener('keydown', onKeyDown);
+ // 以模块级 overlay 栈协调：Escape 仅最顶层弹层响应
+ const overlayStack: symbol[] = [];
+ // effect 内：
+ if (overlayStack[overlayStack.length - 1] !== id) return; // 非顶层直接忽略
+ event.preventDefault();
+ onClose();


─── packages/views/src/craft/td.tsx:145-147 ───
[maintainability · low] closeLabel 默认值 'Close' 硬编码英文：craft 域全部可见文案均走 zh/en 双语契约（versions.tsx 的 zh ?
... : ... 、strings 文案表），但 Drawer 与 Dialog 两处关闭钮的 aria-label 默认固定为英文 'Close'，且当前无任何调用方传入
closeLabel——中文 locale 下读屏用户会听到英文标签，与域内 i18n 口径不一致。建议将默认值交由调用方按 locale 传入（versions.tsx 已有 zh 变量可传
'关闭'），或在组件内提供中英文案回退。

-   className,
-   closeLabel = 'Close',
- }: CraftDrawerProps) {
+ // td.tsx 内提供双语回退（或由调用方显式传入）：
+ function defaultCloseLabel(): string {
+   return typeof navigator !== 'undefined' && navigator.language?.toLowerCase().startsWith('zh') ? '关闭' : 'Close';
+ }
+ // 调用方（versions.tsx 等已有 zh 变量）显式传：
+ <Dialog ... closeLabel={zh ? '关闭' : 'Close'}>


─── apps/web/src/settings/McpSettingsPanel.tsx:1237-1242 ───
[bug · medium] 数值输入迁移到 TInput 时丢失了 type="number"：onChange 里 `Number(String(value))` 对非数字输入（如 "3a"）产生
NaN 写入 draft，受控 value 回显 `String(NaN)` = "NaN"，用户输入框会立即变成 "NaN"（Draft 类型 number | "" 也不拦截
NaN）。旧实现靠原生 number input（min/max）过滤非数字。虽然 onBlur 的 normalizeMcpAdvancedNumber 会把 NaN
归一为默认值、buildMcpConnectionPayload 侧也有兜底，但输入期间的显示已损坏。timeout / retryCount / retryDelay 三处同样问题；另外三处
`className="wk-mcp-number-input"` 均为顶格缩进（<TInput 换行后未对齐），建议一并修正。

                          <TInput
- className="wk-mcp-number-input"
+                           className="wk-mcp-number-input"
+                           type="number"
                            value={draft.timeout === "" ? "" : String(draft.timeout)}
-                           onChange={(value) => setField("timeout", String(value) === "" ? "" : Number(String(value)))}
+                           onChange={(value) => {
+                             const text = String(value);
+                             if (text === "") { setField("timeout", ""); return; }
+                             const parsed = Number(text);
+                             setField("timeout", Number.isNaN(parsed) ? "" : parsed);
+                           }}
                            onBlur={() => setField("timeout", normalizeMcpAdvancedNumber(draft.timeout, 30, 1, 300))}
                          />


─── apps/web/src/settings/CloudSettingsPanel.tsx:133-137 ───
[bug · low] trim 校验丢失：旧实现（save() 与按钮 disabled）都用 `!appId.trim() || !appSecret.trim()` 判断；迁移后
`!form.appId || !form.appSecret` 使纯空格输入绕过本地化 warning（fillRequired）直接进入 saveCredentials，最终落入
cloudCredentialPatch 抛出的未本地化英文异常 "WeKnora Cloud app ID is required"，且按钮在纯空格时也不再禁用。建议恢复 trim
判断，并同步修正下方保存按钮的 `disabled={!form.appId || !form.appSecret}`。

    async function handleSave() {
-     if (!form.appId || !form.appSecret) {
+     if (!form.appId.trim() || !form.appSecret.trim()) {
        pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');
        return;
      }


─── apps/web/src/settings/ModelSettingsPanel.tsx:993-996 ───
[bug · medium] 卡片 onKeyDown 未过滤事件来源：内部「更多」下拉按钮（model-card__more）与删除按钮是真实 <button>，在其上按 Enter 时
keydown 会冒泡到卡片，与按钮自身动作叠加触发 openEdit（点击路径有 model-card__actions 的 stopPropagation 保护，键盘路径没有）。同时
role="button" 的卡片内嵌 TDropdown/TButton 属于 ARIA 交互控件嵌套违规。建议至少加 event.target 守卫，并考虑 Space 键支持。

-                 role={canEdit ? "button" : undefined}
-                 tabIndex={canEdit ? 0 : undefined}
-                 onClick={canEdit ? () => openEdit(model) : undefined}
-                 onKeyDown={canEdit ? (event) => { if (event.key === "Enter") openEdit(model); } : undefined}
+ onKeyDown={canEdit ? (event) => {
+   if (event.target !== event.currentTarget) return;
+   if (event.key === "Enter" || event.key === " ") { event.preventDefault(); openEdit(model); }
+ } : undefined}


─── apps/web/src/settings/PersonalMemoryPanel.tsx:196-196 ───
[bug · medium] TDesign InputNumber 清空时 onChange 回调 value 为 null（同文件 ModelSettingsPanel 已为此新增
fromTInputNumber，注释引用 OCR ocr2-016），此处
Number(null)===0：extract_delay_seconds/interest_threshold/max_items 的 0 会被 memoryWorkspacePatch
校验拒绝，500ms 防抖后弹 saveFailed 错误 toast；extract_min_interval_seconds 的 0 通过校验被静默上送服务端，而 readDraft 又把 0
当默认值回显，造成本地草稿与服务端状态不一致。四处 InputNumber（196/206/254/278 行附近）同病。建议将 fromTInputNumber 提取为共享工具并统一复用。

-             <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_delay_seconds: Number(value) }); debouncedSave(); }} />
+ // fromTInputNumber 建议提取到共享模块（ModelSettingsPanel 已有同款）：
+ onChange={(value) => { const n = fromTInputNumber(value); update({ extract_delay_seconds: n === '' ? 5 : n }); debouncedSave(); }}


─── apps/web/src/settings/PlatformApiKeysPanel.tsx:5-5 ───
[maintainability · low] 裸名 Button 导入后在本文件已无任何使用点（全部按钮均为 TButton 或原生 button），属未使用死导入；且与第 2 行构成对
'tdesign-react' 的两条重复导入语句。建议合并为一条并移除裸名 Button。

- import { Alert, Button } from 'tdesign-react';
+ import { Alert, Button as TButton, Checkbox as TCheckbox, Input as TInput } from 'tdesign-react';


─── apps/web/src/settings/ModelSettingsPanel.tsx:967-967 ───
[maintainability · low] loading 恒为 false 的 TLoading 包装属迁移遗留：加载态永远不会出现，只是徒增一层 DOM 嵌套。若后续没有接线 loading
的计划，建议移除包装直接渲染列表区。



─── apps/web/src/settings/PersonalMemorySettingsPanel.tsx:807-811 ───
[bug · low] 可达性回退：迁移前 usage 弹层由受控 hover/focus/blur + onClick 切换实现，带 aria-expanded，键盘用户 Tab
聚焦即可查看说明；改为 TPopup trigger='hover' 后仅鼠标悬停可触发，键盘/焦点路径失去触达方式，aria-expanded 也一并丢失。建议改 trigger='focus'
或受控 visible 并恢复 aria-expanded。

-           <TPopup
-             // Vue placement="bottom-start"；react 1.18.3 PopupPlacement 无
-             // -start 粒度（库间差异，hover 弹层几何，稳态扫描不可见）。
              placement='bottom-left'
-             trigger='hover'
+             trigger='focus'


─── apps/web/src/settings/PersonalMemoryPanel.tsx:70-71 ───
[maintainability · low] 渲染期间直接写 ref（draftRef.current = draft）属渲染副作用，在 StrictMode
双渲染/并发渲染语义下不保证与已提交状态一致。建议移入 useEffect；500ms 防抖触发时 effect 已提交最新草稿，读取语义不变。

    const draftRef = useRef(draft);
-   draftRef.current = draft;
+   useEffect(() => { draftRef.current = draft; }, [draft]);


─── apps/web/src/settings/SystemAuditLogPanel.tsx:93-93 ───
[bug · medium] CSS 作用域回归：本面板同时被 SettingsPage（.wk-settings-drawer-root 内）和
AdministrationPage（.wk-page 根、抽屉外）挂载，但表格分支的 .wk-audit-table / .wk-audit-tag--* / .wk-audit-load-more
/ .wk-audit-time 等样式全部限定在 .wk-settings-drawer-root 作用域下（settings-wrapper.css ~1073-1095 行）。旧实现用
Tailwind 任意变体内联类（上下文无关），迁移后在 AdministrationPage 中渲染的审计表格完全无样式、tone 标签失色。与同文件 §18 的 .system-audit-log
非作用域写法（注释明确"AdministrationPage 移动端挂载同享"）不一致。建议将表格族样式改为非作用域（或将选择器同时挂 .system-audit-log 根）。

- <div><div className="wk-audit-table-wrap"><table className="wk-audit-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => { const date = auditDateParts(text(row, 'created_at'), locale); const target = auditTargetSummary(row); return <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td><div className="wk-audit-time"><span>{date.date}</span><span>{date.time}</span></div></td><td><div className="wk-audit-actor"><span>{text(row, 'actor_user_id') === '—' ? copy.system : text(row, 'actor_user_id').slice(0, 8)}</span>{text(row, 'actor_role') !== '—' ? <small>{text(row, 'actor_role')}</small> : null}</div></td><td><span className={'wk-audit-tag ' + (AUDIT_TAG_TONES[auditOutcomeTone(row.action)] ?? AUDIT_TAG_TONES.neutral)}>{text(row, 'action')}</span></td><td><div className="wk-audit-target"><strong>{target.key}</strong>{target.diff ? <small>{target.diff}</small> : null}</div></td><td><span className={'wk-audit-tag ' + (AUDIT_TAG_TONES[auditOutcomeTone(row.outcome)] ?? AUDIT_TAG_TONES.neutral)}>{text(row, 'outcome')}</span></td></tr>; })}</tbody></table></div>{cursor > 0 ? <button type="button" className="wk-audit-load-more" onClick={() => void loadMore()} disabled={loading}>{loading ? copy.loading : copy.more}</button> : null}</div></div>}
+ /* settings-wrapper.css：将 .wk-settings-drawer-root .wk-audit-* 选择器去作用域（与 .system-audit-log / .rq-* 一致），
+    保证 AdministrationPage（.wk-page 根）挂载时表格/标签样式同样生效 */
+ .wk-audit-table { ... }
+ .wk-audit-tag--success { ... }


─── apps/web/src/settings/ResourceSettingsPanel.tsx:231-235 ───
[bug · medium] 数字输入迁移为 TInput 时丢失了原有的 min/max 约束（原为 min={field.min ?? 1}、max={field.max ??
(isReplicaField(field.name) ? 10 : 64)}）与 type="number" 语义，越界副本数/索引参数可原样提交后端，可能引发向量库创建/连接失败；同时第 13
行的 isReplicaField 导入已无任何使用点，成为死导入。建议换用 TInputNumber 或在 onChange 中恢复范围钳制。

-       <TInput
-         value={raw}
+       <TInputNumber
+         value={raw === '' ? undefined : Number(raw)}
+         min={field.min ?? 1}
+         max={field.max ?? (isReplicaField(field.name) ? 10 : 64)}
          placeholder={field.default != null ? String(field.default) : ''}
          onChange={(value) => {
-           const text = String(value).trim();
+           if (value == null) { onChange(undefined); return; }
+           onChange(Number(value));
+         }}
+       />


─── apps/web/src/settings/SystemInfoPanel.tsx:93-93 ───
[maintainability · medium] 版本不匹配警示 tag 用 index === 1 魔法下标锚定到"前端版本"行：surface.ts 的 systemInfoRows
当前行序恰为 0=versionLabel、1=frontendVersionLabel，但后续行均为条件渲染，任何人调整行序或插入新行即错位（告警挂错行或漏告警）。建议改按
row.labelKey（或为行增加 dedicated 标志位）锚定，消除对数组顺序的隐式依赖。

-                   {index === 1 && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}
+                   {row.labelKey === 'system.frontendVersionLabel' && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}


─── apps/web/src/settings/SandboxSettingsPanel.tsx:1631-1637 ───
[maintainability · medium] SandboxBackendBadge 组件（含 providerLogo('sandbox', type) 查询与 mono mask 的
--logo-url 注入逻辑）与 SkillSettingsPanel.tsx 内的实现完全重复（约 10 行 ×2）。后端徽章色调/图标映射或 logo 逻辑一旦调整，两处必然漂移（本文件
cube→server、其余→cloud 的映射已在两处各写一份）。建议抽取到共享模块（如 providerLogos.ts 旁或 settings 域共享组件文件），两个面板统一引用。

- function SandboxBackendBadge({ type, size = 'md' }: { type: string; size?: 'xs' | 'sm' | 'md' }) {
-   const logo = providerLogo('sandbox', type);
-   const iconName = type === 'cube' ? 'server' : type === 'disabled' ? 'minus-circle' : 'cloud';
-   const className = `sandbox-badge sandbox-badge--${type} sandbox-badge--${size}${logo?.mode === 'mono' ? ' sandbox-badge--mono' : ''}`;
-   const style = logo?.mode === 'mono' ? { '--logo-url': `url("${logo.url}")` } as CSSProperties : undefined;
-   return <span className={className} style={style} aria-hidden="true">{logo ? null : <TIcon name={iconName} />}</span>;
- }
+ /* 抽取到 apps/web/src/settings/SandboxBackendBadge.tsx 并两处统一 import：
+    export function SandboxBackendBadge({ type, size = 'md' }: { type: string; size?: 'xs' | 'sm' | 'md' }) { ... } */
+ import { SandboxBackendBadge } from './SandboxBackendBadge.tsx';


─── apps/web/src/settings/SkillSettingsPanel.tsx:237-239 ───
[maintainability · medium] 此处 SandboxBackendBadge 为与 SandboxSettingsPanel.tsx
逐字重复的第二份实现（providerLogo('sandbox', type) + mono mask + cube→server/disabled→minus-circle/其余→cloud
映射全部相同），见对该文件的对应意见；建议两处合并为共享组件，避免后续徽章逻辑改动时漂移。



─── apps/web/src/settings/SandboxSettingsPanel.tsx:1902-1902 ───
[bug · low] 沙箱网格卡片（role="button"、tabIndex={0}）的 onKeyDown 由原 Enter/Space 双键收窄为仅 Enter，键盘 Space
激活路径丢失；同文件模板卡（onTemplateCardClick 处）仍保留双键，行为不一致。对 role=button 的元素，Space 是标准激活键，建议恢复并 preventDefault
防止页面滚动。

- onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter') openEdit(item); },
+                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openEdit(item); } },


─── apps/web/src/settings/RuntimeQueuesPanel.tsx:484-486 ───
[maintainability · low] ClockIcon 组件定义后无任何使用（原先唯一的消费点 rq-updated-at 已改为 <TIcon name="time"
/>），成为死代码；同文件 ErrorIcon/InfoIcon/ChevronIcon/ServerIcon 仍在使用，仅此一处可删。

- function ClockIcon() {
-   return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>;
- }
+ /* 已被 <TIcon name="time" /> 取代，删除整个 ClockIcon 定义 */


─── apps/web/src/settings/TenantAuditDrawer.tsx:136-138 ───
[documentation · low] 注释陈述已过时："现状即未定义 keyframes 的 no-op"不成立——sheet-in-right 的 @keyframes 已在 packages
内 shared/wk-legacy.css:47 定义，且经 wk-legacy.tsx（SettingsPage 等面板 import WkStatus/WkCard）全局加载生效，该
inline animation 实际会播放。误导性注释会诱导后续维护者误删动画或误判依赖，请更正为实际情况（keyframes 由 wk-legacy.css 提供）。

    // S6 Tailwind 收编：chrome utilities → settings.td.css §7e 的
-   // .wk-audit-drawer-*（portal 挂 body，unscoped）。inline animation 沿用
-   // sheet-in-right 名（现状即未定义 keyframes 的 no-op，与 packages/ui 删除无关）。
+   // .wk-audit-drawer-*（portal 挂 body，unscoped）。inline animation 的
+   // sheet-in-right keyframes 由 shared/wk-legacy.css 提供（经 wk-legacy.tsx 导入生效）。


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:8-8 ───
[maintainability · low] 本次改动在 react 导入中新加了 useRef，但文件内无任何使用点（仅此一处匹配），属死导入；建议移除以免 lint 噪音并误导后续阅读。

- import { useEffect, useMemo, useRef, useState } from 'react';
+ import { useEffect, useMemo, useState } from 'react';


─── apps/web/src/auth/login.td.css:82-85 ───
[bug · medium] `.login-layout { min-height: 100% }`
在当前应用中不生效：全仓库（index.html、styles.css、main.tsx、router.tsx）均未建立 html/body/#root 的高度链，路由直接把 LoginPage
挂在高度为 auto 的 #root 下，百分比 min-height 对 auto 高度包含块按规范解析为 0。结果是页面高度完全由内容决定，在视口高于内容（约 900px，1080p+
全屏浏览器常见）时底部露出 body 的 #eee 背景，渐变不满屏——而迁移前实现为 `min-h-screen`（100vh），始终铺满视口。这是从 100vh 降级到失效 100%
的视觉回归。建议改回 100vh，或在全局补齐 html/body/#root 高度链以对齐 Vue 事实源的前置条件。

  .login-layout {
    display: flex;
    width: 100%;
-   min-height: 100%;
+   min-height: 100vh;


─── apps/web/src/auth/LoginPage.tsx:413-413 ───
[bug · low] 轮播分页指示器由迁移前的 `<button aria-label onClick>` 改为纯 `<span onClick>`：无 tabIndex、无
role、无键盘事件，键盘用户完全无法切换 slide（span 不可聚焦）。真实 swiper 的可点击 pagination bullet 自带 tabindex="0" +
role="button" + 键盘处理，旧 React 实现也用原生 button；同页语言菜单项尚保留 tabIndex+Enter
处理，分页未同步，构成可访问性回归。建议补齐焦点与键盘语义（或恢复 button 元素）。

-                 <span key={slide.titleKey} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} />
+                 <span key={slide.titleKey} role="button" tabIndex={0} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') setSlideIndex(index); }} />


─── apps/web/src/auth/WorkspaceOnboardingPage.tsx:171-176 ───
[bug · low] 邀请列表弹窗丢失了迁移前的 `closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}`：WkDialog 的
closeLabel 默认值为硬编码英文 'Close'，本页支持运行时语言切换（msg(locale, ...)），非英文 locale 下关闭按钮的 aria-label 将退化为英文，构成
i18n/可访问性回退。建议恢复传入本地化的 closeLabel。

      <TDialog
        open={invitationsVisible}
        title={msg(locale, 'auth.workspaceOnboarding.invitations')}
        onClose={() => setInvitationsVisible(false)}
+       closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}
        className="wk-onb-11"
      >


─── apps/web/src/auth/login.td.css:789-795 ───
[maintainability · low] 本文件存在两处永不生效的死规则：(1) `.knowledge-node:nth-of-type(n + 13)` /
`.connection-line:nth-of-type(n + 13)`——组件仅渲染 12 个节点与 12 条连线（nodeIcons/lines 数组各 12 项），n+13
在任何断点下恒不匹配，768px 断点的 n+9 同理需依赖同类兄弟计数（animated-bg 内 div 与 svg 混排，nth-of-type 按 div 计数恰好可用，但 1024px 的
n+13 是纯死码）；(2) `.swiper-slide` 中 `transition-property: transform;` 与 `transition-property:
transform, opacity;` 连续声明两次，前者恒被后者覆盖。文件对 t-form-item
死规则有"按事实源原样平移"的注释说明，但这两处没有，会误导后续样式审计与排查。建议删除或补注释标明平移语义。

+ /* 仅渲染 12 个节点/连线，n+13 恒不匹配 —— 删除死规则 */
  @media (max-width: 1024px) {
-   .login-layout .knowledge-node:nth-of-type(n + 13) {
-     display: none;
-   }
-   .login-layout .connection-line:nth-of-type(n + 13) {
-     display: none;
+   .login-layout .showcase-subtitle {
+     font-size: 18px;
    }


─── apps/web/src/agents/AgentsPage.tsx:1179-1179 ───
[bug · medium] 加载失败链路断裂(死属性 + 静默吞错):AgentsPageView 本次迁移已从解构中移除 error/notice 且不再渲染任何错误分支,而本组件仍写入
loadError 并传入 error={loadError}、notice 恒为 null——两个 prop 均为死属性。加载失败时 loading={!data && !loadError} 变
false、data 为空 → 页面直接落入 EmptyState,用户看到"无智能体"而非错误提示,t('common.error') 组装的消息永远不可见。建议:在 load 的 catch
中追加 void MessagePlugin.error(message)(与本次迁移的 toast 反馈策略一致),并删除 error/notice 两个死 prop 及
AgentsPageViewProps 中的对应声明。

-       notice={null}
+     }).catch((loadFailure) => {
+       const message = loadFailure instanceof Error ? loadFailure.message : t('common.error');
+       if (active) setLoadError(message);
+       void MessagePlugin.error(message);
+     });
+     // 并删除视图调用处的 error={loadError} / notice={null} 死属性


─── apps/web/src/agents/AgentsPage.tsx:1006-1008 ───
[bug · low] toggleFavorite 失败回滚竞态:rollback 基于回滚时刻的 favoritesRef 再翻转一次,而非发起乐观更新时的快照。连点场景(第一次 add
的失败响应晚于第二次 remove 的成功响应)中,add 的 catch 会把已被 remove 清除的 id 重新加回,UI 显示已收藏而 DB 实际未收藏,状态背离持续到下次 GET
水合。建议失败时以服务端为准重水合(client.userFavorites.list('agent')),仅在网络也不可用时才退回本地翻转。

      void persist.catch(() => {
-       const rollback = new Set(toggleFavoriteId([...favoritesRef.current], id));
-       favoritesRef.current = rollback;
+       // 以服务端为准重水合,避免覆盖其间已成功的 toggle
+       void client.userFavorites?.list('agent').then((rows) => {
+         const ids = rows.map((row) => row.resource_id).filter(Boolean);
+         favoritesRef.current = new Set(ids);
+         writeFavoriteIds(window.localStorage, viewer.userId, tenantKey, ids);
+         setFavorites(new Set(ids));
+       }).catch(() => { /* 网络不可用时可保留原回滚路径 */ });
+     });


─── apps/web/src/agents/AgentsPage.tsx:255-256 ───
[bug · low] ResourceOriginBadge 双重问题:(1) i18n 缺陷——resourceOrigin.* 键不存在于 packages/i18n(仅 Vue 侧
frontend/src/i18n 有),formatMessage 缺键时按 packages/i18n 逻辑直接返回原始 key 字面串,且此处硬编码 'zh-CN' 完全忽略应用
locale。当前唯一调用点(badge.kind === 'creator')的 creatorName 由 cornerBadge 保证非空故坏文案路径暂不可达,但任一新增
mine/tenant/space/shared 变体调用即触发;KnowledgeBasesPage 已改用 locale prop + 本地文案映射,此处未同步(issue-140
round-1/2 OCR 已记录)。(2) 该 text 计算是 4 层嵌套三元表达式,违反 review 规则"禁止嵌套三元"。建议照 KnowledgeBasesPage 方案收口为映射表。

-   const text = variant === 'mine' ? formatMessage('zh-CN', 'resourceOrigin.mine')
-     : variant === 'creator' ? (creatorName || formatMessage('zh-CN', 'resourceOrigin.tenant'))
+ const ORIGIN_FALLBACK: Record<ResourceOriginBadge['variant'], Record<Locale, string>> = {
+   mine: { 'zh-CN': '我的', 'en-US': 'Mine' },
+   tenant: { 'zh-CN': '空间', 'en-US': 'Space' },
+   /* … */
+ };
+ const text = creatorName || ORIGIN_FALLBACK[variant][locale];


─── apps/web/src/agents/AgentsPage.tsx:342-345 ───
[maintainability · low] a11y 回归:折叠分组标题由 <button> 改为 div[role=button] 时丢失了原有的
aria-expanded={!collapsed},读屏用户无法感知分组折叠状态(键盘 Enter/Space 处理已保留)。补回 aria-expanded 即可。

      <div
        className="agent-section-header"
        role="button"
        tabIndex={0}
+       aria-expanded={!collapsed}


─── apps/web/src/agents/AgentEditorModal.tsx:1207-1207 ───
[security · low] href="javascript:void(0)"(存储设置跳转)与仓库自身安全姿态冲突:navigation.ts 明确过滤 javascript:
协议、citation/upload-pipeline/embed 等多处安全测试断言 javascript: URI 不得出现在 href 中;严格 CSP 下该 URI
会被拦截并计入违规上报。迁移前此处是 <button>,属回归。建议改回 button(保留 data-go-storage-settings 测试锚点)。同款问题另见沙箱设置跳转(约 1662
行)。

-                 <a href="javascript:void(0)" className="go-settings-link" data-go-storage-settings onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=storage'); }}>
+                 <button type="button" className="go-settings-link" data-go-storage-settings onClick={() => navigate('/platform/settings?section=storage')}>


─── apps/web/src/agents/AgentEditorModal.tsx:1662-1662 ───
[security · low] href="javascript:void(0)"(沙箱设置跳转),与上一处存储设置链接同款问题:与 navigation.ts 过滤 javascript:
协议的安全姿态冲突、严格 CSP 下被拦截,且迁移前为 <button>。建议一并改为 button。

-                 <a href="javascript:void(0)" className="go-settings-link" data-go-sandbox-settings onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=sandbox'); }}>
+                 <button type="button" className="go-settings-link" data-go-sandbox-settings onClick={() => navigate('/platform/settings?section=sandbox')}>


─── apps/web/src/agents/AgentEditorModal.tsx:0-0 ───
[maintainability · low] kbWarnTimer 死代码(前两轮已记录、本轮仍未修复):kbWarn 状态删除后,该 setTimeout
回调体为空注释,定时器不产生任何效果,仅卸载清理还引用 ref。应连同 kbWarnTimer 的 useRef 声明(约 282 行)与所有 clearTimeout 点一并删除。

+     if (kbWarnTimer.current !== null) window.clearTimeout(kbWarnTimer.current);
+     if (count > 0) {
        void MessagePlugin.warning(t('agentEditor.agentType.kbIncompatibleWarn', { count }));
-       kbWarnTimer.current = window.setTimeout(() => { /* 4s 后自然消失(MessagePlugin 自身超时) */ }, 4000);
+     }
+     // 并删除 kbWarnTimer 的 useRef 声明及卸载清理逻辑


─── apps/web/src/agents/AgentEditorModal.tsx:702-706 ───
[maintainability · low] a11y 回归:提示词占位符标签由 <button> 改为 Tooltip 内的 <span onClick>,不可 Tab 聚焦、无回车触发、丢失
button 语义(data-placeholder-tag 的 click 测试锚点仍可命中,但键盘路径受损)。建议至少补 role="button" tabIndex={0} 并处理
Enter/Space;同款问题也出现在 AgentsPage 的 AgentRail div 项与删除确认弹窗的 span 按钮(circle-btn-txt)。

            <span
              className="placeholder-tag"
+             role="button"
+             tabIndex={0}
              data-placeholder-tag={def.name}
              onClick={() => insertPromptPlaceholder(target, def.name)}
+             onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); insertPromptPlaceholder(target, def.name); } }}
            >{`{{${def.name}}}`}</span>


─── apps/web/src/agents/AgentEditorModal.tsx:1033-1033 ───
[maintainability · low] as never 类型绕过散布:本文件共 17 处 + AgentParserRules 1 处,其中 inputProps={{
"data-field": … } as never } 模式重复 10+ 次。绕过类型检查意味着 tdesign-react 升级改变 props 形状时将静默失效(data-field
测试锚点丢失、popupProps 样式不再生效;Textarea ref 的 as never 若 ref 形状变化,textareaElement 取值静默为
null,占位符插入退化为尾部追加)。建议提取模块级 helper 集中收口(如 const dataFieldProps = (field: string) => ({ 'data-field':
field }) as never;),把断言面收敛到单点。

-                   inputProps={{ "data-field": "max_completion_tokens" } as never}
+ // 模块级集中定义,收敛 as never 断言面
+ const dataFieldProps = (field: string) => ({ 'data-field': field }) as never;
+ // 用法: inputProps={dataFieldProps('max_completion_tokens')}


─── apps/web/src/agents/agents.td.css:2488-2491 ───
[maintainability · low] 无作用域前缀的全局死规则:.modal-enter-active/.modal-leave-active 及后续 .modal-enter-from
.settings-modal 等过渡类来自 Vue <Transition name="modal">,React 端没有任何代码挂载这些类,规则永不生效;且 faq.td.css:2214
起存在同名全局定义,两处死规则互相覆盖,一旦任一侧被激活即产生跨页样式冲突。同文件后段的
.placeholder-popup-wrapper/.placeholder-popup/.placeholder-item 同为未使用且无前缀的全局类(对应记录在案的 `{{` 自动补全未迁移
gap)。建议删除,或按其他平移段惯例补 .settings-overlay 前缀。



─── apps/web/src/agents/agents.td.css:699-700 ───
[maintainability · low] 同一文件内选择器块重复定义:.agent-create-btn 出现两段(前段含 background 渐变 + hover 态,本段为
--td-button-primary-* 变量覆盖),.agent-list-container .agent-section-header 也出现两段(前段
grid-column/pointer-events,后段 position: sticky
系)。两段之间还隔着其他规则,阅读时极易漏看合并语义。建议各自合并为单块(保留全部声明),消除平移产物的机械重复。



─── apps/web/src/agents/AgentParserRules.tsx:177-177 ───
[maintainability · low] 锚元素无 href:不可 Tab 聚焦、不可回车触发,preventDefault 对无 href 锚亦无意义(键盘与读屏操作路径受损);与
AgentEditorModal 两处 go-settings-link 属同一跳转模式但实现不一致(那边是 javascript:void(0),这边干脆无 href)。建议统一为 <button
type="button" className="go-settings">,保留 data-parser-go-config 锚点。

-                       <a className="go-settings" data-parser-go-config onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=parser'); }}>
+                       <button type="button" className="go-settings" data-parser-go-config onClick={() => navigate('/platform/settings?section=parser')}>


─── apps/web/src/agents/SubagentsSection.tsx:0-0 ───
[maintainability · low] Row 组件第三份局部复刻:setting-row 结构已在
AgentEditorModal(Row)/PersonaSection(Row)/本文件各复刻一份,且能力已开始漂移(AgentEditorModal 版支持
error/vertical/emphasize/highlight,此处仅 label)。建议提取为 agents 域共享的 Row(如
agent-editor-row.tsx),三处统一引用,避免后续布局修正漏改其中一份。

- /** 局部 Row(同 PersonaSection,无 Vue 事实源的 setting-row 复刻)。 */
- function Row({ label, children }: { label: string; children: React.ReactNode }) {
+ // 提取到 agents 域共享模块(如 ./agent-editor-row.tsx),三处复刻统一引用:
+ export function Row({ label, children }: { label: string; children: React.ReactNode }) { … }


─── apps/web/src/agents/MbtiTestModal.tsx:124-124 ───
[style · low] 静态内联样式:本文件三处 style={{ color: 'var(--td-error-color)' }}(loadFailed/submitFailed
等)均为非动态样式,违反"避免内联 style,仅动态值例外"的规则;PersonaSection/SubagentsSection 迁移中也有同款写法。建议沉淀为 agents.td.css
的修饰类(如 .wk-ae-mbti-meta--error),与文件既有 wk-ae-mbti-* 类体系一致。

-           <p className="wk-ae-mbti-meta" role="alert" style={{ color: 'var(--td-error-color)' }}>{t('agentEditor.personalization.testLoadFailed')}</p>
+ /* agents.td.css */
+ .wk-ae-mbti-meta--error { color: var(--td-error-color); }
+ <!-- tsx -->
+ <p className="wk-ae-mbti-meta wk-ae-mbti-meta--error" role="alert">…</p>


─── apps/web/src/auth/LoginPage.tsx:434-435 ───
[bug · medium] 注册成功后邮箱预填失效并造成视图/提交值脱节：Input 不再绑定 React state（值由 tdesign Form 内部数据驱动，onValuesChange
仅单向回写 state）。register 成功路径（submit() 内 setMode('login') 且保留 email state，注释写明 "switch to login and
prefill the email"）触发登录表单重新挂载，此时 initialData 恒为 ""，登录邮箱框显示为空；但用户直接点登录时 submit() 仍以 state 中不可见的旧邮箱发起
login 请求——界面显示为空、实际提交带值，与旧实现（value={email} 回显预填）及 Vue 事实源均不一致。建议登录邮箱 FormItem 改为
initialData={email}（仅在挂载时求值：初始为 ''，注册成功重挂载后回显已注册邮箱）；或放弃预填，在注册成功分支同步 setEmail('')。

              <Form labelAlign="top" layout="vertical" onValuesChange={onLoginValuesChange} onSubmit={({ e }) => { void submit(e); }}>
-               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
+               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData={email}>


─── apps/web/src/auth/LoginPage.tsx:346-346 ───
[maintainability · low] animation-delay 存在两处事实源：login.td.css 的 .node-1..12 / .line-1..12 已逐项声明同名
animation-delay（与 AUTH_NODE_DELAYS/AUTH_LINE_DELAYS 当前值一一相等），而 inline style 恒覆盖类规则，CSS 侧 delay
声明实际是死规则。两处并存后续极易漂移（改 CSS 不生效、改常量被疑为无效果）。建议删除 inline animationDelay（CSS 已覆盖），或反向删掉 CSS 中的 delay
声明只保留常量，收敛为单一来源。

-         <div key={index} className={`knowledge-node node-${index + 1}`} style={{ animationDelay: AUTH_NODE_DELAYS[index] }}>
+         <div key={index} className={`knowledge-node node-${index + 1}`}>


─── apps/web/src/platform/PlatformShell.tsx:70-71 ───
[bug · medium] 【round-1/2/3 遗留未修 · bug·medium】键盘分支仍是死代码：React 合成 KeyboardEvent 没有 `button` 属性（属
MouseEvent 接口），`(event as ReactMouseEvent).button` 为 undefined，`undefined !== 0` 恒真 → Enter keydown
永远早退，既不 preventDefault 也不 navigate。dropdown-user-header（role="button" tabIndex={0}，第 1268 行）的 Enter
激活完全失效，键盘用户无法打开个人设置（a11y 回归）。注释 "keyboard events keep the early return" 把死分支固化了。建议按事件类型区分守卫。

  function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
-   if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   const viaKeyboard = event.type === 'keydown';
+   if (event.defaultPrevented || (!viaKeyboard && (event as ReactMouseEvent<Element>).button !== 0) || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;


─── apps/web/src/platform/PlatformShell.tsx:218-218 ───
[maintainability · medium] 【round-1/2/3 遗留未修 ·
maintainability·medium】career/careerSearch/careerRules 三个导航项 label
硬编码中文（'求职档案'/'找岗'/'持续找岗'），绕过同数组其余条目统一使用的 t()/labels i18n 通道（i18n 资源中已确认无 career 导航词条）——非中文 locale
下全局侧栏将混排未翻译中文，破坏五语言对齐目标。建议补 t('menu.career') 等词条或收进 labels record。

-     { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },
+     { key: 'career', href: '/platform/career', label: t('menu.career'), icon: 'career', iconNode: <ReactOnlyNavIcon paths={[/* ... */]} />, match: (p: string) => p === '/platform/career' }, // 并在 i18n 资源补 menu.career / menu.careerSearch / menu.careerRules 词条


─── apps/web/src/platform/PlatformShell.tsx:767-769 ───
[bug · low] 【round-1 遗留未修 · bug·low】渲染体内直接写 ref（sessionActivityRef.current =
sessionActivityEntries）违反 React 渲染纯性约定：并发模式下渲染可能被丢弃或重放，会短暂写入过期/未提交的值，5s 轮询定时器可能基于过期 entries 计算。建议移入
useEffect 同步（间隔回调读取的仍是提交后的最新值，语义不变）。

    const [sessionActivityEntries, setSessionActivityEntries] = useState<SessionActivityEntries>({});
    const sessionActivityRef = useRef<SessionActivityEntries>({});
-   sessionActivityRef.current = sessionActivityEntries;
+   useEffect(() => { sessionActivityRef.current = sessionActivityEntries; }, [sessionActivityEntries]);


─── apps/web/src/platform/PlatformShell.tsx:772-777 ───
[bug · low] 【round-1 遗留未修 · bug·low】与 Vue 语义偏差：Vue store 同步 watch [auth.user.id,
effectiveTenantId]（flush:'sync'）清空条目，此处只看 client 对象身份。若租户切换复用同一 client 实例（switchTenantFromShell
实现决定），旧租户 running 条目会跨租户保留并继续 5s 轮询，直到服务端 403 才清除，期间侧栏显示他租户会话的 running spinner。最直接的补法：switchTenant
成功后同步清空。

-   const sessionActivityClientRef = useRef<Client | null>(null);
-   useEffect(() => {
-     const clientChanged = sessionActivityClientRef.current !== null && sessionActivityClientRef.current !== client;
-     sessionActivityClientRef.current = client;
-     if (clientChanged) setSessionActivityEntries({});
-   }, [client]);
+   // switchTenant 成功分支内补充（对齐 Vue watch [user.id, effectiveTenantId] 的清空语义）：
+   await onTenantSwitch(tenantId);
+   setSessionActivityEntries({});


─── apps/web/src/platform/PlatformShell.tsx:904-912 ───
[maintainability · low] 【round-1 遗留未修 · maintainability·low】clearShellSessionMessages /
deleteShellSession / deleteShellSessions 三处几乎相同的「按 id 删除 sessionActivity
条目」更新块重复（前两处逐字符相同，第三处为批量版）。建议抽 removeSessionActivity(ids: Iterable<string>) helper
统一维护，避免后续修改删除语义时漏改。

-   async function deleteShellSession(sessionId: string): Promise<void> {
-     try { await client.sessions.remove(sessionId); } catch { return; }
-     // Vue menu.vue handleSessionMutation — removed → sessionActivity.update(id, false).
+   function removeSessionActivity(ids: Iterable<string>): void {
      setSessionActivityEntries((prev) => {
-       if (!(sessionId in prev)) return prev;
        const next = { ...prev };
-       delete next[sessionId];
-       return next;
+       let changed = false;
+       for (const id of ids) {
+         if (id in next) { delete next[id]; changed = true; }
+       }
+       return changed ? next : prev;
      });
+   }
+   // 三处调用点改为 removeSessionActivity([sessionId]) / removeSessionActivity(selected)


─── apps/web/src/platform/PlatformShell.tsx:1071-1073 ───
[bug · low] 【round-1 遗留未修 · bug·low】拖拽监听挂在 document 上，仅在「右拖 >40px 越过阈值」或 mouseup 时经 cleanup()
移除；若组件在拖拽会话进行中卸载（路由切换/租户切换重挂 shell），mousemove/mouseup 监听器泄漏，且越阈回调还会对已卸载组件 setState。建议把监听器生命周期纳入
effect/卸载清理。

+     const cleanup = () => {
+       document.removeEventListener('mousemove', onMouseMove);
+       document.removeEventListener('mouseup', onMouseUp);
+     };
      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', onMouseUp);
-   };
+     // 组件卸载兜底（module 级或 ref 持有当前 cleanup，在 unmount effect 中调用）


─── apps/web/src/platform/PlatformShell.tsx:1187-1187 ───
[style · low] 【round-1 遗留未修 · style·low】嵌套三元（检查清单明确禁止），且 iconPair 缺失时回退 src=''——空字符串 src
会被浏览器解析为当前页面 URL，触发一次多余的页面请求。当前 NAV_ICON_URLS 覆盖四个键、React-only 条目走 iconNode，但未来新增 icon 键与
NAV_ICON_URLS 拼写不一致时会静默退化为空图。建议先解出 navIconSrc 局部变量（undefined 时不渲染 img）。

-                         {item.iconNode ?? <img className="icon" src={iconPair ? (active ? iconPair.active : iconPair.default) : ''} alt="" />}
+                 const navIconSrc = iconPair ? (active ? iconPair.active : iconPair.default) : undefined;
+                 // ...
+                 {item.iconNode ?? (navIconSrc ? <img className="icon" src={navIconSrc} alt="" /> : null)}


─── apps/web/src/platform/PlatformShell.tsx:555-555 ───
[maintainability · low] 【round-1 遗留未修 · maintainability·low】tenantSwitcherVisible 不再看 onTenantSwitch
是否传入，而 switchTenant 对 !onTenantSwitch 静默 return。当前唯一生产挂载点 router.tsx:213 恒传入该 prop，今天无实际故障；但可选 prop
的防御语义被移除后，宿主省略 onTenantSwitch 时切换 glyph 与租户子菜单仍渲染、点击零反馈。同时 shouldShowTenantSwitcher 运行时已无调用方（仅
platform-shell-nav.test.ts 引用），与注释「保留给未来 TenantSelector 挂载」存在偏差。建议保留 hasSwitchHandler
门控或给无处理函数分支禁用态。

-   const tenantSwitcherVisible = !isLiteEdition && user.memberships.length > 0;
+   const tenantSwitcherVisible = !isLiteEdition && user.memberships.length > 0 && Boolean(onTenantSwitch);


─── apps/web/src/platform/PlatformShell.tsx:1-1 ───
[maintainability · low] 【round-3 遗留未修 · maintainability·low】新增的 `import * as React from 'react'`
是死导入：全文件没有任何 `React.` 命名空间引用（类型均通过具名 import 获取，JSX 走 automatic runtime，此前该文件也无此导入）。建议删除。



─── apps/web/src/main.tsx:20-20 ───
[maintainability · medium] 【新发现 · maintainability·medium】应用入口全局 import
'./career/opportunity.css'，将单一业务页样式提升为首屏全局 CSS：career 域其余 11
个页面（application/export-deletion/inbox/material/preparation/progress/rule/search/submission/usage/rec
onciliation）全部在各自页面模块内 co-locate 导入，且 OpportunityPage.tsx 自身并未导入该样式（全仓仅 main.tsx 与其测试 readFileSync
引用）——约定不一致并增大入口体积。OpportunityPage 经路由懒加载，把导入移入页面模块即可保持行为一致。

- import './career/opportunity.css';
+ // main.tsx 删除此行；改在 apps/web/src/career/OpportunityPage.tsx 顶部补：
+ // import './opportunity.css';


─── apps/web/src/platform/platform-u.css:387-388 ───
[style · low] 【新发现 · style·low】.wk-cmdk-36 连续声明两次 transition-duration（150ms 后跟 100ms），前者是 codemod
残留的死声明，后者覆盖生效（原 utility 为 duration-100）。易误导后续维护，建议删除 150ms 一行。

-   transition-duration: 150ms;
    transition-duration: 100ms;


─── apps/web/src/platform/platform-u.css:658-659 ───
[maintainability · low] 【round-1 遗留未修 · maintainability·low】outlet 节点同时挂 plat-shell__outlet
wk-shell-5 两个类，.wk-page 的 height/overflow-y 规则在 `.wk-shell-5 .wk-page` 与此处 `.plat-shell__outlet
.wk-page`（后者多 max-width:none!important）两套选择器下重复定义，优先级不同、需两处同步维护，后续改动易分歧。建议合并为一处（保留带 !important
的完整版本即可）。



─── apps/web/src/platform/platform-shell.td.css:1571-1572 ───
[bug · low] 【round-3 遗留未修 · bug·low】z-index 层级冲突：移动端侧栏 overlay 的 z-index:1001
高于全局命令面板（.wk-cmdk-5，z-index 1000）。窄视口侧栏展开时点击 logo_row 搜索按钮打开 palette，面板会被固定定位的侧栏（左侧 260px
全高）遮住一部分。建议降到 palette 之下（如 999）；aside 内部 .user-dropdown（z-index 1000）在 aside 自身层叠上下文内不受影响。

      inset: 0 auto 0 0;
-     z-index: 1001;
+     z-index: 999;


─── apps/web/src/platform/platform-shell.td.css:1568-1568 ───
[bug · medium] 【新发现 · bug·medium】窄屏 overlay 展开后缺少任何收起通路：aside_box--mobile-overlay 为 fixed
定位但无遮罩层，点击路由内容区不收起侧栏，也无 Esc 关闭；且导航 <a> 的 onClick 只 navigate 不
setNarrowSidebarExpanded(false)——移动端点导航后侧栏继续覆盖左侧 260px 内容，只能回头找折叠按钮。建议：① 导航点击后收起窄屏侧栏；②
渲染遮罩层（点击收起）或监听 Esc。



─── apps/web/src/platform/platform-shell.td.css:56-56 ───
[maintainability · low] 【round-1 遗留未修 · maintainability·low】--sidebar-text-inset 定义后全仓（apps/web
侧）无任何引用，属死代码；另外 .menu_box 在 §1 以 `.aside_box .menu_box { display:flex; flex-direction:column }`
定义、又在文件后段以 `.menu_box { position:relative }` 二次定义，可合并减少维护面。建议删除未用变量并合并重复选择器。



─── apps/web/src/platform/session-activity.ts:30-30 ───
[test · low] 【新发现 · test·low】纯函数核心无任何单测：模块头注自述「便于单测锁定行为」，但全仓（含测试目录）对 detectRunningMessageId /
refreshSessionActivityEntry / refreshSessionActivityError 零引用，而 Vue 侧原型
frontend/src/stores/sessionActivityState.test.ts 有对应测试。建议补齐对边界语义的锁定：403/404 立删、连续 3 次失败阈值、messageId
完成后删除、detached 会话消息消失分支，以及 shell 探测 limit:20 截断（长会话较早运行消息漏检）与「列表中任意位置最后一条未完成 assistant」的取值口径。



─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:24-29 ───
[maintainability · low] 未使用的导入：`isSharedKbEditable` 从 @weknora/domain
导入后在本文件内没有任何调用点（全文件仅此一处出现）。属迁移残留死代码，会误导后续维护者以为卡片逻辑中存在共享可编辑性判断，建议删除。

  import {
    canDuplicateKBCard,
    canManageKBCard,
    isKnowledgeBaseInitialized,
-   isSharedKbEditable,
    groupKnowledgeBaseSections,


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1623-1623 ───
[bug · medium] 自绘编辑器弹层缺少键盘可访问性：该 settings-overlay 仅支持点击遮罩或按钮关闭，丢失了 Escape 键关闭、焦点圈闭（focus trap）与打开期间
body 滚动锁。本页其余浮层（删除确认 Dialog、共享详情 TDrawer）使用的 tdesign 组件均默认提供这些能力；此处全屏模态若无
Escape/焦点管理，键盘与读屏用户会被困在弹层背后的页面内容。建议在弹层打开时挂 window keydown 监听并锁滚动。

-         <div className="wk-kb-settings-overlay" onClick={(event) => { if (event.target === event.currentTarget) setDialogOpen(false); }}>
+ // 在组件内补：
+ useEffect(() => {
+   if (!dialogOpen) return;
+   const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setDialogOpen(false); };
+   window.addEventListener('keydown', onKey);
+   const prevOverflow = document.body.style.overflow;
+   document.body.style.overflow = 'hidden';
+   return () => {
+     window.removeEventListener('keydown', onKey);
+     document.body.style.overflow = prevOverflow;
+   };
+ }, [dialogOpen]);


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1874-1874 ───
[bug · low] 问题生成数量输入未做数值防护：`Number(String(value))` 在用户清空或输入非数字时得到 NaN，输入框会直接显示 "NaN"（编辑器校验
validateEditorForm 也未拦截）。虽然 editor-config.ts 的 payload 处有 `|| 3` 兜底使服务端不落脏数据，但 UI 层显示 NaN
属明显瑕疵。知识库设置域已有现成工具 `clampQuestionCount`（knowledge-settings/editorSections.ts，NaN→1、clamp
1..10），建议复用保持两处行为一致。

- <div className="wk-kbl-1"><div><h3 className="wk-kbl-2">{t('knowledgeEditor.advanced.title')}</h3><p className="wk-kbl-3">{t('knowledgeEditor.advanced.description')}</p></div><label className="wk-kbl-12"><TCheckbox checked={editorConfig.questionGenerationConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.advanced.questionGeneration.label')}</label>{editorConfig.questionGenerationConfig.enabled ? <label className="wk-kbl-4">{t('knowledgeEditor.advanced.questionGeneration.countLabel')}<TInput value={String(editorConfig.questionGenerationConfig.questionCount)} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: Number(String(value)) } }))} /></label> : null}<label className="wk-kbl-12"><TCheckbox checked={editorConfig.autoTagConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, autoTagConfig: { ...current.autoTagConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.advanced.autoTag.label')}</label><label className="wk-kbl-4">{t('knowledgeEditor.advanced.tableMetadataInstructions.label')}<TTextarea rows={3} maxlength={4000} value={editorConfig.chunkingConfig.tableMetadataInstructions} placeholder={t('knowledgeEditor.advanced.tableMetadataInstructions.placeholder')} onChange={(value) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, tableMetadataInstructions: String(value) } }))} /><span className="kb-editor-desc-count wk-kbl-13 wk-kbl-count-end" aria-live="polite" data-table-metadata-count="">{editorConfig.chunkingConfig.tableMetadataInstructions.length}/4000</span></label></div>
+ import { clampQuestionCount } from '../knowledge-settings/editorSections.ts';
+ // ...
+ <TInput value={String(editorConfig.questionGenerationConfig.questionCount)} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: clampQuestionCount(Number(String(value))) } }))} />


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:54-55 ───
[maintainability · low] 同一模块的两条重复别名导入：文件顶部已从 'tdesign-react' 具名导入 Button/Checkbox/Input/Textarea
等，此处又以 T 前缀别名再次导入 Checkbox/Input/Select/Textarea，两条 import 语句指向同一包且类名体系并存（Checkbox vs
TCheckbox），阅读时容易误以为是两套组件库。建议合并为一条 import，将别名统一（或全部去别名）。

- // S6 抽屉收编：kb 编辑器深设置留守段离开 packages/ui 旧栈（T15 硬前置），换 tdesign。
- import { Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from 'tdesign-react';
+ // 并入顶部 import：
+ import {
+   Button, Checkbox, Dialog, Input, Loading, MessagePlugin, Popup, Radio, RadioGroup,
+   Select, Skeleton, Textarea, Tooltip,
+ } from 'tdesign-react';
+ // 文件内统一使用无别名类名


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:217-217 ───
[maintainability · low] SpaceAvatar 与 organizations/SpaceAvatar.tsx 形成双实现并存：两处包含完全相同的渐变表、hash 算法与装饰
SVG（约 80 行重复），但 props 分叉——页面版不支持 `avatar`（emoji/图片）属性，organizations 版支持。同屏可见两套头像（左侧栏用页面版，编辑器 share
段的组织选择器用 organizations 版），后续改渐变或尺寸需多点同步。页面版虽 export 但无外部消费方。建议抽取共享的 gradient/hash 工具函数，或让页面版扩展支持
avatar prop，为 Phase 4 归并留好合并点。



─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:102-102 ───
[bug · medium] 自绘抽屉丢失键盘可访问性与滚动锁：与迁移前的 Sheet 实现相比，该 portal 抽屉仅支持点击遮罩/按钮关闭，没有 Escape 关闭、焦点圈闭，也没有打开期间锁定
body 滚动（背后列表仍可滚动）。本次迁移中同域的详情抽屉（KnowledgeBaseActivityPanel）已改用 tdesign TDrawer 并获得这些默认能力，此处建议至少补
Escape 处理与 overflow 锁。

-     <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
+ // 组件内补（visible 变化时挂载）：
+ useEffect(() => {
+   if (!visible) return;
+   const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
+   window.addEventListener('keydown', onKey);
+   const prevOverflow = document.body.style.overflow;
+   document.body.style.overflow = 'hidden';
+   return () => {
+     window.removeEventListener('keydown', onKey);
+     document.body.style.overflow = prevOverflow;
+   };
+ }, [visible, onClose]);


─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 ───
[maintainability · low] 死导出：`permissionTone` 兼容别名在全仓库（含测试）已无任何消费方——组件自身改用
permissionTheme，旧调用方已随迁移清理。保留它会暗示存在外部依赖，建议删除该导出及注释。

- /** 兼容旧名（颜色 tone 语义并入 t-tag theme；default→neutral）。 */
- export const permissionTone = (permission: unknown): 'primary' | 'warning' | 'neutral' => {
-   const theme = permissionTheme(permission);
-   return theme === 'default' ? 'neutral' : theme;
- };
+ // 删除整个 permissionTone 导出块，仅保留 permissionTheme。


─── apps/web/src/knowledge-bases/kb-list.td.css:2426-2427 ───
[maintainability · medium] 死样式（约 200 行）：尾部三段「S6 收编」规则
`.wk-kb-activity-*`、`.wk-kb-share-*`、`.wk-kb-des-*` 在仓库内没有任何 TSX 生产者——组件实际使用的是 kb-u.css 定义的
`wk-kba-*`/`wk-kbs-*`/`wk-kbl-*` 前缀类（前缀已改）。其中 `.kb-editor-desc-count` 虽仍被使用，但与 kb-editor-parity.css
中的同名规则及页面上的 wk-kbl-13 类三重重复定义，存在层叠漂移风险。建议整段删除死前缀规则，`.kb-editor-desc-count` 收敛到单一来源。



─── apps/web/src/knowledge-bases/kb-list.td.css:35-38 ───
[maintainability · low] 无引用的动画与过渡类：`@keyframes dropdownSlideInUp` 定义后无任何规则引用它（消费方仅
dropdownSlideIn）；`.shared-detail-drawer-enter-*`/`-leave-*` 过渡类对应 Vue <Transition name=...>，而新版
SharedKnowledgeBaseDrawer 经 createPortal 直接挂载/卸载，永远不会携带这些类，属平移残留死代码，建议一并删除。



─── apps/web/src/knowledge-bases/kb-editor-parity.css:21-24 ───
[maintainability · low] 留守规则无 DOM 生产者：`label.grid` 选择器在 S6
收编后已无生产者——vectorStore/parser/storage/models 等留守段的字段容器已改用 `wk-kbl-4`（grid，gap:4px）、label 改用
`wk-kbl-5`，全仓库不存在 `className` 含 grid 的 label 元素。且文件头注释声称该规则服务于这些留守段，与实际 DOM 不符，会误导后续维护。同时注意 kb-u.css
的 wk-kbl-4 gap 为 4px，与本规则想表达的 8px（Vue form-label mb8 语义）不一致，属收编时的值漂移，建议删掉死规则或修正注释并核对间距。



─── apps/web/src/knowledge-bases/KnowledgeBaseActivityPanel.tsx:104-105 ───
[bug · low] 筛选下拉缺少关闭交互：action/outcome 两个自定义下拉菜单只能通过再次点击触发按钮或选中某项关闭，点击页面其他区域（表头、表体、面板外）与按 Escape
均不会关闭，且两个菜单可同时打开。表格行点击会打开详情抽屉，而下拉浮层仍悬浮在原位，交互观感卡死。建议为打开状态补 outside-click 与 Escape
关闭（例如在根节点监听，或在打开任一菜单时挂 window click/keydown 一次性监听）。

-   const filterOptions = (values: string[], selectedValue: string, onSelect: (value: string) => void, open: boolean, setOpen: (value: boolean) => void, label: string) => <div className="wk-kba-1">
-     <button type="button" className={`wk-kba-29 ${selectedValue ? 'wk-kba-30' : 'wk-kba-31'}`} aria-label={label} aria-expanded={open} onClick={() => setOpen(!open)}>⌄</button>
+ // 组件内补（actionMenuOpen/outcomeMenuOpen 任一打开时生效）：
+ useEffect(() => {
+   if (!actionMenuOpen && !outcomeMenuOpen) return;
+   const onKey = (event: KeyboardEvent) => {
+     if (event.key === 'Escape') { setActionMenuOpen(false); setOutcomeMenuOpen(false); }
+   };
+   window.addEventListener('keydown', onKey);
+   return () => window.removeEventListener('keydown', onKey);
+ }, [actionMenuOpen, outcomeMenuOpen]);


─── apps/web/src/knowledge/permissions.ts:96-101 ───
[security · high] canManage 门控跨面不一致（high）：新门控只被图谱页消费（KnowledgeGraphPage.tsx:90），其余两个宿主面仍以写权限信号放行 KB
管理入口与 admin 角色——KnowledgeDocumentsPage.tsx:3159 用 canUploadKnowledgeDocuments 设
canContribute，:4234/:4371/:5272 将其作为 canManage 传给 DocumentsPageChrome（齿轮入口门控），:5326 以
`role={canContribute ? "admin" : "viewer"}` 渲染 KnowledgeSettingsPage；WikiPage.tsx:480/1468 同样模式。共享
editor 在文档列表/Wiki 面仍能以 admin 角色打开设置页执行删除等破坏性操作，违反本注释自己引用的 OCR R1-17 红线（editor
写权限不得放宽破坏性门控），且与图谱面行为分叉。建议这两处消费点同步改用 computeKBPermissions(kb, me).canManage。

    const canManage = !!me && (
      isSystemAdmin(me)
      || isCreator(kb, me)
      || sharePermission === 'owner'
      || sharePermission === 'admin'
    );
+   // TODO(对齐): KnowledgeDocumentsPage.tsx:3159 与 WikiPage.tsx:480 仍以
+   // canUploadKnowledgeDocuments 门控设置入口/role，需同步切换到
+   // computeKBPermissions(kb, me).canManage，否则共享 editor 仍可从
+   // 这两个面以 admin 角色进入 KB 设置。


─── apps/web/src/knowledge/permissions.ts:26-29 ───
[documentation · low] 注释与实现不符（low）：注释称"admits only the owner or an explicit KB-level admin"，但实现还接纳
isSystemAdmin（含租户 role 'admin'、is_superuser、membership system_admin）与 isCreator（creator_id 及
user_id/created_by 兜底）共四条放行路径。行为本身有测试锚定（permissions.test.ts:47-50）且满足"editor
不放行"红线，但安全门控上的注释少列两条路径，后续维护者可能按注释口径收窄/误判门控范围。建议注释完整枚举四条放行路径。

-   /** Destructive-management gate. Vue canManageKB (frontend/src/stores/
-    *  organization.ts:818-823) admits only the owner or an explicit KB-level
-    *  admin — NOT every contributor (OCR R1-17: editor/成员写权限不得放宽
-    *  破坏性操作门控). */
+   /** Destructive-management gate (OCR R1-17: editor/成员写权限不得放宽
+    *  破坏性操作门控)。放行路径共四条：系统管理员（isSystemAdmin）、
+    *  KB 创建者（isCreator，对齐 Vue isOwner）、显式 owner、显式 KB 级
+    *  admin；editor/viewer 及普通成员一律不放行。 */


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:736-739 ───
[bug · medium] 自绘复选框缺少键盘焦点态（medium）：原生 input 已被视觉隐藏（1px clip，仍可聚焦），可见面孔由 .kb-checkbox-input span
承担，但规则只覆盖 :checked 与 :disabled 两态；整个 knowledge-settings 域 CSS 中不存在 input:focus-visible +
.kb-checkbox-input 规则。键盘用户 Tab 到"语义检索/知识库索引"开关时焦点落在不可见元素上、可见盒无任何高亮，属可达性回归（tdesign 原生
.t-checkbox__former:focus-visible 有 focus ring，复刻时遗漏）。建议补焦点环规则。

  .indexing-check-head input:checked + .kb-checkbox-input {
    border-color: var(--wk-color-brand, #07c05f);
    background-color: var(--wk-color-brand, #07c05f);
+ }
+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   box-shadow: 0 0 0 2px var(--wk-color-surface, #fff), 0 0 0 4px var(--wk-color-brand, #07c05f);
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[maintainability · low] 死声明（low）：`border-color: none` 不是合法值（border-color 不接受 none），会被浏览器丢弃。该元素本身
border: 0、焦点边框由外层 .kb-text-input-wrap:focus-within 承担，此行无实际效果，仅误导维护者以为内层 input 有焦点描边，建议删除。

- .kb-text-input:focus { outline: none; border-color: none; }
+ .kb-text-input:focus { outline: none; }


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · medium] aria-label 丢失可达名称（medium）：aria-label 挂在不带 role 的包裹 span 上，按 ARIA 规范 generic role
不参与可访问名称计算，等于无效；台账 #8 只说明 Select 根不透传 data-*，但副作用是 tdesign Select 内部 input 现在没有任何可达名称——旧原生 select 的
aria-label={group.label} 被丢掉后，屏幕阅读器无法辨识各解析器分组下拉（PDF/Word/…）各自控制什么。建议改用真实 label 关联（如以 <label> 包裹
Select 并附视觉隐藏文本），或确认 tdesign Select 的属性透传后将 aria-label 直接传给 Select。

-               <span
+               <label
                  data-parser-group={group.key}
-                 aria-label={group.label}
                  style={{ display: 'block' }}
                >
+                 <span className="wb-sr-only">{group.label}</span>
+                 {/* ... Select 保持不变，label 包裹使内部 input 获得可达名称 ... */}


─── apps/web/src/knowledge-settings/GraphSettings.tsx:131-131 ───
[bug · low] 关系类型输入吞逗号（low，随换装保留的既有缺陷）：value 为 local.tags.join(', ') 且 onChange 每次按键即
split/trim/filter(Boolean)
并回写受控值——用户键入尾随逗号后立即被重渲染剥离，分隔符永远无法保留，多个关系类型无法通过键盘逐字录入（只能靠"生成随机标签"按钮或整段粘贴）。建议以原始文本态持有输入、blur 时再归一化为
tags 数组。

-         <div className="wk-graph-field"><strong>关系类型</strong><span className="wk-muted">添加或生成提取使用的关系类型。</span><div className="wk-graph-inline"><TInput aria-label="关系类型" value={local.tags.join(', ')} onChange={(value) => updateField('tags', String(value).split(',').map((tag) => tag.trim()).filter(Boolean))} />{shouldRenderGraphActions(canRunGraphExtract) ? <TButton type="button" loading={actions.tags.status === 'loading'} disabled={!modelId} onClick={() => void action('tags')}>生成随机标签</TButton> : null}</div><ActionStatus state={actions.tags} /></div>
+ <TInput aria-label="关系类型" value={tagsText} onChange={(value) => setTagsText(String(value))} onBlur={() => updateField('tags', tagsText.split(',').map((tag) => tag.trim()).filter(Boolean))} />


─── apps/web/src/knowledge/knowledge-u.css:327-331 ───
[maintainability · low] 死声明（low）：连写两条 transition-duration，150ms 永不生效（被 300ms 覆盖）。原 utility 是
transition-all duration-300，codemod 把默认 150ms 一并带出。保留 300ms 即可，死声明易误导维护者以为有 150ms 过渡。

    transition-property: all;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 300ms;
  }


─── apps/web/src/knowledge/knowledge-u.css:251-253 ───
[maintainability · low] 选择器误译为死规则（low）：`.wk-kg-24 :-webkit-details-marker` 是后代选择器，而
::-webkit-details-marker 伪元素挂在 summary 自身上（原 Tailwind 为 `[&::-webkit-details-marker]:hidden`，即
`.xxx::-webkit-details-marker`），后代形式永不匹配。目前因 .wk-kg-24 的 display:inline-flex 在 Blink/WebKit
下本就抑制了三角标记而无视觉影响，但这条规则是死的；修正为直接附着形式（或删除并注明依赖 display 抑制）。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/knowledge/knowledge-u.css:699-700 ───
[maintainability · low] 跨域错放（low）：.wk-ds-self-start 前缀属 data-sources 域，且全仓唯一使用点是
DataSourcesPage.tsx:441，却被定义在 knowledge 域的 knowledge-u.css 手工段里。当前因所有 CSS 合入同一全局样式表而侥幸生效；一旦
knowledge 域样式被拆分/按需加载，数据源页面将静默丢失 justify-self。建议移入 data-sources/data-sources-u.css。

+ /* .wk-ds-self-start 移至 apps/web/src/data-sources/data-sources-u.css */
  .wk-graph-arrow-stroke { stroke: #c0c4cc; }
- .wk-ds-self-start { justify-self: start; }


─── apps/web/src/administration/AdministrationPage.tsx:155-155 ───
[bug · medium] 邀请邮箱输入框从原 `<Input type="email" required>` 迁移为 TInput 后丢失了 `type="email"` 与
`required`：空值虽仍被 `onInviteFormSubmit` 的 `!email.trim()` 拦截，但客户端邮箱格式校验完全丢失，非法格式（如
"abc"）会直接进入确认步骤并触发一次注定失败的 API 调用，只能靠服务端报错回显；移动端也无法再唤起邮箱键盘。建议补回 `type="email"`（若 TDesign Input 的 type
联合类型不接受 'email'，至少在 onInviteFormSubmit 中增加格式校验并给出错误提示，避免静默 return）。

-   return <main className="wk-page wk-admin-1"><header className="wk-header wk-admin-2"><div><p className="wk-eyebrow wk-admin-3">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="wk-admin-4">{systemAdmin ? t('settings.navGroups.systemAdministration') : t('mobileAdministration.title')}</h1><p className="wk-muted wk-admin-5">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<div className="wk-admin-12"><Card><h2 className="wk-admin-6">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="wk-admin-7" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list wk-admin-8">{members.map((member) => <li key={member.user_id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{member.username}</strong><span className="wk-admin-11">{member.email} · {roleText(member.role)} · {member.status}</span></div><div className="wk-list-actions wk-admin-13"><TSelect value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(value) => void updateRole(member, String(value) as TenantMember['role'])} className="wk-admin-14" options={[{ value: 'owner', label: roleText('owner') }, { value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /><TButton type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>{t('mobileAdministration.remove')}</TButton></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card><Card><h2 className="wk-admin-6">{inviteStep === 'confirm' ? t('mobileAdministration.confirmInviteTitle') : t('mobileAdministration.invite')}</h2>{inviteStep === 'confirm' ? <div className="wk-admin-15"><p className="wk-muted wk-admin-16">{t('mobileAdministration.confirmInviteBody', { email: email.trim(), role: roleText(role) })}</p><div className="wk-admin-17"><TButton type="button" disabled={saving} onClick={() => setInviteStep('form')}>{t('mobileAdministration.back')}</TButton><TButton type="button" loading={saving} onClick={() => void sendInvitation()}>{t('mobileAdministration.confirmSend')}</TButton></div></div> : <form className="wk-admin-15" onSubmit={onInviteFormSubmit}><label className="wk-admin-18">{t('mobileAdministration.inviteEmail')}<TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} /></label><label className="wk-admin-18">{t('mobileAdministration.role', { role: '' }).replace(/: $/, '')}<TSelect value={role} onChange={(value) => setRole(String(value) as typeof role)} options={[{ value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /></label><TButton type="submit" loading={saving}>{t('mobileAdministration.sendInvitation')}</TButton></form>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.openInvitations')}</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>{t('mobileAdministration.noPendingInvitations')}</Status> : <ul className="wk-list wk-admin-8">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span className="wk-admin-11">{roleText(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</span></div><TButton type="button" onClick={() => void revokeInvitation(item)}>{t('mobileAdministration.revoke')}</TButton></li>)}</ul>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.auditLog')}</h2>{audit.length === 0 ? <Status>{t('mobileAdministration.noAuditEntries')}</Status> : <ul className="wk-list wk-admin-8">{audit.map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.action}</strong><span className="wk-admin-11">{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2 className="wk-admin-6">{t('settings.navGroups.systemAdministration')}</h2><ul className="wk-list wk-admin-8">{admins.map((item, index) => <li key={String(item.id ?? index)} className="wk-admin-9"><strong>{String(item.username ?? item.email ?? item.id)}</strong><span className="wk-admin-11">{item.is_active === false ? t('common.disabled') : t('mobileAdministration.status.active')}</span></li>)}</ul></Card><div data-testid="system-administration-panels" className="wk-admin-19">{systemAdminPanelKeys(systemAdmin).map((panel) => panel === 'system-global' ? <SystemGlobalSettingsPanel key={panel} client={client} initialSettings={settings} /> : panel === 'runtime-queues' ? <RuntimeQueuesPanel key={panel} client={client} payload={queues} loading={loading} /> : panel === 'platform-api-keys' ? <PlatformApiKeysPanel key={panel} client={client} initialKeys={apiKeys} /> : <SystemAuditLogPanel key={panel} client={client} payload={systemAudit} />)}</div></> : null}</div></main>;
+ <TInput className="wk-admin-invite-email" type="email" value={email} onChange={(value) => setEmail(String(value))} />


─── apps/web/src/administration/AdministrationPage.tsx:206-209 ───
[bug · low] 跳页输入框迁移到 TInput 时丢失了原有的 `inputMode="numeric"`，移动端不再弹出数字键盘；`onKeydown={(_, context) =>
context.e.key}` 签名与仓库其他 TDesign Input 用法一致且有测试覆盖，无问题，仅需补回 inputMode。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}
          onChange={(value) => setJump(String(value))}
          onBlur={commitJump}
          onKeydown={(_, context) => { if (context.e.key === 'Enter') commitJump(); }} />


─── apps/web/src/administration/AdministrationPage.tsx:153-153 ───
[style · low] 重写后的单行 JSX 保留并延续了多层链式三元表达式（本行的 `loading ? … : members.length === 0 ? … : …`，以及主 return
中的 `inviteStep === 'confirm' ? … : …` 与 `panel === 'system-global' ? … : panel === 'runtime-queues'
? …` 链），违反检查清单"禁止嵌套三元表达式"规则，且数千字符的单行使样式迁移类改动极易引入复制粘贴回归。建议至少将成员列表渲染与 systemAdmin 面板映射提取为局部 render 函数或
`Record<panelKey, () => JSX>` 映射表，压平三元链。

-   if (!manageTenant) return <main className="wk-page wk-admin-1"><header className="wk-header wk-admin-2"><div><p className="wk-eyebrow wk-admin-3">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="wk-admin-4">{t('mobileAdministration.title')}</h1><p className="wk-muted wk-admin-5">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<Card><h2 className="wk-admin-6">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="wk-admin-7" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list wk-admin-8">{members.map((member) => <li key={member.user_id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{member.username}</strong><span className="wk-admin-11">{member.email} · {roleText(member.role)} · {member.status}</span></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card></main>;
+ {loading ? <Status>{t('mobileAdministration.loading')}</Status> : renderMemberList()}


─── apps/web/src/administration/administration-u.css:52-59 ───
[maintainability · medium] 语义色被硬编码为字面量而非引用已有令牌：`--color-primary: #2e6de6`、`--color-accent: #07c05f`
均已在 styles.css :root 定义，本文件 wk-admin-3 却写死 #2e6de6，pager 系列写死 #07c05f、rgba(0,0,0,.6)、rgb(0 0
0/26%)、#e7e7e7；且同文件末尾"手工段"又改用 `var(--color-accent, #07c05f)`，同一文件两套取色方式并存。仓库其他域（如
integrations.td.css）muted 统一走 `var(--color-muted, #66758b)`，与本处 rgba(0,0,0,.6)
取值不一致，品牌色/令牌调整时本页将无法跟随。建议统一改用带 fallback 的 var() 引用。

  .wk-admin-3 {
    margin: 0;
    font-size: 0.78rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
-   color: #2e6de6;
+   color: var(--color-primary, #2e6de6);
  }


─── apps/web/src/administration/administration-u.css:18-23 ───
[maintainability · low] .wk-admin-pager-btn 中 `font-weight: 400` 与随后的 `font-weight: inherit`
重复声明，后者在层叠中胜出，前者成为死代码，按钮字重实际完全取决于父级继承值，与原 Tailwind `font-normal`（400）的语义不再对应（原类名同时携带 font-normal 与
[font:inherit] 的冲突被原样转写）。应二选一：若意图是继承父级字体则删除首条 400 声明；若意图是固定 400 则删除 inherit。

-   font-weight: 400;
    color: rgb(0 0 0/90%);
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
    font-weight: inherit;


─── apps/web/src/platform/PlatformShell.tsx:218-218 ───
[bug · medium] 【round-4 新发现 · bug·medium】career 导航条目用精确匹配 `p === '/platform/career'`，遗漏了 career
域两条子路由 `/platform/career/opportunities/$opportunityId` 与
`/platform/career/evaluations/$evaluationId`（router.tsx:748-765）。求职闭环的核心页面（ApplicationPage/MaterialP
age/PreparationPage/ProgressPage/SubmissionPage/reconciliation 均渲染在 opportunities/:id 路由下，由
SearchPage/InboxPage 下钻进入）访问时，侧栏无任何导航项高亮，与 knowledgeBases（KB_ACTIVE 前缀匹配）、agents、organizations
等条目的前缀匹配惯例不一致，用户在求职主工作流中失去导航位置指示。建议 career 条目补充子路由前缀匹配（保持与 careerSearch/careerRules 精确条目互斥）。

- { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },
+ { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' || p.startsWith('/platform/career/opportunities') || p.startsWith('/platform/career/evaluations') },


─── apps/web/src/agent-marketplace/am-u.css:216-222 ───
[maintainability · medium] 将语义令牌 bg-accent 固化为字面色值 #07c05f(与 .wk-amr-18 第 218 行同样处理)。styles.css:457
定义了 --color-accent: #07c05f 作为 T15 令牌事实源,原 Tailwind utility 编译结果是 var(--color-accent) 引用而非字面值;同批迁移的
administration-u.css:220-221 采用的是 var(--color-accent, #07c05f) 口径。一旦令牌随主题(如 --color-brand 已链到含暗色的
--wk-color-*)调整,发布/评审主按钮底色将脱离主题且两处口径不一致。建议改为变量引用带回退。

  .wk-ava-11 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);
    padding-inline: 12px;
    padding-block: 6px;
    color: #fff;
  }


─── apps/web/src/agent-marketplace/am-u.css:44-53 ───
[maintainability · medium] bg-surface 令牌被固化为字面值 #ffffff。styles.css:424 定义 --color-surface: #ffffff
作为卡片/面板语义令牌,原 utility 编译结果是 var(--color-surface) 引用;同类迁移文件多采用 var(--color-surface, #ffffff)
口径。评审卡片背景若后续随令牌主题化将无法跟随,建议与 accent 一并改为变量引用。

  .wk-amr-7 {
    display: grid;
    gap: 12px;
    border-radius: 8px;
    border-style: solid;
    border-width: 1px;
    border-color: #e7e7ea;
-   background-color: #ffffff;
+   background-color: var(--color-surface, #ffffff);
    padding: 16px;
  }


─── apps/web/src/agent-marketplace/am-u.css:97-99 ───
[maintainability · low] 原 Tailwind 类 break-all 编译为 word-break: break-all,此处改写为 overflow-wrap:
anywhere,两者断行语义与 min-content 尺寸计算并不等价;且项目其他迁移文件(如
settings.td.css:1812-1829、kb-list.td.css:2570、faq.td.css:1540、export-deletion.css:73)对 break-all
统一编码为 word-break: break-all。超长 bundle digest 在窄容器下的换行/溢出行为可能与迁移前不一致,建议对齐全仓口径。

  .wk-amr-14 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    border-radius: 4px;


─── apps/web/src/agent-marketplace/am-u.css:117-124 ───
[maintainability · low] 重复声明:.wk-amr-17 与 .wk-ava-2 逐字相同;.wk-amr-4 ≡ .wk-ava-12、.wk-amr-12 ≡
.wk-ava-13、.wk-amr-3 ≡ .wk-amr-5、.wk-amr-18 ≈ .wk-ava-11
亦然。同一套样式存在多份拷贝,后续调整控件边框/文字规格时易漏改造成两组件视觉漂移。建议跨前缀复用同一基础类(如共享 .wk-am-input / .wk-am-text
类),或至少在注释中标注等价组以便同步修改。



─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1749-1749 ───
[maintainability · low] 嵌套三元表达式违反评审规则且此处最易出错：在模板字符串内用两层三元拼接 i18n 键（granularity…Hint），且
`editorConfig.wikiConfig?.extractionGranularity ?? 'standard'`
子表达式重复求值两次。建议先取出变量再查表。同类嵌套三元还有两处：ResourceOriginBadge 的 `variant === 'mine' ? … : variant ===
'creator' ? … : …` 与 KbCard 头部的 `isSpaceCard ? … : isSharedCard ? … : …`，建议一并改为提前求值或映射表。

- <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${(editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'focused' ? 'Focused' : (editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'exhaustive' ? 'Exhaustive' : 'Standard'}Hint`)}</p>
+ const GRANULARITY_HINT_SUFFIX = { focused: 'Focused', standard: 'Standard', exhaustive: 'Exhaustive' } as const;
+ // ...
+ const granularity = editorConfig.wikiConfig?.extractionGranularity ?? 'standard';
+ <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${GRANULARITY_HINT_SUFFIX[granularity]}Hint`)}</p>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1849-1849 ───
[maintainability · low] storage 留守段存在三处静态内联 style（本行 color、上方 migrate-hint 的 `style={{ color:
'var(--color-warning-text, #b54708)' }}`、models 段 embedding-locked-tip 的 `style={{ margin: '0.5rem 0
0', fontSize: '0.85rem' }}`）。本轮 S6/S7 迁移约定是 Tailwind→kb-u.css 工具类（wk-kbl-*），内联静态样式游离于该体系外，主题 token
更新时不会被覆盖。建议在 kb-u.css 补 wk-kbl-* 类替换（颜色走 var(--td-warning-color) 等 token）。

- return <div className="wk-kbl-1"><div><h3 className="wk-kbl-2">{t('knowledgeEditor.sidebar.storage')}</h3><p className="wk-kbl-3">{t('kbSettings.storage.selectDescription')}</p></div><div className="wk-kbl-4"><label className="wk-kbl-5">{t('kbSettings.storage.instanceLabel')}</label><p className="wk-kbl-9">{t('kbSettings.storage.instanceDesc')}</p><TSelect value={editorConfig.storageBackendId} disabled={storageLocked || editorOptions.loading} aria-label={t('kbSettings.storage.instanceLabel')} options={[...editorOptions.storageBackends.map((backend) => ({ value: backend.id, label: `${backend.name} · ${backend.provider.toUpperCase()}${backend.id === editorOptions.defaultStorageBackendId ? ` · ${t('kbSettings.storage.defaultTag')}` : ''}` })), ...(editorOptions.storageBackends.every((backend) => backend.id !== editorConfig.storageBackendId) && editorConfig.storageBackendId ? [{ value: editorConfig.storageBackendId, label: editorConfig.storageBackendId }] : [])]} onChange={(value) => { const backendId = String(value); const backend = editorOptions.storageBackends.find((candidate) => candidate.id === backendId); setEditorConfig((current) => ({ ...current, storageBackendId: backendId, storageProvider: backend?.provider ?? current.storageProvider })); }} />{storageLocked ? <p className="wk-kbl-10" style={{ color: 'var(--color-warning-text, #b54708)' }} data-storage-migrate-hint="">{t('kbSettings.storage.migrateHint')}</p> : null}{!storageLocked && selectedHint ? <p className="wk-kbl-9" data-storage-instance-hint="">{selectedHint}</p> : null}<a data-storage-manage-instances="" href="/platform/settings?section=storage" className="wk-kbl-11 wk-kbl-link-start" style={{ color: 'var(--color-brand)' }} onClick={(event) => { event.preventDefault(); setDialogOpen(false); navigate('/platform/settings?section=storage'); }}>{t('kbSettings.storage.manageInstances')}</a></div></div>;
+ <a data-storage-manage-instances="" href="/platform/settings?section=storage" className="wk-kbl-11 wk-kbl-link-start wk-kbl-link-brand" onClick={/* ... */}>{t('kbSettings.storage.manageInstances')}</a>
+ /* kb-u.css: .wk-kbl-link-brand { color: var(--color-brand, #07c05f); } */


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1614-1617 ───
[bug · low] 删除确认弹窗的「取消/删除」动作落在 `<span onClick>` 上：不可 Tab 聚焦、无 button 语义，键盘与读屏用户只能依赖 TDialog 默认 Esc
取消，无法用键盘确认删除（Vue 原版同样如此，但迁移清单要求补齐）。建议改为 `<button type="button">` 并在 kb-list.td.css 的 .circle-btn-txt
上重置按钮默认样式（background/border/padding/ font:inherit），视觉不变。

  <div className="circle-btn">
-             <span className="circle-btn-txt" onClick={() => { if (!deleting) setDeletingKb(null); }}>{t('common.cancel')}</span>
-             <span className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</span>
+             <button type="button" className="circle-btn-txt" onClick={() => { if (!deleting) setDeletingKb(null); }}>{t('common.cancel')}</button>
+             <button type="button" className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</button>
            </div>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:178-181 ───
[maintainability · low] OrgGreenIcon（organization-green.svg 内联副本）与本域 SharedKnowledgeBaseDrawer.tsx
中的同名组件完全重复（同一路径、同一 20×20 viewBox）；同屏可见两份（列表卡片 org-source 与共享详情抽屉）。与 finding 6 的 SpaceAvatar
双实现同属归并债务，建议抽到 kb-list-icons.tsx（本域图标模块）统一导出，Phase 4 归并时一处清理。

- /* frontend/src/assets/img/organization-green.svg —— 共享来源空间徽标（20×20）。 */
- function OrgGreenIcon() {
-   return (
-     <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
+ // kb-list-icons.tsx
+ export function OrgGreenIcon({ size = 20, className }: { size?: number; className?: string }) {
+   return <svg className={className} width={size} height={size} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">{/* path 不变 */}</svg>;
+ }
+ // KnowledgeBasesPage.tsx / SharedKnowledgeBaseDrawer.tsx 均改为 import { OrgGreenIcon } from './kb-list-icons.tsx';


─── apps/web/src/knowledge-bases/kb-list.td.css:1840-1851 ───
[maintainability · low] 死样式：`.create-kb-dialog` 选择器在 apps/web/src 无任何生产者（React 端创建对话框为自绘
.wk-kb-settings-overlay，dialogClassName 仅 del-knowledge-dialog）。属 Vue unscoped 块的平移残留，与 findings
10/11 同批，建议随本轮 CSS 清理一并删除。



─── apps/web/src/analytics/analytics-u.css:241-247 ───
[style · low] 格式瑕疵：`border-bottom-width: 1px;border-color` 中 `1px;` 与 `border-color`
之间缺少空格（`.wk-anl-2` 规则中同样存在 `border-bottom-width: 1px;border-color:
#e7e7ea;`）。不影响解析，但破坏了本文件其余规则的排版一致性，建议统一补上空格。

  .wk-anl-an-usage-cell {
    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom-width: 1px;
+   border-color: #eef1f5;
    padding-inline: 10px;
    padding-block: 8px;
    text-align: left;
  }


─── apps/web/src/analytics/analytics-u.css:311-313 ───
[maintainability · low] `.wk-anl-an-usage-num` 在文件中部（text-align: right）与文末此处被定义了两次，`tabular-nums`
与对齐方式分离，后续调整该类样式时容易只改其中一处。建议将 `font-variant-numeric: tabular-nums;` 合并进中部的主规则，保持一个选择器一处定义。

- /* T15 语义化：原 placeholder:text-[rgba(23,26,29,0.35)] / tabular-nums 旧栈 utility。 */
+ /* 中部主规则合并： */
+ .wk-anl-an-usage-num {
+   text-align: right;
+   font-variant-numeric: tabular-nums;
+ }
+ 
+ /* 文末仅保留： */
+ /* T15 语义化：原 placeholder:text-[rgba(23,26,29,0.35)] 旧栈 utility。 */
  .wk-anl-an-text-input::placeholder { color: rgba(23, 26, 29, 0.35); }
- .wk-anl-an-usage-num { font-variant-numeric: tabular-nums; }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:753-758 ───
[maintainability] 孤儿死规则（low）：本次将原生 checkbox 视觉隐藏（1px clip、accent-color 从基础规则移除）后，文件末尾既有的
`.indexing-checks.is-locked input[type='checkbox'] { accent-color: #dcdcdc; }`（约 :887-889，注释 "Locked
(disabled+checked) indexing checkboxes render grey ... not brand green"）不再产生任何视觉效果——accent-color
只作用于原生勾选渲染，而可见面孔现在由 .kb-checkbox-input 承担，锁定态灰色已由上方 input:disabled / input:disabled:checked
系列新规则完整产出。建议随本次换装一并删除该孤儿规则及其注释，避免后续维护者误以为锁定态仍走 accent-color 路径。

- .indexing-check-head input:disabled + .kb-checkbox-input {
-   /* tdesign：.t-is-disabled .t-checkbox__input 一律 #eee 底（勾/未勾皆是），
-    未勾选（无论禁用与否）无对勾。 */
-   border-color: #dcdcdc;
-   background-color: #eee;
- }
+ /* 建议同时删除文件末尾的：
+ .indexing-checks.is-locked input[type='checkbox'] { accent-color: #dcdcdc; }
+ （原生 input 已视觉隐藏，accent-color 不再参与渲染） */


─── apps/web/src/knowledge-settings/parserSettings.tsx:10-11 ───
[maintainability] 死导入（low）：本文件未使用任何 wk-kss-* 类——knowledge-settings-u.css 是
KBShareSettingsSection.tsx 的 Tailwind 平移样式表，且其头注声明的"导入顺序：须在本域 .td.css 之前"契约与本文件无关（本域无
.td.css，KBShareSettingsSection.tsx 已自行按序导入）。此处新增的 side-effect 导入建立了无关组件间的假依赖，建议移除。

  import { Select } from 'tdesign-react';
- import './knowledge-settings-u.css';


─── apps/web/src/analytics/analytics-u.css:114-118 ───
[maintainability · low] 设计令牌被硬编码为字面量：本文件的 accent/surface 系列值（#07c05f、rgba(7,192,95,0.08)、#ffffff 等，含
date-input/text-input 的 :focus 边框、btn-primary 背景）全部内联，而 styles.css 的 T15 块明确以 `--color-accent` /
`--color-accent-wash` / `--color-surface`
作为运行时事实源，同批迁移的兄弟文件（administration-u.css:220、data-sources-u.css:740、documents-list.css:203、kb-list.td
.css 等）也统一采用 `var(--color-*, 字面量回退)` 的引用形式。当前取值一致暂无视觉差异，但后续调整品牌色或主题令牌时本页将脱钩。建议改为带回退的 var() 引用，与既有
-u.css 模式保持一致。

  .wk-anl-an-btn-outline:hover {
    border-style: solid;
-   border-color: #07c05f;
-   background-color: rgba(7, 192, 95, 0.08);
+   border-color: var(--color-accent, #07c05f);
+   background-color: var(--color-accent-wash, rgba(7, 192, 95, 0.08));
  }


─── apps/web/src/apps/AppsPages.tsx:80-80 ───
[maintainability · medium] ConnectionsPage 已改用 tdesign-react 的 TPopconfirm 后，此本地 Popconfirm
组件在全文件再无任何调用点（搜索仅剩 TPopconfirm 使用），成为死代码；apps-u.css 中仅服务于它的 .wk-apps-1 ~ .wk-apps-4
四条规则也随之整体失效。建议连同对应 CSS 一并删除，避免后续读者误以为撤销确认链路仍走该自绘气泡。



─── apps/web/src/apps/AppsPages.tsx:161-163 ───
[bug · low] rowKey 由旧实现的 String(row.action_id ?? index) 索引兜底改为纯字段名：model.ts 的 appRows()
只保证行是对象（AppRow = Record<string, unknown>），并不保证 action_id/id 存在。一旦后端返回缺字段的行，TTable 将得到多个 undefined
key（React 重复 key 告警、行 hover/操作态错乱）。建议改用函数式 rowKey 保留索引兜底，installations 表（rowKey="id"）同理。

          <TTable
-           rowKey="action_id"
-           data={catalog}
+           rowKey={(row, index) => String(row.action_id ?? index)}
+           data={catalog>


─── apps/web/src/apps/AppsPages.tsx:126-126 ───
[style · low] state 单元格存在三层嵌套三元（active/disabled/其它/—）；本轮 risk 列已拆出 riskThemeOf/riskLabelOf，此处建议对齐抽取
stateLabelOf 工具函数（ConnectionsPage 的 state tag theme 两层三元同理），满足'禁止嵌套三元'规范并保持文件内一致风格。

-     { colKey: 'state', title: t('apps.catalog.colState'), width: 110, cell: ({ row }: { row: AppRow }) => { const state = String(row.state ?? '').trim(); const label = state === 'active' ? t('apps.common.stateActive') : state === 'disabled' ? t('apps.common.stateDisabled') : state ? t('apps.common.stateOther', { state }) : '—'; return <TTag theme={state === 'active' ? 'success' : 'default'} size="small">{label}</TTag>; } },
+ function stateLabelOf(state: string, t: AppsTranslate): string {
+   if (state === 'active') return t('apps.common.stateActive');
+   if (state === 'disabled') return t('apps.common.stateDisabled');
+   return state ? t('apps.common.stateOther', { state }) : '—';
+ }


─── apps/web/src/apps/apps.td.css:102-104 ───
[maintainability · low] 本仓库（packages/design-tokens/src/tdesign-theme.css 全文及 TDesign 官方 token 集）均未定义
--td-font-family-code，该声明恒走 monospace 兜底：连接列表等宽短 ID 实际渲染为通用 monospace，而同批 apps-u.css 的
.wk-apps-12（authorization/action 页等宽值）用的是已接线的 --app-font-family-mono
栈（ui-monospace/Menlo/…），同域两套等宽字体不一致。建议改用 --app-font-family-mono 或补定义该 token。

  .connections-view .connections-view__mono {
-   font-family: var(--td-font-family-code, monospace);
+   font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }


─── apps/web/src/apps/AppsPages.tsx:171-171 ───
[maintainability · low] 520/360 为硬编码布局魔法数：上方注释只解释了空态不传 maxHeight 的原因，未给出这两个数值的出处（Vue
源/设计规范）。建议提为具名常量并注明来源，便于后续视觉对账与统一调整（installations 的 360 同理）。

-           maxHeight={catalog.length ? 520 : undefined}
+ /* Vue AppsView.vue maxHeight 出处：*/
+ const CATALOG_TABLE_MAX_HEIGHT = 520;
+ const INSTALLED_TABLE_MAX_HEIGHT = 360;
+ …
+           maxHeight={catalog.length ? CATALOG_TABLE_MAX_HEIGHT : undefined}


─── apps/web/src/apps/AppsPages.tsx:214-215 ───
[maintainability · low] ConnectionsPage 的 columns 数组在组件体内每次渲染重建，且与同文件本轮已抽到模块级的
catalogColumns/installationColumns 工厂风格不一致。建议同样抽为模块级 connectionsColumns(deps) 工厂（传入
t/canManage/revokingId 及回调动词），统一列定义的组织方式，也便于复用与测试。



─── apps/web/src/commercial/surface.tsx:6-7 ───
[maintainability · medium] 本域 5 个页面共 13 处 Button 调用点逐一重复硬编码 theme="default"
variant="outline"（surface.tsx 头注释也已声明这是本域统一约定）。建议在此文件封装域级按钮，内置默认
theme/variant，调用点只写差异属性；否则后续视觉规范调整（如某处改主按钮、全局换 variant）需逐文件逐调用点修改，属典型的可提取重复代码。

- import type { HTMLAttributes, ReactNode } from 'react';
+ import type { ButtonProps } from 'tdesign-react';
+ import { Button as TdButton } from 'tdesign-react';
  import './commercial-u.css';
+ 
+ /** 域级按钮：内置旧栈默认白底细描边（theme=default + variant=outline），调用点可按需覆盖。 */
+ export function Button({ theme = 'default', variant = 'outline', ...props }: ButtonProps) {
+   return <TdButton theme={theme} variant={variant} {...props} />;
+ }


─── apps/web/src/commercial/commercial-u.css:40-41 ───
[maintainability · low] 同一选择器 .wk-bill-usage-number-cell 在文件内被定义两次（第 19-21 行的 text-align 与此处
font-variant-numeric 分散两处），建议合并为一条规则。另有两处可读性问题：(1) wk-bill-1 / wk-bill-2 纯数字后缀类名无法表达"表头行底边框 /
表头半粗体"的意图，建议改为语义命名（如 wk-bill-thead-row、wk-bill-th），并同步更新 BillingPage.tsx 中的引用；(2)
border-bottom-width: 1px;border-color: #eef1f5 单行双声明建议拆为两行，与文件内其余规则排版保持一致。此外文件头注释约定"导入顺序：须在本域
.td.css 之前"，但本域并不存在 td.css（该注释是 commercial 目录下对 td.css 的唯一引用），约定悬空易误导后续维护者。

- /* T15 语义化：原 tabular-nums（用量数值单元格）。 */
- .wk-bill-usage-number-cell { font-variant-numeric: tabular-nums; }
+ /* T15 语义化：原 text-right + tabular-nums（用量数值单元格）。 */
+ .wk-bill-usage-number-cell {
+   text-align: right;
+   font-variant-numeric: tabular-nums;
+ }


─── apps/web/src/configuration/config-u.css:44-47 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea`（特异度 0-1-1）会命中表单内 tdesign 组件渲染的原生内部元素
`.t-input__inner` / `.t-textarea__inner`（tdesign 规则特异度仅 0-1-0），将边框、内边距、背景、字体强加到内层元素上，与 tdesign 外层
`.t-input`/`.t-textarea` 自身的边框和 padding 叠加，在 ModelDebugPanel 与 SkillOperations
表单中产生双边框、内容区被压缩等视觉错乱。旧代码的 `[&_input]`/`[&_textarea]` utility 当时命中的是 @weknora/ui 直渲染的原生元素，迁移后这两个表单内的
Input/Textarea/Select 已全部换成 tdesign，唯一需要该样式的只剩原生 `<input type="file">`。建议将 input 规则限定为
`input[type='file']`，textarea 规则可直接删除（表单内已无原生 textarea）。

- .wk-cfg-ops-4 input {
+ .wk-cfg-ops-4 input[type='file'] {
    width: 100%;
    box-sizing: border-box;
    border-style: solid;


─── apps/web/src/configuration/ui.tsx:12-15 ───
[maintainability · medium] 域内新建的 Card/Status 与本轮同 PR 新增的共享层 `apps/web/src/shared/wk-legacy.tsx` 的
WkCard/WkStatus 完全等价（同为 `<section>`/`<p>` + error→alert 其余→status 的 role
映射），NotFoundPage、JoinPage、DocumentsPage、GraphSettings 等域均复用共享层，此处属重复实现。且已出现值漂移：`.wk-cfg-card` 边框为
#e7e7e7，而 wk-legacy.css 中 `.wk-card`（注释注明逐项复制自原 packages/ui styles.css 的 border-line token）为
#dce3ed——两个"视觉零变化"移植结果不一致，与文件头"逐字一致、视觉零变化"的声明矛盾。建议直接 `import { WkCard as Card, WkStatus as Status }
from '../shared/wk-legacy.tsx'` 复用，删除本域重复封装与 config-u.css 中的 `.wk-cfg-card`/`.wk-cfg-status` 拷贝。

- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
-   const classes = ['wk-cfg-card', className].filter(Boolean).join(' ');
-   return <section className={classes} {...props}>{children}</section>;
- }
+ // 复用共享兼容层，避免与 .wk-card/.wk-status 重复并消除边框色漂移：
+ import { WkCard, WkStatus } from '../shared/wk-legacy.tsx';
+ export const Card = WkCard;
+ export const Status = WkStatus;


─── apps/web/src/configuration/config-u.css:356-360 ───
[maintainability · medium] 本文件存在大量逐字相同的拷贝块：heading 三份（wk-cfg-ops-6 / wk-cfg-ed-1 / wk-cfg-mun-1，各含同款
720px 媒体查询）、actions 三份（wk-cfg-page-4 / wk-cfg-ops-3 / wk-cfg-ed-6）、list reset 三份（wk-cfg-ops-9 /
wk-cfg-page-5 / wk-cfg-mun-4）、list-item 四份（wk-cfg-ops-10 / wk-cfg-page-1 / wk-cfg-ed-8 /
wk-cfg-mun-5）及 mono span 两份（wk-cfg-ops-12 /
wk-cfg-page-3）。后续任何一处改版（如调整间距/断点）都需多点同步，易漏改造成跨页面不一致。建议抽取域内公共类（如
wk-cfg-heading、wk-cfg-heading--stackable、wk-cfg-actions、wk-cfg-list、wk-cfg-list-item）统一复用。

- .wk-cfg-mun-1 {
+ /* 合并三份 heading 拷贝（ops-6/ed-1/mun-1）为单一类 */
+ .wk-cfg-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
+   border-bottom: 1px solid #eef1f5;
+   padding-bottom: 16px;
+   margin-bottom: 16px;
+ }


─── apps/web/src/configuration/config-u.css:258-263 ───
[maintainability · low] 死声明：`.wk-cfg-page-16` 中 `font-size: 0.8rem` 与 `color: #6941c6` 随即被同块的
`font-size: 0.85rem !important` 与 `color: rgba(0,0,0,0.6) !important` 覆盖，永不生效。同类问题还有
`.wk-cfg-ed-5`：`display: flex !important` 之下的 `grid-template-columns: auto 1fr` 无效。虽是忠实平移原 utility
串（原串本身冗余），但作为新写的语义 CSS 应删除无效声明，避免误导后续维护者以为徽标使用紫色/0.8rem。

  .wk-cfg-page-16 {
    border-radius: 999px;
    padding-inline: 0.55rem;
    padding-block: 0.2rem;
-   font-size: 0.8rem;
-   color: #6941c6;
+   background-color: #f4f3ff;
+   margin-right: auto;
+   font-size: 0.85rem !important;
+   color: rgba(0, 0, 0, 0.6) !important;
+ }


─── apps/web/src/configuration/config-u.css:5-8 ───
[maintainability · low] 主题色硬编码为字面量（rgba(0,0,0,.6)、#e7e7e7 等），注释自证对应 --td-text-color-secondary /
--td-component-stroke。同 PR 的 packages/design-tokens/src/tdesign-theme.css 已定义这些变量且包含完整暗色模式（dark 下
secondary 文字为 rgba(255,255,255,.55)、stroke 为
#383838）；字面量在主题切换时不会联动。建议改用带字面量兜底的变量引用，亮色渲染不变、暗色可自动适配：`color: var(--td-text-color-secondary, rgba(0,
0, 0, 0.6))`、`border-color: var(--td-component-stroke, #e7e7e7)`。

  .wk-cfg-ops-1 {
-   /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
-   color: rgba(0, 0, 0, 0.6);
+   color: var(--td-text-color-secondary, rgba(0, 0, 0, 0.6));
  }


─── apps/web/src/commercial/surface.tsx:18-18 ───
[style · low] toneClass 采用三层链式嵌套三元（error → success → warning → 兜底），违反本仓 Review
Checklist「禁止嵌套三元表达式」。仓库内已有更简洁的等价写法：shared/wk-legacy.tsx:29 的 WkStatus 用 `tone !== 'neutral' &&
\`wk-status--${tone}\`` 一层表达式实现同一确定性映射（OCR round-3 亦建议改为模板字符串形式，本处未跟进）。建议对齐：

-   const toneClass = tone === 'error' ? 'wk-cs-status--error' : tone === 'success' ? 'wk-cs-status--success' : tone === 'warning' ? 'wk-cs-status--warning' : '';
+   const toneClass = tone !== 'neutral' ? `wk-cs-status--${tone}` : '';


─── apps/web/src/commercial/surface.tsx:15-16 ───
[maintainability · low] 本 Status（含 commercial-u.css 中 .wk-cs-status 族的
#506078/#b42318/#137333/#9a6700、13px、0.25rem 及 role 映射）与 shared/wk-legacy.tsx 的 WkStatus /
wk-legacy.css 的 .wk-status 族取值逐字相同，已是代码库中第 4
份相同副本（wk-legacy、config-u、data-sources-u、commercial-u）。shared/wk-legacy 正是为「Card/Status 无 TDesign
对应组件」而建的共享兼容层，建议至少对 Status 直接 `export { WkStatus as Status } from '../shared/wk-legacy.tsx'` 复用（Card
因描边色差异 #e7e7e7 vs #dce3ed 如复用需单独覆盖描边），否则后续状态色/尺寸调整需同步修改 4 处。



─── apps/web/src/commercial/BillingPage.tsx:55-55 ───
[documentation · low] 常量已从 Tailwind utilities 换成语义类名（wk-bill-*），且对照对象 UsagePanel 的表格也已迁移为
.usage-table 语义类，但上方第 54 行注释仍写「同款样式（tailwind utilities）」，括注已过时，易误导后续维护者以为此处仍走 utilities 栈，建议同步更新措辞。



─── apps/web/src/data-sources/DataSourcesPage.tsx:452-452 ───
[bug · medium] settings 必填校验随本次迁移丢失：旧实现中 VUE_SETTINGS_FIELDS 的 Textarea 带
`required={!field.optional}`（rss 的 feed_urls 为必填），浏览器会在提交前拦截空值；迁移后该属性被移除，而 save() 仅手动校验了
name/type/schedule（DataSourcesPage.tsx:188-191）、凭证（firstMissingRequiredCredential，:196-202）和 gitlab
project_id（:206-209），没有任何针对 settingsText 的校验，buildDataSourceInput 也不会拦截。结果是空 feed_urls 的 RSS
数据源可被直接创建，随后 :241 的首次自动同步必然失败，留下一条坏行。建议在 save() 的 gitlab 分支后补一段 settings 字段遍历（与凭证走同一条 `${label}
${t('dataSource.isRequired')}` 警示路径）。

-       <fieldset className="wk-ds-27"><legend className="wk-ds-28">{t('dataSource.connectorSettingsLabel')}</legend>{form.type === 'gitlab' ? <div className="wk-ds-8" data-kind="gitlab-projects"><div className="wk-ds-30"><span className="wk-ds-31">{t('dataSource.gitlab.projects')}</span><Button type="button" theme="default" variant="text" size="small" onClick={() => updateForm('gitlabProjects', [...(form.gitlabProjects ?? []), { ...emptyGitLabProject }])}>{t('dataSource.gitlab.addProject')}</Button></div><small className="wk-ds-12">{t('dataSource.gitlab.projectsHint')}</small>{(form.gitlabProjects ?? []).map((project, index) => <div key={index} className="wk-ds-32" data-kind="gitlab-project-row"><div className="wk-ds-33"><strong className="wk-ds-34">{t('dataSource.gitlab.project')} {index + 1}</strong><Button type="button" theme="danger" variant="text" size="small" aria-label={t('common.delete')} onClick={() => updateForm('gitlabProjects', (form.gitlabProjects ?? []).filter((_, at) => at !== index))}>×</Button></div><label className="wk-ds-29"><span>{t('dataSource.gitlab.projectId')} *</span><Input placeholder={t('dataSource.gitlab.projectIdPlaceholder')} value={project.project_id} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, project_id: String(value) } : item))} /></label><label className="wk-ds-29"><span>{t('dataSource.gitlab.ref')}</span><Input placeholder={t('dataSource.gitlab.refPlaceholder')} value={project.ref} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, ref: String(value) } : item))} /></label><label className="wk-ds-29"><span>{t('dataSource.gitlab.paths')}</span><Textarea autosize={{ minRows: 2, maxRows: 2 }} placeholder={t('dataSource.gitlab.pathsPlaceholder')} value={project.pathsText} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, pathsText: String(value) } : item))} /></label></div>)}</div> : VUE_SETTINGS_FIELDS[form.type] ? VUE_SETTINGS_FIELDS[form.type].map((field) => <label key={field.key} className="wk-ds-29"><span>{t(field.label)}</span><Textarea autosize={{ minRows: 3, maxRows: 3 }} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)} onChange={(value) => updateForm('settingsText', setCredentialValue(form.settingsText, field.key, String(value)))} />{field.hint ? <small className="wk-ds-12">{t(field.hint)}</small> : null}</label>) : <Textarea autosize={{ minRows: 4, maxRows: 4 }} value={form.settingsText} onChange={(value) => updateForm('settingsText', String(value))} placeholder={t('dataSource.settingsPlaceholder')} />}</fieldset>
+ // save() 中 gitlab projectRequired 分支之后补充：
+ for (const field of VUE_SETTINGS_FIELDS[form.type] ?? []) {
+   if (!field.optional && !credentialValue(form.settingsText, field.key)) {
+     setMessage({ tone: 'warning', text: `${t(field.label)} ${t('dataSource.isRequired')}` });
+     return;
+   }
+ }


─── apps/web/src/data-sources/DataSourcesPage.tsx:441-441 ───
[bug · medium] `wk-ds-self-start` 在本域 CSS 中无定义：全仓库唯一定义位于
`apps/web/src/knowledge/knowledge-u.css:700`（`.wk-ds-self-start { justify-self: start; }`），而该文件仅由
KnowledgeGraphPage 懒加载导入。未访问过知识图谱页时，prereq 网格（.wk-ds-8 为 grid 容器）内这条链接会失去 justify-self:start
而整行拉伸。文件尾注已为 `.wk-dsui-card` 修复过完全相同的"误置于懒加载 CSS"问题（OCR R1-04/31），此处是同一类遗漏——建议把定义迁入
data-sources-u.css（或直接并入 .wk-ds-15）。



─── apps/web/src/data-sources/DataSourcesPage.tsx:456-456 ───
[bug · medium] 非法 label 嵌套（同步删除复选框）：TDesign Checkbox 的根元素本身是 `<label class="t-checkbox">`，此处又把它包进外层
`<label>`，构成 HTML 规范禁止的 label 嵌套，读屏可能重复播报，个别浏览器存在点击双重触发的隐患。本更新中其他已迁移页面（如
ConfigurationEditor.tsx）的约定是 Checkbox 携带 label 属性独立使用、不套外层 label；且此处 Checkbox 已传
`label={t('dataSource.syncDeletions')}`，外层 label 完全冗余，直接去掉即可。

-       <label><Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} /></label>
+       <Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} />


─── apps/web/src/data-sources/DataSourcesPage.tsx:484-484 ───
[bug · medium] 同样的非法 label 嵌套出现在删除确认面板：`<label className="wk-ds-21">` 内套 TDesign Checkbox（内部自带 label
元素）。Checkbox 已通过 `label={purgeLabelText}` 关联文案，外层 label 冗余且不合规；若需保留 wk-ds-21 的 flex 布局，把外层换成 div 即可。

-       <label className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></label>
+       <div className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></div>


─── apps/web/src/data-sources/DataSourcesPage.tsx:475-477 ───
[bug · low] 删除请求进行中弹窗仍可经 ESC / 右上角关闭：`closeOnOverlayClick={false}` 与 cancelBtn disabled
只挡住了遮罩和取消钮，TDesign Dialog 默认 `closeOnEscKeydown=true`、`showClose=true` 均未随 deleteSubmitting
收紧。用户在请求飞行中关掉弹窗会丢失 loading 反馈；更进一步的边角是：confirmDelete 成功路径无条件
`setDeleteSource(null)`，若用户关掉后又打开了另一个源的删除面板，前一个请求的成功回调会把这个新面板误关。建议至少加
`closeOnEscKeydown={!deleteSubmitting}` 并在 onClose 中带同样的守卫。

      closeOnOverlayClick={false}
+     closeOnEscKeydown={!deleteSubmitting}
      width={440}
-     onClose={() => setDeleteSource(null)}
+     onClose={() => { if (!deleteSubmitting) setDeleteSource(null); }}


─── apps/web/src/data-sources/DataSourcesPage.tsx:451-451 ───
[style · low] 迁移残留的空 className=""：取消替换凭证按钮上保留了无内容的 class 属性，属死代码，建议清理。



─── apps/web/src/data-sources/ui.tsx:10-13 ───
[maintainability · medium] Card 与 shared/wk-legacy.tsx 的 WkCard/WkStatus 逐字重复且取值分叉：本次更新同时新增了共享封装
`apps/web/src/shared/wk-legacy.tsx`（WkCard/WkStatus → .wk-card，边框
#dce3ed，已NotFoundPage/JoinPage/DocumentsPage 等使用），本文件在结构、role 逻辑、tone 变体上与其 1:1 相同，但 `.wk-dsui-card`
边框取 #e7e7e7（Vue --td-component-stroke）。同一应用内现在存在两种卡片描边，而注释声称"视觉 = 既有 .wk-card"，对后续维护者有误导（无法判断
#e7e7e7 与 #dce3ed 哪个是有意为之）。建议改为复用 `import { WkCard, WkStatus } from
'../shared/wk-legacy.tsx'`（configuration/ui.tsx、commercial/surface.tsx 存在同样的四份副本，可一并收编）；若确需保留 Vue
token 的有意分叉，请在注释中显式说明与 .wk-card (#dce3ed) 的差异原因。



─── apps/web/src/data-sources/data-sources-u.css:27-30 ───
[maintainability · low] 重复/自覆盖声明降低可维护性：.wk-ds-2（及 .wk-ds-42、.wk-ds-77）先后声明 `transition-duration:
150ms;` 与 `transition-duration: 200ms;`（仅后者生效）；.wk-ds-25（及 .wk-ds-68）先 `border-style: solid` 后
`dashed`；`.wk-ds-2:focus-visible` 的规则拆散在文件头部与尾部两处（.wk-ds-51、.wk-ds-53 也存在主块+尾部补丁双定义）。虽然这是 utilities
生效值的忠实转写，但读者无法直观判断哪条生效，后续极易被误改。建议每条规则只保留最终生效值，并将同选择器的规则合并到一处（如把 outline-color/offset 并回主块的
:focus-visible）。

    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
-   transition-duration: 200ms;
+   transition-duration: 200ms; /* utilities 叠加后的生效值（duration-150 被 duration-200 覆盖） */
  }


─── apps/web/src/configuration/ui.tsx:20-20 ───
[style · low] Status 组件的 tone→class 映射使用了三层嵌套三元表达式（评审规则明确禁止嵌套三元），可读性差且新增 tone 时需继续嵌套。建议改为查表映射。

-   const toneClass = tone === 'error' ? 'wk-cfg-status--error' : tone === 'success' ? 'wk-cfg-status--success' : tone === 'warning' ? 'wk-cfg-status--warning' : '';
+   const TONE_CLASS: Record<'neutral' | 'error' | 'success' | 'warning', string> = {
+     neutral: '', error: 'wk-cfg-status--error', success: 'wk-cfg-status--success', warning: 'wk-cfg-status--warning',
+   };
+   const toneClass = TONE_CLASS[tone];


─── apps/web/src/apps/AppsPages.tsx:96-96 ───
[bug · medium] PageFrame 的刷新按钮改用 TButton 后丢失了 variant="outline"：TDesign Button 默认
variant='base'（theme=default 的灰色填充按钮，背景色来自 .t-button--variant-base），而 .wk-apps-8 只覆盖了
border-color/文字色/尺寸，未覆盖 background-color，导致 authorization/action 两页头部刷新按钮呈"灰底+灰描边"。这与三处事实源不一致：① 本
hunk 被删注释原话 "the refresh control is Vue's medium outline button (32px, icon + label)"；② Vue 事实源
frontend/src/views/apps/AuthorizationView.vue:8-9 明确写了 variant="outline"；③ 同批迁移的
CatalogPage/ConnectionsPage 刷新按钮均为 <TButton variant="outline" ...>。建议补上
variant="outline"（如需精确复刻旧描边色可同步在 wk-apps-8 补 background-color: #fff）。

- return <main className={`wk-page wk-apps-26 ${gapClass}`}><header className="wk-apps-5"><div><h1 className="wk-apps-6">{title}</h1><p className="wk-apps-7">{description}</p></div><TButton aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton></header>{loading ? <div role="status"><Status>{loadingLabel}</Status></div> : null}{children}</main>;
+ <TButton variant="outline" aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton>


─── apps/web/src/apps/AppsPages.tsx:149-149 ───
[bug · low] CatalogPage 根元素由旧实现的 <main className="wk-page"> 改为 <div
className="apps-view">（ConnectionsPage 同理），而 PlatformShell 的 outlet 不提供任何 <main>
地标（apps/web/src/platform 下无 main），这两个路由因此失去 main
地标；同仓其余页面（FAQPage、AnalyticsPage、DataSourcesPage、DocumentsPage、IntegrationsPage 等）仍各自提供
<main>，形成应用内不一致，屏幕阅读器无法按地标快速跳转。改为 <main className="apps-view"> 即可两全：类名与 DOM 结构不变，不影响 apps.td.css 的
Vue 1:1 样式锚点。

- <div className="apps-view">
+ <main className="apps-view">


─── apps/web/src/experts/ExpertsPage.tsx:722-722 ───
[maintainability · low] `wk-experts-persona` 类在全仓 CSS 中无任何定义（apps/web/src/**/*.css 及 packages
均无匹配），测试与 TSX 也无其他引用点；同元素已有 `data-expert-persona` 属性承担 QA 钩子职责。属迁移后遗留的死类名，建议移除，避免误导后续维护者以为存在对应样式规则。

- className="wk-experts-persona wk-exp-16"
+ className="wk-exp-16"


─── apps/web/src/experts/experts-u.css:796-798 ───
[maintainability · low] `.wk-exp-45` 的 `width: 100%` 仅因在文件中晚于 `.wk-exp-xp-input`（`width:
200px`）出现而胜出——两者同为单类选择器 (0,1,0)，覆盖关系完全依赖文件内规则顺序；加上 wk-exp-N 为无语义编号（42 实为 toast、45
实为宽度补丁），一旦按编号重排或拆分文件，弹窗内输入框将静默回退为 200px 定宽。建议改用复合选择器显式声明覆盖意图，使其与源码顺序解耦。

- .wk-exp-45 {
+ /* 复合选择器显式声明对 .wk-exp-xp-input width:200px 的覆盖，不依赖文件内先后顺序 */
+ .wk-exp-xp-input.wk-exp-45 {
    width: 100%;
  }


─── apps/web/src/experts/experts-u.css:420-421 ───
[style · low] `.wk-exp-9`、`.wk-exp-22`、`.wk-exp-34` 三处存在 `border-bottom-width: 1px;border-color:
...` 两条声明挤在同一行的格式问题（.wk-exp-22/.wk-exp-34 为 border-top 变体），与文件其余部分的一声明一行风格不一致，影响 diff 审阅与 lint
一致性；另各 hover/disabled 态重复声明基类已设置的 `border-style: solid`，属冗余。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/data-sources/data-sources-u.css:508-513 ───
[style · low] 转写残渣（非 finding #7 的重复声明范围）：`border-bottom-width: 1px;` 与 `border-color: #e7e7e7;`
两条声明被挤在同一行（缺换行）。`.wk-ds-71` 存在完全相同的拼接错误。功能上等效，但与文件其余部分的一声明一行风格不一致，属自动转写产物，建议在 #7 的清理中一并规整为每行一条声明。

  .wk-ds-58 {
    border-bottom-style: solid;
    /* 旧栈 line/line-soft→Vue var(--td-component-stroke)=#e7e7e7 */
-   border-bottom-width: 1px;border-color: #e7e7e7;
+   border-bottom-width: 1px;
+   border-color: #e7e7e7;
    padding-block: 0.9rem;
  }


─── apps/web/src/data-sources/data-sources-u.css:612-621 ───
[style · low] 同 .wk-ds-58：`border-bottom-width: 1px;border-color: #e7e7e7;`
两条声明拼接在同一行，建议拆分为每行一条，保持文件风格一致。

  .wk-ds-71 {
    align-items: center !important;
    display: flex;
    justify-content: space-between;
    gap: 16px;
    border-bottom-style: solid;
    /* 旧栈 line/line-soft→Vue var(--td-component-stroke)=#e7e7e7 */
-   border-bottom-width: 1px;border-color: #e7e7e7;
+   border-bottom-width: 1px;
+   border-color: #e7e7e7;
    padding-block: 0.9rem;
  }


─── apps/web/src/experts/ExpertsPage.tsx:46-47 ───
[maintainability · low] `import '../chat/views-chat-u.css'` 是无消费点的死导入：ExpertsPage 渲染的 DOM 中只有
`wk-page`（标记类，定义于平台壳层选择器而非该文件）、`wk-exp-*`（experts-u.css）与 `wk-experts-persona`（全仓无定义，另见已确认
finding）。views-chat-u.css 全部为 `.wk-vc-*` 前缀的 chat 域类，与 experts 页面零交集；persona markdown 经
renderChatMarkdown 产生的 math-block/math-inline 类也定义在 chat.css（且被 .wk-chat-message-content
作用域限定），并不在本导入的文件里。该导入会把约 2545 行 chat CSS 拉入 experts 页面的模块图，既违背本次按域拆分 CSS 的架构意图，也会误导后续维护者以为 experts
页面依赖 chat utilities，建议移除。



─── apps/web/src/faq/FAQPage.tsx:1154-1156 ───
[bug · high] 导出菜单的嵌套顺序反了：Dropdown 的直接子元素是 Tooltip。tdesign-react 的 Dropdown 依赖向唯一子元素注入触发事件与 ref（Popup
cloneElement），Tooltip 组合组件不会把这些透传到内部 Button，点击大概率无法展开导出下拉。同文件上方新建菜单（Tooltip 包 Dropdown）以及 documents
域迁移（KnowledgeDocumentsPage.tsx:1685 `Tooltip > Dropdown > TdButton`）都是正确顺序，此处应改为 Tooltip 在外，并把
aria-label/title 放回 Button 上。

- <Dropdown options={faqExportOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
-                     <Tooltip content={t('knowledgeEditor.faqExport.exportButton')} placement="top">
+ <Tooltip content={t('knowledgeEditor.faqExport.exportButton')} placement="top">
-                       <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" loading={exportLoading} icon={<TIcon name="download" size="16px" />} />
+                     <Dropdown options={faqExportOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
+                       <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" aria-label={t('knowledgeEditor.faqExport.exportButton')} title={t('knowledgeEditor.faqExport.exportButton')} loading={exportLoading} icon={<TIcon name="download" size="16px" />} />
+                     </Dropdown>
+                   </Tooltip>


─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · medium] .answer-tag 的 background/border-color 是非法 CSS：`var(--td-brand-color)1a` /
`var(--td-brand-color)33` 不能在 var() 结果后直接拼接十六进制 alpha 后缀，两条声明会被整体丢弃，检索测试结果中的答案标签将回退为 TdTag 默认着色，与
Vue 基线视觉不一致（该类被 FAQPage.tsx 与 faq-tag-tooltip.test.tsx 依赖）。应改为 color-mix 或 rgba。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/faq/FAQPage.tsx:2176-2181 ───
[maintainability · medium] 死 prop：FAQPageView 不再渲染 message（toast 已收口到此处父组件的 useEffect），但
FAQPageViewProps.message 仍声明、解构并传入（2466 行）；同理 editorTitle prop（693/785/2461 行）在 Drawer header
改为内联三元后已无任何消费者。两者留在 props 契约里会误导后续维护者以为视图仍在消费，建议从接口、解构默认值和父组件传参中一并删除（父组件 2461 行的 editorTitle
三元也随之删除）。



─── apps/web/src/faq/FAQPage.tsx:1838-1839 ───
[bug · medium] 可访问性回退：本轮把多处可聚焦的 <button> 换成了 div+onClick —— result-header（此处，原为
button+aria-expanded）、卡片 card-more-btn、popup-menu-item（1242/1246 行，原为 role=menuitem 的 button）均无
tabIndex/role/键盘事件，键盘与读屏用户无法操作；同时导入弹窗与批量标签弹窗丢失了 role="dialog"/aria-modal/aria-label，FaqTagTooltip
也删掉了点击切换与 role="tooltip"（触屏设备无法查看完整内容）。虽与 agents/kb 域的 div 写法一致，但本文件是从可访问实现回退，建议至少恢复 button 语义或补
role+tabIndex+Enter/Space 处理，并恢复弹窗 aria 与 tooltip 的触屏入口。



─── apps/web/src/faq/FAQPage.tsx:956-960 ───
[style · low] 嵌套三元表达式违反项目规则，且本轮新增了多处：此处 activeTagFilterLabel 三层嵌套（956-960 行）、导入 footer 按钮文案两层（1659 行
`importTask?.status === 'success' ? ... : ... === 'failed' ? ... : ...`）、toast theme 两层（2179
行）。建议提取为映射表或具名辅助函数，如 themeMap / importFooterLabel()。

- const activeTagFilterLabel = activeTagIds.length === 0
-     ? (tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags'))
-     : activeTagIds.length === 1
-       ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
-       : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
+ const tagFilterLabelFor = (ids: readonly number[]): string => {
+     if (ids.length === 0) return tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags');
+     if (ids.length === 1) return tags.find((tag) => tag.id === ids[0])?.name ?? t('knowledgeBase.allTags');
+     return t('knowledgeBase.tagFilterMulti', { count: ids.length });
+   };
+   const activeTagFilterLabel = tagFilterLabelFor(activeTagIds);


─── apps/web/src/faq/FAQPage.tsx:1546-1546 ───
[style · low] 宽松相等 `value == null` 违反项目“禁止 ==/!=”规则；此处与批量标签弹窗的 TdSelect
onChange（`onBatchTagValueChange(value == null || value === '' ? ...)`）共两处，建议改为 `value === null ||
value === undefined || value === ''` 显式判断。

- onChange={(value) => onFormChange({ tagId: value == null || value === '' ? '' : String(value) })}
+ onChange={(value) => onFormChange({ tagId: value === null || value === undefined || value === '' ? '' : String(value) })}


─── apps/web/src/faq/faq.td.css:43-46 ───
[maintainability · low] 本文件包含大量 React 端永不生效的 Vue transition
死样式：.fade-enter/leave-*（此处）、.modal-enter/leave-*（2231-2237）、.slide-down-*、.faq-batch-bar-fade-*；另有
TSX 无任何引用的类块（.tag-menu/.tag-menu-item、.faq-manager
.status-item、.faq-header-meta、.faq-meta-item、.empty-tip、.tag-filter-bar、.tag-load-more、.match-type-t
ag），以及 `.faq-search-drawer .question-tag` 连续重复定义的两个同名规则块。合计上千行维护噪音，建议整块清理并合并重复规则。



─── apps/web/src/faq/FAQPage.tsx:2340-2341 ───
[maintainability · low] 两处一致性问题：1) 注释声称“+ 回滚”，但 updateEntryTag 失败路径只 setMessage
弹错、并无任何回滚逻辑（本地态未做乐观更新所以也无需回滚），注释与实现不符易误导；2) 父组件 confirmBatchTag 中 batchTagBusy 与 batchTagLoading 两个
state 总是同步置位/复位（语义冗余，仅分别喂给弹窗按钮与批量条按钮的 loading），可合并为单一状态。



─── apps/web/src/market/market-u.css:657-661 ───
[maintainability · medium] 主题令牌被固化为字面量，与令牌体系形成双源。原 Tailwind 类 bg-accent/bg-surface/bg-accent-wash 经
@theme 编译为运行时 var(--color-accent)/var(--color-surface) 引用（styles.css L424/L457/L463 定义）；本文件将其展开为
#07c05f（9 处）、#ffffff、rgba(7,192,95,…) 等字面量，而 .wk-mkt-12/.wk-mkt-32 又保留 var(--wk-bg,#fff)——同一
surface/accent 语义在单文件内出现两套来源。一旦 styles.css 调整令牌值或未来引入主题覆盖，MarketPage 将与仍走令牌的页面发生色彩漂移且无编译期提示。建议以
var(--color-accent, #07c05f) 形式承接：生效值不变，同时维持与令牌的联动。

  .wk-mkt-mk-tab[aria-selected="true"] {
-   border-color: #07c05f;
-   background-color: rgba(7, 192, 95, 0.08);
-   color: #07c05f;
+   border-color: var(--color-accent, #07c05f);
+   background-color: var(--color-accent-wash, rgba(7, 192, 95, 0.08));
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/market/market-u.css:378-379 ───
[maintainability · low] 格式异常：两条声明挤占一行（border-bottom-width: 1px;border-color: …），疑似生成器拼接残留，.wk-mkt-32
的 border-top 处同样存在。另外 .wk-mkt-41/.wk-mkt-42 及 .wk-mkt-mk-tab:hover 中重复书写 border-style: solid（基类
.wk-mkt-40/.wk-mkt-mk-tab 已设置）。虽不影响生效值，但会放大 diff 噪音并增加人工核对成本，建议统一为一声明一行并去除冗余声明。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/market/MarketPage.tsx:539-539 ───
[style · low] 嵌套三元表达式违反本组规范"禁止嵌套三元"。此处 Toast 色调选择与安装行状态色选择（L727 wk-mkt-44/45/46
处）均为两层嵌套。本次虽仅替换类名字面量，但既然已触碰该行，建议顺手改为查表映射，提升可读性并便于后续扩展色调。

-           className={'wk-mkt-34 ' + (toast.tone === 'success' ? 'wk-mkt-35' : toast.tone === 'warning' ? 'wk-mkt-36' : 'wk-mkt-37')}
+ const TOAST_TONE_CLASS = { success: 'wk-mkt-35', warning: 'wk-mkt-36', error: 'wk-mkt-37' } as const;
+ // ...
+           className={'wk-mkt-34 ' + TOAST_TONE_CLASS[toast.tone]}


─── apps/web/src/market/MarketPage.tsx:43-43 ───
[documentation · low] 注释漂移：上方注释仍写 "Tailwind v4 utility recipes"（文件头 L4 也提及 "Tailwind class
constants"），但常量值已全部换成语义化 wk-mkt-* 类名，且 apps/web 的 package.json/vite 配置中 Tailwind
已整体移除。保留旧表述会误导维护者去寻找并不存在的 Tailwind 管线，建议同步更新注释（文件头 L3-L5 一并核对）。

+ /* market-u.css 语义化样式配方，整页共享；沿用 ExpertsPage 的常量组织方式。 */
  const MK_PAGE = 'wk-page wk-mkt-mk-page';


─── apps/web/src/market/market-u.css:233-238 ───
[bug · low] 键盘焦点不可见：.wk-mkt-mk-input:focus 将 outline 置为透明且无 :focus-visible 兜底，而 .wk-mkt-mk-tab /
.wk-mkt-mk-btn-primary / .wk-mkt-mk-btn-outline 完全没有 focus
样式——本页所有交互元素（页签、排行页签、安装/发布按钮、搜索框、抽屉内控件）对键盘用户均无可见焦点指示。虽是原 Tailwind 串的等价翻译而非回归，但同批迁移的 platform-u.css
已有 "focus-visible 手工段"（L661-662）补齐此类缺口的先例，建议本文件一并补上。

  .wk-mkt-mk-input:focus {
    border-style: solid;
    border-color: #07c05f;
    outline: 2px solid transparent;
+   outline-offset: 2px;
+ }
+ 
+ /* 参照 platform-u.css 的 focus-visible 手工段，补键盘焦点可见性 */
+ .wk-mkt-mk-tab:focus-visible,
+ .wk-mkt-mk-btn-primary:focus-visible,
+ .wk-mkt-mk-btn-outline:focus-visible,
+ .wk-mkt-mk-input:focus-visible {
+   outline: 2px solid #07c05f;
    outline-offset: 2px;
  }


─── apps/web/src/integrations/views-integrations-u.css:824-829 ───
[bug · high] round-3 遗留 [bug·high] 仅修复了一半：EmbedPreviewModal 的 iframe 已通过 integrations-u.css 的
`.wk-epm-13 iframe` 回补（ocr3-007），但 packages/views 的 EmbedChannelPreviewPanel（嵌入向导 step 6
的"预览"按钮触发，page.tsx:632 挂载）同样结构的 iframe 容器 `.wk-vi-24`（iframe 模式设备屏，`place-items:center` 不拉伸）与
`.wk-vi-28`（挂件面板）没有任何 iframe 尺寸规则——全仓 CSS 中 iframe 选择器仅 `.wk-epm-13 iframe` 与 craft 域一处。结果该面板预览
iframe 以浏览器默认 300×150 渲染，无法铺满 min-height 480px 的设备屏区，与注释 "Former .wk-embed-preview-screen + iframe
rules" 的 Vue 基线不符。请补齐 wk-vi 侧容器规则。

  .wk-vi-24 {
    position: absolute;
    inset: 37px 0 0;
    display: grid;
    place-items: center;
+ }
+ 
+ /* 对齐 integrations-u.css 的 .wk-epm-13 iframe（ocr3-007 同族）：预览 iframe 铺满设备屏/挂件面板 */
+ .wk-vi-24 iframe,
+ .wk-vi-28 iframe {
+   width: 100%;
+   height: 100%;
+   border: 0;
+   background: #ffffff;
  }


─── apps/web/src/integrations/views-integrations-u.css:2323-2324 ───
[bug · medium] round-3 遗留 [bug·medium] 未修：开关选中态永不生效。本条位于 @layer utilities 之外（层在 L2317 关闭）且带
!important，而 `.wk-vi-42` 的 `background-color:#cbd5e1 !important`（L988，层内）竞争同一属性——按 CSS Cascade 5，同为
!important 时 layered 恒胜 unlayered（与 normal 声明方向相反），且层比较先于特异性，故灰色轨道永远压过选中色：渠道启停开关选中后不变色（原 Tailwind 中
peer-checked:bg-primary! 与 bg-[#cbd5e1]! 同层靠特异性取胜，拆层后断裂）。同段 `.wk-vi-chip--active/--idle` 也因置于层外反转了与
unlayered 页面 CSS 的胜负关系（round-3 已记，建议一并归位）。

- .wk-vi-41:checked + .wk-switch-knob--vi { background-color: var(--color-primary, #2e6de6) !important; }
- .wk-vi-41:checked + .wk-switch-knob--vi::after { translate: 16px 0; }
+ /* 方案 A：将两条移入 @layer utilities 内、置于 .wk-vi-42 之后——同层同为 !important 时
+    特异性 (0,3,0) > (0,1,0)，选中态可胜出；
+    方案 B：仅去掉 .wk-vi-42 中 background-color 的 !important（height/width 保留），
+    让层外 important 选中规则压过层内 normal */


─── apps/web/src/integrations/views-integrations-u.css:902-904 ───
[bug · medium] round-3 遗留 [bug·medium] 未修：`calc(100%+4px)` 为非法值——calc 中 `+`
两侧必须有空白，整条声明被浏览器按语法错误丢弃。`.wk-vi-33`（智能体筛选下拉 listbox）为 absolute 定位，left:0 生效但 top 回退
auto/静态位置，下拉框不再相对触发按钮下移 4px，可能与触发按钮本身重叠。该写法沿袭自原 Tailwind 任意值 top-[calc(100%+4px)]（原本就非法），平移时应顺带修正。

  .wk-vi-33 {
    position: absolute;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/integrations/views-integrations-u.css:39-44 ───
[bug · low] round-2/round-3 连续两轮已记、本轮仍未修：`color: color:inherit;` 属性名重复属非法声明，整条被解析器丢弃（本文件共 3 处：L43
clickable / L64 static / L148 title-add，`text-[color:inherit]` 的机翻残留）。当前挂载元素为 article/span
自然继承影响有限，但应修正为 `color: inherit`；另 `.wk-vi-channel-badge-static-class` 中 `rgba(0, 0, 0, 0.6);;`
双分号笔误请一并清理。

  .wk-vi-channel-card-clickable-class {
    width: 100%;
    cursor: pointer;
    background-color: #ffffff;
-   color: color:inherit;
+   color: inherit;
  }


─── apps/web/src/integrations/integrations.td.css:19-20 ───
[maintainability · medium] round-3 遗留 [maintainability·medium]
未修：本节选择器与组件实际类名脱节——ApiPlaygroundDrawer 已改用 wk-apd-*
语义类，`.wk-api-playground-aside/-body/-section(>h4)/-step-label` 及
`.wk-api-playground-status[data-status=…]` 共 10 行规则无任何元素挂载（全仓仅本文件出现），是死样式；其中
`.wk-api-playground-error` 还与组件实际类名
`wk-api-playground-field-error`（ApiPlaygroundDrawer.tsx）拼写不一致。后续在本文件调状态色/section 布局将静默不生效。第 3 节
`.wk-embed-preview-*` 全家（约 100 行）同样为死规则（组件已改用 wk-epm-*/wk-vi-*，唯一活规则是
`.wk-integration-drawer-close`）。建议删除死规则、修正 error 拼写，保留活规则（.wk-api-playground-pre / .wk-button 族 /
三个输入框外观 / .wk-integration-drawer-close）；另注意 .wk-api-playground-pre 的可用性完全依赖 IntegrationsRoutePage
导入本文件（ApiPlaygroundDrawer 自身只 import integrations-u.css），存在跨文件隐式耦合，建议在文件头注明。



─── apps/web/src/integrations/integrations-u.css:100-102 ───
[maintainability · medium] round-3 遗留 [maintainability·medium] 未修：按钮双源定义。组件同时挂
wk-button/wk-button--text 与 wk-apd-8/wk-apd-13，integrations.td.css 的 `.wk-api-playground-overlay
.wk-button` 族（活规则）与本处对 padding/border/background/color 同属性各写一遍，靠 u.css 的 !important + 文件加载顺序 +
选择器特异性三层裁决且方向不一致——`.wk-muted` 色值最终听 td.css（特异性 0,2,0 > 0,1,0，与 wk-apd-2 注释声称的 rgba(0,0,0,.6)
不符），按钮却又靠 u.css !important 压回 td.css。另外：1) wk-apd-13 内 `border-style: solid` 声明两次（普通 +
!important），前者恒死；2) 本文件几乎全部硬编码字面量（#ffffff/#cbd5e1/#2e6de6…）而 td.css 同义规则用 var(--color-*,
fallback)，同域两套口径，主题定制时硬编码侧不可覆盖。建议收敛单一来源（组件去 wk-apd-8/13 全走 td.css 的 .wk-button 族，或反向删除 td.css
按钮规则），并统一 var() token 形式。



─── packages/views/src/integrations/page.tsx:1768-1769 ───
[maintainability · low] round-3 遗留 [maintainability·low] 未修：`{false && ...}`
四段（cli×2/chrome/claw）为条件恒假、永不渲染的死分支，且本轮平移仍在其中持续替换类名与图标（⧉→CopyIcon），后续每次样式调整都要陪着改，徒增维护成本并误导读者以为该路径可达。
建议整体删除；若确有保留意图（待启用占位），请显式注释说明。



─── packages/views/src/integrations/page.tsx:1916-1918 ───
[style · low] round-3 遗留 [style·low] 未修：图标尺寸口径不一——注入尺寸 15px 与回退 SVG 的 14px 以及上一行注释 "1em = 14px"
三者矛盾：apps/web 注入渲染器时图标 15px，views 包直渲染/测试回退时 14px，两环境尺寸漂移。建议统一为一个尺寸并同步修正注释（可提取共享常量）。

  function JumpIcon() {
-   return <SpriteIcon name="jump" size="15px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
+   return <SpriteIcon name="jump" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
  }


─── apps/web/src/integrations/IntegrationsPage.tsx:2-3 ───
[maintainability · low] round-3 遗留 [maintainability·low] 未修：本地 IntegrationsPage
没有任何运行时引用——IntegrationsRoutePage 渲染的是 @weknora/views/integrations/page
的同名组件，全仓对本文件的导入为零（本轮已复核），唯一消费方是 route.test.ts 对源码文本的静态 marker 断言。对不被渲染的组件做 @weknora/ui→tdesign
迁移，实质是在维护一个测试 fixture。建议在文件头明确注明其"仅作 route.test.ts
源码契约、非运行时页面"的地位，或直接删除组件并同步移除对应测试断言，避免后续维护者误以为这是线上集成页而重复维护两套实现。



─── apps/web/src/integrations/EmbedPreviewModal.tsx:92-92 ───
[maintainability · low] round-3 遗留 [maintainability·low] 未修：'正在加载预览…' 为硬编码中文，而同组件 closeLabel()/label
链路其余文案均经 integrationsT 按本地化输出；本组件已 import integrationsT/integrationsLocale 且持有 locale
prop，packages/views 的 embedWizardMessages.ts 已定义 embedPublish.previewLoading（中文值恰为"正在加载预览…"），views
端预览面板正是消费该键，可直接复用，避免 locale 切换时该弹窗中英混排。

-             {!ready ? <span className="wk-muted wk-epm-14">正在加载预览…</span> : null}
+             {!ready ? <span className="wk-muted wk-epm-14">{integrationsT(integrationsLocale(locale), 'embedPublish.previewLoading')}</span> : null}


─── apps/web/src/organizations/OrganizationsPage.tsx:1026-1027 ───
[bug · medium] round-3 遗留未修复（键盘可达性回归）：org 卡片由旧版 `role="button" tabIndex={0} onKeyDown(Enter)` 改为纯
onClick 的 div，键盘用户（Tab + Enter）无法打开组织设置弹窗，屏幕阅读器也不会将其播报为可操作元素。同理，底部 del-org-dialog 的确认/取消为 `<span
className="circle-btn-txt" onClick>`，无焦点环与键盘激活路径。建议恢复 role="button"/tabIndex/onKeyDown，弹窗按钮改用 Button
或补足键盘语义（Vue 原生 t-dialog 的 footer 按钮自带 button 语义，span 方案属降级）。

        <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
-         onClick={() => openSettingsModal(org)}>
+         role="button" tabIndex={0}
+         onClick={() => openSettingsModal(org)}
+         onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>


─── apps/web/src/organizations/OrganizationsPage.tsx:1400-1400 ───
[bug · medium] round-3 遗留未修复（5 处 label htmlFor 悬空）：本行 Input（name="organization-name"）未传 id，create 模式
`<label htmlFor="organization-name">` 找不到目标控件；同类还有 join-code、join-search 的 TInput 以及
upgrade-role、join-request-role 的 TSelect（label 有 htmlFor 但控件无 id）。点击 label 不聚焦、可访问性名称缺失。TDesign
控件支持透传 id，建议逐一补上；或改用 aria-label 收口。

-                                 <Input name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />
+                                 <Input id="organization-name" name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />


─── apps/web/src/organizations/org-u.css:928-929 ───
[bug · low] round-3 遗留未修复：`max-height: calc(90vh-120px)` 减号两侧缺空格，属无效 CSS 声明会被整条丢弃（系从旧 Tailwind 同样无效的
calc 值平移而来）。加入组织弹窗内容区因此失去最大高度约束，长内容（搜索结果列表）会顶出弹窗并可能被外层 overflow:hidden 裁剪而无法滚动。修复：改为 `calc(90vh -
120px)`。

  .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);


─── apps/web/src/organizations/OrganizationsPage.tsx:1289-1291 ───
[maintainability · low] round-3 遗留未修复（不可达分支）：列表渲染链 loading / listError /
ordered.length===0&&!loading / ordered.length>0 四个条件已穷尽全部状态组合，末尾 `: null`
永不执行，属死代码。前两个分支的互斥条件也可对齐（`ordered.length === 0 && !loading` 与首分支 `loading && ordered.length === 0`
不对称，读者需推理才能确认穷尽）。建议清理冗余分支或将空态改为显式 else。

-           ) : ordered.length > 0 ? (
+           ) : (
              <div className="org-card-wrap">{cardRows}</div>
-           ) : null}
+           )}


─── apps/web/src/organizations/OrganizationsPage.tsx:291-291 ───
[maintainability · low] round-3 遗留未修复：嵌套三元（stat-member → stat-kb →
agent）违反项目嵌套三元禁令；且三种徽标图标尺寸不一致——IconUser 默认改为 12px，folder/agent 内联 svg 为 14px，并排渲染时视觉不齐。建议改为查表（tone →
icon 映射）并统一尺寸为 14px。

-       {props.tone === 'stat-member' ? <IconUser /> : props.tone === 'stat-kb' ? (
+       {props.tone === 'stat-member' ? <IconUser size={14} /> : props.tone === 'stat-kb' ? (


─── apps/web/src/organizations/OrganizationsPage.tsx:21-22 ───
[maintainability · low] round-3 遗留未修复（导入与控件栈一致性）：① 第 14 行裸导入的 `FormEvent` 未被使用（文件内均为
`React.FormEvent`），属死导入；② 同模块两条 import 产生 Input/TInput、Textarea/TTextarea
双别名（同一组件两个名字），编辑时极易拿错（事实上编辑模式名称字段已用原生 `<input>` 而创建模式用 TDesign Input，行为栈不一致）；③ 编辑模式 `maxlength` 与
TInput 的 `maxlength` 大小写混用。建议合并为单条 import、删除未用 FormEvent、统一编辑/创建两模式的输入控件。

- import { Input as TInput, Select as TSelect, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
- import { Button, Dialog, Input, Popup, Skeleton, Tag, Textarea, Tooltip } from 'tdesign-react';
+ import { Button, Dialog, Input, Popup, Select, Skeleton, Switch, Tag, Textarea, Tooltip } from 'tdesign-react';


─── apps/web/src/organizations/OrganizationsPage.tsx:1332-1334 ───
[bug · low] round-3 遗留未修复（<720px 导航重复 + 注释前提不成立）：注释称"侧栏不可见时的等价 section 切换"，但迁移后 settings-sidebar
无任何移动端隐藏规则（旧代码有 max-[720px]:hidden），小屏下侧栏与该下拉选择器同时显示，出现两套导航；且注释描述与实际行为相悖。建议：要么为 .settings-modal
.settings-sidebar 补 @media (max-width:720px){display:none}，要么删除 wk-org-5 选择器并修正注释。

-                 {/* React 韧性补充（<720px 时侧栏不可见时的等价 section 切换），
-                     Vue 无对应物；桌面扫描态 display:none 零像素影响。 */}
+                 {/* React 韧性补充（Vue 无对应物）；wk-org-5 仅 <720px 显示，
+                     配合 orgs.td.css 中 settings-sidebar 的移动端隐藏规则。 */}
                  <div className="wk-org-5">


─── apps/web/src/faq/FAQPage.tsx:1561-1561 ───
[bug · medium] i18n 键误用：`general.close` 定义于 settings
命名空间（packages/i18n/src/settings.ts），五种语言的值均为「关闭设置 / Close Settings / 設定を閉じる / 설정 닫기 / Закрыть
настройки」，且 formatMessage 会把 settingsMessages 合并进总目录（packages/i18n/src/index.ts:1083），因此此处与 1672
行（批量标签弹窗）两个关闭按钮的 aria-label 会读出「关闭设置」，语义错误并误导读屏用户。应改用通用键 `common.close`（本文件 FAQ_FALLBACK_MESSAGES
已定义为「关闭/Close」，符合既有注释"common.close labels the drawer close button"的约定）。

- <button className="close-btn" aria-label={t('general.close')} onClick={() => onCloseImport()}>
+ <button className="close-btn" aria-label={t('common.close')} onClick={() => onCloseImport()}>


─── apps/web/src/faq/FAQPage.tsx:1659-1659 ───
[bug · low] 不可达死分支 + 潜在误触发：confirmImport 成功路径先 setImportOpen(false)（2386 行）再
setImportTask(...)，失败路径不设置 importTask，且 onOpenImport 每次重置 importTask 为 null——弹窗打开期间 importTask 恒为
null，本行 success→「关闭」/ failed→「重试」的文案分支及上方 disabled={importTask?.status === 'running'}
永远不会生效。且文案为「关闭」时 onClick 仍调用 onImportConfirm()，一旦未来时序变化使该分支可达，点「关闭」会再次提交导入。建议：不可达则删除分支简化为固定文案；若要保留
success 态，则需在文案为「关闭」时改为调用 onCloseImport。

- {importTask?.status === 'success' ? t('common.close') : importTask?.status === 'failed' ? t('common.retry') : t('knowledgeEditor.faqImport.importButton')}
+ {t('knowledgeEditor.faqImport.importButton')}


─── apps/web/src/faq/FAQPage.tsx:991-991 ───
[maintainability · low] 死分支：faqExportOptions 的 value 只有 'export_csv' /
'export_json'，faqCreateOptions 为 'create' / 'import'，检索按钮直接调 handleFaqAction('search')，没有任何调用方派发
'export'，此 case 永不命中，建议删除以免误导后续维护者以为存在该入口。



─── apps/web/src/faq/FAQPage.tsx:521-521 ───
[bug · low] 不可达分支 + 缺失 i18n 键：1) 该空态位于外层 `kbList.length ? (...)` 为真的分支内，而 sortedKbList 由 kbList
重排而来必然非空，`!sortedKbList.length` 永不为真；2) `common.noData` 在 @weknora/i18n 全部目录及本文件
FAQ_FALLBACK_MESSAGES 中均无定义，formatMessage 缺键时原样返回
key（packages/i18n/src/index.ts:1075），一旦未来该分支可达将把原始键名 "common.noData" 渲染给用户。建议直接删除该空态行。



─── apps/web/src/faq/faq.td.css:1726-1735 ───
[maintainability · low] 相邻重复选择器：`.faq-editor-drawer .full-width-input-wrapper .add-item-btn`
在此处与其后紧跟的 !important 块使用同一选择器连续声明两块，其中 `border-radius: 8px`
声明了两次且前者必然被后者（!important）覆盖。建议合并为单个声明块，避免后续修改时改一处漏一处。



─── apps/web/src/shared/wk-legacy.tsx:153-157 ───
[bug · medium] 拖拽 body 全局样式锁在组件卸载时残留：beginResize 直接写入 document.body 的 cursor/userSelect，仅依赖 window
mouseup 的 stop 恢复，而本 cleanup 只移除监听器、不重置 body 样式。可复现路径：按住拖宽手柄时按 Esc（document keydown 触发 onClose →
open=false → 卸载执行 cleanup），此后全局光标残留 col-resize、整页文本无法选中，直到用户重开抽屉并完整拖一次。建议 cleanup 中检测
resizeRef.current 非空时重置。附带：panelWidth 列入依赖导致拖拽期间每次宽度变化都拆除并重绑 window 监听，且 stop 闭包可能捕获上一帧宽度向
localStorage 写入陈旧值——若改用 ref 读取 panelWidth（同 onCloseRef 模式）可一并消除这两个问题。

      return () => {
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', stop);
+       if (resizeRef.current) {
+         resizeRef.current = null;
+         document.body.style.cursor = '';
+         document.body.style.userSelect = '';
+       }
      };
    }, [maxWidth, minWidth, panelWidth, resizable, side, storageKey]);


─── apps/web/src/shared/wk-legacy.tsx:133-135 ───
[bug · medium] localStorage 读写无异常保护：存储被禁用环境（旧版 Safari 隐私模式、无 allow-same-origin 的 iframe sandbox）下
getItem 会抛 SecurityError，此处位于 useEffect 内，异常将沿 effect 冒泡，无 ErrorBoundary 时整棵组件树被卸载（doc-detail 等依赖
storageKey 的长驻抽屉受影响）；stop 回调中的 setItem 在配额耗尽时同样会抛 QuotaExceededError。项目内同类"抽屉宽度持久化"均带 try/catch
惯例（integrations/ApiPlaygroundDrawer.tsx:215、career/RulePage.tsx:158、settings/KnowledgeDocumentsPage
viewMode 等），建议对齐。

      if (!storageKey || typeof window === 'undefined') return;
-     const saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
+     let saved: number;
+     try { saved = Number.parseFloat(window.localStorage.getItem(storageKey) || ''); } catch { return; }
      if (Number.isFinite(saved)) setPanelWidth(Math.max(minWidth, Math.min(maxWidth, saved)));


─── apps/web/src/shared/wk-legacy.css:31-31 ───
[maintainability · medium] --wk-overlay 在现存任何 CSS 中均无定义（packages/ui 已删除，原取值无法复核），兜底 rgba(0,0,0,0.5)
将 100% 生效；而同文件 .wk-sheet-backdrop 及项目内抽屉遮罩惯例（settings.td.css:1846 .wk-audit-drawer-overlay）均为 rgb(23
32 51 / 0.45)。同一弹层兼容层内 dialog 与 sheet 遮罩颜色不一致——若原 --wk-overlay 取墨蓝系，T15 后所有 WkDialog
消费方（kb-documents / knowledge-graph / invitation-inbox / wiki 等）遮罩将发生视觉回归。建议将兜底与 sheet 遮罩统一为 rgb(23
32 51 / 0.45)，或在 design-tokens 中恢复 --wk-overlay 定义使其可核验。

- .wk-dialog-backdrop { position: fixed; inset: 0; display: grid; place-items: center; padding: 1rem; background: var(--wk-overlay, rgba(0, 0, 0, 0.5)); z-index: var(--wk-overlay-dialog-z, 3000); }
+ .wk-dialog-backdrop { position: fixed; inset: 0; display: grid; place-items: center; padding: 1rem; background: rgb(23 32 51 / 0.45); z-index: var(--wk-overlay-dialog-z, 3000); }


─── apps/web/src/shared/wk-legacy.tsx:196-198 ───
[bug · low] aria-label={String(title)} 对 ReactNode 类型的 title 会产出 "[object
Object]"：真实消费方已传复合结构（documents/KnowledgeDocumentDetailPage.tsx:318 的 doc drawer title 为嵌套 span
含操作按钮）。因 aside 同时有 aria-labelledby={titleId} 指向 h2，按 ARIA 优先级实际朗读取 labelledby，垃圾值多数场景被覆盖，但 axe
等可访问性审计工具会标记无效 aria-label。建议直接删除该行（labelledby 已覆盖字符串 title 场景）。

            aria-labelledby={titleId}
            tabIndex={-1}
-           aria-label={String(title)}


─── apps/web/src/shared/shared-u.css:30-32 ───
[maintainability · low] border-style: solid
在同一条规则内重复声明两次（.wk-shared-4/.wk-shared-8/.wk-shared-13/.wk-shared-15
四处均有此迁移工具展开痕迹），无功能影响但造成阅读困惑，建议去重。

    border-style: solid;
    border-width: 1px;
-   border-style: solid;


─── apps/web/src/shared/wk-legacy.tsx:75-75 ───
[maintainability · low] focusable 查询选择器中 [tabindex]:not([tabindex="-1"]) 已排除 tabindex=-1 的元素，后续
.filter((element) => element.getAttribute('tabindex') !== '-1') 语义完全重复，无功能影响但徒增阅读成本，建议二选一（与 WkSheet
的 focusable 查询保持一致即可）。



─── apps/web/src/organizations/OrganizationsPage.tsx:1217-1226 ───
[bug · medium] round-4 新发现（a11y/UX 回归，两处）：① 旧实现图标按钮带 `aria-label` + `title`，迁移后纯图标 Button 仅靠
Tooltip，无任何可访问名称，屏幕阅读器读出的是无名按钮；② Firefox/Safari 中禁用的原生 `<button>` 不派发 mouse 事件，Tooltip 在
`!canManageOrg` 禁用态无法弹出——注释所述「禁用态 tooltip 切 rbac 提示」在非 Chromium 浏览器失效，用户失去 writeGuardTitle
权限提示。空状态的两颗按钮（1277-1286 行，`<Tooltip content={writeGuardTitle} disabled={canManageOrg}>` 包裹 disabled
Button）同样丢失了旧 `title` 兜底。建议在 Button 上恢复 aria-label（可访问名称）并以原生 title 作为禁用态提示兜底。

                    <Button
+                     aria-label={t(locale, 'organization.joinOrg')}
+                     title={canManageOrg ? undefined : writeGuardTitle}
                      variant="text"
                      theme="default"
                      size="small"
                      className="header-action-btn"
                      style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                      disabled={!canManageOrg}
                      onClick={openJoinModal}
                      icon={<TIcon name="enter" size="16px" />}
                    />


─── apps/web/src/organizations/OrganizationsPage.tsx:1480-1481 ───
[maintainability · low] round-4 新发现（双计数器隐患）：创建态（1413 行）的注释与 count render-prop 表明，TDesign Textarea 在
`maxlength` 生效后会自动渲染内置计数器；此处编辑态仍保留手动 `<p className="wk-org-15">{formDescription.length}/500</p>`。当前因
`maxLength`（驼峰）未按 TDesign 契约小写而不生效才未显形——一旦已确认问题 #5 统一大小写修复，本处将出现两份 0/500 计数。建议修复 #5
时与此处一并对齐：改用与创建态相同的 count render-prop 并删除手动 <p>（或维持单一手动计数）。

-                             <TTextarea id="organization-description" name="organization-description" className={ORG_FIELD + ' wk-org-95'} rows={3} maxLength={500} value={formDescription} onChange={(value) => setFormDescription(String(value))} disabled={!settingsCanManage} />
-                             <p className="wk-org-15">{formDescription.length}/500</p>
+                             <TTextarea id="organization-description" name="organization-description" className={ORG_FIELD + ' wk-org-95'} rows={3} maxlength={500} value={formDescription} onChange={(value) => setFormDescription(String(value))} disabled={!settingsCanManage} count={({ count, maxLength }) => <span className="t-textarea__limit">{`${count}/${maxLength}`}</span>} />


─── apps/web/src/wiki/WikiPage.tsx:1183-1183 ───
[maintainability · low] 根节点由 `<main className="wk-page">` 改为 `<div className="knowledge-layout">`
后，本路由整页失去 main landmark：PlatformShell 未提供 `<main>`，全应用其余路由普遍自带 `<main
className="wk-page">`（router.tsx、DocumentsPage、AppsPage 等），屏幕阅读器/快捷导航将无法定位 Wiki 页主区。knowledge-layout
为 class 选择器，外包一层 `<main>` 不影响 Vue 平移样式，建议恢复 landmark。

-     <div className="knowledge-layout">
+     <main className="knowledge-layout">


─── apps/web/src/wiki/wiki-u.css:190-192 ───
[maintainability · low] `.wk-wiki-17` 中 `font-size: 13px` 随后又被 `font-size: inherit`
覆盖，前者是永不生效的死声明（平移自旧 `text-[13px] [font:inherit]` 的层叠顺序）。实际渲染字号为继承值，与 13px
意图相悖且易误导后续维护，二者应删其一：若维持现状渲染请删 `font-size: 13px`，若输入框确需 13px（对齐 Vue 视觉）请删 `font-size: inherit`。

-   font-size: 13px;
    font-family: inherit;
    font-size: inherit;


─── apps/web/src/wiki/wiki-u.css:641-642 ───
[maintainability · low] `.wk-wiki-63` 中 `font-weight: inherit` 随后被 `font-weight: 650 !important`
覆盖，属永不生效的死声明（同 `.wk-wiki-17` 的平移残留），建议删除 `font-weight: inherit` 保持行为不变。

-   font-weight: inherit;
    font-weight: 650 !important;


─── apps/web/src/wiki/wiki-u.css:636-637 ───
[maintainability · low] `color: #1849a9` 是旧 token `--color-primary-deep` 的硬编码值（styles.css 中定义为同值
#1849a9）；且本文件大量 `#07c05f` 硬编码与 wiki-reader.css 新增段使用的 `var(--td-brand-color)`
并存，同一变更内两套取值方式。主题变量（换肤/品牌调整）在这些硬编码位置不会联动，易随主题演进产生色值漂移。平移虽以固定值为策略，但对已有 token 的取值建议改用
`var(--color-primary-deep)`（品牌绿同理用 `var(--td-brand-color)`）。

    text-align: left;
-   color: #1849a9;
+   color: var(--color-primary-deep, #1849a9);


─── apps/web/src/shared/wk-legacy.tsx:166-168 ───
[bug · high] open effect 未等待 `mounted`:组件以 open=true 挂载时(真实消费方
documents/KnowledgeDocumentDetailPage.tsx:316 的文档详情抽屉 `<Sheet open ...>` 与 :334 的 trace 抽屉均为此场景),首次
commit 渲染的是未 portal 的 inline aside;本 effect 在 setMounted 引发的 portal
重渲染之前执行,`panelRef.current?.focus()` 聚焦的节点随即被卸载、焦点回落 body。此后用户直接按 Esc 时
`!panelRef.current?.contains(body)` 命中 return,抽屉关不掉(需先点击面板内部才恢复);Tab 焦点陷阱也因 activeElement=body 不命中
first/last 而失效,可 Tab 穿透到背景页。项目内同构先例 TenantAuditDrawer.tsx:107-108 明确以 `if (!open || !mounted)
return;` 规避该坑(注释:Waits for mounted so focus lands on the portal-mounted panel)。建议同样在 guard 加入
mounted 并纳入 deps。

-     if (!open) return;
+     if (!open || !mounted) return;
      restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      panelRef.current?.focus();
+     // ...
+   }, [open, mounted]);


─── apps/web/src/shared/wk-legacy.tsx:56-60 ───
[bug · medium] 与 WkSheet 同根因:dialog 以 open=true 挂载时,本 effect 捕获的 `dialog` 是 portal 替换前的 inline
节点——该节点随后被卸载,导致:1) `openDialogStack` 压入游离节点(栈内元素永远 contains 不了 activeElement);2) 初始 focus 丢失到
body;3) Esc 的 `!dialog.contains(activeElement)` 恒真直接 return,弹窗关不掉;4) Tab 陷阱在脱离文档的节点上查询。建议 guard 同样加
`!mounted` 并纳入 deps(与 TenantAuditDrawer.tsx:107-131 先例一致)。

-     if (!open) return;
+     if (!open || !mounted) return;
      // ocr3-015：cleanup 运行在 open=false 的 commit 之后，此时节点已卸载、
      // dialogRef.current 已被置 null——栈内元素永不弹出（只增不减，残留栈顶
      // 还会挡住外层弹窗的 Escape 仲裁）。effect 体内捕获元素引用供 cleanup 使用。
      const dialog = dialogRef.current;
+     // ...
+   }, [open, mounted]);


─── apps/web/src/shared/shared-u.css:59-62 ───
[maintainability · low] 迁移保真偏差:原 Tailwind `break-all` 编码的是 `word-break: break-all`,此处写成
`overflow-wrap: anywhere`,两者断行语义不同——`break-all` 允许在任意字符间断行(即使整词本可放下),`anywhere` 仅在词无法放入当前行时才断、且参与
min-content 尺寸计算。含长 token(链接、hash)的标题换行位置将与迁移前不同,与文件头注释"值 = 迁移时 utilities
编码的生效值"的契约不符;`.wk-shared-16`(消息正文)同。若需严格对齐应改回 `word-break:
break-all`。附带:`.wk-shared-9`/`.wk-shared-17` 对应原 `text-xs` 隐含的 `line-height:
1rem`(16px)未平移,行高改为继承值,元信息行距可能轻微变化。

  .wk-shared-7 {
    margin: 0;
-   overflow-wrap: anywhere;
+   word-break: break-all;
    font-size: 20px;


─── apps/web/src/shared/wk-legacy.css:47-48 ───
[bug · low] keyframes 全局生效的副作用:settings/TenantAuditDrawer.tsx:161 的抽屉面板 inline `animation:
'sheet-in-right .2s ease-out'` 在本文件之前是注释明确记录的 no-op(TenantAuditDrawer.tsx:138:"现状即未定义 keyframes 的
no-op");而 TenantMembersPanel 所在 chunk 因 import wk-legacy.tsx 会加载本 CSS,该审计抽屉将开始播放 200ms
滑入动画——超出本兼容层"渲染不变"的目标,且令那条注释过时。若非有意恢复动画,建议使用带命名空间的 keyframes 名(如 wk-sheet-in-right,WkSheet 内联
animation 同步更名),避免波及同名引用方;若有意恢复,请同步更新 TenantAuditDrawer 的注释。

- @keyframes sheet-in-right { from { transform: translateX(24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
- @keyframes sheet-in-left { from { transform: translateX(-24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
+ @keyframes wk-sheet-in-right { from { transform: translateX(24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
+ @keyframes wk-sheet-in-left { from { transform: translateX(-24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }


LLM retry report summary: 106 of 1245 requests affected -- 27 requests failed, 5 requests cancelled, 74 requests recovered after retry

Review planning (17 requests):
- apps/miniprogram/config/index.ts,apps/miniprogram/package.json,apps/miniprogram/src/adapters/career-platform.ts,apps/miniprogram/src/career/export-deletion.config.ts,apps/miniprogram/src/career/export-deletion.gating.ts,apps/miniprogram/src/career/export-deletion.tsx,apps/miniprogram/src/services/career-intent.ts,apps/miniprogram/src/services/career.ts,apps/miniprogram/tests/export-deletion.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/app.config.ts,apps/miniprogram/src/app.scss,apps/miniprogram/src/career/application-material.config.ts,apps/miniprogram/src/career/application-material.tsx,apps/miniprogram/src/career/discovery.config.ts,apps/miniprogram/src/career/discovery.tsx,apps/miniprogram/src/career/progress-preparation.config.ts,apps/miniprogram/src/career/progress-preparation.tsx,apps/miniprogram/src/career/rules-usage-reminders.config.ts,apps/miniprogram/src/career/rules-usage-reminders.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/index.html,apps/web/package.json,apps/web/src/styles.css,apps/web/src/tdesign-icon-offline.ts,apps/web/src/tdesign-locale.tsx,apps/web/src/test-tdom-harness.ts,apps/web/tsconfig.json,apps/web/vite.config.ts,packages/design-tokens/src/styles.css,packages/design-tokens/src/tdesign-theme.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/documents/DocumentsPage.tsx,apps/web/src/documents/DocumentsPageChrome.tsx,apps/web/src/documents/KnowledgeDocumentDetailPage.tsx,apps/web/src/documents/KnowledgeDocumentsPage.tsx,apps/web/src/documents/TagPickerDialog.tsx,apps/web/src/documents/UploadConfirmDialog.tsx,apps/web/src/documents/documents-list.css,apps/web/src/documents/documents-u.css,apps/web/src/documents/documents.td.css,apps/web/src/documents/preview.ts: timed out -> failed
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css,apps/web/src/integrations/views-integrations-u.css,packages/views/src/integrations/page.tsx: timed out -> failed
- ... and 12 more

Core review (77 requests):
- apps/embed/src/EmbedApp.tsx,apps/embed/src/button.tsx,apps/embed/src/styles.css,apps/web/src/embed/EmbedEntryPage.tsx,apps/web/src/embed/embed-u.css: timed out -> failed
- apps/miniprogram/config/index.ts,apps/miniprogram/package.json,apps/miniprogram/src/adapters/career-platform.ts,apps/miniprogram/src/career/export-deletion.config.ts,apps/miniprogram/src/career/export-deletion.gating.ts,apps/miniprogram/src/career/export-deletion.tsx,apps/miniprogram/src/services/career-intent.ts,apps/miniprogram/src/services/career.ts,apps/miniprogram/tests/export-deletion.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/app.config.ts,apps/miniprogram/src/app.scss,apps/miniprogram/src/career/application-material.config.ts,apps/miniprogram/src/career/application-material.tsx,apps/miniprogram/src/career/discovery.config.ts,apps/miniprogram/src/career/discovery.tsx,apps/miniprogram/src/career/progress-preparation.config.ts,apps/miniprogram/src/career/progress-preparation.tsx,apps/miniprogram/src/career/rules-usage-reminders.config.ts,apps/miniprogram/src/career/rules-usage-reminders.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/core/errors.ts,apps/miniprogram/src/core/routes.ts,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/tests/application-material.test.mjs,apps/miniprogram/tests/assembly.test.mjs,apps/miniprogram/tests/career-discovery.test.mjs,apps/miniprogram/tests/career-platform.test.mjs,apps/miniprogram/tests/core.test.mjs,apps/miniprogram/tests/progress-preparation.test.mjs,apps/miniprogram/tests/rules-usage-reminders.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/platform/files.ts,apps/miniprogram/src/services/workbench.ts,apps/miniprogram/src/subpackages/execution/artifact/index.config.ts,apps/miniprogram/src/subpackages/execution/artifact/index.tsx,apps/miniprogram/src/subpackages/execution/artifact/t-button.d.ts,apps/miniprogram/tests/artifact-cleanup.test.mjs,apps/miniprogram/tests/build-output.test.mjs,apps/miniprogram/tests/helpers/taro-stub.mjs,apps/miniprogram/tests/live/quota-inject-proxy.mjs,apps/miniprogram/tests/live/t24r1-live-driver.cjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 72 more

Context compaction (8 requests):
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: cancelled
- apps/web/src/documents/DocumentsPage.tsx,apps/web/src/documents/DocumentsPageChrome.tsx,apps/web/src/documents/KnowledgeDocumentDetailPage.tsx,apps/web/src/documents/KnowledgeDocumentsPage.tsx,apps/web/src/documents/TagPickerDialog.tsx,apps/web/src/documents/UploadConfirmDialog.tsx,apps/web/src/documents/documents-list.css,apps/web/src/documents/documents-u.css,apps/web/src/documents/documents.td.css,apps/web/src/documents/preview.ts: cancelled
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css,apps/web/src/integrations/views-integrations-u.css,packages/views/src/integrations/page.tsx: cancelled
- apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: cancelled
- packages/views/src/chat/agent-selector.tsx,packages/views/src/chat/chat-copy.ts,packages/views/src/chat/composer.tsx,packages/views/src/chat/mermaid.ts,packages/views/src/chat/message-face.tsx,packages/views/src/chat/message-list.tsx,packages/views/src/chat/page.tsx,packages/views/src/chat/session-sidebar.tsx,packages/views/src/chat/tool-approval.tsx,packages/views/src/chat/tool-result.tsx: cancelled
- ... and 3 more

Comment re-location (2 requests):
- apps/web/src/agents/AgentEditorModal.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/SubagentsSection.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Comment filtering (2 requests):
- apps/miniprogram/src/core/errors.ts,apps/miniprogram/src/core/routes.ts,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/tests/application-material.test.mjs,apps/miniprogram/tests/assembly.test.mjs,apps/miniprogram/tests/career-discovery.test.mjs,apps/miniprogram/tests/career-platform.test.mjs,apps/miniprogram/tests/core.test.mjs,apps/miniprogram/tests/progress-preparation.test.mjs,apps/miniprogram/tests/rules-usage-reminders.test.mjs: rate limited (HTTP 429) -> succeeded
- internal/modules/career/application.go,internal/modules/career/career_export.go,internal/modules/career/material.go,internal/modules/career/resource_recovery.go,internal/modules/career/resume_extract.go,internal/modules/career/submission.go,internal/modules/career/upload.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
