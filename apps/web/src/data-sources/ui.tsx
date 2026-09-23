// S5 tdesign 迁移（data-sources 域，Vue 对应 frontend/src/views/knowledge/
// settings/DataSourceSettings.vue + DataSourceEditorDialog.vue）：旧栈 UI 包
// 拆除后的域内原生语义封装。playbook §1 附注：Card/Status 无 TDesign 对应
// 组件——保留原生标签 + 既有类名；类值与 packages/ui/src/index.tsx:29-37
// （Card/Status 合并定义处）的 utilities 逐字一致，视觉零变化。
import type { HTMLAttributes, ReactNode } from 'react';
import './data-sources-u.css';

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card）。 */
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  const classes = ['wk-dsui-card', className].filter(Boolean).join(' ');
  return <section className={classes} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 实际生效的 .wk-status：13px，muted-strong）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  /* T15：旧栈 utility 串语义化为 .wk-dsui-status（含 tone 变体，见 data-sources-u.css）。 */
  const toneClass = tone === 'error' ? 'wk-dsui-status--error' : tone === 'success' ? 'wk-dsui-status--success' : tone === 'warning' ? 'wk-dsui-status--warning' : '';
  const classes = ['wk-dsui-status', toneClass].filter(Boolean).join(' ');
  return <p className={classes} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
