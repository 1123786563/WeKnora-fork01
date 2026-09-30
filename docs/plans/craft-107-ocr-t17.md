Review complete: 14 finding(s) across 3 selected item(s).

─── internal/modules/craft/lifecycle.go:386-389 ───
[bug · high] StopIntentSuperseded 在"停止已确认"的标准观测上会返回 true，与自身文档及 StopIntentOutcome 直接矛盾。锁定的 OpenCode
投影中，被 abort 的消息同样携带 Completed=true：normalizer.go 快照路径仅在 best.completedAt>0 且 finish!="stop" 时才查
MessageAbortedError 并置 Aborted（executor_test.go:1017 的 abort 用例即 completed=5、finish 为空），live 路径 537
行同理。因此权威确认停止的观测是 {Aborted:true, Idle:true, Completed:true}，此处首析取项 o.Completed
命中，函数判定"被正常完成超越"——但文档明确要求 "completed WITHOUT the stop's abort"，且同一观测经 StopIntentOutcome 投影为
StopConfirmed。按本函数声明的用途（判定 lingering requested/unknown intent 已 stale 可清理），一次已落地的 abort
会被误判为正常完成，停止意图被清、Run 永不落 canceled、writer fence 不释放；当前唯一生产调用点（craft_control.go:796）虽以 !requested
兜底，但在该分支内也会把已 abort 的会话误投为 Phase="completed"。建议首析取项补 !o.Aborted 守卫；另可一并评估次析取项：仅 Idle 无 Aborted
也会把等待人工交互的 idle 会话当 settled。

- 	// A normal completion overtakes the stop: either the observation reports
- 	// the delegation Completed, or it settled idle without the stop's abort
- 	// ever landing. Domain-local — no executor-side normalizer import.
- 	return o.Completed || (!o.Aborted && o.Idle)
+ 	// A normal completion overtakes the stop only WITHOUT the stop's abort:
+ 	// the locked runtime marks an aborted message Completed as well
+ 	// (completedAt>0 with finish != "stop"), so Completed alone cannot
+ 	// separate a landed abort from a normal finish.
+ 	return !o.Aborted && (o.Completed || o.Idle)


─── internal/modules/craft/lifecycle.go:375-378 ───
[maintainability · medium] StopIntentOutcome 将"stop 已被请求"前提硬编码为 true 且不进签名，注释宣称的同源纪律（"an abort by
any OTHER mechanism ... must never project one"）在函数内无法落实——它收不到 requested
事实，只能信任调用方。唯一生产调用点恰好暴露了这一缺口：craft_control.go DelegationStatus 中 794 行 phase 用真实
requested（StopStatus(requested, ...)），795 行 fallback 却用本函数（pinned true）；当 intent 存储瞬时读错（760-764 行
Warnf 分支，requested 保持 false）而观测为 Aborted+Idle+未带 completedAt（live 路径 session.error 置 aborted
无需消息完成）时，803 行的同源守卫只修正 phase（且此时 phase 本就是 running，守卫空转），fallback 仍为 StopConfirmed；若
durableStopOutcome 再读失败或无 intent 行（460-468 行原样采用 fallback），只读轮询面将向 DTO 投影 fabricated
confirmed——恰是本函数文档禁止的结果。建议镜像 StopStatus 签名显式传入 requested，使该误用结构上不可行（调用点改为
StopIntentOutcome(requested, observation) 后此不对称即消除）。

- 	if StopStatus(true, o) == "canceled" {
+ 	if StopStatus(requested, o) == "canceled" {
  		return StopConfirmed
  	}
  	return StopRequested


─── internal/modules/craft/lifecycle.go:389-389 ───
[bug · high] `(!o.Aborted && o.Idle)` 这一析取支把"会话空闲"直接当作"正常完成压过了停止"，但空闲本身不构成完成证据：同一观测 `{Idle:true,
Aborted:false}` 上 StopIntentOutcome 返回 StopRequested（stop 仍在途，lifecycle_stop_t17_test.go:38
已钉死），两个新谓词对同一 Observation 给出互斥结论，必有一错。权威判定 opencode.Completed（调用点 craft_control.go:790 就在同一轮询里使用）要求
AssistantParentID 匹配、Finish=="stop"、!PendingTool 等证据，正是"空闲≠完成"的体现；此处为了"Domain-local"丢弃了全部完成证据，仅凭
Idle 就断言"the Run's terminal fact is its completion"。可达后果：生产调用点 796 行以 !requested 为门，持久 intent 为
StopUnknown（754 行不置 requested=true）或 intent 读瞬时失败（760 行）时即进入该分支——一个 abort 结果未定、run 行仍非终态的
Run，仅因会话空闲（如 abort 标记未落盘、turn 记录被裁剪、prompt 从未派发）就被投影为 Phase="completed"；而 durableStopOutcome 优先返回持久
intent，同一响应可同时携带 Phase=completed 与 Outcome=unknown，自相矛盾，违背本文件 T17 头注释"an outcome that could not be
determined stays unknown"。建议由调用方传入权威完成判定（opencode.Completed），域内谓词不再从空闲推导完成。

- 	return o.Completed || (!o.Aborted && o.Idle)
+ // completed is the authoritative executor verdict (opencode.Completed at
+ // the call site) — idleness alone is never completion evidence.
+ func StopIntentSuperseded(o Observation, completed bool) bool {
+ 	return completed && !o.Aborted
+ }


─── packages/views/src/craft/status-notice.tsx:32-32 ───
[bug · high] 该 re-export 引用的 `CraftStopOutcomeStatus` 并未从 `@weknora/contracts`
包入口导出：`packages/contracts/package.json` 的 exports 仅暴露 `"." → ./src/index.ts`，而根 index.ts 对 craft
模块只做点名转发（第 178-182 行的导出清单不含 `CraftStopOutcomeStatus`，`export *` 只用于
analytics/usage/query-history；该类型目前仅存在于 `craft/index.ts` 的 `export * from './web-artifact.ts'`
内部命名空间）。因此此处 `import('@weknora/contracts').CraftStopOutcomeStatus` 在 moduleResolution: Bundler
下无法解析（TS2305/TS2724），注释宣称的"与投影源 compile-time linked"并未成立。当前之所以未被拦截，是因为根 package.json 的
`typecheck:shared` 文件清单不含本文件、stop 测试经 tsx 运行不检查类型、且尚无生产代码 import 本文件——一旦 T20 把 CraftStopNotice 接入
workbench（或本文件进入任何 tsc 程序）即会编译失败。建议在 `packages/contracts/src/index.ts` 的 craft 点名转发清单中补充导出该类型（及
`CRAFT_STOP_OUTCOMES`，与既有 178/179 行模式一致、纯增量），再保留此处的 re-export；同时建议将本文件纳入 typecheck 覆盖以防再回归。

- export type CraftStopOutcomeStatus = import('@weknora/contracts').CraftStopOutcomeStatus;
+ // packages/contracts/src/index.ts（第 178 行清单追加，纯增量）:
+ // export type { ..., CraftStopOutcomeStatus } from './craft/index.ts';
+ // export { ..., CRAFT_STOP_OUTCOMES } from './craft/index.ts';
+ 
+ // 本文件保持:
+ import type { CraftStopOutcomeStatus as ContractStopOutcomeStatus } from '@weknora/contracts';
+ export type CraftStopOutcomeStatus = ContractStopOutcomeStatus;


─── internal/application/service/craft_control.go:689-693 ───
[bug · high] 确认标记持久化失败后没有任何修复路径,且该重放分支主动跳过修复。链条:(1) 首次 Stop 在 runs.Cancel 成功后
PutStopIntent(confirmed) 失败(701-708 行)或进程恰在两次持久写之间崩溃——这正是本注释自己描述的场景——此时 run 行已 canceled 但意图行停留
requested;(2) 成员再次 Stop 重试时走到本分支,直接 confirmedReplay() 返回,不尝试回补 confirmed 标记;(3) 之后所有
DelegationStatus 轮询:意图行非 confirmed(747 行快路径不触发)、run.Status=="canceled" 使 requested=true,最终
durableStopOutcome 读到 requested——永久投影出 Phase=canceled +
Outcome=requested("已请求停止,正在等待执行器确认…")的跨面矛盾,注释宣称的 "the durable answer is confirmed either way"
与实际持久事实相悖。建议在本分支回补标记后再重放。

  				case "canceled":
  					// The CAS already ran (an earlier stop crashed between
- 					// the CAS and the confirmed marker): the durable
- 					// answer is confirmed either way.
+ 					// the CAS and the confirmed marker): back-fill the
+ 					// confirmed marker so the durable answer matches the
+ 					// replay — otherwise every later poll projects
+ 					// Outcome=requested next to Phase=canceled forever.
+ 					_, _ = s.stopIntents.PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
+ 						RunID: req.RunKey.RunID, Status: craft.StopConfirmed,
+ 					})
  					return confirmedReplay(), nil


─── internal/application/service/craft_control.go:701-709 ───
[bug · medium] 确认标记写入失败(merr != nil)时 Stop 返回 Phase=canceled + Outcome=StopConfirmed,但持久意图行停留
requested:观察层面 confirmed 是诚实的,可随后的 DelegationStatus 以持久意图为权威,将同一 run 投影为 Phase=canceled +
Outcome=requested,两面对外答案不一致且(结合上一个 case "canceled" 重放分支不回补标记的问题)永不收敛。建议在 Note
之外考虑同时投影请求重试,或至少让重试路径具备修复能力(见上一条)。



─── internal/application/service/craft_control.go:0-0 ───
[bug · medium] 该分支复用 confirmedReplay(),返回 Note "stop already confirmed; replaying the durable
answer" 与 Outcome=StopConfirmed——与本行内注释 "without claiming a stop journey" 自相矛盾,也违背 531-535
行自己写明的纪律("conflating them fabricates a stop confirmation")。前端 status-notice.tsx 会把 confirmed
渲染为已停止横幅,把会话删除/通用 cancel 误报为"停止已确认";且 DelegationStatus 对同一 run(run canceled、无意图行)经
durableStopOutcome 回退投影 Outcome=requested,形成 Stop 与轮询两面的答案分叉。建议不复用 confirmedReplay,给出独立措辞的应答。

  		if run.Status == "canceled" && intentErrNotFound(ierr) {
  			// The run was canceled by a NON-stop path with no stop intent on
- 			// record — replay the row's terminal fact without claiming a stop
+ 			// record — replay the row's terminal fact WITHOUT claiming a stop
  			// journey (and without aborting again).
- 			return confirmedReplay(), nil
+ 			return CraftStopStatus{Phase: "canceled",
+ 				Note:    "run already canceled by a non-stop path; nothing to stop",
+ 				Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed}}, nil
  		}


─── internal/application/service/craft_control.go:803-807 ───
[bug · medium] 这个守卫是不可达死代码:craft.StopStatus(interaction.go:42-50)只在 requested==true 时返回
"canceled",所以 phase=="canceled" 必然 requested==true,条件永假;而它想防御的真实场景——!requested 且观察为
Aborted+Idle(预算暂停、执行器侧会话删除等非停止机制的 abort)——实际产出 phase=StopStatus(false,…)="running",同时
fallback=StopIntentOutcome(observation)=StopConfirmed,再经 durableStopOutcome(意图行 NotFound 或瞬时读失败时取
fallback)返回 Phase="running" + Outcome=StopConfirmed 的自相矛盾 DTO,直接违反 StopIntentOutcome 自己注释的
same-source 纪律("an abort by any OTHER mechanism … must never project one")。建议改为在 !requested 时钳制
fallback。

- 			if phase == "canceled" && !requested {
+ 			if !requested && fallback == craft.StopConfirmed {
  				// The observation alone cannot claim a stop cancellation the
- 				// durable intent never confirmed (same-source discipline).
- 				phase = "stopping"
+ 				// durable intent never confirmed (same-source discipline):
+ 				// StopStatus never reports "canceled" without requested, so
+ 				// clamp the outcome fallback instead of the unreachable phase.
+ 				fallback = craft.StopRequested
  			}


─── internal/application/service/craft_control.go:203-208 ───
[bug · medium] stopIntents 是普通接口字段,SetStopIntents 做构造后写入,而 Stop/DelegationStatus 在请求 goroutine
上无同步读取。同一 struct 中完全相同注入模式的 executor 专门用 atomic.Pointer 规避该窗口(SetExecutor/currentExecutor);文档 ledger
记载 T20 将在生产容器按同一时机装配该 seam,届时若注入与存活请求交叠即构成数据竞争(Go 内存模型下是未定义行为)。建议对齐本文件既有并发风格,改用 atomic.Pointer。

  func (s *CraftControlService) SetStopIntents(store CraftStopIntentStore) {
  	if s == nil || store == nil {
  		return
  	}
- 	s.stopIntents = store
+ 	s.stopIntents.Store(&store)
  }
+ 
+ // 配套:字段改为 stopIntents atomic.Pointer[CraftStopIntentStore],
+ // 读取处统一走 if held := s.stopIntents.Load(); held != nil { … *held … }


─── internal/application/service/craft_control.go:145-146 ───
[documentation · low] GetStopIntent 的接口文档未钉死 not-found 错误契约,而 intentErrNotFound 及
Stop/DelegationStatus 的全部分支判定都依赖 errors.Is(err, craft.ErrNotFound)。生产实现(T20 所有)若返回裸
sql.ErrNoRows/datastore 错误,所有"无意图行"读取都会被判为瞬时故障:Stop 会向已 canceled 的 run 重复落 requested 意图并再次
abort,confirmedReplay 快路径永不触发,durableStopOutcome 一律退化为 fallback。接口是 T20 实现方的唯一契约来源,建议在此显式钉死。

- 	// GetStopIntent reads one Run's durable stop intent.
+ 	// GetStopIntent reads one Run's durable stop intent. It MUST return
+ 	// an error wrapping craft.ErrNotFound when no intent row exists —
+ 	// callers distinguish "no row" from a transient store failure via
+ 	// errors.Is(err, craft.ErrNotFound).
  	GetStopIntent(context.Context, craft.Scope, string) (craft.StopIntent, error)


─── internal/application/service/craft_control.go:812-813 ───
[bug · low] 尾部兜底(及 770/792 行的 completed 分支)在无意图行(NotFound)时无条件回退 StopRequested:该状态端点是通用的 GET
/sessions/:id/craft/runs/:run_id/delegations/:task_id/status,一个从未请求停止、正常执行中的 delegation 被轮询也会投影出
Outcome=requested("已请求停止,正在等待执行器确认…"),凭空捏造"成员请求过停止"这一事实;一旦 T20 把 Outcome 序列化到前端 CraftStopNotice
横幅即成为用户可见的误导。建议仅在存在停止旅程证据(意图行存在、run 行 canceled 或 requested=true)时投影 Outcome,否则返回零值 Outcome。



─── internal/application/service/craft_control.go:546-547 ───
[documentation · low] T17 分支把 runs.Cancel(旧路径中"使所有围栏写失败、从而阻断一切新 dispatch"的机制)推迟到确认之后,而本次变更内(含
lifecycle.go 新增的 66 行辅助类型)没有任何组件消费停止意图来阻断 dispatch——意图 store 的消费方仅本文件的 Stop/DelegationStatus。ledger
已把"dispatch 阻断改查持久 stop intent"与容器装配 SetStopIntents 都排给 T20 同批落地,因此建议在此处补一行注释显式记录该耦合,避免 T20 只装配
store 而不带 dispatch 阻断时,成员请求停止后 run 在 abort 生效前的窗口内仍可派发新工作。

  		// 1. The stop intent persists BEFORE anything is aborted: an accepted
- 		//    stop survives every later failure.
+ 		//    stop survives every later failure. NOTE: unlike the pre-T17
+ 		//    branch below, nothing here blocks new dispatch until T20 lands
+ 		//    the intent-aware dispatch check — the run row stays nonterminal
+ 		//    by design and the window is bounded by the Abort in step 3.


─── internal/application/service/craft_control.go:624-627 ───
[bug · medium] 此处无条件 PutStopIntent(StopUnknown) 会把行级已终态确认的停止永久降级为 unknown/stopping，且无收敛路径。链条：首次 Stop
在 runs.Cancel(676 行)成功后、confirmed 标记写入前崩溃或标记写入失败(701-708 行，即 689-693 行注释自述的场景)——run 行已
canceled、intent 停留 requested；成员重试 Stop 时，537 行(intent≠confirmed)与 540 行(要求 intentErrNotFound，此处
ierr==nil)都不早退，流程继续再次 Abort+Observe；已被停止/清理的会话 Observe 失败后，本分支把 intent 从 requested 降级写入 unknown 并回答
Phase=stopping。此后 DelegationStatus：747 行快路径(intent==confirmed)永不触发，778-786 行把 unknown 原样回放——run
行明明已终态 canceled，轮询面却永久显示 "stopping + 停止结果不明，等待核对"，再次重试 Stop 又循环复写 unknown。与已确认问题 #1(case "canceled"
重放分支跳过修复)不同，这条是本分支对已终态事实的主动降级。建议在写 unknown 前检查 run.Status：已 canceled 时跳过降级(并宜顺势回补 confirmed 标记，与 #1
的修复方向一致)，仅对行级仍非终态的 run 记录 unknown。

- 		if s.stopIntents != nil {
+ 		if s.stopIntents != nil && run.Status != "canceled" {
+ 			// Only a still-nonterminal run records the durable unknown; a
+ 			// terminally canceled row replays its confirmed fact instead of
+ 			// downgrading the stop journey to unknown.
  			if _, uerr := s.stopIntents.PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
  				RunID: req.RunKey.RunID, Status: craft.StopUnknown,
  			}); uerr == nil {


─── internal/application/service/craft_control.go:794-802 ───
[bug · medium] superseded 收敛对 Outcome 投影完全无效，悬空 intent 在两条表面上永久错配。两个缺口：(1) 795 行把 fallback 传入
durableStopOutcome(452-469 行)，但后者在 intent 行读取成功时直接返回 intent.Status、丢弃 fallback——而本分支可达的前提恰是
intent=StopUnknown 且读取成功(754 行对 unknown 不置 requested)，因此 800 行的 fallback=StopRequested 是死赋值，最终仍投影
Outcome=unknown("停止结果不明，等待核对")与 Phase=completed 永久并存，正是 797-799 行注释声称要消除的错配；(2)
更常见的陈旧形态——intent=requested 的停止被 succeeded/failed 正常完成超越——因 754 行将 requested 置
true，本分支(!requested)永不可达，768-770、790-792、812-813 行及 Stop 的 524-529、579-583、656-661 行全部投影
Outcome=requested("已请求停止，正在等待执行器确认…")与 Phase=completed/failed 并存，且没有任何路径退役该 intent(Stop 对终态 run
早退不写)。上一轮评审要求的"完成/失败路径覆写或退役悬空 intent"实际未落地：谓词只修了 phase。建议以行级终态为准判定超越(如
craft.WriterRunTerminal(run.Status) && run.Status != "canceled"，存储的 succeeded/failed 委托结果同理)，命中时绕过陈旧
intent 行改投 superseded 后的诚实结果(或在该等路径退役/覆写 intent 行)。

- 			phase := craft.StopStatus(requested, observation)
- 			fallback := craft.StopIntentOutcome(observation)
+ 			superseded := craft.WriterRunTerminal(run.Status) && run.Status != "canceled"
  			if !requested && craft.StopIntentSuperseded(observation) {
- 				// A normal completion overtook a never-confirmed stop: the
- 				// durable intent is stale and projecting it would show
- 				// "waiting for the executor" next to Phase=completed forever.
- 				fallback = craft.StopRequested
+ 				superseded = true
+ 			}
+ 			if superseded {
+ 				// A non-canceled terminal outcome overtakes the stop: the
+ 				// durable intent is stale — project the superseded fact, not
+ 				// the lingering intent row (durableStopOutcome would replay it).
  				phase = "completed"
+ 				return CraftStopStatus{Phase: phase,
+ 					Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
  			}


LLM retry report summary: 1 of 25 requests affected -- 1 request recovered after retry

Core review (1 request):
- internal/application/service/craft_control.go: rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
