import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { normalizeKnowledgeEditorSection, visibleKnowledgeEditorSections } from './editor-sections';

describe('knowledge editor section topology', () => {
  it('keeps the Vue group order for document edit mode', () => {
    assert.deepEqual(visibleKnowledgeEditorSections({ type: 'document', editing: true }).map((group) => group.key),
      ['basic', 'processing', 'data', 'integration', 'management']);
    assert.deepEqual(visibleKnowledgeEditorSections({ type: 'document', editing: true }).flatMap((group) => group.items), [
      'basic', 'models', 'vectorStore',
      'parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced', 'storage', 'datasource',
      'share', 'activity',
    ]);
  });

  it('keeps the Vue base sections for FAQ creation, including vector store', () => {
    const groups = visibleKnowledgeEditorSections({ type: 'faq', editing: false });
    assert.deepEqual(groups.map((group) => group.key), ['basic']);
    const sections = groups.flatMap((group) => group.items);
    assert.deepEqual(sections, ['basic', 'models', 'vectorStore', 'faq']);
  });

  it('keeps the Vue vector-store binding section for FAQ editing', () => {
    const groups = visibleKnowledgeEditorSections({ type: 'faq', editing: true, canViewActivity: true });
    assert.deepEqual(groups.map((group) => group.key), ['basic', 'integration', 'management']);
    const sections = groups.flatMap((group) => group.items);
    assert.deepEqual(sections, ['basic', 'models', 'vectorStore', 'faq', 'share', 'activity']);
  });

  it('hides the activity section when the Vue owner/admin gate fails', () => {
    const sections = visibleKnowledgeEditorSections({ type: 'document', editing: true, canViewActivity: false }).flatMap((group) => group.items);
    assert.equal(sections.includes('activity'), false);
    assert.ok(visibleKnowledgeEditorSections({ type: 'document', editing: true, canViewActivity: true })
      .flatMap((group) => group.items).includes('activity'));
  });

  it('allows callers to apply Vue permission visibility to edit-only sections', () => {
    const sections = visibleKnowledgeEditorSections({
      type: 'document',
      editing: true,
      canShare: false,
      canViewActivity: false,
    }).flatMap((group) => group.items);
    assert.equal(sections.includes('share'), false);
    assert.equal(sections.includes('activity'), false);
    assert.ok(sections.includes('datasource'));
  });

  it('falls back to basic when a stale section becomes hidden', () => {
    assert.equal(normalizeKnowledgeEditorSection('share', { type: 'faq', editing: false }), 'basic');
    assert.equal(normalizeKnowledgeEditorSection('faq', { type: 'faq', editing: false }), 'faq');
  });
});
