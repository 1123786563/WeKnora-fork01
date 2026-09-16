import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/** 原生 range 的项目封装：保留 Vue 所需的键盘、步进和表单语义。 */
export function Range({ className, min, max, step, ...props }: Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>) {
  return <input type="range" min={min} max={max} step={step} className={cn('cursor-pointer accent-accent disabled:cursor-not-allowed disabled:opacity-55', className)} {...props} />;
}
