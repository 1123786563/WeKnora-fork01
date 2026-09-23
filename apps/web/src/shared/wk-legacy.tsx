// S6 编辑抽屉收编共享兼容层：@weknora/ui 的语义 div/p 封装（playbook §1 附行：
// Card/Status 无 TDesign 对应组件）在离开 @weknora/ui（T15 硬前置）后以原生
// 标签 + 语义类名延续。视觉规则自 packages/ui/src/styles.css:102-114 原样平移
// 至同目录 wk-legacy.css（token 字面量与 theme.css 取值逐项相等，T15 删除
// @weknora/ui 后渲染不变）。类名保持 .wk-card / .wk-status 族——它们是现有
// 测试与消费方 CSS 的查询锚点（styles.css:97 头注先例）。
import type { HTMLAttributes, ReactNode } from 'react';
import './wk-legacy.css';

function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ');
}

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card，原 @weknora/ui Card）。 */
export function WkCard({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  return <section className={cn('wk-card', className)} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 既有 .wk-status：13px，muted-strong；原 @weknora/ui Status）。 */
export function WkStatus({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  return <p className={cn('wk-status', tone !== 'neutral' && `wk-status--${tone}`)} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
