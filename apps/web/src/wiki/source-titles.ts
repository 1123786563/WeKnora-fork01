// ─── Vue WikiBrowser.vue source-title hydration ───
//
// Pipeline ingest stores `source_refs` as bare knowledge ids (see
// wiki_ingest_batch), so filenames do not leak into LLM citation strings.
// Vue resolves each bare id with `getKnowledgeDetails(id)` —
// GET /api/v1/knowledge/{id} — reading `title || file_name || fileName`,
// caching per id, keeping the truncated-id fallback on failure (deleted or
// inaccessible docs) and guarding stale loops with a request-sequence
// counter. This module ports that engine 1:1 for the React reader footer;
// the documents capability already exists on the api-client
// (`client.knowledge.documents.get`), so no new endpoint is added.

export interface SourceDocumentMeta {
  title?: string;
  file_name?: string;
  fileName?: string;
}

export type SourceDocumentFetcher = (documentId: string) => Promise<SourceDocumentMeta | null>;

export interface SourceRefTitleHydrator {
  /** Cached display title for a bare ref id, or null before hydration. */
  titleFor(id: string): string | null;
  /** Snapshot of the id → title cache. */
  cachedTitles(): Record<string, string>;
  /** Bare refs (no `|title` suffix) that are not cached yet, deduped. */
  pendingIds(refs: unknown): string[];
  /**
   * Resolve titles for every pending bare ref. Resolves with the full
   * cache snapshot and never rejects — failures keep the truncated-id
   * fallback like the Vue catch branch.
   */
  hydrate(refs: unknown): Promise<Record<string, string>>;
}

export function createSourceRefTitleHydrator(fetchDocument: SourceDocumentFetcher): SourceRefTitleHydrator {
  const cache = new Map<string, string>();
  // Vue sourceRefTitleRequestSeq: a newer hydration pass supersedes any
  // in-flight older pass, which stops applying results mid-loop.
  let seq = 0;

  function isBareRef(ref: unknown): ref is string {
    return typeof ref === 'string' && ref.length > 0 && ref.indexOf('|') < 0;
  }

  return {
    titleFor(id: string): string | null {
      return cache.get(id) ?? null;
    },
    cachedTitles(): Record<string, string> {
      return Object.fromEntries(cache);
    },
    pendingIds(refs: unknown): string[] {
      if (!Array.isArray(refs)) return [];
      const ids = refs.filter(isBareRef).filter((ref) => !cache.has(ref));
      return [...new Set(ids)];
    },
    async hydrate(refs: unknown): Promise<Record<string, string>> {
      const ids = this.pendingIds(refs);
      const current = ++seq;
      for (const id of ids) {
        try {
          const doc = await fetchDocument(id);
          if (current !== seq) return this.cachedTitles();
          const title = doc?.title || doc?.file_name || doc?.fileName;
          if (title) cache.set(id, title);
        } catch {
          // Keep the truncated-ID fallback when the doc was deleted or is
          // inaccessible (Vue hydrateSourceRefTitles catch branch).
        }
      }
      return this.cachedTitles();
    },
  };
}
