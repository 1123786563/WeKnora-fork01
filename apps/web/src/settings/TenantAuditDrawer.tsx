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
  // S6 Tailwind 收编：chrome utilities → settings.td.css §7e 的
  // .wk-audit-drawer-*（portal 挂 body，unscoped）。inline animation 沿用
  // sheet-in-right 名（现状即未定义 keyframes 的 no-op，与 packages/ui 删除无关）。
  const content = <>
    <div
      className="wk-audit-drawer-overlay"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    />
    <div
      className="wk-audit-drawer-resize"
      style={{ right: `calc(${drawerWidth}px - 6px)` }}
      role="separator"
      aria-orientation="vertical"
      onMouseDown={onResizeStart}
    >
      <span aria-hidden className={'wk-audit-drawer-resize-handle' + (resizing ? ' is-resizing' : '')} />
    </div>
    <aside
      ref={panelRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabIndex={-1}
      className="wk-audit-drawer-panel"
      style={{ width: `${drawerWidth}px`, animation: 'sheet-in-right .2s ease-out' }}
    >
      <header className="wk-audit-drawer-head">
        <h2 id={titleId} className="wk-audit-drawer-title">{title}</h2>
      </header>
      <div className="wk-audit-drawer-body">{children}</div>
    </aside>
  </>;
  return !mounted || typeof document === 'undefined' ? content : createPortal(content, document.body);
}
