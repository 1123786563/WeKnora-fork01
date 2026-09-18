// MX-006 probe · 丢失 ACK 与进程重启对账观察器
// 用真实 domain 协调器 + 内存 store 演练 frozen 场景；transport 是测试边界（计数与脚本化响应），
// 业务规则全部来自产品代码 createSubmissionCoordinator。
import {
  createInMemorySubmissionStore,
  createSubmissionCoordinator,
  type MobileStartInput,
  type SubmissionScope,
  type SubmissionTransport,
} from '../../../packages/domain/src/mobile/submission.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  startCount: number;
  lookupRequestId: string;
  runId: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'same-request-and-input' || input.fault !== 'ack-drop-process-restart') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const scope: SubmissionScope = { origin: 'https://weknora.example', tenantID: 't1', userID: 'u1' };
  const payload: MobileStartInput = {
    request_id: 'request-original',
    session_id: 'session-1',
    agent_id: 'agent-1',
    target_id: 'platform',
    workspace_ref: 'workspace-1',
    text: '整理本周反馈',
    budget_upper: 100,
  };

  let startCount = 0;
  let lookupCount = 0;
  // 服务端语义脚本：Start 已被服务端持久化但 ACK 丢弃（抛错）；lookup 按真实状态机返回 admitted+原 run。
  const transport: SubmissionTransport = {
    start: async () => {
      startCount += 1;
      throw new Error('simulated ack drop after server persist');
    },
    lookup: async (requestId) => {
      lookupCount += 1;
      if (requestId !== 'request-original') throw new Error(`unexpected lookup id ${requestId}`);
      return { state: 'admitted', run_id: 'run-original' };
    },
  };

  const store = createInMemorySubmissionStore();
  // 进程 1：提交 → ACK 丢失 → awaiting_reconciliation（已落盘同一 request_id）
  const first = createSubmissionCoordinator(store, transport);
  const firstOutcome = await first.submit(payload, scope);
  if (firstOutcome.entry.phase !== 'awaiting_reconciliation') {
    throw new Error(`expected awaiting_reconciliation after ack drop, got ${firstOutcome.entry.phase}`);
  }

  // 进程重启：同一持久 store 恢复（内存 store 模拟持久层），新协调器实例。
  const restarted = createSubmissionCoordinator(store, transport);
  const outcome = await restarted.submit(payload, scope);
  if (outcome.dispatched) {
    throw new Error('restart must not re-POST the same request_id');
  }
  if (outcome.entry.phase !== 'bound' || !outcome.entry.run_id) {
    throw new Error(`expected bound entry after reconciliation, got ${outcome.entry.phase}`);
  }
  if (lookupCount === 0) throw new Error('reconciliation must use request lookup');

  return { startCount, lookupRequestId: outcome.entry.request_id, runId: outcome.entry.run_id };
}
