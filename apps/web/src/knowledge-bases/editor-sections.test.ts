import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { normalizeKnowledgeEditorSection, visibleKnowledgeEditorSections } from './editor-sections';

describe('knowledge editor section topology', () => {
  it('keeps the Vue group order for document edit mode', () => {
    assert.deepEqual(visibleKnowledgeEditorSections({ type: 'document', editing: true }).map((group) => group.key),
      ['basic', 'processing', 'data', 'integration', 'management']);
  });

  it('does not expose document-only or edit-only sections for FAQ creation', () => {
    const sections = visibleKnowledgeEditorSections({ type: 'faq', editing: false }).flatMap((group) => group.items);
    assert.deepEqual(sections, ['basic', 'models', 'faq']);
  });

  it('falls back to basic when a stale section becomes hidden', () => {
    assert.equal(normalizeKnowledgeEditorSection('share', { type: 'faq', editing: false }), 'basic');
    assert.equal(normalizeKnowledgeEditorSection('faq', { type: 'faq', editing: false }), 'faq');
  });
});
