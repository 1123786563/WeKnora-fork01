import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './lib/utils.ts';

/**
 * shadcn/ui 风格按钮：cva variants 管理视觉形态与状态。
 * 视觉基线 = 既有 .wk-button（apps/web styles.css，当前实际生效样式）：
 * 默认 = 白底细描边；primary = 控件绿实底（Vue tdesign 主操作）；
 * text = 透明文本钮；danger = 文本钮 + 危险色。
 */
const buttonVariants = cva(
  'inline-flex min-h-8 cursor-pointer items-center justify-center gap-1.5 rounded-control font-sans text-sm leading-5 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/35 disabled:cursor-not-allowed disabled:opacity-60',
  {
    variants: {
      variant: {
        default:
          'border border-line-control bg-surface px-[0.8rem] py-[0.55rem] text-ink hover:border-accent',
        primary:
          'border border-accent bg-accent px-[0.8rem] py-[0.55rem] text-surface hover:bg-accent-strong',
        text:
          'border border-transparent bg-transparent px-[0.5rem] py-[0.3rem] text-muted-strong hover:bg-hover-wash focus-visible:bg-hover-wash',
        danger:
          'border border-transparent bg-transparent px-[0.5rem] py-[0.3rem] text-danger hover:bg-hover-wash focus-visible:bg-hover-wash',
      },
      size: {
        small: 'wk-button-size-small min-h-7 text-xs',
        medium: 'wk-button-size-medium',
        large: 'wk-button-size-large min-h-10 px-4 text-base',
      },
    },
    defaultVariants: { variant: 'default', size: 'medium' },
  },
);

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  children?: ReactNode;
  /** 既有 API：loading 时展示省略号并禁用（aria-busy）。 */
  loading?: boolean;
}

export function Button({ children, loading = false, disabled, className, variant, size, ...props }: ButtonProps) {
  return (
    <button
      className={cn(buttonVariants({ variant, size }), className)}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? <span className="wk-button-loading" aria-hidden="true" /> : null}
      {children}
    </button>
  );
}
