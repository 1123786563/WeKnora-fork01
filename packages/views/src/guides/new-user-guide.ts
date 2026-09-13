// Persistence + trigger semantics for the WeKnora welcome tour, ported from
// the Vue baseline:
//   frontend/src/config/contextualGuides.ts (GLOBAL_USER_GUIDE_KEY,
//   OPEN_NEW_USER_GUIDE_EVENT, openNewUserGuide, isGlobalUserGuideDone)
//   frontend/src/components/NewUserGuide.vue (onMounted auto-open with 700ms
//   delayed double-check, onFinish writing '1')
//
// The storage contract is structural so tests can stub it without a DOM.

/** Exact Vue literal (frontend/src/config/contextualGuides.ts:3). */
export const GLOBAL_USER_GUIDE_KEY = 'weknora:new-user-guide-done:v1';

/** Exact Vue literal (frontend/src/config/contextualGuides.ts:4). */
export const OPEN_NEW_USER_GUIDE_EVENT = 'weknora:open-new-user-guide';

/** Vue writes the literal '1' on finish AND on skip/dismiss. */
export const GLOBAL_USER_GUIDE_DONE_VALUE = '1';

export interface KeyValueStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

/** Vue: isGlobalUserGuideDone() — localStorage.getItem(KEY) === '1'. */
export function isNewUserGuideDone(storage: KeyValueStorage): boolean {
  return storage.getItem(GLOBAL_USER_GUIDE_KEY) === GLOBAL_USER_GUIDE_DONE_VALUE;
}

/**
 * Vue: NewUserGuide.vue onMounted checks localStorage.getItem(KEY) !== '1'
 * before arming the 700ms timer, and re-checks the same condition inside the
 * timer callback before actually opening. This predicate is used for both
 * checks so the component mirrors the double-check exactly.
 */
export function shouldAutoOpenNewUserGuide(storage: KeyValueStorage): boolean {
  return storage.getItem(GLOBAL_USER_GUIDE_KEY) !== GLOBAL_USER_GUIDE_DONE_VALUE;
}

/** Vue: NewUserGuide.vue onFinish — localStorage.setItem(KEY, '1'). */
export function markNewUserGuideDone(storage: KeyValueStorage): void {
  storage.setItem(GLOBAL_USER_GUIDE_KEY, GLOBAL_USER_GUIDE_DONE_VALUE);
}

/**
 * Vue: contextualGuides.ts openNewUserGuide() — any component (e.g. the user
 * menu help button) can re-open the tour by dispatching this window event.
 */
export function openNewUserGuide(targetWindow: Pick<Window, 'dispatchEvent'> = window): boolean {
  return targetWindow.dispatchEvent(new CustomEvent(OPEN_NEW_USER_GUIDE_EVENT));
}

export { guideMessage, NEW_USER_GUIDE_MESSAGES, NEW_USER_GUIDE_STEPS } from './steps.ts';
export type { GuidePlacement, NewUserGuideAction, NewUserGuideStep } from './steps.ts';
