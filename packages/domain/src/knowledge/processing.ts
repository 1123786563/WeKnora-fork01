import type { KnowledgeProcessingStatus } from '@weknora/contracts';

export const knowledgeProcessingStatuses: readonly KnowledgeProcessingStatus[] = [
  'pending', 'processing', 'finalizing', 'completed', 'failed', 'deleting', 'cancelled',
];

export function normalizeKnowledgeProcessingStatus(value: unknown): KnowledgeProcessingStatus {
  if (typeof value === 'string' && knowledgeProcessingStatuses.includes(value as KnowledgeProcessingStatus)) {
    return value as KnowledgeProcessingStatus;
  }
  throw new Error(`Unknown knowledge processing status: ${String(value)}`);
}

export function isKnowledgeProcessingTerminal(status: KnowledgeProcessingStatus): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}
