import assert from 'node:assert/strict';
import test from 'node:test';
import { renderMarkdownFixture } from './DevMarkdownPage.tsx';

test('development Markdown fixture uses the shared safe renderer', () => {
  const html = renderMarkdownFixture('# Fixture\n\n<script>alert(1)</script>\n\n```mermaid\ngraph TD; A-->B\n```');
  assert.match(html, /<h1>Fixture<\/h1>/);
  assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.match(html, /data-markdown-diagram="mermaid"/);
  assert.doesNotMatch(html, /<script>/);
});
