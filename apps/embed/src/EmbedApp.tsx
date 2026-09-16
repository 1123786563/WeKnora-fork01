import { useEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent, type KeyboardEvent } from 'react';
import { extractEmbedToken, type EmbedPayload, type EmbedPublicConfig, type EmbedSession } from '@weknora/api-client';
import { createEmbedClient, type EmbedClient } from '@weknora/api-client/embed';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import type { ChatStreamEvent, MessageSuggestionSet } from '@weknora/contracts';
import { createEmbedBridgeGuard, EMBED_MESSAGE_SOURCE } from '@weknora/views/embed/bridge';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';

import { Button } from '../../../packages/ui/src/button.tsx';
import '../../../packages/ui/src/theme.css';
import { attachmentUploadsFromFiles, embedAssistantLabel, embedMessageError, embedUploadLabel, formatEmbedConversationTimestamp, formatEmbedFileSize, imageDataUrisFromFiles, partitionUploadFiles, resolveEmbedLocale, resolveEmbedUploadCapabilities, shouldShowEmbedTimestamp, sourceListFromReferences, translate } from './embed-ui.ts';
import { channelIdFromPath, parentOriginFromReferrer, readStoredSession, readVisitorId, writeStoredSession } from './bootstrap.ts';

interface EmbedRuntime {
  token: string;
  visitorId: string;
  session: EmbedSession;
  config: EmbedPublicConfig;
}

type EmbedMessage = EmbedPayload & { role?: string; content?: unknown; id?: string; created_at?: string };

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
  const [suggestedLoading, setSuggestedLoading] = useState(false);
  const [input, setInput] = useState('');
  const [status, setStatus] = useState<'loading' | 'ready' | 'sending' | 'error'>('loading');
  const [error, setError] = useState('');
  const [locale, setLocale] = useState(() => new URLSearchParams(window.location.search).get('locale') || 'en-US');
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [pickedImages, setPickedImages] = useState<File[]>([]);
  const [pickedAttachments, setPickedAttachments] = useState<File[]>([]);
  const [webSearchEnabled, setWebSearchEnabled] = useState(false);
  const [sessionTitle, setSessionTitle] = useState('');
  const [messageSuggestions, setMessageSuggestions] = useState<Record<string, MessageSuggestionSet>>({});
  const [suggestionBusy, setSuggestionBusy] = useState<string | null>(null);
  const [suggestionAttribution, setSuggestionAttribution] = useState<{ suggestion_set_id: string; question_id: string } | null>(null);
  const loadedSuggestionIds = useRef(new Set<string>());
  const pickedImagePreviews = useMemo(() => pickedImages.map((file) => URL.createObjectURL(file)), [pickedImages]);
  const [userHasScrolledUp, setUserHasScrolledUp] = useState(false);
  const contentRef = useRef<HTMLElement | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const activeAssistantRef = useRef('');
  const hostContextRef = useRef<Record<string, unknown>>({});

  useEffect(() => () => { pickedImagePreviews.forEach((preview) => URL.revokeObjectURL(preview)); }, [pickedImagePreviews]);

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
        if (!active) return;
        const next: EmbedRuntime = { token, visitorId, session, config };
        setRuntime(next);
        setMessages(history as EmbedMessage[]);
        setSuggested([]);
        setSuggestedLoading(config.show_suggested_questions !== false);
        setStatus('ready');
        document.title = titleFor(config);
        postToHost({ type: 'ready', channel_id: channelId });
        if (config.show_suggested_questions !== false) {
          void client.embed.public.suggestedQuestions(channelId, token).then((questions) => {
            if (active) setSuggested(questions);
          }).catch(() => {
            if (active) setSuggested([]);
          }).finally(() => {
            if (active) setSuggestedLoading(false);
          });
        } else {
          setSuggestedLoading(false);
        }
      } catch (cause) {
        if (!active) return;
        bootstrapped = false;
        setStatus('error');
        setError(cause instanceof Error ? cause.message : 'Unable to load this embed channel.');
        setSuggestedLoading(false);
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
    const node = contentRef.current;
    if (!node) return;
    if (!userHasScrolledUp) node.scrollTop = node.scrollHeight;
  }, [messages, suggested, userHasScrolledUp]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    return () => { delete document.documentElement.dataset.theme; };
  }, [theme]);

  useEffect(() => {
    if (!runtime || runtime.config.show_suggested_questions === false) return;
    for (const message of messages) {
      const id = textOf(message.id);
      if (message.role === 'assistant' && message.is_completed === true && id && !loadedSuggestionIds.current.has(id)) {
        loadedSuggestionIds.current.add(id);
        void loadMessageSuggestions(id);
      }
    }
  }, [messages, runtime]);

  async function sendMessage(event?: FormEvent, forcedQuery?: string, attributionOverride?: { suggestion_set_id: string; question_id: string }) {
    event?.preventDefault();
    const query = (forcedQuery ?? input).trim();
    if (!runtime || (!query && pickedImages.length === 0 && pickedAttachments.length === 0) || status === 'sending') return;
    const controller = new AbortController();
    abortRef.current = controller;
    const imageFiles = [...pickedImages];
    const attachmentFiles = [...pickedAttachments];
    setStatus('sending');
    const imageUris = await imageDataUrisFromFiles(imageFiles).catch(() => []);
    const attachmentUploads = await attachmentUploadsFromFiles(attachmentFiles).catch(() => []);
    const localAssistantId = `assistant-${Date.now()}`;
    activeAssistantRef.current = localAssistantId;
    const userMessage: EmbedMessage = {
      id: `user-${Date.now()}`, role: 'user', content: query, created_at: new Date().toISOString(),
      images: imageUris.map((url) => ({ url })),
      attachments: attachmentFiles.map((file) => ({ file_name: file.name, file_size: file.size })),
    };
    const assistantMessage: EmbedMessage = { id: localAssistantId, role: 'assistant', content: '', created_at: new Date().toISOString(), is_completed: false };
    setMessages((current) => [...current, userMessage, assistantMessage]);
    setInput('');
    // Files become data-URI payloads (Vue useEmbedChatSession.ts behavior);
    // the server strips them when the channel disables uploads.
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
          mentioned_items: [],
          ...(imageUris.length > 0 ? { images: imageUris } : {}),
          ...(attachmentUploads.length > 0 ? { attachment_uploads: attachmentUploads } : {}),
          ...((attributionOverride ?? suggestionAttribution) ? { suggestion_attribution: attributionOverride ?? suggestionAttribution } : {}),
          web_search_enabled: Boolean(runtime.config.allow_web_search && runtime.config.agent_web_search_enabled && webSearchEnabled),
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
        if (kind === 'title' || kind === 'session_title') setSessionTitle(eventContent(event));
          if (kind === 'error') next.error = eventContent(event) || 'Chat stream failed';
          return next;
        }));
        if (kind === 'complete') {
          postToHost({ type: 'message_received', channel_id: channelId, session_id: runtime.session.id, content: eventContent(event) }, true);
          const completedId = nextId || activeAssistantRef.current || localAssistantId;
          loadedSuggestionIds.current.add(completedId);
          void loadMessageSuggestions(completedId);
        }
      }, controller.signal);
    } catch (cause) {
      if (!controller.signal.aborted) {
        const message = cause instanceof Error ? cause.message : 'Chat failed.';
        setError(message);
        // Preserve the failed turn in the transcript. This mirrors the Vue
        // embed's error state and keeps failures visible after partial output.
        setMessages((current) => current.map((item) => item.id === localAssistantId ? { ...item, error: message, is_completed: false } : item));
      }
    } finally {
      if (activeAssistantRef.current === localAssistantId) activeAssistantRef.current = '';
      setSuggestionAttribution(null);
      abortRef.current = null;
      setStatus('ready');
    }
  }

  async function loadMessageSuggestions(messageId: string, regenerate = false): Promise<void> {
    if (!runtime || runtime.config.show_suggested_questions === false || !messageId) return;
    setSuggestionBusy(messageId);
    try {
      let set = await client.embed.public.ensureMessageSuggestions(channelId, runtime.token, runtime.session.id, messageId, runtime.session.signature, runtime.visitorId, regenerate);
      for (let attempt = 0; set.status === 'generating' && attempt < 120; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
        set = await client.embed.public.messageSuggestions(channelId, runtime.token, runtime.session.id, messageId, runtime.session.signature, runtime.visitorId);
      }
      setMessageSuggestions((current) => ({ ...current, [messageId]: set }));
      if (set.status === 'ready' && set.questions.length > 0) {
        void client.embed.public.recordMessageSuggestionEvent(channelId, runtime.token, runtime.session.id, runtime.session.signature, runtime.visitorId, set.id, 'impression').catch(() => undefined);
      }
    } catch {
      // Vue treats follow-up generation as optional; transcript failures remain visible.
    } finally {
      setSuggestionBusy((current) => current === messageId ? null : current);
    }
  }

  async function chooseMessageSuggestion(messageId: string, questionId: string, question: string): Promise<void> {
    if (!runtime || status === 'sending') return;
    const set = messageSuggestions[messageId];
    if (!set) return;
    setSuggestionAttribution({ suggestion_set_id: set.id, question_id: questionId });
    void client.embed.public.recordMessageSuggestionEvent(channelId, runtime.token, runtime.session.id, runtime.session.signature, runtime.visitorId, set.id, 'click', questionId).catch(() => undefined);
    await sendMessage(undefined, question, { suggestion_set_id: set.id, question_id: questionId });
  }

  function dismissMessageSuggestions(messageId: string): void {
    if (!runtime) return;
    const set = messageSuggestions[messageId];
    if (!set) return;
    setMessageSuggestions((current) => { const next = { ...current }; delete next[messageId]; return next; });
    void client.embed.public.recordMessageSuggestionEvent(channelId, runtime.token, runtime.session.id, runtime.session.signature, runtime.visitorId, set.id, 'dismiss').catch(() => undefined);
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
    try {
      const session = await client.embed.public.createSession(channelId, runtime.token);
      writeStoredSession(channelId, { ...session, agentId: runtime.config.agent_id });
      setRuntime({ ...runtime, session });
      setMessages([]);
      setSessionTitle('');
      setMessageSuggestions({});
      loadedSuggestionIds.current.clear();
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to start a new conversation.');
    }
  }

  const primaryColor = typeof runtime?.config.primary_color === 'string' ? runtime.config.primary_color : '#2864dc';
  // Channel default_locale wins over the URL locale (Vue EmbedPage.vue).
  const effectiveLocale = resolveEmbedLocale(runtime?.config, locale);
  const t = (key: string, fallback: string) => translate(effectiveLocale, key, fallback);
  const { allowFileUpload, allowImageUpload } = resolveEmbedUploadCapabilities(runtime?.config);
  const channelTitle = runtime ? titleFor(runtime.config) : 'WeKnora';
  const useSessionHeaderTitle = runtime?.config.header_title_mode === 'session';
  const title = useSessionHeaderTitle && sessionTitle.trim() ? sessionTitle.trim() : channelTitle;
  const subtitle = runtime?.config.agent_name && (!useSessionHeaderTitle || !sessionTitle.trim() || runtime.config.agent_name !== title)
    ? textOf(runtime.config.agent_name) : '';
  const addFiles = (files: FileList | null) => {
    const result = partitionUploadFiles(Array.from(files ?? []), pickedImages.length, pickedAttachments.length);
    setPickedImages((current) => [...current, ...result.images]);
    setPickedAttachments((current) => [...current, ...result.attachments]);
  };
  const removeImage = (index: number) => setPickedImages((current) => current.filter((_file, item) => item !== index));
  const removeAttachment = (index: number) => setPickedAttachments((current) => current.filter((_file, item) => item !== index));
  const handleComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void sendMessage();
    }
  };

  return (
    <main className="embed-shell" style={{ '--embed-primary': primaryColor } as CSSProperties} data-locale={locale}>
      <header className="embed-header">
        <div className="embed-mark" aria-hidden="true">{textOf(runtime?.config.agent_avatar) || '✦'}</div>
        <div className="embed-heading"><strong>{title}</strong><span>{subtitle || (runtime ? embedAssistantLabel(effectiveLocale) : embedAssistantLabel(effectiveLocale))}</span></div>
        <Button variant="text" size="small" className="embed-icon-button" type="button" onClick={() => void startNewSession()} disabled={!runtime || status === 'sending' || messages.length === 0} aria-label={t('embed.newChat', 'New conversation')}>＋</Button>
      </header>
      <section className="embed-content" ref={contentRef} aria-live="polite" onScroll={(event) => { const node = event.currentTarget; setUserHasScrolledUp(node.scrollHeight - node.scrollTop - node.clientHeight > 48); }}>
        {status === 'loading' ? <p className="embed-state">{t('embed.loading', 'Loading…')}</p> : null}
        {status === 'error' ? <div className="embed-state embed-error"><strong>{t('embed.sessionFailed', 'Unable to start chat')}</strong><span>{error}</span></div> : null}
        {status !== 'error' && runtime?.config.welcome_message && messages.length === 0 ? <div className="embed-welcome">{runtime.config.welcome_message}</div> : null}
        {status !== 'error' && messages.length === 0 && (suggestedLoading || suggested.length > 0) ? <div className="embed-suggestions"><p className="embed-suggestions-title">{suggested.length > 0 ? t('embed.suggestedQuestions', 'You can ask me') : null}</p>{suggestedLoading && suggested.length === 0 ? [1, 2, 3, 4].map((item) => <div className="embed-suggestion-skeleton" key={item} />) : suggested.map((item, index) => <button type="button" key={`${textOf(item.question)}-${index}`} onClick={() => void sendMessage(undefined, textOf(item.question))}>{textOf(item.question)}</button>)}</div> : null}
        <div className="embed-messages">
          {messages.map((message, index) => <article key={textOf(message.id) || `${message.role}-${index}`}>
            {shouldShowEmbedTimestamp(messages, index) ? <time className="embed-timestamp" dateTime={message.created_at}>{formatEmbedConversationTimestamp(message.created_at, effectiveLocale)}</time> : null}
            <div className={`embed-message embed-message-${message.role === 'user' ? 'user' : 'assistant'}`}>
            <div className="embed-bubble embed-bubble-assistant">
            {message.role === 'user' && Array.isArray(message.images) ? <div className="embed-message-images">{message.images.map((image, imageIndex) => <img key={imageIndex} src={textOf(asRecord(image).url ?? asRecord(image).data)} alt="" />)}</div> : null}
            {message.role === 'user' && Array.isArray(message.attachments) ? <div className="embed-message-attachments">{message.attachments.map((attachment, attachmentIndex) => <div key={attachmentIndex}><strong>{textOf(asRecord(attachment).file_name)}</strong><small>{formatEmbedFileSize(Number(asRecord(attachment).file_size) || 0)}</small></div>)}</div> : null}
            {message.role === 'user' ? textOf(message.content) : null}
            {message.role !== 'user' && textOf(message.content) ? <div className="embed-markdown" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(textOf(message.content)) }} /> : null}
            {message.role !== 'user' && message.error ? <span className="embed-message-error" role="alert">{embedMessageError(effectiveLocale, textOf(message.error))}</span> : null}
            {message.role !== 'user' && sourceListFromReferences(message.references).length > 0 ? <ul className="embed-sources"><li className="embed-sources-title">{t('embed.referencesTitle', 'Sources')}</li>{sourceListFromReferences(message.references).map((source, sourceIndex) => <li key={source.knowledgeId + '-' + source.chunkId + '-' + sourceIndex} className="embed-source">{source.title}</li>)}</ul> : null}
            {message.role !== 'user' && message.id && messageSuggestions[textOf(message.id)]?.status === 'ready' && messageSuggestions[textOf(message.id)]!.questions.length > 0 ? <div className="embed-followups"><div className="embed-followups-heading"><span>{t('embed.followUpQuestions', 'Follow-up questions')}</span><span><button type="button" onClick={() => void loadMessageSuggestions(textOf(message.id), true)} disabled={!messageSuggestions[textOf(message.id)]!.allow_regenerate || suggestionBusy === message.id}>{t('embed.refresh', 'Refresh')}</button><button type="button" onClick={() => dismissMessageSuggestions(textOf(message.id))}>{t('embed.dismiss', 'Dismiss')}</button></span></div>{messageSuggestions[textOf(message.id)]!.questions.map((question) => <button type="button" key={question.id} onClick={() => void chooseMessageSuggestion(textOf(message.id), question.id, question.text)}>{question.text}</button>)}</div> : null}
          </div>
            </div>
          </article>)}
          {status === 'sending' ? <div className="embed-typing" role="status" aria-label={t('embed.loading', 'Loading…')}><span /></div> : null}
        </div>
        {userHasScrolledUp ? <button type="button" className="embed-scroll-bottom" onClick={() => { setUserHasScrolledUp(false); const node = contentRef.current; if (node) node.scrollTop = node.scrollHeight; }} aria-label="Scroll to bottom">⌄</button> : null}
      </section>
      <form className="embed-composer" onSubmit={(event) => void sendMessage(event)}>
        {(pickedAttachments.length > 0 || pickedImages.length > 0) ? <div className="embed-picked"><div className="embed-picked-files">{pickedAttachments.map((file, index) => <div className="embed-file-chip" key={`${file.name}-${index}`}><span>◫</span><span className="embed-file-chip-name">{file.name}</span><button type="button" onClick={() => removeAttachment(index)} aria-label={`Remove ${file.name}`}>×</button></div>)}</div><div className="embed-picked-images">{pickedImages.map((file, index) => <div className="embed-image-thumb" key={`${file.name}-${index}`}><img src={pickedImagePreviews[index]} alt={file.name} /><button type="button" onClick={() => removeImage(index)} aria-label={`Remove ${file.name}`}>×</button></div>)}</div></div> : null}
        <textarea value={input} onChange={(event) => setInput(event.target.value)} onKeyDown={handleComposerKeyDown} placeholder={t('embed.inputPlaceholder', 'Ask a question…')} rows={2} disabled={!runtime || status === 'loading' || status === 'error'} />
        <div className="embed-composer-bar"><div className="embed-composer-controls">{allowFileUpload ? <><label className={`embed-control ${pickedImages.length > 0 ? 'active' : ''}`} aria-label={embedUploadLabel(effectiveLocale, 'image')}>▧<input type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple hidden onChange={(event) => addFiles(event.target.files)} /></label><label className={`embed-control ${pickedAttachments.length > 0 ? 'active' : ''}`} aria-label={embedUploadLabel(effectiveLocale, 'file')}>⌕<input type="file" multiple hidden onChange={(event) => addFiles(event.target.files)} /></label></> : null}{runtime?.config.allow_web_search === true && runtime.config.agent_web_search_enabled === true ? <button type="button" className={`embed-control ${webSearchEnabled ? 'active' : ''}`} onClick={() => { const next = !webSearchEnabled; setWebSearchEnabled(next); hostContextRef.current = { ...hostContextRef.current, web_search_enabled: next }; }} aria-pressed={webSearchEnabled} aria-label={t('embed.webSearch', 'Web search')}>◎</button> : null}</div>{status === 'sending' ? <Button variant="text" type="button" className="embed-send embed-stop" onClick={() => void stopMessage()}>{t('embed.stop', 'Stop')}</Button> : <Button variant="primary" type="submit" className="embed-send" disabled={!runtime || (!input.trim() && pickedImages.length === 0 && pickedAttachments.length === 0)}>{t('embed.send', 'Send')}</Button>}</div>
      </form>
    </main>
  );
}
