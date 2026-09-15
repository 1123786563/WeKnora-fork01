import React, { type HTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './lib/utils.ts';

const alertVariants = cva(
  'rounded-card border px-3 py-2.5 text-[13px] leading-relaxed',
  {
    variants: {
      tone: {
        neutral: 'border-line bg-surface text-muted-strong',
        info: 'border-[#b3ccf5] bg-surface-wash text-primary-soft',
        success: 'border-[#8ce0af] bg-[rgba(7,192,95,0.08)] text-accent-strong',
        warning: 'border-[#fedf89] bg-[#fffaeb] text-[#b45309]',
        danger: 'border-[#f1b8b3] bg-[#fdecea] text-danger',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

/** 行内警示条（对齐既有 popconfirm/横幅语言；不承担 toast 职责）。 */
export function Alert({ className, tone, ...props }: HTMLAttributes<HTMLDivElement> & VariantProps<typeof alertVariants>) {
  return <div role="alert" className={cn(alertVariants({ tone }), className)} {...props} />;
}
