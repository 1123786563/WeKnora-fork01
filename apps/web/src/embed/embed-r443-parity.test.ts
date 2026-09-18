import assert from 'node:assert/strict';
import test from 'node:test';

// R443/A2 — embed chat tail: scroll-paged history, per-message follow-up
// suggestions and citation pills. Vue contracts:
// - pagination: frontend/src/composables/useEmbedChatSession.ts getmsgList —
//   cursor = batch[0].created_at (server returns newest-first), next page is
//   fetched with before_time=<cursor> when the scroll container hits the top,
//   batches are PREPENDED, pagination stops on empty batch / short batch /
//   unchanged cursor / fetch failure, and the scroll offset is restored by
//   scrollTop = newScrollHeight - oldScrollHeight (useChatStreamHandler
//   handleMsgList L490-510).
// - follow-ups: frontend/src/views/embed/EmbedChatCore.vue loadFollowUpsSuggestions —
//   per completed assistant message, GET/POST sessions/:id/messages/:mid/suggestions,
//   poll while status==='generating', render only when status==='ready'
//   (message.suggestionSet = set?.status === 'ready' ? set : null); message id
//   is String(message.id || message.assistant_message_id || ''); clicking a
//   question sends its text and attaches suggestion_attribution to the next
//   chat request (useEmbedChatSession setSuggestionAttribution + sendMsg).
// - citations: frontend/src/utils/citationMarkdown.ts preprocessCitationTags —
//   <web url title/> becomes an inline link pill (domain label) and
//   <kb doc chunk_id/> becomes an inline pill whose popover loads the chunk
//   content (EmbedBotMessage useEmbedCitationPopover).
import {
  appendHistoryPage,
  followUpMessageId,
  historyCursor,
  parseCitationSegments,
  scrollOffsetAfterPrepend,
  shouldTriggerHistoryLoad,
  suggestionAttributionBody,
  toReadySuggestions,
  mapHistoryMessages,
  type EmbedChatMessage,
} from './chat-data.ts';
import { embedText } from './messages.ts';

// The follow-up surface tests load EmbedEntryPage.tsx (marked + dompurify +
// embed-chat.css). Mirror the wiki test harness: stub .css for node and bind
// jsdom globals before the dynamic import so DOMPurify attaches to a window.
import * as nodeModule from 'node:module';
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

const olderBatch = [
  { id: 'm3', role: 'assistant', content: 'older answer', created_at: '2024-01-01T10:00:00Z' },
  { id: 'm2', role: 'user', content: 'older question', created_at: '2024-01-01T09:59:00Z' },
];
const current: EmbedChatMessage[] = [
  { role: 'user', content: 'new question' },
  { role: 'assistant', content: 'new answer' },
];

test('historyCursor reads the oldest row timestamp like Vue created_at cursor', () => {
  assert.equal(historyCursor(olderBatch), '2024-01-01T10:00:00Z');
  assert.equal(historyCursor([]), '');
  assert.equal(historyCursor([{ created_at: 42 }]), '42');
});

test('appendHistoryPage prepends the older batch and keeps the batch order', () => {
  // Vue: hasMore stays true only while the batch fills the limit (limit=2 here).
  const page = appendHistoryPage(current, olderBatch, '2024-01-02T00:00:00Z', 2);
  assert.equal(page.cursor, '2024-01-01T10:00:00Z');
  assert.equal(page.hasMore, true);
  assert.deepEqual(page.messages.map((m) => m.content), [
    'older answer',
    'older question',
    'new question',
    'new answer',
  ]);
});

test('appendHistoryPage stops on empty batch, short batch and unchanged cursor', () => {
  const empty = appendHistoryPage(current, [], '2024-01-02T00:00:00Z', 20);
  assert.equal(empty.hasMore, false);
  assert.equal(empty.messages.length, current.length, 'empty batch leaves messages untouched');

  const short = appendHistoryPage(current, olderBatch, '2024-01-02T00:00:00Z', 20);
  assert.equal(short.hasMore, false, 'batch shorter than limit is the last page');

  const dup = appendHistoryPage(current, olderBatch, '2024-01-01T10:00:00Z', 20);
  assert.equal(dup.hasMore, false, 'unchanged cursor means the server has nothing newer');
  assert.deepEqual(dup.messages, current, 'unchanged cursor leaves messages untouched');
});

test('shouldTriggerHistoryLoad guards like Vue onChatScrollTop', () => {
  assert.equal(shouldTriggerHistoryLoad(0, false, true), true, 'scrollTop <= 0 with more history loads');
  assert.equal(shouldTriggerHistoryLoad(0, true, true), false, 'a load already in flight');
  assert.equal(shouldTriggerHistoryLoad(0, false, false), false, 'no more history');
  assert.equal(shouldTriggerHistoryLoad(40, false, true), false, 'not at the top yet');
});

test('scrollOffsetAfterPrepend restores the viewport like handleMsgList', () => {
  assert.equal(scrollOffsetAfterPrepend(1200, 2000), 800);
});

test('mapHistoryMessages keeps ids and completion for follow-up loading', () => {
  const mapped = mapHistoryMessages([
    { id: 'a1', role: 'assistant', content: 'done', is_completed: true },
    { role: 'assistant', content: 'no id' },
  ]);
  assert.equal(mapped[0].id, 'a1');
  assert.equal(mapped[0].is_completed, true);
  assert.equal(mapped[1].id, undefined);
});

test('followUpMessageId mirrors Vue message.id || assistant_message_id', () => {
  assert.equal(followUpMessageId({ id: 'x' }), 'x');
  assert.equal(followUpMessageId({ assistant_message_id: 'y' }), 'y');
  assert.equal(followUpMessageId({}), '');
});

test('toReadySuggestions gates on status ready like message.suggestionSet', () => {
  const set = {
    id: 'set-1',
    status: 'ready',
    allow_regenerate: true,
    questions: [
      { id: 'q1', text: ' Follow up? ' },
      { id: 'q2', text: '' },
      { text: 'no id' },
    ],
  };
  const ready = toReadySuggestions(set);
  assert.equal(ready?.id, 'set-1');
  assert.equal(ready?.allowRegenerate, true);
  assert.deepEqual(ready?.questions, [{ id: 'q1', text: 'Follow up?' }]);

  assert.equal(toReadySuggestions({ ...set, status: 'generating' }), null);
  assert.equal(toReadySuggestions(null), null);
  assert.equal(toReadySuggestions(undefined), null);
});

test('suggestionAttributionBody attaches the clicked set to the chat request', () => {
  const body = { query: 'hi' };
  assert.deepEqual(
    suggestionAttributionBody(body, { suggestionSetId: 'set-1', questionId: 'q1' }),
    { query: 'hi', suggestion_attribution: { suggestion_set_id: 'set-1', question_id: 'q1' } },
  );
  assert.deepEqual(suggestionAttributionBody(body, null), body, 'no attribution leaves the body untouched');
  assert.deepEqual(body, { query: 'hi' }, 'input body is not mutated');
});

test('follow-up copy exists in all five locales with Vue embedPublish wording', () => {
  assert.equal(embedText('zh-CN', 'followUpQuestions'), '继续问');
  assert.equal(embedText('zh-CN', 'refreshSuggestedQuestions'), '换一批');
  assert.equal(embedText('en-US', 'followUpQuestions'), 'Keep asking');
  assert.equal(embedText('en-US', 'refreshSuggestedQuestions'), 'More');
  for (const locale of ['ja-JP', 'ko-KR', 'ru-RU'] as const) {
    assert.ok(embedText(locale, 'followUpQuestions').length > 0, `${locale} followUpQuestions`);
    assert.ok(embedText(locale, 'refreshSuggestedQuestions').length > 0, `${locale} refreshSuggestedQuestions`);
  }
});

test('EmbedChatSurface renders a ready follow-up set under its answer', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages: [{
      role: 'assistant',
      content: 'answer',
      suggestions: { id: 'set-1', allowRegenerate: true, questions: [{ id: 'q1', text: 'Why?' }] },
    }],
    suggestedQuestions: [],
    suggestedLoading: false,
    welcomeMessage: '',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  assert.match(html, /Keep asking/);
  assert.match(html, /Why\?/);
  assert.match(html, /More/, 'allow_regenerate renders the Vue refresh action');
});

test('EmbedChatSurface hides dismissed follow-ups', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages: [{
      role: 'assistant',
      content: 'answer',
      suggestions: { id: 'set-1', allowRegenerate: false, questions: [{ id: 'q1', text: 'Why?' }] },
      suggestionsDismissed: true,
    }],
    suggestedQuestions: [],
    suggestedLoading: false,
    welcomeMessage: '',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  assert.doesNotMatch(html, /Why\?/);
});

test('parseCitationSegments splits web and kb tags like preprocessCitationTags', () => {
  const segments = parseCitationSegments(
    'before<web url="https://example.com/a" title="Ex"/><kb doc="Report" chunk_id="c-1"/>after',
    [],
  );
  // Adjacent pills have no text between them, so no text segment is emitted.
  assert.deepEqual(
    segments.map((s) => s.type),
    ['text', 'web', 'kb', 'text'],
  );
  const web = segments[1];
  assert.equal(web.type === 'web' && web.url, 'https://example.com/a');
  assert.equal(web.type === 'web' && web.title, 'Ex');
  assert.equal(web.type === 'web' && web.domain, 'example.com');
  const kb = segments[2];
  assert.equal(kb.type === 'kb' && kb.doc, 'Report');
  assert.equal(kb.type === 'kb' && kb.chunkId, 'c-1');
});

test('parseCitationSegments resolves positional chunk ids against references', () => {
  const refs = [
    { id: 'chunk-a', knowledge_title: 'Alpha' },
    { id: 'chunk-b', knowledge_title: 'Beta' },
  ];
  const byDoc = parseCitationSegments('<kb doc="Beta" chunk_id="123"/>', refs);
  const kb1 = byDoc.find((s) => s.type === 'kb');
  assert.equal(kb1?.type === 'kb' && kb1.chunkId, 'chunk-b', 'doc title match wins over raw ids');
  const byPosition = parseCitationSegments('<kb doc="Doc" chunk_id="2"/>', refs);
  const kb2 = byPosition.find((s) => s.type === 'kb');
  assert.equal(kb2?.type === 'kb' && kb2.chunkId, 'chunk-b', 'numeric ids map by position');
  const passthrough = parseCitationSegments(
    `<kb doc="X" chunk_id="0f0e0d0c-0b0a-0908-0700-060504030201"/>`,
    refs,
  );
  const kb3 = passthrough.find((s) => s.type === 'kb');
  assert.equal(kb3?.type === 'kb' && kb3.chunkId, '0f0e0d0c-0b0a-0908-0700-060504030201', 'uuid chunk ids pass through');
  // Vue drops <kb doc="..."/> tags with no resolvable chunk id entirely.
  const dropped = parseCitationSegments('a<kb doc="Alpha"/>b', refs);
  assert.deepEqual(dropped, [{ type: 'text', text: 'ab' }]);
});

test('EmbedChatSurface renders citation pills inside answers', async () => {
  const { EmbedChatSurface } = await import('./EmbedEntryPage.tsx');
  const { renderToStaticMarkup } = await import('react-dom/server');
  const React = (await import('react')).default;
  const html = renderToStaticMarkup(React.createElement(EmbedChatSurface, {
    messages: [{
      role: 'assistant',
      content: 'see <web url="https://example.com/a" title="Ex"/><kb doc="Report" chunk_id="c-1"/>',
    }],
    suggestedQuestions: [],
    suggestedLoading: false,
    welcomeMessage: '',
    locale: 'en-US',
    onSuggest: () => {},
  }));
  assert.match(html, /example\.com/, 'web pill shows the domain');
  assert.match(html, /Report/, 'kb pill shows the doc name');
  assert.match(html, /data-chunk-id="c-1"/, 'kb pill carries the chunk id for the popover');
});
