import assert from 'node:assert/strict';
import test from 'node:test';

import { toolResultPresentation } from './tool-result.tsx';

test('maps a known tool display type to a readable structured presentation', () => {
  assert.deepEqual(toolResultPresentation({
    id: 'tool-1',
    name: 'search',
    result: { display_type: 'search_results', data: { total: 2 } },
  }), {
    renderer: 'search-results',
    title: 'Search results',
    text: '{\n  "total": 2\n}',
    contentMode: 'structured-data',
  });
});

test('keeps an unknown tool result as readable plain text', () => {
  assert.deepEqual(toolResultPresentation({ id: 'tool-2', result: '<script>alert(1)</script>' }), {
    renderer: 'plain-text',
    title: 'Tool result',
    text: '<script>alert(1)</script>',
    contentMode: 'plain-text',
  });
});

test('redacts secret-shaped fields before presenting structured tool output', () => {
  const presentation = toolResultPresentation({
    id: 'tool-3',
    result: { display_type: 'search_results', data: { api_key: 'secret', nested: { access_token: 'token' } } },
  });

  assert.match(presentation.text, /\[redacted\]/);
  assert.doesNotMatch(presentation.text, /"api_key": "secret"|"access_token": "token"/);
});
