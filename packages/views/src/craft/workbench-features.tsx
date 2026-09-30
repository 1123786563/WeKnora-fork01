import type { ReactNode } from 'react';

export type CraftWorkbenchSlot = 'header' | 'conversation' | 'aside';
export interface CraftWorkbenchFeatureContext {
  sessionId: string;
  canWrite: boolean;
  selectedVersionId: string | null;
}
export interface CraftWorkbenchFeature {
  name: string;
  slot: CraftWorkbenchSlot;
  render(context: CraftWorkbenchFeatureContext): ReactNode;
}

const slots: readonly CraftWorkbenchSlot[] = ['header', 'conversation', 'aside'];

// Construct once in the web adapter before rendering. The result is immutable
// so feature registration cannot vary across re-renders or a resumed Task.
export function createCraftWorkbenchFeatures(entries: readonly CraftWorkbenchFeature[]): readonly CraftWorkbenchFeature[] {
  const names = new Set<string>();
  const sorted = [...entries].sort((a, b) => a.name.localeCompare(b.name));
  for (const entry of sorted) {
    if (!/^[a-z][a-z0-9_-]*$/.test(entry.name)) throw new Error('invalid Craft feature name');
    if (names.has(entry.name)) throw new Error('duplicate Craft feature name');
    if (!slots.includes(entry.slot)) throw new Error('invalid Craft feature slot');
    if (typeof entry.render !== 'function') throw new Error('Craft feature render unavailable');
    names.add(entry.name);
    Object.freeze(entry);
  }
  return Object.freeze(sorted);
}
