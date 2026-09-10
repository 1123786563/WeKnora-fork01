import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react';
export { Dialog } from './dialog.tsx';
export type { DialogProps } from './dialog.tsx';
import './styles.css';

export function Button({ children, loading = false, disabled, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode; loading?: boolean }) {
  return <button className="wk-button" aria-busy={loading || undefined} disabled={disabled || loading} {...props}>{loading ? <span className="wk-button-loading" aria-hidden="true">…</span> : null}{children}</button>;
}
export function Card({ children, ...props }: HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return <section className="wk-card" {...props}>{children}</section>;
}

export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  return <p className={`wk-status wk-status-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
