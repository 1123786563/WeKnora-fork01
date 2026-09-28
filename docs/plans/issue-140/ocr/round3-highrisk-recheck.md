# OCR round-3-resume 最高危项人工复查（round-4 前置核对）

- 复查日期：2026-09-28
- 复查基线：worktree HEAD = 76df0cee0（`git diff --stat 77917de10 76df0cee0 -- internal apps packages` 为空，即集成台账之后仅 docs 提交，四域修复代码未被回退）
- 复核范围：ocr-round-3-resume.md 中属于 #140 实现面的全部 critical + high（共 7 条：critical 1 / high 6，与台账口径一致；apps/web/src/chat、packages/views/src 等其余 15 条 high 属 Vue→React 迁移面或非 career 域，不在本次范围）
- 方法：逐条以 `git show HEAD:<path>` 实证当前代码形态，按语义定位（行号以 HEAD 实测为准），不以修复提交说明为准

## 逐条结论

1. `[RESOLVED] critical apps/web/src/settings/TenantMembersPanel.tsx:911 三处裸 <Icon>（import 已改名 TIcon）导致 TS2304 编译断裂 —— 消除提交 24821e5b1（ocr3-001）`。HEAD 证据：严格正则 `(^|[^A-Za-z])<Icon[ />]` 在 HEAD 版文件 0 匹配；import 为 `import { Icon as TIcon } from 'tdesign-icons-react'`（L8），全文件 15 处均为 `<TIcon>`。
2. `[RESOLVED] high apps/miniprogram/src/career/discovery.config.ts:1-2 缺 enableShareAppMessage: true，useShareAppMessage 回调不挂载、分享导入链路不可达 —— 消除提交 4f1c765ea（ocr3-004）`。HEAD 证据：config 第 4 行即 `enableShareAppMessage: true`，附 ocr3-004 注释。
3. `[RESOLVED] high apps/miniprogram/src/career/application-material.tsx:212-213 pendingMaterialWrite/pendingApplication/pendingSubmission 存续期四类写入按钮未禁用，二次歧义失败覆盖旧 intent 永久丢失对账凭据 —— 消除提交 4f1c765ea（ocr3-003，配合 ocr3-030/031 链路硬守卫）`。HEAD 证据：创建申请 L149 `disabled={pendingApplication !== null}`；保存草稿 L223 与确认新版本 L234 `disabled={pendingMaterial !== null}`；记录投递 L309 `disabled={pendingSubmission !== null}`；services/career.ts 的 writeRecoverable 已收敛到 recoverableWrite（stamp 守卫 + 同 kind 未对账 intent 硬守卫 + abandon 出口）。
4. `[RESOLVED] security·high apps/miniprogram/src/adapters/career-platform.ts:235 openExportedDocument/taroDownload 绕过作用域守卫（无 stamp/abort/isCurrent/途中限额），登出或切空间后旧账号材料仍可落本机并打开 —— 消除提交 4f1c765ea（ocr3-002）`。HEAD 证据：openExportedDocument 现具备 stamp 捕获 + 发起前 isCurrent 预检、controller 注册并 abort 联动（stopSubscriptions/abortAll 中断在途下载）、下载后/复制后/打开前共 3 次 isCurrent 复验、onProgressUpdate 超 MAX_EXPORT_BYTES 中途中止、grant.size 下载前预检、finally release(controller)——对齐 T06 files.ts 三层守卫并附回归测试（OCR3-002 F4/F5/F6）。
5. `[RESOLVED] high internal/modules/career/application.go:200-202 合并前评估 E.OpportunityID 不经 merge-chain 解析直接与快照归属比较，合并后旧评估创建申请永久 400 且无自愈 —— 消除提交 0d50acb4a（ocr3-016）`。HEAD 双修复证据：application.go:205 `canonicalOpportunityID(tx, scope, evaluation.OpportunityID) != snapshot.OpportunityID`（旧版本搁浅行自愈）；reconciliation.go 合并事务新增 evaluationRecord.opportunity_id 迁移（与 observations/snapshots 同模式，含注释）；回归测试 reconciliation_test.go:446（ocr3-016）。
6. `[RESOLVED] high internal/modules/career/search_rule.go:560-563 规则 period requestID 固定 + 指纹含 ExpectedRevision，crash 搁浅 claiming 行后 ErrIdempotencyConflict 使该规则该 period 永久卡死并连带 TriggerDueRules 整批失败 —— 消除提交 0d50acb4a（ocr3-017）`。HEAD 证据：search_rule.go:572-578 捕获 ErrIdempotencyConflict 后改调 `searchOnceUnderStoredFingerprint(ctx, s, requestID)`；该函数在 search_once.go:293 有完整实现（按已存 searchRecord 的存储指纹 claim 接管、终态直接回放、in-flight awaitSearchTerminal、不重复消耗 admission）；含搁浅 claim 接管/终态回放回归测试。
7. `[RESOLVED] high packages/api-client/src/career.ts:1326 全部写方法本地预检抛裸 TypeError，被页面侧 isUncertainWrite 判 unknown 误入回执恢复（红线违规）—— 消除提交 24821e5b1（ocr3-018）`。HEAD 证据：原 finding 行（现 L1326）已为 `throw new CareerValidationError('application request identifiers and revision must be valid')`；全文件 89 处 `throw new CareerValidationError`；剩余 `throw new TypeError` 全部位于响应解码器（decode* 函数），属 ocr3-018 有意保留（网络响应畸形=不确定，应进恢复链），与本地预检语义相反，符合红线。

## 统计

- 7 resolved / 0 open（critical 1/1、high 6/6 全部消除）
- 消除提交分布：24821e5b1 两项（#1 critical、#7）；4f1c765ea 三项（#2、#3、#4）；0d50acb4a 两项（#5、#6）

## 备注

- 本复查只覆盖 critical/high；round-3 域内其余 44 medium / 92 low 未逐条复核（其中幂等红线相关 medium 已在 4f1c765ea/0d50acb4a 声明修复并附测试，如 ocr3-026/029/030/031/032/128/129）。
- 台账 77917de10 之后到 HEAD 仅 3 个 docs 提交（5fd1a1de3、34221bbb8、76df0cee0），无代码改动，上述消除状态在当前 HEAD 成立。
