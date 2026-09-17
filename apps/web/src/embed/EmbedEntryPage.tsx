import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { createEmbedClient, type EmbedClient, type HttpTransport } from '@weknora/api-client';
import { ChatComposer } from '@weknora/views/chat/composer';
import { isLocale, type Locale } from '@weknora/i18n/runtime';
import { createEmbedBridge, mapEmbedError, parseEmbedToken } from './bridge.ts';
import {
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
import { embedText, type EmbedTextKey } from './messages.ts';
import {
  appendHistoryPage,
  extractStreamReferences,
  isWebSearchReference,
  mapHistoryMessages,
  normalizeSuggestedQuestions,
  referenceContent,
  referenceHeadline,
  referenceTitle,
  referenceUrl,
  scrollOffsetAfterPrepend,
  shouldTriggerHistoryLoad,
  suggestionAttributionBody,
  toReadySuggestions,
  type EmbedChatMessage,
  type EmbedReference,
  type EmbedReadySuggestions,
  type EmbedSuggestionAttribution,
  type EmbedSuggestionItem,
} from './chat-data.ts';
import { renderEmbedChatMarkdown } from './markdown.ts';
import './embed-chat.css';
// Vue EmbedBotMessage.vue imports katex/dist/katex.min.css for the answer face.
import 'katex/dist/katex.min.css';

// Vue parity for the isolated embed entry:
// - entry: frontend/embed.html + frontend/src/embed-main.ts (separate document,
//   light theme forced, full-height canvas). React keeps a single Vite entry, so
//   apps/web/index.html applies the same light-theme bootstrap for /embed/* and
//   main.tsx mounts this page instead of the platform shell.
// - handshake: frontend/src/composables/useEmbedBridge.ts — token from URL, else
//   bootstrap_request to the host, provide_token back, ready after bootstrap.
// - theme: frontend/src/views/embed/EmbedPage.vue pageStyle/badgeStyle
//   (channel primary_color -> --embed-primary).
// - size: the page fills the iframe viewport (100vh, overflow hidden) — the
//   host snippet/widget owns the frame size, the embed never resizes it.

export interface EmbedEntryPageProps {
  /** Channel id; defaults to /embed/:channelId from the pathname. */
  channelId?: string;
  apiBaseUrl?: string;
  /** Host window when embedded in an iframe; production default is window.parent. */
  parentWindow?: unknown;
  referrer?: string;
  /** Test seam: preconfigured embed client. */
  embedClient?: EmbedClient;
  /** Test seam: message target override. */
  postMessage?: (target: unknown, payload: Record<string, unknown>, targetOrigin: string) => void;
}

export function embedChannelIdFromPath(pathname: string): string {
  const match = /\/embed\/([^/?#]+)/.exec(pathname);
  return match ? decodeURIComponent(match[1]) : '';
}

function readEmbedLocaleFromUrl(search: string, hash: string): Locale | '' {
  const hashPart = hash.startsWith('#') ? hash.slice(1) : hash;
  const fromQuery = new URLSearchParams(search).get('locale') || '';
  const raw = fromQuery || new URLSearchParams(hashPart).get('locale') || '';
  return normalizeEmbedLocale(raw);
}

function createFetchHttpTransport(): HttpTransport {
  return {
    async send(request) {
      const response = await fetch(request.url, {
        method: request.method,
        headers: request.headers,
        body: request.body === undefined ? undefined : JSON.stringify(request.body),
        signal: request.signal,
      });
      const text = await response.text();
      let body: unknown = text;
      try { body = JSON.parse(text); } catch { /* SSE / non-JSON bodies stay text */ }
      const headers: Record<string, string> = {};
      response.headers.forEach((value, key) => { headers[key] = value; });
      return { status: response.status, headers, body };
    },
  };
}

function getOrCreateVisitorId(channelId: string): string {
  try {
    const key = embedVisitorStorageKey(channelId);
    const existing = window.localStorage.getItem(key)?.trim();
    if (existing) return existing;
    const id = typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `v-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    window.localStorage.setItem(key, id);
    return id;
  } catch {
    return `v-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
}

interface StoredSession { id: string; sig: string }

function readStoredSession(channelId: string): StoredSession | null {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(embedChatSessionStorageKey(channelId)) || 'null') as StoredSession | null;
    return parsed && typeof parsed.id === 'string' && typeof parsed.sig === 'string' && parsed.id ? parsed : null;
  } catch {
    return null;
  }
}

function writeStoredSession(channelId: string, session: StoredSession | null): void {
  try {
    const key = embedChatSessionStorageKey(channelId);
    if (session) window.localStorage.setItem(key, JSON.stringify(session));
    else window.localStorage.removeItem(key);
  } catch { /* private mode: persistence is best-effort */ }
}

type EmbedConfig = {
  channel_id?: string;
  display_title?: string;
  page_title?: string;
  name?: string;
  agent_name?: string;
  agent_avatar?: string;
  welcome_message?: string;
  primary_color?: string;
  default_locale?: string;
  show_suggested_questions?: boolean;
} & Record<string, unknown>;

export function EmbedEntryPage(props: EmbedEntryPageProps = {}) {
  const channelId = props.channelId ?? embedChannelIdFromPath(window.location.pathname);
  const inIframe = typeof window === 'undefined' ? false : window.parent !== window;
  const parentWindow = props.parentWindow !== undefined ? props.parentWindow : inIframe ? window.parent : null;
  const postMessage = props.postMessage ?? ((target: unknown, payload: Record<string, unknown>, targetOrigin: string) => {
    (target as Window).postMessage(payload, targetOrigin);
  });

  const [locale, setLocale] = useState<Locale>(() => {
    const fromUrl = readEmbedLocaleFromUrl(window.location.search, window.location.hash);
    if (fromUrl) return fromUrl;
    const nav = navigator.language;
    return isLocale(nav) ? nav : 'en-US';
  });
  // Vue hostLocalePinned: an explicit host locale wins over channel default.
  const localePinnedRef = useRef(Boolean(readEmbedLocaleFromUrl(window.location.search, window.location.hash)));
  const [phase, setPhase] = useState<'bootstrapping' | 'awaiting' | 'ready' | 'error'>('bootstrapping');
  const [errorKey, setErrorKey] = useState<EmbedTextKey>('loadError');
  const [config, setConfig] = useState<EmbedConfig | null>(null);
  const [session, setSession] = useState<StoredSession | null>(null);
  const [apiToken, setApiToken] = useState('');
  const [hostContext, setHostContext] = useState<Record<string, unknown>>({});
  const [messages, setMessages] = useState<EmbedChatMessage[]>([]);
  const [suggestedQuestions, setSuggestedQuestions] = useState<string[]>([]);
  const [suggestedLoading, setSuggestedLoading] = useState(false);
  const [draft, setDraft] = useState('');
  const [streaming, setStreaming] = useState(false);
  // Scroll-paged history (useEmbedChatSession.ts limit=20 / created_at cursor).
  const [historyLoadingOlder, setHistoryLoadingOlder] = useState(false);
  const historyCursorRef = useRef('');
  const hasMoreHistoryRef = useRef(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(false);
  const messagesRef = useRef<EmbedChatMessage[]>([]);
  messagesRef.current = messages;
  const sessionRef = useRef<StoredSession | null>(null);
  sessionRef.current = session;
  const apiTokenRef = useRef('');
  apiTokenRef.current = apiToken;
  const suggestionAttributionRef = useRef<EmbedSuggestionAttribution | null>(null);
  const followUpsEnabledRef = useRef(false);

  const bridge = useMemo(
    () => createEmbedBridge({ parentWindow: (parentWindow ?? {}) as object, referrer: props.referrer ?? (typeof document !== 'undefined' ? document.referrer : undefined) }),
    [parentWindow, props.referrer],
  );
  const client = useMemo(
    () => props.embedClient ?? createEmbedClient({ baseURL: props.apiBaseUrl || window.location.origin, transport: createFetchHttpTransport() }),
    [props.embedClient, props.apiBaseUrl],
  );

  const postToHost = useCallback((payload: Record<string, unknown>, options: { sensitive?: boolean } = {}): boolean => {
    if (!parentWindow) return false;
    return bridge.post(payload, (p, targetOrigin) => postMessage(parentWindow, p, targetOrigin), options);
  }, [bridge, parentWindow, postMessage]);

  // --- per-message follow-up suggestions (EmbedChatCore.vue
  // loadFollowUpSuggestions/loadPersistedFollowUps) -------------------------
  const patchMessageByRowId = useCallback((rowId: string | undefined, patch: Partial<EmbedChatMessage>) => {
    if (!rowId) return;
    setMessages((prev) => prev.map((m) => (m.id === rowId ? { ...m, ...patch } : m)));
  }, []);

  const loadFollowUpsFor = useCallback(async (messageId: string, opts: { ensure?: boolean; regenerate?: boolean } = {}) => {
    const sess = sessionRef.current;
    const token = apiTokenRef.current;
    if (!followUpsEnabledRef.current || !sess || !token || !messageId) return;
    const visitorId = getOrCreateVisitorId(channelId);
    patchMessageByRowId(messageId, { suggestions: null });
    try {
      let raw = opts.ensure || opts.regenerate
        ? await client.embed.public.ensureMessageSuggestions(channelId, token, sess.id, messageId, sess.sig, visitorId, opts.regenerate === true)
        : await client.embed.public.messageSuggestions(channelId, token, sess.id, messageId, sess.sig, visitorId);
      // Vue polls once per second while the set is still generating (max 120s).
      for (let attempt = 0; raw.status === 'generating' && attempt < 120; attempt++) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        if (sessionRef.current?.id !== sess.id || !messagesRef.current.some((m) => m.id === messageId)) return;
        raw = await client.embed.public.messageSuggestions(channelId, token, sess.id, messageId, sess.sig, visitorId);
      }
      patchMessageByRowId(messageId, { suggestions: toReadySuggestions(raw) });
    } catch {
      patchMessageByRowId(messageId, { suggestions: null });
    }
  }, [channelId, client, patchMessageByRowId]);

  const loadPersistedFollowUps = useCallback((rows: EmbedChatMessage[]) => {
    for (const row of rows) {
      // Vue loadPersistedFollowUps: completed answers only, never re-requested
      // for rows already carrying a set (suggestionSet === undefined guard).
      if (row.role === 'assistant' && row.is_completed && row.id && row.suggestions === undefined) {
        void loadFollowUpsFor(row.id);
      }
    }
  }, [loadFollowUpsFor]);

  // --- scroll-paged history (useEmbedChatSession.ts onChatScrollTop) --------
  const loadOlderHistory = useCallback(async () => {
    const sess = sessionRef.current;
    const token = apiTokenRef.current;
    if (!sess || !token || !hasMoreHistoryRef.current) return;
    if (historyLoadingOlder) return;
    setHistoryLoadingOlder(true);
    try {
      const rows = await client.embed.public.messages(channelId, token, sess.id, {
        limit: 20,
        beforeTime: historyCursorRef.current || undefined,
        signature: sess.sig,
        visitorId: getOrCreateVisitorId(channelId),
      });
      const page = appendHistoryPage(messagesRef.current, rows, historyCursorRef.current, 20);
      historyCursorRef.current = page.cursor;
      hasMoreHistoryRef.current = page.hasMore;
      setMessages(page.messages);
      setHasMoreHistory(page.hasMore);
      loadPersistedFollowUps(page.messages.filter((m) => !messagesRef.current.includes(m)));
    } catch {
      // Vue marks pagination done when the fetch fails.
      hasMoreHistoryRef.current = false;
      setHasMoreHistory(false);
    } finally {
      setHistoryLoadingOlder(false);
    }
  }, [channelId, client, historyLoadingOlder, loadPersistedFollowUps]);

  const bootstrap = useCallback(async (embedToken: string) => {
    if (!channelId || !embedToken) return;
    setPhase('bootstrapping');
    try {
      // Secure mode (useEmbedBridge L116-138): exchange publish tokens for a
      // short-lived session token; production fails closed instead of falling
      // back to the long-lived publish token.
      let apiToken = embedToken;
      if (!isEmbedSessionToken(embedToken)) {
        const exchange = await client.embed.public.exchange(channelId, embedToken);
        if (!exchange.sessionToken) throw new Error('embed session exchange returned no token');
        apiToken = exchange.sessionToken;
      }

      const cfg = await client.embed.public.config(channelId, apiToken) as EmbedConfig;
      setConfig(cfg);
      followUpsEnabledRef.current = cfg.show_suggested_questions === true;
      if (!localePinnedRef.current) {
        const channelLocale = normalizeEmbedLocale(String(cfg.default_locale ?? ''));
        if (channelLocale) setLocale(channelLocale);
      }

      // Resume the persisted session when still valid; otherwise create one
      // (useEmbedBridge L151-172). On resume the stored rows are backfilled
      // into the face exactly like the Vue resetAndLoad -> getmsgList pass
      // (useEmbedChatSession.ts, limit 20).
      const stored = readStoredSession(channelId);
      const visitorId = getOrCreateVisitorId(channelId);
      let resolved: StoredSession | null = null;
      if (stored) {
        try {
          const history = await client.embed.public.messages(channelId, apiToken, stored.id, { limit: 20, signature: stored.sig, visitorId });
          resolved = stored;
          const page = appendHistoryPage([], history, '', 20);
          setMessages(page.messages);
          historyCursorRef.current = page.cursor;
          hasMoreHistoryRef.current = page.hasMore;
          setHasMoreHistory(page.hasMore);
          loadPersistedFollowUps(page.messages);
        } catch { /* stale/expired: create a fresh signed session */ }
      }
      if (!resolved) {
        const created = await client.embed.public.createSession(channelId, apiToken);
        resolved = { id: created.id, sig: created.signature };
        // Vue also calls getmsgList for a brand-new session and gets an empty
        // batch (no render change), so skipping the fetch here is equivalent;
        // it also closes pagination like the Vue empty-batch branch.
        historyCursorRef.current = '';
        hasMoreHistoryRef.current = false;
        setHasMoreHistory(false);
      }
      writeStoredSession(channelId, resolved);
      setSession(resolved);
      setApiToken(apiToken);
      sessionRef.current = resolved;
      apiTokenRef.current = apiToken;

      // Channel-level suggested questions (EmbedChatCore.vue
      // fetchSuggestedQuestions): only when the channel enables them.
      if (cfg.show_suggested_questions === true) {
        setSuggestedLoading(true);
        try {
          setSuggestedQuestions(normalizeSuggestedQuestions(await client.embed.public.suggestedQuestions(channelId, apiToken)));
        } catch { setSuggestedQuestions([]); } finally { setSuggestedLoading(false); }
      } else {
        setSuggestedQuestions([]);
      }

      setPhase('ready');
      postToHost(embedReadyPayload(channelId));
    } catch (error) {
      const mapped = mapEmbedError(error);
      setErrorKey(mapped.kind === 'disabled' ? 'channelDisabled' : mapped.kind === 'session' ? 'sessionFailed' : mapped.kind === 'exchange' ? 'invalidChannel' : 'loadError');
      setPhase('error');
    }
  }, [channelId, client, postToHost, loadPersistedFollowUps]);

  useEffect(() => {
    if (!channelId) {
      setErrorKey('missingChannel');
      setPhase('error');
      return;
    }
    const initialToken = parseEmbedToken({ search: window.location.search, hash: window.location.hash });
    if (initialToken) {
      void bootstrap(initialToken);
      return;
    }
    if (!parentWindow) {
      // Top-level navigation without a token cannot bootstrap (Vue L236-242).
      setErrorKey('missingChannel');
      setPhase('error');
      return;
    }
    setPhase('awaiting');
    postToHost(embedBootstrapRequestPayload(channelId));
    const onMessage = (event: MessageEvent) => {
      if (!bridge.accept(event)) return;
      const parsed = parseEmbedHostMessage(event.data);
      if (!parsed) return;
      if (parsed.type === 'provide_token') {
        if (parsed.channelId && parsed.channelId !== channelId) return;
        void bootstrap(parsed.token);
      } else if (parsed.type === 'set_context') {
        setHostContext((prev) => ({ ...prev, ...parsed.payload }));
      } else if (parsed.type === 'set_locale') {
        const next = normalizeEmbedLocale(parsed.locale);
        if (next) {
          localePinnedRef.current = true;
          setLocale(next);
        }
      } else if (parsed.type === 'open_with_query') {
        setDraft(parsed.query);
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [channelId]);

  const headerTitle = useMemo(() => {
    const cfg = config;
    if (!cfg) return embedText(locale, 'defaultChatTitle');
    return String(cfg.display_title || cfg.page_title || cfg.name || cfg.agent_name || '').trim() || embedText(locale, 'defaultChatTitle');
  }, [config, locale]);

  const themeVars = embedThemeVars(typeof config?.primary_color === 'string' ? config.primary_color : undefined);
  const badge = embedBadgeStyle(typeof config?.primary_color === 'string' ? config.primary_color : undefined);

  const submit = useCallback(async (submission: { query?: unknown }) => {
    const raw = String(submission.query ?? '').trim();
    if (!raw || !session || streaming) return;
    const query = buildQueryWithHostContext(raw, hostContext);
    setMessages((prev) => [...prev, { role: 'user', content: raw }]);
    setDraft('');
    postToHost(embedMessageSentPayload(channelId, session.id, raw), { sensitive: true });
    setStreaming(true);
    let answer = '';
    let assistantMessageId = '';
    setMessages((prev) => [...prev, { role: 'assistant', content: '' }]);
    // Vue setSuggestionAttribution: the set/question clicked in the follow-up
    // card rides on the very next chat request, then the pending value clears.
    const attribution = suggestionAttributionRef.current;
    suggestionAttributionRef.current = null;
    try {
      await client.embed.public.chat({
        channelId,
        token: apiToken,
        sessionId: session.id,
        signature: session.sig,
        visitorId: getOrCreateVisitorId(channelId),
        body: suggestionAttributionBody({ query }, attribution),
      }, (event) => {
        // Vue useChatStreamHandler keeps data.assistant_message_id as the id of
        // the live answer row; follow-ups need it once the turn completes.
        const eventId = typeof event.assistant_message_id === 'string' ? event.assistant_message_id : '';
        if (eventId) assistantMessageId = eventId;
        const text = typeof event.content === 'string' ? event.content : '';
        if (text) {
          answer += text;
          setMessages((prev) => {
            const next = [...prev];
            const last = next.length > 0 ? next[next.length - 1] : undefined;
            next[next.length - 1] = { role: 'assistant', content: answer, ...(last?.references ? { references: last.references } : {}), ...(assistantMessageId ? { id: assistantMessageId } : {}) };
            return next;
          });
        }
        // References ride the same SSE events (useChatStreamHandler
        // extractKnowledgeReferences) and are attached to the live answer row.
        const refs = extractStreamReferences(event);
        if (refs.length) {
          setMessages((prev) => {
            const next = [...prev];
            const last = next.length > 0 ? next[next.length - 1] : undefined;
            if (!last || last.role !== 'assistant') return prev;
            next[next.length - 1] = { ...last, references: refs };
            return next;
          });
        }
      });
      if (answer) postToHost(embedMessageReceivedPayload(channelId, session.id, answer), { sensitive: true });
      // Vue onTurnComplete -> loadFollowUpSuggestions(message, ensure=true).
      if (assistantMessageId) void loadFollowUpsFor(assistantMessageId, { ensure: true });
    } catch {
      setMessages((prev) => {
        const next = [...prev];
        next[next.length - 1] = { role: 'assistant', content: answer || embedText(locale, 'loadError') };
        return next;
      });
    } finally {
      setStreaming(false);
    }
  }, [apiToken, channelId, client, hostContext, locale, postToHost, session, streaming, loadFollowUpsFor]);

  const startNewChat = useCallback(() => {
    if (!session || !messages.length) return;
    void (async () => {
      try {
        const created = await client.embed.public.createSession(channelId, apiToken);
        const next = { id: created.id, sig: created.signature };
        writeStoredSession(channelId, next);
        setSession(next);
        sessionRef.current = next;
        setMessages([]);
        messagesRef.current = [];
        historyCursorRef.current = '';
        hasMoreHistoryRef.current = false;
        setHasMoreHistory(false);
      } catch { /* keep the current session when creation fails (Vue L192-208) */ }
    })();
  }, [apiToken, channelId, client, messages.length, session]);

  if (phase === 'error') return <div className="flex h-screen items-center justify-center p-6 text-center text-[var(--text-secondary, #6b7280)]">{embedText(locale, errorKey)}</div>;
  if (phase !== 'ready' || !session) return <div className="flex h-screen items-center justify-center p-6 text-center text-[var(--text-secondary, #6b7280)]">{embedText(locale, phase === 'awaiting' ? 'awaitingToken' : 'loading')}</div>;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-white" style={themeVars as CSSProperties}>
      <header className="flex shrink-0 items-center gap-3 border-b border-[#eef1f5] px-4 py-3">
        <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] text-[18px]" style={badge as CSSProperties}>
          {typeof config?.agent_avatar === 'string' && config.agent_avatar ? config.agent_avatar : '💬'}
        </span>
        <div className="min-w-0 flex-1">
          <h1 className="m-0 truncate text-[15px] font-semibold leading-tight text-[#1f2329]">{headerTitle}</h1>
        </div>
        <button
          type="button"
          className="shrink-0 cursor-pointer border-0 bg-transparent p-1 text-[#6b7280] hover:text-[color:var(--embed-primary,#2563eb)] disabled:cursor-not-allowed disabled:opacity-40"
          disabled={!messages.length}
          title={embedText(locale, 'newChat')}
          aria-label={embedText(locale, 'newChat')}
          onClick={startNewChat}
        >+</button>
      </header>
      <EmbedChatSurface
        messages={messages}
        suggestedQuestions={suggestedQuestions}
        suggestedLoading={suggestedLoading}
        welcomeMessage={typeof config?.welcome_message === 'string' ? config.welcome_message : ''}
        locale={locale}
        onSuggest={(question) => void submit({ query: question })}
        loadingOlder={historyLoadingOlder}
        hasMoreHistory={hasMoreHistory}
        onReachTop={() => void loadOlderHistory()}
        fetchChunk={async (chunkId) => {
          const data = await client.embed.public.chunk(channelId, apiToken, chunkId) as Record<string, unknown>;
          return typeof data.content === 'string' ? data.content : '';
        }}
        onFollowUpSelect={(entry, set, question) => {
          // Vue handleFollowUpSelect: attach attribution, then send the text;
          // the completed turn re-ensures its own follow-up set in submit().
          suggestionAttributionRef.current = { suggestionSetId: set.id, questionId: question.id };
          void submit({ query: question.text });
        }}
        onDismissFollowUps={(entry) => patchMessageByRowId(entry.id, { suggestionsDismissed: true })}
        onRegenerateFollowUps={(entry) => entry.id && void loadFollowUpsFor(entry.id, { regenerate: true })}
      />
      <div className="shrink-0 px-3 pb-3">
        <EmbedComposer
          draft={draft}
          disabled={streaming}
          placeholder={embedText(locale, 'inputPlaceholder')}
          sendLabel={embedText(locale, 'send')}
          onDraftChange={setDraft}
          onSubmit={submit}
        />
      </div>
    </div>
  );
}

// The message list half of EmbedChatCore.vue: welcome bubble (hidden once the
// visitor speaks), channel suggested-question cards (click sends the question),
// history/live messages, per-answer follow-up suggestion cards, inline citation
// pills and the docInfo.vue-style references block.
export function EmbedChatSurface(props: {
  messages: EmbedChatMessage[];
  suggestedQuestions: string[];
  suggestedLoading: boolean;
  welcomeMessage: string;
  locale: Locale;
  onSuggest: (question: string) => void;
  /** Scroll-paged history (useEmbedChatSession onChatScrollTop, debounced 500ms). */
  loadingOlder?: boolean;
  hasMoreHistory?: boolean;
  onReachTop?: () => void;
  /** Embed chunk loader backing the kb citation pill popover. */
  fetchChunk?: (chunkId: string) => Promise<string>;
  onFollowUpSelect?: (entry: EmbedChatMessage, set: EmbedReadySuggestions, question: EmbedSuggestionItem) => void;
  onDismissFollowUps?: (entry: EmbedChatMessage, set: EmbedReadySuggestions) => void;
  onRegenerateFollowUps?: (entry: EmbedChatMessage, set: EmbedReadySuggestions) => void;
}) {
  const hasUserMessage = props.messages.some((entry) => entry.role === 'user');
  const welcome = props.welcomeMessage.trim();
  const showSuggested = !hasUserMessage && (props.suggestedLoading || props.suggestedQuestions.length > 0);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const reachTopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Scroll restore for prepended history (handleMsgList: scrollTop =
  // newScrollHeight - oldScrollHeight).
  const prevHeightRef = useRef(0);
  const wasLoadingOlderRef = useRef(false);
  const prevCountRef = useRef(props.messages.length);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const newHeight = el.scrollHeight;
    if (wasLoadingOlderRef.current && !props.loadingOlder && prevHeightRef.current > 0 && props.messages.length > prevCountRef.current && el.scrollTop <= 0) {
      el.scrollTop = scrollOffsetAfterPrepend(prevHeightRef.current, newHeight);
    }
    wasLoadingOlderRef.current = Boolean(props.loadingOlder);
    prevHeightRef.current = newHeight;
    prevCountRef.current = props.messages.length;
  }, [props.messages, props.loadingOlder]);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el || !props.onReachTop || reachTopTimerRef.current) return;
    // Vue debounces the top check by 500ms and guards in-flight/done states.
    if (!shouldTriggerHistoryLoad(el.scrollTop, Boolean(props.loadingOlder), props.hasMoreHistory === true)) return;
    reachTopTimerRef.current = setTimeout(() => {
      reachTopTimerRef.current = null;
      props.onReachTop?.();
    }, 500);
  };
  useEffect(() => () => { if (reachTopTimerRef.current) clearTimeout(reachTopTimerRef.current); }, []);

  // Citation pill popover (EmbedBotMessage useEmbedCitationPopover openKb).
  const [citationFloat, setCitationFloat] = useState<{ doc: string; chunkId: string; top: number; left: number; content: string; error: string } | null>(null);
  const chunkCacheRef = useRef(new Map<string, { content: string; error?: string }>());
  const openCitation = (doc: string, chunkId: string, el: HTMLElement) => {
    const rect = el.getBoundingClientRect();
    setCitationFloat({
      doc,
      chunkId,
      top: rect.bottom + window.scrollY + 6,
      left: Math.min(rect.left + window.scrollX, window.innerWidth - 320),
      content: '',
      error: '',
    });
    const scope = chunkCacheRef.current;
    const cached = scope.get(chunkId);
    if (cached) {
      setCitationFloat({ doc, chunkId, top: rect.bottom + 6, left: Math.min(rect.left, window.innerWidth - 320), content: cached.content, error: cached.error || '' });
      return;
    }
    if (!props.fetchChunk) {
      scope.set(chunkId, { content: '', error: 'Failed to load' });
      setCitationFloat({ doc, chunkId, top: rect.bottom + 6, left: Math.min(rect.left, window.innerWidth - 320), content: '', error: 'Failed to load' });
      return;
    }
    void (async () => {
      try {
        const content = String(await props.fetchChunk?.(chunkId) || '').trim();
        scope.set(chunkId, { content });
        setCitationFloat((cur) => (cur && cur.chunkId === chunkId ? { ...cur, content } : cur));
      } catch {
        scope.set(chunkId, { content: '', error: 'Failed to load' });
        setCitationFloat((cur) => (cur && cur.chunkId === chunkId ? { ...cur, error: 'Failed to load' } : cur));
      }
    })();
  };

  return (
    <div ref={scrollRef} className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4" onScroll={onScroll}>
      {welcome && !hasUserMessage ? <div className="self-start max-w-[85%] rounded-[12px] bg-[#f5f7fa] px-3 py-2 text-[14px] text-[#1f2329]">{welcome}</div> : null}
      {props.loadingOlder ? <div className="embed-history-loading self-center text-[12px] text-[#6b7280]">…</div> : null}
      {showSuggested ? (
        <div className="embed-suggested flex flex-col gap-2" aria-busy={props.suggestedLoading}>
          {props.suggestedQuestions.length > 0 ? <p className="m-0 text-[13px] font-medium text-[#6b7280]">{embedText(props.locale, 'suggestedQuestions')}</p> : null}
          {props.suggestedLoading && props.suggestedQuestions.length === 0
            ? [0, 1, 2, 3].map((n) => <div key={n} className="h-10 animate-pulse rounded-[10px] bg-[#f0f0f0]" />)
            : props.suggestedQuestions.map((question) => (
              <button
                key={question}
                type="button"
                className="block w-full cursor-pointer rounded-[10px] border border-[#eef1f5] bg-white px-3 py-2.5 text-left text-[13px] leading-snug text-[#1f2329] hover:border-[#d8dde5]"
                onClick={() => props.onSuggest(question)}
              >{question}</button>
            ))}
        </div>
      ) : null}
      {props.messages.map((entry, index) => entry.role === 'user' ? (
        <div key={index} className="self-end max-w-[85%] rounded-[12px] px-3 py-2 text-[14px] text-white" style={{ background: 'var(--embed-primary, #2563eb)' }}>{entry.content}</div>
      ) : (
        <div key={index} className="embed-answer-row self-start flex max-w-[85%] flex-col gap-2">
          <div className="rounded-[12px] bg-[#f5f7fa] px-3 py-2 text-[14px] text-[#1f2329]">
            <EmbedMessageContent
              content={entry.content}
              references={entry.references}
              onCitation={openCitation}
            />
            {entry.references && entry.references.length > 0 ? <EmbedReferences references={entry.references} locale={props.locale} /> : null}
          </div>
          {entry.suggestions && !entry.suggestionsDismissed && entry.suggestions.questions.length > 0 ? (
            <EmbedFollowUps
              entry={entry}
              set={entry.suggestions}
              locale={props.locale}
              onSelect={props.onFollowUpSelect}
              onDismiss={props.onDismissFollowUps}
              onRegenerate={props.onRegenerateFollowUps}
            />
          ) : null}
        </div>
      ))}
      {citationFloat ? (
        <div
          className="embed-citation-float fixed z-50 max-w-[300px] rounded-[8px] border border-[#e7eaef] bg-white px-3 py-2 text-[12px] text-[#1f2329] shadow-lg"
          style={{ top: citationFloat.top, left: citationFloat.left }}
          onMouseLeave={() => setCitationFloat(null)}
        >
          <div className="font-medium">{citationFloat.doc}</div>
          {citationFloat.error ? <div className="text-[#b91c1c]">{citationFloat.error}</div> : citationFloat.content
            ? <div className="mt-1 max-h-48 overflow-y-auto whitespace-pre-wrap break-words text-[#4b5563]">{citationFloat.content}</div>
            : <div className="text-[#6b7280]">…</div>}
        </div>
      ) : null}
    </div>
  );
}

// FollowUpSuggestions.vue visible behavior: title row with optional refresh
// (allow_regenerate) and dismiss controls, then one button per question.
function EmbedFollowUps(props: {
  entry: EmbedChatMessage;
  set: EmbedReadySuggestions;
  locale: Locale;
  onSelect?: (entry: EmbedChatMessage, set: EmbedReadySuggestions, question: EmbedSuggestionItem) => void;
  onDismiss?: (entry: EmbedChatMessage, set: EmbedReadySuggestions) => void;
  onRegenerate?: (entry: EmbedChatMessage, set: EmbedReadySuggestions) => void;
}) {
  return (
    <div className="embed-followups w-full rounded-[12px] border border-[#eef1f5] bg-[#f8fafc] p-3" aria-live="polite">
      <div className="mb-2 flex items-center justify-between text-[13px] font-semibold text-[#6b7280]">
        <span>{embedText(props.locale, 'followUpQuestions')}</span>
        <span className="flex gap-1">
          {props.set.allowRegenerate ? (
            <button
              type="button"
              className="cursor-pointer rounded-[6px] border-0 bg-transparent px-2 py-1 text-[12px] text-[#6b7280] hover:text-[color:var(--embed-primary,#2563eb)]"
              onClick={() => props.onRegenerate?.(props.entry, props.set)}
            >{embedText(props.locale, 'refreshSuggestedQuestions')}</button>
          ) : null}
          <button
            type="button"
            aria-label={embedText(props.locale, 'close')}
            className="cursor-pointer rounded-[6px] border-0 bg-transparent px-2 py-1 text-[12px] text-[#6b7280] hover:text-[color:var(--embed-primary,#2563eb)]"
            onClick={() => props.onDismiss?.(props.entry, props.set)}
          >✕</button>
        </span>
      </div>
      <div className="flex flex-col gap-1.5">
        {props.set.questions.map((question) => (
          <button
            key={question.id || question.text}
            type="button"
            className="flex w-full cursor-pointer items-center justify-between rounded-[8px] border border-[#eef1f5] bg-white px-3 py-2 text-left text-[13px] leading-snug text-[#1f2329] hover:border-[#d8dde5]"
            onClick={() => props.onSelect?.(props.entry, props.set, question)}
          >
            <span>{question.text}</span>
            <span aria-hidden="true" className="text-[#9ca3af]">↗</span>
          </button>
        ))}
      </div>
    </div>
  );
}

// Embed bot answer rendering (EmbedBotMessage.vue): the full answer goes
// through the Vue markdown pipeline — citations are converted to pill HTML
// BEFORE marked and re-injected AFTER it, then the whole HTML is sanitized.
// The React face renders that sanitized HTML and resolves kb-pill clicks by
// delegation, the same way useEmbedCitationPopover binds `.citation-kb`.
function EmbedMessageContent(props: {
  content: string;
  references?: EmbedReference[];
  onCitation: (doc: string, chunkId: string, el: HTMLElement) => void;
}) {
  const html = useMemo(
    () => renderEmbedChatMarkdown(props.content, (props.references ?? []) as unknown as Record<string, unknown>[]),
    [props.content, props.references],
  );
  if (!html) return null;
  return (
    <div
      className="embed-chat-markdown"
      onClick={(event) => {
        const target = event.target as Element | null;
        const pill = target?.closest?.('.citation-kb');
        if (pill) {
          props.onCitation(pill.getAttribute('data-doc') || '', pill.getAttribute('data-chunk-id') || '', pill as HTMLElement);
        }
      }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}

function truncateReferenceContent(content: string, limit = 80): string {
  return content.length > limit ? `${content.slice(0, limit)}...` : content;
}

// Minimal docInfo.vue parity: collapsible block with the Vue headline copy,
// web_search rows as external links, chunk rows expanding to the stored chunk
// content on click (Vue uses a popup fed by the same reference data).
function EmbedReferences(props: { references: EmbedReference[]; locale: Locale }) {
  return (
    <details className="embed-refs mt-2 border-t border-[#e7eaef] pt-2 text-[12px]">
      <summary className="cursor-pointer text-[#6b7280]">{referenceHeadline(props.references, props.locale)}</summary>
      <div className="mt-1.5 flex flex-col gap-1.5">
        {props.references.map((ref, index) => isWebSearchReference(ref) ? (
          <a
            key={index}
            className="break-all text-[color:var(--embed-primary,#2563eb)] underline-offset-2 hover:underline"
            href={referenceUrl(ref)}
            target="_blank"
            rel="noopener noreferrer"
          >{referenceTitle(ref)}</a>
        ) : (
          <details key={index} className="embed-ref-chunk">
            <summary className="cursor-pointer break-all text-[#1f2329]">{index + 1}. {truncateReferenceContent(referenceContent(ref))}</summary>
            <div className="mt-1 max-h-48 overflow-y-auto whitespace-pre-wrap break-words text-[#4b5563]">{referenceContent(ref)}</div>
          </details>
        ))}
      </div>
    </details>
  );
}

// Thin adapter so the embed face reuses the platform chat composer
// (packages/views/src/chat/composer.tsx) without its agent/model chrome.
function EmbedComposer(props: {
  draft: string;
  disabled: boolean;
  placeholder: string;
  sendLabel: string;
  onDraftChange: (value: string) => void;
  onSubmit: (submission: { query?: unknown }) => void;
}) {
  return (
    <div className="embed-composer" aria-label={props.sendLabel}>
      <ChatComposer
        draft={props.draft}
        disabled={props.disabled}
        onDraftChange={props.onDraftChange}
        onSubmit={(submission) => props.onSubmit(submission as { query?: unknown })}
      />
      {/* The placeholder/send copy above rides on the shared composer's own
          controls; keep the embed strings exposed for screen readers. */}
      <span className="sr-only">{props.placeholder}</span>
    </div>
  );
}
