Review complete: 12 finding(s) across 3 selected item(s).

─── internal/modules/craft/lifecycle.go:382-384 ───
[maintainability · medium] StopIntentMayWriteRunTerminal 是导出的域门控，但全库检索显示它没有任何生产调用点（仅
lifecycle_stop_t17_test.go / craft_stop_t17_test.go 引用）：本次 T17 流中真正写 canceled 终态的位置（service 层确认分支的
runs.Cancel）是以观察相位 craft.StopStatus(true, observation)=="canceled" 门控的，并未查阅持久 intent 状态。也就是说"仅
confirmed 可写 Run 终态"这条本函数文档声明的不变量，当前只是由另一处（StopStatus 的 Aborted&&Idle
条件，interaction.go:43）附带成立——confirmed 判定条件现在存在两份独立编码（StopStatus 与
StopIntentOutcome），再加一个未被调用的门控谓词，三者漂移时不会被编译器或任何调用链发现。建议在终态写入点实际接入该谓词，或在文档注释中明确标注接线时点（如
T20），避免它被误认为已在生产路径生效。

+ // StopIntentMayWriteRunTerminal reports whether the durable stop state of a
+ // Run permits writing the terminal canceled status on the Run row. Only a
+ // confirmed stop does: requested and unknown are nonterminal — the accepted
+ // HTTP response must never terminalize the Run, and the writer fence and
+ // the promotion gate stay in force until the authoritative outcome.
+ // NOTE(T20): the terminal write site must consult this predicate; today the
+ // confirmed write is gated by StopStatus(observation) == "canceled" only.
  func StopIntentMayWriteRunTerminal(status StopOutcomeStatus) bool {
  	return status == StopConfirmed
  }


─── internal/modules/craft/lifecycle.go:370-375 ───
[bug · medium] StopIntentOutcome 对"停止被正常完成超越"的观察（o.Completed 为真，或 !Aborted && Idle）仍返回 requested，而
T00 词汇表内又没有第四态可表达该终局；结合 service 层以持久 intent 为投影权威（durableStopOutcome 优先返回
intent.Status），一次停止请求与正常完成竞态后，intent 行将永久停留 requested：断线重连/刷新按持久 intent 重建投影时，已终态（succeeded/failed）的
Run 会永远携带 outcome=requested（前端文案"已请求停止，正在等待执行器确认…"），与 Phase=completed 并存；轮询面上甚至可能出现
Phase="canceled"（来自观察）而 Outcome="requested"（来自持久 intent）的分裂投影。域层缺少一条"Run 由其他终态路径落定后收敛
requested/unknown intent"的规则，建议补充显式收敛谓词并在完成/失败路径上覆写或退役悬空 intent。

- func StopIntentOutcome(o Observation) StopOutcomeStatus {
- 	if o.Aborted && o.Idle {
- 		return StopConfirmed
- 	}
- 	return StopRequested
+ // StopIntentSuperseded reports whether a persisted requested/unknown stop
+ // intent is superseded by a terminal Run outcome written by another path
+ // (a normal completion or failure that raced the stop): the projection
+ // must retire the intent instead of replaying "requested" forever.
+ func StopIntentSuperseded(runStatus string) bool {
+ 	return WriterRunTerminal(runStatus) && runStatus != "canceled"
  }


─── internal/modules/craft/lifecycle.go:370-375 ───
[bug · medium] StopIntentOutcome 缺少"停止已被请求"这一前提，与同包 StopStatus(requested, o) 的 canceled
判定不同源：StopStatus 要求 requested && o.Aborted && o.Idle，而本函数无条件把 Aborted && Idle 映射为
StopConfirmed。实际调用点（craft_control.go DelegationStatus ~758-760 行）在同一响应里分别用两者计算 Phase 与 Outcome：当
requested 为 false（pre-T17 装配 stopIntents==nil 时 requested 仅为 run.Status=="canceled"；或 GetStopIntent
瞬时出错且 run 非 canceled）而观察恰为 Aborted && Idle（如预算暂停等其它机制触发的 abort、或异常终止后的残留观察）时，同一响应会给出
Phase=StopStatus(false,o)="running" 但 Outcome.Status=StopConfirmed
的自相矛盾投影——一次未被请求（或未被持久记录）的停止被谎报为已确认，前端三态文案将显示错误状态。建议为函数补充 requested 前提参数（requested=false 时返回
StopRequested），或直接委托 StopStatus(true, o)=="canceled" 复用同一判定，顺带消除两处独立维护的确认条件漂移风险。

- func StopIntentOutcome(o Observation) StopOutcomeStatus {
- 	if o.Aborted && o.Idle {
+ func StopIntentOutcome(requested bool, o Observation) StopOutcomeStatus {
+ 	if requested && o.Aborted && o.Idle {
  		return StopConfirmed
  	}
  	return StopRequested
  }
+ // 或与 StopStatus 共用同一判定：
+ // func StopIntentOutcome(requested bool, o Observation) StopOutcomeStatus {
+ // 	if requested && StopStatus(requested, o) == "canceled" {
+ // 		return StopConfirmed
+ // 	}
+ // 	return StopRequested
+ // }


─── packages/views/src/craft/status-notice.tsx:30-30 ───
[maintainability · medium] T00 冻结契约 packages/contracts/src/craft/web-artifact.ts 已公开导出同名的
CraftStopOutcomeStatus（CRAFT_STOP_OUTCOMES 派生的 'requested' | 'confirmed' | 'unknown'，经
craft/index.ts 的 export * 可从 @weknora/contracts 导入）。此处手工重复声明该联合类型，导致：(1) 投影数据源（契约的
CraftStopOutcome.status，T20 接线时）与横幅词表之间失去编译期关联，词表演进时会静默漂移；(2) 两个同名类型因导入路径不同而产生歧义，下游可能误从 views 导入。同目录
share.tsx/home.tsx/library.tsx 均已有从 @weknora/contracts 导入冻结契约类型（含运行时词表数组）的既定先例，建议直接复用契约导出。

- export type CraftStopOutcomeStatus = 'requested' | 'confirmed' | 'unknown';
+ // 顶部 import 处增加：
+ import type { CraftStopOutcomeStatus } from '@weknora/contracts';
+ 
+ // 如需保留本模块对外导出面，可改为类型再导出而非重新声明：
+ export type { CraftStopOutcomeStatus };


─── packages/views/src/craft/status-notice.tsx:32-36 ───
[maintainability · low] 同一文件内重复硬编码成员可见文案 '已停止'：STATUS_NOTICE_TEXT.canceled（第 11 行）与
STOP_OUTCOME_NOTICE_TEXT.confirmed 表达的是同一个成员可见事实（任务已停止，且此处 kind 也恰好映射为
'canceled'）。两处独立维护会在文案演进时漂移——例如产品统一改为「任务已停止」时只改一处，confirmed 停止结果与 canceled 生命周期横幅对同一状态展示两种措辞。建议直接复用
STATUS_NOTICE_TEXT.canceled 建立显式关联。

  const STOP_OUTCOME_NOTICE_TEXT: Record<CraftStopOutcomeStatus, string> = {
    requested: '已请求停止，正在等待执行器确认…',
-   confirmed: '已停止',
+   confirmed: STATUS_NOTICE_TEXT.canceled,
    unknown: '停止结果不明，等待核对',
  };


─── internal/application/service/craft_control.go:527-533 ───
[bug · high] T17 窗口内停止请求失去了"阻断继续执行/派发"的手段。pre-T17 依赖立即 runs.Cancel 使所有 fenced write
失败从而阻断新派发（被删除的注释即此语义）；T17 把 run 行终态化推迟到确认之后，而持久化 stop intent 目前没有任何围栏/派发消费方（GetStopIntent
在生产代码中仅本文件出现；craft_workspace.go:564 observeCraftWriterRun 只读 run 行状态）。因此 PutStopIntent(requested)
之后若 exec.Abort 未送达（失败仅记入 abortNote、不阻断流程），run 仍为 running、其 fenced
写与新派发照常成功，被接受的停止对仍在执行的委托没有权威约束，与"服务端维持权威停止"的规约相悖。建议：在 fenced write/派发闸门处消费持久化
intent（requested/unknown 即拒绝该 run 的新派发与写），或为围栏引入可消费的非终态 stopping 事实。



─── internal/application/service/craft_control.go:518-522 ───
[bug · high] run.Status=="canceled" 并非停止旅程独有的事实：craft_lifecycle.go:99（会话删除的 CancelSessionRuns）与通用
CancelAgentRun 端点（handler/session/agent_run.go:281）都经同一 run 控制器把 run 行置为
canceled。此短路把这些场景一律误判为"停止已确认"：返回伪造的 Outcome=StopConfirmed（持久化 intent 中并无 confirmed 记录，Note 还声称
replaying the durable answer），并跳过对可能仍在运行的委托子执行的 Abort。建议：仅当持久化 intent 为 StopConfirmed 时才回放
confirmed，否则继续后续流程（确认段的 runs.Cancel 冲突分支已能正确处理已 canceled 的行）。

- 		// marker): the repeated stop replays the confirmed answer and never
- 		// aborts again.
  		if run.Status == "canceled" {
+ 			if intent, ierr := s.stopIntents.GetStopIntent(detachCtx, req.Scope, req.RunKey.RunID); ierr == nil &&
+ 				intent.Status == craft.StopConfirmed {
- 			return confirmedReplay(), nil
+ 				return confirmedReplay(), nil
+ 			}
+ 			// A non-stop cancellation (session deletion, the generic run
+ 			// cancel): the stop journey still aborts the sub-execution below;
+ 			// the confirmation CAS tolerates the already-canceled row.
  		}


─── internal/application/service/craft_control.go:758-760 ───
[bug · medium] 持久化确认只有"再次调用 Stop"这一条推进路径：DelegationStatus 按设计只读、unknown 分支注释里的 awaits reconciliation
没有任何实现。停止返回 "stopping" 后若客户端只轮询，run 永久非终态、writer fence 永久持有。且此处 phase 来自实时观察（aborted+idle 即
"canceled"）而 Outcome 来自仍为 requested 的持久化 intent，同一响应会出现 phase="canceled" + Outcome=requested
的矛盾组合；观察不可用（如刷新后无 executor）时又漂回 "stopping"，与"页面刷新读到同一持久事实"的注释矛盾。建议：轮询 phase 与持久化 outcome 同源（intent 非
confirmed 时不报 canceled），并补充 unknown→confirmed 的对账路径或明确只有重复 Stop 才推进确认。



─── internal/application/service/craft_control.go:683-687 ───
[maintainability · low] pre-T17 装配下本函数其余分支都回填了 StopRequested fallback，唯独紧随其后（未变更）的 `return
CraftStopStatus{Phase: phase}, nil` 返回零值 Outcome（Status 为 ""），同一 DTO 出现第四种取值；一旦 T20 把 Outcome
接上序列化，前端 CraftStopNotice 的 Record 查表（packages/views/src/craft/status-notice.tsx，键只有
requested/confirmed/unknown）对 "" 会渲染为无文案的 canceled 横幅。建议该尾分支回填 StopConfirmed（此分支 phase 即
"canceled"），保持词汇表封闭。

- 		return CraftStopStatus{
- 			Phase:   "canceled",
- 			Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed},
- 		}, nil
- 	}
+ 	// 尾分支（pre-T17 确认路径）保持 DTO 一致：
+ 	return CraftStopStatus{Phase: phase,
+ 		Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed}}, nil


─── internal/application/service/craft_control.go:723-728 ───
[bug · low] ierr != nil（意图存储读失败）时 requested 保持 false：存储抖动的瞬间轮询面从 "stopping" 闪回 "running"，与本变更反复强调的
fail-closed 降级叙事相反；而同一函数里 Observe 失败且 intent=requested 时又把原始错误直接上抛——同一故障面给出两种不同呈现。建议区分 ErrNotFound
与其他读错误：非 NotFound 时上抛（或保守保持 requested=true）。

  		case ierr == nil && run.Status != "canceled":
- 			// The persisted intent (requested, or an unresolved unknown) is
- 			// what tells the poll the member asked to stop: the T17 run row
- 			// stays nonterminal until the confirmation, so the stopping
- 			// status cannot come from the run row alone.
  			requested = true
+ 		case ierr != nil && !errors.Is(ierr, craft.ErrNotFound):
+ 			// 意图存储读故障：如实上抛，而不是悄悄当作"未请求停止"
+ 			return CraftStopStatus{}, ierr
+ 		}


─── internal/application/service/craft_control.go:453-458 ───
[maintainability · low] GetStopIntent 的错误被静默吞掉并降级为 fallback：瞬时故障下可能把已 confirmed 的停止投影成
requested，与函数自身"persisted intent is the authority"的注释相悖；且 ErrNotFound（从未停止过）与其他错误不加区分——从未停止的 run
也会被投影为 requested（"成员已请求停止"）。错误既不上抛也不记录日志，排障时不可见。建议：非 ErrNotFound 的失败至少记 Warnf，语义上仅在确实无意图行时使用
fallback。

  	if s.stopIntents != nil {
- 		if intent, err := s.stopIntents.GetStopIntent(ctx, scope, runID); err == nil {
+ 		intent, err := s.stopIntents.GetStopIntent(ctx, scope, runID)
+ 		switch {
+ 		case err == nil:
  			return craft.StopOutcome{RunID: runID, Status: intent.Status}
+ 		case !errors.Is(err, craft.ErrNotFound):
+ 			logger.Warnf(ctx, "craft: stop intent read failed for run %s: %v", runID, err)
  		}
  	}
  	return craft.StopOutcome{RunID: runID, Status: fallback}


─── internal/application/service/craft_control.go:621-627 ───
[bug · high] T17 快速中止场景下确认写入被本分支吞掉，run 行永久非终态。机制：真实
opencode.Executor.Abort（executor.go:367-375）在内联快照确认 Aborted 时会调用 SaveResult 写入 Status:"canceled"
的委托结果；该写入经 lockToolRun→fenced（agent_run.go:1270-1274）要求 run 行仍为 running/recovering。pre-T17 先
runs.Cancel 使该写入必然失败（ErrLeaseLost，结果从不落库）；而 T17 刻意把终态化推迟到确认之后，Abort 执行时 run 行仍是 running，于是 canceled
结果成功落库。随后本处（Observe 之后、确认尾部之前）的 GetResult 读到该结果即提前返回：Phase 报 "canceled"、Note 谎称 "completed
normally"、Outcome 停在 requested，而 runs.Cancel（终态 CAS）与 PutStopIntent(confirmed)（行 645-687）永远不执行——run
行永久非终态、writer fence 永久持有、intent 永久 requested。且重复 Stop 在更早的 step-2 GetResult（行
556）就被同一结果短路，该状态无法通过停止面自愈。T17 旅程测试使用不写结果的 controlExecutor fake，未覆盖此路径。建议：T17 路径下
result.Status=="canceled" 不得从本分支退出——它是 executor 自身的中止确认，应流转到下方的终态写入+confirmed 标记（并修正 Note 文案）；step-2
分支同理需放行 canceled 结果以允许重试推进确认。

  		if result, rerr := s.store.GetResult(detachCtx, req.Scope, req.TaskID); rerr == nil {
+ 			if s.stopIntents == nil || result.Status != "canceled" {
- 			return CraftStopStatus{
+ 				return CraftStopStatus{
- 				Phase: stopPhaseForResult(result), Result: &result,
+ 					Phase: stopPhaseForResult(result), Result: &result,
- 				Note:    "the delegation completed normally while the stop was in flight; the original result is preserved",
+ 					Note:    "the delegation completed normally while the stop was in flight; the original result is preserved",
- 				Outcome: requestedOutcome(),
+ 					Outcome: requestedOutcome(),
- 			}, nil
+ 				}, nil
+ 			}
+ 			// Under T17 the executor persists the canceled result while the
+ 			// run row is still nonterminal: that is its own abort confirmation
+ 			// — fall through to the terminal write and the confirmed marker
+ 			// below instead of exiting with Outcome=requested.
  		}


LLM retry report summary: 3 of 48 requests affected -- 3 requests recovered after retry

Core review (3 requests):
- internal/application/service/craft_control.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_control.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_control.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
