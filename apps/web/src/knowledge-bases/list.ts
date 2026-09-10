import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseListState =
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; message: string };

export async function loadKnowledgeBases(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  signal?: AbortSignal,
  params: { creator?: 'all' | 'mine' | 'others' } = {},
): Promise<KnowledgeBaseListState> {
  try {
    return { status: 'success', items: await client.knowledgeBases.list(params) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load knowledge bases' };
  }
}

export async function saveKnowledgeBase(client: Pick<WeKnoraClient, 'knowledgeBases'>, id: string | null, input: { name: string; type?: 'document' | 'faq' }) {
  if (!input.name.trim()) throw new Error('Knowledge base name is required');
  return id
    ? client.knowledgeBases.update(id, input)
    : client.knowledgeBases.create(input);
}

export async function deleteKnowledgeBase(client: Pick<WeKnoraClient, 'knowledgeBases'>, id: string): Promise<void> {
  if (!id.trim()) throw new Error('Knowledge base id is required');
  await client.knowledgeBases.remove(id);
}
