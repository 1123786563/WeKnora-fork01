import type { InteractionRecord } from '@weknora/contracts';

/**
 * 交互决定控制器（MX-019 / M09）。
 * 冻结规则：
 * - 打开确认前刷新当前详情（list/get 最新交互），Sheet 捕获 revision/generation；
 * - 确认提交时以**捕获时**的 revision/digest 发送——提交前详情已过期（服务器 revision 前进）
 *   则**零发送**并导向 refresh（409 语义前置化）；客户端绝不重发旧决定；
 * - ACK 只代表决定已记录——不宣告外部写入成功；状态以重读/事件流为准；
 * - 409/过期/撤权：关闭旧确认，提示重新核对（nextAction=refresh）。
 */

export interface InteractionSnapshot {
  record: InteractionRecord;
  fetchedAt: string;
}

export type ApprovalAction = 'refresh' | 'submit' | 'acknowledged' | 'conflict';

export interface ApprovalDecisionResult {
  action: ApprovalAction;
  reason?: 'stale_revision' | 'already_decided' | 'not_pending' | 'server_conflict';
  decisionCount: number;
}

export interface InteractionPorts {
  list(runID: string): Promise<InteractionRecord[]>;
  decide(input: InteractionRecord): Promise<InteractionRecord>;
  /** 服务器当前事实源（打开确认前强制刷新） */
  now(): string;
}

export function createInteractionController(ports: InteractionPorts) {
  let decisionCount = 0;

  return {
    /** 打开确认：强制刷新当前详情（禁止使用陈旧缓存直接确认）。 */
    async openConfirmation(runID: string, interactionID: string): Promise<InteractionSnapshot | null> {
      const records = await ports.list(runID);
      const record = records.find((row) => row.id === interactionID) ?? null;
      return record ? { record, fetchedAt: ports.now() } : null;
    },
    /**
     * 确认决定：stale-confirm 守卫——快照 revision 落后于最新事实（或快照非 pending）时
     * 零发送并返回 refresh；相同决定幂等（already_decided → acknowledged）。
     */
    async confirm(snapshot: InteractionSnapshot, latest: InteractionRecord, decision: 'approve' | 'reject', generation: { accept(): boolean }): Promise<ApprovalDecisionResult> {
      if (!generation.accept()) {
        decisionCount += 0;
        return { action: 'refresh', reason: 'stale_revision', decisionCount };
      }
      if (latest.id !== snapshot.record.id) {
        return { action: 'refresh', reason: 'stale_revision', decisionCount };
      }
      if (latest.decision_id !== '') {
        // 已有决定：幂等视为已记录（不重发）
        return { action: 'acknowledged', reason: 'already_decided', decisionCount };
      }
      if (snapshot.record.expected_revision !== latest.expected_revision || snapshot.record.args_hash !== latest.args_hash) {
        // 打开后服务器事实已前进：关闭旧确认，导向重新核对
        return { action: 'refresh', reason: 'stale_revision', decisionCount };
      }
      if (snapshot.record.kind !== 'tool_approval' || !['approve', 'reject'].includes(decision)) {
        return { action: 'refresh', reason: 'not_pending', decisionCount };
      }
      const decided: InteractionRecord = {
        ...snapshot.record,
        action: decision,
        decision_id: `decision-${snapshot.record.id}-${snapshot.record.expected_revision}`,
      };
      const ack = await ports.decide(decided);
      decisionCount += 1;
      // ACK 只表示决定已记录；外部执行以事件流为准——action=acknowledged 而非「成功」
      void ack;
      return { action: 'acknowledged', decisionCount };
    },
  };
}

export type InteractionController = ReturnType<typeof createInteractionController>;
