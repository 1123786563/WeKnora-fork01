import React, { type HTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './lib/utils.ts';

const alertVariants = cva(
  'rounded-card border px-3 py-2.5 text-[13px] leading-relaxed',
  {
    variants: {
      tone: {
        neutral: 'border-line bg-surface text-muted-strong',
        info: 'border-info-line bg-surface-wash text-primary-soft',
        success: 'border-success-line bg-accent-wash text-accent-strong',
        warning: 'border-warning-line bg-warning-wash text-warning-strong',
        danger: 'border-danger-line bg-danger-wash text-danger',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

/** 行内警示条（对齐既有 popconfirm/横幅语言；不承担 toast 职责）。 */
export function Alert({ className, tone, ...props }: HTMLAttributes<HTMLDivElement> & VariantProps<typeof alertVariants>) {
  return <div role="alert" className={cn(alertVariants({ tone }), className)} {...props} />;
}
