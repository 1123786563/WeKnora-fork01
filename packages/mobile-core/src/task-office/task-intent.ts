/**
 * TaskHandle.act 的意图合同（module-seams §5.2：act(TaskIntent) 执行 steer、queue-next、
 * stop 等受控意图；decision 已由 #38 的 office.decide 承载，share 属后续 Issue）。
 * 三个意图对应三种用户可明确表达的干预：
 * - steer：调整当前 Run（安全点注入）；
 * - queue-next：为下一 Run 排队一条指令——Run 仍在运行时由模块本地持有（单写者规则：
 *   活动 Run 占据该 Task 的写通道，服务端对活动 Run 的 queue_next 是 409），观察到终态
 *   后发出；本地持有非耐久（跨进程耐久属 #40，且持久化选型 ADR 未决，本模块不引入）；
 * - stop：停止当前 Run——请求、确认、结果未知三态分别呈现（Issue #37 AC1）。
 */
export type TaskIntent =
  | { kind: 'steer'; text: string }
  | { kind: 'queue-next'; text: string; intentId?: string }
  | { kind: 'stop' };

/** 停止三态：requested=请求已被服务端接受（202）、尚未观察到终态；confirmed=已观察到
 * canceled（或自然终态获胜——正常完成从不被改写为取消）；unknown=命令投递结果未知，
 * 核对前阻止同句柄一切后续写意图（Issue #37 AC2）。 */
export type StopPhase = 'requested' | 'confirmed' | 'unknown';

export type InterventionOutcome = 'accepted' | 'parked' | 'conflict' | 'unknown';

/** 一次干预的诚实回执：boundRunId 是指令实际绑定的 Run（AC「看到指令实际绑定的 Run」）；
 * queue-next 已被服务端准入下一 Run 时携带 nextRunId；revision 是命令携带的真实观察
 * revision（202 后的 +1 是服务端 CAS 证明，同样真实，绝无编造值）。 */
export interface InterventionReceipt {
  intent: TaskIntent;
  outcome: InterventionOutcome;
  boundRunId: string;
  revision: number;
  nextRunId?: string;
  note?: string;
  at: string;
}

/** unknown 的核对规则（快照事实驱动，取消不可逆）：观察到 canceled ⇒ 停止其实已落地；
 * 其余任何状态 ⇒ 取消 CAS 不可能已落地 ⇒ 未落地（门解除，用户可重试）。 */
export function resolveUnknownStop(observedRunStatus: string): 'confirmed' | 'not-landed' {
  return observedRunStatus === 'canceled' ? 'confirmed' : 'not-landed';
}
