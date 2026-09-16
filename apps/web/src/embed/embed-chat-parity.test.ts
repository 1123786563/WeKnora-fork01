import assert from 'node:assert/strict';
import test from 'node:test';

// R442/A3 — Vue embed parity for history backfill, references and suggested
// questions. Vue behavior:
// - history: frontend/src/composables/useEmbedChatSession.ts resetAndLoad ->
//   getmsgList(getEmbedMessageList, limit 20) -> handleMsgList renders stored
//   messages after a session resume.
// - references: frontend/src/composables/useChatStreamHandler.ts
//   extractKnowledgeReferences (L156-164) reads data.knowledge_references ||
//   data.data.references || data.data.knowledge_references off the SSE events
//   and attaches them to the assistant row; docInfo.vue renders them grouped
//   by document with a click popup showing the chunk content.
// - suggested questions: frontend/src/views/embed/EmbedChatCore.vue
//   fetchSuggestedQuestions -> GET suggested-questions, shown only before the
//   visitor has sent anything when show_suggested_questions is on; clicking a
//   card sends that question.
import { extractStreamReferences, mapHistoryMessages, normalizeSuggestedQuestions, referenceHeadline, type EmbedChatMessage } from './chat-data.ts';
import { embedText } from './messages.ts';

test('mapHistoryMessages mirrors the Vue session rows into the entry face', () => {
  const rows = [
    { role: 'user', content: 'hello' },
    { role: 'assistant', content: 'hi there', knowledge_references: [{ content: 'chunk', knowledge_title: 'Doc' }] },
    { role: 'system', content: 'noise' },
    { content: 'no role' },
    { role: 'assistant', content: 42 },
  ];
  const mapped = mapHistoryMessages(rows as Record<string, unknown>[]);
  assert.deepEqual(
    mapped.map((m) => m.role),
    ['user', 'assistant', 'assistant'],
  );
  assert.equal(mapped[0].content, 'hello');
  assert.equal(mapped[1].content, 'hi there');
  assert.equal(mapped[1].references?.length, 1);
  // Numeric content is stringified like the Vue String(session.content || '').
  assert.equal(mapped[2].content, '42');
});

test('extractStreamReferences reads the three Vue SSE sources in order', () => {
  const refs = [{ content: 'a' }];
  assert.equal(extractStreamReferences({ knowledge_references: refs }), refs);
  assert.equal(extractStreamReferences({ data: { references: refs } }), refs);
  assert.equal(extractStreamReferences({ data: { knowledge_references: refs } }), refs);
  assert.deepEqual(extractStreamReferences({ data: { references: 'nope' } }), []);
  assert.deepEqual(extractStreamReferences({}), []);
  assert.deepEqual(extractStreamReferences(null), []);
});

test('normalizeSuggestedQuestions keeps Vue question strings and drops empties', () => {
  assert.deepEqual(
    normalizeSuggestedQuestions([{ question: ' A ' }, { question: '' }, {}, { question: 'B' }]),
    ['A', 'B'],
  );
  assert.deepEqual(normalizeSuggestedQuestions(undefined), []);
  assert.deepEqual(normalizeSuggestedQuestions('nope' as unknown as Record<string, unknown>[]), []);
});

test('referenceHeadline reproduces the Vue docInfo headerText branches', () => {
  const doc = { knowledge_title: 'D' };
  const web = { chunk_type: 'web_search', url: 'https://x' };
  // Every non-web chunk belongs to a group (Vue key: knowledge_id ||
  // knowledge_title || item.id), so a bare chunk still counts as one document.
  assert.equal(referenceHeadline([{ content: 'c' }], 'en-US'), 'Referenced 1 document(s)');
  assert.equal(referenceHeadline([web], 'en-US'), 'Referenced 1 web result(s)');
  assert.equal(referenceHeadline([doc, web], 'en-US'), 'Referenced 1 document(s) and 1 web result(s)');
  assert.equal(referenceHeadline([doc, web], 'zh-CN'), '引用了1篇文档和1条网页');
});

test('referenceHeadline counts document GROUPS, not raw chunks (docInfo groupedKnowledgeRefs)', () => {
  const sameDoc = { knowledge_id: 'k-1', knowledge_title: 'Alpha' };
  const chunks = [
    { knowledge_id: 'k-1', knowledge_title: 'Alpha', content: 'chunk-1' },
    { knowledge_id: 'k-1', knowledge_title: 'Alpha', content: 'chunk-2' },
    { knowledge_id: 'k-2', knowledge_title: 'Beta', content: 'chunk-3' },
    { knowledge_title: 'Gamma', content: 'chunk-4' },
    { id: 'k-1', content: 'id-fallback-collapses-into-k-1' },
  ];
  assert.equal(referenceHeadline(chunks, 'en-US'), 'Referenced 3 document(s)', '5 chunks collapse to 3 groups by knowledge_id/title/id');
  assert.equal(
    referenceHeadline([...chunks, { chunk_type: 'web_search', url: 'https://y' }], 'zh-CN'),
    '引用了3篇文档和1条网页',
  );
});

test('new embed copy exists in all five locales and interpolates params', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    assert.ok(embedText(locale, 'suggestedQuestions').length > 0, `${locale} suggestedQuestions`);
    assert.ok(embedText(locale, 'referencesTitle', { count: 3 }).length > 0, `${locale} referencesTitle`);
    assert.ok(!embedText(locale, 'referencesDocCount', { count: 2 }).includes('{count}'), `${locale} interpolation`);
  }
  assert.equal(embedText('zh-CN', 'suggestedQuestions'), '你可以这样问我');
});

test('EmbedChatSurface renders history messages, references and suggested cards', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const messages: EmbedChatMessage[] = [
    { role: 'user', content: 'q1' },
    { role: 'assistant', content: 'a1', references: [{ content: 'chunk body', knowledge_title: 'DocA' }, { chunk_type: 'web_search', url: 'https://example.com/a', knowledge_title: 'Web' }] },
  ];
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages,
    suggestedQuestions: ['What is WeKnora?'],
    suggestedLoading: false,
    welcomeMessage: '',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  // History backfill renders both roles.
  assert.match(html, /q1/);
  assert.match(html, /a1/);
  // References block with the Vue headline and chunk popup body.
  assert.match(html, /Referenced 1 document\(s\) and 1 web result\(s\)/);
  assert.match(html, /chunk body/);
  assert.match(html, /https:\/\/example\.com\/a/);
  // Suggested cards only render before the visitor speaks (Vue
  // showSuggestedBlock requires !hasUserMessage), so they stay hidden here.
  assert.doesNotMatch(html, /You can ask me/);
});

test('EmbedChatSurface shows suggested cards before the visitor sends anything', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages: [],
    suggestedQuestions: ['What is WeKnora?'],
    suggestedLoading: false,
    welcomeMessage: 'welcome!',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  assert.match(html, /You can ask me/);
  assert.match(html, /What is WeKnora\?/);
  assert.match(html, /welcome!/);
});

test('EmbedChatSurface hides suggested cards once the visitor has sent a message', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages: [{ role: 'user', content: 'already asked' }],
    suggestedQuestions: ['Later question'],
    suggestedLoading: false,
    welcomeMessage: 'welcome!',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  // Vue showSuggestedBlock requires !hasUserMessage; the welcome bubble also
  // hides after the visitor speaks.
  assert.doesNotMatch(html, /Later question/);
  assert.doesNotMatch(html, /welcome!/);
});
