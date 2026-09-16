import assert from 'node:assert/strict';
import test from 'node:test';

import { attachmentUploadsFromFiles, embedAssistantLabel, embedErrorPrefix, embedMessageError, embedUploadLabel, formatEmbedConversationTimestamp, formatEmbedFileSize, imageDataUrisFromFiles, partitionUploadFiles, resolveEmbedLocale, resolveEmbedUploadCapabilities, sourceListFromReferences } from './embed-ui.ts';

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

test('upload labels follow the Vue embed locale table', () => {
  assert.equal(embedUploadLabel('zh-CN', 'file'), '上传附件');
  assert.equal(embedUploadLabel('zh-CN', 'image'), '上传图片');
  assert.equal(embedUploadLabel('en-US', 'file'), 'Upload file');
  assert.equal(embedUploadLabel('xx-YY', 'image'), 'Upload image');
});

test('assistant fallback labels stay localized', () => {
  assert.equal(embedAssistantLabel('zh-CN'), 'AI 助手');
  assert.equal(embedAssistantLabel('ja-JP'), 'AIアシスタント');
  assert.equal(embedAssistantLabel('xx-YY'), 'AI Assistant');
});

test('message error prefixes stay localized', () => {
  assert.equal(embedErrorPrefix('zh-CN'), '错误：');
  assert.equal(embedErrorPrefix('ja-JP'), 'エラー：');
  assert.equal(embedErrorPrefix('xx-YY'), 'Error: ');
});

test('message errors remain visible when a stream already has partial content', () => {
  assert.equal(embedMessageError('zh-CN', '回答中断'), '错误：回答中断');
  assert.equal(embedMessageError('en-US', ''), '');
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

test('upload validation matches the Vue limits and separates image files', () => {
  const image = new File(['png'], 'p.png', { type: 'image/png' });
  const text = new File(['hello'], 'notes.txt', { type: 'text/plain' });
  const result = partitionUploadFiles([image, text]);
  assert.deepEqual(result.images, [image]);
  assert.deepEqual(result.attachments, [text]);
  assert.deepEqual(result.rejected, []);
  assert.equal(partitionUploadFiles([image], 5).rejected[0]?.reason, 'count');
  assert.equal(partitionUploadFiles([new File(['x'], 'bad.svg', { type: 'image/svg+xml' })]).rejected[0]?.reason, 'type');
});

test('file-size labels match the Vue attachment cards', () => {
  assert.equal(formatEmbedFileSize(512), '512 B');
  assert.equal(formatEmbedFileSize(2048), '2.0 KB');
  assert.equal(formatEmbedFileSize(2 * 1024 * 1024), '2.0 MB');
  assert.equal(formatEmbedFileSize(0), '');
});

test('conversation timestamps follow the Vue today/yesterday/year buckets', () => {
  const now = new Date(2026, 8, 16, 12, 0);
  assert.equal(formatEmbedConversationTimestamp(new Date(2026, 8, 16, 9, 4).toISOString(), 'en-US', now), 'Today 09:04');
  assert.equal(formatEmbedConversationTimestamp(new Date(2026, 8, 15, 9, 4).toISOString(), 'en-US', now), 'Yesterday 09:04');
  assert.equal(formatEmbedConversationTimestamp(new Date(2026, 8, 2, 9, 4).toISOString(), 'en-US', now), 'Sep 2, 09:04');
  assert.equal(formatEmbedConversationTimestamp(new Date(2025, 8, 2, 9, 4).toISOString(), 'zh-CN', now), '2025年9月2日 09:04');
  assert.equal(formatEmbedConversationTimestamp('invalid', 'en-US', now), '');
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
