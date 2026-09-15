import { useCallback, useEffect, useRef, useState, type RefObject } from "react";

type MarqueeMode = "add" | "subtract";

export interface MarqueeSelectionState {
  visible: boolean;
  mode: MarqueeMode;
  left: number;
  top: number;
  width: number;
  height: number;
}

export function applyMarqueeSelection(
  base: ReadonlySet<string>,
  hit: Iterable<string>,
  mode: MarqueeMode,
): Set<string> {
  const next = new Set(base);
  for (const id of hit) {
    if (mode === "subtract") next.delete(id);
    else next.add(id);
  }
  return next;
}

interface MarqueeSelectionOptions {
  containerRef: RefObject<HTMLElement | null>;
  selected: ReadonlySet<string>;
  setSelected: (update: (current: Set<string>) => Set<string>) => void;
  enabled?: boolean;
  minDragDistance?: number;
  onSelectionStart?: () => void;
}

const ignoredMarqueeTargets = "button, a, input, textarea, select, label";

/** Vue useMarqueeSelect parity for document lists. */
export function useMarqueeSelection({
  containerRef,
  selected,
  setSelected,
  enabled = true,
  minDragDistance = 6,
  onSelectionStart,
}: MarqueeSelectionOptions): MarqueeSelectionState & { onMouseDown: (event: React.MouseEvent) => void } {
  const drag = useRef<{ x: number; y: number; mode: MarqueeMode; active: boolean; visible: boolean }>({
    x: 0, y: 0, mode: "add", active: false, visible: false,
  });
  const [state, setState] = useState<MarqueeSelectionState>({
    visible: false, mode: "add", left: 0, top: 0, width: 0, height: 0,
  });

  const onMouseDown = useCallback((event: React.MouseEvent) => {
    if (!enabled || event.button !== 0 || (event.target as Element | null)?.closest(ignoredMarqueeTargets)) return;
    const container = containerRef.current;
    if (!container) return;
    const item = (event.target as Element | null)?.closest<HTMLElement>('[data-select-id]');
    const id = item?.dataset.selectId;
    drag.current = { x: event.clientX, y: event.clientY, mode: id && selected.has(id) ? "subtract" : "add", active: true, visible: false };
    setState((current) => ({ ...current, visible: false }));
  }, [containerRef, enabled, selected]);

  useEffect(() => {
    const move = (event: MouseEvent) => {
      const current = drag.current;
      if (!current.active) return;
      const dx = event.clientX - current.x;
      const dy = event.clientY - current.y;
      if (!current.visible && Math.hypot(dx, dy) < minDragDistance) return;
      if (!current.visible) {
        current.visible = true;
        onSelectionStart?.();
        document.body.style.userSelect = "none";
      }
      const container = containerRef.current;
      if (!container) return;
      const rect = container.getBoundingClientRect();
      const leftClient = Math.min(current.x, event.clientX);
      const topClient = Math.min(current.y, event.clientY);
      setState({
        visible: true,
        mode: current.mode,
        left: leftClient - rect.left + container.scrollLeft,
        top: topClient - rect.top + container.scrollTop,
        width: Math.abs(dx),
        height: Math.abs(dy),
      });
      const box = { left: leftClient, top: topClient, right: Math.max(current.x, event.clientX), bottom: Math.max(current.y, event.clientY) };
      const hits = new Set<string>();
      container.querySelectorAll<HTMLElement>('[data-select-id]').forEach((element) => {
        const itemRect = element.getBoundingClientRect();
        if (!(box.right < itemRect.left || box.left > itemRect.right || box.bottom < itemRect.top || box.top > itemRect.bottom)) {
          const itemId = element.dataset.selectId;
          if (itemId) hits.add(itemId);
        }
      });
      setSelected((currentSelection) => {
        return applyMarqueeSelection(currentSelection, hits, current.mode);
      });
    };
    const end = () => {
      if (!drag.current.active) return;
      drag.current.active = false;
      drag.current.visible = false;
      document.body.style.removeProperty("user-select");
      setState((current) => ({ ...current, visible: false, mode: "add" }));
    };
    document.addEventListener("mousemove", move);
    document.addEventListener("mouseup", end);
    return () => {
      document.removeEventListener("mousemove", move);
      document.removeEventListener("mouseup", end);
      document.body.style.removeProperty("user-select");
    };
  }, [containerRef, minDragDistance, onSelectionStart, setSelected]);

  return { ...state, onMouseDown };
}

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
  } else {
    // Vue toggles the requested ID even if a refresh or pagination change has
    // removed its row from the current page. Only range selection needs a
    // current-page index.
    if (input.checked) next.add(input.id);
    else next.delete(input.id);
  }
  return { selected: next, lastIndex: index };
}

/**
 * Vue KnowledgeBase.vue toggleSelectAll parity. The header checkbox changes
 * only the current page, so selections retained from a different page remain
 * available to the batch actions and total counter.
 */
export function toggleDocumentPageSelection(input: {
  ids: readonly string[];
  selected: ReadonlySet<string>;
  checked: boolean;
}): Set<string> {
  const next = new Set(input.selected);
  for (const id of input.ids) {
    if (input.checked) next.add(id);
    else next.delete(id);
  }
  return next;
}
