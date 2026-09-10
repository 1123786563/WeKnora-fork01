import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseListState =
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; message: string };

export async function loadKnowledgeBases(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  signal?: AbortSignal,
): Promise<KnowledgeBaseListState> {
  try {
    return { status: 'success', items: await client.knowledgeBases.list({}) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load knowledge bases' };
  }
}
