import type { TaskBackendPort, TaskBackendStartInput, TaskBackendStartAck } from '../task-office/task-office.ts';
import type { InboxItem, InteractionActionValue, InteractionBackendPort } from '../task-office/attention-inbox.ts';
import type { LegacyFollowUpInput, LegacyTaskBackendPort } from '../task-office/legacy-tasks.ts';
import type { KnowledgeQABackendPort, KnowledgeQAAskInput } from '../task-office/knowledge-qa.ts';
import type { OfflineGate } from './offline-gate.ts';

/** 危险动作装饰器（组合根专用）：派发前经 Offline Gate 拒绝，读通道与已获准的生命周期操作透传。 */

export function guardTaskBackend(backend: TaskBackendPort, gate: OfflineGate): TaskBackendPort {
  return {
    ...backend,
    async start(input: TaskBackendStartInput): Promise<TaskBackendStartAck> {
      await gate.assertOnline('run');
      return backend.start(input);
    },
  };
}

export function guardInteractionBackend(port: InteractionBackendPort, gate: OfflineGate): InteractionBackendPort {
  return {
    ...port,
    async decide(input: { item: InboxItem; decisionId: string; action: InteractionActionValue }) {
      await gate.assertOnline('approval');
      return port.decide(input);
    },
  };
}

export function guardLegacyTaskBackend(port: LegacyTaskBackendPort, gate: OfflineGate): LegacyTaskBackendPort {
  return {
    ...port,
    async followUp(input: LegacyFollowUpInput) {
      await gate.assertOnline('run'); // knowledge-chat 追问触发服务端执行：按 Run 语义拒绝
      return port.followUp(input);
    },
  };
}

export function guardKnowledgeQABackend(port: KnowledgeQABackendPort, gate: OfflineGate): KnowledgeQABackendPort {
  return {
    ...port,
    async ask(input: KnowledgeQAAskInput) {
      await gate.assertOnline('run'); // 知识问答触发服务端执行：按 Run 语义拒绝（与 legacy followUp 同语义）
      return port.ask(input);
    },
  };
}
