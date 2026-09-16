import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/** 原生 radio 的样式化封装：保留浏览器分组、键盘和表单语义。 */
export function Radio({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input type="radio" className={cn('h-3.5 w-3.5 shrink-0 cursor-pointer accent-[#07c05f] disabled:cursor-not-allowed', className)} {...props} />;
}
