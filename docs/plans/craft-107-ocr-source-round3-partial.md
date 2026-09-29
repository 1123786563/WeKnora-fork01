Review partially complete: 5 finding(s); 9 of 26 selected item(s) failed.

─── internal/container/craft_web_build.go:377-377 ───
[bug · high] 生产环境中 W01 构建证据将永远无法建立：本行无条件传入 nil，而 CraftWebBuildEvidence 的首个检查就是 observedExitCode ==
nil → ErrConflict，因此后续 `evidence.BuildRan = build.BuildRan; evidence.BuildExitCode =
build.BuildExitCode`（384-386 行）已成为不可达死代码。同时，每个真实运行的合法 build-log 都会落入下面的 Warn 分支（"refusing build log
... receipt is unavailable"），把正常运行当告警噪音。全代码库检索确认：新增的 CraftWebBuildReceiptRepository（含迁移
000138/000217 建立的 craft_web_build_receipts 表）没有任何生产写入方或读取方——RecordTerminal/Read
仅在测试中被调用，本函数签名也未提供任何回执读取通道。若 Spec #107 要求 "T14 live acceptance evidence is recorded in the durable
ledger"，该 durable ledger 目前永远不会被写入。建议：为 craftWebBuildEvidenceSource 增加一个服务端回执读取参数（按 run 身份查询回执并以
receipt.ExitCode 作为 observedExitCode），并在执行协调器观测到构建进程终态时调用 RecordTerminal 写入回执；若回执接线属于后续
PR，至少应移除/隔离这段恒为死的赋值路径，并将"回执体系未接线"与"回执冲突"在告警级别上区分开，避免每个合法构建都产生告警。

- 		build, err := CraftWebBuildEvidence(log, pin, nil)
+ func craftWebBuildEvidenceSource(
+ 	inner service.ArtifactEvidenceSource,
+ 	readLog func(context.Context, craft.Task) ([]byte, error),
+ 	readReceipt func(context.Context, craft.Task) (*int, error), // server-owned exit code from the durable receipt ledger
+ 	pin CraftWebToolchainPin,
+ ) service.ArtifactEvidenceSource {
+ 	...
+ 	exit, rerr := readReceipt(ctx, task)
+ 	if rerr != nil {
+ 		// warn once: evidence stays unobserved (fail-closed)
+ 	}
+ 	build, err := CraftWebBuildEvidence(log, pin, exit)


─── internal/application/repository/craft_web_build_receipt.go:121-121 ───
[bug · high] 该回执仓库（连同迁移 000138/000217、不可变触发器）在本变更集中完全没有生产接线：全代码库内
NewCraftWebBuildReceiptRepository/RecordTerminal/Read 只出现在 *_test.go
中，没有任何执行协调器写入服务器观测的执行结果，也没有任何证据路径读取它。与之对应，internal/container/craft_web_build.go 的
craftWebBuildEvidenceSource 现在固定传入 nil 回执并 fail-closed——两者合起来意味着 W01 构建证据在合并后永久 not_run，而 durable
ledger 永远为空。这与任务描述中 "T14 live acceptance evidence is recorded in the durable ledger"
的验收口径不符。请在本次变更中一并接线（docker exec 终态处 RecordTerminal，证据源按 run 身份 Read 并以回执 ExitCode 传入
CraftWebBuildEvidence），或明确拆分后续 PR 并在本文件标注接线缺口，避免发布一个只写不读、实际无人写入的表和仓库。

- func (r *CraftWebBuildReceiptRepository) RecordTerminal(ctx context.Context, receipt CraftWebBuildReceipt) (CraftWebBuildReceipt, error) {
+ // 执行协调器侧（示意）：在观测到构建进程终态时写入回执
+ // stored, err := receipts.RecordTerminal(ctx, CraftWebBuildReceipt{...ExitCode: observedExit, ProcessState: state...})
+ // 证据源侧：以 task/run 身份 Read 回执，用 receipt.ExitCode 作为 observedExitCode 传入 CraftWebBuildEvidence


─── apps/web/src/features/craft/routes.tsx:1077-1081 ───
[style · low] 此渲染块形成三层嵌套条件表达式（`budgetPauseView !== null` → `viewed` → prop 内 `budgetExtensionAction
!== null && !budgetExtensionBusy ? ... : undefined`），违反前端规范的嵌套三元禁令；且 onRequestExtension 内的多行内联箭头函数降低
JSX 可读性。建议将点击处理提取为 useCallback 备忘的 handler（内部用 ref/投影 action 自行守卫），prop 处仅保留单层三元或直接传值，必要时将 viewed
分支提取为局部变量/子渲染函数。

-               onRequestExtension={budgetExtensionAction !== null && !budgetExtensionBusy ? (runId) => {
+ // 组件体中提取：
+ const handleBudgetExtensionRequest = useCallback(
+   (runId: string) => {
+     if (budgetExtensionAction === null || budgetExtensionBusy) return;
-                 void requestBudgetExtension(runId, budgetExtensionAction).catch((error: unknown) => {
+     void requestBudgetExtension(runId, budgetExtensionAction).catch((error: unknown) => {
-                   setSyncError(error instanceof Error ? error.message : String(error));
+       setSyncError(error instanceof Error ? error.message : String(error));
-                 });
+     });
-               } : undefined}
+   },
+   [budgetExtensionAction, budgetExtensionBusy, requestBudgetExtension],
+ );
+ 
+ // JSX 中仅保留单层条件：
+ onRequestExtension={budgetExtensionAction === null || budgetExtensionBusy ? undefined : handleBudgetExtensionRequest}


─── internal/modules/craftegress/adapter.go:227-228 ───
[bug · medium] 撕裂路径的 journal.Resolve 错误被 `_ =` 静默丢弃。旧代码此调用恒为
`definitive=false`（保持停靠），失败无后果；但新语义下撕裂的非 5xx 响应（如撕裂的 200）会以 `definitive=true`
落地——此时持久化失败会把停靠身份永久搁浅，后续同指纹重试永远复用该 ID 并撞网关 409，这正是下方完整路径（第 244-249 行）注释里声明"that deadlock must at
least be VISIBLE"并用 ErrorWithFields 记录的同一后果。两条路径的可见性要求应一致：definitive 停靠解析失败至少要留下日志。

  			definitive := responseBodyReadOutcomeIsDefinitive(resp.StatusCode)
- 			_ = a.journal.Resolve(attemptID, digest, resp.StatusCode, definitive)
+ 			if err := a.journal.Resolve(attemptID, digest, resp.StatusCode, definitive); err != nil {
+ 				// A definitive resolve that failed to persist strands the parked
+ 				// identity (same-fingerprint retries reuse it and hit the
+ 				// gateway's 409 forever) — the deadlock must at least be visible.
+ 				logger.ErrorWithFields(r.Context(), err, map[string]any{
+ 					"craft_attempt_id": attemptID, "definitive": definitive, "gateway_status": resp.StatusCode,
+ 				})
+ 			}


─── internal/modules/craftegress/adapter.go:342-344 ───
[maintainability · low] `status != http.StatusBadGateway`（502）是死条件：502 属 5xx，已被首个条件 `status <
http.StatusInternalServerError` 排除，永远为真。同时上方注释"resolves non-5xx responses except 409/502"自相矛盾（502 并非
non-5xx），容易让读者误以为存在某种 4xx 语义差异。建议删去冗余子句并把注释改为与实际策略一致的表述（409 与全部 5xx 停靠）。

  func responseBodyReadOutcomeIsDefinitive(status int) bool {
- 	return status < http.StatusInternalServerError && status != http.StatusConflict && status != http.StatusBadGateway
+ 	return status < http.StatusInternalServerError && status != http.StatusConflict
  }


LLM retry report summary: 15 of 140 requests affected -- 5 requests failed, 10 requests recovered after retry

Review planning (2 requests):
- internal/container/craft_interaction.go,internal/modules/craft/input_code.go: timed out -> failed
- internal/application/repository/craft_stop_intent.go,internal/application/service/craft_budget.go,internal/application/service/craft_control.go,internal/handler/session/craft_budget_pause.go: rate limited (HTTP 429) -> succeeded

Core review (12 requests):
- internal/application/repository/craft_run_capture_promotion.go,internal/container/craft_run_capture_promotion.go,internal/container/craft_run_capture_wiring.go: timed out -> failed
- internal/application/repository/craft_stop_intent.go,internal/application/service/craft_budget.go,internal/application/service/craft_control.go,internal/handler/session/craft_budget_pause.go: timed out -> failed
- internal/container/craft_interaction.go,internal/modules/craft/input_code.go: timed out -> failed
- apps/web/src/features/craft/routes.tsx,internal/modules/craft/web_contracts.go,packages/api-client/src/craft/index.ts,packages/api-client/src/index.ts: network error -> succeeded
- apps/web/src/features/craft/routes.tsx,internal/modules/craft/web_contracts.go,packages/api-client/src/craft/index.ts,packages/api-client/src/index.ts: provider error (HTTP 500) -> succeeded
- ... and 7 more

Comment filtering (1 request):
- internal/application/service/craft_artifacts.go,internal/application/service/craft_export_consent.go,internal/modules/craft/archive.go,internal/modules/craftegress/adapter.go: timed out -> failed

Per-attempt detail: --format json (retry_report).
