import type { ScopeLease } from '../runtime/types.ts';
import type { OfflineGate } from '../offline/offline-gate.ts';
import type { TaskCommandPort } from '../task-office/task-detail.ts';

/** T17（#47）research 深模块类型合同：委派行/批注行/草稿/事件。 */

export type ResearchStatus = 'assigned' | 'completed';

export interface ResearchDelegationRow {
  delegationId: string;
  runId: string;
  sessionId: string;
  objective: string;
  sources: string[];
  status: ResearchStatus;
  summary?: string;
  createdAt: string;
}

export interface ResearchAnnotationRow {
  annotationId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  authorId: string;
  createdAt: string;
}

/** wire 语义行 Port：api-client `./mobile/research` 与场景 Adapter 同构。 */
export interface ResearchBackendPort {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}

/** 离线批注草稿（设计 spec：「Offline mode permits approved reads, drafts and annotations」）。 */
export interface ResearchAnnotationDraft {
  draftId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  draftedAt: string;
}

/** 草稿仓储 Port：组合根用 Scoped Vault drafts 命名空间适配（加密、scope 隔离）。 */
export interface ResearchDraftsPort {
  put(draft: ResearchAnnotationDraft): Promise<void>;
  list(): Promise<ResearchAnnotationDraft[]>;
  remove(draftId: string): Promise<void>;
}

export interface TaskResearchPorts {
  remote: ResearchBackendPort;
  /** 修订请求的命令通道（#37）；缺失 → requestRevision fail closed。 */
  commands?: TaskCommandPort;
  /** 离线探测（#40）；缺失 → annotate/requestRevision 直接走网络（物理离线由传输层兜底）。
   *  在场时 requestRevision 属 Run 命令：派发前经 assertOnline('run') 结构化拒绝（终局修复）。 */
  gate?: OfflineGate;
  /** 离线批注草稿仓储；缺失且离线 → annotate fail closed（不静默丢批注）。 */
  drafts?: ResearchDraftsPort;
}

export interface TaskResearch {
  open(input: { lease: ScopeLease }): TaskResearchHandle;
}

export type ResearchAnnotationReceipt =
  | { status: 'recorded'; annotation: ResearchAnnotationRow }
  | { status: 'drafted'; draftId: string };

export interface ResearchRevisionInput {
  runId: string;
  materialId: string;
  baseVersion: string;
  note: string;
  action: 'steer' | 'queue_next';
  /** #37 修订纪律：来自任务详情视图的观察值（或服务端 CAS 证明的 +1）。 */
  expectedRevision: number;
  intentId?: string;
}

export interface ResearchRevisionReceipt {
  intent: 'revision-request';
  outcome: 'accepted' | 'conflict';
  action: 'steer' | 'queue_next';
  nextRunId?: string;
  at: string;
}

export type ResearchEvent =
  | { type: 'scope-closed'; reason?: string }
  | { type: 'annotation-drafted'; draftId: string }
  | { type: 'draft-flushed'; draftId: string };

export interface TaskResearchHandle {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  delegations(runId: string): Promise<ResearchDelegationRow[]>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationReceipt>;
  annotations(runId: string): Promise<ResearchAnnotationRow[]>;
  requestRevision(input: ResearchRevisionInput): Promise<ResearchRevisionReceipt>;
  flushAnnotationDrafts(input: { runId: string }): Promise<Array<{ draftId: string; outcome: 'recorded' | 'conflict' | 'failed' }>>;
  pendingDrafts(): Promise<ResearchAnnotationDraft[]>;
  subscribe(listener: (event: ResearchEvent) => void): () => void;
  close(reason: string): void;
}
