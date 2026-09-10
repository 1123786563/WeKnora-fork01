import { useEffect, useId, useRef, type ReactNode } from 'react';

export interface DialogProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  closeLabel?: string;
}

/** A DOM-only primitive: focus enters the dialog, Escape closes it, and focus returns on close. */
export function Dialog({ open, title, children, onClose, closeLabel = 'Close' }: DialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  const titleId = useId();

  useEffect(() => {
    if (!open) return;
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialogRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      restoreRef.current?.focus();
      restoreRef.current = null;
    };
  }, [onClose, open]);

  if (!open) return null;
  return <div className="wk-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <section className="wk-dialog" role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} ref={dialogRef}>
      <header className="wk-dialog-header"><h2 id={titleId}>{title}</h2><button className="wk-dialog-close" type="button" onClick={onClose} aria-label={closeLabel}>×</button></header>
      <div className="wk-dialog-body">{children}</div>
    </section>
  </div>;
}
