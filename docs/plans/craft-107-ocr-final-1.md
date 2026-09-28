Review complete: 73 finding(s) across 120 selected item(s).

─── apps/desktop/vite.config.ts:69-69 ───
[bug · high] desktop 别名与 apps/web/vite.config.ts 未保持 lockstep，desktop 构建将断裂。本文件第 56-58 行注释自述契约：craft
子路径别名必须与 apps/web/vite.config.ts 同步，否则裸前缀别名 '@weknora/views'（第 70 行）会把深层导入重写为 '<index.ts>/craft/xxx'
导致 ENOTDIR。本次 apps/web/vite.config.ts 新增了 6 条别名，desktop 只补了 web-promotion 与 access
两条，缺少：'@weknora/views/craft/usage'、'@weknora/views/craft/input-expand'、'@weknora/views/craft/workben
ch-edit'（三者均被共享 web 入口 apps/desktop/src/main.tsx:11 动态导入的
apps/web/src/features/craft/routes.tsx:34-37 直接引用）以及 '@weknora/domain/craft/usage'（被
packages/views/src/craft/usage.tsx:25 引用，会被裸 '@weknora/domain'（第 38 行）前缀重写为
'<query-key.ts>/craft/usage'）。这四个路径在 desktop dev/build 中均无法解析。

        '@weknora/views/craft/access': fileURLToPath(new URL('../../packages/views/src/craft/access.tsx', import.meta.url)),
+       '@weknora/views/craft/usage': fileURLToPath(new URL('../../packages/views/src/craft/usage.tsx', import.meta.url)),
+       '@weknora/views/craft/input-expand': fileURLToPath(new URL('../../packages/views/src/craft/input-expand.tsx', import.meta.url)),
+       '@weknora/views/craft/workbench-edit': fileURLToPath(new URL('../../packages/views/src/craft/workbench-edit.tsx', import.meta.url)),


─── apps/web/e2e/craft-stack.sh:62-62 ───
[test · medium] WEB_ORIGIN 切换为 http://localhost:$PORT_VITE，但第 371 行 vite 仍以 --host 127.0.0.1 启动（仅监听
IPv4 回环）。localhost 在多数系统同时解析为 ::1 与 127.0.0.1，且解析顺序常偏好 IPv6（Node ≥17 / 部分 Linux 环境）；一旦解析命中 ::1，页面加载与
wait_http 探测都会 connection refused，e2e 栈在跨环境（尤其 CI Linux）不可移植。建议让监听地址与 WEB_ORIGIN 对齐（如 --host
localhost 由 vite 做双栈回环绑定，或显式双栈监听），并验证后再依赖该主机名契约。



─── apps/web/src/features/craft/routes.tsx:1071-1071 ───
[bug · medium] 扩容入口用组件级 canWrite（仅 ownerId === currentMeId 的 owner）门控 onRequestExtension，而
budgetPauseView.canExtend 来自服务端对 owner 或租户账单管理员的投影（craft_budget_pause.go 的 MayExtendBudget）。非 owner
的账单管理员会拿到 canExtend=true，面板随之渲染 pauseCanExtend 文案与"申请增加预算"按钮，但因 onRequestExtension=undefined
按钮永久禁用（usage.tsx 中 disabled={!onRequestExtension}）——服务器授权与客户端入口不一致，形成功能缺口。requestBudgetExtension
本就依赖服务端复查 actor（注释亦自述），这里应以服务端投影为门控而非 owner-only 的 canWrite。

-             onRequestExtension={canWrite ? (runId) => {
+             onRequestExtension={budgetPauseView.canExtend ? (runId) => {


─── apps/web/src/features/craft/routes.tsx:544-544 ───
[bug · medium] 每次点击生成全新幂等键（'extend-' + crypto.randomUUID() 在回调内创建），且按钮无 in-flight 禁用。服务端 Extend
语义是"按键 exactly-once"（同键重放幂等、异键即一次新扩容，见 craft_budget.go
Extend/ExtendTaskLimit），因此：网络超时但服务端已落账后用户重试、或快速双击，都会携带新键造成重复扩容——每次即 +10 calls / +10,000,000
micro-credits 的实际资源授予，幂等键对它声称保护的场景完全失效。仓库既有先例 apps/web/src/commercial/TaskBudget.tsx:38-52,116
正是为"flaky connection 永不双倍预算"用 useRef 持有同一键直至确认成功、并以 busy 禁用按钮。建议按 runId 在 ref
中持有键（成功或确定性失败后才重置），并在请求期间禁用按钮。

-         key: 'extend-' + crypto.randomUUID(),
+         key: extensionKeyRef.current ?? (extensionKeyRef.current = 'extend-' + crypto.randomUUID()),


─── apps/web/src/features/craft/routes.tsx:545-546 ───
[maintainability · medium] 扩容量子（10 次调用 / 10,000,000
micro-credits）是直接决定计费资源授予量的业务数字，硬编码在客户端且注释自述为"部署默认值"——与部署侧策略形成双源：一旦服务端/部署调整默认量子，客户端静默漂移，无编译期或运行期校验。建
议由服务端在 budget/pause 视图中投影建议量子（服务端权威、客户端只消费），或至少提取为带部署契约注释的共享常量，避免散落在装配层。



─── apps/web/src/features/craft/routes.tsx:514-514 ───
[maintainability · low] 该 effect 依赖 activeRun 的对象标识而非 (runId, waitReason) 原始值：toActiveRun 在每次
setWorkbenchInfo（初始加载与 refreshWorkbench）都产生新对象字面量，因此任何 workbenchInfo 刷新（即便 runId/waitReason
完全不变）都会重跑 effect——先 setBudgetPauseView(null)（面板卸载闪烁）再重复请求 budgetPause，与注释声称的"fetched ONCE per run
identity"相悖。当前两个 refreshWorkbench 调用点恰好都伴随 run 状态变化所以未显性触发，但这是留给后续调用者的地雷。建议依赖原始值并在 effect 内取
activeRun。

-   }, [sessionId, activeRun, craftApi, scopeController]);
+   }, [sessionId, activeRun?.id, activeRun?.waitReason, craftApi, scopeController]);


─── apps/web/src/features/craft/routes.tsx:685-690 ───
[style · low] 此处为三层嵌套三元（同 session 且未存在 ? 追加 : 同 session ? 保留 : 重置），同文件 budgetPauseView
JSX（budgetPauseView !== null ? viewed ? … : … : null）同样是嵌套三元，均违反检查清单"禁止嵌套三元"且降低可读性。建议提取纯函数（如
mergeAssociatedInput(prior, sessionId, input) 用早退分支）或拆分为具名中间变量/渲染辅助函数。



─── apps/web/src/features/craft/routes.tsx:315-319 ───
[maintainability · low] catch 将网络故障、5xx 等瞬态异常与 403/404 权限拒绝统一映射为"无权限或已删除"，对排障形成误导：transport 抛出的
ApiError 携带 status/code，完全可区分"服务暂不可用，请重试"与"权限拒绝"（T10
设计仅约束服务端拒绝不带原因，不要求客户端混淆瞬态错误）。另外这些文案硬编码中文，绕过了该文件其余面板使用的 locale（CraftLocale
zh/en）机制。建议至少为瞬态错误给出独立文案，并考虑接入 locale。



─── apps/web/src/features/craft/routes.tsx:523-527 ───
[maintainability · low] toActiveRun 是无组件状态依赖的纯投影函数，却定义在组件体中部、位于其首个消费者（初始加载 effect 第 457 行的
toActiveRun(view)）之后，且 refreshWorkbench 的 useCallback 依赖数组 [craftApi, scopeController] 未包含它——仓库没有任何
eslint/exhaustive-deps 工具兜底，这属于纯靠人工记忆维持的脆弱写法（当前因函数纯净而无功能影响，但任何人给 toActiveRun 加入对 state/props
的引用都会静默产生陈旧闭包）。建议与 parseCraftRoute/sha256Hex 一样提升为模块级函数：既消除 use-before-define，也让 useCallback
依赖数组诚实，无需将组件内函数塞进 deps。

-   const toActiveRun = (view: CraftWorkspaceView): { id: string; status: string; waitReason: string } | null =>
-     view.active_run === null
+ // 移至模块顶层（与 parseCraftRoute 同级）：
+ function toActiveRun(view: CraftWorkspaceView): { id: string; status: string; waitReason: string } | null {
+   return view.active_run === null
-       ? null
+     ? null
-       : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason };
+     : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason };
-   const refreshWorkbench = useCallback(async (targetSessionId: string): Promise<void> => {
+ }
+ // 组件内删除原 const toActiveRun = ... 定义，refreshWorkbench 依赖数组保持 [craftApi, scopeController] 即为完整。


─── cmd/craft-egress-adapter/main.go:104-108 ───
[maintainability · low] serveFailed 路径的 os.Exit(1) 不执行 defer,与本文件中"serve 错误回流主协程以保住 deferred
adapter.Close()"的注释自相矛盾——该路径下 journal 文件句柄未被显式关闭。虽然每条记录写入时已 fsync、数据无实际丢失,但既然注释明确以此为目的设计,就应在退出前显式关闭。

  	if serveFailed {
  		// Exit NON-zero: K8s onFailure restarts only non-zero exits, and
  		// alerting treats zero as healthy — a bind failure must be visible.
+ 		// os.Exit 不执行 defer:退出前显式关闭 journal 句柄。
+ 		_ = adapter.Close()
  		os.Exit(1)
  	}


─── cmd/craft-egress-adapter/main.go:99-99 ───
[other · low] 优雅关停预算固定 10 秒,而单次转发预算上限为 CRAFT_EGRESS_FORWARD_TIMEOUT(默认 6 分钟)。正常 SIGTERM 下,超过 10
秒的在途生成会在 Shutdown 超时后随进程退出被硬切,沙箱侧收到中断、该 attempt 驻留为 unresolved(协议上可容忍,重启后 replay
保持驻留)。但关停预算与转发预算相差两个数量级,建议让排水预算随 forwardTimeout 派生(如 forwardTimeout+关停余量)或至少在部署文档中明确该取舍,避免运维误以为 10
秒足以排空在途请求。

- 	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
+ 	drainBudget := forwardTimeout + 30*time.Second // 与转发预算同量级,排空在途生成
+ 	shutdownCtx, cancel := context.WithTimeout(context.Background(), drainBudget)


─── cmd/craft-egress-adapter/main.go:79-79 ───
[security · low] http.Server 仅设置 ReadHeaderTimeout,未设置 ReadTimeout 与 IdleTimeout;readBounded 读取 body
无服务端截止时间,慢速/恶意的本地客户端可无限期占用 handler 协程(信号关停只能兜底正常退出场景)。WriteTimeout 需保持不设(6
分钟量级的长转发会被它误杀),但请求体读取与空闲连接超时应补齐。

- 	server := &http.Server{Addr: listen, Handler: adapter, ReadHeaderTimeout: 10 * time.Second}
+ 	server := &http.Server{
+ 		Addr:              listen,
+ 		Handler:           adapter,
+ 		ReadHeaderTimeout: 10 * time.Second,
+ 		ReadTimeout:       2 * time.Minute, // 16MB body 的慢速读取上界
+ 		IdleTimeout:       5 * time.Minute,
+ 		// WriteTimeout 保持不设:转发预算可达 6 分钟,设置会误杀长生成
+ 	}


─── internal/application/repository/craft_docker_send_claim.go:242-242 ───
[maintainability · low] load() 同时服务于 outputless 协议的 Bind/Claim/RecordDockerExecEventPair
失败路径，但基础设施错误被统一包装为 normal-INPUT 域哨兵 ErrCraftDockerNormalInputUnavailable（"Docker normal input store
is unavailable"），outputless 流程的 DB 故障会被误诊为输入存储故障；对称地，normalInputExists 又用 dockerOutputDBError（output
域）包装。另外 dockerNormalInputDBError 用 %v 包装底层错误（对比 dockerOutputDBError 的 %w），调用方无法对底层 DB 错误做
errors.Is/As（如 context.DeadlineExceeded 的重试判定只能靠字符串）。建议为 send-claim 域引入独立哨兵并用 %w 保留原因链。

- 	return row, dockerNormalInputDBError(err)
+ func (r *CraftDockerSendClaimRepository) load(ctx context.Context, key CraftChargeStartKey) (craftDockerSendClaimRow, error) {
+ 	var row craftDockerSendClaimRow
+ 	err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error
+ 	if errors.Is(err, gorm.ErrRecordNotFound) {
+ 		return craftDockerSendClaimRow{}, craftDockerSendConflict("operation not found")
+ 	}
+ 	if err != nil {
+ 		return craftDockerSendClaimRow{}, fmt.Errorf("%w: %w", ErrCraftDockerSendClaimUnavailable, err)
+ 	}
+ 	return row, nil
+ }


─── internal/application/repository/craft_run_capture.go:264-270 ───
[bug · medium] ClaimForDrain 未检查 RowsAffected：并发场景下（回执已被 ticker 或另一 drain 认领/迁移，WHERE
state='pending' 命中 0 行）仍返回 nil error，且把调用方本地的 receipt.State 置为 "capturing"，伪装成认领成功。这是 drain/ticker
双路径并发重做同一回执（各自上传全新物理对象、输家泄漏为无引用 resource 行）的关键缺口之一——认领本应充当互斥，但 0 行命中被当成功使其失效。建议 0
行命中时返回可识别的冲突错误，让调用方跳过本轮（由占有方推进）。

- 		res := tx.Model(&craftRunCaptureRow{}).
- 			Where("tenant_id=? AND workspace_id=? AND run_id=? AND state = 'pending'", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).
- 			Updates(map[string]any{"state": "capturing", "last_error": "", "updated_at": time.Now()})
  		if res.Error != nil {
  			return res.Error
+ 		}
+ 		if res.RowsAffected == 0 {
+ 			// 已被其他路径认领或状态已迁移：不得伪装成功，否则并发重做的上传泄漏。
+ 			return fmt.Errorf("%w: capture receipt is not pending (owned by another drain)", craft.ErrConflict)
  		}
  		return nil


─── internal/application/repository/craft_run_capture.go:126-129 ───
[maintainability · low] s.ticks++ 为非原子自增并立即取模读取。当前唯一调用链是 Start 启动的单个 ticker goroutine（RecoverTick →
RecoverPendingTick），store 实例为 runner 专属，无现实竞争；但 RecoverPendingTick 是导出方法，任何未来的并发调用（重复
Start、管理端点手动触发、多 runner 共享 store）即构成数据竞争，且竞争下取模轮次错乱可使昂贵的全历史 JOIN 合成偏离预期节奏。建议改用
atomic.AddUint64/LoadUint64（一行改动）或明确注释单调用方约束。

  func (s *CraftRunCaptureStore) RecoverPendingTick(ctx context.Context, limit int) ([]CraftRunCapture, error) {
- 	s.ticks++
- 	return s.recoverPending(ctx, limit, s.ticks%craftCaptureSynthesisEveryNth == 1)
+ 	tick := atomic.AddUint64(&s.ticks, 1)
+ 	return s.recoverPending(ctx, limit, tick%craftCaptureSynthesisEveryNth == 1)
  }


─── internal/application/service/craft_budget.go:419-422 ───
[bug · high] restart 路径（journal 为 definitely_unstarted 时）对 commercial_reservations 的 DELETE
使用了不存在的列名：该表的键列是 `key`（ReservationRow 的 gorm tag 为 column:key，MarkReservationDispatchedInTx 的原生 SQL
也用 `key = ?`），`reservation_key` 是 commercial_budget_lot_allocations 的列。生成的 `DELETE ... WHERE
reservation_key = ?` 在 PostgreSQL 报 "column reservation_key does not exist"、在 SQLite 报 "no such
column"，整个重启事务必然失败——注释声称的 definitely_unstarted 干净重启永远不会成功，调用方每次重试同一 activityID
都收到数据库内部错误而非设计的重启语义。另外即便修正列名，直接物理删除已 dispatched 的预留行也不回退 Reserve 时累加的三处
held_micro（commercial_budget_accounts / commercial_task_budgets / commercial_budget_lots），随后
ReserveInTx 会再次累加 held，导致任务/账户可用额度被永久蚕食（dispatched 预留从不被 cancel/expiry 清零，Reconcile 查不到该行会直接
continue，held 再无回收路径）。建议改为先经 commercial 的释放/对账路径（如 reconcile 判定 provably-unstarted 后 release，回退 held
并清理 lot allocations），再在同一事务删除预留行。

- 			if err := tx.Where("tenant_id = ? AND reservation_key = ?", row.TenantID, callKey).
+ 			// 回退 dispatched 预留的 held 额度（account/task/lots + lot allocations）后
+ 			// 再删除预留行；至少先修正列名：
+ 			if err := tx.Where("tenant_id = ? AND key = ?", row.TenantID, callKey).
  				Delete(&repocommercial.ReservationRow{}).Error; err != nil {
  				return err
  			}


─── internal/application/service/craft_budget.go:1268-1270 ───
[bug · medium] BudgetPause 读取路径内嵌 authorizeBudgetActor（仅 Task owner 或租户 admin/owner 放行），与 handler 层的
TaskRead 门禁及其注释语义直接矛盾：GetCraftBudgetPause 先用 RequireTaskAccess(TaskRead) 放行
Collaborator/Viewer（internal/modules/craft/access.go 中 read 对三种角色均允许），随后服务内部将其 403 拒绝——普通协作者完全无法看到
Run 因预算耗尽被暂停的原因、用量与"联系 owner"入口，can_extend=false 的降级文案分支变成死代码。读取授权应由传输层的 TaskRead
门禁承担（MayExtendBudget/ExtendAndResume 已各自独立复核 authorizeBudgetActor），建议移除此处的 authorizeBudgetActor
调用；ExtendAndResume 在调用 s.BudgetPause 前已先通过 authorizeBudgetActor，移除后其授权语义不变。

- 	if err := s.authorizeBudgetActor(ctx, scope); err != nil {
- 		return craft.BudgetPause{}, err
- 	}
+ 	// 读取授权由 handler 层的 TaskRead 门禁承担；
+ 	// 扩展授权仍在 ExtendAndResume/MayExtendBudget 中独立复核。
+ 	_ = scope


─── internal/application/service/craft_budget.go:851-855 ───
[bug · low] 序号分配循环把 Create 的任意错误一律当作并发抢占序号槽 continue 重试：context 取消、连接中断、序列化失败等真实数据库故障会被静默重试 16 轮后伪装成
ErrConflict "sandbox call sequence contention"，掩盖根因并拉长失败路径（调用方看到的冲突码也不可据此安全重试）。同文件新增的
prepareCraftChargeStartWithProtocol 已采用 isUniqueViolation
区分唯一冲突与真实故障，此处应保持同一判别纪律（注意：一旦判定为非唯一冲突错误需直接返回，且 continue 前 call 变量不应残留半初始化状态）。

  		if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
+ 			if !isUniqueViolation(err) {
+ 				return err
+ 			}
  			// A concurrent sandbox authorization took this sequence slot;
  			// re-read the fresh maximum and retry with the next one.
  			continue
  		}


─── internal/application/service/craft_collaborator_run.go:85-88 ───
[bug · medium] 经此入口（PostCraftRun 现在把所有 Run 启动都路由到 StartCollaboratorRun）被 Collaborator 发起的
Run，发起者自己既不能停止也不能轮询：StartRun 的 Admission.UserID 固定为 session.UserID（存储所有者，craft_session.go 中 Submit 处
`UserID: session.UserID`），而 CraftControlService.Stop 与 DelegationStatus 都以 `run.UserID !=
scope.UserID` 拒绝调用者（返回 ErrForbidden）。结果是：Collaborator 持 TaskWrite 可以启动串行编辑，但该 Run 失控时只有 Task Owner
能停（也仅有 Owner 能查 delegation 状态），T09 的测试文件中也没有任何停止路径的用例固定这一行为。若"停止权威仅归 Owner"是有意设计，需要在此 seam
文档与测试中显式声明；否则应在 Stop/DelegationStatus 中放行 `run.ActorUserID == caller`（或改为 TaskWrite/TaskRead 门控）。

  	run, acquisition, err := s.StartRun(ctx, scope, req)
  	if err != nil {
  		return run, acquisition, err
  	}
+ 	// NOTE: 当前 Stop/DelegationStatus 以 run.UserID(=存储所有者) 授权，
+ 	// 协作者发起的 Run 只能由 Owner 停止/轮询——若非有意，请放行
+ 	// run.ActorUserID == caller 或改为 TaskWrite/TaskRead 门控，并补测试固定。


─── internal/application/service/craft_collaborator_run.go:92-92 ───
[bug · low] auditCollaboratorRunStart 的去重探测与写入都使用请求 ctx：StartRun 已提交后客户端断开（或网关取消请求）会同时取消探测和写入，T09 的
craft.run_started timeline 行就此永久丢失（无补偿/重试路径，仅 agent_runs.actor_user_id 保留身份）。本包内 Stop 对已承诺的持久化写已用
craftControlContext（值保留、取消隔离）处理同类问题，这里应保持同一纪律。

- 	s.auditCollaboratorRunStart(ctx, scope, initiatorRole, run, req)
+ 	detachCtx, cancel := craftControlContext(ctx)
+ 	defer cancel()
+ 	s.auditCollaboratorRunStart(detachCtx, scope, initiatorRole, run, req)


─── internal/application/service/craft_control.go:873-881 ───
[bug · medium] DelegationStatus 的两个 superseded 分支（此处与函数尾部 WriterRunTerminal 分支）绕过了上方 stopJourney
门控，无条件投影 Outcome=StopRequested。可达路径：一个从未触碰停止面的 run（无 intent 行、run.Status 非 canceled），当 delegation
结果行缺失时（failed run 的 SaveResult 未落盘、或 Completed 观察先于 SaveResult 提交的竞态窗口），轮询即返回
Outcome=StopRequested，宣称"成员请求过停止"。这与本函数自己声明的原则（"A delegation that never touched the stop surface
must not fabricate one — the zero Outcome is the honest no stop journey"）直接矛盾，也与 GetResult
命中路径不一致：同一个从未停止的 run，结果行存在时返回零 Outcome（projectOutcome 门控生效），结果行缺失时反而虚构 requested。一旦 T20 序列化器把
Outcome 投影到前端，正常完成/失败的 run 会被报成有停止请求。修复应在 stopJourney 为 false 时返回零 Outcome（注意不能直接改用
projectOutcome——它会经 durableStopOutcome 回放 stale intent 行，正是本分支刻意绕过的）。

  			if (craft.WriterRunTerminal(run.Status) && run.Status != "canceled") ||
  				craft.StopIntentSuperseded(observation) {
  				settled := "completed"
  				if run.Status == "failed" {
  					settled = "failed"
  				}
- 				return CraftStopStatus{Phase: settled,
- 					Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
+ 				outcome := craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}
+ 				if !stopJourney {
+ 					outcome = craft.StopOutcome{}
+ 				}
+ 				return CraftStopStatus{Phase: settled, Outcome: outcome}, nil
  			}


─── internal/application/service/craft_control.go:900-909 ───
[bug · medium] 函数尾部的 superseded 分支存在与观察块内 superseded 分支完全相同的问题：绕过 stopJourney 门控，对从未停止过的 run（结果行缺失的
succeeded/failed run，exec 不可用或观察未完成时落到此分支）无条件投影 Outcome=StopRequested，虚构一个不存在的停止请求，与函数前文"零 Outcome
是诚实的 no stop journey"原则矛盾。修复方式与观察块相同：stopJourney 为 false 时返回零 Outcome。

  	if craft.WriterRunTerminal(run.Status) {
  		// The superseded tail: the run row settled normally while the stop
  		// never confirmed — same bypass projection as the observe block.
  		settled := "completed"
  		if run.Status == "failed" {
  			settled = "failed"
  		}
- 		return CraftStopStatus{Phase: settled,
- 			Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
+ 		outcome := craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}
+ 		if !stopJourney {
+ 			outcome = craft.StopOutcome{}
+ 		}
+ 		return CraftStopStatus{Phase: settled, Outcome: outcome}, nil
  	}


─── internal/application/service/craft_control.go:578-582 ───
[bug · medium] 此处的注释承诺"nothing here blocks new dispatch until T20 lands the intent-aware dispatch
check in the same batch as the container's SetStopIntents assembly"，但该检查未随本批落地：全代码库 GetStopIntent
的消费仅在 craft_control.go 的 Stop/DelegationStatus 内，GuardDispatch（craft_lifecycle.go）只检查 sandbox 删除
tombstone，delegation 派发侧（craft_delegate.go 的 guard 检查）没有任何停止意图拦截。后果：PutStopIntent 之后、terminal CAS 之前
run 行保持非终态，所有 fenced write 照常成功——若 exec.Abort 交付失败（Stop 中仅记入 abortNote，不使停止失败）或 executor 在 abort
生效前继续派发新 delegation，成员已请求停止后的新工作仍可完成并落盘；pre-T17 的立即 runs.Cancel 曾通过 fence 全部写入阻断此场景。注释宣称"window is
bounded by the Abort in step 3"，但 step 3 的 Abort 是 best-effort（失败不阻断流程），abort 失败时窗口无界。建议：补上 dispatch
侧的意图检查（如 GuardDispatch 或 Delegate 派发前读 intent 行），或恢复停止接受后的写侧 fence，并同步修正此注释。



─── internal/application/service/craft_control.go:849-853 ───
[bug · medium] 此处的 StopUnknown 回放在 Observe 失败时不检查 run 行是否已终态，导致 superseded 收敛逻辑（下方 861-880 / 900-908
行）在该路径上永远不可达。可达路径：stop 以 unknown 落盘 → run 行随后正常 settled（succeeded/failed）且结果行缺失（SaveResult
未落盘的窗口，与已确认问题相同前提）→ 沙箱结束后 exec.Observe 持续报错 → 轮询永远返回 Phase="stopping"/Outcome=unknown，而 run
行早已终态。与本函数自己在 superseded 分支声明的"终态事实压过陈旧 intent"原则矛盾。建议在回放 unknown 前加
`craft.WriterRunTerminal(run.Status) && run.Status != "canceled"` 守卫，终态时投影 settled 事实。

  				if s.currentStopIntents() != nil {
+ 					if craft.WriterRunTerminal(run.Status) && run.Status != "canceled" {
+ 						// run 行已终态：陈旧 unknown 不得把轮询钉在 stopping（同下方 superseded 投影）
+ 						settled := "completed"
+ 						if run.Status == "failed" {
+ 							settled = "failed"
+ 						}
+ 						return CraftStopStatus{Phase: settled,
+ 							Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
+ 					}
  					if intent, ierr := s.currentStopIntents().GetStopIntent(ctx, scope, key.RunID); ierr == nil &&
  						intent.Status == craft.StopUnknown {
  						return CraftStopStatus{
- 							Phase:   "stopping",
+ 								Phase:   "stopping",


─── internal/application/service/craft_control.go:591-594 ───
[bug · low] Stop() 中其余所有返回路径（包括函数开头对 succeeded/failed 的 supersededOutcome() 投影）都设置了 Outcome，唯独这两个迁入
pre-T17 else 分支的 Cancel 冲突返回漏掉了——产出零值 StopOutcome（Status=""），正是本函数尾部注释自己指出的"零值 Status 是前端 banner
无法查询的第四个值"。同一逻辑情形（stop 与正常完成竞态、完成先落地）在函数开头走 supersededOutcome()，此处应保持一致。

  					case "succeeded":
- 						return CraftStopStatus{Phase: "completed", Note: "run completed normally before the stop landed"}, nil
+ 						return CraftStopStatus{Phase: "completed", Note: "run completed normally before the stop landed",
+ 							Outcome: supersededOutcome()}, nil
  					case "failed":
- 						return CraftStopStatus{Phase: "failed", Note: "run failed before the stop landed"}, nil
+ 						return CraftStopStatus{Phase: "failed", Note: "run failed before the stop landed",
+ 							Outcome: supersededOutcome()}, nil


─── internal/application/service/craft_docker_normal_exec.go:360-360 ───
[bug · medium] 此收敛分支在生产 provider 下不可达：DockerNormalExecClient.ObserveAttachedExec 在
positiveStartEvidence=false 时，非 Running 一律返回 Unknown（不会返回 Succeeded/Failed——注释明确 terminal 判定依赖进程内
start 证据）。因此 `!startEvidence && state != Running && state != Unknown`
永远为假，注释承诺的"claimed-never-started 收敛到 FAILED 终态、repl ay 读取确定状态"实际不会发生：claim 被消费但 ExecAttach 从未成功的
exec（崩溃窗口或 attach 失败且无 Running 证据）将永久停留在 Unknown，且该分支承诺的"空 sealed output row"也从未被创建。要实现该收敛，需要
provider 契约支持无进程内证据时区分"从未 attach"与"终态"（如受限面使用的 daemon exec_start/exec_die 事件对），或将 start 证据持久化。



─── internal/application/service/craft_export_consent.go:245-248 ───
[maintainability · low] 校验顺序与同文件 `view()` 不一致：`view()` 在最前先做 nil 守卫与 scope 完整性检查，而 `DecideExport` 把
`scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == ""` 放在了两次 deny 审计之后。当前 HTTP 面经
`craftScope` 保证 scope 完整所以不可达，但任何未来的非 HTTP 调用方传入不完整 scope 时，会先以 tenant_id=0、空 session 的键写入
audit_logs 去重窗口与审计行，再被拒绝——产生无法归属的审计脏数据；同时 `DecideExport` 也缺少 `view()` 携带的 `s == nil || s.versions ==
nil || s.evidence == nil` 守卫（nil 接收者会在 `s.taskAccess` 处 panic，而 `view()` 会 fail-closed）。建议把这两组守卫提升到
TaskShare 检查之前，与 `view()` 对齐。

+ 	if s == nil || s.versions == nil || s.evidence == nil {
+ 		return CraftExportConsentView{}, craft.ErrForbidden
+ 	}
+ 	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
+ 		return CraftExportConsentView{}, fmt.Errorf("%w: incomplete export consent request", craft.ErrInvalidInput)
+ 	}
  	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskShare); err != nil {
  		s.auditExportConsent(ctx, scope, versionID, craftExportDenyActionTaskAccess, "denied", map[string]string{"attempted_decision": string(decision)})
  		return CraftExportConsentView{}, err
  	}


─── internal/application/service/craft_export_consent.go:224-229 ───
[security · medium] T13 读路径缺少 caller 身份复核：同车道 T12 `ExportBundle` 与本文件 `DecideExport` 都在
`RequireTaskAccess` 之后执行 `caller := types.CallerFromContext(ctx); caller.TenantID != scope.TenantID
|| caller.UserID != scope.UserID → ErrForbidden（并写 deny 审计）`，而本读路径只依赖
`RequireTaskAccess(TaskRead)`——其校验的是 scope.UserID 的成员资格，并非 ctx 实际调用者。当前 HTTP 面经 `craftScope`
使二者恒等所以不可利用，但该读面暴露的敏感度与 T12 describe 相当（完整 manifest 含每个成员的 origins/ref/digest、受限派生分类、决定及
owner_id），文件自身声明 "The service owns every authority check"，任何未来非 HTTP 调用方以他人 scope
调用时即可绕过调用者绑定。建议补齐同一复核与 deny 审计，保持三个面一致的纵深防御。

  func (s *CraftExportConsentService) ExportConsentView(ctx context.Context, scope craft.Scope, versionID string) (CraftExportConsentView, error) {
  	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
  		return CraftExportConsentView{}, err
+ 	}
+ 	caller := types.CallerFromContext(ctx)
+ 	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
+ 		s.auditExportConsent(ctx, scope, versionID, craftExportDenyActionCallerIdentity, "denied", nil)
+ 		return CraftExportConsentView{}, craft.ErrForbidden
  	}
  	return s.view(ctx, scope, versionID)
  }


─── internal/application/service/craft_run_capture.go:176-179 ───
[bug · medium] claimErr 被整体吞掉且无日志，capture() 无条件继续执行昂贵的全树走查+上传。叠加两个事实使 craftCaptureDrainWindow
门失效：(1) ticker 的 freshness gate 只在 SELECT 时评估——recoverReceipts 顺序处理最多 100 条回执（单条 capture 可达分钟级、扫描预算
5 分钟），T0 读到的 pending 回执被 drain 认领后，ticker 到达该条时 ClaimForDrain 命中 0 行仍返回"成功"（见 ClaimForDrain 缺少
RowsAffected 检查）；(2) RecoverPendingForRun（drain 路径）完全不带 freshness gate，可直接取回 ticker 正在处理的 capturing
回执。结果：drain 与 ticker 可同时跑两次全树 sha256 走查 + uploadCapture（SaveBytes 每次新建 resource 行），输家的上传泄漏为无引用
resource 行——正是注释声称该门要防止的后果（数据完整性由 BeginCapture/Seal 幂等与 draft CAS 兜底，故定级 medium）。建议：认领失败（含 0
行）时重读回执状态，非 sealed/advanced 即跳过本轮并交还占有方，而非静默继续。

  		claimed, claimErr := s.captures.ClaimForDrain(ctx, receipt)
- 		if claimErr == nil {
+ 		if claimErr != nil {
+ 			// 认领失败：回执正被其他路径（drain/ticker）持有或状态已迁移。
+ 			// 重读确认；仅 sealed/advanced 可继续（幂等采纳），否则跳过本轮，
+ 			// 避免并发重做导致输家上传泄漏为无引用 resource 行。
+ 			current, rereadErr := s.captures.RecoverPendingForRun(ctx, receipt.Scope.TenantID, receipt.RunID)
+ 			if rereadErr != nil || len(current) == 0 || (current[0].State != "sealed" && current[0].State != "advanced") {
+ 				return craft.DraftHead{}, fmt.Errorf("%w: capture receipt owned by another drain", craft.ErrConflict)
+ 			}
+ 			receipt = current[0]
+ 		} else {
  			receipt = claimed
  		}


─── internal/container/craft_exec_policy_wiring.go:62-62 ───
[bug · medium] 持久输出配额与请求校验上限不一致：repository.validCraftDockerNormalInput 允许 OutputLimit 最大到
MaxCraftDockerOutputBytes（8MB），而这里装配的 CraftDockerOutputRepository 硬编码 1MB。凡是 OutputLimit 在 (1MB,
8MB] 区间的合法请求会完整走完暂存、门禁与执行，然后在存储层于 1MB 处被静默截断（truncated=true）：quota 错误经 provider 的 boundedWriter 记为
startErr，Execute 的 OutputComplete 恒为 false，每次都返回 normalUnknownError（Unknown 态错误），调用方无法从校验契约得知 2MB
超出部署配额。建议二选一：让装配配额与校验上限共用同一常量（craftDockerNormalOutputQuota =
repository.MaxCraftDockerOutputBytes），或在暂存/校验阶段以部署配额为界拒绝 OutputLimit 超限的请求（返回明确的
ErrCraftDockerNormalInputInvalid/TooLarge 而非截断+unknown）。

- const craftDockerNormalOutputQuota = 1 << 20
+ // 与 repository 校验上限保持同一来源，或在 NewCraftDockerNormalInputRepository
+ // 暂存阶段以该配额拒绝 OutputLimit 超限请求，而不是执行后静默截断。
+ const craftDockerNormalOutputQuota = repository.MaxCraftDockerOutputBytes


─── internal/container/craft_run_capture_promotion.go:97-101 ───
[bug · high] 晋升扫描存在确定性饿死：扫描谓词为 state IN ('sealed','advanced') AND draft_revision IS NOT NULL，而
draft_revision 仅由 MarkAdvanced 写入（Seal/EnsurePending/恢复合成 SQL 均不写），即窗口内全是 advanced
回执；晋升成功（或被拒绝）后回执无任何状态/updated_at 改写，生产代码也没有 craft_run_captures 的清理/归档路径。一旦全局（跨租户）累计 ≥32 条终态回执，ORDER
BY updated_at ASC LIMIT 32 将永远被最旧的已处理回执占满，新 sealed→advanced 的回执永远排不进窗口——晋升对新 Run
静默失效，新版本停留为私有草稿、旧默认版本保留席位，且无任何告警指向根因。RecoverRun 的 per-run 触发复用同一全局扫描，同样被饿死（也与其自身"不为无关 Run
付全局扫描成本"的注释矛盾：每次运行完成都在 worker 槽内跑跨租户扫描+最多 32 次探针）。次要影响：探针未注册期间（当前状态）每 15s 对同一批回执重复打出最多 32 条
WARN。建议在晋升成功时回写回执（如 state='promoted' 或 promoted_version_id+updated_at）并在扫描 WHERE
中排除，或改为游标分页/仅扫描未晋升回执。

- 	if err := p.db.WithContext(ctx).
- 		Table("craft_run_captures").
- 		Select("tenant_id, workspace_id, run_id, owner_id, session_id, state, draft_revision").
- 		Where("state IN ? AND draft_revision IS NOT NULL", []string{"sealed", "advanced"}).
- 		Order("updated_at ASC").Limit(limit).Scan(&rows).Error; err != nil {
+ 	// 晋升成功后回写终态，扫描排除已完成回执，避免最旧 32 条永久占满窗口：
+ 	// p.db.Model(&craftRunCaptureRow{}).Where("tenant_id=? AND workspace_id=? AND run_id=? AND state='advanced'", ...).
+ 	//     Updates(map[string]any{"state": "promoted", "updated_at": time.Now()})
+ 	Where("state = 'advanced' AND draft_revision IS NOT NULL AND promoted_version_id IS NULL").


─── internal/container/craft_run_capture_promotion.go:34-36 ───
[maintainability · low] 包级 registeredCraftWebPageLoadProbe 由 Register 无锁写入、装配路径（craft_runtime.go:290
与 craft_run_capture_wiring.go:125）无锁读取。当前无生产写入方（T14 未落地）、读取均发生在单线程容器装配期，尚无竞争；但该注册接口为 T14
预留，届时若在容器装配/ticker 启动之后注册（如独立模块 init 或 HTTP 管理路径），即成为数据竞争，晋升路径可能消费 nil 或部分初始化的探针。建议用 atomic.Value
存储，或在注释中强制"仅允许装配前注册"的约束并由 Registered 侧做原子读。

+ var registeredCraftWebPageLoadProbe atomic.Value // service.WebPageLoadProbe
+ 
  func RegisterCraftWebPageLoadProbe(probe service.WebPageLoadProbe) {
- 	registeredCraftWebPageLoadProbe = probe
+ 	registeredCraftWebPageLoadProbe.Store(probe) // 仅允许容器装配前调用
  }


─── internal/container/craft_run_capture_wiring.go:213-216 ───
[bug · medium] per-Run drain 路径上的 T20 晋升触发存在两个叠加缺陷：

1. ctx 已过期：drainCraftCaptureAfterTerminal 给整个 AfterTerminal（含此处）的预算是
15s（craftCaptureAfterTerminalBudget），而 svc.RecoverRun 的采集正是该预算的主要消耗方（这正是设置预算的原因）。任何非平凡回执封存完毕后
drainCtx 几乎必然已到期，随后的 promoteTerminalReceipts 的首个 DB 查询即以 context deadline exceeded 失败，只能打出 "terminal
receipt scan failed" 警告——即“本 Run 封存完成后立即晋升其 head”这一注释声明的目标，恰在真正发生封存的场景下从不生效，只剩周期 tick 兜底（而 tick
路径又被已确认的扫描饿死问题卡住）。

2. 全局扫描而非定向：promoteTerminalReceipts 是跨租户、ORDER BY updated_at ASC LIMIT 32 的全局扫描，与刚 drain 完的 Run
无关。注释说“the drain sealed this Run's output; attempt the four-check promotion of the sealed
head”，但刚封存的回执通常不在最旧 32 条窗口内；相反，每次 Run 完成都让该 worker 的执行槽为其他租户的回执做 ReadRevision + versions.Get，乃至（T14
注册后）外部浏览器页面探测，直到 15s 预算耗尽。

建议：为晋升单独派生有界上下文（context.WithTimeout(context.WithoutCancel(ctx), ...)），并给 promoter 增加按 (tenant_id,
run_id) 过滤的定向扫描变体供本路径使用，全局扫描仅保留在 RecoverTick 上。

  	// T20: the drain sealed this Run's output; attempt the four-check
  	// promotion of the sealed head (best-effort, idempotent, fail-closed).
- 	r.promoter.promoteTerminalReceipts(ctx, craftPromotionScanLimit)
+ 	// Promotion gets its own bounded context: the capture drain above usually
+ 	// consumed the AfterTerminal budget, and the per-Run trigger must target
+ 	// exactly this Run's receipts instead of the global oldest-first scan.
+ 	promoCtx, promoCancel := context.WithTimeout(context.WithoutCancel(ctx), craftPromotionAfterTerminalBudget)
+ 	defer promoCancel()
+ 	r.promoter.promoteReceiptsForRun(promoCtx, fence.TenantID, fence.RunID)
  }


─── internal/container/craft_runview_material.go:109-114 ───
[maintainability · low] materialHandleFromBoundView 的 store 参数在函数体内完全未使用（第 109-142 行未出现任何 store
读取），但 MaterialHandle 和 MaterialHandleForCapture 两个调用方都传入了 store。在这条安全敏感的不透明能力构造路径上，未使用的参数容易让后续维护者误以为
material handle 的推导依赖 RunViewStore 的额外读取（例如误以为此处有第二次 Load 或一致性检查），从而在改动时引入对 store
状态的错误依赖或遗漏真正的验证位置。建议移除该参数，或如确需保留请在注释中说明仅为签名一致性。

  func (p *CraftRunViewContainerProvider) materialHandleFromBoundView(
  	ctx context.Context,
- 	store craft.RunViewStore,
  	admittedKey craft.RunViewKey,
  	view craft.RunView,
  ) (CraftRunViewMaterialHandle, error) {


─── internal/handler/session/craft_share.go:44-48 ───
[maintainability · low] MountCraftShareRoutes 全库无任何调用点（特性注册闭包直接调用
RegisterCraftShareRoutes），属死代码；且其注释声称 "is called through the T00 constrained feature registry at
central assembly time" 与实际装配路径不符，会误导后续维护者以为存在第二条挂载链路。建议删除该方法，或修正注释说明真实挂载路径（RegisterCraftShareFeature
→ registry.Register 闭包 → RegisterCraftShareRoutes）。

- // MountCraftShareRoutes is called through the T00 constrained feature
- // registry at central assembly time.
- func (h *CraftShareHandler) MountCraftShareRoutes(group CraftRouteGroup) {
- 	RegisterCraftShareRoutes(group, h)
- }
+ // (删除 MountCraftShareRoutes；挂载仅经 RegisterCraftShareFeature 的
+ // registry.Register("share", func(group CraftRouteGroup) {
+ // 	RegisterCraftShareRoutes(group, NewCraftShareHandler(share))
+ // }) 闭包发生。)


─── internal/handler/session/craft_share.go:153-166 ───
[maintainability · low] 对客户端返回通用 StatusText 的防泄漏设计是合理的，但底层错误被完全丢弃：本函数既不通过 c.Error
挂载错误也不记日志，而请求日志中间件（middleware/logger.go:253）只记录 c.Errors.Last()。结果是最外层默认 503 分支（DB
宕机、文件存储故障等基础设施错误）以及 RevokeShare 中以 %v 包装进 ErrConflict 的确认读失败原因，在服务端不留任何痕迹，线上排障时只能看到 409/503
状态码而无根因。与兄弟 craftHTTPError（经 c.Error(apperrors...) 保留可观测性）的惯例不一致。建议在映射状态码的同时用 logger.ErrorWithFields
或 c.Error(err) 将底层错误留档（至少覆盖 default 分支），客户端响应体保持不变。

  func craftShareHTTPError(c *gin.Context, err error) {
  	code := http.StatusServiceUnavailable
  	switch {
  	case errors.Is(err, craft.ErrInvalidInput):
  		code = http.StatusBadRequest
  	case errors.Is(err, craft.ErrForbidden):
  		code = http.StatusForbidden
  	case errors.Is(err, craft.ErrNotFound):
  		code = http.StatusNotFound
  	case errors.Is(err, craft.ErrConflict):
  		code = http.StatusConflict
+ 	}
+ 	if code >= http.StatusInternalServerError {
+ 		// 客户端只看到通用状态文案；根因在服务端留档以便排障。
+ 		logger.ErrorWithFields(c.Request.Context(), err, map[string]interface{}{"path": c.FullPath(), "status": code})
  	}
  	c.JSON(code, gin.H{"error": http.StatusText(code)})
  }


─── internal/modules/craft/export_manifest.go:295-303 ───
[security · medium] 校验实现与注释声明的不变量不一致：注释称 "a colon anywhere in any segment ... is refused — Windows
resolves both as drive paths or NTFS ADS streams"，但循环内条件 `len(segment) >= 2 && segment[1] == ':'`
只拒绝形如 `c:evil` 的盘符前缀段（冒号在段内第 2 个字符且前一字符为字母）。`report:v2`、`notes:hidden` 这类段（冒号在其他位置）会同时通过
`ValidateArtifactPath`（该函数完全不拒绝冒号，Linux 沙箱可合法产出此类文件名）和本防御检查，随后作为 zip 条目名写入下载包（artifact_download.go
直接以 `member.Path` 作为条目名）——在 Windows 解压时即成为 NTFS
备用数据流（ADS），正是注释声称已拒绝的走私向量。建议改为拒绝任何包含冒号的段；这样也顺带完全覆盖前面的首段盘符检查（可删除该冗余分支，`reserved[normalized] ||` 与末尾
`reserved[member.Path]` 两处同样被 `reservedFold` 覆盖，可一并清理）。

- 		// Per-SEGMENT drive check: a colon anywhere in any segment (foo/c:evil
- 		// puts the drive spec mid-path; c:evil is drive-relative) is refused —
- 		// Windows resolves both as drive paths or NTFS ADS streams.
+ 		// Per-SEGMENT drive/ADS check: a colon in ANY segment (foo/c:evil puts
+ 		// the drive spec mid-path; notes:hidden is an NTFS ADS stream) is
+ 		// refused — Windows resolves both as drive paths or ADS streams.
  		for _, segment := range strings.Split(member.Path, "/") {
- 			if len(segment) >= 2 && segment[1] == ':' &&
- 				((segment[0] >= 'a' && segment[0] <= 'z') || (segment[0] >= 'A' && segment[0] <= 'Z')) {
- 				return fmt.Errorf("%w: bundle member %q segment %q uses a drive-letter path", ErrInvalidInput, member.Path, segment)
+ 			if strings.Contains(segment, ":") {
+ 				return fmt.Errorf("%w: bundle member %q segment %q uses a drive-letter or ADS path", ErrInvalidInput, member.Path, segment)
  			}
  		}


─── internal/modules/craftegress/adapter.go:282-285 ───
[security · medium] 适配器对沙箱客户端完全可控的 path+RawQuery 未做任何前缀白名单,却对每一次转发统一附加 Bearer
执行凭据。已核实网关转发面挂载在与全部业务路由共享的主 Gin
路由器上(internal/router/router.go:264-265,/api/v1/craft/model-gateway/v1/*,位于全局 Auth
之前)。虽然其余路由有各自的鉴权面,cmg1 凭据目前无法在其他路由通过认证,但这使适配器成为"携有效凭据的任意路径代理":任何未来信任 Bearer 的路由、任何回显/记录
Authorization 头的处理器都会把执行凭据暴露给被攻陷的沙箱。建议在 targetURL 中将可转发路径限制为模型网关前缀(如以配置项声明允许的路径集合,默认仅
/v1/chat/completions 与 /v1/models)。

+ 	// 只允许模型网关转发面:客户端可控制的 path 不得携执行凭据触达同源的其他路由。
+ 	allowed := map[string]bool{"/v1/chat/completions": true, "/v1/models": true} // 可按部署配置扩展
+ 	if !allowed[path] {
+ 		http.Error(w, "egress path refused", http.StatusNotFound)
+ 		return "", false
+ 	}
  	joined := *a.gateway
  	joined.Path = strings.TrimSuffix(joined.Path, "/") + path
  	joined.RawQuery = r.URL.RawQuery
  	return joined.String(), true


─── internal/modules/craftegress/adapter.go:218-222 ───
[bug · medium] 响应头已返回但 body 读取失败/超限时,该分支无条件以 definitive=false 驻留 attempt。然而此时 resp.StatusCode
是网关已达成结局的直接证据:网关仅在 409(绑定冲突)与 502(ACTIVITY_UNRESOLVED)上表达未知结局,其余状态码(尤其 2xx)意味着网关侧已 Resolve(Started)
并完成 usage 记账——是确定性结局。此处驻留后,同指纹重试会复用被驻留的 ID,永远命中网关 409 ACTIVITY_UNRESOLVED(适配器→网关链路被代理/LB
掐断长响应是现实场景),该逻辑请求在人工 reconciliation 之前永久不可完成。与主路径不同,残缺 body 无法解析信封,应退化为按状态码判定:非 409/502 一律
definitive(2xx 下重试铸造新 ID 重新计费是正确语义——两次物理发送),409/502 保持驻留。

  	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, a.maxBody+1))
  	if readErr != nil || int64(len(responseBody)) > a.maxBody {
  		if attemptID != "" {
  			digest := craftEgressRequestDigest(r.Method, r.URL.Path, r.URL.RawQuery, body)
- 			_ = a.journal.Resolve(attemptID, digest, resp.StatusCode, false)
+ 			// 头已返回但 body 残缺,无法解析信封:退化为按状态码判定——
+ 			// 非 409/502 的状态码本身就是网关确定性结局的证据。
+ 			definitive := resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusBadGateway
+ 			_ = a.journal.Resolve(attemptID, digest, resp.StatusCode, definitive)


─── internal/modules/craftegress/adapter.go:195-195 ───
[performance · low] strings.NewReader(string(body)) 对最长 16MB 的请求体做了一次整份字节→字符串复制再交给
Reader,每次转发都多分配一份请求体内存。body 本身已是 []byte 且不再复用,直接 bytes.NewReader(body) 零拷贝等价。

- 	req, err := http.NewRequestWithContext(r.Context(), r.Method, forwardURL, strings.NewReader(string(body)))
+ 	req, err := http.NewRequestWithContext(r.Context(), r.Method, forwardURL, bytes.NewReader(body))


─── internal/modules/craftegress/adapter.go:237-237 ───
[bug · high] 非 {409,502} 的 5xx（典型：入口/反向代理注入的 504，以及 body 撕裂无法解析的 502）被一律判定为 definitive
并解除驻留，重试将铸造全新身份，存在同一逻辑请求双份计费风险。

该判定的隐含前提是"到达适配器的每个响应都是网关自身做出的确定性裁决"，但 target.go 默认拒绝私网/环回网关主机，生产拓扑必然经公网入口（nginx/ingress）访问网关：入口的
proxy_read_timeout（nginx 默认 60s）远小于网关 5 分钟、适配器 6 分钟的转发预算。长生成在入口被掐断返回 504 时，网关侧物理发送仍在进行、将完成
BeginBinding/Resolve(Started) 与 usage 记账，而适配器已把该指纹 unpark——OpenCode/SDK 对 5xx
自动重试会带新身份发起第二次物理调用，同一逻辑请求双份计费，正是本模块注释反复强调要避免的后果（"bill the same logical request twice"）。

与本模块"宁可可见死锁、不可静默双计费"的取舍一致，建议：仅当状态 <500，或响应携带可解析且已知为 definitive 的网关信封（UPSTREAM_ERROR、上游透传的可解析 JSON）时才
definitive；无法证明来源的 5xx（504/503、撕裂 body）一律按未知驻留——由此产生的 409 循环可见、可对账。若保留现行为，至少应要求部署侧入口读超时 ≥
适配器转发预算并写入文档。

- 		definitive := !gatewayReportsActivityUnresolved(resp.StatusCode, responseBody)
+ 		// Definitive only when the response provably carries the gateway's
+ 		// own judgment: status < 500, or a parseable envelope whose code is
+ 		// known definitive (UPSTREAM_ERROR, upstream passthrough JSON). Any
+ 		// other 5xx (ingress 504 read-timeout, torn/unparseable bodies) must
+ 		// stay unresolved: parking is visible and reconcilable, unparking
+ 		// can bill one logical request twice.
+ 		definitive := gatewayOutcomeIsDefinitive(resp.StatusCode, responseBody)


─── internal/modules/craftegress/journal.go:96-99 ───
[performance · low] replay 用 os.ReadFile 一次性读入整个 journal 并 strings.Split 切行:记录上限 1<<20 条、每条约 150-200
字节时,峰值内存为文件内容 + 全部行字符串双份(数百 MB)再加 states/ordinals 两个 map。长期运行后重启会出现不必要的内存尖峰。append-only 逐行格式天然适合
bufio.Scanner 流式扫描,torn-tail 截断逻辑可用 scanner 偏移累加同样实现。

- 	data, err := os.ReadFile(j.path)
+ 	file, err := os.Open(j.path)
  	if err != nil {
+ 		return fmt.Errorf("craftegress: journal replay: %w", err)
+ 	}
+ 	defer func() { _ = file.Close() }()
+ 	scanner := bufio.NewScanner(file)
+ 	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
+ 	var offset int64
+ 	for scanner.Scan() {
+ 		line := scanner.Bytes()
+ 		// ...逐行处理,offset += int64(len(line)) + 1
+ 	}
+ 	if err := scanner.Err(); err != nil {
  		return fmt.Errorf("craftegress: journal replay: %w", err)
  	}


─── internal/modules/execution/sandbox/docker_normal_exec.go:125-131 ───
[maintainability · low] `started` 表条目在任何 attach 尝试后永不清理（注释明确“remains set after any attach
attempt”）。若该 client 与共享的 `DockerRemoteClient` 一样按进程/端点生命周期存在，每个 normal exec 都会永久保留一个 exec ID
条目（约百字节级），长期运行服务中构成无界增长。同链路的服务层对等表（`CraftDockerNormalExecService.started`、`CraftDockerRestrictedExec
.running`）均设了 4096 上限并整体重置，建议此处采用同样的有界策略（封顶重置或随容器移除/终态驱逐），安全语义仅降级为与进程重启等价的防御深度。

  type DockerNormalExecClient struct {
  	api            DockerNormalExecEngine
  	rpcTimeout     time.Duration
  	maxInputBytes  int64
  	maxOutputBytes int64
- 	started        sync.Map // exec ID -> struct{}; remains set after any attach attempt.
+ 	startedMu      sync.Mutex
+ 	started        map[string]struct{} // exec ID -> struct{}; capped table, see markStarted.
+ }
+ 
+ // markStarted records the one-consumed start claim with a bounded table:
+ // past the cap it resets wholesale, degrading only in-process double-start
+ // protection to the durable claim's authority (same as a process restart).
+ const dockerNormalExecStartedCap = 4096
+ 
+ func (c *DockerNormalExecClient) markStarted(execID string) bool {
+ 	c.startedMu.Lock()
+ 	defer c.startedMu.Unlock()
+ 	if _, loaded := c.started[execID]; loaded {
+ 		return false
+ 	}
+ 	if len(c.started) >= dockerNormalExecStartedCap {
+ 		c.started = make(map[string]struct{})
+ 	}
+ 	c.started[execID] = struct{}{}
+ 	return true
  }


─── internal/modules/execution/sandbox/docker_restricted_exec.go:137-140 ───
[bug · medium] 身份不匹配分支返回 `(unknown, nil)`，与紧邻注释“identity mismatch is an explicit error, not a silent
unknown”直接矛盾，也与同包 `inspectRaw`（docker_normal_exec.go）对同一条件返回 `ErrDockerNormalExecIdentityMismatch`
的行为不一致。后果：消费方 `CraftDockerRestrictedExec.Wait`（craft_docker_restricted_exec.go:221-229）把
State=Unknown 且 err==nil 视为“非终态证明，继续轮询到上限”，一个持久性的身份异常会被掩盖为整个 wait
窗口内的空转轮询，而不是作为显式错误浮出，违背本模块“绝不明猜、异常显式化”的契约。

  	if inspected.ID != receipt.ExecID || inspected.ContainerID != receipt.ContainerID {
- 		// (identity mismatch is an explicit error, not a silent unknown)
- 		return unknown, nil
+ 		// Identity mismatch is an explicit error, not a silent unknown.
+ 		return unknown, ErrDockerNormalExecIdentityMismatch
  	}


─── internal/modules/workbench/service/workbench/notification.go:120-122 ───
[bug · low] 耗尽 maxCursorHoleSkips（1024）后固定返回 ErrCursorExpired 且检查点原地不动：对超过 1024
个洞的蜂窝窗口（family-protected 前缀保留策略即可产生 {4,6,8,...} 形状），该 run 会在每个 worker tick 重放完全相同的 1024 次逐洞读，然后以错误中止
RunOnce（notification_worker.go 中 err 即 return）——重新引入本函数要消除的"字典序靠后 run 投影饿死"，且每 tick 浪费约 1024
次查询。建议耗尽后回退保留窗口头（与 !ok 分支同策略：root = FirstEventSeq-1 再读一次），窗口头必连续，可保证进度、解除对后继 run
的阻塞，丢失的洞内/跳过事件与本函数已声明的 best-effort 语义一致。

  		readErr = err // anchored at the NEW root
+ 	}
+ 	// Budget exhausted (a >1024-hole swiss-cheese window): falling back to
+ 	// the retained window head guarantees progress instead of re-creating
+ 	// the per-tick starvation this recovery exists to remove.
+ 	first, ferr := p.runs.FirstEventSeq(ctx, key)
+ 	if ferr != nil || first == 0 {
+ 		return after, nil, agentruntime.ErrCursorExpired
  	}
+ 	events, rerr := p.runs.ReadEvents(ctx, key, first-1, limit)
+ 	if rerr != nil {
- 	return after, nil, agentruntime.ErrCursorExpired
+ 		return after, nil, agentruntime.ErrCursorExpired
+ 	}
+ 	return first - 1, events, nil


─── package.json:19-19 ───
[test · low] typecheck:shared 本次补列了 status-notice.tsx、workbench-edit.tsx、export.tsx，但同批新增的
packages/views/src/craft/input-expand.tsx、share.tsx（以及本轮被修改的
usage.tsx、access.tsx）未列入，且它们无法从任何已列根文件传递覆盖——views barrel（packages/views/src/index.ts）不导出任何 craft
视图，workbench-edit/export/status-notify 等已列文件也只依赖 @weknora/contracts。这意味着这些新视图在共享类型检查中零覆盖，仅靠
typecheck:web 兜底，与"同批文件同脚本覆盖"的既有做法不一致。建议补齐 input-expand.tsx、share.tsx（或说明覆盖策略）。



─── packages/api-client/src/craft/index.ts:298-304 ───
[bug · critical] exportConsent 与服务端 wire 形状错配：T13
服务端（internal/handler/session/craft_export_consent.go GetCraftExportConsent，c.JSON(http.StatusOK,
craftExportConsentBody(view))，其 HTTP 测试 craft_export_consent_http_t13_test.go 直接把顶层响应体解为
version_id/manifest_digest/state/... 扁平结构）成功时返回的是裸扁平 consent 体，没有 {success, data} envelope。而
unwrap() 要求 envelope.success === true 才放行——扁平体没有 success 字段，因此每一次成功响应都会抛 ApiError('CRAFT_ERROR',
'Craft request failed')，导出确认面板永远无法加载。方法注释本身声明"resolved value 是 RAW consent wire payload"，与 unwrap
的用法自相矛盾。建议改为直接返回原始响应体（与 download()/openSource 的 raw 惯例一致），由 views 层 projectExportConsentView →
parseCraftExportConsentView 做唯一校验（解析器期望的正是该扁平形状）。

      async exportConsent(sessionId: string, versionId: string, signal?: AbortSignal): Promise<unknown> {
-       return unwrap(await request({
+       // 服务端成功时返回裸扁平 consent wire（无 success/data envelope）；
+       // views 层 projectExportConsentView/parseCraftExportConsentView 持有唯一校验。
+       return request({
          method: 'GET',
          path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/export/consent',
          signal,
-       }), 'export consent view');
+       });
      },


─── packages/api-client/src/craft/index.ts:318-323 ───
[bug · critical] decideExportConsent 存在与 exportConsent 相同的 envelope 错配：服务端
DecideCraftExport（craft_export_consent.go:156）成功时同样返回裸扁平 consent 体（其 HTTP 测试对 200 决策响应直接读顶层 state
字段佐证），unwrap 会因缺少 success 字段抛 ApiError。更严重的是此时服务端已持久化记录了 owner 的决策（approved/rejected
已落库），客户端却报告失败，用户会误以为决策未生效而反复重试。修复方向与 exportConsent 一致：去掉 unwrap 返回原始体（同时保留 4xx/409 错误路径——非 2xx 已由
request 层 errorFromResult 抛出）。

-       return unwrap(await request({
+       return request({
          method: 'POST',
          path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/export/consent/decision',
          body: { decision, manifest_digest: manifestDigest },
          signal,
-       }), 'export consent decision');
+       });


─── packages/api-client/src/craft/index.ts:332-340 ───
[bug · high] can_extend 读取层级错误：服务端 GetCraftBudgetPause（craft_budget_pause.go:103-107）的响应是
{"success": true, "data": {run_id, reason, limit, used}, "can_extend": bool}——can_extend 位于 envelope
顶层、与 data 平级。而此处先 unwrap 取出 data（即 pause 四字段），再从 data 里读 can_extend，永远是 undefined → canExtend 恒为
false。即使 owner/billing-admin 通过了服务端 MayExtendBudget 校验，扩展入口也永不显示（CraftBudgetPauseNotice 的
canExtend=false 分支只渲染 contact-owner 文案），T20 预算扩展功能经此客户端不可达。应先保留原始 envelope 从顶层读 can_extend，再 unwrap
取 pause 字段；顺带消除 (data as Record<string, unknown>)['can_extend'] 连续三次的重复转换。

-       const data = unwrap(await request({
+       const envelope = await request({
          method: 'GET',
          path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs/' + encodeURIComponent(runId) + '/budget/pause',
          signal,
-       }), 'budget pause view');
-       const canExtend = typeof (data as Record<string, unknown>)['can_extend'] === 'boolean'
-         ? ((data as Record<string, unknown>)['can_extend'] as boolean)
-         : false;
+       });
+       const data = unwrap(envelope, 'budget pause view');
+       // can_extend 位于 envelope 顶层（与 data 平级），缺失/非布尔时 fail-closed 为 false。
+       const rawCanExtend = (envelope as Record<string, unknown>)['can_extend'];
+       const canExtend = typeof rawCanExtend === 'boolean' ? rawCanExtend : false;
        return { pause: parseCraftBudgetPause(data), canExtend };


─── packages/contracts/src/craft/web-artifact.ts:115-117 ───
[maintainability · low] version_id 与 manifest_digest 在这里被完整校验了两遍：第一次是合成对象传入 parseCraftExportManifest
时其内部的 id(v.version_id, 'version_id') / id(v.manifest_digest, 'manifest_digest')，第二次是紧随其后的 versionID
/ digest 两个局部变量。功能上正确（fail-closed 不受影响），但属于同一逻辑的重复实现，未来调整校验规则时需要改两处。直接复用 manifest 解析结果即可消除重复，零行为变化。

-   const files = parseCraftExportManifest({ version_id: v.version_id, manifest_digest: v.manifest_digest, files: v.files }).files;
-   const versionID = id(v.version_id, 'version_id');
-   const digest = id(v.manifest_digest, 'manifest_digest');
+   const manifest = parseCraftExportManifest({ version_id: v.version_id, manifest_digest: v.manifest_digest, files: v.files });
+   const versionID = manifest.version_id;
+   const digest = manifest.manifest_digest;


─── packages/contracts/src/index.ts:178-178 ───
[maintainability · low] 本次导出行补齐了 web-artifact.ts
的大部分公共面（CRAFT_STOP_OUTCOMES、CRAFT_WRITER_ACQUIRE_OUTCOMES、CRAFT_DECISIONS、CRAFT_EXPORT_* 及对应
parser），但同一文件里的 web-check 词汇
CRAFT_WEB_CHECK_OUTCOMES、CraftWebCheckOutcome、CraftWebCheckEvidence、parseCraftWebCheckEvidence
未列入。它们目前只能经 craft/index.ts 的 `export * from './web-artifact.ts'`
在二级桶可达，顶层包消费者无法直接引用：domain/web-promotion.ts 因此只能写 NonNullable<CraftVersionView['web_evidence']>
间接推导类型，也无法把 'passed' 字面量钉在 CraftWebCheckOutcome 上（这正是已确认的 PASSED_OUTCOME
硬编码漂移问题的修复前置——不导出该词汇表，下游就无法建立类型关联）。建议在两行导出中一并补齐这四个名字。

- export type { CraftSessionKind, CraftRunStatus, CraftTerminalRunStatus, CraftBudgetPause, CraftCheckStatus, CraftEventKind, CraftStopOutcomeStatus, CraftStopOutcome, CraftWriterAcquireStatus, CraftWriterAcquireOutcome, CraftDecisionStatus, CraftExportConsentState, CraftExportOriginKind, CraftExportOriginRef, CraftExportFile, CraftExportManifest, CraftExportDecision, CraftExportConsentView, CraftSessionCreatedView, CraftSessionSummaryView, CraftCapabilitiesView, CraftSessionPageView, CraftRunView, CraftFileVersionView, CraftVersionCheckView, CraftVersionView, CraftVersionsPageView, CraftWorkspaceRefView, CraftWorkspaceView, CraftInputView, CraftPreviewTicketView, CraftRunEventView, CraftEventPayloadView } from './craft/index.ts';
+ export type { CraftSessionKind, CraftRunStatus, CraftTerminalRunStatus, CraftBudgetPause, CraftCheckStatus, CraftWebCheckOutcome, CraftWebCheckEvidence, CraftEventKind, CraftStopOutcomeStatus, CraftStopOutcome, CraftWriterAcquireStatus, CraftWriterAcquireOutcome, CraftDecisionStatus, CraftExportConsentState, CraftExportOriginKind, CraftExportOriginRef, CraftExportFile, CraftExportManifest, CraftExportDecision, CraftExportConsentView, CraftSessionCreatedView, CraftSessionSummaryView, CraftCapabilitiesView, CraftSessionPageView, CraftRunView, CraftFileVersionView, CraftVersionCheckView, CraftVersionView, CraftVersionsPageView, CraftWorkspaceRefView, CraftWorkspaceView, CraftInputView, CraftPreviewTicketView, CraftRunEventView, CraftEventPayloadView } from './craft/index.ts';
+ export { CRAFT_SESSION_KINDS, CRAFT_RUN_STATUSES, parseCraftBudgetPause, CRAFT_TERMINAL_RUN_STATUSES, CRAFT_CHECK_STATUSES, CRAFT_WEB_CHECK_OUTCOMES, parseCraftWebCheckEvidence, CRAFT_EVENT_KINDS, CRAFT_STOP_OUTCOMES, CRAFT_WRITER_ACQUIRE_OUTCOMES, CRAFT_DECISIONS, CRAFT_EXPORT_ORIGIN_KINDS, CRAFT_EXPORT_CONSENT_STATES, parseCraftWriterAcquireOutcome, parseCraftExportManifest, parseCraftExportDecision, parseCraftExportConsentView, parseCraftSessionCreated, parseCraftSessionPage, parseCraftRunView, parseCraftVersionView, parseCraftVersionsPage, parseCraftWorkspaceView, parseCraftInputView, parseCraftPreviewTicket, parseCraftRunEvent, parseCraftEventPayload } from './craft/index.ts';


─── packages/domain/src/craft/web-promotion.ts:31-32 ───
[maintainability · low] PASSED_OUTCOME = 'passed' 是硬编码字面量，未与 contracts 已冻结的检查结果词汇表
CRAFT_WEB_CHECK_OUTCOMES（['passed', 'failed', 'not_run']，web-artifact.ts:3）建立类型关联：若词汇表未来调整，此处会静默漂移且
TypeScript 无法捕获（常量类型是宽泛的 string）。本文件已经 import type CraftVersionView，补一个类型标注即可把字面量钉死在词汇表上，零运行时成本。

+ import type { CraftVersionView, CraftWebCheckOutcome } from '@weknora/contracts';
+ 
  /** The passed outcome literal, shared by every four-check comparison. */
- const PASSED_OUTCOME = 'passed';
+ const PASSED_OUTCOME: CraftWebCheckOutcome = 'passed';


─── internal/container/craft_web_build.go:313-315 ───
[security · high] 伪造面未闭合:build-log.json 位于容器内 craft 用户可写的 /workspace/output(Dockerfile `install -d
-o craft ... /workspace/output`),而它需要匹配的全部身份字段对沙箱内进程都是可知的——toolchain digest / template sha256 /
version 固化在镜像内 0555 世界可读的 /opt/craft/web/toolchain.lock.json,runtime digest 本身就是通过 --runtime-digest
命令行参数传入沙箱并由 build.py 原样写入日志的。因此恶意 Run 可直接手写 exit 0 的 build-log.json 并手写未经净化的 index.html:BuildChecks
会把 CheckBuild 记为 passed(internal/modules/craft/version.go BuildChecks 只看
BuildRan/BuildExitCode),entry 检查只核对文件在采集清单内(自洽,不绑定模板),页面探针对任意可加载页面都会通过——整条 build.py 净化管线与模板钉扎被绕过,且预览
CSP 的 script-src 含 'unsafe-inline'(craft_preview.go PreviewCSP),绕过的后果不止于视觉。建议 BuildExitCode
的权威来源改为服务器侧已观测的执行回执(dispatch/exec 协调器本身能看到真实退出码,参考 input_code.go 中"closed by execution-receipt
evidence"的既定路线),build-log.json 仅作交叉核对而非唯一事实来源;在回据落地前,至少在文档中把"日志可被沙箱伪造"记为已知残留,而不是以"REAL exit
status"自居。



─── internal/container/craft_web_build.go:388-390 ───
[maintainability · low] 两个不同条件被合并成一个语义错误的错误消息:src == nil 才是"reader 未装配",而 task 无 SessionID 是任务缺身份,此时
reader 已装配。后者返回普通 error(非 fs.ErrNotExist),在 craftWebBuildEvidenceSource 中会落入 `!errors.Is(err,
fs.ErrNotExist)` 分支、每次评估都触发 Warnf——批量评估无会话任务时制造日志噪音,稀释真正的篡改/漂移告警信号。建议拆分条件:缺 SessionID 时返回
fs.ErrNotExist(按"未观测"静默处理),src == nil 保留原消息(或同样返回 ErrNotExist,由上层一次性告警)。

- 		if src == nil || strings.TrimSpace(task.Scope.SessionID) == "" {
+ 		if src == nil {
  			return nil, errors.New("craft web build log reader is not assembled")
+ 		}
+ 		if strings.TrimSpace(task.Scope.SessionID) == "" {
+ 			return nil, fs.ErrNotExist
  		}


─── internal/container/craft_web_build_review.go:167-172 ───
[security · medium] --input/--output 的取值在形状检查中完全不受约束:绝对路径、含 `..` 的路径、乃至重复 flag(如 `--output
/workspace/output --output /tmp/x`,argparse 取最后一个生效)都能通过;而 T03 层 ReviewNormalExec 的词法筛查只拒绝"操作数位于只读
inputs 树内"(internal/modules/craft/input_code.go 中非 inputs 树内的操作数作为 pure data operand 放行),并不把
--output 限制在 /workspace/output。这违背了本 gate 自己声明的"the entry cannot be borrowed to dispatch arbitrary
commands":经此入口派发的 build.py 可读写容器内任意用户可写/可读路径(--input 指向任意 content.json
时其字节会被渲染进被采集产物)。--runtime-digest 同样不受约束,派发时传错值会让真实构建产出的日志被 pin.RuntimeDigest
比对拒绝、构建证据静默丢失。建议在形状检查中收敛取值类 flag:每个 flag 仅允许出现一次,--input/--output 必须位于容器 workspace 布局之下且不含 `..`。

  		if name == "toolchain" {
  			if value != craftWebBuildToolchainPath {
  				return false
  			}
  			toolchainSeen = true
+ 		}
+ 		if name == "input" || name == "output" {
+ 			if !strings.HasPrefix(value, "/workspace/") || strings.Contains(value, "..") {
+ 				return false
+ 			}
  		}


─── docker/craft/web/build.py:67-67 ───
[security · medium] 黑名单未覆盖 <style>/<link> 标签与 style= 属性:staged 片段可携带任意内联 CSS(固定定位遮罩、隐藏免责声明、在 app
内嵌预览 iframe 中伪装 UI)。已确认预览 origin 的 CSP 恰好允许 style-src 'self'
'unsafe-inline'(internal/application/service/craft_preview.go PreviewCSP),所以构建期净化层是唯一防线,而它目前对 CSS
完全没有;CSS_FETCH_RE 只拦 url()/@import(网络外联),拦不住纯视觉欺骗。建议把 style|link 纳入 EMBED_TAG_RE(模板自身的 <link> 不经
render_html 扫描,不受影响);style= 属性若需放行,应另加白名单约束或在预览侧去掉 'unsafe-inline'。

- EMBED_TAG_RE = re.compile(r"<\s*(base|iframe|object|embed|form|script|meta)\b", re.IGNORECASE)
+ EMBED_TAG_RE = re.compile(r"<\s*(base|iframe|object|embed|form|script|meta|style|link)\b", re.IGNORECASE)


─── docker/craft/web/build.py:249-250 ───
[style · low] 此赋值是对函数前段已校验过的同名变量(template_pin = lock.get("template"))的冗余重复赋值,值完全相同,属死存储,影响可读性,建议删除。

-     template_pin = lock["template"]
      if not isinstance(template_pin.get("version"), str) or not template_pin["version"].strip():


─── internal/application/repository/agent_run_lifecycle.go:126-130 ───
[bug · medium] DeleteSessionRuns 存在 TOCTOU 竞态：Pluck 用无锁 SELECT 且只过滤 status NOT IN
('succeeded','failed')，而 cancelRunTx 持锁后会拒绝 succeeded/failed 并返回 ErrConflict。若某运行在 Pluck
之后、cancelRunTx 加锁之前被并发 Finalize 为终态（两者都要竞争同一 run 行锁，谁后到谁看到对方已提交的状态），整个删除会话事务将回滚，DeleteSessionRuns
向上返回 Conflict；调用链 fenceSessionRuns → handler 无任何重试，用户删除会话会偶发 5xx/冲突（下次重试时 Pluck
已排除该终态运行才能成功）。建议在删除场景对"列出后才落终态"的运行宽容跳过而非中止，例如 ErrConflict 时在已持有的行锁下复读状态，确认已变为 succeeded/failed 则
continue。

  		for _, id := range ids {
- 			if err := s.cancelRunTx(tx, agentruntime.RunKey{TenantID: tenantID, RunID: id}, "session_deleted"); err != nil {
+ 			key := agentruntime.RunKey{TenantID: tenantID, RunID: id}
+ 			if err := s.cancelRunTx(tx, key, "session_deleted"); err != nil {
+ 				if errors.Is(err, agentruntime.ErrConflict) {
+ 					var current agentRunRow
+ 					if readErr := runScope(tx, key).Take(&current).Error; readErr == nil &&
+ 						(current.Status == "succeeded" || current.Status == "failed") {
+ 						continue // 列出后才落终态：结算事实需保留，跳过而非中止整个删除
+ 					}
+ 				}
  				return err
  			}
  		}


─── internal/application/repository/agent_run_lifecycle.go:62-62 ───
[bug · low] 计费挂起分支忽略了 json.Marshal 的错误（payload, _ := ...），而几行之下直接取消分支对同样的 Marshal 做了错误检查，两处不一致。虽然
map[string]string 实际不会序列化失败，但忽略错误的写法削弱了错误处理一致性，也让后续维护者难以判断这里是否为有意为之的 best-effort。建议与直接取消分支保持一致的处理方式。

- 		payload, _ := json.Marshal(map[string]string{"reason": reason})
+ 		payload, err := json.Marshal(map[string]string{"reason": reason})
+ 		if err != nil {
+ 			return err
+ 		}


─── internal/application/repository/craft_stop_intent.go:106-111 ───
[bug · low] PutStopIntent 在 upsert 之后回读 durable 行时把错误静默丢弃：`if err := ...Take(&settled).Error; err ==
nil` 只在成功时更新 effective，失败时仍 `return nil` 提交事务，并以调用方传入的 intent 作为返回值。在插入竞态窗口（锁定 Take 读到
NotFound、并发写者先提交了 confirmed、CASE 保住 confirmed）叠加这次回读失败时，方法会返回 requested/unknown 而 durable 行是
confirmed —— 违反本方法自己声明的 "Answer the DURABLE row" 契约，调用方（control service 的幂等重放）会基于错误状态继续分支。数据库不变量本身由
CASE 保住，所以影响有限，但此处应与其他读路径一致地传播错误并回滚，而不是把失败读转换成成功提交。

  		var settled craftStopIntentRow
  		if err := tx.Where("tenant_id = ? AND session_id = ? AND run_id = ?",
- 			scope.TenantID, scope.SessionID, intent.RunID).Take(&settled).Error; err == nil {
- 			effective = craft.StopIntent{RunID: settled.RunID, Status: settled.Status}
+ 			scope.TenantID, scope.SessionID, intent.RunID).Take(&settled).Error; err != nil {
+ 			return err
  		}
+ 		effective = craft.StopIntent{RunID: settled.RunID, Status: settled.Status}
  		return nil


─── internal/application/service/craft_artifacts.go:596-605 ───
[security · high] 服务端 HTML denylist 仅在 kind == KindWeb 时执行，但该保护对其他可预览 kind 完全失效：stageAndUpload 对所有
kind 枚举并采集整个 OutputDir（非 web kind 的 manifest.json 门只校验交付物形状，不过滤其余成员），而 PreviewableKind 对
web/document/spreadsheet/slides 四种 kind 均返回 true，Issue() 会在同一隔离预览源上为它们签发票据，lookup 按清单路径提供任意成员。因此一个
document/slides 会话的 Run 可以发布携带完全未筛查 .html/.svg 成员的版本；预览响应的 CSP 为 script-src 'self'
'unsafe-inline'（允许内联脚本/事件处理器执行），且 CSP 无法阻止顶层导航（location 赋值、<a href="https://...">、meta refresh 均不受
fetch 指令约束）——终端用户浏览器可被导航到任意外域，这正是 BrowserNavigationProtected 门与本次服务端筛查所要封堵的威胁（代码自身注释明确输出树是 Agent
可写、不可信的）。相比之下，web kind 内 .css/.js 成员未筛查的风险已被 CSP 兜住（style-src/img-src 'self' 阻断 @import/url()
外联，未引用的 .js 不会执行）。建议：HTML 家族成员筛查不应以 kind 为条件（四种 kind 共用同一服务边界与信任模型），同时对固定路径 assets/craft-web.js
的钉扎校验也应无 kind 条件地生效。

- 		if kind == craft.KindWeb && craftScreenWebMemberIsHTML(rel) {
+ 		// The screen must not be kind-gated: every previewable kind shares the
+ 		// same isolated preview origin and the same Agent-writable output tree.
+ 		if craftScreenWebMemberIsHTML(rel) {
  			if err := craftScreenWebHTMLMember(rel, data); err != nil {
  				logger.Warnf(ctx, "[CraftArtifact] server-side web screen rejected member %q of run %s: %v", rel, task.Fence.RunID, err)
  				return nil, err
  			}
  		}
  		// The template shell's script exemption is only honest when the
  		// referenced asset member itself matches the pinned digest.
- 		if kind == craft.KindWeb {
- 			if err := craftScreenVerifyPinnedAsset(rel, data); err != nil {
+ 		if err := craftScreenVerifyPinnedAsset(rel, data); err != nil {


─── internal/application/service/craft_web_screen.go:110-110 ───
[maintainability · medium] craftScreenPinnedAssetDigest 把 docker/craft/web/toolchain.lock.json 中
craft-web.js 的
sha256（已核对当前值一致：5b930dd9...30064）复制成了第二份硬编码，且该摘要全仓库仅在此处出现——没有任何测试或构建步骤交叉校验两者。该资产已有版本迭代（lock 中为
v1.1.1），后续升级只改 lock 不改此常量时，模板壳引用的资产与常量不一致，craftScreenVerifyPinnedAsset 会拒绝每一次合法 web 轮次采集（web
产物功能整体不可用，fail-closed 但属可用性故障）；反之若从暂存内容反推更新常量则豁免保护名存实亡。建议从嵌入的 toolchain.lock.json 派生该值（构建期
embed/解析），或至少添加单元测试断言常量 == lock 文件中 craft-web.js 的 sha256。

  const craftScreenPinnedAssetDigest = "5b930dd96b33bf707653ed6c1b5dc294cb8a790ee7ba7b6adfd13d65a0130064"
+ 
+ // craft_web_screen_lock_test.go 中增加防漂移校验：
+ // func TestPinnedAssetDigestMatchesToolchainLock(t *testing.T) {
+ // 	raw, err := os.ReadFile("../../../docker/craft/web/toolchain.lock.json")
+ // 	require.NoError(t, err)
+ // 	var lock struct {
+ // 		Dependencies []struct {
+ // 			Name   string `json:"name"`
+ // 			SHA256 string `json:"sha256"`
+ // 		} `json:"dependencies"`
+ // 	}
+ // 	require.NoError(t, json.Unmarshal(raw, &lock))
+ // 	for _, dep := range lock.Dependencies {
+ // 		if dep.Name == "craft-web.js" {
+ // 			require.Equal(t, craftScreenPinnedAssetDigest, dep.SHA256,
+ // 				"pinned asset digest drifted from toolchain.lock.json")
+ // 		}
+ // 	}
+ // }


─── internal/application/service/craft_archive.go:196-200 ───
[maintainability · low] 回滚路径中关联 Count 查询失败时静默 continue（保守跳过删除、对象泄漏），与紧随其后的 DeleteFile 失败路径的 Warnf
不对称：Count 失败导致的对象泄漏既无日志也无任何痕迹，长期运行下存储泄漏不可审计。跳过删除本身是安全方向（宁可泄漏也不误删仍被引用的对象），但失败应当留痕。既有 craft_inputs.go
中是同样写法，建议此处连同该模式一起补上日志。

  			var associations int64
  			if err := s.db.WithContext(cleanupCtx).Model(&craftWorkspaceInputRow{}).
- 				Where("ref = ?", createdRef).Count(&associations).Error; err != nil || associations != 0 {
+ 				Where("ref = ?", createdRef).Count(&associations).Error; err != nil {
+ 				logger.Warnf(cleanupCtx, "[CraftArchive] rollback association count failed for ref %s (object may leak until reclamation): %v", createdRef, err)
+ 				continue
+ 			}
+ 			if associations != 0 {
  				continue
  			}


─── internal/application/repository/craft_version.go:339-343 ───
[bug · medium] sameCraftVersionEvidence 对顶层 AcquiredAt 特意用 .Equal 防止 time.Time
的表示敏感比较（注释自己也说明了幻影冲突风险），但紧接着的 Sources 元素循环却用裸 != 比较 KnowledgeSourceRecord——该结构体内含 AcquiredAt
time.Time，Go 的 == 对 time.Time 按位置/单调表示比较：同一时刻若在两次编码中以不同时区偏移序列化（如 "+08:00" vs "Z"，常见于一边来自内存 Local
时间、另一边经持久层归一为 UTC 的重建路径），解码后 == 判为不等，导致同一事实的 PublishWithEvidence 幂等重放被误报 craft.ErrConflict（"already
pinned different evidence"），破坏本函数自己声明并逐字段守护的 replay-identity 契约。建议对 source 做逐字段比较，时间字段用 .Equal。

  	for i := range a.Sources {
- 		if a.Sources[i] != b.Sources[i] {
+ 		x, y := a.Sources[i], b.Sources[i]
+ 		if x.ID != y.ID || x.Ref != y.Ref || x.Digest != y.Digest ||
+ 			x.TenantID != y.TenantID || x.ExcerptBytes != y.ExcerptBytes ||
+ 			!x.AcquiredAt.Equal(y.AcquiredAt) {
  			return false
  		}
  	}


─── internal/application/service/craft_artifacts.go:287-289 ───
[bug · medium] PromoteWebVersion 的 revision fence 是纯 check-then-act：s.drafts.Read 校验与后续
s.versions.Publish/PublishWithEvidence 之间没有任何事务性或 CAS 关联（DraftHeadStore 与 VersionStore
是两个独立存储）。并发交错下 fence 可被绕过：P_A 针对旧 run A 校验通过 head rev N → run B 密封 head rev N+1 且 P_B 校验并发布成功 → P_A
的发布最后落库。此时版本 A 的 created_at 最新，而仓库 List 按 created_at DESC 排序、SelectDefaultVersion
取首个四检查全过者，预览默认席位将长期展示已被 run B 取代的旧 run 文件——这正是本 fence 注释声明要拒绝的 "an old run's files silently promoted
as the newest version"（协作成员并发触发两次 promotion 即可命中，publishWithEvidence 的记录加载+事务进一步拉宽窗口）。建议在发布事务内（或发布后以
revision CAS 复核，失败则回滚/标记）重新验证 head 仍为该校验过的 (Revision, SourceRunID, ManifestDigest)。



─── internal/application/service/craft_web_screen.go:172-174 ───
[security · low] 屏蔽范围只覆盖 HTML 族扩展名（.html/.htm/.svg/.xhtml/.xht），而 craftScreenCSSFetchRe（url( /
@import）与外链 URL 拒绝规则对真正承载这些形状的 .css 成员完全不做检测：index.html 可合法 <link rel=stylesheet
href="assets/app.css">（相对引用通过 craftScreenAbsoluteRefRe），web kind 又没有 manifest
成员集门（craftValidateStagedManifest 对 web 直接放行），因此一个携带 @import url("https://…") 或 background:url(//…) 的
staged CSS 可通过采集进入不可变版本并在预览源上服务。当前仅靠 serve 端 PreviewCSP（style-src 'self'、img-src 'self' data:
blob:、connect-src 'none'）在浏览器侧拦截外链 fetch，采集期 screen 自身宣称的 "offline local assets only" 约束对 CSS
文件并未成立——一旦后续放松 CSP 或新增不经该 handler 的服务路径，此出口即静默重开（round-3 修复要堵的 egress 通道）。建议对 .css 成员至少执行外链 URL +
css fetch + 控制字符变体的同一 denylist，或在注释中显式记录对该 CSP 的依赖。



─── internal/modules/craft/archive.go:180-188 ───
[security · high] countPaxHeader 中对压缩比的检查是空操作:if 体内只有注释,没有任何 return/标记。而注释声称 "checked again at
finish()" —— 但 finish() 只检查 members 为空并排序,并未复查 b.total 或比例。结果是:pax
扩展头(TypeXHeader/TypeXGlobalHeader)的解压字节从未受到 MaxArchiveExpandedBytes 或 MaxArchiveCompressionRatio
的任何约束。

实际影响:ExtractArchive 是纯 CPU 的内存内调用(不接收 ctx,不经过任何可被取消的 IO),服务层 craft_archive.go:52 的 30s 超时无法中断解压循环 ——
常量注释里 "the pure-CPU decompression loop itself is bounded indirectly by the byte and entry ceilings
(including pax header bytes)" 是唯一声称的 CPU 约束,而它并不存在。一个仅由扩展头构成的 ~20MiB tar.gz 可以解出数十 GB 的 pax 流,单请求烧
CPU 数分钟,绕过本模块文档化的全部天花板。另外若标准库 tar.Reader 在内部消化 x/g 条目,该 case 甚至不会触发,计数更无从谈起 —— 稳妥做法是用计数 reader 包住整个
tar 流,对所有解压字节(头块+数据+pax)统一封顶。

顺带:MaxArchiveExtractDuration 注释中残留的悬空句 "which bounds the CPU an archive can consume: ... both byte
totals are capped above" 与事实相反,建议一并修正。

- func (b *archiveBudget) countPaxHeader(size int64) {
+ func (b *archiveBudget) countPaxHeader(size int64) error {
  	if size <= 0 {
- 		return
+ 		return nil
  	}
  	b.total += size
+ 	if b.total > MaxArchiveExpandedBytes {
+ 		return fmt.Errorf("%w: archive pax metadata expands to %d bytes over the %d cap",
+ 			ErrInvalidInput, b.total, MaxArchiveExpandedBytes)
+ 	}
  	if b.total > b.compressed*MaxArchiveCompressionRatio {
- 		// checked again at finish(), but failing early stops the CPU burn
+ 		return fmt.Errorf("%w: archive pax metadata expands %d bytes from %d compressed bytes over the %d:1 ratio cap",
+ 			ErrInvalidInput, b.total, b.compressed, MaxArchiveCompressionRatio)
  	}
+ 	return nil
  }
+ 
+ // 调用处:
+ //	if err := budget.countPaxHeader(header.Size); err != nil {
+ //		return nil, err
+ //	}
+ // 并在 finish() 中对 b.total 做同样的双重复查(与注释声明一致);更稳妥的做法是用带累计字节计数的 io.Reader 包住 tar.Reader,对全部解压字节统一执行 MaxArchiveExpandedBytes/比例封顶。


─── internal/modules/craft/input_code.go:462-466 ───
[bug · medium] 这里对 php 的 post-script 位置参数筛查是无条件的:只要解释器是 php,任何指向 inputs 树的位置参数都被拒绝。但紧邻注释写明例外应当只针对 "a
positional after -S is a router script"(php -S 的路由脚本),而代码没有跟踪 -S 是否出现。后果是 `php gen.php
inputs/data.csv` 这类合法的数据读取(与 `python3 gen.py inputs/data.csv` 对 python 是放行的)被整体拒绝 ——
与文档语义和其他解释器的行为不一致,属于超出文档意图的误拒。建议在 flag 分支记录 -S 出现与否,仅在 -S 之后的位置参数上应用该拒绝。

- 			if isPHPFamily(path.Base(req.Command[interpreterOffset(req.Command)])) {
+ // 在解释器操作数循环前声明 phpServer := false,并在 flag 分支中:
+ //	if strings.HasPrefix(arg, "-") {
+ //		...
+ //		phpServer = phpServer || arg == "-S"
+ //		programFileNext = programFileFlag(arg)
+ //		continue
+ //	}
+ // 之后仅对 -S 之后的位置参数执行路由脚本拒绝:
+ 			if phpServer && isPHPFamily(path.Base(req.Command[interpreterOffset(req.Command)])) {
  				if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
  					return p.deny("interpreter_input", abs, "")
  				}
  			}


─── internal/modules/craft/input_code.go:831-833 ───
[documentation · low] programFileFlag 的文档注释首行是复制粘贴残留的 hasExecForwardFlag 文档("hasExecForwardFlag
reports whether any argv token is find's execution forwarding flag..."),与函数实际语义不符;而文件后方的
hasExecForwardFlag 反而没有文档。两段注释错位会误导后续维护者对这两个判定函数的理解。

+ // programFileFlag reports whether an option token selects a PROGRAM FILE in
+ // a multi-program interpreter family: awk/mawk/gawk -f/--file, php -B/-F/-R/-E
+ // (both attached and bare separated forms).
+ func programFileFlag(arg string) bool {
+ ...
+ // 并将 hasExecForwardFlag 的文档移至其函数定义处:
  // hasExecForwardFlag reports whether any argv token is find's execution
  // forwarding flag (separate or =-attached forms).
- // programFileFlag reports whether an option token selects a PROGRAM FILE in
+ func hasExecForwardFlag(command []string) bool {


─── internal/modules/craft/input_code.go:894-899 ───
[bug · medium] carriesProgramTextFlag 在短选项组的任意位置命中 c/e/r/m 即判定为程序文本并整体拒绝,会确定性误拒一批标准、良性的解释器启动形式:java
在 interpreterCommands 中被显式支持,但 `java -jar out/app.jar`(命中 'r')、`java -cp lib Main` / `-classpath`(命中
'c')、`java -ea`(命中 'e')全部被拒;同理 `pwsh -File gen.ps1`(命中 'e')、`bash -e gen.sh`(命中
'e')也被拒。这些正是该策略自己给出的允许替代方案("have the sub-executor generate the code in the writable Workspace and
execute the generated file instead")的典型执行形式——对 java/ps1 生成代码而言几乎不存在可通过的写法。方向上是
fail-closed(无越权风险),但属于功能性误拒。建议参照本文件 wrapperValueFlags 的做法为常用解释器建模短旗标的取值性(java 的
-jar/-cp/-classpath、pwsh 的 -File 等按普通旗标+attached-value 筛查处理),未知旗标再退回 c/e/r/m 启发式并保持
fail-closed,避免精确的 -c/-e/-r/-m 程序文本检测被削弱。

- func carriesProgramTextFlag(arg string) bool {
- 	for _, r := range arg[1:] {
- 		switch r {
- 		case 'c', 'e', 'r', 'm':
- 			return true
+ // 已知良性旗标(按解释器)先按普通旗标处理:attached 值仍经
+ // flagValueCandidates 筛查;未知旗标才退回 c/e/r/m 启发式并 fail-closed。
+ var benignInterpreterFlags = map[string]map[string]bool{
+ 	"java":    {"-jar": true, "-cp": true, "-classpath": true, "-ea": true},
+ 	"pwsh":    {"-File": true},
+ 	"powershell": {"-File": true},
+ }
+ 
+ // in Review(), pre-script region:
+ if benignInterpreterFlags[path.Base(req.Command[interpreterOffset(req.Command)])][arg] {
+ 	for _, value := range flagValueCandidates(arg) {
+ 		if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
+ 			return p.deny("interpreter_input", abs, "")
+ 		}
+ 	}
+ 	continue
- 		}
+ }


─── packages/views/package.json:29-30 ───
[maintainability · medium] 本次为 access/input-expand/usage 三个新面板登记了出口映射，但同批新增的
src/craft/workbench-edit.tsx 未加入 exports。已确认 apps/web/src/features/craft/routes.tsx:34 以深路径
`@weknora/views/craft/workbench-edit` 导入 CraftEditRequestPanel，目前仅靠 apps/web 的 tsconfig paths（126
行）与 vite 别名（119 行）逐路径兜底才可解析——绕过了包出口约定。后果：(1) 任何走 Node 风格 exports 解析的消费方（如以包说明符导入的 node --test
用例或其他包）将得到 ERR_PACKAGE_PATH_NOT_EXPORTED；(2)
与所有兄弟面板（全部登记）不一致，别名需在多个构建配置中重复维护。建议补登记出口条目，使别名退化为纯路径优化而非正确性依赖。

      "./craft/usage": "./src/craft/usage.tsx",
      "./craft/workbench": "./src/craft/workbench.tsx",
+     "./craft/workbench-edit": "./src/craft/workbench-edit.tsx",


─── packages/views/src/craft/input-expand.tsx:14-15 ───
[maintainability · low] 客户端硬编码归档扩展名白名单 ['.zip','.tar','.tgz','.gz']，与服务端
internal/modules/craft/archive.go:265 的 `[]string{".zip", ".tar", ".tgz", ".gz"}`
构成双份业务词表。当前两份恰好一致，但无编译期关联：服务端将来新增格式（如 .tar.gz/.bz2）时客户端入口不会出现（漏显示），反向则显示后被服务端拒绝。建议与
CRAFT_STOP_OUTCOMES 等冻结词表同纪律：在 @weknora/contracts 导出 CRAFT_ARCHIVE_EXTENSIONS，客户端谓词从该常量派生，消除漂移面。

- /** The server's own archive-extension vocabulary (craft/archive.go). */
- const CRAFT_ARCHIVE_EXTENSIONS = ['.zip', '.tar', '.tgz', '.gz'] as const;
+ // packages/contracts/src/craft/web-artifact.ts（与 CRAFT_STOP_OUTCOMES 同处冻结）
+ // export const CRAFT_ARCHIVE_EXTENSIONS = ['.zip', '.tar', '.tgz', '.gz'] as const;
+ 
+ // input-expand.tsx
+ import { CRAFT_ARCHIVE_EXTENSIONS } from '@weknora/contracts';
+ export function isExpandableArchiveInput(input: CraftInputView): boolean {
+   const name = input.name.toLowerCase();
+   return CRAFT_ARCHIVE_EXTENSIONS.some((extension) => name.endsWith(extension));
+ }


─── packages/views/src/craft/workbench-edit.tsx:166-168 ───
[maintainability · low] textarea 硬编码 id="craft-edit-prompt" 且 label 的 htmlFor 指向它。当前 routes.tsx 仅在
aside 槽位单实例挂载，暂无冲突；但该面板是可复用导出组件，一旦同页多实例（如多任务并排或响应式双布局），DOM id 重复会破坏 label 关联与无障碍语义。建议改用
React.useId() 生成实例级 id。另注：label 可见文本直接复用 placeholder 文案（"描述这次要做的修改…"），作为字段标签语义偏弱，宜用独立的字段名（如"修改说明"）。

-         <label className="wk-craft-hint" htmlFor="craft-edit-prompt">{strings.placeholder}</label>
-         <textarea
-           id="craft-edit-prompt"
+   const promptId = React.useId();
+   // ...
+   <label className="wk-craft-hint" htmlFor={promptId}>{strings.title}</label>
+   <textarea id={promptId} data-testid="craft-edit-prompt" ... />


LLM retry report summary: 5 of 260 requests affected -- 3 requests failed, 2 requests recovered after retry

Review planning (2 requests):
- internal/application/repository/craft_preview_check.go,internal/application/repository/craft_stop_intent.go,internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/modules/craft/archive.go,internal/modules/craft/citation.go,internal/modules/craft/contracts.go,internal/modules/craft/input_code.go,internal/modules/craft/lifecycle.go,internal/modules/craft/release.go,internal/modules/craft/version.go,internal/modules/craft/web_contracts.go: timed out -> failed

Core review (3 requests):
- docker/craft/Dockerfile,docker/craft/runtime-config.json,docker/craft/web/build.py,docker/craft/web/deps/craft-web.css,docker/craft/web/deps/craft-web.js,docker/craft/web/template.html,docker/craft/web/toolchain.lock.json,internal/container/craft_web_build.go,internal/container/craft_web_build_review.go,skills/craft-web-build/manifest.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/application/repository/craft_preview_check.go,internal/application/repository/craft_stop_intent.go,internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_preview_check.go,internal/application/repository/craft_stop_intent.go,internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go: rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
