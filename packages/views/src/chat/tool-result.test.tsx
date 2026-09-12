import assert from 'node:assert/strict';
import test from 'node:test';

import {
  databaseQueryView,
  grepResultsView,
  searchResultsView,
  shellExecView,
  toolResultPresentation,
  ToolResultView,
  webSearchResultsView,
} from './tool-result.tsx';

/* ---- existing presentation tests ---- */

test('maps a known tool display type to a readable structured presentation', () => {
  const presentation = toolResultPresentation({
    id: 'tool-1',
    name: 'search',
    result: { display_type: 'search_results', data: { total: 2 } },
  });
  assert.equal(presentation.renderer, 'search-results');
  assert.equal(presentation.title, 'Search results');
  assert.equal(presentation.contentMode, 'structured-data');
});

test('keeps an unknown tool result as readable plain text', () => {
  assert.deepEqual(toolResultPresentation({ id: 'tool-2', result: '<script>alert(1)</script>' }), {
    renderer: 'plain-text',
    title: 'Tool result',
    text: '<script>alert(1)</script>',
    contentMode: 'plain-text',
    data: {},
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

/* ---- SearchResults ---- */

const SEARCH_FIXTURE = {
  display_type: 'search_results',
  query: 'refund policy',
  results: [
    {
      result_index: 1, chunk_id: 'c1', knowledge_id: 'k1', knowledge_title: 'Refunds.pdf',
      content: 'Full refunds within 30 days.', score: 0.91, relevance_level: 'High Relevance',
      match_type: 'hybrid',
    },
    {
      result_index: 2, chunk_id: 'c2', knowledge_id: 'k1', knowledge_title: 'Refunds.pdf',
      content: 'Refunds are processed in 5 business days.', score: 0.74, relevance_level: 'Medium Relevance',
      match_type: 'hybrid',
    },
    {
      result_index: 3, chunk_id: 'c3', knowledge_id: 'k2', knowledge_title: 'FAQ.docx',
      content: 'How long do refunds take?', score: 0.62, relevance_level: 'Low Relevance',
      match_type: 'keyword', faq_standard_question: 'How long do refunds take?',
    },
  ],
};

test('searchResultsView groups chunks per document, FAQ entries stay separate', () => {
  const view = searchResultsView(SEARCH_FIXTURE);
  assert.equal(view.rows.length, 2);
  assert.equal(view.rows[0]!.title, 'Refunds.pdf');
  assert.match(view.rows[0]!.meta, /2/);
  assert.equal(view.rows[0]!.snippets.length, 2);
  assert.equal(view.rows[1]!.title, 'How long do refunds take?');
  assert.equal(view.query, 'refund policy');
});

test('searchResultsView reports an empty state', () => {
  assert.deepEqual(searchResultsView({ display_type: 'search_results' }), { query: '', rows: [] });
});

test('searchResultsView drops secret-shaped content via presentation redaction', () => {
  const presentation = toolResultPresentation({
    id: 't', name: 'search',
    result: { display_type: 'search_results', data: { results: [{ knowledge_id: 'k', knowledge_title: 'x', chunk_id: 'c', content: 'body text', auth_token: 'hunter2', result_index: 1 }] } },
  });
  assert.doesNotMatch(JSON.stringify(presentation.data), /hunter2/);
});

/* ---- WebSearchResults ---- */

const WEB_SEARCH_FIXTURE = {
  display_type: 'web_search_results',
  query: 'weknora',
  count: 2,
  results: [
    { result_index: 1, title: 'WeKnora docs', url: 'https://docs.example.com/weknora', snippet: 'Open-source knowledge retrieval.', published_at: '2024-05-01T00:00:00Z' },
    { result_index: 2, title: 'GitHub', url: 'https://github.com/example/weknora', snippet: 'Source code.', age: '2 days ago' },
  ],
};

test('webSearchResultsView lists title, url and snippet', () => {
  const view = webSearchResultsView(WEB_SEARCH_FIXTURE);
  assert.equal(view.rows.length, 2);
  assert.equal(view.rows[0]!.title, 'WeKnora docs');
  assert.equal(view.rows[0]!.url, 'https://docs.example.com/weknora');
  assert.equal(view.rows[0]!.snippet, 'Open-source knowledge retrieval.');
  assert.equal(view.rows[1]!.meta, '2 days ago');
});

test('webSearchResultsView tolerates missing results', () => {
  assert.deepEqual(webSearchResultsView({ display_type: 'web_search_results', query: 'q', count: 0, results: [] }).rows, []);
});

/* ---- DatabaseQuery ---- */

test('databaseQueryView formats columns and rows into a string table', () => {
  const view = databaseQueryView({
    display_type: 'database_query',
    columns: ['id', 'name'],
    row_count: 2,
    rows: [
      { id: 1, name: 'alpha' },
      { id: null, name: { nested: true } },
    ],
  });
  assert.deepEqual(view.columns, ['id', 'name']);
  assert.deepEqual(view.rows, [['1', 'alpha'], ['(null)', '{"nested":true}']]);
  assert.equal(view.rowCount, 2);
});

test('databaseQueryView handles a no-rows result', () => {
  const view = databaseQueryView({ display_type: 'database_query', columns: ['id'], row_count: 0, rows: [] });
  assert.deepEqual(view.rows, []);
});

/* ---- GrepResults ---- */

const GREP_FIXTURE = {
  display_type: 'grep_results',
  query: 'TODO',
  patterns: ['TODO'],
  result_count: 2,
  total_matches: 3,
  knowledge_results: [],
  chunk_results: [
    { chunk_id: 'c1', knowledge_id: 'k1', knowledge_base_id: 'kb1', knowledge_title: 'server.py', match_snippet: 'line 12: # TODO fix retry', title_match: false, chunk_index: 0 },
    { chunk_id: 'c2', knowledge_id: 'k2', knowledge_base_id: 'kb1', knowledge_title: 'client.py', match_snippet: 'line 40: // TODO refactor', title_match: true, chunk_index: 3 },
  ],
};

test('grepResultsView lists per-chunk rows with snippet content', () => {
  const view = grepResultsView(GREP_FIXTURE);
  assert.equal(view.rows.length, 2);
  assert.equal(view.rows[0]!.title, 'server.py');
  assert.equal(view.rows[0]!.snippet, 'line 12: # TODO fix retry');
  assert.equal(view.rows[1]!.meta, 'chunk #3');
  assert.equal(view.pattern, 'TODO');
});

test('grepResultsView falls back to knowledge_results rows', () => {
  const view = grepResultsView({
    display_type: 'grep_results',
    patterns: ['err'],
    knowledge_results: [{
      knowledge_id: 'k9', knowledge_base_id: 'kb', knowledge_title: 'guide.md',
      chunk_hit_count: 2, total_pattern_hits: 4, distinct_patterns: 1, pattern_counts: { err: 4 },
      match_snippet: 'line 7: err handling', title_match: false,
    }],
  });
  assert.equal(view.rows.length, 1);
  assert.equal(view.rows[0]!.meta, '2 chunk hits · 4 keyword hits');
});

test('grepResultsView empty state', () => {
  assert.deepEqual(grepResultsView({ display_type: 'grep_results', patterns: ['x'], knowledge_results: [] }).rows, []);
});

/* ---- ShellExec ---- */

test('shellExecView builds command and stream view from data', () => {
  const view = shellExecView({
    display_type: 'shell_exec',
    command: 'pytest -q', work_dir: '/repo', exit_code: 1, duration_ms: 1500,
    stdout: '1 failed', stderr: 'boom', stdout_truncated: false,
  });
  assert.equal(view.command, 'pytest -q');
  assert.equal(view.workDir, '/repo');
  assert.equal(view.exitCode, 1);
  assert.equal(view.durationLabel, '1.5s');
  assert.equal(view.stdout, '1 failed');
  assert.equal(view.stderr, 'boom');
  assert.equal(view.empty, false);
});

test('shellExecView falls back to tool arguments for the command', () => {
  const view = shellExecView({ exit_code: 0 }, { command: 'ls -la' }, '');
  assert.equal(view.command, 'ls -la');
  assert.equal(view.exitCode, 0);
});

test('shellExecView flags binary and empty output', () => {
  const binary = shellExecView({ stdout_binary: true, stderr_binary: true });
  assert.equal(binary.stdoutBinary, true);
  assert.equal(binary.stderrBinary, true);
  assert.equal(binary.empty, false);
  const empty = shellExecView({});
  assert.equal(empty.empty, true);
});

/* ---- dispatch ---- */

function renderType(result: unknown): string {
  const element = ToolResultView({ toolCall: { id: 't', name: 'x', result } }) as {
    props: { children: { props: { children: unknown } }[] };
  };
  const body = element.props.children[1] as { type: { name?: string } };
  return body.type.name ?? String(body.type);
}

test('ToolResultView selects the typed renderer by display type', () => {
  assert.equal(renderType({ display_type: 'search_results', data: {} }), 'SearchResultsRenderer');
  assert.equal(renderType({ display_type: 'web_search_results', data: {} }), 'WebSearchResultsRenderer');
  assert.equal(renderType({ display_type: 'database_query', data: {} }), 'DatabaseQueryRenderer');
  assert.equal(renderType({ display_type: 'grep_results', data: {} }), 'GrepResultsRenderer');
  assert.equal(renderType({ display_type: 'shell_exec', data: {} }), 'ShellExecRenderer');
  assert.equal(renderType('plain output'), 'GenericToolResultRenderer');
});
