import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type {
  ResearchAnnotationDraft, ResearchAnnotationReceipt, ResearchAnnotationRow, ResearchBackendPort,
  ResearchDelegationRow, ResearchDraftsPort, ResearchEvent, ResearchRevisionInput, ResearchRevisionReceipt,
  TaskResearch, TaskResearchHandle, TaskResearchPorts,
} from './types.ts';

/** T17（#47）research 深模块：委派/批注/修订的全部编排纪律藏在句柄后。
 *  Screen 只表达意图；顺序、scope 检查、离线草稿、错误映射都在这里。 */

export type ResearchErrorCode =
  | 'RESEARCH_SCOPE_CHANGED'
  | 'RESEARCH_INVALID_INPUT'
  | 'RESEARCH_NOT_FOUND'
  | 'RESEARCH_CONFLICT'
  | 'RESEARCH_COMMAND_UNAVAILABLE'
  | 'RESEARCH_COMMAND_CONFLICT'
  | 'RESEARCH_DRAFT_UNAVAILABLE'
  | 'RESEARCH_BACKEND';

export class ResearchError extends Error {
  constructor(readonly code: ResearchErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'ResearchError';
  }
}

// 行 Port 类型经本模块 re-export（测试与消费方同源导入）。
export type { ResearchBackendPort, ResearchDraftsPort, ResearchEvent } from './types.ts';

const MAX_SOURCES = 8;
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));

/** 跨包契约码优先读 Error.code（api-client remote 的 coded() 形态），回退到 message。 */
const failureCode = (failure: unknown): string => {
  const code = (failure as { code?: unknown } | null)?.code;
  if (typeof code === 'string' && code !== '') return code;
  return messageOf(failure);
};

function backendFailure(failure: unknown): ResearchError {
  const code = failureCode(failure);
  if (code.includes('RESEARCH_BASE_VERSION_CONFLICT')) return new ResearchError('RESEARCH_CONFLICT', { cause: failure });
  if (code.includes('RESEARCH_SCOPE_CHANGED')) return new ResearchError('RESEARCH_SCOPE_CHANGED', { cause: failure });
  if (code.includes('RESEARCH_NOT_FOUND')) return new ResearchError('RESEARCH_NOT_FOUND', { cause: failure });
  return new ResearchError('RESEARCH_BACKEND', { cause: failure });
}

export function createTaskResearch(ports: TaskResearchPorts): TaskResearch {
  const nextDraftId = (): string => `research-ann-${(++draftSeq).toString()}`;
  let draftSeq = 0;
  return {
    open({ lease }: { lease: ScopeLease }): TaskResearchHandle {
      let closed = false;
      const listeners = new Set<(event: ResearchEvent) => void>();
      const emit = (event: ResearchEvent) => { for (const listener of listeners) listener(event); };
      const check = (): void => {
        if (closed || !leaseActive(lease)) throw new ResearchError('RESEARCH_SCOPE_CHANGED');
      };
      const requireDrafts = (): ResearchDraftsPort => {
        if (ports.drafts === undefined) throw new ResearchError('RESEARCH_DRAFT_UNAVAILABLE');
        return ports.drafts;
      };
      const validateDelegation = (input: { objective: string; sources: string[] }): void => {
        if (typeof input.objective !== 'string' || input.objective.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
        if (!Array.isArray(input.sources) || input.sources.length === 0 || input.sources.length > MAX_SOURCES) throw new ResearchError('RESEARCH_INVALID_INPUT');
        if (input.sources.some((source) => typeof source !== 'string' || source.trim() === '')) throw new ResearchError('RESEARCH_INVALID_INPUT');
      };
      return {
        async delegate(input) {
          check();
          validateDelegation(input);
          try {
            return await ports.remote.delegate({ runId: input.runId, objective: input.objective.trim(), sources: input.sources.map((source) => source.trim()) });
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async delegations(runId) {
          check();
          try {
            return (await ports.remote.list(runId)).delegations;
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async complete(input) {
          check();
          if (input.summary.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          try {
            return await ports.remote.complete(input);
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async annotate(input): Promise<ResearchAnnotationReceipt> {
          check();
          if (typeof input.body !== 'string' || input.body.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          if (input.baseVersion.trim() === '' || input.materialId.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          const offline = ports.gate ? (await ports.gate.status()) === 'offline' : false;
          if (offline) {
            const drafts = requireDrafts();
            const draftId = nextDraftId();
            if (!DRAFT_ID_PATTERN.test(draftId)) throw new ResearchError('RESEARCH_DRAFT_UNAVAILABLE');
            const draft: ResearchAnnotationDraft = {
              draftId, runId: input.runId, materialId: input.materialId.trim(),
              baseVersion: input.baseVersion.trim(), body: input.body.trim(), draftedAt: new Date().toISOString(),
            };
            await drafts.put(draft);
            emit({ type: 'annotation-drafted', draftId });
            return { status: 'drafted', draftId };
          }
          try {
            const annotation = await ports.remote.annotate(input);
            return { status: 'recorded', annotation };
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async annotations(runId) {
          check();
          try {
            return (await ports.remote.annotations(runId)).annotations;
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async requestRevision(input): Promise<ResearchRevisionReceipt> {
          check();
          if (ports.commands === undefined) throw new ResearchError('RESEARCH_COMMAND_UNAVAILABLE');
          if (input.note.trim() === '' || input.baseVersion.trim() === '' || input.materialId.trim() === '') {
            throw new ResearchError('RESEARCH_INVALID_INPUT');
          }
          // Global Constraints：「修订请求属 Run 命令，离线一律拒绝（OfflineGate 断言）」——
          // 与同族 Run 命令先例（task-office.ts start()/askKnowledge() 的入口断言）同款：
          // 在下方 try/catch 之外上抛，OfflineGateError（OFFLINE_ACTION_BLOCKED:run）原样
          // 透出，绝不包装成 ResearchError/RESEARCH_BACKEND（「服务端暂时不可用」误导文案）。
          // if 形式而非 ?.：gate 缺省时不引入额外微任务（与 start 同一注释约定）。
          if (ports.gate !== undefined) await ports.gate.assertOnline('run');
          // 确定性版本钉定文本：Lead Agent 依据该文本在授权版本上派生新版本。
          const text = `请基于版本 ${input.baseVersion.trim()} 修订材料 ${input.materialId.trim()}：${input.note.trim()}`;
          try {
            const ack = await ports.commands.command({
              runId: input.runId, action: input.action, text,
              expectedRevision: input.expectedRevision, ...(input.intentId === undefined ? {} : { intentId: input.intentId }),
            });
            return {
              intent: 'revision-request', outcome: 'accepted', action: input.action,
              ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at: new Date().toISOString(),
            };
          } catch (failure) {
            // 真实契约码是 TASK_COMMAND_CONFLICT（task-detail.ts:86 契约注释 +
            // api-client task-office.ts:262 的 coded()；不是 TASK_OFFICE_* 前缀）。
            if (failureCode(failure).includes('TASK_COMMAND_CONFLICT')) {
              throw new ResearchError('RESEARCH_COMMAND_CONFLICT', { cause: failure });
            }
            throw new ResearchError('RESEARCH_BACKEND', { cause: failure });
          }
        },
        async flushAnnotationDrafts({ runId }) {
          check();
          const drafts = requireDrafts();
          const outcomes: Array<{ draftId: string; outcome: 'recorded' | 'conflict' | 'failed' }> = [];
          for (const draft of await drafts.list()) {
            if (draft.runId !== runId) continue;
            try {
              check();
              await ports.remote.annotate({ runId: draft.runId, materialId: draft.materialId, baseVersion: draft.baseVersion, body: draft.body });
              await drafts.remove(draft.draftId);
              outcomes.push({ draftId: draft.draftId, outcome: 'recorded' });
              emit({ type: 'draft-flushed', draftId: draft.draftId });
            } catch (failure) {
              if (failure instanceof ResearchError && failure.code === 'RESEARCH_SCOPE_CHANGED') throw failure;
              if (failureCode(failure).includes('RESEARCH_BASE_VERSION_CONFLICT')) {
                outcomes.push({ draftId: draft.draftId, outcome: 'conflict' }); // 版本已过期：草稿保留，待用户重读版本后重提
                continue;
              }
              outcomes.push({ draftId: draft.draftId, outcome: 'failed' });
            }
          }
          return outcomes;
        },
        async pendingDrafts() {
          check();
          if (ports.drafts === undefined) return [];
          return ports.drafts.list();
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => listeners.delete(listener);
        },
        close(reason: string) {
          if (closed) return;
          closed = true;
          emit({ type: 'scope-closed', reason });
        },
      };
    },
  };
}
