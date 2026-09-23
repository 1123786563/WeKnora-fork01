// S5 tdesign 迁移（configuration 域，React 独有管理页：Vue 无 /platform/
// configuration 路由）：旧栈 UI 包拆除后的域内原生语义封装。playbook §1 附注：
// Card/Status 无 TDesign 对应组件——保留原生标签 + 既有类名；类值与
// packages/ui/src/index.tsx:29-37（Card/Status 合并定义处）的 utilities
// 逐字一致，视觉零变化。表单控件（Button/
// Input/Select/Textarea/Checkbox）一律走 tdesign-react（本域旧默认按钮译
// theme="default" variant="outline"）。文件名避开既有 surface.ts（域逻辑模块）。
import type { HTMLAttributes, ReactNode } from 'react';
import './config-u.css';

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card）。 */
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  const classes = ['wk-cfg-card', className].filter(Boolean).join(' ');
  return <section className={classes} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 实际生效的 .wk-status：13px，muted-strong）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  const toneClass = tone === 'error' ? 'text-danger' : tone === 'success' ? 'text-success-text' : tone === 'warning' ? 'text-warning-text' : '';
  const classes = ['my-[0.25rem] text-[13px] text-muted-strong', toneClass].filter(Boolean).join(' ');
  return <p className={classes} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
