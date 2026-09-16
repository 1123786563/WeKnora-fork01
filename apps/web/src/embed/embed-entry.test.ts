import assert from 'node:assert/strict';
import test from 'node:test';

// Vue parity references:
// - handshake protocol: frontend/src/api/embed/index.ts:439-771
//   (provide_token/set_context/set_locale/open_with_query inbound;
//   bootstrap_request/ready/message_sent/message_received outbound)
// - theme injection: frontend/src/views/embed/EmbedPage.vue pageStyle/badgeStyle
//   (channel primary_color -> --embed-primary + color-mix badge)
// - host context prefix: frontend/src/utils/embedContext.ts buildQueryWithHostContext
// - tokens/storage: frontend/src/api/embed/index.ts:59-157
import {
  EMBED_HOST_SOURCE,
  EMBED_MESSAGE_SOURCE,
  embedBadgeStyle,
  embedBootstrapRequestPayload,
  embedChatSessionStorageKey,
  embedMessageReceivedPayload,
  embedMessageSentPayload,
  embedReadyPayload,
  embedThemeVars,
  embedVisitorStorageKey,
  isEmbedSessionToken,
  normalizeEmbedLocale,
  parseEmbedHostMessage,
} from './host-protocol.ts';
import { buildQueryWithHostContext } from './host-context.ts';

test('host protocol source tags match the Vue weknora-embed/weknora-host pair', () => {
  assert.equal(EMBED_MESSAGE_SOURCE, 'weknora-embed');
  assert.equal(EMBED_HOST_SOURCE, 'weknora-host');
});

test('parses the four Vue host message types and rejects foreign payloads', () => {
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'provide_token', token: ' pub_1 ' }), {
    type: 'provide_token',
    token: 'pub_1',
    channelId: undefined,
  });
  assert.deepEqual(
    parseEmbedHostMessage({ source: 'weknora-host', type: 'provide_token', token: 't', channel_id: 'c2' }),
    { type: 'provide_token', token: 't', channelId: 'c2' },
  );
  // provide_token without a token is dropped (Vue onEmbedHostToken L738-747).
  assert.equal(parseEmbedHostMessage({ source: 'weknora-host', type: 'provide_token' }), null);
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'set_context', payload: { user: 'alice' } }), {
    type: 'set_context',
    payload: { user: 'alice' },
  });
  // set_context without payload degrades to an empty merge (payload || {}).
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'set_context' }), {
    type: 'set_context',
    payload: {},
  });
  // set_locale accepts both payload.locale and flat locale (L750-759).
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'set_locale', payload: { locale: 'zh-CN' } }), {
    type: 'set_locale',
    locale: 'zh-CN',
  });
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'set_locale', locale: 'en-US' }), {
    type: 'set_locale',
    locale: 'en-US',
  });
  assert.equal(parseEmbedHostMessage({ source: 'weknora-host', type: 'set_locale' }), null);
  // open_with_query accepts both payload.query and flat query (L762-771).
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'open_with_query', payload: { query: ' hi ' } }), {
    type: 'open_with_query',
    query: 'hi',
  });
  assert.deepEqual(parseEmbedHostMessage({ source: 'weknora-host', type: 'open_with_query', query: 'q' }), {
    type: 'open_with_query',
    query: 'q',
  });
  // Foreign source / unknown type / non-object payloads are ignored.
  assert.equal(parseEmbedHostMessage({ source: 'weknora-embed', type: 'provide_token', token: 't' }), null);
  assert.equal(parseEmbedHostMessage({ source: 'weknora-host', type: 'unknown' }), null);
  assert.equal(parseEmbedHostMessage('bootstrap'), null);
  assert.equal(parseEmbedHostMessage(null), null);
});

test('outbound payload builders mirror the Vue postToParent payloads with sensitive flags', () => {
  assert.deepEqual(embedBootstrapRequestPayload('c1'), { source: 'weknora-embed', type: 'bootstrap_request', channel_id: 'c1' });
  assert.deepEqual(embedReadyPayload('c1'), { source: 'weknora-embed', type: 'ready', channel_id: 'c1' });
  assert.deepEqual(embedMessageSentPayload('c1', 's1', 'hello'), {
    source: 'weknora-embed',
    type: 'message_sent',
    channel_id: 'c1',
    session_id: 's1',
    query: 'hello',
  });
  assert.deepEqual(embedMessageReceivedPayload('c1', 's1', 'answer'), {
    source: 'weknora-embed',
    type: 'message_received',
    channel_id: 'c1',
    session_id: 's1',
    content: 'answer',
  });
});

test('theme injection maps the channel primary color like EmbedPage.vue pageStyle/badgeStyle', () => {
  assert.deepEqual(embedThemeVars(undefined), {});
  assert.deepEqual(embedThemeVars('  '), {});
  assert.deepEqual(embedThemeVars('#2563eb'), { '--embed-primary': '#2563eb' });
  assert.deepEqual(embedBadgeStyle('#2563eb'), {
    background: 'color-mix(in srgb, #2563eb 12%, transparent)',
    color: '#2563eb',
  });
  assert.deepEqual(embedBadgeStyle(undefined), {});
});

test('host locale messages normalize to the five supported locales', () => {
  assert.equal(normalizeEmbedLocale('zh-CN'), 'zh-CN');
  assert.equal(normalizeEmbedLocale('en-US'), 'en-US');
  assert.equal(normalizeEmbedLocale('ja-JP'), 'ja-JP');
  assert.equal(normalizeEmbedLocale('ko-KR'), 'ko-KR');
  assert.equal(normalizeEmbedLocale('ru-RU'), 'ru-RU');
  // BCP-47 primary subtag and case-insensitive fallbacks.
  assert.equal(normalizeEmbedLocale('zh'), 'zh-CN');
  assert.equal(normalizeEmbedLocale('ZH-tw'), 'zh-CN');
  assert.equal(normalizeEmbedLocale('fr-FR'), '');
  assert.equal(normalizeEmbedLocale(''), '');
  assert.equal(normalizeEmbedLocale('  '), '');
});

test('embed token/storage helpers mirror the Vue api/embed constants', () => {
  assert.equal(isEmbedSessionToken('ems_abc'), true);
  assert.equal(isEmbedSessionToken('pub_abc'), false);
  assert.equal(embedVisitorStorageKey('c1'), 'weknora-embed-visitor:c1');
  assert.equal(embedChatSessionStorageKey('c1'), 'weknora-embed-session:c1');
});

test('host context prefixes the user query like frontend/src/utils/embedContext.ts', () => {
  assert.equal(buildQueryWithHostContext('q', undefined), 'q');
  assert.equal(buildQueryWithHostContext('q', {}), 'q');
  assert.equal(
    buildQueryWithHostContext('q', { page: '/docs', role: null, empty: '', nested: { a: 1 } }),
    '[Host context]\npage: /docs\nnested: {"a":1}\n\nq',
  );
});
