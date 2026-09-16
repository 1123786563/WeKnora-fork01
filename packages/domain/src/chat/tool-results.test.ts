import assert from 'node:assert/strict';
import test from 'node:test';

import { CHAT_MARKDOWN_POLICY, normalizeToolResult } from './tool-results.ts';

test('maps legacy display types to explicit renderer families', () => {
  assert.equal(normalizeToolResult({ displayType: 'search_results', data: { results: [] } }).renderer, 'search-results');
  assert.equal(normalizeToolResult({ displayType: 'wiki_replace_text', data: { updated_count: 1 } }).renderer, 'wiki-edit');
  assert.equal(normalizeToolResult({ toolName: 'discover_mcp_tools', output: '{}' }).renderer, 'mcp-discovery');
  assert.equal(normalizeToolResult({ toolName: 'call_mcp_tool', output: 'ok' }).renderer, 'mcp-call');
});

test('unknown tools fall back to readable plain text without an executable HTML channel', () => {
  const malicious = '<img src=x onerror="globalThis.pwned=true"><script>alert(1)</script>';
  const result = normalizeToolResult({
    displayType: 'future_tool',
    toolName: 'future_tool',
    output: malicious,
  });

  assert.deepEqual(result, {
    renderer: 'plain-text',
    displayType: 'future_tool',
    toolName: 'future_tool',
    success: null,
    text: malicious,
    data: {},
    contentMode: 'plain-text',
  });
  assert.equal('html' in result, false);
});

test('uses errors and structured values as readable fallback text instead of swallowing output', () => {
  assert.equal(normalizeToolResult({ displayType: 'future', error: 'permission denied' }).text, 'permission denied');
  assert.equal(normalizeToolResult({ displayType: 'future', output: { rows: [1, 2] } }).text, '{\n  "rows": [\n    1,\n    2\n  ]\n}');
});

test('requires raw HTML to stay disabled and rendered HTML to be sanitized at the DOM boundary', () => {
  assert.deepEqual(CHAT_MARKDOWN_POLICY, {
    rawHtml: 'escape',
    sanitizeRenderedHtml: true,
  });
});
