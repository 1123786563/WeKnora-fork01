import React, { type SelectHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/**
 * 原生 select 的样式化封装（shadcn select-native 形态）。
 * 刻意不用 Radix Select：既有表单与 e2e 依赖原生 select 语义与键盘行为。
 */
export const Select = React.forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(function Select({ className, ...props }, ref) {
  return <select ref={ref} className={cn('h-8 w-full cursor-pointer appearance-none rounded-[3px] border border-line-input bg-surface bg-[url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns=%27http://www.w3.org/2000/svg%27 width=%2710%27 height=%276%27%3E%3Cpath d=%27M1 1l4 4 4-4%27 stroke=%27%2366758b%27 stroke-width=%271.5%27 fill=%27none%27/%3E%3C/svg%3E")] bg-[position:right_8px_center] bg-no-repeat px-2 pr-6 text-[13px] text-black/90 outline-none transition-colors hover:border-accent focus:border-accent focus:ring-2 focus:ring-accent/20 aria-[invalid=true]:border-danger aria-[invalid=true]:focus:ring-danger/20 disabled:cursor-not-allowed disabled:border-line-input disabled:bg-disabled-bg disabled:text-black/25', className)} {...props} />;
});
