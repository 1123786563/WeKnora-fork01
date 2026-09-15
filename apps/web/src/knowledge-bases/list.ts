import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseListState =
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; code?: string; message: string };

export async function loadKnowledgeBases(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  signal?: AbortSignal,
  params: { creator?: 'all' | 'mine' | 'others' } = {},
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

export interface KnowledgeBaseSaveInput {
  [key: string]: unknown;
  name: string;
  type?: 'document' | 'faq';
  description?: string;
  embedding_model_id?: string;
  summary_model_id?: string;
}

function objectValue(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

/**
 * The Vue editor validates the complete draft immediately before submit.
 * Keep that contract at the mutation boundary too, so non-visual callers
 * (deep links, desktop, and tests) cannot bypass the same rules.
 */
export function validateKnowledgeBaseSaveInput(input: KnowledgeBaseSaveInput): void {
  const name = input.name.trim();
  if (!name) throw new Error('Knowledge base name is required');
  if ([...name].length > 50) throw new Error('Knowledge base name must be 50 characters or fewer');
  if (typeof input.description === 'string' && [...input.description].length > 200) {
    throw new Error('Knowledge base description must be 200 characters or fewer');
  }

  const type = input.type ?? 'document';
  if (type !== 'document' && type !== 'faq') throw new Error('Knowledge base type is invalid');

  if (type === 'faq') {
    const faq = objectValue(input.faq_config);
    if ('index_mode' in faq && !String(faq.index_mode ?? '').trim()) {
      throw new Error('Knowledge base FAQ index mode is required');
    }
    // Vue requires a summary model for both document and FAQ editors.
    if (!String(input.summary_model_id ?? '').trim()) throw new Error('Knowledge base summary model is required');
    return;
  }

  const strategy = objectValue(input.indexing_strategy);
  if (Object.keys(strategy).length > 0) {
    const enabled = ['vector_enabled', 'keyword_enabled', 'wiki_enabled', 'graph_enabled']
      .some((key) => strategy[key] === true);
    if (!enabled) throw new Error('at least one indexing strategy is required');
    if ((strategy.vector_enabled === true || strategy.keyword_enabled === true)
      && !String(input.embedding_model_id ?? '').trim()) {
      throw new Error('embedding model is required');
    }
  }

  // Keep the first failing section aligned with Vue's validateForm: strategy,
  // embedding, then summary model.
  if (!String(input.summary_model_id ?? '').trim()) throw new Error('Knowledge base summary model is required');

  const vlm = objectValue(input.vlm_config ?? input.image_processing_config);
  if (vlm.enabled === true && !String(vlm.model_id ?? '').trim()) {
    throw new Error('multimodal model is required');
  }
}

export async function saveKnowledgeBase(client: Pick<WeKnoraClient, 'knowledgeBases'>, id: string | null, input: KnowledgeBaseSaveInput) {
  validateKnowledgeBaseSaveInput(input);
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
