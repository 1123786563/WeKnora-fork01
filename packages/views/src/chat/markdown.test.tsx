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

/*
 * Vue renders content images through t-image, whose error state shows the
 * 图片无法显示 placeholder with a 预览 preview trigger when the load fails
 * (WikiBrowser/browser evidence 2026-09-19). Markdown and raw-HTML images both
 * get the same wrapper + hidden fallback markup.
 */
test('markdown images render inside the chat-image wrapper with the hidden fallback', () => {
  const html = renderChatMarkdown('![图](https://example.com/a.png)');
  assert.match(html, /data-wk-chat-image/);
  assert.match(html, /data-wk-chat-image-img/);
  assert.match(html, /class="wk-chat-image-error"/);
  assert.match(html, /图片无法显示/);
  assert.match(html, /data-wk-chat-image-preview/);
  assert.match(html, /预览/);
});

test('raw <img> tags are whitelisted with safe-src enforcement', () => {
  const safe = renderChatMarkdown('<img src="https://example.com/b.png" alt="截图">');
  assert.match(safe, /data-wk-chat-image/);
  assert.match(safe, /src="https:\/\/example\.com\/b\.png"/);
  assert.match(safe, /alt="截图"/);
  assert.match(safe, /class="wk-chat-image-error"/);

  const unsafe = renderChatMarkdown('<img src="javascript:alert(1)" alt="x">');
  assert.match(unsafe, /<p>无效的图片链接<\/p>/);
  assert.doesNotMatch(unsafe, /javascript:/i);

  // Raw non-img HTML stays escaped (only <img> is whitelisted).
  const html = renderChatMarkdown('<div onclick="alert(1)">hi</div>');
  assert.match(html, /&lt;div onclick=&quot;alert\(1\)&quot;&gt;hi&lt;\/div&gt;/);
  assert.doesNotMatch(html, /<div onclick/);
});

test('raw images inside fenced code blocks are not whitelisted', () => {
  const html = renderChatMarkdown('```\n<img src="https://example.com/b.png">\n```');
  assert.match(html, /&lt;img src=&quot;https:\/\/example\.com\/b\.png&quot;&gt;/);
  assert.doesNotMatch(html, /data-wk-chat-image/);
});

test('image fallback labels carry the host-provided locale strings', () => {
  const html = renderChatMarkdown('![x](https://example.com/a.png)', { imageFailedLabel: 'Image unavailable', imagePreviewLabel: 'Preview' });
  assert.match(html, /Image unavailable/);
  assert.match(html, /Preview/);
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
