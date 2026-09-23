import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';

/**
 * Embed 本地按钮（T15：packages/ui 旧栈退役后的 1:1 生效值复刻）。
 *
 * 原 Button 来自 packages/ui/src/button.tsx（cva + 旧栈 theme token utilities）；
 * embed 只消费 text/primary × small/medium 四种组合，这里按当时的计算样式
 * （unlayered .wk-button-size-small 恒胜 layered utilities 的层叠结果）编码为
 * .embed-btn 语义类（apps/embed/src/styles.css），视觉基线不变：
 * - 基类：inline-flex 居中、6px 圆角、系统字体栈、150ms 颜色过渡、
 *   accent 35% 焦点环、disabled 60% 透明；
 * - text：透明底 + muted-strong 文字 + hover 浅底 #f2f5fa；
 * - primary：#07c05f 实底白字（hover #0a8f4c）；
 * - small：28px 高（min-height 24px）、12px/16px 字、2px 8px 内边距。
 */
export type EmbedButtonVariant = 'text' | 'primary';
export type EmbedButtonSize = 'small' | 'medium';

export interface EmbedButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: EmbedButtonVariant;
  size?: EmbedButtonSize;
  children?: ReactNode;
}

export function Button({
  children,
  variant = 'text',
  size = 'medium',
  type = 'button',
  className,
  ...props
}: EmbedButtonProps) {
  const classes = [
    'embed-btn',
    variant === 'primary' ? 'embed-btn--primary' : 'embed-btn--text',
    size === 'small' ? 'embed-btn--small' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');
  return (
    <button type={type} className={classes} {...props}>
      {children}
    </button>
  );
}
