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

export interface KnowledgeBaseSaveInput {
  [key: string]: unknown;
  name: string;
  type?: 'document' | 'faq';
  description?: string;
  embedding_model_id?: string;
  summary_model_id?: string;
}

export async function saveKnowledgeBase(client: Pick<WeKnoraClient, 'knowledgeBases'>, id: string | null, input: KnowledgeBaseSaveInput) {
  if (!input.name.trim()) throw new Error('Knowledge base name is required');
  return id
    ? client.knowledgeBases.update(id, input)
    : client.knowledgeBases.create(input);
}

export async function deleteKnowledgeBase(client: Pick<WeKnoraClient, 'knowledgeBases'>, id: string): Promise<void> {
  if (!id.trim()) throw new Error('Knowledge base id is required');
  await client.knowledgeBases.remove(id);
}

// ---- Card-grid page model: parallel owned + shared fetch with explicit loading state ----

import type { SharedKnowledgeBaseLike } from '@weknora/domain';

export type KnowledgeBaseListPageState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'success'; owned: KnowledgeBase[]; shared: SharedKnowledgeBaseLike[] };

interface ListPageClient {
  knowledgeBases: { list(params?: Record<string, string>): Promise<KnowledgeBase[]> };
  identity: {
    organizations: {
      knowledgeBaseShares: { listShared(signal?: AbortSignal): Promise<unknown> };
    };
  };
}

export async function loadKnowledgeBaseListPage(
  client: ListPageClient,
  signal?: AbortSignal,
  params: { creator?: 'all' | 'mine' | 'others' } = {},
): Promise<KnowledgeBaseListPageState> {
  try {
    const [owned, shared] = await Promise.all([
      client.knowledgeBases.list(params),
      client.identity.organizations.knowledgeBaseShares.listShared(signal) as Promise<SharedKnowledgeBaseLike[]>,
    ]);
    return { status: 'success', owned, shared };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load knowledge bases' };
  }
}

export type DeleteGuardResult = 'deleted' | 'in-flight';

/**
 * Guarantees at most one DELETE per knowledge base while a confirmation
 * is still in flight; repeated confirms during the request are dropped.
 */
export function createDeleteGuard(client: Pick<WeKnoraClient, 'knowledgeBases'>) {
  const inFlight = new Set<string>();
  return {
    async confirm(id: string): Promise<DeleteGuardResult> {
      if (inFlight.has(id)) return 'in-flight';
      inFlight.add(id);
      try {
        await deleteKnowledgeBase(client, id);
        return 'deleted';
      } finally {
        inFlight.delete(id);
      }
    },
  };
}