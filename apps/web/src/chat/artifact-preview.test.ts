import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MessageList } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('message artifact rows expose preview and protected download actions', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{ id: 'message-1', session_id: 'session-1', role: 'assistant', content: 'done', artifacts: [{ index: 0, file_name: 'guide.md', file_type: 'text/markdown' }] }],
    onArtifactPreview: async () => ({ body: '# guide', contentType: 'text/markdown' }),
    onArtifactDownload: async () => undefined,
  }));
  assert.match(html, />Preview<\/button>/);
  assert.match(html, />Download<\/button>/);
});
