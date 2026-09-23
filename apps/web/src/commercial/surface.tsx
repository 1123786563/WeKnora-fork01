// S5 tdesign 迁移（React 独有 commercial 域）：旧栈 UI 包拆除后的域内原生
// 语义封装。playbook §1 附注：Card/Status 无 TDesign 对应组件——保留原生标签
// + 既有类名；类值与 packages/ui/src/index.tsx:29-37（Card/Status 合并定义
// 处）的 utilities 逐字一致，视觉零变化。Button 一律走 tdesign-react（本域
// 调用点 theme="default" variant="outline" 对应旧栈默认白底细描边）。
import type { HTMLAttributes, ReactNode } from 'react';
import './commercial-u.css';

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card）。 */
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  const classes = ['wk-cs-card', className].filter(Boolean).join(' ');
  return <section className={classes} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 实际生效的 .wk-status：13px，muted-strong）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  const toneClass = tone === 'error' ? 'text-danger' : tone === 'success' ? 'text-success-text' : tone === 'warning' ? 'text-warning-text' : '';
  const classes = ['my-[0.25rem] text-[13px] text-muted-strong', toneClass].filter(Boolean).join(' ');
  return <p className={classes} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
