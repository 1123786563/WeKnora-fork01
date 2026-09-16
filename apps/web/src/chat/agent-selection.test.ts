import assert from 'node:assert/strict';
import test from 'node:test';

import { buildWebChatStreamOptions, initialAgentSelection, mergeChatAttachmentExtensions, normalizeChatAttachmentExtensions, resolveChatAttachmentLimits, shouldPollAttachmentStatus, validateChatAttachment } from './agent-selection.ts';

test('selected agent switches the web chat stream to agent mode with an explicit agent id', () => {
  assert.deepEqual(buildWebChatStreamOptions('session/1', '  Summarize this  ', 'agent/1'), {
    sessionId: 'session/1',
    mode: 'agent',
    body: { query: '  Summarize this  ', agent_enabled: true, agent_id: 'agent/1', channel: 'web' },
  });
});

test('no selected agent keeps the web chat stream on knowledge mode', () => {
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', ''), {
    sessionId: 'session-1',
    mode: 'knowledge',
    body: { query: 'Question', channel: 'web' },
  });
});

test('includes the selected model in the web chat stream body', () => {
  assert.equal(buildWebChatStreamOptions('session-1', 'Question', '', undefined, undefined, undefined, ' model-1 ').body.summary_model_id, 'model-1');
  assert.equal('summary_model_id' in buildWebChatStreamOptions('session-1', 'Question', '', undefined, undefined, undefined, '   ').body, false);
});

test('keeps a knowledge-base chat deep-link scoped to its knowledge base', () => {
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', '', 'kb-1'), {
    sessionId: 'session-1',
    mode: 'knowledge',
    body: { query: 'Question', channel: 'web', knowledge_base_ids: ['kb-1'] },
  });
});

test('includes only uploaded attachment ids in the stream body', () => {
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', '', undefined, ['att-1', 'att-2']), {
    sessionId: 'session-1',
    mode: 'knowledge',
    body: { query: 'Question', channel: 'web', attachment_ids: ['att-1', 'att-2'] },
  });
});

test('includes selected KB mentions and omits the field when none are selected', () => {
  const options = buildWebChatStreamOptions('session-1', 'Question', '', undefined, undefined, [
    { id: 'kb-1', name: '产品文档', type: 'kb', kb_type: 'document', kb_id: 'kb-1', kb_name: '产品文档' },
  ]);
  assert.deepEqual(options.body.mentioned_items, [
    { id: 'kb-1', name: '产品文档', type: 'kb', kb_type: 'document', kb_id: 'kb-1', kb_name: '产品文档' },
  ]);
  assert.deepEqual(options.body.knowledge_base_ids, ['kb-1']);
  assert.equal('mentioned_items' in buildWebChatStreamOptions('session-1', 'Question', '').body, false);
});

test('deduplicates an explicitly scoped KB and its KB mention', () => {
  const options = buildWebChatStreamOptions('session-1', 'Question', '', 'kb-1', undefined, [
    { id: 'kb-1', name: '产品文档', type: 'kb', kb_id: 'kb-1' },
    { id: 'kb-2', name: 'FAQ', type: 'kb' },
  ]);

  assert.deepEqual(options.body.knowledge_base_ids, ['kb-1', 'kb-2']);
});

test('preserves Vue resource mention types and identifiers in the stream body', () => {
  const mentions = [
    { id: 'file-1', name: '设计文档', type: 'file' as const, kb_id: 'kb-1', kb_name: '产品库' },
    { id: 'tag-1', name: '重要', type: 'tag' as const, kb_id: 'kb-1' },
    { id: 'mcp-1', name: 'Docs MCP', type: 'mcp' as const },
    { id: 'skill-1', name: 'summarize', type: 'skill' as const, skill_name: 'summarize' },
  ];
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', 'agent-1', undefined, undefined, mentions).body.mentioned_items, mentions);
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', 'agent-1', undefined, undefined, mentions).body.knowledge_ids, ['file-1']);
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', 'agent-1', undefined, undefined, mentions).body.tag_ids, ['tag-1']);
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', 'agent-1', undefined, undefined, mentions).body.mcp_service_ids, ['mcp-1']);
  assert.deepEqual(buildWebChatStreamOptions('session-1', 'Question', 'agent-1', undefined, undefined, mentions).body.skill_names, ['summarize']);
  const quickAnswer = buildWebChatStreamOptions('session-1', 'Question', '', undefined, undefined, mentions).body;
  assert.deepEqual(quickAnswer.knowledge_ids, ['file-1']);
  assert.deepEqual(quickAnswer.tag_ids, ['tag-1']);
  assert.equal('mcp_service_ids' in quickAnswer, false);
  assert.equal('skill_names' in quickAnswer, false);
});

test('validates Vue attachment limits before creating an upload row', () => {
  assert.equal(validateChatAttachment({ name: 'guide.pdf', size: 1024 }, 0), undefined);
  assert.equal(validateChatAttachment({ name: 'guide.exe', size: 1024 }, 0), 'unsupported-type');
  assert.equal(validateChatAttachment({ name: 'guide.pdf', size: 50 * 1024 * 1024 + 1 }, 0), 'too-large');
  assert.equal(validateChatAttachment({ name: 'guide.pdf', size: 1024 }, 5), 'too-many');
});

test('uses the runtime MAX_FILE_SIZE_MB override instead of a hard-coded cap', () => {
  const limits = resolveChatAttachmentLimits({ MAX_FILE_SIZE_MB: 1 }, '50');
  assert.equal(limits.maxSizeBytes, 1024 * 1024);
  assert.equal(validateChatAttachment({ name: 'guide.pdf', size: 1024 * 1024 + 1 }, 0, limits), 'too-large');
});

test('accepts parser engine extensions while retaining the static extension baseline', () => {
  const dynamic = normalizeChatAttachmentExtensions(['custom', '.PDF', 'url', '']);
  assert.deepEqual(dynamic, ['.custom', '.pdf']);
  assert.equal(validateChatAttachment({ name: 'notes.custom', size: 100 }, 0, undefined, ['.custom']), undefined);
  assert.equal(validateChatAttachment({ name: 'notes.pdf', size: 100 }, 0, undefined, ['.custom']), 'unsupported-type');
  assert.deepEqual(mergeChatAttachmentExtensions(undefined).includes('.pdf'), true);
  assert.deepEqual(mergeChatAttachmentExtensions(['custom']).includes('.custom'), true);
});

test('polls server attachment states until ready or failed', () => {
  assert.equal(shouldPollAttachmentStatus('uploaded'), true);
  assert.equal(shouldPollAttachmentStatus('processing'), true);
  assert.equal(shouldPollAttachmentStatus('ready'), false);
  assert.equal(shouldPollAttachmentStatus('failed'), false);
});

test('initial agent selection accepts a requested URL agent only when it is enabled', () => {
  const agents = [{ id: 'agent/1', name: 'Research' }, { id: 'agent/2', name: 'Disabled' }];
  assert.equal(initialAgentSelection('?agentId=agent%2F1', agents, ['agent/2']), 'agent/1');
  assert.equal(initialAgentSelection('?agentId=agent%2F2', agents, ['agent/2']), '');
  assert.equal(initialAgentSelection('?agentId=missing', agents, []), '');
});
