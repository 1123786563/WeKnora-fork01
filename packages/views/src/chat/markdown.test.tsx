import assert from 'node:assert/strict';
import test from 'node:test';

import { renderChatMarkdown } from './markdown.ts';
import { messageArtifactItems, renderMessageHtml } from './message-list.tsx';

test('renders Markdown structures used by assistant answers', () => {
  const html = renderChatMarkdown([
    '# 标题',
    '',
    '| 名称 | 值 |',
    '| --- | --- |',
    '| CJK | 中文 |',
    '',
    '```mermaid',
    'graph TD; A-->B',
    '```',
    '',
    '公式 $x^2$。',
  ].join('\n'));

  assert.match(html, /<h1>标题<\/h1>/);
  assert.match(html, /<table>/);
  assert.match(html, /data-markdown-diagram="mermaid"/);
  assert.match(html, /class="katex"/);
  assert.match(html, /中文/);
});

test('escapes raw HTML and drops unsafe link and image destinations', () => {
  const html = renderChatMarkdown([
    '<script>alert(1)</script>',
    '[坏链接](javascript:alert(1))',
    '![坏图片](javascript:alert(1))',
  ].join('\n'));

  assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(html, /javascript:/i);
  assert.doesNotMatch(html, /<script/i);
});

/*
 * Vue botmsg.vue markdown image contract (R464): when the image destination
 * fails isValidImageURL, the renderer emits the localized placeholder
 * paragraph `invalidImageHtml: () => `<p>${t('error.invalidImageLink')}</p>``
 * instead of silently dropping the image or showing the alt text.
 */
test('invalid image destinations render the Vue invalid-image placeholder paragraph', () => {
  const html = renderChatMarkdown('![替代文本](javascript:alert(1))');
  assert.match(html, /<p>无效的图片链接<\/p>/);
  assert.doesNotMatch(html, /javascript:/i);
  assert.doesNotMatch(html, /替代文本/);

  const relative = renderChatMarkdown('![图](not-a-url.png)');
  assert.match(relative, /<p>无效的图片链接<\/p>/);
});

test('the invalid-image placeholder carries the host-provided locale label', () => {
  const html = renderChatMarkdown('![alt](javascript:alert(1))', { invalidImageLabel: 'Invalid image link' });
  assert.match(html, /<p>Invalid image link<\/p>/);
});

test('valid image destinations still render img elements', () => {
  const html = renderChatMarkdown('![图](https://example.com/a.png)');
  assert.match(html, /<img src="https:\/\/example\.com\/a\.png" alt="图"/);
});

test('turns citation protocol tags into accessible reference buttons', () => {
  const html = renderChatMarkdown('答案 <kb doc="guide.md" chunk_id="chunk-1" />');

  assert.match(html, /data-citation-id="chunk-1"/);
  assert.match(html, /aria-label="引用 guide\.md"/);
  assert.match(html, />guide\.md<\/button>/);
});

test('keeps an unclosed streaming fence in a readable code block', () => {
  const html = renderChatMarkdown('```python\nprint("中文")');

  assert.match(html, /<pre><code class="language-python">/);
  assert.match(html, /print\(&quot;中文&quot;\)/);
});

test('message rendering uses the shared safe Markdown renderer', () => {
  const html = renderMessageHtml({
    id: 'message-1',
    session_id: 'session-1',
    role: 'assistant',
    content: '**回答**',
  });

  assert.match(html, /<strong>回答<\/strong>/);
  assert.doesNotMatch(html, /white-space/);
});

test('message rendering threads the invalid-image label into the shared renderer', () => {
  const html = renderMessageHtml({ content: '![x](javascript:alert(1))' }, 'Invalid image link');
  assert.match(html, /<p>Invalid image link<\/p>/);
});

test('message artifacts retain only public metadata for protected download actions', () => {
  const artifacts = messageArtifactItems({
    artifacts: [{ index: 0, file_name: 'report.csv', file_type: 'text/csv', file_size: 4, source_path: '/private/report.csv' }],
  });

  assert.deepEqual(artifacts, [{ index: 0, fileName: 'report.csv', fileType: 'text/csv', fileSize: 4 }]);
});
