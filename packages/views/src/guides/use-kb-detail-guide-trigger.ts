// Detail-entry trigger for the kbDetail contextual guide (the one-shot
// "knowledge base is empty" welcome tour).
//
// Vue baseline (read-only): frontend/src/views/knowledge/KnowledgeBase.vue
// mounts `<ContextualGuide tour="kbDetail" :when="showKbDetailContextualGuide" />`
// (line 2741) so the tour arms whenever the detail page shows an editable,
// non-FAQ, empty knowledge base — deep link, list card click, any tab. The
// React client has no per-page wrapper; the shell-level ContextualGuideHost
// (platform shell) is fed by openContextualGuide(tour) trigger calls, so this
// hook re-fires the same trigger the card-click path already uses whenever
// the Vue `when` condition turns true on the detail page.
//
// Dismissal/welcome-tour gating stays in the host (Vue ContextualGuide.vue
// tryOpen + scheduleOpen): an already-finished tour or a pending global
// welcome tour never opens here.
import { useEffect } from 'react';
import { openContextualGuide, shouldArmKbDetailGuideOnEntry } from './contextual-guides.ts';

/** Inputs mirror Vue showKbDetailContextualGuide (KnowledgeBase.vue:339-345). */
export interface KbDetailGuideTriggerInput {
  /** Route param: Vue kbId. Falsy never arms. */
  knowledgeBaseId: string;
  /** Vue kbInfo.type — 'faq' libraries never see this tour. */
  kbType?: string | null;
  /** Vue canEdit (owner/admin/share-editor); React passes the canEditKB-parity flag. */
  canEdit: boolean;
  /** Vue !docListLoading — document list request settled successfully. */
  documentsLoading: boolean;
  /** Vue cardList.length (loaded first page items). */
  documentCount: number;
}

/**
 * Fires openContextualGuide('kbDetail') exactly when the Vue `when` computed
 * turns true (and on KB identity changes while it holds); the shell host
 * applies the done/welcome-tour gates and the per-tour open delay.
 */
export function useKbDetailGuideTrigger(input: KbDetailGuideTriggerInput): void {
  const when = shouldArmKbDetailGuideOnEntry(input);
  useEffect(() => {
    if (!when) return;
    openContextualGuide('kbDetail');
  }, [when, input.knowledgeBaseId]);
}
