import { normalizeKnowledgeProcessingStatus, isKnowledgeProcessingActive } from '@weknora/domain/knowledge/processing';

// Per-document action availability (Vue parity: doc-content.vue offers
// re-parse for processed content and cancel-parse only while parsing is in
// flight — routes_knowledge.go:110-111).

export interface DocumentRowActions {
  canReparse: boolean;
  canCancelParse: boolean;
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
  reparse(id: string): Promise<void>;
  cancelParse(id: string): Promise<void>;
}

/** Per-document reparse; resolves after the backend accepts the request. */
export async function reparseDocument(api: DocumentMutationApi, id: string): Promise<void> {
  await api.reparse(id);
}

/** Cancel parse for every document currently pending/processing/finalizing. */
export async function cancelParseDocuments(
  api: DocumentMutationApi,
  documents: readonly { id: string; parse_status?: string }[],
): Promise<string[]> {
  const targets = documents.filter((document) => documentRowActions(document.parse_status).canCancelParse);
  for (const document of targets) await api.cancelParse(document.id);
  return targets.map((document) => document.id);
}
