import assert from 'node:assert/strict';
import test from 'node:test';

import { buildWebChatStreamOptions, initialAgentSelection, resolveChatAttachmentLimits, shouldPollAttachmentStatus, validateChatAttachment } from './agent-selection.ts';

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
