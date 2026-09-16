import type { HTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './lib/utils.ts';

const badgeVariants = cva(
  'inline-flex items-center rounded-pill border px-2 py-px text-xs font-medium',
  {
    variants: {
      tone: {
        neutral: 'border-line bg-surface-alt text-muted-strong',
        primary: 'border-transparent bg-surface-wash text-primary-strong',
        success: 'border-transparent bg-[rgba(0,168,112,0.08)] text-accent-deep',
        warning: 'border-[#fedf89] bg-[#fffaeb] text-[#b45309]',
        danger: 'border-transparent bg-[rgba(213,73,65,0.1)] text-danger',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

export function Badge({ className, tone, ...props }: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
