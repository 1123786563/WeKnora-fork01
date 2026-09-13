/** Vue KnowledgeBase.vue toggleSelectRow parity for the current page order. */
export function toggleDocumentSelection(input: {
  ids: readonly string[];
  selected: ReadonlySet<string>;
  id: string;
  checked: boolean;
  shiftKey?: boolean;
  lastIndex: number;
}): { selected: Set<string>; lastIndex: number } {
  const next = new Set(input.selected);
  const index = input.ids.indexOf(input.id);
  if (input.shiftKey && input.lastIndex >= 0 && index >= 0) {
    const start = Math.min(input.lastIndex, index);
    const end = Math.max(input.lastIndex, index);
    for (let i = start; i <= end; i += 1) {
      if (input.checked) next.add(input.ids[i]);
      else next.delete(input.ids[i]);
    }
  } else if (index >= 0) {
    if (input.checked) next.add(input.id);
    else next.delete(input.id);
  }
  return { selected: next, lastIndex: index };
}
