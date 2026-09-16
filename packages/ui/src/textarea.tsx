import * as React from 'react';
import type { TextareaHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/** 语义多行输入（与 Input 同一控件语言：中性描边 + 控件绿 focus）。 */
export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea({ className, ...props }, ref) {
  return <textarea ref={ref} className={cn('w-full rounded-[3px] border border-line-input bg-surface px-2 py-1.5 text-[13px] text-black/90 outline-none transition-colors placeholder:text-placeholder hover:border-accent focus:border-accent focus:ring-2 focus:ring-accent/20 disabled:cursor-not-allowed disabled:border-line-input disabled:bg-disabled-bg disabled:text-black/25', className)} {...props} />;
});
