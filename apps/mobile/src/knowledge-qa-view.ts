import type { KnowledgeResource } from '@weknora/domain/mobile';
import { EVIDENCE_KIND_LABEL, evidenceRetryable, type KnowledgeQAEvidenceCitation, type KnowledgeQATurn, type TaskOffice } from '@weknora/mobile-core';

/** T15（#45）知识问答屏控制器：Screen 只见状态与意图（module-seams §5.2/§10）。 */

export interface KnowledgeQAControllerPorts {
  office: Pick<TaskOffice, 'askKnowledge'>;
  /** 可检索知识（Resource Shelf browse 投影）；缺省空列表（检索范围是可选输入）。 */
  knowledge?(): Promise<readonly KnowledgeResource[]>;
}

export type KnowledgeQAPhase = 'loading' | 'idle' | 'asking' | 'answered' | 'failed';

export interface KnowledgeQAViewState {
  phase: KnowledgeQAPhase;
  question: string;
  knowledge: readonly KnowledgeResource[];
  selectedKnowledgeIds: readonly string[];
  asking: boolean;
  turn?: KnowledgeQATurn;
  statusLine?: string;
  retryAvailable: boolean;
  error?: string;
}

export const EVIDENCE_NO_EVIDENCE_COPY = '知识库中没有支持回答本问题的证据。';
export const EVIDENCE_REVOKED_COPY = '本回答所依据的知识访问已被撤销，结果已作废。';
export const EVIDENCE_REASONING_INCOMPLETE_COPY = '推理未完成：当前部署尚未接入语义推理服务。';

/** 证据时间戳展示：RFC3339 → `YYYY-MM-DD HH:mm:ss UTC`（去掉 T/Z 噪音且保持
 * UTC 标注，不做本地时区换算——证据时间是审计语义，模糊不得、歧义也不得）。
 * 解析失败原样返回，不因格式化丢掉原始审计信息。 */
export function formatEvidenceTimestamp(iso: string): string {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(iso)) return iso;
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return iso;
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${parsed.getUTCFullYear()}-${pad(parsed.getUTCMonth() + 1)}-${pad(parsed.getUTCDate())} ${pad(parsed.getUTCHours())}:${pad(parsed.getUTCMinutes())}:${pad(parsed.getUTCSeconds())} UTC`;
}

/** 证据行展示：来源 · 版本 · 检索时间 · 类别——四个可核对维度都必须可见。 */
export function evidenceCitationLine(citation: KnowledgeQAEvidenceCitation): string {
  const title = citation.title === undefined || citation.title === '' ? citation.knowledgeId : citation.title;
  return `${title} · v${citation.revision} · ${formatEvidenceTimestamp(citation.retrievedAt)} · ${EVIDENCE_KIND_LABEL[citation.kind]}`;
}

export interface KnowledgeQAController {
  state(): KnowledgeQAViewState;
  subscribe(listener: (state: KnowledgeQAViewState) => void): () => void;
  update(patch: { question?: string }): void;
  toggleKnowledge(knowledgeId: string): void;
  refreshKnowledge(): Promise<void>;
  ask(): Promise<KnowledgeQATurn | undefined>;
  retry(): Promise<KnowledgeQATurn | undefined>;
  whenInitialized(): Promise<void>;
  dispose(): void;
}

function statusLineOf(turn: KnowledgeQATurn): string | undefined {
  if (turn.evidence.reasoning.requested && turn.evidence.reasoning.state === 'incomplete') return EVIDENCE_REASONING_INCOMPLETE_COPY;
  if (turn.evidence.state === 'no_evidence') return EVIDENCE_NO_EVIDENCE_COPY;
  if (turn.evidence.state === 'revoked') return EVIDENCE_REVOKED_COPY;
  return undefined;
}

export function createKnowledgeQAController(ports: KnowledgeQAControllerPorts): KnowledgeQAController {
  let state: KnowledgeQAViewState = {
    phase: 'loading',
    question: '',
    knowledge: [],
    selectedKnowledgeIds: [],
    asking: false,
    retryAvailable: false,
  };
  let lastQuestion = '';
  let disposed = false;
  const listeners = new Set<(state: KnowledgeQAViewState) => void>();
  const publish = (next: KnowledgeQAViewState): void => {
    state = next;
    if (!disposed) for (const listener of [...listeners]) listener(state);
  };
  const initialized = (async (): Promise<void> => {
    try {
      const knowledge = ports.knowledge === undefined ? [] : await ports.knowledge();
      if (disposed) return;
      publish({ ...state, knowledge, phase: 'idle' });
    } catch {
      if (!disposed) publish({ ...state, phase: 'idle' }); // 知识列表失败不阻塞提问（范围是可选输入）
    }
  })();
  const runAsk = async (): Promise<KnowledgeQATurn | undefined> => {
    // 空问题防线在两层：Screen 的提问按钮 disabled（question.trim() === ''）与
    // TaskOffice.askKnowledge 对空白输入 fail closed——控制器不重复设卡（重试等
    // 程序化重放路径需要原样执行）。
    const question = state.question.trim();
    lastQuestion = question;
    publish({ ...state, asking: true, error: undefined });
    try {
      const selected = state.selectedKnowledgeIds;
      const turn = await ports.office.askKnowledge({
        question,
        ...(selected.length === 0 ? {} : { knowledgeBaseIds: [...selected] }),
      });
      publish({
        ...state,
        phase: 'answered',
        question: '',
        asking: false,
        turn,
        statusLine: statusLineOf(turn),
        retryAvailable: evidenceRetryable(turn.evidence),
      });
      return turn;
    } catch (error) {
      publish({ ...state, phase: 'failed', asking: false, error: error instanceof Error ? error.message : String(error) });
      return undefined;
    }
  };
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    update(patch) {
      publish({ ...state, ...(patch.question === undefined ? {} : { question: patch.question }) });
    },
    toggleKnowledge(knowledgeId) {
      const selected = state.selectedKnowledgeIds.includes(knowledgeId)
        ? state.selectedKnowledgeIds.filter((id) => id !== knowledgeId)
        : [...state.selectedKnowledgeIds, knowledgeId];
      publish({ ...state, selectedKnowledgeIds: selected });
    },
    async refreshKnowledge() {
      if (ports.knowledge === undefined) return;
      try {
        publish({ ...state, knowledge: await ports.knowledge() });
      } catch { /* 维持现状 */ }
    },
    ask: runAsk,
    async retry() {
      if (!state.retryAvailable) return undefined;
      publish({ ...state, question: lastQuestion });
      return runAsk();
    },
    whenInitialized: () => initialized,
    dispose() {
      disposed = true;
      listeners.clear();
    },
  };
}
