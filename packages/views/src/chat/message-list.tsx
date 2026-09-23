import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ChatMessage, FeedbackRating, MessageSuggestionSet } from '@weknora/contracts';
import { hasSessionChanged, scrollTopAfterPrepend, shouldStickToBottom } from '@weknora/domain/chat/session-state';
import { isArtifactExpired, normalizeArtifactList, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import { assistantMessageExtras } from '@weknora/domain/chat/message-extras';
import { shouldShowConversationTimestamp } from '@weknora/domain/chat/message-timestamps';
import { renderChatMarkdown } from './markdown.ts';
import { hydrateMermaidBlocksWithBrowserDefaults } from './mermaid.ts';
import {
  ArtifactPreview,
  artifactPreviewModel,
  ARTIFACT_PREVIEW_DEFAULT_WIDTH,
  clampArtifactPreviewWidth,
  formatArtifactDateTime,
  formatArtifactSize,
  type ArtifactPreviewPayload,
} from './artifact-preview.tsx';
import { resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';
import { splitHistoryThinking, type LiveThinkingState } from './live-thinking.ts';
import {
  BotMessageFace,
  ConversationTimestamp,
  UserMessageFace,
} from './message-face.tsx';

/*
 * Vue 事实源（frontend/src/views/chat/index.vue 消息区 + usermsg/botmsg 面）：
 * .chat_scroll_box（滚动容器）> .msg_list（16px 列距、max 960）> .msg-item-wrapper
 * > [time.conversation-time] + .message-row > usermsg/botmsg。
 * wk-* hook 类随 Vue DOM 保留（apps/web 测试锚点：wk-chat-message-scroll /
 * wk-chat-messages / wk-chat-timestamp / wk-chat-message-row--user /
 * wk-chat-typing / wk-chat-scroll-bottom 等）。
 */
/** 工具行（.wk-list li 生效值；T15 语义化为 .wk-chat-tool-list-item）。 */
export const TOOL_LIST_ITEM = 'wk-chat-tool-list-item';

export type AssistantTimelineItem =
  | { kind: 'tool'; id: string; name?: string; status: 'pending' | 'completed' | 'failed'; result?: unknown }
  | { kind: 'finish' };

export interface PendingChatMessage {
  content: string;
  status: 'pending' | 'failed';
  error?: string;
}

export interface MessageListProps {
  messages: readonly ChatMessage[];
  pending?: PendingChatMessage;
  onRetry?(): void;
  loadingOlder?: boolean;
  hasMore?: boolean;
  onLoadOlder?(): void;
  suggestions?: MessageSuggestionSet;
  onSuggestionClick?(questionId: string, text: string): void;
  onRefreshSuggestions?(): void;
  onDismissSuggestions?(): void;
  onCitationClick?(citationId: string): void;
  /** Host-owned Vue botmsg knowledge-base action; absent means unavailable. */
  onBookmark?(messageId: string): void | Promise<void>;
  /** SP11 message feedback entry: hosts that provide it get the like/dislike
   *  pair on every assistant toolbar; absent hides the whole affordance
   *  (the embed channel never wires it — same capability gate as onBookmark). */
  onRateMessage?(messageId: string, rating: FeedbackRating): void;
  /** SP11 toggle-off path: clears the persisted rating for a message. */
  onRemoveRating?(messageId: string): void;
  /** SP11 current rating lookup driving aria-pressed; undefined = unrated. */
  ratingOf?(messageId: string): FeedbackRating | undefined;
  /** Vue usermsg/botmsg 分叉 entry: fork this session at the given message. */
  onForkMessage?(messageId: string): void;
  /** Vue index.vue forkAffordanceOf gate; absent hides every fork button. */
  canForkMessage?(messageId: string): boolean;
  onArtifactDownload?(messageId: string, artifactIndex: number): Promise<void>;
  onArtifactPreview?(messageId: string, artifactIndex: number): Promise<ArtifactPreviewPayload>;
  sessionId?: string | null;
  /** True while the turn streams and no assistant content has arrived (Vue index.vue typing dots). */
  typingIndicator?: boolean;
  /** Vue ChatReferencesDrawer entry: the collapsed 检索完成 summary toggles the
   *  shared reference panel; absent renders the summary without a toggle. */
  onToggleReferences?(): void;
  /** Whether the shared reference panel is currently expanded (drives aria-expanded). */
  referencesOpen?: boolean;
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
  /**
   * 滚动容器 ref 注入（page 层持有）：Vue .scroll-to-bottom-btn 挂在 .chat 根，
   * 点击回底需要访问 .chat_scroll_box；同时把「已上滚」状态上报给 page。
   */
  scrollContainerRef?: { current: HTMLDivElement | null };
  onScrolledUpChange?(scrolledUp: boolean): void;
}

export function messageArtifactItems(message: Record<string, unknown>): ChatArtifact[] {
  return normalizeArtifactList(Array.isArray(message.artifacts) ? message.artifacts : undefined);
}

export function renderMessageHtml(message: Pick<ChatMessage, 'content'>, invalidImageLabel?: string): string {
  return renderChatMarkdown(message.content, invalidImageLabel ? { invalidImageLabel } : {});
}

export async function writeClipboardText(text: string, clipboard?: { writeText(t: string): Promise<void> }): Promise<void> {
  const target = clipboard ?? (typeof navigator !== 'undefined' ? navigator.clipboard : undefined);
  if (target?.writeText) {
    await target.writeText(text);
    return;
  }
  if (typeof document === 'undefined') throw new Error('Clipboard is unavailable');
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  try { document.execCommand('copy'); } finally { textarea.remove(); }
}

export function isBookmarkActionAvailable(onBookmark?: MessageListProps['onBookmark']): boolean {
  return Boolean(onBookmark);
}

export function isFeedbackAvailable(onRateMessage?: MessageListProps['onRateMessage']): boolean {
  return typeof onRateMessage === 'function';
}

/*
 * Vue AgentStreamDisplay order: reasoning first, tool calls next, finish last.
 * R464: the reasoning item itself is gone — the Vue main chat face never
 * renders the `thinking` field or the persisted `agent_steps` reasoning text
 * (botmsg.vue only shows `<think>`-tag content via deepThink and the agent
 * timeline is a separate surface), so the simplified React timeline keeps
 * only the tool calls and the finish node.
 */
export function assistantTimelineItems(message: ChatMessage): AssistantTimelineItem[] {
  const extras = assistantMessageExtras(message);
  const items: AssistantTimelineItem[] = [];
  for (const call of extras.toolCalls) items.push({ kind: 'tool', ...call });
  if (message.is_completed === true && items.length > 0) items.push({ kind: 'finish' });
  return items;
}

/*
 * R466-A2 — Vue deepThink.vue over a restored history message: a persisted
 * answer keeps its full `<think>…</think>` block, and the history face mounts
 * with thinking=false, i.e. folded under chat.deepThoughtCompleted「已深度思考」
 * (deepThink.vue onMounted isFold=true) with the reasoning one click away.
 * An unclosed block (interrupted turn) restores the live「思考中...」state —
 * content stays visible while thinking (v-show="!isFold || thinking").
 */
function HistoryDeepThink({ copy: copyTable, state }: { copy: ChatCopyTable; state: LiveThinkingState }) {
  if (!state.showThink) return null;
  const content = state.thinkContent
    ? <p className="wk-vc-message-list-1">{state.thinkContent}</p>
    : null;
  if (state.thinking) {
    return <section className="wk-chat-history-think wk-chat-history-think--live wk-vc-message-list-2" aria-label={copyTable.thinkingAlt}>
      <p role="status" className="wk-vc-message-list-3">
        <span className="wk-vc-message-list-4 wk-mr-none" aria-hidden="true" />
        {copyTable.thinking}
      </p>
      {content}
    </section>;
  }
  return <details className="wk-chat-history-think wk-chat-history-think--done wk-vc-message-list-5">
    <summary className="wk-vc-message-list-6">{copyTable.deepThoughtCompleted}</summary>
    {content}
  </details>;
}

function TypingIndicator({ copy: copyTable }: { copy: ChatCopyTable }) {
  // Vue index.vue showGlobalTypingIndicator：.chat-global-wait + spinner（12px 圆环）
  return <div className="chat-global-wait wk-chat-typing" role="status" aria-label={copyTable.thinkingAlt}>
    <span className="chat-global-wait__spinner" aria-hidden="true" />
  </div>;
}

/*
 * 流式 agent 工具时间线（React 流式呈现保留面）：完成态折叠根由
 * RagPipelineProgressFace 承载（Vue AgentStreamDisplay 完成态）；仅在
 * 消息未完成且带工具调用时以时间线形式展开（Vue 流式 steps 的 React 对应物）。
 */
function StreamingToolTimeline({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const items = assistantTimelineItems(message);
  if (items.length === 0) return null;
  return <section className="wk-chat-message-extras wk-vc-message-list-7" aria-label={copyTable.thinkingAndTools}>
    <ol className="wk-chat-agent-timeline wk-vc-message-list-8">
      {items.map((item) => item.kind === 'tool' ? <li key={item.id} className="wk-chat-agent-timeline-item wk-vc-message-list-9" data-status={item.status}>
        <details className="wk-vc-message-list-10">
          <summary className="wk-vc-message-list-11">{item.name ?? item.id}</summary>
          {item.result !== undefined ? <pre className="wk-vc-message-list-12">{typeof item.result === 'string' ? item.result : JSON.stringify(item.result, null, 2)}</pre> : null}
        </details>
        <small className="wk-vc-message-list-13">{item.status}</small>
      </li> : <li key="finish" className="wk-chat-agent-timeline-item wk-vc-message-list-14" data-status="completed" role="status">✓ <span>{copyTable.approvalResolved}</span></li>)}
    </ol>
  </section>;
}

/** 产物入口（host 能力门控）：完成消息带 artifacts 时展开下载/预览行。 */
function ArtifactList({ copy: copyTable, message, onDownload, onPreview, onOpenList }: { copy: ChatCopyTable; message: ChatMessage; onDownload?: MessageListProps['onArtifactDownload']; onPreview?: (messageId: string, artifactIndex: number) => void | Promise<void>; onOpenList?: (messageId: string) => void }) {
  const artifacts = messageArtifactItems(message);
  if (artifacts.length === 0) return null;
  return <section className="wk-chat-artifacts wk-vc-message-list-15" aria-label={copyTable.artifacts}>
    <div className="wk-vc-message-list-16"><h3 className="wk-vc-message-list-17">{copyTable.artifacts}</h3>{onOpenList ? <button type="button" className="wk-chat-artifacts-open wk-vc-message-list-18" onClick={() => onOpenList(message.id)}>{copyTable.artifacts}</button> : null}</div>
    <ul className="wk-vc-message-list-19">{artifacts.map((artifact) => {
      const expired = isArtifactExpired(artifact);
      const previewable = artifactPreviewModel(artifact, copyTable).kind !== 'download-only';
      return <li key={artifact.index} className="wk-vc-message-list-20"><span>{artifact.fileName}{artifact.version ? ` · v${artifact.version}` : ''}</span>{expired ? <small role="status" className="wk-vc-message-list-21">{copyTable.expired}</small> : <>{previewable && onPreview ? <button type="button" className="wk-vc-message-list-22" onClick={() => void onPreview(message.id, artifact.index)}>{copyTable.preview}</button> : null}{onDownload ? <button type="button" className="wk-vc-message-list-22" onClick={() => void onDownload(message.id, artifact.index)}>{copyTable.download}</button> : <small className="wk-vc-message-list-21">{copyTable.available}</small>}</>}</li>;
    })}</ul>
  </section>;
}

/** Vue shouldRenderAssistantMessage（index.vue）：agent 完成但无事件/引用/内容时整行隐藏。 */
export function shouldRenderAssistantMessage(message: ChatMessage): boolean {
  const row = message as Record<string, unknown>;
  // Vue shouldRenderAssistantMessage：流式中的 assistant 行始终渲染。
  if (message.is_completed !== true) return true;
  const hasStream = Array.isArray(row.agentEventStream) && (row.agentEventStream as unknown[]).length > 0;
  if (hasStream) return true;
  if (Array.isArray(row.knowledge_references) && (row.knowledge_references as unknown[]).length > 0) return true;
  if (typeof message.content === 'string' && message.content.trim()) return true;
  if (!Array.isArray(row.agent_steps) || (row.agent_steps as unknown[]).length === 0) return true;
  return false;
}

export function MessageList({ copy, messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, onCitationClick, onBookmark, onRateMessage, onRemoveRating, ratingOf, onForkMessage, canForkMessage, onArtifactDownload, onArtifactPreview, sessionId = null, typingIndicator = false, onToggleReferences, referencesOpen = false, scrollContainerRef, onScrolledUpChange }: MessageListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const localContainerRef = useRef<HTMLDivElement>(null);
  const containerRef = scrollContainerRef ?? localContainerRef;
  const stickToBottom = useRef(true);
  const previousSessionId = useRef<string | null>(sessionId);
  const previewRequestId = useRef(0);
  const [preview, setPreview] = useState<{ messageId: string; artifact?: ChatArtifact; payload?: ArtifactPreviewPayload; loading: boolean; error?: string } | null>(null);
  const [previewWidth, setPreviewWidth] = useState(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const previewWidthRef = useRef(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const artifactDrawerRef = useRef<HTMLElement | null>(null);
  const resizeCleanup = useRef<(() => void) | null>(null);
  useEffect(() => {
    previewRequestId.current += 1;
    setPreview(null);
  }, [sessionId]);
  useEffect(() => {
    try {
      const stored = Number(window.localStorage.getItem('weknora-chat-artifact-preview-width'));
      if (Number.isFinite(stored)) {
        const width = clampArtifactPreviewWidth(stored, window.innerWidth);
        previewWidthRef.current = width;
        setPreviewWidth(width);
      }
    } catch {
      // Storage is optional in embedded/webview contexts.
    }
    return () => resizeCleanup.current?.();
  }, []);
  useEffect(() => {
    if (preview) artifactDrawerRef.current?.focus();
  }, [preview?.messageId, preview?.artifact?.index]);
  const previousLayout = useRef<{ firstId?: string; length: number; height: number; top: number }>({ length: 0, height: 0, top: 0 });
  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (hasSessionChanged(previousSessionId.current, sessionId)) {
      previousSessionId.current = sessionId;
      stickToBottom.current = true;
      previousLayout.current = { length: 0, height: 0, top: 0 };
      container.scrollTop = 0;
      return;
    }
    const previous = previousLayout.current;
    const firstId = messages[0]?.id;
    const prepended = previous.length > 0 && messages.length > previous.length && firstId !== previous.firstId;
    // R492 chat parity: Vue loads history and keeps the scroll at the top
    // (first message visible); only live-stream growth scrolls to the bottom.
    // The initial 0 → N history fill must therefore not trigger stick-to-bottom.
    const firstHistoryFill = previous.length === 0 && messages.length > 0;
    if (prepended) container.scrollTop = scrollTopAfterPrepend(previous.top, previous.height, container.scrollHeight);
    else if (stickToBottom.current && !firstHistoryFill) container.scrollTop = container.scrollHeight;
    previousLayout.current = { firstId, length: messages.length, height: container.scrollHeight, top: container.scrollTop };
  }, [messages.length, messages[0]?.id, messages.at(-1)?.content, pending?.content, pending?.status]);

  useEffect(() => {
    const root = containerRef.current;
    if (!root || typeof window === 'undefined' || !root.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let disposed = false;
    void (async () => {
      if (disposed) return;
      await hydrateMermaidBlocksWithBrowserDefaults(root);
    })().catch(() => {
      // The escaped Mermaid source remains visible when the optional renderer
      // or sanitizer cannot load in a particular WebView/runtime.
    });
    return () => { disposed = true; };
  }, [messages, pending?.content, pending?.status]);

  function onScroll() {
    const container = containerRef.current;
    if (!container) return;
    stickToBottom.current = shouldStickToBottom({ scrollTop: container.scrollTop, scrollHeight: container.scrollHeight, clientHeight: container.clientHeight });
    if (container.scrollTop <= 0 && hasMore && !loadingOlder) onLoadOlder?.();
    const scrolledUp = container.scrollHeight - (container.scrollTop + container.clientHeight) > 200;
    onScrolledUpChange?.(scrolledUp);
  }

  function openArtifactList(messageId: string): void {
    const message = messages.find((item) => item.id === messageId);
    if (!message || messageArtifactItems(message).length === 0) return;
    previewRequestId.current += 1;
    setPreview({ messageId, loading: false });
  }

  async function openArtifactPreview(messageId: string, artifactIndex: number): Promise<void> {
    if (!onArtifactPreview) return;
    const message = messages.find((item) => item.id === messageId);
    const artifact = message ? messageArtifactItems(message).find((item) => item.index === artifactIndex) : undefined;
    if (!artifact) return;
    const requestId = ++previewRequestId.current;
    setPreview({ messageId, artifact, loading: true });
    try {
      const payload = await onArtifactPreview(messageId, artifactIndex);
      if (requestId === previewRequestId.current) setPreview({ messageId, artifact, payload, loading: false });
    } catch (cause) {
      if (requestId === previewRequestId.current) setPreview({ messageId, artifact, loading: false, error: cause instanceof Error ? cause.message : t.chatPreviewFailed });
    }
  }

  function closeArtifactDrawer(): void {
    previewRequestId.current += 1;
    setPreview(null);
  }

  function openArtifactFromList(artifactIndex: number): void {
    if (!preview) return;
    void openArtifactPreview(preview.messageId, artifactIndex);
  }

  function backToArtifactList(): void {
    if (!preview) return;
    previewRequestId.current += 1;
    setPreview({ messageId: preview.messageId, loading: false });
  }

  function startArtifactResize(event: React.PointerEvent<HTMLDivElement>): void {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = previewWidth;
    const move = (moveEvent: PointerEvent) => {
      const width = clampArtifactPreviewWidth(startWidth + startX - moveEvent.clientX, window.innerWidth);
      previewWidthRef.current = width;
      setPreviewWidth(width);
    };
    const end = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', end);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      resizeCleanup.current = null;
      try { window.localStorage.setItem('weknora-chat-artifact-preview-width', String(previewWidthRef.current)); } catch { /* optional */ }
    };
    resizeCleanup.current?.();
    resizeCleanup.current = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', end);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', end, { once: true });
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  const previewMessage = preview ? messages.find((message) => message.id === preview.messageId) : undefined;
  const previewArtifacts = previewMessage ? messageArtifactItems(previewMessage) : undefined;

  return <div ref={containerRef} className="chat_scroll_box wk-chat-message-scroll" onScroll={onScroll}>
    <div className="msg_list wk-chat-messages" aria-label={t.messagesLabel}>
    {messages.map((message, index) => {
      const isAssistant = message.role === 'assistant';
      // Vue handleMsgList restore split: the persisted `<think>…</think>`
      // block becomes the folded deepThink header and the message body keeps
      // only the post-tag answer (tags never reach the markdown renderer).
      const historyThink = isAssistant ? splitHistoryThinking(message.content) : null;
      const showSeparator = shouldShowConversationTimestamp(messages, index);
      const row = message as Record<string, unknown>;
      const steerForked = row.steerForked === true;
      if (isAssistant && !shouldRenderAssistantMessage(message)) {
        // Vue .is-empty-segment：wrapper 保留（display:none）。
        return <div key={message.id || `assistant-${message.created_at}-${index}`} className="msg-item-wrapper is-empty-segment" />;
      }
      return <Fragment key={message.id || `${message.role}-${message.created_at}-${index}`}>
        <div className={'msg-item-wrapper' + (steerForked ? ' is-steer-prefix' : '')}>
          {showSeparator ? <ConversationTimestamp copy={t} value={message.created_at} /> : null}
          {message.role === 'user' ? (
            <div className="message-row wk-chat-message-row wk-chat-message-row--user" data-message-id={message.id || undefined}>
              {/* steer retry/remove：宿主 steer 回调（onSteerRetry/onSteerRemove）经
                  ChatRoutePage 作用于队列 chip；消息行内的失败重试/移除入口当前无
                  host 接线（占位 no-op），Vue usermsg steer-failure 面保留结构。 */}
              <UserMessageFace
                copy={t}
                message={message}
                onRetrySteer={undefined}
                onRemoveSteer={undefined}
              />
            </div>
          ) : (
            <div className="message-row wk-chat-message-row">
              {historyThink?.showThink ? <HistoryDeepThink copy={t} state={historyThink} /> : null}
              {message.is_completed === true ? null : <StreamingToolTimeline copy={t} message={message} />}
              <BotMessageFace
                copy={t}
                message={message}
                sessionId={sessionId}
                content={historyThink?.answer ?? message.content ?? ''}
                onBookmark={onBookmark}
                onCitationClick={onCitationClick}
                onToggleReferences={onToggleReferences}
                referencesOpen={referencesOpen}
              />
              <ArtifactList
                copy={t}
                message={message}
                onDownload={onArtifactDownload}
                onPreview={onArtifactPreview ? openArtifactPreview : undefined}
                onOpenList={onArtifactPreview || onArtifactDownload ? openArtifactList : undefined}
              />
            </div>
          )}
        </div>
      </Fragment>;
    })}
    {pending ? <div className="msg-item-wrapper">
      <div className="message-row wk-chat-message-row wk-chat-message-row--user">
        <div className="user_msg_container">
          <div className="user_msg wk-chat-message-bubble">
            <p className="wk-vc-message-list-23">{pending.content}</p>
            {pending.status === 'pending' ? <p role="status" className="wk-chat-pending-state wk-vc-message-list-24">{t.sending}</p> : <p role="alert" className="wk-chat-pending-state wk-vc-message-list-24">{pending.error ?? t.sendFailed}</p>}
            {pending.status === 'failed' && onRetry ? <button type="button" className="wk-chat-retry wk-vc-message-list-25" onClick={onRetry}>{t.retry}</button> : null}
          </div>
        </div>
      </div>
    </div> : null}
    {typingIndicator ? <TypingIndicator copy={t} /> : null}
    </div>
    {preview ? <>
      <button type="button" className="wk-chat-artifact-drawer-overlay" aria-label={t.close} onClick={closeArtifactDrawer} />
      <aside
        className="wk-chat-artifact-drawer"
        ref={artifactDrawerRef}
        style={{ width: preview.artifact ? `${previewWidth}px` : '440px' }}
        role="dialog"
        aria-modal="false"
        aria-label={preview.artifact ? `${preview.artifact.fileName} preview` : t.artifacts}
        tabIndex={-1}
        onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); closeArtifactDrawer(); } }}
      >
        <header className="wk-chat-artifact-drawer-header">
          {preview.artifact ? <button type="button" className="wk-chat-artifact-drawer-back" aria-label={t.artifactPreviewBack} onClick={backToArtifactList}>‹</button> : null}
          <h2>{preview.artifact?.fileName ?? t.artifacts}</h2>
          {preview.artifact && onArtifactDownload ? <button type="button" className="wk-chat-artifact-drawer-action" onClick={() => void onArtifactDownload(preview.messageId, preview.artifact!.index)}>{t.download}</button> : null}
          <button type="button" className="wk-chat-artifact-drawer-close" aria-label={t.close} onClick={closeArtifactDrawer}>×</button>
        </header>
        {preview.artifact ? <div className="wk-chat-artifact-drawer-body"><ArtifactPreview artifact={preview.artifact} payload={preview.payload} loading={preview.loading} error={preview.error} copy={t} showHeader={false} onClose={backToArtifactList} /></div> : <ul className="wk-chat-artifact-drawer-list">{(previewArtifacts ?? []).map((artifact) => {
          const expired = isArtifactExpired(artifact);
          return <li key={artifact.index} className="wk-chat-artifact-drawer-item">
            <button type="button" className="wk-chat-artifact-drawer-item-main" disabled={expired || !onArtifactPreview} onClick={() => openArtifactFromList(artifact.index)}>
              <span className="wk-chat-artifact-drawer-item-name" title={artifact.fileName}>{artifact.fileName}</span>
              <small>{formatArtifactSize(artifact.fileSize)} · {formatArtifactDateTime(artifact.createdAt ?? artifact.modTime)}{expired ? ` · ${t.expired}` : ''}</small>
            </button>
            {onArtifactDownload && !expired ? <button type="button" className="wk-chat-artifact-drawer-action" aria-label={`${t.download}: ${artifact.fileName}`} onClick={() => void onArtifactDownload(preview.messageId, artifact.index)}>{t.download}</button> : null}
          </li>;
        })}</ul>}
        {preview.artifact ? <div className="wk-chat-artifact-drawer-resize" role="separator" aria-orientation="vertical" aria-label={t.artifacts} onPointerDown={startArtifactResize} /> : null}
      </aside>
    </> : null}
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions wk-vc-message-list-26" aria-label={t.followUpQuestions}>
      <div className="wk-chat-suggestions-heading wk-vc-message-list-27"><h2 className="wk-vc-message-list-28">{t.followUpQuestions}</h2><div><button type="button" className="wk-vc-message-list-29" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>{t.suggestedRefresh}</button><button type="button" className="wk-vc-message-list-29" onClick={onDismissSuggestions}>{t.dismiss}</button></div></div>
      <div className="wk-chat-suggestions-grid wk-vc-message-list-30">{suggestions.questions.map((question) => <button type="button" key={question.id} className="wk-vc-message-list-31" onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small className="wk-vc-message-list-32">FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}
