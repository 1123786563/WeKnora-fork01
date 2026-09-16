// React port of the WeKnora welcome tour.
//
// Vue baseline (authoritative, read-only):
//   frontend/src/components/NewUserGuide.vue  — trigger/persistence wrapper
//   frontend/src/components/SpotlightGuide.vue — spotlight card renderer
//
// Trigger semantics (NewUserGuide.vue onMounted, lines 84-93):
//   - always listen for the weknora:open-new-user-guide window event (re-open
//     entry point, e.g. a future user-menu help button);
//   - auto-open 700ms after mount, but only while the done-key is unset;
//     the key is re-checked inside the timer callback.
// Persistence (NewUserGuide.vue onFinish, line 64-67):
//   - write GLOBAL_USER_GUIDE_KEY = '1' on finish AND on skip/dismiss.
// Renderer semantics (SpotlightGuide.vue): spotlight hole + 4-piece backdrop
// for anchored steps, full backdrop + centered card for target-less steps,
// non-clickable progress dots, skip on the left, prev/next/done on the right,
// Esc dismisses, arrow keys navigate, card focuses itself on every step.
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react';
// NOTE: guides.css is imported by the platform shell (apps/web), not here —
// packages/views must stay css-import-free for the shared typecheck.
import {
  GLOBAL_USER_GUIDE_KEY,
  OPEN_NEW_USER_GUIDE_EVENT,
  markNewUserGuideDone,
  shouldAutoOpenNewUserGuide,
} from './new-user-guide.ts';
import {
  guideMessage,
  NEW_USER_GUIDE_MESSAGES,
  NEW_USER_GUIDE_STEPS,
  type NewUserGuideLocale,
  type NewUserGuideStep,
} from './steps.ts';
import type { KeyValueStorage } from './new-user-guide.ts';

import {
  computeBackdropPieces,
  computeCardStyle,
  computeHighlightHole,
  GUIDE_BACKDROP_COLOR,
  GUIDE_CARD_WIDTH,
  GUIDE_HOLE_RADIUS,
  type GuideHoleRect,
} from './geometry.ts';

export { computeBackdropPieces, computeCardStyle, computeHighlightHole, GUIDE_BACKDROP_COLOR, GUIDE_CARD_WIDTH, GUIDE_HOLE_RADIUS };
export type { GuideHoleRect };
/** Vue hardcodes 120ms between target-locate retries (SpotlightGuide.vue:290). */
const GUIDE_LOCATE_RETRIES = 12;

/** Vue queryTarget (lines 253-262): first selector part with a real box. */
function queryTarget(selector: string, root: ParentNode): Element | null {
  for (const part of selector.split(',').map((s) => s.trim()).filter(Boolean)) {
    const el = root.querySelector(part);
    if (!el) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width > 2 && rect.height > 2) return el;
  }
  return null;
}

/** Vue measureNeighborGap/computed style access for the hole computation. */
function elementViewport(): { getComputedStyle: (el: unknown) => { marginBottom: string } } {
  return { getComputedStyle: (el: unknown) => window.getComputedStyle(el as Element) };
}

/** Shell callbacks replacing the Vue uiStore side effects (expand sidebar,
 * open the models settings section). All optional. */
export interface NewUserGuideActions {
  expandSidebar?: () => void;
  openModelsSettings?: () => void;
  closeGuideSettings?: () => void;
}

export interface NewUserGuideProps {
  locale: NewUserGuideLocale;
  /** Defaults to window.localStorage at interaction time. */
  storage?: KeyValueStorage;
  actions?: NewUserGuideActions;
  /** Vue hardcodes 700ms before the auto-open check (NewUserGuide.vue:87). */
  autoOpenDelayMs?: number;
  /** Vue SpotlightGuide beforeDelayMs default (line 77). */
  beforeDelayMs?: number;
  /** Vue hardcodes 120ms between target-locate retries (line 290). */
  locateRetryDelayMs?: number;
}

type MessageKey = keyof (typeof NEW_USER_GUIDE_MESSAGES)['zh-CN'];

function stepMessageKey(locale: NewUserGuideLocale, key: string, kind: 'title' | 'desc'): string {
  // The catalog is closed, so the concatenation is always a known key.
  return guideMessage(locale, ('newUserGuide.steps.' + key + '.' + kind) as MessageKey);
}

export function NewUserGuide({
  locale,
  storage,
  actions,
  autoOpenDelayMs = 700,
  beforeDelayMs = 280,
  locateRetryDelayMs = 120,
}: NewUserGuideProps): ReactNode {
  const [active, setActive] = useState(false);
  const [index, setIndex] = useState(0);
  const [vw, setVw] = useState(0);
  const [vh, setVh] = useState(0);
  const [hole, setHole] = useState<GuideHoleRect | null>(null);
  const [cardSize, setCardSize] = useState({ width: GUIDE_CARD_WIDTH, height: 220 });

  const rootRef = useRef<HTMLDivElement | null>(null);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const activeRef = useRef(false);
  activeRef.current = active;
  const actionsRef = useRef(actions);
  actionsRef.current = actions;
  const storageRef = useRef(storage);
  storageRef.current = storage;
  const settingsOpenedByGuideRef = useRef(false);

  const getStorage = (): KeyValueStorage => storageRef.current ?? window.localStorage;

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
    if (el) {
      setCardSize({ width: el.offsetWidth, height: el.offsetHeight });
    }
  };

  const goTo = (nextIndex: number) => {
    if (nextIndex < 0 || nextIndex >= NEW_USER_GUIDE_STEPS.length) return;
    clearRetryTimer();
    setIndex(nextIndex);
  };

  const locate = (stepIndex: number, retry: number): void => {
    setVw(window.innerWidth);
    setVh(window.innerHeight);
    const step = NEW_USER_GUIDE_STEPS[stepIndex];
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

  // Vue onStepChange (NewUserGuide.vue lines 69-73): moving away from the
  // models step closes the settings section the guide opened.
  const closeGuideSettings = () => {
    if (settingsOpenedByGuideRef.current) {
      actionsRef.current?.closeGuideSettings?.();
      settingsOpenedByGuideRef.current = false;
    }
  };

  // Step driver — Vue open()/goTo() (SpotlightGuide.vue lines 309-326, 356-361):
  // run the step's before-hook, honor the before delay, then locate and focus.
  useEffect(() => {
    if (!active) return;
    clearRetryTimer();
    const step = NEW_USER_GUIDE_STEPS[index];
    if (!step) return;
    if (step.key !== 'models') closeGuideSettings();
    let cancelled = false;
    void (async () => {
      if (step.action === 'expand-sidebar') actionsRef.current?.expandSidebar?.();
      else if (step.action === 'open-models-settings') {
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
  }, [active, index, beforeDelayMs, locateRetryDelayMs]);

  // Vue onMounted (NewUserGuide.vue lines 84-93): re-open event listener plus
  // the delayed, double-checked auto-open.
  useEffect(() => {
    const handleOpenEvent = () => {
      if (activeRef.current) return;
      setActive(true);
    };
    window.addEventListener(OPEN_NEW_USER_GUIDE_EVENT, handleOpenEvent);
    let timer: number | null = null;
    if (shouldAutoOpenNewUserGuide(getStorage())) {
      timer = window.setTimeout(() => {
        if (shouldAutoOpenNewUserGuide(getStorage())) {
          setActive(true);
        }
      }, autoOpenDelayMs);
    }
    return () => {
      window.removeEventListener(OPEN_NEW_USER_GUIDE_EVENT, handleOpenEvent);
      if (timer !== null) window.clearTimeout(timer);
      closeGuideSettings();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoOpenDelayMs]);

  // Vue resize/scroll listeners while the guide is open (lines 351-395).
  useEffect(() => {
    if (!active) return;
    const onViewportChange = () => locate(index, 0);
    window.addEventListener('resize', onViewportChange);
    window.addEventListener('scroll', onViewportChange, true);
    return () => {
      window.removeEventListener('resize', onViewportChange);
      window.removeEventListener('scroll', onViewportChange, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, index]);

  // Vue finish + dismiss (SpotlightGuide.vue lines 341-349; NewUserGuide.vue
  // line 64-67): both completion and skip persist the done-key.
  const finish = () => {
    markNewUserGuideDone(getStorage());
    closeGuideSettings();
    clearRetryTimer();
    setHole(null);
    setActive(false);
  };

  const next = () => goTo(index + 1);
  const prev = () => goTo(index - 1);

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      finish();
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      prev();
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      next();
    }
  };

  if (!active) return null;

  const step = NEW_USER_GUIDE_STEPS[index] ?? NEW_USER_GUIDE_STEPS[0];
  const isLast = index === NEW_USER_GUIDE_STEPS.length - 1;
  const skipLabel = guideMessage(locale, 'newUserGuide.skip');
  const title = stepMessageKey(locale, step.key, 'title');
  const desc = stepMessageKey(locale, step.key, 'desc');
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
      onKeyDown={handleKeyDown}
      data-testid="wk-new-user-guide"
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
        data-testid="wk-new-user-guide-card"
      >
        <button type="button" className="wk-guide__close" aria-label={skipLabel} onClick={finish}>
          <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" aria-hidden="true">
            <path d="M4 4l8 8M12 4l-8 8" />
          </svg>
        </button>

        <div className="wk-guide__progress">
          {NEW_USER_GUIDE_STEPS.map((entry, entryIndex) => {
            let className = 'wk-guide__dot';
            if (entryIndex === index) className += ' is-active';
            else if (entryIndex < index) className += ' is-done';
            return <span key={entry.key} className={className} />;
          })}
        </div>

        <p className="wk-guide__step-label">
          {guideMessage(locale, 'newUserGuide.stepOf', { current: index + 1, total: NEW_USER_GUIDE_STEPS.length })}
        </p>
        <h3 className="wk-guide__title">{title}</h3>
        <p className="wk-guide__desc">{desc}</p>

        <div className="wk-guide__actions">
          <button type="button" className="wk-guide__skip" onClick={finish}>
            {skipLabel}
          </button>
          <div className="wk-guide__actions-main">
            {index > 0 && (
              <button type="button" className="wk-guide__btn wk-guide__btn--outline" onClick={prev}>
                {guideMessage(locale, 'newUserGuide.prev')}
              </button>
            )}
            {!isLast ? (
              <button type="button" className="wk-guide__btn wk-guide__btn--primary" onClick={next}>
                {guideMessage(locale, 'newUserGuide.next')}
              </button>
            ) : (
              <button type="button" className="wk-guide__btn wk-guide__btn--primary" onClick={finish}>
                {guideMessage(locale, 'newUserGuide.done')}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export { GLOBAL_USER_GUIDE_KEY, OPEN_NEW_USER_GUIDE_EVENT };
