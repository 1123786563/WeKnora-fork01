import { OfflineGateError, ResearchError } from '@weknora/mobile-core';
import type {
  MaterialEntry, ResearchAnnotationRow, ResearchDelegationRow, ResearchErrorCode, TaskResearchHandle,
} from '@weknora/mobile-core';

export interface ResearchViewState {
  loading: boolean;
  delegations?: ResearchDelegationRow[];
  annotations?: ResearchAnnotationRow[];
  materials?: MaterialEntry[];
  pendingDrafts?: number;
  error?: string;
  notice?: string;
}

/** ResearchError 错误码 → 用户文案（键类型=ResearchErrorCode，新码缺文案即类型错）。 */
export const RESEARCH_ERROR_COPY: Record<ResearchErrorCode, string> = {
  RESEARCH_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  RESEARCH_INVALID_INPUT: '研究目标或来源填写不完整。',
  RESEARCH_NOT_FOUND: '该研究委派或批注已不存在，请刷新。',
  RESEARCH_CONFLICT: '材料版本已更新，批注未落库；请重新打开该版本后再批注。',
  RESEARCH_COMMAND_UNAVAILABLE: '此部署暂不支持修订请求通道。',
  RESEARCH_COMMAND_CONFLICT: '任务状态已变化，修订请求被拒绝；请刷新任务详情后重试。',
  RESEARCH_DRAFT_UNAVAILABLE: '当前无法安全保存离线批注草稿，请联网后再批注。',
  RESEARCH_BACKEND: '服务端暂时不可用，请稍后重试。',
};

/** 终局修复（终局审查发现 1）：修订请求属 Run 命令（Global Constraints「离线一律拒绝」），
 *  模块以 OfflineGateError（OFFLINE_ACTION_BLOCKED:run）原样上抛——此处给出结构化离线
 *  判决的用户文案，绝不落 RESEARCH_BACKEND 的「服务端暂时不可用」（误导用户以为服务端故障）。 */
export const RESEARCH_OFFLINE_REVISION_COPY = '当前离线：修订请求属于运行指令，请联网后再提交。';

const messageOf = (failure: unknown): string => {
  if (failure instanceof OfflineGateError) return RESEARCH_OFFLINE_REVISION_COPY;
  if (failure instanceof ResearchError) return RESEARCH_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};

export interface ResearchController {
  state(): ResearchViewState;
  subscribe(listener: (state: ResearchViewState) => void): () => void;
  load(): Promise<void>;
  delegate(input: { objective: string; sources: string }): Promise<void>;
  annotate(input: { materialId: string; baseVersion: string; body: string }): Promise<void>;
  flushDrafts(): Promise<void>;
  requestRevision(input: { materialId: string; baseVersion: string; note: string; action: 'steer' | 'queue_next'; expectedRevision: number }): Promise<void>;
  dispose(): void;
}

/** 研究页控制器：load 驱动委派/批注/材料三投影，annotate 走句柄（含离线草稿），
 *  requestRevision 透传修订纪律参数；dispose 关闭句柄。 */
export function createResearchController(
  handle: TaskResearchHandle,
  input: { runId: string; materials?: () => Promise<MaterialEntry[]> },
): ResearchController {
  let state: ResearchViewState = { loading: true };
  let disposed = false;
  const listeners = new Set<(state: ResearchViewState) => void>();
  const publish = (next: Partial<ResearchViewState>) => {
    state = { ...state, ...next };
    for (const listener of listeners) listener(state);
  };
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    async load() {
      if (disposed) return;
      publish({ loading: true, error: undefined });
      try {
        const [delegations, annotations, pending] = await Promise.all([
          handle.delegations(input.runId),
          handle.annotations(input.runId),
          handle.pendingDrafts().then((drafts) => drafts.filter((draft) => draft.runId === input.runId).length).catch(() => 0),
        ]);
        const materials = input.materials === undefined ? undefined : await input.materials().catch(() => undefined);
        publish({ loading: false, delegations, annotations, ...(materials === undefined ? {} : { materials }), pendingDrafts: pending });
      } catch (failure) {
        publish({ loading: false, error: messageOf(failure) });
      }
    },
    async delegate({ objective, sources }) {
      if (disposed) return;
      const list = sources.split(/[\s,，、]+/).map((source) => source.trim()).filter((source) => source !== '');
      try {
        await handle.delegate({ runId: input.runId, objective, sources: list });
        publish({ notice: '只读研究已委派。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async annotate({ materialId, baseVersion, body }) {
      if (disposed) return;
      try {
        const receipt = await handle.annotate({ runId: input.runId, materialId, baseVersion, body });
        publish({ notice: receipt.status === 'drafted' ? '当前离线：批注已存为加密草稿，联网后点「同步批注」提交。' : '批注已记录（生成新批注记录，原版本保持不变）。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async flushDrafts() {
      if (disposed) return;
      try {
        const outcomes = await handle.flushAnnotationDrafts({ runId: input.runId });
        const conflicts = outcomes.filter((outcome) => outcome.outcome === 'conflict').length;
        publish({ notice: conflicts > 0 ? `批注同步完成，${conflicts} 条因版本更新需重读后重提。` : '批注同步完成。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async requestRevision({ materialId, baseVersion, note, action, expectedRevision }) {
      if (disposed) return;
      try {
        const receipt = await handle.requestRevision({ runId: input.runId, materialId, baseVersion, note, action, expectedRevision });
        publish({ notice: receipt.outcome === 'accepted' ? '修订请求已提交（将生成新版本，已批注版本保持不变）。' : '修订请求被拒绝：任务状态已变化。' });
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    dispose() {
      disposed = true;
      handle.close('research-route-unmount');
    },
  };
}
