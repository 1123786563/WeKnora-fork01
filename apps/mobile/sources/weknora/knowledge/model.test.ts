import { describe, expect, it } from 'vitest';
import {
  canKnowledgeAction,
  documentTitle,
  getKnowledgeViewState,
  knowledgePaths,
  type KnowledgePermission,
} from './model';

describe('mobile knowledge detail model', () => {
  it('uses the Vue-backed detail, preview, download, Wiki and FAQ paths', () => {
    expect(knowledgePaths.detail('doc/1')).toBe('/api/v1/knowledge/doc%2F1');
    expect(knowledgePaths.preview('doc/1')).toBe('/api/v1/knowledge/doc%2F1/preview');
    expect(knowledgePaths.download('doc/1')).toBe('/api/v1/knowledge/doc%2F1/download');
    expect(knowledgePaths.wikiPages('kb/1')).toBe('/api/v1/knowledgebase/kb%2F1/wiki/pages');
    expect(knowledgePaths.faqEntries('kb/1')).toBe('/api/v1/knowledge-bases/kb%2F1/faq/entries');
  });

  it('allows preview and download for viewers but never exposes Wiki or FAQ writes', () => {
    const viewer: KnowledgePermission = { canView: true, canEdit: false };
    expect(canKnowledgeAction('preview', viewer)).toBe(true);
    expect(canKnowledgeAction('download', viewer)).toBe(true);
    expect(canKnowledgeAction('wiki-edit', viewer)).toBe(false);
    expect(canKnowledgeAction('faq-edit', viewer)).toBe(false);
    expect(canKnowledgeAction('preview', { canView: false, canEdit: false })).toBe(false);
  });

  it('keeps loading, forbidden, error and ready states explicit', () => {
    expect(getKnowledgeViewState({ loading: true, permission: { canView: true, canEdit: false } })).toBe('loading');
    expect(getKnowledgeViewState({ loading: false, permission: { canView: false, canEdit: false } })).toBe('forbidden');
    expect(getKnowledgeViewState({ loading: false, permission: { canView: true, canEdit: false }, error: 'offline' })).toBe('error');
    expect(getKnowledgeViewState({ loading: false, permission: { canView: true, canEdit: false } })).toBe('ready');
  });

  it('prefers the document title and falls back to the file name', () => {
    expect(documentTitle({ id: '1', title: 'A title', file_name: 'a.md' })).toBe('A title');
    expect(documentTitle({ id: '1', file_name: 'a.md' })).toBe('a.md');
    expect(documentTitle({ id: '1' })).toBe('Untitled document');
  });
});
