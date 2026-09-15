import assert from 'node:assert/strict';
import test from 'node:test';

import { attachmentUploadsFromFiles, imageDataUrisFromFiles, resolveEmbedLocale, resolveEmbedUploadCapabilities, sourceListFromReferences } from './embed-ui.ts';

// Vue baselines: EmbedPage.vue applies the channel default_locale;
// EmbedBotMessage.vue renders knowledge_references as a source list;
// useEmbedChatSession.ts converts picked files into data-URI payloads.

test('channel default_locale wins, then URL locale, then en-US', () => {
  assert.equal(resolveEmbedLocale({ default_locale: 'zh-CN' }, 'ja-JP'), 'zh-CN');
  assert.equal(resolveEmbedLocale({ default_locale: 'klingon' }, 'ja-JP'), 'ja-JP');
  assert.equal(resolveEmbedLocale({}, 'xx-YY'), 'en-US');
  assert.equal(resolveEmbedLocale({ default_locale: 'ko-KR' }, ''), 'ko-KR');
  assert.equal(resolveEmbedLocale(undefined, ''), 'en-US');
});

test('upload controls require the same permission conjunction as the Vue embed', () => {
  assert.deepEqual(resolveEmbedUploadCapabilities({ allow_file_upload: true, agent_image_upload_enabled: true }), {
    allowFileUpload: true,
    allowImageUpload: true,
  });
  assert.deepEqual(resolveEmbedUploadCapabilities({ allow_file_upload: true, agent_image_upload_enabled: false }), {
    allowFileUpload: false,
    allowImageUpload: false,
  });
  assert.deepEqual(resolveEmbedUploadCapabilities({ allow_file_upload: false, agent_image_upload_enabled: true }), {
    allowFileUpload: false,
    allowImageUpload: false,
  });
});

test('knowledge references map to a flat source list', () => {
  const references = [
    { knowledge_title: 'Setup guide', knowledge_id: 'kb-1', chunk_id: 'c1', score: 0.9 },
    { knowledge_title: 'FAQ', knowledge_id: 'kb-2' },
    { knowledge_title: '', knowledge_id: 'kb-3' },
    'garbage',
  ];
  const sources = sourceListFromReferences(references);
  assert.deepEqual(sources, [
    { title: 'Setup guide', knowledgeId: 'kb-1', chunkId: 'c1' },
    { title: 'FAQ', knowledgeId: 'kb-2', chunkId: '' },
    { title: 'kb-3', knowledgeId: 'kb-3', chunkId: '' },
  ]);
  assert.deepEqual(sourceListFromReferences(undefined), []);
});

test('attachment uploads become data-URI payloads with names and sizes', async () => {
  const file = new File(['hello'], 'notes.txt', { type: 'text/plain' });
  Object.defineProperty(file, 'size', { value: 5 });
  const uploads = await attachmentUploadsFromFiles([file]);
  assert.equal(uploads.length, 1);
  assert.equal(uploads[0].file_name, 'notes.txt');
  assert.equal(uploads[0].file_size, 5);
  assert.ok(uploads[0].data.startsWith('data:text/plain;base64,'));
  assert.deepEqual(await attachmentUploadsFromFiles([]), []);
});

test('image uploads become data-URI payloads', async () => {
  const file = new File(['png'], 'p.png', { type: 'image/png' });
  const images = await imageDataUrisFromFiles([file]);
  assert.equal(images.length, 1);
  assert.ok(images[0].startsWith('data:image/png;base64,'));
});
