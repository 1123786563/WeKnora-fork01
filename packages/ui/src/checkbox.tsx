import * as React from 'react';
import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

/**
 * 原生 checkbox 的样式化封装：保留原生表单语义与键盘行为，
 * 视觉跟随控件绿（accent-color），禁用态继承原生。
 */
export function Checkbox({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input type="checkbox" className={cn('h-3.5 w-3.5 shrink-0 cursor-pointer accent-[#07c05f] disabled:cursor-not-allowed', className)} {...props} />;
}
