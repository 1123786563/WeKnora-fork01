import assert from 'node:assert/strict';
import test from 'node:test';

import {
  chunkDetailView,
  databaseQueryView,
  documentInfoView,
  grepResultsView,
  knowledgeBaseListView,
  planView,
  relatedChunksView,
  searchResultsView,
  shellExecView,
  thinkingView,
  toolResultPresentation,
  ToolResultView,
  ChunkDetailRenderer,
  KnowledgeBaseListRenderer,
  RelatedChunksRenderer,
  webFetchView,
  webSearchResultsView,
  WebFetchRenderer,
} from './tool-result.tsx';
import { resolveChatCopy } from './chat-copy.ts';

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
  assert.equal(view.rows[1]!.meta, '1 chunk hits · 标题匹配');
  assert.equal(view.pattern, 'TODO');
});

test('grepResultsView groups chunk hits and localizes title-match metadata in every locale', () => {
  const data = {
    display_type: 'grep_results',
    patterns: ['err'],
    chunk_results: [
      { chunk_id: 'c1', knowledge_id: 'k1', knowledge_title: 'guide.md', match_snippet: 'first', title_match: false },
      { chunk_id: 'c2', knowledge_id: 'k1', knowledge_title: 'guide.md', match_snippet: 'second', title_match: true },
    ],
  };
  const expected = {
    'zh-CN': '标题匹配',
    'en-US': 'title',
    'ja-JP': 'タイトル一致',
    'ko-KR': '제목',
    'ru-RU': 'заголовок',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const view = grepResultsView(data, resolveChatCopy(locale));
    assert.equal(view.rows.length, 1, `${locale} should group chunks by document`);
    assert.match(view.rows[0]!.meta, new RegExp(label));
    assert.equal(view.rows[0]!.snippet, 'first');
  }
});

test('grepResultsView keeps FAQ groups distinct and uses FAQ metadata even when title matches', () => {
  const data = {
    display_type: 'grep_results',
    chunk_results: [
      { chunk_id: 'c1', faq_id: 'faq-1', knowledge_id: 'k1', faq_question: 'How do I reset it?', chunk_type: 'faq', match_snippet: 'reset', title_match: true },
      { chunk_id: 'c2', faq_id: 'faq-1', knowledge_id: 'k1', faq_question: 'How do I reset it?', chunk_type: 'faq', match_snippet: 'again', title_match: false },
      { chunk_id: 'c3', faq_id: 'faq-2', knowledge_id: 'k1', faq_question: 'How do I pay?', chunk_type: 'faq', match_snippet: 'pay', title_match: true },
    ],
  };
  const view = grepResultsView(data, resolveChatCopy('zh-CN'));
  assert.equal(view.rows.length, 2);
  assert.deepEqual(view.rows.map((row) => row.key), ['faq-1', 'faq-2']);
  assert.deepEqual(view.rows.map((row) => row.meta), ['FAQ 条目', 'FAQ 条目']);
  assert.deepEqual(view.rows.map((row) => row.snippet), ['reset', 'pay']);
  assert.equal(grepResultsView(data, resolveChatCopy('ja-JP')).rows[0]!.meta, 'FAQ項目');
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

test('grepResultsView localizes title-match metadata while keeping the default call compatible', () => {
  const data = {
    display_type: 'grep_results',
    patterns: ['err'],
    knowledge_results: [{ knowledge_id: 'k9', knowledge_title: 'guide.md', chunk_hit_count: 1, total_pattern_hits: 1, title_match: true }],
  };
  assert.match(grepResultsView(data).rows[0]!.meta, /标题匹配/);
  assert.match(grepResultsView(data, resolveChatCopy('en-US')).rows[0]!.meta, /title/);
  assert.match(grepResultsView(data, resolveChatCopy('ja-JP')).rows[0]!.meta, /タイトル一致/);
  assert.match(grepResultsView(data, resolveChatCopy('ko-KR')).rows[0]!.meta, /제목/);
  assert.match(grepResultsView(data, resolveChatCopy('ru-RU')).rows[0]!.meta, /заголовок/);
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

/* ---- ChunkDetail ---- */

test('chunkDetailView surfaces chunk content with metadata', () => {
  const view = chunkDetailView({
    chunk_id: 'c-9', knowledge_id: 'k-2', chunk_index: 4, content_length: 128,
    content: 'Refunds are processed within 5 business days.',
  });
  assert.equal(view.chunkId, 'c-9');
  assert.equal(view.knowledgeId, 'k-2');
  assert.equal(view.chunkIndexLabel, '#4');
  assert.equal(view.contentLength, 128);
  assert.match(view.content, /5 business days/);
});

test('ChunkDetailRenderer localizes the full-content label in every locale', () => {
  const expected = {
    'zh-CN': '完整内容',
    'en-US': 'Full content',
    'ja-JP': '全文',
    'ko-KR': '전체 내용',
    'ru-RU': 'Полный текст',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const element = ChunkDetailRenderer({
      data: { chunk_id: 'c-9', content: 'content' },
      copy: resolveChatCopy(locale),
    });
    assert.match(JSON.stringify(element), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

test('ChunkDetailRenderer localizes the chunk ID label in every locale', () => {
  const expected = {
    'zh-CN': '片段ID:',
    'en-US': 'Chunk ID:',
    'ja-JP': 'チャンクID:',
    'ko-KR': '청크 ID:',
    'ru-RU': 'ID фрагмента:',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const element = ChunkDetailRenderer({
      data: { chunk_id: 'c-9' },
      copy: resolveChatCopy(locale),
    });
    assert.match(JSON.stringify(element), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

test('ChunkDetailRenderer localizes the document ID label in every locale', () => {
  const expected = {
    'zh-CN': '文档ID:',
    'en-US': 'Document ID:',
    'ja-JP': 'ドキュメントID:',
    'ko-KR': '문서 ID:',
    'ru-RU': 'ID документа:',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const element = ChunkDetailRenderer({
      data: { knowledge_id: 'k-2' },
      copy: resolveChatCopy(locale),
    });
    assert.match(JSON.stringify(element), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

test('ChunkDetailRenderer localizes the content length label and unit in every locale', () => {
  const expected = {
    'zh-CN': ['内容长度:', '128 字'],
    'en-US': ['Content length:', '128 characters'],
    'ja-JP': ['コンテンツ長:', '128文字'],
    'ko-KR': ['내용 길이:', '128자'],
    'ru-RU': ['Длина содержимого:', '128 символов'],
  } as const;
  for (const [locale, [label, value]] of Object.entries(expected)) {
    const element = ChunkDetailRenderer({
      data: { chunk_id: 'c-9', content_length: 128 },
      copy: resolveChatCopy(locale),
    });
    const serialized = JSON.stringify(element);
    assert.match(serialized, new RegExp(label.replace(/[.*+?^${}()|[\\]\\]/g, '\\\\$&')));
    assert.match(serialized, new RegExp(value.replace(/[.*+?^${}()|[\\]\\]/g, '\\\\$&')));
  }
});

test('ChunkDetailRenderer localizes the position label in every locale', () => {
  const expected = {
    'zh-CN': '位置:',
    'en-US': 'Position:',
    'ja-JP': '位置:',
    'ko-KR': '위치:',
    'ru-RU': 'Позиция:',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const element = ChunkDetailRenderer({
      data: { chunk_id: 'c-9', chunk_index: 4 },
      copy: resolveChatCopy(locale),
    });
    assert.match(JSON.stringify(element), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

/* ---- RelatedChunks ---- */

test('relatedChunksView lists chunk positions and scores', () => {
  const view = relatedChunksView({
    chunks: [
      { index: 1, chunk_id: 'c1', chunk_index: 3, content: 'alpha', score: 0.8123 },
      { index: 2, chunk_id: 'c2', chunk_index: 7, content: 'beta' },
    ],
  });
  assert.equal(view.rows.length, 2);
  assert.equal(view.rows[0]!.indexLabel, '#1');
  assert.equal(view.rows[0]!.positionLabel, 'chunk #3');
  assert.equal(view.rows[0]!.score, 0.8123);
  assert.equal(view.rows[1]!.score, null);
  assert.deepEqual(relatedChunksView({}).rows, []);
});

test('RelatedChunksRenderer localizes the empty state in every locale', () => {
  const expected = {
    'zh-CN': '没有找到相关片段',
    'en-US': 'No related chunks found',
    'ja-JP': '関連するチャンクが見つかりません',
    'ko-KR': '관련 청크를 찾을 수 없습니다',
    'ru-RU': 'Связанные фрагменты не найдены',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const emptyObject = RelatedChunksRenderer({ data: {}, copy: resolveChatCopy(locale) });
    const emptyChunks = RelatedChunksRenderer({ data: { chunks: [] }, copy: resolveChatCopy(locale) });
    assert.match(JSON.stringify(emptyObject), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
    assert.match(JSON.stringify(emptyChunks), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

/* ---- KnowledgeBaseList ---- */

test('knowledgeBaseListView lists names, ids and descriptions', () => {
  const view = knowledgeBaseListView({
    count: 2,
    knowledge_bases: [
      { index: 1, id: 'kb-1', name: 'HR policies', description: 'Internal HR docs' },
      { index: 2, id: 'kb-2', name: 'FAQ', description: '' },
    ],
  });
  assert.equal(view.count, 2);
  assert.equal(view.rows[0]!.name, 'HR policies');
  assert.equal(view.rows[0]!.id, 'kb-1');
  assert.equal(view.rows[1]!.description, '');
});

test('KnowledgeBaseListRenderer localizes the count in every locale', () => {
  const expected = {
    'zh-CN': '共 2 个知识库',
    'en-US': '2 knowledge bases',
    'ja-JP': '2件のナレッジベース',
    'ko-KR': '총 2개 지식베이스',
    'ru-RU': '2 баз знаний',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    const element = KnowledgeBaseListRenderer({
      data: { count: 2, knowledge_bases: [{ id: 'kb-1', name: 'HR policies' }] },
      copy: resolveChatCopy(locale),
    });
    assert.match(JSON.stringify(element), new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

/* ---- DocumentInfo ---- */

test('documentInfoView collects title, type, size and metadata per document', () => {
  const view = documentInfoView({
    documents: [
      {
        title: 'Refunds.pdf', type: 'pdf', source: 'upload', knowledge_id: 'k-1',
        chunk_count: 12, file_name: 'Refunds.pdf', file_type: 'pdf', file_size: 2048,
        metadata: { author: 'Finance', pages: 9 },
      },
    ],
  });
  const row = view.rows[0]!;
  assert.equal(row.title, 'Refunds.pdf');
  assert.equal(row.sourceLabel, 'pdf · upload');
  assert.equal(row.chunkCount, 12);
  assert.equal(row.fileLabel, 'Refunds.pdf · (pdf) · 2.0 KB');
  assert.deepEqual(row.metadata, [
    { key: 'author', value: 'Finance' },
    { key: 'pages', value: '9' },
  ]);
  assert.deepEqual(documentInfoView({}).rows, []);
});

/* ---- WebFetch ---- */

test('webFetchView shows url, status and extracted content summary', () => {
  const view = webFetchView({
    results: [
      { url: 'https://docs.example.com/guide', status: 'success', summary: 'How to install.', content_length: 4321, method: 'get' },
      { url: 'https://blocked.example.com/x', status: 'failed', error_code: 'TIMEOUT', error_message: 'timed out' },
    ],
  });
  assert.equal(view.rows[0]!.hostname, 'docs.example.com');
  assert.equal(view.rows[0]!.statusKind, 'ok');
  assert.equal(view.rows[0]!.summary, 'How to install.');
  assert.equal(view.rows[0]!.method, 'GET');
  assert.equal(view.rows[0]!.contentLengthLabel, '4321 chars');
  assert.equal(view.rows[1]!.statusKind, 'failed');
  assert.equal(view.rows[1]!.errorCode, 'TIMEOUT');
  assert.equal(view.rows[1]!.errorMessage, 'timed out');
});

test('WebFetchRenderer localizes the empty-url fallback while preserving URL rendering', () => {
  const element = WebFetchRenderer({
    data: { results: [{ status: 'failed' }, { url: 'https://docs.example.com/guide', status: 'success' }] },
    copy: resolveChatCopy('ja-JP'),
  });
  const serialized = JSON.stringify(element);
  assert.match(serialized, /不明なリンク/);
  assert.match(serialized, /docs\.example\.com/);
  assert.doesNotMatch(serialized, /Unknown link/);
});

test('WebFetchRenderer localizes summary, partial-content and raw-text labels', () => {
  const element = WebFetchRenderer({
    data: {
      results: [{
        url: 'https://docs.example.com/guide',
        status: 'success',
        summary_status: 'failed',
        raw_content: '原始页面内容',
        content_length: 8,
        truncated: true,
      }],
    },
    copy: resolveChatCopy('ja-JP'),
  });
  const serialized = JSON.stringify(element);
  assert.match(serialized, /要約/);
  assert.match(serialized, /要約失敗/);
  assert.match(serialized, /ページの一部/);
  assert.match(serialized, /元テキスト/);
  assert.doesNotMatch(serialized, /Summary generation failed/);
  assert.doesNotMatch(serialized, /Raw text/);
  assert.doesNotMatch(serialized, /truncated/);
});

test('WebFetchRenderer prefers a rendered summary over its failed summary status', () => {
  const element = WebFetchRenderer({
    data: {
      results: [{
        summary: '可用な要約',
        summary_status: 'failed',
        summary_error_code: 'SUMMARY_TIMEOUT',
        summary_error_message: 'summary generation timed out',
      }],
    },
    copy: resolveChatCopy('ja-JP'),
  });
  const serialized = JSON.stringify(element);
  assert.match(serialized, /可用な要約/);
  assert.doesNotMatch(serialized, /要約失敗/);
  assert.doesNotMatch(serialized, /SUMMARY_TIMEOUT/);
  assert.doesNotMatch(serialized, /summary generation timed out/);
});

test('webFetchView keeps summary error fallback when no summary is available', () => {
  const view = webFetchView({
    results: [{ summary_status: 'failed', summary_error_code: 'SUMMARY_TIMEOUT', summary_error_message: 'summary generation timed out' }],
  });
  assert.equal(view.rows[0]!.errorCode, 'SUMMARY_TIMEOUT');
  assert.equal(view.rows[0]!.errorMessage, 'summary generation timed out');
});

/* ---- Thinking / Plan ---- */

test('thinkingView returns the reasoning text without falling through to raw output', () => {
  assert.equal(thinkingView({ thought: 'Let me check the refund policy.' }), 'Let me check the refund policy.');
  assert.equal(thinkingView({}, 'fallback reasoning'), 'fallback reasoning');
  assert.equal(thinkingView({}), '');
});

test('planView keeps step status indicators', () => {
  const view = planView({
    task: 'Answer the refund question',
    steps: [
      { id: 's1', description: 'Search knowledge base', status: 'completed' },
      { id: 's2', description: 'Draft answer', status: 'in_progress' },
      { id: 's3', description: 'Cite sources', status: 'pending' },
      { id: 's4', description: 'Unknown status', status: 'weird' },
    ],
  });
  assert.equal(view.task, 'Answer the refund question');
  assert.deepEqual(view.steps.map((step) => step.status), ['completed', 'in_progress', 'pending', 'pending']);
  assert.deepEqual(planView({}).steps, []);
});

/* ---- Renderer routing ---- */

test('typed renderers cover the expanded display types', () => {
  for (const displayType of ['chunk_detail', 'related_chunks', 'knowledge_base_list', 'document_info', 'web_fetch_results', 'thinking', 'plan']) {
    const presentation = toolResultPresentation({
      id: 't',
      result: { display_type: displayType, data: {} },
    });
    assert.notEqual(presentation.renderer, 'plain-text', displayType);
    assert.equal(presentation.contentMode, 'structured-data', displayType);
  }
});
