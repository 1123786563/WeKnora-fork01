import { useId, type ReactNode } from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cn } from './lib/utils.ts';

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
  const titleId = useId();
  return <DialogPrimitive.Root open={open} onOpenChange={(nextOpen) => { if (!nextOpen) onClose(); }}>
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="wk-dialog-backdrop" />
      <DialogPrimitive.Content className={cn('wk-dialog', className)} aria-labelledby={titleId} aria-modal="true">
        <header className="wk-dialog-header"><DialogPrimitive.Title asChild><h2 id={titleId}>{title}</h2></DialogPrimitive.Title><DialogPrimitive.Close asChild><button className="wk-dialog-close" type="button" aria-label={closeLabel}>×</button></DialogPrimitive.Close></header>
        <div className="wk-dialog-body">{children}</div>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  </DialogPrimitive.Root>;
}
