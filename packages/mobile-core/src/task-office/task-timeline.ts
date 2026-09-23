/**
 * Task 详情纯投影（module-seams §5：Task Office 拥有三层状态与 Timeline 投影）。
 * 三层状态依据 CONTEXT.md：任务生命周期（进行中/已完成/已取消/已归档）、
 * Run 状态（run_status）与关注状态（attention）分别存在；终态推进规则逐字
 * 镜像服务端 agent_run_snapshot.go projectExecutionEvents，不自行发明。
 * Timeline 是持久事件的事实流投影：已知类型分类，未知类型保留原文归入
 * activity（durable ingestion 不丢未知类型，投影同样不丢）；原始证据默认
 * 折叠、按需展开（CONTEXT.md「任务时间线」）。
 */
export type TaskLifecycleState = 'active' | 'completed' | 'canceled' | 'archived';
export type TaskTimelineKind = 'run_status' | 'conclusion' | 'tool_activity' | 'approval' | 'artifact' | 'activity';

export interface TaskTimelineSourceEvent {
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export interface TaskTimelineEntry {
  seq: number;
  occurredAt: string;
  kind: TaskTimelineKind;
  type: string;
  summary: string;
  evidence: { payload: Record<string, unknown> };
}

export function taskLifecycleOf(archivedAt: string | undefined, runStatus: string): TaskLifecycleState {
  if (archivedAt !== undefined && archivedAt !== '') return 'archived';
  if (runStatus === 'succeeded') return 'completed';
  if (runStatus === 'canceled') return 'canceled';
  return 'active';
}

export function isTerminalRunStatus(status: string): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'canceled';
}

/** 镜像 internal/application/repository/agent_run_snapshot.go:104-134 的推进与优先级。 */
export function terminalRunStatusOf(base: string, events: ReadonlyArray<{ type: string }>): string {
  if (base !== 'queued' && base !== 'running' && base !== 'reconciling') return base;
  let succeeded = false;
  let failed = false;
  let canceled = false;
  for (const event of events) {
    if (event.type === 'run.completed' || event.type === 'execution.succeeded' || event.type === 'status.succeeded') succeeded = true;
    else if (event.type === 'run.failed' || event.type === 'execution.failed' || event.type === 'status.failed') failed = true;
    else if (event.type === 'run.canceled' || event.type === 'execution.canceled' || event.type === 'status.canceled') canceled = true;
  }
  if (canceled) return 'canceled';
  if (failed) return 'failed';
  if (succeeded) return 'succeeded';
  return base;
}

/** App 重启合并：按 seq 去重升序；同 seq 以权威 snapshot 载荷为准；高于水位的持久行是损坏缓存，不得回放。 */
export function mergeEventHistory<E extends { seq: number }>(persisted: readonly E[], snapshot: readonly E[], watermark: number): E[] {
  const unique = new Map<number, E>();
  for (const event of [...persisted, ...snapshot]) {
    if (event.seq > watermark) continue;
    unique.set(event.seq, event);
  }
  return [...unique.values()].sort((a, b) => a.seq - b.seq);
}

const KIND_OF_TYPE: Record<string, TaskTimelineKind> = {
  'run.started': 'run_status', 'run.status': 'run_status', 'run.failed': 'run_status', 'run.canceled': 'run_status',
  'attempt.started': 'run_status', 'attempt.replaced': 'run_status', 'attempt.finished': 'run_status',
  'execution.failed': 'run_status', 'status.failed': 'run_status', 'execution.canceled': 'run_status', 'status.canceled': 'run_status',
  'run.completed': 'conclusion', 'execution.succeeded': 'conclusion', 'status.succeeded': 'conclusion',
  'tool.started': 'tool_activity', 'tool.planned': 'tool_activity', 'tool.completed': 'tool_activity', 'tool.result': 'tool_activity',
  'interaction.required': 'approval', 'decision.required': 'approval',
  'artifact.created': 'artifact', 'artifact.available': 'artifact',
};

const EVENT_SUMMARIES: Record<string, string> = {
  'run.started': '任务已开始', 'run.completed': '任务已完成', 'run.failed': '任务未能完成', 'run.canceled': '任务已取消',
  'tool.started': '正在使用工具', 'tool.completed': '工具处理完成',
  'interaction.required': '需要你的确认', 'decision.required': '需要一次决定',
  'artifact.created': '已生成产物', 'artifact.available': '产物已就绪',
};

const KIND_LABELS: Record<TaskTimelineKind, string> = {
  run_status: '运行状态', conclusion: 'Agent 结论', tool_activity: '工具活动', approval: '审批', artifact: '产物', activity: '活动',
};

export function timelineKindLabel(kind: TaskTimelineKind): string {
  return KIND_LABELS[kind];
}

export function projectTimeline(events: readonly TaskTimelineSourceEvent[]): TaskTimelineEntry[] {
  return [...events].sort((a, b) => a.seq - b.seq).map((event) => ({
    seq: event.seq,
    occurredAt: event.occurredAt,
    kind: KIND_OF_TYPE[event.type] ?? 'activity',
    type: event.type,
    summary: EVENT_SUMMARIES[event.type] ?? '执行状态已更新',
    evidence: { payload: event.payload },
  }));
}
