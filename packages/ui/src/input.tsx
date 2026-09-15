import * as React from 'react';
import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/** 语义输入框：中性描边 + 控件绿 focus（视觉 = 本组件既有的 utilities 声明）。 */
export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** Convenience alias for consumers that do not already set aria-invalid. */
  invalid?: boolean;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(function Input({ className, invalid, 'aria-invalid': ariaInvalid, ...props }, ref) {
  const isInvalid = invalid ?? ariaInvalid;
  return <input ref={ref} aria-invalid={isInvalid || undefined} className={cn('h-8 w-full rounded-field border border-line-input bg-surface px-2 text-[13px] leading-[22px] text-ink outline-none transition-colors placeholder:text-placeholder hover:border-accent focus:border-accent focus:shadow-none focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-accent/35 aria-[invalid=true]:border-danger aria-[invalid=true]:focus:border-danger disabled:cursor-not-allowed disabled:border-line-input disabled:bg-disabled-bg disabled:text-muted/45', className)} {...props} />;
});
