// React port of the WeKnora contextual guides.
//
// Vue baseline (authoritative, read-only):
//   frontend/src/components/SpotlightGuide.vue          — spotlight card renderer
//   frontend/src/components/ContextualGuide.vue         — when→open scheduling,
//                                                         global-guide gating,
//                                                         dismissal persistence
//   frontend/src/components/KbCreateContextualGuide.vue / AgentCreateContextualGuide.vue /
//   TenantModelsGuide.vue                               — per-tour step assembly
//
// The spotlight mechanics reuse the shared primitives in guides/geometry.ts
// (hole, backdrop pieces, card placement — same math as SpotlightGuide.vue).
// Vue mounts one wrapper per page; the React client keeps ONE host (mounted by
// the platform shell) fed by openContextualGuide(tour) trigger calls, because
// most React route changes reload the document (see contextual-guides.ts for
// the sessionStorage hand-off).
import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react';
import {
  CONTEXTUAL_GUIDE_OPEN_DELAY_MS,
  OPEN_CONTEXTUAL_GUIDE_EVENT,
  contextualGuideMessage,
  contextualGuideStepPrefix,
  consumePendingContextualGuide,
  isContextualGuideDone,
  isGlobalUserGuideDone,
  markContextualGuideDone,
  focusAgentEditorSection,
  focusKbEditorSection,
  resolveContextualGuideSteps,
  shouldOpenContextualGuide,
  type ContextualGuideOpenDetail,
  type ContextualGuideStep,
  type ContextualGuideTourId,
  type ContextualGuideTriggerOptions,
} from './contextual-guides.ts';
import type { KeyValueStorage } from './new-user-guide.ts';
import type { NewUserGuideLocale } from './steps.ts';
import {
  computeBackdropPieces,
  computeCardStyle,
  computeHighlightHole,
  GUIDE_BACKDROP_COLOR,
  GUIDE_CARD_WIDTH,
  GUIDE_HOLE_RADIUS,
  type GuideHoleRect,
} from './geometry.ts';

export {
  computeBackdropPieces,
  computeCardStyle,
  computeHighlightHole,
  GUIDE_BACKDROP_COLOR,
  GUIDE_CARD_WIDTH,
  GUIDE_HOLE_RADIUS,
};

/** Vue SpotlightGuide.vue hardcodes 12 retries × 120ms (lines 289-290). */
const GUIDE_LOCATE_RETRIES = 12;

/** Shell callbacks replacing the Vue uiStore side effects. All optional. */
export interface ContextualGuideActions {
  openModelsSettings?: () => void;
  closeGuideSettings?: () => void;
}

function queryTarget(selector: string, root: ParentNode): Element | null {
  for (const part of selector.split(',').map((s) => s.trim()).filter(Boolean)) {
    const el = root.querySelector(part);
    if (!el) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width > 2 && rect.height > 2) return el;
  }
  return null;
}

function elementViewport(): { getComputedStyle: (el: unknown) => { marginBottom: string } } {
  return { getComputedStyle: (el: unknown) => window.getComputedStyle(el as Element) };
}

export interface ContextualGuideProps {
  tour: ContextualGuideTourId;
  locale: NewUserGuideLocale;
  /** Vue wrapper props (isFaq / needsEmbedding / isAgentMode / variant). */
  options?: ContextualGuideTriggerOptions;
  /** Controlled by the host; the Vue wrappers drive the same flag internally. */
  open: boolean;
  /** finish/skip/Esc — the host persists the dismissal (Vue onFinish). */
  onDismiss: () => void;
  storage?: KeyValueStorage;
  actions?: ContextualGuideActions;
  /** Vue SpotlightGuide beforeDelayMs default (line 77). */
  beforeDelayMs?: number;
  /** Vue hardcodes 120ms between target-locate retries (line 290). */
  locateRetryDelayMs?: number;
}

/**
 * Spotlight renderer for one contextual tour. Controlled-open: the host (or a
 * test) decides visibility; persistence lives in the host so both dismissal
 * paths (finish and skip) cascade alsoCompleteTours exactly once.
 */
export function ContextualGuide({
  tour,
  locale,
  options,
  open,
  onDismiss,
  storage,
  actions,
  beforeDelayMs = 280,
  locateRetryDelayMs = 120,
}: ContextualGuideProps): ReactNode {
  const steps = useMemo(() => resolveContextualGuideSteps(tour, options ?? {}), [tour, options]);
  const prefix = useMemo(() => contextualGuideStepPrefix(tour, options ?? {}), [tour, options]);

  const [index, setIndex] = useState(0);
  const [vw, setVw] = useState(0);
  const [vh, setVh] = useState(0);
  const [hole, setHole] = useState<GuideHoleRect | null>(null);
  const [cardSize, setCardSize] = useState({ width: GUIDE_CARD_WIDTH, height: 220 });

  const rootRef = useRef<HTMLDivElement | null>(null);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const actionsRef = useRef(actions);
  actionsRef.current = actions;
  const settingsOpenedByGuideRef = useRef(false);

  // Vue re-opens at step 0 whenever the wrapper activates (open(), line 356).
  useEffect(() => {
    if (open) setIndex(0);
  }, [open, tour]);

  const clearRetryTimer = () => {
    if (retryTimerRef.current !== null) {
      window.clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  };

  const measureCard = async () => {
    // Vue awaits nextTick so the card reflects the new step content first.
    await new Promise((resolve) => window.setTimeout(resolve, 0));
    const el = cardRef.current;
    if (el) setCardSize({ width: el.offsetWidth, height: el.offsetHeight });
  };

  const goTo = (nextIndex: number) => {
    if (nextIndex < 0 || nextIndex >= steps.length) return;
    clearRetryTimer();
    setIndex(nextIndex);
  };

  const locate = (stepIndex: number, retry: number): void => {
    setVw(window.innerWidth);
    setVh(window.innerHeight);
    const step = steps[stepIndex];
    if (!step || !step.target) {
      setHole(null);
      void measureCard();
      return;
    }
    const el = queryTarget(step.target, document);
    if (!el) {
      if (retry < GUIDE_LOCATE_RETRIES) {
        clearRetryTimer();
        retryTimerRef.current = window.setTimeout(() => locate(stepIndex, retry + 1), locateRetryDelayMs);
        return;
      }
      if (step.optional) {
        goTo(stepIndex + 1);
        return;
      }
      setHole(null);
      void measureCard();
      return;
    }
    if (typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'smooth' });
    }
    const rect = el.getBoundingClientRect();
    setHole(computeHighlightHole(el, rect, elementViewport(), window.innerWidth, window.innerHeight));
    void measureCard();
  };

  // Vue TenantModelsGuide onStepChange (lines 64-68): leaving the addModel
  // step closes the settings section the guide opened.
  const closeGuideSettings = () => {
    if (settingsOpenedByGuideRef.current) {
      actionsRef.current?.closeGuideSettings?.();
      settingsOpenedByGuideRef.current = false;
    }
  };

  // Step driver — Vue SpotlightGuide goTo() (lines 309-326): run the step's
  // before-hook, honor the before delay, then locate and focus the card.
  useEffect(() => {
    if (!open) return;
    clearRetryTimer();
    const step = steps[index];
    if (!step) return;
    if (step.action?.kind !== 'open-models-settings') closeGuideSettings();
    let cancelled = false;
    void (async () => {
      if (step.action?.kind === 'focus-kb-editor-section' || step.action?.kind === 'focus-agent-editor-section') {
        // Vue before callbacks dispatch the editor focus-section events; the
        // same window events stay the integration point for React editors.
        if (step.action.kind === 'focus-kb-editor-section') focusKbEditorSection(step.action.section);
        else focusAgentEditorSection(step.action.section);
      } else if (step.action?.kind === 'open-models-settings') {
        actionsRef.current?.openModelsSettings?.();
        settingsOpenedByGuideRef.current = true;
      }
      if (step.action && beforeDelayMs > 0) {
        await new Promise((resolve) => window.setTimeout(resolve, beforeDelayMs));
      }
      if (cancelled) return;
      locate(index, 0);
      rootRef.current?.focus();
    })();
    return () => {
      cancelled = true;
      clearRetryTimer();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, index, tour, beforeDelayMs, locateRetryDelayMs]);

  // Vue resize/scroll listeners while the guide is open (lines 351-395).
  useEffect(() => {
    if (!open) return;
    const onViewportChange = () => locate(index, 0);
    window.addEventListener('resize', onViewportChange);
    window.addEventListener('scroll', onViewportChange, true);
    return () => {
      window.removeEventListener('resize', onViewportChange);
      window.removeEventListener('scroll', onViewportChange, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, index]);

  // Unmount / close: the Vue wrappers clear the guide-opened settings too.
  useEffect(() => {
    if (open) return;
    closeGuideSettings();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => () => clearRetryTimer(), []);

  if (!open) return null;

  const step = steps[index] ?? steps[0];
  if (!step) return null;
  const isLast = index === steps.length - 1;
  const skipLabel = contextualGuideMessage(locale, 'contextualGuide.skip');
  const title = contextualGuideMessage(locale, `${prefix}.${step.key}.title`);
  const desc = contextualGuideMessage(locale, `${prefix}.${step.key}.desc`);
  const cardStyle = computeCardStyle(vw, vh, hole, cardSize, step.placement);
  const holeFrameStyle: CSSProperties = hole
    ? {
        left: String(hole.x) + 'px',
        top: String(hole.y) + 'px',
        width: String(hole.width) + 'px',
        height: String(hole.height) + 'px',
        borderRadius: String(GUIDE_HOLE_RADIUS) + 'px',
      }
    : {};

  return (
    <div
      className="wk-guide"
      role="dialog"
      aria-modal="true"
      aria-label={title}
      tabIndex={-1}
      ref={rootRef}
      data-testid="wk-contextual-guide"
      data-guide-tour={tour}
      onKeyDown={(event: ReactKeyboardEvent<HTMLDivElement>) => {
        if (event.key === 'Escape') {
          event.preventDefault();
          onDismiss();
        } else if (event.key === 'ArrowLeft') {
          event.preventDefault();
          goTo(index - 1);
        } else if (event.key === 'ArrowRight') {
          event.preventDefault();
          goTo(index + 1);
        }
      }}
    >
      {hole ? (
        <>
          <div className="wk-guide__spot" style={{ ...holeFrameStyle, boxShadow: '0 0 0 9999px ' + GUIDE_BACKDROP_COLOR }} aria-hidden="true" />
          {computeBackdropPieces(vw, vh, hole).map((piece, pieceIndex) => (
            <div key={pieceIndex} className="wk-guide__backdrop wk-guide__backdrop--hit" style={piece} />
          ))}
        </>
      ) : (
        <div className="wk-guide__backdrop wk-guide__backdrop--full" />
      )}

      {hole && <div className="wk-guide__ring" style={holeFrameStyle} aria-hidden="true" />}

      <div
        ref={cardRef}
        className={hole ? 'wk-guide__card' : 'wk-guide__card wk-guide__card--center'}
        style={cardStyle}
        data-testid="wk-contextual-guide-card"
      >
        <button type="button" className="wk-guide__close" aria-label={skipLabel} onClick={onDismiss}>
          <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" aria-hidden="true">
            <path d="M4 4l8 8M12 4l-8 8" />
          </svg>
        </button>

        <div className="wk-guide__progress">
          {steps.map((entry, entryIndex) => {
            let className = 'wk-guide__dot';
            if (entryIndex === index) className += ' is-active';
            else if (entryIndex < index) className += ' is-done';
            return <span key={entry.key} className={className} />;
          })}
        </div>

        <p className="wk-guide__step-label">
          {contextualGuideMessage(locale, 'contextualGuide.stepOf', { current: index + 1, total: steps.length })}
        </p>
        <h3 className="wk-guide__title">{title}</h3>
        <p className="wk-guide__desc">{desc}</p>
        {step.interact ? <p className="wk-guide__interact-hint">{contextualGuideMessage(locale, 'contextualGuide.interactHint')}</p> : null}

        <div className="wk-guide__actions">
          <button type="button" className="wk-guide__skip" onClick={onDismiss}>
            {skipLabel}
          </button>
          {!step.interact ? (
            <div className="wk-guide__actions-main">
              {index > 0 ? (
                <button type="button" className="wk-guide__btn wk-guide__btn--outline" onClick={() => goTo(index - 1)}>
                  {contextualGuideMessage(locale, 'contextualGuide.prev')}
                </button>
              ) : null}
              {!isLast ? (
                <button type="button" className="wk-guide__btn wk-guide__btn--primary" onClick={() => goTo(index + 1)}>
                  {contextualGuideMessage(locale, 'contextualGuide.next')}
                </button>
              ) : (
                <button type="button" className="wk-guide__btn wk-guide__btn--primary" onClick={onDismiss}>
                  {contextualGuideMessage(locale, 'contextualGuide.done')}
                </button>
              )}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export interface ContextualGuideHostProps {
  locale: NewUserGuideLocale;
  /** Defaults to window.localStorage at interaction time. */
  storage?: KeyValueStorage;
  /** Defaults to window.sessionStorage (pending-intent hand-off). */
  sessionStorage?: Storage;
  actions?: ContextualGuideActions;
  /** Vue poll interval while waiting for the global guide (400ms, line 72). */
  pollIntervalMs?: number;
  /** Test hook: replaces the catalog open delay when set. */
  openDelayOverrideMs?: number;
  beforeDelayMs?: number;
  locateRetryDelayMs?: number;
}

interface ActiveGuideRequest {
  tour: ContextualGuideTourId;
  options?: ContextualGuideTriggerOptions;
}

/**
 * Shell-level host replacing Vue's per-page wrapper components. Scheduling is
 * ContextualGuide.vue verbatim: ignore while active, ignore finished tours,
 * wait for the global welcome tour (400ms poll), then open after the tour's
 * openDelayMs — re-checking both conditions inside the timer callback.
 */
export function ContextualGuideHost({
  locale,
  storage,
  sessionStorage,
  actions,
  pollIntervalMs = 400,
  openDelayOverrideMs,
  beforeDelayMs,
  locateRetryDelayMs,
}: ContextualGuideHostProps): ReactNode {
  const [request, setActive] = useState<ActiveGuideRequest | null>(null);
  const activeRef = useRef(false);
  activeRef.current = request !== null;
  const storageRef = useRef(storage);
  storageRef.current = storage;
  const openTimerRef = useRef<number | null>(null);
  const pollTimerRef = useRef<number | null>(null);

  const getStorage = (): KeyValueStorage => storageRef.current ?? window.localStorage;
  const getSessionStorage = (): Storage | undefined => sessionStorage ?? window.sessionStorage;

  const clearTimers = () => {
    if (openTimerRef.current !== null) {
      window.clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
    if (pollTimerRef.current !== null) {
      window.clearTimeout(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  };

  const schedule = (detail: ContextualGuideOpenDetail) => {
    if (activeRef.current) return;
    if (isContextualGuideDone(getStorage(), detail.tour)) return;
    // NOTE: the sessionStorage intent is consumed only at activation (and at
    // host mount). A trigger followed by a full-page navigation therefore
    // leaves the intent behind for the destination page's host, mirroring the
    // Vue wrapper mounting on the destination view.
    const arm = () => {
      if (openTimerRef.current !== null) window.clearTimeout(openTimerRef.current);
      const delay = openDelayOverrideMs ?? CONTEXTUAL_GUIDE_OPEN_DELAY_MS[detail.tour];
      openTimerRef.current = window.setTimeout(() => {
        openTimerRef.current = null;
        // Vue tryOpen timer body (ContextualGuide.vue:46-49): re-check both
        // gates, then consume the one-shot intent at activation so a later
        // reload cannot resurrect the tour.
        if (activeRef.current || isContextualGuideDone(getStorage(), detail.tour)) return;
        consumePendingContextualGuide(getSessionStorage());
        setActive({ tour: detail.tour, options: detail.options });
      }, delay);
    };
    if (shouldOpenContextualGuide(getStorage(), detail.tour, true)) {
      arm();
      return;
    }
    // Vue scheduleOpen poll (lines 62-75): wait for the global tour to end.
    const poll = () => {
      pollTimerRef.current = null;
      if (isContextualGuideDone(getStorage(), detail.tour)) return;
      if (isGlobalUserGuideDone(getStorage())) {
        arm();
        return;
      }
      pollTimerRef.current = window.setTimeout(poll, pollIntervalMs);
    };
    pollTimerRef.current = window.setTimeout(poll, pollIntervalMs);
  };

  // Trigger entry points: page code dispatches OPEN_CONTEXTUAL_GUIDE_EVENT via
  // openContextualGuide(); the pending hand-off covers full-page navigations.
  useEffect(() => {
    const onOpenEvent = (event: Event) => {
      const detail = (event as CustomEvent<ContextualGuideOpenDetail>).detail;
      if (!detail || typeof detail.tour !== 'string') return;
      schedule(detail);
    };
    window.addEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onOpenEvent);
    const pending = consumePendingContextualGuide(getSessionStorage());
    if (pending) schedule(pending);
    return () => {
      window.removeEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onOpenEvent);
      clearTimers();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Vue onFinish/onDismiss (ContextualGuide.vue:77-82): both paths persist.
  const dismiss = () => {
    const current = request;
    clearTimers();
    setActive(null);
    if (current) markContextualGuideDone(getStorage(), current.tour);
  };

  if (!request) return null;
  return (
    <ContextualGuide
      tour={request.tour}
      locale={locale}
      options={request.options}
      open
      onDismiss={dismiss}
      storage={storage}
      actions={actions}
      beforeDelayMs={beforeDelayMs}
      locateRetryDelayMs={locateRetryDelayMs}
    />
  );
}
