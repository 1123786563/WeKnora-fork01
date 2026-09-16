import type { LabelHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

export function Label({ className, ...props }: LabelHTMLAttributes<HTMLLabelElement>) {
  return <label className={cn('text-[13px] font-medium text-ink', className)} {...props} />;
}
