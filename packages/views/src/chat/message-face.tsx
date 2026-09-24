import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ChatMessage, FeedbackRating } from '@weknora/contracts';
import { copyAnswerText } from '@weknora/domain/chat/copy-answer';
import { groupChatReferences } from '@weknora/domain/chat/references';
import { conversationTimeLabels, type ChatCopyTable } from './chat-copy.ts';
import { formatConversationTimestampLabel } from '@weknora/domain/chat/message-timestamps';
import { renderChatMarkdown } from './markdown.ts';

/*
 * Vue 事实源（frontend/src/views/chat/）——消息面 DOM 1:1：
 * - MessageTimestamp.vue → time.conversation-time
 * - usermsg.vue → .user_msg_container > .mentioned_items + .user_msg
 * - botmsg.vue → .bot_msg > div(flex col gap 8) > .rag-answer-stack
 *   > .rag-pipeline-progress（完成折叠根）+ .agent-stream-display.is-rag-mode
 *   > .streaming-steps-container > .event-item.event-answer > .answer-event
 *   > .answer-content.markdown-content + .answer-toolbar（t-button×N）
 * 消息标志推导对齐 Vue useChatStreamHandler.handleMsgList /
 * restoreQuickAnswerFlags / ensureRagPipelineHistoryStream：
 * - isAgentMode ⇔ agent_steps 非空（或完成消息经 rag 历史恢复）；
 * - 快问答模式下完成消息 isRagMode=true，答案经 agent answer 事件渲染。
 *
 * 本包不带 tdesign 依赖（packages/views/package.json 由并行流共享）：t-icon 与
 * t-button 以同构手写 DOM 输出（svg.t-icon>use 命中本地 sprite；t-button 类族
 * 结构与 tdesign-react 1.18.3 Button 渲染一致，几何由 chat.td.css
 * .answer-toolbar .t-button 规一），弹层（ChatHeader 菜单）由宿主经
 * page.tsx headerSlot 注入真实 tdesign 组件。
 */

/** tdesign Icon 同构输出（sprite <use>，与 tdesign-icons-react 同路径，台账 #10）。 */
function SpriteIcon(props: { name: string; className?: string }) {
  return (
    <svg className={'t-icon t-icon-' + props.name + (props.className ? ' ' + props.className : '')} viewBox="0 0 24 24" width="1em" height="1em" fill="none" aria-hidden="true">
      <use href={'#t-icon-' + props.name} />
    </svg>
  );
}

/** tdesign Button（small/outline/round、icon-only）同构输出。 */
function ToolbarButton(props: { icon: string; title: string; className?: string; disabled?: boolean; onClick?(): void }) {
  return (
    <button
      type="button"
      className={'t-button t-button--theme-default t-button--variant-outline t-button--shape-round t-size-s' + (props.className ? ' ' + props.className : '')}
      title={props.title}
      aria-label={props.title}
      disabled={props.disabled}
      aria-disabled={props.disabled ? 'true' : undefined}
      onClick={props.disabled ? undefined : props.onClick}
    >
      <span className="t-button__text">
        <SpriteIcon name={props.icon} />
      </span>
    </button>
  );
}

/** Vue usermsg.mentionTagClass/Icon（tag/mcp/skill/file 四色 chip）。 */
function mentionTagClass(item: { type: string; kb_type?: string }): string {
  if (item.type === 'kb') return item.kb_type === 'faq' ? 'faq-tag' : 'kb-tag';
  return `${item.type || 'file'}-tag`;
}

function mentionTagIcon(item: { type: string }): string {
  if (item.type === 'tag') return 'tag';
  if (item.type === 'mcp') return 'tools';
  if (item.type === 'skill') return 'system-code';
  return 'file';
}

interface MentionedItemView { id: string; name: string; type: string; kb_type?: string }

function mentionedItemsOf(message: ChatMessage): MentionedItemView[] {
  const raw = message.mentioned_items;
  if (!Array.isArray(raw)) return [];
  return raw.filter((item): item is MentionedItemView =>
    typeof item === 'object' && item !== null
    && typeof (item as MentionedItemView).id === 'string'
    && typeof (item as MentionedItemView).name === 'string');
}

/** Vue .mentioned_items + .mentioned_tag（chat-resource-chips.less）。 */
export function MentionedItemsTags(props: { message: ChatMessage; justify?: 'flex-start' | 'flex-end' }) {
  const items = mentionedItemsOf(props.message);
  if (items.length === 0) return null;
  return (
    <div className="wk-chat-mentioned-items mentioned_items" style={{ justifyContent: props.justify ?? 'flex-start' }}>
      {items.map((item) => (
        <span key={item.id} className={'wk-chat-mentioned-tag mentioned_tag ' + mentionTagClass(item)}>
          <span className="tag_icon">
            {item.type === 'kb'
              ? <SpriteIcon name={item.kb_type === 'faq' ? 'chat-bubble-help' : 'folder'} />
              : <SpriteIcon name={mentionTagIcon(item)} />}
          </span>
          <span className="tag_name">{item.name}</span>
        </span>
      ))}
    </div>
  );
}

/** Vue MessageTimestamp.vue（time.conversation-time，label 由 domain 格式化对齐）。 */
export function ConversationTimestamp(props: { copy: ChatCopyTable; value: unknown }) {
  const label = formatConversationTimestampLabel(props.value, conversationTimeLabels(props.copy));
  const datetime = typeof props.value === 'string' ? props.value : '';
  if (!label) return null;
  return <time className="wk-chat-timestamp conversation-time" dateTime={datetime}>{label}</time>;
}

/** Vue usermsg.vue（@提及 chip 行 + 图片/附件 + 右对齐气泡）。 */
export function UserMessageFace(props: {
  copy: ChatCopyTable;
  message: ChatMessage;
  onRetrySteer?(steerId: string): void;
  onRemoveSteer?(steerId: string): void;
}) {
  const message = props.message;
  const row = message as Record<string, unknown>;
  const images = Array.isArray(row.images) ? row.images as { url?: string }[] : [];
  const attachments = Array.isArray(row.attachments) ? row.attachments as { id?: string; file_name?: string; file_size?: number }[] : [];
  const steerFailed = row._steerFailed === true;
  const steerId = typeof message.steer_id === 'string' ? message.steer_id : undefined;
  return (
    <div className="user_msg_container wk-chat-user-message">
      <MentionedItemsTags message={message} justify="flex-end" />
      {images.length > 0 ? (
        <div className="user_images">
          {images.map((img, idx) => <img key={idx} src={img.url} className="user_image_thumb" alt="" />)}
        </div>
      ) : null}
      {attachments.length > 0 ? (
        <div className="user_attachments">
          {attachments.map((att, idx) => (
            <div key={idx} className="user_attachment_card">
              <div className="attachment_card_icon">
                <svg viewBox="0 0 40 48" fill="none" xmlns="http://www.w3.org/2000/svg" width="36" height="44" aria-hidden="true">
                  <rect width="40" height="48" rx="4" fill="#4A90D9" />
                  <path d="M8 6h16l8 8v28a2 2 0 01-2 2H8a2 2 0 01-2-2V8a2 2 0 012-2z" fill="#5BA3E8" />
                  <path d="M24 6l8 8h-6a2 2 0 01-2-2V6z" fill="#3A7BC8" />
                  <rect x="10" y="20" width="20" height="2" rx="1" fill="white" fillOpacity="0.9" />
                  <rect x="10" y="26" width="20" height="2" rx="1" fill="white" fillOpacity="0.9" />
                  <rect x="10" y="32" width="14" height="2" rx="1" fill="white" fillOpacity="0.9" />
                </svg>
              </div>
              <div className="attachment_card_info">
                <div className="attachment_card_name">{att.file_name}</div>
                <div className="attachment_card_meta">
                  {fileExt(att.file_name)}
                  {att.file_size ? <>&nbsp;·&nbsp;{formatFileSize(att.file_size)}</> : null}
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : null}
      <div className="user_msg wk-chat-message-bubble">{message.content}</div>
      {steerFailed ? (
        <div className="steer-failure" role="status">
          <span>{props.copy.steerFailed}</span>
          <button type="button" aria-label={props.copy.steerRetry} onClick={() => steerId && props.onRetrySteer?.(steerId)}><SpriteIcon name="refresh" /></button>
          <button type="button" aria-label={props.copy.remove} onClick={() => steerId && props.onRemoveSteer?.(steerId)}><SpriteIcon name="close" /></button>
        </div>
      ) : null}
    </div>
  );
}

function fileExt(fileName?: string): string {
  return (fileName || '').split('.').pop()?.toUpperCase() || 'FILE';
}

function formatFileSize(bytes?: number): string {
  if (!bytes) return '';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

/**
 * Vue RagPipelineProgress.vue 完成折叠根（history 稳态）：
 * .rag-pipeline-progress > .sr-only + .tree-container > .tool-event >
 * .action-card.tree-root > .tree-root-toolbar > button.tree-root-expand
 * （状态文本「检索完成」+ 引用计数「引用了N篇文档」+ chevron）。
 */
export function RagPipelineProgressFace(props: {
  copy: ChatCopyTable;
  message: ChatMessage;
  liveStatusText: string;
  /** Vue toggleExpanded：折叠根切换共享引用抽屉（旧 React 行为对齐）。缺省只读。 */
  onToggle?(): void;
  /** 引用抽屉当前开合态（aria-expanded + chevron 方向）。 */
  referencesOpen?: boolean;
}) {
  const row = props.message as Record<string, unknown>;
  const refs = [...(Array.isArray(row.knowledge_references) ? row.knowledge_references as unknown[] : []),
    ...(Array.isArray(row.references) ? row.references as unknown[] : [])];
  const groups = groupChatReferences(refs);
  const docCount = groups.flatMap((group) => (group.kind === 'document' ? group.items : [])).length;
  const webCount = groups.flatMap((group) => (group.kind === 'web' ? group.items : [])).length;
  if (docCount === 0 && webCount === 0) return null;
  // Vue RagPipelineProgress.vue:502-515 三分支：doc+web 组合文案 / doc / web。
  const referenceText = docCount > 0 && webCount > 0
    ? props.copy.referencesDocAndWebCount.replace('{docCount}', String(docCount)).replace('{webCount}', String(webCount))
    : docCount > 0
      ? props.copy.referencesDocCount.replace('{count}', String(docCount))
      : props.copy.referencesWebCount.replace('{count}', String(webCount));
  return (
    <div className="rag-pipeline-progress">
      <div className="sr-only" role="status" aria-live="polite">{props.liveStatusText}</div>
      <div className="tree-container">
        <div className="tool-event">
          <div className="action-card tree-root">
            <div className="tree-root-toolbar">
              <button
                type="button"
                className="tree-root-expand"
                aria-label={props.copy.searchDone}
                aria-expanded={props.referencesOpen === true}
                onClick={props.onToggle}
              >
                <span className="tree-root-status">{props.copy.searchDone}</span>
                <span className="tree-root-reference">{referenceText}</span>
                <SpriteIcon name={props.referencesOpen === true ? 'chevron-down' : 'chevron-right'} className="tree-root-expand__icon" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Vue ChatRequestInfoButton.vue（info-circle t-button + t-popup placement=top 带箭头弹层）。 */
function RequestInfoButton(props: { copy: ChatCopyTable; message: ChatMessage; sessionId: string | null }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLSpanElement | null>(null);
  const row = props.message as Record<string, unknown>;
  const requestId = typeof row.request_id === 'string' ? row.request_id : '';
  // Vue rows（ChatRequestInfoButton.vue computed）：requestId/messageId/sessionId +
  // method+url、sentAt —— 后两者仅在 debugRequest（实时调试负载）存在时展示；
  // 历史消息无 debug 负载，React 不从 created_at 伪造 sentAt 行。
  const rows = [
    { label: 'Request ID', value: requestId },
    { label: props.copy.requestInfoMessageId, value: props.message.id },
    { label: props.copy.requestInfoSessionId, value: props.sessionId ?? '' },
  ].filter((item) => item.value);
  useEffect(() => {
    if (!open) return;
    const onDocClick = (event: MouseEvent) => {
      if (rootRef.current && event.target instanceof Node && !rootRef.current.contains(event.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);
  const copyAll = () => {
    const text = rows.map((item) => `${item.label}: ${item.value}`).join('\n');
    void navigator.clipboard?.writeText(text).catch(() => undefined);
  };
  const toggle = () => setOpen((value) => !value);
  // Vue t-popup 经 popper 落位（偏移取整到设备像素）；纯 CSS 的 left:50% +
  // translateX(-50%) / bottom:100% 会停在分数坐标（卡片宽 342.42、React 工具栏
  // 行高分数 → 204.79/339.78 vs Vue 205/340），全卡文字 AA 随之错位。开层后一次
  // 性测量取整锚定（偏移相对触发钮 wrap，布局回流仍自动跟随；宽度/行高由内容
  // 决定、开层后不变）：水平按"卡片中心=按钮中心"取整、垂直按"wrap 底缘=按钮
  // 顶缘"取整。
  const [anchor, setAnchor] = useState<{ left: number; bottom: number } | null>(null);
  useLayoutEffect(() => {
    if (!open) return;
    const span = rootRef.current;
    const card = span?.querySelector('.chat-request-card--popover');
    if (span && card) {
      // 视口坐标 fixed 锚定：消息列 .msg-item-wrapper 有 overflow 裁剪（contain:
      // layout style），absolute 弹层左缘越出消息列左界会被裁掉 56px——fixed 的
      // CB 是视口，不被中间裁剪祖先截断（同 composer mention-menu 先例）。
      // 偏移取整到设备像素（Vue popper 行为），否则全卡文字 AA 错位。
      const sr = span.getBoundingClientRect();
      const cw = card.getBoundingClientRect().width;
      setAnchor({
        left: Math.round(sr.left + sr.width / 2 - cw / 2),
        bottom: Math.round(window.innerHeight - sr.top),
      });
    }
  }, [open]);
  return (
    <span className="chat-request-info-wrap" ref={rootRef} style={{ position: 'relative', display: 'inline-flex' }}>
      <ToolbarButton icon="info-circle" title={props.copy.requestInfo} onClick={toggle} />
      {open ? (
        <div className="chat-request-popover-wrap" style={anchor === null ? undefined : { left: `${anchor.left}px`, bottom: `${anchor.bottom}px`, transform: 'none' }}>
          <div className="chat-request-card chat-request-card--popover" role="dialog" aria-label={props.copy.requestInfo}>
            <div className="chat-request-card-header">
              <span className="chat-request-card-title">{props.copy.requestInfo}</span>
              {rows.length > 0 ? (
                <button type="button" className="chat-request-card-copy" title={props.copy.copy} aria-label={props.copy.copy} onClick={copyAll}>
                  <SpriteIcon name="copy" />
                </button>
              ) : null}
            </div>
            {rows.length === 0 ? (
              <div className="chat-request-empty">{props.copy.requestInfoEmpty}</div>
            ) : (
              <div className="chat-request-card-body">
                {rows.map((item) => (
                  <div key={item.label} className="chat-request-row">
                    <span className="chat-request-label">{item.label}</span>
                    <span className="chat-request-value">{item.value}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
          <span className="chat-request-popover-arrow" aria-hidden="true" />
        </div>
      ) : null}
    </span>
  );
}

async function writeClipboardText(text: string): Promise<void> {
  const target = typeof navigator !== 'undefined' ? navigator.clipboard : undefined;
  if (target?.writeText) { await target.writeText(text); return; }
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

export interface AnswerToolbarProps {
  copy: ChatCopyTable;
  message: ChatMessage;
  sessionId: string | null;
  /** Vue answerFullyRendered 门（完成后才渲染工具条）。 */
  rendered: boolean;
  onBookmark?(messageId: string): void | Promise<void>;
  onRateMessage?(messageId: string, rating: FeedbackRating): void;
  onRemoveRating?(messageId: string): void;
  ratingOf?(messageId: string): FeedbackRating | undefined;
  /** Vue showRequestInfo：request_id/id 存在即渲染 ⓘ。 */
  showRequestInfo: boolean;
}

/** Vue AgentStreamDisplay answer-toolbar + botmsg answer-toolbar（chat-message-shared.less）。 */
export function AnswerToolbar(props: AnswerToolbarProps) {
  const { copy } = props;
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current); }, []);
  if (!props.rendered) return null;
  const text = copyAnswerText(props.message.content);
  async function copyAnswer() {
    if (!text) return;
    try {
      await writeClipboardText(text);
      setCopied(true);
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), 2000);
    } catch { /* clipboard denied */ }
  }
  const row = props.message as Record<string, unknown>;
  return (
    <div className="answer-toolbar wk-chat-answer-toolbar">
      <ToolbarButton icon="copy" title={copied ? copy.copied : copy.copy} className="wk-chat-copy" onClick={() => void copyAnswer()} />
      <ToolbarButton icon="bookmark-add" title={copy.addToKnowledgeBase} className="wk-chat-bookmark" disabled={!props.onBookmark} onClick={() => props.onBookmark?.(props.message.id)} />
      {row.is_fallback === true ? (
        <ToolbarButton icon="info-circle" title={copy.fallbackHint} className="fallback-icon-btn" />
      ) : null}
      {props.showRequestInfo ? <RequestInfoButton copy={copy} message={props.message} sessionId={props.sessionId} /> : null}
    </div>
  );
}

/**
 * Vue AgentStreamDisplay 的 rag 模式 answer 事件面（完成态）：
 * .agent-stream-display.is-rag-mode > .streaming-steps-container >
 * .event-item.event-answer > div > .answer-event > .answer-content + .answer-toolbar。
 */
export function AgentStreamAnswerFace(props: {
  copy: ChatCopyTable;
  message: ChatMessage;
  content: string;
  sessionId: string | null;
  onBookmark?(messageId: string): void | Promise<void>;
  onCitationClick?(citationId: string): void;
}) {
  const row = props.message as Record<string, unknown>;
  const showRequestInfo = Boolean(row.request_id || props.message.id);
  const html = renderChatMarkdown(props.content, {});
  return (
    <div className="agent-stream-display is-rag-mode">
      <div className="streaming-steps-container">
        <div className="event-item event-answer">
          <div>
            <div className="answer-event">
              <div
                className="answer-content markdown-content wk-chat-message-content"
                onClick={(event) => {
                  const target = event.target;
                  if (!(target instanceof HTMLElement)) return;
                  const citation = target.closest<HTMLElement>('[data-citation-id]')?.dataset.citationId;
                  if (citation) props.onCitationClick?.(citation);
                }}
                dangerouslySetInnerHTML={{ __html: html }}
              />
              <AnswerToolbar
                copy={props.copy}
                message={props.message}
                sessionId={props.sessionId}
                rendered={props.message.is_completed === true}
                onBookmark={props.onBookmark}
                showRequestInfo={showRequestInfo}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Vue botmsg.vue 完成消息面：.bot_msg > div(flex col gap 8) > .rag-answer-stack。
 * 快问答/Agent 历史消息统一走 rag-answer-stack（restoreQuickAnswerFlags +
 * ensureRagPipelineHistoryStream 恢复后的稳态）。
 */
export function BotMessageFace(props: {
  copy: ChatCopyTable;
  message: ChatMessage;
  sessionId: string | null;
  /** history think 块剥离后的答案正文（Vue handleMsgList 切分）。 */
  content: string;
  onBookmark?(messageId: string): void | Promise<void>;
  onCitationClick?(citationId: string): void;
  /** Vue ChatReferencesDrawer 入口：折叠根「检索完成」切换共享引用面板。 */
  onToggleReferences?(): void;
  /** 共享引用面板当前开合态（折叠根 aria-expanded + chevron 方向）。 */
  referencesOpen?: boolean;
}) {
  const liveStatus = props.copy.searchDone;
  return (
    <div className="bot_msg wk-chat-bot-message">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        <div className="rag-answer-stack">
          <RagPipelineProgressFace
            copy={props.copy}
            message={props.message}
            liveStatusText={liveStatus}
            onToggle={props.onToggleReferences}
            referencesOpen={props.referencesOpen}
          />
          <AgentStreamAnswerFace
            copy={props.copy}
            message={props.message}
            content={props.content}
            sessionId={props.sessionId}
            onBookmark={props.onBookmark}
            onCitationClick={props.onCitationClick}
          />
        </div>
      </div>
    </div>
  );
}
