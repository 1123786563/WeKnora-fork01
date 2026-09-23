// S6 编辑抽屉收编共享兼容层：packages/ui 旧栈 的语义 div/p 封装（playbook §1 附行：
// Card/Status 无 TDesign 对应组件）在离开 packages/ui 旧栈（T15 硬前置）后以原生
// 标签 + 语义类名延续。视觉规则自 packages/ui/src/styles.css:102-114 原样平移
// 至同目录 wk-legacy.css（token 字面量与 theme.css 取值逐项相等，T15 删除
// packages/ui 旧栈 后渲染不变）。类名保持 .wk-card / .wk-status 族——它们是现有
// 测试与消费方 CSS 的查询锚点（styles.css:97 头注先例）。
//
// S6+ 追加（T15 前置）：WkDialog / WkSheet 是 packages/ui dialog.tsx / sheet.tsx
// 的逐行 DOM 同构副本（渲染树、类名、aria、焦点陷阱、Esc/遮罩关闭语义不变），
// 供扫描锚定弹层（kb-settings / 上传确认 / doc-detail 抽屉等）在 T15 删包后
// 继续 0% 回归。原组件的 Tailwind utilities 承载值平移进 wk-legacy.css 的
// .wk-dialog*/.wk-sheet* 规则（逐项对照 packages/ui styles.css:115-131 与
// sheet.tsx:108-146 的 utilities）。
import React, { useEffect, useId, useRef, useState, type CSSProperties, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import './wk-legacy.css';

function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ');
}

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card，原 packages/ui 旧栈 Card）。 */
export function WkCard({ children, className, ...props }: React.HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  return <section className={cn('wk-card', className)} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 既有 .wk-status：13px，muted-strong；原 packages/ui 旧栈 Status）。 */
export function WkStatus({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  return <p className={cn('wk-status', tone !== 'neutral' && `wk-status--${tone}`)} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}

const openDialogStack: HTMLElement[] = [];

/** packages/ui 旧栈 Dialog 的 DOM 同构副本（packages/ui/src/dialog.tsx:20-53 全量语义）。 */
export function WkDialog({ open, title, children, onClose, closeLabel = 'Close', className, portal = true }: {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  closeLabel?: string;
  className?: string;
  /** Keep SSR/static rendering inline; browser usage portals to body by default. */
  portal?: boolean;
}) {
  const dialogRef = useRef<HTMLElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true); }, []);
  useEffect(() => {
    if (!open) return;
    if (dialogRef.current) openDialogStack.push(dialogRef.current);
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialogRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (openDialogStack[openDialogStack.length - 1] !== dialogRef.current) return;
        const activeElement = document.activeElement;
        if (activeElement && !dialogRef.current?.contains(activeElement)) return;
        event.preventDefault();
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((element) => element.getAttribute('tabindex') !== '-1');
      if (focusable.length === 0) { event.preventDefault(); dialogRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      const active = document.activeElement;
      if (event.shiftKey && (active === first || active === dialogRef.current)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && active === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      const index = dialogRef.current ? openDialogStack.indexOf(dialogRef.current) : -1;
      if (index >= 0) openDialogStack.splice(index, 1);
      restoreRef.current?.focus();
      restoreRef.current = null;
    };
  }, [onClose, open]);
  if (!open) return null;
  const content = <div className="wk-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className={className ? `wk-dialog ${className}` : 'wk-dialog'} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} ref={dialogRef}><header className="wk-dialog-header"><h2 id={titleId}>{title}</h2><button className="wk-dialog-close" type="button" onClick={onClose} aria-label={closeLabel}>×</button></header><div className="wk-dialog-body">{children}</div></section></div>;
  return !portal || !mounted || typeof document === 'undefined' ? content : createPortal(content, document.body);
}

/** packages/ui 旧栈 Sheet 的 DOM 同构副本（packages/ui/src/sheet.tsx:29-147 全量语义，
 *  含自研左缘拖宽 handle——tdesign Drawer 无对应（playbook §1 #15），doc-detail
 *  等长驻抽屉依赖它，随本兼容层延续）。 */
export function WkSheet({ open, title, children, onClose, closeLabel = 'Close', side = 'right', width = '420px', resizable = false, minWidth = 320, maxWidth = 1400, storageKey, resizeLabel = 'Resize drawer', className, headerIcon, portal = true }: {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  closeLabel?: string;
  /** 侧向：right（默认，从右滑出）或 left。 */
  side?: 'left' | 'right';
  /** 面板宽度（默认 420px；可传任意 CSS 宽度）。 */
  width?: string;
  /** Enable the Vue-style left-edge width handle for long-running detail drawers. */
  resizable?: boolean;
  minWidth?: number;
  maxWidth?: number;
  storageKey?: string;
  resizeLabel?: string;
  className?: string;
  /** Optional badge rendered beside the title inside the header. */
  headerIcon?: ReactNode;
  portal?: boolean;
}) {
  const initialWidth = /^\d+(?:\.\d+)?px$/.test(width) ? Number.parseFloat(width) : 420;
  const [panelWidth, setPanelWidth] = useState(initialWidth);
  const panelRef = useRef<HTMLElement>(null);
  const titleId = useId();
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true); }, []);
  const restoreRef = useRef<HTMLElement | null>(null);
  const resizeRef = useRef<{ startX: number; startWidth: number } | null>(null);
  useEffect(() => {
    if (!storageKey || typeof window === 'undefined') return;
    const saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
    if (Number.isFinite(saved)) setPanelWidth(Math.max(minWidth, Math.min(maxWidth, saved)));
  }, [maxWidth, minWidth, storageKey]);
  useEffect(() => {
    if (!resizable) return;
    const move = (event: MouseEvent) => {
      if (!resizeRef.current) return;
      const delta = side === 'right' ? resizeRef.current.startX - event.clientX : event.clientX - resizeRef.current.startX;
      setPanelWidth(Math.max(minWidth, Math.min(maxWidth, resizeRef.current.startWidth + delta)));
    };
    const stop = () => {
      if (!resizeRef.current) return;
      resizeRef.current = null;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      if (storageKey) window.localStorage.setItem(storageKey, String(panelWidth));
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', stop);
    return () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', stop);
    };
  }, [maxWidth, minWidth, panelWidth, resizable, side, storageKey]);
  const beginResize = (event: ReactMouseEvent) => {
    if (!resizable) return;
    event.preventDefault();
    resizeRef.current = { startX: event.clientX, startWidth: panelWidth };
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };
  useEffect(() => {
    if (!open) return;
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
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'));
      if (focusable.length === 0) { event.preventDefault(); panelRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (event.shiftKey && (document.activeElement === first || document.activeElement === panelRef.current)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('keydown', onKeyDown); restoreRef.current?.focus(); restoreRef.current = null; };
  }, [onClose, open]);
  if (!open) return null;
  const content = (
    <div className="wk-sheet-backdrop" role="presentation" onPointerDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
        <aside
          ref={panelRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          tabIndex={-1}
          aria-label={String(title)}
          data-side={side}
          className={cn('wk-sheet', `wk-sheet--${side}`, className)}
          style={{ width: resizable ? `${panelWidth}px` : width, animation: side === 'right' ? 'sheet-in-right .2s ease-out' : 'sheet-in-left .2s ease-out' } as CSSProperties}
        >
          {resizable ? <div role="separator" aria-orientation="vertical" aria-label={resizeLabel} title={resizeLabel} className={`wk-sheet-resize ${side === 'right' ? 'wk-sheet-resize--right' : 'wk-sheet-resize--left'}`} onMouseDown={beginResize} /> : null}
          <header className="wk-sheet-header">
            {headerIcon ? (
              <div className="wk-sheet-header-icon-row">
                <div className="wk-sheet-header-icon">{headerIcon}</div>
                <h2 id={titleId} className="wk-sheet-title">{title}</h2>
              </div>
            ) : (
              <h2 id={titleId} className="wk-sheet-title">{title}</h2>
            )}
            <button
              type="button"
              aria-label={closeLabel}
              className="wk-sheet-close"
              onClick={onClose}
            >
              ×
            </button>
          </header>
          <div className="wk-sheet-body">{children}</div>
        </aside>
    </div>
  );
  return !portal || !mounted || typeof document === 'undefined' ? content : createPortal(content, document.body);
}
