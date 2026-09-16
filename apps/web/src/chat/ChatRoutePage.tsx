import { useEffect, useMemo, useRef, useState } from 'react';
import type { AgentConfiguration, ChatMessage, ChatSession, MessageSuggestionSet, ModelConfiguration, WeKnoraClient } from '@weknora/api-client';
import type { ChatStreamEvent } from '@weknora/contracts';
import { chatDraftKey } from '@weknora/domain/chat/draft';
import { initialChatStreamState, reduceChatStream, type ChatApproval } from '@weknora/domain/chat/reducer';
import { appendMessages, hasOlderMessages, sessionGroups, sessionPageCount } from '@weknora/domain/chat/session-state';
import { readStoredGroupMode, storeGroupMode } from '@weknora/domain/chat/session-grouping';
import { ChatPage } from '@weknora/views/chat/page';
import { resolveChatCopy } from '@weknora/views/chat/chat-copy';
import { openContextualGuide } from '@weknora/views/guides/contextual-guides';
import type { ChatMentionView, ChatSubmission } from '@weknora/views/chat/composer';
import type { ScopeController } from '@weknora/domain/scope';
import { chatSessionIdFromPath, SHELL_SESSION_ROUTE_EVENT } from './session-route.ts';
import { buildWebChatStreamOptions, CHAT_ATTACHMENT_DEFAULT_EXTENSIONS, initialAgentSelection, mergeChatAttachmentExtensions, resolveChatAttachmentLimits, shouldPollAttachmentStatus, validateChatAttachment, type ChatMentionItem } from './agent-selection.ts';
import { listChatModels, MODEL_CHIP_NOT_CONFIGURED, resolveChatModelChip } from './model-chip.ts';
import { readStoredLocale } from '../i18n.ts';
import { resolveChatAttachmentValidationMessage } from './attachment-messages.ts';
import { resolveChatSessionSourceOptions } from './session-source-options.ts';
import { loadStarterQuestions } from './starter-questions.ts';
import { createWebTerminalController, webSocketTarget, type WebTerminalController, type WebTerminalSnapshot } from './terminal.ts';
import { saveArtifactDownload } from './artifact-download.ts';
import { externalCitationTarget } from './citation.ts';
import { findResumeTargetMessage, markChatMessageStopped } from './resume.ts';
import { buildSteerAction, isSteerConflict, type SteerMentionItem } from './steer-submit.ts';
import { ChatStreamApplicationError, feedWithLastEventId, isChatStreamApplicationError, resumeStreamOptions, type LastEventIdHolder } from './stream-recovery.ts';
import { prepareSendRun } from './send-run.ts';
import { applyOAuthApprovalCancellation, applyOAuthApprovalResolution, applyToolApprovalResolution } from './approval-state.ts';
import { chatClearConfirmation } from './clear-confirmation.ts';
import './chat.css';

interface ChatRoutePageProps {
  client: WeKnoraClient;
  scopeController: ScopeController;
  apiBaseUrl?: string;
  knowledgeBaseId?: string;
  canViewChannelSessions?: boolean;
}

interface ChatAttachmentView {
  id: string;
  name: string;
  status: 'pending' | 'uploading' | 'uploaded' | 'processing' | 'ready' | 'failed';
  attachmentId?: string;
  error?: string;
}

type ChatAttachmentRecord = { file: File; attachmentId?: string; sessionId?: string; generation: number };

function draftStorageKey(scope: ReturnType<ScopeController['current']>['scope'], sessionId: string): string {
  return JSON.stringify(chatDraftKey({ ...scope, sessionId }));
}

function chatModelStorageKey(scope: ReturnType<ScopeController['current']>['scope']): string {
  return `weknora:last-chat-model:${scope.origin}:${scope.userId ?? 'anonymous'}:${scope.tenantId ?? 'default'}`;
}

function readStoredChatModelId(scope: ReturnType<ScopeController['current']>['scope']): string {
  try { return window.localStorage.getItem(chatModelStorageKey(scope))?.trim() ?? ''; } catch { return ''; }
}

export function ChatRoutePage({ client, scopeController, apiBaseUrl = '', knowledgeBaseId, canViewChannelSessions = false }: ChatRoutePageProps) {
  const scope = scopeController.current();
  const copy = resolveChatCopy(readStoredLocale());
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sessionSource, setSessionSource] = useState('web');
  const [sessionKeyword, setSessionKeyword] = useState('');
  const [sessionPage, setSessionPage] = useState(1);
  const [sessionPageCountValue, setSessionPageCountValue] = useState(1);
  // Sidebar group-by toggle persists across reloads (Vue sessionGrouping.ts).
  const [sessionGroupMode, setSessionGroupMode] = useState<'none' | 'date'>(() => readStoredGroupMode());
  const [streamState, setStreamState] = useState(initialChatStreamState);
  const [agents, setAgents] = useState<AgentConfiguration[]>([]);
  const [disabledAgentIds, setDisabledAgentIds] = useState<string[]>([]);
  // Chat models for the composer chip (Vue chatResources chatModels); the
  // KnowledgeQA filter lives in model-chip.ts like the Vue store.
  const [chatModels, setChatModels] = useState<ModelConfiguration[]>([]);
  const [selectedModelId, setSelectedModelId] = useState(() => readStoredChatModelId(scope.scope));
  // Empty-state suggested questions (creatChat view) come from the selected
  // agent's suggested-questions surface; absent without an agent selection.
  const [starterQuestions, setStarterQuestions] = useState<string[]>([]);
  const [starterQuestionsLoading, setStarterQuestionsLoading] = useState(false);
  const [starterQuestionsRefreshKey, setStarterQuestionsRefreshKey] = useState(0);
  const [selectedAgentId, setSelectedAgentId] = useState(() => {
    const query = new URLSearchParams(window.location.search);
    return (query.get('agentId') ?? query.get('agent_id') ?? '').trim();
  });
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(() => chatSessionIdFromPath(window.location.pathname));
  const selectedSessionIdRef = useRef(selectedSessionId);
  const chatRunIdRef = useRef(0);
  const sendInFlightRef = useRef(false);
  // Aborts the in-flight chat stream on stop / session switch / unmount.
  const streamAbortRef = useRef<AbortController | null>(null);
  // Per-assistant-message approval snapshots so pending/resolved cards survive
  // the post-turn history refresh and revisiting a session.
  const approvalMemoryRef = useRef<Map<string, ChatApproval[]>>(new Map());
  // continue-stream is started at most once per persisted incomplete message.
  const resumeStartedRef = useRef<Map<string, string>>(new Map());
  const [draft, setDraft] = useState('');
  const [mentionOptions, setMentionOptions] = useState<ChatMentionView[]>([]);
  const [mentionedItems, setMentionedItems] = useState<ChatMentionView[]>([]);
  const [mentionLoading, setMentionLoading] = useState(false);
  const [mentionError, setMentionError] = useState<string>();
  const mentionLoadedRef = useRef(false);
  const mentionLoadingRef = useRef(false);
  const mentionGenerationRef = useRef(0);
  const [attachments, setAttachments] = useState<ChatAttachmentView[]>([]);
  const [supportedAttachmentExtensions, setSupportedAttachmentExtensions] = useState<readonly string[]>(CHAT_ATTACHMENT_DEFAULT_EXTENSIONS);
  const attachmentRecordsRef = useRef(new Map<string, ChatAttachmentRecord>());
  const attachmentUploadControllersRef = useRef(new Map<string, AbortController>());
  const attachmentPollTimersRef = useRef(new Map<string, number>());
  const attachmentGenerationsRef = useRef(new Map<string, number>());
  const attachmentCounterRef = useRef(0);
  const [loadingSessions, setLoadingSessions] = useState(true);
  // Vue Input-field.vue agent-model watch: the selected agent's config.model_id
  // binds the conversation model; the user's explicit model pick is persisted
  // per origin/user/tenant so it survives a fresh chat route.
  const agentModelId = useMemo(() => {
    const agent = agents.find((item) => item.id === selectedAgentId);
    const modelId = (agent?.config as Record<string, unknown> | undefined)?.model_id;
    return typeof modelId === 'string' ? modelId : undefined;
  }, [agents, selectedAgentId]);
  const modelChip = useMemo(() => resolveChatModelChip({
    models: chatModels,
    agentModelId,
    notConfiguredLabel: MODEL_CHIP_NOT_CONFIGURED[readStoredLocale()] ?? MODEL_CHIP_NOT_CONFIGURED['zh-CN'],
  }), [agentModelId, chatModels]);
  const modelChipLabel = modelChip.label;
  const modelChipContext = modelChip.context;
  const modelChipIsDefault = modelChip.isDefaultContext;
  const modelOptions = useMemo(() => chatModels
    .map((model) => ({ id: String(model.id ?? '').trim(), name: String(model.display_name ?? model.name ?? model.id ?? '').trim() }))
    .filter((model) => model.id.length > 0 && model.name.length > 0), [chatModels]);
  useEffect(() => {
    if (!selectedModelId) return;
    try { window.localStorage.setItem(chatModelStorageKey(scope.scope), selectedModelId); } catch { /* storage may be unavailable */ }
  }, [scope.scope, selectedModelId]);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const [hasMoreMessages, setHasMoreMessages] = useState(false);
  // History refresh in-flight flag (Vue sessions.messages loader).
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [suggestions, setSuggestions] = useState<MessageSuggestionSet | undefined>();
  const suggestionForMessage = useRef<string | null>(null);
  const impressionForSuggestion = useRef<string | null>(null);
  const [error, setError] = useState<string | undefined>();
  const [terminal, setTerminal] = useState<WebTerminalSnapshot>({ status: 'idle', output: '' });
  const terminalController = useRef<WebTerminalController | null>(null);
  const storageKey = useMemo(
    () => selectedSessionId ? draftStorageKey(scope.scope, selectedSessionId) : null,
    [scope.scope, selectedSessionId],
  );

  // Mention resources belong to the active client + scope. A late response
  // from a previous tenant/client must not repopulate the next tenant's
  // picker, and a scope teardown must release the in-flight guard so the next
  // scope can issue a fresh request.
  useEffect(() => {
    const generation = ++mentionGenerationRef.current;
    mentionLoadedRef.current = false;
    mentionLoadingRef.current = false;
    setMentionOptions([]);
    setMentionedItems([]);
    setMentionLoading(false);
    setMentionError(undefined);
    return () => {
      if (mentionGenerationRef.current !== generation) return;
      ++mentionGenerationRef.current;
      mentionLoadedRef.current = false;
      mentionLoadingRef.current = false;
      setMentionLoading(false);
    };
  }, [client, scope.scope]);

  useEffect(() => {
    selectedSessionIdRef.current = selectedSessionId;
  }, [selectedSessionId]);

  // Vue creatChat.vue:81-83 + line 50 — the chat contextual tour arms on chat
  // entry (globalCreatChat / kbCreatChat routes); the React chat page IS that
  // entry, so arm once on mount. The shell-level host applies the dismissal +
  // welcome-tour gates.
  useEffect(() => {
    openContextualGuide('chat');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The platform shell's session list navigates by route: on global chat
  // routes it hands the switch to this page — selectSession pushes the new
  // /platform/chat/:id route, which the shell mirrors into its active-row
  // highlight (Vue keeps one sidebar in menu.vue; chat/index.vue has none).
  useEffect(() => {
    const onShellSessionRoute = (event: Event) => {
      const sessionId = (event as CustomEvent<{ sessionId?: string }>).detail?.sessionId;
      if (typeof sessionId === 'string' && sessionId) selectSession(sessionId);
    };
    window.addEventListener(SHELL_SESSION_ROUTE_EVENT, onShellSessionRoute);
    return () => window.removeEventListener(SHELL_SESSION_ROUTE_EVENT, onShellSessionRoute);
  }, []);

  useEffect(() => () => {
    chatRunIdRef.current += 1;
    streamAbortRef.current?.abort();
    streamAbortRef.current = null;
    for (const controller of attachmentUploadControllersRef.current.values()) controller.abort();
    attachmentUploadControllersRef.current.clear();
    for (const timer of attachmentPollTimersRef.current.values()) window.clearTimeout(timer);
    attachmentPollTimersRef.current.clear();
    for (const record of attachmentRecordsRef.current.values()) {
      if (record.attachmentId && record.sessionId) {
        void client.chat.attachments.remove(record.sessionId, record.attachmentId, scope.signal).catch((cause) => {
          console.error('Attachment cleanup failed during chat unmount', cause);
        });
      }
    }
    attachmentRecordsRef.current.clear();
    attachmentGenerationsRef.current.clear();
  }, []);

  useEffect(() => {
    let active = true;
    setLoadingSessions(true);
    void client.sessions.list({ page: sessionPage, pageSize: 30, source: sessionSource || undefined, keyword: sessionKeyword || undefined, signal: scope.signal }).then(
      (result) => {
        if (active && scopeController.isCurrent(scope.scope)) {
          setSessions(result.data);
          const pageCount = sessionPageCount(result.total, result.page_size);
          setSessionPageCountValue(pageCount);
          if (result.page !== sessionPage) setSessionPage(result.page);
        }
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : copy.operationFailed); },
    ).finally(() => { if (active) setLoadingSessions(false); });
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController, sessionPage, sessionSource, sessionKeyword]);

  useEffect(() => {
    let active = true;
    void client.configuration.agents.listWithState({ creator: 'all', signal: scope.signal }).then(
      (result) => {
        if (!active || !scopeController.isCurrent(scope.scope)) return;
        setAgents(result.items);
        setDisabledAgentIds(result.disabledOwnAgentIds);
        setSelectedAgentId((current) => initialAgentSelection(`?agentId=${encodeURIComponent(current)}`, result.items, result.disabledOwnAgentIds));
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : copy.operationFailed); },
    );
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController]);

  // Vue AttachmentUpload discovers additional parser-supported extensions at
  // runtime. Keep the static baseline if this optional capability is offline.
  useEffect(() => {
    let active = true;
    const parserEngines = client.knowledgeBases?.settings?.parserEngines;
    if (!parserEngines) return () => { active = false; };
    void parserEngines().then(
      (result) => {
        if (!active || !scopeController.isCurrent(scope.scope)) return;
        const dynamic = result.data.filter((engine) => engine.Available !== false).flatMap((engine) => engine.FileTypes ?? []);
        setSupportedAttachmentExtensions(mergeChatAttachmentExtensions(dynamic));
      },
      () => { if (active) setSupportedAttachmentExtensions(CHAT_ATTACHMENT_DEFAULT_EXTENSIONS); },
    );
    return () => { active = false; };
  }, [client, scope.scope, scopeController]);

  // Vue Input-field.vue loadChatModels: fetch the model list once so the
  // composer chip can show the real model name + context spec; a failure is
  // non-fatal and degrades to the localized 未配置 fallback (Vue logs and
  // keeps the empty list).
  useEffect(() => {
    let active = true;
    void client.configuration.models.list(scope.signal).then(
      (result) => {
        if (active && scopeController.isCurrent(scope.scope)) {
          const models = listChatModels(result);
          setChatModels(models);
          setSelectedModelId((current) => current && models.some((model) => String(model.id) === current) ? current : String(models[0]?.id ?? ''));
        }
      },
      () => { if (active) setChatModels([]); },
    );
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController]);

  useEffect(() => {
    if (!selectedSessionId) {
      setMessages([]);
      setStreamState(initialChatStreamState());
      setHasMoreMessages(false);
      setSuggestions(undefined);
      suggestionForMessage.current = null;
      impressionForSuggestion.current = null;
      return;
    }
    let active = true;
    setSuggestions(undefined);
    suggestionForMessage.current = null;
    impressionForSuggestion.current = null;
    setLoadingMessages(true);
    setError(undefined);
    setStreamState(initialChatStreamState());
    void client.sessions.messages(selectedSessionId, { limit: 50, signal: scope.signal }).then(
      (result) => {
        if (active && scopeController.isCurrent(scope.scope)) {
          setMessages(appendMessages([], result));
          setHasMoreMessages(hasOlderMessages(result, 50));
          const assistant = result.filter((message) => message.role === 'assistant' && message.is_completed).at(-1);
          if (assistant) {
            suggestionForMessage.current = assistant.id;
            void loadSuggestions(selectedSessionId, assistant.id, false, scope.signal);
          }
        }
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : copy.operationFailed); },
    ).finally(() => { if (active) setLoadingMessages(false); });
    return () => { active = false; };
  }, [client, selectedSessionId, scope.signal, scope.scope, scopeController]);

  // Resume an interrupted turn: when the newest persisted assistant message is
  // incomplete, attach to it via the continue-stream GET and feed the same
  // reducer so the partial answer keeps streaming after a reload.
  useEffect(() => {
    if (!selectedSessionId || loadingMessages || sendInFlightRef.current) return;
    const resumeId = findResumeTargetMessage(messages);
    if (!resumeId) return;
    const sessionId = selectedSessionId;
    if (resumeStartedRef.current.get(sessionId) === resumeId) return;
    resumeStartedRef.current.set(sessionId, resumeId);
    const controller = new AbortController();
    streamAbortRef.current = controller;
    const runId = ++chatRunIdRef.current;
    setStreamState(initialChatStreamState());
    const feed = createStreamFeed(sessionId, runId, resumeId);
    void client.chat.continueStream(sessionId, resumeId, feed, controller.signal).catch(() => {
      // Non-IM resume failures surface as errors; the partial answer stays.
      if (runId === chatRunIdRef.current && selectedSessionIdRef.current === sessionId) {
        setError(copy.operationFailed);
      }
    }).finally(() => {
      if (streamAbortRef.current === controller) streamAbortRef.current = null;
    });
  }, [client, loadingMessages, messages, selectedSessionId]);

  // session_title SSE: patch the session title and notify the sidebar (the
  // sidebar renders from the sessions list).
  useEffect(() => {
    const title = streamState.sessionTitle;
    if (!title || !selectedSessionId) return;
    setSessions((current) => current.map((session) => session.id === selectedSessionId && session.title !== title
      ? { ...session, title }
      : session));
  }, [selectedSessionId, streamState.sessionTitle]);

  useEffect(() => {
    if (!suggestions || suggestions.status !== 'ready' || !selectedSessionId) return;
    const key = `${selectedSessionId}:${suggestions.id}`;
    if (impressionForSuggestion.current === key) return;
    impressionForSuggestion.current = key;
    void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'impression', '', scope.signal).catch(() => undefined);
  }, [client, scope.signal, selectedSessionId, suggestions]);

  async function loadSuggestions(sessionId: string, messageId: string, ensure: boolean, signal?: AbortSignal): Promise<void> {
    try {
      const result = ensure
        ? await client.chat.suggestions.ensure(sessionId, messageId, false, signal)
        : await client.chat.suggestions.get(sessionId, messageId, signal);
      let current = result;
      for (let attempt = 0; current.status === 'generating' && attempt < 120; attempt += 1) {
        await new Promise<void>((resolve) => window.setTimeout(resolve, 1000));
        if (signal?.aborted || !scopeController.isCurrent(scope.scope) || selectedSessionIdRef.current !== sessionId) return;
        current = await client.chat.suggestions.get(sessionId, messageId, signal);
      }
      if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId && suggestionForMessage.current === messageId) {
        setSuggestions(current.status === 'ready' ? current : undefined);
      }
    } catch {
      // A deployment may have suggestion generation disabled or may not have
      // generated a set for an older message. This is not a chat-history error.
      if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId && suggestionForMessage.current === messageId) setSuggestions(undefined);
    }
  }

  async function loadOlderMessages(): Promise<void> {
    const sessionId = selectedSessionId;
    if (!sessionId || loadingOlderMessages || !hasMoreMessages) return;
    const oldest = messages[0]?.created_at;
    if (!oldest) { setHasMoreMessages(false); return; }
    setLoadingOlderMessages(true);
    try {
      const oldestId = messages[0]?.id;
      const batch = await client.sessions.messages(sessionId, { beforeTime: oldest, limit: 50, signal: scope.signal });
      if (!scopeController.isCurrent(scope.scope) || selectedSessionIdRef.current !== sessionId) return;
      setMessages((current) => appendMessages(current, batch));
      setHasMoreMessages(hasOlderMessages(batch, 50) && batch[0]?.id !== oldestId);
    } catch (cause) {
      if (scopeController.isCurrent(scope.scope)) setError(cause instanceof Error ? cause.message : copy.operationFailed);
    } finally { setLoadingOlderMessages(false); }
  }

  useEffect(() => {
    setDraft(storageKey ? window.localStorage.getItem(storageKey) ?? '' : '');
  }, [storageKey]);

  useEffect(() => {
    terminalController.current?.close();
    terminalController.current = null;
    setTerminal({ status: 'idle', output: '' });
    if (!selectedSessionId) return;
    const target = webSocketTarget(apiBaseUrl, window.location.origin);
    const controller = createWebTerminalController({
      sessionId: selectedSessionId,
      wsOrigin: target.origin,
      basePath: target.basePath,
      issueTicket: (signal) => client.sandbox.issueTicket(selectedSessionId, signal),
      socketFactory: (url) => new WebSocket(url) as unknown as import('./terminal.ts').WebTerminalSocket,
      errorCopy: copy.operationFailed,
    });
    terminalController.current = controller;
    const unsubscribe = controller.subscribe(setTerminal);
    return () => {
      unsubscribe();
      controller.close();
      if (terminalController.current === controller) terminalController.current = null;
    };
  }, [apiBaseUrl, client, scope.signal, selectedSessionId]);

  // Reload starters whenever the new-conversation view is shown and the agent
  // selection changes; failures degrade to an empty list (starter-questions.ts).
  useEffect(() => {
    if (selectedSessionId || !selectedAgentId) {
      setStarterQuestions([]);
      setStarterQuestionsLoading(false);
      return;
    }
    let active = true;
    setStarterQuestionsLoading(true);
    void loadStarterQuestions(client.configuration.agents, selectedSessionId, selectedAgentId, scope.signal).then(
      (questions) => {
        if (!active) return;
        setStarterQuestions(questions);
        setStarterQuestionsLoading(false);
      },
    );
    return () => { active = false; };
  }, [client, selectedAgentId, selectedSessionId, scope.signal, starterQuestionsRefreshKey]);

  function refreshStarterQuestions(): void {
    if (selectedSessionId || !selectedAgentId) return;
    setStarterQuestionsRefreshKey((current) => current + 1);
  }

  function selectSession(sessionId: string, preserveAttachments = false) {
    chatRunIdRef.current += 1;
    streamAbortRef.current?.abort();
    streamAbortRef.current = null;
    selectedSessionIdRef.current = sessionId;
    setSelectedSessionId(sessionId);
    setMentionedItems([]);
    if (!preserveAttachments) void clearAttachments(true);
    window.history.pushState({}, '', `/platform/chat/${encodeURIComponent(sessionId)}`);
  }

  function attachmentIsCurrent(localId: string, generation: number, sessionId: string): boolean {
    const record = attachmentRecordsRef.current.get(localId);
    return Boolean(record && record.generation === generation && record.sessionId === sessionId && attachmentGenerationsRef.current.get(localId) === generation);
  }

  function clearAttachmentPoll(localId: string): void {
    const timer = attachmentPollTimersRef.current.get(localId);
    if (timer !== undefined) window.clearTimeout(timer);
    attachmentPollTimersRef.current.delete(localId);
  }

  function cancelAttachmentOperation(localId: string): void {
    attachmentGenerationsRef.current.set(localId, (attachmentGenerationsRef.current.get(localId) ?? 0) + 1);
    attachmentUploadControllersRef.current.get(localId)?.abort();
    attachmentUploadControllersRef.current.delete(localId);
    clearAttachmentPoll(localId);
  }

  function scheduleAttachmentPoll(localId: string, sessionId: string, attachmentId: string, generation: number): void {
    clearAttachmentPoll(localId);
    const timer = window.setTimeout(() => void pollAttachmentStatus(localId, sessionId, attachmentId, generation), 800);
    attachmentPollTimersRef.current.set(localId, timer);
  }

  async function pollAttachmentStatus(localId: string, sessionId: string, attachmentId: string, generation: number): Promise<void> {
    if (!attachmentIsCurrent(localId, generation, sessionId)) return;
    const controller = new AbortController();
    attachmentUploadControllersRef.current.set(localId, controller);
    try {
      const uploaded = await client.chat.attachments.get(sessionId, attachmentId, controller.signal);
      if (!attachmentIsCurrent(localId, generation, sessionId)) return;
      setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: uploaded.status, error: uploaded.error_message } : item));
      if (shouldPollAttachmentStatus(uploaded.status)) scheduleAttachmentPoll(localId, sessionId, attachmentId, generation);
    } catch (cause) {
      if (controller.signal.aborted || !attachmentIsCurrent(localId, generation, sessionId)) return;
      const message = cause instanceof Error ? cause.message : copy.operationFailed;
      setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: 'failed', error: message } : item));
    } finally {
      if (attachmentUploadControllersRef.current.get(localId) === controller) attachmentUploadControllersRef.current.delete(localId);
    }
  }

  async function uploadAttachment(localId: string, sessionId: string): Promise<string> {
    const record = attachmentRecordsRef.current.get(localId);
    if (!record) throw new Error('Attachment is no longer available.');
    const generation = record.generation;
    record.sessionId = sessionId;
    const controller = new AbortController();
    attachmentUploadControllersRef.current.set(localId, controller);
    setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: 'uploading', error: undefined } : item));
    try {
      const uploaded = await client.chat.attachments.upload(sessionId, {
        file: record.file,
        fileName: record.file.name,
        ...(selectedAgentId ? { agentId: selectedAgentId } : {}),
      }, controller.signal);
      if (!attachmentIsCurrent(localId, generation, sessionId)) {
        try { await client.chat.attachments.remove(sessionId, uploaded.id, scope.signal); }
        catch (cause) { setError(cause instanceof Error ? `Attachment cleanup failed: ${cause.message}` : copy.operationFailed); }
        throw new Error('Attachment upload was cancelled.');
      }
      record.attachmentId = uploaded.id;
      if (uploaded.status === 'failed') throw new Error(uploaded.error_message || 'Attachment upload failed.');
      setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: uploaded.status, attachmentId: uploaded.id, error: uploaded.error_message } : item));
      if (shouldPollAttachmentStatus(uploaded.status)) scheduleAttachmentPoll(localId, sessionId, uploaded.id, generation);
      return uploaded.id;
    } catch (cause) {
      if (controller.signal.aborted) throw cause instanceof Error ? cause : new Error('Attachment upload cancelled.');
      const message = cause instanceof Error ? cause.message : copy.operationFailed;
      if (attachmentIsCurrent(localId, generation, sessionId)) setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: 'failed', error: message } : item));
      throw cause instanceof Error ? cause : new Error(message);
    } finally {
      if (attachmentUploadControllersRef.current.get(localId) === controller) attachmentUploadControllersRef.current.delete(localId);
    }
  }

  async function selectAttachment(file: File): Promise<void> {
    const limits = resolveChatAttachmentLimits();
    const validation = validateChatAttachment(file, attachmentRecordsRef.current.size, limits, supportedAttachmentExtensions);
    if (validation) {
      const message = resolveChatAttachmentValidationMessage(readStoredLocale(), validation, file.name, Math.round(limits.maxSizeBytes / (1024 * 1024)), limits.maxFiles);
      const localId = `rejected-attachment-${++attachmentCounterRef.current}`;
      setAttachments((items) => [...items, { id: localId, name: file.name, status: 'failed', error: message }]);
      return;
    }
    const localId = `local-attachment-${++attachmentCounterRef.current}`;
    const generation = 1;
    attachmentGenerationsRef.current.set(localId, generation);
    attachmentRecordsRef.current.set(localId, { file, generation });
    setAttachments((items) => [...items, { id: localId, name: file.name, status: 'pending' }]);
    const sessionId = selectedSessionIdRef.current;
    if (sessionId) void uploadAttachment(localId, sessionId).catch(() => undefined);
  }

  async function removeAttachment(localId: string): Promise<void> {
    const record = attachmentRecordsRef.current.get(localId);
    cancelAttachmentOperation(localId);
    if (!record?.attachmentId || !record.sessionId) {
      attachmentRecordsRef.current.delete(localId);
      setAttachments((items) => items.filter((item) => item.id !== localId));
      return;
    }
    if (record?.attachmentId && record.sessionId) {
      try {
        await client.chat.attachments.remove(record.sessionId, record.attachmentId, scope.signal);
        attachmentRecordsRef.current.delete(localId);
        setAttachments((items) => items.filter((item) => item.id !== localId));
      } catch (cause) {
        const message = cause instanceof Error ? cause.message : copy.operationFailed;
        attachmentGenerationsRef.current.set(localId, (attachmentGenerationsRef.current.get(localId) ?? 0) + 1);
        setAttachments((items) => items.map((item) => item.id === localId ? { ...item, status: 'failed', error: `Attachment removal failed: ${message}` } : item));
      }
    }
  }

  async function uploadPendingAttachments(sessionId: string): Promise<string[]> {
    const pending = attachments.filter((item) => item.status === 'pending');
    const ids: string[] = [];
    for (const item of pending) ids.push(await uploadAttachment(item.id, sessionId));
    const failed = attachments.find((item) => item.status === 'failed');
    if (failed) throw new Error(failed.error || 'Attachment upload failed.');
    for (const item of attachments) {
      const record = attachmentRecordsRef.current.get(item.id);
      if (record?.attachmentId && item.status !== 'failed' && !ids.includes(record.attachmentId)) ids.push(record.attachmentId);
    }
    return ids;
  }

  async function clearAttachments(cleanup = false): Promise<void> {
    const records = [...attachmentRecordsRef.current.entries()];
    for (const [localId] of records) cancelAttachmentOperation(localId);
    attachmentRecordsRef.current.clear();
    attachmentGenerationsRef.current.clear();
    setAttachments([]);
    if (!cleanup) return;
    for (const [, record] of records) {
      if (!record.attachmentId || !record.sessionId) continue;
      try { await client.chat.attachments.remove(record.sessionId, record.attachmentId, scope.signal); }
      catch (cause) { setError(cause instanceof Error ? `Attachment cleanup failed: ${cause.message}` : copy.operationFailed); }
    }
  }

  function selectAgent(agentId: string) {
    setSelectedAgentId(agentId);
    const url = new URL(window.location.href);
    if (agentId) url.searchParams.set('agentId', agentId);
    else url.searchParams.delete('agentId');
    window.history.replaceState({}, '', `${url.pathname}${url.search}`);
  }

  function changeSessionSource(source: string): void {
    setSessionSource(source);
    setSessionPage(1);
  }

  function changeSessionKeyword(keyword: string): void {
    setSessionKeyword(keyword);
    setSessionPage(1);
  }

  function rememberApprovalSnapshot(assistantMessageId: string | undefined, approvals: ChatApproval[]): void {
    if (assistantMessageId && approvals.length > 0) approvalMemoryRef.current.set(assistantMessageId, approvals);
  }

  function rememberApprovalResolution(pendingId: string, decision: string): void {
    for (const [messageId, approvals] of approvalMemoryRef.current) {
      approvalMemoryRef.current.set(messageId, approvals.map((approval) => approval.pendingId === pendingId
        ? { ...approval, status: 'resolved' as const, decision }
        : approval));
    }
  }

  async function resolveToolApproval(pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>, reason?: string): Promise<void> {
    await client.chat.approvals.resolveTool(pendingId, { decision, ...(modifiedArgs ? { modifiedArgs } : {}), ...(reason ? { reason } : {}) }, scope.signal);
    rememberApprovalResolution(pendingId, decision);
    setStreamState((current) => ({ ...current, approvals: applyToolApprovalResolution(current.approvals, pendingId, decision) }));
  }

  async function cancelOAuth(pendingId: string): Promise<void> {
    await client.chat.approvals.cancelOAuth(pendingId, scope.signal);
    setStreamState((current) => ({ ...current, oauthApprovals: applyOAuthApprovalCancellation(current.oauthApprovals, pendingId, copy.oauthCancel) }));
  }

  function newSteerId(): string {
    const uuid = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : undefined;
    return uuid ?? 'steer-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
  }

  async function steer(content: string, selectedMentions: readonly ChatMentionView[] = mentionedItems): Promise<void> {
    const sessionId = selectedSessionId;
    if (!sessionId) throw new Error('Create or select a conversation first.');
    const streaming = streamState.phase === 'streaming';
    const steerMentions: SteerMentionItem[] = selectedMentions.map((item) => ({
      id: item.id,
      name: item.name,
      type: item.type,
      ...(item.kbType ? { kbType: item.kbType } : {}),
      ...(item.kbId ? { kbId: item.kbId } : {}),
      ...(item.kbName ? { kbName: item.kbName } : {}),
      ...(item.skillName ? { skillName: item.skillName } : {}),
    }));
    const action = buildSteerAction({
      streaming,
      content,
      assistantMessageId: streamState.assistantMessageId,
      mentionedItems: steerMentions,
      newSteerId,
    });
    if (action.kind === 'send') return send(action.submission);
    try {
      await client.chat.steer.enqueue(sessionId, action.input, scope.signal);
    } catch (cause) {
      if (!isSteerConflict(cause)) throw cause;
      // 409: the run moved past the message id we expected. Re-base onto the
      // freshest assistant message id and retry once.
      const rebased = streamState.assistantMessageId;
      if (!rebased || rebased === action.input.expectedAssistantMessageId) throw cause;
      await client.chat.steer.enqueue(sessionId, { ...action.input, expectedAssistantMessageId: rebased }, scope.signal);
    }
  }

  async function downloadArtifact(messageId: string, artifactIndex: number): Promise<void> {
    const sessionId = selectedSessionIdRef.current;
    if (!sessionId) throw new Error('Select a conversation before downloading an artifact.');
    try {
      const artifacts = await client.chat.artifacts.message(sessionId, messageId, scope.signal);
      const artifact = artifacts.find((item) => item.index === artifactIndex);
      if (!artifact) throw new Error('Artifact is no longer available.');
      const response = await client.chat.artifacts.download(sessionId, messageId, artifact.index, scope.signal);
      await saveArtifactDownload(artifact, response, async (content, filename) => {
        const blob = content instanceof Blob ? content : new Blob([content]);
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = filename;
        anchor.click();
        URL.revokeObjectURL(url);
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.operationFailed);
    }
  }

  async function previewArtifact(messageId: string, artifactIndex: number) {
    const sessionId = selectedSessionIdRef.current;
    if (!sessionId) throw new Error('Select a conversation before previewing an artifact.');
    const artifacts = await client.chat.artifacts.message(sessionId, messageId, scope.signal);
    const artifact = artifacts.find((item) => item.index === artifactIndex);
    if (!artifact) throw new Error('Artifact is no longer available.');
    const response = await client.chat.artifacts.download(sessionId, messageId, artifact.index, scope.signal);
    return { body: response.body, contentType: response.contentType };
  }

  async function authorizeOAuth(pendingId: string, serviceId: string): Promise<void> {
    const authorization = await client.configuration.mcp.oauth.authorizeUrl(serviceId, {
      redirectURI: `${window.location.origin}/api/v1/mcp-oauth/callback`,
      frontendRedirect: `${window.location.origin}/`,
    }, scope.signal);
    if (!authorization.authorizationUrl || !authorization.authorizationAttempt) throw new Error('MCP authorization could not be started.');
    const popup = window.open(authorization.authorizationUrl, 'mcp_oauth', 'width=600,height=720');
    if (!popup) throw new Error('MCP authorization popup was blocked.');
    try {
      for (let attempt = 0; attempt < 80; attempt += 1) {
        const status = await client.configuration.mcp.oauth.status(serviceId, authorization.authorizationAttempt, scope.signal);
        if (status.authorized) {
          await client.chat.approvals.resolveOAuth(pendingId, { serviceId, decision: 'authorize' }, scope.signal);
          setStreamState((current) => ({ ...current, oauthApprovals: applyOAuthApprovalResolution(current.oauthApprovals, pendingId, true) }));
          return;
        }
        if (popup.closed) throw new Error('MCP authorization was cancelled before completion.');
        await new Promise<void>((resolve) => window.setTimeout(resolve, 1500));
      }
      throw new Error('MCP authorization timed out.');
    } finally {
      try { popup.close(); } catch { /* cross-origin popup close may throw */ }
    }
  }

  async function createSession() {
    setError(undefined);
    try {
      const session = await client.sessions.create();
      setSessions((current) => [session, ...current.filter((item) => item.id !== session.id)]);
      selectSession(session.id);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.operationFailed);
    }
  }

  async function renameSession(sessionId: string, requestedTitle?: string): Promise<void> {
    const current = sessions.find((session) => session.id === sessionId);
    const title = requestedTitle?.trim().replace(/\s+/g, ' ').slice(0, 80);
    if (!title || title === current?.title) return;
    const updated = await client.sessions.update(sessionId, { title, description: current?.description }, scope.signal);
    setSessions((items) => items.map((session) => session.id === sessionId ? updated : session));
  }

  async function toggleSessionPin(sessionId: string, pinned: boolean): Promise<void> {
    try {
      await (pinned ? client.sessions.pin(sessionId, scope.signal) : client.sessions.unpin(sessionId, scope.signal));
      setSessions((items) => items.map((session) => session.id === sessionId ? { ...session, is_pinned: pinned } : session));
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.operationFailed); }
  }

  async function deleteSession(sessionId: string): Promise<void> {
    if (!window.confirm(copy.deleteConfirmBody)) return;
    try {
      await client.sessions.remove(sessionId, scope.signal);
      setSessions((items) => items.filter((session) => session.id !== sessionId));
      if (selectedSessionId === sessionId) {
        selectedSessionIdRef.current = null;
        setSelectedSessionId(null); setMessages([]); setStreamState(initialChatStreamState());
        window.history.pushState({}, '', '/platform/creatChat');
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.operationFailed); }
  }

  async function clearMessages(): Promise<void> {
    if (!selectedSessionId || !window.confirm(chatClearConfirmation(readStoredLocale()))) return;
    try {
      await client.sessions.clear(selectedSessionId, scope.signal);
      setMessages([]);
      setSuggestions(undefined);
      setHasMoreMessages(false);
      setStreamState(initialChatStreamState());
      approvalMemoryRef.current.clear();
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.operationFailed); }
  }

  function updateDraft(value: string) {
    setDraft(value);
    if (storageKey) window.localStorage.setItem(storageKey, value);
  }

  function loadMentionOptions(): void {
    if (mentionLoadedRef.current || mentionLoadingRef.current) return;
    const generation = mentionGenerationRef.current;
    mentionLoadedRef.current = true;
    mentionLoadingRef.current = true;
    setMentionLoading(true);
    setMentionError(undefined);
    const documentSearch = client.knowledge.documents?.search
      ? client.knowledge.documents.search({ recent: true, offset: 0, limit: 20 })
      : Promise.resolve({ data: [] } as any);
    const hasDocumentSearch = typeof client.knowledge.documents?.search === 'function';
    const mcpList = client.configuration?.mcp?.list
      ? client.configuration.mcp.list()
      : Promise.resolve([] as any[]);
    const hasMcpList = typeof client.configuration?.mcp?.list === 'function';
    const skillList = client.configuration?.skills?.list
      ? client.configuration.skills.list()
      : Promise.resolve([] as any[]);
    const hasSkillList = typeof client.configuration?.skills?.list === 'function';
    void Promise.allSettled([
      client.knowledgeBases.list({ creator: 'all' }),
      documentSearch,
      mcpList,
      skillList,
    ]).then(
      async (results) => {
        if (generation !== mentionGenerationRef.current || !scopeController.isCurrent(scope.scope)) return;
        const primarySuccess = results[0].status === 'fulfilled'
          || (hasDocumentSearch && results[1].status === 'fulfilled')
          || (hasMcpList && results[2].status === 'fulfilled')
          || (hasSkillList && results[3].status === 'fulfilled');
        if (!primarySuccess) {
          mentionLoadedRef.current = false;
          setMentionError(copy.knowledgeBasesLoadFailed);
          return;
        }
        const kbValues = results[0].status === 'fulfilled' ? results[0].value : [];
        const kbItems = kbValues.map((item) => ({
          id: item.id,
          name: item.name,
          type: 'kb' as const,
          kbType: item.type === 'faq' ? 'faq' as const : 'document' as const,
        }));
        const tagResults = await Promise.allSettled(kbValues.map((item) => client.knowledge.documents.tags(item.id, { page_size: 200 })));
        if (generation !== mentionGenerationRef.current || !scopeController.isCurrent(scope.scope)) return;
        const tagItems = tagResults.flatMap((result, index) => {
          if (result.status !== 'fulfilled') return [];
          const kb = kbValues[index];
          return result.value.map((tag: any) => ({
            id: String(tag.id),
            name: String(tag.name ?? tag.label ?? tag.id),
            type: 'tag' as const,
            kbId: kb?.id,
            kbName: kb?.name,
          }));
        });
        const fileResult = results[1].status === 'fulfilled' ? results[1].value.data : [];
        const fileItems = Array.isArray(fileResult) ? fileResult.map((item: any) => ({
          id: String(item.id),
          name: String(item.title ?? item.file_name ?? item.id),
          type: 'file' as const,
          kbId: item.knowledge_base_id ?? item.kb_id,
          kbName: item.knowledge_base_name ?? '',
        })) : [];
        const mcpItems = results[2].status === 'fulfilled' ? results[2].value.map((item: any) => ({
          id: item.id,
          name: item.name,
          type: 'mcp' as const,
          description: item.description ?? item.usage_instructions ?? '',
          toolCount: item.catalog?.tool_count,
          catalogStale: Boolean(item.catalog?.stale),
        })) : [];
        const skillItems = results[3].status === 'fulfilled' ? results[3].value.map((item: any) => ({
          id: item.name,
          name: item.name,
          type: 'skill' as const,
          skillName: item.name,
          description: item.description ?? '',
        })) : [];
        setMentionOptions([...kbItems, ...tagItems, ...fileItems, ...mcpItems, ...skillItems]);
      },
      (cause: unknown) => {
        if (generation !== mentionGenerationRef.current || !scopeController.isCurrent(scope.scope)) return;
        mentionLoadedRef.current = false;
        setMentionError(cause instanceof Error ? cause.message : copy.knowledgeBasesLoadFailed);
      },
    ).finally(() => {
      if (generation !== mentionGenerationRef.current || !scopeController.isCurrent(scope.scope)) return;
      mentionLoadingRef.current = false;
      setMentionLoading(false);
    });
  }

  function selectMention(item: ChatMentionView): void {
    setMentionedItems((current) => current.some((selected) => selected.id === item.id) ? current : [...current, item]);
  }

  function removeMention(id: string): void {
    setMentionedItems((current) => current.filter((item) => item.id !== id));
  }

  function selectSuggestion(questionId: string, text: string): void {
    if (suggestions && selectedSessionId) void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'click', questionId, scope.signal).catch(() => undefined);
    updateDraft(text);
  }

  function refreshSuggestions(): void {
    const messageId = streamState.assistantMessageId ?? messages.filter((message) => message.role === 'assistant' && message.is_completed).at(-1)?.id;
    if (messageId && selectedSessionId) { suggestionForMessage.current = messageId; void loadSuggestions(selectedSessionId, messageId, true, scope.signal); }
  }

  function dismissSuggestions(): void {
    if (suggestions && selectedSessionId) void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'dismiss', '', scope.signal).catch(() => undefined);
    setSuggestions(undefined);
  }

  function openCitation(citationId: string): void {
    const target = externalCitationTarget(citationId);
    if (target) window.open(target, '_blank', 'noopener,noreferrer');
  }

  async function openTerminal(): Promise<void> {
    if (!terminalController.current) throw new Error('Select a conversation before opening the terminal.');
    await terminalController.current.open({ provision: true, signal: scope.signal });
  }

  async function terminalInput(input: string): Promise<void> {
    terminalController.current?.sendInput(input);
  }

  function terminalResize(cols: number, rows: number): void {
    terminalController.current?.resize(cols, rows);
  }

  function closeTerminal(): void {
    terminalController.current?.close();
  }

  // Shared reducer feed for both the send POST stream and the continue-stream
  // GET resume; keeps the live stream state, the transient assistant row
  // (with thinking/toolCalls folded in so they survive the history refresh),
  // the approval snapshot memory, and mid-run injected user bubbles in sync.
  function createStreamFeed(sessionId: string, runId: number, transientId: string): (event: ChatStreamEvent) => void {
    let runState = initialChatStreamState();
    const injectedId = (steerId: string, userMessageId?: string) => userMessageId ?? `injected-${steerId}`;
    return (event: ChatStreamEvent) => {
      if (runId !== chatRunIdRef.current || selectedSessionIdRef.current !== sessionId) return;
      runState = reduceChatStream(runState, event);
      setStreamState(runState);
      rememberApprovalSnapshot(runState.assistantMessageId, Object.values(runState.approvals));
      if (runState.phase === 'error') throw new ChatStreamApplicationError(runState.error ?? 'Chat stream failed');
      const injectedRows = runState.injectedUserMessages.map((injected) => ({
        id: injectedId(injected.steerId, injected.userMessageId),
        session_id: sessionId, role: 'user' as const, content: injected.content, is_completed: true,
      }));
      const hasLiveAssistant = Boolean(runState.answer) || Boolean(runState.thinking) || Object.keys(runState.toolCalls).length > 0;
      if (hasLiveAssistant || injectedRows.length > 0) {
        const injectedIds = new Set(injectedRows.map((row) => row.id));
        setMessages((current) => [...current.filter((item) => item.id !== transientId && !injectedIds.has(item.id)), ...injectedRows, ...(hasLiveAssistant ? [{
          id: transientId, session_id: sessionId, role: 'assistant' as const, content: runState.answer,
          is_completed: runState.phase === 'completed',
          thinking: runState.thinking,
          tool_calls: Object.values(runState.toolCalls),
          tool_approvals: Object.values(runState.approvals),
        }] : [])]);
      }
      if (runState.phase === 'completed' && runState.assistantMessageId && runState.assistantMessageId !== suggestionForMessage.current) {
        suggestionForMessage.current = runState.assistantMessageId;
        void loadSuggestions(sessionId, runState.assistantMessageId, true, scope.signal);
      }
    };
  }

  async function stopStream(): Promise<void> {
    const sessionId = selectedSessionIdRef.current;
    if (!sessionId || streamState.phase !== 'streaming') return;
    const messageId = streamState.assistantMessageId;
    const controller = streamAbortRef.current;
    streamAbortRef.current = null;
    controller?.abort();
    chatRunIdRef.current += 1;
    setStreamState((current) => ({ ...current, phase: 'stopped', artifactsPending: false }));
    if (messageId) {
      try { await client.chat.stop(sessionId, messageId, scope.signal); } catch { /* local stop still applies */ }
    }
    setMessages((current) => markChatMessageStopped(current, sessionId, messageId));
  }

  async function send(submission: ChatSubmission): Promise<void> {
    if (sendInFlightRef.current) throw new Error('A chat request is already running.');
    sendInFlightRef.current = true;
    // The stream controller is created inside prepareSendRun AFTER the inline
    // session-create/selectSession teardown: selectSession aborts the
    // registered controller, so a pre-installed one would abort this send
    // before its HTTP request is ever issued.
    let runController: AbortController | null = null;
    try {
      const run = await prepareSendRun({
        selectedSessionId,
        content: submission.content,
        createSession: async (title) => {
          const created = await client.sessions.create({ title });
          setSessions((current) => [created, ...current.filter((item) => item.id !== created.id)]);
          return created;
        },
        onSessionSelected: (created) => selectSession(created.id, true),
      });
      runController = run.controller;
      streamAbortRef.current = runController;
      const sessionId = run.sessionId;
      if (attachments.some((item) => item.status === 'uploading')) throw new Error('Attachment is still uploading.');
      const attachmentIds = await uploadPendingAttachments(sessionId);
      const runId = ++chatRunIdRef.current;
      const feed = createStreamFeed(sessionId, runId, `stream-${sessionId}`);
      setStreamState(initialChatStreamState());
      const streamMentions: ChatMentionItem[] = mentionedItems.map((item) => ({
        id: item.id,
        name: item.name,
        type: item.type,
        ...(item.kbType ? { kb_type: item.kbType } : {}),
        ...(item.kbId || item.type === 'kb' ? { kb_id: item.kbId ?? item.id } : {}),
        ...(item.kbName || item.type === 'kb' ? { kb_name: item.kbName ?? item.name } : {}),
        ...(item.skillName ? { skill_name: item.skillName } : {}),
      }));
      const streamOptions = { ...buildWebChatStreamOptions(sessionId, submission.content, selectedAgentId, knowledgeBaseId, attachmentIds, streamMentions, submission.modelId ?? selectedModelId), signal: runController.signal };
      // Track the newest SSE event id so a mid-flight transport failure can
      // resume exactly once with the Last-Event-ID header before the error
      // surfaces (Vue parity: EventSource-style automatic reconnection).
      const lastEventId: LastEventIdHolder = {};
      try {
        await client.chat.stream(streamOptions, feedWithLastEventId(feed, lastEventId));
      } catch (cause) {
        if (runController.signal.aborted) return;
        if (isChatStreamApplicationError(cause)) throw cause;
        const retry = resumeStreamOptions(streamOptions, lastEventId.id);
        try {
          if (!retry) throw cause;
          await client.chat.stream(retry, feedWithLastEventId(feed, lastEventId));
        } catch (retryCause) {
          if (runController.signal.aborted) return;
          if (!isChatStreamApplicationError(retryCause) && scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId) {
            setStreamState((current) => ({ ...current, phase: 'error', error: retryCause instanceof Error ? retryCause.message : copy.operationFailed, artifactsPending: false }));
          }
          throw retryCause;
        }
      }
      if (runId !== chatRunIdRef.current || selectedSessionIdRef.current !== sessionId) return;
      // The server persists the user message before opening the stream. Refresh
      // the bounded history after a successful turn so the UI replaces the
      // transient assistant row with authoritative user/assistant message ids;
      // the pending composer remains the sole visible sending/failed row.
      try {
        const persisted = await client.sessions.messages(sessionId, { limit: 50, signal: scope.signal });
        if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId) setMessages(appendMessages([], persisted));
      } catch {
        // The streamed answer remains visible if the post-turn history refresh
        // is unavailable; a later session selection reloads authoritative data.
      }
      clearAttachments();
      setMentionedItems([]);
    } catch (cause) {
      // A stop request or session switch aborts the stream on purpose; that is
      // not a failed submission.
      if (runController?.signal.aborted) return;
      throw cause;
    } finally {
      sendInFlightRef.current = false;
      if (runController && streamAbortRef.current === runController) streamAbortRef.current = null;
    }
  }

  return <ChatPage
    sessions={sessions}
    selectedSessionId={selectedSessionId}
    messages={messages}
    draft={draft}
    loadingSessions={loadingSessions}
    loadingMessages={loadingMessages}
    error={error}
    onSelectSession={selectSession}
    onCreateSession={() => void createSession()}
    onDraftChange={updateDraft}
    attachments={attachments}
    onAttachmentSelect={selectAttachment}
    onRemoveAttachment={removeAttachment}
    attachmentAccept={supportedAttachmentExtensions}
    mentionOptions={mentionOptions}
    mentionedItems={mentionedItems}
    mentionLoading={mentionLoading}
    mentionError={mentionError}
    onMentionOpen={loadMentionOptions}
    onMentionSelect={selectMention}
    onMentionRemove={removeMention}
    agents={agents.map((agent) => ({ id: agent.id, name: agent.name, disabled: disabledAgentIds.includes(agent.id) }))}
    selectedAgentId={selectedAgentId}
    onAgentChange={selectAgent}
    modelLabel={modelChipLabel}
    modelContext={modelChipContext}
    modelContextIsDefault={modelChipIsDefault}
    modelOptions={modelOptions}
    selectedModelId={selectedModelId}
    onModelChange={setSelectedModelId}
    starterQuestions={starterQuestions}
    starterQuestionsLoading={starterQuestionsLoading}
    onRefreshStarterQuestions={refreshStarterQuestions}
    onStarterQuestionClick={(question) => updateDraft(question)}
    toolApprovals={(() => {
      const live = Object.values(streamState.approvals);
      if (live.length > 0) return live;
      const latestAssistantId = streamState.assistantMessageId
        ?? messages.filter((message) => message.role === 'assistant').at(-1)?.id;
      return latestAssistantId ? approvalMemoryRef.current.get(latestAssistantId) ?? [] : [];
    })()}
    oauthApprovals={Object.values(streamState.oauthApprovals)}
    onResolveToolApproval={resolveToolApproval}
    onAuthorizeOAuth={authorizeOAuth}
    onCancelOAuth={cancelOAuth}
    onSteer={steer}
    onRenameSession={renameSession}
    onToggleSessionPin={toggleSessionPin}
    onDeleteSession={deleteSession}
    sessionGroups={sessionGroups(sessions, new Date(), sessionGroupMode)}
    sessionSource={sessionSource}
    sessionSourceOptions={resolveChatSessionSourceOptions(readStoredLocale(), canViewChannelSessions)}
    onSessionSourceChange={changeSessionSource}
    sessionKeyword={sessionKeyword}
    onSessionKeywordChange={changeSessionKeyword}
    sessionPage={sessionPage}
    sessionPageCount={sessionPageCountValue}
    onSessionPageChange={setSessionPage}
    sessionGroupMode={sessionGroupMode}
    onSessionGroupModeChange={(mode) => { setSessionGroupMode(mode); storeGroupMode(mode); }}
    onClearSession={clearMessages}
    loadingOlderMessages={loadingOlderMessages}
    hasMoreMessages={hasMoreMessages}
    onLoadOlderMessages={() => void loadOlderMessages()}
    suggestions={suggestions}
    onSuggestionClick={selectSuggestion}
    onRefreshSuggestions={refreshSuggestions}
    onDismissSuggestions={dismissSuggestions}
    onCitationClick={openCitation}
    onArtifactDownload={downloadArtifact}
    onArtifactPreview={previewArtifact}
    terminal={selectedSessionId ? terminal : undefined}
    onOpenTerminal={selectedSessionId ? openTerminal : undefined}
    onTerminalInput={selectedSessionId ? terminalInput : undefined}
    onTerminalResize={selectedSessionId ? terminalResize : undefined}
    onCloseTerminal={selectedSessionId ? closeTerminal : undefined}
    stream={{ phase: streamState.phase, thinking: streamState.thinking, references: streamState.references, toolCalls: Object.values(streamState.toolCalls), artifactsPending: streamState.artifactsPending }}
    onStopStream={() => void stopStream()}
    send={send}
  />;
}
