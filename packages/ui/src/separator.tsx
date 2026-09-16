import type { HTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

export function Separator({ className, orientation = 'horizontal', ...props }: HTMLAttributes<HTMLDivElement> & { orientation?: 'horizontal' | 'vertical' }) {
  return <div role="separator" aria-orientation={orientation} className={cn('bg-line-soft', orientation === 'horizontal' ? 'h-px w-full' : 'h-full w-px', className)} {...props} />;
}
