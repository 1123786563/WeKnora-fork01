import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react';
import './styles.css';

export function Button({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode }) {
  return <button className="wk-button" {...props}>{children}</button>;
}
export function Card({ children, ...props }: HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return <section className="wk-card" {...props}>{children}</section>;
}

export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success'; children: ReactNode }) {
  return <p className={`wk-status wk-status-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
