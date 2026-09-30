import React, { type SelectHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/**
 * 原生 select 的样式化封装（shadcn select-native 形态）。
 * 刻意不用 Radix Select：既有表单与 e2e 依赖原生 select 语义与键盘行为。
 */
export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
}

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(function Select({ className, invalid, 'aria-invalid': ariaInvalid, ...props }, ref) {
  const isInvalid = invalid ?? ariaInvalid;
  return <select ref={ref} role="combobox" aria-invalid={isInvalid || undefined} className={cn('h-8 w-full cursor-pointer appearance-none rounded-field border border-line-input bg-surface bg-[url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns=%27http://www.w3.org/2000/svg%27 width=%2710%27 height=%276%27%3E%3Cpath d=%27M1 1l4 4 4-4%27 stroke=%27%2366758b%27 stroke-width=%271.5%27 fill=%27none%27/%3E%3C/svg%3E")] bg-[position:right_8px_center] bg-no-repeat px-2 pr-6 text-[13px] leading-[22px] text-ink outline-none transition-colors hover:border-accent focus:border-accent focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-accent/35 aria-[invalid=true]:border-danger aria-[invalid=true]:focus:border-danger disabled:cursor-not-allowed disabled:border-line-input disabled:bg-disabled-bg disabled:text-muted/45', className)} {...props} />;
});
