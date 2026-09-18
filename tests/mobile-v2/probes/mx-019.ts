// MX-019 probe · 审批冻结修订观察器
// frozen 场景：open-revision4-current5 × stale-confirm。
// 真实 createInteractionController：快照 revision=4 打开，服务器事实已到 5——
// 陈旧确认零发送（decisionCount=0）并导向 refresh；不重发旧决定。
import { createInteractionController } from '../../../apps/mobile/sources/weknora/interactions/controller.ts';
import type { InteractionRecord } from '@weknora/contracts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  decisionCount: number;
  nextAction: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'open-revision4-current5' || input.fault !== 'stale-confirm') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const pendingAtRevision4: InteractionRecord = {
    id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4,
  };
  const serverNowRevision5: InteractionRecord = {
    id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:bb', expected_revision: 5,
  };

  function makeController(counter: { calls: number }) {
    return createInteractionController({
      list: async () => [serverNowRevision5],
      decide: async (value) => {
        counter.calls += 1;
        return { ...value, decision_id: `d-${value.id}` };
      },
      now: () => '2026-09-18T08:00:00Z',
    });
  }
  const generation = { accept: () => true };

  // ── frozen 场景（独立计数）：快照 revision4，服务器已到 5 → 陈旧确认零发送 ──
  const frozenCounter = { calls: 0 };
  const frozenController = makeController(frozenCounter);
  const snapshot = { record: pendingAtRevision4, fetchedAt: '2026-09-18T07:59:00Z' };
  const latest = (await frozenController.openConfirmation('run-1', 'i-1'))?.record ?? serverNowRevision5;
  const result = await frozenController.confirm(snapshot, latest, 'approve', generation);
  if (result.action !== 'refresh' || result.reason !== 'stale_revision') {
    throw new Error(`stale confirm must yield refresh, got ${JSON.stringify(result)}`);
  }
  if (frozenCounter.calls !== 0) throw new Error('stale confirm must not send any decision');

  // ── 附加验证（独立实例，不计入 frozen 观测）──
  // 新鲜快照 → 提交一次，ACK=已记录
  const freshCounter = { calls: 0 };
  const freshController = makeController(freshCounter);
  const fresh = { record: serverNowRevision5, fetchedAt: '2026-09-18T08:00:01Z' };
  const ok = await freshController.confirm(fresh, serverNowRevision5, 'approve', generation);
  if (ok.action !== 'acknowledged' || freshCounter.calls !== 1) throw new Error('fresh confirm must dispatch exactly once and acknowledge');
  // 重复决定幂等：不重发
  const decidedRecord: InteractionRecord = { ...serverNowRevision5, decision_id: 'd-existing', action: 'approve' };
  const idempotent = await freshController.confirm(fresh, decidedRecord, 'approve', generation);
  if (idempotent.action !== 'acknowledged' || idempotent.reason !== 'already_decided' || freshCounter.calls !== 1) {
    throw new Error('already-decided interaction must be idempotent without re-dispatch');
  }
  // 空间切换（generation 失效）→ 零发送导向 refresh
  const staleGeneration = { accept: () => false };
  const scoped = await freshController.confirm(fresh, serverNowRevision5, 'approve', staleGeneration);
  if (scoped.action !== 'refresh' || freshCounter.calls !== 1) throw new Error('scope-invalidated confirm must not dispatch');

  return { decisionCount: frozenCounter.calls, nextAction: result.action };
}
