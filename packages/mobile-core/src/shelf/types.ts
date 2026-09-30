import type { AgentOption, ConnectionResource, KnowledgeResource } from '@weknora/domain/mobile';
import type { ScopeLease } from '../runtime/types.ts';

export type ResourceClass = 'agent' | 'knowledge' | 'connection';

/** 与 #32 VaultRevokeReason 值集一致（deployment-change/tenant-switch/sign-out/dispose）。 */
export type ShelfCloseReason = 'deployment-change' | 'tenant-switch' | 'sign-out' | 'dispose';

export interface ResourceClassVerdict {
  state: 'supported' | 'unavailable' | 'forbidden';
  reason: string;
}

/** 领域状态投影（spec §6.2：不返回原始 Agent/KB/Connection DTO）。 */
export interface ResourcePage {
  tenantId: string;
  agents: readonly AgentOption[];
  knowledge: readonly KnowledgeResource[];
  connections: readonly ConnectionResource[];
  classVerdicts: Readonly<Record<ResourceClass, ResourceClassVerdict>>;
}

export interface ResourceQuery {
  kind?: AgentOption['kind'];
  keyword?: string;
}

/** spec §6.2：SelectionVerdict 明确 allowed、unavailable 或 forbidden 及原因。 */
export type SelectionVerdict =
  | { allowed: true; selection: { kind: 'agent'; agentId: string } | { kind: 'knowledge_ref'; knowledgeId: string } }
  | { allowed: false; state: 'unavailable' | 'forbidden'; reason: string };

export type ShelfInvalidationEvent =
  | { type: 'scope-closed'; reason: ShelfCloseReason }
  | { type: 'authorization-revoked'; resourceClass: ResourceClass };

export interface ResourceShelfHandle {
  browse(query?: ResourceQuery): Promise<ResourcePage>;
  selection(input: { agentId?: string; knowledgeId?: string }): SelectionVerdict;
  subscribe(listener: (event: ShelfInvalidationEvent) => void): () => void;
  close(reason: ShelfCloseReason): void;
}

export interface ResourceShelf {
  /** 以有效 Scope Lease 开一个 shelf；lease 无效立即抛 SHELF_LEASE（fail closed）。 */
  open(scope: { lease: ScopeLease }): ResourceShelfHandle;
}
