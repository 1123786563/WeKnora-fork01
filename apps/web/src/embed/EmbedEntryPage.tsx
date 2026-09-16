import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
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

interface ChatEntry { role: 'user' | 'assistant'; content: string }

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
  const [messages, setMessages] = useState<ChatEntry[]>([]);
  const [draft, setDraft] = useState('');
  const [streaming, setStreaming] = useState(false);

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
      if (!localePinnedRef.current) {
        const channelLocale = normalizeEmbedLocale(String(cfg.default_locale ?? ''));
        if (channelLocale) setLocale(channelLocale);
      }

      // Resume the persisted session when still valid; otherwise create one
      // (useEmbedBridge L151-172).
      const stored = readStoredSession(channelId);
      let resolved: StoredSession | null = null;
      if (stored) {
        try {
          await client.embed.public.messages(channelId, apiToken, stored.id, { limit: 1, signature: stored.sig, visitorId: getOrCreateVisitorId(channelId) });
          resolved = stored;
        } catch { /* stale/expired: create a fresh signed session */ }
      }
      if (!resolved) {
        const created = await client.embed.public.createSession(channelId, apiToken);
        resolved = { id: created.id, sig: created.signature };
      }
      writeStoredSession(channelId, resolved);
      setSession(resolved);
      setApiToken(apiToken);
      setPhase('ready');
      postToHost(embedReadyPayload(channelId));
    } catch (error) {
      const mapped = mapEmbedError(error);
      setErrorKey(mapped.kind === 'disabled' ? 'channelDisabled' : mapped.kind === 'session' ? 'sessionFailed' : mapped.kind === 'exchange' ? 'invalidChannel' : 'loadError');
      setPhase('error');
    }
  }, [channelId, client, postToHost]);

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
    setMessages((prev) => [...prev, { role: 'assistant', content: '' }]);
    try {
      await client.embed.public.chat({
        channelId,
        token: apiToken,
        sessionId: session.id,
        signature: session.sig,
        visitorId: getOrCreateVisitorId(channelId),
        body: { query },
      }, (event) => {
        const text = typeof event.content === 'string' ? event.content : '';
        if (text) {
          answer += text;
          setMessages((prev) => {
            const next = [...prev];
            next[next.length - 1] = { role: 'assistant', content: answer };
            return next;
          });
        }
      });
      if (answer) postToHost(embedMessageReceivedPayload(channelId, session.id, answer), { sensitive: true });
    } catch {
      setMessages((prev) => {
        const next = [...prev];
        next[next.length - 1] = { role: 'assistant', content: answer || embedText(locale, 'loadError') };
        return next;
      });
    } finally {
      setStreaming(false);
    }
  }, [apiToken, channelId, client, hostContext, locale, postToHost, session, streaming]);

  const startNewChat = useCallback(() => {
    if (!session || !messages.length) return;
    void (async () => {
      try {
        const created = await client.embed.public.createSession(channelId, apiToken);
        const next = { id: created.id, sig: created.signature };
        writeStoredSession(channelId, next);
        setSession(next);
        setMessages([]);
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
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
        {typeof config?.welcome_message === 'string' && config.welcome_message ? <div className="self-start max-w-[85%] rounded-[12px] bg-[#f5f7fa] px-3 py-2 text-[14px] text-[#1f2329]">{config.welcome_message}</div> : null}
        {messages.map((entry, index) => entry.role === 'user' ? (
          <div key={index} className="self-end max-w-[85%] rounded-[12px] px-3 py-2 text-[14px] text-white" style={{ background: 'var(--embed-primary, #2563eb)' }}>{entry.content}</div>
        ) : (
          <div key={index} className="self-start max-w-[85%] rounded-[12px] bg-[#f5f7fa] px-3 py-2 text-[14px] text-[#1f2329]">{entry.content}</div>
        ))}
      </div>
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
