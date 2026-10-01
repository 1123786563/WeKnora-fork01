// 旧栈 UI 包拆除后的域内原生语义封装（同 data-sources/ui.tsx 与
// plugins/status.tsx 惯例）：Card/Status 无 TDesign 对应组件，保留原生标签；
// Button 由调用方直接改用 tdesign-react。本域无独立 CSS，用内联样式保持
// 自包含（商业三页为 React-only 域，无 Vue 像素基线）。
import type { HTMLAttributes, ReactNode } from 'react';

/** 语义卡片：白底、细描边、圆角（视觉 = 旧 .wk-card）。 */
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  const classes = ['wk-card', className].filter(Boolean).join(' ');
  return <section className={classes} style={{ background: '#fff', border: '1px solid var(--td-component-border, #dcdcdc)', borderRadius: 6, padding: '1rem' }} {...props}>{children}</section>;
}

/** 内联状态文本：13px muted，tone 变体（同 plugins/status.tsx 视觉）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  const color = tone === 'error' ? '#b42318' : tone === 'success' ? '#137333' : tone === 'warning' ? '#9a6700' : '#506078';
  return <p role={tone === 'error' ? 'alert' : 'status'} style={{ margin: '0.25rem 0', fontSize: 13, color }}>{children}</p>;
}
