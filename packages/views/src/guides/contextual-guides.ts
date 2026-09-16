// Contextual guide catalog + trigger/persistence semantics, ported from the
// Vue baseline (read-only):
//   frontend/src/config/contextualGuides.ts        — tour ids, storage keys,
//                                                    open delays, alsoComplete
//   frontend/src/components/ContextualGuide.vue    — when→open scheduling,
//                                                    global-guide gating, dismissal
//   frontend/src/components/KbCreateContextualGuide.vue   — dynamic steps
//   frontend/src/components/AgentCreateContextualGuide.vue — dynamic steps
//   frontend/src/components/TenantModelsGuide.vue  — variant steps + settings
//
// The storage contract and the step side effects are structural so tests can
// exercise them without a DOM: storage is injectable (KeyValueStorage) and the
// Vue `before` callbacks become declarative actions the component maps onto
// window events / shell callbacks (same pattern as guides/steps.ts).
import { GLOBAL_USER_GUIDE_KEY, type KeyValueStorage } from './new-user-guide.ts';
import { formatGuidePattern, type NewUserGuideLocale } from './steps.ts';
import { CONTEXTUAL_GUIDE_MESSAGES, type ContextualGuideMessageKey } from './contextual-guide-messages.ts';

export { GLOBAL_USER_GUIDE_KEY };

/** Exact Vue literal (frontend/src/config/contextualGuides.ts:10-11). */
export const KB_EDITOR_FOCUS_SECTION_EVENT = 'weknora:kb-editor-focus-section';
/** Exact Vue literal (frontend/src/config/contextualGuides.ts:11). */
export const AGENT_EDITOR_FOCUS_SECTION_EVENT = 'weknora:agent-editor-focus-section';

/**
 * React-only host channel: page code calls openContextualGuide(tour) instead
 * of mounting a per-page wrapper component (the Vue shape); the shell-level
 * ContextualGuideHost listens for this event. Documented as a React-side
 * mechanism in the slice evidence.
 */
export const OPEN_CONTEXTUAL_GUIDE_EVENT = 'weknora:open-contextual-guide';

/**
 * React-only hand-off for triggers that precede a full-page navigation (the
 * React client reloads on most route changes, unlike the Vue SPA). The intent
 * is written next to the dispatch and consumed by the host on receipt or on
 * the next page load, so "navigate then guide" behaves like Vue's mounted
 * wrapper landing on the destination page.
 */
export const CONTEXTUAL_GUIDE_PENDING_KEY = 'weknora:contextual-guide-pending:v1';

/** Vue: focusKbEditorSection (contextualGuides.ts:22-26). */
export function focusKbEditorSection(section: string, targetWindow: Pick<Window, 'dispatchEvent'> = window): boolean {
  return targetWindow.dispatchEvent(new CustomEvent(KB_EDITOR_FOCUS_SECTION_EVENT, { detail: { section } }));
}

/** Vue: focusAgentEditorSection (contextualGuides.ts:28-32). */
export function focusAgentEditorSection(section: string, targetWindow: Pick<Window, 'dispatchEvent'> = window): boolean {
  return targetWindow.dispatchEvent(new CustomEvent(AGENT_EDITOR_FOCUS_SECTION_EVENT, { detail: { section } }));
}

export type ContextualGuideTourId =
  | 'kbList'
  | 'kbCreate'
  | 'kbDetail'
  | 'chat'
  | 'tenantModels'
  | 'agentList'
  | 'agentCreate';

export const CONTEXTUAL_GUIDE_TOUR_IDS: readonly ContextualGuideTourId[] = [
  'kbList', 'kbCreate', 'kbDetail', 'chat', 'tenantModels', 'agentList', 'agentCreate',
];

/** Exact Vue literals (contextualGuides.ts:48,63,70,76,92,116,129). */
export const CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS: Record<ContextualGuideTourId, string> = {
  kbList: 'weknora:contextual-guide-kb-list:v2',
  kbCreate: 'weknora:contextual-guide-kb-create:v3',
  kbDetail: 'weknora:contextual-guide-kb-detail:v1',
  chat: 'weknora:contextual-guide-chat:v1',
  tenantModels: 'weknora:contextual-guide-tenant-models:v1',
  agentList: 'weknora:contextual-guide-agent-list:v1',
  agentCreate: 'weknora:contextual-guide-agent-create:v1',
};

/** Vue openDelayMs per tour (contextualGuides.ts + the three wrappers). */
export const CONTEXTUAL_GUIDE_OPEN_DELAY_MS: Record<ContextualGuideTourId, number> = {
  kbList: 500,
  kbCreate: 450,
  kbDetail: 600,
  chat: 800,
  tenantModels: 500,
  agentList: 500,
  agentCreate: 450,
};

/** Vue alsoCompleteTours: completing one tour retires these too. */
export const CONTEXTUAL_GUIDE_ALSO_COMPLETE_TOURS: Partial<Record<ContextualGuideTourId, readonly ContextualGuideTourId[]>> = {
  kbCreate: ['kbList'],
  agentCreate: ['agentList'],
};

/** Vue GuidePlacement (types/spotlightGuide.ts:1). */
export type ContextualGuidePlacement = 'right' | 'left' | 'bottom' | 'top';

/**
 * Declarative replacement for the Vue `before` callbacks: the component
 * maps these onto window events / shell callbacks when the step activates.
 */
export type ContextualGuideStepAction =
  | { kind: 'focus-kb-editor-section'; section: string }
  | { kind: 'focus-agent-editor-section'; section: string }
  | { kind: 'open-models-settings' };

export interface ContextualGuideStep {
  key: string;
  /** Highlight target selector; a missing/undefined target centers the card. */
  target?: string;
  placement?: ContextualGuidePlacement;
  /** Skip this step entirely when the target cannot be located (Vue optional). */
  optional?: boolean;
  /** Guide clicks the highlighted area directly; no next button (Vue interact). */
  interact?: boolean;
  /** Side effect run when the step becomes active (Vue before). */
  action?: ContextualGuideStepAction;
}

/** Per-tour dynamic inputs (Vue wrapper props). */
export interface ContextualGuideTriggerOptions {
  /** KbCreateContextualGuide.vue :is-faq. */
  isFaq?: boolean;
  /** KbCreateContextualGuide.vue :needs-embedding. */
  needsEmbedding?: boolean;
  /** AgentCreateContextualGuide.vue :is-agent-mode. */
  isAgentMode?: boolean;
  /** TenantModelsGuide.vue variant (default documentKb). */
  variant?: 'documentKb' | 'agent';
}

/** Static tour catalogs — selectors/placements/options are the Vue literals. */
export const CONTEXTUAL_GUIDE_STEPS: Record<
  'kbList' | 'kbDetail' | 'chat' | 'agentList' | 'tenantModels',
  readonly ContextualGuideStep[]
> = {
  // contextualGuides.ts:51-60 — empty list spotlights the centered primary CTA
  // first, falling back to the header create button.
  kbList: [
    {
      key: 'create',
      target: '.empty-state-btn[data-guide="kb-list-create"], [data-guide="kb-list-create"]',
      placement: 'bottom',
      interact: true,
    },
  ],
  // contextualGuides.ts:79-89.
  kbDetail: [
    { key: 'intro' },
    { key: 'upload', target: '[data-guide="kb-detail-add-doc"]', placement: 'bottom' },
    { key: 'done' },
  ],
  // contextualGuides.ts:95-113.
  chat: [
    { key: 'kb', target: '[data-guide="chat-kb-mention"]', placement: 'top', optional: true },
    { key: 'input', target: '[data-guide="chat-input"]', placement: 'top' },
    { key: 'send', target: '[data-guide="chat-send"]', placement: 'top' },
    { key: 'done' },
  ],
  // contextualGuides.ts:119-126.
  agentList: [
    {
      key: 'create',
      target: '.empty-state-btn[data-guide="agent-list-create"], [data-guide="agent-list-create"]',
      placement: 'bottom',
      interact: true,
    },
  ],
  // TenantModelsGuide.vue:36-48 — the addModel step opens the models settings
  // section in place (Vue uiStore.openSettings('models')).
  tenantModels: [
    { key: 'intro' },
    {
      key: 'addModel',
      target: '[data-guide="settings-add-model"], [data-guide="settings-models"]',
      placement: 'left',
      action: { kind: 'open-models-settings' },
    },
    { key: 'done' },
  ],
};

/**
 * KbCreateContextualGuide.vue:25-138 — steps assembled by document-库/FAQ and
 * the indexing strategy (needsEmbedding). Order and optionality are the Vue
 * literals; every `before` becomes a focus-kb-editor-section action.
 */
export function kbCreateGuideSteps(options: ContextualGuideTriggerOptions = {}): readonly ContextualGuideStep[] {
  const isFaq = options.isFaq === true;
  const needsEmbedding = options.needsEmbedding === true;
  const focus = (section: string): ContextualGuideStepAction => ({ kind: 'focus-kb-editor-section', section });
  const steps: ContextualGuideStep[] = [
    { key: 'type', target: '[data-guide="kb-create-type"]', placement: 'right', action: focus('basic') },
    { key: 'name', target: '[data-guide="kb-create-name"]', placement: 'right', action: focus('basic') },
  ];
  if (!isFaq) {
    steps.push({ key: 'indexing', target: '[data-guide="kb-create-indexing"]', placement: 'right', action: focus('basic') });
  }
  steps.push(
    { key: 'navModels', target: '[data-guide="kb-editor-nav-models"]', placement: 'right', action: focus('models') },
    { key: 'llm', target: '[data-guide="kb-create-llm"]', placement: 'right', action: focus('models') },
  );
  if (needsEmbedding) {
    steps.push({ key: 'embedding', target: '[data-guide="kb-create-embedding"]', placement: 'right', action: focus('models'), optional: true });
  }
  if (!isFaq) {
    steps.push(
      { key: 'parser', target: '[data-guide="kb-editor-nav-parser"]', placement: 'right', action: focus('parser'), optional: true },
      { key: 'chunking', target: '[data-guide="kb-editor-nav-chunking"]', placement: 'right', action: focus('chunking'), optional: true },
      { key: 'storage', target: '[data-guide="kb-editor-nav-storage"]', placement: 'right', action: focus('storage'), optional: true },
      { key: 'navMultimodal', target: '[data-guide="kb-editor-nav-multimodal"]', placement: 'right', action: focus('multimodal'), optional: true },
      { key: 'multimodalToggle', target: '[data-guide="kb-create-multimodal-toggle"]', placement: 'right', action: focus('multimodal'), optional: true },
      { key: 'multimodalVllm', target: '[data-guide="kb-create-multimodal-vllm"]', placement: 'right', action: focus('multimodal'), optional: true },
    );
  } else {
    steps.push({ key: 'faq', target: '[data-guide="kb-editor-nav-faq"]', placement: 'right', action: focus('faq'), optional: true });
  }
  steps.push({ key: 'submit', target: '[data-guide="kb-create-submit"]', placement: 'top', action: focus('basic') });
  return steps;
}

/**
 * AgentCreateContextualGuide.vue:24-111 — steps assembled by agent mode;
 * navTools only appears in smart-reasoning (agent) mode.
 */
export function agentCreateGuideSteps(options: ContextualGuideTriggerOptions = {}): readonly ContextualGuideStep[] {
  const isAgentMode = options.isAgentMode === true;
  const focus = (section: string): ContextualGuideStepAction => ({ kind: 'focus-agent-editor-section', section });
  const steps: ContextualGuideStep[] = [
    { key: 'mode', target: '[data-guide="agent-create-mode"]', placement: 'right', action: focus('basic') },
    { key: 'agentType', target: '[data-guide="agent-create-agent-type"]', placement: 'right', action: focus('basic'), optional: true },
    { key: 'name', target: '[data-guide="agent-create-name"]', placement: 'right', action: focus('basic') },
    { key: 'navModel', target: '[data-guide="agent-editor-nav-model"]', placement: 'right', action: focus('model') },
    { key: 'model', target: '[data-guide="agent-create-model"]', placement: 'right', action: focus('model') },
    { key: 'navKnowledge', target: '[data-guide="agent-editor-nav-knowledge"]', placement: 'right', action: focus('knowledge') },
    { key: 'knowledge', target: '[data-guide="agent-create-knowledge"]', placement: 'right', action: focus('knowledge') },
    { key: 'navWebsearch', target: '[data-guide="agent-editor-nav-websearch"]', placement: 'right', action: focus('websearch'), optional: true },
    { key: 'navMultimodal', target: '[data-guide="agent-editor-nav-multimodal"]', placement: 'right', action: focus('multimodal'), optional: true },
    { key: 'multimodal', target: '[data-guide="agent-create-multimodal"]', placement: 'right', action: focus('multimodal'), optional: true },
  ];
  if (isAgentMode) {
    steps.push({ key: 'navTools', target: '[data-guide="agent-editor-nav-tools"]', placement: 'right', action: focus('tools'), optional: true });
  }
  steps.push({ key: 'submit', target: '[data-guide="agent-create-submit"]', placement: 'top', action: focus('basic'), interact: true });
  return steps;
}

/** Steps for any tour, resolving the dynamic builders (Vue wrapper computed). */
export function resolveContextualGuideSteps(tour: ContextualGuideTourId, options: ContextualGuideTriggerOptions = {}): readonly ContextualGuideStep[] {
  if (tour === 'kbCreate') return kbCreateGuideSteps(options);
  if (tour === 'agentCreate') return agentCreateGuideSteps(options);
  return CONTEXTUAL_GUIDE_STEPS[tour];
}

/**
 * vue-i18n step-copy prefix for a tour: TenantModelsGuide.vue:26-30 switches
 * to contextualGuide.tenantModels.stepsAgent for the agent variant.
 */
export function contextualGuideStepPrefix(tour: ContextualGuideTourId, options: ContextualGuideTriggerOptions = {}): string {
  if (tour === 'tenantModels' && options.variant === 'agent') return 'contextualGuide.tenantModels.stepsAgent';
  return `contextualGuide.${tour}.steps`;
}

/** Vue: isContextualGuideDone (contextualGuides.ts:137-139) — '1' means done. */
export function isContextualGuideDone(storage: KeyValueStorage, tourId: ContextualGuideTourId): boolean {
  return storage.getItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS[tourId]) === '1';
}

/** Vue: markContextualGuideDone (contextualGuides.ts:141-147) — cascade alsoCompleteTours. */
export function markContextualGuideDone(storage: KeyValueStorage, tourId: ContextualGuideTourId): void {
  storage.setItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS[tourId], '1');
  for (const id of CONTEXTUAL_GUIDE_ALSO_COMPLETE_TOURS[tourId] ?? []) {
    storage.setItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS[id], '1');
  }
}

/** Vue global gate (contextualGuides.ts:149-151). */
export function isGlobalUserGuideDone(storage: KeyValueStorage): boolean {
  return storage.getItem(GLOBAL_USER_GUIDE_KEY) === '1';
}

/**
 * Vue KnowledgeBase.vue:339-345 showKbDetailContextualGuide — the kbDetail
 * tour arms on detail-page entry for an editable, non-FAQ knowledge base
 * whose document list finished loading empty. Tab-independent: the Vue
 * computed never reads the active tab, so graph/wiki tab entries open it too.
 * `documentsLoading` maps to Vue docListLoading; `documentCount` to
 * cardList.length (first page items).
 */
export function shouldArmKbDetailGuideOnEntry(input: {
  knowledgeBaseId: string;
  kbType?: string | null;
  canEdit: boolean;
  documentsLoading: boolean;
  documentCount: number;
}): boolean {
  const kbType = typeof input.kbType === 'string' ? input.kbType.trim().toLowerCase() : '';
  return Boolean(input.knowledgeBaseId)
    && kbType !== 'faq'
    && input.canEdit
    && !input.documentsLoading
    && input.documentCount === 0;
}

/**
 * Vue ContextualGuide.vue tryOpen guard (lines 40-50): a contextual guide
 * opens only while the trigger condition holds, the tour was never finished,
 * and the global welcome tour has already ended (no stacked overlays).
 */
export function shouldOpenContextualGuide(storage: KeyValueStorage, tourId: ContextualGuideTourId, when: boolean): boolean {
  return when && !isContextualGuideDone(storage, tourId) && isGlobalUserGuideDone(storage);
}

/** Payload of OPEN_CONTEXTUAL_GUIDE_EVENT and of the sessionStorage hand-off. */
export interface ContextualGuideOpenDetail {
  tour: ContextualGuideTourId;
  options?: ContextualGuideTriggerOptions;
}

/**
 * Trigger helper ("openGuide(id)"): records the intent for the next document
 * (full-page navigations kill the current React tree, so the pending entry
 * lets the destination page's host pick the tour up) and dispatches the event
 * for a host already mounted in this document.
 */
export function openContextualGuide(
  tour: ContextualGuideTourId,
  options?: ContextualGuideTriggerOptions,
  targetWindow: Pick<Window, 'dispatchEvent' | 'sessionStorage'> = window,
): void {
  queuePendingContextualGuide({ tour, options }, targetWindow.sessionStorage);
  targetWindow.dispatchEvent(new CustomEvent<ContextualGuideOpenDetail>(OPEN_CONTEXTUAL_GUIDE_EVENT, { detail: { tour, options } }));
}

/** Write the one-shot intent (React-only; see CONTEXTUAL_GUIDE_PENDING_KEY). */
export function queuePendingContextualGuide(detail: ContextualGuideOpenDetail, sessionStorage?: Storage): void {
  try {
    sessionStorage?.setItem(CONTEXTUAL_GUIDE_PENDING_KEY, JSON.stringify(detail));
  } catch { /* storage unavailable (quota/private mode): the in-document event still fires */ }
}

/** Read + clear the one-shot intent; returns null when absent/corrupt. */
export function consumePendingContextualGuide(sessionStorage?: Storage): ContextualGuideOpenDetail | null {
  try {
    const raw = sessionStorage?.getItem(CONTEXTUAL_GUIDE_PENDING_KEY);
    if (!raw) return null;
    sessionStorage?.removeItem(CONTEXTUAL_GUIDE_PENDING_KEY);
    const parsed: unknown = JSON.parse(raw);
    if (parsed && typeof parsed === 'object' && typeof (parsed as ContextualGuideOpenDetail).tour === 'string') {
      return parsed as ContextualGuideOpenDetail;
    }
  } catch { /* corrupt entry: drop it */ }
  return null;
}

type ContextualGuideLabelKey =
  | 'contextualGuide.stepOf'
  | 'contextualGuide.skip'
  | 'contextualGuide.prev'
  | 'contextualGuide.next'
  | 'contextualGuide.done'
  | 'contextualGuide.interactHint';

/**
 * Catalog lookup + vue-i18n rendering for contextual-guide copy: named
 * params ({current}/{total}) interpolate via formatGuidePattern and the
 * literal escapes ({'@'}) resolve to the bare character, mirroring the
 * vue-i18n message compiler so the rendered strings match the Vue client.
 */
export function contextualGuideMessage(
  locale: NewUserGuideLocale,
  key: ContextualGuideMessageKey | ContextualGuideLabelKey | (string & {}),
  values?: Record<string, string | number>,
): string {
  const table = CONTEXTUAL_GUIDE_MESSAGES[locale] as Record<string, string>;
  const pattern = table[key] ?? key;
  const withLiterals = pattern.replace(/\{'([^']+)'\}/g, (_, literal: string) => literal);
  return values ? formatGuidePattern(withLiterals, values) : withLiterals;
}
