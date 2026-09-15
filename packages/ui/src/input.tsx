import * as React from 'react';
import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/** 语义输入框：中性描边 + 控件绿 focus（视觉 = 本组件既有的 utilities 声明）。 */
export const Input = React.forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input({ className, ...props }, ref) {
  return <input ref={ref} className={cn('h-8 w-full rounded-[3px] border border-line-input bg-surface px-2 text-[13px] text-black/90 outline-none transition-colors placeholder:text-placeholder hover:border-accent focus:border-accent focus:ring-2 focus:ring-accent/20 aria-[invalid=true]:border-danger aria-[invalid=true]:focus:ring-danger/20 disabled:cursor-not-allowed disabled:border-line-input disabled:bg-disabled-bg disabled:text-black/25', className)} {...props} />;
});
