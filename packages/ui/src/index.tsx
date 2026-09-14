import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react';
import { cn } from './lib/utils.ts';
export { Dialog } from './dialog.tsx';
export type { DialogProps } from './dialog.tsx';
export { Input } from './input.tsx';
export { NumberInput } from './number-input.tsx';
export { Switch } from './switch.tsx';
export { cn } from './lib/utils.ts';
import './styles.css';

export function Button({ children, loading = false, disabled, className, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode; loading?: boolean }) {
  return <button className={cn('wk-button inline-flex min-h-8 items-center justify-center gap-1.5 rounded-md border border-[#b8c5d6] bg-white px-3 py-2 text-sm text-[#172033] transition-colors hover:border-[#2e6de6] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2e6de6]/35 disabled:cursor-not-allowed disabled:opacity-60', className)} aria-busy={loading || undefined} disabled={disabled || loading} {...props}>{loading ? <span className="wk-button-loading" aria-hidden="true">…</span> : null}{children}</button>;
}
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return <section className={cn('wk-card rounded-lg border border-[#dce3ed] bg-white p-4', className)} {...props}>{children}</section>;
}

export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  return <p className={cn('wk-status my-2 text-sm text-[#506078]', tone === 'error' && 'wk-status-error text-[#b42318]', tone === 'success' && 'wk-status-success text-[#137333]', tone === 'warning' && 'wk-status-warning text-[#9a6700]')} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
