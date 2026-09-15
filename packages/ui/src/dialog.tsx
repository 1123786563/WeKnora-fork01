import { useEffect, useId, useRef, type ReactNode } from 'react';

export interface DialogProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  closeLabel?: string;
  className?: string;
}

export function getDialogFocusableElements(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((element) => element.getAttribute('tabindex') !== '-1');
}

/** Project-owned primitive: shadcn/Radix remains a substrate, while this
 * wrapper keeps Vue's focus, Escape, outside-close and test-host contract. */
export function Dialog({ open, title, children, onClose, closeLabel = 'Close', className }: DialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  useEffect(() => {
    if (!open) return;
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialogRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = getDialogFocusableElements(dialogRef.current);
      if (focusable.length === 0) { event.preventDefault(); dialogRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      const active = document.activeElement;
      if (event.shiftKey && (active === first || active === dialogRef.current)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && active === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('keydown', onKeyDown); restoreRef.current?.focus(); restoreRef.current = null; };
  }, [onClose, open]);
  if (!open) return null;
  return <div className="wk-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className={className ? `wk-dialog ${className}` : 'wk-dialog'} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} ref={dialogRef}><header className="wk-dialog-header"><h2 id={titleId}>{title}</h2><button className="wk-dialog-close" type="button" onClick={onClose} aria-label={closeLabel}>×</button></header><div className="wk-dialog-body">{children}</div></section></div>;
}
