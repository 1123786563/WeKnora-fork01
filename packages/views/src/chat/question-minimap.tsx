import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type RefObject } from 'react';
import type { ChatCopyTable } from './chat-copy.ts';

/*
 * px-chat-sandbox 收敛件：会话问答目录（question-minimap）。
 * Vue 事实源（逐块平移）：
 * - frontend/src/utils/chatQuestionMinimap.ts（度量/纯函数，全量照搬）
 * - frontend/src/composables/useChatQuestionMinimap.ts（scroll/resize/RO 观测 → hook）
 * - frontend/src/components/chat/ChatQuestionMinimap.vue（模板 + 交互 + scoped 样式；
 *   样式入 apps/web/src/chat/chat.td.css §18）
 * 挂载位置同 Vue index.vue:143 —— .chat 下、.chat_thread 的紧邻兄弟（绝对定位于
 * .chat 左缘竖直居中）。可见门槛：滚动溢出 && outline 消息数 >= 2（静态 chat 页
 * 不溢出故不渲染；沙箱面板压缩列宽后 Vue 端出现、React 端缺位即 15.765% 根因②）。
 */

/* ===== utils/chatQuestionMinimap.ts 照搬 ===== */

export const QUESTION_TICK_INSET_PX = 8;
export const QUESTION_TICK_GAP_PX = 8;
export const QUESTION_TICK_MOUNTAIN_GAIN = 1.5;
export const CURRENT_TICK_SCALE = 1.7;
export const ACTIVE_QUESTION_TOP_OFFSET_PX = 72;
export const VIEWPORT_BAND_MIN_HEIGHT_PX = 16;
export const QUESTION_MINIMAP_TRACK_MAX_PX = 360;
export const QUESTION_MINIMAP_TRACK_RATIO = 0.5;

export function questionMinimapTrackHeight(questionCount: number, clientHeight: number): number {
  if (questionCount <= 0 || clientHeight <= 0) return 0;
  const maxHeight = Math.min(QUESTION_MINIMAP_TRACK_MAX_PX, clientHeight * QUESTION_MINIMAP_TRACK_RATIO);
  const naturalHeight = questionCount === 1
    ? QUESTION_TICK_INSET_PX * 2
    : (questionCount - 1) * QUESTION_TICK_GAP_PX + QUESTION_TICK_INSET_PX * 2;
  return Math.min(naturalHeight, maxHeight);
}

export interface ChatMessageLike {
  id?: string;
  role?: string;
  content?: string;
  images?: unknown[];
  attachments?: unknown[];
}

export interface OutlineMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  hasAttachments: boolean;
  answerContent: string;
}

export interface QuestionTick {
  id: string;
  yRatio: number;
  yPx: number;
}

export function isChatOverflowing(scrollHeight: number, clientHeight: number): boolean {
  return scrollHeight > clientHeight;
}

export function shouldShowQuestionMinimap(overflowing: boolean, questionCount: number): boolean {
  return overflowing && questionCount >= 2;
}

export function collectOutlineMessages(messages: readonly ChatMessageLike[]): OutlineMessage[] {
  const questions: OutlineMessage[] = [];

  for (let index = 0; index < messages.length; index++) {
    const message = messages[index];
    if ((message.role !== 'user' && message.role !== 'assistant') || !message.id) continue;

    let answerContent = '';
    for (let next = index + 1; message.role === 'user' && next < messages.length; next++) {
      const following = messages[next];
      if (following.role === 'user') break;
      if (following.role === 'assistant') {
        answerContent = following.content ?? '';
        break;
      }
    }

    questions.push({
      id: message.id,
      role: message.role,
      content: message.content ?? '',
      hasAttachments:
        (message.images?.length ?? 0) > 0 || (message.attachments?.length ?? 0) > 0,
      answerContent,
    });
  }

  return questions;
}

/** Include partially visible messages and long answers spanning the viewport. */
export function visibleMessageIds(
  items: Array<{ id: string; offsetTop: number; offsetBottom: number }>,
  scrollTop: number,
  clientHeight: number,
): Set<string> {
  if (clientHeight <= 0) return new Set();
  const bottom = scrollTop + clientHeight;
  return new Set(items.filter(item => (
    item.offsetBottom > item.offsetTop && item.offsetBottom > scrollTop && item.offsetTop < bottom
  )).map(item => item.id));
}

export function questionDisplayText(content: string | undefined, attachmentPlaceholder: string): string {
  const normalized = (content ?? '').replace(/\s+/g, ' ').trim();
  return normalized.length > 0 ? normalized : attachmentPlaceholder;
}

export function answerPreviewText(content: string | undefined): string {
  const normalized = (content ?? '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[#>*_~-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  return normalized;
}

export function tickMountainScale(tickY: number, pointerY: number | null): number {
  if (pointerY === null) return 1;
  const dy = tickY - pointerY;
  const sigma = QUESTION_TICK_GAP_PX;
  return 1 + QUESTION_TICK_MOUNTAIN_GAIN * Math.exp(-0.5 * (dy / sigma) ** 2);
}

export function tickDisplayScale(
  tickY: number,
  pointerY: number | null,
  isCurrent: boolean,
): number {
  if (pointerY !== null) return tickMountainScale(tickY, pointerY);
  return isCurrent ? CURRENT_TICK_SCALE : 1;
}

export function nearestTickId(
  ticks: Array<{ id: string; yPx: number }>,
  pointerY: number,
): string | null {
  if (ticks.length === 0) return null;

  let bestId = ticks[0].id;
  let bestDist = Math.abs(ticks[0].yPx - pointerY);
  for (let index = 1; index < ticks.length; index++) {
    const dist = Math.abs(ticks[index].yPx - pointerY);
    if (dist < bestDist) {
      bestDist = dist;
      bestId = ticks[index].id;
    }
  }
  return bestId;
}

export function offsetFromScrollContent(
  anchorTop: number,
  containerTop: number,
  scrollTop: number,
): number {
  return anchorTop - containerTop + scrollTop;
}

export function mapQuestionTicks(
  items: Array<{ id: string; offsetTop?: number }>,
  trackHeight: number,
): QuestionTick[] {
  const n = items.length;
  if (n === 0 || trackHeight <= 0) {
    return [];
  }

  if (n === 1) {
    return [{
      id: items[0].id,
      yRatio: 0.5,
      yPx: trackHeight / 2,
    }];
  }

  const preferredSpan = (n - 1) * QUESTION_TICK_GAP_PX;
  const minY = Math.min(QUESTION_TICK_INSET_PX, trackHeight / 2);
  const maxY = Math.max(minY, trackHeight - minY);
  const available = maxY - minY;
  const span = preferredSpan <= trackHeight ? preferredSpan : available;
  const start = (trackHeight - span) / 2;

  return items.map((item, index) => {
    const yRatio = index / (n - 1);
    return {
      id: item.id,
      yRatio,
      yPx: start + yRatio * span,
    };
  });
}

export function activeQuestionId(
  items: Array<{ id: string; offsetTop: number }>,
  scrollTop: number,
  topOffsetPx: number = ACTIVE_QUESTION_TOP_OFFSET_PX,
): string | null {
  if (items.length === 0) {
    return null;
  }

  const threshold = scrollTop + topOffsetPx;
  let active: string | null = null;

  for (const item of items) {
    if (item.offsetTop <= threshold) {
      active = item.id;
    }
  }

  return active ?? items[0].id;
}

/* ===== composables/useChatQuestionMinimap.ts → hook ===== */

export interface QuestionMinimapMeasure {
  visible: boolean;
  questions: OutlineMessage[];
  ticks: QuestionTick[];
  activeId: string | null;
  visibleIds: Set<string>;
  anchoredIds: Set<string>;
  trackHeight: number;
}

const EMPTY_MEASURE: QuestionMinimapMeasure = {
  visible: false,
  questions: [],
  ticks: [],
  activeId: null,
  visibleIds: new Set(),
  anchoredIds: new Set(),
  trackHeight: 0,
};

export function useChatQuestionMinimap(
  scrollContainerRef: RefObject<HTMLElement | null>,
  messages: readonly ChatMessageLike[],
): QuestionMinimapMeasure {
  const questions = useMemo(() => collectOutlineMessages(messages), [messages]);
  const [measure, setMeasure] = useState<QuestionMinimapMeasure>(EMPTY_MEASURE);
  const frameId = useRef<number | null>(null);

  const measureNow = useCallback(() => {
    const el = scrollContainerRef.current;
    if (!el) {
      setMeasure({ ...EMPTY_MEASURE, questions });
      return;
    }

    const overflowing = isChatOverflowing(el.scrollHeight, el.clientHeight);
    const anchors = Array.from(el.querySelectorAll<HTMLElement>('[data-message-id]'));
    const anchorByMessageId = new Map<string, HTMLElement>();
    for (const anchor of anchors) {
      const messageId = anchor.dataset.messageId;
      if (messageId) anchorByMessageId.set(messageId, anchor);
    }
    const containerTop = el.getBoundingClientRect().top;
    const measured = questions.flatMap((question) => {
      const anchor = anchorByMessageId.get(question.id);
      if (!anchor) return [];
      const rect = anchor.getBoundingClientRect();
      if (rect.height <= 0) return [];

      return [{
        id: question.id,
        offsetTop: offsetFromScrollContent(rect.top, containerTop, el.scrollTop),
        offsetBottom: offsetFromScrollContent(rect.bottom, containerTop, el.scrollTop),
      }];
    });

    const trackHeight = questionMinimapTrackHeight(measured.length, el.clientHeight);
    setMeasure({
      visible: shouldShowQuestionMinimap(overflowing, questions.length),
      questions,
      ticks: mapQuestionTicks(measured, trackHeight),
      activeId: activeQuestionId(measured, el.scrollTop),
      visibleIds: visibleMessageIds(measured, el.scrollTop, el.clientHeight),
      anchoredIds: new Set(measured.map((item) => item.id)),
      trackHeight,
    });
  }, [questions, scrollContainerRef]);

  const scheduleMeasure = useCallback(() => {
    if (frameId.current !== null) return;
    frameId.current = window.requestAnimationFrame(() => {
      frameId.current = null;
      measureNow();
    });
  }, [measureNow]);

  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el || typeof ResizeObserver === 'undefined') {
      scheduleMeasure();
      return scheduleMeasure;
    }

    el.addEventListener('scroll', scheduleMeasure, { passive: true });
    window.addEventListener('resize', scheduleMeasure);
    const resizeObserver = new ResizeObserver(scheduleMeasure);
    resizeObserver.observe(el);
    const content = el.firstElementChild;
    if (content) resizeObserver.observe(content);
    scheduleMeasure();

    return () => {
      el.removeEventListener('scroll', scheduleMeasure);
      window.removeEventListener('resize', scheduleMeasure);
      resizeObserver.disconnect();
      if (frameId.current !== null) {
        window.cancelAnimationFrame(frameId.current);
        frameId.current = null;
      }
    };
  }, [scheduleMeasure, scrollContainerRef, questions]);

  return measure;
}

/* ===== ChatQuestionMinimap.vue → 组件 ===== */

const CLOSE_DELAY_MS = 150;
const RAIL_INSET_PX = 0;

export interface ChatQuestionMinimapProps {
  copy: Pick<ChatCopyTable, 'questionMinimapAriaLabel' | 'questionMinimapTitle' | 'questionMinimapAttachmentPlaceholder' | 'thinking'>;
  scrollContainerRef: RefObject<HTMLElement | null>;
  messages: readonly ChatMessageLike[];
  onJump(messageId: string): void;
}

export function ChatQuestionMinimap(props: ChatQuestionMinimapProps) {
  const { copy } = props;
  const rootRef = useRef<HTMLElement | null>(null);
  const [hoverOpen, setHoverOpen] = useState(false);
  const [pinnedOpen, setPinnedOpen] = useState(false);
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [pointerY, setPointerY] = useState<number | null>(null);
  const [keyboardIndex, setKeyboardIndex] = useState(-1);
  const [isCoarsePointer, setIsCoarsePointer] = useState(false);
  const closeTimer = useRef<number | null>(null);

  const { visible, questions, ticks, activeId, visibleIds, anchoredIds, trackHeight } =
    useChatQuestionMinimap(props.scrollContainerRef, props.messages);

  const panelOpen = hoverOpen || pinnedOpen;
  const anchoredQuestions = questions.filter((question) => anchoredIds.has(question.id));
  const peakId = keyboardIndex >= 0
    ? questions[keyboardIndex]?.id ?? null
    : hoveredId ?? activeId;
  const peakTurn = questions.find((question) => question.id === peakId) ?? null;
  const highlightedIds = useMemo(() => {
    const ids = new Set(visibleIds);
    if ((hoveredId || keyboardIndex >= 0) && peakId) ids.add(peakId);
    return ids;
  }, [visibleIds, hoveredId, keyboardIndex, peakId]);
  const peakYPx = ticks.find((item) => item.id === peakId)?.yPx ?? 0;
  const mountainPointerY = keyboardIndex >= 0 ? peakYPx : pointerY;

  const questionText = (question: OutlineMessage) => (
    question.role === 'assistant'
      ? answerPreviewText(question.content) || copy.thinking
      : questionDisplayText(question.content, copy.questionMinimapAttachmentPlaceholder)
  );
  const answerText = (question: OutlineMessage) => answerPreviewText(question.answerContent);

  const clearCloseTimer = () => {
    if (closeTimer.current === null) return;
    window.clearTimeout(closeTimer.current);
    closeTimer.current = null;
  };

  const closePanel = () => {
    clearCloseTimer();
    setHoverOpen(false);
    setPinnedOpen(false);
    setHoveredId(null);
    setPointerY(null);
    setKeyboardIndex(-1);
  };

  const scheduleClose = () => {
    clearCloseTimer();
    closeTimer.current = window.setTimeout(() => {
      setHoverOpen(false);
      setHoveredId(null);
      setPointerY(null);
      closeTimer.current = null;
    }, CLOSE_DELAY_MS);
  };

  const handleQuestionClick = (id: string) => {
    if (!anchoredIds.has(id)) return;
    closePanel();
    props.onJump(id);
  };

  const handleRailClick = () => {
    if (!isCoarsePointer) {
      if (peakId) handleQuestionClick(peakId);
      return;
    }
    clearCloseTimer();
    setPinnedOpen((open) => !open);
    setHoverOpen(false);
    if (!pinnedOpen) {
      setHoveredId(activeId ?? questions[0]?.id ?? null);
    }
  };

  const handleRailPointerMove = (event: React.PointerEvent<HTMLButtonElement>) => {
    if (isCoarsePointer) return;
    const target = event.currentTarget;
    const nextPointerY = event.clientY - target.getBoundingClientRect().top;
    setPointerY(nextPointerY);
    setHoveredId(nearestTickId(ticks, nextPointerY));
  };

  const syncKeyboardIndexToActive = () => {
    const index = questions.findIndex((question) => question.id === activeId);
    setKeyboardIndex(index >= 0 && anchoredIds.has(questions[index].id) ? index : -1);
  };

  const moveKeyboard = (direction: 1 | -1) => {
    const anchored = anchoredQuestions;
    if (anchored.length === 0) return;

    const wasOpen = panelOpen;
    clearCloseTimer();
    setHoverOpen(true);
    if (!wasOpen) {
      syncKeyboardIndexToActive();
    }
    const currentQuestion = questions[keyboardIndex];
    const currentAnchoredIndex = currentQuestion
      ? anchored.findIndex((question) => question.id === currentQuestion.id)
      : -1;
    const activeAnchoredIndex = anchored.findIndex((question) => question.id === activeId);
    const startIndex = currentAnchoredIndex >= 0 ? currentAnchoredIndex : activeAnchoredIndex;
    const nextAnchoredIndex = Math.min(
      anchored.length - 1,
      Math.max(0, (startIndex >= 0 ? startIndex : 0) + direction),
    );
    const nextQuestion = anchored[nextAnchoredIndex];
    setKeyboardIndex(questions.findIndex((question) => question.id === nextQuestion.id));
  };

  const handleRailKeydown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowRight') {
      event.preventDefault();
      moveKeyboard(1);
      return;
    }
    if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') {
      event.preventDefault();
      moveKeyboard(-1);
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const question = peakTurn;
      if (question && anchoredIds.has(question.id)) {
        closePanel();
        props.onJump(question.id);
      }
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      closePanel();
    }
  };

  const handleDocumentPointerDown = (event: PointerEvent) => {
    if (!isCoarsePointer || !panelOpen) return;
    const target = event.target as Node | null;
    if (target && rootRef.current?.contains(target)) return;
    closePanel();
  };

  useEffect(() => {
    setIsCoarsePointer(window.matchMedia('(pointer: coarse)').matches);
    document.addEventListener('pointerdown', handleDocumentPointerDown);
    return () => {
      clearCloseTimer();
      document.removeEventListener('pointerdown', handleDocumentPointerDown);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!visible) return null;

  return (
    <nav
      ref={rootRef}
      className="question-minimap"
      style={{ left: `${RAIL_INSET_PX}px`, height: `${trackHeight}px` } as CSSProperties}
      aria-label={copy.questionMinimapAriaLabel}
      onMouseEnter={() => {
        if (isCoarsePointer) return;
        clearCloseTimer();
        setHoverOpen(true);
        setKeyboardIndex(-1);
      }}
      onMouseLeave={() => {
        if (isCoarsePointer) return;
        scheduleClose();
      }}
    >
      <button
        className="question-minimap__rail"
        type="button"
        aria-label={copy.questionMinimapAriaLabel}
        aria-haspopup="dialog"
        aria-expanded={panelOpen}
        onClick={handleRailClick}
        onKeyDown={handleRailKeydown}
        onPointerMove={handleRailPointerMove}
      >
        {ticks.map((tick) => (
          <span
            key={tick.id}
            className={'question-minimap__tick' + (highlightedIds.has(tick.id) ? ' question-minimap__tick--active' : '')}
            style={{
              top: `${tick.yPx}px`,
              transform: `translateY(-50%) scaleX(${tickDisplayScale(tick.yPx, mountainPointerY, highlightedIds.has(tick.id))})`,
            }}
          />
        ))}
      </button>
      {panelOpen ? <div className="question-minimap__bridge" aria-hidden="true" /> : null}
      {panelOpen && peakTurn ? (
        <section
          className="question-minimap__panel"
          role="dialog"
          aria-label={copy.questionMinimapTitle}
          style={{ top: `${peakYPx}px` }}
          onClick={() => handleQuestionClick(peakTurn.id)}
        >
          <p className="question-minimap__question" title={questionText(peakTurn)}>
            {questionText(peakTurn)}
          </p>
          {answerText(peakTurn) ? <p className="question-minimap__answer">{answerText(peakTurn)}</p> : null}
        </section>
      ) : null}
    </nav>
  );
}
