import { forwardRef, type HTMLAttributes, type ReactNode } from 'react';
import * as TabsPrimitive from '@radix-ui/react-tabs';
import { cn } from './lib/utils.ts';

/** shadcn/ui Tabs（Radix）：方向键漫游、aria 接线开箱即用。 */
export const Tabs = TabsPrimitive.Root;

export const TabsList = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <TabsPrimitive.List ref={ref} className={cn('flex items-center gap-1 border-b border-line-soft', className)} {...props} />
  ),
);
TabsList.displayName = 'TabsList';

export const TabsTrigger = forwardRef<HTMLButtonElement, { value: string; children: ReactNode } & HTMLAttributes<HTMLButtonElement>>(
  ({ className, children, ...props }, ref) => (
    <TabsPrimitive.Trigger
      ref={ref}
      className={cn(
        'cursor-pointer rounded-t-control border border-b-0 border-transparent px-3 py-1.5 text-[13px] text-muted-strong transition-colors hover:text-ink',
        'focus-visible:outline-[3px] focus-visible:outline-offset-[-2px] focus-visible:outline-primary/35 disabled:cursor-not-allowed disabled:opacity-55',
        'data-[state=active]:border-line data-[state=active]:bg-canvas data-[state=active]:font-medium data-[state=active]:text-ink',
        className,
      )}
      {...props}
    >
      {children}
    </TabsPrimitive.Trigger>
  ),
);
TabsTrigger.displayName = 'TabsTrigger';

export const TabsContent = forwardRef<HTMLDivElement, { value: string; children: ReactNode } & HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => (
    <TabsPrimitive.Content ref={ref} className={cn('pt-3 focus-visible:outline-none', className)} {...props}>
      {children}
    </TabsPrimitive.Content>
  ),
);
TabsContent.displayName = 'TabsContent';
