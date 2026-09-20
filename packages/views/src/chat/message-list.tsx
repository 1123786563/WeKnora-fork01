import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ChatMessage, FeedbackRating, MessageSuggestionSet } from '@weknora/contracts';
import { hasSessionChanged, scrollTopAfterPrepend, shouldStickToBottom } from '@weknora/domain/chat/session-state';
import { isArtifactExpired, normalizeArtifactList, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import { assistantMessageExtras } from '@weknora/domain/chat/message-extras';
import {
  formatConversationTimestampLabel,
  shouldShowConversationTimestamp,
} from '@weknora/domain/chat/message-timestamps';
import { copyAnswerText } from '@weknora/domain/chat/copy-answer';
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
import { conversationTimeLabels, formatChatCopy, resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';
import { groupChatReferences } from '@weknora/domain/chat/references';
import { splitHistoryThinking, type LiveThinkingState } from './live-thinking.ts';

/*
 * chat.css → utilities (Tailwind migration). The wk-* classes stay as
 * DOM/test anatomy hooks; conflicts with the legacy unlayered .wk-chat-*
 * rules in apps/web/src/styles.css keep a small guard block in chat.css
 * (utilities lose the @layer cascade) until the Orchestrator deletes it.
 */
/** .wk-chat-answer-toolbar button (+ :hover:not([aria-disabled]) / [aria-disabled]) */
const ANSWER_TOOL_BUTTON = "inline-flex h-[30px] w-[30px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] transition-[background-color,color] duration-[150ms] ease-[ease] hover:not-[aria-disabled='true']:bg-[#f3f3f3] hover:not-[aria-disabled='true']:text-[rgba(0,0,0,0.9)] [aria-disabled='true']:cursor-default [aria-disabled='true']:opacity-55";
/** .wk-chat-typing-dots i (+ nth-child delays via arbitrary animation-delay) */
const TYPING_DOT = "h-[12px] w-[12px] rounded-full border-[1.5px] border-[#c5c5c5] border-t-[rgba(0,0,0,0.6)] bg-[#dcdcdc] opacity-85 animate-[wk-chat-typing-bounce_1.2s_infinite_ease-in-out] motion-reduce:animate-none motion-reduce:opacity-60";
/** .wk-chat-message-row--user .wk-chat-message-bubble (Vue .user_msg pill) */
const USER_BUBBLE = "ml-auto box-border w-max max-w-[min(76%,820px)] rounded-[8px] bg-[#f3f3f3] px-[12px] py-[8px] text-left text-[16px] leading-[1.6] text-[rgba(0,0,0,0.9)] whitespace-pre-wrap break-words [overflow-wrap:anywhere]";

/** Vue usermsg.mentioned_items — the @-mention chips pinned above the user
 *  bubble (right-aligned; Vue chat-resource-chips.less .chat-mentioned-items
 *  with justify flex-end + .chat-mentioned-tag chip geometry). */
interface MentionedItemView { id: string; name: string; type: string; kb_type?: string }

function mentionedItemsOf(message: ChatMessage): MentionedItemView[] {
  const raw = message.mentioned_items;
  if (!Array.isArray(raw)) return [];
  return raw.filter((item): item is MentionedItemView =>
    typeof item === 'object' && item !== null
    && typeof (item as MentionedItemView).id === 'string'
    && typeof (item as MentionedItemView).name === 'string');
}

function MentionedItemsTags(props: { message: ChatMessage }) {
  const items = mentionedItemsOf(props.message);
  if (items.length === 0) return null;
  return (
    <div className="wk-chat-mentioned-items m-0 mb-[2px] flex max-w-full flex-wrap justify-end gap-[6px]">
      {items.map((item) => (
        <span
          key={item.id}
          className={'wk-chat-mentioned-tag inline-flex min-h-[26px] max-w-[200px] box-border items-center gap-[5px] cursor-default rounded-[6px] border border-[#e4e7ec] bg-[#f0f2f5] px-[8px] py-[3px] text-[12px] font-medium leading-[18px] ' + (item.type === 'kb' ? 'is-kb' : '')}
        >
          <span className={'inline-flex shrink-0 items-center justify-center text-[14px] ' + (item.type === 'kb' ? 'text-[#00a870]' : 'text-[rgba(0,0,0,0.45)]')}>
            {item.type === 'kb'
              ? <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" /></svg>
              : <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M14 3v5h5M6 3h8l4 4v14H6z" /></svg>}
          </span>
          <span className="overflow-hidden text-ellipsis whitespace-nowrap">{item.name}</span>
        </span>
      ))}
    </div>
  );
}
/** .wk-list li effective values (the styles.css rule wins the unlayered tie) */
export const TOOL_LIST_ITEM = "flex items-baseline justify-between gap-[1rem] border-b border-[#edf0f5] py-[0.9rem]";

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

function CopyAnswerButton({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  const text = copyAnswerText(message.content);
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current); }, []);
  if (!text) return null;
  async function copy() {
    try {
      await writeClipboardText(text);
      setCopied(true);
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard permission denied: the button simply stays in place.
    }
  }
  return <button type="button" className={`${ANSWER_TOOL_BUTTON}${copied ? ' wk-chat-copy is-copied text-[#07c05f]' : ' wk-chat-copy'}`} onClick={() => void copy()} aria-label={copied ? copyTable.copied : copyTable.copy} title={copied ? copyTable.copied : copyTable.copy}>
    <CopyIcon />
  </button>;
}

function CopyIcon() {
  return <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
    <path d="M10.5 5.5v-2a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2" />
  </svg>;
}

export function isBookmarkActionAvailable(onBookmark?: MessageListProps['onBookmark']): boolean {
  return Boolean(onBookmark);
}

export function isFeedbackAvailable(onRateMessage?: MessageListProps['onRateMessage']): boolean {
  return typeof onRateMessage === 'function';
}

/**
 * Vue ChatRequestInfoButton parity: an ⓘ button on the assistant toolbar that
 * opens a small card with the request's debug rows (request/message/session
 * ids and the sent time) plus a copy-all action.
 */
function RequestInfoButton({ copy: t, message, sessionId }: { copy: ChatCopyTable; message: ChatMessage; sessionId: string | null }) {
  const [open, setOpen] = useState(false);
  const requestId = typeof (message as Record<string, unknown>).request_id === 'string' ? String((message as Record<string, unknown>).request_id) : '';
  const rows = [
    { label: 'Request ID', value: requestId },
    { label: t.requestInfoMessageId, value: message.id },
    { label: t.requestInfoSessionId, value: sessionId ?? '' },
    { label: t.requestInfoSentAt, value: typeof message.created_at === 'string' ? message.created_at : '' },
  ].filter((row) => row.value);
  const copyAll = () => {
    const text = rows.map((row) => `${row.label}: ${row.value}`).join('\n');
    void navigator.clipboard?.writeText(text).catch(() => undefined);
  };
  return <div className="relative">
    <button type="button" className={ANSWER_TOOL_BUTTON} title={t.requestInfo} aria-label={t.requestInfo} aria-expanded={open} onClick={() => setOpen((value) => !value)}>
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9" /><path d="M12 16v-4M12 8h.01" /></svg>
    </button>
    {open ? <div className="absolute bottom-full left-0 z-20 mb-[6px] w-[320px] rounded-[10px] border border-[#dce3ed] bg-white p-[12px] shadow-[0_6px_24px_rgba(15,23,42,0.12)]" role="dialog" aria-label={t.requestInfo}>
      <div className="flex items-center justify-between gap-2 border-b border-[#eef1f5] pb-[6px]">
        <strong className="text-[13px] text-[#101828]">{t.requestInfo}</strong>
        <button type="button" aria-label={t.copy} title={t.copy} className="cursor-pointer border-0 bg-transparent p-[2px] text-[rgba(0,0,0,0.5)] hover:text-[rgba(0,0,0,0.9)]" onClick={copyAll}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><rect x="9" y="9" width="11" height="11" rx="2" /><path d="M5 15V5a2 2 0 012-2h10" /></svg>
        </button>
      </div>
      <div className="mt-[6px] grid gap-[4px]">
        {rows.map((row) => <div key={row.label} className="flex items-baseline justify-between gap-2 text-[12px]"><span className="shrink-0 text-[rgba(0,0,0,0.45)]">{row.label}</span><span className="truncate font-mono text-[rgba(0,0,0,0.75)]" title={row.value}>{row.value}</span></div>)}
      </div>
    </div> : null}
  </div>;
}

function BookmarkAnswerButton({ copy: copyTable, messageId, onBookmark }: { copy: ChatCopyTable; messageId: string; onBookmark?: MessageListProps['onBookmark'] }) {
  // Keep the control truthful until the host provides the Vue manual-editor
  // action; consumers with that capability get the same enabled affordance.
  const unavailable = !isBookmarkActionAvailable(onBookmark);
  return <button type="button" className={`wk-chat-bookmark ${ANSWER_TOOL_BUTTON}`} aria-label={copyTable.addToKnowledgeBase} title={copyTable.addToKnowledgeBase} aria-disabled={unavailable ? 'true' : undefined} disabled={unavailable} onClick={() => void onBookmark?.(messageId)}>
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 2.5h8a1 1 0 0 1 1 1V14l-5-2.6L3 14V3.5a1 1 0 0 1 1-1Z" />
      <path d="M8 5.5v4M6 7.5h4" />
    </svg>
  </button>;
}

function FallbackInfoButton({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const fallback = (message as Record<string, unknown>).is_fallback === true;
  if (!fallback) return null;
  return <button type="button" className={`wk-chat-request-info ${ANSWER_TOOL_BUTTON}`} aria-label={copyTable.fallbackHint} title={copyTable.fallbackHint}>
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" aria-hidden="true">
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 7.2v3.4" />
      <circle cx="8" cy="5" r="0.7" fill="currentColor" stroke="none" />
    </svg>
  </button>;
}

/** Thumbs-up silhouette (Lucide geometry scaled to the 16x16 toolbar grid); down rotates it 180°. */
function ThumbIcon({ down = false }: { down?: boolean }) {
  return <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <g transform={down ? 'rotate(180 8 8)' : undefined}>
      <path d="M5 6.8V14" />
      <path d="M9.8 4.3 9.2 6.8h3.5a1.2 1.2 0 0 1 1.15 1.54l-1.4 4.8a1.2 1.2 0 0 1-1.15.86H3.2a1.2 1.2 0 0 1-1.2-1.2V8a1.2 1.2 0 0 1 1.2-1.2h1.1a1.2 1.2 0 0 0 1.07-.67L8 2a1.9 1.9 0 0 1 1.8 2.3Z" />
    </g>
  </svg>;
}

/*
 * SP11 like/dislike pair: aria-pressed mirrors ratingOf; clicking the pressed
 * rating removes it (toggle), any other click (re)rates the message. The
 * pressed button's tooltip switches to the remove hint so the affordance is
 * truthful before the click.
 */
function FeedbackButtons({ copy: copyTable, message, ratingOf, onRateMessage, onRemoveRating }: {
  copy: ChatCopyTable;
  message: ChatMessage;
  ratingOf?: MessageListProps['ratingOf'];
  onRateMessage?: MessageListProps['onRateMessage'];
  onRemoveRating?: MessageListProps['onRemoveRating'];
}) {
  const current = ratingOf?.(message.id);
  const toggle = (rating: FeedbackRating) => {
    if (current === rating) {
      onRemoveRating?.(message.id);
      return;
    }
    onRateMessage?.(message.id, rating);
  };
  const likeLabel = current === 'like' ? copyTable.feedbackRemoveTooltip : copyTable.feedbackLikeTooltip;
  const dislikeLabel = current === 'dislike' ? copyTable.feedbackRemoveTooltip : copyTable.feedbackDislikeTooltip;
  return <>
    <button type="button" className={`${ANSWER_TOOL_BUTTON} wk-chat-feedback-like`} aria-label={likeLabel} title={likeLabel} aria-pressed={current === 'like'} onClick={() => toggle('like')}>
      <ThumbIcon />
    </button>
    <button type="button" className={`${ANSWER_TOOL_BUTTON} wk-chat-feedback-dislike`} aria-label={dislikeLabel} title={dislikeLabel} aria-pressed={current === 'dislike'} onClick={() => toggle('dislike')}>
      <ThumbIcon down />
    </button>
  </>;
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

function TypingIndicator({ copy: copyTable }: { copy: ChatCopyTable }) {
  return <li className="wk-chat-typing m-0 flex min-h-[28px] list-none items-center border-b border-[#edf0f5] py-[0.8rem]" role="status" aria-label={copyTable.thinkingAlt}>
    <span className="wk-chat-typing-dots inline-flex gap-[4px]" aria-hidden="true"><i className={TYPING_DOT} /><i className={`${TYPING_DOT} [animation-delay:150ms]`} /><i className={`${TYPING_DOT} [animation-delay:300ms]`} /></span>
  </li>;
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
  const content = state.thinkContent
    ? <p className="mt-[6px] mb-0 max-h-[200px] overflow-y-auto whitespace-pre-wrap break-words leading-[1.6] text-[rgba(0,0,0,0.6)]">{state.thinkContent}</p>
    : null;
  if (state.thinking) {
    return <section className="wk-chat-history-think wk-chat-history-think--live mb-[10px] rounded-[8px] border border-[#e7e7e7] bg-white px-[14px] py-[8px] text-[12px]" aria-label={copyTable.thinkingAlt}>
      <p role="status" className="m-0 flex items-center gap-[8px] font-medium text-[rgba(0,0,0,0.9)]">
        <span className="h-[6px] w-[6px] animate-pulse rounded-full bg-[#0052d9] motion-reduce:animate-none" aria-hidden="true" />
        {copyTable.thinking}
      </p>
      {content}
    </section>;
  }
  return <details className="wk-chat-history-think wk-chat-history-think--done mb-[10px] rounded-[8px] border border-[#e7e7e7] bg-white px-[14px] py-[6px] text-[12px]">
    <summary className="cursor-pointer select-none font-medium text-[rgba(0,0,0,0.9)]">{copyTable.deepThoughtCompleted}</summary>
    {content}
  </details>;
}

function completedReferenceDocCount(message: ChatMessage): number {
  // Vue AgentStreamDisplay counts distinct documents (web hits excluded) for
  // 引用了{count}篇文档; the same grouping powers the shared reference panel.
  const row = message as Record<string, unknown>;
  const raw = [...(Array.isArray(row.knowledge_references) ? row.knowledge_references : []), ...(Array.isArray(row.references) ? row.references : [])];
  return groupChatReferences(raw).flatMap((group) => (group.kind === 'document' ? group.items : [])).length;
}

function AssistantExtras(props: { copy: ChatCopyTable; message: ChatMessage; onToggleReferences?: () => void; referencesOpen?: boolean }) {
  const items = assistantTimelineItems(props.message);
  if (items.length === 0) return null;
  if (props.message.is_completed === true) {
    // Vue AgentStreamDisplay completed face: the live tool timeline folds into
    // the single 检索完成 summary line, with the cited-document count beside it
    // and the reference panel one click away (ChatReferencesDrawer).
    const docCount = completedReferenceDocCount(props.message);
    return <section className='wk-chat-message-extras mt-[8px]' aria-label={props.copy.thinkingAndTools}>
      <button type='button' className='wk-chat-retrieval-summary inline-flex cursor-pointer items-center gap-[6px] rounded-[6px] border-0 bg-transparent px-0 py-[2px] text-[12px] text-[rgba(0,0,0,0.45)] transition-[color,background] duration-200 ease-[ease] hover:bg-[#f3f3f3] hover:text-[rgba(0,0,0,0.9)]' aria-expanded={props.referencesOpen === true} onClick={() => props.onToggleReferences?.()}>
        <span>{props.copy.searchDone}</span>
        {docCount > 0 ? <span>{formatChatCopy(props.copy, 'referencesDocCount', { count: docCount })}</span> : null}
      </button>
    </section>;
  }
  return <section className='wk-chat-message-extras mt-[8px] rounded-[8px] border border-[#e7e7e7] text-[13px]' aria-label={props.copy.thinkingAndTools}>
    <ol className='wk-chat-agent-timeline m-0 list-none p-[6px]'>
      {items.map((item) => item.kind === 'tool' ? <li key={item.id} className={`wk-chat-agent-timeline-item flex items-baseline justify-between gap-[1rem] border-b border-[#edf0f5] py-[6px] last:border-b-0`} data-status={item.status}>
        <details className='min-w-0'>
          <summary className='cursor-pointer truncate'>{item.name ?? item.id}</summary>
          {item.result !== undefined ? <pre className='mt-[6px] max-h-[180px] overflow-auto whitespace-pre-wrap break-words rounded-[6px] bg-[#f9f9f9] p-[8px] text-[12px]'>{typeof item.result === 'string' ? item.result : JSON.stringify(item.result, null, 2)}</pre> : null}
        </details>
        <small className='shrink-0 text-[rgba(0,0,0,0.4)]'>{item.status}</small>
      </li> : <li key='finish' className='wk-chat-agent-timeline-item flex items-center gap-[6px] py-[6px] text-[rgba(0,0,0,0.6)]' data-status='completed' role='status'>✓ <span>{props.copy.approvalResolved}</span></li>)}
    </ol>
  </section>;
}

function ArtifactList({ copy: copyTable, message, onDownload, onPreview, onOpenList }: { copy: ChatCopyTable; message: ChatMessage; onDownload?: MessageListProps['onArtifactDownload']; onPreview?: (messageId: string, artifactIndex: number) => void | Promise<void>; onOpenList?: (messageId: string) => void }) {
  const artifacts = messageArtifactItems(message);
  if (artifacts.length === 0) return null;
  return <section className="wk-chat-artifacts mt-[0.7rem] border-t border-[#edf0f5] pt-[0.5rem]" aria-label={copyTable.artifacts}>
    <div className="mb-[0.35rem] flex items-center justify-between gap-[0.5rem]"><h3 className="m-0 text-[0.85rem] text-[rgba(0,0,0,0.6)]">{copyTable.artifacts}</h3>{onOpenList ? <button type="button" className="wk-chat-artifacts-open cursor-pointer border-0 bg-transparent p-0 text-[12px] text-[#245a9b] underline" onClick={() => onOpenList(message.id)}>{copyTable.artifacts}</button> : null}</div>
    <ul className="m-0 flex list-none flex-wrap gap-[0.4rem] p-0">{artifacts.map((artifact) => {
      const expired = isArtifactExpired(artifact);
      const previewable = artifactPreviewModel(artifact, copyTable).kind !== 'download-only';
      return <li key={artifact.index} className="flex items-center gap-[0.4rem] text-[13px] text-[rgba(0,0,0,0.9)]"><span>{artifact.fileName}{artifact.version ? ` · v${artifact.version}` : ''}</span>{expired ? <small role="status" className="text-[#66758b]">{copyTable.expired}</small> : <>{previewable && onPreview ? <button type="button" className="cursor-pointer rounded-[5px] border border-[#b9d1f2] bg-white px-[0.45rem] py-[0.15rem] text-[12px] text-[#245a9b]" onClick={() => void onPreview(message.id, artifact.index)}>{copyTable.preview}</button> : null}{onDownload ? <button type="button" className="cursor-pointer rounded-[5px] border border-[#b9d1f2] bg-white px-[0.45rem] py-[0.15rem] text-[12px] text-[#245a9b]" onClick={() => void onDownload(message.id, artifact.index)}>{copyTable.download}</button> : <small className="text-[#66758b]">{copyTable.available}</small>}</>}</li>;
    })}</ul>
  </section>;
}

export function MessageList({ copy, messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, onCitationClick, onBookmark, onRateMessage, onRemoveRating, ratingOf, onForkMessage, canForkMessage, onArtifactDownload, onArtifactPreview, sessionId = null, typingIndicator = false, onToggleReferences, referencesOpen = false }: MessageListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const timestampLabels = conversationTimeLabels(t);
  const containerRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);
  const previousSessionId = useRef<string | null>(sessionId);
  const previewRequestId = useRef(0);
  const [preview, setPreview] = useState<{ messageId: string; artifact?: ChatArtifact; payload?: ArtifactPreviewPayload; loading: boolean; error?: string } | null>(null);
  const [previewWidth, setPreviewWidth] = useState(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const previewWidthRef = useRef(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const artifactDrawerRef = useRef<HTMLElement | null>(null);
  const resizeCleanup = useRef<(() => void) | null>(null);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
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
    setShowScrollToBottom(container.scrollHeight - (container.scrollTop + container.clientHeight) > 200);
  }

  function scrollToBottom(): void {
    const container = containerRef.current;
    if (!container) return;
    const reducedMotion = typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    container.scrollTo({ top: container.scrollHeight, behavior: reducedMotion ? 'auto' : 'smooth' });
  }

  function onContentClick(event: React.MouseEvent<HTMLDivElement>): void {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const citation = target.closest<HTMLElement>('[data-citation-id]')?.dataset.citationId;
    if (citation) onCitationClick?.(citation);
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
  const previewArtifacts = previewMessage ? messageArtifactItems(previewMessage) : [];

  return <div ref={containerRef} className="wk-chat-message-scroll relative mx-auto min-h-0 w-full max-w-[960px] flex-1 overflow-auto scroll-smooth pt-[8px] px-[4px] pb-[4px] [scrollbar-width:auto]" onScroll={onScroll}>
    {showScrollToBottom ? <button type="button" className="wk-chat-scroll-bottom sticky bottom-[12px] z-[10] mx-auto mt-[-48px] mb-[12px] block h-[36px] w-[36px] cursor-pointer rounded-full border border-[#e7e7e7] bg-white text-[rgba(0,0,0,0.6)] shadow-[0_2px_8px_rgba(0,0,0,0.1)]" aria-label={t.chatScrollBottom} onClick={scrollToBottom}>↓</button> : null}
    {hasMore ? <button type="button" className="wk-chat-load-older mx-auto mb-[12px] block cursor-pointer rounded-[8px] border border-[#dcdcdc] bg-transparent px-[14px] py-[4px] text-[12px] text-[rgba(0,0,0,0.6)] disabled:cursor-not-allowed disabled:opacity-60" disabled={loadingOlder} onClick={onLoadOlder}>{loadingOlder ? t.loadingHistory : t.loadOlder}</button> : null}
    <ol className="wk-chat-messages m-0 flex list-none flex-col gap-[16px] p-0" aria-label={t.messagesLabel}>
    {messages.map((message, index) => {
      const isAssistant = message.role === 'assistant';
      // Vue handleMsgList restore split: the persisted `<think>…</think>`
      // block becomes the folded deepThink header and the message body keeps
      // only the post-tag answer (tags never reach the markdown renderer).
      const historyThink = isAssistant ? splitHistoryThinking(message.content) : null;
      const showSeparator = shouldShowConversationTimestamp(messages, index);
      return <Fragment key={message.id}>
        {showSeparator ? <li className="wk-chat-timestamp block list-none select-none px-0 pt-[4px] pb-[8px] text-center text-[12px] leading-[20px] text-[rgba(0,0,0,0.26)] tabular-nums" role="separator">{formatConversationTimestampLabel(message.created_at, timestampLabels)}</li> : null}
        <li data-role={message.role} className={isAssistant ? 'wk-chat-message-row wk-chat-message-row--assistant flex w-full flex-col' : 'wk-chat-message-row wk-chat-message-row--user flex w-full flex-col'}>
        <div className={isAssistant ? 'wk-chat-message-body flex min-w-0 max-w-full flex-col' : 'wk-chat-message-body flex min-w-0 max-w-full flex-col items-end'}>
          {historyThink?.showThink ? <HistoryDeepThink copy={t} state={historyThink} /> : null}
          {isAssistant ? <AssistantExtras copy={t} message={message} onToggleReferences={onToggleReferences} referencesOpen={referencesOpen} /> : null}
          {isAssistant ? <div className="wk-chat-message-content m-0 text-[16px] leading-[1.6] text-[rgba(0,0,0,0.9)] break-words [overflow-wrap:anywhere]" onClick={onContentClick} dangerouslySetInnerHTML={{ __html: renderMessageHtml({ content: historyThink?.answer ?? message.content }, t.invalidImageLink) }} /> : <>
            <MentionedItemsTags message={message} />
            <div className={`wk-chat-message-bubble ${USER_BUBBLE}`}>{message.content}</div>
          </>}
          {isAssistant ? <div className="wk-chat-answer-toolbar mt-[6px] ml-[-7px] flex min-h-[30px] items-center justify-start gap-[4px]">
            <CopyAnswerButton copy={t} message={message} />
            <BookmarkAnswerButton copy={t} messageId={message.id} onBookmark={onBookmark} />
            <FallbackInfoButton copy={t} message={message} />
            <RequestInfoButton copy={t} message={message} sessionId={sessionId} />
            {suggestions?.status === 'generating' && index === messages.length - 1 ? (
              <span className="wk-chat-follow-up-loading inline-flex items-center gap-[4px] text-[12px] text-[rgba(0,0,0,0.45)]" role="status">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M9 18h6M10 21h4" /><path d="M12 3a6 6 0 00-4 10c.6.6 1 1.2 1 2h6c0-.8.4-1.4 1-2a6 6 0 00-4-10z" /></svg>
                {t.followUpQuestionsLoading}
              </span>
            ) : null}
          </div> : null}
          {isAssistant ? <ArtifactList copy={t} message={message} onDownload={onArtifactDownload} onPreview={onArtifactPreview ? openArtifactPreview : undefined} onOpenList={onArtifactPreview || onArtifactDownload ? openArtifactList : undefined} /> : null}
        </div>
      </li>
      </Fragment>;
    })}
    {pending ? <li data-role="user" data-status={pending.status} className="wk-chat-message-row wk-chat-message-row--user flex w-full flex-col">
      <div className="wk-chat-message-body flex min-w-0 max-w-full flex-col items-end">
        <div className={`wk-chat-message-bubble ${USER_BUBBLE}`}>
          <p className="mt-[0.3rem] mb-0 [overflow-wrap:anywhere]">{pending.content}</p>
          {pending.status === 'pending' ? <p role="status" className="wk-chat-pending-state mt-[0.3rem] mb-0 [overflow-wrap:anywhere] text-[12px] text-[rgba(0,0,0,0.4)]">{t.sending}</p> : <p role="alert" className="wk-chat-pending-state mt-[0.3rem] mb-0 [overflow-wrap:anywhere] text-[12px] text-[rgba(0,0,0,0.4)]">{pending.error ?? t.sendFailed}</p>}
          {pending.status === 'failed' && onRetry ? <button type="button" className="wk-chat-retry mt-[4px] self-end cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onRetry}>{t.retry}</button> : null}
        </div>
      </div>
    </li> : null}
    {typingIndicator ? <TypingIndicator copy={t} /> : null}
    </ol>
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
        {preview.artifact ? <div className="wk-chat-artifact-drawer-body"><ArtifactPreview artifact={preview.artifact} payload={preview.payload} loading={preview.loading} error={preview.error} copy={t} showHeader={false} onClose={backToArtifactList} /></div> : <ul className="wk-chat-artifact-drawer-list">{previewArtifacts.map((artifact) => {
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
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions mx-0 my-[1rem] w-full max-w-[960px] rounded-[8px] border border-[#dce3ed] p-[0.8rem]" aria-label={t.followUpQuestions}>
      <div className="wk-chat-suggestions-heading flex items-center justify-between gap-[0.6rem]"><h2 className="mt-[0.35rem] mb-[0.35rem] text-[1rem] font-normal text-[rgba(0,0,0,0.4)]">{t.followUpQuestions}</h2><div><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>{t.suggestedRefresh}</button><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onDismissSuggestions}>{t.dismiss}</button></div></div>
      <div className="wk-chat-suggestions-grid mt-[8px] grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-[0.5rem]">{suggestions.questions.map((question) => <button type="button" key={question.id} className="cursor-pointer overflow-hidden text-ellipsis whitespace-nowrap rounded-[6px] border border-[#dce3ed] bg-white p-[0.65rem] text-left text-[13px] leading-[1.5] text-[rgba(0,0,0,0.9)] shadow-[0_1px_2px_rgba(0,0,0,0.04)] hover:border-[rgba(0,0,0,0.1)] hover:shadow-[0_2px_6px_rgba(0,0,0,0.05)]" onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small className="ml-[6px] mt-[0.25rem] block text-[#66758b]">FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}
