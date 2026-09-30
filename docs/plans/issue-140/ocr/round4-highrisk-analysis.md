# OCR round-4 高危 findings 独立复核（HEAD 76df0cee0）

复核基准：/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 @ 76df0cee0，仅只读核验（cat/grep/git log）。
范围：ocr-round-4.md 全部 [security·high] 1 条 + [bug·high] 9 条；另按指示补核用户点名重点项（material status 契约，报告定级 bug·low）与 #140 实现面内触及核心红线的 [bug·medium]。
判定口径：VALID=当前 HEAD 代码证据成立；FALSE_POSITIVE=代码证据推翻；OUT_OF_SCOPE=代码属实但不属 #140 career 实现面（Vue→React 迁移旧面），不在 #140 修复范围。

## 一、高危 findings（security·high 1 + bug·high 9）

[VALID] bug·high internal/modules/career/career_export.go:643 边界文案承诺 search_rules「及其运行与发现待办（随完整导出携带后删除）」，但 CareerExportSearchRule(193-202) 只含规则行字段、buildCareerExportArchive(523-533) 不读 career_search_rule_runs/career_search_discovery_todos，二者却在 careerPurgeTables(703+) 被删除——用户被告知随导出携带的数据被不可逆销毁 fixHint=归档补 RuleRuns/DiscoveryTodos 分区，或将 643 行文案改为「不随导出携带，删除后不可恢复」。（注：报告对 CareerExportArchive 217-220 注释违约的指控过宽——注释所列分区均已携带；真正违约的是 643 行边界文案。evaluations/searches 等其余未导出分区的边界文案已如实写「随删除一并清除」，不在违约范围）

[VALID] bug·high apps/miniprogram/src/career/application-material.tsx:149,151 「显式继续」勾选框 acknowledged(41,144-145) 无任何消费点：按钮 disabled 只看 pendingApplication(149)，请求恒发 continueDespiteHardFailure: ineligible(151)=true，服务端 ErrApplicationHardIneligible 门禁（application.go:231-233）被永久绕过，165 行 hard_ineligible_requires_continue 分支成死代码；Web 端 ApplicationPage.tsx:149,241 有双门禁对照 fixHint=disabled 加 `|| (ineligible && !acknowledged)`，请求改 `continueDespiteHardFailure: ineligible && acknowledged`。

[VALID] bug·high apps/web/src/career/ProgressPage.tsx:163-168 lookupReceipt 的 catch 无 instanceof ReceiptMismatchError 分支：acceptReceipt(102-103) 在 try 内(162)抛出的确定性协议错误与网络失败一同落 setWritePhase('unknown') 引导循环查询；同文件 runWrite:140 已正确处理，属同文件内不一致 fixHint=catch 顶部补 instanceof 分支：置 error、清 attempt、提示刷新后重新提交。

[VALID] bug·high apps/web/src/career/SearchPage.tsx:210 queryReceipt 中 stored.requestId !== attempt.requestId 置 phase 'unknown' 并提示「请重试查询」，构成 unknown→查询→mismatch→unknown 死循环；同页 send():162 同条件正确置 error(invalid_response)+清 attempt，自相矛盾 fixHint=对齐 send()：setError({code:'invalid_response'})、setPhase('idle')、setAttempt(undefined)、放弃本次结果。

[VALID] bug·high apps/web/src/career/ExportDeletionPage.tsx:71-75 clearPrivate 未重置 revision/revisionState(49-50)、verifyMessage(65)、lastExportRequest(66)、deletedAnnounced(69)，readRevision 依赖 [client,scopeController](94) 不含 scope.scope.generation，CareerPage.tsx:215 挂载无 key 不重挂载，291 行 forbidden/scope-changed 分支只渲染消息无恢复按钮（'ready' 只能经 193-202 的用户按钮复位，而该按钮在 291 分支内不可见）——残留 deletedAnnounced=true 会吞掉新空间首次删除的 onCareerDeleted+verifyOldGrants(214-217)，lastExportRequest 跨空间重放旧回执查询 fixHint=clearPrivate 补齐上述 state/refs 重置、readRevision 副作用加 generation 依赖、forbidden/scope-changed 分支提供「重新读取」按钮。

[VALID] bug·high packages/api-client/src/career.ts:1255-1257 open()/list()/changes() 以 `as` 强转绕过 decodeCareerView/decodeCareerChangeSet（career-core contracts.ts:41,46 已导出），client.ts:357 直接以 createCareerApi 为公共入口，10+ 页面直调 client.career.open()（MaterialPage:142 将未校验 revision 写入状态并作为写请求 expectedRevision 来源），desk 边界解码（desk.ts:78-83）对直调路径不生效 fixHint=三处改套 decodeCareerView/decodeCareerChangeSet（desk 边界重复解码幂等无害）。

[OUT_OF_SCOPE] security·high apps/web/src/knowledge/permissions.ts:96-101 canManage 门控跨面不一致属实：严格门控仅被 KnowledgeGraphPage.tsx:90 消费；KnowledgeDocumentsPage.tsx:3159 用 canUploadKnowledgeDocuments 设 canContribute，:4234/:4371 传 canManage={canContribute}（齿轮入口），:5326 以 role={canContribute?"admin":"viewer"} 渲染 KnowledgeSettingsPage；WikiPage.tsx:480 同款——共享 editor 在文档/Wiki 面仍能以 admin 角色打开 KB 设置。但 knowledge/documents/wiki 域属 Vue→React 迁移面，不在 #140 career 修复范围 fixHint(供迁移线)=两处消费点同步改用 computeKBPermissions(kb, me).canManage。

[OUT_OF_SCOPE] bug·high apps/web/src/faq/FAQPage.tsx:1153-1157 导出菜单嵌套反了属实：同文件新建菜单为正确顺序 Tooltip>Dropdown，导出菜单却是 Dropdown>Tooltip>Button；tdesign-react 1.18.3 源码验证机制成立——Dropdown 把唯一子元素交 Popup（Dropdown.js:127），getTriggerNode 对 forwardRef 的 Tooltip cloneElement 注 ref（useTrigger.js:336-349），而 Tooltip 的 imperative handle 是 {...popupRef.current}（Tooltip.js:106-108），Popup handle 为 {getPopupElement,...} 纯对象非 DOM（Popup.js:342-352），getTriggerElement 的 instanceof Element 判定失败返回 null（useTrigger.js:92-98），事件绑定 effect `if (!element) return`（useTrigger.js:174-175）→ 点击无法展开。faq 域属迁移面非 #140 fixHint(供迁移线)=改为 Tooltip 在外、aria-label/title 放回 Button。

[OUT_OF_SCOPE] bug·high apps/web/src/integrations/views-integrations-u.css:824-877 配合 packages/views/src/integrations/page.tsx:669,674 属实：.wk-vi-24（place-items:center 不拉伸）与 .wk-vi-28 无任何 iframe 尺寸规则，全 integrations 域 CSS 仅 integrations-u.css:303 `.wk-epm-13 iframe` 一处（ocr3-007 只修了 EmbedPreviewModal）→ 嵌入向导 step 6 预览 iframe 以默认 300x150 渲染。packages/views 与 integrations 迁移面非 #140 fixHint(供迁移线)=补 `.wk-vi-24 iframe, .wk-vi-28 iframe { width:100%; height:100%; border:0 }`。

[OUT_OF_SCOPE] bug·high apps/web/src/shared/wk-legacy.tsx:166-188 WkSheet open effect 未等 mounted 属实：guard 仅 `if (!open) return`(166)、deps [open](188)，mounted(125-126) 存在但未参与；渲染门控 226 行使首 commit 渲染 inline aside、setMounted 后才 portal，effect 在首 commit 执行聚焦的节点随即被卸载、焦点回落 body，Esc 分支(172) contains 判定恒不命中关不掉、Tab 陷阱失效；TenantAuditDrawer.tsx:108 有 `if (!open || !mounted) return` 先例；真实消费方 KnowledgeDocumentDetailPage.tsx:316,334 均 `<Sheet open ...>` 挂载即命中。shared 迁移兼容层非 #140 fixHint(供迁移线)=guard 加 `|| !mounted` 并将 mounted 纳入 deps。

## 二、用户点名重点项（报告 bug·low，按指示补核）

[VALID] bug·low internal/modules/career/material.go:580 career_materials.status 确认后仍报 draft 属实：ConfirmMaterial 成功事务将行状态写回 MaterialStatusDraft(580)，confirmed 只存在于回执(563)与 version 行，MaterialStatusConfirmed 枚举永不落库；Material() 视图(761 row.Status)与导出归档 materials[].Status（career_export.go Status: mat.Status）对已确认版本材料恒报 draft；apps/web/src/career/ExportDeletionPage.test.tsx:32 夹具期望 status:'confirmed' 与服务端行为相悖（夹具为 mock 故测试仍绿，但回执契约与视图/导出契约分叉属实） fixHint=确认成功落 status=MaterialStatusConfirmed（下次 EditMaterial:416 回落 draft），或删除死枚举并统一以 versionCount 表达确认态、同步修夹具。

## 三、#140 实现面内触及核心红线的 [bug·medium]

[VALID] bug·medium internal/modules/career/application.go:352-361 删除/外呼交错窗口遗留孤儿 Workbench 投影属实：Ensure 成功后 updateApplicationLink 返 ErrApplicationNotFound 仅返回 OutcomeUnknownError，无投影回收补偿（全文件无 applicationTaskRemover/RemoveCareerApplicationTaskProjections 引用）；DeleteCareer finalize 的防御清扫仅重清 Career 行、不重跑 remove_workbench_tasks 步骤（career_export.go:1021,1046-1047,1137）→ 残留引用已删申请的任务投影，破坏 deleted 回执真实性（导出/删除撤销访问红线邻接） fixHint=updateApplicationLink 返 NotFound 的 ready 分支经删除端口补偿回收本次投影，或 finalize 持锁清扫后重跑 RemoveCareerApplicationTaskProjections。

[VALID] bug·medium internal/modules/career/submission.go:169-193 并发双确认以 500 逃逸属实：isReceiptRaceError（office.go:836 文本回退含 "unique constraint"）判 ambiguous 后仅按 requestID 重放（不同 requestID 必未命中），随后 return txErr 裸 unique 错误穿透公共边界；application.go:324-329 有 occupied 复查返回类型化冲突的先例，submission 侧缺失（幂等 requestID 红线：类型化冲突） fixHint=重放未命中时按 tenant/user/application_id 复查并返回 ErrSubmissionAlreadyConfirmed。

[VALID] bug·medium internal/modules/workbench/service/workbench/application_task.go:268-272 SQLite race 分类器漏判属实：marker 仅两个 PG 索引名与锁类文本；internal/container/container.go:1363-1367 gorm.Open 未启用 TranslateError，SQLite 唯一冲突消息 "UNIQUE constraint failed: agent_runs.tenant_id, ..." 不含索引名不匹配任何 marker，ensureWithRetry:116 将原始驱动错误直接抛给 Career → 归入 OutcomeUnknown、申请停 linking（可重放自愈但并发幂等重放契约在 SQLite 部署失效；PG 不受影响）（幂等 requestID 红线） fixHint=为两索引补精确到列清单的 SQLite marker，或主连接启用 gorm.Config.TranslateError。

[VALID] bug·medium apps/web/src/career/RulePage.tsx:229 查询回执 mismatch 置 phase 'unknown' 属实（原文可查），与 send 路径及 reconciliation.tsx（ocr2-082）确立的「确定性协议错误不得路由进回执恢复」红线相悖，形成 unknown→查询→mismatch→unknown 循环 fixHint=置 error、清 attempt、提示更换新请求编号。

[VALID] bug·medium apps/web/src/career/ExportDeletionPage.tsx:167-173 lookupExportReceipt catch 未识别 acceptExport 抛出的 ReceiptMismatchError 属实：catch 仅 forbidden 检查后直接 setExportPhase('unknown')；同文件 runExport:152 有 instanceof 先例 fixHint=catch 补 instanceof 分支：置 error、清 exportAttempt、提示更换请求编号。

[VALID] bug·medium apps/web/src/career/ExportDeletionPage.tsx:272-278 lookupDeletionReceipt 同款属实：acceptDeletion(206) 在 try 内可抛 ReceiptMismatchError，catch 仅 forbidden 后置 unknown；runDeletion:256 有 instanceof 先例 fixHint=同上（deletionAttempt 清除+置 error）。

[VALID] bug·medium apps/web/src/career/ExportDeletionPage.tsx:222-224,331-335 status 'deleting' 无恢复入口属实：acceptDeletion 将 deleting 置 phase 'error'，「查询删除回执」按钮仅 deletionPhase==='unknown' 渲染(331)、「用原请求编号重试删除」仅 status==='partial' 渲染(335)，文案「可查询删除回执查看最新状态」无对应按钮（删除流程红线邻接） fixHint=deleting 状态且 deletionAttempt 存在时补渲染查询回执按钮。

[VALID] bug·medium apps/web/src/career/PreparationPage.tsx:219-226 lookupReceipt catch 未识别 acceptReceipt 抛出的 ReceiptMismatchError 属实：catch 仅 forbidden/invalid_request 后置 unknown；runGenerate:207 有 instanceof 先例 fixHint=catch 补 instanceof 分支：置 error、清 attempt。

[VALID] bug·medium apps/web/src/career/SearchPage.tsx:256-258 importResult 未校验回执请求编号属实：imported 未与本次 requestId 比对直接存为该行 receipt 并渲染证据链接，可能把其他请求的证据当本次导入结果展示；同页 send():162 与 OpportunityPage:92 均校验（回执身份校验红线） fixHint=补 `imported.requestId !== requestId` 时置确定错误并放弃本次结果。

[VALID] bug·medium apps/web/src/career/OpportunityPage.tsx:100,144-160 内联确定码清单遗漏 'not_found' 属实：importAttempt/importURLAttempt 未匹配 not_found 时直接 setState('unknown')「网络未能确认保存结果」，对已被删除引用（空间删除可移除引用对象，服务端 not_found 恒 404 确定）陷入无法终止的恢复循环；protocol.ts:37 基座与 ProgressPage 清单均含 not_found fixHint=两处内联清单补 'not_found' 并给明确文案（或改用 endpointDefiniteCodes 叠加模式）。

[FALSE_POSITIVE] bug·medium apps/web/src/career/protocol.ts:37 「基座清单缺 revision_conflict 会被判 uncertain 路由进回执恢复」不成立：errorDetails 完整保留 ApiError 的 status（protocol.ts:28-30；errorFromResult errors.ts:49-61 恒携带 HTTP status），服务端 revision_conflict 恒映射 409（internal/modules/career/handler.go:92-94），isUncertainWrite 第 38 行 status 分支 `409>=500 || 409<400` 返回 false 已判为确定失败，唯一直接消费方 OpportunityPage:215 走该路径不会进 unknown；将 revision_conflict 显式加入清单属一致性加固而非修 bug。

[VALID] bug·medium packages/api-client/src/career.ts:938-939 非 draft 快照 digest 校验缺口属实：digest 良构仅在 opportunityId/snapshotId 存在时要求，仅携带 snapshotSha256:"garbage" 的畸形负载通过两分支并经返回值 spread 直送 UI，与 936 行注释「whatever it does carry must still be a well-formed digest」相悖（快照引用完整性；红线关联弱，属防御性解码严格性缺口） fixHint=快照任一字段（含 snapshotSha256 自身）存在即要求 digest 良构。

## 统计

- 高危 10 条（security·high 1 + bug·high 9）：VALID 6（F1-F6，全部位于 #140 实现面）；OUT_OF_SCOPE 4（F7-F10，代码全部属实但属 Vue→React 迁移旧面）；FALSE_POSITIVE 0。
- 点名重点项 1 条（material status 契约，报告定级 bug·low）：VALID 1。
- #140 面触及红线 medium 12 条核查：VALID 11；FALSE_POSITIVE 1（protocol.ts 基座清单——status 分支兜底推翻影响主张）。
- 合计核查 23 条：VALID 18 / FALSE_POSITIVE 1 / OUT_OF_SCOPE 4。
- 结论：#140 实现面内需修复的高危共 6 条（F1 导出删除不对称、F2 小程序显式继续门禁失效、F3/F4 回执不匹配误路由 unknown、F5 导出删除页作用域清理不完整、F6 api-client 解码绕过），外加 11 条红线 medium；报告无 #140 面内的高危误报。
