import type { KnowledgeDocument } from '@weknora/api-client';
import { previewKindForFile, previewStatus, type KnowledgePreviewKind } from '@weknora/domain/knowledge/preview';

export interface KnowledgeDocumentPreviewModel {
  kind: KnowledgePreviewKind;
  ready: boolean;
  path: string;
  fileName: string;
}

export function buildDocumentPreview(document: KnowledgeDocument, previewPath: string): KnowledgeDocumentPreviewModel {
  const fileName = document.file_name || document.title || 'document';
  const status = previewStatus(document);
  return { kind: previewKindForFile(fileName), ready: status.kind === 'ready', path: previewPath, fileName };
}
