import { normalizeKnowledgeProcessingStatus, isKnowledgeProcessingActive } from '@weknora/domain/knowledge/processing';

// Per-document action availability (Vue parity: doc-content.vue offers
// re-parse for processed content and cancel-parse only while parsing is in
// flight — routes_knowledge.go:110-111).

export interface DocumentRowActions {
  canReparse: boolean;
  canCancelParse: boolean;
}

/** Keep Vue batch-reparse semantics: never submit documents already parsing. */
export function filterReparseIds(
  ids: readonly string[],
  documents: readonly { id: string; parse_status?: string }[],
): string[] {
  const seen = new Set<string>();
  return ids.filter((id) => {
    if (seen.has(id)) return false;
    seen.add(id);
    const document = documents.find((item) => item.id === id);
    return !document || !documentRowActions(document.parse_status).canCancelParse;
  });
}

export function documentRowActions(parseStatus: string | undefined): DocumentRowActions {
  let active = false;
  let known = false;
  if (parseStatus) {
    try {
      const status = normalizeKnowledgeProcessingStatus(parseStatus);
      known = true;
      active = isKnowledgeProcessingActive(status);
    } catch {
      known = false;
    }
  }
  return {
    // Reparse applies to any known status (backend accepts it per document).
    canReparse: known,
    canCancelParse: active,
  };
}

export interface DocumentMutationApi {
  reparse(id: string, processConfig?: unknown): Promise<void>;
  cancelParse(id: string): Promise<void>;
}

/** Per-document reparse; resolves after the backend accepts the request. */
export async function reparseDocument(
  api: DocumentMutationApi,
  id: string,
  processConfig?: unknown,
): Promise<void> {
  await api.reparse(id, processConfig);
}

/** Cancel parse for every document currently pending/processing/finalizing. */
export async function cancelParseDocuments(
  api: DocumentMutationApi,
  documents: readonly { id: string; parse_status?: string }[],
): Promise<string[]> {
  const seen = new Set<string>();
  const targets = documents.filter((document) => {
    if (seen.has(document.id)) return false;
    seen.add(document.id);
    return documentRowActions(document.parse_status).canCancelParse;
  });
  let firstError: unknown;
  for (const document of targets) {
    try {
      await api.cancelParse(document.id);
    } catch (error) {
      // Keep trying other selected documents; the caller still receives the
      // original server error for its localized feedback path.
      firstError ??= error;
    }
  }
  if (firstError !== undefined) throw firstError;
  return targets.map((document) => document.id);
}
