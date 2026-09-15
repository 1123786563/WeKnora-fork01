import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseListState =
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; code?: string; message: string };

export async function loadKnowledgeBases(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  signal?: AbortSignal,
): Promise<KnowledgeBaseListState> {
  try {
    return { status: 'success', items: await client.knowledgeBases.list({}) };
  } catch (error) {
    if (signal?.aborted) throw error;
    const value = error as { code?: unknown; response?: { data?: { error?: { code?: unknown } } } };
    const code = typeof value?.code === 'string' ? value.code : value?.response?.data?.error?.code;
    return { status: 'error', ...(typeof code === 'string' ? { code } : {}), message: error instanceof Error ? error.message : 'Unable to load knowledge bases' };
  }
}
