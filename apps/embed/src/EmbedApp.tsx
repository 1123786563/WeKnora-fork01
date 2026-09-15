import { useEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent } from 'react';
import { extractEmbedToken, type EmbedPayload, type EmbedPublicConfig, type EmbedSession } from '@weknora/api-client';
import { createEmbedClient, type EmbedClient } from '@weknora/api-client/embed';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import type { ChatStreamEvent } from '@weknora/contracts';
import { createEmbedBridgeGuard, EMBED_MESSAGE_SOURCE } from '@weknora/views/embed/bridge';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';

import { attachmentUploadsFromFiles, imageDataUrisFromFiles, resolveEmbedLocale, resolveEmbedUploadCapabilities, sourceListFromReferences, translate } from './embed-ui.ts';
import { channelIdFromPath, parentOriginFromReferrer, readStoredSession, readVisitorId, writeStoredSession } from './bootstrap.ts';

interface EmbedRuntime {
  token: string;
  visitorId: string;
  session: EmbedSession;
  config: EmbedPublicConfig;
}

type EmbedMessage = EmbedPayload & { role?: string; content?: unknown; id?: string };

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function textOf(value: unknown): string {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object') {
    const item = asRecord(value);
    if (typeof item.text === 'string') return item.text;
    if (typeof item.content === 'string') return item.content;
  }
  return value == null ? '' : String(value);
}

function eventContent(event: ChatStreamEvent): string {
  const data = asRecord(event.data);
  return textOf(event.content ?? data.content);
}

function eventMessageId(event: ChatStreamEvent): string {
  const data = asRecord(event.data);
  return textOf(event.message_id ?? data.message_id);
}

function responseType(event: ChatStreamEvent): string {
  return typeof event.response_type === 'string' ? event.response_type : typeof event.type === 'string' ? event.type : '';
}

function isSessionToken(token: string): boolean {
  return token.trim().startsWith('ems_');
}

function makeTransport() {
  const fetcher: FetchLike = (input, init) => fetch(input, { ...init, credentials: 'omit' });
  return createJsonTransport(fetcher);
}

function titleFor(config: EmbedPublicConfig): string {
  const value = config.display_title ?? config.page_title ?? config.name;
  return typeof value === 'string' && value.trim() ? value.trim() : 'WeKnora';
}

export function EmbedApp() {
  const channelId = useMemo(() => channelIdFromPath(window.location.pathname), []);
  const client = useMemo<EmbedClient>(() => createEmbedClient({
    baseURL: import.meta.env.VITE_API_BASE_URL || window.location.origin,
    // Embed deliberately uses an anonymous transport. The Embed API adds its
    // own channel-scoped Authorization header; browser cookies are omitted.
    transport: makeTransport(),
  }), []);
  const parentGuard = useMemo(() => createEmbedBridgeGuard(parentOriginFromReferrer(document.referrer)), []);
  const [runtime, setRuntime] = useState<EmbedRuntime | null>(null);
  const [messages, setMessages] = useState<EmbedMessage[]>([]);
  const [suggested, setSuggested] = useState<EmbedPayload[]>([]);
  const [input, setInput] = useState('');
  const [status, setStatus] = useState<'loading' | 'ready' | 'sending' | 'error'>('loading');
  const [error, setError] = useState('');
  const [locale, setLocale] = useState(() => new URLSearchParams(window.location.search).get('locale') || 'en-US');
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [pickedImages, setPickedImages] = useState<File[]>([]);
  const [pickedAttachments, setPickedAttachments] = useState<File[]>([]);
  const abortRef = useRef<AbortController | null>(null);
  const activeAssistantRef = useRef('');
  const hostContextRef = useRef<Record<string, unknown>>({});

  function postToHost(payload: Record<string, unknown>, sensitive = false) {
    if (window.parent === window || (sensitive && !parentGuard.canSendSensitive())) return;
    window.parent.postMessage({ source: EMBED_MESSAGE_SOURCE, ...payload }, parentGuard.targetOrigin());
  }

  useEffect(() => {
    if (!channelId) {
      setStatus('error');
      setError('Missing embed channel id.');
      return;
    }
    let active = true;
    let bootstrapped = false;
    const visitorId = readVisitorId(channelId);

    const bootstrap = async (providedToken: string) => {
      const initial = providedToken.trim();
      if (!initial || bootstrapped) return;
      bootstrapped = true;
      setStatus('loading');
      setError('');
      try {
        const token = isSessionToken(initial) ? initial : (await client.embed.public.exchange(channelId, initial)).sessionToken;
        const config = await client.embed.public.config(channelId, token);
        const stored = readStoredSession(channelId);
        const agentId = textOf(config.agent_id);
        let session: EmbedSession | null = null;
        if (stored && (!stored.agentId || !agentId || stored.agentId === agentId)) {
          try {
            await client.embed.public.messages(channelId, token, stored.id, { limit: 1, signature: stored.signature, visitorId });
            session = stored;
          } catch { /* stale or rotated session; create a new signed handle */ }
        }
        if (!session) {
          session = await client.embed.public.createSession(channelId, token);
          writeStoredSession(channelId, { ...session, agentId });
        }
        const history = await client.embed.public.messages(channelId, token, session.id, { limit: 40, signature: session.signature, visitorId });
        const questions = config.show_suggested_questions === false ? [] : await client.embed.public.suggestedQuestions(channelId, token).catch(() => []);
        if (!active) return;
        const next: EmbedRuntime = { token, visitorId, session, config };
        setRuntime(next);
        setMessages(history as EmbedMessage[]);
        setSuggested(questions);
        setStatus('ready');
        document.title = titleFor(config);
        postToHost({ type: 'ready', channel_id: channelId });
      } catch (cause) {
        if (!active) return;
        bootstrapped = false;
        setStatus('error');
        setError(cause instanceof Error ? cause.message : 'Unable to load this embed channel.');
      }
    };

    const listener = (event: MessageEvent) => {
      if (!parentGuard.accept({ sourceIsParent: window.parent !== window && event.source === window.parent, origin: event.origin, data: event.data })) return;
      const data = asRecord(event.data);
      if (data.type === 'provide_token' && (!data.channel_id || data.channel_id === channelId)) void bootstrap(textOf(data.token));
      if (data.type === 'set_context') hostContextRef.current = { ...hostContextRef.current, ...asRecord(data.payload) };
      if (data.type === 'set_locale') setLocale(textOf(asRecord(data.payload).locale ?? data.locale) || 'en-US');
      if (data.type === 'set_theme') setTheme(textOf(asRecord(data.payload).theme ?? data.theme) === 'dark' ? 'dark' : 'light');
      if (data.type === 'open_with_query') setInput(textOf(asRecord(data.payload).query ?? data.query));
    };
    window.addEventListener('message', listener);

    const locationToken = extractEmbedToken(window.location.href);
    if (locationToken) void bootstrap(locationToken);
    else if (window.parent !== window) postToHost({ type: 'bootstrap_request', channel_id: channelId });
    else {
      setStatus('error');
      setError('Missing embed token.');
    }
    return () => {
      active = false;
      window.removeEventListener('message', listener);
      abortRef.current?.abort();
    };
  }, [channelId, client, parentGuard]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    return () => { delete document.documentElement.dataset.theme; };
  }, [theme]);

  async function sendMessage(event?: FormEvent) {
    event?.preventDefault();
    const query = input.trim();
    if (!runtime || (!query && pickedImages.length === 0 && pickedAttachments.length === 0) || status === 'sending') return;
    const controller = new AbortController();
    abortRef.current = controller;
    const localAssistantId = `assistant-${Date.now()}`;
    activeAssistantRef.current = localAssistantId;
    const userMessage: EmbedMessage = { id: `user-${Date.now()}`, role: 'user', content: query, created_at: new Date().toISOString() };
    const assistantMessage: EmbedMessage = { id: localAssistantId, role: 'assistant', content: '', created_at: new Date().toISOString(), is_completed: false };
    setMessages((current) => [...current, userMessage, assistantMessage]);
    setInput('');
    setStatus('sending');
    // Files become data-URI payloads (Vue useEmbedChatSession.ts behavior);
    // the server strips them when the channel disables uploads.
    const imageUris = await imageDataUrisFromFiles(pickedImages).catch(() => []);
    const attachmentUploads = await attachmentUploadsFromFiles(pickedAttachments).catch(() => []);
    setPickedImages([]); setPickedAttachments([]);
    postToHost({ type: 'message_sent', channel_id: channelId, session_id: runtime.session.id, query }, true);
    try {
      await client.embed.public.chat({
        channelId,
        token: runtime.token,
        sessionId: runtime.session.id,
        signature: runtime.session.signature,
        visitorId: runtime.visitorId,
        mode: runtime.config.agent_id && runtime.config.agent_id !== 'builtin-quick-answer' ? 'agent' : 'knowledge',
        body: {
          session_id: runtime.session.id,
          knowledge_base_ids: Array.isArray(runtime.config.knowledge_base_ids) ? runtime.config.knowledge_base_ids : [],
          knowledge_ids: [],
          agent_enabled: runtime.config.agent_id !== 'builtin-quick-answer',
          agent_id: runtime.config.agent_id,
          web_search_enabled: false,
          mentioned_items: [],
          ...(imageUris.length > 0 ? { images: imageUris } : {}),
          ...(attachmentUploads.length > 0 ? { attachment_uploads: attachmentUploads } : {}),
          query: hostContextRef.current.query_prefix ? `${textOf(hostContextRef.current.query_prefix)}${query}` : query,
        },
      }, (event) => {
        const kind = responseType(event);
        const nextId = eventMessageId(event);
        if (nextId) activeAssistantRef.current = nextId;
        setMessages((current) => current.map((message) => {
          if (message.id !== localAssistantId && message.id !== activeAssistantRef.current) return message;
          const next = { ...message };
          if (nextId) next.id = nextId;
          if (kind === 'answer' || kind === 'thinking') next.content = `${textOf(next.content)}${eventContent(event)}`;
          if (kind === 'references') next.references = Array.isArray(asRecord(event.data).references) ? asRecord(event.data).references : event.references;
          if (kind === 'complete') next.is_completed = true;
          if (kind === 'error') next.error = eventContent(event) || 'Chat stream failed';
          return next;
        }));
        if (kind === 'complete') postToHost({ type: 'message_received', channel_id: channelId, session_id: runtime.session.id, content: eventContent(event) }, true);
      }, controller.signal);
    } catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : 'Chat failed.');
    } finally {
      if (activeAssistantRef.current === localAssistantId) activeAssistantRef.current = '';
      abortRef.current = null;
      setStatus('ready');
    }
  }

  async function stopMessage() {
    const current = runtime;
    const messageId = activeAssistantRef.current;
    abortRef.current?.abort();
    abortRef.current = null;
    if (current && messageId && !messageId.startsWith('assistant-')) {
      await client.embed.public.stop(channelId, current.token, current.session.id, messageId, current.session.signature, current.visitorId).catch(() => undefined);
    }
    setStatus('ready');
  }

  async function startNewSession() {
    if (!runtime) return;
    const session = await client.embed.public.createSession(channelId, runtime.token);
    writeStoredSession(channelId, { ...session, agentId: runtime.config.agent_id });
    setRuntime({ ...runtime, session });
    setMessages([]);
  }

  const primaryColor = typeof runtime?.config.primary_color === 'string' ? runtime.config.primary_color : '#2864dc';
  // Channel default_locale wins over the URL locale (Vue EmbedPage.vue).
  const effectiveLocale = resolveEmbedLocale(runtime?.config, locale);
  const t = (key: string, fallback: string) => translate(effectiveLocale, key, fallback);
  const { allowFileUpload, allowImageUpload } = resolveEmbedUploadCapabilities(runtime?.config);
  const title = runtime ? titleFor(runtime.config) : 'WeKnora';

  return (
    <main className="embed-shell" style={{ '--embed-primary': primaryColor } as CSSProperties} data-locale={locale}>
      <header className="embed-header">
        <div className="embed-mark" aria-hidden="true">✦</div>
        <div className="embed-heading"><strong>{title}</strong><span>{runtime ? textOf(runtime.config.agent_name) || 'AI assistant' : 'AI assistant'}</span></div>
        <button className="embed-icon-button" type="button" onClick={() => void startNewSession()} disabled={!runtime || status === 'sending' || messages.length === 0} aria-label={t('embed.newChat', 'New conversation')}>＋</button>
      </header>
      <section className="embed-content" aria-live="polite">
        {status === 'loading' ? <p className="embed-state">Loading…</p> : null}
        {status === 'error' ? <div className="embed-state embed-error"><strong>Unable to start chat</strong><span>{error}</span></div> : null}
        {status !== 'error' && runtime?.config.welcome_message && messages.length === 0 ? <div className="embed-welcome">{runtime.config.welcome_message}</div> : null}
        {status !== 'error' && messages.length === 0 && suggested.length > 0 ? <div className="embed-suggestions">{suggested.map((item, index) => <button type="button" key={`${textOf(item.question)}-${index}`} onClick={() => { setInput(textOf(item.question)); }}>{textOf(item.question)}</button>)}</div> : null}
        <div className="embed-messages">
          {messages.map((message, index) => <article className={`embed-message embed-message-${message.role === 'user' ? 'user' : 'assistant'}`} key={textOf(message.id) || `${message.role}-${index}`}>
            <div className="embed-bubble embed-bubble-assistant">
            {message.role === 'user' ? textOf(message.content) : message.error && !textOf(message.content) ? <span>{'Error: ' + textOf(message.error)}</span> : null}
            {message.role !== 'user' && textOf(message.content) ? <div className="embed-markdown" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(textOf(message.content)) }} /> : null}
            {message.role !== 'user' && sourceListFromReferences(message.references).length > 0 ? <ul className="embed-sources"><li className="embed-sources-title">{t('embed.referencesTitle', 'Sources')}</li>{sourceListFromReferences(message.references).map((source, sourceIndex) => <li key={source.knowledgeId + '-' + source.chunkId + '-' + sourceIndex} className="embed-source">{source.title}</li>)}</ul> : null}
          </div>
          </article>)}
        </div>
      </section>
      <form className="embed-composer" onSubmit={(event) => void sendMessage(event)}>
        <textarea value={input} onChange={(event) => setInput(event.target.value)} placeholder={t('embed.inputPlaceholder', 'Ask a question…')} rows={1} disabled={!runtime || status === 'loading' || status === 'error'} />{allowFileUpload ? <label className="embed-upload">{'Attach'}<input type="file" multiple hidden onChange={(event) => setPickedAttachments(Array.from(event.target.files ?? []))} /></label> : null}{allowImageUpload ? <label className="embed-upload">{"Image"}<input type="file" accept="image/*" multiple hidden onChange={(event) => setPickedImages(Array.from(event.target.files ?? []))} /></label> : null}
        {status === 'sending' ? <button type="button" className="embed-send" onClick={() => void stopMessage()}>{t('embed.stop', 'Stop')}</button> : <button type="submit" className="embed-send" disabled={!runtime || (!input.trim() && pickedImages.length === 0 && pickedAttachments.length === 0)}>{t('embed.send', 'Send')}</button>}
      </form>
    </main>
  );
}
