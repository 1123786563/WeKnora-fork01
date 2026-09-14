import { strict as assert } from 'node:assert';
import test from 'node:test';
import { formatMessage, type Locale } from '@weknora/i18n';

const locales: Locale[] = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'];
const keys = [
  'mobileChat.loadSessionsFailed', 'mobileChat.loadKnowledgeBasesFailed', 'mobileChat.loadMessagesFailed',
  'mobileChat.loadAttachmentsFailed', 'mobileChat.loadSteerQueueFailed', 'mobileChat.streamFailed',
  'mobileChat.resumeFailed', 'mobileChat.sendFailed', 'mobileChat.stopFailed', 'mobileChat.attachFailed',
  'mobileChat.downloadArtifactFailed', 'mobileChat.previewArtifactFailed', 'mobileChat.shareArtifactFailed',
  'mobileChat.noConversations', 'mobileChat.noKnowledgeBases', 'mobileChat.noQueuedInstructions',
  'mobileChat.responseFailedRetry', 'mobileChat.resuming', 'mobileChat.steerQueue', 'mobileChat.toolApprovalRequired',
  'mobileChat.mcpAuthorizationRequired', 'mobileChat.file', 'mobileChat.preview', 'mobileChat.summarize',
  'mobileChat.relatedFiles', 'mobileChat.summarizePrompt', 'mobileChat.relatedFilesPrompt', 'mobileChat.attachFile', 'mobileChat.chatMessage', 'mobileChat.title',
  'mobileChat.new', 'mobileChat.newConversation', 'mobileChat.approve', 'mobileChat.reject',
  'mobileChat.authorize', 'mobileChat.tool', 'mobileChat.after',
] as const;

test('ChatScreen user-facing state and control keys resolve in every supported locale', () => {
  for (const locale of locales) {
    for (const key of keys) {
      const value = formatMessage(locale, key);
      assert.notEqual(value, key, `${locale}:${key} must be translated`);
      assert.ok(value.trim(), `${locale}:${key} must not be empty`);
    }
  }
});

test('ChatScreen critical failure fallbacks retain distinct localized meanings', () => {
  for (const locale of locales) {
    const load = formatMessage(locale, 'mobileChat.loadMessagesFailed');
    const send = formatMessage(locale, 'mobileChat.sendFailed');
    const preview = formatMessage(locale, 'mobileChat.previewArtifactFailed');
    assert.notEqual(load, send, locale);
    assert.notEqual(send, preview, locale);
  }
});
