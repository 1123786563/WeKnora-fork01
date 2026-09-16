import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MessageList, resolveChatCopy } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('message artifact rows expose preview and protected download actions', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{ id: 'message-1', session_id: 'session-1', role: 'assistant', content: 'done', artifacts: [{ index: 0, file_name: 'guide.md', file_type: 'text/markdown' }] }],
    // Pin zh-CN: Node resolves navigator.language ('en-US'); assertions verify
    // the Vue-baseline zh copy byte-exact.
    copy: resolveChatCopy('zh-CN'),
    onArtifactPreview: async () => ({ body: '# guide', contentType: 'text/markdown' }),
    onArtifactDownload: async () => undefined,
  }));
  assert.match(html, />预览<\/button>/);
  assert.match(html, />下载<\/button>/);
});

test('message artifact rows keep the download-only artifact list available without a preview handler', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{ id: 'message-2', session_id: 'session-1', role: 'assistant', content: 'done', artifacts: [{ index: 0, file_name: 'chart.html', file_type: 'text/html' }] }],
    copy: resolveChatCopy('zh-CN'),
    onArtifactDownload: async () => undefined,
  }));

  assert.match(html, />产物<\/button>/);
});
