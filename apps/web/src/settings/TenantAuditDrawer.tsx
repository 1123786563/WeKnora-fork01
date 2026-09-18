import { useEffect, useId, useRef, useState, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

/**
 * React port of frontend/src/components/settings/SettingDrawer.vue as used by
 * TenantMembers.vue for the audit log (TenantMembers.vue:383-386): a right-side
 * drawer with a draggable left-edge width handle, persisted width, Esc /
 * overlay-click close, focus restore and destroy-on-close semantics.
 *
 * Hand-written DOM contract (no Radix/Portal) matching packages/ui Dialog so
 * the minimal node --test DOM globals keep working. Width clamping mirrors
 * SettingDrawer.vue:162-166 (cap = min(maxWidth, viewport), floor = min(minWidth, cap)).
 */
export interface TenantAuditDrawerProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  /** Initial width when the user has no persisted preference (px). */
  width?: number;
  minWidth?: number;
  maxWidth?: number;
  /** localStorage key remembering the chosen width; '' disables persistence. */
  storageKey?: string;
}

function getDrawerFocusableElements(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((element) => element.getAttribute('tabindex') !== '-1');
}

export function TenantAuditDrawer({ open, title, children, onClose, width = 560, minWidth = 480, maxWidth = 1200, storageKey = '' }: TenantAuditDrawerProps) {
  const panelRef = useRef<HTMLElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const [mounted, setMounted] = useState(false);
  // Raw user preference; rendering clamps against the live viewport like the
  // Vue computed drawerWidthPx (SettingDrawer.vue:187-193).
  const [rawWidth, setRawWidth] = useState(width);
  const [viewportWidth, setViewportWidth] = useState(() => (typeof window !== 'undefined' && Number.isFinite(window.innerWidth) ? window.innerWidth : maxWidth));
  const [resizing, setResizing] = useState(false);
  const resizeRef = useRef<{ startX: number; startWidth: number } | null>(null);

  const clampWidth = (candidate: number, viewport: number): number => {
    const cap = Math.min(maxWidth, viewport);
    const floor = Math.min(minWidth, cap);
    return Math.max(floor, Math.min(cap, Math.round(candidate)));
  };

  // Persisted width wins over the prop default (SettingDrawer.vue:173-187).
  useEffect(() => {
    setMounted(true);
    if (typeof window === 'undefined' || !storageKey) return;
    try {
      const raw = window.localStorage.getItem(storageKey);
      const saved = raw === null ? Number.NaN : Number(raw);
      if (Number.isFinite(saved)) setRawWidth(clampWidth(saved, window.innerWidth));
    } catch { /* localStorage can throw in private mode / quota errors. */ }
    const onWindowResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener('resize', onWindowResize, { passive: true });
    return () => window.removeEventListener('resize', onWindowResize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storageKey]);

  const drawerWidth = clampWidth(rawWidth, viewportWidth);

  // Drag-resize (SettingDrawer.vue:218-248): delta = startX - clientX so
  // dragging left widens the right-anchored panel; persist on mouseup.
  useEffect(() => {
    if (!resizing) return;
    const move = (event: MouseEvent) => {
      if (!resizeRef.current) return;
      const delta = resizeRef.current.startX - event.clientX;
      setRawWidth(clampWidth(resizeRef.current.startWidth + delta, window.innerWidth));
    };
    const stop = () => {
      resizeRef.current = null;
      setResizing(false);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      setRawWidth((current) => {
        const next = clampWidth(current, window.innerWidth);
        if (storageKey) { try { window.localStorage.setItem(storageKey, String(next)); } catch { /* storage is optional */ } }
        return next;
      });
    };
    document.addEventListener('mousemove', move);
    document.addEventListener('mouseup', stop);
    return () => {
      document.removeEventListener('mousemove', move);
      document.removeEventListener('mouseup', stop);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resizing, storageKey]);

  function onResizeStart(event: ReactMouseEvent) {
    event.preventDefault();
    resizeRef.current = { startX: event.clientX, startWidth: drawerWidth };
    setResizing(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  // Esc to close (scoped to the drawer like packages/ui Dialog), Tab focus
  // trap, focus restore on close — same contract as the project Dialog.
  // Waits for `mounted` so focus lands on the portal-mounted panel, not the
  // pre-portal inline copy that gets replaced right after first open.
  useEffect(() => {
    if (!open || !mounted) return;
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    panelRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const activeElement = document.activeElement;
        if (activeElement && !panelRef.current?.contains(activeElement)) return;
        event.preventDefault();
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.key !== 'Tab' || !panelRef.current) return;
      const focusable = getDrawerFocusableElements(panelRef.current);
      if (focusable.length === 0) { event.preventDefault(); panelRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      const active = document.activeElement;
      if (event.shiftKey && (active === first || active === panelRef.current)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && active === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('keydown', onKeyDown); restoreRef.current?.focus(); restoreRef.current = null; };
  }, [onClose, open, mounted]);

  if (!open) return null;
  // Vue header note (SettingDrawer.vue:13-19): no redundant X button — the
  // close affordances are Esc + the underlying overlay click.
  const content = <>
    <div
      className="fixed inset-0 z-[var(--wk-overlay-settings-z)] bg-[rgb(23_32_51_/_0.45)]"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    />
    <div
      className="group fixed bottom-0 top-0 z-[calc(var(--wk-overlay-settings-z)+2)] w-3 cursor-col-resize"
      style={{ right: `calc(${drawerWidth}px - 6px)` }}
      role="separator"
      aria-orientation="vertical"
      onMouseDown={onResizeStart}
    >
      <span aria-hidden className={'mx-auto block h-12 w-0.5 rounded-[1px] bg-[var(--wk-border,#dce3ed)] opacity-55 transition-opacity group-hover:opacity-100 group-hover:bg-accent ' + (resizing ? 'opacity-100! bg-accent!' : '')} />
    </div>
    <aside
      ref={panelRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabIndex={-1}
      className="fixed bottom-0 right-0 top-0 z-[calc(var(--wk-overlay-settings-z)+1)] flex max-w-full flex-col overflow-hidden border-l border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] shadow-[0_20px_60px_rgba(23,32,51,0.2)] outline-none"
      style={{ width: `${drawerWidth}px`, animation: 'sheet-in-right .2s ease-out' }}
    >
      <header className="flex shrink-0 items-center gap-2.5 border-b border-[var(--wk-border,#dce3ed)] px-[18px] py-[14px]">
        <h2 id={titleId} className="m-0 overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold leading-[1.4] text-[var(--wk-text,#172033)]">{title}</h2>
      </header>
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden px-[18px] py-4">{children}</div>
    </aside>
  </>;
  return !mounted || typeof document === 'undefined' ? content : createPortal(content, document.body);
}
