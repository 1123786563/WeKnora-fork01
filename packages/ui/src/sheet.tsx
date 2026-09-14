import type { CSSProperties, ReactNode } from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cn } from './lib/utils.ts';

export interface SheetProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  closeLabel?: string;
  /** 侧向：right（默认，从右滑出）或 left。 */
  side?: 'left' | 'right';
  /** 面板宽度（默认 420px；可传任意 CSS 宽度）。 */
  width?: string;
  className?: string;
}

const sideClasses = {
  right: 'right-0 top-0 h-full border-l',
  left: 'left-0 top-0 h-full border-r',
} as const;

/**
 * 侧滑抽屉（Radix Dialog 承载）：用于 API 调试、向导等与主内容并行的临时表面。
 * z 取 1200 层级，置于常规弹窗（1100）之上，对齐 integrations 抽屉既定语义。
 */
export function Sheet({ open, title, children, onClose, closeLabel = 'Close', side = 'right', width = '420px', className }: SheetProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-[1200] bg-[rgb(23_32_51_/_0.45)]" />
        <DialogPrimitive.Content
          aria-describedby={undefined}
          className={cn(
            'fixed z-[1201] flex max-w-full flex-col overflow-y-auto border-line bg-surface shadow-[0_20px_60px_rgba(23,32,51,0.2)]',
            sideClasses[side],
            className,
          )}
          style={{ width, animation: side === 'right' ? 'sheet-in-right .2s ease-out' : 'sheet-in-left .2s ease-out' } as CSSProperties}
        >
          <header className="flex items-start justify-between gap-4 px-5 py-4">
            <DialogPrimitive.Title asChild>
              <h2 className="m-0 text-[1.05rem] font-semibold text-ink">{title}</h2>
            </DialogPrimitive.Title>
            <DialogPrimitive.Close
              aria-label={closeLabel}
              className="cursor-pointer rounded-control border-0 bg-transparent px-1.5 py-1 text-[1.25rem] leading-none text-muted hover:bg-hover-wash focus-visible:outline-[3px] focus-visible:outline-offset-2 focus-visible:outline-primary/35"
            >
              ×
            </DialogPrimitive.Close>
          </header>
          <div className="mt-1 flex-1 px-5 pb-5">{children}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
