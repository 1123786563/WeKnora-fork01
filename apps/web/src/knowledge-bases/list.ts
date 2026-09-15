import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';

type KnowledgeBaseListParams = NonNullable<Parameters<WeKnoraClient['knowledgeBases']['list']>[0]>;

export type KnowledgeBaseListState =
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; code?: string; message: string };

export async function loadKnowledgeBases(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  signal?: AbortSignal,
  params: KnowledgeBaseListParams = {},
): Promise<KnowledgeBaseListState> {
  try {
    return { status: 'success', items: await client.knowledgeBases.list(params) };
  } catch (error) {
    if (signal?.aborted) throw error;
    const value = error as { code?: unknown; response?: { data?: { error?: { code?: unknown } } } };
    const code = typeof value?.code === 'string' ? value.code : value?.response?.data?.error?.code;
    return { status: 'error', ...(typeof code === 'string' ? { code } : {}), message: error instanceof Error ? error.message : 'Unable to load knowledge bases' };
  }
}
